# Relatório de Revisão — Jornada WhatsApp financeira sem falso sucesso

- **Data:** 2026-07-10
- **PRD:** `.specs/prd-jornada-whatsapp-financeira-sem-falso-sucesso` (spec-version 2, hash `b3ed073a…`)
- **Escopo:** working tree não commitado (63 arquivos, +1817/−215), tasks 1.0–8.0 `done`
- **Skill:** `@.claude/skills/review/` + 5 subagentes especializados

## 1. Veredito Final

**APPROVED**

Todos os objetivos, funcionalidades core 1–9, RF-01..RF-30, ADR-001..ADR-006, DoD e regras de
governança estão atendidos com evidência concreta. Zero achados bloqueantes. Os dois itens `low` da
área de identidade (drift documental do ADR-006 e amplitude do teste de anonimização) foram fechados
nesta rodada; o item de lint era limitação de ambiente do subagente e passa limpo com o binário
pinado do repositório. Nenhum falso sucesso, nenhuma personalização perdida, nenhuma confirmação
duplicada indistinguível, correlação por WAMID completa, scorer honesto per-run.

## 2. Arquivos e Referências Lidos

- **Especificação:** `prd.md`, `techspec.md`, `tasks.md`, ADR-001..ADR-006, `_orchestration_report.md`, `1.0`–`8.0_execution_report.md`.
- **Orçamento:** `internal/budgets/domain/commands/create_budget.go`; `internal/agents/application/workflows/{onboarding_workflow,budget_creation_workflow}.go` (+ testes).
- **Pending-entry:** `internal/agents/application/workflows/{pending_entry_workflow,pending_entry_decisions}.go`; `internal/agents/application/usecases/{pending_entry_continuer,register_attempt,idempotent_write}.go`; `write_ledger_repository.go`; `internal/platform/workflow/engine.go` (+ testes/integração).
- **Observabilidade:** `internal/platform/agent/{runtime,ports,errors}.go`; `internal/agents/application/usecases/{run_close,pending_entry_continuer,card_create_confirm_continuer,budget_creation_continuer}.go`; `internal/platform/whatsapp/status/{types,record_message_status,postgres/repository}.go`; `internal/agents/application/reconciliation/reconcile_run_consistency.go`; `internal/agents/infrastructure/postgres/audit_reconciliation.go`; `migrations/000008_*.{up,down}.sql` (+ `migrations_integration_test.go`).
- **Scorers/golden:** `internal/platform/scorer/scorer.go`; `internal/agents/application/agents/scoring_hooks.go`; `internal/agents/application/scorers/{write_persistence_accuracy,behavioral_scorers,mecontrola_scorers}.go`; `internal/agents/application/golden/{cases_journey,journey_test,ratio,harness_realllm_test,registry}.go`; `internal/agents/application/postdeploy/{gate,reader}.go`.
- **Identidade:** `internal/identity/application/usecases/{establish_principal,auth_event_payload,record_gateway_auth_failure}.go`; `internal/identity/domain/{auth_resolve_path,entities/auth_event,services/principal_workflow}.go`; `internal/identity/infrastructure/repositories/postgres/{user_identity_repository,auth_events_repository}.go` (+ testes/integração).

## 3. Matriz de Rastreabilidade

### Objetivos do PRD

| Objetivo | Status | Evidência |
|---|---|---|
| Zero falso sucesso | atendido | `DecidePostWrite` (routed+Nil ⇒ `StepStatusFailed`+`ErrWriteAcceptedWithoutResource`); engine mapeia `StepStatusFailed→RunStatusFailed`; golden `TestInvariantNoFalseSuccessOnEmptyResource`; real-LLM `erro_de_tool_nao_inventa_sucesso` PASS |
| Zero cancelamento indevido | atendido | falha de escrita mantém `PendingStatusActive`, nunca `Cancelled` (RF-11); `Cancelled` só p/ cancelamento explícito/expiração/substituição |
| Zero personalização perdida | atendido | `create_budget.go:69` `!= 10000`; ramo `confirm` não aplica default com valores não-nulos; integração `TestInteg_PersonalizacaoCasoReal`; real-LLM `jornada_personalizacao_valida…` 3/3 |
| Correlação completa | atendido | `InboundRequest.Validate()` exige `MessageID`; `runtime.go` seta `CorrelationKey=MessageID`; CHECK `char_length BETWEEN 1 AND 256` |
| Concordância de estado | atendido | `audit_reconciliation.go` + `DecideViolations` cobrem 5 invariantes (empty_correlation_key, succeeded_without_effect, failed_with_orphan_write, status_divergence, workflow_state_divergence) |
| Scorer honesto | atendido | `write_persistence_accuracy` (code-based, per-run) reprova write sem efeito; `AfterTool` propaga `Outcome` real (resultBytes + tools com erro) |
| Regressão barrada | atendido | golden verde (56 unit) + real-LLM ≥0,90 por categoria (228,6s) |

### Funcionalidades Core, RF, ADRs

