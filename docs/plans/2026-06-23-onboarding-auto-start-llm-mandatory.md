# Plano: Auto-start de Onboarding após ATIVAR com LLM Mandatório

- **Data**: 2026-06-23
- **Módulos afetados**: `internal/onboarding`, `internal/agent`, `configs`
- **Skills obrigatórias**: `go-implementation` (Etapas 1–5), `mastra`
- **Prioridade**: Alta

---

## Problema

Após o usuário enviar `ATIVAR [token]` no WhatsApp:
1. O sistema responde "Sua conta foi ativada!" — e **para**.
2. O onboarding só inicia quando o usuário enviar **outra mensagem** (fricção desnecessária).
3. O LLM de onboarding (`RunOnboardingTurn`) existe e está wired, mas ainda é controlado por feature flag — deve ser **sempre habilitado**.

## Solução

Encadear, sem interação adicional do usuário:

```
ATIVAR → "Conta ativada!" → Apresentação do bot → LLM envia primeira pergunta de onboarding
```

E tornar o LLM **mandatório** removendo o feature flag: o caminho LLM é sempre ativo; o FSM é fallback de degradação apenas.

## Arquitetura Atual (o que já existe — não alterar)

| Componente | Localização | Estado |
|---|---|---|
| LLM onboarding agent | `internal/agent/application/usecases/run_onboarding_turn.go` | ✅ Wired end-to-end |
| OnboardingAgent (Tier 1 LLM + Tier 2 FSM fallback) | `internal/agent/application/services/onboarding_agent.go` | ✅ Operacional |
| Conversation history | `agent_sessions.recent_turns` JSONB | ✅ Já persiste por `RunOnboardingTurn` |
| FSM fallback | `internal/onboarding/domain/services/onboarding_workflow.go` | ✅ Somente fallback |
| Evento `onboarding.subscription_bound` | outbox após BindAndConsume | ✅ Publicado com `user_id` |

**Fluxo pós-ativação hoje**: usuário autenticado → `AgentRoute` → `IntentRouter` → `OnboardingAgent.Handle()` → **LLM** (`RunOnboardingTurn`).

## Por que NÃO usar `startResult.Reply` do `StartBudgetConfiguration`?

`startResult.Reply` é do FSM ("Beleza! Qual a sua renda mensal?"). Se enviado antes, colide com o `emitWelcome()` do LLM — usuário receberia dois cumprimentos. O LLM DEVE ser o único a enviar a primeira pergunta.

---

## Fase 1 — Auto-start: sessão criada + intro do bot

**Escopo**: `internal/onboarding` (5 arquivos). Após ativação bem-sucedida:
1. Envia `"welcome_activated"` → "Sua conta foi ativada! Bem-vindo."
2. Envia `"onboarding_intro"` → apresentação do bot (nova chave de mensagem).
3. Cria sessão de onboarding via `StartBudgetConfiguration` (necessário para `RunOnboardingTurn` ler `InProgress = true`).
4. **NÃO envia** `startResult.Reply` — o LLM envia a primeira pergunta via Fase 2.

### F1-1. `configs/config.go`

Adicionar em `WhatsAppConfig` após `InvalidCountry` (~linha 160):

```go
OnboardingIntro string `mapstructure:"WA_MSG_ONBOARDING_INTRO"`
```

### F1-2. `internal/onboarding/infrastructure/config/runtime.go`

Adicionar no mapa `messages` (~linha 42):

```go
"onboarding_intro": waCfg.OnboardingIntro,
```

### F1-3. `internal/onboarding/application/usecases/consume_magic_token.go`

Adicionar `UserID string` em `ConsumeResult`:

```go
type ConsumeResult struct {
    Outcome ConsumeOutcome
    UserID  string  // preenchido somente quando Outcome == ConsumeOutcomeActivated
}
```

Em `mapResult`, bloco `err == nil` (linha ~207):

```go
return ConsumeResult{
    Outcome: ConsumeOutcomeActivated,
    UserID:  result.magicToken.ConsumedByUserID(),
}, nil
```

> `ConsumedByUserID()` retorna `string` — confirmado em `magic_token.go` linha 111.

### F1-4. `internal/onboarding/application/services/whatsapp_message_processor.go`

**A. Nova interface** (R6.3 — interface no consumidor):

```go
type StartBudgetConfigurationUseCase interface {
    Execute(ctx context.Context, in usecases.StartBudgetConfigurationInput) (usecases.StartBudgetConfigurationResult, error)
}
```

**B. Campo + construtor**:

```go
type WhatsAppMessageProcessor struct {
    // ... campos existentes ...
    startBudgetUseCase StartBudgetConfigurationUseCase
}

func NewWhatsAppMessageProcessor(
    consumeUseCase ConsumeMagicTokenUseCase,
    fallbackUseCase TryFallbackActivationUseCase,
    processUseCase ProcessOnboardingMessageUseCase,
    startBudgetUseCase StartBudgetConfigurationUseCase,  // novo
    waGateway WhatsAppGateway,
    messages map[string]string,
    o11y observability.Observability,
) *WhatsAppMessageProcessor
```

**C. `HandleActivation` — após enviar `replyKey`** (substituir `return nil` final, linha ~106):

```go
replyKey := consumeOutcomeToMessageKey(result.Outcome)
p.sendMessage(ctx, from.String(), p.msg(replyKey))

if result.Outcome == ConsumeOutcomeActivated {
    p.sendMessage(ctx, from.String(), p.msg("onboarding_intro"))
    userID, parseErr := uuid.Parse(result.UserID)
    if parseErr == nil {
        if _, startErr := p.startBudgetUseCase.Execute(ctx, usecases.StartBudgetConfigurationInput{
            UserID:  userID,
            Channel: entities.OnboardingChannelWhatsApp,
        }); startErr != nil {
            slog.WarnContext(ctx, "onboarding.processor.start_budget_failed",
                "from", payload.MaskMobile(fromE164),
                "error", startErr.Error(),
            )
        }
        // Não envia startResult.Reply — LLM (Fase 2) envia a primeira pergunta
    }
}
return nil
```

> Falha em `startBudgetUseCase` é logada como warn — a ativação já foi confirmada ao usuário.

### F1-5. `internal/onboarding/module.go`

Atualizar `newWhatsAppMessageProcessor` para injetar `useCases.startBudgetConfiguration` (já instanciado na linha 289):

```go
return services.NewWhatsAppMessageProcessor(
    useCases.consumeToken,
    useCases.fallbackActivation,
    useCases.processOnboardingMessage,
    useCases.startBudgetConfiguration,  // novo
    deps.whatsAppGateway,
    deps.runtimeCfg.Messages,
    o11y,
)
```

---

## Fase 2 — LLM greeting proativo via event consumer (`internal/agent`)

A sessão criada em Fase 1 publica o evento `onboarding.subscription_bound` via outbox. Um novo consumer em `internal/agent` escuta esse evento e dispara o LLM proativamente para enviar a primeira saudação/pergunta (`emitWelcome()`).

**Bounded context preservado**: `internal/agent` já consome eventos do outbox. Não há dependência direta de `internal/agent` em `internal/onboarding` — comunicação exclusivamente via evento.

### F2-1. Adicionar `peer_e164` ao payload do evento `subscription_bound`

**Arquivo**: `internal/onboarding/application/binding/subscription_binding.go`

O payload atual só tem `user_id`. Adicionar `peer_e164` para o consumer saber para qual número enviar:

```go
payload: map[string]any{
    "user_id":   userResult.UserID,
    "peer_e164": fromE164,  // novo campo
},
```

> `SubscriptionBoundSessionConsumer` existente ignora campos extras (unmarshaling permissivo) — não precisa ser alterado.

### F2-2. `internal/agent/infrastructure/messaging/database/consumers/onboarding_bound_consumer.go` (novo)

Padrão: idêntico aos outros consumers — R-ADAPTER-001.2, R-AGENT-WF-001.6 (Thread-first via AgentRuntime).

