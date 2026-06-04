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

# Prompt enriquecido v2

```text
Quero que voce implemente, sem discovery e sem resposta especulativa, o uso obrigatorio, mandatorio e inegociavel de `github.com/JailtonJunior94/devkit-go/pkg/observability/otel` neste repositorio Go. O resultado precisa ser robusto, eficiente, escalavel, production-ready, production-proof e sem falso positivo de conclusao.

Seu trabalho nao e "sugerir" como poderia ficar. Seu trabalho e entregar a implementacao final, coerente com o codigo real do repositorio, cobrindo bootstrap, injecao de dependencias, spans, logs estruturados e propagacao de contexto na cadeia `handlers -> usecases -> repositories/clients/servers`.

O ponto de partida obrigatorio da implementacao e sempre `cmd/server/server.go` e/ou `cmd/worker/worker.go`.

Tambem e obrigatorio, mandatorio e inegociavel:
1. usar a skill `go-implementation` e seus exemplos como base normativa de implementacao;
2. usar os exemplos deste prompt como referencia normativa de desenho, mas sempre adaptados ao estado real do repositorio e da biblioteca;
3. entregar codigo final com `0 comentarios`, sem comentarios de linha, bloco ou doc comments adicionados pela implementacao;
4. nao declarar sucesso parcial como se fosse implementacao completa.

Antes de qualquer alteracao, carregue obrigatoriamente:
1. `AGENTS.md`
2. `.github/skills/agent-governance/SKILL.md`
3. `.github/skills/go-implementation/SKILL.md`
4. As referencias realmente necessarias da skill Go para arquitetura, interfaces, observabilidade, configuracao, persistencia e testes
5. `go.mod`
6. `configs/config.go`
7. `cmd/server/server.go`
8. `cmd/worker/worker.go`
9. `internal/platform/httpclient/...`
10. O pacote real de observabilidade atualmente usado pelo repositorio
11. Handlers, use cases, repositories, clients e servers relevantes do fluxo afetado

Contexto real minimo ja verificado e que deve orientar a implementacao:
1. `go.mod` declara Go `1.26.2` e `github.com/JailtonJunior94/devkit-go v0.4.0`.
2. `cmd/server/server.go` e `cmd/worker/worker.go` carregam config com `configs.LoadConfig(".")`, criam logger e `events.NewBus()`, inicializam observabilidade antes do database manager e fazem shutdown explicito desse componente.
3. Os entrypoints atuais ainda chamam `observability.NewProvider(cfg)` e passam `provider.Observability()` para `database.NewManager`, `billing.NewModule` e `chiserver.New`.
4. `internal/platform/httpclient.NewClient` ja exige `devkitobs.Observability` e encapsula `devkit-go/pkg/httpclient.NewObservableClient`.
5. Assuma o estado atual do codebase como fonte da verdade, inclusive quando houver inconsistencias aparentes entre imports, arquivos presentes, worktree e documentacao historica.
6. O repositorio e um monolito modular em Go; as fronteiras arquiteturais precisam ser preservadas.
7. O fluxo funcional deve continuar obedecendo `handler -> usecase -> repository e/ou client http`, sem acoplamento invertido.
8. Existem regras rigidas de PII. Nunca logar email, WhatsApp, CPF, card data, segredos, DSN com senha ou payload sensivel bruto. Em `billing`, trate como sensiveis no minimo `customer.email`, `customer.mobile`, `customer.cpf`, `card.*` e `payment.*.card.*`.

Ambiguidades obrigatorias a resolver contra o codigo real antes de editar:
1. O snippet de referencia usa `cfg.HTTPConfig.ServiceName`, mas o codigo atual declara `cfg.HTTPConfig.ServiceNameAPI`.
2. O snippet de referencia usa `ExporterEndpoint`, `ExporterInsecure` e `ExporterProtocol`, mas o `configs/config.go` atual declara `OTLPEndpoint`, `OTLPHeaders`, `TraceSampleRate`, `LogLevel`, `LogFormat` e `ServiceVersion`.
3. O estado atual do codebase prevalece sobre exemplos antigos, documentacao anterior, suposicoes sobre o branch e qualquer contexto historico divergente.
4. Os entrypoints atuais usam `billing.WithProvider(provider)`; se a implementacao padronizar o nome local para `o11y`, isso deve ser feito de forma consciente e consistente, sem inventar APIs inexistentes.
5. Portanto, NAO invente campos, nomes, tipos ou APIs. Use a API real da dependencia e o wiring real do repositorio como fonte da verdade.

Objetivo principal inegociavel:
1. Tornar obrigatorio o uso real de `devkit-go/pkg/observability/otel` no bootstrap principal, no minimo em `cmd/server/server.go` e `cmd/worker/worker.go`, direta ou indiretamente por meio de wrapper local coerente.
2. Garantir que a observabilidade seja propagada explicitamente por construtor para componentes que recebem request, coordenam caso de uso, executam IO, falam com banco, outbox, jobs ou integracoes HTTP.
3. Padronizar spans e logs estruturados com nomes consistentes, contexto correto, cardinalidade controlada e sem vazamento de PII.
4. Eliminar lacunas onde o `o11y` e inicializado, mas o fluxo real continua sem instrumentacao relevante.

Definicao objetiva do que SERA implementado:
1. Bootstrap:
   - validar e mapear a configuracao real para a API real de `otel`;
   - inicializar o `o11y` antes de database, HTTP clients e modulos que dependem dele;
   - manter shutdown deterministico, ordenado e sem vazamento de recursos.
2. Wiring:
   - partir obrigatoriamente dos entrypoints reais e do wiring efetivamente presente no estado atual do codebase;
   - propagar `o11y` ou `o11y.Observability()` explicitamente por construtor;
   - nao introduzir singleton global, service locator ou acesso oculto.
3. Instrumentacao:
   - handlers devem abrir span de entrada e emitir log estruturado de recebimento da operacao com campos permitidos;
   - use cases devem abrir span filho, propagar `context.Context` e registrar somente eventos relevantes;
   - repositories e clients devem instrumentar chamadas de IO sem expor SQL bruto, DSN sensivel, headers sigilosos ou payloads sensiveis;
   - jobs, runners e workers devem manter tracing/logging coerente com o mesmo `o11y` obrigatorio.
4. Testes e comprovacao:
   - ajustar ou criar testes proporcionais ao impacto para validar wiring critico, propagacao de dependencia e comportamento essencial observavel;
   - nao encerrar a tarefa enquanto houver caminho critico sem observabilidade obrigatoria comprovavel.

Regras de robustez, eficiencia, escalabilidade e producao:
1. Nao criar explosao de spans: nao abrir span em helper trivial, getter, mapper simples ou loop de alto volume sem justificativa concreta.
2. Nao criar explosao de logs: evitar log por item em lote, payload inteiro, SQL completo, headers completos ou qualquer dado de cardinalidade descontrolada.
3. Logs e atributos devem usar somente campos operacionais uteis e com cardinalidade controlada.
4. Nao usar `context.Background()` no meio do fluxo de request/job onde o contexto recebido deveria ser propagado.
5. Nao duplicar erro em tres camadas com o mesmo log; registrar no boundary correto com contexto suficiente.
6. Nao considerar a tarefa pronta se existir apenas bootstrap instrumentado sem spans/logs coerentes nas bordas e IO relevante.
7. Nao considerar a tarefa pronta se o `o11y` estiver presente, mas nao for realmente consumido pelos componentes criticos.
8. Nao considerar a tarefa pronta se a implementacao depender de wrappers vazios, no-op helpers ou spans sem semantica.
9. Nao considerar a tarefa pronta se houver risco de falso positivo operacional por logar "success" antes de concluir a operacao real.

Padrao minimo esperado de nomes e semantica:
1. spans em formato consistente, por exemplo:
   - `billing_webhook_handler.handle`
   - `billing_ingest_webhook_usecase.execute`
   - `subscription_repository.find_active_by_user_id_for_update`
   - `kiwify_client.get_subscription`
   - `worker.reconcile_subscriptions`
2. logs com campos como `layer`, `operation`, `module`, `provider`, `event_id`, `request_id`, `correlation_id`, `user_id` somente quando permitido e seguro;
3. proibido anexar payload sensivel bruto, card data, cpf, email, segredo ou token em atributo de span ou campo de log;
4. se houver regra local de redaction/masking, ela prevalece sobre conveniencia de debug.

Proibicoes explicitas:
1. Nao quebrar o wiring atual de database manager, modules ou `internal/platform/httpclient`.
2. Nao substituir DI manual por framework.
3. Nao usar `&http.Client{}` direto fora de testes.
4. Nao burlar `internal/platform/httpclient` para chamadas outbound.
5. Nao alterar regra de negocio, maquina de estados, locking, idempotencia ou politicas de PII sob o pretexto de instrumentacao.
6. Nao inventar novos campos de config se os atuais bastarem.
7. Nao deixar comentarios no codigo final.

Criterios de aceitacao anti-falso-positivo:
1. `cmd/server/server.go` e `cmd/worker/worker.go` passam a depender obrigatoriamente de `devkit-go/pkg/observability/otel`, sem ambiguidade sobre o backend efetivo usado.
2. O `o11y` resultante e injetado explicitamente nas dependencias relevantes; nao existe acesso global implicito.
3. Os fluxos criticos realmente passam a gerar spans/logs coerentes em handlers, use cases e IO relevante, e nao apenas no bootstrap.
4. `context.Context` e propagado corretamente do inicio ao fim dos fluxos instrumentados.
5. A instrumentacao respeita PII, mascaramento e segredos, sem falso ganho de observabilidade a custo de vazamento.
6. A implementacao evita span/log noise e nao introduz custo operacional desnecessario em hot paths.
7. O shutdown do provider permanece correto e sem recurso aberto.
8. O codigo final possui `0 comentarios`.
9. A resposta final descreve exatamente quais arquivos foram alterados, qual wiring foi adotado e por que isso elimina falso positivo de "observabilidade obrigatoria".

Saida esperada:
1. Analise curta das ambiguidades reais antes de alterar.
2. Implementacao completa.
3. Testes e ajustes proporcionais ao risco.
4. Resumo final objetivo, em PT-BR, explicando o que ficou obrigatorio no bootstrap e no fluxo ponta a ponta.

Se houver conflito entre este prompt, o snippet fornecido, `AGENTS.md`, `agent-governance`, `go-implementation` e o estado real do repositorio, prevalecem `AGENTS.md`, `go-implementation`, o codigo real e a opcao mais segura.
```

