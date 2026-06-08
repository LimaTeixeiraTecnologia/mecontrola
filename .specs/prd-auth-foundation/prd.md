# Documento de Requisitos do Produto (PRD)

<!-- spec-version: 4 -->

<!--
Histórico de versões:
- v1 (2026-06-08): escopo MVP production-ready da fundação de autenticação e autorização do MeControla (`internal/identity/application/auth` + `internal/platform/whatsapp`). Insumos: brainstorming decisório `docs/discoveries/brainstorms/brainstorm-autenticacao-autorizacao-mvp-whatsapp/` e discovery técnico `docs/discoveries/technical-auth-foundation-mvp-whatsapp-llm/`. Substitui o middleware `RequireUser` transitório por header `X-User-ID` declarado em `.specs/prd-card-crud-mvp/prd.md` e referenciado em `.specs/prd-categories-crud/prd.md` e `.specs/prd-budgets-monthly/prd.md`.
- v2 (2026-06-08): eliminação total de suposições e questões em aberto via 7 decisões travadas: (S-01) LLM in-process no mesmo binário Go; (S-02) `internal/platform/whatsapp` como path final; (S-03) `internal/agent` como nome do orquestrador LLM; (S-04) rate-limit fixo 60 msg/min + burst 60, sem env var override; (Q-01) retenção `auth_events` 180 dias + housekeeping mensal; (Q-02) anonimização via `UPDATE user_id = NULL` simples e irreversível; (Q-03) numeração da migration alocada pela techspec a partir do próximo livre em `migrations/`, validada por CI. Novos RFs adicionados (RF-22 a RF-24).
- v3 (2026-06-08): eliminação de 12 lacunas implícitas em RFs e schema, descobertas por revisão adversarial. Decisões travadas: (A1) coluna `reason TEXT NULL` com CHECK enumerado em `auth_events`; (A2) CHECK de `source` restrito a `'whatsapp'` no MVP; (A3) paths HTTP `/api/v1/whatsapp/inbound` (POST) e `/api/v1/whatsapp/verify` (GET); (A4) `trace_id` OTel como único identificador de correlação — sem `correlation_id` separado, sem coluna em `auth_events`; (B1) Unit of Work do `identity` envolve `FindUserByWhatsApp` + outbox publish na mesma TX curta; (B2) depguard bloqueia toda `r.Header.Get`/`.Values` em handlers com allowlist explícita (`X-Request-ID`, `Content-Type`, `Idempotency-Key`); (B3) deleção do código antigo de `meta_signature.go` e `raw_body_buffer.go` no MESMO PR que migra onboarding para o novo pacote; (B4) só `SourceWhatsApp` declarada como constante Go no MVP — `SourceJWT`/`SourceSystem` ficam documentados em ADR-001, não em código; (C1) load test k6/vegeta como acceptance criteria do épico; (C2) `task auth:smoke` automatizado bloqueia release se falhar; (C3) atualização cross-PRD obrigatória em `prd-onboarding-magic-token` por causa da migração do middleware; (C4) `Limiter` implementa `Start(ctx)`/`Shutdown(ctx)` com wiring via `module.go` no padrão lifecycle. RFs ajustados (RF-09, RF-12, RF-19) + RFs novos (RF-25 a RF-32). Esta é a versão definitiva para handoff à techspec.
- v4 (2026-06-08): eliminação de 4 lacunas adicionais descobertas em revisão adversarial da techspec. Decisões travadas: (A2-bis) `invalid_payload` adicionado ao CHECK de `reason` para cobrir JSON malformado pós-HMAC; (B1-bis) `MarkUserDeleted` MUST publicar evento outbox `user.deleted` com payload `{event_id, user_id, deleted_at}`, este PRD verifica e adiciona se ausente; (B4-bis) dispatcher roteia mensagens não-ATIVAR de usuário ativo para um **stub do agent** que responde via template Meta pré-aprovado "MeControla recebida — estamos cuidando da sua mensagem" enquanto `internal/agent` não existe; (C2-bis) migration de seed para staging cria usuário fixo de smoke test com WA configurável via `STAGING_SMOKE_WA`. RFs novos (RF-33 a RF-36) + RF-09 ajustado para incluir `invalid_payload`. Esta é a versão definitiva para handoff à v2 da techspec.
-->

## Visão Geral

A fundação de autenticação e autorização do MeControla resolve o vácuo de segurança que hoje obriga os PRDs de domínio (`card`, `categories`, `budgets`) a declarar um middleware `RequireUser` **transitório** que confia em header `X-User-ID` cru. Esse vácuo viola diretamente a premissa de negócio inegociável "cada usuário é dono exclusivo das próprias informações financeiras" e impede a entrega dos módulos de domínio em produção.

Esta funcionalidade entrega o contrato `auth.Principal` em `context.Context` como **única fonte de identidade** consumida pelos casos de uso, junto com a infraestrutura mínima para o canal inicial (WhatsApp + LLM in-process): validação de assinatura Meta com rotação, dispatcher único de mensagens, resolução `wa_id → user_id`, audit log transacional via outbox, rate-limit por usuário e linter custom que proíbe handlers consumirem identidade fora do contrato.

O escopo é **MVP cirúrgico em camadas**: o canal WhatsApp recebe implementação completa; app móvel e web ficam para PRDs futuros que plugam novas boundaries HTTP sem refatorar o domínio. Esta decisão arquitetural foi consolidada em duas skills anteriores (`decision-brainstorming` + `technical-discovery-production`) e é **inegociável** neste PRD. Detalhes de algoritmo, schema, ADRs, plano de testes, observabilidade e segurança constam no dossiê técnico que serve de insumo direto para a especificação técnica (`create-technical-specification`).

### Volumetria-alvo e SLO do MVP

- **Usuários ativos**: 500 a 5.000 no horizonte de 12 meses.
- **Carga inbound**: ~500 mensagens WhatsApp/minuto em pico (média ~1 msg/segundo).
- **Tabela `auth_events`**: ~7.000 inserts/dia em estado estacionário, ~210k linhas/mês.
- **Disponibilidade**: SLO mensal de 99,5% no endpoint `/whatsapp/inbound` (≈3h36min de error budget/mês).
- **Latência**: p99 webhook end-to-end < 300ms; p99 de resolução `wa_id → user_id` < 100ms; p99 do middleware `RequireUser` (overhead) < 1ms.

