<!-- spec-hash-prd: 96940ca79c35758e9004066b267dbad5f723f9e453eb66fa3714f568b993dab9 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Refatoração Canônica do `internal/agent` e Canal WhatsApp Oficial

> PRD consumido: `.specs/prd-refatoracao-agent-canonico/prd.md` (spec-version 2, RF-01..45).
> Skills obrigatórias: `go-implementation` (Etapas 1–5 + R0–R7) e `mastra` (R-AGENT-WF-001).
> `go.mod`: **Go 1.26.4** — todos os recursos R7 (generics, `slices`/`maps`, `errors.Join`, `min/max`,
> `cmp`, range-over-int) estão disponíveis.

## Resumo Executivo

Esta refatoração consolida o `internal/agent` em torno de um **único canal (WhatsApp Oficial da
Meta)**, formaliza o **contrato LLM↔domínio como Structured Output tipado com `Strict=true`**,
introduz **roteamento de modelo por classe de tarefa** e **editar/apagar por referência com
desambiguação**, e **elimina** Telegram, caminhos *legacy* do kernel, código morto e eventos órfãos
— com motivadores documentados aqui e em ADRs.

A base já é 100% aderente ao modelo Mastra (Agent→Workflow→Tool→binding→usecase, Thread/Run
auditável, kernel genérico com suspend/resume durável, state-as-type). A estratégia é **incremental e
não-destrutiva do que funciona**: reusar ao máximo os primitivos existentes (`Engine[S]`,
`destructive_confirm`, `ConfirmState`, `WriteGuard`, bindings) e remover o que sobrou da fase de
transição (flag `kernelEnabled`, `continuePendingExpenseConfirmationLegacy`, `parity_test`,
`EnableKernel`, fallback morto de budget). Nenhuma capacidade nova entra como `case intent.Kind` no
switch de domínio — tudo via registry/workflow/mapa de `OperationKind` (R-AGENT-WF-001.1). DMMF é
aplicado onde carrega invariante: novos estados (`AwaitingSelect`), operações (`OperationDeleteByRef`,
`OperationEditByRef`), classe de modelo e plano multi-tool são **tipos fechados**; toda decisão de
seleção/parada é **função pura determinística**, sem LLM no meio (RF-10).

## Arquitetura do Sistema

### Visão Geral dos Componentes

Pipeline canônico (inalterado no esqueleto, reforçado nos contratos):

```
WhatsApp Cloud API (Meta)
  → internal/platform/whatsapp (webhook: verify, HMAC-SHA256, dedup wamid, dispatcher)
  → identity.EstablishPrincipal (E164 → user_id)
  → outbox: agent.whatsapp.inbound.v1
  → internal/agent: WhatsAppInboundConsumer
  → IntentRouter.RouteWhatsApp → AgentRuntime.Execute (Thread→Run)
  → DailyLedgerAgent.Handle
       1) tryResumeInbound (pending expense → pending approval/seleção → budget session)
       2) ParseInbound (ÚNICO call-site LLM; Structured Output Strict=true → plano 1..N intents)
       3) executor determinístico → IntentRegistry.Resolve(kind) → Workflow → Tool → binding → usecase
  → resposta → Graph API /messages (texto) via WhatsAppOutbound
```

**Componentes NOVOS:**

- `LLMClass` (tipo fechado) + `ClassRouter` — resolve a chain (primário+fallback+breaker) por classe
  de tarefa (`parse`, `onboarding`, `conversational`). Em `internal/agent/...services` + config.
- Plano multi-tool: extensão do `ParseInbound` para emitir `IntentPlan` (1..N intents) + `PlanExecutor`
  determinístico em `internal/agent/application/workflow`.
- `SearchTransactions` (usecase) + `SearchByDescription` (repo) + `NewSearchQuery` (VO) no
  **`internal/transactions`** (porta de entrada nova, dono do dado).
