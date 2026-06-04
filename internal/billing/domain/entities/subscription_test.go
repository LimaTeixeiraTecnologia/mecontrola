package entities_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	billingdomain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	identityentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	identityservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/services"
)

var (
	_validUserID, _    = identityentities.NewUserID("550e8400-e29b-41d4-a716-446655440000")
	_validSubID, _     = entities.NewSubscriptionID("sub-001")
	_validWebhookID, _ = valueobjects.NewWebhookEventID("550e8400-e29b-41d4-a716-446655440001")
	_now               = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	_periodStart       = _now
	_periodEnd         = _now.Add(30 * 24 * time.Hour)
)

func validParams() entities.NewSubscriptionParams {
	return entities.NewSubscriptionParams{
		ID:                 _validSubID,
		UserID:             _validUserID,
		Provider:           "kiwify",
		ExternalSubID:      mustExternalSubID("ext-sub-001"),
		PlanCode:           valueobjects.PlanCodeMonthly,
		InitialStatus:      valueobjects.SubscriptionStatusActive,
		PeriodStart:        _periodStart,
		PeriodEnd:          _periodEnd,
		LastEventAt:        _now,
		LastWebhookEventID: _validWebhookID,
		CreatedAt:          _now,
	}
}

func mustExternalSubID(v string) valueobjects.ExternalSubscriptionID {
	id, err := valueobjects.NewExternalSubscriptionID(v)
	if err != nil {
		panic(err)
	}
	return id
}

// --- SubscriptionSuite ---

type SubscriptionSuite struct {
	suite.Suite
}

func TestSubscriptionSuite(t *testing.T) {
	suite.Run(t, new(SubscriptionSuite))
}

func (s *SubscriptionSuite) TestNewSubscription_Valid() {
	sub, err := entities.NewSubscription(validParams())
	s.NoError(err)
	s.NotNil(sub)
	s.Equal("sub-001", sub.ID().String())
	s.Equal("ext-sub-001", sub.ExternalSubscriptionID().String())
}

func (s *SubscriptionSuite) TestNewSubscription_EmptyID() {
	p := validParams()
	p.ID = entities.SubscriptionID{}
	_, err := entities.NewSubscription(p)
	s.True(errors.Is(err, billingdomain.ErrSubscriptionRequiresID))
}

func (s *SubscriptionSuite) TestNewSubscription_EmptyProvider() {
	p := validParams()
	p.Provider = ""
	_, err := entities.NewSubscription(p)
	s.True(errors.Is(err, billingdomain.ErrSubscriptionRequiresProvider))
}

func (s *SubscriptionSuite) TestNewSubscription_InvalidInitialStatus_Expired() {
	p := validParams()
	p.InitialStatus = valueobjects.SubscriptionStatusExpired
	_, err := entities.NewSubscription(p)
	s.True(errors.Is(err, billingdomain.ErrSubscriptionInitialStatusInvalid))
}

func (s *SubscriptionSuite) TestNewSubscription_InvalidInitialStatus_Refunded() {
	p := validParams()
	p.InitialStatus = valueobjects.SubscriptionStatusRefunded
	_, err := entities.NewSubscription(p)
	s.True(errors.Is(err, billingdomain.ErrSubscriptionInitialStatusInvalid))
}

func (s *SubscriptionSuite) TestNewSubscription_InvalidInitialStatus_PastDue() {
	p := validParams()
	p.InitialStatus = valueobjects.SubscriptionStatusPastDue
	_, err := entities.NewSubscription(p)
	s.True(errors.Is(err, billingdomain.ErrSubscriptionInitialStatusInvalid))
}

func (s *SubscriptionSuite) TestNewSubscription_PeriodEndBeforeStart() {
	p := validParams()
	p.PeriodEnd = p.PeriodStart.Add(-time.Hour)
	_, err := entities.NewSubscription(p)
	s.True(errors.Is(err, billingdomain.ErrSubscriptionRequiresPeriod))
}

