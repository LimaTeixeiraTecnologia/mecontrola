<!-- spec-hash-prd: 1f811a026ec03efc9d4fb146425f9642bebbbbbdac51b3db2336b1c909f5076d -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Jornada de Ativação via WhatsApp

## Resumo Executivo

A jornada de ativação já possui ~80% das peças no código (Activation Session = `magic_token`, e-mail, página `/ativar`, `ConsumeMagicToken`, `SubscriptionBindingService`), mas a integração de consumo da ativação por WhatsApp está **0% wired em produção** e a UX exige código visível. Esta spec fecha a lacuna e remove a fricção com mudanças cirúrgicas, **reaproveitando os primitivos existentes** sem reescrever domínio.

Três decisões estruturais (cada uma com ADR): **(1)** a ativação por telefone é **event-driven** — o `dispatcher` (adapter de plataforma, fino) publica um evento `onboarding.activation.attempted.v1` quando a mensagem vem de número não vinculado, e um **consumer do onboarding** consome esse evento e orquestra a ativação (preserva R-ADAPTER-001 e o layering `platform → onboarding`, espelhando o `agentRoute` já existente); **(2)** a **janela de ativação de 24h** é avaliada a partir de `paidAt` como **guard de domínio puro** + uma **nova query** por telefone, sem alterar a semântica de `expiresAt` (criado no checkout) nem o job de expiração; **(3)** e-mail e página passam a apontar para `/ativar` com mensagem **"Ativar o meu plano"**, correlação por telefone normalizado E.164, com fallback por token apenas no caso de borda sem telefone, e **Telegram removido**.

A modelagem segue DMMF: estados como tipos fechados (`TokenStatus`, `ActivationPath`, `ConsumeOutcome` já existem), `Decide*`/guards puros no domínio, `Execute` como pipeline de passos privados (Princípio 5), pure-core/IO-shell (Princípio 6), smart constructors para o novo VO de telefone (Princípio 1/2).

## Arquitetura do Sistema

### Visão Geral dos Componentes

**Novos componentes:**

- `internal/platform/phone` (novo pacote) — smart constructor único de normalização E.164 BR. Fonte de verdade compartilhada por `identity` e `billing` (elimina divergência de formato Kiwify×Meta).
- `internal/onboarding/application/usecases/activate_from_inbound.go` (novo) — usecase que orquestra a ativação a partir da primeira mensagem: correlação por telefone (janela 24h) → fallback por token transicional (borda) → bind+consume → no-match com throttle. **Não envia boas-vindas** (desacopladas). Pipeline DMMF.
- `internal/onboarding/infrastructure/messaging/database/consumers/welcome_consumer.go` (novo) — consumer idempotente de `onboarding.subscription_bound` que envia as 2 mensagens de boas-vindas (`welcome_activated` + `onboarding_intro`) com retry próprio; desacopla entrega de ativação. Adapter fino.
- `internal/onboarding/application/dtos/input/activate_from_inbound_input.go` (novo) — DTO com `Validate()` (R-DTO-VALIDATE-001).
- `internal/onboarding/infrastructure/messaging/database/consumers/activation_attempt_consumer.go` (novo) — adapter fino que consome `onboarding.activation.attempted.v1` e delega ao usecase.
- Query nova `FindActivableByMobile` no repositório de `magic_token`.
- Domain guard puro novo `MagicToken.IsActivationWindowOpen(now, window)`.
- Store durável de throttle de no-match (`onboarding_activation_nomatch_throttle`) — espelha o padrão `internal/platform/whatsapp/dedup` (InsertIfAbsent por `(mobile_e164, window)` + housekeeping job). Robusto entre instâncias/reinícios e torna a resposta de no-match idempotente sob reentrega do evento.
- Endpoint beacon de jornada (`POST /api/v1/onboarding/tokens/{token}/opened`) + usecase `RecordJourneyTimestamp` — registra `page_opened_at`/`whatsapp_opened_at` (RF-35), fora do caminho de leitura `GET /state`.

