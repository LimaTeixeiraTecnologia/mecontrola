# Golden Sets: Pending Entry Workflow

Este documento define os cenarios de referencia para o workflow `pending-entry` em `internal/agents/application/workflows/pending_entry_workflow.go`. Os cenarios cobrem transicoes de estado criticas e devem ser mantidos alinhados aos testes em `pending_entry_workflow_test.go`.

## Cenario 1: Primeira entrada suspende aguardando slot

Entrada:
- Estado inicial: `PendingEntryState{Awaiting: AwaitingSlotCategory, ResumeText: ""}`
- ResourceID/threadID validos.

Execucao:
- `makePendingEntryStep` detecta `ResumeText == ""`.
- Preenche `ResponseText` com prompt do slot.
- Retorna `StepStatusSuspended` com `SuspendReason == SuspendAwaitingInput`.

Saida esperada:
- `output.Status == StepStatusSuspended`
- `output.State.ResponseText` nao vazio.
- `output.State.SuspendedAt` preenchido.

## Cenario 2: Cancelamento explicito

Entrada:
- Estado: `PendingEntryState{Awaiting: AwaitingSlotCard, ResumeText: "cancelar"}`

Execucao:
- Usuario envia mensagem de cancelamento.
- `handleCardSlotResume` detecta `isCancelMessage(text) == true`.

Saida esperada:
- `output.Status == StepStatusCompleted`
- `output.State.Status == PendingStatusCancelled`
- `output.State.ResponseText == "Tudo certo, o registro foi cancelado."`

## Cenario 3: Expiracao por timeout

Entrada:
- Estado: `PendingEntryState{Awaiting: AwaitingSlotCard, ResumeText: "nubank", SuspendedAt: now.Add(-40 * time.Minute)}`

Execucao:
- `handleCardSlotResume` detecta `isExpired(state, now) == true`.

Saida esperada:
- `output.Status == StepStatusCompleted`
- `output.State.Status == PendingStatusExpired`
- `output.State.ResponseText` informa expiracao.

## Cenario 4: Resposta substitui operacao

Entrada:
- Estado: `PendingEntryState{Awaiting: AwaitingSlotCard, ResumeText: "gastei 50 mercado"}`
- Texto reconhecido como nova operacao completa.

Execucao:
- `handleCardSlotResume` detecta `isNewCompleteOperation(text) == true`.

Saida esperada:
- `output.Status == StepStatusCompleted`
- `output.State.Status == PendingStatusReplaced`
- `output.State.ResponseText` vazio para permitir novo workflow.

## Criterios de manutencao

- Todo novo estado `PendingStatus*` ou `Awaiting*` deve ter golden set.
- Transicoes `AwaitingSlotCategory -> AwaitingSlotCard -> AwaitingSlotConfirmation` devem ser cobertas.
- Nenhuma string solta em campos de status; todos devem usar constantes tipadas.
