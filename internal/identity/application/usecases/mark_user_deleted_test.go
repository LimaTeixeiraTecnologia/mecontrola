package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	application "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	interfacesmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces/mocks"
	usecasemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

type MarkUserDeletedSuite struct {
	suite.Suite
	ctx           context.Context
	obs           observability.Observability
	uowMock       *usecasemocks.UnitOfWorkVoid
	factoryMock   *interfacesmocks.MockRepositoryFactory
	repoMock      *interfacesmocks.MockUserRepository
	publisherMock *outboxmocks.Publisher
}

func TestMarkUserDeleted(t *testing.T) {
	suite.Run(t, new(MarkUserDeletedSuite))
}

func (s *MarkUserDeletedSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.uowMock = usecasemocks.NewUnitOfWorkVoid(s.T())
	s.factoryMock = interfacesmocks.NewMockRepositoryFactory(s.T())
	s.repoMock = interfacesmocks.NewMockUserRepository(s.T())
	s.publisherMock = outboxmocks.NewPublisher(s.T())
}

func (s *MarkUserDeletedSuite) TestExecute() {
	type args struct {
		input input.MarkUserDeleted
	}

	type dependencies struct {
		factoryMock   *interfacesmocks.MockRepositoryFactory
		publisherMock *outboxmocks.Publisher
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(error)
	}{
		{
			name: "deve marcar usuario como deletado e publicar user.deleted com sucesso",
			args: args{input: input.MarkUserDeleted{ID: "a0a0a0a0-0000-0000-0000-000000000001"}},
			dependencies: dependencies{
				factoryMock: func() *interfacesmocks.MockRepositoryFactory {
					s.factoryMock.EXPECT().UserRepository(mock.Anything).Return(s.repoMock).Once()
					s.repoMock.EXPECT().MarkDeleted(mock.Anything, "a0a0a0a0-0000-0000-0000-000000000001", mock.Anything).Return(nil).Once()
					return s.factoryMock
				}(),
				publisherMock: func() *outboxmocks.Publisher {
					s.publisherMock.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(ev outbox.Event) bool {
						return ev.AggregateUserID == "a0a0a0a0-0000-0000-0000-000000000001"
					})).Return(nil).Once()
					return s.publisherMock
				}(),
			},
			expect: func(err error) {
				s.Require().NoError(err)
			},
		},
		{
			name: "deve propagar erro de usuario nao encontrado",
			args: args{input: input.MarkUserDeleted{ID: "a0a0a0a0-0000-0000-0000-000000000002"}},
			dependencies: dependencies{
				factoryMock: func() *interfacesmocks.MockRepositoryFactory {
					s.factoryMock.EXPECT().UserRepository(mock.Anything).Return(s.repoMock).Once()
					s.repoMock.EXPECT().MarkDeleted(mock.Anything, "a0a0a0a0-0000-0000-0000-000000000002", mock.Anything).Return(application.ErrUserNotFound).Once()
					return s.factoryMock
				}(),
				publisherMock: s.publisherMock,
			},
			expect: func(err error) {
				s.Require().Error(err)
				s.Require().ErrorIs(err, application.ErrUserNotFound)
			},
		},
		{
			name: "deve propagar erro de infraestrutura",
			args: args{input: input.MarkUserDeleted{ID: "a0a0a0a0-0000-0000-0000-000000000003"}},
			dependencies: dependencies{
				factoryMock: func() *interfacesmocks.MockRepositoryFactory {
					ioErr := errors.New("db unavailable")
					s.factoryMock.EXPECT().UserRepository(mock.Anything).Return(s.repoMock).Once()
					s.repoMock.EXPECT().MarkDeleted(mock.Anything, "a0a0a0a0-0000-0000-0000-000000000003", mock.Anything).Return(ioErr).Once()
					return s.factoryMock
				}(),
				publisherMock: s.publisherMock,
			},
			expect: func(err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "db unavailable")
			},
		},
		{
			name: "deve propagar erro do outbox e causar rollback",
			args: args{input: input.MarkUserDeleted{ID: "a0a0a0a0-0000-0000-0000-000000000004"}},
			dependencies: dependencies{
				factoryMock: func() *interfacesmocks.MockRepositoryFactory {
					s.factoryMock.EXPECT().UserRepository(mock.Anything).Return(s.repoMock).Once()
					s.repoMock.EXPECT().MarkDeleted(mock.Anything, "a0a0a0a0-0000-0000-0000-000000000004", mock.Anything).Return(nil).Once()
					return s.factoryMock
				}(),
				publisherMock: func() *outboxmocks.Publisher {
					s.publisherMock.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("outbox unavailable")).Once()
					return s.publisherMock
				}(),
			},
			expect: func(err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "outbox unavailable")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sut := NewMarkUserDeleted(s.uowMock, scenario.dependencies.factoryMock, scenario.dependencies.publisherMock, s.obs)
			err := sut.Execute(s.ctx, scenario.args.input)
			scenario.expect(err)
		})
	}
}
