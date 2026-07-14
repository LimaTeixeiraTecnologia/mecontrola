<!-- spec-hash-prd: 29e6b1f85b7ab124ddcebf489a232415486ded02879f703c621e9c43294fed0b -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Distribuição personalizada do orçamento no onboarding

Fonte: `.specs/prd-distribuicao-personalizada-onboarding/prd.md` (spec-version 1) e `docs/us/us-distribuicao-personalizada-onboarding.md` (US-001).
Alvo primário: `internal/agents/application/workflows/onboarding_workflow.go` (passo `stepBudgetReviewID` / `reviewAwaitDistribution`).
Skills obrigatórias materializadas: `go-implementation` (R0–R7, R-TESTING-001, R-ADAPTER-001), `domain-modeling-production` (state-as-type, decisão pura), `design-patterns-mandatory` (gate executado — resultado `reject`/não aplicar padrão), `mastra` (consumir substrato, LLM só nas call-sites sancionadas, suspend/resume durável).

## Resumo Executivo

O passo de distribuição do onboarding é um `Step` durável do kernel `internal/platform/workflow` que hoje classifica a resposta do usuário via LLM em `{confirm, percent, reais}` (schema/prompt compartilhados com `budget_creation_workflow`) e converte para basis points com `DecideAllocationsBP`, embutindo mensagens de erro de UI dentro do `error` retornado. Esta entrega: (1) introduz um sub-estado de espera fechado novo `reviewAwaitPersonalize` no enum `reviewAwaitKind` (onboarding-only) para o modo "personalizar"; (2) extrai a decisão de saldo (passou/faltou/dentro-da-tolerância) para uma função pura nova `DecideDistributionBalance` que retorna um tipo fechado, com o texto de UI renderizado na camada de workflow (padrão DMMF — decisão pura + state-as-type); (3) adiciona uma classificação de intenção onboarding-only (`accept | personalize | values` + `mixedUnit`) que não toca o schema compartilhado; (4) generaliza a conversão em basis points para absorver arredondamento na maior categoria (tolerância ±0,5% / ±R$0,05); (5) enriquece os prompts de extração para aceitar valores por extenso; e (6) emite um contador de outcome de distribuição com rótulo fechado.

As melhorias que vivem no núcleo compartilhado (decisão de saldo, tolerância, extração por extenso) valem para os dois fluxos (onboarding e `budget_creation`), conforme PRD RF-15; o modo personalizar e sua copy permanecem exclusivos do onboarding. Nenhuma função de decisão é duplicada. O gate de design pattern foi executado (`select_pattern.py` → `reject`): a solução é código direto (funções puras + enum fechado) sobre a máquina de estados que o kernel já oferece; não se aplica padrão GoF (ver ADR-001).

## Arquitetura do Sistema

### Visão Geral dos Componentes

Componentes modificados/novos (todos em `internal/agents/application/workflows/`, salvo indicação):

- `onboarding_workflow.go` (modificado):
  - `reviewAwaitKind` (enum fechado) ganha a constante `reviewAwaitPersonalize`; `String()`/`IsValid()` atualizados.
  - Nova classificação onboarding-only: tipo `distributionIntentKind` (fechado), struct `distributionIntentExtract`, `distributionIntentSchema`, `distributionIntentSystemPrompt`.
  - `handleReviewAwaitDistribution` passa a rotear por intenção (`accept | personalize | values`); nova função `handleReviewAwaitPersonalize`.
  - `methodologyPrompt` (copy anuncia "não → personalizar"), novos prompts `personalizePrompt(monthlyBudgetCents)` e renderização de mensagens de saldo `renderBalanceMessage(...)`.
  - `summaryPrompt` passa a anexar aviso único de categorias zeradas.
  - Emissão do contador `agents_onboarding_distribution_total` (label `outcome`).
