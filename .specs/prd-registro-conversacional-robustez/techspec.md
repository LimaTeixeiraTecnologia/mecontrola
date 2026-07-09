<!-- spec-hash-prd: aa6ebc2e1f1661f7f154591370b83cd2ce3ebbc59ca2dfa0f24a4501850bc69e -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Registro Conversacional Robusto

## Resumo Executivo

Esta especificação torna o registro conversacional de transações confiável, fluido e canônico em BRL,
corrigindo cada gap comprovado do incidente de produção de 2026-07-08 (usuário `f56e1142`). O trabalho
concentra-se no consumidor `internal/agents` (agente, tools, workflows, usecases) e em uma migração de
seed em `internal/categories`, **sem alterar o kernel genérico** `internal/platform/workflow` (que já
oferece `Retry` e propagação de `StepStatusFailed`) e reusando o helper canônico
`internal/platform/money`.

A causa-raiz é observabilidade: o passo de escrita engole o erro e retorna `StepStatusCompleted`,
deixando `platform_runs.error` vazio e sem trace pesquisável (ADR-002). A correção estrutural é parar
de engolir o erro no passo do consumidor (deixando o kernel marcar `RunStatusFailed` e registrar o
span) e propagar o motivo real ao Run auditável. Sobre essa base entram: seed da folha income
`Salário > Salário` com slug distinto (ADR-001), guarda de kind com reclassificação (ADR-005), retry
transitório limitado a 2 tentativas reusando o combinador do kernel (ADR-003), confirmação única com o
gate HITL como único dono (ADR-004), correção da heurística de múltiplos lançamentos (prompt),
consolidação de 6 call-sites em `money.BRL()` e slot de forma de pagamento com exemplos e proibição de
inferência. Todas as mudanças preservam DMMF state-as-type, R-ADAPTER-001, R-AGENT-WF-001 e
R-WF-KERNEL-001.

## Arquitetura do Sistema

### Visão Geral dos Componentes

Componentes **modificados** (nenhum kernel; nenhuma nova camada):

- `migrations/NNNNNN_add_salario_base_leaf.{up,down}.sql` **(novo)** — seed idempotente da folha
  income `salario-base` (display "Salário") + termos no `category_dictionary` + bump de
  `category_editorial_version` (ADR-001).
- `internal/agents/application/workflows/pending_entry_workflow.go` — fim do swallow em
  `executeWithIdempotency` (:444-461), `executeDirectWrite` (:463-473) e `validateCategoryForWrite`
  (:413-442): retornam `StepStatusFailed` + erro real em falha (ADR-002); aplicação de retry
  transitório no trecho de escrita (ADR-003); reclassificação por kind e clarify único (ADR-005).
- `internal/platform/agent/runtime.go` — `finishRun`/`closeRun` (:155-211, :259) passam a gravar o
  motivo real em `run.Error`/`platform_runs.error` e emitir span de erro pesquisável no worker
  (ADR-002).
- `internal/agents/application/usecases/pending_entry_continuer.go` (:79) e `register_attempt.go`
  — **passam a abrir/fechar um Run auditável em `platform_runs`** ao redor de `engine.Resume`
  (resolvendo Thread + `wamid`), propagando o motivo real para `platform_runs.error` e emitindo span
  de erro pesquisável com `thread_id`/`run_id`/`wamid`; ponto de aplicação do retry (ADR-003). Fecha
  o gap de confirmação não auditável (ADR-002, decisão de run-table). O kernel mantém
  `workflow_runs.last_error` (mecanismo genérico), sem duplicação semântica.
- `internal/agents/application/usecases/idempotent_write.go` — consumido pelo retry; sem mudança de
  contrato (o `reconciled` já garante at-most-once).
- `internal/agents/application/agents/mecontrola_agent.go` — instruções: confirmação só pelo gate
  (ADR-004); heurística de múltiplos lançamentos ignora separador de milhar (R5); proibição de
  inferir forma de pagamento e exemplos no slot (R8).
- `internal/agents/application/workflows/category_resolution.go` — filtro/priorização por kind
  (ADR-005).
