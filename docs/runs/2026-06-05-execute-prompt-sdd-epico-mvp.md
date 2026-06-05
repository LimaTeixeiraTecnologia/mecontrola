# Run — 2026-06-05 — Execute prompt SDD épico MVP

**Origem:** execução do prompt em `docs/prompts/prompt-sdd-epico-mvp.md`.

**Decisão do usuário (inegociável):** substituir entrega de SDD standalone pelo PRD via skill `create-prd`, com escopo E1 (`identity-foundation`).

## Contexto

O prompt original solicitava SDD completo para o próximo épico elegível do MVP. Após exploração paralela de `docs/discoveries/`, `docs/epics/`, `AGENTS.md`, `CLAUDE.md` e working tree (`internal/`, `cmd/`, `migrations/`), constatou-se:

1. **Roadmap** (`docs/epics/README.md`): E1 é raiz; bloqueia E2 e E3; E4 é pós-MVP.
2. **Inconsistência documental:** `docs/epics/epic-01-identity-foundation.md` declara `status: prd_done` e cita `.specs/prd-identity-foundation/prd.md`, mas `.specs/` está vazio. Por `AGENTS.md`, working tree prevalece → PRD precisa ser materializado.
3. **Working tree:** `internal/identity/`, `internal/billing/` são placeholders; `internal/platform/outbox`, `internal/platform/worker`, `internal/platform/events`, `internal/platform/httpclient` e bootstrap em `cmd/server` e `cmd/worker` já estão prontos para reuso.

O usuário, ao revisar a fase de decisão, redirecionou: **"não, eu quero iniciar o prd com a skill create-prd, inegociável e mandatório"** e em seguida removeu `is_admin` do escopo do User.

## Decisão de enquadramento

- **Épico alvo:** E1 — `identity-foundation` (`internal/identity/`).
- **Skill a invocar:** `create-prd`.
- **Caminho esperado do PRD:** `.specs/prd-identity-foundation/prd.md` (final decidido pela skill).
- **Foco:** primeira versão do MVP; E4 (`reconciliation-hardening`) fora.

## Escopo do PRD (E1)

### Incluído
- Agregado `User` com PK `UUID v4`, soft delete obrigatório (`deleted_at`), filtragem `WHERE deleted_at IS NULL` em todas as queries.
- Value Objects imutáveis:
  - `WhatsAppNumber` com normalização **E.164 BR-only** no construtor.
  - `Email` com validação básica.
- **Sem RBAC, sem JWT, sem sessions, sem `is_admin`** — override explícito do usuário; nenhum atributo de autorização no agregado nesta versão.
- Tabela `user_whatsapp_history` registrando mudanças de número (schema + método repo; trocar número não é fluxo MVP).
- Port `UserRepository` em `internal/identity/application` + impl Postgres em `internal/identity/infrastructure/repositories/postgres`.
- Função pura `IsEntitled(sub, now) bool` no domínio (sem I/O, sem cache, sem efeito colateral).
- Contrato mínimo `Subscription` em `identity/domain` (`status` + `period_end` + `grace_period_end`) para `IsEntitled` sem cross-module.
- Mascaramento de PII em logs (telefone, email) em ponto único reutilizável.
- Regras `depguard` em `.golangci.yml` enforçando fronteiras hexagonais.
- Atualizar `doc.go`/`README.md`/`AGENTS.md` removendo menções a RBAC/JWT.

### Fora de escopo
- Implementação `Subscription` completa, webhook Kiwify, `EntitlementService`, máquina de estados → E2.
- `SignupToken`, `/api/checkout-session`, handler `ATIVAR`, outreach → E3.
- Comando admin "trocar número de WhatsApp" (só schema + repo).
- Runbook LGPD exclusão + job anonimização 30 dias → E4.
- Painel admin web, magic link email → pós-MVP.
- Multi-país WhatsAppNumber (BR-only).
- Override administrativo `entitlement_overrides` → E4.
- Métricas/alertas/dashboards (logs estruturados suficientes).

### Critérios de aceite
- CA-01: cobertura 100% em `NewWhatsAppNumber`, `NewEmail`, `IsEntitled` (`go test -cover`).
- CA-02: lint `depguard` verde no CI sem violação em `internal/identity/`.
- CA-03: grep por `JWT`, `RBAC`, `role` em `internal/identity/**/*.go` retorna apenas comentários históricos explícitos.
- CA-04: smoke E2E com Postgres real (testcontainers/docker-compose): upsert por `whatsapp_number`, soft delete + invisibilidade, registro histórico.
- CA-05: build passa em `ai-spec check-spec-drift .specs/prd-identity-foundation/tasks.md` (na etapa subsequente).

### Restrições inegociáveis
- "PK de `User` é UUID v4. Telefone muda; UUID não."
- "`WhatsAppNumber` é VO imutável; APIs internas trafegam o VO, nunca `string`."
- "Soft delete obrigatório; nunca hard delete."
- "Sem RBAC, sem JWT, sem sessions, sem `is_admin`."
- "Regra '1 user = 1 subscription ativa'."
- "Função `IsEntitled` deve ser pura (sem I/O, sem cache, sem efeito colateral)."
- "`internal/identity/domain` não importa `application` ou `infrastructure` (enforçado por depguard)."
- "Mascaramento de PII em logs em ponto único reutilizável."

## Insumos para a skill `create-prd`

- `docs/epics/README.md`
- `docs/epics/epic-01-identity-foundation.md`
- `docs/discoveries/discovery-identity-entitlement.md`
- `AGENTS.md`
- `CLAUDE.md`
- `.claude/rules/governance.md`
- Working tree: `internal/identity/`, `internal/platform/outbox/`, `cmd/server/server.go`, `cmd/worker/worker.go`, `migrations/000001_outbox_events.up.sql`.

## Verificação pós-skill

1. Confirmar PRD criado em `.specs/prd-identity-foundation/prd.md` (ou caminho equivalente decidido pela skill).
2. Validar conteúdo: objetivo, escopo incluído/fora, restrições inegociáveis, critérios de aceite numerados, riscos, dependências.
3. Validar que `is_admin` **não aparece** em parte alguma do PRD.
4. Não disparar `create-technical-specification` nem `create-tasks` nesta rodada.
5. Não alterar código de aplicação.

## Não-objetivos

- Não implementar código.
- Não escrever SDD standalone (substituído por PRD via skill).
- Não decompor em tasks.
- Não avançar E2/E3/E4.
