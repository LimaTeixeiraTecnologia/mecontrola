# Runbook: Rate Limit do Webhook WhatsApp (B7)

## Visão Geral

O endpoint `POST /api/v1/whatsapp/inbound` possui rate limit por IP configurável via variáveis de ambiente. O middleware é aplicado **antes** da validação HMAC para mitigar DoS CPU-bound.

## Configuração

| Variável | Padrão | Descrição |
|---|---|---|
| `WHATSAPP_WEBHOOK_RATE_LIMIT_PER_MIN` | `600` | Requisições permitidas por minuto por IP |
| `WHATSAPP_WEBHOOK_RATE_LIMIT_BURST` | `100` | Burst máximo instantâneo por IP |

## Diagrama da Chain

```
POST /api/v1/whatsapp/inbound
  -> rate-limit (IP)       [B7 — novo]
  -> raw body buffer        [existente]
  -> HMAC validation        [existente]
  -> dedup                  [existente]
  -> handler                [existente]
```

## Métricas

- `whatsapp_webhook_rate_limit_exceeded_total`: contador incrementado a cada 429 retornado.

Alerta recomendado:

```promql
rate(whatsapp_webhook_rate_limit_exceeded_total[5m]) > 1
```

Dispara page se sustentado por 10 minutos.

## Diagnóstico

### Muitos 429 inesperados

1. Verificar se o proxy reverso (Caddy) está repassando o IP real do cliente via `X-Real-IP` ou `X-Forwarded-For`.
2. Verificar se o CIDR do proxy está na lista de `ONBOARDING_TRUSTED_PROXIES` (o middleware de rate limit usa a mesma lógica de extração de IP).
3. Aumentar temporariamente `WHATSAPP_WEBHOOK_RATE_LIMIT_PER_MIN` se a Meta estiver enviando volume legítimo acima do limite.

### Whitelist de IPs Meta (pós-go-live opcional)

A Meta publica os CIDRs dos seus servidores em `https://developers.facebook.com/docs/graph-api/webhooks/getting-started`. Após go-live, considere:

1. Obter os CIDRs oficiais da Meta.
2. Adicionar lógica de bypass no middleware para IPs da Meta (requer mudança de código — avaliar risco de bypass completo vs ajuste de limite).
3. Alternativa mais segura: aumentar o burst para acomodar picos legítimos da Meta sem whitelist.

## Ajuste de Limites

Redeploy necessário após alterar as variáveis. Sem reinicialização de estado (em memória), o novo limite entra em vigor imediatamente após o restart.

## Rollback

Setar `WHATSAPP_WEBHOOK_RATE_LIMIT_PER_MIN=0` não desabilita o limite (valor inválido cai para padrão). Para desabilitar completamente, é necessário remover o middleware da chain via deploy de código.
