# Runbook: Rotacionar Secrets na VPS (Hostinger)

**Última revisão:** 2026-06-15
**Referências:** ADR-009 (Viper + configs), ADR-013 (segurança operacional)
**Substitui:** versão anterior que documentava `flyctl` (Fly.io obsoleto — produção é VPS).

## Quando Usar

- Rotação periódica programada de credenciais (recomendado: trimestral).
- Suspeita de vazamento de secret.
- Funcionário com acesso ao `.env` saiu da empresa.
- Mudança de provedor de serviço (novo token OTLP, nova senha do banco, novo webhook secret Kiwify).

## Princípio: Zero-downtime via `_CURRENT` + `_NEXT`

Para secrets de validação assimétrica (gateway HMAC, Kiwify webhook, Meta app secret), o
sistema aceita **dois secrets simultaneamente** (`_CURRENT` e `_NEXT`). Isso permite:
1. Adicionar `_NEXT` com o novo valor.
2. Restart sem downtime — server passa a aceitar ambos.
3. Atualizar o cliente externo (painel Kiwify / Meta / sistemas internos).
4. Promover `_NEXT` para `_CURRENT` e limpar `_NEXT`.
5. Restart — agora apenas o novo é aceito.

Para secrets de uso unilateral (DB password, OTLP API key), a rotação é atômica (passos
3-4 abaixo).

---

## Pré-requisitos

```sh
# Acesso SSH à VPS
ssh deploy@<vps-host>

# No VPS:
cd /opt/mecontrola
ls -la .env       # deve ter chmod 600, owner root ou deploy
```

---

## Procedimento: Gateway Auth Secret (HMAC-SHA256)

### 1. Gerar novo secret

```sh
NEW=$(openssl rand -hex 32)
echo "NOVO secret: $NEW"
# Anotar em local seguro (1Password, Bitwarden, gerenciador da empresa).
```

### 2. Adicionar como `_NEXT`

```sh
sudo nano /opt/mecontrola/.env
# Adicionar/modificar:
#   IDENTITY_GATEWAY_SHARED_SECRET_NEXT=<NEW>
# Manter:
#   IDENTITY_GATEWAY_SHARED_SECRET_CURRENT=<antigo>
```

### 3. Reiniciar server e worker (rolling)

```sh
docker compose \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  up -d --no-deps --force-recreate server worker

# Validar:
docker compose ... logs server | grep "gateway secrets loaded"
# Esperado: "CURRENT + NEXT both active"
```

### 4. Atualizar clientes (sistemas que assinam requests)

Caso de uso típico: outro serviço interno que faz HMAC para chamar `/api/v1/identity/users`.
Atualizar o secret do lado do cliente para o novo valor.

### 5. Promover `_NEXT` para `_CURRENT`

```sh
sudo nano /opt/mecontrola/.env
# Trocar:
#   IDENTITY_GATEWAY_SHARED_SECRET_CURRENT=<NEW>
#   IDENTITY_GATEWAY_SHARED_SECRET_NEXT=      (vazio)
```

### 6. Restart final

```sh
docker compose ... up -d --no-deps --force-recreate server worker
docker compose ... logs server | tail -50
# Verificar: nenhum erro de assinatura nos últimos 5 min via auth_events
psql <conn> -c "SELECT reason, count(*) FROM auth_events WHERE created_at > now() - interval '5 minutes' GROUP BY 1;"
```

---

## Procedimento: Kiwify Webhook Secret (HMAC-SHA1)

Mesmo padrão de `_CURRENT`/`_NEXT`:

```sh
# 1. Gerar
NEW=$(openssl rand -hex 20)

# 2. Adicionar como _NEXT em .env
sudo nano /opt/mecontrola/.env
# KIWIFY_WEBHOOK_SECRET_NEXT=<NEW>

# 3. Restart
docker compose ... up -d --no-deps --force-recreate server

# 4. Atualizar no painel Kiwify:
#    Integrações → Webhooks → editar → Secret = <NEW>
#    Salvar.

# 5. Validar com botão "Testar" do painel Kiwify
docker compose ... logs server | grep "kiwify webhook"
# Deve aceitar assinaturas dos dois secrets durante o overlap.

# 6. Promover NEXT → CURRENT, limpar NEXT, restart final.
```

