package billing_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing"
)

type stubManager struct{}

func (s *stubManager) Driver() database.Driver              { return "" }
func (s *stubManager) DBTX(_ context.Context) database.DBTX { return nil }
func (s *stubManager) BeginTx(_ context.Context, _ database.TxOptions) (database.Tx, error) {
	return nil, nil
}
func (s *stubManager) Ping(_ context.Context) error     { return nil }
func (s *stubManager) Shutdown(_ context.Context) error { return nil }

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
	}, noop.NewProvider(), manager.Manager(&stubManager{}))

	assert.NoError(t, err)
	assert.NotNil(t, module.RepositoryFactory)
	assert.NotNil(t, module.WebhookRouter)
	assert.NotNil(t, module.ReconciliationJob)
	assert.NotNil(t, module.KiwifyEventsHousekeeper)
	assert.NotNil(t, module.GraceExpirationJob)
	assert.NotNil(t, module.SubscriptionEventPublisher)
	assert.Len(t, module.EventHandlers, 3)
}
