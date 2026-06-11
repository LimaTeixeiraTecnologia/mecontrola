package configs

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
)

type Config struct {
	AppConfig        AppConfig        `mapstructure:",squash"`
	HTTPConfig       HTTPConfig       `mapstructure:",squash"`
	DBConfig         DBConfig         `mapstructure:",squash"`
	O11yConfig       O11yConfig       `mapstructure:",squash"`
	OutboxConfig     OutboxConfig     `mapstructure:",squash"`
	KiwifyConfig     KiwifyConfig     `mapstructure:",squash"`
	BillingConfig    BillingConfig    `mapstructure:",squash"`
	OnboardingConfig OnboardingConfig `mapstructure:",squash"`
	WhatsAppConfig   WhatsAppConfig   `mapstructure:",squash"`
	IdentityConfig   IdentityConfig   `mapstructure:",squash"`
	BudgetsConfig    BudgetsConfig    `mapstructure:",squash"`
}

type IdentityConfig struct {
	AuthEventsHousekeepingSchedule string `mapstructure:"IDENTITY_AUTH_EVENTS_HOUSEKEEPING_SCHEDULE"`
	AuthEventsHousekeepingBatch    int    `mapstructure:"IDENTITY_AUTH_EVENTS_HOUSEKEEPING_BATCH"`
	AuthEventsRetentionDays        int    `mapstructure:"IDENTITY_AUTH_EVENTS_RETENTION_DAYS"`
}

type BudgetsConfig struct {
	PendingReaperInterval   string        `mapstructure:"BUDGETS_PENDING_REAPER_INTERVAL"`
	PendingTTLHours         int           `mapstructure:"BUDGETS_PENDING_TTL_HOURS"`
	PendingTTL              time.Duration `mapstructure:"-"`
	AbandonedDraftCron      string        `mapstructure:"BUDGETS_ABANDONED_DRAFT_CRON"`
	RetentionPurgeCron      string        `mapstructure:"BUDGETS_RETENTION_PURGE_CRON"`
	RetentionPurgeBatchSize int           `mapstructure:"BUDGETS_RETENTION_PURGE_BATCH_SIZE"`
}

type OnboardingConfig struct {
	TokenTTLDays            int    `mapstructure:"ONBOARDING_TOKEN_TTL_DAYS"`
	OutreachGapHours        int    `mapstructure:"ONBOARDING_OUTREACH_GAP_HOURS"`
	OutreachEnabled         bool   `mapstructure:"ONBOARDING_OUTREACH_ENABLED"`
	CheckoutCORSOrigins     string `mapstructure:"ONBOARDING_CHECKOUT_CORS_ORIGINS"`
	TrustedProxies          string `mapstructure:"ONBOARDING_TRUSTED_PROXIES"`
	CheckoutRateLimitPerMin int    `mapstructure:"ONBOARDING_CHECKOUT_RATE_LIMIT_PER_MIN"`
	CheckoutRateLimitBurst  int    `mapstructure:"ONBOARDING_CHECKOUT_RATE_LIMIT_BURST"`
	StateRateLimitPerMin    int    `mapstructure:"ONBOARDING_STATE_RATE_LIMIT_PER_MIN"`
	StateRateLimitBurst     int    `mapstructure:"ONBOARDING_STATE_RATE_LIMIT_BURST"`
	KiwifyCheckoutURLs      string `mapstructure:"ONBOARDING_KIWIFY_CHECKOUT_URLS"`
	KiwifyAllowedHosts      string `mapstructure:"ONBOARDING_KIWIFY_ALLOWED_HOSTS"`
	MetaRetentionDays       int    `mapstructure:"ONBOARDING_META_RETENTION_DAYS"`
	MetaCleanupSchedule     string `mapstructure:"ONBOARDING_META_CLEANUP_SCHEDULE"`
	TokenExpirationSchedule string `mapstructure:"ONBOARDING_TOKEN_EXPIRATION_SCHEDULE"`
	MaxTokenLookupAttempts  int    `mapstructure:"ONBOARDING_MAX_TOKEN_LOOKUP_ATTEMPTS"`
	TokenEncryptionKey      string `mapstructure:"ONBOARDING_TOKEN_ENCRYPTION_KEY"`
}

type WhatsAppConfig struct {
	PhoneNumberID        string `mapstructure:"META_PHONE_NUMBER_ID"`
	AccessToken          string `mapstructure:"META_ACCESS_TOKEN"`
	AppSecret            string `mapstructure:"META_APP_SECRET"`
	AppSecretNext        string `mapstructure:"META_APP_SECRET_NEXT"`
	VerifyToken          string `mapstructure:"META_VERIFY_TOKEN"`
	OutreachTemplateName string `mapstructure:"META_OUTREACH_TEMPLATE_NAME"`
	BotNumberE164        string `mapstructure:"META_BOT_NUMBER_E164"`
	BotNumberDisplay     string `mapstructure:"META_BOT_NUMBER_DISPLAY"`
	WelcomeActivated     string `mapstructure:"WA_MSG_WELCOME_ACTIVATED"`
	AlreadyActive        string `mapstructure:"WA_MSG_ALREADY_ACTIVE"`
	CodeAlreadyUsed      string `mapstructure:"WA_MSG_CODE_ALREADY_USED_OTHER_ACCOUNT"`
	PaymentProcessing    string `mapstructure:"WA_MSG_PAYMENT_STILL_PROCESSING_RETRY"`
	CodeExpired          string `mapstructure:"WA_MSG_CODE_EXPIRED_CONTACT_SUPPORT"`
	CodeInvalid          string `mapstructure:"WA_MSG_CODE_INVALID_CHECK_AGAIN"`
	SystemUnavailable    string `mapstructure:"WA_MSG_SYSTEM_UNAVAILABLE_RETRY"`
	PleaseUseAtivar      string `mapstructure:"WA_MSG_PLEASE_USE_ATIVAR_COMMAND"`
	InvalidCountry       string `mapstructure:"WA_MSG_INVALID_COUNTRY"`
	AgentStubReceived    string `mapstructure:"WA_MSG_AGENT_STUB_RECEIVED"`
}

