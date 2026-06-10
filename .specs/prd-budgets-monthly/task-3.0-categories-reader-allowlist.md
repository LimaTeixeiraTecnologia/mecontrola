# Tarefa 3.0: CategoriesReader cross-module + cache + allowlist de produtores

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar a única dependência cross-module síncrona de budgets: a interface consumer-defined `CategoriesReader` (ADR-001), seu adapter Postgres delegando ao `CategoriesModule`, o cache local com TTL 60s + bust por `editorial_version` (RT-31), e a constante Go da allowlist de produtores autorizados (RT-28). Resolução das 5 raízes oficiais ocorre **uma vez** no boot — falha aborta o startup do módulo.

<requirements>
- Interface declarada em `internal/budgets/application/interfaces/categories_reader.go` (consumer-defined; AGENTS.md).
- Adapter em `internal/budgets/infrastructure/repositories/postgres/categories_reader_adapter.go` consome use cases expostos em `internal/categories` (`ResolveBySlug([]string)` e `ValidateSubcategory(uuid.UUID)`).
- Coordenar com `internal/categories`: adicionar/expor esses use cases via `CategoriesModule` se ainda não existirem. Atualizar a techspec de categories se necessário.
- Cache em `internal/budgets/infrastructure/config/categories_cache.go`: 5 raízes resolvidas 1x no boot e mantidas em memória pelo lifetime; subcategorias com TTL 60s + bust quando `EditorialVersion` muda.
- Constante de allowlist em `internal/budgets/infrastructure/config/producers.go` — slice/map de `producer_source` autorizados; mudança exige PR + deploy (RF-72a, RT-28). Sem runtime mutation, sem leitura de arquivo/env.
- Aceita identificador de subcategoria com `deprecated_at` preenchido (RF-04e).
- Indisponibilidade do reader em runtime → retorna erro tipado para que use cases respondam 503 nas validações novas (RT-18).
- Zero comentários em `.go`.
</requirements>

## Subtarefas

- [ ] 3.1 Declarar a interface `CategoriesReader` em `application/interfaces/categories_reader.go`.
- [ ] 3.2 Expor (e implementar onde faltar) os use cases `ResolveBySlug` e `ValidateSubcategory` no `CategoriesModule`.
- [ ] 3.3 Implementar o adapter `categories_reader_adapter.go` chamando os use cases.
- [ ] 3.4 Implementar o cache em `infrastructure/config/categories_cache.go` (sync.Mutex + TTL; sem cache distribuído — OUT-09).
- [ ] 3.5 Implementar a constante `producers.go` (5–10 `producer_source` esperados; iniciar vazio e crescer por PR).
- [ ] 3.6 Unit tests do adapter com mockery; integration test em `internal/budgets/infrastructure/repositories/postgres/categories_reader_adapter_integration_test.go` com seed das raízes oficiais.

## Detalhes de Implementação

Consultar a seção **Pontos de Integração** e a interface `CategoriesReader` em `techspec.md`. ADR vinculada: [`adr-001-categories-reader-consumer-defined.md`](./adr-001-categories-reader-consumer-defined.md).

`internal/categories` hoje expõe `CategoryRepository.GetByID`. Antes de codar o adapter de budgets, criar (ou confirmar) o use case `ResolveBySlug` em `internal/categories/application/usecases/` e expô-lo no `CategoriesModule`. **Não** adicionar `GetBySlug` no `CategoryRepository` — a operação fica no nível de use case para preservar a fronteira semântica.

## Critérios de Sucesso

- Boot do `BudgetsModule` falha com erro estruturado quando uma das 5 raízes não resolve.
- Cache devolve a mesma instância para repetidas chamadas dentro do TTL; bust por `editorial_version` invalida no primeiro acesso seguinte.
- Allowlist em código rejeita `producer_source` ausente com erro tipado consumível pelo handler/consumer.
- Integration test verifica: (a) resolução das 5 raízes; (b) validação de subcategoria ativa; (c) validação de subcategoria com `deprecated_at` retorna sucesso + flag `deprecated=true`.
- Linter limpo; sem comentários em `.go`.

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

- `internal/budgets/application/interfaces/categories_reader.go` (novo)
- `internal/budgets/infrastructure/repositories/postgres/categories_reader_adapter.go` (novo)
- `internal/budgets/infrastructure/repositories/postgres/categories_reader_adapter_integration_test.go` (novo)
- `internal/budgets/infrastructure/config/categories_cache.go` (novo)
- `internal/budgets/infrastructure/config/producers.go` (novo)
- `internal/categories/application/usecases/resolve_by_slug.go` (novo, ou validar existência)
- `internal/categories/module.go` (alterar para expor os use cases)
- Referência: `internal/categories/application/interfaces/category_repository.go`
