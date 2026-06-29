# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Indexação assíncrona de embeddings via decorator publicador de `MessageStore`
- **Data:** 2026-06-29
- **Status:** Aceita
- **Decisores:** Time de plataforma
- **Relacionados:** PRD RF-17,18,19; techspec.md; review prd-platform-mastra (achado B3)

## Contexto

A review de `prd-platform-mastra` identificou (B3) que a indexação assíncrona de embeddings **não está conectada**: `memory.AppendMessage` não publica evento de outbox e o `EmbeddingIndexHandler` não é registrado em worker. Resultado: `platform_embeddings` nunca é populada e o semantic recall retorna vazio. Além disso, o `AgentRuntime` persiste mensagens chamando `MessageStore.Append` **diretamente** (runtime.go:142,150), não via o usecase `AppendMessage` — então publicar no usecase não cobriria o caminho do agente.

## Decisão

Introduzir em `internal/platform/memory` um **decorator de `MessageStore`** (`publishingMessageStore`) que, após `Append` bem-sucedido, publica um evento de outbox (`platform.memory.embedding.index.v1`, payload `IndexMessagePayload{ResourceID, ThreadID, MessagePK, Content, Model}`). Um **consumer/worker** registrado consome o evento, gera o embedding via `llm.Embed` (OpenRouter) e grava por `SemanticRecall.Index`. A idempotência é dupla: dedup por `event_id` no consumer e `ON CONFLICT (source_message_pk, model) DO NOTHING` no `Index` (já presente). O processamento é **fora do caminho crítico** (sem latência LLM na resposta) com **shutdown cooperativo** (sem leak). O `internal/agents` injeta o `MessageStore` decorado no `AgentRuntime`.

## Alternativas Consideradas

- **Publicar no usecase `AppendMessage`**: rejeitada — o `AgentRuntime` não usa o usecase; cobertura incompleta.
- **Alterar o `AgentRuntime` para chamar o usecase**: rejeitada agora — muda a plataforma além do necessário; o decorator é menos invasivo e genérico.
- **Indexação síncrona no caminho da resposta**: rejeitada — adiciona latência de embedding na resposta ao usuário; viola "fora do caminho crítico".

## Consequências

### Benefícios Esperados
- Recall semântico funciona de verdade; correção genérica reutilizável por qualquer consumidor.
- Caminho crítico sem latência de embedding.

### Trade-offs e Custos
- Mais um decorator + consumer/worker para operar; eventual consistency do recall (aceitável).

### Riscos e Mitigações
- Duplicação/leak: idempotência por `event_id` + `ON CONFLICT`; shutdown cooperativo; teste de replay.
- Falha de embedding: erro logado/métrica; mensagem permanece persistida (degradação controlada do recall).

## Plano de Implementação

1. `publishingMessageStore` + interface `MessageIndexPublisher` (adapter outbox).
2. Registrar `EmbeddingIndexHandler` como consumer no worker (dedup por `event_id`).
3. `internal/agents` injeta o store decorado no `AgentRuntime`.
4. Teste de integração: Append→evento→worker→`platform_embeddings`→Recall ANN; replay não duplica.

## Monitoramento e Validação

- Métrica de indexação (sucesso/erro) com labels enums; `platform_embeddings` populada em integração.

## Impacto em Documentação e Operação

- Runbook: novo consumer/worker de indexação; reprocessamento idempotente.

## Revisão Futura

- Revisar se a plataforma passar a expor um `MemoryService` de alto nível que centralize Append+index.