- `TransactionSearcher` (tool contract) + `TransactionSearcherAdapter` (binding) no agent.
- Steps `resolve_candidates` + `select_target` no `destructive_confirm`; `AwaitingSelect`,
  `TargetCandidate`, `OperationDeleteByRef`/`OperationEditByRef` em `confirmation`.
- Kinds `KindDeleteTransactionByRef`/`KindEditTransactionByRef` (state-as-type) + entradas no mapa
  `intentToOperationKind`.
- Gate de CI `scripts/ci/agent-data-boundary.sh` (fronteira de dados).
- Migration `000020_drop_telegram_channel` (up/down).

**Componentes MODIFICADOS:** `parse_inbound.go` (Strict=true, plano), `module.go`/`config.go` (classes
de modelo, remoção Telegram), `daily_ledger_agent.go` (remoção legacy, mapa by-ref),
`destructive_confirm.go` (steps novos), `intent.go`/`prompts.go` (kinds + schema by-ref).

**Componentes REMOVIDOS:** ver §"Eliminação" e ADR-005/006/007.

## Design de Implementação

### Interfaces Chave

#### 1. Roteamento de modelo por classe (ADR-002)

```go
type LLMClass int

const (
	LLMClassParse LLMClass = iota + 1
	LLMClassOnboarding
	LLMClassConversational
)

type ClassRouter interface {
	Interpreter(class LLMClass) (usecases.IntentInterpreter, bool)
}
```

`ClassRouter` é construído em `module.go` a partir de uma config por classe. Cada classe resolve para
um `FallbackChain` próprio (primário + fallbacks) com `CircuitBreaker` independente — reusa
`buildLLMChain` existente, agora chamado 3×. `ParseInbound` recebe o interpreter de `LLMClassParse`;
onboarding recebe `LLMClassOnboarding`; `ComposeConversationalReply` recebe `LLMClassConversational`.

Config (substitui campos planos atuais em `AgentConfig`):

```go
type AgentModelClassConfig struct {
	Primary   string   // ex: AGENT_LLM_PARSE_PRIMARY_MODEL
	Fallbacks []string // ex: AGENT_LLM_PARSE_FALLBACK_MODELS (csv)
	MaxTokens int
}
// AgentConfig ganha: Parse, Onboarding, Conversational AgentModelClassConfig
// Mantém: OpenRouter{BaseURL,APIKey}, HTTPReferer, XTitle, Temperature, RequestTimeout,
//         Circuit{Failures,Window,Cooldown}, PolicyMinConfidence, MaxInputChars.
```

Compat: as env atuais (`AGENT_LLM_PRIMARY_MODEL`, `AGENT_LLM_FALLBACK_MODELS`,
`AGENT_ONBOARDING_LLM_MODEL`) viram a config da classe correspondente; novas envs por classe são
adicionadas. Migração de env documentada em ADR-002.

#### 2. Structured Output Strict=true (ADR-003)

```go
// parse_inbound.go — schema passa a Strict=true
schema: &interfaces.JSONSchemaSpec{
	Name:   "mecontrola_parse_intent",
	Strict: true,                         // era false
	Schema: prompting.ParseIntentJSONSchema(),
}
```

Restrição derivada: com `Strict=true`, o JSON Schema do OpenRouter exige `additionalProperties:false`
(já presente) e que **todas as propriedades estejam em `required`** (hoje só `kind,confidence`). O
schema de parse será ajustado para `required` completo com tipos que aceitam vazio/zero (strings
default `""`, inteiros default `0`) — o `Intent` já trata zero-values via smart constructors. Modelos
elegíveis limitam-se aos que suportam structured outputs estritos; haiku/gpt-5-nano ficam inelegíveis
(RF-18/19). Guard real-LLM (`RUN_REAL_LLM`) valida cada modelo configurado por classe antes de
promover a primário.

#### 3. Plano multi-tool 1..N (ADR-004)

