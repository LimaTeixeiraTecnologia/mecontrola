# Tarefa 8.0: Integração sob feature flag + wiring + resume-before-parse

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Conectar o kernel ao agent sob feature flag: `dispatchWrite` e `continuePendingExpenseConfirmation`
passam a delegar ao `Engine.Start`/`Engine.Resume` quando a flag está ligada, mantendo o caminho atual
como fallback (rollback instantâneo). Wiring de `Engine`/`Store`/`HousekeepingJob` em `module.go`,
incluindo a drenagem do draft legado.

<requirements>
- RF-18: o agent consome o kernel mantendo Thread/Run/WorkingMemory/PendingStep semânticos próprios.
- RF-22: write de transactions migrado para multi-step do kernel, coexistindo (aditivo) sob flag.
- RF-25: adicionar fluxo multi-step novo exige apenas registrar passos no seam, zero novo `case intent.Kind`.
- Ver ADR-005 (feature flag/cutover) e ADR-003 (drenagem do draft legado).
</requirements>

## Subtarefas

- [ ] 8.1 `daily_ledger_agent.go`: sob `WORKFLOW_KERNEL_TRANSACTIONS_WRITE_ENABLED`, `dispatchWrite`
  chama `Engine.Start(Definition transactions_write)`; flag off mantém o caminho atual intacto.
- [ ] 8.2 `continuePendingExpenseConfirmation`: sob flag, chama `Engine.Resume(key=user:channel)`;
  ausência de run suspenso ⇒ segue para o parse; fallback de leitura do `agent_sessions.pending_action`
  legado durante a janela de drenagem.
- [ ] 8.3 `module.go`: DI manual do `Engine`, `Store` (sobre `sessionDB`), `StoreFactory`, registro do
  `HousekeepingJob` na lista de jobs do `worker.Manager`, e fiação da flag/config.
- [ ] 8.4 Demonstração de DX (RF-25): registrar os passos via seam (`buildRegistry`/equivalente) sem
  adicionar `case intent.Kind`.

## Detalhes de Implementação

Ver techspec.md → "Fluxo de Dados", "Resume" e "Sequenciamento (itens 7–8)". Carregar `mastra`.
Sem tx cross-módulo: snapshot no `sessionDB`, persist no módulo transactions (idempotência por run
cobre no-duplicate). Métricas `agent_*` permanecem inalteradas.

## Critérios de Sucesso

- Flag off ⇒ comportamento atual byte-a-byte; flag on ⇒ caminho kernel ativo com fallback de drenagem.
- `HousekeepingJob` registrado e ativo no worker.
- Nenhum novo `case intent.Kind`; seam é o único ponto de extensão.
- Gates `R-AGENT-WF-001`/`R-ADAPTER-001` verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — integração Workflow/Tool + ciclo resume no `internal/agent` sob R-AGENT-WF-001.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agent/application/services/{daily_ledger_agent,agent_workflows}.go`
- `internal/agent/module.go`
- `configs/config.go` (consumo da flag/config)
