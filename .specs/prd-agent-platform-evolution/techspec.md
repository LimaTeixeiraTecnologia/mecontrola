<!-- spec-hash-prd: 88de69d62688347098bf7f771c6405e496b424fda2fc9dcbf0c5f513b61eaf56 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Evolução da Plataforma de Agentes (MVP: Human-in-the-Loop)

## Resumo Executivo

Esta techspec especifica o **MVP** da iniciativa: **capacidade B — Human-in-the-Loop (HITL)
para ações destrutivas/sensíveis** (deletar último lançamento, editar último lançamento, deletar
cartão e confirmar a reconfiguração de budget no commit). As capacidades **A (plano multi-tool)** e
**C (recuperação + memória)** do PRD ficam como **fases posteriores**, cada uma com seu próprio gate
de implementação; esta techspec registra suas decisões de produto já resolvidas, mas **detalha
apenas B** para garantir um MVP com zero lacunas.

A entrega assenta sobre uma **correção fundacional do kernel** (`internal/platform/workflow`): o
`Engine.Resume` hoje **substitui** o estado inteiro do snapshot pelo payload de resume
(`engine.go:172-177`), o que perderia os dados da operação suspensa. Trocaremos a substituição por
**JSON merge-patch** (RFC 7386) — o resume passa a carregar apenas o delta (ex.: `{"ResumeText":"sim"}`)
aplicado sobre o `Snapshot.State`. Isso corrige um defeito latente para **todos** os consumidores do
kernel mantendo-o genérico (R-WF-KERNEL-001), e torna o snapshot a **fonte única de verdade** no
resume — eliminando a necessidade de side-store de draft.

Sobre essa base, os quatro fluxos destrutivos migram do caminho legacy (`dispatchWrite` →
`IntentWorkflow`) para um **único workflow de confirmação no kernel** (`destructive_confirm`), com um
passo `confirm` que suspende aguardando aprovação humana e retoma de forma idempotente, durável e
auditável. O HITL nasce **sempre ligado** em produção (sem feature flag), por decisão do solicitante;
o risco operacional dessa escolha é mitigado pelo fato de a mudança ser **aditiva-de-segurança**
(introduz confirmação, não nova capacidade destrutiva) e coberta por testes de não regressão.

## Arquitetura do Sistema

### Visão Geral dos Componentes

**Plataforma — `internal/platform/workflow` (modificado):**
- `engine.go` (**modificado**): `Resume` aplica **JSON merge-patch** do payload sobre `Snapshot.State`
  em vez de substituir. Novo helper interno `mergeStatePatch(base, patch []byte) ([]byte, error)`.
- `codec.go` (**modificado**): adiciona `MergePatch(base, patch []byte) ([]byte, error)` (genérico,
  opera sobre JSON; sem conhecimento de domínio).
- `store.go` (**inalterado**): `ResumeApplier[S]` permanece como hook não usado; documentado como
  alternativa rejeitada na ADR-001.

**Agent — `internal/agent` (novo/modificado):**
- `domain/confirmation/draft.go` (**novo**): `ConfirmState` (estado do kernel para HITL) + tipos
  fechados `OperationKind` e `AwaitingApproval` (DMMF state-as-type).
- `application/workflow/destructive_confirm.go` (**novo**): `NewDestructiveConfirmDefinition` —
  `platform.Definition[ConfirmState]` com `Sequence(authorize, replay, policy, audit_begin, prepare,
  confirm, execute, format)`. Reutiliza os passos de guarda existentes (`steps/authorize.go`,
  `replay.go`, `policy.go`, `audit_begin.go`).
- `application/workflow/steps/prepare_target.go` (**novo**): resolve o alvo da operação (último
  lançamento, cartão, alocação de budget) e compõe o prompt de confirmação; short-circuit se o alvo
  não existir.
- `application/workflow/steps/confirm_gate.go` (**novo**): passo de suspend/resume da aprovação
  humana (semântica estrita + TTL + re-prompt-1x).
- `application/workflow/steps/execute_destructive.go` (**novo**): efetiva a mutação despachando por
  `OperationKind` para o binding/usecase existente (sem regra de negócio nova).
- `application/services/daily_ledger_agent.go` (**modificado**): roteia os 4 kinds destrutivos para
  o novo caminho HITL via **registry** (não por `switch case` crescente); adiciona
  `continuePendingApproval` (resume **antes** do `ParseInbound`).
