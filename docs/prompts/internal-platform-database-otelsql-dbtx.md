# Prompt original

Criar `internal/platform/database` em Go seguindo este exemplo com Options Pattern sobre `github.com/JailtonJunior94/devkit-go/pkg/database/postgres_otelsql`, expondo `DatabaseOption`, `WithDSN`, `WithMaxOpenConns`, `WithMaxIdleConns`, `WithConnMaxLifetime`, `WithConnMaxIdleTime`, `WithMetrics`, `WithQueryLogging`, `WithServiceName` e `NewDatabaseManager(ctx, opts...)`.

O uso desejado deve acontecer em:
- `cmd/worker/worker.go`
- `cmd/server/server.go`
- `cmd/migrate/migrate.go`

No formato:

```go
dbManager, err := database.NewDatabaseManager(
    ctx,
    database.WithMetrics(true),
    database.WithDSN(cfg.DBConfig.DSN()),
    database.WithConnMaxLifetime(5*time.Minute),
    database.WithConnMaxIdleTime(2*time.Minute),
    database.WithServiceName(cfg.HTTPConfig.ServiceName),
    database.WithMaxOpenConns(cfg.DBConfig.DBMaxOpenConns),
    database.WithMaxIdleConns(cfg.DBConfig.DBMaxIdleConns),
)
if err != nil {
    return fmt.Errorf("run: failed to connect to database: %v", err)
}
o11y.Logger().Info(ctx, "database connection established with OpenTelemetry instrumentation")
```

Tambem quero que a injecao funcione corretamente e que o repositorio use este formato:

```go
type userRepository struct {
    db   database.DBTX
    o11y observability.Observability
    fm   *metrics.FinancialMetrics
}

func NewUserRepository(db database.DBTX, o11y observability.Observability, fm *metrics.FinancialMetrics) interfaces.UserRepository {
    return &userRepository{
        db:   db,
        o11y: o11y,
        fm:   fm,
    }
}
```

E obrigatorio, mandatorio e inegociavel carregar a skill `go-implementation`, seus exemplos e tambem usar os exemplos deste proprio prompt como referencia obrigatoria de desenho, sempre adaptados ao contexto real do repositorio.

Mandatorio e inegociavel ter `0 comentarios` no codigo produzido.

# Prompt enriquecido

