//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
	txpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/repositories/postgres"
)

type TransactionCardItemsSuite struct {
	suite.Suite
	ctx         context.Context
	db          *sqlx.DB
	txRepo      interfaces.TransactionRepository
	invoiceRepo interfaces.CardInvoiceRepository
}

func TestTransactionCardItemsSuite(t *testing.T) {
	suite.Run(t, new(TransactionCardItemsSuite))
}

func (s *TransactionCardItemsSuite) SetupSuite() {
	s.ctx = context.Background()
	db, _ := testcontainer.Postgres(s.T())
	s.db = db
	s.txRepo = txpostgres.NewTransactionRepository(noop.NewProvider(), db)
	s.invoiceRepo = txpostgres.NewCardInvoiceRepository(noop.NewProvider(), db)
}

func (s *TransactionCardItemsSuite) seedUserAndCard() (uuid.UUID, uuid.UUID) {
	userID := uuid.New()
	cardID := uuid.New()
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, $2, 'ACTIVE', now(), now())
	`, userID, "+55119"+userID.String()[0:8])
	s.Require().NoError(err)
	_, err = s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.cards (id, user_id, bank, nickname, closing_day, due_day)
		VALUES ($1, $2, 'Nubank', $3, 10, 20)
	`, cardID, userID, "card-"+cardID.String()[0:8])
	s.Require().NoError(err)
	return userID, cardID
}

func (s *TransactionCardItemsSuite) newCreditTransaction(userID, cardID uuid.UUID, installments int) *entities.Transaction {
	amount, _ := valueobjects.NewMoney(int64(installments) * 1000)
	desc, _ := valueobjects.NewDescription("Compra parcelada")
	rm, _ := valueobjects.NewRefMonth("2026-06")
	now := time.Now().UTC()

	tx := entities.NewTransaction(
		uuid.New(),
		valueobjects.UserIDFromUUID(userID),
		valueobjects.DirectionOutcome,
		valueobjects.PaymentMethodCreditCard,
		amount, desc,
		valueobjects.CategoryIDFromUUID(seedExpenseRootID),
		option.None[valueobjects.SubcategoryID](),
		"Custo Fixo", "Aluguel",
		expenseEvidence(),
		rm, now, now,
	)
	inst, _ := valueobjects.NewInstallmentCount(installments)
	snap, _ := valueobjects.NewCardBillingSnapshot(10, 20)
	tx.SetCardBilling(valueobjects.CardIDFromUUID(cardID), inst, snap)
	return &tx
}

func (s *TransactionCardItemsSuite) TestReplaceItemsAndGetItemsByTransactionID() {
	userID, cardID := s.seedUserAndCard()
	tx := s.newCreditTransaction(userID, cardID, 3)

	txID, created, err := s.txRepo.Create(s.ctx, tx)
	s.Require().NoError(err)
	s.True(created)

	now := time.Now().UTC()
	items := make([]*entities.CardInvoiceItem, 0, 3)
	for i := 0; i < 3; i++ {
		rmStr := time.Date(2026, time.Month(6+i), 1, 0, 0, 0, 0, time.UTC).Format("2006-01")
		rm, rmErr := valueobjects.NewRefMonth(rmStr)
		s.Require().NoError(rmErr)

		inv, upsertErr := s.invoiceRepo.UpsertByMonth(s.ctx, userID, cardID, rm, now, now.AddDate(0, 0, 10))
		s.Require().NoError(upsertErr)

		amount, _ := valueobjects.NewMoney(1000)
		item := entities.NewCardInvoiceItem(
			uuid.New(), inv.ID(), txID,
			valueobjects.UserIDFromUUID(userID),
			rm, i+1, amount, now,
		)
		items = append(items, &item)

		s.Require().NoError(s.invoiceRepo.ApplyDelta(s.ctx, inv.ID(), 1000, inv.Version()))
	}

	s.Require().NoError(s.txRepo.ReplaceItems(s.ctx, txID, items))

	got, getErr := s.txRepo.GetItemsByTransactionID(s.ctx, txID)
	s.Require().NoError(getErr)
	s.Require().Len(got, 3)
	for idx, it := range got {
		s.Equal(idx+1, it.InstallmentIndex())
		s.Equal(int64(1000), it.Amount().Cents())
	}
}

