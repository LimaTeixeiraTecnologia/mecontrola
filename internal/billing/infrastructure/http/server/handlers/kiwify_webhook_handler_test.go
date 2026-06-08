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
	factory      *ucmocks.RepositoryFactory
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
	s.factory = ucmocks.NewRepositoryFactory(s.T())
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
		s.factory,
		nil,
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
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func buildRequest(t *testing.T, payload []byte) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/billing/webhooks/kiwify", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Kiwify-Signature", signPayload(payload, testWebhookSecret))
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

	envelope := map[string]any{
		"id":      "env-001",
		"trigger": trigger,
		"data":    data,
	}
	raw, _ := json.Marshal(envelope)
	return raw
}

func (s *KiwifyWebhookHandlerSuite) expectAudit(expectedStatus string) *string {
	captured := ""
	s.factory.EXPECT().KiwifyEventRepository(mock.Anything).Return(s.eventRepo).Maybe()
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
			name:         "deve responder 202 para compra aprovada",
			payload:      kiwifyPayload("compra_aprovada", nil),
			buildRequest: func(payload []byte) *http.Request { return buildRequest(s.T(), payload) },
			setup: func() {
				s.saleApproved.EXPECT().Execute(mock.Anything, mock.Anything).Return(nil).Once()
			},
			expectStatus: http.StatusAccepted,
		},
		{
			name:    "deve responder 401 para assinatura invalida",
			payload: kiwifyPayload("compra_aprovada", nil),
			buildRequest: func(payload []byte) *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(payload)))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Kiwify-Signature", "wrongsig")
				return req
			},
			expectStatus: http.StatusUnauthorized,
		},
		{
			name:    "deve auditar assinatura invalida",
			payload: kiwifyPayload("compra_aprovada", nil),
			buildRequest: func(payload []byte) *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(payload)))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Kiwify-Signature", "wrong")
				return req
			},
			expectStatus: http.StatusUnauthorized,
			expectAudit:  middleware.SignatureStatusInvalid,
		},
		{
			name:    "deve responder 415 para content type invalido",
			payload: kiwifyPayload("compra_aprovada", nil),
			buildRequest: func(payload []byte) *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(payload)))
				req.Header.Set("Content-Type", "text/plain")
				req.Header.Set("X-Kiwify-Signature", signPayload(payload, testWebhookSecret))
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
			payload:      kiwifyPayload("compra_aprovada", map[string]any{"tracking": map[string]any{"s1": ""}}),
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
			payload:      kiwifyPayload("compra_aprovada", nil),
			buildRequest: func(payload []byte) *http.Request { return buildRequest(s.T(), payload) },
			setup: func() {
				s.saleApproved.EXPECT().Execute(mock.Anything, mock.Anything).Return(usecases.ErrEventAlreadyProcessed).Once()
			},
			expectStatus: http.StatusAccepted,
		},
		{
			name:         "deve responder 202 para subscription renewed",
			payload:      kiwifyPayload("subscription_renewed", map[string]any{"subscription": map[string]any{"id": "sub-abc", "updated_at": time.Now().UTC().Format(time.RFC3339)}}),
			buildRequest: func(payload []byte) *http.Request { return buildRequest(s.T(), payload) },
			setup: func() {
				s.subRenewed.EXPECT().Execute(mock.Anything, mock.Anything).Return(nil).Once()
			},
			expectStatus: http.StatusAccepted,
		},
		{
			name:         "deve responder 202 para subscription late",
			payload:      kiwifyPayload("subscription_late", nil),
			buildRequest: func(payload []byte) *http.Request { return buildRequest(s.T(), payload) },
			setup: func() {
				s.subLate.EXPECT().Execute(mock.Anything, mock.Anything).Return(nil).Once()
			},
			expectStatus: http.StatusAccepted,
		},
		{
			name:         "deve responder 202 para subscription canceled",
			payload:      kiwifyPayload("subscription_canceled", nil),
			buildRequest: func(payload []byte) *http.Request { return buildRequest(s.T(), payload) },
			setup: func() {
				s.subCanceled.EXPECT().Execute(mock.Anything, mock.Anything).Return(nil).Once()
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
			name:       "deve responder 202 com segredo rotacionado",
			payload:    kiwifyPayload("compra_aprovada", nil),
			secretNext: "secret-next",
			buildRequest: func(payload []byte) *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(payload)))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Kiwify-Signature", signPayload(payload, "secret-next"))
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