- `internal/agents/application/workflows/pending_entry_decisions.go` — slot de forma de pagamento com
  exemplos no prompt inicial; correção do mapa `knownPaymentMethods` (`"doc"` inválido).
- `internal/agents/application/workflows/onboarding_workflow.go` e `pending_entry_workflow.go` —
  consolidação de `formatBRL`/`formatAmountBR` em `money.BRL()` (R7).
- Novo predicado `IsTransient(err) bool` no consumidor (`internal/agents`, camada application) (ADR-003).

Fluxo de dados inbound (inalterado, apenas endurecido):
`WhatsApp inbound → AgentRuntime.Execute → Thread/Run → mecontrola_agent (loop tool-calling) →
register_expense/income → register_attempt → workflow.Engine.Start (suspende no gate HITL) → clarify
→ usuário "sim" → consumer → Engine.Resume (merge-patch) → executeWrite (retry+idempotência) →
Run auditável (sucesso ou failed com erro real)`.

## Design de Implementação

### Interfaces Chave

Contrato de propagação de erro no passo do workflow (ADR-002) — assinaturas existentes, semântica
corrigida:

```go
func executeWithIdempotency(ctx context.Context, state PendingEntryState,
    ledger interfaces.TransactionsLedger, idem IdempotentWriter,
) (workflow.StepOutput[PendingEntryState], error)

func executeDirectWrite(ctx context.Context, state PendingEntryState,
    ledger interfaces.TransactionsLedger,
) (workflow.StepOutput[PendingEntryState], error)
```

Regra: falha real de escrita/infra ⇒ `return StepOutput{State: state (ResponseText amigável),
Status: workflow.StepStatusFailed}, fmt.Errorf("agents.workflow.pending_entry: write: %w", err)`.
Cancelamento de negócio e replay/reconciled permanecem `StepStatusCompleted`.

Idempotência de escrita (existente, `idempotent_write.go:42-138`):

```go
type WriteFn func(ctx context.Context) (uuid.UUID, bool, error) // (resourceID, reconciled, err)

func (uc *IdempotentWrite) Execute(ctx context.Context, userID uuid.UUID, wamid string,
    itemSeq int, operation string, resourceKind string, write WriteFn,
) (IdempotentWriteResult, error)
```

Retry transitório (ADR-003) — **loop localizado** no trecho de escrita (não o combinador de nível
`Step[S]`, nem `Engine.MaxAttempts`; ver ADR-003 para a justificativa), respeitando `ctx.Done()`:

```go
const maxWriteAttempts = 2
var lastErr error
for attempt := 1; attempt <= maxWriteAttempts; attempt++ {
    resourceID, outcome, lastErr = idem.Execute(ctx, state.UserID, state.MessageID, state.ItemSeq,
        state.OperationKind.String(), resourceKindForState(state), writeFn)
    if lastErr == nil || !IsTransient(lastErr) {
        break
    }
    select {
    case <-ctx.Done():
        return workflow.StepOutput[PendingEntryState]{State: state, Status: workflow.StepStatusFailed}, ctx.Err()
    case <-time.After(backoffWithJitter(attempt)): // ~100ms base, teto <~2s total
    }
}
```

Predicado de transitório (novo, consumidor):

```go
func IsTransient(err error) bool // timeout, context.DeadlineExceeded, conn reset; default: false
```

**Decisão de padrão (gate `design-patterns-mandatory`)** — seletor determinístico
(`select_pattern.py`) executado com sinais `prefer_direct_solution`, `single_variant_only`,
`low_change_frequency` e constraints `team_needs_low_cognitive_load`, `preserve_public_contract`:
resultado **`reject` → não aplicar padrão GoF**. O combinador `Retry` do kernel é utilitário de
resiliência já existente (não um pattern a introduzir); a classificação transitório-vs-permanente é
um predicado direto `IsTransient`. Alternativa rejeitada: Strategy/Decorator para a política de retry
(uma única política, sem variação em runtime — indireção sem retorno). Fonte de resiliência:
`go-implementation/references/resilience.md`.

