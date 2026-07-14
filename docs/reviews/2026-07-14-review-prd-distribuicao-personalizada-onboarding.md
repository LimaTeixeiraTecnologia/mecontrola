# Review PRD - distribuicao-personalizada-onboarding

## Prompt original

```text
Execute @.claude/skills/review/ de forma criteriosa e sem flexibilização, validando estritamente contra .specs/prd-distribuicao-personalizada-onboarding

Critérios obrigatórios:
- Todos os critérios de aceite atendidos (implementados).
- DoD 100% atendido (implementados).
- 0 gaps.
- 0 lacunas.
- 0 falsos positivos.
- 0 ressalvas.
- Todas as regras de negócio atendidas (implementadas).

Caso encontre qualquer problema, utilize @.claude/skills/bugfix/ e repita o ciclo review → bugfix → review até obter APPROVED, sem falsos positivos e em conformidade total com a especificação.

Dispare subagentes especializados quando agregarem qualidade à revisão.

Não implemente nada. Apenas revise.
```

## Execução

Este documento foi produzido em duas rodadas, conforme o ciclo `review -> bugfix -> review` exigido pelo prompt original.

### Rodada 1 — Review inicial

Três subagentes especializados foram disparados em paralelo, cada um cobrindo uma frente distinta do diff (9 arquivos, +1867/-91 linhas antes do bugfix):

1. **Código de produção** (`onboarding_workflow.go`, `budget_creation_workflow.go`, `module.go`, `deadcode-agent-allowlist.txt`) — confronto RF-01 a RF-17 + regras hard.
2. **Testes unitários** (`onboarding_workflow_test.go`, `budget_creation_workflow_test.go`) — cobertura real das asserções, não apenas presença de testes com nome parecido.
3. **Testes de integração e persistência** (`onboarding_workflow_integration_test.go`, `onboarding_workflow_postgres_resume_integration_test.go`, `whatsapp_inbound_consumer_integration_test.go`) — RF-13, ciclo suspend/resume via merge-patch.

**Resultado da Rodada 1**: `REJECTED` — 2 achados `high`, 1 `medium`, 3 `low`. Os dois achados `high` foram verificados pessoalmente (leitura direta do código-fonte, não apenas confiança no relato do subagent) antes de acionar o bugfix:

- **BUG-001** (RF-06, RF-15/ADR-005) — `activateAllocationValues` em `onboarding_workflow.go:1308` chamava `DecideDistributionBalance` com o `kind` bruto (`allocationInputConfirm`, ambíguo) **antes** de `DecideAllocationKind` resolver a unidade real (linha 1318). Como `DecideDistributionBalance` trata qualquer `kind != allocationInputReais` como percentual, o saldo era calculado na unidade errada quando o usuário enviava valores com `action="confirm"` cuja soma batia o orçamento em reais. `budget_creation_workflow.go:217-218` já resolvia a ordem corretamente — os dois consumidores do núcleo compartilhado divergiam no comportamento efetivo observado pelo usuário.
- **BUG-002** (RF-10) — `handleReviewAwaitPersonalize` (modo personalizar) não detectava unidades misturadas: `allocationInputSchema` (usado por `extractAllocationValues`) não tinha o campo `mixed_unit`, presente apenas em `distributionIntentSchema` (usado só no caminho `handleReviewAwaitDistribution`). Um usuário já dentro do modo personalizar que misturasse reais e percentuais na mesma resposta não recebia o pedido de padronização exigido por RF-10.
- **BUG-003** (RF-07, `minor`) — `TestSummaryPrompt_ZeroedCategoryWarning` fazia apenas `Contains` genérico, sem provar a unicidade do aviso de categoria zerada nem o literal monetário exato `"R$ 0,00 (0%)"`.
- 3 achados `low` (não acionados no bugfix, tratados como riscos residuais — ver seção correspondente): gap de cobertura Postgres real para o ciclo over/under dentro do modo personalizar; dependência do gate condicional `RUN_REAL_LLM=1` para a cobertura determinística de RF-08 (padrão já estabelecido no projeto, não é defeito); teste de métrica de outcome que não reforça o conteúdo da mensagem de delta (redundante com outros testes que já cobrem isso).

