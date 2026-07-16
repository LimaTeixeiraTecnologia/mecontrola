<!-- spec-hash-prd: 7d6202635134d884361f8109f89e510aa45fe46184c15f6744f3a017437bba03 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Recorrência do Orçamento por Linguagem Natural no Onboarding

PRD: `.specs/prd-recorrencia-orcamento-onboarding/prd.md` (spec-version 2, RF-01..RF-20).

## Resumo Executivo

A mudança é aditiva e confinada a um único arquivo de produção: `internal/agents/application/workflows/onboarding_workflow.go` (o step `BuildRecurrenceStep`). Toda a capacidade de replicar o orçamento por 1 a 12 meses já existe ponta a ponta no módulo `internal/budgets` — a interface `BudgetPlanner.CreateRecurrence(...months int)`, o adapter de binding, o DTO de input (validação 1–12), o comando de domínio (`minRecurrenceMonths=1`/`maxRecurrenceMonths=12`) e o usecase (`for range cmd.Months`) — e **nenhum desses componentes muda**. O único hardcode que impede escolher N é o call-site `onboarding_workflow.go:1548` (`CreateRecurrence(ctx, userUUID, competence, 12)`).

A solução segue os padrões DMMF já vigentes no próprio arquivo: (1) uma decisão pura `DecideRecurrence` sem IO que resolve a intenção extraída + quantidade em um resultado tipado; (2) dois tipos-estado fechados (`recurrenceIntentKind` e `recurrenceOutcomeKind`), espelhando `distributionIntentKind`; (3) uma extração LLM estruturada única por turno (um `agent.Execute`), preservando a contagem de chamadas dos testes full-flow; (4) confirmação encadeada via campo de estado, espelhando o padrão `GoalConfirmation`; (5) um contador de outcome com cardinalidade controlada, espelhando `agents_onboarding_distribution_total`. O gate de qualidade estende o suite real-LLM dedicado ao step (`TestRecurrenceExtractionGate`), não o registry golden (que cobre tools do agente diário, não steps do onboarding). A liberação é direta, sem feature flag (RF-19), e onboardings suspensos retomam de forma retrocompatível (RF-20) porque o estado é JSON aditivo.

## Arquitetura do Sistema

### Visão Geral dos Componentes

Todos os componentes novos/modificados vivem em `internal/agents/application/workflows/onboarding_workflow.go` (produção) e seus testes.

Componentes **novos**:
- `recurrenceIntentKind` — enum fechado da intenção extraída pelo LLM: `recurrenceIntentNegative | recurrenceIntentPositive | recurrenceIntentUnclear`. Métodos `String()`, `IsValid()`, `ParseRecurrenceIntentKind(string)`. Espelha `distributionIntentKind` (`onboarding_workflow.go:184-222`).
- `recurrenceOutcomeKind` — enum fechado da decisão resolvida, que também é a fonte do rótulo de métrica: `recurrenceOutcomeNone | recurrenceOutcomeDefault | recurrenceOutcomeSpecific | recurrenceOutcomeInvalid | recurrenceOutcomeAmbiguous`. `String()` retorna exatamente os valores de rótulo do RF-16 (`no_recurrence`, `default_12`, `specific_months`, `invalid_reprompt`, `ambiguous_reprompt`); `IsValid()`.
- `recurrenceDecision` — struct de resultado da decisão: `{ Outcome recurrenceOutcomeKind; Months int }`.
- `DecideRecurrence(intent, hasMonths, months)` — função pura sem IO/contexto (padrão `DecideMonthlyBudgetCents` `:342`, `DecideGoalValueCents` `:335`).
- `recurrenceDecisionSchema` + `recurrenceExtract` — schema/struct de saída estruturada dedicados ao step de recorrência (NÃO reutilizar o `recurrenceSchema` atual, compartilhado com `summary_confirm` — ver Riscos).
- `newRecurrenceOutcomeCounter` / `recordRecurrenceOutcome` / const `recurrenceOutcomeMetric = "agents_onboarding_recurrence_total"` — espelham o par de `agents_onboarding_distribution_total` (`:1258-1283`).
- Constantes de copy (Tom de Voz): novo `conclusionRecurrencePrompt` (RF-14), `recurrenceInvalidReprompt` (RF-07), confirmações `recurrenceConfirmationNone`/`recurrenceConfirmationDefault` e helper `recurrenceConfirmationFor(months)`/`monthsLabel(n)`.

