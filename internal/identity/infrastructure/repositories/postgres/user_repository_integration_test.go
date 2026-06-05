//go:build integration

package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
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

func (s *UserRepositorySuite) TestCA04d_ReanimateWithinWindowPreservesUUID() {
	ctx := context.Background()
	wa := s.newNumber("+5511988880004")
	repo := s.newRepo()

	email, eerr := valueobjects.NewEmail("original@example.com")
	s.Require().NoError(eerr)
	candidate := entities.New(wa, entities.WithEmail(email), entities.WithDisplayName("Original"))
	inserted, err := repo.UpsertByWhatsAppNumber(ctx, candidate, time.Now().UTC())
	s.Require().NoError(err)

	originalID := inserted.ID()
	s.Require().NoError(repo.MarkDeleted(ctx, originalID, time.Now().UTC()))

	deleted, err := repo.FindByWhatsAppNumberIncludingDeleted(ctx, wa)
	s.Require().NoError(err)
	s.Require().Equal(originalID, deleted.ID())
	s.Require().Equal(entities.StatusDeleted, deleted.Status())

	deleted.Reanimate(time.Now().UTC())
	reanimated, err := repo.Reanimate(ctx, deleted, time.Now().UTC())
	s.Require().NoError(err)

	s.Equal(originalID, reanimated.ID())
	s.Equal(entities.StatusActive, reanimated.Status())
	s.True(reanimated.DeletedAt().IsZero())
	s.Empty(reanimated.Email().String())
	s.Empty(reanimated.DisplayName())

	found, err := repo.FindByID(ctx, originalID)
	s.Require().NoError(err)
	s.Equal(originalID, found.ID())
	s.Equal(entities.StatusActive, found.Status())
}

func (s *UserRepositorySuite) TestCA04e_OutsideWindowUpsertCreatesNewUUIDAndPreservesDeleted() {
	ctx := context.Background()
	wa := s.newNumber("+5511988880005")
	repo := s.newRepo()

	candidate := entities.New(wa)
	inserted, err := repo.UpsertByWhatsAppNumber(ctx, candidate, time.Now().UTC())
	s.Require().NoError(err)
	originalID := inserted.ID()

	deletedAt := time.Now().UTC().Add(-31 * 24 * time.Hour)
	dbtx := s.mgr.DBTX(ctx)
	_, err = dbtx.ExecContext(ctx,
		`UPDATE users SET status = 'DELETED', deleted_at = $1, updated_at = $1 WHERE id = $2`,
		deletedAt, originalID,
	)
	s.Require().NoError(err)

	newCandidate := entities.New(wa)
	newInserted, err := repo.UpsertByWhatsAppNumber(ctx, newCandidate, time.Now().UTC())
	s.Require().NoError(err)

	s.NotEqual(originalID, newInserted.ID())
	s.Equal(entities.StatusActive, newInserted.Status())

	var oldStatus string
	var oldDeletedAt sql.NullTime
	row := dbtx.QueryRowContext(ctx,
		`SELECT status, deleted_at FROM users WHERE id = $1`,
		originalID,
	)
	s.Require().NoError(row.Scan(&oldStatus, &oldDeletedAt))
	s.Equal(string(entities.StatusDeleted), oldStatus)
	s.True(oldDeletedAt.Valid)
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

func (s *UserRepositorySuite) TestUniqueConstraintMapping_Email_ViaRepo() {
	ctx := context.Background()
	wa1 := s.newNumber("+5511988880020")
	wa2 := s.newNumber("+5511988880021")
	email, err := valueobjects.NewEmail("unique_test@example.com")
	s.Require().NoError(err)

	repo := s.newRepo()
	candidate1 := entities.New(wa1, entities.WithEmail(email))
	_, err = repo.UpsertByWhatsAppNumber(ctx, candidate1, time.Now().UTC())
	s.Require().NoError(err)

	candidate2 := entities.New(wa2, entities.WithEmail(email))
	_, err = repo.UpsertByWhatsAppNumber(ctx, candidate2, time.Now().UTC())
	s.Require().Error(err)
	s.True(errors.Is(err, application.ErrEmailInUse), "esperava sentinel ErrEmailInUse, obtive: %v", err)
}

func (s *UserRepositorySuite) TestUniqueConstraintMapping_WhatsApp_ViaReanimate() {
	ctx := context.Background()
	wa := s.newNumber("+5511988880030")
	repo := s.newRepo()

	candidate := entities.New(wa)
	original, err := repo.UpsertByWhatsAppNumber(ctx, candidate, time.Now().UTC())
	s.Require().NoError(err)
	originalID := original.ID()

	s.Require().NoError(repo.MarkDeleted(ctx, originalID, time.Now().UTC()))

	conflicting := entities.New(wa)
	conflictingInserted, err := repo.UpsertByWhatsAppNumber(ctx, conflicting, time.Now().UTC())
	s.Require().NoError(err)
	s.Require().NotEqual(originalID, conflictingInserted.ID())

	deleted, err := repo.FindByWhatsAppNumberIncludingDeleted(ctx, wa)
	s.Require().NoError(err)
	if deleted.ID() != originalID {
		dbtx := s.mgr.DBTX(ctx)
		row := dbtx.QueryRowContext(ctx,
			`SELECT id, whatsapp_number, email, display_name, status, created_at, updated_at, deleted_at
			   FROM users WHERE id = $1`, originalID)
		var id, whatsapp, status string
		var email, displayName sql.NullString
		var createdAt, updatedAt time.Time
		var delAt sql.NullTime
		s.Require().NoError(row.Scan(&id, &whatsapp, &email, &displayName, &status, &createdAt, &updatedAt, &delAt))
		hydrated, hydErr := entities.Hydrate(id, whatsapp, email.String, displayName.String, status, createdAt, updatedAt, delAt.Time)
		s.Require().NoError(hydErr)
		deleted = hydrated
	}

	deleted.Reanimate(time.Now().UTC())
	_, err = repo.Reanimate(ctx, deleted, time.Now().UTC())
	s.Require().Error(err)
	s.True(errors.Is(err, application.ErrWhatsAppNumberInUse), "esperava sentinel ErrWhatsAppNumberInUse, obtive: %v", err)
}

func (s *UserRepositorySuite) TestFindByWhatsAppNumber_NotFound() {
	ctx := context.Background()
	wa := s.newNumber("+5511988889999")
	repo := s.newRepo()

	_, err := repo.FindByWhatsAppNumber(ctx, wa)
	s.Require().Error(err)
	s.Assert().True(errors.Is(err, application.ErrUserNotFound))
}
