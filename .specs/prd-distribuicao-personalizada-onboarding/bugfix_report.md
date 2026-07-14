# Relatorio de Bugfix

- Total de bugs no escopo: 3
- Corrigidos: 3
- Testes de regressao adicionados: 3
- Pendentes: nenhum
- Estado final: done

## Bugs

### BUG-001

- ID: BUG-001
- Severidade: major
- Origem: RF-06 + RF-15/ADR-005, achado da skill review sobre este PRD
- Estado: fixed
- Causa raiz: em `activateAllocationValues` (`internal/agents/application/workflows/onboarding_workflow.go`), `DecideDistributionBalance(kind, values, state.MonthlyBudgetCents)` era chamado com o `kind` bruto (ainda `allocationInputConfirm` quando a intencao extraida era `action=confirm`) ANTES do calculo de `resolvedKind := DecideAllocationKind(...)`. Como `DecideDistributionBalance` trata qualquer `kind != allocationInputReais` como percentual (target=100), o saldo era calculado na unidade errada (percentual) quando a soma informada pelo usuario batia o orcamento em reais. O `budget_creation_workflow.go` (linhas 217-218, `handleBudgetDistributionSlot`) ja resolvia `kind = DecideAllocationKind(...)` antes de chamar `DecideDistributionBalance`, evidenciando a divergencia de ordem entre os dois consumidores do nucleo compartilhado (RF-15/ADR-005).
- Arquivos alterados:
  - `internal/agents/application/workflows/onboarding_workflow.go:1307-1319` (funcao `activateAllocationValues`): reordenada a chamada de `DecideAllocationKind` para ANTES de `DecideDistributionBalance`, passando `resolvedKind` (nao o `kind` bruto) para o calculo de saldo. Nenhuma assinatura publica de `DecideDistributionBalance`/`DecideAllocationKind` foi alterada — apenas a ordem/argumento da chamada em `activateAllocationValues`, espelhando exatamente `budget_creation_workflow.go:217-218`.
- Teste de regressao: `TestOnboardingWorkflowSuite/TestBuildBudgetReviewStep/BUG-001_regressao:_resume_em_reviewAwaitDistribution_com_action=confirm_cuja_soma_bate_o_orcamento_em_reais_deve_reportar_saldo/delta_em_R$,_nao_em_percentual` (`internal/agents/application/workflows/onboarding_workflow_test.go`, cenario adicionado na tabela de `TestBuildBudgetReviewStep`) — action=`confirm`, soma dos valores = 13.500,00 (bate o orcamento mensal de R$ 13.500,00 em reais, nao em 100%); assevera que o step SUSPENDE em `reviewAwaitConfirm` (ativacao bem-sucedida), provando que o saldo foi calculado com a unidade REAL resolvida (se calculado como percentual, a soma 13500 ficaria `distributionUnder` por ~13400 pontos e jamais ativaria).
- Validacao: `go build ./...` PASS; `go vet ./...` PASS; `go test -race ./internal/agents/...` PASS (1285 testes); `go test -tags integration -race ./internal/agents/...` PASS (1378 testes); golden real-LLM `TestBudgetReviewPersonalizeAndBalanceGate` (8/8 subtests) e `TestBudgetCreationExtractionRealLLMSuite` (14/14 subtests) PASS; `task lint:run` PASS (0 issues).

### BUG-002

