# Tarefa 3.0: Remover runner do host de produção e migrar deploy para GitHub-hosted via SSH

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

O runner de CI self-hosted roda **no host de produção** (`/home/github-runner`), consumindo ~50 GiB (build cache 22 GiB + imagens 25 GiB + workspace 2,4 GiB) e disputando os 2 vCPU nos builds. Remover totalmente e migrar build/deploy para GitHub-hosted, com deploy via SSH — preservando todos os gates e o rollback.

<requirements>
- RF-07: remover totalmente o runner do host e recuperar o disco.
- RF-08: build/scan/sign em runners GitHub-hosted, com gates bloqueantes preservados.
- RF-09: job de deploy em GitHub-hosted via SSH/docker context, preservando SOPS, secrets, migrate, stack deploy, health e rollback.
- RF-10: prune agendado de build cache/imagens + alerta de disco no host.
</requirements>

## Subtarefas

- [ ] 3.1 Desregistrar o runner no GitHub; `systemctl stop/disable`; remover diretórios e usuário `github-runner`; `docker builder prune -af` e `image prune -af`.
- [ ] 3.2 Reescrever job `deploy` de `runs-on: [self-hosted, staging]` para `ubuntu-24.04`, executando `deploy-swarm.sh` via SSH (chave em secret, host key fixada), preservando descriptografia SOPS, `create-secrets.sh`, migrate com advisory lock, `docker stack deploy`, waiters de health e rollback automático.
- [ ] 3.3 Confirmar build/scan(Trivy)/sign(cosign) em GitHub-hosted com os gates bloqueantes intactos.
- [ ] 3.4 Adicionar prune agendado (systemd-timer/cron) controlado e alerta de disco (`node_filesystem_avail_bytes`).
- [ ] 3.5 Validar o novo caminho de deploy em staging antes de produção.

## Detalhes de Implementação

Ver `techspec.md` REQ-03. Manter idempotência do deploy e o rollback por health já presentes em `deploy-swarm.sh`.

## Critérios de Sucesso

- Nenhum processo de runner no host (`ps`); `df /` com disco majoritariamente livre.
- Deploy verde pelo novo pipeline GitHub-hosted → VPS via SSH, com health e rollback funcionando.
- Trivy e cosign continuam bloqueantes; alerta de disco ativo.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `taskfile-production` — reescrever jobs/targets de CI/CD e integrar prune e deploy via SSH na pipeline.

## Testes da Tarefa

- [ ] Testes unitários (não aplicável; validação por dry-run de pipeline)
- [ ] Testes de integração (deploy end-to-end em staging pelo novo caminho, com rollback exercido)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `.github/workflows/ci-cd.yml`
- `deployment/scripts/deploy-swarm.sh`, `deployment/scripts/create-secrets.sh`, `deployment/scripts/render-stack.py`
- `taskfiles/deploy.yml`, `taskfiles/swarm.yml`, `taskfiles/ci.yml`
- `deployment/telemetry/grafana/provisioning/alerting/rules.yaml`
