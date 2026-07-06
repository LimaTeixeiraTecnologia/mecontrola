# Análise Crítica — Módulo `internal/agents`

**Data:** 2026-07-06 · **Modo:** estrito, sem flexibilização · **Escopo:** análise apenas (nenhum código alterado)
**Base:** working tree em HEAD `e03c2a1` (com modificações não commitadas) · Go 1.26.4
**Evidência executada:** build normal, `go test` unit, `go vet -tags integration`, real-LLM via OpenRouter (`.env`), gates de governança, 4 subagentes especializados + verificação manual.

---

## 1. Sumário Executivo

**Status geral: NÃO PRONTO.**
**Nota de robustez: 6 / 10.**

Justificativa: a arquitetura de fronteira, o caminho de criação de lançamentos e o runtime auditável são sólidos e foram validados com LLM real (agentes 23s / scorers 59s, exit 0). Porém existem **2 defeitos CRÍTICOS** que atingem o usuário e passam despercebidos pela suíte de testes: (1) **as 4 operações de recorrência falham em runtime** com `ErrUsecaseUnauthorized` porque o adapter de recorrência não injeta `auth.Principal`; (2) **o pacote de testes de integração de `binding` não compila** (chamada obsoleta a `NewCreateTransaction`), deixando CA09-reconciled e transactions-integration mortos — introduzido pelo próprio commit HEAD `e03c2a1`. Somam-se `adjust_allocation` com IDOR (userId vindo do LLM) e runs de confirmação suspensos que viram órfãos e bloqueiam o usuário. Não pode ir para main nem ser usado por usuários reais até correção.

---

## 2. O que Está Bom

1. **Provider único OpenRouter, sem fallback/circuit breaker** — `module.go:123` `llm.NewOpenRouterProvider`; nenhum client openai/anthropic/gemini/mistral. Conforme R-AGENT-WF-001.4.
2. **LLM só nas 3 call-sites sancionadas** — loop de tool-calling (`agent.go:186`), step `Stream` (`agent.go:264`, `onboarding_workflow.go:251`), scorer LLM-judged (`scorer/llm_judged.go:91`). Zero LLM dentro de tool `exec` ou do kernel.
3. **Loop de tool-calling é bounded** — `agent.go:184` `for round := 0; round < a.maxToolRounds`, com `WithMaxToolRounds(12)` (`mecontrola_agent.go:143`); exaustão → `ErrMaxToolRounds` (falha limpa, sem loop infinito).
4. **Idempotência DB-level no caminho de create** — `UNIQUE (wamid, item_seq, operation)` (`migrations/000001_initial_schema.up.sql:2291`) + `ON CONFLICT DO NOTHING` + tratamento de `UniqueViolation` como no-op (`write_ledger_repository.go:67,82-84`).
5. **Runtime totalmente auditável** — todo `Run` inserido com `status=running` antes de executar e fechado com `status/outcome/error/EndedAt/DurationMs` (`runtime.go:102-114,244-259`); workflow com `StepRecord` e CAS otimista (`engine.go:471-521`).
6. **Resume por merge-patch seguro** — aplica `MergePatch` sobre o snapshot (fonte única), valida decode antes de executar, resume vazio é no-op (`engine.go:216-228`). Conforme R-WF-KERNEL-001.7.
7. **HITL com estado fechado persistido antes da pergunta** — `AwaitingApproval`/`OperationKind` como enums fechados com `IsValid`/`Parse*` (`confirm_state.go:11-104`); resume antes do parse no consumer (`whatsapp_inbound_consumer.go:144→176`).
8. **Fronteira cross-module limpa na aplicação** — todas as interfaces em `application/interfaces/` com tipos próprios (`types.go`/`errors.go`); nenhuma struct concreta de outro módulo vaza para `tools/`/`usecases/`; zero SQL em tools/consumers; adapters não compartilham transação com outro módulo.
9. **Kernel puro** — `internal/platform/workflow` não importa nenhum pacote de domínio (grep vazio). Conforme R-WF-KERNEL-001.1.
10. **Zero comentários, zero panic de produção, zero `context.Background/TODO`** em `internal/agents` (gates vazios). Testes de usecase 100% whitebox testify/suite com mocks mockery.
11. **Guard anti-simulação** — `runtime.go:203-217` impede resposta de sucesso alucinada quando a escrita falha (validado real-LLM: EP01).
12. **Retention job robusto** — cancelável via `ctx.Done()` por batch, delete limitado por `LIMIT`, timeout 5min, registrado no worker (`purge_ledger.go:74-101`, `worker.go:323`).

