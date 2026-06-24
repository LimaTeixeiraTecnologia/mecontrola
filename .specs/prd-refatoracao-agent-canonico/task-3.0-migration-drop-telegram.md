# Tarefa 3.0: Migration 000020 drop Telegram (schema) + verificação pré-deploy

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Remover do schema os objetos específicos de Telegram via nova migration reversível (sem editar a
baseline), assumindo zero usuários Telegram em produção, com verificação pré-deploy fail-fast.

<requirements>
- RF-05: remover coluna/índice/CHECK específicos de Telegram (`telegram_external_id`, índice parcial, CHECK `channel IN ('whatsapp','telegram')` → `'whatsapp'`) via nova migration ALTER, sem editar baseline, com rollback.
- RF-42: tabelas/colunas que não serão mais usadas removidas via migration reversível; nenhuma tabela do agent em uso é removida sem evidência.
</requirements>

## Subtarefas

- [ ] 3.1 Criar `migrations/000020_drop_telegram_channel.up.sql`: `DROP INDEX onboarding_tokens_telegram_external_id_idx`; `ALTER TABLE onboarding_tokens DROP COLUMN telegram_external_id`; recriar 3 CHECK constraints (`channel_processed_messages`, `user_identities`, `onboarding_sessions`) só com `'whatsapp'`. Incluir `DELETE FROM channel_processed_messages WHERE channel='telegram'` (dedup residual descartável) antes dos `ADD CONSTRAINT`.
- [ ] 3.2 Criar `migrations/000020_drop_telegram_channel.down.sql`: restaurar coluna/índice + CHECK `IN ('whatsapp','telegram')` (estado do baseline).
- [ ] 3.3 Verificação pré-deploy (runbook de release): `SELECT count(*) FROM user_identities WHERE channel='telegram'` e idem `onboarding_sessions`; se > 0, **abortar** e escalar (premissa violada).
- [ ] 3.4 Ajustar `migrations/migrations_integration_test.go` (remover asserts de coluna/insert telegram; manter `assertTableMissing(telegram_processed_updates)`).
- [ ] 3.5 Teste de integração da migration up/down (testcontainers).

## Detalhes de Implementação

Ver `adr-005-eliminate-telegram.md` §"LISTA 3 — MIGRATION ALTER" (SQL up/down completos) e techspec
§"Modelos de Dados". Não editar `000001_initial_baseline.up.sql`.

## Critérios de Sucesso

- Migration up/down idempotente e reversível; aplica constraints whatsapp-only.
- Verificação pré-deploy documentada no runbook; fail-fast se houver dados Telegram.
- Teste de integração de migrations verde.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários (n/a — SQL; cobrir via integração).
- [ ] Testes de integração (migration up/down em Postgres testcontainer; pré-cleanup de dedup; constraints aplicadas).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `migrations/000020_drop_telegram_channel.up.sql` (novo)
- `migrations/000020_drop_telegram_channel.down.sql` (novo)
- `migrations/migrations_integration_test.go`
- `docs/runbooks/deploy-producao.md` (passo de verificação pré-deploy)
