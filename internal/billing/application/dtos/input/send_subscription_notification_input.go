package input

import "encoding/json"

type SendSubscriptionNotificationInput struct {
	EventType string
	Payload   json.RawMessage
}