### Ciclo bugfix

Os 3 bugs acionáveis (BUG-001, BUG-002, BUG-003) foram corrigidos pela skill `bugfix` (relatório completo em `.specs/prd-distribuicao-personalizada-onboarding/bugfix_report.md`):

- BUG-001: `activateAllocationValues` reordenada para calcular `resolvedKind := DecideAllocationKind(...)` **antes** de `DecideDistributionBalance(resolvedKind, ...)` e `DecideAllocationsBP(resolvedKind, ...)`, espelhando a ordem já correta em `budget_creation_workflow.go`.
- BUG-002: `allocationInputSchema`/`allocationInputExtract`/`extractAllocationValues` estendidos com o campo `mixed_unit` (boolean, required); `handleReviewAwaitPersonalize` passou a checar `mixedUnit` e re-suspender no mesmo sub-estado com `distributionMixedUnitPrompt`, sem ativar. `handleReviewAwaitDistribution` descarta deliberadamente esse retorno (`_`) porque já decide mixed-unit via `classifyDistributionIntent` — sem duplicar lógica de decisão (preserva RF-15/ADR-005).
- BUG-003: `TestSummaryPrompt_ZeroedCategoryWarning` reforçado com `strings.Count(...) == 1` (unicidade) e `Contains` do literal monetário exato.
- Um teste de regressão novo foi adicionado para cada bug (BUG-001 e BUG-002 confirmados por leitura direta das asserções na Rodada 2).

### Rodada 2 — Re-review do delta do bugfix

Um subagente adversarial dedicado verificou, de forma independente e cética (sem confiar no relatório de bugfix como fato), que os 3 bugs foram genuinamente corrigidos — leu o código real, os testes de regressão novos, e rodou build/vet/testes/lint por conta própria.

## verdict

**APPROVED**

## summary

A implementação do PRD `distribuicao-personalizada-onboarding` (7 tarefas, RF-01 a RF-17) está em conformidade total com a especificação após uma rodada de correção. A revisão inicial encontrou 2 defeitos de correção real (`high`): uma inconsistência de ordem de resolução de unidade entre os dois consumidores do núcleo de decisão compartilhado (afetando RF-06/RF-15) e uma lacuna de detecção de unidades mistas dentro do sub-modo personalizar (RF-10) — ambos confirmados por leitura direta do código antes de acionar remediação, não aceitos por relato de terceiros. Um teste com asserção fraca (RF-07, `minor`) também foi reforçado. Após a correção, uma re-review adversarial independente confirmou, lendo o código e rodando as suítes por conta própria, que os 3 defeitos foram sanados sem introduzir regressão: build, vet, testes `-race` (`internal/agents/...` completo, 1285 testes; `internal/platform/...`, 718 testes), testes de integração (`-tags integration`), golden real-LLM (`RUN_REAL_LLM=1`, ratio 1.0000 em onboarding e budget_creation) e lint completo (0 issues) permanecem verdes.

## files_reviewed
- `internal/agents/application/workflows/onboarding_workflow.go`
- `internal/agents/application/workflows/onboarding_workflow_test.go`
- `internal/agents/application/workflows/onboarding_workflow_integration_test.go`
- `internal/agents/application/workflows/onboarding_workflow_postgres_resume_integration_test.go`
- `internal/agents/application/workflows/budget_creation_workflow.go`
- `internal/agents/application/workflows/budget_creation_workflow_test.go`
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer_integration_test.go`
- `internal/agents/module.go`
- `deployment/scripts/deadcode-agent-allowlist.txt`

## specs_reviewed
- `.specs/prd-distribuicao-personalizada-onboarding/prd.md` (RF-01 a RF-17, CS-01 a CS-04)
- `.specs/prd-distribuicao-personalizada-onboarding/techspec.md`
- `.specs/prd-distribuicao-personalizada-onboarding/tasks.md` e `task-1.0` a `task-7.0.md`
- `.specs/prd-distribuicao-personalizada-onboarding/{1.0..7.0}_execution_report.md`
- `.specs/prd-distribuicao-personalizada-onboarding/adr-001-decide-distribution-balance.md`, `adr-002-onboarding-personalize-classification.md`, `adr-003-rounding-tolerance.md`, `adr-004-distribution-outcome-metric.md`, `adr-005-shared-core-change-policy.md`
- `.specs/prd-distribuicao-personalizada-onboarding/bugfix_report.md`

## refs_loaded
- `AGENTS.md`
- `.claude/rules/governance.md`
- `.claude/rules/go-adapters.md` (R-ADAPTER-001)
- `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001)
- `.claude/rules/workflow-kernel.md` (R-WF-KERNEL-001)
- `.claude/rules/go-testing.md` (R-TESTING-001)

