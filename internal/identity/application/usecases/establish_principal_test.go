package usecases_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
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
		idRepo    *interfacesmocks.MockUserIdentityRepository
		publisher *outboxmocks.Publisher
	}

	wa := s.mustWhatsApp(validWA)
	channelWA := valueobjects.ChannelWhatsApp()
	externalIDWA, errExt := valueobjects.NewExternalID(channelWA, wa.String())
	s.Require().NoError(errExt)

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
				deps.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(deps.idRepo).Once()
				deps.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(entities.UserIdentity{}, false, nil).Once()
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
				deps.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(deps.idRepo).Once()
				deps.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(entities.UserIdentity{}, false, nil).Once()
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
				deps.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(deps.idRepo).Once()
				deps.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(entities.UserIdentity{}, false, nil).Once()
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
				deps.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(deps.idRepo).Once()
				deps.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(entities.UserIdentity{}, false, nil).Once()
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
				deps.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(deps.idRepo).Once()
				deps.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(entities.UserIdentity{}, false, nil).Once()
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
				deps.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(deps.idRepo).Once()
				deps.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(entities.UserIdentity{}, false, nil).Once()
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
				deps.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(deps.idRepo).Once()
				deps.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(entities.UserIdentity{}, false, nil).Once()
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
			name: "deve degradar client_ip invalido para vazio sem erro",
			args: args{in: input.EstablishPrincipalInput{
				WhatsAppNumber: validWA,
				ClientIPRaw:    "not-an-ip",
			}},
			setup: func(deps dependencies) {
				deps.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(deps.idRepo).Once()
				deps.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(entities.UserIdentity{}, false, nil).Once()
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
		{
			name: "deve resolver via user_identities sem tocar users (path identity)",
			args: args{in: input.EstablishPrincipalInput{WhatsAppNumber: validWA}},
			setup: func(deps dependencies) {
				identity, errID := entities.NewUserIdentity(
					uuid.MustParse("b1b1b1b1-0000-0000-0000-000000000002"),
					uuid.MustParse("a0a0a0a0-0000-0000-0000-000000000001"),
					channelWA, externalIDWA, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				)
				s.Require().NoError(errID)
				deps.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(deps.idRepo).Once()
				deps.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(identity, true, nil).Once()
				deps.publisher.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(ev outbox.Event) bool {
					return ev.AggregateUserID == "a0a0a0a0-0000-0000-0000-000000000001"
				})).Return(nil).Once()
			},
			expect: func(p auth.Principal, err error) {
				s.Require().NoError(err)
				s.Require().False(p.IsZero())
				s.Equal(auth.SourceWhatsApp, p.Source)
				s.Equal("a0a0a0a0-0000-0000-0000-000000000001", p.UserID.String())
			},
		},
		{
			name: "deve propagar erro do lookup identity (db indisponivel)",
			args: args{in: input.EstablishPrincipalInput{WhatsAppNumber: validWA}},
			setup: func(deps dependencies) {
				deps.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(deps.idRepo).Once()
				deps.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).
					Return(entities.UserIdentity{}, false, errors.New("identity db unavailable")).Once()
			},
			expect: func(p auth.Principal, err error) {
				s.Require().Error(err)
				s.Contains(err.Error(), "identity db unavailable")
				s.True(p.IsZero())
			},
		},
		{
			name: "deve retornar ErrUnknownUser quando ambos caminhos retornam vazio",
			args: args{in: input.EstablishPrincipalInput{WhatsAppNumber: validWA}},
			setup: func(deps dependencies) {
				deps.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(deps.idRepo).Once()
				deps.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(entities.UserIdentity{}, false, nil).Once()
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
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			deps := dependencies{
				factory:   interfacesmocks.NewMockRepositoryFactory(s.T()),
				repo:      interfacesmocks.NewMockUserRepository(s.T()),
				idRepo:    interfacesmocks.NewMockUserIdentityRepository(s.T()),
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

func (s *EstablishPrincipalSuite) TestExecute_FallbacksTraceIDWhenRequestIDMissing() {
	const validWA = "+5511987654321"

	factory := interfacesmocks.NewMockRepositoryFactory(s.T())
	repo := interfacesmocks.NewMockUserRepository(s.T())
	idRepo := interfacesmocks.NewMockUserIdentityRepository(s.T())
	publisher := outboxmocks.NewPublisher(s.T())
	o11y := fake.NewProvider()

	wa := s.mustWhatsApp(validWA)
	channelWA := valueobjects.ChannelWhatsApp()
	externalIDWA, errExt := valueobjects.NewExternalID(channelWA, wa.String())
	s.Require().NoError(errExt)
	hydratedUser, err := entities.Hydrate(
		"a0a0a0a0-0000-0000-0000-000000000001",
		wa.String(),
		"", "", "ACTIVE",
		time.Time{}, time.Time{}, time.Time{},
	)
	s.Require().NoError(err)

	factory.EXPECT().UserIdentityRepository(mock.Anything).Return(idRepo).Once()
	idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(entities.UserIdentity{}, false, nil).Once()
	factory.EXPECT().UserRepository(mock.Anything).Return(repo).Once()
	repo.EXPECT().TryFindActiveByWhatsApp(mock.Anything, wa).Return(hydratedUser, true, nil).Once()

	var captured outbox.Event
	publisher.EXPECT().Publish(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, ev outbox.Event) error {
		captured = ev
		return nil
	}).Once()

	uow := usecasemocks.NewUnitOfWorkGeneric[usecases.EstablishResult](s.T())
	sut := usecases.NewEstablishPrincipal(uow, factory, publisher, o11y)

	_, err = sut.Execute(s.ctx, input.EstablishPrincipalInput{WhatsAppNumber: validWA})
	s.Require().NoError(err)

	var payload map[string]any
	s.Require().NoError(json.Unmarshal(captured.Payload, &payload))
	s.Equal("fake-trace-id", payload["request_id"])
}
