package usecases_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type spyUoWExpense struct {
	calls int
}

func (s *spyUoWExpense) DBTX() database.DBTX { return nil }

func (s *spyUoWExpense) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	s.calls++
	return fn(ctx, nil)
}

type spyUoWVoid struct {
	calls int
}

func (s *spyUoWVoid) DBTX() database.DBTX { return nil }

func (s *spyUoWVoid) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	s.calls++
	return fn(ctx, nil)
}

type ApplyPendingEventSuite struct {
	suite.Suite
	ctx        context.Context
	factory    *mockInterfaces.RepositoryFactory
	expenses   *mockInterfaces.ExpenseRepository
	budgets    *mockInterfaces.BudgetRepository
	categories *mockInterfaces.CategoriesReader
	publisher  *mockInterfaces.ExpenseCommittedPublisher
	uowExpense *uowMocks.UnitOfWorkExpense
	uowVoid    *uowMocks.UnitOfWorkVoid
	useCase    *usecases.ApplyPendingEvent
}

func TestApplyPendingEventSuite(t *testing.T) {
	suite.Run(t, new(ApplyPendingEventSuite))
}

func (s *ApplyPendingEventSuite) SetupTest() {
	s.ctx = context.Background()
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.expenses = mockInterfaces.NewExpenseRepository(s.T())
	s.budgets = mockInterfaces.NewBudgetRepository(s.T())
	s.factory.EXPECT().ExpenseRepository(mock.Anything).Return(s.expenses).Maybe()
	s.factory.EXPECT().BudgetRepository(mock.Anything).Return(s.budgets).Maybe()
	s.categories = mockInterfaces.NewCategoriesReader(s.T())
	s.publisher = mockInterfaces.NewExpenseCommittedPublisher(s.T())
	s.uowExpense = uowMocks.NewUnitOfWorkExpense(s.T())
	s.uowVoid = uowMocks.NewUnitOfWorkVoid(s.T())

	autoDraft := usecases.NewCreateOrAutoDraftForExpense(s.factory)
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	upsert := usecases.NewUpsertExpense(s.factory, s.categories, s.publisher, autoDraft, s.uowExpense, noop.NewProvider(), loc)
	del := usecases.NewDeleteExpense(s.factory, s.publisher, s.uowVoid, noop.NewProvider(), loc)

	s.useCase = usecases.NewApplyPendingEvent(s.factory, upsert, del, 24*time.Hour, noop.NewProvider())
}

func (s *ApplyPendingEventSuite) buildPayload(subcategoryID string, competence string, amountCents int64, occurredAt time.Time) []byte {
	type pendingEventPayload struct {
		SubcategoryID string    `json:"subcategory_id"`
		Competence    string    `json:"competence"`
		AmountCents   int64     `json:"amount_cents"`
		OccurredAt    time.Time `json:"occurred_at"`
	}
	raw, _ := json.Marshal(pendingEventPayload{
		SubcategoryID: subcategoryID,
		Competence:    competence,
		AmountCents:   amountCents,
		OccurredAt:    occurredAt,
	})
	return raw
}

func (s *ApplyPendingEventSuite) buildExistingExpense(userID uuid.UUID, extIDStr string, version int64) entities.Expense {
	source, _ := valueobjects.NewProducerSource("kiwify")
	extID, _ := valueobjects.NewExternalTransactionID(extIDStr)
	competence, _ := valueobjects.NewCompetence("2026-06")
	rootSlug, _ := valueobjects.ParseRootSlug("expense.custo_fixo")
	subID := uuid.New()
	now := time.Now().UTC()
	return entities.HydrateExpense(uuid.New(), userID, source, extID, subID, rootSlug, competence, 5000, now, version, nil, nil, now, now)
}

func (s *ApplyPendingEventSuite) TestCreate_ExpenseNotFound_Applied() {
	userID := uuid.New()
	extIDStr := "00000000-0000-4000-8000-000000000001"
	source, _ := valueobjects.NewProducerSource("kiwify")
	extID, _ := valueobjects.NewExternalTransactionID(extIDStr)
	subID := uuid.New()
	payload := s.buildPayload(subID.String(), "2026-06", 5000, time.Now().UTC())

	evt := entities.NewPendingEvent(uuid.New(), source, userID, extID, 1, valueobjects.MutationKindCreate, payload, time.Now().UTC())

	s.expenses.EXPECT().
		GetByIdentity(s.ctx, mock.Anything).
		Return(entities.Expense{}, entities.ExpenseTombstone{}, interfaces.ErrExpenseNotFound).
		Once()

	s.categories.EXPECT().
		ValidateExpenseSubcategory(s.ctx, mock.Anything).
		Return("expense.custo_fixo", false, nil).
		Once()

	s.expenses.EXPECT().
		GetByIdentity(s.ctx, mock.Anything).
		Return(entities.Expense{}, entities.ExpenseTombstone{}, interfaces.ErrExpenseNotFound).
		Once()

	s.budgets.EXPECT().
		GetByUserCompetence(s.ctx, mock.Anything, mock.Anything).
		Return(entities.Budget{}, interfaces.ErrBudgetNotFound).
		Once()

	s.budgets.EXPECT().
		CreateDraft(s.ctx, mock.Anything).
		Return(nil).
		Once()

	s.expenses.EXPECT().
		Insert(s.ctx, mock.Anything).
		Return(nil).
		Once()

	s.publisher.EXPECT().
		Publish(s.ctx, mock.Anything, mock.Anything).
		Return(nil).
		Once()

	outcome, err := s.useCase.Execute(s.ctx, nil, evt)

	s.NoError(err)
	s.Equal(usecases.PendingEventOutcomeApplied, outcome)
}

