# Relatorio de Bugfix

- Total de bugs no escopo: 1
- Corrigidos: 1
- Testes de regressao adicionados: 1
- Pendentes: []
- Estado final: done

## Bugs
- ID: BUG-ONBOARDING-BUDGET-INCOME-LOOP
- Severidade: major
- Origem: finding de review
- Estado: fixed
- Causa raiz: sessao ativa de configuracao de orcamento dependia exclusivamente de `budget_total_cents` e `budget_allocations` extraidos pelo parser LLM; quando o usuario respondia apenas com a renda em texto puro e o parser nao devolvia `configure_budget`, o `change` ficava vazio e o fluxo repetia a pergunta da renda.
- Arquivos alterados: `internal/agent/application/services/daily_ledger_agent.go`, `internal/agent/application/services/intent_router_budget_config_test.go`
- Teste de regressao: `TestPendingSessionPlainIncomeTextFallsBackDeterministically`
- Validacao: fallback deterministico para `workflow.ParseMoneyCents(text)` apenas no caminho de sessao ativa de orcamento quando o parser nao trouxe renda nem alocacoes.

## Comandos Executados
- `gofmt -w internal/agent/application/services/daily_ledger_agent.go internal/agent/application/services/intent_router_budget_config_test.go` -> ok
- `go test -count=1 ./internal/agent/application/services` -> ok
- `go vet ./internal/agent/application/services` -> ok
- `golangci-lint run ./internal/agent/application/services/...` -> ok
- `go build ./internal/agent/application/services` -> ok

## Riscos Residuais
- O transcript original ainda diverge do onboarding canônico atual do repositório; esta correção resolve a causa raiz do loop de renda no fluxo de orçamento pendente, mas não prova qual variante de prompt estava ativa em produção no incidente original.
