# Endurecimento Production-Proof — WhatsApp → Agent → OpenRouter → Módulos

- Data: 2026-06-22
- Escopo: `internal/agent`, `internal/platform/whatsapp`, `internal/identity` (estabelecimento de principal), `cmd/server`
- Tipo: diagnóstico + especificação de remediação (não é PRD; é plano técnico de hardening)
- **Skill obrigatória para QUALQUER implementação derivada deste plano:** `.agents/skills/go-implementation/SKILL.md`
  (Etapas 1–5 + Checklist R0–R7). Regras transversais aplicáveis: `R-ADAPTER-001` (`.claude/rules/go-adapters.md`),
  `R-TXN-WORKFLOWS-001` (`.claude/rules/transactions-workflows.md`), zero comentários em `.go` de produção.
- **Referência de modelagem (quando necessário):** *Domain Modeling Made Functional* (DMMF, Scott Wlaschin) —
  smart constructor, state-as-type, transições puras. Já alinhado a `domain-modeling.md` do repositório.
- **Fronteira inegociável:** `internal/agent → internal/<modulos>` via `binding/` (adapter→usecase). NUNCA invadir
  outro módulo (sem SQL direto, sem compartilhar transação com a UoW de outro módulo, sem importar repo de domínio
  alheio). Toda escrita do agent em tabela agent-owned usa UoW própria do agent.

> **STATUS (2026-06-22): TODAS AS 10 FASES IMPLEMENTADAS E VALIDADAS na `main`** (sem branch/PR).
> Concluídas: P0-1 (auditoria agent-owned `agent_decisions`, migration 000011), P0-2 (guarda authz fail-closed +
> métrica `agent_authz_denied_total`), P0-3 (sanitização/PII), P1-1 (`trace_id` no evento), P1-2 (retry só em
> LEITURA — escrita nunca re-tentada p/ não duplicar lançamento), P1-3 (status callbacks Meta, migration 000012 +
> tabela `whatsapp_message_status`), P1-4 (housekeeping dedup), P1-5 (métrica `agent_llm_tokens_total`), P2-2
> (remoção de config morto), P2-3 (testes de regressão authz/retry/isolamento).
> Evidência: `go build ./...`, `go vet ./...`, `go test ./...` e migrations integration — todos verdes. Onboarding
> intacto (e2e + onboarding verdes em cada lote). Gates zero-comentários / SQL-adapter / init / R-TXN OK.
> Resíduos documentados: `agent_decisions.resulting_event_id` usa `uuid.New()` (bindings não expõem event id real);
> guarda P0-2 é defense-in-depth (isolamento já estrutural).
> P1-5: métrica `agent_llm_tokens_total{model,type}` emitida no provider OpenRouter (`recordTokens`).
> P2-2: removido config morto `AGENT_LLM_PROMPT_PAD_TOKENS` (struct+default+`.env.example`; sem uso em produção/teste).
> Pendentes como PR contida cada (superfície de regressão em `cmd/server`/contratos): P1-4 (housekeeping dedup),
> P0-1 (auditoria agent-owned + wiring), P0-2 (authz defense-in-depth), P1-2 (retry agent→módulo),
> P1-3 (status callbacks Meta), P2-3 (testes de isolamento/replay).
> P1-1 entregue: `trace_id` (via `Tracer().SpanFromContext(ctx).TraceID()`) como campo explícito em
> `interfaces.IntentEvent` + payload `trace_id,omitempty` em `intent_event_publisher.go`, populado em
> `IntentRouter.publishEvent`. Testes: shape com trace_id + omitempty. Suite agent (e2e+onboarding) verde.
> Regra arquitetural reforçada: `internal/agent` chama `internal/<modulos>` via `binding/`→usecase e mantém
> persistência própria quando necessário (agent-owned), sem compartilhar transação com outro módulo.
> P0-3 entregue: `internal/agent/application/sanitize/{sanitizer.go,sanitizer_test.go}`, wiring em
> `parse_inbound.go` (`NewParseInbound(chain, cfg.MaxInputChars, o11y)` + `Sanitizer.Clean`), config
> `AGENT_LLM_MAX_INPUT_CHARS` (default 2000, validação prod `(0..8192]`) em `configs/config.go` + `.env.example`.
> Gates: `go build ./...`, `go vet ./...`, `go test ./internal/agent/...` (inclui e2e + onboarding), `go test
> ./configs/...` — todos verdes. Zero-comentários/R0/R5/R7 OK. **Onboarding não regrediu** (é tratado antes do
> parser no `route()`, então a sanitização do `parse_inbound` não o afeta). Design de 6.A/6.B preservado para reuso.

