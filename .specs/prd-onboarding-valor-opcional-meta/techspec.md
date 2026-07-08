<!-- spec-hash-prd: 4052540751695ef747eb0c656a6009cdca4331ee803d639e0ce5bcb1e5a2fc15 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica

## Resumo Executivo

A funcionalidade adiciona captura **opcional** do valor monetário da meta financeira no `step-goal` do onboarding conversacional (`internal/agents/application/workflows/onboarding_workflow.go`), sem introduzir migration, dependência externa ou nova estrutura de correlação. A extração combinada meta+valor é feita numa única chamada ao parser LLM sancionado (`agent.Agent.Execute` com `llm.Schema` strict), estendendo o padrão vigente de `goalSchema`/`incomeSchema`. O valor trafega no `OnboardingState` (serializado no `Snapshot.State` do kernel de workflow) até o `step-conclusion`, onde é persistido condicionalmente em `platform_resources.metadata` via merge JSONB e mencionado na mensagem final.

Três decisões materiais orientam a implementação e têm ADR dedicada: (1) **par sentinela `hasAmount`+`amountBRL` sob strict schema** com dois schemas (combinado e value-only) para robustez no gpt-4o-mini (ADR-001); (2) **estado do valor como `int64` sentinela** (`0` = não informado, domínio estritamente positivo) mais flag booleana `GoalValueAsked` — DMMF state-as-type, sem `Option`/`Result` (ADR-002); (3) **gate de merge via harness real-LLM ≥ 0.90 medido em `openai/gpt-4o-mini`** (ADR-003). O gate de design-pattern (`design-patterns-mandatory`) retornou `reject` para todo candidato: nenhum padrão de catálogo novo — reutilizam-se os idiomas existentes (factory-function `Decide*`, closure `Build*Step`, flag de tipo fechado).

## Arquitetura do Sistema

### Visão Geral dos Componentes

Todos os pontos em `internal/agents/application/workflows/onboarding_workflow.go` salvo indicação. Âncoras de linha verificadas contra o código atual.

- **`OnboardingState`** (struct, ~L146-159) — *modificado*: dois campos novos `GoalValueCents int64` e `GoalValueAsked bool`. Sem `omitempty` (ver ADR-002 / Risco R2).
- **`DecideGoalValueCents`** (função pura nova, junto a `DecideGoal` L161 / `DecideIncomeCents` L169) — *novo*: smart constructor puro; ausência/zero/negativo → não informado (válido). Distinto de `DecideIncomeCents` (RF-08).
- **`goalWithValueSchema` / `goalValueSchema`** (vars `map[string]any`, junto a `goalSchema` L359 / `incomeSchema` L368) — *novo*: schemas strict de extração combinada e value-only.
- **`goalWithValueExtract` / `goalValueExtract`** (structs de unmarshal, junto a `goalExtract` ~L331) — *novo*.
- **`_goalWithValueSystemPrompt` / `_goalValueSystemPrompt` / `_goalValueReprompt`** (consts, junto aos prompts L412+) — *novo*. `_goalReprompt` (L417) e `_welcomeGoalPrompt` (L412) — *modificados* (repergunta combinada e convite opcional ao valor, RF-13/RF-03.1).
- **`BuildGoalStep`** (closure, L492-521) — *reestruturado*: cobre extração combinada, repergunta combinada (meta ausente), repergunta específica de valor (valor ausente), guarda "asked once", avanço-independente-de-valor. Meta continua obrigatória (zero regressão).
- **`BuildConclusionStep`** (closure, L731-782) — *modificado* apenas no bloco de persistência/mensagem final (L774-780): metadata condicional + `conclusionFinalMessage` value-aware. WM markdown (L774) **intocada** (RF-16).
- **`conclusionFinalMessage`** (L468) — *modificado*: ganha parâmetro `valueCents int64`; reusa `formatBRL` (L291). Caller único (L780).
- **Harness real-LLM** (arquivo novo `internal/agents/application/agents/onboarding_goal_value_realllm_test.go`) — *novo*: casos rotulados, ratio de acerto, assert `>= 0.90`.

### Fluxo de Dados

