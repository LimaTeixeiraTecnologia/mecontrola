<!-- spec-hash-prd: 48acf5bbae04e963357453918477daf059e55f11ebfcaa8d31943f8af666b571 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Catálogo Canônico de Capabilities do Agent

> PRD: `.specs/prd-agent-capability-catalog/prd.md` (spec-version 2) · Roadmap: `docs/plans/2026-06-25-mastra-gap-map-mecontrola.md` (Fase 1+2).
> Governança: `AGENTS.md`, `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001), `.claude/rules/workflow-kernel.md` (R-WF-KERNEL-001), `.claude/rules/go-adapters.md` (R-ADAPTER-001), `.claude/rules/go-testing.md` (R-TESTING-001). Skills: `go-implementation`, `mastra`. DMMF (Wlaschin) — state-as-type, smart constructors, sem string livre em estado.

## Resumo Executivo

O `internal/agent` mantém hoje **duas fontes desacopladas** para classificar cada `intent.Kind`: a execução resolve o workflow via `IntentRegistry.Resolve` (`internal/agent/application/workflow/registry.go:69`, alimentado por `buildRegistry()` em `agent_workflows.go:23-69`), enquanto a auditoria/métrica deriva os labels `workflow`/`tool` de um `switch` paralelo manual — `workflowFor`/`toolFor` (`internal/agent/application/services/agent_runtime.go:208-245`). Esse switch **já está mentindo em produção**: `KindQueryIncomeSummary` executa no workflow `transactions` mas é auditado como `conversational` (cai no `default`, e `toolFor` retorna `""` — nenhuma métrica de tool é emitida); `KindBudgetRecurrence` executa em `budget` mas é auditado como `conversational`; `KindDeleteTransactionByRef`/`KindEditTransactionByRef` (destrutivos) também caem em `conversational`. Conhecimento operacional adicional (destrutivo/sensível) vive num mapa ad hoc `intentToOperationKind` (`daily_ledger_agent.go:648-654`).

A solução introduz um **catálogo canônico de capabilities** (`CapabilitySpec` rica e tipada) construído a partir do **mesmo wiring** que monta o registry, tornando-se a fonte única de verdade. O `AgentRuntime` passa a **derivar** os labels `workflow`/`tool` do catálogo (matando o switch paralelo, ADR-002); a classificação destrutivo/sensível dos gates HITL passa a derivar do `CapabilitySpec` (`RequiresConfirmation`/`Mode`, ADR-003); o catálogo é **listável programaticamente** (base para auditoria, futuros console e evals); um **teste-guard** garante cobertura total e equivalência de labels por kind, fechando o caminho para drift futuro. A skill `mastra` e um checklist de extensão passam a refletir os 5 seams reais. A mudança é estrutural e de documentação — **não** toca o ponto de parse LLM, vive inteiramente em `internal/agent` e não vaza semântica para o kernel genérico.

## Arquitetura do Sistema

### Visão Geral dos Componentes

**Novos (em `internal/agent/application/capability/`):**
- `CapabilitySpec` (`capability/spec.go`) — struct canônica rica + tipo fechado `CapabilityMode` (state-as-type).
- `Catalog` (`capability/catalog.go`) — coleção imutável construída a partir das `CapabilitySpec`; expõe `Lookup(kind) (CapabilitySpec, bool)`, `List() []CapabilitySpec` e `Classify(kind) (workflow, tool string)`. Validações de unicidade/integridade no construtor (smart constructor).
- `BuildCatalog(...)` (`capability/build.go`) — função que declara as `CapabilitySpec` de todas as capabilities (roteáveis + destrutivas HITL), invocada pelo mesmo seam de wiring do registry.

**Modificados:**
- `AgentRuntime` (`internal/agent/application/services/agent_runtime.go`) — recebe `*capability.Catalog` por injeção; `Execute` deriva `workflow`/`tool` via `catalog.Classify(kind)`; `workflowFor`/`toolFor` removidos (ou reduzidos a delegadores finos sobre o catálogo).
- `DailyLedgerAgent` (`internal/agent/application/services/daily_ledger_agent.go`) — `isDestructiveKind`/`resolveOperationKind` passam a consultar o catálogo (`RequiresConfirmation`); `intentToOperationKind` permanece como mapa de `OperationKind` (tipo fechado) **referenciado pelo catálogo**, sem novo `case` de domínio no switch.
- `module.go` (wiring do agent) — constrói o catálogo no mesmo ponto de `buildRegistry()` e o injeta no `AgentRuntime`.
- Skill `mastra` (`.agents/skills/mastra/SKILL.md` + `references/`) — declara os 5 seams reais e o checklist de extensão.

**Não-modificados (fronteiras preservadas):** kernel `internal/platform/workflow` (sem qualquer conhecimento de catálogo/capability — R-WF-KERNEL-001); ponto de parse LLM (R-AGENT-WF-001.4); `IntentRegistry`/`composite` (continuam resolvendo execução — o catálogo é metadata paralela alimentada pelo mesmo wiring, não substitui o registry de execução).

### Fluxo de dados (resumo)
```
wiring (module.go)
  ├─ buildRegistry()  → IntentRegistry (execução)              [inalterado]
  └─ BuildCatalog()   → capability.Catalog (classificação)     [novo, mesmo wiring]

