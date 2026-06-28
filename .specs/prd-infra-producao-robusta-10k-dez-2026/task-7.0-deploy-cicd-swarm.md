# Tarefa 7.0: Adaptar Scripts de Deploy e CI/CD para docker stack deploy

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adaptar os scripts de deploy (`deploy.sh`, `deploy-local.sh`) e o workflow `.github/workflows/ci-cd.yml` para realizar `docker stack deploy` no Swarm, mantendo rollback manual por imagem anterior.

<requirements>
- Cobrir RF-12: imagem distroless, nonroot, tag imutável, nunca `latest`.
- Cobrir RF-19: deploy repetível e reversível para imagem anterior em caso de falha.
- Cobrir RF-20: restart policy, resource limits (já no compose.swarm.yml, validar via deploy).
</requirements>

## Subtarefas

- [ ] 7.1 Criar `deployment/scripts/deploy-swarm.sh` com fluxo SSH + `docker stack deploy`.
- [ ] 7.2 Adaptar `deployment/scripts/deploy.sh` para usar `deploy-swarm.sh` ou mantê-lo como legacy.
- [ ] 7.3 Adaptar `deployment/scripts/deploy-local.sh` para Swarm.
- [ ] 7.4 Capturar tag da imagem anterior para rollback manual.
- [ ] 7.5 Adaptar `.github/workflows/ci-cd.yml` job `deploy` para chamar `deploy-swarm.sh`.
- [ ] 7.6 Garantir que migrations sejam executadas antes do deploy dos services.
- [ ] 7.7 Validar health checks de `server-1`, `server-2`, `worker-1`, `worker-2` após deploy.
- [ ] 7.8 Documentar rollback manual no runbook.

## Detalhes de Implementação

Ver seção "7. CI/CD e Deploy" de `techspec.md`. O fluxo de deploy deve:

1. Fazer git pull na VPS.
2. Criar/atualizar Docker secrets.
3. Fazer backup do `.env` para S3.
4. Executar migrations via `docker run --rm` (não como service Swarm, para garantir uma única execução).
5. Fazer `docker stack deploy`.
6. Aguardar health checks.
7. Em caso de falha, fazer rollback manual para imagem anterior.

Exemplo de execução do migrate:

```bash
docker run --rm \
  --env-file "${VPS_DEPLOY_PATH}/.env" \
  "${IMAGE_NAME}:${IMAGE_TAG}" \
  migrate
```

## Critérios de Sucesso

- `bash deployment/scripts/deploy-swarm.sh <tag>` realiza deploy completo em staging.
- GitHub Actions executa deploy automático na branch `main`.
- Rollback manual para tag anterior restaura aplicação saudável em ≤ 10 min.
- Nenhuma imagem `latest` é usada em produção.
- Health checks de todos os services passam após deploy.

## Skills Necessárias

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Teste de deploy bem-sucedido em staging.
- [ ] Teste de rollback manual simulando falha de health check.
- [ ] Teste de deploy via GitHub Actions em ambiente de staging.
- [ ] Teste de que migrations não rodam concorrentemente.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `deployment/scripts/deploy.sh`
- `deployment/scripts/deploy-local.sh`
- `deployment/scripts/deploy-swarm.sh` (novo)
- `deployment/scripts/create-secrets.sh`
- `deployment/scripts/backup-env-s3.sh`
- `.github/workflows/ci-cd.yml`
- `deployment/runbooks/rollback.md`
