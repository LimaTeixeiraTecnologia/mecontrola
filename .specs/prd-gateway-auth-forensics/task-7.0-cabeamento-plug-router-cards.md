# Tarefa 7.0: Cabeamento module.go + plug router cards + integration test

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Conecta o middleware às fronteiras reais: expõe factory de `RequireGatewayAuth` via `internal/identity/module.go` consumindo `SecretPair` da config + use case `RecordGatewayAuthFailure`; insere o middleware no chain de `internal/card/.../router.go` **imediatamente antes** de `InjectPrincipalFromHeaderWithO11y` (ADR-004). Cobre com integration test que prova chain real + persistência de `auth_events` com `reason="gateway_*"` em falha.

<requirements>
- RF-01: 100% das rotas com `InjectPrincipalFromHeader` ganham `RequireGatewayAuth` à frente (hoje apenas `internal/card`)
- RF-12: tabela de rotas (ADR-007) honrada — webhooks `/api/v1/whatsapp/*` e `/api/v1/kiwify/*` continuam sem gateway
- Ordem fixa (ADR-004): `RequireGatewayAuth → InjectPrincipalFromHeader → RequireUser → idempotency middleware`
- Integration test usa `testcontainers-go` (padrão do repo) e cobre: (a) gateway válido + Principal injetado → handler executa; (b) gateway inválido → 401 + `auth_events` row com `reason="gateway_invalid_signature"`
- Zero comentário em `.go`
- Sem regressão em smoke test de webhooks (não devem exigir gateway)
</requirements>

## Subtarefas

- [ ] 7.1 Adicionar factory exportado em `internal/identity/module.go`: `NewRequireGatewayAuth(...) func(http.Handler) http.Handler` consumindo config + use case da tarefa 5.0.
- [ ] 7.2 Editar `internal/card/infrastructure/http/server/router.go`: na função `Register`, inserir `sub.Use(middleware.RequireGatewayAuth(deps))` **antes** de `sub.Use(middleware.InjectPrincipalFromHeaderWithO11y(rt.o11y))`.
- [ ] 7.3 Atualizar wiring do CardRouter: campo novo `gatewayAuth func(http.Handler) http.Handler` injetado via `NewCardRouter` ou via deps inteiros recebidos do module.
- [ ] 7.4 Atualizar `cmd/server/server.go` para injetar o middleware ao montar o CardRouter (seguir pattern existente).
- [ ] 7.5 Integration test em `internal/identity/infrastructure/http/server/middleware/require_gateway_auth_integration_test.go` com build tag `//go:build integration`: levanta Postgres via testcontainers, aplica migrations, monta servidor mínimo com chain real (gateway + inject + handler stub), executa 2 cenários.
- [ ] 7.6 Validar manualmente com `curl` local: `curl -H 'X-User-ID: <uuid>' http://localhost:<port>/api/v1/cards` → 401; com headers de gateway válidos → 200 (ou 401 do `RequireUser` se user não existe — esperado).

## Detalhes de Implementação

Ver techspec seção "Fluxo de Dados" + ADR-004 (ordem) + ADR-007 (tabela de rotas que pulam). A inclusão do middleware no router de cards é o **único ponto** de uso atual; outros routers só ganharão quando adotarem `InjectPrincipalFromHeader`.

## Critérios de Sucesso

- `go build ./...` PASS.
- `go test ./internal/card/... -v` PASS (testes existentes do router não regridem).
- `go test -tags=integration ./internal/identity/infrastructure/http/server/middleware/... -v` PASS para os 2 cenários do integration test.
- `task lint` PASS. `task lint:user-isolation` PASS.
- Inspeção visual do diff de `router.go`: 1 linha nova (`sub.Use(...)`), antes do existente `InjectPrincipalFromHeaderWithO11y`.
- Webhooks (whatsapp, kiwify) NÃO foram tocados — confirmado por diff.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Integration test: gateway válido → handler executa; gateway inválido → 401 + `auth_events` row registrada
- [ ] Diff manual confirma ordem da chain conforme ADR-004
- [ ] Smoke test webhooks continua passando

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/identity/module.go` (modificado)
- `internal/card/infrastructure/http/server/router.go` (modificado)
- `internal/card/infrastructure/http/server/router_test.go` (modificado se houver expectativa de chain)
- `cmd/server/server.go` (modificado — wiring do CardRouter)
- `internal/identity/infrastructure/http/server/middleware/require_gateway_auth_integration_test.go` (novo)
