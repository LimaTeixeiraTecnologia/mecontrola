# Refator advisory — `internal/budgets`

> Modo: **advisory** (default do prompt em `docs/refactors/internal-budgets.md`).
> Nenhuma alteração de comportamento, contrato público, evento publicado, payload de outbox, schema de banco, semântica de erro HTTP ou ordem de side effects será proposta. Toda sugestão preserva rigorosamente a semântica atual e pode ser executada incrementalmente como diff seguro.
>
> Skill obrigatória declarada: `.agents/skills/go-implementation/SKILL.md` (e `.agents/skills/refactor/SKILL.md` para qualquer execução posterior).
> Espelhar este plano em `docs/planos/2026-06-12-refactor-internal-budgets.md` após aprovação (preferência do usuário).

## Contexto

O módulo `internal/budgets` cresceu organicamente cobrindo: criação/ativação/exclusão de orçamento mensal, upsert/exclusão de despesas (interna e externa via webhook), alertas de threshold (80/100%), recorrência, eventos pendentes (com idempotência por `event_id`), abandonment de drafts e purga de retenção. A separação `infrastructure → application → domain` está respeitada e os adapters (HTTP handlers, jobs, consumers, producer outbox) já estão finos. Os agregados centrais — `Budget` e `Expense` — possuem smart constructors razoáveis e invariantes razoavelmente protegidos.

Por outro lado a exploração detectou pontos de erosão que não exigem mudança de comportamento:

1. **Lógica de transição duplicada** entre `services.AlertStateResolver`, `EvaluateAlert.insertAlert` e a SQL CTE em `ThresholdStateRepository.UpsertIfTransition` (decisão de retroatividade aparece três vezes).
2. **Entidades anêmicas** (`Alert`, `ThresholdState`) cuja mutação de estado vive nos use cases / SQL, não no agregado.
3. **Use cases grandes e com múltiplas responsabilidades** (`IngestExternalExpense` ~277 LOC, `EvaluateAlert` ~248, `UpsertExpense` ~246) misturam pré-validação, roteamento, persistência e publicação no mesmo arquivo.
4. **Decisão de evento dentro do use case**: `UpsertExpense` constrói `interfaces.ExpenseCommittedEnvelope` (com `MutationKind` e `CutoffCompetenceBR`) inline; o producer já é fino, mas o envelope é uma estrutura `interfaces.` e não um evento de domínio tipado.
5. **State machine implícita** em `Alert.state` (5 valores) e `PendingEvent.state` (4 valores) sem invariante de transição explícita no agregado.
6. **Mappers** estão saudáveis hoje, mas `mappers.summary.go` faz cálculo de percentuais em cima do `Budget` carregado — fronteira entre projeção e regra é tênue.
7. **Hardcode de estados em SQL** (`AlertRepository.CountDelivered` usa `state IN (1, 2)`).

A motivação técnica: aumentar coesão, mover decisões para o domínio (inspirado seletivamente em DMMF, espelhando o que já foi adotado em `internal/transactions`), reduzir o risco de drift de regra e diminuir a superfície dos use cases — **sem** mexer em invariantes externos.

## Princípios e restrições

- **R0–R7** (`go-implementation/references/architecture.md`) são `[HARD]`. Aplicar especialmente R6 (`context.Context` em IO), R7.6 (`errors.Join` para acumular validação, `%w` para wrapping) e R-ADAPTER-001 (zero comentário em `.go`, adapters finos).
- Inspiração DMMF é **seletiva**: smart constructors mais fortes onde houver invariante real; `Decide*` puro apenas onde já existir decisão complexa (alert, pending-event, recurrence); evento de domínio tipado quando o producer hoje recebe estrutura semântica de evento. **Não** introduzir `Result/Either`, monad pipelines, currying, function-as-DI ou sealed unions artificiais.
- Nenhuma proposta cria interface sem consumidor real, nem refactor cosmético sem ganho.
- Sem `panic`, sem `init()`, sem `clock.Clock` injetado (memory: tempo é `time.Now().UTC()` inline).
- Todas as propostas são **advisory**: cada item é um diff pequeno, testável independentemente, com gate de regressão claro.
- Comportamento observável imutável: forma do JSON HTTP, status code, payload do outbox (`budgets.expense.committed.v1`), shape do `outbox.Event`, ordering de side effects, métricas e nomes de spans permanecem idênticos.

## Plano incremental (ordem sugerida, do diff menor → maior)

Cada item lista: **arquivo(s) afetado(s)**, **princípio DMMF aplicado e trade-off**, **validação proporcional**, **risco residual**.

