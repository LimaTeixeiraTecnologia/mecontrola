//go:build integration

package usecases_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	billingrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/repositories/postgres"
	identityentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	identityvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	dbpkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability/fakes"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type ProcessBillingEventIntegrationSuite struct {
	suite.Suite
	ctx         context.Context
	mgr         *dbpkg.Manager
	webhookRepo *billingrepos.PgxWebhookEventRepository
	subRepo     *billingrepos.PgxSubscriptionRepository
	ingestUC    *usecases.IngestKiwifyWebhookUseCase
	processUC   *usecases.ProcessBillingEventUseCase
	registry    outbox.Registry
	publisher   outbox.Publisher
}

func TestProcessBillingEventIntegration(t *testing.T) {
	suite.Run(t, new(ProcessBillingEventIntegrationSuite))
}

func (s *ProcessBillingEventIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	cfg := s.startPostgres()

	mgr, err := dbpkg.NewManager(s.ctx, cfg, nil)
	s.Require().NoError(err)
	s.mgr = mgr

	s.Require().NoError(dbpkg.RunMigrations(s.ctx, s.mgr))
	s.webhookRepo = billingrepos.NewPgxWebhookEventRepository(s.mgr)
	s.subRepo = billingrepos.NewPgxSubscriptionRepository(s.mgr)
	s.buildUCs()
}

func (s *ProcessBillingEventIntegrationSuite) TearDownSuite() {
	if s.mgr != nil {
		s.Require().NoError(s.mgr.Shutdown(context.Background()))
	}
}

func (s *ProcessBillingEventIntegrationSuite) SetupTest() {
	dbtx := s.mgr.DBTX(s.ctx)
	_, err := dbtx.ExecContext(s.ctx,
		"TRUNCATE billing_event_applications, webhook_events, outbox_deliveries, outbox_events, subscriptions, users CASCADE")
	s.Require().NoError(err)
}

func (s *ProcessBillingEventIntegrationSuite) startPostgres() *configs.Config {
	container, err := tcpostgres.Run(s.ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("testuser"),
		tcpostgres.WithPassword("testpassword"),
		tcpostgres.BasicWaitStrategies(),
	)
	s.Require().NoError(err)
	s.T().Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	host, err := container.Host(s.ctx)
	s.Require().NoError(err)
	port, err := container.MappedPort(s.ctx, "5432")
	s.Require().NoError(err)

	return &configs.Config{
		DBConfig: configs.DBConfig{
			Host:     host,
			Port:     int(port.Num()),
			User:     "testuser",
			Password: "testpassword",
			Name:     "testdb",
			SSLMode:  "disable",
			MaxConns: 10,
			MinConns: 2,
		},
	}
}

func (s *ProcessBillingEventIntegrationSuite) buildUCs() {
	registry := outbox.NewRegistry()
	s.registry = registry

	subName, err := outbox.NewSubscriptionName("billing-event-processor")
	s.Require().NoError(err)
	evtName, err := events.NewEventName("billing.kiwify.received")
	s.Require().NoError(err)

	storage := outbox.NewPgxStorage(s.mgr)
	s.publisher = outbox.NewPublisher(storage, registry, nil)

	idGen := &sequentialIDGenerator{}
	clk := &staticClock{t: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)}

	userResolver := &sqlUserResolver{mgr: s.mgr, clk: clk}

	entitlementCache := &nopEntitlementCache{}
	bus := events.NewBus(events.WithBufferSize(10))

	txRunnerIngest := dbpkg.NewUnitOfWork[output.IngestWebhookResult](s.mgr)
	txRunnerProcess := dbpkg.NewUnitOfWork[usecases.ProcessBillingEventResult](s.mgr)

	parsableProvider := &kiwifyParsableProvider{clk: clk}

	s.ingestUC = usecases.NewIngestKiwifyWebhookUseCase(
		parsableProvider,
		s.webhookRepo,
		s.publisher,
		txRunnerIngest,
		idGen,
		clk,
		fakes.NoopObservability(),
		fakes.NoopUsecaseMetrics(),
	)

	processUC := usecases.NewProcessBillingEventUseCase(
		s.webhookRepo,
		s.subRepo,
		parsableProvider,
		userResolver,
		entitlementCache,
		bus,
		txRunnerProcess,
		idGen,
		clk,
		slog.Default(),
		fakes.NoopObservability(),
		fakes.NoopUsecaseMetrics(),
	)

	s.Require().NoError(registry.Register(outbox.Subscription{
		Name:      subName,
		EventType: evtName,
		Handler:   processUC.Handle,
	}))

	s.processUC = processUC
}