func (s *SubscriptionSuite) TestNewSubscription_PeriodEndEqualStart() {
	p := validParams()
	p.PeriodEnd = p.PeriodStart
	_, err := entities.NewSubscription(p)
	s.True(errors.Is(err, billingdomain.ErrSubscriptionRequiresPeriod))
}

func (s *SubscriptionSuite) TestNewSubscription_ZeroPeriodStart() {
	p := validParams()
	p.PeriodStart = time.Time{}
	_, err := entities.NewSubscription(p)
	s.True(errors.Is(err, billingdomain.ErrSubscriptionRequiresPeriod))
}

func (s *SubscriptionSuite) TestNewSubscription_UnknownPlanCode() {
	p := validParams()
	p.PlanCode = valueobjects.PlanCodeUnknown
	_, err := entities.NewSubscription(p)
	s.Error(err)
	s.True(errors.Is(err, valueobjects.ErrUnknownPlanCode))
}

func (s *SubscriptionSuite) TestNewSubscription_TrialingStatus() {
	p := validParams()
	p.InitialStatus = valueobjects.SubscriptionStatusTrialing
	sub, err := entities.NewSubscription(p)
	s.NoError(err)
	s.NotNil(sub)
}

func (s *SubscriptionSuite) TestActivate_FromTrialing() {
	p := validParams()
	p.InitialStatus = valueobjects.SubscriptionStatusTrialing
	sub, _ := entities.NewSubscription(p)

	newEnd := _periodEnd.Add(30 * 24 * time.Hour)
	period := services.PeriodChange{NewStart: _periodEnd, NewEnd: newEnd}
	at := _now.Add(time.Hour)

	err := sub.Activate(at, period)
	s.NoError(err)
	s.Equal(identityservices.StatusActive, sub.Status())
	s.Equal(at, sub.LastEventAt())
	s.Equal(newEnd, sub.CurrentPeriodEnd())
}

func (s *SubscriptionSuite) TestActivate_FromRefunded_IllegalTransition() {
	sub := makeSubWithStatus(s, valueobjects.SubscriptionStatusRefunded)
	err := sub.Activate(_now, services.NoPeriodChange())
	s.True(errors.Is(err, services.ErrIllegalTransition))
}

func (s *SubscriptionSuite) TestRenew_FromActive() {
	sub := makeSubWithStatus(s, valueobjects.SubscriptionStatusActive)
	newEnd := _periodEnd.Add(30 * 24 * time.Hour)
	period := services.PeriodChange{NewStart: _periodEnd, NewEnd: newEnd}

	err := sub.Renew(_now.Add(time.Hour), period)
	s.NoError(err)
	s.Equal(identityservices.StatusActive, sub.Status())
	s.Equal(newEnd, sub.CurrentPeriodEnd())
}

func (s *SubscriptionSuite) TestMarkPastDue_FromActive() {
	sub := makeSubWithStatus(s, valueobjects.SubscriptionStatusActive)
	at := _now.Add(time.Hour)

	err := sub.MarkPastDue(at)
	s.NoError(err)
	s.Equal(identityservices.StatusPastDue, sub.Status())
	s.Equal(at, sub.LastEventAt())
	s.Equal(_periodEnd, sub.CurrentPeriodEnd(), "period_end deve ser preservado em MarkPastDue")
	expectedGrace := at.Add(services.DefaultGracePeriod)
	s.Equal(expectedGrace, sub.GracePeriodEnd())
}

func (s *SubscriptionSuite) TestMarkPastDue_FromTrialing_IllegalTransition() {
	sub := makeSubWithStatus(s, valueobjects.SubscriptionStatusTrialing)
	err := sub.MarkPastDue(_now)
	s.True(errors.Is(err, services.ErrIllegalTransition))
}

func (s *SubscriptionSuite) TestCancel_FromActive() {
	sub := makeSubWithStatus(s, valueobjects.SubscriptionStatusActive)
	err := sub.Cancel(_now.Add(time.Hour))
	s.NoError(err)
	s.Equal(identityservices.StatusCanceledPending, sub.Status())
}

