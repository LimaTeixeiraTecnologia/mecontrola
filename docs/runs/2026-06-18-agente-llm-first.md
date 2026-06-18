# Run — Agente conversacional LLM-first robusto (internal/agent)

- Data: 2026-06-18
- Branch base: main
- Skill obrigatória: **`go-implementation` (Etapas 1–5)** — carregada antes de qualquer edição Go.
  Go declarado em `go.mod`: **1.26.4** (todos os recursos R7 disponíveis).
- Regras transversais obrigatórias: R-ADAPTER-001 (zero comentários Go, adapters finos),
  R-TXN-WORKFLOWS-001, governança `.claude/rules/governance.md`.
- Modelagem de domínio: **DMMF** (`domain-modeling.md`) onde houver tipo/estado.

## Context

O usuário quer uma conversa fluida e confiável via WhatsApp em que a LLM (Gemini 2.5 Flash Lite
via OpenRouter) entenda intenções e execute **ações reais** nos módulos: cadastrar/contar cartões,
configurar orçamento mensal com split percentual sobre as **5 categorias canônicas do produto**
— Custos fixos, Metas, Prazeres, Liberdade financeira e Conhecimento — (ex.: 10 mil → Custos fixos 35%,
Metas 20%, Prazeres 15%, Liberdade financeira 20%, Conhecimento 10%; soma = 100%) e lançar transações
(salário, supermercado, iFood) na categoria correta. Objetivo: MVP robusto, production-grade, sem
falso positivo.

**Decisão do usuário (2026-06-18):** manter apenas as 5 categorias existentes; **não** adicionar
"Conforto". Isso elimina qualquer mudança de taxonomia/constraint — o modelo de 5 já é totalmente
suportado (banco, domínio `RootSlug`, onboarding).

Esta avaliação foi verificada adversarialmente (subagents) contra o código real, corrigindo um
relatório inicial excessivamente otimista.

## Governança e padrões inegociáveis

Aplicáveis a TODAS as fases deste plano, sem exceção, flexibilização ou atalho por deadline:

- **Skill `go-implementation` obrigatória** — carregar e executar as **Etapas 1–5** do SKILL.md antes
  de qualquer edição Go; verificar versão no `go.mod`; rodar o Checklist R0–R7 de `references/build.md`
  e reportar resultado.
- **0 (zero) comentários em código Go** — R-ADAPTER-001.1 `[HARD]`. Exceções únicas: `// Code generated`,
  `//go:`, `//nolint:` com justificativa na mesma linha. Gate de verificação deve retornar vazio.
- **Adapters finos** — R-ADAPTER-001.2: fluxo `adapter → usecase`; sem SQL direto, regra de negócio ou
  branching de domínio em `handlers/`, `consumers/`, `producers/`, `jobs/handlers/`.
- **DMMF (Domain Modeling Made Functional) onde houver modelagem de domínio** — seguir
  `domain-modeling.md` (prevalece sobre estilo Uber para tipo/estado): smart constructors para todos os
  VOs/commands (validação só aí), discriminated unions para estados, state-as-type, workflow pipeline
  com `Decide*` puro. **Proibido** (hard): Result/Either customizado, currying, DSL de pipeline.
  Aplica-se especialmente a: argumentos de tool, alocações de orçamento (`RootSlug`+`BasisPoints`,
  invariante soma=10000), estado da sessão de slot-filling (union de ações pendentes).
- **R0/R5.12/R6/R7.6** — sem `init()`; sem `panic` em produção; `context.Context` em toda fronteira de
  IO (interface no consumidor); `errors.Join`/`fmt.Errorf("ctx: %w", err)` para erros. Sem abstrair
  tempo (usar `time.Now().UTC()` inline). Goroutines canceláveis, shutdown cooperativo, sem leak.
- **MVP robusto, eficiente, econômico e production-proof** — não-negociável: caminho mínimo que
  funciona ponta a ponta com evidência; eficiência de tokens (prompt enxuto, memória curta com TTL e
  N pequeno, reuso de seed/resolver existentes em vez de chamadas extras de LLM); idempotência por
  `event_id`; observabilidade nos novos caminhos; sem falso positivo (cada claim com teste/evidência).

## Persistência (inegociável)

Toda ação do agente DEVE persistir de verdade através dos **módulos existentes** em `internal/<módulo>/`,
nunca via caminho próprio ou mock:

- **Escritas de domínio** passam pelos usecases já existentes (sem reimplementar regra):
  `card.CreateCard`, `budgets.CreateBudget`/`ActivateBudget`, `transactions.CreateTransaction`/
  `CreateCardPurchase`. O agente é orquestrador fino (adapter → usecase), conforme R-ADAPTER-001.2.
- **Estado de conversa/sessão** (slot-filling + memória curta) é persistido seguindo o **mesmo padrão
  de persistência dos módulos**: `internal/platform/database` — leitura por repositório via DI,
  escrita via `uow` + `RepositoryFactory` (padrão agnóstico do projeto). Tabela nova `agent_sessions`
  com migration própria, repositório em `internal/agent/infrastructure/repositories/postgres/` e
  wiring no `module.go`. Nada de estado em memória de processo (R6.6: zero estado global).
