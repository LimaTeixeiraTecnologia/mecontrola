# Hostinger Production Deploy — MeControla

> Data: 2026-06-16
> Objetivo: subir o MVP do MeControla em VPS Hostinger production-ready em **menos de 2 horas**, do zero (VPS recém-criada) ao smoke `task mvp:telegram:drive` verde apontando para o domínio público.
> Audiência: operador (você ou outra pessoa). Cada passo é copy-pasteable, cada seção tem critério de sucesso verificável.
> Premissa: o repositório já está auditado e o MVP foi validado localmente — ver `docs/runbooks/2026-06-15-mvp-local-end-to-end.md` e `docs/runbooks/2026-06-16-telegram-e2e-quickstart.md`.

## Convenções

- Comandos prefixados com `# (LOCAL)` rodam na sua máquina.
- Comandos prefixados com `# (VPS)` rodam no servidor Hostinger via SSH.
- `<placeholder>` deve ser substituído antes de colar.
- Todo bloco `verificação` tem critério objetivo (HTTP 200, número de containers, linha em log). Se a verificação falhar, **não** prossiga — consulte Seção 13 (Troubleshooting).
- Domínio de referência neste runbook: `mecontrola.app.br` (substitua pelo seu).

---

## 0. Pré-requisitos

Confirme cada item antes de começar. Se algum estiver vermelho, resolva primeiro — não há atalho.

| Item | Como confirmar |
|---|---|
| VPS Hostinger ativa, **KVM 2** ou superior (2 vCPU, 8 GB RAM, 100 GB SSD) | Painel hPanel → VPS → status `Running` |
| Imagem da VPS: **Ubuntu 22.04 LTS** ou **Debian 12** | hPanel → VPS → OS Information |
| Domínio registrado e DNS apontando para o IP da VPS (record `A`) | `dig +short mecontrola.app.br` retorna o IP exato da VPS |
| Acesso SSH como `root` com chave pública instalada (sem password) | `ssh -i ~/.ssh/<key> root@<IP> 'whoami'` retorna `root` |
| Conta Resend ativa, chave `re_...` em mãos | https://resend.com/api-keys |
| Conta Kiwify com webhook configurável (Admin → Webhooks) | painel Kiwify acessível |
| Bot Telegram criado via @BotFather, token + secret token em mãos | siga Fase 1 de `docs/runbooks/2026-06-16-telegram-e2e-quickstart.md` |
| Conta Backblaze B2 ou AWS S3 com bucket criado para backups | painel B2/S3 abre o bucket |
| Chave OpenRouter (`AGENT_MODE=openrouter`) com créditos | https://openrouter.ai/keys |
| GitHub PAT com escopo `read:packages` (para `docker pull ghcr.io/...`) | `gh auth status` |
| (Opcional) Conta Cloudflare proxiada — neste runbook NÃO usaremos proxy CF na frente do Caddy para não conflitar com Let's Encrypt HTTP-01 |  |
| (Opcional) Meta WhatsApp Business — este runbook **não exige WhatsApp**; subimos canal Telegram-only |  |

### Verificação

```bash
# (LOCAL)
dig +short mecontrola.app.br
ssh -i ~/.ssh/<key> root@<IP> 'cat /etc/os-release | grep PRETTY_NAME'
```

Saída esperada:

```
<IP-DA-VPS>
PRETTY_NAME="Ubuntu 22.04.x LTS"
```

Se `dig` retornar vazio ou IP diferente: aguarde propagação DNS (até 24 h para record novo) antes de prosseguir.

---

## 1. Preparação do VPS (hardening)

### 1.1. Login inicial

```bash
# (LOCAL)
ssh -i ~/.ssh/<key> root@<IP>
```

### 1.2. Atualização base

```bash
# (VPS)
apt-get update && apt-get -y upgrade
apt-get -y install ca-certificates curl gnupg git ufw rsync jq
```

### 1.3. Instalar Docker Engine + plugin compose

```bash
# (VPS)
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg

echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
  > /etc/apt/sources.list.d/docker.list

apt-get update
apt-get -y install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

systemctl enable --now docker
```

### Verificação

```bash
# (VPS)
docker --version
docker compose version
docker run --rm hello-world | grep -i 'working correctly'
```

Saída esperada:

```
Docker version 27.x.x ...
Docker Compose version v2.x.x
Hello from Docker!
```

### 1.4. Clonar repositório (precisa do `vps-hardening.sh`)

```bash
# (VPS)
mkdir -p /opt
git clone https://github.com/LimaTeixeiraTecnologia/mecontrola.git /opt/mecontrola
cd /opt/mecontrola
git rev-parse --short HEAD
```

### 1.5. Hardening (script idempotente)

O script `deployment/scripts/vps-hardening.sh` já existe no repo. Ele configura: `fail2ban`, `unattended-upgrades`, SSH (PasswordAuthentication=no, MaxAuthTries=3), swap 2 GB, sysctl swappiness=10, **UFW** (allow 22/80/443).

