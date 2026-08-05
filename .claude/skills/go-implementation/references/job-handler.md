# Job Handler

<!-- TL;DR
Job handler e adapter fino de worker/cron: monta input, chama use case e respeita cancelamento e drain quando houver execucao longa.
Keywords: job, worker, cron, adapter
Load complete when: tarefa altera job handler ou registro de job.
-->

## Objetivo
Padronizar jobs com lifecycle previsivel e sem logica deslocada.

## Regras
- Job handler delega para use case.
- Cancelamento via `context.Context` e obrigatorio quando houver I/O ou execucao longa.
- Politica de schedule, retry e timeout deve vir do runner/plataforma quando ja existir.
- Nao mover regra de negocio para o handler do job.

## Validacao Minima
- `go test -count=1` no pacote do job/use case alterado.
- `go build` no worker ou modulo afetado quando o registro do job mudar.

## Proibido
- SQL direto em job handler.
- Dependencia direta de repositorio/client quando o use case existente cobre o fluxo.
