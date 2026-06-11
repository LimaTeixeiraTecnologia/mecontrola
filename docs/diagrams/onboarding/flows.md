# Onboarding Flows

![Onboarding container context](../system/mecontrola-container.svg)

## Objetivo do modulo

`internal/onboarding` gerencia a criacao de checkout session, o ciclo de vida dos magic tokens, a ativacao por WhatsApp, outreach proativo e o vinculo entre assinatura paga e usuario.

## Arquivos .puml por fluxo

- [ONB-01-create-checkout-session.puml](./ONB-01-create-checkout-session.puml)
- [ONB-02-token-state.puml](./ONB-02-token-state.puml)
- [ONB-03-activation-via-whatsapp.puml](./ONB-03-activation-via-whatsapp.puml)
- [ONB-04-fallback-activation.puml](./ONB-04-fallback-activation.puml)
- [ONB-05-mark-token-paid.puml](./ONB-05-mark-token-paid.puml)
- [ONB-06-paid-without-token.puml](./ONB-06-paid-without-token.puml)
- [ONB-07-outreach-job.puml](./ONB-07-outreach-job.puml)
- [ONB-08-token-expiration.puml](./ONB-08-token-expiration.puml)
- [ONB-09-meta-cleanup.puml](./ONB-09-meta-cleanup.puml)

## Entradas, saidas e artefatos

### Endpoints

- `POST /api/v1/onboarding/checkout`
- `GET /api/v1/onboarding/tokens/{token}/state`

### Entradas async

- Consumer `billing.subscription.activated`
- Consumer `billing.subscription.activated_without_token`
- Job `onboarding.outreach_job`
- Job `onboarding-token-expiration`
- Job `onboarding-meta-processed-cleanup`
- Mensagens roteadas do webhook `POST /api/v1/whatsapp/inbound`

### Saidas

- Escrita em `magic_tokens`, `support_signals`, `subscription bindings`
- Escrita em `outbox_events` de `onboarding.subscription_bound`
- Chamadas outbound para `Meta WhatsApp Cloud API`
- Chamada de gateway para `identity.UpsertUserByWhatsApp`

## Matriz de fluxos

| ID | Origem | Tipo | Saida principal |
| --- | --- | --- | --- |
| ONB-01 | `POST /api/v1/onboarding/checkout` | sync | Cria token e devolve checkout URL |
| ONB-02 | `GET /api/v1/onboarding/tokens/{token}/state` | sync | Informa readiness e monta wa.me URL |
| ONB-03 | `POST /api/v1/whatsapp/inbound` com `ATIVAR <token>` | sync + async | Consome token, upserta usuario e publica `onboarding.subscription_bound` |
| ONB-04 | `POST /api/v1/whatsapp/inbound` sem comando valido | sync | Tenta fallback activation e responde via Meta |
| ONB-05 | consumer `billing.subscription.activated` | async | Marca token como pago |
| ONB-06 | consumer `billing.subscription.activated_without_token` | async | Trata compra sem token/funnel |
| ONB-07 | `onboarding.outreach_job` | sync | Envia outreach via Meta |
| ONB-08 | `onboarding-token-expiration` | sync | Expira tokens vencidos |
| ONB-09 | `onboarding-meta-processed-cleanup` | sync | Limpa tabelas auxiliares da integracao Meta |

## Percurso detalhado

### ONB-01 - Criar checkout session

Origem:
- `CreateCheckoutHandler.Handle`

Percurso:
1. O router aplica:
   - rate limiter de checkout;
   - CORS restrito;
   - `AllowContentType("application/json")`.
2. O handler exige `plan_id`.
3. Chama `CreateCheckoutSession.Execute`.
4. O use case:
   - valida o plano;
   - gera token com `TokenCipher`;
   - cria `MagicToken` em UoW;
   - monta URL de checkout via `checkout.KiwifyURLBuilder`.
5. Responde `201` com `checkout_url`.

Banco:
- Escrita em `magic_tokens`

Sistema externo:
- Nao chama Kiwify em runtime; apenas gera URL a partir da configuracao.

### ONB-02 - Consultar estado do token

Origem:
- `TokenStateHandler.Handle`

