# Comunicacao Entre Modulos — Padrao Outbox

Documentacao do padrao de comunicacao assincrona entre modulos via Transactional Outbox com Postgres.

## Referencias de codigo

| Componente | Arquivo |
|---|---|
| Outbox core | `internal/platform/outbox/outbox.go` |
| Dispatcher job | `internal/platform/outbox/dispatcher.go` |
| Publisher | `internal/platform/outbox/publisher.go` |
| Storage Postgres | `internal/platform/outbox/storage_postgres.go` |
| Envelope | `internal/platform/outbox/envelope.go` |
| Events interface | `internal/platform/events/events.go` |
| Worker bootstrap | `cmd/worker/worker.go` |
| Identity consumers | `internal/identity/infrastructure/messaging/database/consumers/` |
| Billing producers | `internal/billing/infrastructure/messaging/database/producers/` |
| Agents WhatsApp route (producer) | `internal/agents/module.go` (`buildWhatsAppAgentRoute`) |
| Agents inbound consumer | `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go` |
| Agents embedding index handler | `internal/platform/memory/infrastructure/indexer/handler.go` |

---

## Visao Geral do Padrao

```mermaid
flowchart LR
    subgraph SERVER ["Server Process"]
        UC["Use Case / Service"]
        DB_BIZ[("Tabela de negocio")]
        PUB["outbox.Publisher"]
        OUT_TBL[("outbox table")]
    end

    subgraph WORKER ["Worker Process"]
        JOB["OutboxDispatcherJob\n@every 1s"]
        CLAIM["ClaimBatch\npessimistic lock"]
        REG["Handler Registry\nevento -> Handlers"]
        HA["Handler A"]
        HB["Handler B"]
        MARK["Mark status"]
    end

    UC -->|"INSERT biz logic"| DB_BIZ
    UC --> PUB
    PUB -->|"INSERT outbox\nmesma tx"| OUT_TBL

    JOB --> CLAIM
    CLAIM -->|"SELECT FOR UPDATE\nSKIP LOCKED"| OUT_TBL
    CLAIM --> REG
    REG --> HA
    REG --> HB
    HA --> MARK
    HB --> MARK
    MARK -->|"UPDATE status\npublished/retry/failed"| OUT_TBL
```

---

## Ciclo de Vida de um Evento no Outbox

```mermaid
stateDiagram-v2
    [*] --> pending : INSERT transacional com biz logic

    pending --> processing : ClaimBatch locked_by instanceID
    processing --> published : handler.Handle OK MarkPublished
    processing --> pending_retry : handler.Handle falha MarkPendingRetry
    pending_retry --> processing : next_attempt_at menor que now
    processing --> failed : tentativas maior que max_attempts MarkFailed

    published --> [*] : deletado pelo OutboxReaperJob apos retention
    failed --> [*] : retido para diagnostico
```

---

## Sequencia: Publicar e Consumir Evento

```mermaid
sequenceDiagram
    autonumber

    participant UC as Use Case
    participant UOW as UnitOfWork
    participant REPO as Repositorio
    participant PUB as outbox.Publisher
    participant OUT as outbox table
    participant JOB as OutboxDispatcherJob
    participant REG as Handler Registry
    participant HA as Handler A
    participant HB as Handler B

    UC->>+UOW: Do(ctx, func(tx))
    UOW->>+REPO: CRUD logica de negocio
    REPO->>OUT: INSERT tabela de dominio
    REPO-->>-UOW: ok

    UOW->>+PUB: Publish(ctx, outbox.Event)
    Note over PUB,OUT: INSERT outbox na mesma tx<br/>id, type, aggregate_type,<br/>aggregate_id, aggregate_user_id,<br/>payload, metadata, occurred_at,<br/>status=pending, attempts=0
    PUB->>OUT: INSERT outbox
    PUB-->>-UOW: nil
    UOW-->>-UC: COMMIT ambos

    Note over JOB: roda a cada 1s no Worker
    JOB->>OUT: SELECT FOR UPDATE SKIP LOCKED
    Note over JOB,OUT: WHERE status IN (pending, pending_retry)<br/>AND next_attempt_at <= now<br/>LIMIT batchSize
    OUT-->>JOB: []Row

    loop Para cada Row
        JOB->>+REG: HandlersOf(row.Type)
        REG-->>-JOB: []Handler

        par Handlers em paralelo
            JOB->>+HA: Handle(ctx, event)
            HA-->>-JOB: nil
        and
            JOB->>+HB: Handle(ctx, event)
            HB-->>-JOB: nil
        end

        alt Todos OK
            JOB->>OUT: MarkPublished(id)
        else Algum falhou
            JOB->>OUT: MarkPendingRetry(id, lastErr, nextAttemptAt)
            Note over JOB,OUT: Backoff exponencial com jitter 20%<br/>base dobra a cada tentativa
        else Max attempts atingido
            JOB->>OUT: MarkFailed(id, lastErr)
        end
    end
```

