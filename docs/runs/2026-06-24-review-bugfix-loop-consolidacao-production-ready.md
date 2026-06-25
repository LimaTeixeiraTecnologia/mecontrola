# Prompt Mandatório — Review + Bugfix Loop até APPROVED (Consolidação Mastra/Workflows)

- **Data:** 2026-06-24
- **Bundle de referência:** `.specs/prd-arquitetura-agente-consolidacao/` (tasks 1.0…8.0)
- **Plano-fonte:** `docs/plans/2026_06_24_arquitetura_agente_mastra_workflows_bounded_contexts.md`
- **Skills:** `review` (ciclo de revisão) + `bugfix` (remediação por achado)
- **Critério de encerramento:** veredito `APPROVED` real — zero achados `critical`/`high`/`medium`, todos os gates R-* verdes com saída executada, todos os DoDs verificados contra o código, suíte global verde.

> **Uso:** cole o bloco **PROMPT PRONTO PARA USO** como mensagem inicial de uma sessão dedicada.
> O loop só pode ser encerrado quando o veredito for genuinamente `APPROVED` — não `APPROVED_WITH_REMARKS`,
> não "aparentemente ok". Toda evidência deve ser saída real de comando, nunca descrição.

---

## PROMPT PRONTO PARA USO

```text
Execute ciclos iterativos de `review` + `bugfix` sobre o resultado das 8 tasks do bundle
.specs/prd-arquitetura-agente-consolidacao/ até o veredito ser APPROVED de forma inegociável.

─────────────────────────────────────────────────────────────────
OBJETIVO INEGOCIÁVEL
─────────────────────────────────────────────────────────────────
Cada ciclo termina com veredito canônico da skill `review`:
  APPROVED               → encerrar; relatório final obrigatório.
  APPROVED_WITH_REMARKS  → tratar como REJECTED; executar bugfix; nova rodada.
  REJECTED               → executar bugfix para cada achado; nova rodada.
  BLOCKED                → resolver o bloqueio antes de nova rodada.

Não há limite de rodadas. O loop só para em APPROVED genuíno.
Falso positivo (marcar APPROVED sem evidência real) invalida toda a sessão.

─────────────────────────────────────────────────────────────────
ESCOPO DE REVISÃO — o que revisar em cada ciclo
─────────────────────────────────────────────────────────────────
Revisar TODOS os arquivos alterados pelas 8 tasks. Pontos obrigatórios de leitura:

Produção:
  internal/agent/application/services/daily_ledger_agent.go
  internal/agent/application/services/agent_workflows.go
  internal/agent/application/services/intent_router.go
  internal/agent/application/workflow/composite.go
  internal/agent/application/workflow/transactions_write.go (ou equivalente)
  internal/agent/application/workflow/destructive_confirm.go (ou equivalente)
  internal/agent/application/workflow/steps/ (todos os steps)
  internal/agent/application/tools/income_tools.go
  internal/agent/application/tools/contracts.go
  internal/agent/application/tools/formatting.go         (formatIncomeSummary)
  internal/agent/infrastructure/binding/income_summary.go
  internal/agent/domain/intent/intent.go
  internal/agent/application/prompting/prompts.go        (JSON Schema)
  internal/agent/application/usecases/parse_inbound.go
  internal/agent/module.go
  configs/config.go                                      (TransactionsWriteEnabled default)
  cmd/server/server.go
  cmd/worker/worker.go

Testes críticos (revisar correctness, não apenas compilação):
  internal/agent/application/workflow/parity_test.go     (rede de segurança kernel)
  internal/agent/application/services/intent_router_test.go
  internal/agent/application/services/hitl_routing_test.go
  internal/agent/application/services/hitl_budget_gate_test.go
  internal/agent/domain/intent/intent_registry_test.go   (22 kinds exaustivos)
  internal/agent/e2e/module_test.go
  internal/agent/application/services/kernel_e2e_test.go
  internal/agent/infrastructure/binding/income_summary_test.go (se existir)
  internal/agent/application/tools/income_tools_test.go   (se existir)

─────────────────────────────────────────────────────────────────
REGRAS DE GOVERNANÇA (HARD — executar gate, não descrever)
─────────────────────────────────────────────────────────────────
Toda regra abaixo deve ser verificada com saída real de comando antes de cada veredito.
Gate que falha = achado `critical` bloqueante, independente de o restante estar ok.

R-WF-KERNEL-001.1 — kernel sem import de domínio:
  grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
    "internal/agent\|internal/transactions\|internal/billing\|internal/identity" \
    internal/platform/workflow/ \
    && echo "FAIL" || echo "OK"

R-WF-KERNEL-001.2 — sem SQL fora do adapter postgres no kernel:
  grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
    "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
    internal/platform/workflow/ \
    | grep -v "infrastructure/postgres" \
    && echo "FAIL" || echo "OK"

R-WF-KERNEL-001.3 — estados como tipos fechados (sem string solta):
  grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
    "RunStatus\s*=\s*\"[^\"]*\"\|StepStatus\s*=\s*\"[^\"]*\"\|SuspendReason\s*=\s*\"[^\"]*\"" \
    internal/platform/workflow/ \
    && echo "FAIL" || echo "OK"

R-WF-KERNEL-001.4 — sem label de alta cardinalidade em métricas do kernel:
  grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
    '"user_id"\|"correlation_key"\|"category_id"' \
    internal/platform/workflow/ \
    && echo "FAIL" || echo "OK"

R-WF-KERNEL-001.5 — sem LLM no kernel:
  grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
    "openai\|anthropic\|openrouter\|gemini\|mistral\|llm\|ParseInbound\|FallbackChain\|CircuitBreaker" \
    internal/platform/workflow/ \
    && echo "FAIL" || echo "OK"

R-WF-KERNEL-001.7 — merge-patch no resume (não substitui estado inteiro):
  grep -n "current = rs\|current = decoded\|current = resumed" \
    internal/platform/workflow/engine.go \
    && echo "FAIL" || echo "OK"

R-ADAPTER-001.1 — zero comentários em Go de produção:
  grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" "^[[:space:]]*//" \
    internal/agent/ internal/platform/workflow/ \
    | grep -Ev "(//go:|//nolint:|// Code generated)" \
    && echo "FAIL" || echo "OK"

R-ADAPTER-001.2 — sem SQL em tools/workflow (agent):
  grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
    "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
    internal/agent/application/tools/ \
    internal/agent/application/workflow/ \
    && echo "FAIL" || echo "OK"

R-AGENT-WF-001.1 — switch de domínio não cresceu:
  f=$(find internal/agent -name "daily_ledger_agent.go" ! -name "*_test.go")
  cases=$(grep -cE "^[[:space:]]*case intent\.Kind" "$f" || true)
  [ "${cases:-0}" -gt 1 ] && echo "FAIL: switch cresceu (cases=$cases)" || echo "OK"

R-AGENT-WF-001.3/.7-A — tipos fechados sem string solta:
  grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
    "AwaitingApproval\s*=\s*\"[^\"]*\"\|OperationKind\s*=\s*\"[^\"]*\"\|ToolOutcome\s*=\s*\"[^\"]*\"\|RunStatus\s*=\s*\"[^\"]*\"" \
    internal/agent/ \
    && echo "FAIL" || echo "OK"

R-AGENT-WF-001.7 — resume HITL antes do parse (ordem determinística):
  f=$(find internal/agent -name "daily_ledger_agent.go" ! -name "*_test.go")
  grep -n "continuePendingExpenseConfirmation\|continuePendingApproval\|parser\.Parse\|ParseInbound" "$f" \
    | grep -v "_test" | head -10
  # Verificar manualmente: continuePendingExpenseConfirmation deve aparecer
  # antes de continuePendingApproval, e ambos antes de qualquer chamada ao parser.

─────────────────────────────────────────────────────────────────
DoD E CRITÉRIOS DE ACEITE POR TASK (verificar TODOS contra o código real)
─────────────────────────────────────────────────────────────────
Ler cada task file e confrontar CADA item do DoD contra o diff/código atual.
Um item não atendido = achado `high` bloqueante. Nenhuma exceção.

Task 1.0 — DoD:
  1. `WORKFLOW_KERNEL_TRANSACTIONS_WRITE_ENABLED` default `true` em configs/config.go.
  2. Não existe caminho em que `confirmEngine` seja nil e um destrutivo execute.
  3. Nenhum teste de produção depende de `TransactionsWriteEnabled=false`.
  4. Build, vet e suites verdes.

Task 2.0 — DoD:
  1. `continuePendingExpenseConfirmationLegacy` e executores de draft via side-store ausentes.
  2. Side-store não é fonte de verdade no resume (snapshot do kernel é fonte única).
  3. `matchesExpenseConfirmation` tem definição única (sem duplicação).
  4. Card purchase via `ForceCategory` preservado e testado.
  5. `parity_test.go` adaptado e verde.

Task 3.0 — DoD:
  1. `newWriteGuard`/`WriteGuard`/guard no composite inexistentes.
  2. Cadeia authorize/replay/policy/audit existe APENAS como steps do kernel.
  3. `composite` serve leituras sem guard.
  4. Nenhum decision audit fica sem `settle`.
  5. `parity_test.go` verde.

Task 4.0 — DoD:
  1. Ramo de bypass sem confirmação removido (não existe path `confirmEngine == nil` + destrutivo).
  2. Caminho único de execução por operação destrutiva.
  3. `dispatchWriteDestructive`/`wireBudgetCommitGate` preservados (são o canônico).
  4. `ConfirmState` persistido antes da pergunta; resume antes do parse; sem run suspenso órfão.
  5. `AwaitingApproval`/`OperationKind` tipos fechados; sem string solta.

Task 5.0 — DoD:
  1. `RenderSystemPrompt` (e constantes órfãs) ausentes em produção e testes.
  2. Fonte única de roteamento por kind = registry canônica.
  3. Teste de paridade specs↔kinds presente e verde.
  4. Build + suites verdes.

Task 6.0 — DoD:
  1. `AgentModuleOption` e `With*` ausentes.
  2. `SessionStore` obrigatório; `OnboardingLLM` nullable explícito (nunca obrigatório).
  3. `cmd/server` e `cmd/worker` compilando e com comportamento preservado.
  4. Build dos dois entrypoints + suites verdes.

Task 7.0 — DoD:
  1. `intent_registry_test.go` presente e cobre: round-trip, slug não-default, paridade
     bidirecional schema↔kind, builders exaustivos para todos os kinds.
  2. `allKinds()` (ou equivalente) deriva de fonte única (range `iota`), não lista manual.
  3. Paridade bidirecional implementada: todo kind no código está no schema e vice-versa.
  4. `go test ./internal/agent/domain/intent/...` verde.

Task 8.0 — DoD:
  1. Kind, schema, parse, interface, tool, binding, registro e injeção implementados.
  2. `KindQueryIncomeSummary.IsWrite() == false`; não passa por guard/HITL.
  3. Sem `case intent.Kind` novo no switch de `daily_ledger_agent.go`.
  4. Teste exaustivo da Task 7.0 cobre 22 kinds (incluindo `KindQueryIncomeSummary`).
  5. Testes da tool (sucesso/erro/resolver ausente) e do binding presentes e verdes.

─────────────────────────────────────────────────────────────────
CHECKLIST DE CORRECTNESS — revisar por arquivo (não apenas compilação)
─────────────────────────────────────────────────────────────────
Para cada arquivo do escopo, verificar:

parity_test.go:
  - Todos os cenários do kernel (autoLog, authzDenied, replay, policy, ambiguousChoiceResume,
    needsConfirmResume, needsConfirmCancel, usecaseError, auditConflict, auditFail,
    missingResolver) estão presentes e cobrem o comportamento correto.
  - `runKernelResume` verifica estado APÓS resume, não antes.
  - `parityStore` implementa corretamente `Load`/`Save`/`Insert`/`AppendStep`/`DeleteCompleted`.
  - Nenhum cenário testa comportamento de código legacy removido (regressão de expectativa).

income_summary.go (binding):
  - Filtra `direction == "income"` corretamente (atualmente faz no loop do adapter).
  - `Count` em `IncomeSummaryResult` — verificar se o campo existe na struct e se o binding
    preenche corretamente (ou se é derivado de `len(Sources)` em `formatIncomeSummary`).
  - `withWhatsAppPrincipal` é compatível com o contexto de autorização do use case `ListTransactions`.
  - Erro de `uuid.Parse` retorna erro tipado, não panic.

income_tools.go (tool):
  - `formatIncomeSummary` existe em `formatting.go` e formata corretamente (não é stub).
  - `WithReadRetry` existe e é o helper correto para leituras.
  - `OutcomeMissingResolver` retornado quando `reader == nil`.
  - `Descriptor().Description` não é `"query_income_summary"` (string técnica exposta ao usuário)
    — verificar se deve ser texto amigável em PT-BR.

intent_registry_test.go:
  - `allKinds()` ou equivalente itera `KindUnknown..KindQueryIncomeSummary` via range `iota`,
    não lista manual que pode divergir.
  - Teste de paridade bidirecional: (a) todo Kind do código tem slug no schema; (b) todo slug
    do schema tem Kind no código — AMBAS direções.
  - Prova de eficácia documentada: teste falha ao remover um slug do schema, passa ao reverter.

agent_workflows.go (registry/routableKinds):
  - `routableKinds()` inclui `KindQueryIncomeSummary`.
  - `buildRegistry()` registra a tool `QueryIncomeSummary` no workflow correto (`transactions`).
  - `warnMissingToolBindingsKinds()` (ou equivalente) retorna lista que inclui
    `KindQueryIncomeSummary` — binding ausente gera warning.

module.go:
  - `AgentModuleOption` e `With*` ausentes (DoD 6.0).
  - `sessionDB` é parâmetro obrigatório direto (não optional).
  - `OnboardingLLM` é campo nullable (não obrigatório).
  - Wiring de `IncomeSummaryReader` presente e liga `ListTransactionsUC` ao binding.

─────────────────────────────────────────────────────────────────
PROTOCOLO DE CICLO (obrigatório, sem atalho)
─────────────────────────────────────────────────────────────────
Ciclo N:
  1. Registrar `git rev-parse HEAD` (SHA_N) como âncora de diff.
  2. Invocar skill `review` sobre o escopo completo definido acima.
     - Exportar `AI_REVIEW_PRIOR_SHA=<SHA_N-1>` quando N > 1 (revisar apenas delta de bugfix).
     - Executar TODOS os gates R-* com saída real e colar output.
     - Confrontar CADA item do DoD de cada task contra o código.
     - Revisar correctness dos arquivos críticos listados acima.
  3. Coletar veredito canônico.
  4. Se `REJECTED` ou `APPROVED_WITH_REMARKS`:
     a. Para cada achado, invocar skill `bugfix` com o bug em formato canônico
        (`.agents/skills/agent-governance/references/bug-schema.json`).
     b. Executar `go build ./... && go test ./...` após cada bugfix; registrar saída real.
     c. Voltar ao passo 1 com N+1.
  5. Se `BLOCKED`: resolver bloqueio explicitamente antes de nova rodada.
  6. Se `APPROVED`: ir para Encerramento.

─────────────────────────────────────────────────────────────────
SUÍTE FINAL (obrigatória antes de APPROVED)
─────────────────────────────────────────────────────────────────
Antes de emitir APPROVED, executar e colar saída real de:

  go build ./...
  go vet ./internal/agent/... ./internal/platform/workflow/...
  go test -count=1 ./internal/agent/... ./internal/platform/workflow/...
  go test -count=1 ./...   (suíte global — 0 FAIL obrigatório)

E todos os gates R-* listados acima, com saída real de cada comando.

─────────────────────────────────────────────────────────────────
ENCERRAMENTO — relatório final obrigatório
─────────────────────────────────────────────────────────────────
Ao atingir APPROVED, emitir relatório com:
  - Número de ciclos executados e SHA de cada rodada.
  - Lista de achados corrigidos por ciclo (severidade, arquivo, linha, fix aplicado).
  - Saída real de todos os gates R-* (OK/FAIL com output do comando).
  - Saída real de `go build ./...` e `go test ./...`.
  - Riscos residuais conhecidos e suposições assumidas.
  - Veredito final: APPROVED — pronto para main.

─────────────────────────────────────────────────────────────────
NÃO-OBJETIVOS (não fazer)
─────────────────────────────────────────────────────────────────
  - Não refatorar além do necessário para atingir APPROVED.
  - Não introduzir features, abstrações ou dependências novas.
  - Não commitar nem abrir PR sem pedido explícito do usuário.
  - Não marcar APPROVED com base em "aparentemente correto" — toda afirmação
    deve ter evidência de comando executado.
  - Não flexibilizar nenhuma regra R-* por pressa, conveniência ou deadline.
  - Não aceitar APPROVED_WITH_REMARKS como encerramento.
```

