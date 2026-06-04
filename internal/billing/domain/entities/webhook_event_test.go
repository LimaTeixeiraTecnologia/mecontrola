package entities_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	billingdomain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type WebhookEventSuite struct {
	suite.Suite
}

func TestWebhookEventSuite(t *testing.T) {
	suite.Run(t, new(WebhookEventSuite))
}

func validWebhookParams() entities.NewWebhookEventParams {
	evtID, _ := valueobjects.NewWebhookEventID("550e8400-e29b-41d4-a716-446655440002")
	extID, _ := valueobjects.NewExternalEventIDCascade([]byte(`{"id":"ext-001"}`))
	return entities.NewWebhookEventParams{
		ID:              evtID,
		Provider:        "kiwify",
		ExternalEventID: extID,
		EventType:       "compra_aprovada",
		Signature:       "tok-abc",
		HeadersJSON:     json.RawMessage(`{}`),
		Payload:         json.RawMessage(`{"id":"ext-001"}`),
		ReceivedAt:      time.Now(),
	}
}

func (s *WebhookEventSuite) TestNewWebhookEvent_Valid() {
	evt, err := entities.NewWebhookEvent(validWebhookParams())
	s.NoError(err)
	s.Equal("kiwify", evt.Provider())
	s.Equal("compra_aprovada", evt.EventType())
	s.NotEmpty(evt.Payload())
}

func (s *WebhookEventSuite) TestNewWebhookEvent_EmptyID() {
	p := validWebhookParams()
	p.ID = valueobjects.WebhookEventID{}
	_, err := entities.NewWebhookEvent(p)
	s.True(errors.Is(err, billingdomain.ErrWebhookEventRequiresID))
}

func (s *WebhookEventSuite) TestNewWebhookEvent_EmptyProvider() {
	p := validWebhookParams()
	p.Provider = ""
	_, err := entities.NewWebhookEvent(p)
	s.True(errors.Is(err, billingdomain.ErrWebhookEventRequiresProvider))
}

func (s *WebhookEventSuite) TestNewWebhookEvent_EmptyExternalEventID() {
	p := validWebhookParams()
	p.ExternalEventID = valueobjects.ExternalEventID{}
	_, err := entities.NewWebhookEvent(p)
	s.True(errors.Is(err, billingdomain.ErrWebhookEventRequiresExternalID))
}

func (s *WebhookEventSuite) TestNewWebhookEvent_EmptyPayload() {
	p := validWebhookParams()
	p.Payload = nil
	_, err := entities.NewWebhookEvent(p)
	s.True(errors.Is(err, billingdomain.ErrWebhookEventRequiresPayload))
}

func (s *WebhookEventSuite) TestWebhookEvent_Accessors() {
	p := validWebhookParams()
	evt, err := entities.NewWebhookEvent(p)
	s.Require().NoError(err)
	s.False(evt.ID().IsZero())
	s.False(evt.ExternalEventID().IsZero())
	s.Equal("tok-abc", evt.Signature())
	s.NotNil(evt.HeadersJSON())
	s.False(evt.ReceivedAt().IsZero())
}
