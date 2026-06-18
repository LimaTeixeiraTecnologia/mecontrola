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

## Plano de ação

Ordem por dependência; cada fase é entregável verificável. Paralelizar via subagents por área
(client/tools, sessão, system prompt) conforme política do projeto.

> **Fase 0 (taxonomia "Conforto") — ELIMINADA** por decisão do usuário (2026-06-18): manter as 5
> categorias canônicas. Nenhuma migration de taxonomia, nenhum ALTER de constraint, nenhum enum novo.

### Fase 1 — OpenRouter: suporte a tool-calling
- Estender `chatRequest` com `Tools []tool` e `ToolChoice` (`client.go`), mantendo `response_format`
  como fallback. Parse de `choices[0].message.tool_calls` (nome + arguments JSON).
- Tratar resposta sem tool-call (texto puro = resposta conversacional) e tool-calls malformados.
- Manter circuit breaker, fallback chain e observabilidade já existentes.
- Validação: testes unitários com httptest cobrindo tool_call válido, ausência de tool_call,
  arguments inválidos, erro upstream.

### Fase 2 — Catálogo de tools (porta application)
- Definir o catálogo de tools mapeando 1:1 para usecases já existentes:
  `create_card`, `list_cards`, `count_cards`, `create_budget` (total + allocations RootSlug/BasisPoints,
  soma=10000), `log_expense`, `log_income`, `log_card_purchase`, `monthly_summary`, `query_card`,
  `query_category`. Reusar adapters de `infrastructure/dispatcher/` e o resolver de categoria de
  `log_transaction_from_agent.go` (não reimplementar resolução de categoria).
- Novo orquestrador (substitui o switch de 16 kinds no caminho vivo): recebe tool-call → valida
  argumentos via smart constructors → injeta Principal → chama usecase → formata reply.
- Para transações, manter `category_hint` → `SearchDictionary` → UUID; em ambíguo/sem match,
  **perguntar ao usuário** (não default silencioso).
- Validação: testes de roteamento por tool, incluindo argumentos inválidos e categoria ambígua.

### Fase 3 — Sessão / slot-filling + memória curta
- Nova tabela `agent_sessions` (migration) e repo seguindo o padrão `internal/platform/database`
  (leitura por repo via DI; escrita via uow+factory): chave por `user_id`+canal, guarda
  `pending_action` (ex.: budget em construção com slots já preenchidos) + últimos N turnos (texto +
  papel) com TTL.
- Carregar sessão no início do handle; anexar histórico curto e estado pendente ao input do LLM;
  persistir turno ao final. Sem abstrair tempo (usar `time.Now().UTC()` inline).
- Multi-turno de orçamento: acumular total e alocações entre mensagens até completar 100% e então
  chamar `create_budget`.
- Validação: teste e2e (estilo `e2e/features/`) do fluxo "10 mil reais" → percentuais em mensagem
  seguinte → orçamento persistido com 6 alocações somando 10000 bp.

### Fase 4 — System prompt robusto e confiável
Reescrever `application/prompting/persona.system.tmpl` + prompt de tool-use, em PT-BR, com seções:
- **Papel & tom:** parceiro financeiro no WhatsApp; respostas curtas, sem jargão, sem JSON/código.
- **Catálogo de tools:** descrição de cada tool, parâmetros obrigatórios e quando usá-la.
- **Regras de slot-filling:** se faltar parâmetro obrigatório, **perguntar** em vez de inventar;
  confirmar antes de escrita sensível (criar orçamento/cartão); nunca inventar valores monetários.
- **Resolução de categoria:** usar o hint; se ambíguo, listar opções e perguntar.
- **Orçamento:** exigir soma = 100%; se o usuário der só alguns percentuais, perguntar o restante;
  mapear nomes falados → root slugs canônicos (incl. Conforto).
- **Segurança/limites:** ações só no escopo do usuário autenticado; recusar fora de domínio
  financeiro; idempotência por `event_id`.
- **Few-shots** cobrindo: cadastrar cartão, contar cartões, configurar orçamento (1 e 2 turnos),
  lançar salário/supermercado/iFood, categoria ambígua.
- Validação: testes de render do prompt + e2e dos cenários acima.

### Limpeza
- Remover o pipeline morto `HandleInboundMessage` + `IntentDispatcher` (e adapters/portas exclusivos)
  após a migração para tools, evitando duas arquiteturas paralelas.

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

## Verificação end-to-end

1. `task test` / testes unitários novos (client tools, orquestrador, sessão, prompt render).
2. Gates obrigatórios: `.claude/rules/go-adapters.md` (zero comentários; sem SQL/regra em adapter) e
   `.claude/rules/transactions-workflows.md`.
3. Checklist R0–R7 (`references/build.md`): greps de R0/R1/R5/R7 + `go build ./...`, `go vet ./...`,
   `go test ./... -count=1 -race`, `mockery --config mockery.yml --dry-run`.
4. E2E (`internal/agent/e2e/features/`): cadastrar cartão; "quantos cartões tenho"; configurar
   orçamento nas **5 categorias** em 2 turnos (soma 10000 bp, persistido via `budgets.CreateBudget`);
   lançar salário (income), supermercado e iFood (outcome) com categoria correta; categoria ambígua →
   agente pergunta. Confirmar persistência real consultando os repositórios dos módulos.
5. Smoke manual via webhook WhatsApp (sandbox Meta) confirmando recv→LLM(tool-call)→ação→reply.

## Log de execução

- **2026-06-18 — decisões e descobertas (sem código):**
  - Verificado que o modelo de 5 categorias está cravado em 3 camadas (CHECK constraints no banco,
    enum `RootSlug` em budgets, `CategoryKind`/`SuggestDefaultSplit`/`mapCategoryKindToRootSlug` em
    onboarding).
  - Usuário decidiu **manter só as 5 categorias** (sem "Conforto") → **Fase 0 eliminada**; nenhuma
    mudança de taxonomia/constraint/enum.
  - Reforçado mandato: **agente sempre chama os usecases reais** dos módulos (regra de negócio fica
    nos módulos); persistência via `internal/platform/database` (uow + factory) para `agent_sessions`.

_(próximas entradas: por fase — arquivos alterados, validações executadas, riscos residuais, suposições)_
