package handlers_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	ucmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/server/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/server/middleware"
)

const testWebhookSecret = "test-secret"

type KiwifyWebhookHandlerSuite struct {
	suite.Suite
	saleApproved *ucmocks.ProcessSaleApproved
	subRenewed   *ucmocks.ProcessSubscriptionRenewed
	subLate      *ucmocks.ProcessSubscriptionLate
	subCanceled  *ucmocks.ProcessSubscriptionCanceled
	refund       *ucmocks.ProcessRefundOrChargeback
	eventRepo    *ucmocks.KiwifyEventRepository
}

func TestKiwifyWebhookHandlerSuite(t *testing.T) {
	suite.Run(t, new(KiwifyWebhookHandlerSuite))
}

func (s *KiwifyWebhookHandlerSuite) SetupTest() {
	s.saleApproved = ucmocks.NewProcessSaleApproved(s.T())
	s.subRenewed = ucmocks.NewProcessSubscriptionRenewed(s.T())
	s.subLate = ucmocks.NewProcessSubscriptionLate(s.T())
	s.subCanceled = ucmocks.NewProcessSubscriptionCanceled(s.T())
	s.refund = ucmocks.NewProcessRefundOrChargeback(s.T())
	s.eventRepo = ucmocks.NewKiwifyEventRepository(s.T())
}

func (s *KiwifyWebhookHandlerSuite) newHandler(secretNext string) http.Handler {
	o11y := noop.NewProvider()
	uc := usecases.NewProcessKiwifyWebhook(
		s.saleApproved,
		s.subRenewed,
		s.subLate,
		s.subCanceled,
		s.refund,
		s.eventRepo,
		o11y,
	)
	h := handlers.NewKiwifyWebhookHandler(uc, o11y)

	return middleware.RawBody(
		middleware.HMACSignature(testWebhookSecret, secretNext)(
			http.HandlerFunc(h.Handle),
		),
	)
}