## 1. Context

O mecontrola recebe mensagens do WhatsApp (Meta Cloud API), valida e roteia no `internal/platform/whatsapp`,
estabelece o principal (`phone → user_id`) no `internal/identity`, interpreta a intenção com LLM via OpenRouter
(`internal/agent`) e executa a ação chamando use cases de outros módulos por **função Go in-process** através da
camada `internal/agent/infrastructure/binding/*`. A arquitetura é boa para MVP. Este plano lista o que manter, o que
falta e especifica a remediação priorizada (P0/P1/P2), com prompt de implementação ao final.

> **Esclarecimento de 2026-06-22 que recalibra prioridades:** os lançamentos criados pelo agent são **registros
> para relatório, sem movimentação financeira real**. Logo, gates de confirmação/aprovação financeira **não se
> aplicam**. O foco production-proof passa a ser **idempotência, rastreabilidade, isolamento por `user_id` e PII**.

### Achados verificados em código (anti-falso-positivo)

| Achado | Verificação | Consequência para prioridade |
|--------|-------------|------------------------------|
| Replay de webhook já é bloqueado | `dispatcher.go:131-142` deduplica WAMID **antes** do agent | Idempotência extra no agent (P0-1) é **defense-in-depth redundante** → baixo retorno |
| LLM não injeta tenant | `parse_intent.system.tmpl:45` proíbe `user_id/tenant_id`; handlers usam `principal.UserID` | Override de tenant via LLM é **estruturalmente impossível** |
| Isolamento por tenant existe na borda de dados | repos filtram `WHERE user_id = $1` (`monthly_summary_repository.go:71`, etc.) | P0-2 (AuthzChecker) é **defense-in-depth, não bloqueador** |
| Sem limite de tamanho de input ao LLM | `parse_inbound.go:86` faz só `TrimSpace` | **Gap real e não-redundante** → P0-3 é a recomendação de maior valor |

**Recomendação para MVP production-proof (sem falso positivo):** priorizar **P0-3** (endurecimento de input + PII),
único gap verificado e não-redundante. P0-1 (idempotência) e P0-2 (authz) ficam como defense-in-depth opcional.

> Links externos do pedido original são apenas referência conceitual. Todo o diagnóstico está ancorado no código
> real (arquivos + linhas).

## 2. Mapa do fluxo (como é hoje)

```
Meta Cloud API
  └─POST /api/v1/whatsapp/inbound
     ├─ middleware rate-limit por IP (trusted-proxy CIDR)
     ├─ signature.Compose → RawBody(256KB) + HMAC-SHA256 (current+next, compare constante)
     └─ InboundHandler.Handle
        └─ Dispatcher.Route (internal/platform/whatsapp/dispatcher/dispatcher.go:106)
           ├─ ExtractFirstMessage (payload/parser.go)
           ├─ checkTimestamp  (janela 5 min; rejeita stale ANTES do dedup)   [linha 35,126,183]
           ├─ dedup.InsertIfAbsent(WAMID)  ON CONFLICT DO NOTHING            [linha 131]
           ├─ MatchActivationCommand → onboardingRoute                        [linha 145]
           ├─ EstablishPrincipal (identity)  phone→user_id                    [linha 149]
           ├─ limiter.Allow(user_id) (token bucket)                           [linha 166]
           └─ auth.WithPrincipal(ctx) → agentRoute                            [linha 179]
              └─ IntentRouter.route (application/services/intent_router.go:481)
                 ├─ onboardingRunner.Run (function-calling)                   [linha 491]
                 ├─ parser.Parse → OpenRouter (JSON-schema strict, T=0)       [linha 521]
                 │    └─ providers/openrouter/client.go  (+ fallback_chain + circuit_breaker)
                 └─ switch kind { 17 intents }                                [linha 544-587]
                    └─ route<Kind> → binding/<x>.go (adapter) → <modulo>.UseCase.Execute
                       └─ outbox.Publisher (agent.intent.executed / .rejected)
```