> Antes de rodar: confirme que sua chave pública SSH está em `/root/.ssh/authorized_keys`. O script aborta se não houver `authorized_keys` para evitar lockout.

```bash
# (VPS)
ls -lah /root/.ssh/authorized_keys
bash /opt/mecontrola/deployment/scripts/vps-hardening.sh
```

### Verificação

```bash
# (VPS)
fail2ban-client status sshd | grep 'Currently banned'
ufw status verbose | grep -E '22/tcp|80/tcp|443/tcp'
swapon --show
sshd -t && echo 'sshd config OK'
```

Saída esperada:

```
`- Currently banned: 0
22/tcp                     ALLOW IN    Anywhere                   # SSH
80/tcp                     ALLOW IN    Anywhere                   # HTTP (Caddy/ACME)
443/tcp                    ALLOW IN    Anywhere                   # HTTPS
NAME       TYPE SIZE USED PRIO
/swapfile  file   2G   0B   -2
sshd config OK
```

### 1.6. Validar SSH em segundo terminal (NÃO feche o atual)

```bash
# (LOCAL, em outro terminal)
ssh -i ~/.ssh/<key> root@<IP> 'echo connected'
```

Se falhar, restaure SSH a partir do backup criado pelo script: `cp /etc/ssh/sshd_config.bak.<data> /etc/ssh/sshd_config && systemctl restart ssh`.

---

## 2. Configurar `.env` de produção

### 2.1. Layout no servidor

```bash
# (VPS)
cd /opt/mecontrola
cp .env.example .env
chown root:root .env
chmod 600 .env
```

### 2.2. Gerar secrets

Rode no LOCAL ou na VPS — qualquer ambiente com `openssl`.

```bash
# (ANYWHERE)
echo "DB_PASSWORD=$(openssl rand -hex 24)"
echo "KIWIFY_WEBHOOK_SECRET=$(openssl rand -hex 32)"
echo "ONBOARDING_TOKEN_ENCRYPTION_KEY=$(openssl rand -hex 32)"
echo "TELEGRAM_SECRET_TOKEN=$(openssl rand -hex 32)"
echo "IDENTITY_GATEWAY_SHARED_SECRET_CURRENT=$(openssl rand -hex 32)"
echo "GRAFANA_ADMIN_PASSWORD=$(openssl rand -base64 24)"
```

Cole os valores em `/opt/mecontrola/.env`. Em paralelo, anote em **gestor de senhas** (1Password/Bitwarden) — perder a `ONBOARDING_TOKEN_ENCRYPTION_KEY` em produção **invalida todos os tokens já emitidos**.

### 2.3. Variáveis críticas (todas obrigatórias em `ENVIRONMENT=production`)

Edite `/opt/mecontrola/.env` garantindo:

```dotenv
# ---- Aplicação ----
ENVIRONMENT=production
APP_MODE=server
PORT=8080

# ---- Domínio / Caddy ----
APP_DOMAIN=mecontrola.app.br
CADDY_EMAIL=ops@mecontrola.app.br

# ---- Imagem (digest imutável, NÃO use latest) ----
IMAGE_NAME=ghcr.io/limateixeiratecnologia/mecontrola
IMAGE_TAG=<sha-curto-do-commit-que-quer-deployar>

# ---- Banco ----
DB_HOST=pgbouncer
DB_PORT=6432
DB_USER=mecontrola
DB_PASSWORD=<openssl rand -hex 24>
DB_NAME=mecontrola_db
DB_SSL_MODE=disable
DATABASE_URL=postgres://mecontrola:<DB_PASSWORD>@postgres:5432/mecontrola_db?sslmode=disable

# ---- CORS ----
CORS_ALLOWED_ORIGINS=https://www.mecontrola.app.br,https://mecontrola.app.br
ONBOARDING_CHECKOUT_CORS_ORIGINS=https://www.mecontrola.app.br,https://mecontrola.app.br
ONBOARDING_TRUSTED_PROXIES=127.0.0.1/32,::1/128,172.16.0.0/12

# ---- Email (Resend em produção; Hostinger não envia SMTP confiável) ----
EMAIL_PROVIDER=resend
EMAIL_FROM_ADDRESS=noreply@mecontrola.app.br
EMAIL_FROM_NAME=MeControla
EMAIL_ACTIVATE_URL=https://mecontrola.app.br/activate
RESEND_API_KEY=re_<sua-chave>
RESEND_BASE_URL=https://api.resend.com
EMAIL_HTTP_TIMEOUT=10s

# ---- Kiwify ----
KIWIFY_CLIENT_ID=<do painel Kiwify>
KIWIFY_CLIENT_SECRET=<do painel Kiwify>
KIWIFY_ACCOUNT_ID=<do painel Kiwify>
KIWIFY_PRODUCT_ID_MONTHLY=<uuid do produto mensal>
KIWIFY_PRODUCT_ID_QUARTERLY=<uuid do produto trimestral>
KIWIFY_PRODUCT_ID_ANNUAL=<uuid do produto anual>
KIWIFY_WEBHOOK_SECRET=<openssl rand -hex 32>
KIWIFY_WEBHOOK_TOKEN_HEADER=X-Kiwify-Webhook-Token

