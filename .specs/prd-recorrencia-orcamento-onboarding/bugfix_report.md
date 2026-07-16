# Relatorio de Bugfix

- Total de bugs no escopo: 2
- Corrigidos: 2
- Testes de regressao adicionados: 1 novo (BUG-REVIEW-02) + 1 teste existente estendido (BUG-REVIEW-01)
- Pendentes: nenhum
- Estado final: done

## Rodada 2 (review 2026-07-16 — foco 0 regressao e production-ready)
- ID: BUG-REVIEW-02
- Severidade: minor (low) — lacuna de cobertura de teste
- Origem: finding de review confrontado contra PRD RF-16 (objetivo "Observabilidade de produto: 100% das saidas do step sao contabilizadas") e contra o relatorio 4.0_execution_report.md, que prometeu explicitamente que a tarefa 5.0 trocaria o `nil` dos testes pelo counter/asseçoes de outcome — promessa nao cumprida.
- Estado: fixed
- Causa raiz: o counter `agents_onboarding_recurrence_total` e chamado corretamente nos 5 desfechos em producao (`onboarding_workflow.go` linhas 1801/1804/1808/1823), mas nenhum teste comprovava o incremento com o rotulo `outcome` correto, ao contrario dos steps irmaos `monthly_budget` (`TestBuildMonthlyBudgetStep_OutcomeMetric`) e `distribution` (`TestBuildBudgetReviewStep_DistributionOutcomeMetric`), que possuem testes de metrica dedicados. `TestBuildRecurrenceStep` passa `nil` como counter, nao exercitando a instrumentacao.
- Arquivos alterados: `internal/agents/application/workflows/onboarding_workflow_test.go` (novo `TestBuildRecurrenceStep_OutcomeMetric`, espelhando byte-a-byte o padrao do teste de metrica de distribution: `fake.NewProvider()` + `GetCounter` + assert `outcome`), cobrindo os 5 desfechos `no_recurrence`/`default_12`/`specific_months`/`invalid_reprompt`/`ambiguous_reprompt`.
- Teste de regressao: `TestBuildRecurrenceStep_OutcomeMetric` (5 subcenarios, PASS) — prova que cada desfecho incrementa o counter uma vez com `Fields[0].Key=="outcome"` e o valor exato de `recurrenceOutcomeKind.String()`.
- Validacao: `gofmt -l` (limpo); `go test ./internal/agents/application/workflows/ -race -run TestBuildRecurrenceStep_OutcomeMetric -v` (5/5 PASS); `go test ./internal/agents/... -race` (13 pacotes, 0 falhas); `golangci-lint run ./internal/agents/application/workflows/...` (0 issues); gate real-LLM `RUN_REAL_LLM=1 ...TestRecurrenceExtractionGate` reexecutado independentemente (hits=18 total=18 ratio=1.0000 falso_sucesso=0, modelo openai/gpt-4o-mini).
- Nota de isolamento: a unica falha em `task lint:run` (`lint:deadcode` -> `BuildBudgetManageWorkflow` em `budget_manage_workflow.go`) pertence a feature `operacao-conversacional-diaria` presente no working tree sujo, NAO ao escopo da recorrencia; nenhum simbolo da recorrencia e sinalizado como codigo morto.

## Bugs
- ID: BUG-REVIEW-01
- Severidade: minor
- Origem: finding de review (docs/reviews/2026-07-15-review-prd-recorrencia-orcamento-onboarding.md), confrontado contra `.specs/prd-recorrencia-orcamento-onboarding/techspec.md` secao "Testes existentes a atualizar" (obrigacao de incluir "confirmacoes" no map de superficies de `TestM02_NoRendaTermInAnyOnboardingSurface`)
- Estado: fixed
- Causa raiz: o mapa `surfaces` em `TestM02_NoRendaTermInAnyOnboardingSurface` (guard-rail que varre todas as superficies de mensagem do onboarding contra o termo proibido "renda") foi atualizado para incluir `recurrenceConfirmationNone` e `recurrenceConfirmationDefault`, mas nao incluiu a superficie dinamica gerada por `recurrenceConfirmationFor(months)` para N != 12 (que usa `recurrenceConfirmationTemplate` interpolado via `monthsLabel`) — uma superficie de mensagem nova, visivel ao usuario, ficou fora da varredura do guard-rail.
- Arquivos alterados: `internal/agents/application/workflows/onboarding_workflow_test.go` (linha 4289, adicionada entrada `"recurrenceConfirmationFor(3)": recurrenceConfirmationFor(3)` ao mapa `surfaces`)
- Teste de regressao: reaproveita o teste existente `TestM02_NoRendaTermInAnyOnboardingSurface`, que agora tambem varre a superficie dinamica `recurrenceConfirmationFor(3)`.
- Validacao: `go build ./...` (sem erros); `go vet ./...` (sem erros); `go test ./internal/agents/application/workflows/... -race -run TestOnboardingWorkflowSuite` (269 subtestes, PASS); `go test ./internal/agents/... -race` (suite completa do modulo, PASS, sem regressao); `task lint:run` (0 issues, todos os gates PASS).

## Comandos Executados
- `go build ./...` -> sem erros
- `go vet ./...` -> sem erros
- `go test ./internal/agents/application/workflows/... -race -run TestOnboardingWorkflowSuite` -> PASS (269 subtestes)
- `go test ./internal/agents/... -race` -> PASS (todos os pacotes do modulo, sem regressao)
- `task lint:run` -> 0 issues; `lint:auth-bypass`, `lint:outbox-user-id`, `lint:deadcode` todos PASS

## Riscos Residuais
- Nenhum risco critico novo. Risco residual documentado (nao bloqueante, fora do escopo desta correcao): a fixture full-flow WhatsApp (`whatsapp_inbound_consumer_integration_test.go`) usa um stub simples de `BudgetPlanner` (nao um mock estrito) e portanto nao oferece, por si so, uma garantia estrutural de "CreateRecurrence nao chamado" no cenario negativo daquele fluxo especifico — essa garantia estrutural forte ja existe no gate dedicado `TestRecurrenceExtractionGate` (RF-17/ADR-004), que e o instrumento exigido pela especificacao para essa verificacao. Nenhum RF, ADR ou task exige essa asserção adicional na fixture E2E.
