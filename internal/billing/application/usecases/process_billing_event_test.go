package usecases_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	billingmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	identityentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	identityvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability/fakes"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

// fakeTxRunnerProcess executa fn de forma síncrona com tx nil para testes unitários.
type fakeTxRunnerProcess struct {
	err error
}

func (f *fakeTxRunnerProcess) Do(ctx context.Context, fn func(ctx context.Context, tx database.DBTX) (usecases.ProcessBillingEventResult, error)) (usecases.ProcessBillingEventResult, error) {
	if f.err != nil {
		return usecases.ProcessBillingEventResult{}, f.err
	}
	return fn(ctx, nil)
}

const (
	_processWebhookUUID   = "550e8400-e29b-41d4-a716-446655440010"
	_processSubUUID       = "550e8400-e29b-41d4-a716-446655440011"
	_processUserUUID      = "550e8400-e29b-41d4-a716-446655440012"
	_processEventOutboxID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
)

func buildOutboxEvt(t *testing.T, webhookID, provider string) outbox.Event {
	t.Helper()
	payload, err := json.Marshal(map[string]string{
		"webhook_event_id": webhookID,
		"provider":         provider,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	eventID, _ := events.NewEventID(_processEventOutboxID)
	eventName, _ := events.NewEventName("billing.kiwify.received")
	evt, err := outbox.NewEvent(outbox.NewEventParams{
		ID:            eventID,
		EventType:     eventName,
		AggregateType: "webhook_event",
		AggregateID:   webhookID,
		Payload:       payload,
	})
	if err != nil {
		t.Fatalf("new outbox event: %v", err)
	}
	return evt
}

func buildCanonicalEvt(eventType valueobjects.CanonicalEventType, occurredAt time.Time) services.CanonicalEvent {
	number, _ := identityvo.NewWhatsAppNumber("+5511999990000")
	return services.CanonicalEvent{
		Type:                   eventType,
		ExternalEventID:        "ext-001",
		ExternalSubscriptionID: "sub-ext-001",
		PlanCode:               valueobjects.PlanCodeMonthly,
		OccurredAt:             occurredAt,
		PeriodStart:            occurredAt,
		PeriodEnd:              occurredAt.Add(30 * 24 * time.Hour),
		Customer: services.CanonicalCustomer{
			WhatsApp: number,
		},
	}
}

func buildTestUserAndSub(t *testing.T, now time.Time) (*identityentities.User, *entities.Subscription) {
	t.Helper()
	userID, _ := identityentities.NewUserID(_processUserUUID)
	number, _ := identityvo.NewWhatsAppNumber("+5511999990000")
	user, _ := identityentities.NewUser(identityentities.NewUserParams{
		ID:        userID,
		Number:    number,
		CreatedAt: now,
		UpdatedAt: now,
	})
	subID, _ := entities.NewSubscriptionID(_processSubUUID)
	extSubID, _ := valueobjects.NewExternalSubscriptionID("sub-ext-001")
	webhookID, _ := valueobjects.NewWebhookEventID(_processWebhookUUID)
	sub, _ := entities.NewSubscription(entities.NewSubscriptionParams{
		ID:                 subID,
		UserID:             userID,
		Provider:           "kiwify",
		ExternalSubID:      extSubID,
		PlanCode:           valueobjects.PlanCodeMonthly,
		InitialStatus:      valueobjects.SubscriptionStatusActive,
		PeriodStart:        now.Add(-30 * 24 * time.Hour),
		PeriodEnd:          now.Add(30 * 24 * time.Hour),
		LastEventAt:        now.Add(-1 * time.Hour),
		LastWebhookEventID: webhookID,
		CreatedAt:          now.Add(-30 * 24 * time.Hour),
	})
	return user, sub
}

type ProcessBillingEventSuite struct {
	suite.Suite

	ctx          context.Context
	now          time.Time
	webhookRepo  *billingmocks.WebhookEventRepository
	subRepo      *billingmocks.SubscriptionRepository
	provider     *billingmocks.BillingProvider
	userResolver *billingmocks.UserResolver
	cache        *billingmocks.EntitlementCache
	idGenerator  *billingmocks.IDGenerator
	bus          *events.Bus
}

func TestProcessBillingEventSuite(t *testing.T) {
	suite.Run(t, new(ProcessBillingEventSuite))
}

func (s *ProcessBillingEventSuite) SetupTest() {
	s.ctx = context.Background()
	s.now = time.Now().UTC()
	s.webhookRepo = billingmocks.NewWebhookEventRepository(s.T())
	s.subRepo = billingmocks.NewSubscriptionRepository(s.T())
	s.provider = billingmocks.NewBillingProvider(s.T())
	s.userResolver = billingmocks.NewUserResolver(s.T())
	s.cache = billingmocks.NewEntitlementCache(s.T())
	s.idGenerator = billingmocks.NewIDGenerator(s.T())
	s.bus = events.NewBus()
}

func (s *ProcessBillingEventSuite) buildUC() *usecases.ProcessBillingEventUseCase {
	return usecases.NewProcessBillingEventUseCase(
		s.webhookRepo,
		s.subRepo,
		s.provider,
		s.userResolver,
		s.cache,
		s.bus,
		&fakeTxRunnerProcess{},
		s.idGenerator,
		slog.Default(),
		fakes.NoopObservability(),
		fakes.NoopUsecaseMetrics(),
	)
}

func (s *ProcessBillingEventSuite) TestHandle() {
	now := s.now
	webhookID, _ := valueobjects.NewWebhookEventID(_processWebhookUUID)
	rawPayload := []byte(`{"webhook_event_type":"compra_aprovada"}`)
	outboxEvt := buildOutboxEvt(s.T(), _processWebhookUUID, "kiwify")
	var user *identityentities.User
	var sub *entities.Subscription
	var canonical services.CanonicalEvent

	scenarios := []struct {
		name   string
		setup  func()
		evt    outbox.Event
		expect func(err error)
	}{
		{
			name: "happy path: purchase_approved → ACTIVE",
			evt:  outboxEvt,
			setup: func() {
				s.webhookRepo.EXPECT().
					FindRawPayload(mock.Anything, webhookID).
					Return(rawPayload, nil).Once()
				s.provider.EXPECT().
					ParseEvent(mock.Anything).
					Return(canonical, nil).Once()
				s.userResolver.EXPECT().
					UpsertByWhatsAppNumber(mock.Anything, canonical.Customer.WhatsApp).
					Return(user, nil).Once()
				s.subRepo.EXPECT().
					FindActiveByUserIDForUpdate(mock.Anything, user.ID()).
					Return(sub, nil).Once()
				s.webhookRepo.EXPECT().
					RecordApplication(mock.Anything, webhookID, sub.ID(), mock.AnythingOfType("time.Time")).
					Return(true, nil).Once()
				s.subRepo.EXPECT().
					Upsert(mock.Anything, sub).
					Return(nil).Once()
				s.webhookRepo.EXPECT().
					MarkProcessed(mock.Anything, webhookID, mock.AnythingOfType("time.Time")).
					Return(nil).Once()
				s.cache.EXPECT().
					Invalidate(user.ID()).Once()
			},
			expect: func(err error) { s.NoError(err) },
		},
		{
			name: "evento stale ignorado: OccurredAt < LastEventAt → não chama RecordApplication nem Upsert",
			evt:  outboxEvt,
			setup: func() {
				s.webhookRepo.EXPECT().
					FindRawPayload(mock.Anything, webhookID).
					Return(rawPayload, nil).Once()
				staleCanonical := buildCanonicalEvt(valueobjects.CanonicalEventPurchaseApproved,
					sub.LastEventAt().Add(-1*time.Hour))
				s.provider.EXPECT().
					ParseEvent(mock.Anything).
					Return(staleCanonical, nil).Once()
				s.userResolver.EXPECT().
					UpsertByWhatsAppNumber(mock.Anything, staleCanonical.Customer.WhatsApp).
					Return(user, nil).Once()
				s.subRepo.EXPECT().
					FindActiveByUserIDForUpdate(mock.Anything, user.ID()).
					Return(sub, nil).Once()
			},
			expect: func(err error) { s.NoError(err) },
		},
		{
			name: "payload outbox sem webhook_event_id → ErrPermanent",
			evt: func() outbox.Event {
				eventID, _ := events.NewEventID(_processEventOutboxID)
				eventName, _ := events.NewEventName("billing.kiwify.received")
				evt, _ := outbox.NewEvent(outbox.NewEventParams{
					ID:            eventID,
					EventType:     eventName,
					AggregateType: "webhook_event",
					AggregateID:   "agg",
					Payload:       json.RawMessage(`{"bad":"payload"}`),
				})
				return evt
			}(),
			setup: func() {},
			expect: func(err error) {
				s.Error(err)
				s.True(errors.Is(err, outbox.ErrPermanent))
			},
		},
		{
			name: "ParseEvent falha → ErrPermanent",
			evt:  outboxEvt,
			setup: func() {
				s.webhookRepo.EXPECT().
					FindRawPayload(mock.Anything, webhookID).
					Return(rawPayload, nil).Once()
				s.provider.EXPECT().
					ParseEvent(mock.Anything).
					Return(services.CanonicalEvent{}, errors.New("evento desconhecido")).Once()
			},
			expect: func(err error) {
				s.Error(err)
				s.True(errors.Is(err, outbox.ErrPermanent))
			},
		},
		{
			name: "ParseEvent com tipo desconhecido → erro transitório",
			evt:  outboxEvt,
			setup: func() {
				s.webhookRepo.EXPECT().
					FindRawPayload(mock.Anything, webhookID).
					Return(rawPayload, nil).Once()
				s.provider.EXPECT().
					ParseEvent(mock.Anything).
					Return(services.CanonicalEvent{}, interfaces.ErrUnknownProviderEventType).Once()
			},
			expect: func(err error) {
				s.Error(err)
				s.False(errors.Is(err, outbox.ErrPermanent))
			},
		},
		{
			name: "falha transitória no repo de subscription retorna erro não-permanente",
			evt:  outboxEvt,
			setup: func() {
				s.webhookRepo.EXPECT().
					FindRawPayload(mock.Anything, webhookID).
					Return(rawPayload, nil).Once()
				s.provider.EXPECT().
					ParseEvent(mock.Anything).
					Return(canonical, nil).Once()
				s.userResolver.EXPECT().
					UpsertByWhatsAppNumber(mock.Anything, canonical.Customer.WhatsApp).
					Return(user, nil).Once()
				s.subRepo.EXPECT().
					FindActiveByUserIDForUpdate(mock.Anything, user.ID()).
					Return(nil, errors.New("connection refused")).Once()
			},
			expect: func(err error) {
				s.Error(err)
				s.False(errors.Is(err, outbox.ErrPermanent))
			},
		},
		{
			name: "RecordApplication retorna recorded=false → idempotência, sem Upsert",
			evt:  outboxEvt,
			setup: func() {
				s.webhookRepo.EXPECT().
					FindRawPayload(mock.Anything, webhookID).
					Return(rawPayload, nil).Once()
				s.provider.EXPECT().
					ParseEvent(mock.Anything).
					Return(canonical, nil).Once()
				s.userResolver.EXPECT().
					UpsertByWhatsAppNumber(mock.Anything, canonical.Customer.WhatsApp).
					Return(user, nil).Once()
				s.subRepo.EXPECT().
					FindActiveByUserIDForUpdate(mock.Anything, user.ID()).
					Return(sub, nil).Once()
				s.webhookRepo.EXPECT().
					RecordApplication(mock.Anything, webhookID, sub.ID(), mock.AnythingOfType("time.Time")).
					Return(false, nil).Once()
			},
			expect: func(err error) { s.NoError(err) },
		},
		{
			name: "transição ilegal → ErrPermanent",
			evt:  outboxEvt,
			setup: func() {
				s.webhookRepo.EXPECT().
					FindRawPayload(mock.Anything, webhookID).
					Return(rawPayload, nil).Once()
				illegalCanonical := buildCanonicalEvt(valueobjects.CanonicalEventLate, now.Add(time.Hour))
				illegalSub := buildExpiredSub(s.T(), now)
				s.provider.EXPECT().
					ParseEvent(mock.Anything).
					Return(illegalCanonical, nil).Once()
				s.userResolver.EXPECT().
					UpsertByWhatsAppNumber(mock.Anything, illegalCanonical.Customer.WhatsApp).
					Return(user, nil).Once()
				s.subRepo.EXPECT().
					FindActiveByUserIDForUpdate(mock.Anything, user.ID()).
					Return(illegalSub, nil).Once()
			},
			expect: func(err error) {
				s.Error(err)
				s.True(errors.Is(err, outbox.ErrPermanent))
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			user, sub = buildTestUserAndSub(s.T(), now)
			canonical = buildCanonicalEvt(valueobjects.CanonicalEventPurchaseApproved, now)
			scenario.setup()
			uc := s.buildUC()
			err := uc.Handle(s.ctx, scenario.evt)
			scenario.expect(err)
		})
	}
}

// TestHandle_NewSubscription verifica que purchase_approved sem subscription existente cria nova subscription.
func (s *ProcessBillingEventSuite) TestHandle_NewSubscription() {
	now := s.now
	user, _ := buildTestUserAndSub(s.T(), now)
	webhookID, _ := valueobjects.NewWebhookEventID(_processWebhookUUID)
	rawPayload := []byte(`{"webhook_event_type":"compra_aprovada"}`)
	outboxEvt := buildOutboxEvt(s.T(), _processWebhookUUID, "kiwify")
	canonical := buildCanonicalEvt(valueobjects.CanonicalEventPurchaseApproved, now)

	s.webhookRepo.EXPECT().
		FindRawPayload(mock.Anything, webhookID).
		Return(rawPayload, nil).Once()
	s.provider.EXPECT().
		ParseEvent(mock.Anything).
		Return(canonical, nil).Once()
	s.userResolver.EXPECT().
		UpsertByWhatsAppNumber(mock.Anything, canonical.Customer.WhatsApp).
		Return(user, nil).Once()
	// Sem subscription ativa → ErrSubscriptionNotFound.
	s.subRepo.EXPECT().
		FindActiveByUserIDForUpdate(mock.Anything, user.ID()).
		Return(nil, usecases.ErrSubscriptionNotFound).Once()
	s.idGenerator.EXPECT().NewID().Return(_processSubUUID).Once()
	s.webhookRepo.EXPECT().
		RecordApplication(mock.Anything, webhookID, mock.AnythingOfType("entities.SubscriptionID"), mock.AnythingOfType("time.Time")).
		Return(true, nil).Once()
	s.subRepo.EXPECT().
		Upsert(mock.Anything, mock.AnythingOfType("*entities.Subscription")).
		Return(nil).Once()
	s.webhookRepo.EXPECT().
		MarkProcessed(mock.Anything, webhookID, mock.AnythingOfType("time.Time")).
		Return(nil).Once()
	s.cache.EXPECT().
		Invalidate(user.ID()).Once()

	uc := s.buildUC()
	err := uc.Handle(s.ctx, outboxEvt)
	s.NoError(err)
}

// TestHandle_CanonicalEventTypes testa os tipos de evento canônico Renewed, Late, Canceled, Refunded.
func (s *ProcessBillingEventSuite) TestHandle_CanonicalEventTypes() {
	now := s.now
	webhookID, _ := valueobjects.NewWebhookEventID(_processWebhookUUID)
	rawPayload := []byte(`{"webhook_event_type":"renovacao"}`)
	outboxEvt := buildOutboxEvt(s.T(), _processWebhookUUID, "kiwify")

	scenarios := []struct {
		name      string
		eventType valueobjects.CanonicalEventType
	}{
		{
			name:      "renewed: Active→Active via Renew",
			eventType: valueobjects.CanonicalEventRenewed,
		},
		{
			name:      "late: Active→PastDue",
			eventType: valueobjects.CanonicalEventLate,
		},
	}

	for _, tc := range scenarios {
		s.Run(tc.name, func() {
			s.SetupTest()
			user, sub := buildTestUserAndSub(s.T(), now)
			canonical := buildCanonicalEvt(tc.eventType, now)
			currentSub := sub
			s.webhookRepo.EXPECT().
				FindRawPayload(mock.Anything, webhookID).
				Return(rawPayload, nil).Once()
			s.provider.EXPECT().
				ParseEvent(mock.Anything).
				Return(canonical, nil).Once()
			s.userResolver.EXPECT().
				UpsertByWhatsAppNumber(mock.Anything, canonical.Customer.WhatsApp).
				Return(user, nil).Once()
			s.subRepo.EXPECT().
				FindActiveByUserIDForUpdate(mock.Anything, user.ID()).
				Return(currentSub, nil).Once()
			s.webhookRepo.EXPECT().
				RecordApplication(mock.Anything, webhookID, currentSub.ID(), mock.AnythingOfType("time.Time")).
				Return(true, nil).Once()
			s.subRepo.EXPECT().
				Upsert(mock.Anything, mock.AnythingOfType("*entities.Subscription")).
				Return(nil).Once()
			s.webhookRepo.EXPECT().
				MarkProcessed(mock.Anything, webhookID, mock.AnythingOfType("time.Time")).
				Return(nil).Once()
			s.cache.EXPECT().
				Invalidate(user.ID()).Once()

			uc := s.buildUC()
			err := uc.Handle(s.ctx, outboxEvt)
			s.NoError(err)
		})
	}
}

// TestHandle_CanceledEvent testa transição Active→CanceledPending via Cancel.
func (s *ProcessBillingEventSuite) TestHandle_CanceledEvent() {
	now := s.now
	user, sub := buildTestUserAndSub(s.T(), now)
	webhookID, _ := valueobjects.NewWebhookEventID(_processWebhookUUID)
	rawPayload := []byte(`{"webhook_event_type":"cancelamento"}`)
	outboxEvt := buildOutboxEvt(s.T(), _processWebhookUUID, "kiwify")
	canonical := buildCanonicalEvt(valueobjects.CanonicalEventCanceled, now)

	s.webhookRepo.EXPECT().FindRawPayload(mock.Anything, webhookID).Return(rawPayload, nil).Once()
	s.provider.EXPECT().ParseEvent(mock.Anything).Return(canonical, nil).Once()
	s.userResolver.EXPECT().UpsertByWhatsAppNumber(mock.Anything, canonical.Customer.WhatsApp).Return(user, nil).Once()
	s.subRepo.EXPECT().FindActiveByUserIDForUpdate(mock.Anything, user.ID()).Return(sub, nil).Once()
	s.webhookRepo.EXPECT().RecordApplication(mock.Anything, webhookID, sub.ID(), mock.AnythingOfType("time.Time")).Return(true, nil).Once()
	s.subRepo.EXPECT().Upsert(mock.Anything, mock.AnythingOfType("*entities.Subscription")).Return(nil).Once()
	s.webhookRepo.EXPECT().MarkProcessed(mock.Anything, webhookID, mock.AnythingOfType("time.Time")).Return(nil).Once()
	s.cache.EXPECT().Invalidate(user.ID()).Once()

	uc := s.buildUC()
	err := uc.Handle(s.ctx, outboxEvt)
	s.NoError(err)
}

// TestHandle_RefundedEvent testa transição Active→Refunded via Refund.
func (s *ProcessBillingEventSuite) TestHandle_RefundedEvent() {
	now := s.now
	user, sub := buildTestUserAndSub(s.T(), now)
	webhookID, _ := valueobjects.NewWebhookEventID(_processWebhookUUID)
	rawPayload := []byte(`{"webhook_event_type":"reembolso"}`)
	outboxEvt := buildOutboxEvt(s.T(), _processWebhookUUID, "kiwify")
	canonical := buildCanonicalEvt(valueobjects.CanonicalEventRefunded, now)
	canonical.RefundAmountCents = 9900

	s.webhookRepo.EXPECT().FindRawPayload(mock.Anything, webhookID).Return(rawPayload, nil).Once()
	s.provider.EXPECT().ParseEvent(mock.Anything).Return(canonical, nil).Once()
	s.userResolver.EXPECT().UpsertByWhatsAppNumber(mock.Anything, canonical.Customer.WhatsApp).Return(user, nil).Once()
	s.subRepo.EXPECT().FindActiveByUserIDForUpdate(mock.Anything, user.ID()).Return(sub, nil).Once()
	s.webhookRepo.EXPECT().RecordApplication(mock.Anything, webhookID, sub.ID(), mock.AnythingOfType("time.Time")).Return(true, nil).Once()
	s.subRepo.EXPECT().Upsert(mock.Anything, mock.AnythingOfType("*entities.Subscription")).Return(nil).Once()
	s.webhookRepo.EXPECT().MarkProcessed(mock.Anything, webhookID, mock.AnythingOfType("time.Time")).Return(nil).Once()
	s.cache.EXPECT().Invalidate(user.ID()).Once()

	uc := s.buildUC()
	err := uc.Handle(s.ctx, outboxEvt)
	s.NoError(err)
}

// TestHandle_ChargebackEvent testa canonical.Chargeback → Refund.
func (s *ProcessBillingEventSuite) TestHandle_ChargebackEvent() {
	now := s.now
	user, sub := buildTestUserAndSub(s.T(), now)
	webhookID, _ := valueobjects.NewWebhookEventID(_processWebhookUUID)
	rawPayload := []byte(`{"webhook_event_type":"chargeback"}`)
	outboxEvt := buildOutboxEvt(s.T(), _processWebhookUUID, "kiwify")
	canonical := buildCanonicalEvt(valueobjects.CanonicalEventChargeback, now)
	canonical.RefundAmountCents = 9900

	s.webhookRepo.EXPECT().FindRawPayload(mock.Anything, webhookID).Return(rawPayload, nil).Once()
	s.provider.EXPECT().ParseEvent(mock.Anything).Return(canonical, nil).Once()
	s.userResolver.EXPECT().UpsertByWhatsAppNumber(mock.Anything, canonical.Customer.WhatsApp).Return(user, nil).Once()
	s.subRepo.EXPECT().FindActiveByUserIDForUpdate(mock.Anything, user.ID()).Return(sub, nil).Once()
	s.webhookRepo.EXPECT().RecordApplication(mock.Anything, webhookID, sub.ID(), mock.AnythingOfType("time.Time")).Return(true, nil).Once()
	s.subRepo.EXPECT().Upsert(mock.Anything, mock.AnythingOfType("*entities.Subscription")).Return(nil).Once()
	s.webhookRepo.EXPECT().MarkProcessed(mock.Anything, webhookID, mock.AnythingOfType("time.Time")).Return(nil).Once()
	s.cache.EXPECT().Invalidate(user.ID()).Once()

	uc := s.buildUC()
	err := uc.Handle(s.ctx, outboxEvt)
	s.NoError(err)
}

// TestHandle_UnknownEventType testa tipo de evento desconhecido → ErrPermanent.
func (s *ProcessBillingEventSuite) TestHandle_UnknownEventType() {
	now := s.now
	user, sub := buildTestUserAndSub(s.T(), now)
	webhookID, _ := valueobjects.NewWebhookEventID(_processWebhookUUID)
	rawPayload := []byte(`{"webhook_event_type":"unknown"}`)
	outboxEvt := buildOutboxEvt(s.T(), _processWebhookUUID, "kiwify")
	// CanonicalEventType(0) é o valor zero/desconhecido.
	unknownCanonical := buildCanonicalEvt(valueobjects.CanonicalEventType(99), now)

	s.webhookRepo.EXPECT().FindRawPayload(mock.Anything, webhookID).Return(rawPayload, nil).Once()
	s.provider.EXPECT().ParseEvent(mock.Anything).Return(unknownCanonical, nil).Once()
	s.userResolver.EXPECT().UpsertByWhatsAppNumber(mock.Anything, unknownCanonical.Customer.WhatsApp).Return(user, nil).Once()
	s.subRepo.EXPECT().FindActiveByUserIDForUpdate(mock.Anything, user.ID()).Return(sub, nil).Once()

	uc := s.buildUC()
	err := uc.Handle(s.ctx, outboxEvt)
	s.Error(err)
	s.True(errors.Is(err, outbox.ErrPermanent))
}

// TestHandle_UpsertUserFails verifica que falha em UpsertByWhatsAppNumber retorna erro.
func (s *ProcessBillingEventSuite) TestHandle_UpsertUserFails() {
	webhookID, _ := valueobjects.NewWebhookEventID(_processWebhookUUID)
	rawPayload := []byte(`{"webhook_event_type":"compra_aprovada"}`)
	outboxEvt := buildOutboxEvt(s.T(), _processWebhookUUID, "kiwify")
	now := s.now
	canonical := buildCanonicalEvt(valueobjects.CanonicalEventPurchaseApproved, now)

	s.webhookRepo.EXPECT().FindRawPayload(mock.Anything, webhookID).Return(rawPayload, nil).Once()
	s.provider.EXPECT().ParseEvent(mock.Anything).Return(canonical, nil).Once()
	s.userResolver.EXPECT().
		UpsertByWhatsAppNumber(mock.Anything, canonical.Customer.WhatsApp).
		Return(nil, errors.New("upsert failed")).Once()

	uc := s.buildUC()
	err := uc.Handle(s.ctx, outboxEvt)
	s.Error(err)
	s.False(errors.Is(err, outbox.ErrPermanent))
}

// TestHandle_InvalidWebhookEventID verifica payload com webhook_event_id não-UUID → ErrPermanent.
func (s *ProcessBillingEventSuite) TestHandle_InvalidWebhookEventID() {
	eventID, _ := events.NewEventID(_processEventOutboxID)
	eventName, _ := events.NewEventName("billing.kiwify.received")
	evt, _ := outbox.NewEvent(outbox.NewEventParams{
		ID:            eventID,
		EventType:     eventName,
		AggregateType: "webhook_event",
		AggregateID:   "agg",
		Payload:       json.RawMessage(`{"webhook_event_id":"not-a-uuid","provider":"kiwify"}`),
	})

	uc := s.buildUC()
	err := uc.Handle(s.ctx, evt)
	s.Error(err)
	s.True(errors.Is(err, outbox.ErrPermanent))
}

// TestHandle_FindRawPayloadFails verifica que erro em FindRawPayload retorna erro não-permanente.
func (s *ProcessBillingEventSuite) TestHandle_FindRawPayloadFails() {
	webhookID, _ := valueobjects.NewWebhookEventID(_processWebhookUUID)
	outboxEvt := buildOutboxEvt(s.T(), _processWebhookUUID, "kiwify")

	s.webhookRepo.EXPECT().
		FindRawPayload(mock.Anything, webhookID).
		Return(nil, errors.New("db error")).Once()

	uc := s.buildUC()
	err := uc.Handle(s.ctx, outboxEvt)
	s.Error(err)
	s.False(errors.Is(err, outbox.ErrPermanent))
}

// TestHandle_BusSubscriber verifica que o evento volátil billing.subscription.state_changed é publicado no bus.
func (s *ProcessBillingEventSuite) TestHandle_BusSubscriber() {
	now := s.now
	user, sub := buildTestUserAndSub(s.T(), now)
	webhookID, _ := valueobjects.NewWebhookEventID(_processWebhookUUID)
	rawPayload := []byte(`{"webhook_event_type":"compra_aprovada"}`)
	outboxEvt := buildOutboxEvt(s.T(), _processWebhookUUID, "kiwify")
	canonical := buildCanonicalEvt(valueobjects.CanonicalEventPurchaseApproved, now)

	received := make(chan output.StateChangedEvent, 1)
	_, _ = events.Subscribe(s.bus, func(_ context.Context, evt output.StateChangedEvent) error {
		received <- evt
		return nil
	})

	s.webhookRepo.EXPECT().FindRawPayload(mock.Anything, webhookID).Return(rawPayload, nil).Once()
	s.provider.EXPECT().ParseEvent(mock.Anything).Return(canonical, nil).Once()
	s.userResolver.EXPECT().UpsertByWhatsAppNumber(mock.Anything, canonical.Customer.WhatsApp).Return(user, nil).Once()
	s.subRepo.EXPECT().FindActiveByUserIDForUpdate(mock.Anything, user.ID()).Return(sub, nil).Once()
	s.webhookRepo.EXPECT().RecordApplication(mock.Anything, webhookID, sub.ID(), mock.AnythingOfType("time.Time")).Return(true, nil).Once()
	s.subRepo.EXPECT().Upsert(mock.Anything, sub).Return(nil).Once()
	s.webhookRepo.EXPECT().MarkProcessed(mock.Anything, webhookID, mock.AnythingOfType("time.Time")).Return(nil).Once()
	s.cache.EXPECT().Invalidate(user.ID()).Once()

	uc := s.buildUC()
	err := uc.Handle(s.ctx, outboxEvt)
	s.NoError(err)
	select {
	case evt := <-received:
		s.Equal("billing.subscription.state_changed", evt.Name().String())
		s.Equal(sub.ID().String(), evt.SubscriptionID)
		s.Equal(user.ID().String(), evt.UserID)
		s.Equal(canonical.Customer.WhatsApp.String(), evt.WhatsAppNumber)
		s.Equal(valueobjects.SubscriptionStatusActive.String(), evt.PreviousState)
		s.Equal(valueobjects.SubscriptionStatusActive.String(), evt.NewState)
		s.Equal(valueobjects.TransitionReasonPurchaseApproved.String(), evt.TransitionReason)
		s.Equal(sub.PeriodEnd(), evt.PeriodEnd)
	case <-time.After(200 * time.Millisecond):
		s.Fail("evento state_changed não foi publicado")
	}
}

// TestHandle_RefundNegativeAmount verifica que RefundAmountCents negativo → ErrPermanent.
func (s *ProcessBillingEventSuite) TestHandle_RefundNegativeAmount() {
	now := s.now
	user, sub := buildTestUserAndSub(s.T(), now)
	webhookID, _ := valueobjects.NewWebhookEventID(_processWebhookUUID)
	rawPayload := []byte(`{"webhook_event_type":"reembolso"}`)
	outboxEvt := buildOutboxEvt(s.T(), _processWebhookUUID, "kiwify")
	canonical := buildCanonicalEvt(valueobjects.CanonicalEventRefunded, now)
	canonical.RefundAmountCents = -100 // inválido

	s.webhookRepo.EXPECT().FindRawPayload(mock.Anything, webhookID).Return(rawPayload, nil).Once()
	s.provider.EXPECT().ParseEvent(mock.Anything).Return(canonical, nil).Once()
	s.userResolver.EXPECT().UpsertByWhatsAppNumber(mock.Anything, canonical.Customer.WhatsApp).Return(user, nil).Once()
	s.subRepo.EXPECT().FindActiveByUserIDForUpdate(mock.Anything, user.ID()).Return(sub, nil).Once()

	uc := s.buildUC()
	err := uc.Handle(s.ctx, outboxEvt)
	s.Error(err)
	s.True(errors.Is(err, outbox.ErrPermanent))
}

// TestHandle_MarkProcessedFails verifica que erro em MarkProcessed retorna erro.
func (s *ProcessBillingEventSuite) TestHandle_MarkProcessedFails() {
	now := s.now
	user, sub := buildTestUserAndSub(s.T(), now)
	webhookID, _ := valueobjects.NewWebhookEventID(_processWebhookUUID)
	rawPayload := []byte(`{"webhook_event_type":"compra_aprovada"}`)
	outboxEvt := buildOutboxEvt(s.T(), _processWebhookUUID, "kiwify")
	canonical := buildCanonicalEvt(valueobjects.CanonicalEventPurchaseApproved, now)

	s.webhookRepo.EXPECT().FindRawPayload(mock.Anything, webhookID).Return(rawPayload, nil).Once()
	s.provider.EXPECT().ParseEvent(mock.Anything).Return(canonical, nil).Once()
	s.userResolver.EXPECT().UpsertByWhatsAppNumber(mock.Anything, canonical.Customer.WhatsApp).Return(user, nil).Once()
	s.subRepo.EXPECT().FindActiveByUserIDForUpdate(mock.Anything, user.ID()).Return(sub, nil).Once()
	s.webhookRepo.EXPECT().RecordApplication(mock.Anything, webhookID, sub.ID(), mock.AnythingOfType("time.Time")).Return(true, nil).Once()
	s.subRepo.EXPECT().Upsert(mock.Anything, sub).Return(nil).Once()
	s.webhookRepo.EXPECT().MarkProcessed(mock.Anything, webhookID, mock.AnythingOfType("time.Time")).Return(errors.New("db error")).Once()

	uc := s.buildUC()
	err := uc.Handle(s.ctx, outboxEvt)
	s.Error(err)
	s.Contains(err.Error(), "marcar webhook processado")
}

// TestHandle_UpsertFails verifica que erro em Upsert retorna erro.
func (s *ProcessBillingEventSuite) TestHandle_UpsertFails() {
	now := s.now
	user, sub := buildTestUserAndSub(s.T(), now)
	webhookID, _ := valueobjects.NewWebhookEventID(_processWebhookUUID)
	rawPayload := []byte(`{"webhook_event_type":"compra_aprovada"}`)
	outboxEvt := buildOutboxEvt(s.T(), _processWebhookUUID, "kiwify")
	canonical := buildCanonicalEvt(valueobjects.CanonicalEventPurchaseApproved, now)

	s.webhookRepo.EXPECT().FindRawPayload(mock.Anything, webhookID).Return(rawPayload, nil).Once()
	s.provider.EXPECT().ParseEvent(mock.Anything).Return(canonical, nil).Once()
	s.userResolver.EXPECT().UpsertByWhatsAppNumber(mock.Anything, canonical.Customer.WhatsApp).Return(user, nil).Once()
	s.subRepo.EXPECT().FindActiveByUserIDForUpdate(mock.Anything, user.ID()).Return(sub, nil).Once()
	s.webhookRepo.EXPECT().RecordApplication(mock.Anything, webhookID, sub.ID(), mock.AnythingOfType("time.Time")).Return(true, nil).Once()
	s.subRepo.EXPECT().Upsert(mock.Anything, sub).Return(errors.New("db error")).Once()

	uc := s.buildUC()
	err := uc.Handle(s.ctx, outboxEvt)
	s.Error(err)
	s.Contains(err.Error(), "upsert subscription")
}

// TestHandle_RecordApplicationFails verifica que erro em RecordApplication retorna erro.
func (s *ProcessBillingEventSuite) TestHandle_RecordApplicationFails() {
	now := s.now
	user, sub := buildTestUserAndSub(s.T(), now)
	webhookID, _ := valueobjects.NewWebhookEventID(_processWebhookUUID)
	rawPayload := []byte(`{"webhook_event_type":"compra_aprovada"}`)
	outboxEvt := buildOutboxEvt(s.T(), _processWebhookUUID, "kiwify")
	canonical := buildCanonicalEvt(valueobjects.CanonicalEventPurchaseApproved, now)

	s.webhookRepo.EXPECT().FindRawPayload(mock.Anything, webhookID).Return(rawPayload, nil).Once()
	s.provider.EXPECT().ParseEvent(mock.Anything).Return(canonical, nil).Once()
	s.userResolver.EXPECT().UpsertByWhatsAppNumber(mock.Anything, canonical.Customer.WhatsApp).Return(user, nil).Once()
	s.subRepo.EXPECT().FindActiveByUserIDForUpdate(mock.Anything, user.ID()).Return(sub, nil).Once()
	s.webhookRepo.EXPECT().RecordApplication(mock.Anything, webhookID, sub.ID(), mock.AnythingOfType("time.Time")).Return(false, errors.New("db error")).Once()

	uc := s.buildUC()
	err := uc.Handle(s.ctx, outboxEvt)
	s.Error(err)
	s.Contains(err.Error(), "registrar aplicação")
}

// TestHandle_NoOpEvents verifica que eventos Renewed/Late/Canceled/Refunded com sub==nil → no-op (applied=false).
func (s *ProcessBillingEventSuite) TestHandle_NoOpEventsWithNilSub() {
	now := s.now
	user, _ := buildTestUserAndSub(s.T(), now)
	webhookID, _ := valueobjects.NewWebhookEventID(_processWebhookUUID)
	rawPayload := []byte(`{"webhook_event_type":"renovacao"}`)
	outboxEvt := buildOutboxEvt(s.T(), _processWebhookUUID, "kiwify")

	eventTypes := []valueobjects.CanonicalEventType{
		valueobjects.CanonicalEventRenewed,
		valueobjects.CanonicalEventLate,
		valueobjects.CanonicalEventCanceled,
		valueobjects.CanonicalEventRefunded,
		valueobjects.CanonicalEventChargeback,
	}

	for _, et := range eventTypes {
		s.Run(et.String(), func() {
			s.SetupTest()
			canonical := buildCanonicalEvt(et, now)
			canonical.RefundAmountCents = 9900
			s.webhookRepo.EXPECT().FindRawPayload(mock.Anything, webhookID).Return(rawPayload, nil).Once()
			s.provider.EXPECT().ParseEvent(mock.Anything).Return(canonical, nil).Once()
			s.userResolver.EXPECT().UpsertByWhatsAppNumber(mock.Anything, canonical.Customer.WhatsApp).Return(user, nil).Once()
			// Nenhuma subscription encontrada → no-op, sub retorna nil, cache não é invalidado.
			s.subRepo.EXPECT().FindActiveByUserIDForUpdate(mock.Anything, user.ID()).Return(nil, usecases.ErrSubscriptionNotFound).Once()

			uc := s.buildUC()
			err := uc.Handle(s.ctx, outboxEvt)
			s.NoError(err, "evento %s com sub=nil deve ser no-op", et.String())
		})
	}
}

// TestHandle_IdempotentReplay verifica que 5 replays do mesmo evento produzem 1 row em billing_event_applications.
func (s *ProcessBillingEventSuite) TestHandle_IdempotentReplay() {
	now := s.now
	user, sub := buildTestUserAndSub(s.T(), now)
	webhookID, _ := valueobjects.NewWebhookEventID(_processWebhookUUID)
	rawPayload := []byte(`{"webhook_event_type":"compra_aprovada"}`)
	outboxEvt := buildOutboxEvt(s.T(), _processWebhookUUID, "kiwify")
	canonical := buildCanonicalEvt(valueobjects.CanonicalEventPurchaseApproved, now)

	recordedCount := 0

	s.webhookRepo.EXPECT().
		FindRawPayload(mock.Anything, webhookID).
		Return(rawPayload, nil).Times(5)
	s.provider.EXPECT().
		ParseEvent(mock.Anything).
		Return(canonical, nil).Times(5)
	s.userResolver.EXPECT().
		UpsertByWhatsAppNumber(mock.Anything, canonical.Customer.WhatsApp).
		Return(user, nil).Times(5)
	s.subRepo.EXPECT().
		FindActiveByUserIDForUpdate(mock.Anything, user.ID()).
		Return(sub, nil).Times(5)
	s.webhookRepo.EXPECT().
		RecordApplication(mock.Anything, webhookID, sub.ID(), mock.AnythingOfType("time.Time")).
		RunAndReturn(func(_ context.Context, _ valueobjects.WebhookEventID, _ entities.SubscriptionID, _ time.Time) (bool, error) {
			recordedCount++
			return true, nil
		}).Once()
	s.subRepo.EXPECT().
		Upsert(mock.Anything, sub).
		Return(nil).Once()
	s.webhookRepo.EXPECT().
		MarkProcessed(mock.Anything, webhookID, mock.AnythingOfType("time.Time")).
		Return(nil).Once()
	s.cache.EXPECT().
		Invalidate(user.ID()).Once()

	uc := s.buildUC()
	for range 5 {
		err := uc.Handle(s.ctx, outboxEvt)
		s.NoError(err)
	}
	s.Equal(1, recordedCount, "deve ter gravado exatamente 1 row em billing_event_applications")
}

// buildExpiredSub cria uma subscription em status EXPIRED para testar transições ilegais.
// Usa RehydrateSubscription para definir diretamente o status terminal sem passar pela máquina de estados.
func buildExpiredSub(t *testing.T, now time.Time) *entities.Subscription {
	t.Helper()
	userID, _ := identityentities.NewUserID(_processUserUUID)
	subID, _ := entities.NewSubscriptionID(_processSubUUID)
	extSubID, _ := valueobjects.NewExternalSubscriptionID("sub-ext-001")
	webhookID, _ := valueobjects.NewWebhookEventID(_processWebhookUUID)
	return entities.RehydrateSubscription(entities.RehydrateSubscriptionParams{
		ID:                 subID,
		UserID:             userID,
		Provider:           "kiwify",
		ExternalSubID:      extSubID,
		PlanCode:           valueobjects.PlanCodeMonthly,
		Status:             valueobjects.SubscriptionStatusExpired,
		PeriodStart:        now.Add(-60 * 24 * time.Hour),
		PeriodEnd:          now.Add(-1 * time.Hour),
		LastEventAt:        now.Add(-2 * time.Hour),
		LastWebhookEventID: webhookID,
		CreatedAt:          now.Add(-60 * 24 * time.Hour),
		UpdatedAt:          now.Add(-1 * time.Hour),
	})
}
