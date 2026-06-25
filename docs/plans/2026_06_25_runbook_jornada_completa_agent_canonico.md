# Runbook — Jornada Completa do Agente MeControla (Refatoração Canônica)

> Documento-companheiro do PRD/techspec `.specs/prd-refatoracao-agent-canonico/`.
> Objetivo: descrever **toda a jornada** do usuário no WhatsApp — onboarding e operação diária —
> com **1:1 de fidelidade ao código real**, mostrando para cada passo: (a) a **experiência do
> usuário** (conversa real, strings verbatim do código), (b) o **caminho de código**
> (símbolos reais `arquivo:linha`), e (c) as **tabelas persistidas** (DDL real).
>
> Princípio inegociável deste runbook: **0 gap, 0 lacuna, 0 falso positivo**. Tudo abaixo foi
> verificado contra o código em 2026-06-25 (5 clusters de auditoria + greps dirigidos). Onde o
> PRD/techspec divergia do código, a divergência está registrada em §2 (Errata) e corrigida nos
> arquivos do spec.
>
> Modelo de referência de estilo: `docs/plans/2026_06_24_arquitetura_agente_mastra_workflows_bounded_contexts.md`.

---

## 1. Como ler este runbook

**Legenda de dono da escrita** (qual bounded context grava cada tabela):

| Tag | Módulo | Escreve em (exemplos) |
|-----|--------|------------------------|
| `(I)` | `internal/identity` | `users`, `user_identities`, `identity_entitlements` |
| `(O)` | `internal/onboarding` | `onboarding_sessions`, `onboarding_tokens` |
| `(A)` | `internal/agent` | `agent_threads`, `agent_runs`, `agent_sessions`, `agent_decisions`, `agent_working_memory`, `agent_observations` |
| `(K)` | `internal/platform/workflow` (kernel) | `workflow_runs`, `workflow_steps` |
| `(P)` | `internal/platform/outbox` | `outbox_events` |
| `(T)` | `internal/transactions` | `transactions`, `transactions_recurring_templates` |
| `(B)` | `internal/budgets` | `budgets`, `budgets_allocations` |
| `(C)` | `internal/card` | `cards` |
| `(Cat)` | `internal/categories` | `categories`, `category_dictionary` (somente leitura na jornada) |

**Convenções:**
- Strings entre aspas e blocos de conversa são **verbatim** do código (com `file:line`). Onde há
  `fmt.Sprintf`, o template e os campos estão indicados.
- `[NOVO]` marca código que **ainda não existe** e entra nesta refatoração (Tasks 6–9).
- `[REMOVER]` marca código existente que sai (Tasks 2–5).
- Valores monetários são formatados por `formatBRL` (centavos → `R$ x.xxx,xx`).

---

## 2. Estado verificado do código + Errata do spec (2026-06-25)

Auditoria "0 falso positivo" do PRD/techspec contra o código. Divergências encontradas e corrigidas:

| # | Afirmação do spec | Realidade do código | Correção aplicada |
|---|-------------------|---------------------|-------------------|
| 1 | RF-39 remove `continuePendingExpenseConfirmationLegacy` | **Não existe** (já removido); sem símbolo `*Legacy`; sem `PendingExpenseConfirmationGateway` | Removido do escopo (PRD RF-39). Alvos reais que **existem**: `kernelEnabled` (`daily_ledger_agent.go:107/153/372`), `EnableKernel` (`:371` + `intent_router.go`), `parity_test.go`, `WorkflowKernelConfig.TransactionsWriteEnabled` (`configs/config.go:368`, `module.go:504`) |
| 2 | RF-41 remove `transactions.card_purchase.deleted` | **TEM consumer** `recomputeConsumer` (`transactions/module.go:321`) | **MANTER** — removê-lo quebra recompute do resumo mensal (PRD RF-41 corrigido) |
| 3 | `external.expense.v1` "tem par" (manter) | Consumer-only: `ExternalExpenseConsumer`+`IngestExternalExpense` em budgets, **zero producer** | Reclassificado **órfão consumer-sem-producer** → remover pipeline (ver §5 e recomendação §13) |
| 4 | RF-41 não lista `onboarding.income_registered` | Producer-only (`save_onboarding_income.go:79`), **zero consumer**; renda flui por `splits_calculated` | **Adicionado** à remoção (decisão do solicitante: remover) |
| 5 | "Bug latente NewAmount grava 0" | `hitl_adapters.go:89` passa `NewAmountCents`; populado em `daily_ledger_agent.go:622` | **Falso positivo** — já correto; manter só risco de optimistic-lock |
| 6 | RF-40 branch morto de budget config | Não localizado (`wireBudgetCommitGate`/`budget_session`/`budget_tools` alcançáveis); `deadcode` **não instalado** | Gated em `deadcode`; se nada apontar, RF-40 já satisfeito |
| 7 | "deadcode/staticcheck instalados" | Só `staticcheck` em `~/go/bin` | Instalar `deadcode` é pré-requisito da Task 4/5 |
| 8 | RF-10: LLM só em parse/onboarding/conversacional | `ConfigureBudgetConversation` (`module.go:466`, via `llmModule.Interpreter`) é 4º call-site de LLM | Reconciliar: sancionar exceção OU migrar p/ parse estruturado (decisão aberta — §13) |

**Confirmado SEM divergência** (techspec correto): footprint Telegram completo existe; migrations
000020/000021 livres (maior atual = `000019_create_workflow_runtime`); `telegram_external_id` em
`000001_initial_baseline` (`onboarding_tokens`) + índice parcial; 3 CHECK constraints de canal;
`step_index` ausente; `parse_inbound.go:97` `Strict:false`; `ParseIntentJSONSchema` required só
`[kind, confidence]`; onboarding usa tool-calling com `claude-haiku-4.5`; `ClassRouter`/`LLMClass`
ausentes; `KindDeleteTransactionByRef`/`KindEditTransactionByRef`/`SearchByDescription` ausentes;
17/17 usecases-porta existentes; `ConfirmState` sem `AwaitingSelect`/`*ByRef`/`TargetCandidate`;
`destructive_confirm` com 8 steps.

---

## 3. Pipeline canônico (esqueleto real)

```
WhatsApp Cloud API (Meta)
 → internal/platform/whatsapp  (verify, HMAC-SHA256, dedup wamid → channel_processed_messages)
 → identity.EstablishPrincipal (E164 → user_id)  [(I)]
 → outbox: agent.whatsapp.inbound.v1            [(P)]  EventTypeWhatsAppInbound (module.go:206)
 → internal/agent: WhatsAppInboundConsumer
 → IntentRouter.RouteWhatsApp → AgentRuntime.Execute
       startRun: ThreadGateway.GetOrCreate(user_id, channel)   → agent_threads  [(A)]
                 RunGateway.Insert(status='running')            → agent_runs     [(A)]
 → DailyLedgerAgent.Handle (daily_ledger_agent.go:203)
     1) tryResumeInbound (daily_ledger_agent.go:184) — ORDEM DETERMINÍSTICA:
          a) continuePendingExpenseConfirmation  (:186, se kernelEnabled)
          b) continuePendingApproval             (:191, gate HITL — confirmEngine)
          c) budgetRunner.Continue               (:196, conversa de orçamento)
        → só se nada resumiu:
     2) ParseInbound (ÚNICO call-site LLM de domínio; Strict=true [NOVO] → intent/plano)
     3) executor determinístico:
          - LEITURA  → WorkflowRegistry.Resolve(kind) → Tool → binding → usecase
          - ESCRITA  → kernel Engine[ExpenseState] (transactions_write) → workflow_runs/steps [(K)]
          - DESTRUTIVA/HITL → kernel Engine[ConfirmState] (destructive_confirm) → suspend/resume
       finishRun: RunGateway → agent_runs (status, outcome, duration_ms)  [(A)]
 → resposta → Graph API /messages (texto)
```

