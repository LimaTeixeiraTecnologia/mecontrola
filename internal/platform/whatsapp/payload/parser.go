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
				text := ""
				if msg.Text != nil {
					text = msg.Text.Body
				}
				msgs = append(msgs, Message{
					From:      "+" + msg.From,
					WAMID:     msg.ID,
					Timestamp: msg.Timestamp,
					Text:      text,
				})
			}
		}
	}
	return msgs, nil
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
