# Resultado da Revisão — Orquestração Conversacional Confiável do Agente MeControla

- **Data:** 2026-07-09
- **PRD:** `.specs/prd-orquestracao-conversacional-confiavel/` (spec-version 1, hash `67713f8f…`)
- **Prompt executado:** `docs/reviews/2026-07-09-review-prd-orquestracao-conversacional-confiavel.md`
- **Ciclo:** review → bugfix → review (1 rodada de remediação, 2 achados corrigidos)

---

## 1. Veredito Final

### `APPROVED`

O contrato de **0 gaps, 0 lacunas, 0 falsos positivos e 0 ressalvas** foi cumprido após uma rodada de
`bugfix` que corrigiu os 2 achados acionáveis pela causa raiz, com teste de regressão para cada um. Todos
os RF-01..RF-57, o critério de sucesso primário composto, os ADR-001..ADR-005, as regras hard de governança
e o gate real-LLM pré-deploy (≥ 0,90 por categoria) estão implementados e validados com evidência concreta.

---

## 2. Arquivos e Referências Lidos

**Especificação:** `prd.md`, `techspec.md`, `tasks.md` (encontrado — RF-06 confrontado), ADR-001..ADR-005,
relatórios de execução `1.0..8.0_execution_report.md`.

**Governança:** `AGENTS.md`, `CLAUDE.md`, `.claude/rules/{governance,go-adapters,go-testing,agent-workflows-tools,workflow-kernel,transactions-workflows,input-dto-validate}.md`.

**Código (produção + testes) — lido pelos 4 subagentes + verificação direta:**
- Guardas/runtime: `agents/guard_chain.go`, `agents/guards/{multi_item,verbatim_relay,empty_answer,internal_terms,success_without_tool,card_provenance,types}.go`, `agents/mecontrola_agent.go`, `agents/scoring_hooks.go`, `platform/agent/{runtime,types,agent,ports,noop_hooks}.go` + testes.
- Tools/workflows: `tools/{register_expense,create_recurrence,query_card_invoice,resolve_card}.go`, `binding/card_manager_adapter.go`, `workflows/{pending_entry,onboarding,destructive_confirm,card_create_confirm,budget_creation}_workflow.go` + integração; `card/{domain/errors,application/usecases/get_card,application/interfaces/errors,infrastructure/repositories/postgres/card_repository}.go`.
- Scorers/golden/gates: `scorers/{mecontrola_scorers,behavioral_scorers}.go`, `golden/*` (harness det. + real-LLM), `postdeploy/{gate,regression_contract}.go`, `infrastructure/persistence/postdeploy/aggregate_reader.go`, `module.go`, `taskfiles/test.yml`, `docs/{runbooks,alerts,dashboards}/mecontrola-agent-gate-posdeploy.*`.
- Governança: `configs/config.go`, `cmd/{server,worker}`, `.env.example`, `deployment/config/prod.env`.

---

## 3. Matriz de Rastreabilidade

Status: `atendido` / `não atendido` / `não verificável`. (Todos `atendido` após o bugfix.)

