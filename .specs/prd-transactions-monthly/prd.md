# Documento de Requisitos do Produto (PRD)

<!-- spec-version: 1 -->

<!--
Histórico de versões:
- v1 (2026-06-11): versão inicial derivada do brainstorming decisório (`.agents/skills/decision-brainstorming/discoveries/brainstorm-modulo-lancamentos-mensais-income-outcome/`) e do discovery técnico (`.agents/skills/technical-discovery-production/discoveries/technical-modulo-lancamentos-mensais-income-outcome/`), ambos validados com `SUCCESS`.
-->

> **Origem**: brainstorming decisório em `.agents/skills/decision-brainstorming/discoveries/brainstorm-modulo-lancamentos-mensais-income-outcome/` e discovery técnico em `.agents/skills/technical-discovery-production/discoveries/technical-modulo-lancamentos-mensais-income-outcome/`, ambos validados em 2026-06-11.

## Visão Geral

O `mecontrola` ainda não possui uma fonte canônica de movimentações financeiras por usuário e mês de referência. Sem ela, `internal/budgets` não recebe consumo real por categoria e o usuário não consegue acompanhar caixa diário — o que bloqueia o uso prático do app.

Este PRD define o MVP production-ready do módulo **internal/transactions**: CRUD completo de lançamentos avulsos (entrada/saída) cobrindo todas as formas de pagamento vigentes no mercado brasileiro em 2026 (PIX, TED, débito em conta, débito em cartão, dinheiro, boleto), além de compras de cartão de crédito que simulam fatura com competência derivada das datas de fechamento e vencimento do cartão do `internal/card` e suportam parcelamento em até 24 vezes. Lançamentos recorrentes (salário mensal, assinatura, prestação fixa) são automatizados via templates materializados por job mensal idempotente.

Cada lançamento atualiza os agregados `income`, `outcome` e `total = income - outcome` por (`user_id`, `ref_month`) através da projeção `monthly_summary`, mantida por consumer reativo do `outbox` com reconciliação diária. Cada operação publica eventos de domínio (`transactions.transaction.*`, `transactions.card_purchase.*`, `transactions.recurring_template.*`) consumidos por `internal/budgets` para atualizar consumo por categoria sem acoplamento síncrono.

O MVP é inegociável em **robustez sem comprometer prazo**: idempotência por header obrigatória, optimistic locking por `version`, outbox at-least-once com DLQ lógica, observabilidade OTel completa, testes de integração com testcontainers e rollout big-bang controlado por feature flag `transactions.enabled`. Edição retroativa em faturas fechadas, snapshot estático do `BillingCycle` do cartão e consistência eventual da projeção mensal são trade-offs explicitamente aceitos.

## Objetivos

- **OBJ-01**: Permitir o registro, edição, consulta paginada e exclusão lógica de lançamentos financeiros do usuário (entrada e saída) com idempotência ponta-a-ponta.
- **OBJ-02**: Apresentar para cada competência mensal (`ref_month` no formato `YYYY-MM`) os totais `income`, `outcome` e `total` corretos após cada operação, com janela máxima de consistência eventual de 5 segundos no p95.
- **OBJ-03**: Simular fatura de cartão de crédito com competência derivada das datas oficiais do cartão (`closing_day`/`due_day`) e suportar parcelamento em 1 a 24 vezes, preservando integridade entre compra-pai e parcelas.
- **OBJ-04**: Automatizar lançamentos recorrentes mensais e anuais via templates materializados por job idempotente, sem duplicação em re-execuções.
- **OBJ-05**: Disparar eventos de domínio sobre cada mutação financeira para que `internal/budgets` (e consumidores futuros) reaja sem acoplamento síncrono.
- **OBJ-06**: Atender baseline LGPD para dados financeiros: isolar logicamente por `user_id` em toda query SQL, jamais registrar PII de descrição/valor em logs e preservar trilha auditável de mutação via eventos persistidos no outbox.
- **OBJ-07**: Suportar volumetria base de 1.000 usuários × 100 lançamentos/mês com SLO p99 write < 300 ms, listagem cursor < 200 ms, resumo mensal < 100 ms.

## Histórias de Usuário

