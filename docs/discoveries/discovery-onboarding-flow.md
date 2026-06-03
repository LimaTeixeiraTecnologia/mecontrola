# Discovery: Onboarding do MeControla (Landing → Checkout → WhatsApp)

> **Contexto:** Produto 100% no WhatsApp. Landing em <https://www.mecontrola.app.br>.
> **Stack:** Backend Go + Postgres + Redis + LLM + WhatsApp Business API.
> **Provider de pagamento:** Kiwify (decisão do doc de billing).
> **Princípio inegociável:** identidade do WhatsApp **garantida pelo próprio canal**, não pelo que o usuário digita no checkout.

-----

## 0. TL;DR — o fluxo escolhido

```
Landing → Checkout Kiwify → Thank-you page (sua) → wa.me com magic token → Bot ativa conta
```

Esse é o **único** fluxo que resolve simultaneamente:

- Baixa fricção de compra (checkout antes de qualquer fricção de bot)
- Identidade do WhatsApp **garantida** (o número que abre o `wa.me` é o número de verdade)
- Zero ambiguidade entre “número que pagou” e “número que vai usar”
- Fallback robusto se o user comprar e nunca abrir o WhatsApp

-----

## 1. Fluxo completo, passo a passo

```
[1] Landing mecontrola.app.br
      │  CTA "Assinar agora" (preço visível, sem fricção)
      │  POST /api/checkout-session → gera magic_token
      ▼
[2] Redirect pra checkout Kiwify
      │  URL: https://pay.kiwify.com.br/{produto}?s={magic_token}
      │  User preenche: email, WhatsApp (campo do Kiwify), cartão/Pix
      ▼
[3] Pagamento aprovado
      │  Kiwify dispara webhook compra_aprovada → seu backend
      │  Backend marca token como PAID
      │  Kiwify redireciona pra sua thank-you page
      ▼
[4] Thank-you page (mecontrola.app.br/obrigado?token={magic_token})
      │  Botão grande: "📱 Abrir WhatsApp e ativar"
      │  Link: https://wa.me/SEU_BOT?text=ATIVAR%20{magic_token}
      │  JS tenta auto-redirect em mobile após 2s
      ▼
[5] WhatsApp abre com mensagem pré-preenchida
      │  User só aperta enviar
      ▼
[6] Bot recebe "ATIVAR abc123"
      │  Resolve token → vincula user_id ↔ whatsapp_number_real
      │  Marca token como CONSUMED
      ▼
[7] Bot responde "✅ Pronto, manda seu primeiro gasto"
```

-----

## 2. Por que esse fluxo (e não os outros)

### ❌ Alternativa A: Landing → Checkout → email com link

Clássico de infoproduto. **Errado pro seu caso.**

- Email não é o canal do produto.
- User esquece de abrir, vai pro spam, ou clica do desktop e o `wa.me` não dispara o app.
- Quebra de contexto: paga no celular, recebe email, abre depois.

### ❌ Alternativa B: Landing → WhatsApp primeiro → bot manda link de pagamento

Funciona, mas adiciona fricção **antes** do pagamento. Conversão cai.

- Vale **apenas se** você oferecer trial grátis primeiro — aí faz sentido, porque o objetivo é experimentar antes de comprar.
- No MVP, conversão direta na landing > conversão via bot.

### ❌ Alternativa C: Confiar no número que o user digita no checkout da Kiwify

Frágil.

- User digita errado, formata diferente, usa o número da casa em vez do celular.
- Sem deep link `wa.me`, não há garantia de match.
- Vira fonte de tickets de suporte: “paguei e não funciona”.

### ✅ Fluxo escolhido: Checkout primeiro + deep link na thank-you page

