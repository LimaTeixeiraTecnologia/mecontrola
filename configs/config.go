// Package configs fornece carregamento e validação de configuração via Viper.
// Referência: ADR-009 — Viper v1.21.0 + pasta configs/ + Validate() fail-fast + DSN/SafeDSN.
package configs

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config é a struct raiz de configuração da aplicação.
// Cada grupo é incorporado via mapstructure:",squash" para que as env vars
// sejam mapeadas diretamente sem prefixo de grupo.
type Config struct {
	AppConfig  AppConfig  `mapstructure:",squash"`
	HTTPConfig HTTPConfig `mapstructure:",squash"`
	DBConfig   DBConfig   `mapstructure:",squash"`
	O11yConfig O11yConfig `mapstructure:",squash"`
}

// AppConfig agrupa variáveis de identidade da aplicação.
type AppConfig struct {
	// Environment define o ambiente de execução: local | staging | production.
	Environment string `mapstructure:"ENVIRONMENT"`
	// AppMode define o modo de operação: server | worker | migrate.
	AppMode string `mapstructure:"APP_MODE"`
}

// HTTPConfig agrupa variáveis do servidor HTTP.
type HTTPConfig struct {
	// Port é a porta TCP em que o servidor HTTP escuta (1–65535).
	Port int `mapstructure:"PORT"`
	// ServiceNameAPI é o nome do serviço exposto nos headers e traces.
	ServiceNameAPI string `mapstructure:"SERVICE_NAME_API"`
	// CORSAllowedOrigins é a lista de origens permitidas no CORS, separadas por vírgula.
	CORSAllowedOrigins string `mapstructure:"CORS_ALLOWED_ORIGINS"`
}

// DBConfig agrupa variáveis de conexão com o banco de dados Postgres.
type DBConfig struct {
	Host     string `mapstructure:"DB_HOST"`
	Port     int    `mapstructure:"DB_PORT"`
	User     string `mapstructure:"DB_USER"`
	Password string `mapstructure:"DB_PASSWORD"`
	Name     string `mapstructure:"DB_NAME"`
	SSLMode  string `mapstructure:"DB_SSL_MODE"`

	// Pool tunables.
	MaxConns     int `mapstructure:"DB_MAX_CONNS"`
	MinConns     int `mapstructure:"DB_MIN_CONNS"`
	MaxIdleConns int `mapstructure:"DB_MAX_IDLE_CONNS"`
}

// DSN retorna a connection string completa com senha em texto claro.
// AVISO: DSN() NUNCA deve ser logado diretamente.
// Use SafeDSN() para logs e mensagens de erro.
func (d *DBConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.Name, d.SSLMode,
	)
}

// SafeDSN retorna a connection string com a senha mascarada como "***".
// É o único formato permitido em logs, mensagens de erro e traces.
func (d *DBConfig) SafeDSN() string {
	return fmt.Sprintf(
		"postgres://%s:***@%s:%d/%s?sslmode=%s",
		d.User, d.Host, d.Port, d.Name, d.SSLMode,
	)
}

// O11yConfig agrupa variáveis de observabilidade (OpenTelemetry + logging).
type O11yConfig struct {
	// OTLPEndpoint é o endpoint gRPC do coletor OTLP.
	OTLPEndpoint string `mapstructure:"OTEL_EXPORTER_OTLP_ENDPOINT"`
	// OTLPHeaders são os headers de autenticação para o coletor OTLP (ex.: Grafana Cloud).
	OTLPHeaders string `mapstructure:"OTEL_EXPORTER_OTLP_HEADERS"`
	// TraceSampleRate define a fração de traces amostrados [0.0..1.0].
	TraceSampleRate float64 `mapstructure:"OTEL_TRACE_SAMPLE_RATE"`
	// LogLevel define o nível mínimo de log: debug | info | warn | error.
	LogLevel string `mapstructure:"LOG_LEVEL"`
	// LogFormat define o formato do log: json | text.
	LogFormat string `mapstructure:"LOG_FORMAT"`
	// ServiceVersion é a versão semântica do serviço injetada nos traces e logs.
	ServiceVersion string `mapstructure:"SERVICE_VERSION"`
}

// LoadConfig carrega a configuração a partir do arquivo .env no diretório path
// e das variáveis de ambiente do processo.
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

	// Defaults seguros para uso local.
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

	// Valida Environment.
	switch c.AppConfig.Environment {
	case "local", "staging", "production":
		// válido
	default:
		errs = append(errs, fmt.Sprintf(
			"ENVIRONMENT inválido %q: deve ser um de {local, staging, production}",
			c.AppConfig.Environment,
		))
	}

	// Valida Port.
	if c.HTTPConfig.Port < 1 || c.HTTPConfig.Port > 65535 {
		errs = append(errs, fmt.Sprintf(
			"PORT inválido %d: deve estar no intervalo [1..65535]",
			c.HTTPConfig.Port,
		))
	}

	// Valida TraceSampleRate.
	if c.O11yConfig.TraceSampleRate < 0 || c.O11yConfig.TraceSampleRate > 1 {
		errs = append(errs, fmt.Sprintf(
			"OTEL_TRACE_SAMPLE_RATE inválido %.4f: deve estar no intervalo [0..1]",
			c.O11yConfig.TraceSampleRate,
		))
	}

	// Validações específicas de production.
	if c.AppConfig.Environment == "production" {
		// Senha DB < 16 chars.
		if len(c.DBConfig.Password) < 16 {
			errs = append(errs, "DB_PASSWORD deve ter ao menos 16 caracteres em production")
		}

		// Placeholders inseguros conhecidos na senha.
		for _, placeholder := range InsecurePlaceholders {
			if c.DBConfig.Password == placeholder {
				errs = append(errs, fmt.Sprintf(
					"DB_PASSWORD contém placeholder inseguro %q: substitua por valor real em production",
					placeholder,
				))
				break
			}
		}

		// Valida ausência de placeholders inseguros no usuário DB.
		for _, placeholder := range InsecurePlaceholders {
			if c.DBConfig.User == placeholder {
				errs = append(errs, fmt.Sprintf(
					"DB_USER contém placeholder inseguro %q em production",
					placeholder,
				))
				break
			}
		}

		// Valida ausência de placeholders inseguros nos headers OTLP (credenciais Grafana Cloud).
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
