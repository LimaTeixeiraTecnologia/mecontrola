# Discovery — Limite de cartão para alerta `card_limit_near`

> Data: 2026-06-16
> Status: **AGUARDANDO DECISÃO PO**
> Bloqueia: Gap #3 do MVP roadmap, alerta da imagem da jornada do produto ("Sua fatura no cartão X está em Y. Você já utilizou R$Z do limite.")

## Contexto

A imagem da jornada do produto MeControla define 3 alertas proativos:
1. Categoria ≥80% do limite
2. Meta ≥50% atingida
3. **Cartão ≥85% do limite usado**

Os dois primeiros estão implementados e disparando (módulo `internal/budgets`, job `ThresholdAlertsJob`). O terceiro tem:
- Kind enum `ThresholdAlertCardLimit` definido (`internal/budgets/domain/services/threshold_workflow.go:16`)
- Threshold ratio configurável (`BUDGETS_THRESHOLD_CARD_RATIO`, default 0.85)
- Payload de evento outbox pronto
- Migration `budget_alerts_sent` aceita `kind='card_limit_near'`

Mas **não dispara em produção** porque `buildSnapshots` no `EvaluateThresholdAlerts` não emite snapshots de cartão. Motivo descoberto:

## Root cause — gap de modelagem

`mecontrola.cards` (migration `000001_initial_baseline.up.sql:840`) tem as colunas:
```
id, user_id, name, nickname, closing_day, due_day, created_at, updated_at, deleted_at
```

**Não há `limit_cents`**. Sem o denominador, `usedRatio = totalSpentCents / limitCents` é incalculável.

Isto **NÃO é um gap de wiring**. É um gap de **modelagem de domínio** — o agregado `Card` (`internal/card/domain/entities/card.go`) nunca capturou o limite de crédito como atributo de negócio.

## Decisão necessária

### Opção A — `limit_cents` como atributo do agregado Card

**Quando faz sentido**: o limite raramente muda; histórico de mudança não tem valor de produto.

**Mudanças**:
- Migration `000005_cards_limit_cents.up.sql`:
  ```sql
  ALTER TABLE mecontrola.cards
    ADD COLUMN limit_cents BIGINT NOT NULL DEFAULT 0
    CHECK (limit_cents >= 0);
  ```
- VO `CardLimit` em `internal/card/domain/valueobjects/`
- Estender `entities.Card` + `entities.NewCardInput` + `entities.HydrateCard`
- Repo postgres: INSERT/UPDATE/SELECT com nova coluna
- DTOs HTTP `CreateCardInput`/`UpdateCardInput`/`CardOutput`
- Handlers: `Create/Update` validam, `List/GetInvoice` retornam
- IntentRouter PR-BR no QueryCard inclui "limite restante" se >0
- Test em 4 camadas (VO, entity, repo integração, handler)
- ThresholdAlertsJob: novo método `ListActiveCardsForThresholdScan` no repo + branch `buildCardSnapshots` no usecase

Custo: ~15-20 arquivos novos/alterados. 1 migration. ~3-4 dias de implementação + revisão.

### Opção B — Tabela `mecontrola.card_credit_limits` com histórico

**Quando faz sentido**: PO quer rastrear aumentos de limite (LTV/risco), ou produto vai oferecer "veja como seu limite cresceu".

**Mudanças**:
- Migration `000005_card_credit_limits.up.sql`:
  ```sql
  CREATE TABLE mecontrola.card_credit_limits (
    id UUID PRIMARY KEY,
    card_id UUID NOT NULL REFERENCES mecontrola.cards(id) ON DELETE CASCADE,
    limit_cents BIGINT NOT NULL CHECK (limit_cents >= 0),
    effective_from TIMESTAMPTZ NOT NULL,
    effective_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
  );
  CREATE INDEX idx_card_credit_limits_card_active
    ON mecontrola.card_credit_limits(card_id, effective_from DESC)
    WHERE effective_until IS NULL;
  ```
- Domínio adicional: `CardCreditLimit` agregado, smart constructor
- Usecase `SetCardLimit` que fecha o anterior e abre o novo
- Repo + handler + dto
- IntentRouter, ThresholdAlertsJob: idem opção A
- Test em 5 camadas

Custo: ~25 arquivos. 1 migration + tabela nova. ~5-6 dias.

## Recomendação técnica

**Opção A**. Motivos:
1. Para o telos do MVP (alerta proativo de cartão), histórico não agrega.
2. O usuário típico do produto não atualiza limite com frequência (talvez 1× por ano).
3. Se depois o produto pivotar para histórico, podemos migrar A→B com 1 migration que move dados.
4. Custo de B (5-6 dias) não justifica ganho para o MVP de R$ 29,90/mês com 0 clientes.

## Próximos passos (após decisão PO)

1. PO aprova Opção A ou B → criar PRD `prd-card-limit-cents`.
2. Skill `create-technical-specification` gera techspec.
3. Skill `create-tasks` quebra em incrementos.
4. Execute via skill `execute-task`.
5. Após `Card.limit_cents` exposto: agente que tinha sido bloqueado (Gap #3) destrava em ~2h (já tem usecase + workflow + migration prontos).

## O que **não deve** ser feito

- ❌ Inventar coluna `limit_cents` sem PRD aprovado (viola anti-alucinação AGENTS.md §"Contexto e Anti-Alucinacao" item 6).
- ❌ Hardcode de "limite = R$ 5000" no código para destravar o alerta.
- ❌ Fazer parse do nome do cartão (`"Nubank 5k"` → 5000) para inferir limite.

## Referências

- Discovery anterior: `docs/runbooks/2026-06-15-mvp-gap-analysis.md`
- Subagent investigation: task_a936ae002e6613ce7 (failed safely com `needs_input`)
- Workflow DMMF: `internal/budgets/domain/services/threshold_workflow.go`
- Job pronto: `internal/budgets/application/usecases/evaluate_threshold_alerts.go`
