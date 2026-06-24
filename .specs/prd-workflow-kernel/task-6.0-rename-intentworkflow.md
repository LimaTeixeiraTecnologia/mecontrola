# Tarefa 6.0: Rename Workflow → IntentWorkflow no agent

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Refactor mecânico que renomeia o atual `workflow.Workflow`/`composite`/`Registry` do `internal/agent`
para `IntentWorkflow`/`intentComposite`/`IntentRegistry`, liberando o nome canônico `Workflow` para o
kernel genérico. **Sem mudança de comportamento** — suíte verde é o gate.

<requirements>
- RF-19: kernel detém o nome canônico `Workflow`; o "Workflow" do agent passa a `IntentWorkflow`.
- Ver ADR-001 (nomenclatura para desambiguação).
</requirements>

## Subtarefas

- [ ] 6.1 Renomear tipos/arquivos em `internal/agent/application/workflow/` (`workflow.go`,
  `composite.go`, `registry.go`) para `IntentWorkflow`/`intentComposite`/`IntentRegistry`, preservando
  contrato e comportamento.
- [ ] 6.2 Atualizar referências em `internal/agent/application/services/{daily_ledger_agent,agent_workflows}.go`
  e quaisquer consumidores.
- [ ] 6.3 Garantir suíte de testes do agent 100% verde após o rename (nenhum teste alterado em semântica).

## Detalhes de Implementação

Ver techspec.md → "Visão Geral dos Componentes / Modificados" e ADR-001. Carregar a skill `mastra`
(mapa Mastra→Go) antes de tocar `internal/agent`. Refactor isolado: não introduzir kernel aqui.

## Critérios de Sucesso

- `IntentWorkflow`/`IntentRegistry` em uso; nome `Workflow` livre para o kernel.
- Comportamento idêntico: testes do agent verdes sem mudança de asserção semântica.
- Gates `R-AGENT-WF-001`/`R-ADAPTER-001` continuam passando.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — alteração estrutural em `internal/agent` (workflow/registry); seguir o mapa Mastra→Go e R-AGENT-WF-001.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agent/application/workflow/{workflow,composite,registry}.go` (rename)
- `internal/agent/application/services/{daily_ledger_agent,agent_workflows}.go` (referências)