```
step-goal (resume) ─► a.Execute(goalWithValueSchema, strict) ─► goalWithValueExtract
   ├─ DecideGoal(goal) ────────────────► state.Goal   (obrigatório; loop se vazio)
   └─ DecideGoalValueCents(has, brl) ──► state.GoalValueCents (opcional; 0=ausente)
         │ meta ok + valor ausente + !asked ─► suspend _goalValueReprompt (asked=true)
         │ meta ausente ─────────────────────► suspend _goalReprompt combinado (asked=true)
         └─ resume value-only ─► a.Execute(goalValueSchema) ─► DecideGoalValueCents ─► advance
step-conclusion ─► metadata{objetivo_financeiro[, objetivo_financeiro_valor_centavos]} (merge JSONB)
               └─► FinalMessage = conclusionFinalMessage(goal, valueCents)
```

## Design de Implementação

### Interfaces Chave

Smart constructor puro (assinatura reconciliada — schema entrega `hasAmount`; estado guarda sentinela; ver ADR-001/ADR-002):

```go
func DecideGoalValueCents(hasAmount bool, amountBRL float64) (int64, bool) {
    if !hasAmount || amountBRL <= 0 {
        return 0, false
    }
    return int64(math.Round(amountBRL * 100)), true
}
```

Mensagem final value-aware (assinatura muda; caller único em L780):

```go
func conclusionFinalMessage(goal string, valueCents int64) string {
    objetivo := fmt.Sprintf("Seu objetivo \"%s\"", goal)
    if valueCents > 0 {
        objetivo = fmt.Sprintf("Seu objetivo \"%s\" (meta de %s)", goal, formatBRL(valueCents))
    }
    return fmt.Sprintf(
        "Tudo pronto! 🚀 %s está registrado.\n\n"+
            "Agora é só começar: me envie seus gastos e receitas no dia a dia (ex.: \"gastei R$ 50 no mercado\" ou \"recebi R$ 200 de freela\") que eu registro tudo pra você. Vamos juntos! 💪",
        objetivo,
    )
}
```

`BuildGoalStep` reestruturado (Go, zero comentários — branch trace completo na tabela de mapeamento):

```go
func BuildGoalStep(a agent.Agent) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
    return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
        if state.ResumeText == "" {
            state.Phase = PhaseGoal
            return suspendStep(state, _welcomeGoalPrompt), nil
        }
        resumeText := state.ResumeText
        state.ResumeText = ""

        if state.Goal == "" {
            extracted, err := a.Execute(ctx, agent.Request{
                Messages: []llm.Message{
                    {Role: "system", Content: _goalWithValueSystemPrompt},
                    {Role: "user", Content: resumeText},
                },
                Schema: &llm.Schema{Name: "goal_with_value_extract", Strict: true, Schema: goalWithValueSchema},
            })
            if err != nil {
                return failStep(state, fmt.Errorf("agents.onboarding.goal: parse: %w", err))
            }
            var extract goalWithValueExtract
            if err := json.Unmarshal(extracted.RawJSON, &extract); err != nil {
                return failStep(state, fmt.Errorf("agents.onboarding.goal: unmarshal: %w", err))
            }
            goal, err := DecideGoal(extract.Goal)
            if err != nil {
                state.GoalValueAsked = true
                return suspendStep(state, _goalReprompt), nil
            }
            state.Goal = goal
            if cents, ok := DecideGoalValueCents(extract.HasAmount, extract.AmountBRL); ok {
                state.GoalValueCents = cents
            }
            if state.GoalValueCents == 0 && !state.GoalValueAsked {
                state.GoalValueAsked = true
                return suspendStep(state, _goalValueReprompt), nil
            }
            return completeStep(state), nil
        }

        if !state.GoalValueAsked {
            state.GoalValueAsked = true
            return suspendStep(state, _goalValueReprompt), nil
        }

        extracted, err := a.Execute(ctx, agent.Request{
            Messages: []llm.Message{
                {Role: "system", Content: _goalValueSystemPrompt},
                {Role: "user", Content: resumeText},
            },
            Schema: &llm.Schema{Name: "goal_value_extract", Strict: true, Schema: goalValueSchema},
        })
        if err != nil {
            return failStep(state, fmt.Errorf("agents.onboarding.goal_value: parse: %w", err))
        }
        var extract goalValueExtract
        if err := json.Unmarshal(extracted.RawJSON, &extract); err != nil {
            return failStep(state, fmt.Errorf("agents.onboarding.goal_value: unmarshal: %w", err))
        }
        if cents, ok := DecideGoalValueCents(extract.HasAmount, extract.AmountBRL); ok {
            state.GoalValueCents = cents
        }
        return completeStep(state), nil
    }
}
```

