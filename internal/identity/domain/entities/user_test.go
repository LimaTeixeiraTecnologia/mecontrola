package entities_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

func validWA(t *testing.T) valueobjects.WhatsAppNumber {
	t.Helper()
	wa, err := valueobjects.NewWhatsAppNumber("+5511987654321")
	require.NoError(t, err)
	return wa
}

func validEmail(t *testing.T) valueobjects.Email {
	t.Helper()
	em, err := valueobjects.NewEmail("test@example.com")
	require.NoError(t, err)
	return em
}

type UserSuite struct {
	suite.Suite
}

func TestUserSuite(t *testing.T) {
	suite.Run(t, new(UserSuite))
}

func (s *UserSuite) SetupTest() {}

func (s *UserSuite) TestNew() {
	type args struct {
		opts  []entities.Option
		count int
	}

	scenarios := []struct {
		name   string
		args   args
		expect func([]entities.User)
	}{
		{
			name: "deve criar usuario ativo com id preenchido e datas utc",
			args: args{count: 1},
			expect: func(users []entities.User) {
				s.Require().Len(users, 1)
				s.Equal(entities.StatusActive, users[0].Status())
				s.NotEmpty(users[0].ID())
				s.True(users[0].DeletedAt().IsZero())
				s.True(users[0].CreatedAt().Equal(users[0].CreatedAt().UTC()))
			},
		},
		{
			name: "deve aplicar opcoes ao criar usuario",
			args: args{
				opts: []entities.Option{
					entities.WithEmail(validEmail(s.T())),
					entities.WithDisplayName("Alice"),
				},
				count: 1,
			},
			expect: func(users []entities.User) {
				s.Require().Len(users, 1)
				s.Equal("test@example.com", users[0].Email().String())
				s.Equal("Alice", users[0].DisplayName())
			},
		},
		{
			name: "deve gerar ids distintos para usuarios diferentes",
			args: args{count: 2},
			expect: func(users []entities.User) {
				s.Require().Len(users, 2)
				s.NotEqual(users[0].ID(), users[1].ID())
			},
		},
		{
			name: "deve gerar id em formato uuid",
			args: args{count: 1},
			expect: func(users []entities.User) {
				s.Require().Len(users, 1)
				s.Len(users[0].ID(), 36)
				s.Regexp(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, users[0].ID())
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			users := make([]entities.User, 0, scenario.args.count)
			whatsApp := validWA(s.T())
			for range scenario.args.count {
				users = append(users, entities.New(whatsApp, scenario.args.opts...))
			}

			scenario.expect(users)
		})
	}
}

func (s *UserSuite) TestLifecycle() {
	type args struct {
		now time.Time
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func() entities.User
		act    func(*entities.User, time.Time)
		expect func(entities.User, time.Time)
	}{
		{
			name: "deve marcar usuario como deletado atualizando status e deleted at",
			args: args{now: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)},
			setup: func() entities.User {
				return entities.New(validWA(s.T()))
			},
			act: func(user *entities.User, now time.Time) {
				user.MarkDeleted(now)
			},
			expect: func(user entities.User, now time.Time) {
				s.Equal(entities.StatusDeleted, user.Status())
				s.Equal(now, user.DeletedAt())
				s.Equal(now, user.UpdatedAt())
			},
		},
		{
			name: "deve reativar usuario limpando pii e deleted at",
			args: args{now: time.Date(2026, 1, 20, 10, 0, 0, 0, time.UTC)},
			setup: func() entities.User {
				user := entities.New(
					validWA(s.T()),
					entities.WithEmail(validEmail(s.T())),
					entities.WithDisplayName("Alice"),
				)
				user.MarkDeleted(time.Date(2026, 1, 10, 10, 0, 0, 0, time.UTC))
				return user
			},
			act: func(user *entities.User, now time.Time) {
				user.Reanimate(now)
			},
			expect: func(user entities.User, now time.Time) {
				s.Equal(entities.StatusActive, user.Status())
				s.True(user.DeletedAt().IsZero())
				s.Empty(user.Email().String())
				s.Empty(user.DisplayName())
				s.Equal(now, user.UpdatedAt())
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			user := scenario.setup()
			scenario.act(&user, scenario.args.now)
			scenario.expect(user, scenario.args.now)
		})
	}
}

