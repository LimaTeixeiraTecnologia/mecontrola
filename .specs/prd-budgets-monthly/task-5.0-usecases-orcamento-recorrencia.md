# Tarefa 5.0: Use cases de orçamento + auto-draft + recorrência

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar os use cases de planejamento mensal: `CreateBudget`, `ActivateBudget`, `DeleteDraftBudget`, `CreateRecurrence` e o auxiliar interno `CreateOrAutoDraftForExpense` (usado por `UpsertExpense` na tarefa 6.0). Todos consomem `uow.UnitOfWork[T]` do devkit-go diretamente (padrão `internal/billing/application/usecases/`) — sem abstração `TxRunner` local.

<requirements>
- Cada use case com mutação injeta `uow.UnitOfWork[T]` no construtor e executa via `uow.Do(ctx, fn)`.
- `CreateBudget` impõe RF-01..RF-08, RF-09 (consultáveis após criação).
- `ActivateBudget` impõe RF-07/RF-07a/RF-07b (soma = 10000bp; total > 0); usa `AllocationDistributor` para calcular `planned_cents` por raiz; rejeita ativo (RF-08).
- `DeleteDraftBudget` aceita rascunho manual ou automático (RF-09b); rejeita ativo com 409 (RF-09c); operação não afeta despesas da competência (RF-09d cobre recriação subsequente).
- `CreateRecurrence` valida `source_competence` (RF-23a: ativado OU rascunho com 100%; rejeita expurgada/sem total/rascunho automático sem alocações); aplica até 12 meses (RF-19); por competência retorna `created|updated|completed_from_draft|conflict|failure` (RF-21a/b); nunca sobrescreve ativado (RF-24).
- `CreateOrAutoDraftForExpense` é auxiliar interno (não exposto): cria rascunho automático sem valor/alocações exclusivamente quando primeira despesa válida da competência commita (RF-12/RF-12a/RF-12b).
- Mockery para todas as interfaces consumidas; unit tests cobrem cada cenário.
- Zero comentários em `.go` de produção.
</requirements>

## Subtarefas

- [ ] 5.1 `application/dtos/input/{create_budget,activate_budget,delete_draft,create_recurrence}_input.go`.
- [ ] 5.2 `application/dtos/output/{budget,recurrence_result}_output.go`.
- [ ] 5.3 `application/usecases/create_budget.go` + mockery + unit test.
- [ ] 5.4 `application/usecases/activate_budget.go` (chama `AllocationDistributor`) + unit test cobrindo distribuição de centavos residuais.
- [ ] 5.5 `application/usecases/delete_draft_budget.go` + unit test (rejeita ativo).
- [ ] 5.6 `application/usecases/create_recurrence.go` + unit test cobrindo RF-21a/b (multi-status) e RF-23a (rejeições).
- [ ] 5.7 `application/usecases/create_or_auto_draft_for_expense.go` (interno) + unit test cobrindo RF-12a (só commita junto da primeira despesa).
- [ ] 5.8 Atualizar `mockery.yml` se preciso e regenerar mocks.

## Detalhes de Implementação

Ver seção **Interfaces Chave** (exemplo de `uow.UnitOfWork[T]`) e **Design de Implementação** na `techspec.md`. Padrão de DI espelha `internal/billing/application/usecases/process_sale_approved.go` (construtor explícito, struct com `uow` + observability + repositórios + `CategoriesReader`).

Para a competência BR (RT-17/RT-27): receber `time.Location` como dependência do construtor (resolvida no `module.go` no boot). Não chamar `time.LoadLocation` dentro do use case.

## Critérios de Sucesso

- Cobertura unitária ≥ 85% nos use cases.
- `go test -race -count=1 ./internal/budgets/application/usecases/...` verde.
- Linter limpo.
- Nenhuma chamada SQL direta (R-ADAPTER-001.2 não se aplica aqui mas o spirit vale: use cases consomem repositórios).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/budgets/application/dtos/input/*.go` (novo, parcial)
- `internal/budgets/application/dtos/output/*.go` (novo, parcial)
- `internal/budgets/application/usecases/{create_budget,activate_budget,delete_draft_budget,create_recurrence,create_or_auto_draft_for_expense}.go` (novo)
- `internal/budgets/application/usecases/mocks/` (gerados)
- `mockery.yml` (atualizar)
- Referência: `internal/billing/application/usecases/process_sale_approved.go` (padrão UoW)
