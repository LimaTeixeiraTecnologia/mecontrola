package configs_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

type ConfigSuite struct {
	suite.Suite
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}

func (s *ConfigSuite) SetupTest() {}

func (s *ConfigSuite) TestValidate() {
	type args struct {
		build func() *configs.Config
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func()
		expect func(cfg *configs.Config, err error)
	}{
		{
			name: "deve validar config local com sucesso",
			args: args{
				build: func() *configs.Config {
					return s.newBaseConfig()
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar erro quando environment invalido",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.AppConfig.Environment = "dev"
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "ENVIRONMENT inválido")
			},
		},
		{
			name: "deve retornar erro quando environment vazio",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.AppConfig.Environment = ""
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "ENVIRONMENT inválido")
			},
		},
		{
			name: "deve retornar erro quando port zero",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.HTTPConfig.Port = 0
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "PORT inválido")
			},
		},
		{
			name: "deve retornar erro quando port acima de 65535",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.HTTPConfig.Port = 65536
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "PORT inválido")
			},
		},
		{
			name: "deve aceitar port minimo valido igual a 1",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.HTTPConfig.Port = 1
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve aceitar port maximo valido igual a 65535",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.HTTPConfig.Port = 65535
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar erro quando trace sample rate negativo",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.O11yConfig.TraceSampleRate = -0.1
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "OTEL_TRACE_SAMPLE_RATE inválido")
			},
		},
		{
			name: "deve retornar erro quando trace sample rate acima de 1",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.O11yConfig.TraceSampleRate = 1.1
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "OTEL_TRACE_SAMPLE_RATE inválido")
			},
		},
		{
			name: "deve aceitar trace sample rate zero como valido",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.O11yConfig.TraceSampleRate = 0
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve aceitar trace sample rate um como valido",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.O11yConfig.TraceSampleRate = 1
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar erro quando production com senha curta",
			args: args{
				build: func() *configs.Config {
					cfg := s.newProductionConfig()
					cfg.DBConfig.Password = "short"
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "DB_PASSWORD deve ter ao menos 16 caracteres")
			},
		},
		{
			name: "deve aceitar production com senha de exatamente 16 caracteres",
			args: args{
				build: func() *configs.Config {
					cfg := s.newProductionConfig()
					cfg.DBConfig.Password = "exactly16chars!!"
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar erro quando production com placeholder na senha",
			args: args{
				build: func() *configs.Config {
					cfg := s.newProductionConfig()
					cfg.DBConfig.Password = "CHANGE_ME_USE_STRONG_PASSWORD"
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "placeholder inseguro")
			},
		},
		{
			name: "deve retornar erro quando production com placeholder no usuario",
			args: args{
				build: func() *configs.Config {
					cfg := s.newProductionConfig()
					cfg.DBConfig.User = "your_secret_key"
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "DB_USER contém placeholder inseguro")
			},
		},
		{
			name: "deve aceitar production valida",
			args: args{
				build: func() *configs.Config {
					return s.newProductionConfig()
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve validar staging sem exigir senha longa",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.AppConfig.Environment = "staging"
					cfg.DBConfig.Password = "short"
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve acumular multiplos erros de validacao",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.AppConfig.Environment = "invalid"
					cfg.HTTPConfig.Port = 0
					cfg.O11yConfig.TraceSampleRate = 2
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "ENVIRONMENT inválido", "PORT inválido", "OTEL_TRACE_SAMPLE_RATE inválido")
			},
		},
		{
			name: "deve aceitar outbox configurado com valores validos",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.OutboxConfig = s.newValidOutboxConfig()
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve rejeitar retry max attempts zero",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.OutboxConfig = s.newValidOutboxConfig()
					cfg.OutboxConfig.RetryMaxAttempts = 0
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "OUTBOX_RETRY_MAX_ATTEMPTS inválido")
			},
		},
		{
			name: "deve rejeitar retry max attempts acima de 3",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.OutboxConfig = s.newValidOutboxConfig()
					cfg.OutboxConfig.RetryMaxAttempts = 4
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "OUTBOX_RETRY_MAX_ATTEMPTS inválido")
			},
		},
		{
			name: "deve aceitar retry max attempts igual a 1",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.OutboxConfig = s.newValidOutboxConfig()
					cfg.OutboxConfig.RetryMaxAttempts = 1
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve aceitar retry max attempts igual a 3",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.OutboxConfig = s.newValidOutboxConfig()
					cfg.OutboxConfig.RetryMaxAttempts = 3
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve rejeitar dispatcher batch size zero",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.OutboxConfig = s.newValidOutboxConfig()
					cfg.OutboxConfig.DispatcherBatchSize = 0
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "OUTBOX_DISPATCHER_BATCH_SIZE inválido")
			},
		},
		{
			name: "deve rejeitar dispatcher batch size acima de 500",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.OutboxConfig = s.newValidOutboxConfig()
					cfg.OutboxConfig.DispatcherBatchSize = 501
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "OUTBOX_DISPATCHER_BATCH_SIZE inválido")
			},
		},
		{
			name: "deve rejeitar housekeeping retention days zero",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.OutboxConfig = s.newValidOutboxConfig()
					cfg.OutboxConfig.HousekeepingRetentionDays = 0
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "OUTBOX_HOUSEKEEPING_RETENTION_DAYS inválido")
			},
		},
		{
			name: "deve rejeitar housekeeping retention days acima de 3650",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.OutboxConfig = s.newValidOutboxConfig()
					cfg.OutboxConfig.HousekeepingRetentionDays = 3651
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "OUTBOX_HOUSEKEEPING_RETENTION_DAYS inválido")
			},
		},
		{
			name: "deve rejeitar housekeeping schedule invalido",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.OutboxConfig = s.newValidOutboxConfig()
					cfg.OutboxConfig.HousekeepingSchedule = "nao-e-cron"
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "OUTBOX_HOUSEKEEPING_SCHEDULE inválido")
			},
		},
		{
			name: "deve aceitar housekeeping schedule weekly valido",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.OutboxConfig = s.newValidOutboxConfig()
					cfg.OutboxConfig.HousekeepingSchedule = "@weekly"
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve rejeitar reaper interval invalido",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.OutboxConfig = s.newValidOutboxConfig()
					cfg.OutboxConfig.ReaperInterval = "invalido"
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "OUTBOX_REAPER_INTERVAL inválido")
			},
		},
		{
			name: "deve aceitar reaper interval valido",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.OutboxConfig = s.newValidOutboxConfig()
					cfg.OutboxConfig.ReaperInterval = "@every 5m"
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve rejeitar max conns zero quando tunables configurados",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.DBConfig = configs.DBConfig{MaxConns: 0, MinConns: 1, MaxIdleConns: 1}
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "DB_MAX_CONNS deve ser maior que zero")
			},
		},
		{
			name: "deve rejeitar min conns maior que max conns",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.DBConfig = configs.DBConfig{MaxConns: 2, MinConns: 3, MaxIdleConns: 1}
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "DB_MIN_CONNS não pode ser maior que DB_MAX_CONNS")
			},
		},
		{
			name: "deve rejeitar max idle maior que max conns",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.DBConfig = configs.DBConfig{MaxConns: 2, MinConns: 1, MaxIdleConns: 3}
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "DB_MAX_IDLE_CONNS não pode ser maior que DB_MAX_CONNS")
			},
		},
		{
			name: "deve rejeitar conn max lifetime negativo",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.DBConfig = configs.DBConfig{MaxConns: 2, MinConns: 1, MaxIdleConns: 1, ConnMaxLifetime: -time.Second}
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "DB_CONN_MAX_LIFETIME não pode ser negativo")
			},
		},
		{
			name: "deve rejeitar conn max idle time negativo",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.DBConfig = configs.DBConfig{MaxConns: 2, MinConns: 1, MaxIdleConns: 1, ConnMaxIdleTime: -time.Second}
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "DB_CONN_MAX_IDLE_TIME não pode ser negativo")
			},
		},
		{
			name: "deve rejeitar conn max idle time maior que conn max lifetime",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.DBConfig = configs.DBConfig{
						MaxConns:        2,
						MinConns:        1,
						MaxIdleConns:    1,
						ConnMaxLifetime: 30 * time.Second,
						ConnMaxIdleTime: time.Minute,
					}
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "DB_CONN_MAX_IDLE_TIME não pode ser maior que DB_CONN_MAX_LIFETIME")
			},
		},
		{
			name: "deve validar kiwify http timeout invalido",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBillingEnabledConfig()
					cfg.KiwifyConfig.HTTPTimeout = 2 * time.Minute
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "KIWIFY_HTTP_TIMEOUT inválido")
			},
		},
		{
			name: "deve validar kiwify retry attempts invalido",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBillingEnabledConfig()
					cfg.KiwifyConfig.HTTPRetryMaxAttempts = 50
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "KIWIFY_HTTP_RETRY_MAX_ATTEMPTS inválido")
			},
		},
		{
			name: "deve validar kiwify retry backoff obrigatorio quando attempts configurado",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBillingEnabledConfig()
					cfg.KiwifyConfig.HTTPRetryBackoff = 0
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "KIWIFY_HTTP_RETRY_BACKOFF inválido")
			},
		},
		{
			name: "deve validar production kiwify secrets ausentes quando habilitado",
			args: args{
				build: func() *configs.Config {
					cfg := s.newProductionConfig()
					cfg.KiwifyConfig.ClientID = "some-client-id"
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "KIWIFY_WEBHOOK_SECRET é obrigatório", "KIWIFY_ACCOUNT_ID é obrigatório")
			},
		},
		{
			name: "deve permitir production sem kiwify configurado",
			args: args{
				build: func() *configs.Config {
					cfg := s.newProductionConfig()
					cfg.KiwifyConfig = configs.KiwifyConfig{}
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve validar billing cache capacidade invalida",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBillingEnabledConfig()
					cfg.BillingConfig.EntitlementCacheCapacity = 500
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "BILLING_ENTITLEMENT_CACHE_CAPACITY inválido")
			},
		},
		{
			name: "deve validar billing cache ttl invalida",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBillingEnabledConfig()
					cfg.BillingConfig.EntitlementCacheTTL = 2 * time.Hour
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "BILLING_ENTITLEMENT_CACHE_TTL inválido")
			},
		},
		{
			name: "deve validar billing rate limit invalido",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBillingEnabledConfig()
					cfg.KiwifyConfig.RateLimitMaxRequestsPerMin = 0
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "KIWIFY_RATE_LIMIT_MAX_REQUESTS_PER_MIN inválido")
			},
		},
		{
			name: "deve rejeitar production sem gateway secret current",
			args: args{
				build: func() *configs.Config {
					cfg := s.newProductionConfig()
					cfg.IdentityConfig.GatewaySharedSecretCurrent = ""
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "IDENTITY_GATEWAY_SHARED_SECRET_CURRENT é obrigatório em production")
			},
		},
		{
			name: "deve rejeitar production com gateway secret com entropia insuficiente (menos de 32 bytes)",
			args: args{
				build: func() *configs.Config {
					cfg := s.newProductionConfig()
					cfg.IdentityConfig.GatewaySharedSecretCurrent = strings.Repeat("a1", 16)
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "IDENTITY_GATEWAY_SHARED_SECRET_CURRENT deve ter ao menos 32 bytes")
			},
		},
		{
			name: "deve rejeitar production com gateway secret hex invalido",
			args: args{
				build: func() *configs.Config {
					cfg := s.newProductionConfig()
					cfg.IdentityConfig.GatewaySharedSecretCurrent = strings.Repeat("ZZ", 32)
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "IDENTITY_GATEWAY_SHARED_SECRET_CURRENT deve ser hex válido")
			},
		},
		{
			name: "deve aceitar production com gateway secret de exatamente 32 bytes",
			args: args{
				build: func() *configs.Config {
					cfg := s.newProductionConfig()
					cfg.IdentityConfig.GatewaySharedSecretCurrent = strings.Repeat("a1", 32)
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve aceitar production com gateway secret next opcional ausente",
			args: args{
				build: func() *configs.Config {
					cfg := s.newProductionConfig()
					cfg.IdentityConfig.GatewaySharedSecretNext = ""
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve rejeitar production com gateway secret next com entropia insuficiente",
			args: args{
				build: func() *configs.Config {
					cfg := s.newProductionConfig()
					cfg.IdentityConfig.GatewaySharedSecretNext = strings.Repeat("b2", 16)
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "IDENTITY_GATEWAY_SHARED_SECRET_NEXT deve ter ao menos 32 bytes")
			},
		},
		{
			name: "deve rejeitar production com gateway secret next hex invalido",
			args: args{
				build: func() *configs.Config {
					cfg := s.newProductionConfig()
					cfg.IdentityConfig.GatewaySharedSecretNext = strings.Repeat("ZZ", 32)
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "IDENTITY_GATEWAY_SHARED_SECRET_NEXT deve ser hex válido")
			},
		},
		{
			name: "deve aceitar non-production sem gateway secret",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.IdentityConfig.GatewaySharedSecretCurrent = ""
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve rejeitar non-production com gateway secret current invalido",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.IdentityConfig.GatewaySharedSecretCurrent = "zz"
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "IDENTITY_GATEWAY_SHARED_SECRET_CURRENT deve ser hex válido")
			},
		},
		{
			name: "ValidateCORS_deve rejeitar production com CORS_ALLOWED_ORIGINS vazio",
			args: args{
				build: func() *configs.Config {
					cfg := s.newProductionConfig()
					cfg.HTTPConfig.CORSAllowedOrigins = ""
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "CORS_ALLOWED_ORIGINS obrigatorio em production")
			},
		},
		{
			name: "ValidateCORS_deve rejeitar production com CORS_ALLOWED_ORIGINS=*",
			args: args{
				build: func() *configs.Config {
					cfg := s.newProductionConfig()
					cfg.HTTPConfig.CORSAllowedOrigins = "*"
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "CORS_ALLOWED_ORIGINS=* proibido em production")
			},
		},
		{
			name: "ValidateCORS_deve aceitar production com CORS_ALLOWED_ORIGINS lista valida",
			args: args{
				build: func() *configs.Config {
					cfg := s.newProductionConfig()
					cfg.HTTPConfig.CORSAllowedOrigins = "https://app.mecontrola.com.br,https://checkout.mecontrola.com.br"
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.NoError(err)
			},
		},
		{
			name: "ValidateCORS_deve aceitar development com CORS_ALLOWED_ORIGINS qualquer valor",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.HTTPConfig.CORSAllowedOrigins = "*"
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve rejeitar rate limit auth por usuario zerado",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.AuthRateLimit.PerUserPerMin = 0
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "AUTH_RATE_LIMIT_PER_USER_PER_MIN deve ser maior que zero")
			},
		},
		{
			name: "deve rejeitar rate limit webhook burst zerado",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.WhatsAppConfig.WebhookRateLimitBurst = 0
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "WHATSAPP_WEBHOOK_RATE_LIMIT_BURST deve ser maior que zero")
			},
		},
		{
			name: "deve rejeitar card closing offset days zero",
			args: args{
				build: func() *configs.Config {
					cfg := s.newProductionConfig()
					cfg.OnboardingConfig.CardClosingOffsetDays = 0
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "ONBOARDING_CARD_CLOSING_OFFSET_DAYS inválido")
			},
		},
		{
			name: "deve rejeitar card closing offset days negativo",
			args: args{
				build: func() *configs.Config {
					cfg := s.newProductionConfig()
					cfg.OnboardingConfig.CardClosingOffsetDays = -5
					return cfg
				},
			},
			setup: func() {},
			expect: func(_ *configs.Config, err error) {
				s.assertConfigError(err, "ONBOARDING_CARD_CLOSING_OFFSET_DAYS inválido")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.setup()
			cfg := scenario.args.build()
			err := cfg.Validate()
			scenario.expect(cfg, err)
		})
	}
}