# Exemplo concreto do que sera implementado

O exemplo abaixo nao e para copia cega. Ele existe para deixar explicito o desenho esperado do resultado final a partir, obrigatoriamente, de `cmd/server/server.go` e/ou `cmd/worker/worker.go`.

```go
func Run(ctx context.Context) error {
	cfg, err := configs.LoadConfig(".")
	if err != nil {
		return err
	}

	logger := slog.Default()
	eventBus := events.NewBus()

	o11y, err := observability.NewProvider(cfg)
	if err != nil {
		return err
	}

	mgr, err := database.NewManager(ctx, cfg, o11y.Observability())
	if err != nil {
		return errors.Join(err, o11y.Shutdown(context.Background()))
	}

	identityModule, err := identity.NewModule(identity.WithDatabase(mgr))
	if err != nil {
		return errors.Join(err, o11y.Shutdown(context.Background()), mgr.Shutdown(context.Background()))
	}

	billingModule, err := billing.NewModule(
		billing.WithConfig(cfg),
		billing.WithEventBus(eventBus),
		billing.WithLogger(logger),
		billing.WithDatabase(mgr),
		billing.WithProvider(o11y),
		billing.WithUserRepository(identityModule.Ports.UserRepository),
	)
	if err != nil {
		return errors.Join(err, o11y.Shutdown(context.Background()), mgr.Shutdown(context.Background()))
	}

	runnerManager := platformworker.NewManager(
		logger,
		slices.Concat(identityModule.Runners(), billingModule.Runners())...,
	)
	if err := runnerManager.Start(ctx); err != nil {
		return errors.Join(err, o11y.Shutdown(context.Background()), mgr.Shutdown(context.Background()))
	}

	server, err := chiserver.New(
		o11y.Observability(),
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
			o11y.Shutdown(context.Background()),
			mgr.Shutdown(context.Background()),
		)
	}

	server.RegisterRouters(slices.Concat(identityModule.Routers(), billingModule.Routers())...)

	if err := server.Start(ctx); err != nil {
		return errors.Join(
			err,
			runnerManager.Stop(context.Background()),
			o11y.Shutdown(context.Background()),
			mgr.Shutdown(context.Background()),
		)
	}

	<-ctx.Done()

	return errors.Join(
		server.Stop(context.Background()),
		runnerManager.Stop(context.Background()),
		o11y.Shutdown(context.Background()),
		mgr.Shutdown(context.Background()),
	)
}
```

