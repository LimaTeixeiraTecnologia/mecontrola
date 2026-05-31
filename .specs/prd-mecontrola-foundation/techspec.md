<!-- spec-hash-prd: 9e6ca834f250a525a0e1864992f77741d850f55bd22405fb8e0d5807d2fce7f4 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — MeControla Backend Foundation

PRD consumido: [`.specs/prd-mecontrola-foundation/prd.md`](./prd.md) v9.

Referências governança carregadas: `agent-governance/references/ddd.md` (R-DDD-001), `error-handling.md` (R-ERR-001), `security.md` (R-SEC-001), `security-app.md`, `testing.md` (R-TEST-001), `shared-architecture.md`, `shared-patterns.md`, `object-calisthenics-go/references/rules.md`.

## Resumo Executivo

Foundation production-proof do MeControla materializa um **chassi hexagonal Go 1.26.3** sobre `devkit-go` v0.4.0, com **seis módulos de domínio esqueletados** (`identity`, `conversation`, `agent`, `finance`, `notifications`, `telemetry`) e **cross-cutting em `internal/infrastructure/`** (substitui o histórico `internal/platform/` do discovery). Composição central via construtores explícitos (DI manual, sem container), `manager.Manager` único do `devkit-go/pkg/database` injetado em cada módulo, `UnitOfWork[T]` tipado por agregado para transações, **eventbus in-process tipado via generics Go 1.26** com publicação atômica pós-commit, **erros sentinel por módulo** wrapped com `fmt.Errorf %w` e traduzidos no boundary HTTP para Problem Details RFC 7807 (defaults do `devkit-go/pkg/http_server`), e **validação dual** (`go-playground/validator` no boundary + Value Objects auto-validados no domain). Testes via stdlib + `testify` + `mockery`, integração com `testcontainers-go` (Postgres ephemeral), migrations via `//go:embed`, deploy Fly.io `gru` com OTLP para Grafana Cloud free tier. **Object Calisthenics aplicado como heurística** — não dogma — em revisão automática pela skill `object-calisthenics-go` em PRs.

A foundation não implementa lógica de negócio: cada módulo nasce com `domain/`, `application/`, `adapters/` vazios + um README de scaffolding (pattern de aggregate/entity/VO) que os PRDs subsequentes (Identity, Conversation, etc.) seguirão. O único módulo com código funcional é `internal/infrastructure/` + `cmd/server`.

## Arquitetura do Sistema

### Visão Geral dos Componentes

```
mecontrola/
├── cmd/                                         # binário único `mecontrola` via cobra (D-19; ADR-010)
│   ├── main.go                                  # root cobra; registra server/worker/migrate
│   ├── server/cmd.go                            # `mecontrola server` → HTTP + scheduler placeholder
│   ├── worker/cmd.go                            # `mecontrola worker` → runtime worker idle (placeholder)
│   └── migrate/cmd.go                           # `mecontrola migrate` → golang-migrate up + exit
├── configs/                                     # configuração centralizada (D-17 + D-18; ADR-009)
│   ├── config.go                                # Viper + Config + groups + Validate() + DSN()/SafeDSN()
│   └── config_test.go                           # table-driven; 100% cobertura dos validadores
├── internal/
│   ├── infrastructure/                          # cross-cutting; substitui internal/platform/
│   │   ├── observability/                       # composição devkit-go pkg/observability + OTLP gRPC
│   │   ├── database/                            # manager.Manager central + UoW[T] + migrations embed
│   │   ├── http/                                # composição devkit-go pkg/http_server + middlewares
│   │   ├── events/                              # eventbus tipado: Publish[E], Subscribe[E]
│   │   ├── clock/                               # Clock interface (real + fake p/ testes)
│   │   ├── errors/                              # Problem Details mapper (boundary translator)
│   │   └── runtime/                             # AppMode parser + bootstrap (server/worker/all)
│   ├── identity/        {domain,application,adapters}/   # esqueleto vazio + README scaffold
│   ├── conversation/    {domain,application,adapters}/   # esqueleto vazio + README scaffold
│   ├── agent/           {domain,application,adapters}/   # esqueleto vazio + README scaffold
│   ├── finance/         {domain,application,adapters}/   # esqueleto vazio + README scaffold
│   ├── notifications/   {domain,application,adapters}/   # esqueleto vazio + README scaffold
│   └── telemetry/       {domain,application,adapters}/   # esqueleto vazio + README scaffold
├── migrations/                                  # *.sql; embarcado via //go:embed
├── taskfiles/                                   # automação isolada (skill taskfile-production)
├── Taskfile.yml                                 # orquestrador raiz
├── .env.example                                 # contrato de chaves (D-17 + D-18)
└── .specs/prd-mecontrola-foundation/            # PRD + techspec + ADRs
```

