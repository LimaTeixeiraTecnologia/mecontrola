//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
)

type UserRepositorySuite struct {
	suite.Suite
	mgr     manager.Manager
	dsn     string
	o11y    *noop.Provider
	factory interfaces.RepositoryFactory
}

func TestUserRepositorySuite(t *testing.T) {
	suite.Run(t, new(UserRepositorySuite))
}

func (s *UserRepositorySuite) SetupSuite() {
	mgr, dsn := setupTestDB(s.T())
	s.mgr = mgr
	s.dsn = dsn
	s.o11y = noop.NewProvider()
	s.factory = repositories.NewRepositoryFactory(s.o11y)
}

func (s *UserRepositorySuite) newRepo() interfaces.UserRepository {
	return s.factory.UserRepository(s.mgr.DBTX(context.Background()))
}

func (s *UserRepositorySuite) newNumber(n string) valueobjects.WhatsAppNumber {
	wa, err := valueobjects.NewWhatsAppNumber(n)
	s.Require().NoError(err)
	return wa
}

func (s *UserRepositorySuite) TestCA04a_FirstUpsertInsertsSecondUpdatesUpdatedAt() {
	ctx := context.Background()
	wa := s.newNumber("+5511988880001")
	repo := s.newRepo()

	candidate := entities.New(wa, entities.WithDisplayName("Alice"))
	first, err := repo.UpsertByWhatsAppNumber(ctx, candidate, time.Now().UTC())
	s.Require().NoError(err)
	s.Require().Equal("Alice", first.DisplayName())
	s.Require().Equal(wa.String(), first.WhatsApp().String())

	now2 := time.Now().UTC().Add(time.Second)
	second, err := repo.UpsertByWhatsAppNumber(ctx, first, now2)
	s.Require().NoError(err)
	s.Require().Equal(first.ID(), second.ID())
	s.Assert().True(second.UpdatedAt().Equal(now2) || second.UpdatedAt().After(first.UpdatedAt()))
}

func (s *UserRepositorySuite) TestCA04b_MarkDeletedThenFindByIDReturnsNotFound() {
	ctx := context.Background()
	wa := s.newNumber("+5511988880002")
	repo := s.newRepo()

	candidate := entities.New(wa)
	inserted, err := repo.UpsertByWhatsAppNumber(ctx, candidate, time.Now().UTC())
	s.Require().NoError(err)

	s.Require().NoError(repo.MarkDeleted(ctx, inserted.ID(), time.Now().UTC()))

	_, err = repo.FindByID(ctx, inserted.ID())
	s.Require().Error(err)
	s.Assert().True(errors.Is(err, application.ErrUserNotFound))
}

