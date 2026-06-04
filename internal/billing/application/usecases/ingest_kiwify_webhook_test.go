package usecases_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/output"
	billingmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability/fakes"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

// fakeTxRunnerIngest executa fn de forma síncrona com tx nil para testes unitários.
type fakeTxRunnerIngest struct {
	err error
}

func (f *fakeTxRunnerIngest) Do(ctx context.Context, fn func(ctx context.Context, tx database.DBTX) (output.IngestWebhookResult, error)) (output.IngestWebhookResult, error) {
	if f.err != nil {
		var zero output.IngestWebhookResult
		return zero, f.err
	}
	return fn(ctx, nil)
}

type IngestKiwifyWebhookSuite struct {
	suite.Suite

	ctx         context.Context
	provider    *billingmocks.BillingProvider
	webhookRepo *billingmocks.WebhookEventRepository
	idGenerator *billingmocks.IDGenerator
	storage     *outboxmocks.Storage
	registry    *outboxmocks.Registry
}

func TestIngestKiwifyWebhookSuite(t *testing.T) {
	suite.Run(t, new(IngestKiwifyWebhookSuite))
}

func (s *IngestKiwifyWebhookSuite) SetupTest() {
	s.ctx = context.Background()
	s.provider = billingmocks.NewBillingProvider(s.T())
	s.webhookRepo = billingmocks.NewWebhookEventRepository(s.T())
	s.idGenerator = billingmocks.NewIDGenerator(s.T())
	s.storage = outboxmocks.NewStorage(s.T())
	s.registry = outboxmocks.NewRegistry(s.T())
}

func (s *IngestKiwifyWebhookSuite) buildPublisher() outbox.Publisher {
	return outbox.NewPublisher(s.storage, s.registry, nil)
}

func (s *IngestKiwifyWebhookSuite) validPayload() []byte {
	return json.RawMessage(`{"id":"ext-001","webhook_event_type":"compra_aprovada"}`)
}

func (s *IngestKiwifyWebhookSuite) validHeaders() map[string]string {
	return map[string]string{"X-Kiwify-Webhook-Token": "secret"}
}

func (s *IngestKiwifyWebhookSuite) validInput() input.IngestWebhookInput {
	return input.IngestWebhookInput{
		RawBody:    s.validPayload(),
		Headers:    s.validHeaders(),
		ReceivedAt: time.Now().UTC(),
	}
}

const _validUUID1 = "550e8400-e29b-41d4-a716-446655440001"
const _validUUID2 = "550e8400-e29b-41d4-a716-446655440002"

