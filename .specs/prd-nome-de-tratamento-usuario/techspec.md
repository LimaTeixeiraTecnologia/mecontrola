<!-- spec-hash-prd: b45c1dbc63fae3ad42064108461db0b6ed1823c3f375ec6e907bf8b30976a904 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Nome de Tratamento do Usuário

## Resumo Executivo

A funcionalidade adiciona um "nome de tratamento" (forma de tratamento afetiva do usuário) capturado no onboarding e alterável por linguagem natural no dia a dia, persistido em `platform_resources` de duas formas complementares: uma chave estruturada `metadata["nome_tratamento"]` (mirror analítico) e uma seção `## Nome de Tratamento` no `working_memory` (conteúdo markdown) — esta última é obrigatória porque o runtime injeta apenas o `working_memory` (conteúdo) no system prompt do agente, nunca o `metadata` JSONB (`internal/platform/agent/runtime.go:304-328`).

A implementação reutiliza integralmente o blueprint já comprovado do fluxo `goal-edit`: (1) no onboarding, o passo no-op `step-welcome` é reaproveitado como passo de captura do nome (mesma estrutura/ordem/IDs do `Sequence`, zero risco de cutover), carregando o nome no estado durável e materializando-o num único ponto de escrita (o passo de conclusão), evitando o clobber de coluna do `working_memory`; (2) no dia a dia, um novo workflow durável dedicado `treatment-name-edit` espelha o `goal-edit` **sem o slot de confirmação sim/não** — aplica imediatamente e confirma, com estados como tipos fechados (DMMF state-as-type), TTL/reaper, idempotência estrutural por run suspenso único, e resume-antes-do-parse via `ResumeDispatcher`. A validação segue o padrão canônico: funções `Decide*` puras com suíte testify, scorers de tom (determinístico + LLM-judged) e gate golden real-LLM ≥ 0,90 por categoria com 0 falso-sucesso.

Referência de decisões: [ADR-001](adr-001-persistencia-nome-tratamento.md), [ADR-002](adr-002-workflow-edicao-nome.md), [ADR-003](adr-003-captura-onboarding-step-welcome.md), [ADR-004](adr-004-gate-aceite-golden-nome.md).

## Arquitetura do Sistema

### Visão Geral dos Componentes

**Novos componentes**

- `workflows/treatment_name_edit_state.go` — `TreatmentNameEditState` + tipo fechado `TreatmentNameEditStatus` (state-as-type).
- `workflows/treatment_name_edit_decisions.go` — funções puras `DecideTreatmentName` (compartilhada com o onboarding) e `DecideTreatmentNameEditExpiry`.
- `workflows/treatment_name_edit_workflow.go` — `BuildTreatmentNameEditWorkflow`, o step, `executeTreatmentNameEdit`, `ContinueTreatmentNameEdit`, `BuildTreatmentNameEditReaper`, `TreatmentNameEditKey`.
- `workflows/working_memory_sections.go` — aliases finos `replaceWorkingMemorySection`/`workingMemorySectionBody`/`parseWorkingMemorySections` que encapsulam os helpers de seção já existentes em `goal_edit_workflow.go` (comportamento genérico por heading), consumidos pelo novo fluxo e pela conclusão do onboarding, sem alterar o `goal-edit`.
- `tools/edit_treatment_name.go` — tool fina `tool.NewTool[EditTreatmentNameInput, EditTreatmentNameOutput]` que delega a `engine.Start` do workflow.
- `golden/cases_treatment_name.go` — casos golden do fluxo (categoria nova `CategoryTreatmentName`).

**Componentes modificados**

- `workflows/onboarding_workflow.go` — `OnboardingState` ganha `TreatmentName`/`TreatmentNameAsked`; `BuildWelcomeStep` vira o passo de captura do nome; `BuildConclusionStep` compõe as duas seções num único `Upsert` e adiciona `nome_tratamento` ao metadata; consts de prompt.
- `agents/mecontrola_agent.go` — `mecontrolaAgentInstructions` ganha orientação para (a) chamar `edit_treatment_name` na intenção de troca e (b) usar o nome de tratamento vigente (seção `## Nome de Tratamento` da working memory) de forma natural, sem excesso.
- `messages/catalog.go` — builders determinísticos de mensagem do fluxo.
- `module.go` — wiring do engine/registry/resumer/reaper/tool do `treatment-name-edit`; inclusão em `WithWriteToolSet`, no `SuspendedRunIndex` e no `ResumeDispatcher`.
- `golden/case.go` + `golden/registry.go` + `golden/registry_test.go` — nova categoria e append do builder de casos.
- `postdeploy/regression_contract.go` — inclusão de `edit_treatment_name`.

