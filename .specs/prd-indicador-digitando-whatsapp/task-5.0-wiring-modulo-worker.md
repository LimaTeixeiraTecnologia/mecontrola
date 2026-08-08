# Tarefa 5.0: Wiring do módulo agents e do worker com ajuste do stub de boot

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Conectar a cadeia completa: estender a interface local do módulo agents, propagar a flag de configuração por `agents.Deps` até a option do consumer, popular o campo no worker e ajustar o stub do teste de boot. Esta é a tarefa de integração e por isso é sequencial.

<requirements>
- RF-05: a flag chega da configuração ao consumer por DI explícita em construtor.
- RF-06: suíte de boot e integração verde sem alteração de comportamento com flag desligada.
- RF-09: nenhuma interferência em dedup, outbox, persistência ou contagens; o typing não passa pelo outbox.
</requirements>

## Subtarefas

- [ ] 5.1 Estender a interface local `whatsAppGateway` (`internal/agents/module.go:54-56`) com `SendTypingIndicator(ctx context.Context, wamid string) error`.
- [ ] 5.2 Adicionar o campo `TypingIndicatorEnabled bool` em `agents.Deps` e, no wiring do consumer (module.go:415-435), anexar `consumers.WithTypingIndicator(deps.WhatsAppGateway, deps.TypingIndicatorEnabled)` às options quando `deps.WhatsAppGateway` não for nulo.
- [ ] 5.3 Popular o campo em `cmd/worker/worker.go:432-465` com `r.cfg.AgentConfig.WhatsAppTypingIndicatorEnabled`.
- [ ] 5.4 Ajustar o stub de gateway em `internal/agents/module_boot_integration_test.go:29-115` com o método novo retornando nil.
- [ ] 5.5 Adicionar caso de integração em `whatsapp_inbound_consumer_integration_test.go` com flag ligada e sender stub, afirmando uma única emissão com o WAMID correto.
- [ ] 5.6 Confirmar que os testes de boot, integração e e2e existentes passam sem alteração de asserts.

## Detalhes de Implementação

Ver `techspec.md`, seções "Arquitetura do Sistema" e "Sequenciamento de Desenvolvimento", e ADR-002. DI manual explícita no estilo do módulo; proibido introduzir `NewModule(opts...)` ou providers novos.

## Critérios de Sucesso

- `go test -race -count=1 ./internal/agents/...` verde, incluindo boot e integração.
- `go build ./...`, `go vet ./...` e `golangci-lint run` no escopo verdes.
- Diff sem alteração de asserts preexistentes; única mudança em testes antigos é o método novo no stub de boot.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — wiring do consumidor agentivo `internal/agents` no fluxo de WhatsApp inbound, com DI explícita e sem reimplementar primitivos da plataforma.

## Testes da Tarefa

- [ ] Testes unitários (módulo: wiring da option com flag ligada e desligada)
- [ ] Testes de integração (boot do módulo e caso novo do consumer com flag ligada)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/module.go`
- `internal/agents/module_boot_integration_test.go`
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer_integration_test.go`
- `cmd/worker/worker.go`
