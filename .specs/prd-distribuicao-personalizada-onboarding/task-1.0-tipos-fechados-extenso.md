# Tarefa 1.0: Tipos fechados de estado e extração por extenso

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Introduzir as bases de tipos fechados desta feature e habilitar a leitura de valores por extenso no passo de distribuição, sem alterar comportamento de fluxo ainda. É a fundação de compilação para as tarefas 2.0 e 3.0.

<requirements>
- RF-08: aceitar valores por extenso, monetário e percentual no passo de distribuição (habilitação via prompt compartilhado).
- RF-14: estados de espera do passo como tipos fechados enumerados (state-as-type).
</requirements>

## Subtarefas

- [x] 1.1 Adicionar a constante `reviewAwaitPersonalize` ao enum `reviewAwaitKind` (`iota + 1`, zero-value inválido) e atualizar `String()` e `IsValid()` em `internal/agents/application/workflows/onboarding_workflow.go`.
- [x] 1.2 Criar o tipo fechado `distributionIntentKind` (`distributionIntentAccept | distributionIntentPersonalize | distributionIntentValues`) com `String()`/`IsValid()` e parser, sentinel error tipado.
- [x] 1.3 Criar o tipo fechado `distributionBalanceKind` (`distributionBalanced | distributionOver | distributionUnder`) com `String()`/`IsValid()`.
- [x] 1.4 Enriquecer `allocationInputSystemPrompt` com exemplos de conversão por extenso (ex.: "mil reais" → 1000, "quinhentos" → 500), alinhado ao padrão de `goalWithValueSystemPrompt`/`goalValueSystemPrompt` — beneficia onboarding e budget_creation (compartilhado).

## Detalhes de Implementação

Ver `techspec.md` seções "Interfaces Chave" (tipos fechados) e "Modelos de Dados"; ADR-002 (sub-estado e classificação). Não usar prefixo `_` em identificador (R5.26); enums começam em `iota + 1` (R5.8). Zero comentários em Go de produção (R-ADAPTER-001.1).

## Critérios de Sucesso

- Os três tipos fechados compilam com `String()`/`IsValid()` e zero-value inválido; nenhum estado representado como string livre (RF-14 / R-AGENT-WF-001.3).
- `allocationInputSystemPrompt` contém exemplos por extenso sem remover nenhum exemplo existente (RF-08); nenhuma mudança de assinatura pública.
- `go build ./...` e `go vet ./...` verdes; nenhum comportamento de fluxo alterado ainda.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `domain-modeling-production` — modelar os estados de espera como tipos fechados (state-as-type) com zero-value inválido.
- `mastra` — os tipos vivem no consumidor `internal/agents` sobre o substrato; estados fechados exigidos por R-AGENT-WF-001.3.

## Testes da Tarefa

- [x] Testes unitários: `String()`/`IsValid()`/parser dos três tipos fechados (valores válidos e inválidos), package whitebox testify/suite.
- [x] Testes de integração: não aplicável nesta tarefa.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go` (tipos + prompt compartilhado)
- `internal/agents/application/workflows/onboarding_workflow_test.go` (testes dos tipos)
