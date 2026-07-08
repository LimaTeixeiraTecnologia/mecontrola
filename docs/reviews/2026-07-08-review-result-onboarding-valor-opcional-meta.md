# Resultado da Revisão — Valor Opcional da Meta no Onboarding

Execução do prompt `docs/reviews/2026-07-08-review-prd-onboarding-valor-opcional-meta.md` com subagentes especializados e ciclo `review → bugfix → review`.

## 1. Status

**APPROVED**

## 2. Escopo Revisado

- Spec: `.specs/prd-onboarding-valor-opcional-meta/{prd,techspec,tasks}.md`, `task-1.0..5.0`, `adr-001..003`
- Código: `internal/agents/application/workflows/onboarding_workflow.go` (diff `+126 −33`)
- Testes: `internal/agents/application/workflows/onboarding_workflow_test.go`; harness `internal/agents/application/agents/onboarding_goal_value_realllm_test.go`
- Cadeia de modelo do gate: `mecontrola_agent_realllm_test.go`, `mecontrola_agent.go`, `internal/platform/agent/agent.go`, `internal/platform/llm/openrouter.go`
- Consumidos: `internal/platform/memory/{ports.go,infrastructure/postgres/working_memory_repository.go}`

## 3. Matriz de Rastreabilidade

### Requisitos Funcionais

| RF | Descrição | Status | Evidência |
|----|-----------|--------|-----------|
| RF-01 | Extração combinada meta+valor em 1 chamada | atendido | `goalWithValueSchema`/`goalWithValueExtract`; `BuildGoalStep` bloco `Goal==""` |
| RF-02 | Valor opcional, nunca bloqueia | atendido | `DecideGoalValueCents` retorna `(0,false)` sem erro |
| RF-03 | Objetivo obrigatório, loop bounded MaxAttempts | atendido | `DecideGoal` err → `suspend _goalReprompt`; teste loop idempotente |
| RF-03.1 | Repergunta combinada quando falta objetivo | atendido | `_goalReprompt` combinado + `GoalValueAsked=true` |
| RF-03.2 | Repergunta específica de valor quando só falta valor | atendido | `_goalValueReprompt`; branches L579-582 e L586-589 (ambos testados) |
| RF-03.3 | Valor perguntado no máximo 1x | atendido | guarda `GoalValueAsked`; teste "resume ... nao deve reperguntar valor de novo" |
| RF-04 | Após repergunta de valor, avança independente da resposta | atendido | bloco value-only → `completeStep` |
| RF-05 | Valor inválido tratado como ausência, sem erro técnico | atendido | `DecideGoalValueCents(_, <=0)→(0,false)`; teste `valor-invalido-zero` |
| RF-06 | Recusa na 1ª msg segue regra uniforme | atendido | harness `meta-com-recusa-valor` |
| RF-07 | Converte p/ centavos, positivo; sem teto | atendido | `int64(math.Round(amountBRL*100))` |
| RF-08 | Smart constructor puro distinto de `DecideIncomeCents` | atendido | `DecideGoalValueCents` puro, sem ctx/IO |
| RF-09 | Formatos monetários (5) | atendido | prompts com 5 exemplos; harness 5 formatos verdes |
| RF-10 | Valor sobrevive no `OnboardingState` até conclusão | atendido | campo sem `omitempty`; teste merge-patch |
| RF-11 | Persiste `objetivo_financeiro_valor_centavos` quando >0 | atendido | `BuildConclusionStep`; teste `UpsertMetadata` por `map` exato |
| RF-12 | Omite chave quando sem valor | atendido | teste de omissão por igualdade de `map` |
| RF-13 | Mensagem inicial convida ao valor opcional | atendido | `_welcomeGoalPrompt`; asserção adicionada (F-04) |
| RF-13.1 | Sem eco por campo no step-goal | atendido | `completeStep` avança direto |
| RF-14 | Gate real-LLM ≥0.90 em gpt-4o-mini | atendido | **executado: 8/8 ratio 1.0000 em `openai/gpt-4o-mini`** |
| RF-15 | Mensagem final menciona valor quando presente | atendido | `conclusionFinalMessage(goal, valueCents)`; testes com/sem valor |
| RF-16 | WM markdown inalterada | atendido | `Upsert` = `"## Objetivo Financeiro\n\n"+Goal`; asserção literal |