type KiwifyConfig struct {
	APIBaseURL                 string        `mapstructure:"KIWIFY_API_BASE_URL"`
	AccountID                  string        `mapstructure:"KIWIFY_ACCOUNT_ID"`
	ProductIDMonthly           string        `mapstructure:"KIWIFY_PRODUCT_ID_MONTHLY"`
	ProductIDQuarterly         string        `mapstructure:"KIWIFY_PRODUCT_ID_QUARTERLY"`
	ProductIDAnnual            string        `mapstructure:"KIWIFY_PRODUCT_ID_ANNUAL"`
	WebhookSecret              string        `mapstructure:"KIWIFY_WEBHOOK_SECRET"`
	WebhookSecretNext          string        `mapstructure:"KIWIFY_WEBHOOK_SECRET_NEXT"`
	WebhookTokenHeader         string        `mapstructure:"KIWIFY_WEBHOOK_TOKEN_HEADER"`
	ClientID                   string        `mapstructure:"KIWIFY_CLIENT_ID"`
	ClientSecret               string        `mapstructure:"KIWIFY_CLIENT_SECRET"`
	OAuthTokenSafetyMargin     time.Duration `mapstructure:"KIWIFY_OAUTH_TOKEN_SAFETY_MARGIN"`
	RateLimitMaxRequestsPerMin int           `mapstructure:"KIWIFY_RATE_LIMIT_MAX_REQUESTS_PER_MIN"`
	RateLimitBurst             int           `mapstructure:"KIWIFY_RATE_LIMIT_BURST"`
	ReconciliationInterval     string        `mapstructure:"KIWIFY_RECONCILIATION_INTERVAL"`
	ReconciliationBatchSize    int           `mapstructure:"KIWIFY_RECONCILIATION_BATCH_SIZE"`
	HTTPTimeout                time.Duration `mapstructure:"KIWIFY_HTTP_TIMEOUT"`
	HTTPRetryMaxAttempts       int           `mapstructure:"KIWIFY_HTTP_RETRY_MAX_ATTEMPTS"`
	HTTPRetryBackoff           time.Duration `mapstructure:"KIWIFY_HTTP_RETRY_BACKOFF"`
}

func (k KiwifyConfig) Safe() map[string]any {
	return map[string]any{
		"api_base_url":              k.APIBaseURL,
		"account_id_set":            k.AccountID != "",
		"product_id_monthly_set":    k.ProductIDMonthly != "",
		"product_id_quarterly_set":  k.ProductIDQuarterly != "",
		"product_id_annual_set":     k.ProductIDAnnual != "",
		"webhook_token_header":      k.WebhookTokenHeader,
		"rate_limit":                k.RateLimitMaxRequestsPerMin,
		"reconciliation_interval":   k.ReconciliationInterval,
		"reconciliation_batch_size": k.ReconciliationBatchSize,
		"http_timeout":              k.HTTPTimeout.String(),
		"http_retry_max_attempts":   k.HTTPRetryMaxAttempts,
		"http_retry_backoff":        k.HTTPRetryBackoff.String(),
		"client_id_set":             k.ClientID != "",
		"client_secret_set":         k.ClientSecret != "",
		"webhook_secret_set":        k.WebhookSecret != "",
	}
}

type BillingConfig struct {
	EntitlementCacheCapacity         int           `mapstructure:"BILLING_ENTITLEMENT_CACHE_CAPACITY"`
	EntitlementCacheTTL              time.Duration `mapstructure:"BILLING_ENTITLEMENT_CACHE_TTL"`
	AnonymizationSchedule            string        `mapstructure:"BILLING_ANONYMIZATION_SCHEDULE"`
	AnonymizationBatchSize           int           `mapstructure:"BILLING_ANONYMIZATION_BATCH_SIZE"`
	AnonymizationRetentionDays       int           `mapstructure:"BILLING_ANONYMIZATION_RETENTION_DAYS"`
	KiwifyEventsRetentionDays        int           `mapstructure:"BILLING_KIWIFY_EVENTS_RETENTION_DAYS"`
	KiwifyEventsHousekeepingSchedule string        `mapstructure:"BILLING_KIWIFY_EVENTS_HOUSEKEEPING_SCHEDULE"`
	KiwifyEventsHousekeepingBatch    int           `mapstructure:"BILLING_KIWIFY_EVENTS_HOUSEKEEPING_BATCH"`
	GraceExpirationSchedule          string        `mapstructure:"BILLING_GRACE_EXPIRATION_SCHEDULE"`
}

type AppConfig struct {
	Environment string `mapstructure:"ENVIRONMENT"`
	AppMode     string `mapstructure:"APP_MODE"`
}

