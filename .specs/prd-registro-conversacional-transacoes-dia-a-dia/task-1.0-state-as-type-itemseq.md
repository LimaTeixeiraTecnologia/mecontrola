# Tarefa 1.0: State-as-type `PendingOperationKind` + campo `ItemSeq` no `PendingEntryState`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Preparar a fundação da idempotência durável: dar contrato state-as-type completo ao `PendingOperationKind` (fonte do componente `operation` da chave) e adicionar o campo `ItemSeq` ao `PendingEntryState`, populado por `RegisterAttempt`. Sem esses dois, a chave `(wamid, itemSeq, operation)` não pode ser montada em `executeWrite` (Tarefa 2.0).

<requirements>
- RF-19: chave `(wamid, itemSeq, operation)` — este passo entrega `itemSeq` no estado e `operation` via `String()` tipado.
- RF-20: idempotência ancorada no wamid original — o estado já carrega `MessageID` (wamid original); aqui garantimos os demais componentes da chave.
- DMMF state-as-type: `PendingOperationKind` fechado com `String()`/`IsValid()`/`ParsePendingOperationKind` (ver `techspec.md` › Modelos de Dados e ADR-001).
- Campo aditivo, compatível com snapshots antigos (zero-value `0`).
</requirements>

## Subtarefas

- [ ] 1.1 Garantir/implementar `func (k PendingOperationKind) String() string` com switch exaustivo mapeando para `register_expense`|`register_income`|`edit_entry`|`create_recurrence`.
- [ ] 1.2 Implementar `IsValid()` por faixa e `ParsePendingOperationKind(s string) (PendingOperationKind, error)` que rejeita valor desconhecido (smart constructor).
- [ ] 1.3 Adicionar `ItemSeq int` a `PendingEntryState` (`pending_entry_state.go`).
- [ ] 1.4 Popular `ItemSeq` em `RegisterAttempt.RegisterExpense`/`RegisterIncome`/`CreateRecurrence`/`EditEntry` a partir do `ItemSeq` do comando (MVP = 0).
- [ ] 1.5 Testes de ida e volta `String()`↔`ParsePendingOperationKind` + erro em valor inválido.

## Detalhes de Implementação

Ver `techspec.md` › **Modelos de Dados** (contrato `PendingOperationKind`, campo `ItemSeq`) e **ADR-001** (por que `ItemSeq` é necessário no estado; wamid original em `MessageID`). Não duplicar aqui.

## Critérios de Sucesso

- `PendingOperationKind` é tipo fechado com `String()` exaustivo, `IsValid()` e smart constructor que rejeita valor inválido.
- `PendingEntryState.ItemSeq` existe e é populado por todos os construtores de `RegisterAttempt`.
- `go build ./internal/agents/...` e `go vet` limpos; zero comentários em `.go` de produção (R-ADAPTER-001.1).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — altera estado fechado e workflow do consumidor agentivo (`internal/agents`), primitivo state-as-type do stack Mastra Go.

## Testes da Tarefa

- [ ] Testes unitários (`String()`↔`Parse` ida e volta, erro em inválido; `RegisterAttempt` popula `ItemSeq`)
- [ ] Testes de integração (n/a — coberto no harness em 8.0)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/pending_entry_state.go` — `PendingOperationKind`, `ItemSeq`.
- `internal/agents/application/usecases/register_attempt.go` — popular `ItemSeq`.
- `internal/agents/application/usecases/register_entry.go` — comandos (fonte do `ItemSeq`).
