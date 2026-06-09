package payload

import (
	"encoding/json"
	"fmt"
)

func ExtractFirstMessage(raw []byte) (Message, bool, error) {
	var p webhookPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return Message{}, false, fmt.Errorf("whatsapp.payload: unmarshal: %w", err)
	}
	for _, e := range p.Entry {
		for _, c := range e.Changes {
			if len(c.Value.Messages) == 0 {
				continue
			}
			msg := c.Value.Messages[0]
			text := ""
			if msg.Text != nil {
				text = msg.Text.Body
			}
			return Message{
				From:      "+" + msg.From,
				WAMID:     msg.ID,
				Timestamp: msg.Timestamp,
				Text:      text,
			}, true, nil
		}
	}
	return Message{}, false, nil
}

func MaskMobile(mobile string) string {
	if len(mobile) < 4 {
		return "****"
	}
	return mobile[:3] + "****" + mobile[len(mobile)-4:]
}
