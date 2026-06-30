# Tarefa 6.0: Consumers de ativação e boas-vindas + evento

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar o evento `onboarding.activation.attempted.v1`, o `ActivationAttemptConsumer` (adapter fino que delega ao usecase 5.0) e o `WelcomeConsumer` desacoplado que entrega as duas mensagens de boas-vindas ao consumir `onboarding.subscription_bound`.

<requirements>
- RF-25: inbound integrado ao consumo (não mais descartado).
- RF-27/RF-28: boas-vindas + apresentação do assistente com convite "Vamos começar?".
- RF-32/RF-33: duas mensagens de texto livre (`welcome_activated` + `onboarding_intro`) dentro da janela de 24h; sem botão interativo.
- RF-34: jornada encerra na boas-vindas.
- Evento sem usuário em `noUserEventAllowlist`; consumers finos (R-ADAPTER-001), idempotentes.
</requirements>

## Subtarefas

- [ ] 6.1 Definir o evento `onboarding.activation.attempted.v1` (payload `{peer_e164, text, message_id}`) e adicioná-lo a `internal/platform/outbox/system_event_allowlist.go` (`noUserEventAllowlist`).
- [ ] 6.2 Criar `infrastructure/messaging/database/consumers/activation_attempt_consumer.go` (delegando a `ActivateFromInbound`).
- [ ] 6.3 Criar `infrastructure/messaging/database/consumers/welcome_consumer.go` consumindo `onboarding.subscription_bound` (`peer_e164`+`user_id`), idempotente por event id, enviando as 2 mensagens via `SendTextMessage`.
- [ ] 6.4 Registrar ambos em `internal/onboarding/module.go` `EventHandlers` e expor o que for necessário ao wiring.

## Detalhes de Implementação

Ver techspec.md, "Visão Geral dos Componentes", "Fluxo de Dados" e ADR-001 (decisões 2 e 3). Espelhar `whatsapp_inbound_consumer.go` (adapter fino) e o padrão `EventHandlerRegistration`.

## Critérios de Sucesso

- `ActivationAttemptConsumer` processa o evento e delega ao usecase; reentrega não duplica efeito.
- `WelcomeConsumer` entrega as 2 mensagens; reentrega não reenvia (idempotência).
- Evento aceito pelo outbox (presente na allowlist sem usuário).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unitários dos dois consumers (payload inválido, sucesso, idempotência).
- [ ] Integração (testcontainers) do `ActivationAttemptConsumer` ponta-a-ponta (evento → token CONSUMED → bound).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/outbox/system_event_allowlist.go`
- `internal/onboarding/infrastructure/messaging/database/consumers/activation_attempt_consumer.go` (novo)
- `internal/onboarding/infrastructure/messaging/database/consumers/welcome_consumer.go` (novo)
- `internal/onboarding/module.go`