- **Idempotência**: writes idempotentes por `event_id` (dedup já existe no dispatcher WhatsApp);
  reentrega da Meta não pode duplicar lançamento nem orçamento.

## Economia & eficiência (best practices)

Princípios aplicados a todas as fases para manter a conversa fluida, barata e robusta:

1. **1 chamada de LLM por mensagem** — tool-calling resolve "entender + extrair args" numa única ida;
   proibido pipeline classificar-depois-agir em 2 chamadas.
2. **Resolução de categoria é DB, não LLM** — manter `category_hint` → `SearchDictionary` (Postgres,
   custo zero de token); o LLM não raciocina sobre a árvore de categorias.
3. **Estado estruturado > transcrição** — injetar um `pending_action` compacto (1-2 linhas) em vez de
   reenviar os turnos crus; maior corte de tokens e mais determinístico.
4. **Prompt caching no prefixo estático** — system prompt + definições de tools são fixos → marcar
   como prefixo cacheável (OpenRouter/Gemini). Paga os tokens das tools uma vez, não a cada mensagem.
5. **Modelo barato como padrão** — Gemini 2.5 Flash Lite primário; fallback chain só em falha.
6. **Fast-path determinístico de confirmação** — "sim/confirma" com `pending_action` aguardando
   confirmação executa SEM nova chamada de LLM.
7. **Memória curta com TTL e N pequeno** — guardar só os últimos 2-4 turnos; expira sozinho.
8. **Guardas production-proof na conversa** — confirmar antes de escrita sensível; nunca inventar
   valor monetário; categoria ambígua → listar e perguntar (não default silencioso); fallback gracioso.

## Padrão de integração entre módulos (decisão arquitetural — 2026-06-18)

**O agente chama USECASES dos outros módulos, nunca os handlers HTTP.** `internal/agent` e o handler
HTTP são adapters irmãos sobre a mesma camada de aplicação; o ponto de integração correto no monólito
modular é o usecase. Verificado: `internal/card/.../handlers/create.go` (handler fino: principal +
parse + `usecase.Execute` + map erro→status, zero regra) e `internal/agent/.../dispatcher/cards_adapter.go`
(adapter fino: chama o mesmo `CreateCardUseCase.Execute(cardinput.CreateCard{...})`).

Proibido: chamar handler in-process (httptest); chamar a REST API pela rede; o agente abrir UoW/SQL
sobre tabelas de outro módulo; reimplementar validação de domínio.

Contrato das tools (a manter na refatoração):
- Interface no consumidor (R6): o agente declara a interface do usecase localmente (já faz).
- Reusar os DTOs de input/output dos módulos (`cardinput.CreateCard`, `budgets…CreateBudgetInput`) — sem DTOs paralelos.
- **Injetar o principal no contexto** antes de chamar o usecase (substitui o middleware `require_user`) — `auth.WithPrincipal`.
- Não revalidar regra; traduzir erro sentinel do domínio (`errors.Is(err, domain.ErrNicknameConflict)`) → mensagem de conversa.
- **1 tool = 1 usecase = 1 transação** (UoW do próprio usecase); sem escrita atômica multi-módulo pelo agente (usar evento/outbox se necessário).

## Maturidade verificada (estado atual)

Caminho **vivo** = `IntentRouter.RouteWhatsApp/RouteTelegram` → `ParseInbound` (16 intent kinds) →
adapters. O pipeline `HandleInboundMessage` + `IntentDispatcher` é **código morto** (instanciado em
`internal/agent/module.go:493-506`, nunca chamado).

| Dimensão | Estado | Evidência |
|---|---|---|
| OpenRouter como fonte da verdade | ✅ Maduro | `infrastructure/providers/openrouter/client.go` — POST real `/api/v1/chat/completions`, Bearer, circuit breaker, fallback chain |
| Gemini 2.5 Flash Lite | ✅ Configurado | `.env.example:256` `google/gemini-2.5-flash-lite` + fallbacks; allowlist em `domain/valueobjects/model_slug.go` |
| WhatsApp receber | ✅ Maduro | webhook + HMAC-SHA256 + dedup + rate-limit + janela anti-replay; montado `/api/v1/whatsapp` em `cmd/server` |
| WhatsApp enviar | ✅ Maduro | `internal/onboarding/infrastructure/http/client/meta/client.go` → Graph API; gateway `SendTextMessage` |
| Lançar transação c/ categoria | ✅ Funciona no caminho vivo | `application/usecases/log_transaction_from_agent.go:128-180` resolve `category_hint` → UUID via `SearchDictionary`; Principal injetado |
| Conhecimento dos outros módulos | ⚠️ Parcial | reads/writes de cards, transactions, budgets, categories cabláveis; mas só via intents fixos |

## Gaps reais (bloqueadores do objetivo)

1. **Sem memória de conversa (stateless).** `LLMRequest{SystemPrompt, UserMessage}` não tem campo de
   histórico; o seed só carrega categorias/cartões/data. Fluxo multi-turno ("10 mil reais" e depois
   os percentuais) é impossível. (`application/interfaces/llm_provider.go`, `infrastructure/loader/prompt_context_loader.go:54-92`)