- Como **pessoa física controlando suas finanças**, quero registrar um gasto à vista (PIX, débito, dinheiro) atribuindo descrição, valor, categoria, subcategoria opcional e data, para que o total do mês corrente reflita imediatamente a saída e meu budget por categoria seja atualizado.
- Como **pessoa física**, quero registrar uma compra parcelada no cartão de crédito informando o cartão, o número de parcelas e o valor total, para que o sistema crie automaticamente as parcelas nas faturas dos meses corretos com base na data de fechamento e vencimento do cartão.
- Como **pessoa física**, quero editar totalmente um lançamento avulso (descrição, valor, categoria, subcategoria, tipo entrada/saída e forma de pagamento), para corrigir erros sem precisar apagar e recriar.
- Como **pessoa física**, quero editar uma compra parcelada e ver todas as parcelas atualizadas em cascata, para refletir o ajuste em todas as faturas afetadas.
- Como **pessoa física**, quero excluir logicamente um lançamento ou uma compra parcelada, para corrigir um lançamento equivocado sem deixar resíduo no resumo mensal.
- Como **pessoa física**, quero listar lançamentos de uma competência paginados por cursor, para revisar o extrato sem latência perceptível mesmo em meses cheios.
- Como **pessoa física**, quero consultar `income`, `outcome` e `total` de qualquer mês, para entender meu fluxo de caixa do período.
- Como **pessoa física**, quero cadastrar um lançamento recorrente (salário, mensalidade) com frequência mensal ou anual, dia do mês e data de início/fim opcional, para que o sistema crie o lançamento automaticamente todos os meses sem ação manual.
- Como **operador do módulo `internal/budgets`**, quero receber eventos `transactions.transaction.created`, `transactions.card_purchase.created` e equivalentes de update/delete via `outbox`, para atualizar consumo por categoria sem acoplamento síncrono com o caminho de escrita do usuário.
- Como **operador do MVP**, quero poder desligar o módulo via feature flag `transactions.enabled` em < 5 minutos, para conter incidente sem dependência de revert/deploy.

## Funcionalidades Core

1. **Lançamento avulso (Transaction)**: cria, edita, consulta, lista e exclui logicamente um lançamento de income ou outcome com forma de pagamento `pix`, `ted`, `debit_in_account`, `debit_card`, `cash` ou `boleto`. A forma de pagamento `doc` é aceita apenas em registros legados (somente leitura); criação ou edição com `doc` é rejeitada por validação. Cada mutação atualiza a projeção mensal e dispara evento.
2. **Compra de cartão de crédito com fatura simulada (CardPurchase + CardInvoice + CardInvoiceItem)**: cria uma compra parcelada (1 a 24 vezes) que materializa N parcelas em faturas mensais (`CardInvoice` por `card_id`/`ref_month`). Competência é calculada a partir de snapshot estático de `closing_day`/`due_day` lido do `internal/card` no momento da criação. Edição da compra cascateia em todas as parcelas, inclusive em faturas fechadas.
3. **Resumo mensal (MonthlySummary)**: para qualquer `ref_month`, expõe `income_cents`, `outcome_cents`, `total_cents` materializados por consumer reativo do `outbox`. Job diário de reconciliação detecta e corrige drift contra `SUM(transactions) + SUM(card_invoice_items)`.
4. **Extrato mensal unificado**: lista paginada por cursor combinando lançamentos avulsos e itens de fatura do mês, ordenada por `created_at` decrescente. Reflete eventos materializados de recorrência.
5. **Lançamento recorrente (RecurringTemplate)**: cadastra, edita, consulta e exclui templates de lançamento recorrente com frequência `monthly` ou `yearly`, `day_of_month` (1 a 28), `started_at`, `ended_at` opcional e `installments_total` (para crédito recorrente). Job mensal materializa de forma idempotente garantindo zero duplicação por `(template_id, ref_month)`.
6. **Eventos de domínio via outbox**: publica `transactions.transaction.{created|updated|deleted}.v1`, `transactions.card_purchase.{created|updated|deleted}.v1` e `transactions.recurring_template.{created|updated|deleted}.v1` na mesma transação SQL da mutação, com `event_id` UUID e `trace_id` propagado em `metadata`.
7. **Idempotência e controle de concorrência**: toda rota de mutação exige header `Idempotency-Key` (escopo `transactions`) e todo agregado tem coluna `version BIGINT` para optimistic locking; conflito retorna `409`.
8. **Observabilidade operacional**: spans OTel `transactions.<layer>.<operation>`, métricas Prometheus RED + drift + consumer lag + dead-letter, logs estruturados sem PII de descrição/valor, dashboard Grafana `transactions-overview` e 4 alertas (drift, write p99, consumer lag, dead-letter).
9. **Rollout controlado**: feature flag `configs.TransactionsConfig.Enabled` controla registro de rotas, consumers e jobs; rollback em < 5 minutos via flag off.