| Item | Status | Evidência-chave |
|---|---|---|
| Core 1 / RF-01,02,03,04,29 / ADR-003 | atendido | validação simétrica `!=10000` em domínio + `DecideDistribution==10000` no workflow; `DecideAllocationKind` puro reais-vs-percent; testes 9000/11000/10000 + caso real |
| Core 2 / RF-05,06 | atendido | pendência determinística; `ErrRunAlreadyExists→ActivePendingEntryMessage`; `MultiItemOrientationMessage` ausente do fluxo de pendência |
| Core 3 / RF-07,08,10,11,12 / ADR-001 | atendido | 1 ledger + 1 transação por WAMID original; escrita aceita vazia ⇒ erro tipado; `TestRF08_PlatformMessagesContainsInboundAndFinalResponse` |
| Core 4 / RF-09,30 / ADR-002 | atendido | chave ledger `(wamid,item_seq,operation)` inalterada; `maxFailedWriteResumes=1`; replay vs retry por outcome; TTL 30min ⇒ `PendingStatusExpired` |
| Core 5 / RF-13,14,15,16,22,23,24 / ADR-005 | atendido | 3 continuers via `closeObservedRun` (log+`agents_run_update_errors_total`); migration 000008 backfill antes do CHECK; labels fechados |
| Core 6 / RF-17,18 / ADR-004 | atendido | scorer per-run code-based; `no_hallucination` endurecido; `AfterTool` sem descarte; layering (scorer não importa agent) |
| Core 7 / RF-19 | atendido | `MessageDeliveryState` fechado (not_received/failed/delivered) read-only |
| Core 8 / RF-20,21 / ADR-006 | atendido | `ensureIdentityLink` (mesma tx) via `InsertIfAbsent`+SAVEPOINT; `AuthResolvePath` fechado; integração legacy→identity idempotente + concorrência |
| Core 9 / RF-25,26,27,28 | atendido | golden anonimizado 5 categorias de jornada; gates Go verdes; real-LLM ≥0,90/categoria; adapters finos |

**Todas as tasks 1.0–8.0:** refletidas na implementação e cobertas por testes. **DoD:** invariantes + golden + gates Go + real-LLM + gates de governança — todos verdes.

## 4. Achados

Nenhum achado bloqueante encontrado com base nas evidências revisadas. Dois itens `low` foram
identificados e **fechados nesta rodada**:

- **F-1 (low, ADR-006) — RESOLVIDO:** o código usa `InsertIfAbsent`+`SAVEPOINT`/`ROLLBACK TO SAVEPOINT` (padrão canônico Postgres que evita envenenar a transação externa do `uow.Do`), superior à prescrição original do ADR (`Insert`+no-op no chamador, que abortaria a jornada). O código já estava correto; a ADR-006 (§1 e Plano item 5) foi atualizada para refletir a implementação materializada.
- **F-2 (low, RF-26 lint) — NÃO-DEFEITO:** o subagente não conseguiu rodar `golangci-lint` por incompatibilidade de versão do ambiente (binário v1.64.8 vs config v2). Com o binário pinado do repositório (`task lint:run`), o gate retorna **0 issues** + todos os gates de governança PASS.
- **Observação de anonimização (low) — RESOLVIDA:** `TestJourneyGoldenIsAnonymized` asseria a blocklist só contra `c.Input`; endurecido para varrer `Origin`, `ResponseDescribe` e `PriorTurns[].UserMessage`. Passa (56 testes), confirmando ausência de dado pessoal em todos os campos.

## 5. Bugs Canônicos para Bugfix

Nenhum bug canônico gerado.

## 6. Validações Executadas

| Validação | Comando | Resultado |
|---|---|---|
| Build | `go build ./...` | exit 0 |
| Vet | `go vet ./...` | exit 0 |
| Test -race | `go test -race -count=1` (pacotes alterados + jornada) | 2459 passed / 85 pacotes, exit 0 |
| Lint pinado + governança | `task lint:run` | golangci-lint **0 issues**; auth-bypass, outbox-user-id (+regressão 7/7), deadcode **PASS**; exit 0 |
| Zero comentários (R-ADAPTER-001.1) | grep prod | limpo |
| Prefixo `_` (R5.26) | grep arquivos alterados | limpo |
| State-as-type | grep status=string | apenas constantes tipadas `AuthResolvePath` (correto) |
| Golden (delta anonimização) | `go test ./internal/agents/application/golden/ -count=1` | 56 passed, exit 0 |
| **Real-LLM (RF-27)** | `RUN_REAL_LLM=1 go test -tags integration -run TestGoldenRealLLMSuite` | **PASS ≥0,90 por categoria, 228,6s** (todas as 5 jornadas novas 3/3 + `erro_de_tool_nao_inventa_sucesso`) |

Testes de integração Postgres (`-tags integration` + testcontainers) das áreas de write-ledger,
correlação, reconciliação e identidade estão presentes e cobrem os contratos; executam em CI com a
tag ativa.

## 7. Subagentes

| Subagente | Área | Veredito | Achados |
|---|---|---|---|
| review-budget-customization | RF-01..04,29 / ADR-003 | APPROVED | 0 |
| review-pending-entry-idempotency | RF-05..12,30 / ADR-001,002 | APPROVED | 0 |
| review-runs-observability | RF-13..16,19,22..24 / ADR-005 | APPROVED | 0 |
| review-scorers-golden | RF-17,18,25,27 / ADR-004 | APPROVED | 0 (1 obs. low, resolvida) |
| review-governance-tests | RF-20,21,26,28 / ADR-006 | APPROVED_WITH_REMARKS | 2 low (F-1, F-2), ambos resolvidos/não-defeito |

Decisão final centralizada no agente principal; todos os itens `low` fechados antes do veredito.

## 8. Riscos Residuais e Ressalvas

Nenhuma ressalva, gap ou lacuna identificada.

## 9. Próxima Ação

Nenhuma ação de remediação pendente. A entrega está pronta para commit. As mudanças (63 arquivos)
permanecem **não commitadas** no working tree — o commit/push é decisão do usuário. A ADR-006 e o
teste de anonimização foram atualizados nesta rodada de revisão.
