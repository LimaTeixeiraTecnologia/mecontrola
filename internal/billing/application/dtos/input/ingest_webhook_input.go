package input

import (
	"encoding/json"
	"strings"
	"time"
)

// IngestWebhookInput carrega o payload bruto e os headers do webhook recebido.
type IngestWebhookInput struct {
	RawBody             []byte
	Headers             map[string]string
	SignatureHeaderName string
	ReceivedAt          time.Time
}

// HeadersJSON serializa os headers filtrados (sem Authorization/Cookie) para armazenamento em JSONB.
func (i IngestWebhookInput) HeadersJSON() json.RawMessage {
	filtered := make(map[string]string, len(i.Headers))
	for k, v := range i.Headers {
		lower := strings.ToLower(k)
		if lower == "authorization" || lower == "cookie" {
			continue
		}
		filtered[k] = v
	}
	b, _ := json.Marshal(filtered)
	return b
}
