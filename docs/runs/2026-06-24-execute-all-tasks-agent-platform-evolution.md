# Prompt Mandatório — Executar TODAS as tasks do `prd-agent-platform-evolution` (MVP HITL, production-proof)

- **Data:** 2026-06-24
- **Alvo:** `.specs/prd-agent-platform-evolution/` (PRD + techspec + 4 ADRs + tasks.md + task-1.0…9.0)
- **Skill orquestradora:** `execute-all-tasks` (governance) — espelhada em `.github/skills/execute-all-tasks/`, `.agents/skills/execute-all-tasks/`, `.claude/skills/execute-all-tasks/`
- **Skills obrigatórias por tarefa:** `go-implementation` (toda edição `.go`, inegociável), `agent-governance` (auto)
- **Pré-condição de governança (gate, RF-21–27):** task 1.0 redige addendum `R-AGENT-WF-001.7` (AwaitingApproval) + nota merge-patch em `workflow-kernel.md` ANTES de qualquer `.go` do HITL
- **Cobertura MVP:** RF-08…RF-14 (capacidade B HITL) + RF-21…RF-27 (transversal). RF-01..07 e RF-15..20 = tarefas 8.0/9.0 (roadmap, `pending` intencional — não implementar neste ciclo)
- **Caminho crítico:** 1.0 → (2.0 ∥ 3.0) → 4.0 → 5.0 → 6.0 → 7.0 · **Tarefas adiadas:** 8.0 ∥ 9.0 (`pending` = placeholders de roadmap, sem código)
- **Correção fundacional:** task 2.0 corrige defeito latente de `Engine.Resume` (substituição → merge-patch); pré-requisito de 4.0..7.0

---

## PROMPT PRONTO PARA USO (copie e cole na sessão de execução)

```text
MISSÃO (inegociável): executar TODAS as tarefas do PRD `prd-agent-platform-evolution` até o fim,
entregando o MVP de Human-in-the-Loop (HITL) para ações destrutivas ROBUSTO e PRODUCTION-READY/PROOF,
com 0 gaps, 0 lacunas e 0 falso positivo. Atender FIELMENTE o Definition of Done (DoD) e os Critérios
de Aceite de CADA tarefa, com EVIDÊNCIA verificável.

INVOCAÇÃO CANÔNICA
- Invoque a skill `execute-all-tasks` com o slug: agent-platform-evolution
- O orquestrador roda o pré-voo (hook `pre-execute-all-tasks.sh`, `ai-spec skills --verify`,
  `ai-spec check-spec-drift .specs/prd-agent-platform-evolution/tasks.md`), constrói o DAG a partir
  de `.specs/prd-agent-platform-evolution/tasks.md` e spawna um subagent FRESH por tarefa via
  `execute-task`.
- NUNCA executar `execute-task` inline no orquestrador. Respeitar o contrato YAML estrito
  (`status`, `report_path`, `summary`) e a cadeia de validação (evidência física + consistência de
  tasks.md). Halt-first: qualquer tarefa ≠ done para a wave, gera relatório e encerra.

ORDEM E PARALELISMO (respeitar tasks.md, sem exceção)
- 1.0 é GATE bloqueante: addendum `R-AGENT-WF-001.7` (AwaitingApproval como tipo fechado) +
  nota merge-patch em `workflow-kernel.md` ANTES de qualquer código Go do HITL. Não escrever `.go`
  enquanto 1.0 não estiver `done`.
- Caminho crítico: 1.0 → (2.0 ∥ 3.0) → 4.0 → 5.0 → 6.0 → 7.0.
- Paralelizar SOMENTE 2.0 ∥ 3.0 (kernel plataforma vs tipos do agent — áreas disjuntas) e 8.0 ∥ 9.0
  (ambas adiadas, sem código), apenas se o tool suportar spawn nativo; senão, degradar para
  sequencial e registrar no relatório.
- Tarefas 8.0 e 9.0 são GUARDA-CHUVA DE ROADMAP: permanecem `pending` sem alteração de código de
  produção. Marcar `done` APENAS após confirmar que nenhum código foi alterado e registrar como
  "planejada-não-implementada" no report.

REGRAS DE IMPLEMENTAÇÃO (HARD — não flexibilizar por ferramenta, hook, conveniência ou deadline)

Go (toda edição `.go`):
- Carregar `go-implementation` e cumprir ETAPAS 1–5 + CHECKLIST R0–R7 integralmente.
- Zero comentários em `.go` de produção (R-ADAPTER-001.1); exceções exclusivas: `//go:build`,
  `//go:generate`, `//go:embed`, `//nolint:<razão>`, `// Code generated` (linha 1).
