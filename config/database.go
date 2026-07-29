package config

import (
	"strings"

	"github.com/goravel/framework/contracts/database/driver"
	postgresfacades "github.com/goravel/postgres/facades"

	"goravel/app/facades"
)

func init() {
	config := facades.Config()
	isVercel := strings.TrimSpace(config.EnvString("VERCEL", "")) == "1"
	poolDefaults := configuredDatabasePoolDefaults(isVercel)
	config.Add("database", map[string]any{
		// Default database connection name
		"default": config.Env("DB_CONNECTION", "postgres"),
		// Database connections
		"connections": map[string]any{
			"postgres": map[string]any{
				"host": config.Env(
					"SUPABASE_POOLER_HOST",
					config.Env("DB_HOST", "127.0.0.1"),
				),
				"port": configuredDatabasePort(
					isVercel,
					config.EnvString("SUPABASE_TRANSACTION_POOLER_PORT", ""),
					config.EnvString("SUPABASE_SESSION_POOLER_PORT", ""),
					config.EnvString("DB_PORT", "5432"),
				),
				"database": config.Env(
					"SUPABASE_DB_NAME",
					config.Env("DB_DATABASE", "artly_social"),
				),
				"username": config.Env(
					"SUPABASE_POOLER_USER",
					config.Env("DB_USERNAME", "postgres"),
				),
				"password": config.Env(
					"SUPABASE_DB_PASSWORD",
					config.Env("DB_PASSWORD"),
				),
				"sslmode": config.Env(
					"DB_SSLMODE",
					"require",
				),
				"schema":   config.Env("DB_SCHEMA", "public"),
				"timezone": config.Env("DB_TIMEZONE", "UTC"),
				"singular": false,
				"prefix":   "",
				"via": func() (driver.Driver, error) {
					return postgresfacades.Postgres("postgres")
				},
			},
		},
		// Pool configuration
		"pool": map[string]any{
			// Sets the maximum number of connections in the idle
			// connection pool.
			//
			// If MaxOpenConns is greater than 0 but less than the new MaxIdleConns,
			// then the new MaxIdleConns will be reduced to match the MaxOpenConns limit.
			//
			// If n <= 0, no idle connections are retained.
			"max_idle_conns": config.Env(
				"DB_MAX_IDLE_CONNS",
				poolDefaults.maxIdleConnections,
			),
			// Sets the maximum number of open connections to the database.
			//
			// If MaxIdleConns is greater than 0 and the new MaxOpenConns is less than
			// MaxIdleConns, then MaxIdleConns will be reduced to match the new
			// MaxOpenConns limit.
			//
			// If n <= 0, then there is no limit on the number of open connections.
			"max_open_conns": config.Env(
				"DB_MAX_OPEN_CONNS",
				poolDefaults.maxOpenConnections,
			),
			// Sets the maximum amount of time a connection may be idle.
			//
			// Expired connections may be closed lazily before reuse.
			//
			// If d <= 0, connections are not closed due to a connection's idle time.
			// Unit: Second
			"conn_max_idletime": config.Env(
				"DB_CONN_MAX_IDLETIME",
				poolDefaults.connectionMaxIdleTime,
			),
			// Sets the maximum amount of time a connection may be reused.
			//
			// Expired connections may be closed lazily before reuse.
			//
			// If d <= 0, connections are not closed due to a connection's age.
			// Unit: Second
			"conn_max_lifetime": config.Env(
				"DB_CONN_MAX_LIFETIME",
				poolDefaults.connectionMaxLifetime,
			),
		},
		// Sets the threshold for slow queries in milliseconds, the slow query will be logged.
		// Unit: Millisecond
		"slow_threshold": 200,
		// Migration Repository Table
		//
		// This table keeps track of all the migrations that have already run for
		// your application. Using this information, we can determine which of
		// the migrations on disk haven't actually been run in the database.
		"migrations": map[string]any{
			// A dedicated name avoids colliding with migration ledgers in other
			// schemas managed by Supabase extensions.
			"table": "artly_goravel_migrations",
		},
	})
}

type databasePoolDefaults struct {
	maxIdleConnections    int
	maxOpenConnections    int
	connectionMaxIdleTime int
	connectionMaxLifetime int
}

func configuredDatabasePort(
	isVercel bool,
	transactionPoolerPort string,
	sessionPoolerPort string,
	databasePort string,
) string {
	if isVercel {
		if port := strings.TrimSpace(transactionPoolerPort); port != "" {
			return port
		}
	}
	if port := strings.TrimSpace(sessionPoolerPort); port != "" {
		return port
	}
	if port := strings.TrimSpace(databasePort); port != "" {
		return port
	}

	return "5432"
}

func configuredDatabasePoolDefaults(isVercel bool) databasePoolDefaults {
	if isVercel {
		return databasePoolDefaults{
			maxIdleConnections:    0,
			maxOpenConnections:    5,
			connectionMaxIdleTime: 60,
			connectionMaxLifetime: 300,
		}
	}

	return databasePoolDefaults{
		maxIdleConnections:    10,
		maxOpenConnections:    100,
		connectionMaxIdleTime: 3600,
		connectionMaxLifetime: 3600,
	}
}