**Componentes modificados:**

- `internal/platform/whatsapp/dispatcher/dispatcher.go` — remove o ramo `MatchActivationCommand` (RF-29); no ramo `ErrUnknownUser`, invoca um callback injetado `activationRoute` (publica o evento). Continua genérico.
- `internal/onboarding/module.go` — constrói o usecase, o consumer, expõe o callback `WhatsAppActivationRoute` e registra o `EventHandler`.
- `cmd/server/whatsapp_wiring.go` — injeta `WhatsAppActivationRoute` no `dispatcher.New(...)`.
- `internal/onboarding/application/usecases/consume_magic_token.go` — passa a exigir também `IsActivationWindowOpen` (24h de `paidAt`) além de `!IsExpiredAt`, unificando a janela nos dois caminhos de correlação (telefone e token).
- `internal/onboarding/infrastructure/repositories/postgres/magic_token_repository.go` (`UpdateMarkConsumed`) — endurecido: checar `RowsAffected == 0` (perda da corrida `WHERE status='PAID'`) e sinalizar `AlreadyActive` em vez de publicar segundo `subscription_bound` (correção de concorrência multi-instância).
- `internal/onboarding/application/usecases/send_activation_email.go` / `activation_email_consumer.go` — destino do CTA passa a ser `${ActivationPageURL}/ativar?token=...`; **suprime envio** quando a assinatura já está vinculada a um usuário (recompra de usuário já ativo), além do skip atual por token CONSUMED/EXPIRED — evita link inócuo.
- `internal/onboarding/application/usecases/get_token_state.go` — `wa_me_url` com `Ativar o meu plano` (telefone presente) ou token cru (borda sem telefone); sem `ATIVAR`.
- `internal/billing/application/usecases/kiwifypayload/commands.go` — normaliza `Customer.Mobile` para E.164 via `internal/platform/phone`.
- `internal/identity/domain/valueobjects/whatsapp_number.go` — delega normalização ao novo pacote (single source of truth).
- `configs/config.go` — novas chaves `ONBOARDING_ACTIVATION_PAGE_URL`, `ONBOARDING_ACTIVATION_WINDOW_HOURS`, mensagens `WA_MSG_ONBOARDING_INTRO` (já existe) e `WA_MSG_ACTIVATION_NOT_FOUND` (nova).
- `migrations/` — nova migration aditiva (índice de correlação por telefone + colunas de timestamps da jornada).
- Landing `mecontrola-landingpage`: `src/pages/ativar.astro`, `public/js/activate.js`, `tests/playwright/activate.spec.ts` — remoção do Telegram; nenhuma montagem de mensagem (usa `wa_me_url` as-is).

### Fluxo de Dados (alvo)

