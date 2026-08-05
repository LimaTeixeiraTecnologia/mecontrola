# Tarefa 3.0: Template genérico WhatsApp/Meta sem regressão

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Evoluir o contrato de notification para suportar template genérico tipado, preservando `SendText`, `SendActivationTemplate`, outreach e ativação atuais.

<requirements>
- Cobrir RF-11, RF-12, RF-13, REQ-01, REQ-02, REQ-03, REQ-04, REQ-05 e REQ-06.
- `SendActivationTemplate` deve continuar compatível.
- `TemplateMessage` deve validar campos obrigatórios antes de chamar Meta.
- O gateway WhatsApp deve delegar para `meta.Client.SendTemplate`.
- Todos os stubs/fakes/mocks que implementam `ChannelGateway` devem ser atualizados.
</requirements>

## Subtarefas

- [ ] 3.1 Adicionar `TemplateMessage` tipado em `internal/platform/notification`.
- [ ] 3.2 Adicionar `SendTemplate` preservando métodos existentes.
- [ ] 3.3 Atualizar adapter WhatsApp e bootstrap.
- [ ] 3.4 Implementar tradução para `components` Meta no gateway concreto.
- [ ] 3.5 Atualizar fakes/stubs/mocks e testes de regressão.

## Detalhes de Implementação

Referenciar `techspec.md` seções `Interfaces Chave` e `Pontos de Integração`.

## Critérios de Sucesso

- Template genérico envia payload Meta esperado em teste com `httptest`.
- Ativação atual continua renderizando `ATIVAR <token>`.
- Texto livre existente continua funcionando.
- Nenhuma regra financeira entra no adapter.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `design-patterns-mandatory` — confirma a evolução direta de contrato/adapter sem pattern formal.

## Testes da Tarefa

- [ ] `go test -race -count=1 ./internal/platform/notification/...`
- [ ] `go test -race -count=1 ./internal/onboarding/infrastructure/gateway/...`
- [ ] `go test -race -count=1 ./internal/onboarding/infrastructure/http/client/meta/...`
- [ ] `go vet ./internal/platform/notification/... ./internal/onboarding/infrastructure/gateway/... ./internal/onboarding/infrastructure/http/client/meta/...`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/notification/channel.go`
- `internal/platform/notification/channel_test.go`
- `internal/platform/notification/adapters/whatsapp.go`
- `internal/platform/notification/adapters/adapters_test.go`
- `internal/bootstrap/channel.go`
- `internal/onboarding/infrastructure/gateway/whatsapp_gateway.go`
- `internal/onboarding/infrastructure/gateway/whatsapp_gateway_test.go`
- `internal/onboarding/infrastructure/http/client/meta/client.go`
- `internal/onboarding/infrastructure/http/client/meta/client_test.go`