```go
package consumers

import (
    "context"
    "encoding/json"
    "fmt"
    "log/slog"

    "github.com/google/uuid"
    "github.com/JailtonJunior94/devkit-go/pkg/observability"

    appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type OnboardingBoundConsumer struct {
    agentRuntime   appinterfaces.AgentRuntimeExecutor
    o11y           observability.Observability
    decodeFailed   observability.Counter
}

func NewOnboardingBoundConsumer(
    agentRuntime appinterfaces.AgentRuntimeExecutor,
    o11y observability.Observability,
) *OnboardingBoundConsumer {
    return &OnboardingBoundConsumer{
        agentRuntime: agentRuntime,
        o11y:         o11y,
        decodeFailed: o11y.Metrics().Counter(
            "agent_onboarding_bound_consumer_decode_failed_total",
            "Total de falhas ao decodificar evento subscription_bound no agente",
            "1",
        ),
    }
}

type onboardingBoundPayload struct {
    UserID   string `json:"user_id"`
    PeerE164 string `json:"peer_e164"`
}

func (c *OnboardingBoundConsumer) Handle(ctx context.Context, event events.Event) error {
    ctx, span := c.o11y.Tracer().Start(ctx, "agent.consumer.onboarding_bound")
    defer span.End()

    var envelope outbox.Envelope
    if err := json.Unmarshal(event.Payload(), &envelope); err != nil {
        c.decodeFailed.Add(ctx, 1)
        return fmt.Errorf("agent.consumer.onboarding_bound: unmarshal envelope: %w", err)
    }

    var p onboardingBoundPayload
    if err := json.Unmarshal(envelope.Payload, &p); err != nil {
        c.decodeFailed.Add(ctx, 1)
        return fmt.Errorf("agent.consumer.onboarding_bound: unmarshal payload: %w", err)
    }

    userID, err := uuid.Parse(p.UserID)
    if err != nil {
        c.decodeFailed.Add(ctx, 1)
        return fmt.Errorf("agent.consumer.onboarding_bound: parse user_id: %w", err)
    }

    if p.PeerE164 == "" {
        slog.WarnContext(ctx, "agent.consumer.onboarding_bound.missing_peer",
            "user_id", userID.String(),
        )
        return nil
    }

    if err := c.agentRuntime.Execute(ctx, appinterfaces.AgentExecuteInput{
        UserID:    userID,
        Channel:   "whatsapp",
        Peer:      p.PeerE164,
        Text:      "__onboarding_welcome__",
    }); err != nil {
        slog.WarnContext(ctx, "agent.consumer.onboarding_bound.execute_failed",
            "user_id", userID.String(),
            "error", err.Error(),
        )
        return fmt.Errorf("agent.consumer.onboarding_bound: execute: %w", err)
    }
    return nil
}
```

> `AgentRuntimeExecutor` é uma interface definida no consumidor (R6.3). `Text: "__onboarding_welcome__"` é o sentinel: `RunOnboardingTurn` detecta `phase == ""` → `emitWelcome()` independente do texto.

### F2-3. `internal/agent/module.go`

Registrar o consumer no bootstrap do módulo agente, escutando `"onboarding.subscription_bound"`:

```go
onboardingBoundConsumer := consumers.NewOnboardingBoundConsumer(agentRuntime, o11y)
// Adicionar ao dispatcher de eventos para "onboarding.subscription_bound"
```

---

## Fase 3 — LLM Mandatório: remover feature flag

O `OnboardingAgent` hoje usa `OnboardingLLMEnabled` para decidir se monta o Tier 1 (LLM). A regra mandatória remove essa condicionalidade.

### F3-1. `internal/agent/module.go`

**Remover** o early-return condicional em `buildOnboardingRunner()`:

```go
// REMOVER:
if !b.cfg.AgentConfig.OnboardingLLMEnabled {
    return
}
```

O LLM runner é sempre construído. Se `OnboardingModel` não estiver configurado, o startup falha com erro explícito (ver F3-2).

### F3-2. `configs/config.go` — gate de validação mandatório

Na função `Validate()` de `AgentConfig` (ou `Config`):

```go
if strings.TrimSpace(c.AgentConfig.OnboardingModel) == "" {
    return errors.New("AGENT_ONBOARDING_LLM_MODEL é obrigatório — onboarding sem LLM é proibido")
}
```

### F3-3. `configs/config.go` — remover campo `OnboardingLLMEnabled`

```go
// REMOVER:
OnboardingLLMEnabled bool `mapstructure:"AGENT_ONBOARDING_LLM_ENABLED"`
```

Atualizar todos os usos de `OnboardingLLMEnabled` no codebase (grep para confirmar escopo):

```bash
grep -rn "OnboardingLLMEnabled" internal/ configs/ cmd/ --include="*.go"
```

---

## Shared-patterns.md — Padrões Aplicados

| Padrão | Onde aplicado neste plano |
|---|---|
| **Repository Pattern** | Consumer lê do outbox via interface; não acessa SQL diretamente |
| **Dependency Injection** | `StartBudgetConfigurationUseCase`, `AgentRuntimeExecutor` injetados via construtor |
| **Error Handling Cross-Stack** | Falha no LLM greeting logada como warn; falha de decode retorna erro para retry do outbox |
| **Value Objects** | `UserID uuid.UUID`, `Channel` string tipado por constante |

**Cross-module reuse futura**: mover `ConversationMessage` + `TurnHistory` para `internal/platform/conversation` (ação pós-MVP).

---

## Mastra Message History — Situação

