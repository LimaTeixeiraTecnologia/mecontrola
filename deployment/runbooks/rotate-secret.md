# Runbook: Rotacionar Secrets no Fly.io

**Referências:** ADR-009 (Viper + configs), ADR-013 (segurança operacional)

## Quando Usar

- Rotação periódica programada de credenciais.
- Suspeita de vazamento de secret.
- Mudança de provedor de serviço (ex: novo token OTLP, nova senha do banco).

## Pré-requisitos

```sh
flyctl auth login
export FLY_APP=mecontrola
```

## Passo a Passo

### 1. Definir o novo valor

Gere senhas com no mínimo 32 caracteres:

```sh
openssl rand -base64 32
```

### 2. Atualizar o secret no Fly.io

```sh
flyctl secrets set DB_PASSWORD="<novo-valor>" --app ${FLY_APP}
```

Para múltiplos secrets de uma vez:

```sh
flyctl secrets set \
  DB_PASSWORD="<novo-valor>" \
  OTEL_EXPORTER_OTLP_HEADERS="<nova-api-key>" \
  --app ${FLY_APP}
```

O `flyctl secrets set` faz o app reiniciar automaticamente com rolling deploy.

### 3. Verificar restart

```sh
flyctl status -a ${FLY_APP}
```

Aguarde ambos `app` e `worker` voltarem ao estado `started`.

### 4. Smoke test

```sh
curl -s https://mecontrola.fly.dev/ready | jq .
```

Resposta esperada: `{"status":"ok"}` com HTTP 200.

### 5. Verificar logs de startup

```sh
flyctl logs -a ${FLY_APP} -p app   --since 5m
flyctl logs -a ${FLY_APP} -p worker --since 5m
```

Não deve haver erros de conexão com banco ou OTLP.

### 6. Revogar o secret antigo

Após confirmar que o novo valor funciona, revogar o antigo no provedor correspondente
(ex: Grafana Cloud → API Keys → Delete).

## Secrets gerenciados

| Secret | Descrição |
|---|---|
| `DB_PASSWORD` | Senha do Postgres |
| `DB_USER` | Usuário do Postgres |
| `OTEL_EXPORTER_OTLP_HEADERS` | API key do Grafana Cloud OTLP |
| `FLY_API_TOKEN` | Token do Fly.io (usado no CI) |

## Auditoria

Após rotação, registrar no log de auditoria interno:
- Data/hora da rotação
- Secret rotacionado (sem o valor)
- Motivo (periódico / incidente)
- Responsável