func (s *ConfigSuite) TestLoadConfig() {
	type args struct {
		path func() string
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(path string)
		expect func(cfg *configs.Config, err error)
	}{
		{
			name: "deve carregar config com arquivo valido",
			args: args{
				path: func() string {
					return "./testdata/valid"
				},
			},
			setup: func(_ string) {},
			expect: func(cfg *configs.Config, err error) {
				s.Require().NoError(err)
				s.Require().NotNil(cfg)
				s.Equal("local", cfg.AppConfig.Environment)
				s.Equal(8080, cfg.HTTPConfig.Port)
				s.Equal("localhost", cfg.DBConfig.Host)
				s.Equal(1.0, cfg.O11yConfig.TraceSampleRate)
			},
		},
		{
			name: "deve retornar erro quando local sem arquivo env",
			args: args{
				path: func() string {
					return s.T().TempDir()
				},
			},
			setup: func(_ string) {
				s.T().Setenv("ENVIRONMENT", "local")
			},
			expect: func(cfg *configs.Config, err error) {
				s.Error(err)
				s.Nil(cfg)
				s.ErrorContains(err, "arquivo .env obrigatório não encontrado")
			},
		},
		{
			name: "deve carregar production sem arquivo usando env vars",
			args: args{
				path: func() string {
					return s.T().TempDir()
				},
			},
			setup: func(_ string) {
				s.T().Setenv("ENVIRONMENT", "production")
				s.T().Setenv("PORT", "8080")
				s.T().Setenv("DB_HOST", "db.fly.internal")
				s.T().Setenv("DB_PORT", "5432")
				s.T().Setenv("DB_USER", "mecontrola")
				s.T().Setenv("DB_PASSWORD", "productionStrongPassword123!")
				s.T().Setenv("DB_NAME", "mecontrola_db")
				s.T().Setenv("DB_SSL_MODE", "require")
				s.T().Setenv("OTEL_TRACE_SAMPLE_RATE", "0.2")
				s.T().Setenv("SERVICE_NAME_API", "mecontrola-api")
				s.T().Setenv("IDENTITY_GATEWAY_SHARED_SECRET_CURRENT", strings.Repeat("a1", 32))
				s.T().Setenv("CORS_ALLOWED_ORIGINS", "https://app.mecontrola.com.br,https://checkout.mecontrola.com.br")
				s.T().Setenv("META_ACCESS_TOKEN", "EAAreal-access-token-for-testing")
				s.T().Setenv("META_PHONE_NUMBER_ID", "1234567890123")
				s.T().Setenv("META_APP_SECRET", "real-app-secret-for-testing")
				s.T().Setenv("META_VERIFY_TOKEN", "real-verify-token-for-testing")
				s.T().Setenv("ONBOARDING_TOKEN_ENCRYPTION_KEY", "testencryptionkey1234567890abcde")
				s.T().Setenv("OPENROUTER_API_KEY", "sk-real-key-for-testing")
			},
			expect: func(cfg *configs.Config, err error) {
				s.Require().NoError(err)
				s.Require().NotNil(cfg)
				s.Equal("production", cfg.AppConfig.Environment)
				s.Equal("db.fly.internal", cfg.DBConfig.Host)
			},
		},
		{
			name: "deve retornar erro quando production fixture e insegura",
			args: args{
				path: func() string {
					return "./testdata/insecure-prod"
				},
			},
			setup: func(_ string) {},
			expect: func(cfg *configs.Config, err error) {
				s.Error(err)
				s.Nil(cfg)
				s.ErrorContains(err, "placeholder inseguro")
			},
		},
		{
			name: "deve aplicar defaults do pool de banco",
			args: args{
				path: func() string {
					return s.T().TempDir()
				},
			},
			setup: func(path string) {
				s.writeEnvFile(path, s.minimalLocalEnv())
			},
			expect: func(cfg *configs.Config, err error) {
				s.Require().NoError(err)
				s.Require().NotNil(cfg)
				s.Equal(30*time.Minute, cfg.DBConfig.ConnMaxLifetime)
				s.Equal(5*time.Minute, cfg.DBConfig.ConnMaxIdleTime)
			},
		},
		{
			name: "deve aplicar defaults do outbox",
			args: args{
				path: func() string {
					return s.T().TempDir()
				},
			},
			setup: func(path string) {
				s.writeEnvFile(path, s.minimalLocalEnv())
			},
			expect: func(cfg *configs.Config, err error) {
				s.Require().NoError(err)
				s.Require().NotNil(cfg)
				s.Equal(true, cfg.OutboxConfig.DispatcherEnabled)
				s.Equal(500*time.Millisecond, cfg.OutboxConfig.DispatcherTickInterval)
				s.Equal(50, cfg.OutboxConfig.DispatcherBatchSize)
				s.Equal(10*time.Second, cfg.OutboxConfig.DispatcherHandlerTimeout)
				s.Equal(3, cfg.OutboxConfig.RetryMaxAttempts)
				s.Equal(2*time.Second, cfg.OutboxConfig.RetryBaseBackoff)
				s.Equal(5*time.Minute, cfg.OutboxConfig.RetryMaxBackoff)
				s.Equal(90, cfg.OutboxConfig.HousekeepingRetentionDays)
				s.Equal("@daily", cfg.OutboxConfig.HousekeepingSchedule)
				s.Equal("@every 1m", cfg.OutboxConfig.ReaperInterval)
				s.Equal(5*time.Minute, cfg.OutboxConfig.ReaperStuckAfter)
			},
		},
		{
			name: "deve sobrescrever defaults do outbox via env",
			args: args{
				path: func() string {
					return s.T().TempDir()
				},
			},
			setup: func(path string) {
				s.T().Setenv("OUTBOX_DISPATCHER_ENABLED", "false")
				s.T().Setenv("OUTBOX_DISPATCHER_BATCH_SIZE", "100")
				s.T().Setenv("OUTBOX_RETRY_MAX_ATTEMPTS", "3")
				s.T().Setenv("OUTBOX_HOUSEKEEPING_RETENTION_DAYS", "30")
				s.T().Setenv("OUTBOX_HOUSEKEEPING_SCHEDULE", "@weekly")
				s.T().Setenv("OUTBOX_REAPER_INTERVAL", "@every 5m")
				s.writeEnvFile(path, s.minimalLocalEnv())
			},
			expect: func(cfg *configs.Config, err error) {
				s.Require().NoError(err)
				s.Require().NotNil(cfg)
				s.Equal(false, cfg.OutboxConfig.DispatcherEnabled)
				s.Equal(100, cfg.OutboxConfig.DispatcherBatchSize)
				s.Equal(3, cfg.OutboxConfig.RetryMaxAttempts)
				s.Equal(30, cfg.OutboxConfig.HousekeepingRetentionDays)
				s.Equal("@weekly", cfg.OutboxConfig.HousekeepingSchedule)
				s.Equal("@every 5m", cfg.OutboxConfig.ReaperInterval)
			},
		},
		{
			name: "deve retornar erro quando porta carregada e invalida",
			args: args{
				path: func() string {
					return s.T().TempDir()
				},
			},
			setup: func(path string) {
				s.writeEnvFile(path, "ENVIRONMENT=local\nPORT=99999\nOTEL_TRACE_SAMPLE_RATE=1.0\n")
			},
			expect: func(cfg *configs.Config, err error) {
				s.Error(err)
				s.Nil(cfg)
				s.ErrorContains(err, "PORT inválido")
			},
		},
		{
			name: "deve retornar erro quando trace sample rate carregado e invalido",
			args: args{
				path: func() string {
					return s.T().TempDir()
				},
			},
			setup: func(path string) {
				s.writeEnvFile(path, "ENVIRONMENT=local\nPORT=8080\nOTEL_TRACE_SAMPLE_RATE=2.5\n")
			},
			expect: func(cfg *configs.Config, err error) {
				s.Error(err)
				s.Nil(cfg)
				s.ErrorContains(err, "OTEL_TRACE_SAMPLE_RATE inválido")
			},
		},
		{
			name: "deve aplicar defaults da kiwify",
			args: args{
				path: func() string {
					return s.T().TempDir()
				},
			},
			setup: func(path string) {
				s.T().Setenv("KIWIFY_ACCOUNT_ID", "account-test")
				s.writeEnvFile(path, s.minimalLocalEnv())
			},
			expect: func(cfg *configs.Config, err error) {
				s.Require().NoError(err)
				s.Require().NotNil(cfg)
				s.Equal("https://public-api.kiwify.com", cfg.KiwifyConfig.APIBaseURL)
				s.Equal("account-test", cfg.KiwifyConfig.AccountID)
				s.Equal("X-Kiwify-Webhook-Token", cfg.KiwifyConfig.WebhookTokenHeader)
				s.Equal(5*time.Minute, cfg.KiwifyConfig.OAuthTokenSafetyMargin)
				s.Equal(100, cfg.KiwifyConfig.RateLimitMaxRequestsPerMin)
				s.Equal(10, cfg.KiwifyConfig.RateLimitBurst)
				s.Equal("@hourly", cfg.KiwifyConfig.ReconciliationInterval)
				s.Equal(200, cfg.KiwifyConfig.ReconciliationBatchSize)
				s.Equal(10*time.Second, cfg.KiwifyConfig.HTTPTimeout)
				s.Equal(3, cfg.KiwifyConfig.HTTPRetryMaxAttempts)
				s.Equal(time.Second, cfg.KiwifyConfig.HTTPRetryBackoff)
			},
		},
		{
			name: "deve aplicar defaults do billing",
			args: args{
				path: func() string {
					return s.T().TempDir()
				},
			},
			setup: func(path string) {
				s.writeEnvFile(path, s.minimalLocalEnv())
			},
			expect: func(cfg *configs.Config, err error) {
				s.Require().NoError(err)
				s.Require().NotNil(cfg)
				s.Equal(50000, cfg.BillingConfig.EntitlementCacheCapacity)
				s.Equal(5*time.Minute, cfg.BillingConfig.EntitlementCacheTTL)
				s.Equal("@daily", cfg.BillingConfig.AnonymizationSchedule)
				s.Equal(500, cfg.BillingConfig.AnonymizationBatchSize)
				s.Equal(365, cfg.BillingConfig.AnonymizationRetentionDays)
			},
		},
		{
			name: "deve manter ONBOARDING_CARD_CLOSING_OFFSET_DAYS quando definido",
			args: args{
				path: func() string {
					return s.T().TempDir()
				},
			},
			setup: func(path string) {
				s.writeEnvFile(path, "ENVIRONMENT=local\nPORT=8080\nOTEL_TRACE_SAMPLE_RATE=1.0\nONBOARDING_CARD_CLOSING_OFFSET_DAYS=15\n")
			},
			expect: func(cfg *configs.Config, err error) {
				s.Require().NoError(err)
				s.Require().NotNil(cfg)
				s.Equal(15, cfg.OnboardingConfig.CardClosingOffsetDays)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			path := scenario.args.path()
			loadConfig := configs.LoadConfig
			scenario.setup(path)
			cfg, err := loadConfig(path)
			scenario.expect(cfg, err)
		})
	}
}

