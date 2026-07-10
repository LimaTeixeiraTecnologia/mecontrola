<!-- spec-hash-prd: b3ed073a189476b0b0f275e097a35dc8f4a128ab9b9bf3343052e07ce17ffd2e -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Jornada WhatsApp financeira sem falso sucesso

PRD: `.specs/prd-jornada-whatsapp-financeira-sem-falso-sucesso/prd.md` (spec-version 2, 30 RFs, 10 decisões).
Skills aplicadas: `go-implementation`, `domain-modeling-production`, `mastra`, `design-patterns-mandatory`.

## Resumo Executivo

Esta feature elimina o falso sucesso na jornada financeira WhatsApp por meio de correções locais e
reforço de invariantes sobre os workflows, tools e adapters já existentes — **sem novo pattern GoF**
(o seletor `design-patterns-mandatory` retornou `reject`; a solução é refactor local e imposição de
invariantes) e **sem criação de schema** (todas as 14 tabelas já existem em
`000001_initial_schema.up.sql`; apenas uma migration aditiva de coluna é introduzida). A investigação
detalhada (cinco frentes de código) confirmou que os defeitos são de três naturezas: (a) **domínio**
— validação assimétrica de distribuição de orçamento e sobrescrita indevida da personalização; (b)
**máquina de estado do pending-entry** — uma escrita aceita sem recurso durável finalizava como
`StepStatusCompleted`+`PendingStatusCancelled`, que o kernel mapeia para `RunStatusSucceeded` (falso
sucesso), impedindo inclusive o retry já existente; (c) **observabilidade/dados** — o scorer nunca
recebia o `outcome` real da tool (cadeia de descarte em `ScoringHooks.AfterTool`), os continuers
engoliam o erro de `run.Update`, e o vínculo canônico de identidade não era materializado no fallback
legado.

A estratégia central é **tornar estados ilegais irrepresentáveis** (DMMF state-as-type): a escrita
aceita sem recurso passa a ser erro de negócio tipado com `StepStatusFailed`; `PendingStatusCancelled`
fica reservado a cancelamento/expiração/substituição; o `RunSample` passa a carregar o `outcome`
determinístico da tool para um scorer per-run de persistência; e o WAMID vira invariante de fronteira
no `InboundRequest.Validate()`. Seis ADRs registram as decisões materiais. A validação é protegida por
golden set da conversa real, gates Go e gate real-LLM ≥0,90 por categoria.

## Arquitetura do Sistema

### Visão Geral dos Componentes

Componentes **modificados** (nenhum componente novo de plataforma; consumo do substrato Mastra
existente conforme regra de ouro da skill `mastra`):

