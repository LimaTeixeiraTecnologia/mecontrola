package payload

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

func ExtractMessages(raw []byte) ([]Message, error) {
	var p webhookPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("whatsapp.payload: unmarshal: %w", err)
	}
	var msgs []Message
	for _, e := range p.Entry {
		for _, c := range e.Changes {
			for _, msg := range c.Value.Messages {
				parsed, ok := parseMessage(msg)
				if !ok {
					continue
				}
				msgs = append(msgs, parsed)
			}
		}
	}
	return msgs, nil
}

func parseMessage(msg message) (Message, bool) {
	if msg.Type == "audio" {
		return parseAudioMessage(msg)
	}

	text := ""
	if msg.Text != nil {
		text = msg.Text.Body
	}
	return Message{
		From:      "+" + msg.From,
		WAMID:     msg.ID,
		Timestamp: msg.Timestamp,
		Type:      MessageTypeText,
		Text:      text,
	}, true
}

func parseAudioMessage(msg message) (Message, bool) {
	if msg.Audio == nil {
		return Message{}, false
	}
	if msg.Audio.ID == "" || msg.Audio.MimeType == "" || msg.Audio.SHA256 == "" {
		return Message{}, false
	}
	return Message{
		From:      "+" + msg.From,
		WAMID:     msg.ID,
		Timestamp: msg.Timestamp,
		Type:      MessageTypeAudio,
		Audio: &Audio{
			MediaID:  msg.Audio.ID,
			MimeType: msg.Audio.MimeType,
			SHA256:   msg.Audio.SHA256,
			Voice:    msg.Audio.Voice,
		},
	}, true
}

func ExtractFirstMessage(raw []byte) (Message, bool, error) {
	msgs, err := ExtractMessages(raw)
	if err != nil {
		return Message{}, false, err
	}
	if len(msgs) == 0 {
		return Message{}, false, nil
	}
	return msgs[0], true, nil
}

func ParseEpochTimestamp(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	ts, err := strconv.ParseInt(s, 10, 64)
	if err != nil || ts <= 0 {
		return time.Time{}, false
	}
	return time.Unix(ts, 0).UTC(), true
}

func MaskMobile(mobile string) string {
	if len(mobile) < 4 {
		return "****"
	}
	return mobile[:3] + "****" + mobile[len(mobile)-4:]
}
