# Plano: Isolamento de Agentes, Gaps de Persistência e Robustez — Parte 3

- **Data**: 2026-06-23
- **Módulos afetados**: `internal/agent`, `internal/onboarding`, `configs`
- **Skills obrigatórias**: `$go-implementation` (Etapas 1–5), `$mastra`
- **Prioridade**: Alta
- **Dependência**: Parte 1 (`2026-06-23-onboarding-auto-start-llm-mandatory.md`) e Parte 2 (`2026-06-23-onboarding-persistencia-isolada-conclusao-deterministica-part-2.md`)

---

## Objetivo

Mapear toda a persistência do `internal/agent`, identificar gaps concretos e lacunas de isolamento
introduzidos pelo plano de auto-start (Parte 1) e pelo isolamento de persistência (Parte 2), e
prescrever as correções obrigatórias para que o MVP seja robusto, idempotente e livre de
interferência entre agentes.

---

## 1. Mapa Completo de Persistência

### 1.1 Tabelas do `internal/agent`

| Tabela | Chave natural | Escopo | O que armazena |
|---|---|---|---|
| `agent_threads` | `UNIQUE(user_id, channel)` | por usuário/canal | Identidade Thread Mastra — mapeia `(user_id, channel)` a um `thread_id` |
| `agent_runs` | `PK(id)`, idx `(thread_id, started_at)` | por execução | Audit trail de cada `AgentRuntime.Execute`: workflow, tool, outcome, status, duration |
| `agent_decisions` | `UNIQUE(user_id, channel, message_id)` | por mensagem | Idempotência de escrita — LLM snapshot, status de decisão, `resulting_event_id` |
| `agent_sessions` | `UNIQUE(user_id, channel)` | por usuário/canal | Estado transiente: `pending_action` (JSONB draft) + `recent_turns` (JSONB histórico) |
| `agent_working_memory` | `PK(user_id)` | por usuário (cross-channel) | WorkingMemory Mastra — markdown injetado no system prompt |
| `agent_observations` | `PK(id)`, idx `(user_id, channel)` | por usuário/canal | Fatos de curto prazo com janela de 90 dias |

### 1.2 Tabelas do `internal/onboarding`

| Tabela | Chave natural | Escopo | O que armazena |
|---|---|---|---|
| `onboarding_sessions` | `PK(user_id)` | por usuário | Estado FSM V2: `state`, `payload` JSONB, `updated_at` |
| `onboarding_tokens` | `UNIQUE(token_hash)` | por token | Lifecycle PENDING → PAID → CONSUMED → EXPIRED |

### 1.3 Colunas JSONB e structs Go

| Tabela.Coluna | Go struct serializado | Discriminador de tipo |
|---|---|---|
| `agent_sessions.pending_action` | `pendingexpense.Draft` | campo `Kind = "pending_expense"` |
| `agent_sessions.pending_action` | `onboardingv2draft.Draft` | campo `Kind = "onboarding_v2"` |
| `agent_sessions.pending_action` | `budgetdraft.Draft` | campo `Kind = "budget_config"` |
| `agent_sessions.recent_turns` | `[]entities.ConversationMessage` | array JSON |
| `agent_decisions.redacted_response` | `json.RawMessage` | snapshot bruto do LLM |
| `onboarding_sessions.payload` | `OnboardingSessionPayload` (FSM) | campo `state` TEXT governa o fluxo |
| `onboarding_tokens.metadata` | `map[string]any` | livre |

### 1.4 Repositórios e interfaces

| Interface | Implementação Postgres | Métodos-chave |
|---|---|---|
| `AgentThreadRepository` | `postgres/agent_thread_repository.go` | `GetByUserAndChannel`, `Upsert` |
| `AgentRunRepository` | `postgres/agent_run_repository.go` | `Insert`, `UpdateOnFinish` |
| `AgentDecisionRepository` | `postgres/agent_decision_repository.go` | `Insert`, `FindByMessage`, `UpdateSettlement` |
| `AgentSessionRepository` | `postgres/agent_session_repository.go` | `GetByUserAndChannel`, `Upsert`, `Update`, `DeleteExpired` |
| `WorkingMemoryRepository` | `postgres/working_memory_repository.go` | `Get`, `Upsert` |
| `ObservationRepository` | `postgres/observation_repository.go` | `Insert`, `ListRecent`, `DeleteExpired`, `DeleteOldestBeyondLimit` |

---

## 2. Isolamento entre Agentes — Estado Atual

Não existe isolamento por instância de agente. Uma única instância de `DailyLedgerAgent` e
`OnboardingAgent` é compartilhada entre todos os usuários. O isolamento é inteiramente
database-enforced via predicados e constraints:

