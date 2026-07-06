---
name: mastra
version: 1.0.0
category: agent
prerequisites: [agent-governance, go-implementation]
description: "Build and extend the mecontrola Mastra-equivalent Go agent stack over internal/platform/{agent,llm,memory,workflow,tool,scorer} and the real internal/agents financial consumer. Use for new or changed agents, tools, workflows, memory/recall, scorers/evals, structured output, Thread->Run runtime, WhatsApp inbound, onboarding, destructive confirmation, or idempotent financial writes. Do not use as TypeScript Mastra API guidance or for generic non-agent Go work."
---

# Mastra Go Port — Plataforma Agentiva do MeControla

Use esta skill para trabalhar no port Go de conceitos Mastra do mecontrola. Trate a documentação oficial do Mastra como inspiração conceitual; trate o codebase atual como fonte mandatória.

## Ordem Mandatória

1. Carregue `agent-governance` e `go-implementation` antes de qualquer alteração Go.
2. Verifique o código real antes de copiar qualquer exemplo: `internal/platform/{agent,llm,memory,workflow,tool,scorer}` e `internal/agents`.
3. Carregue `references/core-concepts.md` para localizar o primitivo correto.
4. Carregue `references/INDEX.yaml` e somente as referências do tipo de tarefa, respeitando o limite de contexto do repositório.
5. Implemente comportamento novo no consumidor `internal/agents` ou em outro consumidor explícito; não reimplemente primitivos da plataforma.
6. Valide com `references/rules-checklist.md` e com os gates da skill Go aplicável.

## Regra de Ouro

Consuma o substrato, não o recrie. O substrato reutilizável vive em `internal/platform/{agent,llm,memory,workflow,tool,scorer}`. O consumidor de referência atual é `internal/agents`, o agente financeiro `mecontrola`, não um exemplo histórico externo.

## Fluxo Canônico Thread -> Run

```text
WhatsApp/outro inbound
  -> application/usecases/HandleInbound
  -> agent.AgentRuntime.Execute
     -> memory.ThreadGateway.GetOrCreate(resourceId, threadId)
     -> agent.RunStore.Insert(RunStatusRunning)
     -> agent.AgentRegistry.Resolve(agentId)
     -> buildMessages(system + WorkingMemory + Recent(20) + user)
     -> agent.Agent.Execute
        -> llm.Provider.Complete
        -> tool.ToolHandle.Invoke
     -> memory.MessageStore.Append(user, assistant)
     -> closeRun(RunStatusSucceeded|RunStatusFailed)
```

Distinga defaults da plataforma e configuração do consumidor: `agent.NewAgent` usa `defaultMaxToolRounds = 5`, mas `BuildMeControlaAgent` configura `WithMaxToolRounds(12)`.

## Mapa Real Mastra -> Go

| Conceito Mastra | Substrato | Consumidor real |
| --- | --- | --- |
| Agent | `internal/platform/agent` (`Agent`, `AgentRuntime`, `AgentRegistry`) | `internal/agents/application/agents/mecontrola_agent.go` (`BuildMeControlaAgent`) |
| Tool | `internal/platform/tool` (`NewTool[I,O]`, `ToolHandle`) | `internal/agents/application/tools/*` (`BuildRegisterExpenseTool`, `BuildQueryMonthTool`, etc.) |
| Workflow/Step | `internal/platform/workflow` (`Engine[S]`, `Definition[S]`, `Step[S]`) | `BuildOnboardingWorkflow`, `BuildDestructiveConfirmWorkflow` |
| Memory/Recall | `internal/platform/memory` (`ThreadGateway`, `MessageStore`, `WorkingMemory`, `SemanticRecall`) | `module.go` wiring + onboarding working memory |
| Scorer/Evals | `internal/platform/scorer` (`Scorer`, `ScorerRunner`) | `BuildMeControlaScorers`, `NewScoringHooks` |
| LLM/Structured output | `internal/platform/llm` + `agent.NewDecoder[T]` | prompts, tool schemas, workflow extraction, LLM-judged scorer |
| Runtime context | `agent.WithRunID`, `agent.InboundIdentityFromContext`, `workflow.WithRuntime` | idempotent writes and scoring hooks |

## Padrões Invioláveis

- Use `internal/platform/workflow` apenas como kernel genérico. Não importe domínio, agent, memory, LLM, SQL fora de `infrastructure/postgres`, nem semântica financeira no kernel.
- Modele estados como tipos fechados com zero value inválido quando aplicável: `RunStatus`, `ToolOutcome`, `StepStatus`, `SuspendReason`, `ScorerKind`, `MessageRole` e estados do consumidor.
- Mantenha tools como adapters finos: schema JSON estrito, leitura de contexto quando necessário, delegação para use case/interface, retorno tipado.
- Para escrita financeira idempotente, use `agent.InboundIdentityFromContext`, use cases existentes e registre a tool em `agent.WithWriteToolSet`.
- Preserve onboarding e confirmação destrutiva como workflows duráveis com suspend/resume; não substitua por branching solto no handler ou no prompt.
- Use OpenRouter via `internal/platform/llm`. Não crie fallback chain, provider paralelo ou chamada HTTP direta para LLM.
- Nunca copie exemplos TypeScript Mastra para Go. Traduza conceitos para os primitivos reais do repositório.

## Referências por Tarefa

| Tarefa | Leia |
| --- | --- |
| Entender arquitetura, layering e símbolos reais | `references/core-concepts.md` |
| Criar/alterar consumidor agentivo no padrão `internal/agents` | `references/build-new-agent.md` |
| Adicionar/alterar tool financeira | `references/build-new-agent.md`, `references/llm-structured-output.md` |
| Alterar onboarding ou confirmação destrutiva | `references/workflow-engine.md`, `references/state-as-type.md` |
| Alterar Thread, Message, WorkingMemory, recall ou indexer | `references/memory-recall.md`, `references/thread-run-runtime.md` |
| Alterar scorers, hooks ou evals | `references/scorer-evals.md` |
| Alterar LLM, streaming ou structured output | `references/llm-structured-output.md` |
| Adicionar enum/estado fechado | `references/state-as-type.md` |
| Validar antes de concluir | `references/rules-checklist.md` |

## Inspiração Mastra Oficial

Use os conceitos oficiais apenas para decidir o primitivo correto: Agent para tarefas abertas que decidem ferramentas; Workflow para fluxo definido e retomável; Tool para capacidade externa com schema; Memory para continuidade; Scorer/Evals para avaliação assíncrona; Skill para instruções carregáveis. A API copiável neste repositório é sempre a API Go local.
