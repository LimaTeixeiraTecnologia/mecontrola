# Tarefa 1.0: Domínio de mês — MonthReference, DecideCompetence, Prev e FormatCompetencePtBR

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar, em `internal/budgets`, os primitivos determinísticos e puros de competência: o tipo fechado `MonthReference` (união discriminada), a função pura `DecideCompetence(ref, now)`, o método `Competence.Prev()` (simétrico a `Next()`) e a formatação por extenso `FormatCompetencePtBR`. Sem IO, sem relógio interno (`now` injetado), sem LLM. Nenhuma alteração de armazenamento (`YYYY-MM` permanece).

<requirements>
- RF-13: resolução relativa determinística (mês atual = corrente; mês passado = corrente-1; mês que vem = corrente+1) em America/Sao_Paulo, com `now` injetado.
- RF-14: mês nomeado com ano resolve para `YYYY-MM` exato.
- RF-15: mês nomeado sem ano → `ClarifyMissingYear` (não assume).
- RF-16: sem mês/referência reconhecível → `ClarifyUnrecognized`.
- RF-18 (função): `FormatCompetencePtBR(c) → "<mês> de <ano>"` pt-BR; armazenamento ISO inalterado.
- RF-19: idioma pt-BR fixo.
</requirements>

## Subtarefas

- [ ] 1.1 `MonthReference` (`MonthRefKind` fechado: Current, Previous, Next, Explicit, NamedWithoutYear, Unknown) + `ClarifyReason` fechado (None, MissingYear, Unrecognized), com `String()`/`IsValid()`/`Parse*`.
- [ ] 1.2 `DecideCompetence(ref MonthReference, now time.Time) (Competence, ClarifyReason, error)` puro, cobrindo todos os kinds; Explicit inválido → erro.
- [ ] 1.3 `Competence.Prev() Competence` (via `AddDate(0,-1,0)` sobre `CompetenceFromTime`), simétrico a `Next()`.
- [ ] 1.4 `FormatCompetencePtBR(c Competence) string` (tabela fixa de 12 meses).
- [ ] 1.5 Testes puros de tabela (viradas de ano dez↔jan; "junho de 2026", "janeiro de 2025"; sem ano → clarify).

## Detalhes de Implementação

Ver techspec.md → "Design de Implementação > Interfaces Chave" (assinatura de `DecideCompetence`) e ADR-002/ADR-003. `now` chega já convertido para America/Sao_Paulo na borda (tool exec) — proibido `time.Now()` interno (feedback "proibido abstrair tempo"). Reuso de `NewCompetence`/`CompetenceFromTime`/`Next` existentes em `internal/budgets/domain/valueobjects/competence.go`.

## Critérios de Sucesso

- `go build`, `go vet`, `go test -race`, lint verdes no módulo `internal/budgets`.
- Cobertura de tabela para todos os `MonthRefKind` e viradas de ano; nenhum uso de `time.Now()` dentro das funções puras.
- Zero comentários em `.go` de produção; sem prefixo `_` em identificadores.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `domain-modeling-production` — modelagem de tipo fechado (união discriminada `MonthReference`) e `Decide*` puro no domínio de competência.
- `design-patterns-mandatory` — gate de desenho da união discriminada (state-as-type) vs alternativas; justificar não usar Strategy para formatação.

## Testes da Tarefa

- [ ] Testes unitários (tabela pura, sem mock) para `DecideCompetence`, `Prev`, `FormatCompetencePtBR`.
- [ ] Testes de integração (não aplicável nesta tarefa — domínio puro).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/budgets/domain/valueobjects/month_reference.go` (novo)
- `internal/budgets/domain/valueobjects/competence.go` (modificado: `Prev`, `FormatCompetencePtBR`)
- `internal/budgets/domain/valueobjects/*_test.go` (novos testes)
