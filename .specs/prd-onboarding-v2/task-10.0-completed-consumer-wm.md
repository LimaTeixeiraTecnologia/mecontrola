# Tarefa 10.0: [agent] OnboardingCompletedConsumer (WorkingMemory assíncrona) + wiring

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar, em `internal/agent`, o consumidor `OnboardingCompletedConsumer` que reage ao evento
`onboarding.completed`: lê o contexto via binding `GetOnboardingContext` e sintetiza/persiste a
WorkingMemory (agent-owned), de forma idempotente e com retry pelo outbox. Remover a síntese inline
(`synthesizeAndStoreWM`) do dispatcher.

<requirements>
- RF-26: handoff por sinal determinístico (evento `onboarding.completed`).
- RF-34: síntese de WorkingMemory assíncrona, idempotente, com retry; nunca inline/bloqueante.
- ADR-003.
</requirements>

## Subtarefas

- [ ] 10.1 Criar `onboarding_completed_consumer.go` (adapter fino): decode envelope → binding `GetOnboardingContext` → `WorkingMemory.Upsert` (idempotente: no-op se WM já tem conteúdo).
- [ ] 10.2 Registrar o consumer no `module.go`/`buildEventHandlers` para `OnboardingCompleted.EventType()` ("onboarding.completed").
- [ ] 10.3 Remover `synthesizeAndStoreWM` inline do dispatcher (e deps `wmWriter`/`contextReader`).
- [ ] 10.4 Testes (suite testify): payload válido→Upsert; WM existente→no-op; erro de contexto/Upsert→erro (retry); user_id inválido→decodeFailed+erro.

## Detalhes de Implementação

Ver techspec.md → "WorkingMemory assíncrona (DR-02)" e ADR-003. Consumer é adapter fino
(R-ADAPTER-001.2); WorkingMemory é primitivo do agente (R-AGENT-WF-001.8).

## Critérios de Sucesso

- WM sintetizada via consumer (não inline); idempotente; falha retorna erro para retry.
- Dispatcher sem síntese inline nem deps de WM.
- **DR-10**: `max_attempts` configurado para `onboarding.completed`; após o teto, dead-letter +
  métrica/alerta `outbox_dead_letter_total{event_type}` (sem retry infinito).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `mastra` — consumer de evento + WorkingMemory (primitivo Mastra agent-owned) no `internal/agent`.

go-implementation (linguagem, auto) e agent-governance (governança, auto) também se aplicam.

## Testes da Tarefa

- [ ] Testes unitários (suite testify; 4 cenários do consumer)
- [ ] Testes de integração (T12 — consumir `onboarding.completed` persiste WM; reprocesso não duplica)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Definition of Done (DoD)

- [ ] Consumer criado e registrado para `onboarding.completed`.
- [ ] Síntese inline removida do dispatcher.
- [ ] Zero comentários no `.go`; sem SQL direto no consumer.
- [ ] `go build ./internal/agent/...` e `go test ./internal/agent/infrastructure/messaging/... -run OnboardingCompleted` passam.

## Critérios de Aceite (validações executáveis)

```bash
go build ./internal/agent/... && \
go test ./internal/agent/infrastructure/messaging/database/consumers/... -count=1
grep -rn "synthesizeAndStoreWM" internal/agent/infrastructure/onboarding/ && echo FAIL || echo OK
```

## Arquivos Relevantes
- `internal/agent/infrastructure/messaging/database/consumers/onboarding_completed_consumer.go` (novo)
- `internal/agent/infrastructure/onboarding/onboarding_tool_dispatcher.go` (modificado — remove WM inline)
- `internal/agent/module.go` (registro do consumer)
