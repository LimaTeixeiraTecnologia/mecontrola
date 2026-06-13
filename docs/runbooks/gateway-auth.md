# Runbook: Gateway Authentication (HMAC-SHA256)

## Visao geral do contrato

O middleware `RequireGatewayAuth` enforça a fronteira de confiança LLM ↔ API via HMAC-SHA256.
Toda requisição ao router `/api/v1/cards` deve carregar três headers:

| Header | Formato | Exemplo |
|---|---|---|
| `X-User-ID` | UUID v4 lowercase | `00000000-0000-0000-0000-000000000000` |
| `X-Gateway-Timestamp` | Unix seconds decimal | `1700000000` |
| `X-Gateway-Auth` | HMAC-SHA256 hex lowercase (64 chars) | `174e5aa8...babe` |

**Canonicalização (ADR-001)**

```
canonical = strings.ToLower(X-User-ID) + "." + X-Gateway-Timestamp
hmac      = hex(hmac_sha256(secret_bytes, canonical))
```

O header `X-Gateway-Timestamp` é passado como string exata (sem reparseio). O separador é `.` (ponto), escolhido por não aparecer em UUID nem em epoch decimal.

**Janela de replay (ADR-003)**

O servidor aceita timestamps dentro de ±60s do seu relógio. Requisiçoes fora da janela retornam 401 com `result=stale_timestamp`. Sem cache de nonce no MVP.

**Caddy strip (pré-requisito)**

O Caddy retira os headers `X-User-ID`, `X-Gateway-Auth` e `X-Gateway-Timestamp` de requisições externas antes de encaminhar ao app Go. Headers externos não chegam ao middleware. Requisições vindas da LLM (upstream confiável) passam com os headers intactos.

## Vetor de teste fixo

Vetor reproduzível cross-linguagem para validar implementação do cliente:

| Campo | Valor |
|---|---|
| `user_id` | `00000000-0000-0000-0000-000000000000` |
| `timestamp` | `1700000000` |
| `secret` | `test-secret-32-bytes-padding-aaaa` (UTF-8, 36 bytes) |
| `canonical` | `00000000-0000-0000-0000-000000000000.1700000000` |
| `expected_hex` | `174e5aa87139ef38ab5968b10cd88fb33d0aa084a57f30f61e3d273ad709babe` |

Verificação em Python:

```python
import hmac, hashlib
secret = b"test-secret-32-bytes-padding-aaaa"
canonical = "00000000-0000-0000-0000-000000000000.1700000000"
result = hmac.new(secret, canonical.encode(), hashlib.sha256).hexdigest()
assert result == "174e5aa87139ef38ab5968b10cd88fb33d0aa084a57f30f61e3d273ad709babe"
```

Verificação em Go (microbenchmark em `require_gateway_auth_bench_test.go`):

```go
secret := []byte("test-secret-32-bytes-padding-aaaa")
canonical := "00000000-0000-0000-0000-000000000000.1700000000"
mac := hmac.New(sha256.New, secret)
mac.Write([]byte(canonical))
got := hex.EncodeToString(mac.Sum(nil))
// got == "174e5aa87139ef38ab5968b10cd88fb33d0aa084a57f30f61e3d273ad709babe"
```

## Procedimento de rotacao

Ver checklist detalhado em `docs/runbooks/gateway-auth-rotation.md`.

Visao geral (ADR-002):

1. Gerar novo secret: `openssl rand -hex 32`
2. Provisionar em `IDENTITY_GATEWAY_SHARED_SECRET_NEXT` no host (`.env`)
3. Reload graceful do container Go
4. Atualizar cliente LLM para usar o novo secret
5. Monitorar `identity_gateway_auth_total{result="rotated"}` subir
6. Quando `result="valid"` cair para 0: promover `NEXT` → `CURRENT`, esvaziar `NEXT`, reload

## Plano de rollout cutover (ADR-005)

Ver `docs/runbooks/gateway-auth-rotation.md` para o checklist completo.

Resumo atomico:

1. **Pre-deploy**: provisionar `IDENTITY_GATEWAY_SHARED_SECRET_CURRENT` no host; cliente LLM em modo shadow (assina e envia, servidor ainda nao enforça).
2. **Deploy atomico**: app Go + Caddyfile na mesma janela. App sobe em modo enforce. Caddy strip ativo.
3. **Validacao E2E imediata**:
   - `curl -v https://api.mecontrola.app/api/v1/cards` sem headers → deve retornar `401`
   - `curl` com headers validos via gateway → deve retornar `200`
4. **Criterio de rollback**: ≥ 10 requests com `result="invalid_signature"` ou `result="missing_header"` em 5 min → revert imediato (revert commit app + Caddy).

**Sem feature flag de enforce on/off** (ADR-005 rejeitou soft-launch por criar estado inseguro silencioso).

## Troubleshooting por result

| `result` | Causa | Acao |
|---|---|---|
| `missing_header` | Um dos tres headers ausente | Verificar se cliente LLM envia todos os headers; verificar se Caddy nao esta strippando em upstream |
| `stale_timestamp` | `X-Gateway-Timestamp` fora da janela ±60s | Sincronizar NTP no host da LLM; verificar drift de relogio |
| `invalid_signature` | HMAC calculado incorretamente | Comparar canonical com vetor de teste fixo acima; verificar encoding do secret (bytes vs string); verificar separador `.` |
| `valid` | Autenticacao OK com CURRENT | Operacao normal |
| `rotated` | Autenticacao OK com NEXT | Rotacao em progresso; aguardar migracao total do cliente antes de promover |

**Diagnostico rapido via logs**

```bash
docker logs mecontrola-app 2>&1 | grep "gateway auth failed"
```

Campos logados: `result`, `request_id`, `client_ip`, `user_id_prefix` (primeiros 8 chars). Nunca logado: `signature`, `secret`, `user_id` completo.

**Diagnostico via metrica**

```promql
sum by (result) (rate(identity_gateway_auth_total[5m]))
```

## Alertas operacionais

| Alerta | Condicao | Severidade | Acao |
|---|---|---|---|
| `GatewayAuthFailureHigh` | `rate(identity_gateway_auth_total{result=~"invalid_.*\|stale_.*\|missing_.*"}[5m]) > 1` por 10 min | page | Verificar logs; acionar troubleshooting por result acima |
| `GatewayAuthRotationPending` | `increase(identity_gateway_auth_total{result="rotated"}[1h]) > 0` por 7 dias | info | Concluir promocao NEXT → CURRENT |
| `GatewayAuthDown` | `absent(identity_gateway_auth_total)` por 5 min | critical | App pode estar down; verificar healthz |

Regras Prometheus definidas em `docs/alerts/gateway-auth.yaml` (criado na segunda onda do plano-fonte).

## Referencias

- ADR-001: `.specs/prd-gateway-auth-forensics/adr-001-hmac-canonicalization.md`
- ADR-002: `.specs/prd-gateway-auth-forensics/adr-002-secret-rotation.md`
- ADR-003: `.specs/prd-gateway-auth-forensics/adr-003-replay-window.md`
- ADR-004: `.specs/prd-gateway-auth-forensics/adr-004-middleware-chain-order.md`
- ADR-005: `.specs/prd-gateway-auth-forensics/adr-005-rollout-cutover.md`
- Checklist de rotacao: `docs/runbooks/gateway-auth-rotation.md`
- Dashboard: `docs/dashboards/auth-module.json`
- Gate CI: `deployment/scripts/lint-auth-bypass.sh`