## 3. O que está BOM (preservar — não regredir)

| Área | Evidência | Por que é bom |
|------|-----------|---------------|
| Verificação de webhook | `signature/hmac.go`, `handlers/verify_handler.go` | HMAC-SHA256 sobre raw body, compare constante, rotação current+next; verify-token correto |
| Anti-replay | `dispatcher.go:35,126,131,192` | Janela 5 min **antes** do dedup por WAMID (`ON CONFLICT DO NOTHING`); corpo limitado a 256KB |
| Fronteiras de módulo | `internal/agent/infrastructure/binding/*` | R-ADAPTER-001 respeitado: agent importa só interfaces de use case; sem SQL/branching no adapter |
| Resiliência LLM | `services/fallback_chain.go`, `services/circuit_breaker.go` | Fallback multi-modelo + circuit breaker por modelo; erros classificados (401/402/408/429/5xx) |
| Determinismo | `usecases/parse_inbound.go`, `providers/openrouter/client.go` | `temperature=0` + JSON-schema strict + validação de invariante pós-LLM com fallback `KindUnknown` |
| Observabilidade | spans OTel em cada etapa; ~20 counters/histogramas | Não loga prompt/resposta na aplicação (sem vazamento por log) |
| Rate limit | middleware IP + `ratelimit/limiter.go` por user_id | Dois níveis, GC de buckets |
| Lifecycle | `cmd/server/server.go` | Shutdown coordenado limiter(5s)→DB(15s)→o11y(10s); contexto de sinal propagado |

## 4. Lacunas (o que falta / melhorar)

### P0 — Bloqueadores de produção (segurança & integridade financeira)

**P0-1 — Sem trilha de auditoria/idempotência da decisão do agent.**
> NOTA (esclarecido 2026-06-22): os lançamentos do agent são **registros para relatório, sem movimentação
> financeira real**. Logo, **NÃO há gate de confirmação** — foi removido do escopo. O valor remanescente deste item
> é **idempotência** (não duplicar o mesmo lançamento de relatório vindo do mesmo WAMID) e **rastreabilidade**.

A saída do LLM cria lançamento / ativa budget / registra cartão direto no DB (`intent_router.go:544-587` →
`binding/*`). Existe `agent.intent.executed` no outbox, mas ele não amarra `prompt + resposta-do-LLM + trace_id +
escrita resultante` num registro consultável, nem garante idempotência por mensagem.
**Impacto:** lançamento duplicável e decisão do agent não rastreável ponta-a-ponta. Severidade rebaixada de
bloqueador-financeiro para **rastreabilidade/idempotência** após o esclarecimento.

**P0-2 — Autorização por ação ausente no agent.**
O agent confia que cada use case valida ownership. `user_id` isola tenant, mas não há checagem explícita
"este principal pode esta ação sobre este recurso" antes do dispatch no `route()`.
**Impacto:** um bug de authz em qualquer módulo passa direto pelo canal WhatsApp.

**P0-3 — PII financeira enviada verbatim ao OpenRouter.**
Valores, merchant e apelido de cartão vão no prompt sem minimização/masking e sem cláusula no-train/retenção
documentada do provider (`providers/openrouter/client.go`, prompts em `infrastructure/llm/prompts/`).
**Impacto:** risco LGPD e exposição a terceiro.

**P0-4 — Sem defesa de prompt injection.**
Texto do usuário entra no prompt sem sanitização/escape nem limite de tamanho pré-LLM. O persona prompt tem
instrução anti-override, mas isso é insuficiente para um canal que dispara escrita financeira.

### P1 — Confiabilidade & rastreabilidade

- **P1-1 — `trace_id` fora do payload do evento de intent.** Trace existe no contexto OTel mas não é propagado
  para o payload do evento de outbox → difícil correlacionar webhook→intent→escrita em incidente.
- **P1-2 — Sem retry/backoff no caminho agent→módulo.** Erro de use case vira mensagem ao usuário imediatamente;
  o circuit breaker cobre só o LLM (`intent_router.go:596-602`).
