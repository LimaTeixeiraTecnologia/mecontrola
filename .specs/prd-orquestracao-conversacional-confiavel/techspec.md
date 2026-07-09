<!-- spec-hash-prd: 67713f8f900642b871bfc248104879765aa242954b30e5bb026527012c84a1e9 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Orquestração Conversacional Confiável do Agente MeControla

> PRD: `.specs/prd-orquestracao-conversacional-confiavel/prd.md` (spec-version 1).
> ADRs: `adr-001-guard-chain-cor.md`, `adr-002-runtime-robustez-truncamento.md`,
> `adr-003-cardid-provenance.md`, `adr-004-scorers-comportamentais.md`,
> `adr-005-golden-harness-gate.md`.
> Governança vinculante: `go-implementation` (Go 1.26.5), `mastra`, `domain-modeling-production`,
> `design-patterns-mandatory`, `R-AGENT-WF-001`, `R-WF-KERNEL-001`, `R-ADAPTER-001`, `R-TESTING-001`,
> `R-DTO-VALIDATE-001`.

## Resumo Executivo

A solução materializa as regras críticas de segurança conversacional — hoje presas no prompt
monolítico de `mecontrola_agent.go:17-250` — em **código determinístico observável**, sem reescrever o
substrato `internal/platform` e preservando o contrato público `BuildMeControlaAgent`. O eixo é uma
**cadeia de guardas conversacionais no padrão Chain of Responsibility** (confirmado pelo seletor
determinístico de `design-patterns-mandatory`: `status=ok`, primário `Chain of Responsibility`, score 4,
alternativa simples "sequência explícita de funções", sem complementar), implementada como um decorator
`agent.Agent` no consumidor `internal/agents`. O `MultiItemGuard` atual é **absorvido como o primeiro
handler pré-LLM** da cadeia. Handlers pós-LLM impõem verbatim-relay, no-empty-answer, no-internal-terms,
no-success-without-tool e proveniência de cartão.

Em paralelo, a técnica endurece o **runtime da plataforma agente** (extensão aditiva, não reescrita):
truncamento por length vira o estado fechado `ToolOutcomeTruncated` com falha-segura; os gaps hoje
silenciosos (`RunStore.Update` engolido, `MessageStore.Append` sem métrica, erro só do primeiro tool)
passam a emitir métrica e log; e o teto de tokens sobe para reduzir truncamento falso. A **avaliação**
ganha scorers comportamentais code-based rodando em produção (persistidos em `platform_scorer_results`)
mais um golden set versionado com harness em dois níveis: determinístico no CI por-PR e real-LLM ≥ 0,90
pré-deploy. O gate pós-deploy usa agregados produtivos com amostra mínima, margem e a métrica
`tool-call-accuracy` redefinida. A dívida hard R5.26 (identificadores `_` nos workflows) é fechada por
renome mecânico.

## Arquitetura do Sistema

### Visão Geral dos Componentes

**Novos (consumidor `internal/agents`):**

- `internal/agents/application/agents/guard_chain.go` — decorator `guardChainAgent` (Chain of
  Responsibility) implementando `agent.Agent`; runner que percorre `PreGuard`/`PostGuard` ordenados,
  com observabilidade por handler. Substitui `WithMultiItemGuard` (ADR-001).
- `internal/agents/application/agents/guards/` — handlers: `multi_item.go` (absorve o guard atual),
  `verbatim_relay.go`, `empty_answer.go`, `internal_terms.go`, `success_without_tool.go`,
  `card_provenance.go` (ADR-001, ADR-003).
- `internal/agents/application/scorers/` — novos scorers code-based comportamentais e ajuste do
  registro em `BuildMeControlaScorers` (ADR-004).
- `internal/agents/application/golden/` — fixtures do golden set (sintéticos + incidentes
  anonimizados) e o harness de avaliação em dois modos (ADR-005).

**Modificados (plataforma — extensão aditiva, sem reescrita; kernel `internal/platform/workflow`
intocado):**