2. **Configuração de orçamento quebrada para o caso de uso.** `configure_budget` não tem payload;
   apenas dispara `onboarding.StartBudgetConfiguration`, que força **split fixo 5 categorias
   (40/10/15/20/15)** e descarta percentuais ditos pelo usuário. (`application/services/intent_router.go:608-627`,
   `internal/onboarding/domain/services/onboarding_workflow.go:558-590`)
3. ~~Taxonomia incompleta (Conforto)~~ — **RESOLVIDO por decisão do usuário (2026-06-18)**: manter
   apenas as 5 categorias canônicas. Não é mais um gap. O modelo de 5 está totalmente suportado:
   - **Banco**: seed `mecontrola.categories` + CHECK constraints `budgets_allocations_root_chk`/
     `budgets_expenses_root_chk` (`migrations/000001_initial_baseline.up.sql:1344,1371`) já listam os 5.
   - **Domínio budgets**: enum `RootSlug` (`root_slug.go`) cobre os 5 (`ParseRootSlug`/`CanonicalOrder`).
   - **Domínio onboarding**: `CategoryKind` + `SuggestDefaultSplit` (40/10/15/20/15) + `mapCategoryKindToRootSlug`.
   Convenção: slug de categoria usa hífen (`liberdade-financeira`); root slug de budget usa
   prefixo+underscore (`expense.liberdade_financeira`). **Nenhuma migration de taxonomia necessária.**
4. **Faltam intents de cartão.** Não há `create_card` (cadastrar cartão) nem `count_cards`
   ("quantos cartões eu tenho"); só `list_cards`.
5. **OpenRouter sem tool-calling.** `chatRequest` só tem `response_format` (`client.go:76-81`); não há
   `tools`/`tool_choice`. Migrar para function-calling exige estender o client.
6. **Sem store de sessão do agente.** Nenhuma tabela/repo de sessão ou histórico existe em `internal/agent`.

## Decisões aprovadas pelo usuário

- **Estado:** slot-filling + memória curta (sessão por usuário: ação pendente + últimos N turnos).
- **Arquitetura de intent:** migrar para **tool/function-calling nativo** do OpenRouter.
- **Orçamento:** `configure_budget` estruturado → `budgets.CreateBudget` direto, sobre as **5 categorias
  existentes** (sem "Conforto").
- **Agente = orquestrador fino (inegociável):** `internal/agent` SEMPRE invoca os usecases reais dos
  demais módulos (`card`, `budgets`, `transactions`, `categories`, `onboarding`) — que contêm a regra
  de negócio completa da API. O agente nunca reimplementa regra, nunca acessa banco direto, nunca
  ramifica domínio (R-ADAPTER-001.2). Cada tool mapeia 1:1 para um usecase existente.

## Mapeamento de tools → usecases (MVP) — assinaturas verificadas

Regras transversais a TODAS as tools (production-proof, sem falso positivo):
- **Identidade:** injetar `auth.WithPrincipal(ctx, Principal{UserID, Source: SourceWhatsApp})` antes de
  chamar o usecase (substitui o middleware HTTP) **e** preencher `UserID` no DTO quando o DTO o exigir.
  Verificado: usecases de transactions leem `auth.FromContext(ctx)`; card/budgets recebem `UserID` no input.
- **Reuso de DTOs:** usar os DTOs reais dos módulos (sem DTOs paralelos).
- **Validação:** mora no domínio/smart constructor; a tool só molda argumentos e traduz erro sentinel → UX.
- **Transação:** 1 tool = 1 usecase = 1 UoW; sem escrita atômica multi-módulo no agente.
- **Economia:** 1 chamada de LLM por turno; resolução de categoria é DB (`SearchDictionary`), token zero;
  `competence` derivada inline (`time.Now().UTC().Format("2006-01")`), nunca pedida ao usuário sem necessidade.

> **Nomenclatura (decisão 2026-06-18):** uma única tool `record_transaction` com `direction`
> (`income`/`outcome`) representa entrada e saída — substitui `log_expense`/`log_income`. Mais econômico
> (um schema só) e fiel ao domínio (`transactions` usa `direction`). Mapeia para o `LogTransactionFromAgent`
> existente, que já trata ambas as direções.

### Núcleo MVP (must-have — cobre os objetivos do usuário)

