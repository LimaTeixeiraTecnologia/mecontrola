//go:build integration

package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type MarkUserDeletedIntegrationSuite struct {
	suite.Suite
	ctx  context.Context
	db   *sqlx.DB
	o11y *noop.Provider
}

func TestMarkUserDeletedIntegration(t *testing.T) {
	suite.Run(t, new(MarkUserDeletedIntegrationSuite))
}

func (s *MarkUserDeletedIntegrationSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *MarkUserDeletedIntegrationSuite) SetupSuite() {
	db, _ := testcontainer.Postgres(s.T())
	s.db = db
	s.o11y = noop.NewProvider()
}

func setupMarkDeletedTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, _ := testcontainer.Postgres(t)
	return db
}

func (s *MarkUserDeletedIntegrationSuite) outboxCfg() configs.OutboxConfig {
	return configs.OutboxConfig{RetryMaxAttempts: 3}
}

func (s *MarkUserDeletedIntegrationSuite) newPublisher() outbox.Publisher {
	storage := outbox.NewPostgresStorage(s.db)
	return outbox.NewPostgresPublisher(storage, s.outboxCfg())
}

func (s *MarkUserDeletedIntegrationSuite) seedActiveUser(wa string) entities.User {
	s.T().Helper()
	factory := repositories.NewRepositoryFactory(s.o11y)
	repo := factory.UserRepository(s.db)
	waNum, err := valueobjects.NewWhatsAppNumber(wa)
	s.Require().NoError(err)
	candidate := entities.New(waNum)
	user, err := repo.UpsertByWhatsAppNumber(s.ctx, candidate, time.Now().UTC())
	s.Require().NoError(err)
	return user
}

func (s *MarkUserDeletedIntegrationSuite) countOutboxByType(eventType string) int {
	var total int
	err := s.db.QueryRowContext(
		s.ctx,
		`SELECT COUNT(*) FROM outbox_events WHERE event_type = $1`,
		eventType,
	).Scan(&total)
	s.Require().NoError(err)
	return total
}

func (s *MarkUserDeletedIntegrationSuite) newSUT() *usecases.MarkUserDeleted {
	factory := repositories.NewRepositoryFactory(s.o11y)
	u := uow.NewUnitOfWork(s.db)
	return usecases.NewMarkUserDeleted(u, factory, s.newPublisher(), s.o11y)
}

func (s *MarkUserDeletedIntegrationSuite) TestMarkUserDeleted() {
	type args struct {
		userID string
	}

	scenarios := []struct {
		name   string
		setup  func() args
		expect func(error)
	}{
		{
			name: "usuario ativo: marca como deletado e publica user.deleted no outbox",
			setup: func() args {
				const wa = "+5511900000001"
				user := s.seedActiveUser(wa)
				return args{userID: user.ID()}
			},
			expect: func(err error) {
				s.Require().NoError(err)
				s.GreaterOrEqual(s.countOutboxByType("user.deleted"), 1)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			a := scenario.setup()
			sut := s.newSUT()
			err := sut.Execute(s.ctx, input.MarkUserDeleted{ID: a.userID})
			scenario.expect(err)
		})
	}
}
