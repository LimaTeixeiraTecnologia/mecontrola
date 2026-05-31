# ADR-010 — `spf13/cobra` v1.10.2 + binário único com subcomandos `server`/`worker`/`migrate`

## Metadados

- **Título:** Adoção de `spf13/cobra` para expor binário único `mecontrola` com subcomandos independentes
- **Data:** 2026-05-31
- **Status:** Aceita
- **Decisores:** @JailtonJunior94
- **Relacionados:** [PRD §RF-02, §RF-03, §D-19, §CS-21, §CS-22](./prd.md), [techspec §Arquitetura, §Interfaces Chave](./techspec.md), [ADR-009 (Viper)](./adr-009-viper-configs-validate.md), [Referência: financial/cmd/main.go](https://github.com/JailtonJunior94/financial/blob/main/cmd/main.go)

## Contexto

A versão v6 da techspec adotava **binário único `cmd/server`** com seleção de subsistema via env var `APP_MODE ∈ {server, worker, all}` + flag `--migrate-only` para executar migrations. Durante revisão, o tech lead apontou que o pattern adotado em outros projetos da org (`JailtonJunior94/financial/cmd/main.go`) usa **`spf13/cobra` com subcomandos**, separando responsabilidades por comando próprio (`api`, `consumer`, `worker`, `migrate`) em vez de switch interno via env.

Requisitos derivados da nova orientação:
- Cada modo é um subcomando próprio com seu `Run()`, isolado em `cmd/<subcmd>/`.
- O root command (`mecontrola`) lista os subcomandos disponíveis.
- Adição de subcomandos futuros (e.g. `consumer` quando PRD de mensageria entrar) não exige refactor do core.
- Migrations viram subcomando `mecontrola migrate`, dispensando flag `--migrate-only`.

## Decisão

- **Lib:** `github.com/spf13/cobra` **v1.10.2** (última estável, publicada 2025-12-04). Pinada no `go.mod`.
- **Layout:**
  - `cmd/main.go`: root cobra (`mecontrola`); apenas registra subcomandos e chama `root.Execute()`.
  - `cmd/server/cmd.go`: expõe `func New() *cobra.Command` para `mecontrola server` — sobe HTTP + scheduler in-process placeholder via `runtime.Bootstrap(cfg, runtime.ModeServer)`.
  - `cmd/worker/cmd.go`: análogo para `mecontrola worker` — sobe runtime worker idle (sem jobs registrados na foundation; placeholders até PRDs subsequentes).
  - `cmd/migrate/cmd.go`: `mecontrola migrate` — chama `database.RunMigrations(ctx, manager)` via `golang-migrate` e termina.
- **Sem `APP_MODE` env e sem flag `--migrate-only`**: removidos da v7 do PRD; cada modo é um subcomando independente.
- **AppMode interno**: `runtime.AppMode` continua existindo como VO interno (`ModeServer` | `ModeWorker`) para que `runtime.Bootstrap` saiba quais subsistemas iniciar; é detalhe de implementação, não interface CLI.
- **Cobertura de `cmd/`**: excluída do gate de cobertura (D-11 + D-22); validação via `cmd_integration_test.go` que compila e executa o binário real.
- **Naming**: subcomandos em **inglês** (`server`, `worker`, `migrate`), curtos, kebab-case se necessário no futuro (`run-migrations` evitado em favor de `migrate`).

## Alternativas Consideradas

1. **Manter `APP_MODE` env + `--migrate-only` flag (v6 anterior)**.
   - Vantagens: zero dependência adicional; `os.Args` mínimo.
   - Desvantagens: viola pattern da org; switch interno em `main()` confunde leitura; novos modos exigem mexer no `main` em vez de adicionar arquivo isolado.
2. **Binários separados (`cmd/server/main.go`, `cmd/worker/main.go`, `cmd/migrate/main.go`)**.
   - Vantagens: máxima separação; imagens menores por binário.
   - Desvantagens: 3 binários no Fly = 3 imagens vs 1 imagem com subcomandos; `Dockerfile` mais complexo; pode aumentar tempo de build/release.
3. **`urfave/cli` em vez de cobra**.
   - Vantagens: API mais minimalista.
   - Desvantagens: divergência com o pattern da org; menos ecossistema (cobra é dominante em Go CLI).

## Consequências

### Benefícios Esperados

- Pattern repetível entre projetos da org (financial é referência).
- Novos modos = novo arquivo isolado em `cmd/<subcmd>/cmd.go` (open/closed).
- `mecontrola --help` documenta a CLI sem `man page` adicional.
- Cada subcomando tem seu próprio `--help`, flags, `--config`, etc., sem poluir os outros.
- Migration vira subcomando explícito — runbook é `mecontrola migrate`, não `mecontrola --migrate-only`.

### Trade-offs e Custos

- +1 dependência (`spf13/cobra`); leve (~150 KB no binário).
- Reorganização do `cmd/` em sub-pacotes (overhead pequeno; ganho compensa).
- Convenção de naming a documentar no README.

### Riscos e Mitigações

- **Risco:** cobertura excluída de `cmd/` esconde bug de wiring entre subcomando e `runtime.Bootstrap`.
  - **Mitigação:** `cmd_integration_test.go` compila o binário e exercita todos os subcomandos com testcontainers; gate obrigatório no CI.
- **Risco:** subcomando duplica setup (carregamento de config, logger, etc.).
  - **Mitigação:** factory `runtime.Bootstrap(cfg, mode)` concentra inicialização; cada subcomando chama uma única função.
- **Risco:** Dev cria subcomando novo sem registrar no root.
  - **Mitigação:** convenção verificada por integration test (`mecontrola --help` deve listar exatamente os subcomandos esperados); fail no test.

## Plano de Implementação

1. `go get github.com/spf13/cobra@v1.10.2`.
2. `cmd/main.go`: root `&cobra.Command{Use: "mecontrola", Short: "MeControla CLI"}` + `root.AddCommand(server.New(), worker.New(), migrate.New())` + `root.Execute()`.
3. `cmd/server/cmd.go`: `func New() *cobra.Command` com `RunE` que faz `configs.LoadConfig(".") → runtime.Bootstrap(cfg, runtime.ModeServer) → app.Run(cmd.Context())`.
4. `cmd/worker/cmd.go`: idem com `runtime.ModeWorker`.
5. `cmd/migrate/cmd.go`: `configs.LoadConfig(".") → database.NewManager(cfg) → database.RunMigrations(ctx, m) → log da versão final`.
6. `cmd_integration_test.go`: compila binário (`go build -o /tmp/mecontrola ./cmd`) + executa `--help` para cada subcomando + executa `migrate` com testcontainers Postgres + valida exit codes (CS-21 + CS-22).
7. Atualizar `Dockerfile` (a criar no marco M5): `CMD ["./mecontrola", "server"]` como default.
8. Atualizar `fly.toml` (a criar no marco M5): `processes` com `app = "/app/mecontrola server"` e (futuro) `worker = "/app/mecontrola worker"`.
9. README: seção "Comandos" com `mecontrola --help`, `mecontrola server`, `mecontrola worker`, `mecontrola migrate`.

## Monitoramento e Validação

- Métrica `bootstrap_duration_seconds{subcommand}` (label custom em cima do existente).
- Log estruturado no startup informa subcomando ativo + versão + commit SHA.
- CI valida que `mecontrola --help` lista exatamente 3 subcomandos (regressão).

## Impacto em Documentação e Operação

- README atualizado.
- Runbook "Deploy via Fly": `fly deploy` segue; processo default é `mecontrola server`.
- Runbook "Aplicar migration em prod": `fly ssh console -C "/app/mecontrola migrate"`.
- Runbook "Subir worker dedicado" (quando Epic 09 entrar): adicionar processo `worker` no `fly.toml`.

## Revisão Futura

- Revisitar quando subcomandos cruzarem 8 (sinal de erosão; considerar separar em apps Fly distintos).
- Revisitar adicionar flags globais (e.g. `--config <path>`) quando demanda surgir.
- Revisitar para subcomando `admin` (export DSAR etc.) quando Epic 11 (LGPD operacional) entrar.
