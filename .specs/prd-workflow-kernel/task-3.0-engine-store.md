# Tarefa 3.0: Engine + porta Store + fake (suspend/resume, retry, observabilidade)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar o `Engine[S]` (Start/Resume, cursor de retomada, laço de retry, falha terminal,
observabilidade/auditoria por passo), a porta `Store` (interface no consumidor) com `Snapshot`/
`StepRecord` e um `Store` fake in-memory para testes. Sem adapter postgres ainda.

<requirements>
- RF-06: suspend/resume como capacidade de passo (sinal tipado) com retomada do ponto de suspensão.
- RF-08: snapshot durável só quando `Definition.Durable=true`; leitura pura executa in-process.
- RF-09: resume idempotente (lógica de nível de engine; prova de restart fica em 4.0).
- RF-11: retry por passo com política configurável (máx. tentativas + backoff).
- RF-12: falha terminal determinística → run `failed`, sem retry infinito.
- RF-13: Run e Step auditáveis (status, `duration_ms`, `attempt`, erro) via `StepRecord`.
- RF-14: observabilidade por passo (span/métrica) com cardinalidade controlada.
- RF-15: estados fechados nas assinaturas do engine/store.
</requirements>

## Subtarefas

- [ ] 3.1 `store.go`: porta `Store` (`Insert`/`Load`/`Save`(CAS)/`AppendStep`/`DeleteCompleted`),
  structs `Snapshot`/`StepRecord`, `RetryPolicy`, `ErrVersionConflict`.
- [ ] 3.2 `engine.go`: `Engine[S]`, `Definition[S]`, `Start`/`Resume`, `RunResult[S]`; cursor de
  retomada (reentrar no passo suspenso com estado mesclado), short-circuit de passos de guarda,
  laço de `Retry` no nível do run e falha terminal por `MaxAttempts`.
- [ ] 3.3 Observabilidade: spans `workflow.engine.start|resume`, `workflow.step.execute` e métricas
  `workflow_runs_total`/`workflow_run_duration_seconds`/`workflow_steps_total`/`workflow_step_duration_seconds`/
  `workflow_suspend_total`/`workflow_resume_total` (labels enums fechados; sem `user_id`/`correlation_key`).
- [ ] 3.4 `Store` fake in-memory + testes de Start/Resume/Suspend/cursor/retry/falha terminal e do
  comportamento `Durable=false` (não persiste).

## Detalhes de Implementação

Ver techspec.md → "Interfaces Chave", "Resume", "Retry e falha terminal" e "Monitoramento e
Observabilidade". `time.Now().UTC()` inline (sem abstração de tempo). `Store` é interface no consumidor
(R6.3). `defer func(){ _ = ... }()` quando aplicável.

## Critérios de Sucesso

- `Start`/`Resume` cobrem auto-conclusão, suspensão+retomada, short-circuit e falha terminal.
- `Durable=false` não toca o `Store` (verificável no fake).
- Métricas/spans emitidos com cardinalidade controlada.
- Tudo testável sem postgres (fake in-memory).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/workflow/store.go` (novo)
- `internal/platform/workflow/engine.go` (novo)
- `internal/platform/workflow/engine_test.go`, `store_fake_test.go` (novos)