```go
type IntentStep struct {
	Intent     intent.Intent
	Confidence valueobjects.Confidence
}

type IntentPlan struct {
	Steps []IntentStep // 1..N, ordem determinística do parse
}

type PlanExecutor interface {
	Execute(ctx context.Context, in PlanInput) (RouteResult, error)
}
```

`ParseInbound` passa a poder retornar `IntentPlan` (schema ganha `plan: [{...}]` opcional; ausência =
plano de 1, idêntico ao atual — RF-01). O `PlanExecutor` é modelado como **workflow durável do
kernel** (`Definition[PlanState]`), onde `PlanState` carrega os intents ordenados, um **cursor** e as
`Reply` acumuladas. Cada passo do plano executa um intent pelo caminho existente
(`dispatchWrite`/registry); um passo **destrutivo/sensível** delega ao sub-fluxo `destructive_confirm`
que **suspende** o snapshot no `confirm_gate`. Como o plano é durável, a suspensão de um passo
**suspende o plano inteiro**; ao confirmar, o `Resume` retoma **a partir do cursor** os passos
restantes (decisão: "suspende o plano inteiro"). Há **short-circuit** em falha dura de escrita e
**agregação determinística** das respostas (junção das `Reply` na ordem). Condição de parada = função
pura sobre o estado (sem LLM — RF-06/RF-10). Plano de 1 passo NÃO altera o fluxo atual.

```go
type PlanState struct {
	Steps   []IntentStep // ordenado, persistido no snapshot (fonte única — R-WF-KERNEL-001.7)
	Cursor  int          // próximo passo a executar; resume continua daqui
	Replies []string     // agregação determinística
	// ... ids/canal para auditoria
}
```

**Eficiência (durabilidade condicional):** um plano **somente-leitura** (todos os passos são reads)
NÃO precisa de snapshot durável — executa em memória, sem custo de persistência no kernel. A
durabilidade (`Definition.Durable=true`) é ligada **apenas quando o plano contém ≥1 passo de escrita
ou destrutivo** (que pode suspender via HITL). Decisão determinística pura sobre os kinds do plano
(`intent.Kind.IsWrite()`), sem LLM.

#### 4. Busca por referência no transactions (ADR-008)

```go
// internal/transactions/domain/valueobjects/search_query.go
type SearchQuery struct{ value string }
func NewSearchQuery(s string) (SearchQuery, error) // não-vazia, len>=2, trim

// internal/transactions/application/interfaces/transaction_repository.go (novo método)
SearchByDescription(ctx context.Context, userID uuid.UUID, q valueobjects.SearchQuery,
	refMonth option.Option[valueobjects.RefMonth], limit int) ([]*entities.Transaction, error)

// internal/transactions/application/usecases/search_transactions.go (novo usecase)
func (uc *SearchTransactions) Execute(ctx context.Context, query string, refMonth string, limit int) ([]output.Transaction, error)
```

SQL no repo do dono: `description ILIKE '%'||$q||'%' AND user_id=$1 AND deleted_at IS NULL
ORDER BY created_at DESC LIMIT $n` (n pequeno, ex. 10). Validação no smart constructor (R-TXN-002).
**Proibido** filtrar/loop no agent (R-AGENT-WF-001.2). Adapter fino no agent:

```go
// agent/application/tools/contracts.go
type TransactionSearcher interface {
	Execute(ctx context.Context, in TransactionSearchInput) (TransactionSearchResult, error)
}
type TransactionSearchInput struct { UserID, Query, RefMonth string; Limit int }
type TransactionSearchResult struct { Candidates []TransactionView } // já tem ID, Version, AmountCents, Description, OccurredAt
```

#### 5. Desambiguação reusando `destructive_confirm` (ADR-008)

Extensão de `confirmation.ConfirmState` (state-as-type):