Crescimento além desse teto exige revisão deste PRD.

## Objetivos

- **OBJ-01**: eliminar definitivamente o middleware `RequireUser` transitório por header `X-User-ID` cru declarado em `.specs/prd-card-crud-mvp/prd.md`, substituindo-o por contrato `auth.Principal` em `context.Context` antes da implementação de `card`, `categories` e `budgets`.
- **OBJ-02**: garantir isolamento absoluto por usuário (zero leak cross-tenant) em todos os módulos de domínio, fazendo `Principal` a única fonte legítima de `user_id` consumida por casos de uso.
- **OBJ-03**: blindar o canal WhatsApp contra spoofing de webhook via HMAC SHA-256 com rotação `current+next`, contra abuso via rate-limit por `user_id`, contra replay via dedup WAMID e contra prompt injection via separação entre contexto autenticado e argumentos do LLM.
- **OBJ-04**: cumprir LGPD em audit (tabela `auth_events` com retenção ≥ 90 dias) e direito à exclusão (anonimização de `user_id` em `auth_events` ao marcar usuário deletado).
- **OBJ-05**: preparar a arquitetura para evolução para app móvel e web sem refactor do domínio, documentando via ADR o contrato de boundary HTTP futura (JWT Ed25519 + refresh) que implementará a mesma interface conceitual de Principal.
- **OBJ-06**: respeitar o Padrão Obrigatório de Módulo do `AGENTS.md` e as Regras Estritas R0–R7 da skill `go-implementation`, sem introduzir dependência externa nova ao `go.mod`.
- **OBJ-07**: tornar inviável por linter custom que um handler HTTP de qualquer módulo leia identificação fora do contrato `auth.Principal` (proibição de `X-User-ID`, leitura de payload, ou `os.Getenv` direto para secrets de auth).

### Métricas de Sucesso

- **M-01**: 100% dos PRDs `card`, `categories` e `budgets` removem o `X-User-ID` transitório no primeiro PR de implementação, passando a depender exclusivamente do middleware `RequireUser` deste módulo.
- **M-02**: 0 (zero) ocorrência de leitura direta de header `X-User-`* em arquivos `internal/*/infrastructure/http/server/handlers/**`, validada por linter custom no CI.
- **M-03**: 100% das requisições autenticadas geram exatamente 1 evento `auth.principal_established` ou `auth.failed` projetado para `auth_events` (idempotência por `event_id`, garantida pelo outbox).
- **M-04**: p99 do middleware `RequireUser` (overhead puro, sem o trabalho do handler) ≤ 1 ms, medido em microbenchmark Go.
- **M-05**: p99 da resolução `wa_id → user_id` ≤ 100 ms, medido em produção via histograma OTel.
- **M-06**: p99 do webhook `/whatsapp/inbound` end-to-end ≤ 300 ms na volumetria-alvo.
- **M-07**: SLO mensal de disponibilidade ≥ 99,5% no endpoint `/whatsapp/inbound`, monitorado em dashboard "Auth Module" no Grafana.
- **M-08**: 0 ocorrência de `wa_id` cru, `META_APP_SECRET`, `Authorization` headers ou payload Meta integral em logs estruturados, spans OTel ou métricas, validado por teste de regressão que inspeciona o output do logger configurado.
- **M-09**: 0 (zero) métrica OTel com `user_id` como label (cardinalidade controlada), validado por revisão de PR.
- **M-10**: 100% das migrations associadas (criação de `auth_events`) possuem `down` que preserva dados via rename (jamais `DROP TABLE` direto).
- **M-11**: 0 ocorrência de `init()` ou `panic` em código de produção dos novos pacotes, validado por `go vet` + linter customizado no CI.
- **M-12**: taxa de assinatura inválida no webhook < 0,1% das requisições em estado estacionário; alerta operacional acima de 1%.
- **M-13**: rate-limit por `user_id` aplicado com 60 tokens/min + burst 60; nenhum cliente excedente continua dispatchando para o agent.
- **M-14**: ADR-001 publicado em `docs/adrs/auth/` documentando o contrato `auth.Principal` e a boundary HTTP futura para app/web, referenciado no PRD futuro de app/web.

## Histórias de Usuário

- **US-01 — Mensagem normal no WhatsApp é atribuída ao dono**
  Como usuário ativado do MeControla, quero que toda mensagem que eu enviar pelo WhatsApp seja inequivocamente vinculada à minha conta antes de qualquer registro financeiro ser criado, para que nenhuma informação minha vaze para outro usuário e nenhum registro alheio entre na minha base.

- **US-02 — Webhook hostil é rejeitado sem registrar dano**
  Como operador do MeControla, quero que qualquer POST no endpoint do WhatsApp sem assinatura HMAC válida seja rejeitado com 401 imediato, registrado no audit log como tentativa falha e sem qualquer dispatch para casos de uso, para que ataques de spoofing fiquem rastreáveis e sem efeito colateral.

- **US-03 — Caso de uso de domínio confia no contrato, não no transporte**
  Como desenvolvedor de `card`, `categories` ou `budgets`, quero que meus handlers recebam `user_id` exclusivamente via `auth.FromContext(ctx)` e que o compilador/linter me impeça de ler qualquer header de identificação, para que isolamento por tenant seja garantido por desenho e não por revisão manual.

- **US-04 — LLM tool não pode escalar privilégio**
  Como arquiteto do MeControla, quero que as tools chamadas pelo LLM derivem `user_id` exclusivamente do `context.Context` injetado pelo dispatcher e nunca de argumentos do prompt, para que prompt injection não permita um usuário operar sobre dados de outro.

- **US-05 — Operador rotaciona o secret Meta sem downtime**
  Como operador do MeControla, quero rotacionar o `META_APP_SECRET` aproveitando uma janela `CURRENT+NEXT` documentada em runbook, para que a troca aconteça sem queda do canal WhatsApp e com visibilidade operacional via métricas.

- **US-06 — Usuário deletado tem seu rastro anonimizado**
  Como responsável por LGPD do MeControla, quero que, ao marcar um usuário como deletado, o `user_id` desse usuário em `auth_events` seja anonimizado preservando `occurred_at`, `kind` e `source`, para honrar simultaneamente o direito à exclusão e a obrigação legal de retenção mínima de audit.

