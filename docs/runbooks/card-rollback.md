# Card Module — Rollback Runbook

## Status

Este runbook foi **substituído** pela baseline única `V0`.

O diretório `migrations/` não possui mais as migrations incrementais `000007` e `000008`, então instruções como `migrate down 2` para arquivar `cards` e `idempotency_keys` **não são mais válidas**.

## Regra Atual

- `local`, `staging` e `production` nascem do zero com:
  - `migrations/000001_initial_baseline.up.sql`
  - `migrations/000001_initial_baseline.down.sql`
- O rollback de banco agora é apenas:
  - restaurar a partir de backup, ou
  - recriar o schema com a baseline V0 em ambiente descartável

## O que não fazer

- Não executar runbooks antigos que dependam de:
  - `000007_create_platform_idempotency_keys`
  - `000008_create_card_cards`
  - tabelas `*_archived_20260609120000`
- Não assumir que existe rollback incremental seguro para `cards` isoladamente.

## Procedimento correto

Se houver incidente envolvendo o módulo `card`:

1. Reverter o deploy da aplicação, se necessário.
2. Se o problema for apenas de código, manter o banco intacto.
3. Se o problema exigir voltar o schema inteiro em ambiente não produtivo, recriar a base a partir da V0.
4. Em produção, usar backup/restore documentado no runbook de backup do projeto.

## Referência

- Baseline ativa: `migrations/000001_initial_baseline.up.sql`
- Restore operacional: `docs/runbooks/backup-restore.md`