Componentes **modificados**:
- `OnboardingState` (`:310-325`) — dois campos aditivos: `RecurrenceMonths int json:"recurrenceMonths"` e `RecurrenceConfirmation string json:"recurrenceConfirmation"`. O campo `Recurrence bool` é preservado.
- `BuildRecurrenceStep` (`:1517-1554`) — nova assinatura `BuildRecurrenceStep(a agent.Agent, budgets interfaces.BudgetPlanner, rec observability.Counter)`; lógica reescrita para extração única → `DecideRecurrence` → aplicar/confirmar/repergunta.
- `recurrenceSystemPrompt` (`:821`) — reescrito para extrair intent + hasMonths + months com conversão por extenso (estilo `distributionIntentSystemPrompt` `:789-796`).
- `recurrenceSummaryLine` (`:966-971`) — assinatura `recurrenceSummaryLine(recurrence bool, months int)`; reflete N; fallback legado `months<=0 && recurrence → 12` (RF-20).
- `conclusionSummaryMessage` (`:973-991`) — passa `state.RecurrenceMonths` para `recurrenceSummaryLine`.
- `BuildCardsStep` (`:1149-1161`) — prefixa `state.RecurrenceConfirmation` no prompt inicial e zera o campo, espelhando `BuildMonthlyBudgetStep` (`:1113-1118`).
- `BuildOnboardingWorkflow` (`:1637-1651`) — cria `rec` no bloco `if o11y != nil` e passa a `BuildRecurrenceStep`.

Componentes **inalterados** (evidência de 0 regressão): todo `internal/budgets` (interface, adapter, DTO, comando, usecase), `postdeploy/regression_contract.go` (o `OnboardingWorkflowID` e os scorers não mudam — nenhum scorer novo é introduzido), o kernel `internal/platform/workflow`, e o `recurrenceSchema`/`yesNoExtract` usados por `summary_confirm`.

### Fluxo de Dados (step de recorrência, por turno)

1. Primeira entrada (`ResumeText == ""`): `state.Phase = PhaseRecurrence`; `suspendStep(state, conclusionRecurrencePrompt)`.
2. Resposta do usuário (`ResumeText != ""`): **um** `agent.Execute` com `recurrenceDecisionSchema` → `recurrenceExtract{Intent, HasMonths, Months}`.
3. `ParseRecurrenceIntentKind(extract.Intent)` → `DecideRecurrence(intent, extract.HasMonths, extract.Months)` → `recurrenceDecision`.
4. Despacho por `decision.Outcome` (mapa de decisão, sem branching de domínio no adapter):
   - `Invalid` → `recordRecurrenceOutcome(invalid_reprompt)`; `suspendStep(recurrenceInvalidReprompt)`; **não** chama `CreateRecurrence`.
   - `Ambiguous` → `recordRecurrenceOutcome(ambiguous_reprompt)`; `suspendStep(conclusionRecurrencePrompt)`; **não** chama `CreateRecurrence`.

