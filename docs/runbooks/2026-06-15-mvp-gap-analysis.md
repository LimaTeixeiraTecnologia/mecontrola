# Runbook: MVP Gap Analysis — Production-Proof

**Data:** 2026-06-15
**Escopo:** O que falta implementar para um MVP MeControla **robusto, eficiente, econômico e
production-proof inegociável** — cobrindo API Go, landing page Astro e infraestrutura/deploy.
**Diretiva de canal:** Telegram (sandbox) primeiro, WhatsApp (oficial) quando o número Business
for liberado.
**Documento companheiro:** `2026-06-15-onboarding-identity-end-to-end.md` (runbook E2E do que
já funciona).

---

## Production-Proof Local — Evidência Verificada (2026-06-15)

Status do ambiente de desenvolvimento (proof real, executado nesta sessão):

| Item | Status | Evidência |
|------|--------|-----------|
| Docker compose local (postgres + otel-lgtm) | ✅ Validado | `task local:infra` sobe ambos; `docker ps` mostra `mecontrola-postgres-1 (healthy)` |
| Migrations aplicadas | ✅ Validado | 36 tabelas no schema `mecontrola` + `schema_migrations` em `public` |
| Server Go compila + sobe | ✅ Validado | `go run ./cmd server` boota sem erro; PID estável |
| Health check end-to-end | ✅ Validado | `GET /health` → 200 `{"status":"healthy","database":{"status":"healthy"}}` |
| DB pool funcional | ✅ Validado | Healthcheck inclui `database.status` healthy |
| Graceful shutdown | ✅ Validado | SIGTERM → server tenta finalizar OTel (context canceled é esperado em shutdown forçado) |
| VSCode debug — launch.json | ✅ Configurado | `.vscode/launch.json` com server, worker, migrate, testes (file/pkg/integration), attach-to-PID, compound; `preLaunchTask: infra:up` |
| VSCode debug — tasks.json | ✅ Configurado | `.vscode/tasks.json` com infra:up, local:up/down/logs, migrate:up, test:unit/integration, lint |
| VSCode settings + extensions | ✅ Configurado | `.vscode/settings.json` (goimports, formatOnSave, race detector); `.vscode/extensions.json` (Go, Docker, YAML, Task) |
| `.env` local destravado | ✅ Corrigido | Adicionadas `ONBOARDING_KIWIFY_CHECKOUT_URLS` (4 URLs Kiwify reais), `IDENTITY_GATEWAY_SHARED_SECRET_CURRENT` (gerado), CORS, `META_BOT_NUMBER_*`, `AGENT_MODE=stub`. Corrigida quebra de parse em linha 58 (`OUTBOX_REAPER_INTERVAL` com espaço sem aspas) |
| Checkout endpoint sem Origin | ⚠️ 500 Internal Error | Reproduz consistentemente. Causa não diagnosticada — provavelmente nil-deref em algum middleware quando `Origin` é vazio. **Adicionar ao P0-API1 (panic recovery) e investigar como bug separado.** |
| Checkout endpoint com Origin `http://localhost:4321` | ⚠️ 403 "origin not allowed" | `corsMiddleware` em `internal/onboarding/infrastructure/http/server/router.go:51-66` apenas seta headers; não bloqueia. A rejeição vem de outro local não identificado. **Adicionar como bug `local-cors-localhost-rejected`.** |
| Kiwify webhook assinado | ⚠️ 401 invalid signature | HMAC-SHA1 hex computado bate com expectativa do middleware (`internal/billing/.../middleware/hmac_signature.go`). Possível causa: trailing newline no body via `curl -d @file`. **Validar canonicalização no middleware como bug `local-kiwify-signature-newline`.** |

**Conclusão honesta:** infraestrutura base (Docker, Postgres, server Go, debug VSCode, observabilidade)
está pronta e funcional localmente. **3 bugs locais identificados durante validação** que
precisam de fix individual (não são bloqueadores para iniciar dev, mas bloqueiam o
"production-proof local" sem falso positivo).

---

## Sumário Executivo

