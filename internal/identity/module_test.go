package identity_test

import (
	"context"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
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

func TestNewIdentityModule_FieldsNotNil(t *testing.T) {
	module, err := identity.NewIdentityModule(&configs.Config{}, noop.NewProvider(), manager.Manager(&stubManager{}))
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
