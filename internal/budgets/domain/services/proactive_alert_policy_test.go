package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
)

type ProactiveAlertPolicySuite struct {
	suite.Suite
}

func TestProactiveAlertPolicySuite(t *testing.T) {
	suite.Run(t, new(ProactiveAlertPolicySuite))
}

func (s *ProactiveAlertPolicySuite) TestDecideCategoryKind() {
	scenarios := []struct {
		name         string
		spentCents   int64
		plannedCents int64
		expectKind   ThresholdAlertKind
		expectOK     bool
	}{
		{name: "abaixo de 80 nao alerta", spentCents: 799, plannedCents: 1000, expectOK: false},
		{name: "exatamente 80 alerta 80", spentCents: 800, plannedCents: 1000, expectKind: ThresholdAlertCategory80, expectOK: true},
		{name: "entre 80 e 100 alerta 80", spentCents: 999, plannedCents: 1000, expectKind: ThresholdAlertCategory80, expectOK: true},
		{name: "noventa por cento continua alertando 80", spentCents: 900, plannedCents: 1000, expectKind: ThresholdAlertCategory80, expectOK: true},
		{name: "exatamente 100 alerta 100", spentCents: 1000, plannedCents: 1000, expectKind: ThresholdAlertCategory100, expectOK: true},
		{name: "acima de 100 alerta 100", spentCents: 1500, plannedCents: 1000, expectKind: ThresholdAlertCategory100, expectOK: true},
		{name: "planejado zero nao alerta", spentCents: 500, plannedCents: 0, expectOK: false},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			kind, ok := DecideCategoryKind(scenario.spentCents, scenario.plannedCents)
			s.Equal(scenario.expectOK, ok)
			if scenario.expectOK {
				s.Equal(scenario.expectKind, kind)
			}
		})
	}
}

func (s *ProactiveAlertPolicySuite) TestThreshold90NeverEmittable() {
	s.False(IsReleaseOneEmittable(ThresholdAlertCategory90))
	s.Equal(0, KindPriority(ThresholdAlertCategory90))

	for spent := int64(0); spent <= 1200; spent += 10 {
		kind, ok := DecideCategoryKind(spent, 1000)
		if ok {
			s.NotEqual(ThresholdAlertCategory90, kind)
		}
	}
}

func (s *ProactiveAlertPolicySuite) TestGoalAndLegacyKindsAreNotEmittable() {
	s.False(IsReleaseOneEmittable(ThresholdAlertGoal))
	s.False(IsReleaseOneEmittable(ThresholdAlertCategory))
}

func (s *ProactiveAlertPolicySuite) TestReleaseOneKindsAreEmittable() {
	for _, kind := range []ThresholdAlertKind{
		ThresholdAlertCategory80,
		ThresholdAlertCategory100,
		ThresholdAlertBudgetMissingMonthStart,
		ThresholdAlertBudgetNotReviewedDay3,
	} {
		s.True(IsReleaseOneEmittable(kind), kind.String())
		s.NotZero(KindPriority(kind), kind.String())
	}
}

func (s *ProactiveAlertPolicySuite) TestPriorityOrderMatchesSpec() {
	s.Less(KindPriority(ThresholdAlertCategory100), KindPriority(ThresholdAlertBudgetNotReviewedDay3))
	s.Less(KindPriority(ThresholdAlertBudgetNotReviewedDay3), KindPriority(ThresholdAlertBudgetMissingMonthStart))
	s.Less(KindPriority(ThresholdAlertBudgetMissingMonthStart), KindPriority(ThresholdAlertCategory80))
}