- `infrastructure/binding/` (**reuso**): `LastTransactionDeleterAdapter`,
  `LastTransactionEditorAdapter`, `CardDeleterAdapter`, `BudgetConfigCommitterAdapter` — chamados
  pelo passo `execute`, sem alteração de contrato de mutação.

### Fluxo de Dados (HITL)

```
inbound → IntentRouter.route
  ├─ continuePendingExpenseConfirmation (categoria — existente)
  ├─ continuePendingApproval (NOVO): Engine.Resume(destructive_confirm, key, {"ResumeText": <texto>})
  │     ├─ run suspenso encontrado → confirm_gate interpreta confirm/cancel/ambíguo/expirado
  │     │     ├─ confirmado → execute (mutação) → format → succeeded
  │     │     ├─ cancelado  → short-circuit (sem efeito) → succeeded
  │     │     ├─ ambíguo 1ª vez → re-prompt → suspended
  │     │     ├─ ambíguo 2ª vez → cancela → succeeded
  │     │     └─ expirado (TTL) → cancela sem efeito → succeeded → handled=false (parse o novo texto)
  │     └─ nenhum run suspenso → handled=false
  └─ ParseInbound → kind.IsWrite() && kind ∈ destrutivos
        → Engine.Start(destructive_confirm, key, initial)
            → authorize→replay→policy→audit_begin→prepare→confirm(suspende)→ aguarda resposta
```

`correlationKey = "<user_id>:<channel>"` (mesmo padrão do fluxo de categoria). O workflow de
confirmação tem **ID próprio** (`destructive_confirm`), distinto de `transactions_write`, então o
`Engine.Resume` por `(workflow, key)` é não-ambíguo: há no máximo um run suspenso por workflow por
chave.

## Design de Implementação

### Interfaces Chave

**Kernel — merge-patch no resume (genérico, sem domínio):**

```go
func (c Codec[S]) MergePatch(base, patch []byte) ([]byte, error) {
    var baseMap, patchMap map[string]any
    if err := json.Unmarshal(base, &baseMap); err != nil {
        return nil, fmt.Errorf("workflow: codec: merge base: %w", err)
    }
    if err := json.Unmarshal(patch, &patchMap); err != nil {
        return nil, fmt.Errorf("workflow: codec: merge patch: %w", err)
    }
    merged := mergeMaps(baseMap, patchMap)
    out, err := json.Marshal(merged)
    if err != nil {
        return nil, fmt.Errorf("workflow: codec: merge marshal: %w", err)
    }
    return out, nil
}
```

`Engine.Resume` (`engine.go`) substitui o bloco de replace por:

```go
if len(resume) > 0 {
    mergedBytes, mErr := e.codec.MergePatch(snap.State, resume)
    if mErr != nil {
        return RunResult[S]{}, fmt.Errorf("workflow.engine.resume: merge resume: %w", mErr)
    }
    merged, dErr := e.codec.Decode(mergedBytes)
    if dErr != nil {
        return RunResult[S]{}, fmt.Errorf("workflow.engine.resume: decode merged: %w", dErr)
    }
    current = merged
}
```

**Agent — tipos fechados (DMMF state-as-type):**

```go
type OperationKind int

const (
    OperationDeleteLast OperationKind = iota + 1
    OperationEditLast
    OperationDeleteCard
    OperationBudgetCommit
)

type AwaitingApproval int

const (
    AwaitingNone AwaitingApproval = iota
    AwaitingConfirm
)
```

(ambos com `String()`, `IsValid()` e `Parse*` — espelhando `RunStatus`/`AwaitingKind` existentes.)

**Agent — workflow de confirmação (consome o kernel):**

```go
func NewDestructiveConfirmDefinition(deps DestructiveConfirmDeps) platform.Definition[ConfirmState] {
    root := platform.Sequence[ConfirmState]("destructive_confirm_seq",
        steps.NewAuthorize(deps.Authorize, deps.DenyReply),
        steps.NewReplay(deps.Replay),
        steps.NewPolicy(deps.Policy),
        steps.NewAuditBegin(deps.AuditBegin, deps.OnSettle, deps.ReplayReply, deps.AuditFailReply),
        steps.NewPrepareTarget(deps.Targets),
        steps.NewConfirmGate(deps.TTL),
        steps.NewExecuteDestructive(deps.Executors),
        steps.NewFormat(formatDestructiveReply),
    )
    return platform.Definition[ConfirmState]{
        ID:          DestructiveConfirmWorkflowID,
        Root:        root,
        Durable:     true,
        MaxAttempts: deps.MaxAttempts,
    }
}
```

