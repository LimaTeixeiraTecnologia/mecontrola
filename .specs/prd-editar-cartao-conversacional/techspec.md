<!-- spec-hash-prd: 2b7b7937760529072f88464a49a36799f7f20c4b27e4b559b456ed26637b3b9d -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Editar Cartão pela Conversa (WhatsApp)

Consome: `.specs/prd-editar-cartao-conversacional/prd.md` (spec-version 1).
ADRs: [adr-001](./adr-001-workflow-dedicado-card-update-confirm.md), [adr-002](./adr-002-optimistic-lock-versao-servidor.md), [adr-003](./adr-003-card-update-state-depara-fix-duedday.md).

## Resumo Executivo

A edição de cartão pela conversa passa a ser um **workflow dedicado durável `card-update-confirm`** no consumidor `internal/agents`, simétrico ao `card-create-confirm` já existente e comprovado. A tool `update_card` deixa de gravar direto e de reusar o workflow compartilhado `destructive-confirm`: ela lê o cartão atual (para montar o de-para e capturar a versão no servidor), monta um estado fechado `CardUpdateState` e inicia o workflow, que confirma com o usuário (HITL universal), revalida a versão no commit via lock otimista atômico, grava de forma idempotente (`operation="update_card"`) e devolve mensagens determinísticas — sem falso sucesso. O gap de versão é fechado expondo `Version` de ponta a ponta e adicionando um `ExpectedVersion` **opcional** ao contrato de update do módulo `internal/card`, mantendo o endpoint REST existente 100% compatível. O defeito de payload de `due_day` é eliminado porque o novo estado carrega explicitamente todos os campos alterados.

O trabalho reusa integralmente o substrato de plataforma (kernel `internal/platform/workflow`, `IdempotentWriter`, `ThreadGateway`, `RunStore`, reaper e store Postgres `platform_*`); nenhum primitivo é reimplementado e nenhum schema novo de agente é criado. O gate de design patterns retornou `reject` (não aplicar padrão GoF novo — reuso de Workflow/Step, State-as-type, Adapter e Factory Function).

## Arquitetura do Sistema

### Visão Geral dos Componentes

Componentes **novos** (consumidor `internal/agents`):
- `application/workflows/card_update_state.go` — `CardUpdateStatus` (tipo fechado) e `CardUpdateState`.
- `application/workflows/card_update_decisions.go` — `DecideCardUpdateConfirmation` (pura), TTL/reprompt, `buildCardUpdateQuestion` (de-para). Reusa o tipo `CardConfirmAction` existente.
- `application/workflows/card_update_confirm_workflow.go` — `BuildCardUpdateConfirmWorkflow`, step de avaliação, `executeUpdateCard` (idempotente), `isCardUpdateDomainError`, `cardUpdateDomainErrorMessage`.
- `application/usecases/card_update_confirm_continuer.go` — `CardUpdateConfirmContinuer` (resume + Run auditável), análogo ao de criação.
- `infrastructure/jobs/handlers/card_update_reaper_job.go` — reaper do workflow de edição.

Componentes **modificados**:
- `application/tools/update_card.go` — reescrita: lê cartão atual, monta `CardUpdateState`, inicia workflow; remove gravação direta, remove uso de `ConfirmState`/`destructive-confirm`, remove `version` do schema de entrada.
- `application/interfaces/types.go` — `Card` ganha `Version int64`; `CardUpdate` ganha `ExpectedVersion *int64` e `ClosingDay *int` (RF-17).
- `infrastructure/binding/card_manager_adapter.go` — `mapCardOutput` propaga `Version`; `UpdateCard` propaga `ExpectedVersion` e `ClosingDay`.
- `application/workflows/confirm_state.go` — remove a constante final `OpUpdateCard` (última do enum; sem reordenar as demais).
- `application/workflows/destructive_confirm_workflow.go` — remove `OpUpdateCard` do `buildExecMap`, `executeUpdateCard` e o case de `successMessage`.
- `module.go` — wiring do engine/def/continuer/reaper de `card-update-confirm`; `update_card` deixa de receber `confirmEngine/confirmDef`.
- `infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go` — insere `tryContinueCardUpdate` na cadeia de resume, imediatamente após `tryContinueCardCreate`.
- `application/agents/mecontrola_agent.go` — instrução de `update_card` alinhada ao padrão de `create_card` (repasse verbatim de `clarifyPrompt`/`confirmationPrompt`; nunca afirmar atualização sem retorno do sistema).
- `application/golden/cases_card.go` e `application/golden/harness_realllm_test.go` — casos golden de edição e schema de captura de `update_card` atualizado.

