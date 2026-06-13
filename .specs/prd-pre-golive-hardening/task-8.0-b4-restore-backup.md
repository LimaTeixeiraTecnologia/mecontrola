# Tarefa 8.0: B4 — Restore de backup automatizado + cron staging

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementa script automatizado de restore de backup criptografado (rclone + age → Postgres efêmero → smoke queries) e agenda cron mensal em staging. Sem essa validação, o backup é teórico. Esta tarefa fecha o último bloqueante operacional do go-live.

<requirements>
- RF-11: script `deployment/scripts/pg-restore-smoke.sh` cobre baixa + descriptografia + restore + smoke + cleanup
- RF-12: idempotente, exit 0 OK / ≠0 falha com mensagem clara
- RF-13: runbook `docs/runbooks/backup-restore.md` cobrindo pré-requisitos, execução, agendamento, troubleshooting
- RF-14: cron mensal em staging configurado e validado
- Smoke queries mínimas: 3 tabelas críticas (`users`, `cards`, `transactions` ou equivalente per-user)
</requirements>

## Subtarefas

- [ ] 8.1 Criar `deployment/scripts/pg-restore-smoke.sh` com etapas:
  1. Verificar pré-reqs: `rclone`, `age`, `docker`, `psql` (apenas se necessário).
  2. Baixar último dump cifrado: `rclone copy <remote>:<bucket>/<latest> /tmp/<file>`.
  3. Descriptografar: `age -d -i <key-path> /tmp/<file> > /tmp/dump.sql`.
  4. Subir Postgres efêmero: `docker run --rm -d --name pg-restore-smoke -e POSTGRES_PASSWORD=... -p <ephemeral-port>:5432 postgres:<version-pinned>`.
  5. Aguardar Postgres pronto (`pg_isready` loop).
  6. Restaurar: `psql -h localhost -p <port> -U postgres < /tmp/dump.sql`.
  7. Smoke queries: `psql -c "SELECT count(*) FROM mecontrola.users;"` (× 3 tabelas) com asserção `count > 0`.
  8. Cleanup: `docker rm -f pg-restore-smoke`, `rm /tmp/dump.sql` (manter dump cifrado para auditoria opcional).
  9. Exit 0 se tudo OK; ≠0 se qualquer passo falhou.
- [ ] 8.2 Criar `docs/runbooks/backup-restore.md` cobrindo:
  - Pré-requisitos (binários instalados + chave age).
  - Execução manual.
  - Agendamento cron mensal em staging.
  - Critérios de sucesso (smoke queries OK).
  - Troubleshooting (chave inválida, espaço em disco, Postgres não sobe).
- [ ] 8.3 Configurar cron mensal em staging (`crontab -e` em VPS staging) que invoca o script e envia alerta em caso de falha.
- [ ] 8.4 Adicionar receita `task backup:restore-smoke` em `taskfiles/security.yml` para uso local.
- [ ] 8.5 Executar uma vez em staging real e documentar resultado (output do smoke + count das 3 tabelas) no commit/PR.

## Detalhes de Implementação

Ver plano-fonte §5 B4. Reusa o `pg-dump.sh` existente como referência de estilo. Skill `taskfile-production` para a receita.

## Critérios de Sucesso

- `bash deployment/scripts/pg-restore-smoke.sh` em staging exit 0.
- Cron mensal configurado e primeira execução validada.
- Runbook revisado, sem TODOs.
- Alerta de falha configurado (e.g. Grafana ou simples email/Slack via cron).

## Skills Necessárias

<!-- MANDATÓRIO -->

- `taskfile-production` — adicionar receita `backup:restore-smoke` em `taskfiles/security.yml` seguindo padrão idempotente do projeto (RF-11)

## Testes da Tarefa

- [ ] Execução manual em staging (dump real)
- [ ] Re-execução idempotente
- [ ] Cron disparou ao menos 1 vez

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `deployment/scripts/pg-restore-smoke.sh` (novo)
- `docs/runbooks/backup-restore.md` (novo)
- `taskfiles/security.yml` (modificado)