- Núcleo compartilhado (definido em `onboarding_workflow.go`, consumido também por `budget_creation_workflow.go`):
  - Nova função pura `DecideDistributionBalance(kind, valuesBySlug, monthlyBudgetCents) DistributionBalance` + tipo fechado `distributionBalanceKind` e struct `DistributionBalance`.
  - `DecideAllocationsBP` refatorada: remove as mensagens de over/under (movidas para o saldo) e passa a fechar 10000 por maior-resto para percentual e reais (absorção de arredondamento); mantém rejeição de negativo, confirm-com-valores e orçamento ausente.
  - `allocationInputSystemPrompt` enriquecido com exemplos por extenso (beneficia ambos os fluxos).
- `budget_creation_workflow.go` (modificado): `handleBudgetDistributionSlot` passa a chamar `DecideDistributionBalance` antes de `DecideAllocationsBP` e renderiza a mensagem de saldo no seu próprio reprompt; sem modo personalizar.
- `internal/agents/module.go` (modificado): `BuildOnboardingWorkflow` recebe `observability.Observability` para criar o contador de distribuição; wiring em `module.go:231`.

Fluxo de dados (inbound → suspend): WhatsApp `p.Text` → `resolve_onboarding_or_agent.go:143` marshaling `{"resumeText": msg}` → `Engine.Resume` (merge-patch, `codec.go:31-46`) → `OnboardingState.ResumeText` → handler do passo → `StepOutput{Suspended, Suspension{Prompt}}` → `resolve_onboarding_or_agent.go:155` → `whatsapp_inbound_consumer.go` `sendReply` → `SendTextMessage`.

## Design de Implementação

### Interfaces Chave

Tipos fechados novos (state-as-type; `iota + 1`, zero-value inválido — R5.8 / DMMF):

```go
type reviewAwaitKind int

const (
	reviewAwaitDistribution reviewAwaitKind = iota + 1
	reviewAwaitConfirm
	reviewAwaitPersonalize
)

type distributionIntentKind int

const (
	distributionIntentAccept distributionIntentKind = iota + 1
	distributionIntentPersonalize
	distributionIntentValues
)

type distributionBalanceKind int

const (
	distributionBalanced distributionBalanceKind = iota + 1
	distributionOver
	distributionUnder
)
```

Decisão pura de saldo (sem IO, sem `context.Context`, determinística — DMMF Princípio "Decide* puro"):

```go
type DistributionBalance struct {
	Status   distributionBalanceKind
	Unit     allocationInputKind
	Target   float64
	Sum      float64
	DeltaAbs float64
}

func DecideDistributionBalance(kind allocationInputKind, valuesBySlug map[string]float64, monthlyBudgetCents int64) DistributionBalance
```

Assinaturas dos handlers e da conversão (compatibilidade com o kernel `func(context.Context, S) (StepOutput[S], error)`):

```go
func handleReviewAwaitDistribution(ctx context.Context, a agent.Agent, budgets interfaces.BudgetPlanner, dist observability.Counter, state OnboardingState) (workflow.StepOutput[OnboardingState], error)
func handleReviewAwaitPersonalize(ctx context.Context, a agent.Agent, budgets interfaces.BudgetPlanner, dist observability.Counter, state OnboardingState) (workflow.StepOutput[OnboardingState], error)

func DecideAllocationsBP(kind allocationInputKind, valuesBySlug map[string]float64, monthlyBudgetCents int64) (map[string]int, error)
```

Contador de outcome (RF-16), criado via `o11y.Metrics().Counter(...)` e incrementado com `.Add(ctx, 1, observability.String("outcome", v))` — espelha `agents_budget_creation_total` (`internal/agents/application/usecases/budget_creation_continuer.go:36-65`).

### Modelos de Dados

Sem mudança de schema de banco. O estado durável é `OnboardingState` (`onboarding_workflow.go:181-195`), já serializado como `Snapshot.State` (JSON) pelo kernel; o campo novo do sub-estado é o próprio `ReviewAwait reviewAwaitKind` (já existente, com JSON tag), que agora admite `reviewAwaitPersonalize`. Como o resume aplica merge-patch sobre o snapshot (`codec.go:31-46`), o sub-estado sobrevive a suspend/resume sem persistência adicional (evidência: agente de kernel — `OnboardingState` é o `S` de `Engine[S]`).

