package services_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ThresholdWorkflowSuite struct {
	suite.Suite
}

func TestThresholdWorkflowSuite(t *testing.T) {
	suite.Run(t, new(ThresholdWorkflowSuite))
}

func (s *ThresholdWorkflowSuite) defaultConfig() services.ThresholdConfig {
	cat, _ := valueobjects.NewThresholdRatio(0.80)
	goal, _ := valueobjects.NewThresholdRatio(0.50)
	return services.ThresholdConfig{Category: cat, Goal: goal}
}

func (s *ThresholdWorkflowSuite) TestDecideAlerts_Category() {
	userID := uuid.New()
	budgetID := uuid.New()
	refDay := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	cfg := s.defaultConfig()

	cases := []struct {
		name    string
		spent   int64
		planned int64
		want    bool
	}{
		{"79.9% — sem alerta", 799, 1000, false},
		{"exatamente 80% — alerta", 800, 1000, true},
		{"80.01% — alerta", 8001, 10000, true},
		{"100% — alerta", 1000, 1000, true},
		{"acima de 100% — alerta", 1500, 1000, true},
		{"planned zero — sem alerta", 100, 0, false},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			snaps := []services.ActiveBudgetSnapshot{
				{
					UserID:       userID,
					BudgetID:     budgetID,
					Kind:         services.ThresholdAlertCategory,
					RootSlug:     valueobjects.RootSlugCustoFixo,
					PlannedCents: tc.planned,
					SpentCents:   tc.spent,
				},
			}
			alerts := services.ThresholdWorkflow{}.DecideAlerts(snaps, cfg, nil, refDay)
			if tc.want {
				s.Len(alerts, 1)
				s.Equal(services.ThresholdAlertCategory, alerts[0].Kind)
				s.Equal(refDay, alerts[0].RefDay)
				s.Equal(tc.planned-tc.spent, alerts[0].AmountRemainingCents)
			} else {
				s.Empty(alerts)
			}
		})
	}
}

func (s *ThresholdWorkflowSuite) TestDecideAlerts_Goal() {
	userID := uuid.New()
	budgetID := uuid.New()
	refDay := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	cfg := s.defaultConfig()

	cases := []struct {
		name    string
		spent   int64
		planned int64
		want    bool
	}{
		{"49% — sem alerta", 49, 100, false},
		{"50% — alerta", 50, 100, true},
		{"51% — alerta", 51, 100, true},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			snaps := []services.ActiveBudgetSnapshot{
				{
					UserID:       userID,
					BudgetID:     budgetID,
					Kind:         services.ThresholdAlertGoal,
					RootSlug:     valueobjects.RootSlugMetas,
					PlannedCents: tc.planned,
					SpentCents:   tc.spent,
				},
			}
			alerts := services.ThresholdWorkflow{}.DecideAlerts(snaps, cfg, nil, refDay)
			if tc.want {
				s.Len(alerts, 1)
				s.Equal(services.ThresholdAlertGoal, alerts[0].Kind)
			} else {
				s.Empty(alerts)
			}
		})
	}
}

func (s *ThresholdWorkflowSuite) TestDecideAlerts_DedupSkipsAlreadySent() {
	userID := uuid.New()
	budgetID := uuid.New()
	refDay := time.Date(2026, 6, 15, 12, 30, 0, 0, time.UTC)
	expectedDay := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	cfg := s.defaultConfig()

	sent := map[services.ThresholdSentKey]struct{}{
		{
			UserID:   userID,
			BudgetID: budgetID,
			Kind:     services.ThresholdAlertCategory,
			RefDay:   expectedDay,
		}: {},
	}

	snaps := []services.ActiveBudgetSnapshot{
		{
			UserID:       userID,
			BudgetID:     budgetID,
			Kind:         services.ThresholdAlertCategory,
			RootSlug:     valueobjects.RootSlugCustoFixo,
			PlannedCents: 1000,
			SpentCents:   900,
		},
	}

	alerts := services.ThresholdWorkflow{}.DecideAlerts(snaps, cfg, sent, refDay)
	s.Empty(alerts)
}

func (s *ThresholdWorkflowSuite) TestDecideAlerts_TruncatesRefDayToUTCDay() {
	userID := uuid.New()
	budgetID := uuid.New()
	refDay := time.Date(2026, 6, 15, 18, 45, 13, 0, time.UTC)
	expectedDay := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	cfg := s.defaultConfig()

	snaps := []services.ActiveBudgetSnapshot{
		{
			UserID:       userID,
			BudgetID:     budgetID,
			Kind:         services.ThresholdAlertCategory,
			RootSlug:     valueobjects.RootSlugCustoFixo,
			PlannedCents: 1000,
			SpentCents:   900,
		},
	}

	alerts := services.ThresholdWorkflow{}.DecideAlerts(snaps, cfg, nil, refDay)
	s.Len(alerts, 1)
	s.Equal(expectedDay, alerts[0].RefDay)
	s.Equal(int32(9000), alerts[0].PercentUsedBps)
}

func (s *ThresholdWorkflowSuite) TestDecideAlerts_MultipleSnapshots() {
	userID := uuid.New()
	refDay := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	cfg := s.defaultConfig()

	snaps := []services.ActiveBudgetSnapshot{
		{UserID: userID, BudgetID: uuid.New(), Kind: services.ThresholdAlertCategory, RootSlug: valueobjects.RootSlugCustoFixo, PlannedCents: 1000, SpentCents: 900},
		{UserID: userID, BudgetID: uuid.New(), Kind: services.ThresholdAlertCategory, RootSlug: valueobjects.RootSlugPrazeres, PlannedCents: 1000, SpentCents: 100},
		{UserID: userID, BudgetID: uuid.New(), Kind: services.ThresholdAlertGoal, RootSlug: valueobjects.RootSlugMetas, PlannedCents: 1000, SpentCents: 600},
	}

	alerts := services.ThresholdWorkflow{}.DecideAlerts(snaps, cfg, nil, refDay)
	s.Len(alerts, 2)
}
