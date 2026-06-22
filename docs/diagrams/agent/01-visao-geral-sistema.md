# Visão Geral do Sistema — MeControla

Diagrama de nível macro mostrando os processos, módulos internos e dependências externas.

---

## Contexto Externo

```mermaid
flowchart TB
    USER["👤 Usuário\nWhatsApp / Telegram"]

    subgraph EXTERNAL ["Serviços Externos"]
        WA_API["WhatsApp Cloud API\nMeta Graph API"]
        TG_API["Telegram Bot API"]
        OR_API["OpenRouter\nGemini / GPT / Mistral / Claude"]
        KW_API["Kiwify\nPagamentos e assinaturas"]
        OTEL_SRV["Grafana LGTM\nOTEL Collector"]
        PG[("PostgreSQL")]
    end

    subgraph MECONTROLA ["MeControla"]
        SERVER["mecontrola server\nHTTP + Webhooks"]
        WORKER["mecontrola worker\nJobs + Outbox Dispatcher"]
    end

    USER -->|"envia mensagem"| WA_API
    USER -->|"envia mensagem"| TG_API
    WA_API -->|"POST /api/v1/whatsapp/inbound"| SERVER
    TG_API -->|"POST /api/v1/telegram/inbound"| SERVER
    KW_API -->|"POST /api/v1/kiwify/webhook"| SERVER
    SERVER -->|"POST /{phone_id}/messages"| WA_API
    SERVER -->|"POST /sendMessage"| TG_API
    SERVER -->|"POST /api/v1/chat/completions"| OR_API
    SERVER -->|"lê / escreve"| PG
    SERVER -->|"INSERT outbox events"| PG
    WORKER -->|"ClaimBatch + Mark"| PG
    WORKER -->|"notificações assíncronas"| WA_API
    SERVER -->|"OTLP traces/métricas"| OTEL_SRV
    WORKER -->|"OTLP traces/métricas"| OTEL_SRV
```

---

## Módulos Internos — Server

```mermaid
flowchart TB
    subgraph ENTRY ["Entrada (cmd/server)"]
        WH_WA["WhatsApp Webhook Router\n/api/v1/whatsapp/inbound"]
        WH_TG["Telegram Webhook Router\n/api/v1/telegram/inbound"]
        WH_KW["Kiwify Webhook Router\n/api/v1/kiwify/webhook"]
        HTTP_API["HTTP REST APIs\n/api/v1/..."]
    end

    subgraph PLATFORM ["Platform (cross-cutting)"]
        OUTBOX["Outbox\nPublisher + Storage"]
        NOTIFY["Notification\nMultiChannelGateway"]
        DB["Database\nPool + UoW"]
    end

    subgraph MODULES ["Módulos de Domínio"]
        ID["Identity\nusuários · auth · entitlements\nWhatsApp dedup · rate limiter"]
        CAT["Categories\ncatálogo · dicionário + ETag cache"]
        BILL["Billing\nassinaturas Kiwify\ngrace period · reconciliação"]
        ONB["Onboarding\nmagic token · ativação\noutreach · WA/TG processor"]
        CARD["Card\nCRUD cartões\nfatura por competência"]
        BUD["Budgets\norçamentos mensais\nalertas de threshold"]
        TX["Transactions\nlançamentos · recorrência\nresumo reconciliado"]
        AGT["Agent\nintent parsing LLM\nroteamento · session store\nonboarding LLM"]
    end

    subgraph ADAPTERS ["Adapters Externos"]
        META["Meta Graph API client"]
        OR_CLIENT["OpenRouter client"]
        KW_CLIENT["Kiwify API client"]
    end

    WH_WA --> ID
    WH_WA --> ONB
    WH_WA --> AGT
    WH_TG --> ID
    WH_TG --> ONB
    WH_TG --> AGT
    WH_KW --> BILL
    HTTP_API --> ID
    HTTP_API --> CARD
    HTTP_API --> TX
    HTTP_API --> BUD
    HTTP_API --> CAT

    AGT --> OR_CLIENT
    AGT --> TX
    AGT --> BUD
    AGT --> CARD
    AGT --> CAT

    ONB --> META
    BILL --> KW_CLIENT
    NOTIFY --> META

    ID --> OUTBOX
    BILL --> OUTBOX
    ONB --> OUTBOX
    AGT --> OUTBOX
    TX --> OUTBOX

    OUTBOX --> DB
    ID --> DB
    CAT --> DB
    BILL --> DB
    ONB --> DB
    CARD --> DB
    BUD --> DB
    TX --> DB
    AGT --> DB
```

---

## Módulos Internos — Worker (Jobs Agendados)

```mermaid
flowchart TB
    subgraph WORKER ["cmd/worker"]
        EVENTS["Event Dispatcher\nHandler Registry"]
    end

    subgraph OUTBOX_JOBS ["Outbox Jobs"]
        J1["OutboxDispatcherJob\n@every 1s\nClaim + dispatch + mark"]
        J2["OutboxReaperJob\ndeleta eventos publicados"]
        J3["OutboxHousekeepingJob\nreset eventos travados"]
    end

    subgraph DOMAIN_JOBS ["Domain Jobs"]
        J4["AuthEventsHousekeepingJob"]
        J5["BillingReconciliationJob"]
        J6["BillingGraceExpirationJob\n@daily"]
        J7["OnboardingOutreachJob"]
        J8["OnboardingExpirationJob"]
        J9["BudgetsThresholdAlertsJob"]
        J10["CardInvoiceDueAlertsJob"]
        J11["RecurringMaterializerJob"]
        J12["MonthlySummaryReconcilerJob"]
    end

    subgraph HANDLERS ["Event Handlers (via Outbox)"]
        H_ID["Identity\nauth events · subscription"]
        H_BILL["Billing\nnotification handlers"]
        H_ONB["Onboarding\nsubscription binding"]
        H_CARD["Card\ncriação ao onboarding"]
        H_BUD["Budgets\npending events"]
        H_TX["Transactions\nmonthly summary recompute"]
    end

    WORKER --> J1
    WORKER --> J2
    WORKER --> J3
    WORKER --> J4
    WORKER --> J5
    WORKER --> J6
    WORKER --> J7
    WORKER --> J8
    WORKER --> J9
    WORKER --> J10
    WORKER --> J11
    WORKER --> J12

    J1 --> EVENTS
    EVENTS --> H_ID
    EVENTS --> H_BILL
    EVENTS --> H_ONB
    EVENTS --> H_CARD
    EVENTS --> H_BUD
    EVENTS --> H_TX
```
