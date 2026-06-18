//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
	txpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/repositories/postgres"
)

type TransactionRepositorySuite struct {
	suite.Suite
}

func TestTransactionRepositorySuite(t *testing.T) {
	suite.Run(t, new(TransactionRepositorySuite))
}

func (s *TransactionRepositorySuite) newTransaction(userID uuid.UUID) *entities.Transaction {
	dir := valueobjects.DirectionOutcome
	pm := valueobjects.PaymentMethodPix
	amount, _ := valueobjects.NewMoney(5000)
	desc, _ := valueobjects.NewDescription("Supermercado")
	catID := valueobjects.CategoryIDFromUUID(uuid.New())
	rm, _ := valueobjects.NewRefMonth("2026-06")
	now := time.Now().UTC()

	tx := entities.NewTransaction(
		uuid.New(),
		valueobjects.UserIDFromUUID(userID),
		dir, pm, amount, desc, catID,
		option.None[valueobjects.SubcategoryID](),
		"Custo Fixo", "",
		rm, now, now,
	)
	return &tx
}

func (s *TransactionRepositorySuite) TestCreateAndGetByID() {
	db, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	ctx := context.Background()
	repo := txpostgres.NewTransactionRepository(o11y, db)

	userID := uuid.New()
	tx := s.newTransaction(userID)

	s.Require().NoError(repo.Create(ctx, tx))

	found, err := repo.GetByID(ctx, tx.ID(), userID)
	s.Require().NoError(err)
	s.Equal(tx.ID(), found.ID())
	s.Equal(int64(5000), found.Amount().Cents())
	s.Equal("2026-06", found.RefMonth().String())
}

func (s *TransactionRepositorySuite) TestGetByID_NotFound_OtherUser() {
	db, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	ctx := context.Background()
	repo := txpostgres.NewTransactionRepository(o11y, db)

	_, err := repo.GetByID(ctx, uuid.New(), uuid.New())
	s.Require().Error(err)
	s.True(errors.Is(err, interfaces.ErrTransactionNotFound))
}

func (s *TransactionRepositorySuite) TestUpdateWithVersion_Success() {
	db, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	ctx := context.Background()
	repo := txpostgres.NewTransactionRepository(o11y, db)

	userID := uuid.New()
	tx := s.newTransaction(userID)
	s.Require().NoError(repo.Create(ctx, tx))

	amount2, _ := valueobjects.NewMoney(9999)
	desc2, _ := valueobjects.NewDescription("Farmácia")
	rm, _ := valueobjects.NewRefMonth("2026-06")
	now := time.Now().UTC()
	tx.Update(
		tx.Direction(), tx.PaymentMethod(), amount2, desc2,
		tx.CategoryID(), tx.SubcategoryID(),
		"Custo Fixo", "",
		rm, now, now,
	)

	s.Require().NoError(repo.UpdateWithVersion(ctx, tx, 1))

	found, err := repo.GetByID(ctx, tx.ID(), userID)
	s.Require().NoError(err)
	s.Equal(int64(9999), found.Amount().Cents())
	s.Equal(int64(2), found.Version())
}

func (s *TransactionRepositorySuite) TestUpdateWithVersion_VersionConflict() {
	db, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	ctx := context.Background()
	repo := txpostgres.NewTransactionRepository(o11y, db)

	userID := uuid.New()
	tx := s.newTransaction(userID)
	s.Require().NoError(repo.Create(ctx, tx))

	tx.Update(tx.Direction(), tx.PaymentMethod(), tx.Amount(), tx.Description(),
		tx.CategoryID(), tx.SubcategoryID(), "", "",
		tx.RefMonth(), tx.OccurredAt(), time.Now().UTC())

	err := repo.UpdateWithVersion(ctx, tx, 999)
	s.Require().Error(err)
	s.True(errors.Is(err, interfaces.ErrTransactionVersionConflict))
}

func (s *TransactionRepositorySuite) TestSoftDelete() {
	db, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	ctx := context.Background()
	repo := txpostgres.NewTransactionRepository(o11y, db)

	userID := uuid.New()
	tx := s.newTransaction(userID)
	s.Require().NoError(repo.Create(ctx, tx))

	s.Require().NoError(repo.SoftDelete(ctx, tx.ID(), userID, 1, time.Now().UTC()))

	_, err := repo.GetByID(ctx, tx.ID(), userID)
	s.Require().Error(err)
	s.True(errors.Is(err, interfaces.ErrTransactionNotFound))
}