```text
Quero que voce implemente a capacidade compartilhada `internal/platform/database` para este repositorio Go, criando um wrapper consistente sobre `github.com/JailtonJunior94/devkit-go/pkg/database/postgres_otelsql` com Options Pattern, instrumentacao OpenTelemetry e injecao correta para repositories e bootstrap da aplicacao.

Tambem e obrigatorio, mandatorio e inegociavel:
1. usar a skill `go-implementation` e seus exemplos como base normativa de implementacao;
2. usar os exemplos deste proprio prompt como referencia obrigatoria de bootstrap, API e injecao, adaptando-os ao estado real do repositorio quando houver divergencia;
3. entregar codigo com `0 comentarios` no resultado final, sem comentarios de linha, bloco, doc comments ou observacoes inline adicionadas pela implementacao.

Antes de qualquer alteracao, carregue obrigatoriamente:
1. `AGENTS.md`
2. `.github/skills/agent-governance/SKILL.md`
3. `.github/skills/go-implementation/SKILL.md`
4. As referencias/examples da skill Go relevantes para esta tarefa:
   - `.github/skills/go-implementation/references/architecture.md`
   - `.github/skills/go-implementation/references/interfaces.md`
   - `.github/skills/go-implementation/references/persistence.md`
   - `.github/skills/go-implementation/references/observability.md`
   - `.github/skills/go-implementation/references/configuration.md`
   - `.github/skills/go-implementation/references/testing.md`
   - `.github/skills/go-implementation/references/examples-infrastructure.md`
   - `.github/skills/go-implementation/references/examples-domain-flow.md`
   - `.github/skills/go-implementation/references/examples-testing.md`
5. `go.mod` para respeitar a versao declarada do Go e as dependencias reais do projeto.

Contexto real do repositorio que deve orientar a implementacao:
- `go.mod` declara Go `1.26.2` e `github.com/JailtonJunior94/devkit-go v0.4.0`.
- O repositorio e um monolito modular em Go; capacidades tecnicas compartilhadas devem viver em `internal/platform`.
- `internal/platform` nao pode importar `internal/<modulo>/...`.
- O fluxo arquitetural deve continuar preservado: transporte/scheduler/bootstrap -> application/usecase -> repositories e/ou clients.
- Na arvore local atual, `cmd/server/server.go`, `cmd/worker/worker.go` e `cmd/migrate/migrate.go` ja importam `internal/platform/database` e usam `database.NewManager(...)`.
- Na arvore local atual, `cmd/server/server.go` e `cmd/worker/worker.go` usam `platformworker.NewManager(...)`.
- Na arvore local atual, especificamente `cmd/server/server.go` usa composicao manual via `chiserver.New(...)` e `server.RegisterRouters(...)`.
- Na arvore local atual, os paths `internal/identity/module.go`, `internal/billing/module.go`, `internal/identity/...`, `internal/billing/...`, `migrations/...` e `internal/platform/database/...` aparecem removidos do working tree, embora os entrypoints ainda dependam deles.
- O estado atual do working tree e a fonte da verdade para esta tarefa, mesmo quando houver divergencia com exemplos historicos, documentacao anterior ou expectativas do prompt.
- Portanto, trate o repositorio atual como um codebase em drift parcial: antes de aplicar o novo desenho de banco, confirme se a implementacao deve restaurar esses packages, recria-los, ou adaptar os entrypoints ao novo estado real.
- Se os packages de `identity` e `billing` forem restaurados no processo, preserve o contrato transacional historico com `database.NewUnitOfWork[...]` e o wiring de outbox.

Atencao obrigatoria a inconsistencias entre o snippet desejado e o codigo real:
- No codigo atual, os nomes de configuracao sao `cfg.DBConfig.MaxConns`, `cfg.DBConfig.MaxIdleConns`, `cfg.DBConfig.ConnMaxLifetime`, `cfg.DBConfig.ConnMaxIdleTime` e `cfg.HTTPConfig.ServiceNameAPI`.
- O snippet de referencia menciona `cfg.HTTPConfig.ServiceName`, `cfg.DBConfig.DBMaxOpenConns` e `cfg.DBConfig.DBMaxIdleConns`, mas esses nomes nao existem hoje no repositorio.
- Portanto, adapte o desenho aos nomes reais ja existentes em `configs/config.go`, a menos que exista motivo arquitetural forte e justificado para mudar a configuracao. Nao invente campos novos sem necessidade concreta.
- O mesmo vale para a API exata do pacote `postgres_otelsql`: reutilize os tipos/exportacoes reais do devkit-go e adapte o wrapper local apenas quando houver diferenca objetiva entre o exemplo e a biblioteca.

Objetivo principal:
1. Criar ou completar `internal/platform/database` como capacidade compartilhada de acesso a banco, baseada em `postgres_otelsql`.
2. Expor uma API com Options Pattern no espirito deste exemplo:
   - `type DatabaseOption func(*postgres.Config)`
   - `WithDSN`
   - `WithMaxOpenConns`
   - `WithMaxIdleConns`
   - `WithConnMaxLifetime`
   - `WithConnMaxIdleTime`
   - `WithMetrics`
   - `WithQueryLogging`
   - `WithServiceName`
   - `NewDatabaseManager(ctx context.Context, opts ...DatabaseOption)`
3. Atualizar os bootstraps de:
   - `cmd/server/server.go`
   - `cmd/worker/worker.go`
   - `cmd/migrate/migrate.go`
   para usar o novo entrypoint com configuracao explicita, mantendo o comportamento atual de lifecycle/shutdown e observabilidade.
4. Garantir que repositories passem a depender de `database.DBTX` por injecao, em vez de acoplar diretamente ao manager para executar queries.
5. Manter separado:
   - o papel de lifecycle/connection manager/transaction orchestration;
   - o papel de executor de queries (`DBTX`) usado pelos repositories.

Diretrizes de desenho obrigatorias:
1. Preserve a ideia de DI manual por construtores; nao use framework de DI.
2. Reaproveite ao maximo os tipos do `postgres_otelsql` e do codebase atual; evite wrappers redundantes.
3. Se o manager for necessario para lifecycle, shutdown, `DBTX(ctx)` default, transacoes ou UnitOfWork, mantenha-o como dependencia de bootstrap/aplicacao onde fizer sentido.
4. Repositories concretos devem receber `database.DBTX` no construtor quando sua responsabilidade for apenas executar queries, seguindo o espirito deste formato:
   ```go
   type userRepository struct {
       db   database.DBTX
       o11y observability.Observability
       fm   *metrics.FinancialMetrics
   }
   ```
5. Os exemplos da skill `go-implementation` e os exemplos deste prompt devem ser seguidos obrigatoriamente como referencia de desenho, sempre com adaptacao ao contexto real do repositorio quando houver conflito objetivo.
6. O codigo final deve ter `0 comentarios`; nao adicionar comentarios de qualquer tipo.
7. Nao force repositories a conhecer detalhes de conexao, pool ou lifecycle se isso puder ficar encapsulado na camada de composicao/bootstrap.
8. Preserve o contrato transacional atual. Se algum repository hoje depende de `UnitOfWork` ou precisa operar com `tx database.DBTX`, modele a injecao de forma que uma transacao ativa consiga substituir o executor padrao sem hack nem estado global.
9. Nao quebre `billing`, `identity`, `migrate` nem o wiring do outbox.
10. Logging e erros devem seguir as convencoes do repositorio: mensagem contextual curta, sem logar DSN com senha, sem fallback silencioso.
11. Para logs/telemetria, respeite a API real de observabilidade do projeto. Se `o11y.Logger().Info(...)` nao existir exatamente assim, adapte semanticamente ao logger/observability real sem inventar abstrações desnecessarias.
12. Nao copiar literalmente o snippet de referencia quando ele conflitar com o repositorio; adapte ao contexto real.

Arquivos e areas minimas que devem ser inspecionadas antes de editar:
- `git status --short`
- `configs/config.go`
- `cmd/server/server.go`
- `cmd/worker/worker.go`
- `cmd/migrate/migrate.go`
- imports quebrados ou paths ausentes referenciados por `cmd/server/server.go`, `cmd/worker/worker.go` e `cmd/migrate/migrate.go`
- `internal/identity/module.go`, se o arquivo existir ou precisar ser restaurado
- `internal/billing/module.go`, se o arquivo existir ou precisar ser restaurado
- repositories postgres que hoje dependem de `*database.Manager`, se ainda estiverem presentes ou se precisarem ser recriados
- usos atuais de `database.NewUnitOfWork[...]`, se ainda estiverem presentes ou se precisarem ser restaurados

Requisitos funcionais:
1. O pacote `internal/platform/database` deve centralizar a criacao da conexao/pool instrumentado com OpenTelemetry.
2. O pacote deve fornecer defaults seguros e explicitos para pool/configuracao, com override por options.
3. Os tres entrypoints (`server`, `worker`, `migrate`) devem passar a usar o novo bootstrap do banco com options claras.
4. Repositories devem aceitar `database.DBTX` por construtor para queries e comandos.
5. O desenho deve continuar suportando transacao por `database.NewUnitOfWork[...]` ou mecanismo equivalente ja usado no projeto.

Requisitos nao funcionais:
1. Sem vazar segredo em logs, erros ou traces: nunca expor `cfg.DBConfig.DSN()` em logs; para logs, usar formato seguro/redactado quando necessario.
2. Pool com limites explicitos e instrumentacao configuravel.
3. Propagacao de `context.Context` em todas as operacoes de IO.
4. Shutdown deterministico da conexao/manager.
5. Testabilidade: ajustar/adicionar testes proporcionais ao impacto da mudanca.

Proibicoes explicitas:
- Nao implementar estado global de banco.
- Nao usar `init()`.
- Nao acoplar `internal/platform/database` a modulos de negocio.
- Nao introduzir interfaces sem consumidor real.
- Nao substituir `DBTX` por `any`/`interface{}`.
- Nao quebrar o fluxo existente de `UnitOfWork`.
- Nao inventar nomes de config ou APIs que nao existam sem primeiro validar no codebase.
- Nao ignorar os exemplos da skill `go-implementation` nem os exemplos deste prompt.
- Nao deixar comentarios no codigo final sob nenhuma forma.

Criterios de aceitacao:
1. Existe uma implementacao compartilhada em `internal/platform/database` baseada em `postgres_otelsql` com Options Pattern.
2. `cmd/server/server.go`, `cmd/worker/worker.go` e `cmd/migrate/migrate.go` usam o novo entrypoint do banco com options explicitas.
3. Repositories relevantes deixam de depender diretamente de `*database.Manager` para executar queries e passam a receber `database.DBTX` por injecao onde isso for apropriado.
4. O lifecycle de conexao, shutdown e observabilidade continua funcionando corretamente.
5. `identity` e `billing` continuam respeitando as fronteiras arquiteturais e o wiring do modulo.
6. O fluxo transacional com `database.NewUnitOfWork[...]` continua viavel e correto.
7. A implementacao respeita os nomes reais de configuracao e as APIs reais do repositorio, mesmo que o exemplo fornecido use nomes levemente diferentes.
8. O codigo final entregue possui `0 comentarios`.
9. A implementacao segue obrigatoriamente a skill `go-implementation`, seus exemplos e os exemplos deste prompt, com adaptacao ao contexto real quando necessario.
10. A resposta final lista os arquivos alterados, explica o desenho adotado e explicita como a injecao de `DBTX` foi preservada em cenarios transacionais e nao transacionais.

Saida esperada:
1. Analise curta do impacto antes de codar, citando os pontos de acoplamento atuais.
2. Implementacao completa e coerente com o repositorio.
3. Testes e ajustes necessarios para cobrir o novo wiring.
4. Resumo final em PT-BR, objetivo, com foco em arquitetura, injecao, transacoes e observabilidade.

Se houver conflito entre o exemplo fornecido, `AGENTS.md`, `agent-governance` e `go-implementation`, prevalecem `AGENTS.md` e a restricao mais segura.
```