```go
// novos tipos fechados
const ( // AwaitingApproval ganha AwaitingSelect
	AwaitingNone AwaitingApproval = iota
	AwaitingConfirm
	AwaitingSelect
)
const ( // OperationKind ganha 2 entradas
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
// ConfirmState ganha: SearchQuery string; Candidates []TargetCandidate;
//   TargetTxID string; TargetVersion int64; TargetDesc string; TargetAmount int64; NewAmount int64
```

Sequência do workflow `destructive_confirm` para by-ref (steps novos em **negrito**):

```
authorize → replay → policy → audit_begin
  → resolve_candidates  (NOVO: chama TransactionSearcher; popula Candidates)
  → select_target       (NOVO: 0→shortcut "não encontrei"; 1→auto; N→suspende AwaitingSelect, lista enumerada)
  → prepare_target      (REUSA: monta PromptText do confirm a partir de Target*)
  → confirm_gate        (REUSA: AwaitingConfirm, sim/não, TTL, reprompt único — contrato ADR-003)
  → execute_destructive (REUSA: executors by-ref usam Target* + version p/ optimistic lock)
  → format
```

`select_target` é **função pura determinística**: parseia índice 1-based do `ResumeText`, valida
`1<=n<=len(Candidates)`, copia `Candidates[n-1]` para `Target*`; inválido → reprompt único depois
cancela (espelha `confirm_gate`). Candidatos são **persistidos no snapshot** e NÃO re-buscados no
resume (fonte única de verdade — R-WF-KERNEL-001.7), garantindo que "2" mapeie sempre ao mesmo `txID`.

#### 6. Gate de fronteira de dados (ADR-001)

```bash
# scripts/ci/agent-data-boundary.sh — falha o build (exit 1) em violação
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
  internal/agent/application/ internal/agent/infrastructure/binding/ \
  | grep -v "infrastructure/repositories/postgres" && exit 1
# import de repo/infra de outro BC dentro de internal/agent
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "internal/\(transactions\|budgets\|card\|categories\|onboarding\)/infrastructure/repositories" \
  internal/agent/ && exit 1
```

### Modelos de Dados

- **Sem nova tabela.** A desambiguação e o HITL reusam `workflow_runs`/`workflow_steps` (snapshot do
  kernel) — `ConfirmState` serializa os novos campos em JSON (RF-39: nenhuma tabela do agent removida
  sem evidência de não-uso; nenhuma nova criada).
- **Migration `000021_agent_decisions_step_index`** (ADR-004): adiciona `step_index INT NOT NULL
  DEFAULT 0` em `agent_decisions` e troca o índice único `(user_id, channel, message_id)` →
  `(user_id, channel, message_id, step_index)`, para idempotência **por passo** do plano multi-tool
  (uma mensagem com N escritas). Ações single usam `step_index=0` (idêntico ao atual). `down` reverte.
- **Onboarding migra de tool-calling → json_schema** (ADR-003): `run_onboarding_turn` deixa de usar
  `Tools`/`ToolChoice`; passa a `response_format json_schema` com `Strict=true`; o
  `onboarding_tool_dispatcher` despacha a partir do objeto estruturado (não de `ToolCalls`). Preservar
  os mesmos passos/slots do Documento Oficial.
- **Migration `000020_drop_telegram_channel`** (up/down completos no ADR-005): drop coluna
  `onboarding_tokens.telegram_external_id` + índice parcial; recria 3 CHECK constraints
  (`channel_processed_messages`, `user_identities`, `onboarding_sessions`) de
  `IN ('whatsapp','telegram')` → `IN ('whatsapp')`. **Premissa aceita: zero usuários Telegram reais em
  produção** (canal era piloto). A migration limpa apenas dedup residual
  (`DELETE FROM channel_processed_messages WHERE channel='telegram'`) e aplica as constraints.
  **Verificação pré-deploy obrigatória (fail-fast)**: rodar contagem
  `SELECT count(*) FROM {user_identities,onboarding_sessions} WHERE channel='telegram'` antes do
  deploy; se > 0, abortar e escalar (a premissa foi violada) — não aplicar a migration às cegas.