type HTTPConfig struct {
	Port               int    `mapstructure:"PORT"`
	ServiceNameAPI     string `mapstructure:"SERVICE_NAME_API"`
	ServiceNameWorker  string `mapstructure:"SERVICE_NAME_WORKER"`
	CORSAllowedOrigins string `mapstructure:"CORS_ALLOWED_ORIGINS"`
}

type DBConfig struct {
	Host     string `mapstructure:"DB_HOST"`
	Port     int    `mapstructure:"DB_PORT"`
	User     string `mapstructure:"DB_USER"`
	Password string `mapstructure:"DB_PASSWORD"`
	Name     string `mapstructure:"DB_NAME"`
	SSLMode  string `mapstructure:"DB_SSL_MODE"`

	MaxConns        int           `mapstructure:"DB_MAX_CONNS"`
	MinConns        int           `mapstructure:"DB_MIN_CONNS"`
	MaxIdleConns    int           `mapstructure:"DB_MAX_IDLE_CONNS"`
	ConnMaxLifetime time.Duration `mapstructure:"DB_CONN_MAX_LIFETIME"`
	ConnMaxIdleTime time.Duration `mapstructure:"DB_CONN_MAX_IDLE_TIME"`
}

const databaseSearchPath = "mecontrola,public"
const migrationTableQueryParam = "&x-migrations-table=%22public%22.%22schema_migrations%22&x-migrations-table-quoted=true"

func (d *DBConfig) DSN() string {
	return d.formatDSN(true)
}

func (d *DBConfig) MigrationDSN() string {
	return d.formatDSN(true) + migrationTableQueryParam
}

func (d *DBConfig) formatDSN(withSearchPath bool) string {
	if !withSearchPath {
		return fmt.Sprintf(
			"postgres://%s:%s@%s:%d/%s?sslmode=%s",
			d.User, d.Password, d.Host, d.Port, d.Name, d.SSLMode,
		)
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s&search_path=%s",
		d.User, d.Password, d.Host, d.Port, d.Name, d.SSLMode, databaseSearchPath,
	)
}

func (d *DBConfig) SafeDSN() string {
	return fmt.Sprintf(
		"postgres://%s:***@%s:%d/%s?sslmode=%s&search_path=%s",
		d.User, d.Host, d.Port, d.Name, d.SSLMode, databaseSearchPath,
	)
}

type O11yConfig struct {
	ServiceVersion   string  `mapstructure:"OTEL_SERVICE_VERSION"`
	ExporterEndpoint string  `mapstructure:"OTEL_EXPORTER_OTLP_ENDPOINT"`
	ExporterProtocol string  `mapstructure:"OTEL_EXPORTER_OTLP_PROTOCOL"`
	ExporterInsecure bool    `mapstructure:"OTEL_EXPORTER_OTLP_INSECURE"`
	TraceSampleRate  float64 `mapstructure:"OTEL_TRACE_SAMPLE_RATE"`
	LogLevel         string  `mapstructure:"LOG_LEVEL"`
	LogFormat        string  `mapstructure:"LOG_FORMAT"`
}

func (o O11yConfig) NormalizedExporterEndpoint() string {
	endpoint := strings.TrimSpace(o.ExporterEndpoint)
	if endpoint == "" {
		return ""
	}

	if strings.EqualFold(strings.TrimSpace(o.ExporterProtocol), "grpc") {
		endpoint = strings.TrimPrefix(endpoint, "http://")
		endpoint = strings.TrimPrefix(endpoint, "https://")
		endpoint = strings.TrimSuffix(endpoint, "/")
	}

	return endpoint
}

type OutboxConfig struct {
	DispatcherEnabled         bool          `mapstructure:"OUTBOX_DISPATCHER_ENABLED"`
	DispatcherTickInterval    time.Duration `mapstructure:"OUTBOX_DISPATCHER_TICK_INTERVAL"`
	DispatcherBatchSize       int           `mapstructure:"OUTBOX_DISPATCHER_BATCH_SIZE"`
	DispatcherHandlerTimeout  time.Duration `mapstructure:"OUTBOX_DISPATCHER_HANDLER_TIMEOUT"`
	RetryMaxAttempts          int           `mapstructure:"OUTBOX_RETRY_MAX_ATTEMPTS"`
	RetryBaseBackoff          time.Duration `mapstructure:"OUTBOX_RETRY_BASE_BACKOFF"`
	RetryMaxBackoff           time.Duration `mapstructure:"OUTBOX_RETRY_MAX_BACKOFF"`
	HousekeepingRetentionDays int           `mapstructure:"OUTBOX_HOUSEKEEPING_RETENTION_DAYS"`
	HousekeepingSchedule      string        `mapstructure:"OUTBOX_HOUSEKEEPING_SCHEDULE"`
	ReaperInterval            string        `mapstructure:"OUTBOX_REAPER_INTERVAL"`
	ReaperStuckAfter          time.Duration `mapstructure:"OUTBOX_REAPER_STUCK_AFTER"`
}

type configLoader struct {
	v    *viper.Viper
	path string
}

func (l *configLoader) requiresLocalEnvFile() bool {
	return l.v.GetString("ENVIRONMENT") != "production"
}

