# Transcript do Brainstorming Decisório

## Contexto Inicial

**Solicitação**: criar módulo CRUD de cartões de crédito do usuário no `mecontrola` (Go), com nome, apelido, data de vencimento (`due_day`) e data de fechamento (`closing_day`). Entrega MVP **production-ready/proof inegociável**. O módulo é base para o futuro módulo de transações, no qual a forma de pagamento será cartão de crédito.

**Regra crítica de negócio (cálculo de fatura)**:
- Dado: `due_day` (dia do vencimento da fatura) e `closing_day` (dia do fechamento, X dias antes do vencimento).
- Para uma compra em data `D`:
  - Se `D <= closing_date_do_ciclo`, a compra cai na fatura cujo vencimento é o próximo `due_day`.
  - Se `D > closing_date_do_ciclo`, a compra cai na fatura do mês seguinte.
- Exemplo informado: vencimento dia 01, fechamento N dias antes (ex.: 25 do mês anterior). Compra em 02/jun → fatura 01/jul. Compra em 28/jun → fatura 01/ago.

**Pesquisa oficial 2026 (bandeiras BR)** — síntese consolidada:
- Visa (4xxxxx, 13/16/19 dígitos), Mastercard (2221–2720 e 51–55, 16), Elo (BINs publicados pela Elo: 636368, 438935, 504175, 506699, 5067, 4576, 4011, 636297, 451416; 16), Hipercard (606282, 38/60; 13–19), Amex (34, 37; 15), JCB (3528–3589; 16–19), Diners (300–305, 36, 38, 39; 14–19), Discover (6011, 622126–622925, 644–649, 65; 16–19), UnionPay (62, 81; 16–19), Aura (50; 16), Cabal/Banescard/Sorocred/Credz (BIN público não documentado em fonte oficial — uso doméstico/regional).
- **Bandeiras NÃO definem ciclo de fatura** (Visa Core Rules, Mastercard Rules): o ciclo é definido pelo **emissor** (banco) por contrato com o titular. Implicação: `closing_day` e `due_day` são atributos por cartão, configurados pelo usuário, não inferidos da bandeira.
- **BACEN**: Res. 4.658/2018 (segurança cibernética); Resolução BCB 468/2025 e IN BCB 621/2025 disciplinam envio da fatura (mín. 2 dias antes do vencimento) e rotativo — nenhuma fixa `closing_day`.
- **LGPD + PCI-DSS 4.0**: `mecontrola` é aplicação não-PCI (não captura/processa pagamento). Pode persistir apenas: `bandeira`, `nome do titular`, `últimos 4 dígitos` (truncado), `validade` (sem PAN). **Proibido**: PAN completo, CVV/CVC, trilha, PIN — mesmo cifrados.

## Rodada 1 - Entendimento do Problema

### Perguntas e respostas

**Q1.1 — Qual problema central o módulo de cartões resolve agora?**
- Opções: (A) Fundar entidade para transações futuras; (B) CRUD puro sem ciclo; (C) Substituir hack atual; (D) Preparar agregação Open Finance.
- **Resposta**: **A — Fundar entidade para transações futuras**.
- Implicação: o cálculo de fatura é parte do core do MVP, não pode ser adiado.

**Q1.2 — O que define sucesso INEGOCIÁVEL do MVP production-ready?**
- Opções: (A) CRUD + cálculo determinístico; (B) CRUD + BIN/Luhn; (C) CRUD apenas; (D) CRUD + limite + multi-titular.
- **Resposta**: **A — CRUD + cálculo de fatura determinístico**.
- Implicação: função pura `InvoiceFor(purchaseDate, closingDay, dueDay, tz) → (closingDate, dueDate)` é entregável crítico, com testes exaustivos para edge cases (mês 31, fevereiro/29, virada de ano, fuso BRT/UTC, ano bissexto).

**Q1.3 — Qual risco real de adiar a regra de cálculo de fatura?**
- Opções: (A) Recalcular histórico; (B) Bloqueio do módulo de transações; (C) Decisão dispersa; (D) Nenhum.
- **Resposta**: **B — Bloqueio do módulo de transações**.
- Implicação: confirma prioridade da função `InvoiceFor` no MVP.

**Q1.4 — Quem é o emissor do cartão no modelo de dados?**
- Opções: (A) Texto livre; (B) Lista curada BR; (C) Sem campo emissor no MVP; (D) ISPB/COMPE.
- **Resposta**: **C — Sem campo emissor no MVP**.
- Implicação: agregado `Card` no MVP carrega apenas `name`, `nickname`, `closing_day`, `due_day`, `user_id` e `timezone` (decisão pendente). Bandeira, last4, validade entram apenas se confirmados em rodadas seguintes; emissor fica fora.


## Rodada 2 - Escopo e Restrições

### Perguntas e respostas

**Q2.1 — Como o usuário informa o ciclo de fatura?**
- **Resposta**: **A — Dois dias do mês: `closing_day` + `due_day` (1-31)**.
- Implicação: schema `cards.closing_day SMALLINT CHECK BETWEEN 1 AND 31` + `cards.due_day SMALLINT CHECK BETWEEN 1 AND 31`. Algoritmo aplica clamp `min(day, daysInMonth(month, year))` para fev (28/29), abr/jun/set/nov (30).