func (s *ConfigSuite) TestLoadConfigActivationDefaults() {
	type args struct {
		path func() string
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(path string)
		expect func(cfg *configs.Config, err error)
	}{
		{
			name: "deve aplicar defaults das chaves de ativacao em OnboardingConfig",
			args: args{
				path: func() string {
					return s.T().TempDir()
				},
			},
			setup: func(path string) {
				s.writeEnvFile(path, s.minimalLocalEnv())
			},
			expect: func(cfg *configs.Config, err error) {
				s.Require().NoError(err)
				s.Require().NotNil(cfg)
				s.Equal(24, cfg.OnboardingConfig.ActivationWindowHours)
				s.Equal("https://mecontrola.app.br", cfg.OnboardingConfig.ActivationPageURL)
				s.Equal("@daily", cfg.OnboardingConfig.ActivationNoMatchThrottleHousekeepingSchedule)
				s.Equal(7, cfg.OnboardingConfig.ActivationNoMatchThrottleRetentionDays)
				s.Equal(500, cfg.OnboardingConfig.ActivationNoMatchThrottleBatch)
			},
		},
		{
			name: "deve aplicar default de WA_MSG_ACTIVATION_NOT_FOUND em WhatsAppConfig",
			args: args{
				path: func() string {
					return s.T().TempDir()
				},
			},
			setup: func(path string) {
				s.writeEnvFile(path, s.minimalLocalEnv())
			},
			expect: func(cfg *configs.Config, err error) {
				s.Require().NoError(err)
				s.Require().NotNil(cfg)
				s.NotEmpty(cfg.WhatsAppConfig.ActivationNotFound)
			},
		},
		{
			name: "deve sobrescrever ONBOARDING_ACTIVATION_WINDOW_HOURS via env",
			args: args{
				path: func() string {
					return s.T().TempDir()
				},
			},
			setup: func(path string) {
				s.T().Setenv("ONBOARDING_ACTIVATION_WINDOW_HOURS", "48")
				s.writeEnvFile(path, s.minimalLocalEnv())
			},
			expect: func(cfg *configs.Config, err error) {
				s.Require().NoError(err)
				s.Require().NotNil(cfg)
				s.Equal(48, cfg.OnboardingConfig.ActivationWindowHours)
			},
		},
		{
			name: "deve sobrescrever ONBOARDING_ACTIVATION_PAGE_URL via env",
			args: args{
				path: func() string {
					return s.T().TempDir()
				},
			},
			setup: func(path string) {
				s.T().Setenv("ONBOARDING_ACTIVATION_PAGE_URL", "https://staging.mecontrola.app.br")
				s.writeEnvFile(path, s.minimalLocalEnv())
			},
			expect: func(cfg *configs.Config, err error) {
				s.Require().NoError(err)
				s.Require().NotNil(cfg)
				s.Equal("https://staging.mecontrola.app.br", cfg.OnboardingConfig.ActivationPageURL)
			},
		},
		{
			name: "deve sobrescrever WA_MSG_ACTIVATION_NOT_FOUND via env",
			args: args{
				path: func() string {
					return s.T().TempDir()
				},
			},
			setup: func(path string) {
				s.T().Setenv("WA_MSG_ACTIVATION_NOT_FOUND", "Numero nao reconhecido. Fale com o suporte.")
				s.writeEnvFile(path, s.minimalLocalEnv())
			},
			expect: func(cfg *configs.Config, err error) {
				s.Require().NoError(err)
				s.Require().NotNil(cfg)
				s.Equal("Numero nao reconhecido. Fale com o suporte.", cfg.WhatsAppConfig.ActivationNotFound)
			},
		},
		{
			name: "deve sobrescrever throttle housekeeping via env",
			args: args{
				path: func() string {
					return s.T().TempDir()
				},
			},
			setup: func(path string) {
				s.T().Setenv("ONBOARDING_ACTIVATION_NOMATCH_THROTTLE_HOUSEKEEPING_SCHEDULE", "@weekly")
				s.T().Setenv("ONBOARDING_ACTIVATION_NOMATCH_THROTTLE_RETENTION_DAYS", "14")
				s.T().Setenv("ONBOARDING_ACTIVATION_NOMATCH_THROTTLE_BATCH", "1000")
				s.writeEnvFile(path, s.minimalLocalEnv())
			},
			expect: func(cfg *configs.Config, err error) {
				s.Require().NoError(err)
				s.Require().NotNil(cfg)
				s.Equal("@weekly", cfg.OnboardingConfig.ActivationNoMatchThrottleHousekeepingSchedule)
				s.Equal(14, cfg.OnboardingConfig.ActivationNoMatchThrottleRetentionDays)
				s.Equal(1000, cfg.OnboardingConfig.ActivationNoMatchThrottleBatch)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			path := scenario.args.path()
			scenario.setup(path)
			cfg, err := configs.LoadConfig(path)
			scenario.expect(cfg, err)
		})
	}
}

