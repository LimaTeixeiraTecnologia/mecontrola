# Tarefa 2.0: Pacote `internal/platform/idempotency/` (Storage + Middleware + Recorder)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Entregar o pacote genérico de idempotência: interface `Storage`, implementação `PostgresStorage` (pgx puro sobre `database.DBTX`), tipo `IdempotencyContext` propagado via `context.Context`, middleware chi com pré-check/replay/conflict + caching best-effort de 4xx, e `responseRecorder` com cap de 64 KB. Reuso por `internal/card` no MVP; arquitetura preparada para `billing`/`identity` em fase 2.

<requirements>
- API pública: `Storage`, `Record`, `IdempotencyContext`, `WithContext`/`FromContext`, `NewPostgresStorage`, `Middleware`.
- Pré-check do middleware: 400 `missing_idempotency_key` se header ausente; 400 `invalid_idempotency_key` se >128 ASCII; 409 `idempotency_conflict` em hit + hash diverge.
- Replay byte-idêntico em hit + hash igual (mesmo `status` + `body`).
- Miss: injeta `IdempotencyContext{Scope, Key, UserID, RequestHash, ExpiresAt}` no ctx; NÃO grava 2xx (responsabilidade do use case dentro do UoW — ADR-006). Grava best-effort em tx separada para 4xx; descarta 5xx.
- `responseRecorder` cap 64 KB → `ErrResponseTooLarge` ⇒ middleware promove 500 ao cliente, descarta cache, loga `card.idempotency.body_overflow`.
- `PostgresStorage.Put` usa `INSERT … ON CONFLICT DO NOTHING RETURNING` para resolver race; em conflito com hash diferente retorna `ErrHashMismatch`.
- Mocks gerados via mockery (entrada em `mockery.yml`).
- Sem `init()`, sem `panic`, sem `clock.Clock`, sem `var _ Interface = (*Type)(nil)` (R0/R5.12/R6.4/R6.7).
- Zero comentários em arquivos `.go` de produção (`R-ADAPTER-001.1`).
</requirements>

## Subtarefas

- [ ] 2.1 `storage.go` — interfaces, `Record`, sentinels `ErrNotFound`, `ErrHashMismatch`, `ErrResponseTooLarge`.
- [ ] 2.2 `postgres_storage.go` — `Get` (`SELECT … WHERE expires_at > now()`), `Put` (`INSERT ON CONFLICT DO NOTHING RETURNING`, fallback `SELECT` para detectar mismatch).
- [ ] 2.3 `context.go` — `IdempotencyContext` + `WithContext`/`FromContext`.
- [ ] 2.4 `recorder.go` — `responseRecorder` envolvendo `http.ResponseWriter` com buffer cap 64 KB.
- [ ] 2.5 `middleware.go` — orquestra pré-check, encadeia `next` com ctx populado, decide gravação pós-handler conforme classe HTTP.
- [ ] 2.6 `mockery.yml` — adicionar entrada para `Storage`.
- [ ] 2.7 Unit tests: header ausente/inválido, hit-match, hit-mismatch, miss + 2xx (não grava direto), miss + 4xx (grava), miss + 5xx (não grava), overflow.
- [ ] 2.8 Integration test (`//go:build integration`) com testcontainers: 10 goroutines disputando mesma key → 1 inserção + 9 replays byte-idênticos.

## Detalhes de Implementação

Ver `.specs/prd-card-crud-mvp/techspec.md` §"Plataforma — idempotência genérica" e §"Interfaces Chave", `adr-001-platform-idempotency-package.md` e `adr-006-idempotency-atomicity-via-uow.md`.

## Critérios de Sucesso

- Interface `Storage` é a única dependência consumida por testes de unit do middleware (zero acoplamento ao Postgres em unit tests).
- `Middleware` é parametrizável por `scope` para permitir reuso futuro por `billing`/`identity`.
- Integration test de race produz exatamente 1 linha em `mecontrola.idempotency_keys` e 10 respostas com `status` e `body` idênticos byte-a-byte.
- p99 de overhead do middleware em miss < 5 ms (validado em micro-benchmark com `go test -bench`).

### Definition of Done

- [ ] Pacote `internal/platform/idempotency/` criado e commitado.
- [ ] `go test -race -count=1 ./internal/platform/idempotency/...` verde.
- [ ] `go test -race -count=1 -tags=integration ./internal/platform/idempotency/...` verde.
- [ ] `go vet ./internal/platform/idempotency/...` e `golangci-lint run ./internal/platform/idempotency/...` limpos.
- [ ] Gate `grep -rn --include="*.go" --exclude="*_test.go" "^[[:space:]]*//" internal/platform/idempotency/ | grep -Ev "//go:|//nolint:|// Code generated"` retorna vazio.
- [ ] Mocks gerados via `task generate-mocks` (ou comando equivalente).
- [ ] RF-28, RF-30, RF-31, RF-32 explicitamente apontados no PR.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: middleware completo (todas as classes de status), `postgres_storage` com pgx mockado, `responseRecorder` com cap.
- [ ] Testes de integração: race de 10 goroutines + TTL expirado tratado como miss.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/platform/idempotency/storage.go` (novo)
- `internal/platform/idempotency/postgres_storage.go` (novo)
- `internal/platform/idempotency/context.go` (novo)
- `internal/platform/idempotency/recorder.go` (novo)
- `internal/platform/idempotency/middleware.go` (novo)
- `internal/platform/idempotency/*_test.go` (novo)
- `internal/platform/idempotency/postgres_storage_integration_test.go` (novo, `//go:build integration`)
- `internal/platform/idempotency/mocks/` (gerado)
- `mockery.yml` (modificar)
