# Fluxo: WhatsApp Webhook → Dispatcher

Diagrama de sequência detalhado do caminho de uma mensagem desde a chegada no webhook até o roteamento final.

## Referências de código

| Componente | Arquivo |
|---|---|
| Router HTTP | `internal/identity/infrastructure/http/server/whatsapp_router.go` |
| Middleware signature | `internal/platform/whatsapp/signature/` |
| Middleware rate limit | `internal/platform/whatsapp/ratelimit/` |
| InboundHandler | `internal/platform/whatsapp/handlers/inbound_handler.go` |
| Dispatcher | `internal/platform/whatsapp/dispatcher/dispatcher.go` |
| Payload parser | `internal/platform/whatsapp/payload/` |
| Dedup | `internal/platform/whatsapp/dedup/` |
| EstablishPrincipal | `internal/identity/application/usecases/establish_principal.go` |
| Wiring | `cmd/server/whatsapp_wiring.go` |

---

## Sequência Completa

```mermaid
sequenceDiagram
    autonumber

    actor WA as WhatsApp Cloud API
    participant RL as RateLimit MW
    participant SIG as HMAC-SHA256 MW
    participant IH as InboundHandler
    participant DISP as Dispatcher
    participant PAY as payload.Extract
    participant DEDUP as DedupRepository
    participant EST as EstablishPrincipal
    participant LIM as WhatsAppLimiter
    participant PUB as outbox.Publisher
    participant ONB as onboardingRoute
    participant AGT as agentRoute

    WA->>+RL: POST /api/v1/whatsapp/inbound
    Note over RL: Rate limit por IP<br/>600 req/min, burst=100
    alt rate limit excedido
        RL-->>WA: 429 Too Many Requests
    end

    RL->>+SIG: passa request
    Note over SIG: Valida X-Hub-Signature-256<br/>HMAC-SHA256(body, META_APP_SECRET)<br/>Suporta rotacao com META_APP_SECRET_NEXT
    alt assinatura invalida
        SIG-->>WA: 401 Unauthorized
    end

    SIG->>+IH: raw body (validado)
    IH->>+DISP: Route(ctx, json.RawMessage)

    DISP->>+PAY: ExtractFirstMessage(raw)
    Note over PAY: entry[0].changes[0].value.messages[0]<br/>retorna Message{From, Text, WAMID, Timestamp}
    PAY-->>-DISP: Message

    Note over DISP: checkTimestamp<br/>janela: 5 minutos
    alt timestamp invalido ou muito antigo
        DISP-->>IH: OutcomeStaleTS
        Note over DISP: staleWebhook counter++
        IH-->>WA: 200 OK (ack silencioso)
    end

    DISP->>+DEDUP: InsertIfAbsent(ctx, WAMID)
    alt WAMID ja existe
        DEDUP-->>DISP: false
        DISP-->>IH: OutcomeDuplicate
        IH-->>WA: 200 OK (ack silencioso)
    end
    DEDUP-->>-DISP: true (novo)

    Note over DISP: channels.MatchActivationCommand(text)<br/>detecta padrao "ATIVAR token"
    alt texto e comando de ativacao
        DISP->>+ONB: onboardingRoute(ctx, msg)
        Note over ONB: HandleActivation(ctx, from, token)
        ONB-->>-DISP: OutcomeOnboarding
        DISP-->>IH: OutcomeOnboarding
        IH-->>WA: 200 OK
    end

    DISP->>+EST: Execute(ctx, {WhatsAppNumber: msg.From})
    Note over EST: busca usuario por numero<br/>verifica entitlements
    alt usuario nao encontrado (ErrUnknownUser)
        EST-->>DISP: ErrUnknownUser
        DISP->>PUB: Publish(auth.unknown_user)
        DISP->>+ONB: onboardingRoute(ctx, msg)
        Note over ONB: HandleFallback(ctx, from)
        ONB-->>-DISP: OutcomeFallback
        DISP-->>IH: OutcomeFallback
        IH-->>WA: 200 OK
    end
    EST-->>-DISP: Principal{UserID, ...}

    Note over DISP: auth.WithPrincipal(ctx, principal)<br/>injeta Principal no context
    DISP->>+LIM: limiter.Allow(principal.UserID)
    Note over LIM: per-user token bucket<br/>120/min, burst=60
    alt rate limit por usuario excedido
        LIM-->>DISP: false
        DISP->>PUB: Publish(auth.failed)
        DISP-->>IH: OutcomeRateLimited
        IH-->>WA: 200 OK
    end
    LIM-->>-DISP: true

    DISP->>+AGT: agentRoute(ctx, msg)
    Note over AGT: agentModule.WhatsAppAgentRoute<br/>Principal ja esta no ctx
    AGT-->>-DISP: OutcomeAgent

    DISP-->>-IH: OutcomeAgent
    IH-->>-WA: 200 OK

    Note over DISP: metrica: whatsapp_dispatcher_route_total<br/>label outcome: onboarding, agent, fallback,<br/>rate_limited, duplicate, stale_webhook
```

---

## Estrutura do Payload Meta Cloud API

```mermaid
flowchart LR
    subgraph JSON ["JSON Webhook Body"]
        ROOT["object: whatsapp_business_account"]
        ENTRY["entry[0]"]
        CHANGE["changes[0]"]
        VALUE["value"]
        MSG["messages[0]"]
        FIELDS["from: E.164 sem +\nid: WAMID unico\ntimestamp: Unix epoch\ntype: text\ntext.body: conteudo"]
    end

    ROOT --> ENTRY --> CHANGE --> VALUE --> MSG --> FIELDS
```

---

## Decision Tree dos Outcomes

```mermaid
flowchart TD
    IN([Mensagem recebida]) --> TS{Timestamp valido?\njanela 5 min}
    TS -- Nao --> STALE[OutcomeStaleTS\nignora silenciosamente]
    TS -- Sim --> DUP{WAMID ja\nprocessado?}
    DUP -- Sim --> DUPLICATE[OutcomeDuplicate\nignora silenciosamente]
    DUP -- Nao --> CMD{"Texto e\n'ATIVAR token'?"}
    CMD -- Sim --> ONB_ACT[OutcomeOnboarding\nativacao de conta]
    CMD -- Nao --> USER{Usuario existe\nno DB?}
    USER -- Nao --> ONB_FALL[OutcomeFallback\nmensagem de boas-vindas]
    USER -- Sim --> RATE{Rate limit\npor usuario ok?}
    RATE -- Excedido --> RL[OutcomeRateLimited\nPublica auth.failed]
    RATE -- OK --> AGENT[OutcomeAgent\nprocessa com LLM]
```

---

## Configuracao Relevante

```bash
# Webhook security
META_APP_SECRET=<sha256-hmac-key>
META_APP_SECRET_NEXT=<rotacao-opcional>
META_VERIFY_TOKEN=<handshake-GET>

# Rate limits globais (por IP)
WHATSAPP_WEBHOOK_RATE_LIMIT_PER_MIN=600
WHATSAPP_WEBHOOK_RATE_LIMIT_BURST=100

# Rate limit por usuario autenticado
AUTH_RATE_LIMIT_PER_USER_PER_MIN=120
AUTH_RATE_LIMIT_PER_USER_BURST=60
```
