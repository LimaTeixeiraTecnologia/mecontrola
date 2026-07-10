# Tarefa 7.0: Migrations aditivas — resolve_path e backfill/CHECK de correlation_key

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Tarefa **fundacional** de banco de dados: criar a migration aditiva `000008` que (a) adiciona a
coluna `resolve_path` em `mecontrola.auth_events` com CHECK de enum fechado
(`identity`/`legacy`/`backfill`) e (b) faz o backfill idempotente dos runs legados de
`mecontrola.platform_runs` com `correlation_key` vazio, seguido do CHECK de comprimento
`char_length(correlation_key) BETWEEN 1 AND 256`. Cobre a parte de DDL de RF-15 (CHECK de
`correlation_key`) e RF-21 (coluna `resolve_path`).

A coluna `resolve_path` é pré-requisito da Tarefa 6.0 (identidade canônica, ADR-006), que persiste
o path de resolução em `auth_events`; a Tarefa 6.0 depende desta migration.

Detalhes de decisão, contexto e riscos vivem em `adr-005-correlacao-wamid-e-run-update-observavel.md`
(item 4: backfill + `platform_runs_correlation_len_chk`, R1) e
`adr-006-identidade-canonica-resolve-path.md` (item 2: coluna `resolve_path` aditiva com
`auth_events_reason_check` preservado intacto) — **referenciar, não duplicar**.

<requirements>
- Migration puramente ADITIVA e reentrante: `IF NOT EXISTS` na coluna, `SET LOCAL lock_timeout`,
  schema `mecontrola` explícito, seguindo o padrão das migrations existentes (ex.: `000005`).
- `up.sql` — `auth_events`: `ADD COLUMN IF NOT EXISTS resolve_path TEXT NULL` +
  `ADD CONSTRAINT auth_events_resolve_path_chk CHECK (resolve_path IS NULL OR resolve_path IN
  ('identity','legacy','backfill'))`. A constraint `auth_events_reason_check` existente
  (proíbe `reason` não-nulo quando `kind <> 'failed'`) permanece **intacta** — eixos ortogonais em
  colunas separadas.
- `up.sql` — `platform_runs`: backfill idempotente **antes** do CHECK —
  `UPDATE mecontrola.platform_runs SET correlation_key = 'legacy:' || id::text WHERE
  correlation_key = ''` seguido de `ADD CONSTRAINT platform_runs_correlation_len_chk CHECK
  (char_length(correlation_key) BETWEEN 1 AND 256)` (validado, **não** `NOT VALID`).
- Ordem obrigatória no `up.sql`: o backfill executa ANTES do `ADD CONSTRAINT` de comprimento; caso
  contrário a migration aborta contra os 4 runs legados vazios (R1 do ADR-005).
- `down.sql`: `DROP CONSTRAINT auth_events_resolve_path_chk` + `DROP COLUMN resolve_path` e
  `DROP CONSTRAINT platform_runs_correlation_len_chk`. O backfill NÃO é desfeito (é idempotente).
- Nenhuma alteração de schema fora dessas duas tabelas; nenhuma reescrita de baseline `000001`.
</requirements>

## Subtarefas

- [ ] 7.1 Criar `migrations/000008_auth_events_resolve_path.up.sql` seguindo o padrão de `000005`
  (`SET LOCAL lock_timeout`, schema `mecontrola`, `IF NOT EXISTS`): bloco `auth_events`
  (coluna `resolve_path` + `auth_events_resolve_path_chk`).
- [ ] 7.2 No mesmo `up.sql`, bloco `platform_runs`: backfill idempotente dos runs legados vazios
  (`UPDATE ... WHERE correlation_key = ''`) ANTES do `ADD CONSTRAINT platform_runs_correlation_len_chk`.
- [ ] 7.3 Criar `migrations/000008_auth_events_resolve_path.down.sql`: `DROP CONSTRAINT` das duas
  constraints + `DROP COLUMN resolve_path` (`IF EXISTS` para reentrância); não reverter o backfill.
