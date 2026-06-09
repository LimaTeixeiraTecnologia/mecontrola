package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/handlers"
	handlersmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/handlers/mocks"
)

type CreateCheckoutHandlerSuite struct {
	suite.Suite
	ctx context.Context
}

func TestCreateCheckoutHandlerSuite(t *testing.T) {
	suite.Run(t, new(CreateCheckoutHandlerSuite))
}

func (s *CreateCheckoutHandlerSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *CreateCheckoutHandlerSuite) TestHandle() {
	type args struct {
		body   string
		origin string
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(*handlersmocks.CreateCheckoutSessionUseCase)
		expect func(*httptest.ResponseRecorder)
	}{
		{
			name: "deve retornar created quando sucesso",
			args: args{body: `{"plan_id":"11111111-1111-1111-1111-111111111111"}`},
			setup: func(useCase *handlersmocks.CreateCheckoutSessionUseCase) {
				useCase.EXPECT().Execute(mock.Anything, input.CreateCheckoutSessionInput{
					PlanID: "11111111-1111-1111-1111-111111111111",
				}).Return(output.CreateCheckoutSessionOutput{
					CheckoutURL: "https://pay.kiwify.com.br/abc?sck=tok123",
					TokenID:     "token-id-1",
				}, nil).Once()
			},
			expect: func(recorder *httptest.ResponseRecorder) {
				s.Equal(http.StatusCreated, recorder.Code)
				var response map[string]any
				s.Require().NoError(json.NewDecoder(recorder.Body).Decode(&response))
				s.Equal("https://pay.kiwify.com.br/abc?sck=tok123", response["checkout_url"])
			},
		},
		{
			name:  "deve retornar bad request sem plan id",
			args:  args{body: `{}`},
			setup: func(useCase *handlersmocks.CreateCheckoutSessionUseCase) {},
			expect: func(recorder *httptest.ResponseRecorder) {
				s.Equal(http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "deve retornar bad request para json invalido",
			args:  args{body: `not-json`},
			setup: func(useCase *handlersmocks.CreateCheckoutSessionUseCase) {},
			expect: func(recorder *httptest.ResponseRecorder) {
				s.Equal(http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "deve retornar bad request para plano desconhecido",
			args: args{body: `{"plan_id":"unknown-plan"}`},
			setup: func(useCase *handlersmocks.CreateCheckoutSessionUseCase) {
				useCase.EXPECT().Execute(mock.Anything, input.CreateCheckoutSessionInput{PlanID: "unknown-plan"}).
					Return(output.CreateCheckoutSessionOutput{}, application.ErrUnknownPlan).Once()
			},
			expect: func(recorder *httptest.ResponseRecorder) {
				s.Equal(http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "deve retornar service unavailable quando checkout indisponivel",
			args: args{body: `{"plan_id":"22222222-2222-2222-2222-222222222222"}`},
			setup: func(useCase *handlersmocks.CreateCheckoutSessionUseCase) {
				useCase.EXPECT().Execute(mock.Anything, input.CreateCheckoutSessionInput{
					PlanID: "22222222-2222-2222-2222-222222222222",
				}).Return(output.CreateCheckoutSessionOutput{}, application.ErrCheckoutUnavailable).Once()
			},
			expect: func(recorder *httptest.ResponseRecorder) {
				s.Equal(http.StatusServiceUnavailable, recorder.Code)
			},
		},
		{
			name: "deve aceitar origem permitida",
			args: args{
				body:   `{"plan_id":"11111111-1111-1111-1111-111111111111"}`,
				origin: "https://www.mecontrola.app.br",
			},
			setup: func(useCase *handlersmocks.CreateCheckoutSessionUseCase) {
				useCase.EXPECT().Execute(mock.Anything, input.CreateCheckoutSessionInput{
					PlanID: "11111111-1111-1111-1111-111111111111",
				}).Return(output.CreateCheckoutSessionOutput{CheckoutURL: "https://pay.kiwify.com.br/abc"}, nil).Once()
			},
			expect: func(recorder *httptest.ResponseRecorder) {
				s.Equal(http.StatusCreated, recorder.Code)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			useCase := handlersmocks.NewCreateCheckoutSessionUseCase(s.T())
			scenario.setup(useCase)
			handler := handlers.NewCreateCheckoutHandler(useCase, func(string) {}, func() {}, noop.NewProvider())

			request := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/checkout", strings.NewReader(scenario.args.body))
			request.Header.Set("Content-Type", "application/json")
			if scenario.args.origin != "" {
				request.Header.Set("Origin", scenario.args.origin)
			}

			recorder := httptest.NewRecorder()
			handler.Handle(recorder, request)
			scenario.expect(recorder)
		})
	}
}
