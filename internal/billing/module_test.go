package billing_test

import (
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing"
)

func TestNewBillingModule_FieldsNotNil(t *testing.T) {
	module, err := billing.NewBillingModule(&configs.Config{
		KiwifyConfig: configs.KiwifyConfig{
			APIBaseURL:             "https://example.com",
			HTTPTimeout:            time.Second,
			HTTPRetryBackoff:       time.Second,
			ClientID:               "client",
			ClientSecret:           "secret",
			AccountID:              "account",
			WebhookSecret:          "secret",
			WebhookSecretNext:      "next",
			ProductIDMonthly:       "m",
			ProductIDQuarterly:     "q",
			ProductIDAnnual:        "a",
			OAuthTokenSafetyMargin: time.Second,
		},
	}, noop.NewProvider(), (*sqlx.DB)(nil))

	assert.NoError(t, err)
	assert.NotNil(t, module.RepositoryFactory)
	assert.NotNil(t, module.WebhookRouter)
	assert.NotNil(t, module.ReconciliationJob)
	assert.NotNil(t, module.KiwifyEventsHousekeeper)
	assert.NotNil(t, module.GraceExpirationJob)
	assert.NotNil(t, module.SubscriptionEventPublisher)
	assert.Len(t, module.EventHandlers, 3)
}
