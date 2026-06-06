package input

import "encoding/json"

type ProjectSubscriptionEvent struct {
	EventType string
	Payload   json.RawMessage
}
