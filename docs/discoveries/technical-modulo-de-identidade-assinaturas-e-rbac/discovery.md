# DOSSIÊ DE DISCOVERY TÉCNICO

## Título
Módulo de identidade, assinaturas e RBAC do MeControla — viabilizar jornada cadastro→pagamento Pix→ativação no backend Go canônico, preservando WhatsApp como canal de uso diário em release v1.1.

## Resumo Executivo
Contexto:
O MeControla é um monolito Go modular hexagonal com módulos `identity`, `finance`, `conversation`, `agent`, `notifications` e `telemetry`, infraestrutura compartilhada em `internal/infrastructure` (PostgreSQL, outbox transacional, OTel, runtime, http, errors) e Go 1.26.3. A landing pública vive em repositório separado (`https://github.com/LimaTeixeiraTecnologia/mecontrola-landingpage`) e promete uso 100% no WhatsApp após pagamento. O brainstorming decisório selecionou a Alternativa 5 (híbrida em fases com Mercado Pago Pix no MVP) e esta descoberta detalha sua execução production-proof na AWS.

Recomendação:
Materializar v1 como beta fechado (sem divulgação pública) cobrindo cadastro/login OIDC via Cognito, checkout Pix avulso por período, bloqueio por status de assinatura, RBAC tabular com ownership por `owner_user_id` e auditoria. Materializar v1.1 com vinculação WhatsApp por código temporário usando WhatsApp Cloud API oficial Meta. Infra AWS provisionada por Terraform: ECS Fargate (API + worker), RDS Postgres Multi-AZ, Cognito, SES, KMS, Secrets Manager, ALB+WAF, observabilidade via OTel Collector → Grafana Cloud.

Status de viabilidade:
viável com restrições — aprovado se os bloqueadores listados na Viabilidade Técnica forem tratados antes da release v1.

## Necessidade e Objetivos
Problema atual:
A landing pública vende planos e promete uso 100% no WhatsApp, mas não há backend de identidade/assinatura/RBAC/ownership disponível para sustentar a jornada de compra→ativação→uso. Sem esse módulo, o produto não pode cobrar, ativar, bloquear nem isolar dados por usuário. O risco inaceitável registrado no brainstorming combina vazamento cross-user, cobrança incorreta/não reconciliada e dependência forte de provedor sem plano de troca.

Objetivos de negócio:
- Validar receita por assinatura recorrente Pix com 500→5k MAU em 12 meses sem violar promessa de uso WhatsApp.
- Preservar confiança garantindo que nenhum usuário acesse dados de outro e que toda cobrança seja idempotente e auditável.
- Manter custo AWS dentro de US$150-250/mês na v1 e US$300-450/mês na v1.1 sem sacrificar SLOs.

Objetivos técnicos:
- Implementar identidade interna, assinatura, RBAC tabular e ownership por `owner_user_id` no monolito Go, preservando fronteiras hexagonais.
- Integrar OIDC (Amazon Cognito) para autenticação e Mercado Pago Pix para cobrança, ambos atrás de portas de aplicação trocáveis.
- Reusar `outbox.Publisher` (commit `4b7149e`) para side-effects pós-commit críticos (lembretes, ativação, bloqueio, e-mail) e `events.Bus` apenas para sinais voláteis.
- Entregar webhooks idempotentes do Mercado Pago, reconciliação periódica e telemetria de SLO desde o v1.
- Cumprir LGPD para CPF/telefone via pgcrypto + KMS com hashes determinísticos para lookup.

## Materiais de Apoio
- `docs/discoveries/brainstorm-modulo-de-identidade-assinaturas-e-rbac/decision-brief.md` — decisão Alternativa 5 com trade-offs aceitos.
- `docs/discoveries/brainstorm-modulo-de-identidade-assinaturas-e-rbac/option-scorecard.md` — pontuação 39/45 da Alternativa 5.
- `docs/discoveries/brainstorm-modulo-de-identidade-assinaturas-e-rbac/assumptions.md` — hipóteses confirmadas e não validadas.
- `docs/discoveries/brainstorm-modulo-de-identidade-assinaturas-e-rbac/transcript.md` — histórico completo das 5 rodadas anteriores.
- `AGENTS.md` — regras transversais; seção "Outbox vs events.Bus" com contrato `outbox.Publisher`.
- `internal/identity/`, `internal/finance/`, `internal/infrastructure/outbox/` — código existente que será estendido.
- `migrations/0001_*.sql`, `migrations/0002_outbox.up.sql` — fundação atual de schema.
- `https://www.mecontrola.app.br/` — landing pública (planos, FAQ, promessa WhatsApp).
- `https://github.com/LimaTeixeiraTecnologia/mecontrola-landingpage` — repositório da landing (declarado pelo usuário).
- Work item 29 Azure DevOps — não consultado nesta sessão (MCP exige login interativo). Risco listado em "Itens em Aberto".

## Escopo
Inclui:
- v1: APIs de cadastro/login/refresh/logout via Cognito Authorization Code + PKCE.
- v1: Domínio de assinatura com planos mensal/trimestral/anual, criação de cobrança Pix no Mercado Pago, webhook idempotente, ativação por sucesso e bloqueio por expiração.
- v1: RBAC tabular (`roles`, `permissions`, `role_permissions`, `user_roles`) e ownership por `owner_user_id` aplicado em middleware e repository scoping.
- v1: Auditoria de eventos sensíveis (login, alteração de plano, mudança de papel, leitura administrativa) em tabela `audit_log` consultável.
- v1: E-mails transacionais via SES (recibo Pix, lembrete D-3/D-1, expiração, reativação).
- v1: Observabilidade OTel → Grafana Cloud com dashboards de funil, SLO e webhook MP.
- v1: Provisionamento AWS por Terraform (Cognito, ECS Fargate, RDS Multi-AZ, ALB, WAF, NAT, Secrets Manager, KMS, SES, IAM).
- v1: Endpoint admin LGPD `/admin/lgpd/requests` com fluxo manual de exclusão.
- v1.1: Vinculação WhatsApp por código temporário usando WhatsApp Cloud API oficial Meta; bloqueio de comandos quando assinatura inativa.