O `RunOnboardingTurn` já persiste conversation history em `agent_sessions.recent_turns` (JSONB, janela de 3 pares). **Nenhuma coluna nova em `onboarding_sessions` é necessária** para o caminho LLM. A Fase 2 anterior (migração SQL + `OnboardingTurn` entity) foi descartada.

---

## Definition of Done (DoD)

- [ ] **F1**: `HandleActivation` envia `welcome_activated` + `onboarding_intro` e cria sessão via `StartBudgetConfiguration`.
- [ ] **F1**: `ConsumeResult.UserID` populado quando `Outcome == ConsumeOutcomeActivated`.
- [ ] **F1**: `startResult.Reply` **não** é enviado (LLM é o responsável pela primeira pergunta).
- [ ] **F2**: Evento `onboarding.subscription_bound` carrega `peer_e164` no payload.
- [ ] **F2**: `OnboardingBoundConsumer` registrado no agent module e escutando o evento correto.
- [ ] **F2**: Consumer chama `AgentRuntime.Execute()` com sentinel e o LLM envia `emitWelcome()`.
- [ ] **F3**: Campo `OnboardingLLMEnabled` removido de `configs/config.go` e de todos os usos.
- [ ] **F3**: Startup falha explicitamente se `AGENT_ONBOARDING_LLM_MODEL` estiver vazio.
- [ ] **F3**: Early-return condicional em `buildOnboardingRunner()` removido.
- [ ] Zero comentários em todos os arquivos `.go` alterados (gate R-ADAPTER-001.1).
- [ ] Sem SQL direto em consumer, processor ou wiring (gate R-ADAPTER-001.2).
- [ ] `go build ./...` passa sem erro.
- [ ] `go test ./internal/onboarding/application/... ./internal/agent/application/... ./configs/...` passa.

## Critérios de Aceite

1. **Happy path**: enviar `ATIVAR [token-válido]` → receber no WhatsApp, sem enviar outra mensagem:
   - Mensagem 1: texto de `WA_MSG_WELCOME_ACTIVATED`
   - Mensagem 2: texto de `WA_MSG_ONBOARDING_INTRO`
   - Mensagem 3: primeira saudação do LLM (via `emitWelcome()` do `RunOnboardingTurn`)

2. **LLM mandatório**: aplicação com `AGENT_ONBOARDING_LLM_MODEL=""` **não sobe** — retorna erro de configuração na inicialização.

3. **Idempotência do consumer assíncrono**: se `SubscriptionBoundSessionConsumer` do onboarding module disparar antes do `OnboardingBoundConsumer`, a sessão em `StartBudgetOutcomeResume` não reenvia mensagem duplicada.

4. **Degradação controlada**: se o LLM falhar durante `emitWelcome()`, o consumer loga warn e retorna erro para retry do outbox — sem quebrar a ativação que já foi confirmada.

5. **Zero comentários**: `grep` de comentários proibidos retorna vazio em todos os arquivos alterados.

6. **Sem feature flag**: `grep -r "OnboardingLLMEnabled" internal/ configs/` retorna vazio após a implementação.

---

## Ordem de Implementação Recomendada

1. F3 (remover feature flag) — menor risco, independente, valida compilação.
2. F1 (auto-start onboarding) — adiciona campos e lógica no `internal/onboarding`.
3. F2-1 (adicionar `peer_e164` ao evento) — pequena mudança em `binding/`.
4. F2-2 + F2-3 (consumer + wiring no agent module) — nova infra, valida end-to-end.

## Gates de Verificação

```bash
# Build completo
go build ./...

# Testes afetados
go test ./internal/onboarding/application/... \
        ./internal/agent/application/... \
        ./internal/agent/infrastructure/... \
        ./configs/...

# Zero comentários
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "^[[:space:]]*//" \
  internal/onboarding/application/ \
  internal/agent/infrastructure/messaging/database/consumers/ \
  | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL" || echo "OK"

# Sem SQL em adapters
grep -rn "QueryContext\|ExecContext\|db\.Query\|tx\.Exec" \
  internal/agent/infrastructure/messaging/database/consumers/onboarding_bound_consumer.go \
  && echo "FAIL: SQL direto" || echo "OK"

# LLM flag removido
grep -rn "OnboardingLLMEnabled" internal/ configs/ cmd/ --include="*.go" \
  && echo "FAIL: flag ainda presente" || echo "OK"

# Switch de domínio não cresceu
f=$(find internal/agent -name "daily_ledger_agent.go" ! -name "*_test.go")
cases=$(grep -cE "^[[:space:]]*case intent\.Kind" "$f" || true)
[ "${cases:-0}" -gt 1 ] && echo "FAIL: switch cresceu" || echo "OK"
```