| Componente | Caminho | Mudança | RF | ADR |
|-----------|---------|---------|----|----|
| Comando de orçamento | `internal/budgets/domain/commands/create_budget.go` | `NewCreateBudgetCommand:69` `> 10000` → `!= 10000` (validação simétrica, fonte única) | RF-03, RF-29 | ADR-003 |
| Workflow de onboarding | `internal/agents/application/workflows/onboarding_workflow.go` | `DecideAllocationsBP` ramo `confirm` só aplica default se `sum(values)==0`; prompt endurecido; `DecideAllocationKind` puro determinístico (obrigatório) | RF-01, RF-29 | ADR-003 |
| Workflow pending-entry | `internal/agents/application/workflows/pending_entry_workflow.go` | erro tipado `ErrWriteAcceptedWithoutResource`+`StepStatusFailed` no ramo `resourceID==Nil`; `DecidePostWrite` puro; `ProcessedMessageID` no accept; `maxFailedWriteResumes` 2→1; transição `Active→Expired`; nova constante `ActivePendingEntryMessage` | RF-05, RF-07, RF-09, RF-10, RF-11, RF-12, RF-30 | ADR-001, ADR-002 |
| Continuers de resume | `internal/agents/application/usecases/pending_entry_continuer.go`, `card_create_confirm_continuer.go`, `budget_creation_continuer.go` | `closeRun` não engole `run.Update`: log+métrica `agents_run_update_errors_total` | RF-14, RF-15, RF-23 | ADR-005 |
| Runtime da plataforma | `internal/platform/agent/runtime.go`, `ports.go` | `InboundRequest.Validate()` exige `MessageID` (sentinela `ErrEmptyMessageID`); span com `wamid` | RF-13, RF-22 | ADR-005 |
| RegisterAttempt | `internal/agents/application/usecases/register_attempt.go` | `ErrRunAlreadyExists` → `ActivePendingEntryMessage` (não `MultiItemOrientationMessage`) | RF-06 | — |
| EstablishPrincipal | `internal/identity/application/usecases/establish_principal.go` | `ensureIdentityLink` na mesma tx no fallback legado; `AuthResolvePath` fechado; `resolve_path` em auth_events | RF-20, RF-21 | ADR-006 |
| Scorer de plataforma | `internal/platform/scorer/scorer.go` | `ToolCallRecord.Outcome string` (aditivo) | RF-17 | ADR-004 |
| ScoringHooks | `internal/agents/application/agents/scoring_hooks.go` | `AfterTool` deixa de descartar `resultBytes` e tools com erro; `extractOutcome` | RF-17 | ADR-004 |
| Scorers do consumidor | `internal/agents/application/scorers/*.go` | novo `write_persistence_accuracy` (code-based); `no_hallucination` endurecido | RF-17, RF-18 | ADR-004 |
| Status WhatsApp | `internal/platform/whatsapp/status/*.go` | `LookupDeliveryState` + `MessageDeliveryState` fechado (read-only) | RF-19 | — |
| Auditoria de reconciliação | `internal/agents/infrastructure/postgres/` (novo `audit_reconciliation.go`) | query read-only de concordância de estado | RF-16 | ADR-005 |
| Migration aditiva | `migrations/000008_auth_events_resolve_path.up.sql` | `ADD COLUMN resolve_path` + CHECK; backfill dos 4 runs legados + CHECK `platform_runs.correlation_key` (obrigatório) | RF-15, RF-21 | ADR-005, ADR-006 |

### Fluxo de Dados (invariante central)

O caminho canônico `dedup → identidade → entitlement → resumer/workflow → tool/use case →
ledger/transação/orçamento → mensagem → run/scorer/trace` é preservado. Os pontos de imposição de
invariante adicionados:

1. **Fronteira do runtime**: `InboundRequest.Validate()` rejeita `MessageID` vazio → todo run nasce
   com WAMID em `CorrelationKey` (ADR-005).
2. **Domínio de orçamento**: `NewCreateBudgetCommand` rejeita `Σbp != 10000` → nenhum draft parcial
   gravável (ADR-003).
3. **Pós-escrita do pending-entry**: `DecidePostWrite` (puro) ⇒ `resourceID==Nil && outcome!=Replay`
   ⇒ `StepStatusFailed` + `ErrWriteAcceptedWithoutResource`; sucesso só com efeito durável (ADR-001).
4. **Scorer per-run**: `write_persistence_accuracy` reprova run cuja write-tool não produziu efeito
   (ADR-004).

## Design de Implementação

### Interfaces e tipos-chave

Erro de negócio tipado e decisão pura pós-escrita (ADR-001), pacote `workflows`:

```go
var ErrWriteAcceptedWithoutResource = errors.New("workflows.pending_entry: escrita aceita sem recurso durável")

func DecidePostWrite(outcome agent.ToolOutcome, resourceID uuid.UUID) (PendingStatus, workflow.StepStatus, error) {
    if outcome != agent.ToolOutcomeReplay && resourceID == uuid.Nil {
        return PendingStatusActive, workflow.StepStatusFailed, ErrWriteAcceptedWithoutResource
    }
    return PendingStatusCompleted, workflow.StepStatusCompleted, nil
}
```

Invariante de fronteira do runtime (ADR-005), pacote `agent`:

