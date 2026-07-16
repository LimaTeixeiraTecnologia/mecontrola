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

type TransactionEditCandidatesSuite struct {
	suite.Suite
}

func TestTransactionEditCandidatesSuite(t *testing.T) {
	suite.Run(t, new(TransactionEditCandidatesSuite))
}

func (s *TransactionEditCandidatesSuite) newTransaction(userID uuid.UUID, desc string, amountCents int64, refMonth string) *entities.Transaction {
	amount, _ := valueobjects.NewMoney(amountCents)
	d, _ := valueobjects.NewDescription(desc)
	rm, _ := valueobjects.NewRefMonth(refMonth)
	now := time.Now().UTC()
	tx := entities.NewTransaction(
		uuid.New(),
		valueobjects.UserIDFromUUID(userID),
		valueobjects.DirectionOutcome,
		valueobjects.PaymentMethodPix,
		amount, d,
		valueobjects.CategoryIDFromUUID(seedExpenseRootID),
		option.None[valueobjects.SubcategoryID](),
		"Custo Fixo", "Aluguel",
		expenseEvidence(),
		rm, now, now,
	)
	return &tx
}

func (s *TransactionEditCandidatesSuite) TestSearchEditCandidates_ByAmountOnly() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	repo := txpostgres.NewTransactionRepository(noop.NewProvider(), db)

	userID := uuid.New()
	rm, _ := valueobjects.NewRefMonth("2026-06")
	s.Require().NoError(createTx(repo, ctx, s.newTransaction(userID, "Farmácia", 2500, "2026-06")))
	s.Require().NoError(createTx(repo, ctx, s.newTransaction(userID, "Mercado", 4200, "2026-06")))

	results, err := repo.SearchEditCandidates(ctx, userID, 2500, "", rm, 5)
	s.Require().NoError(err)
	s.Require().Len(results, 1)
	s.Equal("Farmácia", results[0].Description().String())
}

func (s *TransactionEditCandidatesSuite) TestSearchEditCandidates_ByTermOnly() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	repo := txpostgres.NewTransactionRepository(noop.NewProvider(), db)

	userID := uuid.New()
	rm, _ := valueobjects.NewRefMonth("2026-06")
	s.Require().NoError(createTx(repo, ctx, s.newTransaction(userID, "Mercado Extra", 4200, "2026-06")))
	s.Require().NoError(createTx(repo, ctx, s.newTransaction(userID, "Farmácia", 2500, "2026-06")))

	results, err := repo.SearchEditCandidates(ctx, userID, 0, "mercado", rm, 5)
	s.Require().NoError(err)
	s.Require().Len(results, 1)
	s.Equal("Mercado Extra", results[0].Description().String())
}

func (s *TransactionEditCandidatesSuite) TestSearchEditCandidates_ByAmountOrTerm() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	repo := txpostgres.NewTransactionRepository(noop.NewProvider(), db)

	userID := uuid.New()
	rm, _ := valueobjects.NewRefMonth("2026-06")
	s.Require().NoError(createTx(repo, ctx, s.newTransaction(userID, "Mercado Extra", 4200, "2026-06")))
	s.Require().NoError(createTx(repo, ctx, s.newTransaction(userID, "Farmácia", 2500, "2026-06")))
	s.Require().NoError(createTx(repo, ctx, s.newTransaction(userID, "Padaria", 900, "2026-06")))

	results, err := repo.SearchEditCandidates(ctx, userID, 2500, "mercado", rm, 5)
	s.Require().NoError(err)
	s.Require().Len(results, 2)
}

func (s *TransactionEditCandidatesSuite) TestSearchEditCandidates_RestrictedToRefMonth() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	repo := txpostgres.NewTransactionRepository(noop.NewProvider(), db)

	userID := uuid.New()
	rm, _ := valueobjects.NewRefMonth("2026-06")
	s.Require().NoError(createTx(repo, ctx, s.newTransaction(userID, "Farmácia", 2500, "2026-06")))
	s.Require().NoError(createTx(repo, ctx, s.newTransaction(userID, "Farmácia do mês passado", 2500, "2026-05")))

	results, err := repo.SearchEditCandidates(ctx, userID, 2500, "", rm, 5)
	s.Require().NoError(err)
	s.Require().Len(results, 1)
	s.Equal("Farmácia", results[0].Description().String())
}

func (s *TransactionEditCandidatesSuite) TestSearchEditCandidates_OrderedByRecencyAndLimited() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	repo := txpostgres.NewTransactionRepository(noop.NewProvider(), db)

	userID := uuid.New()
	rm, _ := valueobjects.NewRefMonth("2026-06")
	for i := 0; i < 8; i++ {
		s.Require().NoError(createTx(repo, ctx, s.newTransaction(userID, "Padaria do bairro", 900, "2026-06")))
		time.Sleep(time.Millisecond)
	}

	results, err := repo.SearchEditCandidates(ctx, userID, 0, "padaria", rm, 5)
	s.Require().NoError(err)
	s.Require().Len(results, 5)
	for i := 0; i < len(results)-1; i++ {
		s.False(results[i].CreatedAt().Before(results[i+1].CreatedAt()))
	}
}

func (s *TransactionEditCandidatesSuite) TestSearchEditCandidates_UserIsolation() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	repo := txpostgres.NewTransactionRepository(noop.NewProvider(), db)

	userA := uuid.New()
	userB := uuid.New()
	rm, _ := valueobjects.NewRefMonth("2026-06")
	s.Require().NoError(createTx(repo, ctx, s.newTransaction(userA, "Farmácia A", 2500, "2026-06")))
	s.Require().NoError(createTx(repo, ctx, s.newTransaction(userB, "Farmácia B", 2500, "2026-06")))

	results, err := repo.SearchEditCandidates(ctx, userA, 2500, "", rm, 5)
	s.Require().NoError(err)
	s.Require().Len(results, 1)
	s.Equal("Farmácia A", results[0].Description().String())
}

func (s *TransactionEditCandidatesSuite) TestSearchEditCandidates_ExcludesDeleted() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	repo := txpostgres.NewTransactionRepository(noop.NewProvider(), db)

	userID := uuid.New()
	rm, _ := valueobjects.NewRefMonth("2026-06")
	deleted := s.newTransaction(userID, "Farmácia cancelada", 2500, "2026-06")
	s.Require().NoError(createTx(repo, ctx, deleted))
	s.Require().NoError(repo.SoftDelete(ctx, deleted.ID(), userID, 1, time.Now().UTC()))

	results, err := repo.SearchEditCandidates(ctx, userID, 2500, "", rm, 5)
	s.Require().NoError(err)
	s.Empty(results)
}