- `internal/platform/agent/types.go` — nova constante fechada `ToolOutcomeTruncated` (ADR-002).
- `internal/platform/agent/runtime.go` — consumir `result.TruncatedByLength`; observar
  `RunStore.Update`; métrica de `MessageStore.Append`; agregar erros de múltiplas tools (ADR-002).
- `internal/agents/application/agents/scoring_hooks.go` — `AfterTool` passa a capturar o `argsJSON` em
  `ToolCallRecord.Args` (hoje descartado como `_ []byte`), habilitando scorers que inspecionam
  argumentos (ADR-004).
- `internal/agents/application/agents/mecontrola_agent.go` / wiring — teto de tokens elevado e
  **configurável via `AGENT_MECONTROLA_MAX_TOKENS`** (default 3072) passado a `WithDefaultMaxTokens`;
  wrap por `WithGuardChain(...)` em vez de `WithMultiItemGuard(...)` (ADR-001, ADR-002).
- Tools consumidoras de `cardId` (`register_expense`, `create_recurrence`, `query_card_invoice`) —
  validação de existência do cartão para o `resourceId` (ADR-003).
- `internal/agents/application/workflows/onboarding_workflow.go`,
  `budget_creation_workflow.go` — renome dos identificadores `_`-prefixados (RF-44).

### Fluxo de dados (inbound → resposta)

```text
WhatsApp inbound
  -> consumer (resumers de pending/confirm ANTES do agente — inalterado)
  -> AgentRuntime.Execute
     -> ThreadGateway.GetOrCreate -> RunStore.Insert(Running)
     -> guardChainAgent.Execute
        -> [PRÉ] PreGuards em ordem (multi-item, ...) -> curto-circuita (Result) OU passa
        -> next.Execute (LLM loop tool-calling)          [economia RF-48: pré curto-circuita sem LLM]
        -> [PÓS] PostGuards em ordem (verbatim, empty, internal-terms, success-without-tool,
                 card-provenance) -> pode sobrescrever Result para fallback seguro
     -> runtime: truncamento? empty? writeToolGuardFailed? -> RunStatus/ToolOutcome (ADR-002)
     -> MessageStore.Append (com métrica de erro) ; closeRun (Update observado)
     -> ScoringHooks.AfterExecute -> ScorerRunner.Observe (async; scorers comportamentais)
```

Observação de fronteira: os *resumers* de pendência/confirmação/expiração/cancelamento vivem no
**consumer inbound** (antes do runtime) e permanecem inalterados — a cadeia de guardas é estritamente
**agent-level** e não os duplica. A economia de LLM de RF-48 para esses casos já ocorre no resumer;
a cadeia adiciona a economia do multi-item (pré-LLM) e a segurança pós-LLM.

## Design de Implementação

### Interfaces Chave

Cadeia de guardas (state-as-type na decisão; DMMF — estados ilegais irrepresentáveis):

```go
type GuardDecision struct {
	Handled bool
	Result  agent.Result
}

type PreGuard interface {
	Name() string
	Inspect(ctx context.Context, in agent.Request) GuardDecision
}

type PostGuard interface {
	Name() string
	Inspect(ctx context.Context, in agent.Request, out agent.Result) GuardDecision
}

type guardChainAgent struct {
	agent.Agent
	pre     []PreGuard
	post    []PostGuard
	metrics guardMetrics
	o11y    observability.Observability
}

func WithGuardChain(a agent.Agent, o11y observability.Observability, pre []PreGuard, post []PostGuard) agent.Agent
```

`Execute` percorre `pre` em ordem; o primeiro `GuardDecision{Handled:true}` curto-circuita (sem LLM).
Caso nenhum trate, delega a `g.Agent.Execute`; em seguida percorre `post`, permitindo que um handler
sobrescreva o `Result`. `Stream` é delegado por embedding (paridade com o guard atual). Cada handler
emite `agent_guard_decisions_total{agent_id, guard, decision}` com `decision ∈ {pass, handled}`
(cardinalidade fechada) e um atributo de span.