- **P1-3 — Status callbacks da Meta não tratados.** Sem endpoint para delivery/read/failed; envio de saída é
  `WithoutRetry()`. Usuário pode não receber resposta e o sistema não percebe.
- **P1-4 — Tabela de dedup cresce sem limite.** `channel_processed_messages` sem job de limpeza/particionamento.
- **P1-5 — Sem tracking de custo/tokens.** `prompt_tokens`/`completion_tokens` retornados mas não persistidos nem
  expostos em métrica; sem teto de gasto por usuário.

### P2 — Qualidade conversacional & testes

- **P2-1 — Conversa single-turn.** `recent_turns` persistido em `agent_sessions` mas não enviado ao LLM. **Decidido NÃO-GOAL do MVP** (ver seção 11 com prós/contras + custo).
- **P2-2 — Config morta / truncamento silencioso.** `AGENT_LLM_PROMPT_PAD_TOKENS` nunca usado; sem pré-flight de
  tokens (truncamento só detectado por `finish_reason=length`).
- **P2-3 — Lacunas de teste.** Faltam testes de: isolamento multi-usuário; replay/idempotência; recuperação de
  falha parcial no budget config; dispatch sem principal no contexto (auth-bypass).

## 5. Correções (falsos positivos descartados — transparência)

1. **"GET `/inbound` → verifyHandler é bug"** — FALSO POSITIVO. É o challenge de verificação da Meta (GET) na
   callback URL; mapear `/verify` e `/inbound` GET ao verify handler é intencional (`whatsapp_router.go:51-52`).
2. **"Intent duplicado quando a janela de dedup (~5 dias) expira"** — FALSO POSITIVO. `checkTimestamp` rejeita
   webhook > 5 min antes do dedup (`dispatcher.go:35,126,192`); não há janela de 5 dias. Resíduo real = só o
   crescimento da tabela de dedup (P1-4).

## 6. Especificação de remediação

> Cada item respeita a fronteira `adapter → usecase` e a regra de zero comentários em `.go`. Nada de SQL/branching
> de domínio em handler/consumer/producer/job. Para `internal/transactions` vale o gate ADR-006 (lógica só em
> `Decide*`).

### P0-1 — Trilha de auditoria de decisão + gate de confirmação
- **Domínio:** novo agregado `agent_decision` (use case em `internal/agent/application/usecases`), persistido em
  nova tabela `mecontrola.agent_decisions` (migration nova): `id, user_id, channel, message_id (WAMID), intent_kind,
  prompt_sha256, llm_model, llm_response_raw (jsonb, sem PII crua — ver P0-3), trace_id, decided_action,
  resulting_event_id, status (executed|rejected|awaiting_confirmation), created_at`.
- **Fluxo (respeitando fronteira `internal/agent` → `internal/<modulos>`):** a tabela `agent_decisions` é
  **propriedade do módulo agent**. `IntentRouter.route` grava a decisão como `pending` numa transação **própria do
  agent** (`platform/database/uow` + `Do[T]`) **antes** de chamar o `binding` de escrita; após o retorno do use case
  do outro módulo, faz um segundo write agent-owned para `executed`/`rejected` + `resulting_event_id`. **NÃO
  compartilhar transação com o módulo de domínio** — o use case alvo (ex: `transactions.CreateTransactionUC`) gerencia
  sua própria `uow.Do` internamente; enlistá-lo violaria a fronteira. A atomicidade é trocada por um anchor de
  auditoria/idempotência (`pending → settled`).
- **Gate de confirmação:** REMOVIDO do escopo (lançamentos são para relatório, sem movimentação real). O agregado
  mantém o estado `awaiting_confirmation` apenas como estado de domínio reservado, não usado no fluxo atual.
- **Idempotência:** índice único `(user_id, channel, message_id)` em `agent_decisions`; `Insert` que colide com
  WAMID já processado retorna `ErrAgentDecisionConflict` → router trata como replay (não re-executa o lançamento).
- **Arquivos:** `internal/agent/application/services/intent_router.go` (pontos de escrita: `routeLogExpense:595`,
  `routeLogIncome`, `routeLogCardPurchase`, `routeCreateCard`, `routeConfigureBudget`); novo
  `internal/agent/application/usecases/record_agent_decision.go`; novo repo em
  `internal/agent/infrastructure/repositories/postgres/`; nova migration em `migrations/`.

