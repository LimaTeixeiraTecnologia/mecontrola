# DOSSIÊ DE DISCOVERY TÉCNICO

## Título
MeControla — Fundação de Autenticação e Autorização (MVP WhatsApp + LLM in-process)

## Resumo Executivo
Contexto:
O MeControla precisa entregar `internal/card`, `internal/categories` e `internal/budgets` cujos PRDs declaram um middleware `RequireUser` transitório por header `X-User-ID` cru, sem AuthN real. A interface inicial é WhatsApp + LLM in-process; app móvel e web são evolução futura. A premissa de negócio é inegociável: nenhum usuário pode ver dados de outro. Já existem primitivos reutilizáveis em onboarding (HMAC SHA-256 com rotação, raw body buffer, rate-limit, dedup WAMID, parse Meta).

Recomendação:
Implementar a fundação como `auth.Principal` in-process via `context.Context` em `internal/identity/application/auth` + ADR documentando boundary HTTP futura (JWT Ed25519 para app/web). Centralizar parse/dedup/signature do WhatsApp em `internal/platform/whatsapp` com dispatcher único que roteia para onboarding (pré-ativação) ou para o agent (pós-ativação com Principal resolvido). Audit log via `outbox.Publisher` → tabela `auth_events`. Rate-limit token bucket por `user_id` com `sync.Map` + cleanup TTL. Linter custom proíbe handlers lerem identificação fora de `auth.FromContext`.

Status de viabilidade:
viável com restrições

## Necessidade e Objetivos
Problema atual:
Nenhum mecanismo de autenticação ou autorização end-user existe. O middleware `RequireUser` previsto em PRDs de domínio confia em `X-User-ID` cru, o que permitiria a qualquer cliente HTTP se passar por qualquer usuário. Isso viola diretamente o requisito de negócio "usuário é dono das suas informações". Sem este módulo, card/categories/budgets não podem ser implementados em produção.

Objetivos de negócio:
- Garantir isolamento absoluto por usuário em todos os módulos de domínio (zero leak cross-tenant).
- Habilitar implementação de card/categories/budgets removendo o `X-User-ID` transitório.
- Suportar a jornada do MeControla descrita no infográfico (anúncio → checkout → e-mail → WhatsApp → onboarding → uso diário) sem fricção adicional para o usuário final (autenticação implícita no canal WhatsApp).
- Cumprir LGPD em audit e direito à exclusão.

Objetivos técnicos:
- Contrato `auth.Principal` único, derivado exclusivamente de boundary autenticada, consumido por todos os usecases.
- Reutilizar primitivos existentes (HMAC SHA-256 com rotação em `meta_signature.go`, dedup WAMID, raw body buffer, rate-limit base) e centralizá-los em `internal/platform/whatsapp`.
- Audit transacional via `outbox.Publisher` (mesmo padrão de billing/onboarding).
- Manter a arquitetura preparada para JWT Ed25519 + refresh quando app/web chegarem, sem refactor de domínio (contrato `auth.Principal` é estável).
- Aderência total a R0–R7 da skill `go-implementation`.

## Materiais de Apoio
- Bundle de brainstorming aprovado: `docs/discoveries/brainstorms/brainstorm-autenticacao-autorizacao-mvp-whatsapp/`.
- PRDs dependentes: `.specs/prd-card-crud-mvp/prd.md`, `.specs/prd-categories-crud/prd.md`, `.specs/prd-budgets-monthly/prd.md`.
- Código existente reusável: `internal/onboarding/infrastructure/http/server/middleware/meta_signature.go`, `internal/onboarding/infrastructure/http/server/middleware/raw_body_buffer.go`, `internal/onboarding/infrastructure/http/server/middleware/rate_limit.go`, `internal/onboarding/infrastructure/http/server/handlers/whatsapp_inbound_handler.go`, `internal/identity/application/usecases/find_user_by_whatsapp.go`, `internal/identity/domain/valueobjects/` (WhatsAppNumber), `internal/identity/domain/pii/`, `internal/platform/outbox/`.
- ADRs relevantes do onboarding: ADR-009 (deploy VPS Hostinger), ADR-011 (cross-module bind retry, secrets, log shipping).
- `AGENTS.md` Padrão Obrigatório de Módulo + `CLAUDE.md` Outbox.
- Referências `go-implementation` carregadas sob demanda: `architecture.md`, `security.md`, `observability.md`, `examples-domain-flow.md`, `examples-infrastructure.md`, `concurrency.md` (para rate-limit + goroutine de cleanup), `build.md` (checklist R0–R7).

