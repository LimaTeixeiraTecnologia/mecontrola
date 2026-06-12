# DOSSIÊ DE DISCOVERY TÉCNICO

## Título
internal/transactions - Modulo de Lancamentos Mensais (Income/Outcome) com Fatura de Cartao e Recorrencia

## Resumo Executivo
Contexto:
O aplicativo precisa de fonte canonica de movimentacoes financeiras por usuario e mes de referencia. Hoje `internal/budgets` e `internal/card` existem, mas nao ha modulo central que registre entradas e saidas (income/outcome) com agregados `income`, `outcome`, `total` por (`user_id`, `ref_month`). Lancamentos de cartao de credito precisam compor uma fatura com competencia derivada das datas de fechamento/vencimento do `internal/card`, incluindo parcelamento. Cada movimento deve disparar eventos de dominio via outbox para que `internal/budgets` consuma sem acoplamento sincrono. Ausencia do modulo bloqueia o uso do app.

Recomendação:
Construir o modulo `internal/transactions` (Go 1.26.4) seguindo o Padrao Obrigatorio de Modulo do repositorio (R-ADAPTER-001, R-GOV-001 e Regras Estritas R0-R7 de `go-implementation`). Adotar a Alternativa 2 confirmada no brainstorming: agregados `Transaction`, `CardPurchase`, `CardInvoice`, `CardInvoiceItem`, `RecurringTemplate`; projecao `monthly_summary` mantida por consumer reativo do outbox + job diario de reconciliacao; parcelamento expandido na criacao em 1 `CardPurchase` ancorando N `CardInvoiceItem`s; snapshot de `BillingCycle{ClosingDay,DueDay}` lido do `internal/card` no momento do create; idempotency-key middleware obrigatorio + optimistic locking via coluna `version BIGINT`; OTel completo com 4 alertas operacionais; rollout big-bang controlado por feature flag `transactions.enabled`.

Status de viabilidade:
viável

## Necessidade e Objetivos
Problema atual:
Nao existe modulo no repositorio que registre lancamentos financeiros do usuario nem que materialize a fatura simulada de cartao de credito. Sem ele, `internal/budgets` nao recebe consumo real por categoria e o usuario nao consegue acompanhar caixa diario, o que bloqueia o uso pratico do app.

Objetivos de negócio:
- Permitir que o usuario registre, edite, consulte e exclua todo lancamento de income/outcome do mes corrente e meses anteriores.
- Apresentar para cada mes (`ref_month` no formato `YYYY-MM`) os totais `income`, `outcome` e `total = income - outcome` com atualizacao perceptivel apos cada operacao.
- Para lancamentos pagos com cartao de credito, simular a fatura do mes correto (competencia derivada de fechamento/vencimento do cartao) e suportar parcelamento em ate 24 vezes.
- Disparar eventos de dominio para que `internal/budgets` atualize consumo por categoria sem acoplamento sincrono com a UI.
- Suportar lancamento recorrente (salario mensal, assinatura, prestacao fixa) automatizado por job mensal idempotente.

Objetivos técnicos:
- Modulo `internal/transactions` aderente ao Padrao Obrigatorio de Modulo (domain, application, infrastructure/{http,messaging/database,jobs}) e ao contrato R-ADAPTER-001 (zero comentarios em codigo Go de producao, adapter fino handler->usecase).
- Persistencia em PostgreSQL com tabelas separadas por agregado e indices focados no padrao de leitura por (`user_id`, `ref_month`).
- Idempotency-Key middleware (`scope=transactions`) em todas as mutacoes + optimistic locking via coluna `version BIGINT`.
- Outbox at-least-once com retry exponencial (backoff 3s,9s,27s,81s; `RetryMaxAttempts` configuravel) e DLQ logica com metrica e alerta.
- OTel completo: spans `transactions.usecase.*`, `transactions.repository.*`, `transactions.consumer.*`, `transactions.job.*`; metricas Prometheus RED + drift de summary + consumer lag + dead-letter; logs estruturados sem PII de descricao/amount.
- Testes: suites unitarias com mockery + testify/suite e suites de integracao com testcontainers Postgres (build tag `integration`).
- Feature flag `transactions.enabled` (em `configs.TransactionsConfig`) para rollback rapido.

## Materiais de Apoio
- Bundle decision-brainstorming: `.agents/skills/decision-brainstorming/discoveries/brainstorm-modulo-lancamentos-mensais-income-outcome/` (decision-brief.md, transcript.md, assumptions.md, option-scorecard.md, bundle.json).
- Modulos do repositorio: `internal/budgets`, `internal/card`, `internal/categories`, `internal/identity`.
- Plataforma compartilhada: `internal/platform/outbox` (publisher.go, outbox.go, storage_postgres.go), `internal/platform/idempotency/middleware.go`, `internal/platform/testcontainer/postgres.go`.
- Migracoes existentes: `migrations/000007_create_platform_idempotency_keys.up.sql`, `000008_create_card_cards.up.sql`, `000012_create_budgets_baseline.up.sql`, `000013_create_budgets_abandoned_draft_signals.up.sql`.
- Auth: `internal/identity/infrastructure/http/server/middleware/inject_principal_from_header.go` (header `X-User-ID`, `auth.WithPrincipal`).
- Governanca: `AGENTS.md`, `CLAUDE.md`, `.claude/rules/governance.md`, `.claude/rules/go-adapters.md`.
- Regras de implementacao Go: `.agents/skills/go-implementation/SKILL.md` (R0-R7) e `references/INDEX.yaml`.
- BACEN: Resolucao BCB 290/2023 (descontinuacao do DOC com fim operacional em 2024); PIX e TED como meios eletronicos vigentes em 2026.
- `go.mod`: Go 1.26.4.

## Escopo
Inclui:
- Agregado `Transaction` para lancamentos avulsos com tipo entrada/saida e formas de pagamento `pix`, `ted`, `debit_in_account`, `debit_card`, `cash`, `boleto`.
- Agregado `CardPurchase` (compra-pai de cartao de credito) que ancora N `CardInvoiceItem`s linkados via `purchase_id` e agrupados em `CardInvoice` por (`card_id`, `ref_month`).
- Agregado `RecurringTemplate` (modelo de lancamento recorrente) com frequencia (`monthly`, `yearly`), data de inicio, data de fim opcional e regras de materializacao mensal idempotente.
- Projecao `monthly_summary(user_id, ref_month, income_cents, outcome_cents, total_cents, version, updated_at)` mantida por consumer do outbox; job diario de reconciliacao com metrica de drift.
- Endpoints REST CRUD versionados em `/v1`: `POST/GET/PATCH/DELETE /v1/transactions`, `POST/GET/PATCH/DELETE /v1/card-purchases`, `POST/GET/PATCH/DELETE /v1/recurring-templates`, `GET /v1/months/{ref_month}` (resumo), `GET /v1/months/{ref_month}/entries` (extrato unificado paginado).
- Idempotency-Key middleware obrigatorio em POST/PATCH/DELETE com escopo `transactions`.
- Optimistic locking por `version BIGINT` em todos os agregados mutaveis.
- Eventos publicados via `outbox.Publisher`: `transactions.transaction.{created,updated,deleted}.v1`, `transactions.card_purchase.{created,updated,deleted}.v1`, `transactions.recurring_template.{created,updated,deleted}.v1`.
- OTel completo (spans, metricas Prometheus, logs estruturados, tracing distribuido com `trace_id` propagado no `outbox.Event.Metadata`).
- Paginacao cursor base64(`created_at`,`id`) consistente com `internal/budgets`.
- Multi-tenant logico: toda query filtra por `user_id` extraido do principal middleware.
- Feature flag `configs.TransactionsConfig.Enabled` para rollback.

