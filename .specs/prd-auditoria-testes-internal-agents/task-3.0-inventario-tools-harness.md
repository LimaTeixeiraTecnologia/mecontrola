# Tarefa 3.0: Sincronizar inventário real de tools e harness de cobertura

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Eliminar drift estrutural entre `buildFinancialTools`, `module_test.go`, scorers e harnesses de cobertura, tornando `tool.ID()` derivado da composição real a única fonte de verdade. A tarefa mexe diretamente em tools, scorers e evals do consumidor financeiro real; por isso exige a skill processual `mastra`.

<requirements>
- Cobrir RF-06, RF-07, RF-08 e RF-12.
- Derivar `actualIDs` a partir de `buildFinancialTools(...)` com doubles mínimos das dependências.
- Criar/ajustar matriz `coverageByTool` com igualdade exata de conjuntos contra os IDs reais.
- Separar cobertura obrigatória por inventário de cenários extras de roteamento.
- Corrigir listas/scorers auxiliares que hoje mantêm inventário manual divergente.
- Preservar a suite complementar `realllm`, mas sem duplicar fonte de verdade do inventário.
</requirements>

## Subtarefas

- [ ] 3.1 Criar helper de inventário real a partir de `buildFinancialTools(...)`.
- [ ] 3.2 Ajustar `module_test.go`, `mecontrola_scorers.go` e/ou `mecontrola_tools_realllm_test.go` para usar ou validar contra os IDs reais.
- [ ] 3.3 Separar `coverageByTool` de `routingScenarios`, com falha explícita para tool sem cenário.

## Detalhes de Implementação

Consultar `techspec.md`, especialmente:
- `## Arquitetura do Sistema` para a fonte única de verdade em `buildFinancialTools`.
- `## Modelos de Dados` para a regra de igualdade exata entre `tool.ID()` reais e chaves da matriz obrigatória.
- `ADR-002` para a decisão de inventário real de tools.

## Critérios de Sucesso

- O inventário real de tools é derivado da composição do módulo e não de listas manuais.
- A suite falha automaticamente quando surgir nova tool sem cenário correspondente.
- Cenários extras de roteamento não mascaram cobertura incompleta do inventário.
- O runtime complementar continua útil, mas deixa de ser a fonte primária do conjunto de tools.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — a tarefa altera tools, scorers/evals e o consumidor `internal/agents` sobre o stack agentivo real do projeto.

## Testes da Tarefa

- [ ] `go test -race -count=1 ./internal/agents/...`
- [ ] `go test -race -count=1 ./internal/agents/application/scorers/...`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/module.go`
- `internal/agents/module_test.go`
- `internal/agents/application/scorers/mecontrola_scorers.go`
- `internal/agents/application/scorers/mecontrola_tools_realllm_test.go`
