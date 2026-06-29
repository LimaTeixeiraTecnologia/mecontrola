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

type GetOrCreateThreadSuite struct {
	suite.Suite
	ctx         context.Context
	obs         *fake.Provider
	gatewayMock *mocks.ThreadGateway
}

func TestGetOrCreateThreadSuite(t *testing.T) {
	suite.Run(t, new(GetOrCreateThreadSuite))
}

func (s *GetOrCreateThreadSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.gatewayMock = mocks.NewThreadGateway(s.T())
}

func (s *GetOrCreateThreadSuite) TestExecute() {
	type args struct {
		in input.GetOrCreateThreadInput
	}
	type dependencies struct {
		gatewayMock *mocks.ThreadGateway
	}

	threadID := uuid.New()
	expectedThread := memory.Thread{
		ID:         threadID,
		ResourceID: "user-123",
		ThreadID:   "whatsapp",
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(thread memory.Thread, err error)
	}{
		{
			name: "deve retornar thread existente ou criado",
			args: args{in: input.GetOrCreateThreadInput{ResourceID: "user-123", ThreadID: "whatsapp"}},
			dependencies: dependencies{
				gatewayMock: func() *mocks.ThreadGateway {
					s.gatewayMock.EXPECT().
						GetOrCreate(mock.Anything, "user-123", "whatsapp").
						Return(expectedThread, nil).
						Once()
					return s.gatewayMock
				}(),
			},
			expect: func(thread memory.Thread, err error) {
				s.NoError(err)
				s.Equal(expectedThread.ID, thread.ID)
				s.Equal("user-123", thread.ResourceID)
			},
		},
		{
			name:         "deve retornar erro de validacao quando resource_id vazio",
			args:         args{in: input.GetOrCreateThreadInput{ResourceID: "", ThreadID: "whatsapp"}},
			dependencies: dependencies{gatewayMock: s.gatewayMock},
			expect: func(thread memory.Thread, err error) {
				s.Error(err)
				s.ErrorIs(err, memory.ErrEmptyResourceID)
			},
		},
		{
			name:         "deve retornar erro de validacao quando thread_id vazio",
			args:         args{in: input.GetOrCreateThreadInput{ResourceID: "user-123", ThreadID: ""}},
			dependencies: dependencies{gatewayMock: s.gatewayMock},
			expect: func(thread memory.Thread, err error) {
				s.Error(err)
				s.ErrorIs(err, memory.ErrEmptyThreadID)
			},
		},
		{
			name: "deve propagar erro do gateway",
			args: args{in: input.GetOrCreateThreadInput{ResourceID: "user-123", ThreadID: "whatsapp"}},
			dependencies: dependencies{
				gatewayMock: func() *mocks.ThreadGateway {
					s.gatewayMock.EXPECT().
						GetOrCreate(mock.Anything, "user-123", "whatsapp").
						Return(memory.Thread{}, errors.New("db error")).
						Once()
					return s.gatewayMock
				}(),
			},
			expect: func(thread memory.Thread, err error) {
				s.Error(err)
				s.Equal(memory.Thread{}, thread)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewGetOrCreateThread(scenario.dependencies.gatewayMock, s.obs)
			thread, err := uc.Execute(s.ctx, scenario.args.in)
			scenario.expect(thread, err)
		})
	}
}
