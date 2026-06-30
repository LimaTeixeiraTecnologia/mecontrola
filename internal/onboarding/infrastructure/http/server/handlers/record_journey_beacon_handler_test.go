package handlers_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/handlers"
	handlersmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/handlers/mocks"
)

type RecordJourneyBeaconHandlerSuite struct {
	suite.Suite
}

func TestRecordJourneyBeaconHandlerSuite(t *testing.T) {
	suite.Run(t, new(RecordJourneyBeaconHandlerSuite))
}

func (s *RecordJourneyBeaconHandlerSuite) TestHandle() {
	type args struct {
		token string
		body  string
	}
	type dependencies struct {
		useCase *handlersmocks.RecordJourneyTimestampUseCase
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(*httptest.ResponseRecorder)
	}{
		{
			name: "deve responder 204 para page_opened com token valido",
			args: args{token: "tok-page", body: `{"event":"page_opened"}`},
			dependencies: dependencies{
				useCase: func() *handlersmocks.RecordJourneyTimestampUseCase {
					m := handlersmocks.NewRecordJourneyTimestampUseCase(s.T())
					m.EXPECT().
						Execute(mock.Anything, input.RecordJourneyTimestampInput{ClearToken: "tok-page", Event: "page_opened"}).
						Return(nil).
						Once()
					return m
				}(),
			},
			expect: func(rec *httptest.ResponseRecorder) {
				s.Equal(http.StatusNoContent, rec.Code)
			},
		},
		{
			name: "deve responder 204 para whatsapp_opened com token valido",
			args: args{token: "tok-wa", body: `{"event":"whatsapp_opened"}`},
			dependencies: dependencies{
				useCase: func() *handlersmocks.RecordJourneyTimestampUseCase {
					m := handlersmocks.NewRecordJourneyTimestampUseCase(s.T())
					m.EXPECT().
						Execute(mock.Anything, input.RecordJourneyTimestampInput{ClearToken: "tok-wa", Event: "whatsapp_opened"}).
						Return(nil).
						Once()
					return m
				}(),
			},
			expect: func(rec *httptest.ResponseRecorder) {
				s.Equal(http.StatusNoContent, rec.Code)
			},
		},
		{
			name: "deve responder 204 mesmo quando token invalido (nao vaza estado)",
			args: args{token: "invalid-tok", body: `{"event":"page_opened"}`},
			dependencies: dependencies{
				useCase: func() *handlersmocks.RecordJourneyTimestampUseCase {
					m := handlersmocks.NewRecordJourneyTimestampUseCase(s.T())
					m.EXPECT().
						Execute(mock.Anything, input.RecordJourneyTimestampInput{ClearToken: "invalid-tok", Event: "page_opened"}).
						Return(nil).
						Once()
					return m
				}(),
			},
			expect: func(rec *httptest.ResponseRecorder) {
				s.Equal(http.StatusNoContent, rec.Code)
			},
		},
		{
			name: "deve responder 204 mesmo quando usecase retorna erro interno",
			args: args{token: "tok-err", body: `{"event":"page_opened"}`},
			dependencies: dependencies{
				useCase: func() *handlersmocks.RecordJourneyTimestampUseCase {
					m := handlersmocks.NewRecordJourneyTimestampUseCase(s.T())
					m.EXPECT().
						Execute(mock.Anything, input.RecordJourneyTimestampInput{ClearToken: "tok-err", Event: "page_opened"}).
						Return(errors.New("db error")).
						Once()
					return m
				}(),
			},
			expect: func(rec *httptest.ResponseRecorder) {
				s.Equal(http.StatusNoContent, rec.Code)
			},
		},
		{
			name: "deve responder 204 quando discriminador invalido",
			args: args{token: "tok-bad-ev", body: `{"event":"invalid_event"}`},
			dependencies: dependencies{
				useCase: func() *handlersmocks.RecordJourneyTimestampUseCase {
					m := handlersmocks.NewRecordJourneyTimestampUseCase(s.T())
					m.EXPECT().
						Execute(mock.Anything, input.RecordJourneyTimestampInput{ClearToken: "tok-bad-ev", Event: "invalid_event"}).
						Return(nil).
						Once()
					return m
				}(),
			},
			expect: func(rec *httptest.ResponseRecorder) {
				s.Equal(http.StatusNoContent, rec.Code)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			handler := handlers.NewRecordJourneyBeaconHandler(scenario.dependencies.useCase, noop.NewProvider())
			router := chi.NewRouter()
			router.Post("/api/v1/onboarding/tokens/{token}/opened", handler.Handle)

			req := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/onboarding/tokens/"+scenario.args.token+"/opened",
				strings.NewReader(scenario.args.body),
			)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			scenario.expect(rec)
		})
	}
}
