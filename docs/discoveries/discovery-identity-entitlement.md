# Discovery: Identidade Mínima + Entitlement (sem RBAC)

> **Contexto:** SaaS de controle financeiro via WhatsApp. Backend Go + Postgres + Redis + LLM.
> **Objetivo:** modelar identidade e gate de acesso (pago/não pago) pro MVP sem cair na armadilha de RBAC prematuro.
> **Princípio:** construa a abstração na **segunda** vez que a dor aparecer, não na primeira.

-----

## 0. Por que NÃO RBAC no MVP

RBAC existe pra resolver **“quem pode fazer o quê dentro de um recurso compartilhado”**. Seu produto hoje é:

- 1 usuário = 1 número de WhatsApp = 1 conta financeira individual
- Não tem time, org, recurso compartilhado
- Não tem “admin do workspace” vs “viewer”
- A mensagem do WhatsApp já é autenticada pelo provedor (Meta garante o `from`)

Adicionar RBAC agora = tabelas de `roles`, `permissions`, `role_permissions`, policy engine (Casbin/OPA), middleware de autorização, cache de permissões. Tudo isso pra resolver **zero problema real do MVP**.

### Identity ≠ Authentication ≠ Authorization

|Conceito                |Precisa no MVP?|O que é                                                |
|------------------------|---------------|-------------------------------------------------------|
|**Identity**            |✅ sim          |Quem é esse número de WhatsApp? UUID estável.          |
|**Entitlement**         |✅ sim          |Esse user tem assinatura ativa agora?                  |
|**Authentication**      |parcial        |WhatsApp já autentica via Meta. Admin web = magic link.|
|**Authorization (RBAC)**|❌ não          |Sem múltiplos atores no mesmo recurso.                 |

Linkar user ↔ subscription é **só uma FK**: `subscriptions.user_id → users.id`. RBAC não resolve nada disso.

-----

## 1. Modelagem de Identity (mínima, production-proof)

```sql
-- Identidade canônica
CREATE TABLE users (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  whatsapp_number TEXT NOT NULL UNIQUE,           -- E.164: +5511988887777
  display_name    TEXT,
  email           TEXT UNIQUE,                    -- opcional, pra recibo/admin web
  is_admin        BOOLEAN NOT NULL DEFAULT false, -- você + suporte (resolve 100% no MVP)
  status          TEXT NOT NULL DEFAULT 'ACTIVE', -- ACTIVE | BLOCKED | DELETED
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at      TIMESTAMPTZ                     -- soft delete (LGPD)
);
CREATE INDEX ON users (email) WHERE email IS NOT NULL;
CREATE INDEX ON users (status) WHERE status != 'ACTIVE';

-- Histórico de números (user troca de celular, mantém dados)
CREATE TABLE user_whatsapp_history (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  number       TEXT NOT NULL,                     -- E.164
  active       BOOLEAN NOT NULL,
  linked_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  unlinked_at  TIMESTAMPTZ,
  reason       TEXT                               -- 'port_in', 'support_change', etc.
);
CREATE INDEX ON user_whatsapp_history (user_id, active);
CREATE INDEX ON user_whatsapp_history (number);
```

### Decisões que te salvam depois

1. **PK é UUID, não é o número.** Número muda (porta operadora, troca chip). FK de subscription aponta pra UUID. **Inegociável.**
1. **`is_admin` como bool simples**, não tabela `roles`. Quando precisar de 3+ tipos de ator, vira `text[] roles` numa migration de 5 minutos. Não invente abstração antes da dor.
1. **Sem `password_hash`, sem `sessions`** — autenticação no WhatsApp é o canal. Pro futuro admin web, magic link via email basta.
1. **Soft delete** (`deleted_at`) por LGPD — usuário pede exclusão, você marca e tem 30 dias pra anonimizar de fato.
1. **Normalização E.164 obrigatória** na escrita. Nunca aceite “11988887777” cru — sempre `+5511988887777`. Uma função de normalização no momento da escrita resolve.

### Função de normalização (Go)

