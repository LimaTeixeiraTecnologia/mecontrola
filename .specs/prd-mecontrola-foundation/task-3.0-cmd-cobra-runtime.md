# Tarefa 3.0: `cmd/` cobra root + subcomandos `server`/`worker`/`migrate` + `internal/infrastructure/runtime`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Materializar o **entrypoint CLI** do binário `mecontrola` via `spf13/cobra` v1.10.2 seguindo o pattern de `JailtonJunior94/financial/cmd/main.go` (ADR-010), e o `internal/infrastructure/runtime` que provê o `AppMode` VO + `Bootstrap(cfg, mode)` que injeta `configs.Config` em todos os subsistemas. Cobre **RF-02** (subcomandos cobra) e **RF-03** (subcomando `migrate`).

<requirements>
- `cmd/main.go` = root cobra (`mecontrola`) sem lógica de negócio (registra subcomandos e executa).
- `cmd/server/cmd.go` expõe `func New() *cobra.Command` que sobe HTTP + scheduler placeholder via `runtime.Bootstrap(cfg, runtime.ModeServer)`.
- `cmd/worker/cmd.go` análogo com `runtime.ModeWorker` (runtime idle até PRDs futuros registrarem jobs).
- `cmd/migrate/cmd.go` chama `database.RunMigrations(ctx, manager)` e termina (sem subir HTTP).
- `internal/infrastructure/runtime/{app.go,bootstrap.go,mode.go,*_test.go}` com VO `AppMode` ∈ {server, worker}, interface `App { Run(ctx) error; Shutdown(ctx) error }`, `Bootstrap(cfg *configs.Config, mode AppMode) (App, error)`.
- **Sem `APP_MODE` env e sem flag `--migrate-only`** — proibidos (drop da v6).
- Lib: `github.com/spf13/cobra@v1.10.2`.
- `cmd/` excluído da cobertura (D-22); validado por integration test em 10.0.
- `runtime` testado unitariamente (table-driven do `AppMode`; mock de subsistemas para `Bootstrap`).
</requirements>

## Subtarefas

- [ ] 3.1 `go get github.com/spf13/cobra@v1.10.2`.
- [ ] 3.2 Criar `internal/infrastructure/runtime/mode.go` com VO `AppMode` (`ModeServer`, `ModeWorker`); função `ParseAppMode(s string) (AppMode, error)` table-driven.
- [ ] 3.3 Criar `internal/infrastructure/runtime/app.go` com interface `App` + struct concreta com slice de `Subsystem` interno; `Run` inicia cada subsistema; `Shutdown` para na ordem inversa via `Shutdowner` do devkit-go.
- [ ] 3.4 Criar `internal/infrastructure/runtime/bootstrap.go` com `Bootstrap(cfg *configs.Config, mode AppMode) (App, error)` — instancia stub para subsistemas (concretizados em 4.0–7.0; aqui usa interfaces para permitir injeção em teste).
- [ ] 3.5 Criar `internal/infrastructure/runtime/runtime_test.go` (table-driven `ParseAppMode`; mock de `Subsystem` para `Bootstrap` + `App.Run`).
- [ ] 3.6 Criar `cmd/server/cmd.go` com `func New() *cobra.Command` cujo `RunE` chama `configs.LoadConfig(".") → runtime.Bootstrap(cfg, runtime.ModeServer) → app.Run(cmd.Context())`.
- [ ] 3.7 Criar `cmd/worker/cmd.go` análogo com `ModeWorker`.
- [ ] 3.8 Criar `cmd/migrate/cmd.go` que chama `configs.LoadConfig(".") → database.NewManager(cfg)` (interface temporária; concretizada em 5.0) → `database.RunMigrations(ctx, m)`.
- [ ] 3.9 Criar `cmd/main.go` com root cobra `mecontrola` + `root.AddCommand(server.New(), worker.New(), migrate.New())` + `root.Execute()`.

## Detalhes de Implementação

Ver techspec §"Interfaces Chave" (sketches) + ADR-010 §Decisão.

**Pendência cross-task**: `database.NewManager` e `database.RunMigrations` são definidos em 5.0; nesta task usar **interface placeholder** em `internal/infrastructure/runtime` para destacar a dependência sem bloquear (mock no teste; implementação concreta vinculada em 5.0).

## Critérios de Sucesso

- `go build ./cmd/...` compila e produz `mecontrola`.
- `./mecontrola --help` lista exatamente 3 subcomandos: `server`, `worker`, `migrate`.
- `./mecontrola server --help`, `./mecontrola worker --help`, `./mecontrola migrate --help` respondem sem erro.
- `./mecontrola server` rejeita ausência de `.env` em dev com erro explícito (re-validado em integration test 10.0).
- `go test ./internal/infrastructure/runtime/...` ≥ 95% cobertura.
- Cobre RF-02, RF-03, RF-09 parcial (`cmd/`).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: `runtime_test.go` table-driven `ParseAppMode` (válido/inválido); `Bootstrap` injeção (mock subsistemas) — assert ordem de start/shutdown.
- [ ] Testes de integração: deferidos para tarefa 10.0 (`cmd_integration_test.go`).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `cmd/main.go`
- `cmd/server/cmd.go`
- `cmd/worker/cmd.go`
- `cmd/migrate/cmd.go`
- `internal/infrastructure/runtime/{mode.go,app.go,bootstrap.go,runtime_test.go}`
- `go.mod`, `go.sum`
- `.golangci.yml` (`cmd/` excluído da cobertura gate, mantido em lint normal)
