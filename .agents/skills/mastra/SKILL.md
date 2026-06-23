---
name: mastra
version: 3.0.0
category: agent
prerequisites: [agent-governance, go-implementation]
description: "Base canônica para construir e estender agentes, workflows e tools no módulo Go internal/agent do mecontrola. Conceitos Mastra (Agent, Workflow, Tool, Thread, Run, WorkingMemory, Pending Step) mapeados ao código real e às regras R-AGENT-WF-001 e go-implementation. Use sempre que adicionar/alterar um agente, workflow, tool, outcome, kind ou o ciclo Thread→Run no internal/agent."
---

# Mastra no mundo Go — base do internal/agent

Este skill traz os conceitos do framework [Mastra](https://github.com/mastra-ai/mastra) (Agent,
Workflow, Tool, Thread, Run, WorkingMemory, Pending Step) para o módulo Go `internal/agent` do
mecontrola, mapeados ao **código real** e às regras hard `R-AGENT-WF-001`.

O código Go do `internal/agent` já é 100% aderente ao modelo Mastra. Este skill **não** reintroduz
TypeScript: ele é o guia operacional para evoluir o módulo preservando o padrão.

## Princípio inegociável: verifique no código, não na memória

Antes de escrever qualquer código, confirme o estado atual no repositório. As assinaturas, enums e
seams abaixo podem evoluir — sempre abra o arquivo citado e valide antes de copiar.

## Fluxo canônico (R-AGENT-WF-001.1)

```
IntentRouter
  -> AgentRuntime.Execute            (Thread-first → Run auditável)
  -> WorkflowRegistry.Resolve(kind)
  -> Workflow.Execute                (composite; aplica WriteGuard se kind.IsWrite())
  -> Tool.Execute                    (adapter fino)
  -> binding -> usecase -> domain -> repo
```

Regra-mãe: **todo comportamento novo entra como Workflow/Tool reutilizando bindings e usecases —
nunca como novo `case intent.Kind` no switch**. O `daily_ledger_agent.go` permanece fino.

## Mapa Mastra → Go (onde mora cada conceito)

| Mastra            | Go (mecontrola)                              | Arquivo |
| ----------------- | -------------------------------------------- | ------- |
| Agent             | `DailyLedgerAgent` + `AgentRuntime`          | `internal/agent/application/services/{daily_ledger_agent,agent_runtime}.go` |
| Workflow          | `workflow.Workflow` / `composite`            | `internal/agent/application/workflow/{workflow,composite,registry}.go` |
| Tool              | `tools.Tool` (adapter fino)                  | `internal/agent/application/tools/tool.go` |
| Thread            | `entities.Thread` + `ThreadGateway`          | `internal/agent/domain/entities/thread.go`, `application/services/agent_runtime.go` |
| Run               | `entities.Run` + `RunGateway`                | `internal/agent/domain/entities/run.go` |
| WorkingMemory     | `entities.WorkingMemory` + `ContextBuilder`  | `internal/agent/domain/entities/working_memory.go`, `application/prompting/context_builder.go` |
| Pending Step      | `pendingexpense.Draft` + resume              | `internal/agent/domain/pendingexpense/draft.go` |
| Model router (TS) | OpenRouter + FallbackChain (env-driven)      | `internal/agent/infrastructure/providers/openrouter/`, `application/services/fallback_chain.go` |

## Onde colar código novo: o seam

A função `buildRegistry()` em
`internal/agent/application/services/agent_workflows.go` é **o único ponto** onde workflows e tools
são montados. Toda extensão passa por ela. Ver `references/add-workflow-tool.md`.

## Referências (carregue só o necessário — economia de contexto)

| Pergunta / tarefa                                   | Referência |
| --------------------------------------------------- | ---------- |
| Quero entender os primitivos e onde cada um mora    | [`references/core-concepts.md`](references/core-concepts.md) |
| Quero adicionar um novo workflow/tool (read ou write) | [`references/add-workflow-tool.md`](references/add-workflow-tool.md) |
| Preciso de novo `ToolOutcome`/`Kind`/`RunStatus`    | [`references/state-as-type.md`](references/state-as-type.md) |
| Estou mexendo em authz/replay/policy/audit de escrita | [`references/write-guard.md`](references/write-guard.md) |
| Estou no ciclo Thread→Run, métricas ou gateways     | [`references/thread-run-runtime.md`](references/thread-run-runtime.md) |
| Preciso salvar/retomar estado de clarificação       | [`references/pending-step.md`](references/pending-step.md) |
| Vou tocar LLM / system prompt / working memory      | [`references/parse-llm-boundary.md`](references/parse-llm-boundary.md) |
| Vou validar a mudança antes do merge                | [`references/rules-checklist.md`](references/rules-checklist.md) |

Matriz determinística de carregamento por tarefa: [`references/INDEX.yaml`](references/INDEX.yaml).

## Contrato hard e precedência

- Regra hard do módulo: `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001.1–001.8).
- Adaptadores finos e zero comentários em Go: `.claude/rules/go-adapters.md` (R-ADAPTER-001).
- DMMF state-as-type prevalece sobre estilo genérico: `.claude/rules/governance.md`.
- Implementação Go: skill `go-implementation` é **obrigatória** antes de qualquer edição `.go`
  (Etapas 1–5 + checklist R0–R7).

## Passo a passo (sempre)

1. Carregar `go-implementation` (e `agent-governance` se a mudança for transversal).
2. Ler `references/core-concepts.md` para situar o conceito Mastra no código Go.
3. Carregar a referência da tarefa via `INDEX.yaml` (máx. 4 simultâneas).
4. Implementar no seam (`buildRegistry`), mantendo Tool fina (sem regra/SQL/branching de domínio).
5. Validar com `references/rules-checklist.md` e reportar evidência.
