package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	application "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	interfacesmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces/mocks"
	usecasemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

type EstablishPrincipalSuite struct {
	suite.Suite
	ctx       context.Context
	obs       observability.Observability
	factory   *interfacesmocks.MockRepositoryFactory
	repo      *interfacesmocks.MockUserRepository
	idRepo    *interfacesmocks.MockUserIdentityRepository
	publisher *outboxmocks.Publisher
	uow       *usecasemocks.UnitOfWorkGeneric[EstablishResult]
}

func TestEstablishPrincipal(t *testing.T) {
	suite.Run(t, new(EstablishPrincipalSuite))
}

func (s *EstablishPrincipalSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.factory = interfacesmocks.NewMockRepositoryFactory(s.T())
	s.repo = interfacesmocks.NewMockUserRepository(s.T())
	s.idRepo = interfacesmocks.NewMockUserIdentityRepository(s.T())
	s.publisher = outboxmocks.NewPublisher(s.T())
	s.uow = usecasemocks.NewUnitOfWorkGeneric[EstablishResult](s.T())
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
		name         string
		args         args
		dependencies dependencies
		expect       func(auth.Principal, error)
	}{
		{
			name: "deve retornar Principal para usuario ativo",
			args: args{in: input.EstablishPrincipalInput{WhatsAppNumber: validWA}},
			dependencies: dependencies{
				factory: func() *interfacesmocks.MockRepositoryFactory {
					s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
					s.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(entities.UserIdentity{}, false, nil).Once()
					s.factory.EXPECT().UserRepository(mock.Anything).Return(s.repo).Once()
					s.repo.EXPECT().TryFindActiveByWhatsApp(mock.Anything, wa).Return(hydratedUser, true, nil).Once()
					s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
					s.idRepo.EXPECT().InsertIfAbsent(mock.Anything, mock.AnythingOfType("entities.UserIdentity")).Return(true, nil).Once()
					return s.factory
				}(),
				publisher: func() *outboxmocks.Publisher {
					s.publisher.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(ev outbox.Event) bool {
						return ev.AggregateUserID == "a0a0a0a0-0000-0000-0000-000000000001"
					})).Return(nil).Once()
					return s.publisher
				}(),
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
			dependencies: dependencies{
				factory: func() *interfacesmocks.MockRepositoryFactory {
					s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
					s.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(entities.UserIdentity{}, false, nil).Once()
					s.factory.EXPECT().UserRepository(mock.Anything).Return(s.repo).Once()
					s.repo.EXPECT().TryFindActiveByWhatsApp(mock.Anything, wa).Return(entities.User{}, false, nil).Once()
					return s.factory
				}(),
				publisher: func() *outboxmocks.Publisher {
					s.publisher.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(ev outbox.Event) bool {
						return ev.AggregateUserID == ""
					})).Return(nil).Once()
					return s.publisher
				}(),
			},
			expect: func(p auth.Principal, err error) {
				s.Require().ErrorIs(err, application.ErrUnknownUser)
				s.True(p.IsZero())
			},
		},
		{
			name: "deve propagar erro de banco de dados",
			args: args{in: input.EstablishPrincipalInput{WhatsAppNumber: validWA}},
			dependencies: dependencies{
				factory: func() *interfacesmocks.MockRepositoryFactory {
					s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
					s.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(entities.UserIdentity{}, false, nil).Once()
					s.factory.EXPECT().UserRepository(mock.Anything).Return(s.repo).Once()
					dbErr := errors.New("db unavailable")
					s.repo.EXPECT().TryFindActiveByWhatsApp(mock.Anything, wa).Return(entities.User{}, false, dbErr).Once()
					return s.factory
				}(),
				publisher: s.publisher,
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
			dependencies: dependencies{
				factory: func() *interfacesmocks.MockRepositoryFactory {
					s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
					s.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(entities.UserIdentity{}, false, nil).Once()
					s.factory.EXPECT().UserRepository(mock.Anything).Return(s.repo).Once()
					s.repo.EXPECT().TryFindActiveByWhatsApp(mock.Anything, wa).Return(hydratedUser, true, nil).Once()
					s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
					s.idRepo.EXPECT().InsertIfAbsent(mock.Anything, mock.AnythingOfType("entities.UserIdentity")).Return(true, nil).Once()
					return s.factory
				}(),
				publisher: func() *outboxmocks.Publisher {
					pubErr := errors.New("outbox unavailable")
					s.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(pubErr).Once()
					return s.publisher
				}(),
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
			dependencies: dependencies{
				factory: func() *interfacesmocks.MockRepositoryFactory {
					s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
					s.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(entities.UserIdentity{}, false, nil).Once()
					s.factory.EXPECT().UserRepository(mock.Anything).Return(s.repo).Once()
					s.repo.EXPECT().TryFindActiveByWhatsApp(mock.Anything, wa).Return(hydratedUser, true, nil).Once()
					s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
					s.idRepo.EXPECT().InsertIfAbsent(mock.Anything, mock.AnythingOfType("entities.UserIdentity")).Return(true, nil).Once()
					return s.factory
				}(),
				publisher: func() *outboxmocks.Publisher {
					s.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()
					return s.publisher
				}(),
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
			dependencies: dependencies{
				factory: func() *interfacesmocks.MockRepositoryFactory {
					s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
					s.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(entities.UserIdentity{}, false, nil).Once()
					s.factory.EXPECT().UserRepository(mock.Anything).Return(s.repo).Once()
					s.repo.EXPECT().TryFindActiveByWhatsApp(mock.Anything, wa).Return(hydratedUser, true, nil).Once()
					s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
					s.idRepo.EXPECT().InsertIfAbsent(mock.Anything, mock.AnythingOfType("entities.UserIdentity")).Return(true, nil).Once()
					return s.factory
				}(),
				publisher: func() *outboxmocks.Publisher {
					s.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()
					return s.publisher
				}(),
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
			dependencies: dependencies{
				factory: func() *interfacesmocks.MockRepositoryFactory {
					s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
					s.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(entities.UserIdentity{}, false, nil).Once()
					s.factory.EXPECT().UserRepository(mock.Anything).Return(s.repo).Once()
					s.repo.EXPECT().TryFindActiveByWhatsApp(mock.Anything, wa).Return(hydratedUser, true, nil).Once()
					s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
					s.idRepo.EXPECT().InsertIfAbsent(mock.Anything, mock.AnythingOfType("entities.UserIdentity")).Return(true, nil).Once()
					return s.factory
				}(),
				publisher: func() *outboxmocks.Publisher {
					s.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()
					return s.publisher
				}(),
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
			dependencies: dependencies{
				factory: func() *interfacesmocks.MockRepositoryFactory {
					s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
					s.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(entities.UserIdentity{}, false, nil).Once()
					s.factory.EXPECT().UserRepository(mock.Anything).Return(s.repo).Once()
					s.repo.EXPECT().TryFindActiveByWhatsApp(mock.Anything, wa).Return(hydratedUser, true, nil).Once()
					s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
					s.idRepo.EXPECT().InsertIfAbsent(mock.Anything, mock.AnythingOfType("entities.UserIdentity")).Return(true, nil).Once()
					return s.factory
				}(),
				publisher: func() *outboxmocks.Publisher {
					s.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()
					return s.publisher
				}(),
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
			dependencies: dependencies{
				factory:   s.factory,
				publisher: s.publisher,
			},
			expect: func(p auth.Principal, err error) {
				s.Require().Error(err)
				s.Contains(err.Error(), "parse request_id")
				s.True(p.IsZero())
			},
		},
		{
			name: "deve resolver via user_identities sem tocar users (path identity)",
			args: args{in: input.EstablishPrincipalInput{WhatsAppNumber: validWA}},
			dependencies: dependencies{
				factory: func() *interfacesmocks.MockRepositoryFactory {
					identity, errID := entities.NewUserIdentity(
						uuid.MustParse("b1b1b1b1-0000-0000-0000-000000000002"),
						uuid.MustParse("a0a0a0a0-0000-0000-0000-000000000001"),
						channelWA, externalIDWA, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
					)
					s.Require().NoError(errID)
					s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
					s.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(identity, true, nil).Once()
					return s.factory
				}(),
				publisher: func() *outboxmocks.Publisher {
					s.publisher.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(ev outbox.Event) bool {
						return ev.AggregateUserID == "a0a0a0a0-0000-0000-0000-000000000001"
					})).Return(nil).Once()
					return s.publisher
				}(),
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
			dependencies: dependencies{
				factory: func() *interfacesmocks.MockRepositoryFactory {
					s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
					s.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).
						Return(entities.UserIdentity{}, false, errors.New("identity db unavailable")).Once()
					return s.factory
				}(),
				publisher: s.publisher,
			},
			expect: func(p auth.Principal, err error) {
				s.Require().Error(err)
				s.Contains(err.Error(), "identity db unavailable")
				s.True(p.IsZero())
			},
		},
		{
			name: "path legacy: cria vinculo e emite resolve_path=legacy",
			args: args{in: input.EstablishPrincipalInput{WhatsAppNumber: validWA}},
			dependencies: dependencies{
				factory: func() *interfacesmocks.MockRepositoryFactory {
					s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
					s.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(entities.UserIdentity{}, false, nil).Once()
					s.factory.EXPECT().UserRepository(mock.Anything).Return(s.repo).Once()
					s.repo.EXPECT().TryFindActiveByWhatsApp(mock.Anything, wa).Return(hydratedUser, true, nil).Once()
					s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
					s.idRepo.EXPECT().InsertIfAbsent(mock.Anything, mock.AnythingOfType("entities.UserIdentity")).Return(true, nil).Once()
					return s.factory
				}(),
				publisher: func() *outboxmocks.Publisher {
					s.publisher.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(ev outbox.Event) bool {
						var decoded map[string]any
						if err := json.Unmarshal(ev.Payload, &decoded); err != nil {
							return false
						}
						return decoded["resolve_path"] == "legacy"
					})).Return(nil).Once()
					return s.publisher
				}(),
			},
			expect: func(p auth.Principal, err error) {
				s.Require().NoError(err)
				s.Require().False(p.IsZero())
			},
		},
		{
			name: "path legacy: vinculo ja existente e no-op sem falhar jornada",
			args: args{in: input.EstablishPrincipalInput{WhatsAppNumber: validWA}},
			dependencies: dependencies{
				factory: func() *interfacesmocks.MockRepositoryFactory {
					s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
					s.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(entities.UserIdentity{}, false, nil).Once()
					s.factory.EXPECT().UserRepository(mock.Anything).Return(s.repo).Once()
					s.repo.EXPECT().TryFindActiveByWhatsApp(mock.Anything, wa).Return(hydratedUser, true, nil).Once()
					s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
					s.idRepo.EXPECT().InsertIfAbsent(mock.Anything, mock.AnythingOfType("entities.UserIdentity")).
						Return(false, nil).Once()
					return s.factory
				}(),
				publisher: func() *outboxmocks.Publisher {
					s.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()
					return s.publisher
				}(),
			},
			expect: func(p auth.Principal, err error) {
				s.Require().NoError(err)
				s.Require().False(p.IsZero())
			},
		},
		{
			name: "path legacy: erro nao-sentinela no vinculo nao aborta a jornada",
			args: args{in: input.EstablishPrincipalInput{WhatsAppNumber: validWA}},
			dependencies: dependencies{
				factory: func() *interfacesmocks.MockRepositoryFactory {
					s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
					s.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(entities.UserIdentity{}, false, nil).Once()
					s.factory.EXPECT().UserRepository(mock.Anything).Return(s.repo).Once()
					s.repo.EXPECT().TryFindActiveByWhatsApp(mock.Anything, wa).Return(hydratedUser, true, nil).Once()
					s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
					s.idRepo.EXPECT().InsertIfAbsent(mock.Anything, mock.AnythingOfType("entities.UserIdentity")).
						Return(false, errors.New("db contention")).Once()
					return s.factory
				}(),
				publisher: func() *outboxmocks.Publisher {
					s.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()
					return s.publisher
				}(),
			},
			expect: func(p auth.Principal, err error) {
				s.Require().NoError(err)
				s.Require().False(p.IsZero())
			},
		},
		{
			name: "path identity: nao cria vinculo (Insert nao chamado)",
			args: args{in: input.EstablishPrincipalInput{WhatsAppNumber: validWA}},
			dependencies: dependencies{
				factory: func() *interfacesmocks.MockRepositoryFactory {
					identity, errID := entities.NewUserIdentity(
						uuid.MustParse("c2c2c2c2-0000-0000-0000-000000000003"),
						uuid.MustParse("a0a0a0a0-0000-0000-0000-000000000001"),
						channelWA, externalIDWA, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
					)
					s.Require().NoError(errID)
					s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
					s.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(identity, true, nil).Once()
					return s.factory
				}(),
				publisher: func() *outboxmocks.Publisher {
					s.publisher.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(ev outbox.Event) bool {
						var decoded map[string]any
						if err := json.Unmarshal(ev.Payload, &decoded); err != nil {
							return false
						}
						return decoded["resolve_path"] == "identity"
					})).Return(nil).Once()
					return s.publisher
				}(),
			},
			expect: func(p auth.Principal, err error) {
				s.Require().NoError(err)
				s.Require().False(p.IsZero())
			},
		},
		{
			name: "deve retornar ErrUnknownUser quando ambos caminhos retornam vazio",
			args: args{in: input.EstablishPrincipalInput{WhatsAppNumber: validWA}},
			dependencies: dependencies{
				factory: func() *interfacesmocks.MockRepositoryFactory {
					s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
					s.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(entities.UserIdentity{}, false, nil).Once()
					s.factory.EXPECT().UserRepository(mock.Anything).Return(s.repo).Once()
					s.repo.EXPECT().TryFindActiveByWhatsApp(mock.Anything, wa).Return(entities.User{}, false, nil).Once()
					return s.factory
				}(),
				publisher: func() *outboxmocks.Publisher {
					s.publisher.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(ev outbox.Event) bool {
						return ev.AggregateUserID == ""
					})).Return(nil).Once()
					return s.publisher
				}(),
			},
			expect: func(p auth.Principal, err error) {
				s.Require().ErrorIs(err, application.ErrUnknownUser)
				s.True(p.IsZero())
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sut := NewEstablishPrincipal(s.uow, scenario.dependencies.factory, scenario.dependencies.publisher, s.obs)
			p, err := sut.Execute(s.ctx, scenario.args.in)
			scenario.expect(p, err)
		})
	}
}

func (s *EstablishPrincipalSuite) TestExecute_FallbacksTraceIDWhenRequestIDMissing() {
	const validWA = "+5511987654321"

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

	s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
	s.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(entities.UserIdentity{}, false, nil).Once()
	s.factory.EXPECT().UserRepository(mock.Anything).Return(s.repo).Once()
	s.repo.EXPECT().TryFindActiveByWhatsApp(mock.Anything, wa).Return(hydratedUser, true, nil).Once()
	s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
	s.idRepo.EXPECT().InsertIfAbsent(mock.Anything, mock.AnythingOfType("entities.UserIdentity")).Return(true, nil).Once()

	var captured outbox.Event
	s.publisher.EXPECT().Publish(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, ev outbox.Event) error {
		captured = ev
		return nil
	}).Once()

	sut := NewEstablishPrincipal(s.uow, s.factory, s.publisher, s.obs)

	_, err = sut.Execute(s.ctx, input.EstablishPrincipalInput{WhatsAppNumber: validWA})
	s.Require().NoError(err)

	var payload map[string]any
	s.Require().NoError(json.Unmarshal(captured.Payload, &payload))
	s.Equal("fake-trace-id", payload["request_id"])
}
