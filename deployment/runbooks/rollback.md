# Runbook: Rollback MeControla

**Referências:** ADR-011 (Fly.io rolling deploy)

## Quando Usar

- Deploy introduziu regressão detectada por health check ou monitoring.
- Ambos os processes (`app`, `worker`) não estão em `started` após deploy.
- Erro crítico detectado nos logs pós-deploy.

## Pré-requisitos

```sh
flyctl auth login
export FLY_APP=mecontrola
```

## Passo a Passo

### 1. Identificar a release anterior

```sh
flyctl releases -a ${FLY_APP}
```

Saída exemplo:
```
VERSION  STATUS   DESCRIPTION           USER     DATE
v42      deployed Deploy image sha-abc  bot      2026-05-31
v41      deployed Deploy image sha-xyz  jailton  2026-05-30
```

### 2. Rollback para release anterior

```sh
flyctl deploy \
  --image ghcr.io/limateixeiratecnologia/mecontrola:<sha-anterior> \
  --strategy immediate \
  --app ${FLY_APP}
```

Use `--strategy immediate` para rollback urgente (derruba e sobe sem rolling).

### 3. Verificar recover

```sh
flyctl status -a ${FLY_APP}
# Ambos app e worker devem estar em started

curl -s https://mecontrola.fly.dev/health | jq .
curl -s https://mecontrola.fly.dev/ready | jq .
```

### 4. Investigar causa raiz

```sh
flyctl logs -a ${FLY_APP} -p app   --since 1h
flyctl logs -a ${FLY_APP} -p worker --since 1h
```

## Validacao Pós-Rollback

```sh
flyctl status -a ${FLY_APP} | grep started
echo "app OK" && echo "worker OK"
```

## Após Rollback

1. Abrir issue descrevendo a regressão.
2. Reverter o commit problemático na branch `main` via `git revert`.
3. Criar novo PR com fix + teste de regressão.
4. Fazer deploy normal após CI verde.
