//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
	txpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/repositories/postgres"
)

type DayEntriesRepositorySuite struct {
	suite.Suite
}

func TestDayEntriesRepositorySuite(t *testing.T) {
	suite.Run(t, new(DayEntriesRepositorySuite))
}

func (s *DayEntriesRepositorySuite) newTx(userID uuid.UUID, desc string, amountCents int64, direction valueobjects.Direction, refMonth string, occurredAt time.Time) *entities.Transaction {
	amount, _ := valueobjects.NewMoney(amountCents)
	d, _ := valueobjects.NewDescription(desc)
	rm, _ := valueobjects.NewRefMonth(refMonth)
	evidence := expenseEvidence()
	catID := seedExpenseRootID
	catName, subName := "Custo Fixo", "Aluguel"
	if direction == valueobjects.DirectionIncome {
		evidence = incomeEvidence()
		catID = seedIncomeRootID
		catName, subName = "Salário", "Décimo Terceiro"
	}
	tx := entities.NewTransaction(
		uuid.New(),
		valueobjects.UserIDFromUUID(userID),
		direction,
		valueobjects.PaymentMethodPix,
		amount, d,
		valueobjects.CategoryIDFromUUID(catID),
		option.None[valueobjects.SubcategoryID](),
		catName, subName,
		evidence,
		rm, occurredAt, time.Now().UTC(),
	)
	return &tx
}

func (s *DayEntriesRepositorySuite) TestListDayEntries_ReturnsOnlyTargetDay() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	txRepo := txpostgres.NewTransactionRepository(noop.NewProvider(), db)
	repo := txpostgres.NewDayEntriesRepository(noop.NewProvider(), db)

	userID := uuid.New()
	day := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	otherDay := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

	s.Require().NoError(createTx(txRepo, ctx, s.newTx(userID, "Farmácia", 4500, valueobjects.DirectionOutcome, "2026-07", day)))
	s.Require().NoError(createTx(txRepo, ctx, s.newTx(userID, "Freelance", 50000, valueobjects.DirectionIncome, "2026-07", day)))
	s.Require().NoError(createTx(txRepo, ctx, s.newTx(userID, "Uber ontem", 3000, valueobjects.DirectionOutcome, "2026-07", otherDay)))

	entries, err := repo.ListDayEntries(ctx, userID, "2026-07-28")
	s.Require().NoError(err)
	s.Require().Len(entries, 2)
	s.Equal("Farmácia", entries[0].Description)
	s.Equal("outcome", entries[0].Direction)
	s.Equal("Freelance", entries[1].Description)
	s.Equal("income", entries[1].Direction)
}

func (s *DayEntriesRepositorySuite) TestListDayEntries_EmptyDay() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	repo := txpostgres.NewDayEntriesRepository(noop.NewProvider(), db)

	entries, err := repo.ListDayEntries(ctx, uuid.New(), "2026-07-28")
	s.Require().NoError(err)
	s.Empty(entries)
}

func (s *DayEntriesRepositorySuite) TestListDayEntries_InvalidDay() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	repo := txpostgres.NewDayEntriesRepository(noop.NewProvider(), db)

	_, err := repo.ListDayEntries(ctx, uuid.New(), "28/07/2026")
	s.Require().Error(err)
}
