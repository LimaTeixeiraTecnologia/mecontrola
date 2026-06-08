package services_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type TransitionsSuite struct {
	suite.Suite
}

func TestTransitionsSuite(t *testing.T) {
	suite.Run(t, new(TransitionsSuite))
}

func (s *TransitionsSuite) SetupTest() {}

func (s *TransitionsSuite) TestTransitionService() {
	type args struct {
		current     valueobjects.Status
		next        valueobjects.Status
		trigger     services.Trigger
		occurredAt  time.Time
		lastEventAt time.Time
	}

	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	scenarios := []struct {
		name   string
		args   args
		expect func(services.TransitionService)
	}{
		{
			name: "deve permitir transicao de active para past due",
			args: args{
				current: valueobjects.StatusActive,
				next:    valueobjects.StatusPastDue,
			},
			expect: func(transitionService services.TransitionService) {
				assert.True(s.T(), transitionService.CanTransition(valueobjects.StatusActive, valueobjects.StatusPastDue))
			},
		},
		{
			name: "deve bloquear transicao de refunded para active",
			args: args{
				current: valueobjects.StatusRefunded,
				next:    valueobjects.StatusActive,
			},
			expect: func(transitionService services.TransitionService) {
				assert.False(s.T(), transitionService.CanTransition(valueobjects.StatusRefunded, valueobjects.StatusActive))
			},
		},
		{
			name: "deve mapear bootstrap por compra para active",
			args: args{
				trigger: services.TriggerSaleApproved,
			},
			expect: func(transitionService services.TransitionService) {
				status, ok := transitionService.TargetStatus(0, services.TriggerSaleApproved)
				assert.True(s.T(), ok)
				assert.Equal(s.T(), valueobjects.StatusActive, status)
			},
		},
		{
			name: "deve mapear renovacao em past due para active",
			args: args{
				current: valueobjects.StatusPastDue,
				trigger: services.TriggerSubscriptionRenewed,
			},
			expect: func(transitionService services.TransitionService) {
				status, ok := transitionService.TargetStatus(valueobjects.StatusPastDue, services.TriggerSubscriptionRenewed)
				assert.True(s.T(), ok)
				assert.Equal(s.T(), valueobjects.StatusActive, status)
			},
		},
		{
			name: "deve mapear late em active para past due",
			args: args{
				current: valueobjects.StatusActive,
				trigger: services.TriggerSubscriptionLate,
			},
			expect: func(transitionService services.TransitionService) {
				status, ok := transitionService.TargetStatus(valueobjects.StatusActive, services.TriggerSubscriptionLate)
				assert.True(s.T(), ok)
				assert.Equal(s.T(), valueobjects.StatusPastDue, status)
			},
		},
		{
			name: "deve mapear cancelamento em past due para canceled pending",
			args: args{
				current: valueobjects.StatusPastDue,
				trigger: services.TriggerSubscriptionCanceled,
			},
			expect: func(transitionService services.TransitionService) {
				status, ok := transitionService.TargetStatus(valueobjects.StatusPastDue, services.TriggerSubscriptionCanceled)
				assert.True(s.T(), ok)
				assert.Equal(s.T(), valueobjects.StatusCanceledPending, status)
			},
		},
		{
			name: "deve manter refunded quando trigger de refund ocorrer novamente",
			args: args{
				current: valueobjects.StatusRefunded,
				trigger: services.TriggerRefunded,
			},
			expect: func(transitionService services.TransitionService) {
				status, ok := transitionService.TargetStatus(valueobjects.StatusRefunded, services.TriggerRefunded)
				assert.True(s.T(), ok)
				assert.Equal(s.T(), valueobjects.StatusRefunded, status)
			},
		},
		{
			name: "deve bloquear cancelamento a partir de expired",
			args: args{
				current: valueobjects.StatusExpired,
				trigger: services.TriggerSubscriptionCanceled,
			},
			expect: func(transitionService services.TransitionService) {
				status, ok := transitionService.TargetStatus(valueobjects.StatusExpired, services.TriggerSubscriptionCanceled)
				assert.False(s.T(), ok)
				assert.Equal(s.T(), valueobjects.Status(0), status)
			},
		},
		{
			name: "deve identificar renewed antigo apos late recente como regressao",
			args: args{
				current:     valueobjects.StatusPastDue,
				trigger:     services.TriggerSubscriptionRenewed,
				occurredAt:  now.Add(-2 * time.Hour),
				lastEventAt: now,
			},
			expect: func(transitionService services.TransitionService) {
				assert.True(s.T(), transitionService.IsRegression(valueobjects.StatusPastDue, services.TriggerSubscriptionRenewed, now.Add(-2*time.Hour), now))
			},
		},
		{
			name: "deve identificar compra antiga apos expiracao como regressao",
			args: args{
				current:     valueobjects.StatusExpired,
				trigger:     services.TriggerSaleApproved,
				occurredAt:  now.Add(-2 * time.Hour),
				lastEventAt: now,
			},
			expect: func(transitionService services.TransitionService) {
				assert.True(s.T(), transitionService.IsRegression(valueobjects.StatusExpired, services.TriggerSaleApproved, now.Add(-2*time.Hour), now))
			},
		},
		{
			name: "deve ignorar late antigo quando o status permanece igual",
			args: args{
				current:     valueobjects.StatusPastDue,
				trigger:     services.TriggerSubscriptionLate,
				occurredAt:  now.Add(-2 * time.Hour),
				lastEventAt: now,
			},
			expect: func(transitionService services.TransitionService) {
				assert.False(s.T(), transitionService.IsRegression(valueobjects.StatusPastDue, services.TriggerSubscriptionLate, now.Add(-2*time.Hour), now))
			},
		},
		{
			name: "deve ignorar evento mais novo em relacao ao atual",
			args: args{
				current:     valueobjects.StatusPastDue,
				trigger:     services.TriggerSubscriptionRenewed,
				occurredAt:  now.Add(2 * time.Hour),
				lastEventAt: now,
			},
			expect: func(transitionService services.TransitionService) {
				assert.False(s.T(), transitionService.IsRegression(valueobjects.StatusPastDue, services.TriggerSubscriptionRenewed, now.Add(2*time.Hour), now))
			},
		},
		{
			name: "deve considerar que refund sempre prevalece",
			args: args{
				current:     valueobjects.StatusActive,
				trigger:     services.TriggerRefunded,
				occurredAt:  now.Add(-2 * time.Hour),
				lastEventAt: now,
			},
			expect: func(transitionService services.TransitionService) {
				assert.False(s.T(), transitionService.IsRegression(valueobjects.StatusActive, services.TriggerRefunded, now.Add(-2*time.Hour), now))
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			transitionService := services.NewTransitionService()
			_ = scenario.args
			scenario.expect(transitionService)
		})
	}
}