- **US-07 — Cliente abusivo não satura a infraestrutura**
  Como operador do MeControla, quero que um único usuário enviando mais de 60 mensagens por minuto pelo WhatsApp seja barrado pelo rate-limit, registrado no audit como `rate_limited` e tenha sua mensagem respondida com 200 OK ao Meta (sem retry storm), para que abusos não derrubem o serviço para os demais.

- **US-08 — Evolução para app/web sem refactor de domínio**
  Como engenheiro responsável pelo roadmap, quero que o contrato `auth.Principal` esteja documentado por ADR de modo que, quando app móvel e web chegarem, a única coisa a adicionar seja uma nova boundary HTTP que popule Principal a partir de JWT, sem alterar usecases nem repositories.

## Funcionalidades Core

- **FC-01 — Contrato `auth.Principal` em `context.Context`**
  Pacote `internal/identity/application/auth` expõe a struct minimal `Principal` (composta por `user_id UUID` e `source`), helpers `WithPrincipal(ctx, p)` e `FromContext(ctx) (Principal, bool)`. É a única forma legítima do domínio conhecer a identidade do usuário. Importância: torna o isolamento por tenant uma propriedade arquitetural, não convencional. Forma de atuar: usecases recebem `ctx`, leem Principal, derivam `user_id`, repassam para repositórios.

- **FC-02 — Middleware HTTP `RequireUser`**
  Substitui de forma definitiva o `RequireUser` transitório do PRD card. Retorna 401 imediato com corpo genérico `{"message":"unauthorized"}` quando `Principal` está ausente do `ctx`, sem revelar motivo. Importância: é o ponto único de aplicação da autorização em fronteira HTTP. Forma de atuar: composto após qualquer boundary que popula Principal (hoje só o dispatcher WhatsApp; amanhã também JWT/app/web).

- **FC-03 — Pacote compartilhado `internal/platform/whatsapp`**
  Centraliza validação HMAC SHA-256 (`signature/`), parse do envelope Meta (`payload/`), porta de dedup WAMID (`dedup/`), dispatcher único (`dispatcher/`) e rate-limit por `user_id` (`ratelimit/`). Migra primitivos hoje em `internal/onboarding/.../middleware` via Strangler Fig, preservando o fluxo de ativação existente. Importância: elimina duplicação quando o agent (LLM) existir e padroniza ponto de entrada. Forma de atuar: webhook único recebe POST, valida assinatura, faz dedup, roteia para onboarding (pré-ativação) ou agent (pós-ativação com Principal).

- **FC-04 — Resolução `wa_id → user_id` via Identity**
  Usecase `EstablishPrincipal(ctx, WhatsAppNumber) (Principal, error)` em `internal/identity/application/usecases` consulta `FindUserByWhatsApp`, publica `auth.principal_established` ou `auth.unknown_user` via `outbox.Publisher` na mesma transação e retorna o `Principal` ou erro tipado. Importância: ponto único de transição transporte → domínio. Forma de atuar: dispatcher chama o usecase logo após dedup WAMID e antes de qualquer rate-limit ou roteamento de negócio.

- **FC-05 — Audit log transacional via outbox**
  Tabela `auth_events` (PK UUID, padrão do projeto) recebe projeção do consumer `auth_events_consumer` que lê eventos `auth.principal_established`, `auth.failed{reason}` e `auth.unknown_user`. Idempotência garantida pelo padrão do outbox (`event_id`). Retenção mínima 90 dias. Importância: rastreabilidade LGPD e incident response. Forma de atuar: cada decisão de auth publica exatamente 1 evento outbox; consumer projeta para tabela; dashboard e alertas leem da tabela e das métricas.

- **FC-06 — Rate-limit por `user_id`**
  Token bucket de 60 tokens/min + burst 60 + refill 1 token/segundo, mantido em `sync.Map[uuid.UUID]*bucket` com goroutine de cleanup TTL 5 min (cancelável por `ctx` no shutdown cooperativo). Excedente publica `auth.failed{reason='rate_limited'}` e responde 200 OK ao Meta (evita retry storm). Importância: barra abusos sem afetar usuários regulares. Forma de atuar: aplicado pelo dispatcher imediatamente após resolver Principal e antes de chamar o handler do agent.

- **FC-07 — Linter custom proibindo `X-User-ID` e env-secret direto**
  Regra `depguard` bloqueia leitura de headers de identificação (`r.Header.Get("X-User-...")`) em qualquer arquivo dentro de `internal/*/infrastructure/http/server/handlers/**`. Regra `forbidigo` bloqueia `os.Getenv` para chaves cujo nome contenha `SECRET`, `TOKEN` ou `KEY` fora do pacote de configuração canônico. Importância: torna bypass de Principal um erro de compilação/CI, não um erro de revisão. Forma de atuar: configurado em `.golangci.yml`, falha o CI no primeiro PR que infringir.

- **FC-08 — ADR-001 contrato Principal + boundary HTTP futura**
  Documento `docs/adrs/auth/adr-001-principal-in-process-and-future-http-boundary.md` registra (a) a decisão de Principal in-process no MVP, (b) o contrato estável que qualquer boundary futura deve implementar, (c) o esqueleto não-implementado de uma boundary JWT Ed25519 + refresh para app/web. Importância: ancora a evolução do produto sem refactor de domínio. Forma de atuar: PRD futuro de app/web cita o ADR e implementa a boundary HTTP que popula Principal antes do `RequireUser`.

- **FC-09 — Rotação de `META_APP_SECRET` com janela `CURRENT+NEXT`**
  Runbook `docs/runbooks/auth-meta-secret-rotation.md` documenta o procedimento passo-a-passo aproveitando o suporte `secretCurrent+secretNext` já existente no middleware de assinatura. Métrica `meta_signature_status_total{status='rotated'}` indica progresso da rotação. Importância: rotação operacional sem downtime nem KMS. Forma de atuar: operador segue runbook; métrica audita uso prolongado de slot `rotated`.

