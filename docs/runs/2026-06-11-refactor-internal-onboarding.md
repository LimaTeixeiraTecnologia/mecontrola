# Refactor `internal/onboarding` — Plano

## Contexto

O prompt enriquecido em `docs/refactors/onboarding.md` pede para reduzir parser/branching em `module.go` e na use case `ConsumeMagicToken`, aplicar DDD tático e usar factories apenas onde houver invariantes reais. A exploração confirma três hotspots:

1. `internal/onboarding/module.go` concentra três helpers de parsing (`parseCheckoutURLs`, `parseCSV`, `buildMessagesMap`) e os invoca a partir de quatro métodos diferentes do builder, misturando bootstrap com regra de configuração.
2. `application/usecases/consume_magic_token.go` (`handlePaidToken`, linhas 179–223) e `application/usecases/try_fallback_activation.go` (`executeInTx`, linhas 87–123) executam a mesma coreografia: `identityGateway.UpsertUserByWhatsApp` → `subscriptionBinder.BindUser` → `magicToken.MarkConsumed` → `tokenRepo.UpdateMarkConsumed` → construir payload via `buildSubscriptionBoundPayload` → `outbox.NewEvent` → `publisher.Publish`. São ~40 linhas duplicadas com nomes de erro divergentes.
3. `buildSubscriptionBoundPayload` é função livre no pacote `usecases`, replicada conceitualmente em duas use cases e sem encapsular o tipo de evento (`onboarding.subscription_bound`, `AggregateType` etc.).

Handlers HTTP, jobs e consumers já estão finos (verificação direta nos quatro caminhos do R-ADAPTER-001). O `domain/services/TransitionService` existe mas não é usado por `ConsumeMagicToken`; nada disso precisa mudar agora — o escopo do refactor é application + módulo.

Resultado esperado: `module.go` deixa de carregar parsing, as duas use cases passam a delegar a coreografia compartilhada a um application service concreto e o evento outbox é produzido por um único construtor. Comportamento público (outcomes, métricas, logs estruturados, payload do evento) preservado.

## Decisões aprovadas

- Coreografia compartilhada: **application service concreto** `SubscriptionBindingService` em `internal/onboarding/application/services/` — struct, sem interface, injetada nas duas use cases.
- Parsing de config: **factory function** `NewOnboardingRuntimeConfig(cfg, waCfg) (OnboardingRuntimeConfig, error)` produzindo um value object imutável com os mapas/slices já tipados.

## Mudanças

### 1. Factory de configuração de runtime

**Novo arquivo**: `internal/onboarding/config/runtime.go`

- Pacote `config`.
- `OnboardingRuntimeConfig` (struct) com campos públicos: `CheckoutURLs map[string]string`, `KiwifyAllowedHosts []string`, `TrustedProxies []string`, `CheckoutCORSOrigins []string`, `Messages map[string]string`, `TokenTTL time.Duration`, `OutreachGap time.Duration`, `MetaRetention time.Duration`.
- Construtor `NewOnboardingRuntimeConfig(cfg cfgmodule.OnboardingConfig, waCfg cfgmodule.WhatsAppConfig) (OnboardingRuntimeConfig, error)` faz parse uma única vez, retorna erro se `KiwifyCheckoutURLs` produzir mapa vazio ou se uma chave duplicar.
- Funções privadas `parseKeyValueLines` e `parseCSV` ficam confinadas a este pacote.
- Mensagens compõem o mapa hoje em `buildMessagesMap`; movido como construtor estático (sem dependência de runtime).

**Atualizações em `module.go`**:

- `moduleBuilder` ganha campo `runtimeCfg config.OnboardingRuntimeConfig`, populado no início de `NewOnboardingModule` (falha rápido em erro).
- Remover `parseCheckoutURLs`, `parseCSV`, `buildMessagesMap`.
- `buildUseCases` consome `runtimeCfg.CheckoutURLs`, `runtimeCfg.KiwifyAllowedHosts`, `runtimeCfg.TokenTTL`, etc.
- `buildPublicRouter` consome `runtimeCfg.TrustedProxies`, `runtimeCfg.CheckoutCORSOrigins`.
- `buildMessageProcessor` consome `runtimeCfg.Messages`.