RF-09 (repergunta sem limite): os desfechos `Invalid`/`Ambiguous` retornam `suspendStep` sem avançar a fase nem manter contador; no próximo resume o step re-extrai e re-decide, reperguntando indefinidamente até uma intenção válida — sem cap novo. O abandono permanece coberto pelo reaper existente `BuildOnboardingReaper` (`OnboardingStaleAfter = 7*24h`, `:39,:1660`), inalterado.
   - `None` → `state.RecurrenceConfirmation = recurrenceConfirmationNone`; `recordRecurrenceOutcome(no_recurrence)`; `completeStep`; **não** chama `CreateRecurrence`.
   - `Default`/`Specific` → resolve `competence` (America/Sao_Paulo + fallback UTC via `competenceLocation`), `budgets.CreateRecurrence(ctx, userUUID, competence, decision.Months)`; em sucesso: `state.Recurrence = true`, `state.RecurrenceMonths = decision.Months`, `state.RecurrenceConfirmation = recurrenceConfirmationFor(decision.Months)`, `recordRecurrenceOutcome(decision.Outcome.String())`, `completeStep`.
5. No step seguinte (`BuildCardsStep`), `state.RecurrenceConfirmation` é prefixado ao prompt e zerado.
6. No `BuildConclusionStep`, o resumo usa `state.Recurrence` + `state.RecurrenceMonths`.

## Design de Implementação

### Interfaces Chave

Nenhuma interface pública nova. A porta consumida permanece a existente (inalterada):

```go
type BudgetPlanner interface {
    CreateRecurrence(ctx context.Context, userID uuid.UUID, competence string, months int) error
}
```

Evidência de suporte 1–12 ponta a ponta (não muda): `internal/agents/application/interfaces/budget_planner.go:13`; `internal/agents/infrastructure/binding/budget_planner_adapter.go:120-134`; `internal/budgets/application/dtos/input/create_recurrence_input.go:23-24` (`Months < 1 || Months > 12`); `internal/budgets/domain/commands/create_recurrence.go:11-14,39-41`; `internal/budgets/application/usecases/create_recurrence.go:65-70`.

### Modelos de Dados

Tipos-estado fechados (DMMF state-as-type; nunca string livre em assinatura):

```go
type recurrenceIntentKind int

const (
    recurrenceIntentNegative recurrenceIntentKind = iota + 1
    recurrenceIntentPositive
    recurrenceIntentUnclear
)

type recurrenceOutcomeKind int

const (
    recurrenceOutcomeNone recurrenceOutcomeKind = iota + 1
    recurrenceOutcomeDefault
    recurrenceOutcomeSpecific
    recurrenceOutcomeInvalid
    recurrenceOutcomeAmbiguous
)

type recurrenceDecision struct {
    Outcome recurrenceOutcomeKind
    Months  int
}
```

Decisão pura (regra de negócio da resolução vive só aqui; sem IO, determinística; testável sem mock):

```go
const (
    recurrenceDefaultMonths = 12
    recurrenceMinMonths     = 1
    recurrenceMaxMonths     = 12
)

func DecideRecurrence(intent recurrenceIntentKind, hasMonths bool, months int) recurrenceDecision {
    if hasMonths {
        if months >= recurrenceMinMonths && months <= recurrenceMaxMonths {
            return recurrenceDecision{Outcome: recurrenceOutcomeSpecific, Months: months}
        }
        return recurrenceDecision{Outcome: recurrenceOutcomeInvalid}
    }
    switch intent {
    case recurrenceIntentPositive:
        return recurrenceDecision{Outcome: recurrenceOutcomeDefault, Months: recurrenceDefaultMonths}
    case recurrenceIntentNegative:
        return recurrenceDecision{Outcome: recurrenceOutcomeNone}
    default:
        return recurrenceDecision{Outcome: recurrenceOutcomeAmbiguous}
    }
}
```

Prioridade (RF-06): uma quantidade válida (`hasMonths` + 1–12) **sempre** prevalece — resolve para `Specific` independentemente da intenção (mesma precedência do budget review, onde número presente vence "não"). Quantidade fora de 1–12 resolve para `Invalid` (RF-07), inclusive quando a intenção parece positiva. Sem quantidade: positiva→`Default`, negativa→`None`, `unclear`/intenção não parseável→`Ambiguous` (RF-08).