### P0-2 — `AuthzChecker` explícito no IntentRouter
- **Interface (no consumidor):** `type AuthzChecker interface { Authorize(ctx, userID uuid.UUID, action string) error }`
  declarada em `internal/agent/application/services` (R6: interface no consumidor).
- **Uso:** chamada no início de cada `route<Kind>` de escrita; em falha → `OutcomeUsecaseError` + reply genérico
  (sem vazar motivo). Implementação inicial pode ser allow-all logado (baseline), evoluindo para checagem real.
- **Arquivo:** `intent_router.go` (injetar no struct + chamar nos handlers de escrita).

### P0-3 — Minimização/masking de PII + sanitização de input
- **Saída→LLM:** introduzir camada de redaction antes de montar o prompt em `parse_inbound.go` /
  `compose_conversational_reply.go` — manter o necessário para classificação, mascarar apelido de cartão e dígitos
  sensíveis; `llm_response_raw` da auditoria (P0-1) deve guardar versão redigida.
- **Input:** sanitizar/limitar tamanho do texto do usuário antes do prompt (limite configurável
  `AGENT_LLM_MAX_INPUT_CHARS`); rejeitar/escapar payloads anômalos.
- **Documentar** cláusula no-train/retention do OpenRouter no header do plano e no `.env.example`.

### P0-4 — coberto por P0-3 (sanitização + limite) somado ao persona prompt existente.

### P1 (resumo de implementação)
- **P1-1:** adicionar `trace_id` (via `span.TraceID()` — fast path da devkit, padrão do repo em
  `establish_principal.go:137`; NÃO usar `Context().TraceID()`) ao payload dos eventos de intent em
  `internal/agent/infrastructure/events/intent_event_publisher.go`.
- **P1-2:** wrapper de retry com backoff limitado para erros transitórios nos `binding/*` de escrita (idempotência
  garantida por P0-1 + WAMID).
- **P1-3:** endpoint `POST /api/v1/whatsapp/status` + tabela de status de saída; tornar envio de saída retentável.
- **P1-4:** job de limpeza/particionamento de `channel_processed_messages` (TTL operacional, ex: 30 dias).
- **P1-5:** persistir tokens por decisão (reusar P0-1) + métrica `agent_llm_tokens_total{model,kind}`; alerta de teto.

### P2 (resumo)
- **P2-1:** ❌ **NÃO-GOAL do MVP** (decisão 2026-06-22) — ver seção 11 com a análise de custo. Conversa segue single-turn.
- **P2-2:** ✅ feito — removido `AGENT_LLM_PROMPT_PAD_TOKENS`.
- **P2-3:** ✅ feito — testes de authz/retry/isolamento (e demais cobertos pelas suítes existentes).

### 6.A — Design validado da fundação P0-1 (auditoria agent-owned) — POC revertida, preservada

> Tudo dentro de `internal/agent` (fronteira respeitada). Build/gofmt/vet/test passaram na POC antes da reversão.

Arquivos a (re)criar:
- `migrations/000011_create_agent_decisions.{up,down}.sql` — tabela `mecontrola.agent_decisions` espelhando o estilo
  de `000010` (lock/statement_timeout, FK `user_id`→`users` ON DELETE CASCADE, CHECKs de tamanho, down via rename-
  archive). Colunas: `id, user_id, channel, message_id, intent_kind, prompt_sha256, llm_model, redacted_response
  jsonb, trace_id, decided_action, resulting_event_id null, status, created_at, settled_at null`. Índice **único
  `(user_id, channel, message_id)`** (idempotência) + índices `(user_id, created_at DESC)` e `(status)`. CHECK
  `status IN ('pending','executed','rejected','awaiting_confirmation')` e `char_length(prompt_sha256)=64`.
- `internal/agent/domain/valueobjects/decision_status.go` — enum `DecisionStatus` (`iota+1`: pending, executed,
  rejected, awaiting_confirmation) com `String()`, `IsZero()`, `IsSettled()`, `ParseDecisionStatus`.
