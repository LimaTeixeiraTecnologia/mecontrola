package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	application "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/handlers"
	handlermocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/handlers/mocks"
)

type UpsertHandlerSuite struct {
	suite.Suite
	ctx context.Context
}

func TestUpsertHandler(t *testing.T) {
	suite.Run(t, new(UpsertHandlerSuite))
}

func (s *UpsertHandlerSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *UpsertHandlerSuite) requestWithJSON(body any) *http.Request {
	var buffer bytes.Buffer
	s.Require().NoError(json.NewEncoder(&buffer).Encode(body))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/identity/users", &buffer)
	req.Header.Set("Content-Type", "application/json")
	return req.WithContext(s.ctx)
}

func (s *UpsertHandlerSuite) requestWithRaw(raw string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/identity/users", bytes.NewBufferString(raw))
	req.Header.Set("Content-Type", "application/json")
	return req.WithContext(s.ctx)
}

func (s *UpsertHandlerSuite) TestHandle() {
	type args struct {
		request *http.Request
	}

	type dependencies struct {
		useCase *handlermocks.MockUpsertUseCase
	}

	now := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)

	scenarios := []struct {
		name   string
		args   args
		setup  func(dependencies)
		expect func(*httptest.ResponseRecorder)
	}{
		{
			name: "deve responder 200 para payload valido",
			args: args{
				request: s.requestWithJSON(map[string]string{
					"whatsapp":     "+5511987654321",
					"email":        "user@example.com",
					"display_name": "Test User",
				}),
			},
			setup: func(deps dependencies) {
				deps.useCase.EXPECT().Execute(
					mock.Anything,
					input.UpsertUserByWhatsApp{
						WhatsAppNumber: "+5511987654321",
						Email:          "user@example.com",
						DisplayName:    "Test User",
					},
				).Return(output.UpsertUserByWhatsApp{
					ID:             "some-uuid",
					WhatsAppNumber: "+5511987654321",
					Email:          "user@example.com",
					DisplayName:    "Test User",
					Status:         "active",
					CreatedAt:      now,
					UpdatedAt:      now,
				}, nil).Once()
			},
			expect: func(response *httptest.ResponseRecorder) {
				s.Equal(http.StatusOK, response.Code)

				var payload map[string]any
				s.Require().NoError(json.NewDecoder(response.Body).Decode(&payload))
				s.Equal("some-uuid", payload["id"])
			},
		},
		{
			name: "deve responder 400 para json malformado",
			args: args{request: s.requestWithRaw(`{not valid json`)},
			setup: func(deps dependencies) {
				_ = deps
			},
			expect: func(response *httptest.ResponseRecorder) {
				s.Equal(http.StatusBadRequest, response.Code)

				var payload map[string]any
				s.Require().NoError(json.NewDecoder(response.Body).Decode(&payload))
				errorsPayload, ok := payload["errors"].(map[string]any)
				s.Require().True(ok)
				s.Equal("invalid_payload", errorsPayload["code"])
			},
		},
		{
			name: "deve responder 400 para whatsapp invalido",
			args: args{
				request: s.requestWithJSON(map[string]string{
					"whatsapp": "123",
					"email":    "user@example.com",
				}),
			},
			setup: func(deps dependencies) {
				deps.useCase.EXPECT().Execute(
					mock.Anything,
					input.UpsertUserByWhatsApp{
						WhatsAppNumber: "123",
						Email:          "user@example.com",
					},
				).Return(output.UpsertUserByWhatsApp{}, application.ErrInvalidWhatsApp).Once()
			},
			expect: func(response *httptest.ResponseRecorder) {
				s.Equal(http.StatusBadRequest, response.Code)
			},
		},
		{
			name: "deve responder 400 para email invalido",
			args: args{
				request: s.requestWithJSON(map[string]string{
					"whatsapp": "+5511987654321",
					"email":    "not-an-email",
				}),
			},
			setup: func(deps dependencies) {
				deps.useCase.EXPECT().Execute(
					mock.Anything,
					input.UpsertUserByWhatsApp{
						WhatsAppNumber: "+5511987654321",
						Email:          "not-an-email",
					},
				).Return(output.UpsertUserByWhatsApp{}, application.ErrInvalidEmail).Once()
			},
			expect: func(response *httptest.ResponseRecorder) {
				s.Equal(http.StatusBadRequest, response.Code)
			},
		},
		{
			name: "deve responder 409 para whatsapp em uso",
			args: args{
				request: s.requestWithJSON(map[string]string{
					"whatsapp": "+5511987654321",
				}),
			},
			setup: func(deps dependencies) {
				deps.useCase.EXPECT().Execute(
					mock.Anything,
					input.UpsertUserByWhatsApp{WhatsAppNumber: "+5511987654321"},
				).Return(output.UpsertUserByWhatsApp{}, application.ErrWhatsAppNumberInUse).Once()
			},
			expect: func(response *httptest.ResponseRecorder) {
				s.Equal(http.StatusConflict, response.Code)
			},
		},
		{
			name: "deve responder 409 para email em uso",
			args: args{
				request: s.requestWithJSON(map[string]string{
					"whatsapp": "+5511987654321",
					"email":    "taken@example.com",
				}),
			},
			setup: func(deps dependencies) {
				deps.useCase.EXPECT().Execute(
					mock.Anything,
					input.UpsertUserByWhatsApp{
						WhatsAppNumber: "+5511987654321",
						Email:          "taken@example.com",
					},
				).Return(output.UpsertUserByWhatsApp{}, application.ErrEmailInUse).Once()
			},
			expect: func(response *httptest.ResponseRecorder) {
				s.Equal(http.StatusConflict, response.Code)
			},
		},
		{
			name: "deve responder 500 para erro interno",
			args: args{
				request: s.requestWithJSON(map[string]string{
					"whatsapp": "+5511987654321",
				}),
			},
			setup: func(deps dependencies) {
				deps.useCase.EXPECT().Execute(
					mock.Anything,
					input.UpsertUserByWhatsApp{WhatsAppNumber: "+5511987654321"},
				).Return(output.UpsertUserByWhatsApp{}, errors.New("unexpected db error")).Once()
			},
			expect: func(response *httptest.ResponseRecorder) {
				s.Equal(http.StatusInternalServerError, response.Code)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			deps := dependencies{
				useCase: handlermocks.NewMockUpsertUseCase(s.T()),
			}
			scenario.setup(deps)

			sut := handlers.NewUpsertUserByWhatsAppHandler(deps.useCase, noop.NewProvider())
			response := httptest.NewRecorder()
			sut.Handle(response, scenario.args.request)

			scenario.expect(response)
		})
	}
}