**Q2.2 — Fronteira arquitetural.**
- **Resposta**: **A — Bounded context próprio: `internal/card`**.
- Implicação: novo pacote raiz com `domain/`, `usecase/`, `adapter/` (Postgres), `handler/` (HTTP), `wiring/`. Segue Padrão Obrigatório de Módulo do `AGENTS.md`. Porta pública para módulo de transações: `CardLookup` + `InvoiceFor`.

**Q2.3 — Onde vive `InvoiceFor`.**
- **Resposta**: **A — Serviço de domínio puro em `internal/card/domain`**.
- Implicação: função pura, sem IO, sem deps; chamável tanto pelo CRUD quanto pelo futuro módulo de transações sem repositório. Testes table-driven exaustivos.

**Q2.4 — Campos obrigatórios no MVP do agregado.**
- **Resposta**: **`name` + `nickname` + `closing_day` + `due_day`**.
- Implicação: bandeira (`brand`) e `last4` ficam fora do MVP.

**Q2.5 — Timezone canônico.**
- **Resposta**: **A — `America/Sao_Paulo` (BRT/-03)**.
- Implicação: persistência em UTC; cálculo do ciclo em `America/Sao_Paulo` via `time.LoadLocation` (carregado uma vez, reutilizado). Compra `purchase_date` em UTC é convertida para BRT antes de comparar com `closing_date`.

**Q2.6 — Política de exclusão.**
- **Resposta**: **A — Soft-delete (`deleted_at TIMESTAMPTZ NULL`) + arquivamento**.
- Implicação: queries de listagem filtram `deleted_at IS NULL`; transações futuras podem referenciar `card_id` arquivado sem violação de FK.

**Q2.7 — Regra de unicidade.**
- **Resposta**: **B — Unicidade `(user_id, nickname)` entre ativos**.
- Implicação: índice parcial Postgres `CREATE UNIQUE INDEX cards_user_nickname_active ON cards (user_id, nickname) WHERE deleted_at IS NULL`.

**Q2.8 — Fora-de-escopo do MVP (multi-select).**
- **Resposta**: **TODAS as quatro** — limite de crédito, brand/last4, multi-titular, Open Finance.
- Implicação: MVP minimalista, foco em CRUD + ciclo. Esses itens entram em backlog pós-MVP.


## Rodada 3 - Alternativas

### Conjunto comparativo de alternativas (algoritmo de ciclo)
- **A — Função pura `InvoiceFor` + clamp + tests exaustivos**: stateless, O(1), reentrante; testes table-driven com 50+ fixtures cobrindo fev/29, dia 31, virada de ano, due<closing, due==closing.
- **B — Pré-computação materializada (`invoices` table)**: ao criar/editar cartão, gera N próximos `invoices(card_id, period, closing_at, due_at)`; lookup via SQL. Exige job de rollover. Fonte dupla de verdade.
- **C — Híbrido: função pura + materialized view Postgres**: cálculo runtime + view materializada `next_invoices` para dashboards. REFRESH necessário a cada UPDATE.
- **D — Biblioteca de recorrência (RRULE/cron lib)**: modela ciclo como recorrência RFC 5545 via `teambition/rrule-go` ou similar. Overengineering para regra trivial.

### Decisões da rodada

**Q3.1 — Abordagem do cálculo.**
- **Resposta**: **A — Função pura `InvoiceFor` + clamp + tests exaustivos**.

**Q3.2 — Convenção `closing` vs `due`.**
- **Resposta**: **A — Auto-detecção pelo algoritmo**.
- Regra: se `closing_day > due_day`, fechamento é no mês anterior ao vencimento (caso Itaú/Bradesco — fecha 25, vence 5). Se `closing_day < due_day`, fechamento no mesmo mês (caso Nubank tradicional — fecha 18, vence 25). Sem flag.

**Q3.3 — Estratégia de carga em transações futuras.**
- **Resposta**: **A — `InvoiceFor` inline na criação da transação + denormalização de `due_date`/`closing_date` no registro de transação**.
- Implicação: consultas de fatura O(1) por índice composto `(card_id, due_date)`. Mudanças no ciclo do cartão NÃO recalculam transações antigas — histórico financeiro preservado.

**Q3.4 — Idempotência do CRUD.**
- **Resposta**: **A — Header `Idempotency-Key` em POST/PUT/DELETE**.
- Implicação: reutilizar padrão existente em `internal/billing`/`internal/identity`; tabela `card_idempotency_keys(key, user_id, response_hash, created_at)` com TTL 24h.


## Rodada 4 - Trade-offs

**Q4.1 — Velocidade vs robustez.**
- **Resposta**: **Robustez extrema — 50+ fixtures table-driven + property-based tests via `testing/quick` (ou `go-cmp` + fuzzing leve)**.
- Implicação: tempo total ~6 dias (5 + 1 de robustez extrema). Custo aceito explicitamente em troca de blindagem da regra financeira.

