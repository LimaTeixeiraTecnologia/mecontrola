# Resultado da Revisão — Orçamento Retroativo Conversacional e Mês por Extenso

- **Data:** 2026-07-09
- **PRD:** `.specs/prd-orcamento-retroativo-conversacional-e-mes-por-extenso/` (spec-version 2, spec-hash `ed471323…`)
- **Módulo:** `internal/agents` sobre `internal/platform`; domínio `internal/budgets`
- **Método:** review estrito (skill `review`) + 3 subagentes especializados (domínio / workflow+continuer / tool+wiring+consumer) + gates real-LLM `RUN_REAL_LLM=1` + integração Postgres.

## 1. Resumo Executivo

**Veredito: `APPROVED`.** Todos os RF-01 a RF-30, todos os critérios de aceite, todo o DoD e todas as regras de negócio estão implementados e validados, com **0 gaps, 0 lacunas, 0 ressalvas e 0 falsos positivos**. A única ressalva candidata (TTL não refrescado por turno) foi **investigada e descartada como falso positivo**: a implementação espelha fielmente o contrato de confirmação nomeado pela SUP-1 (fluxo de confirmação destrutiva), que ancora `SuspendedAt` no Start e não refresca por turno.

## 2. Matriz de Rastreabilidade

| Item | Status | Evidência |
|---|---|---|
| RF-01 tool fina `create_budget` por registry | ✅ | `create_budget.go:94-163`; sem SQL/regra/branching; `module.go:339` |
| RF-02 workflow durável HITL | ✅ | `budget_creation_workflow.go:32-59` (`Durable:true`) |
| RF-03 distribuição por diálogo, sem reaproveitar perfil | ✅ | `budgetDistributionPrompt:143-155` (default só ofertado) |
| RF-04 total>0 e soma=10000 p/ ativar | ✅ | `budget_creation_decisions.go:43-59`; `Budget.Activate` |
| RF-05 retroativo sem limite inferior | ✅ | integração `2020-01` ativado `integration_test.go:150-168` |
| RF-06 estado de espera persistido antes da pergunta; resume merge-patch antes do parse | ✅ | `budgetSuspend:61-74`; kernel `engine.go` merge-patch |
| RF-07 confirmação obrigatória; run nunca fica suspended | ✅ | `integration_test.go:236-320` (cancel/TTL/reaper→succeeded/failed) |
| RF-08 "sim" cria+ativa, mês por extenso | ✅ | `executeBudgetCreation:273-307`; `budgetActivatedMessage:317` |
| RF-09 "não"/"cancela" sem efeito | ✅ | `handleBudgetConfirmSlot:249-253`; `countBudgets==0` |
| RF-10 idempotência (run-key resourceId + replay msgID + unicidade) | ✅ | `BudgetCreationKey:28`; `DecideBudgetConfirmation:91`; fora do `WithWriteToolSet` `module.go:234` |
| RF-11/RF-12 unicidade e draft futuro como existente | ✅ | `ErrBudgetConflict:290-294`; `integration_test.go:202-234` |
| RF-13/RF-14/RF-15/RF-16 resolução determinística de mês | ✅ | `month_reference.go:120-142` (`DecideCompetence` puro) |
| RF-17 resolver em query_month/query_plan | ✅ | `competence_reference.go:9-24`; fallback mês corrente preservado |
| RF-18/RF-19 mês por extenso pt-BR; ISO inalterado | ✅ | `FormatCompetencePtBR` `competence.go:79-103`; `String()` ISO |
| RF-20..RF-24 retrospectiva por composição | ✅ | instrução `mecontrola_agent.go:234-241`; gate real-LLM 6/6 |
| RF-25 capacidade só via tool | ✅ | instrução `mecontrola_agent.go:168,245` |
| RF-26 mensagem específica ≠ fallback genérico | ✅ | `executeBudgetCreation:295`; `continuer:79-83`; fallback `consumer:335` |
| RF-27/RF-29 run auditável, cardinalidade controlada | ✅ | `continuer:126-172`; labels `agent_id`/`tool`/`outcome` |
| RF-28 estados fechados state-as-type | ✅ | `budget_creation_state.go:11-94` (String/IsValid/Parse) |
| RF-30 erro persistido auditável | ✅ | `budgetFail`/`StepStatusFailed`+erro; `run.Error` `continuer:153` |
| DoD build/vet/lint/race | ✅ | `go build`, `go vet`, `golangci-lint` (0 issues), `-race` verdes |
| DoD gate real-LLM ≥0.90 | ✅ | todos os gates **ratio=1.0000** (abaixo) |