## Requisitos Funcionais

### Lançamentos avulsos (Transaction)

- **RF-01**: O sistema deve oferecer `POST /v1/transactions` que cria um lançamento avulso autenticado pelo `auth.Principal` injetado pelo `EstablishPrincipal` do `internal/identity`.
- **RF-02**: O payload de criação deve aceitar `direction` (entrada/saída), `payment_method` (`pix`, `ted`, `debit_in_account`, `debit_card`, `cash`, `boleto`), `amount_cents` (>0), `description`, `category_id`, `subcategory_id` opcional e `occurred_at`. A criação deve rejeitar `payment_method=doc` com `400 validation_error`.
- **RF-03**: O sistema deve calcular `ref_month` como `YYYY-MM` em fuso `America/Sao_Paulo` a partir de `occurred_at` e persisti-lo junto ao lançamento.
- **RF-04**: O sistema deve validar a existência da categoria e da subcategoria (quando informada) chamando `internal/categories`, gravar o `category_id`/`subcategory_id` como FK e gravar `category_name_snapshot` e `subcategory_name_snapshot` para preservar histórico mesmo se a categoria for renomeada ou removida.
- **RF-05**: O sistema deve oferecer `PATCH /v1/transactions/{id}` que permite editar totalmente `direction`, `amount_cents`, `description`, `category_id`, `subcategory_id`, `payment_method` e `occurred_at` de um lançamento avulso, atualizando `ref_month` quando `occurred_at` mudar de mês.
- **RF-06**: O sistema deve oferecer `GET /v1/transactions/{id}` que retorna o lançamento avulso pelo seu identificador, verificando propriedade pelo `user_id` do principal; recurso de outro usuário retorna `404 not_found`.
- **RF-07**: O sistema deve oferecer `GET /v1/transactions?ref_month=YYYY-MM&cursor=&limit=` que lista lançamentos avulsos do usuário no mês informado, paginado por cursor base64(`created_at`,`id`) descendente, limit padrão 50, máximo 200.
- **RF-08**: O sistema deve oferecer `DELETE /v1/transactions/{id}` que aplica soft-delete (`deleted_at = now`), preservando trilha de auditoria.
- **RF-09**: Toda rota de mutação (`POST`, `PATCH`, `DELETE`) deve exigir header `Idempotency-Key` com escopo `transactions`; replay com mesmo hash retorna a resposta cacheada; replay com hash divergente retorna `409 idempotency_conflict`.
- **RF-10**: Toda atualização deve aplicar optimistic locking pela coluna `version BIGINT`; conflito retorna `409 conflict` com `code=transaction_version_conflict`.

### Compra de cartão de crédito (CardPurchase + CardInvoice + CardInvoiceItem)