---

## Critérios de aceite do próprio run (como saber que o prompt foi cumprido)

1. Veredito final da skill `review` = `APPROVED` (não `APPROVED_WITH_REMARKS`).
2. Todos os gates R-* com saída real `OK` (R-WF-KERNEL-001.1..7, R-ADAPTER-001.1/.2, R-AGENT-WF-001.1/.2/.3/.7/.7-A).
3. Suíte global `go test ./...` com 0 FAIL e `go build ./...` limpo.
4. DoD de cada task (1.0…8.0) verificado item a item contra o código com evidência.
5. `parity_test.go` verde e semanticamente correto (cenários cobrem comportamento real).
6. `intent_registry_test.go` com 22 kinds; paridade bidirecional schema↔kind provada.
7. `income_summary.go` e `income_tools.go` sem lacunas de correctness.
8. Relatório final com ciclos, achados corrigidos, gates e riscos residuais — sem lacuna crítica conhecida.

## Referências

- Bundle: `.specs/prd-arquitetura-agente-consolidacao/tasks.md` + `task-1.0…task-8.0`
- Relatórios de execução: `.specs/prd-arquitetura-agente-consolidacao/1.0_execution_report.md`…`8.0_execution_report.md`
- Plano-fonte: `docs/plans/2026_06_24_arquitetura_agente_mastra_workflows_bounded_contexts.md`
- Regras: `.claude/rules/{agent-workflows-tools,workflow-kernel,go-adapters,go-testing,governance}.md`
- ADRs: `.specs/prd-agent-platform-evolution/adr-00{1,2,3,4}-*.md`
- Bug schema: `.agents/skills/agent-governance/references/bug-schema.json`
- Severity mapping: `.agents/skills/agent-governance/references/severity-mapping.md`