```
Checkout → cria magic_token PENDING (sem telefone)            create_checkout_session.go:74
   ↓
Webhook Kiwify order_approved → ProcessSaleApproved           process_sale_approved.go:35
   → commands.go normaliza Customer.Mobile → E.164            kiwifypayload/commands.go:18
   → publica billing.subscription.activated (telefone E.164)
   ↓ (outbox)
SubscriptionPaidConsumer → MarkTokenPaid (PENDING→PAID,       subscription_paid_consumer.go:44
   seta paidAt + customer_mobile_e164 E.164)
ActivationEmailConsumer → SendActivationEmail                 activation_email_consumer.go:39
   → CTA = ${ActivationPageURL}/ativar?token=<clear>          send_activation_email.go (modificado)
   ↓
Usuário abre /ativar?token= → GET /tokens/{token}/state       get_token_state.go (modificado)
   → wa_me_url = wa.me/<bot>?text=Ativar+o+meu+plano   (telefone presente)
   → wa_me_url = wa.me/<bot>?text=<token>  (borda sem telefone)
   ↓
Usuário envia "Ativar o meu plano" → POST /api/v1/whatsapp/inbound            inbound_handler.go:27
   → Dispatcher.Route                                         dispatcher.go:102
     → dedup WAMID (InsertIfAbsent)
     → establish.Execute → ErrUnknownUser
     → activationRoute(msg) → publica                         (callback novo)
         onboarding.activation.attempted.v1 {peer_e164,text,message_id}
   ↓ (outbox, tick ~500ms)
ActivationAttemptConsumer → ActivateFromInbound.Execute       (novos)
   → FindActivableByMobile(peer, now-24h)                     (query nova)
       ├─ achou → BindAndConsume (ActivationPathFallbackE164)  subscription_binding.go:41
       │   → UpsertUserByWhatsApp + BindUser + MarkConsumed(RowsAffected guard) + publish onboarding.subscription_bound
       ├─ não achou + texto contém token válido (com/sem prefixo "ATIVAR", transição) → ConsumeMagicToken (ActivationPathDirect)
       └─ nada → throttle por telefone (store durável); se 1ª vez na janela → SendTextMessage(activation_not_found)  [no-match, métrica/audit]
   ↓ (outbox)
WelcomeConsumer ← onboarding.subscription_bound {peer_e164, user_id}   (novo, idempotente, retry próprio)
   → SendTextMessage(welcome_activated) + SendTextMessage(onboarding_intro)   (RF-32/RF-33)
```

## Design de Implementação

### Interfaces Chave

Smart constructor de telefone (DMMF Princípio 1/2, R6.8) — fonte única:

```go
package phone

type Mobile struct{ e164 string }

func NewMobileBR(raw string) (Mobile, error)
func (m Mobile) String() string
func NormalizeBR(raw string) (string, error)
```

Guard de domínio puro na entidade (DMMF Princípio 6 — sem ctx, sem IO):

```go
func (m MagicToken) IsActivationWindowOpen(now time.Time, window time.Duration) bool {
    return m.status == valueobjects.TokenStatusPaid &&
        !m.paidAt.IsZero() &&
        !now.Before(m.paidAt) &&
        now.Sub(m.paidAt) <= window
}
```

Nova porta no repositório (interface no consumidor — `application/interfaces/magic_token_repository.go`):

```go
FindActivableByMobile(ctx context.Context, mobileE164 string, paidAfter time.Time) (entities.MagicToken, error)
```

`UpdateMarkConsumed` passa a retornar/checar linhas afetadas; `RowsAffected == 0` (perda da corrida `WHERE status='PAID'`) é mapeado para `ConsumeOutcomeAlreadyActive`, sem segundo `subscription_bound`.

Store de throttle de no-match (porta no consumidor, adapter postgres):

```go
type NoMatchThrottle interface {
    AllowReply(ctx context.Context, mobileE164 string, windowStart time.Time) (bool, error) // InsertIfAbsent
}
```

`ConsumeMagicToken.Execute` enforça **ambos** os guards: `!IsExpiredAt(now)` (teto de vida do checkout) **e** `IsActivationWindowOpen(now, window)` (janela de 24h de `paidAt`).

Novo usecase (pipeline DMMF Princípio 5; `Execute` orquestra passos privados):

```go
type ActivateFromInbound struct { /* repo, binding, consume, gateway, msgs, window, o11y, métricas */ }

func (uc *ActivateFromInbound) Execute(ctx context.Context, in input.ActivateFromInboundInput) (ActivateOutcome, error)

type ActivateOutcome uint8
const (
    ActivateOutcomePhoneMatched ActivateOutcome = iota + 1
    ActivateOutcomeTokenMatched
    ActivateOutcomeAlreadyActive
    ActivateOutcomeNoMatch
)
```

DTO de input com validação na fronteira (R-DTO-VALIDATE-001):

```go
type ActivateFromInboundInput struct {
    PeerE164  string
    Text      string
    MessageID string
}
func (i *ActivateFromInboundInput) Validate() error // errors.Join; nomeia campos; peer não-vazio, message_id não-vazio
```