func (s *ProactiveAlertPolicySuite) TestDecideDailyRoundAlertsKeepsOnePerUserByPriority() {
	userA := uuid.New()
	userB := uuid.New()

	candidates := []DomainAlert{
		{UserID: userA, BudgetID: uuid.New(), Kind: ThresholdAlertCategory80},
		{UserID: userA, BudgetID: uuid.New(), Kind: ThresholdAlertCategory100},
		{UserID: userA, BudgetID: uuid.New(), Kind: ThresholdAlertBudgetNotReviewedDay3},
		{UserID: userB, BudgetID: uuid.New(), Kind: ThresholdAlertCategory80},
	}

	selected, suppressed := DecideDailyRoundAlerts(candidates, map[uuid.UUID]struct{}{})

	s.Require().Len(selected, 2)
	byUser := map[uuid.UUID]ThresholdAlertKind{}
	for _, alert := range selected {
		byUser[alert.UserID] = alert.Kind
	}
	s.Equal(ThresholdAlertCategory100, byUser[userA])
	s.Equal(ThresholdAlertCategory80, byUser[userB])

	s.Len(suppressed, 2)
	for _, drop := range suppressed {
		s.Equal(userA, drop.UserID)
		s.Equal(SuppressedByPriority, drop.Reason)
	}
}

func (s *ProactiveAlertPolicySuite) TestDecideDailyRoundAlertsSuppressesUserAlreadyAlertedInEarlierRound() {
	userA := uuid.New()
	userB := uuid.New()

	candidates := []DomainAlert{
		{UserID: userA, BudgetID: uuid.New(), Kind: ThresholdAlertBudgetMissingMonthStart},
		{UserID: userB, BudgetID: uuid.New(), Kind: ThresholdAlertCategory80},
	}
	alreadyAlertedToday := map[uuid.UUID]struct{}{
		userA: {},
	}

	selected, suppressed := DecideDailyRoundAlerts(candidates, alreadyAlertedToday)

	s.Require().Len(selected, 1)
	s.Equal(userB, selected[0].UserID)

	s.Require().Len(suppressed, 1)
	s.Equal(userA, suppressed[0].UserID)
	s.Equal(SuppressedByFrequency, suppressed[0].Reason)
}

func (s *ProactiveAlertPolicySuite) TestDecideDailyRoundAlertsSuppressesBlockedKinds() {
	userID := uuid.New()
	candidates := []DomainAlert{
		{UserID: userID, Kind: ThresholdAlertGoal},
		{UserID: userID, Kind: ThresholdAlertCategory90},
	}

	selected, suppressed := DecideDailyRoundAlerts(candidates, map[uuid.UUID]struct{}{})

	s.Empty(selected)
	s.Require().Len(suppressed, 2)
	for _, drop := range suppressed {
		s.Equal(SuppressedKindBlocked, drop.Reason)
	}
}

func (s *ProactiveAlertPolicySuite) TestDecideDailyRoundAlertsIsDeterministic() {
	userID := uuid.New()
	candidates := []DomainAlert{
		{UserID: userID, BudgetID: uuid.MustParse("00000000-0000-0000-0000-0000000000ff"), Kind: ThresholdAlertCategory80},
		{UserID: userID, BudgetID: uuid.MustParse("00000000-0000-0000-0000-00000000000a"), Kind: ThresholdAlertCategory80},
	}

	first, _ := DecideDailyRoundAlerts(candidates, map[uuid.UUID]struct{}{})
	second, _ := DecideDailyRoundAlerts(candidates, map[uuid.UUID]struct{}{})

	s.Require().Len(first, 1)
	s.Require().Len(second, 1)
	s.Equal(first[0].BudgetID, second[0].BudgetID)
}

