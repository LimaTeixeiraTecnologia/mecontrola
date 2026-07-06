# Tarefa 4.0: Integração TransactionsLedger, Edição, Recorrência, Idempotência e CategoryWriteGate

<critical>Ler prd.md, techspec.md e scenarios.md desta pasta — tarefa invalidada se pulado</critical>

## Visão Geral

Implementar o step de escrita real do workflow `pending-entry`: montar `interfaces.RawTransaction` com evidência categorial completa, chamar `TransactionsLedger.CreateTransaction`, `TransactionsLedger.UpdateTransaction` (edição) ou `TransactionsLedger.CreateRecurringTemplate` (recorrência), garantir idempotência por `OriginWamid`/`OriginOperation`, tratar erro de ledger sem declarar sucesso e deixar `CategoryWriteGate` em `internal/transactions` como defesa final. Toda escrita (registrar/editar/recorrência) só ocorre APÓS o gate universal `AwaitingSlotConfirmation → "sim"` (RF-38); nenhuma escrita é síncrona. Edição (RF-43) e recorrência (RF-25) são incorporadas nesta tarefa.

<requirements>
- Escrita só após ResolveForWrite retornar sucesso (vem de 3.0) e internal/transactions aprovar pelo CategoryWriteGate
- RawTransaction deve conter evidência categorial: rootCategoryId, subcategoryId, categoryVersion, categorySource="user_selected_candidate"
- Idempotência: OriginWamid = messageId da operação original; OriginItemSeq quando disponível; OriginOperation = "pending_entry_register"
- Replay: mesmo OriginWamid → detectar via IdempotentWrite → não duplicar transação (G7-09, CA-07)
- Erro de ledger → resposta sem sucesso → pendência pode ser mantida se erro transitório (G7-15, CA-06)
- Sucesso confirmado apenas após retorno real de CreateTransaction/UpdateTransaction/CreateRecurringTemplate com ID ou evidência de recurso criado (RF-21)
- Confirmação universal (RF-38): TODA escrita (register/edit/recurrence) exige AwaitingSlotConfirmation → "sim" antes de persistir; nenhuma tool escreve síncrono; M-07=0
- Edição (RF-43): PendingEntryState carrega OperationKind=PendingOpEditEntry, TargetTransactionID e TargetVersion resolvidos server-side; a escrita chama TransactionsLedger.UpdateTransaction respeitando TargetVersion; nunca cria nova transação
- Recorrência (RF-25, G9-01): CreateRecurringTemplate exige confirmação explícita do usuário antes de escrever (AwaitingSlot=Confirmation → "sim"); cancelamento antes da confirmação fecha pendência sem escrita (G9-02); o binding delega a `internal/transactions` create_recurring_template.go — nunca reimplementar o template no consumidor
- Zero comentários Go de produção
- Zero SQL direto em tools ou workflows (R-ADAPTER-001.2)
</requirements>

## Subtarefas

- [ ] 4.1 Implementar step `write_transaction` no workflow: monta `RawTransaction` com todos os campos de `PendingEntryState` + evidência categorial; chama `TransactionsLedger.CreateTransaction`; fecha pendência com status=`Completed` e `ResponseText` de confirmação
- [ ] 4.2 Implementar idempotência: antes de chamar `CreateTransaction`, verificar replay via `OriginWamid` + `OriginOperation`; retornar resultado de replay sem segunda escrita
- [ ] 4.3 Implementar tratamento de erro de ledger: erro retorna `PendingEntryResult` sem sucesso; `ResponseText` não pode conter "registrei", "anotei", "salvo"; pendência pode ser mantida para retry quando erro for transitório
- [ ] 4.4 Implementar fluxo de edição: quando `PendingOperation=PendingOpEditEntry`, montar update a partir de `TargetTransactionID`+`TargetVersion` do snapshot; após `AwaitingSlotConfirmation → "sim"`, chamar `TransactionsLedger.UpdateTransaction` respeitando `TargetVersion`; nunca criar nova transação (RF-43)
- [ ] 4.5 Implementar fluxo de recorrência: quando `PendingOperation=PendingOpCreateRecurrence`, step `confirm_recurrence` abre `AwaitingSlot=Confirmation`; "sim" explícito → `CreateRecurringTemplate` (binding delega a `internal/transactions` create_recurring_template.go); "não"/cancelamento → `status=Cancelled` sem escrita (G9-01, G9-02)
- [ ] 4.6 Verificar que `CategoryWriteGate` em `internal/transactions` bloqueia escrita sem evidência categorial válida (defesa final — não duplicar lógica, apenas garantir que o gate é acionado)
- [ ] 4.7 Testes com double de `TransactionsLedger`: escrita bem-sucedida, replay idempotente, erro de ledger sem sucesso simulado, edição via UpdateTransaction respeitando TargetVersion, recorrência com confirmação, recorrência cancelada

## Detalhes de Implementação

Ver `techspec.md` seções **"Retomada de Pendência"** (passos 6–8), **"Idempotência"** e ADR-001 em `.specs/prd-conversa-agentiva-fluida/`.

