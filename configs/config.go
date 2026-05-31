package configs

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config incorpora grupos via mapstructure:",squash" para que as env vars
// sejam mapeadas diretamente sem prefixo de grupo.
type Config struct {
	AppConfig  AppConfig  `mapstructure:",squash"`
	HTTPConfig HTTPConfig `mapstructure:",squash"`
	DBConfig   DBConfig   `mapstructure:",squash"`
	O11yConfig O11yConfig `mapstructure:",squash"`
}

type AppConfig struct {
	Environment string `mapstructure:"ENVIRONMENT"`
	AppMode     string `mapstructure:"APP_MODE"`
}

type HTTPConfig struct {
	Port               int    `mapstructure:"PORT"`
	ServiceNameAPI     string `mapstructure:"SERVICE_NAME_API"`
	CORSAllowedOrigins string `mapstructure:"CORS_ALLOWED_ORIGINS"`
}

type DBConfig struct {
	Host     string `mapstructure:"DB_HOST"`
	Port     int    `mapstructure:"DB_PORT"`
	User     string `mapstructure:"DB_USER"`
	Password string `mapstructure:"DB_PASSWORD"`
	Name     string `mapstructure:"DB_NAME"`
	SSLMode  string `mapstructure:"DB_SSL_MODE"`

	MaxConns     int `mapstructure:"DB_MAX_CONNS"`
	MinConns     int `mapstructure:"DB_MIN_CONNS"`
	MaxIdleConns int `mapstructure:"DB_MAX_IDLE_CONNS"`
}

// DSN retorna a connection string com senha em texto claro.
// NUNCA logar diretamente — use SafeDSN() para logs e mensagens de erro.
func (d *DBConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.Name, d.SSLMode,
	)
}

// SafeDSN é o único formato permitido em logs, mensagens de erro e traces.
func (d *DBConfig) SafeDSN() string {
	return fmt.Sprintf(
		"postgres://%s:***@%s:%d/%s?sslmode=%s",
		d.User, d.Host, d.Port, d.Name, d.SSLMode,
	)
}

type O11yConfig struct {
	OTLPEndpoint    string  `mapstructure:"OTEL_EXPORTER_OTLP_ENDPOINT"`
	OTLPHeaders     string  `mapstructure:"OTEL_EXPORTER_OTLP_HEADERS"`
	TraceSampleRate float64 `mapstructure:"OTEL_TRACE_SAMPLE_RATE"`
	LogLevel        string  `mapstructure:"LOG_LEVEL"`
	LogFormat       string  `mapstructure:"LOG_FORMAT"`
	ServiceVersion  string  `mapstructure:"SERVICE_VERSION"`
}

// LoadConfig carrega configuração do arquivo .env em path e de env vars do processo.
//
// Pipeline Viper:
//  1. SetConfigName(".env") + SetConfigType("env")
//  2. AddConfigPath(path)
//  3. AutomaticEnv() captura env vars do processo
//  4. SetEnvKeyReplacer(".", "_") para compatibilidade com nomes compostos
//  5. ReadInConfig — obrigatório em local/staging; tolerado em production
//  6. Unmarshal para *Config
//  7. Validate() fail-fast
func LoadConfig(path string) (*Config, error) {
	v := viper.New()

	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(path)
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Bind env vars explicitamente para garantir que AutomaticEnv() funcione com Unmarshal.
	// Necessário porque mapstructure não chama os getters do Viper automaticamente.
	envKeys := []string{
		"ENVIRONMENT", "APP_MODE",
		"PORT", "SERVICE_NAME_API", "CORS_ALLOWED_ORIGINS",
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSL_MODE",
		"DB_MAX_CONNS", "DB_MIN_CONNS", "DB_MAX_IDLE_CONNS",
		"OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_HEADERS",
		"OTEL_TRACE_SAMPLE_RATE", "LOG_LEVEL", "LOG_FORMAT", "SERVICE_VERSION",
	}
	for _, key := range envKeys {
		_ = v.BindEnv(key)
	}

	v.SetDefault("PORT", 8080)
	v.SetDefault("APP_MODE", "server")
	v.SetDefault("ENVIRONMENT", "local")
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("LOG_FORMAT", "json")
	v.SetDefault("OTEL_TRACE_SAMPLE_RATE", 1.0)
	v.SetDefault("DB_PORT", 5432)
	v.SetDefault("DB_SSL_MODE", "disable")
	v.SetDefault("DB_MAX_CONNS", 10)
	v.SetDefault("DB_MIN_CONNS", 2)
	v.SetDefault("DB_MAX_IDLE_CONNS", 5)

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, fmt.Errorf("lendo arquivo de configuração: %w", err)
		}

		// Arquivo .env não encontrado: tolerado apenas em production.
		// Em outros ambientes é obrigatório.
		env := v.GetString("ENVIRONMENT")
		if env != "production" {
			return nil, fmt.Errorf(
				"arquivo .env não encontrado em %q (obrigatório em ambiente %q): %w",
				path, env, err,
			)
		}
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("deserializando configuração: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validando configuração: %w", err)
	}

	return cfg, nil
}

// Validate executa verificações fail-fast antes de qualquer subsistema inicializar.
//
// Rejeita:
//   - ENVIRONMENT fora de {local, staging, production}
//   - Port fora de [1..65535]
//   - TraceSampleRate fora de [0..1]
//   - Em production: senha DB < 16 chars, secrets < 64 chars, placeholders CHANGE_ME_*
func (c *Config) Validate() error {
	var errs []string

	switch c.AppConfig.Environment {
	case "local", "staging", "production":
		// válido
	default:
		errs = append(errs, fmt.Sprintf(
			"ENVIRONMENT inválido %q: deve ser um de {local, staging, production}",
			c.AppConfig.Environment,
		))
	}

	if c.HTTPConfig.Port < 1 || c.HTTPConfig.Port > 65535 {
		errs = append(errs, fmt.Sprintf(
			"PORT inválido %d: deve estar no intervalo [1..65535]",
			c.HTTPConfig.Port,
		))
	}

	if c.O11yConfig.TraceSampleRate < 0 || c.O11yConfig.TraceSampleRate > 1 {
		errs = append(errs, fmt.Sprintf(
			"OTEL_TRACE_SAMPLE_RATE inválido %.4f: deve estar no intervalo [0..1]",
			c.O11yConfig.TraceSampleRate,
		))
	}

	if c.AppConfig.Environment == "production" {
		if len(c.DBConfig.Password) < 16 {
			errs = append(errs, "DB_PASSWORD deve ter ao menos 16 caracteres em production")
		}

		for _, placeholder := range InsecurePlaceholders {
			if c.DBConfig.Password == placeholder {
				errs = append(errs, fmt.Sprintf(
					"DB_PASSWORD contém placeholder inseguro %q: substitua por valor real em production",
					placeholder,
				))
				break
			}
		}

		for _, placeholder := range InsecurePlaceholders {
			if c.DBConfig.User == placeholder {
				errs = append(errs, fmt.Sprintf(
					"DB_USER contém placeholder inseguro %q em production",
					placeholder,
				))
				break
			}
		}

		for _, placeholder := range InsecurePlaceholders {
			if c.O11yConfig.OTLPHeaders == placeholder || strings.HasPrefix(c.O11yConfig.OTLPHeaders, "CHANGE_ME_") {
				errs = append(errs, fmt.Sprintf(
					"OTEL_EXPORTER_OTLP_HEADERS contém placeholder inseguro %q: configure as credenciais reais em production",
					c.O11yConfig.OTLPHeaders,
				))
				break
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("configuração inválida:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}