func (s *ConfigSuite) TestNormalizedExporterEndpoint() {
	scenarios := []struct {
		name     string
		config   configs.O11yConfig
		expected string
	}{
		{
			name: "deve remover scheme em grpc",
			config: configs.O11yConfig{
				ExporterProtocol: "grpc",
				ExporterEndpoint: "http://localhost:4317",
			},
			expected: "localhost:4317",
		},
		{
			name: "deve remover trailing slash em grpc",
			config: configs.O11yConfig{
				ExporterProtocol: "grpc",
				ExporterEndpoint: "https://otel-lgtm:4317/",
			},
			expected: "otel-lgtm:4317",
		},
		{
			name: "deve preservar endpoint http",
			config: configs.O11yConfig{
				ExporterProtocol: "http/protobuf",
				ExporterEndpoint: "http://localhost:4318",
			},
			expected: "http://localhost:4318",
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.expected, scenario.config.NormalizedExporterEndpoint())
		})
	}
}

func (s *ConfigSuite) TestDBConfigAccessors() {
	type args struct {
		build func() *configs.DBConfig
		call  func(db *configs.DBConfig) string
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func()
		expect func(result string)
	}{
		{
			name: "deve ocultar senha no safe dsn com senha alfanumerica",
			args: args{
				build: func() *configs.DBConfig {
					return &configs.DBConfig{
						Host:     "localhost",
						Port:     5432,
						User:     "user",
						Password: "Senha123!",
						Name:     "dbname",
						SSLMode:  "disable",
					}
				},
				call: func(db *configs.DBConfig) string {
					return db.SafeDSN()
				},
			},
			setup: func() {},
			expect: func(result string) {
				s.NotContains(result, "Senha123!")
				s.Contains(result, "***")
			},
		},
		{
			name: "deve ocultar senha no safe dsn com simbolos",
			args: args{
				build: func() *configs.DBConfig {
					return &configs.DBConfig{
						Host:     "localhost",
						Port:     5432,
						User:     "user",
						Password: "sup3r-secret@2026",
						Name:     "dbname",
						SSLMode:  "disable",
					}
				},
				call: func(db *configs.DBConfig) string {
					return db.SafeDSN()
				},
			},
			setup: func() {},
			expect: func(result string) {
				s.NotContains(result, "sup3r-secret@2026")
				s.Contains(result, "***")
			},
		},
		{
			name: "deve manter formato esperado no safe dsn",
			args: args{
				build: func() *configs.DBConfig {
					return &configs.DBConfig{
						Host:     "db.example.com",
						Port:     5432,
						User:     "mecontrola",
						Password: "anypassword",
						Name:     "mecontrola_db",
						SSLMode:  "require",
					}
				},
				call: func(db *configs.DBConfig) string {
					return db.SafeDSN()
				},
			},
			setup: func() {},
			expect: func(result string) {
				s.Equal("postgres://mecontrola:***@db.example.com:5432/mecontrola_db?sslmode=require&search_path=mecontrola,public", result)
			},
		},
		{
			name: "deve incluir senha em texto claro no dsn",
			args: args{
				build: func() *configs.DBConfig {
					return &configs.DBConfig{
						Host:     "db.example.com",
						Port:     5432,
						User:     "mecontrola",
						Password: "supersecretpassword",
						Name:     "mecontrola_db",
						SSLMode:  "require",
					}
				},
				call: func(db *configs.DBConfig) string {
					return db.DSN()
				},
			},
			setup: func() {},
			expect: func(result string) {
				s.Contains(result, "supersecretpassword")
				s.Contains(result, "postgres://mecontrola:supersecretpassword@db.example.com:5432/mecontrola_db?sslmode=require&search_path=mecontrola,public")
			},
		},
		{
			name: "deve usar mesmo search path no migration dsn",
			args: args{
				build: func() *configs.DBConfig {
					return &configs.DBConfig{
						Host:     "db.example.com",
						Port:     5432,
						User:     "mecontrola",
						Password: "supersecretpassword",
						Name:     "mecontrola_db",
						SSLMode:  "require",
					}
				},
				call: func(db *configs.DBConfig) string {
					return db.MigrationDSN()
				},
			},
			setup: func() {},
			expect: func(result string) {
				s.Equal("postgres://mecontrola:supersecretpassword@db.example.com:5432/mecontrola_db?sslmode=require&search_path=mecontrola,public&x-migrations-table=%22mecontrola%22.%22schema_migrations%22&x-migrations-table-quoted=true", result)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.setup()
			db := scenario.args.build()
			result := scenario.args.call(db)
			scenario.expect(result)
		})
	}
}