## Escopo
Inclui:
- Pacote `internal/identity/application/auth` com `Principal` (struct mínima por valor: `UserID uuid.UUID`, `Source string`), helpers `WithPrincipal(ctx, Principal)` e `FromContext(ctx) (Principal, bool)`, constantes `SourceWhatsApp` (e futuras `SourceJWT`, `SourceSystem`).
- Middleware `RequireUser` em `internal/identity/infrastructure/http/server/middleware/require_user.go` — retorna 401 imediato sem corpo descritivo (mensagem genérica `{"message":"unauthorized"}`) quando Principal ausente.
- Pacote compartilhado `internal/platform/whatsapp` com: validador HMAC (movido do onboarding, mantendo current+next), raw body buffer, dedup WAMID (porta + adapter), parse de payload Meta, dispatcher único que roteia mensagens entre handlers de onboarding (não-ativado) e agent (ativado, com Principal resolvido). Mover gradualmente o que existe em onboarding.
- Resolver `wa_id → user_id` no dispatcher via `Identity.FindUserByWhatsApp` (sem cache no MVP).
- Tabela `auth_events` (migration `00XX_create_identity_auth_events`) com PK UUID conforme padrão da tabela `users`. Schema compacto.
- Projector `internal/identity/infrastructure/messaging/database/consumers/auth_events_consumer.go` que consome eventos `auth.principal_established`, `auth.failed`, `auth.unknown_user` do outbox e insere em `auth_events`.
- Eventos publicados na mesma transação do upsert/lookup via `outbox.Publisher` (idempotência por `event_id`).
- Rate-limit token bucket por `user_id` em `internal/platform/whatsapp/ratelimit` com `sync.Map[uuid.UUID]*bucket` + goroutine cleanup TTL 5min.
- Linter custom (depguard) bloqueando `r.Header.Get("X-User-ID")` em `internal/*/infrastructure/http/server/handlers/**` + linter forbidigo proibindo `os.Getenv` para secrets de auth.
- Métricas OTel: `auth_principal_established_total{source}`, `auth_failed_total{reason}`, `auth_unknown_wa_id_total`, `auth_rate_limit_hits_total`, `auth_resolve_wa_duration_seconds` (histograma), `meta_signature_status_total{status}` (já existe em onboarding — mover).
- Spans OTel: `auth.resolve_principal`, `auth.require_user`, `whatsapp.dispatcher.route`.
- ADR `docs/adrs/auth/adr-001-principal-in-process-and-future-http-boundary.md`.
- Runbook `docs/runbooks/auth-meta-secret-rotation.md` documentando rotação do `META_APP_SECRET` com janela `CURRENT+NEXT`.
- Atualização dos PRDs `prd-card-crud-mvp`, `prd-categories-crud`, `prd-budgets-monthly` removendo o `X-User-ID` transitório (será feita pelo PRD de auth, não por este discovery).

Exclui:
- JWT Ed25519, JWKs, kid rotation, refresh token (PRD futuro para app/web).
- Tabela `auth_sessions` persistida.
- Provedores OIDC externos.
- mTLS interno.
- RBAC e papéis (todos os usuários têm o mesmo papel hoje).
- Login web ou fluxos UI de autenticação direta.
- Rate-limit global por IP (foco é por `user_id`).
- Revogação ativa por deny-list (foi avaliada e rejeitada para o MVP).
- Cache `wa_id → user_id` (rejeitado para o MVP; gatilho documentado para reativar se p99 > 100ms por 3 dias consecutivos).
- Replay protection por timestamp Meta (não há header oficial Meta para isso; reavaliar se Meta publicar).

## Premissas e Restrições
Premissas:
- LLM + WhatsApp bridge rodam in-process com o backend Go (H1 do bundle de brainstorming).
- Volumetria-alvo do MVP: 500–5000 usuários ativos, pico ~500 msg/min, SLO 99.5% mensal, p99 webhook < 300ms, p99 `resolve_principal` < 100ms.
- `internal/identity` continuará sendo a fonte de verdade da identidade (`FindUserByWhatsApp`, `UpsertUserByWhatsApp`).
- Migrações Postgres seguem padrão `<seq>_<descricao>.up.sql/.down.sql` em `migrations/`.
- Padrão de PK das tabelas de domínio: `id UUID NOT NULL` + `CONSTRAINT <table>_pkey PRIMARY KEY (id)` + `created_at TIMESTAMPTZ DEFAULT now()`.
- `outbox.Publisher` é canal único para eventos com garantia transacional.
- Sem cluster horizontal no MVP (uma instância VPS); rate-limit in-memory é aceitável.
- LLM tools são funções Go nativas no orquestrador `internal/agent` (a ser criado em PRD próprio); recebem `ctx` com Principal e derivam `user_id` exclusivamente de `auth.FromContext`.

Restrições:
- Deploy permanece em VPS Hostinger; sem KMS gerenciado.
- Postgres é o único storage; sem Redis/Memcached/Vault.
- Zero dependência externa nova (sem Auth0/Clerk/Keycloak/Supabase/Vault). Permitido apenas o que já está em `go.mod`.
- R0 (sem `init()`), R5.12 (sem `panic` em produção), R6 (`context.Context` em fronteiras de IO; interface no consumidor), R7.6 (`errors.Join` + `%w`), goroutines canceláveis com shutdown cooperativo.
- LGPD: PII mascarada (R7), audit log ≥ 90 dias, revogação de Principal cessada ao marcar usuário deletado, retenção legal preservada anonimizada.
- Toda métrica OTel com cardinalidade controlada: `user_id` NUNCA pode ser label.

