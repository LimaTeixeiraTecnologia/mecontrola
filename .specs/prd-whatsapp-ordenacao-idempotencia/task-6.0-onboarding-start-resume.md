# Tarefa 6.0: Onboarding Start idempotente-resume + persistência de turnos

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Tratar a violação do índice único parcial `workflow_runs_active_key_uidx` no `Start` do onboarding como
retomada (resume) do run ativo, sob a serialização por usuário do claim particionado (fim do TOCTOU), e
persistir os turnos de onboarding em `platform_messages` na mesma thread do agente (RF-09, RF-10, RF-11,
RF-12; ADR-003).

<requirements>
- RF-09: violação de `workflow_runs_active_key_uidx (workflow, correlation_key)` ao iniciar onboarding → tratada como resume do run ativo, não `onboarding_error` genérico.
- RF-10: resolução onboarding-vs-agente atômica sob a serialização por usuário (RF-01), eliminando a janela TOCTOU `Load`→check→`Start`.
- RF-11: turnos de onboarding (inbound e outbound) persistidos em `platform_messages` via o mesmo contrato `memory.MessageStore.Append` do agent runtime, na mesma thread `(resourceId=userID, threadId=peer)` (D-15).
- RF-12: usuário com marcador de conclusão presente NÃO reinicia onboarding; a mensagem segue para o agente (PRESERVAR o bloqueio existente).
- Adapter/store `workflow`: mapear `SQLSTATE 23505` do `Insert` para `ErrRunAlreadyExists` (sentinela existente), mantendo o kernel `engine.Start` genérico (R-WF-KERNEL-001 — sem domínio, mecanismo `unique_violation → Load → resume`).
- `engine.Start`: ao receber `ErrRunAlreadyExists` do `Insert`, `Load` + retornar resume (reaproveitar caminho `resolveConflictOrFail`/resume existente); merge-patch no resume preservado (R-AGENT-WF-001.7).
- Persistir turno no ponto único do fluxo (evitar duplo-persistir em resume).
- Zero comentários; `OnboardingPhase` permanece tipo fechado.
</requirements>

## Subtarefas

- [ ] 6.1 `.../workflow/infrastructure/postgres/store.go`: mapear `SQLSTATE 23505` do `Insert` → `ErrRunAlreadyExists`.
- [ ] 6.2 `engine.Start`: ao receber `ErrRunAlreadyExists` do `Insert`, `Load` + resume (genérico, sem domínio).
- [ ] 6.3 `resolve_onboarding_or_agent.go`: tratar o retorno de resume como handled; garantir atomicidade sob claim particionado (tarefa 2.0); preservar o bloqueio por marcador de conclusão.
- [ ] 6.4 `onboarding_workflow.go`: chamar `messages.Append` para inbound e outbound de cada passo, na mesma thread do agente.
- [ ] 6.5 Testes unitários (testify/suite): violação de índice → resume (não `onboarding_error`); usuário concluído não reinicia; turno persistido uma vez.

## Detalhes de Implementação

Ver ADR-003 §Contexto/§Decisão/§Plano de Implementação e techspec §Arquivos Relevantes. A atomicidade
por usuário vem do claim particionado (tarefa 2.0, dependência) — a checagem `Load`→marcador→`Start`
deixa de ter corrida porque só há 1 evento do usuário em voo. A verificação de concorrência real (duas
`Start` simultâneas) é a CA-04 na tarefa 8.0.

## Critérios de Sucesso

- Conflito de Start vira resume do run existente (0 `onboarding_error`), sem perder progresso.
- Usuário concluído (marcador presente) não reinicia; mensagem vai ao agente.
- Turnos de onboarding aparecem em `platform_messages` (mesma thread), persistidos uma vez.
- Kernel `engine.Start` permanece genérico (sem domínio); SQLSTATE tratado no adapter.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `mastra` — altera `resolve_onboarding_or_agent`, `onboarding_workflow` e o contrato `MessageStore.Append` (Thread→Run, persistência de turnos); a skill é a base canônica do ciclo de onboarding/agente.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

Unitários testify/suite para `resolve_onboarding_or_agent` (violação → resume; concluído não reinicia).
CA-04 (duas Start concorrentes → 1 run, 2ª retoma, 0 error + turnos em `platform_messages`) é integração
na tarefa 8.0.

## Rollback

Reverter a captura de `unique_violation` no store restaura o comportamento atual (erro →
`onboarding_error`); a persistência de turnos pode ser desligada isoladamente (ADR-003 §Riscos).

## Done-when

- Suites unitárias verdes; resume no lugar de `onboarding_error`.
- Turnos de onboarding presentes em `platform_messages` (mesma thread do agente).
- Gate executável de pureza do kernel (R-WF-KERNEL-001.1 — deve retornar vazio):
  ```bash
  grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
    "internal/transactions\|internal/billing\|internal/identity\|internal/platform/agent\|internal/platform/memory\|intent\.\|pendingexpense\|onboarding" \
    internal/platform/workflow/ \
    && echo "FAIL: domínio no kernel workflow" && exit 1 || true
  ```
- Validação proporcional (concorrência — Start resume): `go build ./...`, `go vet ./internal/platform/workflow/... ./internal/agents/...`, `go test -race -count=1 ./internal/platform/workflow/... ./internal/agents/application/usecases/...`.

## Arquivos Relevantes
- `internal/agents/application/usecases/resolve_onboarding_or_agent.go`
- `internal/platform/workflow/engine.go` (`Start`, resume)
- `internal/platform/workflow/infrastructure/postgres/store.go` (mapeamento SQLSTATE 23505)
- `internal/agents/application/workflows/onboarding_workflow.go` (persistência de turnos)
- `internal/platform/agent/runtime.go` (contrato `messages.Append`), `internal/platform/memory/...`
