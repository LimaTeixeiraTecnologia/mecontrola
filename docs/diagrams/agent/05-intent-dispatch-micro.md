# Intent Dispatch — Nivel Micro

Documentacao detalhada do roteamento de intents dentro do IntentRouter, cobrindo todos os 17 kinds, handlers correspondentes e fluxos especiais (onboarding LLM, budget conversation, fallback).

## Referencias de codigo

| Componente | Arquivo |
|---|---|
| IntentRouter | `internal/agent/application/services/intent_router.go` |
| Intent kinds | `internal/agent/domain/intent/intent.go` |
| ParseInbound | `internal/agent/application/usecases/parse_inbound.go` |
| Agent module (wiring) | `internal/agent/module.go` |
| Budget draft domain | `internal/agent/domain/budgetdraft/` |
| Binding adapters | `internal/agent/infrastructure/binding/` |
| Onboarding runner | `internal/agent/infrastructure/onboarding/` |

---

## Todos os Intent Kinds e Handlers

```mermaid
flowchart TD
    START([ParsedIntent recebida]) --> KIND{intent.Kind}

    KIND -->|KindLogExpense| H1["ExpenseLogger\nlog_transaction_from_agent.go"]
    KIND -->|KindLogIncome| H2["IncomeLogger\nvia ExpenseLogger direction=income"]
    KIND -->|KindLogCardPurchase| H3["CardPurchaseLogger\nlog_card_purchase_from_agent.go"]
    KIND -->|KindQueryCard| H4["CardInvoiceReader + CardLister\nquery_card.go"]
    KIND -->|KindListCards| H5["CardLister\nlist_cards.go"]
    KIND -->|KindCreateCard| H6["CardCreator\ncreate_card.go"]
    KIND -->|KindCountCards| H7["CardCounter\ncount_cards.go"]
    KIND -->|KindMonthlySummary| H8["MonthlySummaryReader\nmonthly_summary.go"]
    KIND -->|KindHowAmIDoing| H9["MonthlySummaryReader\nhow_am_i_doing.go"]
    KIND -->|KindListTransactions| H10["TransactionLister\nlist_transactions.go"]
    KIND -->|KindDeleteLastTransaction| H11["LastDeleter\ndelete_last_transaction.go"]
    KIND -->|KindEditLastTransaction| H12["LastEditor\nedit_last_transaction.go"]
    KIND -->|KindCreateRecurring| H13["RecurringCreator\ncreate_recurring.go"]
    KIND -->|KindListRecurring| H14["RecurringLister\nlist_recurring.go"]
    KIND -->|KindQueryCategory| H15["CategorySearcher\nquery_category.go"]
    KIND -->|KindQueryGoal| H16["GoalReader\nquery_goal.go"]
    KIND -->|KindConfigureBudget| H17["BudgetConvoOrCommitter\nconfigure_budget.go"]
    KIND -->|KindUnknown| H18["FallbackReply\nComposeConversationalReply"]

    H1 & H2 & H3 & H4 & H5 & H6 & H7 & H8 & H9 --> REPLY
    H10 & H11 & H12 & H13 & H14 & H15 & H16 & H17 & H18 --> REPLY

    REPLY["reply string\nWhatsAppGateway.SendTextMessage"]
```

---

## Fluxo Completo do IntentRouter

