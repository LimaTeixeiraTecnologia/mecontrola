# Relatorio de Bugfix — Review 2026-06-09

- Total de bugs no escopo: 10 (review 2026-06-09) + 17 (review 2026-06-08 anterior, preservado abaixo)
- Corrigidos nesta rodada: 10
- Rejeitados como falso positivo: 0
- Testes de regressao adicionados ou corrigidos: 5 novos + 3 ajustados
- Pendentes: 0
- Estado final: fixed

## Bugs (review 2026-06-09)

### BUG-18 — Transicao PAST_DUE -> EXPIRED nunca era materializada nem notificada
- ID: BUG-18
- Severidade: critical
- Origem: review 2026-06-09 finding #1 (RF-20)
- Estado: fixed
- Causa raiz: `billing.subscription.expired_after_grace` era declarado em `events.go` e o `NotificationHandler` registrado em `module.go`, mas nao havia produtor, use case, nem job. Subscriptions em `PAST_DUE` com `grace_end` vencido permaneciam no estado `PAST_DUE` indefinidamente. O gate de acesso funcionava via `IsEntitled` runtime (ADR-005), mas nenhuma notificacao chegava ao assinante.
- Arquivos alterados:
  - `internal/billing/domain/services/transitions.go` (novo `TriggerGraceExpired` + linha `PastDue -> Expired` na tabela)
  - `internal/billing/domain/entities/subscription.go` (novo `MarkExpiredAfterGrace` + `applyExpired`)
  - `internal/billing/application/interfaces/subscription_event_publisher.go` (novo `PublishExpired`)
  - `internal/billing/application/interfaces/subscription_repository.go` (novo `ListPastDueGraceExpired` + struct `ExpiredGraceCandidate`)
  - `internal/billing/infrastructure/messaging/database/producers/events.go` (`SubscriptionExpiredAfterGracePayload`)
  - `internal/billing/infrastructure/messaging/database/producers/subscription_event_publisher.go` (`PublishExpired`)
  - `internal/billing/infrastructure/repositories/postgres/subscription_repository.go` (query `ListPastDueGraceExpired`)
  - `internal/billing/application/usecases/process_subscription_grace_expired.go` (novo use case)
  - `internal/billing/infrastructure/jobs/handlers/grace_expiration_job.go` (novo job cron `@every 30m`)
  - `internal/billing/module.go` (wiring do job)
  - `cmd/worker/worker.go` (registro do `GraceExpirationJob`)
  - `configs/config.go` (env `BILLING_GRACE_EXPIRATION_SCHEDULE`)
  - `.specs/prd-billing-pipeline/prd.md` (nota de implementacao em RF-20, fechando drift #8)
- Teste de regressao: `ProcessSubscriptionGraceExpiredSuite/TestPastDueComGracaVencidaTransitaParaExpired` valida transicao+publicacao; `TestQuandoNaoExisteCandidatoNaoEmitePublicacao` valida no-op.
- Validacao: `go test -count=1 ./internal/billing/...` aprovado; `go vet` limpo.

### BUG-19 — Renovacao Kiwify criava assinatura paralela (placeholder) violando RF-17
- ID: BUG-19
- Severidade: high
- Origem: review 2026-06-09 finding #2
- Estado: fixed
- Causa raiz: `process_subscription_renewed.applyRenewal` buscava por `FindByOrderID(in.OrderID)`. No protocolo real Kiwify, `order_id` muda a cada cobranca de renovacao; a busca falhava e o fluxo caia em `createPlaceholder`, gerando uma subscription paralela com `funnel_token=""` e `user_id=NULL`. `extendExisting` ficava morto em producao e RF-17 era violada silenciosamente (constraint `billing_subscriptions_user_active_uniq_idx` nao disparava porque `user_id` era NULL).
- Arquivos alterados:
  - `internal/billing/application/interfaces/subscription_repository.go` (novo `FindByKiwifySubID`)
  - `internal/billing/infrastructure/repositories/postgres/subscription_repository.go` (impl `FindByKiwifySubID`)
  - `internal/billing/application/usecases/process_subscription_renewed.go` (continuidade por `kiwify_subscription_id`; placeholder removido; novo `ErrRenewedWithoutBaseSubscription`)
- Teste de regressao: `ProcessSubscriptionRenewedSuite/TestExecute` cenario "deve retornar erro quando renovacao chegar sem assinatura base por kiwify_sub_id" + "deve estender periodo de assinatura ativa" agora usando `FindByKiwifySubID`.
- Validacao: `go test -count=1 ./internal/billing/application/usecases/` aprovado.

### BUG-20 — PAST_DUE e CANCELED_PENDING bloqueados em producao por uso indevido de order_id
- ID: BUG-20
- Severidade: high
- Origem: review 2026-06-09 finding #3
- Estado: fixed
- Causa raiz: o dispatcher em `process_kiwify_webhook.go` propagava `OrderID: p.OrderID` para `ProcessSubscriptionLateInput` e `ProcessSubscriptionCanceledInput`, mas `p.OrderID` nos eventos de atraso/cancelamento Kiwify e o da cobranca atual, nao o da venda original. `FindByOrderID(in.OrderID)` falhava e nenhuma transicao era aplicada.
- Arquivos alterados:
  - `internal/billing/application/usecases/process_subscription_late.go` (novo `resolveSubscription`: tenta `FindByKiwifySubID` primeiro, fallback para `FindByOrderID`)
  - `internal/billing/application/usecases/process_subscription_canceled.go` (idem)
- Teste de regressao: ajuste nos testes `ProcessSubscriptionLateSuite` e `ProcessSubscriptionCanceledSuite` para mockar `FindByKiwifySubID`; `process_subscription_renewed_test.go` cobre o caminho de continuidade.
- Validacao: `go test -count=1 ./internal/billing/application/usecases/` aprovado.

### BUG-21 — eventKey de refund nao incluia trigger, colidindo order_refunded com chargeback
- ID: BUG-21
- Severidade: medium
- Origem: review 2026-06-09 finding #4
- Estado: fixed
- Causa raiz: `eventKey := fmt.Sprintf("refund:%s", in.SaleID)` ignorava o trigger. Reentrega de `chargeback` apos `order_refunded` (mesma venda) era descartada como duplicidade, mascarando potencial correcao financeira legitima.
- Arquivos alterados: `internal/billing/application/usecases/process_refund_or_chargeback.go`.
- Teste de regressao: `ProcessRefundOrChargebackSuite/TestExecute` cenario "deve permitir reaplicar chargeback apos order_refunded pois chaves de evento sao independentes (regressao #4)".
- Validacao: `go test -count=1 ./internal/billing/application/usecases/` aprovado.

### BUG-22 — Migration 0004 deixava planos com placeholders quando KIWIFY_PRODUCT_ID_* nao estavam setados
- ID: BUG-22
- Severidade: medium
- Origem: review 2026-06-09 finding #5
- Estado: fixed
- Causa raiz: `migrations/0004_create_billing_plans.up.sql` inseria `('<id-mensal>', 'MONTHLY', ...)` literal. `module.go` so chamava `ConfigureProductIDs` quando ao menos uma env var estava setada. Em dev sem env, a tabela permanecia com placeholders sintaticos invalidos e qualquer `FindByKiwifyProductID` retornava `ErrPlanNotFound`. Fail-fast existia apenas em production.
- Arquivos alterados:
  - `migrations/0004_create_billing_plans.up.sql` (placeholders renomeados para `__PLACEHOLDER_*__`)
  - `internal/billing/module.go` (nova funcao `ensurePlansConfigured` que detecta envs vazios OU prefixadas com `__PLACEHOLDER_`, e falha o bootstrap)
- Teste de regressao: ajuste em `internal/billing/module_test.go` para cenario com envs preenchidos.
- Validacao: `go test -count=1 ./internal/billing/` aprovado.

### BUG-23 — UpsertByOrder nao persistia customer_mobile_e164, customer_email, external_sale_id
- ID: BUG-23
- Severidade: medium
- Origem: review 2026-06-09 finding #6
- Estado: fixed
- Causa raiz: `UpsertByOrder` antiga assinatura ignorava colunas adicionadas pela migration 0012, deixando-as permanentemente NULL no billing.
- Arquivos alterados:
  - `internal/billing/application/interfaces/subscription_repository.go` (nova struct `UpsertByOrderParams`)
  - `internal/billing/infrastructure/repositories/postgres/subscription_repository.go` (INSERT/UPDATE estendido com `kiwify_subscription_id`, `external_sale_id`, `customer_mobile_e164`, `customer_email` via `NULLIF($x, '')`)
  - `internal/billing/application/usecases/process_sale_approved.go` (passa todos os campos no params struct)
  - `internal/billing/application/dtos/input/process_sale_approved_input.go` (novo campo `KiwifySubID`)
- Teste de regressao: cobertura via `process_sale_approved_test.go` (ajustes de assinatura no mock); o teste E2E em `internal/billing/infrastructure/http/server/handlers/kiwify_webhook_integration_test.go` continua verde, exercitando o caminho real.
- Validacao: `go test -count=1 ./internal/billing/...` aprovado.

### BUG-24 — Loop de paginacao em reconcile sem teto podia entrar em laco infinito
- ID: BUG-24
- Severidade: medium
- Origem: review 2026-06-09 finding #7
- Estado: fixed
- Causa raiz: `for page := 1; ; page++` em `reconcile_subscriptions.go` apenas saia em `!HasMore`. Bug upstream na Kiwify ou paginacao inconsistente podia gerar loop infinito ate ctx cancelar.
- Arquivos alterados: `internal/billing/application/usecases/reconcile_subscriptions.go` (`reconcileMaxPages = 1000` const + `ErrReconcileMaxPagesExceeded` + log estruturado ao atingir).
- Teste de regressao: `ReconcileSubscriptionsSuite/TestMaxPagesGuard` simula `HasMore=true` por 1000 paginas e exige erro.
- Validacao: `go test -count=1 ./internal/billing/application/usecases/ -run TestMaxPagesGuard` aprovado.

### BUG-25 — Drift documental: RF-20 falava em PAST_DUE -> EXPIRED sem job, contradizendo ADR-005
- ID: BUG-25
- Severidade: medium
- Origem: review 2026-06-09 finding #8
- Estado: fixed
- Causa raiz: PRD versao 1 declarava `PAST_DUE -> EXPIRED` como transicao notificavel mas ADR-005 (decidida apos PRD) movera a decisao para runtime via `IsEntitled`. Apos BUG-18, agora HA materializacao via job; o PRD precisava ser atualizado para coerencia.
- Arquivos alterados: `.specs/prd-billing-pipeline/prd.md` (nota de implementacao agregada em RF-20 referenciando ADR-005 e o `ExpireGraceJob`).
- Teste de regressao: N/A (documental).
- Validacao: leitura cruzada manual.

### BUG-26 — Status rotated aceito sem metrica de observabilidade
- ID: BUG-26
- Severidade: low
- Origem: review 2026-06-09 finding #9
- Estado: fixed
- Causa raiz: `computeSignatureStatus` aceitava `rotated` (assinatura valida via `KIWIFY_WEBHOOK_SECRET_NEXT`) sem emitir metrica especifica. Rotacao de secret invisivel operacionalmente.
- Arquivos alterados: `internal/billing/application/usecases/process_kiwify_webhook.go` (novo counter `billing_webhook_signature_rotated_total` + log estruturado).
- Teste de regressao: cobertura via os testes existentes de `KiwifyWebhookHandler` (signature flow); o counter e instrumentacao, validavel em smoke real.
- Validacao: `go test -count=1 ./internal/billing/...` aprovado.

### BUG-27 — Blank assigns em scanRow ocultavam drift e nao propagavam userID/kiwifySubID
- ID: BUG-27
- Severidade: low
- Origem: review 2026-06-09 finding #10
- Estado: fixed
- Causa raiz: `subscription_repository.scanRow` lia 11 colunas mas descartava 3 com `_ = orderID; _ = userID; _ = kiwifySubID`. Pratica oculta intencao e impedia future-proofing.
- Arquivos alterados: `internal/billing/infrastructure/repositories/postgres/subscription_repository.go` (scanRow refatorado para ler apenas 9 colunas necessarias; query SELECT enxuto via `subscriptionSelectColumns`).
- Teste de regressao: cobertura via integration tests existentes em `migrations/migrations_integration_test.go` e suite Postgres.
- Validacao: `go vet ./...` limpo; `go test -count=1 ./internal/billing/...` aprovado.

## Comandos Executados (evidencia)

- `go vet ./...` -> exit 0 (limpo)
- `go test -count=1 ./...` -> exit 0 (todos os 47+ pacotes verde)
- `task lint:run` -> 3 issues residuais em `internal/onboarding/infrastructure/http/server/middleware/rate_limit_test.go` e `internal/onboarding/infrastructure/jobs/handlers/token_expiration_job_test.go` (PRE-EXISTENTES, fora do escopo billing); ZERO issues nas mudancas desta rodada.
- `task mocks:mocks` -> regen aprovada para billing; erros pre-existentes em onboarding nao introduzidos por esta rodada.
- `go build ./...` -> exit 0.

## Riscos Residuais

- O job `ExpireGraceJob` carrega plano e funnel token vazios na hidratacao da subscription dentro do use case (`expireOne`); o evento outbox carrega apenas `subscription_id`, `period_end`, `grace_end` e `occurred_at`. O `NotificationHandler` precisa apenas do `subscription_id` para resolver via projector identity — ok para o MVP. Se notificacoes futuras precisarem do plan/funnel, considerar enriquecer a hidratacao via leitura de `FindByID` adicional.
- O fallback `executeWithoutToken` em `process_sale_approved.go` permanece como decisao de produto pre-existente (nao escopo desta rodada). RF-03 ("compras sem token devem ser rejeitadas") merece confirmacao posterior do produto, conforme observado no review.
- Mocks em `internal/billing/application/usecases/mocks/` foram sincronizados manualmente a partir de `internal/billing/application/interfaces/mocks/`. O processo correto de longo prazo seria adicionar essas interfaces ao `mockery.yml` ou remover a duplicacao (drift ja documentado).
- `scripts/verify-go-mod.sh` referenciado pela governanca ainda nao existe (drift herdado, fora deste escopo).

---

## Historico — Review 2026-06-08 (preservado para auditoria)

- Total de bugs no escopo: 17
- Corrigidos: 15
- Rejeitados como falso positivo: 1
- Testes de regressao adicionados ou corrigidos: 15
- Pendentes: 1 validacao externa obrigatoria
- Estado final: blocked (resolvido posteriormente em commits `e99999d`, `9e3c7e5`, `bda45d1`, `ef50573`)

### BUGs 1-17 — ver historico anterior em git log
- BUG-01 a BUG-17 corrigidos na rodada anterior. BUG-17 (validacao real do protocolo Kiwify) resolvido via commits citados acima apos sandbox real.