---

## 3. Gaps Encontrados

### CRÍTICO

**G-01 — As 4 operações de recorrência falham em runtime com `ErrUsecaseUnauthorized`.**
- Severidade: **CRÍTICO**
- Local: `internal/agents/infrastructure/binding/recurrence_manager_adapter.go` (todo o arquivo)
- Impacto: `create_recurrence`, `list_recurrences`, `update_recurrence`, `delete_recurrence` (4 de 24 tools) estão quebradas. A comunicação com `internal/transactions` (recurring-template usecases) é rejeitada na porta de autorização.
- Evidência: o adapter **não tem nenhuma referência a `auth`/`Principal`** (verificado: grep vazio). O único bridge do módulo está em `transactions_ledger_adapter.go:54-66` (`principalCtx` → `auth.WithPrincipal`). O inbound path **não** seta `auth.Principal` globalmente (grep vazio em `runtime.go`/`handle_inbound.go`/`whatsapp_inbound_consumer.go`). Os usecases exigem principal: `create_recurring_template.go:54-56`, `list_recurring_templates.go:44-46`, `update_recurring_template.go:53-55`, `delete_recurring_template.go:41-43` retornam `ErrUsecaseUnauthorized`. Execução destrutiva de recorrência (`destructive_confirm_workflow.go:341-355`) também é principal-less.
- Por que passou: `recurrence_manager_adapter.go` **não tem teste** e nenhum teste de integração/real-LLM exercita recorrência.

**G-02 — Pacote de testes de integração de `binding` não compila.**
- Severidade: **CRÍTICO** (regressão de qualidade / rede de segurança morta)
- Local: `internal/agents/infrastructure/binding/ca09_reconciled_integration_test.go:86`, `internal/agents/infrastructure/binding/transactions_integration_test.go:109`
- Impacto: `go test -tags integration ./internal/agents/infrastructure/binding/` → `[build failed]`. Os testes CA09-reconciled (idempotência/reconciliação) e transactions-integration **nunca rodam**.
- Evidência: `not enough arguments in call to txusecases.NewCreateTransaction` — falta o argumento `CategoryWriteGate`, adicionado ao construtor no commit HEAD `e03c2a1` (task 4.0 CategoryWriteEvidence). Os testes não foram atualizados. `go vet -tags integration ./internal/agents/...` reporta o erro; os pacotes `application/agents` e `application/scorers` compilam normalmente.

### ALTO

**G-03 — `adjust_allocation` confia em `userId` fornecido pelo LLM (formato IDOR) e é inutilizável.**
- Severidade: **ALTO**
- Local: `internal/agents/application/tools/adjust_allocation.go:15,35,40,64`
- Impacto: identidade de autorização vem do modelo (`UserID string json:"userId"`, required no schema) e é repassada a `BudgetPlanner.EditCategoryPercentage` sem checagem de principal (`budget_planner_adapter.go:131-136`). Todas as outras tools derivam identidade de `agent.InboundIdentityFromContext(ctx)` (ex.: `register_income.go:63`). Além do risco de escopo cruzado, o modelo nunca recebe o UUID do usuário → a tool também é **inoperante** na prática.

**G-04 — Run de confirmação suspenso vira órfão e bloqueia o usuário indefinidamente.**
- Severidade: **ALTO**
- Local: `internal/platform/workflow/infrastructure/postgres/store.go:294-303` (`DeleteCompleted` só purga `succeeded`/`failed`); `destructive_confirm_workflow.go:83` (TTL avaliado só no resume); `delete_entry.go:104-110` (bloqueio)
- Impacto: confirmação abandonada fica `Suspended` para sempre. `engine.Start` → `ErrRunAlreadyExists` (`engine.go:109-112`) faz toda nova operação destrutiva responder "Há uma confirmação pendente". Não há reaper: `ListSuspended` (`store.go:256`) existe mas **não tem nenhum chamador em produção** (verificado). Os reapers registrados são só de outbox/budgets.

