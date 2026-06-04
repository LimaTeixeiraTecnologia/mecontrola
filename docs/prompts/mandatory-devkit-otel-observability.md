# Prompt original

Quero implementar de forma mandatória e inegociável o uso de `github.com/JailtonJunior94/devkit-go/pkg/observability/otel` neste projeto.

Use como referência de startup `cmd/worker/worker.go` e `cmd/server/server.go`, no espírito deste exemplo:

```go
cfg, err := configs.LoadConfig(".")
if err != nil {
    return fmt.Errorf("run: failed to load config: %v", err)
}

ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

o11yConfig := &otel.Config{
    Environment:     cfg.Environment,
    ServiceName:     cfg.HTTPConfig.ServiceName,
    ServiceVersion:  cfg.O11yConfig.ServiceVersion,
    TraceSampleRate: cfg.O11yConfig.TraceSampleRate,
    OTLPEndpoint:    cfg.O11yConfig.ExporterEndpoint,
    Insecure:        cfg.O11yConfig.ExporterInsecure,
    LogLevel:        observability.LogLevel(cfg.O11yConfig.LogLevel),
    OTLPProtocol:    otel.OTLPProtocol(cfg.O11yConfig.ExporterProtocol),
    LogFormat:       observability.LogFormat(cfg.O11yConfig.LogFormat),
}

o11y, err := otel.NewProvider(context.Background(), o11yConfig)
if err != nil {
    return fmt.Errorf("run: failed to create observability provider: %v", err)
}
```

Tambem quero uso obrigatório em `handlers -> usecases -> repositories/clients/servers`, no espírito deste exemplo:

```go
ctx, span := h.o11y.Tracer().Start(r.Context(), "invoice_handler.list_by_card")
defer span.End()

h.o11y.Logger().Info(
    ctx,
    "request_received",
    observability.String("operation", "list_by_card"),
    observability.String("layer", "handler"),
    observability.String("entity", "invoice"),
    observability.String("correlation_id", correlationID),
    observability.String("user_id", user.ID),
)
```

Uso mandatório, obrigatório e inegociável da skill `go-implementation`, de seus exemplos e também dos exemplos deste próprio prompt como referência obrigatória e normativa de desenho, sempre adaptados ao contexto real do repositório.

Mandatório e inegociável ter `0 comentários` no código produzido.

# Prompt enriquecido