func (s *ConfigSuite) TestInsecurePlaceholders() {
	type args struct {
		values func() []string
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func()
		expect func(values []string)
	}{
		{
			name: "deve manter lista de placeholders nao vazia",
			args: args{
				values: func() []string {
					return configs.InsecurePlaceholders
				},
			},
			setup: func() {},
			expect: func(values []string) {
				s.NotEmpty(values)
			},
		},
		{
			name: "deve conter placeholders conhecidos",
			args: args{
				values: func() []string {
					return configs.InsecurePlaceholders
				},
			},
			setup: func() {},
			expect: func(values []string) {
				s.Contains(values, "CHANGE_ME_USE_STRONG_PASSWORD")
				s.Contains(values, "CHANGE_ME_GENERATE_SECURE_SECRET_KEY_MIN_64_CHARS")
				s.Contains(values, "your_secret_key")
				s.Contains(values, "financial@password")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.setup()
			values := scenario.args.values()
			scenario.expect(values)
		})
	}
}

func (s *ConfigSuite) TestKiwifySafe() {
	type args struct {
		build func() configs.KiwifyConfig
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func()
		expect func(safe map[string]any)
	}{
		{
			name: "deve manter valores http seguros",
			args: args{
				build: func() configs.KiwifyConfig {
					return configs.KiwifyConfig{
						HTTPTimeout:          15 * time.Second,
						HTTPRetryMaxAttempts: 5,
						HTTPRetryBackoff:     2 * time.Second,
					}
				},
			},
			setup: func() {},
			expect: func(safe map[string]any) {
				s.Equal("15s", safe["http_timeout"])
				s.Equal(5, safe["http_retry_max_attempts"])
				s.Equal("2s", safe["http_retry_backoff"])
			},
		},
		{
			name: "deve redactar secrets e ids configurados",
			args: args{
				build: func() configs.KiwifyConfig {
					return configs.KiwifyConfig{
						APIBaseURL:                 "https://public-api.kiwify.com",
						AccountID:                  "account-id-value",
						WebhookSecret:              "super-secret-webhook",
						WebhookTokenHeader:         "X-Kiwify-Webhook-Token",
						ClientID:                   "client-id-value",
						ClientSecret:               "super-secret-client",
						RateLimitMaxRequestsPerMin: 100,
						ReconciliationInterval:     "@hourly",
						ReconciliationBatchSize:    200,
					}
				},
			},
			setup: func() {},
			expect: func(safe map[string]any) {
				s.Equal("https://public-api.kiwify.com", safe["api_base_url"])
				s.Equal(true, safe["account_id_set"])
				s.Equal(false, safe["product_id_monthly_set"])
				s.Equal(false, safe["product_id_quarterly_set"])
				s.Equal(false, safe["product_id_annual_set"])
				s.Equal("X-Kiwify-Webhook-Token", safe["webhook_token_header"])
				s.Equal(100, safe["rate_limit"])
				s.Equal("@hourly", safe["reconciliation_interval"])
				s.Equal(200, safe["reconciliation_batch_size"])
				s.Equal(true, safe["client_id_set"])
				s.Equal(true, safe["client_secret_set"])
				s.Equal(true, safe["webhook_secret_set"])
				for key, value := range safe {
					strValue, ok := value.(string)
					if !ok {
						continue
					}
					s.NotContains(strValue, "super-secret-webhook", key)
					s.NotContains(strValue, "super-secret-client", key)
					s.NotContains(strValue, "client-id-value", key)
					s.NotContains(strValue, "account-id-value", key)
				}
			},
		},
		{
			name: "deve indicar secrets nao configurados",
			args: args{
				build: func() configs.KiwifyConfig {
					return configs.KiwifyConfig{
						APIBaseURL: "https://public-api.kiwify.com",
					}
				},
			},
			setup: func() {},
			expect: func(safe map[string]any) {
				s.Equal(false, safe["client_id_set"])
				s.Equal(false, safe["account_id_set"])
				s.Equal(false, safe["client_secret_set"])
				s.Equal(false, safe["webhook_secret_set"])
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.setup()
			kiwifyConfig := scenario.args.build()
			safe := kiwifyConfig.Safe()
			scenario.expect(safe)
		})
	}
}