# ---- Telegram (Telegram-only MVP) ----
TELEGRAM_ENABLED=true
TELEGRAM_BOT_TOKEN=<BotFather>
TELEGRAM_BOT_ID=<numérico de getMe>
TELEGRAM_BOT_USERNAME=<sem @>
TELEGRAM_SECRET_TOKEN=<openssl rand -hex 32>
TELEGRAM_WEBHOOK_PATH=/api/v1/channels/telegram/webhook
ONBOARDING_TELEGRAM_DIRECT_ENABLED=true

# ---- WhatsApp (opcional — manter CHANGE_ME provoca falha de boot) ----
# Para deploy Telegram-only, preencha META_* com valores dummy estáveis (não vazios).
META_PHONE_NUMBER_ID=000000000000000
META_ACCESS_TOKEN=disabled
META_APP_SECRET=disabled-app-secret-32-chars-min-aaa
META_VERIFY_TOKEN=disabled-verify-token

# ---- Agent / LLM ----
AGENT_MODE=openrouter
OPENROUTER_API_KEY=<sua-chave>
AGENT_LLM_PRIMARY_MODEL=google/gemini-2.5-flash-lite

# ---- Budgets / Alertas ----
BUDGETS_THRESHOLD_ALERTS_MODE=job

# ---- Gateway HMAC ----
IDENTITY_GATEWAY_SHARED_SECRET_CURRENT=<openssl rand -hex 32>
IDENTITY_GATEWAY_AUTH_WINDOW=60s

# ---- Onboarding ----
ONBOARDING_TOKEN_ENCRYPTION_KEY=<openssl rand -hex 32>

# ---- Backup pgBackRest (S3/B2) ----
PGBACKREST_S3_ENDPOINT=s3.us-east-005.backblazeb2.com
PGBACKREST_S3_BUCKET=mecontrola-prod-backups
PGBACKREST_S3_REGION=us-east-005
PGBACKREST_S3_KEY=<key-id>
PGBACKREST_S3_KEY_SECRET=<key-secret>

# ---- Backup lógico (rclone + age) ----
BACKUP_REMOTE=b2:mecontrola-prod-backups
AGE_RECIPIENT=age1<chave-pública-age>
AGE_KEY_FILE=/etc/age/key.txt
RETENTION_DAYS=30

# ---- Observabilidade Grafana Cloud (opcional mas recomendado) ----
LOKI_URL=https://logs-prod-013.grafana.net/loki/api/v1/push
LOKI_USER_ID=<numérico>
LOKI_API_KEY=glc_<token>
GRAFANA_ADMIN_USER=admin
GRAFANA_ADMIN_PASSWORD=<openssl rand -base64 24>

# ---- AlertManager SMTP ----
ALERTMANAGER_FROM_EMAIL=alerts@mecontrola.app.br
ALERTMANAGER_TO_EMAIL=oncall@mecontrola.app.br
ALERTMANAGER_SMTP_HOST=smtp.resend.com
ALERTMANAGER_SMTP_USER=resend
ALERTMANAGER_SMTP_PASSWORD=<RESEND_API_KEY>
```

### Verificação (zero `CHANGE_ME`)

```bash
# (VPS)
chmod 600 /opt/mecontrola/.env
stat -c '%a %U:%G %n' /opt/mecontrola/.env
grep -c CHANGE_ME /opt/mecontrola/.env
grep -E '^(APP_DOMAIN|EMAIL_PROVIDER|TELEGRAM_ENABLED|AGENT_MODE|BUDGETS_THRESHOLD_ALERTS_MODE|DB_HOST|DB_PORT|ENVIRONMENT|IMAGE_TAG)=' /opt/mecontrola/.env
```

Saída esperada:

```
600 root:root /opt/mecontrola/.env
0
APP_DOMAIN=mecontrola.app.br
EMAIL_PROVIDER=resend
TELEGRAM_ENABLED=true
AGENT_MODE=openrouter
BUDGETS_THRESHOLD_ALERTS_MODE=job
DB_HOST=pgbouncer
DB_PORT=6432
ENVIRONMENT=production
IMAGE_TAG=<sha>
```

Se `grep -c CHANGE_ME` retornar > 0: corrija antes de prosseguir. `Config.Validate()` em `ENVIRONMENT=production` aborta o boot.

### 2.4. Verificar `IMAGE_TAG` existe no GHCR

```bash
# (VPS)
echo "<GHCR_PAT>" | docker login ghcr.io -u <github-user> --password-stdin
docker manifest inspect ghcr.io/limateixeiratecnologia/mecontrola:${IMAGE_TAG:-$(grep '^IMAGE_TAG=' /opt/mecontrola/.env | cut -d= -f2)} >/dev/null && echo OK
```

Saída esperada: `OK`. Se falhar com `manifest unknown`: a tag não existe — rode o CI no GitHub primeiro, espere o workflow `build-image` terminar e copie o SHA.

---

## 3. Configurar Caddy + SSL automático

O `Caddyfile` em `deployment/caddy/Caddyfile` já lê `{$APP_DOMAIN}` e `{$CADDY_EMAIL}`, bloqueia `/admin`, `/debug/pprof` e `/metrics` externos, e strip de headers `X-User-ID`, `X-Gateway-Auth`, `X-Gateway-Timestamp`. Não há ação manual exceto garantir que as portas 80 e 443 estão abertas (UFW já liberou na Seção 1) e que **nenhum outro processo** está usando 80/443 no host.

### Verificação

```bash
# (VPS)
ss -tlnp | grep -E ':80|:443' || echo 'porta 80/443 livre (esperado)'
```

Saída esperada: `porta 80/443 livre (esperado)`. Se algo aparecer (ex: nginx residual), pare e desinstale antes de prosseguir.

---

## 4. Subir stack

### 4.1. Pull das imagens

```bash
# (VPS)
cd /opt/mecontrola
set -a; . ./.env; set +a
docker compose \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  pull
```

Saída esperada: cada serviço imprime `Pulled` ou `already up to date`. Nenhum `manifest unknown`.

### 4.2. Aplicar migrations

```bash
# (VPS)
docker compose \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  run --rm migrate
```

Saída esperada (últimas linhas):

```
applied migration 000001_...
...
applied migration 0000NN_...
migrate: done
```

Container exit code `0`. Se exit code != 0, leia `docker compose ... logs migrate` e consulte Seção 13.

### 4.3. Subir aplicação + reverse proxy

```bash
# (VPS)
docker compose \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  up -d postgres pgbouncer server worker caddy
```

### 4.4. Subir stack de observabilidade

```bash
# (VPS)
docker compose \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  up -d promtail otelcol prometheus alertmanager node-exporter postgres-exporter blackbox-exporter grafana
```

### Verificação

```bash
# (VPS)
docker compose -f deployment/compose/compose.yml -f deployment/compose/compose.prod.yml ps \
  --format 'table {{.Service}}\t{{.Status}}'
