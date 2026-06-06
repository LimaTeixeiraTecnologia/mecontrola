# ADR-005 — Idempotência por event_key composto + State machine determinística + last_event_at vector

## Metadados

- **Título:** Idempotência e ordering para eventos Kiwify
- **Data:** 2026-06-05
- **Status:** Aceita
- **Decisores:** PO (jailton), arquitetura (AI)
- **Relacionados:** `.specs/prd-billing-pipeline/techspec.md` §5.3, §7, RF-11, RF-12, ADR-003

## Contexto

PRD RF-11 e RF-12 exigem (a) idempotência por identificador único de evento e (b) tratamento correto de eventos fora de ordem cronológica. A Kiwify usa `envelope.id` que **muda a cada retry** (observado na Banking API e prática conhecida) — não serve como chave de dedupe. A Public API entrega 6 triggers que podem chegar duplicados, fora de ordem, ou intercalados (renewed antes do approved original).

Sem desenho explícito: cobranças duplicadas, regressões de estado, audit trail confuso.

## Decisão

Combinação de três mecanismos:

### 1. Chave de idempotência composta (`event_key`) por trigger

Persistida em `billing_processed_events(event_key TEXT PRIMARY KEY, trigger, recurso_id, occurred_at, applied_at, status)`. INSERT antes do mutation; conflito (`pgErrCode 23505`) → no-op silencioso (return 202).

| Trigger | event_key |
| --- | --- |
| `compra_aprovada` | `compra_aprovada:{sale.id}` |
| `subscription_renewed` | `subscription_renewed:{subscription.id}:{subscription.updated_at_iso8601}` |
| `subscription_late` | `subscription_late:{subscription.id}:{subscription.updated_at_iso8601}` |
| `subscription_canceled` | `subscription_canceled:{subscription.id}` |
| `compra_reembolsada` | `refund:{sale.id}` |
| `chargeback` | `refund:{sale.id}` (mesmo prefixo) |

Refund e chargeback compartilham prefixo porque o PRD os trata como o mesmo efeito de negócio (REFUNDED imediato) — receber ambos para a mesma sale é no-op idempotente.

### 2. State machine explícita

Tabela de transições permitidas em `internal/billing/domain/services/transitions.go`:

```
                ACTIVE    PAST_DUE  CANCELED_PENDING  EXPIRED   REFUNDED
ACTIVE          extend    →set      →set              (time)    →set
PAST_DUE        →set      extend    →set              (grace)   →set
CANCELED_PENDING →extend  -         -                 (period)  →set
EXPIRED         →set      -         -                 -         →set
REFUNDED        -         -         -                 -         -
```

Convenções:
- `extend` = `subscription_renewed` aplicado (estende `period_end`).
- `→set` = transição direta permitida.
- `-` = transição não permitida (registrada como `superseded`).
- `(time)/(grace)/(period)` = decididos pela função `IsEntitled` em runtime, **não** persistidos por job.

`REFUNDED` é terminal: sempre vence chegando, nunca regride.

### 3. `last_event_at` vector

`billing_subscriptions.last_event_at TIMESTAMPTZ NOT NULL` atualizado a cada transição. No início do use case:

```
if payload.occurred_at <= sub.last_event_at && transition_is_regression(sub.status, target_status):
    processed_events.status = 'superseded'
    return 202
```

Transições "regression": qualquer movimento de estado mais "recente" para mais "antigo" no ciclo de vida (ex.: `RENEWED` chegando depois de `LATE` recente, ou `LATE` chegando depois de `RENEWED` recente). REFUNDED nunca é regression.

## Alternativas Consideradas

1. **Apenas order_id/subscription_id (sem status/timestamp) como event_key.** Recusada — não distingue eventos sucessivos para a mesma assinatura (ex.: late → renewed → late vira no-op no segundo late).
2. **Hash SHA-256 do raw body inteiro.** Recusada — robusto a colisões mas falha se a Kiwify reenviar com leve variação (ex.: ordem de campos JSON diferente após retry).
3. **Last-event-wins puro (sem state machine).** Recusada — chargeback antigo poderia ser sobrescrito por renewed posterior; inaceitável para auditoria contábil.
4. **Reorder buffer com janela de 30s.** Recusada — adiciona latência (viola M-04 p95 ≤ 30s) e complexidade.

## Consequências

### Benefícios Esperados

- Idempotência garantida por constraint de DB (não confia em código).
- Out-of-order absorvido sem regressão de estado.
- Audit trail explícito em `processed_events` (campos `status='applied|superseded'`).
- Tratamento simétrico para refund/chargeback (mesmo efeito → mesmo key prefix).

### Trade-offs e Custos

- +1 tabela (`billing_processed_events`) com crescimento linear no volume de webhooks. Mitigação: housekeeping fora do MVP (E4).
- State machine explícita = código adicional vs simplicidade de "last-event-wins"; trade-off pago em legibilidade pelo ganho em correção.

### Riscos e Mitigações

- **R:** Kiwify pode mudar formato de `subscription.id` ou `updated_at`. **M:** Parser tolerante; teste com payload real validado pré-execução.
- **R:** Renewed chega antes de Activated (sub não existe localmente). **M:** Use case cria placeholder ACTIVE com período derivado; quando Activated chega, é no-op idempotente.
- **R:** Refund chega antes do approved original. **M:** Sub criada como REFUNDED já terminal; approved posterior é no-op idempotente (state machine não permite REFUNDED → ACTIVE).

## Plano de Implementação

1. Migration `0006_create_billing_processed_events.up.sql`.
2. `internal/billing/domain/services/transitions.go` + tests exaustivos (6×6 transições).
3. Cada use case: open transaction → INSERT processed_events → aplicar mutation → emit outbox event → commit.
4. Teste de integração `idempotency_replay_test.go` (repete 5×, espera 1 transição + 4 duplicates).
5. Teste de integração `out_of_order_test.go` (renewed → late stale → late fresh).

## Monitoramento e Validação

- Métrica `billing_processed_event_duplicates_total{trigger}`.
- Métrica `billing_event_superseded_total{trigger}`.
- Log: cada duplicate/superseded inclui `event_key`, `original_applied_at`.

## Impacto em Documentação e Operação

- Runbook §9.4: spike de duplicates esperado em Kiwify retry storm; verificar `signature_status` antes de assumir bug.

## Revisão Futura

- Reabrir se o volume de `processed_events` crescer além do esperado (definir housekeeping em E4).
- Reabrir se a Kiwify expandir triggers (será necessário acrescentar entradas na tabela de transições).
