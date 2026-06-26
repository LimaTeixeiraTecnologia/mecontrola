# Tarefa 5.0: Skill mastra + checklist de extensão (5 seams)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Atualizar a skill `mastra` (`.agents/skills/mastra/SKILL.md` + `references/`) para deixar de afirmar que `buildRegistry()` é o único seam e declarar os **5 seams reais** de evolução do agente, mais um **checklist de extensão** objetivo cobrindo os 6 pontos — incluindo o passo obrigatório de registrar a `CapabilitySpec` no catálogo.

<requirements>
- RF-14: skill `mastra` declara os 5 seams reais: (1) registry, (2) kernel write path, (3) confirmation engine, (4) plan executor, (5) resume chain ordering.
- RF-15: checklist objetivo de extensão para: novo kind, nova tool, novo workflow, novo pending state, novo gate de confirmação, novo plan step — cada um incluindo "registrar `CapabilitySpec`".
</requirements>

## Subtarefas

- [ ] 5.1 Localizar e corrigir a(s) afirmação(ões) de "buildRegistry é o único seam" na skill e referências.
- [ ] 5.2 Documentar os 5 seams com ponteiros ao código real: registry (`buildRegistry`/`IntentRegistry`), kernel write (`buildKernelDefinition`/`NewTransactionsWriteDefinition`), confirmation (`NewDestructiveConfirmDefinition`), plan executor (`plan_executor.go`), resume chain (`continuePendingExpenseConfirmation → continuePendingPlan → continuePendingApproval`).
- [ ] 5.3 Adicionar checklist de extensão (6 pontos) com o passo "registrar `CapabilitySpec` no catálogo" em cada caminho aplicável.
- [ ] 5.4 Referenciar o catálogo canônico como fonte única de classificação/auditoria.

## Detalhes de Implementação

Ver `techspec.md` → "Conformidade com Padrões" e o roadmap `docs/plans/2026-06-25-mastra-gap-map-mecontrola.md` (Fase 1, seção "skill mastra"). Esta tarefa é de documentação (sem código Go); pode rodar em paralelo a 3.0 (arquivos disjuntos). Não alterar comportamento de runtime.

## Critérios de Sucesso

- A skill não contém mais a afirmação de seam único.
- Os 5 seams estão descritos com ponteiros verificáveis ao código.
- O checklist cobre os 6 pontos e cita o registro de `CapabilitySpec`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — a tarefa atualiza a própria skill `mastra` e seu modelo de seams/extensão do `internal/agent`.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `.agents/skills/mastra/SKILL.md` (modificado)
- `.agents/skills/mastra/references/*` (modificado conforme necessário)
- Referência: `internal/agent/application/services/{agent_workflows.go,daily_ledger_agent.go}`, `internal/agent/application/workflow/{plan_executor.go,destructive_confirm.go}`
