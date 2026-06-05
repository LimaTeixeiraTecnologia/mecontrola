package outbox

import (
	"encoding/json"
	"fmt"
	"time"
)

type Envelope struct {
	ID         string            `json:"id"`
	EventType  string            `json:"event_type"`
	OccurredAt time.Time         `json:"occurred_at"`
	Metadata   map[string]string `json:"metadata"`
	Payload    json.RawMessage   `json:"payload"`
}

func Pack(row Row) Envelope {
	return Envelope{
		ID:         row.ID,
		EventType:  row.Type,
		OccurredAt: row.OccurredAt,
		Metadata:   row.Metadata,
		Payload:    json.RawMessage(row.Payload),
	}
}

func Unpack(e Envelope) ([]byte, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("outbox: pack envelope: %w", err)
	}
	return b, nil
}