- **FC-10 — Anonimização de `user_id` em `auth_events` ao deletar usuário**
  Usecase `MarkUserDeleted` (já existente) publica evento `user.deleted` que o `auth_events_consumer` consome anonimizando o campo `user_id` na tabela `auth_events` (substituição por `NULL` preservando `occurred_at`, `kind`, `source`). Importância: cumpre simultaneamente LGPD art. 18 (exclusão) e obrigação de retenção mínima de audit. Forma de atuar: pipeline transacional outbox + consumer, mesma garantia idempotente.

## Requisitos Funcionais

- **RF-01**: Expor pacote `internal/identity/application/auth` com a struct `Principal` minimal (somente `user_id UUID` e `source PrincipalSource`), helpers `WithPrincipal(ctx, Principal)` e `FromContext(ctx) (Principal, bool)`, e constante `SourceWhatsApp`. `Principal` é imutável por valor; a chave de `context` é um tipo não-exportado.
- **RF-02**: Expor middleware HTTP `RequireUser(next http.Handler) http.Handler` em `internal/identity/infrastructure/http/server/middleware` que retorna HTTP 401 com corpo `{"message":"unauthorized"}` e header `Content-Type: application/json` quando `Principal` está ausente no `ctx`. O middleware não revela motivo da falha, não loga `user_id` automaticamente e não consulta nenhum banco.
- **RF-03**: Expor usecase `EstablishPrincipal(ctx, vo WhatsAppNumber) (Principal, error)` em `internal/identity/application/usecases` que: (a) chama `FindUserByWhatsApp`; (b) publica `auth.principal_established{user_id, source='whatsapp'}` via `outbox.Publisher` na mesma transação em caso de sucesso; (c) publica `auth.unknown_user` em caso de usuário não encontrado/deletado e retorna `ErrUnknownUser` tipado; (d) propaga outros erros via `%w`.
- **RF-04**: Criar pacote `internal/platform/whatsapp` com os submódulos `signature/`, `payload/`, `dedup/`, `dispatcher/` e `ratelimit/`. Migração via Strangler Fig: criar novo pacote, migrar `internal/onboarding/.../middleware/meta_signature.go`, `raw_body_buffer.go`, parse Meta e dedup WAMID, e depois reescrever `onboarding` para consumir o novo pacote. O comportamento observável do webhook de onboarding existente não pode regredir.
- **RF-05**: Expor middleware HMAC SHA-256 em `internal/platform/whatsapp/signature` mantendo a semântica `secretCurrent+secretNext` e os status `valid/invalid/rotated`. Assinatura ausente é tratada como inválida. Inválida → HTTP 401 com corpo genérico + publicação de `auth.failed{reason='invalid_signature'}`.
- **RF-06**: Expor `Dispatcher.Route(ctx, msg) RouteOutcome` em `internal/platform/whatsapp/dispatcher` que aplica regex de `ATIVAR <token>` para rotear para o handler de onboarding (pré-ativação), chama `EstablishPrincipal` quando a mensagem não é de ativação, aplica rate-limit por `user_id` em caso de Principal estabelecido, e roteia para o handler de fallback de onboarding quando o usuário não existe.
- **RF-07**: Expor `Limiter.Allow(userID uuid.UUID) bool` em `internal/platform/whatsapp/ratelimit` com bucket de 60 tokens (constante exportada `DefaultBucketCapacity = 60`), refill 1 token/segundo (constante `DefaultRefillPerSecond = 1`) e burst 60. Estado mantido em `sync.Map[uuid.UUID]*bucket`. Os parâmetros MUST ser constantes Go no MVP (sem env var override, sem flag, sem config file) — ajuste futuro exige PR com revisão de código. Excedente publica `auth.failed{reason='rate_limited'}` e o webhook responde HTTP 200 ao Meta (evita retry storm).
- **RF-08**: O `Limiter` MUST expor `Shutdown(ctx) error` e a goroutine de cleanup MUST ser cancelável via `ctx` injetado pelo lifecycle do app. A goroutine remove buckets inativos > 5 minutos a cada 60 segundos. Métrica `whatsapp_ratelimit_buckets_count` (gauge) é expostas e alerta dispara se > 10.000.
- **RF-09**: Criar migration `00XX_create_identity_auth_events.up.sql` + `.down.sql` para a tabela `auth_events` com schema: `id UUID NOT NULL`, `occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()`, `user_id UUID NULL`, `kind TEXT NOT NULL`, `source TEXT NOT NULL`, `reason TEXT NULL`, `CONSTRAINT auth_events_pkey PRIMARY KEY (id)`, `CONSTRAINT auth_events_kind_check CHECK (kind IN ('principal_established','failed','unknown_user'))`, `CONSTRAINT auth_events_source_check CHECK (source = 'whatsapp')` (CHECK restrito ao MVP; migrations futuras adicionam `'jwt'` quando boundary HTTP for implementada), `CONSTRAINT auth_events_reason_check CHECK ((kind = 'failed' AND reason IN ('invalid_signature','unknown_wa_id','invalid_country','invalid_payload','rate_limited','db_unavailable')) OR (kind <> 'failed' AND reason IS NULL))`. Índices: `(user_id, occurred_at DESC) WHERE user_id IS NOT NULL` e `(occurred_at DESC, reason) WHERE kind='failed'`. `down` migration MUST preservar dados via `ALTER TABLE ... RENAME TO auth_events_archived_<ts>`, nunca `DROP TABLE` direto.
- **RF-10**: Criar `auth_events_consumer` em `internal/identity/infrastructure/messaging/database/consumers` que projeta eventos `auth.principal_established`, `auth.failed{reason}` e `auth.unknown_user` para `auth_events`. Idempotência por `event_id` (padrão obrigatório do outbox).
- **RF-11**: O `auth_events_consumer` MUST consumir o evento `user.deleted` (publicado pelo usecase `MarkUserDeleted` existente) e atualizar todas as linhas em `auth_events` com aquele `user_id` para `user_id = NULL`, preservando `occurred_at`, `kind` e `source`. A operação MUST ser idempotente.
- **RF-12**: Adicionar regra `depguard` em `.golangci.yml` que bloqueia TODA chamada a `r.Header.Get(...)` e `r.Header.Values(...)` em arquivos dentro de `internal/*/infrastructure/http/server/handlers/**`, com allowlist explícita restrita a `X-Request-ID`, `Content-Type`, `Idempotency-Key` (e outras chaves operacionalmente seguras a serem incluídas conforme necessidade, sempre por PR com revisão). Qualquer leitura de header não-allowlisted MUST falhar o CI. Justificativa: headers HTTP são case-insensitive (`X-User-Id`, `x-user-id`, `X-USER-ID` equivalem), e bloquear apenas `X-User-*` permitiria bypass via `r.Header[textproto.CanonicalMIMEHeaderKey(...)]`. A regra ampla com allowlist torna a violação um erro de compilação/CI, não de revisão humana.
- **RF-13**: Adicionar regra `forbidigo` em `.golangci.yml` que bloqueia `os.Getenv` para chaves cujo nome contenha `SECRET`, `TOKEN` ou `KEY` em qualquer arquivo fora do pacote canônico de configuração (`internal/platform/config`). Falha do linter MUST falhar o CI.
- **RF-14**: Publicar ADR `docs/adrs/auth/adr-001-principal-in-process-and-future-http-boundary.md` descrevendo: contrato `auth.Principal`, regra de invariância (handlers/usecases nunca leem identificação fora do contrato), esboço da boundary HTTP futura (JWT Ed25519 + refresh + claims mínimos + JWKs), e relação com os PRDs `card`, `categories`, `budgets`.
- **RF-15**: Publicar runbook `docs/runbooks/auth-meta-secret-rotation.md` com o procedimento de rotação `CURRENT+NEXT` e checklist de validação (métrica `meta_signature_status_total{status='rotated'}`).
- **RF-16**: Publicar runbook `docs/runbooks/auth-incident-response.md` para alertas `auth_failed_total{reason='invalid_signature'}`, `auth_failed_total{reason='db_unavailable'}` e `outbox_publish_failed_total{kind=~"auth\\..*"}`.
- **RF-17**: Instrumentar as métricas OTel: `auth_principal_established_total{source}`, `auth_failed_total{reason}`, `auth_unknown_wa_id_total`, `auth_rate_limit_hits_total`, `auth_resolve_wa_duration_seconds` (histograma), `whatsapp_dispatcher_route_total{outcome}`, `whatsapp_ratelimit_buckets_count` (gauge), `whatsapp_ratelimit_cleanup_duration_seconds`. Reusar `meta_signature_status_total{status}` já presente em onboarding. Nenhuma métrica MUST ter `user_id` como label.
- **RF-18**: Instrumentar spans OTel: `auth.resolve_principal` (atributos `source`, `outcome ∈ {found, unknown, error}`, `user_id` apenas em sucesso), `auth.require_user` (atributo `result ∈ {pass, unauthorized}`), `whatsapp.dispatcher.route` (atributos `outcome`, `is_activation`, `is_dedup`). Nenhum span MUST conter `wa_id` cru.
- **RF-19**: Logar via `slog` com campos consistentes: `trace_id` e `span_id` (propagados automaticamente do span OTel ativo via handler do devkit), `user_id` quando Principal estabelecido, `source`, `wa_id_masked` (via VO `WhatsAppNumber.Masked()` existente em `internal/identity/domain/pii`). Não introduzir `correlation_id` separado — `trace_id` é o único identificador de correlação no MVP (audit↔logs↔traces ligam por `trace_id`). Teste de regressão MUST inspecionar o output do logger configurado e falhar se identificar `wa_id` cru, `Authorization` ou `META_APP_SECRET` em qualquer campo.
- **RF-20**: Publicar dashboard Grafana "Auth Module" com painéis para cada métrica do RF-17 e alertas: `auth_failed_total{reason='invalid_signature'}` > 0 em 5 min, `auth_failed_total{reason='db_unavailable'}` > 1 em 1 min, `whatsapp_ratelimit_buckets_count` > 10000, `auth_resolve_wa_duration_seconds` p99 > 100 ms sustentado por 3 dias, `outbox_publish_failed_total{kind=~"auth\\..*"}` > 0.
- **RF-21**: Atualizar `.specs/prd-card-crud-mvp/prd.md`, `.specs/prd-categories-crud/prd.md` e `.specs/prd-budgets-monthly/prd.md` removendo a referência ao middleware `RequireUser` transitório por `X-User-ID` e passando a depender deste PRD como pré-condição. Atualização MUST ocorrer no primeiro PR de implementação de cada um desses PRDs, não neste PRD.
- **RF-22**: Criar housekeeping job mensal em `internal/identity/infrastructure/jobs/handlers/auth_events_housekeeping_job.go` (seguindo padrão de `internal/billing/infrastructure/jobs/handlers/`) que executa `DELETE FROM auth_events WHERE occurred_at < now() - INTERVAL '180 days'`. O job MUST: (a) rodar uma vez por mês (cron-like ou trigger por outbox housekeeping existente); (b) emitir métrica `auth_events_housekeeping_deleted_total` (counter); (c) emitir métrica `auth_events_housekeeping_duration_seconds` (histograma); (d) operar em lotes de 10.000 linhas por iteração com `LIMIT` para evitar lock de tabela; (e) ser idempotente (rodar duas vezes no mesmo mês não causa efeito colateral negativo).
- **RF-23**: Travar as decisões de produto inegociáveis no MVP, derivadas das suposições eliminadas em v2: (a) orquestrador LLM roda **in-process** no mesmo binário Go (rejeitado sidecar, externo); (b) pacote do dispatcher WhatsApp é `internal/platform/whatsapp` (rejeitado `messaging/whatsapp`); (c) pacote do orquestrador é `internal/agent` (rejeitado `assistant`/`copilot`/`chat`); (d) rate-limit é constante Go fixa (rejeitado env var override). Qualquer mudança nessas decisões exige novo PRD ou novo `v3+` deste PRD.
- **RF-24**: A migration de criação de `auth_events` MUST receber o próximo número livre de 4 dígitos disponível em `migrations/` no momento da geração da techspec. A techspec é a fonte de verdade da numeração final. CI MUST validar que nenhum número de migration está duplicado (regra a ser declarada na techspec).
- **RF-25**: Expor o webhook unificado WhatsApp em dois endpoints HTTP versionados: `POST /api/v1/whatsapp/inbound` (recebe mensagens, valida HMAC SHA-256, dispatcha) e `GET /api/v1/whatsapp/verify` (handshake `verify_token` do Meta, retorna `hub.challenge`). Prefixo `/api/v1` alinhado com `.specs/prd-card-crud-mvp/prd.md` (`/api/v1/cards`). Mudança de path exige novo PRD ou novo `v4+` deste PRD.
- **RF-26**: `EstablishPrincipal` MUST executar `FindUserByWhatsApp` e a publicação dos eventos `auth.principal_established` ou `auth.unknown_user` via `outbox.Publisher` dentro de uma **única transação curta** obtida via Unit of Work do `identityFactory` (padrão ADR-008 `repository-factory-per-module`). A transação MUST terminar (Commit ou Rollback) antes do dispatcher avançar para rate-limit ou roteamento. Falha de outbox MUST causar rollback e retornar erro tipado ao dispatcher; o webhook responde HTTP 5xx ao Meta (gera retry). Isso garante atomicidade audit↔resolução.
- **RF-27**: A constante Go `SourceWhatsApp` (do tipo `PrincipalSource`) MUST ser a ÚNICA constante exportada de fonte de Principal no MVP. Valores futuros (`SourceJWT`, `SourceSystem`) MUST ser documentados em ADR-001 mas NÃO declarados como constantes Go enquanto não tiverem implementação real consumindo. A migration que adicionar `'jwt'` ao CHECK de `source` MUST ser entregue no mesmo PR que entrega a constante Go correspondente e o teste de uso.
- **RF-28**: A migração do código de `internal/onboarding/.../middleware/meta_signature.go`, `raw_body_buffer.go` e o parser Meta para `internal/platform/whatsapp` MUST acontecer em **PR único atômico** que: (a) cria os novos pacotes; (b) reescreve onboarding para consumir os novos pacotes; (c) **deleta** os arquivos antigos do onboarding no mesmo PR. Não é permitido deixar duplicação em coexistência entre PRs. Justificativa: duplicação de código de criptografia em produção amplia surface de erro humano e contraria o objetivo de eliminação de duplicação.
- **RF-29**: Acceptance criteria do épico de auth: load test k6 (ou vegeta) em ambiente de staging com Postgres real, sustentando **500 mensagens/min por 10 minutos** com p99 webhook < 300 ms e zero erro 5xx. Script versionado em `scripts/load-test/auth-webhook.k6.js`. O épico só pode ser declarado concluído quando o relatório do load test for anexado ao último PR.
- **RF-30**: Acceptance criteria de release: comando `task auth:smoke` MUST: (a) enviar POST de payload Meta válido com HMAC SHA-256 assinado para `/api/v1/whatsapp/inbound` em ambiente de staging; (b) validar HTTP 200 OK; (c) confirmar via SQL que uma linha foi inserida em `auth_events` com `kind='principal_established'` ou `kind='unknown_user'` dentro de 5 segundos. CI MUST executar `task auth:smoke` no merge para `main` e o pipeline de deploy MUST executar o smoke após cada deploy em staging, abortando o deploy em produção se o smoke falhar.
- **RF-31**: Atualizar `.specs/prd-onboarding-magic-token/prd.md` (spec-version bump) registrando: (a) que `meta_signature.go`, `raw_body_buffer.go` e o parser Meta foram migrados para `internal/platform/whatsapp` por este PRD; (b) referência ao novo `adr-002-auth-strangler-fig-onboarding.md` (a ser publicado por este épico) detalhando a migração; (c) reaviso de que o comportamento observável dos endpoints de onboarding (HMAC, dedup WAMID, parse Meta) não regrediu. Atualização MUST ocorrer no mesmo PR do RF-28.
- **RF-32**: O `Limiter` do rate-limit MUST expor as funções `Start(ctx context.Context) error` e `Shutdown(ctx context.Context) error`. `Start` inicia a goroutine de cleanup TTL 5min com `ctx` cancelável; `Shutdown` cancela a goroutine e aguarda finalização cooperativa (timeout configurável, default 5 segundos). Ambos MUST ser registrados no `module.go` do `internal/identity` (ou do pacote `internal/platform/whatsapp/ratelimit` se preferível na techspec) seguindo o padrão lifecycle.Hook usado por `outbox.Dispatcher` e `housekeeping`. `cmd/api/main.go` MUST chamar `Start` antes de servir HTTP e `Shutdown` em `SIGTERM` antes de fechar PG. Race detector MUST ser obrigatório nos testes do pacote `ratelimit`.
- **RF-33**: O CHECK de `reason` em `auth_events` MUST incluir o valor `'invalid_payload'` além dos já listados em RF-09, cobrindo o caso de JSON malformado recebido após HMAC validado. O dispatcher MUST publicar `auth.failed{reason='invalid_payload'}` quando o parser do envelope Meta falhar e MUST responder HTTP 200 OK ao Meta (evita retry storm de payload corrupto).
- **RF-34**: O usecase `MarkUserDeleted` (em `internal/identity/application/usecases/mark_user_deleted.go`) MUST publicar o evento outbox `user.deleted` na mesma transação do soft-delete do usuário (`UPDATE users SET deleted_at = now(), status = 'DELETED' WHERE id = $1`). Payload JSON: `{"event_id": "<uuid v7>", "user_id": "<uuid>", "deleted_at": "<rfc3339>"}`. `outbox.Event.Type = "user.deleted"`, `AggregateType = "user"`, `AggregateID = user_id`. Este PRD MUST verificar se a publicação já existe; se ausente, MUST adicionar como parte do épico de auth (não delegar ao PRD identity-foundation). O `auth_events_consumer` (RF-11) MUST consumir esse evento e anonimizar `user_id` em `auth_events` via `UPDATE auth_events SET user_id = NULL WHERE user_id = $1`.
- **RF-35**: Enquanto `internal/agent` não existir, o dispatcher MUST rotear mensagens **não-ATIVAR** de usuário ativo (Principal estabelecido + rate-limit ok) para um **stub do agent** que: (a) envia ao usuário, via `WhatsAppGateway` reusado do onboarding, o template Meta pré-aprovado **"MeControla recebeu sua mensagem — estamos preparando sua experiência"** (ou variante traduzida aprovada pelo time de produto); (b) emite log INFO `agent_stub_invoked` com `trace_id`, `user_id`, `wa_id_masked`; (c) emite métrica `whatsapp_dispatcher_route_total{outcome="agent_stub"}`. Quando o PRD `internal/agent` for criado, o stub MUST ser removido e substituído pelo orquestrador real, no MESMO PR de implementação do agent.
- **RF-36**: Para suportar `task auth:smoke` em staging, MUST existir uma migration **condicional de seed** que cria um usuário ativo determinístico em ambiente de staging com `whatsapp_number` lido da env var `STAGING_SMOKE_WA`. A migration MUST: (a) ter `up` que só executa se `current_database()` contém substring `staging` (ou via flag de ambiente equivalente, decidido pela techspec); (b) ser idempotente (`ON CONFLICT (id) DO NOTHING`); (c) usar UUID fixo conhecido para o user de smoke (constante exportada em `scripts/smoke/`); (d) ter `down` que apaga apenas esse user específico. O env var `STAGING_SMOKE_WA` MUST estar documentada em `docs/runbooks/auth-incident-response.md` ou runbook próprio de smoke.

