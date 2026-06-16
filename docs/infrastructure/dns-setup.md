# Setup: DNS — Cloudflare + Caddy ACME

**Última revisão:** 2026-06-15
**Domínio:** `mecontrola.app.br`
**Cobre:** landing (Cloudflare Pages) + API (VPS direto com Caddy/Let's Encrypt).

## Arquitetura DNS

```
mecontrola.app.br          → Cloudflare Pages (landing)
www.mecontrola.app.br      → Cloudflare Pages (redirect → apex)
api.mecontrola.app.br      → VPS IP (Caddy + Let's Encrypt — NÃO proxiado pelo CF)
```

⚠️ A API **NÃO usa Cloudflare proxy** para evitar problemas com:
- Webhooks de Kiwify/Meta que verificam cert end-to-end (Cloudflare termina TLS).
- Streaming OTLP.
- Mutual TLS futuro.

Caddy gera certificado Let's Encrypt diretamente.

## Etapas

### 1. Registrar domínio (já feito)

`mecontrola.app.br` está registrado. Confirmar registrar em `https://whois.registro.br/`.

### 2. Configurar Cloudflare como DNS

1. Criar conta gratuita em https://cloudflare.com.
2. **Add Site → mecontrola.app.br**.
3. Cloudflare mostra os 2 nameservers (ex: `ada.ns.cloudflare.com`, `cody.ns.cloudflare.com`).
4. No painel do Registro.br → DNS → alterar nameservers para os de Cloudflare.
5. Aguardar propagação (1-24h, geralmente 5-15 min).

Validar:
```sh
dig NS mecontrola.app.br +short
# Esperado: ada.ns.cloudflare.com cody.ns.cloudflare.com
```

### 3. Records DNS

No painel Cloudflare → DNS → Records:

| Type | Name | Content | Proxy | TTL |
|------|------|---------|-------|-----|
| CNAME | `@` (apex) | `<projeto>.pages.dev` (Cloudflare Pages) | ✅ Proxied | Auto |
| CNAME | `www` | `mecontrola.app.br` | ✅ Proxied | Auto |
| A | `api` | `<IP-DO-VPS>` | ❌ DNS only | Auto |

Substituir `<IP-DO-VPS>` pelo IP fornecido pela Hostinger (ver `infrastructure/vps-hostinger-setup.md`).

### 4. Configurar redirect www → apex

Cloudflare → **Rules → Redirect Rules → Create**:

- When: `URL: hostname equals www.mecontrola.app.br`
- Then: `Static → 301 → https://mecontrola.app.br$1`

### 5. Cloudflare Pages — vincular landing

1. Cloudflare → **Workers & Pages → Create → Pages**.
2. Connect to Git: GitHub `LimaTeixeiraTecnologia/limateixeira-landingpage`.
3. Production branch: `main`.
4. Build command: `make build` (ou `pnpm build`).
5. Output: `dist`.
6. Environment variables:
   - `PUBLIC_GA_ID` = `G-XXXXXXXXXX` (anotar GA4 Measurement ID)
7. Save & Deploy.
8. Após primeiro deploy OK, **Custom domains → Set up → mecontrola.app.br** (e `www.mecontrola.app.br`).

### 6. Caddy ACME (API)

O `Caddyfile` (`deployment/caddy/Caddyfile`) já está configurado para issuar certificado
Let's Encrypt automaticamente:

```caddy
{$APP_DOMAIN} {
    encode gzip
    log { ... }
    reverse_proxy server:8080
}
```

A variável `APP_DOMAIN` vem de `.env` (`api.mecontrola.app.br`).

Pré-requisitos para ACME funcionar:
- Porta 80 e 443 abertas no UFW (`vps-hardening.sh` já faz).
- DNS A `api.mecontrola.app.br` → IP do VPS resolvendo corretamente.
- `CADDY_EMAIL` definido no `.env` (Let's Encrypt usa para notificações de expiração).

Na primeira request HTTPS à `api.mecontrola.app.br`, Caddy:
1. Detecta que não tem cert válido.
2. Resolve challenge ACME via HTTP-01 (porta 80).
3. Obtém cert Let's Encrypt em ~30s.
4. Renova automaticamente 30 dias antes de expirar.

### 7. Validar TLS

```sh
# Conexão direta
curl -v https://api.mecontrola.app.br/health 2>&1 | grep -E "SSL|TLS|subject|issuer"

# Cert details
echo | openssl s_client -connect api.mecontrola.app.br:443 \
  -servername api.mecontrola.app.br 2>/dev/null | \
  openssl x509 -noout -subject -issuer -dates

# Esperado:
#   subject= /CN=api.mecontrola.app.br
#   issuer= /C=US/O=Let's Encrypt/CN=R3
#   notBefore=..., notAfter=...
```

### 8. Validar landing

```sh
curl -sIL https://mecontrola.app.br | head -5
# Esperado: HTTP/2 200, server: cloudflare

curl -sIL https://www.mecontrola.app.br | head -5
# Esperado: 301 Moved → https://mecontrola.app.br
```

## Monitoramento de DNS / SSL

- **Prometheus alert `SSLCertExpiring`** já configurado: alerta se cert expira em < 30 dias
  (`deployment/monitoring/prometheus-rules.yaml`).
- Cloudflare envia e-mail se nameservers caírem.
- UptimeRobot gratuito (https://uptimerobot.com) pode monitorar `https://api.mecontrola.app.br/health`
  externamente (5 min).

## Trocar IP do VPS

Caso precise migrar para outro VPS:

1. Provisionar novo VPS conforme `vps-hostinger-setup.md`.
2. Subir stack no novo VPS (apontar pgBackRest para os backups antigos para PITR).
3. Cloudflare → DNS → editar A record `api` → novo IP.
4. Aguardar propagação (5-15 min, dependendo de TTL).
5. Validar `curl https://api.mecontrola.app.br/health` no novo IP.
6. Desligar VPS antigo após 24h sem tráfego.

## Referências

- Cloudflare DNS: https://developers.cloudflare.com/dns/
- Caddy Automatic HTTPS: https://caddyserver.com/docs/automatic-https
- Registro.br nameservers: https://registro.br/tecnologia/dns