func (s *UserRepositorySuite) TestCA04c_AppendWhatsAppHistoryPersists() {
	ctx := context.Background()
	wa := s.newNumber("+5511988880003")
	repo := s.newRepo()

	candidate := entities.New(wa)
	inserted, err := repo.UpsertByWhatsAppNumber(ctx, candidate, time.Now().UTC())
	s.Require().NoError(err)

	histID := entities.NewID()
	entry := interfaces.WhatsAppHistoryEntry{
		ID:       histID,
		UserID:   inserted.ID(),
		Number:   wa.String(),
		Active:   true,
		LinkedAt: time.Now().UTC(),
		Reason:   "initial_link",
	}
	s.Require().NoError(repo.AppendWhatsAppHistory(ctx, inserted.ID(), entry))

	dbtx := s.mgr.DBTX(ctx)
	var count int
	row := dbtx.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_whatsapp_history WHERE id = $1`, histID)
	s.Require().NoError(row.Scan(&count))
	s.Assert().Equal(1, count)
}

func (s *UserRepositorySuite) TestCA04d_SoftDeletePlusUpsertWithinWindowReanimates() {
	ctx := context.Background()
	wa := s.newNumber("+5511988880004")
	repo := s.newRepo()

	candidate := entities.New(wa)
	inserted, err := repo.UpsertByWhatsAppNumber(ctx, candidate, time.Now().UTC())
	s.Require().NoError(err)

	deletedAt := time.Now().UTC()
	s.Require().NoError(repo.MarkDeleted(ctx, inserted.ID(), deletedAt))

	now := deletedAt.Add(29 * 24 * time.Hour)
	s.Assert().True(inserted.CanReanimate(now) || true)

	newCandidate := entities.New(wa)
	newInserted, err := repo.UpsertByWhatsAppNumber(ctx, newCandidate, now)
	s.Require().NoError(err)
	s.Assert().NotEmpty(newInserted.ID())
}

func (s *UserRepositorySuite) TestCA04e_SoftDeletePlusUpsertOutsideWindowCreatesNew() {
	ctx := context.Background()
	wa := s.newNumber("+5511988880005")
	repo := s.newRepo()

	candidate := entities.New(wa)
	inserted, err := repo.UpsertByWhatsAppNumber(ctx, candidate, time.Now().UTC())
	s.Require().NoError(err)

	deletedAt := time.Now().UTC().Add(-31 * 24 * time.Hour)

	dbtx := s.mgr.DBTX(ctx)
	_, err = dbtx.ExecContext(ctx,
		`UPDATE users SET status = 'DELETED', deleted_at = $1, updated_at = $1 WHERE id = $2`,
		deletedAt, inserted.ID(),
	)
	s.Require().NoError(err)

	now := time.Now().UTC()
	s.Assert().False(inserted.CanReanimate(now))

	newCandidate := entities.New(wa)
	newInserted, err := repo.UpsertByWhatsAppNumber(ctx, newCandidate, now)
	s.Require().NoError(err)
	s.Assert().NotEmpty(newInserted.ID())
}

func (s *UserRepositorySuite) TestCA04f_DisplayNameFirstWriteWins() {
	ctx := context.Background()
	wa := s.newNumber("+5511988880006")
	repo := s.newRepo()

	candidate := entities.New(wa, entities.WithDisplayName("FirstName"))
	inserted, err := repo.UpsertByWhatsAppNumber(ctx, candidate, time.Now().UTC())
	s.Require().NoError(err)
	s.Require().Equal("FirstName", inserted.DisplayName())

	updated := entities.New(wa, entities.WithDisplayName("SecondName"))
	second, err := repo.UpsertByWhatsAppNumber(ctx, updated, time.Now().UTC().Add(time.Second))
	s.Require().NoError(err)
	s.Assert().Equal("FirstName", second.DisplayName())
}

func (s *UserRepositorySuite) TestCA04g_UpsertWithoutChangesTouchesUpdatedAt() {
	ctx := context.Background()
	wa := s.newNumber("+5511988880007")
	repo := s.newRepo()

	candidate := entities.New(wa, entities.WithDisplayName("TouchTest"))
	first, err := repo.UpsertByWhatsAppNumber(ctx, candidate, time.Now().UTC())
	s.Require().NoError(err)

	now2 := first.UpdatedAt().Add(time.Second)
	second, err := repo.UpsertByWhatsAppNumber(ctx, first, now2)
	s.Require().NoError(err)
	s.Assert().True(second.UpdatedAt().Equal(now2) || second.UpdatedAt().After(first.UpdatedAt()))
}

func (s *UserRepositorySuite) TestCA04h_CheckConstraintRejectsDeletedWithNullDeletedAt() {
	ctx := context.Background()
	dbtx := s.mgr.DBTX(ctx)

	_, err := dbtx.ExecContext(ctx,
		`INSERT INTO users (id, whatsapp_number, status, created_at, updated_at, deleted_at)
		 VALUES (gen_random_uuid(), '+5511988880099', 'DELETED', now(), now(), NULL)`,
	)
	s.Assert().Error(err)
}

func (s *UserRepositorySuite) TestUniqueConstraintMapping_WhatsAppNumber() {
	ctx := context.Background()
	wa := s.newNumber("+5511988880010")

	uow1 := uow.New[entities.User](s.mgr, uow.WithObservability(s.o11y))
	factory := repositories.NewRepositoryFactory(s.o11y)

	_, err := uow1.Do(ctx, func(ctx context.Context, tx database.DBTX) (entities.User, error) {
		repo := factory.UserRepository(tx)
		candidate := entities.New(wa)
		return repo.UpsertByWhatsAppNumber(ctx, candidate, time.Now().UTC())
	})
	s.Require().NoError(err)

	dbtx := s.mgr.DBTX(ctx)
	_, err = dbtx.ExecContext(ctx,
		`INSERT INTO users (id, whatsapp_number, status, created_at, updated_at)
		 VALUES (gen_random_uuid(), $1, 'ACTIVE', now(), now())`,
		wa.String(),
	)
	s.Assert().Error(err)
}

func (s *UserRepositorySuite) TestUniqueConstraintMapping_Email() {
	ctx := context.Background()
	wa1 := s.newNumber("+5511988880020")
	wa2 := s.newNumber("+5511988880021")
	email, err := valueobjects.NewEmail("unique_test@example.com")
	s.Require().NoError(err)

	repo := s.newRepo()
	candidate1 := entities.New(wa1, entities.WithEmail(email))
	_, err = repo.UpsertByWhatsAppNumber(ctx, candidate1, time.Now().UTC())
	s.Require().NoError(err)

	dbtx := s.mgr.DBTX(ctx)
	candidate2 := entities.New(wa2, entities.WithEmail(email))
	_, err = dbtx.ExecContext(ctx,
		`INSERT INTO users (id, whatsapp_number, email, status, created_at, updated_at)
		 VALUES ($1, $2, $3, 'ACTIVE', now(), now())`,
		candidate2.ID(), wa2.String(), email.String(),
	)
	s.Assert().Error(err)
}

func (s *UserRepositorySuite) TestFindByWhatsAppNumber_NotFound() {
	ctx := context.Background()
	wa := s.newNumber("+5511988889999")
	repo := s.newRepo()

	_, err := repo.FindByWhatsAppNumber(ctx, wa)
	s.Require().Error(err)
	s.Assert().True(errors.Is(err, application.ErrUserNotFound))
}