- ID: BUG-002
- Severidade: major
- Origem: RF-10, achado da skill review
- Estado: fixed
- Causa raiz: `handleReviewAwaitPersonalize` chamava `extractAllocationValues`, que usa `allocationInputSchema`/`allocationInputExtract` — schema sem campo `mixed_unit`. A deteccao de unidade mista so existia em `distributionIntentSchema`/`classifyDistributionIntent`, usada exclusivamente por `handleReviewAwaitDistribution`. No modo personalizar (sub-fluxo introduzido pelo mesmo PRD, RF-01), a extracao LLM unica decidia silenciosamente uma unidade (percent OU reais) quando o usuario misturava unidades na mesma mensagem, sem pedir padronizacao — violando RF-10, que exige tratamento identico "no passo de distribuicao" (incluindo o sub-modo personalizar).
- Decisao de design: entre as duas abordagens sugeridas — (a) reaproveitar `classifyDistributionIntent` dentro do modo personalizar, ou (b) estender `allocationInputSchema`/`extractAllocationValues` com um campo `mixed_unit` reutilizado pelos dois handlers — foi escolhida a abordagem (b). Motivo: o modo personalizar sempre trata a resposta do usuario como valores (nunca precisa decidir entre accept/personalize/values), entao adicionar uma segunda chamada LLM via `classifyDistributionIntent` (abordagem a) geraria uma chamada LLM redundante e custo/latencia extras sem necessidade. Estender o schema/extract ja usado (abordagem b) mantem uma unica chamada LLM por resposta do usuario, preserva Structured Output estrito (R-AGENT-WF-001.4) e reutiliza a mesma constante de mensagem (`distributionMixedUnitPrompt`) sem duplicar logica de decisao, atendendo RF-15/ADR-005 (nucleo compartilhado sem duplicacao).
- Arquivos alterados:
  - `internal/agents/application/workflows/onboarding_workflow.go:600-607` (`allocationInputExtract`): adicionado campo `MixedUnit bool \`json:"mixed_unit"\``.
  - `internal/agents/application/workflows/onboarding_workflow.go:660-672` (`allocationInputSchema`): adicionada propriedade `mixed_unit` (boolean) e incluida em `required`, mantendo `additionalProperties: false` (Structured Output estrito preservado).
  - `internal/agents/application/workflows/onboarding_workflow.go:767-778` (`allocationInputSystemPrompt`): adicionada instrucao para o LLM classificar `mixed_unit=true` quando a mesma mensagem misturar reais e percentual entre categorias, espelhando a instrucao ja usada em `distributionIntentSystemPrompt`.
  - `internal/agents/application/workflows/onboarding_workflow.go:1282-1309` (`extractAllocationValues`): assinatura estendida para retornar tambem `mixedUnit bool` (extraido de `input.MixedUnit`); chamada em `handleReviewAwaitDistribution` (caso `distributionIntentValues`) atualizada para descartar o novo retorno com `_` (a deteccao de unidade mista nesse caminho ja ocorre antes, via `classifyDistributionIntent`, sem regressao).
  - `internal/agents/application/workflows/onboarding_workflow.go:1398-1409` (`handleReviewAwaitPersonalize`): adicionado bloco que, quando `mixedUnit=true`, mantem `state.ReviewAwait = reviewAwaitPersonalize`, registra `distributionOutcomeMixedUnit` e responde com a mesma constante `distributionMixedUnitPrompt`, sem ativar nem avancar — preservando o contrato de estado de espera (R-AGENT-WF-001.7: estado de espera do modo personalizar mantido, resume seguinte tratado normalmente).
- Teste de regressao: `TestOnboardingWorkflowSuite/TestBuildBudgetReviewStep/BUG-002_regressao:_resume_em_reviewAwaitPersonalize_com_unidades_mistas_na_mesma_mensagem_deve_pedir_unidade_unica_sem_ativar_nem_avancar` (`internal/agents/application/workflows/onboarding_workflow_test.go`, cenario adicionado na tabela de `TestBuildBudgetReviewStep`) — resposta "custo fixo 40%, prazeres R$ 300" em `reviewAwaitPersonalize`, mock LLM retorna `MixedUnit: true`; assevera prompt igual a `distributionMixedUnitPrompt`, `ReviewAwait` permanece `reviewAwaitPersonalize` (nao avanca) e `Allocations` permanece `nil` (nao ativa).
- Validacao: `go build ./...` PASS; `go vet ./...` PASS; `go test -race ./internal/agents/...` PASS (1285 testes); `go test -tags integration -race ./internal/agents/...` PASS (1378 testes, incluindo `whatsapp_inbound_consumer_integration_test.go` — esse arquivo usa apenas `distributionIntentAcceptExtract` no fluxo mockado, nao afetado pela extensao do schema `allocationInputExtract`/`allocationInputSchema`); golden real-LLM `TestBudgetReviewPersonalizeAndBalanceGate` (8/8 subtests, incluindo `unidades_misturadas_pede_unidade_unica`) e `TestBudgetCreationExtractionRealLLMSuite` (14/14 subtests) PASS; `task lint:run` PASS (0 issues).

### BUG-003

- ID: BUG-003
- Severidade: minor
- Origem: RF-07, achado da skill review
- Estado: fixed
- Causa raiz: `TestSummaryPrompt_ZeroedCategoryWarning` (`internal/agents/application/workflows/onboarding_workflow_test.go`) verificava apenas presenca de substrings genericas (`"zeradas"`, labels de categoria) sem comprovar as duas garantias exigidas por RF-07: (1) o aviso de categorias zeradas aparece uma UNICA vez no prompt (nao duplicado); (2) as categorias zeradas aparecem no resumo com o literal monetario exato `"R$ 0,00 (0%)"`. O teste passaria mesmo se o aviso estivesse duplicado ou o valor formatado estivesse incorreto.
- Arquivos alterados:
  - `internal/agents/application/workflows/onboarding_workflow_test.go:1184-1199` (`TestSummaryPrompt_ZeroedCategoryWarning`): adicionadas asseracoes `s.Equal(1, strings.Count(promptWithZero, "zeradas"), ...)` (unicidade do aviso) e `s.Contains(promptWithZero, "🎓 Conhecimento: R$ 0,00 (0%)", ...)` / `s.Contains(promptWithZero, "🎉 Prazeres: R$ 0,00 (0%)", ...)` (literal monetario exato ao lado do label de cada categoria zerada, confirmando o formato produzido por `renderAllocationLines` em `onboarding_workflow.go:568-576`).
