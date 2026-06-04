package usecases_test

import (
	"context"
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
	identityentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	identityvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability/fakes"
)

const (
	_entitlementUserUUID = "550e8400-e29b-41d4-a716-446655440020"
	_entitlementSubUUID  = "550e8400-e29b-41d4-a716-446655440021"
	_entitlementExtSub   = "sub-ext-check-001"
)

type CheckEntitlementSuite struct {
	suite.Suite

	ctx     context.Context
	now     time.Time
	subRepo *billingmocks.SubscriptionRepository
	cache   *billingmocks.EntitlementCache
}

func TestCheckEntitlementSuite(t *testing.T) {
	suite.Run(t, new(CheckEntitlementSuite))
}

func (s *CheckEntitlementSuite) SetupTest() {
	s.ctx = context.Background()
	s.now = time.Now().UTC()
	s.subRepo = billingmocks.NewSubscriptionRepository(s.T())
	s.cache = billingmocks.NewEntitlementCache(s.T())
}

func (s *CheckEntitlementSuite) buildUC() *usecases.CheckEntitlementUseCase {
	return usecases.NewCheckEntitlementUseCase(s.subRepo, s.cache, fakes.NoopObservability(), fakes.NoopUsecaseMetrics())
}

func (s *CheckEntitlementSuite) buildUserID() identityentities.UserID {
	id, _ := identityentities.NewUserID(_entitlementUserUUID)
	return id
}

