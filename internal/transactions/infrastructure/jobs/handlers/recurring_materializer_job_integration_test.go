//go:build integration

package handlers_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
	handlers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/jobs/handlers"
	txpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/repositories/postgres"
)

type RecurringMaterializerJobIntegrationSuite struct {
	suite.Suite
}

func TestRecurringMaterializerJobIntegrationSuite(t *testing.T) {
	suite.Run(t, new(RecurringMaterializerJobIntegrationSuite))
}

func (s *RecurringMaterializerJobIntegrationSuite) buildTemplate(userID uuid.UUID, day int) *entities.RecurringTemplate {
	pm := valueobjects.PaymentMethodPix
	amount, _ := valueobjects.NewMoney(5000)
	desc, _ := valueobjects.NewDescription("Salário")
	seedIncomeRootID := uuid.MustParse("86dd34b0-7342-525a-9a30-b1b5a76b109f")
	seedIncomeLeafID := uuid.MustParse("98455e74-b1f3-5b9c-a8d8-05db0cdb465d")
	freq := valueobjects.FrequencyMonthly
	dom, _ := valueobjects.NewDayOfMonth(day)
	inst, _ := valueobjects.NewInstallmentCount(1)
	now := time.Now().UTC()
	ev := valueobjects.ReconstituteEvidence(
		seedIncomeRootID, seedIncomeLeafID, "income", "salario/decimo-terceiro",
		"matched", 1.0, "high", "exact", "canonical_name", "salário",
		"matched canonical_name salário", valueobjects.CategoryDecisionSourceAutoMatched,
		10, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	)
	t := entities.NewRecurringTemplate(
		uuid.New(),
		valueobjects.UserIDFromUUID(userID),
		valueobjects.DirectionIncome, pm,
		option.None[valueobjects.CardID](),
		amount, desc,
		valueobjects.CategoryIDFromUUID(seedIncomeRootID),
		option.None[valueobjects.SubcategoryID](),
		"Salário", "Décimo Terceiro",
		ev,
		freq, dom, inst,
		now.Add(-24*time.Hour), option.None[time.Time](), now,
	)
	return &t
}

func (s *RecurringMaterializerJobIntegrationSuite) TestJobRunsCorrectly() {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	s.Require().NoError(err)

	cfg := configs.TransactionsConfig{RecurringMaterializerCron: "@daily"}
	job := handlers.NewRecurringMaterializerJob(
		&stubMaterializeUseCase{},
		loc,
		cfg,
	)

	s.Equal("transactions-recurring-materializer", job.Name())
	s.Equal("@daily", job.Schedule())
}

func (s *RecurringMaterializerJobIntegrationSuite) TestTwoExecutions_OnlyOneMaterializationRow() {
	db, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	ctx := context.Background()

	loc, err := time.LoadLocation("America/Sao_Paulo")
	s.Require().NoError(err)

	today := time.Date(2026, time.January, 15, 12, 0, 0, 0, loc)
	dayOfMonth := today.Day()

	userID := uuid.New()
	template := s.buildTemplate(userID, dayOfMonth)

	templateRepo := txpostgres.NewRecurringTemplateRepository(o11y, db)
	s.Require().NoError(templateRepo.Create(ctx, template))

	materializationRepo := txpostgres.NewRecurringMaterializationRepository(o11y, db)
	refMonth := valueobjects.RefMonthFromTime(today, loc)
	now := time.Now().UTC()

	inserted1, err1 := materializationRepo.InsertIfAbsent(ctx, template.ID(), refMonth, nil, nil, now)
	s.Require().NoError(err1)
	s.True(inserted1)

	inserted2, err2 := materializationRepo.InsertIfAbsent(ctx, template.ID(), refMonth, nil, nil, now)
	s.Require().NoError(err2)
	s.False(inserted2, "segunda execução não deve inserir linha duplicada")
}

type stubMaterializeUseCase struct{}

func (u *stubMaterializeUseCase) Execute(ctx context.Context, today time.Time) error {
	return nil
}
