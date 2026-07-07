# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Integração da idempotência de escrita em `executeWrite` com chave ancorada no wamid original
- **Data:** 2026-07-07
- **Status:** Aceita
- **Decisores:** time de plataforma / agentes
- **Relacionados:** `prd.md` (RF-19, RF-20; Decisão D-04), `techspec.md`, `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001), `.claude/rules/go-adapters.md` (R-ADAPTER-001)

## Contexto

O `pending-entry workflow` executa a escrita financeira em `executeWrite → callLedger`, chamando diretamente `ledger.CreateTransaction`/`UpdateTransaction`/`CreateRecurringTemplate`. O use case `IdempotentWrite` e o `WriteLedgerRepository` (tabela `mecontrola.agents_write_ledger`, unique `(wamid, item_seq, operation)`) existem, mas em produção **só são consumidos pelo job de retenção `PurgeLedger`** — a dedup durável de escrita nunca é acionada no caminho produtivo (`IdempotentWrite` só é instanciado em testes).

Já existe uma segunda dedup, de **nível de mensagem**, via `ProcessedMessageID`/`IncomingMessageID` no `PendingEntryState` (coberta por `TestG7_09_ReplayIdempotente`). Essa dedup vive no snapshot do workflow e some quando o snapshot é purgado após conclusão; ela protege contra reprocessamento da mesma mensagem enquanto o run existe, mas **não é durável** e não sobrevive à limpeza do run nem a redelivery após conclusão.

Restrições que moldam a decisão:

1. **Wamid da confirmação ≠ wamid do lançamento.** A resolução via `agent.InboundIdentityFromContext(ctx)` no adapter `transactionsLedgerAdapter` retorna, no momento do resume, o wamid da mensagem de **confirmação** ("sim"), não o da mensagem **original** do lançamento. A Decisão D-04 exige ancorar a chave no wamid **original**, que está persistido em `PendingEntryState.MessageID`. Logo, a dedup precisa ser montada onde `state` está disponível — dentro de `executeWrite` — e não no adapter outbound.
2. **Ciclo de import.** `internal/agents/application/usecases` importa `internal/agents/application/workflows` (via `RegisterAttempt`, `PendingEntryContinuer`). `IdempotentWrite`, `WriteLedgerRepository` e `WriteLedgerEntry` vivem em `usecases`. Portanto `workflows` **não pode** importar `usecases` sem criar ciclo.
3. **`PendingEntryState` não carrega `ItemSeq`.** A chave `(wamid, itemSeq, operation)` exige o `itemSeq`, hoje ausente do estado (só existe em `RegisterExpenseCommand.ItemSeq`).

## Decisão

Integrar `IdempotentWrite` **dentro de `executeWrite`**, envolvendo a chamada a `callLedger`, com a chave `(state.MessageID, state.ItemSeq, state.OperationKind.String())` — wamid **original**, conforme D-04.

Para respeitar o layering sem ciclo, aplicar **porta no consumidor** (R6 go-implementation):

1. Declarar, no pacote `workflows`, uma interface de porta e um tipo de função de escrita que retornam **apenas primitivos** (sem tipos de `usecases`):

   ```go
   package workflows

   type IdempotentWriteFn func(ctx context.Context) (resourceID uuid.UUID, reconciled bool, err error)

   type IdempotentWriter interface {
       Execute(
           ctx context.Context,
           userID uuid.UUID,
           wamid string,
           itemSeq int,
           operation string,
           resourceKind string,
           write IdempotentWriteFn,
       ) (resourceID uuid.UUID, outcome agent.ToolOutcome, err error)
   }
   ```

2. `BuildPendingEntryWorkflow` passa a receber `idem IdempotentWriter` como novo parâmetro; `executeWrite` chama `idem.Execute(...)` com uma `IdempotentWriteFn` que encapsula `callLedger`.

3. Um **adapter fino** no pacote `module` (que já importa `usecases`, `workflows` e `agent`) adapta o concreto `*usecases.IdempotentWrite` (cujo `Execute` retorna `(IdempotentWriteResult, error)`) para a porta `workflows.IdempotentWriter` (retornando `(uuid.UUID, agent.ToolOutcome, error)`). Sem lógica de negócio no adapter — só desestrutura o result.

4. Adicionar `ItemSeq int` a `PendingEntryState`, populado por `RegisterAttempt` a partir do comando. Para o MVP de 1 transação por mensagem (RF-16), `itemSeq` é sempre `0`, mas o campo torna a chave completa e à prova de evolução.

5. Mapear `resourceKind` de forma determinística a partir de `OperationKind`: `transaction` para expense/income/edit, `recurring_template` para recorrência.

A dedup durável de write-ledger **compõe** com a dedup de mensagem existente: a de mensagem evita reprocessar o mesmo turno enquanto o run vive; a de write-ledger garante **no máximo uma escrita** por `(wamid_original, itemSeq, operation)` de forma durável, sobrevivendo a restart e a redelivery pós-conclusão. Em replay, `executeWrite` completa o run com o mesmo texto de sucesso (renderizado de `state`) e outcome `ToolOutcomeReplay`, sem segundo INSERT.

## Alternativas Consideradas

- **Dedup no adapter outbound (`transactionsLedgerAdapter`).** Envolveria `createTx.Execute` em `IdempotentWrite` usando `InboundIdentityFromContext`. **Rejeitada:** no resume, esse contexto expõe o wamid da confirmação, não o original (viola D-04); e mistura idempotência num adapter que deve permanecer fino (R-ADAPTER-001.2).
- **Mover a escrita para fora do workflow (num use case wrapper de `engine.Resume`).** **Rejeitada:** a escrita é durável e ocorre no passo do kernel; retirá-la quebraria a semântica suspend/resume e a atomicidade do run (R-AGENT-WF-001).
- **Colocar `IdempotentWrite`/`WriteLedger` no pacote `workflows` para eliminar o ciclo.** **Rejeitada:** `IdempotentWrite` é um use case de aplicação e é consumido por `RegisterAttempt`; movê-lo inverteria responsabilidades e ainda exigiria o repositório em `usecases`.
- **Interface de porta retornando `usecases.IdempotentWriteResult`.** **Rejeitada:** reintroduz o import de `usecases` em `workflows` (ciclo). A porta retorna primitivos + `agent.ToolOutcome` (pacote de plataforma, sem ciclo).

## Consequências

### Benefícios Esperados

- Zero duplicidade durável por `(wamid_original, itemSeq, operation)` (M-02 = 0), sobrevivendo a restart e redelivery.
- Ponto de escrita único e auditável; `ToolOutcomeReplay` já suportado pela superfície de saída das tools.
- Layering preservado sem ciclo de import; `workflows` permanece sem conhecer `usecases`.

### Trade-offs e Custos

- `BuildPendingEntryWorkflow` ganha um parâmetro; call sites (module.go e o harness de teste) precisam passar a porta (fake no teste).
- Um adapter fino adicional no `module` para desestruturar o result.
- Novo campo `ItemSeq` no estado (serializado no snapshot) — mudança aditiva, compatível com snapshots antigos (zero-value `0`).

### Riscos e Mitigações

- **Risco:** divergência entre a dedup de mensagem e a de write-ledger causar comportamento confuso. **Mitigação:** teste explícito cobrindo (a) replay de mensagem e (b) replay de write-ledger por wamid original, documentando que são camadas complementares.
- **Risco:** `OperationKind.String()` inexistente ou não determinístico. **Mitigação:** garantir `String()`/`IsValid()`/`ParseOperationKind` no tipo fechado (state-as-type) com teste de ida e volta.
- **Rollback:** remover o parâmetro `idem` e a chamada em `executeWrite` reverte ao comportamento atual sem tocar schema (a tabela já existe e continua sendo purgada).

## Plano de Implementação

1. Adicionar `ItemSeq int` a `PendingEntryState`; popular em `RegisterAttempt.*`.
2. Garantir `OperationKind.String()/IsValid()/ParseOperationKind` (state-as-type).
3. Declarar `IdempotentWriter` + `IdempotentWriteFn` em `workflows`.
4. Alterar `BuildPendingEntryWorkflow` para receber `idem IdempotentWriter`; envolver `callLedger` em `idem.Execute` dentro de `executeWrite`; mapear `resourceKind`.
5. Criar o adapter `*usecases.IdempotentWrite → workflows.IdempotentWriter` no `module`; construir `IdempotentWrite` a partir do `writeLedgerRepo` já existente (module.go:139) e injetar em `BuildPendingEntryWorkflow` (module.go:193).
6. Atualizar o harness (`newPEHarness`) para injetar a porta (fake in-memory) e adicionar teste de replay por write-ledger.

## Monitoramento e Validação

- Métrica existente do `IdempotentWrite` (counter) passa a registrar replays em produção; label de outcome enum-fechado (sem `user_id`/`wamid` como label — R-AGENT-WF-001.5 / R-TXN-004).
- Validação: teste de workflow com fake write-ledger provando 1 INSERT para 2 execuções da mesma chave; suíte real-LLM inalterada; `agents_write_ledger` sem linhas duplicadas.
- Critério de sucesso: M-02 = 0 duplicatas.

## Impacto em Documentação e Operação

- Runbook de agentes: registrar que a dedup durável de escrita agora está ativa no caminho produtivo (antes só o job de retenção usava a tabela).

## Revisão Futura

- Revisitar quando o multi-item (RF-16, `itemSeq > 0`) sair de non-goal: a chave já suporta, mas a geração de `itemSeq` por item precisará de contrato próprio.