- **Schema de parse**: `required` completo para satisfazer `Strict=true` (ADR-003).

### Endpoints de API

- **Webhook WhatsApp** (inalterado): `GET /whatsapp/webhook` (verify), `POST /whatsapp/webhook`
  (inbound, HMAC-SHA256). Egress: `POST /{phone_number_id}/messages` (Graph API v18).
- **Removido**: rota de webhook Telegram (`telegram_router.go`, `composeTelegramWebhookRouter`).

## Pontos de Integração

- **Meta WhatsApp Cloud API** (já integrado): verify token (constant-time), assinatura
  `X-Hub-Signature-256` (HMAC-SHA256), dedup por `wamid` em `channel_processed_messages`, Graph API
  para envio. Sem mudança estrutural; apenas canal único.
- **OpenRouter** (`/api/v1/chat/completions`): `response_format: json_schema` com `strict:true` por
  classe. Auth Bearer. Tratamento de erro: fallback chain + circuit breaker por classe; exaustão →
  resposta determinística de erro/clarificação (nunca execução adivinhada — RF-08).
- **Módulos donos** via porta de entrada (binding→usecase): transactions, card, budgets, categories,
  onboarding. **Nenhum acesso a tabela de outro BC** (gate CI — RF-18).

## Abordagem de Testes

### Testes Unitários

- Padrão canônico `testify/suite` whitebox + `fake.NewProvider()` + mocks `mockery` (R-TESTING-001).
- `select_target`/`resolve_candidates`: tabela de cenários puros (0/1/N candidatos, índice válido/
  inválido, reprompt, resume). Sem LLM, sem IO.
- `PlanExecutor`: plano 1 (não-regressão), plano N em ordem, short-circuit em falha de escrita,
  agregação determinística.
- `ParseInbound` Strict=true: schema com `required` completo; decode de plano; confidence clamp.
- `ClassRouter`: resolução por classe, fallback, breaker independente.
- `NewSearchQuery`: invariantes (vazio, len<2).
- Confirmação by-ref: optimistic-lock conflict mapeado para mensagem amigável (não `auditWriteFailed`).

### Testes de Integração

> Critérios atendidos (≥2): fronteiras de IO críticas (Postgres: search ILIKE, kernel snapshot,
> migration ALTER) e risco de migração com data-cleanup. **Adotar integration tests.**

- `testcontainers-go` (`//go:build integration`): `SearchByDescription` (ILIKE, limit, isolamento por
  user), migration `000020` up/down + pré-cleanup de `channel='telegram'`, resume durável do
  `destructive_confirm` by-ref (suspend AwaitingSelect → resume índice → confirm → execute).

### Testes E2E

- WhatsApp-only (remover `f04_telegram_agent_flow.feature` e steps). Novos cenários Godog:
  `Apaga o Uber` (1 resultado→confirma→apaga), `Apaga o mercado` (N→escolhe→confirma),
  `O Uber foi 42 e não 35` (edita por ref), plano multi-tool (`paguei 50 no mercado e quanto gastei?`).
- Guard real-LLM (`RUN_REAL_LLM`): valida Strict=true por modelo de classe (parse/onboarding).

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Gate de fronteira de dados** (ADR-001) — script CI + verde no estado atual (blindagem antes de
   tudo).
2. **Eliminação Telegram** (ADR-005): editar shared → deletar Telegram-only → migration 000020 →
   ajustar/remover testes → `go build/test`. (Maior superfície; primeiro para reduzir ruído.)
3. **Kernel caminho único** (ADR-006): **remover a flag `WorkflowKernelConfig.TransactionsWriteEnabled`**
   (kernel sempre-on; deps ausentes = **falha de boot**, nunca fallback silencioso) → remover
   `EnableKernel`, `kernelEnabled`, `continuePendingExpenseConfirmationLegacy`, `parity_test`
   (migrando casos de resume p/ suite kernel-only), fallback morto de budget.