| Tool | Usecase-alvo (arquivo) | Args do LLM → DTO | Identidade | Erros sentinel → UX |
|---|---|---|---|---|
| `create_card` | `card.CreateCard` (`card/application/usecases/create_card.go`) → `input.CreateCard{UserID,Name,Nickname,ClosingDay,DueDay,LimitCents}` | `name?`, `nickname`, `closing_day`, `due_day`, `limit_cents?` | `UserID`=principal no DTO | `ErrInvalidClosingDay/DueDay`→"dia inválido (1-31)"; `ErrNicknameConflict`→"já tens um cartão com esse apelido"; `ErrCardLimitTooLarge`→"limite acima do máximo" |
| `count_cards` | **decisão**: `card.CountCards` (novo, repo `COUNT`) — NÃO usar `ListCards(limit=10)` (subconta >10) | — | `UserID`=principal | — |
| `configure_budget` | `budgets.CreateBudget` + `budgets.ActivateBudget` (`create_budget.go`,`activate_budget.go`) → `CreateBudgetInput{UserID,Competence,TotalCents,Allocations[{RootSlug,BasisPoints}]}` depois `ActivateBudgetInput{UserID,Competence}` | `total_cents`, `allocations[{root_slug, basis_points}]` (LLM mapeia nome falado→root_slug; `basis_points=percent*100`) | `UserID`=principal; `Competence`=mês atual derivado | `ErrBudgetAllocationSumExceeds10000`/`…MustBe10000`→"os percentuais precisam somar 100%"; `ErrBudgetConflict`→"já há orçamento neste mês; quer substituir?" |
| `record_transaction` (unifica entrada/saída) | `LogTransactionFromAgent` (resolve hint→UUID via `categories.SearchDictionary`, depois `transactions.CreateTransaction` via `binding/transaction_log.go`) | `direction` (`income`\|`outcome`), `amount_cents`, `merchant`/`category_hint`, `payment_method?` | command `UserID`=principal **+** principal no ctx (binding) | `ErrLogTransactionCategoryNotFound`/`…Ambiguous`→listar e **perguntar**, nunca default; income usa hint default `salário` |
| `monthly_summary` | `budgets.GetMonthlySummary` (`Execute(ctx, userID string, competence string)`) | — | `userID`=principal; `competence`=mês atual | `ErrBudgetNotFound`→"ainda não tens orçamento neste mês; quer configurar?" |

### Allowlist canônica de root_slug (5 categorias — para o LLM mapear)

`custos fixos`→`expense.custo_fixo` · `metas`→`expense.metas` · `prazeres`→`expense.prazeres` ·
`liberdade financeira`→`expense.liberdade_financeira` · `conhecimento`→`expense.conhecimento`.
(Convenção: nome do produto em PT-BR → root slug com prefixo `expense.` e underscore.)

### Fluxo `configure_budget` (slot-filling, multi-turno econômico)

1. Usuário: "orçamento de 10 mil" → tool registra `total_cents=1000000` no `pending_action`; agente pergunta os percentuais.
2. Usuário envia percentuais → LLM emite `allocations` (soma deve dar 10000 bp). Se faltar categoria, `pending_action` guarda o parcial e o agente pergunta só o que falta (não reprocessa).
3. Completo (soma=10000): **confirmar** → `CreateBudget` (draft) → `ActivateBudget` (valida soma=10000, distribui `planned_cents`). Persistência real em budgets.
4. Idempotência: reentrega não recria; conflito de competência → oferecer substituir (`DeleteDraftBudget` + recriar).

### Diferido (pós-MVP, mesmo padrão tool→usecase)

`log_card_purchase` (`transactions.CreateCardPurchase`; exige resolver nome do cartão→`card_id` + categoria),
`list_cards`/`query_card` (`card.ListCards`/`GetCard`), `query_category` (`categories.SearchDictionary`/`ListCategories`),
recorrências e edit/delete last. Listados para não dar falsa impressão de cobertura total no MVP.

## Referências oficiais (OpenRouter) — fundamentam o design (consultadas 2026-06-18)

Tool-calling (`https://openrouter.ai/docs/guides/features/tool-calling`, `.../api/reference/parameters`):
- Segue o shape da OpenAI; convertido para provedores não-OpenAI automaticamente.
- `tools` deve ser enviado em TODA request (o router valida o schema a cada chamada) → contam como
  input tokens sempre; daí a importância do caching do prefixo.
- `tool_choice`: `"none"` | `"auto"` | `"required"` | `{"type":"function","function":{"name":"…"}}`.
- `parallel_tool_calls` (default `true`). **Decisão MVP: `false`** — 1 ação por turno, determinístico,
  mais barato e mais fácil de confirmar/idempotente.
- Com `tools`/`tool_choice`, o OpenRouter só roteia para provedores que suportam tool use.

Prompt caching (`https://openrouter.ai/docs/guides/best-practices/prompt-caching`):
- **Gemini 2.5 Flash/Pro: caching implícito automático** — sem `cache_control` manual.
- Limiar de elegibilidade: **≥1024 tokens** (Gemini 2.5 Flash) / 4096 (Pro). Confirmar Flash **Lite**
  especificamente (família Flash suporta; não assumir — validar antes de prometer o desconto).
- Tokens em cache custam **0,25x** do input; contexto repetido fica ~60–80% mais barato.
- **Regra Gemini (crítica):** há um único `systemInstruction` tratado como imutável para cache. Logo,
  o prefixo estático (persona + catálogo de tools) deve ficar no system; **NÃO** colocar contexto
  dinâmico ali.

### Regra de arquitetura de prompt (economia inegociável)

- **System prompt = estático e cacheável**: persona + catálogo de tools + regras. Nunca muda por
  usuário/dia. (Hoje o `prompt_builder.go` injeta categorias/cartões/data no system — **mudar**: isso
  invalida o cache implícito do Gemini a cada usuário.)
