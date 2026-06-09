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
	usecasemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

type ProjectAuthEventSuite struct {
	suite.Suite
	ctx context.Context
}

func TestProjectAuthEvent(t *testing.T) {
	suite.Run(t, new(ProjectAuthEventSuite))
}

func (s *ProjectAuthEventSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ProjectAuthEventSuite) validPayload(kind string, userID *string, reason *string) []byte {
	p := map[string]any{
		"event_id":    uuid.New().String(),
		"kind":        kind,
		"source":      "whatsapp",
		"occurred_at": time.Now().UTC().Format(time.RFC3339),
	}
	if userID != nil {
		p["user_id"] = *userID
	}
	if reason != nil {
		p["reason"] = *reason
	}
	raw, err := json.Marshal(p)
	s.Require().NoError(err)
	return raw
}

func (s *ProjectAuthEventSuite) TestExecute() {
	type args struct {
		in input.ProjectAuthEvent
	}
	type dependencies struct {
		factory *ifacemocks.RepositoryFactory
		repo    *ifacemocks.AuthEventsRepository
	}

	scenarios := []struct {
		name   string
		args   func() args
		setup  func(dependencies)
		expect func(error)
	}{
		{
			name: "deve inserir auth event com user_id",
			args: func() args {
				uid := uuid.New().String()
				return args{in: input.ProjectAuthEvent{
					EventType: "auth.principal_established",
					Payload:   s.validPayload("principal_established", &uid, nil),
				}}
			},
			setup: func(deps dependencies) {
				deps.factory.EXPECT().AuthEventsRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(ev entities.AuthEvent) bool {
					return ev.Kind() == entities.AuthEventKindPrincipalEstablished &&
						ev.UserID() != nil &&
						ev.Source() == entities.AuthEventSourceWhatsApp
				})).Return(nil).Once()
			},
			expect: func(err error) {
				s.Require().NoError(err)
			},
		},
		{
			name: "deve inserir auth event sem user_id",
			args: func() args {
				return args{in: input.ProjectAuthEvent{
					EventType: "auth.unknown_user",
					Payload:   s.validPayload("unknown_user", nil, nil),
				}}
			},
			setup: func(deps dependencies) {
				deps.factory.EXPECT().AuthEventsRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(ev entities.AuthEvent) bool {
					return ev.Kind() == entities.AuthEventKindUnknownUser &&
						ev.UserID() == nil
				})).Return(nil).Once()
			},
			expect: func(err error) {
				s.Require().NoError(err)
			},
		},
		{
			name: "deve inserir auth event com reason",
			args: func() args {
				reason := "invalid_signature"
				return args{in: input.ProjectAuthEvent{
					EventType: "auth.failed",
					Payload:   s.validPayload("failed", nil, &reason),
				}}
			},
			setup: func(deps dependencies) {
				deps.factory.EXPECT().AuthEventsRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().Insert(mock.Anything, mock.MatchedBy(func(ev entities.AuthEvent) bool {
					return ev.Kind() == entities.AuthEventKindFailed &&
						ev.Reason() != nil &&
						*ev.Reason() == entities.AuthEventReasonInvalidSignature
				})).Return(nil).Once()
			},
			expect: func(err error) {
				s.Require().NoError(err)
			},
		},
		{
			name: "deve propagar erro do repositorio",
			args: func() args {
				return args{in: input.ProjectAuthEvent{
					EventType: "auth.principal_established",
					Payload:   s.validPayload("principal_established", nil, nil),
				}}
			},
			setup: func(deps dependencies) {
				deps.factory.EXPECT().AuthEventsRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().Insert(mock.Anything, mock.Anything).
					Return(fmt.Errorf("db error")).Once()
			},
			expect: func(err error) {
				s.Require().Error(err)
				s.ErrorContains(err, "insert")
			},
		},
		{
			name: "payload invalido deve retornar erro de decode",
			args: func() args {
				return args{in: input.ProjectAuthEvent{
					EventType: "auth.principal_established",
					Payload:   []byte("not-json"),
				}}
			},
			setup: func(deps dependencies) {},
			expect: func(err error) {
				s.Require().Error(err)
				s.ErrorContains(err, "decode")
			},
		},
		{
			name: "event_id invalido deve retornar erro de parse",
			args: func() args {
				p := map[string]any{
					"event_id":    "not-a-uuid",
					"kind":        "principal_established",
					"source":      "whatsapp",
					"occurred_at": time.Now().UTC().Format(time.RFC3339),
				}
				raw, err := json.Marshal(p)
				s.Require().NoError(err)
				return args{in: input.ProjectAuthEvent{
					EventType: "auth.principal_established",
					Payload:   raw,
				}}
			},
			setup: func(deps dependencies) {},
			expect: func(err error) {
				s.Require().Error(err)
				s.ErrorContains(err, "parse event_id")
			},
		},
		{
			name: "occurred_at invalido deve retornar erro de parse",
			args: func() args {
				p := map[string]any{
					"event_id":    uuid.New().String(),
					"kind":        "principal_established",
					"source":      "whatsapp",
					"occurred_at": "not-a-time",
				}
				raw, err := json.Marshal(p)
				s.Require().NoError(err)
				return args{in: input.ProjectAuthEvent{
					EventType: "auth.principal_established",
					Payload:   raw,
				}}
			},
			setup: func(deps dependencies) {},
			expect: func(err error) {
				s.Require().Error(err)
				s.ErrorContains(err, "parse occurred_at")
			},
		},
		{
			name: "user_id invalido deve retornar erro de parse",
			args: func() args {
				uid := "not-a-uuid"
				p := map[string]any{
					"event_id":    uuid.New().String(),
					"kind":        "principal_established",
					"source":      "whatsapp",
					"occurred_at": time.Now().UTC().Format(time.RFC3339),
					"user_id":     uid,
				}
				raw, err := json.Marshal(p)
				s.Require().NoError(err)
				return args{in: input.ProjectAuthEvent{
					EventType: "auth.principal_established",
					Payload:   raw,
				}}
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
			factory := ifacemocks.NewRepositoryFactory(s.T())
			scenario.setup(dependencies{factory: factory, repo: repo})

			a := scenario.args()
			sut := usecases.NewProjectAuthEvent(factory, usecasemocks.NewFakeManager(), noop.NewProvider())
			err := sut.Execute(s.ctx, a.in)
			scenario.expect(err)
		})
	}
}
