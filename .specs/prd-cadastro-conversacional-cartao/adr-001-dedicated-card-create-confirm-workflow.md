# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Workflow de confirmação dedicado para cadastro de cartão (`card-create-confirm`)
- **Data:** 2026-07-08
- **Status:** Aceita
- **Decisores:** Usuário (product owner), engenharia de plataforma agentiva
- **Relacionados:** PRD `.specs/prd-cadastro-conversacional-cartao/prd.md` (RF-01, RF-02, RF-03, RF-04, RF-18, RF-21); techspec `techspec.md`; R-AGENT-WF-001; R-WF-KERNEL-001; `internal/agents/application/workflows/destructive_confirm_workflow.go`

## Contexto

O cadastro de cartão por conversa (RF-02) exige confirmação humana explícita antes da escrita, com
estado de espera fechado, persistido no `Snapshot` do kernel antes de perguntar, retomado por
merge-patch antes do parse, com TTL, semântica sim/não/ambíguo e limpeza determinística.

O repositório já possui um gate de confirmação genérico — `destructive_confirm_workflow.go`
(`ConfirmState` + enum `OperationKind` com `map[OperationKind]executeFn`) — que já recebe
`CardManager`, já cobre operações de cartão (`OpDeleteCard`, `OpUpdateCard`), inclui criação não
destrutiva (`OpConfirmRegister`) e tem TTL de 5 min (`confirmTTL`), reaper (`ConfirmReaperJob`) e
posição no resume chain (`DestructiveConfirmContinuer`, antes do `ParseInbound`).

Foram avaliadas duas abordagens: (A) estender o gate genérico com `OpCreateCard`; (B) criar um
workflow dedicado `card-create-confirm`. A decisão do product owner foi (B).

## Decisão

Criar um workflow **dedicado** `card-create-confirm` no consumidor `internal/agents`, sobre o kernel
genérico `internal/platform/workflow` (`Engine[CardCreateState]`), com:

- Estado próprio `CardCreateState` (tipo fechado, state-as-type), serializado no `Snapshot`.
- TTL próprio de **15 min** (`cardCreateConfirmTTL`), isolado do gate destrutivo (que permanece 5 min).
- Continuer próprio (`CardCreateConfirmContinuer`) inserido no resume chain do
  `WhatsAppInboundConsumer` **antes** do `ParseInbound`, após `pending_entry` e `destructive_confirm`.
- Reaper próprio via `workflow.NewStaleSuspendedReaper(store, CardCreateConfirmWorkflowID, ...)`.
- Função de decisão pura `DecideCardCreateConfirmation` (sem IO, testável sem mock).

O workflow NÃO importa domínio no kernel, NÃO contém SQL, NÃO invoca LLM (R-WF-KERNEL-001); a escrita
é delegada ao `IdempotentWriter` dos agents (ver ADR-003), que encapsula `CardManager.CreateCard`.

## Alternativas Consideradas

### (A) Estender `destructive_confirm_workflow` com `OpCreateCard`

- **Descrição:** adicionar `OpCreateCard` ao enum `OperationKind`, ao `execMap`, a `successMessage` e
  um `executeCreateCard`, espelhando `update_card.go`.
- **Vantagens:** DRY máximo; reuso de suspend/resume, semântica sim/não/ambíguo, reaper e resume
  chain; exclusão mútua automática via chave única `resourceID:confirm`; menos código novo.
- **Desvantagens:** o TTL de 15 min do cadastro exigiria TTL por operação no gate compartilhado
  (mudança em caminho usado por operações destrutivas em produção); mistura semântica de "confirmação
  destrutiva/sensível" com "criação"; qualquer regressão no gate afeta deletar/editar cartão e
  lançamento simultaneamente.
- **Motivo de rejeição:** o product owner priorizou isolamento total de risco sobre os fluxos
  destrutivos já em produção e independência de TTL/estado, aceitando o custo de duplicação de máquina.

### (B) Workflow dedicado `card-create-confirm` (escolhida)

- **Descrição:** máquina de confirmação própria, estado próprio, TTL próprio, continuer e reaper
  próprios.
- **Vantagens:** zero risco de regressão nos fluxos destrutivos existentes; TTL e estado 100%
  independentes; semântica de criação isolada; fiel ao texto do PRD (RN2).
- **Desvantagens:** duplica a máquina de confirmação (step eval, continuer, reaper, wiring); adiciona
  um terceiro caminho de resume a coordenar no consumer.

## Consequências

### Benefícios Esperados

- Isolamento de blast radius: fluxos destrutivos permanecem intocados (TTL 5 min, comportamento atual).
- Estado card-scoped fechado, sem qualquer acoplamento a `TransactionsLedger`/`categoryValidator`
  (satisfaz RN2 na íntegra).
- TTL de 15 min honrado trivialmente (constante do próprio workflow).

### Trade-offs e Custos

- Duplicação da mecânica de confirmação (eval step, continuer, reaper, wiring em `module.go`).
- Um terceiro caminho de resume no `WhatsAppInboundConsumer`, exigindo ordem determinística e testes
  de exclusão mútua (RF-18).
- Manutenção futura: correções de semântica de confirmação precisam ser replicadas entre os gates.

### Riscos e Mitigações

- **Risco:** dois estados de espera suspensos simultâneos (pending_entry e card-create) causarem
  ambiguidade de resume. **Mitigação:** a cadeia resume-consome-mensagem garante que um novo
  `card-create` só é iniciado (via tool no `handleAgentInbound`, último da cadeia) quando nenhum
  outro gate está suspenso; ordem determinística documentada; teste de integração cobrindo o cenário.
- **Risco:** run suspenso órfão. **Mitigação:** reaper dedicado + limpeza determinística
  (run sempre concluído após efetivar/cancelar/expirar).
- **Rollback:** remover o continuer do resume chain e desregistrar a tool `create_card`; o workflow
  dedicado é aditivo e não altera fluxos existentes.

## Plano de Implementação

1. `CardCreateState` + enums fechados (`CardCreateStatus`; reuso de `AwaitingKind`).
2. `DecideCardCreateConfirmation` pura + testes de tabela sem mock.
3. `BuildCardCreateConfirmWorkflow(idem, cards)` (`Definition[CardCreateState]`, Durable, MaxAttempts=1).
4. `CardCreateConfirmContinuer` + reaper + wiring em `module.go` e no `WhatsAppInboundConsumer`.
5. Tool `create_card` que inicia o workflow (ver techspec §Design de Implementação).

## Monitoramento e Validação

- Métrica `agents_write_total{operation="create_card",outcome}` (ADR-003).
- Métricas de kernel `workflow_suspend_total`/`workflow_resume_total{workflow="card-create-confirm"}`.
- Contador do continuer análogo a `agents_destructive_confirm_total`.
- Critério de sucesso: harness real-LLM ≥ 0.90 + teste determinístico de regressão do incidente (RF-22).
- Revisar se, em 2 gates de confirmação, a duplicação justificar unificação futura.

## Impacto em Documentação e Operação

- Runbook do agente: novo fluxo de cadastro de cartão + estado suspenso `card-create-confirm`.
- Dashboards: painel de `agents_write_total{operation="create_card"}` e suspend/resume do novo workflow.
- Alertas: run suspenso além de TTL para `card-create-confirm`.

## Revisão Futura

- Reavaliar unificação dos gates de confirmação se um terceiro gate de "criação com confirmação"
  surgir, ou se a duplicação gerar divergência de semântica entre os gates.