- `internal/agent/domain/entities/agent_decision.go` — agregado **DMMF**: struct com campos não exportados +
  `AgentDecisionParams`; `NewPendingDecision`/`NewAwaitingConfirmationDecision` (smart constructor valida obrigatórios
  e `prompt_sha256` = 64 hex via `hex.DecodeString`); transições **puras e imutáveis** `Execute(eventID, settledAt)`
  e `Reject(settledAt)` (retornam nova cópia, recusam re-settle); getters incl. `ResultingEventID()/SettledAt()` com
  `(valor, bool)`. Tempo entra **por parâmetro** (sem Clock — `time.Now().UTC()` no caller).
- `internal/agent/application/interfaces/agent_decision_repository.go` — `AgentDecisionRepository{ Insert,
  UpdateSettlement }`, `AgentDecisionRepositoryFactory`, sentinelas `ErrAgentDecisionNotFound/Conflict`.
- `internal/agent/infrastructure/repositories/postgres/agent_decision_repository.go` + `nullable.go` — espelha
  `agent_session_repository.go` (tracer, `pgerrcode.UniqueViolation`→`ErrAgentDecisionConflict`, `jsonOrDefault`);
  helpers `nullableUUID/nullableTime` para colunas opcionais.
- `factory.go` — adicionar método `AgentDecisionRepository(db) interfaces.AgentDecisionRepository`.
- Teste de domínio `agent_decision_test.go` — `testify/suite` table-driven, **sem mocks** (digest válido 64-hex).

Wiring (não feito na POC): `IntentRouter` grava `pending` em UoW própria do agent **antes** do dispatch de escrita;
após retorno do `binding`, `UpdateSettlement` para `executed`(+`resulting_event_id`)/`rejected`. Threading
necessário: `messageID` (WAMID, já chega em `route(...messageID string)`), `prompt_sha256`+`llm_model`
(**requer mudança de contrato**: `ParseInboundOutput` hoje é `{Intent, Raw, DirectReply}` — não expõe modelo nem
prompt; surfacer isso), `trace_id` (`span.TraceID()`). **Gate de confirmação: NÃO implementar** (relatório).

### 6.B — Design validado do P0-3 (sanitização de input) — POC revertida, preservada — **RECOMENDADO**

> Núcleo puro, sem dependência de outros módulos. Masking conservador de alta precisão (zero falso positivo).

- `internal/agent/application/sanitize/sanitizer.go` — `Sanitizer` com `NewSanitizer(maxRunes int) (*Sanitizer,
  error)` compilando regex via **`regexp.Compile`** (não `MustCompile` fora de `main`). Método `Clean(raw) (string,
  error)`:
  1. `TrimSpace`; vazio → `ErrEmpty`.
  2. `normalizeControl`: remove control-chars/runas inválidas, converte `\n\t`→espaço, colapsa whitespace
     (`strings.Fields`).
  3. mascara **CPF formatado** `\b\d{3}\.\d{3}\.\d{3}-\d{2}\b` → `[REDACTED_CPF]`.
  4. mascara **cartão** `\b\d(?:[ -]?\d){12,18}\b` (13–19 dígitos) → `[REDACTED_CARD]`.
  5. `capRunes` ao limite (default `DefaultMaxRunes = 2000`).
  - **Conservador de propósito:** NÃO mascarar 11 dígitos crus (colidiria com telefone BR) nem valores curtos
    (amounts ficam intactos para o LLM classificar). Precisão alta = sem falso positivo na classificação.
- Wiring: injetar `*Sanitizer` no construtor de `ParseInbound` (`NewParseInbound` cria via `NewSanitizer`); em
  `Execute`, trocar `trimmed := strings.TrimSpace(input.Text)` por `Clean`, tratando `ErrEmpty`→`ErrParseInboundEmptyText`.
  O texto limpo alimenta `RenderUser`, fallback `NewUnknown` e (futuro) `redacted_response` da auditoria.
- Config (follow-up): expor `AGENT_LLM_MAX_INPUT_CHARS` em `AgentConfig` e passar a `NewSanitizer`; default 2000
  enquanto não configurado.
- Teste `sanitizer_test.go` — `testify/suite` table-driven cobrindo: preserva mensagem legítima, mantém amount curto,
  mascara CPF formatado, mascara cartão (com/sem separador), normaliza control-chars, aplica cap, default quando ≤0.
