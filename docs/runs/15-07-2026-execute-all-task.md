# Execução Completa — PRD `prd-recorrencia-orcamento-onboarding`

Data: 2026-07-15/16
Fonte única: `.specs/prd-recorrencia-orcamento-onboarding/` (prd.md, techspec.md, 4 ADRs, tasks.md, 6 task-*.md)
Skill de orquestração: `execute-all-tasks` (subagent fresh por tarefa via `task-executor`, sequencial obrigatório)

## Snapshot Inicial vs Final

| Métrica | Inicial | Final |
|---|---|---|
| Total de tarefas | 6 | 6 |
| `pending` | 6 | 0 |
| `done` | 0 | 6 |
| Drift PRD↔tasks.md (`ai-spec check-spec-drift`) | — | `OK: sem drift detectado` |

## Ordem de Execução (sequencial, sem paralelismo)

Todas as tarefas tocam `internal/agents/application/workflows/onboarding_workflow.go` (arquivo único) — `Paralelizável` era `Não`/`—` em todas as linhas de `tasks.md`, execução 100% sequencial conforme obrigatório.

| # | Tarefa | Status final | Requisitos cobertos | Relatório |
|---|---|---|---|---|
| 1.0 | Decisão pura `DecideRecurrence` e tipos-estado fechados | done | RF-01,02,03,04,06,07,08,17 | `.specs/prd-recorrencia-orcamento-onboarding/1.0_execution_report.md` |
| 2.0 | Schema dedicado, prompt e copy no Tom de Voz | done | RF-04,05,07,08,14,15 | `.specs/prd-recorrencia-orcamento-onboarding/2.0_execution_report.md` |
| 3.0 | Estado com meses e resumo retrocompatível | done | RF-11,13,20 | `.specs/prd-recorrencia-orcamento-onboarding/3.0_execution_report.md` |
| 4.0 | Counter de outcome do step de recorrência | done | RF-16 | `.specs/prd-recorrencia-orcamento-onboarding/4.0_execution_report.md` |
| 5.0 | Reescrita do `BuildRecurrenceStep` e confirmação encadeada | done | RF-01,02,03,07,08,09,10,12,18,19 | `.specs/prd-recorrencia-orcamento-onboarding/5.0_execution_report.md` |
| 6.0 | Gate real-LLM com 0 falso-sucesso e fixture full-flow | done | RF-17,18 | `.specs/prd-recorrencia-orcamento-onboarding/6.0_execution_report.md` |

Cobertura de requisitos: 100% dos RFs do PRD (RF-01 a RF-20) mapeados em pelo menos uma tarefa `done`.

## Resumo por Tarefa

**1.0 — DecideRecurrence puro + tipos fechados.** Adicionados `recurrenceIntentKind`, `recurrenceOutcomeKind` (state-as-type) e a função pura `DecideRecurrence`, espelhando o padrão já usado por `DecideMonthlyBudgetCents`/`distributionIntentKind`. Testes tabela cobrindo enum round-trip e todas as precedências de RF-06/07/08. Build/vet/lint limpos; `internal/agents/... -race` com 1117 testes, sem regressão. Review adversarial independente: APPROVED, 0 findings.

**2.0 — Schema, prompt e copy.** Criados `recurrenceDecisionSchema`/`recurrenceExtract`/`recurrenceDecisionSystemPrompt` **novos**, sem tocar `recurrenceSchema`/`yesNoExtract`/`summaryConfirmSystemPrompt` existentes (risco de integração do tasks.md respeitado). Uma regressão crítica foi introduzida na primeira passagem (mutação in-place do prompt legado) e **corrigida antes da conclusão** por revisão adversarial dedicada — o prompt original de `summary_confirm` foi restaurado intacto e o conteúdo novo isolado na constante dedicada.

**3.0 — Estado com meses + resumo retrocompatível.** `OnboardingState` ganhou `RecurrenceMonths`/`RecurrenceConfirmation`; `recurrenceSummaryLine` passou a refletir N meses reais com fallback legado de 12 para estado pré-migração. Build/vet/test-race/lint verdes.

