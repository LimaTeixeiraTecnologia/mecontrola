# Tarefa 8.0: Implementar tool de leitura query_income_summary

<critical>Ler o plano-fonte `docs/plans/2026_06_24_arquitetura_agente_mastra_workflows_bounded_contexts.md` (seção "Exemplo Completo — Adicionar uma Nova Tool de Leitura", Passos 1–9) antes de iniciar.</critical>

## Visão Geral

Adicionar a capacidade de consulta read-only `query_income_summary` ("Quanto recebi esse mês?") seguindo o padrão Mastra aprovado (Opção A): novo `intent.Kind` (não-write), schema, interface, tool fina, binding sobre o use case `ListTransactions`, registro no workflow e injeção no módulo. Valida end-to-end que o padrão `Tool → binding → usecase` é o caminho único para novas capacidades — sem novo `case` de domínio.

<requirements>
- `KindQueryIncomeSummary` (não-write) com smart constructor; `IsWrite()==false` (não passa por WriteGuard/HITL).
- `ParseInbound` reconhece o kind; `ParseIntentJSONSchema` inclui `query_income_summary` no enum.
- Tool fina: zero regra de negócio, SQL ou branching de domínio (R-AGENT-WF-001.2); LLM apenas no parse (R-AGENT-WF-001.4).
- Binding reusa `ListTransactions` do bounded context `transactions`; agregação complexa (se necessária) vive em `internal/transactions`, não no agente.
- Registro via `routableKinds`/`buildRegistry` — nenhum `case intent.Kind` novo em `daily_ledger_agent.go` (R-AGENT-WF-001.1).
- Execução observável como Run (R-AGENT-WF-001.5); `ToolOutcome`/`Kind` tipados (R-AGENT-WF-001.3).
- O teste exaustivo da Task 7.0 é estendido para incluir o novo kind (deve permanecer verde).
- Zero comentários em Go de produção (R-ADAPTER-001.1).
</requirements>

## Subtarefas

- [ ] 8.1 Novo `intent.Kind` + smart constructor `NewQueryIncomeSummary` (`domain/intent/intent.go`); atualizar `String()`/`ParseKind()`/`IsWrite()`.
- [ ] 8.2 `ParseInbound` (`build`) reconhece o kind; `ParseIntentJSONSchema` adiciona o slug ao enum.
- [ ] 8.3 Interface `IncomeSummaryReader` + DTOs (`tools/contracts.go`).
- [ ] 8.4 Tool fina `QueryIncomeSummary` (`tools/income_tools.go`).
- [ ] 8.5 Binding `incomeSummaryReaderAdapter` (`infrastructure/binding/income_summary.go`) reusando `ListTransactions`.
- [ ] 8.6 Registrar a tool no workflow `transactions` (`agent_workflows.go`) e em `routableKinds`.
- [ ] 8.7 Injetar dependência no módulo (`module.go`, `IntentRouterDeps` + `attachIncomeSummaryReader`).
- [ ] 8.8 Estender o teste exaustivo da Task 7.0 (novo kind no schema/builder).

## Detalhes de Implementação

Ver plano-fonte, Passos 1–9 (com exemplos de código a adaptar, não copiar literalmente). Pré-condições: Task 5.0 (registry canônica é fonte única — o registro do novo kind segue o padrão consolidado) e Task 7.0 (teste exaustivo que deve incluir o novo kind). Reusar `WithReadRetry`, `Recorder` e `currentCompetence` já existentes nos tools de leitura.

## Critérios de Sucesso

- "Quanto recebi esse mês?" roteia para `KindQueryIncomeSummary`, executa sem WriteGuard, retorna resumo formatado.
- Tool sem regra/SQL/branching; binding reusa `ListTransactions`.
- Teste exaustivo (7.0) verde com 22 kinds.

## Definition of Done (DoD)

1. Kind, schema, parse, interface, tool, binding, registro e injeção implementados.
2. `IsWrite()==false`; não passa por guard/HITL.
3. Sem `case intent.Kind` novo no switch de `daily_ledger_agent.go`.
4. Teste exaustivo da Task 7.0 estendido e verde; testes da tool (sucesso/erro/resolver ausente) e integração presentes.
5. Build + suites verdes; gates de adapter/agent limpos.

## Critérios de Aceite (gates executáveis)

```bash
cd /Users/jailtonjunior/Git/mecontrola

grep -rn "KindQueryIncomeSummary\|query_income_summary" internal/agent --include="*.go" \
  | grep -E "intent.go|prompts.go|parse_inbound|income_tools|income_summary|agent_workflows" \
  && echo "OK: pontos de extensão presentes" || echo "FAIL: extensão incompleta"

# Switch de domínio não cresceu (R-AGENT-WF-001.1)
f=internal/agent/application/services/daily_ledger_agent.go
cases=$(grep -cE "^[[:space:]]*case intent\.Kind" "$f" || true)
[ "${cases:-0}" -gt 1 ] && echo "FAIL: switch cresceu" || echo "OK"

# Sem SQL direto em tools/workflow (R-AGENT-WF-001.2)
grep -rn --exclude-dir=mocks --exclude="*_test.go" "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
  internal/agent/application/tools/ internal/agent/application/workflow/ \
  && echo "FAIL: SQL direto em tool/workflow" || echo "OK"

# Zero comentários
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" "^[[:space:]]*//" \
  internal/agent/application/tools/ internal/agent/infrastructure/binding/ \
  | grep -Ev "(//go:|//nolint:|// Code generated)" && echo "FAIL: comentários proibidos" || echo "OK"

go build ./... && go test ./internal/agent/...
```

## Skills Necessárias

- `go-implementation` — nova tool/binding/kind em Go de produção (Etapas 1–5 + checklist R0–R7).
- `mastra` — fluxo canônico `IntentRouter → Workflow → Tool → binding → usecase`, nova capacidade como Tool/Kind (R-AGENT-WF-001.1/.2/.4).

## Testes da Tarefa

- [ ] Testes unitários: `NewQueryIncomeSummary` produz o kind; `ParseInbound` com `"kind":"query_income_summary"`; tool com mock `IncomeSummaryReader` (sucesso, erro de use case, resolver ausente).
- [ ] Testes de integração: "Quanto recebi esse mês?" → resposta formatada.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`.</critical>

## Arquivos Relevantes
- `internal/agent/domain/intent/intent.go`
- `internal/agent/application/usecases/parse_inbound.go`
- `internal/agent/application/prompting/prompts.go`
- `internal/agent/application/tools/contracts.go`, `income_tools.go`
- `internal/agent/infrastructure/binding/income_summary.go`
- `internal/agent/application/services/agent_workflows.go`
- `internal/agent/module.go`
- `internal/agent/domain/intent/intent_registry_test.go` (estender)