### Modelos de Dados

Campos novos em `OnboardingState` (sem `omitempty` — ver ADR-002 / Risco R2):

```go
GoalValueCents int64 `json:"goalValueCents"`
GoalValueAsked bool  `json:"goalValueAsked"`
```

Schemas de extração (strict; `additionalProperties:false`; todos os campos `required` — ver ADR-001):

```go
var goalWithValueSchema = map[string]any{
    "type": "object",
    "properties": map[string]any{
        "goal":      map[string]any{"type": "string"},
        "hasAmount": map[string]any{"type": "boolean"},
        "amountBRL": map[string]any{"type": "number"},
    },
    "required":             []any{"goal", "hasAmount", "amountBRL"},
    "additionalProperties": false,
}

var goalValueSchema = map[string]any{
    "type": "object",
    "properties": map[string]any{
        "hasAmount": map[string]any{"type": "boolean"},
        "amountBRL": map[string]any{"type": "number"},
    },
    "required":             []any{"hasAmount", "amountBRL"},
    "additionalProperties": false,
}

type goalWithValueExtract struct {
    Goal      string  `json:"goal"`
    HasAmount bool    `json:"hasAmount"`
    AmountBRL float64 `json:"amountBRL"`
}

type goalValueExtract struct {
    HasAmount bool    `json:"hasAmount"`
    AmountBRL float64 `json:"amountBRL"`
}
```

Persistência (metadata JSONB, sem migration; merge `||` em `WorkingMemoryRepository.UpsertMetadata`, `internal/platform/memory/infrastructure/postgres/working_memory_repository.go:75`):

```go
metadata := map[string]any{"objetivo_financeiro": state.Goal}
if state.GoalValueCents > 0 {
    metadata["objetivo_financeiro_valor_centavos"] = state.GoalValueCents
}
```

Chave `objetivo_financeiro_valor_centavos`: `int64` em centavos, espelhando `IncomeCents`. Omitida quando `GoalValueCents == 0` (RF-12); presente quando `> 0` (RF-11).

### Endpoints de API

Não aplicável — o fluxo é conversacional via inbound WhatsApp, sem novo endpoint HTTP.

## Pontos de Integração

- **OpenRouter (único provider LLM)**: duas novas call-sites de parse (`agent.Agent.Execute`), ambas call-sites sancionadas (step de parse, R-AGENT-WF-001.4). Nenhuma dependência nova; `agent.Agent` já injetado em `BuildGoalStep`. Tratamento de erro: `failStep` com `fmt.Errorf("ctx: %w", err)` em falha de `Execute`/`Unmarshal`, idêntico ao padrão vigente. Conversão de formatos coloquiais (RF-09) é responsabilidade do LLM (retorna `amountBRL float64`), não de parser Go.
- **Postgres (`platform_resources.metadata`)**: escrita via port `memory.WorkingMemory.UpsertMetadata` (`internal/platform/memory/ports.go:21`); SQL vive só no adapter postgres. Sem migration.

## Abordagem de Testes

### Testes Unitários

Whitebox `package workflows`, testify/suite quando aplicável (R-TESTING-001). Mock de `agent.Agent` via mockery para os steps; `DecideGoalValueCents` testado sem mock (função pura).

- **`DecideGoalValueCents`** (puro, sem mock): tabela input→output cobrindo `(true, 400000)→(40000000,true)`, `(true, 0.01)→(1,true)`, `(true, 0)→(0,false)`, `(true, -50)→(0,false)`, `(false, 400000)→(0,false)`. Fecha RF-07/RF-08.
- **`BuildGoalStep`** (mock `agent.Agent`): sete cenários da tabela de mapeamento (meta+valor juntos; meta sem valor→repergunta valor; sem meta→repergunta combinada; resume após combinado com meta→complete sem repergunta de valor; resume após combinado sem meta→loop meta; resume value-only válido→salva; resume value-only recusa→avança sem valor). Valida RF-01..RF-06, RF-10, e a não-regressão da obrigatoriedade da meta.
- **`conclusionFinalMessage`**: com valor → contém `(meta de R$ 400.000,00)`; sem valor → string idêntica à atual (RF-15).
- **`BuildConclusionStep`** (mock `memory.WorkingMemory`): assert que `UpsertMetadata` recebe `objetivo_financeiro_valor_centavos` só quando `GoalValueCents > 0` (RF-11/RF-12); assert que `Upsert` (WM markdown) recebe exatamente `"## Objetivo Financeiro\n\n"+Goal`, sem valor (RF-16).
- **Regressão de resume (Risco R1)**: teste que aplica merge-patch `{"resumeText":"..."}` sobre um `Snapshot.State` com `goalValueCents>0`/`goalValueAsked=true` e verifica preservação de ambos os campos após o merge.

