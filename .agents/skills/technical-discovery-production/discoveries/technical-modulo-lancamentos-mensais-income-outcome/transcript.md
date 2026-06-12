# Transcript do Discovery Técnico

## Contexto Inicial
Discovery deriva diretamente do bundle de brainstorming `.agents/skills/decision-brainstorming/discoveries/brainstorm-modulo-lancamentos-mensais-income-outcome/` (Alternativa 2 confirmada). Objetivo: produzir dossie tecnico production-ready do novo modulo de lancamentos mensais (income/outcome) por usuario e mes de referencia, com fatura de cartao via integracao com `internal/card`, e eventos de dominio consumidos por `internal/budgets`.

Materiais de apoio carregados:
- Bundle decision-brainstorming (decision-brief, transcript, assumptions, scorecard).
- Codigo real do repositorio: `internal/budgets`, `internal/card`, `internal/categories`, `internal/identity`, `internal/platform/outbox`, `internal/platform/idempotency`, `internal/platform/testcontainer`.
- Padroes verificados: outbox.Publisher (UUID event_id, retry/backoff, tabela platform.outbox), idempotency middleware (tabela mecontrola.idempotency_keys, scope+key+user_id), principal middleware (X-User-ID), optimistic locking BIGINT version, cursor base64(created_at,id), spans OTel `module.layer.operation`, mockery + testify/suite + testcontainers Postgres, migrations golang-migrate, `responses.ErrorWithDetails`.
- Go 1.26.4 declarado em `go.mod`.

Conhecimento setorial: Resolucao BCB 290/2023 anunciou descontinuacao do DOC com fim operacional em 2024; PIX (Resolucao BCB 1/2020 e posteriores) e TED permanecem como meios eletronicos vigentes em 2026. PIX + TED + cartoes (credito/debito) + debito em conta + dinheiro + boleto formam o conjunto canonico do mercado brasileiro.

## Rodada 1 - Objetivo, escopo e criticidade

**Q1.** Nome final do modulo Go.
- Opcoes: (a) transactions, (b) ledger, (c) finance-entries, (d) cashflow.
- Resposta: transactions. Definicao: `internal/transactions`; eventos `transactions.<aggregate>.<action>.v1`.

**Q2.** Lancamento recorrente automatico (salario, assinatura) entra no MVP?
- Opcoes: (a) Fora do MVP, (b) Dentro do MVP, (c) Como flag opcional.
- Resposta: Dentro do MVP. Definicao: agregado `RecurringTemplate` (frequencia mensal/anual, data inicio, data fim opcional) + job mensal que materializa `Transaction` ou `CardPurchase` em lote idempotente. Escopo do MVP aumenta em ~2-3 semanas; aceito pelo usuario.

**Q3.** Politica de categoria/subcategoria.
- Opcoes: (a) FK + snapshot do nome, (b) So FK, (c) Snapshot completo (kind, parent_id, name).
- Resposta: FK + snapshot do nome. Definicao: colunas `category_id`, `subcategory_id` (FK virtual; sem ON DELETE CASCADE), `category_name_snapshot`, `subcategory_name_snapshot` gravados em create/update; relatorios usam snapshot para preservar historico.

**Q4.** Restricao dominante do MVP.
- Opcoes: (a) Robustez sem comprometer prazo, (b) Compliance LGPD, (c) Custo minimo, (d) Velocidade.
- Resposta: Robustez sem comprometer prazo. Definicao: idempotencia + outbox + observabilidade + testes de integracao sao inegociaveis; prazo cede antes que qualidade.

## Rodada 2 - Arquitetura, dados, volumetria e custo

**Q5.** Formas de pagamento canonicas.
- Opcoes: (a) Sem DOC com retrocompat, (b) Lista pedida literal, (c) Minimo + extensao futura, (d) Maximo do mercado.
- Resposta: Sem DOC com retrocompatibilidade.
- Definicao: enum `payment_method` no MVP: `pix`, `ted`, `debit_in_account`, `credit_card`, `debit_card`, `cash`, `boleto`. Leitura aceita o token literal `doc` em registros importados ou legados sem aceitar em novos creates (rejeitado em validacao de input do POST/PATCH).

**Q6.** Schema SQL: tabela unica ou separadas.
- Opcoes: (a) Separadas, (b) Tabela unica + discriminator, (c) Separadas + view materializada.
- Resposta: Separadas. Definicao: `transactions`, `card_purchases`, `card_invoices`, `card_invoice_items`, `recurring_templates`, `monthly_summary`. Indice composto `(user_id, ref_month, created_at, id)` em `transactions` e `card_invoice_items`. View materializada postergada para v2 (avaliar pelo drift de listagem).

