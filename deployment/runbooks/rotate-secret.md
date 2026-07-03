# Runbook: Rotacionar Secrets na VPS (Hostinger)

**Última revisão:** 2026-07-03
**Referências:** ADR-009 (Viper + configs), ADR-013 (segurança operacional)
**Substitui:** versão anterior que documentava `.env` na VPS.

## Quando Usar

- Rotação periódica programada de credenciais (recomendado: trimestral).
- Suspeita de vazamento de secret.
- Funcionário com acesso ao repositório/GitHub secrets saiu da empresa.
- Mudança de provedor de serviço (novo token OTLP, nova senha do banco, novo webhook secret Kiwify).

## Princípio: Zero-downtime via `_CURRENT` + `_NEXT`

Para secrets de validação assimétrica (gateway HMAC, Kiwify webhook, Meta app secret), o
sistema aceita **dois secrets simultaneamente** (`_CURRENT` e `_NEXT`). Isso permite:
1. Adicionar `_NEXT` com o novo valor.
2. Deploy — server passa a aceitar ambos.
3. Atualizar o cliente externo (painel Kiwify / Meta / sistemas internos).
4. Promover `_NEXT` para `_CURRENT` e limpar `_NEXT`.
5. Deploy — agora apenas o novo é aceito.

Para secrets de uso unilateral (DB password, OTLP API key), a rotação é atômica.

---

## Pré-requisitos

- `sops` e `age` instalados localmente.
- Chave privada age em `~/.config/sops/age/keys.txt` (ou via `AGE_PRIVATE_KEY`).
- Acesso SSH à VPS para deploy.

```sh
# Verifique se consegue descriptografar
sops --decrypt deployment/config/prod.secrets.env > /dev/null && echo "OK"
```

---

## Procedimento: Gateway Auth Secret (HMAC-SHA256)

### 1. Gerar novo secret

```sh
NEW=$(openssl rand -hex 32)
echo "NOVO secret: $NEW"
# Anotar em local seguro (1Password, Bitwarden, gerenciador da empresa).
```

### 2. Editar `deployment/config/prod.secrets.env`

```sh
sops deployment/config/prod.secrets.env
```

Adicionar/modificar:
```
IDENTITY_GATEWAY_SHARED_SECRET_NEXT=<NEW>
```

Manter:
```
IDENTITY_GATEWAY_SHARED_SECRET_CURRENT=<antigo>
```

Salvar e sair do editor. O SOPS re-criptografa automaticamente.

### 3. Commit e deploy

```sh
git add deployment/config/prod.secrets.env
git commit -m "security: adiciona NEXT do gateway auth secret"
git push origin main
```

O CI/CD faz o deploy automaticamente.

### 4. Validar ambos os secrets ativos

```sh
ssh deploy@<vps-host>
docker service logs mecontrola_server-1 | grep "gateway secrets loaded"
# Esperado: "CURRENT + NEXT both active"
```

### 5. Atualizar clientes (sistemas que assinam requests)

Caso de uso típico: outro serviço interno que faz HMAC para chamar `/api/v1/identity/users`.
Atualizar o secret do lado do cliente para o novo valor.

### 6. Promover `_NEXT` para `_CURRENT`

```sh
sops deployment/config/prod.secrets.env
```

Trocar:
```
IDENTITY_GATEWAY_SHARED_SECRET_CURRENT=<NEW>
IDENTITY_GATEWAY_SHARED_SECRET_NEXT=
```

Commit e push:
```sh
git add deployment/config/prod.secrets.env
git commit -m "security: promove NEXT para CURRENT do gateway auth secret"
git push origin main
```

### 7. Validar

```sh
docker service logs mecontrola_server-1 | tail -50
# Verificar: nenhum erro de assinatura nos últimos 5 min via auth_events
psql <conn> -c "SELECT reason, count(*) FROM auth_events WHERE created_at > now() - interval '5 minutes' GROUP BY 1;"
```

---

## Procedimento: Kiwify Webhook Secret (HMAC-SHA1)

Mesmo padrão de `_CURRENT`/`_NEXT`:

```sh
# 1. Gerar
NEW=$(openssl rand -hex 20)

# 2. Adicionar como _NEXT em prod.secrets.env
sops deployment/config/prod.secrets.env
# KIWIFY_WEBHOOK_SECRET_NEXT=<NEW>

# 3. Commit e deploy
git add deployment/config/prod.secrets.env
git commit -m "security: adiciona NEXT do kiwify webhook secret"
git push origin main

# 4. Atualizar no painel Kiwify:
#    Integrações → Webhooks → editar → Secret = <NEW>
#    Salvar.

# 5. Validar com botão "Testar" do painel Kiwify
docker service logs mecontrola_server-1 | grep "kiwify webhook"
# Deve aceitar assinaturas dos dois secrets durante o overlap.

# 6. Promover NEXT → CURRENT, limpar NEXT, commit e deploy.
sops deployment/config/prod.secrets.env
git add deployment/config/prod.secrets.env
git commit -m "security: promove NEXT para CURRENT do kiwify webhook secret"
git push origin main
```