Regra de ouro (R-AGENT-WF-001 / R-WF-KERNEL-001): **nenhum** `case intent.Kind` novo no switch de
`daily_ledger_agent.go`; resolução por `WorkflowRegistry`/registry-map. Tool é adapter fino. Kernel
não conhece domínio. LLM só no parse (+ exceções sancionadas: `KindUnknown` conversacional,
onboarding e — a reconciliar — `ConfigureBudgetConversation`).

---

## 4. Mapa de tabelas (DDL real, schema `mecontrola`)

> Colunas-chave verbatim das migrations. Tipos exatos preservados.

### Identidade & ativação `(I)/(O)`
- **users** (`000001`): `id uuid PK`, `whatsapp_number text`, `email text?`, `display_name text?`,
  `status text DEFAULT 'ACTIVE'` (CHECK `IN ('ACTIVE','DELETED')`), `created_at/updated_at timestamptz`,
  `deleted_at timestamptz?`. Único: `whatsapp_number` ativo.
- **user_identities** (`000001`): `id uuid PK`, `user_id uuid FK`, `channel text`
  (CHECK `IN ('whatsapp','telegram')` → **`('whatsapp')` após migration 000020**), `external_id text`,
  `verified_at`, `created_at`, `unlinked_at?`. Único ativo: `(channel, external_id)`.
- **identity_entitlements** (`000001`): `user_id uuid PK/FK`, `subscription_id uuid`, `status text`
  (TRIALING/ACTIVE/PAST_DUE/CANCELED_PENDING/EXPIRED/REFUNDED), `period_end`, `grace_end?`, `updated_at`.
- **onboarding_tokens** (`000001`): `id uuid PK`, `token_hash bytea UNIQUE`, `status text`
  (PENDING/PAID/CONSUMED/EXPIRED), `plan_id text`, `expires_at`, `activation_token_ciphertext text`,
  `customer_mobile_e164 text?`, `consumed_by_user_id uuid?`, `activation_path text?`, `metadata jsonb`,
  **`telegram_external_id text?` + índice parcial → DROP na migration 000020**.

### Onboarding `(O)`
- **onboarding_sessions** (`000001`): `user_id uuid PK/FK`, `channel text`
  (CHECK `IN ('whatsapp','telegram')` → **`('whatsapp')` após 000020**), `state text`,
  `payload jsonb DEFAULT '{}'`, `updated_at`. `payload` carrega `OnboardingSessionPayload`
  (objective, income_cents, cards[], custom_split[], phase, recent_turns, welcome_sent_at,
  completed_at, first_tx_recorded, objective_profile).

### Agente `(A)`
- **agent_threads** (`000017`): `id uuid PK`, `user_id uuid FK`, `channel text`
  (CHECK len 1..32), `created_at/updated_at`. **UNIQUE `(user_id, channel)`** = Thread Mastra.
- **agent_runs** (`000018`): `id uuid PK`, `thread_id uuid FK`, `user_id`, `channel`, `message_id`,
  `agent_id`, `workflow`, `tool_name`, `intent_kind`, `outcome`, `status text`
  (CHECK `IN ('running','succeeded','failed')`), `error`, `decision_id uuid? FK→agent_decisions`,
  `started_at`, `ended_at?`, `duration_ms bigint`.
- **agent_sessions** (`000010`): `id uuid PK`, `user_id FK`, `channel`,
  `pending_action jsonb` (≤16KB; draft de onboarding v2 / pending expense),
  `recent_turns jsonb` (≤64KB), `expires_at`. **UNIQUE `(user_id, channel)`**.
- **agent_decisions** (`000011`): `id uuid PK`, `user_id FK`, `channel`, `message_id`,
  `intent_kind`, `prompt_sha256 (len=64)`, `llm_model`, `redacted_response jsonb`, `trace_id`,
  `decided_action`, `resulting_event_id uuid?`, `status text`
  (CHECK `IN ('pending','executed','rejected','awaiting_confirmation')`), `created_at`, `settled_at?`.
  **UNIQUE `(user_id, channel, message_id)`** → **`(user_id, channel, message_id, step_index)` após
  migration 000021** `[NOVO]` (idempotência por passo do plano multi-tool).
- **agent_working_memory** (`000013`): `user_id uuid PK`, `content text`, `updated_at`.
  Injetada no system prompt (R-AGENT-WF-001.8).
- **agent_observations** (`000014`): `id uuid PK`, `user_id`, `channel`, `content`, `created_at`,
  `expires_at DEFAULT now()+90d`.

### Kernel `(K)`
- **workflow_runs** (`000019`): `id uuid PK`, `workflow text`, `correlation_key text`, `status text`
  (CHECK `IN ('running','suspended','succeeded','failed')`), `suspend_reason text`,
  `cursor int DEFAULT 0`, `state jsonb`, `attempts int`, `max_attempts int`, `version bigint`
  (lock otimista), `last_error`, `created_at/updated_at`, `ended_at?`.
  **UNIQUE parcial `(workflow, correlation_key) WHERE status IN ('running','suspended')`**.
- **workflow_steps** (`000019`): `id uuid PK`, `run_id uuid FK`, `step_id text`, `seq int`,
  `status text` (completed/suspended/failed/skipped), `attempt int`, `duration_ms`, `error`,
  `started_at`, `ended_at?`. UNIQUE `(run_id, seq, attempt)`.

### Domínio financeiro `(T)/(B)/(C)/(Cat)`
- **transactions** (`000001`): `id uuid PK`, `user_id`, `direction smallint`, `payment_method smallint`,
  `amount_cents bigint` (CHECK `>0`), `description text`, `category_id uuid`, `subcategory_id uuid?`,
  `category_name_snapshot text`, `ref_month text` (CHECK `^\d{4}-(0[1-9]|1[0-2])$`), `occurred_at`,
  `version bigint` (lock otimista), `deleted_at?` (soft delete), `created_at/updated_at`.
- **transactions_recurring_templates** (`000001`): `id uuid PK`, `user_id`, `direction`, `payment_method`,
  `card_id uuid?`, `amount_cents`, `description`, `category_id`, `frequency smallint`,
  `day_of_month smallint`, `installments_total smallint`, `started_at`, `ended_at?`, `version`, `deleted_at?`.
