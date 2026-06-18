package usecases_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	ifacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
)

type AnonymizeUserAuthEventsSuite struct {
	suite.Suite
	ctx context.Context
}

func TestAnonymizeUserAuthEvents(t *testing.T) {
	suite.Run(t, new(AnonymizeUserAuthEventsSuite))
}

func (s *AnonymizeUserAuthEventsSuite) SetupTest() {
	s.ctx = context.Background()
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
		repo *ifacemocks.AuthEventsRepository
	}

	scenarios := []struct {
		name   string
		args   func() args
		setup  func(dependencies)
		expect func(error)
	}{
		{
			name: "deve anonimizar linhas do user",
			args: func() args {
				return args{in: input.AnonymizeUserAuthEvents{
					Payload: s.validPayload(uuid.New().String()),
				}}
			},
			setup: func(deps dependencies) {
				deps.repo.EXPECT().AnonymizeByUserID(mock.Anything, mock.AnythingOfType("uuid.UUID")).
					Return(nil).Once()
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
			setup: func(deps dependencies) {
				deps.repo.EXPECT().AnonymizeByUserID(mock.Anything, mock.AnythingOfType("uuid.UUID")).
					Return(fmt.Errorf("db error")).Once()
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
			setup: func(deps dependencies) {},
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
			setup: func(deps dependencies) {},
			expect: func(err error) {
				s.Require().Error(err)
				s.ErrorContains(err, "parse user_id")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			repo := ifacemocks.NewAuthEventsRepository(s.T())
			scenario.setup(dependencies{repo: repo})

			a := scenario.args()
			sut := usecases.NewAnonymizeUserAuthEvents(repo, noop.NewProvider())
			err := sut.Execute(s.ctx, a.in)
			scenario.expect(err)
		})
	}
}