# Melhorias aplicadas

- Tornou obrigatoria a carga de `AGENTS.md`, `agent-governance`, `go-implementation` e dos exemplos/referencias Go realmente pertinentes ao problema.
- Amarrou o prompt ao estado real do repositorio, citando os entrypoints, modules e repositories hoje acoplados a `*database.Manager`.
- Explicou a principal ambiguidade do pedido: o snippet usa nomes de config que nao existem hoje (`ServiceName`, `DBMaxOpenConns`, `DBMaxIdleConns`), enquanto o repositorio usa `ServiceNameAPI`, `MaxConns` e `MaxIdleConns`.
- Separou explicitamente duas responsabilidades que precisavam ficar claras no prompt: lifecycle/UnitOfWork/manager vs. executor de queries `DBTX` injetado nos repositories.
- Tornou explicito que o uso da skill `go-implementation`, de seus exemplos e dos exemplos do proprio prompt e obrigatorio e inegociavel.
- Adicionou a exigencia objetiva de `0 comentarios` no codigo final.
- Adicionou criterios de aceitacao verificaveis para wiring, observabilidade, transacoes, protecao de segredos, ausencia de comentarios e aderencia arquitetural.
- Atualizou o prompt para refletir o drift atual do working tree: entrypoints ainda dependem de `billing`, `identity`, `migrations` e `internal/platform/database`, mas esses paths nao estao presentes localmente neste momento.
- Declarou explicitamente que o estado atual do working tree e a fonte da verdade para a tarefa, acima de exemplos historicos do proprio prompt.