Schema de classificação de intenção onboarding-only (Structured Output estrito, `llm.Schema{Strict:true}`), **apenas de intenção + unidade-misturada** — NÃO extrai valores, para preservar intacta a extração compartilhada:

```go
var distributionIntentSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"action":     map[string]any{"type": "string", "enum": []any{"accept", "personalize", "values"}},
		"mixed_unit": map[string]any{"type": "boolean"},
	},
	"required":             []any{"action", "mixed_unit"},
	"additionalProperties": false,
}
```

Design em dois passos (garantia de zero-regressão): (1) o pré-classificador de intenção onboarding-only roda **antes** da extração de valores e retorna `action` + `mixed_unit`; (2) quando `action=values`, a extração de valores usa o `allocationInputSchema`/`allocationInputSystemPrompt` **compartilhado e inalterado** (mesmo comportamento de hoje), seguida de `DecideAllocationKind`. Precedência de classificação: `values` > `personalize` > `accept` — se a mensagem contém números utilizáveis, `action=values` mesmo que traga a palavra "não" (RF: recusa+valores → processa valores). `mixed_unit=true` curto-circuita para a mensagem de unidade única (RF-10) antes da extração. `budget_creation` permanece 100% intocado no seu caminho de classificação/extração (usa o `allocationInputSchema`/`allocationInputSystemPrompt` compartilhado, sem intenção `personalize`). Custo aceito do design: no caminho `values`, duas chamadas LLM (intenção + extração); em `accept`/`personalize`, uma. Trade-off deliberado a favor de zero-regressão do caminho de valores existente.

### Endpoints de API

Não aplicável. A superfície é conversacional (WhatsApp inbound → workflow durável), sem novo endpoint HTTP.

## Pontos de Integração

- LLM: única call-site sancionada (`agent.Agent.Execute` com `llm.Schema`), OpenRouter como único provider (mastra). A classificação de intenção/valores e a detecção de unidade-misturada acontecem aqui via Structured Output; nada de LLM no kernel nem nas funções `Decide*`.
- Observabilidade: `observability.Observability` (devkit-go) para o contador de outcome; nenhum novo serviço externo.
- Persistência do orçamento: reutiliza `interfaces.BudgetPlanner` (`SuggestAllocation`, `CreateBudget`, `DeleteDraftBudget`, `GetMonthlySummary`, `ActivateBudget`) via `applyDraftBudget` (`onboarding_workflow.go:913-952`) — inalterado.

## Abordagem de Testes

### Testes Unitários

Padrão canônico obrigatório: testify/suite whitebox, table-driven, mockery, `fake.NewProvider()` (R-TESTING-001; `.claude/rules/go-testing.md`).

- Funções puras (sem mock): `DecideDistributionBalance` (balanced dentro da tolerância; over com delta em % e em R$; under idem; unidade correta) e `DecideAllocationsBP` refatorada (fecha 10000 por maior-resto em percent e reais; absorção de arredondamento; rejeita negativo; confirm-com-valores; orçamento ausente).
- Mapeamento de intenção (`distributionIntentKind`) e roteamento dos handlers com `agent.Agent` mockado: `accept`→default→confirm; `personalize`→`reviewAwaitPersonalize`+prompt âncora+ZERO; `values` válidos→confirm; `values` over/under→mensagem de delta na unidade+permanece no sub-estado; `mixed_unit`→pede unidade única.
- Aviso de categoria zerada no resumo (RF-07): assert de que a linha de aviso aparece quando há BP 0 e some quando não há.
- Contador `agents_onboarding_distribution_total`: assert via `fake.FakeMetrics` do label `outcome` para cada caminho.

Testes de baseline a atualizar (evidência do mapeamento de blast radius; mudança intencional por RF-15/ADR-001):

