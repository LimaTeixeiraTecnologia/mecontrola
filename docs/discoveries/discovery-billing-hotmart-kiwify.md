# Discovery: Cobrança Recorrente Hotmart/Kiwify no SaaS de Controle Financeiro via WhatsApp

> **Stack alvo:** Backend Go + Postgres + Redis + LLM (intenções) + WhatsApp
> **Objetivo do MVP:** integrar pagamento + recorrência + gate de uso (pago/não pago)
> **Princípio inegociável:** production-proof desde o dia 1 — idempotência, reconciliação, máquina de estados única.

-----

## 0. Aviso de arquitetura

Hotmart e Kiwify **não foram desenhadas para SaaS recorrente** — foram desenhadas para infoproduto. Funcionam, mas você paga caro em fee e perde controle sobre dunning, retry de cartão e churn involuntário. Para MVP é ótima escolha (cuidam de Pix/Boleto/Cartão, KYC, PCI, antifraude). Planeje migrar para **Asaas / Pagar.me / Stripe** quando passar de ~R$ 30k MRR — a economia paga o esforço.

**Por isso, abstraia o provider desde o dia 1.** Mesmo começando com Kiwify só, o código deve falar com uma `interface BillingProvider`.

-----

## 1. Taxas e escolha do provider

|Provider               |Taxa                     |Mensalidade          |Observação                               |
|-----------------------|-------------------------|---------------------|-----------------------------------------|
|**Hotmart**            |9,9% + R$ 1,00 por venda |Não                  |Recorrência tem custo adicional por plano|
|**Kiwify** (padrão)    |8,99% + R$ 2,49 por venda|Não                  |Taxa fixa pesada em ticket baixo         |
|**Kiwify** (plano pago)|Taxa zero                |Sim, mensalidade fixa|Vale a pena com volume                   |

### Simulação no seu cenário

|Ticket mensal|Hotmart líquido|Kiwify líquido|Vencedor          |
|-------------|---------------|--------------|------------------|
|R$ 19,90     |R$ 16,93       |R$ 15,62      |Hotmart (+R$ 1,31)|
|R$ 29,90     |R$ 25,94       |R$ 24,62      |Hotmart (+R$ 1,32)|
|R$ 49,90     |R$ 43,96       |R$ 42,93      |Hotmart (+R$ 1,03)|
|R$ 97,00     |R$ 86,40       |R$ 85,79      |Empate            |

### Recomendação

- **MVP:** comece com **Kiwify** — API REST moderna (`public-api.kiwify.com`), docs decentes, eventos de assinatura claros.
- Se ticket < R$ 29: avalie o plano pago da Kiwify (taxa zero) ou Hotmart.
- Hotmart tem postback antigo (form-encoded em versões legadas) e API mais burocrática — só vale pela base maior de afiliados, irrelevante para SaaS.

-----

## 2. Arquitetura proposta

```
WhatsApp → Backend Go → LLM → lançamentos
                ↑
                │ (gate: checa entitlement antes de processar)
                │
        ┌───────┴────────┐
        │ EntitlementSvc │  ← Redis cache + Postgres
        └───────▲────────┘
                │
        ┌───────┴────────────────────────────┐
        │ BillingEventProcessor (worker)     │
        └───────▲────────────────────────────┘
                │ consome
        ┌───────┴────────┐
        │ Queue           │  ← Redis Streams ou NATS JetStream
        └───────▲────────┘
                │ publica
        ┌───────┴──────────────────────────────┐
        │ WebhookIngress (HTTP handler Go)     │
        │ /webhooks/kiwify   /webhooks/hotmart │
        └───▲──────────────────▲───────────────┘
            │                  │
         Kiwify             Hotmart
```

**Regra de ouro:** o handler HTTP do webhook **só faz 3 coisas**:

1. Valida assinatura/token.
1. Persiste raw payload + dedup.
1. Retorna 200 em < 2s.

Processamento vai para worker async. Isso é o que separa integração de brinquedo de production-proof.