func (s *UserSuite) TestCanReanimate() {
	type args struct {
		elapsed   time.Duration
		deletedAt func() time.Time
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(entities.User, time.Time)
	}{
		{
			name: "deve permitir reanimacao exatamente na janela limite",
			args: args{elapsed: domain.ReanimationWindow},
			expect: func(user entities.User, now time.Time) {
				s.True(user.CanReanimate(now))
			},
		},
		{
			name: "deve permitir reanimacao antes de expirar a janela",
			args: args{elapsed: domain.ReanimationWindow - time.Nanosecond},
			expect: func(user entities.User, now time.Time) {
				s.True(user.CanReanimate(now))
			},
		},
		{
			name: "deve negar reanimacao apos expirar a janela",
			args: args{elapsed: domain.ReanimationWindow + time.Nanosecond},
			expect: func(user entities.User, now time.Time) {
				s.False(user.CanReanimate(now))
			},
		},
		{
			name: "deve negar reanimacao quando deleted at for zero",
			args: args{
				deletedAt: func() time.Time {
					return time.Time{}
				},
			},
			expect: func(user entities.User, now time.Time) {
				s.False(user.CanReanimate(now))
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			whatsApp := validWA(s.T())
			user := entities.New(whatsApp)
			base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			now := base.Add(scenario.args.elapsed)

			if scenario.args.deletedAt == nil {
				user.MarkDeleted(base)
				scenario.expect(user, now)
				return
			}

			hydrated, err := entities.Hydrate(
				user.ID(),
				whatsApp.String(),
				"",
				"",
				string(entities.StatusDeleted),
				user.CreatedAt(),
				user.UpdatedAt(),
				scenario.args.deletedAt(),
			)
			s.Require().NoError(err)

			scenario.expect(hydrated, base.Add(domain.ReanimationWindow))
		})
	}
}

func (s *UserSuite) TestSetDisplayNameIfEmpty() {
	type args struct {
		initial string
		next    string
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(entities.User)
	}{
		{
			name: "deve preencher display name quando estiver vazio",
			args: args{next: "Alice"},
			expect: func(user entities.User) {
				s.Equal("Alice", user.DisplayName())
			},
		},
		{
			name: "deve preservar display name quando ja existir valor",
			args: args{initial: "Original", next: "Novo Nome"},
			expect: func(user entities.User) {
				s.Equal("Original", user.DisplayName())
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			options := make([]entities.Option, 0, 1)
			if scenario.args.initial != "" {
				options = append(options, entities.WithDisplayName(scenario.args.initial))
			}

			user := entities.New(validWA(s.T()), options...)
			user.SetDisplayNameIfEmpty(scenario.args.next)

			scenario.expect(user)
		})
	}
}

func (s *UserSuite) TestSetEmailIfEmpty() {
	type args struct {
		initial string
		next    string
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(entities.User)
	}{
		{
			name: "deve preencher email quando estiver vazio",
			args: args{next: "test@example.com"},
			expect: func(user entities.User) {
				s.Equal("test@example.com", user.Email().String())
			},
		},
		{
			name: "deve preservar email quando ja existir valor",
			args: args{initial: "test@example.com", next: "other@example.com"},
			expect: func(user entities.User) {
				s.Equal("test@example.com", user.Email().String())
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			options := make([]entities.Option, 0, 1)
			if scenario.args.initial != "" {
				options = append(options, entities.WithEmail(validEmail(s.T())))
			}

			user := entities.New(validWA(s.T()), options...)
			nextEmail, err := valueobjects.NewEmail(scenario.args.next)
			s.Require().NoError(err)

			user.SetEmailIfEmpty(nextEmail)

			scenario.expect(user)
		})
	}
}

func (s *UserSuite) TestHydrate() {
	type args struct {
		id          string
		whatsApp    string
		email       string
		displayName string
		status      string
		deletedAt   time.Time
	}

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	scenarios := []struct {
		name   string
		args   args
		expect func(entities.User, error)
	}{
		{
			name: "deve reconstruir usuario sem gerar novo id",
			args: args{
				id:          "fixed-uuid-1234",
				whatsApp:    validWA(s.T()).String(),
				email:       validEmail(s.T()).String(),
				displayName: "Alice",
				status:      string(entities.StatusActive),
			},
			expect: func(user entities.User, err error) {
				s.Require().NoError(err)
				s.Equal("fixed-uuid-1234", user.ID())
				s.Equal("Alice", user.DisplayName())
				s.Equal(entities.StatusActive, user.Status())
				s.True(user.DeletedAt().IsZero())
			},
		},
		{
			name: "deve aceitar email vazio sem erro",
			args: args{
				id:       "id-1",
				whatsApp: validWA(s.T()).String(),
				status:   string(entities.StatusActive),
			},
			expect: func(user entities.User, err error) {
				s.Require().NoError(err)
				s.Empty(user.Email().String())
				s.Empty(user.DisplayName())
			},
		},
		{
			name: "deve retornar erro para whatsapp invalido",
			args: args{
				id:       "id-1",
				whatsApp: "not-a-number",
				status:   string(entities.StatusActive),
			},
			expect: func(user entities.User, err error) {
				s.Require().Error(err)
				s.Equal(entities.User{}, user)
			},
		},
		{
			name: "deve retornar erro para email invalido",
			args: args{
				id:       "id-1",
				whatsApp: validWA(s.T()).String(),
				email:    "not-an-email",
				status:   string(entities.StatusActive),
			},
			expect: func(user entities.User, err error) {
				s.Require().Error(err)
				s.Equal(entities.User{}, user)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			user, err := entities.Hydrate(
				scenario.args.id,
				scenario.args.whatsApp,
				scenario.args.email,
				scenario.args.displayName,
				scenario.args.status,
				now,
				now,
				scenario.args.deletedAt,
			)

			scenario.expect(user, err)
		})
	}
}