# Exemplo de codigo real para analisar a proposta

O exemplo abaixo parte obrigatoriamente de `cmd/server/server.go` e `cmd/worker/worker.go`, e deve ser lido como proposta-alvo de composicao. Ele nao presume que todos os packages citados ja existam no working tree atual.

Ele preserva a separacao entre:

- `Manager` como responsavel por lifecycle, pool, shutdown e `UnitOfWork`
- `DBTX` como executor injetado nos repositories
- use cases como fronteira transacional
- handlers e clients sem acoplamento a detalhes de conexao

## Bootstrap de `cmd/server/server.go`

```go
func Run(ctx context.Context) error {
	cfg, err := configs.LoadConfig(".")
	if err != nil {
		return err
	}

	logger := slog.Default()

	provider, _, err := observability.NewProvider(cfg)
	if err != nil {
		return err
	}

	dbManager, err := database.NewDatabaseManager(
		ctx,
		database.WithMetrics(true),
		database.WithDSN(cfg.DBConfig.DSN()),
		database.WithConnMaxLifetime(cfg.DBConfig.ConnMaxLifetime),
		database.WithConnMaxIdleTime(cfg.DBConfig.ConnMaxIdleTime),
		database.WithServiceName(cfg.HTTPConfig.ServiceNameAPI),
		database.WithMaxOpenConns(cfg.DBConfig.MaxConns),
		database.WithMaxIdleConns(cfg.DBConfig.MaxIdleConns),
	)
	if err != nil {
		shutdownErr := provider.Shutdown(context.Background())
		if shutdownErr != nil {
			return errors.Join(err, shutdownErr)
		}
		return err
	}

	identityModule, err := identity.NewModule(identity.WithDatabase(dbManager))
	if err != nil {
		return errors.Join(err, provider.Shutdown(context.Background()), dbManager.Shutdown(context.Background()))
	}

	billingModule, err := billing.NewModule(
		billing.WithConfig(cfg),
		billing.WithLogger(logger),
		billing.WithDatabase(dbManager),
		billing.WithProvider(provider),
		billing.WithUserRepository(identityModule.Ports.UserRepository),
	)
	if err != nil {
		return errors.Join(err, provider.Shutdown(context.Background()), dbManager.Shutdown(context.Background()))
	}

	runnerManager := platformworker.NewManager(
		logger,
		slices.Concat(identityModule.Runners(), billingModule.Runners())...,
	)
	if err := runnerManager.Start(ctx); err != nil {
		return errors.Join(err, provider.Shutdown(context.Background()), dbManager.Shutdown(context.Background()))
	}

	server, err := chiserver.New(
		provider.Observability(),
		chiserver.WithPort(strconv.Itoa(cfg.HTTPConfig.Port)),
		chiserver.WithServiceName(cfg.HTTPConfig.ServiceNameAPI),
		chiserver.WithServiceVersion(cfg.O11yConfig.ServiceVersion),
		chiserver.WithEnvironment(cfg.AppConfig.Environment),
		chiserver.WithCORS(cfg.HTTPConfig.CORSAllowedOrigins),
		chiserver.WithMetrics(),
		chiserver.WithTracing(),
		chiserver.WithOTelMetrics(),
	)
	if err != nil {
		return errors.Join(
			err,
			runnerManager.Stop(context.Background()),
			provider.Shutdown(context.Background()),
			dbManager.Shutdown(context.Background()),
		)
	}
	server.RegisterRouters(slices.Concat(identityModule.Routers(), billingModule.Routers())...)

	if err := server.Start(ctx); err != nil {
		return errors.Join(
			err,
			runnerManager.Stop(context.Background()),
			provider.Shutdown(context.Background()),
			dbManager.Shutdown(context.Background()),
		)
	}

	return errors.Join(
		runnerManager.Stop(context.Background()),
		provider.Shutdown(context.Background()),
		dbManager.Shutdown(context.Background()),
	)
}
```

