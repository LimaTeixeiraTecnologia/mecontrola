# Identity Flows

![Identity container context](../system/mecontrola-container.svg)

## Objetivo do modulo

`internal/identity` gerencia usuarios, estabelecimento de principal, projections de assinatura, trilha de eventos de autenticacao e a borda HTTP do webhook de WhatsApp compartilhada com `onboarding`.

## Arquivos .puml por fluxo

- [IDN-01-upsert-user.puml](./IDN-01-upsert-user.puml)
- [IDN-02-whatsapp-verify.puml](./IDN-02-whatsapp-verify.puml)
- [IDN-03-whatsapp-inbound-routing.puml](./IDN-03-whatsapp-inbound-routing.puml)
- [IDN-04-billing-subscription-projection.puml](./IDN-04-billing-subscription-projection.puml)
- [IDN-05-onboarding-subscription-bound.puml](./IDN-05-onboarding-subscription-bound.puml)
- [IDN-06-auth-events-consumer.puml](./IDN-06-auth-events-consumer.puml)
- [IDN-07-auth-events-housekeeping.puml](./IDN-07-auth-events-housekeeping.puml)

## Entradas, saidas e artefatos

### Endpoints

- `POST /api/v1/identity/users/`
- `GET /api/v1/whatsapp/verify`
- `POST /api/v1/whatsapp/inbound`

### Entradas async

- Consumer `billing.subscription.activated`
- Consumer `billing.subscription.renewed`
- Consumer `billing.subscription.past_due`
- Consumer `billing.subscription.canceled`
- Consumer `billing.subscription.refunded`
- Consumer `onboarding.subscription_bound`
- Consumer `auth.principal_established`
- Consumer `auth.failed`
- Consumer `auth.unknown_user`
- Consumer `user.deleted`
- Job `identity-auth-events-housekeeping`

### Saidas

- Escrita em `users`, `entitlements`, `subscription projection`, `auth_events`
- Escrita em `whatsapp dedup`
- Escrita em `outbox_events` de `auth.failed`, `auth.principal_established` e `user.deleted`

## Matriz de fluxos

| ID | Origem | Tipo | Saida principal |
| --- | --- | --- | --- |
| IDN-01 | `POST /api/v1/identity/users/` | sync | Upsert de usuario por WhatsApp |
| IDN-02 | `GET /api/v1/whatsapp/verify` | sync | Handshake com Meta |
| IDN-03 | `POST /api/v1/whatsapp/inbound` | sync + async | Dedup, establish principal e roteamento para onboarding ou agent |
| IDN-04 | consumer `billing.subscription.*` | async | Atualiza projection de assinatura/entitlement |
| IDN-05 | consumer `onboarding.subscription_bound` | async | Atualiza projection de vinculo onboarding-assinatura |
| IDN-06 | consumer `auth.*` e `user.deleted` | async | Grava ou anonimiza auth_events |
| IDN-07 | `identity-auth-events-housekeeping` | sync | Limpa auth_events antigos |

## Percurso detalhado

### IDN-01 - Upsert de usuario

Origem:
- `UpsertUserByWhatsAppHandler.Handle`

Percurso:
1. O handler decodifica `whatsapp`, `email`, `display_name`.
2. Chama `UpsertUserByWhatsApp.Execute`.
3. O use case valida VOs de WhatsApp e email.
4. Usa `UserRepository` dentro de UoW para criar ou atualizar o usuario.
5. Responde o DTO de usuario consolidado.

Banco:
- Leitura/escrita em `users`

### IDN-02 - Verificacao do webhook Meta

Origem:
- `GET /api/v1/whatsapp/verify`

Percurso:
1. `VerifyHandler.Handle` compara `hub.verify_token`.
2. Se valido, devolve o challenge.
3. Nenhuma persistencia e feita.

### IDN-03 - Mensagem inbound do WhatsApp

Origem:
- `POST /api/v1/whatsapp/inbound`

Percurso:
1. `WhatsAppWebhookRouter` aplica `signature.Compose` para validar assinatura da Meta e preservar o raw body.
2. `InboundHandler.Handle` extrai o corpo bruto do contexto.
3. Chama `Dispatcher.Route`.
4. O dispatcher:
   - extrai a primeira mensagem do payload Meta;
   - grava `wamid` em `WhatsAppDedupRepository.InsertIfAbsent`;
   - se duplicado, encerra como `OutcomeDuplicate`;
   - detecta comando `ATIVAR <token>` por regex.
5. Se for comando `ATIVAR`, envia para `onboardingRoute`, sem establish principal.
6. Se nao for comando:
   - chama `EstablishPrincipal.Execute` com o numero de WhatsApp;
   - se o usuario for desconhecido, envia para `onboardingRoute`;
   - se houver principal valido, consulta `WhatsAppLimiter.Allow`.
7. Se o rate limit falhar:
   - publica `auth.failed` via outbox;
   - encerra como `OutcomeRateLimited`.
8. Se o principal for valido:
   - injeta principal no contexto;
   - envia para `agentRoute`.
9. O handler HTTP responde `200` em sucesso logico e `503` quando o dispatcher falha de forma a pedir retry da Meta.

Banco:
- Escrita em `whatsapp dedup`
- Leitura/escrita em `users` para establish principal
- Escrita em `outbox_events` para `auth.failed` e eventos de autenticacao correlatos