Exclui:
- Transferencia entre contas (mover dinheiro entre contas proprias do usuario) - fica para v2.
- Multi-moeda - MVP somente em BRL com `amount_cents INT8` para preservar precisao decimal.
- Anexos/comprovantes (upload de nota/recibo) - fica para v2.
- DOC como forma de pagamento aceita em novos creates (descontinuado pela BCB em 2024). Leitura aceita o token `doc` apenas em registros legados/imports.
- Imutabilidade de fatura pos-pagamento (edicao retroage em faturas fechadas no MVP).
- Consumo de eventos de `internal/card` e `internal/categories` no MVP (snapshot estatico).
- Relatorios analiticos avancados, exportacao CSV/PDF, GraphQL.

## Premissas e Restrições
Premissas:
- `internal/card` expoe `CardRepository.GetByIDForUser(ctx, cardID, userID)` retornando entidade com `Cycle.ClosingDay` e `Cycle.DueDay`. O modulo `internal/transactions` consome esse contrato como reader via interface declarada no `application/interfaces/` do proprio modulo (interface no consumidor - R6.3).
- `internal/categories` expoe `CategoryRepository.GetByID(ctx, id)` para validar categoria e subcategoria; subcategoria nao ha relacionamento parent_id explicito (referenciado via `parent_id` opcional no query type) e o transactions usa `GetByID` por id da subcategoria e id da categoria-pai.
- `outbox.Publisher` (em `internal/platform/outbox`) e operacional, com `event_id` validado como UUID, retry/backoff configuravel e tabela `platform.outbox`. Idempotencia por `event_id` e garantida.
- `idempotency.Middleware` (em `internal/platform/idempotency`) e operacional, com chave composta `(scope, key, user_id)` na tabela `mecontrola.idempotency_keys`.
- Principal middleware `InjectPrincipalFromHeader` lê `X-User-ID` e injeta `auth.Principal{UserID}` no contexto.
- Job scheduler do repositorio (similar ao `BudgetsConfig.AbandonedDraftCron`) e usado pelo `RecurringTemplate` job mensal e pelo `MonthlySummaryReconciler` job diario.
- Volumetria MVP: 1k usuarios x 100 lancamentos/mes + parcelamento medio 3x = ~100k linhas/mes em `transactions` e ~30k linhas/mes em `card_invoice_items`. Sem particionamento no MVP.
- Resolucao BCB 290/2023 confirma fim do DOC em 2024; PIX, TED, debito em conta, cartoes (credito/debito), dinheiro e boleto sao os meios canonicos.

Restrições:
- Versão Go: 1.26.4 (declarada em `go.mod`).
- R-GOV-001: governanca transversal aplicada.
- R-ADAPTER-001: zero comentarios em arquivos `.go` de producao (excecao apenas para `//go:`, `//nolint:` e `// Code generated`); adapter fino com fluxo `handler/consumer/job/producer -> usecase`; sem SQL direto em adapter.
- R0-R7 do `go-implementation` (sem `init()`, sem panic em producao, `context.Context` em fronteiras de IO, interface no consumidor, sem `var _ Interface = (*Type)(nil)`, `errors.Join` e `%w`, `log/slog` ou `observability.Logger`, sem `clock.Clock` em usecase/repository).
- Persistencia exclusivamente PostgreSQL via golang-migrate; sem trigger SQL para regra de dominio.
- Outbox e padrao obrigatorio para mensageria de dominio; sem broker externo no MVP.
- Multi-tenant logico por `user_id`; sem RLS no Postgres no MVP (filtro aplicacional).
- Sem `interface{}`/`any` em assinaturas de dominio; usar tipos concretos (R6.2) com interfaces apenas nas fronteiras consumidoras.
- Sem framework de DI externo; wiring em `internal/transactions/module.go` similar a `internal/budgets/module.go`.

## Viabilidade Técnica
Status:
viável

Justificativa:
Todas as primitivas requeridas ja existem no repositorio e em uso por modulos analogos: `outbox.Publisher`, `idempotency.Middleware`, principal middleware, `observability.Observability`, padroes de cursor, mockery + testify/suite + testcontainers Postgres, golang-migrate, `responses.ErrorWithDetails`. O agregado `CardPurchase` reaproveita o `internal/card` apenas como reader. A complexidade do MVP esta dentro do orcamento do time, comparavel a entrega recente de `internal/budgets` (commit 099671e). Volumetria MVP cabe em PostgreSQL sem particionamento. Risco residual mais relevante (consistencia eventual de `monthly_summary`) tem mitigacao desenhada (consumer + reconciliacao diaria + metrica de drift + alerta).

Bloqueadores:
- Nenhum bloqueador estrutural identificado.

## Arquitetura Atual
- `internal/budgets`: agregados ricos `Budget`, `Expense`, `Alert`, `ThresholdState`, com use cases write/read, consumers, producers, jobs (`BudgetsConfig.AbandonedDraftCron`). Optimistic locking por `version BIGINT`. Eventos `budgets.expense.committed.v1` publicados via outbox.
- `internal/card`: agregado `Card` com `BillingCycle{ClosingDay,DueDay}`; reader `CardRepository.GetByIDForUser` disponivel.
- `internal/categories`: `CategoryRepository.List/GetByID`; subcategoria modelada como `Category` com `ParentID`.
- `internal/identity`: principal middleware (`X-User-ID` -> `auth.Principal{UserID}`).
- `internal/platform/outbox`: publisher + storage Postgres + dispatcher; eventos com `ID` UUID, `Type` dot-separated, `AggregateType`, `AggregateID`, `Payload []byte`, `Metadata map[string]string`, `OccurredAt time.Time`.
- `internal/platform/idempotency`: middleware HTTP que persiste `(scope, key, user_id)` em `mecontrola.idempotency_keys` com `request_hash` SHA-256 e cache de status/body para replays.
- Configs unificadas em `configs/config.go` via Viper + env (`mapstructure`).
- Observabilidade via `observability.Observability` (Tracer + Logger + atributos).
- Sem modulo de lancamentos; sem modelo de income/outcome; sem entidade Fatura.

