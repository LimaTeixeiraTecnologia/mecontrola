# Tarefa 1.0: Migration 000017 + storage_postgres (Insert/ClaimBatch)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adiciona coluna `aggregate_user_id UUID NULL` em `mecontrola.outbox_events` com index parcial e atualiza `internal/platform/outbox/storage_postgres.go` para incluir o campo em INSERT e ClaimBatch.

<requirements>
- RF-01: migration `000017_outbox_events_aggregate_user_id.up.sql/.down.sql`
- RF-02: index parcial `WHERE aggregate_user_id IS NOT NULL`
- RF-03: `down` reverte com DROP INDEX + DROP COLUMN
- RF-11: Insert popula coluna (NULL quando string vazia)
- RF-12: ClaimBatch retorna coluna no SELECT + Scan
- RF-13: housekeeping inalterado (filtra por published_at, não user)
- Zero comentário em `.go` produção
- Sem nova dep em `go.mod`
</requirements>

## Subtarefas

- [ ] 1.1 Criar `migrations/000017_outbox_events_aggregate_user_id.up.sql` com `ALTER TABLE ... ADD COLUMN aggregate_user_id UUID NULL` + `CREATE INDEX IF NOT EXISTS outbox_events_aggregate_user_id_idx ON ... (aggregate_user_id) WHERE aggregate_user_id IS NOT NULL` (sem `CONCURRENTLY` — incompatível com tx do golang-migrate; coluna nasce 100% NULL, build do índice é instantâneo).
- [ ] 1.2 Criar `migrations/000017_outbox_events_aggregate_user_id.down.sql` reversa.
- [ ] 1.3 Atualizar `internal/platform/outbox/storage_postgres.go` `Insert`: adicionar `aggregate_user_id` na lista de colunas e `VALUES`, passar `nilIfEmpty(evt.AggregateUserID)` como param.
- [ ] 1.4 Atualizar `ClaimBatch` SELECT + Scan: usar `sql.NullString` para `aggregate_user_id`; setar `r.AggregateUserID = ns.String` se Valid.
- [ ] 1.5 Helper local `nilIfEmpty(s string) any` retorna `nil` se vazio, `s` caso contrário.
- [ ] 1.6 Teste integration: `migrations_integration_test.go` (suite existente) continua verde após up → down → up.

## Detalhes de Implementação

Ver techspec seção "Modelos de Dados". `ADD COLUMN UUID NULL` é metadata-only em Postgres (instantâneo). Decisão deliberada de **não usar `CONCURRENTLY`**: golang-migrate envolve cada arquivo em transação e `CREATE INDEX CONCURRENTLY` exige rodar fora de tx — implementar isso exigiria split do arquivo, hook custom ou script pós-migration, complexidade incompatível com escopo v1. Como a coluna nasce 100% NULL, o índice parcial `WHERE aggregate_user_id IS NOT NULL` materializa zero linhas no momento do build; `AccessExclusiveLock` é instantâneo. Ver `1.0_execution_report.md`.

## Critérios de Sucesso

- `task migrate:up` aplica `000017` sem erro.
- `psql -c "\d mecontrola.outbox_events"` mostra coluna + index.
- `task migrate:down` reverte limpo.
- `migrations_integration_test.go` PASS.
- `go test -count=1 ./internal/platform/outbox/...` PASS.
- `task lint && task test && task vulncheck` PASS.

## Skills Necessárias

<!-- MANDATÓRIO -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Integration de migration (up → down → up)
- [ ] Round-trip Insert + ClaimBatch com e sem AggregateUserID
- [ ] Inspeção manual `\d` no psql

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `migrations/000017_outbox_events_aggregate_user_id.up.sql` (novo)
- `migrations/000017_outbox_events_aggregate_user_id.down.sql` (novo)
- `internal/platform/outbox/storage_postgres.go` (modificado)
