# Workflow Kernel Generico — Regra de Plataforma

- Rule ID: R-WF-KERNEL-001
- Severidade: hard
- Escopo: `internal/platform/workflow/`
- ADR de origem: ADR-004 (`.specs/prd-workflow-kernel/adr-004-governance-gate.md`)

## Objetivo

Garantir que o kernel de workflow em `internal/platform/workflow` permaneca um mecanismo generico
de orquestacao de passos, sem qualquer dependencia ou conhecimento de dominio. O kernel oferece
primitivos (`Step`, `Engine`, `Store`, combinadores, suspend/resume, retry); a semantica de dominio
(Thread, Run auditavel de agent, WorkingMemory, PendingStep, intent) e exclusiva de `internal/agent`
e modulos consumidores.

## R-WF-KERNEL-001.1 — Proibido import de pacote de dominio [HARD]

Nenhum arquivo em `internal/platform/workflow/` pode importar pacotes de dominio ou de bounded
contexts:

- `internal/agent/...`
- `internal/transactions/...`
- `internal/billing/...`
- `internal/identity/...`
- pacotes que contenham `intent`, `agent`, `pendingexpense`, `category`, `transaction`

O kernel opera sobre um estado generico `S any` e uma `correlationKey string` opaca. Proibido
receber `user_id`, `channel`, `intent.Kind` ou qualquer outro tipo semantico de dominio como
parametro publico.

Gate de verificacao (deve retornar vazio antes de merge):

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "internal/agent\|internal/transactions\|internal/billing\|internal/identity" \
  internal/platform/workflow/ \
  && echo "FAIL: import de dominio em workflow kernel" && exit 1 \
  || true
```

## R-WF-KERNEL-001.2 — Proibido regra de negocio, SQL e branching de dominio [HARD]

O kernel nao pode conter:

1. Regra ou calculo de negocio de qualquer bounded context.
2. Query SQL direta (`QueryContext`, `ExecContext`, `db.Query`, `tx.Exec`, `db.Exec`) fora do
   adapter Postgres (`internal/platform/workflow/infrastructure/postgres/`).
3. Branching sobre estado semantico de dominio (comparar campos com significado de negocio para
   decidir comportamento).

O adapter Postgres em `infrastructure/postgres/` pode usar SQL exclusivamente para persistir e
carregar `Snapshot` e `StepRecord` — sem logica de dominio.

Gate de verificacao — SQL fora do adapter (deve retornar vazio antes de merge):

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
  internal/platform/workflow/ \
  | grep -v "infrastructure/postgres" \
  && echo "FAIL: SQL fora do adapter postgres no kernel" && exit 1 \
  || true
```

## R-WF-KERNEL-001.3 — Estados sao tipos fechados (state-as-type) [HARD]

`RunStatus`, `StepStatus` e `SuspendReason` DEVEM ser tipos fechados com constantes enumeradas.
Nunca representar esses estados como `string` solta em assinaturas publicas do kernel.

Valores permitidos:

- `RunStatus`: `RunStatusRunning`, `RunStatusSuspended`, `RunStatusSucceeded`, `RunStatusFailed`.
- `StepStatus`: `StepStatusCompleted`, `StepStatusSuspended`, `StepStatusFailed`, `StepStatusSkipped`.
- `SuspendReason`: `SuspendAwaitingInput` (enumerado; extensivel apenas via nova constante tipada).

Persistencia em coluna TEXT e permitida via metodo `String()` ou mapeamento explicito no adapter;
a fronteira de codigo Go permanece tipada.

Gate de verificacao (deve retornar vazio antes de merge):

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "RunStatus\s*=\s*\"[^\"]*\"\|StepStatus\s*=\s*\"[^\"]*\"\|SuspendReason\s*=\s*\"[^\"]*\"" \
  internal/platform/workflow/ \
  && echo "FAIL: estado como string solta no kernel" && exit 1 \
  || true
```

## R-WF-KERNEL-001.4 — Cardinalidade controlada em metricas [HARD]

Nenhum label de metrica Prometheus em `internal/platform/workflow/` pode carregar `user_id`,
`correlation_key` ou `category_id`. Labels permitidos: `workflow`, `step`, `status`, `outcome`.

Herda R-TXN-004 e R-AGENT-WF-001.5 para metrica de cardinalidade controlada.

Gate de verificacao (deve retornar vazio antes de merge):

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  '"user_id"\|"correlation_key"\|"category_id"' \
  internal/platform/workflow/ \
  && echo "FAIL: label de alta cardinalidade em metrica do kernel" && exit 1 \
  || true
```

## R-WF-KERNEL-001.5 — LLM proibido no kernel [HARD]

O kernel nao pode invocar LLM, prompt rendering, fallback chain ou qualquer client de modelo de
linguagem. Preserva R-AGENT-WF-001.4: LLM aparece exclusivamente no step de parse (`ParseInbound`)
no `internal/agent`.

Gate de verificacao (deve retornar vazio antes de merge):

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "openai\|anthropic\|openrouter\|gemini\|mistral\|llm\|ParseInbound\|FallbackChain\|CircuitBreaker" \
  internal/platform/workflow/ \
  && echo "FAIL: referencia a LLM no kernel" && exit 1 \
  || true