func (s *ConfigSuite) TestValidateForMigrate() {
	s.Run("aceita production sem secrets de app que Validate rejeita", func() {
		cfg := s.newBaseConfig()
		cfg.AppConfig.Environment = "production"
		cfg.O11yConfig.TraceSampleRate = 0.2

		s.Error(cfg.Validate())
		s.NoError(cfg.ValidateForMigrate())
	})

	s.Run("rejeita environment invalido", func() {
		cfg := s.newBaseConfig()
		cfg.AppConfig.Environment = "dev"

		s.assertConfigError(cfg.ValidateForMigrate(), "ENVIRONMENT inválido")
	})

	s.Run("rejeita trace sample rate fora do intervalo", func() {
		cfg := s.newBaseConfig()
		cfg.O11yConfig.TraceSampleRate = 2

		s.assertConfigError(cfg.ValidateForMigrate(), "OTEL_TRACE_SAMPLE_RATE inválido")
	})

	s.Run("rejeita pool tunables invalidos", func() {
		cfg := s.newBaseConfig()
		cfg.DBConfig.MaxConns = 5
		cfg.DBConfig.MinConns = 10

		s.assertConfigError(cfg.ValidateForMigrate(), "DB_MIN_CONNS não pode ser maior que DB_MAX_CONNS")
	})
}

