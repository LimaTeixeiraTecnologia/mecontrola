<!-- review-report: 2026-07-07 -->
<!-- reviewed-sha: ad42aa1b64d03fd48f0848c1893a0b31912ae767 -->
<!-- prd-hash: 1134ba7717b0f9dea2a79fc428631ad01153d5f0275fafff4d883e26ddc2765d -->

# Resultado da Revisão — PRD Conversa Agentiva Fluida (spec-version 3)

## Veredito

`REJECTED`

Motivo: dois achados `critical`/`high` no gate de confirmação universal e no fluxo de edição, **mais** a quebra do fluxo de clarificação categorial — o cenário motivador central do PRD (CA-01 / US-01) — no caminho vivo de produção.

## Resumo Executivo

- Total de RF avaliados: 43
- Total de CA avaliados: 17
- Achados críticos: 1
- Achados high: 3
- Achados medium: 4
- Achados low: 4
- RF não atendidos: RF-10 (condicional), RF-13 (agent-side recorrência/edição), RF-14 (edit/recorrência), RF-30 (agent-side), RF-35 (parcial), RF-38 (recorrência), RF-42, RF-43 (recorrência)
- CA não atendidos: CA-01, CA-15, CA-16, CA-17
- Riscos residuais: harness dirige o workflow direto (`engine.Start/Resume`) sem exercer as tools reais → falso positivo de aceite; M-06 sem cenário determinístico; idempotência de confirmação depende só do lifecycle do Run

## Achados (consolidados e verificados no código)

