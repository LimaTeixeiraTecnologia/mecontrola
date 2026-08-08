# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Contagem diária via platform_runs com porta de leitura declarada pelo consumidor
- **Data:** 2026-08-07
- **Status:** Aceita
- **Decisores:** responsável pelo produto mecontrola, agente de engenharia
- **Relacionados:** `.specs/prd-limite-interacoes-diarias-assistente/prd.md`, `.specs/prd-limite-interacoes-diarias-assistente/techspec.md`, `adr-001-gate-como-resolvedor-no-consumer.md`

## Contexto

O gate de limite diário precisa saber quantas interações o usuário já consumiu no dia corrente. O PRD proíbe tabela ou migration nova (RF-02) e exige que a fonte seja `mecontrola.platform_runs`, que já registra um Run por dispatch com `resource_id` e `started_at` (`migrations/000001_initial_schema.up.sql:2367-2388`) e já possui o índice `platform_runs_resource_started_idx (resource_id, started_at DESC)` (`:2390-2391`). O port `agent.RunStore` do substrato expõe apenas `Insert`, `Update` e `Load` (`internal/platform/agent/ports.go:153-157`), sem operação de contagem. A governança do repositório exige interface declarada pelo consumidor (R6.3), tipos concretos por padrão (R6.2) e proíbe vazar necessidade de um consumidor para o substrato compartilhado.

## Decisão

A contagem é feita por um contrato privado `dailyInteractionCounter` declarado no pacote `usecases` de `internal/agents` (padrão de `WriteLedgerRepository` em `usecases/write_ledger.go:24-28`), com implementação `NewDailyInteractionCounter(db database.DBTX, o11y)` em `internal/agents/infrastructure/repositories/postgres`, no padrão de `audit_reconciliation.go:19-21`: `database/sql` puro sobre `database.DBTX`, placeholders `$N`, schema `mecontrola.`, erros com wrap `%w`. A query é `SELECT COUNT(*) FROM mecontrola.platform_runs WHERE resource_id = $1 AND started_at >= $2`, coberta pelo índice existente. O substrato `internal/platform/agent` não é alterado.

## Alternativas Consideradas

- Tabela de quota com contador incremental (`user_id`, `day`, `count`): vantagem de leitura O(1) e de independência da semântica de Runs; desvantagens de migration nova, estado paralelo a sincronizar com falhas de Run, necessidade de incremento transacional e de limpeza de linhas antigas. Rejeitada pelo PRD (RF-02) e pelo custo de complexidade frente a uma query indexada suficiente.
- Estender `agent.RunStore` com `CountStartedSince`: vantagem de reuso do repositório existente em `internal/platform/agent/infrastructure/postgres/run_store.go`; desvantagens de alterar o substrato por necessidade de um único consumidor e de esbarrar no drift confirmado dos mocks de `platform/agent`, gerados por mockery mas fora do `.mockery.yml`, o que exigiria correção adicional de tooling. Rejeitada pela regra de interface no consumidor e pela proteção do kernel.
- Interface exportada em `internal/agents/application/interfaces` com mock gerado: vantagem de mock automático; desvantagem de exportar contrato sem consumidor externo real, contrariando R6.2 e R6.3, e de aumentar a superfície pública do módulo. Rejeitada; os testes usam fake manual, padrão já adotado nas suítes do módulo.

## Consequências

### Benefícios Esperados

- Zero migration e zero estado paralelo: a auditoria de Runs já existente vira fonte de verdade da cota.
- Substrato intocado, sem drift novo de tooling de mocks.
- Query servida pelo índice existente, custo desprezível frente a uma chamada de LLM.
- Contrato privado pequeno, testável com fake manual e com teste de integração real via testcontainers.

### Trade-offs e Custos

- Acoplamento semântico: qualquer Run de qualquer agente do mesmo `resource_id` conta na mesma cota; hoje só existe `mecontrola-agent` (`whatsapp_inbound_consumer.go:23`), e a reintrodução de um segundo agente exige revisão desta ADR.
- Runs que falham após o dispatch consomem cota (RF-11), comportamento deliberado e registrado no PRD.
- Leitura por mensagem inbound adiciona uma round trip ao Postgres por dispatch, aceita pelo baixo custo relativo.

### Riscos e Mitigações

- Risco: crescimento de `platform_runs` degradar a contagem. Mitigação: o índice restringe o varrimento às linhas do usuário; se a volumetria futura exigir, uma projeção agregada pode ser introduzida sem mudar o contrato do consumidor.
- Risco: divergência entre contagem e percepção do usuário em dia de instabilidade (Runs falhos contam). Mitigação: regra explícita no PRD e mensagem de bloqueio que informa renovação à meia noite.
- Rollback: desativação por `AGENT_DAILY_INTERACTION_LIMIT=0` remove a consulta do caminho quente, pois o resolvedor retorna permitido sem chamar o contador.

## Plano de Implementação

1. Declarar `dailyInteractionCounter` no pacote `usecases`.
2. Implementar `daily_interaction_counter.go` em `internal/agents/infrastructure/repositories/postgres` com a query de contagem.
3. Cobrir com `daily_interaction_counter_integration_test.go` (build tag `integration`, provisionamento via `internal/platform/testcontainer/postgres.go:13-16`).
4. Critério de conclusão: teste de integração verde cobrindo filtro por `resource_id`, filtro por janela e fronteira inclusiva do `since`; gate `task ci:agent-boundary` sem violação de SQL fora de repositório.

## Monitoramento e Validação

- Erros de contagem propagados com wrap `%w` e registrados em log `Error`; métrica de bloqueio `agents_daily_limit_total` separa `allowed` de `blocked`.
- Latência da query observável pelos spans do use case `agents.usecase.resolve_daily_interaction_limit`.
- Critério para revisão: degradação de latência do dispatch, introdução de segundo agente ou adoção de limites por plano.

## Impacto em Documentação e Operação

- Techspec e esta ADR documentam a fonte de contagem; nenhum runbook novo é exigido, pois não há estado operacional novo.
- Dashboards existentes podem adicionar painel da métrica de bloqueio sem mudança de infraestrutura.

## Revisão Futura

- Revisar ao introduzir limites por plano (fonte passaria a combinar quota por assinatura), ao registrar segundo agente, ou se a contagem por query mostrar custo relevante em produção.
