# Tarefa 6.0: Continuer, reaper, wiring e limpeza do enum compartilhado

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Conectar o workflow `card-update-confirm` ao runtime: continuer de resume (Run auditável), reaper de TTL, wiring em `module.go`, inserção na cadeia de resume do consumer WhatsApp e remoção do caminho antigo (`OpUpdateCard`) do `destructive-confirm`.

<requirements>
- `CardUpdateConfirmContinuer` retoma o run e abre Run auditável; métrica `agents_card_update_confirm_total{outcome}` com cardinalidade controlada.
- Reaper `agents-card-update-reaper` (TTL 15min, cron `*/5 * * * *`, batch 100).
- `tryContinueCardUpdate` inserido logo após `tryContinueCardCreate`.
- `OpUpdateCard`, `executeUpdateCard` e o case de `successMessage` removidos do `destructive-confirm`.
- Cobre RF-09, RF-14, RF-29, RF-30, RF-31.
</requirements>

## Subtarefas

- [ ] 6.1 Criar `internal/agents/application/usecases/card_update_confirm_continuer.go` (análogo ao de criação: `Continue`, `openRun`, merge-patch `{resumeText, incomingMessageId}`, métrica `agents_card_update_confirm_total`).
- [ ] 6.2 Criar `internal/agents/infrastructure/jobs/handlers/card_update_reaper_job.go` usando `workflow.NewStaleSuspendedReaper(store, CardUpdateConfirmWorkflowID, 15m, 100, o11y)`.
- [ ] 6.3 Wiring em `internal/agents/module.go`: `NewEngine[CardUpdateState]`, `BuildCardUpdateConfirmWorkflow`, continuer, reaper e resolver do consumer; `update_card` deixa de receber `confirmEngine/confirmDef`.
- [ ] 6.4 Inserir `tryContinueCardUpdate` na cadeia de resume de `whatsapp_inbound_consumer.go` imediatamente após `tryContinueCardCreate`; wiring `WithCardUpdateResolver`.
- [ ] 6.5 Remover `OpUpdateCard` de `confirm_state.go` em TODOS os sites: a constante (última do enum, valor 7 — sem reordenar as demais), o case em `String()`, o case em `ParseOperationKind` e ajustar o limite superior de `IsValid()` de `<= OpUpdateCard` para `<= OpDeleteRecurrence`. Em `destructive_confirm_workflow.go`: remover a entrada de `buildExecMap`, a função `executeUpdateCard` e o case de `OpUpdateCard` em `successMessage`. As 6 operações remanescentes do `destructive-confirm` (`OpDeleteEntry`, `OpEditEntry`, `OpDeleteCard`, `OpConfirmRegister`, `OpUpdateRecurrence`, `OpDeleteRecurrence`) permanecem intactas.

## Detalhes de Implementação

Ver `techspec.md` seções "Arquitetura do Sistema", "Monitoramento e Observabilidade" e ADR-001. Depende de 4.0 e 5.0.

## Critérios de Sucesso

- Resume de edição ocorre antes do agente, sem colidir com create (chaves distintas).
- Run auditável por execução; métrica com rótulos enum-only (sem `user_id`/`card_id`).
- Reaper purga runs suspensos além do TTL sem estado órfão.
- `destructive-confirm` continua íntegro para delete/edit de outras entidades; build/vet/lint/race verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — wiring Thread→Run, continuer de resume, reaper e cadeia de inbound do substrato de agent; métricas de Run com cardinalidade controlada.

## Testes da Tarefa

- [ ] Testes unitários: `card_update_confirm_continuer` (handled/response/outcome); ordenação da cadeia de resume no consumer; reaper.
- [ ] Testes de integração: `//go:build integration` — resume end-to-end via continuer com Postgres real; verificação de que `destructive-confirm` segue funcional para delete/edit.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/usecases/card_update_confirm_continuer.go`
- `internal/agents/infrastructure/jobs/handlers/card_update_reaper_job.go`
- `internal/agents/module.go`
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`
- `internal/agents/application/workflows/confirm_state.go`
- `internal/agents/application/workflows/destructive_confirm_workflow.go`
