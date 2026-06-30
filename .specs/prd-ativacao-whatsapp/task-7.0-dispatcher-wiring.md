# Tarefa 7.0: Dispatcher event-driven e wiring

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Tornar o dispatcher event-driven para ativação: remover o ramo legado `ATIVAR`, e no ramo de usuário não vinculado publicar `onboarding.activation.attempted.v1` via callback injetado. Wirar o callback no servidor.

<requirements>
- RF-25: integração de produção do inbound com o consumo (não descartar o resultado).
- RF-29: remover o ramo `MatchActivationCommand` (`ATIVAR <token>`) do dispatcher.
- RF-18: o backend constrói o link com mensagem "Oi" (validar que o dispatcher não reintroduz código).
- Dispatcher permanece genérico (R-ADAPTER-001): apenas invoca o callback, simétrico a `agentRoute`.
</requirements>

## Subtarefas

- [ ] 7.1 Remover o bloco `channels.MatchActivationCommand` de `internal/platform/whatsapp/dispatcher/dispatcher.go:141`.
- [ ] 7.2 Adicionar o campo/callback `activationRoute func(ctx, msg) RouteOutcome` ao `Dispatcher` e invocá-lo no ramo `ErrUnknownUser`.
- [ ] 7.3 Expor `WhatsAppActivationRoute` em `internal/onboarding/module.go` (closure que publica o evento via `OutboxPublisher`).
- [ ] 7.4 Injetar o callback em `cmd/server/whatsapp_wiring.go` (`dispatcher.New(...)`).
- [ ] 7.5 Remover o consumo de `WA_MSG_PLEASE_USE_ATIVAR_COMMAND`.

## Detalhes de Implementação

Ver techspec.md, "Visão Geral dos Componentes", "Fluxo de Dados" e ADR-001. Manter dedup por WAMID antes do publish. Não injetar domínio do onboarding no dispatcher.

## Critérios de Sucesso

- Número não vinculado dispara `activationRoute` (evento publicado); número vinculado roteia ao agente.
- Ramo `ATIVAR` ausente; dispatcher continua genérico e fino.
- Wiring compila e o evento chega ao `ActivationAttemptConsumer`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unitários do dispatcher (não-vinculado → activationRoute; vinculado → agente; sem ramo ATIVAR).
- [ ] Teste de wiring/bootstrap do servidor (compila e injeta o callback).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/whatsapp/dispatcher/dispatcher.go`
- `internal/onboarding/module.go`
- `cmd/server/whatsapp_wiring.go`
- `internal/platform/channels/activation_command.go` (uso removido)