func (s *SubscriptionSuite) TestExpire_FromTrialing() {
	sub := makeSubWithStatus(s, valueobjects.SubscriptionStatusTrialing)
	err := sub.Expire(_now.Add(time.Hour))
	s.NoError(err)
	s.Equal(identityservices.StatusExpired, sub.Status())
}

func (s *SubscriptionSuite) TestExpire_FromActive_IllegalTransition() {
	sub := makeSubWithStatus(s, valueobjects.SubscriptionStatusActive)
	err := sub.Expire(_now)
	s.True(errors.Is(err, services.ErrIllegalTransition))
}

// --- RefundSuite ---

type RefundSuite struct {
	suite.Suite
}

func TestRefundSuite(t *testing.T) {
	suite.Run(t, new(RefundSuite))
}

func (s *RefundSuite) TestRefund_FromEachNonTerminalState() {
	type row struct {
		name   string
		status valueobjects.SubscriptionStatus
	}
	cases := []row{
		{"from Active", valueobjects.SubscriptionStatusActive},
		{"from PastDue", valueobjects.SubscriptionStatusPastDue},
		{"from CanceledPending", valueobjects.SubscriptionStatusCanceledPending},
	}

	amount, _ := valueobjects.NewMoneyBRL(5000)

	for _, tc := range cases {
		s.Run(tc.name, func() {
			sub := makeSubWithStatusT(s.T(), tc.status)
			err := sub.Refund(_now.Add(time.Hour), amount, valueobjects.TransitionReasonChargebackReceived)
			s.NoError(err)
			s.Equal(identityservices.StatusRefunded, sub.Status())
			s.Equal(amount.Cents(), sub.RefundAmountCents().Cents())
		})
	}
}

func (s *RefundSuite) TestRefund_FromTerminalStates_IllegalTransition() {
	type row struct {
		name   string
		status valueobjects.SubscriptionStatus
	}
	cases := []row{
		{"from Expired", valueobjects.SubscriptionStatusExpired},
		{"from Refunded", valueobjects.SubscriptionStatusRefunded},
	}

	amount, _ := valueobjects.NewMoneyBRL(0)

	for _, tc := range cases {
		s.Run(tc.name, func() {
			sub := makeSubWithStatusT(s.T(), tc.status)
			err := sub.Refund(_now, amount, valueobjects.TransitionReasonChargebackReceived)
			s.True(errors.Is(err, services.ErrIllegalTransition))
		})
	}
}

func (s *RefundSuite) TestRefund_StoresAmount() {
	sub := makeSubWithStatusT(s.T(), valueobjects.SubscriptionStatusPastDue)
	amount, _ := valueobjects.NewMoneyBRL(12345)
	_ = sub.Refund(_now, amount, valueobjects.TransitionReasonChargebackReceived)
	s.Equal(int64(12345), sub.RefundAmountCents().Cents())
}

// --- IdentitySatisfactionSuite ---

type IdentitySatisfactionSuite struct {
	suite.Suite
}

func TestIdentitySatisfactionSuite(t *testing.T) {
	suite.Run(t, new(IdentitySatisfactionSuite))
}

func (s *IdentitySatisfactionSuite) TestSubscriptionImplementsIdentityContract() {
	sub, err := entities.NewSubscription(validParams())
	s.Require().NoError(err)

	checker := identityservices.NewEntitlementChecker()
	result := checker.IsEntitled(sub, _now.Add(-time.Hour))
	s.True(result, "assinatura ACTIVE com period_end no futuro deve ter entitlement granted")
}

func (s *IdentitySatisfactionSuite) TestEntitlementDeniedAfterPeriodEnd() {
	sub, _ := entities.NewSubscription(validParams())
	checker := identityservices.NewEntitlementChecker()
	result := checker.IsEntitled(sub, _periodEnd.Add(time.Hour))
	s.False(result, "assinatura ACTIVE com period_end no passado deve ter entitlement denied")
}

