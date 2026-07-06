# Tarefa 5.0: Register Tools — `outcome=clarify`, abertura de pendência e CardManager

<critical>Ler prd.md, techspec.md e scenarios.md desta pasta — tarefa invalidada se pulado</critical>

## Visão Geral

Adaptar `register_expense.go` e `register_income.go` para NUNCA escrever síncrono: toda tentativa (registrar despesa/renda, editar, recorrência) abre a pendência `pending-entry` e retorna um outcome de "pendência aberta" (clarify OU confirmation). Lançamentos totalmente especificados e não ambíguos abrem direto em `AwaitingSlotConfirmation` (RF-38, RF-40). Implementar o use case `RegisterAttempt` que orquestra a tentativa: quando houver clarify, abre a pendência `pending-entry` **antes** de devolver a resposta à tool; quando não houver clarify, abre a pendência já em `AwaitingSlotConfirmation`. Adicionar a tool de edição que resolve `TargetTransactionID`/`TargetVersion` server-side e abre pendência com `PendingOperation=PendingOpEditEntry` (RF-43). Integrar `CardManager` existente para resolver cartão via apelido quando `AwaitingSlot=Card`. Usar `InboundExecutionFromContext(ctx)` (de 2.0) para obter `threadID` e montar a key `<resourceID>:<threadID>:pending-entry` sem depender do LLM. Incorpora o `AwaitingSlotConfirmation` obrigatório antes de toda escrita (RF-38, spec-version 3 do scenarios.md).

<requirements>
- register_expense.go e register_income.go: adapters finos; delegam para RegisterAttempt; NUNCA escrevem síncrono; sempre abrem pendência durável e retornam outcome de pendência (clarify OU confirmation)
- RegisterAttempt: quando RegisterEntry.classify retorna ToolOutcomeClarify, chama workflow.Engine.Start ANTES de retornar clarify; quando classify é seguro/completo, abre pendência direto em AwaitingSlotConfirmation; se Start falhar, retorna erro sem afirmar sucesso
- Tool de edição: resolve TargetTransactionID/TargetVersion server-side, abre pendência com PendingOperation=PendingOpEditEntry; write path (4.0) chama UpdateTransaction respeitando TargetVersion; nunca cria nova transação (RF-43)
- Confirmação obrigatória (RF-38, spec-version 3): toda escrita financeira (registrar/editar/recorrência) exige AwaitingSlotConfirmation antes de persistir; turno de confirmação deve ser o último estado antes de write_transaction (de 4.0); M-07=0
- CardManager: integrar via interface existente; quando CC sem cartão identificado, abrir AwaitingSlot=Card; resumeText com apelido → CardManager.ResolveByNickname → cardId no snapshot
- identity_context.go: InboundExecutionFromContext(ctx) retornando resourceID, threadID, messageID, itemSeq — usado por tools de escrita para correlação server-side sem depender do LLM
- Zero regra de negócio financeira nas tools (R-ADAPTER-001.2)
- Zero SQL direto em tools (R-ADAPTER-001.2)
- Zero branching de domínio em tools
- Zero comentários Go de produção
</requirements>

## Subtarefas

- [ ] 5.1 Usar `InboundExecutionFromContext(ctx)` (de 2.0, em `internal/platform/agent/identity_context.go`) nas tools de escrita para obter `resourceID, threadID, messageID, itemSeq` e montar a key de correlação server-side sem depender do LLM
- [ ] 5.2 Implementar `RegisterAttempt` use case em `internal/agents/application/usecases/register_attempt.go`: orquestra tentativa de registro; quando `classify` retorna `clarify`, chama `Engine.Start("resourceID:threadID:pending-entry", initialState)` abrindo em `AwaitingSlot` do slot faltante; quando `classify` é seguro/completo, abre pendência direto em `AwaitingSlotConfirmation`; nunca escreve síncrono; se `Start` falha, retorna erro
- [ ] 5.3 Atualizar `register_expense.go`: delegar a `RegisterAttempt`; retornar outcome de pendência (clarify OU confirmation) somente após pendência aberta com sucesso; nunca escrever síncrono
- [ ] 5.4 Atualizar `register_income.go`: mesmo padrão de 5.3
- [ ] 5.5 Implementar tool de edição (`edit_entry.go`): resolve `TargetTransactionID`/`TargetVersion` server-side, abre pendência com `PendingOperation=PendingOpEditEntry` e `AwaitingSlotConfirmation`; write path (4.0) chama `UpdateTransaction` respeitando `TargetVersion`; nunca cria nova transação (RF-43)
- [ ] 5.6 Implementar step `await_card` no workflow (em pending_entry_workflow.go de 2.0): quando `AwaitingSlot=Card`, chamar `CardManager.ResolveByNickname(resumeText)` via interface; armazenar `cardId` no snapshot; erro se cartão não resolvido
- [ ] 5.7 Implementar step `await_confirmation` no workflow: apresentar resumo (`valor, categoria raiz > folha, data, pagamento`) para confirmação; aceite explícito ("sim"/"confirmar"/"ok"/"pode") → prossegue para write_transaction; cancelamento no turno de confirmação → status=Cancelled; resposta ambígua → ConfirmRepromptCount 0→1; segundo ambíguo → status=Cancelled sem escrita (M-07=0, RF-38)
- [ ] 5.8 Testes unitários: RegisterAttempt com clarify → Engine.Start chamado; classify seguro → pendência abre em AwaitingSlotConfirmation; Engine.Start falhando → retorna erro; tool de register/edit não afirma sucesso sem write real