Percurso:
1. O handler extrai `{token}`.
2. Chama `GetTokenState.Execute`.
3. O use case avalia se o token:
   - existe;
   - esta pago;
   - ainda nao foi consumido;
   - esta pronto para ativacao.
4. Se pronto:
   - devolve `ready_to_activate=true`;
   - monta `wa_me_url`;
   - informa `bot_number_display`.
5. Se nao pronto:
   - devolve `ready_to_activate=false`;
   - incrementa metrica de acesso invalido;
   - aplica pequeno jitter anti-enumeracao.

Banco:
- Leitura em `magic_tokens`

### ONB-03 - Ativacao via WhatsApp

Origem:
- `POST /api/v1/whatsapp/inbound` roteado para `WhatsAppMessageProcessor.HandleActivation`

Percurso:
1. O `Dispatcher` detecta `ATIVAR <token>`.
2. `WhatsAppMessageProcessor.HandleActivation` valida o numero E.164 do remetente.
3. Chama `ConsumeMagicToken.Execute` com:
   - token digitado;
   - `FromE164`;
   - `ActivationPathDirect`.
4. O use case de consumo:
   - abre transacao;
   - valida token e estado de pagamento;
   - chama `SubscriptionBindingService`.
5. `SubscriptionBindingService`:
   - usa `identityGateway.UpsertUserByWhatsApp`;
   - usa `subscriptionBinder` para associar assinatura e usuario;
   - publica `onboarding.subscription_bound` via outbox.
6. O processor envia mensagem de resposta ao usuario pela `WhatsAppGateway`, que delega para `Meta client`.

Banco:
- Leitura/escrita em `magic_tokens`
- Escrita em `subscription bindings`
- Escrita em `outbox_events`

Sistemas externos:
- `Meta WhatsApp Cloud API` para mensagem de confirmacao

### ONB-04 - Fallback activation

Origem:
- `POST /api/v1/whatsapp/inbound` sem comando `ATIVAR`

Percurso:
1. O dispatcher chama `WhatsAppMessageProcessor.HandleFallback`.
2. O processor valida o numero.
3. Chama `TryFallbackActivation.Execute`.
4. O use case tenta inferir se ja existe token pago elegivel para o numero.
5. Se ativar com sucesso, envia mensagem `welcome_activated`.
6. Se nao, envia mensagem orientando o uso do comando `ATIVAR`.

### ONB-05 - Token marcado como pago

Origem:
- Consumer `billing.subscription.activated`

Percurso:
1. `SubscriptionPaidConsumer.Handle` desserializa o envelope de billing.
2. Se `funnel_token` vier vazio, registra warning e encerra.
3. Chama `MarkTokenPaid.Execute` com:
   - `subscription_id`
   - `funnel_token`
   - `customer_mobile_e164`
   - `customer_email`
   - `external_sale_id`
   - `paid_at`
4. O use case move o token para estado pago e associa metadados da assinatura.

Banco:
- Escrita em `magic_tokens`

### ONB-06 - Compra sem token

Origem:
- Consumer `billing.subscription.activated_without_token`

Percurso:
1. `PaidWithoutTokenConsumer.Handle` desserializa o payload.
2. Chama `HandlePaidWithoutToken.Execute`.
3. O use case registra sinal de suporte ou trilha de follow-up para compra sem token original.

Banco:
- Escrita em `support_signals` e tabelas auxiliares do onboarding

### ONB-07 - Outreach

Origem:
- Job `onboarding.outreach_job`
- Schedule fixo: `5 * * * *`

Percurso:
1. Se `OutreachEnabled=false`, o job sai sem efeito.
2. Se habilitado, `SendOutreach.Execute` busca tokens elegiveis e sinais pendentes.
3. O use case usa `WhatsAppGateway.SendTextMessage`.
4. O gateway chama o client HTTP da Meta via `internal/platform/httpclient`.

### ONB-08 - Expiracao de tokens

Origem:
- Job `onboarding-token-expiration`
- Schedule: `cfg.TokenExpirationSchedule`

Percurso:
1. O job chama `ExpireTokens.Execute`.
2. O use case encontra tokens vencidos e altera seu estado.

Banco:
- Escrita em `magic_tokens`

### ONB-09 - Limpeza Meta processed

