// Package outbox implementa o Transactional Outbox para o mecontrola.
//
// # Idempotência obrigatória por event_id
//
// Todo [Handler] registrado DEVE ser idempotente por event.ID: executar o mesmo evento
// duas vezes com o mesmo event_id DEVE produzir o mesmo estado final sem duplicação de
// side-effects (cobranças, notificações, projeções). O dispatcher garante at-least-once —
// não exactly-once. O handler é responsável por garantir idempotência, tipicamente via
// upsert por event_id ou tabela de deduplicação.
//
// # Quando usar outbox.Publisher vs events.Bus
//
// Use [Publisher] (este pacote) para side-effects críticos que precisam ser entregues
// mesmo após crash, deploy ou reinício do worker: notificações, projeções persistentes,
// integrações externas disparadas pós-commit.
//
// Use events.Bus (ADR-003) para sinais voláteis in-process que podem ser perdidos sem
// impacto ao produto: telemetria em tempo real, propagação de cache local, triggers de UI.
// O Bus descarta mensagens quando o canal do subscriber está cheio — comportamento intencional
// para sinais que não justificam persistência.
//
// Ambos coexistem; um não substitui o outro. Documente explicitamente no godoc do seu handler
// qual dos dois foi escolhido e por quê (ver RF-38 / ADR-016).
package outbox
