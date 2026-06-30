# Orchestration Report — Jornada de Ativação via WhatsApp
**Run:** 2026-06-30
**PRD:** `.specs/prd-ativacao-whatsapp/prd.md` (RF-01..RF-37, spec-version 2)
**Status final:** `done`

---

## Snapshot Inicial vs Final

| # | Título | Inicial | Final |
|---|--------|---------|-------|
| 1.0 | Normalização E.164 (`internal/platform/phone`) | pending | **done** |
| 2.0 | Migration aditiva (índice telefone, timestamps, throttle) | pending | **done** |
| 3.0 | Config e mensagens da jornada de ativação | pending | **done** |
| 4.0 | Domínio e portas (janela, query telefone, throttle, concorrência) | pending | **done** |
| 5.0 | Usecase `ActivateFromInbound` + DTO `Validate()` | pending | **done** |
| 6.0 | Consumers de ativação e boas-vindas + evento | pending | **done** |
| 7.0 | Dispatcher event-driven e wiring | pending | **done** |
| 8.0 | E-mail para `/ativar`, estado do token e timestamps server-side | pending | **done** |
| 9.0 | Beacon de jornada (`page_opened`/`whatsapp_opened`) | pending | **done** |
| 10.0 | Landing (sem Telegram + beacon) e E2E ponta a ponta | pending | **done** |

**Total:** 10 tarefas | **Done:** 10 | **Falhas:** 0 | **Puladas:** 0

---

## Waves Executadas

### Wave 1 — Fundação (paralela)
| Tarefa | Subagent | Tokens | Duração | Status |
|--------|----------|--------|---------|--------|
| 1.0 | task-executor | 70.711 | 223s | done |
| 2.0 | task-executor | 85.369 | 208s | done |
| 3.0 | task-executor | 117.549 | 308s | done |

### Wave 2 — Domínio/E-mail/Beacon (paralela)
| Tarefa | Subagent | Tokens | Duração | Status |
|--------|----------|--------|---------|--------|
| 4.0 | task-executor | 84.542 | 1038s | done |
| 8.0 | task-executor | 111.262 | 964s | done ¹ |
| 9.0 | task-executor | 84.524 | 1105s | done |

¹ `report_path` retornado como absoluto (violação F13 de contrato); evidência física verificada no path relativo, tasks.md marcado done — aceito com nota.

### Wave 3 — Usecase (sequencial)
| Tarefa | Subagent | Tokens | Duração | Status |
|--------|----------|--------|---------|--------|
| 5.0 | task-executor | 147.119 | 794s | done |

### Wave 4 — Consumers (sequencial)
| Tarefa | Subagent | Tokens | Duração | Status |
|--------|----------|--------|---------|--------|
| 6.0 | task-executor | 155.098 | 737s | done |

### Wave 5 — Dispatcher (sequencial)
| Tarefa | Subagent | Tokens | Duração | Status |
|--------|----------|--------|---------|--------|
| 7.0 | task-executor | 153.483 | 795s | done |

### Wave 6 — Landing + E2E (sequencial)
| Tarefa | Subagent | Tokens | Duração | Status |
|--------|----------|--------|---------|--------|
| 10.0 | task-executor | 128.226 | 1124s | done |

**Total de tokens:** ~1.037.883 | **Subagent kills:** 0 (soft-discard Claude in-process) | **Paralelismo:** nativo (Agent tool)

---

## Gates Finais — Todos PASS

| Gate | Resultado |
|------|-----------|
| Zero comentários em `.go` de produção (R-ADAPTER-001.1) | ✓ PASS |
| Sem SQL em adapters (R-ADAPTER-001.2) | ✓ PASS |
| Sem `switch intent.Kind` no primitivo de agent | ✓ PASS |
| Sem LLM no kernel (`internal/platform/workflow`) | ✓ PASS |
| Métricas sem `user_id`/`telefone`/`category_id` como label Prometheus | ✓ PASS |
| Sem Telegram no backend | ✓ PASS |
| `ai-spec check-spec-drift` | ✓ OK: sem drift |
| `go build ./...` | ✓ build limpo |
| Testes unitários `go test ./...` (excl. integração/e2e) | ✓ todos ok |

