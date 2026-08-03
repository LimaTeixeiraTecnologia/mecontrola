# Tarefa 1.0: Reconciliação alerta↔métrica + gate de CI de auditoria

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Auditar `deployment/telemetry/grafana/provisioning/alerting/rules.yaml` contra o inventário de métricas realmente emitidas pelo código atual e reconciliar as inconsistências: corrigir para a métrica viva equivalente ou remover (com justificativa registrada) os alertas que referenciam séries mortas herdadas do `internal/agent` descontinuado. Criar um script de auditoria em `scripts/observability/audit-alert-metrics` que extrai os nomes de métrica do `rules.yaml` e os confronta com o inventário de métricas emitidas, e plugá-lo como gate no CI. Cobre RF-07 e RF-20; segue ADR-003.

<requirements>
- Auditar todo alerta/painel provisionado em `rules.yaml` contra o inventário de métricas emitidas no código (via `o11y.Metrics().*`) mais a allowlist de métricas devkit.
- Corrigir para a métrica viva equivalente ou remover (com justificativa registrada) os alertas que referenciam as métricas mortas confirmadas: `agent_intent_parsed_total`, `agent_llm_fallback_exhausted_total`, `agent_policy_blocks_total`, `agent_intent_routed_total`, `agent_idempotency_replay_total` (herdadas do `internal/agent` descontinuado) e `outbox_pending_jobs` (não emitida).
- NÃO remover alertas cujas métricas são emitidas pelo devkit (`http_server_request_count`/`_active`, `database.pool.wait_count`) — são válidas, não drift.
- Ao final, nenhum alerta pode referenciar série morta.
- Criar `scripts/observability/audit-alert-metrics` que extrai os nomes de métrica do `rules.yaml` e confronta com o inventário (código + allowlist devkit `http_server_request_*`/`database_pool_*`), falhando se algum alerta referenciar série não emitida.
- Plugar o script como gate em `.github/workflows/ci.yml`.
- Não regredir nenhum alerta válido existente (não-regressão, RF-20).
</requirements>

## Subtarefas

- [ ] 1.1 Extrair o inventário de métricas emitidas: nomes usados em `o11y.Metrics().*` no código + a allowlist conhecida de métricas devkit (`http_server_request_*`, `database_pool_*`).
- [ ] 1.2 Auditar `rules.yaml`; para cada referência a métrica morta, migrar a query para a equivalente viva do `internal/agents` atual quando existir, ou remover o alerta com justificativa registrada.
- [ ] 1.3 Implementar `scripts/observability/audit-alert-metrics`: parsear os nomes de métrica de `rules.yaml` e confrontar com o inventário; sair com erro se houver referência a série não emitida (respeitando a allowlist devkit).
- [ ] 1.4 Plugar o script como passo/gate em `.github/workflows/ci.yml`.
- [ ] 1.5 Confirmar que o gate roda limpo sobre o `rules.yaml` reconciliado (CI verde).

## Detalhes de Implementação

Ver techspec.md, seção "Abordagem de Testes → Testes de Integração" (reconciliação RF-07 como gate de CI) e "Sequenciamento de Desenvolvimento → Ordem de Build" (item 1, reconciliação primeiro, sem dependência de código). O contrato completo de decisão está em `adr-003-alert-metric-reconciliation.md` (contexto das seis métricas mortas confirmadas por `grep` com 0 ocorrências, política corrigir-ou-remover, allowlist devkit como mitigação de falso negativo, script como gate de CI). Métricas devkit (`http_server_request_count`, `http_server_request_active`, `database.pool.wait_count`) são emitidas pela lib e NÃO são drift (ADR-003, Contexto).

## Critérios de Sucesso

- Nenhum alerta em `rules.yaml` referencia as seis métricas mortas confirmadas nem qualquer outra série ausente do código atual.
- Cada remoção de alerta tem justificativa registrada; cada correção aponta para métrica viva equivalente.
- `scripts/observability/audit-alert-metrics` retorna sucesso (exit 0) sobre o `rules.yaml` reconciliado e falha (exit ≠ 0) quando uma métrica morta é introduzida.
- O gate está ativo em `.github/workflows/ci.yml` e o CI fica verde.
- Nenhum alerta válido (incluindo os que usam métricas devkit) removido ou renomeado indevidamente.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

Verificar o script de auditoria com dois casos: (a) `rules.yaml` contendo uma métrica morta → o script falha (exit ≠ 0); (b) `rules.yaml` reconciliado/limpo → o script passa (exit 0).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `deployment/telemetry/grafana/provisioning/alerting/rules.yaml` — alertas auditados/reconciliados.
- `scripts/observability/audit-alert-metrics` — script de auditoria (novo, gate de CI).
- `.github/workflows/ci.yml` — passo de gate de auditoria.
