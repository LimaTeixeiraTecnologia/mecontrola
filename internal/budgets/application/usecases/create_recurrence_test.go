package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type CreateRecurrenceSuite struct {
	suite.Suite
	ctx     context.Context
	factory *mockInterfaces.RepositoryFactory
	repo    *mockInterfaces.BudgetRepository
	uow     *uowMocks.UnitOfWorkVoid
	useCase *usecases.CreateRecurrence
}

func TestCreateRecurrenceSuite(t *testing.T) {
	suite.Run(t, new(CreateRecurrenceSuite))
}

func (s *CreateRecurrenceSuite) SetupTest() {
	s.ctx = context.Background()
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.repo = mockInterfaces.NewBudgetRepository(s.T())
	s.factory.EXPECT().BudgetRepository(mock.Anything).Return(s.repo).Maybe()
	s.uow = uowMocks.NewUnitOfWorkVoid(s.T())
	s.useCase = usecases.NewCreateRecurrence(s.factory, s.uow, noop.NewProvider())
}

func buildSourceBudget(userID uuid.UUID, comp valueobjects.Competence) entities.Budget {
	now := time.Now().UTC()
	b := entities.NewBudget(userID, comp, 100000, now)
	slug1, _ := valueobjects.ParseRootSlug("expense.custo_fixo")
	slug2, _ := valueobjects.ParseRootSlug("expense.conhecimento")
	b.SetAllocations([]entities.Allocation{
		entities.NewAllocation(b.ID(), slug1, 6000, 60000),
		entities.NewAllocation(b.ID(), slug2, 4000, 40000),
	})
	return b
}

func (s *CreateRecurrenceSuite) TestExecute_InvalidUserID() {
	_, err := s.useCase.Execute(s.ctx, input.CreateRecurrenceInput{
		UserID:           "not-a-uuid",
		SourceCompetence: "2026-06",
		Months:           1,
	})

	s.ErrorIs(err, usecases.ErrBudgetInvalidUserID)
}

func (s *CreateRecurrenceSuite) TestExecute_InvalidCompetence() {
	_, err := s.useCase.Execute(s.ctx, input.CreateRecurrenceInput{
		UserID:           uuid.New().String(),
		SourceCompetence: "bad",
		Months:           1,
	})

	s.ErrorIs(err, usecases.ErrBudgetInvalidCompetence)
}

func (s *CreateRecurrenceSuite) TestExecute_InvalidMonthsZero() {
	_, err := s.useCase.Execute(s.ctx, input.CreateRecurrenceInput{
		UserID:           uuid.New().String(),
		SourceCompetence: "2026-06",
		Months:           0,
	})

	s.ErrorIs(err, usecases.ErrRecurrenceInvalidMonths)
}

func (s *CreateRecurrenceSuite) TestExecute_InvalidMonthsAbove12() {
	_, err := s.useCase.Execute(s.ctx, input.CreateRecurrenceInput{
		UserID:           uuid.New().String(),
		SourceCompetence: "2026-06",
		Months:           13,
	})

	s.ErrorIs(err, usecases.ErrRecurrenceInvalidMonths)
}

func (s *CreateRecurrenceSuite) TestExecute_SourceNotFound() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")

	s.repo.EXPECT().
		GetByUserCompetence(s.ctx, userID, comp).
		Return(entities.Budget{}, interfaces.ErrBudgetNotFound).
		Once()

	_, err := s.useCase.Execute(s.ctx, input.CreateRecurrenceInput{
		UserID:           userID.String(),
		SourceCompetence: "2026-06",
		Months:           1,
	})

	s.ErrorIs(err, usecases.ErrRecurrenceSourceInvalid)
}

func (s *CreateRecurrenceSuite) TestExecute_SourceNegativeTotal() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")
	now := time.Now().UTC()

	zeroBudget := entities.NewBudget(userID, comp, 0, now)

	s.repo.EXPECT().
		GetByUserCompetence(s.ctx, userID, comp).
		Return(zeroBudget, nil).
		Once()

	_, err := s.useCase.Execute(s.ctx, input.CreateRecurrenceInput{
		UserID:           userID.String(),
		SourceCompetence: "2026-06",
		Months:           1,
	})

	s.ErrorIs(err, usecases.ErrRecurrenceSourceNegativeTotal)
}

func (s *CreateRecurrenceSuite) TestExecute_SourceDraftWithPartialAllocations() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")
	now := time.Now().UTC()

	b := entities.NewBudget(userID, comp, 100000, now)
	slug1, _ := valueobjects.ParseRootSlug("expense.custo_fixo")
	b.SetAllocations([]entities.Allocation{
		entities.NewAllocation(b.ID(), slug1, 5000, 0),
	})

	s.repo.EXPECT().
		GetByUserCompetence(s.ctx, userID, comp).
		Return(b, nil).
		Once()

	_, err := s.useCase.Execute(s.ctx, input.CreateRecurrenceInput{
		UserID:           userID.String(),
		SourceCompetence: "2026-06",
		Months:           1,
	})

	s.ErrorIs(err, usecases.ErrRecurrenceSourceDraftWithoutFullAllocs)
}

