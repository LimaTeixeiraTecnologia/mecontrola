# Transcript do Discovery Técnico

## Contexto Inicial

- Origem: bundle de brainstorming `docs/discoveries/brainstorm-modulo-de-identidade-assinaturas-e-rbac` (status `done`, Alternativa 5 aprovada explicitamente pelo usuário: "documentar e seguir com esse plano, 100% robusto, eficiente, economico, production-ready/proof de forma inegociável").
- Alternativa selecionada: Híbrida em fases com contratos fortes e Mercado Pago Pix no MVP. Identidade interna, RBAC, ownership, plano, auditoria e webhooks permanecem no monolito Go canônico; OIDC gerenciado cobre autenticação; Mercado Pago Pix cobre pagamento inicial; WhatsApp é o canal principal de uso após ativação.
- Codebase observado: monolito Go modular hexagonal em `internal/{identity,finance,conversation,agent,notifications,telemetry}`; infraestrutura existente em `internal/infrastructure/{database,outbox,events,runtime,http,observability,errors,clock}`; Go 1.26.3; stack Fiber + gRPC + PostgreSQL + OpenTelemetry; outbox transacional já implementado (commit `4b7149e feat(outbox): implemente fundacao transacional de eventos`).
- Decisões pré-existentes do brainstorming (não rediscutidas, apenas detalhadas tecnicamente):
  - D1: Alternativa 5 (híbrida em fases).
  - D2: `https://www.mecontrola.app.br/` como camada de aquisição/cadastro/checkout/status mínimo.
  - D3: Backend Go canônico para identidade interna, assinatura, RBAC, ownership, auditoria, onboarding, autorização.
  - D4: WhatsApp 100% para uso diário após pagamento, ativação e vinculação segura.
  - D5: Mercado Pago Pix como PSP/meio inicial; webhook idempotente, reconciliação, bloqueio/desbloqueio por status da assinatura.
  - D6: OIDC gerenciado para autenticação (Cognito ou Entra External ID conforme estratégia de nuvem); RBAC crítico no MeControla.
  - D7: Vinculação WhatsApp por token/código temporário com expiração e auditoria — sem login por senha dentro do WhatsApp.
- Decisões pendentes herdadas do brainstorming (entrada deste discovery):
  - Escolher explicitamente o IdP OIDC (Cognito, Entra External ID, Firebase/Auth).
  - Confirmar planos finais, ticket e regra de recorrência Pix no Mercado Pago.
  - Confirmar provedor/API oficial de WhatsApp e limites operacionais.
  - Definir se o site é landing estática + backend separado ou aplicação web integrada.
  - Consultar work item 29 do Azure DevOps (bloqueado por login interativo do MCP).
- Restrições inegociáveis confirmadas: segurança e isolamento cross-user, idempotência de webhooks, auditoria, monetização recorrente, monolito modular Go com fronteiras hexagonais preservadas, outbox para side-effects críticos pós-commit.

## Materiais de Apoio

- `docs/discoveries/brainstorm-modulo-de-identidade-assinaturas-e-rbac/decision-brief.md`
- `docs/discoveries/brainstorm-modulo-de-identidade-assinaturas-e-rbac/option-scorecard.md`
- `docs/discoveries/brainstorm-modulo-de-identidade-assinaturas-e-rbac/assumptions.md`
- `docs/discoveries/brainstorm-modulo-de-identidade-assinaturas-e-rbac/transcript.md`
- `AGENTS.md` (regras transversais, contrato outbox vs events.Bus).
- `internal/identity/README.md`, `internal/finance/README.md` (a serem consultados conforme rodadas evoluírem).
- Landing pública em `https://www.mecontrola.app.br/` (referência de proposta de valor e planos).
- Work item 29 Azure DevOps — **ausente nesta sessão por bloqueio de login interativo no MCP**. Risco registrado em itens em aberto.
- Site da landing: repositório separado `https://github.com/LimaTeixeiraTecnologia/mecontrola-landingpage` (declarado pelo usuário na Rodada 1). APIs de cadastro/onboarding/checkout permanecem neste backend Go.

