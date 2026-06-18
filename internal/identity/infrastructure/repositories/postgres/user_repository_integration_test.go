//go:build integration

package postgres_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
)

type UserRepositoryIntegrationSuite struct {
	suite.Suite
	ctx     context.Context
	db      *sqlx.DB
	o11y    *noop.Provider
	factory interfaces.RepositoryFactory
}

func TestUserRepositoryIntegrationSuite(t *testing.T) {
	suite.Run(t, new(UserRepositoryIntegrationSuite))
}

func (s *UserRepositoryIntegrationSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *UserRepositoryIntegrationSuite) SetupSuite() {
	db, _ := setupTestDB(s.T())
	s.db = db
	s.o11y = noop.NewProvider()
	s.factory = repositories.NewRepositoryFactory(s.o11y)
}

func (s *UserRepositoryIntegrationSuite) newRepo() interfaces.UserRepository {
	return s.factory.UserRepository(s.db)
}

func (s *UserRepositoryIntegrationSuite) newNumber(number string) valueobjects.WhatsAppNumber {
	whatsApp, err := valueobjects.NewWhatsAppNumber(number)
	s.Require().NoError(err)
	return whatsApp
}

func (s *UserRepositoryIntegrationSuite) newEmail(raw string) valueobjects.Email {
	email, err := valueobjects.NewEmail(raw)
	s.Require().NoError(err)
	return email
}

func (s *UserRepositoryIntegrationSuite) TestUpsertByWhatsAppNumber() {
	type args struct {
		firstWhatsApp  string
		secondWhatsApp string
		email          string
		displayName    string
		secondName     string
	}

	type result struct {
		first entities.User
		next  entities.User
		err   error
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(interfaces.UserRepository, args) result
		expect func(result)
	}{
		{
			name: "deve inserir na primeira chamada e atualizar updated at na segunda",
			args: args{
				firstWhatsApp: "+5511988880001",
				displayName:   "Alice",
			},
			setup: func(repo interfaces.UserRepository, args args) result {
				whatsApp := s.newNumber(args.firstWhatsApp)
				firstNow := time.Now().UTC()
				first, err := repo.UpsertByWhatsAppNumber(s.ctx, entities.New(whatsApp, entities.WithDisplayName(args.displayName)), firstNow)
				if err != nil {
					return result{err: err}
				}

				secondNow := firstNow.Add(time.Second)
				next, err := repo.UpsertByWhatsAppNumber(s.ctx, first, secondNow)
				return result{first: first, next: next, err: err}
			},
			expect: func(out result) {
				s.Require().NoError(out.err)
				s.Equal("Alice", out.first.DisplayName())
				s.Equal(out.first.ID(), out.next.ID())
				s.True(out.next.UpdatedAt().Equal(out.first.UpdatedAt().Add(time.Second)) || out.next.UpdatedAt().After(out.first.UpdatedAt()))
			},
		},
		{
			name: "deve preservar primeiro display name ao receber nova tentativa",
			args: args{
				firstWhatsApp: "+5511988880006",
				displayName:   "FirstName",
				secondName:    "SecondName",
			},
			setup: func(repo interfaces.UserRepository, args args) result {
				whatsApp := s.newNumber(args.firstWhatsApp)
				first, err := repo.UpsertByWhatsAppNumber(s.ctx, entities.New(whatsApp, entities.WithDisplayName(args.displayName)), time.Now().UTC())
				if err != nil {
					return result{err: err}
				}

				next, err := repo.UpsertByWhatsAppNumber(
					s.ctx,
					entities.New(whatsApp, entities.WithDisplayName(args.secondName)),
					time.Now().UTC().Add(time.Second),
				)
				return result{first: first, next: next, err: err}
			},
			expect: func(out result) {
				s.Require().NoError(out.err)
				s.Equal("FirstName", out.next.DisplayName())
			},
		},
		{
			name: "deve tocar updated at mesmo sem alterar outros campos",
			args: args{
				firstWhatsApp: "+5511988880007",
				displayName:   "TouchTest",
			},
			setup: func(repo interfaces.UserRepository, args args) result {
				whatsApp := s.newNumber(args.firstWhatsApp)
				first, err := repo.UpsertByWhatsAppNumber(s.ctx, entities.New(whatsApp, entities.WithDisplayName(args.displayName)), time.Now().UTC())
				if err != nil {
					return result{err: err}
				}

				secondNow := first.UpdatedAt().Add(time.Second)
				next, err := repo.UpsertByWhatsAppNumber(s.ctx, first, secondNow)
				return result{first: first, next: next, err: err}
			},
			expect: func(out result) {
				s.Require().NoError(out.err)
				s.True(out.next.UpdatedAt().Equal(out.first.UpdatedAt().Add(time.Second)) || out.next.UpdatedAt().After(out.first.UpdatedAt()))
			},
		},
		{
			name: "deve criar novo uuid fora da janela e preservar o registro deletado",
			args: args{
				firstWhatsApp: "+5511988880005",
			},
			setup: func(repo interfaces.UserRepository, args args) result {
				whatsApp := s.newNumber(args.firstWhatsApp)
				first, err := repo.UpsertByWhatsAppNumber(s.ctx, entities.New(whatsApp), time.Now().UTC())
				if err != nil {
					return result{err: err}
				}

				deletedAt := time.Now().UTC().Add(-31 * 24 * time.Hour)
				_, err = s.db.ExecContext(
					s.ctx,
					`UPDATE users SET status = 'DELETED', deleted_at = $1, updated_at = $1 WHERE id = $2`,
					deletedAt,
					first.ID(),
				)
				if err != nil {
					return result{err: err}
				}

				next, err := repo.UpsertByWhatsAppNumber(s.ctx, entities.New(whatsApp), time.Now().UTC())
				return result{first: first, next: next, err: err}
			},
			expect: func(out result) {
				s.Require().NoError(out.err)
				s.NotEqual(out.first.ID(), out.next.ID())
				s.Equal(entities.StatusActive, out.next.Status())

				var oldStatus string
				var oldDeletedAt sql.NullTime
				err := s.db.QueryRowContext(
					s.ctx,
					`SELECT status, deleted_at FROM users WHERE id = $1`,
					out.first.ID(),
				).Scan(&oldStatus, &oldDeletedAt)
				s.Require().NoError(err)
				s.Equal(string(entities.StatusDeleted), oldStatus)
				s.True(oldDeletedAt.Valid)
			},
		},
		{
			name: "deve mapear violacao de unicidade de email",
			args: args{
				firstWhatsApp:  "+5511988880020",
				secondWhatsApp: "+5511988880021",
				email:          "unique_test@example.com",
			},
			setup: func(repo interfaces.UserRepository, args args) result {
				email := s.newEmail(args.email)
				first, err := repo.UpsertByWhatsAppNumber(
					s.ctx,
					entities.New(s.newNumber(args.firstWhatsApp), entities.WithEmail(email)),
					time.Now().UTC(),
				)
				if err != nil {
					return result{err: err}
				}

				_, err = repo.UpsertByWhatsAppNumber(
					s.ctx,
					entities.New(s.newNumber(args.secondWhatsApp), entities.WithEmail(email)),
					time.Now().UTC(),
				)
				return result{first: first, err: err}
			},
			expect: func(out result) {
				s.Require().Error(out.err)
				s.ErrorIs(out.err, application.ErrEmailInUse)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			repo := s.newRepo()
			out := scenario.setup(repo, scenario.args)
			scenario.expect(out)
		})
	}
}

