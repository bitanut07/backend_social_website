package config

import "goravel/app/facades"

func init() {
	config := facades.Config()
	config.Add("supabase", map[string]any{
		"url":                    config.Env("SUPABASE_URL", ""),
		"publishable_key":        config.Env("SUPABASE_PUBLISHABLE_KEY", ""),
		"secret_key":             config.Env("SUPABASE_SECRET_KEY", ""),
		"jwks_url":               config.Env("SUPABASE_JWKS_URL", ""),
		"allow_demo_user_header": config.Env("ALLOW_DEMO_USER_HEADER", false),
	})
}
