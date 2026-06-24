# ADR-001 — Gate de CI de Fronteira de Dados do Agent

## Metadados

- **Título:** Gate automatizado: `internal/agent` só acessa tabela própria e consome outros módulos por porta de entrada
- **Data:** 2026-06-24
- **Status:** Aceita
- **Decisores:** Plataforma / dono do `internal/agent`
- **Relacionados:** PRD `prd-refatoracao-agent-canonico` (RF-18, RF-19, RF-20), `R-AGENT-WF-001.2`, `R-ADAPTER-001.2`, techspec §"Gate de fronteira de dados"

## Contexto

O PRD exige (RF-18) que o `internal/agent` acesse **apenas suas próprias tabelas** e consuma outros
bounded contexts **somente** por porta de entrada. A descoberta confirmou que o código atual **já
satisfaz** essa regra (zero SQL direto a outro BC, zero import de repositório/infra de outro
contexto — tudo via `binding → usecase`). O risco não é o estado atual, é a **regressão silenciosa**:
sem gate automatizado, um futuro PR pode introduzir SQL direto ou import de repo de outro módulo.

## Decisão

Adicionar um gate de CI (`scripts/ci/agent-data-boundary.sh`) que **falha o build** se:

1. Houver SQL direto (`QueryContext`/`ExecContext`/`db.Query`/`tx.Exec`/`db.Exec`) em
   `internal/agent/application/` ou `internal/agent/infrastructure/binding/` (exceto o adapter
   Postgres das tabelas próprias do agent).
2. Houver import de `internal/<outro-bc>/infrastructure/repositories` dentro de `internal/agent/`.

O gate é executado no pipeline e localmente via Taskfile. Tabelas próprias do agent
(`agent_sessions`, `agent_decisions`, `agent_working_memory`, `agent_observations`, `agent_threads`,
`agent_runs`, e `workflow_runs`/`workflow_steps` do kernel) continuam acessíveis pelo adapter Postgres
do próprio agent/kernel.

## Alternativas Consideradas

- **Convenção + code review** (status quo): depende de disciplina humana; não é "production-proof" nem
  "0 falso positivo". Rejeitada — o PRD pede blindagem verificável.
- **Lint customizado (analyzer Go)**: mais robusto que grep, porém custo de implementação/manutenção
  maior para o mesmo resultado no MVP. Adiável; o grep cobre os padrões reais hoje.

## Consequências

### Benefícios Esperados

- Fronteira de bounded context inviolável por construção; regressão vira falha de build.
- Reforça `R-AGENT-WF-001.2`/`R-ADAPTER-001.2` de forma executável.

### Trade-offs e Custos

- Grep pode gerar falso positivo se um nome de variável coincidir; mitigado por escopo de path e
  exclusões (`mocks`, `_test.go`, adapter postgres do agent).

### Riscos e Mitigações

- **Risco:** gate quebrar build por match espúrio. **Mitigação:** padrões ancorados em chamadas reais;
  exceção explícita do adapter Postgres do agent. **Rollback:** desabilitar o passo no pipeline.

## Plano de Implementação

1. Criar `scripts/ci/agent-data-boundary.sh`. 2. Adicionar receita no Taskfile + passo no CI.
3. Rodar no estado atual (deve passar — verde). 4. Documentar no runbook do agent.

## Monitoramento e Validação

- Sucesso: gate verde no estado atual e vermelho em PR de teste que injete SQL direto no agent.

## Impacto em Documentação e Operação

- Atualizar runbook do agent e `AGENTS.md`/regra `go-adapters.md` com o gate.

## Revisão Futura

- Reavaliar troca por analyzer Go se o grep gerar ruído recorrente.