Exclui:
- Login por senha dentro do WhatsApp.
- Onboarding conversacional completo (financeiro inicial via WhatsApp) — escopo do agente, fora desta descoberta.
- Pix Automático/Subscriptions do Mercado Pago no v1 (avaliado, adiado por maturidade e por escolha de modelo Pix avulso).
- SSO corporativo, organizações/equipes e múltiplos PSPs (portas internas previstas, implementação adiada).
- BFF Node intermediário, SPA servida pelo backend Go e SSR — decisão: landing standalone consumindo API via CORS.
- Self-service LGPD com portabilidade automática (avaliado, adiado para v2).
- Multi-region active-active.

## Premissas e Restrições
Premissas:
- Volumetria base: 500 MAU iniciais com crescimento linear até 5k MAU em 12 meses; pico mensal de checkout na primeira semana (~60% do volume).
- Ticket médio R$29,90 (referência do brainstorming) para projeção financeira; revisão antes do v1.
- Cognito Lite/Essentials atende 500→5k MAU dentro do free tier (10k MAU gratuitos para login direto/social).
- WhatsApp Cloud API gratuito até 1k service conversations/mês, suficiente para v1.1 piloto.
- DPO disponível e política de privacidade publicada antes do v1.
- Conta AWS corporativa com owner claro e budget habilitado antes do v1.

Restrições:
- Monolito modular Go com arquitetura hexagonal preservada — sem extrair microsserviço sem necessidade comprovada.
- Outbox transacional (commit `4b7149e`) é o mecanismo canônico para side-effects pós-commit; `events.Bus` apenas para sinais voláteis (regra `AGENTS.md`).
- AWS é a nuvem-alvo única; região `sa-east-1` preferida por latência BR e LGPD.
- Segurança e isolamento cross-user são inegociáveis — qualquer regressão de autorização bloqueia release.
- Custo AWS sob teto US$150-250 (v1) e US$300-450 (v1.1) com budget alarm 80%/100%.
- Compromisso "production-proof inegociável" do usuário: nenhum atalho que sacrifique segurança, idempotência ou auditoria.

## Viabilidade Técnica
Status:
viável com restrições

Justificativa:
O codebase já oferece fundação para o módulo: hexagonal por módulo, `UnitOfWork[T]`, outbox transacional implementado, OTel SDK, runtime server/worker, Fiber e gRPC. As decisões da Rodada 1 (recorte v1 sem WhatsApp linking + AWS + RBAC tabular + landing standalone) reduzem complexidade incremental. O risco maior está concentrado em três áreas externas que precisam ser tratadas em paralelo: aprovação da Meta para Cloud API (timing v1.1), processo de retirada do sandbox do SES e estabilização do webhook Mercado Pago em ambiente real.

Bloqueadores:
- Aprovação Meta Business + Display Name para WhatsApp Cloud API (prazo típico 1-3 semanas) — bloqueia v1.1.
- Saída do sandbox do AWS SES (necessária para envio fora de e-mails verificados) — bloqueia v1 público.
- Consulta ao work item 29 do Azure DevOps (MCP bloqueado por login interativo) — risco de divergência com backlog oficial.
- Confirmação de tarifa Mercado Pago Pix atual para volume estimado — bloqueia projeção financeira final.
- DPO nomeado e política de privacidade publicada em `mecontrola.app.br` — bloqueia v1.

## Arquitetura Atual
- Monolito Go 1.26.3 modular hexagonal em `internal/{identity,finance,conversation,agent,notifications,telemetry}`. Cada módulo expõe `application/`, `domain/` e `adapters/`.
- Infraestrutura compartilhada em `internal/infrastructure/`: `database` (pgx + migrations), `outbox` (publisher transacional com dispatcher + mocks/fakes, commit `4b7149e`), `events` (bus in-process volátil, ADR-003), `runtime` (CLI cobra `api`/`worker`), `http` (Fiber), `observability` (OTel SDK + tracer/meter providers), `errors`, `clock`.
- Banco: PostgreSQL via pgx; migrations versionadas em `migrations/` (`0001_*`, `0002_outbox.up.sql`).
- Identidade já documentada como fronteira para usuário, sessão, JWT/refresh, RBAC e auditoria de acesso (README de `internal/identity`).
- Não há provisionamento Terraform versionado em produção ainda; não há integração Cognito, Mercado Pago, SES nem Cloud API; não há schema de plano/assinatura/RBAC tabular instalado.

## Arquitetura Proposta
Componentes:
- `mecontrola-landingpage` (Next ou Astro) hospedada em Vercel/Cloudflare Pages — landing pública, captura, redireciona para SPA autenticada (mesmo repo ou subdomínio) que consome API Go via CORS allowlist.
- Backend Go (este repositório):
  - Módulo `identity` estendido com casos de uso: `RegisterUser`, `LoginCallback`, `RefreshToken`, `LogoutUser`, `LinkWhatsApp` (v1.1), `AssignRole`, `EnsureOwnership`.
  - Módulo `finance` estendido com agregados `Plan`, `Subscription`, `Payment` e portas `PaymentProvider`, `BillingScheduler`, `EmailNotifier`.
  - Módulo `notifications` para SES e (v1.1) WhatsApp Cloud API; ambos atrás de portas trocáveis.
  - Camada HTTP Fiber para REST (`/v1/auth/*`, `/v1/me`, `/v1/plans`, `/v1/subscriptions`, `/v1/payments/webhook/mercadopago`, `/admin/lgpd/requests`).
  - Worker `runtime worker` consumindo outbox dispatcher (já existente) com handlers idempotentes para `payment.confirmed`, `subscription.activated`, `subscription.expired`, `whatsapp.link.requested`.
- Cognito User Pool (provisionado via Terraform) — App Client SPA + JWKS validation no backend.
- RDS Postgres 17 Multi-AZ (`sa-east-1`) — schema produto + outbox.
- SES com domain identity `mecontrola.app.br` (DKIM+SPF+DMARC) para e-mails transacionais.
- Mercado Pago como provedor externo de Pix (porta `PaymentProvider` na aplicação; adapter em `internal/finance/adapters/mercadopago`).
- KMS para chave envelope dos campos `cpf`/`whatsapp_phone` (DEK pgcrypto + KEK KMS); Secrets Manager para credenciais MP, Cognito, SES, Cloud API.
- OTel Collector sidecar em cada task Fargate → Grafana Cloud Free/Pro (Loki/Tempo/Mimir).
- ALB + AWS WAF (regras OWASP managed) na frente da API; VPC com subnets pública (ALB) / privada (Fargate) / isolada (RDS); NAT Gateway para egresso; VPC Endpoints para SES/Secrets/S3.

