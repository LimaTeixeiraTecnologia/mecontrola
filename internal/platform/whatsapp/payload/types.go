package payload

type MessageType int

const (
	MessageTypeText MessageType = iota + 1
	MessageTypeAudio
)

func (t MessageType) String() string {
	switch t {
	case MessageTypeText:
		return "text"
	case MessageTypeAudio:
		return "audio"
	default:
		return "unknown"
	}
}

func (t MessageType) IsValid() bool {
	return t == MessageTypeText || t == MessageTypeAudio
}

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
	From      string     `json:"from"`
	ID        string     `json:"id"`
	Timestamp string     `json:"timestamp"`
	Type      string     `json:"type"`
	Text      *textBody  `json:"text,omitempty"`
	Audio     *audioBody `json:"audio,omitempty"`
}

type textBody struct {
	Body string `json:"body"`
}

type audioBody struct {
	ID       string `json:"id"`
	MimeType string `json:"mime_type"`
	SHA256   string `json:"sha256"`
	Voice    bool   `json:"voice"`
}

type Message struct {
	From      string
	WAMID     string
	Timestamp string
	Type      MessageType
	Text      string
	Audio     *Audio
}

type Audio struct {
	MediaID  string
	MimeType string
	SHA256   string
	Voice    bool
}