## Viabilidade Técnica
Status:
viável com restrições

Justificativa:
A maior parte da infraestrutura crítica já existe e é reutilizável: HMAC SHA-256 com rotação (`meta_signature.go`), dedup WAMID, raw body buffer, parse Meta, outbox transacional, observabilidade OTel, repositório `FindUserByWhatsApp`. O esforço se concentra em (1) extrair os primitivos para `internal/platform/whatsapp` sem regredir o comportamento existente do onboarding, (2) criar o pacote `auth` enxuto, (3) criar o projector + tabela `auth_events`, (4) implementar rate-limit por `user_id` com goroutine de cleanup cancelável, (5) implementar linter custom. As restrições derivam principalmente do contexto de VPS sem KMS e da decisão de operar sem cache no MVP, ambos com gatilhos de revisão claros.

Bloqueadores:
- Nenhum bloqueador técnico identificado.

## Arquitetura Atual
- `internal/onboarding/infrastructure/http/server` possui webhook `WhatsAppInboundHandler` que valida assinatura Meta (via middleware `MetaSignature`), faz dedup por WAMID, extrai mensagem, valida número via VO `identityvo.NewWhatsAppNumber` e dispatcha entre comando `ATIVAR <token>` e fallback de outreach. Não resolve `wa_id → user_id` porque é fluxo pré-ativação.
- `internal/identity/application/usecases/find_user_by_whatsapp.go` provê o lookup; `UpsertUserByWhatsApp` cria/atualiza; `MarkUserDeleted` faz soft-delete (status='DELETED').
- `internal/platform/outbox/` provê `Publisher`, `Dispatcher`, `Envelope`, idempotência por `event_id`, housekeeping, dead-letter.
- Não existe pacote `auth`. Não existe middleware `RequireUser` em produção. Não existe orquestrador LLM (`internal/agent`).
- Card/categories/budgets só existem como PRDs com a nota explícita do middleware transitório `X-User-ID`.

## Arquitetura Proposta
Componentes:
- `internal/identity/application/auth/principal.go` — tipo `Principal` (struct por valor `{UserID uuid.UUID; Source PrincipalSource}`), `PrincipalSource` (tipo string com constantes `SourceWhatsApp`, `SourceSystem`; reservar `SourceJWT` em comentário para futuro), helpers `WithPrincipal(ctx, p) context.Context`, `FromContext(ctx) (Principal, bool)`. Chave de contexto é tipo privado não exportado.
- `internal/identity/infrastructure/http/server/middleware/require_user.go` — `RequireUser(next http.Handler) http.Handler` que retorna 401 com `{"message":"unauthorized"}` (Content-Type application/json) quando Principal ausente. Não vaza motivo. Não loga `user_id` (mantém middleware enxuto; logging é responsabilidade do handler com `slog.With("user_id", ...)` quando útil).
- `internal/identity/application/usecases/establish_principal.go` — usecase `EstablishPrincipal(ctx, WhatsAppNumber) (Principal, error)`. Compõe `FindUserByWhatsApp`, publica `auth.principal_established` (ou `auth.unknown_user`) via `outbox.Publisher` na mesma transação. Retorna `Principal` ou `ErrUnknownUser` (typed error em `internal/identity/application/errors.go`).
- `internal/platform/whatsapp/` — novo pacote. Subpastas:
  - `signature/` (movido de `onboarding/.../middleware/meta_signature.go` + raw body buffer): exporta `HMACMiddleware(secretCurrent, secretNext, metrics)` e `SignatureStatusFromContext`.
  - `payload/` — parser do envelope Meta + `ExtractFirstMessage(payload) (Message, bool)`.
  - `dedup/` — porta `MessageRepository.InsertIfAbsent(ctx, wamid)` + métrica `meta_duplicate_messages_total`. Implementação concreta segue no módulo consumidor.
  - `dispatcher/` — `Dispatcher.Route(ctx, msg) RouteOutcome` aplica regex de `ATIVAR <token>` e roteia: (a) ativação → handler onboarding; (b) usuário existe e está ativo → `EstablishPrincipal` + handler agent (futuro `internal/agent`); (c) usuário não existe → handler fallback do onboarding.
  - `ratelimit/` — token bucket por `uuid.UUID` com `sync.Map[uuid.UUID]*bucket` + goroutine cleanup. API: `Limiter.Allow(userID) bool` + `Limiter.Shutdown(ctx) error`. Bucket: 60 tokens, refill 1 token/seg, burst 60. Cleanup remove buckets inativos > 5min a cada 60s. Goroutine cancelável por `ctx`.
- `internal/identity/infrastructure/messaging/database/consumers/auth_events_consumer.go` — consumidor outbox que projeta eventos `auth.*` para a tabela `auth_events`. Idempotência por `event_id` (já garantida pelo padrão do outbox).
- `internal/identity/infrastructure/repositories/postgres/auth_events_repository.go` — `Insert(ctx, AuthEvent) error`. Estrutura mínima.
- Migration `00XX_create_identity_auth_events.up.sql` + `.down.sql`.
- Linter custom: `tools/lint/forbid_x_user_id.go` (ou regra em `.golangci.yml` via `depguard`) bloqueando `r.Header.Get("X-User-ID")` em handlers fora do diretório `internal/platform/auth-test/` se houver fixtures.

