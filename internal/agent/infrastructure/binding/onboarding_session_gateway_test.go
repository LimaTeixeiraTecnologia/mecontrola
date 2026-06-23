package binding

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/onboardingv2draft"
)

type fakeOnbSessionRepo struct {
	rec      appinterfaces.AgentSessionRecord
	getErr   error
	upsErr   error
	upserted *appinterfaces.AgentSessionRecord
}

func (f *fakeOnbSessionRepo) GetByUserAndChannel(_ context.Context, _ uuid.UUID, _ string) (appinterfaces.AgentSessionRecord, error) {
	return f.rec, f.getErr
}

func (f *fakeOnbSessionRepo) Upsert(_ context.Context, record appinterfaces.AgentSessionRecord) error {
	f.upserted = &record
	return f.upsErr
}

type OnboardingSessionGatewaySuite struct {
	suite.Suite
	ctx context.Context
	uid uuid.UUID
}

func TestOnboardingSessionGatewaySuite(t *testing.T) {
	suite.Run(t, new(OnboardingSessionGatewaySuite))
}

func (s *OnboardingSessionGatewaySuite) SetupTest() {
	s.ctx = context.Background()
	s.uid = uuid.New()
}

func (s *OnboardingSessionGatewaySuite) TestLoadNotFoundReturnsFalse() {
	repo := &fakeOnbSessionRepo{getErr: appinterfaces.ErrAgentSessionNotFound}
	gw := NewOnboardingSessionGateway(repo)
	_, found, err := gw.Load(s.ctx, s.uid, "whatsapp")
	s.NoError(err)
	s.False(found)
}

func (s *OnboardingSessionGatewaySuite) TestLoadWrongKindReturnsFalse() {
	repo := &fakeOnbSessionRepo{rec: appinterfaces.AgentSessionRecord{
		PendingAction: []byte(`{"kind":"budget_config"}`),
	}}
	gw := NewOnboardingSessionGateway(repo)
	_, found, err := gw.Load(s.ctx, s.uid, "whatsapp")
	s.NoError(err)
	s.False(found)
}

func (s *OnboardingSessionGatewaySuite) TestLoadEmptyPendingActionReturnsFalse() {
	repo := &fakeOnbSessionRepo{rec: appinterfaces.AgentSessionRecord{
		PendingAction: []byte(`{}`),
	}}
	gw := NewOnboardingSessionGateway(repo)
	_, found, err := gw.Load(s.ctx, s.uid, "whatsapp")
	s.NoError(err)
	s.False(found)
}

func (s *OnboardingSessionGatewaySuite) TestLoadHappyPath() {
	draft := onboardingv2draft.New().WithIncome(500000)
	raw, err := onboardingv2draft.Encode(draft)
	s.Require().NoError(err)
	repo := &fakeOnbSessionRepo{rec: appinterfaces.AgentSessionRecord{
		PendingAction: raw,
		RecentTurns:   []byte("[]"),
	}}
	gw := NewOnboardingSessionGateway(repo)
	loaded, found, err := gw.Load(s.ctx, s.uid, "whatsapp")
	s.NoError(err)
	s.True(found)
	s.Equal(int64(500000), loaded.IncomeCents())
}

func (s *OnboardingSessionGatewaySuite) TestSaveNewSessionCreatesRecord() {
	repo := &fakeOnbSessionRepo{getErr: appinterfaces.ErrAgentSessionNotFound}
	gw := NewOnboardingSessionGateway(repo)
	draft := onboardingv2draft.New().WithIncome(300000)
	err := gw.Save(s.ctx, s.uid, "whatsapp", draft)
	s.NoError(err)
	s.NotNil(repo.upserted)
	s.Equal([]byte("[]"), repo.upserted.RecentTurns)
	pending := repo.upserted.PendingAction
	restored, restErr := onboardingv2draft.Restore(pending)
	s.NoError(restErr)
	s.Equal(int64(300000), restored.IncomeCents())
}

func (s *OnboardingSessionGatewaySuite) TestSaveExistingSessionPreservesRecentTurns() {
	existingTurns := []byte(`[{"role":"user","content":"oi"}]`)
	repo := &fakeOnbSessionRepo{rec: appinterfaces.AgentSessionRecord{
		ID:            uuid.New(),
		UserID:        s.uid,
		Channel:       "whatsapp",
		RecentTurns:   existingTurns,
		PendingAction: []byte(`{}`),
	}}
	gw := NewOnboardingSessionGateway(repo)
	draft := onboardingv2draft.New().WithIncome(300000)
	err := gw.Save(s.ctx, s.uid, "whatsapp", draft)
	s.NoError(err)
	s.Require().NotNil(repo.upserted)
	s.Equal(existingTurns, repo.upserted.RecentTurns)
}

func (s *OnboardingSessionGatewaySuite) TestSaveInfraErrorPropagated() {
	repo := &fakeOnbSessionRepo{getErr: errors.New("connection reset")}
	gw := NewOnboardingSessionGateway(repo)
	err := gw.Save(s.ctx, s.uid, "whatsapp", onboardingv2draft.New())
	s.Error(err)
	s.Contains(err.Error(), "load for save")
}

func (s *OnboardingSessionGatewaySuite) TestClearSetsPendingActionEmpty() {
	repo := &fakeOnbSessionRepo{rec: appinterfaces.AgentSessionRecord{
		PendingAction: []byte(`{"kind":"onboarding_v2","step":1}`),
		RecentTurns:   []byte("[]"),
	}}
	gw := NewOnboardingSessionGateway(repo)
	err := gw.Clear(s.ctx, s.uid, "whatsapp")
	s.NoError(err)
	s.Require().NotNil(repo.upserted)
	s.Equal([]byte("{}"), repo.upserted.PendingAction)
}

func (s *OnboardingSessionGatewaySuite) TestClearWhenNotFoundDoesNotError() {
	repo := &fakeOnbSessionRepo{getErr: appinterfaces.ErrAgentSessionNotFound}
	gw := NewOnboardingSessionGateway(repo)
	err := gw.Clear(s.ctx, s.uid, "whatsapp")
	s.NoError(err)
	s.Nil(repo.upserted)
}

func (s *OnboardingSessionGatewaySuite) TestLoadRestoreErrorPropagated() {
	repo := &fakeOnbSessionRepo{rec: appinterfaces.AgentSessionRecord{
		PendingAction: []byte(`{"kind":"onboarding_v2","step":0}`),
	}}
	gw := NewOnboardingSessionGateway(repo)
	_, found, err := gw.Load(s.ctx, s.uid, "whatsapp")
	s.Error(err)
	s.False(found)
	s.Contains(err.Error(), "restore")
}
