# Evolução do `internal/agent`: Tool Registry + OnboardingAgent + DailyLedgerAgent

> **Skill obrigatória:** toda alteração Go carrega `.agents/skills/go-implementation/SKILL.md` (Etapas 1–5)
> e respeita R-ADAPTER-001 (zero comentários, adapters finos), R-DTO-VALIDATE-001, R-TESTING-001,
> R0–R7 e o Padrão Obrigatório de Módulo do `AGENTS.md`.

## Contexto

`internal/agent` já é maduro (parse single-shot com JSON schema estrito via OpenRouter, onboarding
LLM com tool-calling no haiku, bindings → usecases, `agent_sessions`/`agent_decisions`, outbox,
circuit breaker, auditoria). O objetivo é torná-lo **robusto, evolutivo e production-ready** sem
reescrita, usando mastra apenas como inspiração conceitual. Fluxo obrigatório preservado:
`WhatsApp/Telegram → dispatcher → agent → OpenRouter → módulos (usecases)`.

## Decisões aprovadas (usuário)

1. **Arquitetura mastra-aligned: módulos = tools via Tool Registry no kernel.** Cada capacidade de
   módulo (`record_transaction`, `monthly_summary`, `list_cards`, `create_card`, `count_cards`,
   `configure_budget`, ...) é uma Tool; o `execute` é o `binding/*` adapter já existente
   (`adapter → usecase`). Não recriar bindings.
2. **Mecanismo diário = seleção estruturada de 1 tool por vez** (saída JSON estrita). Mantém
   `parse_intent.system.tmpl` + `ParseIntentJSONSchema` **intactos** (validados, flash-lite).
   Onboarding mantém tool-calling nativo no haiku.
3. **System prompt de tools gerado do registry**, reusando o texto já validado de `RenderToolSystem`
   (hoje dormente). Registry é fonte única e valida no build: toda tool tem binding; todo intent
   kind tem tool. Anti-drift.
4. **Entrega:** plano em `docs/runs` + PRs incrementais e contidos; implementação via subagents.

## Restrição inegociável — prompting validado

`internal/agent/application/prompting` é o modelo validado, deve funcionar 100%, sem falso positivo.
Os 4 `.tmpl` + funções de render NÃO podem regredir. Mudança permitida apenas aditiva (ex.:
`RenderToolSystem` derivar do registry produzindo texto equivalente; nunca tocar
`parse_intent.system.tmpl`/`onboarding.system.tmpl`/`persona.system.tmpl`/`budgets.system.tmpl`).

## Mapeamento mastra → mecontrola

| Conceito mastra | Tradução (NÃO copiar TS) |
|---|---|
| Agent (instructions+model+tools+memory) | `Agent{Handle}` em `application/agents/` sobre o kernel |
| Tool (schema + execute) | `ToolSpec` no registry; `execute` = `binding/*` adapter → usecase |
| Tool registry / composability | `tools.Registry` valida no build tool↔binding↔intent kind |
| Memory (working) | `agent_sessions` determinística; sem multi-turn ao LLM |
| Workflow (autor controla grafo) | dispatch determinístico do supervisor por intent kind |
| Supervisor agent | `IntentRouter` fino escolhe OnboardingAgent vs DailyLedgerAgent |

## Arquitetura alvo

```
dispatcher → IntentRouter (supervisor fino)
                 ├── onboarding em progresso? → OnboardingAgent.Handle()  (tool-calling nativo/haiku)
                 └── senão                    → DailyLedgerAgent.Handle()
                                                  ├── parse (seleção estruturada, flash-lite, prompt validado)
                                                  └── dispatch por intent kind → Tool(execute=binding) → usecase → outbox

Agent Kernel: LLM runtime (fallback chain + circuit breaker) | Tool Registry | leitura de sessão
              (working memory) | decision auditor
```

## Fases (PRs contidos; implementação via subagents)

### Fase A — Kernel + Tool Registry
- `internal/agent/application/tools`: `ToolSpec{Name, IntentKind, Description, ArgsSchema}` + `Registry`
  com validação no build (toda tool tem execute/binding; todo intent kind dispatchável tem tool).
- `RenderToolSystem` passa a derivar do registry (ativa o dormente; texto equivalente ao validado).
- Kernel agrega runtime LLM, registry, decisionAuditor e leitura de sessão. Interface
  `Agent{Handle(ctx, principal, InboundMessage) RouteResult}`.

### Fase B — OnboardingAgent
- Extrair ramo `onboardingRunner.Run` + `degradeOnboarding` + `onboarding.Continue`
  (intent_router.go:520-542) para `application/agents/onboarding_agent.go`. Testes testify/suite.

### Fase C — DailyLedgerAgent
- Extrair budget session + `parser.Parse` + dispatch por kind para
  `application/agents/daily_ledger_agent.go`. Reusa o registry para dispatch. Prompts validados intactos.

### Fase D — Supervisor fino
- `IntentRouter.route()` reduzido a: validar → escolher agent → delegar → auditar/publicar.
  `module.go` monta kernel + registry + 2 agents (DI explícita). RouteWhatsApp/Telegram e outbox intactos.

### Fase E — Endurecimento (corrigido por leitura de código real)
- **REAL (manter): ambiguidade/not-found de categoria → resposta determinística.** Hoje
  `routeLogExpense`/`routeLogIncome` (intent_router.go:706-712, 724-730) colapsam
  ambiguous/not-found/no-hint num único `registerFailedText`. Fix: erro tipado carregando candidatos
  + mensagem que pede esclarecimento (single-turn; não toca prompting).
- **REAL (manter): erros de invariante amigáveis** (ex.: dia inválido em recorrência) com mensagem de correção.
- **REAL (manter): observabilidade JSON inválido vs recusa** em parse_inbound.go (outcome distinto).
- ~~FALSO POSITIVO: métrica de exaustão do fallback chain~~ — já existe `agent_llm_fallback_exhausted_total` (fallback_chain.go:35-39,101).
- ~~FALSO POSITIVO: validação pré-write de cartão~~ — já existe; `CardPurchaseCreatorAdapter` (transaction_query.go:88-95) checa cartão antes de qualquer escrita.

### Limpeza
- `internal/agent/infrastructure/llm/prompts/` é pacote órfão (duplica RenderSystem/RenderUser/
  ParseIntentJSONSchema, sem referências) — remover em PR próprio.

## Validação

- Por fase: `go build ./...`, `go test ./internal/agent/... -count=1 -race`, gates R-ADAPTER-001,
  R-DTO-VALIDATE-001, R-TESTING-001.
- Paridade: suíte `internal/agent/e2e` deve passar sem alteração após A–D (refactor preservante).
- Prompting: `git diff internal/agent/application/prompting` deve ficar vazio nos `.tmpl`; só
  `prompts.go` muda se `RenderToolSystem` passar a derivar do registry, com teste comparando o texto.
- Registry: teste que falha se algum intent kind dispatchável não tiver tool, ou tool sem binding.

## Non-goals (MEMORY — não re-propor)

- Multi-turn ao LLM (P2-1): conversa single-turn; working memory determinística, não histórico no prompt.
- Gate de confirmação financeira: lançamentos report-only; priorizar idempotência/rastreabilidade.
- Abstração de tempo (Clock/now injetado): `time.Now().UTC()` inline.
- Tool-calling nativo no diário: avaliado; mantido só no onboarding (flash-lite é flaky em tool-calling).
