# Agent Workflows e Tools — Padrao Canonico

- Rule ID: R-AGENT-WF-001
- Severidade: hard
- Escopo: `internal/platform/agent/` (primitivo generico de agent; ex-`internal/agent`, descontinuado)
- Plano de origem: `docs/runs/2026-06-23-evolucao-dailyagent-mastra.md`
- PRD vigente: `.specs/prd-platform-mastra/prd.md` (spec-version 2)

## Emenda 2026-06-29 — Re-escopo para o primitivo de agent da plataforma; internal/agent descontinuado [HARD]

Origem: `.specs/prd-platform-mastra/prd.md` (spec-version 2).

O modulo `internal/agent` sera descontinuado e apagado definitivamente. Esta regra deixa de ter
escopo `internal/agent/` e passa a reger o **primitivo generico de agent da plataforma**, em
`internal/platform/agent/` (com memory/threads correlatos em `internal/platform/memory`). O kernel
`internal/platform/workflow` e a base evolutiva aproveitada da inspiracao Mastra e permanece intacto.

Mudancas mandatorias desta emenda:

1. Thread, Run, WorkingMemory e PendingStep sao PERMITIDOS em `internal/platform` como primitivos
   genericos sobre chaves opacas (`resourceId`, `threadId`). Ficam REVOGADAS todas as clausulas que
   os declaravam "exclusivos de internal/agent" ou "proibidos fora de internal/agent" (em especial
   R-AGENT-WF-001.6, addendum .6-A, addendum .8-A e o item global correspondente).
2. As invariantes ESTRUTURAIS permanecem hard e passam a reger o primitivo da plataforma:
   roteamento `Workflow -> Tool -> binding -> usecase` (.1), Tool fina sem regra/SQL/branching (.2),
   `ToolOutcome`/`RunStatus` como tipos fechados (.3), LLM so no step de parse (.4), Run auditavel
   (.5), Thread-first (.6, re-escopado), pending step antes de clarify (.7), gate HITL com
   `AwaitingApproval`/`OperationKind` fechados (.7-A), WorkingMemory no system prompt (.8).
3. Os GATES e exemplos que citam artefatos de dominio do agent removido (`daily_ledger_agent.go`,
   `pendingexpense`, `intent.Kind` de orcamento, `categoryClarification`, budget commit) ficam
   SUPERSEDED com a remocao de `internal/agent`. A techspec do `prd-platform-mastra` deve reemitir
   os gates equivalentes apontando para os arquivos genericos da plataforma. Ate la, valem como
   referencia historica do contrato comportamental a preservar, nao como caminho de arquivo literal.
4. `ThreadGateway`/`RunGateway` (ou equivalentes) passam a viver em `internal/platform/agent`; a
   proibicao de implementa-los "fora de internal/agent" fica revogada.

## Objetivo

Tornar **Workflow + Tool o padrao canonico e obrigatorio** de roteamento do primitivo de agent da
plataforma (`internal/platform/agent`), via `WorkflowRegistry`. Todo comportamento novo entra como
`Workflow`/`Tool` reutilizando bindings e usecases dos consumidores — nunca como `case` de dominio
embutido. As regras abaixo herdam e reforcam R-ADAPTER-001 e a precedencia DMMF de
`.claude/rules/governance.md`.

## R-AGENT-WF-001.1 — Roteamento `Workflow -> Tool -> binding -> usecase` [HARD]

O fluxo canonico de execucao e:

```
IntentRouter -> WorkflowRegistry.Resolve(kind) -> Workflow.Execute -> Tool.Execute -> binding -> usecase -> domain -> repo
```

Proibido:

1. Adicionar novo `case` de dominio ao `switch` de
   `internal/agent/application/.../daily_ledger_agent.go`. Cada intent kind novo DEVE ser atendido
   por um `Workflow` registrado no `WorkflowRegistry`.
2. Logica de roteamento por intent kind fora de um `Workflow` (ex: branching sobre `intent.Kind`
   em handler, consumer, job ou entrypoint para decidir qual usecase chamar).
3. Chamar binding ou usecase diretamente do entrypoint sem passar por `Workflow -> Tool`.

`daily_ledger_agent.go` deve permanecer fino: orquestra registry, guarda de escrita e formatacao
compartilhada. Resolucao de kind acontece exclusivamente via `WorkflowRegistry.Resolve(kind)`.

## R-AGENT-WF-001.2 — Tool e adapter fino de responsabilidade unica [HARD]

Cada `Tool` tem **uma unica responsabilidade** e e um adapter fino sobre `binding -> usecase`.
Herda integralmente R-ADAPTER-001.2.