- `onboarding_workflow_test.go`: `TestDecideAllocationsBP` linhas 442 (confirm-com-valores permanece), 482 (a asserção de "90%" migra para teste de `DecideDistributionBalance`), 508 (a asserção de "orçamento mensal" migra para o saldo); `TestDecideAllocationKind` 537/545/553 (inalterado); `TestDecideErrorMessagesExcludeRenda` 2537/2541/2545/2549 (as mensagens de over/under saem de `DecideAllocationsBP`; a garantia "sem renda" passa a valer sobre `renderBalanceMessage`); cenário de resume `1331` ("90%" continua presente pois a soma é ecoada; segue `NotContains "o usuário"/"você orienta"`).
- `budget_creation_workflow_test.go`: 163/183/235/254 (o prefixo de reprompt "não consegui aplicar essa distribuição" e "não entendi sua resposta" permanecem; o `reason` de over/under passa a vir do saldo); `budget_creation_decisions_test.go` 88/111 (mantidos — `DecideBudgetDistribution` continua validando 10000 como rede de segurança); integração 181 (distribuição parcial continua não ativando).

### Testes de Integração

Critérios do template: há fronteira de IO crítica (persistência do snapshot no Postgres do kernel) e o sub-estado novo precisa sobreviver a resume real. Recomendado reutilizar o teste existente `onboarding_workflow_postgres_resume_integration_test.go` estendendo-o com um ciclo suspend→resume no `reviewAwaitPersonalize` (garante que o novo enum persiste e retoma via merge-patch). Sem novos containers além dos já usados (build tag `//go:build integration`).

### Testes E2E

Validação real-LLM (política do projeto: `RUN_REAL_LLM=1` com credenciais OpenRouter) cobrindo os 5 comportamentos do PRD + tolerância + unidades misturadas, no estilo dos golden reais já existentes (`budget_creation_workflow_real_llm_test.go`). Critério de aprovação: os cenários golden dos comportamentos passam de forma determinística.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. Tipos fechados novos + `String()`/`IsValid()` (`reviewAwaitPersonalize`, `distributionIntentKind`, `distributionBalanceKind`) — base sem dependência.
2. `DecideDistributionBalance` puro + testes unitários (fecha o contrato de saldo antes de qualquer render).
3. Refatorar `DecideAllocationsBP` (maior-resto para ambos; remover over/under) + testes; ajustar `DecideBudgetDistribution` como rede de segurança.
4. Classificação onboarding-only (`distributionIntentSchema`/prompt) + mapeamento + `renderBalanceMessage`/`personalizePrompt`/copy de `methodologyPrompt` + aviso de zero no `summaryPrompt`.
5. Handlers `handleReviewAwaitDistribution` (roteia intenção) e `handleReviewAwaitPersonalize` + contador de outcome; wiring de `observability.Observability` em `BuildBudgetReviewStep`/`BuildOnboardingWorkflow`/`module.go`.
6. Atualizar `budget_creation_workflow.go` para consumir o saldo (RF-15) e atualizar seus testes.
7. Integração (resume Postgres do novo sub-estado) + golden real-LLM.

### Dependências Técnicas

Nenhuma infra nova. Depende apenas do kernel `internal/platform/workflow`, do substrato `internal/platform/{agent,memory,llm}` e do `interfaces.BudgetPlanner` já existentes.

## Monitoramento e Observabilidade

- Métrica nova (RF-16): contador `agents_onboarding_distribution_total`, unidade `"1"`, label fechado `outcome` ∈ `{personalize_entered, accepted_default, accepted_values, over, under, mixed_unit, tolerance_absorbed}`. Precedência de emissão no aceite: quando houve absorção de arredondamento emite `tolerance_absorbed`; caso contrário `accepted_values` (ADR-003/ADR-004). Cardinalidade controlada — proibido `user_id`/`category_id` (R-TXN-004, R-AGENT-WF-001.5). Criado com `o11y.Metrics().Counter(...)`, incrementado com `.Add(ctx, 1, observability.String("outcome", v))`; guarda nil-safe como em `budget_creation_continuer.go`.
- Escopo do aviso de categoria zerada (RF-07): apenas no resumo de confirmação do onboarding (`summaryPrompt`); `budget_creation` NÃO recebe o aviso nesta entrega (fora da lista de melhorias compartilhadas de RF-15).
- Métricas existentes preservadas: `agents_onboarding_*` (outcome completed/resumed em `resolve_onboarding_or_agent.go`) e o reaper `workflow_stale_suspended_reaped_total` (`reaper.go`), que continua cobrindo abandono no sub-estado personalizar (CS-02).
- Logs/traces: o span do passo já existe; erros seguem wrapping `%w` (R5.10). Sem novo dashboard obrigatório; o label `outcome` alimenta os painéis de onboarding existentes.