Schema de saída estruturada dedicado (novo — não tocar no `recurrenceSchema` compartilhado):

```go
type recurrenceExtract struct {
    Intent    string `json:"intent"`
    HasMonths bool   `json:"hasMonths"`
    Months    int    `json:"months"`
}

var recurrenceDecisionSchema = map[string]any{
    "type": "object",
    "properties": map[string]any{
        "intent":    map[string]any{"type": "string", "enum": []any{"negative", "positive", "unclear"}},
        "hasMonths": map[string]any{"type": "boolean"},
        "months":    map[string]any{"type": "integer"},
    },
    "required":             []any{"intent", "hasMonths", "months"},
    "additionalProperties": false,
}
```

Campos aditivos no estado (retrocompatíveis — ausência desserializa para zero-value; RF-13/RF-20):

```go
type OnboardingState struct {
    // ... campos existentes preservados, incluindo Recurrence bool `json:"recurrence"`
    RecurrenceMonths      int    `json:"recurrenceMonths"`
    RecurrenceConfirmation string `json:"recurrenceConfirmation"`
}
```

### Endpoints de API

Não aplicável — a mudança é interna ao workflow de onboarding conversacional (WhatsApp); não há endpoint HTTP novo.

## Pontos de Integração

- **OpenRouter (via `agent.Agent`)**: a extração de linguagem natural é uma call-site sancionada (loop/parse do agente), consistente com os demais steps do onboarding. Provider único; sem fallback chain. Tratamento de erro: falha de `agent.Execute`/`json.Unmarshal` → `failStep` com wrapping `fmt.Errorf("agents.onboarding.recurrence: ...: %w", err)` (padrão vigente `:1533-1537`).
- **Módulo `internal/budgets`** (via `BudgetPlanner` binding): aplicação da recorrência; contrato inalterado; replay idempotente por estado (retorna `conflict`/`updated` sem erro — `internal/budgets/application/usecases/create_recurrence.go:156-158,198-202`), garantindo 0 regressão no resume durável.

## Abordagem de Testes

### Testes Unitários

Arquivo: `internal/agents/application/workflows/onboarding_workflow_test.go` (whitebox `package workflows`, testify/suite, mocks mockery — padrão vigente).

Decisão pura e enums (sem mock):
- `DecideRecurrence` table-driven cobrindo: negativa→`None`; positiva sem meses→`Default`(12); `hasMonths` com 1, 3, 12→`Specific`; fronteiras 0 e 13 e 24→`Invalid`; `unclear` sem meses→`Ambiguous`; precedência: negativa + meses válidos→`Specific`; positiva + meses inválidos→`Invalid`.
- `ParseRecurrenceIntentKind` round-trip + string inválida→erro; `recurrenceIntentKind.IsValid` zero-value; `recurrenceOutcomeKind.String`/`IsValid` (asserta os 5 rótulos exatos do RF-16).

Comportamento do step (mock `agent.Agent` retornando `RawJSON` do novo schema; mock `BudgetPlanner`):
- primeira entrada suspende com o novo `conclusionRecurrencePrompt` e `PhaseRecurrence`.
- negativa → `CreateRecurrence` **não** chamado (mock `.Times(0)` / ausência de EXPECT), `state.Recurrence==false`, `RecurrenceConfirmation==recurrenceConfirmationNone`, `completeStep`.
- positiva → `CreateRecurrence(..., 12)` `.Once()`, `Recurrence==true`, `RecurrenceMonths==12`, confirmação default, `completeStep`.
- específica 3 → `CreateRecurrence(..., 3)` `.Once()`, `RecurrenceMonths==3`, confirmação "3 meses".
- específica 1 → confirmação singular "1 mês" (helper `monthsLabel`).
- inválida (13 e 0) → suspende com `recurrenceInvalidReprompt`, `CreateRecurrence` não chamado.
- ambígua ("talvez") → suspende com `conclusionRecurrencePrompt`, `CreateRecurrence` não chamado (corrige o cenário atual `:2994-3016`, que hoje completa sem recorrência).
- erro em `CreateRecurrence` → `StepStatusFailed`.
- `BuildCardsStep` prefixa `RecurrenceConfirmation` e zera o campo.
- `conclusionSummaryMessage`/`recurrenceSummaryLine`: reflete N (ex.: "3 meses"), "desligada" quando `Recurrence==false`, e fallback legado `Recurrence==true && RecurrenceMonths==0 → 12` (RF-20).

