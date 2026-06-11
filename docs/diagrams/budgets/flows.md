# Budgets Flows

![Budgets container context](../system/mecontrola-container.svg)

## Objetivo do modulo

`internal/budgets` gerencia orcamentos mensais, despesas, recorrencia, alertas por threshold e ingestao de eventos externos de despesa.

## Arquivos .puml por fluxo

- [BUD-01-create-budget.puml](./BUD-01-create-budget.puml)
- [BUD-02-create-expense.puml](./BUD-02-create-expense.puml)
- [BUD-03-update-expense.puml](./BUD-03-update-expense.puml)
- [BUD-04-delete-expense.puml](./BUD-04-delete-expense.puml)
- [BUD-05-evaluate-alert.puml](./BUD-05-evaluate-alert.puml)
- [BUD-06-ingest-external-expense.puml](./BUD-06-ingest-external-expense.puml)
- [BUD-07-pending-events-reaper.puml](./BUD-07-pending-events-reaper.puml)
- [BUD-08-summary-and-alerts-read.puml](./BUD-08-summary-and-alerts-read.puml)

## Entradas, saidas e artefatos

### Entradas HTTP

- `POST /api/v1/budgets/`
- `POST /api/v1/budgets/recurrence`
- `GET /api/v1/budgets/alerts`
- `POST /api/v1/budgets/expenses`
- `PATCH /api/v1/budgets/expenses/{id}`
- `DELETE /api/v1/budgets/expenses/{id}`
- `POST /api/v1/budgets/{competence}/activate`
- `DELETE /api/v1/budgets/{competence}`
- `GET /api/v1/budgets/{competence}/summary`

### Entradas async

- Consumer `budgets.expense.committed.v1`
- Consumer `external.expense.v1`
- Job `budgets-abandoned-draft-reaper`
- Job `budgets-pending-events-reaper`
- Job `budgets-retention-purge`

### Saidas

- Escrita em `budgets`, `allocations`, `expenses`, `alerts`, `threshold_states`, `pending_events`
- Leitura do modulo `categories` via `CategoriesReaderAdapter`
- Publicacao em `outbox_events` de `budgets.expense.committed.v1`

## Matriz de fluxos

| ID | Origem | Tipo | Saida principal |
| --- | --- | --- | --- |
| BUD-01 | `POST /api/v1/budgets/` | sync | Cria rascunho/estrutura de budget |
| BUD-02 | `POST /api/v1/budgets/expenses` | sync + async | Cria despesa e publica `budgets.expense.committed.v1` |
| BUD-03 | `PATCH /api/v1/budgets/expenses/{id}` | sync + async | Atualiza despesa e republica `budgets.expense.committed.v1` |
| BUD-04 | `DELETE /api/v1/budgets/expenses/{id}` | sync + async | Soft delete e publica `budgets.expense.committed.v1` com mutation delete |
| BUD-05 | consumer `budgets.expense.committed.v1` | async | Recalcula thresholds e grava alertas |
| BUD-06 | consumer `external.expense.v1` | async | Aplica create/update/delete externo ou enfileira pending event |
| BUD-07 | `budgets-pending-events-reaper` | sync | Reaplica pending events em backlog |
| BUD-08 | `GET /api/v1/budgets/{competence}/summary` e `GET /api/v1/budgets/alerts` | sync | Leitura agregada |

## Percurso detalhado

### BUD-01 - Criacao de budget

Origem:
- `CreateBudgetHandler.Handle`

Percurso:
1. `middleware.RequireUser` garante principal autenticado.
2. O handler decodifica `competence`, `total_cents` e `allocations`.
3. Chama `CreateBudget.Execute`.
4. O use case valida command/domain invariants.
5. Persiste budget e allocations no repositrio transacional.
6. Retorna `Location: /api/v1/budgets/{competence}`.

Banco:
- Escrita em `budgets` e `allocations`

### BUD-02 e BUD-03 - Upsert de despesa

Origem:
- `UpsertExpenseHandler.HandleCreate`
- `UpsertExpenseHandler.HandleUpdate`

