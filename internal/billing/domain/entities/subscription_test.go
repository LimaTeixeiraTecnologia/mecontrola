package entities_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type SubscriptionSuite struct {
	suite.Suite
}

func TestSubscriptionSuite(t *testing.T) {
	suite.Run(t, new(SubscriptionSuite))
}

func (s *SubscriptionSuite) TestActivate_SetsActiveAndPeriodEndFromOccurredAt() {
	subscription := newSubscription(s.T())
	occurredAt := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)

	err := subscription.Activate(occurredAt)

	s.Require().NoError(err)
	s.Equal(valueobjects.StatusActive, subscription.Status())
	s.Equal(occurredAt.Add(30*24*time.Hour), subscription.PeriodEnd())
	s.True(subscription.GraceEnd().IsZero())
	s.Equal(occurredAt, subscription.LastEventAt())
}

func (s *SubscriptionSuite) TestRenew_ExtendsFromCurrentPeriodEndWhenSubscriptionStillActive() {
	subscription := newSubscription(s.T())
	initialAt := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	s.Require().NoError(subscription.Activate(initialAt))

	renewedAt := initialAt.Add(10 * 24 * time.Hour)
	err := subscription.Renew(renewedAt)

	s.Require().NoError(err)
	s.Equal(valueobjects.StatusActive, subscription.Status())
	s.Equal(initialAt.Add(60*24*time.Hour), subscription.PeriodEnd())
	s.Equal(renewedAt, subscription.LastEventAt())
}

func (s *SubscriptionSuite) TestRenew_ReactivatesCanceledPendingAndStartsFromOccurredAtWhenExpired() {
	subscription := newSubscription(s.T())
	initialAt := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s.Require().NoError(subscription.Activate(initialAt))
	s.Require().NoError(subscription.MarkCanceled(initialAt.Add(5 * 24 * time.Hour)))

	renewedAt := initialAt.Add(45 * 24 * time.Hour)
	err := subscription.Renew(renewedAt)

	s.Require().NoError(err)
	s.Equal(valueobjects.StatusActive, subscription.Status())
	s.Equal(renewedAt.Add(30*24*time.Hour), subscription.PeriodEnd())
	s.True(subscription.GraceEnd().IsZero())
}

func (s *SubscriptionSuite) TestMarkPastDue_SetsGraceWindowFromOccurredAt() {
	subscription := newSubscription(s.T())
	activatedAt := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	s.Require().NoError(subscription.Activate(activatedAt))

	lateAt := activatedAt.Add(31 * 24 * time.Hour)
	err := subscription.MarkPastDue(lateAt, 3*24*time.Hour)

	s.Require().NoError(err)
	s.Equal(valueobjects.StatusPastDue, subscription.Status())
	s.Equal(lateAt.Add(3*24*time.Hour), subscription.GraceEnd())
	s.Equal(lateAt, subscription.LastEventAt())
}

func (s *SubscriptionSuite) TestMarkCanceled_PreservesPeriodEnd() {
	subscription := newSubscription(s.T())
	activatedAt := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	s.Require().NoError(subscription.Activate(activatedAt))
	expectedPeriodEnd := subscription.PeriodEnd()

	err := subscription.MarkCanceled(activatedAt.Add(5 * 24 * time.Hour))

	s.Require().NoError(err)
	s.Equal(valueobjects.StatusCanceledPending, subscription.Status())
	s.Equal(expectedPeriodEnd, subscription.PeriodEnd())
	s.True(subscription.GraceEnd().IsZero())
}

func (s *SubscriptionSuite) TestMarkRefunded_IsTerminalAndClearsGrace() {
	subscription := newSubscription(s.T())
	activatedAt := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	s.Require().NoError(subscription.Activate(activatedAt))
	s.Require().NoError(subscription.MarkPastDue(activatedAt.Add(31*24*time.Hour), 3*24*time.Hour))

	refundedAt := activatedAt.Add(32 * 24 * time.Hour)
	err := subscription.MarkRefunded(refundedAt)

	s.Require().NoError(err)
	s.Equal(valueobjects.StatusRefunded, subscription.Status())
	s.True(subscription.GraceEnd().IsZero())
	s.Equal(refundedAt, subscription.LastEventAt())

	err = subscription.MarkCanceled(refundedAt.Add(24 * time.Hour))
	s.Require().ErrorIs(err, entities.ErrTransitionNotAllowed)
}

func (s *SubscriptionSuite) TestMethods_RejectZeroOccurredAt() {
	subscription := newSubscription(s.T())

	assert.ErrorIs(s.T(), subscription.Activate(time.Time{}), entities.ErrOccurredAtRequired)
	assert.ErrorIs(s.T(), subscription.Renew(time.Time{}), entities.ErrOccurredAtRequired)
	assert.ErrorIs(s.T(), subscription.MarkPastDue(time.Time{}, 3*24*time.Hour), entities.ErrOccurredAtRequired)
	assert.ErrorIs(s.T(), subscription.MarkCanceled(time.Time{}), entities.ErrOccurredAtRequired)
	assert.ErrorIs(s.T(), subscription.MarkRefunded(time.Time{}), entities.ErrOccurredAtRequired)
}

func newSubscription(t *testing.T) entities.Subscription {
	t.Helper()

	plan, err := valueobjects.NewPlan("MONTHLY", 30)
	require.NoError(t, err)
	token, err := valueobjects.NewFunnelToken("funnel-token")
	require.NoError(t, err)

	return entities.NewSubscription(plan, token)
}