**Passo `prepare` e `execute` despacham por `OperationKind` via mapa (não `switch` de domínio):**

```go
type TargetResolver func(ctx context.Context, state ConfirmState) (ConfirmState, error)
type DestructiveExecutor func(ctx context.Context, state ConfirmState) (ConfirmState, error)

type PrepareTargetDeps struct{ Targets map[OperationKind]TargetResolver }
type ExecuteDestructiveDeps struct{ Executors map[OperationKind]DestructiveExecutor }
```

Cada `TargetResolver`/`DestructiveExecutor` é um adapter fino sobre os bindings existentes
(`LastTransactionDeleterAdapter`, `LastTransactionEditorAdapter`, `CardDeleterAdapter`,
`BudgetConfigCommitterAdapter`) — **sem** regra de negócio, SQL ou branching de domínio (R-ADAPTER-001.2).

### Modelos de Dados

**`ConfirmState` (estado serializado pelo codec do kernel — JSON):**

```go
type ConfirmState struct {
    UserID      uuid.UUID
    Channel     string
    MessageID   string
    Confidence  float64
    Operation   OperationKind

    TargetTransactionID string
    TargetVersion       int64
    NewAmountCents      int64
    CardID              string
    CardHint            string
    BudgetDraftJSON     []byte

    PromptText      string
    Awaiting        AwaitingApproval
    RepromptCount   int
    SuspendedAt     string

    ResumeText  string
    DecisionID  uuid.UUID
    Outcome     tools.ToolOutcome
    Reply       string
    ShortCircuit bool
}

func (s ConfirmState) IsDone() bool { return s.ShortCircuit }
```

**Persistência:** reutiliza as tabelas do kernel `workflow_runs` + `workflow_steps` (migration
`000019_create_workflow_runtime`, já existente). **Nenhuma migration nova** é exigida pelo MVP — o
`ConfirmState` é serializado em `workflow_runs.state` (coluna existente). O TTL do gate é calculado
no resume a partir de `SuspendedAt` (gravado em UTC inline via `time.Now().UTC()`), não por coluna
dedicada.

**Sem side-store de draft:** com o merge-patch, o snapshot do kernel é a fonte única; `pending_action`
em `agent_sessions` **não** é usado por este fluxo (permanece exclusivo do fluxo de budget/categoria
legacy).

### Endpoints de API

Não aplicável — o fluxo é conversacional (WhatsApp/Telegram), sem novos endpoints HTTP.

## Pontos de Integração

- **Bindings existentes (mutação):** `b.transactionsModule.DeleteTransactionUC`,
  `GetTransactionUC`+`UpdateTransactionUC`, `b.cardModule.SoftDeleteCardUC`,
  `b.budgetsModule.CreateBudgetUC`+`ActivateBudgetUC` — chamados pelo passo `execute`, sem mudança de
  assinatura. O gate de budget intercepta **antes** do `ActivateBudgetUC` (commit), preservando o
  fluxo multi-turn existente até o ponto de commit.
- **Audit trail existente:** `decisionAuditor` (`decision_audit.go`) registra cada decisão; o passo
  `audit_begin` e o `OnSettle`/`SettleRegistry` já integram o Run ao decision-id (reuso 1:1 do
  padrão de `transactions_write`).

## Abordagem de Testes

### Testes Unitários

Seguem `R-TESTING-001` (testify/suite, whitebox, `fake.NewProvider()`, IIFE por mock).

- **Kernel merge-patch** (`codec_test.go`, `engine_test.go`): merge preserva campos do snapshot;
  delta sobrescreve apenas chaves presentes; `null` remove chave; resume vazio = no-op; **teste de
  regressão do defeito**: suspender com payload rico → resume com `{"ResumeText":"x"}` → asserts de
  que os campos originais sobrevivem.
- **`confirm_gate`** (`steps_test.go`): confirma → completed+proceed; cancela → short-circuit sem
  efeito; ambíguo 1ª vez → suspended (reprompt, `RepromptCount=1`); ambíguo 2ª vez → cancela;
  expirado (SuspendedAt > TTL) → cancela sem efeito.
- **`prepare_target`/`execute_destructive`**: alvo inexistente → short-circuit com mensagem; sucesso
  → outcome `routed`; cada `OperationKind` mapeado ao executor correto (mock do binding).