Direcionamento:
- `onboardingRoute` chama `onboarding.WhatsAppMessageProcessor`
- `agentRoute` chama `agent.NewStubAgent(...).HandleMessage`

### IDN-04 - Projection de assinatura vinda de billing

Origem:
- Consumers do worker para `billing.subscription.*`

Percurso:
1. `SubscriptionEventProjector.Handle` recebe `outbox.Envelope`.
2. Encaminha `event_type` e `payload` para `ProjectSubscriptionEvent.Execute`.
3. O use case atualiza projection local de subscription e entitlement.

Banco:
- Escrita em `subscription projection`
- Escrita eventual em `entitlements`

### IDN-05 - Projection de bind de onboarding

Origem:
- Consumer `onboarding.subscription_bound`

Percurso:
1. `SubscriptionBoundProjector.Handle` desserializa o envelope.
2. Encaminha para `ProjectSubscriptionEvent.Execute`.
3. O use case consolida o vinculo entre usuario e assinatura dentro da projection local.

### IDN-06 - Trilha de autenticacao

Origem:
- Consumers `auth.principal_established`, `auth.failed`, `auth.unknown_user`, `user.deleted`

Percurso:
1. `AuthEventsConsumer.Handle` separa o percurso por `event_type`.
2. Para `auth.principal_established`, `auth.failed`, `auth.unknown_user`:
   - chama `ProjectAuthEvent.Execute`;
   - grava linha em `auth_events`.
3. Para `user.deleted`:
   - chama `AnonymizeUserAuthEvents.Execute`;
   - anonimiza eventos historicos do usuario.

Banco:
- Escrita/anonimizacao em `auth_events`

### IDN-07 - Housekeeping

Origem:
- Job `identity-auth-events-housekeeping`
- Schedule: `cfg.IdentityConfig.AuthEventsHousekeepingSchedule` ou `@monthly`

Percurso:
1. O job chama `CleanupAuthEvents.Execute`.
2. Remove ou arquiva eventos fora da retencao configurada.

## Rotas internas e dependencias cruzadas

- `identity` alimenta `onboarding` com `UpsertUserByWhatsApp` via adapter de gateway.
- `identity` recebe eventos de `billing` e `onboarding`.
- A borda de WhatsApp HTTP e registrada por `cmd/server/composeWhatsAppWebhookRouter`, combinando `identity` e `onboarding`.

## Observacoes arquiteturais

- A deduplicacao de `wamid` protege o bot contra retries da Meta.
- O roteamento onboarding vs agent e decidido no dispatcher, nao no handler HTTP.
- O modulo usa outbox para eventos de auth, preservando rastreabilidade mesmo em falhas posteriores do worker.

## Eficiencia, robustez e operacao

- `Caminho critico`
  - IDN-03 e o fluxo mais sensivel, combinando verificacao de assinatura, dedup, leitura de usuario e rate limiting.
- `Controles de robustez`
  - assinatura Meta na borda;
  - deduplicacao por `wamid`;
  - rate limit por `user_id`;
  - outbox para `auth.failed` e demais eventos de auth;
  - consumers separados para projection e trilha de auth.
- `Falhas esperadas`
  - payload Meta invalido: falha definitiva logica, sem processamento de negocio;
  - erro ao gravar dedup ou ao estabelecer principal: falha transiente, handler devolve `503` para permitir retry da Meta;
  - duplicate delivery: encerramento idempotente;
  - evento `user.deleted` reprocessado: comportamento no-op esperado.
- `Observabilidade`
  - counters de rota do dispatcher e rate-limit hits;
  - logs estruturados com `wa_id_masked`, `wamid` e outcome;
  - trilha `auth_events` serve como auditoria operacional e de seguranca.
- `Capacidade`
  - webhook inbound e bound por IO de banco em dedup + principal lookup;
  - crescimento de `auth_events` e `subscription projection` exige housekeeping e retenção.

## Guardrails operacionais

### Precondicoes e pos-condicoes

- `IDN-03`
  - pre: assinatura Meta valida, acesso ao storage de dedup e identidade disponivel;
  - pos: mensagem roteada uma unica vez para onboarding ou agent, ou rejeitada para retry controlado.
- `IDN-04/05/06`
  - pre: envelope valido no outbox;
  - pos: projection ou trilha de auth atualizada de forma idempotente.

### Invariantes

- um `wamid` nao pode produzir processamento de negocio duas vezes;
- `auth_events` deve permanecer anonimizavel por `user.deleted`;
- projections de subscription precisam refletir ordem efetiva dos eventos aceitos.

### Runbook resumido

- Meta recebendo 503 frequente:
  - validar storage de dedup e DB principal;
  - verificar falhas em `EstablishPrincipal`;
  - amostrar payloads invalidos e latencia do dispatcher.
- backlog de auth events:
  - checar consumidor e housekeeping;
  - medir crescimento de `auth_events` versus retenção.
- onboarding ou agent nao recebem mensagens:
  - inspecionar outcome do dispatcher;
  - validar regex do comando `ATIVAR` e limiter.

### Sinais e thresholds recomendados

- alerta se `auth_rate_limit_hits_total` explodir fora do comportamento esperado;
- alerta se `whatsapp_dispatcher_route_total{outcome=invalid}` subir abruptamente;
- alerta se duplicatas de `wamid` subirem muito, indicando retry excessivo da Meta;
- alerta se housekeeping mensal nao reduzir volume de `auth_events`.