Forma de pagamento (R8) — `PaymentMethod` **já é tipo fechado** com smart constructor
`ParsePaymentMethodForCreate` (`internal/transactions/domain/valueobjects/payment_method.go:53`) que
rejeita código inválido (gate DMMF satisfeito no write). Não se introduz novo tipo no consumidor; o
fix é corrigir `knownPaymentMethods` (`pending_entry_decisions.go:118`, `"doc":"doc"` → código fora
do enum) para produzir apenas códigos válidos, tendo o smart constructor como defesa final (agora
não-silenciosa, ADR-002).

### Modelos de Dados

Tipos fechados existentes, reusados sem introduzir strings livres (DMMF state-as-type):

- `valueobjects.Kind` (`KindIncome=1`, `KindExpense=2`), `valueobjects.Confidence`
  (`high|medium|low`), `valueobjects.SignalType` (`canonical_name|alias|phrase|merchant|segment`).
- `PendingStatus` (`Active|Completed|Cancelled|Expired|Replaced`), `AwaitingSlot`
  (`Category|PaymentMethod|Card|Date|Confirmation|Correction`), `PendingOperationKind`.
- `agent.RunStatus` (`Running|Succeeded|Failed`), `agent.ToolOutcome`
  (`Routed|Clarify|UsecaseError|MissingResolver|Replay|Reconciled`).
- `workflow.StepStatus` (`Completed|Suspended|Failed|Skipped`), `workflow.RunStatus`.

Schema seed (ADR-001), respeitando `UNIQUE (kind, slug)`. Os IDs abaixo (`<uuidv5>`, `<uN>`) são
**derivados deterministicamente** (não inventados): folha =
`uuid.NewSHA1(categoryNamespace, "income:salario-base")`; termos =
`uuid.NewSHA1(categoryNamespace, "dict:income:salario-base:"+term_normalized)` — literais fixados na
migração e cobertos pelo teste de drift (ADR-001):

```sql
INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('<uuidv5>', 'salario-base', 'Salário', 'income', '86dd34b0-7342-525a-9a30-b1b5a76b109f', 'consumption')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.category_dictionary (id, category_id, kind, term, signal_type, confidence, is_ambiguous) VALUES
('<u1>', '<uuidv5>', 'income', 'salario',            'canonical_name', 'high', false),
('<u2>', '<uuidv5>', 'income', 'salário',            'alias',          'high', false),
('<u3>', '<uuidv5>', 'income', 'meu salário',        'phrase',         'high', false),
('<u4>', '<uuidv5>', 'income', 'recebi salário',     'phrase',         'high', false),
('<u5>', '<uuidv5>', 'income', 'recebi meu salário', 'phrase',         'high', false)
ON CONFLICT (id) DO UPDATE SET term = EXCLUDED.term, confidence = EXCLUDED.confidence, is_ambiguous = EXCLUDED.is_ambiguous;

UPDATE mecontrola.category_editorial_version SET version = version + 1;
```

### Endpoints de API

Nenhum endpoint novo. O caminho inbound (`agents.route.whatsapp_inbound`) permanece; a mudança é
interna ao processamento do worker.

## Pontos de Integração

- **OpenRouter (LLM)**: único provider (`llm.NewOpenRouterProvider`), call-site sancionada no loop do
  agent. Sem novo uso de LLM; a heurística R5 e a política de confirmação R4 são via prompt
  (instruções do agente), não novas chamadas.
- **PostgreSQL**: seed via `golang-migrate` (`migrations/embed.go`, driver `pgx5`); escrita de domínio
  via módulo `transactions` (dedup por chave → `reconciled`); `agents_write_ledger` agent-owned.
- **Observabilidade (otel-lgtm)**: spans (Tempo), métricas Prometheus (`agents_write_total`,
  `agent_runs_total`, `agents_pending_entry_slot_total`), logs (Loki). Cardinalidade controlada.

## Abordagem de Testes

### Testes Unitários

- `IsTransient(err)` — tabela: timeout/deadline/conn-reset ⇒ true; validação de domínio/`ErrKindMismatch`
  ⇒ false.