- **budgets** (`000001`): `id uuid PK`, `user_id`, `competence text` (`YYYY-MM`), `total_cents bigint`,
  `state smallint` (CHECK `IN (1,2)` = draft/active), `activated_at?`, `auto_draft bool`,
  `created_at/updated_at`. UNIQUE `(user_id, competence)`.
- **budgets_allocations** (`000001`): PK `(budget_id, root_slug)`, `basis_points int` (0..10000),
  `planned_cents bigint`. `root_slug` CHECK `IN` (as 5 categorias oficiais
  `expense.custo_fixo|conhecimento|prazeres|metas|liberdade_financeira`).
- **cards** (`000001`): `id uuid PK`, `user_id FK`, `name text`, `nickname text`, `closing_day smallint`
  (1..31), `due_day smallint` (1..31), `limit_cents bigint`, `version bigint`, `deleted_at?`.
  UNIQUE ativo `(user_id, nickname)`.
- **categories** / **category_dictionary** (`000001`): taxonomia + dicionário de termos (lookup de
  categoria por merchant/alias; `term_normalized` gerado por unaccent). **Somente leitura** pelo agent
  (via `SearchDictionaryUC`); nunca escritas na jornada.

### Mensageria `(P)`
- **outbox_events** (`000001`): `id uuid PK`, `event_type text`, `aggregate_type/id`,
  `aggregate_user_id uuid?`, `payload jsonb`, `status smallint` (1=pending,2=processing,3=published,4=dead),
  `attempts/max_attempts`, `next_attempt_at`, `occurred_at`. Idempotência por `event_id` no consumer.
- **channel_processed_messages** (`000001`): PK `(channel, message_id)`, `processed_at`. Dedup de `wamid`.
  CHECK canal → `('whatsapp')` após 000020.

---

## 5. Mapa de eventos (producer → consumer) — pós-correção 2026-06-25

| Event-type | Producer | Consumer | Veredito |
|------------|----------|----------|----------|
| `transactions.transaction.created.v1` | transactions | budgets (`module.go:155`) + transactions recompute (`:316`) | **KEEP** |
| `transactions.transaction.updated.v1` | transactions | transactions recompute (`:317`) | **KEEP** |
| `transactions.transaction.deleted.v1` | transactions | budgets (`:156`) + recompute (`:318`) | **KEEP** |
| `transactions.card_purchase.created.v1` | transactions | budgets (`:157`) + recompute (`:319`) | **KEEP** |
| `transactions.card_purchase.updated.v1` | transactions | recompute (`:320`) | **KEEP** |
| `transactions.card_purchase.deleted.v1` | transactions | recompute (`:321`) | **KEEP** (corrige RF-41) |
| `onboarding.splits_calculated` | onboarding | budgets (`:154`) | **KEEP** |
| `onboarding.card_registered` | onboarding | card (`:107`) | **KEEP** |
| `onboarding.completed` | onboarding | agent (`:220`) | **KEEP** |
| `agent.intent.rejected` / `agent.intent.executed` | agent | — | **ÓRFÃO → remover** |
| `budgets.budget_activated.v1` | budgets (`activate_budget`, `edit_category_percentage:123`) | — | **ÓRFÃO → remover publish** |
| `transactions.recurring_template.{created,updated,deleted}` | transactions | — | **ÓRFÃO → remover** |
| `onboarding.income_registered` | onboarding (`save_onboarding_income:79`) | — | **ÓRFÃO → remover** (decisão) |
| `external.expense.v1` | **nenhum** | budgets (`:153`) | **ÓRFÃO consumer-only → remover pipeline** (recomendado) |

> ⚠️ Cartões **não publicam eventos**: `CreateCard`/`UpdateCard`/`SoftDeleteCard` gravam `cards` via
> UoW direto, sem outbox. O único evento de card é `card.invoice_due.v1` (alerta de fatura, fora do MVP).
> `EditCategoryPercentage` e `ActivateBudget` publicam `budgets.budget_activated.v1` — que é órfão;
> ao removê-lo (Task 5), remover só a chamada `Publish`, mantendo a escrita em `budgets_allocations`.

---

## 6. JORNADA 1 — Onboarding (8 etapas oficiais)

> Conduzido pelo `OnboardingAgent` → `RunOnboardingTurn` → usecases de `internal/onboarding`.
> **Mudança desta refatoração (ADR-003, Task 6):** o turno deixa de usar **tool-calling**
> (`Tools`+`ToolChoice:"auto"`, `run_onboarding_turn.go:387`) e passa a **`response_format
> json_schema` com `Strict=true`**; o `onboarding_tool_dispatcher` despacha a partir do **objeto
> estruturado** (não de `ToolCalls`). Modelo `claude-haiku-4.5` **será revalidado** sob strict
> (guard `RUN_REAL_LLM`); se quebrar, substituído por modelo elegível (RF-19). Passos/slots do
> Documento Oficial preservados 1:1.

Fluxo de tabelas por etapa (idêntico ao código atual; só muda o contrato LLM):

| Etapa (`phase`) | Mensagem do usuário | Usecases (`internal/onboarding`) | Tabelas escritas | Evento outbox |
|-----------------|---------------------|----------------------------------|------------------|---------------|
| Boas-vindas (`''`) | `Oi` | `MarkWelcomeSent`, `SetOnboardingPhase('objective')` | `(I) users`, `(I) user_identities`, `(I) onboarding_tokens` (CONSUMED), `(O) onboarding_sessions`, `(A) agent_threads`, `(A) agent_runs` | — |
| Objetivo (`objective`) | `Quero quitar minhas dívidas` | `SaveOnboardingObjective`, `SetOnboardingPhase('budget')` | `(O) onboarding_sessions` (`payload.objective`), `(A) agent_sessions.pending_action` | — |
| Orçamento (`budget`) | `4000` | `SaveOnboardingIncome`, `SuggestBudgetSplit`, `SetOnboardingPhase('cards')` | `(O) onboarding_sessions`, `(P) outbox_events`, `(A) agent_sessions` | `onboarding.income_registered` **[REMOVER órfão — renda segue em splits]** |
| Cartões (`cards`) | `Nubank 13` | `SaveOnboardingCard`, `SetOnboardingPhase('financial_plan')` | `(O) onboarding_sessions` (`payload.cards[]`), `(P) outbox_events` | `onboarding.card_registered` → **(C) cards** (consumer) |
| Plano (`financial_plan`) | `Sim` | `SaveOnboardingBudgetSplits`, `SetOnboardingPhase('first_tx')` | `(O) onboarding_sessions` (`payload.custom_split[]`), `(P) outbox_events`, `(A) agent_sessions` (limpo) | `onboarding.splits_calculated` → **(B) budgets/budgets_allocations** |
| 1º lançamento (`first_tx`) | `Mercado 120 pix` | `RecordTransaction` (via `tools.ExpenseRecorder`), `MarkFirstTransactionRecorded`, `CompleteOnboardingSession` | `(T) transactions`, `(O) onboarding_sessions` (`completed_at`, `state='active'`), `(P) outbox_events` | `onboarding.completed` → **(A) agent** (consumer) |