- **RF-11**: O sistema deve oferecer `POST /v1/card-purchases` que cria uma compra parcelada de cartão de crédito com `card_id`, `total_amount_cents` (>0), `installments_total` (1 a 24), `description`, `category_id`, `subcategory_id` opcional, `purchased_at`.
- **RF-12**: O sistema deve consultar `internal/card.CardRepository.GetByIDForUser(ctx, card_id, user_id)` e gravar `card_closing_day` e `card_due_day` como snapshot estático na própria `CardPurchase`. Mudanças posteriores nas datas do cartão **não** retroagem em compras já registradas.
- **RF-13**: Cartão inexistente ou pertencente a outro usuário deve retornar `404 card_not_found`. Falha de comunicação com `internal/card` deve retornar `502 card_lookup_failed` (sem default genérico de datas).
- **RF-14**: O sistema deve calcular a competência da primeira parcela como segue: se `purchased_at.day <= card_closing_day`, a primeira fatura é o mês corrente vencendo no `card_due_day` do mês corrente; senão, a próxima fatura. As parcelas seguintes incrementam um mês cada.
- **RF-15**: O sistema deve dividir `total_amount_cents` em `installments_total` parcelas com soma exatamente igual ao total (centavos residuais distribuídos nas primeiras parcelas em ordem determinística).
- **RF-16**: Para cada parcela, o sistema deve garantir a existência de `CardInvoice` por (`user_id`, `card_id`, `ref_month`) via upsert idempotente (constraint `UNIQUE(user_id, card_id, ref_month)`) e gravar o `CardInvoiceItem` ligado via `purchase_id` e `invoice_id`.
- **RF-17**: O sistema deve oferecer `PATCH /v1/card-purchases/{id}` que permite editar `total_amount_cents`, `installments_total`, `description`, `category_id`, `subcategory_id`, `purchased_at`. A edição deve recriar/atualizar todas as parcelas em cascata, inclusive em faturas já fechadas, dentro da mesma transação.
- **RF-18**: O sistema deve oferecer `DELETE /v1/card-purchases/{id}` que aplica soft-delete na compra-pai e em todas as parcelas vinculadas em uma única transação.
- **RF-19**: O sistema deve oferecer `GET /v1/card-purchases/{id}` que retorna a compra com a lista de parcelas (`installment_index`, `ref_month`, `amount_cents`, `invoice_id`).
- **RF-20**: O sistema deve oferecer `GET /v1/card-purchases?card_id=&ref_month=&cursor=&limit=` que lista compras paginadas por cursor.
- **RF-21**: O sistema deve oferecer `GET /v1/cards/{card_id}/invoices/{ref_month}` que retorna a fatura simulada do cartão no mês informado com `items_total_cents`, `closing_at`, `due_at` e a lista de itens.
- **RF-22**: Idempotência (`Idempotency-Key`) e optimistic locking (`version`) aplicam-se a todas as mutações de `card-purchases` e respondem com os mesmos códigos da RF-09 e RF-10.

### Resumo mensal e extrato unificado (MonthlySummary)

- **RF-23**: O sistema deve oferecer `GET /v1/months/{ref_month}` que retorna `income_cents`, `outcome_cents`, `total_cents` e `updated_at` da projeção `monthly_summary` para o usuário autenticado.
- **RF-24**: O sistema deve oferecer `GET /v1/months/{ref_month}/entries?cursor=&limit=` que lista o extrato unificado do mês combinando lançamentos avulsos e itens de fatura, paginado por cursor base64(`created_at`,`id`) descendente.
- **RF-25**: Toda mutação confirmada (commit) em `Transaction`, `CardPurchase` ou `CardInvoiceItem` deve causar recálculo eventual da projeção `monthly_summary` para todas as competências afetadas (a anterior e a nova, quando `ref_month` mudar).
- **RF-26**: O consumer de recálculo deve ser idempotente: reprocessar o mesmo evento ou disparar recálculo paralelo da mesma chave (`user_id`, `ref_month`) não pode corromper `income_cents`, `outcome_cents` ou `total_cents`.
- **RF-27**: Um job diário de reconciliação deve recalcular `SUM(transactions) + SUM(card_invoice_items)` por usuário/mês com atividade nas últimas 48 h e comparar com `monthly_summary`; divergências devem incrementar `transactions_monthly_summary_drift_total` e corrigir a projeção.
- **RF-28**: Caso `monthly_summary` ainda não tenha sido projetado, `GET /v1/months/{ref_month}` deve responder `200` com totais zerados e `updated_at=null`, nunca `404`.

### Lançamento recorrente (RecurringTemplate)

