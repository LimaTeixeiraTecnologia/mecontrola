# ADR-011 — `period_end` é fonte autoritativa do provider Kiwify

## Metadados

- **Título:** Origem do `period_end` em transições de Subscription
- **Data:** 2026-06-03
- **Status:** Aceita
- **Decisores:** Equipe de domínio + plataforma
- **Relacionados:** `prd-billing-pipeline/prd.md` (RF-17, RF-19, F-3), `techspec.md` §StateMachine, ADR-010

## Contexto

Em transições `→ACTIVE` (ativação inicial, renovação, reativação), o agregado precisa atualizar `period_start` e `period_end`. Duas fontes possíveis:

1. **Provider (Kiwify)** — campo `subscription.current_period_end` no payload do webhook. RF-30 do PRD declara extração deste campo no `payload_mapper.go`.
2. **Cálculo local** — `period_end = period_start + BillingPeriod.Length()` onde `Length()` é determinístico por `PlanCode` (30/90/365 dias).

Casos onde divergem:
- Kiwify aplica proração em upgrade (MONTHLY → ANNUAL) — `current_period_end` reflete o ajuste, cálculo local não.
- Cliente faz pagamento adiantado — Kiwify pode estender `current_period_end` além do nominal.
- Bug no provider — Kiwify retorna data errada, dependência cega propaga.

## Decisão

**Trust provider:** `Subscription.period_end` em transições é populado diretamente de `canonical.PeriodEnd` (extraído de `payload.subscription.current_period_end`). `BillingPeriod.Length()` é usado apenas:
- Para `period_start` quando o payload não informar (default: `now`).
- Como referência em `IsEntitled` para fallback se `period_end` for `time.Zero` (defesa em profundidade).

**Defesa contra divergência:** o `payload_mapper.go` valida sanidade do `period_end` contra cálculo local com tolerância de **±14 dias** (cobre proração mensal extrema):

```go
expected := canonical.PeriodStart.Add(BillingPeriod.Length(plan))
diff := canonical.PeriodEnd.Sub(expected)
if diff < -14*24*time.Hour || diff > 14*24*time.Hour {
    // métrica + log warn, não bloqueia
    metric.RecordPeriodDivergence(plan, diff)
    logger.WarnContext(ctx, "kiwify period_end fora da janela esperada", ...)
}
```

Divergência fora da janela emite métrica `billing_period_divergence_total{plan_code, sign}` e log warn — **não bloqueia** o processamento. Operação humana investiga.

## Alternativas Consideradas

### Compute local sempre (`newEnd = previousEnd + Length`)

- Vantagem: determinístico, independe de bugs do provider.
- Desvantagem: ignora proração, pagamento adiantado, ajustes manuais do operador no painel Kiwify. Reconciliação pega depois, mas janela de inconsistência é ruim para entitlement.
- Rejeitada por divergir da realidade comercial.

### Híbrido com hard validation (rejeita divergência > X dias)

- Vantagem: barrar bug crítico do provider.
- Desvantagem: bloqueia processamento de evento legítimo se Kiwify emitir período não-padrão; cliente fica em estado errado mais tempo.
- Rejeitada por escolher entre dois males.

### Sempre rebuild via `FetchSubscription` (call sincrono ao Kiwify)

- Vantagem: garante consistência com provider em todo evento.
- Desvantagem: latência alta no caminho do processor (~250ms p99); estoura rate limit Kiwify; defeats outbox.
- Rejeitada por overhead.

## Consequências

### Benefícios Esperados

- Alinhamento com realidade comercial do provider.
- Customer percebido como ativo durante exato período da Kiwify (sem divergência sub-cliente).
- Reconciliation tem o mesmo critério (canonical do provider) — converge naturalmente.

### Trade-offs e Custos

- Dependência da corretude de `subscription.current_period_end` no payload Kiwify.
- Bug do provider pode causar entitlement errado até detecção via métrica.

### Riscos e Mitigações

- **Risco:** Kiwify retorna `current_period_end = time.Zero` ou data passada. **Mitigação:** `payload_mapper.go` rejeita (`ErrPayloadDecode`) → DLQ → alerta operacional.
- **Risco:** drift silencioso (Kiwify e nosso modelo divergem dia a dia em ±1d por timezone). **Mitigação:** tolerância ±14d cobre; `billing_period_divergence_total{sign=positive|negative}` alerta padrão crescente.
- **Risco:** cliente paga adiantado mas evento de extensão chega depois de reconciliação. **Mitigação:** reconciliation publica evento sintético aplicando estado remoto, agregado atualiza naturalmente.

## Plano de Implementação

1. `payload_mapper.go` extrai `payload.subscription.current_period_start` e `current_period_end` para `canonical.PeriodStart` / `canonical.PeriodEnd`.
2. `StateMachine.Apply` passa `PeriodChange{NewStart: canonical.PeriodStart, NewEnd: canonical.PeriodEnd}` para `Subscription.applyTransition`.
3. Sanity check em mapper (warning, não erro).
4. Métrica `billing_period_divergence_total` registrada na divergência.
5. Teste unit: payload com período válido (within tolerance), com proração (+5d), com divergência crítica (+30d → log warn).

## Monitoramento e Validação

- Métrica `billing_period_divergence_total{plan_code, sign}`.
- Alerta em > 5% das ativações com divergência crítica em 24h.
- Span OTel `billing.payload.parse` inclui atributo `period_diff_days`.

## Impacto em Documentação e Operação

- AGENTS.md billing documenta política.
- Runbook: investigação de `period_divergence_total > threshold` orienta a comparar amostra de payloads.

## Revisão Futura

- Se divergência crítica > 0.1% em produção sustentado, evoluir para validação rejeitando (mover para "Híbrido hard").
