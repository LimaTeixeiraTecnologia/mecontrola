<!-- governance-schema: 1.0.0 -->
# Regras para Agentes de IA

Este diretorio centraliza regras para uso com agentes de IA em tarefas reais de analise, alteracao e validacao de codigo.

## Objetivo

Use estas instrucoes para manter consistencia, seguranca e qualidade ao trabalhar com codigo, configuracao, validacao e evolucao de sistemas.

## Arquitetura: monolito modular

O projeto aparenta ser um monolito modular, com separacao relevante por modulos, dominios ou componentes internos. A governanca deve proteger essas fronteiras e evitar dependencias circulares.

Stack detectada: Go.
Frameworks detectados: Fiber, gRPC.

## Go — Regra Mandatória e Inegociável

Toda implementação, alteração ou revisão de código Go DEVE obrigatoriamente seguir o protocolo completo de `.agents/skills/go-implementation/SKILL.md`. Nenhuma exceção por diferença de ferramenta, agente ou conveniência operacional.

### Protocolo obrigatório (Etapas 1–5)

**Etapa 1 — Base obrigatória (sempre):**
- Carregar `.agents/skills/go-implementation/SKILL.md`.
- Ler `references/architecture.md`.
- Verificar versão em `go.mod` antes de usar qualquer API ou dependência nova.
- Aplicar todas as Regras Estritas R0–R7 (`[HARD]`, bloqueantes de merge).

**Etapa 2 — Referências por gatilho (carregar apenas o necessário):**

| Gatilho da tarefa | Referência obrigatória |
|---|---|
| Interfaces, construtores, DI, fronteiras | `references/interfaces.md` |
| Generics, constraints, componentes reutilizáveis | `references/generics.md` |
| Goroutines, channels, cancelamento, worker pools | `references/concurrency.md` |
| Strategy, observer, máquina de estado | `references/patterns-behavioral.md` |
| Logging, tracing, métricas, health checks | `references/observability.md` |
| Handlers HTTP/gRPC, middlewares, DTOs | `references/api.md` |
| Repositories, transactions, migrations, queries | `references/persistence.md` |
| Configuração, env vars, bootstrap de dependências | `references/configuration.md` |
| Retries, circuit breakers, timeouts externos | `references/resilience.md` |
| Mensagens, filas, outbox, idempotência | `references/messaging.md` |
| Autenticação, autorização, segredos, rate limit | `references/security.md` |
| Estratégia de testes, testcontainers, cobertura | `references/testing.md` |
| Fluxo end-to-end (domain → service → handler → test) | `references/examples-domain-flow.md` |
| Exemplos de fuzz, table-driven, invariantes | `references/examples-testing.md` |
| Graceful shutdown, paginação, versionamento de API | `references/examples-infrastructure.md` |
| Dockerfile, CI, build flags, gates de qualidade | `references/build.md` |
| Startup ordenado, drain, shutdown de goroutines | `references/graceful-lifecycle.md` |

**Economia de contexto (obrigatório):**
- Máximo 4 referências simultâneas por tarefa. Se mais de 4 forem necessárias, priorizar as 3 mais críticas e registrar as demais como contexto não carregado.
- Nunca carregar `references/patterns-structural.md` para Factory Function, Functional Options, Adapter, Decorator ou Facade — esses patterns já estão inline no SKILL.md (~960 tokens economizados).
- Nunca carregar referências de domínios não afetados pela mudança.

**Etapa 3 — Modelar antes de escrever:**
- Identificar fronteiras arquiteturais afetadas.
- Confirmar que a interface fica no consumidor (não no produtor).
- Confirmar que nenhuma camada viola o fluxo `infrastructure → application → domain`.

**Etapa 4 — Implementar:**
- Adaptar exemplos ao contexto real — nunca replicar literalmente.
- Aplicar R0–R7 durante a escrita, não só na revisão.

**Etapa 5 — Validar (proporcional ao risco):**
- Executar Checklist R0–R7 de `references/build.md` e reportar resultado.
- Mínimo: `go build`, `go vet`, `go test -race -count=1` no pacote alterado.
- Lint: `golangci-lint run` no escopo da mudança.
- Registrar riscos residuais e suposições assumidas.

