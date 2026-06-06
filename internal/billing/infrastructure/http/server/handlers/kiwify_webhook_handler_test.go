package handlers_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/server/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/server/middleware"
)

const testWebhookSecret = "test-secret"

type stubSaleApproved struct{ err error }

func (s *stubSaleApproved) Execute(_ context.Context, _ input.ProcessSaleApprovedInput) error {
	return s.err
}

type stubSubRenewed struct{ err error }

func (s *stubSubRenewed) Execute(_ context.Context, _ input.ProcessSubscriptionRenewedInput) error {
	return s.err
}

type stubSubLate struct{ err error }

func (s *stubSubLate) Execute(_ context.Context, _ input.ProcessSubscriptionLateInput) error {
	return s.err
}

type stubSubCanceled struct{ err error }

func (s *stubSubCanceled) Execute(_ context.Context, _ input.ProcessSubscriptionCanceledInput) error {
	return s.err
}

type stubRefund struct{ err error }

func (s *stubRefund) Execute(_ context.Context, _ input.ProcessRefundOrChargebackInput) error {
	return s.err
}

type stubKiwifyEventRepo struct {
	signatureStatus string
}

func (s *stubKiwifyEventRepo) Persist(_ context.Context, _ string, _ string, _ []byte, signatureStatus string) error {
	s.signatureStatus = signatureStatus
	return nil
}
func (s *stubKiwifyEventRepo) MarkProcessed(_ context.Context, _ string, _ time.Time) error {
	return nil
}
func (s *stubKiwifyEventRepo) DeleteOlderThan(_ context.Context, _ time.Time, _ int) (int64, error) {
	return 0, nil
}

type stubRepositoryFactory struct {
	kiwifyRepo interfaces.KiwifyEventRepository
}

func (f *stubRepositoryFactory) SubscriptionRepository(_ database.DBTX) interfaces.SubscriptionRepository {
	return nil
}
func (f *stubRepositoryFactory) ProcessedEventRepository(_ database.DBTX) interfaces.ProcessedEventRepository {
	return nil
}
func (f *stubRepositoryFactory) KiwifyEventRepository(_ database.DBTX) interfaces.KiwifyEventRepository {
	if f.kiwifyRepo != nil {
		return f.kiwifyRepo
	}
	return &stubKiwifyEventRepo{}
}
func (f *stubRepositoryFactory) PlanRepository(_ database.DBTX) interfaces.PlanRepository {
	return nil
}
func (f *stubRepositoryFactory) ReconciliationCheckpointRepository(_ database.DBTX) interfaces.ReconciliationCheckpointRepository {
	return nil
}

func buildTestHandler(
	saleErr, renewedErr, lateErr, canceledErr, refundErr error,
) http.Handler {
	o11y := noop.NewProvider()
	uc := usecases.NewProcessKiwifyWebhook(
		&stubSaleApproved{err: saleErr},
		&stubSubRenewed{err: renewedErr},
		&stubSubLate{err: lateErr},
		&stubSubCanceled{err: canceledErr},
		&stubRefund{err: refundErr},
		&stubRepositoryFactory{},
		nil,
		o11y,
	)
	h := handlers.NewKiwifyWebhookHandler(uc, o11y)

	return middleware.RawBody(
		middleware.HMACSignature(testWebhookSecret, "")(
			http.HandlerFunc(h.Handle),
		),
	)
}

func signPayload(payload []byte) string {
	mac := hmac.New(sha256.New, []byte(testWebhookSecret))
	mac.Write(payload)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func buildRequest(t *testing.T, payload []byte) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/billing/webhooks/kiwify", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Kiwify-Signature", signPayload(payload))
	return req
}

func kiwifyPayload(trigger string, extra map[string]any) []byte {
	data := map[string]any{
		"id":         fmt.Sprintf("sale-%s", trigger),
		"order_id":   "order-123",
		"product_id": "prod-456",
		"updated_at": time.Now().UTC().Format(time.RFC3339),
		"tracking":   map[string]any{"s1": "funnel-token-abc"},
	}
	maps.Copy(data, extra)
	env := map[string]any{
		"id":      "env-001",
		"trigger": trigger,
		"data":    data,
	}
	raw, _ := json.Marshal(env)
	return raw
}

func TestKiwifyWebhookHandler_202_CompraAprovada(t *testing.T) {
	handler := buildTestHandler(nil, nil, nil, nil, nil)
	payload := kiwifyPayload("compra_aprovada", nil)

	req := buildRequest(t, payload)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
}

