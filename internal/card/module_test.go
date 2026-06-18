package card_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	cardidentity "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/identity"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/notification"
)

func TestNewCardModule_FieldsNotNil(t *testing.T) {
	o11y := noop.NewProvider()
	cfg := &configs.Config{}

	passthrough := func(next http.Handler) http.Handler { return next }
	m, err := card.NewCardModule(context.Background(), cfg, o11y, (*sqlx.DB)(nil), passthrough, nil, nil)

	assert.NoError(t, err, "NewCardModule nao deve retornar erro")
	assert.NotNil(t, m.RepositoryFactory, "RepositoryFactory deve ser nao-nil")
	assert.NotNil(t, m.CardRouter, "CardRouter deve ser nao-nil")
	assert.NotNil(t, m.CardLookup, "CardLookup deve ser nao-nil")
	assert.Nil(t, m.InvoiceDueAlertsJob, "InvoiceDueAlertsJob deve ser nil sem gateway/resolver")
}

func TestNewCardModule_WiresInvoiceDueJobWhenChannelAvailable(t *testing.T) {
	o11y := noop.NewProvider()
	cfg := &configs.Config{}
	cfg.CardConfig.InvoiceDueAlertsEnabled = true

	passthrough := func(next http.Handler) http.Handler { return next }
	gateway := notification.NewMultiChannelGateway(map[string]notification.ChannelSenders{})
	resolver := cardidentity.NewUserChannelResolverAdapter(nil)

	m, err := card.NewCardModule(context.Background(), cfg, o11y, (*sqlx.DB)(nil), passthrough, gateway, resolver)

	assert.NoError(t, err, "NewCardModule nao deve retornar erro")
	assert.NotNil(t, m.InvoiceDueAlertsJob, "InvoiceDueAlertsJob deve ser nao-nil com gateway/resolver")
	hasInvoiceDueHandler := false
	for _, reg := range m.EventHandlers {
		if reg.EventType == "card.invoice_due.v1" {
			hasInvoiceDueHandler = true
		}
	}
	assert.True(t, hasInvoiceDueHandler, "consumer card.invoice_due.v1 deve estar registrado")
}