- **Roteamento** (`daily_ledger_agent_test.go`): os 4 kinds suspendem na 1ª mensagem (não executam);
  `continuePendingApproval` resume na 2ª; **não regressão**: kinds não-destrutivos inalterados.
- **Tipos fechados**: `OperationKind`/`AwaitingApproval` rejeitam valor inválido em `Parse*`.

### Testes de Integração

> Critérios atendidos: (1) fronteira de IO crítica (persistência de snapshot + CAS de versão no
> Postgres) onde mock não garante correção; (2) o defeito de resume só se manifesta com
> serialização/round-trip real. **Integration tests são recomendados e adotados.**

- `internal/platform/workflow/.../store_integration_test.go` (**estender**): suspender → recarregar de
  outro processo simulado → resume com delta → asserta efeito único (idempotência) e CAS de versão
  sob resume concorrente (a 2ª perde por `ErrVersionConflict`, efeito **não** duplica).
- HITL E2E (testcontainers Postgres, build tag `//go:build integration`): mensagem 1 (ex.: "apaga o
  último") → suspende (nada deletado no banco) → mensagem 2 ("sim") → deleta exatamente uma vez →
  mensagem 2 repetida (replay messageID) → sem segunda deleção.

### Testes E2E

Fluxo conversacional completo dos 4 cenários (delete/edit/card/budget-commit) cobrindo confirmar,
cancelar, ambíguo→reprompt→cancela, e expiração→fall-through para nova intenção.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Kernel merge-patch (fundacional)** — `codec.MergePatch` + `Engine.Resume` + testes de regressão
   do defeito. Pré-requisito de tudo; corrige o defeito latente isoladamente e mantém os fluxos
   atuais verdes (o fluxo de categoria kernel passa a recuperar estado corretamente).
2. **Tipos fechados + `ConfirmState`** (`domain/confirmation/`).
3. **Passos novos** (`prepare_target`, `confirm_gate`, `execute_destructive`) + reuso dos passos de
   guarda; testes unitários isolados.
4. **`NewDestructiveConfirmDefinition`** + wiring no `module.go` (engine `Engine[ConfirmState]`,
   resolvers/executors a partir dos bindings existentes).
5. **Roteamento no agent** — registry dos 4 kinds → HITL; `continuePendingApproval` antes do parse;
   gate de budget no ponto de commit.
6. **Governança + integração + E2E** — addendum de regra, integration/E2E, gates `R-*`.

### Dependências Técnicas

- Migration do kernel `000019_create_workflow_runtime` já aplicada (sem migration nova no MVP).
- Bindings de mutação já existentes e wired em `module.go`.

## Monitoramento e Observabilidade

Reuso da observabilidade do kernel (`workflow_runs_total`, `workflow_steps_total`,
`workflow_suspend_total`, `workflow_resume_total`, `workflow_version_conflict_total`) + métrica de
agent (`agent_intent_routed_total`) com outcomes existentes (`clarify`, `routed`, `usecase_error`).

- **Novos labels** apenas de enums fechados: `workflow="destructive_confirm"`, `step`,
  `operation` (= `OperationKind.String()`), `status`, `outcome`. **Proibido** `user_id`/`category_id`/
  `correlation_key` como label (R-WF-KERNEL-001.4 / R-AGENT-WF-001.5).
- **Logs-chave:** `agent.hitl.suspended`, `agent.hitl.confirmed`, `agent.hitl.cancelled`,
  `agent.hitl.expired`, `agent.hitl.reprompt`, com `operation` e `run_id` (sem PII).
- **Auditoria:** cada gate vira um Run + decision audit; aprovação/negação/expiração ficam
  rastreáveis por `decision_id`/`run_id`.

## Considerações Técnicas

### Decisões Chave

- **ADR-001** — Correção do resume do kernel via JSON merge-patch (substituição → merge).
- **ADR-002** — HITL sempre-on sobre o kernel para 4 operações destrutivas (sem feature flag) e
  unificação num único workflow `destructive_confirm`.
- **ADR-003** — Contrato de confirmação: estado fechado `AwaitingApproval`, semântica estrita +
  TTL + re-prompt único, idempotência por `messageID`.
- **ADR-004** — Gate de budget no ponto de commit (antes de `ActivateBudgetUC`), preservando o fluxo
  multi-turn existente.

### Riscos Conhecidos

- **HITL sempre-on, sem rollout gradual nem kill-switch** (escolha do solicitante): mudança de
  comportamento em produção (ops destrutivas passam a exigir confirmação). *Mitigação:* a mudança é
  **aditiva-de-segurança** (reduz risco, não adiciona poder destrutivo); cobertura de não regressão
  obrigatória; reversão exige deploy. *Risco residual aceito e registrado* (ADR-002).
- **Alteração no kernel afeta todos os consumidores** (categoria-clarificação inclusive).
  *Mitigação:* teste de regressão do defeito + paridade dos fluxos atuais (`parity_test.go`) antes do
  merge; merge-patch é semanticamente compatível com resume vazio (no-op).
- **Colisão de runs suspensos por chave** (categoria vs aprovação na mesma `(user,channel)`).
  *Mitigação:* workflow IDs distintos; a ordem de tentativa de resume é determinística
  (categoria → aprovação → parse); cada `Resume` retorna no-run-found quando não suspenso.
- **Expiração de gate deixa snapshot suspenso até o próximo inbound.** *Mitigação:* ao detectar TTL
  estourado no resume, o run **completa** como cancelado (housekeeping purga runs concluídos); o
  texto novo do usuário segue para o parse (`handled=false`).
- **`edit_last` usa o novo valor vindo do intent (LLM).** *Mitigação:* o prompt de confirmação
  **exibe** o valor/efeito antes de aplicar; confirmação estrita exigida.

### Conformidade com Padrões

- `R-WF-KERNEL-001`: a alteração do kernel permanece **genérica** (opera sobre JSON, sem import de
  domínio, sem regra de negócio, estados fechados, cardinalidade controlada).
- `R-AGENT-WF-001`: roteamento por **registry** (sem novo `case intent.Kind` no switch — gate .1);
  Tool/passos finos sem regra/SQL/branching de domínio (.2); LLM apenas no parse (.4); Run auditável
  (.5); resume antes do `ParseInbound` (.6/.7). **Addendum** estende .7 para cobrir o estado de
  espera `AwaitingApproval` (ADR-003 + atualização de `.claude/rules/agent-workflows-tools.md`).
- `R-ADAPTER-001`: zero comentários; adapters finos; sem SQL fora do adapter postgres do kernel.
- `R-TESTING-001`: suites whitebox testify; `R-DTO-VALIDATE-001` quando houver input DTO novo.
- `go-implementation` R0–R7: sem `init()`, sem `panic`, `context.Context` em IO, `errors.Join`/`%w`,
  goroutines canceláveis (paralelismo do kernel já cumpre), tempo via `time.Now().UTC()` inline.

### Arquivos Relevantes e Dependentes

- **Modificados (plataforma):** `internal/platform/workflow/codec.go`, `engine.go`,
  `internal/platform/workflow/engine_test.go`, `codec_test.go`.
- **Novos (agent):** `internal/agent/domain/confirmation/draft.go`,
  `internal/agent/application/workflow/destructive_confirm.go`,
  `internal/agent/application/workflow/steps/prepare_target.go`,
  `steps/confirm_gate.go`, `steps/execute_destructive.go`.
- **Modificados (agent):** `internal/agent/application/services/daily_ledger_agent.go`,
  `internal/agent/application/services/agent_workflows.go`,
  `internal/agent/application/services/intent_router.go` (KernelDeps), `internal/agent/module.go`.
- **Reuso (sem alteração de contrato):** `internal/agent/infrastructure/binding/transaction_query.go`,
  `cards_write.go`, `budget_config.go`; `steps/authorize.go`, `replay.go`, `policy.go`,
  `audit_begin.go`, `format.go`, `persist.go`.
- **Governança:** `.claude/rules/agent-workflows-tools.md` (addendum .7), `.claude/rules/workflow-kernel.md`
  (nota de merge-patch).

## Fases Posteriores (fora do MVP — registradas para rastreabilidade)

- **Fase A — Plano multi-tool determinístico:** `Plan = []Intent` aditivo na saída do parse (mantém
  `intent.Intent` single; 1 item = comportamento atual); executor itera via registry; **stop-on-first
  write-failure**, leituras seguintes prosseguem. Gate próprio; não detalhado aqui.
- **Fase C — Recuperação contextual estruturada + memória:** retrieval por query estruturada
  (histórico do usuário + taxonomia de categorias) injetado no `ContextBuilder`; resumo de histórico
  longo via pipeline assíncrono existente; memória observacional estruturada/versionada. Sem RAG
  vetorial. Gate próprio; não detalhado aqui.

Estas fases herdam integralmente os gates `R-*` e a fronteira kernel-vs-agent desta techspec.
