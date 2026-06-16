package onboarding_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding"
)

type stubDBTX struct{}

func (s *stubDBTX) ExecContext(_ context.Context, _ string, _ ...any) (database.Result, error) {
	return nil, nil
}

func (s *stubDBTX) QueryContext(_ context.Context, _ string, _ ...any) (database.Rows, error) {
	return nil, nil
}

func (s *stubDBTX) QueryRowContext(_ context.Context, _ string, _ ...any) database.Row {
	return nil
}

type stubManager struct{}

func (s *stubManager) Driver() database.Driver              { return "" }
func (s *stubManager) DBTX(_ context.Context) database.DBTX { return &stubDBTX{} }
func (s *stubManager) BeginTx(_ context.Context, _ database.TxOptions) (database.Tx, error) {
	return nil, nil
}
func (s *stubManager) Ping(_ context.Context) error     { return nil }
func (s *stubManager) Shutdown(_ context.Context) error { return nil }

func TestNewOnboardingModule_FieldsNotNil(t *testing.T) {
	module, err := onboarding.NewOnboardingModule(
		manager.Manager(&stubManager{}),
		configs.OnboardingConfig{
			TokenTTLDays:            7,
			OutreachGapHours:        24,
			CheckoutCORSOrigins:     "https://example.com",
			KiwifyCheckoutURLs:      "basic=https://example.com/checkout",
			KiwifyAllowedHosts:      "example.com",
			MetaRetentionDays:       30,
			MetaCleanupSchedule:     "0 3 * * *",
			TokenExpirationSchedule: "0 4 * * *",
			TokenEncryptionKey:      "12345678901234567890123456789012",
		},
		configs.WhatsAppConfig{
			PhoneNumberID:        "123",
			AccessToken:          "token",
			OutreachTemplateName: "template",
			BotNumberE164:        "+5511999999999",
			BotNumberDisplay:     "+55 11 99999-9999",
			WelcomeActivated:     "welcome",
			AlreadyActive:        "active",
			CodeAlreadyUsed:      "used",
			PaymentProcessing:    "processing",
			CodeExpired:          "expired",
			CodeInvalid:          "invalid",
			SystemUnavailable:    "unavailable",
			PleaseUseAtivar:      "ativar",
			InvalidCountry:       "country",
		},
		configs.TelegramConfig{},
		configs.OutboxConfig{},
		configs.EmailConfig{
			Provider:    "smtp",
			FromAddress: "noreply@example.com",
			FromName:    "MeControla",
			ActivateURL: "http://localhost:4321/activate",
			SMTPHost:    "localhost",
			SMTPPort:    1025,
			SMTPTimeout: 5 * time.Second,
		},
		identity.IdentityModule{},
		noop.NewProvider(),
	)

	assert.NoError(t, err)
	assert.NotNil(t, module.PublicRouter)
	assert.NotNil(t, module.WhatsAppGateway)
	assert.NotNil(t, module.WhatsAppMessageProcessor)
	assert.NotNil(t, module.SubscriptionConsumer)
	assert.NotNil(t, module.PaidWithoutTokenConsumer)
	assert.NotNil(t, module.OutreachJob)
	assert.NotNil(t, module.ExpirationJob)
	assert.NotNil(t, module.MetaProcessedMessagesCleanup)
	assert.Len(t, module.EventHandlers, 3)
	assert.Equal(t, "billing.subscription.activated", module.EventHandlers[0].EventType)
	assert.Equal(t, "billing.subscription.activated", module.EventHandlers[1].EventType)
	assert.Equal(t, "billing.subscription.activated_without_token", module.EventHandlers[2].EventType)
}