### F1. Eliminar hardcode de `state IN (1, 2)` em `AlertRepository`
- **Arquivos**: `internal/budgets/infrastructure/repositories/postgres/alert_repository.go` (~linha 64) e `internal/budgets/domain/entities/alert.go` (expor função `DeliveredAlertStates() []int` ou similar, sem expor o valor numérico).
- **Inspiração**: encapsulamento DDD básico (não DMMF).
- **Trade-off**: +1 função de domínio; remove risco silencioso se a enum `AlertState` mudar.
- **Validação**: testes unitários existentes do repositório (se houver) + `go test ./internal/budgets/...`.
- **Risco**: nenhum — refactor literal.

### F2. Promover transições `PendingEvent` para invariante de agregado
- **Arquivos**: `internal/budgets/domain/entities/pending_event.go`.
- **Mudança**: `Transition(to PendingState, reason, now)` já existe; reforçar com tabela de transições válidas (Pending→Applied, Pending→Failed, Pending→Expired) e rejeitar transições indevidas com erro tipado (`ErrPendingInvalidTransition`). Hoje o método aceita silenciosamente alguns caminhos.
- **Princípio DMMF**: state-as-method com invariante explícita no agregado (não state-as-type — não vale a pena criar união discriminada aqui).
- **Trade-off**: erro novo em domínio; chamadores (apply_pending_event, reaper) precisam tratar — mas comportamento atual já reflete essa intenção, então propagação é trivial.
- **Validação**: `go test ./internal/budgets/domain/entities/...` cobrir matriz de transições; smoke em `apply_pending_event` e `run_pending_events_reaper`.
- **Risco**: baixo — caminhos hoje válidos continuam válidos; só os indevidos passam a falhar explicitamente.

### F3. Centralizar regra de retroatividade no `AlertStateResolver`
- **Arquivos**: `internal/budgets/domain/services/alert_state_resolver.go`, `internal/budgets/application/usecases/evaluate_alert.go`.
- **Problema**: `EvaluateAlert.insertAlert` chama `Resolve` duas vezes (~linhas 190 e 199) e o `Resolve` é chamado com inputs parcialmente repetidos. Além disso, a regra "expenseCompetence < cutoff ⇒ SuppressedRetroactive" também aparece na CTE do `UpsertIfTransition`.
- **Mudança**: encapsular a decisão completa (`competence`, `cutoff`, `deliveredCount`) num único `Resolve(ctx)` retornando `AlertDecision{State, Reason}`; eliminar a segunda chamada. **Não** mover a regra para fora da SQL nesta fase — fica como F8.
- **Princípio DMMF**: função de decisão pura (sem `ctx`, sem repo) na camada domain.
- **Trade-off**: clareza e fonte única; SQL continua duplicando a verificação até F8.
- **Validação**: tabela de testes do resolver cobrindo combinações (retroactive × ratelimited × normal); `go test ./internal/budgets/application/usecases -run EvaluateAlert`.
- **Risco**: baixo, somente leitura de dados já carregados.

### F4. Extrair `Decide*` puro para alerta
- **Arquivos**: novo `internal/budgets/domain/services/alert_workflow.go` com `DecideAlert(cmd, budget, sumByRoot, currentlyCrossed, transitions, deliveredCount, now) AlertDecision`; refactor de `evaluate_alert.go` para load → decide → persist.
- **Princípio DMMF**: `Decide*` puro (espelhando `internal/transactions` adr-006), inputs imutáveis, sem `ctx`/repo.
- **Trade-off**: +1 arquivo; ganho real porque `EvaluateAlert` passa a ser orquestração trivial e testável sem mocks.
- **Validação**: testes unitários puros do `DecideAlert`; testes existentes de `EvaluateAlert` continuam passando sem alteração de input/output.
- **Risco**: médio — refactor estrutural do maior use case da camada de alertas; deve sair em diff isolado.

### F5. Tipar `ExpenseCommittedEnvelope` como evento de domínio
- **Arquivos**: hoje em `internal/budgets/application/interfaces/...` (definição) e `internal/budgets/infrastructure/messaging/database/producers/expense_committed_publisher.go` (mapeamento → outbox).
- **Mudança**: mover `ExpenseCommittedEnvelope` para `internal/budgets/domain/events/expense_committed.go` (struct + smart constructor `NewExpenseCommitted(...)` validando `MutationKind`, `CutoffCompetenceBR`, etc.); o producer continua fino, agora recebendo um evento de domínio já decidido. `UpsertExpense` e `DeleteExpense` constroem o evento via smart constructor.
- **Princípio DMMF**: evento como tipo de domínio (não estrutura `interfaces.`); construtor protege payload do outbox.
- **Trade-off**: rename + path; **payload JSON do outbox permanece idêntico** (tags `json` preservadas) — validar em teste de serialização.
- **Validação**: snapshot do JSON publicado (golden file) antes/depois; teste do consumer existente continua deserializando idem.
- **Risco**: médio — toca múltiplos pacotes; gate de regressão é o golden do outbox.