func (s *IngestKiwifyWebhookSuite) TestExecute() {
	type setupFn func()
	type args struct {
		in      input.IngestWebhookInput
		txError error
	}

	scenarios := []struct {
		name   string
		args   args
		setup  setupFn
		expect func(result output.IngestWebhookResult, err error)
	}{
		{
			name: "happy path: evento novo inserido e publicado",
			args: args{in: s.validInput()},
			setup: func() {
				s.provider.EXPECT().
					VerifySignature(s.validPayload(), s.validHeaders()).
					Return(nil).Once()
				s.idGenerator.EXPECT().NewID().Return(_validUUID1).Once()
				s.idGenerator.EXPECT().NewID().Return(_validUUID2).Once()
				webhookID, _ := valueobjects.NewWebhookEventID(_validUUID1)
				s.webhookRepo.EXPECT().
					InsertIfNew(mock.Anything, mock.AnythingOfType("entities.WebhookEvent")).
					Return(true, nil).Once()
				eventName, _ := events.NewEventName("billing.kiwify.received")
				s.registry.EXPECT().
					SubscriptionsFor(eventName).
					Return([]outbox.Subscription{{
						Name:      mustSubscriptionName(s.T(), "billing-event-processor"),
						EventType: eventName,
						Handler:   func(_ context.Context, _ outbox.Event) error { return nil },
					}}).Once()
				s.storage.EXPECT().
					InsertEvent(mock.Anything, nil, mock.AnythingOfType("outbox.Event")).
					Return(nil).Once()
				s.storage.EXPECT().
					InsertDeliveries(mock.Anything, nil, mock.AnythingOfType("events.EventID"), mock.AnythingOfType("[]outbox.SubscriptionName")).
					Return(nil).Once()
				_ = webhookID
			},
			expect: func(result output.IngestWebhookResult, err error) {
				s.NoError(err)
				s.False(result.Duplicate)
				s.False(result.WebhookEventID.IsZero())
			},
		},
		{
			name: "assinatura inválida retorna erro",
			args: args{in: s.validInput()},
			setup: func() {
				s.provider.EXPECT().
					VerifySignature(s.validPayload(), s.validHeaders()).
					Return(errors.New("assinatura inválida")).Once()
			},
			expect: func(result output.IngestWebhookResult, err error) {
				s.Error(err)
				s.Contains(err.Error(), "ingest kiwify")
			},
		},
		{
			name: "payload inválido (vazio) retorna erro na extração de external_event_id",
			args: args{in: input.IngestWebhookInput{
				RawBody: []byte{},
				Headers: s.validHeaders(),
			}},
			setup: func() {
				s.provider.EXPECT().
					VerifySignature([]byte{}, s.validHeaders()).
					Return(nil).Once()
			},
			expect: func(result output.IngestWebhookResult, err error) {
				s.Error(err)
			},
		},
		{
			name: "evento duplicado retorna Duplicate=true",
			args: args{in: s.validInput()},
			setup: func() {
				s.provider.EXPECT().
					VerifySignature(s.validPayload(), s.validHeaders()).
					Return(nil).Once()
				s.idGenerator.EXPECT().NewID().Return(_validUUID1).Once()
				s.webhookRepo.EXPECT().
					InsertIfNew(mock.Anything, mock.AnythingOfType("entities.WebhookEvent")).
					Return(false, nil).Once()
			},
			expect: func(result output.IngestWebhookResult, err error) {
				s.NoError(err)
				s.True(result.Duplicate)
			},
		},
		{
			name: "falha no publisher retorna erro",
			args: args{in: s.validInput()},
			setup: func() {
				s.provider.EXPECT().
					VerifySignature(s.validPayload(), s.validHeaders()).
					Return(nil).Once()
				s.idGenerator.EXPECT().NewID().Return(_validUUID1).Once()
				s.idGenerator.EXPECT().NewID().Return(_validUUID2).Once()
				s.webhookRepo.EXPECT().
					InsertIfNew(mock.Anything, mock.AnythingOfType("entities.WebhookEvent")).
					Return(true, nil).Once()
				eventName, _ := events.NewEventName("billing.kiwify.received")
				s.registry.EXPECT().
					SubscriptionsFor(eventName).
					Return([]outbox.Subscription{}).Once()
			},
			expect: func(result output.IngestWebhookResult, err error) {
				s.Error(err)
				s.Contains(err.Error(), "publicar outbox")
			},
		},
		{
			name: "InsertIfNew retorna erro → erro propagado",
			args: args{in: s.validInput()},
			setup: func() {
				s.provider.EXPECT().
					VerifySignature(s.validPayload(), s.validHeaders()).
					Return(nil).Once()
				s.idGenerator.EXPECT().NewID().Return(_validUUID1).Once()
				s.webhookRepo.EXPECT().
					InsertIfNew(mock.Anything, mock.AnythingOfType("entities.WebhookEvent")).
					Return(false, errors.New("db error")).Once()
			},
			expect: func(result output.IngestWebhookResult, err error) {
				s.Error(err)
				s.Contains(err.Error(), "inserir webhook_event")
			},
		},
		{
			name: "txRunner retorna erro → erro propagado",
			args: args{in: s.validInput(), txError: errors.New("tx error")},
			setup: func() {
				s.provider.EXPECT().
					VerifySignature(s.validPayload(), s.validHeaders()).
					Return(nil).Once()
			},
			expect: func(result output.IngestWebhookResult, err error) {
				s.Error(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			scenario.setup()
			uc := usecases.NewIngestKiwifyWebhookUseCase(
				s.provider,
				s.webhookRepo,
				s.buildPublisher(),
				&fakeTxRunnerIngest{err: scenario.args.txError},
				s.idGenerator,
				fakes.NoopObservability(),
				fakes.NoopUsecaseMetrics(),
			)
			result, err := uc.Execute(s.ctx, scenario.args.in)
			scenario.expect(result, err)
		})
	}
}

func mustSubscriptionName(t *testing.T, name string) outbox.SubscriptionName {
	t.Helper()
	sn, err := outbox.NewSubscriptionName(name)
	if err != nil {
		t.Fatalf("NewSubscriptionName(%q): %v", name, err)
	}
	return sn
}

// TestInsertIfNew_Idempotency verifica que 5 replays do mesmo webhook produzem 1 única row.
func (s *IngestKiwifyWebhookSuite) TestInsertIfNew_Idempotency() {
	s.provider.EXPECT().
		VerifySignature(s.validPayload(), s.validHeaders()).
		Return(nil).Times(5)
	s.idGenerator.EXPECT().NewID().Return(_validUUID1).Maybe()
	s.idGenerator.EXPECT().NewID().Return(_validUUID2).Maybe()

	insertedCount := 0
	s.webhookRepo.EXPECT().
		InsertIfNew(mock.Anything, mock.AnythingOfType("entities.WebhookEvent")).
		RunAndReturn(func(_ context.Context, _ entities.WebhookEvent) (bool, error) {
			if insertedCount == 0 {
				insertedCount++
				return true, nil
			}
			return false, nil
		}).Times(5)

	eventName, _ := events.NewEventName("billing.kiwify.received")
	s.registry.EXPECT().
		SubscriptionsFor(eventName).
		Return([]outbox.Subscription{{
			Name:      mustSubscriptionName(s.T(), "billing-event-processor"),
			EventType: eventName,
			Handler:   func(_ context.Context, _ outbox.Event) error { return nil },
		}}).Maybe()
	s.storage.EXPECT().
		InsertEvent(mock.Anything, nil, mock.AnythingOfType("outbox.Event")).
		Return(nil).Maybe()
	s.storage.EXPECT().
		InsertDeliveries(mock.Anything, nil, mock.AnythingOfType("events.EventID"), mock.AnythingOfType("[]outbox.SubscriptionName")).
		Return(nil).Maybe()

	uc := usecases.NewIngestKiwifyWebhookUseCase(
		s.provider,
		s.webhookRepo,
		s.buildPublisher(),
		&fakeTxRunnerIngest{},
		s.idGenerator,
		fakes.NoopObservability(),
		fakes.NoopUsecaseMetrics(),
	)

	duplicateCount := 0
	for range 5 {
		result, err := uc.Execute(s.ctx, s.validInput())
		s.NoError(err)
		if result.Duplicate {
			duplicateCount++
		}
	}
	s.Equal(1, insertedCount, "deve ter inserido exatamente 1 row")
	s.Equal(4, duplicateCount, "deve ter detectado 4 duplicatas")
}

func (s *IngestKiwifyWebhookSuite) TestExecute_UsesConfiguredSignatureHeaderForAudit() {
	payload := s.validPayload()
	headers := map[string]string{"X-Custom-Kiwify-Token": "custom-secret"}
	in := input.IngestWebhookInput{
		RawBody:             payload,
		Headers:             headers,
		SignatureHeaderName: "X-Custom-Kiwify-Token",
		ReceivedAt:          time.Now().UTC(),
	}

	s.provider.EXPECT().
		VerifySignature(payload, headers).
		Return(nil).Once()
	s.idGenerator.EXPECT().NewID().Return(_validUUID1).Once()
	s.idGenerator.EXPECT().NewID().Return(_validUUID2).Once()
	s.webhookRepo.EXPECT().
		InsertIfNew(mock.Anything, mock.AnythingOfType("entities.WebhookEvent")).
		RunAndReturn(func(_ context.Context, event entities.WebhookEvent) (bool, error) {
			s.Equal("custom-secret", event.Signature())
			return true, nil
		}).Once()

	eventName, _ := events.NewEventName("billing.kiwify.received")
	s.registry.EXPECT().
		SubscriptionsFor(eventName).
		Return([]outbox.Subscription{{
			Name:      mustSubscriptionName(s.T(), "billing-event-processor"),
			EventType: eventName,
			Handler:   func(_ context.Context, _ outbox.Event) error { return nil },
		}}).Once()
	s.storage.EXPECT().
		InsertEvent(mock.Anything, nil, mock.AnythingOfType("outbox.Event")).
		Return(nil).Once()
	s.storage.EXPECT().
		InsertDeliveries(mock.Anything, nil, mock.AnythingOfType("events.EventID"), mock.AnythingOfType("[]outbox.SubscriptionName")).
		Return(nil).Once()

	uc := usecases.NewIngestKiwifyWebhookUseCase(
		s.provider,
		s.webhookRepo,
		s.buildPublisher(),
		&fakeTxRunnerIngest{},
		s.idGenerator,
		fakes.NoopObservability(),
		fakes.NoopUsecaseMetrics(),
	)

	result, err := uc.Execute(s.ctx, in)
	s.NoError(err)
	s.False(result.Duplicate)
}