- **Contexto dinâmico nas mensagens de turno** (não no system): seed de categorias/cartões, data
  atual, `pending_action` resumido, últimos 2–4 turnos. Assim o prefixo permanece idêntico e elegível
  a cache; só o sufixo curto varia.
- Resultado: 1 chamada de LLM por turno, prefixo cacheado a 0,25x, sufixo mínimo.

## Plano de ação

Ordem por dependência; cada fase é entregável verificável. Paralelizar via subagents por área
(client/tools, sessão, system prompt) conforme política do projeto.

> **Fase 0 (taxonomia "Conforto") — ELIMINADA** por decisão do usuário (2026-06-18): manter as 5
> categorias canônicas. Nenhuma migration de taxonomia, nenhum ALTER de constraint, nenhum enum novo.

### Fase 1 — OpenRouter: suporte a tool-calling (blueprint verificado)
Arquivos: `internal/agent/infrastructure/providers/openrouter/client.go` (structs verbatim atuais:
`chatRequest:76`, `chatMessage:71`, `chatResponse`, `Interpret:136`) e
`internal/agent/application/interfaces/llm_provider.go` (`LLMRequest`/`LLMResponse`).

1. **Request** — adicionar ao `chatRequest`: `Tools []toolDefinition json:"tools,omitempty"` e
   `ToolChoice any json:"tool_choice,omitempty"` + `ParallelToolCalls *bool json:"parallel_tool_calls,omitempty"` (cravar `false` no MVP).
   Novos structs: `toolDefinition{Type:"function", Function:toolFunction{Name,Description,Parameters map[string]any}}`.
2. **Response** — estender `chatMessage` com `ToolCalls []toolCall json:"tool_calls,omitempty"`;
   `toolCall{ID, Type, Function functionCall{Name, Arguments string}}` (arguments é **string JSON**, não objeto).
   Estender `LLMRequest` com `Tools []*ToolSpec`, `ToolChoice string`; `LLMResponse` com `ToolCalls []ToolCall{ID, FunctionName, ArgumentsJSON map[string]any}`.
3. **Interpret** — se `len(req.Tools)>0`: setar `Tools`+`ToolChoice`+`ParallelToolCalls(false)` e
   `ResponseFmt=nil` (**mutuamente exclusivos** no OpenRouter); senão manter `response_format` (fallback).
   Detectar: `tool_calls` não-vazio → ação; `content` sem tool_calls → resposta conversacional (texto puro).
   Parse defensivo do `arguments` (string→map); tool_call malformado = erro tratado, sem panic.
4. **Resiliência/observabilidade preservadas** (não mudam assinatura): `FallbackChain.Interpret`
   (`fallback_chain.go:55`), circuit breaker, métricas `agent_llm_provider_*` (call_total, errors_total
   por `reason`, latency_seconds). Adicionar counter `agent_llm_provider_tool_calls_total{model,function}`.
5. **Caching**: enviar system+tools como prefixo estável (ver "Regra de arquitetura de prompt"); Gemini
   faz caching implícito (≥1024 tokens) — nenhuma mudança de header necessária.
- Testes (httptest, espelhando `openrouter/client_test.go`): tool_call válido; sem tool_call (texto);
  arguments inválidos; `tools` ignora `response_format`; erro upstream/empty choices.

### Fase 2 — Orquestrador de tools + catálogo (porta application)
Núcleo MVP (tabela acima): `create_card`, `count_cards`, `configure_budget`, `record_transaction`,
`monthly_summary`. Reusar adapters de `infrastructure/dispatcher/` e o resolver de categoria de
`log_transaction_from_agent.go`.

1. **`ToolOrchestrator`** (novo em `application/services/`): `Handle(ctx, principal, channel, toolCall)`
   → resolve handler por nome no catálogo → injeta Principal (`auth.WithPrincipal`) → chama o usecase-adapter
   → retorna `RouteResult{Reply, Outcome, Kind}` (struct atual `intent_router.go:95`).
2. **Integração no `route`** (`intent_router.go:386`): manter `RouteWhatsApp/RouteTelegram` (gateway send
   `:324`, `publishEvent` defer `:320/:337`, `record` `:842`), o early-return de texto vazio e o check de
   onboarding; **substituir o switch de 16 kinds** (`:422-461`) por `r.orchestrator.Handle(...)`.
   Preservar todos os call-sites de `record(ctx,kind,channel,outcome)` e os 7 `Outcome*`.
3. **`count_cards` exige `card.CountCards` novo** (verificado: `ListCards` é paginado `Limit`/`Cursor`;
   limite fixo subconta) — usecase fino + `COUNT` no repositório, seguindo `go-implementation`/R-ADAPTER-001.
4. Validação de argumentos via smart constructors (DMMF); categoria ambígua/sem match → **perguntar**,
   nunca default. `parallel_tool_calls=false` ⇒ 1 tool por turno.
- Testes: roteamento por tool, argumentos inválidos, categoria ambígua, preservação de métrica/evento.