-----

## 3. Camadas e responsabilidades

### 3.1 WebhookIngress (handler HTTP)

Ordem obrigatória dentro do handler:

1. **Verificar assinatura** antes de qualquer parse pesado:
- Kiwify: token configurado na criação do webhook, vem em header `X-Kiwify-Signature` ou query `?token=`.
- Hotmart: header `X-Hotmart-Hottok`, comparar com token do painel.
1. **Deduplicar** por chave externa única:
- Kiwify: `event_id` do payload.
- Hotmart: usa `(transaction, event, version)` como chave composta.
- Implementação: `INSERT ... ON CONFLICT DO NOTHING` na tabela `webhook_events`.
1. **Persistir raw payload** (JSONB) com status `RECEIVED`.
1. **Publicar na fila** apenas o `event_id` (worker relê do banco).
1. **Retornar 200** rápido. Se demorar, ambas as plataformas retentam e bagunçam tudo.

#### Pseudocódigo Go

```go
func (h *Handler) HandleKiwify(w http.ResponseWriter, r *http.Request) {
    body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // cap 1MB

    if !verifyKiwifySignature(r.Header.Get("X-Kiwify-Signature"), body, h.secret) {
        http.Error(w, "invalid signature", http.StatusUnauthorized)
        return
    }

    var meta struct {
        EventID string `json:"event_id"`
        Event   string `json:"event"`
        OrderID string `json:"order_id"`
    }
    if err := json.Unmarshal(body, &meta); err != nil {
        http.Error(w, "bad payload", http.StatusBadRequest)
        return
    }

    err := h.repo.InsertEvent(r.Context(), "kiwify", meta.EventID, body)
    if errors.Is(err, ErrDuplicate) {
        w.WriteHeader(http.StatusOK) // idempotente
        return
    }
    if err != nil {
        http.Error(w, "db error", http.StatusInternalServerError)
        return
    }

    _ = h.queue.Publish(r.Context(), "billing.events", meta.EventID)
    w.WriteHeader(http.StatusOK)
}
```

### 3.2 BillingEventProcessor (worker)

- Consome da fila, busca raw event no Postgres, **normaliza** para modelo canônico:

  ```go
  type BillingEvent struct {
      Type           string    // ACTIVATED, RENEWED, LATE, CANCELED, REFUNDED, CHARGEBACK
      ExternalUserID string
      ExternalSubID  string
      AmountCents    int
      OccurredAt     time.Time
      PeriodEnd      time.Time
      Provider       string
  }
  ```
- Aplica máquina de estados (seção 4).
- Marca evento como `PROCESSED` ou `FAILED` com `error_message`.
- Retry com backoff exponencial, `max_attempts=10`, depois DLQ.

### 3.3 EntitlementService

Chamado **a cada mensagem do WhatsApp**. Não pode ir ao Postgres toda vez.

```
GET entitlement:{user_id} no Redis
  → cache hit → decide
  → cache miss → lê Postgres → popula Redis com TTL = min(period_end - now, 1h)
```

A chave expira **automaticamente quando a assinatura vence**. TTL = `period_end - now` com cap em 1h.

-----

## 4. Máquina de estados da subscription

Estados internos canônicos (independentes de provider):

|Estado                   |Libera uso?       |Significado                                                         |
|-------------------------|------------------|--------------------------------------------------------------------|
|`TRIALING`               |sim               |Período grátis (você controla — Hotmart/Kiwify não têm trial nativo)|
|`ACTIVE`                 |sim               |Pago e dentro do período                                            |
|`PAST_DUE`               |sim (grace)       |Falha de cobrança, em grace period (sugiro 3 dias)                  |
|`CANCELED_PENDING`       |sim até period_end|Cancelou mas tem acesso até fim do ciclo pago                       |
|`EXPIRED`                |**não**           |Fim de período sem renovação                                        |
|`REFUNDED` / `CHARGEBACK`|**não**           |Bloqueio imediato                                                   |

### Mapeamento Kiwify → estado

