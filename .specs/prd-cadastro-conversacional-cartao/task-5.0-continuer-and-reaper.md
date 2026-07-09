# Tarefa 5.0: Continuer e Reaper do card-create-confirm

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechar o ciclo de retomada e limpeza do workflow dedicado `card-create-confirm`: um continuer que abre um `Run` auditável, retoma o snapshot suspenso por merge-patch e mapeia o resultado para resposta/métrica, mais um job de reaper que purga runs suspensos além do TTL. Cobre o caminho de resume (RF-18), a auditabilidade com erro persistido (RF-15), a métrica de cardinalidade controlada (RF-16) e a ausência de run órfão (RF-21). Referência: techspec.md §Design de Implementação (Continuer + Reaper), §Idempotência/Auditoria/Métrica, §Exclusão Mútua e Ordem de Resume; ADR-001.

<requirements>
- `CardCreateConfirmContinuer` que abre `Run` (`RunStatusRunning`) via `RunStore`, monta merge-patch `{"resumeText":message,"incomingMessageId":wamid}`, chama `engine.Resume(def, CardCreateKey(resourceID), patch)`, mapeia `CardCreateState.ResponseText`→reply em suspended/completed e fecha o run (`closeRun(RunStatusSucceeded|Failed, err)`) populando a coluna de erro em falha de infra (RF-15). Assinatura de retorno `(handled bool, reply string, err error)`.
- Contador análogo a `agents_destructive_confirm_total`; labels apenas enums fechados, sem `user_id` (RF-16, R-AGENT-WF-001.5).
- Job de reaper em `internal/agents/infrastructure/jobs/handlers/` espelhando `ConfirmReaperJob`, com `workflow.NewStaleSuspendedReaper(store, CardCreateConfirmWorkflowID, 15m, batch, o11y)` e schedule `*/5 * * * *`; garante zero run suspenso órfão (RF-21).
- Zero comentários; job handler como adapter fino (R-ADAPTER-001).
</requirements>

## Subtarefas

- [ ] 5.1 Criar `internal/agents/application/usecases/card_create_confirm_continuer.go` com `CardCreateConfirmContinuer` espelhando `PendingEntryContinuer`: `openRun`/`closeRun`, `engine.Resume` com merge-patch `{"resumeText","incomingMessageId"}` sobre `CardCreateKey(resourceID)`, mapeamento `CardCreateState.ResponseText`→reply para suspended/completed, `closeRun(RunStatusFailed, err.Error())` em falha de infra (RF-15), retorno `(handled bool, reply string, err error)`.
- [ ] 5.2 Emitir o contador do continuer análogo a `agents_destructive_confirm_total` com labels de enum fechado (`outcome`/`result`), sem `user_id`/`category_id` (RF-16, R-AGENT-WF-001.5).
- [ ] 5.3 Criar `internal/agents/infrastructure/jobs/handlers/card_create_reaper_job.go` espelhando `ConfirmReaperJob`, backed por `workflow.NewStaleSuspendedReaper(store, CardCreateConfirmWorkflowID, 15m, batch, o11y)`, schedule `*/5 * * * *` (RF-21).
- [ ] 5.4 Escrever testes unitários do continuer e do reaper (ver §Testes da Tarefa).

## Detalhes de Implementação

Ver techspec.md §Design de Implementação (Continuer `CardCreateConfirmContinuer` + Reaper), §Idempotência, Auditoria e Métrica (RF-14/15/16, ADR-003), §Exclusão Mútua e Ordem de Resume (RF-18), §Monitoramento e Observabilidade, e ADR-001 (workflow dedicado, TTL 15 min, continuer e reaper próprios). O continuer segue o padrão de `PendingEntryContinuer` (abre/fecha `Run`, resume por merge-patch antes do parse) e o reaper segue o padrão de `ConfirmReaperJob` sobre `workflow.NewStaleSuspendedReaper`. Não duplicar prosa da techspec — consultá-la como fonte.

## Critérios de Sucesso

- Continuer não-suspenso retorna `handled=false` e não emite reply; suspenso/completado retorna `ResponseText` como reply.
- Falha de infra fecha o run com `RunStatusFailed` e a mensagem de erro persistida na coluna do run (RF-15); nunca falha silenciosa.
- Métrica do continuer registrada com outcome tipado, sem label de alta cardinalidade (RF-16).
- Reaper retorna a contagem de runs purgados; nenhum run permanece `RunStatusSuspended` além do TTL (RF-21).
- `build`, `vet`, `test -race` e `lint` verdes no módulo alterado; zero comentários em `.go` de produção.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — continuer, Run auditável e reaper no padrão do substrato agentivo.
- `design-patterns-mandatory` — gate de desenho do trio Go obrigatório para o continuer e o reaper.

## Testes da Tarefa

- [ ] Testes unitários — continuer (testify/suite, whitebox, `fake.NewProvider()`, mocks do `.mockery.yml`): não-suspenso→`handled=false`; suspenso→reply de `ResponseText`; falha de infra→`closeRun` `RunStatusFailed` com erro persistido; outcome de métrica registrado. Reaper job: `Reap` retorna contagem.
- [ ] Testes de integração — cobertos na Tarefa 9.0 (ciclo suspend→resume, falha de infra populando `run.error`).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/agents/application/usecases/card_create_confirm_continuer.go` (novo)
- `internal/agents/application/usecases/card_create_confirm_continuer_test.go` (novo)
- `internal/agents/infrastructure/jobs/handlers/card_create_reaper_job.go` (novo)
- `internal/agents/infrastructure/jobs/handlers/card_create_reaper_job_test.go` (novo)
- `internal/agents/application/usecases/pending_entry_continuer.go` (template)
- `internal/agents/application/usecases/destructive_confirm_continuer.go` (template — métrica)
- `internal/agents/infrastructure/jobs/handlers/confirm_reaper_job.go` (template)

## Dependências

- Depende de 4.0 (workflow + execução idempotente: `CardCreateState`, `DecideCardCreateConfirmation`, `BuildCardCreateConfirmWorkflow`, `CardCreateKey`, `CardCreateConfirmWorkflowID`).
- Paralelizável com 6.0.
