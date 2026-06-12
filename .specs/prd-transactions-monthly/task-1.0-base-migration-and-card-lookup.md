# Tarefa 1.0: Base — migration `000014` + cross-module `GetCardForUser`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Cria a base inegociável do módulo: schema Postgres completo (7 tabelas com naming `transactions_*` exceto a principal `transactions`), migrations up/down idempotentes, e novo use case fino `GetCardForUser` em `internal/card` consumido pelo adapter outbound de `internal/transactions`. Tarefa raiz que desbloqueia todas as demais.

<requirements>
- Migration `migrations/000014_create_transactions_baseline.{up,down}.sql` com 7 tabelas: `transactions`, `transactions_card_purchases`, `transactions_card_invoices`, `transactions_card_invoice_items`, `transactions_recurring_templates`, `transactions_recurring_materializations`, `transactions_monthly_summary`.
- Todas com `version BIGINT NOT NULL DEFAULT 1` quando agregado mutável; `deleted_at TIMESTAMPTZ NULL` quando aplica soft-delete; check constraints conforme schema na techspec; índices `WHERE deleted_at IS NULL`.
- Constraint `transactions_card_invoices_uk UNIQUE (user_id, card_id, ref_month)` é gate para `UpsertByMonth` idempotente.
- Constraint `transactions_card_invoice_items_purchase_uk UNIQUE (purchase_id, installment_index)`.
- PK composta `transactions_recurring_materializations_pkey (template_id, ref_month)` para idempotência double-layer do job de recorrência.
- Sem FK cross-module (RT-22): `category_id`/`subcategory_id`/`card_id` são UUIDs sem REFERENCES para outras tabelas de módulos distintos.
- Sem trigger SQL para regra de domínio (RT-02).
- Novo use case `internal/card/application/usecases/get_card_for_user.go` fino que reusa `RepositoryFactory.CardRepository.GetByIDForUser` e retorna `valueobjects.CardBillingSnapshot` (ou tipo equivalente do `internal/card`).
- `internal/card/module.go` expõe `CardLookup *usecases.GetCardForUser` em `CardModule` sem mudar `InvoiceFor` existente.
- `migrations/migrations_integration_test.go` valida up/down sem perda (já existe no repo; estender para cobrir `000014`).
</requirements>

## Subtarefas

- [ ] 1.1 Migration `up` em `migrations/000014_create_transactions_baseline.up.sql` com `SET LOCAL lock_timeout='5s'; SET LOCAL statement_timeout='120s';` no topo (padrão do `000012`).
- [ ] 1.2 Migration `down` com `DROP TABLE IF EXISTS ... CASCADE` em ordem reversa de dependência.
- [ ] 1.3 Atualizar `migrations/migrations_integration_test.go` para validar `000014` em modo up/down/up.
- [ ] 1.4 Criar `internal/card/application/usecases/get_card_for_user.go` com struct + `NewGetCardForUser(factory, mgr, o11y)` + `Execute(ctx, cardID, userID) (BillingSnapshot, error)`. Reusar `repo.GetByIDForUser`.
- [ ] 1.5 Expor `CardLookup` em `internal/card/module.go` (`CardModule.CardLookup *usecases.GetCardForUser`).
- [ ] 1.6 Unit test em `get_card_for_user_test.go` com `testify/suite` + mock de `CardRepository` (mockery): happy path + `ErrCardNotFound` + ownership mismatch.

## Detalhes de Implementação

Referência: techspec seção "Modelos de Dados" (SQL completo das 7 tabelas com índices, PKs, UKs, check constraints e enums `iota+1`); ADR-001 (snapshot estático de `BillingCycle`); audit fix #2 (nome `CardBillingSnapshot` único).

## Critérios de Sucesso

- `task migrate` aplica `000014` sem erro em DB limpo.
- `task migrate-down` reverte sem erro.
- `go test -race -count=1 ./migrations/...` passa.
- `go test -race -count=1 ./internal/card/...` passa (incluindo novos testes).
- `golangci-lint run ./internal/card/... ./migrations/...` limpo.
- Zero comentários em `.go` de produção (R-ADAPTER-001.1).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unit test `get_card_for_user_test.go`: happy path, not found, ownership mismatch.
- [ ] Integration test `migrations_integration_test.go` cobrindo up/down/up de `000014`.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `migrations/000014_create_transactions_baseline.up.sql` (novo)
- `migrations/000014_create_transactions_baseline.down.sql` (novo)
- `migrations/migrations_integration_test.go` (modificado)
- `internal/card/application/usecases/get_card_for_user.go` (novo)
- `internal/card/application/usecases/get_card_for_user_test.go` (novo)
- `internal/card/module.go` (modificado — expor `CardLookup`)
- `mockery.yml` (modificado — registrar interfaces do `internal/card` se ainda não estiverem)