## Arquitetura Proposta
Componentes:
- `internal/transactions/domain/entities/{transaction,card_purchase,card_invoice,card_invoice_item,recurring_template,monthly_summary}.go`: agregados ricos com factory function (`NewTransaction(...)`) e metodos `Update*` retornando `error` com `errors.Join` para multi-violacao.
- `internal/transactions/domain/valueobjects/{amount,payment_method,direction,ref_month,installments,frequency,money_brl}.go`: VOs com validacao no construtor.
- `internal/transactions/domain/services/{competence_calculator,monthly_summary_reducer}.go`: regras puras de dominio (calculo de `ref_month` a partir do `BillingCycle` snapshot; redutor de `income/outcome/total`).
- `internal/transactions/application/interfaces/{transaction_repository,card_invoice_repository,card_purchase_repository,recurring_template_repository,monthly_summary_repository,card_reader,category_reader,event_publisher,clock_reader}.go`: interfaces declaradas no consumidor.
- `internal/transactions/application/usecases/{create_transaction,update_transaction,delete_transaction,list_transactions,create_card_purchase,update_card_purchase,delete_card_purchase,list_card_purchases,create_recurring_template,update_recurring_template,delete_recurring_template,list_recurring_templates,get_month_summary,list_month_entries,recompute_monthly_summary,materialize_recurring_month}.go`: use cases com Command Object (R6.6) para writes.
- `internal/transactions/application/dtos/{input,output}/...`: DTOs.
- `internal/transactions/application/mappers/...`: conversao entity <-> dto.
- `internal/transactions/infrastructure/http/server/handlers/...`: handlers REST finos (R-ADAPTER-001.2) que chamam `usecase.Execute(ctx, cmd)` e mapeiam erro de dominio para status HTTP.
- `internal/transactions/infrastructure/http/server/router.go`: registro das rotas com `idempotency.Middleware(scope="transactions", ...)` e `InjectPrincipalFromHeader`.
- `internal/transactions/infrastructure/repositories/postgres/...`: implementacao das interfaces de repositorio.
- `internal/transactions/infrastructure/messaging/database/producers/...`: 3 producers (transaction, card_purchase, recurring_template) que serializam payload pre-decidido pelo usecase e chamam `outbox.Publisher.Publish`.
- `internal/transactions/infrastructure/messaging/database/consumers/monthly_summary_recompute_consumer.go`: consome eventos `transactions.transaction.*.v1` e `transactions.card_purchase.*.v1`, identifica `ref_month` afetado e dispara `RecomputeMonthlySummary` usecase.
- `internal/transactions/infrastructure/jobs/handlers/{recurring_materializer_job,monthly_summary_reconciler_job}.go`: jobs que chamam `MaterializeRecurringMonth` e job de reconciliacao diaria.
- `internal/transactions/module.go`: wiring (factory de repos, use cases, handlers, consumers, jobs); registro de feature flag.
- `configs/config.go`: nova `TransactionsConfig` com `Enabled bool`, `IdempotencyTTL time.Duration`, `RecurringMaterializeCron string`, `MonthlySummaryReconcileCron string`.

Fluxo de alto nível:
1. Cliente envia `POST /v1/transactions` (lancamento avulso) com header `Idempotency-Key` e `X-User-ID`.
2. `InjectPrincipalFromHeader` injeta `auth.Principal{UserID}`; `idempotency.Middleware` persiste/consulta a chave em `mecontrola.idempotency_keys`.
3. Handler valida payload (DTO -> Command), chama `CreateTransaction.Execute(ctx, cmd)`.
4. Use case carrega referencias necessarias (`category_reader.GetByID` para validar categoria + snapshot do nome) e cria agregado `Transaction` (factory function).
5. Use case abre transacao SQL via `manager.Manager`, persiste em `transactions` via repository, grava evento `transactions.transaction.created.v1` na mesma tx no outbox (`outbox.Publisher.Publish`). Commit.
6. Dispatcher do outbox publica o evento; `internal/budgets` consome para atualizar consumo por categoria; consumer interno `MonthlySummaryRecompute` recalcula `monthly_summary(user_id, ref_month)`.
7. Handler retorna `201 Created` com DTO de saida.

Fluxo paralelo para credito (`POST /v1/card-purchases`):
1. Cliente envia compra com `card_id`, `total_amount_cents`, `installments` (1..24), `purchased_at`, `description`, `category_id`.
2. Use case `CreateCardPurchase` valida principal e chama `card_reader.GetByIDForUser(ctx, cardID, userID)` para obter `BillingCycle`.
3. Calcula `competence_ref_month` da primeira parcela via `CompetenceCalculator.For(purchased_at, BillingCycle)` (se `purchased_at` antes do `closing_day` do mes, primeira fatura e o mes corrente vencendo no `due_day`; senao a proxima fatura).
4. Cria agregado `CardPurchase` ancorando N `CardInvoiceItem` (uma por parcela) com `ref_month` incrementado mes a mes. Garante existencia de `CardInvoice` por (`card_id`, `ref_month`) (upsert idempotente).
5. Persiste tudo em uma unica transacao SQL + evento `transactions.card_purchase.created.v1` com payload contendo array de items e seus refs.
6. Commit; outbox dispatcher publica; consumer recalcula `monthly_summary` para cada `ref_month` afetado.

Fluxo de recorrencia:
1. Job mensal `RecurringMaterializerJob` (cron configuravel) seleciona `recurring_templates` ativos (sem `ended_at` ou com `ended_at >= today`).
2. Para cada template, calcula a proxima ocorrencia (`monthly`/`yearly`) que cai no mes corrente.
3. Use case `MaterializeRecurringMonth(template_id, ref_month)` cria `Transaction` ou `CardPurchase` de forma idempotente: chave logica `(template_id, ref_month)`; constraint UNIQUE garante zero duplicacao em re-execucoes.

Decisão arquitetural:
Alternativa 2 (Transactions + CardPurchase + CardInvoice/Items) com `monthly_summary` mantido eventual por consumer reativo. Estrutura modular reaproveita 100% dos padroes do repositorio. Adapters finos cumprem R-ADAPTER-001. Use cases write com Command Object (R6.6) e use cases read com query objects estruturados. Interfaces declaradas no consumidor (`application/interfaces`) seguindo R6.3. Sem injecao de `clock.Clock` (R6.7); horario derivado de `time.Now().UTC()` no ponto de uso.

