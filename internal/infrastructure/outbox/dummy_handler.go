package outbox

import (
	"context"
	"log/slog"
)

// DummyHandler é um Handler trivial usado para testes de integração e smoke tests.
// Registra o recebimento do evento via log estruturado sem processar nenhuma lógica de negócio.
// É idempotente: executar o mesmo evento múltiplas vezes não produce side-effects.
//
// Payload jamais aparece em chamada slog.* (R-SEC-001).
func DummyHandler(ctx context.Context, evt Event) error {
	slog.InfoContext(ctx, "outbox: dummy handler recebeu evento",
		slog.String("event_id", evt.ID().String()),
		slog.String("event_type", evt.Type().String()),
	)
	return nil
}
