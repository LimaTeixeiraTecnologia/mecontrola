# Runbook: Rotação do META_APP_SECRET (CURRENT + NEXT)

## Visão Geral

Este runbook descreve o procedimento para rotacionar o `META_APP_SECRET` sem downtime, aproveitando o suporte a `secretCurrent + secretNext` implementado em `internal/platform/whatsapp/signature/hmac.go`.

**Tempo estimado:** 5–15 minutos
**Risco:** Baixo (zero downtime quando seguido corretamente)
**Pré-requisito:** Acesso SSH à VPS e ao painel Meta Business

---

## Contexto

O middleware HMAC aceita dois secrets simultaneamente:

| Slot | Variável de ambiente | Descrição |
|------|---------------------|-----------|
| CURRENT | `META_APP_SECRET` | Secret ativo no Meta |
| NEXT | `META_APP_SECRET_NEXT` | Novo secret a ser promovido |

Quando uma requisição é validada pelo slot NEXT, a métrica `meta_signature_status_total{status="rotated"}` é incrementada, indicando que o novo secret está em uso.

---

## Procedimento de Rotação

### Etapa 1 — Gerar novo secret no Meta Business Manager

1. Acesse o painel Meta Business Manager → App → Configurações → Segredos.
2. Gere um novo App Secret.
3. Anote o valor gerado (não salve em arquivo plaintext).

### Etapa 2 — Configurar NEXT na VPS (janela dupla)

```bash
ssh $VPS_USER@$VPS_HOST

# Editar o env file da aplicação
nano $VPS_DEPLOY_PATH/.env

# Adicionar ou atualizar:
META_APP_SECRET_NEXT=<novo_secret_aqui>

# Restartar a aplicação sem downtime (graceful reload)
docker compose -f $VPS_DEPLOY_PATH/docker-compose.yml up -d --no-deps --force-recreate api
```

**Validação:** Aguardar 2 minutos e verificar métricas:
```promql
meta_signature_status_total{status="rotated"}
```
Se o valor aumentar, o Meta está enviando com o novo secret. Prosseguir para Etapa 3.

### Etapa 3 — Promover NEXT para CURRENT

```bash
ssh $VPS_USER@$VPS_HOST

nano $VPS_DEPLOY_PATH/.env

# Substituir:
META_APP_SECRET=<novo_secret_aqui>
META_APP_SECRET_NEXT=   # limpar ou remover

# Reiniciar
docker compose -f $VPS_DEPLOY_PATH/docker-compose.yml up -d --no-deps --force-recreate api
```

### Etapa 4 — Validar estabilidade pós-rotação

```bash
# Aguardar 5 minutos e verificar:
# 1. Métrica status="rotated" deve retornar a zero
# 2. Métrica status="valid" deve continuar incrementando
# 3. Sem alertas de auth_failed_total{reason="invalid_signature"}
```

```promql
rate(meta_signature_status_total{status="valid"}[5m])   > 0
rate(meta_signature_status_total{status="invalid"}[5m]) == 0
meta_signature_status_total{status="rotated"}           == 0
```

---

## Checklist de Validação

- [ ] NEXT configurado antes de CURRENT ser removido
- [ ] Métrica `meta_signature_status_total{status="rotated"}` observada durante janela de transição
- [ ] CURRENT promovido para novo valor
- [ ] NEXT removido ou zerado
- [ ] Nenhum alerta `auth_failed_total{reason="invalid_signature"}` disparado durante a rotação
- [ ] Dashboard "Auth Module" sem anomalias após 10 minutos

---

## Rollback

Se a rotação causar falhas:

```bash
ssh $VPS_USER@$VPS_HOST
nano $VPS_DEPLOY_PATH/.env

# Restaurar o secret anterior em CURRENT
META_APP_SECRET=<secret_anterior>
META_APP_SECRET_NEXT=

docker compose -f $VPS_DEPLOY_PATH/docker-compose.yml up -d --no-deps --force-recreate api
```

---

## Alertas relacionados

| Alerta | Significado | Ação |
|--------|-------------|------|
| `auth_failed_total{reason="invalid_signature"} > 0 em 5min` | Secret incorreto ou janela perdida | Verificar env vars; executar rollback se necessário |
| `meta_signature_status_total{status="rotated"} > 0 por > 30min` | NEXT não foi promovido | Executar Etapa 3 |

---

## Variáveis de Ambiente Referenciadas

| Variável | Obrigatória | Descrição |
|----------|-------------|-----------|
| `META_APP_SECRET` | Sim | Secret atual validado pelo slot CURRENT |
| `META_APP_SECRET_NEXT` | Apenas durante rotação | Novo secret no slot NEXT |
| `STAGING_SMOKE_WA` | Em staging | Número WhatsApp do usuário de smoke test (formato E.164) |