```mermaid
sequenceDiagram
    autonumber

    participant AGT as AgentRoute
    participant IR as IntentRouter
    participant IP as IntentParser
    participant BSS as BudgetSessionStore
    participant BC as BudgetCancelCheck
    participant ONB as OnboardingCheck
    participant ONB_RUN as OnboardingRunner
    participant HANDLER as Intent Handler
    participant FALLBACK as ConversationalReply
    participant EVT as EventPublisher
    participant GW as WhatsAppGateway

    AGT->>+IR: RouteWhatsApp(ctx, principal, msg)

    alt msg.Text vazio
        IR->>GW: SendTextMessage fallbackMissingText
        IR-->>AGT: RouteResult empty_text
    end

    IR->>+ONB: IsOnboarding(ctx, userID)
    ONB-->>-IR: bool

    alt usuario em onboarding
        IR->>+ONB_RUN: RunOnboardingTurn(ctx, userID, text)
        Note over ONB_RUN: modelo: anthropic/claude-haiku-4.5<br/>MaxTokens: 512<br/>gerencia fases via use cases:<br/>GetContext, SaveObjective, SaveIncome<br/>SaveCard, SaveBudgetSplits<br/>MarkFirstTx, SetPhase, Complete
        ONB_RUN-->>-IR: reply string
        IR->>GW: SendTextMessage reply
        IR-->>AGT: RouteResult routed kind=onboarding
    end

    IR->>+BSS: GetDraft(ctx, userID)
    BSS-->>-IR: BudgetDraft ou nil

    alt BudgetDraft ativo
        IR->>+BC: matchesBudgetCancel(text)
        Note over BC: palavras: cancelar, cancela,<br/>deixa pra la, esquece, parar
        alt texto e cancelamento
            BC-->>IR: true
            IR->>BSS: DeleteDraft
            IR->>GW: SendTextMessage budgetCancelledText
            IR-->>AGT: RouteResult budget_cancelled
        end
        BC-->>IR: false

        IR->>+HANDLER: BudgetConvo.Execute(ctx, userID, text, draft)
        Note over HANDLER: conversa LLM coleta splits por categoria<br/>modelo primario Gemini Flash Lite
        HANDLER-->>-IR: reply, newDraft
        IR->>BSS: SaveDraft newDraft
        IR->>GW: SendTextMessage reply
        IR-->>AGT: RouteResult routed kind=configure_budget
    end

    IR->>+IP: Parse(ctx, userID, text)
    IP-->>-IR: ParsedIntent

    alt Parse falhou ErrFallbackChainExhausted
        IR->>GW: SendTextMessage fallbackParseError
        IR->>EVT: PublishRejected reason=parse_error
        IR-->>AGT: RouteResult parse_error
    end

    alt DirectReply (LLM retornou texto livre)
        IR->>+FALLBACK: Execute(ctx, text)
        Note over FALLBACK: segunda chamada LLM com FreeText=true<br/>MaxTokens: AGENT_LLM_PROSE_MAX_TOKENS=200
        FALLBACK-->>-IR: reply
        IR->>GW: SendTextMessage reply
        IR->>EVT: PublishExecuted kind=direct_reply
        IR-->>AGT: RouteResult routed kind=direct_reply
    end

    IR->>+HANDLER: dispatch(ctx, principal, intent)
    alt Handler falhou
        HANDLER-->>IR: error
        IR->>GW: SendTextMessage fallbackUsecaseError
        IR->>EVT: PublishRejected reason=usecase_error
        IR-->>AGT: RouteResult usecase_error
    end
    HANDLER-->>-IR: reply

    IR->>EVT: PublishExecuted intent event
    IR->>GW: SendTextMessage reply
    IR-->>-AGT: RouteResult routed
```

---

## Tabela Completa: Kind Handler Modulo

| Kind | Handler / Use Case | Modulo | Operacao |
|------|-------------------|--------|---------|
| `KindLogExpense` | `ExpenseLogger` | Transactions | Cria Transaction direction=outcome |
| `KindLogIncome` | `ExpenseLogger` | Transactions | Cria Transaction direction=income |
| `KindLogCardPurchase` | `CardPurchaseLogger` | Transactions | Cria CardPurchase com parcelas |
| `KindQueryCard` | `CardInvoiceReader + CardLister` | Card | Lista cartoes e busca fatura do mes |
| `KindListCards` | `CardLister` | Card | Lista cartoes (limit 200) |
| `KindCreateCard` | `CardCreator` | Card | Cria cartao com closing_day, due_day |
| `KindCountCards` | `CardCounter` | Card | Conta cartoes do usuario |
| `KindMonthlySummary` | `MonthlySummaryReader` | Budgets | Resumo receita/despesa/saldo do mes |
| `KindHowAmIDoing` | `MonthlySummaryReader` | Budgets | Versao conversacional do resumo |
| `KindListTransactions` | `TransactionLister` | Transactions | Lista ultimas N transacoes |
| `KindDeleteLastTransaction` | `LastDeleter` | Transactions | Soft-delete da ultima transacao |
| `KindEditLastTransaction` | `LastEditor` | Transactions | Edita ultima transacao |
| `KindCreateRecurring` | `RecurringCreator` | Transactions | Cria recorrencia com day_of_month |
| `KindListRecurring` | `RecurringLister` | Transactions | Lista recorrencias ativas |
| `KindQueryCategory` | `CategorySearcher` | Categories | Busca categoria por nome |
| `KindQueryGoal` | `GoalReader` | Budgets | Consulta meta/objetivo de orcamento |
| `KindConfigureBudget` | `BudgetConvo / BudgetCommitter` | Budgets | Inicia ou continua config de orcamento |
| `KindUnknown` | `ComposeConversationalReply` | Agent LLM | Resposta livre via LLM |