## 3. Não Conformidades Encontradas

Nenhuma.

## 4. Ressalva Candidata Investigada (descartada)

- **TTL ancorado no Start, não refrescado por turno** (`budget_creation_workflow.go:63-64`). Um subagente sugeriu alinhar ao pending-entry (que refresca). **Descartado como falso positivo:** a PRD SUP-1 nomeia explicitamente o *fluxo de confirmação destrutiva* como contrato a reutilizar, e esse fluxo ancora `SuspendedAt` no tool Start (`delete_entry.go:100`, `update_card.go:137`) sem refrescar em suspend/reprompt (`destructive_confirm_workflow.go:73-124`). Budget-creation reproduz o mesmo padrão (`create_budget.go:142` + `budgetSuspend` preservando a âncora), com TTL de 30 min (vs. 5 min) proporcional ao fluxo multi-turno. Conforme ao contrato nomeado — não é defeito.
- Itens info restantes (deadcode `DecideBudgetPendingResume` já em `deadcode-agent-allowlist.txt:18`; fakes hand-rolled seguindo convenção de `pending_entry_continuer_test.go`; replay benigno) são exceções sancionadas/convenções, não defeitos.

## 5. Evidências de Validação

```
go build ./...                                  → OK
go vet ./internal/agents/... ./internal/budgets/... → OK
golangci-lint run ./internal/agents/... ...     → 0 issues
go test -race -count=1 ./internal/agents/... ./internal/budgets/... → todos ok
gofmt -l (arquivos novos)                        → limpo
```

Gates de governança (todos vazios/OK): zero comentários em produção; sem SQL direto em tools/consumers; sem `switch case intent.Kind`; sem label de alta cardinalidade em métrica (hits eram campo JSON/atributo de span, não label Prometheus).

Integração Postgres (`//go:build integration`, testcontainers) — **6/6 PASS**: retroativo cria+ativa; unicidade não duplica; draft futuro tratado como existente; confirmação negada limpa estado; TTL expirado encerra; reaper encerra órfão.

Gates **real-LLM** (`RUN_REAL_LLM=1`, `openai/gpt-4o-mini`) — todos ≥0.90:

| Gate | Cenários | Ratio |
|---|---|---|
| create_budget routing | criação, retroativo jun/2026, jan/2025, "mês passado"→extenso, mês sem ano→clarifica | **1.0000** (15/15) |
| retrospectiva composição | com orçamento (0%), sem orçamento c/ lançamentos (mostra+oferta) | **1.0000** (6/6) |
| falha persistência → msg específica | run auditável, texto distinto do fallback | **PASS** |
| extração total | 5 formatos BRL | **1.0000** (5/5) |
| extração distribuição | aceita/customiza | **1.0000** (3/3) |
| confirmação sim/não | determinística | **1.0000** (5/5) |

Saídas reais confirmam "junho de 2026"/"maio de 2026" por extenso (RF-18) e RF-23 (oferece criar + mostra realizado).

## 6. Considerações Finais

Nenhuma ressalva, gap ou lacuna identificada. O incidente de produção original está fechado neste caminho: existe tool/fluxo que persiste e ativa o orçamento (fim da promessa quebrada), o mês relativo resolve deterministicamente, a competência é exibida por extenso, e a falha de persistência produz mensagem específica com erro auditável. **APPROVED.** Alteração não commitada.
