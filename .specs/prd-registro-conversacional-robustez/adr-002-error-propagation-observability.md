# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Propagação de erro de escrita para Run, span pesquisável e log (fim do swallow)
- **Data:** 2026-07-08
- **Status:** Aceita
- **Decisores:** Time de plataforma MeControla
- **Relacionados:** PRD (RF-10..RF-13), techspec.md, `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001.5), `.claude/rules/workflow-kernel.md`

## Contexto

Causa-raiz provada do incidente: o erro de escrita é **engolido**. No passo do pending workflow,
`executeWithIdempotency` (`pending_entry_workflow.go:444-461`) e `executeDirectWrite` (`:463-473`)
recebem `idemErr`/`writeErr`, mas **descartam o erro**, setam `state.Status = PendingStatusCancelled`
com `ResponseText` amigável e retornam `StepStatusCompleted` com `err=nil`. `validateCategoryForWrite`
(`:413-442`) faz o mesmo ao falhar `ResolveForWrite`.

Consequência em cadeia:
- `engine.runStep` (`engine.go:431-454`) só chama `span.RecordError` e marca `StepStatusFailed`
  quando `err != nil`; recebendo `err=nil`/`Completed`, o span do step encerra como sucesso.
- `engine.execute` (`:263-354`) só marca `RunStatusFailed` em `stepErr != nil` ou
  `out.Status == StepStatusFailed`; logo grava `snap.Status = RunStatusSucceeded` e `LastError=""`.
- `runtime.finishRun` (`runtime.go:155-211`) chama `closeRun(..., errStr="")` → `closeRun` (`:259`)
  grava `run.Error = ""` em `platform_runs`.

Resultado: métrica `agents_write_total{outcome="usecase_error"}` incrementa (o counter está em
`idempotent_write.go:87-90` e já há `span.RecordError` no usecase em `:86`), mas o Run fica com
`error` em branco e não há trace de erro pesquisável do run do agente no worker. Além disso, a falha
upstream do `register_income` nunca chega ao passo de escrita: a tool retorna `ToolOutcomeClarify` e
delega ao `engine.Start`; a escrita ocorre no resume ("sim"), então o erro (quando existe) só pode
ser capturado no passo do workflow.

**Fato arquitetural decisivo (verificado):** existem duas tabelas de run distintas —
`mecontrola.platform_runs` (Run auditável do agente, coluna `error`, status
`running|succeeded|failed`, com `thread_id`/`resource_id` nativos, `migrations/000001:2367-2387`) e
`mecontrola.workflow_runs` (snapshot do kernel, coluna `last_error`, `migrations/000001:1156-1183`;
persistida em `workflow/infrastructure/postgres/store.go:178`). O turno de confirmação ("sim") é
processado por `pending_entry_continuer.go:79` chamando `engine.Resume` **direto, sem passar pelo
`AgentRuntime`** — portanto hoje **não cria linha em `platform_runs`**. Como a escrita só ocorre no
resume durável, o erro de escrita cai em `workflow_runs.last_error`, e a confirmação **não é um Run
auditável** — um gap adicional frente a R-AGENT-WF-001.5 ("toda execução é um Run auditável").

## Decisão

Estabelecer um **contrato de propagação de erro** de duas partes, sem violar R-ADAPTER-001 (adapters
finos) nem R-WF-KERNEL-001 (kernel genérico):

**Parte A — o passo do workflow para de engolir erro.** Em `executeWithIdempotency`,
`executeDirectWrite` e `validateCategoryForWrite`, quando a falha for real (não replay, não
cancelamento de negócio), retornar `(StepOutput{Status: StepStatusFailed, State: state}, err)` com o
erro real (`fmt.Errorf("ctx: %w", err)`), preservando `state.ResponseText` amigável para o usuário.
Distinguir três desfechos:
- **falha de escrita/infra** (idemErr/writeErr) → `StepStatusFailed` + erro propagado.
- **cancelamento de negócio** (usuário disse "não", categoria incompatível sem candidato) →
  permanece `StepStatusCompleted` com `PendingStatusCancelled` (não é erro).
- **replay/reconciled** → `StepStatusCompleted` sucesso.

Com isso o kernel genérico já faz o resto sem mudança: `engine.runStep` chama `span.RecordError(err)`
e marca o step failed; `engine.execute` grava `snap.Status = RunStatusFailed` e
`snap.LastError = err.Error()`. Nenhuma regra de domínio entra no kernel.

**Parte B — o turno de confirmação/resume passa a ser um Run auditável em `platform_runs`.**
O caminho de resume (`pending_entry_continuer.go` / `register_attempt`) deve **abrir e fechar um Run
auditável do agente** (`platform_runs`, `RunStatus` fechado) ao redor de `engine.Resume`, resolvendo
`Thread(resourceId, threadId)` e associando `wamid`. Ao receber `RunResult{Status: RunStatusFailed}`
(ou erro retornado pelo kernel — que já gravou `workflow_runs.last_error`), propagar o motivo real
para `closeRun(..., errStr=<motivo real>)`, preenchendo `platform_runs.error` — satisfazendo o
critério de aceite literal ("erro gravado em `platform_runs.error`") e tornando toda confirmação
auditável (fecha o gap de R-AGENT-WF-001.5). O kernel permanece gravando `workflow_runs.last_error`
(sem duplicação semântica: `last_error` é o mecanismo genérico; `platform_runs.error` é o Run
semântico do agente). O turno de tool-calling inicial (onde `register_income`/runtime pode falhar
antes do suspend) já passa pelo `AgentRuntime`/`finishRun` → `closeRun`, e também deve carregar o
motivo real em `platform_runs.error`.

Em ambos os turnos, o consumidor emite um span de erro **pesquisável** no worker (nome estável, ex.:
`agents.pending_entry.resume` / `agents.usecase.register_attempt.*`) com `RecordError` + status error
e atributos `thread_id`, `run_id`, `wamid`. A identidade de domínio (`thread_id`/`run_id`/`wamid`)
vive no span/log do **consumidor**, nunca como label de métrica de alta cardinalidade e nunca no
kernel genérico (o span `workflow.step.execute` do kernel carrega só `workflow`/`step`). Log nível
ERROR com `thread_id`, `run_id`, `wamid` (R-TXN-004: cardinalidade controlada nas métricas).

A mensagem ao usuário permanece amigável ("Não consegui registrar. Tente novamente em breve."); o
detalhe técnico vai apenas para `platform_runs.error`/`workflow_runs.last_error`, span e log.

## Alternativas Consideradas

1. **Só preencher a métrica (já existe) e deixar run.error vazio** — Descartada: o gap não é métrica,
   é `platform_runs.error` vazio + ausência de trace pesquisável; não resolve o diagnóstico.
2. **Branch de domínio no kernel para inspecionar `PendingStatusCancelled`** — Descartada: viola
   R-WF-KERNEL-001.2 (kernel não pode ter branching de domínio). A distinção
   falha-vs-cancelamento vive no passo do consumidor, que decide o `StepStatus`.
3. **Gravar erro só no log** — Descartada: sem `platform_runs.error` e span, o operador continua sem
   rastro atribuível (a lição central do incidente).
4. **Aceitar erro de escrita só em `workflow_runs.last_error` (duas tabelas, sem Run no resume)** —
   Descartada: relaxaria o critério de aceite literal ("`platform_runs.error`") e manteria a
   confirmação como execução não auditável, contrariando R-AGENT-WF-001.5.
5. **Espelhar `workflow_runs.last_error` em `platform_runs` sem tornar o resume um Run real** —
   Descartada: duplica estado de erro em duas tabelas com risco de divergência e não resolve a
   ausência de auditoria da confirmação.

## Consequências

### Benefícios Esperados

- Zero falha silenciosa: toda escrita falha tem `platform_runs.error`, span de erro pesquisável e log
  ERROR correlacionados (`thread_id`/`run_id`/`wamid`) — a métrica-alvo primária do PRD.
- Diagnóstico futuro por Tempo/Loki sem depender de tabelas vazias.

### Trade-offs e Custos

- Runs que hoje terminavam "succeeded" com cancelamento por erro passarão a `failed` — muda a
  distribuição de `agent_runs_total{status}`. É o comportamento correto; atualizar dashboards.
- Necessário cuidado para não classificar cancelamento de negócio ("não") como `failed`.

### Riscos e Mitigações

- **Risco:** retornar `StepStatusFailed` em workflow durável dispara o retry do kernel
  (`MaxAttempts`) de forma indesejada. **Mitigação:** o retry transitório é governado pela ADR-003
  (política explícita e classificação de transitório); falha permanente marca failed sem re-loop
  infinito (`MaxAttempts` finito) e o pending permanece retomável.
- **Risco:** vazar detalhe técnico ao usuário. **Mitigação:** `ResponseText` amigável é preservado;
  erro real só em telemetria.

## Plano de Implementação

1. Refatorar os três pontos de swallow (`executeWithIdempotency`, `executeDirectWrite`,
   `validateCategoryForWrite`) para retornar `StepStatusFailed` + erro nas falhas reais (Parte A).
2. Fazer o caminho de resume (`pending_entry_continuer`/`register_attempt`) abrir/fechar um Run
   auditável em `platform_runs` ao redor de `engine.Resume`, resolvendo Thread e `wamid`, e propagar
   o motivo real para `closeRun(errStr=...)` → `platform_runs.error` (Parte B).
3. No turno de tool-calling, garantir que `finishRun`/`closeRun` também carreguem o motivo real
   quando a tool (`register_income`/runtime) falhar antes do suspend.
4. Emitir span de erro pesquisável no consumidor (`RecordError` + status error + `thread_id`/
   `run_id`/`wamid`) em ambos os turnos; log ERROR correlacionado.
5. Testes de integração: falha de escrita no resume ⇒ `platform_runs.error` preenchido +
   `workflow_runs.last_error` + span error + log ERROR; falha upstream da tool ⇒ `platform_runs.error`;
   cancelamento "não" ⇒ run não-failed; sucesso ⇒ run succeeded.

## Monitoramento e Validação

- Critério de aceite: "Falha de escrita grava erro no Run, span e log" e "Run de escrita falha produz
  trace de erro pesquisável".
- Métrica `agents_write_total{outcome}` mantida; `agent_runs_total{status="failed"}` passa a ter
  causa rastreável no Run.

## Impacto em Documentação e Operação

- Runbook de agents: como localizar o span/`platform_runs.error` de um registro falho.
- Dashboards: revisar painéis que assumiam "succeeded" para cancelamentos por erro.

## Revisão Futura

- Revisitar quando a techspec do `prd-platform-mastra` reemitir os gates de HITL (hoje SUPERSEDED
  como caminho literal), para manter o contrato de erro alinhado ao consumidor.