### Regras de robustez transversais (resumo executivo)

- **R0** — `init()` proibida em produção.
- **R1** — toda função deve ser método de struct (exceto `main`, factory, helpers de teste).
- **R5.8** — enums com `iota+1`; zero value reservado para "não inicializado".
- **R5.10** — erros: `errors.New` (estático), `fmt.Errorf("ctx: %w", err)` (wrapping), `var errX = errors.New(...)` (sentinel). Tratar erro uma única vez.
- **R5.12** — sem `panic` em produção.
- **R5.26** — globais não exportados em camelCase; sem prefixo `_`.
- **R6** — `context.Context` em toda fronteira de IO; DI via construtores explícitos.
- **R6.4** — `var _ Interface = (*Type)(nil)` proibido.
- **R6.7** — `clock.Clock` proibido em use cases e repositórios; `time.Now().UTC()` inline.
- **R7.1** — `any` em vez de `interface{}`.
- **R7.2** — `log/slog` para logging estruturado.
- **R7.6** — `errors.Join` para agregar erros.
- Goroutines: sempre canceláveis via `context.Context`, sem leak, participam do shutdown coordenado.

## Estrutura de Pastas

```
.
.ai_spec_harness.json
.editorconfig
.env.example
.github
.github/agents
.github/agents/bugfix.agent.md
.github/agents/prd-writer.agent.md
.github/agents/project-analyzer.agent.md
.github/agents/refactorer.agent.md
.github/agents/reviewer.agent.md
.github/agents/task-executor.agent.md
.github/agents/task-planner.agent.md
.github/agents/technical-specification-writer.agent.md
.github/copilot-instructions.md
.github/dependabot.yml
.github/hooks
.github/hooks/governance.json
.github/hooks/post-execute-task.sh
.github/hooks/post-wave.sh
.github/hooks/pre-execute-all-tasks.sh
.github/hooks/subagent-stop-wrapper.sh
.github/hooks/validate-governance.sh
.github/hooks/validate-preload.sh
.github/skills
.github/skills/agent-governance
.github/skills/agent-governance/SKILL.md
.github/skills/agent-governance/references
.github/skills/agent-governance/references/bug-schema.json
.github/skills/agent-governance/references/ddd.md
.github/skills/agent-governance/references/enforcement-matrix.md
.github/skills/agent-governance/references/error-handling.md
.github/skills/agent-governance/references/messaging.md
.github/skills/agent-governance/references/multiple-choice-protocol.md
.github/skills/agent-governance/references/observability.md
.github/skills/agent-governance/references/persistence.md
.github/skills/agent-governance/references/security-app.md
.github/skills/agent-governance/references/security.md
.github/skills/agent-governance/references/severity-mapping.md
.github/skills/agent-governance/references/shared-architecture.md
.github/skills/agent-governance/references/shared-lifecycle.md
.github/skills/agent-governance/references/shared-patterns.md
.github/skills/agent-governance/references/shared-testing.md
.github/skills/agent-governance/references/testing.md
.github/skills/agent-governance/scripts
.github/skills/agent-governance/scripts/detect-architecture.sh
.github/skills/agent-governance/scripts/detect-toolchain.sh
.github/skills/agent-governance/triggers
.github/skills/agent-governance/triggers/go.yaml
.github/skills/agent-governance/triggers/node.yaml
.github/skills/agent-governance/triggers/python.yaml
.github/skills/analyze-project
.github/skills/analyze-project/SKILL.md
.github/skills/analyze-project/assets
.github/skills/analyze-project/assets/agents-template.md
.github/skills/analyze-project/assets/ai-tool-template.md
.github/skills/analyze-project/scripts
.github/skills/analyze-project/scripts/generate-governance.sh
.github/skills/analyze-project/scripts/lib
.github/skills/analyze-project/scripts/lib/codex-config.sh
.github/skills/analyze-project/scripts/lib/find-manifests.sh
.github/skills/bugfix
.github/skills/bugfix/SKILL.md
.github/skills/bugfix/assets
.github/skills/bugfix/assets/bugfix-report-template.md
.github/skills/bugfix/references
.github/skills/bugfix/references/canonical-bug-format.md
.github/skills/bugfix/scripts
.github/skills/bugfix/scripts/validate-bug-input.py
.github/skills/confluence-changelog-publisher
.github/skills/confluence-changelog-publisher/SKILL.md
.github/skills/create-prd
.github/skills/create-prd/SKILL.md
.github/skills/create-prd/assets
.github/skills/create-prd/assets/prd-template.md
.github/skills/create-tasks
.github/skills/create-tasks/SKILL.md
.github/skills/create-tasks/assets
.github/skills/create-tasks/assets/task-template.md
.github/skills/create-tasks/assets/tasks-template.md
```

