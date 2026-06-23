# Tarefa 5.0: [onboarding] SuggestBudgetSplit + binding

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar, em `internal/onboarding`, o usecase `SuggestBudgetSplit`: resolve o perfil de objetivo
(`ParseObjectiveProfile(hint)` → `classifyByKeyword(objective)` → default `OrganizeSpending`), obtém
os **basis points** do `SplitTemplate` e **delega o cálculo cents** ao binding de `internal/budgets`
(`SuggestAllocation`, Tarefa 4.0) via um seam `BudgetAllocator`. Expor via binding ao agente.

<requirements>
- RF-13: distribuição automática variável por objetivo.
- RF-13a: objetivo ambíguo → default `OrganizeSpending`.
- ADR-004 (resolução híbrida), ADR-006 (matemática cents delegada a budgets; sem reimplementar).
</requirements>

## Subtarefas

- [ ] 5.1 Criar `application/usecases/suggest_budget_split.go` com a resolução híbrida do perfil.
- [ ] 5.2 Definir o seam `BudgetAllocator` (interface no consumidor) e delegar o cálculo cents a `internal/budgets`.
- [ ] 5.3 Expor `SuggestBudgetSplit` via binding ao agente; wiring do seam → binding de budgets em `cmd/server` (declarar; sem implementar fora do escopo do doc).
- [ ] 5.4 Testes unitários (resolução por hint/keyword/default; delegação ao allocator mockado).

## Detalhes de Implementação

Ver techspec.md → "Interfaces Chave" (`SuggestBudgetSplit`, `BudgetAllocator`) e ADR-004/006. O
usecase NÃO multiplica cents — só resolve perfil→basis points e chama o allocator (budgets).

## Critérios de Sucesso

- O perfil resolvido segue a ordem hint → keyword → default.
- O cálculo cents vem exclusivamente do allocator (budgets); o usecase não contém `× incomeCents`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem). go-implementation (linguagem, auto) aplica-se.

## Testes da Tarefa

- [ ] Testes unitários (suite testify; allocator mockado; hint/keyword/default)
- [ ] Testes de integração (E2E em T12 — preview do split no fluxo)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Definition of Done (DoD)

- [ ] Resolução híbrida do perfil implementada e testada.
- [ ] Delegação a `internal/budgets` via seam; zero matemática cents no onboarding.
- [ ] Binding exposto ao agente; zero comentários no `.go` de produção.
- [ ] `go build ./internal/onboarding/...` e `go test ./internal/onboarding/application/usecases/... -run SuggestBudgetSplit` passam.

## Critérios de Aceite (validações executáveis)

```bash
go build ./internal/onboarding/... && \
go test ./internal/onboarding/application/usecases/... -run SuggestBudgetSplit -count=1
# onboarding não reimplementa a matemática cents de distribuição
grep -rnE "incomeCents \* |totalCents \* int64" internal/onboarding/application/usecases/suggest_budget_split.go && echo FAIL || echo OK
```

## Arquivos Relevantes
- `internal/onboarding/application/usecases/suggest_budget_split.go` (novo)
- `internal/onboarding/application/binding/` (seam `BudgetAllocator` + binding)
- depende de: Tarefa 1.0 (perfil/basis points), Tarefa 4.0 (budgets SuggestAllocation)