func (l *configLoader) load() (*Config, error) {
	l.v.SetConfigName(".env")
	l.v.SetConfigType("env")
	l.v.AddConfigPath(l.path)
	l.v.AutomaticEnv()
	l.v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	for _, key := range l.envKeys() {
		_ = l.v.BindEnv(key)
	}

	l.v.SetDefault("PORT", 8080)
	l.v.SetDefault("APP_MODE", "server")
	l.v.SetDefault("ENVIRONMENT", "local")
	l.v.SetDefault("LOG_LEVEL", "info")
	l.v.SetDefault("LOG_FORMAT", "json")
	l.v.SetDefault("OTEL_SERVICE_VERSION", "dev")
	l.v.SetDefault("OTEL_TRACE_SAMPLE_RATE", 1.0)
	l.v.SetDefault("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")
	l.v.SetDefault("OTEL_EXPORTER_OTLP_INSECURE", true)
	l.v.SetDefault("DB_PORT", 5432)
	l.v.SetDefault("DB_SSL_MODE", "disable")
	l.v.SetDefault("DB_MAX_CONNS", 10)
	l.v.SetDefault("DB_MIN_CONNS", 2)
	l.v.SetDefault("DB_MAX_IDLE_CONNS", 5)
	l.v.SetDefault("DB_CONN_MAX_LIFETIME", 30*time.Minute)
	l.v.SetDefault("DB_CONN_MAX_IDLE_TIME", 5*time.Minute)

	l.setOutboxDefaults()
	l.setKiwifyDefaults()
	l.setBillingDefaults()
	l.setBudgetsDefaults()
	l.setOnboardingDefaults()
	l.setWhatsAppDefaults()

	if err := l.v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, fmt.Errorf("lendo arquivo de configuração: %w", err)
		}
		if l.requiresLocalEnvFile() {
			return nil, fmt.Errorf("arquivo .env obrigatório não encontrado em %q para ENVIRONMENT=%s", l.path, l.v.GetString("ENVIRONMENT"))
		}
	}

	cfg := &Config{}
	if err := l.v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("deserializando configuração: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validando configuração: %w", err)
	}

	return cfg, nil
}

func (l *configLoader) envKeys() []string {
	return []string{
		"ENVIRONMENT", "APP_MODE",
		"PORT", "SERVICE_NAME_API", "CORS_ALLOWED_ORIGINS",
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSL_MODE",
		"DB_MAX_CONNS", "DB_MIN_CONNS", "DB_MAX_IDLE_CONNS",
		"DB_CONN_MAX_LIFETIME", "DB_CONN_MAX_IDLE_TIME",
		"OTEL_SERVICE_VERSION", "OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_PROTOCOL",
		"OTEL_EXPORTER_OTLP_INSECURE", "OTEL_TRACE_SAMPLE_RATE", "LOG_LEVEL", "LOG_FORMAT",
		"SERVICE_NAME_WORKER",
		"OUTBOX_DISPATCHER_ENABLED",
		"OUTBOX_DISPATCHER_TICK_INTERVAL",
		"OUTBOX_DISPATCHER_BATCH_SIZE",
		"OUTBOX_DISPATCHER_HANDLER_TIMEOUT",
		"OUTBOX_RETRY_MAX_ATTEMPTS",
		"OUTBOX_RETRY_BASE_BACKOFF",
		"OUTBOX_RETRY_MAX_BACKOFF",
		"OUTBOX_HOUSEKEEPING_RETENTION_DAYS",
		"OUTBOX_HOUSEKEEPING_SCHEDULE",
		"OUTBOX_REAPER_INTERVAL",
		"OUTBOX_REAPER_STUCK_AFTER",
		"KIWIFY_API_BASE_URL",
		"KIWIFY_ACCOUNT_ID",
		"KIWIFY_PRODUCT_ID_MONTHLY",
		"KIWIFY_PRODUCT_ID_QUARTERLY",
		"KIWIFY_PRODUCT_ID_ANNUAL",
		"KIWIFY_WEBHOOK_SECRET",
		"KIWIFY_WEBHOOK_SECRET_NEXT",
		"KIWIFY_WEBHOOK_TOKEN_HEADER",
		"KIWIFY_CLIENT_ID",
		"KIWIFY_CLIENT_SECRET",
		"KIWIFY_OAUTH_TOKEN_SAFETY_MARGIN",
		"KIWIFY_RATE_LIMIT_MAX_REQUESTS_PER_MIN",
		"KIWIFY_RATE_LIMIT_BURST",
		"KIWIFY_RECONCILIATION_INTERVAL",
		"KIWIFY_RECONCILIATION_BATCH_SIZE",
		"KIWIFY_HTTP_TIMEOUT",
		"KIWIFY_HTTP_RETRY_MAX_ATTEMPTS",
		"KIWIFY_HTTP_RETRY_BACKOFF",
		"BILLING_ENTITLEMENT_CACHE_CAPACITY",
		"BILLING_ENTITLEMENT_CACHE_TTL",
		"BILLING_ANONYMIZATION_SCHEDULE",
		"BILLING_ANONYMIZATION_BATCH_SIZE",
		"BILLING_ANONYMIZATION_RETENTION_DAYS",
		"BILLING_KIWIFY_EVENTS_RETENTION_DAYS",
		"BILLING_KIWIFY_EVENTS_HOUSEKEEPING_SCHEDULE",
		"BILLING_KIWIFY_EVENTS_HOUSEKEEPING_BATCH",
		"ONBOARDING_TOKEN_TTL_DAYS",
		"ONBOARDING_OUTREACH_GAP_HOURS",
		"ONBOARDING_OUTREACH_ENABLED",
		"ONBOARDING_CHECKOUT_CORS_ORIGINS",
		"ONBOARDING_TRUSTED_PROXIES",
		"ONBOARDING_CHECKOUT_RATE_LIMIT_PER_MIN",
		"ONBOARDING_CHECKOUT_RATE_LIMIT_BURST",
		"ONBOARDING_STATE_RATE_LIMIT_PER_MIN",
		"ONBOARDING_STATE_RATE_LIMIT_BURST",
		"ONBOARDING_KIWIFY_CHECKOUT_URLS",
		"ONBOARDING_KIWIFY_ALLOWED_HOSTS",
		"ONBOARDING_META_RETENTION_DAYS",
		"ONBOARDING_META_CLEANUP_SCHEDULE",
		"ONBOARDING_TOKEN_EXPIRATION_SCHEDULE",
		"ONBOARDING_MAX_TOKEN_LOOKUP_ATTEMPTS",
		"ONBOARDING_TOKEN_ENCRYPTION_KEY",
		"META_PHONE_NUMBER_ID",
		"META_ACCESS_TOKEN",
		"META_APP_SECRET",
		"META_APP_SECRET_NEXT",
		"META_VERIFY_TOKEN",
		"META_OUTREACH_TEMPLATE_NAME",
		"META_BOT_NUMBER_E164",
		"META_BOT_NUMBER_DISPLAY",
		"WA_MSG_WELCOME_ACTIVATED",
		"WA_MSG_ALREADY_ACTIVE",
		"WA_MSG_CODE_ALREADY_USED_OTHER_ACCOUNT",
		"WA_MSG_PAYMENT_STILL_PROCESSING_RETRY",
		"WA_MSG_CODE_EXPIRED_CONTACT_SUPPORT",
		"WA_MSG_CODE_INVALID_CHECK_AGAIN",
		"WA_MSG_SYSTEM_UNAVAILABLE_RETRY",
		"WA_MSG_PLEASE_USE_ATIVAR_COMMAND",
		"WA_MSG_INVALID_COUNTRY",
		"WA_MSG_AGENT_STUB_RECEIVED",
		"BUDGETS_PENDING_REAPER_INTERVAL",
		"BUDGETS_PENDING_TTL_HOURS",
		"BUDGETS_ABANDONED_DRAFT_CRON",
		"BUDGETS_RETENTION_PURGE_CRON",
		"BUDGETS_RETENTION_PURGE_BATCH_SIZE",
	}
}

