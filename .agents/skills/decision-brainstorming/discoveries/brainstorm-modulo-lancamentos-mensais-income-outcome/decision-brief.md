# DECISION BRIEF

## Problema
O aplicativo precisa de um modulo central para registrar lancamentos financeiros por usuario e mes de referencia (ex: 06/2026), com agregados `income`, `outcome` e `total` (= income - outcome). Hoje os modulos `internal/budgets` e `internal/card` existem mas nao ha fonte canonica de movimentacoes do usuario, o que bloqueia o uso real do app. Lancamentos de cartao de credito precisam respeitar a competencia da fatura (mes de vencimento) usando as datas de fechamento/vencimento ja modeladas em `internal/card`, com suporte a parcelamento. Cada lancamento deve disparar eventos de dominio para que `internal/budgets` (e futuramente outros consumers) reaja sem acoplamento sincrono.

## Objetivo
Definir a modelagem de dominio e a arquitetura do novo modulo de lancamentos mensais de forma production-ready ja no MVP, com CRUD completo (criar, editar totalmente, excluir), simulacao de fatura para cartao de credito com parcelamento, projecao materializada do resumo mensal, integracao com `internal/budgets` via eventos de dominio publicados pelo `outbox.Publisher`, e aderencia obrigatoria ao Padrao Obrigatorio de Modulo do repositorio (R0-R7) e ao contrato R-ADAPTER-001. Sucesso preliminar: bundle aprovado, scorecard validado e direcao pronta para `technical-discovery-production` produzir contratos, schema SQL, formato de eventos, fronteiras com `internal/card`/`internal/categories` e politica de re-projecao de `monthly_summary`.

## Escopo Inicial
Inclui:
- Agregado `Transaction` para lancamentos avulsos (Debito em conta, Pix, Cartao de debito, TED, DOC) com tipo entrada/saida, descricao, valor BRL, categoria, subcategoria opcional, data e `ref_month` derivado da data.
- Agregado `CardPurchase` (compra-pai de cartao de credito) com 1..N `CardInvoiceItem`s linkados, sob `CardInvoice` por (`card_id`, `ref_month`), com `ref_month` derivado de snapshot de `closing_day`/`due_day` do `internal/card`.
- Projecao `monthly_summary(user_id, ref_month, income, outcome, total)` mantida por consumer reativo do outbox.
- Endpoints REST CRUD: criar, listar (paginacao cursor), obter, editar totalmente e excluir lancamentos e compras de cartao; obter resumo mensal por `ref_month`.
- Idempotency-Key middleware em todas as mutacoes; optimistic locking via coluna `version`.
- Eventos publicados via outbox: `TransactionCreated/Updated/Deleted`, `CardPurchaseCreated/Updated/Deleted`.
- Observabilidade OTel (spans em handler/usecase/repo/consumer, metricas Prometheus, logs estruturados).
- Multi-tenant logico: toda query filtra por `user_id` derivado do principal middleware.
- Testes unitarios (mockery) e de integracao (testcontainers Postgres) cobrindo handlers, use cases, repos e consumer.
- Validacao da lista canonica de formas de pagamento contra BACEN/SPB (Debito em conta, Pix, Cartao credito, Cartao debito, TED, DOC) durante o discovery tecnico.

Exclui:
- Transferencia entre contas (mover dinheiro entre contas proprias do usuario).
- Multi-moeda (so BRL no MVP).
- Anexos/comprovantes (upload de nota/recibo).
- Lancamento recorrente automatico (salario mensal, assinatura) - decisao pendente; default fora do MVP ate confirmacao.
- Imutabilidade de fatura pos-pagamento (edicao retroage em faturas fechadas no MVP).
- Consumo de eventos de `internal/card` ou `internal/categories` no MVP (snapshot estatico).
- API GraphQL, exportacao CSV/PDF, relatorios analiticos avancados.

## Restrições
- Stack Go conforme `go.mod`; cumprir Regras Estritas R0-R7 da skill `go-implementation`.
- R-ADAPTER-001 inegociavel: zero comentarios em `.go` de producao; adapter fino com fluxo `handler/consumer/job/producer -> usecase`.
- Persistencia em PostgreSQL; sem trigger SQL para regra de dominio.
- Outbox + Idempotency-Key + optimistic locking sao padrao obrigatorio para mutacoes.
- Mensageria de dominio passa exclusivamente pelo `outbox.Publisher`; sem broker externo no MVP.
- Multi-tenant logico por `user_id` em toda query; auth via principal middleware.
- Observabilidade OTel obrigatoria em handler, usecase, repo e consumer.
- Snapshot estatico das datas (`closing_day`, `due_day`) do `internal/card` na criacao; mudanca posterior do cartao NAO retroage no MVP.
- Consistencia eventual de `monthly_summary` aceita pelo usuario.
- Edicao de compra parcelada cascateia em todas as parcelas, inclusive em faturas ja fechadas.