## Dados e Integrações
Domínios de dados:
- `transactions`: lancamento avulso. Colunas: `id UUID PK`, `user_id UUID NOT NULL`, `direction SMALLINT NOT NULL` (1=income, 2=outcome), `payment_method SMALLINT NOT NULL` (enum `iota+1`: pix, ted, debit_in_account, debit_card, cash, boleto, plus reserved `doc`=99 read-only), `amount_cents BIGINT NOT NULL` (>0), `description TEXT NOT NULL`, `category_id UUID NOT NULL`, `category_name_snapshot TEXT NOT NULL`, `subcategory_id UUID NULL`, `subcategory_name_snapshot TEXT NULL`, `occurred_at TIMESTAMPTZ NOT NULL`, `ref_month CHAR(7) NOT NULL` (`YYYY-MM`), `version BIGINT NOT NULL DEFAULT 0`, `created_at`, `updated_at`, `deleted_at NULL`. Indices: PK; `(user_id, ref_month, created_at, id)`; `(user_id, category_id)`; `(deleted_at) WHERE deleted_at IS NULL`.
- `card_purchases`: compra-pai de credito. Colunas: `id UUID PK`, `user_id UUID NOT NULL`, `card_id UUID NOT NULL`, `card_closing_day SMALLINT NOT NULL` (snapshot), `card_due_day SMALLINT NOT NULL` (snapshot), `direction SMALLINT NOT NULL DEFAULT 2`, `installments_total SMALLINT NOT NULL` (1..24), `total_amount_cents BIGINT NOT NULL`, `description TEXT NOT NULL`, `category_id UUID NOT NULL`, `category_name_snapshot TEXT NOT NULL`, `subcategory_id UUID NULL`, `subcategory_name_snapshot TEXT NULL`, `purchased_at TIMESTAMPTZ NOT NULL`, `version BIGINT NOT NULL`, `created_at`, `updated_at`, `deleted_at NULL`. Indices: PK; `(user_id, card_id, purchased_at)`.
- `card_invoices`: fatura por cartao e mes. Colunas: `id UUID PK`, `user_id UUID NOT NULL`, `card_id UUID NOT NULL`, `ref_month CHAR(7) NOT NULL`, `closing_at DATE NOT NULL`, `due_at DATE NOT NULL`, `items_total_cents BIGINT NOT NULL DEFAULT 0`, `version BIGINT NOT NULL`, `created_at`, `updated_at`. Constraint: `UNIQUE(user_id, card_id, ref_month)`.
- `card_invoice_items`: parcelas geradas. Colunas: `id UUID PK`, `purchase_id UUID NOT NULL FK -> card_purchases(id) ON DELETE RESTRICT`, `invoice_id UUID NOT NULL FK -> card_invoices(id) ON DELETE RESTRICT`, `user_id UUID NOT NULL`, `ref_month CHAR(7) NOT NULL`, `installment_index SMALLINT NOT NULL` (1..N), `amount_cents BIGINT NOT NULL`, `version BIGINT NOT NULL`, `created_at`, `updated_at`. Constraint: `UNIQUE(purchase_id, installment_index)`. Indices: `(user_id, ref_month, created_at, id)`; `(invoice_id)`.
- `recurring_templates`: template recorrente. Colunas: `id UUID PK`, `user_id UUID NOT NULL`, `direction SMALLINT NOT NULL`, `payment_method SMALLINT NOT NULL`, `card_id UUID NULL` (apenas quando `payment_method=credit_card`), `amount_cents BIGINT NOT NULL`, `description TEXT NOT NULL`, `category_id UUID NOT NULL`, `category_name_snapshot TEXT NOT NULL`, `subcategory_id UUID NULL`, `subcategory_name_snapshot TEXT NULL`, `frequency SMALLINT NOT NULL` (1=monthly, 2=yearly), `started_at DATE NOT NULL`, `ended_at DATE NULL`, `day_of_month SMALLINT NOT NULL` (1..28 para evitar drift de fim de mes), `installments_total SMALLINT NOT NULL DEFAULT 1`, `version BIGINT NOT NULL`, `created_at`, `updated_at`, `deleted_at NULL`. Indices: PK; `(user_id, ended_at)`; tabela auxiliar `recurring_materializations(template_id, ref_month) PK` para idempotencia do job.
- `monthly_summary`: projecao de leitura. Colunas: `user_id UUID NOT NULL`, `ref_month CHAR(7) NOT NULL`, `income_cents BIGINT NOT NULL DEFAULT 0`, `outcome_cents BIGINT NOT NULL DEFAULT 0`, `total_cents BIGINT NOT NULL DEFAULT 0`, `version BIGINT NOT NULL DEFAULT 0`, `recomputed_at TIMESTAMPTZ NOT NULL`. PK `(user_id, ref_month)`.

Integrações:
- `internal/card.CardRepository.GetByIDForUser` (somente leitura) - consumido por `card_reader` interface declarada em `internal/transactions/application/interfaces`.
- `internal/categories.CategoryRepository.GetByID` (somente leitura) - consumido por `category_reader` interface declarada em `internal/transactions/application/interfaces`.
- `internal/identity` principal middleware (`X-User-ID` -> `auth.Principal{UserID}`).
- `internal/platform/outbox.Publisher` para publicacao de eventos via mesma tx do agregado.
- `internal/platform/idempotency.Middleware` para mutacoes HTTP.
- `internal/budgets` (downstream): consome eventos `transactions.transaction.*` e `transactions.card_purchase.*` para atualizar consumo por categoria.

Consistência requerida:
híbrida

Justificativa de consistencia: dentro da tx SQL, `Transaction` ou `CardPurchase` + `outbox.Event` sao gravados atomicamente (consistencia forte local). A projecao `monthly_summary` e consumida eventualmente pelo consumer (consistencia eventual com janela ms-seg) + job diario de reconciliacao para fechar drift residual.

## Volumetria e Capacidade
Volume atual:
0 lancamentos hoje (modulo inexistente).

Pico esperado:
1.000 usuarios ativos x 100 lancamentos/mes => 100k linhas/mes em `transactions`. Cartao de credito com parcelamento medio 3x e penetracao 30% => 30.000 itens/mes em `card_invoice_items`. Pico diario: ~3.300 lancamentos/dia distribuidos; rajadas previsiveis em final de mes/recebimento de salario (~10k/dia, 60% concentrados em 4 horas = 700/h = ~12 req/s no caminho de write).

Taxa de crescimento:
Crescimento conservador de 20% MoM no primeiro semestre pos-lancamento. Apos 12 meses, tabela transactions com ~2M linhas, card_invoice_items com ~500k linhas - tamanhos administraveis sem particionamento.

SLO alvo:
- p99 write (`POST /v1/transactions`): < 300ms.
- p99 listagem (`GET /v1/months/{ref_month}/entries` cursor): < 200ms.
- p99 read summary (`GET /v1/months/{ref_month}`): < 100ms.
- Lag p95 do consumer `monthly_summary`: < 5s do commit ao update da projecao.
- Disponibilidade do caminho de escrita: 99.5% / mes (compativel com tier inicial).

