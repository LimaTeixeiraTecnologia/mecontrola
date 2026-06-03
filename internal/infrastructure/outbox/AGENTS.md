# Agentes de IA — Módulo `outbox`

<!-- RF-38 / ADR-016 -->

Consulte `doc.go` nesta pasta para o contrato completo do pacote e `docs/runbooks/outbox.md` para procedimentos operacionais.

## Papel do Módulo

`internal/infrastructure/outbox` implementa o **Transactional Outbox** do MeControla: garante entrega at-least-once de eventos após o commit transacional do agregado, mesmo diante de crash, deploy ou reinício do worker. Consulte `ADR-016` (`.specs/prd-mecontrola-foundation/adr-016-outbox-publisher-opt-in.md`) para o contexto arquitetural completo.

## Contrato Publisher / Subscription / Handler

```go
// Publicar um evento dentro de uma transação
err := publisher.Publish(ctx, tx, event)

// Registrar um handler para um tipo de evento
err := registry.Register(outbox.Subscription{
    Name:      "meu-handler",   // único por (Name, EventType)
    EventType: events.MustEventName("modulo.acao"),
    Handler:   meuHandler,
})

// Handler deve ser idempotente por event.ID
var meuHandler outbox.Handler = func(ctx context.Context, evt outbox.Event) error {
    // Usar evt.ID() como chave de deduplicação (upsert ou tabela dedup)
    return nil
}
```

## Regra Obrigatória de Idempotência

**Todo `Handler` registrado DEVE ser idempotente por `event.ID`.**

O Dispatcher garante entrega at-least-once — não exactly-once. Executar o mesmo evento duas vezes com o mesmo `event_id` DEVE produzir o mesmo estado final sem duplicação de side-effects (cobranças, notificações, projeções).

Estratégias aceitas:
- `INSERT ... ON CONFLICT (event_id) DO NOTHING` na tabela de destino.
- Tabela de deduplicação `processed_events(event_id PRIMARY KEY)`.
- Upsert por `aggregate_id` + `event_version` quando o estado final é determinístico.

## Critério: outbox.Publisher vs events.Bus

| Critério | Use `outbox.Publisher` | Use `events.Bus` (ADR-003) |
|---|---|---|
| Falha de entrega é aceitável | Não | Sim |
| Side-effect persiste após crash | Necessário | Não necessário |
| Exemplos | Notificações, projeções, integrações | Cache local, telemetria real-time, triggers de UI |
| Overhead por chamada | +1 INSERT transacional | Zero (canal em memória) |

**Ambos coexistem.** Documentar no godoc do handler qual foi escolhido e por quê.

## Comandos `ai-spec` rápidos

```bash
# Criar PRD de nova feature que usa este pacote
ai-spec create-prd

# Derivar techspec do PRD aprovado
ai-spec create-technical-specification

# Decompor em tarefas
ai-spec create-tasks

# Executar tarefa
ai-spec execute-task

# Verificar drift de spec
ai-spec check-spec-drift .specs/prd-outbox-<feature>/tasks.md
```

## Referências

- `doc.go` — godoc do pacote com contratos detalhados
- `docs/runbooks/outbox.md` — procedimentos operacionais (DLQ, re-enfileiramento, LGPD, rollout)
- `.specs/prd-mecontrola-foundation/adr-016-outbox-publisher-opt-in.md` — ADR-016
- `.specs/prd-outbox-event-driven/` — PRD, techspec e ADRs de implementação
