# ADR-006 — Stack de testes: stdlib + testify + mockery + testcontainers

## Metadados

- **Título:** Adoção de `testify`, `mockery` e `testcontainers-go` desde a foundation
- **Data:** 2026-05-31
- **Status:** Aceita
- **Decisores:** @JailtonJunior94
- **Relacionados:** [PRD §D-13, §RF-18](./prd.md), [techspec §Estratégia de Testes](./techspec.md), [R-TEST-001](../../.agents/skills/agent-governance/references/testing.md)

## Contexto

A foundation precisa fixar a stack de testes antes do primeiro código de produção entrar. O orchestrator já distribui `.mockery.yml` no baseline e a skill `go-implementation` referencia testify como ferramenta padrão. R-TEST-001 exige: doubles determinísticos, table-driven, separação clara de suites, sem rede real.

## Decisão

Adotar **três bibliotecas** + build tag para separar suites:
- **`stdlib testing`** como motor.
- **`github.com/stretchr/testify`** para `assert`, `require` e `suite`.
- **`github.com/vektra/mockery/v2`** para mocks gerados a partir de interfaces, configurado em `.mockery.yml`.
- **`github.com/testcontainers/testcontainers-go`** para testes de integração com Postgres ephemeral.
- Build tag `//go:build integration` separa unit (`task test:unit`) de integration (`task test:integration`).
- **Sem gate de cobertura no MVP** (D-11): cobertura é reportada como artefato no PR.

## Alternativas Consideradas

1. **stdlib testing + gomock**.
   - Vantagens: gomock é mantido pelo Google; integração com `go generate`.
   - Desvantagens: orchestrator usa mockery; ergonomia inferior; sem `require` equivalente.
2. **stdlib + interface-as-test (sem mock gerado)**.
   - Vantagens: zero dependência; fakes manuais explícitos.
   - Desvantagens: boilerplate explode com >5 interfaces; viola Object Calisthenics #7 (entidades pequenas) — fake manual grande não cabe na regra.
3. **stdlib + testify + go-sqlmock para integração (sem testcontainers)**.
   - Vantagens: mais rápido; sem Docker.
   - Desvantagens: não valida driver/SQL real; viola PRD D-13.

## Consequências

### Benefícios Esperados

- Ergonomia alta (`require.NoError` + `assert.Equal` reduzem ruído).
- Mocks gerados por interface ⇒ Refactor de interface quebra teste em compile-time, não em runtime.
- Testcontainers valida pgx + SQL + migrations contra Postgres real.
- Aderente a R-TEST-001 §Determinismo (sem rede real; testcontainers usa rede local controlada).

### Trade-offs e Custos

- Docker obrigatório em runners de CI (já é padrão no `ubuntu-latest`).
- Suites de integração ~3x mais lentas que unit (~30 s vs 10 s). Aceitável; rodam em job dedicado.
- mockery exige `go generate` ou `task mocks:generate` antes de testar.

### Riscos e Mitigações

- **Risco:** Testcontainers indisponível em fork de contribuidor externo sem Docker.
  - **Mitigação:** `requires.preconditions` no Taskfile valida `docker info` antes de rodar `task test:integration`; falha clara.
- **Risco:** Mocks defasados quando interface muda.
  - **Mitigação:** `task mocks:generate` no `task ci` regenera antes do test; diff falha o PR se mocks desatualizados.
- **Risco:** Uso indevido de `assert` em vez de `require` em setup ⇒ teste continua após falha de setup ⇒ erro confuso.
  - **Mitigação:** convenção em CODEOWNERS / review: `require` em setup, `assert` em verificações finais.

## Plano de Implementação

1. Adicionar dependências ao `go.mod`: `testify`, `mockery`, `testcontainers-go`.
2. Copiar `.mockery.yml` do baseline do orchestrator e adaptar para listar interfaces de `internal/infrastructure/`.
3. `taskfiles/test.yml`: tarefas `test:unit`, `test:integration` (com `//go:build integration`), `mocks:generate`.
4. Primeiro integration test: `internal/infrastructure/database/uow_integration_test.go` validando migration up/down + UoW commit/rollback.

## Monitoramento e Validação

- Métrica de CI: tempo total de `task ci` < 5 min na main; alerta se cruzar 8 min.
- Cobertura reportada em PR como comentário (artefato anexável); sem gate.
- Job de integration test verde como requirement de merge.

## Impacto em Documentação e Operação

- README do projeto: seção "Testes" com `task test:unit` e `task test:integration`.
- Convenção: PRD subsequente que cria interface declara em `.mockery.yml`.

## Revisão Futura

- Revisitar gate de cobertura quando o primeiro módulo de negócio entrar (Identity).
- Revisitar testcontainers se Fly Postgres tier exigir paridade de versão (atualizar imagem).