### Testes de Integração

Critérios do template: (a) fronteira de IO crítica = LLM real (mocks não garantem correção de extração NL); (b) o projeto já teve falso-verde de extração mascarada (memória de projeto C4). Duas respostas "sim" → harness real-LLM adotado (não testcontainers; a fronteira crítica aqui é o LLM, não o banco).

Arquivo `internal/agents/application/agents/onboarding_goal_value_realllm_test.go`, `//go:build integration`, gate `RUN_REAL_LLM=1` + `OPENROUTER_API_KEY` via `buildRealLLMProvider(t)` (`mecontrola_agent_realllm_test.go:26`), modelo default `openai/gpt-4o-mini` (override `AGENT_HARNESS_MODEL`). Chama `workflows.BuildGoalStep(a)` diretamente (padrão de `onboarding_methodology_realllm_test.go:83`). Casos rotulados cobrindo os 3 cenários (valor junto / ausente / inválido-recusa) e os 5 formatos de RF-09; computa `hits/total` e `require.GreaterOrEqual(ratio, 0.90)`. Ver ADR-003 para o contrato do gate e a composição de casos. Este é o **gate de merge** (RF-14).

### Testes E2E

Não requerido além do harness real-LLM do `step-goal`. O fluxo completo de onboarding já é coberto pelos testes existentes; a mudança é aditiva e localizada.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. `DecideGoalValueCents` + testes unitários puros (base, sem dependência) — fecha RF-07/RF-08.
2. Campos em `OnboardingState` (`GoalValueCents`, `GoalValueAsked`, sem `omitempty`) + teste de preservação em merge-patch (Risco R1) — fecha RF-10.
3. Schemas + structs de extração + system prompts (ADR-001).
4. `BuildGoalStep` reestruturado + testes unitários com mock de `agent.Agent` — fecha RF-01..RF-06, RF-13/RF-13.1.
5. `conclusionFinalMessage` (assinatura) + bloco de persistência condicional em `BuildConclusionStep` + testes — fecha RF-11/RF-12/RF-15/RF-16.
6. Harness real-LLM (ADR-003) — gate de merge RF-14.

### Dependências Técnicas

- Nenhuma infra nova. `OPENROUTER_API_KEY` já disponível no ambiente de execução do harness (`RUN_REAL_LLM=1`).

## Monitoramento e Observabilidade

Sem novas métricas Prometheus (o valor da meta não gera evento nem tem cardinalidade de negócio). Os spans/observabilidade existentes do onboarding cobrem os novos passos de parse por herança do `Build*Step`. Nenhum label de alta cardinalidade introduzido (R-TXN-004 / R-AGENT-WF-001.5). Logs de falha via `failStep`/`fmt.Errorf` com contexto (`agents.onboarding.goal`, `agents.onboarding.goal_value`).

## Considerações Técnicas

### Decisões Chave

- **ADR-001** — Extração combinada com par sentinela `hasAmount`+`amountBRL` sob strict schema, com dois schemas (combinado + value-only). Justificativa: robustez de sinal no gpt-4o-mini (o gate ≥0.90) e responsabilidade única por call-site.
- **ADR-002** — Estado do valor como `int64` sentinela (`0`=não informado; domínio estritamente positivo torna o sentinela seguro) + flag booleana `GoalValueAsked`; DMMF state-as-type, sem `Option`/`Result`/`Either`/currying/DSL. Inclui contrato anti-`omitempty` e preservação em merge-patch.
- **ADR-003** — Gate de merge via harness real-LLM ≥ 0.90 medido em `openai/gpt-4o-mini`; composição de casos e mitigação por instruction-by-example.
- **Gate de design-pattern (sem ADR — resultado negativo)**: `design-patterns-mandatory` retornou `reject` para Strategy (extração), State (fluxo de repergunta) e Builder/Template Method/Chain of Responsibility (rejeitados por over-engineering). Aprovado: reuso de factory-function `Decide*` + closure `Build*Step` + flag de tipo fechado. Justificativa: economia (menos tipos/indireção), eficiência (sem ganho em hot-path/manutenção) e robustez (menor superfície de falha; estados ilegais inexprimíveis).