**UX verbatim (fallback real do código — `onboarding_scripts.go`):**

```
👋 Oi! Eu sou o *MeControla*, seu parceiro financeiro.

📊 Aqui no MeControla todo dinheiro é organizado em apenas *5 categorias*:

💰 *Custo Fixo*  ... 🎓 *Conhecimento* ... 🎉 *Prazeres* ... 🎯 *Metas* ... 🏦 *Liberdade Financeira*

🔵 *Etapa 1/4 — Objetivo*
Qual é o seu objetivo principal? (ex: quitar dívidas, fazer uma viagem, criar uma reserva)
```

> Detalhe completo das 6 mensagens (objetivo → orçamento → cartões → plano → 1º lançamento → conclusão)
> está no plano de referência `2026_06_24_arquitetura...md` §"Exemplo de Conversa Real — Onboarding".
> A refatoração **não altera o texto**; altera apenas o transporte LLM (json_schema strict) e remove o
> evento órfão `income_registered`.

---

## 7. JORNADA 2 — Operação diária

> A partir daqui o onboarding está concluído (`onboarding_sessions.state='active'`). Toda mensagem
> passa por `AgentRuntime.Execute` (grava `agent_threads`/`agent_runs`), depois `tryResumeInbound`
> (nada pendente) e `ParseInbound` (LLM, Strict=true).

### 2.1 Registrar despesa — `Mercado 120 pix`

**Caminho de código (escrita durável via kernel):**
```
ParseInbound → intent.KindRecordExpense (IsWrite()=true → kernel)
 → Engine[steps.ExpenseState].Start  (workflow="transactions_write", correlation_key="{userID}:{channel}")
 → steps: Authorize → Replay → Policy → AuditBegin → ResolveCategory → Persist → Format
      AuditBegin  → grava agent_decisions (status='pending', decided_action, prompt_sha256, llm_model)
      ResolveCategory → categories.SearchDictionaryUC (lookup "mercado" → expense.custo_fixo/supermercado)
      Persist → binding kernelPersistExpense → tools.ExpenseRecorder
              → usecases.RecordTransactionFromAgent → transactions.CreateTransaction
```
Tools/binding reais: `tools.RecordExpense` (`tools/transactions_tools.go:14`), contrato
`ExpenseRecorder` (`tools/contracts.go:109`), binding `TransactionLoggerAdapter`
(`binding/transaction_log.go:66`), persist do kernel `binding/kernel_transaction.go`.

**Tabelas:**
- `(K) workflow_runs` — run `transactions_write` (running → succeeded); `(K) workflow_steps` (7 linhas).
- `(A) agent_decisions` — decisão auditável (status `pending` → `executed`, `settled_at`, `resulting_event_id`).
- `(T) transactions` — 1 linha (`direction=outcome`, `payment_method=pix`, `amount_cents=12000`,
  `category_id`, `category_name_snapshot='custo_fixo/supermercado'`, `ref_month`, `version=1`).
- `(P) outbox_events` — `transactions.transaction.created.v1`.
- `(B) budgets`/`budgets_allocations` — atualizados pelo consumer `transactionCreatedConsumer` (`budgets/module.go:155`).
- `(A) agent_runs` — finishRun (`outcome=routed`, `duration_ms`).

**UX (verbatim — `FormatPersistedExpense`, `formatting.go:96`):**
```
💸 *Transação realizada!*
*R$ 120,00* em *Mercado*
📂 custo_fixo/supermercado
🔔 *Atualizando seu orçamento automaticamente...*
```

### 2.2 Registrar receita — `Recebi salário 4000`

Mesmo caminho do 2.1 com `intent.KindRecordIncome` (`direction=income`); tool `tools.RecordIncome`
(`transactions_tools.go:50`), mesmo `ExpenseRecorder`/`RecordTransactionFromAgent`. Tabelas idênticas
(transaction com `direction=income`), evento `transactions.transaction.created.v1`.

**UX (verbatim — `formatPersistedIncome`, `formatting.go:114`):**
```
💰 *Recebimento registrado!*
*R$ 4.000,00* de *salário*
📂 income/salario
✅ Anotei na sua conta.
```

### 2.3 Compra parcelada — `Comprei 1200 em 3x no Nubank`

Caminho: `intent.KindRecordCardPurchase` → tool `tools.RecordCardPurchase` (`transactions_tools.go:86`),
contrato `CardPurchaseLogger` (`contracts.go:134`), persist `binding/kernel_transaction.go`
(`kernelPersistCardPurchase`) → usecase `transactions.CreateCardPurchase` (gera parcelas + competências
futuras). O agente **não** calcula parcelas (regra no módulo dono — RF-24/27).

**Tabelas:** `(K) workflow_runs/steps`, `(A) agent_decisions`, `(T) transactions` (linhas de parcela com
`ref_month` futuros), `(P) outbox_events` (`transactions.card_purchase.created.v1`),
`(B) budgets` via `cardPurchaseCreatedConsumer` (`budgets/module.go:157`).

**UX (verbatim — `FormatPersistedCardPurchase`, `formatting.go:443`):**
```
💳 *Compra parcelada registrada!*
*R$ 1.200,00* em *3x* no *Nubank*
📂 custo_fixo/compras
✅ Anotei nas suas faturas.
```

### 2.4 Consultas (leitura — sem kernel, sem escrita)

Leituras passam por `WorkflowRegistry.Resolve(kind) → Tool → binding → usecase` (sem WriteGuard, sem
`workflow_runs`). Só gravam `agent_runs` (auditoria do turno).

| Mensagem | Kind | Usecase (dono) | UX (função/`formatting.go`) |
|----------|------|----------------|------------------------------|
| `Resumo do mês` | `KindMonthlySummary` | budgets `GetMonthlySummary` | `formatMonthlySummary:144` → `📊 *Resumo de 2026-06*\n• Gasto total: R$ … / planejado R$ …\n• Custo Fixo: …` |
| `Como estou?` | `KindHowAmIDoing` | budgets `GetMonthlySummary` | `formatHowAmIDoing:399` → `📊 *Como você está* (2026-06)\nVocê gastou R$ … de R$ … planejados (X%)…` (≥80% vira `⚠️ *Atenção Proativa*`) |
| `Quanto gastei em prazeres?` | `KindQueryCategory` | budgets `GetMonthlySummary` | `formatCategoryAllocation:173` → `📊 *Prazeres* (2026-06): R$ … de R$ … planejados (X% da meta).` |
| `Meus lançamentos` | `KindListTransactions` | transactions `ListTransactions` | `formatTransactionList:466` → `📋 *Lançamentos de 2026-06* (N)\n• Entradas: R$ …\n• Saídas: R$ …` |
| `Quanto recebi esse mês?` | `KindQueryIncomeSummary` | transactions `ListTransactions` (filtro income) | `formatIncomeSummary:664` → `💰 *Entradas de 2026-06*\nTotal: *R$ …*\n\nDetalhamento:\n• salário: R$ …` |
| `Meus cartões` | `KindListCards` | card `ListCards` | `formatCardList:255` → `💳 *Seus cartões* (N)\n• *Nubank* (fecha dia 13, vence dia 20)` |
| `Quantos cartões tenho?` | `KindCountCards` | card `CountCards` | `formatCardCount:345` → `💳 Você tem *1 cartão* cadastrado.` |

