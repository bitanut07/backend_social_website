package httpdriver

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRequestBoundaryRejectsKnownOversizedBodyBeforeHandler(t *testing.T) {
	var called atomic.Bool
	handler := newRequestBoundaryHandler(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			called.Store(true)
		}),
		32,
		time.Second,
	)
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/messages",
		strings.NewReader(strings.Repeat("a", 33)),
	)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	require.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
	require.False(t, called.Load())
	requireErrorCode(t, response.Body.Bytes(), "PAYLOAD_TOO_LARGE")
	require.Equal(t, "no-store, private", response.Header().Get("Cache-Control"))
}

func TestRequestBoundaryCapsChunkedBodyBeforeGoravelCanReadItAll(
	t *testing.T,
) {
	const maximumBytes = int64(64)
	source := &countingReader{
		reader: bytes.NewReader(bytes.Repeat([]byte("x"), 1024)),
	}
	handler := newRequestBoundaryHandler(
		http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			_, _ = io.ReadAll(request.Body)
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte(`{"error":{"code":"BAD_REQUEST"}}`))
		}),
		maximumBytes,
		time.Second,
	)
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/messages",
		source,
	)
	request.ContentLength = -1
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	require.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
	requireErrorCode(t, response.Body.Bytes(), "PAYLOAD_TOO_LARGE")
	require.LessOrEqual(t, source.bytesRead.Load(), maximumBytes+1)
}

func TestRequestBoundaryAllowsBodyWithinLimit(t *testing.T) {
	handler := newRequestBoundaryHandler(
		http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			body, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			require.Equal(t, `{"body":"xin chào"}`, string(body))
			writer.WriteHeader(http.StatusCreated)
			_, _ = writer.Write([]byte(`{"data":{"id":"20000000-0000-4000-8000-000000000001"}}`))
		}),
		1024,
		time.Second,
	)
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/messages",
		strings.NewReader(`{"body":"xin chào"}`),
	)
	request.Header.Set("Content-Type", "application/json;not-a-parameter")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	require.Equal(t, http.StatusCreated, response.Code)
	require.JSONEq(
		t,
		`{"data":{"id":"20000000-0000-4000-8000-000000000001"}}`,
		response.Body.String(),
	)
}

func TestRequestBoundaryReturnsJSONTimeoutContract(t *testing.T) {
	handler := newRequestBoundaryHandler(
		http.HandlerFunc(func(
			_ http.ResponseWriter,
			request *http.Request,
		) {
			<-request.Context().Done()
		}),
		1024,
		10*time.Millisecond,
	)
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/posts",
		nil,
	)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	require.Equal(t, http.StatusServiceUnavailable, response.Code)
	requireErrorCode(t, response.Body.Bytes(), "REQUEST_TIMEOUT")
	require.Equal(
		t,
		"application/json; charset=utf-8",
		response.Header().Get("Content-Type"),
	)
	require.Equal(t, "nosniff", response.Header().Get("X-Content-Type-Options"))
}

func TestRequestBoundaryDoesNotAffectNonAPIRoutes(t *testing.T) {
	handler := newRequestBoundaryHandler(
		http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "text/html")
			_, _ = writer.Write([]byte("<h1>Artly</h1>"))
		}),
		1,
		time.Nanosecond,
	)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "text/html", response.Header().Get("Content-Type"))
	require.Equal(t, "<h1>Artly</h1>", response.Body.String())
}

func TestRequestBoundaryCapsBodiesOnEveryPath(t *testing.T) {
	for _, path := range []string{"/", "/unknown", "//api/v1/messages"} {
		t.Run(path, func(t *testing.T) {
			var called atomic.Bool
			source := &countingReader{
				reader: bytes.NewReader(bytes.Repeat([]byte("x"), 1024)),
			}
			handler := newRequestBoundaryHandler(
				http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
					called.Store(true)
				}),
				64,
				time.Second,
			)
			request := httptest.NewRequest(http.MethodPost, path, source)
			request.ContentLength = -1
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			require.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
			require.False(t, called.Load())
			requireErrorCode(t, response.Body.Bytes(), "PAYLOAD_TOO_LARGE")
			require.LessOrEqual(t, source.bytesRead.Load(), int64(65))
		})
	}
}

func TestRequestBoundaryRejectsMalformedJSONBeforeDownstream(t *testing.T) {
	testCases := map[string]string{
		"syntax error":    `{"body":"student@example.com"`,
		"number overflow": `{"body":"student@example.com","n":1e10000}`,
	}

	for name, body := range testCases {
		t.Run(name, func(t *testing.T) {
			var called atomic.Bool
			handler := newRequestBoundaryHandler(
				http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
					called.Store(true)
				}),
				1024,
				time.Second,
			)
			request := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/messages",
				strings.NewReader(body),
			)
			request.Header.Set(
				"Content-Type",
				"application/json;not-a-parameter",
			)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			require.Equal(t, http.StatusBadRequest, response.Code)
			require.False(t, called.Load())
			requireErrorCode(t, response.Body.Bytes(), "BAD_REQUEST")
			require.NotContains(
				t,
				response.Body.String(),
				"student@example.com",
			)
		})
	}
}

func TestRequestBoundaryDropsUntrustedProxyHeaders(t *testing.T) {
	var forwardedFor string
	var realIP string
	var forwarded string
	handler := newRequestBoundaryHandler(
		http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			forwardedFor = request.Header.Get("X-Forwarded-For")
			realIP = request.Header.Get("X-Real-IP")
			forwarded = request.Header.Get("Forwarded")
			writer.WriteHeader(http.StatusNoContent)
		}),
		1024,
		time.Second,
	)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Forwarded-For", "203.0.113.10")
	request.Header.Set("X-Real-IP", "203.0.113.11")
	request.Header.Set("Forwarded", "for=203.0.113.12")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code)
	require.Empty(t, forwardedFor)
	require.Empty(t, realIP)
	require.Empty(t, forwarded)
}

func requireErrorCode(t *testing.T, body []byte, expectedCode string) {
	t.Helper()

	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(body, &payload))
	require.Equal(t, expectedCode, payload.Error.Code)
}

type countingReader struct {
	reader    io.Reader
	bytesRead atomic.Int64
}

func (reader *countingReader) Read(buffer []byte) (int, error) {
	count, err := reader.reader.Read(buffer)
	reader.bytesRead.Add(int64(count))
	return count, err
}
