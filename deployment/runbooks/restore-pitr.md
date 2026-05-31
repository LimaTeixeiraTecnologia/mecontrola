# Runbook: Restore Fly Postgres via PITR

**Referências:** ADR-011 (Fly Postgres), ADR-005 (validação)

## Quando Usar

- Corrupção de dados causada por bug ou operação humana incorreta.
- Necessidade de restaurar estado do banco a um ponto no tempo específico.

## Pré-requisitos

```sh
flyctl auth login
export FLY_APP=mecontrola
export FLY_PG_APP=mecontrola-db
```

## Passo a Passo

### 1. Pausar o app em produção

```sh
flyctl scale count app=0 worker=0 --app ${FLY_APP}
flyctl status -a ${FLY_APP}
```

### 2. Criar fork do Postgres no ponto desejado

```sh
flyctl postgres backup list --app ${FLY_PG_APP}

flyctl postgres restore \
  --app ${FLY_PG_APP} \
  --restore-target-time "2026-05-30T14:00:00Z"
```

Substitua o timestamp pelo ponto desejado em UTC ISO-8601.

### 3. Verificar restore do banco

```sh
flyctl postgres connect --app ${FLY_PG_APP}
\dt
SELECT COUNT(*) FROM health_probe;
\q
```

### 4. Executar migrations pós-restore

```sh
flyctl ssh console -a ${FLY_APP} -C '/mecontrola migrate'
```

### 5. Smoke test do banco

```sh
flyctl postgres connect --app ${FLY_PG_APP} -c \
  "SELECT COUNT(*) FROM health_probe;"
```

Deve retornar ao menos 1 linha.

### 6. Reubicar app

```sh
flyctl scale count app=1 worker=1 --app ${FLY_APP}
flyctl status -a ${FLY_APP}
```

### 7. Smoke test pós-restore

```sh
curl -s https://mecontrola.fly.dev/ready | jq .
flyctl logs -a ${FLY_APP} -p app
```

## Pontos de Atenção

- O PITR do Fly Postgres tem retenção padrão de **7 dias**.
- Qualquer restore cria um novo cluster — atualizar `DATABASE_URL` no app se necessário.
- Documentar o incidente após recuperação com timestamp de restore e causa raiz.