| ID | Severidade | Arquivo | Linha | Impacto | Hint de Correção |
|----|-----------|---------|-------|---------|------------------|
| F-01 | critical | internal/agents/application/tools/create_recurrence.go | 114-135 | `create_recurrence` (viva: `module.go:227` write-set + `module.go:322`) persiste template recorrente **sincronamente** via `writer.Execute → recurrences.CreateRecurrence`, sem turno `AwaitingSlotConfirmation`. Viola RF-38/RF-43/CA-16/D-10/ADR-004; M-07>0 para recorrência. O ramo pendente `PendingOpCreateRecurrence` existe no workflow mas nenhuma tool o inicia → morto. | Rotear recorrência pela pending-entry (abrir `AwaitingSlotConfirmation` com `OperationKind=PendingOpCreateRecurrence`, `Kind`, candidatos, `CategoryVersion`); remover a escrita síncrona da tool; persistir só em `executeWrite`. |
| F-02 | high | internal/agents/application/tools/edit_entry.go | 89-103 | O estado é construído sem `Kind` nem `Candidates`, embora `in.EntryKind` seja capturado (:19, required). No resume, `buildRawUpdate` usa `pendingDirection(state.Kind)` (workflow:363) → `"outcome"` para `Kind` zero, e `chosenCandidate` (workflow:356) retorna `CategoryID=uuid.Nil`. Edição de receita é corrompida para despesa e/ou rejeitada por `internal/transactions`. CA-17 inatingível. | Propagar `in.EntryKind` para `state.Kind`; suportar clarificação/preservação de categoria na edição (raiz+folha canônicos) antes de `AwaitingSlotConfirmation`. |
| F-03 | high | internal/agents/application/workflows/pending_entry_decisions.go | 150-186, 227 | `DecideCategoryChoice` (RF-42/CA-15 número OU nome + revalidação raiz+folha) é chamado **só por testes**. `register_attempt` abre pendência durável em `AwaitingSlotCategory` (usecase:71/107/141/144/176); no resume, `handleSlotResume → DecidePendingResume` não tem ramo para `AwaitingSlotCategory` → **Reprompt (1ª) → Cancel (2ª)**. O continuer consome a mensagem (`Handled=true`), curto-circuitando o protocolo LLM de re-register. Resultado: usuário responde "custo fixo"/"2" e a operação é cancelada. CA-01/CA-15/RF-27/RF-42 quebrados no caminho vivo. | Ligar `DecideCategoryChoice` ao step quando `Awaiting==AwaitingSlotCategory`: promover o candidato escolhido a `Candidates[0]`, revalidar via `ResolveForWrite`, transicionar para `AwaitingSlotConfirmation`. |
| F-04 | high | internal/agents/application/agents/pending_entry_harness_test.go | (ausência) | A fonte oficial (RF-33/D-07) **não mede M-06**: nenhum cenário determinístico de confusão entre pendências (todos usam a mesma key `user:thread-001:pending-entry`). Colisão de chave / resume no Run errado passaria despercebida. | Adicionar cenário com 2 pendências (mesmo user, threads distintos) assertando que resume de A não altera B; ou formalizar M-06 vacuamente 0 por escopo single-pending-per-thread. |
| F-05 | medium | internal/agents/application/workflows/pending_entry_workflow.go | 279-292 | `executeWrite` só revalida a categoria (`ResolveForWrite`) quando `len(Candidates)>0`; para edição (Candidates vazio) a revalidação é pulada. RF-10/RF-14/RF-30 não são incondicionais no agent-side. | Tornar a revalidação incondicional para todo `OperationKind` que exija categoria; bloquear escrita se `len(Candidates)==0`. |
| F-06 | medium | internal/agents/application/usecases/register_entry.go | (todo) | Caminho `RegisterEntry`/`RegisterExpense`/`RegisterIncome` síncrono (report-only revogado por spec-v3/ADR-004) ainda presente e satisfaz `entryRegistrar`. Não wired hoje, mas regressão latente (re-wire reintroduz escrita sem gate). | Remover `RegisterEntry` ou colapsar em `RegisterAttempt` para que só o caminho gated satisfaça a interface. |
| F-07 | medium | internal/agents/application/workflows/pending_entry_workflow.go | 293; decisions:193 | Escrita confirmada não passa por `IdempotentWrite`; `ConfirmActionReplay` é morto (`msg.MessageID` = id da confirmação ≠ `state.MessageID` = wamid original). Idempotência (RF-20/RF-43/CA-07) depende só do módulo transactions + término one-shot do Run. | Rotear escrita confirmada por `IdempotentWrite` chaveado pela identidade inbound original, ou corrigir a comparação de replay; adicionar caso de harness de confirmação redelivered. |
| F-08 | medium | internal/agents/application/tools/create_recurrence.go | 22,54 | `subcategoryId` opcional (omitempty, ausente de `required`) permite recorrência raiz-sem-folha no limite do agente (contra RF-30/D-04/RF-35); IDs vêm direto do LLM sem `ResolveForWrite` (contra RF-13/RF-14 agent-side). M-04=0 só porque transactions rejeita downstream. | Revalidar categoria no fluxo gated (F-01); tornar `subcategoryId` obrigatório quando a direção exigir folha. |
| F-09 | medium | internal/agents/application/usecases/register_attempt.go | 47-49, 173-176 | Decisão de transição de estado inicial (`credit_card && CardID==nil → AwaitingSlotCard`; `len==1 → AwaitingSlotConfirmation`) embutida na usecase com magic string `"credit_card"`, fora de um `Decide*` puro (RF-19/RF-36). | Extrair `DecideInitialSlot(...)` puro; substituir `"credit_card"` por constante/tipo fechado de forma de pagamento. |
| F-10 | low | internal/agents/application/workflows/pending_entry_workflow.go | 97-162 | `handleCardSlotResume` reimplementa inline expire/cancel/replace/reprompt em vez de delegar a um `Decide*` puro (como `handleSlotResume`/`handleConfirmationResume`). Regra de transição duplicada → risco de divergência. | Extrair `DecideCardSlotResume(state,msg,now)` puro; manter no handler só o IO de resolução do cartão. |
| F-11 | low | internal/agents/application/tools/classify_category.go | 120-144 | `classifyWriteDecision` recomputa razões de bloqueio (versão/outcome/ambiguidade/raiz==sub) dentro da tool. Restrição Técnica proíbe "decisão categorial complexa dentro da tool"; mitigado por deferir allow/block real a `IsWriteEligible`. | Expor a razão de bloqueio a partir de `internal/categories` em vez de recomputar. |
| F-12 | low | internal/agents/application/workflows/pending_entry_state.go | 170 vs 190 | Campo `Operation PendingOperation` (com `Kind`) aparenta não ser lido em produção; só `OperationKind` é usado. Estado redundante aumenta superfície de erro de serialização/merge-patch. | Remover `Operation`/`PendingOperation` se confirmado sem uso, ou unificar. |
| F-13 | low | internal/agents/application/tools/register_expense.go | 108 | `if installments <= 0 { installments = 1 }` — única lógica condicional sobre campo de valor dentro de tool; aceitável (campo opcional, validação real no command), registrado por rigor. | Mover default para o schema/command constructor. |

## Files Reviewed