## Rodada 1 - Recorte técnico, hospedagem, autorização e camada web

- Pergunta 1: Qual é o recorte técnico da primeira release em produção?
  - Resposta do usuário: "Site em outro repo: https://github.com/LimaTeixeiraTecnologia/mecontrola-landingpage, mas as APIs de cadastro/onboarding deve ser construida aqui no backend, OIDC (utilizar em deployment terraform para criar cognito), pix + bloqueio".
  - Interpretação operacional: recorte v1 cobre site (landing externa) + APIs de cadastro/onboarding/auth aqui no backend Go + OIDC com Amazon Cognito provisionado por Terraform + checkout Pix + ativação/bloqueio por status de assinatura. Vinculação WhatsApp e onboarding conversacional **ficam para fases posteriores** salvo correção na Rodada 2. Risco registrado: o produto promete WhatsApp 100% — exigir confirmação explícita do gating de "produto pronto" antes de release pública.
- Pergunta 2: Qual é a hospedagem-alvo do backend Go e do worker?
  - Resposta do usuário: AWS (Cognito + ECS/Fargate ou App Runner).
  - Implicação prática: AWS é restrição estratégica de primeira classe. Cognito é o IdP OIDC. Postgres = RDS. Worker e API em Fargate ou App Runner (decidir na Rodada 2). SES viável para e-mail transacional. CloudWatch + OTel exporter para AMP/X-Ray ou Grafana Cloud.
- Pergunta 3: Qual é o modelo de autorização da v1?
  - Resposta do usuário: RBAC tabular (roles + permissions persistidas) + ownership por owner_user_id (Recomendado).
  - Implicação prática: tabelas `roles`, `permissions`, `role_permissions`, `user_roles`; toda query de recurso sensível filtra por `owner_user_id`; policy checada em middleware na entrada (porta de aplicação) **e** scoping enforced no repositório. Teste obrigatório: tentativa cross-user retorna 404 (não 403 informativo), com auditoria.
- Pergunta 4: Como tratar a camada web?
  - Resposta do usuário: Landing estática (Next/Astro) standalone consumindo API Go via CORS (Recomendado).
  - Implicação prática: landing hospedada à parte (Vercel/Cloudflare Pages); backend Go expõe API REST autenticada por JWT Cognito; CORS allowlist explícita (origem do site); tokens via Authorization Code + PKCE no browser; refresh com rotação; CSP estrita.

## Rodada 2 - Cobrança, compute, volumetria, proteção de dados

- Pergunta 1: Qual é o modelo de cobrança no v1?
  - Resposta do usuário: Pix avulso por período (mensal/trimestral/anual) com lembrete e bloqueio na expiração.
  - Implicação prática: a cada início de período, backend cria nova cobrança Pix via API do Mercado Pago, agenda lembrete em D-3 e D-1 (jobs idempotentes), e bloqueia uso quando `subscription.expires_at < now()`. `PaymentProvider` é uma porta de aplicação; implementação inicial é `mercadopago.PixProvider`. `BillingScheduler` (cron + worker outbox) cuida da geração de cobranças, lembretes e expiração. Webhook do MP é a única fonte de verdade de pagamento confirmado e é idempotente por `(provider, event_id, payment_id)`. Trade-off aceito: maior churn (recorrência manual) em troca de menor risco operacional/regulatório frente a Pix Automático recém-lançado.
- Pergunta 2: Qual perfil de compute AWS?
  - Resposta do usuário: ECS Fargate (serviço contínuo) + tasks separadas para worker.
  - Implicação prática: 1 serviço Fargate para API atrás de ALB com WAF (mínimo regras OWASP managed); 1 task Fargate para worker do outbox dispatcher; mesma imagem com command diferente (CMD `api` vs `worker`). Auto-scaling target tracking por CPU/req. VPC com subnets privadas; ALB em subnet pública; RDS em subnet isolada. Secrets Manager para credenciais (MP, Cognito client secret). Logs e métricas via CloudWatch + OTel Collector sidecar/exporter exportando para Grafana Cloud (ou X-Ray; decisão posterior).