- **Complemento P0-3 (doc, não-código):** registrar no `.env.example`/README a política no-train/retention do
  OpenRouter (headers/data-policy) — confirmar que requests não são usados para treino.

## 7. Sequência sugerida (recalibrada)
**Recomendado p/ MVP:** 1. **P0-3** (sanitização/PII — seção 6.B, único gap não-redundante) → 2. P1-1 (`trace_id` no
evento) → 3. P1-4 (limpeza `channel_processed_messages`) → 4. P1-5 (tokens/custo) → 5. P2-2/P2-3.
**Defense-in-depth opcional (não bloqueia MVP):** P0-1 (auditoria — seção 6.A), P0-2 (AuthzChecker).
Itens de saída/entrega (P1-2 retry, P1-3 status callbacks) conforme necessidade operacional.

## 8. Verificação
- **Gates de regra (devem retornar vazio):** greps de `.claude/rules/go-adapters.md` e
  `.claude/rules/transactions-workflows.md`; grep de zero-comentários.
- **Unit:** `go test ./internal/agent/...` incluindo novos testes P2-3.
- **E2E:** `internal/agent/e2e/` (BDD) + `RUN_REAL_LLM=1 OPENROUTER_API_KEY=... go test ./internal/agent/e2e -run Recognition`.
- **Replay manual:** reenviar mesmo WAMID dentro/fora da janela de 5 min → observar `whatsapp_stale_webhook_total`
  e `OutcomeDuplicate`.
- **P0-3 (recomendado):** `go test ./internal/agent/application/sanitize/...`; enviar mensagem com CPF/cartão e
  confirmar masking no prompt; enviar texto gigante e confirmar cap.
- **Auditoria (só se P0-1 for implementado):** após uma intent de escrita, validar linha em `agent_decisions` com
  `trace_id`, `prompt_sha256`, `resulting_event_id` consistentes.

## 9. Riscos / suposições
- **P0-3 masking:** manter precisão alta (CPF formatado + 13–19 dígitos). Não mascarar 11 dígitos crus (telefone BR)
  nem valores curtos — evita falso positivo que quebraria a classificação do LLM. Validar com os testes da seção 6.B.
- **P0-1 (se feito):** `agent_decisions.redacted_response` deve nascer já redigido (reusar o Sanitizer de 6.B); nunca
  gravar PII crua só para auditar. Não compartilhar transação com módulo de domínio (fronteira).
- **Gate de confirmação:** fora de escopo — lançamentos são para relatório, sem movimentação real.
- **P0-2 (se feito):** `AuthzChecker` allow-all é andaime, não proteção; só fecha com regra real. Lembrar que o
  isolamento por `user_id` já é estrutural (ver achados verificados na seção 1) — P0-2 é defense-in-depth.

---

## 10. PROMPT DE IMPLEMENTAÇÃO (copiar/colar para iniciar a execução)