## Padrao Arquitetural

Predominio de packages internos coesos, com estrutura orientada por dominio ou componente.

### Fluxo de Dependencias

- Transporte e infrastructure devem depender de casos de uso ou servicos explicitos, nao do contrario.
- Dominio nao deve conhecer detalhes de HTTP, banco, filas, serializacao ou drivers.
- Infraestrutura pode implementar contratos consumidos pela aplicacao, preservando dependencia para dentro.

### Layout Obrigatorio por Modulo

Novos modulos, novas features e refatoracoes estruturais em bounded contexts DEVEM usar separacao fisica clara por responsabilidade:

```text
internal/<modulo>/
  application/
    dtos/
      input/
      output/
    usecases/
    interfaces/
  domain/
    entities/
    valueobjects/
    services/
    interfaces/
  infrastructure/
    jobs/
      handlers/
    messaging/
      database/
        producers/
        consumers/
      kafka/
        producers/
        consumers/
      rabbit/
        producers/
        consumers/
      nats/
        producers/
        consumers/
    repositories/
      postgres/
      mssql/
    http/
      server/
      client/
```

Responsabilidades obrigatorias:

1. `application/`: orquestracao de use cases, DTOs de entrada em `dtos/input`, DTOs de saida em `dtos/output` e interfaces/contratos consumidos pela aplicacao. Nao pode conter IO concreto, drivers, SDKs externos, SQL, brokers ou handlers HTTP.
2. `domain/`: entidades, value objects, domain services stateless, invariantes e interfaces/contratos que pertencem a linguagem ubiqua do dominio. Nao pode conhecer application, infrastructure, serializacao, banco, HTTP, filas ou configuracao.
3. `infrastructure/`: implementacoes concretas de IO e integracoes por tecnologia. Implementa contratos definidos em `application/` ou `domain/`, sem vazar tipos concretos para dentro do dominio.
4. `infrastructure/http/client`: toda e qualquer chamada HTTP outbound para APIs externas. Deve usar `internal/platform/httpclient`.
5. `infrastructure/http/server`: rotas e handlers HTTP/gRPC inbound providos pelo modulo.
6. `infrastructure/jobs/handlers`: handlers de jobs registrados pelo modulo via `internal/platform/worker/job`.
7. `infrastructure/messaging/database/consumers`: consumers do transporte local persistido registrados via `internal/platform/worker/consumer`.
8. `infrastructure/messaging/database/producers`: producers/event dispatchers internos usando `internal/platform/events`.
9. `infrastructure/messaging/<broker>/producers`: publicacao/envio de mensagens para Kafka, Rabbit ou NATS quando houver broker externo.
10. `infrastructure/messaging/<broker>/consumers`: consumo/processamento de mensagens vindas de Kafka, Rabbit ou NATS quando houver broker externo.
11. `infrastructure/repositories/postgres` e `infrastructure/repositories/mssql`: persistencia concreta por banco.

### Padrao Obrigatorio de Modulo

`internal/identity` e `internal/billing` DEVEM seguir DI manual explicita em `module.go`, no estilo `InvoiceModule`: construtor direto, struct concreta e campos nomeados para os artefatos reais do bounded context.

Regras obrigatorias:

