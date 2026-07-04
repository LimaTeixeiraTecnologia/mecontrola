//go:build integration

package handlers_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
	handlers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/jobs/handlers"
	txpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/repositories/postgres"
)

type MonthlySummaryReconcilerJobIntegrationSuite struct {
	suite.Suite
}

func TestMonthlySummaryReconcilerJobIntegrationSuite(t *testing.T) {
	suite.Run(t, new(MonthlySummaryReconcilerJobIntegrationSuite))
}

func (s *MonthlySummaryReconcilerJobIntegrationSuite) TestJobScheduleAndName() {
	cfg := configs.TransactionsConfig{MonthlySummaryReconcilerCron: "@daily"}
	job := handlers.NewMonthlySummaryReconcilerJob(&stubReconcileUseCase{}, cfg)
	s.Equal("transactions-monthly-summary-reconciler", job.Name())
	s.Equal("@daily", job.Schedule())
}

func (s *MonthlySummaryReconcilerJobIntegrationSuite) TestDriftDetectedAndCorrected() {
	db, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	ctx := context.Background()

	summaryRepo := txpostgres.NewMonthlySummaryRepository(o11y, db)

	userID := uuid.New()
	refMonth, _ := valueobjects.NewRefMonth("2026-06")
	now := time.Now().UTC()

	s.Require().NoError(summaryRepo.Upsert(ctx, userID, refMonth, 100000, 50000, now))

	existing, err := summaryRepo.Get(ctx, userID, refMonth)
	s.Require().NoError(err)
	s.Equal(int64(100000), existing.IncomeCents(), "drift artificial injetado")

	factory := &integrationRepositoryFactory{o11y: o11y}
	reconcileUc := usecases.NewReconcileMonthlySummary(
		factory.TransactionRepository(db),
		factory.CardInvoiceRepository(db),
		factory.MonthlySummaryRepository(db),
		48,
		o11y,
	)

	cfg := configs.TransactionsConfig{MonthlySummaryReconcilerCron: "@daily"}
	job := handlers.NewMonthlySummaryReconcilerJob(reconcileUc, cfg)

	s.Require().NoError(job.Run(ctx))

	corrected, err := summaryRepo.Get(ctx, userID, refMonth)
	s.Require().NoError(err)
	s.Equal(int64(0), corrected.IncomeCents(), "drift corrigido: sem transações reais, income deve ser 0")
}

type integrationRepositoryFactory struct {
	o11y observability.Observability
}

func (f *integrationRepositoryFactory) TransactionRepository(db database.DBTX) interfaces.TransactionRepository {
	return txpostgres.NewTransactionRepository(f.o11y, db)
}

func (f *integrationRepositoryFactory) CardInvoiceRepository(db database.DBTX) interfaces.CardInvoiceRepository {
	return txpostgres.NewCardInvoiceRepository(f.o11y, db)
}

func (f *integrationRepositoryFactory) RecurringTemplateRepository(db database.DBTX) interfaces.RecurringTemplateRepository {
	return txpostgres.NewRecurringTemplateRepository(f.o11y, db)
}

func (f *integrationRepositoryFactory) MonthlySummaryRepository(db database.DBTX) interfaces.MonthlySummaryRepository {
	return txpostgres.NewMonthlySummaryRepository(f.o11y, db)
}

func (f *integrationRepositoryFactory) RecurringMaterializationRepository(db database.DBTX) interfaces.RecurringMaterializationRepository {
	return txpostgres.NewRecurringMaterializationRepository(f.o11y, db)
}

type stubReconcileUseCase struct{}

func (u *stubReconcileUseCase) Execute(ctx context.Context) error { return nil }
