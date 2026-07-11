# Tarefa 7.0: Agente — alargar `edit_entry` (paridade) + comando/estado/`buildRawUpdate` + re-resolução categoria + `WithWriteToolSet`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechar a paridade de campos da edição conversacional no caminho vivo (`pending-entry`, `PendingOpEditEntry`): alargar o schema de `edit_entry`, propagar os campos por `EditEntryCommand → PendingEntryState → buildRawUpdate`, re-resolver categoria quando categoria/direção mudam, e submeter a edição ao write-tool set (ADR-002).

<requirements>
- RF-05/RF-06: campos editáveis por despesa (valor, descrição, data, categoria, forma de pagamento, cartão, parcelas, direção) e por receita (sem pagamento/cartão).
- RF-07: múltiplos campos do mesmo alvo em uma mensagem.
- RF-08: guarda de múltiplas transações por mensagem permanece.
- RF-09..RF-11: slot-filling reusado (perguntas verbatim; um campo por vez; descrição literal).
- RF-12: editar categoria dispara `classify_category`; múltiplos candidatos → lista.
- RF-13: migração para crédito exige `resolve_card`; cartão não encontrado → listar/pedir escolha; `cardId` nunca inventado.
- RF-14: mudar direção re-resolve categoria compatível com o kind.
- RF-15: migração para fora de crédito bloqueada com parcelas em aberto.
- RF-03/RF-04: alvo do próprio usuário (ownership); mensagens distintas para inexistente vs soft-deleted.
- RF-31: roteamento por registry (sem `switch case intent.Kind`); incluir `edit_entry` em `WithWriteToolSet`.
- Tool fina; `TargetVersion` carregada do alvo (ADR-003/ADR-004).
</requirements>

## Subtarefas

- [ ] 7.1 Alargar `EditEntryInput` + schema de `edit_entry` (novos campos + `version`) mantendo adapter fino (`edit_entry.go`).
- [ ] 7.2 Estender `EditEntryCommand` (ponteiros por campo + `TargetVersion` + `CardNickname`/`CategoryTerm`/`Direction`/`PaymentMethod`/`Installments`) e `RegisterAttempt.EditEntry` para propagar; re-resolução de categoria quando categoria ou direção mudam.
- [ ] 7.3 Estender `PendingEntryState` (campos-alvo de edição) e `buildRawUpdate` para mapear os campos novos ao `RawUpdateTransaction`.
- [ ] 7.4 Ownership + mensagens distintas (não localizado vs já excluído) no fluxo de edição.
- [ ] 7.5 Incluir `edit_entry` em `agent.WithWriteToolSet(...)` (`module.go`).
- [ ] 7.6 Testes testify/suite do `RegisterAttempt.EditEntry` e das decisões de slot para os campos novos.

## Detalhes de Implementação

Ver `techspec.md` (Interfaces Chave — `EditEntryCommand`; Fluxo de Dados) e `adr-002`. Caminho canônico = `pending-entry`; `destructive_confirm.OpEditEntry` permanece intocado.

## Critérios de Sucesso

- Cada campo editável por conversa chega ao `UpdateTransaction` correto, respeitando invariantes de domínio.
- `edit_entry` coberta pelo write-tool set; sem `switch case intent.Kind`.
- Gates R-AGENT-WF-001 (1/2/3), R-ADAPTER-001 verdes.
- `go build`, `go vet`, `go test -race`, lint do módulo verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — alargar tool + roteamento por registry + slot-filling no substrato de agente.
- `domain-modeling-production` — estados fechados (`PendingOperationKind`/`AwaitingSlot`) e propagação sem vazar regra para o adapter.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/tools/edit_entry.go`
- `internal/agents/application/usecases/register_attempt.go`
- `internal/agents/application/workflows/pending_entry_state.go`
- `internal/agents/application/workflows/pending_entry_workflow.go`
- `internal/agents/module.go`
- `internal/agents/application/agents/mecontrola_agent.go`
