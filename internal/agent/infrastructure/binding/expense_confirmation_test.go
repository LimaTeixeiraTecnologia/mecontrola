package binding

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/pendingexpense"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type fakeSessionRepo struct {
	records map[string]appinterfaces.AgentSessionRecord
}

func newFakeSessionRepo() *fakeSessionRepo {
	return &fakeSessionRepo{records: make(map[string]appinterfaces.AgentSessionRecord)}
}

func (f *fakeSessionRepo) key(userID uuid.UUID, channel string) string {
	return userID.String() + ":" + channel
}

func (f *fakeSessionRepo) Create(_ context.Context, record appinterfaces.AgentSessionRecord) error {
	k := f.key(record.UserID, record.Channel)
	if _, exists := f.records[k]; exists {
		return appinterfaces.ErrAgentSessionConflict
	}
	f.records[k] = record
	return nil
}

func (f *fakeSessionRepo) GetByUserAndChannel(_ context.Context, userID uuid.UUID, channel string) (appinterfaces.AgentSessionRecord, error) {
	k := f.key(userID, channel)
	rec, ok := f.records[k]
	if !ok {
		return appinterfaces.AgentSessionRecord{}, appinterfaces.ErrAgentSessionNotFound
	}
	return rec, nil
}

func (f *fakeSessionRepo) Update(_ context.Context, record appinterfaces.AgentSessionRecord) error {
	k := f.key(record.UserID, record.Channel)
	f.records[k] = record
	return nil
}

func (f *fakeSessionRepo) Upsert(_ context.Context, record appinterfaces.AgentSessionRecord) error {
	k := f.key(record.UserID, record.Channel)
	f.records[k] = record
	return nil
}

func (f *fakeSessionRepo) DeleteExpired(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

type fakeUpsertErrorRepo struct {
	fakeSessionRepo
	err error
}

func (f *fakeUpsertErrorRepo) Upsert(_ context.Context, _ appinterfaces.AgentSessionRecord) error {
	return f.err
}

type ExpenseConfirmationSuite struct {
	suite.Suite
	ctx     context.Context
	userID  uuid.UUID
	channel string
}

func TestExpenseConfirmationSuite(t *testing.T) {
	suite.Run(t, new(ExpenseConfirmationSuite))
}

func (s *ExpenseConfirmationSuite) SetupTest() {
	s.ctx = context.Background()
	s.userID = uuid.New()
	s.channel = "whatsapp"
}

func (s *ExpenseConfirmationSuite) buildDraft() pendingexpense.Draft {
	return pendingexpense.Draft{
		AmountCents:   9900,
		Merchant:      "Padaria Central",
		PaymentMethod: "credit_card",
		Direction:     "outcome",
		OccurredAt:    "2026-06-23",
		CategoryID:    uuid.New().String(),
		CategoryPath:  "expense.prazeres",
	}
}

func (s *ExpenseConfirmationSuite) TestSaveAndLoad_Roundtrip() {
	repo := newFakeSessionRepo()
	adapter := NewPendingExpenseConfirmationAdapter(repo, nil)
	original := s.buildDraft()

	err := adapter.Save(s.ctx, s.userID, s.channel, original)
	s.Require().NoError(err)

	loaded, found, err := adapter.Load(s.ctx, s.userID, s.channel)
	s.Require().NoError(err)
	s.True(found)
	s.Equal(original.AmountCents, loaded.AmountCents)
	s.Equal(original.Merchant, loaded.Merchant)
	s.Equal(original.PaymentMethod, loaded.PaymentMethod)
	s.Equal(original.Direction, loaded.Direction)
	s.Equal(original.OccurredAt, loaded.OccurredAt)
	s.Equal(original.CategoryID, loaded.CategoryID)
	s.Equal(original.CategoryPath, loaded.CategoryPath)
}

func (s *ExpenseConfirmationSuite) TestLoad_MissingSession_ReturnsFalse() {
	repo := newFakeSessionRepo()
	adapter := NewPendingExpenseConfirmationAdapter(repo, nil)

	loaded, found, err := adapter.Load(s.ctx, s.userID, s.channel)
	s.Require().NoError(err)
	s.False(found)
	s.Equal(pendingexpense.Draft{}, loaded)
}

func (s *ExpenseConfirmationSuite) TestClear_MakesLoadReturnFalse() {
	repo := newFakeSessionRepo()
	adapter := NewPendingExpenseConfirmationAdapter(repo, nil)

	err := adapter.Save(s.ctx, s.userID, s.channel, s.buildDraft())
	s.Require().NoError(err)

	err = adapter.Clear(s.ctx, s.userID, s.channel)
	s.Require().NoError(err)

	_, found, err := adapter.Load(s.ctx, s.userID, s.channel)
	s.Require().NoError(err)
	s.False(found)
}

func (s *ExpenseConfirmationSuite) TestLoad_RecordWithoutPendingExpenseType_ReturnsFalse() {
	repo := newFakeSessionRepo()
	err := repo.Upsert(s.ctx, appinterfaces.AgentSessionRecord{
		ID:            uuid.New(),
		UserID:        s.userID,
		Channel:       s.channel,
		PendingAction: []byte(`{"_t":"budget_config","d":{}}`),
		RecentTurns:   []byte("[]"),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		ExpiresAt:     time.Now().Add(time.Hour),
	})
	s.Require().NoError(err)

	adapter := NewPendingExpenseConfirmationAdapter(repo, nil)
	_, found, loadErr := adapter.Load(s.ctx, s.userID, s.channel)
	s.Require().NoError(loadErr)
	s.False(found)
}

func (s *ExpenseConfirmationSuite) TestSave_RepoError_PropagatesWrapped() {
	fakeRepo := &fakeUpsertErrorRepo{
		fakeSessionRepo: fakeSessionRepo{records: make(map[string]appinterfaces.AgentSessionRecord)},
		err:             errors.New("db down"),
	}
	adapter := NewPendingExpenseConfirmationAdapter(fakeRepo, nil)

	err := adapter.Save(s.ctx, s.userID, s.channel, s.buildDraft())
	s.Require().Error(err)
	s.Contains(err.Error(), "pending expense save")
}

func (s *ExpenseConfirmationSuite) TestClear_RepoError_PropagatesWrapped() {
	fakeRepo := &fakeUpsertErrorRepo{
		fakeSessionRepo: fakeSessionRepo{records: make(map[string]appinterfaces.AgentSessionRecord)},
		err:             errors.New("db down"),
	}
	adapter := NewPendingExpenseConfirmationAdapter(fakeRepo, nil)

	err := adapter.Clear(s.ctx, s.userID, s.channel)
	s.Require().Error(err)
	s.Contains(err.Error(), "pending expense clear")
}

type fakeUoW struct {
	called bool
}

func (f *fakeUoW) DBTX() database.DBTX {
	return nil
}

func (f *fakeUoW) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	f.called = true
	return fn(ctx, nil)
}

func (s *ExpenseConfirmationSuite) TestSave_WithUnitOfWork_CallsDo() {
	repo := newFakeSessionRepo()
	uow := &fakeUoW{}
	adapter := NewPendingExpenseConfirmationAdapter(repo, uow)

	err := adapter.Save(s.ctx, s.userID, s.channel, s.buildDraft())
	s.Require().NoError(err)
	s.True(uow.called)
}