```go
func Run(ctx context.Context) error {
	cfg, err := configs.LoadConfig(".")
	if err != nil {
		return err
	}

	logger := slog.Default()
	eventBus := events.NewBus()

	o11y, err := observability.NewProvider(cfg)
	if err != nil {
		return err
	}

	mgr, err := database.NewManager(ctx, cfg, o11y.Observability())
	if err != nil {
		return errors.Join(err, o11y.Shutdown(context.Background()))
	}

	identityModule, err := identity.NewModule(identity.WithDatabase(mgr))
	if err != nil {
		return errors.Join(err, o11y.Shutdown(context.Background()), mgr.Shutdown(context.Background()))
	}

	billingModule, err := billing.NewModule(
		billing.WithConfig(cfg),
		billing.WithEventBus(eventBus),
		billing.WithLogger(logger),
		billing.WithDatabase(mgr),
		billing.WithProvider(o11y),
		billing.WithUserRepository(identityModule.Ports.UserRepository),
	)
	if err != nil {
		return errors.Join(err, o11y.Shutdown(context.Background()), mgr.Shutdown(context.Background()))
	}

	runnerManager := platformworker.NewManager(
		logger,
		slices.Concat(identityModule.Runners(), billingModule.Runners())...,
	)
	if err := runnerManager.Start(ctx); err != nil {
		return errors.Join(err, o11y.Shutdown(context.Background()), mgr.Shutdown(context.Background()))
	}

	<-ctx.Done()

	return errors.Join(
		runnerManager.Stop(context.Background()),
		o11y.Shutdown(context.Background()),
		mgr.Shutdown(context.Background()),
	)
}
```

