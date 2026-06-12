# Transactions Workflows — Regra de Dominio DMMF

- Rule ID: R-TXN-WORKFLOWS-001
- Severidade: hard
- Escopo: `internal/transactions/domain/services/`, `internal/transactions/application/usecases/`
- ADR de origem: ADR-006

## Objetivo

Codificar o gate de revisao "regra de dominio fora de `Decide*` bloqueia PR" para o modulo
`internal/transactions`, preservando a integridade do padrao DMMF seletivo.

## Os 5 Workflows e seu `Decide*` Obrigatorio

Todo calculo de negocio, validacao de invariante e construcao de domain event para os fluxos
abaixo DEVE ocorrer exclusivamente dentro da funcao `Decide*` correspondente:

| Workflow | Funcao `Decide*` | Arquivo |
|----------|-----------------|---------|
| Criar/Atualizar Transaction | `DecideCreate`, `DecideUpdate` | `domain/services/transaction_workflow.go` |
| Criar/Atualizar CardPurchase (incluindo parcelas) | `DecideCreate`, `DecideUpdate` | `domain/services/card_purchase_workflow.go` |
| Materializar Recorrencia | `DecideMaterializeForDay` | `domain/services/recurring_workflow.go` |
| Deletar Transaction | logica de soft-delete via `DecideUpdate` com `deleted_at` | `domain/services/transaction_workflow.go` |
| Deletar CardPurchase | logica de soft-delete via `DecideUpdate` | `domain/services/card_purchase_workflow.go` |

## Regras Hard (DMMF)

### R-TXN-001 — `Decide*` e puro e obrigatorio [HARD]

Funcoes `Decide*` DEVEM ser:
- Puras: sem efeitos colaterais, sem IO, sem chamada de repositorio ou servico externo.
- Deterministicas: dado o mesmo input, produzem o mesmo output.
- Testadas unitariamente sem mock algum.

Proibido dentro de `Decide*`:
- Chamadas de repositorio (`ctx`, `db`).
- Geracao de IDs aleatorios (receber `ids []uuid.UUID` como parametro e consumir em ordem).
- Acesso a `time.Now()` (receber `now time.Time` como parametro).
- Logging ou instrumentacao.

### R-TXN-002 — Validacao fora de smart constructors e proibida [HARD]

Validacao de invariante de dominio (ex: `amount_cents > 0`, `installments in 1..24`,
`direction in {income, outcome}`) DEVE ocorrer exclusivamente em smart constructors dos
value objects e commands (`domain/valueobjects/`, `domain/commands/`).

Proibido:
- `if input.Amount <= 0 { return error }` em use case.
- Validacao de campo em handler HTTP.
- Validacao duplicada fora do construtor do VO/command.

### R-TXN-003 — Producers so mapeiam domain event para envelope [HARD]

Producers em `infrastructure/messaging/database/producers/` DEVEM apenas:
1. Receber domain event tipado (`entities.TransactionCreated`, etc.) como parametro.
2. Serializar para JSON.
3. Construir `outbox.Envelope` com campos pre-calculados pelo `Decide*`.
4. Publicar via `outbox.Publisher`.

Proibido em producers:
- Calcular `ref_months_affected`, `event_id`, `aggregate_*`, `occurred_at`.
- Decidir tipo de evento com base em estado da entidade.
- Branching sobre campos do agregado.

### R-TXN-004 — Cardinalidade controlada em metricas [HARD]

Nenhum label de metrica Prometheus no modulo transactions pode carregar `user_id` ou
`category_id`. Labels permitidos: `direction`, `payment_method`, `installments_bucket`,
`frequency`, `reason`, `operation`, `kind`.

Metrica `transactions_idempotency_replay_total` usa label `operation` (ex: `create_transaction`,
`create_card_purchase`); nunca usar `user_id` como label.

## Gate de Revisao — ADR-006

O seguinte check DEVE ser executado em toda PR que toque `internal/transactions/`:

**Regra de dominio fora de `Decide*` bloqueia PR:**

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "amount_cents\|direction\|installments\|payment_method\|day_of_month" \
  internal/transactions/application/usecases/ \
  internal/transactions/infrastructure/http/server/handlers/ \
  | grep -v "Decide\|command\|input\." \
  && echo "FAIL: logica de dominio fora de Decide* detectada" && exit 1 \
  || true
```

**Producers sem calculo de dominio:**

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "ref_months_affected\|EventID\|AggregateID\|occurred_at" \
  internal/transactions/infrastructure/messaging/database/producers/ \
  | grep -v "evt\.\|envelope\.\|outbox\." \
  && echo "FAIL: calculo de dominio em producer" && exit 1 \
  || true
```

## Revisao Futura — ADR-006

**Criterio de revisao anual obrigatorio:**

Esta regra deve ser revisada quando qualquer das seguintes condicoes ocorrer:
- Novo workflow de dominio adicionado ao modulo `transactions` (ex: estorno, transferencia).
- Mudanca no padrao `Decide*` adotado em outro modulo que se torne referencia do projeto.
- Evidencia de que a purezas de `Decide*` impede refatoracao necessaria (documentar caso em ADR-006 addendum).

Data proxima de revisao: 2027-06-12. Responsavel: time de plataforma.

## Referencias

- `ADR-006` em `.specs/prd-transactions-monthly/techspec.md`
- `domain-modeling.md` em `.agents/skills/agent-governance/references/`
- `.claude/rules/governance.md` (precedencia: `domain-modeling.md` prevalece sobre Uber para tipo e estado)
- Runbook: `docs/runbooks/transactions.md`
- Alertas: `docs/alerts/transactions.yaml`
- Dashboard: `docs/dashboards/transactions-overview.json`