func TestKiwifyWebhookHandler_401_InvalidSignature(t *testing.T) {
	handler := buildTestHandler(nil, nil, nil, nil, nil)
	payload := kiwifyPayload("compra_aprovada", nil)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Kiwify-Signature", "wrongsig")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestKiwifyWebhookHandler_401_InvalidSignatureIsAudited(t *testing.T) {
	repo := &stubKiwifyEventRepo{}
	uc := usecases.NewProcessKiwifyWebhook(
		&stubSaleApproved{},
		&stubSubRenewed{},
		&stubSubLate{},
		&stubSubCanceled{},
		&stubRefund{},
		&stubRepositoryFactory{kiwifyRepo: repo},
		nil,
		noop.NewProvider(),
	)
	h := handlers.NewKiwifyWebhookHandler(uc, noop.NewProvider())
	handler := middleware.RawBody(
		middleware.HMACSignature(testWebhookSecret, "")(
			http.HandlerFunc(h.Handle),
		),
	)
	payload := kiwifyPayload("compra_aprovada", nil)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Kiwify-Signature", "wrong")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Equal(t, middleware.SignatureStatusInvalid, repo.signatureStatus)
}

func TestKiwifyWebhookHandler_415_WrongContentType(t *testing.T) {
	handler := buildTestHandler(nil, nil, nil, nil, nil)
	payload := kiwifyPayload("compra_aprovada", nil)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("X-Kiwify-Signature", signPayload(payload))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnsupportedMediaType, rr.Code)
}

func TestKiwifyWebhookHandler_413_BodyTooLarge(t *testing.T) {
	handler := buildTestHandler(nil, nil, nil, nil, nil)

	big := make([]byte, 256*1024+1)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(big)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
}

func TestKiwifyWebhookHandler_422_FunnelTokenMissing(t *testing.T) {
	handler := buildTestHandler(usecases.ErrFunnelTokenMissing, nil, nil, nil, nil)
	payload := kiwifyPayload("compra_aprovada", map[string]any{
		"tracking": map[string]any{"s1": ""},
	})

	req := buildRequest(t, payload)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
}

func TestKiwifyWebhookHandler_422_UnknownTrigger(t *testing.T) {
	handler := buildTestHandler(nil, nil, nil, nil, nil)
	payload := kiwifyPayload("trigger_desconhecido", nil)

	req := buildRequest(t, payload)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
}

func TestKiwifyWebhookHandler_202_Idempotent(t *testing.T) {
	handler := buildTestHandler(usecases.ErrEventAlreadyProcessed, nil, nil, nil, nil)
	payload := kiwifyPayload("compra_aprovada", nil)

	req := buildRequest(t, payload)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
}

func TestKiwifyWebhookHandler_202_SubscriptionRenewed(t *testing.T) {
	handler := buildTestHandler(nil, nil, nil, nil, nil)
	payload := kiwifyPayload("subscription_renewed", map[string]any{
		"subscription": map[string]any{
			"id":         "sub-abc",
			"updated_at": time.Now().UTC().Format(time.RFC3339),
		},
	})

	req := buildRequest(t, payload)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
}

func TestKiwifyWebhookHandler_202_SubscriptionLate(t *testing.T) {
	handler := buildTestHandler(nil, nil, nil, nil, nil)
	payload := kiwifyPayload("subscription_late", nil)

	req := buildRequest(t, payload)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
}

func TestKiwifyWebhookHandler_202_SubscriptionCanceled(t *testing.T) {
	handler := buildTestHandler(nil, nil, nil, nil, nil)
	payload := kiwifyPayload("subscription_canceled", nil)

	req := buildRequest(t, payload)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
}

func TestKiwifyWebhookHandler_202_Chargeback(t *testing.T) {
	handler := buildTestHandler(nil, nil, nil, nil, nil)
	payload := kiwifyPayload("chargeback", nil)

	req := buildRequest(t, payload)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
}

func TestKiwifyWebhookHandler_202_RotatedSecret(t *testing.T) {
	const secretNext = "secret-next"
	o11y := noop.NewProvider()
	uc := usecases.NewProcessKiwifyWebhook(
		&stubSaleApproved{},
		&stubSubRenewed{},
		&stubSubLate{},
		&stubSubCanceled{},
		&stubRefund{},
		&stubRepositoryFactory{},
		nil,
		o11y,
	)
	h := handlers.NewKiwifyWebhookHandler(uc, o11y)
	handler := middleware.RawBody(
		middleware.HMACSignature(testWebhookSecret, secretNext)(
			http.HandlerFunc(h.Handle),
		),
	)

	payload := kiwifyPayload("compra_aprovada", nil)
	mac := hmac.New(sha256.New, []byte(secretNext))
	mac.Write(payload)
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Kiwify-Signature", sig)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)

	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, true, body["received"])
}
