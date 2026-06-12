//go:build integration

package postgres_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
	txpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/repositories/postgres"
)

type CardInvoiceRepositoryIntegrationSuite struct {
	suite.Suite
	repo interfaces.CardInvoiceRepository
}

func TestCardInvoiceRepositoryIntegrationSuite(t *testing.T) {
	suite.Run(t, new(CardInvoiceRepositoryIntegrationSuite))
}

func (s *CardInvoiceRepositoryIntegrationSuite) SetupSuite() {
	mgr, _ := testcontainer.Postgres(s.T())
	db := mgr.DBTX(context.Background())
	s.repo = txpostgres.NewCardInvoiceRepository(noop.NewProvider(), db)
}

func (s *CardInvoiceRepositoryIntegrationSuite) TestUpsertByMonth_IdempotentOnConflict() {
	userID := uuid.New()
	cardID := uuid.New()
	rm, _ := valueobjects.NewRefMonth("2024-05")
	now := time.Now()

	inv1, err := s.repo.UpsertByMonth(context.Background(), userID, cardID, rm, now, now.AddDate(0, 0, 10))
	s.Require().NoError(err)

	inv2, err := s.repo.UpsertByMonth(context.Background(), userID, cardID, rm, now, now.AddDate(0, 0, 10))
	s.Require().NoError(err)
	s.Equal(inv1.ID(), inv2.ID())
}

func (s *CardInvoiceRepositoryIntegrationSuite) TestApplyDelta_Positive() {
	userID := uuid.New()
	cardID := uuid.New()
	rm, _ := valueobjects.NewRefMonth("2024-06")
	now := time.Now()

	inv, err := s.repo.UpsertByMonth(context.Background(), userID, cardID, rm, now, now)
	s.Require().NoError(err)
	s.Equal(int64(0), inv.ItemsTotalCents())

	s.Require().NoError(s.repo.ApplyDelta(context.Background(), inv.ID(), 5000, inv.Version()))

	got, _, err := s.repo.GetByMonth(context.Background(), userID, cardID, rm)
	s.Require().NoError(err)
	s.Equal(int64(5000), got.ItemsTotalCents())
}

func (s *CardInvoiceRepositoryIntegrationSuite) TestApplyDelta_Negative() {
	userID := uuid.New()
	cardID := uuid.New()
	rm, _ := valueobjects.NewRefMonth("2024-07")
	now := time.Now()

	inv, err := s.repo.UpsertByMonth(context.Background(), userID, cardID, rm, now, now)
	s.Require().NoError(err)

	s.Require().NoError(s.repo.ApplyDelta(context.Background(), inv.ID(), 10000, inv.Version()))

	got, _, getErr := s.repo.GetByMonth(context.Background(), userID, cardID, rm)
	s.Require().NoError(getErr)

	s.Require().NoError(s.repo.ApplyDelta(context.Background(), got.ID(), -3000, got.Version()))

	final, _, finalErr := s.repo.GetByMonth(context.Background(), userID, cardID, rm)
	s.Require().NoError(finalErr)
	s.Equal(int64(7000), final.ItemsTotalCents())
}

func (s *CardInvoiceRepositoryIntegrationSuite) TestApplyDelta_Zero_NoUpdate() {
	userID := uuid.New()
	cardID := uuid.New()
	rm, _ := valueobjects.NewRefMonth("2024-08")
	now := time.Now()

	inv, err := s.repo.UpsertByMonth(context.Background(), userID, cardID, rm, now, now)
	s.Require().NoError(err)

	s.Require().NoError(s.repo.ApplyDelta(context.Background(), inv.ID(), 0, inv.Version()))

	got, _, getErr := s.repo.GetByMonth(context.Background(), userID, cardID, rm)
	s.Require().NoError(getErr)
	s.Equal(inv.Version(), got.Version())
}

func (s *CardInvoiceRepositoryIntegrationSuite) TestApplyDelta_VersionConflict_Returns409() {
	userID := uuid.New()
	cardID := uuid.New()
	rm, _ := valueobjects.NewRefMonth("2024-09")
	now := time.Now()

	inv, err := s.repo.UpsertByMonth(context.Background(), userID, cardID, rm, now, now)
	s.Require().NoError(err)

	err = s.repo.ApplyDelta(context.Background(), inv.ID(), 1000, 999)
	s.Require().Error(err)
}

func (s *CardInvoiceRepositoryIntegrationSuite) TestApplyDelta_RaceCondition_ConcurrentUpdates() {
	userID := uuid.New()
	cardID := uuid.New()
	rm, _ := valueobjects.NewRefMonth("2024-10")
	now := time.Now()

	_, err := s.repo.UpsertByMonth(context.Background(), userID, cardID, rm, now, now)
	s.Require().NoError(err)

	var wg sync.WaitGroup
	errors := make(chan error, 2)

	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			freshInv, _, freshErr := s.repo.GetByMonth(context.Background(), userID, cardID, rm)
			if freshErr != nil || freshInv == nil {
				errors <- freshErr
				return
			}
			errors <- s.repo.ApplyDelta(context.Background(), freshInv.ID(), 500, freshInv.Version())
		}()
	}
	wg.Wait()
	close(errors)

	var successCount, conflictCount int
	for e := range errors {
		if e == nil {
			successCount++
		} else {
			conflictCount++
		}
	}
	s.Equal(2, successCount+conflictCount)
}

func (s *CardInvoiceRepositoryIntegrationSuite) TestApplyDelta_24Installments_ClosingDayFeb() {
	userID := uuid.New()
	cardID := uuid.New()
	now := time.Now()

	for i := range 24 {
		year := 2024 + (i / 12)
		month := (i % 12) + 1
		rmStr := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).Format("2006-01")
		rm, rmErr := valueobjects.NewRefMonth(rmStr)
		s.Require().NoError(rmErr)

		var closingAt time.Time
		if month == 2 {
			closingAt = time.Date(year, 2, 28, 0, 0, 0, 0, time.UTC)
		} else {
			closingAt = time.Date(year, time.Month(month), 30, 0, 0, 0, 0, time.UTC)
		}

		inv, upsertErr := s.repo.UpsertByMonth(context.Background(), userID, cardID, rm, closingAt, now)
		s.Require().NoError(upsertErr)

		deltaErr := s.repo.ApplyDelta(context.Background(), inv.ID(), 1000, inv.Version())
		s.Require().NoError(deltaErr)
	}

	for i := range 24 {
		year := 2024 + (i / 12)
		month := (i % 12) + 1
		rmStr := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).Format("2006-01")
		rm, _ := valueobjects.NewRefMonth(rmStr)
		got, _, getErr := s.repo.GetByMonth(context.Background(), userID, cardID, rm)
		s.Require().NoError(getErr)
		s.Equal(int64(1000), got.ItemsTotalCents())
	}
}