func (s *ApplyPendingEventSuite) TestCreate_ExpenseAlreadyExists_ObsoleteIdempotent() {
	userID := uuid.New()
	extIDStr := "00000000-0000-4000-8000-000000000002"
	source, _ := valueobjects.NewProducerSource("kiwify")
	extID, _ := valueobjects.NewExternalTransactionID(extIDStr)
	subID := uuid.New()
	payload := s.buildPayload(subID.String(), "2026-06", 5000, time.Now().UTC())

	existing := s.buildExistingExpense(userID, extIDStr, 1)
	evt := entities.NewPendingEvent(uuid.New(), source, userID, extID, 1, valueobjects.MutationKindCreate, payload, time.Now().UTC())

	s.expenses.EXPECT().
		GetByIdentity(s.ctx, mock.Anything).
		Return(existing, entities.ExpenseTombstone{}, nil).
		Once()

	outcome, err := s.useCase.Execute(s.ctx, nil, evt)

	s.NoError(err)
	s.Equal(usecases.PendingEventOutcomeObsoleteIdempotent, outcome)
}

func (s *ApplyPendingEventSuite) TestUpdate_VersionMatch_Applied() {
	userID := uuid.New()
	extIDStr := "00000000-0000-4000-8000-000000000003"
	source, _ := valueobjects.NewProducerSource("kiwify")
	extID, _ := valueobjects.NewExternalTransactionID(extIDStr)
	subID := uuid.New()
	payload := s.buildPayload(subID.String(), "2026-06", 7000, time.Now().UTC())

	existing := s.buildExistingExpense(userID, extIDStr, 1)
	evt := entities.NewPendingEvent(uuid.New(), source, userID, extID, 2, valueobjects.MutationKindUpdate, payload, time.Now().UTC())

	s.expenses.EXPECT().
		GetByIdentity(s.ctx, mock.Anything).
		Return(existing, entities.ExpenseTombstone{}, nil).
		Once()

	s.categories.EXPECT().
		ValidateExpenseSubcategory(s.ctx, mock.Anything).
		Return("expense.custo_fixo", false, nil).
		Once()

	s.expenses.EXPECT().
		GetByIdentity(s.ctx, mock.Anything).
		Return(existing, entities.ExpenseTombstone{}, nil).
		Once()

	s.expenses.EXPECT().
		Update(s.ctx, mock.Anything, int64(1)).
		Return(nil).
		Once()

	s.publisher.EXPECT().
		Publish(s.ctx, mock.Anything, mock.Anything).
		Return(nil).
		Once()

	outcome, err := s.useCase.Execute(s.ctx, nil, evt)

	s.NoError(err)
	s.Equal(usecases.PendingEventOutcomeApplied, outcome)
}

func (s *ApplyPendingEventSuite) TestUpdate_VersionGap_StillPending() {
	userID := uuid.New()
	extIDStr := "00000000-0000-4000-8000-000000000004"
	source, _ := valueobjects.NewProducerSource("kiwify")
	extID, _ := valueobjects.NewExternalTransactionID(extIDStr)
	subID := uuid.New()
	payload := s.buildPayload(subID.String(), "2026-06", 7000, time.Now().UTC())

	existing := s.buildExistingExpense(userID, extIDStr, 1)
	evt := entities.NewPendingEvent(uuid.New(), source, userID, extID, 3, valueobjects.MutationKindUpdate, payload, time.Now().UTC())

	s.expenses.EXPECT().
		GetByIdentity(s.ctx, mock.Anything).
		Return(existing, entities.ExpenseTombstone{}, nil).
		Once()

	outcome, err := s.useCase.Execute(s.ctx, nil, evt)

	s.NoError(err)
	s.Equal(usecases.PendingEventOutcomeStillPending, outcome)
}