```go
package identity

import (
    "fmt"
    "regexp"
    "strings"
)

var digitsOnly = regexp.MustCompile(`\D`)

// NormalizeWhatsAppBR normaliza número BR pra E.164.
// Aceita: 11988887777, (11) 98888-7777, +5511988887777, 5511988887777
// Retorna: +5511988887777
func NormalizeWhatsAppBR(input string) (string, error) {
    digits := digitsOnly.ReplaceAllString(input, "")

    switch len(digits) {
    case 10, 11: // sem código do país
        digits = "55" + digits
    case 12, 13: // com 55
        if !strings.HasPrefix(digits, "55") {
            return "", fmt.Errorf("número inválido: %s", input)
        }
    default:
        return "", fmt.Errorf("tamanho inválido: %s", input)
    }

    // Garante o 9 no celular (DDDs brasileiros pós-2012)
    if len(digits) == 12 {
        // 55 + DDD(2) + 8 dígitos → injeta 9
        digits = digits[:4] + "9" + digits[4:]
    }

    if len(digits) != 13 {
        return "", fmt.Errorf("não foi possível normalizar: %s", input)
    }

    return "+" + digits, nil
}
```

**Regra:** chame `NormalizeWhatsAppBR` em **todo ponto de entrada** (webhook do WhatsApp, magic token, admin manual). Nunca confie no input cru.

-----

## 2. Entitlement: o gate de acesso

A pergunta que sua aplicação faz a cada mensagem do WhatsApp:

> **“Esse `user_id` pode usar o produto AGORA?”**

Resposta em < 5ms (não pode ir no Postgres toda vez).

### Modelagem

```sql
-- Tabela já modelada no doc de billing, repetida pra contexto
CREATE TABLE subscriptions (
  id                   UUID PRIMARY KEY,
  user_id              UUID NOT NULL REFERENCES users(id),
  provider             TEXT NOT NULL,
  provider_sub_id      TEXT NOT NULL,
  status               TEXT NOT NULL,             -- TRIALING|ACTIVE|PAST_DUE|CANCELED_PENDING|EXPIRED|REFUNDED
  plan_code            TEXT NOT NULL,
  current_period_end   TIMESTAMPTZ NOT NULL,
  grace_period_end     TIMESTAMPTZ,
  canceled_at          TIMESTAMPTZ,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (provider, provider_sub_id)
);
CREATE INDEX ON subscriptions (user_id, status);
```

### Regra de “tem acesso?” (uma função pura)

```go
package entitlement

import "time"

type Status string

const (
    StatusTrialing         Status = "TRIALING"
    StatusActive           Status = "ACTIVE"
    StatusPastDue          Status = "PAST_DUE"
    StatusCanceledPending  Status = "CANCELED_PENDING"
    StatusExpired          Status = "EXPIRED"
    StatusRefunded         Status = "REFUNDED"
)

type Subscription struct {
    Status            Status
    CurrentPeriodEnd  time.Time
    GracePeriodEnd    *time.Time
}

// IsEntitled é a única fonte de verdade. Pura, testável, sem I/O.
func IsEntitled(sub *Subscription, now time.Time) bool {
    if sub == nil {
        return false
    }
    switch sub.Status {
    case StatusActive, StatusTrialing:
        return now.Before(sub.CurrentPeriodEnd)
    case StatusCanceledPending:
        return now.Before(sub.CurrentPeriodEnd) // mantém acesso até fim do ciclo pago
    case StatusPastDue:
        return sub.GracePeriodEnd != nil && now.Before(*sub.GracePeriodEnd)
    case StatusExpired, StatusRefunded:
        return false
    }
    return false
}
```

**Por quê função pura:** testes unitários cobrem 100% das transições sem mock de banco. Você dorme tranquilo.

-----

## 3. EntitlementService (com cache)

