package httpdriver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	contractsconfig "github.com/goravel/framework/contracts/config"
	contractsroute "github.com/goravel/framework/contracts/route"
)

const (
	defaultReadHeaderTimeout = 5 * time.Second
	defaultReadTimeout       = 10 * time.Second
	defaultWriteTimeout      = 15 * time.Second
	defaultIdleTimeout       = 60 * time.Second

	requestTooLargeBody = `{"error":{"code":"PAYLOAD_TOO_LARGE","message":"Dữ liệu gửi lên vượt quá giới hạn cho phép","details":{}}}`
	malformedJSONBody   = `{"error":{"code":"BAD_REQUEST","message":"Không thể đọc dữ liệu gửi lên","details":{}}}`
	requestTimeoutBody  = `{"error":{"code":"REQUEST_TIMEOUT","message":"Yêu cầu xử lý quá lâu, vui lòng thử lại","details":{}}}`
)

type Options struct {
	MaxBodyBytes      int64
	RequestTimeout    time.Duration
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

type SecureRoute struct {
	contractsroute.Route

	config  contractsconfig.Config
	options Options
	handler http.Handler

	mu        sync.Mutex
	server    *http.Server
	tlsServer *http.Server
}

func NewSecureRoute(
	base contractsroute.Route,
	config contractsconfig.Config,
	options Options,
) *SecureRoute {
	options = withDefaultTimeouts(options)

	secureRoute := &SecureRoute{
		Route:   base,
		config:  config,
		options: options,
	}
	secureRoute.handler = newRequestBoundaryHandler(
		http.HandlerFunc(base.ServeHTTP),
		options.MaxBodyBytes,
		options.RequestTimeout,
	)

	return secureRoute
}

func (r *SecureRoute) ServeHTTP(
	writer http.ResponseWriter,
	request *http.Request,
) {
	r.handler.ServeHTTP(writer, request)
}

func (r *SecureRoute) Test(request *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, request)

	return recorder.Result(), nil
}

