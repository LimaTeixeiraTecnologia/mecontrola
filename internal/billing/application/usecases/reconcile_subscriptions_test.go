package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/output"
	billingmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	identityentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability/fakes"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

// fakeTxRunnerReconcile executa fn de forma síncrona com tx nil para testes unitários.
type fakeTxRunnerReconcile struct {
	err error
}

func (f *fakeTxRunnerReconcile) Do(
	ctx context.Context,
	fn func(ctx context.Context, tx database.DBTX) (output.ReconciliationReport, error),
) (output.ReconciliationReport, error) {
	if f.err != nil {
		return output.ReconciliationReport{}, f.err
	}
	return fn(ctx, nil)
}

const (
	_reconcileSubUUID1  = "550e8400-e29b-41d4-a716-446655440030"
	_reconcileUserUUID1 = "550e8400-e29b-41d4-a716-446655440031"
	_reconcileEventUUID = "550e8400-e29b-41d4-a716-446655440032"
)

type ReconcileSubscriptionsSuite struct {
	suite.Suite

	ctx         context.Context
	now         time.Time
	subRepo     *billingmocks.SubscriptionRepository
	webhookRepo *billingmocks.WebhookEventRepository
	provider    *billingmocks.BillingProvider
	storage     *outboxmocks.Storage
	registry    *outboxmocks.Registry
	idGenerator *billingmocks.IDGenerator
}

func TestReconcileSubscriptionsSuite(t *testing.T) {
	suite.Run(t, new(ReconcileSubscriptionsSuite))
}

func (s *ReconcileSubscriptionsSuite) SetupTest() {
	s.ctx = context.Background()
	s.now = time.Now().UTC()
	s.subRepo = billingmocks.NewSubscriptionRepository(s.T())
	s.webhookRepo = billingmocks.NewWebhookEventRepository(s.T())
	s.provider = billingmocks.NewBillingProvider(s.T())
	s.storage = outboxmocks.NewStorage(s.T())
	s.registry = outboxmocks.NewRegistry(s.T())
	s.idGenerator = billingmocks.NewIDGenerator(s.T())
}

func (s *ReconcileSubscriptionsSuite) buildPublisher() outbox.Publisher {
	return outbox.NewPublisher(s.storage, s.registry, nil)
}

func (s *ReconcileSubscriptionsSuite) buildUC() *usecases.ReconcileSubscriptionsUseCase {
	return usecases.NewReconcileSubscriptionsUseCase(
		s.subRepo,
		s.webhookRepo,
		s.provider,
		s.buildPublisher(),
		&fakeTxRunnerReconcile{},
		s.idGenerator,
		fakes.NoopObservability(),
		fakes.NoopUsecaseMetrics(),
	)
}

func (s *ReconcileSubscriptionsSuite) buildActiveSub() *entities.Subscription {
	now := s.now
	userID, _ := identityentities.NewUserID(_reconcileUserUUID1)
	subID, _ := entities.NewSubscriptionID(_reconcileSubUUID1)
	extSubID, _ := valueobjects.NewExternalSubscriptionID("sub-ext-reconcile-001")
	webhookID, _ := valueobjects.NewWebhookEventID(_reconcileEventUUID)
	sub, _ := entities.NewSubscription(entities.NewSubscriptionParams{
		ID:                 subID,
		UserID:             userID,
		Provider:           "kiwify",
		ExternalSubID:      extSubID,
		PlanCode:           valueobjects.PlanCodeMonthly,
		InitialStatus:      valueobjects.SubscriptionStatusActive,
		PeriodStart:        now.Add(-15 * 24 * time.Hour),
		PeriodEnd:          now.Add(15 * 24 * time.Hour),
		LastEventAt:        now.Add(-15 * 24 * time.Hour),
		LastWebhookEventID: webhookID,
		CreatedAt:          now.Add(-15 * 24 * time.Hour),
	})
	return sub
}