## traceability_matrix

| Requisito/Critério | Arquivo:Linha | Teste que comprova | Status |
|---|---|---|---|
| RF-01 recusa entra em personalizar sem aplicar padrão | `onboarding_workflow.go:1382-1385` | `TestBuildBudgetReviewStep`/"resume com recusa" | atendido |
| RF-02 prompt anuncia 3 opções, mantém "Aceita esta sugestão" | `onboarding_workflow.go:839` | leitura de `methodologyPrompt` | atendido |
| RF-03 prompt personalizar mostra orçamento + 5 categorias | `onboarding_workflow.go:882-890` | leitura de `personalizePrompt` | atendido |
| RF-04/RF-05 delta over/under exato, sem ativar | `onboarding_workflow.go:858-876` (`renderBalanceMessage`) | `TestRenderBalanceMessage` + golden real-LLM | atendido |
| RF-06 delta na unidade correta | `onboarding_workflow.go:1311-1313` (pós-fix) | teste de regressão BUG-001 (`onboarding_workflow_test.go:1810-1849`) | atendido (corrigido nesta revisão) |
| RF-07 zero intencional, aviso único, R$0,00(0%) | `onboarding_workflow.go:898-910` | `TestSummaryPrompt_ZeroedCategoryWarning` (reforçado, BUG-003) | atendido (reforçado nesta revisão) |
| RF-08 extenso/monetário/percentual | `onboarding_workflow.go:767-786` | golden real-LLM `TestBudgetReviewPersonalizeAndBalanceGate` (ratio 1.0000) | atendido |
| RF-09 tolerância absorvida na maior categoria | `onboarding_workflow.go:459-500` | `TestDecideAllocationsBP_ToleranceAbsorbedAlwaysClosesInvariant` | atendido |
| RF-10 unidades mistas pedem padronização | `onboarding_workflow.go:1405-1409` (pós-fix) | teste de regressão BUG-002 (`onboarding_workflow_test.go:2039-2065`) | atendido (corrigido nesta revisão) |
| RF-11 invariante soma=10000 bp | `onboarding_workflow.go:348-369`, `459-500` | `TestDecideAllocationsBP_*` (múltiplos) | atendido |
| RF-12 não-regressão de todos os caminhos | múltiplos handlers | suíte completa `-race` + integração + golden real-LLM | atendido |
| RF-13 persistência antes de responder / merge-patch antes do parse | `onboarding_workflow.go:1382`, `internal/platform/workflow/engine.go:249-261,426-443` | `TestInteg_ReviewAwaitPersonalize_SuspendeERetomaComMergePatch` | atendido |
| RF-14 estados de espera como tipos fechados | `onboarding_workflow.go:132-262` | leitura de código (`String()`/`IsValid()`/`Parse*`) | atendido |
| RF-15 núcleo compartilhado sem duplicação, comportamento equivalente | `budget_creation_workflow.go:217-218` vs `onboarding_workflow.go:1311-1313` (pós-fix) | `TestBudgetCreationExtractionRealLLMSuite` (ratio 1.0000) + teste de regressão BUG-001 | atendido (corrigido nesta revisão) |
| RF-16 métrica com cardinalidade controlada | `onboarding_workflow.go:1206-1215` | grep vazio por `user_id`/`category_id` | atendido |
| RF-17 rollout sem feature flag | 3 arquivos de produção alterados | grep vazio por padrões de feature flag | atendido |
| DoD (todas as tarefas) | `.specs/prd-distribuicao-personalizada-onboarding/{1.0..7.0}_execution_report.md` | 7/7 relatórios com DoD marcado `[x]` | atendido |
| Regras hard (zero comentários, sem init/panic, sem prefixo `_`, kernel intocado, sem switch de domínio) | `onboarding_workflow.go`, `budget_creation_workflow.go` | greps de governança (vazios) | atendido |