Gargalos conhecidos:
- Bulk de parcelamento em compras com 24 parcelas: insere 24 itens + upsert de 24 faturas em uma tx; sob carga, latencia pode exceder 300ms. Mitigacao: insert em batch single SQL (multi-row INSERT) + indice de upsert focado.
- Consumer de `monthly_summary` em rajada de fim de mes pode acumular lag. Mitigacao: idempotencia por chave logica + processamento concorrente shardeado por `user_id`.
- Job mensal de recorrencia executa em horario fixo; com 1k usuarios e ate 5 templates/usuario, ate 5k materializacoes/mes. Tempo de execucao previsto < 60s; aceitavel.
- Listagem de extrato unificado (union all de `transactions` + `card_invoice_items` por `ref_month`) pode escalar mal sem indice cobrindo `ref_month`. Mitigacao: indice composto `(user_id, ref_month, created_at, id)` em ambas as tabelas; query usa `UNION ALL` + ORDER BY explorando indices.

## Segurança e Compliance
Classificação dos dados:
Dados pessoais sensiveis (financeiros): `amount_cents`, `description`, `card_id`, `purchase` historico. PII potencial em `description` (texto livre). Classificacao LGPD: "Dados pessoais comuns" com risco moderado (informacao financeira). Sem dados de saude, raca, orientacao sexual.

Autenticação e autorização:
- Autenticacao via principal middleware `InjectPrincipalFromHeader` (header `X-User-ID`); evolui para JWT/OIDC em release posterior do `internal/identity` sem mudancas no `internal/transactions`.
- Autorizacao: multi-tenant logico. Toda query SQL filtra por `user_id` extraido de `auth.FromContext(ctx)`. Use case rejeita request sem principal com `ErrPrincipalRequired -> 401`.
- Validacao de propriedade do `card_id`: use case chama `card_reader.GetByIDForUser(ctx, cardID, userID)` antes de criar `CardPurchase`. Falha retorna `ErrCardNotFound -> 404`.
- Validacao de propriedade da `category_id`: use case chama `category_reader.GetByID(ctx, categoryID)` e checa `category.UserID == principal.UserID` (ou se `internal/categories` for global, valida `kind`).
- Testes de integracao garantem isolacao tenant: cenarios com usuario A tentando ler/escrever recurso de usuario B retornam 404, nunca 403, para nao vazar existencia.

Gestão de segredos:
- Sem segredos especificos do modulo. DB credentials e tokens de observabilidade reutilizam segredos atuais do repositorio (Viper + env vars). Sem nova variavel sensivel adicionada.

Criptografia:
- Em transito: TLS no ingress padrao do repositorio.
- Em repouso: TLS + criptografia at-rest do RDS/PostgreSQL gerenciado (responsabilidade da infraestrutura). Sem `pgcrypto` no campo `description` no MVP (justificado pela decisao Q9 = Padrao); revisita em release futuro se LGPD exigir.

Auditoria e rastreabilidade:
- Trilha implicita via eventos do outbox: `transactions.transaction.*` e `transactions.card_purchase.*` preservam `event_id`, `aggregate_id`, `occurred_at`, `payload` por reten cao do outbox (configuravel; default 90 dias).
- `created_at`, `updated_at`, `deleted_at` em todas as tabelas.
- Soft-delete via `deleted_at` preserva historico para auditoria; queries excluem registros soft-deleted por padrao.

Compliance/LGPD:
- Direito de exclusao (LGPD Art.18-VI): cobertura via soft-delete + futuro endpoint `DELETE /v1/users/{user_id}/data` no `internal/identity` (fora deste discovery). Modulo nao bloqueia, apenas aplica soft-delete na requisicao.
- Direito de acesso/portabilidade: futuro endpoint de export no `internal/identity`. `internal/transactions` exporta JSON sob demanda via use case `ListByUser`.
- Logs nao registram `description`, `amount_cents`, `category_name_snapshot` (PII). Registram apenas IDs, `ref_month`, `payment_method`, `direction`, `category_id`, `trace_id`.

## Confiabilidade e Resiliência
SLA/SLO:
- SLO de write: 99.5% / mes.
- SLO de read: 99.9% / mes (caminho de leitura nao depende de outbox dispatcher).
- SLO de consumer lag: p95 < 5s.
- Error budget mensal: 3h36min de downtime aceito no write.

RTO/RPO:
- RTO: 1h (restauracao de backup mais recente do PostgreSQL).
- RPO: <= 15min (PITR do gerenciado). Outbox `event_id` UUID permite republicacao idempotente apos restore.

Estratégia de retry/idempotência:
- Mutacoes HTTP: `Idempotency-Key` middleware obrigatorio. Cache de status/body com TTL configuravel (default 24h). Replay devolve resposta cacheada se `request_hash` bater; retorna 409 se divergir.
- Optimistic locking: agregados mutaveis tem `version BIGINT`; update verifica `WHERE id=? AND version=?` e incrementa; conflito retorna `409 Conflict` com `code=conflict`.
- Eventos: outbox `event_id` UUID; consumidores tratam mensagem como at-least-once e deduplicam por `event_id` na tabela de processamento ou via natureza idempotente da operacao (RecomputeMonthlySummary e idempotente por (`user_id`, `ref_month`)).
- Recurring job: idempotencia logica `(template_id, ref_month)` em `recurring_materializations` PK.
- MonthlySummary recompute: idempotente por design (SELECT SUM + UPSERT).

Degradação/contingência:
- `card_reader` indisponivel: `POST /v1/card-purchases` falha rapidamente com `502 Bad Gateway` (`code=card_lookup_failed`) e cliente pode retentar. Nao usar default generico (decisao Q10).
- Outbox dispatcher off: writes continuam funcionando (evento fica enfileirado na tabela). Consumers de downstream (`internal/budgets`, recompute summary) ficam atrasados; alerta dispara quando consumer lag > 5s p95.
- Consumer `MonthlySummaryRecompute` em loop de erro: backoff exponencial 3s/9s/27s/81s; apos 5 tentativas evento e marcado como dead-letter (`status=dead`) e alerta `transactions_outbox_dead_letter_total > 0` dispara. Reprocessamento manual via runbook.
- Postgres indisponivel: caminho de write retorna 503; cliente reata. Health-check do modulo verifica `SELECT 1`.

Rollback:
- Rollback de feature: desligar `configs.TransactionsConfig.Enabled` (feature flag) -> handler nao registra rotas. Lancamentos existentes permanecem; nenhuma perda de dado.
- Rollback de codigo: revert PR + deploy. Migracoes sao backward-compatible (apenas ADD; sem DROP no MVP).
- Rollback de schema: migracoes ate `000018_create_transactions_baseline` tem `.down.sql` correspondente. Em emergencia, fluxo: feature flag off -> tabela cai por migration down em janela controlada.