```

## R-WF-KERNEL-001.6 — Zero comentarios em Go de producao [HARD]

Herda R-ADAPTER-001.1: nenhum arquivo `.go` em `internal/platform/workflow/` pode conter
comentarios de linha (`//`) ou bloco (`/* */`), com excecao unica de:

- Cabecalho `// Code generated` na linha 1 (arquivos gerados por ferramenta).
- Diretivas de compilador: `//go:build`, `//go:generate`, `//go:embed`, `//nolint:` com
  justificativa na mesma linha.

Gate de verificacao (deve retornar vazio antes de merge):

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "^[[:space:]]*//" \
  internal/platform/workflow/ \
  | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL: comentarios proibidos no kernel" && exit 1 \
  || true
```

## R-WF-KERNEL-001.7 — Contrato de resume via JSON merge-patch [HARD]

Adicionado em 2026-06-24 (ADR-001, `.specs/prd-agent-platform-evolution/adr-001-kernel-resume-merge-patch.md`).

`Engine.Resume` DEVE aplicar o payload de resume como **delta JSON merge-patch (RFC 7386)** sobre
`Snapshot.State`, nunca substituir o estado inteiro pelo payload. Isso garante que o estado suspenso
rico seja preservado quando o consumidor passa apenas um subconjunto dos campos no resume
(ex.: `{"ResumeText":"sim"}`).

Contratos obrigatorios:

- `Codec[S].MergePatch(base, patch []byte) ([]byte, error)` — operacao generica sobre JSON puro,
  sem conhecimento de dominio; chave com valor `null` remove a chave (semantica RFC 7386).
- Resume vazio (`len(resume) == 0`) e **no-op** — mantém compatibilidade com chamadas existentes.
- O `Snapshot.State` e a **fonte unica de verdade** no resume; consumidores NAO devem manter
  side-store de draft para recuperar estado suspenso — o snapshot do kernel e suficiente.
- O merge opera sobre `map[string]any` (round-trip JSON); sem tipo de dominio exposto no kernel.

Proibido:

- Substituir `Snapshot.State` inteiro pelo payload de resume (regressao ao defeito latente).
- Expor no `MergePatch` qualquer tipo semantico de dominio (`AwaitingApproval`, `OperationKind`,
  `ConfirmState`) — a operacao e generica e opera sobre bytes JSON.
- Incluir logica de dominio (branching sobre campos do estado) no `MergePatch` ou no bloco de resume
  do `Engine` (preserva R-WF-KERNEL-001.2).

Gate de verificacao — substituicao de estado inteiro no resume (deve retornar vazio antes de merge):

```bash
grep -n "current = rs\|current = decoded\|current = resumed" \
  internal/platform/workflow/engine.go \
  && echo "FAIL: resume substitui estado inteiro — usar MergePatch" && exit 1 \
  || true
```

Gate de verificacao — MergePatch sem tipo de dominio (deve retornar vazio antes de merge):

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "AwaitingApproval\|OperationKind\|ConfirmState\|AwaitingKind\|pendingexpense" \
  internal/platform/workflow/ \
  && echo "FAIL: tipo de dominio em MergePatch ou engine do kernel" && exit 1 \
  || true
```

## Permitido (consumo pelo agent e demais modulos)

O `internal/agent` e qualquer outro modulo podem **consumir** o kernel:

- Instanciar `Engine[S]` passando sua estrutura de estado proprio como `S`.
- Registrar `Step[S]` que chamam bindings e usecases do modulo consumidor.
- Usar `Store` (porta) com o adapter Postgres.

O consumidor e responsavel por manter sua semantica propria (Thread, WorkingMemory, PendingStep
no caso do agent) sem delegar essa responsabilidade ao kernel.

## Proibido (R-WF-KERNEL-001 global)

- Aprovar PR que adicione import de pacote de dominio a `internal/platform/workflow/`.
- Aprovar PR com regra de negocio, branching de dominio ou LLM no kernel.
- Representar `RunStatus`/`StepStatus`/`SuspendReason` como `string` solta.
- Usar `user_id`, `correlation_key` ou `category_id` como label de metrica.
- Flexibilizar estas regras por diferenca de ferramenta, conveniencia ou deadline.

## Referencias

- ADR-004 (prd-workflow-kernel): `.specs/prd-workflow-kernel/adr-004-governance-gate.md`
- ADR-001 (prd-agent-platform-evolution): `.specs/prd-agent-platform-evolution/adr-001-kernel-resume-merge-patch.md` — merge-patch no resume
- R-AGENT-WF-001: `.claude/rules/agent-workflows-tools.md` — semantica exclusiva do agent
- R-ADAPTER-001: `.claude/rules/go-adapters.md` — zero comentarios e adaptadores finos
- R-TXN-004: `.claude/rules/transactions-workflows.md` — cardinalidade de metricas
- `governance.md`: `.claude/rules/governance.md` — precedencia DMMF state-as-type
- PRD: `.specs/prd-workflow-kernel/prd.md` (RF-01, RF-14, RF-15, RF-27)
- Techspec: `.specs/prd-workflow-kernel/techspec.md`
- PRD (evolucao): `.specs/prd-agent-platform-evolution/prd.md` (RF-21..RF-27)
- Techspec (evolucao): `.specs/prd-agent-platform-evolution/techspec.md`
