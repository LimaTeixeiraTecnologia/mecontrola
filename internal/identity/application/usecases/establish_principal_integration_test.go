//go:build integration

package usecases_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	application "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type EstablishPrincipalIntegrationSuite struct {
	suite.Suite
	ctx  context.Context
	db   *sqlx.DB
	o11y *noop.Provider
}

func TestEstablishPrincipalIntegration(t *testing.T) {
	suite.Run(t, new(EstablishPrincipalIntegrationSuite))
}

func (s *EstablishPrincipalIntegrationSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *EstablishPrincipalIntegrationSuite) SetupSuite() {
	db, _ := testcontainer.Postgres(s.T())
	s.db = db
	s.o11y = noop.NewProvider()
}

func setupEstablishTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, _ := testcontainer.Postgres(t)
	return db
}

func (s *EstablishPrincipalIntegrationSuite) outboxCfg() configs.OutboxConfig {
	return configs.OutboxConfig{RetryMaxAttempts: 3}
}

func (s *EstablishPrincipalIntegrationSuite) newPublisher() outbox.Publisher {
	storage := outbox.NewPostgresStorage(s.db)
	return outbox.NewPostgresPublisher(storage, s.outboxCfg())
}

func (s *EstablishPrincipalIntegrationSuite) seedActiveUser(wa string) entities.User {
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

func (s *EstablishPrincipalIntegrationSuite) countOutboxByType(eventType string) int {
	var total int
	err := s.db.QueryRowContext(
		s.ctx,
		`SELECT COUNT(*) FROM outbox_events WHERE event_type = $1`,
		eventType,
	).Scan(&total)
	s.Require().NoError(err)
	return total
}

func (s *EstablishPrincipalIntegrationSuite) countActiveIdentitiesByExternalID(externalID string) int {
	var total int
	err := s.db.QueryRowContext(
		s.ctx,
		`SELECT COUNT(*) FROM mecontrola.user_identities WHERE channel = $1 AND external_id = $2 AND unlinked_at IS NULL`,
		"whatsapp",
		externalID,
	).Scan(&total)
	s.Require().NoError(err)
	return total
}

func (s *EstablishPrincipalIntegrationSuite) latestResolvePathForAggregate(aggregateUserID string) *string {
	var payload []byte
	err := s.db.QueryRowContext(
		s.ctx,
		`SELECT payload FROM outbox_events WHERE aggregate_user_id = $1 AND event_type = 'auth.principal_established' ORDER BY created_at DESC LIMIT 1`,
		aggregateUserID,
	).Scan(&payload)
	s.Require().NoError(err)
	var decoded struct {
		ResolvePath *string `json:"resolve_path"`
	}
	s.Require().NoError(json.Unmarshal(payload, &decoded))
	return decoded.ResolvePath
}

func (s *EstablishPrincipalIntegrationSuite) newSUT() *usecases.EstablishPrincipal {
	factory := repositories.NewRepositoryFactory(s.o11y)
	u := uow.NewUnitOfWork(s.db)
	return usecases.NewEstablishPrincipal(u, factory, s.newPublisher(), s.o11y)
}

func (s *EstablishPrincipalIntegrationSuite) TestEstablishPrincipal() {
	type args struct {
		waNumber string
	}

	scenarios := []struct {
		name   string
		setup  func() args
		expect func(auth.Principal, error)
	}{
		{
			name: "usuario ativo: retorna Principal e linha outbox auth.principal_established",
			setup: func() args {
				const wa = "+5511900000001"
				s.seedActiveUser(wa)
				return args{waNumber: wa}
			},
			expect: func(p auth.Principal, err error) {
				s.Require().NoError(err)
				s.False(p.IsZero())
				s.Equal(auth.SourceWhatsApp, p.Source)
				s.GreaterOrEqual(s.countOutboxByType("auth.principal_established"), 1)
			},
		},
		{
			name: "usuario inexistente: retorna ErrUnknownUser e linha outbox auth.unknown_user",
			setup: func() args {
				return args{waNumber: "+5511900000099"}
			},
			expect: func(p auth.Principal, err error) {
				s.Require().ErrorIs(err, application.ErrUnknownUser)
				s.True(p.IsZero())
				s.GreaterOrEqual(s.countOutboxByType("auth.unknown_user"), 1)
			},
		},
		{
			name: "rollback observavel: outbox invalido causa erro e nenhuma linha adicional e inserida",
			setup: func() args {
				const wa = "+5511900000002"
				s.seedActiveUser(wa)
				return args{waNumber: wa}
			},
			expect: func(p auth.Principal, err error) {
				s.Require().NoError(err)
				s.False(p.IsZero())
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			a := scenario.setup()
			sut := s.newSUT()
			p, err := sut.Execute(s.ctx, input.EstablishPrincipalInput{WhatsAppNumber: a.waNumber})
			scenario.expect(p, err)
		})
	}
}

func (s *EstablishPrincipalIntegrationSuite) TestLegacyResolutionCreatesCanonicalLink() {
	const wa = "+5511900001001"
	user := s.seedActiveUser(wa)
	s.Require().Equal(0, s.countActiveIdentitiesByExternalID(wa))

	sut := s.newSUT()
	p, err := sut.Execute(s.ctx, input.EstablishPrincipalInput{WhatsAppNumber: wa})
	s.Require().NoError(err)
	s.False(p.IsZero())

	s.Equal(1, s.countActiveIdentitiesByExternalID(wa))

	resolvePath := s.latestResolvePathForAggregate(user.ID())
	s.Require().NotNil(resolvePath)
	s.Equal("legacy", *resolvePath)
}

func (s *EstablishPrincipalIntegrationSuite) TestSecondResolutionUsesIdentityWithoutDuplicateLink() {
	const wa = "+5511900001002"
	user := s.seedActiveUser(wa)

	sut := s.newSUT()

	_, firstErr := sut.Execute(s.ctx, input.EstablishPrincipalInput{WhatsAppNumber: wa})
	s.Require().NoError(firstErr)
	s.Equal(1, s.countActiveIdentitiesByExternalID(wa))
	firstPath := s.latestResolvePathForAggregate(user.ID())
	s.Require().NotNil(firstPath)
	s.Equal("legacy", *firstPath)

	_, secondErr := sut.Execute(s.ctx, input.EstablishPrincipalInput{WhatsAppNumber: wa})
	s.Require().NoError(secondErr)
	s.Equal(1, s.countActiveIdentitiesByExternalID(wa))
	secondPath := s.latestResolvePathForAggregate(user.ID())
	s.Require().NotNil(secondPath)
	s.Equal("identity", *secondPath)
}

func (s *EstablishPrincipalIntegrationSuite) TestConcurrentResolutionCreatesSingleLink() {
	const wa = "+5511900001003"
	s.seedActiveUser(wa)

	const workers = 4
	errs := make(chan error, workers)
	start := make(chan struct{})
	for range workers {
		go func() {
			<-start
			sut := s.newSUT()
			_, execErr := sut.Execute(s.ctx, input.EstablishPrincipalInput{WhatsAppNumber: wa})
			errs <- execErr
		}()
	}
	close(start)

	for range workers {
		s.Require().NoError(<-errs)
	}

	s.Equal(1, s.countActiveIdentitiesByExternalID(wa))
}
