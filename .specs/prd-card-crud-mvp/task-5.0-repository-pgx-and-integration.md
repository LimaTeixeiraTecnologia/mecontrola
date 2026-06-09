# Tarefa 5.0: Repository pgx + paginação keyset + mapping de erros + integration tests

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar `internal/card/infrastructure/repositories/postgres/card_repository.go` (pgx puro sobre `database.DBTX`) + factory. Cinco métodos: `Insert`, `GetByIDForUser`, `ListByUser` (paginação keyset por tupla composta), `UpdateByIDForUser`, `SoftDeleteByIDForUser`. Mapping de `pgerrcode.UniqueViolation` → `ErrNicknameConflict`. Integration tests com testcontainers Postgres 16 cobrindo todos os caminhos críticos.

<requirements>
- pgx puro via `database.DBTX` (mesmo pattern de `internal/identity/.../postgres/user_repository.go`).
- Sem ORM, sem code generation de queries.
- Soft-delete: `WHERE deleted_at IS NULL` em todas as leituras; UPDATE seta `deleted_at = $now`.
- Operações em cartões soft-deleted retornam `ErrCardNotFound`.
- `Insert` com `INSERT … RETURNING created_at, updated_at`; conflito de unicidade → `pgerrcode.UniqueViolation` + `constraint_name == "cards_user_nickname_active_uniq_idx"` → `ErrNicknameConflict`.
- `ListByUser` usa keyset: `WHERE (created_at, id) < ($cursor_t, $cursor_id) ORDER BY created_at DESC, id DESC LIMIT $limit+1`. `next_cursor` existe se retornou `limit+1` linhas.
- Sem regra de negócio no repository (`R-ADAPTER-001.2` HARD); apenas SQL + mapping de erro.
- Zero comentários em `.go` (`R-ADAPTER-001.1`).
- Persistência em UTC.
- Spans OTel `card.repository.pg.<op>` em cada método.
</requirements>

## Subtarefas

- [ ] 5.1 `infrastructure/repositories/factory.go` — `RepositoryFactory.CardRepository(database.DBTX) interfaces.CardRepository`.
- [ ] 5.2 `infrastructure/repositories/postgres/card_repository.go` — implementação pgx, mapping de erros, spans.
- [ ] 5.3 Tests unit: hydration e mapping de erro com pgx mockado (assertions sobre args de query).
- [ ] 5.4 Integration test (`//go:build integration`) — `insert + read happy path`.
- [ ] 5.5 Integration test — `soft-delete then read returns ErrCardNotFound`.
- [ ] 5.6 Integration test — `concurrent insert same nickname → 1 success + 1 ErrNicknameConflict` (10 goroutines).
- [ ] 5.7 Integration test — paginação keyset estável com ≥ 250 cartões, validando: ordering correto, sem duplicatas, sem item perdido entre páginas, `next_cursor=null` na última.
- [ ] 5.8 Integration test — `update preserves created_at`; `updated_at` recalculado.

## Detalhes de Implementação

Ver `.specs/prd-card-crud-mvp/techspec.md` §"Modelos de Dados" + §"Interfaces Chave". Espelhar pattern de `internal/identity/infrastructure/repositories/postgres/user_repository.go` (já em produção).

## Critérios de Sucesso

- `go test -race -count=1 -cover ./internal/card/infrastructure/repositories/...` cobre métodos críticos.
- `go test -race -count=1 -tags=integration ./internal/card/infrastructure/repositories/...` verde em < 30s.
- `EXPLAIN` confirma uso de `cards_user_pagination_idx` na query de `ListByUser`.
- Gate `R-ADAPTER-001.2`: `grep -rn 'QueryContext\|ExecContext' internal/card/infrastructure/repositories/postgres/` retorna SQL via `database.DBTX` apenas (não em handlers — handlers cobertos em 7.0).
- Sem branching de domínio no repository (assert manual no PR).

### Definition of Done

- [ ] Repository e factory criados.
- [ ] Integration tests cobrem todos os caminhos listados em 5.4..5.8.
- [ ] `go vet` + `golangci-lint run` limpos no pacote.
- [ ] Gate de zero comentários verde.
- [ ] RF-13, RF-14, RF-46 explicitamente apontados no PR.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: mapping de pgerrcode + hydration.
- [ ] Testes de integração: 5 cenários acima com testcontainers Postgres 16.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/card/infrastructure/repositories/factory.go` (novo)
- `internal/card/infrastructure/repositories/postgres/card_repository.go` (novo)
- `internal/card/infrastructure/repositories/postgres/card_repository_test.go` (novo)
- `internal/card/infrastructure/repositories/postgres/card_repository_integration_test.go` (novo)
- Referência: `internal/identity/infrastructure/repositories/postgres/user_repository.go`