| Cenário | Mecanismo | Chave de partição |
|---|---|---|
| Dois usuários distintos | `UNIQUE(user_id, ...)` | `user_id` |
| Mesmo usuário, canais diferentes | `UNIQUE(user_id, channel)` | `(user_id, channel)` |
| Mesma mensagem reenviada | `UNIQUE(user_id, channel, message_id)` | `(user_id, channel, message_id)` |
| WorkingMemory — escrita concorrente | `ON CONFLICT (user_id) DO UPDATE` atômico | `user_id` |
| Session — dois flows simultâneos | `ON CONFLICT (user_id, channel) DO UPDATE` | `(user_id, channel)` |

A separação entre `OnboardingAgent` e `DailyLedgerAgent` é garantida pela prioridade do
`IntentRouter`:

```
pendingExpenseConfirmation → OnboardingAgent.Handle() → DailyLedgerAgent.Handle()
```

Se `OnboardingAgent.Handle()` retorna `true`, o daily nunca é chamado para aquela mensagem.

---

## 3. O que o Plano de Auto-Start Adiciona à Persistência

O plano da Parte 1 **não cria nenhuma tabela nova**. As mudanças de persistência são:

- `onboarding.subscription_bound` payload ganha campo `peer_e164` (JSON, sem migração SQL).
- `agent_sessions.recent_turns` — já persiste histórico pelo `RunOnboardingTurn`. Continua sem mudança.
- `onboarding_sessions` — `StartBudgetConfiguration` já cria o registro; Fase 1 apenas chama o use case existente.
- `agent_runs` — `OnboardingBoundConsumer` chama `AgentRuntime.Execute`, que abre/fecha um `Run` normalmente.

---

## 4. Gaps e Riscos Identificados

### GAP-1 — Race condition F1 × F2 (crítico)

**Problema**: `subscription_bound` é publicado no outbox **dentro** de `BindAndConsume`. A Fase 1
chama `StartBudgetConfiguration` **após** `consumeUseCase.Execute` retornar — depois que o evento
já está gravado no outbox.

Sequência de risco:
```
1. consumeUseCase → BindAndConsume → grava evento no outbox (TX commit)
2. Outbox poller acorda ANTES de HandleActivation chamar startBudgetUseCase
3. OnboardingBoundConsumer dispara → chama RunOnboardingTurn
4. RunOnboardingTurn checa InProgress → onboarding_sessions ainda não existe → Handled=false
5. Consumer retorna nil (evento consumido sem retry)
6. HandleActivation finalmente chama startBudgetUseCase → cria onboarding_sessions
7. LLM nunca reenvia o welcome — evento já consumido
```

**Correção obrigatória**: Se `OnboardingStateReader.Load()` retorna `InProgress=false`, o consumer
deve retornar **erro** (não nil) para forçar retry do outbox. Adicionar log warn com
`"reason": "onboarding_not_started"`. A retentativa virá após a sessão ser criada.

### GAP-2 — `messageID` ausente no OnboardingBoundConsumer (crítico)

**Problema**: `AgentRuntime.Execute` registra `AgentDecision` usando
`(user_id, channel, message_id)` como chave de idempotência. O plano não especifica qual
`messageID` usar no trigger proativo. Sem um `messageID` estável, retentativas do outbox enviam
`emitWelcome()` múltiplas vezes e o usuário recebe boas-vindas duplicadas.

**Correção obrigatória**: usar `envelope.EventID.String()` como `messageID`. É UUID estável por
evento — retentativas usam o mesmo ID e a decisão existente é detectada como replay pela constraint
`UNIQUE(user_id, channel, message_id)`.

```go
if err := c.agentRuntime.Execute(ctx, appinterfaces.AgentExecuteInput{
    UserID:    userID,
    Channel:   "whatsapp",
    Peer:      p.PeerE164,
    Text:      "__onboarding_welcome__",
    MessageID: envelope.EventID.String(),
}); err != nil { ... }
```

### GAP-3 — `AgentRuntimeExecutor` interface inexistente

**Problema**: O plano define `AgentRuntimeExecutor` como interface no consumer (R6.3 — interface
no consumidor), mas o repositório não contém essa interface. `AgentRuntime.Execute` tem assinatura
com parâmetros individuais, não um DTO.

**O que criar** em `internal/agent/application/interfaces/agent_runtime_executor.go`:

```go
type AgentExecuteInput struct {
    UserID    uuid.UUID
    Channel   string
    Peer      string
    Text      string
    MessageID string
}

type AgentRuntimeExecutor interface {
    Execute(ctx context.Context, in AgentExecuteInput) error
}
```

