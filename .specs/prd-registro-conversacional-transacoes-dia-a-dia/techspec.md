<!-- spec-hash-prd: cc934387c2bb366933c786ce190002c5e4aa2764a77a8c68b7251f02044d9242 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Registro Conversacional de Transações do Dia a Dia

## Resumo Executivo

Esta especificação evolui o fluxo de registro conversacional já existente em `internal/agents` para fechar os gaps reais confirmados no código, sem recriar o que já existe. As mudanças são **cirúrgicas e aditivas**, concentradas em quatro superfícies do consumidor agentivo `mecontrola`, consumindo o substrato `internal/platform/{agent,tool,workflow}` (regra de ouro da skill `mastra`: consumir, não recriar) e respeitando DMMF (pure core / IO shell, state-as-type, smart constructors) e R-ADAPTER-001/R-AGENT-WF-001.

Quatro entregas:

1. **Idempotência durável de escrita (gap crítico):** integrar `IdempotentWrite` em `executeWrite` do `pending-entry workflow`, com chave `(wamid_original, itemSeq, operation)`, via porta consumidor-side para evitar ciclo de import (ADR-001).
2. **Datas por dia da semana:** estender o parser puro `parseInputDate` com `parseWeekday`, mantendo pureza e o sentinel de "pedir data" (ADR-002).
3. **Teto de valor:** guarda pura na borda do agente, sem alterar o VO `Money` (ADR-003).
4. **Endurecimento de instruções + correção de mapa de pagamento:** reforçar as `const` de instrução do agente (cinco campos, dias da semana, fronteira multi-item) e **corrigir um bug latente** no `knownPaymentMethods` (valores que não são códigos válidos de `PaymentMethod`).

Nenhuma migração de schema é necessária: a tabela `mecontrola.agents_write_ledger` (unique `(wamid, item_seq, operation)`) já existe. Os campos `occurredAt` já existem em schemas, entidade e comandos — não recriar.

## Arquitetura do Sistema

### Visão Geral dos Componentes

Componentes **modificados** (nenhum módulo novo):

| Componente | Arquivo | Mudança |
|---|---|---|
| `PendingEntryState` | `internal/agents/application/workflows/pending_entry_state.go` | **+campo** `ItemSeq int`; garantir `PendingOperationKind.String()/IsValid()/ParseOperationKind` (state-as-type) |
| `BuildPendingEntryWorkflow` / `executeWrite` / `callLedger` | `internal/agents/application/workflows/pending_entry_workflow.go` | **+param** `idem IdempotentWriter`; envolver `callLedger` em `idem.Execute`; mapear `resourceKind` |
| Porta de idempotência | `internal/agents/application/workflows/` (novo símbolo no pacote) | **+interface** `IdempotentWriter` e tipo `IdempotentWriteFn` (retornam primitivos, sem `usecases`) |
| `parseInputDate` / `parseWeekday` | `internal/agents/application/workflows/pending_entry_decisions.go` | **+função pura** `parseWeekday`; chamada dentro de `parseInputDate` |
| `knownPaymentMethods` | `internal/agents/application/workflows/pending_entry_decisions.go` | **correção** dos valores para códigos válidos de `PaymentMethod` (bug latente) |
| Guarda de valor | `internal/agents/application/tools/` (register_expense/income) | **+função pura** `validateEntryAmount` + `maxEntryAmountCents`; chamada no exec |
| Instruções do agente | `internal/agents/application/agents/mecontrola_agent.go` | **+texto** na `const mecontrolaAgentInstructions` (cinco campos, dias da semana, multi-item, mapeamento de pagamento) |
| Adapter de idempotência + wiring | `internal/agents/module.go` (e binding se preferível) | **+adapter** `*usecases.IdempotentWrite → workflows.IdempotentWriter`; construir `IdempotentWrite` do `writeLedgerRepo`; injetar em `BuildPendingEntryWorkflow` |
| `RegisterAttempt.*` | `internal/agents/application/usecases/register_attempt.go` | popular `PendingEntryState.ItemSeq` a partir do comando |

### Fluxo de dados (após mudanças)

