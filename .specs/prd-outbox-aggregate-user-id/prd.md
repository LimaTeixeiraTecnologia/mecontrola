# Documento de Requisitos do Produto (PRD) — Outbox Aggregate User ID Top-Level

<!-- spec-version: 1 -->

<!--
Histórico de versões:
- v1 (2026-06-12): padronização do envelope `outbox.Event` para carregar `AggregateUserID` como campo top-level (deixar de estar embutido apenas no payload JSON). Decisão deriva da análise crítica de isolamento per-user em `~/.claude/plans/analise-de-forma-criteriosa-shiny-book.md` seção 3 e do bugfix `.specs/prd-gateway-auth-forensics/bugfix_report.md` que cobriu queries SQL. Foco: MVP robusto, eficiente, econômico, production-ready/proof, sem falso positivo, inegociável. Skill `go-implementation` obrigatória.
-->

## Visão Geral

O pacote `internal/platform/outbox` define o envelope canônico de eventos publicados via outbox transacional. Hoje, o `outbox.Event` carrega `AggregateType` e `AggregateID` top-level, mas **NÃO** carrega `AggregateUserID` — o vínculo com o usuário dono do agregado fica embutido no payload JSON de cada evento (ex: `entities.TransactionCreated.UserID`, `entities.CardPurchaseCreated.UserID`, etc.).

Consequências dessa ausência:
1. **Enforcement de tenancy** depende de cada consumer parsear o payload JSON e extrair `user_id` — superfície ampla de erros silenciosos.
2. **Audit log e queries operacionais** sobre `outbox_events` (ex: "quantos eventos pendentes do usuário X?") exigem `payload->>'user_id'` em SQL, sem index, sem garantia de presença.
3. **LGPD / direito à exclusão**: anonimizar/purgar eventos de um usuário deletado exige scan completo da tabela com extração via JSON.
4. **Inconsistência com baseline**: `internal/transactions` (módulo de referência) prepara os eventos com `UserID` em `entities/events.go`, mas o envelope outbox descarta essa informação como campo de primeira classe.

Este PRD adiciona `AggregateUserID` como campo top-level no envelope `outbox.Event` (incluindo coluna em `outbox_events`, campo em `EventInput`, envelope JSON `Pack`, e atualização dos 12 callers atuais distribuídos em 5 módulos).

### Itens cobertos

- Schema: coluna `aggregate_user_id UUID NULL` em `mecontrola.outbox_events` + index parcial.
- Pacote `internal/platform/outbox`: campo `AggregateUserID` em `Event`, `EventInput`, `Row`; validação opcional na v1 (warning) e obrigatória na v2 (futuro PRD).
- Envelope JSON (`Pack`): expõe `aggregate_user_id` top-level para consumers.
- Storage Postgres: INSERT + SELECT + housekeeping consideram o novo campo.
- Atualização dos 12 callers em transactions, budgets, billing, identity, onboarding.
- Métrica `outbox_events_inserted_total{has_user_id}` para acompanhamento de cobertura.

### Itens Fora de Escopo

- **Validação obrigatória (NOT NULL + retorno de erro em `NewEvent` se ausente)**: fica para v2 — exige 100% de cobertura dos callers primeiro.
- **Backfill de registros antigos** com `payload->>'user_id'`: fora do escopo. Decisão (ADR-003): registros antigos permanecem com `aggregate_user_id NULL`, housekeeping eventualmente os limpa via `DeletePublishedBatch`.
- **RLS Postgres usando `aggregate_user_id`**: pós go-live, conforme item da segunda onda do plano-fonte.
- **Migração para Redis/distributed outbox**: fora do escopo de produção single-node.
- **Consumers externos (`/whatsapp/inbound`, `/kiwify/*`)**: não geram eventos via outbox interno; permanecem inalterados.

## Objetivos

- **OBJ-01**: tornar `user_id` do agregado um campo de primeira classe no envelope outbox, eliminando a necessidade de parsear payload JSON para tenancy.
- **OBJ-02**: padronizar todos os 12 callers atuais para popular `AggregateUserID` na construção do evento, alinhando com a entity já tipada (`evt.UserID()` em entities como `TransactionCreated`, `CardPurchaseCreated`, etc.).
- **OBJ-03**: habilitar consultas operacionais e auditoria por usuário sem JSON extraction (`WHERE aggregate_user_id = $1` em vez de `(payload->>'user_id')::uuid = $1`).
- **OBJ-04**: preparar terreno para enforcement obrigatório em v2 (gate de lint, validação no construtor, RLS futuro).
- **OBJ-05**: zero regressão em comportamento observável: consumers existentes continuam funcionando, dispatcher continua publicando, idempotência por `event_id` preservada.
- **OBJ-06**: skill `go-implementation` carregada em toda alteração (R0–R7, R-ADAPTER-001). Zero comentário em `.go` de produção.
- **OBJ-07**: rollout atômico: migration + código + atualização dos callers no mesmo deploy. Sem dual-write nem fase intermediária.