## Experiência do Usuário

A funcionalidade é predominantemente backend, mas a experiência observável ao usuário final pelo WhatsApp é parte do contrato:

- **Mensagem normal pós-ativação**: usuário envia "ifood 58" → backend valida HMAC, resolve Principal, aplica rate-limit, dispatcha para o agent (LLM). Usuário recebe resposta do agent sem nenhuma menção a autenticação. Latência percebida (p99 < 300ms backend + tempo de LLM/Meta).
- **Mensagem de cliente não-ativado**: backend identifica que `wa_id` não corresponde a usuário ativo, roteia para o handler de fallback do onboarding (PRD existente), que conduz o fluxo de ativação. Usuário não percebe troca de pipeline.
- **Tentativa de spoofing**: requisição com assinatura inválida recebe 401 sem corpo descritivo. Usuário final não vê nada (não há `wa_id` legítimo). Operador vê alerta operacional.
- **Rate-limit atingido**: usuário ultrapassa 60 msg/min. Recebe ao menos uma mensagem informativa (template Meta "limite atingido", opcional, definido pelo PRD do agent) e mensagens excedentes são descartadas com `auth.failed{reason='rate_limited'}` registrado. Webhook responde 200 OK ao Meta evitando retry storm.
- **Operador rotaciona secret**: zero downtime durante a janela `CURRENT+NEXT`. Métrica `meta_signature_status_total{status='rotated'}` indica progresso.
- **Acessibilidade**: não aplicável (sem UI no escopo).