### 2.5 Recorrência de orçamento — `Repete meu orçamento pelos próximos 3 meses`

Caminho: `intent.KindCreateRecurring`? **Atenção:** há **dois** conceitos distintos no código —
(a) recorrência de **transação** (`transactions.CreateRecurringTemplate`, tool `tools.CreateRecurring`,
binding `RecurringCreatorAdapter`), e (b) recorrência de **orçamento** (`budgets.CreateRecurrence`,
clona `budgets_allocations` de um mês fonte para N meses). A operação oficial "repetir orçamento por N
meses" (RF-29) usa **budgets.CreateRecurrence** — porta já existente; a Task 9 adiciona a tool/workflow
no seam `buildRegistry` apontando para essa porta. O agente apenas aciona (não recalcula).

**Tabelas (recorrência de orçamento):** `(B) budgets`/`budgets_allocations` (N competências futuras),
`(A) agent_runs`. **Recorrência de transação** grava `(T) transactions_recurring_templates` + publica
`transactions.recurring_template.created.v1` — **evento órfão removido** na Task 5 (manter só a escrita
na tabela; remover o `Publish`).

**UX (recorrência de transação — verbatim `formatPersistedRecurring`, `formatting.go:518`):**
```
🔁 *Recorrência criada!*
*R$ 1.500,00* de saída mensal (dia 10)
📝 aluguel
📂 custo_fixo/moradia
```

### 2.6 Editar % de categoria pós-onboarding — `Quero 40% em metas`

Caminho: `intent.KindEditCategoryPercentage` → tool `tools.EditCategoryPercentage`, contrato
`CategoryPercentageEditor` (`contracts.go:77`) → usecase `budgets.EditCategoryPercentage` (rebalanceia
as demais para somar 100% — regra no dono). O agente não recalcula (RF-30).

**Tabelas:** `(B) budgets_allocations` (rebalance), `(A) agent_runs`. Publica
`budgets.budget_activated.v1` (`edit_category_percentage.go:123`) — **órfão; remover o Publish na Task 5**.

**UX (verbatim — `formatCategoryPercentageUpdated`, `formatting.go:330`):**
```
🎯 *Orçamento ajustado!*
Defini *Metas* com *40%* do seu planejamento. As outras categorias foram rebalanceadas pra somar 100%.
```

### 2.7 Cartões — cadastrar / atualizar (escrita simples, sem evento)

`KindCreateCard` → `tools.CreateCard` (`cards_tools.go:47`), contrato `CardCreator` → usecase
`card.CreateCard`. **Grava só `(C) cards` via UoW; não publica outbox.** `KindUpdateCard` → `card.UpdateCard`.

**UX (verbatim — `formatCreatedCard`, `formatting.go:282`):**
```
💳 *Cartão cadastrado!*
*Inter*
📅 Fecha dia 5, vence dia 12.
```
Apenas **apelido + dias** (nunca limite/banco/bandeira — privacidade RF / Documento Oficial). Erros
mapeados amigavelmente por `createCardErrorText` (`formatting.go:356`): apelido duplicado, dia inválido, etc.
(*Deletar cartão* é destrutivo → §8.4.*)

---

## 8. JORNADA 3 — HITL destrutivo (kernel durável)

> Toda operação destrutiva/sensível segue **Localizar → Exibir → Confirmar → Executar → Confirmar
> sucesso** (Documento Oficial Cap. 11; RF-31). Roda no kernel `Engine[confirmation.ConfirmState]`
> (`destructive_confirm.go:32`), que **suspende** no `confirm_gate` e **retoma** no próximo turno.
> Contrato ADR-003 (RF-38): `sim`→executa, `não`→cancela, ambíguo→re-pergunta 1×→cancela,
> TTL expirado→cancela, replay de `wamid`→não duplica.

**`ConfirmState` real (`confirmation/draft.go:92`)** — campos atuais: `OperationKind`,
`AwaitingApproval` (`AwaitingNone`/`AwaitingConfirm`), `RepromptCount`, `MessageID`, `SuspendedAt`,
`ShortCircuit`, `Expired`, `ResumeText`, `UserID`, `Channel`, `PromptText`, `Reply`, `Outcome`,
`NewAmountCents`, `CardName`, `BudgetDraftJSON`, `ResumeMessageID`, `DecisionID`,
`TargetTransactionID`, `TargetTransactionVersion`.

**Steps reais (`destructive_confirm.go:34-41`):**
`ConfirmAuthorize → ConfirmReplay → ConfirmPolicy → ConfirmAuditBegin → PrepareTarget →
ConfirmGate (SUSPENDE) → ExecuteDestructive → ConfirmFormat`.

**`OperationKind` real (`draft.go:12`):** `OperationDeleteLast`, `OperationEditLast`,
`OperationDeleteCard`, `OperationBudgetCommit`. Mapa `intentToOperationKind`
(`daily_ledger_agent.go:532`). Resolvers/executors em `binding/hitl_adapters.go`.

### 8.1 Apagar último lançamento — `Apaga o último lançamento`

**Turno 1 (suspende):**
```
ParseInbound → KindDeleteLastTransaction → op=OperationDeleteLast
 → Engine[ConfirmState].Start (workflow="destructive_confirm", correlation_key="{userID}:{channel}")
 → Authorize → Replay → Policy → AuditBegin(grava agent_decisions status='awaiting_confirmation')
   → PrepareTarget (resolver NewLastTransactionDeleterResolver, hitl_adapters.go:20):
        busca último tx → popula TargetTransactionID/Version + PromptText
   → ConfirmGate: AwaitingConfirm, persiste snapshot e SUSPENDE
```
**Tabelas turno 1:** `(K) workflow_runs` (status `suspended`, `state`=ConfirmState JSON, `suspend_reason='awaiting_input'`),
`(K) workflow_steps` (até `confirm_gate` com status `suspended`), `(A) agent_decisions`
(`status='awaiting_confirmation'`), `(A) agent_runs`.

**UX turno 1 (verbatim — `hitl_adapters.go:35`):**
```
Você deseja apagar o último lançamento: *Mercado* de R$ 120,00? Responda *sim* para confirmar ou *não* para cancelar.
```

