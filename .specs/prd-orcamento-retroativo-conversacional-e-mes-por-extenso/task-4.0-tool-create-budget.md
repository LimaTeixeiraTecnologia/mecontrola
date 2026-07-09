# Tarefa 4.0: Tool fina create_budget (inicia workflow) + DTO Validate + mapeamento MonthReference

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar a Tool fina `create_budget` (`internal/agents/application/tools`), adapter que NÃO persiste diretamente: resolve a competência via `DecideCompetence` e **inicia** o workflow `budget-creation` (`engine.Start`, chave por `resourceId`). Se a resolução retornar clarificação, retorna a pergunta sem iniciar. Input DTO com `Validate()`. Espelha `adjust_allocation.go`/`update_card.go`.

<requirements>
- RF-01: existência da tool fina `create_budget` resolvida por registry, sem regra/SQL/branching de domínio.
- RF-10: idempotência ancorada na identidade do inbound (chave do run por `resourceId` + replay no gate de confirmação + unicidade); NÃO entra em `WithWriteToolSet`.
- RF-25: capacidade só oferecida quando há fluxo que a execute (a tool existe e inicia o workflow).
- RF-28: outcome/estados como tipos fechados.
</requirements>

## Subtarefas

- [ ] 4.1 `CreateBudgetToolInput{MonthRefKind, Year, Month, TotalCents}` + `Validate() error` (`errors.Join`, nomeia campos).
- [ ] 4.2 Mapeamento do payload → `MonthReference` do domínio (kinds fechados).
- [ ] 4.3 `exec`: obtém identidade (`agent.InboundIdentityFromContext`), `DecideCompetence(ref, time.Now().In(loc))`; se clarify → retorna pergunta (sem iniciar); senão `engine.Start(budget-creation, key=resourceID, initial state)` e retorna o prompt do primeiro slot.
- [ ] 4.4 `BuildCreateBudgetTool(engine, def)` com schemas strict.
- [ ] 4.5 Testes testify/suite whitebox (SUT em `s.Run`, dependencies+IIFE, `fake.NewProvider()`): inicia workflow, clarify não inicia, input inválido.

## Detalhes de Implementação

Ver techspec.md → "Interfaces Chave" (assinatura `BuildCreateBudgetTool`) e ADR-001/ADR-002. Molde: `adjust_allocation.go` (identidade + delegação). `now` convertido para America/Sao_Paulo na borda (`time.Now().In(loc)`), passado a `DecideCompetence`. A tool é starter de workflow (como `delete_entry`), portanto fora do write tool set.

## Critérios de Sucesso

- `go build`, `go vet`, `go test -race`, lint verdes.
- Tool não contém regra de negócio, SQL nem branching de domínio; apenas resolve competência e inicia o workflow.
- `Validate()` cobre campos obrigatórios com mensagens nomeadas; outcome como tipo fechado.
- Zero comentários; testes no modelo canônico testify/suite whitebox com mocks do `.mockery.yml`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — Tool fina (`tool.NewTool[I,O]`) como adapter que inicia workflow via registry, sem regra/SQL/branching; contrato de identidade do inbound.

## Testes da Tarefa

- [ ] Testes unitários testify/suite whitebox (inicia workflow, clarify, input inválido) com engine fake e `mocks`.
- [ ] Testes de integração (coberto na tarefa 7.0 via E2E).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/tools/create_budget.go` (novo)
- `internal/agents/application/tools/create_budget_test.go` (novo)
- `internal/agents/application/tools/adjust_allocation.go` (molde — referência)
- `internal/platform/agent/identity_context.go` (identidade — referência)
