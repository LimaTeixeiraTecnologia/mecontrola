# Runbook de Segurança: Rate-limit Meta Webhook e Acesso ao pg-tunnel

Atualizado: 2026-07-03 | Ref: RF-21, RF-22, REQ-08 (prd-infra-evolucao-kvm2-10k)

## Resumo

Este runbook documenta duas políticas de superfície implementadas para endurecer o ambiente de produção:

1. **Allowlist de IPs da Meta** no rate-limit do webhook WhatsApp (RF-21)
2. **Bind do pg-tunnel restrito ao loopback** (RF-22)

---

## 1. Rate-limit × Webhooks Meta (RF-21)

### Problema

O rate-limit por IP (Caddy + middleware Go) pode estrangular rajadas legítimas de webhooks enviados pela Meta, pois a Meta entrega mensagens de poucos blocos de IP com alta frequência durante picos de uso.

### Solução implementada

**Dupla camada de allowlist:**

#### Caddy (deployment/caddy/Caddyfile e Caddyfile.ratelimit)

Nas rotas `/api/v1/whatsapp/*`, requisições de IPs nos blocos Meta (matcher `@meta_webhook`) são roteadas diretamente para o backend sem passar pelo snippet `(rate-limit)`. Requisições de outros IPs continuam sujeitas ao rate-limit global (100 eventos/s, burst 200).

#### Middleware Go (internal/onboarding/infrastructure/http/server/middleware/rate_limit.go)

O `RateLimiter` aceita `allowlistCIDRs` variadic. IPs na allowlist passam direto sem consumir tokens do bucket. Configurado via `WHATSAPP_WEBHOOK_META_ALLOWLIST_CIDRS` (CSV de CIDRs).

### CIDRs oficiais da Meta para webhooks

Fonte: https://developers.facebook.com/docs/graph-api/webhooks/ip-addresses

| CIDR | Descrição |
|------|-----------|
| 31.13.24.0/21 | Meta (Facebook) AS32934 |
| 31.13.64.0/18 | Meta (Facebook) AS32934 |
| 45.64.40.0/22 | Meta (Facebook) AS32934 |
| 66.220.144.0/20 | Meta (Facebook) AS32934 |
| 69.63.176.0/20 | Meta (Facebook) AS32934 |
| 69.171.224.0/19 | Meta (Facebook) AS32934 |
| 74.119.76.0/22 | Meta (Facebook) AS32934 |
| 103.4.96.0/22 | Meta (Facebook) AS32934 |
| 129.134.0.0/17 | Meta (Facebook) AS32934 |
| 157.240.0.0/17 | Meta (Facebook) AS32934 |
| 173.252.64.0/18 | Meta (Facebook) AS32934 |
| 179.60.192.0/22 | Meta (Facebook) AS32934 |
| 185.60.216.0/22 | Meta (Facebook) AS32934 |
| 204.15.20.0/22 | Meta (Facebook) AS32934 |

**Verificar anualmente** ou sempre que a Meta anunciar mudança de infra: https://developers.facebook.com/docs/graph-api/webhooks/ip-addresses

### Atualizar a allowlist

1. Verificar novos CIDRs no link oficial da Meta acima.
2. Atualizar `WHATSAPP_WEBHOOK_META_ALLOWLIST_CIDRS` em `deployment/config/prod.env` (CSV).
3. Atualizar o matcher `@meta_webhook` no `Caddyfile` e `Caddyfile.ratelimit`.
4. Atualizar o default em `configs/config.go` (`setWhatsAppDefaults`).
5. Atualizar a tabela acima neste runbook.
6. Aplicar via deploy normal (não requer restart manual do Caddy — o deploy recria o container).

### Verificar que a allowlist está ativa

```bash
# Confirmar env no container server
docker exec $(docker ps -q -f name=mecontrola_server) env | grep WHATSAPP_WEBHOOK_META_ALLOWLIST_CIDRS

# Testar que IP Meta não recebe 429 (substitua pelo IP real de um host Meta)
curl -v -X POST https://<APP_DOMAIN>/api/v1/whatsapp/inbound \
  --resolve '<APP_DOMAIN>:443:157.240.1.1' \
  -H 'X-Hub-Signature-256: ...' \
  -d '{}'
```

### Proteção anti-abuso preservada

IPs não listados nos CIDRs Meta continuam sujeitos ao rate-limit padrão:
- **Caddy**: 100 eventos/s por IP, burst 200, HTTP 429 ao exceder
- **Middleware Go**: `WHATSAPP_WEBHOOK_RATE_LIMIT_PER_MIN` (padrão 600/min), burst `WHATSAPP_WEBHOOK_RATE_LIMIT_BURST` (padrão 100)

---

## 2. pg-tunnel: bind loopback (RF-22)

### Problema

O serviço `pg-tunnel` (socat) em `compose.swarm.yml` publicava `0.0.0.0:15432`, expondo o port de acesso ao banco para todas as interfaces de rede do host. A mitigação dependia apenas do ufw.

### Solução implementada

```yaml
ports:
  - target: 5432
    published: "127.0.0.1:15432"
    protocol: tcp
    mode: host
```

O bind `127.0.0.1:15432` garante que o port só é acessível do próprio host (loopback). O ufw permanece como segunda camada de defesa.

### Acesso DBA ao banco em produção

Para acesso administrativo ao banco de produção:

```bash
# A partir do host de produção (via SSH):
psql -h 127.0.0.1 -p 15432 -U <usuario> -d mecontrola

# A partir de máquina local (via SSH tunnel):
ssh -L 15432:127.0.0.1:15432 root@<VPS_IP> -N
psql -h 127.0.0.1 -p 15432 -U <usuario> -d mecontrola
```

**O acesso direto externo ao port 15432 não é mais possível** (nem que o ufw falhe). Apenas via SSH com credencial do host.

### Verificar que o bind está restrito

```bash
# No host de produção (via SSH):
ss -tlnp | grep 15432
# Deve mostrar: 127.0.0.1:15432

# Confirmar que não está em 0.0.0.0
ss -tlnp | grep 15432 | grep "0.0.0.0" && echo "FAIL: bind aberto" || echo "OK: bind loopback"
```

### Situação de ociosidade

O `pg-tunnel` está configurado com `replicas: 1`. Para remover completamente quando ocioso:

```bash
docker service scale mecontrola_pg-tunnel=0
```

Para restaurar acesso de emergência:

```bash
docker service scale mecontrola_pg-tunnel=1
```

---

## Referências

- PRD: `.specs/prd-infra-evolucao-kvm2-10k/prd.md` (RF-21, RF-22)
- Techspec: `.specs/prd-infra-evolucao-kvm2-10k/techspec.md` (REQ-08)
- Task: `.specs/prd-infra-evolucao-kvm2-10k/task-8.0-superficie-meta-pgtunnel.md`
- Middleware: `internal/onboarding/infrastructure/http/server/middleware/rate_limit.go`
- Wiring: `cmd/server/whatsapp_wiring.go`
- Configs: `deployment/caddy/Caddyfile`, `deployment/caddy/Caddyfile.ratelimit`
- Compose: `deployment/compose/compose.swarm.yml`