Testes existentes a atualizar (regressão controlada — comportamento/copy mudam por design):
- `:2938` asserção do prompt inicial → novo `conclusionRecurrencePrompt`.
- `:2959`/`:3035` mock `CreateRecurrence(...,12)` → parametrizado por cenário.
- cenário ambíguo `:2994-3016` → passa a esperar `StepStatusSuspended` (repergunta).
- `:3557`/`:3593` copy do resumo → refletir N/estado.
- `TestM02_NoRendaTermInAnyOnboardingSurface` `:3869/:3876` → incluir `conclusionRecurrencePrompt`, `recurrenceSystemPrompt`, `recurrenceInvalidReprompt` e confirmações no map de superfícies.
- assinatura `BuildRecurrenceStep` em `:3048` (unit) e integração `:716`.

### Testes de Integração

Critérios atendidos: (a) fronteira de IO crítica (LLM real) onde mock não garante compreensão de linguagem natural; (b) o projeto já mantém o suite real-LLM dedicado ao step. Portanto: **estender** `TestRecurrenceExtractionGate` em `internal/agents/application/workflows/onboarding_workflow_integration_test.go:680` (build tag `//go:build integration`, ativado por `RUN_REAL_LLM=1` + `OPENROUTER_API_KEY`, modelo default `openai/gpt-4o-mini`).

- Substituir o `expected{recurrence bool}` por outcome fechado (`recurrenceOutcomeKind`) e cobrir os cenários dos 5 tipos do RF-17: negativa (várias formas do Cenário 1 da US), positiva-12, N numérico ("6 meses", "só 3", "coloca por 6"), N por extenso ("seis meses", "manter por oito meses"), inválido (">12"/"0"), ambíguo ("talvez", emoji).
- Gate `require.GreaterOrEqual(ratio, 0.90, ...)` mantido (padrão `:736`).
- **0 falso-sucesso explícito** (novo): asserção de que nenhum cenário negativo/inválido/ambíguo resultou em chamada de `CreateRecurrence` (via `BudgetPlanner` mock com contagem/spy), e que o N aplicado nos cenários específicos bate exatamente com o esperado — não apenas o ratio agregado.

O registry golden (`internal/agents/application/golden/`) **não** é usado: ele exercita tools do agente diário via `BuildMeControlaAgent`, não os steps do onboarding (confirmado em `cases_onboarding.go:31,44`).

### Testes E2E

O teste full-flow `whatsapp_inbound_consumer_integration_test.go` (`TestInteg_OnboardingFluxoDeCartao...`) atravessa a Sequence completa com cadeia fixa de `.Once()`. O step de recorrência mantém **exatamente uma** chamada `agent.Execute` por turno, preservando a ordem/contagem; apenas o `RawJSON` da fixture de recorrência (`:332/342`) muda do schema antigo `{confirmed:false}` para o novo `{intent,hasMonths,months}`. Atualizar essa fixture é parte da tarefa.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. Tipos-estado fechados + `DecideRecurrence` (puro) + testes unitários determinísticos (base testável sem IO; ADR-001).
2. Schema/prompt dedicados + `recurrenceExtract` + copy Tom de Voz (RF-04/05/07/08/14/15; ADR-002).
3. Campos de estado + `recurrenceSummaryLine`/`conclusionSummaryMessage` + fallback legado (RF-11/13/20; ADR-003).
4. Reescrita de `BuildRecurrenceStep` + confirmação encadeada no `BuildCardsStep` + counter + wiring em `BuildOnboardingWorkflow` (RF-01/02/03/06/09/10/12/16/18; ADR-003/ADR-004).
5. Atualização dos testes existentes impactados + testes unitários novos do step.
6. Extensão do gate real-LLM com 0 falso-sucesso (RF-17; ADR-004).