**Turno 2 — usuário responde `sim` (resume):**
```
tryResumeInbound → continuePendingApproval (daily_ledger_agent.go:191/645)
 → Engine.Resume(correlation_key, mergePatch {"ResumeText":"sim","ResumeMessageID":wamid})
   (R-WF-KERNEL-001.7: merge-patch RFC 7386 sobre Snapshot.State — NÃO substitui estado)
 → ConfirmGate reavalia: "sim" → prossegue
 → ExecuteDestructive (NewLastTransactionDeleterExecutor, hitl_adapters.go:44)
        → tools.LastTransactionDeleter → transactions.DeleteTransaction (soft delete, version check)
 → ConfirmFormat
```
**Tabelas turno 2:** `(T) transactions` (`deleted_at` setado, `version`++), `(P) outbox_events`
(`transactions.transaction.deleted.v1`), `(B) budgets` via `transactionDeletedConsumer` (`:156`),
`(K) workflow_runs` (status `succeeded`, depois purgado pelo housekeeping), `(A) agent_decisions`
(`status='executed'`, `settled_at`), `(A) agent_runs`.

**UX turno 2 (verbatim — `formatDeletedTransaction`, `formatting.go:489`):**
```
🗑️ *Lançamento excluído!*
R$ 120,00 — Mercado (24/06/2026)
```
Cancelar/expirar (`confirm_gate.go:124`): `Ok, operação cancelada. Nada foi alterado.` /
`O tempo de confirmação expirou. A operação foi cancelada.` / ambíguo 2×:
`Não entendi sua resposta. Operação cancelada por segurança.`

### 8.2 Editar último lançamento — `Muda o último pra 90`

Igual a 8.1 com `OperationEditLast`. `NewAmountCents` populado em `daily_ledger_agent.go:622`
(`initial.NewAmountCents = parsed.Intent.AmountCents()`) e usado pelo executor
(`hitl_adapters.go:89`) → `transactions.UpdateTransaction` (lock por `version`).

**UX confirm (`hitl_adapters.go:72`):** `Você deseja atualizar o último lançamento: *Mercado* de
R$ 120,00? Responda *sim*…` → sucesso (`formatEditedTransaction`, `:503`):
```
✏️ *Lançamento atualizado!*
De R$ 120,00 para *R$ 90,00* — Mercado
```

### 8.3 `[NOVO]` Apagar/editar POR REFERÊNCIA — `Apaga o Uber` / `O Uber foi 42 e não 35`

Feature nova (Task 7, ADR-008). Estende o `destructive_confirm` com **2 steps** e tipos fechados novos.

**Código novo (tipos fechados — `confirmation/draft.go`):**
```go
// AwaitingApproval ganha AwaitingSelect [NOVO]
const (
    AwaitingNone AwaitingApproval = iota
    AwaitingConfirm
    AwaitingSelect
)
// OperationKind ganha 2 entradas [NOVO]
const (
    OperationDeleteLast OperationKind = iota + 1
    OperationEditLast
    OperationDeleteCard
    OperationBudgetCommit
    OperationDeleteByRef
    OperationEditByRef
)
type TargetCandidate struct {
    TxID        string `json:"tx_id"`
    Version     int64  `json:"version"`
    Description string `json:"description"`
    AmountCents int64  `json:"amount_cents"`
    OccurredAt  string `json:"occurred_at"`
}
// ConfirmState ganha: SearchQuery string; Candidates []TargetCandidate; NewAmount int64
```

**Porta nova no DONO do dado (`internal/transactions` — NUNCA SQL no agent):**
```go
// domain/valueobjects/search_query.go
func NewSearchQuery(s string) (SearchQuery, error) // trim, len>=2, não-vazia
// application/interfaces/transaction_repository.go (método novo)
SearchByDescription(ctx, userID, q SearchQuery, refMonth option.Option[RefMonth], limit int) ([]*Transaction, error)
// application/usecases/search_transactions.go (usecase novo)
func (uc *SearchTransactions) Execute(ctx, query, refMonth string, limit int) ([]output.Transaction, error)
// SQL no repo do dono: description ILIKE '%'||$q||'%' AND user_id=$1 AND deleted_at IS NULL
//                      ORDER BY created_at DESC LIMIT $n
```
Adapter fino no agent: contrato `TransactionSearcher` + binding `TransactionSearcherAdapter`.

**Steps novos no `destructive_confirm`:**
```
authorize → replay → policy → audit_begin
  → resolve_candidates [NOVO]  (chama TransactionSearcher; popula Candidates do snapshot)
  → select_target      [NOVO]  (FUNÇÃO PURA: 0→shortcut "não encontrei"; 1→auto; N→suspende AwaitingSelect)
  → prepare_target     (REUSA: monta PromptText a partir de Target*)
  → confirm_gate       (REUSA: AwaitingConfirm, sim/não, TTL, reprompt único)
  → execute_destructive(REUSA: executors by-ref usam Target*+version p/ lock otimista)
  → format
```
`select_target` é determinística: parseia índice 1-based do `ResumeText`, valida `1<=n<=len`, copia
`Candidates[n-1]`→`Target*`. Candidatos **persistidos no snapshot** (não re-buscados no resume —
R-WF-KERNEL-001.7), garantindo que "2" mapeie sempre ao mesmo `txID`.

**Tabelas:** `(K) workflow_runs/steps` (snapshot com `Candidates`), `(A) agent_decisions`,
e no execute: `(T) transactions` (delete/update), `(P) outbox_events`, `(B) budgets` (consumer).
**Nenhuma tabela nova** (RF-39).

**UX — 1 resultado (`Apaga o Uber`):** resolve_candidates acha 1 → select_target auto → confirm_gate:
```
Você deseja apagar o último lançamento: *Uber* de R$ 35,00? Responda *sim* para confirmar ou *não* para cancelar.
```
`sim` → `🗑️ *Lançamento excluído!*\nR$ 35,00 — Uber (…)`.

**UX — N resultados (`Apaga o mercado` → vários):** select_target suspende `AwaitingSelect` e lista
enumerada (texto determinístico novo, ex.):
```
Encontrei mais de um lançamento com "mercado". Qual deles?
1) R$ 120,00 — Mercado (24/06)
2) R$ 85,00 — Mercado Extra (22/06)
3) R$ 43,00 — Mercadinho (20/06)
Responda com o número.
```
Usuário `2` → resume aplica índice → prepare_target/confirm_gate → `sim` → exclui o item #2. Nenhuma
efetivação sem item único selecionado **e** confirmado (RF-37).

**UX — edição por referência (`O Uber foi 42 e não 35`):** `OperationEditByRef`, mesmo fluxo;
`NewAmount=4200`; confirm → `✏️ *Lançamento atualizado!*\nDe R$ 35,00 para *R$ 42,00* — Uber`.
Conflito de versão (stale) mapeado para mensagem amigável (não `auditWriteFailed`) via
`ErrTransactionVersionConflict` (`transactions/.../errors.go:11`).

### 8.4 Apagar cartão — `Apaga o Nubank`

`KindDeleteCard` → `OperationDeleteCard`. Resolver `NewCardDeleterResolver` (`hitl_adapters.go:99`),
executor `NewCardDeleterExecutorFn` (`:128`) → `card.SoftDeleteCard` (grava `(C) cards.deleted_at`,
sem outbox).

**UX confirm (`hitl_adapters.go:120`):** `Você deseja remover o cartão *Nubank*? Responda *sim*…`
→ sucesso (`formatDeletedCard`, `formatting.go:322`): `🗑️ *Cartão apagado!*\nRemovi o *Nubank* do seu cadastro.`

