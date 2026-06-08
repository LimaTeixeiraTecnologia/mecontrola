# Tarefa 2.0: auth.Principal + RequireUser + ADR-001 + .golangci.yml (depguard/forbidigo)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementa o contrato de identidade canônico do MeControla. Cria o pacote `internal/identity/application/auth` com a struct `Principal` minimal e os helpers `WithPrincipal`/`FromContext`, o middleware HTTP `RequireUser` e as regras de linter que tornam bypass de identidade um erro de compilação/CI. Publica o ADR-001 que documenta o contrato e a boundary HTTP futura.

<requirements>
- RF-01: pacote `internal/identity/application/auth` com `Principal{UserID, Source}` por valor, helpers `WithPrincipal`/`FromContext`, constante `SourceWhatsApp`.
- RF-02: middleware `RequireUser` retornando 401 imediato sem corpo descritivo.
- RF-12: regra `depguard` bloqueando `r.Header.Get/.Values` em handlers com allowlist (expandida conforme PRE-02 de 1.0).
- RF-13: regra `forbidigo` bloqueando `os.Getenv` para secrets fora do pacote canônico de config identificado em 1.0 (PRE-03).
- RF-14: ADR-001 publicado.
- RF-23: travas de produto (LLM in-process, paths, rate-limit constante) referenciadas no ADR.
- RF-27: apenas `SourceWhatsApp` declarada; `SourceJWT`/`SourceSystem` apenas no ADR.
</requirements>

## Subtarefas

- [ ] 2.1 Criar `internal/identity/application/auth/principal.go` com `Principal`, `PrincipalSource`, `SourceWhatsApp`, chave de contexto privada, `WithPrincipal`, `FromContext`, `IsZero`.
- [ ] 2.2 Criar `internal/identity/application/auth/principal_test.go` com tabela-driven cobrindo round-trip, ctx vazio, `IsZero`, e microbenchmarks `BenchmarkWithPrincipal`/`BenchmarkFromContext` (alvo < 50 ns/op).
- [ ] 2.3 Criar `internal/identity/infrastructure/http/server/middleware/require_user.go` (~30 linhas) seguindo techspec.
- [ ] 2.4 Criar `require_user_test.go` com tabela: (a) sem Principal → 401 + body genérico + Content-Type JSON; (b) com Principal → next.ServeHTTP chamado. Microbenchmark overhead < 1 µs.
- [ ] 2.5 Criar/atualizar `.golangci.yml` na raiz com regras `depguard` e `forbidigo` (allowlist do PRE-02, pacote canônico do PRE-03). Validar com `golangci-lint run`.
- [ ] 2.6 Publicar `.specs/prd-auth-foundation/adr-001-principal-contract-and-future-http-boundary.md` (já existe — confirmar consistência com a implementação final).

## Detalhes de Implementação

Ver techspec `## Design de Implementação > Interfaces Chave` para o esqueleto exato dos 3 arquivos `.go` e regras invariáveis. Ver ADR-001 para racional do contrato e regras invariáveis. R0–R7 da skill `go-implementation` MUST ser respeitadas; especialmente: sem `init()`, sem `panic`, helpers retornam por valor.

## Critérios de Sucesso

- `principal.go` compila; testes verdes; microbenchmarks dentro do alvo.
- `RequireUser` retorna 401 com body exato `{"message":"unauthorized"}` e header `Content-Type: application/json`.
- `golangci-lint run` falha em arquivo de teste artificial que tenta ler `X-User-ID` ou `os.Getenv("META_APP_SECRET")` fora do pacote canônico (smoke negativo).
- ADR-001 referenciado no PR.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários tabela-driven em `principal_test.go` e `require_user_test.go`
- [ ] Microbenchmarks `BenchmarkWithPrincipal`, `BenchmarkFromContext`, `BenchmarkRequireUser_*` (`go test -bench`)
- [ ] Smoke negativo do golangci-lint contra arquivo de teste artificial

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/identity/application/auth/principal.go` (criar)
- `internal/identity/application/auth/principal_test.go` (criar)
- `internal/identity/infrastructure/http/server/middleware/require_user.go` (criar)
- `internal/identity/infrastructure/http/server/middleware/require_user_test.go` (criar)
- `.golangci.yml` (criar/atualizar)
- `.specs/prd-auth-foundation/adr-001-principal-contract-and-future-http-boundary.md` (já existe)