Interface `agent.Agent` a implementar (verbatim do substrato, `ports.go:118-123`):

```go
type Agent interface {
	ID() string
	Instructions() string
	Execute(ctx context.Context, in Request) (Result, error)
	Stream(ctx context.Context, in Request) (ResultStream, error)
}
```

Truncamento como estado fechado (extensão idiomática de `types.go:48-101`):

```go
const (
	ToolOutcomeRouted ToolOutcome = iota + 1
	ToolOutcomeClarify
	ToolOutcomeUsecaseError
	ToolOutcomeMissingResolver
	ToolOutcomeReplay
	ToolOutcomeReconciled
	ToolOutcomeTruncated
)
// String()/IsValid()/ParseToolOutcome ganham o caso "truncated".
```

Scorer comportamental code-based (padrão real de `scorer.Scorer`, `scorer.go:59-63`):

```go
type Scorer interface {
	ID() string
	Kind() ScorerKind
	Score(ctx context.Context, s RunSample) (ScoreResult, error)
}
// RunSample{Input, Output, ExpectedOutput string; ToolCalls []ToolCallRecord{ID,Name string; Args map[string]any}; Metadata map[string]any}
// ScoreResult{Score float64; Reason string; Metadata map[string]any}
```

### Modelos de Dados

- Sem mudança de schema Postgres (fora de escopo do PRD). `platform_scorer_results` já persiste os
  novos scores (mesmo formato `ScorerResult`).
- `ToolCallRecord.Args` (em `scorer.RunSample`) passa a ser **populado** pela `ScoringHooks.AfterTool`
  a partir do `argsJSON` que hoje é descartado. Sem alteração de tipo — o campo já existe.

### Guardas (enforcement) vs Scorers (measurement)

| Regra crítica (US/PRD) | Mecanismo | Local |
|------------------------|-----------|-------|
| Multi-item (pré-LLM, RF-04/05/48) | PreGuard (absorve MultiItemGuard) | `guards/multi_item.go` |
| Verbatim relay (RF-12) | PostGuard: se tool verbatim gerou `verbatimText` e `Content != verbatimText` → força verbatim | `guards/verbatim_relay.go` |
| Resposta vazia (RF-22) | PostGuard: `Content` vazio → fallback seguro (runtime marca Failed) | `guards/empty_answer.go` |
| Termos internos (RF-10) | PostGuard: blocklist fechada → sanitiza/override conservador | `guards/internal_terms.go` |
| Sucesso sem tool (RF-09) | PostGuard: marcador de sucesso sem write-tool bem-sucedido e sem verbatim → fallback + Failed | `guards/success_without_tool.go` |
| cardId provenance (RF-16/17/18) | PostGuard (tool-consumidora chamada sem `resolve_card`/`list_cards` prévio → pedir escolha) + validação de existência na tool | `guards/card_provenance.go` + tools |
| expected_tool, required_args, no_hallucination, verbatim_required, whatsapp_format, no_internal_terms, no_empty_answer, no_duplicate_write, month_reference_correctness (RF-30) | Scorers (mensuração) | `scorers/` |

Handlers pós-LLM só agem sobre violação **inequívoca e determinística** (vazio, mismatch de verbatim,
token da blocklist, sucesso-sem-tool, consumo de cartão sem resolução prévia) — nunca reescrevem uma
resposta válida. Isso evita regressão de fluidez.

### Scorers comportamentais — intrínsecos (prod) vs oracle-dependentes (golden)

Decisão do usuário: code-based em produção + reuso no golden. Nuance técnica (ADR-004): scorers que
não precisam de gabarito rodam em **produção** (`AlwaysSample`, persistidos, sinal contínuo para o gate
pós-deploy); scorers que precisam de expectativa por caso rodam **apenas no golden set** (a expectativa
vem de `RunSample.Metadata`/`ExpectedOutput` do fixture):

