package configs

// InsecurePlaceholders lista os placeholders inseguros conhecidos que o Validate()
// rejeita quando ENVIRONMENT=production.
//
// Qualquer campo de configuração que contenha um destes valores em production
// aborta o bootstrap da aplicação (fail-fast).
var InsecurePlaceholders = []string{
	"CHANGE_ME_USE_STRONG_PASSWORD",
	"CHANGE_ME_GENERATE_SECURE_SECRET_KEY_MIN_64_CHARS",
	"your_secret_key",
	"financial@password",
}