```text
Quero que voce implemente o uso obrigatorio, mandatorio e inegociavel de `github.com/JailtonJunior94/devkit-go/pkg/observability/otel` neste repositorio Go, padronizando bootstrap, injecao de dependencias, spans e logging estruturado em toda a cadeia `handlers -> usecases -> repositories/clients/servers`.

Tambem e obrigatorio, mandatorio e inegociavel:
1. usar a skill `go-implementation` e seus exemplos como base normativa e obrigatoria de implementacao;
2. usar os exemplos deste proprio prompt como referencia obrigatoria de bootstrap, spans e logs, adaptando-os ao estado real do repositorio quando houver divergencia;
3. entregar codigo com `0 comentarios` no resultado final, sem comentarios de linha, bloco, doc comments ou observacoes inline adicionadas pela implementacao.

Antes de qualquer alteracao, carregue obrigatoriamente:
1. `AGENTS.md`
2. `.github/skills/agent-governance/SKILL.md`
3. `.github/skills/go-implementation/SKILL.md`
4. As referencias da skill Go realmente relevantes para esta tarefa:
   - `.github/skills/go-implementation/references/architecture.md`
   - `.github/skills/go-implementation/references/interfaces.md`
   - `.github/skills/go-implementation/references/observability.md`
   - `.github/skills/go-implementation/references/api.md`
   - `.github/skills/go-implementation/references/persistence.md`
   - `.github/skills/go-implementation/references/configuration.md`
   - `.github/skills/go-implementation/references/testing.md`
   - `.github/skills/go-implementation/references/examples-infrastructure.md`
   - `.github/skills/go-implementation/references/examples-domain-flow.md`
   - `.github/skills/go-implementation/references/examples-testing.md`
5. `go.mod` para respeitar a versao declarada do Go e as dependencias reais do projeto.

Contexto real do repositorio que deve orientar a implementacao:
- `go.mod` declara Go `1.26.2` e `github.com/JailtonJunior94/devkit-go v0.4.0`.
- O repositorio e um monolito modular em Go; fronteiras arquiteturais precisam ser preservadas.
- O fluxo de negocio deve continuar obedecendo `handler -> usecase -> repositories e/ou client http`.
- `cmd/server/server.go` e `cmd/worker/worker.go` hoje carregam config via `configs.LoadConfig(".")`, inicializam provider de observabilidade antes do database manager e compoem modulos.
- Hoje os entrypoints importam `internal/platform/observability` e passam `provider.Observability()` para outras dependencias; a implementacao deve reconciliar esse wiring com o uso obrigatorio de `devkit-go/pkg/observability/otel`.
- O projeto possui `internal/platform/httpclient`, e chamadas HTTP outbound precisam continuar seguindo esse wrapper, agora com observabilidade consistente.
- Existem regras especificas de PII em `identity` e `billing`; nunca logar email, WhatsApp, CPF, card data ou outros dados sensiveis em claro. Preserve mascaramento/redaction conforme o modulo.

Atencao obrigatoria a ambiguidades entre o snippet desejado e o codigo atual:
- O snippet de referencia usa `cfg.HTTPConfig.ServiceName`, mas o codigo atual declara `cfg.HTTPConfig.ServiceNameAPI`.
- O snippet de referencia usa `cfg.O11yConfig.ExporterEndpoint`, `ExporterInsecure` e `ExporterProtocol`, mas o `configs/config.go` atual declara `OTLPEndpoint`, `OTLPHeaders`, `TraceSampleRate`, `LogLevel`, `LogFormat` e `ServiceVersion`.
- Portanto, NAO invente campos, nomes ou APIs sem primeiro validar o estado real do repositorio e da dependencia `devkit-go`. Adapte o desenho aos nomes existentes ou ajuste a configuracao somente quando houver necessidade concreta, coerente e consistente com o projeto.
- Se a API exata de `otel.NewProvider` ou do tipo `otel.Config` diferir do snippet fornecido, use a API real da biblioteca e adapte o wiring local sem copiar o exemplo literalmente.

Objetivo principal:
1. Tornar obrigatorio o uso de `github.com/JailtonJunior94/devkit-go/pkg/observability/otel` no bootstrap principal da aplicacao, no minimo em:
   - `cmd/server/server.go`
   - `cmd/worker/worker.go`
2. Garantir que a observabilidade seja propagada explicitamente por construtor para handlers, usecases, repositories, clients e servers relevantes.
3. Padronizar a criacao de spans e logs estruturados em cada camada, respeitando contexto, nomes consistentes e mascaramento de dados.
4. Eliminar lacunas em que fluxos relevantes executem sem tracing/logging estruturado coerente com o provider obrigatorio.

Diretrizes de desenho obrigatorias:
1. Preserve a arquitetura do repositorio e a DI manual por construtores; nao use framework de DI.
2. Nao introduza singleton global de observabilidade nem acesso implicito/oculto ao provider.
3. O bootstrap deve inicializar o provider cedo o suficiente para que database, HTTP clients, handlers e workers recebam a dependencia observavel ja pronta.
4. Toda dependencia que executa IO, recebe request, coordena caso de uso ou fala com integracoes externas deve receber observabilidade explicitamente quando isso for necessario para spans/logs.
5. `handler`, `usecase`, `repository`, `client` e `server` devem manter responsabilidades separadas; observabilidade nao pode virar desculpa para pular camadas.
6. O `context.Context` deve ser propagado em todo boundary de IO e em toda operacao relevante de tracing.
7. Os exemplos da skill `go-implementation` e os exemplos deste prompt devem ser seguidos obrigatoriamente como referencia de desenho, sempre com adaptacao ao contexto real do repositorio quando houver conflito objetivo.
8. O codigo final deve ter `0 comentarios`; nao adicionar comentarios de qualquer tipo.
9. Nao copiar literalmente os snippets do pedido quando conflitam com o repositorio real; adaptar ao contexto.
10. Respeitar as regras do repositorio para logs curtos, erro contextual, ausencia de fallback silencioso e protecao de PII.
11. Se ja existir helper local coerente para observabilidade, reutilize-o em vez de duplicar wrapper sem necessidade.

Padrao minimo esperado de uso nas camadas:
1. Handlers:
   - iniciar span no inicio do request, por exemplo `invoice_handler.list_by_card`;
   - logar evento de entrada com campos estruturados como `operation`, `layer`, `entity`, `correlation_id` e identificadores permitidos;
   - nunca logar payload sensivel em claro.
2. Use cases:
   - envolver a operacao com span/log coerente, sem duplicacao inutil do mesmo evento;
   - manter o foco em orquestracao de regra de negocio e propagacao de contexto.
3. Repositories:
   - garantir telemetria nas operacoes de persistencia sem vazar SQL sensivel, DSN ou PII;
   - manter o contrato de transacao e `DBTX` existente intacto.
4. Clients HTTP:
   - usar `internal/platform/httpclient`;
   - garantir propagacao de contexto, timeouts, tracing e logs coerentes com o provider.
5. Servers/runtimes:
   - bootstrap observabilidade antes das dependencias que consomem provider;
   - garantir shutdown ordenado do provider.

Padrao de nomenclatura e semantica:
- Nomes de spans devem ser consistentes e legiveis, no estilo:
  - `invoice_handler.list_by_card`
  - `invoice_usecase.list_by_card`
  - `invoice_repository.find_by_card_id`
  - `kiwify_client.get_subscription`
  - `http_server.handle_webhook`
- Logs devem ser estruturados e incluir apenas campos uteis para operacao e diagnostico.
- Sempre que houver `correlation_id`, `request_id`, `event_id`, `user_id` ou identificador equivalente permitido, propagá-los de forma consistente.
- Para dados sensiveis, aplicar as regras de mascaramento/redaction do modulo em vez de logar valor bruto.

Arquivos e areas minimas a inspecionar antes de editar:
- `go.mod`
- `configs/config.go`
- `cmd/server/server.go`
- `cmd/worker/worker.go`
- `internal/platform/httpclient/...`
- handlers HTTP e/ou gRPC relevantes
- use cases relevantes em `internal/identity/application/usecases` e `internal/billing/application/usecases`
- repositories e clients concretos que executam IO
- qualquer pacote atual de observabilidade ou provider realmente presente no branch

Requisitos funcionais:
1. O bootstrap principal deve usar `devkit-go/pkg/observability/otel` de forma obrigatoria.
2. O provider de observabilidade deve ser disponibilizado para os componentes relevantes por injecao de dependencia.
3. Handlers devem abrir spans e emitir logs estruturados de request.
4. Use cases devem propagar contexto observavel e registrar spans/logs coerentes.
5. Repositories, clients e servers devem respeitar o provider obrigatorio sem quebrar contratos existentes.
6. O fluxo `handler -> usecase -> repositories e/ou client http` deve continuar intacto.

Requisitos nao funcionais obrigatorios:
1. Sem log de segredos, DSN com senha, payloads sensiveis ou PII em claro.
2. Shutdown deterministico do provider de observabilidade.
3. Propagacao correta de `context.Context` em requests, jobs e IO.
4. Naming consistente de spans/logs entre modulos.
5. Testes proporcionais ao impacto da mudanca, cobrindo pelo menos wiring critico e comportamento observavel essencial.

Proibicoes explicitas:
- Nao usar estado global para observabilidade.
- Nao adicionar helper generico vazio ou abstração sem consumidor real.
- Nao instrumentar ignorando mascaramento de PII.
- Nao logar erro e retornar o mesmo erro em duplicidade.
- Nao inventar campos de config inexistentes sem validar antes.
- Nao quebrar wiring atual de database, runtime, modules ou httpclient.
- Nao usar implementacao parcial: o uso deve ser realmente obrigatorio nos pontos criticos do fluxo.
- Nao ignorar os exemplos da skill `go-implementation` nem os exemplos deste prompt.
- Nao deixar comentarios no codigo final sob nenhuma forma.

Criterios de aceitacao:
1. `cmd/server/server.go` e `cmd/worker/worker.go` usam `github.com/JailtonJunior94/devkit-go/pkg/observability/otel` de forma obrigatoria no bootstrap, adaptado ao codigo real do repositorio.
2. O provider resultante e injetado explicitamente nas dependencias relevantes, sem singleton global.
3. Handlers, usecases, repositories/clients/servers relevantes passam a registrar spans e logs estruturados de forma consistente.
4. O padrao `handler -> usecase -> repositories e/ou client http` permanece preservado.
5. PII, segredos e dados sensiveis continuam protegidos conforme as politicas do repositorio.
6. O shutdown do provider continua correto e nao deixa recurso aberto.
7. O codigo final entregue possui `0 comentarios`.
8. A implementacao segue obrigatoriamente a skill `go-implementation`, seus exemplos e os exemplos deste prompt, com adaptacao ao contexto real quando necessario.
9. A resposta final lista os arquivos alterados, explica o wiring adotado e aponta como a observabilidade ficou obrigatoria de ponta a ponta.

Saida esperada:
1. Analise curta das ambiguidades do pedido versus o codigo real antes de codar.
2. Implementacao completa e coerente com o repositorio.
3. Testes e ajustes necessarios para cobrir o wiring e os pontos criticos.
4. Resumo final objetivo em PT-BR com foco em bootstrap, injecao, spans, logs e preservacao arquitetural.

Se houver conflito entre o snippet fornecido, `AGENTS.md`, `agent-governance`, `go-implementation` e o estado real do repositorio, prevalecem `AGENTS.md`, `go-implementation` e a restricao mais segura.
```