```

Saída esperada: todas as linhas com `Up <X> (healthy)`. Containers ainda em `starting` por mais de 90 s = problema; consulte `docker compose logs <serviço>`.

```bash
# (VPS) — healthz interno
curl -sS http://localhost/healthz -H "Host: ${APP_DOMAIN}" -k
```

Saída esperada: `{"status":"ok"...}` ou HTTP 200 sem corpo.

```bash
# (LOCAL) — SSL público, Let's Encrypt já emitiu cert
curl -sS https://mecontrola.app.br/healthz -o /dev/null -w '%{http_code}\n'
curl -sS https://mecontrola.app.br/healthz | jq .
echo | openssl s_client -connect mecontrola.app.br:443 -servername mecontrola.app.br 2>/dev/null | openssl x509 -noout -issuer -dates
```

Saída esperada:

```
200
{"status":"ok",...}
issuer=C=US, O=Let's Encrypt, CN=...
notBefore=<hoje>
notAfter=<hoje+90d>
```

Se HTTP retornar 526/525 (cert inválido): aguarde 30-60 s para Caddy concluir o ACME, depois `docker compose logs caddy | grep -i 'certificate obtained'`.

---

## 5. Webhook Telegram

### 5.1. Registrar webhook

```bash
# (LOCAL)
TOKEN='<TELEGRAM_BOT_TOKEN>'
SECRET='<TELEGRAM_SECRET_TOKEN>'
DOMAIN='mecontrola.app.br'

curl -sS -X POST "https://api.telegram.org/bot${TOKEN}/setWebhook" \
  -H 'Content-Type: application/json' \
  -d "{
    \"url\": \"https://${DOMAIN}/api/v1/channels/telegram/webhook\",
    \"secret_token\": \"${SECRET}\",
    \"allowed_updates\": [\"message\",\"callback_query\"],
    \"drop_pending_updates\": true
  }" | jq
```

### Verificação

```bash
# (LOCAL)
curl -sS "https://api.telegram.org/bot${TOKEN}/getWebhookInfo" | jq
```

Saída esperada (campos críticos):

```json
{
  "ok": true,
  "result": {
    "url": "https://mecontrola.app.br/api/v1/channels/telegram/webhook",
    "has_custom_certificate": false,
    "pending_update_count": 0,
    "last_error_date": <ausente OU >24h atrás>,
    "ip_address": "<IP-DA-VPS>"
  }
}
```

Se `last_error_message` aparecer com mensagem recente: ver Seção 13.

---

## 6. Webhook Kiwify

### 6.1. Configurar no painel Kiwify

1. Login em https://dashboard.kiwify.com.br
2. Configurações → Webhooks → **Novo webhook**
3. URL: `https://mecontrola.app.br/api/v1/billing/webhooks/kiwify`
4. Eventos: marque **todos** (compra aprovada, recusada, reembolso, chargeback, assinatura cancelada, assinatura renovada, boleto gerado, pix gerado)
5. Secret: cole o valor de `KIWIFY_WEBHOOK_SECRET` do `.env`
6. Salvar

