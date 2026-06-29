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

O fluxo canonico de inbound (substrato `internal/platform/agent`) e:

```
InboundRequest -> AgentRuntime.Execute -> ThreadGateway.GetOrCreate -> RunStore.Insert
  -> AgentRegistry.Resolve(agentId) -> Agent.Execute (loop tool-calling) -> tool.ToolHandle.Invoke -> exec
  -> MessageStore.Append -> closeRun
```

Execucao duravel multi-step usa o kernel: `workflow.Engine[S].Start/Resume` sobre `Step[S]`
(`Sequence`/`Branch`/`Parallel`/`Retry`), com resume por merge-patch.

Proibido:

1. Roteamento por `switch case intent.Kind` (em agente, handler, consumer, job ou entrypoint) para
   decidir qual usecase chamar. Resolucao acontece via `AgentRegistry.Resolve(agentId)` /
   `WorkflowRegistry.Resolve` — comportamento novo entra como novo agente/tool/workflow no consumidor.
2. Chamar binding ou usecase diretamente do entrypoint sem passar pelo `AgentRuntime` (loop tool-calling)
   ou por um `Workflow`/`Tool`.

O agente do consumidor permanece fino: monta tools e instructions e delega ao runtime/registry; nao
contem branching de dominio por kind.

## R-AGENT-WF-001.2 — Tool e adapter fino de responsabilidade unica [HARD]

Cada `Tool` (`tool.NewTool[I,O]`) tem **uma unica responsabilidade** e e um adapter fino: o `exec`
delega a um client/usecase. Herda integralmente R-ADAPTER-001.2.

Proibido em qualquer `Tool` do consumidor (`internal/agents/application/tools/`) ou `Workflow`
(`internal/agents/application/workflows/`):

1. Regra ou calculo de negocio (ex: re-normalizar valores, decidir status de dominio).
2. Query SQL direta (`QueryContext`, `ExecContext`, `db.Query`, `tx.Exec`, `db.Exec`).
3. Branching sobre estado de dominio — comparar campos de entidade para decidir comportamento.

Permitido: validar o input contra o schema, mapear para o DTO/command do usecase, invocar o
client/binding, mapear o retorno para o output tipado e fazer wrapping de erro (`fmt.Errorf("ctx: %w", err)`).
Calculo puro de dominio vive em `domain/`.

Quando houver pre-write (authz + replay + policy + audit), a logica NAO e duplicada por tool: vive em
um `Step[S]` de guarda reutilizavel aplicado pelos workflows de escrita.

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

## R-AGENT-WF-001.4 — LLM apenas nas call-sites sancionadas [HARD]

O LLM aparece exclusivamente nas call-sites sancionadas do substrato:

1. **Loop de tool-calling do Agent** (`agent.Agent.Execute` -> `llm.Provider.Complete`): o ciclo
   completa-com-tools resolve a resposta e as invocacoes de ferramenta.
2. **Step de workflow que chama `Agent.Stream`** (ex.: um passo que gera texto livre a partir do estado).
3. **Scorer LLM-judged** (`scorer.NewLLMJudgedScorer` -> `llm.Provider.Complete`), fora do caminho principal.

OpenRouter e o unico provider (`llm.NewOpenRouterProvider`); nao existe fallback chain nem circuit breaker.
Proibido invocar LLM, prompt rendering ou client de modelo no **kernel** (`internal/platform/workflow`)
ou dentro de uma `Tool` de dominio (o `exec` da tool e deterministico e delega a client/usecase).
Qualquer nova necessidade de LLM deve pertencer a uma dessas tres call-sites sancionadas.

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

## R-AGENT-WF-001.7 — Pending step obrigatorio antes de pedir clarificacao [HARD]

Invariante generica (substrato): quando um passo precisa de input adicional do usuario para prosseguir,
DEVE persistir o estado de espera **antes** de retornar a pergunta de clarificacao/confirmacao;
proibido pedir clarificacao sem salvar o estado de retomada.

Contratos:
- O estado de espera e um tipo fechado (DMMF state-as-type), nunca flag booleano nem string solta.
- E persistido no `Snapshot` do kernel (`workflow.Store`), fonte unica de verdade.
- A retomada (resume) aplica merge-patch sobre o `Snapshot.State` e ocorre **antes** de qualquer parse.
- O estado e limpo imediatamente apos execucao ou cancelamento — nunca fica orphan.

### Addendum R-AGENT-WF-001.7-A — Estado de espera `AwaitingApproval` para gates HITL [HARD]

