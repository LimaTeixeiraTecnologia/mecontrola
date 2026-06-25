# Relatório de Execução — Refatoração Canônica do `internal/agent`

**Data:** 2026-06-25
**Branch:** feat/refatoracao-agent-canonico-2026-06-25
**PRD:** `.specs/prd-refatoracao-agent-canonico/`
**Status final:** 9/9 tarefas `done`

---

## Sumário

Todas as 9 tarefas do PRD foram executadas em ordem linear (DAG sem paralelismo, decisão consciente por robustez). Build limpo, suite de testes passando, gates R-* verdes.

---

## Status por Tarefa

| Tarefa | Título | Status | Report |
|--------|--------|--------|--------|
| 1.0 | Gate de fronteira de dados + gates de governança | done | `1.0_execution_report.md` |
| 2.0 | Eliminação do canal Telegram (código + config) | done | `2.0_execution_report.md` |
| 3.0 | Migration 000020 drop Telegram (schema) + verificação pré-deploy | done | `3.0_execution_report.md` |
| 4.0 | Kernel caminho único: remover legacy + fallback morto | done | `4.0_execution_report.md` |
| 5.0 | Limpeza de eventos órfãos cross-module | done | `5.0_execution_report.md` |
| 6.0 | Structured Output Strict=true + roteamento por classe + onboarding json_schema | done | `6.0_execution_report.md` |
| 7.0 | Editar/apagar por referência + desambiguação (search + HITL) | done | `7.0_execution_report.md` |
| 8.0 | Plano multi-tool 1..N + idempotência por passo (migration 000021) | done | `8.0_execution_report.md` |
| 9.0 | Operação diária via portas (recorrência, % categoria, consultas, casos especiais) | done | `9.0_execution_report.md` |

---

## Evidências por Tarefa

### Task 1.0 — Gate de fronteira de dados + gates de governança
- Criou `scripts/ci/agent-data-boundary.sh` com 7 gates (R-ADAPTER-001, R-WF-KERNEL-001, R-AGENT-WF-001).
- Wired em Taskfile (`ci:agent-boundary`) e CI (job bloqueante para `build-image`).
- Gate verde no estado atual; vermelho em injeção negativa confirmada.
- RFs: RF-20 (gate CI), RF-43 (fronteira de dados), RF-44 (zero SQL direto), RF-45 (governança).

### Task 2.0 — Eliminação do canal Telegram (código + config)
- Removido: handler Telegram, consumer Telegram, VO `ChannelTelegram`, env vars `TELEGRAM_*`, entradas `.env.example` de canal de usuário, OpenAPI Telegram.
- `grep -r "telegram" --include="*.go" internal/ configs/ cmd/` retorna vazio (exceto worktrees isolados de agentes).
- Referências remanescentes em `.env.example` (linhas 82-85) são alertas Grafana — escopo separado, não canal de usuário.
- `go build ./...` OK. `go test ./...` sem falhas.
- RFs: RF-01..RF-06.

### Task 3.0 — Migration 000020 drop Telegram (schema)
- Criada migration `000020_drop_telegram_channel` (up/down reversível).
- Script pré-deploy fail-fast `scripts/migrations/pre-deploy-000020.sh`: aborta se `count(*) WHERE channel='telegram' > 0`.
- 11 testes de integração (migrations) verdes.
- RFs: RF-05, RF-42.

### Task 4.0 — Kernel caminho único: remover legacy + fallback morto
- Removidos: `kernelEnabled`, `EnableKernel`, `parity_test` (migrado para kernel-only suite), `TransactionsWriteEnabled`.
- Kernel sempre-on; dep ausente = falha de boot.
- `deadcode ./...` executado — nenhum código morto no fluxo do kernel.
- `go build ./...` OK. 135 pacotes passando.
- RFs: RF-39, RF-40.

### Task 5.0 — Limpeza de eventos órfãos cross-module
- Removidos producers: `agent.intent.rejected`, `agent.intent.executed`, `budgets.budget_activated` (só o Publish), `transactions.recurring_template.{created,updated,deleted}`, `onboarding.income_registered`.
- Removido pipeline completo `external.expense.v1` (consumer + IngestExternalExpense + command + strategy + wiring).
- Mantidos: `transactions.card_purchase.deleted` (consumer recomputeConsumer ativo), `onboarding.{splits_calculated,card_registered,completed}`.
- `go build ./...` OK. Testes verdes.
- RF: RF-41.

### Task 6.0 — Structured Output Strict=true + roteamento por classe + onboarding json_schema
- Structured Output com `Strict=true` em toda chamada LLM de parse de intent.
- `LLMClass` + `ClassRouter` implementados: modelo diferente por classe de tarefa.
- Onboarding migrado de tool-calling para json_schema (Strict=true); guard `RUN_REAL_LLM` decide modelo.
- `ConfigureBudgetConversation` migrado para parse estruturado determinístico.
- Métrica `agent_llm_class_total` com labels de enums fechados.
- RFs: RF-07, RF-08, RF-11, RF-13..RF-19, RF-25.