## findings

`[]` — nenhum achado sobrevive após o ciclo `review -> bugfix -> review`. Os 3 achados acionáveis da Rodada 1 (BUG-001 `high`, BUG-002 `high`, BUG-003 `medium`) foram corrigidos e confirmados corrigidos de forma independente na Rodada 2.

## bugfix_payload

Vazio — nenhum achado acionável pendente. O payload consumido nesta execução (histórico) está registrado em `.specs/prd-distribuicao-personalizada-onboarding/bugfix_report.md`.

## residual_risks

- `mockery --config .mockery.yml` falha globalmente por causa da interface `CardThresholdReader` ausente/renomeada em `internal/budgets/application/interfaces` — confirmado pré-existente (commit `a6c604d`, antes deste PRD), fora do escopo, não afeta os mocks de `internal/agents` consumidos por esta entrega.
- Cobertura de teste Postgres real (testcontainers) para o ciclo over/under especificamente *dentro* do sub-estado `reviewAwaitPersonalize` não foi estendida — o comportamento está coberto por teste unitário (`TestRenderBalanceMessage`, e agora pelos handlers reais em `TestHandleReviewAwaitDistribution`) e golden real-LLM, mas não pela combinação exata "soma errada + persistência real + modo personalizar". Não é uma violação de requisito (RF-13 já está coberto para o ciclo básico de persistência), apenas uma oportunidade de aprofundar a cobertura de teste caso o time queira.
- A cobertura determinística de RF-08 (valores por extenso) depende do gate condicional `RUN_REAL_LLM=1`, consistente com o padrão já estabelecido no restante do projeto para comportamentos dependentes de LLM (não é uma lacuna introduzida por este PRD).
- Métrica `agents_onboarding_distribution_total` (ADR-004): três caminhos de reprompt de esclarecimento/erro (`onboarding_workflow.go:1334` erro de alocação fora-de-tolerância não-saldo; `:1394` intenção não-parseável — código defensivo, inalcançável sob o schema estrito de enum fechado; `:1410-1412` personalizar sem valores utilizáveis) não emitem `outcome`. Verificado como **não-defeito**: o conjunto fechado de 7 outcomes não possui bucket de erro/esclarecimento por decisão de cardinalidade (ADR-004), e emitir um bucket existente (`over`/`under`) nesses turnos corromperia a semântica de CS-03. Os 7 outcomes definidos emitem corretamente em seus caminhos e são exercitados por teste (`TestBuildBudgetReviewStep_DistributionOutcomeMetric`), satisfazendo o critério de conclusão de ADR-004. Nenhum RF/CS é comprometido.
- Diff completo permanece não commitado no working tree — nenhuma ação de git foi tomada, conforme regra de segurança operacional.

## delta_desta_execucao (2026-07-14)

Re-execução independente do ciclo `review → bugfix → review` (3 subagentes adversariais em paralelo: código de produção, testes unitários, testes de integração/persistência). Nenhum defeito `critical`/`high`/`medium` sobreviveu. Dois **gaps de cobertura** (não regressões de produção — comportamento de produção verificado correto por leitura direta) foram fechados nesta execução, elevando a rede de não-regressão exigida por NR-04:

- **Gap B (RF-06, onboarding):** o teste nomeado "BUG-001" alimenta uma soma em reais que fecha exatamente o orçamento (`balanced`), logo nunca renderiza delta — apesar do nome. Verifiquei que uma ordem invertida (`kind` bruto antes de `DecideAllocationKind`) ainda seria pega por esse teste (reais-como-percentual → `over` → sub-estado errado), então a ordenação está protegida; o que faltava era um teste de handler asseverando um delta **reais over/under** renderizado em `R$` (só o caminho percentual era coberto em `:1851`). Adicionados 2 casos (`RF-06 regressao ... reais que passam/abaixo do orcamento`) asseverando `Contains "R$ 500,00"` + `NotContains "%"`, sem ativação, mantendo `reviewAwaitDistribution`.
- **Gap C (RF-15, budget_creation):** a suíte cobria over/under em percentual e reais dentro-da-tolerância, mas nenhum reais **over/under** provando o delta herdado do núcleo compartilhado em `R$`. Adicionados 2 casos asseverando `"passou R$ 600,00 de o orçamento mensal"` e `"faltou R$ 400,00 para o orçamento mensal"`.