### Fase 3 — Sessão / slot-filling + memória curta (persistida — blueprint verificado)
Padrão `internal/platform/database`: `DBTX`, `uow.UnitOfWork`, helper `uow.Do[T]`, `RepositoryFactory`.

1. **Migration `000010_create_agent_sessions.up/down.sql`** (próximo número após `000009`) — tabela
   `mecontrola.agent_sessions`: `id UUID PK`, `user_id UUID FK users(id) ON DELETE CASCADE`,
   `channel TEXT`, `pending_action JSONB DEFAULT '{}'`, `recent_turns JSONB DEFAULT '[]'`,
   `updated_at/created_at TIMESTAMPTZ`, `expires_at TIMESTAMPTZ`. Índice único parcial
   `(user_id, channel) WHERE expires_at > now()`; índice em `expires_at`; CHECKs de tamanho do JSONB.
   Espelhar `000007_create_platform_idempotency_keys` (expiry passivo) e convenções de `000008`.
2. **Interface** `application/interfaces/agent_session_repository.go`: `AgentSessionRecord` +
   `Create/GetByUserAndChannel/Update/Delete/DeleteExpired`; sentinelas `ErrAgentSessionNotFound/Conflict`.
   **Repo** em `infrastructure/repositories/postgres/agent_session_repository.go` (DBTX, marshal JSONB,
   spans `o11y.Tracer`, wrapping `%w`); **factory** `infrastructure/repositories/factory.go`.
3. **Wiring** em `module.go`: `uow.NewUnitOfWork(db)` + factory; injetar no orquestrador. Leitura por DI;
   escrita via `uow.Do`. Sem estado global (R6.6).
4. **Fluxo**: carregar sessão no início do turno → anexar `pending_action` resumido + 2–4 turnos **na
   mensagem de turno** (não no system) → executar → persistir turno/estado. `time.Now().UTC()` inline.
5. **TTL**: expiry passivo no `WHERE expires_at > now()`; job opcional `DeleteExpired` (cleanup horário).
6. **Orçamento multi-turno**: acumular total + alocações até soma=10000 bp → `CreateBudget`+`ActivateBudget`.
- Validação: teste de repositório (suite + DB) com isolamento por usuário; e2e do fluxo orçamento 2 turnos.

### Fase 4 — System prompt robusto e confiável (cache-aware)
**System prompt = prefixo estático e cacheável** (`persona.system.tmpl` + catálogo de tools); contexto
dinâmico vai na mensagem de turno (Regra de arquitetura de prompt). Seções, em PT-BR:
- **Papel & tom:** parceiro financeiro no WhatsApp; respostas curtas, sem jargão, sem JSON/código.
- **Catálogo de tools:** quando usar cada tool e seus parâmetros obrigatórios (enxuto p/ caber no cache).
- **Slot-filling:** faltou parâmetro obrigatório → **perguntar**, nunca inventar; **confirmar** antes de
  escrita sensível (orçamento/cartão); jamais inventar valor monetário.
- **Categoria:** usar o hint; se ambíguo, listar e perguntar.
- **Orçamento:** soma = 100% nas 5 categorias canônicas; se vier parcial, perguntar o resto; mapear
  nome falado → root slug (`expense.custo_fixo` etc.).
- **Segurança:** só escopo do usuário autenticado; recusar fora do domínio financeiro; idempotência por `event_id`.
- **Few-shots:** cadastrar cartão; contar cartões; orçamento (1 e 2 turnos); `record_transaction`
  salário/supermercado/iFood; categoria ambígua.
- Mudar `prompt_builder.go`/`prompt_context_loader.go` para mover seed/data do system → turno.
- Validação: testes de render do prompt (prefixo estável) + e2e dos cenários.

### Limpeza
- Após a migração para tools, remover o pipeline morto `HandleInboundMessage` + `IntentDispatcher`,
  os `routeXxx` do switch e adapters/portas exclusivos — evitando duas arquiteturas paralelas.
  Preservar `RouteWhatsApp/RouteTelegram`, `route` (até o parse), `record`, `publishEvent` e formatadores.

## Arquivos-chave

- `internal/agent/infrastructure/providers/openrouter/client.go` — tools/tool_choice + parse.
- `internal/agent/application/services/intent_router.go` — substituir switch por orquestrador de tools.
- `internal/agent/application/usecases/log_transaction_from_agent.go` — reusar resolver de categoria.
- `internal/agent/infrastructure/dispatcher/*_adapter.go` — reusar como execução das tools.
- `internal/agent/application/prompting/persona.system.tmpl` + prompt de tool-use — system prompt.
- `internal/agent/infrastructure/loader/prompt_context_loader.go` — anexar histórico/estado.
- Novos: repo/tabela `agent_sessions` (padrão `internal/platform/database`); novo orquestrador de tools.
- `internal/budgets/application/usecases/create_budget.go` — destino do orçamento estruturado (sem mudança de assinatura).
- Usecases-alvo das tools (chamados pelo agente, sem alteração de regra): `card.CreateCard`/`ListCards`,
  `budgets.CreateBudget`/`ActivateBudget`/`GetMonthlySummary`, `transactions.CreateTransaction`/`CreateCardPurchase`.

