# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Memória de longo prazo com pgvector + embeddings via OpenRouter
- **Data:** 2026-06-29
- **Status:** Aceita
- **Decisores:** Time de plataforma
- **Relacionados:** PRD (RF-16..RF-25, RF-38), techspec, ADR-002, ADR-005

## Contexto

O PRD inclui memória de longo prazo com recuperação semântica no MVP, com Postgres como persistência única e OpenRouter como canal LLM/embeddings. Hoje não há embeddings nem pgvector; a working memory atual é texto puro. É preciso paridade com a camada memory do Mastra (thread + working + recall semântico) sem introduzir segundo armazenamento.

## Decisão

Implementar memória em três escopos sobre Postgres: (1) **Thread/Message** (`platform_threads`/`platform_messages`) para histórico conversacional por `(resourceId, threadId)` opacos; (2) **WorkingMemory** por resource (`platform_resources.working_memory`, markdown estruturado); (3) **Recuperação semântica de longo prazo** via extensão `pgvector` (`platform_embeddings` com coluna `vector(N)` + índice ANN), com embeddings gerados por `llm.Embed` (OpenRouter) e sumarização/compressão para conteúdo que exceda a janela. Tudo atrás de portas (`MessageStore`, `WorkingMemory`, `SemanticRecall`, `Embedder`/`Summarizer`) na camada `internal/platform/memory`. pgvector é extensão do próprio Postgres — persistência permanece única.

## Alternativas Consideradas

- **Armazenamento vetorial dedicado (ex.: Qdrant/Pinecone).** Vantagem: recursos ANN avançados. Desvantagem: viola "persistência única em Postgres", adiciona operação/infra. Rejeitada.
- **Só sumarização textual (sem vetor).** Vantagem: menor escopo. Desvantagem: deixa lacuna de recuperação semântica vs Mastra; decisão de produto foi incluir recall. Rejeitada.

## Consequências

### Benefícios Esperados

- Paridade plena com a camada memory do Mastra.
- Persistência única; menos superfície operacional.
- Portas isolam o backend vetorial (troca futura sem afetar consumidores).

### Trade-offs e Custos

- Dependência da extensão `vector` no Postgres (prod e teste).
- Custo de geração de embeddings via OpenRouter; latência adicional de indexação.
- Escolha de dimensionalidade/índice impacta recall e armazenamento.

### Riscos e Mitigações

- **Risco:** `vector` indisponível. **Mitigação:** `CREATE EXTENSION IF NOT EXISTS vector` na migration; imagem testcontainers com pgvector; pré-flight documentado; `SemanticRecall` degradável atrás de porta.
- **Risco:** modelo/dimensionalidade incorretos. **Mitigação:** decisão fechada — `openai/text-embedding-3-small` (`vector(1536)`) com índice **HNSW**, configurável por env; validado na variante E2E real. Indexação assíncrona via `outbox`+`worker` (idempotente por `event_id`), fora do caminho crítico.

## Plano de Implementação

1. Migration `000003` habilita `vector` e cria `platform_embeddings` (+ índice ANN) (ADR-005).
2. Implementar portas e adapters Postgres em `internal/platform/memory`.
3. Integrar `llm.Embed`; indexar mensagens; implementar `Recall(k)` e `Summarizer`.
4. Testes de integração de recall (ANN real) + E2E weather.

## Monitoramento e Validação

- Span `memory.recall`; métrica de latência de embedding/recall (labels `model`, sem alta cardinalidade).
- Critério de sucesso: recall retorna itens relevantes em teste de integração; E2E exercita memória de longo prazo.

## Impacto em Documentação e Operação

- Runbook de operação inclui pré-flight de `pgvector` e config de modelo de embedding.

## Revisão Futura

- Revisitar dimensionalidade/índice e estratégia de sumarização conforme volume real.
