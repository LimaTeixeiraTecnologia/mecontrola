<!-- spec-hash-prd: c3f28341962b9b41d0ef93c214d36f501fdd68c74b8fede2e55da20081128b64 -->
<!-- spec-hash-techspec: 39c5c696abe43f81180e3e47e8f05891fee718ee1cc2f4186b1aab1d1deb0a94 -->
# Especificação Técnica — Outbox Aggregate User ID Top-Level

## Resumo Executivo

Adiciona `aggregate_user_id` como campo de primeira classe no envelope `outbox.Event` (struct Go + coluna SQL + envelope JSON), populando-o em todos os 12 callers atuais distribuídos em 5 módulos. Rollout atômico (migration + código + producers no mesmo deploy), sem dual-write, sem fase intermediária. Validação opcional na v1 (warning + métrica), obrigatória em v2 futura quando coverage = 100%.

Decisões materiais (cravadas em 4 ADRs):
- **ADR-001**: coluna NULL na v1; validação obrigatória + NOT NULL ficam para v2.
- **ADR-002**: rollout atômico single-deploy; sem dual-write.
- **ADR-003**: registros antigos permanecem NULL; sem backfill (housekeeping eventualmente limpa).
- **ADR-004**: allowlist explícita de `EventType` legitimamente de sistema (sem dono).

## Arquitetura do Sistema

### Visão Geral dos Componentes

**Novos**

| Componente | Caminho | Responsabilidade |
|---|---|---|
| Migration | `migrations/000017_outbox_events_aggregate_user_id.up.sql/.down.sql` | Coluna + index parcial |
| `internal/platform/outbox/system_event_allowlist.go` | novo | Allowlist de event types sem user_id |
| `deployment/scripts/lint-outbox-user-id.sh` | novo | Gate CI |

**Modificados**

| Componente | Caminho | Mudança |
|---|---|---|
| `internal/platform/outbox/outbox.go` | — | Campo `AggregateUserID` em `Event`, `EventInput`; validação opcional em `NewEvent`; métrica |
| `internal/platform/outbox/envelope.go` | — | Campo no envelope JSON via `Pack` |
| `internal/platform/outbox/storage_postgres.go` | — | INSERT/SELECT incluem coluna |
| 12 callers (transactions/budgets/billing/identity/onboarding/whatsapp) | — | Popular `AggregateUserID` |
| `taskfiles/lint.yml` | — | Receita `lint:outbox-user-id` |

### Fluxo de Dados

```
Use case / Producer
  -> entity (carrega UserID) -> services.Decide* -> entity.Event{UserID, ...}
  -> Producer monta EventInput{
       AggregateType, AggregateID,
       AggregateUserID: evt.UserID.String(),  // NOVO
       Payload, Metadata, OccurredAt,
     }
  -> outbox.NewEvent(input) -> Event{..., AggregateUserID}
  -> outbox.Publisher.Publish(ctx, evt) -> Storage.Insert -> outbox_events row
  -> Dispatcher.ClaimBatch -> Row{..., AggregateUserID}
  -> Pack(row) -> Envelope JSON com aggregate_user_id top-level
  -> Registry handler consome
```

## Design de Implementação

### Interfaces Chave

**`outbox.Event` (modificado)**

```go
type Event struct {
    ID              string
    Type            string
    AggregateType   string
    AggregateID     string
    AggregateUserID string  // NOVO — string vazia = ausente
    Payload         []byte
    Metadata        map[string]string
    OccurredAt      time.Time
}
```

**`outbox.EventInput` (modificado)**

```go
type EventInput struct {
    ID              string
    Type            string
    AggregateType   string
    AggregateID     string
    AggregateUserID string  // NOVO — opcional na v1
    Payload         []byte
    Metadata        map[string]string
    OccurredAt      time.Time
}
```

**`outbox.NewEvent` (modificado)**