func (s *UserRepositoryIntegrationSuite) TestLifecycleAndQueries() {
	type args struct {
		whatsApp    string
		email       string
		displayName string
	}

	type result struct {
		user      entities.User
		found     entities.User
		err       error
		errSecond error
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(interfaces.UserRepository, args) result
		expect func(result)
	}{
		{
			name: "deve retornar user not found ao buscar id apos mark deleted",
			args: args{
				whatsApp: "+5511988880002",
			},
			setup: func(repo interfaces.UserRepository, args args) result {
				user, err := repo.UpsertByWhatsAppNumber(s.ctx, entities.New(s.newNumber(args.whatsApp)), time.Now().UTC())
				if err != nil {
					return result{err: err}
				}

				err = repo.MarkDeleted(s.ctx, user.ID(), time.Now().UTC())
				if err != nil {
					return result{user: user, err: err}
				}

				_, errSecond := repo.FindByID(s.ctx, user.ID())
				return result{user: user, errSecond: errSecond}
			},
			expect: func(out result) {
				s.Require().NoError(out.err)
				s.Require().Error(out.errSecond)
				s.ErrorIs(out.errSecond, application.ErrUserNotFound)
			},
		},
		{
			name: "deve reanimar usuario dentro da janela preservando uuid",
			args: args{
				whatsApp:    "+5511988880004",
				email:       "original@example.com",
				displayName: "Original",
			},
			setup: func(repo interfaces.UserRepository, args args) result {
				inserted, err := repo.UpsertByWhatsAppNumber(
					s.ctx,
					entities.New(
						s.newNumber(args.whatsApp),
						entities.WithEmail(s.newEmail(args.email)),
						entities.WithDisplayName(args.displayName),
					),
					time.Now().UTC(),
				)
				if err != nil {
					return result{err: err}
				}

				err = repo.MarkDeleted(s.ctx, inserted.ID(), time.Now().UTC())
				if err != nil {
					return result{user: inserted, err: err}
				}

				deleted, err := repo.FindByWhatsAppNumberIncludingDeleted(s.ctx, s.newNumber(args.whatsApp))
				if err != nil {
					return result{user: inserted, err: err}
				}

				deleted.Reanimate(time.Now().UTC())
				found, err := repo.Reanimate(s.ctx, deleted, time.Now().UTC())
				return result{user: inserted, found: found, err: err}
			},
			expect: func(out result) {
				s.Require().NoError(out.err)
				s.Equal(out.user.ID(), out.found.ID())
				s.Equal(entities.StatusActive, out.found.Status())
				s.True(out.found.DeletedAt().IsZero())
				s.Empty(out.found.Email().String())
				s.Empty(out.found.DisplayName())

				foundByID, err := s.newRepo().FindByID(s.ctx, out.user.ID())
				s.Require().NoError(err)
				s.Equal(out.user.ID(), foundByID.ID())
				s.Equal(entities.StatusActive, foundByID.Status())
			},
		},
		{
			name: "deve retornar user not found ao buscar whatsapp inexistente",
			args: args{
				whatsApp: "+5511988889999",
			},
			setup: func(repo interfaces.UserRepository, args args) result {
				_, err := repo.FindByWhatsAppNumber(s.ctx, s.newNumber(args.whatsApp))
				return result{err: err}
			},
			expect: func(out result) {
				s.Require().Error(out.err)
				s.ErrorIs(out.err, application.ErrUserNotFound)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			repo := s.newRepo()
			out := scenario.setup(repo, scenario.args)
			scenario.expect(out)
		})
	}
}

