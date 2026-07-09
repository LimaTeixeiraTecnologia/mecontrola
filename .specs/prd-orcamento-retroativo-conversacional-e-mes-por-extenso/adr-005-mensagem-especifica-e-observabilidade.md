# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Mensagem específica de indisponibilidade (distinta do fallback genérico) e persistência auditável do erro
- **Data:** 2026-07-08
- **Status:** Aceita
- **Decisores:** Time de plataforma / agente financeiro
- **Relacionados:** PRD (RF-25, RF-26, RF-27, RF-30), techspec.md, ADR-001, R-AGENT-WF-001.5

## Contexto

O `WhatsAppInboundConsumer` usa um `fallbackReply` único ("não consegui concluir agora, pode repetir?", `whatsapp_inbound_consumer.go:263`) tanto para ambiguidade ("não entendi") quanto para falha de execução, sem distinção. No incidente, além do fallback genérico, a string exata do erro não ficou persistida: `platform_runs.error` veio vazio, spans sem `status=error`, logs só de outbox — lacuna de observabilidade. O PRD exige mensagem específica de indisponibilidade temporária distinta do fallback (RF-26) e que a causa do run falho seja recuperável (RF-30), mantendo fora de escopo o redesenho geral das mensagens de erro do agente.

## Decisão

1. No workflow `budget-creation` (ADR-001), a falha de persistência (`planner.CreateBudget`/`ActivateBudget` retornando erro) produz `ResponseText` **específico** de indisponibilidade temporária (distinto do `fallbackReply`), e o `BudgetCreationContinuer` retorna `handled=true` com essa mensagem — impedindo que o consumer caia no fallback genérico.
2. O step retorna o erro para o kernel (`StepStatusFailed` + erro), garantindo `Snapshot.LastError` não-vazio e `error` registrado no run auditável (corrige RF-30 no escopo deste caminho). Run auditável com `RunStatus`/`ToolOutcome` fechados (R-AGENT-WF-001.5).
3. Distinção preservada: `ErrBudgetConflict` (unicidade) → mensagem "já existe" (não é erro de indisponibilidade); erro de infraestrutura → mensagem de indisponibilidade temporária; resposta ambígua na confirmação → reprompt/cancel determinístico (não usa fallback).

## Alternativas Consideradas

- **Redesenhar o `fallbackReply` global com taxonomia de erros:** fora de escopo do PRD (redesenho geral das mensagens de erro do agente). Rejeitada; a distinção fica contida no caminho de orçamento.
- **Deixar a falha cair no fallback genérico:** viola RF-26; mantém a confusão do incidente. Rejeitada.

## Consequências

### Benefícios Esperados

- Usuário distingue "falhei ao executar" de "não entendi".
- Causa do run falho recuperável (`LastError`/run.error), fechando a lacuna de observabilidade do incidente neste caminho.

### Trade-offs e Custos

- Distinção contida ao caminho de orçamento (não global); os demais caminhos seguem com o fallback atual (aceito pelo escopo).

### Riscos e Mitigações

- **Mensagem específica não surfacada:** cenário E2E "falha ao persistir devolve mensagem específica e auditável" valida o texto distinto e o run falho auditável.

## Plano de Implementação

1. Mensagem específica no slot de confirmação do workflow em caso de erro de persistência.
2. Continuer retorna `handled=true` com a mensagem específica.
3. Garantir `Snapshot.LastError`/run.error não-vazios na falha.
4. E2E cobrindo o cenário.

## Monitoramento e Validação

- Cenário E2E de falha de persistência no gate; verificação de `run.error` não-vazio.
- Métrica de outcome do consumer distinguindo `budget_creation_error` de `not_confirmed`.

## Impacto em Documentação e Operação

- Runbook: nova mensagem de indisponibilidade e sinal de erro persistido.

## Revisão Futura

- Revisar ao eventual redesenho global das mensagens de erro do agente (hoje fora de escopo).
