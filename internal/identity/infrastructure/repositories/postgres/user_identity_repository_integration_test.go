//go:build integration

package postgres_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
)

type UserIdentityRepositoryIntegrationSuite struct {
	suite.Suite
	ctx     context.Context
	db      *sqlx.DB
	factory interfaces.RepositoryFactory
}

func TestUserIdentityRepositoryIntegrationSuite(t *testing.T) {
	suite.Run(t, new(UserIdentityRepositoryIntegrationSuite))
}

func (s *UserIdentityRepositoryIntegrationSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *UserIdentityRepositoryIntegrationSuite) SetupSuite() {
	db, _ := setupTestDB(s.T())
	s.db = db
	s.factory = repositories.NewRepositoryFactory(noop.NewProvider())
}

func (s *UserIdentityRepositoryIntegrationSuite) newRepo() interfaces.UserIdentityRepository {
	return s.factory.UserIdentityRepository(s.db)
}

func (s *UserIdentityRepositoryIntegrationSuite) seedUser() uuid.UUID {
	id := uuid.New()
	_, err := s.db.ExecContext(
		s.ctx,
		`INSERT INTO users (id, whatsapp_number, status, created_at, updated_at, deleted_at)
		 VALUES ($1, $2, 'ACTIVE', now(), now(), NULL)`,
		id,
		"+5511900000"+id.String()[:4],
	)
	s.Require().NoError(err)
	return id
}

func (s *UserIdentityRepositoryIntegrationSuite) newTelegramIdentity(userID uuid.UUID, externalID string, now time.Time) entities.UserIdentity {
	ch := valueobjects.ChannelTelegram()
	extID, err := valueobjects.NewExternalID(ch, externalID)
	s.Require().NoError(err)
	identity, err := entities.NewUserIdentity(uuid.New(), userID, ch, extID, now)
	s.Require().NoError(err)
	return identity
}

func (s *UserIdentityRepositoryIntegrationSuite) TestInsertAndTryFindActive() {
	scenarios := []struct {
		name   string
		setup  func(interfaces.UserIdentityRepository) (entities.UserIdentity, bool, error)
		expect func(entities.UserIdentity, bool, error)
	}{
		{
			name: "deve inserir identidade e encontrar como ativa",
			setup: func(repo interfaces.UserIdentityRepository) (entities.UserIdentity, bool, error) {
				userID := s.seedUser()
				now := time.Now().UTC().Truncate(time.Microsecond)
				identity := s.newTelegramIdentity(userID, "100001", now)

				err := repo.Insert(s.ctx, identity)
				s.Require().NoError(err)

				var count int
				err = s.db.QueryRowContext(
					s.ctx,
					`SELECT COUNT(*) FROM mecontrola.user_identities WHERE user_id = $1 AND channel = 'telegram' AND unlinked_at IS NULL`,
					userID,
				).Scan(&count)
				s.Require().NoError(err)
				s.Equal(1, count)

				found, ok, findErr := repo.TryFindActive(s.ctx, identity.Channel(), identity.ExternalID())
				return found, ok, findErr
			},
			expect: func(found entities.UserIdentity, ok bool, err error) {
				s.Require().NoError(err)
				s.True(ok)
				s.Equal("telegram", found.Channel().String())
				s.Equal("100001", found.ExternalID().String())
				s.True(found.IsActive())
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			repo := s.newRepo()
			found, ok, err := scenario.setup(repo)
			scenario.expect(found, ok, err)
		})
	}
}

func (s *UserIdentityRepositoryIntegrationSuite) TestInsertDuplicate() {
	scenarios := []struct {
		name   string
		setup  func(interfaces.UserIdentityRepository) (int, error)
		expect func(int, error)
	}{
		{
			name: "deve retornar erro ao inserir identidade duplicada ativa",
			setup: func(repo interfaces.UserIdentityRepository) (int, error) {
				userID := s.seedUser()
				now := time.Now().UTC().Truncate(time.Microsecond)
				identity := s.newTelegramIdentity(userID, "200001", now)

				s.Require().NoError(repo.Insert(s.ctx, identity))

				duplicate, err := entities.NewUserIdentity(uuid.New(), userID, identity.Channel(), identity.ExternalID(), now.Add(time.Second))
				s.Require().NoError(err)

				insertErr := repo.Insert(s.ctx, duplicate)

				var count int
				countErr := s.db.QueryRowContext(
					s.ctx,
					`SELECT COUNT(*) FROM mecontrola.user_identities WHERE user_id = $1 AND channel = 'telegram' AND unlinked_at IS NULL`,
					userID,
				).Scan(&count)
				s.Require().NoError(countErr)

				return count, insertErr
			},
			expect: func(count int, err error) {
				s.Require().Error(err)
				s.ErrorIs(err, application.ErrUserIdentityAlreadyLinked)
				s.Equal(1, count)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			repo := s.newRepo()
			count, err := scenario.setup(repo)
			scenario.expect(count, err)
		})
	}
}