| Bloco | P0 (bloqueador) | P1 (importante) | P2 (pós-MVP) |
|-------|-----------------|-----------------|--------------|
| Landing Page | 3 | 0 | 0 |
| API Core | 7 | 6 | 4 |
| CI/CD | 1 | 0 | 0 |
| Infra/Deploy | 0 | 4 | 3 |
| Docs | 0 | 5 | 0 |
| **Total** | **11** | **15** | **7** |

Estimativa: **6–7 semanas** com 1–2 devs Go + 1 dev frontend de meio período.

Status atual (auditoria 2026-06-15):
- ✅ Endpoints REST corretamente registrados (identity, onboarding, billing, categories, cards,
  transactions, card-purchases, budgets, monthly).
- ✅ Assinaturas HMAC corretas (Kiwify SHA-1, Meta SHA-256, Gateway SHA-256 60s).
- ✅ Outbox + retry + reconciliação Kiwify + reaper implementados.
- ✅ Backups pgBackRest com PITR + crontab + S3 + Prometheus metric.
- ✅ Telemetria OTel + Prometheus + Loki + Grafana configurada; dashboard versionado.
- ✅ VPS hardening base (fail2ban, unattended-upgrades), Caddy ACME, healthchecks por serviço.
- ✅ Coleção Postman com 62 REST + 21 webhooks e pre-request scripts assinando tudo
  automaticamente.
- ❌ Agent conversacional pós-ativação (stub).
- ❌ State machine de onboarding (renda → cartões → categorias → percentuais).
- ❌ Consumer outbound de alertas (domínio calcula, ninguém envia).
- ❌ Panic recovery + job timeouts (riscos críticos de disponibilidade).
- ❌ Trivy/SBOM/cosign removidos do CI (`commit 73b7d61`).

---

## Estratégia de Canal: Telegram (sandbox) → WhatsApp (oficial)

**Problema:** o número WhatsApp Business oficial ainda não foi liberado pela Meta.

**Decisão:** validar todo o fluxo MVP no Telegram primeiro (ativação já implementada em
`internal/onboarding/application/usecases/activate_telegram_by_token.go`; webhook em
`internal/telegram/...`), e migrar para WhatsApp via flag de canal assim que o número estiver
liberado.

**Como evitar retrabalho:** introduzir abstração `Channel` em `internal/platform/channels/`
(item P0-API7) — agent, onboarding usecases e alert dispatcher dependem só da interface. Os
adapters Telegram e WhatsApp implementam o contrato. Trocar de canal é trocar binding no DI.

**Landing page hoje:** `WHATSAPP_URL = wa.me/5511936212870` (número fornecido pelo dono do
produto) — CTA de contato comercial. Quando o bot WhatsApp estiver ativo, esse número vira o
do bot e o texto pré-preenchido vira `ATIVAR <token>`.

---

## P0 — Bloqueadores de MVP

### P0-LP1 · Substituir placeholders na landing page ✅ FEITO (2026-06-15)

**Repo GitHub oficial:** `https://github.com/LimaTeixeiraTecnologia/limateixeira-landingpage`
**Pasta local:** `/Users/jailtonjunior/Git/mecontrola-landingpage`

Arquivo `src/lib/content.ts:127-134` substituído com os valores reais:

```typescript
export const WHATSAPP_URL = 'https://wa.me/5511936212870?text=Ol%C3%A1%2C%20quero%20conhecer%20o%20MeControla';
export const CHECKOUT_URL_MENSAL = 'https://pay.kiwify.com.br/ocPt7sv';      // R$ 29,90
export const CHECKOUT_URL_TRIMESTRAL = 'https://pay.kiwify.com.br/Sh2upAU';  // R$ 74,90
export const CHECKOUT_URL_ANUAL = 'https://pay.kiwify.com.br/HquleKA';       // R$ 239,90
export const LEGAL_NAME = 'Lima Teixeira Tecnologia LTDA';
```

URL extra (não vai pra landing — usar só para QA pago):
`https://pay.kiwify.com.br/ix6YIk3` (R$ 5,00).

**Validação:**
```bash
cd /Users/jailtonjunior/Git/mecontrola-landingpage
make install && make build && make preview
# abrir http://localhost:4321, clicar nos 3 CTAs e validar redirect
```

### P0-LP2 · Configurar `PUBLIC_GA_ID` no GitHub

