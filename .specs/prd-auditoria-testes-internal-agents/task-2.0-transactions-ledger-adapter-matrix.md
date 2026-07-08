# Tarefa 2.0: Expandir a matriz de testes do `transactions_ledger_adapter`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Cobrir integralmente as 9 operações públicas de `TransactionsLedger` no adapter real, com foco em identidade inbound, forwarding de payload e edge cases de transformação. Quando houver mocks, o uso de `.mockery.yaml` do repositório é obrigatório, e a suíte deve seguir `testify/suite` com cenários table-driven no padrão aprovado.

<requirements>
- Cobrir RF-04, RF-05 e RF-12.
- Manter `transactions_ledger_adapter_test.go` como fonte principal da matriz offline do adapter.
- Para cada operação pública, cobrir ao menos `success`, `downstream error` e `principal ausente ou inválido`.
- Validar os edge cases específicos: `SubcategoryID=nil`, `kind` inválido, item inesperado, forwarding de `Version`, `CardID`, `Frequency`, `DayOfMonth` e `Reconciled`.
- Provar que o downstream não é chamado quando a identidade inbound falha.
- Usar `.mockery.yaml` quando a tarefa depender de mocks do módulo `transactions`.
</requirements>

## Subtarefas

- [ ] 2.1 Expandir a suíte existente para cobrir as 9 operações da interface `TransactionsLedger`.
- [ ] 2.2 Adicionar matriz transversal de `principalCtx` para identidade já presente, identidade inbound válida, ausência de identidade e UUID inválido.
- [ ] 2.3 Garantir asserts explícitos de mapping e wrapping de erro.

## Detalhes de Implementação

Consultar `techspec.md`, especialmente:
- `## Interfaces Chave` para a interface `TransactionsLedger`.
- `## Modelos de Dados` para a lista de edge cases obrigatórios por método.
- `## Abordagem de Testes` para a regra de mocks reais do módulo `transactions` e payload exato.

## Critérios de Sucesso

- As 9 operações públicas do adapter possuem prova offline explícita.
- A suíte falha se `principalCtx` aceitar identidade inválida ou se algum downstream for chamado sem identidade válida.
- O padrão de teste segue `testify/suite` + cenários table-driven no estilo aprovado.
- Mocks, quando necessários, respeitam `.mockery.yaml`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] `go test -race -count=1 ./internal/agents/infrastructure/binding/...`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/infrastructure/binding/transactions_ledger_adapter.go`
- `internal/agents/infrastructure/binding/transactions_ledger_adapter_test.go`
- `internal/agents/application/interfaces/transactions_ledger.go`
- `internal/agents/application/tools/read_tools_test.go`