func (r *SecureRoute) Listen(listener net.Listener) error {
	server := r.newServer(listener.Addr().String())
	r.setServer(server, false)
	fmt.Printf("[HTTP] Listening on: http://%s\n", listener.Addr().String())

	if err := server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (r *SecureRoute) ListenTLS(listener net.Listener) error {
	return r.ListenTLSWithCert(
		listener,
		r.config.GetString("http.tls.ssl.cert"),
		r.config.GetString("http.tls.ssl.key"),
	)
}

func (r *SecureRoute) ListenTLSWithCert(
	listener net.Listener,
	certFile string,
	keyFile string,
) error {
	server := r.newServer(listener.Addr().String())
	r.setServer(server, true)
	fmt.Printf("[HTTPS] Listening on: https://%s\n", listener.Addr().String())

	if err := server.ServeTLS(
		listener,
		certFile,
		keyFile,
	); !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (r *SecureRoute) Run(host ...string) error {
	address, err := r.httpAddress(host...)
	if err != nil {
		return err
	}

	server := r.newServer(address)
	r.setServer(server, false)
	fmt.Printf("[HTTP] Listening on: http://%s\n", address)

	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (r *SecureRoute) RunTLS(host ...string) error {
	address := ""
	if len(host) > 0 {
		address = host[0]
	} else {
		tlsHost := r.config.GetString("http.tls.host")
		tlsPort := r.config.GetString("http.tls.port")
		if tlsPort == "" {
			return errors.New("TLS port can't be empty")
		}
		address = net.JoinHostPort(tlsHost, tlsPort)
	}

	return r.RunTLSWithCert(
		address,
		r.config.GetString("http.tls.ssl.cert"),
		r.config.GetString("http.tls.ssl.key"),
	)
}

func (r *SecureRoute) RunTLSWithCert(
	host string,
	certFile string,
	keyFile string,
) error {
	if strings.TrimSpace(host) == "" {
		return errors.New("host can't be empty")
	}
	if strings.TrimSpace(certFile) == "" || strings.TrimSpace(keyFile) == "" {
		return errors.New("certificate can't be empty")
	}

	server := r.newServer(host)
	r.setServer(server, true)
	fmt.Printf("[HTTPS] Listening on: https://%s\n", host)

	if err := server.ListenAndServeTLS(
		certFile,
		keyFile,
	); !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (r *SecureRoute) Shutdown(contexts ...context.Context) error {
	shutdownContext := context.Background()
	if len(contexts) > 0 {
		shutdownContext = contexts[0]
	}

	r.mu.Lock()
	server := r.server
	tlsServer := r.tlsServer
	r.mu.Unlock()

	var shutdownErrors []error
	if server != nil {
		shutdownErrors = append(
			shutdownErrors,
			server.Shutdown(shutdownContext),
		)
	}
	if tlsServer != nil {
		shutdownErrors = append(
			shutdownErrors,
			tlsServer.Shutdown(shutdownContext),
		)
	}

	return errors.Join(shutdownErrors...)
}

func (r *SecureRoute) newServer(address string) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           http.AllowQuerySemicolons(r),
		ReadHeaderTimeout: r.options.ReadHeaderTimeout,
		ReadTimeout:       r.options.ReadTimeout,
		WriteTimeout:      r.options.WriteTimeout,
		IdleTimeout:       r.options.IdleTimeout,
		MaxHeaderBytes: r.config.GetInt(
			"http.drivers.gin.header_limit",
		) << 10,
	}
}

func (r *SecureRoute) setServer(server *http.Server, tls bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if tls {
		r.tlsServer = server
		return
	}
	r.server = server
}

func (r *SecureRoute) httpAddress(host ...string) (string, error) {
	if len(host) > 0 {
		if strings.TrimSpace(host[0]) == "" {
			return "", errors.New("host can't be empty")
		}
		return host[0], nil
	}

	port := r.config.GetString("http.port")
	if port == "" {
		return "", errors.New("port can't be empty")
	}

	return net.JoinHostPort(r.config.GetString("http.host"), port), nil
}

func withDefaultTimeouts(options Options) Options {
	if options.ReadHeaderTimeout <= 0 {
		options.ReadHeaderTimeout = defaultReadHeaderTimeout
	}
	if options.ReadTimeout <= 0 {
		options.ReadTimeout = defaultReadTimeout
	}
	if options.WriteTimeout <= 0 {
		options.WriteTimeout = defaultWriteTimeout
	}
	if options.IdleTimeout <= 0 {
		options.IdleTimeout = defaultIdleTimeout
	}

	return options
}

func newRequestBoundaryHandler(
	next http.Handler,
	maxBodyBytes int64,
	requestTimeout time.Duration,
) http.Handler {
	apiHandler := next
	if requestTimeout > 0 {
		apiHandler = http.TimeoutHandler(
			next,
			requestTimeout,
			requestTimeoutBody,
		)
	}
	dispatch := routeAPIOnly(next, apiHandler)

	return http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		stripUntrustedProxyHeaders(request.Header)

		if maxBodyBytes <= 0 || request.Body == nil {
			dispatch.ServeHTTP(writer, request)
			return
		}

		gate := &responseGate{ResponseWriter: writer}
		if request.ContentLength > maxBodyBytes {
			gate.rejectPayloadTooLarge()
			return
		}

		if isJSONRequest(request) {
			body, err := readBoundedJSONBody(
				gate,
				request.Body,
				maxBodyBytes,
			)
			if err != nil {
				var maxBytesError *http.MaxBytesError
				if errors.As(err, &maxBytesError) {
					gate.rejectPayloadTooLarge()
				} else {
					gate.rejectMalformedJSON()
				}
				return
			}
			if !isJSONObject(body) {
				gate.rejectMalformedJSON()
				return
			}

			request.Body = io.NopCloser(bytes.NewReader(body))
			request.ContentLength = int64(len(body))
			dispatch.ServeHTTP(gate, request)
			return
		}

		limitedBody := http.MaxBytesReader(
			gate,
			request.Body,
			maxBodyBytes,
		)
		request.Body = &limitDetectingReadCloser{
			ReadCloser: limitedBody,
			onLimit:    gate.rejectPayloadTooLarge,
		}

		dispatch.ServeHTTP(gate, request)
	})
}