- **Intrínsecos (prod + golden):** `no_empty_answer`, `whatsapp_format`, `no_internal_terms`,
  `verbatim_required` (compara com `verbatimText` da tool), `no_duplicate_write` (conta write-tools
  bem-sucedidos não-replay em `ToolCalls`), `no_hallucination` (marcador de sucesso sem tool
  bem-sucedido → 0), `required_args` (write-tool sem os args de domínio obrigatórios → penaliza),
  `month_reference_correctness` (se tool de mês chamada, `monthRefKind` presente e consistente com o
  texto — mês nomeado sem ano ⇒ `named_without_year`).
- **Oracle-dependentes (golden apenas):** `expected_tool` (usa `Metadata["expected_tool"]` do caso).

Os 3 scorers atuais permanecem registrados (`tool-call-accuracy`, `completeness`, `categorization`) por
continuidade de baseline (RF-29).

### cardId provenance (RF-16/17/18) — ADR-003

Duas camadas determinísticas:
1. **Tool-level:** `register_expense`/`create_recurrence`/`query_card_invoice`, ao receber `cardId`,
   validam que ele resolve para um cartão real do `resourceId` (usecase de leitura). UUID fabricado →
   `card não encontrado` → erro de domínio limpo (não crash), transformado em `clarify`/fallback.
2. **Chain-level (PostGuard `card_provenance`):** se uma tool consumidora de cartão aparece em
   `Result.ToolCalls` sem que `resolve_card` ou `list_cards` a preceda na mesma sequência → override
   para pedir escolha de cartão. Usa apenas nomes/ordem de tools (`ToolCallRecord.Tool`), sem depender
   de args.

### tool-call-accuracy redefinida (RF-42) — camada de agregação

A redefinição é do **gate pós-deploy**, computada por consulta sobre `platform_runs` +
`platform_scorer_results`, não por mudança de scorer. Denominador exclui runs cujo `outcome ∈
{clarify, replay}` (multi-item, pendência, saudação, idempotência) — i.e., mede acurácia só onde uma
tool era esperada. Especificado no runbook/dashboard (ADR-005), com **amostra mínima** (N ≥ 100 runs ou
≥ 14 dias) e **margem absoluta** por métrica (scorers ≥ baseline + 0,05; taxa de falha ≤ baseline).

## Pontos de Integração

- **OpenRouter** via `internal/platform/llm` (único provider; sem fallback chain — fora de escopo). O
  harness real-LLM consome-o sob tag `realllm`/nightly/pré-deploy.
- **Postgres de produção** (`platform_runs`, `platform_messages`, `platform_scorer_results`,
  `workflow_runs`) — leitura para agregados do gate pós-deploy; sem alteração de schema.

## Abordagem de Testes

### Testes Unitários (CI por-PR, determinístico)

Padrão canônico `testify/suite` whitebox + `mockery` (R-TESTING-001):

- Cada `PreGuard`/`PostGuard`: table-driven cobrindo trata/passa, incluindo regressão de equivalência
  do multi-item absorvido (mensagem verbatim + `ToolOutcomeClarify` idênticos ao guard atual — RF-02) e
  o caso brasileiro `R$ 1.234,56` (não dispara multi-item — RF-05).
- Cada scorer comportamental: entradas `RunSample` fixas → `ScoreResult` esperado (intrínsecos).
- `guardChainAgent`: ordem, curto-circuito pré-LLM (não chama `next`), override pós-LLM, emissão de
  métrica por handler.
- Runtime (ADR-002): truncamento → `ToolOutcomeTruncated` + Failed + fallback; erro de `RunStore.Update`
  não reporta sucesso; erro de `MessageStore.Append` emite métrica; agregação de múltiplos erros de tool.
- Tools de cartão: `cardId` inexistente → erro de domínio limpo (ADR-003).

### Testes de Integração

Critérios atendidos (fronteira de IO crítica + histórico de falso-verde em runs do agente + custo
proporcional): **sim**. `//go:build integration` com testcontainers Postgres para: persistência de
`ScorerResult` dos novos scorers; retomada de workflow pós-deploy (RF-45/46) com reapers; idempotência
de escrita (WAMID duplicado → efeito único).