Fluxo de alto nível:
1. Meta envia POST `/whatsapp/inbound` ao backend.
2. Chain de middleware: `raw_body_buffer` → `whatsapp.signature.HMACMiddleware` (rejeita 401 se inválido; injeta `meta_signature_status_total{status}` em métrica). Se inválido, publica `auth.failed` (reason='invalid_signature') via outbox antes do 401.
3. `whatsapp.dispatcher.Route` faz parse, dedup por WAMID, valida `WhatsAppNumber`. Se inválido, responde 200 OK (Meta requer) e registra `auth.failed` (reason='invalid_country').
4. Dispatcher chama `EstablishPrincipal(ctx, waNumber)`:
   - `FindUserByWhatsApp` retorna usuário ativo → publica `auth.principal_established{user_id, source=whatsapp}` via outbox na mesma TX do dispatcher.
   - Retorna não-encontrado/deletado → publica `auth.unknown_user`. Dispatcher roteia para handler de fallback do onboarding (sem Principal).
5. Se Principal estabelecido, dispatcher chama `ratelimit.Allow(user_id)`. Se rejeita → publica `auth.failed{reason='rate_limited'}` + responde 200 OK (evita Meta retry storm) + opcionalmente envia template "limite atingido".
6. Se Allow=true, `dispatcher` injeta Principal no `ctx` via `auth.WithPrincipal(ctx, p)` e chama handler do agent (LLM dispatcher). Tools recebem o `ctx` e leem `user_id` via `auth.FromContext` exclusivamente.
7. Para chamadas HTTP de domínio (futuro app/web), o middleware `RequireUser` valida que `Principal` está em ctx (preenchido por boundary futura conforme ADR-001). Sem Principal → 401.
8. Consumer `auth_events_consumer` lê outbox e projeta para `auth_events`.

Decisão arquitetural:
Variante D' do bundle de brainstorming: `auth.Principal` in-process via `context.Context` + ADR documentando boundary HTTP futura sem implementá-la agora. Domínio jamais conhece primitivas de transporte. Centralização do WhatsApp em `internal/platform/whatsapp` é refactor incremental para evitar duplicação quando o agent existir; mantém o handler atual do onboarding funcional durante a migração via Strangler Fig (criar pacote novo, mover middlewares e parse, migrar onboarding para consumir o novo pacote, depois adicionar agent).

## Dados e Integrações
Domínios de dados:
- `users` (existente em `internal/identity`) — fonte de verdade para `user_id` e `wa_id`.
- `auth_events` (nova) — audit log projetado do outbox.
- `channel_processed_messages` — dedup canônico por canal/mensagem, compartilhado via porta em `internal/platform/whatsapp/dedup`.
- `platform_outbox_events` (existente) — canal único de eventos.

Integrações:
- Meta WhatsApp Cloud API — inbound via webhook (`/whatsapp/inbound`), outbound via `internal/onboarding/infrastructure/http/client/meta` (não-impactado por este módulo).
- Futuro `internal/agent` — consome `auth.Principal` via `ctx`; LLM tools chamam usecases de card/categories/budgets diretamente com `ctx`.
- Futura boundary HTTP para app/web (PRD próprio) — implementa interface conceitual `PrincipalIssuer` (não materializada em código no MVP) e popula `auth.Principal` antes de chegar ao handler.

Consistência requerida:
forte para auth_events vs comando que disparou o evento (mesma transação do outbox); eventual para projeção (consumer lê do outbox); forte para `users` vs `Principal` resolvido (lookup direto no `find_user_by_whatsapp`, sem cache).

## Volumetria e Capacidade
Volume atual:
Nenhum tráfego de webhook de agente hoje. Onboarding está em fluxo de pré-ativação ainda.

Pico esperado:
- ~500 msg/min total no MVP (média de 1 msg/segundo, pico bursts ~10 msg/seg).
- ~500–5000 usuários ativos.
- `auth_events`: ~7.000 inserts/dia em estado estacionário (principal_established por msg + ~10% de failed/unknown), ~210k linhas/mês.
- Token buckets em memória: até 5000 entradas simultâneas, ~64 bytes cada = ~320KB.
- Lookup `FindUserByWhatsApp`: 1 SELECT por msg = ~500 RPM PG, p99 ~5ms em VPS local.

Taxa de crescimento:
Linear com adoção do produto. Reavaliar quando ultrapassar 5000 usuários ativos OU 5000 msg/min OU p99 webhook > 300ms.

SLO alvo:
- Disponibilidade mensal: 99.5% por endpoint `/whatsapp/inbound` (≈3h36min de error budget).
- p99 `auth.resolve_principal` < 100ms.
- p99 webhook end-to-end < 300ms.
- Erro de assinatura inválida: < 0.1% das requisições (alerta se > 1%).

