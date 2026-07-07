# Golden Sets: Workflow Engine

Este documento define os cenarios de referencia (golden sets) para o `internal/platform/workflow.Engine[S]`. Cada cenario descreve entrada, execucao e saida esperada. Devem ser mantidos alinhados com `internal/platform/workflow/engine_test.go`.

## Cenario 1: Auto-complete duravel

Entrada:
- Workflow ID: `test_workflow`
- Correlation key: `user:ch`
- Estado inicial: `{value: 0}`
- Definicao: `Sequence(root, step_a(+1), step_b(+10))`
- Durable: true

Execucao:
- Step `a` incrementa value em 1.
- Step `b` incrementa value em 10.

Saida esperada:
- `result.Status == RunStatusSucceeded`
- `result.State.Value == 11`
- `result.RunID != uuid.Nil`
- Snapshot persistido com `Status == RunStatusSucceeded` e `State.Value == 11`.

## Cenario 2: Auto-complete nao duravel

Entrada:
- Workflow ID: `read_workflow`
- Correlation key: `user:ch`
- Estado inicial: `{value: 0}`
- Definicao: `Sequence(root, step_a(+5))`
- Durable: false

Execucao:
- Step `a` incrementa value em 5.

Saida esperada:
- `result.Status == RunStatusSucceeded`
- `result.State.Value == 5`
- Nenhum snapshot persistido (`store.Load` retorna `found == false`).

## Cenario 3: Suspend e resume

Entrada:
- Workflow ID: `suspend_workflow`
- Correlation key: `user:ch`
- Estado inicial: `{value: 0}`
- Definicao: `Sequence(root, step_a(+1), suspend_step, step_c(+100))`
- Durable: true

Execucao (Start):
- Step `a` incrementa value em 1.
- Step `suspend` retorna `StepStatusSuspended` com `SuspendReason == SuspendAwaitingInput`.

Saida esperada (Start):
- `result.Status == RunStatusSuspended`
- `result.State.Value == 1`
- Snapshot persistido com `Status == RunStatusSuspended`.

Execucao (Resume):
- Merge-patch aplica `{value: 1}` (sem alteracao).
- Step `c` incrementa value em 100.

Saida esperada (Resume):
- `result.Status == RunStatusSucceeded`
- `result.State.Value == 101`
- Snapshot atualizado para `RunStatusSucceeded`.

## Cenario 4: Erro de step

Entrada:
- Workflow ID: `error_workflow`
- Correlation key: `user:ch`
- Estado inicial: `{value: 0}`
- Definicao: `Sequence(root, step_a(+1), error_step(err))`
- Durable: true

Execucao:
- Step `a` incrementa value em 1.
- Step `error` retorna erro `err`.

Saida esperada:
- `result.Status == RunStatusFailed`
- `result.Error != nil` e contem `err`.
- `result.State.Value == 1` (estado ate o ultimo step bem-sucedido).
- Snapshot persistido com `Status == RunStatusFailed`.

## Cenario 5: Falha por status de step

Entrada:
- Workflow ID: `fail_status_workflow`
- Correlation key: `user:ch`
- Estado inicial: `{value: 0}`
- Definicao: `Sequence(root, fail_status_step)`
- Durable: true

Execucao:
- Step retorna `StepStatusFailed` sem erro.

Saida esperada:
- `result.Status == RunStatusFailed`
- Snapshot persistido com `Status == RunStatusFailed`.

## Cenario 6: Retry ate sucesso

Entrada:
- Workflow ID: `retry_workflow`
- Correlation key: `user:ch`
- Estado inicial: `{value: 0}`
- Definicao: `Sequence(root, Retry(step_flaky, maxAttempts=3))`
- Durable: true

Execucao:
- Step falha nas 2 primeiras tentativas.
- Step completa na 3a tentativa, incrementando value em 7.

Saida esperada:
- `result.Status == RunStatusSucceeded`
- `result.State.Value == 7`
- Snapshot final `RunStatusSucceeded`.

## Criterios de manutencao

- Qualquer mudanca no `Engine` deve manter todos os golden sets passando.
- Novos cenarios so devem ser adicionados quando representarem comportamento contratual publico.
- Estados `RunStatus` e `StepStatus` devem permanecer tipos fechados; nunca usar string solta.
