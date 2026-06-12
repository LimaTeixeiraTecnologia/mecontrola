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

type RecurringTemplateRepositorySuite struct {
	suite.Suite
}

func TestRecurringTemplateRepositorySuite(t *testing.T) {
	suite.Run(t, new(RecurringTemplateRepositorySuite))
}

func (s *RecurringTemplateRepositorySuite) newTemplate(userID uuid.UUID, day int) *entities.RecurringTemplate {
	dir := valueobjects.DirectionIncome
	pm := valueobjects.PaymentMethodPix
	amount, _ := valueobjects.NewMoney(300000)
	desc, _ := valueobjects.NewDescription("Salário")
	catID := valueobjects.CategoryIDFromUUID(uuid.New())
	freq := valueobjects.FrequencyMonthly
	dom, _ := valueobjects.NewDayOfMonth(day)
	inst, _ := valueobjects.NewInstallmentCount(1)
	now := time.Now().UTC()

	t := entities.NewRecurringTemplate(
		uuid.New(),
		valueobjects.UserIDFromUUID(userID),
		dir, pm,
		option.None[valueobjects.CardID](),
		amount, desc, catID,
		option.None[valueobjects.SubcategoryID](),
		"Receita", "",
		freq, dom, inst,
		now, option.None[time.Time](), now,
	)
	return &t
}

func (s *RecurringTemplateRepositorySuite) TestCreateAndGetByID() {
	mgr, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	ctx := context.Background()
	db := mgr.DBTX(ctx)
	repo := txpostgres.NewRecurringTemplateRepository(o11y, db)

	userID := uuid.New()
	template := s.newTemplate(userID, 5)

	s.Require().NoError(repo.Create(ctx, template))

	found, err := repo.GetByID(ctx, template.ID(), userID)
	s.Require().NoError(err)
	s.Equal(template.ID(), found.ID())
	s.Equal(5, found.DayOfMonth().Value())
	s.Equal(int64(300000), found.Amount().Cents())
}

func (s *RecurringTemplateRepositorySuite) TestGetByID_NotFound() {
	mgr, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	ctx := context.Background()
	db := mgr.DBTX(ctx)
	repo := txpostgres.NewRecurringTemplateRepository(o11y, db)

	_, err := repo.GetByID(ctx, uuid.New(), uuid.New())
	s.Require().Error(err)
}

func (s *RecurringTemplateRepositorySuite) TestUpdateWithVersion_Success() {
	mgr, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	ctx := context.Background()
	db := mgr.DBTX(ctx)
	repo := txpostgres.NewRecurringTemplateRepository(o11y, db)

	userID := uuid.New()
	template := s.newTemplate(userID, 5)
	s.Require().NoError(repo.Create(ctx, template))

	amount2, _ := valueobjects.NewMoney(400000)
	desc2, _ := valueobjects.NewDescription("Bônus")
	now := time.Now().UTC()
	template.Update(
		template.Direction(), template.PaymentMethod(), template.CardID(),
		amount2, desc2, template.CategoryID(), template.SubcategoryID(),
		"Receita", "",
		template.Frequency(), template.DayOfMonth(), template.InstallmentsTotal(),
		template.StartedAt(), template.EndedAt(), now,
	)

	s.Require().NoError(repo.UpdateWithVersion(ctx, template, 1))

	found, err := repo.GetByID(ctx, template.ID(), userID)
	s.Require().NoError(err)
	s.Equal(int64(400000), found.Amount().Cents())
	s.Equal(int64(2), found.Version())
}

func (s *RecurringTemplateRepositorySuite) TestUpdateWithVersion_Conflict() {
	mgr, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	ctx := context.Background()
	db := mgr.DBTX(ctx)
	repo := txpostgres.NewRecurringTemplateRepository(o11y, db)

	userID := uuid.New()
	template := s.newTemplate(userID, 5)
	s.Require().NoError(repo.Create(ctx, template))

	amount2, _ := valueobjects.NewMoney(400000)
	desc2, _ := valueobjects.NewDescription("Bônus")
	now := time.Now().UTC()
	template.Update(
		template.Direction(), template.PaymentMethod(), template.CardID(),
		amount2, desc2, template.CategoryID(), template.SubcategoryID(),
		"Receita", "", template.Frequency(), template.DayOfMonth(), template.InstallmentsTotal(),
		template.StartedAt(), template.EndedAt(), now,
	)

	err := repo.UpdateWithVersion(ctx, template, 999)
	s.Require().Error(err)
}

func (s *RecurringTemplateRepositorySuite) TestSoftDelete_Success() {
	mgr, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	ctx := context.Background()
	db := mgr.DBTX(ctx)
	repo := txpostgres.NewRecurringTemplateRepository(o11y, db)

	userID := uuid.New()
	template := s.newTemplate(userID, 5)
	s.Require().NoError(repo.Create(ctx, template))

	s.Require().NoError(repo.SoftDelete(ctx, template.ID(), userID, 1, time.Now().UTC()))

	_, err := repo.GetByID(ctx, template.ID(), userID)
	s.Require().Error(err)
}

func (s *RecurringTemplateRepositorySuite) TestFindActiveByDayOfMonth_Batches() {
	mgr, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	ctx := context.Background()
	db := mgr.DBTX(ctx)
	repo := txpostgres.NewRecurringTemplateRepository(o11y, db)

	userID := uuid.New()
	for i := 0; i < 5; i++ {
		t := s.newTemplate(userID, 15)
		s.Require().NoError(repo.Create(ctx, t))
	}

	asOf := time.Now().UTC()
	results, nextCursor, err := repo.FindActiveByDayOfMonth(ctx, 15, asOf, interfaces.Cursor{}, 3)
	s.Require().NoError(err)
	s.Len(results, 3)
	s.NotEmpty(nextCursor.Value)

	results2, nextCursor2, err := repo.FindActiveByDayOfMonth(ctx, 15, asOf, nextCursor, 3)
	s.Require().NoError(err)
	s.Len(results2, 2)
	s.Empty(nextCursor2.Value)
}

func (s *RecurringTemplateRepositorySuite) TestList_ActiveOnly() {
	mgr, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	ctx := context.Background()
	db := mgr.DBTX(ctx)
	repo := txpostgres.NewRecurringTemplateRepository(o11y, db)

	userID := uuid.New()
	active := s.newTemplate(userID, 5)
	s.Require().NoError(repo.Create(ctx, active))
	deleted := s.newTemplate(userID, 10)
	s.Require().NoError(repo.Create(ctx, deleted))
	s.Require().NoError(repo.SoftDelete(ctx, deleted.ID(), userID, 1, time.Now().UTC()))

	all, _, err := repo.List(ctx, userID, false, interfaces.Cursor{}, 50)
	s.Require().NoError(err)
	s.Len(all, 1)

	activeOnly, _, err := repo.List(ctx, userID, true, interfaces.Cursor{}, 50)
	s.Require().NoError(err)
	s.Len(activeOnly, 1)
}
