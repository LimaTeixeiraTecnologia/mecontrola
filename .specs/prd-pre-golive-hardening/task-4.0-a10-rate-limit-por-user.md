# Tarefa 4.0: A10 — Rate limit por user_id em rotas autenticadas

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Generaliza o middleware existente `internal/onboarding/.../middleware/rate_limit.go` para aceitar `KeyExtractor func(*http.Request) string`. Adiciona variante `ByUserID` que lê `principal.UserID.String()` do context (após `InjectPrincipalFromHeader`). Plug em rotas autenticadas (cards hoje; futuras quando adotarem `Principal`).

<requirements>
- RF-27: generalização com `KeyExtractor` mantendo `ByIP` default (zero regressão em onboarding)
- RF-28: plug em `/api/v1/cards*` após `RequireUser`
- RF-29: envs `AUTH_RATE_LIMIT_PER_USER_PER_MIN` (default 120), `AUTH_RATE_LIMIT_PER_USER_BURST` (default 60)
- RF-30: testes unit cobrindo ByIP (legacy), ByUserID (novo), ByUserIDFallbackIP (combo)
- RF-31: métrica `auth_rate_limit_exceeded_total{scope}` com `scope` ∈ {`ip`, `user`}. Sem `user_id` em label.
- RF-32–34: skills, gates, sem nova dep
- Zero comentário em `.go`
- Generalização preserva 100% do comportamento atual em onboarding (cobertura por testes existentes não pode regredir)
</requirements>

## Subtarefas

- [ ] 4.1 Refatorar `internal/onboarding/.../middleware/rate_limit.go` (ou extrair para pacote compartilhado `internal/platform/ratelimit/`) introduzindo `KeyExtractor` e `RateLimitConfig`.
- [ ] 4.2 Implementar `ByIP`, `ByUserID`, `ByUserIDFallbackIP` como funções `KeyExtractor`. `ByUserID` consulta `auth.FromContext(r.Context())`.
- [ ] 4.3 Adicionar métrica `auth_rate_limit_exceeded_total{scope}` no construtor do middleware.
- [ ] 4.4 Envs novas em `configs/config.go` com defaults (120/60).
- [ ] 4.5 Atualizar callers existentes em onboarding para usar `ByIP` explicitamente (validar zero mudança de comportamento via testes).
- [ ] 4.6 Plug em `internal/card/infrastructure/http/server/router.go` após `RequireUser`, antes do `idempotencyMiddleware`. Ordem na chain: `RequireGatewayAuth → InjectPrincipal → RequireUser → RateLimitByUser → idempotency`.
- [ ] 4.7 Testes unit cobrindo os 3 extractors (RF-30) + integration test do flow autenticado com mock de Principal.

## Detalhes de Implementação

Ver techspec seção "Design de Implementação > Interfaces Chave > A10". A generalização **deve preservar** o comportamento atual de onboarding sem mudança visível (regressão zero, validada por testes existentes).

## Critérios de Sucesso

- `go test ./internal/onboarding/... -v` PASS sem nenhuma mudança nos testes existentes do rate-limit.
- `go test ./internal/platform/ratelimit/... -v` PASS para os 3 extractors.
- `go test -tags=integration ./internal/card/... -v` PASS cobrindo 429 por user.
- `task lint && task test && task vulncheck` PASS.
- Inspeção: 0 comentários em `.go`.

## Skills Necessárias

<!-- MANDATÓRIO -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Tabela de 3 extractors
- [ ] Regressão zero em onboarding
- [ ] Integration test do flow autenticado

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/onboarding/.../middleware/rate_limit.go` (modificado ou extraído)
- `internal/platform/ratelimit/middleware.go` (novo se extrair)
- `internal/platform/ratelimit/extractors.go` (novo se extrair)
- `internal/card/infrastructure/http/server/router.go` (modificado)
- `configs/config.go` (modificado)
- Testes correspondentes