- Pergunta 3: Qual volumetria e SLO?
  - Resposta do usuário: Base + crescimento linear (500→5k MAU/12 meses, 99.5% API, webhook MP processado em ≤ 60s p95).
  - Implicação prática: SLOs adotados: API REST p95 < 300ms, disponibilidade 99.5% mensal; webhook ack < 5s p95; ativação ponta a ponta (pagamento confirmado → assinatura ativa → usuário liberado) < 60s p95; RTO 1h, RPO 5 min. Pico mensal estimado em primeira semana do mês concentrando ~60% das cobranças. Capacidade de banco dimensionada para 5k MAU + audit log retido 12 meses. Observabilidade obrigatória desde o v1.
- Pergunta 4: Qual baseline de proteção de PII?
  - Resposta do usuário: RDS + criptografia em coluna via pgcrypto para CPF e telefone + KMS para chave.
  - Implicação prática: `cpf` e `whatsapp_phone` armazenados como `bytea` cifrados via `pgcrypto.pgp_sym_encrypt(value, key)` com chave gerenciada por KMS via envelope (chave KMS por ambiente, DEK injetada via Secrets Manager rotacionada). Para buscas exatas (login/lookup), guardar `cpf_hash` e `phone_hash` (HMAC-SHA256 com pepper em KMS, determinístico). E-mail em texto (necessário para correlação OIDC) com index único. PII redatada em logs por interceptor central. Rotina de re-encrypt agendada anualmente. Aurora/RDS encryption at rest mantida como defesa adicional (não única).

## Rodada 3 - WhatsApp, gating, resiliência e observabilidade

- Pergunta 1: Qual provedor de WhatsApp Business?
  - Resposta do usuário: WhatsApp Cloud API oficial Meta.
  - Implicação prática: integração direta com Graph API/Cloud API; tokens longos de App + System User; verificação de webhook por `hub.verify_token`; templates submetidos previamente à Meta; assinatura de webhook validada por header `X-Hub-Signature-256`. Onboarding inclui Meta Business + Display Name approval (prazo registrado como risco operacional para o gating do v1.1). Tarifa: gratuito para conversas iniciadas pelo usuário até 1k/mês; cobranças por categoria (utility/marketing) acima disso.
- Pergunta 2: Qual é o gating de produção?
  - Resposta do usuário: v1 vai para produção apenas com web (cadastro/login/checkout/bloqueio); WhatsApp linking entra como v1.1 antes de divulgar publicamente.
  - Implicação prática: v1 é beta fechado com pagamento real e identidade real, mas sem promessa pública de WhatsApp ativa. Landing exibe banner de "ativação por WhatsApp em breve" durante o intervalo v1→v1.1. v1.1 obrigatória antes de qualquer divulgação pública pelo usuário. Critério de saída v1: pagamento Pix end-to-end + RBAC com testes de cross-user passando + observabilidade live + runbook de webhook validado. Critério de saída v1.1: vinculação WhatsApp por código + bloqueio de comandos pós-expiração + auditoria de vinculação.
- Pergunta 3: Qual estratégia de resiliência?
  - Resposta do usuário: Postgres RDS Multi-AZ + outbox transacional + retry exponencial em handlers + dedupe por event_id.
  - Implicação prática: RDS Multi-AZ em duas AZ da região (`sa-east-1` recomendada por LGPD e latência); outbox dispatcher existente (`internal/infrastructure/outbox`) reaproveitado; handlers idempotentes por `event_id` com tabela `outbox_handler_dedup(handler_name, event_id)`; backoff exponencial 1s→2s→4s…→max 5min com jitter; após 10 tentativas, evento vai para `outbox_dead_letter` e dispara alerta P2. Webhook MP: deduplicado por `(provider, payment_id, event_type)`. Connection pool dimensionado para tolerar failover Multi-AZ (~60-120s).