```
Tool register_expense/income (exec)
  ├─ agent.InboundExecutionFromContext(ctx) → userID, threadID, wamid, itemSeq
  ├─ validateEntryAmount(amountCents)                       [ADR-003] guarda pura de valor
  └─ RegisterAttempt.RegisterExpense/Income(cmd)
       ├─ resolveEntryDate(cmd.OccurredAt)                  (existente; assume "hoje" SP)
       └─ engine.Start(def, key, PendingEntryState{ MessageID: wamid, ItemSeq: itemSeq, OccurredAt, OperationKind, Candidates:[cat], ... })

[usuário responde "sim" → PendingEntryContinuer.Continue → engine.Resume(merge-patch {resumeText, incomingMessageId})]
  └─ DecideConfirmation → executeWrite(ctx, state, ledger, cats)
       └─ idem.Execute(ctx, state.UserID, state.MessageID, state.ItemSeq,   [ADR-001]
                       state.OperationKind.String(), resourceKind(state),
                       write = func(c){ ref,e := callLedger(c,state,ledger); return ref.ID, ref.Reconciled, e })
            ├─ replay (FindByKey acha) → ToolOutcomeReplay, sem 2º INSERT
            └─ first → callLedger → ledger.Create/Update/CreateRecurringTemplate → Insert write-ledger

parseInputDate(text, now)  [chamado onde a data é resolvida a partir do texto de clarificação]
  ├─ hoje/ontem/anteontem            (existente)
  ├─ parseWeekday(text, now)         [ADR-002] segunda..domingo (+ "passada/passado")
  ├─ DD/MM, YYYY-MM-DD               (existente)
  └─ "" → suspende e pede data específica  (cobre "semana/mês passado", RF-08)
```

## Design de Implementação

### Interfaces Chave

Porta consumidor-side no pacote `workflows` (retorna primitivos + tipo de plataforma; **sem** importar `usecases`) — ADR-001:

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

`BuildPendingEntryWorkflow` (assinatura nova):

```go
func BuildPendingEntryWorkflow(
    ledger interfaces.TransactionsLedger,
    cards cardNicknameSolver,
    cats categoryValidator,
    idem IdempotentWriter, // novo
) workflow.Definition[PendingEntryState]
```

Adapter fino no `module` (satisfaz a porta desestruturando o result concreto — sem lógica de negócio):

```go
type idempotentWriterAdapter struct{ uc *usecases.IdempotentWrite }

func (a idempotentWriterAdapter) Execute(
    ctx context.Context, userID uuid.UUID, wamid string, itemSeq int,
    operation, resourceKind string, write workflows.IdempotentWriteFn,
) (uuid.UUID, agent.ToolOutcome, error) {
    res, err := a.uc.Execute(ctx, userID, wamid, itemSeq, operation, resourceKind,
        usecases.WriteFn(write))
    return res.ResourceID, res.Outcome, err
}
```

`usecases.WriteFn` e `workflows.IdempotentWriteFn` têm assinatura estrutural idêntica (`func(context.Context) (uuid.UUID, bool, error)`); a conversão nomeada `usecases.WriteFn(write)` é válida em Go.

### Modelos de Dados

`PendingEntryState` — campo aditivo (serializado no snapshot; zero-value `0` compatível com snapshots antigos):

```go
ItemSeq int // idempotency key component; MVP = 0 (RF-16)
```

`PendingOperationKind` — garantir contrato state-as-type (DMMF / mastra state-as-type):

```go
func (k PendingOperationKind) String() string { /* switch exaustivo → "register_expense"|"register_income"|"edit_entry"|"create_recurrence" */ }
func (k PendingOperationKind) IsValid() bool   { return k >= PendingOpRegisterExpense && k <= PendingOpCreateRecurrence }
func ParsePendingOperationKind(s string) (PendingOperationKind, error) // rejeita valor desconhecido
```

`resourceKind(state)` — mapeamento determinístico (sem branching de domínio, só tradução de enum):

```
PendingOpCreateRecurrence → "recurring_template"
default (expense/income/edit) → "transaction"
```

### Funções puras novas

`parseWeekday(text string, now time.Time) (string, bool)` — ADR-002, em `pending_entry_decisions.go`. Recebe `now` (injetado pelo chamador em `America/Sao_Paulo`), retorna data `YYYY-MM-DD` ou `("", false)`. Encaixe em `parseInputDate` antes do fallback de formato explícito.

`validateEntryAmount(cents int64) error` + `const maxEntryAmountCents int64 = 1_000_000_000` — ADR-003, na camada de tools do agente. Rejeita `cents <= 0` e `cents > maxEntryAmountCents`. Chamada no exec de `register_expense`/`register_income` antes de `RegisterAttempt`, produzindo resposta de clarificação amigável (não erro de tool).

### Correção de `knownPaymentMethods` (bug latente)

O mapa atual traduz `"boleto" → "bank_slip"` e `"ted"/"doc"/"transferencia" → "bank_transfer"`, que **não são códigos válidos** de `PaymentMethod` (o VO usa `"boleto"`, `"ted"`, etc.). No slot de clarificação de forma de pagamento, isso faz `ParsePaymentMethod` falhar para métodos in-scope da PRD. Correção: alinhar os valores do mapa aos **códigos exatos do enum da tool / VO** para os métodos suportados pela PRD (`pix`, `cash`, `debit_card`, `debit_in_account`, `credit_card`, `boleto`, `vale_refeicao`, `vale_alimentacao`). Gate de verificação: teste de tabela assertando que **todo valor** de `knownPaymentMethods` é aceito por `ParsePaymentMethod` (VO). Entradas fora do escopo da PRD (`ted`/`doc`/`transferencia`) são registradas em Riscos Conhecidos.