|Evento Kiwify                      |Ação                                                   |
|-----------------------------------|-------------------------------------------------------|
|`compra_aprovada` (primeira)       |cria/ativa subscription, `period_end = now + 30d`      |
|`subscription_renewed`             |estende `period_end += 30d`                            |
|`subscription_late`                |marca `PAST_DUE`, inicia grace period                  |
|`subscription_canceled`            |marca `CANCELED_PENDING` (mantém acesso até period_end)|
|`compra_reembolsada` / `chargeback`|`EXPIRED` imediato + notifica financeiro               |
|`pix_gerado` / `boleto_gerado`     |apenas log, não muda entitlement                       |

### Mapeamento Hotmart → estado

|Evento Hotmart                                   |Ação              |
|-------------------------------------------------|------------------|
|`PURCHASE_APPROVED` / `PURCHASE_COMPLETE`        |ativa             |
|`PURCHASE_BILLET_PRINTED`                        |log               |
|`PURCHASE_DELAYED`                               |`PAST_DUE`        |
|`PURCHASE_CANCELED` / `SUBSCRIPTION_CANCELLATION`|`CANCELED_PENDING`|
|`PURCHASE_REFUNDED` / `PURCHASE_CHARGEBACK`      |`EXPIRED` imediato|

-----

## 5. Modelagem Postgres

```sql
-- 1. Raw events (event sourcing — sua salvação em auditoria)
CREATE TABLE webhook_events (
  id            BIGSERIAL PRIMARY KEY,
  provider      TEXT NOT NULL,            -- 'kiwify' | 'hotmart'
  external_id   TEXT NOT NULL,
  event_type    TEXT NOT NULL,
  payload       JSONB NOT NULL,
  signature_ok  BOOLEAN NOT NULL,
  status        TEXT NOT NULL,            -- RECEIVED | PROCESSED | FAILED
  attempts      INT NOT NULL DEFAULT 0,
  error_message TEXT,
  received_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  processed_at  TIMESTAMPTZ,
  UNIQUE (provider, external_id)          -- idempotência
);
CREATE INDEX ON webhook_events (status, received_at) WHERE status != 'PROCESSED';

-- 2. Subscriptions canônicas
CREATE TABLE subscriptions (
  id                   UUID PRIMARY KEY,
  user_id              UUID NOT NULL REFERENCES users(id),
  provider             TEXT NOT NULL,
  provider_sub_id      TEXT NOT NULL,
  provider_customer_id TEXT,
  status               TEXT NOT NULL,
  plan_code            TEXT NOT NULL,
  current_period_end   TIMESTAMPTZ NOT NULL,
  grace_period_end     TIMESTAMPTZ,
  canceled_at          TIMESTAMPTZ,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (provider, provider_sub_id)
);
CREATE INDEX ON subscriptions (user_id, status);
CREATE INDEX ON subscriptions (current_period_end) WHERE status IN ('ACTIVE','PAST_DUE');

-- 3. Transações (cada cobrança individual)
CREATE TABLE billing_transactions (
  id              UUID PRIMARY KEY,
  subscription_id UUID REFERENCES subscriptions(id),
  provider_txn_id TEXT NOT NULL,
  amount_cents    INT NOT NULL,
  status          TEXT NOT NULL,
  paid_at         TIMESTAMPTZ,
  raw_event_id    BIGINT REFERENCES webhook_events(id),
  UNIQUE (provider_txn_id)
);

-- 4. Tokens de magic link (para linkar checkout ↔ WhatsApp)
CREATE TABLE signup_tokens (
  token            TEXT PRIMARY KEY,
  whatsapp_number  TEXT NOT NULL,
  plan_code        TEXT NOT NULL,
  expires_at       TIMESTAMPTZ NOT NULL,
  consumed_at      TIMESTAMPTZ
);
CREATE INDEX ON signup_tokens (expires_at);
```

-----

## 6. Linkagem comprador ↔ WhatsApp (ponto crítico)

