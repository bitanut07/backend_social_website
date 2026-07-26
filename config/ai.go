package config

import (
	"github.com/goravel/framework/contracts/ai"
	openaifacades "github.com/goravel/openai/facades"
	"goravel/app/facades"
)

func init() {
	config := facades.Config()
	config.Add("ai", map[string]any{
		// Default AI Provider
		//
		// This option controls the default AI provider that will be used.
		"default": config.Env("AI_PROVIDER", "local"),

		// AI Providers
		//
		// Here you may configure each AI provider used by your application.
		// A variety of drivers are available, and each provider may also
		// configure the models available to your application.
		"providers": map[string]any{
			"openai": map[string]any{
				"key": config.Env("OPENAI_API_KEY", ""),
				"models": map[string]any{
					"text": map[string]any{
						"default": config.Env("AI_TEXT_MODEL", "gpt-5.6-luna"),
					},
					"audio": map[string]any{
						"default": "",
					},
					"transcription": map[string]any{
						"default": "",
					},
					"image": map[string]any{
						"default": "",
					},
				},
				"failover": map[string][]string{},
				"url":      config.Env("OPENAI_BASE_URL", ""),
				"via": func() (ai.Provider, error) {
					return openaifacades.OpenAI("openai")
				},
			},
		},
		"model_llm": map[string]any{
			"host":                    config.Env("MODEL_LLM_HOST", ""),
			"ssh_port":                config.Env("MODEL_LLM_SSH_PORT", 22),
			"ssh_user":                config.Env("MODEL_LLM_SSH_USER", ""),
			"ssh_key_path":            config.Env("MODEL_LLM_SSH_KEY_PATH", ""),
			"host_key_sha256":         config.Env("MODEL_LLM_HOST_KEY_SHA256", ""),
			"remote_address":          config.Env("MODEL_LLM_REMOTE_ADDRESS", "127.0.0.1:11434"),
			"model":                   config.Env("MODEL_LLM_MODEL", "qwen3:1.7b"),
			"connect_timeout_seconds": config.Env("MODEL_LLM_CONNECT_TIMEOUT_SECONDS", 8),
			"request_timeout_seconds": config.Env("MODEL_LLM_REQUEST_TIMEOUT_SECONDS", 90),
		},
	})
}