func (s *UserIdentityRepositoryIntegrationSuite) TestUnlink() {
	scenarios := []struct {
		name   string
		setup  func(interfaces.UserIdentityRepository) (entities.UserIdentity, bool, error)
		expect func(entities.UserIdentity, bool, error)
	}{
		{
			name: "deve desvincular identidade e nao encontrar como ativa",
			setup: func(repo interfaces.UserIdentityRepository) (entities.UserIdentity, bool, error) {
				userID := s.seedUser()
				now := time.Now().UTC().Truncate(time.Microsecond)
				identity := s.newTelegramIdentity(userID, "300001", now)

				s.Require().NoError(repo.Insert(s.ctx, identity))

				unlinkAt := now.Add(time.Second)
				s.Require().NoError(repo.Unlink(s.ctx, identity.ID(), unlinkAt))

				var unlinkedAt sql.NullTime
				err := s.db.QueryRowContext(
					s.ctx,
					`SELECT unlinked_at FROM mecontrola.user_identities WHERE id = $1`,
					identity.ID(),
				).Scan(&unlinkedAt)
				s.Require().NoError(err)
				s.True(unlinkedAt.Valid)

				found, ok, findErr := repo.TryFindActive(s.ctx, identity.Channel(), identity.ExternalID())
				return found, ok, findErr
			},
			expect: func(found entities.UserIdentity, ok bool, err error) {
				s.Require().NoError(err)
				s.False(ok)
				s.Equal(entities.UserIdentity{}, found)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			repo := s.newRepo()
			found, ok, err := scenario.setup(repo)
			scenario.expect(found, ok, err)
		})
	}
}

func (s *UserIdentityRepositoryIntegrationSuite) TestListByUser() {
	scenarios := []struct {
		name   string
		setup  func(interfaces.UserIdentityRepository) ([]entities.UserIdentity, int, error)
		expect func([]entities.UserIdentity, int, error)
	}{
		{
			name: "deve retornar apenas identidades ativas ao listar por usuario",
			setup: func(repo interfaces.UserIdentityRepository) ([]entities.UserIdentity, int, error) {
				userID := s.seedUser()
				now := time.Now().UTC().Truncate(time.Microsecond)

				chTelegram := valueobjects.ChannelTelegram()
				extTelegram, err := valueobjects.NewExternalID(chTelegram, "400001")
				s.Require().NoError(err)
				identityTelegram, err := entities.NewUserIdentity(uuid.New(), userID, chTelegram, extTelegram, now)
				s.Require().NoError(err)
				s.Require().NoError(repo.Insert(s.ctx, identityTelegram))

				chWhatsApp := valueobjects.ChannelWhatsApp()
				extWhatsApp, err := valueobjects.NewExternalID(chWhatsApp, "+5511900000400")
				s.Require().NoError(err)
				identityWhatsApp, err := entities.NewUserIdentity(uuid.New(), userID, chWhatsApp, extWhatsApp, now.Add(time.Millisecond))
				s.Require().NoError(err)
				s.Require().NoError(repo.Insert(s.ctx, identityWhatsApp))

				s.Require().NoError(repo.Unlink(s.ctx, identityWhatsApp.ID(), now.Add(time.Second)))

				var count int
				countErr := s.db.QueryRowContext(
					s.ctx,
					`SELECT COUNT(*) FROM mecontrola.user_identities WHERE user_id = $1 AND unlinked_at IS NULL`,
					userID,
				).Scan(&count)
				s.Require().NoError(countErr)

				list, listErr := repo.ListByUser(s.ctx, userID)
				return list, count, listErr
			},
			expect: func(list []entities.UserIdentity, dbCount int, err error) {
				s.Require().NoError(err)
				s.Equal(1, dbCount)
				active := 0
				for _, id := range list {
					if id.IsActive() {
						active++
					}
				}
				s.Equal(1, active)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			repo := s.newRepo()
			list, count, err := scenario.setup(repo)
			scenario.expect(list, count, err)
		})
	}
}

func (s *UserIdentityRepositoryIntegrationSuite) TestFindByUserAndChannel() {
	scenarios := []struct {
		name   string
		setup  func(interfaces.UserIdentityRepository) (entities.UserIdentity, bool, error)
		expect func(entities.UserIdentity, bool, error)
	}{
		{
			name: "deve encontrar identidade por usuario e channel",
			setup: func(repo interfaces.UserIdentityRepository) (entities.UserIdentity, bool, error) {
				userID := s.seedUser()
				now := time.Now().UTC().Truncate(time.Microsecond)
				identity := s.newTelegramIdentity(userID, "500001", now)

				s.Require().NoError(repo.Insert(s.ctx, identity))

				found, ok, findErr := repo.FindByUserAndChannel(s.ctx, userID, identity.Channel())
				return found, ok, findErr
			},
			expect: func(found entities.UserIdentity, ok bool, err error) {
				s.Require().NoError(err)
				s.True(ok)
				s.Equal("telegram", found.Channel().String())
				s.Equal("500001", found.ExternalID().String())
				s.True(found.IsActive())
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			repo := s.newRepo()
			found, ok, err := scenario.setup(repo)
			scenario.expect(found, ok, err)
		})
	}
}