O cliente compra na Kiwify com `joao@gmail.com` e telefone `(11) 99999-0000`, mas no seu WhatsApp aparece como `+5511988887777`. Como ligar?

### Estratégias (em ordem de robustez)

#### Estratégia 1 — Magic token no checkout URL ✅ (recomendada)

Fluxo:

1. User pede `/assinar` no WhatsApp.
1. Backend gera `signup_token` (UUID, TTL 30 min), salva em Redis e tabela `signup_tokens` com `whatsapp_number`.
1. Manda link: `https://pay.kiwify.com.br/abc?s={token}`.
1. Kiwify/Hotmart propagam UTM/custom params no webhook.
1. No worker: lê `s`, resolve número, vincula, marca token como `consumed_at`.

```go
// /assinar no WhatsApp
token := uuid.NewString()
err := h.repo.CreateSignupToken(ctx, token, whatsappNumber, "pro", 30*time.Minute)
sendMessage(whatsappNumber, "Pague aqui: " + checkoutURL + "?s=" + token)

// no BillingEventProcessor após compra_aprovada
token := payload.CustomFields["s"] // ou UTM
whatsappNumber, err := h.repo.ConsumeSignupToken(ctx, token)
user := getOrCreateUserByWhatsApp(ctx, whatsappNumber)
createSubscription(ctx, user.ID, ...)
sendMessage(whatsappNumber, "✅ Assinatura ativa! Pode mandar seus gastos.")
```

#### Estratégia 2 — Campo customizado obrigatório no checkout

Funciona, mas usuário pode normalizar diferente (com/sem 9, com/sem +55).

#### Estratégia 3 — Match por telefone normalizado (E.164)

Fallback apenas. Número da compra pode ser fixo, do WhatsApp é celular.

-----

## 7. Reconciliação (não-negociável)

Webhooks **vão falhar**. Rede cai, deploy reinicia pod no meio do POST, plataforma tem incidente. Precisa de job periódico:

- **A cada 1h:** para toda subscription `ACTIVE` ou `PAST_DUE`, consulta API:
  - Kiwify: `GET https://public-api.kiwify.com/v1/subscriptions/{id}` — limite 100 req/min, faça batch com rate limit.
  - Hotmart: API de Sales History com filtro por subscriber.
- **Diariamente 03:00:** full sweep das assinaturas dos últimos 90 dias.
- **Se divergir:** dispara evento sintético no mesmo pipeline (passa pelo `BillingEventProcessor`). **Nunca** atualize estado direto fora dele — máquina de estados é única.

-----

## 8. Armadilhas (vão te morder se ignorar)

1. **Webhook fora de ordem.** `subscription_renewed` pode chegar antes de `compra_aprovada`. Sempre use `occurred_at` do evento; ignore se representaria regressão.
1. **Hotmart versiona payload** (v1, v2, v2.0.0). Versione seu parser.
1. **Reembolso após 7 dias acontece.** Nunca assuma “pagou está sempre OK”. Sempre escute refund/chargeback e bloqueie.
1. **Cartão expirado.** Plataformas tentam recobrar por X dias e desistem. Implemente régua WhatsApp (“seu cartão falhou, atualize aqui: {link}”) — elas não fazem bem.
1. **Mesma assinatura, IDs diferentes ao recomeçar.** Se cliente cancela e volta, vira `subscription_id` novo. **Vincule por `user_id`**, nunca confie só em `provider_sub_id`.
1. **Race condition no entitlement check.** Múltiplas mensagens em paralelo checam ao mesmo tempo. Cache em Redis resolve, mas invalide via `DEL entitlement:{user_id}` no fim do worker.
1. **Logs com PII.** Webhooks vêm com CPF, email, endereço. Mascare antes de enviar pra Datadog/Loki — LGPD.
1. **Replay em ambiente errado.** Webhooks separados por ambiente (dev/staging/prod), URLs e tokens distintos.

-----

## 9. Roadmap MVP (2 semanas)

### Semana 1 — Ingestão