```go
func NewEvent(input EventInput) (Event, error) {
    // ... validações existentes ...

    if input.AggregateUserID != "" {
        if _, err := uuid.Parse(input.AggregateUserID); err != nil {
            return Event{}, ErrInvalidAggregateUserID
        }
    } else if !isSystemEvent(input.Type) {
        // log warn estruturado + métrica has_user_id="false"
        // sem retornar erro na v1 (compat)
    }
    // ... construção ...
}
```

**`outbox.Pack` (modificado)**

```go
type Envelope struct {
    ID              string            `json:"id"`
    EventType       string            `json:"event_type"`
    AggregateUserID string            `json:"aggregate_user_id,omitempty"`  // NOVO
    OccurredAt      time.Time         `json:"occurred_at"`
    Metadata        map[string]string `json:"metadata"`
    Payload         json.RawMessage   `json:"payload"`
}
```

**Allowlist (novo)**

```go
package outbox

var systemEventTypes = map[string]struct{}{
    // declarar explicitamente os tipos legitimamente sem user_id
    // (a definir durante implementação; provavelmente vazio no MVP)
}

func isSystemEvent(eventType string) bool {
    _, ok := systemEventTypes[eventType]
    return ok
}
```

### Modelos de Dados

**Migration `000017_outbox_events_aggregate_user_id.up.sql`**

```sql
ALTER TABLE mecontrola.outbox_events
    ADD COLUMN aggregate_user_id UUID NULL;

CREATE INDEX IF NOT EXISTS outbox_events_aggregate_user_id_idx
    ON mecontrola.outbox_events (aggregate_user_id)
    WHERE aggregate_user_id IS NOT NULL;
-- Nota: `CONCURRENTLY` não é usado porque golang-migrate envolve cada arquivo em transação.
-- O índice parcial nasce vazio (coluna 100% NULL na criação), então o AccessExclusiveLock é
-- instantâneo. Ver 1.0_execution_report.md.
```

**`down`**

```sql
DROP INDEX IF EXISTS mecontrola.outbox_events_aggregate_user_id_idx;
ALTER TABLE mecontrola.outbox_events DROP COLUMN aggregate_user_id;
```

### Storage (postgres)

`Insert` adiciona coluna no `INSERT INTO ... (col1, col2, ..., aggregate_user_id, ...) VALUES ($1, $2, ..., $N, ...)`. Quando `evt.AggregateUserID == ""`, passar `sql.NullString{}` ou interface vazia (database/sql trata `nil` para `UUID NULL`).

`ClaimBatch` adiciona coluna no SELECT e no Scan com `sql.NullString` ou ponteiro.

## Pontos de Integração

- **12 callers atualizados** (lista em PRD RF-14). Cada um lê `UserID` da entity/event do domínio e popula `AggregateUserID: evt.UserID.String()`.
- **Consumers existentes**: nenhum precisa mudar para funcionar — Go ignora campo JSON desconhecido. Consumers que quiserem aproveitar podem ler o novo campo do envelope.
- **Dispatcher e Reaper**: scan inclui novo campo; comportamento de dispatching/retry preservado.
- **Housekeeping**: sem mudança (filtra por `published_at`).

## Abordagem de Testes

### Testes Unitários

- **`outbox/outbox_test.go`**: tabela cobrindo `NewEvent` com `AggregateUserID` válido, inválido (não-UUID), ausente (warning + métrica), allowlist hit (silent).
- **`outbox/envelope_test.go`**: `Pack` inclui campo; `omitempty` quando string vazia.
- **`outbox/storage_postgres_integration_test.go`** (build tag `integration`): Insert + ClaimBatch round-trip com e sem `AggregateUserID`; SQL real.
- **Cada producer atualizado**: teste verifica que `EventInput.AggregateUserID == expectedUserID.String()`.

### Testes de Integração

