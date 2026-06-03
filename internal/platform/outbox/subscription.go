package outbox

import (
	"fmt"
	"regexp"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
)

// _subscriptionNameRegex valida o formato de SubscriptionName:
// começa com letra minúscula, seguida de 2 a 63 caracteres [a-z0-9_-].
var _subscriptionNameRegex = regexp.MustCompile(`^[a-z][a-z0-9_-]{2,63}$`)

// SubscriptionName é o value object que encapsula o nome de uma subscription.
// Formato obrigatório: ^[a-z][a-z0-9_-]{2,63}$.
type SubscriptionName struct {
	value string
}

// NewSubscriptionName cria um SubscriptionName validando o formato.
func NewSubscriptionName(value string) (SubscriptionName, error) {
	if !_subscriptionNameRegex.MatchString(value) {
		return SubscriptionName{}, fmt.Errorf("outbox: subscription name %q nao segue o formato ^[a-z][a-z0-9_-]{2,63}$", value)
	}
	return SubscriptionName{value: value}, nil
}

// String retorna a representação textual do SubscriptionName.
func (s SubscriptionName) String() string { return s.value }

// Subscription representa o mapeamento estático entre um event_type e um Handler.
// Resolvida em build time no bootstrap do cmd/worker.
type Subscription struct {
	// Name identifica unicamente a subscription (junto com EventType).
	Name SubscriptionName
	// EventType é o tipo de evento que esta subscription consome.
	EventType events.EventName
	// Handler é a função que processa o evento. DEVE ser idempotente por event.ID.
	Handler Handler
}
