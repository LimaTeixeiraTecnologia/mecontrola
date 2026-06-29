# Tarefa 5.0: Memória em `internal/platform/memory`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar `internal/platform/memory`: Thread genérico (`resourceId`/`threadId` opacos), mensagens/turns, working memory por resource, recuperação semântica de longo prazo via pgvector (HNSW) e sumarização. Persistência em Postgres (tabelas da migration 2.0). Embeddings via `llm.Embed` (1.0), indexados assincronamente via `internal/platform/outbox` + `internal/platform/worker` (idempotente por `event_id`).

<requirements>
- RF-15: persistência Postgres única (threads, mensagens, working memory, embeddings).
- RF-16: Thread por `resourceId`/`threadId` opacos.
- RF-17: working memory estruturada por resource (get/upsert).
- RF-18: memória de longo prazo (sumarização/compressão) além da janela.
- RF-19: housekeeping/limpeza determinística onde aplicável.
- RF-20: Thread genérico equivalente ao do Mastra.
- RF-21: mensagens/turns recuperáveis com janela configurável.
- RF-22: working memory disponibilizável ao contexto do agente.
- RF-23: recuperação semântica via pgvector + embeddings via OpenRouter.
- RF-24: sumarização/compressão preservando recuperabilidade.
- RF-25: compartilhamento de contexto controlado por `resourceId`/`threadId`, sem vazamento.
- Embedding `openai/text-embedding-3-small` `vector(1536)`/HNSW; indexação assíncrona idempotente.
</requirements>

## Subtarefas

- [ ] 5.1 Portas `ThreadGateway`, `MessageStore`, `WorkingMemory`, `SemanticRecall`, `Summarizer` + entidades (`Thread`, `Message`, `RecallHit`).
- [ ] 5.2 Adapters Postgres para `platform_threads`/`platform_messages`/`platform_resources` (DBTX + uow, padrão do projeto).
- [ ] 5.3 `SemanticRecall.Index/Recall` sobre `platform_embeddings` (HNSW), consumindo `llm.Embed`.
- [ ] 5.4 Indexação assíncrona: producer de evento no append; consumer/worker que gera embedding e grava, idempotente por `event_id`; shutdown cooperativo sem leak.
- [ ] 5.5 `Summarizer` (compressão de histórico que excede a janela) via `llm.Complete`.

## Detalhes de Implementação

Ver techspec.md "Modelos de Dados", "Pontos de Integração > Indexação assíncrona", ADR-004. Seguir o padrão de persistência do projeto (`internal/platform/database`, `outbox`, `worker`). DTOs de input com `Validate()` (R-DTO-VALIDATE-001).

## Critérios de Sucesso

- Thread/Message/WorkingMemory persistidos e recuperados; recall semântico retorna itens relevantes (integração HNSW real).
- Indexação assíncrona não adiciona latência ao caminho do agente; reprocessamento de `event_id` não duplica.
- Sem vazamento de contexto entre resources/threads distintos (teste).
- Layering: `internal/platform/memory` não importa `internal/platform/agent`; gate grep vazio.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `go-implementation` — implementação Go obrigatória (CLAUDE.md): portas, repos, consumer/worker, testes.
- `mastra` — memória (Thread/WorkingMemory/recall) é primitivo canônico do padrão Mastra portado.

## Testes da Tarefa

- [ ] Testes unitários (testify/suite whitebox): use cases de memory com mocks de portas e `llm`.
- [ ] Testes de integração (`//go:build integration`, testcontainers `pgvector/pgvector:pg16`): persistência real, recall HNSW, idempotência de indexação por `event_id`.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/memory/` (novo) — domain/application/infrastructure.
- `internal/platform/llm/` — `Embed`/`Complete`.
- `internal/platform/outbox/`, `internal/platform/worker/` — indexação assíncrona.
- `migrations/000003_*` — tabelas `platform_threads/messages/resources/embeddings`.