Proibido em qualquer `Tool` (`internal/agent/application/tools/`) ou `Workflow`
(`internal/agent/application/workflow/`):

1. Regra ou calculo de negocio (ex: re-normalizar allocations, decidir status de dominio).
2. Query SQL direta (`QueryContext`, `ExecContext`, `db.Query`, `tx.Exec`, `db.Exec`).
3. Branching sobre estado de dominio — comparar campos de entidade para decidir comportamento.

Permitido: mapear `intent.Intent` para o DTO/command do usecase, invocar o binding, mapear o
retorno para `ToolResult` e fazer wrapping de erro (`fmt.Errorf("ctx: %w", err)`).

A lógica de pre-write (authz + replay + policy + decision audit) NAO e duplicada por tool: vive no
step de guarda reutilizavel (`write_guard.go`) aplicado pelos workflows de escrita.

## R-AGENT-WF-001.3 — `ToolOutcome` e `RunStatus` sao tipos fechados [HARD]

`ToolOutcome` e `RunStatus` DEVEM ser tipos fechados (DMMF state-as-type), nunca strings livres.

- `RunStatus` aceita apenas `running | succeeded | failed`.
- `ToolOutcome` aceita apenas o conjunto enumerado fechado (ex: `routed`, `clarify`,
  `usecaseError`, `missingResolver`).

Proibido:

- Representar outcome ou status como `string` solta em assinatura de `Tool`/`Workflow`/`Run`.
- Construir esses valores a partir de string externa sem smart constructor que rejeite valor
  invalido.

Persistencia em coluna TEXT e permitida via `String()`; a fronteira de codigo permanece tipada.

## R-AGENT-WF-001.4 — LLM apenas no step de parse [HARD]

O LLM aparece exclusivamente no step de parse a montante (`ParseInbound`). Proibido invocar LLM,
prompt rendering ou fallback chain dentro de qualquer `Workflow` ou `Tool` de execucao **de dominio**
(escrita ou leitura). Workflows e tools de dominio operam sobre `intent.Intent` ja parseado e
deterministico.

Excecoes sancionadas (unicas call-sites de LLM fora de `ParseInbound`):

1. **Conversational fallback** (`KindUnknown`): a geracao de resposta livre via fallback chain
   (`delegateFallback` -> `fallback.Reply`) e o escape-hatch conversacional; nenhuma execucao de
   dominio depende dele.
2. **Onboarding**: chain de LLM dedicado, com modelo proprio por decisao de projeto, separado do
   chain principal.

Fora dessas duas excecoes, manter a proibicao integral. Qualquer nova necessidade de LLM no meio da
execucao deve pertencer ao parse ou ser uma variante conversacional/onboarding explicita.

## R-AGENT-WF-001.5 — Toda execucao e um Run auditavel [HARD]

Toda execucao de `Workflow`/`Tool` DEVE ser observavel como um `Run` auditavel contendo, no minimo:
`thread_id`, `run_id`, `workflow`, `tool`, `status` (`RunStatus`), `duration_ms` e `error`
(quando houver). Escritas referenciam o `decision_id` correspondente do audit trail.

Cardinalidade de metricas (herda R-TXN-004): labels permitidos sao enums fechados
(`agent_id`, `channel`, `workflow`, `status`, `tool`, `outcome`). Proibido `user_id` ou
`category_id` como label de metrica.

## R-AGENT-WF-001.6 — Thread-first: toda execucao resolve Thread [HARD]

Toda chamada a `AgentRuntime.Execute` DEVE resolver um `Thread` via `ThreadGateway.GetOrCreate(resourceID, threadID)` antes de iniciar o `Run`. O par opaco `(resourceId, threadId)` e a identidade canonica do Thread, espelhando o modelo Mastra; o mapeamento de `resourceId`/`threadId` para identidades de dominio (ex.: usuario, canal) e responsabilidade do consumidor.

Proibido:
- Iniciar um `Run` sem `thread_id` valido.
- Criar logica de routing sem passar pelo `AgentRuntime` (que garante o ciclo Thread→Run).
- Implementar `ThreadGateway`/`RunGateway` em pacote de dominio; eles pertencem a `internal/platform/agent`.

Este padrao vive no **primitivo de agent da plataforma** (`internal/platform/agent`); Thread, Run e WorkingMemory sao primitivos genericos de plataforma sobre chaves opacas, nao mais exclusivos de um modulo. Modulos de dominio consomem esses primitivos sem reimplementa-los.

### Addendum R-AGENT-WF-001.6-A — Distincao kernel-mecanismo vs agent-semantico [HARD]