Exemplo esperado de instrumentacao nas camadas a partir desses entrypoints:

```go
type KiwifyWebhookHandler struct {
	useCase ingestWebhookExecutor
	o11y    observability.Observability
	logger  *slog.Logger
	header  string
}

func (h *KiwifyWebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.o11y.Tracer().Start(r.Context(), "billing_webhook_handler.handle")
	defer span.End()

	correlationID := r.Header.Get("X-Request-ID")

	h.o11y.Logger().Info(
		ctx,
		"webhook_received",
		observability.String("layer", "handler"),
		observability.String("operation", "handle_kiwify_webhook"),
		observability.String("module", "billing"),
		observability.String("provider", "kiwify"),
		observability.String("correlation_id", correlationID),
	)

	body, err := io.ReadAll(io.LimitReader(r.Body, webhookBodyLimitBytes))
	if err != nil {
		span.RecordError(err)
		writeWebhookJSON(w, http.StatusInternalServerError)
		return
	}

	_, err = h.useCase.Execute(ctx, input.IngestWebhookInput{
		RawBody:             body,
		Headers:             extractHeaders(r),
		SignatureHeaderName: h.header,
		ReceivedAt:          time.Now().UTC(),
	})
	if err != nil {
		span.RecordError(err)
		h.o11y.Logger().Error(
			ctx,
			"webhook_failed",
			observability.String("layer", "handler"),
			observability.String("module", "billing"),
			observability.String("operation", "handle_kiwify_webhook"),
			observability.String("correlation_id", correlationID),
		)
		writeWebhookJSON(w, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

```go
func (u *IngestKiwifyWebhookUseCase) Execute(ctx context.Context, in input.IngestWebhookInput) (output.IngestWebhookResult, error) {
	return observability.Observe(ctx, u.o11y, u.metrics, "billing", "ingest_kiwify_webhook", func(ctx context.Context) (output.IngestWebhookResult, error) {
		return u.txRunner.Do(ctx, func(txCtx context.Context, tx database.DBTX) (output.IngestWebhookResult, error) {
			if err := u.provider.VerifySignature(in.RawBody, in.Headers); err != nil {
				return output.IngestWebhookResult{}, fmt.Errorf("ingest kiwify: %w", err)
			}

			inserted, err := u.webhookRepo.InsertIfNew(txCtx, webhookEvent)
			if err != nil {
				return output.IngestWebhookResult{}, fmt.Errorf("ingest kiwify: inserir webhook_event: %w", err)
			}
			if !inserted {
				return output.IngestWebhookResult{Duplicate: true}, nil
			}

			if err := u.publisher.Publish(txCtx, tx, evt); err != nil {
				return output.IngestWebhookResult{}, fmt.Errorf("ingest kiwify: publicar outbox: %w", err)
			}

			return output.IngestWebhookResult{Duplicate: false, WebhookEventID: webhookEvent.ID()}, nil
		})
	})
}
```

```go
func (r *PgxWebhookEventRepository) InsertIfNew(ctx context.Context, event entities.WebhookEvent) (bool, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "billing_webhook_event_repository.insert_if_new")
	defer span.End()

	result, err := r.dbtx(ctx).ExecContext(ctx, insertIfNewWebhookEvent,
		event.ID().String(),
		event.Provider(),
		event.ExternalEventID().String(),
		event.EventType(),
		event.Signature(),
		[]byte(event.HeadersJSON()),
		[]byte(event.Payload()),
		event.ReceivedAt(),
	)
	if err != nil {
		span.RecordError(err)
		return false, fmt.Errorf("postgres webhook event repository: insert if new: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		span.RecordError(err)
		return false, fmt.Errorf("postgres webhook event repository: rows affected: %w", err)
	}

	return affected > 0, nil
}
```

```go
func (w *wiring) buildKiwifyAdapter(ctx context.Context, subscriptionRepo *billingrepos.PgxSubscriptionRepository) (*kiwifyclient.KiwifyAdapter, error) {
	client, err := platformhttpclient.NewClient(
		w.options.o11y.Observability(),
		platformhttpclient.WithBaseURL(w.options.config.KiwifyConfig.APIBaseURL),
		platformhttpclient.WithTimeout(w.options.config.KiwifyConfig.HTTPTimeout),
		platformhttpclient.WithDefaultRetry(
			w.options.config.KiwifyConfig.HTTPRetryMaxAttempts,
			w.options.config.KiwifyConfig.HTTPRetryBackoff,
		),
		platformhttpclient.WithTarget("kiwify"),
	)
	if err != nil {
		return nil, err
	}

	kiwifyHTTPClient := kiwifyclient.NewClient(
		client,
		w.options.config.KiwifyConfig.RateLimitMaxRequestsPerMin,
		w.options.config.KiwifyConfig.RateLimitBurst,
	)

	oauthClient := kiwifyclient.NewOAuthClient(
		client,
		w.options.config.KiwifyConfig.ClientID,
		w.options.config.KiwifyConfig.ClientSecret,
		w.options.config.KiwifyConfig.OAuthTokenSafetyMargin,
	)

	return kiwifyclient.NewKiwifyAdapter(kiwifyHTTPClient, oauthClient, subscriptionRepo), nil
}
```

Leitura do exemplo:

1. O ponto de partida obrigatorio e `cmd/server/server.go` e/ou `cmd/worker/worker.go`.
2. O `o11y` nasce nesses entrypoints e entra cedo no wiring.
3. O bootstrap injeta a dependencia nos modulos reais antes de banco, server HTTP, runners e clients relevantes.
4. O handler abre o span de entrada com `r.Context()` e nao com `context.Background()`.
5. O use case preserva o contexto e centraliza a orquestracao observavel.
6. O repository instrumenta IO relevante sem vazar SQL bruto, DSN sensivel ou payload sigiloso.
7. O client HTTP continua obrigatoriamente passando por `internal/platform/httpclient`.
8. Nao ha log de payload sensivel, segredo, email, CPF, card data ou DSN bruto.
9. O desenho final e observavel de ponta a ponta, sem trocar arquitetura por magia global.

# Melhorias aplicadas

- Evoluiu o prompt de "instrumentar OTel" para uma especificacao de execucao, com definicao explicita do que sera implementado.
- Amarrou o prompt ao estado real do repositorio, inclusive ao bootstrap existente, ao wrapper local de observabilidade e ao `internal/platform/httpclient`.
- Adicionou gates anti-falso-positivo para impedir conclusao enganosa baseada apenas em bootstrap ou spans vazios.
- Adicionou regras objetivas de eficiencia e escalabilidade para evitar log/spam, cardinalidade descontrolada e custo operacional inutil.
- Tornou explicito que robustez e production-ready significam wiring real, contexto propagado, shutdown correto, PII protegida e instrumentacao relevante nos pontos criticos.
- Incluiu um exemplo concreto do resultado esperado em bootstrap, handler, use case e client, deixando claro o desenho que deve emergir da implementacao.

Exemplo de codigo real para analisar a proposta
