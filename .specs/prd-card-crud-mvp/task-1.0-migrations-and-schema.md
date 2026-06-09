# Tarefa 1.0: Migrations 000004/000005 e schema `mecontrola.cards` + `idempotency_keys`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar as duas migrations idempotentes que precedem qualquer código Go do MVP: `000004_create_platform_idempotency_keys` e `000005_create_card_cards` no schema canônico `mecontrola.`. Ambas têm `down` por rename (jamais `DROP TABLE`). Estender `migrations_integration_test.go` para validar ciclo `up → down → up` e ausência de PCI no schema.

<requirements>
- Schema canônico `mecontrola.` (consistente com baseline 000001).
- `down` por rename `*_archived_<timestamp_estatico>` — sem `DROP TABLE` direto.
- Idempotência via `IF NOT EXISTS` em CREATE TABLE/INDEX.
- Sem nomes de coluna/constraint contendo `pan|cvv|cvc|track|pin` em todo o módulo (defesa estática contra PCI).
- Integration test valida `up`/`down`/`up` para 000004 e 000005.
- ADR-004 (FK física `ON DELETE RESTRICT` → `mecontrola.users(id)`) e ADR-005 (numeração 6 dígitos + rename) já governam decisões — referenciar, não duplicar.
</requirements>

## Subtarefas

- [ ] 1.1 Criar `migrations/000004_create_platform_idempotency_keys.up.sql` com tabela `mecontrola.idempotency_keys` + PK composta `(scope, key, user_id)` + CHECKs (`key BETWEEN 1..128`, `request_hash length = 64`, `response_status BETWEEN 200..599`, `octet_length(response_body) <= 65536`) + índice `expires_at`.
- [ ] 1.2 Criar `migrations/000004_create_platform_idempotency_keys.down.sql` com `ALTER TABLE … RENAME TO idempotency_keys_archived_<ts>` + `DROP INDEX IF EXISTS`.
- [ ] 1.3 Criar `migrations/000005_create_card_cards.up.sql` com tabela `mecontrola.cards` + FK física `ON DELETE RESTRICT` → `mecontrola.users(id)` + CHECKs (`closing_day/due_day BETWEEN 1..31`, comprimentos `name 1..64`, `nickname 1..32`) + índice parcial único `cards_user_nickname_active_uniq_idx` + índice composto `cards_user_pagination_idx`.
- [ ] 1.4 Criar `migrations/000005_create_card_cards.down.sql` análoga ao 1.2.
- [ ] 1.5 Estender `migrations/migrations_integration_test.go` (testcontainers Postgres 16) cobrindo ciclo `up → down → up` das duas novas migrations; verificar presença das tabelas arquivadas após `down`.
- [ ] 1.6 Adicionar test estático (Go ou shell em CI) que `grep -E '\b(pan|cvv|cvc|track|pin)\b' migrations/000005*.sql` retorne 0 matches.

## Detalhes de Implementação

Ver `.specs/prd-card-crud-mvp/techspec.md` §"Modelos de Dados", `adr-004-cards-persistence-and-fk.md` e `adr-005-migrations-numbering-and-rollback.md`. Suffix de rename: timestamp UTC literal embutido (NÃO calculado em runtime) para garantir migration determinística.

## Critérios de Sucesso

- `task migrate-up` aplica 000004 e 000005 sem erro em ambiente limpo.
- `task migrate-down` aplica down e produz `mecontrola.idempotency_keys_archived_*` + `mecontrola.cards_archived_*` (verificado por integration test).
- Reaplicar `up` após `down` é no-op para os objetos arquivados e recria tabelas originais.
- `EXPLAIN (FORMAT TEXT) SELECT … FROM mecontrola.cards WHERE user_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC, id DESC LIMIT 20` usa `cards_user_pagination_idx`.
- Constraints CHECK rejeitam insert de `closing_day=0`, `closing_day=32`, `nickname=''`, `name` com 65 chars, `response_body` com 65537 bytes, `response_status=199`.
- 0 ocorrências de `pan|cvv|cvc|track|pin` no diff dos arquivos `.sql`.

### Definition of Done

- [ ] Arquivos `.sql` (4) criados e commitados.
- [ ] `migrations_integration_test.go` estendido e verde com `go test -race -count=1 -tags=integration ./migrations/...`.
- [ ] `go vet ./migrations/...` limpo.
- [ ] `docs/runbooks/card-rollback.md` rascunhado com nome literal das tabelas arquivadas e comandos `migrate down` (preenchimento final em 9.0).
- [ ] Cobertura de RF-09..12, RF-16 (estrutural), RF-17..19 explicitamente apontada no PR.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: N/A (apenas SQL declarativo).
- [ ] Testes de integração: ciclo up/down/up das duas migrations + assert de CHECKs (insert proibido) + assert de uso do índice composto via `EXPLAIN`.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `migrations/000004_create_platform_idempotency_keys.up.sql` (novo)
- `migrations/000004_create_platform_idempotency_keys.down.sql` (novo)
- `migrations/000005_create_card_cards.up.sql` (novo)
- `migrations/000005_create_card_cards.down.sql` (novo)
- `migrations/migrations_integration_test.go` (modificar)
- `migrations/embed.go` (não modificar — `//go:embed *.sql` já cobre)
- `.specs/prd-card-crud-mvp/adr-004-cards-persistence-and-fk.md`
- `.specs/prd-card-crud-mvp/adr-005-migrations-numbering-and-rollback.md`
