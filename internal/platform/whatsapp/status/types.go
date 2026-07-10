package status

import "errors"

var ErrEmptyMessageID = errors.New("whatsapp.status: message_id vazio")

type MessageDeliveryState string

const (
	DeliveryStateNotReceived MessageDeliveryState = "not_received"
	DeliveryStateFailed      MessageDeliveryState = "failed"
	DeliveryStateDelivered   MessageDeliveryState = "delivered"
)

func (s MessageDeliveryState) String() string {
	return string(s)
}

func (s MessageDeliveryState) IsValid() bool {
	switch s {
	case DeliveryStateNotReceived, DeliveryStateFailed, DeliveryStateDelivered:
		return true
	default:
		return false
	}
}

func DecideDeliveryState(total, failed int) MessageDeliveryState {
	if total <= 0 {
		return DeliveryStateNotReceived
	}
	if failed > 0 {
		return DeliveryStateFailed
	}
	return DeliveryStateDelivered
}

type DeliveryCounts struct {
	Total  int
	Failed int
}

type MessageStatus struct {
	MessageID   string
	Status      string
	RecipientID string
	Timestamp   string
	ErrorCode   string
	ErrorTitle  string
}

type statusPayload struct {
	Entry []statusEntry `json:"entry"`
}

type statusEntry struct {
	ID      string         `json:"id"`
	Changes []statusChange `json:"changes"`
}

type statusChange struct {
	Field string            `json:"field"`
	Value statusChangeValue `json:"value"`
}

type statusChangeValue struct {
	MessagingProduct string         `json:"messaging_product"`
	Statuses         []statusRecord `json:"statuses"`
}

type statusRecord struct {
	ID          string        `json:"id"`
	Status      string        `json:"status"`
	Timestamp   string        `json:"timestamp"`
	RecipientID string        `json:"recipient_id"`
	Errors      []statusError `json:"errors"`
}

type statusError struct {
	Code  int    `json:"code"`
	Title string `json:"title"`
}
