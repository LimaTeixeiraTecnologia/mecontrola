# Tarefa 8.0: whatsapp.dispatcher.Dispatcher + agent stub + integração end-to-end

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementa o `Dispatcher` que orquestra dedup WAMID + parse + roteamento ATIVAR/normal + resolução de Principal + rate-limit + agent stub. Cria `internal/agent` com `AgentHandler` interface + `StubAgent` que envia template Meta "MeControla recebeu sua mensagem" via `WhatsAppGateway` reusado do onboarding. Publica `auth.failed{reason}` para os caminhos de falha (rate_limited, invalid_payload, invalid_country).

<requirements>
- RF-06: `Dispatcher.Route(ctx, msg) (RouteOutcome, error)` aplica regex ATIVAR, chama `EstablishPrincipal`, aplica rate-limit, roteia para onboarding/agent/fallback/rate_limited/duplicate/invalid.
- RF-35: `internal/agent/stub.go` envia template "MeControla recebeu sua mensagem — estamos preparando sua experiência" via WhatsAppGateway reusado; emite log INFO e métrica `whatsapp_dispatcher_route_total{outcome="agent_stub"}`.
</requirements>

## Subtarefas

- [ ] 8.1 Criar `internal/agent/agent.go` com `type AgentHandler interface { HandleMessage(ctx context.Context, msg payload.Message) error }`.
- [ ] 8.2 Criar `internal/agent/stub.go` implementando `AgentHandler` — chama `waGateway.SendText(ctx, msg.From, templates["agent_stub_received"])`; emite log INFO `agent_stub_invoked` com `trace_id`, `user_id`, `wa_id_masked`; emite métrica.
- [ ] 8.3 Criar `internal/agent/stub_test.go` cobrindo: (a) Principal presente → SendText chamado com template correto; (b) gateway falha → erro propagado + log WARN.
- [ ] 8.4 Criar `internal/platform/whatsapp/dispatcher/dispatcher.go` com tipo `Dispatcher` (campos: `dedup`, `parser`, `establish`, `limiter`, `publisher`, `onboardingRoute`, `agentRoute`, `o11y`).
- [ ] 8.5 Implementar `Route(ctx, raw)`: parse → dedup → regex ATIVAR → onboarding OU EstablishPrincipal → Limiter.Allow → agent OR fallback. Cada caminho de falha publica `auth.failed{reason}` via `outbox.Publisher`. WAMID duplicado: só métrica `outcome=duplicate`, sem outbox.
- [ ] 8.6 Implementar publicação de `auth.failed{reason='invalid_payload'}` quando parser falhar após HMAC válido (RF-33).
- [ ] 8.7 Criar `dispatcher_test.go` com mocks: 6 outcomes (onboarding, agent, fallback, rate_limited, duplicate, invalid).
- [ ] 8.8 Criar `dispatcher_integration_test.go`: HMAC válido + payload válido + user ativo → linha em `auth_events` + agent stub chamado; HMAC válido + payload corrupto → `auth.failed{invalid_payload}`; rate-limit excedido → `auth.failed{rate_limited}` + 200 OK; WAMID duplicado → outcome `duplicate` + nenhum evento adicional.
- [ ] 8.9 Atualizar `internal/identity/module.go` para registrar `Dispatcher` e `StubAgent`.

## Detalhes de Implementação

Ver techspec `## Design de Implementação > Interfaces Chave > Dispatcher` para esqueleto e `## Convenção de outbox.Event.Type` para nomeação de eventos. Ver techspec `## Stub do agent (RF-35)` para implementação exata do stub.

## Critérios de Sucesso

- 6 `RouteOutcome` cobertos por testes unitários.
- Integration test verde para cada outcome.
- Métrica `whatsapp_dispatcher_route_total{outcome}` reflete cada caminho.
- WAMID duplicado não gera evento outbox nem linha em `auth_events` (silencioso por design).
- Agent stub envia template real via WhatsAppGateway em integration test (com gateway mockado para não chamar Meta).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários do dispatcher (6 outcomes) e do stub
- [ ] Integration test end-to-end via webhook + dedup + outbox + auth_events

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agent/agent.go` (criar — interface)
- `internal/agent/stub.go` + `_test.go` (criar)
- `internal/platform/whatsapp/dispatcher/dispatcher.go` + `_test.go` + `_integration_test.go` (criar)
- `internal/identity/module.go` (atualizar — wiring)
