# Tarefa 2.0: Consolidar formatação BRL canônica em `money.BRL()`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Tornar `money.BRL()` a fonte única de formatação monetária, migrando os 6 call-sites do módulo
`agents` e removendo os formatadores locais sem separador de milhar. Ver R7 na techspec.

<requirements>
- RF-26: cada valor exibido com 2 casas decimais, milhar com ponto, decimal com vírgula, prefixo "R$ ".
- RF-27: `money.BRL()` é a fonte única; `formatBRL` (onboarding) e `formatAmountBR` (pending) são
  consolidados nele e removidos.
- RF-28: aplica-se a onboarding, confirmação, resumo do mês e mensagens de erro com valores. Alertas
  ficam para `prd-alertas-proativos` (fora deste escopo).
</requirements>

## Subtarefas

- [ ] 2.1 Substituir `formatBRL` por `money.FromCents(cents).BRL()` nos call-sites de
  `onboarding_workflow.go` (`:242`, `:335`, `:508`, `:521`) e remover a função local.
- [ ] 2.2 Substituir `formatAmountBR` por `money.FromCents(cents).BRL()` em `pending_entry_workflow.go`
  (`:639` buildWriteSuccessText, `:721` buildConfirmSummary) e remover a função local.
- [ ] 2.3 Não tocar em `internal/budgets/.../notify_threshold_alert.go` (escopo do prd-alertas-proativos).

## Detalhes de Implementação

Ver techspec.md "Considerações Técnicas → R7 (formatter)". `money.BRL()` já usa `groupThousands`
(`internal/platform/money/money.go:60`) e é coberto por `money_test.go`.

## Critérios de Sucesso

- Zero uso de `formatBRL`/`formatAmountBR` no módulo `agents`; ambas as funções removidas.
- Saídas: 554976→"R$ 5.549,76", 80000000→"R$ 800.000,00", 5000→"R$ 50,00", 5050→"R$ 50,50".
- `go build`/`go vet`/`go test -race` verdes no módulo `agents`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — call-sites vivem em workflows do consumidor `internal/agents` (onboarding/pending).
- `domain-modeling-production` — `Money` como value object e fonte única de formatação.
- `design-patterns-mandatory` — gate `não aplicar padrão` (consolidação em helper existente).

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/money/money.go`, `money_test.go`
- `internal/agents/application/workflows/onboarding_workflow.go`
- `internal/agents/application/workflows/pending_entry_workflow.go`