### 6.2. Teste com payload sintético

> Atenção: este teste valida **apenas** o transporte HTTP + HMAC. O assinatura semântica de cada trigger é validada nos integration tests do CI.

```bash
# (LOCAL)
SECRET='<KIWIFY_WEBHOOK_SECRET>'
DOMAIN='mecontrola.app.br'
BODY='{"order_id":"smoke-test-001","order_status":"approved","product_id":"00000000-0000-0000-0000-000000000000"}'
SIG=$(printf '%s' "$BODY" | openssl dgst -sha1 -hmac "$SECRET" -hex | awk '{print $2}')

curl -sS -o /tmp/kiwify-resp.txt -w 'HTTP %{http_code}\n' \
  -X POST "https://${DOMAIN}/api/v1/billing/webhooks/kiwify?signature=${SIG}" \
  -H 'Content-Type: application/json' \
  -d "$BODY"
cat /tmp/kiwify-resp.txt
```

### Verificação

Saída esperada: `HTTP 202` (aceito; processamento assíncrono). HTTP 401/403 = HMAC errado, releia `KIWIFY_WEBHOOK_SECRET`. HTTP 400 = payload malformado (esperado dropar este teste antes de aprovar em produção — ele apenas valida o transporte).

```bash
# (VPS)
docker compose -f deployment/compose/compose.yml -f deployment/compose/compose.prod.yml \
  logs --tail=50 server | grep -i kiwify
```

Saída esperada: linha contendo `kiwify_webhook_received` ou `event_id=smoke-test-001`.

---

## 7. CORS allowlist

Já configurado na Seção 2 via `CORS_ALLOWED_ORIGINS` e `ONBOARDING_CHECKOUT_CORS_ORIGINS`. Wildcard (`*`) ou lista vazia provocam erro de boot em `ENVIRONMENT=production`.

### Verificação

```bash
# (LOCAL)
curl -sS -X OPTIONS "https://mecontrola.app.br/api/v1/onboarding/checkout" \
  -H 'Origin: https://www.mecontrola.app.br' \
  -H 'Access-Control-Request-Method: POST' \
  -o /dev/null -w '%{http_code} ACAO=%header{access-control-allow-origin}\n'

curl -sS -X OPTIONS "https://mecontrola.app.br/api/v1/onboarding/checkout" \
  -H 'Origin: https://evil.example.com' \
  -H 'Access-Control-Request-Method: POST' \
  -o /dev/null -w '%{http_code} ACAO=%header{access-control-allow-origin}\n'
```

Saída esperada:

```
204 ACAO=https://www.mecontrola.app.br
403 ACAO=
```

(O segundo curl deve **negar** a origem; valor de `Access-Control-Allow-Origin` deve estar vazio ou ausente.)

---

## 8. Backup configurado

### 8.1. pgBackRest (PITR contínuo para S3/B2)

```bash
# (VPS)
bash /opt/mecontrola/deployment/scripts/pgbackrest-setup.sh
```

Script idempotente. Configura `stanza-create`, `archive_command`, primeiro `backup --type=full`.

### Verificação

```bash
# (VPS)
docker compose -f deployment/compose/compose.yml -f deployment/compose/compose.prod.yml \
  exec postgres pgbackrest --stanza=mecontrola info
```

Saída esperada: bloco contendo `status: ok` e ao menos 1 backup `full`.

### 8.2. Backup lógico (pg_dump + age + rclone)

Criar a chave age em **máquina offline** ou ambiente seguro:

```bash
# (LOCAL, ambiente seguro)
age-keygen -o age-key.txt
cat age-key.txt | grep '# public key:'
```

Salve a **chave pública** (`age1...`) no `.env` da VPS (`AGE_RECIPIENT`). Salve o arquivo **privado** (`age-key.txt`) em local off-site (USB criptografado, gestor de senhas). Sem ele, restore é impossível.

Na VPS:

```bash
# (VPS)
mkdir -p /etc/age
echo '<conteúdo de age-key.txt>' > /etc/age/key.txt
chmod 600 /etc/age/key.txt
apt-get -y install age rclone
rclone config  # configurar remote 'b2' (interativo)
```

Smoke test imediato:

```bash
# (VPS)
set -a; . /opt/mecontrola/.env; set +a
bash /opt/mecontrola/deployment/scripts/backup-dump.sh
rclone ls "${BACKUP_REMOTE}" | head -5
```

### Verificação

Saída esperada: linha `mecontrola_<timestamp>.sql.gz.age` em `rclone ls`.

Cron diário 02:00 UTC:

```bash
# (VPS)
cat >/etc/cron.d/mecontrola-backup <<'EOF'
0 2 * * * root /opt/mecontrola/deployment/scripts/backup-dump.sh >> /var/log/mecontrola-backup.log 2>&1
EOF
chmod 644 /etc/cron.d/mecontrola-backup
```

### 8.3. Smoke de restore (recomendado em staging; cuidado em produção)

