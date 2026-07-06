<!-- spec-hash-prd: 1134ba7717b0f9dea2a79fc428631ad01153d5f0275fafff4d883e26ddc2765d -->

# Especificação Técnica — Conversa Agentiva Fluida para Registro Financeiro

## Resumo Executivo

A solução evolui a conversa financeira do `mecontrola-agent` criando uma camada de pendência conversacional durável no consumidor `internal/agents`, construída sobre o kernel genérico `internal/platform/workflow`. O desenho mantém o kernel de workflow intacto, adiciona apenas uma extensão compatível no contexto de identidade do `internal/platform/agent` para expor `ThreadID`, usa `workflow.Engine[PendingEntryState]` com snapshot durável em `workflow_runs`, e intercepta mensagens WhatsApp antes do loop aberto do agente quando existir pendência ativa para o usuário.

O fluxo de escrita continua delegado aos use cases reais por meio das interfaces existentes `CategoriesReader` e `TransactionsLedger`. A decisão crítica é separar três responsabilidades: o agente extrai intenção e chama tools; a pendência tipada preserva slots e decide retomada/cancelamento/substituição; `internal/categories` e `internal/transactions` continuam sendo as autoridades para categoria e persistência. O critério de produção é 0 falso positivo: nenhuma escrita sem raiz + subcategoria folha canônicas, nenhuma resposta de sucesso sem retorno real da tool/use case, e nenhum aceite de retomada sem harness determinístico com Run auditável.

A partir da `spec-version 3` do PRD, quatro invariantes adicionais moldam o desenho: (1) **gate de confirmação obrigatório antes de toda escrita** — nenhuma tool de escrita persiste de forma síncrona; toda operação (registro, edição, recorrência) abre a pendência durável e só persiste após aceite humano explícito no passo `aguardando confirmação`, inclusive quando o lançamento já está totalmente especificado e sem ambiguidade; isso revoga a política anterior de escrita "report-only sem confirmação"; (2) a criação de recorrência é atendida **estendendo a interface `TransactionsLedger`** consumida pelo agente para expor criação de template recorrente, delegando a `internal/transactions/application/usecases/create_recurring_template.go`; (3) a edição preserva `TargetTransactionID` e `TargetVersion` no snapshot da pendência; (4) a seleção entre múltiplos candidatos aceita índice numérico ou nome, resolvido pela decisão pura antes da revalidação canônica.

## Arquitetura do Sistema

### Componentes Novos ou Modificados