func signPayload(payload []byte, secret string) string {
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func buildRequest(t *testing.T, payload []byte) *http.Request {
	t.Helper()
	sig := signPayload(payload, testWebhookSecret)
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/billing/webhooks/kiwify?signature="+sig,
		strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func kiwifyPayload(eventType string, extra map[string]any) []byte {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	data := map[string]any{
		"order_id":           "order-" + eventType,
		"order_ref":          "ref-001",
		"order_status":       "paid",
		"webhook_event_type": eventType,
		"subscription_id":    "sub-abc",
		"Product":            map[string]any{"product_id": "prod-456", "product_name": "Test Plan"},
		"Customer": map[string]any{
			"email":  "test+webhook@example.com",
			"mobile": "+5511900000000",
			"CPF":    "00000000000",
		},
		"Subscription": map[string]any{
			"status":       "active",
			"start_date":   "2026-06-08T14:53:19.679Z",
			"next_payment": "2026-07-08T14:53:23.137Z",
		},
		"TrackingParameters": map[string]any{"sck": "funnel-token-abc", "s1": nil, "src": nil},
		"approved_date":      now,
		"updated_at":         now,
		"created_at":         now,
	}
	maps.Copy(data, extra)
	raw, _ := json.Marshal(data)
	return raw
}

func abandonedCartPayload() []byte {
	return []byte(`{"checkout_link":"IDhfYNV","country":"br","cpf":"30574187242","created_at":"2026-06-05T15:44:25.411Z","email":"johndoe@example.com","id":"c6euk9v1lfj9jqxhfs","name":"John Doe","offer_name":null,"phone":"(10) 5467-4999","product_id":"0f3bf47c-6011-4ea4-8aeb-0e7d744f4acc","product_name":"Example product","status":"abandoned","store_id":"Q33AnzwYbfkFFwS","subscription_plan":"Example subscription"}`)
}

func (s *KiwifyWebhookHandlerSuite) expectAudit(expectedStatus string) *string {
	captured := ""
	s.eventRepo.EXPECT().
		Persist(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, envelopeID string, trigger string, rawBody []byte, signatureStatus string) error {
			captured = signatureStatus
			return nil
		}).
		Maybe()
	if expectedStatus == "" {
		return nil
	}
	return &captured
}

func (s *KiwifyWebhookHandlerSuite) TestKiwifyWebhookHandler() {
	scenarios := []struct {
		name         string
		payload      []byte
		buildRequest func([]byte) *http.Request
		secretNext   string
		setup        func()
		expectStatus int
		expectAudit  string
		expectBody   bool
	}{
		{
			name:         "deve responder 422 quando kiwify subscription id estiver ausente em order_approved",
			payload:      kiwifyPayload("order_approved", map[string]any{"subscription_id": "  "}),
			buildRequest: func(payload []byte) *http.Request { return buildRequest(s.T(), payload) },
			setup: func() {
				s.saleApproved.EXPECT().Execute(mock.Anything, mock.Anything).Return(usecases.ErrKiwifySubscriptionIDInvalid).Once()
			},
			expectStatus: http.StatusUnprocessableEntity,
		},
		{
			name:         "deve responder 202 para order_approved",
			payload:      kiwifyPayload("order_approved", nil),
			buildRequest: func(payload []byte) *http.Request { return buildRequest(s.T(), payload) },
			setup: func() {
				s.saleApproved.EXPECT().Execute(mock.Anything, mock.Anything).Return(nil).Once()
			},
			expectStatus: http.StatusAccepted,
		},
		{
			name:    "deve responder 401 para assinatura invalida",
			payload: kiwifyPayload("order_approved", nil),
			buildRequest: func(payload []byte) *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/?signature=wrongsig", strings.NewReader(string(payload)))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			expectStatus: http.StatusUnauthorized,
		},
		{
			name:    "deve rejeitar assinatura invalida no middleware",
			payload: kiwifyPayload("order_approved", nil),
			buildRequest: func(payload []byte) *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/?signature=wrong", strings.NewReader(string(payload)))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			expectStatus: http.StatusUnauthorized,
		},
		{
			name:    "deve responder 415 para content type invalido",
			payload: kiwifyPayload("order_approved", nil),
			buildRequest: func(payload []byte) *http.Request {
				sig := signPayload(payload, testWebhookSecret)
				req := httptest.NewRequest(http.MethodPost, "/?signature="+sig, strings.NewReader(string(payload)))
				req.Header.Set("Content-Type", "text/plain")
				return req
			},
			expectStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:    "deve responder 413 para corpo acima do limite",
			payload: make([]byte, 256*1024+1),
			buildRequest: func(payload []byte) *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(payload)))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			expectStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name:         "deve responder 422 quando funnel token estiver ausente",
			payload:      kiwifyPayload("order_approved", map[string]any{"TrackingParameters": map[string]any{"sck": nil, "s1": nil, "src": nil}}),
			buildRequest: func(payload []byte) *http.Request { return buildRequest(s.T(), payload) },
			setup: func() {
				s.saleApproved.EXPECT().Execute(mock.Anything, mock.Anything).Return(usecases.ErrFunnelTokenMissing).Once()
			},
			expectStatus: http.StatusUnprocessableEntity,
		},
		{
			name:         "deve responder 422 para trigger desconhecido",
			payload:      kiwifyPayload("trigger_desconhecido", nil),
			buildRequest: func(payload []byte) *http.Request { return buildRequest(s.T(), payload) },
			expectStatus: http.StatusUnprocessableEntity,
		},
		{
			name:         "deve responder 202 quando o evento ja tiver sido processado",
			payload:      kiwifyPayload("order_approved", nil),
			buildRequest: func(payload []byte) *http.Request { return buildRequest(s.T(), payload) },
			setup: func() {
				s.saleApproved.EXPECT().Execute(mock.Anything, mock.Anything).Return(usecases.ErrEventAlreadyProcessed).Once()
			},
			expectStatus: http.StatusAccepted,
		},
		{
			name:         "deve responder 202 para subscription_renewed",
			payload:      kiwifyPayload("subscription_renewed", nil),
			buildRequest: func(payload []byte) *http.Request { return buildRequest(s.T(), payload) },
			setup: func() {
				s.subRenewed.EXPECT().Execute(mock.Anything, mock.Anything).Return(nil).Once()
			},
			expectStatus: http.StatusAccepted,
		},
		{
			name:         "deve responder 202 para subscription_late",
			payload:      kiwifyPayload("subscription_late", nil),
			buildRequest: func(payload []byte) *http.Request { return buildRequest(s.T(), payload) },
			setup: func() {
				s.subLate.EXPECT().Execute(mock.Anything, mock.Anything).Return(nil).Once()
			},
			expectStatus: http.StatusAccepted,
		},
		{
			name:         "deve responder 202 para subscription_canceled",
			payload:      kiwifyPayload("subscription_canceled", nil),
			buildRequest: func(payload []byte) *http.Request { return buildRequest(s.T(), payload) },
			setup: func() {
				s.subCanceled.EXPECT().Execute(mock.Anything, mock.Anything).Return(nil).Once()
			},
			expectStatus: http.StatusAccepted,
		},
		{
			name:         "deve responder 202 para order_refunded",
			payload:      kiwifyPayload("order_refunded", nil),
			buildRequest: func(payload []byte) *http.Request { return buildRequest(s.T(), payload) },
			setup: func() {
				s.refund.EXPECT().Execute(mock.Anything, mock.Anything).Return(nil).Once()
			},
			expectStatus: http.StatusAccepted,
		},
		{
			name:         "deve responder 202 para chargeback",
			payload:      kiwifyPayload("chargeback", nil),
			buildRequest: func(payload []byte) *http.Request { return buildRequest(s.T(), payload) },
			setup: func() {
				s.refund.EXPECT().Execute(mock.Anything, mock.Anything).Return(nil).Once()
			},
			expectStatus: http.StatusAccepted,
		},
		{
			name:         "deve responder 202 no-op para billet_created",
			payload:      kiwifyPayload("billet_created", nil),
			buildRequest: func(payload []byte) *http.Request { return buildRequest(s.T(), payload) },
			expectStatus: http.StatusAccepted,
		},
		{
			name:         "deve responder 202 no-op para pix_created",
			payload:      kiwifyPayload("pix_created", nil),
			buildRequest: func(payload []byte) *http.Request { return buildRequest(s.T(), payload) },
			expectStatus: http.StatusAccepted,
		},
		{
			name:         "deve responder 202 no-op para order_rejected",
			payload:      kiwifyPayload("order_rejected", nil),
			buildRequest: func(payload []byte) *http.Request { return buildRequest(s.T(), payload) },
			expectStatus: http.StatusAccepted,
		},
		{
			name:         "deve responder 202 no-op para carrinho abandonado",
			payload:      abandonedCartPayload(),
			buildRequest: func(payload []byte) *http.Request { return buildRequest(s.T(), payload) },
			expectStatus: http.StatusAccepted,
		},
		{
			name:       "deve responder 202 com segredo rotacionado",
			payload:    kiwifyPayload("order_approved", nil),
			secretNext: "secret-next",
			buildRequest: func(payload []byte) *http.Request {
				sig := signPayload(payload, "secret-next")
				req := httptest.NewRequest(http.MethodPost, "/?signature="+sig, strings.NewReader(string(payload)))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			setup: func() {
				s.saleApproved.EXPECT().Execute(mock.Anything, mock.Anything).Return(nil).Once()
			},
			expectStatus: http.StatusAccepted,
			expectBody:   true,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			capturedAudit := s.expectAudit(scenario.expectAudit)
			if scenario.setup != nil {
				scenario.setup()
			}

			handler := s.newHandler(scenario.secretNext)
			req := scenario.buildRequest(scenario.payload)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.Equal(s.T(), scenario.expectStatus, rr.Code)
			if capturedAudit != nil {
				assert.Equal(s.T(), scenario.expectAudit, *capturedAudit)
			}
			if scenario.expectBody {
				var body map[string]any
				require.NoError(s.T(), json.NewDecoder(rr.Body).Decode(&body))
				assert.Equal(s.T(), true, body["received"])
			}
		})
	}
}
