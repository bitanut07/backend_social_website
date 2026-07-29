package config

import (
	"testing"
	"time"
)

func TestConfiguredHTTPPortPrefersRuntimePort(t *testing.T) {
	t.Parallel()

	if got := configuredHTTPPort("35617", "3000"); got != "35617" {
		t.Fatalf("configuredHTTPPort() = %q, want Vercel runtime port", got)
	}
}

func TestConfiguredHTTPPortFallsBackToApplicationPort(t *testing.T) {
	t.Parallel()

	if got := configuredHTTPPort("", "3005"); got != "3005" {
		t.Fatalf("configuredHTTPPort() = %q, want application port", got)
	}
}

func TestConfiguredHTTPRequestTimeoutDefaultsToSixtySeconds(t *testing.T) {
	t.Parallel()

	if got := configuredHTTPRequestTimeout(""); got != 60*time.Second {
		t.Fatalf("configuredHTTPRequestTimeout() = %s, want 60s", got)
	}
}

func TestConfiguredHTTPRequestTimeoutAcceptsOverride(t *testing.T) {
	t.Parallel()

	if got := configuredHTTPRequestTimeout("25s"); got != 25*time.Second {
		t.Fatalf("configuredHTTPRequestTimeout() = %s, want 25s", got)
	}
}

func TestConfiguredDatabasePortUsesTransactionPoolerOnVercel(t *testing.T) {
	t.Parallel()

	if got := configuredDatabasePort(true, "6543", "5432", "5432"); got != "6543" {
		t.Fatalf("configuredDatabasePort() = %q, want transaction pooler port", got)
	}
}

func TestConfiguredDatabasePortKeepsSessionPoolerLocally(t *testing.T) {
	t.Parallel()

	if got := configuredDatabasePort(false, "6543", "5432", "5433"); got != "5432" {
		t.Fatalf("configuredDatabasePort() = %q, want session pooler port", got)
	}
}

func TestDatabasePoolDefaultsAreConservativeOnVercel(t *testing.T) {
	t.Parallel()

	defaults := configuredDatabasePoolDefaults(true)
	if defaults.maxIdleConnections != 0 {
		t.Fatalf("maxIdleConnections = %d, want 0", defaults.maxIdleConnections)
	}
	if defaults.maxOpenConnections != 5 {
		t.Fatalf("maxOpenConnections = %d, want 5", defaults.maxOpenConnections)
	}
	if defaults.connectionMaxIdleTime != 60 {
		t.Fatalf("connectionMaxIdleTime = %d, want 60", defaults.connectionMaxIdleTime)
	}
	if defaults.connectionMaxLifetime != 300 {
		t.Fatalf("connectionMaxLifetime = %d, want 300", defaults.connectionMaxLifetime)
	}
}