- Pergunta 4: Qual stack de observabilidade?
  - Resposta do usuário: OTel Collector sidecar exportando para Grafana Cloud (Loki/Tempo/Prometheus/Mimir).
  - Implicação prática: Backend exporta OTLP via SDK Go já instalado; OTel Collector sidecar agrega/filtra/exporta para Grafana Cloud; Loki para logs estruturados com retenção 14 dias (free) / 30 dias (pago); Tempo para traces 7 dias; Prometheus/Mimir para métricas 13 meses para SLO histórico. Dashboards versionados em `deploy/grafana/` (skill `otel-grafana-dashboards`). Alertas roteados via Grafana Alerting → e-mail/Slack/Discord. SLO burn-rate alarms para API e webhook.

## Rodada 4 - E-mail, sessão browser, LGPD, orçamento

- Pergunta 1: Qual provedor de e-mail transacional?
  - Resposta do usuário: AWS SES.
  - Implicação prática: Domain identity em `mecontrola.app.br` com DKIM + SPF + DMARC; saída de sandbox solicitada antes da release v1; templates versionados; bounces/complaints consumidos via SNS → SQS → handler que marca usuário como `email_undeliverable`. Cognito usa SES integrado para confirmação/reset. Plano de e-mail próprio do produto: recibo Pix pago, lembrete D-3 e D-1, expiração, reativação.
- Pergunta 2: Qual estratégia de sessão no browser?
  - Resposta do usuário: Authorization Code + PKCE + tokens em memória + refresh rotativo.
  - Implicação prática: SPA usa biblioteca OIDC (oidc-client-ts ou amazon-cognito-auth-js) com PKCE; access token em memória (closure) com TTL curto (~15min); refresh token com rotação habilitada no Cognito User Pool (revoga anterior a cada uso); silent refresh via iframe Hosted UI ou refresh fluxo padrão; logout coordenado entre tabs via BroadcastChannel. CSP estrita (`default-src 'self'; script-src 'self'`); cookies não usados para auth; CORS allowlist do backend Go restrita à origem oficial.
- Pergunta 3: Qual política de retenção e LGPD no v1?
  - Resposta do usuário: Retenção ativa enquanto assinatura ativa + 5 anos pós-cancelamento para dados fiscais; endpoint de exclusão manual via suporte.
  - Implicação prática: política de privacidade publicada antes do v1; DPO nomeado e endereço para titular; soft-delete na cancelação com `deactivated_at`; hard-delete de PII não-fiscal após 30 dias da solicitação formal (registro auditável); retenção de comprovantes fiscais 5 anos; endpoint admin `/admin/lgpd/requests` para registrar solicitação de titular e disparar fluxo manual; ROADMAP v2 para self-service. Justificativa legal: art. 7 da LGPD (execução de contrato e cumprimento de obrigação legal fiscal).
- Pergunta 4: Qual guardrail de orçamento AWS?
  - Resposta do usuário: Faixa baixa: US$150-250/mês v1 (sem WhatsApp) escalando para US$300-450 em v1.1.
  - Implicação prática: AWS Budget configurado com alarmes em 80% (notificação) e 100% (alerta crítico e revisão). Driver dominante esperado: NAT Gateway (~US$33/mês fixo + tráfego), RDS db.t4g.micro Multi-AZ (~US$50/mês), ALB (~US$22/mês fixo), Fargate (1vCPU/2GB + 0.5vCPU/1GB ≈ US$45/mês). Mitigação: usar VPC Endpoints para SES/Secrets Manager/S3 para reduzir tráfego NAT; considerar Fargate Spot para worker. Reavaliação trimestral com cost report.