- **Identidade inegociável:** o número que abre o chat **é** o número real, porque vem do próprio WhatsApp.
- **Magic token amarra tudo:** `compra_aprovada` chega no webhook com o token, primeira mensagem `ATIVAR {token}` chega com o número. Você cruza os dois lados.
- **Fricção mínima:** thank-you page é 1 clique pra abrir o WhatsApp, mensagem já pré-preenchida.
- **Independe de campo digitado**, mas usa esse campo como fallback de segurança.

-----

## 3. Modelagem do magic token

```sql
CREATE TABLE signup_tokens (
  token             TEXT PRIMARY KEY,         -- UUID v4, opaco
  plan_code         TEXT NOT NULL,
  status            TEXT NOT NULL,            -- PENDING | PAID | CONSUMED | EXPIRED
  whatsapp_input    TEXT,                     -- preenchido no webhook (digitado no checkout)
  whatsapp_real     TEXT,                     -- preenchido na ativação (do wa.me)
  email             TEXT,                     -- do checkout
  user_id           UUID REFERENCES users(id),
  subscription_id   UUID REFERENCES subscriptions(id),
  provider_order_id TEXT,                     -- ID da venda na Kiwify
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  paid_at           TIMESTAMPTZ,
  consumed_at       TIMESTAMPTZ,
  expires_at        TIMESTAMPTZ NOT NULL,     -- TTL 7 dias
  fallback_reason   TEXT                      -- preenchido se ativação foi via fallback
);
CREATE INDEX ON signup_tokens (status, expires_at);
CREATE INDEX ON signup_tokens (whatsapp_input) WHERE status = 'PAID';
CREATE INDEX ON signup_tokens (provider_order_id);
```

### Transições de estado

```
PENDING ──────► PAID ──────► CONSUMED
   │              │
   └──► EXPIRED   └──► EXPIRED  (job de cleanup)
```

- `PENDING` → criado no clique do “Assinar” na landing
- `PAID` → webhook `compra_aprovada` carimba `paid_at`
- `CONSUMED` → mensagem `ATIVAR` recebida, user vinculado
- `EXPIRED` → TTL passou sem virar `CONSUMED`

### Por que TTL de 7 dias (e não 30min)

User pode comprar no domingo à noite, só abrir o WhatsApp na segunda. **7 dias** é tempo seguro. Curto demais = aumenta uso do fallback. Longo demais = token vira passe livre. 7d é o sweet spot.

-----

## 4. Endpoints e código

### 4.1 Endpoint que a landing chama no botão “Assinar”

```go
// POST /api/checkout-session
type CheckoutSessionRequest struct {
    PlanCode string `json:"plan_code"` // "monthly", "annual"
}

type CheckoutSessionResponse struct {
    CheckoutURL string `json:"checkout_url"`
    Token       string `json:"token"`
    ExpiresAt   string `json:"expires_at"`
}

func (h *Handler) CreateCheckoutSession(w http.ResponseWriter, r *http.Request) {
    var req CheckoutSessionRequest
    json.NewDecoder(r.Body).Decode(&req)

    token := uuid.NewString()
    expiresAt := time.Now().Add(7 * 24 * time.Hour)

    err := h.repo.CreateSignupToken(r.Context(), &SignupToken{
        Token:     token,
        PlanCode:  req.PlanCode,
        Status:    "PENDING",
        ExpiresAt: expiresAt,
    })
    if err != nil {
        http.Error(w, "internal error", 500)
        return
    }

    checkoutURL := h.kiwify.CheckoutURL(req.PlanCode) + "?s=" + token

    json.NewEncoder(w).Encode(CheckoutSessionResponse{
        CheckoutURL: checkoutURL,
        Token:       token,
        ExpiresAt:   expiresAt.Format(time.RFC3339),
    })
}
```

### 4.2 JS da landing (botão “Assinar”)