**Necessário? SIM**:
- [x] Fronteira IO crítica: tabela `outbox_events` é fonte de verdade auditável.
- [x] 12 sites tocados aumenta risco de regressão.
- [x] Custo já assumido: `testcontainers-go` em uso.

Cobertura mínima:
- Migration `up` → coluna existe; `down` → coluna some.
- Insert + Claim round-trip com user_id presente e ausente.
- Não regredir testes do dispatcher (claim → publish → mark_published).

### Testes E2E

Smoke staging: rodar uma transação real, verificar que `outbox_events.aggregate_user_id` é populado.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Migration + storage**: coluna + Insert/Claim atualizados. Sem código dependente ainda. Testes integration validam round-trip.
2. **outbox.Event + EventInput + NewEvent + envelope.Pack + allowlist + métrica**: campo Go + validação opcional + envelope JSON. Sem caller ainda.
3. **Atualizar 12 callers em paralelo** (5 módulos):
   - transactions (3 producers)
   - budgets (1 producer)
   - billing (1 producer)
   - identity (3 sites)
   - onboarding (3 sites)
   - whatsapp dispatcher (1 site)
4. **Gate `lint:outbox-user-id`** + receita Taskfile.
5. **Smoke + observabilidade**: dashboard mostra `has_user_id="true"` > 99%.

### Dependências Técnicas

- `crypto/sha256`, `encoding/json`, `database/sql`, `github.com/google/uuid` (já em uso).
- Migration via golang-migrate (já em uso).
- Nenhuma nova dep.

## Monitoramento e Observabilidade

**Métricas Prometheus**

- `outbox_events_inserted_total{has_user_id}` com `has_user_id` ∈ {`"true"`, `"false"`}. Sem `event_type` em label se cardinalidade preocupar (avaliar).
- (Existente) `outbox_events_published_total`, etc., sem mudança.

**Alertas**

- `rate(outbox_events_inserted_total{has_user_id="false"}[5m]) / rate(outbox_events_inserted_total[5m]) > 0.01` por 10 min → warn (callers não populando).

**Logs**

- Em `NewEvent` com `AggregateUserID == ""` e `!isSystemEvent`: `slog.Warn("outbox.event.missing_aggregate_user_id", "event_type", ...)`. Sem `user_id` em label.

## Mapeamento Requisito → Decisão → Teste

| RF | ADR | Implementação | Teste |
|---|---|---|---|
| RF-01–RF-04 | ADR-001 | migration 000017 | integration |
| RF-05–RF-10 | ADR-001, ADR-004 | `outbox.go`, `envelope.go` | unit |
| RF-11–RF-13 | ADR-002 | `storage_postgres.go` | integration |
| RF-14 | ADR-002, ADR-003 | 12 callers | unit (cada producer) |
| RF-15 | ADR-004 | allowlist + isSystemEvent | unit |
| RF-16–RF-17 | M-08 | script + Taskfile | gate CI |
| RF-18–RF-20 | governance | go-implementation | CI |

## ADRs Vinculadas

1. [ADR-001 — Coluna NULL na v1 com validação obrigatória adiada para v2](adr-001-nullable-v1-strict-v2.md)
2. [ADR-002 — Rollout atômico single-deploy sem dual-write](adr-002-rollout-atomico-sem-dual-write.md)
3. [ADR-003 — Registros antigos permanecem NULL, sem backfill](adr-003-sem-backfill-registros-antigos.md)
4. [ADR-004 — Allowlist explícita de event types de sistema](adr-004-allowlist-eventos-sistema.md)

## Riscos e Desvios

- **Desvio R-ADAPTER-001.1**: zero. Toda doc fica em PRD/techspec/ADRs.
- **Desvio "não abstrair tempo"**: `OccurredAt` continua passado por argumento; sem `Clock`. ✓
- **Desvio R6**: `NewEvent` é função pura (sem ctx); validação não faz IO. ✓

## Itens em Aberto

Nenhum. 4 ADRs cravadas.
