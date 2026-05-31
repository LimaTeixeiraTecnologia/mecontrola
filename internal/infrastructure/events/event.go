// Package events fornece o eventbus in-process tipado via generics para o mecontrola.
// Referência: ADR-003 — Eventbus tipado via generics + emissão pós-UoW.Commit.
package events

import (
	"fmt"
	"strings"
	"time"
)

// Event é a interface base que todo evento de domínio deve implementar.
// Seguindo ADR-003: publicação atômica pós-UoW.Commit.
type Event interface {
	// Name retorna o identificador do evento em formato kebab-case <modulo>.<acao>.
	Name() EventName
	// OccurredAt retorna o instante de ocorrência do evento.
	OccurredAt() time.Time
	// AggregateID retorna o identificador do agregado que originou o evento.
	AggregateID() string
}

// EventID é o identificador único de um evento gerado com base no clock injetado.
// Usa formato ULID para ordenação lexicográfica por tempo.
type EventID struct {
	value string
}

// NewEventID cria um EventID a partir de uma string não vazia.
// Para geração automática, use GenerateEventID.
func NewEventID(value string) (EventID, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return EventID{}, fmt.Errorf("event id: valor não pode ser vazio")
	}
	return EventID{value: value}, nil
}

// String retorna a representação textual do EventID.
func (e EventID) String() string {
	return e.value
}

// EventName é o identificador do evento em formato kebab-case: <modulo>.<acao>.
// Exemplo: "identity.user-created", "conversation.message-received".
type EventName struct {
	value string
}

// NewEventName cria um EventName a partir de uma string no formato <modulo>.<acao>.
// O valor deve ser não vazio e seguir o padrão kebab-case.
func NewEventName(value string) (EventName, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return EventName{}, fmt.Errorf("event name: valor não pode ser vazio")
	}
	if !strings.Contains(value, ".") {
		return EventName{}, fmt.Errorf("event name: deve seguir o formato <modulo>.<acao>, recebido %q", value)
	}
	return EventName{value: value}, nil
}

// String retorna a representação textual do EventName.
func (n EventName) String() string {
	return n.value
}

// ModuleName representa o nome de um dos módulos de domínio do mecontrola.
// Implementado como enum para garantir que somente módulos válidos sejam referenciados.
type ModuleName struct {
	value string
}

var (
	// ModuleIdentity representa o módulo de identidade.
	ModuleIdentity = ModuleName{value: "identity"}
	// ModuleConversation representa o módulo de conversação.
	ModuleConversation = ModuleName{value: "conversation"}
	// ModuleAgent representa o módulo de agente.
	ModuleAgent = ModuleName{value: "agent"}
	// ModuleFinance representa o módulo financeiro.
	ModuleFinance = ModuleName{value: "finance"}
	// ModuleNotifications representa o módulo de notificações.
	ModuleNotifications = ModuleName{value: "notifications"}
	// ModuleTelemetry representa o módulo de telemetria.
	ModuleTelemetry = ModuleName{value: "telemetry"}
)

// _validModules lista todos os módulos de domínio válidos.
var _validModules = []ModuleName{
	ModuleIdentity,
	ModuleConversation,
	ModuleAgent,
	ModuleFinance,
	ModuleNotifications,
	ModuleTelemetry,
}

// NewModuleName cria um ModuleName validando que o valor pertence ao conjunto dos 6 módulos.
func NewModuleName(value string) (ModuleName, error) {
	value = strings.TrimSpace(value)
	for _, m := range _validModules {
		if m.value == value {
			return m, nil
		}
	}
	return ModuleName{}, fmt.Errorf("module name: %q não é um módulo válido", value)
}

// String retorna a representação textual do ModuleName.
func (m ModuleName) String() string {
	return m.value
}
