# Relatorio de Bugfix

- Total de bugs no escopo: 17
- Corrigidos: 15
- Rejeitados como falso positivo: 1
- Testes de regressao adicionados ou corrigidos: 15
- Pendentes: 1 validacao externa obrigatoria
- Estado final: blocked

## Bugs

### BUG-01 - Renovacao publicava estado anterior
- ID: BUG-01
- Severidade: major
- Origem: finding de review do billing pipeline
- Estado: fixed
- Causa raiz: o use case persistia o novo periodo, mas publicava e retornava o aggregate carregado antes da renovacao.
- Arquivos alterados: `internal/billing/application/usecases/process_subscription_renewed.go`, `internal/billing/application/usecases/process_subscription_renewed_test.go`
- Teste de regressao: `ProcessSubscriptionRenewedSuite/TestSucessoExtendePeriodo` verifica que o aggregate publicado e retornado possui o novo fim de periodo e o instante do evento.
- Validacao: pacote `./internal/billing/application/usecases` aprovado com race detector.

### BUG-02 - Reconciliacao avancava checkpoint apos falha
- ID: BUG-02
- Severidade: critical
- Origem: finding de review do billing pipeline
- Estado: fixed
- Causa raiz: falhas ao reconciliar vendas individuais eram apenas registradas em log; o fluxo continuava e gravava o checkpoint, impedindo nova tentativa da janela.
- Arquivos alterados: `internal/billing/application/usecases/reconcile_subscriptions.go`, `internal/billing/application/usecases/reconcile_subscriptions_test.go`
- Teste de regressao: `ReconcileSubscriptionsSuite/TestCheckpointNaoAtualizadoEmFalhaDeVenda` comprova que uma falha real e retornada e o checkpoint nao avanca.
- Validacao: pacote `./internal/billing/application/usecases` aprovado com race detector.

### BUG-03 - Client ID enviado como account ID da Kiwify
- ID: BUG-03
- Severidade: major
- Origem: finding de review do billing pipeline
- Estado: fixed
- Causa raiz: a configuracao nao possuia campo proprio para account ID e o wiring reutilizava `ClientID` no header `x-kiwify-account-id`.
- Arquivos alterados: `configs/config.go`, `configs/config_test.go`, `internal/billing/module.go`
- Teste de regressao: `ConfigSuite/TestKiwifyConfigDefaultsAplicados`, `ConfigSuite/TestSafeKiwifyConfigRedactaSecrets` e `TestClient_AccountIDHeaderInEveryRequest` comprovam carga, exposicao segura e uso do account ID dedicado.
- Validacao: pacotes `./configs`, `./internal/billing` e `./internal/billing/infrastructure/http/client/kiwify` aprovados.

### BUG-04 - Transicoes pendentes eram descartadas
- ID: BUG-04
- Severidade: critical
- Origem: finding de review do billing pipeline
- Estado: fixed
- Causa raiz: eventos posteriores ao `activated` sem usuario resolvido apenas geravam log; o payload pendente nao era atualizado. Um upsert com token vazio tambem sobrescreveria o token ja conhecido.
- Arquivos alterados: `internal/identity/infrastructure/messaging/database/consumers/subscription_event_projector.go`, `internal/identity/infrastructure/messaging/database/consumers/subscription_event_projector_test.go`, `internal/identity/infrastructure/repositories/postgres/entitlement_repository.go`, `internal/identity/infrastructure/repositories/postgres/entitlement_repository_integration_test.go`
- Teste de regressao: `SubscriptionEventProjectorSuite/TestPastDueWithNoUserUpdatesPending` e `EntitlementRepositoryIntegrationSuite/TestPendingUpdatePreservesFunnelToken` comprovam atualizacao do payload sem perda do token.
- Validacao: pacotes do projector e repository aprovados com race detector; teste de integracao Postgres aprovado.

### BUG-05 - Notification handlers nao registrados
- ID: BUG-05
- Severidade: major
- Origem: finding de review do billing pipeline
- Estado: fixed
- Causa raiz: os notification handlers existentes nunca eram criados nem expostos pelo modulo, e o worker registrava somente handlers do modulo identity.
- Arquivos alterados: `internal/billing/module.go`, `internal/billing/module_test.go`, `cmd/worker/worker.go`
- Teste de regressao: `ModuleSuite/TestNotificationHandlersAreExposedForWorkerRegistration` comprova que os tres handlers reais sao expostos; build e testes do worker comprovam o registro no bootstrap.
- Validacao: pacotes `./internal/billing` e `./cmd/worker` aprovados com race detector; `go build ./...` aprovado.

### BUG-06 - Suite de reconciliacao nao compilava
- ID: BUG-06
- Severidade: major
- Origem: finding de review do billing pipeline
- Estado: fixed
- Causa raiz: o teste de integracao atribuía o retorno duplo de `ExecContext` a uma unica variavel e criava fixture com status de subscription invalido.
- Arquivos alterados: `internal/billing/infrastructure/jobs/handlers/reconciliation_integration_test.go`
- Teste de regressao: `ReconciliationIntegrationSuite/TestReconcileRefundedSaleTransitionsToRefunded` volta a compilar e executar a reconciliacao com fixture valida.
- Validacao: `go test -race -count=1 -tags=integration ./internal/billing/infrastructure/jobs/handlers` aprovado.

