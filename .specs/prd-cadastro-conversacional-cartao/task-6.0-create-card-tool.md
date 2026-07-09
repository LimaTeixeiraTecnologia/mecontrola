# Tarefa 6.0: Tool `create_card` (adapter fino de cadastro conversacional)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar a tool fina `create_card` (`internal/agents/application/tools/create_card.go`) no padrão do substrato agentivo, seguindo o template exato de `update_card.go`. A tool faz slot-filling conversacional, consulta o reconhecimento do banco (ADR-002), força a semântica determinística de dia de fechamento e, quando os dados estão completos, inicia o workflow de confirmação `card-create-confirm` via `engine.Start`. É um adapter fino: sem SQL, sem regra de negócio, sem branching de domínio além de mapeamento.

<requirements>
- `BuildCreateCardTool(engine wf.Engine[workflows.CardCreateState], def wf.Definition[workflows.CardCreateState], cards interfaces.CardManager) tool.ToolHandle` via `tool.NewTool[CreateCardInput, CreateCardOutput]`.
- Input schema `Strict:false` (como `update_card.go:39`) com `nickname`, `bank`, `dueDay`, opcional `closingDay`; declarar `minimum:1`/`maximum:31` em `dueDay`/`closingDay` (validação declarativa de schema — permitida a adapters por R-AGENT-WF-001.2; NÃO é regra de negócio em código — ADR-002).
- Output `CreateCardOutput` com `Outcome` (`needs_slot | needs_closing | needs_confirmation | pending_confirmation_exists`), `ConfirmationPrompt`, `ClarifyPrompt`.
- `exec`: `wf.RuntimeFrom(ctx)` → `agent.InboundRequest{ResourceID, MessageID}`; `ResourceID` → `userID` (RF-17, nunca user_id vindo do conteúdo).
- Slot obrigatório ausente → `needs_slot` com `ClarifyPrompt`.
- `cards.BankRecognized(ctx, bank)`: reconhecido → força `ClosingDayProvided=false` (ignora `closingDay` do LLM — RF-07 determinístico); não reconhecido + closing ausente → `needs_closing` com `ClarifyPrompt` (RF-08, slot conversacional, SEM estado durável — RF-06); não reconhecido + closing presente → `ClosingDay`+`ClosingDayProvided=true`.
- Completo → `engine.Start(ctx, def, CardCreateKey(ResourceID), state)`; em `ErrRunAlreadyExists` → `pending_confirmation_exists` (espelhar `update_card.go:144`).
- `ConfirmationPrompt` determinístico (apelido, banco, vencimento, +fechamento quando provido) para o agente relayar verbatim (RF-13).
- Adapter FINO: zero SQL, zero regra de negócio, zero branching de domínio além de mapeamento (R-ADAPTER-001.2, R-AGENT-WF-001.2). Zero comentários (R-ADAPTER-001.1).
- Requisitos cobertos: RF-01, RF-05, RF-06, RF-07, RF-08, RF-13, RF-17.
</requirements>

## Subtarefas

- [ ] 6.1 Definir `CreateCardInput`/`CreateCardOutput` e o schema JSON (`Strict:false`, `minimum:1`/`maximum:31` em `dueDay`/`closingDay`).
- [ ] 6.2 Implementar `BuildCreateCardTool` via `tool.NewTool[CreateCardInput, CreateCardOutput]`.
- [ ] 6.3 Implementar o `exec`: identidade via `RuntimeFrom`, slot-filling (`needs_slot`), gate de reconhecimento de banco (`needs_closing`/force-derive), `engine.Start` + tratamento de `ErrRunAlreadyExists`, `ConfirmationPrompt` determinístico.
- [ ] 6.4 Escrever os testes unitários da tool (testify/suite whitebox, `fake.NewProvider`, mocks do `.mockery.yml`).

## Detalhes de Implementação

Ver `techspec.md` — seções "Interfaces Chave" (`BuildCreateCardTool`, `CreateCardInput`, `CreateCardOutput`), "Reconhecimento de Banco e Dia de Fechamento (RF-07/08/09, ADR-002)", "Guardrail Anti-Alucinação (RF-13)" e "Exclusão Mútua e Ordem de Resume (RF-18)". Ver `adr-002-closing-day-optional-modeling.md` para a decisão de sentinela + `ClosingDayProvided`, o gate de reconhecimento tool-gated e o range 1..31 declarativo no schema.

Template exato: `internal/agents/application/tools/update_card.go` — `wf.RuntimeFrom(ctx)` → assert `agent.InboundRequest{ResourceID, MessageID}`, parse `ResourceID` como `uuid`, montagem do state, `engine.Start(ctx, def, key, state)`, e o bloco `errors.Is(err, wf.ErrRunAlreadyExists)` (linha 144-156). Não duplicar prosa da techspec neste arquivo.

## Critérios de Sucesso

- `go build`, `go vet` e lint do módulo `internal/agents` sem erros.
- Gate zero comentários (R-ADAPTER-001.1) e sem SQL direto (R-AGENT-WF-001.2) retornam vazio para `create_card.go`.
- `Outcome` mapeado exatamente para o conjunto fechado `needs_slot | needs_closing | needs_confirmation | pending_confirmation_exists`.
- Identidade sempre derivada de `RuntimeFrom` (RF-17); `closingDay` do LLM ignorado quando o banco é reconhecido (RF-07).
- Todos os testes da tarefa passando.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — tool fina tool.NewTool[I,O] no padrão do substrato agentivo.
- `design-patterns-mandatory` — gate de desenho do trio Go obrigatório para a tool.

## Testes da Tarefa

- [ ] Testes unitários (testify/suite, whitebox, `fake.NewProvider`, mocks do `.mockery.yml`):
  - slot obrigatório ausente → `needs_slot`.
  - banco não reconhecido sem `closingDay` → `needs_closing`.
  - banco reconhecido ignora `closingDay` do LLM (`ClosingDayProvided=false`).
  - dados completos → `engine.Start` chamado.
  - `ErrRunAlreadyExists` → `pending_confirmation_exists`.
  - identidade derivada de `RuntimeFrom` (RF-17).
- [ ] Testes de integração — N/A nesta tarefa (cobertos pelas tarefas de workflow/continuer e harness).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/agents/application/tools/create_card.go` (novo) — a tool fina.
- `internal/agents/application/tools/create_card_test.go` (novo) — suite de testes unitários.
- `internal/agents/application/tools/update_card.go` — template canônico a espelhar.
- `internal/agents/application/interfaces/card_manager.go` — `CardManager.BankRecognized` (dep. da tarefa 2.0).
- `internal/agents/application/workflows/card_create_state.go` — `CardCreateState`, `CardCreateKey` (dep. da tarefa 4.0).
- `internal/platform/agent`, `internal/platform/tool`, `internal/platform/workflow` — primitivos consumidos.

Dependências: tarefa 4.0 (engine/def do workflow `card-create-confirm`) e tarefa 2.0 (`CardManager.BankRecognized`). Paralelizável com a tarefa 5.0.