### Testes E2E — harness real-LLM (tag `realllm`, pré-deploy)

Dirige `BuildMeControlaAgent` com OpenRouter real sobre o golden set (RF-35/36), computando o **gate
≥ 0,90** por categoria de cenário: registro despesa/receita, C1–C7, cartões (resolve→registro),
orçamento/mês (named_without_year + verbatim + mês por extenso), recorrências, onboarding, pendências,
confirmações, follow-up, erro de tool, ambiguidade, formato WhatsApp, ausência de termos internos.
Cada caso declara `input`, `expectedTool`, `expectedArgs` (subconjunto), `expectedOutcome` e
`responseProperty` verificável. Fixtures = sintéticos + incidentes reescritos/anonimizados (RF-37).

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Dívida R5.26 (RF-44)** — renome mecânico dos `_`-prefixados (sem comportamento; desbloqueia gates
   de governança limpos para as demais mudanças).
2. **Runtime robustez (ADR-002)** — `ToolOutcomeTruncated`, consumo de `TruncatedByLength`, observação
   de `Update`/`Append`, agregação de erros de tool, teto de tokens. Base observável para o resto.
3. **Captura de args em `ScoringHooks.AfterTool` (ADR-004)** — pré-requisito dos scorers que inspecionam
   argumentos.
4. **Cadeia de guardas (ADR-001)** — `guardChainAgent` + handlers; absorver MultiItemGuard; trocar o
   wrap em `BuildMeControlaAgent`. `card_provenance` depende de ADR-003.
5. **cardId provenance (ADR-003)** — validação nas tools + PostGuard.
6. **Scorers comportamentais (ADR-004)** — intrínsecos (prod) + oracle-dependente (golden).
7. **Golden set + harness + gate pós-deploy (ADR-005)** — fixtures, harness dois níveis, queries de
   agregação/runbook.

Paralelizável: (1) e (2) independentes; (3) independente; (5) antes de (4) apenas para o handler
`card_provenance` (o resto de (4) não depende de (5)); (6) depende de (3); (7) depende de (4)(5)(6).

### Dependências Técnicas

- Go 1.26.5 (`go.mod`). Sem novas dependências externas.
- `OPENROUTER_*` para o harness real-LLM (não roda no CI por-PR).
- Testcontainers para integração.

## Monitoramento e Observabilidade

Métricas novas (cardinalidade fechada — RF-33; herda R-TXN-004/R-AGENT-WF-001.5):

- `agent_guard_decisions_total{agent_id, guard, decision}` — `decision ∈ {pass, handled}`.
- `agent_run_truncated_total{agent_id}` — truncamento por length (RF-23).
- `agent_run_update_errors_total{agent_id}` — falha de `RunStore.Update` (RF-26).
- `agent_message_append_errors_total{agent_id, role}` — falha de `MessageStore.Append` (RF-25);
  `role ∈ {user, assistant}`.
- Scorers comportamentais reusam `scorer_runs_total{scorer_id, kind, outcome}` e
  `scorer_duration_seconds{scorer_id, kind}` (já existentes).

Regra de consistência (RF-26): em falha de `RunStore.Update`, o runtime **não** incrementa
`agent_runs_total` para aquele run (emite `agent_run_update_errors_total`), evitando reportar sucesso de
estado não persistido.

Logs estruturados (sanitizados, sem conteúdo de mensagem — RF-32/34): `run_id`, `agent_id`, `status`,
`outcome`, `stage`, `tool`, `duration_ms`, erro sanitizado, `scorer_id`, `score`, `workflow`, estado de
pendência. Sem `user_id`/`thread_id`/`resource_id` como label de métrica (RF-33).

Critérios de alerta (runbook — ADR-005): subida de `agent_run_truncated_total`,
`agent_run_update_errors_total`, `agent_message_append_errors_total`; queda dos scorers comportamentais
abaixo da margem; presença de escrita duplicada (`no_duplicate_write` < 1).