Componentes **modificados** (módulo de suporte `internal/card`):
- `application/dtos/output/card.go` — `Card` ganha `Version int64`.
- `application/mappers/card_mapper.go` — `ToCardOutput` mapeia `Version`.
- `application/dtos/input/update_card.go` — `UpdateCard` ganha `ExpectedVersion *int64` e `ClosingDay *int` (opcionais); `Validate()` valida `ClosingDay` em 1..31 quando presente.
- `application/usecases/update_card.go` — revalida `ExpectedVersion` contra a versão corrente antes de gravar; propaga ao repositório; em `resolveUpdate`, quando `ClosingDay != nil` constrói o ciclo direto via `NewBillingCycle(closingDay, dueDay)` sem chamar `DaysBeforeDue` (RF-17; evita o `fallbackDaysBeforeDue` para banco não reconhecido).
- `application/interfaces/repository.go` e `infrastructure/repositories/postgres/card_repository.go` — `UpdateByIDForUser` passa a aceitar `expectedVersion *int64` e, quando presente, aplica lock otimista atômico (`AND version = $x`), desambiguando 0 linhas em `ErrCardVersionConflict` vs `ErrCardNotFound`.
- `domain/errors.go` — sentinela `ErrCardVersionConflict`.

### Fluxo de Dados (inbound de edição)

```
WhatsApp inbound
 -> whatsapp_inbound_consumer.tryResumeChain
      [pendingEntry -> destructive -> cardCreate -> cardUpdate(NOVO) -> budgetCreation -> onboarding]
 -> (sem resume pendente) handleAgentInbound -> AgentRuntime.Execute -> loop tool-calling
      -> resolve_card/list_cards/get_card  (identificação + version)
      -> update_card.exec:
           cards.GetCard(cardID,userID) -> atual (valores + Version)
           calcula diffs; se banco novo nao reconhecido e sem closingDay -> needs_closing
           monta CardUpdateState (ExpectedVersion=atual.Version, valores atual+novo)
           engine.Start(card-update-confirm) -> suspende com pergunta de-para
 -> usuário responde "sim"/"não"
 -> whatsapp_inbound_consumer.tryContinueCardUpdate -> CardUpdateConfirmContinuer.Continue
      -> abre Run auditável -> engine.Resume(merge-patch {resumeText, incomingMessageId})
      -> DecideCardUpdateConfirmation -> Accept: executeUpdateCard (idempotente + revalida versão)
      -> mensagem determinística
```

## Design de Implementação

### Interfaces Chave

Estado fechado e decisão pura (novo):

```go
type CardUpdateStatus int

const (
	CardUpdateStatusActive CardUpdateStatus = iota + 1
	CardUpdateStatusCompleted
	CardUpdateStatusCancelled
	CardUpdateStatusExpired
)

type CardUpdateState struct {
	Status             CardUpdateStatus
	Awaiting           AwaitingKind
	UserID             uuid.UUID
	CardID             uuid.UUID
	ExpectedVersion    int64
	CurrentNickname    string
	CurrentBank        string
	CurrentDueDay      int
	NewNickname        *string
	NewBank            *string
	NewDueDay          *int
	NewClosingDay      *int
	MessageID          string
	IncomingMessageID  string
	ProcessedMessageID string
	ConfirmReprompt    int
	SuspendedAt        time.Time
	ResumeText         string
	ResponseText       string
	Expired            bool
}

func DecideCardUpdateConfirmation(state CardUpdateState, msg PendingMessage, now time.Time) CardConfirmAction
```

