# Tarefa 5.0: Wiring do limite diário no módulo e entrypoints

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Ligar o gate de limite diário de ponta a ponta: novos campos em `agents.Deps`, construção do repositório e do use case em `internal/agents/module.go`, registro da option no consumer e repasse das configurações nos dois entrypoints (`cmd/server` e `cmd/worker`), garantindo comportamento idêntico nos dois processos.

<requirements>
- RF-08: limite configurável por variável de ambiente com default 30 e desativação por valor zero, efetivo em ambos os processos que sobem o consumer.
</requirements>

## Subtarefas

- [ ] 5.1 Adicionar `DailyInteractionLimit int` e `DailyLimitReply string` na struct `Deps` (`internal/agents/module.go:99-114`), padrão dos campos diretos `InboundTimeout` e `AgentMaxTokens`
- [ ] 5.2 Construir `dailyInteractionCounter` via `postgres.NewDailyInteractionCounter(deps.DB, deps.O11y)` e `resolveDailyLimit := usecases.NewResolveDailyInteractionLimit(counter, brazilLoc, deps.DailyInteractionLimit, deps.DailyLimitReply, deps.O11y)` reutilizando o `brazilLoc` de `module.go:276-279`
- [ ] 5.3 Registrar `WithDailyLimitResolver(resolveDailyLimit)` no bloco de options do consumer (`module.go:415-439`)
- [ ] 5.4 Repassar as duas configurações em `cmd/server/server.go` (padrão do repasse de áudio em `server.go:256-257`) e em `cmd/worker/worker.go` (bloco de wiring de agents por volta de `worker.go:432`)
- [ ] 5.5 Atualizar o teste de boot do módulo (`internal/agents/module_boot_integration_test.go`) se ele montar `Deps`, mantendo todos os cenários existentes verdes

## Detalhes de Implementação

- Seção `Visão Geral dos Componentes` da techspec.md lista os componentes modificados e o fluxo de dados final.
- Seção `Dependências Técnicas` da techspec.md confirma que não há infraestrutura nova: apenas `deps.DB`, `deps.O11y` e `brazilLoc` já existentes.
- DI manual explícita, sem `init()`, sem functional options no módulo, seguindo o padrão de módulo do AGENTS.md.

## Critérios de Sucesso

- Os dois entrypoints constroem o consumer com o resolvedor configurado; ausência de repasse em um deles é detectada por compilação (campos obrigatórios de `Deps`) ou por teste de boot.
- Com `AGENT_DAILY_INTERACTION_LIMIT=0` o consumer sobe e o gate responde permitido sem consultar o banco.
- `go build ./...`, `go vet ./...`, `go test -race -count=1 ./internal/agents/... ./cmd/...` verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: não aplicável além dos existentes no módulo
- [ ] Testes de integração: boot do módulo com o resolvedor ligado, no padrão de `module_boot_integration_test.go`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/module.go`
- `cmd/server/server.go`
- `cmd/worker/worker.go`
- `internal/agents/module_boot_integration_test.go`