- Sem `init()` (R0), sem `panic` em produção (R5.12), `context.Context` em toda fronteira de IO
  (R6), `errors.Join` + `fmt.Errorf("ctx: %w", err)` (R7).
- Sem abstração de relógio: `time.Now().UTC()` inline no ponto de uso — sem Clock interface, sem
  `now func() time.Time`.
- `defer func() { _ = rows.Close() }()` para rows (bloqueado pelo errcheck do golangci-lint).

Kernel genérico — `internal/platform/workflow` (R-WF-KERNEL-001, HARD):
- SEM import de pacote de domínio (`intent`, `agent`, `transactions`, `billing`, `identity`).
- SEM regra/SQL/branching de domínio fora do adapter Postgres (`infrastructure/postgres/`).
- SEM LLM, prompt rendering, FallbackChain, CircuitBreaker ou ParseInbound.
- Estados TIPOS FECHADOS: `RunStatus`, `StepStatus`, `SuspendReason` — nunca string solta.
- Cardinalidade de métrica controlada: labels apenas `workflow`, `step`, `status`, `outcome` —
  PROIBIDO `user_id`, `correlation_key`, `category_id`.
- task 2.0 ESPECÍFICA: `Codec[S].MergePatch(base, patch []byte) ([]byte, error)` em `codec.go`
  (merge recursivo JSON, `null` remove chave, genérico sem domínio); `Engine.Resume` substitui
  bloco de replace por `MergePatch(snap.State, resume)` + `Decode`; resume vazio = no-op.

Agent — `internal/agent` (R-AGENT-WF-001, HARD):
- NENHUM novo `case intent.Kind` no switch de `daily_ledger_agent.go` (R-AGENT-WF-001.1).
  Roteamento HITL exclusivamente via registry keyed-by-kind dos 4 kinds destrutivos.
- Tool/passos ADAPTERS FINOS: sem regra de negócio, SQL ou branching de domínio (R-AGENT-WF-001.2).
- LLM APENAS no step de parse `ParseInbound` (R-AGENT-WF-001.4) — proibido nos passos HITL.
- `ToolOutcome`, `RunStatus`, `OperationKind`, `AwaitingApproval` = TIPOS FECHADOS com constantes
  enumeradas; nunca string livre em assinatura pública (R-AGENT-WF-001.3 + DMMF state-as-type).
- `continuePendingApproval` ANTES do `ParseInbound`; ordem determinística: categoria → aprovação
  → parse (R-AGENT-WF-001.7 estendido).
- Estado HITL suspenso persiste como `ConfirmState` no snapshot do kernel; sem side-store novo
  em `agent_sessions` para este fluxo.
- Passos de guarda `authorize → replay → policy → audit_begin` REUTILIZADOS 1:1 do
  `transactions_write` (sem duplicar, sem alterar contrato).
- `correlationKey = "<user_id>:<channel>"` — mesmo padrão do fluxo de categoria.
- Workflow ID `"destructive_confirm"` DISTINTO de `"transactions_write"`: resume não-ambíguo.

DMMF / state-as-type (HARD):
- `OperationKind` (`OperationDeleteLast`, `OperationEditLast`, `OperationDeleteCard`,
  `OperationBudgetCommit`) com `String()`, `IsValid()`, `ParseOperationKind`.
- `AwaitingApproval` (`AwaitingNone`, `AwaitingConfirm`) com `String()`, `IsValid()`,
  `ParseAwaitingApproval`.
