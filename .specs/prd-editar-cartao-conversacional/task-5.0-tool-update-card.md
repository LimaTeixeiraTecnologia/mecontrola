# Tarefa 5.0: Reescrita da tool update_card para o workflow dedicado

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Reescrever a tool `update_card` para sempre iniciar o workflow `card-update-confirm`: ler o cartão atual (de-para + versão), montar `CardUpdateState`, tratar `no_changes`/`needs_closing`/`pending`/`needs_confirmation`; remover `version` do schema, a gravação direta e o uso do `destructive-confirm`.

<requirements>
- A tool exige `cardId` (obtido via `resolve_card`/`list_cards`); nunca inventa identificador.
- Lê o cartão atual via `GetCard` para capturar `Version` e valores atuais (de-para).
- Confirmação universal: sempre inicia o workflow (sem gravação direta).
- `version` removido do schema de entrada; `closing_day` aceito apenas para banco não reconhecido.
- Cobre RF-01, RF-04, RF-05, RF-06, RF-07, RF-15, RF-16, RF-17, RF-20.
</requirements>

## Subtarefas

- [ ] 5.1 Reescrever `internal/agents/application/tools/update_card.go`: schema de entrada `{cardId, nickname?, bank?, dueDay?, closingDay?}` (sem `version`); `NewTool` com outcomes fechados (`no_changes`, `needs_closing`, `needs_confirmation`, `pending_confirmation_exists`).
- [ ] 5.2 Exec: parse identidade/`cardId`; `cards.GetCard(cardID, userID)` → atual (valores + `Version`); se não encontrado, outcome de não encontrado.
- [ ] 5.3 Calcular diffs; se nada difere → `no_changes`; se banco novo não reconhecido e sem `closingDay` → `needs_closing` (espelha criação).
- [ ] 5.4 Montar `CardUpdateState` (`CardID`/`UserID` a partir do `uuid.Parse` já feito no exec — não de `atual.ID`/`atual.UserID`, que são `string`; `ExpectedVersion = atual.Version`; `Current*` do cartão atual; `New*` incluindo `NewClosingDay` quando informado; `MessageID`); `engine.Start(card-update-confirm)`; `ErrRunAlreadyExists` → `pending_confirmation_exists`; sucesso → `needs_confirmation` com a pergunta de-para.
- [ ] 5.5 Remover o branch de gravação direta e toda referência a `ConfirmState`/`destructive-confirm` na tool.

## Detalhes de Implementação

Ver `techspec.md` seção "Fluxo de Dados" e ADR-002/ADR-003. Espelhar `create_card.go` (outcomes, slot de banco não reconhecido). Depende de 2.0, 3.0 e 4.0.

## Critérios de Sucesso

- A tool nunca grava direto; sempre passa pela confirmação.
- Sem `version` no schema; identidade e valores atuais vêm do servidor.
- Adapter fino: `exec` delega a `CardManager`/engine; sem regra de negócio, SQL ou branching de domínio.
- Zero comentários em `.go`; `build`/`vet`/`lint`/`test -race` verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — tool como adapter fino do substrato de agent, com schema estrito, outcomes fechados e delegação a use case/engine, sem regra de domínio.

## Testes da Tarefa

- [ ] Testes unitários: `update_card` — `needs_closing` (banco não reconhecido sem fechamento), `no_changes`, não encontrado, `needs_confirmation` (monta estado + de-para), `pending_confirmation_exists` (`ErrRunAlreadyExists`), identidade inválida.
- [ ] Testes de integração: cobertos por 4.0 (workflow) end-to-end.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/tools/update_card.go`
- `internal/agents/application/tools/create_card.go` (molde)
- `internal/agents/application/workflows/card_update_state.go`