| Item | Fonte | Status | Evidência | Validação |
|------|-------|--------|-----------|-----------|
| RF-01 | Grupo A | atendido | `guard_chain.go` CoR ordenada pre/post; wrap `WithGuardChain` preserva `BuildMeControlaAgent` | unit |
| RF-02 | Grupo A | atendido | `guards/multi_item.go` absorve o guard como 1º PreGuard; `MultiItemOrientationMessage`+`ToolOutcomeClarify` verbatim; git-show do arquivo deletado confirma equivalência | unit regressão |
| RF-03 | Grupo A | atendido | cada regra P0 → guard com teste; prompt = reforço | unit |
| RF-04 | Grupo A | atendido | `multi_item.go` curto-circuita antes do LLM sem write-tool; `guard_chain_test.go` prova `next` não chamado | unit |
| RF-05 | Grupo A | atendido | `R$ 1.234,56`/`R$ 13.874,40` não disparam multi-item | unit |
| RF-06 | Grupo A | atendido | `agent_guard_decisions_total{agent_id,guard,decision}` fechado (`pass`/`handled`) | unit |
| RF-07/08 | Grupo B | atendido | matriz C1–C7 no prompt + golden real-LLM (categorias query/follow-up ≥0,90) | real-LLM |
| RF-09 | Grupo B | atendido | `guards/success_without_tool.go` força fallback+erro sem write-tool bem-sucedido | unit |
| RF-10 | Grupo B | atendido (pós-bugfix) | `internal_terms.go` blocklist + prompt asterisco simples; **F-TERMS corrigido**: add `correlation`/`infraestrutura` | unit regressão |
| RF-11 | Grupo C | atendido | tool fina → usecase → workflow confirmação; zero SQL/branching | unit/integração |
| RF-12 | Grupo C | atendido | verbatim em 2 camadas (extractVerbatim + `verbatim_relay` PostGuard) | unit |
| RF-13 | Grupo C | atendido | confirmação resolvida por `pending_entry_workflow` resume, sem write LLM novo | integração |
| RF-14 | Grupo C | atendido | idempotência WAMID+ItemSeq; `TestInteg_DuplicateWamid…SingleEffect` | integração |
| RF-15 | Grupo C | atendido | estado salvo no Snapshot antes de clarify; resume merge-patch antes do parse | integração |
| RF-16 | Grupo D | atendido | `card_provenance.go` PostGuard: consumidora sem resolve prévio → clarify | unit |
| RF-17 | Grupo D | atendido (pós-bugfix) | **F-CARD corrigido**: `GetCard` remapeia not-found → `agentsifaces.ErrCardNotFound`; UUID fabricado → clarify limpo | unit regressão |
| RF-18 | Grupo D | atendido | `resolve_card` `found=false` → escolha (sem auto-create); PostGuard + tool-level | unit |
| RF-19 | Grupo E | atendido | `named_without_year` (month set, year absent) → `DecideCompetence` sem inferência | unit |
| RF-20 | Grupo E | atendido | `MonthRefKind` enum `int` fechado; `ParseMonthRefKind` rejeita inválido; clarifyPrompt verbatim | unit |
| RF-21 | Grupo E | atendido | `FormatCompetencePtBR` "junho de 2026"; 12 meses testados | unit |
| RF-22 | Grupo F | atendido | erro/vazio/truncamento → `Failed`+fallback seguro sem valor inventado | unit |
| RF-23 | Grupo F | atendido | `runtime.go` consulta `TruncatedByLength` → `ToolOutcomeTruncated`+métrica+log+Failed | unit |
| RF-24 | Grupo F | atendido | `MaxTokens` default 3072, `AGENT_MECONTROLA_MAX_TOKENS`, range (0..8192] | unit |
| RF-25 | Grupo F | atendido | `agent_message_append_errors_total{agent_id,role}`+log | unit |
| RF-26 | Grupo F | atendido | `RunStore.Update` err observado; `agent_runs_total` não incrementa em falha | unit |
| RF-27 | Grupo F | atendido | `aggregateToolErrorContent` (cap 3) agrega múltiplos erros | unit |
| RF-28 | Grupo F | atendido | `agent_run_scorer_skipped_total{agent_id,reason}` (enum fechado) | unit |
| RF-29 | Grupo G | atendido | 3 scorers atuais mantidos; `regression_contract_test` asserta 3+9 | unit |
| RF-30 | Grupo G | atendido | 9 scorers comportamentais em `behavioral_scorers.go` | unit |
| RF-31 | Grupo G | atendido | `postdeploy/gate.go` consome ambos os conjuntos | unit |
| RF-32/34 | Grupo G | atendido | log por `run_id`/`scorer_id`/`stage`, sem conteúdo de mensagem | unit |
| RF-33 | Grupo G | atendido | sem label de alta cardinalidade; hits são span/log attrs e JSON envelope | gate grep |
| RF-35/36 | Grupo H | atendido | golden 13 categorias, 33 casos; cada caso declara input/tool/args/outcome/property | unit |
| RF-37 | Grupo H | atendido | sintéticos + incidentes anonimizados; grep PII/WAMID/resourceId vazio | gate grep |
| RF-38 | Grupo H | atendido | tool-call/completude/categorização/falha/p95 (`agent_run_duration_seconds`)/truncamento | unit |
| RF-39 | Grupo I | atendido | **gate real-LLM ≥0,90 por categoria EXECUTADO: `ok golden 168.6s`, exit 0** | real-LLM |
| RF-40 | Grupo I | atendido | det. por-PR (untagged); real-LLM `//go:build integration`+`RUN_REAL_LLM=1`, task `golden:gate` | config |
| RF-41 | Grupo I | atendido | `require.False(failed)` bloqueia categoria < threshold | real-LLM |
| RF-42 | Grupo I | atendido | `RedefinedToolCallAccuracy` denominador exclui clarify/replay; runbook 0,304→0,354 | unit |
| RF-43 | Grupo I | atendido | `gate.go` rollback (falha/scorers/truncamento/dup-write) + `MeetsMinimumSample` N≥100/≥14d | unit |
| RF-44 | Grupo J | atendido | renome underscore em onboarding/budget workflows; gate R5.26 limpo | gate grep |
| RF-45 | Grupo K | atendido | efeito único + texto determinístico (sucesso/cancel/expira/replay); integração resume/TTL/CAS/reaper | integração |
| RF-46 | Grupo K | atendido | terminal → `Succeeded`/`Failed`; reaper encerra órfão; nunca `Suspended` | integração |
| RF-47 | Grupo L | atendido | `ToolOutcomeTruncated` no enum fechado; estados de fronteira `int` | unit |
| RF-48 | Grupo L | atendido | PreGuard curto-circuita sem `next.Execute` (LLM) | unit |
| RF-49 | Grupo M | atendido | `gate.go` baseline 19/4/23, 0,304/0,149/0,565 | unit |
| RF-50 | Grupo M | atendido | thresholds baseline+0,05; `NoRegressionOperational` | unit |
| RF-51 | Grupo M | atendido | `MinimumSampleRuns=100`, `MinimumSampleWindowDays=14` | unit |
| RF-52 | Grupo M | atendido | veredito por `agent_id`/`run_id` rastreável | unit |
| RF-53 | Grupo M | atendido | alerta `AgentRunTruncatedTotalIncreased`; dup-write no gate | config |
| RF-54 | Grupo M | atendido | `module_wiring_source_test` AST 1:1; nenhuma tool removida | unit |
| RF-55 | Grupo M | atendido | `BuildMeControlaAgent`/contratos públicos preservados | unit |
| RF-56/57 | Grupo M | atendido | 18 fluxos existentes cobertos por teste/golden/regressão | unit |
| Critério primário | PRD §Critério de sucesso | atendido | pré-deploy real-LLM ≥0,90 ✓; gate pós-deploy definido; sem regressão; dívida underscore fechada | real-LLM+unit |
| ADR-001..005 | ADRs | atendido | CoR, runtime robustez, cardId 2 camadas (dead branch corrigido), scorers, golden/gate | — |
| R-ADAPTER/AGENT-WF/WF-KERNEL/TESTING/DTO/R5.26 | Governança | atendido | zero comentários, kernel intocado, aditivo, testify/suite, sem underscore | gates+lint |

