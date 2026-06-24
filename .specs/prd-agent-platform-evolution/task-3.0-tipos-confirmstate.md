# Tarefa 3.0: Tipos fechados + ConfirmState (domain/confirmation)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar os tipos fechados `OperationKind` e `AwaitingApproval` (DMMF state-as-type) e o estado
`ConfirmState` serializável pelo codec do kernel, em `internal/agent/domain/confirmation/`. Base do
workflow HITL (capacidade B).

<requirements>
- RF-11: estado de espera do gate usa tipo fechado (`AwaitingApproval`), nunca string livre.
- RF-23: outcomes/estados novos como tipos fechados (state-as-type).
</requirements>

## Subtarefas

- [ ] 3.1 `OperationKind` (`OperationDeleteLast`, `OperationEditLast`, `OperationDeleteCard`, `OperationBudgetCommit`) com `String()`, `IsValid()`, `ParseOperationKind`.
- [ ] 3.2 `AwaitingApproval` (`AwaitingNone`, `AwaitingConfirm`) com `String()`, `IsValid()`, `ParseAwaitingApproval`.
- [ ] 3.3 `ConfirmState` (campos da techspec) + `IsDone()`; garantir round-trip JSON estável (codec do kernel).
- [ ] 3.4 Testes de unidade dos tipos fechados (rejeitam valor inválido em `Parse*`).

## Detalhes de Implementação

Ver `techspec.md` seção "Modelos de Dados" (`ConfirmState`) e "Interfaces Chave" (tipos fechados).
Espelhar o padrão de `RunStatus`/`pendingexpense.AwaitingKind`. Zero comentários (R-ADAPTER-001.1).
Sem IO, sem `context.Context` (tipos/estado puros).

## Critérios de Sucesso

- Tipos fechados com `String()`/`IsValid()`/`Parse*` que rejeitam valor inválido.
- `ConfirmState` serializa/desserializa sem perda (round-trip JSON), compatível com `MergePatch`.
- Nenhuma regra de negócio nos tipos; apenas estado e validação de enum.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários — `Parse*` rejeita inválido; `String()` round-trip; `ConfirmState` JSON round-trip.
- [ ] Testes de integração — não aplicável (domínio puro).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agent/domain/confirmation/draft.go` (novo — tipos fechados + ConfirmState)
- `internal/agent/domain/confirmation/draft_test.go` (novo)
