package identity_test

import (
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
)

func TestNewIdentityModule_FieldsNotNil(t *testing.T) {
	module, err := identity.NewIdentityModule(&configs.Config{}, noop.NewProvider(), (*sqlx.DB)(nil))
	require.NoError(t, err)

	assert.NotNil(t, module.RepositoryFactory)
	assert.NotNil(t, module.UserRouter)
	assert.NotNil(t, module.UpsertUserUseCase)
	assert.NotNil(t, module.FindUserByIDUseCase)
	assert.NotNil(t, module.FindUserByWhatsApp)
	assert.NotNil(t, module.MarkUserDeleted)
	assert.NotNil(t, module.EstablishPrincipal)
	assert.NotNil(t, module.GatewayAuthMiddleware)
	assert.NotNil(t, module.EntitlementReader)
	assert.NotNil(t, module.SubscriptionProjector)
	assert.NotNil(t, module.SubscriptionBoundProjector)
	assert.NotNil(t, module.AuthEventsConsumer)
	assert.NotNil(t, module.AuthEventsHousekeepingJob)
	assert.NotNil(t, module.WhatsAppLimiter)
	assert.NotNil(t, module.WhatsAppDedupRepository)
	assert.NotNil(t, module.OutboxPublisher)
	assert.Len(t, module.EventHandlers, 10)
}

func TestNewRequireGatewayAuth_InvalidSecretFails(t *testing.T) {
	_, err := identity.NewRequireGatewayAuth(
		configs.IdentityConfig{GatewaySharedSecretCurrent: "zz"},
		usecases.NewRecordGatewayAuthFailure(nil, noop.NewProvider()),
		noop.NewProvider(),
	)

	require.Error(t, err)
	assert.ErrorContains(t, err, "decode gateway secret current")
}
