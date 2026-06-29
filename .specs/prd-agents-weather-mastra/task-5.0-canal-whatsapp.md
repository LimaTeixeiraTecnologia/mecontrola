# Tarefa 5.0: Canal WhatsApp (inbound → AgentRuntime → outbound)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Ligar `internal/agents` ao WhatsApp: rota inbound (adapter fino que publica evento de outbox), consumer no worker que chama `HandleInbound`, e resposta enviada pelo gateway WhatsApp existente.

<requirements>
- RF-20: mensagem WhatsApp roteada para `internal/agents`; texto livre vira entrada do agente; resposta enviada ao usuário.
- RF-21: adapters finos (handler/consumer → usecase), reutilizando dedup/assinatura/principal/rate limit existentes.
- RF-22: Run auditável por interação em `platform_runs`; tracing correlacionável; métricas de cardinalidade controlada.
- ADR-003 (mapeamento canal→AgentRuntime; agente direto é o caminho primário).
</requirements>

## Subtarefas

- [ ] 5.1 Rota WhatsApp (`WhatsAppAgentRoute`): adapter fino que publica evento outbox `agents.whatsapp.inbound.v1` (`{user_id, peer, text, message_id}`).
- [ ] 5.2 Consumer no worker: decodifica o evento e chama `HandleInbound` → `AgentRuntime.Execute`; idempotente; expõe `EventHandlers`.
- [ ] 5.3 Resposta outbound via gateway existente (`SendTextMessage(toE164, reply)`).
- [ ] 5.4 Métricas/labels enums (channel/outcome); sem `resource_id`/`thread_id` como label.

## Detalhes de Implementação

Ver techspec.md §"Fluxo de dados (WhatsApp → resposta)", §"Pontos de Integração" (WhatsApp), ADR-003. Reutilizar `internal/platform/whatsapp` (handler/dispatcher) e o gateway de envio existente.

## Critérios de Sucesso

- Inbound de texto ("clima em <cidade>?") produz resposta enviada ao usuário; Run auditável persistido.
- Adapters finos (sem SQL/regra de negócio); consumer idempotente; zero comentários; gofmt limpo.

## Skills Necessárias

<!-- MANDATÓRIO -->

- `mastra` — o mapeamento canal→Thread/Run e a execução via AgentRuntime seguem o modelo Mastra.

## Testes da Tarefa

- [ ] Testes unitários (rota publica evento; consumer chama usecase; adapter fino)
- [ ] Testes de integração (E2E inbound determinístico: mensagem → resposta + persistência; com 7.0)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/infrastructure/messaging/database/consumers/*`, rota em `module.go`; `internal/platform/whatsapp/*` (reuso); gateway de envio existente.
