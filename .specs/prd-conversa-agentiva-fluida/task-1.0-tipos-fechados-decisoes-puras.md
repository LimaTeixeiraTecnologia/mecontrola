# Tarefa 1.0: Tipos Fechados e Decisões Puras de Pendência

<critical>Ler prd.md, techspec.md e scenarios.md desta pasta — tarefa invalidada se pulado</critical>

## Visão Geral

Implementar todos os tipos fechados do estado conversacional pendente e as funções de decisão puras no consumidor `internal/agents/application/workflows/`. Nenhum IO, nenhum LLM, nenhum `context.Context` nas funções de decisão. Esses tipos são a base compilável de todas as tarefas downstream.

<requirements>
- PendingStatus, AwaitingSlot, PendingOperationKind como tipos fechados com iota+1, String(), IsValid(), Parse* e erro sentinel
- PendingOperationKind com 4 constantes: PendingOpRegisterExpense, PendingOpRegisterIncome, PendingOpEditEntry, PendingOpCreateRecurrence (RF-41)
- AwaitingSlotConfirmation é o slot terminal obrigatório antes de TODA escrita financeira (RF-38, RF-39)
- PendingEntryState como snapshot único com todos os campos especificados na techspec, incluindo OperationKind, TargetTransactionID (*uuid.UUID), TargetVersion (int64), ConfirmRepromptCount (int), Frequency, RecurrenceDayOfMonth
- PendingCategoryCandidate com rootCategoryId, rootSlug, subcategoryId, subcategorySlug, path, matchedTerm, score, confidence, matchQuality, matchReason
- DecidePendingResume(state, msg, now) PendingDecision pura — sem IO
- DecideConfirmation(state, msg, now) PendingDecision pura — determinística, sem IO/LLM; sim/não, reprompt único via ConfirmRepromptCount, TTL 30min, idempotência por wamid original (RF-39)
- DecideCategoryChoice(state, candidates, text) CategoryChoiceDecision pura — sem IO; aceita índice numérico OU nome da categoria (RF-42), ambos resolvem o mesmo par raiz+folha canônico
- DecideNewOperationReplacement(state, msg) PendingDecision pura — regra conservadora: valor monetário + verbo de lançamento
- Zero comentários em todos os arquivos .go (R-ADAPTER-001.1)
- Zero var _ Interface = (*Type)(nil) (regra do repositório)
- Zero init()
</requirements>

## Subtarefas

- [ ] 1.1 Criar `pending_entry_state.go` com `PendingStatus` (5 constantes: Active, Completed, Cancelled, Expired, Replaced)
- [ ] 1.2 Criar `pending_entry_state.go` com `AwaitingSlot` (6 constantes: Category, PaymentMethod, Card, Date, Confirmation, Correction) — `AwaitingSlotConfirmation` é o estado terminal obrigatório antes de qualquer write
- [ ] 1.3 Criar `pending_entry_state.go` com `PendingOperationKind` (enum: PendingOpRegisterExpense, PendingOpRegisterIncome, PendingOpEditEntry, PendingOpCreateRecurrence)
- [ ] 1.4 Criar `pending_entry_state.go` com `PendingEntryState` struct completo (todos os campos da techspec: OperationKind, TargetTransactionID, TargetVersion, ConfirmRepromptCount, Frequency, RecurrenceDayOfMonth)
- [ ] 1.5 Criar `pending_category_candidate.go` com `PendingCategoryCandidate` (rootCategoryId, rootSlug, subcategoryId, subcategorySlug, path, matchedTerm, score, confidence, matchQuality, matchReason)
- [ ] 1.6 Criar `pending_entry_decisions.go` com `DecidePendingResume` — handles: expired (TTL 30min), cancelled (cancela/deixa pra lá/não registra), replaced (nova frase completa), slot preenchido, reprompt (RepromptCount 0→1), cancelamento após 2 reprompts
- [ ] 1.7 Criar `pending_entry_decisions.go` com `DecideConfirmation` — handles: sim/confirmar/ok/pode (confirma → prossegue para write), não/cancela (cancela sem escrita), ambíguo 1ª vez (ConfirmRepromptCount 0→1), ambíguo 2ª vez (cancela), TTL expirado, replay por wamid original (RF-39)
- [ ] 1.8 Criar `pending_entry_decisions.go` com `DecideCategoryChoice` — handles: raiz sem folha (bloqueia), candidato único (aceita), múltiplos candidatos (lista), texto incompatível (reprompt); aceita índice numérico OU nome (RF-42) resolvendo o mesmo par raiz+folha
- [ ] 1.9 Criar `pending_entry_decisions.go` com `DecideNewOperationReplacement` — regra: contém valor monetário (R\$) + verbo (gastei/paguei/comprei/recebi/ganhei) = nova operação
- [ ] 1.10 Testes unitários table-driven para todos os tipos (String/Parse/IsValid round-trip) e para todas as decisões cobrindo cenários G7-01..G7-14 do scenarios.md