## Hipóteses
- Modulos existentes (`internal/budgets`, `internal/card`, `internal/categories`, `internal/identity`) ja seguem padrao de agregados ricos + outbox + idempotency-key + version (evidencia: AGENTS.md, CLAUDE.md, commits 099671e/b710c22/b20d3c8).
- `internal/card` expoe `closing_day` e `due_day` por cartao do usuario com leitor adequado para snapshot no caso de credito (evidencia: commit 099671e).
- `outbox.Publisher` com idempotencia por `event_id` esta operacional (evidencia: secao "Outbox" em AGENTS.md, uso em budgets/billing).
- `internal/budgets` consome eventos de dominio para atualizar consumo por categoria (evidencia: commit 099671e e referencias em AGENTS.md).
- Usuario aceita consistencia eventual em `monthly_summary` (evidencia: resposta da Rodada 2 - Q6 = Disponibilidade).
- Lista de formas de pagamento (Debito em conta, Pix, Cartao credito, Cartao debito, TED, DOC) precisa ser validada contra documentacao oficial do BACEN/SPB no discovery tecnico - especialmente o status do DOC apos a descontinuacao do servico anunciada para 2024 (Resolucao BCB 290/2023). Risco baixo, mas precisa de confirmacao formal antes do PRD.
- Volume esperado no MVP cabe em tabela unica sem particionamento; revisar com volumetria real no discovery tecnico.

## Alternativas Avaliadas
### Alternativa 1 - Tabela unica transactions
Resumo:
Uma unica tabela `transactions` armazena todo lancamento, com colunas `payment_method`, `installment_index`, `installment_count` e `ref_month`. Cartao de credito parcelado vira N rows ja com `ref_month` = mes da fatura calculado no momento da criacao. Sem entidade Fatura/Item separada. Agregados `income`, `outcome`, `total` calculados sob demanda via `SUM(...)` em SQL.

Viabilidade:
- Tecnica: simples, mas dificulta integridade do parcelamento sem ancora de compra-pai (delete em massa, update consistente). Conflita com R-ADAPTER-001 se a logica de competencia/parcelamento vazar para handlers.
- Operacional: leitura sob demanda pode degradar com volume.
- Financeira: menor custo inicial; alto custo de evolucao para suportar fatura como entidade.

### Alternativa 2 - Transactions + CardPurchase + CardInvoice/Items
Resumo:
Lancamentos avulsos (Debito em conta, Pix, TED, DOC, Cartao de debito) vivem em `transactions`. Cartao de credito vira agregado `CardPurchase` (compra-pai) que materializa N `CardInvoiceItem`s linkados a `CardInvoice` por (`card_id`, `ref_month`). Projecao `monthly_summary` materializada por consumer reativo do outbox. Snapshot das datas (`closing_day`, `due_day`) do `internal/card` no momento da criacao. Dominio rico, alinhado ao Padrao Obrigatorio de Modulo dos modulos existentes (`internal/budgets`, `internal/card`).

Viabilidade:
- Tecnica: forte aderencia ao padrao do repositorio; expressividade de dominio adequada para evolucao (recorrencia, imutabilidade de fatura, relatorios). Use cases finos atendem R-ADAPTER-001.
- Operacional: consumer recalcula `monthly_summary` por evento; observabilidade simples por span/metric. Leitura O(1).
- Financeira: custo medio de implementacao; menor custo de manutencao e evolucao no medio prazo.

### Alternativa 3 - LedgerEntry append-only com projetores
Resumo:
Toda movimentacao vira `ledger_entry` imutavel append-only. Faturas, summary e historico sao projecoes derivadas via consumers do outbox. Maxima auditabilidade e replays seguros. Updates viram novos eventos com referencia ao original.

Viabilidade:
- Tecnica: padrao event-sourced exige disciplina e infraestrutura (projetores, replays, snapshots) que divergem do padrao atual de update-in-place com `version`.
- Operacional: replays e reprocessamento sao operacao critica e exigem maturidade que o time ainda nao possui no MVP.
- Financeira: custo de implementacao alto; custo de operacao alto; baixa relacao custo/beneficio para MVP.

