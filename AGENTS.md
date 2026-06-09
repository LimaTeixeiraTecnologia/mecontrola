<!-- governance-schema: 1.0.0 -->
# Regras para Agentes de IA

`AGENTS.md` e a fonte canonica deste repositorio. Arquivos especificos de ferramenta (`CLAUDE.md`, `GEMINI.md`, `.github/copilot-instructions.md`, `.codex/config.toml`) podem reforcar estas regras, nunca enfraquecer.

Objetivo: manter consistencia, seguranca, economia de contexto e qualidade em tarefas reais de analise, alteracao e validacao de codigo.

## Contexto do Projeto

- Arquitetura: monolito modular em Go, com bounded contexts em `internal/`.
- Stack detectada: Go; HTTP com Chi/Fiber no historico de dependencias; gRPC em dependencias.
- Modulos ativos: `internal/identity`, `internal/billing`, `internal/platform`.
- Fluxo permitido: `infrastructure -> application -> domain`.
- `domain` nao importa `application`, `infrastructure`, `platform`, banco, HTTP, filas, serializacao, configuracao ou drivers.
- `application` pode importar `domain`, mas nao `infrastructure`.
- Comunicacao cross-module deve usar interface declarada pelo consumidor, domain event/outbox ou contrato explicito.

## Contexto e Anti-Alucinacao

1. Antes de editar, explorar o workspace com `rg`, `find`, `go.mod`, configs e entrypoints relevantes.
2. Nao inventar arquivos, packages, APIs, handlers, repositorios, rotas, migrations, interfaces ou contexto de negocio ausente.
3. Se o codigo real divergir de docs/prompts historicos, prevalece o working tree atual e a regra mais segura.
4. Exemplos externos definem padrao estrutural, nao imports, nomes ou dependencias copiaveis.
5. Se um entrypoint mencionar pacote ausente ou assinatura antiga, registrar drift em vez de mascarar com placeholder.
6. Nao criar campos, adapters, routers, jobs, consumers ou providers ficticios apenas para preencher estrutura.

## Skills Obrigatorias

- Para qualquer tarefa que altere codigo: carregar `.agents/skills/agent-governance/SKILL.md`.
- Para qualquer implementacao, alteracao ou revisao de codigo Go: carregar tambem `.agents/skills/go-implementation/SKILL.md`.
- Para bugfix com remediacao e teste de regressao: carregar `.agents/skills/bugfix/SKILL.md`.
- Para refatoracao incremental de design Go por object calisthenics: carregar `.agents/skills/object-calisthenics-go/SKILL.md`.
- Para governanca do projeto: usar `.agents/skills/analyze-project/SKILL.md`.

Economia obrigatoria:
- Classificar complexidade antes de carregar referencias.
- Carregar no maximo 4 referencias simultaneas; se mais forem necessarias, priorizar as 3 mais criticas e registrar as demais como nao carregadas.
- Preferir TL;DR quando a referencia tiver bloco `<!-- TL;DR -->`.
- Nunca carregar `references/patterns-structural.md` para Factory Function, Functional Options, Adapter, Decorator ou Facade; esses patterns ja estao inline no SKILL.md.
- Nunca carregar referencias de dominios nao afetados pela mudanca.

## Go — Obrigatorio e Inegociavel

Toda tarefa Go DEVE seguir as etapas 1-5 de `.agents/skills/go-implementation/SKILL.md`:

1. Carregar base obrigatoria, ler `references/architecture.md`, verificar `go.mod` e aplicar R0-R7.
2. Carregar apenas referencias acionadas pela tarefa: interfaces/DI, API, persistence, messaging, security, testing, configuration, observability, resilience, concurrency, build ou lifecycle.
3. Modelar antes de escrever: fronteiras, dependencia para dentro, interface no consumidor.
4. Implementar adaptando exemplos ao contexto real, nunca copiando cegamente.
5. Validar proporcionalmente: minimo `go build`, `go vet`, `go test -race -count=1` no pacote alterado e `golangci-lint run` no escopo da mudanca quando disponivel.

