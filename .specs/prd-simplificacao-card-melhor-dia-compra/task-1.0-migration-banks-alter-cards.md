# Tarefa 1.0: Migration `000002` — tabela `banks` + seed; alterar `cards`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar a migration `000002` (up/down) que introduz a tabela de referência `mecontrola.banks` (banco →
`days_before_due`) com seed idempotente dos 8 bancos, e altera `mecontrola.cards`: dropa `limit_cents`
e `name` (com seus CHECK), adiciona `bank` (TEXT, texto original do banco). Sem cartões em produção ⇒
sem backfill de dados.

<requirements>
- RF-10: tabela `banks` administrável (persistida) com seed inicial dos 8 bancos + fallback conceitual de 7 dias.
- RF-17: `cards` ganha `bank` (NOT NULL), perde `limit_cents`; `name` dropado (ADR-005); `closing_day` permanece.
- Migration reversível: `.down.sql` recria `limit_cents`, `name` e seus CHECK; remove `bank` e `banks`.
- Seed no padrão `ON CONFLICT (code) DO NOTHING` (idempotente), como `billing_plans`/`categories`.
- Zero cartões em produção: sem `UPDATE`/backfill; alteração de esquema direta.
</requirements>

## Subtarefas

- [ ] 1.1 Criar `migrations/000002_card_simplification.up.sql`: `CREATE TABLE mecontrola.banks` (PK textual `code`, `name`, `days_before_due SMALLINT`, CHECKs) + `INSERT ... ON CONFLICT DO NOTHING` (nubank 7, itaú 8, santander 8, bradesco 7, banco-do-brasil 7, caixa 7, inter 7, c6-bank 7).
- [ ] 1.2 No mesmo `.up.sql`: `ALTER TABLE mecontrola.cards` drop `cards_limit_cents_chk`+`limit_cents`; drop `cards_name_len_chk`+`name`; add `bank TEXT NOT NULL DEFAULT ''` + `cards_bank_len_chk` (1..64) + `ALTER COLUMN bank DROP DEFAULT`.
- [ ] 1.3 Criar `migrations/000002_card_simplification.down.sql`: recriar `limit_cents` (+CHECK), `name` (+CHECK), remover `bank`; `DROP TABLE mecontrola.banks CASCADE`.
- [ ] 1.4 Validar via `migrations_integration_test.go` (up/down/up idempotente).

## Detalhes de Implementação

Ver `techspec.md` §"Modelos de Dados" (DDL de `banks` e ALTER de `cards`) e ADR-001/ADR-005. Convenção:
`golang-migrate` v4 (pgx5), `//go:embed *.sql`, schema único `mecontrola`, arquivos zero-padded 6 dígitos.
`days_before_due` com CHECK `BETWEEN 1 AND 28`. Seed com `code` já normalizado (`banco-do-brasil`, `c6-bank`).

## Critérios de Sucesso

- `mecontrola.banks` criada com 8 linhas e `days_before_due` corretos por banco.
- `cards.limit_cents` e `cards.name` ausentes; `cards.bank` presente (NOT NULL, CHECK 1..64); `cards.closing_day` intacto.
- Índice único parcial `cards_user_nickname_active_uniq_idx` por `(user_id, nickname)` preservado.
- `migrator.Up()`/`Down()`/`Up()` idempotente e verde no teste de integração.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: n/a (SQL puro).
- [ ] Testes de integração: `migrations/migrations_integration_test.go` — asserir presença/ausência de colunas, seed de `banks`, up/down/up idempotente.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `migrations/000002_card_simplification.up.sql` (novo)
- `migrations/000002_card_simplification.down.sql` (novo)
- `migrations/migrations_integration_test.go` (ajuste de asserção)
- `migrations/000001_initial_schema.up.sql` (referência do DDL de `cards`)
