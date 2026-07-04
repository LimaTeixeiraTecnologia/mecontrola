package onboarding_test

import (
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding"
)

func TestNewOnboardingModule_FieldsNotNil(t *testing.T) {
	module, err := onboarding.NewOnboardingModule(
		(*sqlx.DB)(nil),
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
			InvalidCountry:       "country",
		},
		configs.OutboxConfig{},
		configs.EmailConfig{
			Provider:    "smtp",
			FromAddress: "noreply@example.com",
			FromName:    "MeControla",
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
	assert.NotNil(t, module.WhatsAppActivationRoute)
	assert.NotNil(t, module.SubscriptionConsumer)
	assert.NotNil(t, module.PaidWithoutTokenConsumer)
	assert.NotNil(t, module.OutreachJob)
	assert.NotNil(t, module.ExpirationJob)
	assert.NotNil(t, module.MetaProcessedMessagesCleanup)
	assert.Len(t, module.EventHandlers, 4)
	assert.Equal(t, "billing.subscription.activated", module.EventHandlers[0].EventType)
	assert.Equal(t, "billing.subscription.activated", module.EventHandlers[1].EventType)
	assert.Equal(t, "billing.subscription.activated_without_token", module.EventHandlers[2].EventType)
	assert.Equal(t, "onboarding.activation.attempted.v1", module.EventHandlers[3].EventType)
}
