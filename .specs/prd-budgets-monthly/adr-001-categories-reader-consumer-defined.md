# ADR-001 — Integração com `internal/categories` via interface consumer-defined

## Metadados

- **Título:** CategoriesReader como interface consumer-defined em `internal/budgets`
- **Data:** 2026-06-09
- **Status:** Aceita
- **Decisores:** Time MeControla / AI Agent
- **Relacionados:** [PRD v24](./prd.md) (RT-23, RT-31, RF-04a/b/d/e), [techspec.md](./techspec.md), `AGENTS.md` (regra "interface declarada pelo consumidor"), `internal/categories` (`prd-categories-crud`)

## Contexto

- O módulo `internal/budgets` consome `internal/categories` em runtime para: (a) resolver os IDs das 5 raízes oficiais a partir de slugs imutáveis no boot (RT-31); (b) validar cada `subcategory_id` informado em despesas/alocações (RF-04d) incluindo subcategorias com `deprecated_at` (RF-04e); (c) expor a `editorial_version` corrente para invalidar o cache local.
- O repositório de categories hoje expõe `CategoryRepository.GetByID(ctx, uuid)`. Não há `GetBySlug` nem caso de uso correspondente.
- `AGENTS.md` impõe que comunicação cross-module use **interface declarada pelo consumidor** ou domain event/outbox.
- Budgets precisa falhar startup se as 5 raízes não resolverem (RT-31), preservando determinismo e detectando regressões editoriais em deploy.

## Decisão

Budgets declara em `internal/budgets/application/interfaces/categories_reader.go` a interface consumer-defined:

```go
type CategoriesReader interface {
    ResolveRootsBySlug(ctx context.Context, slugs []string) (map[string]uuid.UUID, error)
    ValidateExpenseSubcategory(ctx context.Context, id uuid.UUID) (rootSlug string, deprecated bool, err error)
    EditorialVersion(ctx context.Context) (int64, error)
}
```

A implementação vive em `internal/budgets/infrastructure/repositories/postgres/categories_reader_adapter.go` e delega a use cases expostos pelo `CategoriesModule` (que serão criados/expandidos em coordenação com a techspec de categories: `ResolveBySlug` para raízes; `ValidateSubcategory` se ainda não existir). Categories implementa esses use cases internamente sobre seu próprio `CategoryRepository`.

Cache local:

- 5 raízes resolvidas uma vez no boot do `BudgetsModule` (`internal/budgets/infrastructure/config/categories_cache.go`); falha aborta o startup.
- Subcategorias cacheadas com TTL máximo de 60 segundos, com bust explícito quando `EditorialVersion` muda.

## Alternativas Consideradas

1. **Estender `CategoryRepository` em categories e budgets importa o repositório**.
   - Vantagens: menor número de tipos a manter.
   - Desvantagens: budgets passa a depender de detalhes internos de persistência de categories; mudanças em paginação/leitura quebram budgets; viola "interface no consumidor".
   - Rejeitada por acoplar camadas de infraestrutura cross-module.

2. **Budgets consulta tabela `categories` diretamente via SQL**.
   - Vantagens: zero hop de chamada; potencial ganho de latência.
   - Desvantagens: duplica conhecimento de schema; viola RT-23 (contrato exposto por categories); incompatível com evolução editorial e `editorial_version`.
   - Rejeitada por quebrar fronteira de bounded context.

## Consequências

### Benefícios Esperados

- Budgets evolui sem conhecer detalhes internos de categories.
- Testes de use cases em budgets ficam com mocks gerados por mockery sobre a interface local.
- Possível troca futura do mecanismo de leitura (ex.: gRPC, gateway) sem alterar use cases de budgets.

### Trade-offs e Custos

- Necessidade de criar um use case em categories (`ResolveBySlug`) caso ainda não exista — coordenação cross-module.
- Pequeno overhead de uma camada extra de adaptador.

### Riscos e Mitigações

- **Risco:** desalinhamento entre o contrato declarado por budgets e a implementação de categories.
  - **Mitigação:** integration test em budgets exercitando o adapter contra Postgres real com seed das raízes.
- **Risco:** evolução de `editorial_version` invalidar cache em momento inconveniente.
  - **Mitigação:** TTL curto (60s) limita janela de inconsistência; raízes permanecem estáveis por contrato editorial imutável.

## Plano de Implementação

1. Coordenar com `internal/categories` para garantir/expor `ResolveBySlug([]string)` e `ValidateSubcategory(uuid)` no `CategoriesModule`.
2. Implementar `CategoriesReader` adapter em `internal/budgets/infrastructure/repositories/postgres/`.
3. Implementar cache local em `infrastructure/config/categories_cache.go`.
4. Boot resolve raízes — falha ao resolver aborta `NewBudgetsModule`.
5. Integration test do adapter com migrations + seed editorial.

## Monitoramento e Validação

- Métrica `budgets_categories_reader_calls_total{op,outcome}` (cardinalidade controlada).
- Métrica `budgets_categories_cache_hit_ratio`.
- Log `ERROR` ao falhar `ResolveRootsBySlug` no boot — bloqueia deploy.

## Impacto em Documentação e Operação

- README de `internal/budgets` referencia esta ADR.
- Runbook: degradação de categories → 503 em validações novas; consultas continuam.

## Revisão Futura

- Revisar se um quinto método (`InvalidateCache`) for necessário em razão de incidentes.
- Revisar se categories adotar um SDK próprio para consumo cross-module.
