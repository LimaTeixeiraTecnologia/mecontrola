# ADR-006 — Janela de reanimação como constante de domínio (`ReanimationWindow`)

## Metadados

- **Título:** Parametrização da janela de reanimação por soft delete
- **Data:** 2026-06-05
- **Status:** Aceita
- **Decisores:** Time MeControla (owner: Jailton Junior)
- **Relacionados:**
  - PRD: [`prd.md`](./prd.md) — RF-08-ter, R-06, F-04
  - Tech Spec: [`techspec.md`](./techspec.md)
  - PRD risco fechado: **R-06**
  - Acoplado a: E4 (job de anonimização após 30 dias).
  - Regra Go: R6.7 (proibido `clock.Clock`; usar `time.Now().UTC()` inline).

## Contexto

O PRD (RF-08-ter) define que `UpsertByWhatsAppNumber` chamado contra um número soft-deletado decide o comportamento por janela temporal:

- `now - deleted_at <= 30 dias` → **reanima** a conta original (mesmo UUID, `deleted_at = NULL`, `status = ACTIVE`), zerando `email` e `display_name` antes da recoleta.
- `now - deleted_at > 30 dias` → cria conta nova com UUID novo.

A janela de 30 dias espelha a janela de anonimização que E4 implementará. R-06 reconhece o acoplamento: se E4 escolher janela diferente (e.g., 60d), o comportamento fica incoerente.

Restrições:

- **R6.7:** não pode existir `Clock` interface compartilhado nem `now func()` injetado. Tempo é dependência local (`time.Now().UTC()` inline ou passado por command object).
- A janela é regra de domínio (decisão sobre reanimar ou criar novo) — vive em `domain`.
- A janela é compartilhada entre `identity` (decisão de reanimar) e `billing/onboarding` (consumidores) — mas o **dono semântico é identity**, porque a decisão de reanimar é do agregado `User`.
- Mudar a janela exige migração coordenada (RF-08-ter explicita "alterações futuras exigem migration coordenada").

## Decisão

Declarar a janela como **constante exportada** em `internal/identity/domain/entities/user.go` (ou em arquivo dedicado `internal/identity/domain/policies.go` se preferido):

```go
package entities

import "time"

// ReanimationWindow é o intervalo máximo após soft delete em que
// UpsertByWhatsAppNumber reanima a conta original (mesmo UUID).
// Após esse intervalo, é criada conta nova.
//
// Acoplada à janela de anonimização programada em E4 (reconciliation-hardening).
// Mudanças exigem migration coordenada e ADR de revisão.
const ReanimationWindow = 30 * 24 * time.Hour
```

A função de decisão no agregado:

```go
// CanReanimate decide se um User soft-deletado pode ser reanimado em `now`.
// Regra: now - deleted_at <= ReanimationWindow.
func (u User) CanReanimate(now time.Time) bool {
    if u.deletedAt.IsZero() {
        return false // não está soft-deletado
    }
    return now.Sub(u.deletedAt) <= ReanimationWindow
}
```

`now` é passado pelo use case (R6.7 — sem clock global). O use case `UpsertByWhatsAppNumber` resolve `now := time.Now().UTC()` no seu próprio escopo.

## Alternativas Consideradas

### A) Constante em pacote `internal/platform/policies` compartilhado

- **Vantagens:** reuso direto por outros módulos.
- **Desvantagens:**
  - Plataforma não importa módulo de negócio nem vice-versa de forma cega; mas regra de negócio (reanimação) é do domínio identity.
  - Sai do dono semântico.
- **Motivo de não escolher:** quebra a fronteira de domínio. Plataforma é técnica, não comportamental.

### B) Valor injetado via `configs.Config`

- **Vantagens:** alterável em runtime sem recompilar.
- **Desvantagens:**
  - Regra de negócio dependente de config externa fica frágil — qualquer typo na env vira incoerência silenciosa entre `identity` e o job de E4.
  - Tempo de janela LGPD não é "configuração operacional"; é decisão de produto.
  - Exige propagar config no domain (viola pureza de `domain`).
- **Motivo de não escolher:** viola fronteira hexagonal.