func (s *ConfigSuite) assertConfigError(err error, messages ...string) {
	s.Require().Error(err)
	for _, message := range messages {
		s.ErrorContains(err, message)
	}
}

func (s *ConfigSuite) newBaseConfig() *configs.Config {
	return &configs.Config{
		AppConfig:     configs.AppConfig{Environment: "local", AppMode: "server"},
		HTTPConfig:    configs.HTTPConfig{Port: 8080},
		DBConfig:      configs.DBConfig{Password: "qualquer", User: "user"},
		O11yConfig:    configs.O11yConfig{TraceSampleRate: 1},
		AuthRateLimit: configs.AuthRateLimitConfig{PerUserPerMin: 120, PerUserBurst: 30},
		WhatsAppConfig: configs.WhatsAppConfig{
			WebhookRateLimitPerMin: 600,
			WebhookRateLimitBurst:  120,
		},
		WorkflowKernelConfig: s.newValidWorkflowKernelConfig(),
		OnboardingConfig: configs.OnboardingConfig{
			AbandonmentTTLHours:    48,
			AbandonmentJobSchedule: "@hourly",
			AbandonmentBatchSize:   100,
		},
	}
}

func (s *ConfigSuite) newProductionConfig() *configs.Config {
	cfg := s.newBaseConfig()
	cfg.AppConfig.Environment = "production"
	cfg.DBConfig.Password = "productionStrongPassword123!"
	cfg.DBConfig.User = "mecontrola"
	cfg.O11yConfig.TraceSampleRate = 0.2
	cfg.IdentityConfig.GatewaySharedSecretCurrent = strings.Repeat("a1", 32)
	cfg.HTTPConfig.CORSAllowedOrigins = "https://app.mecontrola.com.br,https://checkout.mecontrola.com.br"
	cfg.WhatsAppConfig.AccessToken = "EAAreal-access-token-for-testing"
	cfg.WhatsAppConfig.PhoneNumberID = "1234567890123"
	cfg.WhatsAppConfig.AppSecret = "real-app-secret-for-testing"
	cfg.WhatsAppConfig.VerifyToken = "real-verify-token-for-testing"
	cfg.WhatsAppConfig.DedupHousekeepingSchedule = "@daily"
	cfg.WhatsAppConfig.DedupHousekeepingRetentionDays = 30
	cfg.WhatsAppConfig.DedupHousekeepingBatch = 10000
	cfg.OnboardingConfig.TokenEncryptionKey = "testencryptionkey1234567890abcde"
	cfg.AgentConfig.OpenRouterAPIKey = "sk-real-key-for-testing"
	cfg.AgentConfig.PrimaryModel = "openai/gpt-4o-mini"
	cfg.AgentConfig.MaxTokens = 256
	cfg.AgentConfig.MecontrolaMaxTokens = 3072
	cfg.OnboardingConfig.AbandonmentTTLHours = 48
	cfg.OnboardingConfig.AbandonmentJobSchedule = "@hourly"
	cfg.OnboardingConfig.AbandonmentBatchSize = 100
	cfg.OnboardingConfig.CardClosingOffsetDays = 10
	return cfg
}

func (s *ConfigSuite) newBillingEnabledConfig() *configs.Config {
	cfg := s.newBaseConfig()
	cfg.BillingConfig = configs.BillingConfig{
		EntitlementCacheCapacity:   50000,
		EntitlementCacheTTL:        5 * time.Minute,
		AnonymizationBatchSize:     500,
		AnonymizationRetentionDays: 365,
	}
	cfg.KiwifyConfig = configs.KiwifyConfig{
		RateLimitMaxRequestsPerMin: 100,
		HTTPTimeout:                10 * time.Second,
		HTTPRetryMaxAttempts:       3,
		HTTPRetryBackoff:           time.Second,
	}
	return cfg
}

func (s *ConfigSuite) newValidOutboxConfig() configs.OutboxConfig {
	return configs.OutboxConfig{
		RetryMaxAttempts:          3,
		DispatcherBatchSize:       50,
		HousekeepingRetentionDays: 90,
		HousekeepingSchedule:      "@daily",
		ReaperInterval:            "@every 1m",
	}
}

func (s *ConfigSuite) minimalLocalEnv() string {
	return "ENVIRONMENT=local\nPORT=8080\nOTEL_TRACE_SAMPLE_RATE=1.0\n"
}

func (s *ConfigSuite) writeEnvFile(path, content string) {
	err := os.WriteFile(path+"/.env", []byte(content), 0o600)
	s.Require().NoError(err)
}

func (s *ConfigSuite) newValidWorkflowKernelConfig() configs.WorkflowKernelConfig {
	return configs.WorkflowKernelConfig{
		MaxAttempts:               3,
		RetryBaseBackoff:          200 * time.Millisecond,
		RetryMaxBackoff:           5 * time.Second,
		HousekeepingRetentionDays: 30,
		HousekeepingSchedule:      "@daily",
		HousekeepingBatchSize:     500,
	}
}

