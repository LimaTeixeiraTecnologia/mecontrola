# Relatorio de Bugfix

- Total de bugs no escopo: 1
- Corrigidos: 1
- Testes de regressao adicionados: 0 novos (teste existente estendido)
- Pendentes: nenhum
- Estado final: done

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
