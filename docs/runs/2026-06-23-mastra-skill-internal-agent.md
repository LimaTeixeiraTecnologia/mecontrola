# Run 2026-06-23 — Repurpose do skill `mastra` como base Go do `internal/agent`

## Objetivo

Executar `docs/prompts/refactor_internal_agent.md` e tornar `.claude/skills/mastra` a base
reutilizável para, a cada novo agente/workflow/tool, seguir o padrão canônico Workflow + Tool
(R-AGENT-WF-001) com Mastra mapeado ao código Go.

Skill processual obrigatória declarada: **`go-implementation`** (qualquer edição `.go` exige Etapas
1–5 + checklist R0–R7). Nenhuma edição `.go` foi necessária nesta run.

## Descoberta-chave

O refactor R-AGENT-WF-001 **já estava 100% implementado e aderente ao Mastra** (confirmado pelo
usuário). Evidência mapeada na exploração:

- `WorkflowRegistry.Resolve(kind)` — `internal/agent/application/workflow/registry.go`.
- `daily_ledger_agent.go` sem `switch case intent.Kind` de domínio (gate 1: `cases=0`).
- Seam de extensão `buildRegistry()` — `application/services/agent_workflows.go`.
- Tool fina (`tools/tool.go`) + `WriteGuard` compartilhado (`workflow/write_guard.go`).
- Enums fechados: `RunStatus`, `ToolOutcome`, `AwaitingKind`, `TransactionKind`, `Confidence`,
  `DecisionStatus`.
- `Thread`/`Run`/`WorkingMemory` + `AgentRuntime.Execute` (ciclo Thread→Run auditável).
- Pending step `pendingexpense.Draft` (Save/Load/Clear) + resume antes de `ParseInbound`.
- LLM confinado a `ParseInbound` (+ exceções sancionadas: conversational fallback e onboarding).

Logo, o deliverable foi o **skill** + alinhamento de **regra/doc**; sem mudança funcional de runtime.

## Triagem dos 3 gaps sinalizados

- **Gap A — guard `nil` no conversational workflow:** não é defeito. `KindUnknown` não é write;
  `composite.go:75` trata `nil` por design. → documentado em `references/write-guard.md`.
- **Gap B — LLM de onboarding em chain separado:** intencional (modelo dedicado por decisão de
  projeto). → documentado em `references/parse-llm-boundary.md`.
- **Gap C — LLM de fallback dentro do conversational Tool:** resposta conversacional precisa de LLM
  inerentemente; é o escape-hatch do `KindUnknown`. → resolvido tornando a exceção **explícita** na
  regra R-AGENT-WF-001.4 e na referência `parse-llm-boundary.md`. Sem refactor.

## Arquivos criados/modificados

**Skill (`.agents/skills/mastra/`, espelhado por symlink em `.claude/skills/mastra`):**
- `SKILL.md` — reescrito (v3.0.0): guia Go do `internal/agent`, fluxo canônico, mapa Mastra→Go,
  tabela de referências, prerequisites `go-implementation`/`agent-governance`.
- `references/core-concepts.md` — reescrito (primitivos Mastra→Go com caminhos).
- `references/add-workflow-tool.md` — criado (a receita de 6 passos no seam `buildRegistry`).
- `references/state-as-type.md` — criado (enums fechados + smart constructors).
- `references/write-guard.md` — criado (authz/replay/policy/audit; guard `nil` em leitura é correto).
- `references/thread-run-runtime.md` — criado (ciclo Thread→Run, métricas enum-only).
- `references/pending-step.md` — criado (Draft Save/Resume/Clear).
- `references/parse-llm-boundary.md` — criado (LLM só no parse + exceções sancionadas).
- `references/rules-checklist.md` — criado (5 gates prontos para colar).
- `references/INDEX.yaml` — criado (matriz determinística de carregamento por tarefa).
- Removidos: references TS-only (`create-mastra`, `embedded-docs`, `remote-docs`, `migration-guide`,
  `mastra-api`, `common-errors`, `model-selection`) e `scripts/provider-registry.mjs`.

**Regra/registry:**
- `.claude/rules/agent-workflows-tools.md` — R-AGENT-WF-001.4: exceções de LLM (conversational +
  onboarding) tornadas explícitas.
- `skills-lock.json` — removido o entry upstream `mastra` (deixou de ser import `mastra-ai/skills`).

**Código Go:** nenhuma mudança (já aderente).

## Evidência de validação

```
go build ./internal/agent/...   -> exit 0
go vet   ./internal/agent/...    -> exit 0
go test -race -count=1 ./internal/agent/...  -> todos os pacotes OK, sem race
```

Gates R-AGENT-WF-001:
- Gate 1 (switch de domínio em `daily_ledger_agent.go`): **OK** — `cases=0`.
- Gate 2 (zero comentários em `tools/`+`workflow/`): **OK**.
- Gate 3 (sem SQL direto em `tools/`+`workflow/`): **OK**.
- `skills-lock.json` JSON válido; `INDEX.yaml` YAML válido; 9 referências + SKILL presentes.

## Como usar (a partir de agora)

Para um novo agente/workflow/tool: invocar `/mastra`, ler `references/core-concepts.md`, carregar a
referência da tarefa via `INDEX.yaml` (ex.: `add-write-workflow`), implementar no seam
`buildRegistry()` mantendo a Tool fina, e validar com `references/rules-checklist.md`.
