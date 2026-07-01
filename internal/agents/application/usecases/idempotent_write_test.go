package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

type fakeLedger struct {
	entry     WriteLedgerEntry
	found     bool
	findErr   error
	insertErr error
	inserted  []WriteLedgerEntry
}

func (f *fakeLedger) FindByKey(_ context.Context, _ string, _ int, _ string) (WriteLedgerEntry, error) {
	if f.findErr != nil {
		return WriteLedgerEntry{}, f.findErr
	}
	if !f.found {
		return WriteLedgerEntry{}, ErrLedgerEntryNotFound
	}
	return f.entry, nil
}

func (f *fakeLedger) Insert(_ context.Context, entry WriteLedgerEntry) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	f.inserted = append(f.inserted, entry)
	return nil
}

type fakeKeyLocker struct {
	calls   int
	lastKey string
}

func (f *fakeKeyLocker) WithKeyLock(ctx context.Context, key string, fn func(context.Context) error) error {
	f.calls++
	f.lastKey = key
	return fn(ctx)
}

type IdempotentWriteSuite struct {
	suite.Suite
	ctx    context.Context
	obs    observability.Observability
	userID uuid.UUID
	wamid  string
}

func TestIdempotentWriteSuite(t *testing.T) {
	suite.Run(t, new(IdempotentWriteSuite))
}

func (s *IdempotentWriteSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.userID = uuid.New()
	s.wamid = "wamid-" + uuid.NewString()
}

func (s *IdempotentWriteSuite) TestExecute() {
	existingResourceID := uuid.New()
	newResourceID := uuid.New()

	type args struct {
		userID       uuid.UUID
		wamid        string
		itemSeq      int
		operation    string
		resourceKind string
		write        WriteFn
	}
	type dependencies struct {
		ledger *fakeLedger
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result IdempotentWriteResult, err error, ledger *fakeLedger)
	}{
		{
			name: "miss: executa escrita e grava no ledger",
			args: args{
				userID:       s.userID,
				wamid:        s.wamid,
				itemSeq:      0,
				operation:    "create_transaction",
				resourceKind: "transaction",
				write: func(_ context.Context) (uuid.UUID, bool, error) {
					return newResourceID, false, nil
				},
			},
			dependencies: dependencies{
				ledger: &fakeLedger{found: false},
			},
			expect: func(result IdempotentWriteResult, err error, ledger *fakeLedger) {
				s.NoError(err)
				s.Equal(newResourceID, result.ResourceID)
				s.Equal(agent.ToolOutcomeRouted, result.Outcome)
				s.Len(ledger.inserted, 1)
				s.Equal(newResourceID, ledger.inserted[0].ResourceID)
				s.Equal("create_transaction", ledger.inserted[0].Operation)
				s.Equal(0, ledger.inserted[0].ItemSeq)
			},
		},
		{
			name: "miss reconciliado: escrita bateu em linha existente (ON CONFLICT) e grava no ledger",
			args: args{
				userID:       s.userID,
				wamid:        s.wamid,
				itemSeq:      0,
				operation:    "create_transaction",
				resourceKind: "transaction",
				write: func(_ context.Context) (uuid.UUID, bool, error) {
					return newResourceID, true, nil
				},
			},
			dependencies: dependencies{
				ledger: &fakeLedger{found: false},
			},
			expect: func(result IdempotentWriteResult, err error, ledger *fakeLedger) {
				s.NoError(err)
				s.Equal(newResourceID, result.ResourceID)
				s.Equal(agent.ToolOutcomeReconciled, result.Outcome)
				s.Len(ledger.inserted, 1)
				s.Equal(newResourceID, ledger.inserted[0].ResourceID)
			},
		},
		{
			name: "hit: retorna replay sem segunda mutação",
			args: args{
				userID:       s.userID,
				wamid:        s.wamid,
				itemSeq:      1,
				operation:    "create_card_purchase",
				resourceKind: "card_purchase",
				write: func(_ context.Context) (uuid.UUID, bool, error) {
					s.Fail("write não deve ser chamado no replay")
					return uuid.Nil, false, nil
				},
			},
			dependencies: dependencies{
				ledger: &fakeLedger{
					found: true,
					entry: WriteLedgerEntry{
						ID:           uuid.New(),
						UserID:       s.userID,
						WAMID:        s.wamid,
						ItemSeq:      1,
						Operation:    "create_card_purchase",
						ResourceID:   existingResourceID,
						ResourceKind: "card_purchase",
						CreatedAt:    time.Now().UTC(),
					},
				},
			},
			expect: func(result IdempotentWriteResult, err error, ledger *fakeLedger) {
				s.NoError(err)
				s.Equal(existingResourceID, result.ResourceID)
				s.Equal(agent.ToolOutcomeReplay, result.Outcome)
				s.Empty(ledger.inserted)
			},
		},
		{
			name: "erro no ledger lookup retorna erro",
			args: args{
				userID:       s.userID,
				wamid:        s.wamid,
				itemSeq:      0,
				operation:    "create_transaction",
				resourceKind: "transaction",
				write:        func(_ context.Context) (uuid.UUID, bool, error) { return uuid.New(), false, nil },
			},
			dependencies: dependencies{
				ledger: &fakeLedger{findErr: errors.New("db error")},
			},
			expect: func(result IdempotentWriteResult, err error, _ *fakeLedger) {
				s.Error(err)
				s.Equal(IdempotentWriteResult{}, result)
			},
		},
		{
			name: "erro na escrita retorna erro e não grava ledger",
			args: args{
				userID:       s.userID,
				wamid:        s.wamid,
				itemSeq:      0,
				operation:    "create_transaction",
				resourceKind: "transaction",
				write: func(_ context.Context) (uuid.UUID, bool, error) {
					return uuid.Nil, false, errors.New("usecase error")
				},
			},
			dependencies: dependencies{
				ledger: &fakeLedger{found: false},
			},
			expect: func(result IdempotentWriteResult, err error, ledger *fakeLedger) {
				s.Error(err)
				s.Equal(IdempotentWriteResult{}, result)
				s.Empty(ledger.inserted)
			},
		},
		{
			name: "insert no ledger falha retorna erro para prevenir duplicata",
			args: args{
				userID:       s.userID,
				wamid:        s.wamid,
				itemSeq:      2,
				operation:    "delete_transaction",
				resourceKind: "transaction",
				write: func(_ context.Context) (uuid.UUID, bool, error) {
					return newResourceID, false, nil
				},
			},
			dependencies: dependencies{
				ledger: &fakeLedger{found: false, insertErr: errors.New("insert error")},
			},
			expect: func(result IdempotentWriteResult, err error, _ *fakeLedger) {
				s.Error(err)
				s.Equal(IdempotentWriteResult{}, result)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewIdempotentWrite(scenario.dependencies.ledger, s.obs)
			result, err := uc.Execute(
				s.ctx,
				scenario.args.userID,
				scenario.args.wamid,
				scenario.args.itemSeq,
				scenario.args.operation,
				scenario.args.resourceKind,
				scenario.args.write,
			)
			scenario.expect(result, err, scenario.dependencies.ledger)
		})
	}
}