Observacoes obrigatorias sobre o estado atual:

1. Este exemplo preserva o shape atual real de `cmd/server/server.go`: `platformworker.NewManager(...)`, `chiserver.New(...)` e `server.RegisterRouters(...)`.
2. O unico ajuste proposto no bootstrap e trocar o bootstrap de banco atual por `database.NewDatabaseManager(...)`, sem criar outro entrypoint intermediario.
3. O codebase visivel hoje tem drift: o call site acima reflete o entrypoint atual, mas os packages importados por ele nao estao integralmente presentes no working tree.

## Bootstrap de `cmd/worker/worker.go`

```go
func Run(ctx context.Context) error {
	cfg, err := configs.LoadConfig(".")
	if err != nil {
		return err
	}

	logger := slog.Default()

	provider, _, err := observability.NewProvider(cfg)
	if err != nil {
		return err
	}

	dbManager, err := database.NewDatabaseManager(
		ctx,
		database.WithMetrics(true),
		database.WithDSN(cfg.DBConfig.DSN()),
		database.WithConnMaxLifetime(cfg.DBConfig.ConnMaxLifetime),
		database.WithConnMaxIdleTime(cfg.DBConfig.ConnMaxIdleTime),
		database.WithServiceName(cfg.HTTPConfig.ServiceNameAPI),
		database.WithMaxOpenConns(cfg.DBConfig.MaxConns),
		database.WithMaxIdleConns(cfg.DBConfig.MaxIdleConns),
	)
	if err != nil {
		shutdownErr := provider.Shutdown(context.Background())
		if shutdownErr != nil {
			return errors.Join(err, shutdownErr)
		}
		return err
	}

	identityModule, err := identity.NewModule(identity.WithDatabase(dbManager))
	if err != nil {
		return errors.Join(err, provider.Shutdown(context.Background()), dbManager.Shutdown(context.Background()))
	}

	billingModule, err := billing.NewModule(
		billing.WithConfig(cfg),
		billing.WithLogger(logger),
		billing.WithDatabase(dbManager),
		billing.WithProvider(provider),
		billing.WithUserRepository(identityModule.Ports.UserRepository),
	)
	if err != nil {
		return errors.Join(err, provider.Shutdown(context.Background()), dbManager.Shutdown(context.Background()))
	}

	runnerManager := platformworker.NewManager(
		logger,
		slices.Concat(identityModule.Runners(), billingModule.Runners())...,
	)
	if err := runnerManager.Start(ctx); err != nil {
		return errors.Join(err, provider.Shutdown(context.Background()), dbManager.Shutdown(context.Background()))
	}

	slog.InfoContext(ctx, "worker running background modules")

	<-ctx.Done()

	return errors.Join(
		runnerManager.Stop(context.Background()),
		provider.Shutdown(context.Background()),
		dbManager.Shutdown(context.Background()),
	)
}
```

