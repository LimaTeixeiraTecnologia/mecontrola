# Validacao manual do fluxo onboarding + Kiwify

- Data-base do artefato: 2026-06-20
- Base URL default: `http://localhost:8080`
- Objetivo: validar `checkout -> webhook Kiwify simulado -> token pronto -> ativacao via WhatsApp`, sem compra real

## Arquivos

- Collection onboarding: `docs/postman/mecontrola-api.postman_collection.json`
- Collection completa: `docs/postman/mecontrola-api.completo.postman_collection.json`
- Environment template: `docs/postman/mecontrola-api.postman_environment.json`
- Environment local generico: `docs/postman/mecontrola-api.local.postman_environment.json`
- Perfil local Jailton: `docs/postman/mecontrola-api.postman_environment.jailton.json`
- Perfil local Stefany: `docs/postman/mecontrola-api.postman_environment.stefany.json`

## Segredos obrigatorios

Preencher no environment ativo antes de executar:

- `kiwify_webhook_secret`
- `meta_app_secret`
- `meta_verify_token`
- `gateway_secret` se quiser seguir depois para endpoints autenticados
- `whatsapp_phone_number_id`

Mapeamento com o `.env` local:

| Variavel no Postman | Origem no `.env` |
| --- | --- |
| `kiwify_webhook_secret` | `KIWIFY_WEBHOOK_SECRET` |
| `meta_app_secret` | `META_APP_SECRET` |
| `meta_verify_token` | `META_VERIFY_TOKEN` |
| `gateway_secret` | `IDENTITY_GATEWAY_SHARED_SECRET_CURRENT` |
| `whatsapp_phone_number_id` | `META_PHONE_NUMBER_ID` |

Use apenas o `.env` local como fonte de preenchimento e nao versione valores reais de secrets nos environments do Postman.

## Variaveis encadeadas automaticamente

Essas variaveis nao precisam de preenchimento manual no fluxo principal:

- `onboarding_checkout_url`
- `onboarding_token`
- `funnel_token`
- `expected_ready_to_activate`
- `wa_me_url`
- `kiwify_signature`
- `meta_signature`

## Perfis locais prontos

- Jailton
  - `user_email=jailton.junior94@outlook.com`
  - `user_whatsapp=+5511986896322`
  - `user_whatsapp_from=5511986896322`
- Stefany
  - `user_email=stefanykelly.lima@hotmail.com`
  - `user_whatsapp=+5511930111763`
  - `user_whatsapp_from=5511930111763`

## Ordem de execucao

1. Importar a collection e escolher um environment local.
2. Executar `GET /healthz` para confirmar a API no ar.
3. Executar `POST /api/v1/onboarding/checkout`.
4. Confirmar no Postman que `onboarding_checkout_url`, `onboarding_token` e `funnel_token` foram preenchidos.
5. Executar `GET /api/v1/onboarding/tokens/:token/state`.
6. Esperado antes do pagamento simulado:
   - `200 OK`
   - `ready_to_activate=false`
7. Executar `Kiwify — order_approved`.
8. Confirmar no payload do request que:
   - `Customer.email={{user_email}}`
   - `Customer.mobile={{user_whatsapp}}`
   - `TrackingParameters.sck={{funnel_token}}`
9. Esperado no webhook:
   - `202 Accepted`
   - `received=true`
10. Executar novamente `GET /api/v1/onboarding/tokens/:token/state`.
11. Esperado depois do pagamento simulado:
   - `200 OK`
   - `ready_to_activate=true`
   - `wa_me_url` preenchido
12. Executar `POST /api/v1/whatsapp/inbound — comando ATIVAR <token>`.
13. Esperado na ativacao:
   - `200 OK`
   - token consumido pelo numero `{{user_whatsapp_from}}`

## Checks de regressao

- Rodar `POST /api/v1/whatsapp/inbound invalid signature` e esperar `401`.
- Rodar `POST /api/v1/billing/webhooks/kiwify` com secret errado e esperar `401`.
- Reexecutar `Kiwify — order_approved` com o mesmo body apenas se quiser validar idempotencia do backend; para o fluxo principal, deixe o Postman gerar um novo `order_id` com `{{$timestamp}}`.

## Observacoes

- O token de onboarding agora e extraido automaticamente do `checkout_url?sck=...`.
- O webhook Kiwify principal usa o mesmo token do checkout via `{{funnel_token}}`; nao ha mais necessidade de editar `sck` manualmente.
- O request de WhatsApp usa `ATIVAR {{onboarding_token}}`; se o token nao existir, a collection falha no request anterior.
- O backend atual nao expoe webhook Telegram no workspace; a colecao completa foi reduzida ao fluxo real de WhatsApp.