func (s *IdempotentWriteSuite) TestExecuteWithKeyLocker() {
	newResourceID := uuid.New()
	ledger := &fakeLedger{found: false}
	locker := &fakeKeyLocker{}

	uc := NewIdempotentWrite(ledger, s.obs, WithKeyLocker(locker))
	result, err := uc.Execute(
		s.ctx,
		s.userID,
		s.wamid,
		3,
		"create_transaction",
		"transaction",
		func(_ context.Context) (uuid.UUID, bool, error) { return newResourceID, false, nil },
	)

	s.NoError(err)
	s.Equal(newResourceID, result.ResourceID)
	s.Equal(agent.ToolOutcomeRouted, result.Outcome)
	s.Equal(1, locker.calls)
	s.Equal(s.wamid+"|3|create_transaction", locker.lastKey)
	s.Len(ledger.inserted, 1)
	s.Equal(newResourceID, ledger.inserted[0].ResourceID)
}

func (s *IdempotentWriteSuite) TestExecuteNilLockerPreservesBehavior() {
	existingResourceID := uuid.New()
	ledger := &fakeLedger{
		found: true,
		entry: WriteLedgerEntry{
			ID:           uuid.New(),
			UserID:       s.userID,
			WAMID:        s.wamid,
			ItemSeq:      0,
			Operation:    "create_transaction",
			ResourceID:   existingResourceID,
			ResourceKind: "transaction",
			CreatedAt:    time.Now().UTC(),
		},
	}

	uc := NewIdempotentWrite(ledger, s.obs)
	result, err := uc.Execute(
		s.ctx,
		s.userID,
		s.wamid,
		0,
		"create_transaction",
		"transaction",
		func(_ context.Context) (uuid.UUID, bool, error) {
			s.Fail("write não deve ser chamado no replay")
			return uuid.Nil, false, nil
		},
	)

	s.NoError(err)
	s.Equal(existingResourceID, result.ResourceID)
	s.Equal(agent.ToolOutcomeReplay, result.Outcome)
	s.Empty(ledger.inserted)
}
