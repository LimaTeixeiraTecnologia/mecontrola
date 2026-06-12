# Transcript do Brainstorming Decisório

## Contexto Inicial
Usuario solicitou novo modulo de lancamentos financeiros por usuario e mes de referencia (ex: 06/2026), com agregados income, outcome e total (= income - outcome). CRUD completo (criar, editar totalmente: descricao, valor, categoria, subcategoria opcional, tipo entrada/saida e forma de pagamento) e exclusao. Formas de pagamento alvo do mercado financeiro brasileiro em 2026: Debito em conta, Pix, Cartao de credito, Cartao de debito, TED e DOC (validar referencias oficiais BACEN). Cada lancamento deve atualizar income/outcome/total e disparar eventos de dominio. Quando a forma de pagamento for cartao de credito, simular fatura e itens da fatura usando datas de fechamento/vencimento do modulo `internal/card` para determinar a competencia. Foco: MVP robusto, eficiente e production-ready de forma inegociavel.

Observacao tecnica: o repositorio ja possui `internal/budgets`, `internal/card`, `internal/categories`, `internal/identity` seguindo o Padrao Obrigatorio de Modulo descrito em `AGENTS.md`. O novo modulo deve respeitar fronteiras (R0-R7) e o contrato R-ADAPTER-001 (zero comentarios em Go, adapter fino handler->usecase).

## Rodada 1 - Entendimento do Problema

**Q1.** Qual e o problema real que esse modulo precisa resolver primeiro?
- Opcoes oferecidas: (a) Acompanhar caixa diario, (b) Reconciliar com budget, (c) Antecipar fatura cartao, (d) Base para relatorios.
- Resposta do usuario: "Acompanhar caixa diario e todo evento deve adicionar ao budget via evento de dominio".
- Interpretacao registrada: foco primario em caixa diario, com integracao obrigatoria via eventos de dominio para que `internal/budgets` consuma e atualize consumo por categoria.

**Q2.** Qual resultado MINIMO faz o MVP ser considerado sucesso?
- Opcoes: (a) CRUD + totais, (b) CRUD + fatura, (c) CRUD + eventos, (d) Pacote completo (idempotencia, paginacao, observabilidade, metricas).
- Resposta: Pacote completo.

**Q3.** Qual e o risco se NAO entregarmos esse modulo agora?
- Opcoes: (a) Bloqueia uso do app, (b) Bloqueia outros modulos, (c) Atrasa validacao produto, (d) Sem urgencia.
- Resposta: Bloqueia uso do app.

## Rodada 2 - Escopo e Restrições

**Q4.** Como tratar lancamentos de cartao de credito no escopo do MVP?
- Opcoes: (a) Competencia pela fatura, (b) Sempre mes da compra, (c) Configuravel, (d) Parcelado obrigatorio MVP.
- Resposta livre do usuario: "O valor total da fatura entra no outcome do mes, suportar parcelamento e criar faturas futuras."
- Interpretacao consolidada: cartao de credito impacta o outcome do mes da fatura (competencia derivada de fechamento/vencimento do `internal/card`); parcelamento e obrigatorio no MVP, gerando N itens em faturas futuras.

**Q5.** O que fica FORA do MVP?
- Opcoes (multi): recorrencia automatica, anexos, transferencia entre contas, multi-moeda.
- Resposta: Transferencia entre contas, Multi-moeda, Anexos/comprovantes.
- Observacao: usuario NAO marcou "Lancamento recorrente" como fora; tratado como nao confirmado e listado em Decisoes Pendentes.

**Q6.** Qual restricao operacional/arquitetural domina?
- Opcoes: (a) Consistencia total, (b) Disponibilidade (consistencia eventual), (c) Simplicidade time, (d) Custo infra minimo.
- Resposta: Disponibilidade. Resumo `monthly_summary` pode ser eventually consistent atualizado por consumer.

**Q7.** O modulo deve consumir eventos de outros modulos alem de publicar?
- Opcoes: (a) So publica, (b) Publica + consome card, (c) Publica + consome categorias, (d) Bidirecional completo.
- Resposta: So publica no MVP. Snapshot estatico das datas de cartao na criacao do lancamento.

## Rodada 3 - Alternativas
Foram derivadas quatro alternativas comparaveis ao desafio do usuario, garantindo cobertura de simplicidade maxima, robustez maxima e opcoes intermediarias.

- **Alternativa 1 - Tabela unica transactions**: uma unica tabela `transactions` armazena todo lancamento; cartao credito parcelado vira N rows ja com `ref_month` igual ao mes da fatura. Sem entidade de Fatura/Item separada. Agregados calculados sob demanda via `SUM(...)`. Simplicidade maxima, expressividade de dominio baixa, parcelamento dificil de manter coerente sem ancora de compra-pai.
- **Alternativa 2 - Transactions + CardPurchase + CardInvoice/Items**: lancamentos avulsos (Debito em conta, Pix, TED, DOC, Cartao de debito) vivem em `transactions`. Cartao de credito vira agregado `CardPurchase` (compra-pai) que materializa N `CardInvoiceItem`s linkados a `CardInvoice` por `(card_id, ref_month)`. `MonthlyLedger` (`user_id`, `ref_month`) e projecao materializada por consumer. Snapshot das datas do `internal/card` na criacao. Dominio expressivo, alinhado ao Padrao Obrigatorio de Modulo (modulos atuais usam agregados ricos, ex: `Budget`, `Card`).
- **Alternativa 3 - LedgerEntry append-only com projetores**: toda movimentacao vira `ledger_entry` imutavel. Faturas, summary e historico sao projecoes derivadas via consumers do outbox. Maxima auditabilidade. Trade-off: complexidade alta para um MVP, divergencia do padrao atual do repositorio (que usa update-in-place com `version`).
- **Alternativa 4 - Transactions + Summary materializado**: `transactions` + `monthly_summary(user_id, ref_month, income, outcome, total)` mantido por consumer. Credito parcelado expandido em rows com `ref_month=fatura`, sem entidade Fatura separada. Pragmatico mas perde expressividade da fatura (consulta de "minha fatura de junho" exige agregacao implicita por `card_id` + payment_method).