1. O construtor deve ser nomeado pelo modulo, como `NewIdentityModule(...) IdentityModule` ou `NewBillingModule(...) BillingModule`.
2. A struct do modulo deve expor apenas dependencias reais necessarias ao bootstrap ou a outros modulos, como routers, providers, adapters, jobs e consumers.
3. O wiring deve seguir a ordem concreta `repository/client -> use case -> handler -> router/job/consumer/producer`, adaptada ao contexto real.
4. Routers devem implementar `Register(router chi.Router)` e ser registrados no servidor central apenas quando existirem rotas reais.
5. Jobs e consumers devem ser entregues ao `WorkerManager` como `worker.Job` e `worker.Consumer`, sempre passando por `internal/platform/worker/job` e `internal/platform/worker/consumer`.
6. Proibido criar campos, handlers, routers, jobs, consumers, adapters ou providers ficticios para preencher estrutura.
7. Proibido usar `NewModule(opts...)`, `WithDatabase(...)`, `Routers()` ou `Runners()` como novo padrao de composicao de modulo.
8. Antes de criar wiring, verificar se repository, use case, handler, router, provider, job ou consumer existe no workspace. Se nao existir, registrar drift ou lacuna explicitamente em vez de inventar implementacao.
9. Exemplos externos definem o formato arquitetural, mas imports, nomes, dependencias e contratos devem ser adaptados ao estado real deste repositorio.

### Plataforma Tecnica Compartilhada

Capacidades tecnicas reutilizaveis por mais de um modulo DEVEM viver em `internal/platform/`, mantendo visibilidade privada do monolito e evitando `pkg/` sem necessidade de consumo externo.

**Regra de Unicidade:** É PROIBIDO criar implementações locais ou redundantes de capacidades transversais (ex: geração de IDs, clock, hashing) dentro de `internal/<modulo>/...`. Se uma capacidade for necessária em múltiplos contextos, ela deve ser promovida ou criada diretamente em `internal/platform/`.

```text
internal/platform/
  clock/
  database/
  errors/
  http/
  httpclient/
  id/
  observability/
  outbox/
  worker/
    types.go             — interfaces Job e Consumer
    config.go            — Config{ShutdownTimeout}
    errors.go            — erros sentinela de lifecycle
    manager.go           — WorkerManager: unico ponto de start/stop/shutdown
    job/
      types.go           — OverlapPolicy (Skip|Allow)
      adapter.go         — JobAdapter: unico caminho valido para cron jobs
      scheduler.go       — scheduler interno via robfig/cron/v3
    consumer/
      types.go           — Handler, HandlerFunc, Message, Source, Runner
      registration.go    — Registration{Name, EventType, Handler}
      registry.go        — Registry agnostico: Register + Dispatch
      runner.go          — NewRunner(Source, Registry, logger)
      adapter.go         — ConsumerAdapter: unico caminho valido para consumers
      database/
        adapter.go       — adapter para transporte persistido via outbox/banco
  secrets/
```

`internal/platform/` nao e modulo de negocio e nao pode importar `internal/<modulo>/...`. Modulos podem consumir `internal/platform/` apenas nas camadas em que a dependencia tecnica seja permitida pela fronteira arquitetural; `domain` permanece puro e nao pode importar `platform`, banco, HTTP, filas, serializacao, configuracao ou drivers.

### Worker — Orquestração de Jobs e Consumers

O `WorkerManager` e o unico ponto de lifecycle para cron jobs e consumers. Nenhum modulo pode inicializar cron job ou consumer fora do manager.

**Contrato de bootstrap obrigatorio:**

```go
jobs := []worker.Job{
    job.NewAdapter("faturamento-sync", cfg.CronFaturamentoSync, syncJob.Execute),
}

consumers := []worker.Consumer{
    consumer.NewAdapter("faturamento-db", "database",
        consumer.NewRunner(dbSource, registry, logger)),
}

manager := worker.NewManager(
    worker.Config{ShutdownTimeout: 30 * time.Second},
    jobs,
    consumers,
    logger,
)

if err := manager.Start(ctx); err != nil {
    return err
}
```