func (s *ConfigSuite) TestValidateWorkflowKernel() {
	type args struct {
		build func() *configs.Config
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(err error)
	}{
		{
			name: "deve aceitar config valida com defaults",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.WorkflowKernelConfig = s.newValidWorkflowKernelConfig()
					return cfg
				},
			},
			expect: func(err error) { s.NoError(err) },
		},

		{
			name: "deve rejeitar MaxAttempts menor que 1",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.WorkflowKernelConfig = s.newValidWorkflowKernelConfig()
					cfg.WorkflowKernelConfig.MaxAttempts = 0
					return cfg
				},
			},
			expect: func(err error) {
				s.assertConfigError(err, "WORKFLOW_KERNEL_MAX_ATTEMPTS inválido")
			},
		},
		{
			name: "deve rejeitar RetryBaseBackoff zero",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.WorkflowKernelConfig = s.newValidWorkflowKernelConfig()
					cfg.WorkflowKernelConfig.RetryBaseBackoff = 0
					return cfg
				},
			},
			expect: func(err error) {
				s.assertConfigError(err, "WORKFLOW_KERNEL_RETRY_BASE_BACKOFF inválido")
			},
		},
		{
			name: "deve rejeitar RetryMaxBackoff zero",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.WorkflowKernelConfig = s.newValidWorkflowKernelConfig()
					cfg.WorkflowKernelConfig.RetryMaxBackoff = 0
					return cfg
				},
			},
			expect: func(err error) {
				s.assertConfigError(err, "WORKFLOW_KERNEL_RETRY_MAX_BACKOFF inválido")
			},
		},
		{
			name: "deve rejeitar base maior que max backoff",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.WorkflowKernelConfig = s.newValidWorkflowKernelConfig()
					cfg.WorkflowKernelConfig.RetryBaseBackoff = 10 * time.Second
					cfg.WorkflowKernelConfig.RetryMaxBackoff = 1 * time.Second
					return cfg
				},
			},
			expect: func(err error) {
				s.assertConfigError(err, "WORKFLOW_KERNEL_RETRY_BASE_BACKOFF")
				s.assertConfigError(err, "não pode ser maior que WORKFLOW_KERNEL_RETRY_MAX_BACKOFF")
			},
		},
		{
			name: "deve rejeitar HousekeepingRetentionDays menor que 1",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.WorkflowKernelConfig = s.newValidWorkflowKernelConfig()
					cfg.WorkflowKernelConfig.HousekeepingRetentionDays = 0
					return cfg
				},
			},
			expect: func(err error) {
				s.assertConfigError(err, "WORKFLOW_KERNEL_HOUSEKEEPING_RETENTION_DAYS inválido")
			},
		},
		{
			name: "deve rejeitar HousekeepingBatchSize menor que 1",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.WorkflowKernelConfig = s.newValidWorkflowKernelConfig()
					cfg.WorkflowKernelConfig.HousekeepingBatchSize = 0
					return cfg
				},
			},
			expect: func(err error) {
				s.assertConfigError(err, "WORKFLOW_KERNEL_HOUSEKEEPING_BATCH_SIZE inválido")
			},
		},
		{
			name: "deve rejeitar HousekeepingSchedule invalido",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.WorkflowKernelConfig = s.newValidWorkflowKernelConfig()
					cfg.WorkflowKernelConfig.HousekeepingSchedule = "nao-e-cron"
					return cfg
				},
			},
			expect: func(err error) {
				s.assertConfigError(err, "WORKFLOW_KERNEL_HOUSEKEEPING_SCHEDULE inválido")
			},
		},
		{
			name: "deve rejeitar HousekeepingSchedule vazio",
			args: args{
				build: func() *configs.Config {
					cfg := s.newBaseConfig()
					cfg.WorkflowKernelConfig = s.newValidWorkflowKernelConfig()
					cfg.WorkflowKernelConfig.HousekeepingSchedule = ""
					return cfg
				},
			},
			expect: func(err error) {
				s.assertConfigError(err, "WORKFLOW_KERNEL_HOUSEKEEPING_SCHEDULE inválido")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			cfg := scenario.args.build()
			err := cfg.Validate()
			scenario.expect(err)
		})
	}
}

func (s *ConfigSuite) TestLoadConfigReadsSecretsFromFiles() {
	path := s.T().TempDir()
	secretsPath := filepath.Join(path, "secrets")
	s.Require().NoError(os.MkdirAll(secretsPath, 0700))

	s.T().Setenv("MECONTROLA_SECRETS_PATH", secretsPath)
	s.T().Setenv("ENVIRONMENT", "production")
	s.T().Setenv("PORT", "8080")
	s.T().Setenv("DB_HOST", "db")
	s.T().Setenv("DB_PORT", "5432")
	s.T().Setenv("DB_USER", "mecontrola")
	s.T().Setenv("DB_NAME", "mecontrola_db")
	s.T().Setenv("DB_SSL_MODE", "disable")
	s.T().Setenv("OTEL_TRACE_SAMPLE_RATE", "0.1")
	s.T().Setenv("SERVICE_NAME_API", "mecontrola-api")
	s.T().Setenv("CORS_ALLOWED_ORIGINS", "https://app.mecontrola.com.br")
	s.T().Setenv("IDENTITY_GATEWAY_SHARED_SECRET_CURRENT", strings.Repeat("a1", 32))
	s.T().Setenv("META_ACCESS_TOKEN", "from-env-token")
	s.T().Setenv("META_PHONE_NUMBER_ID", "1234567890123")
	s.T().Setenv("META_APP_SECRET", "from-env-secret")
	s.T().Setenv("META_VERIFY_TOKEN", "from-env-verify")
	s.T().Setenv("ONBOARDING_TOKEN_ENCRYPTION_KEY", "testencryptionkey1234567890abcde")

	s.Require().NoError(os.WriteFile(filepath.Join(secretsPath, "DB_PASSWORD"), []byte("from-secret-file\n"), 0600))
	s.Require().NoError(os.WriteFile(filepath.Join(secretsPath, "OPENROUTER_API_KEY"), []byte("from-secret-file\n"), 0600))

	cfg, err := configs.LoadConfig(path)
	s.Require().NoError(err)
	s.Require().NotNil(cfg)
	s.Equal("from-secret-file", cfg.DBConfig.Password)
	s.Equal("from-env-token", cfg.WhatsAppConfig.AccessToken)
	s.Equal("from-secret-file", cfg.AgentConfig.OpenRouterAPIKey)
}

func (s *ConfigSuite) TestLoadConfigFailsWhenSecretFileUnreadable() {
	path := s.T().TempDir()
	secretsPath := filepath.Join(path, "secrets")
	s.Require().NoError(os.MkdirAll(secretsPath, 0700))

	s.T().Setenv("MECONTROLA_SECRETS_PATH", secretsPath)
	s.T().Setenv("ENVIRONMENT", "production")
	s.T().Setenv("PORT", "8080")
	s.T().Setenv("DB_HOST", "db")
	s.T().Setenv("DB_PORT", "5432")
	s.T().Setenv("DB_USER", "mecontrola")
	s.T().Setenv("DB_NAME", "mecontrola_db")
	s.T().Setenv("DB_SSL_MODE", "disable")
	s.T().Setenv("OTEL_TRACE_SAMPLE_RATE", "0.1")
	s.T().Setenv("SERVICE_NAME_API", "mecontrola-api")
	s.T().Setenv("CORS_ALLOWED_ORIGINS", "https://app.mecontrola.com.br")
	s.T().Setenv("IDENTITY_GATEWAY_SHARED_SECRET_CURRENT", strings.Repeat("a1", 32))
	s.T().Setenv("META_ACCESS_TOKEN", "token")
	s.T().Setenv("META_PHONE_NUMBER_ID", "1234567890123")
	s.T().Setenv("META_APP_SECRET", "secret")
	s.T().Setenv("META_VERIFY_TOKEN", "verify")
	s.T().Setenv("ONBOARDING_TOKEN_ENCRYPTION_KEY", "testencryptionkey1234567890abcde")

	// Força erro de leitura criando um diretório com o nome do secret.
	s.Require().NoError(os.Mkdir(filepath.Join(secretsPath, "DB_PASSWORD"), 0700))

	cfg, err := configs.LoadConfig(path)
	s.Require().Error(err)
	s.Nil(cfg)
	s.ErrorContains(err, "carregando secrets")
	s.ErrorContains(err, "DB_PASSWORD")
}