### Fluxo de Dados

- Captura (onboarding): `StartOnboarding` → `step-welcome` suspende com boas-vindas + pergunta do nome → resume → `a.Execute` (schema de extração) → `DecideTreatmentName` → `state.TreatmentName` → … → `step-conclusion` grava, num único `Upsert`, `## Nome de Tratamento` + `## Objetivo Financeiro` e faz `UpsertMetadata({objetivo_financeiro, nome_tratamento})`.
- Uso: `runtime.buildMessages` (`runtime.go:308-312`) injeta o `working_memory` no system prompt; o agente lê a seção `## Nome de Tratamento` e trata o usuário pelo nome.
- Edição (dia a dia): inbound → `whatsapp_inbound_consumer.Handle` → `tryDispatchResume` (resume-antes-do-parse) → se não houver run suspenso, `handleAgentInbound` → LLM chama `edit_treatment_name(name?)` → `engine.Start`; com nome, aplica e confirma (1 turno); sem nome, suspende perguntando; a resposta seguinte volta pelo `ResumeDispatcher` → `ContinueTreatmentNameEdit` → aplica e confirma.

## Design de Implementação

### Interfaces Chave

Tool de entrada (adapter fino, delega ao workflow; espelha `tools/edit_goal.go:27-89`, porém com input tipado para satisfazer RF-07 em turno único):

```go
type EditTreatmentNameInput struct {
    Name string `json:"name"`
}

type EditTreatmentNameOutput struct {
    Status  string `json:"status"`
    Message string `json:"message"`
}

func BuildEditTreatmentNameTool(
    engine workflow.Engine[workflows.TreatmentNameEditState],
    def workflow.Definition[workflows.TreatmentNameEditState],
) tool.ToolHandle
```

Continuação (resume-antes-do-parse; espelha `ContinueGoalEdit`, `goal_edit_workflow.go:269-295`):

```go
func ContinueTreatmentNameEdit(
    ctx context.Context,
    engine workflow.Engine[TreatmentNameEditState],
    def workflow.Definition[TreatmentNameEditState],
    key string,
    userMessage string,
) (bool, string, error)
```

Working memory (primitivo de plataforma consumido diretamente, sem re-abstração — R-AGENT-WF-001.8-A; `internal/platform/memory/ports.go:18-22`):

```go
type WorkingMemory interface {
    Get(ctx context.Context, resourceID string) (string, error)
    Upsert(ctx context.Context, resourceID, content string) error
    UpsertMetadata(ctx context.Context, resourceID string, metadata map[string]any) error
}
```

### Modelos de Dados

Estado durável do fluxo de edição (tipo fechado para o ciclo de vida; sem slot `Awaiting` porque o fluxo tem um único slot, ao contrário do `goal-edit` que tem dois):

```go
type TreatmentNameEditStatus int

const (
    TreatmentNameEditActive TreatmentNameEditStatus = iota + 1
    TreatmentNameEditCompleted
    TreatmentNameEditCancelled
    TreatmentNameEditExpired
)

func (s TreatmentNameEditStatus) String() string { /* ... */ }
func (s TreatmentNameEditStatus) IsValid() bool  { return s >= TreatmentNameEditActive && s <= TreatmentNameEditExpired }
func ParseTreatmentNameEditStatus(v string) (TreatmentNameEditStatus, error) { /* sentinel err */ }

type TreatmentNameEditState struct {
    Status        TreatmentNameEditStatus `json:"status"`
    ResourceID    string                  `json:"resourceId"`
    ProvidedName  string                  `json:"providedName"`
    PreviousName  string                  `json:"previousName"`
    NewName       string                  `json:"newName"`
    RepromptCount int                     `json:"repromptCount"`
    MessageID     string                  `json:"messageId"`
    SuspendedAt   time.Time               `json:"suspendedAt"`
    ResumeText    string                  `json:"resumeText"`
    ResponseText  string                  `json:"responseText"`
    Expired       bool                    `json:"expired"`
}
```

Estado do onboarding (adição aditiva, compatível com snapshots suspensos existentes — decodificam como zero value; `onboarding_workflow.go:310-325`):

