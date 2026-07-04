//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
	txpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/repositories/postgres"
)

type OriginIdempotencySuite struct {
	suite.Suite
	db     database.DBTX
	txRepo interfaces.TransactionRepository
}

func TestOriginIdempotencySuite(t *testing.T) {
	suite.Run(t, new(OriginIdempotencySuite))
}

func (s *OriginIdempotencySuite) SetupSuite() {
	s.db, _ = testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	s.txRepo = txpostgres.NewTransactionRepository(o11y, s.db)
}

func (s *OriginIdempotencySuite) prepareCard(userID, cardID uuid.UUID) {
	ctx := context.Background()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		 VALUES ($1, $2, 'ACTIVE', now(), now()) ON CONFLICT DO NOTHING`,
		userID, "+5511"+userID.String()[:10],
	)
	s.Require().NoError(err)
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO mecontrola.cards (id, user_id, nickname, bank, closing_day, due_day, created_at, updated_at)
		 VALUES ($1, $2, 'test', 'nubank', 10, 20, now(), now()) ON CONFLICT DO NOTHING`,
		cardID, userID,
	)
	s.Require().NoError(err)
}

func (s *OriginIdempotencySuite) newTransactionWithOrigin(userID uuid.UUID, wamid string) *entities.Transaction {
	amount, _ := valueobjects.NewMoney(5000)
	desc, _ := valueobjects.NewDescription("Supermercado")
	catID := valueobjects.CategoryIDFromUUID(uuid.New())
	rm, _ := valueobjects.NewRefMonth("2026-06")
	now := time.Now().UTC()
	tx := entities.NewTransaction(
		uuid.New(),
		valueobjects.UserIDFromUUID(userID),
		valueobjects.DirectionOutcome, valueobjects.PaymentMethodPix,
		amount, desc, catID,
		option.None[valueobjects.SubcategoryID](),
		"Custo Fixo", "",
		rm, now, now,
	)
	if wamid != "" {
		tx.SetOrigin(wamid, 0, "create_transaction")
	}
	return &tx
}

func (s *OriginIdempotencySuite) newCreditTransactionWithOrigin(userID, cardID uuid.UUID, wamid string) *entities.Transaction {
	amount, _ := valueobjects.NewMoney(9000)
	desc, _ := valueobjects.NewDescription("Notebook")
	catID := valueobjects.CategoryIDFromUUID(uuid.New())
	rm, _ := valueobjects.NewRefMonth("2026-06")
	now := time.Now().UTC()
	tx := entities.NewTransaction(
		uuid.New(),
		valueobjects.UserIDFromUUID(userID),
		valueobjects.DirectionOutcome, valueobjects.PaymentMethodCreditCard,
		amount, desc, catID,
		option.None[valueobjects.SubcategoryID](),
		"Eletrônicos", "",
		rm, now, now,
	)
	inst, _ := valueobjects.NewInstallmentCount(3)
	snap, _ := valueobjects.NewCardBillingSnapshot(10, 20)
	tx.SetCardBilling(valueobjects.CardIDFromUUID(cardID), inst, snap)
	if wamid != "" {
		tx.SetOrigin(wamid, 0, "create_transaction")
	}
	return &tx
}

func (s *OriginIdempotencySuite) countTransactions(userID uuid.UUID) int {
	var n int
	s.Require().NoError(s.db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM mecontrola.transactions WHERE user_id = $1`, userID).Scan(&n))
	return n
}

func (s *OriginIdempotencySuite) TestOriginRef_CrashRedelivery_NoDuplicate() {
	ctx := context.Background()
	userID := uuid.New()
	wamid := "wamid." + uuid.New().String()

	first := s.newTransactionWithOrigin(userID, wamid)
	id1, created1, err1 := s.txRepo.Create(ctx, first)
	s.Require().NoError(err1)
	s.True(created1, "primeira criação deve inserir a linha")
	s.NotEqual(uuid.Nil, id1)

	second := s.newTransactionWithOrigin(userID, wamid)
	id2, created2, err2 := s.txRepo.Create(ctx, second)
	s.Require().NoError(err2)
	s.False(created2, "reentrega após crash não deve inserir segunda linha")
	s.Equal(id1, id2, "replay deve devolver o mesmo id canônico")

	s.Equal(1, s.countTransactions(userID), "deve existir exatamente 1 lançamento")
}

func (s *OriginIdempotencySuite) TestOriginRef_CreditTransaction_Idempotent() {
	ctx := context.Background()
	userID := uuid.New()
	cardID := uuid.New()
	s.prepareCard(userID, cardID)
	wamid := "wamid." + uuid.New().String()

	first := s.newCreditTransactionWithOrigin(userID, cardID, wamid)
	id1, created1, err1 := s.txRepo.Create(ctx, first)
	s.Require().NoError(err1)
	s.True(created1)

	second := s.newCreditTransactionWithOrigin(userID, cardID, wamid)
	id2, created2, err2 := s.txRepo.Create(ctx, second)
	s.Require().NoError(err2)
	s.False(created2)
	s.Equal(id1, id2)

	s.Equal(1, s.countTransactions(userID), "deve existir exatamente 1 lançamento de crédito")
}

func (s *OriginIdempotencySuite) TestOriginRef_WithoutOrigin_DoesNotConflict() {
	ctx := context.Background()
	userID := uuid.New()

	first := s.newTransactionWithOrigin(userID, "")
	_, created1, err1 := s.txRepo.Create(ctx, first)
	s.Require().NoError(err1)
	s.True(created1)

	second := s.newTransactionWithOrigin(userID, "")
	_, created2, err2 := s.txRepo.Create(ctx, second)
	s.Require().NoError(err2)
	s.True(created2, "sem origin o índice parcial não se aplica; ambas as linhas são criadas")

	s.Equal(2, s.countTransactions(userID), "comportamento HTTP preservado: 2 linhas")
}
