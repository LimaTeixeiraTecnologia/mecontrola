package handlers_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/require"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/jobs/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
)

type noopOutreachTokenRepo struct{}

func (r *noopOutreachTokenRepo) Insert(_ context.Context, _ entities.MagicToken) error { return nil }
func (r *noopOutreachTokenRepo) FindByHash(_ context.Context, _ []byte) (entities.MagicToken, error) {
	return entities.MagicToken{}, nil
}
func (r *noopOutreachTokenRepo) FindPaidByMobileForFallback(_ context.Context, _ string) (entities.MagicToken, error) {
	return entities.MagicToken{}, nil
}
func (r *noopOutreachTokenRepo) FindPaidForOutreach(_ context.Context, _ time.Time, _ int) ([]entities.MagicToken, error) {
	return nil, nil
}
func (r *noopOutreachTokenRepo) UpdateMarkPaid(_ context.Context, _ entities.MagicToken) error {
	return nil
}
func (r *noopOutreachTokenRepo) UpdateMarkConsumed(_ context.Context, _ entities.MagicToken) error {
	return nil
}
func (r *noopOutreachTokenRepo) UpdateMarkOutreachSent(_ context.Context, _ string, _ time.Time) error {
	return nil
}
func (r *noopOutreachTokenRepo) UpdateMarkOutreachReset(_ context.Context, _ string) error {
	return nil
}
func (r *noopOutreachTokenRepo) BulkExpire(_ context.Context, _ time.Time, _ int) ([]entities.MagicToken, error) {
	return nil, nil
}
func (r *noopOutreachTokenRepo) CountPaidUnconsumed(_ context.Context) (int64, error) { return 0, nil }

type noopOutreachFactory struct{}

func (f *noopOutreachFactory) MagicTokenRepository(_ database.DBTX) appinterfaces.MagicTokenRepository {
	return &noopOutreachTokenRepo{}
}
func (f *noopOutreachFactory) SupportSignalRepository(_ database.DBTX) appinterfaces.SupportSignalRepository {
	return nil
}
func (f *noopOutreachFactory) MetaMessageRepository(_ database.DBTX) appinterfaces.MetaMessageRepository {
	return nil
}
func (f *noopOutreachFactory) OnboardingCleanupRepository(_ database.DBTX) appinterfaces.OnboardingCleanupRepository {
	return nil
}

type noopOutreachGateway struct {
	called bool
}

func (g *noopOutreachGateway) SendActivationTemplate(_ context.Context, _, _, _ string) (string, error) {
	g.called = true
	return "wamid.noop", nil
}

func (g *noopOutreachGateway) SendTextMessage(_ context.Context, _ string, _ string) error {
	return nil
}

type noopManager struct{}

func (m *noopManager) Driver() database.Driver              { return "" }
func (m *noopManager) DBTX(_ context.Context) database.DBTX { return nil }
func (m *noopManager) BeginTx(_ context.Context, _ database.TxOptions) (database.Tx, error) {
	return nil, nil
}
func (m *noopManager) Ping(_ context.Context) error     { return nil }
func (m *noopManager) Shutdown(_ context.Context) error { return nil }

type noopTokenCipher struct{}

func (c *noopTokenCipher) Encrypt(_ context.Context, clearToken string) (string, error) {
	return clearToken, nil
}

func (c *noopTokenCipher) Decrypt(_ context.Context, _ string) (string, error) {
	return "clear-token", nil
}

func buildSendOutreachUseCase(gw appinterfaces.WhatsAppGateway) *usecases.SendOutreach {
	return usecases.NewSendOutreach(
		&noopManager{},
		&noopOutreachFactory{},
		gw,
		&noopTokenCipher{},
		id.NewUUIDGenerator(),
		"activation_reminder",
		2*time.Hour,
		noop.NewProvider(),
	)
}

func TestOutreachJob_Enabled_Runs(t *testing.T) {
	gw := &noopOutreachGateway{}
	uc := buildSendOutreachUseCase(gw)

	j := handlers.NewOutreachJob(uc, true)

	require.Equal(t, "onboarding.outreach_job", j.Name())
	require.NotEmpty(t, j.Schedule())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := j.Run(ctx)
	require.NoError(t, err)
}

func TestOutreachJob_Disabled_SkipsExecution(t *testing.T) {
	gw := &noopOutreachGateway{}
	uc := buildSendOutreachUseCase(gw)

	j := handlers.NewOutreachJob(uc, false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := j.Run(ctx)
	require.NoError(t, err)
	require.False(t, gw.called, "gateway não deve ser chamado quando outreach está desabilitado")
}

func TestOutreachJob_CancelableContext(t *testing.T) {
	gw := &noopOutreachGateway{}
	uc := buildSendOutreachUseCase(gw)

	j := handlers.NewOutreachJob(uc, true)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := j.Run(ctx)
	require.NoError(t, err)
}
