package config

import (
	"strings"
	"time"

	"github.com/goravel/framework/contracts/route"
	ginfacades "github.com/goravel/gin/facades"
	"goravel/app/facades"
	httpdriver "goravel/app/http/driver"
)

func init() {
	config := facades.Config()
	port := configuredHTTPPort(
		config.EnvString("PORT", ""),
		config.EnvString("APP_PORT", "3000"),
	)
	requestTimeout := configuredHTTPRequestTimeout(
		config.EnvString("HTTP_REQUEST_TIMEOUT", ""),
	)
	assistantRequestTimeout := configuredHTTPDuration(
		config.EnvString("HTTP_ASSISTANT_REQUEST_TIMEOUT", "110s"),
		110*time.Second,
	)
	writeTimeout := configuredHTTPDuration(
		config.EnvString("HTTP_WRITE_TIMEOUT", "115s"),
		115*time.Second,
	)
	config.Add("http", map[string]any{
		"default": "gin",
		// HTTP Drivers
		"drivers": map[string]any{
			"gin": map[string]any{
				// Goravel Gin interprets both values as KiB.
				"body_limit":   64,
				"header_limit": 32,
				"route": func() (route.Route, error) {
					return httpdriver.NewSecureRoute(
						ginfacades.Route("gin"),
						config,
						httpdriver.Options{
							MaxBodyBytes:            64 * 1024,
							RequestTimeout:          requestTimeout,
							AssistantRequestTimeout: assistantRequestTimeout,
							WriteTimeout:            writeTimeout,
						},
					), nil
				},
			},
		},
		// HTTP URL
		"url": config.Env("APP_URL", "http://localhost"),
		// HTTP Host
		"host": config.Env("APP_HOST", "127.0.0.1"),
		// HTTP Port
		"port": port,
		// Timeout is enforced by SecureRoute before Goravel materializes the body.
		"request_timeout": 0,
		// HTTPS Configuration
		"tls": map[string]any{
			// HTTPS Host
			"host": config.Env("APP_HOST", "127.0.0.1"),
			// HTTPS Port
			"port": port,
			// SSL Certificate, you can put the certificate in /public folder
			"ssl": map[string]any{
				// ca.pem
				"cert": "",
				// ca.key
				"key": "",
			},
		},
		// Default Client Name
		//
		// This determines which client is used when you call facades.Http() or
		// facades.Http().Client() without passing a specific name.
		"default_client": config.Env("HTTP_CLIENT_DEFAULT", "default"),
		// Client Configurations
		//
		// Here you may define multiple independent client configurations.
		// For example, you might have a "github" client with a specific base URL
		// and a "stripe" client with a longer timeout.
		"clients": map[string]any{
			"default": map[string]any{
				// The base URL for the client. All requests made using this client
				// will be relative to this URL.
				"base_url": config.Env("HTTP_CLIENT_BASE_URL", ""),
				// The maximum amount of time a request can take, including connection
				// establishment, redirects, and reading the response body.
				"timeout": config.Env("HTTP_CLIENT_TIMEOUT", "30s"),
				// The maximum number of idle (keep-alive) connections to keep across
				// ALL hosts. Increasing this helps reuse TCP connections.
				"max_idle_conns": config.Env("HTTP_CLIENT_MAX_IDLE_CONNS", 100),
				// The maximum number of idle (keep-alive) connections to keep PER host.
				// By default, Go sets this to 2, which is often a bottleneck.
				// Increase this value for high-throughput applications.
				"max_idle_conns_per_host": config.Env("HTTP_CLIENT_MAX_IDLE_CONNS_PER_HOST", 2),
				// The maximum total number of connections (active + idle) allowed per host.
				// A value of 0 means no limit.
				"max_conns_per_host": config.Env("HTTP_CLIENT_MAX_CONN_PER_HOST", 0),
				// The maximum amount of time an idle (keep-alive) connection will remain
				// in the pool before closing itself.
				"idle_conn_timeout": config.Env("HTTP_CLIENT_IDLE_CONN_TIMEOUT", "90s"),
			},
		},
	})
}

func configuredHTTPPort(runtimePort string, applicationPort string) string {
	if port := strings.TrimSpace(runtimePort); port != "" {
		return port
	}
	if port := strings.TrimSpace(applicationPort); port != "" {
		return port
	}

	return "3000"
}

func configuredHTTPRequestTimeout(value string) time.Duration {
	return configuredHTTPDuration(value, 60*time.Second)
}

func configuredHTTPDuration(value string, fallback time.Duration) time.Duration {
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return fallback
	}

	return duration
}
