---
epic_id: E1
slug: identity-foundation
title: Fundação do módulo Identity (User, VOs, IsEntitled, soft delete)
status: prd_done
blocked_by: []
blocks: [E2, E3]
source_bundle: .agents/skills/decision-brainstorming/discoveries/brainstorm-consolidacao-core/decision-brief.md
source_discoveries:
  - docs/discoveries/discovery-identity-entitlement.md
artifacts:
  prd: .specs/prd-identity-foundation/prd.md
  techspec: null
  tasks: null
next_skill: create-technical-specification
target_module: internal/identity/
---

# Épico E1 — Identity Foundation

## Bloqueio

**Este épico NÃO é bloqueado por nenhum outro.** É a raiz do roadmap. Os épicos E2 (`billing-pipeline`) e E3 (`onboarding-magic-token`) ficam aguardando este épico ter `status: implemented` antes de iniciar execução de tarefas.

## Contexto e motivação

O módulo `internal/identity/` existe apenas como placeholder `doc.go` que declara responsabilidades de "JWT/refresh, RBAC e audit de acesso" — em conflito direto com as decisões consolidadas no brainstorm `consolidacao-core` (canal WhatsApp é o autenticador; sem RBAC no MVP; `is_admin bool` resolve admin).

Sem esta fundação, qualquer feature de cobrança (E2) ou onboarding (E3) nasce sem `User` para vincular `Subscription`, sem `WhatsAppNumber` normalizado para deduplicar, e replica normalização de telefone em três módulos. Esta é a fatia bloqueadora do roadmap.

## Escopo incluído

- Agregado `User` em `internal/identity/domain` com PK UUID.
- Value Object `WhatsAppNumber` imutável (normalização E.164 BR encapsulada no construtor).
- Value Object `Email` imutável.
- Campo `is_admin bool` em `users` (sem RBAC, sem JWT, sem sessions).
- Soft delete obrigatório com `deleted_at`; filtragem `WHERE deleted_at IS NULL` em todas as queries.
- Tabela `user_whatsapp_history` para registrar mudanças de número (schema + método de repo).
- Port `UserRepository` em `internal/identity/application` + implementação Postgres em `internal/identity/infrastructure`.
- Função pura `IsEntitled(sub, now) bool` em `internal/identity/domain` cobrindo as 6 transições de status.
- Contrato mínimo `Subscription` (status + period_end + grace_period_end) declarado em `identity/domain` para consumo do `IsEntitled` sem cross-module.
- Eliminação de drift: atualizar `doc.go`, `README.md`, `AGENTS.md` do módulo `identity` removendo RBAC/JWT.
- Regras `depguard` em `.golangci.yml` enforçando fronteiras hexagonais.
- Mascaramento de PII em logs (telefone, email).

## Fora de escopo

- Implementação de `Subscription` completa, webhook Kiwify, `EntitlementService`, máquina de estados (vai para E2).
- Implementação de `SignupToken`, `/api/checkout-session`, handler `ATIVAR`, outreach (vai para E3).
- Caso de uso "trocar número de WhatsApp" via comando admin ou fluxo WhatsApp (só schema + método de repo).
- Runbook completo de exclusão LGPD e job de anonimização em 30 dias (só mecanismo soft delete).
- Painel administrativo web e magic link por email.
- Suporte multi-país no `WhatsAppNumber` (MVP é BR-only).
- Override administrativo de entitlement (`entitlement_overrides`).
- Métricas, alertas e dashboards (logs estruturados são suficientes para esta fatia).

## Restrições inegociáveis

- PK de `User` é UUID v4. Telefone muda; UUID não.
- `WhatsAppNumber` é VO imutável; APIs internas trafegam o VO, nunca `string`.
- Soft delete obrigatório; nunca hard delete.
- Sem RBAC, sem JWT, sem sessions.
- Regra "1 user = 1 subscription ativa" — qualquer demanda de família/equipe abre PRD novo + brainstorm.
- Função `IsEntitled` deve ser pura (sem I/O, sem cache, sem efeito colateral).
- `internal/identity/domain` não importa `application` ou `infrastructure` (enforçado por depguard).
- Mascaramento de PII em logs implementado em ponto único reutilizável.

## Critérios de aceite

- **CA-01:** Cobertura 100% em `NewWhatsAppNumber`, `NewEmail`, `IsEntitled` (verificado por `go test -cover`).
- **CA-02:** Lint `depguard` verde no CI sem violação em `internal/identity/`.
- **CA-03:** Grep por `JWT`, `RBAC`, `role` em `internal/identity/**/*.go` retorna apenas referências em testes ou comentários históricos explícitos.
- **CA-04:** Smoke E2E com Postgres real (testcontainers ou docker-compose) cobrindo: upsert por `whatsapp_number`, soft delete + invisibilidade, registro em histórico.
- **CA-05:** Build passa em `ai-spec check-spec-drift .specs/prd-identity-foundation/tasks.md` após techspec + tasks existirem.

## Dependências externas

Nenhuma. É a raiz do roadmap.

## Pré-requisitos não-técnicos

Nenhum.

## Próximos passos sugeridos

```bash
# PRD já existe em .specs/prd-identity-foundation/prd.md

# 1. Materializar techspec
ai-spec create-technical-specification
#  → decisão de migration tool (golang-migrate/goose/atlas)
#  → decisão de framework de testes integração (testcontainers/docker-compose)
#  → decisão de estratégia de mascaramento de PII
#  → ADRs derivados

# 2. Decompor em tarefas incrementais
ai-spec create-tasks

# 3. Executar
ai-spec execute-all-tasks
```

## Riscos residuais

- **R-01:** Decisão de migration tool é nova para o repositório; impacto baixo, mas pode atrasar a primeira tarefa de schema se não for definida na techspec.
- **R-02:** Testcontainers exige Docker no ambiente de CI; verificar antes de fechar techspec.

## Referências

- Bundle: `.agents/skills/decision-brainstorming/discoveries/brainstorm-consolidacao-core/decision-brief.md` (seção **Arquitetura Inegociável**, blocos A e B).
- PRD existente: `.specs/prd-identity-foundation/prd.md`.
- Discovery: `docs/discoveries/discovery-identity-entitlement.md`.
- Governança: `CLAUDE.md`, `AGENTS.md`, `.claude/rules/governance.md`.
