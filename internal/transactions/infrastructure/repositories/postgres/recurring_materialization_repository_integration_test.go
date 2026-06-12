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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
	txpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/repositories/postgres"
)

type RecurringMaterializationRepositorySuite struct {
	suite.Suite
}

func TestRecurringMaterializationRepositorySuite(t *testing.T) {
	suite.Run(t, new(RecurringMaterializationRepositorySuite))
}

func (s *RecurringMaterializationRepositorySuite) buildRecurringTemplate(userID uuid.UUID) *entities.RecurringTemplate {
	dir := valueobjects.DirectionIncome
	pm := valueobjects.PaymentMethodPix
	amount, _ := valueobjects.NewMoney(5000)
	desc, _ := valueobjects.NewDescription("Salário")
	catID := valueobjects.CategoryIDFromUUID(uuid.New())
	freq := valueobjects.FrequencyMonthly
	dom, _ := valueobjects.NewDayOfMonth(5)
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

func (s *RecurringMaterializationRepositorySuite) TestInsertIfAbsent_IdempotentOnSameKey() {
	mgr, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	templateRepo := txpostgres.NewRecurringTemplateRepository(o11y, db)
	materializationRepo := txpostgres.NewRecurringMaterializationRepository(o11y, db)

	userID := uuid.New()
	template := s.buildRecurringTemplate(userID)
	s.Require().NoError(templateRepo.Create(ctx, template))

	refMonth, _ := valueobjects.NewRefMonth("2026-06")
	now := time.Now().UTC()

	inserted1, err1 := materializationRepo.InsertIfAbsent(ctx, template.ID(), refMonth, nil, nil, now)
	s.Require().NoError(err1)
	s.True(inserted1, "primeira inserção deve retornar true")

	inserted2, err2 := materializationRepo.InsertIfAbsent(ctx, template.ID(), refMonth, nil, nil, now)
	s.Require().NoError(err2)
	s.False(inserted2, "segunda inserção (mesmo key) deve retornar false")
}

func (s *RecurringMaterializationRepositorySuite) TestInsertIfAbsent_DifferentRefMonth() {
	mgr, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	templateRepo := txpostgres.NewRecurringTemplateRepository(o11y, db)
	materializationRepo := txpostgres.NewRecurringMaterializationRepository(o11y, db)

	userID := uuid.New()
	template := s.buildRecurringTemplate(userID)
	s.Require().NoError(templateRepo.Create(ctx, template))

	refMonth1, _ := valueobjects.NewRefMonth("2026-06")
	refMonth2, _ := valueobjects.NewRefMonth("2026-07")
	now := time.Now().UTC()

	inserted1, err1 := materializationRepo.InsertIfAbsent(ctx, template.ID(), refMonth1, nil, nil, now)
	s.Require().NoError(err1)
	s.True(inserted1)

	inserted2, err2 := materializationRepo.InsertIfAbsent(ctx, template.ID(), refMonth2, nil, nil, now)
	s.Require().NoError(err2)
	s.True(inserted2, "meses diferentes devem ter entradas independentes")
}

func (s *RecurringMaterializationRepositorySuite) TestTryAdvisoryLock_ReturnsTrue() {
	mgr, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	ctx := context.Background()
	db := mgr.DBTX(ctx)
	materializationRepo := txpostgres.NewRecurringMaterializationRepository(o11y, db)

	templateID := uuid.New()
	refMonth, _ := valueobjects.NewRefMonth("2026-06")

	acquired, _, err := materializationRepo.TryAdvisoryLock(ctx, templateID, refMonth)
	s.Require().NoError(err)
	s.True(acquired)
}

func (s *RecurringMaterializationRepositorySuite) TestConcurrentInsert_OnlyOneSucceeds() {
	mgr, _ := testcontainer.Postgres(s.T())
	o11y := noop.NewProvider()
	ctx := context.Background()

	templateRepo := txpostgres.NewRecurringTemplateRepository(o11y, mgr.DBTX(ctx))

	userID := uuid.New()
	template := s.buildRecurringTemplate(userID)
	s.Require().NoError(templateRepo.Create(ctx, template))

	refMonth, _ := valueobjects.NewRefMonth("2026-06")

	var wg sync.WaitGroup
	insertedCount := 0
	var mu sync.Mutex

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			db := mgr.DBTX(ctx)
			repo := txpostgres.NewRecurringMaterializationRepository(o11y, db)
			inserted, err := repo.InsertIfAbsent(ctx, template.ID(), refMonth, nil, nil, time.Now().UTC())
			if err == nil && inserted {
				mu.Lock()
				insertedCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	s.Equal(1, insertedCount, "apenas uma goroutine deve ter inserido")
}