**Q4.2 — Imutabilidade do histórico.**
- **Resposta**: **Histórico imutável — transação passada NUNCA migra de fatura**.
- Implicação: denormalização de `closing_date`/`due_date` no agregado `Transaction` (futuro módulo); cartão pode mudar ciclo a qualquer momento sem efeito retroativo. Endpoint de recálculo manual fica fora do MVP.

**Q4.3 — API contract.**
- **Resposta**: **Endpoint público `GET /cards/:id/invoices?for=<date>` + porta interna**.
- Implicação: front pode validar UI mesmo antes do módulo de transações; contrato HTTP documentado em OpenAPI/Swagger; porta Go (`CardLookup.InvoiceFor`) reutilizada internamente.

**Q4.4 — Observabilidade mínima.**
- **Resposta**: **Logs estruturados com `trace_id` + Traces OpenTelemetry com spans em domain + adapter**.
- Implicação: span `card.usecase.Create`, `card.adapter.pg.Insert`, `card.domain.InvoiceFor` — permite auditoria de latência e dependência de Postgres. Métricas Prometheus e audit log domínio ficam fora do MVP (entram em fase 2).

### Riscos aceitos
- Sem audit log de domínio no MVP — risco mitigado parcialmente por traces OTel + logs estruturados; aceito em troca de tempo.
- Sem versionamento histórico de ciclos por cartão — risco baixo no MVP (1 usuário, baixo volume); aceito.
- Recalculo retroativo de faturas fica fora — risco financeiro mitigado pela imutabilidade de transações.

### Riscos inaceitáveis (eliminados)
- Persistência de PAN/CVV (PCI-DSS) — modelo só carrega `name`, `nickname`, `closing_day`, `due_day`, `user_id`.
- Recalculo automático de histórico ao mudar ciclo — descartado.
- Cálculo em tempo real toda consulta — descartado em favor de denormalização.


## Rodada 5 - Seleção de Direção

**Síntese apresentada**: Alternativa A — `InvoiceFor` puro + `internal/card` BC + persistência Postgres com soft-delete e unicidade parcial + idempotência via header + cálculo em America/Sao_Paulo + endpoint público + porta interna + logs estruturados com trace_id + OTel spans. Scorecard: 44/45. Tempo: ~6 dias. Aderente a R0–R7. Sem risco PCI/LGPD. Histórico imutável.

**Q5.1 — Decisão final.**
- **Resposta**: **A — Confirmo, prosseguir para discovery técnico**.
- Próximo passo: skill `technical-discovery-production` consumindo este bundle para detalhar schema PostgreSQL, contratos OpenAPI, ADRs, plano de testes e estratégia de migração, antes de `create-prd`.

## Decisões Registradas

| ID | Decisão | Origem | Status |
| --- | --- | --- | --- |
| D1 | Bounded context próprio `internal/card` seguindo Padrão Obrigatório de Módulo | Q2.2 | confirmada |
| D2 | `InvoiceFor` é função pura em `internal/card/domain` | Q2.3 | confirmada |
| D3 | Schema MVP: `name`, `nickname`, `closing_day`, `due_day`, `user_id`, `created_at`, `updated_at`, `deleted_at` | Q2.4 + Q2.6 | confirmada |
| D4 | Timezone canônico `America/Sao_Paulo` para cálculo; UTC para persistência | Q2.5 | confirmada |
| D5 | Soft-delete (`deleted_at`) | Q2.6 | confirmada |
| D6 | Unicidade parcial `(user_id, nickname)` ativos | Q2.7 | confirmada |
| D7 | Fora-de-escopo: limite, brand/last4, multi-titular, Open Finance | Q2.8 | confirmada |
| D8 | Algoritmo com clamp `min(day, daysInMonth)` + auto-detecção `closing_day > due_day` | Q3.2 | confirmada |
| D9 | `InvoiceFor` inline na criação de transação (futuro) com denormalização `closing_date`/`due_date` no agregado Transaction | Q3.3 + Q4.2 | confirmada |
| D10 | Header `Idempotency-Key` em POST/PUT/DELETE, TTL 24h | Q3.4 | confirmada |
| D11 | Robustez extrema: 50+ fixtures table-driven + property-based tests | Q4.1 | confirmada |
| D12 | Histórico financeiro imutável; mudança de ciclo NÃO migra transações antigas | Q4.2 | confirmada |
| D13 | Endpoint público `GET /cards/:id/invoices?for=<date>` + porta Go `CardLookup.InvoiceFor` | Q4.3 | confirmada |
| D14 | Observabilidade MVP: logs estruturados com trace_id + OTel spans em domain/adapter | Q4.4 | confirmada |
| D15 | Aderência total a R0–R7: sem `init()`, sem `panic` em prod, `context.Context` em IO, `errors.Join`/`fmt.Errorf("ctx: %w", err)`, goroutines canceláveis (nada com goroutine prevista no MVP) | AGENTS.md | confirmada |