### Critérios de Aceite das Tasks (1.0–5.0)

Todos `atendido`: pureza/determinismo de `DecideGoalValueCents`; schemas padrão `incomeSchema` strict; 7 cenários de `BuildGoalStep` + meta obrigatória preservada; chave de valor presente sse `>0` e mensagem sem valor byte-idêntica + WM inalterada; harness ratio ≥0.90 reproduzível em gpt-4o-mini com asserts estritos por caso e `Skip` sem env. Gates `go build`/`go vet`/`go test -race`/zero-comentários verdes em todas.

### Regras de Negócio / Governança

`atendido`: DMMF `Decide*` puro; state-as-type (sentinela `int64` + flag, sem `Option`/`Result`); sem `omitempty` (ADR-002); R-AGENT-WF-001 (sem `switch intent.Kind`, LLM só em call-sites de parse); R-ADAPTER-001.1 zero comentários; R-WF-KERNEL-001.7 resume por merge-patch parcial (teste de regressão R1).

## 4. Achados

**Sem achados abertos.** Findings da 1ª rodada, todos remediados no ciclo de bugfix (test-only, zero mudança de produção):
- F-01 (medium→fixed): branch `_goalValueReprompt` com objetivo prévio — coberto.
- F-02 (medium→fixed): loop de meta obrigatória no resume combinado — coberto.
- F-04 (low→fixed): asserção RF-13 do convite ao valor — adicionada.
- F-05 (low→fixed): 4 caminhos `failStep` de parse/unmarshal — cobertos (`BuildGoalStep` 100%).

Observação não-bloqueante (auditoria do harness): o conjunto de 8 casos + `require` hard nos caminhos de recusa torna o gate efetivamente 8/8 — **mais estrito** que a leitura literal "≥0.90"; não mascara falha (segurança-a-mais). Sem impacto em RF-14.

## 5. Validações Executadas

| Comando | Resultado |
|---------|-----------|
| `go build ./internal/agents/...` | OK |
| `go vet ./internal/agents/application/...` | OK |
| `golangci-lint run ./internal/agents/application/workflows/...` | 0 issues |
| `go test -race -count=1 ./internal/agents/application/workflows/...` | 231 passed |
| `RUN_REAL_LLM=1 AGENT_HARNESS_MODEL=openai/gpt-4o-mini go test -tags integration -run GoalValue` | harness ampliado p/ **20 casos**; **4/4 execuções = 20/20 ratio 1.0000** (margem: tolera 2 flips antes de 0.90) |
| Cobertura `BuildGoalStep` / `DecideGoalValueCents` / `conclusionFinalMessage` | 100% / 100% / 100% |
| Gates governança (zero-comment, no-omitempty, no-intent-switch, WM intacta, caller único) | limpos |

## 6. Decisão

Todos os 20 RFs, todos os critérios de aceite das 5 tasks e todas as regras de negócio/governança estão implementados com evidência direta em código, testes unitários e execução real do gate de merge. Os dois subagentes adversariais confirmaram ausência de falso positivo — inclusive a cadeia de seleção de modelo, que exercita genuinamente `openai/gpt-4o-mini`. As lacunas de cobertura detectadas foram fechadas via bugfix (test-only). Sem gaps, sem lacunas, sem ressalvas, sem risco residual aberto.

## 7. Próxima Ação

Nenhuma; implementação aprovada. (Mudanças não commitadas — commit/deploy a critério do owner.)
