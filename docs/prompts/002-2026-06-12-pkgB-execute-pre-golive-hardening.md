# Prompt 002 — 2026-06-12 — Pacote B — Executar PRD Pre Go-Live Hardening

**PRD:** `.specs/prd-pre-golive-hardening/`
**Skill orquestradora:** `execute-all-tasks`
**Total de tarefas:** 8 (cobertura 34/34 RFs)
**Paralelismo seguro:** 1.0–5.0 (Go cirurgico) ‖ 6.0–8.0 (infra/ops)
**Severidade:** bloqueante operacional pre go-live
**Esforco estimado:** ~3 dias uteis com paralelismo

---

## Invocacao

```
/execute-all-tasks .specs/prd-pre-golive-hardening/
```

---

## Mandatorios inegociaveis (aplicar a TODAS as 8 tarefas)

1. **Skill obrigatoria**: carregar `.claude/skills/go-implementation/SKILL.md` (Etapas 1-5, R0-R7, R-ADAPTER-001) em toda subtarefa Go (tarefas 1.0-5.0).
2. **ZERO COMENTARIOS em codigo Go de producao** (R-ADAPTER-001.1). Excecoes apenas: `//go:build`, `//nolint:` com justificativa, `// Code generated`. Caddyfile e shell sao linguagens proprias — comentarios permitidos com moderacao operacional.
3. **Domain Modeling Made Functional** (Wlaschin) onde fizer sentido:
   - B2 (timestamp WhatsApp): nao introduzir VO novo se for trivial — mas se houver invariante reusavel (janela ±5min), considerar smart constructor.
   - A10 (rate limit por user): `KeyExtractor func(*http.Request) string` como tipo nomeado e discriminated `Scope` ("ip" | "user") seguindo DMMF.
   - B6 (CORS guard): validacao no construtor (`Config.Validate()`) — invariante no boot.
4. **Padronizar com `internal/transactions`** (baseline canonico):
   - Repository SQL com `user_id` no `WHERE` em mutacoes (gate `task lint:user-isolation` ja em CI).
   - Smart constructors em VOs.
   - Adapter fino R-ADAPTER-001.2 nos middlewares modificados.
5. **Regra de memoria — sem abstracao de tempo**: `time.Now().UTC()` apenas inline. Sem `Clock`.
6. **Foco**: MVP robusto, eficiente, economico, production-ready/proof, sem falso positivo, inegociavel.

## Decisoes pre-cravadas no plano-fonte (NAO reabrir)

- B2: janela 5 min, 200 OK silencioso, `reason="stale_webhook"` ou `"invalid_webhook_timestamp"`.
- B3: TLS Caddy 1.2+, headers HSTS/nosniff/Referrer-Policy/Permissions-Policy/X-Frame-Options DENY, bloqueio de `/admin`/`/debug/pprof`/`/metrics` externo, strip de `X-User-ID`/`X-Gateway-Auth`/`X-Gateway-Timestamp`.
- B4: pg_dump cifrado com `age`, restore em container efemero, smoke queries em 3 tabelas criticas, cron mensal staging.
- B5: ufw default deny incoming, allow 22/80/443 apenas, SSH sem senha (PasswordAuthentication no).
- B6: production sem `CORS_ALLOWED_ORIGINS` ou com `*` -> erro de boot.
- B7: defaults 600/min + burst 100; reuso de middleware existente.
- A10: `ByIP` legacy + `ByUserID` novo + `ByUserIDFallbackIP` combo; metrica `auth_rate_limit_exceeded_total{scope}`.
- A2/A4: fallback CORS nunca retorna `*` em production; header `Server:` ausente.

## Coordenacao com Pacote A (cutover atomico)

- B3 (Caddyfile strip de `X-Gateway-Auth`) deve coincidir com deploy do Pacote A (gate `RequireGatewayAuth`).
- Apos merge de Pacote A: ampliar migration `000015` ou criar `000016` para CHECK de `reason` com os valores de B2.

## Gates obrigatorios pos cada tarefa

- `task lint && task test && task vulncheck` PASS.
- `task lint:user-isolation` PASS.
- `grep R-ADAPTER-001.1` vazio nos `.go` produção.
- Sem nova dep em `go.mod`.

## Validacao final do pacote

- B2: smoke local — POST payload com timestamp -6min -> 200 + `auth_events.reason="stale_webhook"`.
- B3: `curl -I https://<staging>/healthz` mostra 5 security headers; `curl -I .../debug/pprof` -> 404.
- B4: `bash deployment/scripts/pg-restore-smoke.sh` exit 0 em staging; cron mensal disparado.
- B5: `nmap` externo so 22/80/443; SSH com senha rejeitado.
- B6: app boot em `ENVIRONMENT=production CORS_ALLOWED_ORIGINS=` -> erro com mensagem clara.
- B7: `hey -n 1000 -c 50 https://.../api/v1/whatsapp/inbound` atinge 429 antes de saturar CPU.
- A10: cliente excedendo limit por user retorna 429; metrica `auth_rate_limit_exceeded_total{scope="user"}` incrementa.
- A2/A4: `curl -I /healthz` sem header `Server:`; production rejeita CORS `*`.

## Tarefas-fonte

1. `task-1.0-b6-cors-guard.md`
2. `task-2.0-b2-timestamp-whatsapp.md`
3. `task-3.0-b7-rate-limit-whatsapp.md`
4. `task-4.0-a10-rate-limit-por-user.md`
5. `task-5.0-a2-a4-hardening-cors-server.md`
6. `task-6.0-b3-caddyfile-hardening.md`
7. `task-7.0-b5-firewall-vps.md`
8. `task-8.0-b4-restore-backup.md`