func (s *ApplyPendingEventSuite) TestUpdate_ExpenseNotFound_StillPending() {
	userID := uuid.New()
	extIDStr := "00000000-0000-4000-8000-000000000005"
	source, _ := valueobjects.NewProducerSource("kiwify")
	extID, _ := valueobjects.NewExternalTransactionID(extIDStr)
	subID := uuid.New()
	payload := s.buildPayload(subID.String(), "2026-06", 7000, time.Now().UTC())

	evt := entities.NewPendingEvent(uuid.New(), source, userID, extID, 2, valueobjects.MutationKindUpdate, payload, time.Now().UTC())

	s.expenses.EXPECT().
		GetByIdentity(s.ctx, mock.Anything).
		Return(entities.Expense{}, entities.ExpenseTombstone{}, interfaces.ErrExpenseNotFound).
		Once()

	outcome, err := s.useCase.Execute(s.ctx, nil, evt)

	s.NoError(err)
	s.Equal(usecases.PendingEventOutcomeStillPending, outcome)
}

func (s *ApplyPendingEventSuite) TestDelete_VersionMatch_Applied() {
	userID := uuid.New()
	extIDStr := "00000000-0000-4000-8000-000000000006"
	source, _ := valueobjects.NewProducerSource("kiwify")
	extID, _ := valueobjects.NewExternalTransactionID(extIDStr)

	existing := s.buildExistingExpense(userID, extIDStr, 1)
	evt := entities.NewPendingEvent(uuid.New(), source, userID, extID, 2, valueobjects.MutationKindDelete, []byte("{}"), time.Now().UTC())

	s.expenses.EXPECT().
		GetByIdentity(s.ctx, mock.Anything).
		Return(existing, entities.ExpenseTombstone{}, nil).
		Once()

	s.expenses.EXPECT().
		GetByIdentity(s.ctx, mock.Anything).
		Return(existing, entities.ExpenseTombstone{}, nil).
		Once()

	s.expenses.EXPECT().
		SoftDelete(s.ctx, mock.Anything, int64(1)).
		Return(int64(2), nil).
		Once()

	s.publisher.EXPECT().
		Publish(s.ctx, mock.Anything, mock.Anything).
		Return(nil).
		Once()

	outcome, err := s.useCase.Execute(s.ctx, nil, evt)

	s.NoError(err)
	s.Equal(usecases.PendingEventOutcomeApplied, outcome)
}

func (s *ApplyPendingEventSuite) TestExpiredEvent_Expired() {
	userID := uuid.New()
	extIDStr := "00000000-0000-4000-8000-000000000007"
	source, _ := valueobjects.NewProducerSource("kiwify")
	extID, _ := valueobjects.NewExternalTransactionID(extIDStr)

	receivedAt := time.Now().UTC().Add(-48 * time.Hour)
	evt := entities.NewPendingEvent(uuid.New(), source, userID, extID, 1, valueobjects.MutationKindCreate, []byte("{}"), receivedAt)

	outcome, err := s.useCase.Execute(s.ctx, nil, evt)

	s.NoError(err)
	s.Equal(usecases.PendingEventOutcomeExpired, outcome)
}

func (s *ApplyPendingEventSuite) TestAtomicity_ReusesCallerTransaction_NoNestedUoW() {
	factory := mockInterfaces.NewRepositoryFactory(s.T())
	expenses := mockInterfaces.NewExpenseRepository(s.T())
	budgets := mockInterfaces.NewBudgetRepository(s.T())
	factory.EXPECT().ExpenseRepository(mock.Anything).Return(expenses).Maybe()
	factory.EXPECT().BudgetRepository(mock.Anything).Return(budgets).Maybe()
	categories := mockInterfaces.NewCategoriesReader(s.T())
	publisher := mockInterfaces.NewExpenseCommittedPublisher(s.T())

	upsertSpy := &spyUoWExpense{}
	deleteSpy := &spyUoWVoid{}

	autoDraft := usecases.NewCreateOrAutoDraftForExpense(factory)
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	upsert := usecases.NewUpsertExpense(factory, categories, publisher, autoDraft, upsertSpy, noop.NewProvider(), loc)
	del := usecases.NewDeleteExpense(factory, publisher, deleteSpy, noop.NewProvider(), loc)
	useCase := usecases.NewApplyPendingEvent(factory, upsert, del, 24*time.Hour, noop.NewProvider())

	userID := uuid.New()
	extIDStr := "00000000-0000-4000-8000-000000000099"
	source, _ := valueobjects.NewProducerSource("kiwify")
	extID, _ := valueobjects.NewExternalTransactionID(extIDStr)
	subID := uuid.New()
	payload := s.buildPayload(subID.String(), "2026-06", 5000, time.Now().UTC())

	evt := entities.NewPendingEvent(uuid.New(), source, userID, extID, 1, valueobjects.MutationKindCreate, payload, time.Now().UTC())

	expenses.EXPECT().
		GetByIdentity(s.ctx, mock.Anything).
		Return(entities.Expense{}, entities.ExpenseTombstone{}, interfaces.ErrExpenseNotFound).
		Once()

	categories.EXPECT().
		ValidateExpenseSubcategory(s.ctx, mock.Anything).
		Return("expense.custo_fixo", false, nil).
		Once()

	expenses.EXPECT().
		GetByIdentity(s.ctx, mock.Anything).
		Return(entities.Expense{}, entities.ExpenseTombstone{}, interfaces.ErrExpenseNotFound).
		Once()

	budgets.EXPECT().
		GetByUserCompetence(s.ctx, mock.Anything, mock.Anything).
		Return(entities.Budget{}, interfaces.ErrBudgetNotFound).
		Once()

	budgets.EXPECT().
		CreateDraft(s.ctx, mock.Anything).
		Return(nil).
		Once()

	expenses.EXPECT().
		Insert(s.ctx, mock.Anything).
		Return(nil).
		Once()

	publisher.EXPECT().
		Publish(s.ctx, mock.Anything, mock.Anything).
		Return(nil).
		Once()

	outcome, err := useCase.Execute(s.ctx, nil, evt)

	s.NoError(err)
	s.Equal(usecases.PendingEventOutcomeApplied, outcome)
	s.Equal(0, upsertSpy.calls, "ApplyPendingEvent não deve abrir nova UnitOfWork no upsert")
	s.Equal(0, deleteSpy.calls, "ApplyPendingEvent não deve abrir nova UnitOfWork no delete")
}