Percurso:
1. O handler injeta `UserID` do principal e `Source=api`.
2. `UpsertExpense.Execute` monta `UpsertExpenseCommand`.
3. Resolve a `root_slug` chamando `CategoriesReader.ValidateExpenseSubcategory`.
4. Entra em transacao `uow`.
5. `ExpenseRepository.GetByIdentity` procura a identidade canonica `(user_id, source, external_transaction_id)`.
6. Se a despesa nao existir:
   - `CreateOrAutoDraftForExpense.EnsureExists` cria budget draft automaticamente se necessario;
   - `ExpenseRepository.Insert` grava a despesa.
7. Se a despesa existir e vier `ExpectedVersion`:
   - edita a entidade;
   - `ExpenseRepository.Update` persiste.
8. Em ambos os casos o use case publica `budgets.expense.committed.v1` via `ExpenseCommittedPublisher`.
9. O endpoint responde sem esperar a avaliacao de alertas.

Banco:
- Leitura/escrita em `expenses`
- Leitura/escrita eventual em `budgets` para auto-draft
- Escrita em `outbox_events`

### BUD-04 - Exclusao de despesa

Origem:
- `DeleteExpenseHandler.Handle`

Percurso:
1. O handler resolve `{id}` como `ExternalTransactionID`.
2. Chama `DeleteExpense.Execute`.
3. Em transacao:
   - le a despesa por identidade;
   - ignora se tombstone ou ja deletada;
   - executa `ExpenseRepository.SoftDelete`;
   - publica `budgets.expense.committed.v1` com `mutation_kind=delete`.

Banco:
- Leitura/escrita em `expenses`
- Escrita em `outbox_events`

### BUD-05 - Avaliacao de alertas

Origem:
- Consumer `budgets.expense.committed.v1`

Percurso:
1. `ExpenseCommittedConsumer.Handle` valida `event_type`.
2. Desserializa `user_id`, `competence`, `root_slug`, `committed_at` e `cutoff_competence_br`.
3. Chama `EvaluateAlert.Execute`.
4. O use case:
   - soma gasto por `root_slug` em `ExpenseRepository.SumByRoot`;
   - carrega budget ativo em `BudgetRepository.GetByUserCompetence`;
   - consulta `ThresholdStateRepository.GetCurrentlyCrossed`;
   - calcula transicoes com `services.EvaluateThresholds`;
   - faz `UpsertIfTransition`;
   - grava `AlertRepository.Insert` quando houver crossing relevante.

Banco:
- Leitura em `expenses`
- Leitura em `budgets`
- Leitura/escrita em `threshold_states`
- Escrita em `alerts`

### BUD-06 - Ingestao de evento externo

Origem:
- Consumer `external.expense.v1`

Percurso:
1. `ExternalExpenseConsumer.Handle` decodifica `event_id`, `source`, `operation`, `version`, `competence`, `amount_cents`.
2. Chama `IngestExternalExpense.Execute`.
3. O use case valida:
   - `source` na allowlist;
   - campos obrigatorios;
   - consistencia da versao.
4. Se a operacao for `create`, chama `UpsertExpense.Execute`.
5. Se a operacao for `update` ou `delete`, tenta aplicar direto com `ExpectedVersion=version-1`.
6. Se houver conflito de ordem ou dependencia ausente:
   - serializa payload;
   - grava `PendingEventRepository.Insert`.

Banco:
- Leitura/escrita em `expenses`
- Escrita eventual em `pending_events`
- Escrita em `outbox_events` se a mutacao aplicar com sucesso

### BUD-07 - Pending events reaper

Origem:
- Job `budgets-pending-events-reaper`
- Schedule: `cfg.BudgetsConfig.PendingReaperInterval`

Percurso:
1. O job chama `RunPendingEventsReaper.Execute`.
2. O use case seleciona pendencias dentro da janela configurada.
3. Reaplica cada mutacao chamando `ApplyPendingEvent`, que volta a usar `UpsertExpense` ou `DeleteExpense`.
4. Se a aplicacao funcionar, a pendencia sai do backlog.

### BUD-08 - Leituras agregadas

Origem:
- `GetMonthlySummaryHandler.Handle`
- `ListAlertsHandler.Handle`

