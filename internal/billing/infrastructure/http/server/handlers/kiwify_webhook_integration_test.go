//go:build integration

package handlers_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/server/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/server/middleware"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/messaging/database/producers"
	billingrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	outboxrepo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

const integWebhookSecret = "integ-test-secret"

type WebhookIntegSuite struct {
	suite.Suite
	db              *sqlx.DB
	factory         interfaces.RepositoryFactory
	webhookHandler  http.Handler
	kiwifyProductID string
}

func TestWebhookIntegSuite(t *testing.T) {
	suite.Run(t, new(WebhookIntegSuite))
}

func (s *WebhookIntegSuite) SetupTest() {}

func (s *WebhookIntegSuite) SetupSuite() {
	ctx := context.Background()

	db, _ := testcontainer.Postgres(s.T())
	s.db = db

	o11y := noop.NewProvider()
	s.factory = billingrepos.NewRepositoryFactory(o11y)
	outboxFactory := outboxrepo.NewRepositoryFactory(o11y)
	outboxCfg := configs.OutboxConfig{RetryMaxAttempts: 5}
	idGen := id.NewUUIDGenerator()

	publisher := producers.NewSubscriptionEventPublisher(outboxFactory, outboxCfg, idGen, noop.NewProvider())
	saleUoW := uow.NewUnitOfWork(s.db)
	renewedUoW := uow.NewUnitOfWork(s.db)
	lateUoW := uow.NewUnitOfWork(s.db)
	canceledUoW := uow.NewUnitOfWork(s.db)
	refundUoW := uow.NewUnitOfWork(s.db)

	processSale := usecases.NewProcessSaleApproved(saleUoW, s.factory, publisher, o11y)
	processRenewed := usecases.NewProcessSubscriptionRenewed(renewedUoW, s.factory, publisher, o11y)
	processLate := usecases.NewProcessSubscriptionLate(lateUoW, s.factory, publisher, o11y)
	processCanceled := usecases.NewProcessSubscriptionCanceled(canceledUoW, s.factory, publisher, o11y)
	processRefund := usecases.NewProcessRefundOrChargeback(refundUoW, s.factory, publisher, o11y)
	processWebhook := usecases.NewProcessKiwifyWebhook(
		processSale,
		processRenewed,
		processLate,
		processCanceled,
		processRefund,
		s.factory.KiwifyEventRepository(s.db),
		o11y,
	)

	h := handlers.NewKiwifyWebhookHandler(processWebhook, o11y)

	s.webhookHandler = middleware.RawBody(
		middleware.HMACSignature(integWebhookSecret, "")(
			http.HandlerFunc(h.Handle),
		),
	)

	s.seedKiwifyProductID(ctx)
}

func (s *WebhookIntegSuite) seedKiwifyProductID(ctx context.Context) {
	row := s.db.QueryRowContext(ctx, `SELECT kiwify_product_id FROM billing_plans WHERE code='MONTHLY' LIMIT 1`)
	var pid string
	s.Require().NoError(row.Scan(&pid))
	s.kiwifyProductID = pid
}

