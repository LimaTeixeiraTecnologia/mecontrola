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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
	txpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/repositories/postgres"
)

type MonthlySummaryRepositorySuite struct {
	suite.Suite
	repo interfaces.MonthlySummaryRepository
}

func TestMonthlySummaryRepositorySuite(t *testing.T) {
	suite.Run(t, new(MonthlySummaryRepositorySuite))
}

func (s *MonthlySummaryRepositorySuite) SetupSuite() {
	mgr, _ := testcontainer.Postgres(s.T())
	db := mgr.DBTX(context.Background())
	s.repo = txpostgres.NewMonthlySummaryRepository(noop.NewProvider(), db)
}

func (s *MonthlySummaryRepositorySuite) TestUpsert_Idempotent() {
	userID := uuid.New()
	rm, _ := valueobjects.NewRefMonth("2026-06")
	ctx := context.Background()
	now := time.Now().UTC()

	s.Require().NoError(s.repo.Upsert(ctx, userID, rm, 10000, 5000, now))

	got, err := s.repo.Get(ctx, userID, rm)
	s.Require().NoError(err)
	s.Require().NotNil(got)
	s.Equal(int64(10000), got.IncomeCents())
	s.Equal(int64(5000), got.OutcomeCents())
	s.Equal(int64(5000), got.TotalCents())
	s.Equal(int64(1), got.Version())

	s.Require().NoError(s.repo.Upsert(ctx, userID, rm, 20000, 8000, now))
	got2, err := s.repo.Get(ctx, userID, rm)
	s.Require().NoError(err)
	s.Equal(int64(20000), got2.IncomeCents())
	s.Equal(int64(8000), got2.OutcomeCents())
	s.Equal(int64(12000), got2.TotalCents())
	s.Equal(int64(2), got2.Version())
}

func (s *MonthlySummaryRepositorySuite) TestGet_ReturnsNilWhenNotFound() {
	userID := uuid.New()
	rm, _ := valueobjects.NewRefMonth("2025-01")
	ctx := context.Background()

	got, err := s.repo.Get(ctx, userID, rm)
	s.Require().NoError(err)
	s.Nil(got)
}

func (s *MonthlySummaryRepositorySuite) TestListActiveSince() {
	ctx := context.Background()
	userID := uuid.New()
	rm1, _ := valueobjects.NewRefMonth("2026-04")
	rm2, _ := valueobjects.NewRefMonth("2026-05")
	now := time.Now().UTC()

	s.Require().NoError(s.repo.Upsert(ctx, userID, rm1, 1000, 500, now))
	s.Require().NoError(s.repo.Upsert(ctx, userID, rm2, 2000, 1000, now))

	keys, cursor, err := s.repo.ListActiveSince(ctx, now.Add(-time.Minute), interfaces.Cursor{}, 100)
	s.Require().NoError(err)
	s.Empty(cursor.Value)

	found1, found2 := false, false
	for _, k := range keys {
		if k.UserID == userID && k.RefMonth == "2026-04" {
			found1 = true
		}
		if k.UserID == userID && k.RefMonth == "2026-05" {
			found2 = true
		}
	}
	s.True(found1)
	s.True(found2)
}

func (s *MonthlySummaryRepositorySuite) TestListActiveSince_Cursor() {
	ctx := context.Background()
	userID := uuid.New()
	now := time.Now().UTC()

	months := []string{"2026-01", "2026-02", "2026-03", "2026-04", "2026-05"}
	for i, m := range months {
		rm, _ := valueobjects.NewRefMonth(m)
		s.Require().NoError(s.repo.Upsert(ctx, userID, rm, int64((i+1)*1000), int64((i+1)*500), now))
	}

	page1, cur1, err := s.repo.ListActiveSince(ctx, now.Add(-time.Minute), interfaces.Cursor{}, 3)
	s.Require().NoError(err)
	s.GreaterOrEqual(len(page1), 3)
	s.NotEmpty(cur1.Value)

	page2, cur2, err := s.repo.ListActiveSince(ctx, now.Add(-time.Minute), cur1, 3)
	s.Require().NoError(err)
	s.GreaterOrEqual(len(page2), 0)
	_ = cur2
}