func (s *ReconcileSubscriptionsSuite) TestExecute() {
	sub := s.buildActiveSub()
	validStatuses := []valueobjects.SubscriptionStatus{
		valueobjects.SubscriptionStatusActive,
		valueobjects.SubscriptionStatusPastDue,
	}
	zeroID := entities.SubscriptionID{}

	scenarios := []struct {
		name   string
		setup  func()
		expect func(report output.ReconciliationReport, err error)
	}{
		{
			name: "batch vazio: nenhuma subscription → report zero",
			setup: func() {
				s.subRepo.EXPECT().
					ListByStatusInBatch(mock.Anything, validStatuses, time.Time{}, zeroID, 200).
					Return(nil, nil).Once()
			},
			expect: func(report output.ReconciliationReport, err error) {
				s.NoError(err)
				s.Equal(0, report.Inspected)
				s.Equal(0, report.Diverged)
				s.Equal(0, report.Synced)
			},
		},
		{
			name: "subscription em sincronia: sem divergência, sem evento",
			setup: func() {
				remote := services.CanonicalSubscription{
					ExternalID:  sub.ExternalSubscriptionID().String(),
					Status:      valueobjects.SubscriptionStatusActive,
					PlanCode:    sub.PlanCode(),
					PeriodStart: sub.PeriodStart(),
					PeriodEnd:   sub.PeriodEnd(),
				}
				// Batch com 1 item (< 200) → loop termina após esta chamada.
				s.subRepo.EXPECT().
					ListByStatusInBatch(mock.Anything, validStatuses, time.Time{}, zeroID, 200).
					Return([]*entities.Subscription{sub}, nil).Once()
				s.provider.EXPECT().
					FetchSubscription(mock.Anything, sub.ExternalSubscriptionID().String()).
					Return(remote, nil).Once()
			},
			expect: func(report output.ReconciliationReport, err error) {
				s.NoError(err)
				s.Equal(1, report.Inspected)
				s.Equal(0, report.Diverged)
			},
		},
		{
			name: "subscription divergida: publica evento sintético",
			setup: func() {
				remote := services.CanonicalSubscription{
					ExternalID:  sub.ExternalSubscriptionID().String(),
					Status:      valueobjects.SubscriptionStatusPastDue,
					PlanCode:    sub.PlanCode(),
					PeriodStart: sub.PeriodStart(),
					PeriodEnd:   sub.PeriodEnd(),
				}
				eventName, _ := events.NewEventName("billing.reconciliation.divergence_detected")
				// Batch com 1 item (< 200) → loop termina após esta chamada.
				s.subRepo.EXPECT().
					ListByStatusInBatch(mock.Anything, validStatuses, time.Time{}, zeroID, 200).
					Return([]*entities.Subscription{sub}, nil).Once()
				s.provider.EXPECT().
					FetchSubscription(mock.Anything, sub.ExternalSubscriptionID().String()).
					Return(remote, nil).Once()
				s.idGenerator.EXPECT().NewID().Return(_reconcileEventUUID).Once()
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
			},
			expect: func(report output.ReconciliationReport, err error) {
				s.NoError(err)
				s.Equal(1, report.Inspected)
				s.Equal(1, report.Diverged)
				s.Equal(1, report.Synced)
			},
		},
		{
			name: "FetchSubscription falha → erro propagado",
			setup: func() {
				s.subRepo.EXPECT().
					ListByStatusInBatch(mock.Anything, validStatuses, time.Time{}, zeroID, 200).
					Return([]*entities.Subscription{sub}, nil).Once()
				s.provider.EXPECT().
					FetchSubscription(mock.Anything, sub.ExternalSubscriptionID().String()).
					Return(services.CanonicalSubscription{}, errors.New("timeout")).Once()
			},
			expect: func(report output.ReconciliationReport, err error) {
				s.Error(err)
				s.Contains(err.Error(), "reconciliação")
			},
		},
		{
			name: "ListByStatusInBatch falha → erro propagado",
			setup: func() {
				s.subRepo.EXPECT().
					ListByStatusInBatch(mock.Anything, validStatuses, time.Time{}, zeroID, 200).
					Return(nil, errors.New("db error")).Once()
			},
			expect: func(report output.ReconciliationReport, err error) {
				s.Error(err)
				s.Contains(err.Error(), "reconciliação")
			},
		},
		{
			name: "período divergido: publica evento quando PeriodEnd difere",
			setup: func() {
				remote := services.CanonicalSubscription{
					ExternalID:  sub.ExternalSubscriptionID().String(),
					Status:      valueobjects.SubscriptionStatusActive,
					PlanCode:    sub.PlanCode(),
					PeriodStart: sub.PeriodStart(),
					PeriodEnd:   sub.PeriodEnd().Add(30 * 24 * time.Hour),
				}
				eventName, _ := events.NewEventName("billing.reconciliation.divergence_detected")
				s.subRepo.EXPECT().
					ListByStatusInBatch(mock.Anything, validStatuses, time.Time{}, zeroID, 200).
					Return([]*entities.Subscription{sub}, nil).Once()
				s.provider.EXPECT().
					FetchSubscription(mock.Anything, sub.ExternalSubscriptionID().String()).
					Return(remote, nil).Once()
				s.idGenerator.EXPECT().NewID().Return(_reconcileEventUUID).Once()
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
			},
			expect: func(report output.ReconciliationReport, err error) {
				s.NoError(err)
				s.Equal(1, report.Inspected)
				s.Equal(1, report.Diverged)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			scenario.setup()
			uc := s.buildUC()
			report, err := uc.Execute(s.ctx)
			scenario.expect(report, err)
		})
	}
}

// TestExecute_PublishFails verifica que erro no publisher dentro de reconcileOne propaga erro.
func (s *ReconcileSubscriptionsSuite) TestExecute_PublishFails() {
	sub := s.buildActiveSub()
	validStatuses := []valueobjects.SubscriptionStatus{
		valueobjects.SubscriptionStatusActive,
		valueobjects.SubscriptionStatusPastDue,
	}
	zeroID := entities.SubscriptionID{}

	remote := services.CanonicalSubscription{
		ExternalID:  sub.ExternalSubscriptionID().String(),
		Status:      valueobjects.SubscriptionStatusPastDue,
		PlanCode:    sub.PlanCode(),
		PeriodStart: sub.PeriodStart(),
		PeriodEnd:   sub.PeriodEnd(),
	}

	s.subRepo.EXPECT().
		ListByStatusInBatch(mock.Anything, validStatuses, time.Time{}, zeroID, 200).
		Return([]*entities.Subscription{sub}, nil).Once()
	s.provider.EXPECT().
		FetchSubscription(mock.Anything, sub.ExternalSubscriptionID().String()).
		Return(remote, nil).Once()
	s.idGenerator.EXPECT().NewID().Return(_reconcileEventUUID).Once()

	eventName, _ := events.NewEventName("billing.reconciliation.divergence_detected")
	// Retorna sem subscriptions → publisher.Publish retorna ErrNoSubscriptions → erro propagado.
	s.registry.EXPECT().
		SubscriptionsFor(eventName).
		Return([]outbox.Subscription{}).Once()

	uc := s.buildUC()
	_, err := uc.Execute(s.ctx)
	s.Error(err)
	s.Contains(err.Error(), "reconciliação")
}