Observacoes obrigatorias sobre o estado atual:

1. Este exemplo preserva o shape atual real de `cmd/worker/worker.go`: `platformworker.NewManager(...)` e shutdown via `runnerManager.Stop(...)`.
2. O unico ajuste proposto no bootstrap e trocar o bootstrap de banco atual por `database.NewDatabaseManager(...)`, sem criar outro entrypoint intermediario.
3. O codebase visivel hoje tem drift adicional: o call site de `platformworker.NewManager(...)` visto nos entrypoints nao bate com a assinatura hoje exposta em `internal/platform/worker/manager.go`, entao isso precisa ser reconciliado explicitamente na implementacao.

## Bootstrap de `cmd/migrate/migrate.go`

```go
func Run(ctx context.Context, writer io.Writer) error {
	cfg, err := configs.LoadConfig(".")
	if err != nil {
		return err
	}

	dbManager, err := database.NewDatabaseManager(
		ctx,
		database.WithMetrics(false),
		database.WithDSN(cfg.DBConfig.DSN()),
		database.WithConnMaxLifetime(cfg.DBConfig.ConnMaxLifetime),
		database.WithConnMaxIdleTime(cfg.DBConfig.ConnMaxIdleTime),
		database.WithServiceName(cfg.HTTPConfig.ServiceNameAPI),
		database.WithMaxOpenConns(cfg.DBConfig.MaxConns),
		database.WithMaxIdleConns(cfg.DBConfig.MaxIdleConns),
	)
	if err != nil {
		return fmt.Errorf("conectando ao banco: %w", err)
	}

	if csv := os.Getenv("ADMIN_WHATSAPP_NUMBERS"); csv != "" {
		if err := database.SetAdminWhatsAppNumbers(ctx, dbManager, csv); err != nil {
			shutdownErr := dbManager.Shutdown(context.Background())
			if shutdownErr != nil {
				return errors.Join(err, fmt.Errorf("encerrando conexao com banco: %w", shutdownErr))
			}
			return err
		}
	}

	if err := database.RunMigrations(ctx, dbManager); err != nil {
		runErr := fmt.Errorf("executando migrations: %w", err)
		if shutdownErr := dbManager.Shutdown(context.Background()); shutdownErr != nil {
			return errors.Join(runErr, fmt.Errorf("encerrando conexao com banco: %w", shutdownErr))
		}
		return runErr
	}

	if err := dbManager.Shutdown(context.Background()); err != nil {
		return fmt.Errorf("encerrando conexao com banco: %w", err)
	}

	if _, err := fmt.Fprintln(writer, "migrations aplicadas com sucesso"); err != nil {
		return fmt.Errorf("escrevendo saida do comando migrate: %w", err)
	}

	return nil
}
```

