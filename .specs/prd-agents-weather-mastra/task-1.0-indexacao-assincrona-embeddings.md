# Tarefa 1.0: Indexação assíncrona de embeddings em `internal/platform/memory`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Conectar a indexação assíncrona de embeddings (gap B3): após persistir mensagem, emitir evento de outbox; um consumer/worker idempotente gera o embedding (OpenRouter) e grava `platform_embeddings`, fora do caminho crítico. Resolve o recall semântico vazio.

<requirements>
- RF-18: indexação assíncrona conectada (outbox→worker), idempotente por `event_id`, fora do caminho crítico.
- RF-19: semantic recall demonstrável — `platform_embeddings` populada; recall retorna itens escopados por `resourceId`.
- ADR-002 (decorator publicador de `MessageStore`).
</requirements>

## Subtarefas

- [ ] 1.1 `publishingMessageStore` (decorator de `MessageStore`) + interface `MessageIndexPublisher` (adapter outbox) em `internal/platform/memory`; publica `platform.memory.embedding.index.v1` com `IndexMessagePayload{ResourceID,ThreadID,MessagePK,Content,Model}` após `Append` bem-sucedido.
- [ ] 1.2 Registrar `EmbeddingIndexHandler` como consumer/worker idempotente (dedup por `event_id`); chama `llm.Embed` e grava via `SemanticRecall.Index` (já com `ON CONFLICT`).
- [ ] 1.3 Shutdown cooperativo, sem goroutine leak; métrica/log de sucesso/erro com labels enums.

## Detalhes de Implementação

Ver techspec.md §"Interfaces Chave" (decorator), §"Pontos de Integração" (outbox/worker) e ADR-002. Idempotência dupla: `event_id` no consumer + `ON CONFLICT (source_message_pk, model)` no `Index`.

## Critérios de Sucesso

- Append de mensagem dispara evento de indexação; worker grava `platform_embeddings`.
- Replay do mesmo `event_id` não duplica embedding (integração).
- Recall ANN retorna itens relevantes escopados por `resourceId` (integração testcontainers pgvector).
- Zero comentários em produção; sem leak; gofmt limpo.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `mastra` — primitivos de memória (semantic recall/working memory) e indexação seguem o modelo Mastra mapeado ao código real.

## Testes da Tarefa

- [ ] Testes unitários (decorator publica evento; propaga erro; consumer idempotente)
- [ ] Testes de integração (testcontainers pgvector: Append→evento→worker→`platform_embeddings`→Recall; replay não duplica)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/memory/publishing_message_store.go` (novo), `internal/platform/memory/infrastructure/indexer/*`, consumer/worker de indexação, `internal/platform/memory/infrastructure/postgres/embedding_repository.go` (existente).