`CLOUDFLARE_ACCOUNT_ID` e `CLOUDFLARE_API_TOKEN` já configurados desde 2026-05-24 (confirmado
via `gh secret list`). Falta apenas o GA4:

```bash
gh secret set PUBLIC_GA_ID \
  --repo LimaTeixeiraTecnologia/limateixeira-landingpage \
  --body "G-XXXXXXXXXX"   # substituir pelo Measurement ID real
```

### P0-LP3 · Gerar `og-image.png`

Hoje só existe `public/og-image.svg`; o `Layout.astro:20` referencia
`https://mecontrola.app.br/og-image.png`. Sem o PNG, OG cards (Facebook/Twitter) caem em
fallback genérico.

```bash
cd /Users/jailtonjunior/Git/mecontrola-landingpage
make og-image
git add public/og-image.png && git commit -m "feat(seo): generate og-image.png"
```

---

### P0-API1 · Panic recovery em HTTP handlers e job scheduler

**Risco:** sem `defer recover()`, um único panic intra-request derruba o processo inteiro.
Outbox dispatcher fica stuck; conexões abertas vazam; jobs em voo perdem trabalho.

**Implementar:**

1. **HTTP middleware** — adicionar `chi.Recoverer` (já vem no go-chi) com logger estruturado
   na chain global de `cmd/server/server.go`. Logar `panic_value`, `stack`, `request_id`,
   responder 500 com payload sanitizado.
2. **Job scheduler** — em `internal/platform/worker/job/scheduler.go`, envolver cada
   `job.Run(ctx)` em wrapper que faz `defer func() { if r := recover(); r != nil { ... } }`.
   Marcar o job como `failed` no schedule sem matar o worker.
3. Adicionar métrica `panics_recovered_total{location="http"|"job"}`.

**Esforço:** M (1 dev × 2 dias). **Teste:** injetar panic via flag de debug; verificar que
processo segue de pé e métrica incrementa.

### P0-API2 · Job timeouts próprios

**Risco:** 12 jobs hoje só respeitam o timeout global de shutdown (15s). Jobs longos (Kiwify
reconciliação, anonymization, grace expiration, outbox reaper) podem ser force-killed
mid-flight, deixando dados inconsistentes.

**Implementar:**

1. Adicionar método `Timeout() time.Duration` na interface `Job` em
   `internal/platform/worker/job/`. Default 30s; jobs custom (reconciliação, anonymization)
   declaram explícito (5min, 10min).
2. No scheduler, antes de chamar `Run`, criar `ctx, cancel := context.WithTimeout(parent,
   job.Timeout())`. Logar timeout exceeded como `error` com `job_name`, `elapsed`.
3. Documentar timeout em cada job no comentário de struct (exceção autorizada por
   `R-ADAPTER-001` se necessário, mas idealmente em `agent-governance` doc).
4. Adicionar métrica `job_duration_seconds{job_name}` histograma; alerta se p99 ultrapassar
   80% do timeout configurado.

**Esforço:** M (1 dev × 2 dias).

### P0-API3 · Onboarding pós-ATIVAR (usecases de renda, cartões, categorias, percentuais)