```go
type OnboardingState struct {
    // ... campos existentes inalterados ...
    TreatmentName      string `json:"treatmentName"`
    TreatmentNameAsked bool   `json:"treatmentNameAsked"`
}
```

Funções puras (sem `ctx`, sem IO; testadas por suíte canônica — `goal_edit_decisions.go:53-59` é o análogo):

```go
const treatmentNameMaxLen = 40

func DecideTreatmentName(hasName bool, raw string) (string, bool) {
    if !hasName {
        return "", false
    }
    trimmed := strings.TrimSpace(raw)
    if trimmed == "" || utf8.RuneCountInString(trimmed) > treatmentNameMaxLen {
        return "", false
    }
    return trimmed, true
}

func DecideTreatmentNameEditExpiry(state TreatmentNameEditState, now time.Time) bool {
    return !state.SuspendedAt.IsZero() && now.Sub(state.SuspendedAt) > treatmentNameEditTTL
}
```

Persistência (colunas já existentes; `migrations/000001_initial_schema.up.sql:2340-2347`):

```
platform_resources(resource_id TEXT PK, working_memory TEXT, metadata JSONB DEFAULT '{}', updated_at)
```

- `working_memory` (conteúdo): passa a conter `## Nome de Tratamento\n\n<nome>\n\n## Objetivo Financeiro\n\n<objetivo>`. O sentinel de onboarding-concluído (`strings.Contains(wm, "## Objetivo Financeiro")`, `resolve_onboarding_or_agent.go:78,119`) permanece válido porque as duas seções vivem na mesma string, escrita por um único writer.
- `metadata` (JSONB merge `metadata || EXCLUDED.metadata`, `working_memory_repository.go:89`): ganha a chave `nome_tratamento`. A edição faz `UpsertMetadata({"nome_tratamento": <novo>})`.

### Endpoints de API

Não há novos endpoints HTTP. A superfície é conversacional (WhatsApp inbound) e workflow durável, consumida pelos adapters existentes.

## Pontos de Integração

- OpenRouter (via `llm.Provider`) para extração estruturada do nome — call-site sancionada (step de workflow chamando `agent.Execute` com `Schema`, como já ocorre em `BuildGoalStep`, `onboarding_workflow.go:1027`). Nenhum LLM no kernel de workflow nem em `Decide*` (R-AGENT-WF-001.4 / R-WF-KERNEL-001.5).
- `platform_resources` via `memory.WorkingMemory` (`Get`/`Upsert`/`UpsertMetadata`) — já implementado (`working_memory_repository.go:25-102`).
- Kernel de workflow (`workflow.Engine[S]` + `workflow.Store` Postgres compartilhado, `module.go:195-215`) — resume por merge-patch (R-WF-KERNEL-001.7).

## Abordagem de Testes

### Testes Unitários

Padrão canônico testify/suite (whitebox `package workflows`, `SetupTest`, tabela `args`/`dependencies`(IIFE)/`expect`, SUT em `s.Run`; ver `.claude/rules/go-testing.md` e `goal_edit_decisions_test.go:10-124`):

- `DecideTreatmentName`: nome direto; "me chama de Stef" (extração via schema no step, aqui testa-se o `Decide` com `hasName/raw`); vazio/recusa → `(",false)`; > 40 chars → `(",false)`; trim de bordas.
- `DecideTreatmentNameEditExpiry`: `SuspendedAt` zero → false; dentro da TTL → false; além da TTL → true (`now` injetado — proibido abstrair tempo; `time.Now().UTC()` só nas call-sites).
- Workflow `treatment-name-edit` (fake `WorkingMemory` + fake `agent.Agent`): nome no `ProvidedName` → aplica e completa (1 turno); sem nome → suspende com o prompt de pergunta; resume com nome usável → `Upsert` da seção + `UpsertMetadata` + `ResponseText` verbatim; resume sem nome usável → reprompt (1×) e depois cancelamento; expiry → `handled=false`.
- Onboarding: `step-welcome` (agora captura) suspende com boas-vindas + pergunta; resume extrai nome; `BuildConclusionStep` compõe as duas seções num único `Upsert` e inclui `nome_tratamento` no metadata; ausência de nome → só `## Objetivo Financeiro` e sem chave `nome_tratamento`.
- `messages`: builders determinísticos (confirmação verbatim com o nome; prompt de pergunta).

### Testes de Integração

