# Tarefa 2.0: Interfaces e binding do agente — Version e ExpectedVersion

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Propagar `Version` (leitura) e `ExpectedVersion` (escrita) através do contrato do consumidor `internal/agents`, para que o workflow de edição capture a versão no servidor e a repasse ao módulo card.

<requirements>
- `interfaces.Card` passa a expor `Version`.
- `interfaces.CardUpdate` passa a aceitar `ExpectedVersion`.
- O binding mapeia ambos entre o agente e o módulo card.
- Cobre RF-06, RF-27.
</requirements>

## Subtarefas

- [ ] 2.1 Adicionar `Version int64` ao struct `Card` em `internal/agents/application/interfaces/types.go`.
- [ ] 2.2 Adicionar `ExpectedVersion *int64` e `ClosingDay *int` ao struct `CardUpdate` em `internal/agents/application/interfaces/types.go` (`ClosingDay` para o caso de banco não reconhecido — RF-17).
- [ ] 2.3 Em `internal/agents/infrastructure/binding/card_manager_adapter.go`: `mapCardOutput` propaga `Version`; `UpdateCard` repassa `ExpectedVersion` e `ClosingDay` ao `cardinput.UpdateCard`.
- [ ] 2.4 Regenerar o mock de `CardManager`.

## Detalhes de Implementação

Ver `techspec.md` seção "Interfaces Chave" e ADR-002. Depende da Tarefa 1.0 (campos no módulo card).

## Critérios de Sucesso

- `GetCard`/`ResolveCardByNickname`/`ListCards` no binding retornam `Version` correta.
- `UpdateCard` no binding transporta `ExpectedVersion` ao módulo card.
- `go build`/`vet`/`lint`/`test -race` verdes em `internal/agents`.
- Zero comentários em `.go` (R-ADAPTER-001.1); binding permanece fino (delegação a use case).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — evolui o contrato de consumo do substrato de agent (`CardManager`/binding) que alimenta tools e workflow; mantém o adapter fino delegando a use case.

## Testes da Tarefa

- [ ] Testes unitários: binding `card_manager_adapter` (mapeamento de `Version` e propagação de `ExpectedVersion`), padrão testify/suite + mockery.
- [ ] Testes de integração: não obrigatórios nesta tarefa (cobertos por 1.0 e 4.0).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/interfaces/types.go`
- `internal/agents/infrastructure/binding/card_manager_adapter.go`
- `internal/agents/application/interfaces/mocks/card_manager.go`