Workflow (novo, assinatura simétrica ao create):

```go
const CardUpdateConfirmWorkflowID = "card-update-confirm"

func CardUpdateKey(resourceID string) string { return resourceID + ":card-update" }

func BuildCardUpdateConfirmWorkflow(idem IdempotentWriter, cards interfaces.CardManager) workflow.Definition[CardUpdateState]
```

Escrita idempotente (reuso do contrato existente, `operation="update_card"`):

```go
idem.Execute(ctx, state.UserID, state.MessageID, 0, "update_card", "card", writeFn, isCardUpdateDomainError)
```

Contrato de update do módulo card (evolução aditiva — ADR-002):

```go
type UpdateCard struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	Nickname        *string
	Bank            *string
	DueDay          *int
	ExpectedVersion *int64
}

func (r *cardRepository) UpdateByIDForUser(ctx context.Context, c entities.Card, expectedVersion *int64) (entities.Card, error)
```

Regra: quando `expectedVersion != nil`, a query aplica `AND version = $expected`; em 0 linhas afetadas, o repositório reexecuta uma verificação de existência para retornar `ErrCardVersionConflict` (existe, versão divergente) ou `ErrCardNotFound` (não existe / soft-deleted). Quando `expectedVersion == nil` (caminho REST atual), o comportamento é idêntico ao de hoje.

### Modelos de Dados

- Persistência do workflow: reusa a tabela `platform_workflow_*` (migration 000003) via `workflow.Store`. **Nenhuma migration nova é necessária** para o workflow.
- `CardUpdateState` é serializado como `Snapshot.State` (JSON) pelo kernel; o resume aplica `Codec.MergePatch` sobre esse JSON antes do parse.
- Idempotência: reusa o write ledger existente (`WriteLedgerEntry{UserID, WAMID, ItemSeq, Operation="update_card", ResourceKind="card", ResourceID}`) — sem schema novo.
- Coluna `version` da tabela `mecontrola.cards` já existe (migration 000001); a mudança é apenas expor/consumir esse valor. **Nenhuma migration nova** no módulo card.

### Endpoints de API

Nenhum endpoint novo. O endpoint REST `PUT /api/v1/cards/{id}` do módulo card permanece inalterado em contrato e comportamento (não envia `ExpectedVersion`, portanto não sofre lock otimista — compatibilidade total, ADR-002).

## Pontos de Integração

- Provider LLM: OpenRouter via `internal/platform/llm` (inalterado; a edição não adiciona nova call-site de LLM — o `exec` da tool e o workflow são determinísticos).
- Persistência: Postgres via `workflow.Store` (`platform_*`), write ledger e módulo card (uow + repositório).
- WhatsApp inbound: consumidor existente; apenas um novo elo na cadeia de resume.

## Abordagem de Testes

### Testes Unitários

Padrão canônico testify/suite (whitebox, `fake.NewProvider()`, mocks via `.mockery.yml`, IIFE por mock), espelhando `card_create_confirm_workflow_test.go`:
- `card_update_decisions_test.go` — `DecideCardUpdateConfirmation` puro: Accept (sim/variações), Cancel (não/variações), Reprompt (1ª ambígua), Cancel após reprompt (2ª ambígua), Expire (>15min), Replay (mesmo `ProcessedMessageID`). Sem mock.
- `card_update_confirm_workflow_test.go` — suite com `wfStore` in-memory, `NewEngine[CardUpdateState]`, `BuildCardUpdateConfirmWorkflow(fakeIdem, cardsMock)`; cenários: suspende com de-para; aceita e grava; cancela; ambíguo re-pergunta; expira; replay; sucesso; replay idempotente; erros de domínio (apelido em uso, vencimento inválido, não encontrado, **versão divergente**); falha transitória → `StepStatusFailed` sem falso sucesso.
- `update_card_test.go` (tool) — needs_closing (banco novo não reconhecido sem fechamento); no_changes (nada difere); not_found (GetCard falha); needs_confirmation (monta estado + de-para); pending_confirmation_exists (`ErrRunAlreadyExists`); identidade inválida.
- Módulo card: `update_card_test.go` (use case) — `ExpectedVersion` divergente → `ErrCardVersionConflict`; `ExpectedVersion` igual → grava; `ExpectedVersion` nil → comportamento atual.