### BUG-07 - Assinatura HMAC invalida nao era auditada
- ID: BUG-07
- Severidade: critical
- Origem: finding de review do billing pipeline
- Estado: fixed
- Causa raiz: o middleware encerrava a requisicao antes do handler, impedindo a persistencia do evento e do status de assinatura.
- Arquivos alterados: `internal/billing/infrastructure/http/server/middleware/hmac_signature.go`, `internal/billing/infrastructure/http/server/middleware/hmac_signature_test.go`, `internal/billing/infrastructure/http/server/handlers/kiwify_webhook_handler.go`, `internal/billing/infrastructure/http/server/handlers/kiwify_webhook_handler_test.go`
- Teste de regressao: `TestKiwifyWebhookHandler_401_InvalidSignatureIsAudited` comprova persistencia com `SignatureStatusInvalid` antes da resposta 401.
- Validacao: pacotes de middleware e handlers aprovados com race detector e integracao.

### BUG-08 - Teste RF-17 nao exercitava concorrencia
- ID: BUG-08
- Severidade: major
- Origem: finding de review do billing pipeline
- Estado: fixed
- Causa raiz: o teste inseria subscriptions sequencialmente e aceitava qualquer erro, sem provar a protecao concorrente nem a constraint esperada.
- Arquivos alterados: `migrations/migrations_integration_test.go`
- Teste de regressao: `MigrationSuite/TestUpAndDownForBillingPipelineMigrations` dispara inserts concorrentes e exige exatamente um sucesso e uma violacao da constraint `billing_subscriptions_user_active_uniq_idx`.
- Validacao: `go test -race -count=1 -tags=integration ./migrations` aprovado.

### BUG-09 - Erro de configuracao do cliente Kiwify era ignorado
- ID: BUG-09
- Severidade: critical
- Origem: auditoria production-ready
- Estado: fixed
- Causa raiz: `NewBillingModule` descartava o erro retornado por `kiwify.NewClient`, permitindo bootstrap parcial com dependencia invalida.
- Arquivos alterados: `internal/billing/module.go`, `internal/billing/module_test.go`, `cmd/server/server.go`, `cmd/worker/worker.go`
- Teste de regressao: `TestInvalidKiwifyClientConfigFailsBootstrap` comprova fail-fast no bootstrap.
- Validacao: pacotes `./internal/billing`, `./cmd/server` e `./cmd/worker` aprovados com race detector.

### BUG-10 - Eventos fora de ordem podiam regredir o entitlement
- ID: BUG-10
- Severidade: critical
- Origem: auditoria production-ready
- Estado: fixed
- Causa raiz: o projector confiava no estado contido no payload antigo do outbox, em vez de projetar o estado canonico atual da subscription.
- Arquivos alterados: `internal/identity/infrastructure/messaging/database/consumers/subscription_event_projector.go`, `internal/identity/infrastructure/messaging/database/consumers/subscription_event_projector_test.go`, `internal/identity/module.go`
- Teste de regressao: a suite do projector comprova que um evento `past_due` atrasado projeta o estado canonico `REFUNDED`, sem regressao.
- Validacao: pacote do projector aprovado com race detector em tres repeticoes.

### BUG-11 - Suposta ausencia de job para expirar subscriptions por tempo
- ID: BUG-11
- Severidade: major
- Origem: auditoria production-ready
- Estado: false_positive
- Causa raiz: a auditoria interpretou `EXPIRED` como transicao persistida automaticamente, mas ADR-005 e techspec §5.3 determinam explicitamente que `(time)/(grace)/(period)` sao decididos por `IsEntitled` em runtime e nao persistidos por job. O PRD tambem determina que `PAST_DUE` permanece `PAST_DUE` apos a graca.
- Arquivos alterados: nenhuma alteracao funcional mantida; o job inicialmente criado durante a remediacao foi removido antes da conclusao.
- Teste de regressao: `TestPastDueAfterGraceNotEntitled` e a suite `TestIsEntitled` comprovam perda de acesso sem alterar o estado persistido.
- Validacao: suites de identity/domain e use case aprovadas com race detector.

### BUG-12 - Teste do scheduler era instavel e mascarava o gate race
- ID: BUG-12
- Severidade: major
- Origem: auditoria production-ready
- Estado: fixed
- Causa raiz: o teste assumia execucao subsegundo para uma biblioteca de cron que arredonda o schedule e dependia de sleeps nao deterministas.
- Arquivos alterados: `internal/platform/worker/job/scheduler_test.go`
- Teste de regressao: o proprio teste usa sincronizacao por channel e schedule suportado.
- Validacao: `go test -race -count=3 ./internal/platform/worker/job` aprovado.