- PROIBIDOS em assinaturas públicas: `Result[T,E]` customizado, currying, DSL de pipeline, monads.

Adapters (R-ADAPTER-001.2, HARD):
- `prepare_target.go` e `execute_destructive.go` despacham por `map[OperationKind]TargetResolver` e
  `map[OperationKind]DestructiveExecutor` (não `switch` de domínio).
- Cada resolver/executor = adapter fino sobre binding existente (`LastTransactionDeleterAdapter`,
  `LastTransactionEditorAdapter`, `CardDeleterAdapter`, `BudgetConfigCommitterAdapter`).
- Sem nova assinatura de mutação nos bindings (reuso 1:1).

Confirmação / confirm_gate (ADR-003, HARD):
- 5 caminhos determinísticos: confirma → `completed`+efetiva; cancela → `short-circuit` sem efeito;
  ambíguo 1ª vez → `suspended` (re-prompt, `RepromptCount=1`); ambíguo 2ª vez → cancela;
  expirado (`SuspendedAt` > TTL calculado em UTC inline) → cancela sem efeito + `handled=false`
  (texto do usuário segue para parse).
- Idempotência por `messageID`: replay da mesma confirmação NÃO duplica efeito.
- `ConfirmState.ShortCircuit = true` sinaliza `IsDone()` para o kernel interromper a sequência.

Budget gate (ADR-004, HARD):
- `BudgetSessionRunner` ao atingir 100% inicia gate HITL (`OperationBudgetCommit`) em vez de
  chamar `ActivateBudgetUC` diretamente.
- Cancelar/expirar preserva o budget vigente; coleta multi-turn inalterada até o ponto de commit.

Testes (R-TESTING-001, HARD):
- Padrão testify/suite, whitebox (`package <X>`, NÃO `package <X>_test`), `fake.NewProvider()`,
  `dependencies struct` + IIFE por mock, SUT instanciado dentro do `s.Run`.
- Sem `noop.NewProvider()` em unit tests de usecase; sem `s.SetupTest()` manual.
- `//go:build integration` para testes que exigem Postgres real (testcontainers).

DoD E CRITÉRIOS DE ACEITE COM EVIDÊNCIA (obrigatório por tarefa — sem isso, NÃO marcar `done`)
Cada subagent, via `execute-task`, DEVE produzir `.specs/prd-agent-platform-evolution/<id>_execution_report.md`
com EVIDÊNCIA VERIFICÁVEL dos critérios abaixo:

TAREFA 1.0 — Gate de governança (apenas documentação/regras):
  DoD: addendum `R-AGENT-WF-001.7` redigido cobrindo `AwaitingApproval` como tipo fechado +
  proibição de string livre + exigência de persistência do estado + resume antes do parse + limpeza
  após efetivar/cancelar/expirar. Nota merge-patch em `workflow-kernel.md` (delta JSON, genérico,
  sem domínio). Referências a ADR-001..004 nas regras. Nenhum código de produção alterado.
  Evidência: gate `grep` de zero comentários retorna vazio; coerência com ADR-001..004 confirmada
  textualmente.

TAREFA 2.0 — Kernel merge-patch no resume (fundacional):
  DoD: `Codec[S].MergePatch` implementado e testado (merge preserva campos; delta sobrescreve só
  chaves presentes; `null` remove; vazio = no-op). `Engine.Resume` usa merge-patch.
  Evidência: `go test ./internal/platform/workflow/...` VERDE; teste de regressão do defeito
  (suspender estado rico → resume com `{"ResumeText":"x"}` → campos originais sobrevivem);
  `parity_test.go` VERDE; gate `R-WF-KERNEL-001` (todos os `grep`) retorna VAZIO.

TAREFA 3.0 — Tipos fechados + ConfirmState:
  DoD: `OperationKind`/`AwaitingApproval` com `String()`/`IsValid()`/`Parse*` que rejeitam valor
  inválido. `ConfirmState` com round-trip JSON estável (nenhum campo perdido no `MergePatch`).
  Evidência: `go test ./internal/agent/domain/confirmation/...` VERDE; `Parse*` retornam erro em
  valores fora do enum; round-trip `json.Marshal`→`json.Unmarshal` idêntico.

