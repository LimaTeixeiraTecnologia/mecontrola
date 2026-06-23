# Fronteira de LLM — parse, system prompt e exceções sancionadas

R-AGENT-WF-001.4: o LLM aparece no step de parse a montante (`ParseInbound`). Workflows e tools de
**execução determinística** operam sobre `intent.Intent` já parseado e não chamam LLM.

## ParseInbound (o ponto canônico)

`application/usecases/parse_inbound.go`:

1. `sanitizer.Clean(text)`.
2. `RenderSystem()` + `RenderUser(text)` — system prompt inclui persona + budgets + working memory.
3. `interpreter.Interpret(ctx, LLMRequest{SystemPrompt, UserMessage, JSONSchema})` — **a chamada LLM**.
4. Decodifica para `intent.Intent` tipado; `maybeRetry` re-tenta em `KindUnknown` com cara de comando.

`confidence` vem no schema do parse e alimenta a `Policy` da guarda de escrita.

## WorkingMemory no system prompt (R-AGENT-WF-001.8)

`ContextBuilder` (`application/prompting/context_builder.go`) monta
`SystemPrompt = persona + budgets + working_memory` via `RenderWorkingMemorySystem`. Working memory
é escopo `resource` (por `user_id`), markdown, compartilhada entre canais. Usuário novo sem working
memory **não** é erro — o prompt é renderizado sem a seção.

## Exceções sancionadas (únicas chamadas de LLM fora de ParseInbound)

Resposta conversacional e onboarding precisam inerentemente de LLM e são as **únicas** call-sites
permitidas fora do parse. Não são violação; são o design:

1. **Conversational fallback** — `KindUnknown` → `delegateFallback` → `fallback.Reply` gera a
   resposta livre. É o escape-hatch; nenhuma execução de domínio depende dele.
2. **Onboarding** — chain de LLM dedicado (modelo próprio por decisão de projeto, ver memória do
   projeto). Separado do chain principal de propósito.

**Proibido**, fora dessas duas: invocar LLM/prompt/fallback chain dentro de uma tool ou workflow de
execução determinística (escrita ou leitura de domínio). Se uma feature nova precisa de LLM no meio
da execução, repense — provavelmente o LLM pertence ao parse, ou a feature é uma nova variante
conversacional/onboarding.
