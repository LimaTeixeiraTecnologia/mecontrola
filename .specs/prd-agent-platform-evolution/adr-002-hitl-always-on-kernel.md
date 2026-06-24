# Registro de Decisão Arquitetural (ADR-002)

## Metadados

- **Título:** HITL sempre-on sobre o kernel, unificado em um workflow `destructive_confirm`
- **Data:** 2026-06-24
- **Status:** Aceita
- **Decisores:** Solicitante (produto/eng) + plataforma
- **Relacionados:** `prd.md` (RF-08..RF-14), `techspec.md`, ADR-001, ADR-003, ADR-004,
  `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001)

## Contexto

Hoje as quatro operações destrutivas/sensíveis (deletar último lançamento, editar último lançamento,
deletar cartão, reconfigurar/commitar budget) executam **imediatamente**, sem confirmação, pelo
caminho legacy `dispatchWrite → IntentWorkflow` (todas têm `kind.IsWrite()==true`). O PRD exige
confirmação humana antes de efetivar (RF-08), reusando suspend/resume durável e auditável.

Decisões do solicitante: construir sobre o **kernel corrigido** (ADR-001) e **promover o kernel
sempre-on** para o HITL — **sem feature flag** específica para o gate.

## Decisão

1. **Um único workflow de confirmação no kernel** — `destructive_confirm` — com `ConfirmState`
   carregando `OperationKind` (tipo fechado). `Sequence(authorize, replay, policy, audit_begin,
   prepare, confirm, execute, format)`. Os passos `prepare`/`execute` despacham por `OperationKind`
   via **mapa** (não `switch case intent.Kind`), preservando R-AGENT-WF-001.1.
2. **Roteamento por registry:** os 4 kinds destrutivos passam a resolver para o workflow HITL via um
   registry keyed-by-kind; nenhum novo `case` cresce no switch de `daily_ledger_agent.go`.
3. **Sempre ligado em produção (sem flag de gate):** o HITL nasce ativo. A mutação efetiva continua
   chamando os bindings/usecases existentes, agora **após** a aprovação.
4. **Workflow ID distinto** de `transactions_write`, tornando o resume por `(workflow, key)`
   não-ambíguo (no máximo um run suspenso por workflow por chave).

## Alternativas Consideradas

- **Caminho legacy + side-store (sem kernel):** mais simples e já em prod. **Rejeitada** pelo
  solicitante em favor de consolidar no kernel corrigido (fonte única, durabilidade/auditoria de
  primeira classe).
- **Flag dedicada (`AGENT_HITL_ENABLED`) com rollout gradual:** recomendada pela techspec por
  oferecer kill-switch. **Rejeitada** pelo solicitante (optou por sempre-on). Risco residual
  registrado abaixo.
- **Um workflow por operação (4 definitions):** mais isolamento, porém duplicação de guarda/format e
  resume ambíguo por múltiplos workflow IDs suspensos. **Rejeitada** — unificar é mais DRY e o
  `OperationKind` discrimina internamente.
- **Bundlar o GA do `transactions_write` (cutover amplo):** muito mais superfície de regressão.
  **Rejeitada** — fora do escopo do MVP; segue o plano próprio (ADR-005 do prd-workflow-kernel).

## Consequências

### Benefícios Esperados

- Confirmação durável, idempotente e auditável reusando o kernel (suspend/resume + Run + decision).
- Extensão sem `switch` de domínio (registry + mapa por `OperationKind`).
- Menor superfície que migrar todo o write para o kernel.

### Trade-offs e Custos

- Mudança de comportamento em produção **sem rollout gradual nem kill-switch** por flag.
- Acoplamento dos 4 fluxos a um estado unificado `ConfirmState` (campos opcionais por operação).

### Riscos e Mitigações

- **Risco:** sem flag, um problema no HITL afeta as 4 operações imediatamente em prod.
  **Impacto:** usuário pode não conseguir deletar/editar/commitar até reversão por deploy.
  **Mitigação:** mudança é **aditiva-de-segurança** (confirmar reduz risco, não adiciona poder
  destrutivo); cobertura de não regressão obrigatória; E2E dos 4 cenários antes do merge.
  **Rollback:** reverter o roteamento HITL → legacy (mudança localizada no agent) via deploy.
  **Risco residual aceito e registrado** por decisão explícita do solicitante.
- **Risco:** colisão de runs suspensos (categoria vs aprovação na mesma chave). **Mitigação:**
  workflow IDs distintos; ordem determinística de resume (categoria → aprovação → parse).

## Plano de Implementação

1. `ConfirmState` + tipos fechados (ADR-003).
2. Workflow `destructive_confirm` + passos novos; reuso dos passos de guarda.
3. Resolvers/executors por `OperationKind` sobre os bindings existentes.
4. Roteamento por registry + `continuePendingApproval` antes do parse.
5. Adoção concluída quando os 4 cenários (confirmar/cancelar/ambíguo/expirado) passam em E2E e os
   gates `R-AGENT-WF-001`/`R-ADAPTER-001` retornam vazio.

## Monitoramento e Validação

- Métricas `workflow_*` + `agent_intent_routed_total` com `operation`/`outcome` (enums fechados).
- Logs `agent.hitl.{suspended,confirmed,cancelled,expired,reprompt}`.
- Critério de sucesso: 0 operações destrutivas efetivadas sem confirmação; 100% de resume após
  restart simulado.
- Revisar a decisão de sempre-on se a taxa de cancelamento/fricção indicar necessidade de kill-switch.

## Impacto em Documentação e Operação

- `.claude/rules/agent-workflows-tools.md` (addendum .7 — ADR-003).
- Runbook do agent: novos estados de espera, semântica de confirmação, diagnóstico de gates.

## Revisão Futura

- Revisitar ao introduzir a Fase A (plano multi-tool) — um passo do plano pode disparar um gate HITL.
- Reabrir se operação destrutiva nova exigir gate (estende o `OperationKind`, não o switch).
