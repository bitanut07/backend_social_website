package config

import (
	"goravel/app/facades"
)

func init() {
	config := facades.Config()
	config.Add("cors", map[string]any{
		"paths":                []string{"api/*"},
		"allowed_methods":      []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		"allowed_origins":      []string{config.EnvString("CORS_ALLOWED_ORIGIN", "http://localhost:5173")},
		"allowed_headers":      []string{"Accept", "Authorization", "Content-Type", "X-User-ID"},
		"exposed_headers":      []string{},
		"max_age":              600,
		"supports_credentials": false,
	})
}