### BUG-13 - Nao havia evidencia E2E executavel do pipeline
- ID: BUG-13
- Severidade: major
- Origem: auditoria production-ready
- Estado: fixed
- Causa raiz: o teste de webhook nao despachava o outbox pelo dispatcher real ate a projecao do modulo identity.
- Arquivos alterados: `internal/billing/infrastructure/http/server/handlers/kiwify_webhook_integration_test.go`
- Teste de regressao: o teste de integracao processa webhook assinado, despacha outbox e verifica `identity_entitlements_pending`.
- Validacao: suite completa com tag `integration` aprovada; smoke real local aprovado.

### BUG-14 - IDs de produto Kiwify permaneciam placeholders
- ID: BUG-14
- Severidade: critical
- Origem: auditoria production-ready
- Estado: fixed
- Causa raiz: os seeds usavam placeholders e nao havia mecanismo fail-fast para sincronizar os tres IDs reais configurados.
- Arquivos alterados: `configs/config.go`, `configs/config_test.go`, `internal/billing/application/interfaces/plan_repository.go`, `internal/billing/infrastructure/repositories/postgres/plan_repository.go`, `internal/billing/infrastructure/repositories/postgres/plan_repository_integration_test.go`, `internal/billing/module.go`
- Teste de regressao: `TestConfigureProductIDs` comprova atualizacao atomica dos tres planos; testes de configuracao comprovam all-or-none e obrigatoriedade em producao.
- Validacao: pacotes de config, billing e repository aprovados.

### BUG-15 - Metrica de correcoes da reconciliacao nunca incrementava
- ID: BUG-15
- Severidade: major
- Origem: auditoria production-ready
- Estado: fixed
- Causa raiz: `billing_reconciliation_corrections_total` era criada no job, mas o use case que efetivamente aplicava as correcoes nao recebia nem incrementava o counter.
- Arquivos alterados: `internal/billing/application/usecases/reconcile_subscriptions.go`, `internal/billing/application/usecases/reconcile_subscriptions_test.go`, `internal/billing/infrastructure/jobs/handlers/reconciliation_job.go`
- Teste de regressao: suite de reconciliacao comprova incremento apenas apos correcao bem-sucedida.
- Validacao: pacote de use cases aprovado com race detector em tres repeticoes.

### BUG-16 - Migration bem-sucedida retornava falha por indisponibilidade OTLP
- ID: BUG-16
- Severidade: major
- Origem: smoke production-ready
- Estado: fixed
- Causa raiz: o shutdown agregava erro critico do banco e erro best-effort de flush de observabilidade, produzindo exit code 1 depois de aplicar a migration e induzindo retries operacionais perigosos.
- Arquivos alterados: `cmd/migrate/migrate.go`, `cmd/migrate/migrate_test.go`
- Teste de regressao: `TestRuntimeShutdownDoesNotFailSuccessfulMigrationOnTelemetryFlushError` e `TestRuntimeShutdownPreservesDatabaseShutdownError` comprovam que apenas o erro critico continua fatal.
- Validacao: `go test -race -count=1 ./cmd/migrate` aprovado.

### BUG-17 - Contrato real de integracao Kiwify nao validado em sandbox
- ID: BUG-17
- Severidade: critical
- Origem: ADR-002, tasks L-01/L-03 e auditoria production-ready
- Estado: blocked
- Causa raiz: a documentacao publica consultada nao confirma o header/algoritmo de assinatura assumido, nem o campo real de tracking e a semantica de `updated_at_start_date`; o workspace nao possui credenciais de sandbox para executar a validacao obrigatoria.
- Arquivos alterados: nenhum; nao e seguro alterar o contrato por suposicao.
- Teste de regressao: bloqueado por dependencia externa; exige webhook e venda reais em sandbox Kiwify.
- Validacao: pendente validar `X-Kiwify-Signature`/HMAC-SHA256, `tracking.s1`/`src` e janela de reconciliacao contra o sandbox.

## Comandos Executados

- `go test -count=1 ./...` -> aprovado.
- `go test -race -count=1 ./...` -> aprovado.
- `go test -race -count=1 -tags=integration ./...` -> aprovado.
- `go test -race -count=3 ./internal/platform/worker/job ./internal/billing/application/usecases ./internal/identity/infrastructure/messaging/database/consumers` -> aprovado.
- `go build ./...` -> aprovado.
- `go vet ./...` -> aprovado.
- `golangci-lint run ./...` -> aprovado, `0 issues`.
- `git diff --check` -> aprovado.
- Smoke real local com Postgres, `go run ./cmd migrate`, server, worker e webhook assinado -> `http=202 active_subscriptions=1 published_outbox=1 pending_entitlements=1`.

## Riscos Residuais

- Gate externo obrigatorio: validar em sandbox Kiwify o protocolo real de assinatura, propagacao do funnel token e semantica da janela de reconciliacao antes de promover para producao.
- Drift documental: RF-20 cita uma transicao/evento `PAST_DUE -> EXPIRED`, enquanto o fluxo de negocio, ADR-005 e techspec §5.3 determinam decisao temporal em runtime sem persistir essa transicao. A implementacao preserva ADR-005 e o fluxo detalhado.
- `scripts/verify-go-mod.sh`, citado pela governanca de implementacao Go, nao existe no working tree.