## Garantia de Não-Regressão (não negociável)

Diretriz do solicitante: nenhuma regressão no codebase, em hipótese nenhuma. Invariantes obrigatórias desta entrega:

- NR-01 Caminhos felizes idênticos: entradas que hoje somam exatamente 100% (percentual) ou exatamente o orçamento (reais) produzem os MESMOS basis points de hoje. A conversão por maior-resto (ADR-003) para soma exata é equivalente ao comportamento atual (`centsToBasisPoints` já é o caminho de reais; percentual exato `40/10/10/10/30` → `{4000,1000,1000,1000,3000}` inalterado). Teste dedicado compara BP antes/depois para as distribuições canônicas.
- NR-02 Extração de valores do onboarding preservada: o caminho `values` reutiliza o `allocationInputSchema`/`allocationInputSystemPrompt` compartilhado sem alteração de contrato; o único acréscimo é o pré-classificador de intenção (ADR-002). Golden real-LLM cobre accept e values existentes para provar paridade.
- NR-03 `budget_creation` sem regressão funcional: mantém sua classificação/extração; ganha apenas saldo/tolerância/extenso via núcleo compartilhado (RF-15). `DecideBudgetDistribution` (sum=10000) permanece como rede de segurança. Suíte completa do `budget_creation` verde é gate de merge.
- NR-04 Rejeição de over/under preservada: soma fora da tolerância continua NÃO ativando e re-suspendendo no mesmo sub-estado; muda apenas o texto da mensagem (melhoria RF-04/05), não o efeito. O teste `onboarding_workflow_test.go:1331` continua válido (a soma `90%` segue ecoada; segue `NotContains "o usuário"/"você orienta"`).
- NR-05 Reabertura pelo "não" no resumo preservada: `reviewAwaitConfirm` com "não" continua reabrindo a distribuição na sugestão padrão (`methodologyPrompt`), preservando `onboarding_workflow_test.go:1386`; personalizar NÃO é reaberto automaticamente.
- NR-06 Confirm-com-valores e mensagens sem "renda": `errAllocationConfirmWithValues` mantido; `renderBalanceMessage` não vaza "renda" (o guard de `TestDecideErrorMessagesExcludeRenda` é reapontado para a nova renderização).
- NR-07 Gate executável obrigatório: antes do merge, rodar `go build ./...`, `go vet ./...`, `go test ./... -count=1 -race` (pacotes `internal/agents/...` e `internal/platform/...`), `golangci-lint run`, `mockery --config mockery.yml --dry-run`, os greps R0/R1/R5/R7 (go-implementation Etapa 5) e os gates de `.claude/rules/` (R-ADAPTER-001 zero comentários, R-AGENT-WF-001, R-WF-KERNEL-001). Qualquer item vermelho bloqueia. Adicionalmente, validação real-LLM (`RUN_REAL_LLM=1`) dos comportamentos, pois mocks não exercitam a extração real.
- NR-08 Kernel intocado: nenhuma alteração em `internal/platform/workflow` — toda a lógica vive no consumidor `internal/agents` (R-WF-KERNEL-001 preservado).

## Considerações Técnicas

### Decisões Chave

Cada decisão material tem ADR dedicada nesta pasta:

- ADR-001 (`adr-001-decide-distribution-balance.md`): saldo passou/faltou como decisão pura + tipo fechado, render na camada de workflow (Híbrido DMMF). Inclui o resultado do gate `design-patterns-mandatory` (`select_pattern.py` → `reject` = não aplicar padrão) e as alternativas GoF rejeitadas (State — já provido pelo kernel; Strategy — sem troca de algoritmo em runtime).
- ADR-002 (`adr-002-onboarding-personalize-classification.md`): sub-estado fechado `reviewAwaitPersonalize` + classificação de intenção onboarding-only em dois passos (pré-classificador `accept|personalize|values`+`mixed_unit` antes da extração de valores compartilhada, que fica inalterada), precedência `values > personalize > accept`, sem tocar o schema compartilhado.
- ADR-003 (`adr-003-rounding-tolerance.md`): tolerância ±0,5% / ±R$0,05 sobre a soma bruta, com absorção do resto na maior categoria via maior-resto.
- ADR-004 (`adr-004-distribution-outcome-metric.md`): contador de outcome com rótulo fechado e cardinalidade controlada.
- ADR-005 (`adr-005-shared-core-change-policy.md`): política do núcleo compartilhado (melhorias em ambos os fluxos, personalizar onboarding-only) e plano de atualização de testes do `budget_creation`.