## Detalhes de Implementação

Ver `techspec.md` seções **"Abertura de Pendência"**, **"Interfaces Chave"** e `scenarios.md` **"Convenção Global de Confirmação (spec-version 3)"**.

Fluxo completo com confirmação (RF-38):

```
user: frase de lançamento
  → RegisterAttempt.classify → ToolOutcomeClarify
  → Engine.Start("resourceID:threadID:pending-entry", state)
  → Agent responde com pergunta de clarificação (category/card/payment/date)

user: resposta de slot
  → Engine.Resume(merge-patch{"resumeText":"...","messageId":"..."})
  → step resolve slot → SearchDictionary → ResolveForWrite
  → AwaitingSlot=confirmation
  → Agent: "Confirma? R$ X em <Raiz > Folha> no <pagamento>?"

user: "sim"
  → Engine.Resume(merge-patch{"resumeText":"sim","messageId":"..."})
  → step await_confirmation → aceite explícito → write_transaction (4.0)
  → status=Completed
```

Cancelamento no turno de confirmação (G7-04 equiv.): "não"/"cancela" → status=Cancelled; zero escrita.

Reprompt de confirmação: texto ambíguo → ConfirmRepromptCount=1; segundo ambíguo → status=Cancelled.

CartManager: interface já existente em `internal/agents/application/interfaces/`; tarefa apenas integra o passo `await_card` que chama `ResolveByNickname`. Zero implementação de adapter novo.

## Critérios de Sucesso

- `go build ./internal/agents/...` passa após 5.1..5.8
- `go test -race -count=1 ./internal/agents/application/...` verde
- RegisterAttempt com classify=clarify → Engine.Start chamado antes de retornar (G7-20 parcial)
- RegisterAttempt com classify seguro/completo → pendência abre direto em AwaitingSlotConfirmation; zero escrita síncrona (CA-13, RF-38)
- Tool de edição → resolve TargetTransactionID/TargetVersion server-side; abre pendência PendingOpEditEntry (CA-17, RF-43)
- Engine.Start falhando → tool retorna erro; zero sucesso afirmado
- AwaitingSlot=Card → CardManager.ResolveByNickname chamado; cardId no snapshot (G7-16, CA-10)
- AwaitingSlot=Confirmation → "sim" → write; "não" → zero write; ambíguo 2x → zero write (RF-38, M-07=0)
- Gate R-ADAPTER-001.2: `grep -rn "QueryContext\|ExecContext\|db\.Query" internal/agents/application/tools/` retorna vazio

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — register tools são adapters finos do consumidor internal/agents; abertura de pendência via Engine.Start é primitivo do substrato agent da plataforma; step await_card e await_confirmation são parte do workflow pending-entry

## Testes da Tarefa

- [ ] `register_attempt_test.go`: classify=clarify → Engine.Start em slot faltante; classify=success → Engine.Start direto em AwaitingSlotConfirmation (nunca escrita síncrona); Engine.Start erro → retorna erro
- [ ] `edit_entry_test.go`: resolve TargetTransactionID/TargetVersion server-side; abre pendência PendingOpEditEntry em AwaitingSlotConfirmation (CA-17)
- [ ] `register_expense_test.go` e `register_income_test.go`: tool retorna clarify apenas após Engine.Start ok; tool retorna erro quando Start falha
- [ ] Cenários G7-16 (cartão CC sem cartão → AwaitingSlot=Card → resolve → confirma → write), G7-17 (parcelas), G8-01..G8-03 (CC parcelado com confirmação)
- [ ] Cenário de confirmação: G7-20 com turno de confirmação inserido (AwaitingSlot=confirmation → "sim" → write)
- [ ] Cenário de cancelamento na confirmação: equivalente G7-04 no turno de confirmação

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/platform/agent/identity_context.go` (consumido — InboundExecutionFromContext implementado em 2.0)
- `internal/agents/application/usecases/register_attempt.go` (novo)
- `internal/agents/application/usecases/register_entry.go` (atualizar para retornar payload de clarify)
- `internal/agents/application/tools/register_expense.go` (atualizar)
- `internal/agents/application/tools/register_income.go` (atualizar)
- `internal/agents/application/tools/edit_entry.go` (novo — resolve TargetTransactionID/TargetVersion server-side)
- `internal/agents/application/workflows/pending_entry_workflow.go` (de 2.0 — adicionar steps await_card e await_confirmation)
- `internal/agents/application/interfaces/` (CardManager interface existente)
- `.specs/prd-conversa-agentiva-fluida/techspec.md` (seções "Abertura de Pendência", "Interfaces Chave")
- `.specs/prd-conversa-agentiva-fluida/scenarios.md` (Convenção Global de Confirmação, G7-16, G7-17, G8-01..G8-03)
