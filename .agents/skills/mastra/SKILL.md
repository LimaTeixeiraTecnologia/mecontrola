---
name: mastra
version: 4.0.0
category: agent
prerequisites: [agent-governance, go-implementation]
description: "Base canônica para construir e estender agentes no mecontrola sobre o substrato Mastra-equivalente em internal/platform/{agent,llm,memory,workflow,tool,scorer} e o consumidor de referência internal/agents (port weather). Conceitos Mastra (Agent, Tool, Workflow/Step, Thread, Run, WorkingMemory, Semantic Recall, Scorer/Evals, Structured Output) mapeados ao código real e às regras R-WF-KERNEL-001, R-AGENT-WF-001, R-ADAPTER-001 e go-implementation. Use sempre que criar um novo agente igual a internal/agents, ou adicionar/alterar tool, workflow, scorer, memória ou o ciclo Thread→Run."
---

# Mastra no mundo Go — substrato `internal/platform` + consumidor `internal/agents`

A capacidade agentiva do mecontrola é um **port de comportamento do [Mastra](https://github.com/mastra-ai/mastra)**
(equivalência funcional, não cópia de API TypeScript). Ela vive em **duas camadas**:

- **Substrato reutilizável** em `internal/platform/{agent,llm,memory,workflow,tool,scorer}` — os
  primitivos genéricos (Agent, Tool, Workflow/Step kernel, Thread/Message/WorkingMemory, Semantic
  Recall, Scorer, LLM Provider), sobre Postgres (`platform_*`) e pgvector.
- **Consumidor de referência** `internal/agents` — port 1:1 do exemplo *weather* do Mastra, validado
  end-to-end no WhatsApp. É o **molde para criar novos agentes**.

Regra de ouro: **consuma o substrato, nunca o reimplemente.** Comportamento novo entra como um novo
agente/tool/workflow/scorer no módulo consumidor, montando os primitivos de `internal/platform`.

## Princípio inegociável: verifique no código, não na memória

Antes de escrever qualquer código, confirme o estado atual no repositório. As assinaturas, enums e
construtores abaixo podem evoluir — sempre abra o arquivo citado e valide antes de copiar.

## Fluxo canônico

Execução de um turno (request → resposta), espelhando `AgentRuntime.Execute`:

```
InboundRequest (resourceId, threadId, agentId, message, messageId)
  -> AgentRuntime.Execute                 (internal/platform/agent/runtime.go)
     -> ThreadGateway.GetOrCreate         (internal/platform/memory — Thread por (resourceId, threadId))
     -> RunStore.Insert(Run{Running})     (internal/platform/agent — Run auditável)
     -> AgentRegistry.Resolve(agentId)    -> Agent
     -> buildMessages                     (system = instructions + WorkingMemory; + Recent(20); + user)
     -> Agent.Execute                     (loop LLM tool-calling, máx. 5 rounds)
        -> llm.Provider.Complete          -> ToolHandle.Invoke -> exec tipado
     -> MessageStore.Append (user, assistant)   [publishing store → outbox → indexer de embedding]
     -> closeRun(Run{Succeeded|Failed})
```

Execução durável multi-step (workflow), espelhando o kernel:

```
workflow.Engine[S].Start/Resume          (internal/platform/workflow/engine.go)
  -> Sequence/Branch/Parallel/Retry de Step[S]   (combinators.go)
  -> Snapshot + StepRecord persistidos (Durable)  (store.go)
  -> suspend (StepStatusSuspended) → Resume aplica MergePatch (RFC 7386) sobre Snapshot.State
```

Regra-mãe: **novo comportamento = novo agente/tool/workflow/scorer no consumidor**, montando os
primitivos. Não há switch de domínio nem roteamento por `intent.Kind`.

## Mapa Mastra → Go (onde mora cada conceito)

| Mastra | Substrato `internal/platform` (tipo · arquivo) | Consumidor `internal/agents` |
| --- | --- | --- |
| Agent / lifecycle / registry | `agent.Agent`, `agent.AgentRuntime`, `agent.AgentRegistry` · `agent/{ports,runtime,agent,registry,types}.go` | `application/agents/agent.go` (`BuildWeatherAgent`) |
| Tool (I/O tipado) | `tool.NewTool[I,O]`, `tool.ToolHandle`, `tool.Registry` · `tool/{tool,registry}.go` | `application/tools/tool.go` (`BuildWeatherTool`) |
| Workflow / Step (kernel) | `workflow.Engine[S]`, `workflow.Step[S]`, `workflow.Definition[S]`, `Sequence/Branch/Parallel/Retry`, `Codec.MergePatch` · `workflow/{engine,step,combinators,codec,store}.go` | `application/workflows/workflow.go` (`BuildWeatherWorkflow`) |
| Thread / Message / WorkingMemory | `memory.ThreadGateway`, `memory.MessageStore`, `memory.WorkingMemory` · `memory/{ports,types}.go` | wiring em `module.go` |
| Semantic recall (RAG / pgvector) | `memory.SemanticRecall` + `llm.Embed` + `infrastructure/indexer` (`EmbeddingIndexHandler`, `NewPublishingMessageStore`) | `module.go` (publishing store + index handler via outbox) |
| Scorer / Evals | `scorer.Scorer`, `scorer.ScorerRunner`, `NewToolCallAccuracyScorer`/`NewCompletenessScorer`/`NewLLMJudgedScorer` · `scorer/{scorer,code_based,llm_judged,runner,types}.go` | `application/scorers/scorers.go`, `application/agents/scoring_hooks.go` |
| Run auditável | `agent.Run`, `agent.RunStore` · `agent/infrastructure/postgres/run_store.go` | — (gerenciado pelo runtime) |
| LLM Provider | `llm.Provider` (`Complete`/`Stream`/`Embed`), `llm.NewOpenRouterProvider` · `llm/{provider,types,openrouter}.go` | `module.go` |
| Structured output | `agent.StructuredDecoder`, `agent.NewDecoder[T]`, `llm.StructuredContract[T]`, `llm.Schema` · `agent/decoder.go` | contrato por agente/tool |
| Runtime context (DI) | `workflow.WithRuntime`/`RuntimeFrom`, `agent.WithRunID`/`RunIDFromContext` · `workflow/runtime.go`, `agent/runid_context.go` | hooks (`scoring_hooks.go`) |

## Padrão de novo agente (molde `internal/agents`)

Criar um agente novo = espelhar a estrutura real do consumidor, montando primitivos do substrato:

```
internal/<seu_agente>/
  domain/                         tipos puros do domínio (sem IO) — ex.: forecast.go, weather_condition.go
  application/
    agents/    agent.go           BuildXAgent: agent.NewAgent(id, instructions, provider, o11y, WithTools(...), WithHooks(...))
    tools/     tool.go            BuildXTool: tool.NewTool[In,Out](id, desc, inSchema, outSchema, exec) — exec é adapter fino
    workflows/ workflow.go        BuildXWorkflow: workflow.Definition[S]{Root: Sequence(...), Durable, MaxAttempts}
    scorers/   scorers.go         BuildXScorers: []scorer.ScorerEntry com sampling (code-based + LLM-judged)
    usecases/  handle_inbound.go  fronteira fina: valida input DTO e delega ao agent.AgentRuntime
    dtos/input/                   InboundInput com Validate() (R-DTO-VALIDATE-001)
  infrastructure/
    <client>/                     clients externos (ex.: weather/client.go)
    messaging/database/consumers/ consumer que recebe o evento e chama o usecase
  module.go                       DI manual: provider → registry → memory → runStore → runtime → handleInbound → consumer
```

Wiring de referência **verbatim**: `internal/agents/module.go` (`NewModule`). Sequência:
`llm.NewOpenRouterProvider` → `scorer.NewScorerRunner` + `NewScoringHooks` → `BuildXTool` →
`BuildXAgent` → `agent.NewAgentRegistry().Register(...)` → repositórios de memória (`memorypostgres.*`)
→ (se houver outbox) `memory.NewPublishingMessageStore` + registrar `indexer.NewEmbeddingIndexHandler`
→ `agentpostgres.NewRunStore` → `agent.NewAgentRuntime(...)` → `usecases.NewHandleInbound(runtime, o11y)`.

Ver `references/build-new-agent.md`.

## Referências (carregue só o necessário — economia de contexto)

| Pergunta / tarefa | Referência |
| --- | --- |
| Entender os primitivos, o layering e onde cada um mora | [`references/core-concepts.md`](references/core-concepts.md) |
| Criar um novo agente igual a `internal/agents` | [`references/build-new-agent.md`](references/build-new-agent.md) |
| Adicionar/alterar um workflow durável (Engine/Step/combinators/suspend-resume) | [`references/workflow-engine.md`](references/workflow-engine.md) |
| Mexer em Thread/Message/WorkingMemory/Semantic Recall/indexer | [`references/memory-recall.md`](references/memory-recall.md) |
| Adicionar/alterar scorers (code-based ou LLM-judged) | [`references/scorer-evals.md`](references/scorer-evals.md) |
| Tocar LLM Provider, structured output ou streaming | [`references/llm-structured-output.md`](references/llm-structured-output.md) |
| Novo enum de estado (`RunStatus`/`StepStatus`/`ToolOutcome`/`ScorerKind`...) | [`references/state-as-type.md`](references/state-as-type.md) |
| Ciclo Thread→Run, métricas, gateways do runtime | [`references/thread-run-runtime.md`](references/thread-run-runtime.md) |
| Validar a mudança antes do merge | [`references/rules-checklist.md`](references/rules-checklist.md) |

Matriz determinística de carregamento por tarefa: [`references/INDEX.yaml`](references/INDEX.yaml).

## Contrato hard e precedência

- **Kernel puro** (`internal/platform/workflow`): `.claude/rules/workflow-kernel.md` (R-WF-KERNEL-001) —
  sem import de domínio nem de camada superior, sem regra/SQL/branching/LLM, estados fechados,
  cardinalidade controlada, resume via merge-patch.
- **Primitivo de agent da plataforma** (`internal/platform/agent`): `.claude/rules/agent-workflows-tools.md`
  (R-AGENT-WF-001, re-escopado pela emenda 2026-06-29) — Thread-first, Run auditável, Tool fina,
  estados fechados, LLM só no ponto sancionado.
- **Adaptadores finos e zero comentários em Go**: `.claude/rules/go-adapters.md` (R-ADAPTER-001).
- **Input DTO com `Validate()`**: `.claude/rules/input-dto-validate.md` (R-DTO-VALIDATE-001).
- **DMMF state-as-type prevalece** sobre estilo genérico: `.claude/rules/governance.md`.
- **Implementação Go**: skill `go-implementation` é **obrigatória** antes de qualquer edição `.go`
  (Etapas 1–5 + checklist R0–R7).

## Passo a passo (sempre)

1. Carregar `go-implementation` (e `agent-governance` se a mudança for transversal).
2. Ler `references/core-concepts.md` para situar o conceito Mastra na camada certa (substrato vs consumidor).
3. Carregar a referência da tarefa via `INDEX.yaml` (máx. 4 simultâneas).
4. Implementar **no consumidor**, montando primitivos do substrato; manter Tool/adapter finos
   (sem regra/SQL/branching de domínio) e o kernel intocado.
5. Validar com `references/rules-checklist.md` e reportar evidência.