```bash
# (VPS) — usa porta isolada RESTORE_PORT=15432, não toca em produção
set -a; . /opt/mecontrola/.env; set +a
bash /opt/mecontrola/deployment/scripts/pg-restore-smoke.sh
```

Saída esperada: `SMOKE OK: schema mecontrola validado` ou equivalente.

> Em produção pura (sem staging), faça o smoke restore **antes** de abrir tráfego real. Se não conseguir restaurar do backup, o backup não existe.

---

## 9. Monitoramento

### 9.1. Promtail → Grafana Cloud Loki

Já configurado via variáveis `LOKI_URL`, `LOKI_USER_ID`, `LOKI_API_KEY` (Seção 2). Promtail roda como container (`profiles: []` no `compose.prod.yml`).

### Verificação

```bash
# (VPS)
docker compose -f deployment/compose/compose.yml -f deployment/compose/compose.prod.yml \
  logs --tail=50 promtail | grep -E 'level=(info|warn|error)'
```

Saída esperada: linhas com `level=info` mencionando `Adding target` para containers Docker; **nenhuma** linha `level=error msg="error sending batch"`.

Confirmar logs aterrissando no Grafana Cloud:

```bash
# (LOCAL)
curl -sS -u "${LOKI_USER_ID}:${LOKI_API_KEY}" \
  "https://logs-prod-013.grafana.net/loki/api/v1/query?query=%7Bjob%3D%22docker%22%7D&limit=5" \
  | jq '.data.result | length'
```

Saída esperada: número > 0.

### 9.2. Prometheus + Alertas mínimos

`deployment/monitoring/prometheus-rules.yaml` já está montado no Prometheus. Garanta que as regras abaixo existem (busque no arquivo) e estão ativas:

| Alerta | Expressão | Severidade |
|---|---|---|
| `AgentIntentDecodeFailures` | `rate(agent_intent_parse_decode_failed_total[5m]) / rate(agent_intent_parse_total[5m]) > 0.05` | warning |
| `ThresholdAlertDeliveryFailing` | `increase(budgets_threshold_alert_delivered_total{outcome="channel_failed"}[10m]) > 0` | critical |
| `KiwifyWebhookHmacInvalid` | `increase(kiwify_webhook_hmac_invalid_total[5m]) > 5` | warning |
| `OutboxDispatcherBacklog` | `outbox_pending_count > 1000` | warning |
| `DatabaseConnectionsExhausted` | `sum(pg_stat_activity_count) / pg_settings_max_connections > 0.85` | critical |

### Verificação

```bash
# (VPS)
curl -sS http://127.0.0.1:9090/api/v1/rules | jq '.data.groups[].rules[] | select(.type=="alerting") | .name'
```

Saída esperada: lista contendo ao menos `AgentIntentDecodeFailures` e `ThresholdAlertDeliveryFailing`. Se ausente: editar `deployment/monitoring/prometheus-rules.yaml` (fora do escopo deste runbook — abra issue), reiniciar Prometheus.

### 9.3. AlertManager → email Resend

```bash
# (VPS)
curl -sS http://127.0.0.1:9093/-/healthy
docker compose -f deployment/compose/compose.yml -f deployment/compose/compose.prod.yml \
  logs --tail=30 alertmanager | grep -iE 'ready|listen'
```

Saída esperada: `Healthy` + `level=info msg="Listening on" address=:9093`.

---

## 10. Smoke production (Telegram E2E)

### 10.1. Preparação

A versão de produção do `task mvp:telegram:drive` requer apontar para o **domínio público** em vez de ngrok local. Faça:

```bash
# (LOCAL)
cd ~/Git/mecontrola
cp .env .env.bkp.local
cat >>.env <<EOF

# ---- override smoke prod ----
TELEGRAM_BOT_TOKEN=<MESMO TOKEN DO VPS>
TELEGRAM_SECRET_TOKEN=<MESMO SECRET DO VPS>
KIWIFY_WEBHOOK_SECRET=<MESMO SECRET DO VPS>
EOF

export MECONTROLA_PROD_BASE_URL='https://mecontrola.app.br'
```

### 10.2. Executar smoke

```bash
# (LOCAL)
task mvp:telegram:drive -- --base-url "${MECONTROLA_PROD_BASE_URL}" --skip-prepare
```

### Critério único de sucesso

Você recebe no Telegram, **sem ter mandado nada**, uma mensagem do tipo:

> "Sua fatura no cartão está em R$ 4.500,00. Você já utilizou 90% do limite."

Restaure o `.env` local após smoke:

```bash
# (LOCAL)
mv .env.bkp.local .env
```

> Se `mvp:telegram:drive` não suportar `--base-url`/`--skip-prepare` no momento, abra o script `scripts/smoke/telegram_drive.sh` e substitua manualmente o host `localhost:8080` por `mecontrola.app.br`. Esta inconsistência é gap conhecido (ver "O que NÃO está coberto" no final).

---

## 11. Plano de rollback

### 11.1. Antes de cada deploy: gravar a tag corrente

```bash
# (VPS)
PREV_TAG=$(grep '^IMAGE_TAG=' /opt/mecontrola/.env | cut -d= -f2)
echo "${PREV_TAG}" > /opt/mecontrola/.last-known-good
```

