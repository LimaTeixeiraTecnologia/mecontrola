# Tarefa 1.0: Fechar dívida R5.26 (renome de identificadores `_`-prefixados)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Renomear todos os identificadores de produção prefixados com `_` em
`internal/agents/application/workflows/` para a forma idiomática camelCase, fechando a violação hard
R5.26 sem alterar comportamento.

<requirements>
- RF-44: renomear `_defaultDistributionBP`, `_welcomeGoalPrompt`, `_goalReprompt`, `_goalValueReprompt`,
  `_incomePrompt`, `_incomeReprompt`, `_cardsReprompt`, `_summaryReprompt`, `_conclusionRecurrencePrompt`,
  `_allocationInputSystemPrompt`, `_goalWithValueSystemPrompt`, `_goalValueSystemPrompt` em
  `onboarding_workflow.go` e seus usos em `budget_creation_workflow.go` para camelCase sem `_`, sem
  mudança de comportamento.
</requirements>

## Subtarefas

- [ ] 1.1 Renomear cada identificador `_`-prefixado em `onboarding_workflow.go` (const/var) para camelCase.
- [ ] 1.2 Atualizar os usos em `budget_creation_workflow.go` (`_defaultDistributionBP`,
  `_allocationInputSystemPrompt`) e quaisquer outros arquivos que os referenciem.
- [ ] 1.3 Rodar o gate R5.26 (grep) e confirmar retorno vazio no escopo `internal/agents/`.

## Detalhes de Implementação

Ver `techspec.md` → "Arquivos Relevantes e Dependentes" e a regra R5.26 em `go-implementation`
(catálogo Uber 5.26 revogado quanto ao prefixo `_` pela decisão do projeto 2026-06-04: prefixo `_` é
totalmente proibido). Renome puro; nenhuma lógica muda. Blank identifier `_` isolado permanece.

## Critérios de Sucesso

- Nenhum identificador de produção prefixado com `_` em `internal/agents/` (exceto blank identifier).
- `go build ./...` e `go vet ./...` verdes; comportamento inalterado (testes existentes passam).
- Gate: `grep -rEn "\b(const|var)\s+_[a-zA-Z]" internal/agents/ --include="*.go" | grep -v _test.go`
  retorna vazio.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: suíte existente de onboarding/budget-creation permanece verde (não-regressão).
- [ ] Testes de integração: não aplicável.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go`
- `internal/agents/application/workflows/budget_creation_workflow.go`