**G-05 — `MessageID`/`wamid` não é obrigatório em nenhuma fronteira, mas a idempotência depende dele.**
- Severidade: **ALTO** (gap latente)
- Local: `inbound_input.go:16-31` (Validate não checa `MessageID`); `whatsapp_inbound_consumer.go:133` (não checa `message_id`)
- Impacto: `wamid` vazio → escrita externa OK, depois `Insert` falha no CHECK `length(wamid)>0` (`migrations/000001_initial_schema.up.sql:2288`) → `idempotent_write.go:121` retorna erro após a transação já criada → na redelivery escreve de novo. Em produção `wamid` vem de `msg.WAMID` (`module.go:337`), mas sem defesa-em-profundidade.

**G-06 — Vazamento de camada `domain` de outros módulos dentro do ACL de `binding` (3 ocorrências).**
- Severidade: **ALTO** (fronteira arquitetural literal)
- Local/Evidência:
  - `card_manager_adapter.go:15` importa `internal/card/domain` para `ErrNicknameConflict`/`ErrCardNotFound` (definidos só em `internal/card/domain/errors.go:6-7`).
  - `budget_planner_adapter.go:15` importa `internal/budgets/domain/entities` para `ErrBudgetAlreadyActive` (`budget.go:17`) — inconsistente: o mesmo arquivo usa `budgetsifaces.ErrBudgetNotFound` (application) em `:155`.
  - `categories_reader_adapter.go:14` importa `internal/categories/domain/valueobjects` e chama `catvos.ParseKind` (`kind.go:20`).
- Impacto: confinado ao ACL (não vaza para `application/`), mas cada um cruza a fronteira `domain/` de outro módulo. Causa-raiz: os módulos-fonte não expõem esses sentinels/parser na camada `application`.

### MÉDIO

**G-07 — Regra de classificação de categoria duplicada 3× (lógica de domínio em tool e workflow).**
Local: `register_entry.go:192-236` (`classify`), `classify_category.go:120-141` (`classifyWriteDecision`), `destructive_confirm_workflow.go:286-304` (`isValidClassifyResult`). Mesma política (`version<=0`, `outcome!="matched"`, múltiplos candidatos, `RootCategoryID==CategoryID`). Viola R-AGENT-WF-001.2 (tool/workflow sem regra de domínio) e cria risco de drift. Extrair um predicado único.

**G-08 — Discriminadores de estado como `string` livre.**
Local: `ConfirmState.TargetKind` (`confirm_state.go:110`, branch `case "card"` em `destructive_confirm_workflow.go:376` e `delete_entry.go:83`); `EntryRef.Kind`/`Entry.Kind`/`MonthlyEntry.Kind`/`CategorySearchResult.Outcome`/`CategoryWriteDecision.Kind` (`interfaces/types.go:11,61,182,284,300`) comparados como string (`register_entry.go:195`, `destructive_confirm_workflow.go:290`). Deveriam ser tipos fechados (state-as-type).

**G-09 — Política de confirmação decidida dentro de tool.**
Local: `update_card.go:94` (branch `DueDay==nil` decide write direto vs gate); `delete_entry.go:83` (`EntryKind=="card"` decide `OperationKind`). Regra de negócio em adapter/tool.

**G-10 — Ordering write-antes-do-ledger sem lock no módulo.**
Local: `idempotent_write.go:84` (write) antes de `:100` (Insert). Sob concorrência do mesmo `wamid`, ambos passam `FindByKey` → dupla escrita externa; `ON CONFLICT` deduplica só a linha do ledger. Correção depende inteiramente da serialização per-user do outbox.

**G-11 — TTL de confirmação consome a próxima mensagem não relacionada.**
Local: `destructive_confirm_workflow.go:83-87` + retorno `handled=true` (workflow:183-191). Ao expirar no resume, a próxima mensagem qualquer do usuário é consumida pelo aviso "tempo expirou" e a intenção real é descartada.

**G-12 — Sem timeout por-inbound quando `InboundTimeout` não é configurado.**
Local: `whatsapp_inbound_consumer.go:138-142` e `module.go:239` (condicional a `deps.InboundTimeout > 0`). Loop LLM de até 12 rounds sem deadline no nível do agente.

**G-13 — Reconciliação com branching de domínio no card adapter.**
Local: `card_manager_adapter.go:68-81` — `CreateCard` captura `ErrNicknameConflict`, lista cards e faz loop `c.Nickname == in.Nickname` para sintetizar `CardRef`. Lógica de idempotência/reconciliação em adapter fino (borderline R-ADAPTER-001.2).