**Componentes novos/modificados** (todos novos; greenfield):
- `cmd/main.go`: root cobra (`mecontrola`); registra subcomandos via `root.AddCommand(server.New(), worker.New(), migrate.New())` e executa. Sem lógica de negócio (D-22: cobertura excluída).
- `cmd/server/cmd.go`: define `func New() *cobra.Command` cujo `Run` chama `configs.LoadConfig(".")` → `runtime.Bootstrap(cfg, runtime.ModeServer)` → `app.Run(ctx)` com shutdown coordenado.
- `cmd/worker/cmd.go`: análogo a `server` mas com `runtime.ModeWorker` (foundation: runtime idle aguardando registro de jobs — placeholder).
- `cmd/migrate/cmd.go`: `configs.LoadConfig(".")` → `database.NewManager(cfg)` → `database.RunMigrations(ctx, m)` → log da versão final → exit.
- `configs/config.go` (ADR-009): pasta raiz `configs/` com Viper v1.21.0; struct `Config` com `mapstructure:",squash"` agrupando `AppConfig`, `HTTPConfig`, `DBConfig`, `O11yConfig`; `LoadConfig(path)` carrega `.env` (obrigatório local) + `AutomaticEnv` (Fly prod) + `SetEnvKeyReplacer(".", "_")`; `Validate()` é gate fail-fast.
- `internal/infrastructure/runtime`: VO `AppMode` (`server`|`worker`); função `Bootstrap(cfg *configs.Config, mode AppMode) (App, error)` retorna stack composta (a flag histórica `all` foi descartada com a adoção de subcomandos — D-19).
- `internal/infrastructure/database`: factory `NewManager(cfg) (*manager.Manager, error)`; helper `RunMigrations(ctx, m)` usando `//go:embed migrations/*.sql`.
- `internal/infrastructure/observability`: factory `NewProvider(cfg) (*obs.Provider, shutdown func(context.Context) error, error)` ligando OTLP gRPC ao Grafana Cloud.
- `internal/infrastructure/http`: factory `NewServer(cfg, deps) (*http.Server, error)` aplicando defaults do devkit-go + CORS estrito + healthz.
- `internal/infrastructure/events`: `Bus` com `Publish[E Event]`, `Subscribe[E Event]`, `Close()`; backpressure por buffer configurável.
- `internal/infrastructure/clock`: interface `Clock { Now() time.Time }` + impl `SystemClock` + `FakeClock` em `_test`.
- `internal/infrastructure/errors`: `ToProblemDetails(err) ProblemDetails` mapeando sentinels conhecidos.
- `internal/<modulo>/domain`: pacote vazio com `doc.go` declarando intenção do módulo + README com o scaffold pattern.

**Fluxo de dados (foundation only)**: `Request HTTP → middleware (RequestID + OTel + recovery + CORS + body-limit + timeout) → handler de health/ready/live → consulta a Manager.Manager (para /ready) → resposta JSON`. Worker mode é placeholder: inicializa scheduler vazio e fica aguardando registração de jobs nos PRDs futuros.

## Modelagem de Domínio

A foundation **não introduz domínios de negócio** — todos ficam fora de escopo do PRD (não-objetivos). Os elementos de modelagem nesta foundation são:

### Value Objects de infraestrutura (`internal/infrastructure/`)

