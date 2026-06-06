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

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/require"

	application "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/handlers"
)

type recordedLog struct {
	level  string
	msg    string
	fields []observability.Field
}

type recordingLogger struct {
	mu   sync.Mutex
	logs []recordedLog
}

func (l *recordingLogger) record(level, msg string, fields []observability.Field) {
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
func (l *recordingLogger) With(_ ...observability.Field) observability.Logger { return l }

type recordingObservability struct {
	observability.Observability
	logger *recordingLogger
}

func (o *recordingObservability) Logger() observability.Logger { return o.logger }

func newRecordingObservability() *recordingObservability {
	return &recordingObservability{
		Observability: noop.NewProvider(),
		logger:        &recordingLogger{},
	}
}

func (l *recordingLogger) findField(key string) (observability.Field, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, log := range l.logs {
		for _, f := range log.fields {
			if f.Key == key {
				return f, true
			}
		}
	}
	return observability.Field{}, false
}

func (l *recordingLogger) hasStringValue(needle string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, log := range l.logs {
		for _, f := range log.fields {
			if f.Kind() == observability.FieldKindString && strings.Contains(f.StringValue(), needle) {
				return true
			}
		}
	}
	return false
}

func TestUpsertHandler_SuccessLogsMaskedWhatsApp(t *testing.T) {
	rawWhatsApp := "+5511987654321"
	maskedFragment := "9****-4321"

	uc := &mockUpsertUseCase{
		result: output.UpsertUserByWhatsApp{
			ID:             "user-uuid",
			WhatsAppNumber: rawWhatsApp,
			Email:          "user@example.com",
			DisplayName:    "Test User",
			Status:         "active",
		},
	}
	obs := newRecordingObservability()
	h := handlers.NewUpsertUserByWhatsAppHandler(uc, obs)

	body, err := json.Marshal(map[string]string{
		"whatsapp":     rawWhatsApp,
		"email":        "user@example.com",
		"display_name": "Test User",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/identity/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Handle(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	masked, ok := obs.logger.findField("whatsapp_masked")
	require.True(t, ok, "success log must include whatsapp_masked field")
	require.Equal(t, observability.FieldKindString, masked.Kind())
	require.Contains(t, masked.StringValue(), maskedFragment,
		"masked value must reveal only the last 4 digits")
	require.NotEqual(t, rawWhatsApp, masked.StringValue(),
		"masked value must never equal the raw whatsapp number")

	require.False(t, obs.logger.hasStringValue(rawWhatsApp),
		"raw whatsapp number must never appear in any log field")
}

func TestUpsertHandler_ErrorLogsMaskedWhatsApp(t *testing.T) {
	rawWhatsApp := "+5511987654321"
	maskedFragment := "9****-4321"

	uc := &mockUpsertUseCase{err: errors.New("unexpected db error")}
	obs := newRecordingObservability()
	h := handlers.NewUpsertUserByWhatsAppHandler(uc, obs)

	body, err := json.Marshal(map[string]string{"whatsapp": rawWhatsApp})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/identity/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Handle(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)

	masked, ok := obs.logger.findField("whatsapp_masked")
	require.True(t, ok, "error log must include whatsapp_masked field")
	require.Contains(t, masked.StringValue(), maskedFragment)
	require.False(t, obs.logger.hasStringValue(rawWhatsApp),
		"raw whatsapp number must never appear in any log field")
}

func TestUpsertHandler_ConflictDoesNotLeakWhatsApp(t *testing.T) {
	rawWhatsApp := "+5511987654321"

	uc := &mockUpsertUseCase{err: application.ErrWhatsAppNumberInUse}
	obs := newRecordingObservability()
	h := handlers.NewUpsertUserByWhatsAppHandler(uc, obs)

	body, err := json.Marshal(map[string]string{"whatsapp": rawWhatsApp})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/identity/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Handle(rr, req)

	require.Equal(t, http.StatusConflict, rr.Code)
	require.False(t, obs.logger.hasStringValue(rawWhatsApp),
		"conflict path must not log raw whatsapp number")
}
