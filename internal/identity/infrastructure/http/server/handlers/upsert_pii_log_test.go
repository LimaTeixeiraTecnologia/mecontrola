package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	application "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/handlers"
	handlermocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/handlers/mocks"
)

type UpsertPiiLogSuite struct {
	suite.Suite
}

type recordedLog struct {
	level  string
	msg    string
	fields []observability.Field
}

type recordingLogger struct {
	mu   sync.Mutex
	logs []recordedLog
}

type recordingObservability struct {
	observability.Observability
	logger *recordingLogger
}

func TestUpsertPiiLogSuite(t *testing.T) {
	suite.Run(t, new(UpsertPiiLogSuite))
}

func (s *UpsertPiiLogSuite) SetupTest() {}

func (l *recordingLogger) record(level string, msg string, fields []observability.Field) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.logs = append(l.logs, recordedLog{level: level, msg: msg, fields: fields})
}

func (l *recordingLogger) Debug(_ context.Context, msg string, fields ...observability.Field) {
	l.record("debug", msg, fields)
}

func (l *recordingLogger) Info(_ context.Context, msg string, fields ...observability.Field) {
	l.record("info", msg, fields)
}

func (l *recordingLogger) Warn(_ context.Context, msg string, fields ...observability.Field) {
	l.record("warn", msg, fields)
}

func (l *recordingLogger) Error(_ context.Context, msg string, fields ...observability.Field) {
	l.record("error", msg, fields)
}

func (l *recordingLogger) With(_ ...observability.Field) observability.Logger {
	return l
}

func (l *recordingLogger) findField(key string) (observability.Field, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, entry := range l.logs {
		for _, field := range entry.fields {
			if field.Key == key {
				return field, true
			}
		}
	}

	return observability.Field{}, false
}

func (l *recordingLogger) hasStringValue(needle string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, entry := range l.logs {
		for _, field := range entry.fields {
			if field.Kind() == observability.FieldKindString && strings.Contains(field.StringValue(), needle) {
				return true
			}
		}
	}

	return false
}

func (o *recordingObservability) Logger() observability.Logger {
	return o.logger
}

func (s *UpsertPiiLogSuite) newObservability() *recordingObservability {
	return &recordingObservability{
		Observability: noop.NewProvider(),
		logger:        &recordingLogger{},
	}
}

func (s *UpsertPiiLogSuite) request(body map[string]string) *http.Request {
	rawBody, err := json.Marshal(body)
	s.Require().NoError(err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/identity/users", bytes.NewReader(rawBody))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func (s *UpsertPiiLogSuite) TestHandleLogsMaskedWhatsApp() {
	type args struct {
		request *http.Request
	}

	type dependencies struct {
		useCase *handlermocks.MockUpsertUseCase
		obs     *recordingObservability
	}

	now := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)

	scenarios := []struct {
		name   string
		args   args
		setup  func(dependencies)
		expect func(*httptest.ResponseRecorder, *recordingObservability)
	}{
		{
			name: "deve mascarar whatsapp no log de sucesso",
			args: args{
				request: s.request(map[string]string{
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
					ID:             "user-uuid",
					WhatsAppNumber: "+5511987654321",
					Email:          "user@example.com",
					DisplayName:    "Test User",
					Status:         "active",
					CreatedAt:      now,
					UpdatedAt:      now,
				}, nil).Once()
			},
			expect: func(response *httptest.ResponseRecorder, obs *recordingObservability) {
				s.Equal(http.StatusOK, response.Code)
				field, ok := obs.logger.findField("whatsapp_masked")
				s.Require().True(ok)
				s.Contains(field.StringValue(), "9****-4321")
				s.False(obs.logger.hasStringValue("+5511987654321"))
			},
		},
		{
			name: "deve mascarar whatsapp no log de erro interno",
			args: args{
				request: s.request(map[string]string{
					"whatsapp": "+5511987654321",
				}),
			},
			setup: func(deps dependencies) {
				deps.useCase.EXPECT().Execute(
					mock.Anything,
					input.UpsertUserByWhatsApp{WhatsAppNumber: "+5511987654321"},
				).Return(output.UpsertUserByWhatsApp{}, errors.New("unexpected db error")).Once()
			},
			expect: func(response *httptest.ResponseRecorder, obs *recordingObservability) {
				s.Equal(http.StatusInternalServerError, response.Code)
				field, ok := obs.logger.findField("whatsapp_masked")
				s.Require().True(ok)
				s.Contains(field.StringValue(), "9****-4321")
				s.False(obs.logger.hasStringValue("+5511987654321"))
			},
		},
		{
			name: "deve evitar vazamento do whatsapp em conflito",
			args: args{
				request: s.request(map[string]string{
					"whatsapp": "+5511987654321",
				}),
			},
			setup: func(deps dependencies) {
				deps.useCase.EXPECT().Execute(
					mock.Anything,
					input.UpsertUserByWhatsApp{WhatsAppNumber: "+5511987654321"},
				).Return(output.UpsertUserByWhatsApp{}, application.ErrWhatsAppNumberInUse).Once()
			},
			expect: func(response *httptest.ResponseRecorder, obs *recordingObservability) {
				s.Equal(http.StatusConflict, response.Code)
				s.False(obs.logger.hasStringValue("+5511987654321"))
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			deps := dependencies{
				useCase: handlermocks.NewMockUpsertUseCase(s.T()),
				obs:     s.newObservability(),
			}
			scenario.setup(deps)

			sut := handlers.NewUpsertUserByWhatsAppHandler(deps.useCase, deps.obs)
			response := httptest.NewRecorder()
			sut.Handle(response, scenario.args.request)

			scenario.expect(response, deps.obs)
		})
	}
}
