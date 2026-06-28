# Tarefa 8.0: Atualizar Runbooks e Realizar Testes de Staging/Migração

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Atualizar runbooks operacionais, configurar alertas no Grafana, auditar idempotência dos jobs e realizar testes de staging e migração para produção.

<requirements>
- Cobrir RF-17: runbooks testados de restore PITR e restore completo da VPS.
- Cobrir RF-18: observabilidade mínima com alertas de saturação.
- Cobrir RF-22: housekeeping de logs, métricas, traces e eventos.
- Cobrir RF-23: fila de jobs usa locking no banco.
- Cobrir RF-24: jobs idempotentes antes de escalar para 2 workers.
- Cobrir RF-26: retry com backoff exponencial e DLQ após 3 tentativas (auditar configuração de outbox).
</requirements>

## Subtarefas

- [ ] 8.1 Atualizar `deployment/runbooks/deploy.md` com fluxo Swarm.
- [ ] 8.2 Atualizar `deployment/runbooks/restore-pitr.md` com comandos pgBackRest.
- [ ] 8.3 Atualizar `deployment/runbooks/restore-vps.md` com passos de recuperação a partir de S3.
- [ ] 8.4 Atualizar `deployment/runbooks/rollback.md` com rollback manual de imagem.
- [ ] 8.5 Configurar/validar alertas no Grafana:
  - disco > 80%, CPU > 70% por 5min, RAM > 80%, WAL lag > 15min, fila de jobs > 1000.
- [ ] 8.6 Configurar housekeeping de retenção no LGTM (7 dias logs, 15 dias métricas, 7 dias traces).
- [ ] 8.7 Auditar locking da fila de jobs (`SELECT FOR UPDATE SKIP LOCKED` no outbox).
- [ ] 8.8 Auditar idempotência dos jobs: revisar handlers e confirmar que reprocessamento não duplica efeitos.
- [ ] 8.9 Auditar configuração de retry e DLQ: confirmar `OUTBOX_RETRY_MAX_ATTEMPTS=3`, backoff exponencial e DLQ após tentativas.
- [ ] 8.9 Realizar migração de Compose para Swarm em produção em janela de manutenção.
- [ ] 8.10 Executar testes de carga local (k6/locust) como sanity check.

## Detalhes de Implementação

Ver seção "8. Observabilidade" e seção "Riscos Conhecidos" de `techspec.md`. A auditoria de idempotência deve inspecionar handlers de outbox e jobs do worker, procurando por operações não idempotentes (ex.: inserts sem ON CONFLICT, envio de mensagens sem dedup).

Para o locking da fila, verificar implementação do dispatcher de outbox. Se já usar `SELECT FOR UPDATE SKIP LOCKED`, documentar. Caso contrário, criar tarefa de correção.

## Critérios de Sucesso

- Todos os runbooks estão atualizados e foram testados pelo menos uma vez.
- Alertas do Grafana disparam nos thresholds definidos.
- Retenção de LGTM configurada e validada.
- Auditoria de idempotência concluída com evidências.
- Migração de produção concluída com 2+2 réplicas saudáveis.
- Testes de restore PITR e restore de VPS bem-sucedidos.

## Skills Necessárias

- `otel-grafana-dashboards` — configuração e revisão de dashboards/alertas Grafana para métricas de saturação.

## Testes da Tarefa

- [ ] Teste de restore PITR em instância isolada.
- [ ] Teste de restore completo da VPS a partir de backup S3.
- [ ] Teste de alertas: simular CPU alto e confirmar disparo no Telegram.
- [ ] Teste de idempotência: reprocessar jobs e verificar ausência de duplicatas.
- [ ] Teste de carga: k6/locust simulando 10k usuários (local).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `deployment/runbooks/deploy.md`
- `deployment/runbooks/restore-pitr.md`
- `deployment/runbooks/restore-vps.md`
- `deployment/runbooks/rollback.md`
- `deployment/telemetry/grafana/` (alertas e dashboards)
- `internal/platform/outbox/` (dispatcher e reaper)
- `internal/platform/worker/` (jobs)