---

## Regras de Negócio — Implementadas

| # | Regra | Implementada em |
|---|-------|----------------|
| 1 | Webhook Kiwify → PAID (não ativa); ativação apenas na 1ª msg WA | 4.0, 5.0, 6.0 |
| 2 | Msg do usuário = "Ativar o meu plano"; `wa_me_url` com `?text=Ativar+o+meu+plano`; token nunca exposto | 8.0 |
| 3 | Correlação por telefone E.164 (fonte única `internal/platform/phone`) | 1.0, 4.0, 5.0 |
| 4 | Janela 24h a partir de `paidAt` nos dois caminhos | 4.0 |
| 5 | Idempotência: `UpdateMarkConsumed` checa `RowsAffected==0` → `AlreadyActive`; dedup WAMID | 4.0, 5.0 |
| 6 | Boas-vindas desacopladas: `WelcomeConsumer` idempotente em `onboarding.subscription_bound` | 6.0 |
| 7 | No-match: throttle durável + métrica + audit; nunca silenciar | 4.0, 5.0 |
| 8 | `onboarding.activation.attempted.v1` em `noUserEventAllowlist` | 5.0 |
| 9 | E-mail → `/ativar?token=...`; supressão em recompra | 8.0 |
| 10 | Sem Telegram em lugar nenhum | 7.0, 8.0, 10.0 |
| 11 | Timestamps server-side set-once-if-null; beacon POST `/tokens/{token}/opened` | 8.0, 9.0 |
| 12 | Jornada termina na boas-vindas; sem onboarding/agente financeiro | 6.0, 7.0 |

---

## Artefatos Criados

**Backend (mecontrola):**
- `internal/platform/phone/` — normalização E.164, `NormalizeBR`
- `migrations/000005_*.sql` — índice parcial `phone_e164`, 4 colunas de timestamps, tabela `activation_throttle`
- `internal/onboarding/infrastructure/config/` — `ActivationWindowHours`, `ActivationPageURL`, mensagens
- `internal/onboarding/domain/` — `IsActivationWindowOpen` puro, `FindActivableByMobile`, `UpdateMarkConsumed`, `NoMatchThrottle`
- `internal/onboarding/application/usecases/activate_from_inbound.go` + DTO
- `internal/onboarding/infrastructure/messaging/database/consumers/activation_attempt_consumer.go`
- `internal/onboarding/infrastructure/messaging/database/consumers/welcome_consumer.go`
- `internal/onboarding/infrastructure/http/server/handlers/` — beacon `POST /tokens/{token}/opened`
- `internal/onboarding/e2e/` — suite Godog com 6 cenários (jornada feliz, idempotência, throttle, janela expirada, regressão webhook)
- Dispatcher wired, ramo `ATIVAR` removido, wiring em `whatsapp_wiring.go`

**Landing (mecontrola-landingpage):**
- Botão Telegram removido; `telegram_deep_link` eliminado
- Beacon `page_opened`/`whatsapp_opened` integrado
- Playwright 220 testes verdes

---

## Notas Operacionais

- **Paralelismo Claude Code**: `Agent` tool in-process — sem kill nativo; soft-discard usado. Nenhum timeout estourado.
- **F13 (report_path absoluto)**: tarefa 8.0 retornou path absoluto no YAML; evidência física verificada em path relativo; aceito com nota.
- **Diagnostics gopls**: avisos de build tags (`//go:build integration`, `e2e`) em testes de integração são normais — não indicam erros de compilação. Build `./...` limpo.
- **Limitação conhecida (fora de escopo)**: após ativação, próxima mensagem roteia ao `weather-agent`; documentado na techspec.

---

## Próximos Passos

1. Executar testes de integração com Testcontainers: `go test -tags=integration ./internal/onboarding/...`
2. Executar suite E2E Godog: `go test -tags=e2e ./internal/onboarding/e2e/...`
3. Executar Playwright da landing: `cd mecontrola-landingpage && npx playwright test`
4. Deploy: backend primeiro (inclui beacon), landing depois.
5. Monitorar métricas `onboarding_activation_*` e throttle no Grafana após deploy.
