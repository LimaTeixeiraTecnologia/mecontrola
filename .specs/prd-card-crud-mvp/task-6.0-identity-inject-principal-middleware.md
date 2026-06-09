# Tarefa 6.0: Identity additive — `auth.SourceHeader` + `InjectPrincipalFromHeader` middleware

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Entrega additive em `internal/identity` que viabiliza o `RequireUser` canônico em rotas HTTP do `card`. Adiciona constante `auth.SourceHeader` e middleware `InjectPrincipalFromHeader` que extrai `X-User-ID` (UUID v4), constrói `auth.Principal{UserID, Source: SourceHeader}` e injeta via `auth.WithPrincipal` no `context.Context`. NÃO retorna 401 — apenas segue; enforcement permanece no `RequireUser` canônico (ponto único). Substitui o middleware transitório do PRD original sem mudar o contrato HTTP do `card`.

<requirements>
- Constante exportada em `internal/identity/application/auth/principal.go`: `SourceHeader PrincipalSource = "header"`.
- Middleware em `internal/identity/infrastructure/http/server/middleware/inject_principal_from_header.go`.
- Em ausência/invalidez: segue sem injetar (NÃO escreve 401 — o `RequireUser` canônico cuida).
- Em header válido: chama `auth.WithPrincipal(ctx, p)` e passa adiante.
- Variante `InjectPrincipalFromHeaderWithO11y(o11y)` que adiciona span `auth.inject_principal_from_header` com atributo `principal.source=header` e `result=injected|missing|invalid`.
- Tests: header ausente / inválido (`not-a-uuid`, `00000000-0000-0000-0000-000000000000`) / válido — todos com assert de ctx e ausência de 401 escrito.
- Zero comentários em `.go` produção.
- ADR-003 governa decisão; referenciar, não duplicar.
</requirements>

## Subtarefas

- [ ] 6.1 Estender `internal/identity/application/auth/principal.go` com `SourceHeader`.
- [ ] 6.2 Estender `internal/identity/application/auth/principal_test.go` cobrindo round-trip com `SourceHeader`.
- [ ] 6.3 Criar `internal/identity/infrastructure/http/server/middleware/inject_principal_from_header.go` com `InjectPrincipalFromHeader` + `InjectPrincipalFromHeaderWithO11y`.
- [ ] 6.4 Criar `inject_principal_from_header_test.go` com 4 cenários (ausente, malformado, nil UUID, válido).

## Detalhes de Implementação

Ver `.specs/prd-card-crud-mvp/techspec.md` §"Identity — middleware adicional" e `adr-003-inject-principal-from-header-middleware.md`. Espelhar estilo de `require_user.go` (variante simples + variante com o11y).

## Critérios de Sucesso

- `go test -race -count=1 ./internal/identity/application/auth/... ./internal/identity/infrastructure/http/server/middleware/...` verde.
- Test cobre 4 cenários acima.
- `go vet` + `golangci-lint run` limpos.
- Comportamento: header ausente → ctx sem principal → o `RequireUser` em sequência produz 401 (validar via chain test).

### Definition of Done

- [ ] Constante `SourceHeader` exportada e referenciada em `inject_principal_from_header.go`.
- [ ] Middleware com 2 variantes (simples + com o11y).
- [ ] Tests verdes.
- [ ] Gate de zero comentários verde.
- [ ] RF-27 explicitamente apontado no PR.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: 4 cenários no middleware + round-trip do principal.
- [ ] Testes de integração: chain (`InjectPrincipalFromHeader → RequireUser`) cobrindo header ausente/válido → 401/200.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/identity/application/auth/principal.go` (modificar — adicionar constante)
- `internal/identity/application/auth/principal_test.go` (modificar)
- `internal/identity/infrastructure/http/server/middleware/inject_principal_from_header.go` (novo)
- `internal/identity/infrastructure/http/server/middleware/inject_principal_from_header_test.go` (novo)
- `.specs/prd-card-crud-mvp/adr-003-inject-principal-from-header-middleware.md`