- Teste de regressao: o proprio `TestSummaryPrompt_ZeroedCategoryWarning` reforcado (nao foi necessario criar um teste novo separado — a correcao consistiu em fortalecer as asserçoes do teste existente conforme instrucao da tarefa).
- Validacao: `go build ./...` PASS; `go vet ./...` PASS; `go test -race ./internal/agents/...` PASS (1285 testes, incluindo `TestSummaryPrompt_ZeroedCategoryWarning` reforcado); `task lint:run` PASS (0 issues).

## Comandos Executados

- `go build ./...` -> PASS (sem output, build limpo)
- `go vet ./...` -> PASS (sem output)
- `go test -race ./internal/agents/...` -> PASS: 1285 testes passaram, 0 falhas, 20 pacotes
- `go test -race ./internal/platform/...` -> PASS: 718 testes passaram, 0 falhas, 54 pacotes (verificacao de nao-propagacao de schema para a camada de plataforma)
- `go test -tags integration -race ./internal/agents/...` -> PASS: 1378 testes passaram, 0 falhas, 20 pacotes (inclui `whatsapp_inbound_consumer_integration_test.go`)
- `RUN_REAL_LLM=1 go test -tags integration -run TestOnboardingWorkflowRealLLMSuite/TestBudgetReviewPersonalizeAndBalanceGate ./internal/agents/application/workflows/... -v` (com `.env` OPENROUTER_* carregado) -> PASS: 8/8 subtests (`recusa_entra_em_personalize`, `valores_em_reais_somando_o_orcamento`, `valores_em_percentual_somando_100`, `soma_acima_do_orcamento_permanece_suspenso`, `soma_abaixo_do_orcamento_permanece_suspenso`, `categoria_zerada_aceita_e_fecha_o_total`, `valores_por_extenso`, `tolerancia_absorve_arredondamento_de_percentuais`, `unidades_misturadas_pede_unidade_unica`) — 21.28s
- `RUN_REAL_LLM=1 go test -tags integration -run TestBudgetCreationExtractionRealLLMSuite ./internal/agents/application/workflows/... -v` (com `.env` OPENROUTER_* carregado) -> PASS: 14/14 subtests (`TestBudgetConfirmationSimNaoGate` 5/5, `TestBudgetDistributionExtractionGate` 3/3, `TestBudgetTotalExtractionGate` 5/5) — 10.28s
- `task lint:run` -> PASS: 0 issues; gates adicionais PASS (`lint:auth-bypass`, `lint:outbox-user-id` 7/7, `lint:deadcode`)
- Gate manual `zero comentarios` (R-ADAPTER-001.1): `grep -n "^[[:space:]]*//" internal/agents/application/workflows/onboarding_workflow.go | grep -Ev "(//go:|//nolint:|// Code generated)"` -> vazio (PASS)

## Riscos Residuais

- Nenhum risco residual identificado para os 3 bugs corrigidos dentro do escopo acordado.
- Observacao de design (nao-bug): em `handleReviewAwaitDistribution`, o caminho `distributionIntentValues` agora recebe `mixedUnit` de `extractAllocationValues` mas o descarta (`_`), pois a deteccao de unidade mista nesse caminho ja ocorre antes via `classifyDistributionIntent`/`distributionIntentSchema` (linhas 1360-1364). Isso mantem redundancia proposital e inofensiva (o LLM pode classificar `mixed_unit=true` duas vezes para a mesma mensagem em fluxos diferentes), mas nao introduz duplicacao de logica de decisao — apenas duplicacao de sinal no schema, aceitavel dado que o schema `allocationInputSchema` e compartilhado com `budget_creation_workflow.go`, que tambem ignora esse campo sem impacto.
- Gate de validacao do relatorio executado: `bash .agents/scripts/validate-bugfix-evidence.sh --rf RF-06 --rf RF-07 --rf RF-10 --rf RF-15 .specs/prd-distribuicao-personalizada-onboarding/bugfix_report.md` -> "Validacao do pacote de evidencias de bugfix aprovada" (script aceita multiplos `--rf` diretamente, sem necessidade de invocacao alternativa).