TAREFA 4.0 — Passos HITL (prepare_target, confirm_gate, execute_destructive):
  DoD: `confirm_gate` cobre 5 caminhos. `prepare_target`/`execute_destructive` despacham por mapa.
  Nenhum LLM invocado em qualquer passo. Adapters finos (sem regra/SQL/branching de domínio).
  Evidência: `go test ./internal/agent/application/workflow/steps/...` VERDE cobrindo os 5 caminhos
  do `confirm_gate`; alvo inexistente → short-circuit; cada `OperationKind` → executor correto.

TAREFA 5.0 — Workflow destructive_confirm + wiring:
  DoD: `NewDestructiveConfirmDefinition` com `ID="destructive_confirm"`, `Durable=true`, sequência
  `authorize→replay→policy→audit_begin→prepare→confirm→execute→format`. `Engine[ConfirmState]`
  wired em `module.go` com 4 executores. Nenhum binding com assinatura alterada.
  Evidência: `go build ./internal/agent/...` VERDE; teste unitário de composição (ordem dos passos,
  IDs, mapeamento OperationKind→executor); wiring resolve sem nil.

TAREFA 6.0 — Roteamento HITL no agent + resume + gate de budget:
  DoD: 4 kinds destrutivos suspendem na 1ª mensagem (nada efetivado). `continuePendingApproval`
  antes do `ParseInbound`. `daily_ledger_agent.go` sem novo `case intent.Kind`.
  Budget gate: ao 100%, pede confirmação antes de `ActivateBudgetUC`. Lançamentos comuns sem gate.
  Evidência: gate `R-AGENT-WF-001.1` (`grep -cE "^[[:space:]]*case intent\.Kind"` ≤ 1) retorna
  VAZIO; `go test ./internal/agent/application/services/...` VERDE; não regressão dos kinds comuns
  documentada no report.

TAREFA 7.0 — Integração (testcontainers) + E2E + gates R-*:
  DoD: 0 operações destrutivas sem confirmação; 100% resume após restart simulado. 0 efeito
  duplicado sob resume concorrente ou replay de messageID. 4 cenários E2E (confirmar, cancelar,
  ambíguo→reprompt→cancela, expirar→fall-through) passam para as 4 operações. Todos os gates R-*
  retornam vazio. Build/test/lint verdes.
  Evidência: `go test -tags=integration ./...` VERDE; output dos gates R-* (todos `grep`) colado
  no report; cardinalidade de métrica validada (ausência de `user_id`/`category_id`/
  `correlation_key` nos labels); `parity_test.go` VERDE.

TAREFA 8.0 — [Fase 2 adiada] Plano multi-tool (roadmap placeholder):
  DoD: permanece `pending` → nenhum código de produção alterado → marcar `done` SOMENTE após
  confirmar zero alterações. Status final do report: "planejada-não-implementada; será decomposta
  em rodada própria de create-tasks ao ser priorizada, herdando gates R-*".
  Evidência: `git diff HEAD --name-only` não lista arquivos de produção tocados.

TAREFA 9.0 — [Fase 3 adiada] Recuperação contextual + memória (roadmap placeholder):
  DoD: idem 8.0 — permanece `pending` → zero código → marcar `done` como "planejada-não-implementada".
  Evidência: `git diff HEAD --name-only` não lista arquivos de produção tocados.

GATES R-* GLOBAIS (rodar no final de CADA tarefa que toca código, antes de marcar `done`):

Gate R-WF-KERNEL-001.1 — sem import de domínio no kernel:
  grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
    "internal/agent\|internal/transactions\|internal/billing\|internal/identity" \
    internal/platform/workflow/ \
    && echo "FAIL" && exit 1 || true

Gate R-WF-KERNEL-001.2 — sem SQL fora do adapter Postgres:
  grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
    "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
    internal/platform/workflow/ \
    | grep -v "infrastructure/postgres" \
    && echo "FAIL" && exit 1 || true

