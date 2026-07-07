<!-- bugfix-report: 2026-07-07 -->
<!-- base-sha: ad42aa1b64d03fd48f0848c1893a0b31912ae767 -->

# Relatório de Bugfix — PRD Conversa Agentiva Fluida (spec-version 3)

Remediação dos achados F-01..F-09 do `review_report.md`. Escopo acordado: `bugfix de tudo (F-01..F-09)`.

## Resumo

- Bugs no escopo: 9 (1 critical, 3 high, 5 medium)
- Corrigidos: 9 (`fixed`)
- Testes de regressão adicionados/reescritos: 11
- Validação: `go build`, `go vet`, `go test -race`, `golangci-lint`, `RUN_REAL_LLM=1`

## Bugs corrigidos

### F-01 — `fixed` — [critical] Recorrência escrevia sem gate de confirmação
- **Origem:** review finding F-01 (RF-38, RF-43, CA-16, D-10, ADR-004); M-07>0.
- **Causa raiz:** `create_recurrence.go` persistia via `writer.Execute → recurrences.CreateRecurrence` no `exec` da tool (escrita síncrona), sem abrir pendência.
- **Correção:** nova use case gated `RegisterAttempt.CreateRecurrence` (abre `AwaitingSlotConfirmation` com `OperationKind=PendingOpCreateRecurrence`, preservando `Frequency`/`RecurrenceDayOfMonth`/`CardID`); `create_recurrence.go` reescrita para delegar a ela e retornar `ToolOutcomeClarify` (sem escrita síncrona). `module.go` religa a tool a `registerAttempt`. `subcategoryId` agora obrigatório no schema (cobre F-08).
- **Arquivos:** `application/tools/create_recurrence.go`, `application/usecases/register_attempt.go`, `application/usecases/register_entry.go`, `module.go`.
- **Regressão:** `TestBuildCreateRecurrenceTool_OpensPendingNoSyncWrite_CA16`, `TestBuildCreateRecurrenceTool_MissingSubcategory_Errors`, `TestCreateRecurrence_OpensPendingConfirmation_NoSyncWrite_CA16`.

### F-02 — `fixed` — [high] `edit_entry` descartava EntryKind → direção corrompida / persist falho
- **Origem:** review finding F-02 (RF-25, D-13, CA-17).
- **Causa raiz:** o estado da pendência de edição era construído sem `Kind` nem `Candidates`; `pendingDirection(zero)` retornava `"outcome"` e `CategoryID=uuid.Nil`; `UpdateTransaction` exige Direction+CategoryID+PaymentMethod+OccurredAt → toda edição falhava/corrompia.
- **Correção:** nova use case gated `RegisterAttempt.EditEntry` que busca a transação alvo (`GetTransaction`), deriva `Kind` da direção real, resolve a categoria canônica atual via `classifyForPending` (com a versão editorial de `SearchDictionary`), e faz merge dos campos (amount/description/occurredAt) preservando `TargetTransactionID`/`TargetVersion`/`PaymentMethod`. Tool `edit_entry.go` reescrita para delegar. Quando a categoria atual não resolver, cai em `AwaitingSlotCategory` (clarificação — CA-17).
- **Arquivos:** `application/tools/edit_entry.go`, `application/usecases/register_attempt.go`, `module.go`.
- **Regressão:** `TestBuildEditEntryTool`, `TestEditEntry_PreservesKindAndTarget_CA17`.

### F-03 — `fixed` — [high] `DecideCategoryChoice` era código morto; resposta de categoria caía em reprompt→cancel
- **Origem:** review finding F-03 (RF-27, RF-42, CA-01, CA-15).
- **Causa raiz:** `AwaitingSlotCategory` caía no `default → handleSlotResume → DecidePendingResume`, que não trata categoria; a resposta ("custo fixo"/"2") gerava reprompt e depois cancel. `DecideCategoryChoice` só era chamado por testes.
- **Correção:** novo `handleCategorySlotResume` no dispatch do workflow: aplica `DecideCategoryChoice` sobre os candidatos existentes (seleção por número OU nome), promove o escolhido a `AwaitingSlotConfirmation`; se não casar, re-busca no dicionário (`SearchAndEnrichCandidates`) e re-apresenta lista numerada ou promove candidato único; `RootOnly`/incompatível → reprompt único então cancel. Interface `categoryValidator` do workflow ampliada para incluir `SearchDictionary`.
- **Arquivos:** `application/workflows/pending_entry_workflow.go`.
- **Regressão:** `TestResume_CategorySelectionByNumber_MovesToConfirmation_CA15`, `TestResume_CategorySelectionByName_MovesToConfirmation_CA01`.

### F-04 — `fixed` — [high] Harness não media M-06 (confusão entre pendências)
- **Origem:** review finding F-04 (RF-33, D-07, M-06).
- **Causa raiz:** nenhum cenário determinístico com duas pendências simultâneas.
- **Correção:** teste de isolamento cross-thread: dois runs (`thr-M06-A`/`thr-M06-B`), confirma A e assere que B permanece suspenso e intocado.
- **Arquivos:** teste.
- **Regressão:** `TestResume_TwoPendencies_IsolatedByThread_M06`.