- Heurística R5 (se extraída para função pura auxiliar) e/ou teste de prompt: "R$ 13.874,40" = 1 valor.
- `money.BRL()` já coberto (`money_test.go`); adicionar asserts para os valores do PRD (554976,
  80000000, 5000, 5050).
- Reclassificação por kind (ADR-005): income com candidato expense ⇒ candidato income; sem candidato
  income ⇒ clarify único.
- Propagação de erro (ADR-002): passo retorna `StepStatusFailed` + erro em falha real; `Completed` em
  cancelamento/replay.

Padrão obrigatório: testify/suite whitebox, `fake.NewProvider()`, mocks do `.mockery.yml`, IIFE por
mock (R-TESTING-001).

### Testes de Integração

Critérios atendidos (≥2): fronteiras de IO críticas (Postgres, ledger, snapshot) onde mocks não
garantem correção; incidente real onde unit tests passavam mas a integração falhava silenciosamente.
**Integration tests recomendados** com `//go:build integration` / testcontainers-go.

- Migração de seed (`migrations_integration_test.go`): folha `salario-base` sob raiz Salário;
  dicionário resolve os 5 termos; Décimo Terceiro inalterado; reexecução idempotente; bump de versão.
- Propagação de erro: escrita falha ⇒ `platform_runs.error` preenchido, span de erro com
  `thread_id/run_id/wamid`, log ERROR; cancelamento "não" ⇒ run não-failed.
- Retry + idempotência: falha transitória na 1ª tentativa ⇒ persiste 1x na 2ª; reprocessar mesma
  `(wamid,itemSeq,operation)` ⇒ replay; transação criada + ledger falho ⇒ não reexecuta (reconciled).

### Testes E2E (real-LLM)

Política do repositório (RUN_REAL_LLM=1 com `.env` OPENROUTER_*), cobrindo os cenários Gherkin do PRD:

- Salário resolve `Salário > Salário` sem clarify; income 1387440 centavos.
- Décimo terceiro permanece em `Décimo Terceiro`.
- Compra de livro no pix não pede confirmação duas vezes; LLM não emite confirmação própria;
  cancelamento descarta.
- "R$ 13.874,40" não dispara múltiplos; "gastei 30 no ônibus e 15 no café" dispara com texto exato.
- Despesa sem forma de pagamento pergunta com exemplos; receita não pergunta forma de pagamento.
- Formatação BRL canônica dos 4 valores do PRD.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Seed R1 (ADR-001)** — migração + integration test; base para resolução de salário. Independente.
2. **Consolidação BRL R7** — `money.BRL()` nos 6 call-sites; remover `formatBRL`/`formatAmountBR`
   locais do módulo agents. Independente, baixo risco.
3. **Propagação de erro R3 (ADR-002)** — fim do swallow + Run/span/log. Base para R6 (o retry depende
   de o passo retornar erro).
4. **Retry R6 (ADR-003)** — `IsTransient` + `workflow.Retry` no trecho de escrita. Depende de (3).
5. **Guarda de kind R2 (ADR-005)** — reclassificação + clarify único. Depende de (3) para não engolir.
6. **Confirmação única R4 (ADR-004)** — reescrita de instruções. Independente das demais (prompt).
7. **Heurística R5 + slot R8** — instruções do agente + prompt do slot + fix `knownPaymentMethods`.
8. **Testes real-LLM + integração** — cobertura completa e gate final.

### Dependências Técnicas

- Postgres com migração aplicada (seed) antes dos testes de resolução de salário.
- `.env` com OPENROUTER_* para os testes real-LLM.

## Monitoramento e Observabilidade

- **Métricas** (Prometheus, cardinalidade controlada — labels fechados, sem `user_id`/`category_id`):
  - `agents_write_total{operation,outcome}` (`created|reconciled|replay|usecase_error`) — mantida.
  - `agent_runs_total{agent_id,status}` — `failed` passa a ter causa rastreável no Run.
  - `agents_pending_entry_slot_total{slot,outcome}` — confirmação e forma de pagamento.
