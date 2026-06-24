# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Estado durável relacional + lock otimista + falha terminal determinística
- **Data:** 2026-06-24
- **Status:** Aceita
- **Decisores:** Plataforma
- **Relacionados:** PRD (RF-07, RF-08, RF-09, RF-10, RF-11, RF-12, RF-13); techspec (Modelos de Dados, Retry); ADR-003

## Contexto

- Suspend/resume durável exige persistir o estado do run para sobreviver a restart/crash (RF-07/09).
- Canais (telegram/whatsapp) podem entregar a mesma mensagem em duplicidade, e múltiplas instâncias
  podem retomar o mesmo run concorrentemente (RF-10).
- O projeto já tem padrões consolidados: `uow`/`database.DBTX`, lock otimista por coluna `version`
  (`card_repository.UpdateLimitByIDForUser`), idempotência (`internal/platform/idempotency`),
  migrations versionadas em `mecontrola` e housekeeping via `worker` (reaper de outbox).
- Eficiência importa: runs de leitura pura não devem pagar I/O de snapshot (RF-08).

## Decisão

- **Persistência relacional** em duas tabelas no schema `mecontrola`: `workflow_runs` (run + estado
  serializado em `state JSONB`, `cursor`, `version`, `attempts`/`max_attempts`, `status` fechado) e
  `workflow_steps` (auditoria por passo: `status`, `attempt`, `duration_ms`, `error`). Gravadas pela
  `uow` sobre o `sessionDB`. Migração `000019_create_workflow_runtime`.
- **Snapshot só para escrita/suspensível**: `Definition.Durable=false` ⇒ engine executa in-process e
  não toca as tabelas (RF-08).
- **Concorrência por lock otimista**: `Save` faz `UPDATE ... SET ..., version = version + 1 WHERE id=$
  AND version=$expected`; `RowsAffected()==0 ⇒ ErrVersionConflict`. Índice parcial único
  `(workflow, correlation_key) WHERE status IN ('running','suspended')` garante run ativo único.
  Combinado com a idempotência por decisão já existente.
- **Falha terminal determinística**: esgotado `max_attempts`, o run vira `failed` (auditável + métrica
  `workflow_runs_total{status="failed"}` e `workflow_version_conflict_total`); **sem retry infinito**,
  **sem dead-letter** no MVP.
- **Housekeeping**: `HousekeepingJob` (implementa `worker.Job`) purga runs concluídos além da retenção
  configurável (`WORKFLOW_KERNEL_HOUSEKEEPING_*`), no molde de `outbox.HousekeepingJob`.

## Alternativas Consideradas

- **Tabela única com steps em JSONB**: menos joins/migrations, mas auditoria/consulta por passo fraca.
  Rejeitada (PRD escolheu relacional; observabilidade por passo é objetivo).
- **Lock pessimista (`SELECT FOR UPDATE`)**: simples, mas segura conexão e serializa resumes do mesmo
  run. Rejeitada por custo de conexão; otimista alinha ao padrão existente do projeto.
- **Só idempotência por event_id, sem lock**: janela de corrida em efeitos não cobertos pela chave.
  Rejeitada por insuficiência de garantia.

## Consequências

### Benefícios Esperados

- Resume idempotente e seguro sob entrega dupla/multi-instância, com garantia mais forte que a atual.
- Auditoria por passo (status/duração/tentativa/erro) para diagnóstico.
- Eficiência: leitura pura sem I/O de snapshot.
- Reuso integral dos padrões do projeto (uow, version-CAS, migrations, worker).

### Trade-offs e Custos

- Duas tabelas novas + job de housekeeping a operar.
- `state` em JSONB exige `S` JSON-serializável (campos exportados).
- Sem atomicidade entre `persist` (módulo transactions) e snapshot (sessionDB) — ver Riscos.

### Riscos e Mitigações

- **Risco:** falha entre `persist` e marcar run `succeeded` (sem tx cross-módulo).
  **Mitigação:** ordem `persist`→`succeeded`; re-execução protegida por CAS do run + idempotência do
  módulo transactions; nunca duplica.
- **Risco:** crescimento das tabelas. **Mitigação:** housekeeping com retenção configurável.
- **Rollback:** feature flag desligada não grava snapshot (ADR-005); migração tem `down` idempotente.

## Plano de Implementação

1. Migração `000019` (up/down) + adapter postgres do `Store` (CAS, índice parcial).
2. Engine grava snapshot só quando `Durable`; integration tests de durabilidade/concorrência/housekeeping.
3. Config + validação em `configs/config.go` (+ testes).
4. Conclusão: integration tests verdes (restart→resume sem duplicar; 2 resumes → 1 vencedor; purge correto).

## Monitoramento e Validação

- `workflow_runs_total{workflow,status}`, `workflow_version_conflict_total{workflow}`,
  `workflow_steps_total{workflow,step,status}`, `workflow_suspend_total`, `workflow_resume_total`.
- Sucesso: 0 efeitos duplicados sob resume concorrente; 100% resume após restart simulado.
- Reverter/ajustar se contenção de versão gerar taxa de conflito anômala (indício de hotspot por chave).

## Impacto em Documentação e Operação

- `docs/runbooks/`: novo runbook do kernel (resume preso, conflito de versão, retenção).
- `docs/alerts/` e dashboard Grafana (fast-follow) para as métricas `workflow_*`.
- `.env`/configs documentando as variáveis novas.

## Revisão Futura

- Revisar introdução de dead-letter se a taxa de `failed` terminal exigir inspeção manual recorrente.
- Revisar formato (JSONB vs colunas) se a consulta por passo evoluir.