Campos obrigatórios em `RawTransaction` para evidência categorial:

```
RootCategoryID    uuid.UUID  (de PendingEntryState.Candidates[chosen].RootCategoryID)
SubcategoryID     uuid.UUID  (de PendingEntryState.Candidates[chosen].SubcategoryID)
CategoryVersion   int64      (de PendingEntryState.CategoryVersion preenchido por ResolveForWrite)
CategorySource    string     = "user_selected_candidate"
```

Campos de idempotência:

```
OriginWamid      = PendingEntryState.MessageId (da operação original, não da resposta de categoria)
OriginItemSeq    = int do contexto de inbound quando disponível
OriginOperation  = "pending_entry_register"
```

Confirmação de sucesso (RF-21): `ResponseText` só pode conter afirmação de registro quando `CreateTransaction`/`UpdateTransaction`/`CreateRecurringTemplate` retornar `transactionID != uuid.Nil`/`EntryRef` válido e `error == nil`.

Confirmação universal (RF-38): TODA escrita — register, edit e recurrence — passa por `AwaitingSlotConfirmation`. Resume com `{"resumeText":"sim"}` executa a escrita; resume com `{"resumeText":"não"}` ou "cancela" fecha sem escrita; texto ambíguo → reprompt único (`ConfirmRepromptCount` 0→1), 2ª ambiguidade → cancela. Nenhuma tool escreve síncrono; a confirmação não é específica de recorrência.

Edição (RF-43): `PendingOperation=PendingOpEditEntry` com `TargetTransactionID`/`TargetVersion` resolvidos server-side (tool de edição em 5.0). Após confirmação, `UpdateTransaction(ctx, RawTransaction, TargetVersion)` atualiza a transação existente respeitando a versão; nunca cria nova transação.

Recorrência: `AwaitingSlot=Confirmation` com `PendingOperation=PendingOpCreateRecurrence`. Após confirmação, `TransactionsLedger.CreateRecurringTemplate(ctx, RawRecurringTemplate)` retorna `(EntryRef, error)`; o binding adapter delega a `internal/transactions` create_recurring_template.go, nunca reimplementa o template no consumidor.

## Critérios de Sucesso

- `go build ./internal/agents/...` passa
- `go test -race -count=1 ./internal/agents/...` verde incluindo 4.7
- Escrita bem-sucedida: `TransactionsLedger.CreateTransaction` chamado 1x; resposta contém afirmação de sucesso com valor+categoria (G7-20)
- Replay idempotente: segundo resume com mesmo OriginWamid não chama `CreateTransaction` novamente (G7-09, CA-07)
- Erro de ledger: resposta não contém "registrei"/"anotei"/"salvo"; `M-03=0` (G7-15, CA-06)
- Raiz sem folha: `CategoryWriteGate` em internal/transactions bloqueia; zero escrita (G10-01, M-04=0)
- Edição confirmada: `UpdateTransaction` chamado com `TargetVersion` preservado; nenhuma nova transação criada (CA-17, RF-43)
- Recorrência confirmada: `CreateRecurringTemplate` chamado com frequency, amount, subcategoryId (G9-01, CA-16)
- Recorrência cancelada: zero write (G9-02)
- Confirmação universal: nenhuma escrita sem AwaitingSlotConfirmation → "sim" prévio (M-07=0, RF-38)

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — step de escrita real é parte do workflow pending-entry do consumidor internal/agents; idempotência e CategoryWriteGate são contratos do substrato de escrita financeira

## Testes da Tarefa

- [ ] `transactions_ledger_pending_test.go`: double de TransactionsLedger — write ok, replay sem duplicidade, erro sem sucesso, edição via UpdateTransaction (TargetVersion preservado), recorrência confirmada, recorrência cancelada
- [ ] Cenários G7-09 (replay), G7-15 (erro ledger), G9-01 (recorrência confirmada), G9-02 (recorrência cancelada), G10-01 (raiz sem folha bloqueada), G10-02 (ID LLM inválido rejeitado), G10-03 (sucesso simulado proibido), CA-16 (recorrência via CreateRecurringTemplate), CA-17 (edição via UpdateTransaction)
- [ ] Verificar gate M-03: grep por resposta de sucesso sem evidência de write real deve retornar vazio

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/agents/application/workflows/pending_entry_workflow.go` (de 2.0 — adicionar steps write_transaction e confirm_recurrence)
- `internal/agents/application/interfaces/types.go` (RawTransaction com campos de evidência categorial)
- `internal/agents/infrastructure/binding/transactions_ledger_adapter.go` (CreateRecurringTemplate delega a create_recurring_template.go; UpdateTransaction para edição)
- `internal/transactions/application/usecases/create_recurring_template.go`
- `internal/transactions/application/interfaces/category_write_gate.go`
- `internal/transactions/domain/valueobjects/category_write_evidence.go`
- `.specs/prd-conversa-agentiva-fluida/techspec.md` (seções "Retomada", "Idempotência")
- `.specs/prd-conversa-agentiva-fluida/scenarios.md` (G7-09, G7-15, G9-01, G9-02, G10-01..G10-03)
