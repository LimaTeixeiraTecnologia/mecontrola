package outbox

import "context"

// Handler é a função que processa um evento entregue pelo Dispatcher.
//
// # Idempotência obrigatória
//
// Todo Handler DEVE ser idempotente por event.ID: executar o mesmo evento duas vezes
// com o mesmo ID DEVE produzir o mesmo estado final sem duplicação de side-effects.
// O Dispatcher garante at-least-once — não exactly-once. Use evt.ID() como chave
// de idempotência, tipicamente via:
//   - upsert com ON CONFLICT DO NOTHING por event_id
//   - tabela de deduplicação com INSERT ... ON CONFLICT IGNORE
//   - verificação prévia de estado aplicado (no-op se já processado)
//
// # Classificação de erros
//
// Retorne nil em sucesso.
// Retorne um erro transitório (qualquer erro não-permanente) para retry automático.
// Retorne fmt.Errorf("motivo: %w", outbox.ErrPermanent) para falha terminal
// — a delivery é enviada imediatamente para DLQ sem consumir tentativas.
type Handler func(ctx context.Context, evt Event) error
