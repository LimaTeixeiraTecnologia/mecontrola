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

type TransactionSearchSuite struct {
	suite.Suite
}

func TestTransactionSearchSuite(t *testing.T) {
	suite.Run(t, new(TransactionSearchSuite))
}

func (s *TransactionSearchSuite) newTransactionWithDescription(userID uuid.UUID, desc string) *entities.Transaction {
	amount, _ := valueobjects.NewMoney(3500)
	d, _ := valueobjects.NewDescription(desc)
	rm, _ := valueobjects.NewRefMonth("2026-06")
	now := time.Now().UTC()
	tx := entities.NewTransaction(
		uuid.New(),
		valueobjects.UserIDFromUUID(userID),
		valueobjects.DirectionOutcome,
		valueobjects.PaymentMethodPix,
		amount, d,
		valueobjects.CategoryIDFromUUID(uuid.New()),
		option.None[valueobjects.SubcategoryID](),
		"Custo Fixo", "",
		rm, now, now,
	)
	return &tx
}

func (s *TransactionSearchSuite) TestSearchByDescription_ILIKE() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	repo := txpostgres.NewTransactionRepository(noop.NewProvider(), db)

	userID := uuid.New()
	s.Require().NoError(repo.Create(ctx, s.newTransactionWithDescription(userID, "Uber para o trabalho")))
	s.Require().NoError(repo.Create(ctx, s.newTransactionWithDescription(userID, "Mercado Extra")))

	q, _ := valueobjects.NewSearchQuery("uber")
	results, err := repo.SearchByDescription(ctx, userID, q, option.None[valueobjects.RefMonth](), 10)
	s.Require().NoError(err)
	s.Len(results, 1)
	s.Equal("Uber para o trabalho", results[0].Description().String())
}

func (s *TransactionSearchSuite) TestSearchByDescription_MultipleMatches() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	repo := txpostgres.NewTransactionRepository(noop.NewProvider(), db)

	userID := uuid.New()
	s.Require().NoError(repo.Create(ctx, s.newTransactionWithDescription(userID, "Mercado")))
	s.Require().NoError(repo.Create(ctx, s.newTransactionWithDescription(userID, "Mercado Extra")))
	s.Require().NoError(repo.Create(ctx, s.newTransactionWithDescription(userID, "Mercadinho")))

	q, _ := valueobjects.NewSearchQuery("mercad")
	results, err := repo.SearchByDescription(ctx, userID, q, option.None[valueobjects.RefMonth](), 10)
	s.Require().NoError(err)
	s.Len(results, 3)
}

func (s *TransactionSearchSuite) TestSearchByDescription_LimitApplied() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	repo := txpostgres.NewTransactionRepository(noop.NewProvider(), db)

	userID := uuid.New()
	for i := 0; i < 12; i++ {
		s.Require().NoError(repo.Create(ctx, s.newTransactionWithDescription(userID, "Padaria do bairro")))
		time.Sleep(time.Millisecond)
	}

	q, _ := valueobjects.NewSearchQuery("padaria")
	results, err := repo.SearchByDescription(ctx, userID, q, option.None[valueobjects.RefMonth](), 5)
	s.Require().NoError(err)
	s.Len(results, 5)
}

func (s *TransactionSearchSuite) TestSearchByDescription_UserIsolation() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	repo := txpostgres.NewTransactionRepository(noop.NewProvider(), db)

	userA := uuid.New()
	userB := uuid.New()
	s.Require().NoError(repo.Create(ctx, s.newTransactionWithDescription(userA, "Uber A")))
	s.Require().NoError(repo.Create(ctx, s.newTransactionWithDescription(userB, "Uber B")))

	q, _ := valueobjects.NewSearchQuery("uber")
	results, err := repo.SearchByDescription(ctx, userA, q, option.None[valueobjects.RefMonth](), 10)
	s.Require().NoError(err)
	s.Len(results, 1)
	s.Equal("Uber A", results[0].Description().String())
}

func (s *TransactionSearchSuite) TestSearchByDescription_ExcludesDeleted() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	repo := txpostgres.NewTransactionRepository(noop.NewProvider(), db)

	userID := uuid.New()
	deleted := s.newTransactionWithDescription(userID, "Uber cancelado")
	s.Require().NoError(repo.Create(ctx, deleted))
	s.Require().NoError(repo.SoftDelete(ctx, deleted.ID(), userID, 1, time.Now().UTC()))

	q, _ := valueobjects.NewSearchQuery("uber")
	results, err := repo.SearchByDescription(ctx, userID, q, option.None[valueobjects.RefMonth](), 10)
	s.Require().NoError(err)
	s.Empty(results)
}
