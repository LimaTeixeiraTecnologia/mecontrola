# Tarefa 2.0: Eliminação do canal Telegram (código + config)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Remover integralmente o canal Telegram do código e da configuração, deixando o WhatsApp Oficial da
Meta como canal único. Cobre deleção de arquivos Telegram-only, edição de arquivos compartilhados
(remover ramo Telegram preservando WhatsApp) e remoção de config/env. O schema fica para a Tarefa 3.0.

<requirements>
- RF-01: WhatsApp Oficial da Meta como único canal conversacional (ingress webhook + egress Graph API preservados).
- RF-02: eliminar a árvore `internal/platform/telegram`, consumer inbound do agent, wiring, onboarding-Telegram, adapter de notificação e webhook router Telegram.
- RF-03: remover configuração, env e defaults de Telegram (`TELEGRAM_*`, `ONBOARDING_TELEGRAM_*`, validações de produção).
- RF-04: remover Telegram dos tipos fechados/VOs de canal (Channel VO, `SourceTelegram`, resolução de canal preferido).
- RF-06: resolução de identidade continua derivando user_id por E164; usuário não encontrado roteia para onboarding/ativação.
</requirements>

## Subtarefas

- [ ] 2.1 Editar arquivos compartilhados (LISTA 2 do ADR-005): `intent_router.go`, `channel.go`/`external_id.go`, `resolve_preferred_channel.go`, `principal.go`, `notification/channel.go`, `magic_token.go` (+cascata repo/DTO/handler do `telegram_external_id`), `send_outreach.go`, `onboarding_session.go`/`start_budget_configuration.go`, `budget_configurator.go` (`mapAgentChannelToOnboarding`), `inbound_event_publisher.go`, `agent/module.go`, `onboarding/module.go`, `internal/bootstrap/channel.go`, `cmd/server/server.go`, `cmd/worker/worker.go`, `configs/config.go`, `.env.example`, `platform/channels/activation_command.go`, OpenAPIs.
- [ ] 2.2 Deletar arquivos Telegram-only (LISTA 1 do ADR-005): árvore `internal/platform/telegram/**`, `telegram_inbound_consumer.go`, onboarding-Telegram, `notification/adapters/telegram.go`, `identity/.../telegram_router.go`, `cmd/server/telegram_wiring.go`, `configs/validate_production_telegram_agent_test.go`, scripts/diagramas Telegram-only.
- [ ] 2.3 Ajustar/remover testes que referenciam Telegram (lista no ADR-005), preservando casos WhatsApp.
- [ ] 2.4 NÃO tocar `ALERT_TELEGRAM_*` / alerting do Grafana (decisão: manter — fora de escopo).
- [ ] 2.5 `go build ./...` + `go test ./...` verdes; grep case-insensitive `telegram` em `internal/`/`cmd/`/`configs/` retorna só refs de observabilidade (categoria B) ou nada.

## Detalhes de Implementação

Ver `adr-005-eliminate-telegram.md` (LISTA 1, LISTA 2) e techspec §"Eliminação Telegram". Não duplicar.
Ordem: editar shared → deletar Telegram-only → ajustar testes → build/test.

## Critérios de Sucesso

- Build/test verdes; zero referência ao canal de produto Telegram (exceto alerting Grafana).
- WhatsApp ingress/egress e resolução por E164 inalterados.
- Gate de fronteira de dados (1.0) continua verde.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `mastra` — toca `intent_router`, `module.go`, consumer do agent e VOs de canal do `internal/agent` (R-AGENT-WF-001).

## Testes da Tarefa

- [ ] Testes unitários (Channel VO whatsapp-only; intent_router WhatsApp; send_outreach WhatsApp).
- [ ] Testes de integração (build completo; wiring do server/worker sem Telegram).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- Ver LISTA 1 e LISTA 2 em `adr-005-eliminate-telegram.md` (caminhos exaustivos).
- `internal/agent/application/services/intent_router.go`, `internal/agent/module.go`
- `internal/identity/domain/valueobjects/{channel,external_id}.go`
- `internal/onboarding/module.go`, `configs/config.go`, `.env.example`
