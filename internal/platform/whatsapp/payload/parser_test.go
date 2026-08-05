package payload_test

import (
	"encoding/json"
	"testing"
	"time"

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

func (s *ParserSuite) buildMultiMessagePayload(msgs []map[string]any) []byte {
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
							"messages":          msgs,
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
	raw := []byte(`{"object":"whatsapp_business_account","entry":[{"id":"e1","changes":[{"field":"messages","value":{"messaging_product":"whatsapp","messages":[{"from":"5511999999999","id":"wamid-002","timestamp":"1686000000","type":"image"}]}}]}]}`)

	msg, ok, err := payload.ExtractFirstMessage(raw)

	s.Require().NoError(err)
	s.True(ok)
	s.Equal("+5511999999999", msg.From)
	s.Equal("wamid-002", msg.WAMID)
	s.Equal(payload.MessageTypeText, msg.Type)
	s.Empty(msg.Text)
	s.Nil(msg.Audio)
}

func (s *ParserSuite) TestExtractFirstMessage_TextMessage_TypedAsText() {
	raw := s.buildPayloadJSON("5511999999999", "wamid-001", "ATIVAR abc123")

	msg, ok, err := payload.ExtractFirstMessage(raw)

	s.Require().NoError(err)
	s.True(ok)
	s.Equal(payload.MessageTypeText, msg.Type)
	s.Equal("text", msg.Type.String())
	s.True(msg.Type.IsValid())
	s.Nil(msg.Audio)
}

func (s *ParserSuite) buildAudioPayloadJSON(from, wamid, mediaID, mimeType, sha256 string, voice bool) []byte {
	audio := map[string]any{
		"mime_type": mimeType,
		"sha256":    sha256,
		"voice":     voice,
	}
	if mediaID != "" {
		audio["id"] = mediaID
	}
	rawMsgs := []map[string]any{
		{
			"from":      from,
			"id":        wamid,
			"timestamp": "1754331555",
			"type":      "audio",
			"audio":     audio,
		},
	}
	return s.buildMultiMessagePayload(rawMsgs)
}

func (s *ParserSuite) TestExtractFirstMessage_RealSanitizedAudio_TypedAsAudio() {
	const (
		mediaID  = "1081986107489646"
		mimeType = "audio/ogg; codecs=opus"
		sha256   = "a3a064090479e03e4e84a372ecc07c500312530b0a051a57186725a5a69d165c"
	)
	raw := s.buildAudioPayloadJSON("5511999999999", "wamid-audio-001", mediaID, mimeType, sha256, true)

	msg, ok, err := payload.ExtractFirstMessage(raw)

	s.Require().NoError(err)
	s.True(ok)
	s.Equal("+5511999999999", msg.From)
	s.Equal("wamid-audio-001", msg.WAMID)
	s.Equal("1754331555", msg.Timestamp)
	s.Equal(payload.MessageTypeAudio, msg.Type)
	s.Equal("audio", msg.Type.String())
	s.True(msg.Type.IsValid())
	s.Empty(msg.Text)
	s.Require().NotNil(msg.Audio)
	s.Equal(mediaID, msg.Audio.MediaID)
	s.Equal(mimeType, msg.Audio.MimeType)
	s.Equal(sha256, msg.Audio.SHA256)
	s.True(msg.Audio.Voice)
}

func (s *ParserSuite) TestExtractMessages_AudioWithoutMediaID_Rejected() {
	raw := s.buildAudioPayloadJSON("5511999999999", "wamid-audio-002", "", "audio/ogg; codecs=opus", "a3a064090479e03e4e84a372ecc07c500312530b0a051a57186725a5a69d165c", true)

	msgs, err := payload.ExtractMessages(raw)

	s.Require().NoError(err)
	s.Empty(msgs)
}

func (s *ParserSuite) TestExtractMessages_AudioWithoutMimeType_Rejected() {
	raw := s.buildAudioPayloadJSON("5511999999999", "wamid-audio-003", "1081986107489646", "", "a3a064090479e03e4e84a372ecc07c500312530b0a051a57186725a5a69d165c", true)

	msgs, err := payload.ExtractMessages(raw)

	s.Require().NoError(err)
	s.Empty(msgs)
}

func (s *ParserSuite) TestExtractMessages_AudioWithoutSHA256_Rejected() {
	raw := s.buildAudioPayloadJSON("5511999999999", "wamid-audio-004", "1081986107489646", "audio/ogg; codecs=opus", "", true)

	msgs, err := payload.ExtractMessages(raw)

	s.Require().NoError(err)
	s.Empty(msgs)
}

func (s *ParserSuite) TestExtractFirstMessage_UnknownMessageType_TreatedAsTextFallback() {
	raw := []byte(`{"object":"whatsapp_business_account","entry":[{"id":"e1","changes":[{"field":"messages","value":{"messaging_product":"whatsapp","messages":[{"from":"5511999999999","id":"wamid-005","timestamp":"1686000000","type":"sticker"}]}}]}]}`)

	msg, ok, err := payload.ExtractFirstMessage(raw)

	s.Require().NoError(err)
	s.True(ok)
	s.Equal(payload.MessageTypeText, msg.Type)
	s.Empty(msg.Text)
	s.Nil(msg.Audio)
}

func (s *ParserSuite) TestExtractMessages_MultipleMessages_ReturnsAll() {
	rawMsgs := []map[string]any{
		{"from": "5511999999999", "id": "wamid-001", "timestamp": "1686000001", "type": "text", "text": map[string]any{"body": "msg1"}},
		{"from": "5511999999999", "id": "wamid-002", "timestamp": "1686000002", "type": "text", "text": map[string]any{"body": "msg2"}},
		{"from": "5511999999999", "id": "wamid-003", "timestamp": "1686000003", "type": "text", "text": map[string]any{"body": "msg3"}},
	}
	raw := s.buildMultiMessagePayload(rawMsgs)

	msgs, err := payload.ExtractMessages(raw)

	s.Require().NoError(err)
	s.Len(msgs, 3)
	s.Equal("wamid-001", msgs[0].WAMID)
	s.Equal("1686000001", msgs[0].Timestamp)
	s.Equal("msg1", msgs[0].Text)
	s.Equal("wamid-002", msgs[1].WAMID)
	s.Equal("1686000002", msgs[1].Timestamp)
	s.Equal("wamid-003", msgs[2].WAMID)
	s.Equal("1686000003", msgs[2].Timestamp)
}

func (s *ParserSuite) TestExtractMessages_SingleMessage_BehaviorEquivalent() {
	rawMsgs := []map[string]any{
		{"from": "5511999999999", "id": "wamid-001", "timestamp": "1686000000", "type": "text", "text": map[string]any{"body": "oi"}},
	}
	raw := s.buildMultiMessagePayload(rawMsgs)

	msgs, err := payload.ExtractMessages(raw)

	s.Require().NoError(err)
	s.Len(msgs, 1)
	s.Equal("+5511999999999", msgs[0].From)
	s.Equal("wamid-001", msgs[0].WAMID)
	s.Equal("oi", msgs[0].Text)
}

func (s *ParserSuite) TestExtractMessages_EmptyEntry_ReturnsNil() {
	raw := []byte(`{"object":"whatsapp_business_account","entry":[]}`)

	msgs, err := payload.ExtractMessages(raw)

	s.Require().NoError(err)
	s.Empty(msgs)
}

func (s *ParserSuite) TestExtractMessages_InvalidJSON_ReturnsError() {
	_, err := payload.ExtractMessages([]byte("not-json"))
	s.Error(err)
}

func (s *ParserSuite) TestExtractMessages_OrderPreserved() {
	rawMsgs := []map[string]any{
		{"from": "5511111111111", "id": "wamid-A", "timestamp": "1686000010", "type": "text", "text": map[string]any{"body": "first"}},
		{"from": "5511111111111", "id": "wamid-B", "timestamp": "1686000020", "type": "text", "text": map[string]any{"body": "second"}},
	}
	raw := s.buildMultiMessagePayload(rawMsgs)

	msgs, err := payload.ExtractMessages(raw)

	s.Require().NoError(err)
	s.Require().Len(msgs, 2)
	s.Equal("wamid-A", msgs[0].WAMID)
	s.Equal("wamid-B", msgs[1].WAMID)
	tsA, okA := payload.ParseEpochTimestamp(msgs[0].Timestamp)
	tsB, okB := payload.ParseEpochTimestamp(msgs[1].Timestamp)
	s.True(okA)
	s.True(okB)
	s.True(tsA.Before(tsB), "primeiro timestamp deve preceder o segundo")
}

func (s *ParserSuite) TestParseEpochTimestamp() {
	scenarios := []struct {
		name      string
		input     string
		expectOK  bool
		expectUTC time.Time
	}{
		{
			name:      "valid epoch",
			input:     "1686000000",
			expectOK:  true,
			expectUTC: time.Unix(1686000000, 0).UTC(),
		},
		{
			name:     "empty string",
			input:    "",
			expectOK: false,
		},
		{
			name:     "zero value",
			input:    "0",
			expectOK: false,
		},
		{
			name:     "negative value",
			input:    "-1",
			expectOK: false,
		},
		{
			name:     "non-numeric",
			input:    "not-a-number",
			expectOK: false,
		},
		{
			name:      "large valid epoch",
			input:     "9999999999",
			expectOK:  true,
			expectUTC: time.Unix(9999999999, 0).UTC(),
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			ts, ok := payload.ParseEpochTimestamp(sc.input)
			s.Equal(sc.expectOK, ok)
			if sc.expectOK {
				s.Equal(sc.expectUTC, ts)
				s.Equal(time.UTC, ts.Location())
			} else {
				s.True(ts.IsZero())
			}
		})
	}
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
