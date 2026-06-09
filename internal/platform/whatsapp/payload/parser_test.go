package payload_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/payload"
)

type ParserSuite struct {
	suite.Suite
}

func TestParserSuite(t *testing.T) {
	suite.Run(t, new(ParserSuite))
}

func (s *ParserSuite) buildPayloadJSON(from, wamid, text string) []byte {
	body := map[string]any{
		"object": "whatsapp_business_account",
		"entry": []map[string]any{
			{
				"id": "entry-1",
				"changes": []map[string]any{
					{
						"field": "messages",
						"value": map[string]any{
							"messaging_product": "whatsapp",
							"messages": []map[string]any{
								{
									"from":      from,
									"id":        wamid,
									"timestamp": "1686000000",
									"type":      "text",
									"text":      map[string]any{"body": text},
								},
							},
						},
					},
				},
			},
		},
	}
	b, err := json.Marshal(body)
	s.Require().NoError(err)
	return b
}

func (s *ParserSuite) TestExtractFirstMessage_ValidPayload() {
	raw := s.buildPayloadJSON("5511999999999", "wamid-001", "ATIVAR abc123")

	msg, ok, err := payload.ExtractFirstMessage(raw)

	s.Require().NoError(err)
	s.True(ok)
	s.Equal("+5511999999999", msg.From)
	s.Equal("wamid-001", msg.WAMID)
	s.Equal("ATIVAR abc123", msg.Text)
	s.Equal("1686000000", msg.Timestamp)
}

func (s *ParserSuite) TestExtractFirstMessage_EmptyPayload() {
	raw := []byte(`{"object":"whatsapp_business_account","entry":[]}`)

	msg, ok, err := payload.ExtractFirstMessage(raw)

	s.Require().NoError(err)
	s.False(ok)
	s.Empty(msg.WAMID)
}

func (s *ParserSuite) TestExtractFirstMessage_NoMessages() {
	raw := []byte(`{"object":"whatsapp_business_account","entry":[{"id":"e1","changes":[{"field":"messages","value":{"messaging_product":"whatsapp","messages":[]}}]}]}`)

	msg, ok, err := payload.ExtractFirstMessage(raw)

	s.Require().NoError(err)
	s.False(ok)
	s.Empty(msg.WAMID)
}

func (s *ParserSuite) TestExtractFirstMessage_InvalidJSON() {
	_, _, err := payload.ExtractFirstMessage([]byte("not-json"))
	s.Error(err)
}

func (s *ParserSuite) TestExtractFirstMessage_NilTextBody() {
	// message without text field
	raw := []byte(`{"object":"whatsapp_business_account","entry":[{"id":"e1","changes":[{"field":"messages","value":{"messaging_product":"whatsapp","messages":[{"from":"5511999999999","id":"wamid-002","timestamp":"1686000000","type":"image"}]}}]}]}`)

	msg, ok, err := payload.ExtractFirstMessage(raw)

	s.Require().NoError(err)
	s.True(ok)
	s.Equal("+5511999999999", msg.From)
	s.Equal("wamid-002", msg.WAMID)
	s.Empty(msg.Text)
}

func (s *ParserSuite) TestMaskMobile() {
	scenarios := []struct {
		input    string
		expected string
	}{
		{"+5511999999999", "+55****9999"},
		{"12", "****"},
		{"abc", "****"},
		{"+5511", "+55****5511"},
	}

	for _, sc := range scenarios {
		s.Run(sc.input, func() {
			s.Equal(sc.expected, payload.MaskMobile(sc.input))
		})
	}
}