func (l *configLoader) setKiwifyDefaults() {
	l.v.SetDefault("KIWIFY_API_BASE_URL", "https://public-api.kiwify.com")
	l.v.SetDefault("KIWIFY_WEBHOOK_TOKEN_HEADER", "X-Kiwify-Webhook-Token")
	l.v.SetDefault("KIWIFY_OAUTH_TOKEN_SAFETY_MARGIN", 5*time.Minute)
	l.v.SetDefault("KIWIFY_RATE_LIMIT_MAX_REQUESTS_PER_MIN", 100)
	l.v.SetDefault("KIWIFY_RATE_LIMIT_BURST", 10)
	l.v.SetDefault("KIWIFY_RECONCILIATION_INTERVAL", "@hourly")
	l.v.SetDefault("KIWIFY_RECONCILIATION_BATCH_SIZE", 200)
	l.v.SetDefault("KIWIFY_HTTP_TIMEOUT", 10*time.Second)
	l.v.SetDefault("KIWIFY_HTTP_RETRY_MAX_ATTEMPTS", 3)
	l.v.SetDefault("KIWIFY_HTTP_RETRY_BACKOFF", time.Second)
}

func (l *configLoader) setBudgetsDefaults() {
	l.v.SetDefault("BUDGETS_PENDING_REAPER_INTERVAL", "@every 30s")
	l.v.SetDefault("BUDGETS_PENDING_TTL_HOURS", 24)
	l.v.SetDefault("BUDGETS_ABANDONED_DRAFT_CRON", "0 3 * * *")
	l.v.SetDefault("BUDGETS_RETENTION_PURGE_CRON", "0 4 1 * *")
	l.v.SetDefault("BUDGETS_RETENTION_PURGE_BATCH_SIZE", 500)
}

func (l *configLoader) setBillingDefaults() {
	l.v.SetDefault("BILLING_ENTITLEMENT_CACHE_CAPACITY", 50000)
	l.v.SetDefault("BILLING_ENTITLEMENT_CACHE_TTL", 5*time.Minute)
	l.v.SetDefault("BILLING_ANONYMIZATION_SCHEDULE", "@daily")
	l.v.SetDefault("BILLING_ANONYMIZATION_BATCH_SIZE", 500)
	l.v.SetDefault("BILLING_ANONYMIZATION_RETENTION_DAYS", 365)
	l.v.SetDefault("BILLING_KIWIFY_EVENTS_RETENTION_DAYS", 90)
	l.v.SetDefault("BILLING_KIWIFY_EVENTS_HOUSEKEEPING_SCHEDULE", "@daily")
	l.v.SetDefault("BILLING_KIWIFY_EVENTS_HOUSEKEEPING_BATCH", 500)
}

