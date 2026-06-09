package payload

type webhookPayload struct {
	Object string  `json:"object"`
	Entry  []entry `json:"entry"`
}

type entry struct {
	ID      string   `json:"id"`
	Changes []change `json:"changes"`
}

type change struct {
	Field string      `json:"field"`
	Value changeValue `json:"value"`
}

type changeValue struct {
	MessagingProduct string    `json:"messaging_product"`
	Metadata         metadata  `json:"metadata"`
	Messages         []message `json:"messages"`
}

type metadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

type message struct {
	From      string    `json:"from"`
	ID        string    `json:"id"`
	Timestamp string    `json:"timestamp"`
	Type      string    `json:"type"`
	Text      *textBody `json:"text,omitempty"`
}

type textBody struct {
	Body string `json:"body"`
}

type Message struct {
	From      string
	WAMID     string
	Timestamp string
	Text      string
}