**Risco:** após `ATIVAR <token>`, o user é ativado mas o bot fica em stub ("MeControla recebeu
sua mensagem"). MVP fica funcionalmente quebrado.

**Modelo de dados a criar:**

`internal/identity/domain/entities/user.go` — adicionar `Profile`:
```go
type Profile struct {
    monthlyIncomeCents   int64
    cardIDs              []uuid.UUID
    categoryPercentages  map[uuid.UUID]float64  // soma == 1.0
    onboardingStep       OnboardingStep         // valueobject
}

type OnboardingStep string
const (
    StepInformIncome        OnboardingStep = "inform_income"
    StepLinkCards           OnboardingStep = "link_cards"
    StepReviewCategories    OnboardingStep = "review_categories"
    StepSetPercentages      OnboardingStep = "set_percentages"
    StepCompleted           OnboardingStep = "completed"
)
```

**Usecases a criar:**

| Usecase | Arquivo | Validação obrigatória (smart constructor) |
|---------|---------|-------------------------------------------|
| `InformIncome` | `internal/onboarding/application/usecases/inform_income.go` | Renda > 0, < R$ 10.000.000; user em `StepInformIncome` |
| `LinkCard` | `internal/onboarding/application/usecases/link_card.go` (delega para `card.CreateCardUC`) | User em `StepLinkCards`; máximo 5 cartões; nome único por user |
| `ConfirmCategories` | `internal/onboarding/application/usecases/confirm_categories.go` | Snapshot das 5 default categories no Profile |
| `SetCategoryPercentage` | `internal/onboarding/application/usecases/set_category_percentage.go` | 0 < % < 1; soma de todas == 1.0 ao avançar; user em `StepSetPercentages` |

**Princípios DMMF (R-TXN-WORKFLOWS-001 análogo):**
- Toda validação em smart constructors dos commands.
- Usecases chamam `Decide*` puro que retorna events + new state.
- Adapters Telegram/WhatsApp são finos (R-ADAPTER-001).

**Esforço:** M-L (1 dev × 1.5 semanas).

### P0-API4 · State machine no agent (memória de passo)

**Risco:** agent hoje é stateless (`internal/agent/application/usecases/handle_inbound_message.go:68-100`).
Sem saber em qual passo o user está, ele não consegue rotear mensagens para o usecase certo.

**Implementar:**

1. Em `IntentDispatcher` (`internal/agent/infrastructure/dispatcher/intent_dispatcher.go`),
   antes de dispatchar para um adapter, consultar `User.Profile.OnboardingStep`:
   - `StepInformIncome` → parse número como renda; invoca `InformIncome`.
   - `StepLinkCards` → parse "Nubank 5000 27 06" (nome, limite, fechamento, vencimento);
     invoca `LinkCard`. Botão "concluído" avança step.
   - `StepReviewCategories` → mostra 5 categorias default; confirma com `ConfirmCategories`.
   - `StepSetPercentages` → loop categoria a categoria; invoca `SetCategoryPercentage`.
   - `StepCompleted` → roteamento normal (transactions, queries).
2. Adicionar `GetUserOnboardingState` query usecase consumido pelo agent.
3. Persistir state via `user_repository.UpdateProfile`.

**Esforço:** M (1 dev × 1 semana). Acoplado a P0-API3 e P0-API6.

### P0-API5 · Alertas pró-ativos outbound (multi-canal)

**Risco:** domínio calcula alertas (`internal/budgets/domain/services/alert_workflow.go` —
`PendingDelivery`, `Delivered`, etc) mas **nenhum consumer entrega**. A imagem do produto
promete 3 alertas (categoria >80%, meta >50% antes do prazo, cartão >50% do limite) —
sem isso o produto perde a feature marqueteira.

**Implementar:**

1. Criar consumer `internal/budgets/infrastructure/messaging/consumers/alert_notification_consumer.go`
   que escuta eventos `AlertPendingDelivery` do outbox.
2. Consumer usa a interface `OutboundMessenger` (criada em P0-API7) — implementação Telegram
   primeiro, WhatsApp depois.
3. Registrar consumer no worker manager (`internal/platform/worker/`).
4. Rate limit por user/categoria/dia (item P1-API6 — mas implementar já agora para evitar spam
   em validação).
5. Idempotência: usar `alert.id` como `event_id` no outbox; tabela
   `alert_deliveries(alert_id, channel, status, attempted_at, delivered_at)`.

**Esforço:** M (1 dev × 4-5 dias).

### P0-API6 · Telegram multi-turno como sandbox de validação

**Promovido de P2** — caminho crítico do MVP enquanto WhatsApp Business não sai.

**Status atual:**
- ✅ Webhook Telegram com signature middleware (`internal/telegram/infrastructure/http/...`)
- ✅ Ativação via `ATIVAR <token>` (`activate_telegram_by_token.go`)
- ❌ Diálogo multi-turno (idem WhatsApp — falta P0-API3 + P0-API4)

**Implementar:**
1. Conectar Telegram dispatcher ao mesmo `IntentDispatcher` do agent (já é channel-agnostic em
   parte; falta P0-API7 para terminar de abstrair).
2. Implementar `TelegramOutboundMessenger` (envia mensagem proativa via `sendMessage` da Bot
   API) que cumpre interface `Channel`.
3. Testes E2E: cenário completo de ativação + onboarding + lançamento + alerta no Telegram.
4. Configurar bot Telegram de sandbox em `@MeControla_sandbox_bot`; token em `TELEGRAM_BOT_TOKEN`.

**Esforço:** M (1 dev × 1 semana, paralelizável com P0-API3/4).

### P0-API7 · Abstração de canal (channel-agnostic agent)

**Risco se não fizer:** quando o WhatsApp Business sair, retrabalho enorme — todo o agent +
alerts + onboarding precisariam ser duplicados.

**Implementar:**

1. Criar `internal/platform/channels/channel.go`:
   ```go
   type Channel interface {
       SendText(ctx context.Context, user UserRef, text string) error
       SendButtons(ctx context.Context, user UserRef, prompt string, options []Button) error
       Name() string  // "telegram" | "whatsapp"
   }
   ```
2. Implementar `TelegramChannel` (delega para Bot API).
3. Implementar `WhatsAppChannel` (delega para Meta Cloud `sendMessage`).
4. Registrar no DI conforme env (`PRIMARY_CHANNEL=telegram` ou `whatsapp`); permitir
   broadcast multi-canal para alertas.
5. Refatorar `internal/platform/channels/activation_command.go` para usar a interface (parte
   já existe).

**Esforço:** M (1 dev × 4 dias). Deve ser feito **antes** de P0-API5 e P0-API6.

### P0-CI1 · Supply chain (SBOM + cosign sem Trivy) — REVISADO ⚠️

**Contexto:** commit `73b7d61` removeu o job `scan-and-attest` intencionalmente porque
`aquasecurity/setup-trivy` falhava em toda execução de CI (download do binário v0.62.1
quebrado). Mensagem do commit argumenta que VPS + `govulncheck` já cobrem CVEs em
dependências Go.

**Avaliação crítica:**
- ✅ `govulncheck` cobre CVEs **em dependências Go**.
- ❌ `govulncheck` **NÃO** cobre CVEs em pacotes do **base image Linux** (libc, openssl,
  ca-certificates, etc).
- ❌ Sem SBOM, auditoria de compliance fica cega.
- ❌ Sem cosign, qualquer imagem com nome `ghcr.io/...:tag` pode ser substituída sem
  detecção (`docker pull` confiando em tag mutável).

**Decisão recomendada (espera aprovação):**

**Opção A — Restaurar com Trivy estável** (recomendado):
- Trocar `aquasecurity/setup-trivy` por `aquasecurity/trivy-action` (action oficial que
  baixa via fallback) ou pinar versão estável v0.50.x.
- Manter SBOM + cosign de qualquer forma (não dependem de Trivy).

**Opção B — Apenas SBOM + cosign keyless (sem Trivy)**:
- Reduz escopo: assinatura + provenance attestation sem container scan.
- Aceita o risco de CVE em base image (mitigado por base image distroless minimalista).

**Opção C — Manter como está + adicionar `docker scout` (alternativo a Trivy)**:
- `docker scout` é built-in no Docker desktop e funciona em CI.
- Cobertura equivalente sem o problema de download do binário.

**Esforço:** M (1 dev × 0.5 dia para opção B; 1 dia para A ou C).

**Não vou implementar agora** — escolha da opção exige decisão do dono do produto, e reintroduzir
o problema original do Trivy quebraria CI novamente.

---

## P1 — Importante (lança mas degradado)

### API

- **P1-API1** — Mover validação HMAC Kiwify para middleware (hoje rejeição é em use case).
  Defense-in-depth.
  `internal/billing/infrastructure/http/server/middleware/hmac_signature.go` já existe;
  garantir que está no início do chain antes do handler. **S, 0.5 dia.**

- **P1-API2** — `/readiness` probe que respeita `ctx.Done()` durante shutdown. Hoje só `/health`
  responde DB ping; LB precisa saber quando o pod está drenando.
  Adicionar handler em `cmd/server/server.go` que retorna 503 quando shutdown sinaliza.
  **S, 0.5 dia.**

- **P1-API3** — `ConversationHistory` em `LLMRequest`. Hoje cada mensagem é stateless; LLM
  responde desconectado em multi-turno.
  `internal/agent/application/interfaces/llm_provider.go` — adicionar field; persistir
  threads por user em tabela `conversation_messages(user_id, role, content, created_at)`;
  limit últimas 20 mensagens.
  **S-M, 2 dias.**

- **P1-API4** — Schema NLU documentado + smoke tests. Sem schema explícito, o que o LLM retorna
  é "magia". Documentar em `docs/integrations/agent-nlu-schema.md` o JSON Schema esperado
  (`{module, action, payload}`); validar com smoke tests no agent. **S, 1 dia.**

- **P1-API5** — Formatação por canal (WhatsApp limita 4096 chars; truncar com "…"). Criar
  formatter em `internal/platform/channels/formatter.go`. **S, 1 dia.**

- **P1-API6** — Rate limit alertas: máximo 1 alerta/user/categoria/dia. Sem isso, com 100
  users em hard mês um único spike pode disparar milhares de mensagens (custo Meta + Telegram +
  UX ruim). Implementar throttle persistente em `alert_deliveries`. **S, 1 dia.**

### Infraestrutura

- **P1-INFRA1** — Log rotation no `deployment/compose/compose.prod.yml`. Hoje sem
  `json-file` driver + max-size → disco enche em 1-2 meses. Aplicar em todos services:
  ```yaml
  logging:
    driver: json-file
    options: { max-size: "100m", max-file: "10" }
  ```
  **S, 15 min.**

- **P1-INFRA2** — Outbox lag metric. Adicionar gauge `outbox_events_pending_count` em
  `internal/platform/outbox/dispatcher.go`; alerta Prometheus se >1000 por >5 min.
  Sem isso, dispatcher pode estar atrás e ninguém percebe.
  **S, 0.5 dia.**

- **P1-INFRA3** — SLO/SLI documentado em `docs/slo.md`. Sugestão MVP:
  - Disponibilidade `/api/v1/*`: 99.5% mensal.
  - Auth gateway p99 < 200ms.
  - Outbox lag p95 < 60s.
  - Backup success 100%.
  - Webhook Kiwify success ratio > 99%.
  Sem SLO, alertas existem mas time não sabe quando "está ruim" vs "normal".
  **M, 1 dia.**

- **P1-INFRA4** — SSH hardening completo. Estender `deployment/scripts/vps-hardening.sh`:
  ```bash
  sed -i 's/^#\?PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config
  sed -i 's/^#\?PermitRootLogin.*/PermitRootLogin no/' /etc/ssh/sshd_config
  systemctl restart ssh
  fallocate -l 2G /swapfile && chmod 600 /swapfile && mkswap /swapfile && swapon /swapfile
  echo '/swapfile none swap sw 0 0' >> /etc/fstab
  ```
  **M, 0.5 dia.**

### Docs / Onboarding operacional

- **P1-DOCS1** — `docs/integrations/whatsapp-setup.md`. Passo a passo: criar Meta Business
  Account → App → WhatsApp Cloud → adicionar número → webhook URL → verify token → access
  token permanente → System User. Fluxo de aprovação Meta + tempo médio (3-7 dias).
- **P1-DOCS2** — `docs/integrations/kiwify-setup.md`. Painel Kiwify → produtos (com SKUs
  monthly/quarterly/annual/test) → webhook URL → signature secret → tracking parameters
  (`sck`/`s1`/`src`) → testar com botão "Testar" do painel.
- **P1-DOCS3** — `docs/infrastructure/vps-hostinger-setup.md`. Spec mínima: VPS 2 Plan (4
  vCPU, 8 GB RAM, 100 GB NVMe) ou superior. Inclui checklist pós-provisionamento (script
  hardening, swapfile, firewall ufw, fail2ban, docker, compose, clone repo, .env).
- **P1-DOCS4** — `docs/infrastructure/dns-setup.md`. Cloudflare para landing
  (`mecontrola.app.br` → Cloudflare Pages), DNS A direto para VPS na API
  (`api.mecontrola.app.br` → IP VPS, proxy desligado pra deixar Caddy gerenciar TLS).
- **P1-DOCS5** — Reescrever `deployment/runbooks/rotate-secret.md` — hoje documenta Fly.io.
  Produção é VPS; rotação é `ssh VPS && edit /opt/mecontrola/.env && docker compose
  -f compose.yml -f compose.prod.yml restart server worker`. Documentar duplo-secret
  (`_CURRENT` + `_NEXT`) para zero-downtime rotation.

---

## P2 — Nice-to-have (pós-MVP)

| ID | Item | Esforço |
|----|------|---------|
| P2-API1 | NLU fallback handling (intent ambíguo → solicitar clarificação) | M |
| P2-API2 | Merchant dictionary autocategorization ("ifood"→"alimentação") com aprendizado por user | M |
| P2-API3 | Income trends dashboard (gastos vs orçado vs histórico de meses) | L |
| P2-API4 | Recurring transactions template (pré-populado Netflix/Spotify pelos top brasileiros) | S |
| P2-INFRA1 | Backup restore automático + smoke test em CI semanal | M |
| P2-INFRA2 | Migration rollback runbook em `deployment/runbooks/rollback-migration.md` | S |
| P2-INFRA3 | DLQ separada para outbox + dashboard Grafana específico | S |

---

## Roadmap MVP — 6-7 semanas

| Sprint | Semana | Itens | Resultado |
|--------|--------|-------|-----------|
| 1 | 1 | P0-LP1✅ + P0-LP2 + P0-LP3 + P0-CI1 + P1-INFRA1 + P0-API1 + P0-API2 | Landing live, supply chain restaurada, infra estável |
| 2 | 2 | P0-API7 + P0-API6 | Abstração de canal pronta + Telegram multi-turno |
| 3 | 3-4 | P0-API3 + P0-API4 | Onboarding pós-ATIVAR completo no Telegram |
| 4 | 5 | P0-API5 + P1-API1/2/3 | Alertas outbound + endurecimentos de segurança/UX |
| 5 | 6 | P1-INFRA2/3/4 + P1-DOCS1-5 | Production-proof completo |
| 6 | 7 | Buffer + ativar WhatsApp Business assim que número for liberado + UAT | Go-live oficial |

---

## Validação Final (production-proof checklist)

Antes de campanha paga:

- [ ] Landing live em `https://mecontrola.app.br` com CTAs Kiwify funcionais
- [ ] `gh secret list --repo LimaTeixeiraTecnologia/limateixeira-landingpage` mostra 3 secrets
  (CLOUDFLARE_*, PUBLIC_GA_ID)
- [ ] `og-image.png` gerado e referenciável em `https://mecontrola.app.br/og-image.png`
- [ ] CI verde com Trivy + SBOM + cosign signing + provenance attestation
- [ ] Stack VPS subida via `compose.yml + compose.prod.yml` com logs rotacionados
- [ ] Caddy TLS automático em `api.mecontrola.app.br` validado por `openssl s_client`
- [ ] pgBackRest backup full executado + restore smoke test OK
- [ ] Grafana dashboards mostrando métricas vivas (outbox lag, auth errors, request rate,
  panic recovered, job duration)
- [ ] Alertas Prometheus configurados conforme `prometheus-rules.yaml` e validados
- [ ] Onboarding E2E no Telegram: ATIVAR → renda → cartões → categorias → percentuais →
  lançamento → consulta → alerta proativo entregue
- [ ] Webhook Kiwify configurado no painel real para todos os 6 eventos críticos
- [ ] Webhook Telegram configurado e bot respondendo
- [ ] Webhook Meta (placeholder até número Business sair) com `META_VERIFY_TOKEN` funcional
- [ ] Runbook de rotação de secrets executado em dry-run
- [ ] Runbook de PITR (`deployment/runbooks/restore-vps.md`) executado em staging

---

## Referências cruzadas

- Runbook E2E (jornada do cliente passo a passo): `2026-06-15-onboarding-identity-end-to-end.md`
- Coleção Postman (62 REST + 21 webhooks): `docs/postman/`
- Diagramas C4 e flows por domínio: `docs/diagrams/{onboarding,identity,billing,agent,budgets,card,categories,transactions}/`
- Backup/PITR: `deployment/runbooks/restore-vps.md`
- Hardening VPS: `deployment/scripts/vps-hardening.sh`
- Regras de governança: `.claude/rules/governance.md`, `.claude/rules/go-adapters.md`,
  `.claude/rules/transactions-workflows.md`