```javascript
document.querySelector('#cta-assinar').addEventListener('click', async (e) => {
    e.preventDefault();
    const btn = e.currentTarget;
    btn.disabled = true;
    btn.textContent = 'Carregando...';

    try {
        const res = await fetch('/api/checkout-session', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ plan_code: 'monthly' })
        });

        if (!res.ok) throw new Error('falha ao criar sessão');
        const { checkout_url } = await res.json();
        window.location.href = checkout_url;
    } catch (err) {
        btn.disabled = false;
        btn.textContent = 'Assinar agora';
        alert('Erro ao iniciar pagamento. Tente novamente.');
    }
});
```

### 4.3 Webhook handler `compra_aprovada`

Já modelado no doc de billing, mas o trecho específico do onboarding:

```go
// Dentro do BillingEventProcessor, após persistir subscription:
func (p *Processor) handlePurchaseApproved(ctx context.Context, evt *BillingEvent) error {
    // 1. Extrai token do payload (custom field ou UTM)
    token := evt.CustomFields["s"]
    if token == "" {
        token = evt.UTM["s"]
    }

    // 2. Marca token como PAID
    if token != "" {
        err := p.repo.MarkTokenPaid(ctx, token, &TokenPaidInput{
            WhatsAppInput:   evt.BuyerPhone,
            Email:           evt.BuyerEmail,
            ProviderOrderID: evt.ExternalSubID,
            PaidAt:          evt.OccurredAt,
        })
        if err != nil && !errors.Is(err, ErrTokenNotFound) {
            return err
        }
    }

    // 3. NÃO cria user ainda — espera o ATIVAR no WhatsApp
    //    Subscription fica "órfã" temporariamente, vinculada via token

    return nil
}
```

**Importante:** o `user` só é criado quando o `ATIVAR` chega no WhatsApp. Antes disso, a `subscription` fica sem `user_id` (campo nullable temporariamente, ou em tabela `pending_subscriptions`).

### 4.4 Thank-you page (mecontrola.app.br/obrigado)

```html
<!DOCTYPE html>
<html lang="pt-BR">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>MeControla — Ativação</title>
    <style>
        body { font-family: -apple-system, sans-serif; max-width: 480px;
               margin: 0 auto; padding: 32px; text-align: center; }
        .cta { display: inline-block; background: #25D366; color: white;
               padding: 20px 40px; border-radius: 12px; font-size: 18px;
               text-decoration: none; margin: 24px 0; font-weight: 600; }
        .small { color: #666; font-size: 14px; margin-top: 16px; }
    </style>
</head>
<body>
    <h1>✅ Pagamento confirmado!</h1>
    <p>Falta só 1 passo: ativar sua conta no WhatsApp.</p>

    <a id="cta-whatsapp"
       class="cta"
       href="https://wa.me/SEU_NUMERO_BOT?text=ATIVAR%20{{.Token}}">
        📱 Abrir WhatsApp e ativar
    </a>

    <p class="small">
        Se o WhatsApp não abrir automaticamente, copie e cole no seu WhatsApp:<br>
        <code>ATIVAR {{.Token}}</code><br>
        e envie para o número <strong>+55 11 XXXXX-XXXX</strong>
    </p>

    <script>
        // Auto-redirect em mobile após 2s
        if (/iPhone|iPad|iPod|Android/i.test(navigator.userAgent)) {
            setTimeout(() => {
                document.getElementById('cta-whatsapp').click();
            }, 2000);
        }
    </script>
</body>
</html>
```

### 4.5 Handler do comando `ATIVAR` no WhatsApp