### 2. Application service para vinculação de assinatura

**Novo arquivo**: `internal/onboarding/application/services/subscription_binding.go`

- Pacote `services` (mesmo do `whatsapp_message_processor.go`).
- Struct concreta `SubscriptionBindingService` com dependências: `identityGateway appinterfaces.IdentityGateway`, `subscriptionBinder appinterfaces.SubscriptionBinder`, `publisher outbox.Publisher`, `idGen id.Generator`.
- Construtor `NewSubscriptionBindingService(...)`.
- Método único:
  ```
  BindAndConsume(
      ctx context.Context,
      tokenRepo appinterfaces.MagicTokenRepository,
      magicToken entities.MagicToken,
      fromE164 string,
      path valueobjects.ActivationPath,
      now time.Time,
  ) (entities.MagicToken, error)
  ```
- Sequência: upsert user → bind subscription → `magicToken.MarkConsumed` → `tokenRepo.UpdateMarkConsumed` → constrói evento via novo `events.NewSubscriptionBoundEvent` → publish.
- Erros são wrapped com prefixo neutro `onboarding: bind subscription: ...` (cada use case envolve novamente com seu prefixo via `fmt.Errorf("...: %w", err)` se quiser preservar mensagens — verificado nos testes existentes; ajustar testes apenas se a string assertada mudar).

### 3. Construtor explícito do evento outbox

**Novo arquivo**: `internal/onboarding/application/events/subscription_bound.go`

- Pacote `events`.
- Função pura `NewSubscriptionBoundEvent(eventID, userID string, token entities.MagicToken, path valueobjects.ActivationPath, boundAt time.Time) (outbox.Event, error)`.
- Encapsula `Type = "onboarding.subscription_bound"`, `AggregateType = "onboarding_token"`, `AggregateID = token.ID()`, `Payload` (mesmo JSON atual, com `token_hash_prefix` derivado).
- Remove `buildSubscriptionBoundPayload` de `consume_magic_token.go`.

### 4. Use cases enxutas

**`application/usecases/consume_magic_token.go`**:

- Campo `binding *services.SubscriptionBindingService` substitui `identityGateway`, `subscriptionBinder`, `publisher`, `idGen` (mantém apenas `uow`, `factory`, `o11y`, métricas).
- `NewConsumeMagicToken` ganha um parâmetro `binding *services.SubscriptionBindingService` no lugar dos quatro campos removidos.
- `handlePaidToken` reduz para: chamar `uc.binding.BindAndConsume(...)`, registrar `slog.InfoContext("onboarding.token.consumed", ...)`, retornar `ConsumeInternalResult{magicToken: consumed}`.
- `handleConsumedToken` segue criando `SupportSignal` (não compartilhado com fallback).
- `mapResult` e métricas preservadas.

**`application/usecases/try_fallback_activation.go`**:

- Mesma redução: campo `binding *services.SubscriptionBindingService`; `executeInTx` faz `findToken` → checa expiração → delega ao serviço → loga.
- `publishBound` removido.

**`module.go`**:

- `buildUseCases` cria `bindingService := services.NewSubscriptionBindingService(identityGateway, subscriptionBinder, b.publisher, b.idGen)` e injeta nas duas use cases.

### 5. Helpers locais

- `maskMobile` permanece em `consume_magic_token.go` (usado também em logs do `try_fallback_activation.go`); mover para um arquivo privado `application/usecases/mask.go` para deduplicar import sem criar pacote novo.
- `consumeResultLabel` continua privado em `consume_magic_token.go`.

### 6. Testes

Arquivos a atualizar (mesmas asserções, novas dependências injetadas no setup):