### Testes de Integração

Necessário: sim (fronteira Postgres crítica; resume durável e lock otimista atômico não são garantidos por mock). Usar `testcontainers-go` com build tag `//go:build integration`, espelhando `card_create_confirm_workflow_integration_test.go`:
- `card_update_confirm_workflow_integration_test.go` — módulo card real + `NewPostgresStore` + `NewIdempotentWrite` + `WriteLedgerRepository`: start → suspend → resume "sim" → cartão atualizado no Postgres; replay do mesmo `wamid` não aplica duas vezes; conflito de versão real (segunda edição concorrente altera a versão entre start e commit → `ErrCardVersionConflict` → mensagem determinística).
- `internal/card` — teste de integração do repositório `UpdateByIDForUser` com `expectedVersion` (0 linhas → `ErrCardVersionConflict`; existência confirmada; caminho nil inalterado).

### Testes E2E

Golden/real-LLM (`internal/agents/application/golden`, `CategoryCard`, gate ≥ 0,90 por categoria, `RUN_REAL_LLM=1`):
- Adicionar em `cases_card.go`: editar apelido (resolve_card → update_card); alterar vencimento (resolve_card → update_card, resposta contém aviso de impacto); apelido não encontrado em edição (`resolve_card` found=false → oferecer lista); cancelar edição (prior turn + "não" → sem tool, resposta de cancelamento); banco não reconhecido em edição pede fechamento.
- Atualizar o schema da capture tool `update_card` no harness para `{cardId, nickname, bank, dueDay, closingDay}` (sem `version`).
- Não usar `ExpectedOutcome` com valor inexistente no enum `agent.ToolOutcome`; asserir seleção de tool (`ExpectedTools`) e propriedade de resposta (`ResponseProperty`).

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Módulo card — exposição de versão e lock otimista** (ADR-002): `ErrCardVersionConflict`; `cardoutput.Card.Version` + mapper; `cardinput.UpdateCard.ExpectedVersion`; use case revalida; `UpdateByIDForUser(expectedVersion)` + testes. Base para todo o resto.
2. **Binding + interfaces do agente**: `interfaces.Card.Version`, `interfaces.CardUpdate.ExpectedVersion`, `mapCardOutput`/`UpdateCard` do adapter + testes.
3. **Estado e decisão** (ADR-003): `card_update_state.go`, `card_update_decisions.go` (de-para) + testes puros.
4. **Workflow** (ADR-001): `card_update_confirm_workflow.go` (idempotência + revalidação + classificação de erro) + testes unit/integration.
5. **Tool `update_card` reescrita**: lê cartão atual, monta estado, inicia workflow; remove gravação direta e uso do destructive; ajusta schema (sem `version`) + testes.
6. **Continuer + reaper + wiring**: `card_update_confirm_continuer.go`, `card_update_reaper_job.go`, `module.go`, `whatsapp_inbound_consumer.go`.
7. **Limpeza**: remover `OpUpdateCard` de `confirm_state.go`/`destructive_confirm_workflow.go`.
8. **Prompt + golden**: instrução de `update_card` no `mecontrola_agent.go`; casos golden + schema de captura; rodar gate real-LLM.

### Dependências Técnicas

- Postgres com `platform_workflow_*` (migration 000003) e `mecontrola.cards` (migration 000001) já aplicadas.
- Nenhuma migration nova.

## Monitoramento e Observabilidade