func (s *TransactionCardItemsSuite) TestReplaceItemsUpsertsOnConflict() {
	userID, cardID := s.seedUserAndCard()
	tx := s.newCreditTransaction(userID, cardID, 1)
	txID, _, err := s.txRepo.Create(s.ctx, tx)
	s.Require().NoError(err)

	now := time.Now().UTC()
	rm, _ := valueobjects.NewRefMonth("2026-06")
	inv, upsertErr := s.invoiceRepo.UpsertByMonth(s.ctx, userID, cardID, rm, now, now)
	s.Require().NoError(upsertErr)

	amount1, _ := valueobjects.NewMoney(1000)
	item1 := entities.NewCardInvoiceItem(uuid.New(), inv.ID(), txID, valueobjects.UserIDFromUUID(userID), rm, 1, amount1, now)
	s.Require().NoError(s.txRepo.ReplaceItems(s.ctx, txID, []*entities.CardInvoiceItem{&item1}))

	amount2, _ := valueobjects.NewMoney(2500)
	item2 := entities.NewCardInvoiceItem(uuid.New(), inv.ID(), txID, valueobjects.UserIDFromUUID(userID), rm, 1, amount2, now)
	s.Require().NoError(s.txRepo.ReplaceItems(s.ctx, txID, []*entities.CardInvoiceItem{&item2}))

	got, getErr := s.txRepo.GetItemsByTransactionID(s.ctx, txID)
	s.Require().NoError(getErr)
	s.Require().Len(got, 1)
	s.Equal(int64(2500), got[0].Amount().Cents())
}

func (s *TransactionCardItemsSuite) TestExistsActiveCreditByCard() {
	userID, cardID := s.seedUserAndCard()

	exists, err := s.txRepo.ExistsActiveCreditByCard(s.ctx, cardID, userID)
	s.Require().NoError(err)
	s.False(exists)

	tx := s.newCreditTransaction(userID, cardID, 1)
	_, _, createErr := s.txRepo.Create(s.ctx, tx)
	s.Require().NoError(createErr)

	exists, err = s.txRepo.ExistsActiveCreditByCard(s.ctx, cardID, userID)
	s.Require().NoError(err)
	s.True(exists)

	otherCard := uuid.New()
	existsOther, otherErr := s.txRepo.ExistsActiveCreditByCard(s.ctx, otherCard, userID)
	s.Require().NoError(otherErr)
	s.False(existsOther)
}

func (s *TransactionCardItemsSuite) TestSumByMonthExcludesCredit() {
	userID, cardID := s.seedUserAndCard()
	rm, _ := valueobjects.NewRefMonth("2026-06")

	amount, _ := valueobjects.NewMoney(5000)
	desc, _ := valueobjects.NewDescription("Mercado pix")
	now := time.Now().UTC()
	pixTx := entities.NewTransaction(
		uuid.New(), valueobjects.UserIDFromUUID(userID),
		valueobjects.DirectionOutcome, valueobjects.PaymentMethodPix,
		amount, desc,
		valueobjects.CategoryIDFromUUID(seedExpenseRootID),
		option.None[valueobjects.SubcategoryID](),
		"Custo Fixo", "Aluguel",
		expenseEvidence(),
		rm, now, now,
	)
	_, _, pixErr := s.txRepo.Create(s.ctx, &pixTx)
	s.Require().NoError(pixErr)

	creditTx := s.newCreditTransaction(userID, cardID, 2)
	_, _, creditErr := s.txRepo.Create(s.ctx, creditTx)
	s.Require().NoError(creditErr)

	income, outcome, sumErr := s.txRepo.SumByMonthExcludingCredit(s.ctx, userID, rm)
	s.Require().NoError(sumErr)
	s.Equal(int64(0), income)
	s.Equal(int64(5000), outcome)
}
