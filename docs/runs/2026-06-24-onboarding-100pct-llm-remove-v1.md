# Onboarding 100% LLM — Remoção total do V1, saudação única, fim do loop "Não entendi o valor"

- Data: 2026-06-24
- Branch: `main` (a pedido do usuário, sem branch separada)
- Skill obrigatória: `.agents/skills/go-implementation/SKILL.md` (Go 1.26) — Etapas 1–5 executadas.

## Contexto / Bug

Conversa real no WhatsApp pós-`ATIVAR`: dois balões (intro 🤖 + "Olá! ... qual o seu objetivo financeiro principal?"), e ao responder o objetivo ("Comprar uma casa nova") o bot entrava em loop **"Nao entendi o valor. Envie por exemplo: R$ 3500 ou 3500."**.

Investigação (código + servidor `187.77.45.48` + Postgres + `.env`) revelou **três mecanismos de onboarding sobrepostos**:
1. V1 determinístico (`onboarding_workflow.go` → `decideIncome`/`parseMonetary`) — emitia "Nao entendi o valor".
2. V2 LLM (`run_onboarding_turn.go`, fases + tools) — fluxo correto, parcialmente ativo.
3. Saudação duplicada (fallback LLM do daily agent sobre o sinal `__onboarding_welcome__`) + `onboarding_intro` determinístico.

**Causa raiz**: duas fontes de verdade de estado (`onboarding_sessions.state` V1 + `payload.phase` V2) e fallback determinístico; corrida entre `StartBudgetConfiguration` e o `emitWelcome` clobberava a fase.

## Decisões do usuário
- Remover V1 + saudação duplicada; manter V2 (LLM interpreta cada resposta via tools).
- Boas-vindas: **uma única mensagem gerada por LLM** (haiku-4.5).
- Tudo na `main`.

## Mudanças

### Fonte de verdade única (payload)
- `valueobjects/onboarding_state.go`: enum substituído por 2 constantes string (`in_progress`, `active`).
- `entities/onboarding_session.go`: removido campo `state`/`State()`/`With(...)`; `IsActive() = payload.CompletedAt != nil`; construtores sem `state`.
- `repositories/postgres/onboarding_session_repository.go`: `Find` sem `ParseOnboardingState`; `Upsert` deriva coluna `state` (`in_progress`/`active`). **Sem migração** (constraint só exige `length(state)>0`).
- `usecases/get_onboarding_context.go`: expõe `CompletedAt`, removeu `State`.
- `agent/infrastructure/onboarding/onboarding_state_reader.go`: `InProgress = CompletedAt == nil`.

### Remoção do V1
- Deletados: `domain/services/onboarding_workflow.go`, `application/usecases/process_onboarding_message.go`, `agent/infrastructure/onboarding/onboarding_continuation.go` (+ testes V1). Helper `buildOutboxEvent`/`extractEventID` realocado para `onboarding_event_id.go`.
- `whatsapp/telegram_message_processor.go`: removidos `processUseCase` + `ProcessConversation` + envio de `onboarding_intro`.
- `agent/services/onboarding_agent.go`: removido `degradeOnboarding` e o ramo V1; em erro retorna mensagem de retry (não cai no daily).
- `agent/services/intent_router.go`: removidos `OnboardingContinuation`/`OnboardingConversation`/`deps.Onboarding`.
- `agent/module.go`, `cmd/server/server.go`, `cmd/worker/worker.go`: removido o wiring do adapter V1.

### Saudação única por LLM + fim da corrida
- `emitWelcome` agora gera a saudação via interpreter LLM (`onboardingWelcomeSystemPrompt`), com fallback determinístico (`scriptWelcome`) em erro/empty; seta `phase=objective` e marca welcome.
- `StartBudgetConfiguration`: cria a sessão (`in_progress`) idempotente, sem setar fase, sem reply de etapa V1 — `emitWelcome` é o único dono da fase.

## Validação
- `go build ./...` ✅ · `go vet ./...` (+ tags `e2e integration`) ✅ · `gofmt -l` limpo · `golangci-lint run` → **0 issues**.
- Testes unitários: todos os módulos `ok`.
- Gates: zero-comentários (R-ADAPTER-001.1) ✅; switch de `daily_ledger_agent.go` não cresceu (cases=0) ✅; sem SQL em tools/workflow ✅.
- Grep de regressão (prod): sem `decideIncome`/`parseMonetary`/`ProcessOnboardingMessage`/`OnboardingContinuation`/`OnboardingStateAwaiting`/`degradeOnboarding`.
- **RUN_REAL_LLM (claude-haiku-4.5)** — 5/5 PASS: `ObjectiveToolSelected` (objetivo em texto livre → `save_onboarding_objective`, **sem loop**), `IncomeToolSelected`, `CardToolSelected`, `BudgetSplitsToolSelected`, `QuestionStaysText` (pergunta off-topic permanece texto).

## Pendente (requer deploy)
- Verificação pós-deploy no Postgres do servidor (`phase` evolui; `state ∈ {in_progress,active}`; uma saudação em `recent_turns`).
- Teste real no WhatsApp repetindo o fluxo da imagem.
- `onboarding_intro` permanece como entrada de catálogo morta em `infrastructure/config/runtime.go` (não enviada) — limpeza opcional.
