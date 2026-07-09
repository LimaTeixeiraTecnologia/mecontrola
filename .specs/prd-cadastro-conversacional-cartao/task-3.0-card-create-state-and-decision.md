# Tarefa 3.0: Estado de espera e decisão pura do cadastro de cartão

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar o estado de espera fechado (`CardCreateState` + `CardCreateStatus`) e a função de decisão pura
`DecideCardCreateConfirmation` do workflow `card-create-confirm`, em novos arquivos sob
`internal/agents/application/workflows/`. Espelha os padrões de tipo fechado (`AwaitingKind` em
`confirm_state.go`) e de decisão pura (`DecideConfirmation` em `pending_entry_decisions.go`). A tarefa
é 100% pura (sem IO, sem `context.Context`), independente do módulo `internal/card`, e paralelizável
com a tarefa 1.0. Cobre RF-03 e RF-04 conforme techspec.md §Interfaces Chave e §Semântica de
Confirmação.

<requirements>
- Estado como tipo fechado é HARD (R-AGENT-WF-001.3): `CardCreateStatus` com `iota + 1`, `String()`,
  `IsValid()` e `ParseCardCreateStatus` que rejeita valor inválido.
- `CardCreateState` com os campos exatos da techspec.md §Interfaces Chave; reusar `AwaitingKind`.
- `DecideCardCreateConfirmation` pura (DMMF Decide*): sem IO, sem `context.Context`, determinística.
- `CardConfirmAction` como enum fechado (Accept, Cancel, Reprompt, Expire, Replay).
- Semântica de confirmação conforme techspec.md §Semântica de Confirmação (TTL 15 min, replay, sim/não,
  ambíguo com/sem reprompt).
- Zero comentários em código Go de produção (R-ADAPTER-001.1).
- Testes de tabela SEM mock para a decisão pura + ida-e-volta `String()`↔`Parse` dos enums.
</requirements>

## Subtarefas

- [ ] 3.1 Criar `card_create_state.go`: `CardCreateStatus` (`iota + 1`: Active, Completed, Cancelled,
  Expired) com `String()`/`IsValid()`/`ParseCardCreateStatus`; struct `CardCreateState` com os campos
  da techspec.md §Interfaces Chave (Status, Awaiting, UserID, Nickname, Bank, DueDay, ClosingDay,
  ClosingDayProvided, MessageID, IncomingMessageID, ProcessedMessageID, ConfirmReprompt, SuspendedAt,
  ResumeText, ResponseText, Expired).
- [ ] 3.2 Criar `card_create_decisions.go`: `CardConfirmAction` (enum fechado) e a função pura
  `DecideCardCreateConfirmation(state CardCreateState, msg PendingMessage, now time.Time) CardConfirmAction`.
- [ ] 3.3 Criar os testes de tabela sem mock ao lado dos arquivos.

## Detalhes de Implementação

Ver techspec.md §Interfaces Chave (assinaturas de `CardCreateStatus`, `CardCreateState`,
`CardConfirmAction`, `DecideCardCreateConfirmation`) e §Semântica de Confirmação (RF-03/RF-04). Ver
também ADR-001 (`adr-001-dedicated-card-create-confirm-workflow.md`) para o isolamento do workflow
dedicado e a decisão pura sem mock.

Referências de padrão a espelhar (não duplicar):
- `internal/agents/application/workflows/confirm_state.go` — `AwaitingKind` (tipo fechado a reusar) e
  o padrão `String()`/`IsValid()`/`Parse*`.
- `internal/agents/application/workflows/pending_entry_decisions.go` — `DecideConfirmation` (decisão
  pura, TTL, replay via `ProcessedMessageID`, `PendingMessage`, `reConfirmYes`/`reConfirmNo`).

Semântica de `DecideCardCreateConfirmation` (techspec.md §Semântica de Confirmação):
- TTL 15 min expirado (`now - SuspendedAt > 15min`) → `CardConfirmExpire`.
- `msg.MessageID == state.ProcessedMessageID` → `CardConfirmReplay`.
- "sim/confirmar/ok/pode" → `CardConfirmAccept`.
- "não/nao/cancelar" → `CardConfirmCancel`.
- ambíguo com `ConfirmReprompt >= 1` → `CardConfirmCancel`.
- ambíguo com `ConfirmReprompt == 0` → `CardConfirmReprompt`.

## Critérios de Sucesso

- `CardCreateStatus` e `CardConfirmAction` são tipos fechados com `iota + 1`; nenhuma representação
  como `string` solta em assinatura pública (R-AGENT-WF-001.3).
- `ParseCardCreateStatus` retorna erro em valor inválido; ida-e-volta `String()`↔`Parse` estável.
- `DecideCardCreateConfirmation` é pura: sem IO, sem `context.Context`, determinística — testável sem
  mock algum.
- `card_create_decisions.go` e `card_create_state.go` sem comentários (R-ADAPTER-001.1).
- `go build`, `go vet`, `go test -race` e lint verdes no pacote `internal/agents/application/workflows`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `domain-modeling-production` — state-as-type (CardCreateState/CardCreateStatus) e função Decide pura (DMMF).
- `design-patterns-mandatory` — gate de desenho do trio Go obrigatório para os tipos fechados.
- `mastra` — estado de espera do workflow no padrão do substrato agentivo.

## Testes da Tarefa

- [ ] Testes unitários: table-tests SEM mock para `DecideCardCreateConfirmation` cobrindo
  accept/cancel/reprompt→cancel/expire/replay; ida-e-volta `String()`↔`Parse` de `CardCreateStatus`
  (e do `CardConfirmAction` quando aplicável) + erro no valor inválido.
- [ ] Testes de integração: não se aplicam a esta tarefa (lógica pura, sem IO).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/card_create_state.go` (novo)
- `internal/agents/application/workflows/card_create_decisions.go` (novo)
- `internal/agents/application/workflows/card_create_state_test.go` (novo)
- `internal/agents/application/workflows/card_create_decisions_test.go` (novo)
- `internal/agents/application/workflows/confirm_state.go` (referência — `AwaitingKind`)
- `internal/agents/application/workflows/pending_entry_decisions.go` (referência — `DecideConfirmation`)