## Observabilidade e Operação
Métricas:
- `transactions_transactions_created_total{payment_method, direction}` (counter).
- `transactions_card_purchases_created_total{installments_bucket}` (counter; buckets 1, 2-6, 7-12, 13-24).
- `transactions_recurring_template_created_total{frequency}` (counter).
- `transactions_write_duration_seconds{operation, status}` (histogram; operations: create_transaction, update_transaction, create_card_purchase, ...).
- `transactions_read_duration_seconds{operation}` (histogram).
- `transactions_monthly_summary_recompute_duration_seconds` (histogram).
- `transactions_monthly_summary_drift_total{direction}` (counter; incrementado pelo job de reconciliacao quando detecta divergencia).
- `transactions_outbox_consumer_lag_seconds` (gauge).
- `transactions_outbox_dead_letter_total{event_type}` (counter).
- `transactions_idempotency_replay_total` (counter; herdado do middleware).

Logs:
- Eventos estruturados via `observability.Logger`:
- `transactions.usecase.create_transaction.started/finished/failed` com `user_id`, `transaction_id`, `ref_month`, `payment_method`, `direction`, `category_id`, `trace_id`.
- `transactions.usecase.create_card_purchase.started/finished/failed` com `user_id`, `purchase_id`, `card_id`, `installments_total`, `category_id`.
- `transactions.consumer.monthly_summary_recompute.started/finished/failed` com `user_id`, `ref_month`, `event_id`.
- `transactions.job.recurring_materializer.started/finished/failed` com `templates_processed`, `materializations_created`.
- Logs NUNCA incluem `description`, `amount_cents`, `category_name_snapshot`.

Traces:
- Spans abertos via `o11y.Tracer().Start(ctx, "transactions.<layer>.<operation>")` em handlers, use cases, repositorios, consumers e jobs.
- `trace_id` propagado no `outbox.Event.Metadata["trace_id"]` para conectar publish -> consume.
- Attributes: `user_id`, `ref_month`, `payment_method`, `direction`, `installments_total`, `event_id` (sem PII).

Alertas:
- `transactions_monthly_summary_drift_total > 0` em 5min -> WARNING; > 10 em 1h -> CRITICAL.
- `histogram_quantile(0.99, transactions_write_duration_seconds_bucket) > 0.3s` por 10min -> WARNING.
- `transactions_outbox_consumer_lag_seconds > 5` p95 por 5min -> WARNING; > 30 -> CRITICAL.
- `transactions_outbox_dead_letter_total > 0` -> CRITICAL imediato com runbook.

Dashboards/Runbooks:
- Dashboard Grafana `transactions-overview` com paineis: write/read RED por operacao, idempotency replay rate, summary drift, consumer lag, dead-letter, top categorias por volume (sem PII).
- Runbook `runbooks/transactions/outbox-dead-letter.md`: passo a passo para inspecionar `platform.outbox` (WHERE status='dead'), corrigir causa, reprocessar via `outbox-cli requeue --event-id ...`.
- Runbook `runbooks/transactions/summary-drift.md`: passo a passo para identificar `(user_id, ref_month)` em drift e forcar recompute via job ad-hoc.

## Performance e Escalabilidade
Latência alvo:
- p50/p95/p99 write: 80ms / 180ms / 300ms.
- p50/p95/p99 read summary: 20ms / 60ms / 100ms.
- p50/p95/p99 list entries (cursor 50 itens): 40ms / 120ms / 200ms.
- p95 consumer lag: < 5s.

Estratégia de escala:
- Postgres unico no MVP com leitura/escrita no mesmo nó.
- Indices compostos cobertos: `(user_id, ref_month, created_at, id)` em `transactions` e `card_invoice_items`; `(user_id, card_id, ref_month) UNIQUE` em `card_invoices`.
- Consumer do outbox com worker pool concorrente, sharded por `user_id mod N`.
- Insert em batch para parcelamento (multi-row INSERT).
- Hot keys (rajadas em fim de mes): sem mitigacao no MVP; observar via metricas e considerar fila de prioridade em v2.

Limites conhecidos:
- 1 instancia de Postgres; sem replica de leitura no MVP.
- Sem cache de leitura de `monthly_summary`; leitura sempre via Postgres (latencia ok ate 10k usuarios).
- Job de recorrencia roda em single node; em escala maior precisa distribuir por particao de `user_id`.
- Sem rate-limit dedicado em `POST /v1/card-purchases`; relies em rate-limit global do gateway.

Teste de carga:
- Teste de carga em homologacao antes do go-live: 100 RPS sustentados em `POST /v1/transactions` por 10min; pico 300 RPS por 1min em `POST /v1/card-purchases` com 24 parcelas para validar latencia. Ferramenta: k6 (consistente com pratica do repositorio - confirmar em techspec).
- Teste de soak: 24h em rate medio com observabilidade ligada para verificar memory leak.
- Teste de reconciliacao: simular 1k drifts forcados e validar que job diario corrige todos em uma execucao.

## Custos e Orçamento
Orçamento estimado:
Custo de implementacao (engenharia): aproximadamente 10-12 semanas de 1 engenheiro Go senior (analogo a entrega recente do `internal/budgets` MVP com escopo similar). Custo de infra incremental no MVP: residual (mesmo Postgres, mesma observabilidade, mesma malha HTTP). Sem nova infraestrutura paga.

Drivers de custo:
- Engenharia: implementacao + testes de integracao + dashboards + runbooks.
- Postgres: armazenamento incremental para 6 tabelas (~2GB no primeiro ano com volumetria MVP).
- Observabilidade: cardinalidade de metricas (controlada via labels). Logs estruturados sem PII (volume baixo).
- Outbox: linhas adicionais em `platform.outbox` (~6 eventos/lancamento de credito; ~1 evento/lancamento avulso). Estimado 200k-400k linhas/mes; reten cao 90 dias.

Guardrails de custo:
- Cardinalidade de metricas: labels limitados a enum-like (direction, payment_method, status); sem `user_id` em label.
- Logs sem PII evitam aumento de custo de log shipping/retencao.
- Outbox retention configuravel (`OutboxConfig`); compactacao apos 30 dias se necessario.
- Sem nova feature paga de observabilidade (mesma stack atual).

Plano de otimização:
- Em 6 meses pos-MVP, avaliar particionamento por `ref_month` se volume crescer 5x.
- Em 12 meses pos-MVP, avaliar view materializada para extrato unificado se p99 list > 200ms.
- Em escala 10x, mover consumer para nó dedicado e adicionar replica de leitura para `GET /v1/months/{ref_month}/entries`.

## Riscos e Mitigações
- Risco: Drift entre `monthly_summary` e `SUM(transactions)+SUM(card_invoice_items)` por falha do consumer ou ordem de eventos.
  Impacto: Total exibido ao usuario diverge da soma real; decisoes financeiras incorretas.
  Mitigação: Consumer idempotente por (`user_id`, `ref_month`); job diario `MonthlySummaryReconcilerJob` recalcula e emite metrica `monthly_summary_drift_total`; alerta WARN > 0.
  Dono: time `internal/transactions`.