- **RF-29**: O sistema deve oferecer `POST /v1/recurring-templates` que cria um template recorrente com `direction`, `payment_method`, `card_id` opcional (obrigatório quando `payment_method=credit_card`), `amount_cents`, `description`, `category_id`, `subcategory_id` opcional, `frequency` (`monthly`/`yearly`), `day_of_month` (1 a 28), `started_at`, `ended_at` opcional, `installments_total` (≥1, ≤24, default 1).
- **RF-30**: O sistema deve oferecer `PATCH /v1/recurring-templates/{id}`, `DELETE /v1/recurring-templates/{id}`, `GET /v1/recurring-templates/{id}` e `GET /v1/recurring-templates?cursor=&limit=&active=true|false`.
- **RF-31**: Um job mensal `RecurringMaterializerJob` deve materializar todos os templates ativos (sem `ended_at` ou com `ended_at >= today`) no mês corrente, criando o `Transaction` ou `CardPurchase` correspondente.
- **RF-32**: A materialização deve ser idempotente: a tabela `recurring_materializations` deve usar `(template_id, ref_month)` como `PRIMARY KEY` para garantir zero duplicação em re-execuções do job.
- **RF-33**: Materializações de templates de crédito devem chamar o caminho de criação de `CardPurchase` (RF-11 a RF-16) com o `card_id` do template, herdando snapshot de `BillingCycle`.
- **RF-34**: Edição de template recorrente afeta apenas materializações futuras; ocorrências já materializadas só podem ser alteradas via edição direta do `Transaction` ou `CardPurchase` correspondente.

### Eventos de domínio via outbox

- **RF-35**: Toda criação, edição e exclusão lógica de `Transaction` deve gravar evento `transactions.transaction.{created|updated|deleted}.v1` em `platform.outbox` na mesma transação SQL do agregado.
- **RF-36**: Toda criação, edição e exclusão lógica de `CardPurchase` deve gravar UM ÚNICO evento `transactions.card_purchase.{created|updated|deleted}.v1` contendo o array completo de parcelas no payload, na mesma transação do agregado.
- **RF-37**: Toda criação, edição e exclusão lógica de `RecurringTemplate` deve gravar evento `transactions.recurring_template.{created|updated|deleted}.v1` na mesma transação do agregado.
- **RF-38**: O `event_id` deve ser UUID gerado pelo módulo; `aggregate_type` deve identificar o agregado (`transactions.transaction`, `transactions.card_purchase`, `transactions.recurring_template`); `aggregate_id` deve ser o id do agregado; `occurred_at` deve ser `time.Now().UTC()` no commit; `metadata.trace_id` deve carregar o `trace_id` do span ativo.
- **RF-39**: O consumer `MonthlySummaryRecomputeConsumer` deve consumir eventos `transactions.transaction.*` e `transactions.card_purchase.*` e disparar `RecomputeMonthlySummary(user_id, ref_month)` para cada competência afetada.
- **RF-40**: Após o limite configurado de tentativas (`OutboxConfig.RetryMaxAttempts`), o evento deve ser marcado como dead-letter e disparar alerta `transactions_outbox_dead_letter_total > 0`.

### Autenticação, autorização e isolamento

- **RF-41**: Toda rota deve ser autenticada via `RequireUser` canônico do `internal/identity` (auth.Principal injetado por `EstablishPrincipal`).
- **RF-42**: Toda query SQL do módulo deve filtrar por `user_id` derivado do `auth.Principal`; tentativa de leitura ou mutação de recurso de outro usuário deve retornar `404 not_found` (sem vazar existência).
- **RF-43**: Validação de propriedade do `card_id` e do `category_id` deve ocorrer no use case ANTES da gravação; recurso pertencente a outro usuário deve falhar com `404`.

### Erros e contrato HTTP

- **RF-44**: Erros devem retornar payload no formato `{ "message": "...", "code": "..." }` consistente com `internal/budgets` e `internal/card` (via `responses.ErrorWithDetails`).
- **RF-45**: Validação de input retorna `400 validation_error`; recurso inexistente ou de outro usuário retorna `404 not_found`; conflito de versão retorna `409 conflict`; conflito de idempotency retorna `409 idempotency_conflict`; falha de dependência externa retorna `502`; erro interno retorna `500`.

### Feature flag e rollout

- **RF-46**: O registro das rotas, consumers e jobs do módulo deve depender de `configs.TransactionsConfig.Enabled`. Quando `false`, o módulo não registra nenhuma rota e não consome eventos.
- **RF-47**: Alteração da flag `transactions.enabled` para `false` deve descontinuar o caminho de escrita em < 5 minutos via reload de configuração no `cmd/api`.

## Experiência do Usuário

_(omitido: feature exclusivamente de backend; UX dos clientes consumidores fica a cargo de equipes/sprint próprios.)_

## Restrições Técnicas de Alto Nível

