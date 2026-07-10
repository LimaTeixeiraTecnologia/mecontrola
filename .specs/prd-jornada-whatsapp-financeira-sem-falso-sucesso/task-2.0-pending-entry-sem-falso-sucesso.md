# Tarefa 2.0: Pending-entry — registro determinístico sem falso sucesso

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Corrigir o workflow durável `pending-entry` do agente financeiro para que nenhuma confirmação
de despesa declare sucesso sem efeito financeiro persistido, para que a idempotência de confirmação
repetida seja por pendência/operação (replay vs. retry-controlado) e para que uma despesa simples
nunca receba o texto de múltiplos lançamentos. As decisões firmes desta tarefa estão fixadas em
ADR-001 (escrita aceita sem recurso durável vira falha tipada) e ADR-002 (idempotência por pendência
e retry controlado); esta tarefa referencia — não duplica — `techspec.md`, `adr-001-escrita-aceita-sem-recurso-duravel.md`
e `adr-002-idempotencia-por-pendencia-e-retry-controlado.md`.

<requirements>
- RF-05: despesa única inicia pendência ou registra após confirmação; nunca recebe orientação de múltiplos lançamentos.
- RF-06: nova despesa com pendência ativa recebe mensagem distinta pedindo concluir/cancelar a pendência atual (nunca o texto de múltiplos lançamentos).
- RF-07: ao confirmar, criar exatamente 1 `agents_write_ledger` + 1 transação vinculada ao WAMID original; resposta final informa sucesso do registro, não nova pergunta.
- RF-08: `platform_messages` contém a mensagem inbound e a resposta final da pendência.
- RF-09: idempotência por pendência/operação; 2º "Sim" (WAMID distinto) vira replay se persistiu ou retry controlado se falhou antes de persistir; nunca duplica transação, cancela sem comando ou confirma de novo.
- RF-10: escrita idempotente com `resourceID` vazio ⇒ passo termina `failed`/erro de negócio tipado; `platform_runs.status` nunca `succeeded` sem ledger/transação.
- RF-11: `PendingStatusCancelled` só para cancelamento explícito, expiração ou substituição; nunca para falha de escrita aceita.
- RF-12: resposta de sucesso financeiro só com efeito durável correspondente (ledger + transação).
- RF-30: retry controlado limitado a 1 tentativa adicional por confirmação repetida; TTL de pendência de 30 min; após expiração encerra como `PendingStatusExpired`, nunca falha de escrita.
- Sem novo pattern GoF (seletor `design-patterns-mandatory` = reject): apenas inversão de `StepStatus` + sentinela tipada + reuso do retry existente.
- DMMF state-as-type: `PendingStatus`/`RunStatus`/`StepStatus`/`ToolOutcome` permanecem tipos fechados; falha de escrita aceita modelada como erro tipado, `DecidePostWrite` puro.
- Chave do ledger `(wamid, item_seq, operation)` NÃO muda (ADR-002); `wamid = state.MessageID` original.
- Adapters finos e zero comentários (R-ADAPTER-001); comentários HTML guard-rail preservados neste arquivo.
</requirements>

## Subtarefas

- [ ] 2.1 Extrair função pura `DecidePostWrite(outcome agent.ToolOutcome, resourceID uuid.UUID) (PendingStatus, workflow.StepStatus, error)` e o sentinela `var ErrWriteAcceptedWithoutResource = errors.New(...)` no pacote de workflows do consumidor (ADR-001, decisão 1 e 4). No ramo `outcome != agent.ToolOutcomeReplay && resourceID == uuid.Nil`: retornar `PendingStatusActive`, `workflow.StepStatusFailed`, `ErrWriteAcceptedWithoutResource`; caso contrário `PendingStatusCompleted`, `workflow.StepStatusCompleted`, `nil`.
- [ ] 2.2 Substituir o ramo `resourceID == uuid.Nil` em `executeWithIdempotency` (~L555-559) e o ramo `ref.ID == uuid.Nil` em `executeDirectWrite` (~L586-590) por consumo de `DecidePostWrite`, retornando `StepStatusFailed` + `fmt.Errorf("...: %w", ErrWriteAcceptedWithoutResource)` e mantendo `state.Status = PendingStatusActive` (NUNCA `PendingStatusCancelled`).
- [ ] 2.3 Gravar `state.ProcessedMessageID = state.IncomingMessageID` no ramo ACCEPT (`case ConfirmActionAccept`, ~L368), antes de `executeWrite`, espelhando `card_create_confirm_workflow.go` L151 (hoje só o reprompt grava — ADR-002 decisão 3). NÃO alterar a chave do ledger `(wamid, item_seq, operation)`; idempotência por pendência vem de `wamid = state.MessageID` (original). Replay via `FindByKey` (usecase), retry via `tryResumeFailedWrite`.
- [ ] 2.4 Alterar `const maxFailedWriteResumes = 2` → `= 1` (~L409), aplicando D-10: 1 tentativa adicional (total até 2 escritas). O contador em `IsResumableAfterFailedWrite`/`SeedResumeAfterFailedWrite` bloqueia a partir da 1ª tentativa adicional (ADR-002 decisão 4).
- [ ] 2.5 TTL 30 min com transição EXPLÍCITA `Active → PendingStatusExpired` no resume quando `isExpired(state, now)`, com `ResponseText` determinístico; garantir que o run complete (`StepStatusCompleted`), sem permanecer suspenso; reaper `PendingEntryStaleAfter = 35min` permanece como rede de segurança (ADR-002 decisão 5).
- [ ] 2.6 RF-06: declarar nova constante `ActivePendingEntryMessage` em `pending_entry_workflow.go` (~L61) e trocar em `register_attempt.go` L106 e L147 (ramos `errors.Is(err, wf.ErrRunAlreadyExists)`) de `MultiItemOrientationMessage` para `ActivePendingEntryMessage`. Garantir que `MultiItemOrientationMessage` só apareça em `guards/multi_item.go` (grep em `usecases/` deve retornar vazio). Avaliar o mesmo ramo `ErrRunAlreadyExists` em `CreateRecurrence` e aplicar a mesma correção se presente.
- [ ] 2.7 Merge-patch no resume preservado (R-WF-KERNEL-001.7): payload `{"resumeText":...,"incomingMessageId":...}` aplicado como delta sobre `snap.State`; nenhuma substituição de estado inteiro; nenhum tipo de domínio no kernel.