Gate R-WF-KERNEL-001.3 — estados como tipos fechados (sem string solta):
  grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
    "RunStatus\s*=\s*\"[^\"]*\"\|StepStatus\s*=\s*\"[^\"]*\"\|SuspendReason\s*=\s*\"[^\"]*\"" \
    internal/platform/workflow/ \
    && echo "FAIL" && exit 1 || true

Gate R-WF-KERNEL-001.4 — cardinalidade de métrica controlada:
  grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
    '"user_id"\|"correlation_key"\|"category_id"' \
    internal/platform/workflow/ \
    && echo "FAIL" && exit 1 || true

Gate R-WF-KERNEL-001.5 — sem LLM no kernel:
  grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
    "openai\|anthropic\|openrouter\|gemini\|mistral\|llm\|ParseInbound\|FallbackChain\|CircuitBreaker" \
    internal/platform/workflow/ \
    && echo "FAIL" && exit 1 || true

Gate R-WF-KERNEL-001.6 — zero comentários no kernel:
  grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
    "^[[:space:]]*//" \
    internal/platform/workflow/ \
    | grep -Ev "(//go:|//nolint:|// Code generated)" \
    && echo "FAIL" && exit 1 || true

Gate R-AGENT-WF-001.1 — switch de domínio não cresce:
  f=$(find internal/agent -name "daily_ledger_agent.go" ! -name "*_test.go")
  [ -z "$f" ] && echo "SKIP" || {
    cases=$(grep -cE "^[[:space:]]*case intent\.Kind" "$f" || true)
    [ "${cases:-0}" -gt 1 ] && echo "FAIL: switch cresceu (cases=$cases)" && exit 1 || true
  }

Gate R-AGENT-WF-001.2 — zero comentários em tools/workflow:
  grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
    "^[[:space:]]*//" \
    internal/agent/application/tools/ \
    internal/agent/application/workflow/ 2>/dev/null \
    | grep -Ev "(//go:|//nolint:|// Code generated)" \
    && echo "FAIL" && exit 1 || true

Gate R-AGENT-WF-001.3 — sem SQL direto em tools/workflow:
  grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
    "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
    internal/agent/application/tools/ \
    internal/agent/application/workflow/ 2>/dev/null \
    && echo "FAIL" && exit 1 || true

Gate R-ADAPTER-001.1 — zero comentários global (produção):
  grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" --exclude="*_test.go" \
    "^[[:space:]]*//" internal/ configs/ cmd/ \
    | grep -Ev "(//go:|//nolint:|// Code generated)" \
    && echo "FAIL" && exit 1 || true

Gate R-TESTING-001.1 — sem blackbox package em usecases:
  grep -rn --include="*_test.go" "^package.*_test$" internal/*/application/usecases/ \
    && echo "FAIL" && exit 1 || true

Gate R-TESTING-001.3 — sem noop.NewProvider em usecases:
  grep -rn --include="*_test.go" "noop.NewProvider" internal/*/application/usecases/ \
    && echo "FAIL" && exit 1 || true

RESTRIÇÕES INEGOCIÁVEIS DE ESCOPO
- NENHUMA nova infraestrutura: sem pgvector, sem store vetorial, sem nova migration no MVP (reutiliza
  `workflow_runs`/`workflow_steps` da migration `000019_create_workflow_runtime`).
- NENHUM novo `case intent.Kind` no switch de `daily_ledger_agent.go`.
- NENHUM LLM nos passos HITL (`prepare_target`, `confirm_gate`, `execute_destructive`).
- NENHUMA alteração de assinatura nos bindings de mutação existentes.
- NENHUM side-store em `agent_sessions` para o fluxo HITL (snapshot do kernel é a fonte única).
- NENHUM comentário em `.go` de produção fora das exceções mapeadas em R-ADAPTER-001.1.
- NÃO implementar as capacidades A (plano multi-tool) e C (recuperação + memória) neste ciclo;
  tarefas 8.0 e 9.0 são guarda-chuva de roadmap (`pending` intencional).