---

## Todos os Event Types do Sistema

```mermaid
flowchart TB
    subgraph AUTH ["auth (identity)"]
        E1["auth.principal_established\nAggregateType: User"]
        E2["auth.failed\nAggregateType: User"]
        E3["auth.unknown_user\nAggregateType: WhatsApp"]
    end

    subgraph BILLING ["billing"]
        E4["billing.subscription.activated"]
        E5["billing.subscription.renewed"]
        E6["billing.subscription.past_due"]
        E7["billing.subscription.canceled"]
        E8["billing.subscription.refunded"]
    end

    subgraph ONBOARDING ["onboarding"]
        E9["onboarding.subscription_bound"]
    end

    subgraph AGENTS ["agents (platform substrate)"]
        E10["agents.whatsapp.inbound.v1\nAggregateType: User"]
        E11["platform.memory.embedding.index.v1\nAggregateType: Resource"]
    end
```

---

## Mapa de Producers para Consumers

```mermaid
flowchart LR
    subgraph PROD ["Producers"]
        P1["Identity\nOutboxPublisher"]
        P2["Billing\nSubscriptionEventPublisher"]
        P3["Onboarding\nOnboardingEventPublisher"]
        P4["Agents\nWhatsApp route + PublishingMessageStore"]
    end

    OB[("outbox table\nPostgres")]

    subgraph CONS ["Consumers"]
        C1["Identity\nAuthEventProjector\nSubscriptionProjector"]
        C2["Billing\nNotificationHandler"]
        C3["Onboarding\nSubscriptionBindingHandler"]
        C4["Card\nCardCreationOnOnboardingHandler"]
        C5["Budgets\nPendingEventsHandler"]
        C6["Transactions\nMonthlySummaryRecomputeHandler"]
        C7["Agents\nWhatsAppInboundConsumer\nEmbeddingIndexHandler"]
    end

    P1 --> OB
    P2 --> OB
    P3 --> OB
    P4 --> OB

    OB -->|"auth.*"| C1
    OB -->|"billing.subscription.*"| C2
    OB -->|"billing.subscription.activated"| C3
    OB -->|"onboarding.subscription_bound"| C4
    OB -->|"billing.subscription.*"| C5
    OB -->|"billing.subscription.*"| C6
    OB -->|"agents.whatsapp.inbound.v1\nplatform.memory.embedding.index.v1"| C7
```

---

## Envelope Padrao

```mermaid
classDiagram
    class Event {
        +string ID
        +string Type
        +string AggregateType
        +string AggregateID
        +string AggregateUserID
        +bytes Payload
        +map Metadata
        +time.Time OccurredAt
    }

    class Envelope {
        +string ID
        +string EventType
        +string AggregateUserID
        +time.Time OccurredAt
        +map Metadata
        +json.RawMessage Payload
    }

    class OutboxRow {
        +string ID
        +string Type
        +string AggregateType
        +string AggregateID
        +string AggregateUserID
        +bytes Payload
        +string Status
        +int Attempts
        +string LockedBy
        +time.Time NextAttemptAt
        +string LastError
        +time.Time OccurredAt
    }

    Event --> OutboxRow : persisted as
    OutboxRow --> Envelope : deserialized for handlers
```

---

## Jobs do Worker e Responsabilidades

| Job | Frequencia | Responsabilidade |
|-----|-----------|-----------------|
| `OutboxDispatcherJob` | @every 1s | Claim batch, executa handlers, marca status |
| `OutboxReaperJob` | configuravel | Deleta eventos published apos retention window |
| `OutboxHousekeepingJob` | configuravel | Reset de eventos processing travados |
| `AuthEventsHousekeepingJob` | configuravel | Limpa auth_events antigos |
| `BillingReconciliationJob` | configuravel | Reconcilia assinaturas Kiwify vs DB |
| `BillingGraceExpirationJob` | @daily | Expira assinaturas em grace period (3 dias) |
| `OnboardingOutreachJob` | configuravel | Envia mensagens de outreach (gap min. 2h) |
| `OnboardingExpirationJob` | configuravel | Expira magic tokens apos TTL (7 dias) |
| `BudgetsThresholdAlertsJob` | configuravel | Alertas de threshold de orcamento |
| `CardInvoiceDueAlertsJob` | configuravel | Alertas de fatura vencendo |
| `RecurringMaterializerJob` | configuravel | Materializa transacoes recorrentes |
| `MonthlySummaryReconcilerJob` | configuravel | Reconcilia resumo mensal |