---

## 4. Achados

Todos os achados foram **corrigidos** no ciclo de bugfix desta revisão. Estado pós-remediação:
`Nenhum achado remanescente com base nas evidências revisadas.`

Achados originais (rodada 1) e resolução:

| ID | Severidade | RF | Resolução |
|----|-----------|-----|-----------|
| F-CARD | high | RF-17/ADR-003 | **fixed** — remap not-found em `card_manager_adapter.go:GetCard` + 2 testes |
| F-TERMS | low | RF-10 | **fixed** — `correlation`/`infraestrutura` na blocklist + 3 testes |
| L-1 | low | RF-30/54 | **no_change_needed** — helper público pré-existente, não é gap; remover cortaria RF-54 |
| F-2 | low | RF-33 | **no_change_needed** — observação documental, confirma compliance, não é defeito |

---

## 5. Bugs Canônicos para Bugfix

Consumidos e corrigidos nesta revisão (ver `.specs/prd-orquestracao-conversacional-confiavel/bugfix_report.md`).
Não restam bugs canônicos abertos.

---

## 6. Validações Executadas

| Comando | Escopo | Resultado |
|---------|--------|-----------|
| `go build ./...` | repo | PASS (exit 0) |
| `go vet ./internal/agents/... ./internal/platform/...` | módulos afetados | PASS |
| `go test -race -count=1 ./internal/agents/... ./internal/platform/agent/...` | suíte determinística completa | PASS (todos os pacotes `ok`) |
| `./.tools/bin/golangci-lint run` (v2 pinned) | agents/guards/binding/platform | PASS (0 issues) |
| `RUN_REAL_LLM=1 go test -tags=integration ./internal/agents/application/golden/...` | **gate pré-deploy RF-39** | **PASS (`ok golden 168.6s`, exit 0) — ≥0,90 por categoria contra OpenRouter real** |
| Gate R5.26 (grep underscore) | agents/platform | PASS (vazio) |
| Gate zero-comentários (grep `^//`) | agents/platform | PASS (vazio) |
| Gate cardinalidade métrica | agents/platform | PASS (hits são span/log/JSON, não labels) |
| Gate kernel intocado (`git diff --stat main -- internal/platform/workflow/`) | kernel | PASS (vazio) |
| Testes de regressão novos | `TestGetCard_*` (2), `internal_terms` (3) | PASS |