- Métrica nova `agents_card_update_confirm_total` com rótulo `outcome` (enum fechado: `replied`, `completed`, `cancelled`, `expired`, `error`, `unknown`) — espelha `agents_card_create_confirm_total`. Cardinalidade controlada: proibido `user_id`/`card_id`/`resource_id` como rótulo (R-WF-KERNEL-001.4 / R-TXN-004).
- Run auditável por execução (thread_id, run_id, workflow=`card-update-confirm`, status, duração, erro) via `RunStore` — RF-27.
- Reaper `agents-card-update-reaper` (cron `*/5 * * * *`, TTL 15min, batch 100) purga runs suspensos — RF-31.
- Logs de erro sem PII; classificação de outcome pesquisável no trace.

## Considerações Técnicas

### Decisões Chave

- **ADR-001 — Workflow dedicado `card-update-confirm` simétrico ao create.** Em vez de estender o `destructive-confirm` compartilhado, um workflow próprio isola a semântica de edição (confirmação universal, de-para, idempotência `operation="update_card"`, TTL 15min, reaper, no-false-success). Ver [adr-001](./adr-001-workflow-dedicado-card-update-confirm.md).
- **ADR-002 — Optimistic lock com versão gerenciada no servidor.** Expor `Version` de ponta a ponta e adicionar `ExpectedVersion` **opcional** ao update do módulo card, com lock atômico no repositório; o agente captura a versão no início da confirmação e a revalida no commit; o LLM nunca lida com versão; REST permanece compatível. Ver [adr-002](./adr-002-optimistic-lock-versao-servidor.md).
- **ADR-003 — `CardUpdateState` dedicado com de-para e correção do payload de `due_day`.** Estado fechado carrega valores atuais (de-para) e todos os campos novos, eliminando o defeito de `due_day` omitido do payload atual. Ver [adr-003](./adr-003-card-update-state-depara-fix-duedday.md).
- **Design patterns (gate obrigatório):** seletor determinístico retornou `reject` — não aplicar padrão GoF novo; usar solução direta reusando Workflow/Step (comportamental), State-as-type, Adapter (tool/binding) e Factory Function (`Build*`) já presentes. Justificativa: `prefer_direct_solution` + `low_change_frequency` + `minimize_class_count` — introduzir novo pattern aumentaria indireção sem ganho.

### Mapa Requisito → Decisão → Teste

| RF | Decisão/onde | Teste |
|---|---|---|
| RF-01..RF-04 | identificação por `resolve_card`/`list_cards`/`get_card`; tool nunca inventa id | golden edição; unit tool |
| RF-05 | `CardUpdateState` aceita nickname/bank/dueDay simultâneos | unit workflow multi-campo |
| RF-06 | `version` removido do schema da tool; capturado via `GetCard` (ADR-002) | unit tool; unit use case |
| RF-07 | closing derivado; sem limite | unit (sem campo) |
| RF-08 | confirmação sempre inicia workflow (remove gravação direta) | unit tool; unit workflow |
| RF-09 | estado persistido antes da pergunta (suspend durável) | integration resume |
| RF-10 | `buildCardUpdateQuestion` de-para + nota de impacto (ADR-003) | unit decisions |
| RF-11..RF-13 | `DecideCardUpdateConfirmation` (accept/cancel/reprompt) | unit decisions |
| RF-14 | TTL 15min + expire | unit decisions; reaper |
| RF-15 | `ErrRunAlreadyExists` → pending | unit tool |
| RF-16 | recálculo de fechamento ao mudar banco reconhecido (`DaysBeforeDue`) | unit use case |
| RF-17 | `ClosingDay` plumbado a `cardinput.UpdateCard`/`interfaces.CardUpdate`; branch em `resolveUpdate` usa valor informado | unit `resolveUpdate`; golden |
| RF-18 | payload com `due_day` completo (ADR-003); novo vencimento persistido + fechamento recalculado | integration; unit use case |
| RF-19 | permitir vencimento com parcelas em aberto (aviso) | unit workflow; golden |
| RF-20 | captura+revalidação de versão (ADR-002) | unit + integration conflito |
| RF-21..RF-24 | idempotência, mensagens determinísticas, no-false-success | unit + integration replay/falha |
| RF-25..RF-26 | ownership por identidade; soft-deleted → not found | unit; integration |
| RF-27..RF-31 | Run auditável, métrica enum-only, reaper | integration; revisão de wiring |
| RF-32 | casos golden edição, gate ≥0,90/categoria | golden real-LLM |

