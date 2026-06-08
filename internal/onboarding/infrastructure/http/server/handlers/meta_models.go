package handlers

type metaWebhookPayload struct {
	Object string      `json:"object"`
	Entry  []metaEntry `json:"entry"`
}

type metaEntry struct {
	ID      string       `json:"id"`
	Changes []metaChange `json:"changes"`
}

type metaChange struct {
	Field string          `json:"field"`
	Value metaChangeValue `json:"value"`
}

type metaChangeValue struct {
	MessagingProduct string        `json:"messaging_product"`
	Metadata         metaMetadata  `json:"metadata"`
	Messages         []metaMessage `json:"messages"`
}

type metaMetadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

type metaMessage struct {
	From      string        `json:"from"`
	ID        string        `json:"id"`
	Timestamp string        `json:"timestamp"`
	Type      string        `json:"type"`
	Text      *metaTextBody `json:"text,omitempty"`
}

type metaTextBody struct {
	Body string `json:"body"`
}

type parsedInboundMessage struct {
	From      string
	WAMID     string
	Timestamp string
	Text      string
}

func extractFirstMessage(payload metaWebhookPayload) (parsedInboundMessage, bool) {
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			if len(change.Value.Messages) == 0 {
				continue
			}
			msg := change.Value.Messages[0]
			text := ""
			if msg.Text != nil {
				text = msg.Text.Body
			}
			return parsedInboundMessage{
				From:      "+" + msg.From,
				WAMID:     msg.ID,
				Timestamp: msg.Timestamp,
				Text:      text,
			}, true
		}
	}
	return parsedInboundMessage{}, false
}
