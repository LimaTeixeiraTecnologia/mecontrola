# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Workflow dedicado `card-update-confirm` simétrico ao `card-create-confirm`
- **Data:** 2026-07-10
- **Status:** Aceita
- **Decisores:** Solicitante do produto, time de plataforma/agentes
- **Relacionados:** PRD `.specs/prd-editar-cartao-conversacional/prd.md` (RF-08..RF-15, RF-21..RF-24, RF-27..RF-31), techspec `.specs/prd-editar-cartao-conversacional/techspec.md`, US `docs/us/2026-07-10-us-editar-cartao-conversacional.md`, ADR-002, ADR-003

## Contexto

A edição de cartão pela conversa é frágil e assimétrica. Hoje a tool `update_card`:
- grava direto quando muda apenas `nickname`/`bank` (`update_card.go:98-116`), sem confirmação e sem idempotência;
- reusa o workflow compartilhado `destructive-confirm` (TTL 5min, `destructive_confirm_workflow.go:19`) apenas quando muda `due_day`;
- não tem Run auditável próprio, reaper próprio nem classificação de erro de domínio específica de edição.

A criação, por outro lado, tem um workflow dedicado e comprovado (`card-create-confirm`): durável (`Durable:true, MaxAttempts:1`), HITL, TTL 15min, idempotência via `IdempotentWriter`, reaper e mensagens determinísticas. O PRD exige paridade de robustez, confirmação universal, no-false-success e observabilidade auditável.

Restrições: consumir o substrato `internal/platform/workflow` sem recriá-lo; estados como tipos fechados; zero comentários em `.go`; métricas com cardinalidade controlada.

## Decisão

Criar um **workflow dedicado `card-update-confirm`** no consumidor `internal/agents`, simétrico ao `card-create-confirm`, e migrar a edição para ele:
- `BuildCardUpdateConfirmWorkflow(idem IdempotentWriter, cards interfaces.CardManager)` com `Durable:true, MaxAttempts:1`, chave `resourceID + ":card-update"`.
- Step único de avaliação que suspende com a pergunta de confirmação e, no resume, decide via `DecideCardUpdateConfirmation` (Accept/Cancel/Reprompt/Expire/Replay).
- `executeUpdateCard` grava via `IdempotentWriter.Execute(ctx, userID, wamid, 0, "update_card", "card", writeFn, isCardUpdateDomainError)`, com retry transiente (`maxWriteAttempts`), no-false-success (`StepStatusFailed` em erro não-domínio) e mensagens determinísticas.
- Confirmação **universal**: a tool `update_card` sempre inicia o workflow (remove a gravação direta).
- TTL 15min alinhado ao create; reaper `agents-card-update-reaper` (`*/5 * * * *`); Run auditável e métrica `agents_card_update_confirm_total` (rótulo `outcome` enum-only).
- `CardUpdateConfirmContinuer` retoma o run antes do agente, inserido na cadeia de resume logo após `tryContinueCardCreate`.
- Remover `OpUpdateCard` do workflow compartilhado `destructive-confirm` em todos os sites de `confirm_state.go` (constante, `String()`, `ParseOperationKind`, limite de `IsValid()`) e de `destructive_confirm_workflow.go` (`buildExecMap`, `executeUpdateCard`, `successMessage`). As demais operações do `destructive-confirm` — `OpDeleteEntry`, `OpEditEntry`, `OpDeleteCard`, `OpConfirmRegister`, `OpUpdateRecurrence`, `OpDeleteRecurrence` — continuam nele.

Escopo: apenas o canal conversacional (`internal/agents`). O endpoint REST não é afetado.

## Alternativas Consideradas

- **Estender o `destructive-confirm` compartilhado** (adicionar idempotência, de-para, confirmação universal e alinhar TTL nele).
  - Vantagens: menos arquivos novos; um único workflow de confirmação.
  - Desvantagens: acopla semânticas distintas (deletar lançamento, deletar cartão, editar recorrência, editar cartão) num só estado `ConfirmState`, dificultando idempotência por operação, de-para específico de cartão e TTL por operação; aumenta o branching interno; diverge do padrão já validado de criação.
  - Motivo da rejeição: menor isolamento e maior risco de regressão em operações não relacionadas; o create já provou que o workflow dedicado é o padrão robusto do repositório.
- **Manter a tool com gravação direta e só corrigir o payload de `due_day`.**
  - Vantagens: mínimo esforço.
  - Desvantagens: mantém mutação silenciosa de apelido/banco, sem idempotência e sem no-false-success; não atende o PRD.
  - Motivo da rejeição: não fecha os gaps de robustez exigidos.

## Consequências

### Benefícios Esperados

- Paridade de robustez com a criação: HITL universal, idempotência, no-false-success, TTL e reaper.
- Isolamento da semântica de edição de cartão; menor risco de regressão em outras operações de confirmação.
- Observabilidade auditável por Run e métrica dedicada com cardinalidade controlada.

### Trade-offs e Custos

- Mais arquivos novos (state, decisions, workflow, continuer, reaper) e um novo elo na cadeia de resume.
- Duplicação estrutural controlada com o create (mesma forma, semântica distinta) — aceitável e já é o padrão do repo.

### Riscos e Mitigações

- Risco: remover `OpUpdateCard` do enum compartilhado quebrar o `destructive-confirm`. Mitigação: é a última constante do enum (sem reordenar); cobrir com build/vet e testes existentes do destructive.
- Risco: ordem de resume incorreta (edição competir com create). Mitigação: chaves distintas (`:card-update` vs `:card-create`); inserir `tryContinueCardUpdate` imediatamente após `tryContinueCardCreate`; teste de ordenação.

## Plano de Implementação

1. Criar state/decisions/workflow (ADR-003 detalha o estado e o de-para).
2. Reescrever a tool `update_card` para sempre iniciar o workflow.
3. Continuer + reaper + wiring em `module.go` e no consumer.
4. Remover `OpUpdateCard` do `destructive-confirm`.
5. Testes unit + integration + golden.
Concluído quando: build/vet/lint/race verdes; gates R-AGENT-WF-001/R-ADAPTER-001; gate real-LLM `CategoryCard` ≥ 0,90.

## Monitoramento e Validação

- Métrica `agents_card_update_confirm_total{outcome}`; Run auditável; reaper sem runs órfãos.
- Critério de sucesso: zero mutação silenciosa; zero falso sucesso; edições confirmadas persistidas.
- Revisar se surgir uma terceira operação de edição de cartão que justifique generalização.

## Impacto em Documentação e Operação

- Runbook de agentes: registrar o workflow `card-update-confirm`, sua métrica e o reaper.
- Dashboards Grafana: adicionar painel de `agents_card_update_confirm_total` por `outcome`.

## Revisão Futura

- Revisar se o padrão de confirmação de create/update/delete de cartão convergir a ponto de valer um combinador comum, ou se um novo canal (não WhatsApp) exigir reuso.