> SUPERSEDED como caminho literal (Reemissao 2026-06-29, `prd-platform-mastra`): este addendum foi
> escrito para o gate HITL do agent financeiro removido (`internal/agent`); as operacoes, funcoes
> (`continuePendingApproval`, `continuePendingExpenseConfirmation`, `ParseInbound`) e tipos
> (`AwaitingApproval`, `OperationKind`, `ConfirmState`) citados abaixo pertenciam a esse modulo e
> NAO existem mais. O **contrato comportamental** permanece valido como referencia (estado de espera
> como tipo fechado, persistido no `Snapshot` antes de pedir confirmacao, retomado por merge-patch
> antes de qualquer parse, limpeza deterministica) e deve ser reemitido apontando para os arquivos
> reais quando um consumidor reintroduzir HITL. Ate la, leia o que segue como contrato, nao como
> caminho de codigo.

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

> Reemissao 2026-06-29: com `internal/agent` apagado, os dois gates HITL abaixo ficam SUPERSEDED como
> caminho literal. O contrato (estado de espera fechado, persistido antes da confirmacao, resumido por
> merge-patch antes do parse) permanece hard e deve ser reemitido pelo consumidor que reintroduzir HITL.

Gate de verificacao — sem estado de espera HITL como string solta (deve retornar vazio antes de merge):

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "AwaitingApproval\s*=\s*\"[^\"]*\"\|OperationKind\s*=\s*\"[^\"]*\"" \
  internal/platform/agent/ internal/agents/ 2>/dev/null \
  && echo "FAIL: AwaitingApproval ou OperationKind como string solta" && exit 1 \
  || true
```

Gate de verificacao — resume da aprovacao antes do parse: SUPERSEDED como caminho literal
(`daily_ledger_agent.go`/`continuePendingApproval` pertenciam ao agent removido). Reemitir quando um
consumidor reintroduzir o gate HITL, validando que o resume do estado de espera ocorre antes do parse
do inbound nos arquivos reais desse consumidor.

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

> Reemissao 2026-06-29 (`prd-platform-mastra`): `internal/agent` foi apagado. Os gates abaixo apontam
> para o primitivo de agent da plataforma (`internal/platform/agent`) e para os consumidores
> (`internal/agents`). Os gates de semantica de dominio do agent removido (switch de
> `daily_ledger_agent.go`, `pendingexpense.Draft` em `categoryClarification`) ficam SUPERSEDED como
> caminho literal — valem como contrato comportamental historico (roteamento por registry, salvar
> estado de espera antes de pedir confirmacao) a ser reemitido pela techspec do consumidor que
> reintroduzir HITL/clarificacao.

**1. Roteamento por registry (sem switch de dominio) — primitivo de agent:**

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "^[[:space:]]*case intent\.Kind" \
  internal/platform/agent/ internal/agents/ 2>/dev/null \
  && echo "FAIL: switch de dominio por intent.Kind; use AgentRegistry/WorkflowRegistry" && exit 1 \
  || true
```

**2. Zero comentarios em tools e workflows do consumidor (herda R-ADAPTER-001.1):**

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "^[[:space:]]*//" \
  internal/platform/agent/ \
  internal/agents/application/tools/ \
  internal/agents/application/workflows/ 2>/dev/null \
  | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL: comentarios proibidos em tools/workflow" && exit 1 \
  || true
```

**3. Sem SQL direto em tools/consumers (herda R-ADAPTER-001.2):**

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
  internal/agents/application/tools/ \
  internal/agents/infrastructure/messaging/database/consumers/ 2>/dev/null \
  && echo "FAIL: SQL direto em tool/consumer" && exit 1 \
  || true
```

**4. Estado de espera salvo antes de pedir confirmacao (SUPERSEDED como caminho literal):**

O contrato comportamental — salvar o estado de espera (pending/draft) no snapshot do kernel antes de
retornar a pergunta de confirmacao, e retomar via merge-patch antes de qualquer parse — permanece hard.
O gate executavel sobre `daily_ledger_agent.go`/`categoryClarification` fica suspenso ate o consumidor
que reintroduzir HITL reemitir o equivalente apontando para seus arquivos reais.

## Proibido (R-AGENT-WF-001 global)

- Aprovar PR que roteie por `switch case intent.Kind` em vez de `AgentRegistry`/`WorkflowRegistry`.
- Aprovar PR com regra de negocio, SQL direto ou branching de dominio em `Tool`/`Workflow`.
- Representar estados de fronteira (`agent.ToolOutcome`/`agent.RunStatus`/`agent.AwaitingKind`, `workflow.RunStatus`/`StepStatus`/`SuspendReason`, `scorer.ScorerKind`, `memory.MessageRole`) como string livre.
- Invocar LLM fora das call-sites sancionadas (loop do Agent, step que chama `Stream`, scorer LLM-judged) ou dentro do kernel.
- Pedir clarificacao/confirmacao sem antes persistir o estado de espera (tipo fechado) no `Snapshot` (viola R-AGENT-WF-001.7).
- Efetivar operacao destrutiva/sensivel sem confirmacao humana explicita (viola Addendum R-AGENT-WF-001.7-A).
- Retornar pergunta de confirmacao HITL sem antes persistir o estado de espera (tipo fechado) no `Snapshot` (contrato do Addendum R-AGENT-WF-001.7-A).
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
