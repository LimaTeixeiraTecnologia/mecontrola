# Tarefa 11.0: [agent] Hardening da saudação (GAP-1 + idempotência)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Endurecer o `OnboardingBoundConsumer` em `internal/agent`: usar `MessageID = envelope.EventID`,
verificar replay via `AgentDecision`, retornar **erro** quando a sessão ainda não existe/
`InProgress=false` (forçando retry do outbox — GAP-1), e marcar `welcome_sent_at` **após** o envio
bem-sucedido (via binding `MarkWelcomeSent`). Garantir que falha de LLM no turno não corrompe estado.

<requirements>
- RF-05: retry quando sessão ausente. RF-29: saudação idempotente (event_id + welcome_sent_at).
- RF-08/RF-32: falha de LLM = retry seguro; estado preservado; nada concluído/corrompido.
- ADR-002 (welcome_sent_at), techspec "Idempotência da saudação e ordem de entrega" e "Degradação por falha de LLM".
</requirements>

## Subtarefas

- [ ] 11.1 `MessageID = envelope.EventID.String()`; checar `AgentDecision.FindByMessage` antes de rotear (replay → no-op).
- [ ] 11.2 Sessão ausente/`InProgress=false` → retornar erro (retry); log warn `onboarding_not_started`.
- [ ] 11.3 Após envio bem-sucedido, registrar `AgentDecision` (event_id) e chamar binding `MarkWelcomeSent`.
- [ ] 11.4 Garantir que erro de LLM em `RunOnboardingTurn` não persiste transição (teste).
- [ ] 11.5 Testes (suite testify): 5 cenários do consumer + idempotência + erro de LLM não corrompe.

## Detalhes de Implementação

Ver techspec.md → "Idempotência da saudação e ordem de entrega" e "Degradação por falha de LLM".
Consumer é adapter fino (R-ADAPTER-001.2).

## Critérios de Sucesso

- Reprocessar `subscription_bound` não duplica saudação (event_id/welcome_sent_at).
- Sessão ausente força retry; falha de LLM não conclui/corrompe nem chama `CompleteOnboardingSession`.
- **DR-10**: `max_attempts` para `onboarding.subscription_bound`; após o teto, dead-letter + alerta
  `outbox_dead_letter_total{event_type}` (evita retry infinito quando a sessão nunca é criada).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `mastra` — consumer/Run no `internal/agent`, idempotência por AgentDecision e ciclo Thread→Run.

go-implementation (linguagem, auto) e agent-governance (governança, auto) também se aplicam.

## Testes da Tarefa

- [ ] Testes unitários (suite testify; replay; InProgress=false→erro; envio→MarkWelcomeSent; LLM erro não persiste)
- [ ] Testes de integração (T12 — idempotência do greeting end-to-end)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Definition of Done (DoD)

- [ ] Consumer usa event_id; retry em sessão ausente; welcome marcado após envio.
- [ ] Erro de LLM comprovadamente não persiste transição nem conclui.
- [ ] Zero comentários no `.go`; sem SQL direto no consumer.
- [ ] `go build ./internal/agent/...` e `go test ./internal/agent/infrastructure/messaging/... -run OnboardingBound` passam.

## Critérios de Aceite (validações executáveis)

```bash
go build ./internal/agent/... && \
go test ./internal/agent/infrastructure/messaging/database/consumers/... -run OnboardingBound -count=1
```

## Arquivos Relevantes
- `internal/agent/infrastructure/messaging/database/consumers/onboarding_bound_consumer.go` (modificado)
- `internal/agent/application/usecases/run_onboarding_turn.go` (garantia de não-persistência em erro de LLM)