### 11.2. Rollback de imagem (sem perder dados)

```bash
# (VPS)
cd /opt/mecontrola
PREV_TAG=$(cat /opt/mecontrola/.last-known-good)
sed -i "s|^IMAGE_TAG=.*|IMAGE_TAG=${PREV_TAG}|" .env

docker compose \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  pull server worker

docker compose \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  up -d --no-deps server worker
```

### Verificação

```bash
# (LOCAL)
curl -sS https://mecontrola.app.br/healthz -o /dev/null -w '%{http_code}\n'
# Esperado: 200
```

### 11.3. Rollback de migration (CUIDADO — pode perder dados)

```bash
# (VPS)
docker compose -f deployment/compose/compose.yml -f deployment/compose/compose.prod.yml \
  run --rm migrate down 1
```

> **Política**: rollback de migration em produção exige **dois operadores** (autoria + revisão) e um backup `pgbackrest` recente. Se a migration que está revertendo `DROP`a colunas ou tabelas com dados, restaure via PITR (`deployment/runbooks/restore-pitr.md`) em vez de fazer `down`.

### 11.4. Rollback total (catastrófico — restore PITR)

Siga `deployment/runbooks/restore-pitr.md` para reconstruir o cluster do backup S3/B2.

---

## 12. Checklist final

Marque cada caixa **somente** após executar a verificação correspondente nas seções acima.

```
[ ] DNS A record apontando para VPS IP (Seção 0)
[ ] VPS hardening aplicado: ufw, fail2ban, ssh keys-only (Seção 1)
[ ] /opt/mecontrola/.env com chmod 600 e zero CHANGE_ME (Seção 2)
[ ] IMAGE_TAG existe em ghcr.io (Seção 2.4)
[ ] SSL Let's Encrypt ativo (curl https://<dominio>/healthz → 200, cert issuer = Let's Encrypt) (Seção 4)
[ ] /healthz 200 público (Seção 4)
[ ] Telegram setWebhook OK + getWebhookInfo sem last_error_message recente (Seção 5)
[ ] Kiwify webhook configurado no painel + smoke HTTP 202 (Seção 6)
[ ] CORS rejeita Origin não-allowlisted (Seção 7)
[ ] Resend chave OK + 1 email teste enviado (validar manualmente: trigger 1 onboarding real ou usar dashboard Resend)
[ ] pgBackRest stanza-create + 1 backup full OK (Seção 8.1)
[ ] Backup lógico age+rclone subiu pelo menos 1 arquivo no remote (Seção 8.2)
[ ] Smoke de restore validado em staging OU PITR plano lido e ensaiado (Seção 8.3)
[ ] Logs chegando no Grafana Cloud Loki (Seção 9.1)
[ ] Alertas Prometheus configurados e visíveis em /api/v1/rules (Seção 9.2)
[ ] AlertManager healthy + SMTP Resend configurado (Seção 9.3)
[ ] task mvp:telegram:drive PRODUCTION verde — alerta proativo chega no Telegram (Seção 10)
[ ] /opt/mecontrola/.last-known-good gravado (Seção 11)
[ ] Documentado em gestor interno: IP da VPS, domínio, owner técnico, contato 24x7
[ ] Chave privada age guardada off-site (USB criptografado OU gestor de senhas)
[ ] ONBOARDING_TOKEN_ENCRYPTION_KEY guardada em gestor de senhas
[ ] /metrics, /debug/pprof, /admin retornam 404 do exterior (Seção 13 troubleshooting tem comando)
```

---

## 13. Troubleshooting