## Detalhes de Implementação

Ver `techspec.md` seção **"Estados Fechados"** e **"Decisão Determinística"**.

Tipos obrigatórios e valores aceitos:

```
PendingStatus:       Active=1, Completed=2, Cancelled=3, Expired=4, Replaced=5
AwaitingSlot:        Category=1, PaymentMethod=2, Card=3, Date=4, Confirmation=5, Correction=6
PendingOperationKind: PendingOpRegisterExpense=1, PendingOpRegisterIncome=2, PendingOpEditEntry=3, PendingOpCreateRecurrence=4
```

`DecidePendingResume` recebe `now time.Time` — nunca chamar `time.Now()` internamente.

Cancelamento inequívoco (RF-08): `cancela`, `deixa pra lá`, `não registra` → `PendingDecision{Action: Cancel}`.

Nova operação completa (RF-31): presença de valor monetário (`R\$` seguido de dígitos) e pelo menos um dos verbos `gastei|paguei|comprei|recebi|ganhei` → `PendingDecision{Action: Replace}`.

`RepromptCount` máximo = 1 antes de cancelar automaticamente (G7-14).

`DecideConfirmation(state, msg, now)` (RF-39) é o gate universal antes de toda escrita: `AwaitingSlotConfirmation` é o estado terminal. Confirmação explícita (`sim`/`confirmar`/`ok`/`pode`) → prossegue para write; cancelamento (`não`/`cancela`) → sem escrita; ambíguo 1ª vez → reprompt (`ConfirmRepromptCount` 0→1); ambíguo 2ª vez → cancela; TTL 30min expirado → cancela; replay do wamid original → idempotente. Reusa o contrato SEMÂNTICO do destructive-confirm (não o workflow).

## Critérios de Sucesso

- `go build ./internal/agents/application/workflows/...` passa sem erros
- `go vet ./internal/agents/application/workflows/...` passa sem erros
- `go test -race -count=1 ./internal/agents/application/workflows/...` verde
- Todos os tipos têm round-trip `String() → Parse*()` sem perda
- `DecidePendingResume` com `SuspendedAt` há 31 min retorna `Action=Expire` (G7-08)
- `DecideNewOperationReplacement` com "Gastei R$ 150,00 na farmácia hoje, no pix" retorna `Action=Replace` (G7-01)
- `DecideCategoryChoice` com raiz sem folha retorna bloqueio (G7-03)
- Gate zero comentários: `grep -rn "^[[:space:]]*//" internal/agents/application/workflows/ | grep -Ev "(//go:|//nolint:|// Code generated)"` retorna vazio

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — tipos fechados de PendingStatus/AwaitingSlot/PendingOperationKind são primitivos do substrato agent da plataforma; decisões puras implementam o contrato DMMF Decide* do consumidor internal/agents

## Testes da Tarefa

- [ ] `pending_entry_state_test.go`: String/Parse/IsValid para todos os enums — round-trip sem perda, valores inválidos retornam erro sentinel
- [ ] `pending_entry_decisions_test.go`: table-driven com cenários G7-01 (replace), G7-03 (raiz sem folha), G7-04 (cancela), G7-05 (deixa pra lá), G7-06 (não registra), G7-07 (sim e pix não é categoria), G7-08 (expired 31min), G7-13 (incompatível → reprompt), G7-14 (2º reprompt → cancelamento automático); `DecideConfirmation`: sim → prossegue, não → cancela, ambíguo 2x → cancela (CA-13, CA-14); `DecideCategoryChoice` por índice numérico E por nome (CA-15)
- [ ] Verificar que nenhuma função de decisão importa `context`, `database`, `llm`, `http` ou qualquer pacote de IO

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/agents/application/workflows/pending_entry_state.go` (novo)
- `internal/agents/application/workflows/pending_category_candidate.go` (novo)
- `internal/agents/application/workflows/pending_entry_decisions.go` (novo)
- `internal/agents/application/workflows/pending_entry_state_test.go` (novo)
- `internal/agents/application/workflows/pending_entry_decisions_test.go` (novo)
- `internal/agents/application/workflows/confirm_state.go` (referência de padrão existente)
- `.specs/prd-conversa-agentiva-fluida/techspec.md` (seções "Estados Fechados" e "Decisão Determinística")
- `.specs/prd-conversa-agentiva-fluida/scenarios.md` (G7-01..G7-14 como casos de teste)