Regras Go que devem ficar sempre em memoria:
- R0: `init()` proibida.
- R1: funcoes de dominio/aplicacao/infraestrutura devem ser metodos de struct; excecoes: `main`, factories/construtores e helpers/testes.
- R5.8: enums com `iota + 1`; zero value reservado para nao inicializado, salvo default intencional.
- R5.10: erros com `errors.New`, `fmt.Errorf("ctx: %w", err)` e sentinels quando o caller usa `errors.Is`; tratar erro uma unica vez.
- R5.12: sem `panic` em producao.
- R5.26: globais nao exportados em camelCase; sem prefixo `_`.
- R6: `context.Context` em toda fronteira de IO; DI via construtores explicitos.
- R6.4: `var _ Interface = (*Type)(nil)` proibido.
- R6.7: `clock.Clock` proibido em use cases e repositorios; usar `time.Now().UTC()` inline quando permitido ou passar instante por command object.
- R7.1: usar `any` em vez de `interface{}`.
- R7.2: usar `log/slog` para logging estruturado.
- R7.6: usar `errors.Join` para agregar erros.
- Goroutines sempre cancelaveis via `context.Context`, sem leak e integradas ao shutdown coordenado.

## Layout Obrigatorio por Modulo

Novos modulos, features e refatoracoes estruturais em bounded contexts DEVEM usar:

```text
internal/<modulo>/
  application/
    dtos/input/
    dtos/output/
    usecases/
    interfaces/
  domain/
    entities/
    valueobjects/
    services/
    interfaces/
  infrastructure/
    http/client/
    http/server/
    jobs/handlers/
    messaging/database/producers/
    messaging/database/consumers/
    messaging/<broker>/producers/
    messaging/<broker>/consumers/
    repositories/postgres/
    repositories/mssql/
```

Responsabilidades:
- `application`: orquestracao de use cases, DTOs e interfaces consumidas pela aplicacao; sem IO concreto, SQL, brokers, SDKs externos ou handlers HTTP.
- `domain`: entidades, value objects, domain services stateless, invariantes e contratos da linguagem ubiqua; puro e sem dependencia externa.
- `infrastructure`: implementacoes concretas de IO e integracoes por tecnologia.
- `infrastructure/http/client`: toda chamada HTTP outbound para APIs externas; deve usar `internal/platform/httpclient`.
- `infrastructure/http/server`: rotas e handlers HTTP/gRPC inbound.
- `infrastructure/jobs/handlers`: handlers de jobs registrados via `internal/platform/worker/job`.
- `infrastructure/messaging/database/consumers`: consumers locais persistidos via `internal/platform/worker/consumer`.
- `infrastructure/messaging/database/producers`: producers/event dispatchers usando `internal/platform/events`.

## Padrao Obrigatorio de Modulo

`internal/identity` e `internal/billing` DEVEM seguir DI manual explicita em `module.go`, no estilo `InvoiceModule`: construtor direto, struct concreta e campos nomeados para artefatos reais do bounded context.

Regras:
1. Construtores devem ser nomeados pelo modulo: `NewIdentityModule(...) IdentityModule` e `NewBillingModule(...) BillingModule`.
2. A struct do modulo deve expor apenas dependencias reais necessarias ao bootstrap ou a outros modulos: routers, providers, adapters, jobs e consumers.
3. Wiring segue `repository/client -> use case -> handler -> router/job/consumer/producer`.
4. Routers implementam `Register(router chi.Router)` e so sao registrados quando houver rotas reais.
5. Jobs e consumers sao entregues ao `WorkerManager` como `worker.Job` e `worker.Consumer`, sempre passando pelos adapters de `internal/platform/worker`.
6. Proibido usar `NewModule(opts...)`, `WithDatabase(...)`, `Routers()` ou `Runners()` como novo padrao de composicao.
7. Antes de criar wiring, verificar se repository, use case, handler, router, provider, job ou consumer existe no workspace.

## Regra Obrigatoria para Handlers, Consumers, Jobs e Producers

Nos bounded contexts `internal/identity` e `internal/billing`, os diretórios abaixo sao tratados como portas de entrada ou adapters outbound e devem permanecer finos:

- `infrastructure/http/server/handlers`
- `infrastructure/messaging/database/consumers`
- `infrastructure/jobs/handlers`
- `infrastructure/messaging/database/producers`

Regras obrigatorias:
1. Handlers HTTP, consumers de eventos e jobs sao apenas porta de entrada: decodificam input, chamam use case, traduzem erro/saida e encerram.
2. O fluxo permitido e sempre `handler -> usecase -> repository/service/client`.
3. Proibido implementar regra de negocio, branching de negocio, query SQL, decisao de persistencia, calculo de janela/default, orchestration cross-repository ou chamada direta a repository/client dentro desses handlers/consumers/jobs.
4. `producers` sao adapters outbound: podem serializar e publicar um evento ja decidido pela aplicacao, mas nao podem decidir regra de negocio, trigger, payload semantico ou branching de dominio.
5. Em novos desenvolvimentos, nao injetar `RepositoryFactory`, `manager.Manager`, `database.DBTX`, clients externos ou services de dominio diretamente em handlers/consumers/jobs quando o use case correspondente puder receber essa responsabilidade.
6. Em use cases e services, e proibido criar interfaces locais apenas para expor `DBTX(ctx)`; quando a dependencia for somente o handle de banco, injetar `database.DBTX` concreto na struct consumidora.