func (l *configLoader) setOutboxDefaults() {
	l.v.SetDefault("OUTBOX_DISPATCHER_ENABLED", true)
	l.v.SetDefault("OUTBOX_DISPATCHER_TICK_INTERVAL", 500*time.Millisecond)
	l.v.SetDefault("OUTBOX_DISPATCHER_BATCH_SIZE", 50)
	l.v.SetDefault("OUTBOX_DISPATCHER_HANDLER_TIMEOUT", 10*time.Second)
	l.v.SetDefault("OUTBOX_RETRY_MAX_ATTEMPTS", 15)
	l.v.SetDefault("OUTBOX_RETRY_BASE_BACKOFF", 2*time.Second)
	l.v.SetDefault("OUTBOX_RETRY_MAX_BACKOFF", 5*time.Minute)
	l.v.SetDefault("OUTBOX_HOUSEKEEPING_RETENTION_DAYS", 90)
	l.v.SetDefault("OUTBOX_HOUSEKEEPING_SCHEDULE", "@daily")
	l.v.SetDefault("OUTBOX_REAPER_INTERVAL", "@every 1m")
	l.v.SetDefault("OUTBOX_REAPER_STUCK_AFTER", 5*time.Minute)
}

func LoadConfig(path string) (*Config, error) {
	return (&configLoader{v: viper.New(), path: path}).load()
}

