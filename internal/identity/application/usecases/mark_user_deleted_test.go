package usecases_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	application "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	interfacesmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	usecasemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases/mocks"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

type MarkUserDeletedSuite struct {
	suite.Suite
	ctx context.Context
}

func TestMarkUserDeleted(t *testing.T) {
	suite.Run(t, new(MarkUserDeletedSuite))
}

func (s *MarkUserDeletedSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *MarkUserDeletedSuite) TestExecute() {
	type args struct {
		input input.MarkUserDeleted
	}

	type dependencies struct {
		uow       *usecasemocks.UnitOfWorkVoid
		factory   *interfacesmocks.MockRepositoryFactory
		repo      *interfacesmocks.MockUserRepository
		publisher *outboxmocks.Publisher
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(dependencies)
		expect func(error)
	}{
		{
			name: "deve marcar usuario como deletado e publicar user.deleted com sucesso",
			args: args{input: input.MarkUserDeleted{ID: "a0a0a0a0-0000-0000-0000-000000000001"}},
			setup: func(deps dependencies) {
				deps.factory.EXPECT().UserRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().MarkDeleted(mock.Anything, "a0a0a0a0-0000-0000-0000-000000000001", mock.Anything).Return(nil).Once()
				deps.publisher.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(ev any) bool {
					return true // user.deleted event published
				})).Return(nil).Once()
			},
			expect: func(err error) {
				s.Require().NoError(err)
			},
		},
		{
			name: "deve propagar erro de usuario nao encontrado",
			args: args{input: input.MarkUserDeleted{ID: "a0a0a0a0-0000-0000-0000-000000000002"}},
			setup: func(deps dependencies) {
				deps.factory.EXPECT().UserRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().MarkDeleted(mock.Anything, "a0a0a0a0-0000-0000-0000-000000000002", mock.Anything).Return(application.ErrUserNotFound).Once()
			},
			expect: func(err error) {
				s.Require().Error(err)
				s.Require().ErrorIs(err, application.ErrUserNotFound)
			},
		},
		{
			name: "deve propagar erro de infraestrutura",
			args: args{input: input.MarkUserDeleted{ID: "a0a0a0a0-0000-0000-0000-000000000003"}},
			setup: func(deps dependencies) {
				ioErr := errors.New("db unavailable")
				deps.factory.EXPECT().UserRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().MarkDeleted(mock.Anything, "a0a0a0a0-0000-0000-0000-000000000003", mock.Anything).Return(ioErr).Once()
			},
			expect: func(err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "db unavailable")
			},
		},
		{
			name: "deve propagar erro do outbox e causar rollback",
			args: args{input: input.MarkUserDeleted{ID: "a0a0a0a0-0000-0000-0000-000000000004"}},
			setup: func(deps dependencies) {
				deps.factory.EXPECT().UserRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().MarkDeleted(mock.Anything, "a0a0a0a0-0000-0000-0000-000000000004", mock.Anything).Return(nil).Once()
				deps.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("outbox unavailable")).Once()
			},
			expect: func(err error) {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), "outbox unavailable")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			deps := dependencies{
				uow:       usecasemocks.NewUnitOfWorkVoid(s.T()),
				factory:   interfacesmocks.NewMockRepositoryFactory(s.T()),
				repo:      interfacesmocks.NewMockUserRepository(s.T()),
				publisher: outboxmocks.NewPublisher(s.T()),
			}
			scenario.setup(deps)

			sut := usecases.NewMarkUserDeleted(deps.uow, deps.factory, deps.publisher, noop.NewProvider())
			err := sut.Execute(s.ctx, scenario.args.input)

			scenario.expect(err)
		})
	}
}