Gargalos conhecidos:
- Lookup PG sem cache: se PG cair, webhook cai. Mitigação: PG é single-point-of-failure conhecido do MVP (ADR-009 onboarding).
- Rate-limit in-memory: não compartilhado entre instâncias; bloqueante para escala horizontal. Mitigação: documentar dívida, migrar para PG sliding window quando necessário.
- `sync.Map` cleanup: se goroutine de cleanup travar (bug), buckets crescem sem teto. Mitigação: cleanup é simples (iterar + delete), com teste de carga.

## Segurança e Compliance
Classificação dos dados:
- `wa_id` (E.164): PII (LGPD art. 5º II).
- `user_id` (UUID): identificador interno não-PII.
- `auth_events.user_id`: PII por proxy (pode ser correlacionado).
- `META_APP_SECRET`, `META_APP_SECRET_NEXT`: secret crítico.

Autenticação e autorização:
- AuthN: HMAC SHA-256 do webhook Meta (`X-Hub-Signature-256`) com suporte a `current+next` (rotação sem downtime). Assinatura inválida ou ausente → 401 imediato + `auth.failed`.
- AuthZ: `Principal` em `ctx` é a única fonte de `user_id` consumida por usecases. Linter custom + revisão de código garantem isso. Toda query de domínio incluirá `WHERE user_id = $1` derivado de Principal.
- Middleware `RequireUser` valida presença de Principal; sem fallback. Resposta genérica não-descritiva (evita oracle).
- Tools LLM nunca recebem `user_id` via prompt ou args; sempre via `ctx`. Confirmado como anti-pattern explicitamente rejeitado.

Gestão de segredos:
- `META_APP_SECRET` e `META_APP_SECRET_NEXT` carregados de env file via `internal/platform/config`. Nunca logados, nunca em mensagens de erro, nunca em responses.
- Rotação documentada em runbook (sem KMS): (1) gerar novo secret no Meta App Dashboard, (2) deploy com `META_APP_SECRET_NEXT` preenchido, (3) trocar secret no Meta, (4) deploy com `META_APP_SECRET=NEXT` e `NEXT=""`. Métrica `meta_signature_status_total{status='rotated'}` indica progresso.
- Linter forbidigo proibindo `os.Getenv` direto para secrets de auth — deve passar por config carregado com validação.

Criptografia:
- HMAC SHA-256 via `crypto/hmac` + `crypto/sha256` (stdlib).
- Comparação constant-time via `hmac.Equal` (já usado no código existente).
- Sem criptografia de tokens persistidos (sem tokens persistidos no MVP).

Auditoria e rastreabilidade:
- Tabela `auth_events` com retenção mínima 90 dias.
- Eventos: `principal_established`, `failed`, `unknown_user`.
- `failed` carrega motivo: `invalid_signature`, `unknown_wa_id`, `invalid_country`, `rate_limited`.
- Spans OTel `auth.resolve_principal`, `auth.require_user`, `whatsapp.dispatcher.route` propagam `trace_id` em logs estruturados para correlação.

Compliance/LGPD:
- PII mascarada em logs: VO `WhatsAppNumber.Masked()` (já existe em identity) é a única forma de logar `wa_id`. Linter forbidigo bloqueia `slog` com `wa_id` cru.
- Direito à exclusão (art. 18): `MarkUserDeleted` publica evento outbox `user.deleted` que o `auth_events_consumer` consome para anonimizar `user_id` em `auth_events` (substituir por `NULL` mantendo retenção legal de `occurred_at`/`kind`/`source`).
- Retenção: 90 dias rolling em `auth_events`. Housekeeping job mensal arquiva ou deleta. Decisão de teto final em PRD.

## Confiabilidade e Resiliência
SLA/SLO:
- Disponibilidade `/whatsapp/inbound`: 99.5% mensal.
- p99 latency: webhook < 300ms; resolve_principal < 100ms.

RTO/RPO:
- RTO 1h (restauração de processo Go + dependência PG via runbook existente).
- RPO 0 para `auth_events` (gravado dentro da TX do outbox; durável após commit).

Estratégia de retry/idempotência:
- Meta retenta webhook em falha (5xx); resposta 200 OK em mensagens válidas mesmo quando dispatcher decide não processar (ex.: número inválido) evita retry storm.
- `auth_events_consumer` é idempotente por `event_id` (regra obrigatória do outbox, conforme CLAUDE.md).
- Dedup WAMID já trata replays de webhook.

Degradação/contingência:
- PG indisponível: webhook responde 503 ao Meta (gera retry). Métrica `auth_resolve_failed_total{reason='db_unavailable'}` indica saturação. Sem cache, não há degradação parcial — é binário.
- Outbox indisponível: ainda assim retorna 200 OK ao Meta para evitar retry storm; perde o audit do evento (alerta crítico em `outbox_publish_failed_total > 0`).
- Rate-limit excedido: 200 OK ao Meta + mensagem opcional ao usuário "limite atingido", sem chamar agent.