func (s *ApplyPendingEventSuite) TestAtomicity_MutationFailurePropagates_NoPendingTransition() {
	factory := mockInterfaces.NewRepositoryFactory(s.T())
	expenses := mockInterfaces.NewExpenseRepository(s.T())
	budgets := mockInterfaces.NewBudgetRepository(s.T())
	factory.EXPECT().ExpenseRepository(mock.Anything).Return(expenses).Maybe()
	factory.EXPECT().BudgetRepository(mock.Anything).Return(budgets).Maybe()
	categories := mockInterfaces.NewCategoriesReader(s.T())
	publisher := mockInterfaces.NewExpenseCommittedPublisher(s.T())

	upsertSpy := &spyUoWExpense{}
	deleteSpy := &spyUoWVoid{}

	autoDraft := usecases.NewCreateOrAutoDraftForExpense(factory)
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	upsert := usecases.NewUpsertExpense(factory, categories, publisher, autoDraft, upsertSpy, noop.NewProvider(), loc)
	del := usecases.NewDeleteExpense(factory, publisher, deleteSpy, noop.NewProvider(), loc)
	useCase := usecases.NewApplyPendingEvent(factory, upsert, del, 24*time.Hour, noop.NewProvider())

	userID := uuid.New()
	extIDStr := "00000000-0000-4000-8000-000000000100"
	source, _ := valueobjects.NewProducerSource("kiwify")
	extID, _ := valueobjects.NewExternalTransactionID(extIDStr)
	subID := uuid.New()
	payload := s.buildPayload(subID.String(), "2026-06", 5000, time.Now().UTC())

	evt := entities.NewPendingEvent(uuid.New(), source, userID, extID, 1, valueobjects.MutationKindCreate, payload, time.Now().UTC())

	expenses.EXPECT().
		GetByIdentity(s.ctx, mock.Anything).
		Return(entities.Expense{}, entities.ExpenseTombstone{}, interfaces.ErrExpenseNotFound).
		Once()

	categories.EXPECT().
		ValidateExpenseSubcategory(s.ctx, mock.Anything).
		Return("expense.custo_fixo", false, nil).
		Once()

	expenses.EXPECT().
		GetByIdentity(s.ctx, mock.Anything).
		Return(entities.Expense{}, entities.ExpenseTombstone{}, interfaces.ErrExpenseNotFound).
		Once()

	budgets.EXPECT().
		GetByUserCompetence(s.ctx, mock.Anything, mock.Anything).
		Return(entities.Budget{}, interfaces.ErrBudgetNotFound).
		Once()

	budgets.EXPECT().
		CreateDraft(s.ctx, mock.Anything).
		Return(nil).
		Once()

	wantErr := errors.New("simulated network failure between commits")
	expenses.EXPECT().
		Insert(s.ctx, mock.Anything).
		Return(wantErr).
		Once()

	outcome, err := useCase.Execute(s.ctx, nil, evt)

	s.Error(err)
	s.ErrorIs(err, wantErr)
	s.Equal(usecases.PendingEventOutcome(0), outcome)
	s.Equal(0, upsertSpy.calls, "falha deve propagar do ExecuteInTx sem abrir UoW separada")
}
