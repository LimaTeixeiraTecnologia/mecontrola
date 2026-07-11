# Tarefa 8.0: Agente — confirmação diff antes→depois + conflito version→re-confirm + no-op + idempotência + anti-simulação

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Completar a robustez do fluxo de edição: resumo de confirmação em diff antes→depois, tratamento de conflito de `version` (re-ler + re-confirmar), no-op sem gravação, idempotência da edição e anti-simulação/formatação.

<requirements>
- RF-16: resumo de confirmação com impacto (diff antes→depois).
- RF-17/RF-18: aceite/cancelamento verbatim; ambíguo → reprompt único depois cancela.
- RF-19: estado de espera persistido antes de perguntar; resume por merge-patch antes do parse; TTL coleta 35min / confirmação 5min; expiração cancela sem efeito.
- RF-20/RF-21: gravação com `version`; conflito → re-ler estado atual e re-apresentar confirmação fresca (sem last-writer-wins).
- RF-22: no-op (valores idênticos) não grava, não incrementa version, informa que nada mudou.
- RF-23: idempotência por `(wamid, itemSeq, operation="edit_entry")`; replay reconhecido.
- RF-25/RF-26: nunca afirmar sucesso sem retorno real; erro exato "Não consegui registrar. Tente novamente em breve."; formatação WhatsApp asterisco único; sem termos internos.
- RF-32: cada edição é Run auditável (thread/run/workflow/tool/status/duration/error/decision_id).
</requirements>

## Subtarefas

- [ ] 8.1 `PendingEntryState.Prev*` (valores atuais) + `buildEditConfirmSummary` (diff antes→depois), reusando `buildConfirmSummary` para o estado resultante.
- [ ] 8.2 Tratamento de conflito de `version` no `executeWithIdempotency`/resume: re-ler transação atual (nova version/valores), reconstruir resumo e re-suspender em `AwaitingSlotConfirmation`.
- [ ] 8.3 No-op: consumir a decisão de no-op do domínio (tarefa 1.0) e responder sem gravar/evento.
- [ ] 8.4 Confirmar idempotência `operation="edit_entry"` no `IdempotentWrite` e o mapeamento de desfecho (`Routed`/`Reconciled`/`Replay`).
- [ ] 8.5 Garantir Run auditável e métricas de escrita da edição (cardinalidade controlada).
- [ ] 8.6 Testes testify/suite (conflito, no-op, diff, replay, expiração, anti-simulação) + golden dos textos verbatim.

## Detalhes de Implementação

Ver `techspec.md` (Design de Implementação; Monitoramento) e `adr-003`. Reusar TTL/reprompt/merge-patch já existentes do `pending-entry`.

## Critérios de Sucesso

- Conflito de version nunca sobrescreve silenciosamente; no-op não gera evento; idempotência comprovada.
- Scorer `write_persistence_accuracy` verde para `operation=edit_entry`.
- Sem termos internos vazados; formatação WhatsApp correta.
- `go build`, `go vet`, `go test -race`, lint do módulo verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — confirmação/resume/idempotência/Run auditável no substrato de agente.
- `domain-modeling-production` — estados de espera fechados e contrato de resume por merge-patch (state-as-type).

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/pending_entry_workflow.go`
- `internal/agents/application/workflows/pending_entry_state.go`
- `internal/agents/application/workflows/pending_entry_decisions.go`
- `internal/agents/application/usecases/idempotent_write.go`
- `internal/agents/application/scorers/write_persistence_accuracy.go`