// buildCompraAprovadaPayload retorna payload de compra_aprovada para o número dado.
func (s *ProcessBillingEventIntegrationSuite) buildCompraAprovadaPayload(eventID string, periodEnd time.Time) []byte {
	payload := map[string]any{
		"id":                 eventID,
		"webhook_event_type": "compra_aprovada",
		"occurred_at":        periodEnd.Add(-30 * 24 * time.Hour).Format(time.RFC3339),
		"customer": map[string]any{
			"email":  "user@example.com",
			"mobile": "11999990001",
		},
		"subscription": map[string]any{
			"id":         "sub-ext-001",
			"plan":       "MONTHLY",
			"period_end": periodEnd.Format(time.RFC3339),
		},
		"product": map[string]any{
			"id": "prod-001",
		},
	}
	b, err := json.Marshal(payload)
	s.Require().NoError(err)
	return b
}

func (s *ProcessBillingEventIntegrationSuite) ingestAndProcess(payload []byte) {
	s.T().Helper()
	in := buildIngestInput(payload)
	result, err := s.ingestUC.Execute(s.ctx, in)
	s.Require().NoError(err)
	if result.Duplicate {
		return
	}

	outboxEvt, err := outbox.NewEvent(outbox.NewEventParams{
		ID:            s.mustEventID(),
		EventType:     s.mustEventName("billing.kiwify.received"),
		AggregateType: "webhook_event",
		AggregateID:   result.WebhookEventID.String(),
		Payload:       buildOutboxPayload(result.WebhookEventID.String(), "kiwify"),
		OccurredAt:    time.Now().UTC(),
	})
	s.Require().NoError(err)

	s.Require().NoError(s.processUC.Handle(s.ctx, outboxEvt))
}

func buildOutboxPayload(webhookEventID, provider string) json.RawMessage {
	p := map[string]string{"webhook_event_id": webhookEventID, "provider": provider}
	b, _ := json.Marshal(p)
	return b
}

// TestIdempotency_5xReplay — CA-02: mesmo external_event_id enviado 5x.
func (s *ProcessBillingEventIntegrationSuite) TestIdempotency_5xReplay() {
	scenarios := []struct {
		name string
	}{
		{"5 replays do mesmo external_event_id → 1 billing_event_application (CA-02)"},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			payload := s.buildCompraAprovadaPayload("evt-idempotent-001", time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))

			for range 5 {
				in := buildIngestInput(payload)
				result, err := s.ingestUC.Execute(s.ctx, in)
				s.Require().NoError(err)
				if result.Duplicate {
					continue
				}
				outboxEvt, err := outbox.NewEvent(outbox.NewEventParams{
					ID:            s.mustEventID(),
					EventType:     s.mustEventName("billing.kiwify.received"),
					AggregateType: "webhook_event",
					AggregateID:   result.WebhookEventID.String(),
					Payload:       buildOutboxPayload(result.WebhookEventID.String(), "kiwify"),
					OccurredAt:    time.Now().UTC(),
				})
				s.Require().NoError(err)
				s.Require().NoError(s.processUC.Handle(s.ctx, outboxEvt))
			}

			dbtx := s.mgr.DBTX(s.ctx)
			var appCount int
			s.Require().NoError(dbtx.QueryRowContext(s.ctx,
				"SELECT COUNT(*) FROM billing_event_applications").Scan(&appCount))
			s.Equal(1, appCount, "deve existir apenas 1 billing_event_application (idempotência CA-02)")

			var subCount int
			s.Require().NoError(dbtx.QueryRowContext(s.ctx,
				"SELECT COUNT(*) FROM subscriptions").Scan(&subCount))
			s.Equal(1, subCount, "deve existir apenas 1 subscription")
		})
	}
}