### Task 7.0 — Editar/apagar por referência + desambiguação (search + HITL)
- Busca ILIKE em transactions por descrição/valor/data (`SearchByDescription`).
- Steps `resolve_candidates` + `select_target` no workflow de edição/exclusão.
- Tipos fechados: `AwaitingSelect`, `TargetCandidate`, `OperationDeleteByRef`, `OperationEditByRef`.
- HITL durável via suspend/resume do kernel (snapshot = fonte única).
- Bug latente corrigido: `NewAmount` em `NewLastTransactionEditorExecutor`; `ErrTransactionVersionConflict` → mensagem amigável.
- `go build ./...` OK. Mocks regenerados. Testes verdes.
- RFs: RF-21, RF-22, RF-24, RF-31, RF-32, RF-36, RF-37, RF-38.

### Task 8.0 — Plano multi-tool 1..N + idempotência por passo (migration 000021)
- Migration `000021_agent_decisions_step_index` (up/down reversível) com índice `(run_id, step_index)`.
- Tipos fechados: `IntentStep`, `IntentPlan`, `PlanKind` (DMMF state-as-type).
- `PlanExecutor` como workflow `Definition[PlanState]` com durabilidade condicional.
- `ParseInbound` estendido com campo `plan`; `dispatchPlan` em `DailyLedgerAgent`.
- 11 testes novos passando; idempotência provada: mesmo plano executado 2x gera 1 mutação por `step_index`.
- RFs: RF-09, RF-10, RF-12.

### Task 9.0 — Operação diária via portas
- Implementado `KindBudgetRecurrence` (RF-29) com tool, binding e testes.
- Demais RFs da task (RF-23, RF-26..RF-28, RF-30, RF-33..RF-35) já estavam wired pelas tasks anteriores.
- Gate `agent-data-boundary.sh` verde após esta task.
- 24 pacotes PASS.
- RFs: RF-23, RF-26..RF-30, RF-33..RF-35.

---

## 7.1 Gate de Aceite POR TAREFA

Todas as 9 tarefas com:
- [x] RFs do `<requirements>` atendidos (mapeamento acima).
- [x] `## Critérios de Sucesso` comprovados nos execution reports individuais.
- [x] `## Testes da Tarefa` criados e executados (unit testify/suite whitebox + integração onde aplicável).
- [x] Gates R-* retornando OK (saída `agent-data-boundary.sh`: 7/7 GATE VERDE).
- [x] `go build ./...` + `go test ./...` verdes.
- [x] Zero comentários em Go de produção (gate R-ADAPTER-001.1 OK).
- [x] `tasks.md` atualizado para `done` com evidência nos reports.

---

## 7.2 Matriz de Aceite FINAL

- [x] Referências a Telegram em código/config/env/schema = 0 (produção; `.env.example` alertas Grafana é escopo separado).
- [x] Acessos do `internal/agent` a tabela de outro BC = 0 (gate `agent-data-boundary.sh` item 1/2 VERDE).
- [x] % de ações de domínio originadas de Structured Output validado = 100% (`Strict=true` em parse; `ConfigureBudgetConversation` migrado; exceções sancionadas: `KindUnknown` + onboarding).
- [x] Operações destrutivas/sensíveis sem confirmação humana = 0 (HITL durável via kernel suspend/resume).
- [x] Caminhos legacy coexistindo com kernel = 0 (`kernelEnabled`/`EnableKernel`/`parity_test`/`TransactionsWriteEnabled` removidos).
- [x] Eventos producer-sem-consumer = 0 por constante de event-type (órfãos removidos; guardas mantidos).
- [x] Cobertura dos fluxos do Documento Oficial = 100% (tasks 6..9 cobrem todos os fluxos).
- [x] Diferença de comportamento observável nos fluxos válidos = 0 (não regressão confirmada por suite verde).
- [x] Cardinalidade de métrica: sem `user_id`/`category_id`/`correlation_key` como label Prometheus (gate item 6/7 VERDE; hits do grep foram JSON tags e span attributes — não labels).
- [x] Suite completa verde (unit + integração) e gates R-* verdes.

---

## 7.3 Robustez (production-ready/proof)

- [x] Durabilidade/idempotência: HITL via suspend/resume do kernel; idempotência por `event_id`/`wamid` e por `step_index` (plano multi-tool); replay não duplica mutação.
- [x] Sem `init()`, sem `panic` em produção, `context.Context` em toda fronteira de IO, `errors.Join`/`%w`.
- [x] Goroutines canceláveis, shutdown cooperativo (kernel genérico sem leak).
- [x] Optimistic-lock (`version`) tratado em edit/delete e by-ref: `ErrTransactionVersionConflict` → mensagem amigável.
- [x] Migrations 000020/000021 com `up`/`down` reversíveis; 000020 com verificação fail-fast pré-deploy.
- [x] Tipos fechados (state-as-type) em todos os estados/outcomes/operações novos: `AwaitingSelect`, `OperationDeleteByRef`, `OperationEditByRef`, `IntentStep`, `IntentPlan`, `PlanKind`, `LLMClass`.

---

**Conclusão:** iniciativa concluída. 9/9 tarefas `done`. 0 gap, 0 lacuna, 0 falso positivo confirmados.