Callback injetado no dispatcher (simétrico ao `agentRoute` existente — mantém o adapter genérico):

```go
type Dispatcher struct {
    // ...campos atuais...
    activationRoute func(ctx context.Context, msg payload.Message) RouteOutcome
}
```

Consumer (adapter fino — herda R-ADAPTER-001, espelha `whatsapp_inbound_consumer.go`):

```go
func (c *ActivationAttemptConsumer) Handle(ctx context.Context, event events.Event) error
```

### Modelos de Dados

**Entidade `MagicToken`** (`internal/onboarding/domain/entities/magic_token.go`): sem mudança de campos. Adiciona-se apenas o guard puro `IsActivationWindowOpen`. `paidAt` já é setado em `MarkPaid` (linha 128). `ActivationPath` já tem `ActivationPathFallbackE164` (telefone) e `ActivationPathDirect` (token) — usados sem novas constantes.

**Migration aditiva** (novo arquivo `migrations/0000NN_activation_journey.up.sql`):

```sql
CREATE INDEX IF NOT EXISTS onboarding_tokens_mobile_activable_idx
    ON mecontrola.onboarding_tokens (customer_mobile_e164, paid_at)
    WHERE status = 'PAID';

ALTER TABLE mecontrola.onboarding_tokens
    ADD COLUMN IF NOT EXISTS email_sent_at         TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS page_opened_at        TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS activation_started_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS whatsapp_opened_at    TIMESTAMPTZ NULL;

CREATE TABLE IF NOT EXISTS mecontrola.onboarding_activation_nomatch_throttle (
    mobile_e164  TEXT        NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT onboarding_activation_nomatch_throttle_pkey PRIMARY KEY (mobile_e164, window_start)
);
```

Timestamps da jornada (RF-35) mapeados: `payment_confirmed_at` = `paid_at` (existente); `activation_completed_at` = `consumed_at` (existente); `email_sent_at` e `activation_started_at` = novas colunas escritas no servidor com semântica **set-once-if-null** (idempotente); `page_opened_at` e `whatsapp_opened_at` = novas colunas escritas via **beacon dedicado** (`POST /tokens/{token}/opened`), **nunca** como efeito do `GET /state`. O store de throttle limita a resposta de no-match a 1 por telefone por janela (RF-24) e é higienizado por job (padrão `whatsapp/dedup`).

**Nova query** `FindActivableByMobile` (postgres, espelha `FindPaidByMobileForFallback` sem o gate de outreach):

```sql
SELECT <colunas completas>
  FROM mecontrola.onboarding_tokens
 WHERE status = 'PAID'
   AND customer_mobile_e164 = $1
   AND paid_at > $2
 ORDER BY paid_at DESC
 LIMIT 1
```

`$2 = now - ActivationWindow`. `ORDER BY paid_at DESC LIMIT 1` satisfaz RF-23 (mais recente).

**Novo evento outbox** `onboarding.activation.attempted.v1` (payload imutável, factory — DMMF Princípio 7):

```go
type activationAttemptedPayload struct {
    PeerE164  string `json:"peer_e164"`
    Text      string `json:"text"`
    MessageID string `json:"message_id"`
}
```

`AggregateType = "whatsapp.message"`, `AggregateID = message_id` (WAMID), sem `AggregateUserID` (usuário ainda não existe).

### Endpoints de API

- `GET /api/v1/onboarding/tokens/{token}/state` (existente, modificado): `wa_me_url` passa a usar `Oi` (telefone presente) ou token cru (borda sem telefone); sem campo Telegram. **Permanece idempotente e sem escrita** (não grava `page_opened_at`).
- `POST /api/v1/whatsapp/inbound` (existente): comportamento de roteamento alterado conforme ADR-001 (publica evento de ativação para número não vinculado).
- `POST /api/v1/onboarding/tokens/{token}/opened` (novo, beacon): registra `page_opened_at`/`whatsapp_opened_at` (set-once-if-null, RF-35). Rate-limited (reusa `middleware.NewRateLimiter`), valida token, resposta `204` mesmo em token inválido (não vaza estado). A página `/ativar` chama este beacon ao exibir o CTA e ao acionar o botão "Abrir WhatsApp".