## Considerações Técnicas

### Decisões Chave

- **ADR-001** — Cadeia de guardas conversacionais (Chain of Responsibility), absorvendo MultiItemGuard.
- **ADR-002** — Robustez do runtime: truncamento como `ToolOutcomeTruncated` + falha-segura, teto de
  tokens elevado, observabilidade dos gaps de persistência (extensão aditiva do primitivo de agent).
- **ADR-003** — Proveniência de cardId: validação de existência na tool + PostGuard de cadeia.
- **ADR-004** — Scorers comportamentais: intrínsecos em prod + oracle-dependente no golden; captura de
  args em `AfterTool`; redefinição de `tool-call-accuracy` na agregação.
- **ADR-005** — Golden set versionado + harness dois níveis (determinístico CI / real-LLM ≥ 0,90
  pré-deploy) + gate pós-deploy com amostra mínima e margem.

### Riscos Conhecidos

- **Override pós-LLM regredir fluidez:** mitigado limitando handlers a violações inequívocas e cobrindo
  com golden de regressão (respostas válidas não são tocadas).
- **`no_hallucination`/`success_without_tool` por marcador textual:** heurística de marcadores de
  sucesso pode ter falso positivo/negativo; mitigado por lista conservadora + medição contínua via
  scorer (não só enforcement) + revisão do conjunto de marcadores no golden.
- **Teto de tokens maior → custo/latência:** mitigado por valor moderado (ADR-002) e monitoração de
  `agent_llm_tokens_total`/latência.
- **Alterar `internal/platform/agent`:** risco de tocar substrato; mitigado por serem mudanças
  **aditivas** (novo enum, novos ramos de observação) sem alterar contrato público nem o kernel de
  workflow (que permanece intocado — R-WF-KERNEL-001 preservado).
- **Oracle do gate pós-deploy:** `tool-call-accuracy` redefinida ainda é proxy; mitigado por amostra
  mínima + margem + decisão humana rastreável por `run_id` (RF-51/52).

### Conformidade com Padrões

- `R-AGENT-WF-001` — roteamento por registry (sem `switch case intent.Kind`); tools finas sem
  regra/SQL/branching; LLM só nas call-sites sancionadas; Run auditável; estados fechados; pending step
  antes de clarify (resumers inalterados). Guardas são adapters finos de decisão, sem SQL nem regra de
  domínio (delegam à tool/usecase).
- `R-WF-KERNEL-001` — kernel `internal/platform/workflow` **não é tocado**.
- `R-ADAPTER-001` — zero comentários em Go de produção; guardas/scorers/handlers sem SQL direto.
- `R-TESTING-001` — testify/suite whitebox + mockery; `fake.NewProvider()` nos testes de use case.
- `R-DTO-VALIDATE-001` — inputs de tool com `Validate()` quando aplicável.
- R5.26 (`go-implementation`) — nenhum identificador `_`-prefixado; RF-44 fecha os existentes.

### Arquivos Relevantes e Dependentes

- `internal/agents/application/agents/mecontrola_agent.go` (wrap + teto de tokens)
- `internal/agents/application/agents/multi_item_guard.go` (absorvido/removido)
- `internal/agents/application/agents/guard_chain.go` (novo) + `guards/*` (novo)
- `internal/agents/application/agents/scoring_hooks.go` (captura de args)
- `internal/agents/application/scorers/mecontrola_scorers.go` (novos scorers)
- `internal/agents/application/golden/*` (novo)
- `internal/agents/application/tools/{register_expense,create_recurrence,query_card_invoice}.go`
- `internal/agents/application/workflows/{onboarding,budget_creation}_workflow.go` (renome R5.26)
- `internal/platform/agent/{types.go,runtime.go}` (extensão aditiva)
- `internal/agents/module.go` (wiring de scorers/guardas)
- Runbook/dashboards de produção (gate pós-deploy)
