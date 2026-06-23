# Tarefa 4.0: [budgets] SuggestAllocation + binding

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Expor, em `internal/budgets`, um usecase `SuggestAllocation` que encapsula o domain service
`AllocationDistributor` (`Distribute(totalCents, []AllocationInput{RootSlug, BasisPoints}) →
[]AllocationResult{PlannedCents}`), tornando o cálculo cents disponível por binding sem que outros
módulos toquem a persistência de budgets. É a única fonte da matemática de distribuição (ADR-006).

<requirements>
- RF-13: distribuição automática (cálculo cents) — dono é `internal/budgets`.
- ADR-006: bounded context budgets é dono da alocação; reuso de `AllocationDistributor`.
- Sem persistência: usecase puro de cálculo (não cria budget; materialização segue via evento).
</requirements>

## Subtarefas

- [ ] 4.1 Criar `application/usecases/suggest_allocation.go` (Input/Result + Execute) encapsulando `AllocationDistributor`.
- [ ] 4.2 Expor o usecase no `module.go` e via binding consumível por outros módulos.
- [ ] 4.3 Testes unitários (soma = totalCents; arredondamento half-even; bp inválido).

## Detalhes de Implementação

Ver techspec.md → "Interfaces Chave" (`internal/budgets`) e ADR-006. Reusar `AllocationDistributor`
existente; não duplicar a matemática. Sem IO de banco.

## Critérios de Sucesso

- `SuggestAllocation` retorna soma de `PlannedCents` exatamente igual a `TotalCents`.
- Não cria nem persiste budget; apenas calcula (a criação real continua via `onboarding.splits_calculated`).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem). go-implementation (linguagem, auto) aplica-se.

## Testes da Tarefa

- [ ] Testes unitários (distribuição soma totalCents; arredondamento; bp zero/negativo)
- [ ] Testes de integração (não aplicável — usecase puro de cálculo)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Definition of Done (DoD)

- [ ] `SuggestAllocation` exposto via binding; reusa `AllocationDistributor`.
- [ ] Sem acesso a repositório/uow no usecase de sugestão.
- [ ] Zero comentários no `.go` de produção.
- [ ] `go build ./internal/budgets/...` e `go test ./internal/budgets/application/usecases/... -run SuggestAllocation` passam.

## Critérios de Aceite (validações executáveis)

```bash
go build ./internal/budgets/... && \
go test ./internal/budgets/... -run SuggestAllocation -count=1
```

## Arquivos Relevantes
- `internal/budgets/application/usecases/suggest_allocation.go` (novo)
- `internal/budgets/module.go` (expor usecase/binding)
- `internal/budgets/domain/services/allocation_distributor.go` (reuso, leitura)