---

## Procedimento: Meta App Secret (HMAC-SHA256 WhatsApp)

```sh
# 1. Gerar no painel Meta: developers.facebook.com → App → Settings → Basic → App Secret → "Reset"
NEW=<copiado-do-painel>

# 2. Adicionar como _NEXT em .env
sudo nano /opt/mecontrola/.env
# META_APP_SECRET_NEXT=<NEW>

# 3. Restart
docker compose ... up -d --no-deps --force-recreate server

# 4. Aguardar 5 min — durante esse intervalo, server aceita assinaturas de ambos secrets.
#    O painel Meta passa a usar o NEW imediatamente após o reset.

# 5. Promover NEXT → CURRENT, limpar NEXT, restart final.
```

---

## Procedimento: DB Password (rotação atômica)

⚠️ **Há downtime curto** (≤ 30s) — fazer em janela de baixo tráfego.

```sh
# 1. Gerar
NEW=$(openssl rand -base64 32 | tr -d '/+=' | head -c 32)

# 2. Trocar a senha do role no Postgres
docker compose ... exec postgres psql -U postgres -c \
  "ALTER USER mecontrola WITH PASSWORD '$NEW';"

# 3. Atualizar .env imediatamente
sudo nano /opt/mecontrola/.env
# DB_PASSWORD=<NEW>

# 4. Recriar pgbouncer + server + worker (pgbouncer mantém connection string em userlist)
docker compose ... up -d --no-deps --force-recreate pgbouncer server worker
```

---

## Procedimento: OTLP / Grafana Cloud API Keys

Atômica — sem `_NEXT`:

```sh
# 1. Gerar nova API key no painel Grafana Cloud
# 2. Atualizar .env (LOKI_API_KEY, OTEL_EXPORTER_OTLP_HEADERS, etc)
sudo nano /opt/mecontrola/.env
# 3. Restart server + worker
docker compose ... up -d --no-deps --force-recreate server worker
# 4. Validar logs/traces fluindo no Grafana Cloud
# 5. Revogar a API key antiga no painel Grafana Cloud
```

---

## Audit Trail

Toda rotação deve ser registrada em ticket ou Slack — quem rotacionou, quando, qual secret,
motivo. Exemplo:

```
2026-06-15 14:30 | rotated KIWIFY_WEBHOOK_SECRET | quarterly rotation | by @jailton
```

Não anotar o valor — só a data e o secret.

---

## Aviso: `docker compose down` derruba o banco

`docker compose down` para **todos** os serviços, incluindo postgres e pgbouncer. Em produção,
use sempre comandos direcionados para não causar downtime de banco:

```sh
# Para parar apenas a aplicação (banco continua rodando):
docker compose -f deployment/compose/compose.yml -f deployment/compose/compose.prod.yml \
  stop server worker

# Para reiniciar apenas o banco sem tocar na app:
docker compose -f deployment/compose/compose.yml -f deployment/compose/compose.prod.yml \
  restart postgres pgbouncer
```

`docker compose down` só deve ser usado em manutenção planejada com downtime autorizado.

---

## Rollback de emergência

Se algo quebrar após o restart final:

```sh
# 1. Restaurar imediatamente o secret anterior em .env (mantenha sempre um backup local efêmero)
sudo nano /opt/mecontrola/.env
# 2. Restart
docker compose ... up -d --no-deps --force-recreate server worker
# 3. Investigar logs antes de tentar novamente
```

---

## Referências

- Lista completa de secrets gerenciáveis: `.env.example` (todos os `CHANGE_ME_*`)
- Audit events relacionados: tabela `auth_events`
