# Tarefa 9.0: Gate de aceite — golden real-LLM ≥0,90/cat + consistência transação↔orçamento + gates de governança

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechar a feature com o gate de aceite: suíte golden real-LLM cobrindo os cenários de edição, verificação de consistência transação↔orçamento e execução de todos os gates de governança e da matriz de validação por risco.

<requirements>
- RF-32: Run auditável e métricas verificados (cardinalidade controlada).
- RNF-01: LLM só nas call-sites sancionadas; OpenRouter único provider.
- RNF-02: adapters finos, zero comentários, estados fechados, regra em `Decide*`, DTO `Validate()`.
- RNF-03: PT-BR e formatação WhatsApp.
- RNF-04: sem regressão em criar/consultar/excluir e nos consumidores existentes.
- RNF-05: sem novo LLM inline fora das call-sites sancionadas; latência dentro do orçamento de turno.
- RNF-06: persistência durável do estado + resume por merge-patch, sem side-store.
- D-05: golden real-LLM com razão de acerto ≥ 0,90 por categoria + consistência transação↔orçamento.
</requirements>

## Subtarefas

- [ ] 9.1 Adicionar cenários golden de edição (valor/categoria/data/pagamento/direção/cartão; conflito; no-op; alvo inexistente/excluído) às categorias `expense_income`/`pending`/`confirmation`/`tool_error`.
- [ ] 9.2 Executar a suíte real-LLM (`RUN_REAL_LLM=1`, OpenRouter) e comprovar ratio ≥ 0,90 por categoria; scorer `write_persistence_accuracy` verde.
- [ ] 9.3 Verificação de consistência transação↔orçamento (após editar, `GetMonthlySummary` e thresholds coerentes; sem linha fantasma).
- [ ] 9.4 Rodar os gates grep de governança (R-ADAPTER-001.1/.2, R-AGENT-WF-001, R-TXN-001/003/004, R-WF-KERNEL-001, R-DTO-VALIDATE-001, R-TESTING-001) e a matriz de validação por risco (build/vet/test race/lint + gates de contrato de evento publicado).

## Detalhes de Implementação

Ver `techspec.md` (Abordagem de Testes — E2E; Conformidade com Padrões) e a seção "Governance GATES" da compliance. Dirigir o harness ao estado/invariante semântico sem baixar a régua.

## Critérios de Sucesso

- Golden real-LLM ≥ 0,90 por categoria; consistência transação↔orçamento comprovada.
- Todos os gates de governança verdes; sem regressão.
- Evidências (ratios, saída dos gates) registradas no relatório de execução.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — cenários golden e scorers de agente no substrato (real-LLM harness).

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/golden/` (cases_expense_income.go, cases_pending_confirmation.go, cases_tool_error.go)
- `internal/agents/application/scorers/write_persistence_accuracy.go`
- `internal/budgets/integration/transaction_to_budget_chain_integration_test.go`