## Restrições Técnicas de Alto Nível

- **Stack obrigatório**: Go conforme versão declarada em `go.mod`. Postgres como único storage. `outbox.Publisher` como único canal transacional de eventos de auth. Nenhuma dependência externa nova ao `go.mod` (sem Auth0/Clerk/Keycloak/Supabase/Vault/Redis/Memcached).
- **Deploy**: VPS Hostinger sem KMS/HSM, conforme ADR-009 do PRD `prd-onboarding-magic-token`. Segredos via env file + reverse proxy. Rotação manual documentada em runbook.
- **Compliance/LGPD**: PII mascarada em logs/spans/métricas (VO `WhatsAppNumber.Masked()` já existente é a única forma de logar `wa_id`); `auth_events` com retenção mínima 90 dias; direito à exclusão honrado via anonimização de `user_id` em `auth_events` ao deletar usuário; `user_id` jamais usado como label de métrica OTel.
- **Segurança**: HMAC SHA-256 com rotação `current+next`; comparação constant-time obrigatória (`hmac.Equal`); resposta 401 sem revelar motivo (evita oracle); rate-limit por `user_id` com 60 tokens/min; linter custom proíbe handler ler `X-User-ID` ou env-secret direto; tools do LLM derivam `user_id` exclusivamente de `auth.FromContext(ctx)`.
- **Governança Go**: aderência total a R0–R7 da skill `go-implementation`: R0 (sem `init()`), R5.12 (sem `panic` em produção), R6 (`context.Context` em fronteiras de IO, interface no consumidor nunca no produtor), R7.6 (`errors.Join` + `%w`), goroutines canceláveis com shutdown cooperativo. Padrão Obrigatório de Módulo do `AGENTS.md` para `internal/identity`.
- **Performance**: p99 webhook end-to-end ≤ 300 ms; p99 `auth.resolve_principal` ≤ 100 ms; p99 `RequireUser` overhead ≤ 1 ms (microbenchmark Go); SLO mensal 99,5%. Microbenchmark obrigatório para `WithPrincipal/FromContext` (alvo < 50 ns/op) e `Limiter.Allow` (alvo < 200 ns/op para 5.000 buckets).
- **Concorrência**: rate-limit usa `sync.Map`; goroutine de cleanup é cancelável via `ctx`; race detector obrigatório nos testes do pacote `ratelimit`.
- **Webhook contract Meta**: resposta HTTP 200 OK ao Meta em todos os casos de mensagem decidida (válida, inválida por país, descartada por rate-limit). HTTP 401 reservado a assinatura HMAC inválida. HTTP 5xx reservado a falha de dependência crítica (PG/outbox indisponível) — gera retry do Meta.
- **Migrations**: `down` migration MUST preservar `auth_events` via rename, jamais `DROP TABLE` direto. Padrão de PK UUID seguindo `users`, `billing_*`, etc.
- **Linters**: regras `depguard` + `forbidigo` em `.golangci.yml`; falha do linter falha o CI.
- **Sem cache no MVP**: a resolução `wa_id → user_id` consulta Postgres direto a cada mensagem; cache LRU é gatilho documentado caso p99 estoure 100 ms por 3 dias consecutivos.
- **Volumetria**: 500 a 5.000 usuários ativos; pico ~500 msg/min; ~7k inserts/dia em `auth_events`. Acima desse teto, revisar este PRD.
- **Retenção `auth_events` travada em 180 dias**: housekeeping mensal apaga linhas com `occurred_at < now() - 180 days`. Operação em lotes de 10.000 linhas para evitar lock. Sem arquivamento externo (zero infra nova).
- **Anonimização travada em `UPDATE user_id = NULL`**: irreversível, sem hash com sal. Cumpre LGPD art. 18 preservando `kind`, `source`, `occurred_at` para audit estatístico.
- **LLM in-process travado**: orquestrador roda no mesmo binário Go que o webhook. Sem sidecar, sem serviço remoto. Tools são funções Go nativas; recebem `ctx` com Principal; nunca veem `user_id` em args.
- **Rate-limit como constante Go**: sem env var, sem flag, sem config file. Ajuste exige PR com revisão.