Observacoes obrigatorias sobre o estado atual:

1. Este exemplo preserva o shape atual real de `cmd/migrate/migrate.go`: `configs.LoadConfig(".")`, `database.SetAdminWhatsAppNumbers(...)`, `database.RunMigrations(...)` e shutdown explicito do manager.
2. O unico ajuste proposto no bootstrap e trocar o bootstrap de banco atual por `database.NewDatabaseManager(...)`, mantendo o restante do fluxo de migrate.

## Limite de verdade do exemplo final

De forma mandatória, o que está acima é o limite do que pode ser exemplificado sem falso positivo com base no working tree atual.

Hoje nao ha base suficiente no working tree para fixar como exemplo real e inegociavel os construtores concretos de:

- handler HTTP de billing
- use cases de billing e identity
- repositories postgres de billing e identity
- clients/adapters de Kiwify
- wiring de outbox dentro de `internal/billing`

Esses trechos foram removidos do exemplo final porque os packages correspondentes nao estao presentes localmente neste momento. Se eles forem restaurados, o refinamento deve ser refeito a partir do codigo restaurado, e nao de exemplos historicos.

## Fluxo esperado ponta a ponta sem falso positivo

1. `cmd/server/server.go` e `cmd/worker/worker.go` continuam como pontos obrigatorios de composicao
2. o bootstrap de observabilidade continua ocorrendo antes do bootstrap de banco
3. o bootstrap de banco muda de `database.NewManager(...)` para `database.NewDatabaseManager(...)`, adaptado ao shape real do pacote
4. `identity.WithDatabase(...)` e `billing.WithDatabase(...)` permanecem como contratos de entrada enquanto esses modulos forem a forma real de composicao do codebase
5. `platformworker.NewManager(...)`, `chiserver.New(...)` e `server.RegisterRouters(...)` continuam sendo preservados quando existirem no entrypoint atual
6. `cmd/migrate/migrate.go` continua preservando `SetAdminWhatsAppNumbers`, `RunMigrations` e shutdown explicito
7. qualquer detalhamento adicional de handler, use case, repository ou client so deve ser documentado depois que os respectivos packages existirem novamente no working tree