## Pontos de Integração

- **Meta WhatsApp Cloud API**: client atual (`infrastructure/http/client/meta/client.go`) só envia `text`/`template`. Boas-vindas (`welcome_activated` + `onboarding_intro`) são **texto livre** dentro da janela de 24h aberta pelo usuário — não requer template nem botão interativo (RF-33). Nenhuma extensão do client.
- **Kiwify**: nenhuma mudança de contrato; apenas normalização do `Customer.Mobile` recebido.
- **Landing page (repo separado)**: contrato inalterado do endpoint `/state`; consumo de `wa_me_url` as-is. Mudança restrita à remoção do Telegram.

## Abordagem de Testes

### Testes Unitários

Padrão canônico testify/suite, whitebox, `fake.NewProvider()` (R-TESTING-001):

- `phone.NewMobileBR` / `NormalizeBR`: tabela de casos (`11999999999`, `5511999999999`, `+5511999999999`, formatados, inválidos, vazio) — domínio puro, sem mock.
- `MagicToken.IsActivationWindowOpen`: dentro/fora da janela, `paidAt` zero, status ≠ PAID — domínio puro.
- `ActivateFromInbound.Execute`: cenários phone-matched, token-matched (borda), already-active (replay), no-match; mocks gerados via mockery para repo/binding/consume/gateway; assert envio das **duas** mensagens de boas-vindas no caminho de sucesso e da mensagem de no-match no caminho negativo.
- `ActivateFromInboundInput.Validate`: peer vazio, message_id vazio, válido.
- `SendActivationEmail`: URL gerada = `${base}/ativar?token=<escaped>`; idempotência em token consumido/expirado.
- `GetTokenState`: `wa_me_url` com `Oi` quando telefone presente; com token cru quando ausente; sem `ATIVAR`.
- `kiwifypayload` commands: `Customer.Mobile` normalizado; vazio/inválido → string vazia (borda RF-30).
- Dispatcher: número não vinculado dispara `activationRoute`; número vinculado roteia ao agente; remoção do ramo `ATIVAR`.

### Testes de Integração

Critérios atendidos (fronteira de IO crítica em Postgres + correção de query): **sim**. Usar testcontainers-go com `//go:build integration`:

- `FindActivableByMobile`: semântica de janela (`paid_at > $2`), seleção da mais recente (recompra), ausência de match, isolamento por status.
- `ActivationAttemptConsumer` ponta-a-ponta com Postgres real: evento → consume → token `CONSUMED` → user bound → idempotência em reentrega do mesmo evento.
- Set-once-if-null dos timestamps da jornada.

### Testes E2E

