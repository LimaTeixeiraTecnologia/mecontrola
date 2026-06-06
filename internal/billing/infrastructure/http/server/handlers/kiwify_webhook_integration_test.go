//go:build integration

package handlers_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
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
		Host:     host,
		Port:     portNum,
		User:     "test",
		Password: "test",
		Database: "testdb",
		SSLMode:  "disable",
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
	mac := hmac.New(sha256.New, []byte(integWebhookSecret))
	mac.Write(payload)
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/billing/webhooks/kiwify", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Kiwify-Signature", sig)
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

func (s *WebhookIntegSuite) TestWebhookToOutbox_CompraAprovada_202_OneSubOneProcessedOneOutbox() {
	ctx := context.Background()

	saleID := fmt.Sprintf("sale-integ-webhook-%d", time.Now().UnixNano())
	orderID := fmt.Sprintf("order-integ-webhook-%d", time.Now().UnixNano())
	envelopeID := fmt.Sprintf("env-integ-%d", time.Now().UnixNano())

	data := map[string]any{
		"id":         saleID,
		"order_id":   orderID,
		"product_id": s.kiwifyProductID,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
		"tracking":   map[string]any{"s1": "funnel-token-integ"},
	}
	env := map[string]any{
		"id":      envelopeID,
		"trigger": "compra_aprovada",
		"data":    data,
	}
	payload, err := json.Marshal(env)
	s.Require().NoError(err)

	req := s.buildSignedRequest(payload)
	rr := httptest.NewRecorder()
	s.webhookHandler.ServeHTTP(rr, req)

	s.Equal(http.StatusAccepted, rr.Code)

	var subCount int
	row := s.mgr.DBTX(ctx).QueryRowContext(ctx,
		`SELECT COUNT(*) FROM billing_subscriptions WHERE kiwify_order_id = $1`, orderID)
	s.Require().NoError(row.Scan(&subCount))
	s.Equal(1, subCount, "expected 1 subscription row")

	var procCount int
	row = s.mgr.DBTX(ctx).QueryRowContext(ctx,
		`SELECT COUNT(*) FROM billing_processed_events WHERE recurso_id = $1`, saleID)
	s.Require().NoError(row.Scan(&procCount))
	s.Equal(1, procCount, "expected 1 processed_event row")

	var outboxCount int
	row = s.mgr.DBTX(ctx).QueryRowContext(ctx,
		`SELECT COUNT(*) FROM outbox_events WHERE event_type = $1`,
		producers.EventTypeSubscriptionActivated)
	s.Require().NoError(row.Scan(&outboxCount))
	s.GreaterOrEqual(outboxCount, 1, "expected at least 1 outbox row")

	var kiwifyCount int
	row = s.mgr.DBTX(ctx).QueryRowContext(ctx,
		`SELECT COUNT(*) FROM billing_kiwify_events WHERE envelope_id = $1`, envelopeID)
	s.Require().NoError(row.Scan(&kiwifyCount))
	s.Equal(1, kiwifyCount, "expected 1 kiwify_events row")

	s.dispatchOutbox(ctx)

	var pendingCount int
	row = s.mgr.DBTX(ctx).QueryRowContext(ctx,
		`SELECT COUNT(*) FROM identity_entitlements_pending WHERE subscription_id = (
			SELECT id FROM billing_subscriptions WHERE kiwify_order_id = $1
		)`, orderID)
	s.Require().NoError(row.Scan(&pendingCount))
	s.Equal(1, pendingCount, "expected dispatcher to project subscription into identity pending")
}
