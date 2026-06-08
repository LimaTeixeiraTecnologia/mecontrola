package usecases

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type fakeMarkPaidManager struct{}

func (m *fakeMarkPaidManager) Driver() database.Driver              { return "" }
func (m *fakeMarkPaidManager) DBTX(_ context.Context) database.DBTX { return nil }
func (m *fakeMarkPaidManager) BeginTx(_ context.Context, _ database.TxOptions) (database.Tx, error) {
	return nil, nil
}
func (m *fakeMarkPaidManager) Ping(_ context.Context) error     { return nil }
func (m *fakeMarkPaidManager) Shutdown(_ context.Context) error { return nil }

type fakeMarkPaidRepo struct {
	expectedHash []byte
	updated      entities.MagicToken
}

func (r *fakeMarkPaidRepo) Insert(_ context.Context, _ entities.MagicToken) error { return nil }
func (r *fakeMarkPaidRepo) FindByHash(_ context.Context, tokenHash []byte) (entities.MagicToken, error) {
	if !bytes.Equal(r.expectedHash, tokenHash) {
		return entities.MagicToken{}, domain.ErrTokenNotFound
	}
	token, _ := entities.NewMagicToken("tok-id", tokenHash, "plan-1", time.Now().UTC().Add(7*24*time.Hour))
	token, _ = token.WithActivationTokenCiphertext("cipher-token")
	return token, nil
}
func (r *fakeMarkPaidRepo) FindPaidByMobileForFallback(_ context.Context, _ string) (entities.MagicToken, error) {
	return entities.MagicToken{}, nil
}
func (r *fakeMarkPaidRepo) FindPaidForOutreach(_ context.Context, _ time.Time, _ int) ([]entities.MagicToken, error) {
	return nil, nil
}
func (r *fakeMarkPaidRepo) UpdateMarkPaid(_ context.Context, token entities.MagicToken) error {
	r.updated = token
	return nil
}
func (r *fakeMarkPaidRepo) UpdateMarkConsumed(_ context.Context, _ entities.MagicToken) error {
	return nil
}
func (r *fakeMarkPaidRepo) UpdateMarkOutreachSent(_ context.Context, _ string, _ time.Time) error {
	return nil
}
func (r *fakeMarkPaidRepo) UpdateMarkOutreachReset(_ context.Context, _ string) error { return nil }
func (r *fakeMarkPaidRepo) BulkExpire(_ context.Context, _ time.Time, _ int) ([]entities.MagicToken, error) {
	return nil, nil
}
func (r *fakeMarkPaidRepo) CountPaidUnconsumed(_ context.Context) (int64, error) { return 0, nil }

type fakeMarkPaidFactory struct {
	repo appinterfaces.MagicTokenRepository
}

func (f *fakeMarkPaidFactory) MagicTokenRepository(_ database.DBTX) appinterfaces.MagicTokenRepository {
	return f.repo
}
func (f *fakeMarkPaidFactory) SupportSignalRepository(_ database.DBTX) appinterfaces.SupportSignalRepository {
	return nil
}
func (f *fakeMarkPaidFactory) MetaMessageRepository(_ database.DBTX) appinterfaces.MetaMessageRepository {
	return nil
}
func (f *fakeMarkPaidFactory) OnboardingCleanupRepository(_ database.DBTX) appinterfaces.OnboardingCleanupRepository {
	return nil
}

func TestMarkTokenPaid_UsesDecodedMagicTokenHashAndStoresSubscriptionID(t *testing.T) {
	clearToken, err := valueobjects.NewToken()
	require.NoError(t, err)
	repo := &fakeMarkPaidRepo{expectedHash: clearToken.Hash()}
	uc := NewMarkTokenPaid(&fakeMarkPaidManager{}, &fakeMarkPaidFactory{repo: repo}, noop.NewProvider())

	err = uc.Execute(context.Background(), input.MarkTokenPaidInput{
		SubscriptionID:     "sub-001",
		FunnelToken:        clearToken.ClearText(),
		CustomerMobileE164: "+5511999999999",
		CustomerEmail:      "user@example.com",
		ExternalSaleID:     "sale-001",
		PaidAt:             time.Now().UTC(),
	})

	require.NoError(t, err)
	require.Equal(t, "sub-001", repo.updated.SubscriptionID())
}
