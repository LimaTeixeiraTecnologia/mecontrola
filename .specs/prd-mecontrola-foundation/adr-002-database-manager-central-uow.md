# ADR-002 — `manager.Manager` central + `UnitOfWork[T]` genérico

## Metadados

- **Título:** Composição central do `pkg/database` do devkit-go com UoW tipado por agregado
- **Data:** 2026-05-31
- **Status:** Aceita
- **Decisores:** @JailtonJunior94
- **Relacionados:** [PRD §RF-10](./prd.md), [techspec §Design de Implementação](./techspec.md), [devkit-go v0.4.0 `pkg/database`](https://github.com/JailtonJunior94/devkit-go)

## Contexto

`devkit-go` v0.4.0 entrega `manager.Manager` (wrapper de `pgxpool.Pool` com observabilidade) e `UnitOfWork[T]` (transação tipada por valor de retorno) prontos para uso. O discovery prevê 1–10k usuários ativos com pool de 30 conexões — pool único compartilhado entre módulos é compatível, pool por módulo estouraria o budget de conexões do Fly Postgres dev tier (~50 conexões totais).

A foundation precisa decidir como expor essa infraestrutura aos 6 módulos de domínio futuros sem violar R-DDD-001 (Domain não conhece infra) e sem inviabilizar testes (módulo precisa poder mockar a fronteira).

## Decisão

**`manager.Manager` é instanciado uma única vez em `internal/infrastructure/database`** e injetado em cada módulo via construtor. Cada módulo declara sua **port `Repository`** em `application/` e implementa em `adapters/` consumindo `Manager.Pool()` + `UnitOfWork[T]` tipado pelo agregado (`UnitOfWork[*User]`, `UnitOfWork[*Conversation]`, ...). A transacionalidade é responsabilidade do `application`, nunca do `domain` ou do `adapters`.

## Alternativas Consideradas

1. **Pool dedicado por módulo (`identityPool`, `financePool`, ...)**.
   - Vantagens: isolamento total; falha de tx em um módulo não afeta outro.
   - Desvantagens: estoura budget de 30 conexões (6 módulos × 5 conexões mínimas = já no limite); devkit-go não foi desenhado para múltiplos managers; sobrecarga de configuração.
2. **Repository facade sem UoW explícito, com transação por decorator**.
   - Vantagens: API mais simples no consumer.
   - Desvantagens: esconde transacionalidade do `application` (viola R-DDD-001 §Application Layer: "use cases devem orquestrar"); dificulta debug de tx longa.
3. **Sem UoW: cada adapter abre transação local**.
   - Vantagens: simples para CRUD trivial.
   - Desvantagens: impossível compor múltiplas writes atômicas (e.g. movimentação + atualização de saldo no PRD de Finance); regrida para "transação por linha".

## Consequências

### Benefícios Esperados

- Conformidade total com R-DDD-001 (Application orquestra, Domain puro, Adapter implementa port).
- Aproveita 100% do devkit-go sem reinventar wrapper.
- Pool único respeita budget de 30 conexões do Fly Postgres dev tier.
- UoW tipado pega erro de tipo em compile-time (Go 1.26 generics).
- Eventos podem ser publicados pós-commit (ver ADR-003) de forma atômica.

### Trade-offs e Custos

- Acoplamento (controlado) ao devkit-go: troca de driver exige refactor — aceitável (devkit-go é da mesma org, manutenção própria).
- UoW genérico exige Go 1.26+ (já fixado no PRD).

### Riscos e Mitigações

- **Risco:** Esgotamento do pool em testes paralelos.
  - **Mitigação:** testcontainers com pool de 5 conexões por suite; `t.Parallel()` controlado; CI roda integration tests em job dedicado, não em paralelo com unit.
- **Risco:** Long-running tx travando outras requests.
  - **Mitigação:** `context.WithTimeout` obrigatório em todo `UoW.Do`; default 5 s; sentinel `ErrDeadlineExceeded` mapeado para 504.

## Plano de Implementação

1. `internal/infrastructure/database/manager.go`: factory `NewManager(cfg Config) (*Manager, error)`.
2. `internal/infrastructure/database/uow.go`: helper genérico `UnitOfWork[T any]` wrapping o do devkit-go (re-export tipado).
3. `internal/infrastructure/database/errors.go`: sentinels `ErrConnection`, `ErrMigration`, `ErrDeadlineExceeded`.
4. Integration test em `internal/infrastructure/database/uow_integration_test.go` cobrindo commit + rollback + deadline.

## Monitoramento e Validação

- Métrica `database.tx.duration_ms` (devkit-go) com p95 < 200 ms em /ready.
- Métrica `database.tx.rolledback` < 1% em condição normal.
- Alerta: pool exhaustion (>= 90% de uso por 5 min).

## Impacto em Documentação e Operação

- Cada PRD subsequente que crie agregado deve declarar `UnitOfWork[*<Aggregate>]` em `application/`.
- Runbook "Pool exhaustion" a criar quando primeiro módulo de negócio entrar.

## Revisão Futura

- Revisitar quando volumetria cruzar 10k usuários ativos (gargalo previsto pelo discovery).
- Possível adoção de read-replica via segundo `Manager` somente leitura.
