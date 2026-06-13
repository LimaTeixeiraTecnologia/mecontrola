# ADR-006 — Política de erro 401 + métrica por result

## Metadados

- **Título:** Resposta 401 opaca + métrica e log estruturados granulares
- **Data:** 2026-06-12
- **Status:** Aceita
- **Decisores:** Operador do mecontrola
- **Relacionados:** [PRD](prd.md) RF-03, RF-08, RF-09, RF-10; [techspec](techspec.md) seção Monitoramento

## Contexto

A política de erro do middleware decide:
- O que o client recebe quando falha (vaza informação de motivo?).
- O que é registrado para forense interna.
- Como o operador detecta padrões (taxa de cada motivo).

Trade-off clássico: ergonomia para client legítimo (debug fácil) vs leak para atacante (information disclosure).

## Decisão

**Resposta ao client (uniforme para qualquer motivo de falha):**

- Status: `401 Unauthorized`.
- Body: `{"error":"unauthorized"}`.
- Headers: `Content-Type: application/json`, `Cache-Control: no-store`, **sem** `WWW-Authenticate` (gateway é interno, não user-facing).
- **Sem detalhe** do motivo (não diz se faltou header, se timestamp expirou ou se HMAC errou).

**Telemetria interna (granular):**

- Métrica Prometheus `identity_gateway_auth_total{result}` com `result` ∈ {`valid`, `rotated`, `missing_header`, `invalid_timestamp`, `stale_timestamp`, `invalid_signature`}. Sem `user_id` como label.
- Histograma `identity_gateway_auth_duration_seconds`.
- Span OTel `auth.require_gateway_auth` com atributo `result` e `rotated` (bool). Sem `user_id`, sem signature, sem timestamp raw em atributos.
- Log estruturado em falha: `slog.Warn("gateway auth failed", "result", ..., "request_id", ..., "client_ip", ..., "user_id_prefix", first8(userIDRaw))`.
- Evento outbox `auth.failed` com `reason="gateway_*"` (4 valores: `missing_header`, `invalid_timestamp`, `stale_timestamp`, `invalid_signature`).

**Sem rate-limit por falha** neste PRD (fica para A10 / segunda onda).

## Alternativas Consideradas

1. **Body com detalhe do motivo** — `{"error":"stale_timestamp"}`. **Rejeitada**: vaza para atacante "qual passo da verificação falhou", facilita oráculo. Ganho de DX zero (cliente legítimo lê log do operador, não a response).
2. **403 em vez de 401** — semântica "autenticado mas sem permissão". **Rejeitada**: a falha aqui é de autenticação (assinatura inválida ≡ identidade não provada), 401 é correto. 403 confundiria com falha de RBAC futura.
3. **204 silencioso** (engolir e logar) — útil em webhooks (não dispara retry). **Rejeitada**: aqui o caller é cliente legítimo da nossa LLM; ele PRECISA saber que falhou para corrigir. 401 com body opaco é o mínimo informativo.

## Consequências

### Benefícios Esperados

- Cliente não vaza informação para atacante.
- Operador tem visibilidade granular por motivo via métrica.
- Forense correlaciona `request_id` + `client_ip` + `result` em logs estruturados.

### Trade-offs e Custos

- Cliente legítimo precisa abrir log/dashboard interno para depurar (custo aceitável para integração interna).
- Atacante consegue distinguir 401 (auth falhou) de outros códigos (rotas válidas) — inevitável e aceitável.

### Riscos e Mitigações

- **R-01**: cliente legítimo entra em loop de 401 sem entender o motivo. **Mitigação**: runbook orienta operador a abrir dashboard "Auth Module" e filtrar por `client_ip` do cliente.
- **R-02**: tooling esquece `Cache-Control: no-store` e 401 é cacheado por proxy intermediário. **Mitigação**: header fixo no middleware + teste unitário que valida sua presença.

## Plano de Implementação

1. Constante `errorBodyUnauthorized = []byte(...)` no middleware.
2. Helper `respondUnauthorized(w http.ResponseWriter)` que escreve status + body + headers.
3. Métrica registrada via devkit-go observability (padrão do repo).
4. Span atribuído antes de `defer span.End()`.
5. Use case `RecordGatewayAuthFailure` publica outbox event.
6. Teste unitário cobre: cada `result` produz métrica correta, log correto, body fixo.

## Monitoramento e Validação

- Dashboard "Auth Module" tem painel "Gateway Auth Failures by Reason" (5 linhas, 1 por result de falha).
- Alerta: `rate(identity_gateway_auth_total{result=~"invalid_.*|stale_.*|missing_.*"}[5m]) > 1` por 10 min.

## Impacto em Documentação e Operação

- `docs/runbooks/gateway-auth.md`: troubleshooting por result.
- Dashboard JSON em `docs/dashboards/auth-module.json` (estende o existente do `prd-auth-foundation`).

## Revisão Futura

Revisar quando:
- Volume de 401 legítimo (ataque ou tooling quebrado) demandar rate-limit por IP — implementar A10.
- Migração para JWT: corpos de erro tipicamente seguem RFC 6750 com `WWW-Authenticate: Bearer error=...`.
- Data sugerida: 2027-06-12.