Racional: Alternativa 1 sacrifica integridade de parcelamento e fatura, comprometendo o produto. Alternativa 3 e overkill para MVP. Alternativa 4 esconde a entidade fatura, dificultando UX e relatorios futuros de cartao. Alternativa 2 e a unica que equilibra dominio rico, aderencia ao padrao do repositorio (card, budgets) e suporte natural a parcelamento.

## Rodada 4 - Trade-offs

**Q8.** Edicao de lancamento de cartao ja parcelado.
- Resposta: Edicao total cascateia (atualizar a compra-pai recria/atualiza todas as N parcelas). Trade-off aceito: faturas ja "fechadas" mudam retroativamente, com recalculo do summary dos meses afetados.

**Q9.** Estrategia de idempotencia + concorrencia.
- Resposta: Idempotency-Key middleware (padrao do repo) + optimistic locking via coluna `version` no agregado. Mesma postura de `internal/budgets`, `internal/card` e `internal/identity`.

**Q10.** Eventos de dominio publicados via outbox (multi).
- Resposta: `TransactionCreated/Updated/Deleted` (lancamentos avulsos) e `CardPurchaseCreated/Updated/Deleted` (compras de cartao com parcelas).
- Implicacao: `internal/budgets` consome esses eventos para atualizar consumo por categoria; consumer do proprio modulo lancamentos atualiza `monthly_summary` e materializa faturas afetadas.

**Q11.** Criterios "production-ready" do MVP (multi).
- Resposta: Idempotencia + outbox, Observabilidade OTel (spans, metricas, logs estruturados), Paginacao + guarda multi-tenant logica (filtro por user_id em toda query), Testes integracao + unitarios (testcontainers Postgres).

Trade-offs aceitos consolidados:
- Consistencia eventual de `monthly_summary` (pode haver janela ms-seg entre commit e projecao).
- Edicao retroage em faturas fechadas (nao ha bloqueio de imutabilidade de fatura no MVP).
- Snapshot estatico de `closing_day`/`due_day` do cartao na criacao (mudanca posterior nao retroage; aceitavel para MVP).
- So publica eventos; nao consome `internal/card`/`internal/categories` no MVP.

## Rodada 5 - Seleção de Direção
Sintese apresentada ao usuario com placar:

| Alternativa | Dominio | Velocidade | Risco operacional | Aderencia repo | Total |
| --- | --- | --- | --- | --- | --- |
| 1 - Tabela unica | 2 | 5 | 3 | 2 | 12 |
| 2 - Transactions + CardPurchase + Invoice/Items | 5 | 3 | 4 | 5 | 17 |
| 3 - LedgerEntry append-only | 5 | 1 | 2 | 3 | 11 |
| 4 - Transactions + Summary materializado | 3 | 4 | 3 | 4 | 14 |

Recomendacao preliminar: Alternativa 2.

Decisao explicita do usuario: "Confirmo B" (Alternativa 2 - Transactions + CardPurchase + CardInvoice/Items).

## Decisões Registradas
- D1: Adotar Alternativa 2 (Transactions + CardPurchase + CardInvoice/Items) como direcao de modelagem do novo modulo `internal/transactions` (nome provisorio; a confirmar em discovery tecnica).
- D2: `monthly_summary(user_id, ref_month, income, outcome, total)` mantido por consumer assincrono via outbox; leitura O(1) com consistencia eventual.
- D3: Parcelamento expandido na criacao em 1 `card_purchase` (origem) + N `card_invoice_item`s linkados via `purchase_id`; update/delete da compra-pai cascateia.
- D4: Snapshot das datas (`closing_day`, `due_day`) do `internal/card` no momento da criacao do lancamento de credito; mudanca posterior do cartao NAO retroage no MVP.
- D5: Eventos publicados via outbox: `TransactionCreated/Updated/Deleted` e `CardPurchaseCreated/Updated/Deleted`. `internal/budgets` consome esses eventos.
- D6: Idempotency-Key middleware obrigatorio em todas as mutacoes; optimistic locking via coluna `version`.
- D7: Edicao de compra parcelada cascateia em todas as parcelas, inclusive em faturas ja "fechadas". Sem bloqueio de imutabilidade de fatura no MVP.
- D8: Production-ready no MVP exige: idempotencia + outbox, observabilidade OTel, paginacao cursor + guarda multi-tenant por `user_id`, testes integracao (testcontainers Postgres) e unitarios (mockery).
- D9: Formas de pagamento suportadas no MVP: Debito em conta, Pix, Cartao de credito, Cartao de debito, TED e DOC. Lista deve ser validada contra a documentacao oficial do BACEN/SPB no discovery tecnico (nomenclatura, status DOC apos fim do servico em 2024).
- D10: Fora de escopo do MVP: transferencia entre contas, multi-moeda e anexos/comprovantes. Recorrencia automatica nao foi confirmada e fica como Decisao Pendente.
- D11: Proximo passo: `technical-discovery-production` para definir contratos de API, schema SQL, formato de eventos do outbox, fronteiras com `internal/card` e `internal/categories`, e politica de re-projecao do `monthly_summary`.
