# ADR-010 — Confirmação pós-ATIVAR via texto livre dentro da janela 24h Meta + mapeamento `plan_id → URL` em env

## Metadados

- **Título:** Confirmação ao cliente após ativação e configuração de URLs de checkout
- **Data:** 2026-06-06
- **Status:** Aceita
- **Decisores:** PO (jailton), arquitetura (AI)
- **Relacionados:** `.specs/prd-onboarding-magic-token/techspec.md` §6.5, §6.8, RF-06, RF-07, RF-08; [ADR-005](./adr-005-whatsapp-meta-cloud-api-direct.md)

## Contexto

Duas decisões operacionais residuais materiais ao MVP:

### A) Confirmação ao cliente após ATIVAR
RF-06 exige resposta amigável em cada estado. RF-07 implica ativação atômica + comunicação ao cliente. Meta Cloud API permite **texto livre** apenas dentro da janela de 24h da última mensagem do cliente; fora dela, exige **template aprovado**.

Quando o cliente envia `ATIVAR <token>`, a janela de 24h é aberta automaticamente. Igualmente, no fallback E.164 (RF-10), o cliente envia uma mensagem qualquer antes da ativação — janela aberta.

### B) Mapeamento `plan_id` → URL de checkout Kiwify
Cada produto na Kiwify gera 1 URL pública distinta (`https://pay.kiwify.com.br/<short>`). E2 seedou 3 planos (`billing.plans`) com UUIDs estáveis. A landing precisa receber, para cada plano, uma URL Kiwify pronta com o `?sck={token}` apensado.

## Decisão

### A) Texto livre dentro de 24h
A confirmação ao cliente após ativação bem-sucedida é enviada como:

```http
POST https://graph.facebook.com/v18.0/{phone_number_id}/messages
{ "messaging_product":"whatsapp",
  "to":"<E.164 sem +>",
  "type":"text",
  "text":{ "body":"<copy fixo configurável em WhatsAppConfig.ActivationMessages>" } }
```

Copy:
- `welcome_activated` (estado `direct` ou `fallback_e164`): "Conta ativada com sucesso! 🎉 Pra começar, é só me contar seu primeiro gasto. Ex: *almoço 35*."
- `already_active` (RF-08 idempotência): "Sua conta já está ativa. Pode me contar seu próximo gasto quando quiser."
- `payment_still_processing_retry`: "Recebi seu pedido! Estamos confirmando seu pagamento. Tente novamente em 1 minuto, por favor."
- `code_expired_contact_support`: "Esse código expirou. Fale com a gente em contato@mecontrola.app.br pra resolvermos."
- `code_already_used_other_account`: "Esse código já foi usado em outra conta. Se acha que é engano, fale com a gente em contato@mecontrola.app.br."
- `code_invalid_check_again`: "Não encontrei esse código. Confere se copiou direitinho da página de pagamento."
- `invalid_country` (RF-16): "No momento atendemos apenas números do Brasil. Pra dúvidas, fale com a gente em contato@mecontrola.app.br."
- `please_use_ativar_command` (RF-10 sem outreach): "Olá! Pra ativar sua conta, envie a mensagem: ATIVAR seu-código-aqui (você encontra o código na página que abriu após o pagamento)."

Falha de envio é **best-effort não bloqueante**: transação principal (UoW) **já comitou** com o estado correto; falha aqui vira `slog.Warn(...)` + `onboarding_confirmation_failed_total` + tracing span de erro. Cliente está ativo no DB; próxima mensagem dele reabre a conversa.

Copy é **configurável via env** (`WhatsAppConfig.ActivationMessages map[string]string`), permitindo ajuste de tom sem deploy.

### B) `OnboardingConfig.KiwifyCheckoutURLs`
Mapa `map[uuid.UUID]string` carregado de env:

```bash
ONBOARDING_KIWIFY_CHECKOUT_URLS="11111111-1111-1111-1111-111111111111=https://pay.kiwify.com.br/abc123,22222222-2222-2222-2222-222222222222=https://pay.kiwify.com.br/def456,33333333-3333-3333-3333-333333333333=https://pay.kiwify.com.br/ghi789"
```