`AgentRuntime` deve implementar essa interface ou um shim deve ser criado no wiring.

### GAP-4 — `agent_sessions.pending_action` sem discriminador SQL

**Problema**: três tipos de Draft compartilham a mesma coluna JSONB sem coluna de tipo no schema.
A única separação é o campo `Kind` dentro do JSON. Um bug pode gravar um Draft errado na linha
correta e `continuePendingExpenseConfirmation` tentará deserializar um draft de onboarding como
draft de despesa — erro silencioso ou comportamento incorreto.

**Ação pós-MVP** (baixo risco imediato, mas gap real de observabilidade e segurança):

```sql
ALTER TABLE agent_sessions ADD COLUMN pending_action_kind TEXT;
```

Permite queries de diagnóstico e gate de validação: ao gravar, seta `pending_action_kind`; ao
limpar, nulifica.

### GAP-5 — Estado dual (FSM + LLM) pode divergir

**Problema**: o estado do onboarding é rastreado em dois lugares:

- `onboarding_sessions.state` — FSM state (AwaitingIncome, AwaitingCardDecision, etc.)
- `onboarding_sessions.payload.recent_turns` (após Parte 2) — histórico LLM

Se `OnboardingToolDispatcher` falhar após a tool call do LLM mas antes de persistir a transição
de estado, a FSM fica presa em `InProgress=true` enquanto o LLM considera o passo concluído.

**Verificação obrigatória**: confirmar que `CompleteOnboarding` é idempotente — re-executar não
duplica evento nem quebra a sessão. Confirmar que o use case usa UoW/TX atômica para o write.

### GAP-6 — Ausência de testes para `OnboardingBoundConsumer`

**Problema**: o DoD da Parte 1 lista `./internal/agent/infrastructure/...` no gate de testes mas
não especifica cenários. O consumer novo ficará sem cobertura.

**Cenários obrigatórios** (suite testify, padrão R-TESTING-001):

| Cenário | Resultado esperado |
|---|---|
| Payload válido + `agentRuntime` ok | `Execute` chamado com `MessageID = envelope.EventID.String()` |
| `peer_e164` vazio | retorna `nil` (warn, não erro, sem retry) |
| `agentRuntime.Execute` retorna erro | consumer retorna erro (retry do outbox) |
| `user_id` inválido no payload | `decodeFailed` counter +1, retorna erro |
| `InProgress=false` (GAP-1) | consumer retorna erro (retry até sessão existir) |

### GAP-7 — `DisallowUnknownFields` no consumer existente de `subscription_bound`

**Problema**: o plano menciona que `SubscriptionBoundSessionConsumer` "ignora campos extras". Isso
**deve ser verificado** antes de adicionar `peer_e164`. Se o consumer usa `DisallowUnknownFields`,
a adição do campo novo quebra o consumer existente em produção.

**Ação obrigatória antes de F2-1**:

```bash
grep -rn "DisallowUnknownFields\|decoder.Strict\|Decoder.*Strict" internal/onboarding/ \
  && echo "WARN: verificar impacto de peer_e164" || echo "OK"
```

---

## 5. Ordem de Implementação Revisada

A ordem da Parte 1 recomendava F3 → F1 → F2-1 → F2-2+F2-3. Com os gaps acima, a ordem segura é:

1. **GAP-7 check** — verificar `DisallowUnknownFields` no consumer existente.
2. **GAP-3** — criar `AgentRuntimeExecutor` interface em `application/interfaces/`.
3. **F3** — remover flag `OnboardingLLMEnabled`, validação mandatória de `AGENT_ONBOARDING_LLM_MODEL`.
4. **F2-1** — adicionar `peer_e164` ao payload do evento.
5. **F1** — `ConsumeResult.UserID`, `onboarding_intro`, `startBudgetUseCase` no processor.
6. **F2-2** — `OnboardingBoundConsumer` com `MessageID = envelope.EventID.String()` e retorno de erro em `InProgress=false`.
7. **F2-3** — wiring no `module.go`.
8. **Testes** — suite para `OnboardingBoundConsumer` (GAP-6).

---

## Definition of Done (DoD)