// TestOutOfOrder — CA-03: compra_aprovada (occurred_at=2026-06-01) → subscription_renewed (occurred_at=2026-06-10, period_end=2026-07-01)
// depois chega compra_aprovada duplicada com period_end stale → não deve sobrescrever period_end do renewed.
func (s *ProcessBillingEventIntegrationSuite) TestOutOfOrder() {
	scenarios := []struct {
		name string
	}{
		{"evento fora de ordem — stale é ignorado, estado final reflete maior occurred_at (CA-03)"},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			// 1. compra_aprovada inicial — cria a subscription
			initialPayload := s.buildCompraAprovadaPayload("evt-oo-initial", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
			s.ingestAndProcess(initialPayload)

			// 2. subscription_renewed com occurred_at mais recente — avança period_end para julho
			renewedPayload := s.buildRenewedPayload("evt-oo-renewed", time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC))
			s.ingestAndProcess(renewedPayload)

			// 3. compra_aprovada stale (occurred_at=2026-05-15, period_end=2026-06-14) — deve ser ignorada
			stalePayload := s.buildStaleCompraAprovadaPayload("evt-oo-stale",
				time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC),
				time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC))
			s.ingestAndProcess(stalePayload)

			dbtx := s.mgr.DBTX(s.ctx)
			var periodEnd time.Time
			s.Require().NoError(dbtx.QueryRowContext(s.ctx,
				"SELECT period_end FROM subscriptions LIMIT 1").Scan(&periodEnd))
			s.True(periodEnd.After(time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)),
				"period_end deve refletir evento mais recente (2026-07-01), não o stale")
		})
	}
}

func (s *ProcessBillingEventIntegrationSuite) buildStaleCompraAprovadaPayload(eventID string, periodEnd time.Time, occurredAt time.Time) []byte {
	payload := map[string]any{
		"id":                 eventID,
		"webhook_event_type": "compra_aprovada",
		"occurred_at":        occurredAt.Format(time.RFC3339),
		"customer": map[string]any{
			"email":  "user@example.com",
			"mobile": "11999990001",
		},
		"subscription": map[string]any{
			"id":         "sub-ext-001",
			"plan":       "MONTHLY",
			"period_end": periodEnd.Format(time.RFC3339),
		},
		"product": map[string]any{
			"id": "prod-001",
		},
	}
	b, err := json.Marshal(payload)
	s.Require().NoError(err)
	return b
}

func (s *ProcessBillingEventIntegrationSuite) buildRenewedPayload(eventID string, periodEnd time.Time, occurredAt time.Time) []byte {
	payload := map[string]any{
		"id":                 eventID,
		"webhook_event_type": "subscription_renewed",
		"occurred_at":        occurredAt.Format(time.RFC3339),
		"customer": map[string]any{
			"email":  "user@example.com",
			"mobile": "11999990001",
		},
		"subscription": map[string]any{
			"id":         "sub-ext-001",
			"plan":       "MONTHLY",
			"period_end": periodEnd.Format(time.RFC3339),
		},
		"product": map[string]any{
			"id": "prod-001",
		},
	}
	b, err := json.Marshal(payload)
	s.Require().NoError(err)
	return b
}

func (s *ProcessBillingEventIntegrationSuite) mustEventID() events.EventID {
	id, err := events.NewEventID((&sequentialIDGenerator{n: int(time.Now().UnixNano() % 99)}).NewID())
	s.Require().NoError(err)
	return id
}

func (s *ProcessBillingEventIntegrationSuite) mustEventName(v string) events.EventName {
	n, err := events.NewEventName(v)
	s.Require().NoError(err)
	return n
}

// kiwifyParsableProvider implementa BillingProvider para testes de integração,
// fazendo parse real do payload JSON no formato Kiwify simplificado.
type kiwifyParsableProvider struct {
	clk *staticClock
}

func (*kiwifyParsableProvider) VerifySignature(_ []byte, _ map[string]string) error { return nil }

func (*kiwifyParsableProvider) FetchSubscription(_ context.Context, _ string) (services.CanonicalSubscription, error) {
	return services.CanonicalSubscription{}, nil
}

