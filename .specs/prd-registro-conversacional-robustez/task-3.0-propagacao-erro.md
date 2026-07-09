# Tarefa 3.0: Propagação de erro — fim do swallow + Run auditável no resume

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Parar de engolir o erro de escrita e tornar toda falha rastreável: passo do workflow propaga
`StepStatusFailed` + erro real; o turno de confirmação/resume passa a ser um Run auditável em
`platform_runs` com `error` preenchido; span de erro pesquisável e log ERROR correlacionados por
`thread_id`/`run_id`/`wamid`. Causa-raiz do incidente. Ver ADR-002.

<requirements>
- RF-10: `executeWithIdempotency`, `executeDirectWrite` e `validateCategoryForWrite` propagam o erro
  real para `platform_runs.error` e marcam `StepStatusFailed` (fim do swallow que retorna
  `StepStatusCompleted` com `error` em branco).
- RF-11: falha upstream do income (tool `register_income`/runtime) grava o erro real em
  `platform_runs.error` e emite span de erro pesquisável vinculado a `thread_id`, `run_id`, `wamid`.
- RF-12: log nível ERROR com `thread_id`, `run_id`, `wamid`; mensagem ao usuário permanece amigável.
- RF-13: `agents_write_total{operation,outcome}` mantida (outcome fechado), cardinalidade controlada
  (sem `user_id`/`correlation_key`/`category_id`).
</requirements>

## Subtarefas

- [ ] 3.1 Refatorar `executeWithIdempotency` (`:444-461`), `executeDirectWrite` (`:463-473`) e
  `validateCategoryForWrite` (`:413-442`) para retornar `StepStatusFailed` + `fmt.Errorf("ctx: %w", err)`
  na falha real; preservar `ResponseText` amigável; manter `Completed` em cancelamento/replay.
- [ ] 3.2 Fazer `pending_entry_continuer.go` (`:79`)/`register_attempt.go` abrir/fechar um Run
  auditável em `platform_runs` ao redor de `engine.Resume` (Thread + `wamid`), propagando o motivo
  real para `closeRun(errStr=...)`.
- [ ] 3.3 Garantir no turno de tool-calling (`runtime.go` `finishRun`/`closeRun` `:155-259`) o
  preenchimento do motivo real quando a tool falha antes do suspend.
- [ ] 3.4 Emitir span de erro pesquisável no consumidor (`RecordError` + status error + atributos) e
  log ERROR; manter o kernel intocado.

## Detalhes de Implementação

Ver ADR-002 (Parte A/B, duas tabelas `platform_runs.error` semântico vs `workflow_runs.last_error`
mecânico) e techspec.md "Monitoramento e Observabilidade". Não introduzir branching de domínio no
kernel (R-WF-KERNEL-001); identidade de domínio vive no span/log do consumidor.

## Critérios de Sucesso

- Escrita falha no resume ⇒ `platform_runs.error` + `workflow_runs.last_error` preenchidos, span de
  erro pesquisável e log ERROR com `thread_id`/`run_id`/`wamid`.
- Cancelamento "não" ⇒ run não-`failed`; sucesso ⇒ run `succeeded`.
- Mensagem ao usuário permanece amigável (sem detalhe técnico).
- Nenhum comentário em Go de produção; kernel sem alteração de domínio.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — Run auditável, Thread→Run, substrato `internal/platform/agent` e consumidor `internal/agents`.
- `domain-modeling-production` — estados fechados (`RunStatus`/`StepStatus`/`ToolOutcome`) e distinção falha-vs-cancelamento.
- `design-patterns-mandatory` — gate `não aplicar padrão` (propagação direta de erro, sem estrutura nova).

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/pending_entry_workflow.go`
- `internal/agents/application/usecases/pending_entry_continuer.go`, `register_attempt.go`
- `internal/platform/agent/runtime.go`, `types.go`
- `internal/agents/application/usecases/idempotent_write.go`
