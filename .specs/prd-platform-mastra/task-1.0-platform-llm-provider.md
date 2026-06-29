# Tarefa 1.0: Provider OpenRouter genérico em `internal/platform/llm`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar `internal/platform/llm`: provider OpenRouter genérico, relocando e generalizando o client de `internal/agent/.../openrouter` (módulo a ser apagado). Expõe `Complete`, `Stream` (SSE) e `Embed`, com structured output injetável pelo chamador (sem `intentJSONSchema` de domínio). É a base de tudo que usa LLM/embeddings (5.0, 6.0, 7.0).

<requirements>
- RF-04: comunicação com LLM exclusivamente via OpenRouter.
- RF-05: structured output declarável (schema injetável, `strict`).
- RF-06: contrato de saída estruturada validável na fronteira (decode + validação), falha explícita.
- Reusar `internal/platform/httpclient` e devkit-go `observability`.
- Embeddings: `openai/text-embedding-3-small` (`vector(1536)`), configurável por env.
- Limites default: `RequestTimeout=30s`, retry `MaxAttempts=3` backoff `200ms→10s` (como decorator/step, não no kernel).
- Sem semântica de domínio; sem comentários em Go; tipos fechados onde houver estado.
</requirements>

## Subtarefas

- [ ] 1.1 Definir `Provider` (`Complete`, `Stream`, `Embed`), `Request`, `Response`, `TokenStream`, `Schema`, `StructuredContract[T]` (ver techspec "Interfaces Chave").
- [ ] 1.2 Implementar `Complete` (chat/completions, `response_format: json_schema` injetável) e classificação de erro/upstream (`ErrProviderUpstream`, status classes).
- [ ] 1.3 Implementar `Stream` consumindo SSE (`stream: true`); acumular deltas; detectar `finish_reason: length`/erro.
- [ ] 1.4 Implementar `Embed` (endpoint de embeddings OpenRouter) retornando `[][]float32`.
- [ ] 1.5 Métricas relocadas (`agent_llm_provider_call_total`, `_errors_total`, `_tokens_total`, `_latency_seconds`) e spans (`llm.complete`, `llm.stream`, `llm.embed`).

## Detalhes de Implementação

Ver techspec.md seções "Design de Implementação > Interfaces Chave", "Pontos de Integração > OpenRouter" e ADR-003 (streaming × structured output). Não duplicar; adaptar o client existente `internal/agent/infrastructure/providers/openrouter/client.go` removendo qualquer acoplamento de domínio.

## Critérios de Sucesso

- `internal/platform/llm` compila e não importa nenhum pacote de domínio nem `internal/agent`.
- `Complete`/`Stream`/`Embed` cobertos por unit; structured output decode conforme/não-conforme testado.
- Zero comentários em Go de produção; `go build ./...` e lint verdes.
- Gate: `grep -rn "intentJSONSchema\|internal/agent\|internal/transactions\|internal/billing\|internal/identity" internal/platform/llm/ --include="*.go"` retorna vazio.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `go-implementation` — implementação Go obrigatória e inegociável (CLAUDE.md) para o provider, tipos e testes.
- `mastra` — provider LLM é a base do ciclo agent/tool/structured output do padrão Mastra portado.

## Testes da Tarefa

- [ ] Testes unitários (testify/suite whitebox, `fake.NewProvider()`): Complete sucesso/erro upstream, Stream deltas + truncamento, Embed, decode de structured output conforme/não-conforme.
- [ ] Testes de integração: variante real atrás de `RUN_REAL_LLM` exercitando OpenRouter real (Complete/Stream/Embed).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/llm/` (novo) — provider, tipos, structured output.
- `internal/agent/infrastructure/providers/openrouter/client.go` — fonte a generalizar (módulo será apagado).
- `internal/platform/httpclient/` — client HTTP reutilizado.
