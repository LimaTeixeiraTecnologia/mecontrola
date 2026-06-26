# Tarefa 3.0: Runtime deriva do catálogo + remoção de workflowFor/toolFor

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Injetar o `*capability.Catalog` no `AgentRuntime` (via wiring em `module.go`) e fazer `Execute` derivar os labels `workflow`/`tool` de `catalog.Classify(result.Kind)`, **removendo de vez** `workflowFor`/`toolFor` (D-05). Adicionar o teste de equivalência por kind que prova labels idênticos aos legados **exceto** os 4 kinds de drift corrigido (D-02/ADR-002).

<requirements>
- RF-07: `AgentRuntime.Execute` deriva `workflow`/`tool` de `catalog.Classify`, não de switch manual.
- RF-08: `workflowFor`/`toolFor` removidos (D-05); sem superfície redundante.
- RF-09: labels idênticos ao legado para todos os kinds, exceto os 4 de drift documentado.
- RF-13: cardinalidade de métricas mantida — labels só `agent_id`/`channel`/`workflow`/`status`/`tool`/`outcome`.
- RF-17: teste de equivalência por kind (catálogo × legado) com lista explícita de exceções.
</requirements>

## Subtarefas

- [ ] 3.1 `module.go` (wiring do agent): construir `BuildCatalog()` e injetar `*capability.Catalog` no `NewAgentRuntime`.
- [ ] 3.2 `agent_runtime.go`: adicionar campo `catalog` ao `AgentRuntime`; trocar linhas 80-81 por `workflow, tool := rt.catalog.Classify(result.Kind)`.
- [ ] 3.3 Remover as funções `workflowFor` e `toolFor` (e as constantes que ficarem órfãs, se houver).
- [ ] 3.4 Teste de equivalência por kind: tabela `kind → (workflowLegado, toolLegado)`; assert idêntico salvo as 4 exceções (`QueryIncomeSummary`→`transactions`, `BudgetRecurrence`→`budget`, `Delete/EditTransactionByRef`→destrutivo), cujos valores corretos são afirmados.
- [ ] 3.5 Suite `AgentRuntime` (testify/suite whitebox, `fake.NewProvider()`, mocks por IIFE) validando `Run.Resolve`/`recordMetrics` com labels do catálogo, incluindo um kind de drift.

## Detalhes de Implementação

Ver `techspec.md` → "Interfaces Chave" (`Execute` — derivação), ADR-002 (correção de drift + comunicação de impacto) e "Monitoramento e Observabilidade" (séries afetadas). Conformidade R-TESTING-001 (suite whitebox), R-AGENT-WF-001.5 (cardinalidade), R-ADAPTER-001.1 (zero comentários). **Comunicar no PR** o impacto esperado nas séries Prometheus (queda em `workflow="conversational"`, alta em `transactions`/`budget`, surgimento de `tool="query_income_summary"`).

## Critérios de Sucesso

- `Execute` usa exclusivamente o catálogo; `workflowFor`/`toolFor` não existem mais no código.
- Teste de equivalência por kind verde com exatamente 4 exceções declaradas.
- Suite de `AgentRuntime` verde; nenhum novo label de alta cardinalidade.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — altera o roteamento/observabilidade de runtime do `internal/agent` (workflow/tool/outcome por kind); gatilho da skill acionado.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agent/application/services/agent_runtime.go` (modificado: deriva do catálogo; remove `workflowFor`/`toolFor`)
- `internal/agent/.../module.go` (wiring: constrói e injeta o catálogo)
- `internal/agent/application/services/agent_runtime_test.go` (suite + equivalência por kind)
