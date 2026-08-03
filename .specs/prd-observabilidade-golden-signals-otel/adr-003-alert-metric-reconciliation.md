# Registro de Decisão Arquitetural (ADR-003)

## Metadados

- **Título:** Reconciliação alerta↔métrica: corrigir ou remover alertas que referenciam séries mortas + gate de CI
- **Data:** 2026-08-03
- **Status:** Aceita
- **Decisores:** Engenharia de plataforma (on-call)
- **Relacionados:** PRD (RF-07, RF-20), techspec.md

## Contexto

A auditoria de `deployment/telemetry/grafana/provisioning/alerting/rules.yaml` contra o código atual revelou alertas que referenciam métricas ausentes, herdadas do módulo `internal/agent` descontinuado (emenda 2026-06-29). Confirmado por `grep` no código (0 ocorrências): `agent_intent_parsed_total`, `agent_llm_fallback_exhausted_total`, `agent_policy_blocks_total`, `agent_intent_routed_total`, `agent_idempotency_replay_total`, além de `outbox_pending_jobs` (não emitido). Esses alertas dão falsa sensação de cobertura: com `noDataState: OK` não disparam, mascarando a ausência do sinal. Métricas devkit (`http_server_request_count`, `http_server_request_active`, `database.pool.wait_count`) são válidas (emitidas pela lib, não pelo app) e NÃO são drift.

## Decisão

Para cada alerta/painel que referencia métrica ausente do código atual: (a) se existir métrica viva equivalente no `internal/agents` atual, migrar a query para ela; (b) caso contrário, remover o alerta com justificativa registrada. Ao final, nenhum alerta pode referenciar série morta. Criar um script de auditoria (`scripts/observability/audit-alert-metrics`) que extrai os nomes de métrica de `rules.yaml` e confronta com o inventário de métricas emitidas (código + lista devkit conhecida), tornando-o gate de CI para prevenir drift futuro.

## Alternativas Consideradas

- **Deixar como está**: mantém alertas mortos e risco de falsa cobertura. Rejeitada (contraria "0 falso positivo").
- **Só corrigir manualmente, sem gate**: o drift reincide a cada renomeação de métrica. Rejeitada; o gate é essencial.
- **Recriar as métricas antigas no código novo**: reintroduz conceito do módulo descontinuado. Rejeitada.

## Consequências

### Benefícios Esperados

- Alertas refletem apenas métricas vivas; on-call confia no que dispara.
- Gate de CI previne reincidência do drift.

### Trade-offs e Custos

- Esforço de mapear cada métrica morta para a equivalente viva (ou decidir remoção).
- Manutenção do inventário de métricas usado pelo gate.

### Riscos e Mitigações

- **Risco:** falso negativo do gate por métrica emitida só pelo devkit (não no código). **Mitigação:** o inventário inclui a allowlist de métricas devkit (`http_server_request_*`, `database_pool_*`).
- **Risco:** remover alerta ainda desejado. **Mitigação:** exigir justificativa e revisão humana antes da remoção.

## Plano de Implementação

1. Extrair inventário de métricas emitidas (código `o11y.Metrics().*` + allowlist devkit).
2. Auditar `rules.yaml`; migrar ou remover cada referência morta.
3. Implementar o script de auditoria e plugá-lo no CI (`.github/workflows/ci.yml`).

## Monitoramento e Validação

- Sucesso: script de auditoria retorna limpo; CI verde.
- Falha do gate bloqueia PR que introduza alerta com métrica inexistente.

## Impacto em Documentação e Operação

- `observability/STANDARD.md`: inventário de métricas e a política de reconciliação.
- CI: novo passo de auditoria.

## Revisão Futura

- Revisitar a allowlist devkit ao atualizar a versão do devkit-go.
