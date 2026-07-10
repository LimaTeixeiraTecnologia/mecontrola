//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
)

type AuthEventsRepositoryIntegrationSuite struct {
	suite.Suite
	ctx     context.Context
	db      *sqlx.DB
	factory interfaces.RepositoryFactory
}

func TestAuthEventsRepositoryIntegrationSuite(t *testing.T) {
	suite.Run(t, new(AuthEventsRepositoryIntegrationSuite))
}

func (s *AuthEventsRepositoryIntegrationSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *AuthEventsRepositoryIntegrationSuite) SetupSuite() {
	db, _ := setupTestDB(s.T())
	s.db = db
	s.factory = repositories.NewRepositoryFactory(noop.NewProvider())
}

func (s *AuthEventsRepositoryIntegrationSuite) newRepo() interfaces.AuthEventsRepository {
	return s.factory.AuthEventsRepository(s.db)
}

func ptr[T any](v T) *T { return &v }

func (s *AuthEventsRepositoryIntegrationSuite) TestInsert() {
	scenarios := []struct {
		name       string
		buildEvent func() entities.AuthEvent
		expectErr  bool
	}{
		{
			name: "deve inserir evento principal_established sem erro",
			buildEvent: func() entities.AuthEvent {
				uid := uuid.New()
				ev, err := entities.NewPrincipalEstablished(uid, entities.AuthEventSourceWhatsApp, "", "")
				s.Require().NoError(err)
				return ev
			},
			expectErr: false,
		},
		{
			name: "deve inserir evento unknown_user sem user_id",
			buildEvent: func() entities.AuthEvent {
				ev, err := entities.NewUnknownUser(entities.AuthEventSourceWhatsApp)
				s.Require().NoError(err)
				return ev
			},
			expectErr: false,
		},
		{
			name: "deve inserir evento failed com reason invalid_signature",
			buildEvent: func() entities.AuthEvent {
				ev, err := entities.NewAuthFailed(entities.AuthEventReasonInvalidSignature, entities.AuthEventSourceWhatsApp, nil, "", "")
				s.Require().NoError(err)
				return ev
			},
			expectErr: false,
		},
		{
			name: "deve inserir evento failed com reason invalid_payload",
			buildEvent: func() entities.AuthEvent {
				ev, err := entities.NewAuthFailed(entities.AuthEventReasonInvalidPayload, entities.AuthEventSourceWhatsApp, nil, "", "")
				s.Require().NoError(err)
				return ev
			},
			expectErr: false,
		},
		{
			name: "deve inserir evento failed gateway com request_id e client_ip",
			buildEvent: func() entities.AuthEvent {
				ev, err := entities.NewAuthFailed(entities.AuthEventReasonGatewayInvalidSignature, entities.AuthEventSourceGateway, nil, "req-gateway-001", "10.0.0.1")
				s.Require().NoError(err)
				return ev
			},
			expectErr: false,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			repo := s.newRepo()
			ev := scenario.buildEvent()
			err := repo.Insert(s.ctx, ev)
			if scenario.expectErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
			}
		})
	}
}

func (s *AuthEventsRepositoryIntegrationSuite) TestInsertIdempotence() {
	s.Run("deve ser idempotente por PK — segundo insert e no-op", func() {
		repo := s.newRepo()
		uid := uuid.New()
		ev, err := entities.NewPrincipalEstablished(uid, entities.AuthEventSourceWhatsApp, "", "")
		s.Require().NoError(err)

		s.Require().NoError(repo.Insert(s.ctx, ev))
		s.Require().NoError(repo.Insert(s.ctx, ev))
	})
}

func (s *AuthEventsRepositoryIntegrationSuite) TestInsertGatewayForensicsPersistsFields() {
	repo := s.newRepo()
	ev, err := entities.NewAuthFailed(entities.AuthEventReasonGatewayInvalidSignature, entities.AuthEventSourceGateway, nil, "req-gateway-002", "10.0.0.2")
	s.Require().NoError(err)

	s.Require().NoError(repo.Insert(s.ctx, ev))

	var source, requestID, clientIP string
	err = s.db.QueryRowContext(s.ctx,
		`SELECT source, request_id, host(client_ip) FROM auth_events WHERE id = $1`, ev.ID(),
	).Scan(&source, &requestID, &clientIP)
	s.Require().NoError(err)
	s.Equal("gateway", source)
	s.Equal("req-gateway-002", requestID)
	s.Equal("10.0.0.2", clientIP)
}

func (s *AuthEventsRepositoryIntegrationSuite) TestCheckConstraints() {
	s.Run("deve rejeitar kind invalido", func() {
		repo := s.newRepo()
		id, err := uuid.NewV7()
		s.Require().NoError(err)

		invalidEv := entities.HydrateAuthEvent(
			id,
			time.Now().UTC(),
			nil,
			entities.AuthEventKind("invalid_kind"),
			entities.AuthEventSourceWhatsApp,
			nil,
			nil,
			"",
			"",
		)
		err = repo.Insert(s.ctx, invalidEv)
		s.Require().Error(err, "deve falhar com CHECK constraint violation em kind")
	})

	s.Run("deve rejeitar source invalido", func() {
		repo := s.newRepo()
		id, err := uuid.NewV7()
		s.Require().NoError(err)

		invalidEv := entities.HydrateAuthEvent(
			id,
			time.Now().UTC(),
			nil,
			entities.AuthEventKindUnknownUser,
			entities.AuthEventSource("invalid_source"),
			nil,
			nil,
			"",
			"",
		)
		err = repo.Insert(s.ctx, invalidEv)
		s.Require().Error(err, "deve falhar com CHECK constraint violation em source")
	})

	s.Run("deve rejeitar reason invalido para kind failed", func() {
		repo := s.newRepo()
		id, err := uuid.NewV7()
		s.Require().NoError(err)

		invalidReason := entities.AuthEventReason("invalid_reason")
		invalidEv := entities.HydrateAuthEvent(
			id,
			time.Now().UTC(),
			nil,
			entities.AuthEventKindFailed,
			entities.AuthEventSourceWhatsApp,
			&invalidReason,
			nil,
			"",
			"",
		)
		err = repo.Insert(s.ctx, invalidEv)
		s.Require().Error(err, "deve falhar com CHECK constraint violation em reason")
	})
}

