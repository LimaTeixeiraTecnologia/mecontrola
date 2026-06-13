package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	application "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	interfacesmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	usecasemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

type EstablishPrincipalSuite struct {
	suite.Suite
	ctx context.Context
}

func TestEstablishPrincipal(t *testing.T) {
	suite.Run(t, new(EstablishPrincipalSuite))
}

func (s *EstablishPrincipalSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *EstablishPrincipalSuite) mustWhatsApp(raw string) valueobjects.WhatsAppNumber {
	wa, err := valueobjects.NewWhatsAppNumber(raw)
	s.Require().NoError(err)
	return wa
}

func (s *EstablishPrincipalSuite) TestExecute() {
	const validWA = "+5511987654321"

	type args struct {
		in input.EstablishPrincipalInput
	}

	type dependencies struct {
		factory   *interfacesmocks.MockRepositoryFactory
		repo      *interfacesmocks.MockUserRepository
		publisher *outboxmocks.Publisher
	}

	wa := s.mustWhatsApp(validWA)

	hydratedUser, err := entities.Hydrate(
		"a0a0a0a0-0000-0000-0000-000000000001",
		wa.String(),
		"", "", "ACTIVE",
		time.Time{}, time.Time{}, time.Time{},
	)
	s.Require().NoError(err)

	scenarios := []struct {
		name   string
		args   args
		setup  func(dependencies)
		expect func(auth.Principal, error)
	}{
		{
			name: "deve retornar Principal para usuario ativo",
			args: args{in: input.EstablishPrincipalInput{WhatsAppNumber: validWA}},
			setup: func(deps dependencies) {
				deps.factory.EXPECT().UserRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().TryFindActiveByWhatsApp(mock.Anything, wa).Return(hydratedUser, true, nil).Once()
				deps.publisher.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(ev outbox.Event) bool {
					return ev.AggregateUserID == "a0a0a0a0-0000-0000-0000-000000000001"
				})).Return(nil).Once()
			},
			expect: func(p auth.Principal, err error) {
				s.Require().NoError(err)
				s.Require().False(p.IsZero())
				s.Equal(auth.SourceWhatsApp, p.Source)
			},
		},
		{
			name: "deve retornar ErrUnknownUser e publicar unknown_user para usuario inexistente",
			args: args{in: input.EstablishPrincipalInput{WhatsAppNumber: validWA}},
			setup: func(deps dependencies) {
				deps.factory.EXPECT().UserRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().TryFindActiveByWhatsApp(mock.Anything, wa).Return(entities.User{}, false, nil).Once()
				deps.publisher.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(ev outbox.Event) bool {
					return ev.AggregateUserID == ""
				})).Return(nil).Once()
			},
			expect: func(p auth.Principal, err error) {
				s.Require().ErrorIs(err, application.ErrUnknownUser)
				s.True(p.IsZero())
			},
		},
		{
			name: "deve propagar erro de banco de dados",
			args: args{in: input.EstablishPrincipalInput{WhatsAppNumber: validWA}},
			setup: func(deps dependencies) {
				deps.factory.EXPECT().UserRepository(mock.Anything).Return(deps.repo).Once()
				dbErr := errors.New("db unavailable")
				deps.repo.EXPECT().TryFindActiveByWhatsApp(mock.Anything, wa).Return(entities.User{}, false, dbErr).Once()
			},
			expect: func(p auth.Principal, err error) {
				s.Require().Error(err)
				s.Contains(err.Error(), "db unavailable")
				s.True(p.IsZero())
			},
		},
		{
			name: "deve propagar erro de outbox e nao retornar Principal (rollback observavel via erro)",
			args: args{in: input.EstablishPrincipalInput{WhatsAppNumber: validWA}},
			setup: func(deps dependencies) {
				deps.factory.EXPECT().UserRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().TryFindActiveByWhatsApp(mock.Anything, wa).Return(hydratedUser, true, nil).Once()
				pubErr := errors.New("outbox unavailable")
				deps.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(pubErr).Once()
			},
			expect: func(p auth.Principal, err error) {
				s.Require().Error(err)
				s.Contains(err.Error(), "outbox unavailable")
				s.True(p.IsZero())
			},
		},
		{
			name: "deve persistir request_id e client_ip quando fornecidos",
			args: args{in: input.EstablishPrincipalInput{
				WhatsAppNumber: validWA,
				RequestID:      "req-abc-123",
				ClientIPRaw:    "1.2.3.4",
			}},
			setup: func(deps dependencies) {
				deps.factory.EXPECT().UserRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().TryFindActiveByWhatsApp(mock.Anything, wa).Return(hydratedUser, true, nil).Once()
				deps.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()
			},
			expect: func(p auth.Principal, err error) {
				s.Require().NoError(err)
				s.Require().False(p.IsZero())
			},
		},
		{
			name: "deve aceitar client_ip vazio (NULL semantico) sem erro",
			args: args{in: input.EstablishPrincipalInput{
				WhatsAppNumber: validWA,
				RequestID:      "req-xyz-999",
				ClientIPRaw:    "",
			}},
			setup: func(deps dependencies) {
				deps.factory.EXPECT().UserRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().TryFindActiveByWhatsApp(mock.Anything, wa).Return(hydratedUser, true, nil).Once()
				deps.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()
			},
			expect: func(p auth.Principal, err error) {
				s.Require().NoError(err)
				s.Require().False(p.IsZero())
			},
		},
		{
			name: "deve aceitar input sem request_id e sem client_ip (compat retroativa)",
			args: args{in: input.EstablishPrincipalInput{
				WhatsAppNumber: validWA,
			}},
			setup: func(deps dependencies) {
				deps.factory.EXPECT().UserRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().TryFindActiveByWhatsApp(mock.Anything, wa).Return(hydratedUser, true, nil).Once()
				deps.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()
			},
			expect: func(p auth.Principal, err error) {
				s.Require().NoError(err)
				s.Require().False(p.IsZero())
			},
		},
		{
			name: "deve retornar erro para client_ip invalido",
			args: args{in: input.EstablishPrincipalInput{
				WhatsAppNumber: validWA,
				ClientIPRaw:    "not-an-ip",
			}},
			setup: func(deps dependencies) {},
			expect: func(p auth.Principal, err error) {
				s.Require().Error(err)
				s.Contains(err.Error(), "parse client_ip")
				s.True(p.IsZero())
			},
		},
		{
			name: "deve retornar erro para request_id invalido (muito longo)",
			args: args{in: input.EstablishPrincipalInput{
				WhatsAppNumber: validWA,
				RequestID:      string(make([]byte, 129)),
			}},
			setup: func(deps dependencies) {},
			expect: func(p auth.Principal, err error) {
				s.Require().Error(err)
				s.Contains(err.Error(), "parse request_id")
				s.True(p.IsZero())
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			deps := dependencies{
				factory:   interfacesmocks.NewMockRepositoryFactory(s.T()),
				repo:      interfacesmocks.NewMockUserRepository(s.T()),
				publisher: outboxmocks.NewPublisher(s.T()),
			}
			scenario.setup(deps)

			uow := usecasemocks.NewUnitOfWorkGeneric[usecases.EstablishResult](s.T())
			sut := usecases.NewEstablishPrincipal(uow, deps.factory, deps.publisher, noop.NewProvider())
			p, err := sut.Execute(s.ctx, scenario.args.in)

			scenario.expect(p, err)
		})
	}
}
