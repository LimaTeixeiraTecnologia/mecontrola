package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/handlers"
	handlersmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/handlers/mocks"
)

type TokenStateHandlerSuite struct {
	suite.Suite
	ctx context.Context
}

func TestTokenStateHandlerSuite(t *testing.T) {
	suite.Run(t, new(TokenStateHandlerSuite))
}

func (s *TokenStateHandlerSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *TokenStateHandlerSuite) TestHandle() {
	type args struct {
		token string
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(*handlersmocks.GetTokenStateUseCase)
		expect func(*httptest.ResponseRecorder)
	}{
		{
			name: "deve retornar token pronto para ativacao",
			args: args{token: "some-token"},
			setup: func(useCase *handlersmocks.GetTokenStateUseCase) {
				useCase.EXPECT().Execute(mock.Anything, "some-token").Return(usecases.GetTokenStateResult{
					Output: output.GetTokenStateOutput{
						ReadyToActivate:  true,
						WaMeURL:          "https://wa.me/5511999999999?text=ATIVAR%20tok123",
						BotNumberDisplay: "+55 11 9XXXX-XXXX",
					},
				}, nil).Once()
			},
			expect: func(recorder *httptest.ResponseRecorder) {
				s.Equal(http.StatusOK, recorder.Code)
				s.Equal("no-store", recorder.Header().Get("Cache-Control"))
				var response map[string]any
				s.Require().NoError(json.NewDecoder(recorder.Body).Decode(&response))
				s.Equal(true, response["ready_to_activate"])
				s.NotEmpty(response["wa_me_url"])
				s.NotEmpty(response["bot_number_display"])
			},
		},
		{
			name: "deve omitir campos quando nao estiver pronto",
			args: args{token: "bad-token"},
			setup: func(useCase *handlersmocks.GetTokenStateUseCase) {
				useCase.EXPECT().Execute(mock.Anything, "bad-token").Return(usecases.GetTokenStateResult{
					Output: output.GetTokenStateOutput{ReadyToActivate: false},
					Reason: usecases.TokenStateReasonNotFound,
				}, nil).Once()
			},
			expect: func(recorder *httptest.ResponseRecorder) {
				s.Equal(http.StatusOK, recorder.Code)
				var response map[string]any
				s.Require().NoError(json.NewDecoder(recorder.Body).Decode(&response))
				s.Equal(false, response["ready_to_activate"])
				s.Nil(response["wa_me_url"])
				s.Nil(response["bot_number_display"])
			},
		},
		{
			name: "deve responder ok para estado pendente",
			args: args{token: "tok-pending"},
			setup: func(useCase *handlersmocks.GetTokenStateUseCase) {
				useCase.EXPECT().Execute(mock.Anything, "tok-pending").Return(usecases.GetTokenStateResult{
					Output: output.GetTokenStateOutput{ReadyToActivate: false},
					Reason: usecases.TokenStateReasonPending,
				}, nil).Once()
			},
			expect: func(recorder *httptest.ResponseRecorder) {
				s.Equal(http.StatusOK, recorder.Code)
			},
		},
		{
			name: "deve responder ok para estado expirado",
			args: args{token: "tok-expired"},
			setup: func(useCase *handlersmocks.GetTokenStateUseCase) {
				useCase.EXPECT().Execute(mock.Anything, "tok-expired").Return(usecases.GetTokenStateResult{
					Output: output.GetTokenStateOutput{ReadyToActivate: false},
					Reason: usecases.TokenStateReasonExpired,
				}, nil).Once()
			},
			expect: func(recorder *httptest.ResponseRecorder) {
				s.Equal(http.StatusOK, recorder.Code)
			},
		},
		{
			name: "deve responder ok para estado consumido",
			args: args{token: "tok-consumed"},
			setup: func(useCase *handlersmocks.GetTokenStateUseCase) {
				useCase.EXPECT().Execute(mock.Anything, "tok-consumed").Return(usecases.GetTokenStateResult{
					Output: output.GetTokenStateOutput{ReadyToActivate: false},
					Reason: usecases.TokenStateReasonConsumed,
				}, nil).Once()
			},
			expect: func(recorder *httptest.ResponseRecorder) {
				s.Equal(http.StatusOK, recorder.Code)
			},
		},
		{
			name: "deve responder erro interno quando use case falha",
			args: args{token: "tok-err"},
			setup: func(useCase *handlersmocks.GetTokenStateUseCase) {
				useCase.EXPECT().Execute(mock.Anything, "tok-err").Return(usecases.GetTokenStateResult{}, errors.New("database error")).Once()
			},
			expect: func(recorder *httptest.ResponseRecorder) {
				s.Equal(http.StatusInternalServerError, recorder.Code)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			useCase := handlersmocks.NewGetTokenStateUseCase(s.T())
			scenario.setup(useCase)
			handler := handlers.NewTokenStateHandler(useCase, func(string) {}, noop.NewProvider())
			router := chi.NewRouter()
			router.Get("/api/v1/onboarding/tokens/{token}/state", handler.Handle)

			request := httptest.NewRequest(http.MethodGet, "/api/v1/onboarding/tokens/"+scenario.args.token+"/state", nil)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			scenario.expect(recorder)
		})
	}
}