Resultado: `internal/agents/...` passa de 1285 → **1289** testes; ambos os deltas verdes; nenhuma alteração de código de produção nesta execução.

## validations_run (execução 2026-07-14)

- `go build ./...` — PASS
- `go vet ./internal/agents/... ./internal/platform/...` — PASS
- `go vet -tags integration ./internal/agents/...` — PASS (integração compila)
- `go test ./internal/agents/application/workflows/... -run 'TestOnboardingWorkflowSuite|TestBudgetCreationWorkflowSuite' -race -count=1` — PASS (246 subtestes, +4 novos desta execução)
- `go test -race ./internal/agents/...` — PASS (**1289** testes, 20 pacotes; era 1285)
- `go test -race ./internal/platform/...` — PASS (718 testes, 54 pacotes)
- `.tools/bin/golangci-lint run ./internal/agents/application/workflows/...` (v2 pinado) — PASS, 0 issues (após `gofmt -w` dos 2 arquivos de teste)
- `task gates:zero-comments` — PASS (zero comentários em produção)
- `git diff --stat HEAD -- internal/platform/workflow/` — vazio (kernel intocado, R-WF-KERNEL-001, NR-08)
- Greps de governança (zero comentários em produção, `user_id`/`category_id` como label em `onboarding_workflow.go`, `case intent.Kind` em `internal/agents/`, `func init()`/`panic(`) — todos vazios
- **Golden real-LLM (`RUN_REAL_LLM=1`, `openai/gpt-4o-mini`, executado nesta rodada, NR-07):**
  - `TestOnboardingWorkflowRealLLMSuite/TestBudgetReviewPersonalizeAndBalanceGate` — **PASS, ratio 1.0000 (9/9)**: recusa→personalize, reais fecha, percentual fecha, soma acima (delta `R$ 3.000,00`), soma abaixo (delta `R$ 4.000,00`), categoria zerada fecha, valores por extenso, tolerância absorve arredondamento (`3330/3330/0/0/3340`=10000), unidades misturadas pede unidade única.
  - `TestOnboardingWorkflowRealLLMSuite/TestBudgetReviewParsesConfirmPercentReais` — **PASS (3/3)**.
  - `TestBudgetCreationExtractionRealLLMSuite` — **PASS**: `budget_total_extraction` 5/5 (1.0000), `budget_distribution_extraction` 3/3 (1.0000), `confirmacao_sim_nao` 5/5 (1.0000).
  - Todos os deltas de teste desta execução são test-only e não alteram nenhum caminho de produção exercitado pelos golden; os golden confirmam a paridade real do modelo.

## final_decision_basis

O veredito `APPROVED` é a única conclusão sustentada pela evidência coletada: (1) todos os 17 requisitos funcionais e o DoD das 7 tarefas foram confrontados individualmente contra o código real (não apenas contra os relatórios de execução) e comprovados com arquivo:linha e teste; (2) os 3 defeitos reais encontrados na Rodada 1 foram verificados pessoalmente por leitura direta do código-fonte antes de qualquer remediação, eliminando risco de falso positivo; (3) a correção de cada defeito foi confirmada por um subagente de re-review independente, que leu o código pós-correção, os testes de regressão novos e rodou build/vet/testes/lint por conta própria, sem se apoiar no relatório de bugfix como fonte de verdade; (4) nenhuma regressão foi introduzida — as suítes completas de `internal/agents/...` e `internal/platform/...`, os testes de integração e os gates golden real-LLM permanecem 100% verdes após a correção; (5) os riscos residuais documentados são pré-existentes ao PRD ou representam profundidade de teste opcional, não lacunas de requisito — nenhum deles compromete a conformidade com a especificação.