AgentRuntime.Execute
  → router.route(...) → RouteResult{Kind, Outcome}
  → workflow, tool := catalog.Classify(result.Kind)    (era workflowFor/toolFor)
  → run.Resolve{Workflow, ToolName, IntentKind} + span attrs + recordMetrics

DailyLedgerAgent.dispatch*
  → catalog.Lookup(kind).RequiresConfirmation           (era isDestructiveKind)
  → resolveOperationKind(kind)                           (mapa OperationKind, inalterado)
```

## Design de Implementação

### Interfaces Chave

**Tipo fechado `CapabilityMode` (DMMF state-as-type, R-AGENT-WF-001.3):**
```go
// internal/agent/application/capability/spec.go
type CapabilityMode int

const (
    ModeRead CapabilityMode = iota + 1
    ModeWrite
)

func (m CapabilityMode) String() string        // "read" | "write" (persistência/label)
func (m CapabilityMode) IsValid() bool
func ParseCapabilityMode(s string) (CapabilityMode, error)
```

**`CapabilitySpec` (RF-01/06):**
```go
type CapabilitySpec struct {
    ID                   string         // "transaction.record_expense"
    Description          string
    Kind                 intent.Kind
    WorkflowID           string         // owner real: "transactions"|"budget"|"cards"|"conversational"
    ToolName             string         // label de tool; "" para conversational
    Mode                 CapabilityMode // ModeRead | ModeWrite
    RequiresConfirmation bool           // gate HITL (destrutivo/sensível)
    SupportsSuspend      bool
    SupportsResume       bool
    Channels             []string       // MVP: ["whatsapp"]
    MetricsKey           string         // espelha ToolName por padrão (Q-01)
}
```

**`Catalog` (RF-04/05/07) — smart constructor + classificação derivada:**
```go
// internal/agent/application/capability/catalog.go
type Catalog struct {
    byKind map[intent.Kind]CapabilitySpec
    specs  []CapabilitySpec
}

func NewCatalog(specs ...CapabilitySpec) (*Catalog, error) // valida unicidade ID/Kind, Mode válido, WorkflowID não-vazio