### BAIXO

**G-14 — Buracos de cobertura.** Sem teste: `recurrence_manager_adapter.go` (mascara G-01), `budget_planner_adapter.go`, `purge_ledger.go`, `ledger_retention_job.go`; nenhum teste de integração/real-LLM de recorrência.
**G-15 — `messages.Append` ignorado** (`runtime.go:156,164`; `onboarding_workflow.go:709`) — perda silenciosa de histórico sem log.
**G-16 — `ConfirmState.MessageID` persistido mas nunca usado como replay-guard** (`confirm_state.go:113`).
**G-17 — Execuções destrutivas fora do `IdempotentWrite`** (`destructive_confirm_workflow.go:306-373`); `executeRegister` (`:246-277`) depende de `OriginWamid` no draft.
**G-18 — Fidelidade do scorer** (`scoring_hooks.go:62` `Name = toolID`) sem asserção — risco de drift silencioso do M-04.
**G-19 — `ListMonthlyEntries` type-assert `[]any` com `continue` silencioso** (`transactions_ledger_adapter.go:180-183`) — mudança de schema vira resultado vazio, não erro.
**G-20 — Retention loop sem pausa entre batches** (`purge_ledger.go:74-94`).

---

## 4. Ressalvas

1. **`maxToolRounds` é teto fixo (12)** — generoso, mas fluxos multi-tool longos truncam em `ErrMaxToolRounds` em vez de degradar.
2. **Idempotência do ledger cobre só create_expense/create_income** — edit/delete/card/recurrence dependem do confirm workflow + `Version` otimista; documentar para não presumir dedup universal.
3. **Acoplamento a structs concretas de usecase nos 5 adapters** (`*txusecases.*`, `*cardusecases.*`, etc., `module.go:140-186`) — padrão ACL aceito, mas é a maior superfície de acoplamento a monitorar.
4. **Onboarding agent construído com `nil` tools** (`module.go:198`) — correto (sub-agente text-only), mas significa que a jornada de onboarding não tem ferramentas.
5. **`register_expense` normaliza `installments<=0→1`** (`register_expense.go:85`) — trivial, mas é normalização de input em tool.

---

## 5. Falsos Positivos

1. **Unique constraint `agents_write_ledger` "garante" idempotência** — na verdade deduplica só a linha do ledger. Sob concorrência do mesmo `wamid`, a escrita externa ocorre antes do `Insert` e **não** é impedida; a proteção real é a serialização per-user do outbox + `OriginWamid` do transactions, não a constraint.
2. **TTL de confirmação "limita" runs abandonados** — só é avaliado no resume. Um run abandonado nunca é ativamente expirado; a "proteção" só dispara quando o usuário manda outra mensagem (que é então descartada — G-11).
3. **`ListSuspended` parece housekeeping de suspensos** — é código morto (zero chamadores). Nenhum reaper de runs suspensos roda (G-04).
4. **Teste real-LLM `ToolCoverage_All22Tools` passando parece cobrir todas as tools end-to-end** — exercita a **seleção** de tool pelo LLM, não o caminho de escrita autorizado. As tools de recorrência passam na seleção mas falham em runtime (G-01), pois nenhum teste as leva pelo usecase com principal.
5. **Schema `Strict:true` das tools parece garantir segurança** — valida forma do input, não a proveniência de identidade/autorização. `adjust_allocation` tem schema strict e ainda assim aceita `userId` do LLM (G-03).
6. **Guard anti-simulação (`runtime.go:203-217`) parece proteger todas as escritas** — protege contra sucesso alucinado, mas faz G-01 aparecer como falha honesta ao usuário, não como funcionalidade; não corrige a quebra.

---

## 6. Plano de Ação (ordenado por prioridade)