- `internal/agents/application/workflows/pending_entry_state.go`: novos tipos fechados para pendência conversacional, operação financeira, slot aguardado, status de pendência e decisão de retomada.
- `internal/agents/application/workflows/pending_entry_workflow.go`: workflow durável `PendingEntryWorkflowID = "pending-entry"` para iniciar, retomar, substituir, cancelar, expirar e concluir pendências de registro/edição/recorrência.
- `internal/agents/application/usecases/pending_entry_continuer.go`: use case fino que carrega/retoma a pendência antes do agente aberto, análogo ao `DestructiveConfirmContinuer`.
- `internal/platform/agent/identity_context.go`: hoje `InboundIdentityFromContext(ctx) (resourceID, messageID string, itemSeq int, ok bool)` NÃO expõe `ThreadID`, embora `InboundRequest` já o carregue. Deve ser estendido para persistir `threadID` no contexto de tool invocation via novo `WithToolInvocationContext`/`InboundExecutionFromContext(ctx) (resourceID, threadID, messageID string, itemSeq int, ok bool)`, mantendo `InboundIdentityFromContext` como wrapper legado. Extensão genérica de chave opaca; não introduz semântica de domínio no substrato.
- `internal/agents/application/usecases/register_entry.go`: deve passar a retornar payload suficiente para abrir pendência tanto no caminho `agent.ToolOutcomeClarify` (slot faltante) quanto no caminho totalmente resolvido (que agora também abre pendência no estado `aguardando confirmação` em vez de escrever).
- `internal/agents/application/usecases/register_attempt.go`: novo use case de aplicação para orquestrar a tentativa de registro/edição/recorrência; SEMPRE inicia `pending-entry` antes de devolver resposta à tool — no estado `aguardando categoria`/`pagamento`/`cartão`/`data` quando faltar slot, ou diretamente em `aguardando confirmação` quando tudo já estiver resolvido. Nunca escreve de forma síncrona.
- `internal/agents/application/tools/register_expense.go`, `register_income.go`, e as tools de edição e de recorrência: continuam adapters finos; delegam ao use case de tentativa e retornam um outcome de "pendência aberta" (clarify/confirmação) somente após a pendência durável ter sido criada com sucesso. A tool de edição resolve e repassa a transação alvo; a tool de recorrência repassa os campos de recorrência.
- `internal/agents/application/interfaces/transactions_ledger.go`: deve ganhar um método de criação de template recorrente (ex.: `CreateRecurringTemplate(ctx, RawRecurringTemplate) (EntryRef, error)`), com o adapter em `infrastructure/binding` delegando a `internal/transactions` `CreateRecurringTemplate`. Este é o único ponto que amplia a superfície da autoridade de persistência.
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`: deve chamar `PendingEntryContinuer` antes do onboarding e antes de `HandleInbound`, mantendo ordem explícita de resolução.
- `internal/agents/module.go`: deve montar `workflow.Engine[PendingEntryState]`, definição do workflow, continuer, reaper de pendências e injetar o resolver no consumer.
- `internal/agents/application/interfaces/types.go`: deve expor tipos de candidato categorial com `rootCategoryId`, `rootSlug`, `subcategoryId`, `subcategorySlug`, path e versão editorial quando necessário para resposta/auditoria.

### Fluxo de Dados

```text
WhatsApp inbound
  -> WhatsAppInboundConsumer
     -> PendingEntryContinuer.Continue(userID, peer, message)
        -> workflow.Engine[PendingEntryState].Resume("resourceID:threadID:pending-entry", merge-patch)
        -> CategoriesReader.SearchDictionary / ResolveForWrite
        -> TransactionsLedger.CreateTransaction ou UpdateTransaction quando seguro
        -> resposta curta ou conclusão
     -> DestructiveConfirmContinuer quando não houver pendência de registro
     -> ResolveOnboardingOrAgent quando não houver pendência/confirm
     -> HandleInbound -> AgentRuntime.Execute -> agent loop/tool-calling
```

Nova frase completa de lançamento durante pendência é tratada como substituição: o workflow encerra a pendência anterior com status fechado `replaced`, devolve `handled=false` ao consumer e deixa a mensagem seguir para `HandleInbound` como nova operação explícita. A pendência substituída não pode executar escrita em turnos posteriores.

### Ordem de Resolução no Consumer

1. Validar payload WhatsApp e timeout.
2. `PendingEntryContinuer`: prioridade máxima para respostas curtas, cancelamento, expiração e substituição de pendência financeira.
3. `DestructiveConfirmContinuer`: mantém confirmações destrutivas/sensíveis existentes.
4. `ResolveOnboardingOrAgent`: mantém onboarding durável.
5. `HandleInbound`: executa o agente aberto.

Essa ordem reduz falso roteamento: pendências financeiras são mais específicas que onboarding e agente aberto; confirmação destrutiva continua separada para operações de alteração/exclusão.

### Correlação de Pendência

A correlação funcional da pendência deve ser por thread, não apenas por usuário. A key canônica é:

```text
<resourceID>:<threadID>:pending-entry
```

`resourceID` e `threadID` permanecem opacos e não podem virar label de métrica. Essa escolha cumpre a regra de uma pendência ativa por thread e impede aplicar resposta de outro canal ou conversa ao lançamento errado.

## Design de Implementação

### Estados Fechados

Adicionar tipos no consumidor `internal/agents/application/workflows`, não no kernel:

```go
type PendingStatus int

const (
	PendingStatusActive PendingStatus = iota + 1
	PendingStatusCompleted
	PendingStatusCancelled
	PendingStatusExpired
	PendingStatusReplaced
)

type AwaitingSlot int

const (
	AwaitingSlotCategory AwaitingSlot = iota + 1
	AwaitingSlotPaymentMethod
	AwaitingSlotCard
	AwaitingSlotDate
	AwaitingSlotConfirmation
	AwaitingSlotCorrection
)

type PendingOperationKind int