```go
var ErrEmptyMessageID = errors.New("agent: message_id vazio")

func (i *InboundRequest) Validate() error {
    var errs []error
    if i.MessageID == "" {
        errs = append(errs, fmt.Errorf("message_id: %w", ErrEmptyMessageID))
    }
    return errors.Join(errs...)
}
```

Propagação do efeito ao scorer (ADR-004) — `Outcome` como `string` para preservar o layering
(kernel `scorer` não importa `agent`):

```go
type ToolCallRecord struct {
    ID      string
    Name    string
    Args    map[string]any
    Outcome string
}
```

Tipo fechado de path de resolução de identidade (ADR-006), pacote `identity/domain`:

```go
type AuthResolvePath string

const (
    AuthResolvePathIdentity AuthResolvePath = "identity"
    AuthResolvePathLegacy   AuthResolvePath = "legacy"
    AuthResolvePathBackfill AuthResolvePath = "backfill"
)
```

Estado de entrega distinguível (RF-19), pacote `whatsapp/status`:

```go
type MessageDeliveryState string

const (
    DeliveryStateNotReceived MessageDeliveryState = "not_received"
    DeliveryStateFailed      MessageDeliveryState = "failed"
    DeliveryStateDelivered   MessageDeliveryState = "delivered"
)

type MessageStatusReader interface {
    LookupDeliveryState(ctx context.Context, messageID string) (MessageDeliveryState, error)
}
```

### Modelos de Dados

- **Migration 000008 (aditiva)** — `auth_events.resolve_path TEXT NULL` +
  `CHECK (resolve_path IS NULL OR resolve_path IN ('identity','legacy','backfill'))`. Preserva
  `auth_events_reason_check` intacto (que proíbe `reason` não-nulo quando `kind != 'failed'`),
  motivo pelo qual a coluna nova é necessária em vez de reusar `reason` (ADR-006).
- **Obrigatório, mesma migration** — backfill idempotente dos 4 runs legados
  (`UPDATE mecontrola.platform_runs SET correlation_key = 'legacy:' || id::text WHERE correlation_key = ''`)
  seguido de `ADD CONSTRAINT platform_runs_correlation_len_chk CHECK (char_length(correlation_key)
  BETWEEN 1 AND 256)` (validado, não `NOT VALID`) — defesa em profundidade além da validação em Go
  (ADR-005).
- **Sem alteração de chave** em `agents_write_ledger` (`wamid, item_seq, operation`): a idempotência
  por pendência já é obtida porque `wamid = state.MessageID` (WAMID original, estável por pendência)
  (ADR-002).

### Semântica de idempotência e retry (ADR-002)

| Cenário (2º "Sim", WAMID distinto) | Camada que resolve | Resultado |
|---|---|---|
| 1ª escrita persistiu | `IdempotentWrite.Execute` → `FindByKey` | `ToolOutcomeReplay` + resourceID original → `PendingStatusCompleted`, sem 2ª mutação |
| 1ª falhou antes de persistir | `tryResumeFailedWrite` + `SeedResumeAfterFailedWrite` | retry controlado (máx 1, `maxFailedWriteResumes=1`), mesma chave de ledger |
| mesma mensagem reprocessada | `DecideConfirmation` via `ProcessedMessageID` (gravado no accept) | `ConfirmActionReplay` |
| TTL 30min excedido | `isExpired` no resume | transição `Active → PendingStatusExpired` |

## Abordagem de Testes

### Testes Unitários

Decisões puras (sem mock, sem IO — DMMF), whitebox `package X`:
- `DecidePostWrite`: `routed`+resourceID≠Nil ⇒ Completed; `routed`+Nil ⇒ Failed+`errors.Is(...,
  ErrWriteAcceptedWithoutResource)`+`Active`; `replay` ⇒ Completed (ADR-001).
- `DecideConfirmation`/retry: accept, replay-por-mensagem, retry-1, retry-esgotado, expiração TTL,
  dois "Sim" WAMIDs distintos (ADR-002).