---

## rawIntentDTO — Schema da Resposta LLM

```mermaid
classDiagram
    class rawIntentDTO {
        +string Kind
        +int64 AmountCents
        +string Merchant
        +string CategoryHint
        +string PaymentMethod
        +string CardHint
        +string CategoryName
        +string GoalName
        +string CardName
        +string Nickname
        +string RefMonth
        +string RawText
        +int Installments
        +string Direction
        +string Frequency
        +int DayOfMonth
        +int ClosingDay
        +int DueDay
        +int64 LimitCents
    }

    class Intent {
        +string Kind
        +int64 AmountCents
        +string Merchant
        +string CategoryHint
        +string PaymentMethod
        +string CardHint
        +string RefMonth
        +string RawText
        +int Installments
        +string Direction
        +string Frequency
        +int DayOfMonth
        +string CategoryName
        +string GoalName
        +string CardName
        +string Nickname
        +int ClosingDay
        +int DueDay
        +int64 LimitCents
        +string ResponseHint
        +string DirectReply
        +string Error
    }

    rawIntentDTO --> Intent : mapeado por ParseInbound
```

---

## Fluxo Especial: Onboarding com LLM

```mermaid
sequenceDiagram
    participant IR as IntentRouter
    participant ONB as OnboardingRunner
    participant LLM as FallbackChain claude-haiku-4.5
    participant UC as OnboardingLLM UseCases

    IR->>+ONB: RunOnboardingTurn(ctx, userID, text)
    ONB->>+UC: GetContext(ctx, userID)
    UC-->>-ONB: OnboardingContext fase atual

    ONB->>+LLM: Interpret(ctx, LLMRequest)
    Note over LLM: modelo: anthropic/claude-haiku-4.5<br/>MaxTokens: 512<br/>schema estruturado de resposta
    LLM-->>-ONB: acao a tomar

    alt fase collect_objective
        ONB->>UC: SaveObjective
    else fase collect_income
        ONB->>UC: SaveIncome
    else fase collect_card
        ONB->>UC: SaveCard
    else fase collect_budget
        ONB->>UC: SaveBudgetSplits
    else fase complete
        ONB->>UC: Complete
        ONB->>UC: MarkFirstTx
    end

    ONB->>UC: SetPhase(nextPhase)
    ONB-->>-IR: reply string
```

---

## State Machine: Budget Configuration (Sessao Multi-turn)

```mermaid
stateDiagram-v2
    [*] --> Idle : usuario autenticado

    Idle --> BudgetSession : KindConfigureBudget detectada
    BudgetSession --> BudgetSession : conversa LLM coleta splits
    BudgetSession --> Committed : todos splits coletados BudgetCommitter
    BudgetSession --> Idle : matchesBudgetCancel true DeleteDraft
    Committed --> Idle : orcamento salvo sessao encerrada
```

---

## Textos de Fallback

| Situacao | Texto |
|----------|-------|
| Mensagem vazia | Nao recebi nenhuma mensagem. Me conta o que voce precisa nas suas financas |
| Erro de parse LLM | Nao entendi direito. Pode reformular? Posso te ajudar com cartoes, orcamento e lancamentos. |
| Erro em use case | Tive uma instabilidade para consultar isso agora. Tente de novo em instantes |
| Registro indisponivel | Ainda nao consigo registrar lancamentos por aqui. Ja ja isso fica disponivel pra voce |
| Sem transacoes | Nao encontrei nenhum lancamento recente seu para mexer. Quer registrar um agora? |
| Budget cancelado | Ok, cancelei a configuracao do orcamento. Quando quiser, e so chamar de novo. |

---

## RouteResult Outcomes

| Outcome | Significado |
|---------|------------|
| `routed` | Intent processada e resposta enviada com sucesso |
| `fallback` | Resposta via LLM conversacional kind=unknown |
| `parse_error` | FallbackChain esgotada, enviou mensagem de erro |
| `usecase_error` | Use case falhou, enviou mensagem de instabilidade |
| `missing_resolver` | Canal sem gateway configurado |
| `reply_failed` | Gateway de envio falhou |
| `empty_text` | Mensagem vazia recebida |