Parser em `configs/config.go` valida ao boot:
- cada entrada é `<uuid>=<url>`;
- `<uuid>` é UUID válido;
- `<url>` é HTTPS;
- host de `<url>` ∈ `OnboardingConfig.AllowedCheckoutHosts` (default `pay.kiwify.com.br`).

Falha de validação no boot → app não sobe (fail fast).

`CheckoutURLBuilder.Build(ctx, plan_id, token)`:
1. Lookup `cfg.KiwifyCheckoutURLs[plan_id]` → erro `ErrCheckoutUnavailable` se ausente.
2. Re-valida host (defesa em profundidade contra config mutável em hot reload futuro).
3. Apensa `?sck={token}` preservando query existente.
4. Retorna URL pronta para a landing.

No MVP, a landing envia apenas o `plan_id` do Mensal; demais planos ficam habilitáveis sem mudança de código nem contrato HTTP.

## Alternativas Consideradas

### Para (A)
1. **Template `activation_success` aprovado pela Meta.** Recusada — dobra dependência S-04 sem benefício (janela 24h está aberta); custo extra de aprovação por template; tempo de aprovação imprevisível.
2. **Sem confirmação imediata.** Recusada — viola UX "1 envio para ativar"; cliente não tem feedback de sucesso até mandar próxima mensagem.

### Para (B)
1. **Coluna `checkout_url` em `billing.plans`.** Recusada — E3 lendo schema E2 fere fronteira; migration extra em E2.
2. **Endpoint sem `plan_id` (1 plano hardcoded).** Recusada — breaking change quando habilitar planos múltiplos.
3. **Landing envia `checkout_url` completa para o backend apensar `?sck=`.** Recusada — superfície de injeção; valida domain mas dispersa config entre landing e backend.

## Consequências

### Benefícios
- Confirmação imediata sem custo extra de template Meta.
- Mensagens ajustáveis sem deploy (env).
- Configuração de planos centralizada no backend, escalável de 1 → N sem código novo.
- Defesa contra config envenenada (host allow-list).

### Trade-offs
- Falha rara de envio Meta deixa cliente sem confirmação visual (mitigação: log + métrica + próxima interação reabre conversa).
- Adicionar novo plano = atualizar env + reiniciar (sem hot reload). Aceitável para MVP.

### Riscos e Mitigações
- **R:** Janela 24h fecha entre o `ATIVAR` e o send (rede lenta, fila interna). **M:** Confirmação enviada **dentro da mesma goroutine** após COMMIT do UoW, sem fila. Latência típica < 1s.
- **R:** Copy mal configurada em env (vazia) → mensagem em branco para cliente. **M:** Validação no boot: `WhatsAppConfig.ActivationMessages[k]` exigido para chaves enumeradas; ausência → fail fast.
- **R:** `KiwifyCheckoutURLs` aponta para URL maliciosa por engano de operador. **M:** Host allow-list valida no parse e no `Build`.
- **R:** UUID em env mistura plano com staging/prod (mesmo UUID, URLs distintas). **M:** Configuração por env file separado por ambiente; teste de smoke pós-deploy compara hash da URL retornada contra esperado.

## Plano de Implementação
1. `WhatsAppConfig.ActivationMessages` parseado de env (formato `chave=texto` separado por `\n`).
2. `WhatsAppClient.SendText(ctx, to, body)` novo método.
3. `ConsumeMagicToken` chama `SendText` após COMMIT; erro vira log + métrica.
4. `OnboardingConfig.KiwifyCheckoutURLs` parseado em `configs/config.go`.
5. `CheckoutURLBuilder` em `internal/onboarding/infrastructure/checkout/kiwify_url_builder.go`.
6. Test unitário cobrindo: build com `plan_id` válido, build com `plan_id` desconhecido, host inválido, query existente preservada.

## Monitoramento
- `onboarding_confirmation_failed_total{reason}` (4xx/5xx Meta/timeout).
- `onboarding_checkout_unknown_plan_total` para diagnóstico de mismatch landing↔backend.