```go
package entitlement

import (
    "context"
    "encoding/json"
    "time"

    "github.com/redis/go-redis/v9"
)

type Decision struct {
    Entitled  bool      `json:"entitled"`
    Reason    string    `json:"reason"`              // "active", "no_subscription", "expired", "past_due_grace"
    ExpiresAt time.Time `json:"expires_at"`
}

type Service struct {
    db    Repository
    cache *redis.Client
    clock func() time.Time
}

func (s *Service) Check(ctx context.Context, userID string) (Decision, error) {
    key := "entitlement:" + userID

    // 1. Tenta cache
    if raw, err := s.cache.Get(ctx, key).Bytes(); err == nil {
        var d Decision
        if json.Unmarshal(raw, &d) == nil && s.clock().Before(d.ExpiresAt) {
            return d, nil
        }
    }

    // 2. Lê do Postgres
    sub, err := s.db.GetActiveSubscription(ctx, userID)
    if err != nil {
        return Decision{}, err
    }

    now := s.clock()
    decision := Decision{
        Entitled:  IsEntitled(sub, now),
        ExpiresAt: now.Add(1 * time.Hour), // cap
    }
    if sub != nil {
        decision.Reason = string(sub.Status)
        // TTL = min(period_end - now, 1h) → cache expira junto da subscription
        if !sub.CurrentPeriodEnd.IsZero() && sub.CurrentPeriodEnd.Before(decision.ExpiresAt) {
            decision.ExpiresAt = sub.CurrentPeriodEnd
        }
    } else {
        decision.Reason = "no_subscription"
    }

    // 3. Popula cache
    ttl := time.Until(decision.ExpiresAt)
    if ttl > 0 {
        raw, _ := json.Marshal(decision)
        s.cache.Set(ctx, key, raw, ttl)
    }

    return decision, nil
}

// Invalidate é chamado pelo BillingEventProcessor ao mudar estado da subscription
func (s *Service) Invalidate(ctx context.Context, userID string) error {
    return s.cache.Del(ctx, "entitlement:"+userID).Err()
}
```

### Pontos críticos

1. **TTL inteligente** = `min(period_end - now, 1h)`. Chave expira sozinha quando a assinatura vence — você nunca serve cache stale liberando acesso indevido.
1. **Invalidação síncrona** no final do worker que processa webhook. Ordem importa: atualiza Postgres → invalida cache → publica notificação WhatsApp.
1. **Cache miss não é erro**, é cenário normal. Loga em DEBUG, não em ERROR.
1. **Negative cache** (user sem subscription) **também é cacheado** com TTL curto (5min) — protege Postgres de ataque de mensagens de número aleatório.

-----

## 4. Integração no fluxo do WhatsApp

```go
// handler que recebe mensagem do WhatsApp
func (h *WhatsAppHandler) Handle(ctx context.Context, msg IncomingMessage) error {
    number, err := identity.NormalizeWhatsAppBR(msg.From)
    if err != nil {
        return h.reply(ctx, msg.From, "Número inválido.")
    }

    // 1. Identity: resolve ou cria user
    user, isNew, err := h.users.GetOrCreate(ctx, number)
    if err != nil {
        return err
    }

    // 2. Onboarding pra novo user (não precisa de subscription pra responder isso)
    if isNew {
        return h.reply(ctx, number, "Bem-vindo! Pra começar, assine: " + h.generatePaymentLink(ctx, number))
    }

    // 3. Gate de entitlement
    decision, err := h.entitlement.Check(ctx, user.ID)
    if err != nil {
        return err
    }
    if !decision.Entitled {
        return h.reply(ctx, number, h.copyForBlocked(decision.Reason) + " " + h.generatePaymentLink(ctx, number))
    }

    // 4. Liberado — segue pra LLM
    return h.llm.Process(ctx, user, msg)
}

func (h *WhatsAppHandler) copyForBlocked(reason string) string {
    switch reason {
    case "no_subscription":
        return "Você ainda não tem uma assinatura ativa."
    case "EXPIRED":
        return "Sua assinatura expirou."
    case "REFUNDED":
        return "Sua assinatura foi reembolsada."
    case "PAST_DUE":
        return "Sua cobrança falhou e o período de tolerância acabou."
    default:
        return "Sua assinatura não está ativa."
    }
}
```

**Observe:** o LLM **nunca** é chamado se o user não tem entitlement. Custo zero de token pra inadimplente.

-----

## 5. Admin web (quando você criar) — sem RBAC ainda

Pro painel admin (você + 1-2 pessoas de suporte):

