package meta

type sendMessageRequest struct {
	MessagingProduct string           `json:"messaging_product"`
	To               string           `json:"to"`
	Type             string           `json:"type"`
	Template         *templatePayload `json:"template,omitempty"`
	Text             *textPayload     `json:"text,omitempty"`
}

type templatePayload struct {
	Name       string           `json:"name"`
	Language   templateLanguage `json:"language"`
	Components []any            `json:"components,omitempty"`
}

type templateLanguage struct {
	Code string `json:"code"`
}

type textPayload struct {
	Body string `json:"body"`
}

type sendMessageResponse struct {
	Messages []messageIDEntry `json:"messages"`
}

type messageIDEntry struct {
	ID string `json:"id"`
}

type errorResponse struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Message   string `json:"message"`
	Type      string `json:"type"`
	Code      int    `json:"code"`
	ErrorData any    `json:"error_data,omitempty"`
}