- **RT-01**: Stack Go (`go.mod` declara Go 1.26.4) com `internal/transactions` aderente ao Padrão Obrigatório de Módulo do repositório (domain, application, infrastructure).
- **RT-02**: Persistência exclusivamente em PostgreSQL via `golang-migrate`; migrations adicionadas como `migrations/000018_create_transactions_baseline.{up,down}.sql` (numeração final a confirmar na techspec) e seguintes; sem trigger SQL para regra de domínio.
- **RT-03**: Governança transversal **R-GOV-001** (esta + go-implementation, sem flexibilização).
- **RT-04**: Contrato **R-ADAPTER-001** inegociável: zero comentários em arquivos `.go` de produção (exceto `// Code generated`, `//go:`, `//nolint:` com justificativa); adapter fino com fluxo `handler|consumer|job|producer → usecase`; sem SQL direto em adapter.
- **RT-05**: Regras Estritas **R0–R7** da skill `go-implementation`: sem `init()`, sem `panic` em produção, `context.Context` em fronteiras de IO, interface no consumidor (`R6.3`), proibido `var _ Interface = (*Type)(nil)`, `errors.Join` para agregar, `%w` para wrap, `log/slog` via `observability.Logger`, sem `clock.Clock` (`time.Now().UTC()` no ponto de uso).
- **RT-06**: Identidade via `RequireUser` canônico do `internal/identity` (auth.Principal no `context.Context`).
- **RT-07**: Idempotência via `internal/platform/idempotency.Middleware` com `scope="transactions"` e TTL configurável em `configs.TransactionsConfig.IdempotencyTTL` (default a definir na techspec; sugerido 24 h).
- **RT-08**: Outbox via `internal/platform/outbox.Publisher` com `event_id` UUID, `Type` dot-separated (`transactions.<aggregate>.<action>.v1`); retry exponencial e DLQ lógica controlados por `OutboxConfig`.
- **RT-09**: Optimistic locking via coluna `version BIGINT` em todos os agregados mutáveis (`transactions`, `card_purchases`, `card_invoices`, `card_invoice_items`, `recurring_templates`, `monthly_summary`).
- **RT-10**: Multi-tenant lógico por `user_id`: toda query SQL filtra obrigatoriamente por `user_id`; sem RLS no Postgres no MVP.
- **RT-11**: Volumetria base: 1.000 usuários × 100 lançamentos/mês × penetração de crédito 30% × parcelamento médio 3x ≈ 100 k linhas/mês em `transactions` e 30 k linhas/mês em `card_invoice_items`. Sem particionamento no MVP.
- **RT-12**: SLO write p99 < 300 ms; listagem cursor p99 < 200 ms; resumo mensal p99 < 100 ms; lag de consumer `monthly_summary` p95 < 5 s; disponibilidade write 99.5%/mês.
- **RT-13**: Observabilidade OTel completa via `observability.Observability`: spans `transactions.<layer>.<operation>`, métricas Prometheus listadas no discovery (`transactions_transactions_created_total`, `transactions_card_purchases_created_total`, `transactions_recurring_template_created_total`, `transactions_write_duration_seconds`, `transactions_read_duration_seconds`, `transactions_monthly_summary_recompute_duration_seconds`, `transactions_monthly_summary_drift_total`, `transactions_outbox_consumer_lag_seconds`, `transactions_outbox_dead_letter_total`, `transactions_idempotency_replay_total`), logs estruturados sem PII de `description`, `amount_cents`, `category_name_snapshot`.
- **RT-14**: Tracing distribuído com `trace_id` propagado em `outbox.Event.Metadata` para correlacionar publish ↔ consume.
- **RT-15**: Cardinalidade de labels Prometheus controlada: enums (`direction`, `payment_method`, `installments_bucket`, `status`, `operation`) são permitidos; `user_id` e `category_id` **não** podem aparecer como label.
- **RT-16**: Testes unitários com `mockery` + `testify/suite` e testes de integração com `internal/platform/testcontainer.Postgres` (build tag `integration`).
- **RT-17**: Feature flag `configs.TransactionsConfig.Enabled` em `configs/config.go` (Viper + `mapstructure`); reload em < 5 minutos.
- **RT-18**: Fuso de negócio para cálculo de `ref_month`: `America/Sao_Paulo`, consistente com `prd-budgets-monthly`.
- **RT-19**: Valor monetário sempre em `INT8 amount_cents` (BRL); sem `NUMERIC`; sem multi-moeda no MVP.
- **RT-20**: Snapshots de `BillingCycle` (`closing_day`, `due_day`) são **estáticos**; o módulo **não** consome eventos do `internal/card` no MVP.
- **RT-21**: Conformidade com Resolução BCB 290/2023 — fluxos novos não aceitam `payment_method=doc`; leitura de registros legados com `doc` é tolerada.
- **RT-22**: Categorias e subcategorias são gerenciadas exclusivamente em `internal/categories`; o módulo guarda `FK + snapshot do nome` em todos os agregados que referenciam categoria; sem ON DELETE CASCADE entre tabelas de módulos distintos.