- **Logs** (Loki): ERROR em falha de escrita com `thread_id`, `run_id`, `wamid` (nunca como label de
  métrica).
- **Traces** (Tempo): span de erro pesquisável do run de escrita no worker, emitido pelo
  **consumidor** (nome estável, `RecordError`, status error), correlacionado por
  `thread_id`/`run_id`/`wamid`. O span genérico do kernel (`workflow.step.execute`) carrega só
  `workflow`/`step` (sem identidade de domínio) — a atribuição de identidade é responsabilidade do
  consumidor, preservando o layering R-WF-KERNEL-001.
- **Persistência de erro (duas tabelas, sem duplicação semântica)**: `platform_runs.error` (Run
  auditável do agente, incluindo o novo Run do turno de confirmação) e `workflow_runs.last_error`
  (mecanismo durável genérico do kernel). Escrita falha no resume ⇒ ambos preenchidos.

## Considerações Técnicas

### Decisões Chave

- **ADR-001** — Folha `Salário > Salário` com slug distinto (`salario-base`) e seed idempotente:
  `.specs/prd-registro-conversacional-robustez/adr-001-salario-leaf-seed.md`.
- **ADR-002** — Propagação de erro para Run, span pesquisável e log (fim do swallow):
  `.specs/prd-registro-conversacional-robustez/adr-002-error-propagation-observability.md`.
- **ADR-003** — Retry transitório limitado (2 tentativas) com classificação de erro:
  `.specs/prd-registro-conversacional-robustez/adr-003-bounded-transient-retry.md`.
- **ADR-004** — Confirmação única; gate HITL é o dono, LLM nunca confirma:
  `.specs/prd-registro-conversacional-robustez/adr-004-single-confirmation-ownership.md`.
- **ADR-005** — Guarda de kind com reclassificação antes do write:
  `.specs/prd-registro-conversacional-robustez/adr-005-kind-guard-reclassification.md`.

Decisões sem ADR dedicada (mecânicas, sem fork arquitetural):

- **R5 (prompt)** — cláusula no prompt: "ponto é separador de milhar no padrão brasileiro; R$ 1.234,56
  é UM valor". Ancorado em `mecontrola_agent.go:15-16`. Conectores ("e", "mais", "também", vírgula
  entre itens) inalterados; texto de orientação inalterado.
- **R7 (formatter)** — trocar `formatBRL`/`formatAmountBR` por `money.FromCents(cents).BRL()` nos 6
  call-sites do módulo agents (onboarding `:242,:335,:508,:521`; pending `:639,:721`) e remover as
  duas funções locais. O 7º call-site (`internal/budgets/.../notify_threshold_alert.go`) é do módulo
  budgets e do `prd-alertas-proativos` (que remove o threshold legado) — **fora deste escopo** para
  evitar retrabalho e drift entre PRDs; a fonte única `money.BRL()` fica pronta para alertas
  consumirem.
- **R8 (slot)** — prompt inicial do slot de pagamento (`pending_entry_workflow.go:680`) passa a
  incluir exemplos; instruções do agente proíbem inferir forma de pagamento ausente; correção do mapa
  `knownPaymentMethods` (`pending_entry_decisions.go:108-120`): `"doc"` mapeava para o código
  inválido `"doc"` (fora do enum) — mapear para `ted` (ou remover), alinhando ao enum fechado de
  `register_expense`. Income continua sem pedir forma de pagamento (`confirmPaymentSegment` retorna
  vazio; `registerIncomePaymentMethod = "pix"` fixo).

### Riscos Conhecidos

- **Aderência do LLM (R4/R5/R8)** — instruções dependem do modelo; mitigado por testes real-LLM como
  gate e pela regra literal de repassar `message`.
- **Editorial version bump (ADR-001)** — pending entries suspensos antes do deploy podem sofrer
  `ErrVersionDrift` no resume; impacto restrito à janela de deploy; comportamento é reclassificar,
  não corromper.
- **Classificação transitório (ADR-003)** — risco de classificar erro permanente como transitório;
  mitigado por whitelist estrita e default permanente.