4. **Eventos órfãos** (ADR-007): remover por publisher confirmado (intent.rejected/executed,
   budget_activated, recurring_template ×3) com guarda por constante.
5. **Structured Output Strict=true + classes de modelo** (ADR-002/003): schema `required` completo,
   `ClassRouter`, config por classe, guard real-LLM, **migração do onboarding tool-calling → json_schema**.
6. **Busca por referência + desambiguação** (ADR-008): VO+repo+usecase no transactions → binding →
   steps `resolve_candidates`/`select_target` → kinds/operations → executors by-ref.
7. **Plano multi-tool** (ADR-004): schema de plano → `PlanExecutor` (workflow durável do kernel) →
   agregação; **migration 000021 (replay por `step_index`)** antes de habilitar planos multi-escrita.
8. **Recorrência de orçamento + editar % categoria pós-onboarding**: já têm porta
   (`CreateRecurrence`, `EditCategoryPercentage`); adicionar tool/workflow no seam `buildRegistry`.

### Dependências Técnicas

- Templates de mensagem da Meta NÃO são necessários neste MVP (alertas fora de escopo).
- `deadcode`/`staticcheck` instalados para gate de resíduo (ADR-006/007).
- Modelos OpenRouter elegíveis a Strict=true definidos antes de §5 (guard real-LLM).

## Monitoramento e Observabilidade

- Métricas existentes preservadas; novas com **cardinalidade controlada** (RF-42): labels apenas
  `workflow`, `step`, `status`, `outcome`, `kind`, `class`, `model`. **Proibido**
  `user_id`/`category_id`/`correlation_key`/`message_id`.
- Novos sinais: `agent_llm_class_total{class,model,outcome}`, `agent_plan_steps_total{outcome}`,
  `agent_target_select_total{outcome}` (found/none/multi/reprompt/cancel), reuso de
  `agent_run`/`workflow` metrics do kernel.
- Stack `otel-lgtm`; cada execução é Run auditável (Thread→Run) com `decision_id` nas escritas.

## Considerações Técnicas

### Decisões Chave (ADRs)

- **ADR-001** — Gate de CI de fronteira de dados do agent (blindar RF-18/20).
- **ADR-002** — Roteamento de modelo por classe de tarefa (RF-14..16).
- **ADR-003** — Structured Output `Strict=true` em todas as classes estruturadas; modelos elegíveis
  via guard real-LLM; onboarding deixa de usar haiku (RF-07/08/18/19).
- **ADR-004** — Plano determinístico multi-tool 1..N sobre o caminho existente (RF-12).
- **ADR-005** — Eliminação total do canal Telegram (código + schema via migration ALTER) (RF-02..05).
- **ADR-006** — Kernel como caminho único; remoção do legacy de resume e do `parity_test` com
  pré-condições (RF-32/36).
- **ADR-007** — Remoção de eventos órfãos (inclusive cross-module) com guarda por constante de
  event-type (RF-38/41).
- **ADR-008** — Editar/apagar por referência com desambiguação reusando `destructive_confirm`
  (RF-36/37) + contrato HITL ADR-003 as-is (RF-38).

### Riscos Conhecidos

- **Strict=true quebra modelos**: mitigado por guard real-LLM por classe; haiku/gpt-5-nano fora.
  Onboarding precisa de modelo elegível novo — validar antes do cutover.
- **Migração do onboarding tool-calling → json_schema** (ADR-003): reescreve fluxo que já funciona;
  mitigado preservando passos/slots do Documento Oficial + testes de paridade + guard real-LLM;
  rollback reverte o interpreter de onboarding para tool-calling.
- **Replay de plano multi-write** (ADR-004): idempotência precisa ser por passo; mitigado pela
  migration 000021 (`step_index` no índice único de `agent_decisions`); single = `step_index=0`.