### Métricas de Sucesso

- **M-01**: 100% dos 12 callers atuais populam `AggregateUserID` na construção do evento. Verificado por grep + revisão de PR.
- **M-02**: `outbox_events_inserted_total{has_user_id="true"}` ≥ 99% do total de inserts em estado estacionário (1% de tolerância para eventos legítimamente sem dono, e.g. eventos de sistema). Métrica visível no dashboard.
- **M-03**: 0 (zero) regressão em testes existentes do outbox, dispatcher, reaper, housekeeping, registry.
- **M-04**: 0 (zero) comentário introduzido em `.go` de produção.
- **M-05**: migration aplica em `up` e reverte em `down` sem perda de dados (down apenas `DROP COLUMN`).
- **M-06**: `task lint && task test && task vulncheck` verde.
- **M-07**: 0 (zero) nova dependência em `go.mod`.
- **M-08**: gate de lint adicional `task lint:outbox-user-id` verifica que todo construtor de `outbox.EventInput` popula `AggregateUserID` (exceto eventos explicitamente de sistema, listados em allowlist).

## Histórias de Usuário

- **US-01 — Consultar eventos pendentes de um usuário sem JSON parsing**
  Como operador, quero rodar `SELECT count(*) FROM mecontrola.outbox_events WHERE aggregate_user_id = $1 AND status = 1` para responder em < 100ms quantos eventos pendentes existem para um usuário, sem depender de extração JSON.

- **US-02 — Audit log per-user via single column**
  Como responsável por LGPD, quero filtrar `outbox_events` por `aggregate_user_id` durante uma resposta a solicitação de exclusão, para não precisar parsear cada payload individualmente.

- **US-03 — Consumer tipado sem JSON extraction**
  Como autor de consumer (`ProjectAuthEvent`, `ApplyPendingEvent`, etc.), quero ler `evt.AggregateUserID` direto do envelope, para não precisar deserializar payload + reextrair user_id.

- **US-04 — Operador rastreia adoção via métrica**
  Como operador, quero acompanhar `outbox_events_inserted_total{has_user_id}` no dashboard para validar que 100% dos novos eventos populam o campo antes de promover a validação obrigatória v2.

- **US-05 — Migração sem downtime**
  Como operador, quero que o deploy aplique migration + código novo + producers atualizados juntos, e que registros antigos com `aggregate_user_id NULL` continuem funcionando para o reaper limpar via housekeeping normal.

## Requisitos Funcionais

### Schema

- **RF-01**: Migration `000017_outbox_events_aggregate_user_id.up.sql` adiciona `aggregate_user_id UUID NULL` em `mecontrola.outbox_events`.
- **RF-02**: Index parcial `outbox_events_aggregate_user_id_idx ON mecontrola.outbox_events (aggregate_user_id) WHERE aggregate_user_id IS NOT NULL`.
- **RF-03**: Migration `down` reverte com `DROP INDEX` + `DROP COLUMN` (preserva tabela).
- **RF-04**: Coluna é **NULL** na v1 (compat com registros antigos + agendamento de validação obrigatória para v2). Decisão em ADR-001.

### Pacote `internal/platform/outbox`

- **RF-05**: Struct `Event` ganha campo `AggregateUserID string` (string vazia = ausente; uuid.UUID seria mais tipado, mas string mantém consistência com `AggregateID string` existente).
- **RF-06**: Struct `EventInput` ganha campo `AggregateUserID string`.
- **RF-07**: Struct `Row` herda `Event`, portanto também ganha o campo automaticamente.
- **RF-08**: Função `NewEvent(input EventInput) (Event, error)` aceita `AggregateUserID` opcional na v1; se presente, valida que é UUID válido via `uuid.Parse`. Se ausente, log warn estruturado `outbox.event.missing_aggregate_user_id` com `event_type` (sem `user_id` em label).
- **RF-09**: Função `Pack(row Row) Envelope` inclui `AggregateUserID` no envelope JSON top-level (campo `aggregate_user_id`).
- **RF-10**: Métrica `outbox_events_inserted_total{has_user_id}` com label `has_user_id` ∈ {`"true"`, `"false"`}.