### F-05 — `fixed` — [medium] Revalidação categorial condicional em `executeWrite`
- **Origem:** review finding F-05 (RF-10, RF-14, RF-30).
- **Causa raiz:** revalidação só ocorria com `len(Candidates)>0`; edição/candidatos vazios contornavam.
- **Correção:** `executeWrite` agora bloqueia incondicionalmente quando não há candidato ou quando o candidato é raiz-sem-folha (`SubcategoryID` zero ou == raiz), antes de qualquer chamada ao ledger; revalida via `ResolveForWrite` quando disponível.
- **Arquivos:** `application/workflows/pending_entry_workflow.go`.
- **Regressão:** `TestRootWithoutLeaf_BlockedBeforeLedger_G10_01_M04` (assere bloqueio antes do ledger).

### F-06 — `fixed` — [medium] Caminho report-only síncrono (`RegisterEntry`) latente
- **Origem:** review finding F-06 (revogado por spec-v3/ADR-004).
- **Causa raiz:** `RegisterEntry` (escrita síncrona sem confirmação) permanecia e satisfazia `entryRegistrar`, risco de re-wire.
- **Correção:** removido `RegisterEntry` + seus testes; mantidos apenas helpers compartilhados (`RegisterResult`, comandos, `resolveEntryDate`). Removida a fiação morta de `IdempotentWrite` em `module.go`. Testes de integração/E2E que exercitavam o caminho síncrono revogado foram reescritos para o fluxo gated (register → confirmação → escrita).
- **Arquivos:** `application/usecases/register_entry.go` (removido `RegisterEntry`), `module.go`, `register_entry_test.go` (removido), `mecontrola_agent_e2e_test.go`, `mecontrola_agent_chain_realllm_test.go`, `register_expense_integration_test.go`.

### F-07 — `fixed` — [medium] Escrita confirmada pulava idempotência; branch de replay morto
- **Origem:** review finding F-07 (RF-20, RF-43, CA-07).
- **Causa raiz:** `ConfirmActionReplay` comparava `msg.MessageID == state.MessageID` (wamid original ≠ id da confirmação) → inalcançável.
- **Correção:** novo campo `ProcessedMessageID` no estado, gravado no reprompt de confirmação; `DecideConfirmation` compara contra ele. Idempotência de escrita confirmada garantida pelo ciclo de vida do Run (resume de run concluído é no-op) + `OriginWamid` no módulo transactions — comprovado por teste.
- **Arquivos:** `application/workflows/pending_entry_state.go`, `pending_entry_decisions.go`, `pending_entry_workflow.go`.
- **Regressão:** `TestReplayIdempotent_SameMsgID_NoSecondWrite_G7_09_CA07` (reescrito: 1ª confirmação escreve, 2ª é no-op), `TestDecideConfirmation_Replay`, `TestE2E2_ReconfirmarNaoDuplica`.

### F-08 — `fixed` — [medium] Recorrência aceitava raiz-sem-folha; IDs direto do LLM sem revalidar
- **Origem:** review finding F-08 (RF-30, RF-13, RF-14, D-04).
- **Correção:** coberto por F-01 (recorrência agora passa por `classifyForPending`+`ResolveForWrite` e pelo gate `executeWrite` de F-05); `subcategoryId` obrigatório no schema.
- **Arquivos:** `application/tools/create_recurrence.go`, `application/usecases/register_attempt.go`.

### F-09 — `fixed` — [medium] Decisão de slot inicial embutida na use case com magic string
- **Origem:** review finding F-09 (RF-19, RF-36).
- **Correção:** extraída função pura `DecideInitialAwaiting(categoryAwaiting, paymentMethod, hasCard)` e constante `PaymentMethodCreditCard`; `register_attempt` passa a delegar a ela.
- **Arquivos:** `application/workflows/pending_entry_decisions.go`, `application/usecases/register_attempt.go`.

## Validações executadas

- `go build ./...` → exit 0
- `go vet ./...` → exit 0
- `go build -tags integration ./...` → exit 0; `go vet -tags integration ./internal/agents/...` → limpo
- `go test -race -count=1 ./internal/agents/... ./internal/platform/agent/... ./internal/platform/workflow/...` → todos `ok`
- `golangci-lint run ./internal/agents/...` → limpo nos arquivos alterados (ver riscos residuais)
- `RUN_REAL_LLM=1` (credenciais `.env`): `TestRealLLM_PendingEntry_*` (CA-01/CA-04/CA-06/formatação), `TestRealLLM_RegisterOpensConfirmationGate`, `TestRealLLM_CardPurchaseChain_*` → PASS (gate abre pendência durável, sem escrita prematura)
- Gates governança: zero-comentários, sem SQL em tool/consumer, kernel sem domínio, merge-patch resume, sem `switch intent.Kind`, sem estado string-livre → todos limpos