func routeAPIOnly(
	defaultHandler http.Handler,
	apiHandler http.Handler,
) http.Handler {
	return http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		if !isAPIPath(request.URL.Path) {
			defaultHandler.ServeHTTP(writer, request)
			return
		}

		setAPIHeaders(writer.Header())
		apiHandler.ServeHTTP(writer, request)
	})
}

func isAPIPath(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/")
}

func setAPIHeaders(header http.Header) {
	header.Set("Cache-Control", "no-store, private")
	header.Set("Content-Type", "application/json; charset=utf-8")
	header.Set("Vary", "X-User-ID")
	header.Set("X-Content-Type-Options", "nosniff")
}

func stripUntrustedProxyHeaders(header http.Header) {
	header.Del("Forwarded")
	header.Del("X-Forwarded-For")
	header.Del("X-Real-IP")
}

func isJSONRequest(request *http.Request) bool {
	contentType := request.Header.Get("Content-Type")
	if flagIndex := strings.IndexAny(contentType, " ;\t\r\n"); flagIndex >= 0 {
		contentType = contentType[:flagIndex]
	}

	return strings.EqualFold(
		strings.TrimSpace(contentType),
		"application/json",
	)
}

func readBoundedJSONBody(
	writer http.ResponseWriter,
	body io.ReadCloser,
	maxBodyBytes int64,
) ([]byte, error) {
	limitedBody := http.MaxBytesReader(writer, body, maxBodyBytes)
	defer limitedBody.Close()

	return io.ReadAll(limitedBody)
}

func isJSONObject(body []byte) bool {
	if len(body) == 0 {
		return true
	}

	var object map[string]any
	return json.Unmarshal(body, &object) == nil && object != nil
}

type responseGate struct {
	http.ResponseWriter
	mu          sync.Mutex
	rejected    bool
	wroteHeader bool
}

func (writer *responseGate) WriteHeader(statusCode int) {
	writer.mu.Lock()
	defer writer.mu.Unlock()

	if writer.rejected || writer.wroteHeader {
		return
	}
	writer.wroteHeader = true
	writer.ResponseWriter.WriteHeader(statusCode)
}

func (writer *responseGate) Write(body []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()

	if writer.rejected {
		return len(body), nil
	}
	writer.wroteHeader = true
	return writer.ResponseWriter.Write(body)
}

func (writer *responseGate) rejectPayloadTooLarge() {
	writer.reject(
		http.StatusRequestEntityTooLarge,
		requestTooLargeBody,
	)
}

func (writer *responseGate) rejectMalformedJSON() {
	writer.reject(http.StatusBadRequest, malformedJSONBody)
}

func (writer *responseGate) reject(statusCode int, body string) {
	writer.mu.Lock()
	defer writer.mu.Unlock()

	if writer.rejected || writer.wroteHeader {
		return
	}

	writer.rejected = true
	writer.wroteHeader = true
	setAPIHeaders(writer.Header())
	writer.ResponseWriter.WriteHeader(statusCode)
	_, _ = io.WriteString(writer.ResponseWriter, body)
}

type limitDetectingReadCloser struct {
	io.ReadCloser
	onLimit func()
}

func (reader *limitDetectingReadCloser) Read(buffer []byte) (int, error) {
	count, err := reader.ReadCloser.Read(buffer)
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		reader.onLimit()
	}

	return count, err
}