func (s *CreateRecurrenceSuite) TestExecute_RF21a_CreatesNewBudget() {
	userID := uuid.New()
	sourceComp, _ := valueobjects.NewCompetence("2026-06")
	nextComp, _ := valueobjects.NewCompetence("2026-07")

	source := buildSourceBudget(userID, sourceComp)

	s.repo.EXPECT().
		GetByUserCompetence(s.ctx, userID, sourceComp).
		Return(source, nil).
		Once()

	s.repo.EXPECT().
		GetByUserCompetence(s.ctx, userID, nextComp).
		Return(entities.Budget{}, interfaces.ErrBudgetNotFound).
		Once()

	s.repo.EXPECT().
		CreateDraft(s.ctx, mock.Anything).
		Return(nil).
		Once()

	result, err := s.useCase.Execute(s.ctx, input.CreateRecurrenceInput{
		UserID:           userID.String(),
		SourceCompetence: "2026-06",
		Months:           1,
	})

	s.NoError(err)
	s.Equal("2026-06", result.SourceCompetence)
	s.Len(result.Results, 1)
	s.Equal(output.RecurrenceStatusCreated, result.Results[0].Status)
	s.Equal("2026-07", result.Results[0].Competence)
}

func (s *CreateRecurrenceSuite) TestExecute_RF21b_UpdatesExistingDraft() {
	userID := uuid.New()
	sourceComp, _ := valueobjects.NewCompetence("2026-06")
	nextComp, _ := valueobjects.NewCompetence("2026-07")
	now := time.Now().UTC()

	source := buildSourceBudget(userID, sourceComp)

	existingDraft := entities.NewBudget(userID, nextComp, 50000, now)
	slug1, _ := valueobjects.ParseRootSlug("expense.custo_fixo")
	existingDraft.SetAllocations([]entities.Allocation{
		entities.NewAllocation(existingDraft.ID(), slug1, 10000, 50000),
	})

	s.repo.EXPECT().
		GetByUserCompetence(s.ctx, userID, sourceComp).
		Return(source, nil).
		Once()

	s.repo.EXPECT().
		GetByUserCompetence(s.ctx, userID, nextComp).
		Return(existingDraft, nil).
		Once()

	s.repo.EXPECT().
		Activate(s.ctx, mock.Anything).
		Return(nil).
		Once()

	result, err := s.useCase.Execute(s.ctx, input.CreateRecurrenceInput{
		UserID:           userID.String(),
		SourceCompetence: "2026-06",
		Months:           1,
	})

	s.NoError(err)
	s.Len(result.Results, 1)
	s.Equal(output.RecurrenceStatusUpdated, result.Results[0].Status)
}

func (s *CreateRecurrenceSuite) TestExecute_RF21b_CompletedFromAutoDraft() {
	userID := uuid.New()
	sourceComp, _ := valueobjects.NewCompetence("2026-06")
	nextComp, _ := valueobjects.NewCompetence("2026-07")
	now := time.Now().UTC()

	source := buildSourceBudget(userID, sourceComp)

	autoDraft := entities.NewAutoDraftBudget(userID, nextComp, now)

	s.repo.EXPECT().
		GetByUserCompetence(s.ctx, userID, sourceComp).
		Return(source, nil).
		Once()

	s.repo.EXPECT().
		GetByUserCompetence(s.ctx, userID, nextComp).
		Return(autoDraft, nil).
		Once()

	s.repo.EXPECT().
		Activate(s.ctx, mock.Anything).
		Return(nil).
		Once()

	result, err := s.useCase.Execute(s.ctx, input.CreateRecurrenceInput{
		UserID:           userID.String(),
		SourceCompetence: "2026-06",
		Months:           1,
	})

	s.NoError(err)
	s.Len(result.Results, 1)
	s.Equal(output.RecurrenceStatusCompletedFromDraft, result.Results[0].Status)
}

func (s *CreateRecurrenceSuite) TestExecute_RF23a_ConflictOnActiveExisting() {
	userID := uuid.New()
	sourceComp, _ := valueobjects.NewCompetence("2026-06")
	nextComp, _ := valueobjects.NewCompetence("2026-07")
	now := time.Now().UTC()

	source := buildSourceBudget(userID, sourceComp)

	activeNext := entities.HydrateBudget(
		uuid.New(), userID, nextComp, 100000,
		entities.BudgetStateActive,
		&now,
		false,
		nil,
		now,
		now,
	)

	s.repo.EXPECT().
		GetByUserCompetence(s.ctx, userID, sourceComp).
		Return(source, nil).
		Once()

	s.repo.EXPECT().
		GetByUserCompetence(s.ctx, userID, nextComp).
		Return(activeNext, nil).
		Once()

	result, err := s.useCase.Execute(s.ctx, input.CreateRecurrenceInput{
		UserID:           userID.String(),
		SourceCompetence: "2026-06",
		Months:           1,
	})

	s.NoError(err)
	s.Len(result.Results, 1)
	s.Equal(output.RecurrenceStatusConflict, result.Results[0].Status)
}