## Validação Postgres/integração e E2E (executada, não apenas compilada)

Suíte de integração `-tags integration` executada de ponta a ponta (testcontainers Postgres + `RUN_REAL_LLM=1`):

- `./internal/agents/...` (todos os pacotes) → **`ok`**, incluindo:
  - `TestPendingEntryIntegrationSuite` (gate via Postgres, Start→Resume→write, expiração)
  - `TestRegisterExpenseIntegrationSuite` (register abre pendência durável, sem escrita síncrona)
  - `TestMeControlaAgentE2ESuite` (E2E real-LLM: register → confirmação → 1 transação persistida; reconfirmar não duplica)
  - `TestRealLLM_*` (agents 69s, scorers 54s)

## Correções fora do escopo dos 9 findings (baseline pré-existente exposto)

A suíte de integração do binding NÃO compilava no HEAD (`-tags integration`), logo estes testes nunca rodavam; a correção da compilação expôs bugs pré-existentes de autoria de teste, corrigidos para manter a suíte verde:

- `ca09_reconciled_integration_test.go` / `transactions_integration_test.go`: `NewTransactionsLedgerAdapter` com aridade antiga → arg `nil` para `createRecurringTx`.
- `pending_entry_integration_test.go` (arquivo não escrito por esta remediação): (a) `Snapshot{}` sem `MaxAttempts` violava `workflow_runs_max_attempts_check CHECK (max_attempts > 0)` → `MaxAttempts: 1`; (b) assert de `store.Load` retornar run **succeeded** contradizia a semântica do kernel (`Load` só retorna `running/suspended`) → passa a asserir que o run concluído deixa o conjunto ativo. O caminho gated em si (write 1x na confirmação) já passava.
- `mecontrola_agent_e2e_test.go`: `e2eStubCategoriesReader.ResolveForWrite` retornava decisão vazia (escrito para o antigo `classify` sem enriquecimento); o fluxo gated enriquece candidatos via `ResolveForWrite` → stub passa a devolver o par raiz+folha canônico seedado. Sem isso a pendência abria em `AwaitingSlotCategory` sem candidato e a confirmação não escrevia.

## F-10 — `fixed` — [high] Surfacing do prompt de confirmação (UX do gate)

- **Origem:** achado de validação real-LLM desta remediação (RF-24, O-07, CA-01, CA-13).
- **Causa raiz:** ao abrir a pendência de confirmação, `RegisterResult`/output da tool não devolvia o resumo (`buildConfirmSummary`) ao LLM → em standalone o modelo improvisava ("dificuldades técnicas"), e o usuário nunca via "Confirma? R$ X em *Raiz > Folha*...".
- **Correção:** `RegisterResult.Message` carrega o prompt do `Snapshot` suspenso (`pendingPrompt` lê `Suspend.Prompt`/`State.ResponseText`); as tools `register_expense`/`register_income`/`create_recurrence` expõem `message` no output (schema strict) e `edit_entry` usa como `impactNote`; instrução mandatória no system prompt: em `outcome=clarify` com `message` não-vazio, responder EXATAMENTE com `message`, sem reescrever/inventar sucesso/erro.
- **Arquivos:** `application/usecases/register_entry.go`, `register_attempt.go`, `application/tools/{register_expense,register_income,create_recurrence,edit_entry}.go`, `application/agents/mecontrola_agent.go`.
- **Regressão + validação:** `TestRegisterExpense_ClassifyResolved_OpensConfirmation_CA13` assere `Message` contém "Confirma?"; real-LLM comprova a resposta agora ser a pergunta de confirmação:
  - `TestRealLLM_RegisterOpensConfirmationGate`: "Confirma o lançamento de R$ 150,00 em *Custo Fixo > Supermercado* para hoje no pix?"
  - `TestMeControlaAgentE2ESuite/E2E1`: "Confirma? R$ 50,00 em *Alimentação > Restaurante* para hoje no débito?"
  - `TestRealLLM_CardPurchaseChain`: "Confirma a compra de R$ 3000,00 em *Custo Fixo > Eletrônicos*..."

## Riscos residuais

- `TestRealLLM_PendingEntry_CA01_ClarifyAsksOneQuestion` (fake stub, sem `message`) tem asserção frágil de contagem de "?" contra variância de fraseado do LLM; passa em 3/3 re-execuções, falha esporádica é não-determinismo do modelo, não regressão. Considerar afrouxar a asserção.
- **`staticcheck QF1012`** em `category_resolution.go:69` (`WriteString(fmt.Sprintf)`), função não tocada por esta remediação — débito de lint pré-existente.
- Edição de compra no cartão de crédito: `RawUpdateTransaction`/`buildRawUpdate` não carregam `CardID`/`Installments` (limitação pré-existente do contrato de update do agente); edições de lançamentos non-credit-card funcionam.
