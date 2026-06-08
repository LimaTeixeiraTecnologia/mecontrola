# ADR-001 — Schema `onboarding` dedicado + tabela `onboarding_tokens` com hash

## Metadados

- **Título:** Persistência do magic token em schema próprio com hash SHA-256
- **Data:** 2026-06-06
- **Status:** Aceita
- **Decisores:** PO (jailton), arquitetura (AI)
- **Relacionados:** `.specs/prd-onboarding-magic-token/techspec.md` §6.7, RF-01, RF-03, RF-07, RF-09, RF-11, RF-12; [ADR-002](./adr-002-magic-token-format-base64url-sha256.md)

## Contexto

O PRD exige um magic token opaco persistido com estado (`PENDING → PAID → CONSUMED`, mais `EXPIRED` por job), capturando dados do checkout (mobile, email, sale id), outreach timestamp e dados de consumo (user, mobile real, path). E1 e E2 já mantêm seus próprios schemas lógicos (`identity.*`, `billing.*`). Existem três opções de localização: schema dedicado novo, reuso da tabela `billing.subscriptions`, ou tabela nova no schema `billing`.

Reusar `billing.subscriptions` confundiria semântica (uma sub pode ter múltiplos tokens em cenário de clique duplo S-08) e violaria fronteira E2↔E3. Schema único `public` ou compartilhado com `billing` aumenta acoplamento e dificulta políticas futuras de RLS/privacidade.

## Decisão

Criar schema `onboarding` no mesmo Postgres com tabela `onboarding_tokens` contendo:

- `id UUID PK`
- `token_hash BYTEA NOT NULL UNIQUE` (SHA-256 raw, 32 bytes — token nunca em claro)
- `status TEXT NOT NULL CHECK (...)` (`PENDING|PAID|CONSUMED|EXPIRED`)
- `plan_id UUID NOT NULL`
- `expires_at`, `created_at`, `paid_at`, `consumed_at`, `outreach_sent_at`
- `customer_mobile_e164`, `customer_email`, `external_sale_id` (capturados do checkout via consumer outbox)
- `consumed_by_user_id`, `consumed_by_mobile_e164`, `activation_path` (capturados no consumo)
- `metadata JSONB NOT NULL DEFAULT '{}'::jsonb`

Índices parciais:
- `(status, expires_at) WHERE status IN ('PENDING','PAID')` para job de expiração.
- `(status, paid_at) WHERE status='PAID' AND outreach_sent_at IS NULL` para job de outreach (seek otimizado).
- `(customer_mobile_e164) WHERE status='PAID' AND outreach_sent_at IS NOT NULL` para fallback E.164.

## Alternativas Consideradas

1. **Adicionar colunas em `billing.subscriptions`.** Recusada — mistura responsabilidades, força 1:1 token↔sub que falha em clique duplo, fere fronteira E2↔E3.
2. **Tabela `onboarding_tokens` no schema `billing`.** Recusada — facilita acoplamento acidental por joins; perde isolamento de privacidade.
3. **Schema `public` (sem schema dedicado).** Recusada — repositório já usa schemas lógicos por módulo; quebra padrão.

## Consequências

### Benefícios
- Fronteira clara entre módulos.
- Índices dedicados altamente seletivos para jobs de alto volume.
- Token em claro nunca persistido (resiste a dump de DB).
- Facilita RLS futura por `consumed_by_user_id`.

### Trade-offs
- Migration extra (`CREATE SCHEMA`); operacional aceita.
- Joins cross-schema com `billing.subscriptions` ficam explícitos (intencional — força observação de fronteira).
- Hash SHA-256 raw em `BYTEA` exige conversão na borda (já é o caso em `crypto/sha256.Sum256`).

### Riscos e Mitigações
- **R:** Esquecer índice parcial → seq scan no job de outreach com volume alto. **M:** Test integração valida `EXPLAIN` na primeira tarefa.
- **R:** Migration falha por permissão Postgres. **M:** Validação em pre-prod com role de aplicação antes do release.

## Plano de Implementação
1. Migration `0009_create_onboarding_schema_and_tokens.up.sql` (+`.down.sql`).
2. Repository Postgres com `pgx/v5` + UoW manager (`devkit-go/pkg/database/uow`).
3. Test integração com testcontainers cobrindo CRUD + transições + índices.

## Monitoramento
Métricas de pool e consumo por query (já cobertas pelo `pool_stats_interval` do `manager`).