func (c *Catalog) Lookup(kind intent.Kind) (CapabilitySpec, bool)
func (c *Catalog) List() []CapabilitySpec                  // cópia defensiva, ordem estável
func (c *Catalog) Classify(kind intent.Kind) (workflow, tool string) // fallback: workflowConversational, ""
```

**`AgentRuntime.Execute` — derivação (substitui agent_runtime.go:80-81):**
```go
workflow, tool := rt.catalog.Classify(result.Kind)
// resto inalterado: run.Resolve / span attrs / recordMetrics usam (workflow, tool)
```

### Modelos de Dados

Sem mudança de schema. O catálogo é estrutura em memória, construída no boot a partir do wiring. Nenhuma tabela nova; `Run` continua persistindo `Workflow`/`ToolName`/`IntentKind` (entidade `entities.Run` inalterada). Persistência de enums (`Mode`) via `String()` se necessário em logs — fronteira de código permanece tipada.

**Cobertura canônica (24 capabilities):** 19 kinds roteáveis de `routableKinds()` (incl. `KindQueryIncomeSummary` → `transactions`, `KindBudgetRecurrence` → `budget`), `KindUnknown` → `conversational`, e 5 kinds destrutivos de `intentToOperationKind` (`KindDeleteLastTransaction`, `KindEditLastTransaction`, `KindDeleteCard`, `KindDeleteTransactionByRef`, `KindEditTransactionByRef`), todos com `RequiresConfirmation: true`.

**Drift identificado (corrigido por este trabalho — ver ADR-002):**

| Kind | Label legado (`workflowFor`) | Workflow real (owner) | Ação |
|---|---|---|---|
| `KindQueryIncomeSummary` | `conversational` (default) + tool `""` | `transactions` | **corrigir** label + passa a emitir tool |
| `KindBudgetRecurrence` | `conversational` (default) | `budget` | **corrigir** label |
| `KindDeleteTransactionByRef` | `conversational` (default) | destrutivo (transactions) | **corrigir** label |
| `KindEditTransactionByRef` | `conversational` (default) | destrutivo (transactions) | **corrigir** label |
| demais kinds roteáveis | concordante | concordante | preservar idêntico (RF-09) |

### Endpoints de API

Nenhum endpoint HTTP novo. Esta é uma mudança interna de classificação/observabilidade. A listagem programática (`Catalog.List()`) é consumida em testes e por futuras superfícies (fora deste escopo); não se cria porta HTTP (R-ADAPTER-001 / fora de escopo PRD).

## Pontos de Integração

- **`IntentRegistry`** (execução): inalterado; o catálogo deriva `WorkflowID` do mesmo conjunto de IDs (`transactions`/`budget`/`cards`/`conversational`) usados em `buildRegistry()`. Teste de consistência garante que `Catalog.WorkflowID(kind)` == ID do workflow que `IntentRegistry.Resolve(kind)` retorna, para todo kind roteável.
- **Confirmation engine / HITL** (`internal/agent/domain/confirmation`): `OperationKind` (tipo fechado) permanece; `RequiresConfirmation` no catálogo passa a ser a fonte do "é destrutivo?" consumida por `isDestructiveKind`. O mapa `intentToOperationKind` continua sendo a tradução kind→`OperationKind` (não duplicada no catálogo).
- **Métricas Prometheus** (`agent_runs_total`, `agent_run_duration_seconds`, `agent_tool_invocations_total`): labels e nomes inalterados; só a **origem** dos valores `workflow`/`tool` muda (catálogo, não switch).

## Abordagem de Testes

### Testes Unitários
- **`CapabilityMode` (DMMF):** `String`/`IsValid`/`ParseCapabilityMode` (válidos, inválido, zero-value rejeitado). Sem mock.
- **`Catalog` (smart constructor + classificação):** `NewCatalog` rejeita ID/Kind duplicado, `Mode` inválido, `WorkflowID` vazio; `Lookup` hit/miss; `List` cópia defensiva e ordem estável; `Classify` retorna labels corretos e fallback (`conversational`, `""`) para kind desconhecido. Sem mock.
- **Teste-guard de cobertura (RF-10):** para todo `kind ∈ routableKinds()`, `catalog.Lookup(kind)` retorna `ok==true`. Falha o build na ausência.
- **Teste de consistência registry↔catálogo:** para todo kind roteável, `catalog.Classify(kind).workflow` == ID do workflow que `IntentRegistry.Resolve(kind)` possui.
- **Teste de equivalência por kind (RF-09/RF-17):** tabela `kind → (workflowLegado, toolLegado)` capturando os valores atuais de `workflowFor`/`toolFor`; assert que o catálogo reproduz idêntico **exceto** os 4 kinds de drift documentados (lista explícita), cujos novos valores corretos são afirmados. Garante MS-03 e torna a correção visível, não silenciosa.
- **`AgentRuntime.Execute` (R-TESTING-001 — testify/suite whitebox, `fake.NewProvider()`, mocks por IIFE):** com catálogo injetado, valida que `Run.Resolve` e `recordMetrics` recebem os labels derivados do catálogo; cobre kind de drift (ex.: `QueryIncomeSummary` agora `transactions` + tool emitido).
- **`DailyLedgerAgent` destrutivo:** `isDestructiveKind` derivado do catálogo retorna igual ao mapa atual para os 5 kinds destrutivos; gates HITL não regridem.

### Testes de Integração
> Critérios: (a) fronteira de IO crítica? — não há IO novo; (b) risco de regressão na substituição? — sim, observabilidade. ⇒ a cobertura por **unit + equivalência por kind** é suficiente; **integration não é requerido** para esta mudança estrutural. Os integration/e2e existentes do agent permanecem como rede de não-regressão (RF-16).

### Testes E2E
- Reaproveitar a suíte existente do agent: garantir verde após a troca de fonte (RF-16). Nenhum novo E2E dedicado.

## Sequenciamento de Desenvolvimento

### Ordem de Build
1. **Tipos + catálogo** (`CapabilityMode`, `CapabilitySpec`, `Catalog`, `NewCatalog`) — domínio puro, 100% testável. (Habilita o resto.)
2. **`BuildCatalog`** declarando as 24 `CapabilitySpec` + teste-guard de cobertura e consistência registry↔catálogo.
3. **Wiring** (`module.go`): construir e injetar o catálogo no `AgentRuntime`.
4. **Runtime deriva do catálogo**: `Execute` usa `catalog.Classify`; remover `workflowFor`/`toolFor`; teste de equivalência por kind.
5. **Migração destrutivo/sensível**: `isDestructiveKind` consulta catálogo; validar gates HITL.
6. **Skill `mastra` + checklist de extensão**.
7. **Rodar suíte completa** (RF-16) e validar não-regressão.

### Dependências Técnicas
- Nenhuma infra nova. Depende apenas do wiring existente (`buildRegistry`/`module.go`) e do conjunto de `intent.Kind` atual.

## Monitoramento e Observabilidade

- **Métricas (inalteradas em nome/label; origem corrigida):** `agent_runs_total{agent_id,channel,workflow,status}`, `agent_run_duration_seconds{agent_id,channel,workflow}`, `agent_tool_invocations_total{tool,outcome}`. Cardinalidade controlada — sem `user_id`/`correlation_key`/`category_id` (RF-13 / R-AGENT-WF-001.5 / R-TXN-004).
- **Impacto esperado pós-deploy (comunicar a operação):** queda de volume no label `workflow="conversational"` e surgimento/aumento em `transactions`/`budget` para os kinds corrigidos (`QueryIncomeSummary`, `BudgetRecurrence`, by-ref); aparecimento de `agent_tool_invocations_total{tool="query_income_summary"}` antes ausente. Documentar no PR para não ser lido como anomalia.
- **Logs:** log único no boot listando o catálogo carregado (contagem por workflow/mode) para auditoria de inicialização. Sem comentários em Go (R-ADAPTER-001.1).

## Considerações Técnicas

### Decisões Chave (ADRs)
- **ADR-001** — Catálogo canônico como fonte única, alimentado pelo mesmo wiring do registry; `CapabilitySpec` rica + `CapabilityMode` tipo fechado. `adr-001-catalogo-canonico-fonte-unica.md`. (RF-01..06)
- **ADR-002** — Runtime deriva `workflow`/`tool` do catálogo com fallback conversational; drift de `QueryIncomeSummary`/`BudgetRecurrence`/by-ref **corrigido explicitamente** (não preservado), com teste de equivalência por kind e comunicação de impacto. `adr-002-runtime-deriva-do-catalogo.md`. (RF-07..11, RF-13, RF-17)
- **ADR-003** — Classificação destrutivo/sensível (`isDestructiveKind`) derivada de `RequiresConfirmation`; `OperationKind`/`intentToOperationKind` permanecem como tradução tipada, sem regressão HITL nem novo `case` de domínio. `adr-003-migracao-destrutivo-para-capabilityspec.md`. (RF-12)

### Riscos Conhecidos
- **R1 — Mudança de label observável (drift corrigido):** 4 kinds mudam de `conversational` para o workflow real. Mitigação: teste de equivalência por kind com lista explícita de exceções; comunicar impacto no PR; é correção de defeito, não regressão. Alternativa rejeitada (preservar o label errado) violaria O2/MS-02.
- **R2 — Dupla fonte (catálogo × registry):** se `BuildCatalog` e `buildRegistry` divergirem no `WorkflowID`, reintroduz drift. Mitigação: teste de consistência registry↔catálogo (todo kind roteável); ambos derivam dos mesmos IDs de workflow. Idealmente um único ponto declara ambos — registrado como evolução (não obrigatória no MVP) em ADR-001.
- **R3 — Kinds destrutivos fora de `routableKinds`:** os 5 kinds HITL não estão no registry de execução; o teste-guard de cobertura roteável não os cobre. Mitigação: catálogo cobre kinds destrutivos explicitamente + teste dedicado validando que `RequiresConfirmation==true` para todos os de `intentToOperationKind`.
- **R4 — `KindUnknown` no catálogo:** mapeado a `conversational`/`ModeRead`; garantir que `Classify` para qualquer kind não-catalogado ainda retorne o fallback (defensa em profundidade).

### Conformidade com Padrões
- **R-AGENT-WF-001:** roteamento por registry preservado; **sem novo `case intent.Kind`** de domínio em `daily_ledger_agent.go`; `CapabilityMode` tipo fechado; `Run` auditável com labels derivados; LLM intocado; cardinalidade de métricas controlada.
- **R-WF-KERNEL-001:** catálogo e semântica de capability **exclusivos de `internal/agent`**; kernel genérico não importa nem conhece o catálogo.
- **R-ADAPTER-001:** novos arquivos `.go` sem comentários; sem SQL; runtime continua fino.
- **R-DTO-VALIDATE-001:** não há input DTO novo de fronteira; `NewCatalog` valida via smart constructor com `errors.Join`.
- **R-TESTING-001:** suites whitebox, `fake.NewProvider()`, mocks por IIFE para o teste de `AgentRuntime`.
- **DMMF:** state-as-type (`CapabilityMode`), smart constructor (`NewCatalog`), sem string livre; anti-padrões (Result/Either custom, currying, DSL) não introduzidos.

### Arquivos Relevantes e Dependentes
- **Novos:** `internal/agent/application/capability/spec.go`, `capability/catalog.go`, `capability/build.go` (+ `_test.go` correspondentes).
- **Modificados:** `internal/agent/application/services/agent_runtime.go` (deriva do catálogo; remove `workflowFor`/`toolFor`), `internal/agent/application/services/daily_ledger_agent.go` (`isDestructiveKind` via catálogo), `internal/agent/application/services/agent_workflows.go` ou `module.go` (constrói/injeta catálogo), `.agents/skills/mastra/SKILL.md` (+ `references/`).
- **Dependentes (não modificados, sob teste de não-regressão):** `internal/agent/application/workflow/registry.go`, `composite.go`, `internal/agent/domain/entities/run.go`, `internal/agent/domain/confirmation/`.

## Mapeamento Requisito → Decisão → Teste

| RF | Decisão de design | Teste |
|---|---|---|
| RF-01 | `CapabilitySpec` com 11 campos canônicos | unit (construção) |
| RF-02 | `CapabilityMode` tipo fechado + `Parse`/`IsValid` | unit (enum) |
| RF-03 | `BuildCatalog` no mesmo wiring de `buildRegistry` | unit + consistência registry↔catálogo |
| RF-04 | `Catalog.Lookup(kind) (spec, bool)` | unit (hit/miss) |
| RF-05 | `Catalog.List()` cópia defensiva, ordem estável | unit |
| RF-06 | classificação operacional mínima nos campos | unit + guard destrutivo |
| RF-07 | `Execute` usa `catalog.Classify` | suite `AgentRuntime` |
| RF-08 | `workflowFor`/`toolFor` removidos | inspeção + equivalência por kind |
| RF-09 | labels idênticos exceto drift documentado | equivalência por kind (RF-17) |
| RF-10 | teste-guard cobertura `routableKinds()` | unit (falha build na ausência) |
| RF-11 | fallback `conversational`/`""` em kind desconhecido | unit (`Classify`) |
| RF-12 | `isDestructiveKind` via `RequiresConfirmation` | unit destrutivo + não-regressão HITL |
| RF-13 | labels enumerados; sem alta cardinalidade | inspeção + suite `AgentRuntime` |
| RF-14 | skill `mastra` declara 5 seams reais | revisão da skill |
| RF-15 | checklist de extensão (6 pontos + registrar CapabilitySpec) | revisão da skill |
| RF-16 | suíte agent/workflow verde | execução da suíte |
| RF-17 | equivalência por kind catálogo×legado | unit (tabela por kind) |
