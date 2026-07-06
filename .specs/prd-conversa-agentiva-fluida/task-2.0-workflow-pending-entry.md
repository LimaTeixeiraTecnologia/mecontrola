# Tarefa 2.0: Workflow `pending-entry`: Start/Resume/Confirm/Cancel/Expire/Replaced + Reaper

<critical>Ler prd.md, techspec.md e scenarios.md desta pasta — tarefa invalidada se pulado</critical>

## Visão Geral

Implementar o workflow durável `pending-entry` usando `workflow.Engine[PendingEntryState]` no consumidor `internal/agents/application/workflows/`. O workflow persiste estado no snapshot do kernel via `workflow_runs`, aplica merge-patch no resume (R-WF-KERNEL-001.7) e expõe start/resume sem chamadas reais a ledger ou LLM. O estado terminal antes de QUALQUER escrita financeira é `AwaitingSlotConfirmation` — mesmo para lançamentos totalmente especificados e não ambíguos (RF-38, RF-40); a confirmação reusa o contrato SEMÂNTICO do destructive-confirm (sim/não, reprompt único, TTL) sem reusar aquele workflow. Inclui o reaper de pendências suspensas expiradas como subtarefa e a exposição de `InboundExecutionFromContext` (threadID) em `internal/platform/agent/identity_context.go`.

<requirements>
- WorkflowID constante: "pending-entry"
- Engine tipada: workflow.Engine[PendingEntryState]
- Start: abre pendência, persiste snapshot com status=Active e AwaitingSlot inicial; lançamento totalmente especificado abre direto em AwaitingSlotConfirmation (RF-38, RF-40)
- Resume: aplica JSON merge-patch sobre Snapshot.State; valida expiração (SuspendedAt + 30min); delega decisão a DecidePendingResume (pura, de 1.0)
- Confirmação universal: AwaitingSlotConfirmation é o slot terminal antes de todo write; delega a DecideConfirmation (pura, de 1.0); reusa contrato semântico do destructive-confirm, nunca o workflow destructive-confirm
- Cancel: fecha status=Cancelled; zero escrita posterior possível
- Expire: fecha status=Expired quando TTL vencido; resposta ao usuário via ResponseText
- Replaced: fecha status=Replaced; retorna handled=false para que consumer processe nova frase
- Reaper: workflow.NewStaleSuspendedReaper para "pending-entry" com staleAfter=35*time.Minute (registrado em module.go em 6.0)
- Zero SQL fora do adapter postgres do kernel (R-WF-KERNEL-001.2)
- Zero comentários Go de produção (R-ADAPTER-001.1)
- Zero import de pacote de domínio no kernel (R-WF-KERNEL-001.1)
</requirements>

## Subtarefas

- [ ] 2.1 Criar `pending_entry_workflow.go` com constante `PendingEntryWorkflowID = "pending-entry"` e função de definição do workflow
- [ ] 2.2 Implementar step `start_pending`: recebe `PendingEntryState` inicial, persiste via `Engine.Start`, suspende com `AwaitingSlot` correto
- [ ] 2.3 Implementar step `resume_pending`: aplica merge-patch `{"resumeText":"...", "messageId":"..."}` sobre snapshot; chama `DecidePendingResume`; executa a decisão (cancel/expire/replace/slot preenchido/reprompt)
- [ ] 2.4 Implementar transições de status: `Active → Completed`, `Active → Cancelled`, `Active → Expired`, `Active → Replaced` — todas fechadas, sem string livre
- [ ] 2.5 Estender `internal/platform/agent/identity_context.go` com `InboundExecutionFromContext(ctx)` retornando `resourceID, threadID, messageID string, itemSeq int, ok bool`, mantendo `InboundIdentityFromContext` como wrapper legado — extensão genérica de chave opaca do substrato, sem semântica de domínio
- [ ] 2.6 Implementar `PendingEntryReaper` como configuração de `workflow.NewStaleSuspendedReaper("pending-entry", 35*time.Minute)` — registrado pelo chamador (6.0)
- [ ] 2.7 Testes com engine fake/in-memory: start → resume com merge-patch; confirmação (AwaitingSlotConfirmation → "sim"); cancelamento; expiração; substituição (replaced)

