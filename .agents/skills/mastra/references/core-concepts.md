# Core Concepts — Substrato e Consumidor Real

Use esta referência para escolher onde uma mudança agentiva deve entrar. O código atual tem duas camadas:

- `internal/platform/{agent,llm,memory,workflow,tool,scorer}`: substrato genérico reutilizável.
- `internal/agents`: consumidor financeiro real `mecontrola`, integrado ao WhatsApp, módulos financeiros, onboarding, confirmação destrutiva, memory, tools e scorers.

## Layering

```text
internal/agents
  -> internal/platform/agent
     -> internal/platform/tool
     -> internal/platform/memory
     -> internal/platform/scorer
     -> internal/platform/llm
     -> internal/platform/workflow
```

`internal/platform/workflow` é kernel genérico. Ele não importa `internal/agents`, módulos de domínio, `platform/agent`, `platform/memory` ou LLM. Consumidores montam `Engine[S]` com o próprio estado.

## Agent

- Plataforma: `agent.Agent`, `agent.AgentRuntime`, `agent.AgentRegistry`, `agent.RunStore`.
- Consumidor: `BuildMeControlaAgent(provider, tools, hooks, o11y)`.
- Runtime: resolve Thread, abre Run auditável, injeta WorkingMemory no system prompt, recupera `Recent(20)`, executa agente, persiste mensagens e fecha Run.
- Hook real: `NewScoringHooks` observa input, tool calls e output de forma assíncrona.

## Tool

- Plataforma: `tool.NewTool[I,O](id, desc, inSchema, outSchema, exec)`.
- Consumidor: arquivos em `internal/agents/application/tools`.
- Regra: tool é adapter fino. O `exec` pode ler `agent.InboundIdentityFromContext`, validar IDs, delegar para use case/interface e traduzir resultado. Não faça SQL, regra financeira complexa ou chamada direta a módulo externo dentro da tool.
- Escritas financeiras usam idempotência e devem estar em `agent.WithWriteToolSet`.

## Workflow

- Plataforma: `workflow.Engine[S]`, `Definition[S]`, `Step[S]`, `Sequence`, `Branch`, `Parallel`, `Retry`, `MergePatch`.
- Consumidor real:
  - `BuildOnboardingWorkflow`: coleta objetivo, renda, cartões, distribuição e grava WorkingMemory.
  - `BuildDestructiveConfirmWorkflow`: suspende operações destrutivas ou sensíveis e retoma com confirmação.
- Use workflow quando o fluxo é definido, multi-step, retomável ou precisa de suspensão. Use agent quando a decisão de ferramenta é aberta.

## Memory e Recall

- `ThreadGateway.GetOrCreate(resourceID, threadID)`: identidade opaca.
- `MessageStore.Append/Recent`: histórico por thread.
- `WorkingMemory.Get/Upsert`: memória persistida por resource; o runtime injeta no prompt.
- `SemanticRecall.Index/Recall`: pgvector via embeddings; o `PublishingMessageStore` publica evento para indexação assíncrona quando há outbox.

## Scorer / Evals

- Plataforma: `Scorer`, `ScorerRunner`, `ScorerEntry`, sampling e `ResultStore`.
- Consumidor real: `BuildMeControlaScorers(provider)` registra tool-call accuracy financeiro, completeness por keywords e categorization LLM-judged.
- Scoring é best-effort e assíncrono; não deve bloquear o caminho principal.

## LLM e Structured Output

- `llm.Provider` expõe `Complete`, `Stream` e `Embed`.
- OpenRouter é o provider oficial local.
- Structured output usa `llm.Schema`, `llm.StructuredContract[T]` e `agent.NewDecoder[T]`.
- LLM só aparece nas call-sites sancionadas: agent loop, step consumidor que chama `Stream`/`Complete`, embeddings/indexer e scorer LLM-judged. Nunca no kernel.

## Fonte de Verdade

Verifique assinaturas no código antes de implementar. Esta referência orienta o mapa; o working tree decide nomes, construtores e campos atuais.
