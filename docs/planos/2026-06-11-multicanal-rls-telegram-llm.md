# Plano — Multi-tenant RLS + Telegram + LLM Real

> **Skills obrigatórias:** `go-implementation` (R0–R7 + matriz R-ADAPTER-001.3 por tipo de adapter). Carregar `architecture.md` + refs por tarefa (handler → `api.md` + `http-handler.md`; consumer → `messaging.md` + `consumer.md`; producer → `messaging.md` + `producer.md`).

## Context

O prompt colado pelo usuário descreve um sistema financeiro multicanal greenfield. O repositório `mecontrola` já entrega ~70% disso (WhatsApp + identity + billing + budgets + cards + categories + outbox), mas com arquitetura DDD estrita governada por `AGENTS.md` / `R-ADAPTER-001` (zero comentários, handlers finos, application/domain/infrastructure). Reescrever no estilo `internal/modules/<x>/handler.go` violaria a governança.

Decisões consolidadas com o usuário:

- **Adaptar à arquitetura atual** (DDD + R-ADAPTER-001 + zero comentários). Tratar o prompt como spec funcional, não como código literal.
- **RLS com tenant = user** (sem nova tabela `tenants`). `ENABLE/FORCE` RLS em todas as tabelas de dados de usuário, isolando por `user_id` via `SET LOCAL app.current_user_id` injetado no início de cada transação.
- **Adapter Telegram** espelhando `internal/platform/whatsapp/`, com nova tabela `mecontrola.tenant_identities (channel, external_id, user_id)` desacoplando canal de usuário e migrando o atual `users.whatsapp_number`.
- **OpenRouter** substituindo `StubAgent`, com fallback chain (Gemini → GPT-4.1-nano → DeepSeek → Mistral), system prompt com contexto injetado e `ValidateIntent`.

Fora de escopo desta passada: módulo `transactions` próprio, tabela `tenants` separada, audit append-only (revisitado depois — `budgets` já emite `external.expense.v1`).

---

## Estratégia em camadas

### Camada 1 — RLS sem reescrever repositórios

**Problema:** repositórios usam `uow.Do(ctx, func(ctx, tx database.DBTX) (T, error))`. Inserir `SET LOCAL` em cada query forçaria mudar dezenas de arquivos.

**Solução:** envolver `manager.Manager`/`uow.UnitOfWork` num decorator que, ao abrir transação, lê `auth.Principal` do `ctx` e executa `SET LOCAL app.current_user_id = <uuid>` como primeira statement. Sem principal: erro explícito (rejeita queries pré-autenticação).

**Arquivos:**
- Novo: `internal/platform/tenancy/rls_manager.go` — decorator de `manager.Manager`/`uow.UnitOfWork` injetando `SET LOCAL`. Reutiliza `auth.FromContext` (`internal/identity/.../auth`).
- Editar: `cmd/server/server.go` (~linha 68) — empacotar `dbManager` com o decorator antes de injetar nos módulos.
- Editar: `cmd/worker/worker.go` — mesmo wrap. Para dispatchers de outbox global, usar `tenancy.WithSystemBypass(ctx)` (helper que sinaliza ao decorator para pular o SET LOCAL ou usar role superuser).

**Reuso:** `manager.Manager` de `devkit-go/pkg/database/manager`, `database.DBTX`, `auth.Principal`.

### Camada 2 — Migrations RLS + tenant_identities

Migration `000014_rls_and_tenant_identities.{up,down}.sql`:

1. `CREATE TABLE mecontrola.tenant_identities (id uuid PK, user_id uuid FK users, channel text, external_id text, verified_at timestamptz, created_at timestamptz, UNIQUE(channel, external_id))`.
2. Backfill: `INSERT INTO tenant_identities (user_id, channel, external_id, verified_at) SELECT id, 'whatsapp', whatsapp_number, created_at FROM users WHERE whatsapp_number IS NOT NULL`.
3. `ALTER TABLE ... ENABLE/FORCE ROW LEVEL SECURITY` + `CREATE POLICY tenant_isolation USING (user_id = current_setting('app.current_user_id')::uuid)` em: `users`, `identity_entitlements`, `billing_subscriptions`, `billing_subscription_events`, tabelas `budgets_*`, `cards`, `onboarding_tokens`, `tenant_identities`. **Não aplicar** em: `outbox_events`, `meta_processed_messages`, `platform_idempotency_keys`, `categories`, `categories_dictionary*` (globais/operacionais).
4. **Não dropar** `users.whatsapp_number` ainda — deprecar em migration futura quando código parar de ler.

### Camada 3 — Resolução de identidade por canal

**Problema:** hoje `EstablishPrincipal` busca por `whatsapp_number` direto em `users`. Com `tenant_identities`, vira `(channel, external_id) → user_id`.