**Regras obrigatorias:**

1. Todo cron job DEVE passar por `job.NewAdapter` ou `job.NewAdapterWithPolicy`.
2. Todo consumer DEVE passar por `consumer.NewAdapter`.
3. O `WorkerManager` DEVE ser o unico orquestrador de lifecycle — `Start` e `Stop` sao os unicos pontos de entrada.
4. O modulo constroi um `consumer.Registry`, registra os handlers uma unica vez e passa o registry para o `consumer.NewRunner`.
5. O `consumer/database.NewAdapter` e o adapter para o transporte local persistido via outbox/banco. Ele recebe uma `consumer.Source` injetada pelo modulo — sem reimplementar o outbox.
6. Quando RabbitMQ ou Kafka forem adotados, cada tecnologia adiciona um subpacote em `consumer/<tecnologia>/` expondo `NewAdapter(name, runner)` sem alterar handlers nem registry.
7. Handlers de modulo DEVEM delegar para use cases da camada `application` — nunca acessar repositories diretamente.

**Contrato de handler:**

```go
type Handler interface {
    Handle(ctx context.Context, params map[string]string, body []byte) error
}
```

**Politica de overlap de jobs:**

- `OverlapSkip` (padrao): execucao anterior ainda em curso e ignorada com log `slog.Warn`.
- `OverlapAllow`: execucoes paralelas permitidas (responsabilidade do job ser thread-safe).

**Shutdown gracioso:** `Stop(ctx)` cancela o contexto raiz, drena execucoes em voo via `scheduler.Stop()`, chama `consumer.Stop` em paralelo e aguarda todas as goroutines dentro de `ShutdownTimeout`. Retorna `errStopTimeout` explicitamente se o timeout expirar.

### HTTP Outbound Mandatório

Toda chamada HTTP a APIs externas (Kiwify, LLMs, provedores de notificação, etc.)
DEVE usar `internal/platform/httpclient` como wrapper sobre `devkit-go/pkg/httpclient`.
O wrapper garante timeouts explícitos, tracing W3C, métricas automáticas e política
segura de retry (apenas métodos idempotentes por padrão).

**Padrão de uso (bootstrap do módulo):**

```go
client, err := platformhttpclient.NewClient(
    provider.Observability(),
    platformhttpclient.WithTimeout(cfg.HTTPTimeout),
    platformhttpclient.WithBaseURL(cfg.APIBaseURL),
    platformhttpclient.WithDefaultRetry(cfg.HTTPRetryMaxAttempts, cfg.HTTPRetryBackoff),
    platformhttpclient.WithTarget("kiwify"),
)
```

**Proibido fora de testes:**

1. Instanciar `&http.Client{}` diretamente em código de produção.
2. Chamar `devkit-go/pkg/httpclient.NewObservableClient` sem passar por este wrapper.
3. Realizar requisições HTTP outbound sem timeout explícito.

Camadas auxiliares (rate limit, fluxo OAuth, retry específico de 429) ficam acima
do wrapper, dentro do client da integração em `internal/<modulo>/infrastructure/http/client/`.

### Proibições e Padrões desencorajados

1. **Proibido o uso de pacotes de Clock globais ou compartilhados:** Não criar ou utilizar pacotes como `internal/platform/clock`. O tempo deve ser tratado como uma dependência local.
   - **Use Cases/Domain:** Injetar `now func() time.Time`.
   - **Infrastructure:** Declarar interface `Clock` local e privada se necessário (Interface Segregation).
   - **Motivo:** Reduzir acoplamento desnecessário e "ruído" em camadas de domínio.

Os modulos ativos devem usar `infrastructure/` como camada fisica de implementacoes concretas. Nao criar diretorios alternativos para a mesma responsabilidade.

Fluxo permitido: `infrastructure -> application -> domain`. `application` pode importar `domain`, mas nao pode importar `infrastructure`. `domain` nao pode importar nenhuma camada externa. Comunicacao cross-module deve ocorrer apenas por interface declarada pelo consumidor, domain event/outbox ou contrato explicito.