- [ ] **Dia 1–2:** criar produto teste na Kiwify, configurar webhook apontando para ngrok local, mapear todos os eventos reais (faça compras de R$ 1 com Pix para gerar payload real)
- [ ] **Dia 3–4:** implementar `WebhookIngress` + tabela `webhook_events` + verificação de assinatura + idempotência
- [ ] **Dia 5:** fila (Redis Streams ou NATS JetStream — **evitar Kafka aqui**) + worker básico

### Semana 2 — Estado e gate

- [ ] **Dia 6–7:** máquina de estados + tabelas `subscriptions` / `billing_transactions` + `EntitlementService` com cache Redis
- [ ] **Dia 8:** linkagem WhatsApp via magic token
- [ ] **Dia 9:** integração do gate no fluxo do LLM (interceptor antes da intenção: sem assinatura ativa → responde com link de pagamento)
- [ ] **Dia 10:** job de reconciliação + alertas (Slack/Discord quando webhook falha 3x)

### Pós-MVP imediato

- [ ] Painel admin (lista de subs, reembolsos manuais, override de entitlement)
- [ ] Métricas: MRR, churn, falha de cobrança (Prometheus + Grafana)
- [ ] Migrar para Asaas/Pagar.me quando volume justificar

-----

## 10. Decisões para fechar essa semana

1. **Provider único no MVP:** Kiwify.
1. **Abstração de provider** desde o início: `type BillingProvider interface { ... }`.
1. **Stack de infra:** Postgres (já tem) + Redis Streams (não traz infra nova).
1. **Linkagem:** magic token no checkout URL.
1. **Grace period:** 3 dias em `PAST_DUE` antes de cortar — recuperação de churn involuntário sai de graça.
1. **Trial:** controlado no seu lado (Hotmart/Kiwify não têm nativo).

-----

## 11. Interface Go canônica (ponto de partida)

```go
package billing

import (
    "context"
    "time"
)

type EventType string

const (
    EventActivated  EventType = "ACTIVATED"
    EventRenewed    EventType = "RENEWED"
    EventLate       EventType = "LATE"
    EventCanceled   EventType = "CANCELED"
    EventRefunded   EventType = "REFUNDED"
    EventChargeback EventType = "CHARGEBACK"
)

type Event struct {
    Type           EventType
    ExternalUserID string
    ExternalSubID  string
    AmountCents    int
    OccurredAt     time.Time
    PeriodEnd      time.Time
    Provider       string
    SignupToken    string // do magic link
    RawEventID     int64  // FK pra webhook_events
}

type Provider interface {
    Name() string
    VerifySignature(headers map[string]string, body []byte) bool
    ParseEvent(body []byte) (*Event, error)
    GetSubscription(ctx context.Context, subID string) (*Subscription, error)
}

type Subscription struct {
    ID               string
    Status           string
    CurrentPeriodEnd time.Time
    CanceledAt       *time.Time
}
```

-----

## 12. Checklist de “production-proof” antes de ir pro ar

- [ ] Verificação de assinatura em **todos** os webhooks
- [ ] Idempotência testada (mesmo evento 5x → mesmo estado final)
- [ ] Worker processa eventos fora de ordem corretamente
- [ ] Reconciliação rodando e logando divergências
- [ ] Alertas configurados (webhook falhando, fila crescendo, reconciliação divergente)
- [ ] PII mascarada em logs
- [ ] Tokens/secrets em vault (não em env var commitado)
- [ ] Ambientes dev/staging/prod com webhooks separados
- [ ] Testes E2E com compra real de R$ 1 (Pix) em staging
- [ ] Runbook para: reembolso manual, override de entitlement, replay de webhook
- [ ] Dashboard de métricas: MRR, churn, taxa de falha de cobrança, latência do worker

-----

**Próximo passo sugerido:** abrir issue/task para “Dia 1–2” e fazer a primeira compra teste na Kiwify hoje para coletar payloads reais antes de escrever uma linha de código.