### Storage Postgres

- **RF-11**: `Insert` adiciona `aggregate_user_id` na lista de colunas e no `VALUES`. NULL quando string vazia.
- **RF-12**: `ClaimBatch` adiciona `aggregate_user_id` no SELECT e no Scan.
- **RF-13**: `DeletePublishedBatch` (housekeeping) sem mudança (segue `published_at`, não filtra por user).

### Callers (12 sites)

- **RF-14**: Todos os 12 callers populam `AggregateUserID` quando o agregado tem dono. Lista exaustiva:
  - `internal/transactions/.../producers/transaction_event_publisher.go`
  - `internal/transactions/.../producers/card_purchase_event_publisher.go`
  - `internal/transactions/.../producers/recurring_template_event_publisher.go`
  - `internal/budgets/.../producers/expense_committed_publisher.go`
  - `internal/billing/.../producers/subscription_event_publisher.go`
  - `internal/identity/application/usecases/establish_principal.go`
  - `internal/identity/application/usecases/mark_user_deleted.go`
  - `internal/identity/module.go` (cabeamento)
  - `internal/onboarding/application/binding/subscription_binding.go`
  - `internal/onboarding/application/events/subscription_bound.go`
  - `internal/onboarding/module.go` (cabeamento)
  - `internal/platform/whatsapp/dispatcher/dispatcher.go`
- **RF-15**: Allowlist de eventos legitimamente sem `user_id` (e.g. eventos de sistema), declarada em `internal/platform/outbox/system_event_allowlist.go` com tipo `EventType` constantes. Hoje provavelmente vazia ou cobrindo poucos casos — definir durante implementação.

### Gate de Revisão

- **RF-16**: `deployment/scripts/lint-outbox-user-id.sh` falha CI se construtor de `outbox.EventInput` em código de produção for chamado sem populá-lo. Allowlist explícita por `Type` constante.
- **RF-17**: Receita `task lint:outbox-user-id` em `taskfiles/lint.yml`.

### Cross-Cutting

- **RF-18**: Skill `go-implementation` carregada (R0–R7, R-ADAPTER-001, zero comentários).
- **RF-19**: `task lint && task test && task vulncheck` verde.
- **RF-20**: Zero nova dependência em `go.mod`.

## Riscos e Mitigações

- **R-01**: Migration causa lock longo na tabela `outbox_events` se tiver muitos registros. **Mitigação**: `ALTER TABLE ... ADD COLUMN UUID NULL` em Postgres é metadata-only (instantâneo). Index criado com `CREATE INDEX CONCURRENTLY` na migration.
- **R-02**: Caller esquecer de popular `AggregateUserID` em PR futuro. **Mitigação**: gate `lint:outbox-user-id` falha CI.
- **R-03**: Validação opcional v1 deixa eventos passarem sem `user_id` por bug. **Mitigação**: métrica + alerta operacional `has_user_id="false"` > 1% por 10 min → operador investiga.
- **R-04**: Consumers existentes podem quebrar se envelope JSON ganhar campo novo. **Mitigação**: Go `encoding/json` ignora campos desconhecidos por default; consumers não tipados (mapas) ganham campo extra inofensivo.
- **R-05**: Backfill de registros antigos seria caro e arriscado. **Mitigação**: aceitar registros antigos com NULL; housekeeping limpa registros publicados em ~30 dias; eventualmente todos os registros pendentes serão recentes.
- **R-06**: Dispatcher/reaper podem assumir comportamento atual. **Mitigação**: cobertura por testes existentes do dispatcher (não tocam estrutura, apenas leem campos). Validar que testes continuam verdes.

## Critério de Aceitação Global

- Todos os 20 RFs implementados.
- 8 ADRs cravadas (4 decisões reais + 4 documentações de decisões já tomadas).
- Gate `task lint:outbox-user-id` verde no CI.
- Métrica `outbox_events_inserted_total{has_user_id="true"}` ≥ 99% em smoke staging.
- `task lint && task test && task vulncheck` verde.
- Sem nova dependência em `go.mod`.
- Sem alteração de comportamento observável dos consumers existentes.
