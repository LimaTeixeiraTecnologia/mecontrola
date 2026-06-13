# Prompt 001 — 2026-06-12 — Pacote A — Executar PRD Gateway Authentication + Forensics

**PRD:** `.specs/prd-gateway-auth-forensics/`
**Skill orquestradora:** `execute-all-tasks`
**Total de tarefas:** 8 (cobertura 23/23 RFs)
**Paralelismo seguro:** 1.0 ‖ 3.0
**Severidade:** bloqueante critico de seguranca pre go-live
**Esforco estimado:** ~5 dias uteis

---

## Invocacao

```
/execute-all-tasks .specs/prd-gateway-auth-forensics/
```

---

## Mandatorios inegociaveis (aplicar a TODAS as 8 tarefas)

1. **Skill obrigatoria**: carregar `.claude/skills/go-implementation/SKILL.md` (Etapas 1-5, R0-R7, R-ADAPTER-001) em toda subtarefa que toque codigo Go.
2. **ZERO COMENTARIOS em codigo Go de producao** (R-ADAPTER-001.1). Excecoes apenas: `//go:build`, `//nolint:` com justificativa na mesma linha, `// Code generated`. Cabecalho de politica em `system_event_allowlist.go` (ADR-004) e a unica excecao adicional justificada.
3. **Domain Modeling Made Functional** (Wlaschin) aplicado em tudo que fizer sentido:
   - Smart constructors para `GatewaySignature`, `GatewayTimestamp`, `RequestID`, `ClientIP` — invariante NO construtor, nao em middleware.
   - Discriminated union `GatewayAuthResult` com 5 variantes sealed (`Valid`, `Rotated`, `InvalidSignature`, `StaleTimestamp`, `MissingHeader`); switch exaustivo no consumidor sem `default`.
   - Workflow puro `VerifyGatewayRequest` em `domain/services/` — sem IO, sem `context`, sem `time.Now()`, sem `uuid.New()` interno.
   - Adapter middleware fino (R-ADAPTER-001.2): parse header -> workflow puro -> match -> 200/next ou 401.
4. **Padronizar com `internal/transactions`** (baseline canonico):
   - `Decide*` puros em `domain/services/` recebendo `time.Time` e `uuid.UUID` por argumento.
   - Repository SQL com `WHERE ... AND user_id = $N` em toda mutacao (defesa em profundidade — gate `task lint:user-isolation` ja em CI).
   - Smart constructors em VOs (`internal/transactions/domain/valueobjects/money.go` como referencia).
   - Producers que apenas serializam, sem calculo de dominio.
   - Eventos tipados em `entities/events.go` carregando `UserID uuid.UUID`.
5. **Regra de memoria — sem abstracao de tempo**: `time.Now().UTC()` apenas inline no caller (use case ou middleware). Sem `Clock` interface, sem `now func() time.Time` global.
6. **Foco**: MVP robusto, eficiente, economico, production-ready/proof, sem falso positivo, inegociavel.

## Decisoes pre-cravadas em 8 ADRs (NAO reabrir durante execucao)

- ADR-001: canonical HMAC = `strings.ToLower(userIDRaw) + "." + timestampRaw`.
- ADR-002: rotacao via env duplicada `_CURRENT`/`_NEXT` (padrao WhatsApp/Kiwify).
- ADR-003: janela replay 60s sem cache de nonce (replay aceito; mitigado por Idempotency-Key).
- ADR-004: ordem chain `RequireGatewayAuth -> InjectPrincipal -> RequireUser -> idempotency`.
- ADR-005: rollout cutover atomico (sem soft-launch, sem feature flag).
- ADR-006: 401 opaco `{"error":"unauthorized"}` + metrica granular por `result`.
- ADR-007: tabela de rotas que pulam o gateway (webhooks intactos).
- ADR-008: sanitizacao XFF pega ULTIMO IP, validado com `net.ParseIP`.

## Gates obrigatorios pos cada tarefa

- `task lint && task test && task vulncheck` PASS.
- `task lint:user-isolation` PASS (defesa em profundidade per-user).
- `grep R-ADAPTER-001.1` sobre novos `.go` retorna vazio.
- Cobertura de teste ≥ 95% em workflow puro, ≥ 85% em middleware.
- Diff de `go.mod`: zero adicoes (stdlib + devkit-go + libs ja em uso).

## Validacao final do pacote (apos task 8.0)

- `task lint:auth-bypass` PASS (gate M-09 inegociavel).
- Simulacao adversarial do gate: revert + FAIL + restore + PASS documentado.
- Smoke E2E (plano-fonte secao 9 itens 1, 7): `curl -H "X-User-ID: <uuid>" .../api/v1/cards` retorna 401 sem `X-Gateway-Auth` valido.
- Microbenchmark `BenchmarkRequireGatewayAuth_Valid` < 50µs ns/op.
- Painel Grafana "Auth Module" mostra `identity_gateway_auth_total{result}` por estado.
- Runbook `docs/runbooks/gateway-auth.md` e `gateway-auth-rotation.md` revisados sem TODOs.

## Tarefas-fonte

1. `task-1.0-config-smart-constructors.md`
2. `task-2.0-discriminated-union-workflow-puro.md`
3. `task-3.0-migration-auth-events-forensics.md`
4. `task-4.0-forensics-extractors-establish-principal.md`
5. `task-5.0-record-gateway-auth-failure.md`
6. `task-6.0-middleware-require-gateway-auth.md`
7. `task-7.0-cabeamento-plug-router-cards.md`
8. `task-8.0-gate-lint-runbook-dashboard-benchmark.md`