- `NewCreateBudgetCommand`: `Σbp=9000`⇒erro, `11000`⇒erro, `10000`⇒ok (ADR-003).
- `DecideAllocationsBP`: caso da conversa real (`2500/0/500/0/2000`, renda `500000`) ⇒
  `5000/0/1000/0/4000` fecha 100%; `confirm` com valores não-nulos ⇒ não aplica default (ADR-003).
- `write_persistence_accuracy`: `routed`⇒1; `usecaseError`/vazio⇒0; `replay`/`clarify`⇒neutro 1;
  misto⇒0 (falha-segura) (ADR-004).
- `no_hallucination` endurecido: marcador de sucesso + `replay`⇒0 (ADR-004).
- `InboundRequest.Validate`: `MessageID=""`⇒erro nomeando `message_id` (ADR-005).
- `AuthResolvePath`/`MessageDeliveryState`: `IsValid`/predicado puro (ADR-006, RF-19).

Suites `testify/suite` table-driven (R-TESTING-001) para os use cases que tocam mocks
(`IdempotentWriter`, `RunStore`, `UserIdentityRepository`), com `fake.NewProvider()` e IIFE por mock.

### Testes de Integração

Critérios de adoção atendidos (fronteiras de IO críticas + incidente real onde unit passou e a
integração falhou). Usar `testcontainers-go` com `//go:build integration`:
- Onboarding e2e: distribuição personalizada `2500/0/500/0/2000` ⇒ `budgets_allocations` grava
  `5000/0/1000/0/4000`, budget ativo, **nenhuma** linha `4000/1000/1000/1000/3000` (RF-01, RF-04).
- Write ledger: 2 Insert com `(wamid,item_seq,operation)` iguais ⇒ `ON CONFLICT DO NOTHING`;
  `FindByKey` força ramo replay (RF-09).
- Correlation: run routed e resume ⇒ `correlation_key` não-vazio = WAMID; `Update` com erro injetado
  ⇒ incrementa `agents_run_update_errors_total` + log, sem silenciar (RF-13, RF-14, RF-15).
- Reconciliação: após fluxo financeiro, a query de auditoria retorna 0 violações de invariante; caso
  negativo (run failed com write pré-existente) é **detectado** (RF-16).
- Identidade: resolução legada cria 1 vínculo ativo + `auth_events.resolve_path='legacy'`; 2ª chamada
  idempotente (`identity`, sem 2ª linha); 2 requests concorrentes ⇒ 1 vínculo (RF-20, RF-21).
- Status: inserir 0/`failed`/`sent` ⇒ `LookupDeliveryState` = `not_received`/`failed`/`delivered`
  (RF-19).

### Testes E2E (golden + real-LLM, RF-25, RF-27)

Golden set anonimizado/sintético da conversa real (ativação, personalização válida e inválida,
"Gastei 10 na padaria no dinheiro", "Gastei 19 na padaria no Pix", "Hoje", "Sim", repetição de
"Sim") provando: ausência de falso múltiplo lançamento, ausência de orçamento padrão indevido,
ausência de confirmação duplicada, presença de transação final. Gate real-LLM (`RUN_REAL_LLM=1`)
≥0,90 por categoria — obrigatório por precedente do projeto (unit determinístico já mascarou defeitos
via brittleness).

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Domínio puro, sem dependência cruzada** (paralelizável): `NewCreateBudgetCommand` simétrico
   (ADR-003); `DecidePostWrite` + `ErrWriteAcceptedWithoutResource` (ADR-001); `AuthResolvePath` e
   `MessageDeliveryState` (tipos fechados).
2. **Workflows e estado** (depende de 1): pending-entry (ADR-001/002 — erro tipado, `ProcessedMessageID`
   no accept, `maxFailedWriteResumes=1`, `Active→Expired`); onboarding/budget confirm-branch + prompt
   (ADR-003); `ActivePendingEntryMessage` (RF-06).
3. **Plataforma e observabilidade** (paralelo a 2): `InboundRequest.Validate` (ADR-005);
   `scorer.ToolCallRecord.Outcome` + `ScoringHooks.AfterTool` (ADR-004).