## Modo de trabalho

1. Entender o contexto antes de editar qualquer arquivo.
2. Preferir a menor mudanca segura que resolva a causa raiz.
3. Preservar arquitetura, convencoes e fronteiras ja existentes no contexto analisado.
4. Nao introduzir abstracoes, camadas ou dependencias sem demanda concreta.
5. Atualizar ou adicionar testes quando houver mudanca de comportamento.
6. Rodar validacoes proporcionais a mudanca.
7. Registrar bloqueios e suposicoes explicitamente quando o contexto estiver incompleto.

## Diretrizes de Estrutura

1. Priorize entendimento do codigo e do contexto atual antes de propor refatoracoes.
2. Respeite padroes existentes de nomenclatura, organizacao e tratamento de erro.
3. Defina estrutura simples, evolutiva e com defaults explicitos.
4. Evite reescritas amplas quando uma alteracao localizada resolver o problema.
5. Estabeleca contratos, testes e comandos de validacao cedo quando eles ainda nao existirem.
6. Considere risco de regressao como restricao principal.
7. Evite overengineering disfarcado de arquitetura futura.
8. Elimine comentarios por padrao. Comentarios so devem existir quando forem extremamente relevantes para explicar decisao, invariante, risco, contrato publico ou comportamento nao obvio.
9. Proiba comentarios mecanicos, redundantes ou que apenas repitam nomes de funcoes, structs, campos ou fluxo evidente.

## Regras por Arquitetura

1. Respeitar fronteiras entre modulos e bounded contexts.
2. Evitar dependencia circular entre packages internos.
3. Nao extrair shared helpers sem demanda comprovada de mais de um modulo.

## Regras por Linguagem

Para tarefas que alteram codigo, carregar a skill:

- `.agents/skills/agent-governance/SKILL.md`

Para tarefas que alteram codigo Go, carregar tambem:

- `.agents/skills/go-implementation/SKILL.md`

O uso de `.agents/skills/go-implementation/SKILL.md` e mandatorio para qualquer alteracao em codigo Go. Seguir seus exemplos como referencia de padrao, adaptando ao contexto real do modulo e sem copia cega. Aplicar economia de contexto: carregar somente referencias necessarias pelos gatilhos e pela complexidade da tarefa, preferindo TL;DR quando suficiente.

Para tarefas de revisao ou refatoracao incremental de design em Go guiadas por heuristicas de object calisthenics, carregar tambem:

- `.agents/skills/object-calisthenics-go/SKILL.md`

Para tarefas de correcao de bugs com remediacao e teste de regressao, carregar tambem:

- `.agents/skills/bugfix/SKILL.md`

### Composicao Multi-Linguagem

Em projetos com mais de uma linguagem (ex: monorepo Go + Node), carregar apenas a skill da linguagem afetada pela mudanca. Se a tarefa cruzar linguagens, carregar ambas e aplicar a validacao de cada stack nos arquivos correspondentes. Nao misturar convencoes de uma linguagem em arquivos de outra.

## Referencias

Cada skill lista suas proprias referencias em `references/` com gatilhos de carregamento no respectivo `SKILL.md`. Nao duplicar a listagem aqui — consultar o SKILL.md da skill ativa para saber quais referencias carregar e em que condicao.

## Notas por Ferramenta

- **Claude Code**: skills pre-carregadas via `.claude/skills/`, hooks via `.claude/hooks/`, agents delegados via `.claude/agents/`.
- **Gemini CLI**: commands em `.gemini/commands/*.toml` apontam para skills canonicas. Sem hooks ou agents nativos — o modelo deve seguir as instrucoes procedurais do SKILL.md carregado.
- **Codex**: le `AGENTS.md` como instrucao de sessao. Entradas em `.codex/config.toml` sao metadados para `upgrade.sh`, nao spec oficial do Codex CLI. O agente deve seguir as instrucoes de `AGENTS.md` para descobrir e carregar skills.
- **Copilot**: `.github/copilot-instructions.md` como instrucao principal. `.github/agents/` sao wrappers. Sem hooks nativos — compliance depende do modelo seguir as instrucoes.