Fluxo de alto nível:
1. Usuário visita `mecontrola.app.br`, escolhe plano, clica em "Assinar".
2. SPA redireciona para Cognito Hosted UI (Authorization Code + PKCE) e obtém `id_token`/`access_token`/`refresh_token`.
3. SPA chama `POST /v1/subscriptions` com plano selecionado; backend cria `subscription` (`status=pending`) e via `PaymentProvider.CreatePixCharge` obtém QR Code/copia-e-cola, registra `payment` e devolve ao SPA dentro de transação que enfileira outbox `subscription.created`.
4. Usuário paga via app bancário; Mercado Pago envia webhook `payment.updated` para `POST /v1/payments/webhook/mercadopago`.
5. Handler valida assinatura HMAC do MP, deduplica por `(provider, payment_id, event_type)`, dentro de transação atualiza `payment.status=approved`, ativa `subscription` e enfileira outbox `payment.confirmed` + `subscription.activated`.
6. Dispatcher consome outbox e dispara handlers idempotentes: envio de recibo via SES, atualização de cache de RBAC, registro em `audit_log`.
7. Usuário recebe e-mail com recibo + link de status; v1.1 inclui CTA "vincular WhatsApp" gerando token de uso único.
8. Em v1.1, usuário envia "OI <CÓDIGO>" no WhatsApp; webhook Cloud API resolve o código, marca `identity.whatsapp_link` ativo e libera comandos.
9. Cron de `BillingScheduler` agenda lembrete D-3/D-1 e expiração via outbox; expiração bloqueia uso (HTTP 402 na API; mensagem de "assinatura expirada" no WhatsApp em v1.1).

Decisão arquitetural:
Manter identidade, plano, RBAC, ownership, auditoria e webhook como núcleo canônico do monolito Go, com Cognito e Mercado Pago atrás de portas de aplicação. Toda mutação que precise gerar side-effect pós-commit usa o `outbox.Publisher` na mesma transação do agregado; sinais voláteis (telemetria in-process, cache local) usam `events.Bus`. Autorização aplicada em dois pontos: middleware HTTP valida JWT + papel, e repositórios injetam filtro por `owner_user_id` impedindo bypass. Testes obrigatórios cobrem cross-user retornando 404 (não 403) para não vazar existência.

## Dados e Integrações
Domínios de dados:
- `identity`: `users`, `user_roles`, `roles`, `permissions`, `role_permissions`, `user_external_subjects` (`provider, external_subject` para Cognito sub), `audit_log`, `whatsapp_link_codes` (v1.1), `whatsapp_links` (v1.1), `lgpd_requests`.
- `finance`: `plans`, `subscriptions` (`status`, `started_at`, `current_period_end`, `cancel_requested_at`), `payments` (`provider`, `provider_payment_id`, `status`, `amount`, `paid_at`), `payment_webhook_dedup`, `billing_jobs`.
- Compartilhado (infra): `outbox_events` (existente), `outbox_handler_dedup` (`handler_name`, `event_id`), `outbox_dead_letter`.

Integrações:
- Amazon Cognito (OIDC): JWKS endpoint para validação de JWT, Hosted UI para PKCE, refresh token rotativo. Discovery via `https://cognito-idp.sa-east-1.amazonaws.com/<pool>/.well-known/openid-configuration`. Bibliotecas: `github.com/coreos/go-oidc/v3` para discovery/JWKS, `golang.org/x/oauth2` para fluxo PKCE no SPA (não usado no backend Go salvo testes).
- Mercado Pago Checkout API + Pix: `POST /v1/payments` (criação de cobrança Pix); webhook `POST /webhook/mercadopago` (notificações `payment` topic). Assinatura validada por `x-signature` (HMAC SHA-256 com segredo do Webhook + manifest).
- AWS SES: `SendTemplatedEmail` para templates versionados; consumo de bounce/complaint via SNS→SQS→handler.
- WhatsApp Cloud API (v1.1): Graph API `messages` endpoint; webhook `messages` topic com validação `X-Hub-Signature-256`.
- AWS KMS: `Decrypt`/`GenerateDataKey` para DEK pgcrypto.
- AWS Secrets Manager: leitura on-demand cacheada (TTL 5min) para credenciais externas.

Consistência requerida:
híbrida — consistência forte transacional para identidade/plano/assinatura/payment dentro do Postgres (Serializable Snapshot Isolation onde necessário). Consistência eventual com at-least-once para side-effects via outbox (e-mail, WhatsApp, dashboards). Webhooks externos tratados como at-least-once e deduplicados antes de qualquer mutação.

## Volumetria e Capacidade
Volume atual:
0 usuários em produção; landing publicada sem backend operante para cadastro/checkout.

Pico esperado:
v1: ~500 MAU com pico de 300 checkouts concentrados na primeira semana do mês (~50/dia, ~10/hora pico). Webhook MP: ~1 RPS pico, ~0,1 RPS média. v1.1: até 5k MAU, ~3k mensagens WhatsApp/dia agregadas, webhook Cloud API ~5 RPS pico.

Taxa de crescimento:
Crescimento linear modelado de 500→5k MAU em 12 meses (+375/mês). Cobranças mensais escalam proporcionalmente até ~3k/mês no mês 12.

SLO alvo:
- API REST: p95 < 300ms, disponibilidade 99.5% mensal (≤ 3,6h de downtime/mês).
- Webhook MP: ack < 5s p95, processamento end-to-end (webhook→assinatura ativa) < 60s p95.
- Worker outbox: lag entre `created_at` e `processed_at` < 30s p95.
- E-mail transacional: entregue ao SES em < 10s p95 após enfileiramento.
- RTO 1h (failover Multi-AZ + redeploy); RPO 5min (backup automatizado RDS + WAL).