## Detalhes de Implementação

Ver `techspec.md` (seção de idempotência da pendência e falha de escrita), `adr-001-escrita-aceita-sem-recurso-duravel.md`
(Plano de Implementação passos 1-5; `DecidePostWrite`, sentinela, correção simétrica dos dois caminhos) e
`adr-002-idempotencia-por-pendencia-e-retry-controlado.md` (Plano de Implementação passos 1-5; `ProcessedMessageID`
no ACCEPT, `maxFailedWriteResumes=1`, expiração explícita, chave do ledger imutável, merge-patch preservado).

Invariantes-chave a preservar sem duplicar aqui:
- O engine mapeia `StepStatusFailed → RunStatusFailed` e `StepStatusCompleted → RunStatusSucceeded`
  (`internal/platform/workflow/engine.go`); por isso o ramo sem recurso deve retornar `StepStatusFailed`.
- `tryResumeFailedWrite` só dispara em `RunStatusFailed` com `state.Status == PendingStatusActive` preservado
  (dependência direta de ADR-001 para ADR-002).
- Não introduzir novo valor no enum `ToolOutcome` da plataforma (alternativa (a) rejeitada em ADR-001) —
  a semântica é local ao consumidor via sentinela + `errors.Is`.

## Critérios de Sucesso

- Escrita aceita vazia (sem replay, `resourceID == uuid.Nil`) ⇒ run termina `failed` +
  `errors.Is(err, ErrWriteAcceptedWithoutResource)` verdadeiro + `state.Status == PendingStatusActive`
  (nunca `PendingStatusCancelled`), em ambos os caminhos (`executeWithIdempotency` e `executeDirectWrite`).
- Dois "Sim" com WAMIDs distintos na mesma pendência ⇒ replay (`ToolOutcomeReplay`, reusa `resourceID`) se a 1ª
  persistiu; retry-controlado de exatamente 1 tentativa adicional se a 1ª falhou antes de persistir; NUNCA
  duplica transação — exatamente 1 linha em `agents_write_ledger`.
- Despesa simples nunca recebe `MultiItemOrientationMessage`; pendência ativa ⇒ `ActivePendingEntryMessage`.
- Grep de governança limpo: `MultiItemOrientationMessage` ausente em `internal/agents/application/usecases/`.
- TTL: resume após 30 min ⇒ `PendingStatusExpired` com mensagem visível; run completa, não fica suspenso.
- `platform_messages` contém a mensagem inbound e a resposta final da pendência (RF-08).
- `go build`, `go vet`, `go test -race -count=1` e `golangci-lint run` verdes no escopo alterado; gates de
  governança (R-WF-KERNEL-001, R-AGENT-WF-001, R-ADAPTER-001.1) retornam limpos.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — workflow durável pending-entry, PendingStep, `ToolOutcome`/`RunStatus` fechados, escrita idempotente do substrato.
- `domain-modeling-production` — erro de negócio tipado, state-as-type, `DecidePostWrite` puro.
- `design-patterns-mandatory` — gate: sem novo GoF pattern (inversão de `StepStatus` + sentinela + reuso do retry existente).

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

Unit (puro, sem mock):
- `DecidePostWrite`: tabela cobrindo accept (recurso válido ⇒ `Completed`/`StepStatusCompleted`/nil),
  replay (`ToolOutcomeReplay` + `uuid.Nil` ⇒ `Completed`/`StepStatusCompleted`/nil),
  sem recurso (`!replay` + `uuid.Nil` ⇒ `Active`/`StepStatusFailed`/`ErrWriteAcceptedWithoutResource`).
- `DecideConfirmation`: accept, replay (por `ProcessedMessageID`), retry-1, retry-esgotado
  (`FailedWriteResumeCount` no limite), expiração TTL (`isExpired`), dois "Sim" distintos.

Integração (`write_ledger` / Postgres):
- `Insert` com `ON CONFLICT (wamid, item_seq, operation) DO NOTHING` + `FindByKey` ⇒ 2º "Sim"
  (WAMID distinto, mesma pendência) resolve replay reusando o `resourceID` existente; assertar
  exatamente 1 linha no `agents_write_ledger` e 1 `resourceID` retornado.

E2E real-LLM (`RUN_REAL_LLM=1`):
- Jornada de confirmação nunca declara sucesso sem linha correspondente no ledger; RF-27 `>= 0,90`
  por categoria (validação delegada à Tarefa 8.0 no golden set).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/agents/application/workflows/pending_entry_workflow.go`
- `internal/agents/application/workflows/pending_entry_decisions.go`
- `internal/agents/application/workflows/pending_entry_state.go`
- `internal/agents/application/usecases/register_attempt.go`
- `internal/agents/application/usecases/idempotent_write.go`
- `internal/agents/infrastructure/persistence/write_ledger_repository.go`
- Referência de padrão de dedupe: `internal/agents/application/workflows/card_create_confirm_workflow.go` (L151)
- ADRs: `adr-001-escrita-aceita-sem-recurso-duravel.md`, `adr-002-idempotencia-por-pendencia-e-retry-controlado.md`; `techspec.md`
