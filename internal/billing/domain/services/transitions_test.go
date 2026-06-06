package services_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

func TestTransitionService_CanTransitionMatrix(t *testing.T) {
	t.Parallel()

	transitionService := services.NewTransitionService()
	statuses := []valueobjects.Status{
		valueobjects.StatusTrialing,
		valueobjects.StatusActive,
		valueobjects.StatusPastDue,
		valueobjects.StatusCanceledPending,
		valueobjects.StatusExpired,
		valueobjects.StatusRefunded,
	}
	expected := map[valueobjects.Status]map[valueobjects.Status]bool{
		valueobjects.StatusTrialing: {},
		valueobjects.StatusActive: {
			valueobjects.StatusActive:          true,
			valueobjects.StatusPastDue:         true,
			valueobjects.StatusCanceledPending: true,
			valueobjects.StatusRefunded:        true,
		},
		valueobjects.StatusPastDue: {
			valueobjects.StatusActive:          true,
			valueobjects.StatusPastDue:         true,
			valueobjects.StatusCanceledPending: true,
			valueobjects.StatusRefunded:        true,
		},
		valueobjects.StatusCanceledPending: {
			valueobjects.StatusActive:   true,
			valueobjects.StatusRefunded: true,
		},
		valueobjects.StatusExpired: {
			valueobjects.StatusActive:   true,
			valueobjects.StatusRefunded: true,
		},
		valueobjects.StatusRefunded: {},
	}

	for _, current := range statuses {
		current := current
		for _, next := range statuses {
			next := next
			t.Run(current.String()+"->"+next.String(), func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, expected[current][next], transitionService.CanTransition(current, next))
			})
		}
	}
}

func TestTransitionService_TargetStatus(t *testing.T) {
	t.Parallel()

	transitionService := services.NewTransitionService()
	tests := []struct {
		name       string
		current    valueobjects.Status
		trigger    services.Trigger
		wantStatus valueobjects.Status
		wantOK     bool
	}{
		{
			name:       "bootstrap from zero via purchase",
			trigger:    services.TriggerSaleApproved,
			wantStatus: valueobjects.StatusActive,
			wantOK:     true,
		},
		{
			name:       "renew from past due returns active",
			current:    valueobjects.StatusPastDue,
			trigger:    services.TriggerSubscriptionRenewed,
			wantStatus: valueobjects.StatusActive,
			wantOK:     true,
		},
		{
			name:       "late while active returns past due",
			current:    valueobjects.StatusActive,
			trigger:    services.TriggerSubscriptionLate,
			wantStatus: valueobjects.StatusPastDue,
			wantOK:     true,
		},
		{
			name:       "cancel while past due returns canceled pending",
			current:    valueobjects.StatusPastDue,
			trigger:    services.TriggerSubscriptionCanceled,
			wantStatus: valueobjects.StatusCanceledPending,
			wantOK:     true,
		},
		{
			name:       "refund while refunded keeps refunded",
			current:    valueobjects.StatusRefunded,
			trigger:    services.TriggerRefunded,
			wantStatus: valueobjects.StatusRefunded,
			wantOK:     true,
		},
		{
			name:    "cancel from expired is blocked",
			current: valueobjects.StatusExpired,
			trigger: services.TriggerSubscriptionCanceled,
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			status, ok := transitionService.TargetStatus(tt.current, tt.trigger)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantStatus, status)
		})
	}
}

func TestTransitionService_IsRegression(t *testing.T) {
	t.Parallel()

	transitionService := services.NewTransitionService()
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		current      valueobjects.Status
		trigger      services.Trigger
		occurredAt   time.Time
		lastEventAt  time.Time
		wantDecision bool
	}{
		{
			name:         "renewed after recent late is regression",
			current:      valueobjects.StatusPastDue,
			trigger:      services.TriggerSubscriptionRenewed,
			occurredAt:   now.Add(-2 * time.Hour),
			lastEventAt:  now,
			wantDecision: true,
		},
		{
			name:         "purchase after expiration older than current is regression",
			current:      valueobjects.StatusExpired,
			trigger:      services.TriggerSaleApproved,
			occurredAt:   now.Add(-2 * time.Hour),
			lastEventAt:  now,
			wantDecision: true,
		},
		{
			name:         "late older than current past due is not regression because status is unchanged",
			current:      valueobjects.StatusPastDue,
			trigger:      services.TriggerSubscriptionLate,
			occurredAt:   now.Add(-2 * time.Hour),
			lastEventAt:  now,
			wantDecision: false,
		},
		{
			name:         "newer event is never regression",
			current:      valueobjects.StatusPastDue,
			trigger:      services.TriggerSubscriptionRenewed,
			occurredAt:   now.Add(2 * time.Hour),
			lastEventAt:  now,
			wantDecision: false,
		},
		{
			name:         "refund always wins regardless of timestamp",
			current:      valueobjects.StatusActive,
			trigger:      services.TriggerRefunded,
			occurredAt:   now.Add(-2 * time.Hour),
			lastEventAt:  now,
			wantDecision: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(
				t,
				tt.wantDecision,
				transitionService.IsRegression(tt.current, tt.trigger, tt.occurredAt, tt.lastEventAt),
			)
		})
	}
}