## Fora de Escopo

- **OUT-01**: Transferência entre contas próprias do usuário (entra na v2).
- **OUT-02**: Multi-moeda; apenas BRL no MVP.
- **OUT-03**: Anexos/comprovantes (upload de nota/recibo).
- **OUT-04**: Imutabilidade de fatura pós-pagamento; edição da compra-pai cascateia em faturas já fechadas no MVP. Backlog para v2.
- **OUT-05**: Consumo de eventos de `internal/card` (`card.updated.v1`) e `internal/categories` para retroagir snapshots; mantemos snapshot estático.
- **OUT-06**: Canary por porcentagem de usuários no rollout; apenas big-bang controlado por feature flag.
- **OUT-07**: API GraphQL; apenas REST `/v1`.
- **OUT-08**: Exportação CSV/PDF de extrato e relatórios analíticos avançados.
- **OUT-09**: Endpoints administrativos para regerar `monthly_summary` ou expurgar outbox; operação via SQL/runbook + job diário.
- **OUT-10**: View materializada para extrato unificado; postergada até observar drift de listagem.
- **OUT-11**: Criptografia at-rest aplicativa do campo `description` via `pgcrypto`; LGPD baseline atendido via TLS + criptografia at-rest gerenciada pelo Postgres.
- **OUT-12**: Trilha de auditoria de leitura (quem leu qual lançamento); não há driver regulatório no MVP.
- **OUT-13**: Rate-limit dedicado para `POST /v1/card-purchases`; relies em rate-limit global existente.
- **OUT-14**: Export de dados LGPD por módulo; decisão delegada ao `internal/identity` futuramente.
- **OUT-15**: Recorrência com cláusula contábil sofisticada (anual com pró-rata, mensal com ajuste de fim de mês acima do dia 28); `day_of_month` é restrito a 1..28 para evitar drift.

## Suposições e Questões em Aberto

- **AS-01**: Numeração final das migrations (`000018_*` é sugestão; última migração existente é `000013` em `internal/budgets`; sequência exata será confirmada na techspec).
- **AS-02**: Nome exato de scope do middleware de idempotência (`transactions` vs `internal-transactions`) será confirmado na techspec.
- **AS-03**: Convenção de verbo dos eventos (`created`/`updated`/`deleted` vs `commit` à la `budgets.expense.committed.v1`); ajuste na techspec para manter aderência com convenção do repositório.
- **AS-04**: Política de retenção do outbox para eventos `transactions.*` se diferente do default global.
- **AS-05**: Decisão de UX para aviso ao usuário ao editar fatura já fechada (modal/inline) fica com produto.
- **AS-06**: Job de recorrência roda no dia 1 de cada mês? Ou no `day_of_month` de cada template? Operacionalmente, a recomendação é dia 1 (lote único) com `occurred_at` derivado de `day_of_month`. A confirmar na techspec.
- **AS-07**: Disponibilidade de `k6` no pipeline de homologação para teste de carga (RT-12 SLO) precisa ser confirmada.
- **AS-08**: Avaliar necessidade de rate-limit por usuário em `POST /v1/card-purchases` se observação de produção mostrar abuso com 24 parcelas.
- **AS-09**: Verificar se já existe convenção do repositório para `feature flag` (ex.: `BUDGETS_ENABLED`); `transactions.enabled` é provisório.
- **AS-10**: Sequência exata e DDL das migrations será fechada na techspec; nomes de colunas e índices acima são compromissos de produto, não de schema final.