func (s *ProactiveAlertPolicySuite) TestDecideBudgetLifecycleAlerts() {
	userID := uuid.New()
	budgetID := uuid.New()
	missing := []MissingBudgetSnapshot{{UserID: userID}}
	unreviewed := []UnreviewedBudgetSnapshot{{UserID: userID, BudgetID: budgetID}}
	empty := map[ThresholdSentKey]struct{}{}

	s.Run("dia 1 emite apenas orcamento ausente", func() {
		refDay := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
		alerts := DecideBudgetLifecycleAlerts(missing, unreviewed, empty, refDay)
		s.Require().Len(alerts, 1)
		s.Equal(ThresholdAlertBudgetMissingMonthStart, alerts[0].Kind)
		s.Equal(uuid.Nil, alerts[0].BudgetID)
	})

	s.Run("dia 3 emite ambos", func() {
		refDay := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
		alerts := DecideBudgetLifecycleAlerts(missing, unreviewed, empty, refDay)
		s.Require().Len(alerts, 2)
	})

	s.Run("dia 10 emite apenas nao revisado", func() {
		refDay := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
		alerts := DecideBudgetLifecycleAlerts(missing, unreviewed, empty, refDay)
		s.Require().Len(alerts, 1)
		s.Equal(ThresholdAlertBudgetNotReviewedDay3, alerts[0].Kind)
	})

	s.Run("dedup impede repeticao no mesmo dia", func() {
		refDay := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
		sent := map[ThresholdSentKey]struct{}{
			{UserID: userID, BudgetID: budgetID, Kind: ThresholdAlertBudgetNotReviewedDay3, RefDay: refDay}: {},
		}
		alerts := DecideBudgetLifecycleAlerts(missing, unreviewed, sent, refDay)
		s.Empty(alerts)
	})
}

func (s *ProactiveAlertPolicySuite) TestDedupDoesNotCollapse80And100() {
	userID := uuid.New()
	budgetID := uuid.New()
	refDay := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)

	sent := map[ThresholdSentKey]struct{}{
		{UserID: userID, BudgetID: budgetID, Kind: ThresholdAlertCategory80, RefDay: refDay}: {},
	}

	_, collapsed := sent[ThresholdSentKey{UserID: userID, BudgetID: budgetID, Kind: ThresholdAlertCategory100, RefDay: refDay}]
	s.False(collapsed)
}

func (s *ProactiveAlertPolicySuite) TestClosedTypesRejectUnknownValues() {
	s.False(ThresholdAlertKind(0).IsValid())
	s.False(ThresholdAlertKind(250).IsValid())
	s.False(SuppressionReason(0).IsValid())
	s.False(SuppressionReason(250).IsValid())
	s.False(TemplateCategory(0).IsValid())
	s.False(TemplateStatus(0).IsValid())

	s.True(RequiresExplicitOptIn(TemplateCategoryMarketing))
	s.False(RequiresExplicitOptIn(TemplateCategoryUtility))
	s.True(AllowsRealSend(TemplateStatusApproved))
	s.False(AllowsRealSend(TemplateStatusPending))
	s.False(AllowsRealSend(TemplateStatusRejected))
	s.False(AllowsRealSend(TemplateStatusUnknown))
}

func (s *ProactiveAlertPolicySuite) TestAlertContextValidityAndTopics() {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)

	s.True(IsAlertContextValid(now.Add(-1*time.Hour), now, 24*time.Hour))
	s.True(IsAlertContextValid(now.Add(-24*time.Hour), now, 24*time.Hour))
	s.False(IsAlertContextValid(now.Add(-25*time.Hour), now, 24*time.Hour))
	s.False(IsAlertContextValid(time.Time{}, now, 24*time.Hour))
	s.False(IsAlertContextValid(now, now, 0))
	s.False(IsAlertContextValid(now.Add(time.Hour), now, 24*time.Hour))

	for _, kind := range []ThresholdAlertKind{
		ThresholdAlertCategory80,
		ThresholdAlertCategory100,
		ThresholdAlertBudgetMissingMonthStart,
		ThresholdAlertBudgetNotReviewedDay3,
	} {
		s.NotEmpty(AlertFollowUpTopic(kind), kind.String())
	}
	s.Empty(AlertFollowUpTopic(ThresholdAlertGoal))
	s.Empty(AlertFollowUpTopic(ThresholdAlertCategory90))
}