Rollback:
- Migration down preserva `auth_events` via rename (`ALTER TABLE auth_events RENAME TO auth_events_archived_<ts>`), nunca DROP.
- Sem feature flag: rollout é deploy completo. Rollback é deploy do binário anterior + reverter migration via `migrate down`.
- Como nada existe em produção hoje (card/categories/budgets ainda não foram implementados), não há fluxo de runtime para regredir.

## Observabilidade e Operação
Métricas:
- `auth_principal_established_total{source}` (counter; label `source` ∈ {whatsapp,system}).
- `auth_failed_total{reason}` (counter; label `reason` ∈ {invalid_signature, unknown_wa_id, invalid_country, rate_limited, db_unavailable}; cardinalidade controlada).
- `auth_unknown_wa_id_total` (counter).
- `auth_rate_limit_hits_total` (counter — sem label `user_id` para evitar explosão de cardinalidade).
- `auth_resolve_wa_duration_seconds` (histograma, buckets [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0]).
- `meta_signature_status_total{status}` (counter; label `status` ∈ {valid, invalid, rotated}; já existe em onboarding, será reusada).
- `whatsapp_dispatcher_route_total{outcome}` (counter; label `outcome` ∈ {onboarding, agent, fallback, rejected}).
- `whatsapp_ratelimit_buckets_count` (gauge; expõe número de buckets vivos em memória).
- `whatsapp_ratelimit_cleanup_duration_seconds` (histograma).

Logs:
- `slog` estruturado JSON com campos: `level`, `msg`, `trace_id`, `span_id`, `user_id` (apenas quando Principal estabelecido), `source`, `correlation_id`, `wa_id_masked` (via VO).
- Nunca logar: `wa_id` cru, `META_APP_SECRET`, headers `Authorization`, payload Meta integral em INFO (apenas em DEBUG controlado).
- Eventos chave: `auth.principal_established`, `auth.failed{reason}`, `auth.unknown_user`, `whatsapp.dispatcher.routed{outcome}`, `whatsapp.ratelimit.blocked`.

Traces:
- Span `auth.resolve_principal` com atributos `source`, `user_id` (somente após sucesso), `outcome` ∈ {found, unknown, error}.
- Span `auth.require_user` com atributo `result` ∈ {pass, unauthorized}.
- Span `whatsapp.dispatcher.route` com atributos `outcome`, `is_activation`, `is_dedup`.
- Sem `wa_id` em nenhum atributo de span.

Alertas:
- `auth_failed_total{reason='invalid_signature'} > 0 in 5min` → ataque ou rotação mal configurada.
- `auth_failed_total{reason='db_unavailable'} > 1 in 1min` → saturação PG.
- `whatsapp_ratelimit_buckets_count > 10000` → leak de bucket.
- `auth_resolve_wa_duration_seconds{quantile='p99'} > 0.1 sustained 3d` → gatilho de ativação de cache LRU.
- `outbox_publish_failed_total{kind=~"auth\\..*"} > 0` → audit perdido (crítico).

Dashboards/Runbooks:
- Dashboard Grafana "Auth Module": linhas com cada métrica acima + heatmap de `auth_resolve_wa_duration_seconds`.
- Runbook `docs/runbooks/auth-meta-secret-rotation.md`.
- Runbook `docs/runbooks/auth-incident-response.md` (resposta a alerta de `invalid_signature` ou `db_unavailable`).

## Performance e Escalabilidade
Latência alvo:
- p50 webhook end-to-end: 50ms.
- p99 webhook end-to-end: 300ms.
- p99 `auth.resolve_principal`: 100ms.
- p99 `RequireUser` (overhead puro): 1ms.

Estratégia de escala:
Vertical no MVP (VPS Hostinger). Horizontal exige (1) migrar rate-limit para PG sliding window ou Redis, (2) read replica PG para FindUserByWhatsApp, (3) cache distribuído. Cada um é PR separado com ADR próprio.

Limites conhecidos:
- 1 instância VPS.
- Sem read replica.
- Sem cache distribuído.

Teste de carga:
- Microbenchmark Go (`testing.B`) em `auth.WithPrincipal`/`FromContext` (alvo: < 50ns/op).
- Microbenchmark em rate-limit (`Allow` < 200ns/op para 5000 buckets).
- Load test k6 ou vegeta no webhook simulando 500 msg/min sustained + bursts de 100 msg/seg. Critério: p99 < 300ms.
- Property-based test no HMAC validator (já existe regra em meta_signature_test.go) confirmando: nenhum input randomico aprovado sem secret correto.

## Custos e Orçamento
Orçamento estimado:
baixo

Drivers de custo:
- Espaço em disco PG para `auth_events`: ~210k linhas/mês × ~100 bytes médio = ~21MB/mês. Negligível.
- CPU/RAM extra na VPS: ~5MB heap para rate-limit + 1 goroutine de cleanup. Negligível.
- Dev hours estimados: discovery → 1d; implementação → 5d; testes + revisão → 3d; ADR + runbook + docs → 1d. Total ~10 dias-dev.