Avaliação dos critérios do template: (1) fronteira de IO crítica = `platform_resources` (Postgres) onde o merge de metadata e o overwrite de conteúdo importam — SIM; (2) já há suíte de integração de working memory no repo (`working_memory_repository_integration_test.go`) — SIM. Portanto: estender a suíte de integração existente (`//go:build integration`, testcontainers) para asserir que, pós-onboarding com nome, `SELECT working_memory, metadata->>'nome_tratamento' FROM mecontrola.platform_resources` retorna ambas as seções e a chave; e que a edição substitui a seção preservando `## Objetivo Financeiro`.

### Testes E2E

Gate golden real-LLM (`//go:build integration` + `RUN_REAL_LLM=1` + `OPENROUTER_API_KEY`; `harness_realllm_test.go:24-55`), `goldenGateThreshold = 0.90` por categoria, 3 repetições/caso. Nova categoria `CategoryTreatmentName` com casos:
- "alterar nome com nome informado": input "agora me chama de Stef" → `ExpectedTool: "edit_treatment_name"`, `ExpectedArgs: {name:"Stef"}`.
- "alterar nome sem nome informado": input "quero trocar como você me chama" → `ExpectedTool: "edit_treatment_name"` (sem `name`).
- "confirmação no tom": `ResponseProperty` exige asterisco simples e emoji oficial (reuso de `verbatim_tone_adherence`, `behavioral_scorers.go:307-346`), `Metadata["requires_brand_emoji"]=true`.
Stub `edit_treatment_name` adicionado ao `goldenToolCatalog` (`harness_realllm_test.go:248`). Scorers reutilizados: `tone_adherence` (LLM-judged) + `verbatim_tone_adherence` + `no_hallucination` (guarda de 0 falso-sucesso). Invariante de não-regressão do onboarding validada por unit no estilo `journey_test.go` (composição da working memory sem perder o sentinel).

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. `messages/catalog.go` — builders determinísticos (base para copy verbatim).
2. `workflows/working_memory_sections.go` — aliases de seção (base para conclusão e edição).
3. `workflows/treatment_name_edit_state.go` + `_decisions.go` + testes de suíte — núcleo puro (sem dependências de IO).
4. `workflows/treatment_name_edit_workflow.go` + testes — workflow durável.
5. `onboarding_workflow.go` — estado + `step-welcome` como captura + conclusão compondo seções + testes.
6. `tools/edit_treatment_name.go` + testes — adapter fino.
7. `agents/mecontrola_agent.go` — instruções (chamar tool + usar nome).
8. `module.go` — wiring (engine/registry/resumer/reaper/tool/write-set/index/dispatcher) + `postdeploy/regression_contract.go`.
9. `golden/` — categoria, casos, catálogo de tool, e execução do gate real-LLM.

### Dependências Técnicas

- Infra existente: Postgres (`platform_resources`, `workflow` store), OpenRouter. Nenhuma infra nova.
- Sem feature flag (RF-15): liberação direta; comportamento seguro por padrão (nome opcional).

## Monitoramento e Observabilidade

- Onboarding: counter `agents_onboarding_treatment_name_total{outcome}` com `outcome ∈ {captured, skipped, parse_error}` (espelha `agents_onboarding_monthly_budget_total`, `onboarding_workflow.go:1086-1107`). Cardinalidade controlada — sem `user_id`.
- Edição: reuso do `agents_resume_dispatch_total{workflow="treatment-name-edit", outcome}` e do histograma de duração do `ResumeDispatcher` (`resume_dispatcher.go`); Run auditável aberto/fechado por execução (Thread→Run) com `RunStatus` fechado. Labels permitidos apenas enums fechados (R-AGENT-WF-001.5 / R-TXN-004).
- Logs: erros de `Upsert`/`UpsertMetadata` logados no nível error com `resource_id`, sem PII no corpo além do identificador de recurso já usado.
- KPIs de produto (RF-16): taxa de captura = `captured/(captured+skipped)`; edição bem-sucedida = `outcome=resumed/started` concluídos; aderência de tom via scorers no gate golden.

## Considerações Técnicas

### Decisões Chave

- [ADR-001](adr-001-persistencia-nome-tratamento.md) — persistência dual-write (metadata + seção de conteúdo) com writer único no onboarding e merge de seção na edição.
- [ADR-002](adr-002-workflow-edicao-nome.md) — edição como workflow durável dedicado `treatment-name-edit` sem gate de confirmação, com input de nome opcional na tool para turno único (RF-07).
- [ADR-003](adr-003-captura-onboarding-step-welcome.md) — captura via reaproveitamento do `step-welcome` no-op, carregando o nome no estado e materializando na conclusão (writer único).
- [ADR-004](adr-004-gate-aceite-golden-nome.md) — gate de aceite golden real-LLM ≥ 0,90 por categoria com nova `CategoryTreatmentName` e 0 falso-sucesso.