func (s *UserRepositoryIntegrationSuite) TestConstraintsAndHistory() {
	type args struct {
		whatsApp       string
		conflictNumber string
	}

	type result struct {
		user      entities.User
		err       error
		errSecond error
		count     int
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(interfaces.UserRepository, args) result
		expect func(result)
	}{
		{
			name: "deve persistir historico de whatsapp",
			args: args{
				whatsApp: "+5511988880003",
			},
			setup: func(repo interfaces.UserRepository, args args) result {
				user, err := repo.UpsertByWhatsAppNumber(s.ctx, entities.New(s.newNumber(args.whatsApp)), time.Now().UTC())
				if err != nil {
					return result{err: err}
				}

				entry := interfaces.WhatsAppHistoryEntry{
					ID:       entities.NewID(),
					UserID:   user.ID(),
					Number:   args.whatsApp,
					Active:   true,
					LinkedAt: time.Now().UTC(),
					Reason:   "initial_link",
				}
				err = repo.AppendWhatsAppHistory(s.ctx, user.ID(), entry)
				if err != nil {
					return result{user: user, err: err}
				}

				var count int
				err = s.db.QueryRowContext(
					s.ctx,
					`SELECT COUNT(*) FROM user_whatsapp_history WHERE id = $1`,
					entry.ID,
				).Scan(&count)
				return result{user: user, err: err, count: count}
			},
			expect: func(out result) {
				s.Require().NoError(out.err)
				s.Equal(1, out.count)
			},
		},
		{
			name: "deve rejeitar usuario deletado sem deleted at",
			setup: func(repo interfaces.UserRepository, args args) result {
				_, err := s.db.ExecContext(
					s.ctx,
					`INSERT INTO users (id, whatsapp_number, status, created_at, updated_at, deleted_at)
					 VALUES (gen_random_uuid(), '+5511988880099', 'DELETED', now(), now(), NULL)`,
				)
				return result{err: err}
			},
			expect: func(out result) {
				s.Require().Error(out.err)
			},
		},
		{
			name: "deve mapear violacao de unicidade de whatsapp ao reanimar",
			args: args{
				whatsApp:       "+5511988880030",
				conflictNumber: "+5511988880030",
			},
			setup: func(repo interfaces.UserRepository, args args) result {
				whatsApp := s.newNumber(args.whatsApp)
				original, err := repo.UpsertByWhatsAppNumber(s.ctx, entities.New(whatsApp), time.Now().UTC())
				if err != nil {
					return result{err: err}
				}

				err = repo.MarkDeleted(s.ctx, original.ID(), time.Now().UTC())
				if err != nil {
					return result{user: original, err: err}
				}

				user, err := repo.FindByWhatsAppNumberIncludingDeleted(s.ctx, whatsApp)
				if err != nil {
					return result{user: original, err: err}
				}

				_, err = repo.UpsertByWhatsAppNumber(s.ctx, entities.New(s.newNumber(args.conflictNumber)), time.Now().UTC())
				if err != nil {
					return result{user: user, err: err}
				}

				user.Reanimate(time.Now().UTC())
				_, errSecond := repo.Reanimate(s.ctx, user, time.Now().UTC())
				return result{user: user, errSecond: errSecond}
			},
			expect: func(out result) {
				s.Require().NoError(out.err)
				s.Require().Error(out.errSecond)
				s.ErrorIs(out.errSecond, application.ErrWhatsAppNumberInUse)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			repo := s.newRepo()
			out := scenario.setup(repo, scenario.args)
			scenario.expect(out)
		})
	}
}