### 8.5 Commit de orçamento (gate sensível) — `Ativar orçamento`

`OperationBudgetCommit`. Resolver `NewBudgetCommitResolver` (`hitl_adapters.go:143`), executor
`NewBudgetCommitExecutor` (`:173`) → `budgets.ActivateBudget`. Gate de budget no ponto de commit
(ADR-004). **UX confirm (`hitl_adapters.go:152`):**
```
Você deseja ativar o orçamento de *R$ 4.000,00* (2026-06) com as alocações abaixo? Responda *sim*…

💰 Custo Fixo: R$ 2.000,00
🎓 Conhecimento: R$ 300,00
🎉 Prazeres: R$ 500,00
🎯 Metas: R$ 700,00
🏦 Liberdade Financeira: R$ 500,00
```
**Tabelas:** `(B) budgets` (`state=2 active`, `activated_at`), `(B) budgets_allocations`,
`(K) workflow_runs/steps`, `(A) agent_decisions`. Publica `budgets.budget_activated.v1`
(**órfão → remover Publish na Task 5**, mantendo a ativação).

---

## 9. JORNADA 4 — `[NOVO]` Plano multi-tool 1..N

> Task 8 / ADR-004. Uma única mensagem pode gerar 1..N intents determinísticos.
> `paguei 50 no mercado e quanto gastei esse mês?` → `[RecordExpense, MonthlySummary]`.

**Código novo:**
```go
type IntentStep struct { Intent intent.Intent; Confidence valueobjects.Confidence }
type IntentPlan struct { Steps []IntentStep } // 1..N, ordem do parse
type PlanState struct {
    Steps   []IntentStep
    Cursor  int       // resume continua daqui
    Replies []string  // agregação determinística
}
type PlanExecutor interface { Execute(ctx, in PlanInput) (RouteResult, error) }
```
`ParseInbound` ganha campo `plan: [{...}]` opcional (ausência = plano de 1, idêntico ao atual).
`PlanExecutor` é **workflow durável do kernel** (`Definition[PlanState]`): cada passo executa pelo
caminho existente (kernel write / destructive_confirm); passo destrutivo suspende o **plano inteiro**
e o resume continua do `Cursor`. Short-circuit em falha dura de escrita; agregação por junção das
`Replies` na ordem. Condição de parada = função pura (sem LLM — RF-10).

**Durabilidade condicional:** plano **só-leitura** roda em memória (sem `workflow_runs`); durável
apenas se `≥1` passo `intent.Kind.IsWrite()`.

**Idempotência por passo (migration 000021):** `agent_decisions` ganha `step_index int DEFAULT 0`;
índice único vira `(user_id, channel, message_id, step_index)`. Ações single = `step_index=0` (igual
ao atual); plano com N escritas usa `step_index` 0..N-1 — replay do mesmo `wamid` não duplica nenhuma
mutação.

**UX (agregada determinística):**
```
💸 *Transação realizada!*
*R$ 50,00* em *Mercado*
📂 custo_fixo/supermercado
🔔 *Atualizando seu orçamento automaticamente...*

📊 *Resumo de 2026-06*
• Gasto total: R$ 1.250,00 / planejado R$ 4.000,00
• Custo Fixo: R$ 800,00 / R$ 2.000,00
…
```

---

## 10. JORNADA 5 — Casos especiais / matriz de decisão (RF-33/34)

Cada falha possível tem um handler tipado (princípio "0 falso positivo"). Strings verbatim:

| Caso | Onde | UX (verbatim) |
|------|------|---------------|
| Falta categoria | `text.go:8` `categoryNoHintText` | `Pra registrar certinho, me diz em qual categoria você quer anotar isso? 🙂` |
| Categoria ambígua | `formatting.go:589` `formatCategoryAmbiguous` | `Encontrei mais de uma categoria parecida. Qual delas você quer usar?\n• …\nÉ só me dizer o nome. 🙂` |
| Categoria precisa confirmação | `formatting.go` `formatCategoryNeedsConfirmation` | `Acho que isso entra em *Prazeres*. Posso registrar assim? Se não for, me diz a categoria certa. 🙂` |
| Categoria não encontrada | `formatCategoryNotFound` | `Não encontrei a categoria "x". Pode reformular ou me dizer outra categoria? 🙂` |
| Falta valor (compra parcelada) | `formatting.go:574` `formatCardPurchaseAmountMissing` | `Qual foi o valor total da compra parcelada? 🙂` |
| Cartão não encontrado | `formatting.go:379` `formatCardNotFound` | `💳 Não encontrei um cartão chamado "x"… Quer cadastrá-lo primeiro pra eu acompanhar a fatura?` |
| Cartão ambíguo | `formatting.go:338` `formatCardAmbiguous` | `💳 Encontrei mais de um cartão parecido… Me diz o nome exato pra eu não errar? 🙂` |
| Baixa confiança (write < limiar) | `intent_router.go:27` `policyLowConfidenceText` | `Não tenho certeza se entendi direito pra registrar isso. Pode reescrever com mais detalhes…? 🙂` |
| Replay (wamid já processado) | `intent_router.go:30` `alreadyProcessedText` | `Essa mensagem já foi processada ✅` |
| Falha de escrita/auditoria | `intent_router.go:32` `auditWriteFailedText` | `Não foi possível processar sua mensagem agora. Pode tentar de novo em instantes? 🙏` |
| Erro de integração (usecase) | `text.go` `FallbackUsecaseError` | `Tive uma instabilidade para consultar isso agora. Tente de novo em instantes 🙏` |
| Não entendi (parse inválido) | `text.go` `FallbackParseError` | `Não entendi direito. Pode reformular? Posso te ajudar com cartões, orçamento e lançamentos.` |
| Sem orçamento ativo (editar %) | `text.go` `budgetNotActiveText` | `Você ainda não tem um orçamento ativo neste mês pra eu ajustar. Quer configurar um agora? 🙂` |
| Mensagem vazia | `intent_router.go` `fallbackMissingText` | `Não recebi nenhuma mensagem. Me conta o que você precisa nas suas finanças 😊` |
| Conversa livre (`KindUnknown`) | `compose_conversational_reply.go` | resposta livre via fallback chain (LLM — exceção sancionada RF-10) |

**Desfazer (`Desfaz isso`) — fora do MVP (RF-34):** o agente **redireciona** para apagar/editar o
último lançamento com confirmação HITL (§8.1/8.2); nunca reverte automaticamente.

**Continuidade sem orçamento (RF-26):** registrar despesa/receita funciona mesmo sem `budgets` ativo;
a estrutura mínima é criada pela porta do dono, sem bloquear o usuário.

---

## 11. O que muda em cada Task (mapa Task → código)