- [ ] **GAP-1**: `OnboardingBoundConsumer` retorna **erro** (não nil) quando `InProgress=false`, forçando retry do outbox.
- [ ] **GAP-2**: `MessageID` do trigger proativo é `envelope.EventID.String()` — verificado por teste.
- [ ] **GAP-3**: Interface `AgentRuntimeExecutor` com `AgentExecuteInput{UserID, Channel, Peer, Text, MessageID}` criada em `internal/agent/application/interfaces/`.
- [ ] **GAP-7**: `grep DisallowUnknownFields` executado antes de F2-1 e resultado documentado.
- [ ] **GAP-5**: `CompleteOnboarding` confirmado como idempotente (teste ou revisão de código).
- [ ] **F1**: `HandleActivation` envia `welcome_activated` + `onboarding_intro` e cria sessão via `StartBudgetConfiguration`.
- [ ] **F1**: `ConsumeResult.UserID` populado quando `Outcome == ConsumeOutcomeActivated`.
- [ ] **F1**: `startResult.Reply` do FSM **não** é enviado (LLM é o responsável pela primeira pergunta).
- [ ] **F2**: Evento `onboarding.subscription_bound` carrega `peer_e164` no payload.
- [ ] **F2**: `OnboardingBoundConsumer` registrado no agent module e escutando o evento correto.
- [ ] **F2**: Consumer chama `AgentRuntime.Execute()` com sentinel `__onboarding_welcome__` e `MessageID = envelope.EventID.String()`.
- [ ] **F3**: Campo `OnboardingLLMEnabled` removido de `configs/config.go` e de todos os usos.
- [ ] **F3**: Startup falha explicitamente se `AGENT_ONBOARDING_LLM_MODEL` estiver vazio.
- [ ] **F3**: Early-return condicional em `buildOnboardingRunner()` removido.
- [ ] Testes da suite de `OnboardingBoundConsumer` cobrem os 5 cenários listados no GAP-6.
- [ ] Zero comentários em todos os arquivos `.go` alterados (gate R-ADAPTER-001.1).
- [ ] Sem SQL direto em consumer, processor ou wiring (gate R-ADAPTER-001.2).
- [ ] `go build ./...` passa sem erro.
- [ ] `go test ./internal/onboarding/application/... ./internal/agent/application/... ./internal/agent/infrastructure/... ./configs/...` passa.

---

## Critérios de Aceite

1. **Happy path end-to-end**: enviar `ATIVAR [token-válido]` → receber no WhatsApp, sem enviar outra mensagem:
   - Mensagem 1: texto de `WA_MSG_WELCOME_ACTIVATED`
   - Mensagem 2: texto de `WA_MSG_ONBOARDING_INTRO`
   - Mensagem 3: primeira saudação do LLM via `emitWelcome()` do `RunOnboardingTurn`

2. **LLM mandatório**: aplicação com `AGENT_ONBOARDING_LLM_MODEL=""` não sobe — retorna erro de configuração na inicialização.

3. **Idempotência do trigger proativo**: se o outbox fizer retry do evento `subscription_bound`, o usuário não recebe saudação duplicada. O `AgentDecision` com `MessageID = envelope.EventID` já existe e é detectado como replay.

4. **Race condition GAP-1 tratada**: se o consumer disparar antes de `StartBudgetConfiguration` criar a sessão, retorna erro e o outbox reprocessa. Quando a sessão existir, `emitWelcome()` é enviado normalmente.

5. **Sem interferência entre agentes**: após ativação e durante o onboarding, mensagens do usuário são tratadas exclusivamente pelo `OnboardingAgent`. O `DailyLedgerAgent` não recebe nenhuma mensagem enquanto `InProgress=true`.

6. **Degradação controlada**: se o LLM falhar durante `emitWelcome()`, o consumer loga warn e retorna erro para retry — sem quebrar a ativação que já foi confirmada ao usuário.

7. **Zero comentários**: `grep` de comentários proibidos retorna vazio em todos os arquivos alterados.

8. **Sem feature flag**: `grep -r "OnboardingLLMEnabled" internal/ configs/` retorna vazio após a implementação.

9. **Interface no consumidor (R6.3)**: `OnboardingBoundConsumer` depende de `AgentRuntimeExecutor` (interface local) — não de `*AgentRuntime` concreto.

10. **Gates automatizados passando**:
    ```bash
    # Build
    go build ./...

    # Testes
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

    # DisallowUnknownFields no consumer existente
    grep -rn "DisallowUnknownFields\|decoder.Strict" internal/onboarding/ \
      && echo "WARN: verificar impacto de peer_e164" || echo "OK"

    # Switch de domínio não cresceu
    f=$(find internal/agent -name "daily_ledger_agent.go" ! -name "*_test.go")
    cases=$(grep -cE "^[[:space:]]*case intent\.Kind" "$f" || true)
    [ "${cases:-0}" -gt 1 ] && echo "FAIL: switch cresceu" || echo "OK"
    ```