- Backend: cenário "paga → PAID → /state retorna Oi → inbound não-vinculado → ativado → boas-vindas" (reuso da suíte e2e existente do onboarding).
- Landing (`mecontrola-landingpage`): Playwright `activate.spec.ts` — remover o teste de `telegram_deep_link`/`#activate-tg-btn` (linhas 39-61); manter os demais (loading/ready/error/consumed/countdown).

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. `internal/platform/phone` (smart constructor) + refactor `whatsapp_number.go` para delegar — base sem dependências, destrava normalização. (DMMF P1/P2)
2. Normalização do `Customer.Mobile` em `kiwifypayload/commands.go` (RF-07/RF-21) — garante telefone E.164 no token PAID.
3. Migration aditiva (índice + colunas de timestamp).
4. Domain guard `IsActivationWindowOpen` + query `FindActivableByMobile` (interface + postgres + integration test).
5. `ConsumeMagicToken`: adicionar guard `IsActivationWindowOpen`; `UpdateMarkConsumed` com guard de `RowsAffected` → `AlreadyActive` (correção de concorrência).
6. Store durável de throttle de no-match (tabela + porta + adapter + housekeeping job) — padrão `whatsapp/dedup`.
7. Usecase `ActivateFromInbound` + DTO `Validate()` (correlação telefone → token transicional → no-match com throttle; **sem boas-vindas**) (unit tests).
8. Consumer `ActivationAttemptConsumer` + evento `onboarding.activation.attempted.v1` (incluir em `noUserEventAllowlist`) + registro em `onboarding/module.go` EventHandlers.
8b. `WelcomeConsumer` em `onboarding.subscription_bound` (idempotente, 2 mensagens) + registro no EventHandlers.
9. Dispatcher: remover ramo `ATIVAR`, injetar `activationRoute`; expor `WhatsAppActivationRoute` no onboarding module; wiring em `cmd/server/whatsapp_wiring.go`.
10. E-mail → `/ativar` (config `ONBOARDING_ACTIVATION_PAGE_URL`), supressão se assinatura já vinculada, e `get_token_state` → `Oi`/token-cru (RF-13/RF-20); `email_sent_at`/`activation_started_at`.
11. Beacon `POST /tokens/{token}/opened` + usecase de timestamp + chamada na landing (`page_opened_at`/`whatsapp_opened_at`).
12. Config: `ONBOARDING_ACTIVATION_WINDOW_HOURS`, `ONBOARDING_ACTIVATION_PAGE_URL`, `WA_MSG_ACTIVATION_NOT_FOUND`, throttle (janela/housekeeping); remover uso de `WA_MSG_PLEASE_USE_ATIVAR_COMMAND`.
13. Landing: remover Telegram (astro + js + playwright) + chamada do beacon.
14. E2E + validação `global` (wiring multi-módulo).

### Dependências Técnicas

- Postgres (testcontainers) para integração.
- Variável `ONBOARDING_ACTIVATION_PAGE_URL` provisionada por ambiente (prod: `https://mecontrola.app.br`).
- Deploy coordenado backend↔landing (a landing já consome `wa_me_url` as-is; backend pode ir primeiro sem quebrar a página).

## Monitoramento e Observabilidade

Métricas (cardinalidade controlada — RF-37, R-TXN-004; labels apenas enums fechados):

- `onboarding_activation_attempt_total{outcome}` — `phone_matched|token_matched|already_active|no_match`.
- `onboarding_activation_window_expired_total` — tentativas fora da janela de 24h.
- Reuso de `onboarding_activation_email_dispatched_total{result}` (existente).
- Run auditável por execução do consumer: `message_id`, `outcome`, `duration_ms`, `error` (sem telefone/e-mail/user_id como label).

Logs estruturados em cada passo (RF-36) incluindo resultado da correlação (match/no-match) com telefone mascarado (`payload.MaskMobile`). Spans OTel em `Route`, consumer e usecase, integrados ao stack otel-lgtm existente.

## Considerações Técnicas

### Decisões Chave

- **ADR-001** (`adr-001-activation-event-driven.md`): ativação por telefone via evento outbox + consumer no onboarding, com o dispatcher publicando via callback injetado (rejeitada a chamada síncrona de domínio dentro do dispatcher).
- **ADR-002** (`adr-002-activation-window-paidat.md`): janela de 24h a partir de `paidAt` como guard de domínio + query dedicada, preservando `expiresAt` do checkout e o job de expiração.
- **ADR-003** (`adr-003-oi-correlation-email-page.md`): e-mail/página → `/ativar` com `Oi`, correlação por telefone E.164, fallback por token na borda sem telefone, remoção do Telegram e do legado `ATIVAR`.

### Riscos Conhecidos