- `internal/onboarding/application/usecases/consume_magic_token_test.go`
- `internal/onboarding/application/usecases/try_fallback_activation_test.go`

Strings de erro permanecem estáveis se mantivermos os wraps `fmt.Errorf("onboarding: consume magic token: ...: %w", err)` na use case envolvendo o erro do serviço. Validar caso a caso e ajustar se algum teste asserta substring específica.

Não adicionar teste unitário separado para `SubscriptionBindingService` — comportamento integral é exercido pelas duas use cases existentes (sem ganho marginal). Para `OnboardingRuntimeConfig`, adicionar `runtime_test.go` cobrindo parsing válido, linha vazia ignorada, chave duplicada e CSV vazio (4 casos table-driven).

## Arquivos críticos

Criar:
- `internal/onboarding/config/runtime.go`
- `internal/onboarding/config/runtime_test.go`
- `internal/onboarding/application/services/subscription_binding.go`
- `internal/onboarding/application/events/subscription_bound.go`
- `internal/onboarding/application/usecases/mask.go`

Modificar:
- `internal/onboarding/module.go` (remoção dos 3 helpers + uso do RuntimeConfig + injeção do BindingService)
- `internal/onboarding/application/usecases/consume_magic_token.go`
- `internal/onboarding/application/usecases/try_fallback_activation.go`
- `internal/onboarding/application/usecases/consume_magic_token_test.go`
- `internal/onboarding/application/usecases/try_fallback_activation_test.go`

Não tocar:
- Domain (`entities`, `valueobjects`, `domain/services/transitions.go`).
- Adapters (`infrastructure/http/server/handlers/`, `infrastructure/jobs/handlers/`, `infrastructure/messaging/database/consumers/`, `infrastructure/messaging/database/producers/`).
- `kiwify_url_builder.go` (já enxuto).

## Restrições reforçadas (já endereçadas pelo design)

- Zero comentários Go nos arquivos novos/alterados (R-ADAPTER-001.1).
- Nenhuma interface nova introduzida no lado produtor; `SubscriptionBindingService` é struct concreta consumida por use cases.
- `context.Context` em toda fronteira; `time.Now().UTC()` mantido inline.
- Sem `init`, `panic`, `log.Fatal`, `os.Exit`, globals mutáveis.
- Sem abstração de tempo; sem `var _ Interface = (*Type)(nil)`.
- Sem mover regra para adapters; matriz R-ADAPTER-001.3 inalterada.

## Verificação

1. Build e lint:
   - `gofmt -l internal/onboarding`
   - `go vet ./internal/onboarding/...`
   - `go build ./...`
2. Gates do R-ADAPTER-001 (devem voltar vazio):
   - `grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" --exclude="*_test.go" "^[[:space:]]*//" internal/onboarding | grep -Ev "(//go:|//nolint:|// Code generated)"`
   - `grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" internal/onboarding/infrastructure/{http/server/handlers,messaging/database/consumers,messaging/database/producers,jobs/handlers}`
3. Testes:
   - `go test ./internal/onboarding/...`
   - Confirmar zero regressão em outcomes (`ConsumeOutcomeActivated`, `ConsumeOutcomeAlreadyActive`, `ConsumeOutcomeReuseOtherAccount`, `ConsumeOutcomeNotYetPaid`, `ConsumeOutcomeExpired`, `ConsumeOutcomeNotFound`) e em `FallbackOutcome*`.
4. Smoke de wiring: `go run ./cmd/server -h` (ou `go build ./cmd/server`) para garantir que a factory de RuntimeConfig não quebra o bootstrap.
5. Checklist R0–R7 da skill `go-implementation/references/build.md` reportado no resumo final.

## Pós-aprovação

Após `ExitPlanMode`, antes da execução, replicar este plano em `docs/runs/2026-06-11-refactor-internal-onboarding.md` conforme regra de memória registrada.