func (p *kiwifyParsableProvider) ParseEvent(payload []byte) (services.CanonicalEvent, error) {
	var doc struct {
		ID               string `json:"id"`
		WebhookEventType string `json:"webhook_event_type"`
		OccurredAt       string `json:"occurred_at"`
		Customer         struct {
			Email  string `json:"email"`
			Mobile string `json:"mobile"`
		} `json:"customer"`
		Subscription struct {
			ID        string `json:"id"`
			Plan      string `json:"plan"`
			PeriodEnd string `json:"period_end"`
		} `json:"subscription"`
		Product struct {
			ID string `json:"id"`
		} `json:"product"`
	}
	if err := json.Unmarshal(payload, &doc); err != nil {
		return services.CanonicalEvent{}, err
	}

	eventType, err := parseTestEventType(doc.WebhookEventType)
	if err != nil {
		return services.CanonicalEvent{}, err
	}

	occurredAt := p.clk.Now()
	if doc.OccurredAt != "" {
		if t, e := time.Parse(time.RFC3339, doc.OccurredAt); e == nil {
			occurredAt = t
		}
	}

	periodEnd := occurredAt.Add(30 * 24 * time.Hour)
	if doc.Subscription.PeriodEnd != "" {
		if t, e := time.Parse(time.RFC3339, doc.Subscription.PeriodEnd); e == nil {
			periodEnd = t
		}
	}

	wa, err := identityvo.NewWhatsAppNumber(doc.Customer.Mobile)
	if err != nil {
		wa, _ = identityvo.NewWhatsAppNumber("+5511999990001")
	}

	planCode := valueobjects.PlanCodeMonthly
	extSubID := doc.Subscription.ID
	if extSubID == "" {
		extSubID = doc.ID
	}

	return services.CanonicalEvent{
		Type:                   eventType,
		ExternalEventID:        doc.ID,
		ExternalSubscriptionID: extSubID,
		PlanCode:               planCode,
		OccurredAt:             occurredAt,
		PeriodStart:            occurredAt,
		PeriodEnd:              periodEnd,
		Customer: services.CanonicalCustomer{
			WhatsApp: wa,
			Email:    doc.Customer.Email,
		},
	}, nil
}

func parseTestEventType(t string) (valueobjects.CanonicalEventType, error) {
	switch t {
	case "compra_aprovada":
		return valueobjects.CanonicalEventPurchaseApproved, nil
	case "subscription_renewed":
		return valueobjects.CanonicalEventRenewed, nil
	case "subscription_late":
		return valueobjects.CanonicalEventLate, nil
	case "subscription_canceled":
		return valueobjects.CanonicalEventCanceled, nil
	case "compra_reembolsada":
		return valueobjects.CanonicalEventRefunded, nil
	case "chargeback":
		return valueobjects.CanonicalEventChargeback, nil
	default:
		return valueobjects.CanonicalEventPurchaseApproved, nil
	}
}

// sqlUserResolver implementa billing.UserResolver via SQL direto, sem importar identity/infrastructure.
type sqlUserResolver struct {
	mgr *dbpkg.Manager
	clk *staticClock
}

func (r *sqlUserResolver) UpsertByWhatsAppNumber(ctx context.Context, number identityvo.WhatsAppNumber) (*identityentities.User, error) {
	now := r.clk.Now()
	dbtx := r.mgr.DBTX(ctx)

	var userID string
	var createdAt, updatedAt time.Time
	err := dbtx.QueryRowContext(ctx,
		`SELECT id, created_at, updated_at FROM users WHERE whatsapp_number = $1 AND deleted_at IS NULL`,
		number.String(),
	).Scan(&userID, &createdAt, &updatedAt)
	if err != nil {
		newID := uuid.New().String()
		err2 := dbtx.QueryRowContext(ctx,
			`INSERT INTO users (id, whatsapp_number, display_name, status, created_at, updated_at)
			 VALUES ($1, $2, $2, 'ACTIVE', $3, $3) RETURNING id, created_at, updated_at`,
			newID, number.String(), now,
		).Scan(&userID, &createdAt, &updatedAt)
		if err2 != nil {
			return nil, err2
		}
	}

	uid, err := identityentities.NewUserID(userID)
	if err != nil {
		return nil, err
	}
	return identityentities.NewUser(identityentities.NewUserParams{
		ID:          uid,
		Number:      number,
		DisplayName: number.String(),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	})
}

// nopEntitlementCache é um cache de entitlement que não faz nada (no-op).
type nopEntitlementCache struct{}

func (*nopEntitlementCache) Get(_ identityentities.UserID) (output.EntitlementDecision, bool) {
	return output.EntitlementDecision{}, false
}

func (*nopEntitlementCache) Set(_ identityentities.UserID, _ output.EntitlementDecision, _ time.Duration) {
}

func (*nopEntitlementCache) Invalidate(_ identityentities.UserID) {}
