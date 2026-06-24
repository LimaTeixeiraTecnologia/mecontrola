# Tarefa 5.0: Housekeeping job + configuração de ambiente

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar o job de housekeeping (retenção configurável dos runs concluídos) reaproveitando
`internal/platform/worker`, e adicionar a configuração de ambiente do kernel (flag de cutover, retry e
housekeeping) com validação no padrão de `configs/config.go`.

<requirements>
- RF-17: job de housekeeping com retenção configurável (purga runs concluídos após N dias).
- Configuração de ambiente do kernel: `WORKFLOW_KERNEL_*` (flag, retry/backoff, housekeeping).
</requirements>

## Subtarefas

- [ ] 5.1 `internal/platform/workflow/housekeeping.go`: `HousekeepingJob` implementando `worker.Job`
  (`Name`/`Schedule`/`Timeout`/`Run`) que chama `Store.DeleteCompleted(retention, limit)`, no molde de
  `outbox.HousekeepingJob`.
- [ ] 5.2 `configs/config.go`: bloco `WorkflowKernelConfig` com
  `WORKFLOW_KERNEL_TRANSACTIONS_WRITE_ENABLED` (default false), `WORKFLOW_KERNEL_MAX_ATTEMPTS` (3),
  `WORKFLOW_KERNEL_RETRY_BASE_BACKOFF` (200ms), `WORKFLOW_KERNEL_RETRY_MAX_BACKOFF` (5s),
  `WORKFLOW_KERNEL_HOUSEKEEPING_RETENTION_DAYS` (30), `WORKFLOW_KERNEL_HOUSEKEEPING_SCHEDULE` (`@daily`),
  com validação ("X inválido").
- [ ] 5.3 `configs/config_test.go`: cenários de validação (valores inválidos + defaults).

## Detalhes de Implementação

Ver techspec.md → "Configuração (env)" e "Pontos de Integração". O registro do job no `worker.Manager`
ocorre na tarefa 8.0 (wiring em `module.go`); aqui entrega-se o job e a config. Schedule cron valida via
robfig (mesmo padrão de `OUTBOX_*`).

## Critérios de Sucesso

- `HousekeepingJob` purga runs concluídos além da retenção e preserva ativos/suspensos (coberto por
  integration test em conjunto com 4.0 ou nesta tarefa).
- Config carrega defaults e rejeita valores inválidos com mensagem clara.
- Zero comentários em `.go`.

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
- `internal/platform/workflow/housekeeping.go` (novo)
- `configs/config.go` (bloco WorkflowKernelConfig)
- `configs/config_test.go` (cenários novos)
