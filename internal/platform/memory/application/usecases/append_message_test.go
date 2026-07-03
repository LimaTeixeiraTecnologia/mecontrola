package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/mocks"
)

type AppendMessageSuite struct {
	suite.Suite
	ctx       context.Context
	obs       *fake.Provider
	storeMock *mocks.MessageStore
}

func TestAppendMessageSuite(t *testing.T) {
	suite.Run(t, new(AppendMessageSuite))
}

func (s *AppendMessageSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.storeMock = mocks.NewMessageStore(s.T())
}

func (s *AppendMessageSuite) TestExecute() {
	type args struct {
		in input.AppendMessageInput
	}
	type dependencies struct {
		storeMock *mocks.MessageStore
	}

	threadPK := uuid.New()

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(msg memory.Message, err error)
	}{
		{
			name: "deve adicionar mensagem com sucesso",
			args: args{in: input.AppendMessageInput{
				PlatformThreadID: threadPK,
				ResourceID:       "user-123",
				Role:             "user",
				Content:          "ola",
			}},
			dependencies: dependencies{
				storeMock: func() *mocks.MessageStore {
					s.storeMock.EXPECT().
						Append(mock.Anything, threadPK, mock.AnythingOfType("memory.Message")).
						Return(nil).
						Once()
					return s.storeMock
				}(),
			},
			expect: func(msg memory.Message, err error) {
				s.NoError(err)
				s.Equal(memory.RoleUser, msg.Role)
				s.Equal("ola", msg.Content)
				s.Equal("user-123", msg.ResourceID)
				s.NotEqual(uuid.Nil, msg.ID)
			},
		},
		{
			name: "deve retornar erro de validacao quando content vazio",
			args: args{in: input.AppendMessageInput{
				PlatformThreadID: threadPK,
				ResourceID:       "user-123",
				Role:             "user",
				Content:          "",
			}},
			dependencies: dependencies{storeMock: s.storeMock},
			expect: func(msg memory.Message, err error) {
				s.Error(err)
				s.ErrorIs(err, memory.ErrEmptyContent)
			},
		},
		{
			name: "deve retornar erro de validacao quando role invalido",
			args: args{in: input.AppendMessageInput{
				PlatformThreadID: threadPK,
				ResourceID:       "user-123",
				Role:             "invalid_role",
				Content:          "hello",
			}},
			dependencies: dependencies{storeMock: s.storeMock},
			expect: func(msg memory.Message, err error) {
				s.Error(err)
				s.ErrorIs(err, memory.ErrInvalidRole)
			},
		},
		{
			name: "deve propagar erro do store",
			args: args{in: input.AppendMessageInput{
				PlatformThreadID: threadPK,
				ResourceID:       "user-123",
				Role:             "assistant",
				Content:          "resposta",
			}},
			dependencies: dependencies{
				storeMock: func() *mocks.MessageStore {
					s.storeMock.EXPECT().
						Append(mock.Anything, threadPK, mock.AnythingOfType("memory.Message")).
						Return(errors.New("db error")).
						Once()
					return s.storeMock
				}(),
			},
			expect: func(msg memory.Message, err error) {
				s.Error(err)
				s.Equal(memory.Message{}, msg)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewAppendMessage(scenario.dependencies.storeMock, s.obs)
			msg, err := uc.Execute(s.ctx, scenario.args.in)
			scenario.expect(msg, err)
		})
	}
}