Adicionado em 2026-06-24 (ADR-004) para coexistir com `R-WF-KERNEL-001`.

O kernel generico em `internal/platform/workflow` oferece `Run` como **mecanismo de execucao
duravel** — um `Snapshot` + `StepRecord` com `RunStatus` fechado, identificado por `correlationKey`
opaca. Esse mecanismo e **distinto** do `Run auditavel semantico` do agent:

| Conceito | Kernel (`internal/platform/workflow`) | Primitivo de agent (`internal/platform/agent`) |
|----------|--------------------------------------|--------------------------|
| Run | mecanismo generico; `correlationKey` opaca | Run semantico; vinculado a `thread_id`/`run_id` auditavel |
| Status | `RunStatus` fechado (kernel) | `RunStatus` fechado (agent) — tipos distintos, nao compartilhados |
| Suspend/Resume | `Snapshot` duravel; retomada por `Engine.Resume` | estado de espera generico (ex.: PendingStep/Draft) no snapshot do kernel |
| Thread | ausente no kernel | `(resourceId, threadId)` opacos resolvidos via `ThreadGateway` |
| WorkingMemory | ausente no kernel | `resource`-scoped, no system prompt |

O primitivo de agent da plataforma (`internal/platform/agent`) consome `Engine[S]` do kernel para
seus workflows, passando sua estrutura de estado propria como `S`. A semantica
Thread/WorkingMemory/PendingStep vive nesse primitivo de plataforma — o kernel continua sem conhecer
esses conceitos. A distincao preservada e apenas **kernel-mecanismo vs primitivo-de-agent**: o
kernel oferece o mecanismo anonimo; Thread/Run/WorkingMemory/PendingStep deixam de ser exclusivos de
qualquer modulo e passam a ser primitivos genericos de plataforma.

## R-AGENT-WF-001.7 — Pending step obrigatorio em erro de categoria [HARD]

Quando `categoryClarification` detecta `CategoryAmbiguousError` ou `CategoryNeedsConfirmationError`, DEVE salvar `pendingexpense.Draft` com `AwaitingKind` fechado antes de retornar `OutcomeClarify`. Proibido retornar clarificacao sem salvar o estado de retomada.

Contratos:
- `AwaitingKind` aceita apenas `category_confirm | category_choice` (tipos fechados — DMMF state-as-type).
- `TransactionKind` aceita apenas `expense | income | card_purchase`.
- A retomada (resume) ocorre via `continuePendingExpenseConfirmation`, chamado **antes** de `ParseInbound`.
- O draft e limpo (`Clear`) imediatamente apos execucao ou cancelamento — nunca fica orphan.

Cobre: `KindRecordExpense`, `KindRecordIncome`, `KindRecordCardPurchase`.

### Addendum R-AGENT-WF-001.7-A — Estado de espera `AwaitingApproval` para gates HITL [HARD]

Adicionado em 2026-06-24 (ADR-003 + ADR-002) para cobrir o gate Human-in-the-Loop de operacoes
destrutivas/sensiveis.

**Escopo:** operacoes `deletar ultimo lancamento`, `editar ultimo lancamento`, `deletar cartao` e
`commitar reconfiguracao de budget` (RF-08..RF-13, prd-agent-platform-evolution).

**Estado de espera como tipo fechado (DMMF state-as-type):**

`AwaitingApproval` DEVE ser tipo fechado com constantes enumeradas (`AwaitingNone`, `AwaitingConfirm`)
e metodos `String()`/`IsValid()`/`Parse*`. Proibido representar esse estado como `string` solta em
qualquer assinatura publica do workflow de confirmacao ou de `ConfirmState`.

`OperationKind` DEVE ser tipo fechado com constantes enumeradas (`OperationDeleteLast`,
`OperationEditLast`, `OperationDeleteCard`, `OperationBudgetCommit`) e metodos equivalentes.
Proibido discriminar operacoes por `string` livre — usar mapa `map[OperationKind]...` em vez de
`switch` de dominio (R-AGENT-WF-001.1).

**Persistencia obrigatoria antes de retornar confirmacao (ADR-003):**

O passo `confirm_gate` DEVE persistir `ConfirmState` (com `Awaiting = AwaitingConfirm`) no snapshot
do kernel via `Engine.Start` ou transicao de suspend **antes** de retornar ao usuario a pergunta de
confirmacao. Proibido retornar pergunta de confirmacao sem estado duravel gravado.

O snapshot do kernel e a **fonte unica de verdade** no resume (nao side-store separado); o payload
de resume e um delta JSON merge-patch (ADR-001) — ex.: `{"ResumeText":"sim"}` — aplicado sobre o
`Snapshot.State` completo.