## Testes, mocks e gates (blueprint verificado)

- **Mocks** (`.mockery.yml`, template `testify`, saída `{InterfaceDir}/mocks`): registrar
  `AgentSessionRepository` e as interfaces de tool sob o package `agent/application/interfaces`; gerar com
  `task mocks:mocks`; CI valida com `task mocks:verify` (git diff).
- **Unit (testify/suite + table-driven)**: espelhar `intent_router_test.go` (suite + `SetupTest` + fakes
  + `newRouter()`). LLM faked via `httptest.Server` como em `openrouter/client_test.go` (valida headers,
  body e injeta resposta canned).
- **E2E (godog)**: features em `internal/agent/e2e/features/`; novas — `f05_configure_budget_multi_turn.feature`,
  `f06_create_card.feature`, `f07_record_transaction.feature`. Persistência **asserida via banco**
  (contagem antes/depois) e outbox (`agent.intent.executed.v1`, `transactions.transaction.created.v1`),
  reply capturada por `CapturingGateway`. Idempotência por WAMID já coberta no padrão `f01`.
- **Arquivos de teste por fase**: F1 `client_test.go` (casos tool); F2 `tool_orchestrator_test.go`;
  F3 `agent_session_repository_test.go` (suite + DB, isolamento por usuário); F4 render do prompt + features e2e.

## Verificação end-to-end (comandos verbatim)

1. `task mocks:mocks` && `task mocks:verify` (mocks atualizados).
2. `task lint:run` (inclui gates de auth-bypass/outbox/user-isolation) + `task test:unit`
   (`go test -race -covermode=atomic -short ./...`).
3. **Gates de governança** (devem retornar vazio): comentários proibidos e SQL em adapter
   (`.claude/rules/go-adapters.md`); domínio fora de `Decide*` (`.claude/rules/transactions-workflows.md`).
4. **Checklist R0–R7** (`references/build.md`): greps R0 (`^func init()`), R1, R5
   (`os.Exit`/`log.Fatal`/`panic` fora de main), R7 (`interface{}`) + `go build ./...`, `go vet ./...`,
   `go test ./... -count=1 -race`.
5. `task test:integration` (repos) e `task test:e2e` (`-tags=e2e -timeout=15m ./internal/agent/e2e/...`):
   cadastrar cartão; "quantos cartões tenho" (count correto >10); orçamento nas **5 categorias** em 2
   turnos (soma 10000 bp, persistido via `budgets.CreateBudget`+`ActivateBudget`); `record_transaction`
   salário (income), supermercado e iFood (outcome) com categoria correta; categoria ambígua → pergunta.
   Confirmar persistência real consultando os repositórios dos módulos.
6. `task security:vulncheck` (`govulncheck ./...`).
7. Smoke manual via webhook WhatsApp (sandbox Meta) confirmando recv→LLM(tool-call)→ação→reply.

## Log de execução

- **2026-06-18 — decisões e descobertas (sem código):**
  - Verificado que o modelo de 5 categorias está cravado em 3 camadas (CHECK constraints no banco,
    enum `RootSlug` em budgets, `CategoryKind`/`SuggestDefaultSplit`/`mapCategoryKindToRootSlug` em
    onboarding).
  - Usuário decidiu **manter só as 5 categorias** (sem "Conforto") → **Fase 0 eliminada**; nenhuma
    mudança de taxonomia/constraint/enum.
  - Reforçado mandato: **agente sempre chama os usecases reais** dos módulos (regra de negócio fica
    nos módulos); persistência via `internal/platform/database` (uow + factory) para `agent_sessions`.

- **2026-06-18 — mapeamento de tools (sem código):** verificadas as assinaturas reais —
  `card.CreateCard`/`ListCards` (`UserID` no DTO), `budgets.CreateBudget`/`ActivateBudget`
  (`CreateBudgetInput{UserID,Competence,TotalCents,Allocations[{RootSlug,BasisPoints}]}`,
  `ActivateBudgetInput{UserID,Competence}`), `budgets.GetMonthlySummary(ctx,userID,competence)`,
  `transactions.RawCreateTransaction{Direction,PaymentMethod,AmountCents,Description,CategoryID,SubcategoryID,OccurredAt}`
  e `binding/transaction_log.go` (injeta principal + parse UUID), `categories.SearchDictionaryInput{Query,Kind}`.
  Decisão registrada: **`count_cards` precisa de `card.CountCards` novo** (ListCards é paginado);
  não usar limite fixo como contagem (evita falso positivo).

- **2026-06-18 — enriquecimento (sem código):**
  - Tool de transação unificada: `log_expense`/`log_income` → **`record_transaction`** com `direction`.
  - Documentação oficial OpenRouter consultada (tool-calling e prompt caching). Decisões: `parallel_tool_calls=false`;
    `tools`/`response_format` mutuamente exclusivos; **Gemini = caching implícito** (≥1024 tokens, 0,25x) →
    regra de prompt cache-aware (system estático; contexto dinâmico no turno). Pendência honesta:
    confirmar elegibilidade de caching do Flash **Lite** especificamente.
  - Blueprints verificados incorporados nas Fases 1–4: structs do client (`chatRequest`/`chatMessage`/
    `tool_calls`), `ToolOrchestrator` substituindo o switch (preservando `record`/`publishEvent`/gateway),
    schema `agent_sessions` + repo/factory/migration `000010`, e estratégia de testes/gates/Taskfile.

