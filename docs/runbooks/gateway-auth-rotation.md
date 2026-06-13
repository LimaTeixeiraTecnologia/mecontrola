# Runbook: Rotacao do Secret Gateway Auth

Checklist passo a passo para rotacao sem downtime do `IDENTITY_GATEWAY_SHARED_SECRET` (ADR-002)
e cutover atomico de primeiro deploy (ADR-005).

## Pre-requisitos

- Acesso SSH ao host de producao (VPS Hostinger)
- Acesso ao `.env` do container Go
- Contato com operador da LLM para coordenar janela
- Dashboard "Auth Module" aberto: `docs/dashboards/auth-module.json`

## Rotacao de secret (zero-downtime)

### Passo 1 — Gerar novo secret

```bash
openssl rand -hex 32
# Exemplo de output: a3f8c2e1d4b5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1
```

Guardar o valor gerado. Sera o `NEXT`.

### Passo 2 — Provisionar NEXT no host

No `.env` de producao (ou via `docker secret` / variavel de ambiente):

```bash
# Adicionar ou atualizar:
IDENTITY_GATEWAY_SHARED_SECRET_NEXT=<valor-gerado-no-passo-1>
```

`IDENTITY_GATEWAY_SHARED_SECRET_CURRENT` permanece inalterado.

### Passo 3 — Reload graceful do app Go

```bash
docker compose restart mecontrola-app
# ou, se configurado com SIGHUP:
docker kill --signal=SIGHUP mecontrola-app
```

Verificar que o app subiu: `curl -s https://api.mecontrola.app/healthz | jq .`

### Passo 4 — Atualizar cliente LLM

Coordenar com operador da LLM para trocar o secret usado no calculo do HMAC pelo novo valor (`NEXT`).

O app aceita tanto `CURRENT` quanto `NEXT` simultaneamente:
- Match com `CURRENT` → `result=valid`
- Match com `NEXT` → `result=rotated`

### Passo 5 — Monitorar migracao

No Grafana (painel "Gateway Auth Result"):

```promql
sum by (result) (rate(identity_gateway_auth_total[5m]))
```

Aguardar `result="rotated"` subir e `result="valid"` cair a 0. Isso confirma que todo o trafego ja usa o novo secret.

### Passo 6 — Promover NEXT para CURRENT

Quando `result="valid" == 0`:

```bash
# No .env:
IDENTITY_GATEWAY_SHARED_SECRET_CURRENT=<valor-do-NEXT>
IDENTITY_GATEWAY_SHARED_SECRET_NEXT=   # esvaziar
```

Reload graceful:

```bash
docker compose restart mecontrola-app
```

Verificar: `result="valid"` sobe novamente; `result="rotated"` cai a 0.

### Passo 7 — Validacao final

```bash
# Smoke test com novo secret:
TIMESTAMP=$(date +%s)
USER_ID="00000000-0000-0000-0000-000000000000"
SECRET="<novo-CURRENT>"
CANONICAL="${USER_ID}.${TIMESTAMP}"
SIG=$(echo -n "$CANONICAL" | openssl dgst -sha256 -hmac "$SECRET" | awk '{print $2}')

curl -v \
  -H "X-User-ID: $USER_ID" \
  -H "X-Gateway-Timestamp: $TIMESTAMP" \
  -H "X-Gateway-Auth: $SIG" \
  https://api.mecontrola.app/api/v1/cards
```

Resposta esperada: `200` (ou `401` com `reason=invalid_signature` se credentials do user invalidas, mas nao gateway).

---

## Primeiro deploy (cutover atomico — ADR-005)

### Pre-deploy

- [ ] Provisionar `IDENTITY_GATEWAY_SHARED_SECRET_CURRENT` no host (minimo 32 bytes, hex ou ASCII)
- [ ] `IDENTITY_GATEWAY_SHARED_SECRET_NEXT` vazio (primeira ativacao, sem rotacao ativa)
- [ ] Cliente LLM em modo shadow: assina e envia headers, mas servidor antigo (sem middleware) ignora
- [ ] Vetor de teste fixo validado em staging (ver `docs/runbooks/gateway-auth.md` secao "Vetor de teste fixo")

### Deploy

- [ ] Deploy atomico: app Go com `RequireGatewayAuth` + Caddyfile com strip de headers externos, mesma janela
- [ ] Verificar boot: `docker logs mecontrola-app | tail -20`
- [ ] Verificar healthz: `curl -s https://api.mecontrola.app/healthz | jq .`

### Validacao E2E imediata

- [ ] Curl externo sem headers → deve retornar `401`:
  ```bash
  curl -s -o /dev/null -w "%{http_code}" https://api.mecontrola.app/api/v1/cards
  # esperado: 401
  ```
- [ ] Curl com headers validos via gateway → deve retornar `200` (ou `401` por autenticacao de usuario, nao de gateway):
  ```bash
  # Calcular SIG conforme passo 7 da rotacao acima
  curl -v -H "X-User-ID: $USER_ID" -H "X-Gateway-Timestamp: $TIMESTAMP" -H "X-Gateway-Auth: $SIG" \
    https://api.mecontrola.app/api/v1/cards
  ```
- [ ] `identity_gateway_auth_total{result="valid"} >= 1` no Grafana em 5 min

### Criterio de rollback

Se qualquer das condicoes abaixo ocorrer nos primeiros 5 min:

- ≥ 10 requests com `result="invalid_signature"` ou `result="missing_header"`
- App retornando 500 em `/healthz`
- LLM reportando 100% de 401

Acao:

```bash
git revert HEAD
# Deploy do commit anterior (app + Caddyfile)
docker compose up -d
```

LLM continua enviando headers (idempotente; servidor antigo ignora) — rollback e simetrico.

---

## Referencias

- ADR-002 (rotacao): `.specs/prd-gateway-auth-forensics/adr-002-secret-rotation.md`
- ADR-005 (cutover atomico): `.specs/prd-gateway-auth-forensics/adr-005-rollout-cutover.md`
- Runbook geral: `docs/runbooks/gateway-auth.md`
- Dashboard: `docs/dashboards/auth-module.json`