### Alternativa 4 - Transactions + Summary materializado
Resumo:
`transactions` + `monthly_summary(user_id, ref_month, income, outcome, total)` mantido por consumer. Credito parcelado expandido em rows com `ref_month` da fatura, sem entidade Fatura separada. Pragmatico e rapido, mas esconde a entidade Fatura - consulta de "minha fatura de junho" exige agregacao implicita por `card_id` + `payment_method`.

Viabilidade:
- Tecnica: leitura O(1) de summary, mas perda de expressividade da Fatura compromete UX e relatorios.
- Operacional: consumer e simples; reprojecao por user/mes e tratavel.
- Financeira: custo medio-baixo de implementacao; alto custo de evolucao se Fatura precisar ser modelada depois.

## Trade-offs
- Alternativa 2: aceitar dominio com mais agregados (Transaction, CardPurchase, CardInvoice, CardInvoiceItem, MonthlyLedger) em troca de expressividade e aderencia ao padrao do repositorio.
- Consistencia eventual de `monthly_summary`: aceitar janela ms-seg entre commit do lancamento e atualizacao da projecao para preservar disponibilidade do caminho de escrita.
- Edicao retroativa em faturas fechadas: aceitar que update da compra-pai cascateia em todas as parcelas, inclusive em faturas ja "fechadas", em troca de UX simples; mitigado por evento de dominio que recalcula summary dos meses afetados.
- Snapshot estatico das datas do cartao: aceitar que mudancas em `closing_day`/`due_day` do `internal/card` NAO retroagem em lancamentos passados, em troca de simplicidade e isolamento (so publica, nao consome).
- Eventos granulares por tipo (Transaction + CardPurchase) em vez de um unico `LedgerChanged`: aceitar maior numero de tipos de evento em troca de semantica explicita para `internal/budgets` e futuros consumers.

## Riscos
- Risco: Inconsistencia entre `monthly_summary` e soma real das transacoes/itens de fatura.
  Impacto: usuario ve total errado e pode tomar decisoes financeiras incorretas.
  Probabilidade: media.
  Mitigação: job de reconciliacao diario; metricas de divergencia; recomputacao idempotente por (`user_id`, `ref_month`); evento `MonthlyLedgerRecomputed` opcional para auditoria.
- Risco: Edicao retroativa em fatura fechada confunde o usuario.
  Impacto: divergencia entre fatura real do banco e fatura simulada no app.
  Probabilidade: media.
  Mitigação: avisar UX antes do save; backlog para v2 com imutabilidade pos-pagamento; trilha de auditoria via eventos.
- Risco: DOC oficialmente descontinuado pela BACEN ainda figurar como forma de pagamento.
  Impacto: lista de formas de pagamento desalinhada com mercado.
  Probabilidade: baixa.
  Mitigação: validar nomenclatura e status no discovery tecnico; manter DOC apenas para registro historico se confirmado descontinuado; documentar decisao.
- Risco: Acoplamento implicito com `internal/card` via snapshot quebra se schema do card mudar.
  Impacto: lancamentos novos podem calcular competencia errada.
  Probabilidade: baixa.
  Mitigação: usar reader explicito do card no usecase; contrato versionado; testes de integracao.
- Risco: Volume de eventos no outbox cresce rapido com parcelamento em massa.
  Impacto: latencia de projecao e custo de armazenamento.
  Probabilidade: baixa-media.
  Mitigação: bulk-create de itens dentro da mesma compra emite UM evento `CardPurchaseCreated` (e nao N `CardInvoiceItemAdded`); particionamento por `ref_month` no medio prazo.

## Custos
Estimativa relativa:
média

Drivers de custo:
- Quatro a cinco agregados de dominio com testes unitarios e de integracao.
- Consumer reativo do outbox para `monthly_summary` (codigo, deploy, observabilidade).
- Endpoints REST CRUD para `Transaction` e `CardPurchase` mais leitura de `MonthlyLedger`.
- Schema SQL com pelo menos 5 tabelas (`transactions`, `card_purchases`, `card_invoices`, `card_invoice_items`, `monthly_summary`) mais indices e migrations.
- Testes de integracao com testcontainers Postgres cobrindo CRUD, parcelamento, recomputacao de summary e idempotencia.
- Observabilidade OTel (spans, metricas, logs) em handler, usecase, repo e consumer.