**Resume antes do `ParseInbound` (espelhando o padrao de categoria):**

`continuePendingApproval` DEVE ser chamado **antes** de `ParseInbound` na cadeia de resolucao de
inbound. Ordem deterministica: `continuePendingExpenseConfirmation` → `continuePendingApproval` →
`ParseInbound`. Proibido inverter a ordem ou chamar `ParseInbound` antes de tentar o resume.

**Semantica estrita + re-prompt unico + TTL (ADR-003):**

- Confirmacao explicita (`sim`/`confirmar`/`ok`/`pode`) → executa a operacao, completa o run.
- Cancelamento explicito (`nao`/`cancelar`) → descarta sem efeito, completa o run.
- Resposta ambigua 1a vez → re-pergunta uma vez (`RepromptCount` 0→1), re-suspende.
- Resposta ambigua 2a vez → cancela sem efeito, completa o run.
- TTL expirado (avaliado no resume: `now - SuspendedAt > TTL`) → cancela sem efeito, completa o
  run, devolve `handled=false` (texto do usuario segue para `ParseInbound`).
- Replay de `messageID` ja processado → `OutcomeReplay` via passo `replay` (sem segunda mutacao).

**Limpeza deterministica obrigatoria:**

Apos efetivar/cancelar/expirar, o run DEVE completar (`RunStatusSucceeded` ou `RunStatusFailed`);
nunca permanecer `RunStatusSuspended`. O housekeeping do kernel purga runs concluidos. Proibido
draft orphan ou run suspenso indefinidamente.

**Proibicoes especificas:**

- Proibido efetivar operacao destrutiva/sensivel sem confirmacao humana explicita (viola RF-08).
- Proibido retornar pergunta de confirmacao sem persistir `ConfirmState` com `AwaitingConfirm`.
- Proibido invocar LLM no `confirm_gate` ou em qualquer passo do workflow de confirmacao (R-AGENT-WF-001.4).
- Proibido crescer `case intent.Kind` no switch de `daily_ledger_agent.go` para roteamento HITL —
  usar registry por kind (R-AGENT-WF-001.1 / ADR-002).
- Proibido representar `AwaitingApproval` ou `OperationKind` como `string` livre.
- Proibido side-store separado para o estado do gate HITL — snapshot do kernel e fonte unica (ADR-001).

Gate de verificacao — sem `AwaitingApproval` como string solta (deve retornar vazio antes de merge):

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "AwaitingApproval\s*=\s*\"[^\"]*\"\|OperationKind\s*=\s*\"[^\"]*\"" \
  internal/agent/ \
  && echo "FAIL: AwaitingApproval ou OperationKind como string solta" && exit 1 \
  || true
```

Gate de verificacao — resume de aprovacao antes do parse (deve retornar `continuePendingApproval`
listado antes de `ParseInbound` na chamada inbound):

```bash
f=$(find internal/agent -name "daily_ledger_agent.go" ! -name "*_test.go")
[ -z "$f" ] && { echo "SKIP: daily_ledger_agent.go ausente"; } || {
  grep -n "continuePendingApproval\|ParseInbound" "$f" \
    | grep -v "_test" \
    | awk -F: '{print NR, $0}' \
    | grep -q "continuePendingApproval" \
    && echo "OK: continuePendingApproval presente" \
    || echo "WARN: continuePendingApproval ausente — verificar se gate HITL foi implementado"
}
```

Referencias: ADR-001 (merge-patch no kernel), ADR-002 (HITL sempre-on, registry),
ADR-003 (contrato de confirmacao), ADR-004 (gate de budget no ponto de commit) —
todos em `.specs/prd-agent-platform-evolution/`.

## R-AGENT-WF-001.8 — WorkingMemory no system prompt [HARD]

O `ContextBuilder` (ou equivalente) DEVE incluir o conteudo de `WorkingMemory` do usuario no system prompt quando disponivel. Proibido ignorar a working memory em chamadas de `ParseInbound`.

- `WorkingMemory` e escopo `resource` (por `resourceId`), compartilhada entre threads/canais.
- Formato: markdown estruturado; atualizavel via usecase dedicado.
- Ausencia de working memory (resource novo) NAO e erro — system prompt e renderizado sem ela.

### Addendum R-AGENT-WF-001.8-A — WorkingMemory e primitivo de plataforma [HARD]

Adicionado em 2026-06-24 (ADR-004); revisado em 2026-06-29 (`prd-platform-mastra`).

`WorkingMemory` e um primitivo de plataforma (`internal/platform/memory`), nao mais exclusivo de um
modulo de dominio. O kernel generico (`internal/platform/workflow`) continua SEM WorkingMemory,
system prompt ou qualquer mecanismo de contexto conversacional. Quando o primitivo de agent consome
o kernel via `Engine[S]`, o estado `S` pode conter dados derivados de WorkingMemory, mas a logica de
construcao e injecao desse contexto e responsabilidade do primitivo de agent/memory da plataforma,
nunca do kernel.

## Gate de Verificacao

**1. Switch de dominio nao cresce em `daily_ledger_agent.go`:**

```bash
f=$(find internal/agent -name "daily_ledger_agent.go" ! -name "*_test.go")
[ -z "$f" ] && { echo "SKIP: daily_ledger_agent.go ausente"; } || {
  cases=$(grep -cE "^[[:space:]]*case intent\.Kind" "$f" || true)
  [ "${cases:-0}" -gt 1 ] \
    && echo "FAIL: switch de dominio cresceu em daily_ledger_agent.go (cases=$cases); use WorkflowRegistry" && exit 1 \
    || true
}
```

**2. Zero comentarios em tools e workflows (herda R-ADAPTER-001.1):**

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "^[[:space:]]*//" \
  internal/agent/application/tools/ \
  internal/agent/application/workflow/ 2>/dev/null \
  | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL: comentarios proibidos em tools/workflow" && exit 1 \
  || true
```