```go
// Dentro do roteador de mensagens do WhatsApp
func (h *WhatsAppHandler) handleAtivar(ctx context.Context, msg IncomingMessage) error {
    number, err := identity.NormalizeWhatsAppBR(msg.From)
    if err != nil {
        return h.reply(ctx, msg.From, "Número inválido.")
    }

    // Extrai token: "ATIVAR abc-123-def"
    parts := strings.Fields(msg.Body)
    if len(parts) < 2 {
        return h.reply(ctx, number, "Envie: ATIVAR seguido do código que você recebeu na página de pagamento.")
    }
    token := parts[1]

    // Resolve token
    st, err := h.repo.GetSignupToken(ctx, token)
    if errors.Is(err, ErrTokenNotFound) {
        return h.reply(ctx, number, "Código inválido. Confira o código na página de pagamento.")
    }
    if err != nil {
        return err
    }

    // Idempotência: token já consumido por esse mesmo número?
    if st.Status == "CONSUMED" && st.WhatsAppReal == number {
        return h.reply(ctx, number, "Sua conta já está ativa! Manda um gasto pra começar.")
    }
    if st.Status == "CONSUMED" {
        // Tentativa de reuso por outro número — bloqueia
        return h.reply(ctx, number, "Este código já foi usado por outra conta.")
    }
    if st.Status == "EXPIRED" || time.Now().After(st.ExpiresAt) {
        return h.reply(ctx, number, "Código expirado. Fale com o suporte.")
    }
    if st.Status != "PAID" {
        // Pagamento ainda não confirmou (race condition rara — Pix lento, etc.)
        return h.reply(ctx, number, "Pagamento ainda processando, tente em 1 minuto.")
    }

    // Cria user + vincula subscription, tudo numa transação
    err = h.repo.ActivateFromToken(ctx, &ActivationInput{
        Token:        token,
        WhatsAppReal: number,
        ActivatedAt:  time.Now(),
    })
    if err != nil {
        return err
    }

    // Invalida cache de entitlement (caso exista negative cache)
    h.entitlement.Invalidate(ctx, number)

    return h.reply(ctx, number,
        "✅ Conta ativada! Manda seu primeiro gasto, tipo:\n\n"+
        "_Almoço 35 no cartão_")
}
```

### 4.6 Transação atômica de ativação

```go
func (r *Repo) ActivateFromToken(ctx context.Context, in *ActivationInput) error {
    return r.db.WithTx(ctx, func(tx *sql.Tx) error {
        // 1. Lock no token
        var st SignupToken
        err := tx.QueryRowContext(ctx,
            `SELECT token, status, plan_code, email, subscription_id
             FROM signup_tokens
             WHERE token = $1 FOR UPDATE`, in.Token).
            Scan(&st.Token, &st.Status, &st.PlanCode, &st.Email, &st.SubscriptionID)
        if err != nil { return err }

        if st.Status != "PAID" {
            return fmt.Errorf("token status %s, expected PAID", st.Status)
        }

        // 2. Upsert do user
        var userID string
        err = tx.QueryRowContext(ctx,
            `INSERT INTO users (whatsapp_number, email)
             VALUES ($1, $2)
             ON CONFLICT (whatsapp_number) DO UPDATE
               SET email = COALESCE(users.email, EXCLUDED.email),
                   updated_at = now()
             RETURNING id`, in.WhatsAppReal, st.Email).Scan(&userID)
        if err != nil { return err }

        // 3. Vincula subscription ao user
        _, err = tx.ExecContext(ctx,
            `UPDATE subscriptions SET user_id = $1, updated_at = now()
             WHERE id = $2`, userID, st.SubscriptionID)
        if err != nil { return err }

        // 4. Marca token como CONSUMED
        _, err = tx.ExecContext(ctx,
            `UPDATE signup_tokens
             SET status = 'CONSUMED',
                 whatsapp_real = $1,
                 user_id = $2,
                 consumed_at = $3
             WHERE token = $4`,
            in.WhatsAppReal, userID, in.ActivatedAt, in.Token)
        return err
    })
}
```

-----

## 5. Fallback: “comprou e nunca abriu o WhatsApp”

Inevitável. Cliente paga, fecha o navegador, esquece. Você precisa de fallback.

### Job de outreach automático

Roda a cada hora. Pega tokens em `PAID` há mais de 2h:

```go
func (j *OnboardingJob) RecoverPendingActivations(ctx context.Context) error {
    tokens, err := j.repo.ListPaidUnconsumedTokens(ctx, 2*time.Hour)
    if err != nil { return err }

    for _, t := range tokens {
        if t.WhatsAppInput == "" { continue }

        number, err := identity.NormalizeWhatsAppBR(t.WhatsAppInput)
        if err != nil {
            log.Warn("invalid whatsapp_input", "token", t.Token)
            continue
        }

        // Manda template aprovado do WhatsApp Business
        err = j.whatsapp.SendTemplate(ctx, number, "activation_reminder", map[string]string{
            "token": t.Token,
        })
        if err != nil { log.Error("template send failed", "err", err) }

        j.repo.MarkOutreachSent(ctx, t.Token, time.Now())
    }
    return nil
}
```

### Template do WhatsApp Business (precisa pré-aprovar na Meta)

```
Olá! Seu pagamento do MeControla foi confirmado. 🎉

Pra começar a usar, responda esta mensagem com:

ATIVAR {{1}}

(ou clique aqui: https://wa.me/SEU_BOT?text=ATIVAR%20{{1}})
```

### Fallback do fallback: ativação por match de número

Se o user responder qualquer coisa (não só `ATIVAR`), do número que está em `whatsapp_input` de um token `PAID`:

```go
func (h *WhatsAppHandler) tryFallbackActivation(ctx context.Context, number string, msgBody string) (bool, error) {
    st, err := h.repo.FindPaidTokenByWhatsAppInput(ctx, number)
    if errors.Is(err, ErrTokenNotFound) {
        return false, nil // não tem token pendente, deixa fluxo normal seguir
    }
    if err != nil { return false, err }

    // Match de número: ativa mas loga como fallback
    err = h.repo.ActivateFromToken(ctx, &ActivationInput{
        Token:          st.Token,
        WhatsAppReal:   number,
        ActivatedAt:    time.Now(),
        FallbackReason: "phone_number_match",
    })
    if err != nil { return false, err }

    h.reply(ctx, number, "✅ Sua conta foi ativada! Pode mandar seus gastos.")
    return true, nil
}
```

**Por que esse fallback é seguro:** o `whatsapp_input` veio do checkout assinado pela Kiwify. Match de número normalizado E.164 com pagamento confirmado é evidência suficiente.

-----

## 6. Configuração na Kiwify (checklist manual)

No painel da Kiwify, no produto:

- [ ] Criar produto recorrente (mensal/anual)
- [ ] Configurar webhook apontando pra `https://api.mecontrola.app.br/webhooks/kiwify`
- [ ] Habilitar eventos: `compra_aprovada`, `subscription_renewed`, `subscription_late`, `subscription_canceled`, `compra_reembolsada`, `chargeback`
- [ ] **URL de redirect pós-pagamento**: `https://mecontrola.app.br/obrigado?token={s}` (verificar se Kiwify propaga query params)
- [ ] **Custom field oculto** (se Kiwify suportar) recebendo o `?s=` do checkout — garante que o token volta no webhook
- [ ] Token do webhook em variável de ambiente, **nunca** commitado
- [ ] Testar com compra real de R$ 1,00 (Pix) antes de subir produção

### Caveat sobre custom fields

Se a Kiwify não suportar custom field oculto preenchido via query string, o token precisa voltar via UTM:

```
?utm_source=app&utm_campaign={token}
```

E você lê de `utm_campaign` no webhook. Pior porque tracking blockers podem cortar, mas funciona.

-----

## 7. Armadilhas e como evitar