func (c *Config) Validate() error {
	var errs []string

	switch c.AppConfig.Environment {
	case "local", "staging", "production":
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

	errs = append(errs, c.validatePoolTunables()...)
	errs = append(errs, c.validateOutbox()...)
	errs = append(errs, c.validateBilling()...)
	errs = append(errs, c.validateOnboarding()...)
	errs = append(errs, c.KiwifyConfig.validateProductIDs()...)

	if c.AppConfig.Environment == "production" {
		errs = append(errs, c.validateProduction()...)
	}

	if len(errs) > 0 {
		return fmt.Errorf("configuração inválida:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}

func (c *Config) validatePoolTunables() []string {
	db := c.DBConfig
	if db.MaxConns == 0 && db.MinConns == 0 && db.MaxIdleConns == 0 &&
		db.ConnMaxLifetime == 0 && db.ConnMaxIdleTime == 0 {
		return nil
	}

	var errs []string
	errs = append(errs, validatePoolSizes(db)...)
	errs = append(errs, validatePoolTimeouts(db)...)
	return errs
}

func validatePoolSizes(db DBConfig) []string {
	var errs []string

	if db.MaxConns < 1 {
		errs = append(errs, "DB_MAX_CONNS deve ser maior que zero")
	}
	if db.MinConns < 0 {
		errs = append(errs, "DB_MIN_CONNS não pode ser negativo")
	}
	if db.MaxIdleConns < 0 {
		errs = append(errs, "DB_MAX_IDLE_CONNS não pode ser negativo")
	}
	if db.MaxConns > 0 && db.MinConns > db.MaxConns {
		errs = append(errs, "DB_MIN_CONNS não pode ser maior que DB_MAX_CONNS")
	}
	if db.MaxConns > 0 && db.MaxIdleConns > db.MaxConns {
		errs = append(errs, "DB_MAX_IDLE_CONNS não pode ser maior que DB_MAX_CONNS")
	}

	return errs
}

func validatePoolTimeouts(db DBConfig) []string {
	var errs []string

	if db.ConnMaxLifetime < 0 {
		errs = append(errs, "DB_CONN_MAX_LIFETIME não pode ser negativo")
	}
	if db.ConnMaxIdleTime < 0 {
		errs = append(errs, "DB_CONN_MAX_IDLE_TIME não pode ser negativo")
	}
	if db.ConnMaxLifetime > 0 && db.ConnMaxIdleTime > db.ConnMaxLifetime {
		errs = append(errs, "DB_CONN_MAX_IDLE_TIME não pode ser maior que DB_CONN_MAX_LIFETIME")
	}

	return errs
}

func (c *Config) validateOutbox() []string {
	var errs []string
	o := c.OutboxConfig

	if o.RetryMaxAttempts == 0 && o.DispatcherBatchSize == 0 && o.HousekeepingRetentionDays == 0 {
		return nil
	}

	if o.RetryMaxAttempts < 1 || o.RetryMaxAttempts > 50 {
		errs = append(errs, fmt.Sprintf(
			"OUTBOX_RETRY_MAX_ATTEMPTS inválido %d: deve estar no intervalo [1..50]",
			o.RetryMaxAttempts,
		))
	}

	if o.DispatcherBatchSize < 1 || o.DispatcherBatchSize > 500 {
		errs = append(errs, fmt.Sprintf(
			"OUTBOX_DISPATCHER_BATCH_SIZE inválido %d: deve estar no intervalo [1..500]",
			o.DispatcherBatchSize,
		))
	}

	if o.HousekeepingRetentionDays < 1 || o.HousekeepingRetentionDays > 3650 {
		errs = append(errs, fmt.Sprintf(
			"OUTBOX_HOUSEKEEPING_RETENTION_DAYS inválido %d: deve estar no intervalo [1..3650]",
			o.HousekeepingRetentionDays,
		))
	}

	if o.HousekeepingSchedule != "" {
		if _, err := cron.ParseStandard(o.HousekeepingSchedule); err != nil {
			errs = append(errs, fmt.Sprintf(
				"OUTBOX_HOUSEKEEPING_SCHEDULE inválido %q: %v",
				o.HousekeepingSchedule, err,
			))
		}
	}

	if o.ReaperInterval != "" {
		if _, err := cron.ParseStandard(o.ReaperInterval); err != nil {
			errs = append(errs, fmt.Sprintf(
				"OUTBOX_REAPER_INTERVAL inválido %q: %v",
				o.ReaperInterval, err,
			))
		}
	}

	return errs
}

func (c *Config) validateProduction() []string {
	var errs []string

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

	errs = append(errs, c.validateProductionKiwify()...)
	return errs
}

func (c *Config) validateProductionKiwify() []string {
	k := c.KiwifyConfig

	if !k.isConfigured() {
		return nil
	}

	var errs []string

	if k.WebhookSecret == "" {
		errs = append(errs, "KIWIFY_WEBHOOK_SECRET é obrigatório em production quando Kiwify está habilitado")
	}

	if k.ClientID == "" {
		errs = append(errs, "KIWIFY_CLIENT_ID é obrigatório em production quando Kiwify está habilitado")
	}

	if k.AccountID == "" {
		errs = append(errs, "KIWIFY_ACCOUNT_ID é obrigatório em production quando Kiwify está habilitado")
	}

	if k.ClientSecret == "" {
		errs = append(errs, "KIWIFY_CLIENT_SECRET é obrigatório em production quando Kiwify está habilitado")
	}
	errs = append(errs, k.requiredProductIDErrors()...)

	for _, placeholder := range InsecurePlaceholders {
		if k.WebhookSecret == placeholder {
			errs = append(errs, fmt.Sprintf(
				"KIWIFY_WEBHOOK_SECRET contém placeholder inseguro %q em production",
				placeholder,
			))
			break
		}
	}

	for _, placeholder := range InsecurePlaceholders {
		if k.ClientSecret == placeholder {
			errs = append(errs, fmt.Sprintf(
				"KIWIFY_CLIENT_SECRET contém placeholder inseguro %q em production",
				placeholder,
			))
			break
		}
	}

	return errs
}

func (k KiwifyConfig) isConfigured() bool {
	return k.AccountID != "" || k.ClientID != "" || k.WebhookSecret != "" || k.ClientSecret != "" ||
		k.ProductIDMonthly != "" || k.ProductIDQuarterly != "" || k.ProductIDAnnual != ""
}

func (k KiwifyConfig) requiredProductIDErrors() []string {
	var errs []string
	if k.ProductIDMonthly == "" {
		errs = append(errs, "KIWIFY_PRODUCT_ID_MONTHLY é obrigatório em production quando Kiwify está habilitado")
	}
	if k.ProductIDQuarterly == "" {
		errs = append(errs, "KIWIFY_PRODUCT_ID_QUARTERLY é obrigatório em production quando Kiwify está habilitado")
	}
	if k.ProductIDAnnual == "" {
		errs = append(errs, "KIWIFY_PRODUCT_ID_ANNUAL é obrigatório em production quando Kiwify está habilitado")
	}
	return errs
}

func (k KiwifyConfig) validateProductIDs() []string {
	configured := 0
	for _, productID := range []string{k.ProductIDMonthly, k.ProductIDQuarterly, k.ProductIDAnnual} {
		if productID != "" {
			configured++
		}
	}
	if configured == 0 || configured == 3 {
		return nil
	}
	return []string{"KIWIFY_PRODUCT_ID_MONTHLY, KIWIFY_PRODUCT_ID_QUARTERLY e KIWIFY_PRODUCT_ID_ANNUAL devem ser configurados juntos"}
}

func (c *Config) validateBilling() []string {
	b := c.BillingConfig

	if b.EntitlementCacheCapacity == 0 && b.AnonymizationBatchSize == 0 && b.AnonymizationRetentionDays == 0 && b.KiwifyEventsRetentionDays == 0 {
		return nil
	}

	var errs []string

	if b.EntitlementCacheCapacity < 1000 || b.EntitlementCacheCapacity > 500000 {
		errs = append(errs, fmt.Sprintf(
			"BILLING_ENTITLEMENT_CACHE_CAPACITY inválido %d: deve estar no intervalo [1000..500000]",
			b.EntitlementCacheCapacity,
		))
	}

	if b.EntitlementCacheTTL < time.Second || b.EntitlementCacheTTL > time.Hour {
		errs = append(errs, fmt.Sprintf(
			"BILLING_ENTITLEMENT_CACHE_TTL inválido %s: deve estar no intervalo [1s..1h]",
			b.EntitlementCacheTTL,
		))
	}

	k := c.KiwifyConfig
	if k.RateLimitMaxRequestsPerMin < 1 || k.RateLimitMaxRequestsPerMin > 500 {
		errs = append(errs, fmt.Sprintf(
			"KIWIFY_RATE_LIMIT_MAX_REQUESTS_PER_MIN inválido %d: deve estar no intervalo [1..500]",
			k.RateLimitMaxRequestsPerMin,
		))
	}

	errs = append(errs, validateKiwifyHTTP(k)...)

	return errs
}

func (c *Config) validateOnboarding() []string {
	o := c.OnboardingConfig
	if o.KiwifyCheckoutURLs == "" && !o.OutreachEnabled && c.WhatsAppConfig.PhoneNumberID == "" &&
		c.WhatsAppConfig.AccessToken == "" && c.WhatsAppConfig.AppSecret == "" {
		return nil
	}

	var errs []string
	if o.TokenEncryptionKey == "" {
		errs = append(errs, "ONBOARDING_TOKEN_ENCRYPTION_KEY é obrigatório quando onboarding está habilitado")
	}
	if c.AppConfig.Environment == "production" && o.TokenEncryptionKey == "0123456789abcdef0123456789abcdef" {
		errs = append(errs, "ONBOARDING_TOKEN_ENCRYPTION_KEY deve ser substituida em production")
	}
	if o.TokenEncryptionKey != "" && len(o.TokenEncryptionKey) != 32 &&
		len(o.TokenEncryptionKey) != 43 && len(o.TokenEncryptionKey) != 44 {
		errs = append(errs, "ONBOARDING_TOKEN_ENCRYPTION_KEY deve ter 32 bytes ou base64 de 32 bytes")
	}
	return errs
}

func (l *configLoader) setOnboardingDefaults() {
	l.v.SetDefault("ONBOARDING_TOKEN_TTL_DAYS", 7)
	l.v.SetDefault("ONBOARDING_OUTREACH_GAP_HOURS", 2)
	l.v.SetDefault("ONBOARDING_OUTREACH_ENABLED", false)
	l.v.SetDefault("ONBOARDING_CHECKOUT_CORS_ORIGINS", "https://www.mecontrola.app.br,https://mecontrola.app.br")
	l.v.SetDefault("ONBOARDING_TRUSTED_PROXIES", "127.0.0.1/32,::1/128")
	l.v.SetDefault("ONBOARDING_CHECKOUT_RATE_LIMIT_PER_MIN", 10)
	l.v.SetDefault("ONBOARDING_CHECKOUT_RATE_LIMIT_BURST", 5)
	l.v.SetDefault("ONBOARDING_STATE_RATE_LIMIT_PER_MIN", 30)
	l.v.SetDefault("ONBOARDING_STATE_RATE_LIMIT_BURST", 10)
	l.v.SetDefault("ONBOARDING_KIWIFY_ALLOWED_HOSTS", "pay.kiwify.com.br")
	l.v.SetDefault("ONBOARDING_META_RETENTION_DAYS", 30)
	l.v.SetDefault("ONBOARDING_META_CLEANUP_SCHEDULE", "30 3 * * *")
	l.v.SetDefault("ONBOARDING_TOKEN_EXPIRATION_SCHEDULE", "0 3 * * *")
	l.v.SetDefault("ONBOARDING_MAX_TOKEN_LOOKUP_ATTEMPTS", 5)
}

func (l *configLoader) setWhatsAppDefaults() {
	l.v.SetDefault("META_OUTREACH_TEMPLATE_NAME", "activation_reminder")
	l.v.SetDefault("META_BOT_NUMBER_DISPLAY", "+55 11 9XXXX-XXXX")
	l.v.SetDefault("WA_MSG_WELCOME_ACTIVATED", "Sua conta foi ativada com sucesso! Bem-vindo ao MeControla.")
	l.v.SetDefault("WA_MSG_ALREADY_ACTIVE", "Sua conta ja esta ativa.")
	l.v.SetDefault("WA_MSG_CODE_ALREADY_USED_OTHER_ACCOUNT", "Este codigo ja foi utilizado por outra conta.")
	l.v.SetDefault("WA_MSG_PAYMENT_STILL_PROCESSING_RETRY", "Seu pagamento ainda esta sendo processado. Tente novamente em alguns minutos.")
	l.v.SetDefault("WA_MSG_CODE_EXPIRED_CONTACT_SUPPORT", "Este codigo expirou. Entre em contato com o suporte.")
	l.v.SetDefault("WA_MSG_CODE_INVALID_CHECK_AGAIN", "Codigo invalido. Verifique o link de ativacao e tente novamente.")
	l.v.SetDefault("WA_MSG_SYSTEM_UNAVAILABLE_RETRY", "Sistema temporariamente indisponivel. Tente novamente em alguns minutos.")
	l.v.SetDefault("WA_MSG_PLEASE_USE_ATIVAR_COMMAND", "Para ativar sua conta, envie: ATIVAR seguido do seu codigo de ativacao.")
	l.v.SetDefault("WA_MSG_INVALID_COUNTRY", "Numero de telefone nao suportado. Apenas numeros brasileiros sao aceitos.")
	l.v.SetDefault("WA_MSG_AGENT_STUB_RECEIVED", "MeControla recebeu sua mensagem — estamos preparando sua experiencia.")
}

func validateKiwifyHTTP(k KiwifyConfig) []string {
	if k.HTTPTimeout == 0 && k.HTTPRetryMaxAttempts == 0 && k.HTTPRetryBackoff == 0 {
		return nil
	}

	var errs []string
	if k.HTTPTimeout <= 0 || k.HTTPTimeout > time.Minute {
		errs = append(errs, fmt.Sprintf(
			"KIWIFY_HTTP_TIMEOUT inválido %s: deve estar no intervalo (0..1m]",
			k.HTTPTimeout,
		))
	}
	if k.HTTPRetryMaxAttempts < 0 || k.HTTPRetryMaxAttempts > 10 {
		errs = append(errs, fmt.Sprintf(
			"KIWIFY_HTTP_RETRY_MAX_ATTEMPTS inválido %d: deve estar no intervalo [0..10]",
			k.HTTPRetryMaxAttempts,
		))
	}
	if k.HTTPRetryMaxAttempts > 0 && (k.HTTPRetryBackoff <= 0 || k.HTTPRetryBackoff > 10*time.Second) {
		errs = append(errs, fmt.Sprintf(
			"KIWIFY_HTTP_RETRY_BACKOFF inválido %s: deve estar no intervalo (0..10s] quando há retry habilitado",
			k.HTTPRetryBackoff,
		))
	}
	return errs
}