func (s *SubscriptionSuite) TestAccessors() {
	sub, _ := entities.NewSubscription(validParams())
	s.Equal(_validUserID.String(), sub.UserID().String())
	s.Equal(valueobjects.SubscriptionStatusActive, sub.InternalStatus())
	s.Equal(_periodEnd, sub.PeriodEnd())
	s.Equal(valueobjects.PlanCodeMonthly, sub.PlanCode())
	s.Equal(_now, sub.CreatedAt())
	s.Equal(_now, sub.UpdatedAt())
	s.Nil(sub.DeletedAt())
	s.Equal(_validWebhookID.String(), sub.LastWebhookEventID().String())
	s.Equal("kiwify", sub.Provider())
	s.Equal(_periodStart, sub.PeriodStart())
	snap := sub.SnapshotForNotification()
	s.NotNil(snap)
	s.Equal("sub-001", snap["subscription_id"])
}

func (s *SubscriptionSuite) TestNewSubscriptionID_Empty() {
	_, err := entities.NewSubscriptionID("")
	s.Error(err)
}

func (s *SubscriptionSuite) TestNewSubscriptionID_Valid() {
	id, err := entities.NewSubscriptionID("sub-xyz")
	s.NoError(err)
	s.Equal("sub-xyz", id.String())
	s.False(id.IsZero())
}

func (s *SubscriptionSuite) TestStatus_AllValues() {
	statuses := []struct {
		billing  valueobjects.SubscriptionStatus
		identity identityservices.SubscriptionStatus
	}{
		{valueobjects.SubscriptionStatusTrialing, identityservices.StatusTrialing},
		{valueobjects.SubscriptionStatusActive, identityservices.StatusActive},
		{valueobjects.SubscriptionStatusPastDue, identityservices.StatusPastDue},
		{valueobjects.SubscriptionStatusCanceledPending, identityservices.StatusCanceledPending},
		{valueobjects.SubscriptionStatusExpired, identityservices.StatusExpired},
		{valueobjects.SubscriptionStatusRefunded, identityservices.StatusRefunded},
		{valueobjects.SubscriptionStatusUnknown, identityservices.StatusUnknown},
	}
	for _, tc := range statuses {
		sub := makeSubWithStatus(s, tc.billing)
		s.Equal(tc.identity, sub.Status(), "billing status %v should map to identity status %v", tc.billing, tc.identity)
	}
}

func (s *SubscriptionSuite) TestRenew_FromPastDue() {
	sub := makeSubWithStatus(s, valueobjects.SubscriptionStatusPastDue)
	newEnd := _periodEnd.Add(30 * 24 * time.Hour)
	period := services.PeriodChange{NewStart: _periodEnd, NewEnd: newEnd}

	err := sub.Renew(_now.Add(time.Hour), period)
	s.NoError(err)
	s.Equal(identityservices.StatusActive, sub.Status())
}

// --- helpers ---

func makeSubWithStatus(s *SubscriptionSuite, status valueobjects.SubscriptionStatus) *entities.Subscription {
	return makeSubWithStatusT(s.T(), status)
}

func makeSubWithStatusT(t *testing.T, status valueobjects.SubscriptionStatus) *entities.Subscription {
	t.Helper()
	period, _ := valueobjects.NewBillingPeriodFor(valueobjects.PlanCodeMonthly)
	return entities.RehydrateSubscription(entities.RehydrateSubscriptionParams{
		ID:                 _validSubID,
		UserID:             _validUserID,
		Provider:           "kiwify",
		ExternalSubID:      mustExternalSubID("ext-sub-001"),
		PlanCode:           valueobjects.PlanCodeMonthly,
		Status:             status,
		Period:             period,
		PeriodStart:        _periodStart,
		PeriodEnd:          _periodEnd,
		LastEventAt:        _now,
		LastWebhookEventID: _validWebhookID,
		CreatedAt:          _now,
		UpdatedAt:          _now,
	})
}