Guardrails de custo:
- Sem dependência externa nova (zero custo recorrente).
- Sem managed services.
- Sem incremento de licença.

Plano de otimização:
- Reavaliar cache LRU se p99 estourar (custo: ~1 dia-dev + lib hashicorp/golang-lru já amplamente auditada).
- Reavaliar retenção `auth_events` a cada trimestre.

## Riscos e Mitigações
- Risco: extrair `meta_signature.go` para `internal/platform/whatsapp` quebra onboarding existente.
  Impacto: alto (regressão no fluxo de ativação em produção).
  Mitigação: refactor incremental (Strangler Fig): criar pacote novo, manter o antigo, migrar onboarding para consumir o novo, depois remover. Testes de regressão obrigatórios em `meta_signature_test.go` (já existem).
  Dono: Engenharia.

- Risco: cross-tenant leak por bug de query sem `WHERE user_id`.
  Impacto: alto (violação de premissa de negócio + LGPD).
  Mitigação: linter custom proibindo handler ler header de identificação; convenção obrigatória de que todo repository de domínio receba `user_id` como primeiro argumento; testes de isolamento por usecase (criar 2 usuários, validar que A não vê B).
  Dono: Arquitetura.

- Risco: prompt injection via LLM convence tool a usar `user_id` alheio.
  Impacto: alto (replay/hijack — vetor explícito do brainstorm).
  Mitigação: tools recebem `ctx` com Principal e leem `user_id` exclusivamente de `auth.FromContext`; args do LLM são DTOs de domínio puros sem campo `user_id`. Teste de regressão com prompt adversarial em `internal/agent` (PRD futuro).
  Dono: Arquitetura.

- Risco: goroutine de cleanup do rate-limit trava ou vaza buckets.
  Impacto: médio (RAM cresce sem teto; eventual OOM da VPS).
  Mitigação: goroutine simples (iterar + delete), com `ctx` cancelável, métrica `whatsapp_ratelimit_buckets_count` com alerta em 10k; shutdown cooperativo no graceful-lifecycle do app.
  Dono: Engenharia.

- Risco: sem cache no MVP, pico de mensagens satura PG.
  Impacto: médio (degradação de webhook).
  Mitigação: gatilho de revisão em métrica p99 > 100ms por 3 dias consecutivos; PR de cache LRU já desenhado na techspec.
  Dono: Engenharia/Operações.

- Risco: rotação do `META_APP_SECRET` aplicada incorretamente causa 401 em massa.
  Impacto: alto (downtime do canal WhatsApp).
  Mitigação: runbook detalhado com `CURRENT+NEXT`; ensaio em homologação obrigatório; métrica `meta_signature_status_total{status='rotated'}` indica progresso.
  Dono: Operações.

- Risco: `outbox.Publisher` indisponível faz perder audit event.
  Impacto: alto (compliance LGPD).
  Mitigação: outbox é parte da mesma TX do upsert/lookup; falha de outbox = falha do request (5xx ao Meta → retry). Alerta `outbox_publish_failed_total > 0`.
  Dono: Engenharia.

- Risco: linter custom não pega bypass criativo (ex.: `r.Header.Values("X-User-ID")`).
  Impacto: médio (bypass silencioso).
  Mitigação: regra depguard mais ampla (proibir todo acesso a header começando com `X-User-`); testes de PR que falham se handler consome `Header` para identidade; revisão obrigatória.
  Dono: Engenharia.

## Trade-offs e Decisões
Alternativas consideradas:
- A — Principal in-process via ctx (sem ADR).
- B — JWT Ed25519 curto + JWKs no MVP.
- C — Sessão opaca persistida em Postgres.
- D — Boundary-explicit (ctx + interface JWT documentada).
- D' — Variante minimalista de D: apenas `auth.Principal` + ADR (escolhida).

Decisão tomada:
D' — `auth.Principal` in-process via `context.Context` em `internal/identity/application/auth`, com ADR-001 documentando o contrato de boundary HTTP futura para app/web (JWT Ed25519 + refresh, fora do escopo do MVP). Webhook único centralizado em `internal/platform/whatsapp`. Audit via outbox. Rate-limit por `user_id` in-memory. Sem cache `wa_id → user_id`.

Trade-off aceito:
- Refactor extra (mover meta_signature e parse para `internal/platform/whatsapp`) em troca de eliminar duplicação quando o agent existir.
- Sem cache no MVP (p99 risco em pico) em troca de menos código e gatilho de revisão automatizado.
- Sem JWT no MVP (refactor potencial de boundary HTTP no futuro) em troca de zero gestão de chaves em VPS sem KMS.
- Sem revogação ativa (janela de exposição = TTL implícito de re-resolução) em troca de simplicidade.
- Rate-limit per-user in-memory (não escala horizontal) em troca de zero infra nova.

