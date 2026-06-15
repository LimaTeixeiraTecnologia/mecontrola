package payload_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/payload"
)

func TestExtractFirstMessage_Accepted(t *testing.T) {
	raw := []byte(`{
		"update_id": 12345,
		"message": {
			"message_id": 99,
			"from": {"id": 987654321, "is_bot": false, "language_code": "pt-BR"},
			"chat": {"id": 987654321, "type": "private"},
			"date": 1718294400,
			"text": "Gastei 50 no almoço"
		}
	}`)

	outcome, err := payload.ExtractFirstMessage(raw)
	require.NoError(t, err)
	assert.Equal(t, payload.RejectAccepted, outcome.Kind)
	assert.Equal(t, int64(12345), outcome.UpdateID)
	assert.Equal(t, int64(987654321), outcome.Message.FromUserID)
	assert.Equal(t, int64(987654321), outcome.Message.ChatID)
	assert.Equal(t, int64(99), outcome.Message.MessageID)
	assert.Equal(t, int64(1718294400), outcome.Message.UnixDate)
	assert.Equal(t, "Gastei 50 no almoço", outcome.Message.Text)
	assert.Equal(t, "987654321", outcome.Message.ExternalID())
	assert.Equal(t, "12345", outcome.Message.DedupKey())
}

func TestExtractFirstMessage_InvalidJSON(t *testing.T) {
	outcome, err := payload.ExtractFirstMessage([]byte("{not json"))
	require.Error(t, err)
	assert.Equal(t, payload.RejectInvalidJSON, outcome.Kind)
}

func TestExtractFirstMessage_Rejections(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want payload.RejectionKind
	}{
		{name: "no message", raw: `{"update_id": 7}`, want: payload.RejectNoMessage},
		{name: "missing from", raw: `{"update_id":7,"message":{"message_id":1,"chat":{"id":1,"type":"private"},"date":1,"text":"x"}}`, want: payload.RejectMissingFrom},
		{name: "from id zero", raw: `{"update_id":7,"message":{"message_id":1,"from":{"id":0,"is_bot":false},"chat":{"id":1,"type":"private"},"date":1,"text":"x"}}`, want: payload.RejectMissingFrom},
		{name: "bot sender", raw: `{"update_id":7,"message":{"message_id":1,"from":{"id":5,"is_bot":true},"chat":{"id":5,"type":"private"},"date":1,"text":"x"}}`, want: payload.RejectBotSender},
		{name: "missing text", raw: `{"update_id":7,"message":{"message_id":1,"from":{"id":5,"is_bot":false},"chat":{"id":5,"type":"private"},"date":1,"text":"   "}}`, want: payload.RejectMissingText},
		{name: "non-private chat", raw: `{"update_id":7,"message":{"message_id":1,"from":{"id":5,"is_bot":false},"chat":{"id":-100,"type":"supergroup"},"date":1,"text":"oi"}}`, want: payload.RejectNonPrivateChat},
		{name: "missing date", raw: `{"update_id":7,"message":{"message_id":1,"from":{"id":5,"is_bot":false},"chat":{"id":5,"type":"private"},"date":0,"text":"oi"}}`, want: payload.RejectMissingDate},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome, err := payload.ExtractFirstMessage([]byte(tc.raw))
			require.NoError(t, err)
			assert.Equal(t, tc.want, outcome.Kind, "kind string: %s", outcome.Kind.String())
		})
	}
}

func TestRejectionKind_String(t *testing.T) {
	cases := map[payload.RejectionKind]string{
		payload.RejectAccepted:       "accepted",
		payload.RejectInvalidJSON:    "invalid_json",
		payload.RejectNoMessage:      "no_message",
		payload.RejectMissingFrom:    "missing_from",
		payload.RejectBotSender:      "bot_sender",
		payload.RejectMissingText:    "missing_text",
		payload.RejectNonPrivateChat: "non_private_chat",
		payload.RejectMissingDate:    "missing_date",
		payload.RejectionKind(0):     "invalid",
		payload.RejectionKind(99):    "invalid",
	}
	for k, want := range cases {
		assert.Equal(t, want, k.String())
	}
}

func TestMaskUserID(t *testing.T) {
	assert.Equal(t, "****", payload.MaskUserID(12))
	assert.Equal(t, "***4321", payload.MaskUserID(7654321))
}
