//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/repositories/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type ThresholdAlertSentRepositorySuite struct {
	suite.Suite
}

func TestThresholdAlertSentRepositorySuite(t *testing.T) {
	suite.Run(t, new(ThresholdAlertSentRepositorySuite))
}

func (s *ThresholdAlertSentRepositorySuite) TestInsertSentDedupAndListSentForDay() {
	mgr, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	userID := uuid.New()
	number := "+5511" + uuid.New().String()[:9]
	_, err := db.ExecContext(ctx,
		`INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		 VALUES ($1, $2, 'ACTIVE', now(), now())`,
		userID, number,
	)
	s.Require().NoError(err)

	repo := postgres.NewThresholdAlertSentRepository(noop.NewProvider(), db)
	budgetID := uuid.New()
	refDay := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)

	rec := interfaces.ThresholdAlertSentRecord{
		UserID:   userID,
		BudgetID: budgetID,
		Kind:     services.ThresholdAlertCategory,
		RefDay:   refDay,
		SentAt:   time.Now().UTC(),
	}

	s.Require().NoError(repo.InsertSent(ctx, rec))
	s.Require().NoError(repo.InsertSent(ctx, rec), "InsertSent deve ser idempotente via ON CONFLICT DO NOTHING")

	rec2 := rec
	rec2.Kind = services.ThresholdAlertGoal
	s.Require().NoError(repo.InsertSent(ctx, rec2))

	rec3 := rec
	rec3.RefDay = refDay.AddDate(0, 0, -1)
	s.Require().NoError(repo.InsertSent(ctx, rec3))

	listed, err := repo.ListSentForDay(ctx, refDay)
	s.Require().NoError(err)
	s.Require().Len(listed, 2, "deve listar apenas registros do refDay informado")

	kinds := map[services.ThresholdAlertKind]int{}
	for _, r := range listed {
		s.Equal(userID, r.UserID)
		s.Equal(budgetID, r.BudgetID)
		kinds[r.Kind]++
	}
	s.Equal(1, kinds[services.ThresholdAlertCategory])
	s.Equal(1, kinds[services.ThresholdAlertGoal])

	prev, err := repo.ListSentForDay(ctx, refDay.AddDate(0, 0, -1))
	s.Require().NoError(err)
	s.Require().Len(prev, 1)
	s.Equal(refDay.AddDate(0, 0, -1).Format(time.DateOnly), prev[0].RefDay.UTC().Format(time.DateOnly))
}