---

## Procedimento: Meta App Secret (HMAC-SHA256 WhatsApp)

```sh
# 1. Gerar no painel Meta: developers.facebook.com → App → Settings → Basic → App Secret → "Reset"
NEW=<copiado-do-painel>

# 2. Adicionar como _NEXT em prod.secrets.env
sops deployment/config/prod.secrets.env
# META_APP_SECRET_NEXT=<NEW>

# 3. Commit e deploy
git add deployment/config/prod.secrets.env
git commit -m "security: adiciona NEXT do meta app secret"
git push origin main

# 4. Aguardar 5 min — durante esse intervalo, server aceita assinaturas de ambos secrets.
#    O painel Meta passa a usar o NEW imediatamente após o reset.

# 5. Promover NEXT → CURRENT, limpar NEXT, commit e deploy.
sops deployment/config/prod.secrets.env
git add deployment/config/prod.secrets.env
git commit -m "security: promove NEXT para CURRENT do meta app secret"
git push origin main
```

---

## Procedimento: DB Password (rotação atômica)

⚠️ **Há downtime curto** (≤ 30s) — fazer em janela de baixo tráfego.

```sh
# 1. Gerar
NEW=$(openssl rand -base64 32 | tr -d '/+=' | head -c 32)

# 2. Trocar a senha do role no Postgres
ssh deploy@<vps-host>
docker service scale mecontrola_server-1=0 mecontrola_server-2=0 mecontrola_worker-1=0 mecontrola_worker-2=0
docker exec "$(docker ps -q -f name=mecontrola_postgres)" psql -U postgres -c \
  "ALTER USER mecontrola WITH PASSWORD '$NEW';"

# 3. Atualizar prod.secrets.env
exit # volta para a máquina local
sops deployment/config/prod.secrets.env
# DB_PASSWORD=<NEW>
git add deployment/config/prod.secrets.env
git commit -m "security: rotaciona senha do banco de dados"
git push origin main

# 4. O CI/CD recria os docker secrets e sobe os services com a nova senha.
```

---

## Procedimento: OTLP / Grafana Cloud API Keys

Atômica — sem `_NEXT`:

```sh
# 1. Gerar nova API key no painel Grafana Cloud
# 2. Atualizar prod.secrets.env
sops deployment/config/prod.secrets.env
# 3. Commit e deploy
git add deployment/config/prod.secrets.env
git commit -m "security: rotaciona grafana cloud api key"
git push origin main
# 4. Validar logs/traces fluindo no Grafana Cloud
# 5. Revogar a API key antiga no painel Grafana Cloud
```

---

## Audit Trail

Toda rotação deve ser registrada em ticket ou Slack — quem rotacionou, quando, qual secret,
motivo. Exemplo:

```
2026-07-03 14:30 | rotated KIWIFY_WEBHOOK_SECRET | quarterly rotation | by @jailton
```

Não anotar o valor — só a data e o secret.

---

## Aviso: `docker compose down` derruba o banco

`docker compose down` para **todos** os serviços, incluindo postgres e pgbouncer. Em produção,
use sempre comandos direcionados para não causar downtime de banco:

```sh
# Para parar apenas a aplicação (banco continua rodando):
docker service scale mecontrola_server-1=0 mecontrola_server-2=0 mecontrola_worker-1=0 mecontrola_worker-2=0

# Para reiniciar apenas o banco sem tocar na app:
docker service update --force mecontrola_postgres
```

---

## Rollback de emergência

Se algo quebrar após o deploy final:

```sh
# 1. Reverter imediatamente o secret em prod.secrets.env para o valor anterior
sops deployment/config/prod.secrets.env
# 2. Commit e push
git add deployment/config/prod.secrets.env
git commit -m "security: rollback de secret"
git push origin main
# 3. O CI/CD fará o deploy da versão anterior dos secrets
```

---

## Referências

- Lista completa de secrets gerenciáveis: `deployment/config/prod.secrets.env.example`
- Audit events relacionados: tabela `auth_events`
- Setup inicial de SOPS + age: `deployment/scripts/setup-sops-age.sh`
