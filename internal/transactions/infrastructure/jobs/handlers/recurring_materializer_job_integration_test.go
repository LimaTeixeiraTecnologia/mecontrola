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
	dir := valueobjects.DirectionIncome
	pm := valueobjects.PaymentMethodPix
	amount, _ := valueobjects.NewMoney(5000)
	desc, _ := valueobjects.NewDescription("Salário")
	catID := valueobjects.CategoryIDFromUUID(uuid.New())
	freq := valueobjects.FrequencyMonthly
	dom, _ := valueobjects.NewDayOfMonth(day)
	inst, _ := valueobjects.NewInstallmentCount(1)
	now := time.Now().UTC()
	t := entities.NewRecurringTemplate(
		uuid.New(),
		valueobjects.UserIDFromUUID(userID),
		dir, pm,
		option.None[valueobjects.CardID](),
		amount, desc, catID,
		option.None[valueobjects.SubcategoryID](),
		"Receita", "",
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
	mgr, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	loc, err := time.LoadLocation("America/Sao_Paulo")
	s.Require().NoError(err)

	today := time.Now().In(loc)
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
