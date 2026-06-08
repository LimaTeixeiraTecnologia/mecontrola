# Tarefa 8.0: Wiring de modulo, server, worker, eventos e configuracao

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Integrar o modulo onboarding ao bootstrap real da aplicacao, configuracao, routers HTTP, dispatcher de eventos, worker manager e projector de entitlement apos bind.

<requirements>
- Cobrir integracao operacional de `RF-03`, `RF-09`, `RF-11`, `RF-17`.
- Criar `NewOnboardingModule(...) OnboardingModule` com DI manual explicita, sem options pattern.
- Registrar routers publicos e WhatsApp em `cmd/server/server.go`.
- Registrar consumers `billing.subscription.activated`, `billing.subscription.activated_without_token` e jobs em `cmd/worker/worker.go`.
- Registrar reprojecao de `onboarding.subscription_bound` em identity conforme ADR-011.
- Adicionar `OnboardingConfig` e `WhatsAppConfig` sem secrets hardcoded.
- A execucao posterior deve carregar obrigatoriamente `go-implementation`, carregar exemplos apenas sob demanda, verificar `go.mod` antes de usar recursos da linguagem, partir de `cmd/server/server.go` e/ou `cmd/worker/worker.go`, nao usar `internal/platform/runtime` como ponto de partida e nao adicionar comentarios em arquivos Go.
</requirements>

## Subtarefas

- [ ] 8.1 Implementar `internal/onboarding/module.go` com campos reais e construtor direto.
- [ ] 8.2 Adicionar configs e `.env.example` para onboarding e WhatsApp.
- [ ] 8.3 Registrar routers em `cmd/server/server.go`.
- [ ] 8.4 Registrar consumers e jobs em `cmd/worker/worker.go`.
- [ ] 8.5 Implementar e registrar `SubscriptionBoundProjector` em identity.
- [ ] 8.6 Validar build/vet dos entrypoints e evitar wiring de artefatos inexistentes.

## Detalhes de Implementação

Referenciar `techspec.md` secoes 6.1, 6.2, 6.3, 6.4, 6.9, 6.11 e "Sequenciamento de Desenvolvimento". Verificar os entrypoints reais antes de editar.

## Critérios de Sucesso

- `OnboardingModule` expoe apenas routers, consumers e jobs reais.
- Server registra rotas sem depender de pacote ausente.
- Worker registra eventos e jobs pelos adapters de `internal/platform/worker`.
- Identity consegue reprojetar entitlement a partir de `onboarding.subscription_bound`.
- Configs tem defaults seguros e falham cedo quando segredo obrigatorio faltar em ambiente habilitado.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes de `module.go` quando padrao local permitir.
- [ ] Testes de config parsing.
- [ ] `go test -race -count=1 ./internal/onboarding/... ./internal/identity/...`
- [ ] `go build ./cmd/server ./cmd/worker`
- [ ] `go vet ./cmd/server ./cmd/worker`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/onboarding/module.go`
- `cmd/server/server.go`
- `cmd/worker/worker.go`
- `configs/config.go`
- `.env.example`
- `internal/identity/`
