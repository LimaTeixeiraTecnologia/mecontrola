# Plano: Message History + Working Memory + Observation Memory no internal/agent

**Skill obrigatória**: `go-implementation` (Etapas 1–5)
**Data**: 2026-06-22
**Status**: Concluído

## Contexto

O módulo `internal/agent` tinha sessão (`agent_sessions`) com `recent_turns JSONB` nunca populado e `LLMRequest` era single-turn. Toda interação era stateless. Inspirado no Mastra AI, este plano implementa três camadas de memória para MVP robusto.

| Camada | Mastra | mecontrola | Escopo |
|---|---|---|---|
| **Message History** | `lastMessages` (janela N turnos) | `recent_turns` já no schema | Short-term: 3 pares, sanitizados |
| **Working Memory** | Template markdown + `updateWorkingMemory` tool | Nova tabela `agent_working_memory` | Long-term: perfil persistente do usuário |
| **Observation Memory** | Observer LLM → observações estruturadas | Nova tabela `agent_observations` | Mid-term: resumo assíncrono, fire-and-forget |

**Não implementado (out of scope)**: semantic recall (pgvector), Reflector (compressor LLM), cross-thread recall, vector search.

---

## Fase 1 — Message History (short-term)

### 1.1 Domínio: `ConversationMessage`
**Criado**: `internal/agent/domain/entities/conversation_message.go`
```go
type ConversationMessage struct { Role string; Content string; At time.Time }
```

### 1.2 Serviço: `TurnHistory`
**Criado**: `internal/agent/application/services/turn_history.go`
- `Deserialize/Serialize` round-trip via JSON
- `Append` com janela deslizante `maxPairs=3` (6 mensagens)
- `ToLLMMessages` converte para `[]interfaces.ConversationMessage`
- Testes: `internal/agent/application/services/turn_history_test.go`

### 1.3 `LLMRequest.Messages`
**Modificado**: `internal/agent/application/interfaces/llm_provider.go`
- `ConversationMessage{Role, Content}` struct adicionada
- `Messages []ConversationMessage` adicionado ao `LLMRequest`

### 1.4 OpenRouter client
**Modificado**: `internal/agent/infrastructure/providers/openrouter/client.go`
- `buildRequestBody`: monta `[system, ...history, user]`

### 1.5 `compose_conversational_reply`
**Modificado**: `internal/agent/application/usecases/compose_conversational_reply.go`
- Carrega turns da sessão, sanitiza, passa `Messages` ao LLM
- Após resposta, `Append` + `Serialize` + `Upsert` sessão
- Trata tool call `updateWorkingMemory`

### 1.6 `run_onboarding_turn`
**Modificado**: `internal/agent/application/usecases/run_onboarding_turn.go`
- Carrega history antes do `Interpret`, passa `Messages`, salva após resposta

---

## Fase 2 — Working Memory (long-term)

### Migrations
- `migrations/000013_create_agent_working_memory.up/down.sql`

### Domínio
- `internal/agent/domain/entities/working_memory.go`
- `NewWorkingMemory(userID)`, `Update(content, now)`

### Interface
- `internal/agent/application/interfaces/working_memory_repository.go`
- `WorkingMemoryRepository`: `Get`, `Upsert`
- `WorkingMemoryRepositoryFactory`

### Implementação Postgres
- `internal/agent/infrastructure/repositories/postgres/working_memory_repository.go`
- `Get`: SELECT + `sql.ErrNoRows` → `(_, false, nil)`
- `Upsert`: INSERT ON CONFLICT DO UPDATE

### Template + Prompt
- `internal/agent/application/prompting/working_memory.system.tmpl`
- XML tags: `<working_memory_instructions>`, `<working_memory_template>`, `<working_memory_data>`
- `persona.system.tmpl` recebe bloco `MEMÓRIA DO USUÁRIO` condicional

### Síntese no onboarding
**Modificado**: `internal/agent/infrastructure/onboarding/onboarding_tool_dispatcher.go`
- `contextReader onboardingContextReader` + `wmWriter agentinterfaces.WorkingMemoryRepository`
- `synthesizeAndStoreWM` chamado em `dispatchComplete` (fresh) e `dispatchRecordTransaction` (completion.Completed)
- `buildWMFromSnapshot` formata markdown com objetivo, renda e cartões

---

## Fase 3 — Observation Memory (mid-term, assíncrono)

### Migrations
- `migrations/000014_create_agent_observations.up/down.sql`
- Index em `(user_id, channel, created_at DESC)`, TTL 90 dias

### Domínio
- `internal/agent/domain/entities/observation.go`
- `NewObservation(userID, channel, content, now)` — `ExpiresAt = now + 90d`

### Interface
- `internal/agent/application/interfaces/observation_repository.go`
- `Insert`, `ListRecent`, `DeleteExpired`, `DeleteOldestBeyondLimit`

### Implementação Postgres
- `internal/agent/infrastructure/repositories/postgres/observation_repository.go`

### Serviço `ObservationMemory`
- `internal/agent/application/services/observation_memory.go`
- `MaybeTrigger`: fire-and-forget goroutine com `context.WithTimeout(context.Background(), 30s)` + `recover()`
- `LoadContext`: `ListRecent(3)`, reverse to chronological, join `"\n\n---\n\n"`
- Testes: `internal/agent/application/services/observation_memory_test.go`

---

## Wiring Final (module.go)

**Modificado**: `internal/agent/module.go`
- `agentModuleBuilder` ganhou `wmRepo` e `obsRepo`
- `prepareSessionStore()` inicializa ambos via factories
- `buildLLMModule()`: após `newLLMRuntime`, reconstrói `Conversational` com todos os deps reais (graceful degradation se algum nil)
- `attachOnboardingLLM()`: passa `uc.GetContext` + `b.wmRepo` ao dispatcher, `b.sessionRepo` ao `RunOnboardingTurn`

---

## Validação Final

```
go build ./internal/agent/...   → OK
go vet ./internal/agent/...     → OK
go test -race ./internal/agent/... → OK (todos os pacotes)
zero-comments gate              → OK
```