- **Retry sob workflow durável (ADR-002+003)** — `StepStatusFailed` em passo durável não deve gerar
  loop; teto de tentativas finito e pending retomável controlam.
- **Run auditável no resume (ADR-002)** — abrir/fechar Run em `platform_runs` no caminho de
  confirmação adiciona wiring ao `pending_entry_continuer`/`register_attempt`; risco de dupla escrita
  de status entre `platform_runs` e `workflow_runs` mitigado por responsabilidades distintas
  (`error` semântico vs `last_error` mecânico) e por teste de integração cobrindo os dois turnos.

### Conformidade com Padrões

- `.claude/rules/go-adapters.md` (R-ADAPTER-001) — zero comentários; tools/adapters finos; sem SQL em
  handler/consumer.
- `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001) — roteamento por registry; Tool fina; LLM
  só em call-sites sancionadas; Run auditável; estado de espera fechado persistido antes de clarify;
  WorkingMemory no system prompt.
- `.claude/rules/workflow-kernel.md` (R-WF-KERNEL-001) — kernel intacto: sem import de domínio, sem
  regra/SQL/LLM; estados fechados; merge-patch no resume.
- `.claude/rules/transactions-workflows.md` (R-TXN-004) — cardinalidade de métricas.
- `.claude/rules/go-testing.md` (R-TESTING-001) — testify/suite whitebox + mockery.
- `.claude/rules/input-dto-validate.md` — se novos inputs DTO forem tocados, manter `Validate()`.
- DMMF (`governance.md`) — state-as-type, smart constructors, `Decide*` puro; anti-padrões proibidos.

### Gates das Skills Obrigatórias (aplicados nesta techspec)

- **`go-implementation`** (obrigatória, Go 1.26.5 em `go.mod`): R0 (sem `init()`), R5.12 (sem
  `panic`), R6 (`context.Context` nas fronteiras; interface no consumidor — ex.: `IdempotentWriter`,
  `categoryValidator`), R7.6 (`errors.Join`/`%w` no wrapping), R7 (recursos modernos conforme
  `go.mod`). `IsTransient` usa `errors.Is`/`errors.As` (R5.10). Zero comentários (R-ADAPTER-001.1).
  Testes testify/suite whitebox + mockery (R4/R-TESTING-001). Matriz de validação por risco:
  mudança toca `domain/` (categories VOs) e `application` (agents) ⇒ build+vet+test race+lint no
  projeto para o seed/domínio e no módulo `agents` para o restante.
- **`mastra`** (obrigatória): comportamento novo entra no consumidor `internal/agents` reusando o
  substrato; kernel `internal/platform/workflow` **não é alterado** (reusa `Retry` e propagação de
  `StepStatusFailed`). Tools continuam adapters finos; LLM só nas call-sites sancionadas (loop do
  agent) — R4/R5/R8 são instruções, não novas chamadas. Thread→Run: o resume passa a resolver Thread
  e abrir Run auditável (fecha gap R-AGENT-WF-001.5). Estados fechados preservados
  (`RunStatus`/`ToolOutcome`/`StepStatus`/`AwaitingSlot`/`PendingStatus`). Validar com
  `mastra/references/rules-checklist.md`.
- **`domain-modeling-production`** (obrigatória): state-as-type confirmado em `Kind`, `Confidence`,
  `SignalType`, `PaymentMethod` (`ParsePaymentMethodForCreate` — smart constructor com erro),
  `AwaitingSlot`, `PendingStatus`, `PendingOperationKind`. Nenhuma string livre nova em fronteira
  pública. Validação vive em smart constructors (R-TXN-002); `Decide*` (ex.: `DecideConfirmation`,
  `DecideInitialAwaiting`) permanece puro. Estados ilegais (kind incompatível, payment inválido)
  bloqueados por invariante, agora não-silenciosos (ADR-002).
- **`design-patterns-mandatory`** (gate): seletor executado para R6 ⇒ `reject` (não aplicar padrão);
  loop localizado + predicado direto. Demais mudanças (R1 seed, R2 filtro por kind, R3
  propagação, R4/R5/R8 prompt, R7 helper) ⇒ `não aplicar padrão` (solução direta). Nunca carregar
  `patterns-structural.md` para Factory/Options/Adapter/Decorator/Facade.

Gates executáveis antes do merge (de `mastra/references/rules-checklist.md`, espelhando
`.claude/rules/*`) — todos devem passar:

```bash
go build ./internal/platform/... ./internal/agents/... ./internal/categories/...
go vet  ./internal/platform/... ./internal/agents/... ./internal/categories/...
go test -race -count=1 ./internal/platform/... ./internal/agents/... ./internal/categories/...
# Gate kernel puro (R-WF-KERNEL-001): sem import de domínio/LLM em internal/platform/workflow/
# Gate zero-comentários (R-ADAPTER-001.1) em internal/platform/ internal/agents/
# Gate SQL só no adapter; tools/consumers sem SQL direto (R-ADAPTER-001.2)
# Gate cardinalidade: thread_id/run_id/wamid nunca como label de métrica (R-TXN-004)
golangci-lint run ./internal/agents/... ./internal/categories/...
mockery --config .mockery.yml
# Integration (seed + observabilidade + retry): go test -tags integration -race ./...
# Real-LLM (RUN_REAL_LLM=1 com .env OPENROUTER_*): cenários Gherkin do PRD
```

### Arquivos Relevantes e Dependentes

- `migrations/000001_initial_schema.up.sql` (schema categories/dictionary/ledger), `embed.go`,
  `migrations_integration_test.go`.
- `internal/categories/application/usecases/resolve_category_for_write.go`;
  `internal/categories/domain/valueobjects/{kind,confidence,signal_type}.go`.
- `internal/agents/application/workflows/pending_entry_workflow.go`, `pending_entry_state.go`,
  `pending_entry_decisions.go`, `category_resolution.go`, `onboarding_workflow.go`.
- `internal/agents/application/usecases/idempotent_write.go`, `register_attempt.go`,
  `register_entry.go`.
- `internal/agents/application/tools/register_expense.go`, `register_income.go`.
- `internal/agents/application/agents/mecontrola_agent.go`.
- `internal/platform/agent/runtime.go`, `internal/platform/agent/types.go`.
- `internal/platform/workflow/{engine.go,combinators.go,step.go}` (consumidos, não alterados).
- `internal/platform/money/money.go`, `money_test.go`.
- `internal/agents/infrastructure/persistence/write_ledger_repository.go`.

## Mapeamento Requisito → Decisão → Teste

| RF | Decisão | Arquivo(s) chave | Teste |
|----|---------|------------------|-------|
| RF-01..05 | ADR-001 | migração seed; `resolve_category_for_write.go` | integration seed + real-LLM salário/13º |
| RF-06..09 | ADR-005 | `category_resolution.go`; `validateCategoryForWrite` | unit reclassificação; real-LLM income≠expense |
| RF-10..13 | ADR-002 | `pending_entry_workflow.go:413-473`; `pending_entry_continuer.go:79`; `runtime.go:155-259`; kernel `workflow_runs`/`platform_runs` | integration erro⇒`platform_runs.error`+`workflow_runs.last_error`+span+log; métrica |
| RF-14..18 | ADR-004 | `mecontrola_agent.go:58-66,158-165`; `buildConfirmSummary` | real-LLM confirmação única/cancelamento |
| RF-19..21 | R5 (prompt) | `mecontrola_agent.go:15-16` | real-LLM valor único vs 2 lançamentos |
| RF-22..25 | ADR-003 | `register_attempt.go`; `idempotent_write.go`; `combinators.go` | integration retry/replay/reconcile |
| RF-26..28 | R7 (formatter) | `money.go`; `onboarding_workflow.go`; `pending_entry_workflow.go` | unit `money.BRL()`; real-LLM display |
| RF-29..32 | R8 (slot) | `mecontrola_agent.go`; `pending_entry_workflow.go:680`; `pending_entry_decisions.go` | real-LLM pergunta com exemplos; income sem pagamento |
