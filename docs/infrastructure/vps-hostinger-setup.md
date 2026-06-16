# Setup: VPS Hostinger para MeControla

**Última revisão:** 2026-06-15
**Cobre:** spec mínima, provisionamento, hardening, deploy inicial.
**Não cobre:** rotação de secrets (ver `deployment/runbooks/rotate-secret.md`), restore PITR
(ver `deployment/runbooks/restore-vps.md`).

## Spec mínima

| Recurso | Mínimo recomendado | Justificativa |
|---------|---------------------|----------------|
| CPU | 4 vCPU (KVM ou similar) | Postgres + server + worker + Caddy + observability |
| RAM | 8 GB | Postgres `shared_buffers=2GB`, `effective_cache=6GB`, server/worker 256-512MB cada |
| Disco | 100 GB NVMe SSD | OS + Docker images + PG data + WAL + backups locais staging |
| Bandwidth | Ilimitada (Hostinger oferece) | Backups S3, OTLP traces, webhooks |
| SO | Ubuntu 22.04 LTS (preferido) ou Debian 12 | Validado em `vps-hardening.sh` |

**Plano Hostinger recomendado:** **VPS 2** (4 vCPU, 8 GB RAM, 100 GB NVMe) — ~R$ 60/mês.

## Provisionamento

### 1. Criar VPS no painel Hostinger

1. Login em https://hpanel.hostinger.com.
2. **VPS Hosting → Set up → Linux KVM**.
3. Distribuição: **Ubuntu 22.04 LTS**.
4. Adicionar chave SSH pública (gerar com `ssh-keygen -t ed25519` se não tiver).
5. Confirmar provisão (5-10 min).

### 2. Primeiro acesso SSH

```sh
ssh root@<ip-do-vps>
# Aceitar a fingerprint na primeira conexão.

# Atualizar sistema:
apt-get update && apt-get upgrade -y
apt-get install -y curl wget git ca-certificates gnupg lsb-release
```

### 3. Criar usuário deploy (não-root)

```sh
adduser --disabled-password --gecos "" deploy
usermod -aG sudo deploy
mkdir -p /home/deploy/.ssh
cp /root/.ssh/authorized_keys /home/deploy/.ssh/
chown -R deploy:deploy /home/deploy/.ssh
chmod 700 /home/deploy/.ssh
chmod 600 /home/deploy/.ssh/authorized_keys

# Permitir sudo sem senha (uso de CI/CD)
echo "deploy ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/90-deploy
```

Testar do laptop:
```sh
ssh deploy@<ip-do-vps> 'whoami && sudo whoami'
# Esperado: deploy / root
```

### 4. Instalar Docker

```sh
# Como deploy:
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc

echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] \
  https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo $VERSION_CODENAME) stable" \
  | sudo tee /etc/apt/sources.list.d/docker.list >/dev/null

sudo apt-get update
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

sudo usermod -aG docker deploy
# Re-login para grupo docker aplicar:
exit
ssh deploy@<vps>
docker run hello-world
```

### 5. Clone do repo

```sh
sudo mkdir -p /opt/mecontrola
sudo chown deploy:deploy /opt/mecontrola
git clone https://github.com/LimaTeixeiraTecnologia/mecontrola.git /opt/mecontrola
cd /opt/mecontrola
```

### 6. Executar hardening

```sh
sudo bash deployment/scripts/vps-hardening.sh
```

Esse script:
- Instala/configura fail2ban (SSH + Caddy)
- Instala/configura unattended-upgrades (segurança automática)
- Endurece SSH (PasswordAuthentication=no, PermitRootLogin=prohibit-password)
- Cria swapfile 2GB com swappiness=10
- Configura UFW (allow 22, 80, 443; deny resto)

**Pré-requisito:** chave SSH já em `/home/deploy/.ssh/authorized_keys` (passo 3 acima).
Senão o script aborta para evitar lockout.

### 7. Configurar `.env` de produção

```sh
sudo cp .env.example /opt/mecontrola/.env
sudo chmod 600 /opt/mecontrola/.env
sudo chown deploy:deploy /opt/mecontrola/.env
sudo nano /opt/mecontrola/.env
# Substituir TODOS os CHANGE_ME_* — gerar com:
#   openssl rand -hex 32   (gateway secret, DB password)
#   openssl rand -hex 16   (token encryption key)
#   openssl rand -hex 20   (kiwify webhook secret)
```

Checklist de variáveis críticas:

- `ENVIRONMENT=production`
- `APP_DOMAIN=api.mecontrola.app.br`
- `CADDY_EMAIL=devops@mecontrola.app.br`
- `DB_PASSWORD=<forte>`
- `DB_SSL_MODE=disable` (interno entre containers — VPN-equivalente)
- `IDENTITY_GATEWAY_SHARED_SECRET_CURRENT=<hex-64>`
- `ONBOARDING_TOKEN_ENCRYPTION_KEY=<32-chars-exatos>`
- `KIWIFY_*` (ver `docs/integrations/kiwify-setup.md`)
- `META_*` (ver `docs/integrations/whatsapp-setup.md`)
- `PGBACKREST_S3_*` (config backup S3 Hostinger Object Storage ou AWS S3)
- `LOKI_URL`, `OTEL_EXPORTER_OTLP_ENDPOINT` (Grafana Cloud — gratuito até 50GB/mês)

Validar antes de subir:
```sh
grep "CHANGE_ME" /opt/mecontrola/.env && echo "FAIL: ainda há CHANGE_ME" || echo "OK"
```

### 8. Configurar DNS

Ver `docs/infrastructure/dns-setup.md`.

### 9. Primeiro deploy

```sh
cd /opt/mecontrola
export IMAGE_TAG=$(git rev-parse --short HEAD)
docker pull ghcr.io/limateixeiratecnologia/mecontrola:${IMAGE_TAG}

docker compose \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  up -d

# Verificar:
docker compose ... ps
# Esperado: postgres healthy, server up, worker up, caddy up

# Logs:
docker compose ... logs -f --tail=50
```

### 10. Configurar pgBackRest (backup S3)

```sh
sudo bash deployment/scripts/pgbackrest-setup.sh
# Esse script:
# - Instala pgbackrest no container postgres
# - Cria stanza inicial
# - Roda primeiro full backup
# - Instala crontab (full Domingo 01:00, diff Seg-Sáb 01:00)
```

Validar:
```sh
docker compose ... exec postgres pgbackrest --stanza=mecontrola info
```

### 11. Habilitar observabilidade

A telemetria (OTel + Prometheus + Loki) já sobe junto via `compose.prod.yml`. Para Grafana
Cloud:

1. Criar conta gratuita em https://grafana.com/products/cloud/
2. Anotar `LOKI_URL`, `LOKI_USER_ID`, `LOKI_API_KEY` no `.env`.
3. Anotar `OTEL_EXPORTER_OTLP_ENDPOINT` (Tempo) idem.
4. Restart server + worker para puxar nova config.

### 12. Smoke test pós-deploy

```sh
# Health
curl https://api.mecontrola.app.br/health
# Esperado: 200 {"status":"healthy",...}

# TLS válido
echo | openssl s_client -connect api.mecontrola.app.br:443 \
  -servername api.mecontrola.app.br 2>/dev/null | openssl x509 -noout -dates

# Postman: importar collection + environment, ajustar base_url para
#   https://api.mecontrola.app.br, rodar folder 01-Health.
```

## Manutenção

### Atualização rolling

```sh
ssh deploy@<vps>
cd /opt/mecontrola
git pull origin main
export IMAGE_TAG=$(git rev-parse --short HEAD)
docker pull ghcr.io/limateixeiratecnologia/mecontrola:${IMAGE_TAG}
docker compose ... up -d --no-deps --force-recreate server worker
```

### Verificar saúde

```sh
docker compose ... ps
docker stats --no-stream
df -h
free -h
```

### Logs

```sh
docker compose ... logs -f --tail=100 server
docker compose ... logs -f --tail=100 worker
```

### Reboot da VPS

Hostinger oferece reboot via painel ou:
```sh
sudo reboot
# Aguardar ~30s e re-SSH
docker compose ... ps
# Containers com restart=unless-stopped voltam sozinhos
```

## Custos esperados (estimativa 100 users)

| Item | Custo mensal |
|------|--------------|
| VPS Hostinger VPS 2 | R$ 60 |
| Cloudflare (free tier) | R$ 0 |
| Grafana Cloud (50GB free) | R$ 0 |
| S3 backups (~5GB) | R$ 5 |
| Meta WhatsApp (1000 msgs free + utility) | R$ 150 |
| Kiwify (5,99% + R$1 por tx) | proporcional à receita |
| **Total fixo** | **~R$ 215/mês** |

## Referências

- Hardening: `deployment/scripts/vps-hardening.sh`
- Compose prod: `deployment/compose/compose.prod.yml`
- Restore PITR: `deployment/runbooks/restore-vps.md`
- DNS: `docs/infrastructure/dns-setup.md`
- Rotação de secrets: `deployment/runbooks/rotate-secret.md`
