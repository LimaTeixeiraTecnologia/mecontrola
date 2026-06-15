# Documentação C4 PlantUML — Diagramas de Container

Data: 2026-06-15

## Objetivo

Documentar a arquitetura de cada módulo do MeControla em PlantUML usando o modelo C4 (visão de container). Cada arquivo descreve os containers internos, atores externos, eventos publicados/consumidos, endpoints HTTP e jobs, mais diagramas dinâmicos com os fluxos passo a passo.

## Arquivos Gerados

Todos os diagramas estão em `docs/diagrams/`:

| Arquivo | Módulo | Endpoints | Consumers | Producers | Jobs |
|---------|--------|-----------|-----------|-----------|------|
| `onboarding.puml` | Onboarding | 2 | 2 | 1 evento | 3 |
| `identity.puml` | Identity | 2 | 3 | 4 eventos | 1 |
| `billing.puml` | Billing | 1 webhook | 1 | 7 eventos | 3 |
| `budgets.puml` | Budgets | 8 | 2 | 1 evento | 2 |
| `transactions.puml` | Transactions | 13 | 1 | 8 eventos | 2 |
| `categories.puml` | Categories | 4 | — | — | — |
| `card.puml` | Card | 6 | — | — | — |
| `agent.puml` | Agent | — (via canais) | 1 | 2 eventos | — |

## Mapa de Eventos (Inter-módulos)

```
Billing ──────────────► billing.subscription.activated ──────────► Onboarding (MarkTokenPaid)
                                                        ──────────► Identity (ProjectSubscriptionEvent)
        ──────────────► billing.subscription.activated_without_token → Onboarding (HandlePaidWithoutToken)
        ──────────────► billing.subscription.{renewed,past_due,canceled,refunded} → Identity
        ──────────────► billing.subscription.expired_after_grace (interno)

Onboarding ───────────► onboarding.subscription_bound ───────────► Identity (SubscriptionBoundProjector)

Identity ─────────────► auth.principal_established ──────────────► Identity (AuthEventsConsumer — auditoria)
         ─────────────► auth.failed ─────────────────────────────► Identity (AuthEventsConsumer — auditoria)
         ─────────────► auth.unknown_user ──────────────────────► Identity (AuthEventsConsumer — auditoria)
         ─────────────► user.deleted ────────────────────────────► Identity (AnonymizeUserAuthEvents)

Transactions ─────────► transactions.transaction.{created,updated,deleted}.v1 → Transactions (MonthlySummaryRecompute)
             ─────────► transactions.card_purchase.{created,updated,deleted}.v1 → Transactions
             ─────────► budgets.expense.committed.v1 ────────────► Budgets (EvaluateAlert)

Budgets ──────────────► budgets.expense.committed.v1 ────────────► Budgets (EvaluateAlert — loop interno)

Agent ────────────────► agent.intent.executed.v1 (auditoria)
      ────────────────► agent.intent.rejected.v1 (auditoria)
```

## Como Visualizar

### VS Code (PlantUML extension)
1. Instale a extensão **PlantUML** (jebbs.plantuml)
2. Abra qualquer arquivo `.puml`
3. `Ctrl+Shift+P` → "PlantUML: Preview Current Diagram"

### IntelliJ / GoLand
1. Instale o plugin **PlantUML integration**
2. Abra o arquivo `.puml` e clique no ícone de preview

### CLI (plantuml.jar)
```bash
java -jar plantuml.jar docs/diagrams/*.puml
```

### Online
Cole o conteúdo em https://www.plantuml.com/plantuml/uml/

## Estrutura de Cada Arquivo `.puml`

Cada arquivo contém múltiplos diagramas separados por `@startuml ... @enduml`:

1. **Container Diagram** — visão estática com todos os containers, atores e relacionamentos
2. **Dynamic Diagrams** — um por fluxo principal, mostrando a sequência passo a passo

## Módulos Documentados

### Onboarding
- **Fluxo 1:** Checkout → Pagamento Kiwify → Ativação WhatsApp
- **Fluxo 2:** Outreach Job (tokens PAID sem ativação após 24h)
- **Fluxo 3:** Expiração de Token Orphan
- **Fluxo 4:** Ativação via Telegram

### Identity
- **Fluxo 1:** Upsert User (criar / atualizar / reanimar em 180d)
- **Fluxo 2:** Establish Principal (WhatsApp → UserID, multi-path)
- **Fluxo 3:** Subscription Event → Entitlement (pending → committed)
- **Fluxo 4:** User Deleted → Anonymize Auth Events
- **Fluxo 5:** Gateway Auth Middleware (HMAC-SHA256, janela 60s)

### Billing
- **Fluxo 1:** Webhook Kiwify → validação HMAC → publicação de evento
- **Fluxo 2:** Reconciliação com Kiwify (detecção de discrepâncias)
- **Fluxo 3:** Grace Expiration (PAST_DUE 3d → expired_after_grace)

### Budgets
- **Fluxo 1:** DRAFT → ACTIVE → Despesas → Avaliação de Alertas
- **Fluxo 2:** Expense Committed (via Transactions) → EvaluateAlert

### Transactions
- **Fluxo 1:** CRUD Transaction → Recomputa Monthly Summary (com Coalescer)
- **Fluxo 2:** Recurring Materializer (DMMF: DecideMaterializeForDay)
- **Fluxo 3:** Card Purchase → Cálculo de Fatura

### Categories
- **Fluxo 1:** Listagem com Cache ETag (If-None-Match)
- **Fluxo 2:** Busca Semântica no Dicionário

### Card
- **Fluxo 1:** CRUD de Cartão (com soft delete em cascade)
- **Fluxo 2:** Cálculo de Fatura (InvoiceFor por ciclo de fechamento)

### Agent
- **Fluxo 1:** Mensagem Natural → LLM → Intent → Dispatch → Reply
- **Fluxo 2:** Circuit Breaker + Fallback Chain (Claude → OpenAI → degradação)
