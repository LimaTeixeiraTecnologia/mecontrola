# Diagnóstico de Inchaço e Plano de Simplificação — `internal/agent`

## Contexto

O módulo `internal/agent` cresceu organicamente: cada sprint de intent novo adicionou um `route*()` no `DailyLedgerAgent`, mais funções de formatação de texto no `IntentRouter` e mais wiring no `module.go`. O resultado é correto e testado (109% test/prod ratio), mas custa caro para ler, navegar e manter.

O objetivo é reduzir ruído sem alterar comportamento — refatoração mecânica, testes existentes como prova de não-regressão.

Inspiração de design: Mastra usa registry de handlers tipado + separação de apresentação (text rendering) do dispatch. DMMF valida: funções puras (formatação) devem viver separadas de orquestração.

---

## Diagnóstico Honesto

### O que está INCHADO (mudar)

| Problema | Arquivo | Linhas | Impacto |
|----------|---------|--------|---------|
| 28 funções `format*()` e helpers de texto vivem no `IntentRouter` | `intent_router.go` | 674–1140 (~466 linhas) | Router de 1140 linhas; difícil navegar; funções não testadas isoladamente |
| `route()` é pass-through de 15 linhas (só chama `daily.Handle()`) | `intent_router.go` | 555–570 | Uma camada de indireção sem valor; aumenta stack trace |
| `DecisionAuditDeps` wraps apenas 2 campos usados em 1 lugar | `decision_audit.go` | 17–20 | Ceremony sem benefício; `newDecisionAuditor()` pode receber os 2 campos diretamente |
| `resolveCardByName`, `pickMostRecent`, `moreRecent`, `isTransientReadError` vivem no router | `intent_router.go` | 605–650 | Helpers não relacionados ao routing poluem o arquivo |
| `isWriteKind()` standalone function no `intent_router.go` | `intent_router.go` | 576–578 | Verifique se `kind.IsWrite()` já existe no domain; se sim, remover; se não, mover para `intent/` |

**Total estimado de redução**: 500–520 linhas em `intent_router.go` (de 1140 → ~620).

### O que NÃO está inchado (não tocar)

| Item | Por quê manter |
|------|---------------|
| Switch-case exaustivo em `Handle()` | Correto em Go; compiler-verified; `//nolint:revive` já documenta a escolha |
| 23 interfaces em `intent_router.go` | Contratos hexagonais corretos; permitem mock e inversão de dependência |
| 18 `route*()` methods em `DailyLedgerAgent` | Seguem padrão uniforme; não abstrair prematuramente — os métodos têm variações reais (categoryClarification, budget session, card lookup) |
| `Confidence` value object | DMMF: smart constructor com invariante real (0–1, `Below()`) |
| `ModelSlug` value object | Mesma razão |
| `PolicyEvaluator` domain service | Função pura; isolada; testável sem mock |
| `decisionAuditor` struct | Complexidade real: begin/settle/redact/lookup — não é ceremony |
| `decisionContext` value | Carrega estado transiente de forma segura (value receiver no settle) |

### Não há código morto real

- Todos os módulos são ativos
- Todas as interfaces têm implementação usada
- Imports `_` são legítimos (embed, pgx driver)
- Zero TODO/FIXME/XXX em `internal/agent`
- Transactions é condicional por flag (infra pronta, não é órfão)

---

## Plano de Implementação

### Refactor 1 — Extrair formatação para arquivo separado (MAIOR GANHO)

**Objetivo**: Mover as 28 funções `format*()` e helpers de texto do `intent_router.go` para `internal/agent/application/services/formatting.go` (mesmo package, arquivo separado).

**Arquivos**:
- Criar: `internal/agent/application/services/formatting.go`
- Modificar: `internal/agent/application/services/intent_router.go` (remover linhas 674–1140)

**O que mover** (28 funções puras, sem receiver):
```
formatBRL, formatThousands, formatPersistedExpense, formatPersistedIncome,
registerFailedText, formatMonthlySummary, formatCategoryAllocation,
rootSlugLabel, formatGoalUnavailable, formatGoalProgress, formatCardList,
formatCreatedCard, formatCardCount, createCardErrorText, formatCardNotFound,
formatCardInvoice, formatHowAmIDoing, formatCardPurchaseCardMissing,
formatPersistedCardPurchase, formatTransactionList, formatDeletedTransaction,
formatEditedTransaction, formatPersistedRecurring, formatRecurringList,
frequencyLabel, formatCategoryAmbiguous, formatCategoryNotFound,
matchesBudgetCancel (este último é predicate, vai junto)
```

**Também mover** (helpers de domínio sem receiver):
```
resolveCardByName, pickMostRecent, moreRecent, isTransientReadError
```

**Restrições**:
- Todas as funções permanecem no package `services` — sem mudança de import em nenhum chamador
- Zero alteração de assinatura
- R-ADAPTER-001.1: zero comentários no arquivo novo

