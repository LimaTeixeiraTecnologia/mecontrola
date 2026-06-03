package outbox

import (
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/events"
)

// Registry é o contrato imutável após bootstrap: mapa de event_type para uma ou
// mais Subscriptions populado uma única vez no cmd/worker. Após o bootstrap, apenas
// leituras são permitidas.
//
// Garantias:
//   - Register valida unicidade do par (Name, EventType) e retorna ErrDuplicateSubscription
//     se duplicado (D-09).
//   - SubscriptionsFor retorna cópia defensiva do slice interno para impedir mutação externa.
//   - Validate é chamado por Subsystem.Start e falha explicitamente se qualquer Handler for nil.
//
// A implementação concreta é staticRegistry; use NewRegistry para criá-la.
type Registry interface {
	// Register adiciona uma Subscription ao registry. Retorna ErrDuplicateSubscription
	// se o par (Name, EventType) já estiver registrado.
	Register(s Subscription) error

	// SubscriptionsFor retorna todas as Subscriptions registradas para o EventType dado.
	// Retorna nil se não houver nenhuma. O slice retornado é uma cópia defensiva —
	// mutações nele não afetam o estado interno do Registry.
	SubscriptionsFor(eventType events.EventName) []Subscription

	// Validate verifica o estado global do registry, garantindo que:
	//   (a) todo Handler é não-nil;
	//   (b) todo Name passa pelo formato ^[a-z][a-z0-9_-]{2,63}$;
	//   (c) todo EventType possui ao menos um módulo separado por ".".
	// Deve ser chamado uma única vez, durante Subsystem.Start, antes de iniciar o Dispatcher.
	Validate() error
}

// registryKey é a chave de unicidade do Registry: par (Name, EventType).
type registryKey struct {
	name      string
	eventType string
}

// staticRegistry é a implementação concreta de Registry.
// Não é thread-safe para escrita (Register deve ser chamado apenas no bootstrap,
// antes de qualquer goroutine de leitura ser iniciada).
type staticRegistry struct {
	// subs mapeia EventName para o slice de Subscriptions correspondentes.
	subs map[string][]Subscription
	// seen rastreia os pares (Name, EventType) já registrados para detecção de duplicidade.
	seen map[registryKey]struct{}
}

// NewRegistry cria um Registry estático vazio, pronto para receber Subscriptions via Register.
func NewRegistry() Registry {
	return &staticRegistry{
		subs: make(map[string][]Subscription),
		seen: make(map[registryKey]struct{}),
	}
}

// Register adiciona a Subscription s ao Registry.
// Retorna ErrDuplicateSubscription se o par (s.Name, s.EventType) já foi registrado (D-09).
// Mesmo Name com EventType diferente é permitido.
func (r *staticRegistry) Register(s Subscription) error {
	key := registryKey{
		name:      s.Name.String(),
		eventType: s.EventType.String(),
	}

	if _, exists := r.seen[key]; exists {
		return fmt.Errorf("outbox: register %q / %q: %w",
			s.Name.String(), s.EventType.String(), ErrDuplicateSubscription)
	}

	r.seen[key] = struct{}{}
	r.subs[s.EventType.String()] = append(r.subs[s.EventType.String()], s)

	return nil
}

// SubscriptionsFor retorna todas as Subscriptions registradas para eventType.
// Retorna nil se não houver nenhuma registrada para o tipo dado.
// O slice retornado é uma cópia defensiva: mutações externas não afetam o estado interno.
func (r *staticRegistry) SubscriptionsFor(eventType events.EventName) []Subscription {
	internal := r.subs[eventType.String()]
	if len(internal) == 0 {
		return nil
	}

	// Cópia defensiva: garante que o chamador não corrompa o slice interno.
	copied := make([]Subscription, len(internal))
	copy(copied, internal)

	return copied
}

// Validate percorre todas as Subscriptions registradas e verifica que nenhum Handler é nil.
//
// SubscriptionName e EventType já são validados pelos seus respectivos construtores
// (NewSubscriptionName, events.NewEventName) antes de chegarem ao Register, portanto
// Validate concentra-se na única invariante que pode ser violada em nível de struct:
// um Handler nil.
//
// Deve ser chamado uma única vez, durante Subsystem.Start, antes de iniciar o Dispatcher.
// Retorna um erro descritivo na primeira violação encontrada.
func (r *staticRegistry) Validate() error {
	for _, subs := range r.subs {
		for _, s := range subs {
			if s.Handler == nil {
				return fmt.Errorf("outbox: validate: subscription %q / %q possui Handler nil",
					s.Name.String(), s.EventType.String())
			}
		}
	}

	return nil
}