```go
// Middleware simples
func RequireAdmin(users UserRepo) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            userID := SessionUserID(r) // do cookie/JWT do magic link
            user, err := users.Get(r.Context(), userID)
            if err != nil || !user.IsAdmin {
                http.Error(w, "forbidden", 403)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

Pronto. 10 linhas. Sem Casbin, sem OPA, sem JSON de policies. `is_admin = true` no banco pras pessoas certas.

-----

## 6. Quando RBAC vira inegociável (backlog futuro)

Sinais claros pra adicionar RBAC de verdade:

- **Plano família/casal** (2+ números compartilhando finanças) → tabela `memberships` com roles `owner` / `member`
- **Painel admin com 3+ tipos de operador** (suporte L1, suporte L2, financeiro, dev) → role-based
- **API pública** pra integrações de terceiros → scopes/permissions (`read:transactions`, `write:transactions`)
- **White-label / multi-tenant** → roles dentro de `tenant_id`

Até lá: `is_admin bool` resolve 100%.

### Caminho de migração (quando a hora chegar)

Não destrói nada do que você fez:

```sql
-- Migration futura, não no MVP
CREATE TABLE roles (...);
CREATE TABLE user_roles (user_id, role_id);

-- Mantém is_admin durante a transição
INSERT INTO user_roles (user_id, role_id)
SELECT id, (SELECT id FROM roles WHERE name='admin')
FROM users WHERE is_admin = true;

-- Depois, em outra migration, dropa is_admin
ALTER TABLE users DROP COLUMN is_admin;
```

Custo total de migrar quando precisar: ~1 dia. Custo de carregar RBAC desnecessário no MVP: semanas de manutenção.

-----

## 7. Checklist production-proof do módulo de identidade

- [ ] `NormalizeWhatsAppBR` aplicada em **todos** os pontos de entrada (handler WhatsApp, magic token, admin manual, import CSV)
- [ ] Testes unitários da `NormalizeWhatsAppBR` cobrindo: com/sem +55, com/sem 9, com formatação, com espaços, input vazio
- [ ] Testes unitários da `IsEntitled` cobrindo **todas** as 6 transições de status × (dentro/fora do período)
- [ ] Índice único em `users.whatsapp_number` (constraint de banco, não só validação em código)
- [ ] Soft delete implementado e respeitado em **todas** as queries (`WHERE deleted_at IS NULL`)
- [ ] Cache de entitlement com TTL inteligente, **nunca** TTL fixo eterno
- [ ] Invalidação de cache no fim do `BillingEventProcessor` (depois do commit no Postgres)
- [ ] Negative cache pra “user sem subscription” (TTL 5min)
- [ ] Log estruturado: cada decisão de entitlement gera evento com `user_id`, `decision`, `reason`, `latency_ms`
- [ ] Métrica: `entitlement_check_total{decision="granted|denied", reason="..."}` no Prometheus
- [ ] Runbook: como dar entitlement manual (suporte resolve sem deploy)

-----

## 8. Tarefa de comando manual (suporte resolve sozinho)

Inevitável: cliente vai reclamar “paguei e não funciona”. Tenha desde o dia 1:

```go
// Comando admin: override de entitlement
type GrantEntitlementCmd struct {
    UserID    string
    Until     time.Time
    Reason    string // obrigatório, vai pro audit log
    GrantedBy string // admin que executou
}
```

Tabela de audit log:

```sql
CREATE TABLE entitlement_overrides (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id),
    granted_until TIMESTAMPTZ NOT NULL,
    reason      TEXT NOT NULL,
    granted_by  UUID NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at  TIMESTAMPTZ
);
```

`IsEntitled` consulta também essa tabela (override válido → libera mesmo sem subscription). Resolve o “paguei e não funciona” sem precisar arrumar o webhook na hora.

-----

## 9. Resumo executivo

|Pergunta                               |Resposta                                                                           |
|---------------------------------------|-----------------------------------------------------------------------------------|
|RBAC no MVP?                           |**Não**                                                                            |
|Módulo de identidade?                  |**Sim, mínimo** (users + histórico de número)                                      |
|Precisa pra linkar user ↔ subscription?|Não. FK `subscriptions.user_id` resolve.                                           |
|O que abstrair desde já?               |UUID como PK, normalização E.164, `is_admin` bool, soft delete                     |
|O que NÃO fazer?                       |Casbin, OPA, tabela de roles/permissions, policy engine                            |
|Onde gastar energia?                   |`IsEntitled` puro + testado, cache com TTL inteligente, override manual pra suporte|

**Regra do staff:** construa a abstração na segunda vez que a dor aparecer, não na primeira. RBAC é uma das abstrações que parecem baratas no diagrama e custam caro em manutenção, debug e onboarding.
