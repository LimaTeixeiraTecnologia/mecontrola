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

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
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
		isDomainErr  workflows.DomainErrorClassifier
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
			name: "miss reconciliado: conflito de chave natural propaga como ToolOutcomeReconciled nunca usecaseError",
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
			name: "hit: retorna replay sem segunda mutacao",
			args: args{
				userID:       s.userID,
				wamid:        s.wamid,
				itemSeq:      1,
				operation:    "create_card_purchase",
				resourceKind: "card_purchase",
				write: func(_ context.Context) (uuid.UUID, bool, error) {
					s.Fail("write nao deve ser chamado no replay")
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
			name: "erro na escrita retorna erro e nao grava ledger",
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
				scenario.args.isDomainErr,
			)
			scenario.expect(result, err, scenario.dependencies.ledger)
		})
	}
}

func (s *IdempotentWriteSuite) TestReprocessAfterLedgerInsertFailure_ReconciledNotDoubleWrite_RF25() {
	resourceID := uuid.New()
	ledger := &fakeLedger{found: false, insertErr: errors.New("insert error")}
	uc := NewIdempotentWrite(ledger, s.obs)

	_, err := uc.Execute(
		s.ctx, s.userID, s.wamid, 0, "create_transaction", "transaction",
		func(_ context.Context) (uuid.UUID, bool, error) { return resourceID, false, nil },
		nil,
	)
	s.Error(err)
	s.Empty(ledger.inserted)

	ledger.insertErr = nil
	result, err2 := uc.Execute(
		s.ctx, s.userID, s.wamid, 0, "create_transaction", "transaction",
		func(_ context.Context) (uuid.UUID, bool, error) { return resourceID, true, nil },
		nil,
	)

	s.NoError(err2)
	s.Equal(resourceID, result.ResourceID)
	s.Equal(agent.ToolOutcomeReconciled, result.Outcome)
	s.Len(ledger.inserted, 1)
}

func (s *IdempotentWriteSuite) TestNoAdvisoryLockRequired() {
	newResourceID := uuid.New()
	ledger := &fakeLedger{found: false}

	uc := NewIdempotentWrite(ledger, s.obs)
	result, err := uc.Execute(
		s.ctx,
		s.userID,
		s.wamid,
		0,
		"create_transaction",
		"transaction",
		func(_ context.Context) (uuid.UUID, bool, error) { return newResourceID, false, nil },
		nil,
	)

	s.NoError(err)
	s.Equal(newResourceID, result.ResourceID)
	s.Equal(agent.ToolOutcomeRouted, result.Outcome)
	s.Len(ledger.inserted, 1)
}

func (s *IdempotentWriteSuite) writeOutcomeLabel() string {
	metrics, ok := s.obs.Metrics().(*fake.FakeMetrics)
	s.Require().True(ok, "provider fake deve expor FakeMetrics")
	counter := metrics.GetCounter("agents_write_total")
	s.Require().NotNil(counter, "counter agents_write_total deve ter sido emitido")
	values := counter.GetValues()
	s.Require().Len(values, 1)
	for _, f := range values[0].Fields {
		if f.Key == "outcome" {
			return f.StringValue()
		}
	}
	s.Fail("label outcome ausente na métrica")
	return ""
}

func (s *IdempotentWriteSuite) TestDomainError_EmitsDomainRejectedOutcome() {
	domainErr := errors.New("nickname conflict")
	classifierCalls := 0
	classifier := func(err error) bool {
		classifierCalls++
		return errors.Is(err, domainErr)
	}
	ledger := &fakeLedger{found: false}
	uc := NewIdempotentWrite(ledger, s.obs)

	result, err := uc.Execute(
		s.ctx, s.userID, s.wamid, 0, "create_card", "card",
		func(_ context.Context) (uuid.UUID, bool, error) { return uuid.Nil, false, domainErr },
		classifier,
	)

	s.Error(err)
	s.ErrorIs(err, domainErr)
	s.Equal(IdempotentWriteResult{}, result)
	s.Empty(ledger.inserted)
	s.Equal(1, classifierCalls)
	s.Equal("domain_rejected", s.writeOutcomeLabel())
}

func (s *IdempotentWriteSuite) TestInfraError_EmitsUsecaseErrorOutcome() {
	infraErr := errors.New("db timeout")
	classifier := func(error) bool { return false }
	ledger := &fakeLedger{found: false}
	uc := NewIdempotentWrite(ledger, s.obs)

	result, err := uc.Execute(
		s.ctx, s.userID, s.wamid, 0, "create_card", "card",
		func(_ context.Context) (uuid.UUID, bool, error) { return uuid.Nil, false, infraErr },
		classifier,
	)

	s.Error(err)
	s.ErrorIs(err, infraErr)
	s.Equal(IdempotentWriteResult{}, result)
	s.Empty(ledger.inserted)
	s.Equal("usecase_error", s.writeOutcomeLabel())
}

func (s *IdempotentWriteSuite) TestInfraError_NilClassifier_EmitsUsecaseErrorOutcome() {
	infraErr := errors.New("db timeout")
	ledger := &fakeLedger{found: false}
	uc := NewIdempotentWrite(ledger, s.obs)

	_, err := uc.Execute(
		s.ctx, s.userID, s.wamid, 0, "create_transaction", "transaction",
		func(_ context.Context) (uuid.UUID, bool, error) { return uuid.Nil, false, infraErr },
		nil,
	)

	s.Error(err)
	s.Equal("usecase_error", s.writeOutcomeLabel())
}
