# Scorecard de Alternativas

Escala: 1 = pior ou mais oneroso; 5 = melhor ou menos oneroso no contexto da decisão (consolidação Identity + Billing + Onboarding com restrição tempo+robustez).

| Alternativa | Complexidade | Tempo de entrega | Custo | Escalabilidade | Segurança | Confiabilidade | Observabilidade | Manutenibilidade | Risco operacional | Total | Observação |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Alternativa 1 - α Dossiê monolítico + roadmap único | 4 | 3 | 4 | 3 | 4 | 4 | 3 | 2 | 3 | 30 | Coerência alta, mas drift semântico (billing em finance) e bloqueio sequencial penalizam manutenibilidade |
| Alternativa 2 - β Dossiê modular + 3 épicos paralelizáveis com internal/billing/ separado | 3 | 4 | 4 | 4 | 5 | 5 | 4 | 5 | 4 | 38 | Fronteiras corretas, paralelismo após Identity, contratos imutáveis, alinhada com 2.3=tempo+robustez |
| Alternativa 3 - γ Dossiê thin + walking skeleton primeiro | 5 | 5 | 5 | 2 | 2 | 2 | 3 | 3 | 2 | 29 | Rápido para validar funil, mas viola 2.3=B (production-proof); abre mão de idempotência e auditoria |
| Alternativa 4 - δ Identity + módulo subscription unificado englobando billing + onboarding + entitlement | 2 | 3 | 4 | 4 | 4 | 5 | 4 | 3 | 3 | 32 | Coerência interna alta, mas vira god-module; depguard precisa de refinamento; testes ficam mais lentos |

## Justificativa por critério (Alt β, recomendada)

- **Complexidade (3):** três módulos com fronteiras claras; mais arquivos que α/δ, mas cada módulo é compreensível em isolamento.
- **Tempo de entrega (4):** Identity é bloqueador, mas Billing e Onboarding paralelizam; com 1 dev fica em ~4 semanas, com 2 devs em ~2-3 semanas.
- **Custo (4):** zero infra nova (Postgres + Redis + outbox já existentes); custo é só desenvolvimento.
- **Escalabilidade (4):** módulos independentes podem evoluir; `internal/billing/` pode ganhar provider adicional (Asaas) sem mexer em identity/onboarding.
- **Segurança (5):** PII confinada por módulo; mascaramento centralizado; soft delete obrigatório em users; webhook signature por adapter; sem cross-module promíscuo.
- **Confiabilidade (5):** outbox transacional já pronto; idempotência por `event_id`; reconciliação prevista; máquina de estados única.
- **Observabilidade (4):** métricas por módulo (`identity_user_created_total`, `billing_event_processed_total`, `onboarding_token_consumed_total`); trace por evento.
- **Manutenibilidade (5):** depguard impede import errado; PRD por módulo é curto; quando vier RBAC, plano família, ou Asaas, mexe só em 1 módulo.
- **Risco operacional (4):** runbook por módulo; rollback granular; ponto único de falha é só o webhook ingress (mitigado por outbox + reconciliação).

## Leitura do Resultado
- **Alternativa mais equilibrada:** β (38 pts).
- **Alternativa mais rápida:** γ (5 em tempo, mas perde em confiabilidade).
- **Alternativa mais segura:** β (5 em segurança e confiabilidade).
- **Alternativa mais barata:** γ (5 em custo de implementação, mas custo total incluindo manutenção é maior).
- **Alternativa com maior risco operacional:** γ (2 em risco operacional — walking skeleton sem idempotência).

## Refinamentos aceitos sobre β (Rodada 3)
- `internal/billing/` separado de `internal/finance/` (3.2=B).
- `EntitlementService` dividido: regra pura `IsEntitled` em `internal/identity/domain`; cache + I/O em `internal/billing/application` (3.3=D).
- Roadmap híbrido: Identity bloqueador → Billing + Onboarding paralelos → Reconciliação pós-MVP (3.4=D).