- internal/agents/application/tools/create_recurrence.go
- internal/agents/application/tools/edit_entry.go
- internal/agents/application/tools/register_expense.go
- internal/agents/application/tools/classify_category.go
- internal/agents/application/workflows/pending_entry_workflow.go
- internal/agents/application/workflows/pending_entry_decisions.go
- internal/agents/application/workflows/pending_entry_state.go
- internal/agents/application/workflows/pending_category_candidate.go
- internal/agents/application/workflows/category_resolution.go
- internal/agents/application/usecases/register_attempt.go
- internal/agents/application/usecases/pending_entry_continuer.go
- internal/agents/application/usecases/register_entry.go
- internal/agents/application/interfaces/categories_reader.go / transactions_ledger.go / discriminators.go / types.go
- internal/agents/infrastructure/binding/categories_reader_adapter.go / transactions_ledger_adapter.go
- internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go
- internal/agents/infrastructure/jobs/handlers/confirm_reaper_job.go
- internal/agents/module.go
- internal/platform/agent/identity_context.go
- internal/agents/application/agents/pending_entry_harness_test.go + workflows/*_test.go

## Critérios de Aceite Verificados

- CA-01: **não atendido** — resposta "custo fixo" em `AwaitingSlotCategory` → reprompt→cancel (F-03).
- CA-02: atendido — `TestG7_01`/`DecideNewOperationReplacement`; substituída é status fechado.
- CA-03: atendido — `TestG7_07`/`DecidePendingResume_SimNaoPix`.
- CA-04: atendido (register) — `EnrichCandidatesFromSearch` + `executeWrite` revalida.
- CA-05: atendido — cancelamento no gate (`TestG12_02`).
- CA-06: atendido — `executeWrite` só emite sucesso com ref real (`TestG7_15`).
- CA-07: atendido com ressalva — idempotência via módulo transactions; branch de replay morto (F-07).
- CA-08: atendido — expiração (`TestG7_08`/`TestResume_Confirmation_Expired`).
- CA-09: atendido (register) — `ResolveForWrite` bloqueia raiz-sem-folha.
- CA-10: atendido — `handleCardSlotResume` (`TestG7_16`).
- CA-11: atendido com ressalva — só o turno de substituição é exercido (L).
- CA-12: atendido parcialmente — harness valida estado/escrita/RunStatus; step-audit no-op (F-04/medium).
- CA-13: atendido — `TestG12_01` caminho inequívoco → confirmação obrigatória.
- CA-14: atendido — `TestG12_03` reprompt único → cancela.
- CA-15: **não atendido** — `DecideCategoryChoice` não ligado ao caminho vivo (F-03).
- CA-16: **não atendido** — `create_recurrence` escreve sem confirmação (F-01).
- CA-17: **não atendido** — `edit_entry` não carrega `Kind`/categoria; persist falha/corrompe (F-02).

## Decisões Funcionais Verificadas

- D-01..D-08: atendidas (escopo, substituição, pendência única, contrato canônico, 30min, harness M-01, scorer complementar).
- D-10: **parcialmente atendida** — gate universal vale para register_expense/income, mas recorrência escreve sem confirmação (F-01).
- D-11: atendida (gate como `AwaitingSlotConfirmation`, separado do fluxo destrutivo).
- D-12: atendida na delegação (`CreateRecurringTemplate` não reimplementa), mas o gate é violado (F-01).
- D-13: **parcialmente** — target/version preservados na edição, mas Kind/categoria perdidos (F-02); seleção número/nome não wired (F-03).

## Validações Executadas

- `go build ./...` → exit 0
- `go vet ./internal/agents/... ./internal/platform/...` → exit 0
- `go test -count=1 ./internal/agents/... ./internal/platform/agent/... ./internal/platform/workflow/...` → todos `ok`
- Gates governança (zero-comentários, SQL em adapter, kernel sem domínio, merge-patch resume, switch intent.Kind, estado string-livre) → todos vazios (pass)
- golangci-lint v2.12.2 disponível (não executado nesta rodada)

> Nota crítica: a suíte determinística passa **verde**, mas os achados F-01/F-02/F-03 sobrevivem porque o harness dirige `BuildPendingEntryWorkflow` diretamente via `engine.Start/Resume`, sem passar pelas tools reais (`create_recurrence`, `edit_entry`) nem pelo loop `AgentRuntime`. O verde do harness é um falso positivo de aceite para CA-16/CA-17/M-07 e para o fluxo de clarificação categorial.

## Ciclo Recomendado

Priorizar remediação na ordem F-01 → F-02 → F-03 (bloqueantes), depois F-04..F-09. Validar com `RUN_REAL_LLM=1` (obrigatório para mudanças de agente) além da suíte determinística. Reexecutar review pós-bugfix com `AI_REVIEW_PRIOR_SHA`.
