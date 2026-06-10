package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases/mocks"
	budgetsconfig "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/config"
)

type IngestExternalExpenseSuite struct {
	suite.Suite
	ctx     context.Context
	pending *mockInterfaces.PendingEventRepository
	uow     *uowMocks.UnitOfWorkVoid
	useCase *usecases.IngestExternalExpense
}

func TestIngestExternalExpenseSuite(t *testing.T) {
	suite.Run(t, new(IngestExternalExpenseSuite))
}

func (s *IngestExternalExpenseSuite) SetupTest() {
	s.ctx = context.Background()
	s.pending = mockInterfaces.NewPendingEventRepository(s.T())
	s.uow = uowMocks.NewUnitOfWorkVoid(s.T())
	s.useCase = usecases.NewIngestExternalExpense(s.pending, nil, nil, s.uow, noop.NewProvider())
}

func (s *IngestExternalExpenseSuite) validInput() usecases.IngestExternalExpenseInput {
	return usecases.IngestExternalExpenseInput{
		EventID:               uuid.New().String(),
		Source:                "kiwify",
		ExternalTransactionID: "00000000-0000-4000-8000-000000000099",
		OccurredAt:            time.Now().UTC(),
		UserID:                uuid.New().String(),
		Operation:             "create",
		Version:               1,
		SubcategoryID:         uuid.New().String(),
		Competence:            "2026-06",
		AmountCents:           1000,
	}
}

func (s *IngestExternalExpenseSuite) TestBug1_AllowlistContainsKiwify() {
	s.True(budgetsconfig.IsAllowedProducerSource("kiwify"),
		"AllowedProducerSources must contain at least one canonical producer per RT-28; empty allowlist silently drops all external events (BUG BUDG-INFRA-01)")
	s.NotEmpty(budgetsconfig.AllowedProducerSources,
		"AllowedProducerSources must not be empty per RT-28")
}

func (s *IngestExternalExpenseSuite) TestBug1_SourceOutsideAllowlistIsRejectedWithError() {
	in := s.validInput()
	in.Source = "unknown_producer"

	err := s.useCase.Execute(s.ctx, in)

	s.ErrorIs(err, usecases.ErrIngestExternalExpenseSourceRejected,
		"events from sources outside allowlist must return ErrIngestExternalExpenseSourceRejected (RF-32a/RF-32c)")
}

func (s *IngestExternalExpenseSuite) TestBug2_CreateWithVersionNotOneIsRejected() {
	in := s.validInput()
	in.Operation = "create"
	in.Version = 5

	err := s.useCase.Execute(s.ctx, in)

	s.ErrorIs(err, usecases.ErrIngestExternalExpenseInvalidVersionForCreate,
		"create events with version != 1 must be rejected without persisting pending (RF-36a)")
}

func (s *IngestExternalExpenseSuite) TestBug3_MissingEventIDReturnsStructuredError() {
	in := s.validInput()
	in.EventID = ""

	err := s.useCase.Execute(s.ctx, in)

	s.ErrorIs(err, usecases.ErrIngestExternalExpenseInvalidFields,
		"events with missing required fields must return ErrIngestExternalExpenseInvalidFields (RF-34/RF-39a)")
}

func (s *IngestExternalExpenseSuite) TestBug3_MissingUserIDReturnsStructuredError() {
	in := s.validInput()
	in.UserID = ""

	err := s.useCase.Execute(s.ctx, in)

	s.ErrorIs(err, usecases.ErrIngestExternalExpenseInvalidFields)
}

func (s *IngestExternalExpenseSuite) TestBug3_MissingOccurredAtReturnsStructuredError() {
	in := s.validInput()
	in.OccurredAt = time.Time{}

	err := s.useCase.Execute(s.ctx, in)

	s.ErrorIs(err, usecases.ErrIngestExternalExpenseInvalidFields)
}

func (s *IngestExternalExpenseSuite) TestBug4_MetricsCountersAreRegisteredAtConstruction() {
	pending := mockInterfaces.NewPendingEventRepository(s.T())
	uow := uowMocks.NewUnitOfWorkVoid(s.T())
	uc := usecases.NewIngestExternalExpense(pending, nil, nil, uow, noop.NewProvider())
	s.NotNil(uc, "constructor must register counters budgets_external_expense_source_rejected_total and budgets_external_expense_invalid_fields_total without panicking (RF-39c)")

	in := s.validInput()
	in.Source = "rogue_source"
	err := uc.Execute(s.ctx, in)
	s.ErrorIs(err, usecases.ErrIngestExternalExpenseSourceRejected,
		"executing with rejected source must hit sourceRejected counter and return error (BUDG-INFRA-02)")

	in2 := s.validInput()
	in2.ExternalTransactionID = ""
	err2 := uc.Execute(s.ctx, in2)
	s.ErrorIs(err2, usecases.ErrIngestExternalExpenseInvalidFields,
		"executing with missing fields must hit invalidFields counter and return error (BUDG-INFRA-02)")
}