Gargalos conhecidos:
- NAT Gateway: limite ~5Gbps e custo por GB de saída; mitigação com VPC Endpoints para SES, Secrets Manager, S3.
- Pool de conexões Postgres no Fargate: dimensionar com pgbouncer-style (`github.com/jackc/pgx/v5/pgxpool`) e limitar a 20 conexões por task; preparar para reuso após failover.
- Webhook MP em pico de 1ª semana do mês: garantir auto-scaling do serviço API por CPU/req.
- Outbox dispatcher single-instance no v1: aceitar throughput de até 50 eventos/s; ativar lock advisory para evitar double processing se escalar para 2+ workers.

## Segurança e Compliance
Classificação dos dados:
- Críticos (PII regulada): CPF, telefone WhatsApp.
- Sensíveis (PII identificadora): nome completo, e-mail, endereço futuro.
- Financeiros (retenção fiscal 5 anos): comprovantes Pix, valores cobrados, datas.
- Operacionais: tokens JWT (apenas em memória/headers), logs sanitizados.
- Não regulado: telemetria agregada sem PII.

Autenticação e autorização:
- AuthN: Cognito User Pool com Authorization Code + PKCE; refresh rotativo; tokens em memória na SPA; backend valida JWT por JWKS com checagem de issuer, audience, expiração, nonce e kid.
- AuthZ: RBAC tabular (`roles`/`permissions`/`role_permissions`/`user_roles`); papéis iniciais `user` e `admin`. Toda permissão é uma constante (`subscription:read`, `subscription:cancel`, `lgpd:request:write`, `admin:audit:read`). Middleware Fiber resolve papéis do usuário (cache 60s) e bloqueia chamadas não autorizadas com `403`. Ownership enforced no repositório: toda query de recurso aceita `owner_user_id` como parâmetro obrigatório; tentativa cross-user retorna `not found` (`404`) para não vazar existência.
- Teste obrigatório: tabela de casos cross-user (usuário A tenta acessar recurso de B) deve produzir 404 em todos os endpoints; falha bloqueia merge.

Gestão de segredos:
- Secrets Manager para credenciais Mercado Pago (App Token, Webhook Secret), Cognito Client Secret, Cloud API System User Token, chaves KMS aliases.
- Rotação obrigatória anual para credenciais MP e Cloud API; rotação semestral para chaves DEK pgcrypto via re-encrypt job.
- Backend lê via SDK AWS com cache TTL 5min e refresh em background; nenhum segredo em variáveis de ambiente persistentes ou imagem.

Criptografia:
- Em trânsito: TLS 1.2+ em ALB, RDS (`rds.force_ssl=1`), todas as integrações externas (Cognito, MP, Cloud API, SES).
- Em repouso: RDS encryption at rest com KMS CMK por ambiente; S3 bucket de logs com SSE-KMS.
- Em coluna: `cpf` e `whatsapp_phone` armazenados como `bytea` cifrados via `pgp_sym_encrypt(value, dek)`; DEK derivada por envelope com KMS `Decrypt` cacheada em memória durante o request; hashes `cpf_hash`/`phone_hash` em HMAC-SHA256 com pepper em KMS para lookups exatos.

Auditoria e rastreabilidade:
- Tabela `audit_log` particionada por mês com colunas `actor_user_id`, `target_user_id`, `event_type`, `resource_id`, `metadata_json`, `request_id`, `created_at`.
- Eventos auditados: login bem-sucedido/falho, criação de cobrança, ativação/expiração de assinatura, atribuição/revogação de papel, leitura administrativa de dados, acesso a `lgpd_requests`, vinculação WhatsApp.
- Correlação por `request_id` propagado em headers e via context.Context para traces OTel.
- Retenção 12 meses on-line + 5 anos em S3 Glacier para dados fiscais.

Compliance/LGPD:
- Política de privacidade publicada em `mecontrola.app.br/privacidade` antes do v1 (bloqueador).
- DPO nomeado com canal de contato exposto.
- Bases legais: execução de contrato (assinatura), cumprimento de obrigação legal fiscal (5 anos).
- Direitos do titular: endpoint admin `/admin/lgpd/requests` para registro de solicitação (acesso, correção, exclusão, portabilidade); SLA interno 15 dias úteis; fluxo manual no v1; self-service planejado para v2.
- Soft-delete na cancelação (`deactivated_at`); hard-delete de PII não-fiscal 30 dias após solicitação aprovada, com registro auditável em `audit_log`.
- Logs e métricas sanitizados por interceptor central que mascara CPF, telefone, e-mail e tokens.
- WAF + rate limiting por IP/usuário em `/v1/auth/*` e webhooks para mitigar abuso.

## Confiabilidade e Resiliência
SLA/SLO:
- API REST: 99.5% disponibilidade mensal, p95 < 300ms.
- Webhook MP: 99.9% de ack em < 5s p95; processamento end-to-end < 60s p95.
- Worker outbox: lag entre `created_at` do evento e `processed_at` do handler < 30s p95.
- SLO de erro de RBAC (cross-user): 0 — qualquer violação detectada gera incidente P1.

RTO/RPO:
- RTO: 1h. Coberto por RDS Multi-AZ (failover ~60-120s), ECS auto-recovery, imagens Docker imutáveis em ECR com rollback declarativo por Terraform.
- RPO: 5min. Coberto por backup automatizado RDS (snapshot diário + retenção 7 dias) e WAL contínuo. Restore via Point-in-Time Recovery.

Estratégia de retry/idempotência:
- `outbox.Publisher` na mesma transação do agregado garante at-least-once; handlers idempotentes por `event_id` em `outbox_handler_dedup`.
- Webhook MP deduplicado por `(provider='mercadopago', provider_payment_id, event_type)` antes de qualquer mutação; resposta sempre `200` após persistência do evento, processamento assíncrono por outbox.
- Retry exponencial com jitter no dispatcher: 1s→2s→4s→…→cap 5min; após 10 tentativas, evento migra para `outbox_dead_letter` e dispara alerta P2.
- Chamadas externas com circuit breaker leve por endpoint (Cognito JWKS, MP API, SES, Cloud API) usando contagem de erros + janela deslizante.

