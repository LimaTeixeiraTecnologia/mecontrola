# Tarefa 2.0: Interfaces e binding do consumidor agents (CardManager + NewCard)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Estender as interfaces do consumidor `internal/agents` e o binding do `CardManager` para propagar o dia
de fechamento explícito opcional (sentinela `ClosingDay int` + `ClosingDayProvided bool`, ADR-002) e
expor o sinal de reconhecimento de banco (`BankRecognized`) à camada agentiva, delegando à leitura
`IsBankRecognized` entregue na task 1.0. Ver `techspec.md` (§ Interfaces Chave, § Reconhecimento de
Banco e Dia de Fechamento) e `adr-002-closing-day-optional-modeling.md`.

<requirements>
- RF-07, RF-08, RF-09, RF-20 (ver prd.md e techspec.md § Mapeamento Requisito → Decisão → Teste).
- Depende da task 1.0 (leitura `IsBankRecognized` + branch por `ClosingDayProvided` no módulo `internal/card`).
- Zero comentários em Go de produção (R-ADAPTER-001.1); adapter fino sem regra de negócio, SQL ou branching de domínio (R-ADAPTER-001.2 / R-AGENT-WF-001.2).
- Estender contrato sem alterar assinaturas existentes além do necessário; mudança aditiva.
</requirements>

## Subtarefas

- [x] 2.1 Estender `internal/agents/application/interfaces/types.go` `NewCard` (linha ~122) com `ClosingDay int` e `ClosingDayProvided bool`.
- [x] 2.2 Adicionar `BankRecognized(ctx context.Context, bank string) (bool, error)` à interface `CardManager` em `internal/agents/application/interfaces/card_manager.go`.
- [x] 2.3 Atualizar `internal/agents/infrastructure/binding/card_manager_adapter.go` `CreateCard` (linha ~58): mapear `ClosingDay`/`ClosingDayProvided` para `cardinput.CreateCard`.
- [x] 2.4 Implementar `BankRecognized` no `card_manager_adapter` delegando à leitura `IsBankRecognized` (task 1.0) do módulo card (via novo usecase fino `internal/card/application/usecases/is_bank_recognized.go`).
- [x] 2.5 `.mockery.yml` já declarava `CardManager: {}`; rodado `task mocks` e regenerado `internal/agents/application/interfaces/mocks/card_manager.go`.

## Detalhes de Implementação

Ver `techspec.md` (§ Design de Implementação → Interfaces Chave; § Reconhecimento de Banco e Dia de
Fechamento (RF-07/08/09, ADR-002); § Arquivos Relevantes e Dependentes) e
`adr-002-closing-day-optional-modeling.md` (Decisão itens 1, 3, 4; Plano de Implementação item 4). Não
duplicar a prosa da techspec aqui.

## Critérios de Sucesso

- `NewCard` e `CardManager` estendidos conforme a assinatura da techspec; `card_manager_adapter` mapeia os novos campos e implementa `BankRecognized` delegando à leitura do módulo card.
- `internal/agents` compila; `build`, `vet` e `lint` do módulo alterado passam sem violar R-ADAPTER-001.1/.2.
- Mocks regenerados a partir do `.mockery.yml` e compilando (`task mocks`).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — extensão do binding/interfaces do consumidor agentivo (CardManager) sobre o substrato.
- `design-patterns-mandatory` — gate de desenho do trio Go obrigatório para as interfaces e o adapter.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

Cobertura mínima (unit): teste do `card_manager_adapter` verificando que `CreateCard` propaga
`ClosingDay`/`ClosingDayProvided` para `cardinput.CreateCard`, e que `BankRecognized` delega à leitura
`IsBankRecognized`; mocks regenerados e compilando.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/agents/application/interfaces/types.go` — `NewCard` (+ `ClosingDay`, `ClosingDayProvided`).
- `internal/agents/application/interfaces/card_manager.go` — `CardManager` (+ `BankRecognized`).
- `internal/agents/infrastructure/binding/card_manager_adapter.go` — `CreateCard` (mapear campos) + `BankRecognized` (delegar à task 1.0).
- `.mockery.yml` — atualizar se nova interface/método exigir mock.
- `internal/agents/application/interfaces/mocks/card_manager.go` — mock regenerado via `task mocks`.
