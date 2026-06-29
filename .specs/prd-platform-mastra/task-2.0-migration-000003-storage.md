# Tarefa 2.0: Migration `000003` — storage genérico inspirado no Mastra + pgvector

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar `migrations/000003_platform_mastra.{up,down}.sql` (golang-migrate sequencial). O `up` remove as 7 tabelas `agent_*` do `internal/agent` descontinuado e cria o storage genérico `platform_*` inspirado no storage do Mastra, com extensão `vector`. O `down` é simétrico (recria as `agent_*` copiando o DDL de `000001` e remove o storage novo). `workflow_runs`/`workflow_steps` permanecem intactas.

<requirements>
- RF-35: DROP das 7 tabelas `agent_*` (`agent_sessions`, `agent_decisions`, `agent_threads`, `agent_runs`, `agent_working_memory`, `agent_observations`, `agent_processed_events`).
- RF-36: CREATE `platform_threads`, `platform_messages`, `platform_resources`, `platform_runs`, `platform_embeddings`, `platform_scorer_results`.
- RF-37: chaves opacas; nenhuma coluna com semântica de domínio (sem `intent_kind`); dedup/idempotência genérica.
- RF-38: `CREATE EXTENSION IF NOT EXISTS vector`; `platform_embeddings.embedding vector(1536)` + índice HNSW.
- RF-39: padrão `000NNN_*.{up,down}.sql` com par up/down reversível.
- pgvector provisionado em produção via `deployment/docker/Dockerfile.postgres` (base `postgres:16`).
</requirements>

## Subtarefas

- [ ] 2.1 Escrever `000003_platform_mastra.up.sql`: DROP `agent_*` (ordem respeitando FKs); `CREATE EXTENSION IF NOT EXISTS vector`; CREATE das tabelas `platform_*` com índices (incl. HNSW em `platform_embeddings`).
- [ ] 2.2 Escrever `000003_platform_mastra.down.sql`: DROP `platform_*`; recriar as 7 `agent_*` copiando o DDL exato de `000001`; `DROP EXTENSION` se criada aqui.
- [ ] 2.3 Estender `deployment/docker/Dockerfile.postgres` para incluir a extensão `vector` na imagem `postgres:16`.
- [ ] 2.4 Teste de integração Go (`//go:build integration`) aplicando `000001..000003`, validando tabelas/extensão e `down`/`up` reversíveis.

## Detalhes de Implementação

Ver techspec.md "Modelos de Dados" e ADR-005. Copiar o DDL das `agent_*` de `migrations/000001_initial_schema.up.sql` para o `down`. Schema `mecontrola` (consistente com as tabelas existentes).

## Critérios de Sucesso

- `migrate up` e `migrate down` aplicam sem erro; `up` após `down` é idempotente.
- Após `up`: tabelas `platform_*` presentes, extensão `vector` ativa, `agent_*` ausentes, `workflow_*` intactas.
- Nenhuma coluna de domínio nas `platform_*`.
- Gate: `grep -niE "intent_kind|category_id|direction|payment_method" migrations/000003_platform_mastra.up.sql` retorna vazio.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `go-implementation` — teste de integração de migração (up/down) e wiring em Go obrigatórios (CLAUDE.md).

## Testes da Tarefa

- [ ] Testes de integração (`//go:build integration`, testcontainers `pgvector/pgvector:pg16`): aplicar `000001..000003`, assert de schema, reversibilidade `down`/`up`, criação de índice HNSW.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `migrations/000003_platform_mastra.up.sql` (novo), `migrations/000003_platform_mastra.down.sql` (novo).
- `migrations/000001_initial_schema.{up,down}.sql` — DDL das `agent_*` a copiar para o down.
- `deployment/docker/Dockerfile.postgres` — adicionar extensão `vector`.
