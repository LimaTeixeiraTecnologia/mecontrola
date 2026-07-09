<!-- governance-schema: 1.0.0 -->
# Regras para Agentes de IA

`AGENTS.md` e a fonte canonica deste repositorio. Arquivos especificos de ferramenta (`CLAUDE.md`, `GEMINI.md`, `.github/copilot-instructions.md`, `.codex/config.toml`) podem reforcar estas regras, nunca enfraquecer.

Objetivo: manter consistencia, seguranca, economia de contexto e qualidade em tarefas reais de analise, alteracao e validacao de codigo.

## Contexto do Projeto

- Arquitetura: monolito modular em Go, com bounded contexts em `internal/`.
- Stack detectada: Go; HTTP com Chi/Fiber no historico de dependencias; gRPC em dependencias.
- Modulos ativos: `internal/identity`, `internal/billing`, `internal/platform`, `internal/categories`, `internal/budgets`.
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

## Prompt Engineering, Tool Use e Agentes

Esta secao traduz para o repositorio as praticas oficiais do [Claude Platform](https://platform.claude.com/docs/en/overview) e [OpenAI Developers](https://developers.openai.com/) em 2026. Sao regras duras para todo agente/claude/codex que operar neste codebase.

### Prompt Engineering

1. **XML tags para prompts complexos** `[HARD]`: toda instrucao multi-parte deve usar tags explicitas (`<context>`, `<task>`, `<rules>`, `<example>`, `<format>`, `<output>`). Multi-parte = qualquer prompt com duas ou mais dimensoes distintas (contexto + tarefa, tarefa + regras, formato + exemplo, etc.). Prompts de uma unica dimensao (ex.: "qual a capital da Franca?") nao exigem XML. Nunca misturar contexto, constraints e formato em prosa solta.
2. **Goal-statement primeiro**: o objetivo vem antes de qualquer contexto; a descricao de "done" deve caber em uma frase.
3. **Constraints explicitos em bullets**: regras negativas e positivas como lista, nao como paragrafo.
4. **Exemplos quando o formato importa**: incluir 1-2 exemplos de input/output quando a saida for nao trivial.
5. **Uncertainty rule**: "se incerto, diga explicitamente" — proibido inventar resposta quando a evidencia for insuficiente.
6. **Chain-of-thought explicito**: para raciocinio critico, instruir o modelo a expor o raciocinio antes da conclusao. Raciocinio critico = decisoes de arquitetura, seguranca, concorrencia, persistencia, API publica, transacao, modelagem de dominio ou escolha de pattern.
7. **Prompts pequenos e encadeados**: decompor workflows grandes em prompts focados (parse → validate → decide → persist → publish), nao um mega-prompt. Mega-prompt = qualquer prompt com mais de 5 dimensoes instrucionais distintas ou que exija mais de uma decisao de saida.
8. **Diga o que fazer, nao o que evitar**: instrucao positiva ("responda em prosa corrida") supera a negativa ("nao use markdown") para steering de formato e comportamento.
9. **Ordem para long-context**: em entradas grandes (>=20k tokens), colocar os dados longos no topo e a instrucao/pergunta no fim — melhora a qualidade em ate 30% em entradas multi-documento; pedir ao modelo para ancorar em quotes relevantes antes de agir.
10. **Papel no system prompt**: definir papel/persona no system prompt foca comportamento e tom, mesmo em uma unica frase.

### Tool Use / Function Calling

1. **JSON Schema estrito** `[HARD]`: toda tool/functao exposta a um LLM deve ter schema JSON com `additionalProperties: false` e todos os campos em `required`.
2. **Strict mode obrigatorio**: usar `strict: true` sempre que a API/modelo suportar (OpenAI function calling / structured outputs).
3. **Descricoes precisas**: nome, descricao e parametros da tool devem passar no "intern test" — um humano deve conseguir usa-la so com o que foi fornecido.
4. **Poucas tools por turno**: manter menos de 20 tools ativas por turno; a partir de ~10, agrupar por dominio (`crm`, `billing`, `shipping`) e usar tool search / deferred loading para carregar sob demanda.
5. **Validar input da tool**: antes de delegar ao use case/client, validar o payload contra o schema.
6. **Tool e adapter fino**: herda R-ADAPTER-001.2 — toda tool delega para use case/client; nunca contem regra de negocio, SQL direto ou branching de dominio.
7. **Structured Outputs para saida conformada** `[HARD]`: quando o contrato LLM↔porta exige JSON estruturado, usar Structured Outputs (schema estrito) — nao prefill. Prefill de resposta foi descontinuado nos modelos Claude 4.6+; migrar para Structured Outputs ou tools com enum, pedindo conformidade + retry.
8. **Enums fechados para estados ilegais irrepresentaveis** `[HARD]`: campos de decisao ou label no schema da tool devem ser enums, nunca `string` livre — sinergia direta com DMMF state-as-type e smart constructors. Estado ilegal nao deve caber no schema.
9. **Valores conhecidos via codigo, nao via modelo**: nunca pedir ao LLM para preencher parametro que o sistema ja conhece (session id, principal, tenant); injeta-los no `exec` da tool. Reduz custo, erro e superficie de alucinacao.
10. **Tratar refusal e stop_reason** `[HARD]`: checar `refusal` (OpenAI) e `stop_reason`/`finish_reason=length` (Claude) antes de consumir o payload; degradar com erro tipado. Proibido assumir resposta completa sem verificar o motivo de parada.
11. **`tool_choice` e chamadas paralelas**: forcar tool via `tool_choice` quando a tarefa exige ferramenta; permitir tool calls paralelas quando as chamadas forem independentes entre si.

### Agent Design e Observabilidade

1. **Subagentes para economia de contexto**: investigacao que leia >=10 arquivos ou >20 tool calls deve ser delegada a subagente (`Agent`, `Explore`, `Plan`) — apenas a conclusao retorna a sessao principal.
2. **Registry em vez de switch**: comportamento novo entra como nova tool/agente/workflow registrado; proibido `switch case intent.Kind` (herda R-AGENT-WF-001.1).
3. **Cada run e rastreavel** `[HARD]`: todo tool call/agent run deve registrar `run_id`, `thread_id`, `agent_id`, `status`, `duration_ms` e `error` quando houver.
4. **Golden sets e thresholds**: mudancas em agentes/workflows so passam quando houver eval com golden set e thresholds por axis (corretude, regressao, seguranca).
5. **Verificacao antes de parar**: o agente deve ter um check pass/fail (teste, build, linter, diff contra fixture) antes de declarar a tarefa concluida.
6. **Economia de tokens e custo**: preferir prefixos de prompt estaveis para habilitar prompt caching quando o provider suportar; manter saida concisa; reservar Structured Outputs para o contrato LLM↔porta; usar processamento batch/flex para cargas nao interativas quando o provider oferecer.

## Skills Obrigatorias

- Para qualquer tarefa que altere codigo: carregar `.agents/skills/agent-governance/SKILL.md`.
- Para qualquer implementacao, alteracao ou revisao de codigo Go: carregar tambem `.agents/skills/go-implementation/SKILL.md`.
- Para escolha, aplicacao ou revisao de design patterns: carregar `.agents/skills/design-patterns-mandatory/SKILL.md` (parte do Trio Obrigatorio de Desenvolvimento Go).
- Para modelagem de dominio, discovery de fluxo ou revisao de agregados/eventos/comandos: carregar `.agents/skills/domain-modeling-production/SKILL.md` (parte do Trio Obrigatorio de Desenvolvimento Go).
- `[HARD]` Para criacao, revisao ou correcao de uso de PostgreSQL estrutural ou de acesso: carregar `.agents/skills/postgresql-production-standards/SKILL.md` e inegociavel. Gatilhos obrigatorios: migration, nova tabela, nova coluna, novo indice, novo constraint, alteracao de role/grant ou mudanca de schema. Queries ad-hoc, tuning ou transacoes isoladas entram apenas quando houver evidencia de impacto estrutural ou de desempenho critico; fora dos gatilhos, o uso e proibido.
- Para bugfix com remediacao e teste de regressao: carregar `.agents/skills/bugfix/SKILL.md`.
- Para refatoracao incremental de design Go por object calisthenics: carregar `.agents/skills/object-calisthenics-go/SKILL.md`.
- Para governanca do projeto: usar `.agents/skills/analyze-project/SKILL.md`.

### Trio Obrigatorio de Desenvolvimento Go `[HARD]`

Toda feature nova e toda manutencao em Go DEVE ser governada, sem flexibilidade, pelo trio abaixo. As tres skills ja estao instaladas em `.agents/skills/` e espelhadas em `.claude/skills/`. Modelo de uso: **consultar sempre, materializar por gatilho** — o trio nunca e pulado por diferenca de ferramenta, deadline ou conveniencia; a materializacao pesada (seletor, bundle, rodadas de clarificacao) so ocorre quando o gatilho correspondente e atingido, preservando a economia de contexto.

1. `go-implementation` — SEMPRE carregada em qualquer alteracao Go (Etapas 1-5 + R0-R7). E o entrypoint canonico; as outras duas orbitam em torno dela.
2. `design-patterns-mandatory` — consulta obrigatoria como gate de desenho. Em toda mudanca responde primeiro "aplicar padrao vs. `nao aplicar padrao`": quando codigo direto, funcao simples ou refactor localizado resolvem, a resposta obrigatoria e `nao aplicar padrao` (gate anti-overengineering de custo zero). O seletor deterministico (`scripts/select_pattern.py`) e o bundle so sao materializados quando houver decisao real de introduzir, trocar ou revisar um pattern.
3. `domain-modeling-production` — consulta obrigatoria sempre que a mudanca tocar dominio (linguagem ubiqua, agregados, comandos, eventos, invariantes, estados, fronteiras) ou discovery de novo fluxo. As rodadas de clarificacao e o bundle rodam na fase de planejamento/modelagem; edits mecanicos que nao alteram semantica de dominio consomem apenas os principios DMMF (state-as-type, smart constructor, `Decide*` puro).

Gatilhos de materializacao (quando a skill roda o fluxo completo, nao apenas os principios) — alinhados 1:1 com a "Carga forcada por arquivo de modelagem" de `go-implementation`:
- `design-patterns-mandatory`: introducao ou substituicao de abstracao, nova composicao estrutural, coordenacao entre objetos, variacao de algoritmo/estado, ou revisao explicita de pattern.
- `domain-modeling-production`: diff em `**/domain/entities/**`, `**/domain/valueobjects/**`, `**/domain/commands/**`, `**/application/events/**`, novo bounded context, ou novo workflow de negocio.

Fora dos gatilhos, aplicar apenas os principios do trio (sem carga adicional de referencias) e respeitar a economia abaixo. O trio reforca — nunca afrouxa — os gates existentes (R-ADAPTER-001, R-AGENT-WF-001, R-WF-KERNEL-001 e DMMF state-as-type).

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
- R5.26 `[HARD]`: prefixo `_` em identificador e TOTALMENTE PROIBIDO e nao e Go idiomatico. Nenhuma constante, variavel, campo, funcao, metodo ou global (exportado ou nao) pode comecar com `_` (ex.: `_maxWriteAttempts` proibido; usar `maxWriteAttempts`). Todos em camelCase/PascalCase idiomatico. Unica excecao: o identificador em branco `_` (blank identifier) para descartar valores. Revoga a tolerancia da Uber R5.26 ao prefixo `_` em globais (decisao do projeto 2026-06-04). Gate: `grep -rnE "\b_[a-zA-Z][a-zA-Z0-9]*" --include="*.go"` nao deve achar identificador prefixado com `_` fora de blank identifier.
- R6: `context.Context` em toda fronteira de IO; DI via construtores explicitos.
- R6.4: `var _ Interface = (*Type)(nil)` proibido.
- R6.7: `clock.Clock` proibido em use cases e repositorios; usar `time.Now().UTC()` inline quando permitido ou passar instante por command object.
- R7.1: usar `any` em vez de `interface{}`.
- R7.2: usar `log/slog` para logging estruturado.
- R7.6: usar `errors.Join` para agregar erros.
- Goroutines sempre cancelaveis via `context.Context`, sem leak e integradas ao shutdown coordenado.

## DMMF — Domain Modeling Made Functional

Principios obrigatorios inspirados em *Domain Modeling Made Functional* (Scott Wlaschin), adaptados para Go idiomatico. Anti-padroes proibidos `[HARD]`: `Result[T,E]` customizado, currying, DSL de pipeline, monades ou Either.

1. **State-as-type**: status e outcomes sao tipos fechados (`type RunStatus string` + constantes enumeradas); nunca `string` livre em assinatura publica.
2. **Smart constructors**: VOs e commands expoe apenas construtores que retornam `(T, error)`; campos privados; zero value invalido por construcao.
3. **Decide* puro**: toda regra de negocio vive em funcoes `Decide*` — sem IO, sem `context.Context`, deterministico; recebe `ids []uuid.UUID` e `now time.Time` como parametros quando necessario.
4. **Workflow pipeline**: fluxo linear `parse → validate → decide → persist → publish`; cada passo recebe o resultado tipado do anterior; sem branching de dominio fora do `Decide*`.
5. **Discriminated union via errors.As**: divergencia de fluxo via `errors.As(err, &typed)` ou `errors.Is`; nunca `switch` em campo `string`.
6. **Pending step tipado** (inspiracao Mastra): estado de espera de input do usuario e modelado como tipo fechado (ex.: `AwaitingKind`), nunca flag booleano solta; persistido no `Snapshot` do kernel e retomado por merge-patch antes do parse. Thread = `(resourceId, threadId)` opaco; sinal = mensagem recebida. Esses primitivos (Thread/Run/WorkingMemory/PendingStep) vivem em `internal/platform/{agent,memory}` e sao consumidos pelos modulos, nunca reimplementados em dominio.

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

Em todos os bounded contexts (`internal/identity`, `internal/billing`, `internal/card`, `internal/agents`, `internal/categories`, `internal/budgets`, `internal/transactions` e futuros), os diretorios abaixo sao tratados como portas de entrada ou adapters outbound e devem permanecer finos:

- `infrastructure/http/server/handlers`
- `infrastructure/messaging/database/consumers`
- `infrastructure/jobs/handlers`
- `infrastructure/messaging/database/producers`

Regras obrigatorias:
1. Handlers HTTP, consumers de eventos e jobs sao apenas porta de entrada: decodificam input, chamam use case, traduzem erro/saida e encerram.
2. O fluxo permitido e sempre `handler -> usecase -> repository/service/client`.
3. Proibido implementar regra de negocio, branching de negocio, query SQL, decisao de persistencia, calculo de janela de tempo/paginacao/default, orchestration cross-repository ou chamada direta a repository/client dentro desses handlers/consumers/jobs.
4. `producers` sao adapters outbound: podem serializar e publicar um evento ja decidido pela aplicacao, mas nao podem decidir regra de negocio, trigger, payload semantico ou branching de dominio.
5. Em novos desenvolvimentos, nao injetar `RepositoryFactory`, `manager.Manager`, `database.DBTX`, clients externos ou services de dominio diretamente em handlers/consumers/jobs quando o use case correspondente puder receber essa responsabilidade.
6. Em use cases e services, e proibido criar interfaces locais apenas para expor `DBTX(ctx)`; quando a dependencia for somente o handle de banco, injetar `database.DBTX` concreto na struct consumidora.

Verificacao:
- Gates existentes cobrem `internal/identity`, `internal/billing`, `internal/card`, `internal/agents` e `internal/platform`.
- Novos bounded contexts devem adicionar seus proprios gates antes de expor handlers/consumers/jobs/producers.

## Padrao de Agent — substrato `internal/platform/agent` + consumidor `internal/agents`

A capacidade agentiva e um port de comportamento do Mastra em duas camadas: o substrato reutilizavel `internal/platform/{agent,llm,memory,workflow,tool,scorer}` e o consumidor de referencia `internal/agents` (port weather, molde para novos agentes). Codificado em `.claude/rules/agent-workflows-tools.md` (`R-AGENT-WF-001`, hard, re-escopado pela emenda 2026-06-29 — `internal/agent` foi descontinuado). Skill operacional: `mastra`.

Regras obrigatorias:
1. Fluxo canonico de inbound: `InboundRequest -> AgentRuntime.Execute -> ThreadGateway.GetOrCreate -> RunStore.Insert -> AgentRegistry.Resolve -> Agent.Execute (loop tool-calling) -> MessageStore.Append -> closeRun`. Execucao duravel multi-step pelo kernel `workflow.Engine[S].Start/Resume`.
2. Comportamento novo entra como novo agente/tool/workflow/scorer no consumidor, montando primitivos do substrato. Proibido roteamento por `switch case intent.Kind`; resolucao por `AgentRegistry`/`WorkflowRegistry`.
3. `Tool` e adapter fino (`tool.NewTool[I,O]`, herda R-ADAPTER-001): zero regra de negocio, SQL direto ou branching de dominio; o `exec` delega a client/usecase.
4. Estados de fronteira sao tipos fechados (DMMF state-as-type): `agent.RunStatus`/`agent.ToolOutcome`, `workflow.RunStatus`/`StepStatus`/`SuspendReason`, `scorer.ScorerKind`, `memory.MessageRole`. Toda execucao e um `Run` auditavel (`run_id`, `thread`, `agent_id`, `status`, `duration_ms`, `error`).
5. LLM aparece apenas nas call-sites sancionadas (loop tool-calling do agent, step que chama `Stream`, scorer LLM-judged); nunca no kernel. OpenRouter e o unico provider; sem fallback chain.
6. Toda alteracao Go exige `go-implementation` (Etapas 1-5 + checklist R0-R7) e DMMF conforme a precedencia em `.claude/rules/governance.md`.
7. **Primitivos de plataforma `[HARD]`**: Thread = `(resourceId, threadId)` opaco resolvido a cada execucao via `ThreadGateway.GetOrCreate`; `Run` aberto e fechado com `RunStatus` fechado; pending step = estado de espera fechado salvo no `Snapshot` do kernel; working memory injetada no system prompt quando disponivel; resume por merge-patch antes do parse. Thread/Run/WorkingMemory/PendingStep vivem em `internal/platform/{agent,memory}` e sao consumidos pelos modulos, nunca reimplementados em dominio. Referencia: R-AGENT-WF-001.6–001.8 e addendum `.6-A`/`.8-A`. Os consumidores consomem o kernel generico (`Engine[S]` de `internal/platform/workflow`).

## Kernel Generico de Workflow (`internal/platform/workflow`)

O kernel de workflow em `internal/platform/workflow` e um mecanismo generico de orquestacao de passos (`Step[S]`, `Engine[S]`, combinadores, suspend/resume, retry), codificado em `.claude/rules/workflow-kernel.md` (`R-WF-KERNEL-001`, hard). Gate bloqueante (ADR-004): regra redigida antes de qualquer codigo do kernel.

Regras obrigatorias:
1. Proibido import de pacote de dominio (`internal/transactions`, `internal/billing`, `internal/identity`) ou de camada superior que consome o kernel (`internal/platform/agent`, `internal/platform/memory`), bem como qualquer tipo semantico (`intent`, `pendingexpense`, `category`).
2. O kernel opera sobre estado generico `S any` e `correlationKey string` opaca — sem `user_id`, `resourceId` ou tipo semantico de dominio em assinaturas publicas.
3. Estados (`RunStatus`/`StepStatus`/`SuspendReason`) sao tipos fechados — nunca string livre.
4. SQL apenas no adapter Postgres (`infrastructure/postgres/`); zero regra de negocio, branching de dominio ou LLM no kernel.
5. Metricas com cardinalidade controlada: labels permitidos sao `workflow`, `step`, `status`, `outcome`; proibido `user_id`, `correlation_key`, `category_id`.
6. Zero comentarios em Go de producao (herda R-ADAPTER-001.1).
7. Os consumidores instanciam `Engine[S]` com sua propria struct de estado (ex.: `internal/agents` usa `Engine[WeatherState]` no workflow weather); Thread/WorkingMemory/PendingStep sao primitivos de `internal/platform/{agent,memory}`, nunca do kernel.

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
6. Rodar validacoes proporcionais ao risco. Matriz obrigatoria:
   - `domain/` ou mudanca de API publica/contrato: `go build ./...`, `go vet ./...`, `go test -race -count=1 ./...`, `golangci-lint run ./...`, gates de governanca.
   - `application/` ou `infrastructure/` (sem API publica): `go build ./...`, `go vet ./...`, `go test -race -count=1 ./<modulo-alterado>/...`, `golangci-lint run ./<modulo-alterado>/...`.
   - Adapter (handlers/consumers/jobs/producers): build/vet do pacote + lint + gates R-ADAPTER-001.
   - Scripts, docs, configs (sem codigo Go): `gofmt -l`, `task lint:fmt:check` e validacao sintatica quando aplicavel.
7. Registrar bloqueios, drift e suposicoes explicitamente.
8. Zero comentarios em codigo Go de producao [HARD] — inegociavel. Codigo Go de producao = arquivos `.go` em `cmd/`, `internal/`, `configs/` e pacotes equivalentes, excluindo `*_test.go`, `*.pb.go`, diretorios `mocks/`, `vendor/`, `.tools/` e `tools.go`. Excecoes permitidas: `// Code generated` (gerado por ferramenta), diretivas `//go:` e `//nolint:` com justificativa na mesma linha. Comentarios inline (`x := 1 // explicacao`) tambem sao proibidos; extrair para nome de variavel/funcao.

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

## Referencias Oficiais

As regras de prompt engineering, tool use, agent design e structured outputs deste documento refletem as documentacoes oficiais abaixo, consultadas em 2026:

- [Claude Platform Docs — Intro](https://platform.claude.com/docs/en/overview)
- [Claude — Prompting best practices](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-4-best-practices)
- [Claude — Tool use overview](https://platform.claude.com/docs/en/agents-and-tools/tool-use/overview)
- [Claude — Structured outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs)
- [Claude — MCP connector](https://platform.claude.com/docs/en/agents-and-tools/mcp-connector)
- [Claude Code — Best Practices](https://code.claude.com/docs/en/best-practices)
- [Claude Code — Common Workflows](https://code.claude.com/docs/en/common-workflows)
- [OpenAI — Function calling](https://developers.openai.com/api/docs/guides/function-calling)
- [OpenAI — Structured outputs](https://developers.openai.com/api/docs/guides/structured-outputs)
- [OpenAI — Agents SDK](https://developers.openai.com/api/docs/guides/agents)
- [OpenAI — Cost optimization](https://developers.openai.com/api/docs/guides/cost-optimization)
- [OpenAI — Deployment checklist](https://developers.openai.com/api/docs/guides/deployment-checklist)

## Restricoes Finais

1. Nao assumir versao de linguagem, framework ou runtime sem verificar localmente.
2. Nao alterar comportamento publico sem explicitar.
3. Nao usar exemplos como copia cega.
4. Nao reverter mudancas do usuario.
5. Se houver conflito entre regra generica e convencao real segura do projeto, seguir a opcao mais segura e registrar a decisao.
6. Skills que invocam outras skills devem respeitar `scripts/lib/check-invocation-depth.sh` ou equivalente em `.agents/lib/`, com limite padrao 2.
