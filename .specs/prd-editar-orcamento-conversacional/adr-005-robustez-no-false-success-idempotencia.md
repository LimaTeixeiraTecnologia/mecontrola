# Registro de Decisão Arquitetural (ADR)

## Metadados
- **Título:** Robustez — no-false-success (`StepStatusFailed`), idempotência por `wamid`, alertas no ciclo do job
- **Data:** 2026-07-10
- **Status:** Aceita
- **Decisores:** Plataforma / autor da techspec
- **Relacionados:** PRD (RF-29..RF-36, RF-40, R5, R6, R7, E4); techspec.md; ADR-004; incidentes anteriores de falso-sucesso

## Contexto
Edições são mutações financeiras conversacionais e precisam de garantias de robustez idênticas às do `budget-creation`, evitando os defeitos já vistos em produção: reportar sucesso quando a escrita falhou; reaplicar em reenvio; prender o usuário no fluxo. O PRD exige (RF-30) TTL 30min, (RF-31) reaper, (RF-32) idempotência por `wamid`, (RF-34) mensagem específica de indisponibilidade, (RF-35) falha-segura sem falso sucesso, (RF-36) alertas no ciclo normal do job, e (E4) cancelamento em qualquer passo.

## Decisão
1. **No-false-success:** quando `planner.<op>` retorna erro não-de-domínio, o step retorna `workflow.StepStatusFailed` **com** o erro (sem gravar recurso, sem responder sucesso). O continuer, ao receber `RunStatusFailed` com `ResponseText` preenchido, entrega ao usuário a mensagem específica de indisponibilidade (RF-34) e fecha o Run como `RunStatusFailed` (observável, pesquisável). Erros de domínio conhecidos (ex.: `ErrBudgetNotFound` no confirm) completam com mensagem apropriada, sem falso sucesso.
2. **Idempotência por `wamid`:** o passo de confirmação compara `IncomingMessageID` com `State.MessageID`; reenvio do mesmo `wamid` já processado retorna `BudgetEditActionReplay` (no-op, sem segunda mutação) — mesmo mecanismo do `budget-creation` (`DecideBudgetConfirmation`). A escrita em si é única por execução do run confirmado; não há segundo caminho de escrita.
3. **TTL + reaper:** `budgetEditTTL = 30min`; expiração avaliada no resume (`now - SuspendedAt > TTL`) → completa sem efeito e devolve `handled=false` (texto segue para `ParseInbound`). Reaper dedicado `BuildBudgetEditReaper` (`StaleAfter=35min`) registrado no slice de Jobs, purgando runs suspensos abandonados.
4. **Cancelamento em qualquer passo (E4):** `isBudgetEditConfirmNo` reusa `reConfirmNo`/`isCancelMessage` (`pending_entry_decisions.go:143-150`); frases de cancelamento reconhecidas na coleta e na confirmação encerram o run sem efeito.
5. **Reprompt único (RF-26):** `budgetEditMaxReprompts = 1`; ambiguidade 1x re-pergunta, 2x cancela.
6. **Alertas (RF-36/R5):** a edição só persiste o novo plano; nenhum disparo/reavaliação imediata de alertas — o `ThresholdAlertsJob` recomputa no ciclo normal. Sem acoplamento novo entre edição e motor de alertas.

## Alternativas Consideradas
- **Idempotência via write ledger (`IdempotentWrite`)** como no registro de transações: robusto, mas o `budget-creation` já garante unicidade por run + replay por `messageID`, e a edição é uma escrita única por confirmação. Adotado o mesmo mecanismo do create (menor custo, paridade). Ledger fica como evolução se surgir escrita multi-item.
- **Reavaliar alertas imediatamente após editar:** feedback mais rápido, mas acopla edição ao motor de alertas e amplia risco. Rejeitada (R5).
- **Swallow do erro de escrita com mensagem genérica:** reintroduz falso-sucesso. Rejeitada explicitamente.

## Consequências
### Benefícios Esperados
- Zero falso-sucesso (RF-35/RF-40): falha vira `StepStatusFailed` observável.
- Reenvio não duplica; usuário nunca preso (TTL/cancel).
- Sem novo acoplamento com alertas.

### Trade-offs e Custos
- Em indisponibilidade, o usuário precisa refazer a edição depois (aceitável; melhor que falso-sucesso).

### Riscos e Mitigações
- Risco: erro de escrita classificado como sucesso por engano. Mitigação: teste real-LLM/integração que injeta falha do planner e assert `StepStatusFailed` + ausência de recurso.

## Plano de Implementação
1. `executeBudgetEdit` retorna `StepStatusFailed` + erro em falha não-de-domínio; mensagem específica em `ResponseText`.
2. Continuer trata `RunStatusFailed` com `ResponseText` (entrega msg, fecha Run failed).
3. TTL/reaper/replay espelhando `budget-creation`.
4. Teste de injeção de falha (planner indisponível) no gate.

## Monitoramento e Validação
- Métrica `agents_budget_edit_total{outcome="error"|"expired"|"replied"|"completed"|"cancelled"}`.
- Span/Run `RunStatusFailed` pesquisável por `wamid`/`run_id`.
- Gate: cenário "planner indisponível não gera falso sucesso" verde.

## Impacto em Documentação e Operação
- Runbook: interpretar `outcome=error` e Runs `failed` do `budget-edit`.

## Revisão Futura
- Promover para write ledger dedicado se a edição evoluir para múltiplos itens por mensagem.