**Q7.** Consistencia do `monthly_summary`.
- Opcoes: (a) Eventual via consumer + reconciliacao, (b) Forte na mesma tx, (c) Calculado sob demanda.
- Resposta: Eventual via consumer + reconciliacao. Definicao: consumer do outbox recalcula linha `(user_id, ref_month)` afetada; job diario `MonthlySummaryReconciler` compara `SUM(transactions)+SUM(card_invoice_items)` vs `monthly_summary` e emite metrica `monthly_summary_drift_total` + log de divergencia.

**Q8.** Volumetria base do MVP.
- Opcoes: (a) Pequeno (1k usuarios x 100 lanc/mes), (b) Medio (10k x 150), (c) Sem dado real.
- Resposta: Pequeno. Definicao: ~100k linhas/mes em `transactions` + ~30k linhas/mes em `card_invoice_items` (parcelamento medio 3x). Tabela unica sem particionamento serve 12+ meses. SLO inicial: p99 write <300ms, listagem cursor <200ms, summary read <100ms.

## Rodada 3 - Seguranca, confiabilidade, observabilidade e operacao

**Q9.** Baseline de seguranca.
- Opcoes: (a) Padrao + descricao nao logada, (b) Reforcada + criptografia em repouso, (c) Reforcada + trilha de auditoria de leitura.
- Resposta: Padrao + descricao nao logada. Definicao: principal middleware obrigatorio em todas as rotas; toda query SQL filtra por `user_id` derivado de `auth.FromContext`; logs incluem `transaction_id`, `card_purchase_id`, `ref_month`, `payment_method`, `category_id`; NUNCA incluem `description`, `amount`, `category_name_snapshot`.

**Q10.** Estrategia de resiliencia.
- Opcoes: (a) Outbox + retry + DLQ logica, (b) Outbox + retry infinito, (c) Outbox + degradacao com defaults do cartao.
- Resposta: Outbox + retry + DLQ logica. Definicao: outbox at-least-once; consumer com backoff exponencial (3s, 9s, 27s, 81s, max 5 tentativas configuraveis via `OutboxConfig.RetryMaxAttempts`); apos limite, evento marcado como dead-letter e alerta `transactions_outbox_dead_letter_total > 0`. Card reader indisponivel: create de `credit_card` falha 502 cedo com retry do cliente (sem default generico).

**Q11.** Profundidade de observabilidade.
- Opcoes: (a) OTel completo + dashboards, (b) Metricas + logs, (c) Auditavel para regulacao.
- Resposta: OTel completo + dashboards. Definicao: spans `transactions.usecase.*`, `transactions.repository.*`, `transactions.consumer.*`, `transactions.job.*`; metricas Prometheus RED + drift + lag de consumer + dead-letter; logs estruturados via `observability.Logger`; tracing distribuido com `trace_id` propagado no metadata do outbox; dashboard Grafana minimo e 4 alertas operacionais.

**Q12.** Rollout em producao.
- Opcoes: (a) Big-bang controlado + feature flag, (b) Canary por % de usuarios, (c) Piloto fechado.
- Resposta: Big-bang controlado + feature flag. Definicao: rotas registradas atras de `configs.TransactionsConfig.Enabled`; migrations rodam no deploy padrao golang-migrate; rollback = flag off + revert PR (migracoes sao backward-compatible).

## Sintese Apresentada e Decisao Final

Sintese consolidada em 10 bullets apresentada ao usuario (modulo, pagamentos, fatura cartao, recorrencia, categoria, schema, consistencia, eventos, idempotencia/version, seguranca/observabilidade/rollout). Pergunta de materializacao: "Posso materializar o dossie agora?". Resposta: Materializar agora (Recomendado).

## Decisoes Registradas

- D1: Nome do modulo `internal/transactions`.
- D2: Lancamento recorrente DENTRO do MVP via agregado `RecurringTemplate` + job mensal.
- D3: Categoria armazenada com FK + snapshot do nome.
- D4: Restricao dominante: robustez sem comprometer prazo.
- D5: Formas de pagamento do MVP: pix, ted, debit_in_account, credit_card, debit_card, cash, boleto (DOC apenas leitura legada).
- D6: Schema com tabelas separadas (`transactions`, `card_purchases`, `card_invoices`, `card_invoice_items`, `recurring_templates`, `monthly_summary`).
- D7: `monthly_summary` eventual via consumer outbox + reconciliacao diaria.
- D8: Volumetria base do MVP: 1k usuarios x 100 lancamentos/mes.
- D9: Seguranca padrao + descricao nunca logada.
- D10: Resiliencia: outbox + retry exponencial + DLQ logica + alerta.
- D11: OTel completo (spans, metricas, logs, traces, dashboard, 4 alertas).
- D12: Rollout big-bang com feature flag `transactions.enabled`.