Degradação/contingência:
- Falha de Cognito (validação JWT): API responde 503 com `Retry-After` para chamadas autenticadas; cache local de JWKS válido por 1h para tolerar instabilidade pontual.
- Falha de SES: lembretes/recibos enfileirados em outbox; retry contínuo; alerta após 10min de fila acumulada.
- Falha de MP API na criação de cobrança: SPA mostra mensagem "tente novamente" e log estruturado; nenhuma transação parcial.
- Falha de webhook MP: dedup garante reprocessamento; reconciliação periódica via job `BillingReconciler` que compara `subscriptions.status` com MP a cada 1h para fechar lacunas.
- Falha de RDS: failover Multi-AZ automático; pool reconectado via pgxpool com backoff.

Rollback:
- Imagens versionadas em ECR; deploy declarativo via Terraform + GitHub Actions; rollback por reapontar `task_definition` para revisão anterior.
- Migrations `golang-migrate` aplicadas com `up`/`down` versionados; toda migration testada com rollback em ambiente de homologação antes de produção.
- Feature flag por header/cabeçalho admin para desligar checkout em incidente sem novo deploy.
- Critério de rollback automático: falha de health check 3 vezes seguidas, ou erro 5xx > 5% por 5 minutos consecutivos, ou alarme P1 de cross-user.

## Observabilidade e Operação
Métricas:
- `auth_login_total{result}` (sucesso/falha por tipo).
- `subscription_created_total{plan}`, `subscription_activated_total{plan}`, `subscription_expired_total{plan}`, `subscription_blocked_total{reason}`.
- `payment_webhook_received_total{provider,event_type}`, `payment_webhook_duplicate_total{provider}`, `payment_webhook_processing_seconds{provider}` (histograma).
- `outbox_lag_seconds` (histograma), `outbox_handler_retry_total{handler,result}`, `outbox_dead_letter_total{handler}`.
- `rbac_denied_total{permission,reason}` — `reason=cross_user_attempt` gera alerta P1.
- `email_sent_total{template,result}`, `whatsapp_message_sent_total{template,result}` (v1.1).
- `http_request_duration_seconds{route,method,status}` (já disponível no padrão Fiber+OTel).

Logs:
- Logs estruturados JSON via SDK existente; campos obrigatórios `request_id`, `trace_id`, `span_id`, `user_id` (quando autenticado), `subscription_id`/`payment_id`/`webhook_event_id` (quando aplicável).
- Interceptor central de redação de CPF, telefone, e-mail e tokens.
- Categorias: `auth`, `billing`, `webhook`, `rbac`, `outbox`, `audit`, `infra`. Retenção 14 dias (free)/30 dias (pago) em Loki.

Traces:
- Trace por requisição HTTP com propagação W3C `traceparent`; spans para chamadas externas (Cognito, MP, SES, KMS, RDS); span dedicado ao processamento de webhook MP cobrindo dedup → transação → outbox.
- Trace por job do worker outbox, com span por handler e por chamada externa.
- Retenção 7 dias em Tempo.

Alertas:
- `payment_webhook_processing_seconds p95 > 5s` por 10min — P2.
- `outbox_lag_seconds p95 > 60s` por 15min — P2.
- `outbox_dead_letter_total > 0` na janela 5min — P1.
- `rbac_denied_total{reason="cross_user_attempt"} > 0` — P1 imediato.
- API 5xx > 5% por 5min — P1.
- SES bounce rate > 5% ou complaint rate > 0.1% — P2.
- Cognito 4xx/5xx > 2% por 5min — P2 (cache JWKS protege parcialmente).

Dashboards/Runbooks:
- Dashboard "Funil Aquisição" (cadastro → plano → cobrança → ativação → vinculação WhatsApp v1.1) em `deploy/grafana/dashboards/aquisicao.json`.
- Dashboard "SLO API" e "SLO Webhook MP" com burn-rate.
- Runbooks versionados em `docs/runbooks/`: `webhook-mercadopago.md`, `outbox-dead-letter.md`, `rbac-cross-user-incidente.md`, `failover-rds.md`, `ses-bounce.md`, `cognito-degradado.md`.
- Painel `Admin LGPD` com lista de solicitações pendentes e ação manual.

## Performance e Escalabilidade
Latência alvo:
- API REST autenticada p50 < 80ms / p95 < 300ms.
- `POST /v1/subscriptions` (cria Pix) p95 < 1.5s incluindo round-trip MP.
- Webhook MP p95 < 1s para ack (apenas persistir evento) + processamento outbox < 60s p95.

Estratégia de escala:
- Auto-scaling Fargate por target tracking CPU 60% + ALB request count por target.
- RDS começa em `db.t4g.micro` Multi-AZ; upgrade para `db.t4g.small` planejado em ~2k MAU; leitura sob carga eventual via read replica em `db.t4g.small` se p95 deteriorar.
- Outbox dispatcher: começa com 1 worker; ativa lock advisory por `event_id` se escalar para N workers.
- VPC Endpoints para SES/Secrets/S3 reduzem tráfego NAT e latência.

Limites conhecidos:
- Throughput de NAT Gateway compartilhado e custo por GB egress.
- Limite de conexões Postgres (max_connections ~100 no `t4g.micro`); reservar 80% para API + 20% worker.
- Cota inicial Cloud API gratuita (1k service conversations/mês) — válido apenas para v1.1 piloto.
- Cota inicial SES sandbox (200 e-mails/dia, recipientes verificados) — bloqueio v1 até saída de sandbox.

Teste de carga:
- Antes do v1: cenário k6/Vegeta cobrindo cadastro/login + criação de cobrança Pix em sandbox MP + webhook simulado, com 50 RPS sustentado por 10min validando p95 < 300ms e zero erro de outbox.
- Cenário cross-user (usuário A tentando acessar B) automatizado no CI antes de merge.
- Cenário burst de webhooks (200 webhooks em 1min) validando dedup, idempotência e ack < 5s p95.

## Custos e Orçamento
Orçamento estimado:
- v1 (sem WhatsApp linking): US$150-250/mês. Orçamento alvo US$200/mês com alerta em 80% (US$160) e 100% (US$200).
- v1.1 (com WhatsApp linking): US$300-450/mês. Orçamento alvo US$400/mês com alertas equivalentes.