### Obrigatoriedade por Ferramenta

Codex e Claude DEVEM respeitar TODAS as regras, skills, references, validacoes, restricoes de arquitetura, regras de economia de contexto e politicas de comentarios de forma igualitaria. Isso e obrigatorio e inegociavel.

Nenhuma ferramenta pode flexibilizar regras por ausencia de enforcement automatico, diferenca de runtime, hooks, agentes pre-carregados ou conveniencia operacional. Quando uma ferramenta nao tiver enforcement programatico, a compliance deve ser procedural e igualmente obrigatoria.

Se houver divergencia entre arquivos especificos de ferramenta e `AGENTS.md`, prevalece `AGENTS.md`. Arquivos como `CLAUDE.md`, `GEMINI.md` e `.github/copilot-instructions.md` so podem reforcar ou apontar para a fonte canonica, nunca enfraquecer regras.

### Matrix de Enforcement

| Capacidade | Claude Code | Gemini CLI | Codex | Copilot |
|---|---|---|---|---|
| Carga base automatica | hook PreToolUse | procedural | procedural | procedural |
| Protecao de governanca | hook PostToolUse | procedural | procedural | procedural |
| Skills pre-carregadas | sim (symlinks) | sim (commands) | nao | sim (agents) |
| Enforcement programatico | sim (hooks) | nao | nao | nao |
| Validacao de evidencias | script | procedural | procedural | procedural |

Ferramentas sem enforcement programatico dependem do modelo seguir instrucoes procedurais. A compliance nessas ferramentas continua obrigatoria.

## Economia de Contexto

Carregar o minimo necessario para a tarefa reduz custo de tokens em 35-50%:

| Complexidade | Criterio | O que carregar |
|---|---|---|
| `trivial` | Rename, typo, import, formatacao | Apenas AGENTS.md |
| `standard` | Bug fix, novo metodo, refactor local | AGENTS.md + TL;DR das references afetadas |
| `complex` | Nova feature, interface publica, migracao | AGENTS.md + referencias completas |

- Classificar a complexidade **antes** de carregar qualquer referencia.
- Quando a reference tiver bloco `<!-- TL;DR ... -->`, preferir o TL;DR ao documento completo em tarefas standard.
- Override explicito via `--complexity=<nivel>` prevalece sobre classificacao automatica.

## Validacao

Antes de concluir uma alteracao:

Seguir Etapa 4 de `.agents/skills/agent-governance/SKILL.md` como base canonica.

Comandos detectados no projeto (Go):
1. Rodar fmt: `gofmt -w .`.
2. Rodar test: `go test ./...`.
3. Rodar lint: `golangci-lint run`.

## Outbox

<!-- RF-38 / ADR-016 — contrato do outbox.Publisher -->

Use `outbox.Publisher` (`internal/platform/outbox`) para todo side-effect que precisa ser entregue mesmo apos crash, deploy ou reinicio: notificacoes, projecoes persistentes, integracoes externas disparadas pos-commit. O Publisher garante at-least-once escrevendo atomicamente na transacao do agregado.

**Regra obrigatoria de idempotencia:** Todo handler de outbox DEVE ser idempotente por `event.ID`. O Dispatcher entrega at-least-once; o handler e responsavel por evitar duplicacao via upsert ou tabela de deduplicacao.

## Restricoes

1. Nao inventar contexto ausente.
2. Nao assumir versao de linguagem, framework ou runtime sem verificar.
3. Nao alterar comportamento publico sem deixar isso explicito.
4. Nao usar exemplos como copia cega; adaptar ao contexto real.


### Controle de profundidade de invocacao

- Skills que invocam outros skills (execute-task, refactor) devem verificar profundidade via `scripts/lib/check-invocation-depth.sh`.
- Limite padrao: 2 niveis. Configuravel via `AI_INVOCATION_MAX`.
- Variaveis de ambiente: `AI_INVOCATION_DEPTH` (corrente), `AI_INVOCATION_MAX` (limite).