- Risco: Edicao retroativa em fatura fechada confunde o usuario.
  Impacto: divergencia entre fatura real do banco e fatura simulada no app.
  Mitigação: aviso UX antes do save; backlog para v2 com imutabilidade pos-pagamento; trilha de auditoria via eventos.
  Dono: produto.
- Risco: Snapshot de `BillingCycle` do `internal/card` fica desatualizado se usuario alterar `closing_day`/`due_day` depois.
  Impacto: lancamentos futuros calculam competencia certa, mas itens existentes nao retroagem.
  Mitigação: documentar comportamento no OpenAPI; v2 pode consumir evento `card.updated.v1` para reabrir politica.
  Dono: time `internal/transactions`.
- Risco: Parcelamento em 24 parcelas exibe latencia acima do SLO em rajadas.
  Impacto: p99 write excede 300ms.
  Mitigação: multi-row INSERT; observar com `transactions_write_duration_seconds`; particionar consumer se persistir.
  Dono: time `internal/transactions`.
- Risco: Job de recorrencia duplica materializacao em re-execucao.
  Impacto: lancamentos duplicados no mes.
  Mitigação: tabela `recurring_materializations(template_id, ref_month) PRIMARY KEY` garante idempotencia.
  Dono: time `internal/transactions`.
- Risco: Volume de eventos no outbox cresce alem do previsto.
  Impacto: latencia de dispatcher e custo de storage.
  Mitigação: bulk-create de itens da mesma compra emite UM evento `transactions.card_purchase.created.v1` (nao N); retencao com compactacao apos 30 dias.
  Dono: plataforma.
- Risco: Categorias podem ser excluidas em `internal/categories`, deixando snapshot orfao.
  Impacto: relatorio mostra "Categoria removida".
  Mitigação: snapshot do nome ja registrado; UX exibe snapshot quando categoria nao existe mais.
  Dono: produto.

## Trade-offs e Decisões
Alternativas consideradas:
- Alternativa 1 (brainstorming): tabela unica `transactions` - descartada por integridade fraca de parcelamento.
- Alternativa 2 (brainstorming, recomendada): `Transaction` + `CardPurchase` + `CardInvoice` + `CardInvoiceItem` + `MonthlyLedger` projection.
- Alternativa 3 (brainstorming): LedgerEntry append-only com projetores - descartada por overkill no MVP.
- Alternativa 4 (brainstorming): `transactions` + `monthly_summary` sem entidade Fatura - descartada por perda de expressividade.
- Decisao adicional do discovery: incluir `RecurringTemplate` DENTRO do MVP (vs deixar para v2) - aceito pelo usuario.
- Decisao adicional do discovery: snapshot estatico de `BillingCycle` (vs consumer reativo de `card.updated.v1`) - aceito.
- Decisao adicional do discovery: `monthly_summary` eventual via consumer + reconciliacao diaria (vs forte na mesma tx vs sob demanda) - aceito.
- Decisao adicional do discovery: tabelas separadas (vs tabela unica com discriminator vs separadas + view materializada) - aceito.

Decisão tomada:
Implementar `internal/transactions` com Alternativa 2 estendida por `RecurringTemplate` + job mensal. Persistencia em 6 tabelas dedicadas. Projecao `monthly_summary` eventual via consumer reativo + job diario de reconciliacao. Snapshot de `BillingCycle` na criacao. OTel completo + 4 alertas operacionais. Rollout big-bang com feature flag. Categoria via FK + snapshot do nome.

Trade-off aceito:
- Edicao retroativa em fatura fechada (sem imutabilidade pos-pagamento no MVP).
- Snapshot estatico de cartao (mudancas posteriores nao retroagem).
- Consistencia eventual da projecao (janela ms-seg + reconciliacao diaria).
- Aumento de escopo do MVP em ~2-3 semanas para incluir `RecurringTemplate`.
- DOC apenas leitura legada (rejeitado em creates).
- Sem criptografia em repouso de `description` no MVP.
- Sem RLS no Postgres; isolacao por filtro aplicacional.

## Plano de Entrega e Rollout
Fases:
- Fase 0 (semana 1): Schema SQL + migrations + esqueleto do modulo + wiring no `cmd/api` atras de feature flag desligada. Smoke test em homologacao.
- Fase 1 (semanas 2-3): Agregado `Transaction` + use cases CRUD + handlers REST + outbox producer + testes unitarios e de integracao. Habilitar em homologacao com feature flag ligada para QA interno.
- Fase 2 (semanas 4-5): Agregado `CardPurchase` + `CardInvoice` + `CardInvoiceItem` + use cases CRUD + parcelamento + integracao com `card_reader` + testes. QA cross-modulo com `internal/budgets`.
- Fase 3 (semanas 6-7): Consumer `MonthlySummaryRecompute` + projecao `monthly_summary` + endpoint `GET /v1/months/{ref_month}` + job diario de reconciliacao.
- Fase 4 (semanas 8-9): Agregado `RecurringTemplate` + job mensal de materializacao + testes idempotencia.
- Fase 5 (semanas 10-11): OTel completo + dashboard Grafana + alertas + runbooks + teste de carga em homologacao.
- Fase 6 (semana 12): Go-live em producao com feature flag ligada; observabilidade 24/7 pelas primeiras 2 semanas.

Migração:
Migracao de dados nao se aplica (modulo novo, sem dados legados). Endpoint de import legado pode ser oferecido futuramente para aceitar `payment_method=doc` em registros importados antes de 2024.

Feature flags/canary:
- Feature flag `configs.TransactionsConfig.Enabled` (bool) controla registro das rotas REST e ativacao dos consumers/jobs.
- Sem canary por porcentagem no MVP (single-tenant pequeno). Big-bang controlado: liga em homologacao por 1 semana antes de producao.
- Observabilidade monitora 4 alertas nas primeiras 48h pos-go-live.

Critério de rollback:
- Rollback automatico nao implementado; rollback manual via feature flag.
- Gatilhos para flag off: alerta CRITICAL em `transactions_monthly_summary_drift_total`, `transactions_outbox_dead_letter_total` ou erro 5xx sustentado > 1% por 10min em qualquer rota `/v1/transactions*`, `/v1/card-purchases*`, `/v1/months/*`.
- Procedimento: alterar config + reload do `cmd/api`; tempo de rollback < 5min.

