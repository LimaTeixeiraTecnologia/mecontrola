# Checklist de validação — antes do merge

Rode após qualquer mudança no substrato (`internal/platform/*`) ou em um consumidor (`internal/agents`).
Espelha os gates de `.claude/rules/{workflow-kernel,agent-workflows-tools,go-adapters,input-dto-validate}.md`.

## Build, vet e race

```bash
go build ./internal/platform/... ./internal/agents/...
go vet ./internal/platform/... ./internal/agents/...
go test -race -count=1 ./internal/platform/... ./internal/agents/...
```

## Gate 1 — kernel puro (R-WF-KERNEL-001.1/.2/.5)

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "internal/transactions\|internal/billing\|internal/identity\|internal/platform/agent\|internal/platform/memory\|openai\|anthropic\|openrouter\|llm\b" \
  internal/platform/workflow/ \
  && echo "FAIL: import de dominio/camada superior ou LLM no kernel" || echo "OK gate1"
```

## Gate 2 — SQL só no adapter postgres do kernel (R-WF-KERNEL-001.2)

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
  internal/platform/workflow/ | grep -v "infrastructure/postgres" \
  && echo "FAIL: SQL fora do adapter postgres no kernel" || echo "OK gate2"
```

## Gate 3 — zero comentários em Go de produção (R-ADAPTER-001.1)

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" --exclude="*_test.go" \
  "^[[:space:]]*//" internal/platform/ internal/agents/ \
  | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL: comentarios proibidos" || echo "OK gate3"
```

## Gate 4 — tool/adapter fino do consumidor: sem SQL direto (R-ADAPTER-001.2)

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
  internal/agents/application/tools/ \
  internal/agents/infrastructure/messaging/database/consumers/ 2>/dev/null \
  && echo "FAIL: SQL direto em tool/consumer" || echo "OK gate4"
```

## Gate 5 — input DTO com Validate() (R-DTO-VALIDATE-001)

```bash
for f in $(find internal/agents -path "*/application/dtos/input/*.go" ! -name "*_test.go" ! -name "errors.go"); do
  grep -q "func.*Validate().*error" "$f" || echo "FAIL: sem Validate() em $f"
done; echo "OK gate5"
```

## Gate 6 — cardinalidade de métricas (R-WF-KERNEL-001.4 / R-TXN-004)

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  '"user_id"\|"resource_id"\|"thread_id"\|"correlation_key"\|"category_id"' \
  internal/platform/ \
  | grep -i "metric\|counter\|histogram\|label\|observability.String" \
  && echo "WARN: revisar label de alta cardinalidade em metrica" || echo "OK gate6"
```

## Gate 7 — checklist manual

- [ ] Comportamento novo entrou como agente/tool/workflow/scorer no consumidor — **não** no kernel nem como
      reimplementação de primitivo.
- [ ] Tool fina: o `exec` delega a client/usecase; sem regra/SQL/branching de domínio.
- [ ] Estados de fronteira fechados (`agent.RunStatus`/`ToolOutcome`, `workflow.RunStatus`/`StepStatus`/
      `SuspendReason`, `scorer.ScorerKind`, `memory.MessageRole`) — sem string livre.
- [ ] Kernel intocado quanto a domínio/LLM/SQL; resume via `MergePatch` (nunca substitui estado inteiro).
- [ ] LLM só nas call-sites sancionadas (agent loop, step que chama `Stream`, scorer LLM-judged); nunca no kernel.
- [ ] Structured output validado na conclusão (sync no fim de `Execute`, stream no `Result`); falha explícita.
- [ ] Thread resolvido antes do Run; Run auditável com status fechado; métricas com labels enum-only.
- [ ] Persistência reusa `platform_*` (migration 000003); sem schema novo de agente.
- [ ] Testes no padrão testify/suite (whitebox, `fake.NewProvider()`, IIFE por mock).