Nenhuma falha preexistente relevante. Nota operacional: o `golangci-lint` do sistema é v1.64.8 (rejeita a
config v2); o binário correto é `./.tools/bin/golangci-lint` (usado pelo CI/Taskfile).

---

## 7. Subagentes

Quatro subagentes `reviewer` especializados, disparados em paralelo, cada um retornando apenas achados com
evidência:

- **review-guards-runtime** (Grupos A/B/F/L): `APPROVED_WITH_REMARKS` → F-TERMS (low) + F-2 (documental). Confirmou RF-01..06, RF-09/10, RF-22..28, RF-47/48 com evidência de código e teste.
- **review-financial-tools** (Grupos C/D/E/K): `REJECTED` → F-CARD (high, dead branch RF-17). Confirmou RF-11..21, RF-45/46. Análise de cadeia de sentinelas de erro foi a chave para o achado.
- **review-scorers-golden** (Grupos G/H/I/M): `APPROVED` → L-1 (low, não-defeito). Confirmou RF-29..43, RF-49..57.
- **review-governance-tests** (Grupo J + arquitetura + suíte det.): `APPROVED`, sem achados. Rodou build/vet/test/lint verdes; confirmou RF-44, kernel intocado, aditividade de `internal/platform/agent`.

A síntese, verificação dos achados contra código real, execução do gate real-LLM e o ciclo de bugfix
permaneceram no agente principal.

---

## 8. Riscos Residuais e Ressalvas

`Nenhuma ressalva, gap ou lacuna identificada.`

---

## 9. Próxima Ação

`APPROVED`: a implementação está em **conformidade total** com o PRD, techspec e ADR-001..ADR-005. Os 57 RFs,
o critério de sucesso primário composto e todas as regras hard de governança estão atendidos com evidência
concreta; o gate real-LLM pré-deploy (≥ 0,90 por categoria) foi executado e passou; os 2 achados da revisão
foram corrigidos pela causa raiz com teste de regressão. O working tree permanece **não commitado** (nenhum
commit foi feito nesta revisão). Próximo passo do usuário: commit + execução do gate pós-deploy monitorado
após deploy, conforme runbook `docs/runbooks/mecontrola-agent-gate-posdeploy.md`.