**4.0 — Counter de observabilidade.** `agents_onboarding_recurrence_total` com label `outcome` fechado (cardinalidade controlada, sem `user_id`/`category_id`) e `recordRecurrenceOutcome` (guard nil). Deixado propositalmente sem uso nesta tarefa (wiring é escopo de 5.0) — confirmado via `unusedfunc` esperado.

**5.0 — Reescrita do step (integração).** `BuildRecurrenceStep` reescrito para consumir os 4 blocos anteriores: extração única via `recurrenceDecisionSchema`, `DecideRecurrence` para a decisão pura, dispatch exaustivo por `recurrenceOutcomeKind`, `recordRecurrenceOutcome` wireado, confirmação encadeada preservando uma única chamada `agent.Execute`/turno. Os 8 pontos de teste afetados (prompt inicial, mock 12, ambíguo, resumo, surface map, assinatura, fixture full-flow, assinatura gate) foram localizados por conteúdo (não por linha, que havia mudado) e atualizados. Suíte completa `internal/agents/...` sem `FAIL`.

**6.0 — Gate real-LLM (0 falso-sucesso).** `TestRecurrenceExtractionGate` estendido para 18 subcenários (negativa ×4, positiva-12 ×3, N numérico ×3, N por extenso ×2, inválido ×3, ambíguo ×3), executado com `RUN_REAL_LLM=1` contra `openai/gpt-4o-mini` real (sem mock de LLM): **hits=18 total=18 ratio=1.0000 falso_sucesso=0**. Zero-falso-sucesso comprovado duplamente — assert explícito E estruturalmente (mock `BudgetPlanner` sem `EXPECT(CreateRecurrence)` nos cenários que não devem aplicar recorrência, o que faria o mockery falhar em qualquer chamada inesperada). Fixture full-flow WhatsApp (`whatsapp_inbound_consumer_integration_test.go`) migrada para o novo schema `{intent,hasMonths,months}`, testcontainer Postgres real, PASS.

## Validação Final (pós-integração, nível de repositório)

- `go build ./...` → sem erros
- `go vet ./...` → sem erros
- `go test ./internal/agents/... -race` → **1134 testes passando em 21 pacotes**, 0 falha
- `ai-spec check-spec-drift .specs/prd-recorrencia-orcamento-onboarding/tasks.md` → `OK: sem drift detectado`
- Todos os 6 `*_execution_report.md` presentes e não-vazios (evidência física validada)
- `tasks.md`: todas as 6 linhas com status `done`

## Riscos Residuais Registrados

- Gate real-LLM (`RUN_REAL_LLM=1`, build tag `integration`) depende de `OPENROUTER_API_KEY` e execução manual/curada — não roda no CI padrão (consistente com ADR-004; recomendado reexecutar antes de merge que toque o step de recorrência).
- Flutuação de modelo LLM pode reduzir o ratio abaixo de 0,90 em execuções futuras — mitigação documentada no ADR-004 (ajustar exemplos do prompt antes de relaxar o threshold).
- Nenhum risco crítico aberto. Nenhuma pendência, lacuna ou ressalva conhecida no escopo deste PRD.

## Critérios de Aceite (conferência final)

- [x] 100% de conformidade com o PRD (RF-01..RF-20 cobertos por tarefas `done`)
- [x] 0 desvios (nenhuma tarefa alterou escopo além do especificado)
- [x] 0 lacunas (`ai-spec check-spec-drift` OK; 6/6 tarefas `done`)
- [x] 0 falso positivo (gate real-LLM comprova 0 falso-sucesso; suíte completa verde; build/vet/lint limpos)
- [x] 0 pendências (nenhuma tarefa `pending`/`blocked`/`needs_input`/`failed`)
- [x] 0 ressalvas (reviews adversariais: APPROVED em todas as tarefas revisadas)
- [x] 0 flexibilizações (regras hard do projeto — zero comentários, `Decide*` puro, tipos fechados, cardinalidade controlada — respeitadas em todas as tarefas)

## Status Final

**done** — PRD `prd-recorrencia-orcamento-onboarding` implementado integralmente, 6/6 tarefas, sem divergência entre implementação e especificação. Trabalho **não commitado** (working tree) — aguardando decisão do usuário sobre commit/PR.