func (s *AuthEventsRepositoryIntegrationSuite) TestAnonymizeByUserID() {
	s.Run("deve anonimizar user_id para NULL em todas as linhas do usuario", func() {
		repo := s.newRepo()
		uid := uuid.New()

		for range 2 {
			ev, err := entities.NewPrincipalEstablished(uid, entities.AuthEventSourceWhatsApp, "", "")
			s.Require().NoError(err)
			s.Require().NoError(repo.Insert(s.ctx, ev))
		}

		otherUID := uuid.New()
		otherEv, err := entities.NewPrincipalEstablished(otherUID, entities.AuthEventSourceWhatsApp, "", "")
		s.Require().NoError(err)
		s.Require().NoError(repo.Insert(s.ctx, otherEv))

		s.Require().NoError(repo.AnonymizeByUserID(s.ctx, uid))

		var countWithUID int
		err = s.db.QueryRowContext(s.ctx,
			"SELECT COUNT(*) FROM auth_events WHERE user_id = $1", uid,
		).Scan(&countWithUID)
		s.Require().NoError(err)
		s.Equal(0, countWithUID, "deve haver 0 linhas com o user_id original após anonimização")

		var countOther int
		err = s.db.QueryRowContext(s.ctx,
			"SELECT COUNT(*) FROM auth_events WHERE user_id = $1", otherUID,
		).Scan(&countOther)
		s.Require().NoError(err)
		s.Equal(1, countOther, "o outro usuario nao deve ter sido afetado")
	})

	s.Run("deve ser idempotente — segunda chamada e no-op", func() {
		repo := s.newRepo()
		uid := uuid.New()

		ev, err := entities.NewPrincipalEstablished(uid, entities.AuthEventSourceWhatsApp, "", "")
		s.Require().NoError(err)
		s.Require().NoError(repo.Insert(s.ctx, ev))

		s.Require().NoError(repo.AnonymizeByUserID(s.ctx, uid))
		s.Require().NoError(repo.AnonymizeByUserID(s.ctx, uid))
	})
}

func (s *AuthEventsRepositoryIntegrationSuite) TestDeleteOlderThan() {
	s.Run("deve apagar linhas em lotes respeitando cutoff", func() {
		repo := s.newRepo()

		cutoff := time.Now().UTC().Add(-1 * time.Hour)
		oldTime := time.Now().UTC().Add(-2 * time.Hour)

		for range 5 {
			id, err := uuid.NewV7()
			s.Require().NoError(err)
			ev := entities.HydrateAuthEvent(id, oldTime, nil, entities.AuthEventKindUnknownUser, entities.AuthEventSourceWhatsApp, nil, nil, "", "")
			s.Require().NoError(repo.Insert(s.ctx, ev))
		}

		for range 2 {
			ev, err := entities.NewUnknownUser(entities.AuthEventSourceWhatsApp)
			s.Require().NoError(err)
			s.Require().NoError(repo.Insert(s.ctx, ev))
		}

		n, err := repo.DeleteOlderThan(s.ctx, cutoff, 3)
		s.Require().NoError(err)
		s.Equal(int64(3), n, "deve apagar 3 linhas no primeiro lote")

		n, err = repo.DeleteOlderThan(s.ctx, cutoff, 3)
		s.Require().NoError(err)
		s.Equal(int64(2), n, "deve apagar 2 linhas no segundo lote")

		n, err = repo.DeleteOlderThan(s.ctx, cutoff, 3)
		s.Require().NoError(err)
		s.Equal(int64(0), n, "deve retornar 0 quando nao ha mais linhas para apagar")
	})

	s.Run("deve ser idempotente na segunda execucao", func() {
		repo := s.newRepo()
		cutoff := time.Now().UTC().Add(-1 * time.Minute)

		n, err := repo.DeleteOlderThan(s.ctx, cutoff, 10000)
		s.Require().NoError(err)

		n2, err := repo.DeleteOlderThan(s.ctx, cutoff, 10000)
		s.Require().NoError(err)
		_ = n

		s.Equal(int64(0), n2, "segunda execucao deve ser no-op")
	})
}

func (s *AuthEventsRepositoryIntegrationSuite) TestBenchmarkInsert() {
	s.Run("deve completar 100 inserts sem erro", func() {
		repo := s.newRepo()
		for range 100 {
			ev, err := entities.NewUnknownUser(entities.AuthEventSourceWhatsApp)
			s.Require().NoError(err)
			s.Require().NoError(repo.Insert(s.ctx, ev))
		}
	})
}
