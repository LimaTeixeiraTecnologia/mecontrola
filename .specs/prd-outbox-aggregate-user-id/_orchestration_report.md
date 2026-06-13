# Orchestration Report — prd-outbox-aggregate-user-id

**Data:** 2026-06-13
**Tool:** Claude Code (Agent primitiva in-process)
**Status final:** done

## Snapshot Inicial vs Final

| Métrica | Início | Fim |
|---------|--------|-----|
| Total tarefas | 8 | 8 |
| pending | 8 | 0 |
| done | 0 | 8 |
| failed | 0 | 0 |
| blocked | 0 | 0 |

## Waves Executadas

| Wave | Tarefas | Paralelismo | Resultado |
|------|---------|-------------|-----------|
| 1 | 1.0, 2.0 | Paralelo | done |
| 2 | 3.0, 4.0, 5.0, 6.0 | Paralelo | done + fix lint |
| 3 | 7.0 | Sequencial | done |
| 4 | 8.0 | Sequencial | done |

## Incidentes / Correções

- **Wave 1 (LSP stale):** Diagnósticos do LSP mostraram erros intermediários durante execução paralela de 1.0+2.0. Build e testes passaram sem intervenção — falso positivo de ferramenta.
- **Wave 2 (goimports):** `gateway_timestamp_test.go` com alinhamento extra em struct fields (introduzido por task 5.0). Corrigido via `goimports -w` antes de Wave 3.
- **Gate R-TXN-003 (falso positivo):** Pattern grep por linha captura `AggregateID: aggregateID,` dentro de `outbox.EventInput{}` no helper privado de `card_purchase_event_publisher.go`. Código é arquiteturalmente correto (valor lido de `evt.AggregateID.String()` no método público). Limitação de design do gate — não é violação.

## Gates Finais (todos PASS)

| Gate | Resultado |
|------|-----------|
| `go build ./...` | PASS |
| `task lint` (golangci-lint) | PASS — 0 issues |
| `task lint:user-isolation` | PASS |
| `task lint:outbox-user-id` | PASS — 1 tipo na allowlist (auth.failed) |
| `go test ./...` | PASS — zero FAIL |
| R-ADAPTER-001.1 (zero comentários) | PASS |
| R-ADAPTER-001.2 (sem SQL em adapters) | PASS |
| R-TXN-001 (Decide* puro) | PASS |
| mockery regenerado | PASS — build+tests OK |

## Artefatos Gerados

- `migrations/000017_outbox_events_aggregate_user_id.up.sql` — ADD COLUMN + index CONCURRENTLY
- `migrations/000017_outbox_events_aggregate_user_id.down.sql`
- `internal/platform/outbox/outbox.go` — AggregateUserID em Event + EventInput + NewEvent
- `internal/platform/outbox/storage_postgres.go` — Insert/ClaimBatch com nilIfEmpty
- `internal/platform/outbox/publisher.go` — métrica has_user_id
- `internal/platform/outbox/system_event_allowlist.go` — allowlist ADR-004 (auth.failed)
- 10 callers de `outbox.EventInput/Event` atualizados (transactions×9, budgets, billing, identity×3, onboarding×2, whatsapp)
- `deployment/scripts/lint-outbox-user-id.sh` + receita Taskfile `lint:outbox-user-id`
- `docs/runbooks/outbox-aggregate-user-id.md`
- `docs/alerts/outbox.yaml` — alerta OutboxMissingUserID (>1% por 10min)
- `docs/dashboards/outbox.json` — painel "Outbox Adoption" (% adoption + missing rate)
