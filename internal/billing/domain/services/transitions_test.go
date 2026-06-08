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
		expect func(args, services.TransitionService)
	}{
		{
			name: "deve permitir transicao de active para past due",
			args: args{
				current: valueobjects.StatusActive,
				next:    valueobjects.StatusPastDue,
			},
			expect: func(args args, transitionService services.TransitionService) {
				assert.True(s.T(), transitionService.CanTransition(args.current, args.next))
			},
		},
		{
			name: "deve bloquear transicao de refunded para active",
			args: args{
				current: valueobjects.StatusRefunded,
				next:    valueobjects.StatusActive,
			},
			expect: func(args args, transitionService services.TransitionService) {
				assert.False(s.T(), transitionService.CanTransition(args.current, args.next))
			},
		},
		{
			name: "deve mapear bootstrap por compra para active",
			args: args{
				trigger: services.TriggerSaleApproved,
			},
			expect: func(args args, transitionService services.TransitionService) {
				status, ok := transitionService.TargetStatus(args.current, args.trigger)
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
			expect: func(args args, transitionService services.TransitionService) {
				status, ok := transitionService.TargetStatus(args.current, args.trigger)
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
			expect: func(args args, transitionService services.TransitionService) {
				status, ok := transitionService.TargetStatus(args.current, args.trigger)
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
			expect: func(args args, transitionService services.TransitionService) {
				status, ok := transitionService.TargetStatus(args.current, args.trigger)
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
			expect: func(args args, transitionService services.TransitionService) {
				status, ok := transitionService.TargetStatus(args.current, args.trigger)
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
			expect: func(args args, transitionService services.TransitionService) {
				status, ok := transitionService.TargetStatus(args.current, args.trigger)
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
			expect: func(args args, transitionService services.TransitionService) {
				assert.True(s.T(), transitionService.IsRegression(args.current, args.trigger, args.occurredAt, args.lastEventAt))
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
			expect: func(args args, transitionService services.TransitionService) {
				assert.True(s.T(), transitionService.IsRegression(args.current, args.trigger, args.occurredAt, args.lastEventAt))
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
			expect: func(args args, transitionService services.TransitionService) {
				assert.False(s.T(), transitionService.IsRegression(args.current, args.trigger, args.occurredAt, args.lastEventAt))
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
			expect: func(args args, transitionService services.TransitionService) {
				assert.False(s.T(), transitionService.IsRegression(args.current, args.trigger, args.occurredAt, args.lastEventAt))
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
			expect: func(args args, transitionService services.TransitionService) {
				assert.False(s.T(), transitionService.IsRegression(args.current, args.trigger, args.occurredAt, args.lastEventAt))
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			transitionService := services.NewTransitionService()
			scenario.expect(scenario.args, transitionService)
		})
	}
}
