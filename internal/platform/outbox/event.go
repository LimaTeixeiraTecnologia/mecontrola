package outbox

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
)

// Event é o value object imutável que representa um evento publicado via Outbox.
// Campos não exportados garantem que invariantes só possam ser violadas por NewEvent.
//
// Nota: aggregate_id é mantido como string porque sua forma é definida por cada módulo
// (identity, finance, etc.) — o Outbox apenas armazena opacamente.
type Event struct {
	id            events.EventID
	eventType     events.EventName
	version       uint16
	aggregateType string
	aggregateID   string
	partitionKey  *string
	payload       json.RawMessage
	headers       Headers
	occurredAt    time.Time
}

// NewEventParams agrupa os parâmetros de construção de Event.
// Campos obrigatórios: ID, EventType, AggregateType, AggregateID, Payload.
// Version default: 1. OccurredAt default: time.Now().UTC().
type NewEventParams struct {
	ID            events.EventID
	EventType     events.EventName
	Version       uint16
	AggregateType string
	AggregateID   string
	PartitionKey  *string
	Payload       json.RawMessage
	Headers       Headers
	OccurredAt    time.Time
}

// NewEvent cria um Event validando as invariantes obrigatórias.
// Retorna ErrInvalidEvent wrappado com contexto quando a validação falhar.
func NewEvent(p NewEventParams) (Event, error) {
	if p.ID.String() == "" {
		return Event{}, fmt.Errorf("%w: event id obrigatorio", ErrInvalidEvent)
	}
	if p.EventType.String() == "" {
		return Event{}, fmt.Errorf("%w: event type obrigatorio", ErrInvalidEvent)
	}
	if p.AggregateType == "" {
		return Event{}, fmt.Errorf("%w: aggregate type obrigatorio", ErrInvalidEvent)
	}
	if p.AggregateID == "" {
		return Event{}, fmt.Errorf("%w: aggregate id obrigatorio", ErrInvalidEvent)
	}
	if len(p.Payload) == 0 || !json.Valid(p.Payload) {
		return Event{}, fmt.Errorf("%w: payload nao e JSON valido", ErrInvalidEvent)
	}
	if p.Version == 0 {
		p.Version = 1
	}
	if p.OccurredAt.IsZero() {
		p.OccurredAt = time.Now().UTC()
	}
	headers := p.Headers
	if headers == nil {
		headers = make(Headers)
	} else {
		headers = headers.clone()
	}
	return Event{
		id:            p.ID,
		eventType:     p.EventType,
		version:       p.Version,
		aggregateType: p.AggregateType,
		aggregateID:   p.AggregateID,
		partitionKey:  p.PartitionKey,
		payload:       p.Payload,
		headers:       headers,
		occurredAt:    p.OccurredAt,
	}, nil
}

// ID retorna o identificador único do evento (ULID).
func (e Event) ID() events.EventID { return e.id }

// Type retorna o tipo do evento no formato <modulo>.<acao>.
func (e Event) Type() events.EventName { return e.eventType }

// Version retorna a versão do schema do payload.
func (e Event) Version() uint16 { return e.version }

// AggregateType retorna o tipo do agregado que originou o evento.
func (e Event) AggregateType() string { return e.aggregateType }

// AggregateID retorna o identificador do agregado que originou o evento.
func (e Event) AggregateID() string { return e.aggregateID }

// PartitionKey retorna a chave de particionamento opcional (D-10, reservada para V2).
func (e Event) PartitionKey() *string { return e.partitionKey }

// Payload retorna o payload JSON do evento. Nunca nulo após construção válida.
func (e Event) Payload() json.RawMessage { return e.payload }

// Headers retorna uma cópia dos metadados de propagação.
func (e Event) Headers() Headers { return e.headers.clone() }

// OccurredAt retorna o instante de ocorrência do evento em UTC.
func (e Event) OccurredAt() time.Time { return e.occurredAt }