## Fora de Escopo

- **JWT, JWKs, kid rotation, refresh token**: ficam para PRD futuro dedicado à boundary HTTP de app móvel e web. ADR-001 deste PRD documenta o contrato que esse PRD futuro deve implementar.
- **Tabela `auth_sessions` persistida e revogação ativa**: avaliadas no brainstorming e rejeitadas para o MVP. Revogação imediata após `MarkUserDeleted` não é garantida (janela de exposição equivale ao tempo de re-resolução por mensagem).
- **Provedores OIDC externos** (Auth0, Clerk, Keycloak self-hosted, Supabase Auth): fora do MVP por restrição "zero dependência externa nova".
- **mTLS interno** entre componentes.
- **RBAC e papéis de usuário**: todos os usuários têm o mesmo papel hoje; introdução de papéis fica para PRD próprio quando necessário.
- **Login web ou fluxos UI de autenticação direta**: app/web não estão neste MVP.
- **Rate-limit global por IP**: foco do MVP é por `user_id`; rate-limit por IP fica para PRD operacional próprio caso ataques DDoS por IP se tornem relevantes.
- **Replay protection por timestamp Meta**: Meta não envia header oficial de timestamp; será reavaliado se publicar.
- **Cache `wa_id → user_id`**: rejeitado no MVP com gatilho de revisão métrico documentado.
- **Implementação do orquestrador LLM (`internal/agent`)**: este PRD define apenas o **contrato** que o agent vai consumir (`ctx` com Principal). A criação do pacote `internal/agent`, suas tools, integração com OpenAI/Anthropic e estratégia de prompt-engineering ficam para PRD próprio.
- **Implementação dos módulos `card`, `categories`, `budgets`**: este PRD desbloqueia esses PRDs ao entregar `RequireUser` definitivo, mas não implementa nada neles. A remoção do `X-User-ID` transitório em cada PRD acontece no primeiro PR de implementação respectivo.
- **Validação empírica de 60 msg/min como teto adequado**: ficará como métrica observável pós-lançamento; ajuste numérico fica para revisão futura sem necessitar de novo PRD se for parametrização.
- **Anonimização avançada de IP/correlation_id em `auth_events`**: o MVP grava apenas `id, occurred_at, user_id, kind, source`. Campos extras (`reason`, `correlation_id`, `ip_hash`, `metadata JSONB`) ficam para PRD próprio caso compliance ou incident response demandem.