| Task | Mudança | Arquivos-chave |
|------|---------|----------------|
| 1.0 Data-boundary gate | `scripts/ci/agent-data-boundary.sh` falha build em SQL direto / import de repo de outro BC dentro de `internal/agent` | novo script; verde no estado atual |
| 2.0 Eliminar Telegram (código) | deletar `internal/platform/telegram/**`, consumer telegram do agent (`module.go:210,699`), `notification/adapters/telegram.go`, `identity/.../telegram_router.go`, `cmd/server/telegram_wiring.go`, `TELEGRAM_*`/`ONBOARDING_TELEGRAM_*` (`configs/config.go`, `.env.example`), `SourceTelegram`, `ChannelTelegram()` | ADR-005 |
| 3.0 Migration 000020 | DROP `onboarding_tokens.telegram_external_id` + índice; CHECK canal → `('whatsapp')` em `channel_processed_messages`/`user_identities`/`onboarding_sessions`; `DELETE … WHERE channel='telegram'` (dedup residual). **Pré-deploy fail-fast:** `count(*) telegram` em `user_identities`/`onboarding_sessions` deve ser 0 | ADR-005 |
| 4.0 Kernel único | remover `kernelEnabled`/`EnableKernel`/`parity_test`/`TransactionsWriteEnabled` (kernel sempre-on; dep ausente = falha de boot). `continuePendingExpenseConfirmationLegacy` **já não existe** | ADR-006 (PRÉ-1..4) |
| 5.0 Órfãos cross-module | remover `agent.intent.{rejected,executed}`, `budgets.budget_activated` (Publish em `activate_budget`/`edit_category_percentage:123`), `recurring_template.{created,updated,deleted}`, `onboarding.income_registered` (`save_onboarding_income:79`), pipeline `external.expense.v1` (consumer+`IngestExternalExpense`+command+strategy+registro `budgets/module.go:153`+testes). **MANTER `card_purchase.deleted`** | ADR-007 (lista corrigida) |
| 6.0 Strict=true + classes + onboarding json_schema | `parse_inbound.go:97` `Strict:true`; `ParseIntentJSONSchema` com `required` completo; `ClassRouter`/`LLMClass` + 3× `buildLLMChain`; `run_onboarding_turn.go` tool-calling→json_schema; guard `RUN_REAL_LLM` | ADR-002/003 |
| 7.0 By-ref + desambiguação | `SearchTransactions`/`SearchByDescription`/`NewSearchQuery` (transactions); `TransactionSearcher`+binding; steps `resolve_candidates`/`select_target`; `AwaitingSelect`/`TargetCandidate`/`OperationDeleteByRef`/`OperationEditByRef`; kinds `KindDeleteTransactionByRef`/`KindEditTransactionByRef` | ADR-008 |
| 8.0 Plano multi-tool | schema `plan`; `PlanExecutor` (kernel `Definition[PlanState]`); migration 000021 `step_index` | ADR-004 |
| 9.0 Operação diária via portas | tool/workflow para `budgets.CreateRecurrence` (RF-29) e `budgets.EditCategoryPercentage` (RF-30) no `buildRegistry`; consultas compostas; casos especiais | — |

---

## 12. Gates de verificação (rodar antes de cada merge)

```bash
# Fronteira de dados do agent (Task 1.0) — deve sair vazio
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
  internal/agent/application/ internal/agent/infrastructure/binding/ \
  | grep -v "infrastructure/repositories/postgres" && echo FAIL || echo OK
grep -rn --include="*.go" --exclude="*_test.go" \
  "internal/\(transactions\|budgets\|card\|categories\|onboarding\)/infrastructure/repositories" \
  internal/agent/ && echo FAIL || echo OK

# Telegram zerado (Tasks 2/3)
grep -rni "telegram" internal/ configs/ cmd/ .env.example migrations/ | grep -v "000020" && echo FAIL || echo OK

# Switch de domínio não cresce (R-AGENT-WF-001.1)
# Zero comentários em Go de produção (R-ADAPTER-001.1)
# Estados como tipos fechados (R-WF-KERNEL-001.3 / R-AGENT-WF-001.3)
# Cardinalidade de métrica (sem user_id/category_id/correlation_key/message_id como label)
# deadcode (instalar antes): deadcode ./...  →  confirma RF-40/órfãos
go install golang.org/x/tools/cmd/deadcode@latest
```
Suíte: `task test` (unit, R-TESTING-001 testify/suite whitebox), `task test:integration`
(`testcontainers-go`: `SearchByDescription`, migration 000020 up/down, resume by-ref), E2E Godog
WhatsApp-only + guard `RUN_REAL_LLM` (Strict=true por classe).

---

## 13. Decisões fechadas (2026-06-25) + risco operacional

Todas as questões em aberto foram resolvidas em rodada de múltipla escolha, **todas no caminho
recomendado, sem flexibilização**:

1. **`external.expense.v1` → REMOVER o pipeline** (consumer + `IngestExternalExpense` + command +
   strategy + registro `budgets/module.go:153` + testes). Cumpre RF-41 "consumer-sem-producer = 0" e
   "0 código morto"; nem o Documento Oficial nem o plano arquitetural preveem produtor externo no MVP.
   Entra na Task 5.
2. **RF-40 → instalar `deadcode` + gate.** `go install golang.org/x/tools/cmd/deadcode@latest` e rodar
   `deadcode ./...` na Task 4. Se nada apontar no fluxo de configuração de orçamento, **RF-40 fica
   satisfeito** (sem remoção), com evidência no relatório.
3. **`ConfigureBudgetConversation` (RF-10) → MIGRAR para parse estruturado** (Structured Output
   `Strict=true` + execução determinística). LLM permanece exclusivo do parse — **sem** nova exceção a
   RF-10. Entra na Task 6/9. Garante a meta "100% das ações de domínio nascem de Structured Output".
4. **Onboarding sob Strict=true (RF-19) → guard real-LLM decide.** Rodar `RUN_REAL_LLM` com
   json_schema estrito no `claude-haiku-4.5`; mantém se passar, troca por modelo elegível se quebrar.
   Decisão por evidência (como no parse). Entra na Task 6.

**Risco operacional remanescente (não é decisão — é execução):** Migration 000020 exige verificação
**fail-fast** antes do deploy — `SELECT count(*) FROM {user_identities,onboarding_sessions} WHERE
channel='telegram'` deve ser 0; se > 0, abortar e escalar.

**Nenhuma questão de produto/arquitetura em aberto.**

---

## 14. Resumo da fidelidade (checklist 0-falso-positivo)

- ✅ Toda string de UX é verbatim do código (`file:line` citados).
- ✅ Todo caminho de código usa símbolos reais verificados (tools, bindings, usecases, steps).
- ✅ Todas as tabelas têm DDL real (migration + colunas-chave).
- ✅ Todo evento foi classificado por **constante de event-type** (producer×consumer), não por nome de arquivo.
- ✅ Falsos positivos do spec original (card_purchase.deleted, external.expense, NewAmount bug,
  Legacy symbol) **corrigidos** no PRD/techspec e marcados aqui.
- ✅ Features novas (by-ref, plano multi-tool, Strict=true, ClassRouter) marcadas `[NOVO]` com o
  código-alvo; nada apresentado como já existente.
