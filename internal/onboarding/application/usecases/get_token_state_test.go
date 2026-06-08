package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases/mocks"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type stubGetTokenStateRepo struct {
	token entities.MagicToken
	err   error
}

func (s *stubGetTokenStateRepo) Insert(_ context.Context, _ entities.MagicToken) error {
	return nil
}
func (s *stubGetTokenStateRepo) FindByHash(_ context.Context, _ []byte) (entities.MagicToken, error) {
	return s.token, s.err
}
func (s *stubGetTokenStateRepo) FindPaidByMobileForFallback(_ context.Context, _ string) (entities.MagicToken, error) {
	return entities.MagicToken{}, nil
}
func (s *stubGetTokenStateRepo) FindPaidForOutreach(_ context.Context, _ time.Time, _ int) ([]entities.MagicToken, error) {
	return nil, nil
}
func (s *stubGetTokenStateRepo) UpdateMarkPaid(_ context.Context, _ entities.MagicToken) error {
	return nil
}
func (s *stubGetTokenStateRepo) UpdateMarkConsumed(_ context.Context, _ entities.MagicToken) error {
	return nil
}
func (s *stubGetTokenStateRepo) UpdateMarkOutreachSent(_ context.Context, _ string, _ time.Time) error {
	return nil
}
func (s *stubGetTokenStateRepo) UpdateMarkOutreachReset(_ context.Context, _ string) error {
	return nil
}
func (s *stubGetTokenStateRepo) BulkExpire(_ context.Context, _ time.Time, _ int) ([]entities.MagicToken, error) {
	return nil, nil
}
func (s *stubGetTokenStateRepo) CountPaidUnconsumed(_ context.Context) (int64, error) {
	return 0, nil
}

type stubGetTokenStateFactory struct {
	repo appinterfaces.MagicTokenRepository
}

func (f *stubGetTokenStateFactory) MagicTokenRepository(_ database.DBTX) appinterfaces.MagicTokenRepository {
	return f.repo
}
func (f *stubGetTokenStateFactory) SupportSignalRepository(_ database.DBTX) appinterfaces.SupportSignalRepository {
	return nil
}
func (f *stubGetTokenStateFactory) MetaMessageRepository(_ database.DBTX) appinterfaces.MetaMessageRepository {
	return nil
}
func (f *stubGetTokenStateFactory) OnboardingCleanupRepository(_ database.DBTX) appinterfaces.OnboardingCleanupRepository {
	return nil
}

func TestGetTokenState_ReadyToActivateWhenPaidAndNotExpired(t *testing.T) {
	tok, err := valueobjects.NewToken()
	require.NoError(t, err)

	mt, err := entities.NewMagicToken("id-1", tok.Hash(), "plan-1", time.Now().UTC().Add(7*24*time.Hour))
	require.NoError(t, err)
	paid, err := mt.MarkPaid("sub-001", "+5511999999999", "u@test.com", "sale-1", time.Now().UTC())
	require.NoError(t, err)

	repo := &stubGetTokenStateRepo{token: paid}
	factory := &stubGetTokenStateFactory{repo: repo}
	mgr := mocks.NewFakeManager()

	uc := usecases.NewGetTokenState(mgr, factory, "+5511999999999", "+55 11 9XXXX-XXXX", noop.NewProvider())

	result, err := uc.Execute(context.Background(), tok.ClearText())
	require.NoError(t, err)
	assert.True(t, result.Output.ReadyToActivate)
	assert.NotEmpty(t, result.Output.WaMeURL)
	assert.NotEmpty(t, result.Output.BotNumberDisplay)
}

func TestGetTokenState_NotReadyWhenPending(t *testing.T) {
	tok, err := valueobjects.NewToken()
	require.NoError(t, err)

	mt, err := entities.NewMagicToken("id-2", tok.Hash(), "plan-1", time.Now().UTC().Add(7*24*time.Hour))
	require.NoError(t, err)

	repo := &stubGetTokenStateRepo{token: mt}
	factory := &stubGetTokenStateFactory{repo: repo}
	mgr := mocks.NewFakeManager()

	uc := usecases.NewGetTokenState(mgr, factory, "+5511999999999", "+55 11 9XXXX-XXXX", noop.NewProvider())

	result, err := uc.Execute(context.Background(), tok.ClearText())
	require.NoError(t, err)
	assert.False(t, result.Output.ReadyToActivate)
	assert.Empty(t, result.Output.WaMeURL)
	assert.Equal(t, usecases.TokenStateReasonPending, result.Reason)
}

func TestGetTokenState_NotReadyWhenExpired(t *testing.T) {
	tok, err := valueobjects.NewToken()
	require.NoError(t, err)

	mt, err := entities.NewMagicToken("id-3", tok.Hash(), "plan-1", time.Now().UTC().Add(-1*time.Hour))
	require.NoError(t, err)
	paid, err := mt.MarkPaid("sub-001", "+5511999999999", "u@test.com", "sale-1", time.Now().UTC().Add(-2*time.Hour))
	require.NoError(t, err)

	repo := &stubGetTokenStateRepo{token: paid}
	factory := &stubGetTokenStateFactory{repo: repo}
	mgr := mocks.NewFakeManager()

	uc := usecases.NewGetTokenState(mgr, factory, "+5511999999999", "+55 11 9XXXX-XXXX", noop.NewProvider())

	result, err := uc.Execute(context.Background(), tok.ClearText())
	require.NoError(t, err)
	assert.False(t, result.Output.ReadyToActivate)
	assert.Empty(t, result.Output.WaMeURL)
	assert.Equal(t, usecases.TokenStateReasonExpired, result.Reason)
}

func TestGetTokenState_NotReadyWhenNotFound(t *testing.T) {
	tok, err := valueobjects.NewToken()
	require.NoError(t, err)

	repo := &stubGetTokenStateRepo{err: domain.ErrTokenNotFound}
	factory := &stubGetTokenStateFactory{repo: repo}
	mgr := mocks.NewFakeManager()

	uc := usecases.NewGetTokenState(mgr, factory, "+5511999999999", "+55 11 9XXXX-XXXX", noop.NewProvider())

	result, err := uc.Execute(context.Background(), tok.ClearText())
	require.NoError(t, err)
	assert.False(t, result.Output.ReadyToActivate)
	assert.Equal(t, usecases.TokenStateReasonNotFound, result.Reason)
}

func TestGetTokenState_WaMeURLContainsToken(t *testing.T) {
	tok, err := valueobjects.NewToken()
	require.NoError(t, err)
	clearToken := tok.ClearText()

	mt, err := entities.NewMagicToken("id-4", tok.Hash(), "plan-1", time.Now().UTC().Add(7*24*time.Hour))
	require.NoError(t, err)
	paid, err := mt.MarkPaid("sub-001", "+5511999999999", "u@test.com", "sale-1", time.Now().UTC())
	require.NoError(t, err)

	repo := &stubGetTokenStateRepo{token: paid}
	factory := &stubGetTokenStateFactory{repo: repo}
	mgr := mocks.NewFakeManager()

	uc := usecases.NewGetTokenState(mgr, factory, "+5511999999999", "+55 11 9XXXX-XXXX", noop.NewProvider())

	result, err := uc.Execute(context.Background(), clearToken)
	require.NoError(t, err)
	assert.True(t, result.Output.ReadyToActivate)
	assert.Contains(t, result.Output.WaMeURL, clearToken)
}