func (s *WebhookIntegSuite) buildSignedRequest(payload []byte) *http.Request {
	mac := hmac.New(sha1.New, []byte(integWebhookSecret))
	mac.Write(payload)
	sig := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/billing/webhooks/kiwify?signature="+sig,
		strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func (s *WebhookIntegSuite) dispatchOutbox(ctx context.Context) {
	o11y := noop.NewProvider()
	dispatcher := events.NewDispatcher()
	identityModule, err := identity.NewIdentityModule(&configs.Config{}, o11y, s.db)
	s.Require().NoError(err)
	for _, registration := range identityModule.EventHandlers {
		s.Require().NoError(dispatcher.Register(registration.EventType, registration.Handler))
	}
	cfg := configs.OutboxConfig{
		DispatcherBatchSize:      50,
		DispatcherHandlerTimeout: 5 * time.Second,
		RetryMaxAttempts:         5,
		RetryBaseBackoff:         time.Second,
		RetryMaxBackoff:          time.Minute,
	}
	job := outboxrepo.NewDispatcherJob(
		uow.NewUnitOfWork(s.db),
		outboxrepo.NewRepositoryFactory(o11y),
		dispatcher,
		cfg,
		o11y.Logger(),
		rand.New(rand.NewSource(42)),
	)
	s.Require().NoError(job.Run(ctx))
}

func (s *WebhookIntegSuite) TestWebhookNoOp_TriggersForaDoMVP_Auditados202SemDispatch() {
	scenarios := []struct {
		name      string
		payload   func(orderID string) map[string]any
		noPayload []byte
	}{
		{
			name: "billet_created persiste em billing_kiwify_events sem dispatch downstream",
			payload: func(orderID string) map[string]any {
				return map[string]any{
					"order_id":           orderID,
					"webhook_event_type": "billet_created",
					"order_status":       "waiting_payment",
					"Product":            map[string]any{"product_id": s.kiwifyProductID, "product_name": "Plan"},
					"Customer":           map[string]any{"email": "a@b.com", "mobile": "+5511900000000", "CPF": "00000000000"},
					"TrackingParameters": map[string]any{"sck": "tok", "s1": nil, "src": nil},
					"updated_at":         "2026-06-08 11:53",
					"created_at":         "2026-06-08 11:53",
				}
			},
		},
		{
			name: "pix_created persiste em billing_kiwify_events sem dispatch downstream",
			payload: func(orderID string) map[string]any {
				return map[string]any{
					"order_id":           orderID,
					"webhook_event_type": "pix_created",
					"order_status":       "waiting_payment",
					"Product":            map[string]any{"product_id": s.kiwifyProductID, "product_name": "Plan"},
					"Customer":           map[string]any{"email": "a@b.com", "mobile": "+5511900000000", "CPF": "00000000000"},
					"TrackingParameters": map[string]any{"sck": "tok", "s1": nil, "src": nil},
					"updated_at":         "2026-06-08 11:53",
					"created_at":         "2026-06-08 11:53",
				}
			},
		},
		{
			name: "order_rejected persiste em billing_kiwify_events sem dispatch downstream",
			payload: func(orderID string) map[string]any {
				return map[string]any{
					"order_id":           orderID,
					"webhook_event_type": "order_rejected",
					"order_status":       "refused",
					"Product":            map[string]any{"product_id": s.kiwifyProductID, "product_name": "Plan"},
					"Customer":           map[string]any{"email": "a@b.com", "mobile": "+5511900000000", "CPF": "00000000000"},
					"TrackingParameters": map[string]any{"sck": "tok", "s1": nil, "src": nil},
					"updated_at":         "2026-06-08 11:53",
					"created_at":         "2026-06-08 11:53",
				}
			},
		},
		{
			name: "abandoned_cart sem webhook_event_type ainda persiste em billing_kiwify_events",
			payload: func(orderID string) map[string]any {
				return map[string]any{
					"id":         orderID,
					"status":     "abandoned",
					"email":      "a@b.com",
					"phone":      "+5511900000000",
					"product_id": s.kiwifyProductID,
				}
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx := context.Background()
			orderID := fmt.Sprintf("noop-%d", time.Now().UnixNano())
			payload, err := json.Marshal(scenario.payload(orderID))
			s.Require().NoError(err)
			req := s.buildSignedRequest(payload)
			rr := httptest.NewRecorder()
			s.webhookHandler.ServeHTTP(rr, req)
			s.Equal(http.StatusAccepted, rr.Code)

			var kiwifyCount int
			row := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM billing_kiwify_events WHERE envelope_id = $1 AND signature_status = 'valid'`, orderID)
			s.Require().NoError(row.Scan(&kiwifyCount))
			s.Equal(1, kiwifyCount, "esperado 1 registro de auditoria para trigger no-op em billing_kiwify_events")

			var subCount int
			row = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM billing_subscriptions WHERE kiwify_order_id = $1`, orderID)
			s.Require().NoError(row.Scan(&subCount))
			s.Equal(0, subCount, "trigger no-op nao deve criar subscription")

			var procCount int
			row = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM billing_processed_events WHERE recurso_id = $1`, orderID)
			s.Require().NoError(row.Scan(&procCount))
			s.Equal(0, procCount, "trigger no-op nao deve gerar processed_event")
		})
	}
}

func (s *WebhookIntegSuite) TestWebhookToOutbox_OrderApproved_202_OneSubOneProcessedOneOutbox() {
	scenarios := []struct {
		name string
	}{
		{name: "deve projetar webhook para outbox e identidade pendente"},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx := context.Background()
			orderID := fmt.Sprintf("order-integ-webhook-%d", time.Now().UnixNano())
			subscriptionID := fmt.Sprintf("sub-integ-%d", time.Now().UnixNano())
			now := time.Now().UTC().Format("2006-01-02 15:04:05")
			payloadMap := map[string]any{
				"order_id":           orderID,
				"order_ref":          "ref-integ",
				"order_status":       "paid",
				"webhook_event_type": "order_approved",
				"subscription_id":    subscriptionID,
				"Product":            map[string]any{"product_id": s.kiwifyProductID, "product_name": "Integration Plan"},
				"Customer": map[string]any{
					"email":  "test+integ@example.com",
					"mobile": "+5511900000000",
					"CPF":    "00000000000",
				},
				"Subscription": map[string]any{
					"status":       "active",
					"start_date":   "2026-06-08T14:53:19.679Z",
					"next_payment": "2026-07-08T14:53:23.137Z",
				},
				"TrackingParameters": map[string]any{"sck": "funnel-token-integ", "s1": nil, "src": nil},
				"approved_date":      now,
				"updated_at":         now,
				"created_at":         now,
			}
			payload, err := json.Marshal(payloadMap)
			s.Require().NoError(err)
			req := s.buildSignedRequest(payload)
			rr := httptest.NewRecorder()
			s.webhookHandler.ServeHTTP(rr, req)
			s.Equal(http.StatusAccepted, rr.Code)

			var subCount int
			row := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM billing_subscriptions WHERE kiwify_order_id = $1`, orderID)
			s.Require().NoError(row.Scan(&subCount))
			s.Equal(1, subCount)

			var procCount int
			row = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM billing_processed_events WHERE recurso_id = $1`, orderID)
			s.Require().NoError(row.Scan(&procCount))
			s.Equal(1, procCount)

			var outboxCount int
			row = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_events WHERE event_type = $1`, producers.EventTypeSubscriptionActivated)
			s.Require().NoError(row.Scan(&outboxCount))
			s.GreaterOrEqual(outboxCount, 1)

			var kiwifyCount int
			row = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM billing_kiwify_events WHERE envelope_id = $1`, orderID)
			s.Require().NoError(row.Scan(&kiwifyCount))
			s.Equal(1, kiwifyCount)

			s.dispatchOutbox(ctx)

			var pendingCount int
			row = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM identity_entitlements_pending WHERE subscription_id = (
				SELECT id FROM billing_subscriptions WHERE kiwify_order_id = $1
			)`, orderID)
			s.Require().NoError(row.Scan(&pendingCount))
			s.Equal(1, pendingCount)
		})
	}
}