### Dependências Técnicas

Nenhuma dependência de infraestrutura nova. Gate real-LLM exige `OPENROUTER_API_KEY` + `RUN_REAL_LLM=1` (já usado pelo projeto). Nenhuma migração de banco (estado é snapshot JSON do kernel).

## Monitoramento e Observabilidade

- Métrica nova (RF-16): counter `agents_onboarding_recurrence_total`, label único `outcome` fechado com valores `no_recurrence | default_12 | specific_months | invalid_reprompt | ambiguous_reprompt` (`recurrenceOutcomeKind.String()`). Cardinalidade controlada: **proibido** `months`, `user_id`, `competence` como label (R-TXN-004 / R-AGENT-WF-001.5). Guard nil no `record...` (padrão `:1279`).
- Run auditável do kernel de workflow (thread_id/run_id/status) permanece; nenhuma mudança de tracing além do wrapping de erro existente.
- Dashboards: reaproveitar o painel de onboarding onde já vivem `agents_onboarding_distribution_total`/`agents_onboarding_monthly_budget_total`; adicionar a série de recorrência é opcional e fora do escopo de código deste trabalho.

## Considerações Técnicas

### Decisões Chave

- ADR-001 — Regra de recorrência como decisão pura `DecideRecurrence` + tipos-estado fechados; gate design-patterns-mandatory = **não aplicar padrão** GoF. `.specs/prd-recorrencia-orcamento-onboarding/adr-001-decisao-recorrencia-pura-tipos-fechados.md`
- ADR-002 — Extração LLM única por turno (schema dedicado intent+meses), precedência quantidade>positiva>negativa, ambíguo como estado fechado `unclear`; schema separado para não regredir `summary_confirm`. `.specs/prd-recorrencia-orcamento-onboarding/adr-002-extracao-unica-intent-meses.md`
- ADR-003 — Confirmação encadeada via campo de estado (padrão `GoalConfirmation`) + campo de meses no estado + retrocompatibilidade legado (`Recurrence && months==0 → 12`) para RF-20. `.specs/prd-recorrencia-orcamento-onboarding/adr-003-confirmacao-encadeada-retrocompat-estado.md`
- ADR-004 — Gate real-LLM estendendo `TestRecurrenceExtractionGate` (não o golden registry) com 0 falso-sucesso explícito + counter de outcome. `.specs/prd-recorrencia-orcamento-onboarding/adr-004-gate-real-llm-zero-falso-sucesso.md`

### Riscos Conhecidos

- **`recurrenceSchema` compartilhado**: o `recurrenceSchema` atual (`:687-694`, `{confirmed:bool}`) é usado por `summary_confirm` no budget review (`:1480`, desserializa `yesNoExtract`) além do step de recorrência. Mitigação: introduzir `recurrenceDecisionSchema`/`recurrenceExtract` **novos** e deixar `recurrenceSchema`/`yesNoExtract`/`summaryConfirmSystemPrompt` intocados. Gate de verificação: `summary_confirm` continua desserializando `yesNoExtract` sem mudança.
- **Copy/comportamento de testes existentes**: 8 pontos de teste mudam por design (mapeados na seção de testes). Mitigação: atualização explícita listada; nenhum arquivo golden/postdeploy contém o copy (grep vazio), então o raio é contido ao pacote `workflows`.
- **Snapshot legado in-flight**: onboarding suspenso que já aplicou 12 meses sob o código antigo (`Recurrence==true`, sem `RecurrenceMonths`) retoma no `BuildConclusionStep`. Mitigação: fallback `Recurrence && RecurrenceMonths==0 → 12` em `recurrenceSummaryLine` (RF-20). Sem drain/migração (RF-20).
- **Contagem de chamadas LLM em full-flow**: manter 1 `agent.Execute` por turno no step evita quebrar a cadeia `.Once()` do teste WhatsApp; mudar só o payload da fixture.
- **Incremento de counter em replay**: em replay, o outcome recontabiliza (ex.: `default_12`/`specific_months` novamente). É comportamento aceito (baixa cardinalidade, sem dedupe por replay), consistente com os counters de distribution/monthly-budget existentes.