func (s *TransactionRepositorySuite) TestListByMonth_CursorRoundtrip() {
	db, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	ctx := context.Background()
	repo := txpostgres.NewTransactionRepository(o11y, db)

	userID := uuid.New()
	rm, _ := valueobjects.NewRefMonth("2026-06")

	for i := 0; i < 5; i++ {
		tx := s.newTransaction(userID)
		s.Require().NoError(repo.Create(ctx, tx))
		time.Sleep(2 * time.Millisecond)
	}

	page1, cursor1, err := repo.ListByMonth(ctx, userID, rm, interfaces.Cursor{}, 3)
	s.Require().NoError(err)
	s.Len(page1, 3)
	s.NotEmpty(cursor1.Value)

	page2, cursor2, err := repo.ListByMonth(ctx, userID, rm, cursor1, 3)
	s.Require().NoError(err)
	s.Len(page2, 2)
	s.Empty(cursor2.Value)

	allIDs := make(map[uuid.UUID]bool)
	for _, tx := range page1 {
		allIDs[tx.ID()] = true
	}
	for _, tx := range page2 {
		s.False(allIDs[tx.ID()], "cursor deve ser disjunto")
	}
}

func (s *TransactionRepositorySuite) TestSumByMonth() {
	db, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	ctx := context.Background()
	repo := txpostgres.NewTransactionRepository(o11y, db)

	userID := uuid.New()
	rm, _ := valueobjects.NewRefMonth("2026-06")
	now := time.Now().UTC()

	txOut := entities.NewTransaction(
		uuid.New(), valueobjects.UserIDFromUUID(userID),
		valueobjects.DirectionOutcome, valueobjects.PaymentMethodPix,
		mustMoney(s.T(), 3000), mustDesc(s.T(), "Saída"),
		valueobjects.CategoryIDFromUUID(uuid.New()),
		option.None[valueobjects.SubcategoryID](),
		"Cat", "", rm, now, now,
	)
	txIn := entities.NewTransaction(
		uuid.New(), valueobjects.UserIDFromUUID(userID),
		valueobjects.DirectionIncome, valueobjects.PaymentMethodTED,
		mustMoney(s.T(), 10000), mustDesc(s.T(), "Salário"),
		valueobjects.CategoryIDFromUUID(uuid.New()),
		option.None[valueobjects.SubcategoryID](),
		"Renda", "", rm, now, now,
	)
	s.Require().NoError(repo.Create(ctx, &txOut))
	s.Require().NoError(repo.Create(ctx, &txIn))

	income, outcome, err := repo.SumByMonth(ctx, userID, rm)
	s.Require().NoError(err)
	s.Equal(int64(10000), income)
	s.Equal(int64(3000), outcome)
}

func (s *TransactionRepositorySuite) TestSoftDelete_NotInList() {
	db, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	ctx := context.Background()
	repo := txpostgres.NewTransactionRepository(o11y, db)

	userID := uuid.New()
	rm, _ := valueobjects.NewRefMonth("2026-06")

	active := s.newTransaction(userID)
	s.Require().NoError(repo.Create(ctx, active))

	deleted := s.newTransaction(userID)
	s.Require().NoError(repo.Create(ctx, deleted))
	s.Require().NoError(repo.SoftDelete(ctx, deleted.ID(), userID, 1, time.Now().UTC()))

	list, _, err := repo.ListByMonth(ctx, userID, rm, interfaces.Cursor{}, 50)
	s.Require().NoError(err)

	for _, tx := range list {
		s.NotEqual(deleted.ID(), tx.ID(), "lançamento deletado não deve aparecer na listagem")
	}
	found := false
	for _, tx := range list {
		if tx.ID() == active.ID() {
			found = true
			break
		}
	}
	s.True(found, "lançamento ativo deve aparecer na listagem")
}

func mustMoney(t *testing.T, cents int64) valueobjects.Money {
	t.Helper()
	m, err := valueobjects.NewMoney(cents)
	if err != nil {
		t.Fatalf("mustMoney: %v", err)
	}
	return m
}

func mustDesc(t *testing.T, s string) valueobjects.Description {
	t.Helper()
	d, err := valueobjects.NewDescription(s)
	if err != nil {
		t.Fatalf("mustDesc: %v", err)
	}
	return d
}