func (s *CheckEntitlementSuite) buildActiveSub() *entities.Subscription {
	now := s.now
	userID := s.buildUserID()
	subID, _ := entities.NewSubscriptionID(_entitlementSubUUID)
	extSubID, _ := valueobjects.NewExternalSubscriptionID(_entitlementExtSub)
	webhookID, _ := valueobjects.NewWebhookEventID(_validUUID1)
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

func (s *CheckEntitlementSuite) buildPastDueSub() *entities.Subscription {
	now := s.now
	userID := s.buildUserID()
	subID, _ := entities.NewSubscriptionID(_entitlementSubUUID)
	extSubID, _ := valueobjects.NewExternalSubscriptionID(_entitlementExtSub)
	webhookID, _ := valueobjects.NewWebhookEventID(_validUUID1)
	sub, _ := entities.NewSubscription(entities.NewSubscriptionParams{
		ID:                 subID,
		UserID:             userID,
		Provider:           "kiwify",
		ExternalSubID:      extSubID,
		PlanCode:           valueobjects.PlanCodeMonthly,
		InitialStatus:      valueobjects.SubscriptionStatusActive,
		PeriodStart:        now.Add(-35 * 24 * time.Hour),
		PeriodEnd:          now.Add(-5 * 24 * time.Hour),
		LastEventAt:        now.Add(-35 * 24 * time.Hour),
		LastWebhookEventID: webhookID,
		CreatedAt:          now.Add(-35 * 24 * time.Hour),
	})
	// Transição legal: Active → PastDue.
	if sub != nil {
		_ = sub.MarkPastDue(now.Add(-5 * 24 * time.Hour))
	}
	return sub
}

func (s *CheckEntitlementSuite) TestExecute() {
	userID := s.buildUserID()
	in := input.CheckEntitlementInput{UserID: userID}

	type setupFn func()
	scenarios := []struct {
		name   string
		setup  setupFn
		expect func(decision output.EntitlementDecision, err error)
	}{
		{
			name: "cache hit: retorna decisão cacheada sem consultar repo",
			setup: func() {
				cached := output.EntitlementDecision{Status: "granted", Reason: "active_subscription"}
				s.cache.EXPECT().
					Get(userID).
					Return(cached, true).Once()
			},
			expect: func(decision output.EntitlementDecision, err error) {
				s.NoError(err)
				s.Equal("granted", decision.Status)
			},
		},
		{
			name: "cache miss + subscription ativa → granted",
			setup: func() {
				sub := s.buildActiveSub()
				s.cache.EXPECT().
					Get(userID).
					Return(output.EntitlementDecision{}, false).Once()
				s.subRepo.EXPECT().
					FindActiveByUserID(mock.Anything, userID).
					Return(sub, nil).Once()
				s.cache.EXPECT().
					Set(userID, output.EntitlementDecision{
						Status:    "granted",
						Reason:    "active_subscription",
						ExpiresAt: sub.CurrentPeriodEnd(),
					}, 5*time.Minute).Once()
			},
			expect: func(decision output.EntitlementDecision, err error) {
				s.NoError(err)
				s.Equal("granted", decision.Status)
				s.Equal("active_subscription", decision.Reason)
			},
		},
		{
			name: "cache miss + sem subscription → denied",
			setup: func() {
				s.cache.EXPECT().
					Get(userID).
					Return(output.EntitlementDecision{}, false).Once()
				s.subRepo.EXPECT().
					FindActiveByUserID(mock.Anything, userID).
					Return(nil, usecases.ErrSubscriptionNotFound).Once()
				s.cache.EXPECT().
					Set(userID, output.EntitlementDecision{
						Status: "denied",
						Reason: "no_active_subscription",
					}, 5*time.Minute).Once()
			},
			expect: func(decision output.EntitlementDecision, err error) {
				s.NoError(err)
				s.Equal("denied", decision.Status)
				s.Equal("no_active_subscription", decision.Reason)
			},
		},
		{
			name: "cache miss + subscription past_due dentro da grace → grace",
			setup: func() {
				sub := s.buildPastDueSub()
				s.cache.EXPECT().
					Get(userID).
					Return(output.EntitlementDecision{}, false).Once()
				s.subRepo.EXPECT().
					FindActiveByUserID(mock.Anything, userID).
					Return(sub, nil).Once()
				s.cache.EXPECT().
					Set(userID, output.EntitlementDecision{
						Status:    "grace",
						Reason:    "grace_period",
						ExpiresAt: sub.GracePeriodEnd(),
					}, 5*time.Minute).Once()
			},
			expect: func(decision output.EntitlementDecision, err error) {
				s.NoError(err)
				s.Equal("grace", decision.Status)
				s.Equal("grace_period", decision.Reason)
			},
		},
		{
			name: "erro transitório no repo retorna erro",
			setup: func() {
				s.cache.EXPECT().
					Get(userID).
					Return(output.EntitlementDecision{}, false).Once()
				s.subRepo.EXPECT().
					FindActiveByUserID(mock.Anything, userID).
					Return(nil, errors.New("connection refused")).Once()
			},
			expect: func(decision output.EntitlementDecision, err error) {
				s.Error(err)
				s.Contains(err.Error(), "check entitlement")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			scenario.setup()
			uc := s.buildUC()
			decision, err := uc.Execute(s.ctx, in)
			scenario.expect(decision, err)
		})
	}
}

// TestExecute_NilSubWithoutNotFoundError verifica que nil+nil (repo retorna nil, nil)
// é tratado como "sem subscription" → denied (linha de defesa adicional).
func (s *CheckEntitlementSuite) TestExecute_NilSubWithoutNotFoundError() {
	userID := s.buildUserID()
	in := input.CheckEntitlementInput{UserID: userID}

	s.cache.EXPECT().
		Get(userID).
		Return(output.EntitlementDecision{}, false).Once()
	s.subRepo.EXPECT().
		FindActiveByUserID(mock.Anything, userID).
		Return(nil, nil).Once()
	s.cache.EXPECT().
		Set(userID, output.EntitlementDecision{
			Status: "denied",
			Reason: "no_active_subscription",
		}, 5*time.Minute).Once()

	uc := s.buildUC()
	decision, err := uc.Execute(s.ctx, in)
	s.NoError(err)
	s.Equal("denied", decision.Status)
}

// TestExecute_ExpiredSubDenied verifica que subscription expirada (entitled=false, status=Expired) → denied.
func (s *CheckEntitlementSuite) TestExecute_ExpiredSubDenied() {
	now := s.now
	userID := s.buildUserID()
	in := input.CheckEntitlementInput{UserID: userID}

	// Subscription expirada: período encerrado há mais de 7 dias (fora da grace).
	subID, _ := entities.NewSubscriptionID(_entitlementSubUUID)
	extSubID, _ := valueobjects.NewExternalSubscriptionID(_entitlementExtSub)
	webhookID, _ := valueobjects.NewWebhookEventID(_validUUID1)
	// Usa RehydrateSubscription para criar diretamente com status Expired.
	expiredSub := entities.RehydrateSubscription(entities.RehydrateSubscriptionParams{
		ID:                 subID,
		UserID:             userID,
		Provider:           "kiwify",
		ExternalSubID:      extSubID,
		PlanCode:           valueobjects.PlanCodeMonthly,
		Status:             valueobjects.SubscriptionStatusExpired,
		PeriodStart:        now.Add(-60 * 24 * time.Hour),
		PeriodEnd:          now.Add(-30 * 24 * time.Hour),
		GracePeriodEnd:     now.Add(-23 * 24 * time.Hour),
		LastEventAt:        now.Add(-30 * 24 * time.Hour),
		LastWebhookEventID: webhookID,
		CreatedAt:          now.Add(-60 * 24 * time.Hour),
		UpdatedAt:          now.Add(-30 * 24 * time.Hour),
	})

	s.cache.EXPECT().
		Get(userID).
		Return(output.EntitlementDecision{}, false).Once()
	s.subRepo.EXPECT().
		FindActiveByUserID(mock.Anything, userID).
		Return(expiredSub, nil).Once()
	s.cache.EXPECT().
		Set(userID, output.EntitlementDecision{
			Status: "denied",
			Reason: "subscription_not_active",
		}, 5*time.Minute).Once()

	uc := s.buildUC()
	decision, err := uc.Execute(s.ctx, in)
	s.NoError(err)
	s.Equal("denied", decision.Status)
	s.Equal("subscription_not_active", decision.Reason)
}

func (s *CheckEntitlementSuite) TestExecute_NumberWhatsApp() {
	userID := s.buildUserID()
	_, err := identityvo.NewWhatsAppNumber("+5511999990000")
	s.NoError(err)
	in := input.CheckEntitlementInput{UserID: userID}

	sub := s.buildActiveSub()
	s.cache.EXPECT().Get(userID).Return(output.EntitlementDecision{}, false).Once()
	s.subRepo.EXPECT().FindActiveByUserID(mock.Anything, userID).Return(sub, nil).Once()
	s.cache.EXPECT().Set(userID, output.EntitlementDecision{
		Status:    "granted",
		Reason:    "active_subscription",
		ExpiresAt: sub.CurrentPeriodEnd(),
	}, 5*time.Minute).Once()

	uc := s.buildUC()
	decision, err := uc.Execute(s.ctx, in)
	s.NoError(err)
	s.Equal("granted", decision.Status)
}

func (s *CheckEntitlementSuite) TestExecute_GrantedTTLExpiresAtPeriodEnd() {
	userID := s.buildUserID()
	in := input.CheckEntitlementInput{UserID: userID}
	sub := s.buildActiveSub()
	nearEnd := s.now.Add(2 * time.Minute)
	sub = entities.RehydrateSubscription(entities.RehydrateSubscriptionParams{
		ID:                 sub.ID(),
		UserID:             sub.UserID(),
		Provider:           sub.Provider(),
		ExternalSubID:      sub.ExternalSubscriptionID(),
		PlanCode:           sub.PlanCode(),
		Status:             valueobjects.SubscriptionStatusActive,
		PeriodStart:        sub.PeriodStart(),
		PeriodEnd:          nearEnd,
		LastEventAt:        sub.LastEventAt(),
		LastWebhookEventID: sub.LastWebhookEventID(),
		CreatedAt:          sub.CreatedAt(),
		UpdatedAt:          sub.UpdatedAt(),
	})

	s.cache.EXPECT().Get(userID).Return(output.EntitlementDecision{}, false).Once()
	s.subRepo.EXPECT().FindActiveByUserID(mock.Anything, userID).Return(sub, nil).Once()
	s.cache.EXPECT().Set(userID, output.EntitlementDecision{
		Status:    "granted",
		Reason:    "active_subscription",
		ExpiresAt: nearEnd,
	}, mock.MatchedBy(func(ttl time.Duration) bool {
		return ttl > 0 && ttl <= 2*time.Minute
	})).Once()

	uc := s.buildUC()
	decision, err := uc.Execute(s.ctx, in)
	s.NoError(err)
	s.Equal("granted", decision.Status)
}