1. **`wa.me` não abre no desktop direto.** No desktop, abre WhatsApp Web mas pode não pré-preencher a mensagem. Solução: copy fallback visível na thank-you page (“ou copie: ATIVAR {token}”).
1. **Race condition: webhook chega depois do `ATIVAR`.** Pix demora segundos, mas existe. O bot responde “pagamento ainda processando, tente em 1 minuto”. Não falhe — apenas reagende mentalmente.
1. **Webhook não chega nunca.** Sua reconciliação periódica (já no doc de billing) pega isso.
1. **User compra 2x sem querer (clica duas vezes).** Você terá 2 tokens em `PAID`, 2 subscriptions. Quando ele mandar `ATIVAR {token1}`, ative só uma; quando ativar a segunda, detecte duplicidade e dispare alerta pra suporte reembolsar.
1. **Token usado em outro número.** Bloqueie + alerte suporte. É padrão de fraude (compra com cartão clonado, tenta ativar em outro número).
1. **`wa.me` com texto pré-preenchido cortando token.** WhatsApp limita ~~tamanho de texto pré-preenchido a algo como 4096 chars, então UUID de 36 chars está safe. Mas teste em iOS e Android.
1. **LGPD do `whatsapp_input`.** Esse campo é PII. Mascare em logs e ofereça mecanismo de deletion request.

-----

## 8. Métricas pra acompanhar (desde o dia 1)

- `checkout_session_created_total` — cliques no botão “Assinar”
- `checkout_session_paid_total` — pagamentos aprovados
- `activation_consumed_total{path="direct|fallback_match|outreach"}` — ativações por caminho
- `time_from_paid_to_consumed_seconds` — histograma de quanto user demora pra ativar
- `pending_paid_tokens_total` — gauge dos tokens `PAID` não consumidos (alerta se > N)

**Funil que você quer ver:**

```
checkout_session_created → 100%
checkout_session_paid    → conversão de pagamento (alvo: > 70%)
activation_consumed      → conversão de ativação (alvo: > 90% dos pagos)
first_message_processed  → engajamento real (alvo: > 80% dos ativados)
```

Se `activation_consumed / checkout_session_paid` < 80%, sua thank-you page tem problema. Investigue.

-----

## 9. Checklist production-proof

- [ ] Endpoint `/api/checkout-session` com rate limit (anti-abuse: 10/min por IP)
- [ ] Token sempre UUID v4 (opaco, não enumerável)
- [ ] TTL de 7 dias no token + job de cleanup diário
- [ ] Thank-you page hospedada por você, **não** a padrão da Kiwify
- [ ] Auto-redirect pra `wa.me` em mobile + fallback copy-paste visível
- [ ] Idempotência no handler `ATIVAR` (segundo envio do mesmo token = resposta amigável)
- [ ] Transação atômica `ActivateFromToken` com `FOR UPDATE` (sem race)
- [ ] Job de outreach pra tokens `PAID` há > 2h sem ativação
- [ ] Template de outreach aprovado na Meta (WhatsApp Business)
- [ ] Fallback de ativação por match de número (E.164)
- [ ] Métricas e alertas configurados
- [ ] PII mascarada em logs (`whatsapp_input`, `email`)
- [ ] Webhook da Kiwify em URL distinta por ambiente (dev/staging/prod)
- [ ] Testes E2E: compra de R$ 1 + ativação completa em staging
- [ ] Runbook: como suporte resolve “comprei e não consigo ativar” sem deploy

-----

## 10. Resumo executivo

|Decisão                             |Escolha                                                              |
|------------------------------------|---------------------------------------------------------------------|
|Onde começa o onboarding?           |Landing → Checkout Kiwify (compra primeiro)                          |
|Quem garante identidade do WhatsApp?|Deep link `wa.me` com magic token                                    |
|Thank-you page?                     |**Sua**, não a da Kiwify                                             |
|TTL do magic token?                 |7 dias                                                               |
|Fallback principal?                 |Outreach via WhatsApp Business Template + match por número           |
|User antes da ativação?             |Não existe ainda — subscription fica “órfã” até `ATIVAR`             |
|Trial grátis?                       |Se quiser, segundo CTA na landing vai direto pro `wa.me` sem checkout|

**Regra de ouro:** o **único** sinal confiável da identidade do WhatsApp é o próprio WhatsApp. Tudo que vem do checkout é **input não-verificado** e deve ser tratado como tal — útil pra fallback, nunca como fonte de verdade.