1. **Injetar `auth.Principal` no caminho de recorrência.** Espelhar `transactions_ledger_adapter.go:54-67` em `recurrence_manager_adapter.go` (ou setar principal uma vez no runtime a partir de `InboundIdentity`). — **backend** · **S** · Critério: `create/list/update/delete_recurrence` executam com sucesso em teste de integração com principal; sem `ErrUsecaseUnauthorized`.
2. **Corrigir a compilação dos testes de integração de `binding`.** Atualizar as chamadas de `NewCreateTransaction` (`ca09_reconciled_integration_test.go:86`, `transactions_integration_test.go:109`) para incluir `CategoryWriteGate`. — **backend/QA** · **XS** · Critério: `go test -tags integration ./internal/agents/...` compila e passa; adicionar gate CI que quebra o build se o pacote não compilar.
3. **Remover `userId` do input de `adjust_allocation`; derivar de `InboundIdentityFromContext`.** — **backend/arquiteto** · **XS** · Critério: schema sem `userId`; tool usa identidade do ctx; teste cobre negação de escopo cruzado.
4. **Adicionar reaper de runs suspensos por TTL** consumindo `ListSuspended`, e/ou avaliar TTL fora do resume para não consumir a próxima mensagem. — **backend** · **M** · Critério: run de confirmação abandonado é cancelado após TTL sem exigir mensagem do usuário; usuário não fica bloqueado; a mensagem real seguinte segue para o parse.
5. **Tornar `MessageID`/`wamid` obrigatório** em `inbound_input.Validate()` e no consumer. — **backend** · **XS** · Critério: inbound sem `MessageID` é rejeitado na fronteira, antes de qualquer escrita.
6. **Expor sentinels/parser na camada `application` dos módulos-fonte** (`card`, `budgets`, `categories`) e remover os 3 imports de `domain` no ACL. — **arquiteto/backend** · **S** · Critério: grep de `internal/{card,budgets,categories}/domain` em `internal/agents` (não-teste) retorna vazio.
7. **Extrair predicado único de classificação/write-eligibility** e reusar em `register_entry`, `classify_category`, `destructive_confirm_workflow`. — **backend** · **M** · Critério: uma única fonte da regra; tools/workflow sem branching de domínio.
8. **Promover discriminadores a tipos fechados** (`ConfirmState.TargetKind`, `*.Kind`, `Outcome`). — **backend** · **M** · Critério: nenhuma `string` livre de estado em assinatura pública; branches por tipo fechado.
9. **Cobrir os arquivos sem teste** (recurrence/budget adapters, `purge_ledger`, retention job) + teste de integração/real-LLM de recorrência. — **QA/backend** · **M** · Critério: cobertura > 0 nos arquivos citados; recorrência com teste real-LLM verde.
10. **Garantir timeout por-inbound sempre configurado** (default não-zero) e logar `messages.Append` falho. — **backend/DevOps** · **S** · Critério: loop LLM sempre com deadline; falha de histórico logada.

---

## 7. Parecer sobre Produção

**Não pode ser usado por usuários reais agora.**

Pré-requisitos mínimos (bloqueantes) antes de liberar:
1. G-01 corrigido — recorrência funcional (item 1 do plano).
2. G-02 corrigido — testes de integração de `binding` compilando e passando + gate CI (item 2).
3. G-03 corrigido — `adjust_allocation` sem `userId` do LLM (item 3).
4. G-04 mitigado — reaper/TTL de runs suspensos para não bloquear o usuário (item 4).
5. G-05 corrigido — `MessageID` obrigatório na fronteira (item 5).

O restante (ALTO G-06, MÉDIOs, BAIXOs) pode ser priorizado em sequência, mas G-06 deve entrar antes do próximo release por ser fronteira arquitetural literal.

---

## 8. Registro de Suposições

1. **Testes DB-backed não re-executados nesta análise.** `TestMeControlaAgentE2ESuite` e `ca03_honest_confirmation_integration_test` (application/agents) **compilam** sob `-tags integration`, mas exigem Postgres (testcontainers) e não foram rodados aqui (Docker não acionado). Assumido que passam conforme execuções anteriores registradas (memória 2026-07-04 / 2026-07-06 real-LLM validado); não é evidência desta rodada.
2. **CA09-reconciled e transactions-integration NÃO são suposição — são gap (G-02):** não rodam porque não compilam.
3. **Real-LLM executado de fato:** `application/agents` (6 testes, 23,0s, exit 0) e `application/scorers` (22-tools + EP01 + EP05, 59,4s, exit 0), via `OPENROUTER_BASE_URL`/`OPENROUTER_API_KEY` do `.env` com `RUN_REAL_LLM=1`.
4. **Base = working tree com modificações não commitadas** (git status na abertura mostra vários `M`). A análise reflete o estado atual do working tree, não o último commit isolado.
5. **`OriginWamid` no draft de `executeRegister`** (G-17) não foi rastreado até a origem; assumido presente mas não verificado end-to-end.