- [ ] 7.4 Adicionar método de teste de integração em `migrations/migrations_integration_test.go`
  (padrão `MigrationSuite` / `//go:build integration`) cobrindo up com runs legados vazios, down e
  a preservação de `auth_events_reason_check`.

## Detalhes de Implementação

Ver `adr-005-correlacao-wamid-e-run-update-observavel.md` (Decisão item 4; Riscos R1 — backfill
obrigatório antes do CHECK; Rollback — backfill idempotente não desfeito) e
`adr-006-identidade-canonica-resolve-path.md` (Decisão item 2 — coluna aditiva; Plano item 2 —
`up`/`down` da 000008; Riscos — `ADD COLUMN` de coluna nullable é operação de metadados,
`ADD CONSTRAINT ... CHECK` aditivo, `IF NOT EXISTS` reentrante). DDL alvo confirmado em
`000001_initial_schema.up.sql`: `auth_events.reason` com `auth_events_reason_check`;
`platform_runs.correlation_key TEXT NOT NULL DEFAULT ''`.

Convenções observadas no repositório a seguir: comentários `--` de referência à doc oficial
PostgreSQL são permitidos em `.sql` (a regra de zero comentários R-ADAPTER-001.1 aplica-se apenas a
`.go`); o teste de integração usa `MigrationSuite` (testcontainers Postgres, `golang-migrate` via
`migrations.FS`), com `applyBaseline`/`downToVersion`/`newMigrator` já disponíveis.

Referências: `techspec.md` (RF-15, RF-21) e as duas ADRs acima.

## Critérios de Sucesso

- `migrator.Up()` aplica limpo em banco que contenha runs legados com `correlation_key = ''`
  (o backfill executa antes do CHECK; nenhum abort).
- Após up: coluna `auth_events.resolve_path` existe, `auth_events_resolve_path_chk` rejeita valor
  fora do enum, e `auth_events_reason_check` permanece **presente e inalterada**.
- Após up: `platform_runs_correlation_len_chk` ativo; 0 linhas com `correlation_key = ''`; runs
  legados migrados para `'legacy:' || id`.
- `down` reverte coluna e ambas as constraints; a re-aplicação (`down`→`up`) é idempotente
  (`IF NOT EXISTS` / `IF EXISTS`).
- Migration reentrante: reexecutar `up` não falha.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `postgresql-production-standards` — migration aditiva segura, CHECK constraints, backfill idempotente, lock_timeout e reversibilidade conforme documentação oficial PostgreSQL.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

Testes de integração (seguir `MigrationSuite`, tag `integration`):
- Semear runs legados em `platform_runs` com `correlation_key = ''`, aplicar `up`, assertar 0 vazios
  e que o backfill produziu `'legacy:' || id` (backfill antes do CHECK, sem abort).
- Assertar `auth_events.resolve_path` existe e que `auth_events_resolve_path_chk` rejeita valor
  inválido; assertar que `auth_events_reason_check` continua presente e ativa.
- Assertar `platform_runs_correlation_len_chk` rejeita `''` e string > 256.
- Aplicar `down` e assertar remoção da coluna e das duas constraints; reaplicar `up` (reentrância).

## Arquivos Relevantes
- `migrations/000008_auth_events_resolve_path.up.sql` — coluna `resolve_path` + `auth_events_resolve_path_chk`; backfill + `platform_runs_correlation_len_chk` (backfill ANTES do CHECK).
- `migrations/000008_auth_events_resolve_path.down.sql` — `DROP CONSTRAINT` das duas constraints + `DROP COLUMN resolve_path`; não desfaz o backfill.
- `migrations/migrations_integration_test.go` — novo método de teste na `MigrationSuite` (padrão existente).
- `migrations/000001_initial_schema.up.sql` — DDL de referência (`auth_events`, `platform_runs`); não alterar.