### Riscos Conhecidos

- **Janela TOCTOU entre `GetCard` (início da confirmação) e o commit (15 min depois).** Mitigação: o lock otimista atômico no repositório (`AND version = $expected`) revalida no exato momento do UPDATE; qualquer mudança concorrente (REST ou outra thread) resulta em `ErrCardVersionConflict` determinístico, nunca sobrescrita silenciosa.
- **Remoção de `OpUpdateCard` do enum compartilhado.** `OpUpdateCard` é a última constante do `OperationKind` (valor 7); removê-la não reordena as demais. Mitigação: remover em todos os sites de `confirm_state.go` (constante, `String()`, `ParseOperationKind`, e limite superior de `IsValid()` que passa a `OpDeleteRecurrence`) e de `destructive_confirm_workflow.go` (`buildExecMap`, `executeUpdateCard`, `successMessage`); as 6 operações remanescentes permanecem; cobrir com build/vet e testes do `destructive_confirm_workflow`.
- **Regressão do endpoint REST.** Mitigação: `ExpectedVersion` é opcional (`*int64`); caminho nil mantém a query e o comportamento atuais; teste de integração do repositório cobre os dois caminhos.
- **Brittleness do gate real-LLM** (histórico do repo). Mitigação: casos golden dirigem por seleção de tool e propriedade semântica de resposta, sem depender de string exata frágil; não baixar a régua de 0,90.

### Conformidade com Padrões

- `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001): comportamento novo como workflow/tool no consumidor; tool fina delega a usecase; estados fechados; LLM só nas call-sites sancionadas; Run auditável; pending step salvo antes de pedir confirmação; resume por merge-patch antes do parse.
- `.claude/rules/workflow-kernel.md` (R-WF-KERNEL-001): kernel intocado; sem domínio/SQL/LLM no kernel; SQL só no adapter Postgres do módulo card; métricas enum-only.
- `.claude/rules/go-adapters.md` (R-ADAPTER-001): zero comentários em `.go`; adapters finos; sem SQL em tool/consumer.
- `.claude/rules/input-dto-validate.md` (R-DTO-VALIDATE-001): `cardinput.UpdateCard.Validate()` mantido; `ExpectedVersion` opcional não exige validação semântica adicional.
- `.claude/rules/go-testing.md` (R-TESTING-001): testify/suite whitebox, `fake.NewProvider()`, IIFE por mock.
- DMMF/domain-modeling-production: `Decide*` puro (`DecideCardUpdateConfirmation`, `DecideUpdate`); validação só em smart constructors; state-as-type (`CardUpdateStatus`, `AwaitingKind`, `CardConfirmAction`); `ErrCardVersionConflict` como erro de domínio explícito.

### Arquivos Relevantes e Dependentes

Novos: `internal/agents/application/workflows/card_update_state.go`, `card_update_decisions.go`, `card_update_confirm_workflow.go`; `internal/agents/application/usecases/card_update_confirm_continuer.go`; `internal/agents/infrastructure/jobs/handlers/card_update_reaper_job.go`.

Modificados (agente): `application/tools/update_card.go`, `application/interfaces/types.go`, `infrastructure/binding/card_manager_adapter.go`, `application/workflows/confirm_state.go`, `application/workflows/destructive_confirm_workflow.go`, `module.go`, `infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`, `application/agents/mecontrola_agent.go`, `application/golden/cases_card.go`, `application/golden/harness_realllm_test.go`.

Modificados (módulo card): `application/dtos/output/card.go`, `application/mappers/card_mapper.go`, `application/dtos/input/update_card.go`, `application/usecases/update_card.go`, `application/interfaces/repository.go`, `infrastructure/repositories/postgres/card_repository.go`, `domain/errors.go`, e mocks regenerados (`.mockery.yml` + `task mocks`).