# Melhorias aplicadas

- Tornou explicita a carga obrigatoria de `AGENTS.md`, `agent-governance`, `go-implementation` e das referencias Go mais pertinentes para observabilidade, API, configuracao, persistencia e testes.
- Amarrou o prompt ao estado real do repositorio, citando `go.mod`, `cmd/server/server.go`, `cmd/worker/worker.go`, `configs/config.go` e `internal/platform/httpclient`.
- Explicou as ambiguidades centrais do pedido: o snippet usa nomes de config que hoje nao batem com o repositorio (`ServiceName` vs `ServiceNameAPI`, `ExporterEndpoint`/`ExporterProtocol`/`ExporterInsecure` vs `OTLPEndpoint`/`OTLPHeaders`).
- Transformou o objetivo amplo em escopo implementavel, exigindo observabilidade obrigatoria no bootstrap e na cadeia `handlers -> usecases -> repositories/clients/servers`.
- Tornou explicito que o uso da skill `go-implementation`, de seus exemplos e dos exemplos do proprio prompt e obrigatorio e inegociavel.
- Adicionou a exigencia objetiva de `0 comentarios` no codigo final.
- Adicionou criterios de aceitacao verificaveis para injecao explicita, shutdown correto, protecao de PII, consistencia de spans/logs, ausencia de comentarios e preservacao da arquitetura.
