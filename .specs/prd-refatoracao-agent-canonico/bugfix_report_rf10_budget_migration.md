# Relatorio de Bugfix

- Total de bugs no escopo: 1
- Corrigidos: 1
- Testes de regressao adicionados: 5
- Pendentes: nenhum
- Estado final: done

## Bugs
- ID: RF-10-GAP-08
- Severidade: major
- Origem: RF-10 (techspec errata #8, `.specs/prd-refatoracao-agent-canonico/techspec.md`)
- Estado: fixed
- Causa raiz: `ConfigureBudgetConversation.Execute` (`internal/agent/application/usecases/configure_budget_conversation.go:83`) invocava `interpreter.Interpret` em tempo de EXECUCAO/RESUME (alcancado via `daily_ledger_agent.go` budget runner -> `tools/budget_session.go` -> `convo.Configure`), constituindo um 4o call-site de LLM fora do conjunto sancionado {parse, onboarding, conversacional KindUnknown}. A errata #8 decidiu MIGRAR a extracao para o parse estruturado; apenas `Strict:true` havia sido adicionado a chamada de execucao, sem migrar.
- Arquivos alterados:
  - `internal/agent/domain/intent/intent.go` (Intent carrega `budgetTotalCents`/`budgetAllocs`; `NewConfigureBudget(ConfigureBudgetFields)` smart constructor com validacao de slug/basis-points; novos erros tipados)
  - `internal/agent/application/prompting/prompts.go` (parse schema estendido com `budget_total_cents`/`budget_allocations` strict-complete; remocao de `RenderBudgetConfigSystem`/`budgetConfigSystemPrompt`/`BudgetConfigJSONSchema` mortos)
  - `internal/agent/application/prompting/parse_intent.system.tmpl` (guidance de extracao de orcamento: mapeamento PT-BR->slug, exemplos, distincao vs record_income)
  - `internal/agent/application/usecases/parse_inbound.go` (DTO `budget_total_cents`/`budget_allocations`; build de `KindConfigureBudget` com `ConfigureBudgetFields`)
  - `internal/agent/application/usecases/configure_budget_conversation.go` (Execute agora e merge DETERMINISTICO de `budgetdraft.Change`; LLM removido; interpreter/schema removidos)
  - `internal/agent/application/tools/contracts.go` (`BudgetConversation.Configure` recebe `budgetdraft.Change`)
  - `internal/agent/application/tools/budget_session.go` (`Start`/`Resume` recebem `Change`; `Active`/`Cancel` substituem `Continue` para resume pos-parse)
  - `internal/agent/application/tools/budget_tools.go` (tool monta `Change` do intent parseado)
  - `internal/agent/application/services/daily_ledger_agent.go` (`tryResumeInbound` budget = somente cancel; roteamento budget-ativo apos parse em `Handle` via `Resume`)
  - `internal/agent/infrastructure/binding/budget_config.go` (adapter repassa `Change`)
  - `internal/agent/module.go` (`NewConfigureBudgetConversation(o11y)` sem interpreter; `attachBudgetConfigSession` sem `llmModule`)
  - `internal/agent/application/workflow/plan_helpers.go`, `internal/agent/application/usecases/tool_catalog.go` (ajuste de assinatura do construtor)
- Teste de regressao:
  - `internal/agent/application/usecases/configure_budget_conversation_test.go` (5 cenarios deterministicos)
  - `internal/agent/domain/intent/intent_test.go` (`TestNewConfigureBudget` + rejeicao de slug invalido + basis-points fora de faixa)
  - `internal/agent/application/services/intent_router_budget_config_test.go` (continue via change parseada; final commit+clear; commit-error mantem sessao; non-cancel; cancel)
  - `internal/agent/e2e/budget_realllm_test.go` (real-LLM Extracts/MultiTurn adaptados para `ParseInbound`)
  - `internal/agent/application/prompting/schema_strict_test.go` (cobre schema estendido)
- Validacao: `go build ./...` OK; `go test ./internal/agent/...` OK; `go test ./...` sem FAIL; `go vet ./...` limpo; real-LLM budget tests PASS (gemini-flash-lite); `TestParseInbound_RealLLM_ProductionChain` PASS (gemini + mistral).

## Comandos Executados
- `go build ./...` -> OK (exit 0)
- `go test ./internal/agent/...` -> ok (todos os pacotes)
- `go test ./...` -> sem FAIL
- `go vet ./...` -> limpo
- `gofmt -l internal/agent/` -> vazio (PASS)
- `RUN_REAL_LLM=1 go test -tags integration -run TestConfigureBudget_RealLLM ./internal/agent/e2e/...` -> PASS (2 testes)
- `RUN_REAL_LLM=1 go test -tags integration -run TestParseInbound_RealLLM_ProductionChain ./internal/agent/e2e/...` -> ok (48s)
- RF-10 grep budget -> zero matches (PASS)
- switch-growth gate -> 0 (PASS <=1)
- zero-comments gate internal/agent prod -> PASS

## Riscos Residuais
- Robustez do LLM no turno introdutorio "renda sem percentuais": reforcado por exemplos no system prompt e distincao explicita vs record_income; teste real-LLM passa, mas dependencia de modelo permanece sujeita a variacao. Mitigacao: ate 3 tentativas no fluxo/teste e merge deterministico duravel no draft.
- `internal/agent/application/services/observation_memory.go:102` mantem call-site de LLM de WorkingMemory pre-existente (fora do escopo configure_budget e da errata #8); nao e regressao desta tarefa.