Drivers de custo:
- RDS Postgres `db.t4g.micro` Multi-AZ: ~US$50/mês.
- ALB: ~US$22/mês fixo + LCU (~US$5-10).
- NAT Gateway: ~US$33/mês fixo + tráfego (~US$10-30 conforme uso).
- ECS Fargate: ~US$45/mês (1vCPU/2GB API contínuo + 0.5vCPU/1GB worker).
- Mercado Pago Pix: tarifa por pagamento confirmado (revalidar tabela vigente antes do v1).
- SES: ~US$0,10/1k e-mails (volume baixo no v1, < US$5/mês).
- Cognito Lite/Essentials: US$0 dentro de 10k MAU.
- Grafana Cloud Free: US$0 (3 usuários, 10k séries/14d logs); migração para Pro (~US$8/usuário/mês + ingest) prevista em v1.1.
- Cloud API Meta: US$0 nos primeiros 1k service conversations/mês; estimado < US$50/mês em v1.1 piloto.
- KMS: US$1/CMK/mês × 2 + US$0.03/10k requests.
- Secrets Manager: US$0.40/segredo/mês × ~6 segredos = US$2.40/mês.

Guardrails de custo:
- AWS Budget com alertas 80%/100% por categoria (compute, db, network, observability).
- Tag obrigatória `app=mecontrola, env=<env>, component=<...>` em todo recurso Terraform para cost allocation.
- Quota por Cloud API: review semanal; throttling no `notifications` adapter para evitar surto não esperado.
- Cost Explorer review mensal pelo owner técnico.

Plano de otimização:
- Substituir NAT Gateway por VPC Endpoints (SES, Secrets, S3, ECR, CloudWatch Logs) para reduzir tráfego NAT — economia esperada de US$15-25/mês.
- Avaliar Fargate Spot para worker outbox (~70% de economia) após validar que o sistema tolera interrupções (já tolera por contrato outbox+idempotência).
- Consolidar logs noisy via OTel Collector processor `filter` para reduzir ingest Loki.
- Reavaliar Multi-AZ a cada 6 meses contra SLO real.

## Riscos e Mitigações
- Risco: Vazamento cross-user por falha de filtro de ownership ou regressão em RBAC.
  Impacto: Crítico — perda de confiança, exposição financeira, possível notificação ANPD.
  Mitigação: middleware central + scoping no repositório + suíte de testes de autorização cobrindo cross-user com 404 em todos endpoints, automatizada no CI; métrica `rbac_denied_total{reason="cross_user_attempt"}` com alerta P1; revisão de cada PR que toque domínio sensível.
  Dono: Tech Lead do backend.

- Risco: Webhook Mercado Pago duplicado ou perdido gerando ativação/bloqueio incorreto.
  Impacto: Alto — cobrança não reconhecida, usuário pagante sem acesso, suporte manual.
  Mitigação: dedup por `(provider, provider_payment_id, event_type)`, outbox transacional, retry exponencial, dead-letter alarmado, reconciliação periódica `BillingReconciler` contra API MP.
  Dono: Tech Lead do backend.

- Risco: Lock-in em Cognito ou Mercado Pago.
  Impacto: Médio-Alto — custo de migração futura, mudança regulatória.
  Mitigação: portas `IdentityProvider`/`PaymentProvider` com adapters isolados; armazenar `external_subject` separado de `user_id`; manter plano/RBAC/ownership como dado próprio; documentar contratos.
  Dono: Tech Lead do backend.

- Risco: Atraso na aprovação Meta Cloud API impede v1.1.
  Impacto: Médio — quebra cronograma de divulgação pública.
  Mitigação: iniciar onboarding Meta Business + Display Name em paralelo ao v1; manter banner "ativação WhatsApp em breve" na landing; preparar fallback de envio via Twilio temporário documentado, ativável por adapter swap se aprovação demorar > 60 dias.
  Dono: Produto + Operações.

- Risco: Saída do sandbox SES não concluída até o v1.
  Impacto: Médio — bloqueia recibo Pix e lembretes para usuários reais.
  Mitigação: abrir ticket AWS imediatamente; estabilizar reputação com envios verificados antes; ter Resend como fallback adapter no `notifications` em emergência.
  Dono: Operações.

- Risco: Free tier (Cognito 10k MAU, Cloud API 1k conversations) consumido sem aviso.
  Impacto: Médio — custo surpresa, possível suspensão de serviço.
  Mitigação: budget alarm por MAU/conversation count em CloudWatch + Cost Explorer; revisão mensal; plano pós-free-tier documentado em runbook.
  Dono: Operações.

- Risco: Work item 29 contém requisitos não capturados nesta descoberta.
  Impacto: Médio — possível divergência com backlog oficial.
  Mitigação: prioridade alta para destravar MCP Azure DevOps via login interativo; reuniões de alinhamento documentadas; reavaliar escopo antes de iniciar Epic 01.
  Dono: Produto.

- Risco: Pix avulso gera churn maior que o esperado por exigir ação manual mensal.
  Impacto: Médio — receita projetada não se realiza.
  Mitigação: lembretes D-3/D-1; mensagem clara de "expira em X dias" via WhatsApp em v1.1; plano de migração para Pix Automático/recorrente reavaliado em 6 meses após v1.1.
  Dono: Produto.