- **2026-06-18 — execução iniciada (branch `feat/agent-tool-calling`):**
  - **Fase 1 (tool-calling no client) — CONCLUÍDA.** Alterados:
    `internal/agent/application/interfaces/llm_provider.go` (+`ToolSpec`, `ToolCall`, campos `Tools`/`ToolChoice`
    em `LLMRequest`, `ToolCalls` em `LLMResponse`); `internal/agent/infrastructure/providers/openrouter/client.go`
    (+structs `toolDefinition`/`toolCall`, `chatRequest.Tools/ToolChoice/ParallelToolCalls`, `chatMessage.ToolCalls`,
    detecção de tool_calls + helpers `buildToolDefinitions`/`resolveToolChoice`/`parseToolCalls`, counter
    `agent_llm_provider_tool_calls_total`, `parallel_tool_calls=false`, tools XOR response_format).
    +4 testes httptest em `client_test.go`. Validação: `go build ./...` OK, testes openrouter OK, zero
    comentários OK, sem `interface{}` OK, `go vet` OK.
  - **Pré-requisito `card.CountCards` — CONCLUÍDO.** `CardRepository.CountActiveByUser` (interface + pg
    `SELECT count(*) … deleted_at IS NULL`), DTOs `input.CountCards`/`output.CardCount`, usecase `CountCards`,
    mocks regenerados, +3 testes suite. Validação: build geral OK, testes card OK, zero comentários OK.
  - Pendente: Fases 2 (orquestrador de tools), 3 (agent_sessions), 4 (system prompt cache-aware), limpeza, e2e.
- **2026-06-18 — commits na main (decisão do usuário: trabalhar na main, separando em commits):**
  - `5492af0` — Fase 1 (tool-calling no client) + `card.CountCards`. Publicado em `origin/main`.
  - `46287ee` — **Fase 2a**: catálogo de tools (`record_transaction`, `monthly_summary`, `list_cards`,
    `configure_budget`) + `ToolCallToIntent` (mapeamento puro reusando `build()`/construtores de domínio).
    +6 testes suite. Build/lint(v2)/zero-comentários OK. Publicado em `origin/main`.
  - Ambiente: instalado `golangci-lint v2.12.2` (config do repo é v2; binário local era v1) para o
    pre-commit hook passar — alinhado à CI.
  - **Fase 2b (próxima) — decisão de sequenciamento:** ligar `ParseInbound` às tools (`Interpret` com
    `AgentToolCatalog()` → `ToolCallToIntent`) está entrelaçado com o system prompt (Fase 4): o prompt
    atual `parse_intent.system.tmpl` instrui saída JSON, incompatível com tool-calling. Plano: (1) ajustar
    o system prompt para persona + tools (cache-aware, contexto dinâmico no turno); (2) `ParseInbound`
    chama com tools e, sem tool_call, usar o texto do LLM como resposta direta (evita 2ª chamada de
    fallback — economia); (3) novas tools `create_card`/`count_cards` exigem novos `intent.Kind` +
    handlers no router (`CardCreator`→`card.CreateCard`, `CardCounter`→`card.CountCards`). Depois Fase 3
    (`agent_sessions` multi-turno) e limpeza do código morto.

- **2026-06-18 — Fase 2b (parcial) — `create_card` + `count_cards` CONCLUÍDOS (commit `ff29c40`, na main):**
  - Domínio `intent`: `KindCreateCard`/`KindCountCards` + campos (`cardNickname`, `closingDay`, `dueDay`,
    `limitCents`) + construtores; tool catalog + `ToolCallToIntent`; router deps `CardCreator`/`CardCounter`
    + handlers + tradução de erro sentinela do `card/domain`; adapters finos em `binding/cards.go`
    (→ `card.CreateCard`/`card.CountCards`); `CardModule.CountCardsUC` exposto; wiring no agente.
  - **Alcançabilidade**: adicionei os 2 kinds ao enum/propriedades do `ParseIntentJSONSchema` e ao
    `parse_intent.system.tmpl` (corrigindo gap: estavam wired mas inacessíveis no parse vivo).
  - Validação: build, `go test ./internal/agent/... ./internal/card/...` OK, golangci-lint v2 0 issues,
    zero comentários OK. Feito via subagent task-executor + verificação independente minha.
  - Pendente Fase 2b: ativar tool-calling no `ParseInbound` (hoje ainda JSON-schema) — opcional, pois os
    kinds já funcionam; e resposta direta de texto quando não há tool_call.
  - Próximo: Fase 3 (`agent_sessions` + multi-turno) → orçamento com percentuais customizados; limpeza; e2e.

_(próximas entradas: por fase — arquivos alterados, validações executadas, riscos residuais, suposições)_
