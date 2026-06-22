package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	ifacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces/mocks"
)

type AnonymizeUserAuthEventsSuite struct {
	suite.Suite
	ctx      context.Context
	obs      observability.Observability
	repoMock *ifacemocks.AuthEventsRepository
}

func TestAnonymizeUserAuthEvents(t *testing.T) {
	suite.Run(t, new(AnonymizeUserAuthEventsSuite))
}

func (s *AnonymizeUserAuthEventsSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.repoMock = ifacemocks.NewAuthEventsRepository(s.T())
}

func (s *AnonymizeUserAuthEventsSuite) validPayload(userID string) []byte {
	p := map[string]any{
		"event_id":   uuid.New().String(),
		"user_id":    userID,
		"deleted_at": time.Now().UTC().Format(time.RFC3339),
	}
	raw, err := json.Marshal(p)
	s.Require().NoError(err)
	return raw
}

func (s *AnonymizeUserAuthEventsSuite) TestExecute() {
	type args struct {
		in input.AnonymizeUserAuthEvents
	}

	type dependencies struct {
		repoMock *ifacemocks.AuthEventsRepository
	}

	scenarios := []struct {
		name         string
		args         func() args
		dependencies dependencies
		expect       func(error)
	}{
		{
			name: "deve anonimizar linhas do user",
			args: func() args {
				return args{in: input.AnonymizeUserAuthEvents{
					Payload: s.validPayload(uuid.New().String()),
				}}
			},
			dependencies: dependencies{
				repoMock: func() *ifacemocks.AuthEventsRepository {
					s.repoMock.EXPECT().AnonymizeByUserID(mock.Anything, mock.AnythingOfType("uuid.UUID")).
						Return(nil).Once()
					return s.repoMock
				}(),
			},
			expect: func(err error) {
				s.Require().NoError(err)
			},
		},
		{
			name: "deve propagar erro do repositorio",
			args: func() args {
				return args{in: input.AnonymizeUserAuthEvents{
					Payload: s.validPayload(uuid.New().String()),
				}}
			},
			dependencies: dependencies{
				repoMock: func() *ifacemocks.AuthEventsRepository {
					s.repoMock.EXPECT().AnonymizeByUserID(mock.Anything, mock.AnythingOfType("uuid.UUID")).
						Return(fmt.Errorf("db error")).Once()
					return s.repoMock
				}(),
			},
			expect: func(err error) {
				s.Require().Error(err)
				s.ErrorContains(err, "anonymize_by_user_id")
			},
		},
		{
			name: "payload invalido deve retornar erro de decode",
			args: func() args {
				return args{in: input.AnonymizeUserAuthEvents{
					Payload: []byte("{invalid"),
				}}
			},
			dependencies: dependencies{repoMock: s.repoMock},
			expect: func(err error) {
				s.Require().Error(err)
				s.ErrorContains(err, "decode")
			},
		},
		{
			name: "user_id invalido deve retornar erro de parse",
			args: func() args {
				p := map[string]any{
					"event_id":   uuid.New().String(),
					"user_id":    "not-a-uuid",
					"deleted_at": time.Now().UTC().Format(time.RFC3339),
				}
				raw, err := json.Marshal(p)
				s.Require().NoError(err)
				return args{in: input.AnonymizeUserAuthEvents{Payload: raw}}
			},
			dependencies: dependencies{repoMock: s.repoMock},
			expect: func(err error) {
				s.Require().Error(err)
				s.ErrorContains(err, "parse user_id")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			a := scenario.args()
			sut := NewAnonymizeUserAuthEvents(scenario.dependencies.repoMock, s.obs)
			err := sut.Execute(s.ctx, a.in)
			scenario.expect(err)
		})
	}
}