| # | Sintoma | Causa provável | Fix |
|---|---|---|---|
| 1 | `curl https://<dominio>/healthz` → 502 ou conexão recusada após `docker compose up` | Caddy ainda obtendo cert ACME, ou `server` container unhealthy | `docker compose logs caddy \| grep -i 'certificate obtained'`; aguarde 60 s. Se persistir: `docker compose logs server \| tail -50`, procurar `Config.Validate` fail. |
| 2 | `migrate` exit code != 0 com `connection refused` | `postgres` ainda iniciando ou `DB_HOST=pgbouncer` mas pgbouncer não healthy | Confirme `docker compose ps postgres pgbouncer` ambos `(healthy)`; rode `migrate` novamente. Migrations rodam contra Postgres direto (`DB_HOST=postgres` no compose.prod.yml para o job migrate), não pgbouncer. |
| 3 | OpenRouter `429 Too Many Requests` em produção | Chave sem créditos OU rate limit do tier free atingido | Recarregar saldo em https://openrouter.ai/credits; ou trocar `AGENT_LLM_PRIMARY_MODEL` para modelo mais barato e ajustar `AGENT_LLM_FALLBACK_MODELS`. |
| 4 | Resend retorna 422 `validation_error` no envio de email | `EMAIL_FROM_ADDRESS` usa domínio não verificado no Resend | Adicionar domínio em https://resend.com/domains, configurar SPF/DKIM/DMARC no painel DNS Hostinger; aguardar 1-30 min para propagar; reiniciar `server`. |
| 5 | Telegram `getWebhookInfo` mostra `last_error_message: "Wrong response from the webhook: 401 Unauthorized"` | `TELEGRAM_SECRET_TOKEN` diferente entre `setWebhook` e `.env` no servidor | Re-rodar Seção 5.1 com o **mesmo** secret que está em `.env`; reiniciar `server` se editou env. |
| 6 | Kiwify webhook chegando mas retornando 401 | `KIWIFY_WEBHOOK_SECRET` mismatch OU painel Kiwify configurado com algoritmo diferente (deve ser **SHA-1 hex**, query `?signature=`) | Confirmar no painel Kiwify que assinatura é SHA-1 hex na query string; copiar literal o secret do `.env` da VPS. |
| 7 | `/metrics` ou `/debug/pprof` acessíveis publicamente | Caddyfile carregado errado, ou request indo direto na porta do app | Verificar: `curl -sS -o /dev/null -w '%{http_code}\n' https://<dominio>/metrics` deve ser **404**. Se for 200: `docker compose exec caddy cat /etc/caddy/Caddyfile \| grep '@admin'`; rebuild Caddy. |
| 8 | Postgres OOM em VPS KVM 2 (4 GB RAM) | `shared_buffers` muito alto OU vacuum simultâneo de tabelas grandes | Confirme limit no `compose.prod.yml` (`memory: 2G` para postgres). Reduzir `shared_buffers` em `deployment/postgres/postgresql.conf` para 512MB se VPS tem 4GB. |
| 9 | `docker compose pull` falha com `denied: denied` no GHCR | PAT do GitHub expirado ou sem escopo `read:packages` | Gerar novo PAT em https://github.com/settings/tokens com escopo `read:packages`; `docker login ghcr.io` de novo. |
| 10 | UFW bloqueando tráfego inter-container | `ufw` configurado errado, regra rejeitando bridge Docker | UFW **não** deve interferir em bridge Docker. Confirme `iptables -L DOCKER-USER -n` está vazio; se houver REJECT, remova: `iptables -F DOCKER-USER`. |
| 11 | Promtail `level=error msg="error sending batch" status=401` | `LOKI_API_KEY` ou `LOKI_USER_ID` inválido para o tenant Grafana Cloud | Regerar API key em Grafana Cloud → Connections → Loki → Generate token (scope `logs:write`); atualizar `.env`; `docker compose restart promtail`. |
| 12 | Smoke `task mvp:telegram:drive` trava em "Step 4 timeout (ativação não chega)" | Bot Telegram do `.env` local é diferente do bot do VPS, OU webhook ainda apontando para ngrok antigo | Re-rodar `getWebhookInfo`, confirmar URL = `https://<dominio>/api/v1/channels/telegram/webhook`. Se URL é ngrok antiga: re-rodar Seção 5.1. |
| 13 | Caddy logs `tls: failed to verify certificate: x509: certificate signed by unknown authority` ao chamar Telegram API | CA bundle da imagem Caddy antiga | Garantir `CADDY_IMAGE=caddy:2-alpine` (ou superior); `docker compose pull caddy && docker compose up -d caddy`. |
| 14 | `docker compose ps` mostra `server (unhealthy)` em loop | `Config.Validate()` reprovou variável; container sobe e morre | `docker compose logs server \| head -50` — procurar `panic` ou `level=ERROR msg="config validation failed"`; corrija a variável apontada e re-suba. |

### Verificação rápida de exposição (rodar após cada deploy)

```bash
# (LOCAL)
for path in /metrics /debug/pprof /admin /admin/health; do
  code=$(curl -sS -o /dev/null -w '%{http_code}' "https://mecontrola.app.br${path}")
  echo "${path} → ${code}"
done
```

Saída esperada: **todas** as linhas terminam em `404`. Qualquer 200 = **bloqueio crítico** — não abra tráfego de clientes.

---

## Referências

- Runbook E2E local: `docs/runbooks/2026-06-15-mvp-local-end-to-end.md`
- Runbook Telegram quickstart: `docs/runbooks/2026-06-16-telegram-e2e-quickstart.md`
- Production readiness status: `docs/runbooks/2026-06-16-mvp-production-readiness-status.md`
- Deploy automatizado (CI/CD): `deployment/runbooks/deploy.md`
- Rollback (legado Fly.io — substituído por Seção 11 deste runbook): `deployment/runbooks/rollback.md`
- Restore PITR: `deployment/runbooks/restore-pitr.md`
- Restore total VPS: `deployment/runbooks/restore-vps.md`
- Rotação de secrets: `deployment/runbooks/rotate-secret.md`
- Hardening script: `deployment/scripts/vps-hardening.sh`
- Backup script: `deployment/scripts/backup-dump.sh`
- Smoke restore script: `deployment/scripts/pg-restore-smoke.sh`
- ADRs: ADR-009 (config), ADR-011 (Docker+VPS), ADR-012 (supply chain), ADR-013 (cosign)