### Conformidade com Padrões

- `R-AGENT-WF-001` (`.claude/rules/agent-workflows-tools.md`): step fino que delega a aplicação ao binding/usecase; LLM só na call-site sancionada (parse do step); estados de fronteira como tipos fechados (`recurrenceIntentKind`/`recurrenceOutcomeKind`); Run auditável; roteamento por mapa de decisão (sem `switch case intent.Kind` de domínio). Cardinalidade de métrica controlada (R-AGENT-WF-001.5).
- `R-WF-KERNEL-001` (`.claude/rules/workflow-kernel.md`): nenhuma mudança no kernel; estado permanece no snapshot; resume por merge-patch já vigente.
- `R-ADAPTER-001.1` (`.claude/rules/go-adapters.md`): zero comentários em Go de produção.
- `R-TESTING-001` (`.claude/rules/go-testing.md`): testes de comportamento em whitebox `package workflows`, testify/suite, mocks mockery (o arquivo já segue esse padrão).
- `R-TXN-004` (`.claude/rules/transactions-workflows.md`): label de métrica enum-fechado; sem `user_id`/alta cardinalidade.
- DMMF (`.agents/skills/domain-modeling-production`): `Decide*` puro obrigatório; validação de invariante (1–12) nos smart constructors/comando do domínio (já existente) e na decisão do step; estados ilegais irrepresentáveis via enums fechados.
- design-patterns-mandatory: gate = **não aplicar padrão** estrutural/comportamental GoF (ADR-001) — solução direta (função pura + enums + mapa de decisão) é a mais simples, barata e robusta.

### Arquivos Relevantes e Dependentes

Produção (modificados):
- `internal/agents/application/workflows/onboarding_workflow.go` (único arquivo de produção alterado).

Produção (inalterados — evidência de não regressão):
- `internal/agents/application/interfaces/budget_planner.go`, `internal/agents/infrastructure/binding/budget_planner_adapter.go`, `internal/budgets/application/dtos/input/create_recurrence_input.go`, `internal/budgets/domain/commands/create_recurrence.go`, `internal/budgets/application/usecases/create_recurrence.go`, `internal/agents/module.go` (assinatura de `BuildOnboardingWorkflow` inalterada; counter criado internamente), `internal/agents/application/postdeploy/regression_contract.go`.

Testes (modificados/novos):
- `internal/agents/application/workflows/onboarding_workflow_test.go`
- `internal/agents/application/workflows/onboarding_workflow_integration_test.go`
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer_integration_test.go` (fixture de recorrência)

Testes verificados como **não impactados** (evidência): `onboarding_workflow_postgres_resume_integration_test.go` — as três funções (`TestInteg_Retomada...`, `TestInteg_DentroDo...`, `TestInteg_ReviewAwait...`) exercitam apenas os steps goal/monthly-budget/budget-review e **não alcançam o step de recorrência** (grep sem qualquer referência a `recurrence`/`CreateRecurrence`/`confirmed`); a mudança de schema/estado da recorrência não altera esses cenários. Nenhum ajuste necessário.