## Plataforma Compartilhada

- Capacidades tecnicas reutilizaveis por mais de um modulo devem viver em `internal/platform/`.
- `internal/platform/` nao e modulo de negocio e nao pode importar `internal/<modulo>/...`.
- Modulos podem consumir `internal/platform/` apenas nas camadas em que a dependencia tecnica seja permitida; `domain` permanece puro.
- Proibido criar implementacoes locais redundantes de capacidades transversais como geracao de IDs, hashing, workers, events, HTTP client ou outbox.
- Proibido criar ou usar pacote global de clock compartilhado (`internal/platform/clock`); tempo deve ser dependencia local.

## Worker, HTTP Outbound e Outbox

Worker:
- `WorkerManager` e o unico orquestrador de lifecycle para cron jobs e consumers.
- Todo cron job passa por `job.NewAdapter` ou `job.NewAdapterWithPolicy`.
- Todo consumer passa por `consumer.NewAdapter`.
- Modulo monta `consumer.Registry`, registra handlers uma vez e passa para `consumer.NewRunner`.
- Handlers de modulo delegam para use cases da camada `application`; nunca acessam repositories diretamente.
- Shutdown deve cancelar contexto raiz, drenar execucoes em voo e respeitar `ShutdownTimeout`.

HTTP outbound:
- Toda chamada externa (Kiwify, LLMs, notificacao etc.) deve usar `internal/platform/httpclient`.
- Proibido em producao: `&http.Client{}` direto, `devkit-go/pkg/httpclient.NewObservableClient` fora do wrapper, ou chamada outbound sem timeout explicito.
- Rate limit, OAuth e retry especifico ficam acima do wrapper, no client da integracao.

Outbox:
- Use `outbox.Publisher` (`internal/platform/outbox`) para side-effects que precisam sobreviver a crash, deploy ou reinicio.
- Todo handler de outbox deve ser idempotente por `event.ID`; entrega e at-least-once.

## Modo de Trabalho

1. Entender o contexto antes de editar.
2. Preferir a menor mudanca segura que resolve a causa raiz.
3. Preservar arquitetura, convencoes e fronteiras existentes.
4. Nao introduzir abstracoes, camadas ou dependencias sem demanda concreta.
5. Atualizar/adicionar testes quando houver mudanca de comportamento.
6. Rodar validacoes proporcionais ao risco.
7. Registrar bloqueios, drift e suposicoes explicitamente.
8. Zero comentarios em codigo Go de producao [HARD] — inegociavel. Excecoes: `// Code generated` (gerado por ferramenta), diretivas `//go:` e `//nolint:` com justificativa na mesma linha.

## Validacao

Antes de concluir uma alteracao, seguir `.agents/skills/agent-governance/SKILL.md` e os gates da skill especifica.

Comandos Go detectados:
- Formatacao: `gofmt -w <arquivos-go-alterados>`.
- Testes direcionados: `go test -race -count=1 ./<pacote-alterado>/...`.
- Testes amplos quando proporcional: `go test ./...`.
- Lint: `golangci-lint run` ou escopo equivalente quando disponivel.
- Build/vet: `go build` e `go vet` no escopo alterado quando aplicavel.

## Ferramentas

- Claude Code: `CLAUDE.md` delega para este arquivo; `.claude/skills/` aponta para `.agents/skills/`.
- Codex: deve seguir este arquivo como instrucao canonica.
- Copilot: `.github/copilot-instructions.md` so pode reforcar este arquivo.
- Gemini: `GEMINI.md`/commands so podem reforcar este arquivo.
- Ferramentas sem enforcement automatico continuam obrigadas a cumprir estas regras de forma procedural.

## Restricoes Finais

1. Nao assumir versao de linguagem, framework ou runtime sem verificar localmente.
2. Nao alterar comportamento publico sem explicitar.
3. Nao usar exemplos como copia cega.
4. Nao reverter mudancas do usuario.
5. Se houver conflito entre regra generica e convencao real segura do projeto, seguir a opcao mais segura e registrar a decisao.
6. Skills que invocam outras skills devem respeitar `scripts/lib/check-invocation-depth.sh` ou equivalente em `.agents/lib/`, com limite padrao 2.
