# Tarefa 6.0: `internal/infrastructure/http` — server + middleware stack + health endpoints + Problem Details mapper

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Compor o `devkit-go/pkg/http_server` em `internal/infrastructure/http` provendo o servidor HTTP com **stack completo de middlewares production-ready** (ADR-008): RequestID, Recovery, Timeout, BodyLimit, SecurityHeaders, **CORS estrito (allowlist via env)**, **OTel HTTP instrument**, **Problem Details translator** consumindo o mapper criado em `internal/infrastructure/errors`. Handlers de **`/health`, `/ready`, `/live`** (RF-01). Cobre **RF-01** integralmente.

<requirements>
- Factory `NewServer(cfg *configs.Config, deps Deps) (*http.Server, error)` onde `Deps` carrega `*database.Manager` (de 5.0) e `*observability.Provider` (de 4.0).
- Middleware stack default do devkit-go: RequestID, Recovery, Timeout (25s default), BodyLimit (1 MiB), SecurityHeaders (HSTS, X-Frame-Options DENY, X-Content-Type-Options nosniff, Referrer-Policy, CSP mínimo).
- CORS allowlist via env `CORS_ALLOWED_ORIGINS` (lista separada por vírgula, parseada em `internal/infrastructure/http/middleware.go`); **default vazio em prod** = rejeita qualquer origin; **sem `*` jamais**.
- OTel HTTP instrument envolvendo handler raiz.
- Handlers:
  - `GET /health` → JSON `{"status":"ok","version":"<sha>"}` retorna 200 sempre que processo vivo.
  - `GET /live` → mesmo que `/health` (semântica distinta para Fly probe).
  - `GET /ready` → executa `database.Manager.HealthCheck(ctx)`; 200 se OK; 503 com `ProblemDetails` se não.
- `internal/infrastructure/errors/problem.go` — `ToProblemDetails(err) (ProblemDetails, int)` cobrindo sentinels conhecidos (`database.ErrConnection` → 503; `validator.ValidationErrors` → 400 com extensions; `context.DeadlineExceeded` → 504; default → 500 sem stack).
- Integration test cobrindo: `/ready` 200 quando DB up; `/ready` 503 quando DB down; CORS rejeitando origin não-allowlisted; SecurityHeaders presentes na resposta.
</requirements>

## Subtarefas

- [ ] 6.1 Criar `internal/infrastructure/errors/problem.go` com `ProblemDetails` + `ToProblemDetails(err) (ProblemDetails, int)`.
- [ ] 6.2 Criar `internal/infrastructure/errors/problem_test.go` table-driven cobrindo cada sentinel + default 500.
- [ ] 6.3 Criar `internal/infrastructure/http/server.go` com `NewServer(cfg, deps) (*http.Server, error)` compondo middlewares default do devkit-go.
- [ ] 6.4 Criar `internal/infrastructure/http/middleware.go` com `CORSAllowlist(origins []string) func(http.Handler) http.Handler` parseando env.
- [ ] 6.5 Criar `internal/infrastructure/http/health.go` com handlers `/health`, `/live`, `/ready` registrados antes de qualquer prefixo `/api`.
- [ ] 6.6 Criar `internal/infrastructure/http/server_test.go` (unit, com mock de `database.Manager` e `observability.Provider`).
- [ ] 6.7 Criar `internal/infrastructure/http/http_integration_test.go` com tag `//go:build integration`: sobe `postgres:16-alpine` via testcontainers, valida `/ready` 200 → 503 ao derrubar DB; valida CORS allowlist + SecurityHeaders.
- [ ] 6.8 Concretizar a interface temporária do `Subsystem` HTTP em `runtime.Bootstrap` (de 3.0) usando este `NewServer`.

## Detalhes de Implementação

Ver techspec §"Pontos de Integração" + §"Endpoints de API" + §"Estratégia de Erros" + ADR-004 + ADR-008.

## Critérios de Sucesso

- `go build ./internal/infrastructure/http/... ./internal/infrastructure/errors/...` compila.
- `curl -s localhost:8080/health` retorna `{"status":"ok",...}` 200.
- `curl -s -o /dev/null -w "%{http_code}" localhost:8080/ready` retorna 200 com DB up, 503 com DB down.
- `curl -H "Origin: http://malicious.com" -i localhost:8080/health` rejeita por CORS (sem `Access-Control-Allow-Origin`).
- Response inclui `Strict-Transport-Security`, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`.
- Erro retornado em formato `application/problem+json` (ProblemDetails).
- `go test -tags=integration ./internal/infrastructure/http/...` verde.
- Cobre RF-01 integralmente.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: `problem_test.go` (table-driven sentinel → status); `server_test.go` (mock manager — `/ready` 200/503; CORS allowlist parser).
- [ ] Testes de integração: `http_integration_test.go` testcontainers — derruba DB e valida 503.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/infrastructure/http/server.go`
- `internal/infrastructure/http/middleware.go`
- `internal/infrastructure/http/health.go`
- `internal/infrastructure/http/server_test.go`
- `internal/infrastructure/http/http_integration_test.go`
- `internal/infrastructure/errors/problem.go`
- `internal/infrastructure/errors/problem_test.go`
- `internal/infrastructure/runtime/bootstrap.go` (binding concreto do subsistema HTTP)
- `go.mod`, `go.sum`