## Plano de Entrega e Rollout
Fases:
- Fase 1 — Fundação: pacote `auth`, `Principal`, helpers, ADR-001, testes unitários (microbenchmark + table-driven).
- Fase 2 — Persistência: migration `auth_events`, repository, `EstablishPrincipal` usecase, eventos outbox, consumer/projector, testes de integração com PG real.
- Fase 3 — Platform WhatsApp: extrair `meta_signature`, `raw_body_buffer`, parse, dedup para `internal/platform/whatsapp`; manter onboarding funcional (Strangler Fig); testes de regressão em `meta_signature_test.go`.
- Fase 4 — Rate-limit + dispatcher: `internal/platform/whatsapp/ratelimit` e `dispatcher`; testes de concorrência (race detector obrigatório).
- Fase 5 — Middleware + linter: `RequireUser` + regra depguard/forbidigo; PRs em card/categories/budgets removendo `X-User-ID` transitório passam a depender disso.
- Fase 6 — Operação: runbook rotação, dashboard Grafana, alertas, load test k6.

Migração:
- Card/categories/budgets ainda não estão em produção. Não há migração de tráfego real.
- Webhook onboarding existente continua funcional durante refactor (Strangler Fig).

Feature flags/canary:
Nenhuma feature flag. Nenhum canary. Deploy completo após validação em homologação. Justificativa: nada em produção depende ainda; rollout é greenfield.

Critério de rollback:
- Falha em smoke test pós-deploy (script `task auth:smoke` que envia webhook válido e confirma Principal + linha em `auth_events`).
- Alerta `auth_failed_total{reason='invalid_signature'} > 10/min` por 5 minutos.
- Alerta `outbox_publish_failed_total{kind=~"auth\\..*"} > 0`.

## Decomposição em Épicos e Features
### Epic 01 - Pacote auth e contrato Principal
Objetivo: Estabelecer o contrato `auth.Principal` consumido por todo o domínio.
Feature 01: Tipo `Principal` + helpers `WithPrincipal`/`FromContext` + testes
Feature 02: ADR-001 documentando contrato e boundary HTTP futura
Feature 03: Middleware `RequireUser` + testes table-driven
Feature 04: Linter custom (depguard/forbidigo) + integração com CI

### Epic 02 - Persistência e audit
Objetivo: Persistir eventos de auth via outbox para compliance LGPD.
Feature 01: Migration `auth_events` (up/down preservando dados)
Feature 02: Repository Postgres `auth_events`
Feature 03: Usecase `EstablishPrincipal` + eventos outbox
Feature 04: Consumer/projector `auth_events_consumer`
Feature 05: Anonimização em `MarkUserDeleted` (consumir `user.deleted`)

### Epic 03 - Platform WhatsApp (extração)
Objetivo: Centralizar HMAC, parse, dedup, raw body em pacote compartilhado.
Feature 01: Extrair `meta_signature` para `internal/platform/whatsapp/signature`
Feature 02: Extrair `raw_body_buffer` para `internal/platform/whatsapp/signature`
Feature 03: Extrair parse e tipos Meta para `internal/platform/whatsapp/payload`
Feature 04: Porta de dedup WAMID em `internal/platform/whatsapp/dedup` + adapter Postgres
Feature 05: Migrar onboarding para consumir os novos pacotes (Strangler Fig)

### Epic 04 - Dispatcher e rate-limit
Objetivo: Roteamento único do webhook + proteção contra abuso por user.
Feature 01: Token bucket `Limiter` com `sync.Map` + cleanup goroutine + shutdown cooperativo
Feature 02: `Dispatcher.Route` aplicando regex ATIVAR + resolver Principal + rate-limit
Feature 03: Testes de concorrência com race detector
Feature 04: Microbenchmark `Limiter.Allow` e `auth.WithPrincipal`/`FromContext`

### Epic 05 - Observabilidade e operação
Objetivo: Dashboards, alertas e runbooks operacionais.
Feature 01: Métricas OTel + spans listados no dossiê
Feature 02: Dashboard Grafana "Auth Module"
Feature 03: Alertas `auth_failed_*`, `ratelimit_buckets`, `outbox_publish_failed_*`
Feature 04: Runbook `auth-meta-secret-rotation.md`
Feature 05: Runbook `auth-incident-response.md`
Feature 06: Load test k6 com critério p99 < 300ms

## Itens em Aberto
- Definir teto e estratégia de arquivamento de `auth_events` além da retenção mínima de 90 dias (decidir em PRD).
- Confirmar versão final do nome do diretório `internal/platform/whatsapp` vs `internal/platform/messaging/whatsapp` (avaliar quando outros canais — Telegram, SMS — entrarem no roadmap).
- Validar empiricamente H7 (60 msg/min/user é o teto adequado) após 2 semanas de uso real.
- Decidir se `MarkUserDeleted` anonimiza `auth_events.user_id` para NULL ou para hash com sal rotativo (escolha de privacy engineering, fora deste discovery).
- Reavaliar premissa H10 (LLM in-process) sempre que houver proposta de mover orquestrador LLM para fora do processo Go.
- Avaliar se `internal/agent` é nome final ou se vira `internal/copilot`/`internal/assistant` quando o PRD do orquestrador for criado.
