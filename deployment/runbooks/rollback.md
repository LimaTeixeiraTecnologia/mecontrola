# Runbook: Rollback MeControla

**Referências:** ADR-011 (Docker + VPS), ADR-014 (Docker Swarm single-node)

## Quando Usar

- Deploy introduziu regressão detectada por health check ou monitoring.
- Um ou mais services (`server-1`, `server-2`, `worker-1`, `worker-2`) não ficaram `Running` após deploy.
- Erro crítico detectado nos logs pós-deploy.
- Migração falhou e deixou o banco em estado inconsistente.

## Pré-requisitos

- Acesso SSH à VPS com usuário não-root e chave.
- Docker Swarm ativo no host.
- Tag da imagem anterior conhecida (registrada nos logs do deploy ou via `docker service inspect`).
- `.env` e configs do Git disponíveis.

## Identificar a imagem anterior

No host de deploy, a tag anterior está registrada nos logs do último deploy. Também é possível consultar o service atual:

```sh
STACK=mecontrola
docker service inspect "${STACK}_server-1" --format '{{.Spec.TaskTemplate.ContainerSpec.Image}}'
```

Exemplo de saída:

```
ghcr.io/limateixeiratecnologia/mecontrola:abc12345
```

A tag é o valor após `:` (ex.: `abc12345`).

## Rollback manual para tag anterior

O rollback reimplanta a stack Swarm com a tag anterior. O Swarm fará rolling update dos services:

```sh
export STACK=mecontrola
export VPS_DEPLOY_PATH=/opt/mecontrola
export PREVIOUS_TAG=<tag-anterior>

cd "$VPS_DEPLOY_PATH"
IMAGE_TAG="${PREVIOUS_TAG}" \
  docker stack deploy -c deployment/compose/compose.swarm.yml "${STACK}"
```

Para reversão mais rápida (força recriação imediata, sem rolling delay):

```sh
for svc in server-1 server-2 worker-1 worker-2; do
  docker service update --force "${STACK}_${svc}"
done
```

Atenção: `--force` interrompe tarefas em voo imediatamente; prefira o rolling update padrão salvo emergência.

## Verificar recover

```sh
# Status dos services
docker service ls

# Tarefas em execução
docker stack ps "${STACK}" --no-trunc

# Health checks dos app services
for svc in server-1 server-2; do
  docker ps --filter name="${STACK}_${svc}" --filter health=healthy --format '{{.Names}} {{.Status}}'
done
for svc in worker-1 worker-2; do
  docker ps --filter name="${STACK}_${svc}" --filter health=healthy --format '{{.Names}} {{.Status}}'
done

# Health check via Caddy (se disponível)
curl -fsS http://localhost/healthz
curl -fsS http://localhost/readyz
```

Todos os services devem ter ao menos um container `healthy` e `Running`.

## Em caso de falha da migração

Se o rollback for necessário por causa de migration corrompida:

1. Não execute rollback de imagem antes de avaliar o estado do schema.
2. Consulte `schema_migrations`:
   ```sh
   docker exec "${STACK}_postgres.1.$(docker service ps "${STACK}_postgres" -q | head -n1)" \
     psql -U "${DB_USER}" -d "${DB_NAME}" -c 'SELECT version, dirty FROM schema_migrations;'
   ```
3. Se `dirty=true`, siga o runbook `restore-pitr.md` para restaurar o banco para um ponto antes da migration problemática.
4. Após o banco consistente, reimplante a tag anterior conforme a seção acima.

## Investigar causa raiz

```sh
# Logs dos services de aplicação
for svc in server-1 server-2 worker-1 worker-2; do
  echo "=== ${svc} ==="
  docker service logs --since 30m "${STACK}_${svc}" | tail -n 50
done

# Logs do migrate (último container)
docker ps -a --filter name="${STACK}-migrate-" --format '{{.Names}}'
docker logs "<nome-do-container-migrate>" 2>&1 | tail -n 50

# Logs do Caddy e observabilidade
docker service logs --since 30m "${STACK}_caddy" | tail -n 30
docker service logs --since 30m "${STACK}_otel-lgtm" | tail -n 30
```

## Validação pós-rollback

```sh
ok=true
for svc in server-1 server-2 worker-1 worker-2; do
  if docker ps --filter name="${STACK}_${svc}" --filter health=healthy --format '{{.Names}}' | grep -q .; then
    echo "${svc}: OK"
  else
    echo "${svc}: FALHA"
    ok=false
  fi
done
$ok && echo "rollback concluído com sucesso"
```

## Após rollback

1. Abrir issue descrevendo a regressão.
2. Reverter o commit problemático na branch `main` via `git revert`.
3. Criar novo PR com fix + teste de regressão.
4. Fazer deploy normal após CI verde.
5. Revisar métricas e alertas no Grafana para confirmar estabilidade.