**Arquivos:**
- Novo: `internal/identity/domain/entities/tenant_identity.go`.
- Novo: `internal/identity/domain/interfaces/tenant_identity_repository.go`.
- Novo: `internal/identity/infrastructure/repositories/postgres_tenant_identity.go`.
- Novo: `internal/identity/application/usecases/establish_principal_by_identity.go` — usecase fino orquestrando lookup + entitlement check.
- Editar: `internal/platform/whatsapp/dispatcher/dispatcher.go` (~linha 132) — passar `("whatsapp", waID)` ao novo usecase. Manter usecase antigo enquanto WhatsApp existente migra.

### Camada 4 — Adapter Telegram

Espelho de `internal/platform/whatsapp/`:

- Novo pacote `internal/platform/telegram/` com subpastas `handlers/`, `signature/`, `payload/`, `dispatcher/`, `outbound/`.
- Dedup e rate limit: unificar em `mecontrola.tenant_processed_messages(channel, message_id, ...)` via migration `000015`. Atualizar adapter atual do WhatsApp para usar a nova tabela.
- Signature: validar header `X-Telegram-Bot-Api-Secret-Token` com `hmac.Equal` em tempo constante.
- Payload: parsear `update.message.{from.id, text, message_id}`. `external_id = strconv.FormatInt(from.id, 10)`. `message_id = fmt.Sprintf("tg_%d_%d", update_id, from_id)`.
- Dispatcher: extrair routing onboarding/agent para `internal/platform/channels/router.go` genérico (`ChannelKey + ExternalID + Text + callbacks`). WhatsApp e Telegram viram thin wrappers.
- Config: `TelegramConfig` em `configs/config.go` (token, secret token, bot username, templates, rate limits).
- Wiring: `cmd/server/telegram_wiring.go` espelhando `whatsapp_wiring.go`. Registrar `composeTelegramWebhookRouter()` em `server.go`. Endpoint: `/api/v1/telegram/webhook`.
- Outbound: gateway Telegram em `internal/platform/telegram/outbound/`. `payload.Message` consumido pelo agent ganha campo `Channel` para responder via o canal correto.

### Camada 5 — LLM Real (OpenRouter) substituindo StubAgent

Novo pacote `internal/agent/openrouter/` seguindo DDD:

- `internal/agent/domain/intent.go` — `IntentResult` struct + invariantes.
- `internal/agent/application/interfaces/provider.go` — `Provider.Interpret(ctx, systemPrompt, userMsg) (*IntentResult, error)`.
- `internal/agent/application/usecases/interpret_message.go` — fallback chain via `errors.Join`.
- `internal/agent/application/services/prompt_builder.go` — system prompt com categorias + cards do usuário (consulta `categories.Repository`, `cards.Repository`); cache curto em memória por `user_id` (TTL 5 min).
- `internal/agent/application/services/intent_validator.go` — `ValidateIntent` do prompt §7.5: rejeita `tenant_id`/`user_id` em payload/filters, valida module/action contra allowlists.
- `internal/agent/application/services/intent_dispatcher.go` — roteia intent para usecases existentes (`categories.application.usecases.*`, `cards.application.usecases.*`, `budgets.application.usecases.*`). Intents `transactions.*` retornam "em breve".
- `internal/agent/infrastructure/providers/openrouter.go` — adapter HTTP usando `internal/platform/httpclient.Client` (retry/timeout/observability).
- `internal/agent/infrastructure/handler/agent_handler.go` — implementa `AgentHandler.HandleMessage`, orquestra interpret → validate → dispatch → reply via gateway do canal.
- Configs: `AgentConfig { OpenRouterAPIKey, PrimaryModel, FallbackModels []string, MaxTokens, Temperature, TimeoutSeconds }`. Secret via env/Docker secret.

**Wiring:**
- Editar `cmd/server/whatsapp_wiring.go` (~linha 31) — trocar `agent.NewStubAgent(...)` por `openrouter.New(...)`.
- Mesmo wiring em `telegram_wiring.go`.
- Toggle `AGENT_MODE=stub|openrouter` (default `stub` em dev, `openrouter` em prod) para manter rollback rápido.

### Camada 6 — Pipeline de webhook autenticado

Ordem por request: `channel signature → dedup → rate limit → resolve identity → SET LOCAL (via tx) → dispatcher → agent/onboarding`. RLS isola automaticamente queries dos usecases pois `auth.Principal` já está no `ctx`.

---

## Arquivos críticos a modificar/criar

**Migrations**
- `migrations/000014_rls_and_tenant_identities.{up,down}.sql`
- `migrations/000015_unify_processed_messages.{up,down}.sql`

**Tenancy / RLS**
- `internal/platform/tenancy/rls_manager.go`
- `internal/platform/tenancy/context.go` (`WithSystemBypass`)

**tenant_identities**
- `internal/identity/domain/entities/tenant_identity.go`
- `internal/identity/domain/interfaces/tenant_identity_repository.go`
- `internal/identity/infrastructure/repositories/postgres_tenant_identity.go`
- `internal/identity/application/usecases/establish_principal_by_identity.go`