### C) Valor injetado via construtor do use case

- **Vantagens:** testável sem mock de tempo.
- **Desvantagens:**
  - Duplica o valor em testes (cada `New<Usecase>(window: 30 * 24 * time.Hour, ...)`).
  - Risco de testes usarem valores que não refletem produção.
- **Motivo de não escolher:** custo de cerimônia sem ganho real; constante exportada já permite override em teste com `if testing { ... }` se necessário (e não é necessário).

### D) Lookup em tabela `policies` no banco

- **Vantagens:** mudança operacional sem deploy.
- **Desvantagens:**
  - Adiciona round-trip por chamada.
  - Decoupling excessivo para uma constante que muda raramente.
- **Motivo de não escolher:** overhead desproporcional.

## Consequências

### Benefícios Esperados

- **Decisão visível e versionada** — alteração aparece no diff.
- **Dono semântico claro** (identity); E4 importa a constante quando precisar:
  ```go
  import "github.com/.../internal/identity/domain/entities"
  // E4 usa: entities.ReanimationWindow
  ```
- **Testável trivialmente** com `now := deletedAt.Add(window + time.Second)`.
- **R6.7 respeitada** — sem clock global, sem injeção de `now func()`.

### Trade-offs e Custos

- Alteração exige recompilar e migração coordenada (intencional — é regra LGPD, não config).
- E4 (em módulo separado) tem que importar `internal/identity/domain/entities` para acessar a constante — acoplamento documental aceito.

### Riscos e Mitigações

- **Risco:** E4 implementa job de anonimização com janela hard-coded `30 * 24 * time.Hour` em vez de importar a constante.
  - **Mitigação:** PRD do E4 deve referenciar este ADR; revisão de PR pega.
- **Risco:** mudança de janela quebra coerência entre `CanReanimate` e job E4 já rodando.
  - **Mitigação:** mudança vira migration coordenada com ADR de revisão deste; rollback é trivial (reverter constante).
- **Risco:** testes não cobrem a borda exata (`window` vs `window + 1ns`).
  - **Mitigação:** CA-04(d) e (e) cobrem os dois lados da fronteira.

## Plano de Implementação

1. Declarar `ReanimationWindow` em `internal/identity/domain/entities/user.go` (ou arquivo dedicado).
2. Implementar método `CanReanimate(now time.Time) bool` no agregado `User`.
3. Use case `UpsertByWhatsAppNumber` resolve `now := time.Now().UTC()` no seu escopo e chama `CanReanimate(now)`.
4. Testes parametrizados cobrem:
   - `now - deletedAt == 0` → reanima
   - `now - deletedAt == ReanimationWindow` → reanima (borda inclusiva, conforme RF-08-ter `<= 30d`)
   - `now - deletedAt == ReanimationWindow + time.Nanosecond` → cria novo
   - `now - deletedAt > ReanimationWindow` → cria novo
5. PRD/techspec de E4 referenciam `entities.ReanimationWindow` ao implementar job de anonimização.

## Monitoramento e Validação

- **Validação imediata:** `go test ./internal/identity/domain/...`.
- **Validação cross-épica:** quando E4 chegar, código importa a constante (grep `entities.ReanimationWindow` mostra os dois usos coordenados).
- **Sinal de drift:** discrepância entre data de soft delete e comportamento de reanimação real em smoke E2E indica bug — log estruturado com `reanimated=true/false, days_since_delete=N`.

## Impacto em Documentação e Operação

- `internal/identity/doc.go` documenta `ReanimationWindow` e seu acoplamento com E4.
- PRD/techspec de E4 (futuro) cita este ADR.
- Runbook LGPD (E4) usa a mesma constante.

## Revisão Futura

- Revisitar quando o PRD de E4 fixar definitivamente a janela de anonimização (validar coerência).
- Revisitar se LGPD ou contratos de serviço exigirem janela diferente por região/produto (cenário hipotético; abrir ADR novo).
- Revisitar se "reanimação" virar funcionalidade administrativa (e.g., suporte forçando reanimar após janela) — exige mudança de design, não só de constante.
