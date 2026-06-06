package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	billingvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	identitydomain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain"
)

func TestStatus_StringAndHelpers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		status            billingvo.Status
		wantName          string
		wantTerminal      bool
		wantActiveBilling bool
	}{
		{
			name:              "trialing reserved",
			status:            billingvo.StatusTrialing,
			wantName:          "TRIALING",
			wantTerminal:      false,
			wantActiveBilling: false,
		},
		{
			name:              "active",
			status:            billingvo.StatusActive,
			wantName:          "ACTIVE",
			wantTerminal:      false,
			wantActiveBilling: true,
		},
		{
			name:              "past due",
			status:            billingvo.StatusPastDue,
			wantName:          "PAST_DUE",
			wantTerminal:      false,
			wantActiveBilling: true,
		},
		{
			name:              "canceled pending",
			status:            billingvo.StatusCanceledPending,
			wantName:          "CANCELED_PENDING",
			wantTerminal:      false,
			wantActiveBilling: true,
		},
		{
			name:              "expired",
			status:            billingvo.StatusExpired,
			wantName:          "EXPIRED",
			wantTerminal:      true,
			wantActiveBilling: false,
		},
		{
			name:              "refunded",
			status:            billingvo.StatusRefunded,
			wantName:          "REFUNDED",
			wantTerminal:      true,
			wantActiveBilling: false,
		},
		{
			name:              "zero value reserved",
			status:            0,
			wantName:          "",
			wantTerminal:      false,
			wantActiveBilling: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.wantName, tt.status.String())
			assert.Equal(t, tt.wantTerminal, tt.status.IsTerminal())
			assert.Equal(t, tt.wantActiveBilling, tt.status.IsActiveForBilling())
		})
	}
}

func TestStatus_MatchesIdentitySubscriptionStatus(t *testing.T) {
	t.Parallel()

	billingStatuses := []billingvo.Status{
		billingvo.StatusTrialing,
		billingvo.StatusActive,
		billingvo.StatusPastDue,
		billingvo.StatusCanceledPending,
		billingvo.StatusExpired,
		billingvo.StatusRefunded,
	}
	identityStatuses := []identitydomain.SubscriptionStatus{
		identitydomain.SubscriptionTrialing,
		identitydomain.SubscriptionActive,
		identitydomain.SubscriptionPastDue,
		identitydomain.SubscriptionCanceledPending,
		identitydomain.SubscriptionExpired,
		identitydomain.SubscriptionRefunded,
	}

	if assert.Len(t, billingStatuses, len(identityStatuses)) {
		for idx := range billingStatuses {
			assert.Equal(t, string(identityStatuses[idx]), billingStatuses[idx].String())
		}
	}
}
