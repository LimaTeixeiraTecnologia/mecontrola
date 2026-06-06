# ADR-007 — Plans em tabela seedada via migration (não hardcoded)

## Metadados

- **Título:** Plans em `billing_plans` seedados via migration
- **Data:** 2026-06-05
- **Status:** Aceita
- **Decisores:** PO (jailton), arquitetura (AI)
- **Relacionados:** `.specs/prd-billing-pipeline/techspec.md` §6.7, RF-02

## Contexto

PRD RF-02 define 3 planos: Mensal (30d), Trimestral (90d), Anual (365d). Cada plano tem um `product_id` correspondente na Kiwify. O `compra_aprovada` carrega `product.id`; o sistema precisa traduzir para o código interno e a duração para calcular `period_end`.

Três localizações possíveis: tabela DB, código Go, env config.

## Decisão

Tabela `billing_plans(kiwify_product_id PK, code TEXT UNIQUE CHECK IN ('MONTHLY','QUARTERLY','ANNUAL'), duration_days INT CHECK > 0, currency TEXT DEFAULT 'BRL')` criada e seedada na migration `0004_create_billing_plans.up.sql`. Use case faz lookup via `PlanRepository.GetByKiwifyProductID`. Ausência → `ErrPlanNotFound` (422).

Mudança nos IDs reais Kiwify (rara) → nova migration (`0010_update_billing_plans.up.sql`); sem redeploy de código.

## Alternativas Consideradas

1. **Mapa hardcoded em `internal/billing/domain/plans.go`.** Recusada — mudança exige redeploy. Aceitável no curtíssimo prazo (MVP estável com 3 planos), mas trade-off pequeno para favorecer flexibilidade operacional.
2. **Mapa em `configs.BillingConfig` (env-driven).** Recusada — dispersa source-of-truth entre server e worker; introduz risco de divergência (env desalinhado entre instâncias).
3. **Tabela sem seed (provisionamento manual).** Recusada — primeiro deploy não funcionaria; risco operacional gratuito.

## Consequências

### Benefícios Esperados

- Mapping versionado em migration (auditável em git).
- Mudança de `product_id` sem redeploy.
- Validação em DB (CHECK constraints) bloqueia código `code` inválido e `duration_days <= 0`.

### Trade-offs e Custos

- +1 tabela. Trivial.
- IDs reais Kiwify precisam ser conhecidos no momento da migration. **Mitigação:** placeholder `'<id-mensal>'`/`'<id-trimestral>'`/`'<id-anual>'` na migration de exemplo na techspec; a equipe substitui pelos IDs reais antes do deploy. Sem CI deploy, isso é seguro.

### Riscos e Mitigações

- **R:** Migration roda com placeholders genéricos em produção. **M:** Migração de seed marcada como obrigatória de revisão pré-deploy; PR review checklist menciona explicitamente a substituição dos IDs.
- **R:** Plano adicional no futuro requer ALTER. **M:** CHECK em `code` precisa ser alterado por nova migration; aceitável (planos novos são evento raro).

## Plano de Implementação

1. `migrations/0004_create_billing_plans.up.sql` + `.down.sql`.
2. `internal/billing/application/interfaces/plan_repository.go` (interface).
3. `internal/billing/infrastructure/repositories/postgres/plan_repository.go` (implementação).
4. Substituição dos IDs reais nos placeholders antes do primeiro deploy (revisão de PR).
5. Teste unit + integ.

## Monitoramento e Validação

- Métrica `billing_plan_lookup_failures_total{kiwify_product_id}` — detecta product_id desconhecido (configuração ausente ou plano novo não cadastrado).
- Alerta: > 0 sustained por 10min em prod.

## Impacto em Documentação e Operação

- README operacional: documentar como adicionar plano novo (migration ALTER + INSERT).

## Revisão Futura

- Reabrir se a base de planos crescer (10+) e o mapping precisar de admin UI.
- Reabrir se a Kiwify expor planos via API (poderia ser cache automático).