**4. Pending step salvo em categoryClarification (R-AGENT-WF-001.7):**

```bash
grep -n "OutcomeClarify" internal/agent/application/services/daily_ledger_agent.go \
  | grep -v "savePendingDraft\|buildPendingDraft\|_test\|CategoryNotFound\|CategoryHintMissing" \
  && echo "WARN: revisar se OutcomeClarify retorna sem salvar Draft" || true
```

**3. Sem SQL direto em tools/workflows (herda R-ADAPTER-001.2):**

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
  internal/agent/application/tools/ \
  internal/agent/application/workflow/ 2>/dev/null \
  && echo "FAIL: SQL direto em tool/workflow" && exit 1 \
  || true
```

## Proibido (R-AGENT-WF-001 global)

- Aprovar PR que adicione `case` de dominio ao switch de `daily_ledger_agent.go`.
- Aprovar PR com regra de negocio, SQL direto ou branching de dominio em `Tool`/`Workflow`.
- Representar `ToolOutcome`/`RunStatus`/`AwaitingKind`/`TransactionKind`/`AwaitingApproval`/`OperationKind` como string livre.
- Invocar LLM fora do step de parse.
- Retornar `OutcomeClarify` em erro de categoria sem salvar `pendingexpense.Draft` (viola R-AGENT-WF-001.7).
- Efetivar operacao destrutiva/sensivel sem confirmacao humana explicita (viola Addendum R-AGENT-WF-001.7-A).
- Retornar pergunta de confirmacao HITL sem persistir `ConfirmState` com `AwaitingConfirm` (viola Addendum R-AGENT-WF-001.7-A).
- Iniciar execucao sem resolver Thread + Run via `AgentRuntime` (viola R-AGENT-WF-001.6).
- Implementar Thread, Run, WorkingMemory ou PendingStep em pacote de dominio; esses primitivos
  pertencem a `internal/platform` (agent/memory) e sao consumidos pelos modulos de dominio.
- Flexibilizar estas regras por diferenca de ferramenta, conveniencia ou deadline.

## Referencias

- `.claude/rules/go-adapters.md` (R-ADAPTER-001) — adaptadores finos e zero comentarios
- `.claude/rules/transactions-workflows.md` (R-TXN-004) — cardinalidade de metricas
- `.claude/rules/governance.md` — precedencia DMMF (state-as-type prevalece sobre Uber)
- `domain-modeling.md` em `.agents/skills/go-implementation/references/` — DMMF state-as-type
- `docs/runs/2026-06-23-evolucao-dailyagent-mastra.md` — plano da iniciativa
- ADR-001: `.specs/prd-agent-platform-evolution/adr-001-kernel-resume-merge-patch.md` — merge-patch no resume do kernel
- ADR-002: `.specs/prd-agent-platform-evolution/adr-002-hitl-always-on-kernel.md` — HITL sempre-on, registry, OperationKind
- ADR-003: `.specs/prd-agent-platform-evolution/adr-003-confirmation-contract.md` — AwaitingApproval, semântica estrita, TTL
- ADR-004: `.specs/prd-agent-platform-evolution/adr-004-budget-gate-at-commit.md` — gate de budget no ponto de commit
