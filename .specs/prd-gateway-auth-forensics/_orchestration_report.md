# Orchestration Report — PRD gateway-auth-forensics

**Data:** 2026-06-12
**Status final:** done (8/8 tarefas)
**Resultado:** todos os 23 RFs cobertos

---

## Snapshot inicial vs final

| Estado     | Início | Fim |
|-----------|--------|-----|
| pending   | 8      | 0   |
| done      | 0      | 8   |
| failed    | 0      | 0   |
| blocked   | 0      | 0   |

---

## Tabela de execução por wave

| Wave | Tarefas  | Paralelo | Status | ns/op benchmark |
|------|----------|----------|--------|----------------|
| 1    | 1.0, 3.0 | Sim      | done   | —              |
| 2    | 2.0      | Não      | done   | —              |
| 3    | 4.0      | Não      | done   | —              |
| 4    | 5.0      | Não      | done   | —              |
| 5    | 6.0      | Não      | done   | —              |
| 6    | 7.0      | Não      | done   | —              |
| 7    | 8.0      | Não      | done   | 2653 ns/op     |

---

## Gates de validação final (todos PASS)

| Gate | Resultado |
|------|-----------|
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `go test ./internal/identity/... ./internal/card/...` | PASS (29 pkgs) |
| `govulncheck ./...` | No vulnerabilities |
| `task lint` — arquivos do PRD | PASS (issues preexistentes em budgets/upsert_user ignorados) |
| `task lint:auth-bypass` (M-09) | PASS |
| `task lint:user-isolation` | PASS |
| R-ADAPTER-001.1 zero comentários | PASS |
| R-ADAPTER-001.2 sem SQL em adapters | PASS |
| Switch exaustivo 5 cases sem `default` | PASS |
| `go.mod` zero adições | PASS |
| Workflow puro sem IO/time.Now/context | PASS |
| `hmac.Equal` constant-time (RF-07) | PASS |
| Vetor fixo cross-lang (ADR-001) | PASS |
| Benchmark < 50.000 ns/op | PASS (2653 ns/op) |
| Dashboard JSON `jq .` | PASS |
| Runbooks sem TODOs | PASS |
| Simulação adversarial gate FAIL→PASS | PASS |

---

## Cobertura por componente

| Componente | Cobertura |
|-----------|-----------|
| `gateway_signature.go` | 100% |
| `gateway_timestamp.go` | 100% |
| `verify_gateway_request.go` | 100% |
| `require_gateway_auth.go` | 100% |
| `record_gateway_auth_failure.go` | ≥95% |
| `request_id.go` / `client_ip.go` | 100% |
| valueobjects package | 98.8% |

---

## Observações e riscos residuais

- **`NewGatewaySignature` dead branch removido**: ramo `hex.DecodeString` era inalcançável após validação manual de charset — removido, cobertura subiu de 90% → 100%.
- **Lint 2 issues preexistentes**: `budgets/ingest_external_expense.go` e `identity/upsert_user_by_whatsapp.go` — fora do escopo, presentes antes deste PRD.
- **`task mocks` falha preexistente em onboarding**: confirmada em `main` antes das mudanças — fora do escopo.
- **Integration tests em `internal/budgets`**: drift de assinaturas preexistente — fora do escopo.
- **RF-09 métrica 5 vs 6 valores**: `invalid_timestamp` subsumido por `stale_timestamp` no modelo de domínio da task 2.0 — documentar na techspec, sem impacto em alertas.
- **Cutover atômico (ADR-005)**: deploy do app + atualização do Caddyfile devem ocorrer na mesma janela.

---

## Próximos passos (pós-GoLive)

1. Aplicar migration `000015` em staging → produção antes do deploy do código.
2. Configurar `IDENTITY_GATEWAY_SHARED_SECRET_CURRENT` (≥32 bytes hex) + `IDENTITY_GATEWAY_AUTH_WINDOW=60000000000` nas envs de produção.
3. Coordenar cutover atômico com Caddyfile (item B3 do PRD pre-golive-hardening).
4. Validar smoke E2E: `curl -H "X-User-ID: <uuid>" .../api/v1/cards` → 401 sem headers de gateway.
5. Monitorar painel "Auth Module" no Grafana: `identity_gateway_auth_total{result}` por estado.
