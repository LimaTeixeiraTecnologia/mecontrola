//go:build integration

package handlers_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/migration"
	dbpostgres "github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/server/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/server/middleware"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/messaging/database/producers"
	billingrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	outboxrepo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/migrations"
)

const (
	integWebhookSecret = "integ-test-secret"
	pgImage            = "postgres:16"
)

type WebhookIntegSuite struct {
	suite.Suite
	mgr             manager.Manager
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

	req := tc.ContainerRequest{
		Image:        pgImage,
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}

	container, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	s.Require().NoError(err)

	s.T().Cleanup(func() {
		if terr := container.Terminate(context.Background()); terr != nil {
			s.T().Logf("container terminate: %v", terr)
		}
	})

	host, err := container.Host(ctx)
	s.Require().NoError(err)

	mapped, err := container.MappedPort(ctx, "5432")
	s.Require().NoError(err)

	portNum, err := strconv.Atoi(mapped.Port())
	s.Require().NoError(err)

	cfg := dbpostgres.PostgresConfig{
		DSN: fmt.Sprintf("postgres://test:test@%s:%d/testdb?sslmode=disable&search_path=mecontrola,public", host, portNum),
	}

	mgr, err := manager.New(cfg)
	s.Require().NoError(err)
	s.mgr = mgr

	s.T().Cleanup(func() {
		_ = s.mgr.Shutdown(context.Background())
	})

	dsn := fmt.Sprintf("pgx5://test:test@%s:%d/testdb?sslmode=disable", host, portNum)
	migrator, err := migration.New(s.mgr, migration.EmbedFS{FS: migrations.FS, Root: "."}, migration.WithDSN(dsn))
	s.Require().NoError(err)

	if err := migrator.Up(ctx); err != nil && !errors.Is(err, migration.ErrNoChange) {
		s.Require().NoError(err)
	}

	o11y := noop.NewProvider()
	s.factory = billingrepos.NewRepositoryFactory(o11y)
	outboxFactory := outboxrepo.NewRepositoryFactory(o11y)
	outboxCfg := configs.OutboxConfig{RetryMaxAttempts: 5}
	idGen := id.NewUUIDGenerator()

	publisher := producers.NewSubscriptionEventPublisher(outboxFactory, outboxCfg, idGen)
	saleUoW := uow.New[entities.Subscription](s.mgr, uow.WithObservability(o11y))
	renewedUoW := uow.New[entities.Subscription](s.mgr, uow.WithObservability(o11y))
	lateUoW := uow.New[entities.Subscription](s.mgr, uow.WithObservability(o11y))
	canceledUoW := uow.New[entities.Subscription](s.mgr, uow.WithObservability(o11y))
	refundUoW := uow.New[entities.Subscription](s.mgr, uow.WithObservability(o11y))

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
		s.factory,
		s.mgr.DBTX(context.Background()),
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
	row := s.mgr.DBTX(ctx).QueryRowContext(ctx, `SELECT kiwify_product_id FROM billing_plans WHERE code='MONTHLY' LIMIT 1`)
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
	identityModule := identity.NewIdentityModule(&configs.Config{}, o11y, s.mgr)
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
		uow.New[[]outboxrepo.Row](s.mgr, uow.WithObservability(o11y)),
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
			row := s.mgr.DBTX(ctx).QueryRowContext(ctx, `SELECT COUNT(*) FROM billing_kiwify_events WHERE envelope_id = $1 AND signature_status = 'valid'`, orderID)
			s.Require().NoError(row.Scan(&kiwifyCount))
			s.Equal(1, kiwifyCount, "esperado 1 registro de auditoria para trigger no-op em billing_kiwify_events")

			var subCount int
			row = s.mgr.DBTX(ctx).QueryRowContext(ctx, `SELECT COUNT(*) FROM billing_subscriptions WHERE kiwify_order_id = $1`, orderID)
			s.Require().NoError(row.Scan(&subCount))
			s.Equal(0, subCount, "trigger no-op nao deve criar subscription")

			var procCount int
			row = s.mgr.DBTX(ctx).QueryRowContext(ctx, `SELECT COUNT(*) FROM billing_processed_events WHERE recurso_id = $1`, orderID)
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
			row := s.mgr.DBTX(ctx).QueryRowContext(ctx, `SELECT COUNT(*) FROM billing_subscriptions WHERE kiwify_order_id = $1`, orderID)
			s.Require().NoError(row.Scan(&subCount))
			s.Equal(1, subCount)

			var procCount int
			row = s.mgr.DBTX(ctx).QueryRowContext(ctx, `SELECT COUNT(*) FROM billing_processed_events WHERE recurso_id = $1`, orderID)
			s.Require().NoError(row.Scan(&procCount))
			s.Equal(1, procCount)

			var outboxCount int
			row = s.mgr.DBTX(ctx).QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_events WHERE event_type = $1`, producers.EventTypeSubscriptionActivated)
			s.Require().NoError(row.Scan(&outboxCount))
			s.GreaterOrEqual(outboxCount, 1)

			var kiwifyCount int
			row = s.mgr.DBTX(ctx).QueryRowContext(ctx, `SELECT COUNT(*) FROM billing_kiwify_events WHERE envelope_id = $1`, orderID)
			s.Require().NoError(row.Scan(&kiwifyCount))
			s.Equal(1, kiwifyCount)

			s.dispatchOutbox(ctx)

			var pendingCount int
			row = s.mgr.DBTX(ctx).QueryRowContext(ctx, `SELECT COUNT(*) FROM identity_entitlements_pending WHERE subscription_id = (
				SELECT id FROM billing_subscriptions WHERE kiwify_order_id = $1
			)`, orderID)
			s.Require().NoError(row.Scan(&pendingCount))
			s.Equal(1, pendingCount)
		})
	}
}