const (
	PendingOpRegisterExpense PendingOperationKind = iota + 1
	PendingOpRegisterIncome
	PendingOpEditEntry
	PendingOpCreateRecurrence
)
```

Cada tipo deve seguir o padrão local: `iota + 1`, `String()`, `IsValid()`, `Parse*`, erro sentinel e testes de ida/volta. Não usar string livre como estado público.

`AwaitingSlotConfirmation` é o **gate terminal obrigatório** antes de qualquer escrita: toda operação transita para este estado após os demais slots estarem preenchidos e válidos, e a persistência só ocorre quando o resume traz aceite humano explícito. Não existe caminho de escrita que pule este estado.

### Estado Durável

`PendingEntryState` deve ser o snapshot único do workflow:

```go
type PendingEntryState struct {
	Status          PendingStatus       `json:"status"`
	Awaiting        AwaitingSlot        `json:"awaiting"`
	Operation       PendingOperation    `json:"operation"`
	UserID          uuid.UUID           `json:"userId"`
	ThreadID        string              `json:"threadId"`
	MessageID       string              `json:"messageId"`
	OriginalText    string              `json:"originalText"`
	ResumeText      string              `json:"resumeText"`
	AmountCents     int64               `json:"amountCents"`
	Description     string              `json:"description"`
	PaymentMethod   string              `json:"paymentMethod"`
	CardID          *uuid.UUID          `json:"cardId"`
	Installments    int                 `json:"installments"`
	OccurredAt      string              `json:"occurredAt"`
	Kind            interfaces.CategoryKind `json:"kind"`
	Candidates      []PendingCategoryCandidate `json:"candidates"`
	CategoryVersion int64               `json:"categoryVersion"`
	RepromptCount   int                 `json:"repromptCount"`
	SuspendedAt     time.Time           `json:"suspendedAt"`
	ResponseText    string              `json:"responseText"`
	ResourceID      uuid.UUID           `json:"resourceId"`
	ErrorCode       string              `json:"errorCode"`
	OperationKind     PendingOperationKind `json:"operationKind"`
	TargetTransactionID *uuid.UUID         `json:"targetTransactionId"`
	TargetVersion       int64              `json:"targetVersion"`
	Frequency           string             `json:"frequency"`
	RecurrenceDayOfMonth int               `json:"recurrenceDayOfMonth"`
	ConfirmRepromptCount int               `json:"confirmRepromptCount"`
}
```

`OperationKind` discrimina registro/edição/recorrência sem `switch` de domínio no roteamento (resolução por mapa fechado). `TargetTransactionID`/`TargetVersion` só são preenchidos em edição, resolvidos server-side pela tool de edição, nunca fornecidos pelo LLM. `Frequency`/`RecurrenceDayOfMonth` só em recorrência. `ConfirmRepromptCount` isola o reprompt único do gate de confirmação do `RepromptCount` de coleta de slot.

O estado deve evitar dados sensíveis desnecessários. Ele contém apenas dados já presentes no inbound financeiro e necessários para retomar a operação. `ThreadID` é opaco; não usar como label de métrica.

### Candidato Categorial Canônico

Toda opção persistível deve carregar raiz e folha:

```go
type PendingCategoryCandidate struct {
	RootCategoryID  uuid.UUID `json:"rootCategoryId"`
	RootSlug        string    `json:"rootSlug"`
	SubcategoryID   uuid.UUID `json:"subcategoryId"`
	SubcategorySlug string    `json:"subcategorySlug"`
	Path            string    `json:"path"`
	MatchedTerm     string    `json:"matchedTerm"`
	Score           float64   `json:"score"`
	Confidence      string    `json:"confidence"`
	MatchQuality    string    `json:"matchQuality"`
	MatchReason     string    `json:"matchReason"`
}
```

`CategoriesReader.SearchDictionary` fornece candidatos, não autorização de escrita. A implementação deve enriquecer slugs a partir de `ResolveForWrite` quando houver par raiz+folha escolhido ou de `ListCategories` para apresentação de múltiplas opções. Persistência só pode seguir após `ResolveForWrite(rootID, subcategoryID, kind, expectedVersion)` retornar sucesso e `internal/transactions` aprovar a evidência pelo `CategoryWriteGate`.

### Interfaces Chave

Consumer option novo:

```go
type pendingEntryResolver interface {
	Continue(ctx context.Context, userID, peer, message, messageID string) (PendingEntryResult, error)
}
```

Resultado do continuer:

```go
type PendingEntryResult struct {
	Handled bool
	Message string
	Mode    PendingEntryMode
}
```

`Mode` deve distinguir `replied`, `passThrough`, `completed`, `cancelled`, `expired` e `replaced`. Em `replaced`, `Handled=false` para permitir que a mesma mensagem siga para o agente como nova operação.

Contexto de identidade para tools:

```go
func InboundExecutionFromContext(ctx context.Context) (resourceID, threadID, messageID string, itemSeq int, ok bool)
```

`InboundIdentityFromContext` pode ser mantida como wrapper legado para preservar compatibilidade. A regra técnica é que tools de escrita que abrem pendência precisam de `resourceID`, `threadID`, `messageID` e `itemSeq` server-side.

### Decisão Determinística

Criar funções puras no consumidor:

- `DecidePendingResume(state PendingEntryState, msg PendingMessage, now time.Time) (PendingDecision, error)`
- `DecideCategoryChoice(state PendingEntryState, candidates []PendingCategoryCandidate, text string) (CategoryChoiceDecision, error)`
- `DecideNewOperationReplacement(state PendingEntryState, msg PendingMessage) PendingDecision`
- `DecideConfirmation(state PendingEntryState, msg PendingMessage, now time.Time) (ConfirmDecision, error)` — gate terminal de escrita; determinístico, sem IO/LLM.

Essas funções não devem fazer IO, chamar LLM, acessar banco ou usar `context.Context`. Elas recebem `now` por parâmetro para testes determinísticos e retornam tipos fechados.

### Detecção de Nova Operação Completa

A techspec não deve introduzir parser paralelo amplo ao LLM. Para robustez sem duplicar agente, aplicar uma regra conservadora:

- Se a mensagem contém valor monetário reconhecível e verbo de lançamento (`gastei`, `paguei`, `comprei`, `recebi`, `ganhei`) e data/pagamento opcional, ela é nova operação completa.
- Se a mensagem é curta ou não contém valor monetário, ela é candidata a resposta da pendência.
- Se a mensagem contém cancelamento inequívoco (`cancela`, `deixa pra lá`, `não registra`), cancela a pendência.

Qualquer caso ambíguo que não seja nova operação completa nem slot compatível deve pedir uma única clarificação e manter a pendência.

### Abertura de Pendência

Toda tentativa de escrita (registro, edição, recorrência) abre a pendência antes de qualquer persistência. O use case de tentativa (`register_attempt`) decide o estado inicial:

- Se `RegisterEntry.classify` retornar `agent.ToolOutcomeClarify` (slot categorial faltante/ambíguo), abre em `AwaitingSlotCategory` com `outcome=clarify`.
- Se faltar pagamento/cartão/data, abre no slot correspondente.
- Se todos os slots já estiverem preenchidos e válidos (inclusive categoria inequívoca), abre **diretamente em `AwaitingSlotConfirmation`** — nunca escreve de forma síncrona.

A tool retorna sempre um outcome de "pendência aberta" (clarify ou pedido de confirmação) e nunca `success` no mesmo turno da tentativa. A abertura usa key `<resourceID>:<threadID>:pending-entry` via `workflow.Engine.Start`.

Se a abertura da pendência falhar, a tool deve retornar erro e o runtime não pode emitir sucesso. Se a pendência já existir, `ErrRunAlreadyExists` deve ser tratado como retomada ou substituição conforme decisão determinística. Não criar schema paralelo.

### Retomada de Pendência

Retomada usa `workflow.Engine.Resume` com JSON merge-patch:

```json
{"resumeText":"custo fixo","messageId":"wamid..."}
```

O step do workflow deve:

1. Validar expiração de 30 minutos usando `SuspendedAt`.
2. Decidir cancelamento, substituição, slot preenchido ou reprompt.
3. Resolver categoria via `SearchDictionary`.
4. Apresentar opções quando houver múltiplos candidatos plausíveis, como lista numerada.
5. Aceitar a escolha por índice numérico OU nome (via `DecideCategoryChoice`, pura) e validar por `ResolveForWrite`.
6. Transicionar para `AwaitingSlotConfirmation` e apresentar a confirmação final única quando todos os slots estiverem resolvidos.
7. Somente no resume seguinte, com aceite explícito, montar `interfaces.RawTransaction`/`RawUpdateTransaction`/`RawRecurringTemplate` com evidência categorial completa.
8. Chamar `TransactionsLedger.CreateTransaction`/`UpdateTransaction`/`CreateRecurringTemplate` conforme `OperationKind`, deixando `CategoryWriteGate` em `internal/transactions` como defesa final.
9. Confirmar sucesso somente com retorno real.

### Gate de Confirmação de Escrita

O passo `AwaitingSlotConfirmation` reutiliza o **contrato semântico** já validado no `destructive-confirm`, sem reusar o workflow destrutivo (a techspec rejeita expandir `destructive-confirm`). A decisão pura `DecideConfirmation(state, msg, now)` retorna:

- aceite explícito (`sim`/`confirmar`/`ok`/`pode`) → transição para escrita real;
- cancelamento explícito (`não`/`cancela`/`deixa pra lá`) → fecha `PendingStatusCancelled`, sem escrita;
- resposta ambígua (1ª vez) → `ConfirmRepromptCount` 0→1, re-suspende com re-pergunta única;
- resposta ambígua (2ª vez) → fecha `PendingStatusCancelled`, sem escrita;
- nova frase completa de lançamento → substituição (`PendingStatusReplaced`), `handled=false`;
- expiração (TTL 30 min via `SuspendedAt`) → fecha `PendingStatusExpired`, sem escrita.

O gate nunca invoca LLM (R-AGENT-WF-001.4) e é 100% determinístico. A escrita só ocorre no resume que carrega o aceite; `M-07` (escrita sem confirmação) é comprovado 0 pelo harness verificando a ordem confirm→write no Run auditável.

### Detecção de Escolha de Candidato por Número ou Nome

`DecideCategoryChoice(state, candidates, text)` deve, de forma pura:

- Se `text` for um índice válido (`1..len(candidates)`), selecionar o candidato correspondente.
- Caso contrário, casar `text` normalizado contra `SubcategorySlug`/nome legível dos candidatos; match único → selecionado; nenhum ou múltiplos → devolver ambiguidade para reprompt único.
- Nunca escolher automaticamente o primeiro candidato (RF-27/RF-35).

### Expiração e Reaper

O workflow deve usar TTL funcional de 30 minutos. Além da checagem no step, `module.go` deve registrar `workflow.NewStaleSuspendedReaper` para `PendingEntryWorkflowID` com `staleAfter >= 30*time.Minute`, idealmente `35*time.Minute` para tolerar pequenas diferenças de scheduler. Expiração observável deve fechar status e responder ao usuário quando ele tentar retomar.

### Idempotência

Escrita originada de pendência deve preservar:

- `OriginWamid`: message id da operação original ou confirmação final conforme regra idempotente já usada.
- `OriginItemSeq`: item sequence do inbound quando disponível no contexto.
- `OriginOperation`: `pending_entry_register`.
- `CategorySource`: `user_selected_candidate` para clarificação do usuário.

Não usar IDs fornecidos pelo LLM. A identidade server-side deve continuar vindo do contexto de inbound/tool ou `auth.Principal` injetado pelo adapter. Para conversa retomada, a chave idempotente deve ser estável por operação pendente, não pelo WAMID curto da resposta de categoria.

## Pontos de Integração

Não há integração externa nova. O desenho reutiliza:

- OpenRouter apenas via `internal/platform/llm` no loop do agente, já existente.
- Postgres existente via `internal/platform/workflow/infrastructure/postgres`.
- WhatsApp inbound existente via outbox consumer.
- `internal/categories` via `CategoriesReader`.
- `internal/transactions` via `TransactionsLedger`.

## Abordagem de Testes

### Testes Unitários

- `pending_entry_state_test.go`: `String`/`Parse`/`IsValid` para todos os enums fechados.
- `pending_entry_decision_test.go`: table-driven para resposta curta, cancelamento, expiração, nova operação completa, slot incompatível e substituição.
- `pending_entry_workflow_test.go`: uso de engine fake/in-memory para start/resume e merge-patch sem banco.
- `register_entry_test.go`: clarificação deve produzir outcome sem escrita e com dados suficientes para pendência.
- `whatsapp_inbound_consumer_test.go`: ordem de resolução pending -> destructive -> onboarding -> agent.

### Testes de Integração

Integration tests são obrigatórios porque a feature depende de Postgres/workflow durable/idempotência e falso positivo de escrita já é risco central. Usar build tag `integration` onde o projeto já aplica esse padrão.

- `internal/agents/infrastructure/binding/*_integration_test.go`: validar que pendência retomada persiste transação real e evidência categorial.
- DB-backed harness: validar `platform_runs`, `platform_messages`, `workflow_runs`, `workflow_steps`, `agents_write_ledger` e linha real em `transactions` quando houver escrita.
- `internal/platform/workflow/infrastructure/postgres/*_integration_test.go`: já cobre store; adicionar caso de `pending-entry` apenas se houver comportamento específico de consumidor.
- Teste de expiração: snapshot suspenso com `updated_at`/`SuspendedAt` antigo deve expirar e não escrever.
- Teste de substituição: pendência ativa seguida de nova frase completa não escreve pendência antiga e permite nova operação.

### Harness Determinístico

Criar harness em `internal/agents/application/agents` ou `internal/agents/application/usecases`, sem rede real:

- Provider fake determinístico para mensagens do agente.
- Doubles de `CategoriesReader`, `TransactionsLedger`, `CardManager` e workflow store.
- Captura de Run auditável quando o caminho passar por `AgentRuntime`.
- Assertions obrigatórias: estado final, tool calls esperadas, conteúdo de resposta, ausência/presença de escrita real, `RunStatus`, `ToolOutcome`, `workflow_runs`, `agents_write_ledger` e não duplicidade.

Cenários mínimos:

- CA-01: "mercado" -> clarify -> "custo fixo" -> resolve raiz + folha -> create transaction.
- CA-02: "mercado" -> clarify -> nova frase completa "farmácia" -> pendência antiga replaced -> nova operação segue.
- CA-03: "sim e pix" em pendência de categoria -> não confirma indevidamente; preenche só slot compatível ou pergunta.
- CA-04: múltiplos candidatos -> opções raiz+folha -> escolha -> ResolveForWrite -> write.
- CA-05: cancelamento explícito -> no write.
- CA-06: erro de ledger -> resposta sem sucesso.
- CA-07: replay idempotente -> sem duplicidade.
- CA-08: expiração 30 minutos -> no write.
- CA-09: raiz sem folha -> bloqueio.
- CA-10: cartão crédito sem cartão -> resolver cartão antes de write.
- CA-11: texto compatível com pendência substituída -> no write.
- CA-12: harness valida Run, tool calls e escrita real.
- CA-13: lançamento inequívoco -> abre direto em `AwaitingSlotConfirmation` -> "sim" -> write; sem "sim" -> no write.
- CA-14: confirmação ambígua -> reprompt único -> ambiguidade persistente -> cancel sem write.
- CA-15: múltiplos candidatos -> escolha por número e escolha por nome resolvem o mesmo par canônico -> confirm -> write.
- CA-16: recorrência com categoria válida -> confirm -> `CreateRecurringTemplate` real, idempotente, sem write antes do "sim".
- CA-17: edição com clarify -> confirm -> `UpdateTransaction` na transação alvo preservada, respeitando versão, sem criar nova.

### Gates de Validação

Para implementação:

```bash
go build ./internal/platform/... ./internal/agents/... ./internal/categories/... ./internal/transactions/...
go vet ./internal/platform/... ./internal/agents/... ./internal/categories/... ./internal/transactions/...
go test -race -count=1 ./internal/agents/... ./internal/categories/... ./internal/transactions/...
```

Além dos gates da skill `mastra`, rodar os greps de pureza do kernel, SQL em tools, cardinalidade de métricas e zero comentário Go de produção.

## Sequenciamento de Desenvolvimento

1. Tipos fechados (incluindo `PendingOperationKind`) e decisões puras de pendência — `DecidePendingResume`, `DecideCategoryChoice`, `DecideNewOperationReplacement`, `DecideConfirmation` — no consumidor `internal/agents/application/workflows`.
2. Workflow `pending-entry` com start/resume/cancel/expire/replaced e gate terminal `AwaitingSlotConfirmation`, sem chamadas reais de ledger.
3. Extensão de `internal/platform/agent/identity_context.go` para expor `threadID` (`InboundExecutionFromContext`), mantendo wrapper legado.
4. Extensão de `TransactionsLedger` com `CreateRecurringTemplate` e adapter de binding para `internal/transactions`.
5. Integração com `CategoriesReader` para candidatos raiz+folha, escolha por número/nome e `ResolveForWrite`.
6. Integração com `TransactionsLedger` (create/update/recurring) e idempotência.
7. `register_attempt` abrindo pendência sempre (clarify ou confirmação) a partir das tools de registro, edição e recorrência; nenhuma escrita síncrona.
8. Wiring em `module.go` (engine `PendingEntryState`, reaper) e ordem de resolvers no `WhatsAppInboundConsumer`.
9. Harness determinístico (CA-01..CA-17) e integração Postgres.
10. Reforço de prompt/instruções apenas para refletir o novo contrato e o gate de confirmação, sem depender dele como autoridade.

## Monitoramento e Observabilidade

Métricas novas com labels de baixa cardinalidade:

- `agents_pending_entry_total{outcome}` com outcomes `started|resumed|completed|cancelled|expired|replaced|error`.
- `agents_pending_entry_slot_total{slot,outcome}` com slots fechados.
- `agents_pending_entry_write_total{outcome}` com `success|replay|error|blocked`.
- `agents_pending_entry_duration_seconds{outcome}`.

Proibido label com `user_id`, `thread_id`, `resource_id`, `category_id`, `subcategory_id`, `message_id` ou texto do usuário. Logs podem conter `run_id`, `workflow`, `outcome`, `slot` e erro técnico; evitar texto bruto do usuário salvo quando já permitido pelo padrão de auditoria existente.

## Considerações Técnicas

### Decisões Chave

- ADR-001: Pendência conversacional como workflow durável no consumidor `internal/agents`.
- ADR-002: Contrato categorial raiz + subcategoria folha com IDs/slugs canônicos.
- ADR-003: Harness determinístico como gate primário de produção.
- ADR-004: Confirmação humana obrigatória antes de toda escrita, como gate terminal da pendência reutilizando o contrato semântico de confirmação.

### Alternativas Rejeitadas

- Resolver só por prompt do agente: rejeitado porque prompt não garante persistência de slots, não é auditável como estado e já falhou no caso motivador.
- Criar tabela própria de pendências: rejeitado porque o kernel de workflow já fornece snapshots duráveis, CAS, reaper e resume por merge-patch.
- Reutilizar `ConfirmState` ampliando `destructive-confirm`: rejeitado porque mistura confirmação destrutiva com coleta categorial e aumenta risco de regressão em operações sensíveis.
- Permitir categoria raiz sem folha: rejeitado pelo PRD e pelo contrato de transações.
- Scorer LLM como gate de aceite: rejeitado porque é probabilístico e não prova ausência de falso positivo.
- Escrita "report-only" sem gate de confirmação (política anterior): revogada na `spec-version 3`; toda escrita agora exige aceite humano explícito antes de persistir.
- Escrever de forma síncrona na tool quando o lançamento é inequívoco: rejeitado porque violaria o gate de confirmação obrigatório e removeria a evidência auditável de aceite.
- Reusar o workflow `destructive-confirm` para confirmar registros: rejeitado por misturar confirmação de escrita normal com operações destrutivas; reutiliza-se apenas o contrato semântico dentro de `pending-entry`.
- Adicionar template recorrente no consumidor: rejeitado; recorrência é responsabilidade de `internal/transactions`, exposta por extensão fina de `TransactionsLedger`.

### Riscos e Mitigações

- Risco: parser conservador classificar nova operação completa como resposta de pendência. Mitigação: regra determinística exige valor monetário + verbo de lançamento; cenários canônicos no harness.
- Risco: slugs não disponíveis em `SearchDictionary`. Mitigação: enriquecer candidatos via `ResolveForWrite` para par escolhido e via `ListCategories` para opções apresentadas.
- Risco: `agent.Result.ToolOutcome` não representar `clarify` de tool bem-sucedida. Mitigação: não depender do outcome agregado do runtime; abrir pendência no use case chamado pela tool antes de retornar `outcome=clarify`.
- Risco: duplicidade de escrita no resume. Mitigação: manter `OriginWamid`/`OriginOperation` e `IdempotentWrite`; teste de replay obrigatório.
- Risco: concorrência em mensagens simultâneas. Mitigação: workflow store com CAS e key única `<resourceID>:<threadID>:pending-entry`; conflito retorna resultado seguro sem escrita duplicada.
- Risco: expiração por relógio difícil de testar. Mitigação: decisões puras recebem `now time.Time`; workflow usa `time.Now().UTC()` apenas na borda conforme regra local.

### Conformidade com Padrões

- `go-implementation`: alterações futuras devem seguir task type `cross-cutting`, com validação boundary/global proporcional.
- `mastra`: comportamento novo no consumidor `internal/agents`, substrato intacto, tools finas e Run auditável.
- DMMF: estados fechados, smart constructors/parsers, `Decide*` puro, pipeline parse -> validate -> decide -> persist -> respond.
- AGENTS: sem comentários Go de produção, sem `init`, sem `var _ Interface = (*Type)(nil)`, sem SQL em tools, sem `clock.Clock`.
- Segurança: input do WhatsApp e LLM não confiáveis; validar antes de persistir; não logar segredos ou texto sensível desnecessário.

### Mapeamento RF -> Decisão -> Teste

| RFs | Decisão técnica | Testes |
| --- | --- | --- |
| RF-01..RF-09 | `PendingEntryWorkflow` durável com estados fechados e TTL 30 min | unit workflow, integration expiration |
| RF-10..RF-14, RF-27..RF-30, RF-35 | categoria raiz+folha via `SearchDictionary` + `ResolveForWrite` | category decision unit, binding integration |
| RF-15..RF-17, RF-31..RF-32 | decisão pura para resposta curta, substituição e cancelamento | table-driven decision tests |
| RF-18..RF-23 | pipeline determinístico e escrita via `TransactionsLedger` | harness write/no-write/replay |
| RF-24 | resposta WhatsApp normalizada pelo consumer existente | consumer unit tests |
| RF-25..RF-26 | uma pendência ativa por `<resourceID>:<threadID>:pending-entry` | concurrency/idempotency tests |
| RF-33..RF-34 | harness determinístico + Run auditável | harness CA-12 |
| RF-36..RF-37 | aderência `go-implementation`/`mastra` | gates build/vet/race + checklist |
| RF-38..RF-41 | gate `AwaitingSlotConfirmation` terminal + `DecideConfirmation` puro; toda escrita via pendência | `pending_entry_confirmation_test.go`, harness CA-13/CA-14, M-07 |
| RF-42 | `DecideCategoryChoice` aceita índice ou nome | decision unit + harness CA-15 |
| RF-43 | recorrência/edição via ledger estendido, idempotência e gate de confirmação | binding integration + harness CA-16/CA-17 |

### Arquivos Relevantes e Dependentes

- `internal/agents/application/workflows/confirm_state.go`
- `internal/agents/application/workflows/destructive_confirm_workflow.go`
- `internal/agents/application/usecases/destructive_confirm_continuer.go`
- `internal/agents/application/usecases/register_entry.go`
- `internal/agents/application/tools/register_expense.go`
- `internal/agents/application/tools/register_income.go`
- `internal/agents/application/interfaces/categories_reader.go`
- `internal/agents/application/interfaces/types.go`
- `internal/agents/infrastructure/binding/categories_reader_adapter.go`
- `internal/agents/infrastructure/binding/transactions_ledger_adapter.go`
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`
- `internal/agents/module.go`
- `internal/platform/workflow/engine.go`
- `internal/platform/workflow/infrastructure/postgres/store.go`
- `internal/platform/agent/identity_context.go`
- `internal/categories/application/usecases/search_dictionary.go`
- `internal/categories/application/usecases/resolve_category_for_write.go`
- `internal/transactions/application/interfaces/category_write_gate.go`
- `internal/transactions/domain/valueobjects/category_write_evidence.go`
- `internal/transactions/application/usecases/create_transaction.go`
- `internal/transactions/application/usecases/update_transaction.go`
- `internal/transactions/application/usecases/create_recurring_template.go`
- `internal/agents/application/interfaces/transactions_ledger.go`
- `internal/agents/application/interfaces/discriminators.go`