4. **Consumidores** (depende de 3): continuers `closeRun` + métrica (ADR-005); `write_persistence_accuracy`
   + `no_hallucination` (ADR-004); `ensureIdentityLink` (ADR-006).
5. **Migration + auditoria** (depende de 4): migration 000008; `LookupDeliveryState`; query de
   reconciliação (RF-16).
6. **Testes de integração + golden/real-LLM** (depende de 1–5).

### Dependências Técnicas

- Postgres (testcontainers) para integração; OpenRouter para o gate real-LLM (provider único, sem
  fallback — restrição do PRD).

## Monitoramento e Observabilidade

- Nova métrica `agents_run_update_errors_total` — Counter, labels fechados `workflow`, `stage`,
  `status` (nunca `user_id`/WAMID/`correlation_key`). RF-15/RF-23/RF-24.
- Novo scorer `write_persistence_accuracy` em `platform_scorer_results` — diferencia "write aceita
  sem persistência" (score<1) de write legítima. RF-17/RF-18.
- Logs/spans padronizados com `run_id`, `wamid`, `workflow`, `stage`, `status`, `outcome` ao longo
  do pipeline. RF-22.
- Query de reconciliação (read-only) cruzando `platform_runs × workflow_runs × agents_write_ledger ×
  transactions × platform_scorer_results` — invariante de concordância verificada em teste, sem
  dashboard novo. RF-16.
- Gate de cardinalidade (grep) confirmando ausência de label sensível em métricas. RF-24.

## Considerações Técnicas

### Decisões Chave (ADRs)

- **ADR-001** — [Escrita aceita sem recurso durável vira falha tipada](adr-001-escrita-aceita-sem-recurso-duravel.md) (RF-05/07/10/11/12).
- **ADR-002** — [Idempotência por pendência/operação e retry controlado](adr-002-idempotencia-por-pendencia-e-retry-controlado.md) (RF-09/30).
- **ADR-003** — [Validação simétrica de distribuição de orçamento](adr-003-validacao-simetrica-distribuicao-orcamento.md) (RF-01/02/03/04/29).
- **ADR-004** — [Scorer de persistência per-run](adr-004-scorer-persistencia-per-run.md) (RF-17/18).
- **ADR-005** — [Correlação por WAMID e run.Update observável](adr-005-correlacao-wamid-e-run-update-observavel.md) (RF-13/14/15/16/22/23/24).
- **ADR-006** — [Identidade canônica com coluna resolve_path](adr-006-identidade-canonica-resolve-path.md) (RF-20/21).

RF-06 (mensagem distinta para pendência ativa) e RF-19 (estado de entrega distinguível) são
correções diretas sem ADR próprio — registradas nos componentes acima e cobertas por ADR-005/ADR-006
no que tange observabilidade e tipos fechados.

**Requisitos transversais (sem ADR próprio, aplicados a toda a entrega)**:
- **RF-08** — `platform_messages` contém a mensagem inbound e a resposta final da pendência: mantido
  pelo `MessageStore.Append` existente no fluxo Thread→Run; verificado por teste de integração que
  assere as duas linhas (inbound + resposta final) após a confirmação, evitando regressão de perda
  de mensagem.
- **RF-26** — gates Go (`go build ./...`, `go vet ./...`, `go test ./... -count=1 -race`,
  `golangci-lint run` quando disponível) executados no escopo alterado, mais os greps de governança
  (R0/R1/R5/R7 e R-ADAPTER-001.1 zero comentários); condição de saída de cada tarefa de implementação.
- **RF-28** — handlers, consumers, jobs e tools permanecem adapters finos delegando a use cases
  (R-ADAPTER-001.2): nenhuma das mudanças acima introduz regra de negócio, SQL direto ou branching de
  domínio em adapter; a query de reconciliação (RF-16) vive no adapter postgres apenas como leitura,
  invocada por use case.