## Trade-offs e Decisões
Alternativas consideradas:
- Recorte v1: A (núcleo), B (núcleo + RBAC + WhatsApp linking), C (B + onboarding conversacional), D (tudo + Pix recorrente). Escolhida variação ajustada: núcleo + RBAC + ownership + auditoria sem WhatsApp linking no v1; linking entra como v1.1 gating obrigatório.
- Hospedagem: AWS / Azure / GCP / PaaS. Escolhida AWS por Cognito free tier + maturidade + Terraform.
- RBAC: roles hardcoded / RBAC tabular / RBAC+ABAC / OPA. Escolhido RBAC tabular + ownership.
- Camada web: landing standalone / SPA servida pelo Go / BFF Node / SSR Next. Escolhido landing standalone com CORS.
- Cobrança: Pix avulso / Pix automático / Checkout Pro hospedado / híbrido. Escolhido Pix avulso com `PaymentProvider` como porta para evolução.
- Compute: Fargate / App Runner / Spot worker / Lambda. Escolhido Fargate API + worker.
- Volumetria/SLO: enxuto / base 500→5k / agressivo 50k / sem definição. Escolhido base 500→5k com SLO 99.5%.
- PII: TLS+SSE padrão / pgcrypto coluna / envelope encryption / DAS auditoria. Escolhido pgcrypto + KMS.
- Provedor WhatsApp: Cloud API Meta / Twilio / 360dialog / Z-API. Escolhido Cloud API oficial Meta.
- Gating release: v1 web + v1.1 WhatsApp / v1 com linking / v1 público + "em breve" / waitlist paga. Escolhido v1 beta fechado + v1.1 antes de divulgação.
- Resiliência: Multi-AZ + outbox / single-AZ / Multi-AZ + SQS DLQ / multi-region. Escolhido Multi-AZ + outbox + retry + dedupe.
- Observabilidade: CloudWatch / OTel→Grafana Cloud / self-hosted / Datadog. Escolhido OTel→Grafana Cloud.
- E-mail: SES / Resend / SendGrid / só Cognito. Escolhido SES.
- Sessão: PKCE + memória / cookie httpOnly+BFF / localStorage / Hosted UI redirect. Escolhido PKCE + tokens em memória + refresh rotativo.
- LGPD: 5 anos fiscal + exclusão manual / self-service v1 / sem fluxo / minimização extrema. Escolhido 5 anos fiscal + exclusão manual.
- Orçamento: US$80-150 / US$150-250 / US$400-700 / sem teto. Escolhido US$150-250 v1 / US$300-450 v1.1.

Decisão tomada:
Executar Alternativa 5 no perfil técnico consolidado nesta descoberta: v1 beta fechado com Cognito+Pix+RBAC+Auditoria em AWS Multi-AZ; v1.1 com WhatsApp Cloud API antes de divulgação pública; tudo provisionado por Terraform; observabilidade OTel→Grafana Cloud; LGPD com retenção fiscal e exclusão manual.

Trade-off aceito:
- Dependência externa em Cognito e Mercado Pago, mitigada por portas de aplicação trocáveis.
- Churn maior em Pix avulso em troca de menor risco operacional de Pix automático.
- Divulgação pública atrasada para v1.1 em troca de cumprir compromisso "production-proof inegociável".
- NAT Gateway como custo fixo no v1, mitigado por roadmap de VPC Endpoints.
- Exclusão LGPD manual no v1, com plano de self-service em v2.

## Plano de Entrega e Rollout
Fases:
- Fase 0 (paralela ao v1) — Onboarding Meta Business + Display Name + saída sandbox SES + nomeação DPO + publicação de política de privacidade. Bloqueia v1 público.
- Fase 1 (v1) — Bootstrap AWS via Terraform; identidade OIDC Cognito; planos e cobrança Pix avulso; RBAC tabular + ownership; auditoria; observabilidade; LGPD endpoint admin; testes de carga e cross-user; release beta fechado.
- Fase 2 (v1.1) — WhatsApp Cloud API; vinculação por código temporário; bloqueio de comandos pós-expiração via canal WhatsApp; release pública após gates verdes.
- Fase 3 (v2 — pós-divulgação) — Self-service LGPD; revisão de Pix recorrente; SSO corporativo se demanda surgir; read replica RDS; otimização NAT via VPC Endpoints.

Migração:
- Sem usuários existentes; sem dados legados a migrar.
- Migration `golang-migrate` versionada; cada PR de schema acompanha `up` e `down` testados em homologação.
- Dados de planos populados via seed Terraform/migration; ajustes operacionais via console admin.

Feature flags/canary:
- Flag por configuração (Secrets Manager) para habilitar/desabilitar checkout em emergência.
- Webhook MP separado por path versionado (`/v1/payments/webhook/mercadopago`) para evolução não-disruptiva.
- v1.1 com flag de "WhatsApp linking enabled" por usuário, permitindo piloto com lista controlada antes de release amplo.

Critério de rollback:
- Falha de health check Fargate por 3 vezes seguidas reverte para revisão de task definition anterior automaticamente.
- Erro 5xx > 5% por 5min consecutivos ou alarme P1 cross-user → rollback manual coordenado por on-call.
- Migration com `down` testado obrigatório; rollback de schema documentado em runbook.
- Para v1.1: se Cloud API atingir 5% error rate por 15min, desabilitar flag de linking e operar apenas com canal web; retomar quando estabilizado.

## Decomposição em Épicos e Features
### Epic 01 - Bootstrap de infraestrutura AWS via Terraform
Objetivo: Provisionar fundação AWS reproduzível com Cognito, VPC, RDS Multi-AZ, ECS Fargate, ALB+WAF, SES, KMS, Secrets Manager, IAM, com tags de cost allocation e budgets configurados.
Feature 01: Módulo Terraform `iam` com roles mínimas para Fargate task, execution e admin Terraform.
Feature 02: Módulo Terraform `networking` (VPC, subnets pública/privada/isolada, NAT, route tables).
Feature 03: Módulo Terraform `data` (RDS Postgres 17 Multi-AZ + parameter group + backup + KMS CMK).
Feature 04: Módulo Terraform `identity` (Cognito User Pool, App Client SPA com PKCE, JWKS endpoint, domínio customizado).
Feature 05: Módulo Terraform `compute` (ECS cluster, task definitions API e worker, service, ALB, WAF, autoscaling).
Feature 06: Módulo Terraform `notifications` (SES domain identity, DKIM, SNS+SQS para bounce/complaint).
Feature 07: Módulo Terraform `secrets` (Secrets Manager entries + KMS aliases para credenciais e DEK pgcrypto).
Feature 08: Pipeline GitHub Actions de plan/apply com OIDC federation + budgets + cost allocation tags obrigatórias.

