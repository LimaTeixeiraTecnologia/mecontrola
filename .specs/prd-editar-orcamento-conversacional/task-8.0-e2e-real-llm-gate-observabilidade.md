# Tarefa 8.0: Testes E2E real-LLM + golden + gate + observabilidade

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Validar o fluxo ponta a ponta com LLM real e fechar o gate de release ≥0,90 por categoria, além de garantir a observabilidade (métrica de cardinalidade controlada + Run auditável) e a ausência de falso-sucesso.

<requirements>
- `budget_edit_e2e_real_llm_test.go` (`//go:build integration`, `RUN_REAL_LLM=1`): categorias de cenário — roteamento da operação (EditTotal vs AdjustCategory vs Redistribute), extração de valores (total BRL; categoria+%; distribuição confirm/percent/reais), clarificação de mês (sem ano / irreconhecível), offer_create (não existe / auto-draft vazio), confirmação (sim/não/ambíguo), no-false-success (planner indisponível). Pass ratio ≥ 0,90 por categoria (RF-40).
- Golden cases `application/golden/cases_budget_edit.go` cobrindo as categorias acima (invariante semântico, drive-until-state; sem brittleness). Incluir categoria de **pedido combinado/desambiguação** (RF-03): mensagem com duas mudanças (ex.: total + categoria) deve resultar no agente conduzindo UMA operação por vez (uma tool `edit_budget` por operação), fechando o gate determinístico de RF-03 em vez de deixá-lo apenas como instrução de prompt.
- Métrica `agents_budget_edit_total{operation,outcome}` verificada; labels de cardinalidade controlada (sem `user_id`/`category_id` — RF-38).
- Assert de no-false-success: escrita que falha resulta em `StepStatusFailed` e nenhum recurso persistido (RF-35).
- Assert de alertas (RF-36): edição não dispara reavaliação imediata; recomputo fica a cargo do job existente (verificar ausência de acoplamento novo).
- Rodar `RUN_REAL_LLM=1` com `.env` (OPENROUTER_*), conforme política de validação real-LLM obrigatória.
</requirements>

## Subtarefas
- [ ] 8.1 Golden `cases_budget_edit.go`.
- [ ] 8.2 `budget_edit_e2e_real_llm_test.go` com gate ≥0,90/categoria.
- [ ] 8.3 Asserts de observabilidade (métrica/Run auditável) e no-false-success.
- [ ] 8.4 Execução real-LLM e evidência dos ratios.

## Detalhes de Implementação
Ver techspec.md "Abordagem de Testes"/"Monitoramento" + ADR-005. Moldes: `budget_creation_e2e_real_llm_test.go`, `budget_creation_workflow_real_llm_test.go`, `golden/cases_budget.go`.

## Critérios de Sucesso
- Gate real-LLM ≥ 0,90 por categoria (evidência anexada); zero falso-sucesso comprovado.
- Métrica e Run auditável emitidos com cardinalidade controlada.
- `build`/`vet`/`test -race`/`lint` verdes; suíte de integração verde ao vivo.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `mastra` — evals/scorers e gate real-LLM do fluxo de agente, golden por categoria e asserts de observabilidade do Run.

## Testes da Tarefa
- [ ] Testes unitários (golden determinístico de forma/roteamento)
- [ ] Testes de integração (real-LLM `RUN_REAL_LLM=1`, gate ≥0,90/categoria)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/agents/budget_edit_e2e_real_llm_test.go` (novo)
- `internal/agents/application/workflows/budget_edit_workflow_real_llm_test.go` (novo)
- `internal/agents/application/golden/cases_budget_edit.go` (novo)
