package postgres

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type RowMapperSuite struct {
	suite.Suite

	mapper rowMapper
	now    time.Time
}

func TestRowMapperSuite(t *testing.T) {
	suite.Run(t, new(RowMapperSuite))
}

func (s *RowMapperSuite) SetupTest() {
	s.mapper = rowMapper{}
	s.now = time.Now().UTC().Truncate(time.Second)
}

func (s *RowMapperSuite) validSubscriptionRow() subscriptionRow {
	return subscriptionRow{
		ID:                     "550e8400-e29b-41d4-a716-446655440000",
		UserID:                 "550e8400-e29b-41d4-a716-446655440001",
		Provider:               "kiwify",
		ExternalSubscriptionID: "ext-sub-123",
		PlanCode:               "MONTHLY",
		Status:                 "ACTIVE",
		PeriodStart:            s.now,
		PeriodEnd:              s.now.Add(30 * 24 * time.Hour),
		GracePeriodEnd:         time.Time{},
		RefundAmountCents:      0,
		LastEventAt:            s.now,
		LastWebhookEventID:     "550e8400-e29b-41d4-a716-446655440002",
		CreatedAt:              s.now,
		UpdatedAt:              s.now,
		DeletedAt:              nil,
	}
}

func (s *RowMapperSuite) TestHydrateSubscription() {
	scenarios := []struct {
		name    string
		row     subscriptionRow
		wantErr bool
		check   func(row subscriptionRow)
	}{
		{
			name:    "row válida retorna Subscription sem perda de campos",
			row:     s.validSubscriptionRow(),
			wantErr: false,
			check: func(row subscriptionRow) {
				sub, err := s.mapper.hydrateSubscription(row)
				s.Require().NoError(err)
				s.Require().NotNil(sub)
				s.Equal(row.ID, sub.ID().String())
				s.Equal(row.UserID, sub.UserID().String())
				s.Equal(row.Provider, sub.Provider())
				s.Equal(row.PlanCode, sub.PlanCode().String())
				s.Equal(row.Status, sub.InternalStatus().String())
				s.Equal(row.PeriodEnd.UTC(), sub.PeriodEnd().UTC())
				s.Equal(row.LastEventAt.UTC(), sub.LastEventAt().UTC())
			},
		},
		{
			name: "status INVALID retorna erro antes de chegar a RehydrateSubscription",
			row: func() subscriptionRow {
				r := s.validSubscriptionRow()
				r.Status = "INVALID"
				return r
			}(),
			wantErr: true,
			check: func(row subscriptionRow) {
				sub, err := s.mapper.hydrateSubscription(row)
				s.Error(err)
				s.Nil(sub)
				s.Contains(err.Error(), "INVALID")
			},
		},
		{
			name: "plan_code inválido retorna erro",
			row: func() subscriptionRow {
				r := s.validSubscriptionRow()
				r.PlanCode = "UNKNOWN_PLAN"
				return r
			}(),
			wantErr: true,
			check: func(row subscriptionRow) {
				sub, err := s.mapper.hydrateSubscription(row)
				s.Error(err)
				s.Nil(sub)
			},
		},
		{
			name: "subscription id vazio retorna erro",
			row: func() subscriptionRow {
				r := s.validSubscriptionRow()
				r.ID = ""
				return r
			}(),
			wantErr: true,
			check: func(row subscriptionRow) {
				sub, err := s.mapper.hydrateSubscription(row)
				s.Error(err)
				s.Nil(sub)
			},
		},
		{
			name: "status QUARTERLY retorna Subscription válida",
			row: func() subscriptionRow {
				r := s.validSubscriptionRow()
				r.PlanCode = "QUARTERLY"
				r.PeriodEnd = s.now.Add(90 * 24 * time.Hour)
				return r
			}(),
			wantErr: false,
			check: func(row subscriptionRow) {
				sub, err := s.mapper.hydrateSubscription(row)
				s.Require().NoError(err)
				s.Equal(valueobjects.PlanCodeQuarterly, sub.PlanCode())
			},
		},
		{
			name: "status TRIALING retorna Subscription válida",
			row: func() subscriptionRow {
				r := s.validSubscriptionRow()
				r.Status = "TRIALING"
				return r
			}(),
			wantErr: false,
			check: func(row subscriptionRow) {
				sub, err := s.mapper.hydrateSubscription(row)
				s.Require().NoError(err)
				s.Equal(valueobjects.SubscriptionStatusTrialing, sub.InternalStatus())
			},
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			sc.check(sc.row)
		})
	}
}

func (s *RowMapperSuite) TestHydrateWebhookEvent() {
	scenarios := []struct {
		name    string
		row     webhookEventRow
		wantErr bool
	}{
		{
			name: "row válida retorna WebhookEvent",
			row: webhookEventRow{
				ID:              "550e8400-e29b-41d4-a716-446655440002",
				Provider:        "kiwify",
				ExternalEventID: "ext-event-123",
				EventType:       "compra_aprovada",
				Signature:       "tok123",
				HeadersJSON:     []byte(`{"X-Kiwify-Token":"tok123"}`),
				Payload:         []byte(`{"id":"ext-event-123"}`),
				ReceivedAt:      s.now,
			},
			wantErr: false,
		},
		{
			name: "webhook event id inválido retorna erro",
			row: webhookEventRow{
				ID:              "not-a-uuid",
				Provider:        "kiwify",
				ExternalEventID: "ext-event-123",
				EventType:       "compra_aprovada",
				Payload:         []byte(`{"id":"ext-event-123"}`),
				ReceivedAt:      s.now,
			},
			wantErr: true,
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			event, err := s.mapper.hydrateWebhookEvent(sc.row)
			if sc.wantErr {
				s.Error(err)
				s.Zero(event.ID().String())
			} else {
				s.NoError(err)
				s.Equal(sc.row.ID, event.ID().String())
				s.Equal(sc.row.Provider, event.Provider())
			}
		})
	}
}

func (s *RowMapperSuite) TestParseSubscriptionStatus() {
	scenarios := []struct {
		name    string
		input   string
		want    valueobjects.SubscriptionStatus
		wantErr bool
	}{
		{"TRIALING", "TRIALING", valueobjects.SubscriptionStatusTrialing, false},
		{"ACTIVE", "ACTIVE", valueobjects.SubscriptionStatusActive, false},
		{"PAST_DUE", "PAST_DUE", valueobjects.SubscriptionStatusPastDue, false},
		{"CANCELED_PENDING", "CANCELED_PENDING", valueobjects.SubscriptionStatusCanceledPending, false},
		{"EXPIRED", "EXPIRED", valueobjects.SubscriptionStatusExpired, false},
		{"REFUNDED", "REFUNDED", valueobjects.SubscriptionStatusRefunded, false},
		{"desconhecido retorna erro", "UNKNOWN", valueobjects.SubscriptionStatusUnknown, true},
		{"string vazia retorna erro", "", valueobjects.SubscriptionStatusUnknown, true},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			got, err := s.mapper.parseSubscriptionStatus(sc.input)
			if sc.wantErr {
				s.Error(err)
			} else {
				s.NoError(err)
				s.Equal(sc.want, got)
			}
		})
	}
}