## Suposições e Questões em Aberto

Nenhuma. As 7 suposições e questões da v1 foram eliminadas em v2 (S-01 a S-04, Q-01 a Q-03). As 12 lacunas implícitas descobertas em revisão adversarial da v2 foram eliminadas em v3 (A1, A2, A3, A4, B1, B2, B3, B4, C1, C2, C3, C4). As 12 lacunas adicionais descobertas em revisão adversarial da techspec foram eliminadas em v4 (A1: UUID v7; A2-bis: `invalid_payload`; A3-bis: dedup silencioso; A4-bis: convenção `domain.event`; B1-bis: `user.deleted` publish via MarkUserDeleted; B2-bis: `signature.Compose()` wrapper; B3-bis: framework de teste = mesmo do `migrations_integration_test.go`; B4-bis: stub do agent com template Meta; C1-bis: shutdown order HTTP→Dispatcher→Limiter→Consumer→Housekeeping→PG; C2-bis: migration de seed staging; C3-bis: auditoria de handlers antes do linter; C4-bis: config canonical em `configs/`). Qualquer reabertura exige nova versão deste PRD.

### Travas inegociáveis para a techspec

A techspec (`create-technical-specification`) consumindo este PRD MUST honrar sem reabrir:

1. Pacote do contrato em `internal/identity/application/auth`; pacote do dispatcher em `internal/platform/whatsapp`; pacote do orquestrador LLM em `internal/agent`.
2. `Principal` é struct concreta minimal por valor; única constante de source no MVP é `SourceWhatsApp`.
3. Schema de `auth_events`: `id, occurred_at, user_id, kind, source, reason`; CHECK de `source` restrito a `'whatsapp'`; CHECK de `reason` condicional ao `kind='failed'`.
4. Paths: `POST /api/v1/whatsapp/inbound` e `GET /api/v1/whatsapp/verify`.
5. Transação única no `EstablishPrincipal` via UoW do identity (FindUserByWhatsApp + outbox publish + Commit).
6. Sem `correlation_id` separado; `trace_id` é o único identificador de correlação.
7. Linter custom: depguard bloqueia toda `r.Header.Get/.Values` em handlers com allowlist; forbidigo bloqueia `os.Getenv` para secrets fora de `internal/platform/config`.
8. Strangler Fig em PR único e atômico (cria novo + migra onboarding + deleta antigo).
9. Rate-limit: bucket 60 + refill 1/s + burst 60 como constantes Go (sem env var); cleanup TTL 5 min; `Start`/`Shutdown` via lifecycle hook do `module.go`.
10. Retenção `auth_events` 180 dias com housekeeping mensal em lotes de 10k linhas.
11. Anonimização em `MarkUserDeleted` via `UPDATE user_id = NULL` simples e irreversível, consumido pelo `auth_events_consumer`.
12. LLM in-process no mesmo binário Go; tools recebem `ctx` com Principal e derivam `user_id` exclusivamente de `auth.FromContext(ctx)`.
13. Load test k6 e `task auth:smoke` são acceptance criteria do épico e gate de release.
14. ADR-001 (contrato Principal + boundary HTTP futura) e ADR-002 (Strangler Fig onboarding) são entregáveis obrigatórios.
15. `prd-onboarding-magic-token` recebe spec-version bump documentando a migração no mesmo PR do RF-28/RF-31.
