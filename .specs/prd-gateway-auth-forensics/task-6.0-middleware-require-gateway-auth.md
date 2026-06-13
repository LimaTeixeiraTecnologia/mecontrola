# Tarefa 6.0: Middleware RequireGatewayAuth fino + métricas + spans

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementa o adapter HTTP `RequireGatewayAuth` em `internal/identity/infrastructure/http/server/middleware/`. Adapter fino (R-ADAPTER-001.2): lê headers → invoca workflow puro `VerifyGatewayRequest` (tarefa 2.0) → match exaustivo no `Kind` → 200/next ou 401 + use case `RecordGatewayAuthFailure` (tarefa 5.0). Métricas Prometheus e span OTel cobrindo cada resultado. Sem regra de negócio inline.

<requirements>
- RF-02: lê `X-Gateway-Auth` (hex) e `X-Gateway-Timestamp` (unix decimal)
- RF-03: 401 sem detalhe; body `{"error":"unauthorized"}`; headers `Content-Type: application/json` + `Cache-Control: no-store`; sem `WWW-Authenticate`
- RF-09: métrica `identity_gateway_auth_total{result}` com 6 valores possíveis; sem `user_id` label
- RF-10: histograma `identity_gateway_auth_duration_seconds` com buckets `[0.0001, 0.0005, 0.001, 0.002, 0.005, 0.01, 0.05]`
- RF-13: zero SQL, zero branching de domínio, zero cálculo de HMAC inline (delega ao workflow)
- Switch exaustivo sobre `GatewayAuthResultKind` — listar 5 variantes, sem `default`
- `time.Now().UTC()` inline; passado por argumento ao workflow
- Span OTel `auth.require_gateway_auth` com atributos `result`, `rotated`, `has_user_id`; sem `user_id`, sem `signature`
- Log estruturado em falha: `slog.Warn` com `request_id`, `client_ip`, `result`, `user_id_prefix` (primeiros 8 chars)
- Zero comentário em `.go`
</requirements>

## Subtarefas

- [ ] 6.1 Criar `internal/identity/infrastructure/http/server/middleware/require_gateway_auth.go` com struct `RequireGatewayAuthDeps{Secrets services.SecretPair; Window time.Duration; FailureLogger *usecases.RecordGatewayAuthFailure; O11y observability.Observability}` e factory `RequireGatewayAuth(deps RequireGatewayAuthDeps) func(http.Handler) http.Handler`.
- [ ] 6.2 Implementar handler: tracer span → ler 3 headers → invocar `VerifyGatewayRequest` → switch exaustivo no `Kind` → métrica + ação.
- [ ] 6.3 Helper `respondUnauthorized(w http.ResponseWriter)` com const `_errorBodyUnauthorized = []byte(...)` para escrever body sem alocação.
- [ ] 6.4 Helper `userIDPrefix(raw string) string` que retorna `raw[:8]` ou string vazia se inválido (não panic).
- [ ] 6.5 Em falha (`MissingHeader`, `StaleTimestamp`, `InvalidSignature`): chamar `deps.FailureLogger.Handle(ctx, dto)` propagando ctx + DTO; se publish falhar, log warn e continuar com 401 (não quebrar response).
- [ ] 6.6 Teste unitário cobrindo: Valid → next, Rotated → next, cada falha → 401 + use case chamado, body fixo, `Cache-Control: no-store`, métrica incrementada.
- [ ] 6.7 Mockery para o use case `RecordGatewayAuthFailure` regenerado.

## Detalhes de Implementação

Ver techspec seção "Design de Implementação > Adapter middleware" + ADR-006 (política de erro) + ADR-004 (ordem na chain — relevante para tarefa 7.0).

## Critérios de Sucesso

- `go test ./internal/identity/infrastructure/http/server/middleware/... -run "RequireGatewayAuth" -v -cover` cobertura ≥ 85%.
- Inspeção do diff: middleware tem ≤ 80 LoC; sem SQL; sem branching sobre estado de domínio (apenas sobre `Kind`).
- Switch no middleware tem 5 cases explícitos, sem `default`.
- `task lint` PASS. `task test` PASS.
- `grep -E "QueryContext|ExecContext|db\.Query|db\.Exec" internal/identity/infrastructure/http/server/middleware/require_gateway_auth.go` retorna vazio.
- `grep -rn --include="*.go" --exclude="*_test.go" "^[[:space:]]*//" internal/identity/infrastructure/http/server/middleware/require_gateway_auth.go | grep -Ev "(//go:|//nolint:|// Code generated)"` retorna vazio.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Teste unitário com `httptest.NewRecorder` cobrindo cada uma das 5 variantes
- [ ] Teste que valida headers + body fixo + status na resposta
- [ ] Teste de span e métrica (assert que foram emitidos)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/identity/infrastructure/http/server/middleware/require_gateway_auth.go` (novo)
- `internal/identity/infrastructure/http/server/middleware/require_gateway_auth_test.go` (novo)
- `internal/identity/application/usecases/mocks/` (regenerado)