### Instruções do agente (`const mecontrolaAgentInstructions`)

Adições textuais (sem builder; é `const` string):

- **Cinco campos obrigatórios** por lançamento e regra de não-invenção (RF-01, RF-21).
- **Datas:** o agente **repassa** o texto de data cru em `occurredAt` (incluindo dias da semana como "terça"), **sem convertê-lo**; a conversão é do sistema (ADR-002).
- **Fronteira multi-item (RF-16):** ao detectar múltiplos lançamentos numa mensagem, pedir um por vez, sem registrar nada.
- **Reforço do mapeamento de pagamento** já presente (manter enum) e da formatação `*asterisco simples*` (já presente).

## Pontos de Integração

- **OpenRouter via `internal/platform/llm`** — inalterado; nenhum novo provider/fallback.
- **`internal/transactions` / `internal/card` / `internal/categories`** — consumidos via bindings existentes (`transactions_ledger_adapter`, `card_manager_adapter`, `categories_reader_adapter`); nenhuma alteração de contrato de módulo.
- **`internal/budgets`** — continua consumindo `TransactionCreated`; contrato de evento inalterado.

## Abordagem de Testes

### Testes Unitários

- `parseWeekday` (whitebox, pacote `workflows`): tabela com `now` fixo em cada dia da semana; `segunda..domingo`, `X passada/passado`, variações com acento e `-feira`; não-casamento de "semana/mês passado" (retorna `""`). Puro, sem mock.
- `PendingOperationKind`: ida e volta `String()`↔`ParsePendingOperationKind`, erro em valor inválido.
- `validateEntryAmount`: `0`, negativo, no teto, acima do teto, valor normal.
- `knownPaymentMethods` × `ParsePaymentMethod`: todo valor do mapa parseia para VO válido.
- Testes de use case (`register_attempt` e continuer) seguem R-TESTING-001 (testify/suite whitebox, `fake.NewProvider()`, dependencies struct com IIFE por mock) quando tocados.

### Testes de Integração / Workflow (harness in-memory)

Reaproveitar `pendingEntryHarness` (`internal/agents/application/agents/pending_entry_harness_test.go`). `newPEHarness` passa a injetar a porta `IdempotentWriter` (fake in-memory sobre um `WriteLedger` fake). Novos cenários:

- **Idempotência durável de write-ledger** (distinta de `TestG7_09` que cobre replay de mensagem): mesma `(wamid_original, itemSeq, operation)` executada 2× → **1 INSERT** no ledger fake, 2ª retorna `ToolOutcomeReplay` e completa com o mesmo texto de sucesso.
- **Data por dia da semana**: estado com `OccurredAt` derivado de "terça" resolvido pelo parser.
- **Teto de valor**: exec de tool com `amountCents` acima do teto → resposta de correção, sem `engine.Start`.

Decisão de integration tests (testcontainers): **não** para esta feature — a lógica é pura/estado e o harness in-memory + real-LLM cobre o risco; a dedup durável é validada com fake do `WriteLedger` (o repositório Postgres já tem cobertura própria). Critério de reavaliação: se a dedup durável precisar validar concorrência real no Postgres (`ON CONFLICT DO NOTHING` sob corrida), adicionar um teste `//go:build integration` específico.

### Testes E2E / Real-LLM (obrigatório — `feedback_realllm_validation_required`)

`//go:build integration`, `RUN_REAL_LLM=1` + `OPENROUTER_*` do `.env`. Estender `pending_entry_realllm_test.go` / `mecontrola_agent_realllm_test.go` cobrindo R1–R7 + data por dia da semana + rejeição de "semana/mês passado" + ambiguidade de categoria e de cartão + confirmação/cancelamento + valor inválido. Meta **M-04 ≥ 0,90**; M-03/M-05 = 0 violações.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **State-as-type + estado**: `PendingOperationKind.String()/IsValid()/Parse` + `ItemSeq` em `PendingEntryState` (base para a chave; sem dependência externa).
2. **Porta + executeWrite**: `IdempotentWriter`/`IdempotentWriteFn` no `workflows`; alterar `BuildPendingEntryWorkflow`/`executeWrite`/`callLedger`; `resourceKind`.
3. **Wiring**: adapter no `module`, construir `IdempotentWrite` do `writeLedgerRepo`, injetar; atualizar `RegisterAttempt.*` para popular `ItemSeq`; atualizar `newPEHarness`.
4. **parseWeekday** e encaixe em `parseInputDate`.
5. **validateEntryAmount** nos execs de tool.
6. **Correção `knownPaymentMethods`**.
7. **Instruções do agente**.
8. **Testes** (unit + harness + real-LLM) e validação.