func (s *CreateRecurrenceSuite) TestExecute_SourceAutoDraftWithoutAllocations() {
	userID := uuid.New()
	comp, _ := valueobjects.NewCompetence("2026-06")
	now := time.Now().UTC()

	autoDraftNoAllocs := entities.HydrateBudget(
		uuid.New(), userID, comp, 100000,
		entities.BudgetStateDraft,
		nil,
		true,
		nil,
		now,
		now,
	)

	s.repo.EXPECT().
		GetByUserCompetence(s.ctx, userID, comp).
		Return(autoDraftNoAllocs, nil).
		Once()

	_, err := s.useCase.Execute(s.ctx, input.CreateRecurrenceInput{
		UserID:           userID.String(),
		SourceCompetence: "2026-06",
		Months:           1,
	})

	s.ErrorIs(err, usecases.ErrRecurrenceSourceAutoDraftWithoutAllocs)
}

func (s *CreateRecurrenceSuite) TestExecute_ConflictOnCreateDraft() {
	userID := uuid.New()
	sourceComp, _ := valueobjects.NewCompetence("2026-06")
	nextComp, _ := valueobjects.NewCompetence("2026-07")

	source := buildSourceBudget(userID, sourceComp)

	s.repo.EXPECT().
		GetByUserCompetence(s.ctx, userID, sourceComp).
		Return(source, nil).
		Once()

	s.repo.EXPECT().
		GetByUserCompetence(s.ctx, userID, nextComp).
		Return(entities.Budget{}, interfaces.ErrBudgetNotFound).
		Once()

	s.repo.EXPECT().
		CreateDraft(s.ctx, mock.Anything).
		Return(interfaces.ErrBudgetConflict).
		Once()

	result, err := s.useCase.Execute(s.ctx, input.CreateRecurrenceInput{
		UserID:           userID.String(),
		SourceCompetence: "2026-06",
		Months:           1,
	})

	s.NoError(err)
	s.Len(result.Results, 1)
	s.Equal(output.RecurrenceStatusConflict, result.Results[0].Status)
}

func (s *CreateRecurrenceSuite) TestExecute_ActivateErrorOnExistingDraft() {
	userID := uuid.New()
	sourceComp, _ := valueobjects.NewCompetence("2026-06")
	nextComp, _ := valueobjects.NewCompetence("2026-07")
	now := time.Now().UTC()

	source := buildSourceBudget(userID, sourceComp)
	existingDraft := entities.NewBudget(userID, nextComp, 50000, now)

	s.repo.EXPECT().
		GetByUserCompetence(s.ctx, userID, sourceComp).
		Return(source, nil).
		Once()

	s.repo.EXPECT().
		GetByUserCompetence(s.ctx, userID, nextComp).
		Return(existingDraft, nil).
		Once()

	s.repo.EXPECT().
		Activate(s.ctx, mock.Anything).
		Return(errors.New("db failure")).
		Once()

	result, err := s.useCase.Execute(s.ctx, input.CreateRecurrenceInput{
		UserID:           userID.String(),
		SourceCompetence: "2026-06",
		Months:           1,
	})

	s.NoError(err)
	s.Len(result.Results, 1)
	s.Equal(output.RecurrenceStatusFailure, result.Results[0].Status)
}

func (s *CreateRecurrenceSuite) TestExecute_MultipleMonths() {
	userID := uuid.New()
	sourceComp, _ := valueobjects.NewCompetence("2026-06")

	source := buildSourceBudget(userID, sourceComp)

	s.repo.EXPECT().
		GetByUserCompetence(s.ctx, userID, sourceComp).
		Return(source, nil).
		Once()

	comp07, _ := valueobjects.NewCompetence("2026-07")
	comp08, _ := valueobjects.NewCompetence("2026-08")
	comp09, _ := valueobjects.NewCompetence("2026-09")

	s.repo.EXPECT().
		GetByUserCompetence(s.ctx, userID, comp07).
		Return(entities.Budget{}, interfaces.ErrBudgetNotFound).
		Once()
	s.repo.EXPECT().
		CreateDraft(s.ctx, mock.Anything).
		Return(nil).
		Times(3)
	s.repo.EXPECT().
		GetByUserCompetence(s.ctx, userID, comp08).
		Return(entities.Budget{}, interfaces.ErrBudgetNotFound).
		Once()
	s.repo.EXPECT().
		GetByUserCompetence(s.ctx, userID, comp09).
		Return(entities.Budget{}, interfaces.ErrBudgetNotFound).
		Once()

	result, err := s.useCase.Execute(s.ctx, input.CreateRecurrenceInput{
		UserID:           userID.String(),
		SourceCompetence: "2026-06",
		Months:           3,
	})

	s.NoError(err)
	s.Len(result.Results, 3)
	for _, r := range result.Results {
		s.Equal(output.RecurrenceStatusCreated, r.Status)
	}
}