**Verificação**:
```bash
go build ./internal/agent/...
go test -race -count=1 ./internal/agent/...
```

---

### Refactor 2 — Inline `route()` pass-through

**Objetivo**: Eliminar a camada `route()` (lines 555–570) que só delega para `daily.Handle()`.

**Arquivo**: `internal/agent/application/services/intent_router.go`

**Antes**:
```go
func (r *IntentRouter) RouteWhatsApp(...) RouteResult {
    // setup ...
    result := r.route(ctx, principal, "whatsapp", msg.Peer, msg.Text, msg.MessageID)
    // ...
}

func (r *IntentRouter) route(ctx context.Context, ...) RouteResult {
    if r.onboarding != nil { ... }
    return r.daily.Handle(ctx, ...)
}
```

**Depois**:
```go
func (r *IntentRouter) RouteWhatsApp(...) RouteResult {
    // setup ...
    if r.onboarding != nil { ... }
    result := r.daily.Handle(ctx, principal, "whatsapp", msg.Peer, msg.Text, msg.MessageID)
    // publishEvent ...
}
```

Duplicar o bloco de onboarding nos dois métodos (WhatsApp e Telegram) — são idênticos e pequenos (~10 linhas). Alternativa: extrair `r.dispatch(ctx, principal, channel, peer, text, messageID)` se a duplicação incomodar — mas inlining é mais direto.

**Verificação**: `go test -race -count=1 ./internal/agent/...`

---

### Refactor 3 — Remover `DecisionAuditDeps` wrapper

**Objetivo**: Passar `Factory` e `UoW` diretamente para `newDecisionAuditor()`.

**Arquivo**: `internal/agent/application/services/decision_audit.go`

**Antes**:
```go
type DecisionAuditDeps struct {
    Factory interfaces.AgentDecisionRepositoryFactory
    UoW     uow.UnitOfWork
}

func newDecisionAuditor(o11y observability.Observability, deps DecisionAuditDeps, redactor DecisionRedactor) *decisionAuditor {
```

**Depois**:
```go
func newDecisionAuditor(
    o11y     observability.Observability,
    factory  interfaces.AgentDecisionRepositoryFactory,
    uow      uow.UnitOfWork,
    redactor DecisionRedactor,
) *decisionAuditor {
```

Ajustar o único chamador em `daily_ledger_agent.go:259` (`beginDecisionAudit`).

**Verificação**: `go build ./internal/agent/... && go test ./internal/agent/...`

---

### Refactor 4 — Verificar `isWriteKind()` vs `kind.IsWrite()`

**Objetivo**: Se `intent.Kind` já tiver método `IsWrite()` no domínio, remover a função standalone do `intent_router.go`.

**Verificação prévia**:
```bash
grep -n "IsWrite" internal/agent/domain/intent/intent.go
```

Se existir: substituir `isWriteKind(kind)` por `kind.IsWrite()` e remover a função.
Se não existir: mover `isWriteKind()` para `internal/agent/domain/intent/intent.go` como método `(k Kind) IsWrite() bool`.

---

## Gates de Validação (pós-refactor)

```bash
# R-ADAPTER-001.1: zero comentários em produção
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "^[[:space:]]*//" internal/agent/ \
  | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL" || echo "OK"

# Build limpo
go build ./internal/agent/...

# Testes com race detector
go test -race -count=1 ./internal/agent/...

# Vet
go vet ./internal/agent/...
```

---

## O que NÃO fazer (armadilhas)

1. **Não criar `IntentHandler` registry** — a abstração é tentadora (Mastra usa), mas neste projeto os 18 handlers têm variações reais de assinatura. Uniformizar forçaria type assertions ou structs de request genéricas — mais complexidade, não menos.

2. **Não colar `PolicyEvaluator` inline** — é função pura de domínio, testável, no lugar certo.

3. **Não remover interfaces** — são contratos hexagonais; permitir mocks é o ponto.

4. **Não tocar `daily_ledger_agent.go`** exceto o ajuste de `DecisionAuditDeps` (Refactor 3). Os 18 `route*()` são verbosos mas uniformes; a alternativa (dispatch table) exigiria interface genérica que perde type-safety.

5. **Não criar package `formatting/`** — manter no mesmo package `services` evita export desnecessário e mudança de chamadores.

---

## Resultado Esperado

| Arquivo | Antes | Depois |
|---------|-------|--------|
| `intent_router.go` | 1140 linhas | ~620 linhas |
| `formatting.go` (novo) | — | ~520 linhas |
| `decision_audit.go` | 206 linhas | ~200 linhas |
| `daily_ledger_agent.go` | 827 linhas | ~825 linhas |
| Camadas msg→LLM | 6 | 5 |

Nenhuma mudança de comportamento. Todos os testes existentes validam a não-regressão.

## Skill obrigatória na execução

`go-implementation` — Etapas 1–5, referências: `architecture.md` (R0–R7), `build.md` (checklist).