**Decisão de design pattern (design-patterns-mandatory)**: confirmada em todos os clusters — **não
introduzir novo pattern GoF**. Todas as mudanças são refactors locais (inversão de operador/StepStatus,
erro sentinel, campo aditivo, guarda de ramo, método privado na mesma tx, validação de fronteira,
query de auditoria). Nenhuma família de objetos, estratégia intercambiável em runtime ou
desacoplamento estrutural justifica Strategy/Factory/State/CoR novos; os enums fechados existentes já
satisfazem state-as-type.

### Riscos Conhecidos

- **R1 (ADR-002)** — não alterar a chave do ledger `(wamid,item_seq,operation)`: mudá-la quebraria a
  idempotência histórica e exigiria migração. Mitigação: manter a chave; semântica por-pendência via
  `wamid` original.
- **R2 (ADR-003)** — `DecideAllocationsBP` e helpers são compartilhados entre onboarding e
  budget-creation; a mudança afeta os dois — testar ambos os fluxos.
- **R3 (ADR-004)** — `ScoringHooks.AfterTool` precisa parar de descartar `resultBytes` **e** tools com
  erro Go, senão o scorer permanece cego ao pior caso.
- **R4 (ADR-005)** — CHECK em `correlation_key` falha se os 4 runs legados existirem: backfill
  obrigatório antes; validar MessageID quebra fixtures de teste (varrer `*_test.go`).
- **R5 (ADR-006)** — não chamar `LinkChannelToUser` de dentro de `resolvePrincipal` (uow aninhada);
  tratar `ErrUserIdentityAlreadyLinked` como no-op; nunca falhar a jornada por erro de vínculo.
- **R6 (ADR-002)** — confirmar D-10 "1 retry" = 1 tentativa adicional (total 2 escritas):
  `maxFailedWriteResumes=1`.
- **R7 (ADR-005) — RESOLVIDO 2026-07-10**: as colunas de join foram confirmadas no
  `000001_initial_schema.up.sql` — `workflow_runs(correlation_key, status, state JSONB)`;
  `agents_write_ledger.wamid = platform_runs.correlation_key`; `platform_scorer_results.run_id →
  platform_runs.id`; `transactions` via `agents_write_ledger.resource_id`. Sem suposição pendente.

### Conformidade com Padrões

- `.claude/rules/go-adapters.md` (R-ADAPTER-001): adapters finos, zero comentários.
- `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001): tool fina, estados fechados, LLM só nas
  call-sites sancionadas, Run auditável, pending step antes de clarify, cardinalidade controlada.
- `.claude/rules/workflow-kernel.md` (R-WF-KERNEL-001): kernel genérico sem domínio; merge-patch no
  resume (001.7) preservado; scorer não faz IO.
- `.claude/rules/transactions-workflows.md` (R-TXN-004): cardinalidade de métricas.
- `.claude/rules/input-dto-validate.md` (R-DTO-VALIDATE-001): `Validate()` em input DTOs.
- `.claude/rules/go-testing.md` (R-TESTING-001): suites table-driven com mockery.
- `.claude/rules/governance.md`: precedência DMMF state-as-type.

### Arquivos Relevantes e Dependentes

- `internal/budgets/domain/commands/create_budget.go`, `internal/budgets/domain/entities/budget.go`
- `internal/agents/application/workflows/{onboarding_workflow,budget_creation_workflow,budget_creation_decisions,pending_entry_workflow,pending_entry_decisions,pending_entry_state}.go`
- `internal/agents/application/usecases/{register_attempt,idempotent_write,pending_entry_continuer,card_create_confirm_continuer,budget_creation_continuer}.go`
- `internal/agents/application/scorers/{mecontrola_scorers,behavioral_scorers}.go`,
  `internal/agents/application/agents/scoring_hooks.go`
- `internal/platform/agent/{runtime,ports,types}.go`, `internal/platform/scorer/{scorer,types}.go`
- `internal/identity/application/usecases/{establish_principal,link_channel_to_user}.go`,
  `internal/identity/domain/`
- `internal/platform/whatsapp/status/{record_message_status,postgres/repository}.go`
- `internal/agents/infrastructure/postgres/audit_reconciliation.go` (novo)
- `migrations/000008_auth_events_resolve_path.{up,down}.sql` (novo)