- **Migration ALTER bloqueia se houver `channel='telegram'`**: premissa de zero usuários Telegram +
  **verificação pré-deploy fail-fast** (abortar se contagem > 0); limpeza de dedup residual; rollback
  `down` recria coluna/índice/constraints.
- **Plano com passo destrutivo (HITL no meio do plano)**: o plano é workflow durável do kernel
  (`PlanState` com cursor); a suspensão do passo destrutivo suspende o plano inteiro e o resume
  continua do cursor — sem plano órfão. Risco: resume reconstruir o cursor errado; mitigado por
  cursor/steps persistidos no snapshot (R-WF-KERNEL-001.7).
- **Remoção do legacy antes das pré-condições** deixa espera de categoria órfã: remover só após
  PRÉ-1..PRÉ-4 verdes (ADR-006).
- **Optimistic-lock stale** no edit/delete by-ref: mapear `ErrTransactionVersionConflict` para
  mensagem amigável.
- **Bug latente**: `NewLastTransactionEditorExecutor` não preenche `NewAmount` (edit-last grava 0) —
  corrigir junto no escopo by-ref.
- **ILIKE sem índice**: aceitável no MVP com `LIMIT` + filtro `user_id`; evoluir p/ `pg_trgm` se preciso
  (encapsulado no repo).
- **Falso positivo em evento órfão**: guarda por constante (não por nome de arquivo); manter
  `splits_calculated`/`card_registered`/`external.expense` (têm par).

### Conformidade com Padrões

`R-AGENT-WF-001` (1..8 + addenda), `R-WF-KERNEL-001` (1..7), `R-ADAPTER-001`, `R-TESTING-001`,
`R-DTO-VALIDATE-001`, `R-TXN-WORKFLOWS-001`, governança DMMF (`governance.md`). Checklist R0–R7 da
`go-implementation` por tarefa. Zero comentários em Go de produção (R-ADAPTER-001.1).

### Arquivos Relevantes e Dependentes

- Agent core: `internal/agent/application/services/{daily_ledger_agent,agent_workflows,intent_router}.go`,
  `application/workflow/{registry,composite,destructive_confirm,transactions_write}.go`,
  `application/workflow/steps/*`, `application/tools/*`, `application/usecases/parse_inbound.go`,
  `application/prompting/{prompts,context_builder}.go`, `domain/{intent,confirmation,pendingexpense}/*`,
  `infrastructure/{binding,providers/openrouter,events}/*`, `module.go`.
- Kernel: `internal/platform/workflow/{engine,store,codec,step,combinators}.go` + `infrastructure/postgres`.
- Transactions (porta nova): `application/usecases/search_transactions.go`,
  `application/interfaces/transaction_repository.go`, `infrastructure/repositories/postgres/transaction_repository.go`,
  `domain/valueobjects/search_query.go`.
- Config/wiring: `configs/config.go`, `cmd/server/server.go`, `cmd/worker/worker.go`, `internal/bootstrap/channel.go`, `.env.example`.
- Migrations: `migrations/000020_drop_telegram_channel.{up,down}.sql`,
  `migrations/000021_agent_decisions_step_index.{up,down}.sql`.
- Onboarding LLM (migração tool-calling→json_schema): `application/usecases/run_onboarding_turn.go`,
  `application/usecases/onboarding_tool_catalog.go`, `infrastructure/onboarding/onboarding_tool_dispatcher.go`.
- Idempotência por passo: `domain/entities` (Decision + `step_index`),
  `infrastructure/repositories/postgres/agent_decision_repository.go`.
- CI: `scripts/ci/agent-data-boundary.sh`.
- Eliminação Telegram (deletar): `internal/platform/telegram/**`, consumer telegram do agent, onboarding-Telegram, `internal/platform/notification/adapters/telegram.go`, `internal/identity/infrastructure/http/server/telegram_router.go`, `cmd/server/telegram_wiring.go` — lista exaustiva no ADR-005.