### F6. Extrair allowlist e roteamento de `IngestExternalExpense`
- **Arquivos**: `internal/budgets/application/usecases/ingest_external_expense.go` (277 LOC) → dividir em (a) `validateIngestPreconditions` (allowlist de source + shape do command), (b) `routeMutation` (create/update/delete/pending). **Não** criar interface nova — manter como funções privadas no pacote `usecases`.
- **Princípio**: SRP em use case; não é DMMF stricto sensu, é higiene.
- **Trade-off**: 1 arquivo vira 1 arquivo maior + 2 menores; nenhum contrato externo muda.
- **Validação**: testes existentes do `ingest_external_expense_test.go`; cobertura precisa permanecer ≥ atual.
- **Risco**: baixo, mecânico.

### F7. Smart constructor reforçado em `Allocation` + `Budget.AddAllocation`
- **Arquivos**: `internal/budgets/domain/entities/budget.go`, `internal/budgets/domain/entities/allocation.go`.
- **Problema**: `Budget.SetAllocations` aceita lista; nada impede chamada com soma `≠ 10000` antes do `Activate`. Hoje só `Activate` valida — risco baixo mas latente.
- **Mudança**: expor `Budget.AddAllocation(a Allocation) error` que reforça soma incremental + manter `SetAllocations` apenas como helper interno chamado pelo `HydrateBudget`.
- **Princípio DMMF**: invariante always-on no agregado.
- **Trade-off**: pequeno; chamadores existentes (use cases de create/recurrence) precisam adaptar para AddAllocation.
- **Validação**: testes de entidade cobrindo soma inválida; `create_budget_test.go` e `create_recurrence_test.go`.
- **Risco**: baixo.

### F8. (Avaliar) Mover CTE de `UpsertIfTransition` para domain service
- **Arquivos**: `internal/budgets/infrastructure/repositories/postgres/threshold_state_repository.go` + novo `internal/budgets/domain/services/threshold_transition_decider.go`.
- **Custo**: alto — exige reescrever fluxo de persistência (decidir antes em domínio, depois `INSERT … ON CONFLICT DO UPDATE` simples). **Recomendação**: marcar como fora do escopo desta rodada advisory e abrir como follow-up. Se executado, validar com teste de integração contra Postgres real (já existe `migrations_integration_test.go`).
- **Risco**: alto se feito agora — mexe em concorrência e detecção de retroatividade. **Sugerir como F8 só após F1–F5 consolidarem**.

## O que fica em domain, application e infrastructure após F1–F7

- **domain/**: ganha `events/expense_committed.go`, `services/alert_workflow.go` (`DecideAlert`), enriquecimento de `entities/pending_event.go` (transições explícitas) e `entities/budget.go` (`AddAllocation`). Resolver de alerta consolida competence + cutoff + ratelimite.
- **application/**: use cases ficam mais finos (`evaluate_alert.go`, `upsert_expense.go`, `ingest_external_expense.go`); `interfaces/` deixa de hospedar `ExpenseCommittedEnvelope` (agora evento de domínio).
- **infrastructure/**: producer continua fino — só passa a receber `events.ExpenseCommitted` em vez de `interfaces.ExpenseCommittedEnvelope`. Repositório de alerta deixa de referenciar inteiros mágicos.

## Validação (proporcional)

- `task test` (unit) cobrindo `internal/budgets/...` após cada F.
- `task lint` + gate R-ADAPTER-001.1 (grep de comentário em `.go`).
- Golden file do JSON do outbox (F5).
- Integration test existente (`migrations_integration_test.go`) intocado — só roda para F8.
- Reportar diff de cobertura por pacote.

## Riscos remanescentes

1. F4 e F5 são os refactors maiores; mesmo sem mudança de contrato, devem sair em PRs separados e revisados isoladamente.
2. F8 (CTE → domínio) tem risco de concorrência; tratá-lo só após estabilização de F1–F7.
3. Qualquer divergência entre o que o producer publica e o que o consumer deserializa quebraria silenciosamente — golden file mitiga.
4. Os mappers (`mappers/summary.go`) seguem com cálculo de percentual; só intervir se for promovido a domain service em follow-up (não incluído aqui para preservar diff mínimo).

## Pontos fora de escopo (registrados explicitamente)

- Mudança de semântica de `Budget`, `Expense`, `Alert` ou recorrência.
- Mudança no shape do evento `budgets.expense.committed.v1`.
- Mudança em schema de banco, migrations ou índices.
- Introdução de `Result/Either`, monad pipelines ou DSL de workflow.
- Refactor de `categories_reader` (boundary externa intacta).

## Saída esperada do prompt

- `task_type`: `refactor.advisory.module`.
- Plano incremental: **F1 → F7** (F8 como follow-up).
- Justificativa DMMF por item: registrada acima.
- Validações proporcionais: registradas acima.
- Estado final: `done` (advisory; nenhuma execução nesta rodada).