## Decomposição em Épicos e Features
### Epic 01 - Esqueleto e Schema do Módulo
Objetivo: criar a estrutura do modulo `internal/transactions` aderente ao Padrao Obrigatorio de Modulo, com migrations, wiring de configuracao e feature flag desligada.
Feature 01: Layout de pastas e arquivos base (domain, application, infrastructure, module.go) com pacotes vazios e contratos de governanca atendidos (R0, R-ADAPTER-001.1).
Feature 02: Migration `000018_create_transactions_baseline.up.sql` com tabelas transactions, card_purchases, card_invoices, card_invoice_items, recurring_templates, recurring_materializations, monthly_summary; .down.sql correspondente.
Feature 03: `configs.TransactionsConfig` (Enabled, IdempotencyTTL, RecurringMaterializeCron, MonthlySummaryReconcileCron) + leitura via Viper + injecao em `cmd/api`.

### Epic 02 - CRUD de Lancamentos Avulsos (Transaction)
Objetivo: entregar CRUD completo de `Transaction` com idempotency, version, eventos e testes.
Feature 01: Agregado `Transaction` (entity + VOs) com factory function e regras de invariante.
Feature 02: Repository Postgres para `transactions` com optimistic locking e soft-delete.
Feature 03: Use cases `CreateTransaction`, `UpdateTransaction`, `DeleteTransaction`, `ListTransactions`, `GetTransaction` com Command Object pattern.
Feature 04: Handlers REST `POST/GET/PATCH/DELETE /v1/transactions` + router com `InjectPrincipalFromHeader` + `idempotency.Middleware`.
Feature 05: Producer de eventos `transactions.transaction.{created,updated,deleted}.v1` via `outbox.Publisher`.
Feature 06: Suite unitaria (mockery + testify/suite) + suite de integracao (testcontainers Postgres) para todos os use cases e handlers.

### Epic 03 - Compra de Cartao com Fatura e Parcelamento (CardPurchase + CardInvoice + CardInvoiceItem)
Objetivo: entregar simulacao de fatura com parcelamento e integracao com `internal/card`.
Feature 01: Agregado `CardPurchase` + `CardInvoice` + `CardInvoiceItem` com factory function e regras de invariante (1..24 parcelas, soma de items == total_amount_cents).
Feature 02: Domain service `CompetenceCalculator` que recebe `purchased_at` + `BillingCycle` snapshot e retorna primeira `ref_month` + array de `ref_month` das parcelas seguintes.
Feature 03: Interface `card_reader` em `application/interfaces` + adapter para `internal/card.CardRepository.GetByIDForUser`.
Feature 04: Use cases `CreateCardPurchase`, `UpdateCardPurchase`, `DeleteCardPurchase`, `ListCardPurchases`, `GetCardPurchase`, `GetInvoice` com cascateamento em update/delete.
Feature 05: Repository Postgres com insert em batch para parcelas + upsert idempotente de `card_invoices`.
Feature 06: Handlers REST `/v1/card-purchases` + `/v1/cards/{card_id}/invoices/{ref_month}`.
Feature 07: Producer de eventos `transactions.card_purchase.{created,updated,deleted}.v1` com payload contendo array de items.
Feature 08: Suites de testes unitarios + integracao cobrindo cenarios de parcelamento, edicao retroativa e card_reader indisponivel.

### Epic 04 - Projecao Mensal (monthly_summary) e Reconciliacao
Objetivo: manter projecao eventual consistente via consumer + job diario.
Feature 01: Agregado/projecao `MonthlySummary` + repository com UPSERT idempotente.
Feature 02: Use case `RecomputeMonthlySummary(user_id, ref_month)` que recalcula `income/outcome/total` por SUM.
Feature 03: Consumer `MonthlySummaryRecomputeConsumer` que reage a eventos `transactions.transaction.*` e `transactions.card_purchase.*`.
Feature 04: Job diario `MonthlySummaryReconcilerJob` que detecta drift e emite metrica/log.
Feature 05: Endpoint `GET /v1/months/{ref_month}` com resumo e `GET /v1/months/{ref_month}/entries` com extrato unificado paginado (UNION ALL de transactions + card_invoice_items).
Feature 06: Suites de testes cobrindo idempotencia do consumer e detecccao de drift.

### Epic 05 - Lancamento Recorrente (RecurringTemplate)
Objetivo: automatizar lancamentos recorrentes mensais/anuais com idempotencia.
Feature 01: Agregado `RecurringTemplate` com VOs (`Frequency`, `DayOfMonth 1..28`).
Feature 02: Repository Postgres + tabela auxiliar `recurring_materializations` para idempotencia.
Feature 03: Use cases `CreateRecurringTemplate`, `UpdateRecurringTemplate`, `DeleteRecurringTemplate`, `ListRecurringTemplates`, `MaterializeRecurringMonth`.
Feature 04: Handlers REST `/v1/recurring-templates`.
Feature 05: Job mensal `RecurringMaterializerJob` que percorre templates ativos e materializa lancamento ou compra de cartao.
Feature 06: Producer de eventos `transactions.recurring_template.{created,updated,deleted}.v1`.
Feature 07: Suites de testes cobrindo idempotencia, edge cases (fim de mes, template com `ended_at`).

### Epic 06 - Observabilidade, Operacao e Go-Live
Objetivo: instrumentar OTel, dashboards, alertas e runbooks; conduzir teste de carga e go-live com feature flag.
Feature 01: Spans OTel em handlers/use cases/repositories/consumers/jobs com naming `transactions.<layer>.<operation>`.
Feature 02: Metricas Prometheus listadas (RED + drift + lag + dead-letter) com cardinalidade controlada.
Feature 03: Logs estruturados sem PII de descricao/amount.
Feature 04: Dashboard Grafana `transactions-overview`.
Feature 05: Alertas (4) configurados com PagerDuty/Slack.
Feature 06: Runbooks `outbox-dead-letter` e `summary-drift`.
Feature 07: Teste de carga em homologacao (100 RPS write sustentado, pico 300 RPS, soak 24h).
Feature 08: Go-live com feature flag + monitoramento dedicado nas primeiras 2 semanas.

## Itens em Aberto
- Confirmar nome final do scope para idempotency (`transactions` vs `internal-transactions`) em techspec.
- Confirmar formato exato de `event_type` (`transactions.transaction.created.v1`) com convencao explicita do `internal/budgets` (`budgets.expense.committed.v1`); manter o mesmo padrao verbo no participio se for o caso (`created` vs `commit`); ajustar em techspec.
- Validar com produto a UX de aviso ao editar fatura fechada (modal de confirmacao ou nota inline).
- Decidir se job de recorrencia roda no dia 1 de cada mes ou no `day_of_month` de cada template (impacto operacional).
- Definir politica de retencao do `outbox` exclusivamente para eventos `transactions.*` se diferente do default global.
- Confirmar se rate-limit por usuario em `POST /v1/card-purchases` e necessario (suspeita de abuso com 24 parcelas).
- Confirmar se export de dados LGPD sera do `internal/identity` ou per-modulo (impacto: novo use case `ExportUserData` no `internal/transactions`).
- Confirmar disponibilidade de k6 no pipeline de homologacao para teste de carga.