ENCERRAMENTO
Ao completar todas as tarefas (ou ao gerar halt-first em qualquer não-done), produzir
`.specs/prd-agent-platform-evolution/_orchestration_report.md` com:
- Snapshot inicial vs final (contagem por estado)
- Tabela de tarefas executadas, puladas e adiadas (8.0/9.0)
- Waves e ordem de execução efetiva
- Resultado de TODOS os gates R-* (vazio = pass, com saída = fail registrado)
- Próximos passos (especialmente para 8.0/9.0 quando priorizadas)
```

---

## Referências rápidas para o executor

| Artefato | Path |
|----------|------|
| PRD | `.specs/prd-agent-platform-evolution/prd.md` |
| Techspec | `.specs/prd-agent-platform-evolution/techspec.md` |
| Tasks | `.specs/prd-agent-platform-evolution/tasks.md` |
| ADR-001 (merge-patch) | `.specs/prd-agent-platform-evolution/adr-001-kernel-resume-merge-patch.md` |
| ADR-002 (HITL always-on) | `.specs/prd-agent-platform-evolution/adr-002-hitl-always-on-kernel.md` |
| ADR-003 (confirmação) | `.specs/prd-agent-platform-evolution/adr-003-confirmation-contract.md` |
| ADR-004 (budget gate) | `.specs/prd-agent-platform-evolution/adr-004-budget-gate-at-commit.md` |
| R-AGENT-WF-001 | `.claude/rules/agent-workflows-tools.md` |
| R-WF-KERNEL-001 | `.claude/rules/workflow-kernel.md` |
| R-ADAPTER-001 | `.claude/rules/go-adapters.md` |
| R-TESTING-001 | `.claude/rules/go-testing.md` |
| Skill go-implementation | `.agents/skills/go-implementation/SKILL.md` |
| Skill execute-all-tasks | `.agents/skills/execute-all-tasks/SKILL.md` |

## Arquivos que serão criados/modificados (visão consolidada)

### Kernel — `internal/platform/workflow` (task 2.0)
- `codec.go` — `MergePatch(base, patch []byte) ([]byte, error)` (modificado)
- `engine.go` — bloco `Resume` → merge-patch (modificado)
- `codec_test.go` — testes de merge/null-remove/vazio (novo/estendido)
- `engine_test.go` — teste de regressão do defeito (estendido)
- `infrastructure/postgres/store_integration_test.go` — durabilidade + CAS (estendido)

### Agent — novos (tasks 3.0–6.0)
- `internal/agent/domain/confirmation/draft.go` — `ConfirmState`, `OperationKind`, `AwaitingApproval`
- `internal/agent/application/workflow/destructive_confirm.go` — `NewDestructiveConfirmDefinition`
- `internal/agent/application/workflow/steps/prepare_target.go` — `TargetResolver` por mapa
- `internal/agent/application/workflow/steps/confirm_gate.go` — 5 caminhos determinísticos
- `internal/agent/application/workflow/steps/execute_destructive.go` — `DestructiveExecutor` por mapa

### Agent — modificados (tasks 5.0–6.0)
- `internal/agent/module.go` — `Engine[ConfirmState]` + mapas de resolvers/executors
- `internal/agent/application/services/daily_ledger_agent.go` — registry + `continuePendingApproval`
- `internal/agent/application/services/agent_workflows.go` — registro do workflow HITL
- `internal/agent/application/services/intent_router.go` — `KernelDeps` expandido
- `internal/agent/application/tools/budget_session.go` — gate no commit

### Governança — modificados (task 1.0)
- `.claude/rules/agent-workflows-tools.md` — addendum R-AGENT-WF-001.7 (AwaitingApproval)
- `.claude/rules/workflow-kernel.md` — nota contrato merge-patch
- `.claude/rules/governance.md` — citação do addendum (se necessário)

### Testes E2E / integração (task 7.0)
- `internal/agent/application/services/kernel_e2e_test.go` — 4 cenários HITL
- `internal/agent/application/workflow/parity_test.go` — não regressão (verde)