Percurso:
1. O handler injeta `user_id` do principal.
2. O use case de leitura usa UoW read-only.
3. Monta DTO de saida com mappers de aplicacao.

## Rotas internas e dependencias cruzadas

- O modulo depende do `categoriesModule` para validar subcategoria e resolver `root_slug`.
- O modulo nao chama outros modulos diretamente em runtime HTTP alem desse adaptador de categorias.
- O proprio modulo consome o evento que publica para desacoplar a escrita da despesa da avaliacao de alertas.

## Observacoes arquiteturais

- A criacao de despesa pode auto-criar um budget draft se o budget ainda nao existir.
- O evento `budgets.expense.committed.v1` funciona como gatilho interno de consistencia para alertas.
- `external.expense.v1` usa pending backlog para reorder/resilience, evitando falha definitiva por versao fora de ordem.

## Eficiencia, robustez e operacao

- `Caminho critico`
  - mutacoes de despesa tocam validacao editorial, tabela de expenses e potencialmente budgets auto-draft;
  - avaliacao de alertas desloca o custo de agregacao para o worker.
- `Controles de robustez`
  - `ExpectedVersion` para update/delete;
  - tombstone conflict para identidade canonica reutilizada indevidamente;
  - outbox para `budgets.expense.committed.v1`;
  - pending backlog para eventos externos fora de ordem.
- `Falhas esperadas`
  - subcategoria invalida ou payload invalido: falha definitiva de request;
  - conflito de versao: falha ordenacional, retorna erro ao caller ou vai para backlog externo;
  - dependencia editorial indisponivel: falha transiente do caminho sincrono;
  - backlog de pending events crescendo: sinal operacional de reorder persistente.
- `Observabilidade`
  - counters de decode failed e source rejected nos consumers;
  - logs warn em `upsert_expense`, `delete_expense`, `evaluate_alert` e ingestao externa;
  - tamanho de `pending_events` e volume de `alerts` devem ser acompanhados.
- `Capacidade`
  - `EvaluateAlert` e bound por soma em `expenses` e leitura do budget ativo;
  - fluxos externos podem amplificar escrita em `pending_events` sob reorder alto;
  - jobs de retention e reaper devem ser ajustados ao tamanho historico.

## Guardrails operacionais

### Precondicoes e pos-condicoes

- `BUD-02/BUD-03/BUD-04`
  - pre: principal autenticado, subcategoria valida, banco disponivel;
  - pos: despesa persistida ou rejeitada com causa clara, e envelope `budgets.expense.committed.v1` emitido quando a mutacao efetiva ocorrer.
- `BUD-05`
  - pre: evento committed valido e budget ativo para a competencia;
  - pos: threshold state coerente e alertas gerados apenas em transicao real.
- `BUD-06/BUD-07`
  - pre: source autorizada e evento externo bem formado;
  - pos: mutacao aplicada ou `pending_event` persistido para reprocessamento.

### Invariantes

- a identidade canonica `(user_id, source, external_transaction_id)` define uma unica despesa logica;
- conflito de versao nao pode sobrescrever estado mais novo;
- `pending_events` existe para reorder, nao para armazenar payload invalido;
- `alerts` so devem nascer de crossing real confirmado por `threshold_states`.

### Runbook resumido

- aumento de conflitos em expenses:
  - checar taxa de `expense_version_conflict`;
  - investigar concorrencia de clientes ou replay externo.
- crescimento de `pending_events`:
  - validar ordenacao dos produtores externos;
  - conferir se o reaper esta executando no intervalo esperado;
  - amostrar payloads presos por muitos ciclos.
- ausencia de alertas:
  - confirmar consumo de `budgets.expense.committed.v1`;
  - validar budget ativo e leituras de `SumByRoot`.

### Sinais e thresholds recomendados

- alerta se `pending_events` ultrapassar baseline historico por competencia ou source;
- alerta se `budgets_expense_committed_consumer_decode_failed_total` aumentar acima de zero em producao;
- alerta se `budgets_external_expense_source_rejected_total` subir de forma sustentada;
- alerta se jobs de retention/reaper atrasarem mais de um ciclo.