```
Implemente o hardening do fluxo agêntico do mecontrola descrito em
docs/plans/2026-06-22-agent-flow-hardening.md.

REGRAS OBRIGATÓRIAS (inegociáveis):
- Carregue .agents/skills/go-implementation/SKILL.md e execute Etapas 1–5 + Checklist R0–R7.
- Verifique a versão em go.mod antes de usar qualquer API nova.
- Zero comentários em .go de produção (exceções: //go:, //nolint: com justificativa, // Code generated).
- Respeite R-ADAPTER-001: fluxo adapter→usecase; sem SQL/branching de domínio em handler/consumer/producer/job.
- Para internal/transactions, respeite R-TXN-WORKFLOWS-001 (ADR-006): lógica de domínio só em Decide*.
- context.Context em toda fronteira de IO; interface no consumidor; sem init(); sem panic em produção;
  sem abstrair tempo (time.Now().UTC() inline); errors.Join/fmt.Errorf("ctx: %w", err).
- FRONTEIRA: agent_decisions é tabela do módulo agent; gravar pending/settled em transação PRÓPRIA do agent
  (uow.Do). NUNCA compartilhar transação com o módulo de domínio (não enlistar CreateTransactionUC et al.).

ESCOPO E ORDEM (recalibrado; uma PR contida por item; pare e valide entre itens):
1. P0-3 Sanitização/limite de input + masking PII ao OpenRouter — RECOMENDADO, único gap não-redundante.
   Seguir o design da seção 6.B (Sanitizer puro, regexp.Compile, masking conservador, wire em ParseInbound).
2. P1-1 trace_id no payload dos eventos de intent.
3. P1-4 limpeza/particionamento de channel_processed_messages.
4. P1-5 persistência de tokens + métrica agent_llm_tokens_total.
(Defense-in-depth OPCIONAL, não bloqueia MVP: P0-1 auditoria agent-owned — seção 6.A, SEM gate de confirmação;
 P0-2 AuthzChecker. Implementar só se houver decisão explícita de fazê-lo.)
NÃO IMPLEMENTAR: gate de confirmação financeira (lançamentos são para relatório, sem movimentação real).

PARALELIZAÇÃO: para itens que tocam múltiplas categorias (usecase/repo/handler/migration),
spawnar subagents por categoria conforme a preferência registrada do projeto.

VALIDAÇÃO POR ITEM (obrigatória, sem falso positivo):
- Rodar os greps de gate de .claude/rules/go-adapters.md e transactions-workflows.md (devem retornar vazio).
- go test ./internal/agent/... e os novos testes (isolamento, replay, falha parcial, auth-bypass).
- Reportar arquivos alterados, validações executadas, riscos residuais e suposições.
- NÃO marcar como concluído sem evidência de teste passando. Não expandir escopo além do item.
```

---

## 11. P2-1 — Histórico multi-turn ao LLM: NÃO-GOAL do MVP (decisão 2026-06-22)

**Decisão:** a conversa permanece **single-turn**. P2-1 NÃO será implementada no MVP. A razão NÃO é custo de tokens
(que é desprezível) — é **risco de determinismo/regressão, +1 escrita de DB por mensagem e complexidade vs. ganho
marginal** para um UX de comandos curtos.

### Estado atual (verificado)
- `interfaces.LLMRequest` (`llm_provider.go:27`) é single-message: só `SystemPrompt` + `UserMessage` (sem campo de
  histórico). O `buildRequestBody` do OpenRouter monta apenas `[system, user]`.
- `agent_sessions.recent_turns` existe no schema mas é **placeholder vazio**: gravado sempre como `[]`
  (`binding/budget_config.go:145,172`), nunca populado com trocas reais nem lido para prompt.
- 3 pontos chamam o LLM (todos single-turn): `parse_inbound`, `compose_conversational_reply`, `run_onboarding_turn`.

### Prós vs. Contras
- Prós: coerência conversacional (referências a turnos anteriores), menos repetição, melhor desambiguação na prosa.
- Contras: determinismo↓ (perde a previsibilidade do `temperature=0` single-turn); +1 escrita de DB por mensagem;
  PII precisa ser redigida a cada reenvio (reusar Sanitizer); regressão se aplicado a intent-parse (contamina
  classificação → lançamento duplicado) ou onboarding (quebra narração determinística); complexidade (contrato +
  provider + VO `ConversationTurn` + persistência + testes).

### Estimativa de custo (modelo primário `google/gemini-2.5-flash-lite`; preços aprox., verificar em openrouter.ai)
- Tokens: histórico de 3 turnos ≈ **+390 input/turno de conversa** (zero impacto no intent-parse). Preço aprox.
  $0.10/1M in, $0.40/1M out.
- Premissas: 200 msg/usuário/mês, 15% conversa (30 turnos/mês), janela 3.
- **Δ incremental P2-1 ≈ $0.0012/usuário/mês** (≈ um décimo de centavo). Total do bot ≈ $0.021→$0.022/usuário/mês.
- Escala: 1k usuários ≈ +$1.2/mês; 10k ≈ +$12/mês; 100k ≈ +$120/mês. (Fallback p/ `claude-haiku-4.5` ≈ 10× — raro.)

### Se um dia for reativada — escopo seguro obrigatório
Apenas em `compose_conversational_reply`; janela de 3 turnos; histórico **redigido** via Sanitizer; **NUNCA** em
`parse_inbound` nem onboarding. Modelar `ConversationTurn` como VO DMMF (role enum `iota+1`, content validado).
