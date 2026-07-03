# Relatorio de Bugfix

- Total de bugs no escopo: 6
- Corrigidos: 6
- Testes de regressao adicionados: 1 (TestRF38_CreateRecurrenceFailed_RunStatusFailedInDB) + resiliência/asserts revalidados
- Pendentes: nenhum
- Estado final: done

## Bugs

- ID: A-01
- Severidade: critical
- Origem: finding de review (RF-29 / RF-30 / M-04 / task-7.0) — docs/reviews/2026-07-02-veredito-prd-mecontrola-agent-tools.md
- Estado: fixed
- Causa raiz: harness real-LLM `mecontrola_tools_realllm_test.go` acessava `tc.Name`/`tc.Args` inexistentes em `agent.ToolCallRecord` (campos reais: `Tool/Outcome/Content`) — não compilava; e, mesmo compilando, `register_card_purchase` e `edit_entry` eram roteados incorretamente pelo LLM (thin fakes + descrições ambíguas).
- Arquivos alterados: `internal/agents/application/scorers/mecontrola_tools_realllm_test.go` (mapeamento `tc.Tool`, retry em erro transitório, descrições representativas dos fakes); `internal/agents/application/agents/mecontrola_agent.go` (regras determinísticas: comprar-no-cartão → register_card_purchase; editar/excluir item identificado → chamar a tool imediatamente).
- Teste de regressao: `TestRealLLM_ToolCoverage_All25Tools` (LLM real).
- Validacao: `RUN_REAL_LLM=1 go test -tags integration -run TestRealLLM_ToolCoverage_All25Tools` → M-04=1.00 (25/25), uncovered=[].

- ID: A-02
- Severidade: critical
- Origem: finding de review (RF-40 / task-0.0)
- Estado: fixed
- Causa raiz: `runtime_substrate_integration_test.go` era `package agent` (white-box) importando `internal/platform/agent/infrastructure/postgres`, que importa de volta `internal/platform/agent` → import cycle; o teste de prova de escrita real nunca compilava.
- Arquivos alterados: `internal/platform/agent/runtime_substrate_integration_test.go` (movido para `package agent_test`, stub agent local, prefixos `agent.`).
- Teste de regressao: `TestSubstrateIntegrationSuite` (RF-38/RF-39 + create_recurrence).
- Validacao: `go test -tags integration -run TestSubstrateIntegrationSuite ./internal/platform/agent/` → PASS (testcontainers pg16); `platform_messages role=tool` COUNT>0.

- ID: A-03
- Severidade: critical
- Origem: finding de review (política de evidência / governance.md)
- Estado: fixed
- Causa raiz: `7.0_execution_report.md` declarava `go build -tags integration → OK` sem que o pacote compilasse (falso-positivo).
- Arquivos alterados: `.specs/prd-mecontrola-agent-tools/7.0_execution_report.md` (secão de retificação com estado real revalidado).
- Teste de regressao: N/A (correção documental); lastro empírico nos testes A-01/A-02/A-06.
- Validacao: claims agora verdadeiros — harness compila e passa; asserts DB passam.

- ID: A-04
- Severidade: major (high)
- Origem: finding de review (RF-38 / RF-35 / ADR-003/005)
- Estado: fixed
- Causa raiz: `WithWriteToolSet` omitia `create_recurrence`, deixando escrita de recorrência falha/alucinada fora do guard anti-simulação.
- Arquivos alterados: `internal/agents/module.go` (`WithWriteToolSet(..., "create_recurrence")`).
- Teste de regressao: `TestRF38_CreateRecurrenceFailed_RunStatusFailedInDB` (novo).
- Validacao: `go test -tags integration -run TestSubstrateIntegrationSuite` → PASS (run com create_recurrence falho vira RunStatusFailed).

- ID: A-05
- Severidade: major (high)
- Origem: finding de review (task-2.0 / task-6.0 / RF-19)
- Estado: fixed
- Causa raiz: `NewTransactionsLedgerAdapter` foi expandido para 14 parâmetros; 3 arquivos de teste de integração continuaram chamando com 9 args → build failed sob `-tags integration`.
- Arquivos alterados: `mecontrola_agent_e2e_test.go`, `ca09_reconciled_integration_test.go`, `transactions_integration_test.go` (5 `nil` adicionados antes de `o11y`).
- Teste de regressao: os próprios `TestMeControlaAgentE2ESuite`, `TestCA09ReconciledIntegrationSuite`, `TestTransactionsIntegrationSuite`.
- Validacao: `go test -tags integration` nessas suites → PASS.

- ID: A-06
- Severidade: major (high)
- Origem: finding de review (M-05 / D-10 / RF-29 / task-7.0)
- Estado: fixed
- Causa raiz: harness canônico usava fake capture tools, sem assert de linha real no banco (M-05).
- Arquivos alterados: N/A de produção — evidência DB coberta por `register_expense_integration_test.go` (já existente, agora compilando junto ao pacote) e por `runtime_substrate_integration_test.go` (A-02).
- Teste de regressao: `TestRegisterExpenseIntegrationSuite/TestIdentityInjectedAndWrittenToLedger`.
- Validacao: `go test -tags integration -run TestRegisterExpenseIntegrationSuite ./internal/agents/application/tools/` → PASS (`agents_write_ledger` COUNT>0 + `platform_messages role=tool`).

## Comandos Executados
- `go vet -tags integration ./internal/agents/... ./internal/platform/agent/...` -> OK (todos os pacotes compilam)
- `go test -count=1 ./internal/agents/... ./internal/platform/agent/...` -> OK (unitários, sem regressão)
- `go build ./... && go vet ./... && go mod verify` -> OK
- `go test -tags integration -run 'TestSubstrateIntegrationSuite|TestRegisterExpenseIntegrationSuite' ...` -> PASS (testcontainers pg16)
- `go test -tags integration -run 'TestCA09...|TestTransactions...|TestMeControlaAgentE2ESuite' ...` -> PASS
- `RUN_REAL_LLM=1 go test -tags integration -run TestRealLLM_ToolCoverage_All25Tools ...` -> PASS, M-04=1.00 (25/25)
- `RUN_REAL_LLM=1 go test -tags integration -run TestRealLLM ...` -> PASS (coverage + EP-01 + EP-05)

## Riscos Residuais
- Seleção de tool via LLM real tem variância inerente; mitigada por (a) instruções determinísticas, (b) descrições de fake representativas, (c) retry bounded a erro transitório de transporte. M-04 observado 1.00; alvo D-03 é ≥0.90.
- A implementação permanece uncommitted e não deployada; a resolução em produção só é comprovável após deploy (o defeito observado nos últimos 7 dias é da imagem antiga `571425f`).
