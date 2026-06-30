# Tarefa 1.0: Substrato `WithMaxToolRounds` + exposição de use cases no TransactionsModule

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Preparar o terreno para o agente: (a) estender o primitivo `internal/platform/agent` com uma `AgentOption` `WithMaxToolRounds(n)` (default atual 5 preservado para outros consumidores; o `MeControlaAgent` usará 12); (b) expor publicamente no `TransactionsModule` os use cases já construídos mas não acessíveis (`UpdateCardPurchase`, `DeleteCardPurchase`, `GetMonthlySummary`, `ListMonthlyEntries`) para desbloquear os bindings da tarefa 2.0.

<requirements>
- ADR-007: `maxToolRounds` configurável (default 5 inalterado; agente usa 12) — `internal/platform/agent/agent.go:15`.
- ADR-003: expor campos públicos no `TransactionsModule` (`internal/transactions/module.go:165-248` constrói `updateCP`, `deleteCP`, `getMS`, `listME` sem expor).
- Cobre (enabling): RF-21.2, RF-24, RF-25, RF-26, RF-38.
</requirements>

## Subtarefas

- [ ] 1.1 Adicionar `WithMaxToolRounds(n int) AgentOption` em `internal/platform/agent`; substituir o uso da const `maxToolRounds` por campo do `agentImpl` com default 5 quando a option não for passada.
- [ ] 1.2 Testes do primitivo: default 5 inalterado; com option, respeita `n`; `ErrMaxToolRounds` ainda dispara ao exceder.
- [ ] 1.3 Expor `UpdateCardPurchaseUC`, `DeleteCardPurchaseUC`, `GetMonthlySummaryUC`, `ListMonthlyEntriesUC` como campos públicos do `TransactionsModule` (apenas atribuição; UCs já existem).
- [ ] 1.4 Garantir que o wiring atual continua compilando (nenhum consumidor existente quebra).

## Detalhes de Implementação

Ver techspec.md → "Sequenciamento de Desenvolvimento" passo 0 e 1; ADR-007 (tool rounds) e ADR-003 (exposição). Não alterar comportamento dos use cases de transactions — apenas tornar acessíveis.

## Critérios de Sucesso

- `WithMaxToolRounds` segue go-implementation R0–R7 (sem `init`, sem `panic`, sem abstrair tempo) e DMMF (sem `Result[T,E]`/currying/DSL); zero comentários em `.go` de produção (R-ADAPTER-001.1).
- Default 5 preservado para o weather e demais consumidores (sem regressão).
- `TransactionsModule` expõe os 4 UCs; `go build ./...` e `gofmt` verdes.
- Mudança mínima e genérica no primitivo (não vaza domínio para o kernel/substrato).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — altera o ciclo de execução do agente (`maxToolRounds`) e prepara o consumo dos use cases pelo substrato; base canônica do agente sobre `internal/platform`.

## Testes da Tarefa

- [ ] Testes unitários (testify/suite whitebox, `fake.NewProvider`, IIFE por mock — R-TESTING-001): default vs option; `ErrMaxToolRounds`.
- [ ] Testes de integração: build do projeto inteiro verde após exposição dos UCs.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/agent/agent.go` (`maxToolRounds`, `AgentOption`)
- `internal/transactions/module.go:165-248` (exposição de UCs)
- techspec.md (passo 0/1), ADR-007, ADR-003
