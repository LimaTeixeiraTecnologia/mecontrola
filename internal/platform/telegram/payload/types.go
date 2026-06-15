package payload

type update struct {
	UpdateID int64    `json:"update_id"`
	Message  *message `json:"message,omitempty"`
}

type message struct {
	MessageID int64  `json:"message_id"`
	From      *user  `json:"from,omitempty"`
	Chat      chat   `json:"chat"`
	Date      int64  `json:"date"`
	Text      string `json:"text"`
}

type user struct {
	ID           int64  `json:"id"`
	IsBot        bool   `json:"is_bot"`
	LanguageCode string `json:"language_code,omitempty"`
}

type chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

type RejectionKind int

const (
	RejectAccepted RejectionKind = iota + 1
	RejectInvalidJSON
	RejectNoMessage
	RejectMissingFrom
	RejectBotSender
	RejectMissingText
	RejectNonPrivateChat
	RejectMissingDate
)

func (k RejectionKind) String() string {
	switch k {
	case RejectAccepted:
		return "accepted"
	case RejectInvalidJSON:
		return "invalid_json"
	case RejectNoMessage:
		return "no_message"
	case RejectMissingFrom:
		return "missing_from"
	case RejectBotSender:
		return "bot_sender"
	case RejectMissingText:
		return "missing_text"
	case RejectNonPrivateChat:
		return "non_private_chat"
	case RejectMissingDate:
		return "missing_date"
	default:
		return "invalid"
	}
}

type Message struct {
	UpdateID   int64
	MessageID  int64
	FromUserID int64
	ChatID     int64
	Text       string
	UnixDate   int64
}

func (m Message) ExternalID() string {
	return formatInt64(m.FromUserID)
}

func (m Message) DedupKey() string {
	return formatInt64(m.UpdateID)
}

func MaskUserID(id int64) string {
	raw := formatInt64(id)
	if len(raw) <= 4 {
		return "****"
	}
	return "***" + raw[len(raw)-4:]
}
