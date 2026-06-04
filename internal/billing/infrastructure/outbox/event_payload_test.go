package outbox_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	billingoutbox "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/outbox"
)

type EventPayloadSuite struct {
	suite.Suite
}

func TestEventPayload(t *testing.T) {
	suite.Run(t, new(EventPayloadSuite))
}

func (s *EventPayloadSuite) TestEncodeDecodeRoundtrip() {
	webhookID, err := valueobjects.NewWebhookEventID("550e8400-e29b-41d4-a716-446655440000")
	s.Require().NoError(err)

	raw, err := billingoutbox.EncodeReceivedPayload(webhookID, "kiwify")
	s.Require().NoError(err)
	s.NotEmpty(raw)

	decoded, err := billingoutbox.DecodeReceivedPayload(raw)
	s.Require().NoError(err)

	s.Equal(webhookID.String(), decoded.WebhookEventID)
	s.Equal("kiwify", decoded.Provider)
}

func (s *EventPayloadSuite) TestDecodeJSONInvalidoRetornaErro() {
	_, err := billingoutbox.DecodeReceivedPayload(json.RawMessage(`not-json`))
	s.Error(err)
	s.Contains(err.Error(), "outbox: decode received payload")
}

func (s *EventPayloadSuite) TestEncodePreservaWebhookEventID() {
	webhookID, err := valueobjects.NewWebhookEventID("550e8400-e29b-41d4-a716-446655440001")
	s.Require().NoError(err)

	raw, err := billingoutbox.EncodeReceivedPayload(webhookID, "kiwify")
	s.Require().NoError(err)

	var m map[string]string
	s.Require().NoError(json.Unmarshal(raw, &m))
	s.Equal(webhookID.String(), m["webhook_event_id"])
	s.Equal("kiwify", m["provider"])
}