### Riscos Conhecidos

- Classificação LLM de intenção (accept vs personalize vs values) e de unidade-misturada é probabilística. Mitigação: Structured Output estrito com enum fechado; funções puras a jusante são determinísticas; validação golden real-LLM obrigatória; em ambiguidade, o fluxo re-suspende com orientação (nunca ativa parcial).
- Blast radius do núcleo compartilhado pode regredir `budget_creation`. Mitigação: `DecideDistributionBalance` isola a decisão; `DecideBudgetDistribution` permanece como rede de segurança (sum=10000); suíte de `budget_creation` atualizada e mantida verde como gate de merge (ADR-005).
- Absorção de arredondamento pode mascarar erro real se a banda for larga. Mitigação: banda estreita (±0,5% / ±R$0,05) sobre a soma bruta; acima disso cai em over/under (ADR-003).
- Wiring de `observability.Observability` no `Step` amplia a assinatura de `BuildBudgetReviewStep`/`BuildOnboardingWorkflow`. Mitigação: mudança localizada e já há precedente (reaper e continuers recebem `o11y`); contador nil-safe para testes.

### Conformidade com Padrões

- `.claude/rules/go-adapters.md` (R-ADAPTER-001.1): zero comentários em Go de produção; handlers finos delegando a decisão pura.
- `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001): estados de fronteira como tipos fechados (.3), LLM só nas call-sites sancionadas (.4), estado de espera persistido antes de pedir input e retomado por merge-patch antes do parse (.7), sem `switch case intent.Kind` de domínio (roteamento por intenção fechada no passo, não por kind de domínio).
- `.claude/rules/workflow-kernel.md` (R-WF-KERNEL-001): nenhuma regra/enum de domínio novo no kernel; toda a lógica vive no consumidor `internal/agents`.
- `.claude/rules/go-testing.md` (R-TESTING-001): suite whitebox, dependencies+IIFE, `fake.NewProvider()`.
- `.claude/rules/input-dto-validate.md`: não se aplica diretamente (sem novo input DTO em `application/dtos/input/`); a validação de superfície vive nas funções `Decide*`.
- Go 1.26.5 (`go.mod`): habilitados `maps.Clone`, `errors.Join`, `min/max`, `slices`, `cmp` — usar onde reduzir código (R7).

### Arquivos Relevantes e Dependentes

- `internal/agents/application/workflows/onboarding_workflow.go` (núcleo da mudança).
- `internal/agents/application/workflows/budget_creation_workflow.go`, `budget_creation_state.go`, `budget_creation_decisions.go` (consumidores do núcleo compartilhado — RF-15).
- `internal/agents/application/workflows/onboarding_workflow_test.go`, `onboarding_workflow_integration_test.go`, `onboarding_workflow_postgres_resume_integration_test.go`, `budget_creation_workflow_test.go`, `budget_creation_decisions_test.go`, `budget_creation_workflow_integration_test.go`, `budget_creation_workflow_real_llm_test.go` (testes impactados/novos).
- `internal/agents/module.go` (wiring de `BuildOnboardingWorkflow` + `observability.Observability`).
- `internal/agents/application/usecases/resolve_onboarding_or_agent.go`, `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go` (fluxo resume/entrega — sem alteração de contrato, referência).
- `internal/platform/workflow/{step.go,engine.go,codec.go,store.go,reaper.go}` (kernel — sem alteração).
- `internal/agents/application/interfaces/budget_planner.go`, `internal/platform/money/money.go` (dependências — sem alteração).
