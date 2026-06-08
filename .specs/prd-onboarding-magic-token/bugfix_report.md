# Relatorio de Bugfix

- Total de bugs no escopo: 7
- Corrigidos: 7
- Testes de regressao adicionados: 7
- Pendentes: nenhum
- Estado final: done

## Bugs
- ID: BUG-01
- Severidade: major
- Origem: finding de review; RF-09; task 6.0
- Estado: fixed
- Causa raiz: o fluxo persistia apenas `token_hash`; o job de outreach nao tinha material recuperavel para enviar `ATIVAR <token>` e passava string vazia ao gateway.
- Arquivos alterados: `internal/onboarding/domain/entities/magic_token.go`, `internal/onboarding/application/usecases/create_checkout_session.go`, `internal/onboarding/application/usecases/send_outreach.go`, `internal/onboarding/infrastructure/crypto/token_cipher.go`, `migrations/0010_create_onboarding_schema_and_tokens.up.sql`, `configs/config.go`, `.env.example`
- Teste de regressao: `TestCreateCheckoutSession_PersistsEncryptedTokenForOutreach`, `TestSendOutreach/TestSuccess_SendsToAllCandidates`, `TestTokenCipher_RoundTrip`
- Validacao: `go test ./...`, `go test -race -count=1 ./internal/onboarding/... ./internal/billing/... ./internal/identity/... ./configs/...`, `golangci-lint run ./internal/onboarding/... ./internal/billing/... ./internal/identity/... ./configs/...`

- ID: BUG-02
- Severidade: major
- Origem: finding de review; RF-09; task 6.0
- Estado: fixed
- Causa raiz: `WhatsAppGateway.SendActivationTemplate` ignorava o parametro `token` e chamava o cliente Meta com `components=nil`.
- Arquivos alterados: `internal/onboarding/infrastructure/gateway/whatsapp_gateway.go`
- Teste de regressao: `TestWhatsAppGateway_SendActivationTemplateIncludesAtivarToken`
- Validacao: `go test ./...`, `go vet ./...`, `golangci-lint run ./internal/onboarding/... ./internal/billing/... ./internal/identity/... ./configs/...`

- ID: BUG-03
- Severidade: major
- Origem: finding de review; RF-07; task 4.0
- Estado: fixed
- Causa raiz: o consumer recebia `subscription_id`, mas `MarkTokenPaidInput` descartava o campo; sem `subscription_id` no token, `ConsumeMagicToken` nao conseguia vincular `billing_subscriptions.user_id` na mesma UoW.
- Arquivos alterados: `internal/onboarding/application/dtos/input/mark_token_paid_input.go`, `internal/onboarding/domain/entities/magic_token.go`, `internal/onboarding/application/usecases/mark_token_paid.go`, `internal/onboarding/application/usecases/consume_magic_token.go`, `internal/onboarding/infrastructure/messaging/database/consumers/subscription_paid_consumer.go`, `internal/onboarding/infrastructure/repositories/postgres/subscription_binder.go`, `internal/onboarding/module.go`, `migrations/0010_create_onboarding_schema_and_tokens.up.sql`
- Teste de regressao: `TestSubscriptionPaidConsumer/TestHandleCallsMarkTokenPaid`, `TestMarkTokenPaid_UsesDecodedMagicTokenHashAndStoresSubscriptionID`, `TestConsumeMagicToken/TestTokenPaid_BindsSubscriptionAndPublishesSubscriptionID`
- Validacao: `go test ./...`, `go vet ./...`, `go build ./cmd/server ./cmd/worker`

- ID: BUG-04
- Severidade: major
- Origem: finding de review; task 8.0
- Estado: fixed
- Causa raiz: `onboarding.subscription_bound` era publicado sem `subscription_id`; o projector de identity faz no-op quando o campo esta vazio.
- Arquivos alterados: `internal/onboarding/application/usecases/consume_magic_token.go`, `internal/onboarding/application/usecases/try_fallback_activation.go`
- Teste de regressao: `TestConsumeMagicToken/TestTokenPaid_BindsSubscriptionAndPublishesSubscriptionID`
- Validacao: `go test ./...`, `go test -race -count=1 ./internal/onboarding/... ./internal/billing/... ./internal/identity/... ./configs/...`

- ID: BUG-05
- Severidade: major
- Origem: finding de review; RF-12; task 7.0
- Estado: fixed
- Causa raiz: o payload de `orphan_expired_subscription` usava `token.ID()` no campo `token_hash_prefix`, descumprindo o contrato de suporte baseado no hash do token.
- Arquivos alterados: `internal/onboarding/application/usecases/expire_tokens.go`
- Teste de regressao: `TestExpireTokens/TestPAIDExpiredTokenEmitsOrphanSignal`
- Validacao: `go test ./...`, `go vet ./...`

- ID: BUG-06
- Severidade: minor
- Origem: finding de review; RF-16; task 4.0
- Estado: fixed
- Causa raiz: o handler inbound prefixava `+` no numero Meta e chamava os use cases sem validar o formato BR pelo value object de identity.
- Arquivos alterados: `internal/onboarding/infrastructure/http/server/handlers/whatsapp_inbound_handler.go`
- Teste de regressao: `TestWhatsAppInboundHandler/TestUnsupportedCountry_DoesNotDispatchUseCases`
- Validacao: `go test ./...`, `go vet ./...`

- ID: BUG-07
- Severidade: minor
- Origem: finding de review; lint
- Estado: fixed
- Causa raiz: arquivos alterados estavam fora de goimports, `configLoader.load` excedia limite de statements e `process_kiwify_webhook.go` tinha conversao manual reportada por staticcheck.
- Arquivos alterados: `configs/config.go`, `internal/billing/application/usecases/process_kiwify_webhook.go`, arquivos Go formatados por `goimports`
- Teste de regressao: `golangci-lint run ./internal/onboarding/... ./internal/billing/... ./internal/identity/... ./configs/...`
- Validacao: `golangci-lint run ./internal/onboarding/... ./internal/billing/... ./internal/identity/... ./configs/... -> 0 issues`

## Comandos Executados
- `go test ./internal/onboarding/... ./internal/identity/... ./internal/billing/application/usecases/... ./configs/...` -> pass
- `golangci-lint run ./internal/onboarding/... ./internal/billing/... ./internal/identity/... ./configs/...` -> pass, 0 issues
- `go test ./...` -> pass
- `go vet ./...` -> pass
- `go build ./cmd/server ./cmd/worker` -> pass
- `go test -race -count=1 ./internal/onboarding/... ./internal/billing/... ./internal/identity/... ./configs/...` -> pass

## Riscos Residuais
- Tokens criados antes da coluna `activation_token_ciphertext` existir nao terao material recuperavel para outreach; em ambientes com migration ja aplicada, sera necessaria migration/backfill operacional ou reemissao humana desses tokens.
- A pagina Astro externa continua fora deste repositorio; WCAG/axe-core seguem como contrato documentado para o PR coordenado da landing.
