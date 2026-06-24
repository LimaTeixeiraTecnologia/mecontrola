# Agent Workflows e Tools — Padrao Canonico

- Rule ID: R-AGENT-WF-001
- Severidade: hard
- Escopo: `internal/agent/`
- Plano de origem: `docs/runs/2026-06-23-evolucao-dailyagent-mastra.md`

## Objetivo

Tornar **Workflow + Tool o padrao canonico e obrigatorio** de roteamento do `internal/agent`,
substituindo o `switch` de `daily_ledger_agent.go` por um `WorkflowRegistry`. Todo comportamento
novo entra como `Workflow`/`Tool` reutilizando bindings e usecases existentes — nunca como novo
`case` de dominio. As regras abaixo herdam e reforcam R-ADAPTER-001 e a precedencia DMMF de
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

Toda chamada a `AgentRuntime.Execute` DEVE resolver um `Thread` via `ThreadGateway.GetOrCreate(userID, channel)` antes de iniciar o `Run`. O par `(user_id, channel)` e a identidade canonica do Thread, espelhando o modelo Mastra: `resourceId = user_id`, `threadId = channel`.

Proibido:
- Iniciar um `Run` sem `thread_id` valido.
- Criar logica de routing sem passar pelo `AgentRuntime` (que garante o ciclo Thread→Run).
- Implementar `ThreadGateway` ou `RunGateway` fora de `internal/agent`.

Este padrao e **exclusivo de `internal/agent`**; outros modulos NAO devem ter Thread, Run ou WorkingMemory proprios.

### Addendum R-AGENT-WF-001.6-A — Distincao kernel-mecanismo vs agent-semantico [HARD]

Adicionado em 2026-06-24 (ADR-004) para coexistir com `R-WF-KERNEL-001`.

O kernel generico em `internal/platform/workflow` oferece `Run` como **mecanismo de execucao
duravel** — um `Snapshot` + `StepRecord` com `RunStatus` fechado, identificado por `correlationKey`
opaca. Esse mecanismo e **distinto** do `Run auditavel semantico` do agent:

| Conceito | Kernel (`internal/platform/workflow`) | Agent (`internal/agent`) |
|----------|--------------------------------------|--------------------------|
| Run | mecanismo generico; `correlationKey` opaca | Run semantico; vinculado a `thread_id`/`run_id` auditavel |
| Status | `RunStatus` fechado (kernel) | `RunStatus` fechado (agent) — tipos distintos, nao compartilhados |
| Suspend/Resume | `Snapshot` duravel; retomada por `Engine.Resume` | `pendingexpense.Draft` como estado do run suspenso do kernel |
| Thread | ausente no kernel | `(user_id, channel)` resolvido via `ThreadGateway` |
| WorkingMemory | ausente no kernel | `resource`-scoped, no system prompt |

O `internal/agent` PODE consumir `Engine[S]` do kernel para seus workflows de escrita, passando
sua estrutura de estado propria como `S`. A semantica Thread/WorkingMemory/PendingStep permanece
exclusiva do agent — o kernel nao conhece esses conceitos. Essa distincao nao reabre brecha: a
proibicao de Thread, Run semantico e WorkingMemory **fora de `internal/agent`** se mantem; o
kernel oferece apenas o mecanismo anonimo.

## R-AGENT-WF-001.7 — Pending step obrigatorio em erro de categoria [HARD]

Quando `categoryClarification` detecta `CategoryAmbiguousError` ou `CategoryNeedsConfirmationError`, DEVE salvar `pendingexpense.Draft` com `AwaitingKind` fechado antes de retornar `OutcomeClarify`. Proibido retornar clarificacao sem salvar o estado de retomada.

Contratos:
- `AwaitingKind` aceita apenas `category_confirm | category_choice` (tipos fechados — DMMF state-as-type).
- `TransactionKind` aceita apenas `expense | income | card_purchase`.
- A retomada (resume) ocorre via `continuePendingExpenseConfirmation`, chamado **antes** de `ParseInbound`.
- O draft e limpo (`Clear`) imediatamente apos execucao ou cancelamento — nunca fica orphan.

Cobre: `KindRecordExpense`, `KindRecordIncome`, `KindRecordCardPurchase`.

## R-AGENT-WF-001.8 — WorkingMemory no system prompt [HARD]

O `ContextBuilder` (ou equivalente) DEVE incluir o conteudo de `WorkingMemory` do usuario no system prompt quando disponivel. Proibido ignorar a working memory em chamadas de `ParseInbound`.

- `WorkingMemory` e escopo `resource` (por `user_id`), compartilhada entre canais.
- Formato: markdown estruturado; atualizavel via usecase dedicado.
- Ausencia de working memory (usuario novo) NAO e erro — system prompt e renderizado sem ela.

### Addendum R-AGENT-WF-001.8-A — WorkingMemory e exclusiva do agent [HARD]

Adicionado em 2026-06-24 (ADR-004) para coexistir com `R-WF-KERNEL-001`.

`WorkingMemory` e um conceito semantico exclusivo de `internal/agent`. O kernel generico
(`internal/platform/workflow`) NAO tem WorkingMemory, system prompt ou qualquer mecanismo de
contexto conversacional. Quando o agent consome o kernel via `Engine[S]`, o estado `S` passado
ao kernel pode conter dados derivados de WorkingMemory, mas a logica de construcao e injecao
desse contexto e responsabilidade exclusiva do agent, nunca do kernel.

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
- Representar `ToolOutcome`/`RunStatus`/`AwaitingKind`/`TransactionKind` como string livre.
- Invocar LLM fora do step de parse.
- Retornar `OutcomeClarify` em erro de categoria sem salvar `pendingexpense.Draft` (viola R-AGENT-WF-001.7).
- Iniciar execucao sem resolver Thread + Run via `AgentRuntime` (viola R-AGENT-WF-001.6).
- Implementar Thread, Run, WorkingMemory ou PendingStep em modulo diferente de `internal/agent`.
- Flexibilizar estas regras por diferenca de ferramenta, conveniencia ou deadline.

## Referencias

- `.claude/rules/go-adapters.md` (R-ADAPTER-001) — adaptadores finos e zero comentarios
- `.claude/rules/transactions-workflows.md` (R-TXN-004) — cardinalidade de metricas
- `.claude/rules/governance.md` — precedencia DMMF (state-as-type prevalece sobre Uber)
- `domain-modeling.md` em `.agents/skills/go-implementation/references/` — DMMF state-as-type
- `docs/runs/2026-06-23-evolucao-dailyagent-mastra.md` — plano da iniciativa