## Impactos Operacionais
- Deploy: novo modulo `internal/transactions` (nome provisorio) precisa entrar em `cmd/api`, registrar handlers, jobs e consumers; configuracao via `configs/`.
- Migracao: schema novo via ferramenta de migracao do repositorio; rollback exige drop de tabelas, sem dependencia direta de modulos existentes.
- Operacao do consumer de `monthly_summary`: precisa de retry idempotente, DLQ logica e metricas de divergencia.
- Suporte: alertas para divergencia entre `SUM(transactions)+SUM(card_invoice_items)` e `monthly_summary` por janela.
- Onboarding: documentar contrato de evento (`TransactionCreated/Updated/Deleted`, `CardPurchaseCreated/Updated/Deleted`) para `internal/budgets` e futuros consumers.
- Time: nenhuma necessidade nova de skill; pratica ja consolidada em `internal/budgets` e `internal/card`.

## Segurança
- Autenticacao via principal middleware; toda requisicao precisa de `user_id` autenticado.
- Autorizacao multi-tenant logica: queries SQL filtram por `user_id`; testes de integracao validam que usuario A nao acessa dados do usuario B.
- Dados sensiveis: valores monetarios e descricao podem conter dados pessoais; logs nao devem registrar `description` em texto livre por padrao.
- Compliance/auditoria: eventos de dominio no outbox sao trilha de auditoria; manter `event_id` e `occurred_at` para reconciliacao.
- Idempotency-Key + version evitam dupla criacao e race conditions.

## Observabilidade
- Spans OTel por use case (`transactions.create`, `transactions.update`, `card_purchases.create`, `monthly_summary.recompute`).
- Metricas Prometheus: `transactions_created_total{payment_method=...}`, `card_purchases_created_total{installments=...}`, `monthly_summary_recompute_duration_seconds`, `monthly_summary_drift_total` (divergencia detectada).
- Logs estruturados com `user_id`, `ref_month`, `transaction_id`, `card_purchase_id`, `event_id`.
- Tracing distribuido: propagar contexto do handler ate o consumer via outbox (campo `trace_id`).
- Alertas: divergencia de summary por mais de 5min, taxa de erro do consumer >1%, latencia p99 de write >300ms.

## Escalabilidade
- Volume inicial pequeno (single-user app); tabela unica suporta milhoes de rows com indices em (`user_id`, `ref_month`, `payment_method`).
- Crescimento previsivel: 30-100 lancamentos/mes por usuario ativo; parcelamento amplifica em 1-24x.
- Particionamento por `ref_month` ou por `user_id` no medio prazo se volume crescer rapido (decisao para techspec, nao bloqueante para MVP).
- Consumer de `monthly_summary` pode escalar horizontal por shard de `user_id`.
- Leitura O(1) via projecao materializada cobre listagem por mes e dashboard.

## Alternativa Recomendada
Alternativa 2 - Transactions + CardPurchase + CardInvoice/Items

## Justificativa
A Alternativa 2 e a unica que entrega simultaneamente: (1) dominio expressivo o bastante para representar Fatura como entidade real (necessario para UX de cartao de credito e para evolucoes como imutabilidade pos-pagamento); (2) aderencia direta ao padrao do repositorio - `internal/budgets` e `internal/card` ja modelam agregados ricos com `version`, outbox e idempotency-key, o que reduz risco de divergencia arquitetural; (3) integridade nativa de parcelamento via compra-pai (`CardPurchase`) que ancora todas as parcelas e simplifica update/delete em cascata; (4) integracao com `internal/budgets` por eventos granulares de dominio sem acoplamento sincrono; (5) custo medio que cabe no MVP e maior retorno na evolucao. As alternativas 1 e 4 sacrificam integridade ou expressividade; a 3 e overkill para o estagio do produto. Scorecard: 35 pontos (lider), com melhor confiabilidade (5), manutenibilidade (5) e risco operacional (4) entre as quatro opcoes.

## Decisões Pendentes
- Confirmar se lancamento recorrente automatico (salario mensal, assinatura) entra ou nao no MVP; default atual = fora.
- Validar lista canonica de formas de pagamento contra BACEN/SPB em 2026 (especialmente status do DOC apos descontinuacao anunciada para 2024) durante o `technical-discovery-production`.
- Definir politica final de exclusao/renomeacao de categoria com snapshot ou nao (em conjunto com `internal/categories`) no discovery tecnico.
- Definir nome final do modulo (`internal/transactions` provisorio; alternativas: `internal/ledger`, `internal/finance-entries`).
- Definir estrategia de imutabilidade de fatura pos-pagamento (backlog v2).

## Próximo Passo Recomendado
technical-discovery-production com objetivo de produzir contratos REST, schema SQL, formato dos eventos do outbox, fronteiras com `internal/card` e `internal/categories`, politica de re-projecao do `monthly_summary` e estimativa de volumetria/observabilidade pronta para handoff ao `create-prd`.