### Epic 02 - Identidade e autenticação OIDC
Objetivo: Implementar cadastro/login/logout/refresh via Cognito Authorization Code + PKCE com validação JWT robusta no backend Go e correlação para usuário interno canônico.
Feature 01: Casos de uso `RegisterUser`, `LoginCallback`, `LogoutUser`, `RefreshToken` na camada `application` de `internal/identity`.
Feature 02: Adapter `cognito` em `internal/identity/adapters/cognito` implementando porta `IdentityProvider`.
Feature 03: Middleware Fiber `AuthenticateJWT` com cache JWKS (`go-oidc/v3`) e propagação de `AuthenticatedUser` via `context.Context`.
Feature 04: Tabela `user_external_subjects` e migration correspondente para correlação `(provider, external_subject) → user_id`.
Feature 05: Testes de integração com Cognito local stub + cenários de token inválido/expirado/issuer divergente.

### Epic 03 - Planos, assinaturas e cobrança Pix
Objetivo: Permitir compra de plano, criação de cobrança Pix avulsa via Mercado Pago, ativação por webhook idempotente e bloqueio por expiração.
Feature 01: Agregados `Plan`, `Subscription`, `Payment` no domínio `finance` com regras de transição de estado.
Feature 02: Porta `PaymentProvider` em `application` e adapter `mercadopago` em `adapters/mercadopago` cobrindo criação de cobrança + verificação de assinatura HMAC.
Feature 03: Endpoint `POST /v1/subscriptions` com fluxo transacional (subscription pending + payment + outbox `subscription.created`).
Feature 04: Endpoint `POST /v1/payments/webhook/mercadopago` com dedup `(provider, payment_id, event_type)` e enfileiramento de `payment.confirmed` + `subscription.activated`.
Feature 05: `BillingScheduler` (worker cron) para criação de cobrança no início de cada período + lembretes D-3/D-1 + expiração via outbox.
Feature 06: Job `BillingReconciler` para varredura horária comparando estado MP × banco e fechando lacunas.

### Epic 04 - RBAC tabular e ownership por owner_user_id
Objetivo: Garantir que todo recurso sensível seja acessado apenas pelo dono, com papéis tabulares evoluíveis sem deploy e suíte de testes anti-cross-user.
Feature 01: Migrations e modelos `roles`, `permissions`, `role_permissions`, `user_roles` com seeds para `user` e `admin`.
Feature 02: Casos de uso `AssignRole`, `RevokeRole`, `GrantPermission` com auditoria.
Feature 03: Middleware `AuthorizePermission(perm)` resolvendo papéis do `AuthenticatedUser` (cache 60s) e bloqueando com 403.
Feature 04: Padrão de repositório `Scoped(ownerUserID)` para todos os módulos sensíveis, retornando 404 (`not found`) em tentativa cross-user.
Feature 05: Suíte de testes `tests/security/cross_user_test.go` automatizada no CI cobrindo cada endpoint sensível com pares de usuários (A acessando B).

### Epic 05 - Auditoria, LGPD e observabilidade
Objetivo: Garantir trilha auditável de eventos sensíveis, fluxo LGPD para titulares e observabilidade OTel→Grafana Cloud com SLO e alertas.
Feature 01: Tabela `audit_log` particionada por mês + repositório `AuditLogger` com redação de PII.
Feature 02: Hooks de auditoria em login, alteração de plano, atribuição de papel, leitura administrativa e acesso a `lgpd_requests`.
Feature 03: Endpoint admin `/admin/lgpd/requests` (POST/GET/PATCH) com transições `received → approved → executed` e auditoria.
Feature 04: Interceptor central de redação para logs/metrics e propagação de `request_id`/`trace_id`.
Feature 05: Configuração OTel Collector sidecar + dashboards Grafana versionados em `deploy/grafana/` (skill `otel-grafana-dashboards`).
Feature 06: Alertas Grafana com burn-rate SLO + cross-user P1 + outbox dead-letter P1.

### Epic 06 - Notificações por e-mail (SES)
Objetivo: Entregar e-mails transacionais (recibo Pix, lembrete D-3, lembrete D-1, expiração, reativação, status LGPD) com idempotência via outbox.
Feature 01: Porta `EmailNotifier` em `application` e adapter SES em `notifications/adapters/ses`.
Feature 02: Templates versionados em `deploy/ses-templates/` + script de upload no Terraform.
Feature 03: Handler outbox `EmailDispatchHandler` idempotente por `event_id`, com tratamento de bounce/complaint via SNS→SQS.
Feature 04: Métricas `email_sent_total{template,result}` e dashboard "Funil Aquisição".

### Epic 07 - Vinculação WhatsApp via Cloud API (v1.1)
Objetivo: Permitir que usuário ativo vincule número de WhatsApp por código temporário com auditoria, sem login por senha no canal.
Feature 01: Adapter `whatsapp` em `notifications/adapters/whatsapp` implementando porta `WhatsAppGateway` (Cloud API Meta).
Feature 02: Casos de uso `RequestWhatsAppLinkCode` (gera código, expira em 10min, salva em `whatsapp_link_codes`) e `ConfirmWhatsAppLink` (consome código via webhook Cloud API).
Feature 03: Endpoint `POST /v1/payments/webhook/whatsapp` validado por `X-Hub-Signature-256` com dedup por `webhook_event_id`.
Feature 04: Bloqueio de comandos via WhatsApp quando `subscription.status != active` com mensagem "assinatura expirada — renove em <link>".
Feature 05: Auditoria de vinculação, desvinculação e bloqueio em `audit_log`.

## Itens em Aberto
- Consulta ao work item 29 do Azure DevOps assim que o MCP voltar a aceitar login interativo; cross-check com escopo desta descoberta.
- Confirmação da tabela vigente de tarifas Mercado Pago Pix antes do v1 para projeção financeira final.
- Nomeação formal do DPO e publicação da política de privacidade em `mecontrola.app.br/privacidade` antes do v1.
- Decisão sobre subdomínio da SPA autenticada (`app.mecontrola.app.br` recomendado) versus reaproveitar `www.mecontrola.app.br` com rotas protegidas.
- Avaliação se o adapter Twilio precisa ser implementado como fallback no v1.1 caso aprovação Meta atrase > 60 dias.
- Decisão de exportar logs `audit_log` para S3 imutável (Glacier) com versionamento como reforço de auditoria.
- Aprovação do orçamento Grafana Cloud Pro caso volume ultrapasse o Free tier antes do esperado.
