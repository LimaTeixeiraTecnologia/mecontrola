# Memory — Thread, Message, WorkingMemory, Semantic Recall

O primitivo de memória `internal/platform/memory` é genérico sobre chaves opacas `(resourceId, threadId)`,
sem semântica de domínio. Persistência em Postgres (`platform_threads`, `platform_messages`,
`platform_resources`, `platform_embeddings`) via `infrastructure/postgres`.

## Portas (`ports.go`)

```go
type ThreadGateway interface {
    GetOrCreate(ctx context.Context, resourceID, threadID string) (Thread, error)
}
type MessageStore interface {
    Append(ctx context.Context, threadPK uuid.UUID, m Message) error
    Recent(ctx context.Context, threadPK uuid.UUID, limit int) ([]Message, error)
}
type WorkingMemory interface {
    Get(ctx context.Context, resourceID string) (string, error)
    Upsert(ctx context.Context, resourceID, content string) error
}
type SemanticRecall interface {
    Index(ctx context.Context, resourceID, threadID string, sourceMessagePK uuid.UUID, content, model string, embedding []float32) error
    Recall(ctx context.Context, resourceID, query string, embedding []float32, k int) ([]RecallHit, error)
}
type Summarizer interface { Summarize(ctx, messages) (string, error) }
type MessageIndexPublisher interface { PublishIndex(ctx, IndexMessagePayload) error }
```

## Tipos (`types.go`)

- `Thread{ID, ResourceID, ThreadID, Title, Metadata, CreatedAt, UpdatedAt}`.
- `Message{ID, ThreadPK, ResourceID, Role MessageRole, Content, Parts, CreatedAt}`.
- `MessageRole` fechado: `RoleUser`/`RoleAssistant`/`RoleTool`/`RoleSystem` (`String()`/`IsValid()`/`ParseMessageRole`).
- `RecallHit{ResourceID, ThreadID, Content, Score}`.
- `IndexMessagePayload{ResourceID, ThreadID, MessagePK, Content, Model}`.

## WorkingMemory no system prompt

O runtime (`agent/runtime.go buildMessages`) lê `WorkingMemory.Get(resourceID)` e, se não vazio, concatena
ao system prompt (`instructions + "\n\n## Working Memory\n" + wm`). Ausência de working memory **não é erro**.
Atualize via `Upsert(resourceID, content)` num usecase dedicado.

## Janela de mensagens

`MessageStore.Recent(threadPK, 20)` alimenta o histórico recente; o runtime persiste os turnos (`RoleUser`,
`RoleAssistant`) ao final da execução. Conversas longas podem usar `Summarizer` (compressão).

## Indexação assíncrona de embeddings (RAG / pgvector) — wired

O caminho está **conectado** (não é mais gap):

1. `NewPublishingMessageStore(next, pub, embedModel, o11y)` decora o `MessageStore`: após `Append`, publica
   `IndexMessagePayload` via `MessageIndexPublisher.PublishIndex` (evento `EventTypeEmbeddingIndex =
   "platform.memory.embedding.index.v1"`). Falha de publish é logada mas não quebra o append.
2. No `module.go`, quando há `OutboxPublisher`: usa `indexer.NewOutboxMessageIndexPublisher(outbox)` e
   registra `indexer.NewEmbeddingIndexHandler(provider, semanticRecall, o11y)` para esse EventType.
3. `EmbeddingIndexHandler.Handle` faz parse do payload, gera embedding via `provider.Embed([]string{content})`
   e chama `SemanticRecall.Index(...)`. Idempotência por `event_id` do outbox.
4. `EmbeddingRepository` (`infrastructure/postgres/embedding_repository.go`) grava em `platform_embeddings`
   (`vector(1536)`, índice HNSW). `Recall` faz busca por similaridade.

Métricas: `platform_memory_embedding_index_published_total`, `..._publish_failed_total`,
`..._succeeded_total`, `..._failed_total` (label `model`/`reason`).

## Cuidados
- Não vazar contexto entre `resourceId` distintos (isolamento — RF-25).
- `resourceId`/`threadId` são opacos: nunca embuta semântica de domínio nas chaves.
- Embedding model deve casar com a dimensão da coluna (`vector(1536)` para text-embedding-3-small).

Wiring de referência: `internal/agents/module.go`. Ver também `build-new-agent.md`.
