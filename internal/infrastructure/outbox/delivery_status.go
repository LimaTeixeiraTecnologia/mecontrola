package outbox

// DeliveryStatus é o value object que implementa o State Pattern para o ciclo de vida
// de uma delivery (R-DDD-001). Transições válidas são governadas por CanTransitionTo.
//
// Grafo de transições:
//
//	pending → claimed
//	claimed → processed | pending (reaper) | dead_letter
//	processed → (terminal)
//	dead_letter → pending (re-enfileiramento manual)
type DeliveryStatus struct {
	value string
}

var (
	// StatusPending indica que a delivery aguarda ser processada.
	StatusPending = DeliveryStatus{"pending"}
	// StatusClaimed indica que um worker fez claim e está processando.
	StatusClaimed = DeliveryStatus{"claimed"}
	// StatusProcessed indica que o handler executou com sucesso. Terminal.
	StatusProcessed = DeliveryStatus{"processed"}
	// StatusDeadLetter indica que a delivery esgotou tentativas ou recebeu erro permanente.
	// Pode ser re-enfileirada manualmente para pending via runbook.
	StatusDeadLetter = DeliveryStatus{"dead_letter"}
)

// String retorna o valor textual do status, compatível com a coluna SQL.
func (s DeliveryStatus) String() string { return s.value }

// CanTransitionTo retorna true quando a transição de s para next é válida segundo
// o State Pattern documentado no grafo acima.
func (s DeliveryStatus) CanTransitionTo(next DeliveryStatus) bool {
	switch s {
	case StatusPending:
		return next == StatusClaimed
	case StatusClaimed:
		return next == StatusProcessed || next == StatusPending || next == StatusDeadLetter
	case StatusProcessed:
		return false
	case StatusDeadLetter:
		return next == StatusPending
	}
	return false
}
