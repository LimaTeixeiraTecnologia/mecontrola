# Migration + deploy de produção da plataforma Mastra (2026-06-29)

## Decisão final: forward migration (NÃO unificação)

Inicialmente as migrations foram unificadas num único `000001` sob a premissa "0 usuários".
A inspeção da VPS (`root@187.77.45.48`) revelou **dados reais em produção** (2 users,
3 billing_subscriptions, 1 transaction, 1 budget, seeds). Unificar exigiria `DROP SCHEMA`
(perda de dados). Decisão do usuário: **forward migration preservando dados**.

- Revertida a unificação no repo; mantido histórico sequencial `000001` + `000002`.
- Criada `000003_platform_mastra.{up,down}.sql` (forward v2→v3):
  - up: `DROP` das 7 tabelas `agent_*` + `CREATE EXTENSION vector` + tabelas `platform_*`
    (threads, resources, messages, runs, embeddings, scorer_results) + índices (HNSW, unique
    parcial de embeddings) — DDL extraída verbatim do schema final verificado.
  - down: `DROP` `platform_*` + `DROP EXTENSION vector` + recria as 7 `agent_*` (de `000001`).
- `schema_migrations` permanece em `mecontrola` (prod já estava na v2 lá).
- Gate verificado: schema-diff vazio entre (000001+000002+000003) e o schema-alvo; preservação
  de dados (sentinel user sobrevive); reversibilidade do down.

## Deploy de produção (single-node Swarm)

1. `rsync` do working tree para `/opt/mecontrola-deploy`; build na VPS de 2 imagens
   (`mecontrola:mastra-20260629-191935` app + `mecontrola-postgres:...` com pgvector).
2. `.env` atualizado: `IMAGE_TAG`, `POSTGRES_IMAGE`, `OPENROUTER_*`, `AGENT_LLM_EMBED_MODEL`.
3. Postgres atualizado para a imagem com pgvector (volume preservado).
4. `migrate` (000003) forward → v3.
5. `docker stack deploy` (server-1/2, worker-1/2 na nova imagem).

Estado final verificado em prod: `schema_version=3`, `platform_*`=6, pgvector instalado, HNSW
presente, `agent_*`=0, **dados preservados** (users=2, subs=3, transactions=1, budgets=1,
categories=139); `/health` = `healthy` (database healthy); 0 panic/fatal; 8 serviços 1/1.

## Incidente (postmortem) — corrigido

Ao atualizar a imagem do postgres usei `--update-order start-first`, que rodou DUAS instâncias
postgres no mesmo volume por instantes, corrompendo arquivos (índice `schema_migrations_pkey` e
as 7 tabelas `agent_*` — block 0 truncado). **Recuperação:** REINDEX, `DROP` das `agent_*`
corrompidas (descartáveis — removidas pela 000003 de qualquer forma), restauração de
`schema_migrations` para v2 e reaplicação da 000003. Scan de integridade final: `bad=0`,
dados de usuário/billing/transactions intactos. **Lição:** serviços stateful (DB) DEVEM usar
`--update-order stop-first`, nunca `start-first`.

## Pendência conhecida (pré-existente)

`TestMigrationSuite/TestBaselineUpDownUp` falha (`000001` down faz `DROP SCHEMA mecontrola CASCADE`,
incompatível com o bookkeeping do golang-migrate em `mecontrola` ao descer à v0). Vermelho já
existente em `origin/main`, independente deste trabalho. Resto da suíte (144 pacotes) verde.