func (s *WebhookIntegSuite) TestWebhookToOutbox_SubscriptionCanceled_202_StatusChangedAndOutbox() {
	ctx := context.Background()
	orderID := fmt.Sprintf("order-canceled-webhook-%d", time.Now().UnixNano())
	subscriptionID := fmt.Sprintf("sub-canceled-%d", time.Now().UnixNano())
	now := time.Now().UTC().Format("2006-01-02 15:04:05")

	approvedPayload, err := json.Marshal(map[string]any{
		"order_id":           orderID,
		"order_ref":          "ref-canceled",
		"order_status":       "paid",
		"webhook_event_type": "order_approved",
		"subscription_id":    subscriptionID,
		"Product":            map[string]any{"product_id": s.kiwifyProductID, "product_name": "Integration Plan"},
		"Customer":           map[string]any{"email": "cancel@example.com", "mobile": "+5511900000001", "CPF": "00000000000"},
		"Subscription":       map[string]any{"status": "active", "start_date": "2026-06-08T14:53:19.679Z", "next_payment": "2026-07-08T14:53:23.137Z"},
		"TrackingParameters": map[string]any{"sck": "token-canceled", "s1": nil, "src": nil},
		"approved_date":      now,
		"updated_at":         now,
		"created_at":         now,
	})
	s.Require().NoError(err)

	rr := httptest.NewRecorder()
	s.webhookHandler.ServeHTTP(rr, s.buildSignedRequest(approvedPayload))
	s.Equal(http.StatusAccepted, rr.Code)

	var expectedPeriodEnd time.Time
	row := s.db.QueryRowContext(ctx, `SELECT period_end FROM billing_subscriptions WHERE kiwify_order_id = $1`, orderID)
	s.Require().NoError(row.Scan(&expectedPeriodEnd))

	canceledPayload, err := json.Marshal(map[string]any{
		"order_id":           orderID,
		"order_ref":          "ref-canceled",
		"order_status":       "canceled",
		"webhook_event_type": "subscription_canceled",
		"subscription_id":    subscriptionID,
		"Product":            map[string]any{"product_id": s.kiwifyProductID, "product_name": "Integration Plan"},
		"Customer":           map[string]any{"email": "cancel@example.com", "mobile": "+5511900000001", "CPF": "00000000000"},
		"Subscription":       map[string]any{"status": "canceled", "start_date": "2026-06-08T14:53:19.679Z", "next_payment": "2026-07-08T14:53:23.137Z"},
		"TrackingParameters": map[string]any{"sck": "token-canceled", "s1": nil, "src": nil},
		"updated_at":         time.Now().UTC().Add(time.Minute).Format("2006-01-02 15:04:05"),
		"created_at":         now,
	})
	s.Require().NoError(err)

	rr = httptest.NewRecorder()
	s.webhookHandler.ServeHTTP(rr, s.buildSignedRequest(canceledPayload))
	s.Equal(http.StatusAccepted, rr.Code)

	var status string
	var periodEnd time.Time
	row = s.db.QueryRowContext(ctx, `SELECT status, period_end FROM billing_subscriptions WHERE kiwify_order_id = $1`, orderID)
	s.Require().NoError(row.Scan(&status, &periodEnd))
	s.Equal("CANCELED_PENDING", status)
	s.True(periodEnd.Equal(expectedPeriodEnd))

	var outboxCount int
	row = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = (SELECT id::text FROM billing_subscriptions WHERE kiwify_order_id = $1) AND event_type = $2`, orderID, producers.EventTypeSubscriptionCanceled)
	s.Require().NoError(row.Scan(&outboxCount))
	s.GreaterOrEqual(outboxCount, 1)
}

func (s *WebhookIntegSuite) TestWebhookToOutbox_SubscriptionRenewed_202_PeriodExtendedAndOutbox() {
	ctx := context.Background()
	orderID := fmt.Sprintf("order-renewed-webhook-%d", time.Now().UnixNano())
	subscriptionID := fmt.Sprintf("sub-renewed-%d", time.Now().UnixNano())
	now := time.Now().UTC().Format("2006-01-02 15:04:05")

	approvedPayload, err := json.Marshal(map[string]any{
		"order_id":           orderID,
		"order_ref":          "ref-renewed",
		"order_status":       "paid",
		"webhook_event_type": "order_approved",
		"subscription_id":    subscriptionID,
		"Product":            map[string]any{"product_id": s.kiwifyProductID, "product_name": "Integration Plan"},
		"Customer":           map[string]any{"email": "renew@example.com", "mobile": "+5511900000002", "CPF": "00000000000"},
		"Subscription":       map[string]any{"status": "active", "start_date": "2026-06-08T14:53:19.679Z", "next_payment": "2026-07-08T14:53:23.137Z"},
		"TrackingParameters": map[string]any{"sck": "token-renewed", "s1": nil, "src": nil},
		"approved_date":      now,
		"updated_at":         now,
		"created_at":         now,
	})
	s.Require().NoError(err)

	rr := httptest.NewRecorder()
	s.webhookHandler.ServeHTTP(rr, s.buildSignedRequest(approvedPayload))
	s.Equal(http.StatusAccepted, rr.Code)

	var previousPeriodEnd time.Time
	row := s.db.QueryRowContext(ctx, `SELECT period_end FROM billing_subscriptions WHERE kiwify_order_id = $1`, orderID)
	s.Require().NoError(row.Scan(&previousPeriodEnd))

	renewedPayload, err := json.Marshal(map[string]any{
		"order_id":           orderID,
		"order_ref":          "ref-renewed",
		"order_status":       "paid",
		"webhook_event_type": "subscription_renewed",
		"subscription_id":    subscriptionID,
		"Product":            map[string]any{"product_id": s.kiwifyProductID, "product_name": "Integration Plan"},
		"Customer":           map[string]any{"email": "renew@example.com", "mobile": "+5511900000002", "CPF": "00000000000"},
		"Subscription":       map[string]any{"status": "active", "start_date": "2026-06-08T14:53:19.679Z", "next_payment": "2026-08-08T14:53:23.137Z"},
		"TrackingParameters": map[string]any{"sck": "token-renewed", "s1": nil, "src": nil},
		"updated_at":         time.Now().UTC().Add(time.Minute).Format("2006-01-02 15:04:05"),
		"created_at":         now,
	})
	s.Require().NoError(err)

	rr = httptest.NewRecorder()
	s.webhookHandler.ServeHTTP(rr, s.buildSignedRequest(renewedPayload))
	s.Equal(http.StatusAccepted, rr.Code)

	var status string
	var currentPeriodEnd time.Time
	row = s.db.QueryRowContext(ctx, `SELECT status, period_end FROM billing_subscriptions WHERE kiwify_order_id = $1`, orderID)
	s.Require().NoError(row.Scan(&status, &currentPeriodEnd))
	s.Equal("ACTIVE", status)
	s.True(currentPeriodEnd.After(previousPeriodEnd))

	var outboxCount int
	row = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = (SELECT id::text FROM billing_subscriptions WHERE kiwify_order_id = $1) AND event_type = $2`, orderID, producers.EventTypeSubscriptionRenewed)
	s.Require().NoError(row.Scan(&outboxCount))
	s.GreaterOrEqual(outboxCount, 1)
}