## Detalhes de Implementação

Ver `techspec.md` seções **"Workflow pending-entry"**, **"Retomada de Pendência"**, **"Expiração e Reaper"** e **"Abertura de Pendência"**.

Resume obrigatório via merge-patch (R-WF-KERNEL-001.7):

```json
{"resumeText":"custo fixo","messageId":"wamid-001"}
```

O step de resume aplica o delta sobre `Snapshot.State` completo; nunca substitui o estado inteiro pelo payload.

Status `Replaced` retorna `PendingEntryResult{Handled: false}` para que o consumer processe a nova frase como nova operação (RF-31, RF-32). A pendência substituída não pode executar escrita em turnos posteriores.

`SuspendedAt` é o campo no snapshot que registra o momento de suspensão; a verificação de expiração usa `now.Sub(state.SuspendedAt) > 30*time.Minute` dentro de `DecidePendingResume` (pura, recebe `now`).

Key canônica de correlação: `<resourceID>:<threadID>:pending-entry` — opaca, sem label de métrica.

Se `Engine.Start` retornar `ErrRunAlreadyExists`, tratar como retomada ou substituição conforme decisão determinística de 1.0 — nunca criar schema paralelo.

## Critérios de Sucesso

- `go build ./internal/agents/application/workflows/...` passa
- `go test -race -count=1 ./internal/agents/application/workflows/...` verde incluindo 2.7
- `InboundExecutionFromContext` expõe `threadID` para montar a key `<resourceID>:<threadID>:pending-entry`; `InboundIdentityFromContext` continua compilando como wrapper legado (RF-40)
- Start com estado inicial persiste snapshot e suspende
- Resume com merge-patch `{"resumeText":"cancela"}` fecha com status=`Cancelled` (G7-04)
- Resume com `SuspendedAt` há 31 min fecha com status=`Expired` (G7-08)
- Resume com nova frase completa fecha pendência com status=`Replaced` e retorna `Handled=false` (G7-01)
- `ErrRunAlreadyExists` não causa pânico nem escrita duplicada
- Gate R-WF-KERNEL-001.1: `grep -rn "internal/transactions\|internal/billing\|internal/identity" internal/platform/workflow/` retorna vazio (kernel intacto)
- Gate zero comentários passa

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — workflow durável sobre workflow.Engine[PendingEntryState] do substrato internal/platform/workflow; start/resume/cancel/expire/replaced são primitivos do runtime agent da plataforma

## Testes da Tarefa

- [ ] `pending_entry_workflow_test.go`: engine fake/in-memory — start → suspend; resume com patch → resolve; cancel; expire (now+31min); replaced (handled=false)
- [ ] Cenários G7-04, G7-05, G7-06 (cancelamentos), G7-08 (expiração), G7-01 (replaced) cobertos como casos de teste do workflow
- [ ] Verificar que step de resume aplica merge-patch e não substitui estado inteiro (R-WF-KERNEL-001.7)
- [ ] Verificar que reaper está configurado com `staleAfter=35*time.Minute`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/agents/application/workflows/pending_entry_workflow.go` (novo)
- `internal/agents/application/workflows/pending_entry_workflow_test.go` (novo)
- `internal/platform/agent/identity_context.go` (estender com InboundExecutionFromContext, wrapper legado InboundIdentityFromContext)
- `internal/agents/application/workflows/pending_entry_state.go` (de 1.0)
- `internal/agents/application/workflows/pending_entry_decisions.go` (de 1.0)
- `internal/agents/application/workflows/destructive_confirm_workflow.go` (referência de padrão existente)
- `internal/platform/workflow/engine.go` (consumido, não modificado)
- `internal/platform/workflow/infrastructure/postgres/store.go` (consumido, não modificado)
- `.specs/prd-conversa-agentiva-fluida/techspec.md` (seções "Workflow pending-entry", "Retomada", "Expiração e Reaper")
- `.specs/prd-conversa-agentiva-fluida/scenarios.md` (G7-01, G7-04..G7-08)