### Riscos Conhecidos

- **Clobber do `working_memory`**: `Upsert` é overwrite de coluna inteira (`working_memory_repository.go:58-60`). Mitigação (hard): exatamente um writer de `working_memory` no onboarding (a conclusão), compondo todas as seções; na edição, `replaceWorkingMemorySection` preserva as seções irmãs (baseado em `goalEditReplaceSection`, `goal_edit_workflow.go:241-267`).
- **Janela de inconsistência conteúdo↔metadata**: se `Upsert` (conteúdo) tiver sucesso e `UpsertMetadata` falhar, o metadata fica defasado. Como só o conteúdo alimenta o LLM, o comportamento observável permanece correto; a chave é mirror analítico e cicatriza na próxima edição. Ambas as falhas retornam `StepStatusFailed` (RF-13) sem confirmar sucesso.
- **Ordenação do enum `OnboardingPhase`**: não adicionar `PhaseName` no meio do `iota` (renumera snapshots suspensos). A decisão de reaproveitar `step-welcome` mantém `PhaseWelcome` e evita o risco por completo (ADR-003).
- **Um-fluxo-por-recurso**: incluir `treatment-name-edit` no `SuspendedRunIndex` faz um edit de nome suspenso coexistir com o invariante `ErrMultipleSuspendedRuns` (`suspended_run_index.go:36-38`) — comportamento desejado (um fluxo conversacional por vez).

### Conformidade com Padrões

- `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001): roteamento por registry/tool (sem `switch case intent.Kind`); tool fina sem regra/SQL/branching; estados fechados; LLM só nas call-sites sancionadas; Run auditável; resume-antes-do-parse; working memory no system prompt.
- `.claude/rules/workflow-kernel.md` (R-WF-KERNEL-001): sem domínio/SQL/LLM no kernel; resume por merge-patch.
- `.claude/rules/go-adapters.md` (R-ADAPTER-001): zero comentários; adapters finos.
- `.claude/rules/go-testing.md` (R-TESTING-001): suíte canônica testify.
- `.claude/rules/input-dto-validate.md`: `EditTreatmentNameInput.Validate()` (nome opcional; sem IO).
- DMMF (`domain-modeling-production`): `Decide*` puro; state-as-type; sem `Result[T,E]`/currying/DSL.
- Feedbacks de projeto: sem abstração de tempo (`now` injetado nas `Decide*`); sem `var _ Iface = (*T)(nil)`; sem prefixo `_`.

### Arquivos Relevantes e Dependentes

- `internal/agents/application/workflows/goal_edit_workflow.go`, `goal_edit_state.go`, `goal_edit_decisions.go` — blueprint do fluxo de edição e helpers de seção reutilizados.
- `internal/agents/application/workflows/onboarding_workflow.go` — estado, `step-welcome`, `step-conclusion`.
- `internal/agents/application/workflows/correlation_key.go` — `CorrelationKey` para `TreatmentNameEditKey`.
- `internal/agents/application/tools/edit_goal.go` — blueprint da tool fina.
- `internal/agents/application/usecases/resume_dispatcher.go`, `suspended_run_index.go`, `workflow_resumer.go`, `resolve_onboarding_or_agent.go` — roteamento e sentinel.
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go` — ordem resume→onboarding→agent.
- `internal/agents/application/agents/mecontrola_agent.go` — instruções e attach de tools.
- `internal/agents/application/messages/catalog.go` — mensagens determinísticas.
- `internal/agents/application/golden/` — `case.go`, `registry.go`, `registry_test.go`, `harness_realllm_test.go`, `cases_daily_operations.go`.
- `internal/agents/application/scorers/behavioral_scorers.go`, `mecontrola_scorers.go` — scorers de tom.
- `internal/platform/agent/runtime.go` — injeção de working memory no system prompt.
- `internal/platform/memory/ports.go`, `.../postgres/working_memory_repository.go` — primitivo de working memory.
- `internal/agents/module.go` — composição/wiring.
- `internal/agents/application/postdeploy/regression_contract.go` — contrato de regressão de tools.
- `migrations/000001_initial_schema.up.sql` — DDL de `platform_resources`.