### Dependências Técnicas

- Nenhuma infra nova. `writeLedgerRepo` já é construído em `module.go:139` com `deps.DB` — disponível para construir `IdempotentWrite` antes de `BuildPendingEntryWorkflow` (module.go:193).

## Monitoramento e Observabilidade

- Counter do `IdempotentWrite` (já existente) passa a registrar replays em produção; labels enum-fechados (`outcome`), **sem** `user_id`/`wamid`/`category_id` (R-AGENT-WF-001.5, R-TXN-004).
- Opcional: counter de rejeições de valor por motivo (`amount_non_positive`, `amount_above_ceiling`).
- Logs mantêm o padrão atual; nenhuma PII nova em log.

## Considerações Técnicas

### Decisões Chave

- **ADR-001** — Idempotência em `executeWrite` com chave no wamid original + porta anti-ciclo: [`adr-001-idempotencia-executewrite-wamid-original.md`](adr-001-idempotencia-executewrite-wamid-original.md).
- **ADR-002** — Parser de dias da semana como função pura: [`adr-002-parser-data-dias-semana-funcao-pura.md`](adr-002-parser-data-dias-semana-funcao-pura.md).
- **ADR-003** — Teto de valor na camada do agente: [`adr-003-teto-valor-camada-agente.md`](adr-003-teto-valor-camada-agente.md).

### Riscos Conhecidos

- **Duas camadas de idempotência** (mensagem vs write-ledger durável): documentadas como complementares (ADR-001); risco de confusão mitigado por testes separados.
- **`knownPaymentMethods` fora de escopo** (`ted`/`doc`/`transferencia` → `bank_transfer`, inválido no VO): não são métodos declarados na PRD; ficam como bug latente conhecido a ser corrigido em iniciativa própria — esta PRD corrige apenas os métodos in-scope e adiciona o gate `map → ParsePaymentMethod`.
- **Ciclo de import** se a porta retornar tipos de `usecases`: mitigado retornando primitivos + `agent.ToolOutcome` (ADR-001).
- **Ambiguidade de "segunda"**: contrato fixado (ocorrência mais recente incluindo hoje; "passada" = −7) e testado.

### Conformidade com Padrões

- **R-ADAPTER-001** (zero comentários; adapters finos): tools e adapter de idempotência permanecem finos, delegando a use case; zero comentários em `.go` de produção.
- **R-AGENT-WF-001** (roteamento por registry, sem `switch intent.Kind`; Tool fina; estados fechados; LLM só nas call-sites sancionadas; Run auditável): mantido — a mudança não introduz branching de domínio no adapter nem LLM no kernel; `ToolOutcome`/`RunStatus` seguem tipos fechados.
- **R-WF-KERNEL-001**: o kernel `internal/platform/workflow` **não** é alterado; toda mudança vive no consumidor `internal/agents`.
- **R-TESTING-001**: testes de use case em testify/suite whitebox com `fake.NewProvider()`.
- **DMMF** (`domain-modeling.md`): pure core / IO shell (`parseWeekday`, `validateEntryAmount` puras; `now` injetado, sem `Clock`), state-as-type (`PendingOperationKind`), smart constructors (`ParsePendingOperationKind`); sem `Result[T,E]`/mônadas.
- **R-DTO-VALIDATE-001**: não introduz novo input DTO em `application/dtos/input/`; guardas vivem em funções puras da camada de tools.

### Arquivos Relevantes e Dependentes

- `internal/agents/application/workflows/pending_entry_state.go` — `ItemSeq`, `PendingOperationKind` state-as-type.
- `internal/agents/application/workflows/pending_entry_workflow.go` — `BuildPendingEntryWorkflow`, `executeWrite`, `callLedger`, `resourceKind`, porta `IdempotentWriter`.
- `internal/agents/application/workflows/pending_entry_decisions.go` — `parseInputDate`, `parseWeekday`, `knownPaymentMethods`.
- `internal/agents/application/usecases/idempotent_write.go`, `write_ledger.go` — `IdempotentWrite`, `WriteFn`, `WriteLedgerRepository` (consumidos, não alterados).
- `internal/agents/application/usecases/register_attempt.go`, `register_entry.go` — popular `ItemSeq`.
- `internal/agents/application/tools/register_expense.go`, `register_income.go` — `validateEntryAmount`.
- `internal/agents/application/agents/mecontrola_agent.go` — `const mecontrolaAgentInstructions`.
- `internal/agents/module.go` — adapter + wiring (`writeLedgerRepo` → `IdempotentWrite` → `BuildPendingEntryWorkflow`).
- `internal/agents/application/agents/pending_entry_harness_test.go`, `*_realllm_test.go` — testes.
- `internal/transactions/domain/valueobjects/payment_method.go` — fonte dos códigos válidos (gate do mapa).
