# Tarefa 4.0: Workflow de confirmação `card-create-confirm` + escrita idempotente

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar o workflow durável dedicado `card-create-confirm` em `internal/agents/application/workflows/card_create_confirm_workflow.go`, sobre o kernel `internal/platform/workflow` (`Engine[CardCreateState]`), que confirma o cadastro de cartão antes da escrita e efetiva a criação via `IdempotentWriter` dos agents. Segue o padrão de `destructive_confirm_workflow.go` (step eval suspend/resume) e de `pending_entry_workflow.go` (`executeWithIdempotency`) como template. Ver techspec.md §Design de Implementação, §Semântica de Confirmação, §Idempotência/Auditoria/Métrica; ADR-001 (workflow dedicado); ADR-003 (idempotência + métrica via `IdempotentWriter`, `operation="create_card"`).

<requirements>
- `BuildCardCreateConfirmWorkflow(idem interfaces.IdempotentWriter, cards interfaces.CardManager) workflow.Definition[CardCreateState]` — `Durable: true`, `MaxAttempts: 1`.
- Step eval: se `ResumeText == ""` → suspende com prompt de confirmação, persistindo o `Snapshot` ANTES de perguntar (R-AGENT-WF-001.7).
- No resume, decidir via `DecideCardCreateConfirmation` (task 3.0): Accept → `executeCreateCard`; Cancel → mensagem de cancelamento, `StepStatusCompleted`; Reprompt → re-suspende uma única vez (`cardCreateMaxReprompts`); Expire → `StepStatusCompleted`, `handled=false` (texto segue ao `ParseInbound`); Replay → `StepStatusCompleted`.
- Run sempre concluído (`RunStatusSucceeded`/`RunStatusFailed`) após decisão — nunca permanece suspenso (RF-21).
- `executeCreateCard` chama `idem.Execute(ctx, state.UserID, state.MessageID, 0, "create_card", "card", writeFn)`; `writeFn` → `cards.CreateCard(ctx, interfaces.NewCard{...ClosingDay/ClosingDayProvided...})`, retornando `(cardID uuid.UUID, false, err)` — RF-14 (replay) + RF-16 (`agents_write_total{operation="create_card",outcome}`) num único mecanismo (ADR-003).
- `ErrNicknameConflict` e erros de validação → outcome de domínio + mensagem acionável + run concluído (RF-12; não é falha silenciosa). Erros de infra → retry transiente (`IsTransient`) e, persistindo, `RunStatusFailed` com erro persistido.
- Zero comentários; sem SQL/LLM no workflow; sem domínio no kernel (R-ADAPTER-001.1, R-AGENT-WF-001.2/.4, R-WF-KERNEL-001).
- Requisitos cobertos: RF-02, RF-12, RF-14, RF-16, RF-21.
- Depende de 2.0 (`interfaces.CardManager`/`NewCard`) e 3.0 (`CardCreateState` + `DecideCardCreateConfirmation`).
</requirements>

## Subtarefas

- [ ] 4.1 Constantes: `CardCreateConfirmWorkflowID = "card-create-confirm"`, `cardCreateConfirmTTL = 15 * time.Minute`, `cardCreateMaxReprompts = 1`, e `CardCreateKey(resourceID string) string` = `resourceID + ":card-create"`.
- [ ] 4.2 `BuildCardCreateConfirmWorkflow` + step eval (suspend antes de perguntar; resume por `DecideCardCreateConfirmation`; run sempre concluído).
- [ ] 4.3 `executeCreateCard` via `idem.Execute(..., "create_card", "card", writeFn)`; mapear conflito de apelido/validação → outcome de domínio + mensagem acionável; infra → retry transiente e `RunStatusFailed` com erro.
- [ ] 4.4 Testes unitários table-tests (testify/suite) do workflow.

## Detalhes de Implementação

Ver techspec.md §Design de Implementação (assinaturas `CardCreateConfirmWorkflowID`, `cardCreateConfirmTTL`, `cardCreateMaxReprompts`, `CardCreateKey`, `BuildCardCreateConfirmWorkflow`), §Semântica de Confirmação (RF-03/RF-04) e §Idempotência, Auditoria e Métrica (RF-14/15/16). Ver ADR-001 (workflow dedicado, TTL 15 min isolado do gate destrutivo) e ADR-003 (chave de idempotência = `wamid` = `state.MessageID`, `itemSeq = 0`, `operation="create_card"`, distinção domínio vs infra). Template estrutural: `internal/agents/application/workflows/destructive_confirm_workflow.go` (step eval suspend/resume) e `internal/agents/application/workflows/pending_entry_workflow.go` (`executeWithIdempotency`). Não duplicar prosa dos documentos — referenciá-los.

## Critérios de Sucesso

- `go build`, `go vet`, `go test -race` e lint do módulo `internal/agents` passam.
- Suspensão persiste `CardCreateState` no `Snapshot` antes de qualquer pergunta; resume aplica merge-patch antes do parse.
- Accept invoca `writeFn` via `idem.Execute` com `operation="create_card"`; conflito de apelido produz mensagem acionável e run concluído; falha de infra produz `RunStatusFailed` com erro persistido.
- Nenhum caminho deixa o run em `RunStatusSuspended` após uma decisão (RF-21).
- Zero comentários no arquivo de produção; sem SQL/LLM no workflow.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — workflow durável sobre Engine[S] do kernel e escrita idempotente no padrão do substrato.
- `design-patterns-mandatory` — gate de desenho do trio Go obrigatório para o workflow.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

Cobertura mínima (table-tests testify/suite, whitebox `package workflows`, `fake.NewProvider()`, mocks do `.mockery.yml`):
- Suspensão persiste o estado (`StepStatusSuspended`, `Snapshot` gravado) antes de perguntar.
- Accept → `writeFn` invocado via `idem.Execute` com `operation="create_card"`.
- `ErrNicknameConflict` → mensagem de domínio acionável + run concluído (não silencioso).
- Expiração por TTL → `handled=false`, texto segue ao `ParseInbound`.
- Reprompt (ambíguo 1ª vez) → re-suspende; ambíguo 2ª vez → cancela, run concluído.
- Nenhuma decisão deixa o run em `RunStatusSuspended`.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/agents/application/workflows/card_create_confirm_workflow.go` (novo) — workflow + `executeCreateCard`.
- `internal/agents/application/workflows/card_create_confirm_workflow_test.go` (novo) — testes unitários.
- `internal/agents/application/usecases/idempotent_write.go` (consumido) — `IdempotentWriter`.
- `internal/agents/application/workflows/card_create_state.go` + `card_create_decisions.go` (task 3.0) — `CardCreateState`, `DecideCardCreateConfirmation`.
- `internal/agents/application/interfaces/{types.go,card_manager.go}` (task 2.0) — `NewCard`, `CardManager`.
- Referência de template: `internal/agents/application/workflows/destructive_confirm_workflow.go`, `internal/agents/application/workflows/pending_entry_workflow.go`.