Todos imutáveis, auto-validados via construtor, sem getters mecânicos (Object Calisthenics #3, #9).

| VO | Pacote | Invariante |
| --- | --- | --- |
| `AppMode` | `runtime` | só aceita `server`, `worker`, `all` (enum tipado, parser explícito) |
| `Port` | `configs` (HTTPConfig) | int 1–65535; parsing valida range |
| `Environment` | `configs` (AppConfig) | enum `local`\|`staging`\|`production`; Validate() rejeita outros |
| `DSN` | `configs` (DBConfig) | derivado por `DBConfig.DSN()`; expõe `SafeDSN()` para logs (senha mascarada) |
| `OTLPEndpoint` | `configs` (O11yConfig) | URL válida com scheme `grpc` ou `http` |
| `MigrationVersion` | `database` | uint64 monotônico |
| `RequestID` | `http` (re-export do devkit-go) | UUID v4 ou ULID |
| `EventID` | `events` | ULID gerado por `Clock.Now()` injetado (determinístico em testes) |
| `EventName` | `events` | identificador kebab-case `<modulo>.<acao>` (e.g. `identity.user-created`) |
| `ModuleName` | `events` | enum dos 6 módulos de domínio |
| `HealthStatus` | `http` | `ok` \| `degraded` \| `down`; `String()` para serialização |

### Aggregates e Entidades

**Nenhum agregado de negócio nesta foundation.** A foundation entrega **um aggregate pattern como scaffold** (documentação + interfaces) que cada PRD subsequente seguirá. O contrato canônico é:

```go
// internal/<modulo>/domain/aggregate.go (template)
type AggregateRoot interface {
    ID() AggregateID
    Events() []DomainEvent   // drenado após UoW.Commit; ADR-003
    ClearEvents()
}
```

Não há `struct literal` permitido fora de factories/testes (R-DDD-001 §Proibido).

### Domain Events da foundation

A foundation define somente **a estrutura base** de evento (interface `Event { Name() EventName; OccurredAt() time.Time; AggregateID() string }`) e **não emite nenhum evento real** — a primeira emissão concreta virá no PRD do módulo `conversation` (Epic 06: `MessageReceived`).

## Fronteiras entre Application, Domain e Infrastructure

Enforced por **`depguard` em `.golangci.yml`** (RF-09) e validado em CI. Regras absolutas:

1. **`internal/<modulo>/domain`** NÃO importa:
   - `internal/<modulo>/application`
   - `internal/<modulo>/adapters`
   - `internal/infrastructure/*` (exceto `internal/infrastructure/errors` sentinels base se necessário — caso a caso)
   - `configs/*` (Domain puro não conhece config)
   - `github.com/JailtonJunior/devkit-go/*` (Domain puro, zero infra)
   - `github.com/spf13/viper`
   - Bibliotecas de IO (HTTP, DB driver, OTel runtime)
2. **`internal/<modulo>/application`** NÃO importa:
   - `internal/<modulo>/adapters`
   - Bibliotecas de IO concretas (só interfaces declaradas pelo próprio application)
3. **`internal/<modulo>/adapters`** PODE importar:
   - `internal/<modulo>/domain` + `internal/<modulo>/application` (para satisfazer ports)
   - `internal/infrastructure/*`
   - `devkit-go/*`
4. **Cross-module**: `internal/identity/*` NÃO pode importar `internal/finance/*` diretamente. Comunicação via:
   - **Interface declarada em `application`** do consumidor + adapter no produtor; OU
   - **Domain Event** via `internal/infrastructure/events` (preferencial para integração eventual).
5. **`cmd/server`** é o único lugar onde a composição de todos os módulos é permitida.

Sinais de violação (Object Calisthenics #5, #7): se um pacote `adapters` cresce demais ou se uma struct em `domain` precisar conhecer SQL/HTTP, refatorar antes de mergear.

## Design de Implementação

### Interfaces Chave

```go
// configs/config.go (esboço de assinatura — ADR-009)
type Config struct {
    AppConfig  AppConfig  `mapstructure:",squash"`
    HTTPConfig HTTPConfig `mapstructure:",squash"`
    DBConfig   DBConfig   `mapstructure:",squash"`
    O11yConfig O11yConfig `mapstructure:",squash"`
}
func LoadConfig(path string) (*Config, error)   // Viper + .env + AutomaticEnv + Validate gate
func (c *Config) Validate() error               // fail-fast: env, senhas, secrets, ranges
func (d *DBConfig) DSN() string                 // uso interno; NUNCA logar
func (d *DBConfig) SafeDSN() string             // senha como ***; único formato permitido em logs
```

```go
// internal/infrastructure/runtime/app.go
type AppMode string
const (ModeServer AppMode = "server"; ModeWorker AppMode = "worker")

type App interface {
    Run(ctx context.Context) error
    Shutdown(ctx context.Context) error
}

func Bootstrap(cfg *configs.Config, mode AppMode) (App, error)  // injeta cfg em todos os subsistemas
```

```go
// cmd/server/cmd.go (mesmo pattern para worker e migrate)
func New() *cobra.Command {
    return &cobra.Command{
        Use:   "server",
        Short: "Sobe o servidor HTTP MeControla",
        RunE: func(cmd *cobra.Command, args []string) error {
            cfg, err := configs.LoadConfig(".")
            if err != nil { return err }
            app, err := runtime.Bootstrap(cfg, runtime.ModeServer)
            if err != nil { return err }
            return app.Run(cmd.Context())
        },
    }
}
```

```go
// internal/infrastructure/database/manager.go
type Manager interface {                       // re-export tipado do devkit-go/pkg/database
    Pool() *pgxpool.Pool
    HealthCheck(ctx context.Context) error
}

// UnitOfWork tipado por agregado (consumido pelos PRDs subsequentes)
type UnitOfWork[T any] interface {
    Do(ctx context.Context, fn func(tx Tx) (T, error)) (T, error)
}
```

```go
// internal/infrastructure/events/bus.go
type Event interface {
    Name() EventName
    OccurredAt() time.Time
    AggregateID() string
}

type Bus interface {
    Publish[E Event](ctx context.Context, evt E) error
    Subscribe[E Event](handler func(ctx context.Context, evt E) error) (unsubscribe func(), err error)
    Close(ctx context.Context) error
}
```

```go
// internal/infrastructure/clock/clock.go
type Clock interface { Now() time.Time }
```

```go
// internal/infrastructure/errors/problem.go
type ProblemDetails struct {
    Type     string `json:"type"`
    Title    string `json:"title"`
    Status   int    `json:"status"`
    Detail   string `json:"detail,omitempty"`
    Instance string `json:"instance,omitempty"`
}

func ToProblemDetails(err error) ProblemDetails  // mapeia sentinels conhecidos; default 500
```

### Modelos de Dados

**Tabelas Postgres da foundation** (mínimas, criadas pela migration de exemplo `0001_init.up.sql`):
- `schema_migrations` (gerada por `golang-migrate`).
- `health_probe` (1 linha estática, usada pelo `/ready` para validar SELECT real além do ping de conexão).

Nada além disso — tabelas de domínio (users, transactions, conversations, ...) ficam para os PRDs respectivos.

### Endpoints de API

| Método | Path | Descrição |
| --- | --- | --- |
| `GET` | `/health` | Status do processo. `200` sempre que o binário estiver vivo. |
| `GET` | `/live` | Liveness puro (mesmo que `/health`; distinção semântica p/ Fly probe). |
| `GET` | `/ready` | Readiness: roda `SELECT 1 FROM health_probe`; `200` se OK, `503` com `ProblemDetails` se não. |

Todas as respostas em `application/json`. Erros em formato RFC 7807 (`application/problem+json`).

## Pontos de Integração

| Integração | Componente | Auth | Tratamento de erro |
| --- | --- | --- | --- |
| **Fly Postgres** | `internal/infrastructure/database` | DSN com TLS obrigatório (`sslmode=require`) via Fly secrets | `pgx` errors → sentinel `database.ErrConnection`/`ErrUnique`/... no adapter; nunca expostos |
| **Grafana Cloud OTLP** | `internal/infrastructure/observability` | Basic Auth via env `OTEL_EXPORTER_OTLP_HEADERS` (Fly secret) | exporter retry interno; falhas logadas com `slog.Warn`, nunca derrubam request |
| **Fly secrets** | runtime via env | nativo Fly | falha em `Config` validation = bootstrap abortado |

Nenhuma outra integração externa nesta foundation (sem Meta Cloud API, sem OpenAI — ficam nos PRDs próprios).

## Estratégia de Erros

Ancorada em **R-ERR-001** + **shared-patterns.md §Error Handling Cross-Stack**.

### Modelagem

- **Cada módulo define seus próprios sentinels** em `internal/<modulo>/domain/errors.go`. Exemplo canônico para a foundation:
  ```go
  // internal/infrastructure/database/errors.go
  var (
      ErrConnection = errors.New("database: connection failed")
      ErrMigration  = errors.New("database: migration apply failed")
  )
  ```
- **Sem sentinels globais cross-module** (rejeitado na ADR-004): viola R-DDD-001 (modularização).
- **Mensagens internas curtas, lowercase, estáveis** — usadas só em logs e wrap.

### Wrapping

- Adapters wrappam erros de driver com contexto: `fmt.Errorf("inserting health_probe row: %w", err)`.
- Cadeia preservada para `errors.Is`/`errors.As`.
- Application **não wrappam** com texto de apresentação — só propaga.

### Apresentação (boundary HTTP)

- `internal/infrastructure/errors/problem.go` traduz sentinel → `ProblemDetails`:
  - `ErrConnection` → 503 `database-unavailable`
  - `validator.ValidationErrors` → 400 `invalid-request` com detalhamento por campo (allowlist)
  - `context.DeadlineExceeded` → 504 `timeout`
  - Outros → 500 `internal-error` (sem expor stack/SQL/path interno; R-SEC-001 §Filesystem)
- **Nunca expor stack trace ao usuário final** (R-ERR-001 §Proibido).

### Captura e Propagação

- Captura na fronteira mais externa: middleware HTTP do `devkit-go/pkg/http_server` (recovery + logger + problem-details).
- Worker: cada handler de eventbus envolve seu trabalho em `defer recover()` + log estruturado + republish em DLQ in-process (placeholder).
- `panic` reservado para invariantes impossíveis (fail-fast em bootstrap); nunca para erro recuperável (R-ERR-001 §Proibido).

### Validação

- **`go-playground/validator` v10** no boundary HTTP via tags em DTOs (security-app.md §Input Validation).
- **Value Objects no domain** validados em construtor (`func NewPort(n int) (Port, error)`); domain **não aceita primitivos** (Object Calisthenics #3, R-DDD-001 §Value Objects).
- DTOs `application` → VO `domain` via mapper em `adapters`; application nunca recebe primitivo cru.

### Retry

- Postgres connect retry: 3 tentativas, backoff exponencial via `cenkalti/backoff/v4` (já vendorado pelo `devkit-go`).
- OTLP exporter: retry interno do SDK OTel.
- Nenhum retry em HTTP handlers (responsabilidade do cliente Meta nos PRDs futuros).

## Estratégia de Testes

Ancorada em **R-TEST-001** + `go-implementation/references/testing.md`.

### Stack

- `stdlib testing` + `github.com/stretchr/testify` (`assert`, `require`, `suite`).
- `github.com/vektra/mockery/v2` para mocks de interface, configurado em `.mockery.yml` (template já presente no orchestrator).
- `github.com/testcontainers/testcontainers-go` para integração com Postgres ephemeral.
- Build tags: `//go:build integration` para separar suites; `task test:unit` e `task test:integration` rodam isolados (RF-05).

### Testes Unitários (foundation)

Cobertura prioritária (R-TEST-001 §Cobertura Prioritária):

| Componente | Cenário |
| --- | --- |
| `internal/infrastructure/config` | parsing de env válido; falha de validação por env ausente; range inválido de `Port` |
| `internal/infrastructure/runtime` | `AppMode` parsing (válido/inválido); seleção de subsistemas por modo (table-driven) |
| `internal/infrastructure/events` | `Publish[E]` típico; subscribe + unsubscribe; buffer cheio (backpressure); `Close` idempotente |
| `internal/infrastructure/errors` | `ToProblemDetails` para cada sentinel conhecido (table-driven) + default 500 |
| `internal/infrastructure/clock` | `SystemClock.Now()` ≥ chamada anterior; `FakeClock` determinístico |
| `internal/infrastructure/http` (handlers) | `/health`, `/live`, `/ready` (com mock de `Manager.HealthCheck`) |
| VOs (`Port`, `DSN`, `OTLPEndpoint`, `EventName`, `AppMode`) | construtor válido/inválido (table-driven) + imutabilidade |

Doubles via mockery (apenas para interfaces externas: `Manager`, `Clock`, `Bus`). Sem mocks no domain (R-DDD-001).

### Testes de Integração (foundation)

**Sim** — atendem os 3 critérios do template (DB é fronteira IO crítica; sem precedente de incidente mas custo de testcontainers é baixo; PRD D-13 manda).

| Suite | Componentes | Dependências |
| --- | --- | --- |
| `database_integration_test.go` | `Manager.NewPool` + `RunMigrations` + UoW commit/rollback | `postgres:16-alpine` ephemeral via testcontainers (D-20) |
| `migrations_integration_test.go` | `golang-migrate up` aplica todas as migrations + `down` reverte sem erro | `postgres:16-alpine` ephemeral |
| `events_integration_test.go` | Publish/Subscribe sob carga (1000 eventos concorrentes); valida ordem por subscriber, drop em close | nenhum externo (in-process) |
| `http_integration_test.go` | `/ready` com Postgres real ligado e depois derrubado; valida 200 → 503 | `postgres:16-alpine` ephemeral |
| `cmd_integration_test.go` | compila o binário `mecontrola` e exerce `--help`, `server --help`, `worker --help`, `migrate` (com testcontainers Postgres); valida exit codes (D-22 + CS-21 + CS-22) | `postgres:16-alpine` ephemeral |

Tag: `//go:build integration`. Job CI dedicado em `task test:integration` (RF-18).

### Testes E2E

Fora de escopo da foundation (sem fluxo de negócio para validar ponta a ponta). Entram nos PRDs com handler real (Identity, Conversation, etc.).

## Object Calisthenics — Aplicação Concreta

Conforme `object-calisthenics-go/references/rules.md`, regras como **heurísticas, não dogma**. Aplicação mandatória na foundation, reforçada pela skill `object-calisthenics-go` em review automático de PR (modo `review`):

| Regra | Aplicação concreta na foundation |
| --- | --- |
| #1 Uma indentação por função | Handlers HTTP usam early-return; bootstrap em `runtime` quebrado em funções pequenas (`buildConfig`, `buildDatabase`, `buildObservability`, ...) |
| #2 Sem `else` quando há return cedo | Validators em VOs retornam erro no topo; `else` proibido em `internal/infrastructure/*` por convenção |
| #3 Encapsular primitivos de domínio | Todos os VOs da §Modelagem; nenhum `string`/`int` cru em campos de struct exportada |
| #4 Coleções de primeira classe | `EventBuffer`, `MigrationSet` quando comportamento (filter, sort) emergir — não força agora |
| #5 Um ponto por linha | Proibido `cfg.Database.Pool.Conns()`; chamar `cfg.DatabasePoolConns()` ou passar VO |
| #6 Nomes sem abreviação opaca | `requestCtx` em vez de `ctx2`; `eventBus` em vez de `bus`; pacote `events` (não `evts`) |
| #7 Entidades pequenas | Limite soft: structs com >5 campos sem coesão clara → ADR ou refactor; funções >40 linhas → revisão |
| #8 ≤2 variáveis de instância | Tratado como sinal de alerta para services; aceito >2 em `Config`, `App` (DTOs/composição) |
| #9 Sem getters/setters mecânicos | VOs expõem comportamento (`p.Address()`, `m.IsApplied()`); não `GetX`/`SetX` |

Enforcement:
- `golangci-lint` com `revive` regra `cyclomatic` (limite 10) + `function-length` (limite 40).
- `depguard` para fronteiras hexagonais.
- Skill `object-calisthenics-go` invocada por hook em PRs novos (modo review automático).

## Sequenciamento de Desenvolvimento

### Ordem de Build (Fases 0–1 do roadmap do discovery, semanas 1–6)

1. **Bootstrap do harness + Taskfile + governance** (1–2 dias)
   - `git init -b main`, `ai-spec install --tools claude,gemini,codex,copilot --langs go` (já feito).
   - Aplicar skill `taskfile-production` gerando `Taskfile.yml` + `taskfiles/*.yml` + scripts.
   - `CODEOWNERS`, `.pre-commit-config.yaml`, `task setup`.
   - **Critério de pronto:** `task --list-all` + `validate-taskfile.py` + `ai-spec doctor/lint` verdes.

2. **`configs/config.go` (Viper + grupos + Validate) + `internal/infrastructure/runtime` + `cmd/` cobra** (2 dias)
   - `configs/config.go` com `Config`, `AppConfig`, `HTTPConfig`, `DBConfig`, `O11yConfig`; `LoadConfig(".")` + `Validate()`; `DSN()`/`SafeDSN()`.
   - `.env.example` na raiz com todas as chaves + defaults: `CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:5173`, `OTEL_TRACE_SAMPLE_RATE=1.0`, demais placeholders inseguros (`CHANGE_ME_*`) (D-21).
   - `internal/infrastructure/runtime`: VO `AppMode` ∈ {server, worker}; `Bootstrap(cfg, mode)`.
   - `cmd/main.go` + `cmd/{server,worker,migrate}/cmd.go` com cobra v1.10.2 (D-19, ADR-010).
   - **Critério de pronto:** `task build` + `task test:unit` verdes; `./mecontrola --help` lista 3 subcomandos; `./mecontrola server` rejeita ausência de `.env` em dev; tabela de Validate() table-driven cobre 5 cenários (CS-18..CS-22).

3. **`internal/infrastructure/observability`** (1 dia)
   - Composição do `devkit-go/pkg/observability` com OTLP gRPC; redaction ativa.
   - `slog` + `otelslog` para logs.
   - **Critério de pronto:** spans saem para um coletor local (docker compose) em dev.

4. **`internal/infrastructure/database` + migrations embed** (2 dias)
   - `manager.Manager` factory; helper `RunMigrations` com `//go:embed`.
   - Migration `0001_init` criando `health_probe`.
   - Sentinels `ErrConnection`/`ErrMigration`.
   - Testes de integração com testcontainers.
   - **Critério de pronto:** `task test:integration` verde; `task migrate:up`/`migrate:down` funcionam local.

5. **`internal/infrastructure/http` + handlers de health** (1 dia)
   - Composição do `devkit-go/pkg/http_server` com defaults; CORS estrito (allowlist via env).
   - Handlers `/health`, `/live`, `/ready` (consome `Manager.HealthCheck`).
   - Mapper `ToProblemDetails`.
   - **Critério de pronto:** `task run` sobe o servidor; `curl /ready` responde 200 com DB e 503 sem.

6. **`internal/infrastructure/events` (typed bus)** (1 dia)
   - Generics Go 1.26: `Bus.Publish[E]` + `Subscribe[E]`; backpressure por buffer; `Close()` idempotente.
   - Sem evento real publicado (esqueleto).
   - **Critério de pronto:** integration test com 1000 eventos concorrentes verde.

7. **`internal/infrastructure/clock` + `errors` (Problem Details mapper)** (½ dia)
   - `Clock`/`SystemClock`/`FakeClock`.
   - Mapper completo cobrindo sentinels conhecidos + default 500.

8. **Esqueletos dos 6 módulos de domínio** (½ dia)
   - `internal/<modulo>/{domain,application,adapters}/doc.go` + README com pattern (aggregate/entity/VO template).
   - depguard rules em `.golangci.yml` validando fronteiras.
   - **Critério de pronto:** `task lint` passa; tentar import inválido em test e ver falha.

9. **Dockerfile + fly.toml + CI/CD + supply chain + signing + disclosure** (3–4 dias; M5 do rollout)
   - `Dockerfile` multi-stage: builder `golang:1.26.3-alpine` + runtime `gcr.io/distroless/static-debian12:nonroot` (ADR-011); ≤ 30 MB; UID 65532.
   - `fly.toml`: 2 processes (ADR-011) na região `gru`.
   - `.github/workflows/ci.yml`: jobs unit / integration / lint / build / security (govulncheck + trivy fs) / ai-spec doctor/lint / validate-taskfile / conventional commits / **coverage comment via `fgrosse/go-coverage-report`** (ADR-015); concurrency por PR; cache `.task/`, `~/go/pkg/mod`, mockery.
   - `.github/workflows/cd.yml`: build + push **`ghcr.io/limateixeiratecnologia/mecontrola:<sha>`** + `trivy image` (SBOM SPDX) + **`cosign sign --yes ghcr.io/...@<digest>`** (keyless via OIDC) + **`cosign attest --predicate sbom.json --type spdxjson`** + `flyctl deploy` em push para `main`; release com tag conforme D-05 + assinatura `cosign sign-blob` no tarball.
   - `.github/workflows/auto-merge.yml`: workflow que dá merge em PRs Dependabot com label + CI verde + 1 review (ADR-012).
   - `.github/dependabot.yml`: grupos `gomod`, `github-actions`, `docker`; schedule semanal terça 06:00 UTC (ADR-012).
   - `.trivyignore`: vazio inicial; supressões com data + CVE-ID + justificativa + revisão 7d.
   - `SECURITY.md` na raiz: política de disclosure (canal seguro, SLA 7d, escopo) (ADR-013).
   - `tools.go` na raiz com `//go:build tools` e `import _ "github.com/stretchr/testify"` etc.; `taskfiles/vars.yml` com `GOLANGCI_LINT_VERSION`, `MOCKERY_VERSION`, `GOVULNCHECK_VERSION`, `TRIVY_VERSION`, `COSIGN_VERSION`, `MIGRATE_VERSION`, `PRE_COMMIT_VERSION` (ADR-014).
   - Branch protection na `main` configurada via GitHub para exigir `Require signed commits` (ADR-013); `gitsign` instalado localmente via `task setup`.
   - **Critério de pronto:** PR de exemplo verde + deploy em Fly + `fly status` mostra `app` + `worker` em `started`; `trivy image` sem HIGH/CRITICAL; `cosign verify` `verified=true`; comentário de cobertura postado no PR; `git log --show-signature` mostra "Good signature" (CS-01..CS-04, CS-07, CS-23..CS-30).

10. **Runbooks + README + ADRs versionados** (1 dia)
    - `docs/runbooks/`: deploy, rollback, restore PITR, rotação de secret, upgrade do `ai-spec`.
    - `README.md`: stack + comandos `task` + link p/ PRD.
    - ADRs já em `.specs/prd-mecontrola-foundation/adr-*.md`.

### Dependências Técnicas Bloqueantes

- `devkit-go` v0.4.0 publicado e acessível.
- Fly.io account com app `mecontrola` e Postgres provisionado em `gru`.
- Grafana Cloud free tier com OTLP endpoint + token.
- `ai-spec` v0.26.0+ instalado em CI (via Action oficial) e local.
- Task v3.51.1 instalado (via skill ou Action `arduino/setup-task`).

## Monitoramento e Observabilidade

Composição via `internal/infrastructure/observability` consumindo `devkit-go/pkg/observability`.

### Métricas (Prometheus + OTLP)

Métricas automáticas do `devkit-go` (não duplicar):
- `http.server.request.duration` (histogram)
- `http.server.request.count` (counter)
- `http.server.request.active` (up-down counter)
- `http.server.request.error.count` (counter)
- `database.tx.duration_ms` (histogram)
- `database.tx.committed` (counter)
- `database.tx.rolledback` (counter)

Métricas custom da foundation:
- `bootstrap_duration_seconds` (gauge) — tempo total do `Bootstrap`
- `events_published_total{event_name,outcome}` (counter)
- `events_subscriber_lag_seconds{event_name}` (gauge — só placeholder, valor 0 enquanto sem evento real)
- `health_probe_status{check}` (gauge — `db_ping`, `db_select`)

### Logs

`slog` JSON via `otelslog`; níveis `debug` (dev) / `info` (prod) / `warn` / `error`. Campos automáticos: `request_id`, `trace_id`, `span_id`, `module`, `service.name=mecontrola`, `service.version`. Redaction enforced pelo devkit-go.

### Traces

Root span por request HTTP + por bootstrap. Spans filhos automáticos para DB tx e (futuramente) handlers de eventbus. Exporter OTLP gRPC para Grafana Cloud Tempo; sampling 100% nas 2 primeiras semanas, 20% após estabilização (configurável por env).

### Dashboards (Grafana Cloud)

- "Plataforma": latência HTTP p50/p95/p99, taxa de 5xx, métricas de DB pool e tx, health check trends, bootstrap_duration.
- Alertas mínimos: `/health` indisponível >5min (PagerDuty stub), error rate 5xx >5% em 15min.

Dashboards definidos como código no PRD futuro de observabilidade (Epic 02 do discovery, expandido); foundation só liga o canal.

## Considerações Técnicas

### Decisões Chave (ADRs derivadas)

Cada decisão material foi reificada numa ADR separada em `.specs/prd-mecontrola-foundation/`:

| ADR | Decisão | Arquivo |
| --- | --- | --- |
| ADR-001 | `internal/infrastructure/` substitui `internal/platform/` do discovery | [`adr-001-internal-infrastructure-layout.md`](./adr-001-internal-infrastructure-layout.md) |
| ADR-002 | `manager.Manager` central + `UnitOfWork[T]` genérico | [`adr-002-database-manager-central-uow.md`](./adr-002-database-manager-central-uow.md) |
| ADR-003 | Eventbus tipado via generics + emissão pós-`UoW.Commit` | [`adr-003-typed-eventbus-generics.md`](./adr-003-typed-eventbus-generics.md) |
| ADR-004 | Modelo de erros: sentinels por módulo + RFC 7807 no boundary | [`adr-004-error-sentinels-rfc7807.md`](./adr-004-error-sentinels-rfc7807.md) |
| ADR-005 | Validação dual: validator no boundary + VOs no domain | [`adr-005-validation-dual-strategy.md`](./adr-005-validation-dual-strategy.md) |
| ADR-006 | Stack de testes: stdlib + testify + mockery + testcontainers | [`adr-006-test-stack-testify-mockery.md`](./adr-006-test-stack-testify-mockery.md) |
| ADR-007 | Migrations via `//go:embed` | [`adr-007-migrations-go-embed.md`](./adr-007-migrations-go-embed.md) |
| ADR-008 | HTTP middleware: defaults devkit-go + CORS estrito + OTel | [`adr-008-http-middleware-stack.md`](./adr-008-http-middleware-stack.md) |
| ADR-009 | Viper v1.21.0 + pasta `configs/` + Validate() fail-fast + DSN/SafeDSN | [`adr-009-viper-configs-validate.md`](./adr-009-viper-configs-validate.md) |
| ADR-010 | `spf13/cobra` v1.10.2 + binário único com subcomandos `server`/`worker`/`migrate` | [`adr-010-cobra-subcommands.md`](./adr-010-cobra-subcommands.md) |
| ADR-011 | Deploy stack: Docker distroless nonroot + Fly 2 processes (app + worker) | [`adr-011-docker-fly-deploy.md`](./adr-011-docker-fly-deploy.md) |
| ADR-012 | Supply chain: govulncheck + trivy fs + Dependabot grupado | [`adr-012-supply-chain-scan-deps.md`](./adr-012-supply-chain-scan-deps.md) |
| ADR-013 | GHCR + cosign keyless image signing + SLSA attestations + SECURITY.md + gitsign commits | [`adr-013-signing-attestation-disclosure.md`](./adr-013-signing-attestation-disclosure.md) |
| ADR-014 | Tool pinning: `taskfiles/vars.yml` (binários CLI) + `tools.go` (deps Go com `//go:build tools`) | [`adr-014-tool-pinning-vars-tools-go.md`](./adr-014-tool-pinning-vars-tools-go.md) |
| ADR-015 | Coverage report via `fgrosse/go-coverage-report` action (PR comment) | [`adr-015-coverage-report-pr-comment.md`](./adr-015-coverage-report-pr-comment.md) |

### Riscos Conhecidos

| Risco | Impacto | Mitigação |
| --- | --- | --- |
| Generics no eventbus podem inflar binário ou degradar compile-time | binário maior; build CI mais lento | medir antes/depois com `go build -ldflags="-s -w"`; perfilar com `go build -gcflags=-m`; cap em ≤30 MB e build ≤90 s |
| Migrations via `//go:embed` impedem hotfix sem redeploy | rollback de schema exige deploy de versão anterior | runbook documentado; migrations sempre backward-compatible por uma versão (RF do PRD) |
| `internal/infrastructure/` virar lixeira de tudo que não cabe no domínio | erosão arquitetural | depguard com regras explícitas; review skill `object-calisthenics-go` em PR |
| Grafana Cloud free tier estourar limite de ingestion | perda de observabilidade silenciosa | alertar quando uso ≥80% via Grafana próprio; sampling adaptativo |
| Falta de Domain Events reais na foundation pode mascarar bug no Bus | descoberta tardia em PRD seguinte | integration test com 1000 eventos sintéticos cobre o caminho crítico |
| Testcontainers-go pode falhar em runners sem Docker | CI quebra silencioso em fork | gate `requires.preconditions` no Taskfile valida Docker disponível antes |
| `manager.Manager` central virar SPOF de teste se uso inadequado | testes lentos ou flaky | UoW[T] permite isolar por agregado; SKIP LOCKED na primeira tabela criada por convenção |

### Plano de Rollout

Alinhado às **Fases 0–1 do discovery** (semanas 1–6). A foundation **não vai para produção real com usuário final** — vai para deploy de staging Fly região `gru` validando os critérios de sucesso CS-01..CS-17 do PRD. PRDs subsequentes (Identity, Conversation, etc.) é que adicionam funcionalidade visível.

| Marco | Critério de aceite | Critério de rollback |
| --- | --- | --- |
| M1: PRD + techspec + ADRs aprovados | este documento + 8 ADRs commitados | n/a |
| M2: Taskfile + governance verdes | `task --list-all` + `validate-taskfile.py` + `ai-spec doctor/lint` `pass` | reverter commit |
| M3: `cmd/server` build + unit tests verdes | `task ci` verde local | reverter commit |
| M4: Integration tests verdes em CI | job `test:integration` verde no GitHub Actions | reverter PR |
| M5: Deploy staging Fly `gru` | `/health` 200 + `/ready` 200 + spans no Grafana | `fly releases rollback` ou `fly deploy --image <prev>` |
| M6: Smoke test pós-deploy | curl em 3 endpoints + 1 span correlacionado em Tempo | `fly releases rollback` |
| M7: Runbooks publicados | `docs/runbooks/*.md` reviewed | n/a |

Rollback automático: nenhum no MVP (CD simples). Manual via `fly releases rollback` em <5 min (CS-07). Migrations sempre reversíveis (PRD RF + ADR-007).

Feature flags: não aplicável à foundation (sem feature de usuário). Entram nos PRDs seguintes.

### Conformidade com Padrões

| Regra | Onde aplicada | Verificação |
| --- | --- | --- |
| R-DDD-001 (DDD) | VOs em `internal/infrastructure`; scaffold pattern em `internal/<modulo>/domain` | code review + `golangci-lint` + skill `object-calisthenics-go` |
| R-ERR-001 (errors) | Sentinels por pacote + wrapping + boundary translator | `errcheck` no golangci-lint + testes table-driven do mapper |
| R-SEC-001 (security baseline) | Secrets em Fly secrets + envconfig + redaction OTel + sem hardcoded | `ai-spec lint` + revisão de PR + audit de `.env.example` |
| security-app.md | Validator no boundary + CORS allowlist + HTTPS only + queries via pgx parametrizadas | `golangci-lint` (sqlanalyzer) + test de CORS |
| R-TEST-001 (testing) | Cobertura prioritária definida + table-driven + testcontainers + determinismo | `task test:unit` + `task test:integration` no CI |
| shared-architecture.md | DI manual via construtores; sem container; sem service locator | code review |
| shared-patterns.md (Repository, Factory, UoW) | repos por agregado nos PRDs futuros; foundation deixa `Manager` + UoW prontos | code review |
| Object Calisthenics (rules.md) | aplicação por regra na §Object Calisthenics; enforce em PR via skill | skill `object-calisthenics-go` em review automático |

### Arquivos Relevantes e Dependentes

**Da foundation** (criados nesta techspec):
- `Dockerfile` (multi-stage; distroless nonroot — ADR-011)
- `fly.toml` (2 processes — ADR-011)
- `.github/workflows/{ci.yml,cd.yml,auto-merge.yml}` (com supply chain scan + cosign sign + coverage comment — ADR-012/013/015)
- `.github/dependabot.yml` (grupos + auto-merge — ADR-012)
- `.trivyignore` (vazio inicial)
- `SECURITY.md` (disclosure — ADR-013)
- `tools.go` (`//go:build tools` listando deps Go de tooling — ADR-014)
- `taskfiles/vars.yml` (versões pinadas dos binários CLI externos — ADR-014)
- `cmd/main.go` (root cobra — ADR-010)
- `cmd/server/cmd.go`, `cmd/worker/cmd.go`, `cmd/migrate/cmd.go`
- `cmd_integration_test.go` (smoke do binário com subcomandos)
- `configs/{config.go,config_test.go,insecure.go}` (Viper + grupos + Validate + DSN/SafeDSN — ADR-009)
- `.env.example` (raiz; defaults D-21)
- `internal/infrastructure/runtime/{app.go,mode.go,bootstrap.go,*_test.go}`
- `internal/infrastructure/database/{manager.go,uow.go,migrations.go,errors.go,*_test.go,*_integration_test.go}`
- `internal/infrastructure/observability/{provider.go,redaction.go,provider_test.go}`
- `internal/infrastructure/http/{server.go,middleware.go,health.go,*_test.go,*_integration_test.go}`
- `internal/infrastructure/events/{bus.go,event.go,bus_test.go,bus_integration_test.go}`
- `internal/infrastructure/clock/{clock.go,fake.go,clock_test.go}`
- `internal/infrastructure/errors/{problem.go,problem_test.go}`
- `internal/<modulo>/{domain,application,adapters}/doc.go` para os 6 módulos
- `migrations/0001_init.up.sql`, `migrations/0001_init.down.sql`
- `.golangci.yml`, `.pre-commit-config.yaml`, `.mockery.yml`, `CODEOWNERS`, `.gitignore`, `.editorconfig`
- `Taskfile.yml`, `taskfiles/{build,test,lint,security,mocks,ci}.yml`, `taskfiles/scripts/*`, `.taskrc.yml`, `.env.example`
- `.github/workflows/{ci.yml,cd.yml}`
- `docs/runbooks/*.md`, `README.md`

**Dependentes** (consumirão a foundation nos PRDs seguintes):
- Epic 04 (Identity) — primeiro a usar `Manager` + `UoW[Aggregate]` real e a emitir Domain Event
- Epic 02+ (Observabilidade) — expandirá dashboards definidos como código
- Epic 06 (Webhook WhatsApp) — primeiro a usar `Bus` real com `MessageReceived`
- Epic 12 (CI/CD Fly) — refinará pipeline construído nesta foundation

### Itens em aberto pós-techspec

- Versão exata de `pre-commit` framework a pinar (vem por release do projeto); definir em ADR adicional se necessário.
- Política de sampling OTel após 2 semanas (decisão operacional, não trava implementação).
- Threshold exato do alerta `health-probe-down` (5 min é placeholder; calibrar pós-deploy).

Nenhum bloqueante para iniciar `create-tasks`.