- **Mensagem de número não vinculado sem rate-limit do principal**: o `limiter` atual usa `principal.UserID`, inexistente para número novo. Mitigação implementada: dedup por WAMID antes do publish + **store durável de throttle por telefone** (1 resposta de no-match por janela), que também idempotentiza a resposta sob reentrega do evento.
- **Concorrência multi-instância na ativação**: dois eventos do mesmo telefone processados em paralelo. Mitigação: guard otimista `UPDATE ... WHERE status='PAID'` + checagem de `RowsAffected==0` → `AlreadyActive`; boas-vindas só no outcome de sucesso real.
- **Divergência de formato de telefone Kiwify×Meta**: mitigada pela fonte única `internal/platform/phone`; telefone não-normalizável → string vazia → borda RF-30 (correlação por token), nunca ativação errada.
- **Reentrega do evento de ativação (at-least-once)**: `ConsumeMagicToken`/`BindAndConsume` são idempotentes pela máquina de estado (`PAID→CONSUMED`; replay → `AlreadyActive`); boas-vindas só no outcome `PhoneMatched`/`TokenMatched`, suprimidas em `AlreadyActive`.
- **`whatsapp_opened_at` ("quando possível")**: depende do beacon client-side; coluna pode permanecer nula sem violar a jornada.
- **Boas-vindas parcial**: entrega desacoplada via `WelcomeConsumer` em `onboarding.subscription_bound` com retry próprio e idempotência por event id; falha entre a 1ª e a 2ª mensagem reprocessa sem reativar.
- **Agente pós-ativação é `weather-agent`** (único registrado em `internal/agents`): a boas-vindas promete "assistente financeiro", mas a próxima mensagem roteia ao weather-agent. **Limitação conhecida e fora de escopo** (RF-34 encerra a jornada na boas-vindas); resolver quando o agente financeiro for registrado.
- **Recompra de usuário já vinculado**: e-mail de ativação é suprimido quando a assinatura já tem usuário; acesso permanece pela projeção `billing→identity`.

### Conformidade com Padrões

- `R-ADAPTER-001` — dispatcher/consumer/handler finos; zero comentários em `.go` de produção; sem SQL/branching de domínio no adapter (a correlação vive no usecase).
- `R-DTO-VALIDATE-001` — `ActivateFromInboundInput.Validate()` logo após o span no `Execute`.
- `R-TESTING-001` — testify/suite whitebox, `fake.NewProvider()`, IIFE por mock.
- `R-TXN-004` — métricas sem `user_id`/telefone/e-mail como label.
- `go-implementation` DMMF — smart constructor (`phone.Mobile`), guard puro (`IsActivationWindowOpen`), pipeline (`ActivateFromInbound.Execute`), pure-core/IO-shell, `errors.Join`/`%w`, `iota+1`, sem `init()`, sem `panic` em produção, sem abstração de tempo (`time.Now().UTC()` inline).
- `R-WF-KERNEL-001`/`R-AGENT-WF-001` — não há kernel de workflow nem agente neste escopo; o fluxo de ativação não usa LLM e não é roteado por `switch intent.Kind`.

### Arquivos Relevantes e Dependentes

Novos: `internal/platform/phone/mobile.go`; `internal/onboarding/application/usecases/activate_from_inbound.go`; `internal/onboarding/application/dtos/input/activate_from_inbound_input.go`; `internal/onboarding/infrastructure/messaging/database/consumers/activation_attempt_consumer.go`; `migrations/0000NN_activation_journey.{up,down}.sql`.

Modificados: `internal/platform/whatsapp/dispatcher/dispatcher.go`; `internal/onboarding/domain/entities/magic_token.go`; `internal/onboarding/application/interfaces/magic_token_repository.go`; `internal/onboarding/infrastructure/repositories/postgres/magic_token_repository.go`; `internal/onboarding/application/usecases/{send_activation_email,get_token_state}.go`; `internal/onboarding/module.go`; `internal/billing/application/usecases/kiwifypayload/commands.go`; `internal/identity/domain/valueobjects/whatsapp_number.go`; `cmd/server/whatsapp_wiring.go`; `configs/config.go`. Landing: `src/pages/ativar.astro`, `public/js/activate.js`, `tests/playwright/activate.spec.ts`.