Origem:
- Job `onboarding-meta-processed-cleanup`
- Schedule: `cfg.MetaCleanupSchedule`

Percurso:
1. O job chama `CleanupOnboardingTables.Execute`.
2. O use case remove registros antigos ligados a processamento da integracao Meta.

Banco:
- Escrita/purga em tabelas de limpeza do onboarding

## Rotas internas e dependencias cruzadas

- Consome `billing.subscription.activated` e `billing.subscription.activated_without_token`.
- Usa `identity` como gateway de upsert de usuario.
- Publica `onboarding.subscription_bound`, consumido por `identity`.
- Compartilha a borda HTTP de WhatsApp com `identity`, mas a logica de onboarding vive no processor/use cases do modulo.

## Observacoes arquiteturais

- O pagamento e a ativacao sao deliberadamente separados: billing marca pago; onboarding consome o token depois.
- O binding entre assinatura e usuario e assinado por outbox para sobreviver a falhas apos a persistencia transacional.
- O envio outbound para Meta sempre passa pelo wrapper `internal/platform/httpclient`.

## Eficiencia, robustez e operacao

- `Caminho critico`
  - ONB-03 e o fluxo mais sensivel: token lookup, binding, gateway de identity e resposta outbound para Meta;
  - ONB-07 depende de scanning eficiente de candidatos de outreach.
- `Controles de robustez`
  - rate limiter nos endpoints HTTP publicos;
  - token cifrado e validado antes do bind;
  - separacao entre `mark paid` e `consume token`;
  - outbox para `onboarding.subscription_bound`;
  - fallback controlado para compras sem token.
- `Falhas esperadas`
  - token invalido/expirado/nao pago: falha definitiva de negocio com resposta apropriada ao usuario;
  - falha ao enviar mensagem Meta: falha transiente de side-effect, preservando estado ja persistido;
  - falha no gateway de identity ou no bind: falha transiente, sem concluir ativacao;
  - backlog de outreach ou sinais sem consumo: alerta operacional.
- `Observabilidade`
  - gauge de `tokens_paid_unconsumed`;
  - counters de checkout criado, checkout rate-limited, invalid access e confirmation failed;
  - logs estruturados no processor e nos jobs;
  - monitorar envelhecimento de `magic_tokens` pagos e nao consumidos.
- `Capacidade`
  - ONB-03 e bound por banco + API Meta;
  - outreach e bound por throughput externo da Meta e volume de candidatos;
  - cleanup e expiration precisam acompanhar crescimento historico das tabelas auxiliares.

## Guardrails operacionais

### Precondicoes e pos-condicoes

- `ONB-01`
  - pre: plano conhecido e chave de cifra valida;
  - pos: token persistido e URL de checkout retornada.
- `ONB-03`
  - pre: token existente, pago e ainda nao consumido; gateway de identity operacional;
  - pos: usuario associado, binding persistido, evento `onboarding.subscription_bound` emitido e mensagem de retorno enviada.
- `ONB-05/06`
  - pre: evento de billing valido no outbox;
  - pos: token marcado como pago ou sinal de suporte persistido.

### Invariantes

- token nao pode ser consumido com sucesso duas vezes para contas distintas;
- pagamento nao implica ativacao imediata sem consumo do token;
- `subscription_bound` so deve ser publicado apos persistencia do bind.

### Runbook resumido

- tokens pagos acumulando sem consumo:
  - monitorar gauge `tokens_paid_unconsumed`;
  - verificar falhas no inbound WhatsApp, outreach ou suporte.
- ativacao falhando:
  - validar `TokenCipher`, lookup do token, gateway de identity e resposta Meta;
  - inspecionar se houve persistencia parcial sem publish no outbox.
- outreach parado:
  - checar flag `OutreachEnabled`;
  - validar schedule e erros de throughput/autorizacao na Meta.

### Sinais e thresholds recomendados

- alerta se `onboarding_tokens_paid_unconsumed` ultrapassar o baseline esperado por tempo prolongado;
- alerta se `onboarding_confirmation_failed_total` subir acima de zero de forma sustentada;
- alerta se `ty_page_invalid_access_total` indicar probing ou UX quebrada;
- alerta se jobs de expiration/cleanup atrasarem mais de um ciclo.