### Riscos Conhecidos

- **R1 — merge-patch de estado inteiro zeraria os campos novos**: se algum caller emitir resume com o `OnboardingState` inteiro re-serializado (em vez do delta `{"resumeText":...}`), zero-values sobrescreveriam valor/flag. Mitigação: manter o contrato de patch parcial (comportamento atual; exigido por R-WF-KERNEL-001.7) + teste de regressão (passo 2 do sequenciamento).
- **R2 — `omitempty` proibido** em `goalValueCents`/`goalValueAsked`: com `omitempty`, `0`/`false` sumiriam do encode e um patch de estado inteiro não distinguiria "ausente" de "falso". Tags exatas sem `omitempty`, espelhando `IncomeCents`/`CardsDone`.
- **R3 — confiabilidade do `hasAmount` no gpt-4o-mini**: o par sentinela é defensivo (constructor aceita `hasAmount:false` OU `amountBRL<=0` como ausência). Se o harness ficar <0.90 por o modelo fabricar `amountBRL>0` sem valor no texto, reforçar instruction-by-example no `_goalWithValueSystemPrompt` (mitigação já comprovada no projeto para C4). Só o run real fecha essa variável.
- **R4 — mudança de assinatura de `conclusionFinalMessage`**: caller único verificado (L780); `grep "conclusionFinalMessage("` deve retornar só essa call antes do merge.

### Conformidade com Padrões

- **R-AGENT-WF-001**: comportamento novo entra no `step-goal` existente (sem `switch case intent.Kind`); LLM só nas call-sites de parse sancionadas; estados de fronteira do onboarding permanecem tipos fechados (`OnboardingPhase`); estado de espera (pending step) persistido no `Snapshot` antes de suspender (`suspendStep`).
- **R-WF-KERNEL-001.7**: resume por merge-patch parcial; snapshot é fonte única de verdade (sem side-store).
- **R-ADAPTER-001.1**: zero comentários nos novos símbolos Go de produção.
- **DMMF (governance.md)**: `Decide*` puro/determinístico sem IO; validação só no smart constructor; state-as-type; anti-padrões rejeitados.
- **R-DTO-VALIDATE-001**: não aplicável — extração interna ao workflow via schema LLM, sem input DTO em `application/dtos/input/`.
- **R-TESTING-001**: testes de use case/step em whitebox testify/suite; `fake.NewProvider()` para observabilidade.

### Arquivos Relevantes e Dependentes

- `internal/agents/application/workflows/onboarding_workflow.go` — struct, constructor, schemas, prompts, `BuildGoalStep`, `BuildConclusionStep`, `conclusionFinalMessage` (todos os âncoras acima).
- `internal/platform/memory/ports.go:21` — `WorkingMemory.UpsertMetadata` (consumido, não alterado).
- `internal/platform/memory/infrastructure/postgres/working_memory_repository.go:75` — merge JSONB (consumido, não alterado).
- `internal/platform/agent/ports.go` — `agent.Request`/`agent.Result.RawJSON` (consumido).
- `internal/platform/llm/types.go` — `llm.Schema` (consumido).
- `internal/platform/workflow/step.go:134` — `StepOutput[S]` (consumido pelo harness).
- `internal/agents/application/agents/mecontrola_agent_realllm_test.go:26` / `onboarding_methodology_realllm_test.go` — padrões reusados pelo harness novo.
- `internal/agents/application/agents/onboarding_goal_value_realllm_test.go` — arquivo novo (harness).

## ADRs

- [adr-001-extracao-combinada-sentinela.md](adr-001-extracao-combinada-sentinela.md)
- [adr-002-estado-valor-sentinela-flag.md](adr-002-estado-valor-sentinela-flag.md)
- [adr-003-gate-real-llm-gpt4o-mini.md](adr-003-gate-real-llm-gpt4o-mini.md)