**Telegram**
- `internal/platform/telegram/handlers/{verify,inbound}_handler.go`
- `internal/platform/telegram/signature/secret_token.go`
- `internal/platform/telegram/payload/{message,types}.go`
- `internal/platform/telegram/dispatcher/dispatcher.go`
- `internal/platform/telegram/outbound/gateway.go`
- `internal/platform/channels/router.go`
- `cmd/server/telegram_wiring.go` + editar `cmd/server/server.go` para registrar a rota.
- Editar `configs/config.go` para `TelegramConfig`.

**Agent LLM**
- `internal/agent/domain/intent.go`
- `internal/agent/application/interfaces/provider.go`
- `internal/agent/application/usecases/interpret_message.go`
- `internal/agent/application/services/{prompt_builder,intent_validator,intent_dispatcher}.go`
- `internal/agent/infrastructure/providers/openrouter.go`
- `internal/agent/infrastructure/handler/agent_handler.go`
- Editar `cmd/server/whatsapp_wiring.go` e `telegram_wiring.go`.
- Editar `configs/config.go` para `AgentConfig`.

---

## Reuso obrigatório

- `manager.Manager` / `database.DBTX` / `uow.UnitOfWork[T]` — não recriar abstração de transação.
- `internal/platform/httpclient.Client` (com `WithTimeout`, retry) — para chamadas OpenRouter; não criar `http.Client` novo.
- `auth.Principal` + `auth.FromContext` — fonte canônica de user_id por request.
- `outbox.Publisher` — eventos do agent (ex.: `agent.intent.executed.v1`) via outbox existente com idempotência por `event_id`.
- `internal/platform/whatsapp/{dedup,ratelimit,signature/hmac.go}` como referência de padrão.
- Usecases existentes em `categories`/`cards`/`budgets` — invocados pelo `intent_dispatcher`; não duplicar regra.

---

## Governança a respeitar (inegociável)

- **Zero comentários em `.go`** (R-ADAPTER-001.1). Só `//go:build`, `//nolint:` com motivo na mesma linha, `// Code generated` em mocks.
- **Adapter fino** (R-ADAPTER-001.2): handlers, consumers, jobs, producers sem SQL direto, sem branching de domínio, sem regra de negócio. Tudo via usecase.
- **Skill `go-implementation`** carregada antes de editar; refs conforme matriz R-ADAPTER-001.3 (máx 4 simultâneas).
- **R0** sem `init()`; **R5.12** sem `panic` em produção; **R6** `context.Context` em fronteiras IO, interface no consumidor; **R7.6** `errors.Join` para fallback chain, `fmt.Errorf("ctx: %w", err)` para wrap.
- Sem asserção `var _ Iface = (*T)(nil)`. Sem abstração de tempo — `time.Now().UTC()` inline.

---

## Verificação end-to-end

**Unit**
- `RLSManager` injeta `SET LOCAL` no início da tx; sem principal retorna erro.
- Provider OpenRouter parseia resposta válida e propaga erro HTTP.
- `ValidateIntent` rejeita `user_id`/`tenant_id` em payload e filters.
- `establish_principal_by_identity` retorna `User` quando identity existe; erro quando não.

**Integration (Postgres real via testcontainers — padrão existente)**
- Query em `categories`/`budgets` sem `SET LOCAL` falha por policy.
- Com `SET LOCAL app.current_user_id = '<userA>'` retorna só linhas de A; trocar para `userB` na mesma conexão de pool não vaza (validar reset entre requisições).
- Webhook Telegram: payload válido + Secret Token correto → 200; sem token → 401.

**Comandos**
```
task lint
task test-unit
task test-integration
task vulncheck
```
Mais o gate de R-ADAPTER-001.1 (grep de comentários proibidos) e R-ADAPTER-001.2 (grep de SQL em adapters) de `.claude/rules/go-adapters.md`.

**Smoke manual**
1. `task up`.
2. Aplicar migrations 14 e 15.
3. `curl` Telegram webhook com payload + token → 200; sem token → 401.
4. `curl` WhatsApp webhook → 200, ainda funcional pós-RLS.
5. `SET app.current_user_id = '<uuid-A>'; SELECT count(*) FROM budgets_budgets;` retorna só os de A.

---

## Ordem de execução sugerida

1. Migration 14 (RLS + tenant_identities + backfill).
2. `internal/platform/tenancy/rls_manager.go` + wiring em `cmd/server` e `cmd/worker`.
3. Rodar suite de integration existente — deve continuar verde com RLS ativo (falha aqui revela handler/consumer sem principal).
4. `tenant_identities` repo + usecase + ligar no WhatsApp dispatcher.
5. Migration 15 (unificar processed_messages) + ajuste de dedup WhatsApp.
6. Telegram (signature → payload → dispatcher → wiring → handler).
7. `internal/platform/channels/router.go` compartilhado WA/Telegram.
8. Agent OpenRouter (provider → usecase → prompt → validator → intent_dispatcher → handler).
9. Toggle `AGENT_MODE` para alternar stub/openrouter.
10. Smoke + integration final + vulncheck.

Cada etapa é PR independente, com checklist R0–R7 e evidência de não-regressão. Etapas 2–3 são as mais arriscadas: se RLS quebrar handlers existentes, parar e tratar antes de prosseguir.
