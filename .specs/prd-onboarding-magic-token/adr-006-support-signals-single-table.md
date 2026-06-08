# ADR-006 — Tabela única `support_signals` para RF-12, RF-15 e RF-18

## Metadados

- **Título:** Persistência única para sinais operacionais de suporte
- **Data:** 2026-06-06
- **Status:** Aceita
- **Decisores:** PO (jailton), arquitetura (AI)
- **Relacionados:** `.specs/prd-onboarding-magic-token/techspec.md` §6.7, RF-12, RF-15, RF-18, S-12

## Contexto

O PRD exige três sinais estruturados consultáveis por suporte:
- **RF-12:** subscription órfã expirada (pagou e nunca ativou em 7d).
- **RF-15:** tentativa de reuso de token consumido por outro número.
- **RF-18:** pagamento aprovado sem token associado.

O PRD descreve cada um como "fila ou tabela consultável" e proíbe canal dedicado de alerta no MVP (S-12). Não existe ferramenta de suporte/ticketing integrada hoje.

Opções:
1. **Tabela única com `kind` + JSONB payload.**
2. **Três tabelas tipadas por kind.**
3. **Outbox event consumido por sistema externo (futuro).**
4. **Apenas logs estruturados + métricas.**

## Decisão

**Tabela única `onboarding.support_signals(id, kind, payload JSONB, occurred_at, resolved_at, resolved_by, notes)`** com CHECK `kind IN ('orphan_expired_subscription','paid_without_token','token_reuse_attempt')`.

Schema do `payload` JSONB por `kind` (convenção documentada nesta ADR):

```jsonc
// kind = 'orphan_expired_subscription'
{
  "external_sale_id": "<kiwify_sale_id>",
  "token_hash_prefix": "<8 hex>",
  "expired_at": "ISO8601",
  "has_paid_state": true,
  "customer_mobile_masked": "+55**********",
  "customer_email_masked": "f***@***.com"
}

// kind = 'paid_without_token'
{
  "external_sale_id": "<kiwify_sale_id>",
  "subscription_id": "<uuid>",
  "customer_mobile_masked": "+55**********",
  "customer_email_masked": "f***@***.com",
  "paid_at": "ISO8601"
}

// kind = 'token_reuse_attempt'
{
  "token_hash_prefix": "<8 hex>",
  "from_mobile_masked": "+55**********",
  "consumed_by_mobile_masked": "+55**********",
  "attempt_at": "ISO8601",
  "reason": "different_number"
}
```

Mascaramento via VOs `MaskedMobile`/`MaskedEmail` (E1).

Índice parcial `(kind, occurred_at) WHERE resolved_at IS NULL` para queries de "open signals".

Endpoint de leitura para suporte fica **fora do MVP** (E4). MVP entrega persistência + escrita; consulta é via `psql` no read replica.

## Alternativas Consideradas

1. **Três tabelas tipadas.** Recusada — multiplica migrations, repository code, e DTOs por kind. JSONB com convenção documentada paga o mesmo custo de manutenção com 1/3 do código.
2. **Outbox event `support.signal_emitted`.** Recusada — sem consumer (S-12 confirma que não há sistema externo); evento ficaria órfão na outbox. Padrão outbox é para integração assíncrona, não para persistência consultável.
3. **Apenas logs + métricas.** Recusada — PRD exige literalmente "fila ou tabela consultável"; logs rotacionam, perdem cliente. Métrica é agregada e não permite drill-down por sale_id.

## Consequências

### Benefícios
- Uma tabela, um repositório, um caminho de escrita.
- Schema JSONB evolui sem migration por kind.
- Consultas de suporte triviais (`WHERE kind = ? AND resolved_at IS NULL`).
- Pronto para crescer com novos kinds futuros (E4) sem reestruturação.

### Trade-offs
- Tipagem fraca de payload — exige disciplina de convenção documentada nesta ADR.
- Query por campo dentro de JSONB precisa de função/operator (`payload->>'external_sale_id'`); aceitável dado o volume esperado.

### Riscos e Mitigações
- **R:** Convenção de payload desviada em uso futuro. **M:** Test unitário valida shape do JSON na escrita (struct → MarshalJSON com schema verificado).
- **R:** Volume cresce e dificulta consulta. **M:** Índice parcial + housekeeping em E4 (resolved_at > 90d → archive table).
- **R:** PII em claro acidentalmente no payload. **M:** Repository aceita apenas VOs mascarados nos campos sensíveis; teste unitário garante mascaramento.

## Plano de Implementação
1. Migration `0010_create_support_signals.up.sql`.
2. `domain/entities/support_signal.go`, `domain/valueobjects/support_signal_kind.go`.
3. `application/interfaces/support_signal_repository.go` + impl Postgres.
4. Repository com método `Insert(ctx, signal)` apenas (sem update no MVP — `resolved_at` é manual via psql).
5. Test integração cobrindo cada kind + índice parcial.

## Monitoramento
- Métricas Prometheus correspondentes já listadas em techspec §9.2 (`onboarding_orphan_expired_total`, `onboarding_token_reuse_attempt_total`, `billing_paid_without_token_total`).
