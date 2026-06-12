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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
	txpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/repositories/postgres"
)

type CardPurchaseRepositoryIntegrationSuite struct {
	suite.Suite
	repo        interfaces.CardPurchaseRepository
	invoiceRepo interfaces.CardInvoiceRepository
}

func TestCardPurchaseRepositoryIntegrationSuite(t *testing.T) {
	suite.Run(t, new(CardPurchaseRepositoryIntegrationSuite))
}

func (s *CardPurchaseRepositoryIntegrationSuite) SetupSuite() {
	mgr, _ := testcontainer.Postgres(s.T())
	db := mgr.DBTX(context.Background())
	o11y := noop.NewProvider()
	s.repo = txpostgres.NewCardPurchaseRepository(o11y, db)
	s.invoiceRepo = txpostgres.NewCardInvoiceRepository(o11y, db)
}

func (s *CardPurchaseRepositoryIntegrationSuite) newPurchase(userID, cardID uuid.UUID, totalCents int64, installments int) entities.CardPurchase {
	snap, _ := valueobjects.NewCardBillingSnapshot(10, 20)
	amt, _ := valueobjects.NewMoney(totalCents)
	inst, _ := valueobjects.NewInstallmentCount(installments)
	desc, _ := valueobjects.NewDescription("Integration test purchase")
	catVo, _ := valueobjects.ParseCategoryID(uuid.New().String())
	return entities.NewCardPurchase(
		uuid.New(),
		valueobjects.UserIDFromUUID(userID),
		valueobjects.CardIDFromUUID(cardID),
		amt, inst, desc, catVo,
		option.None[valueobjects.SubcategoryID](),
		"Eletrônicos", "",
		time.Now(), snap, time.Now(),
	)
}

func (s *CardPurchaseRepositoryIntegrationSuite) TestCreate_GetByID_RoundTrip() {
	userID := uuid.New()
	cardID := uuid.New()
	p := s.newPurchase(userID, cardID, 5000, 5)

	s.Require().NoError(s.repo.Create(context.Background(), &p))

	got, err := s.repo.GetByID(context.Background(), p.ID(), userID)
	s.Require().NoError(err)
	s.Equal(p.ID(), got.ID())
	s.Equal(p.TotalAmount().Cents(), got.TotalAmount().Cents())
	s.Equal(int64(1), got.Version())
}

func (s *CardPurchaseRepositoryIntegrationSuite) TestSoftDelete_VersionConflict() {
	userID := uuid.New()
	p := s.newPurchase(userID, uuid.New(), 1000, 1)
	s.Require().NoError(s.repo.Create(context.Background(), &p))

	err := s.repo.SoftDelete(context.Background(), p.ID(), userID, 999, time.Now())
	s.Require().Error(err)
}

func (s *CardPurchaseRepositoryIntegrationSuite) TestSoftDelete_Success() {
	userID := uuid.New()
	p := s.newPurchase(userID, uuid.New(), 1000, 1)
	s.Require().NoError(s.repo.Create(context.Background(), &p))

	err := s.repo.SoftDelete(context.Background(), p.ID(), userID, 1, time.Now())
	s.Require().NoError(err)

	_, getErr := s.repo.GetByID(context.Background(), p.ID(), userID)
	s.Error(getErr)
}

func (s *CardPurchaseRepositoryIntegrationSuite) TestReplaceItems_AtomicUpsert() {
	userID := uuid.New()
	cardID := uuid.New()
	p := s.newPurchase(userID, cardID, 2000, 2)
	s.Require().NoError(s.repo.Create(context.Background(), &p))

	rm1, _ := valueobjects.NewRefMonth("2024-01")
	rm2, _ := valueobjects.NewRefMonth("2024-02")
	inv1, err1 := s.invoiceRepo.UpsertByMonth(context.Background(), userID, cardID, rm1, time.Now(), time.Now())
	inv2, err2 := s.invoiceRepo.UpsertByMonth(context.Background(), userID, cardID, rm2, time.Now(), time.Now())
	s.Require().NoError(err1)
	s.Require().NoError(err2)

	amt, _ := valueobjects.NewMoney(1000)
	item1 := entities.NewCardInvoiceItem(uuid.New(), inv1.ID(), p.ID(), valueobjects.UserIDFromUUID(userID), rm1, 1, amt, time.Now())
	item2 := entities.NewCardInvoiceItem(uuid.New(), inv2.ID(), p.ID(), valueobjects.UserIDFromUUID(userID), rm2, 2, amt, time.Now())

	s.Require().NoError(s.repo.ReplaceItems(context.Background(), p.ID(), []*entities.CardInvoiceItem{&item1, &item2}))

	s.Require().NoError(s.repo.ReplaceItems(context.Background(), p.ID(), nil))
}
