# ADR-003 — Contrato `Subscription` como interface mínima em `internal/identity/domain/services`

## Metadados

- **Título:** Modelagem de `Subscription` como interface no domínio de identity para evitar import cíclico com billing
- **Data:** 2026-06-03
- **Status:** Aceita
- **Decisores:** Engenharia + autor do PRD
- **Relacionados:** PRD (F-04, RF-13, RF-14, FE-01), techspec §Design de Implementação, AGENTS.md "Cross-module"

## Contexto

`IsEntitled(sub, now) bool` é função pura central de entitlement e DEVE viver em `identity/domain` (RF-13). Mas o agregado `Subscription` completo é responsabilidade do Épico E2 em `internal/billing/domain` (FE-01). Se `identity/domain` declarasse uma struct concreta `Subscription`, billing teria duas escolhas igualmente ruins:

1. Importar `identity/domain.Subscription` para usar como tipo do seu agregado — viola fronteira de bounded context.
2. Definir seu próprio `billing.Subscription` e converter para `identity.Subscription` em todo ponto de uso — duplica modelo e cria drift.

`depguard` já proíbe `internal/finance/*` ↔ `internal/identity/*`, e regra equivalente deverá cobrir `billing` ↔ `identity` no PRD de E2.

## Decisão

Declarar `Subscription` em `identity/domain/services/subscription.go` como **interface** com três getters mínimos: `Status() SubscriptionStatus`, `CurrentPeriodEnd() time.Time`, `GracePeriodEnd() time.Time`. O enum `SubscriptionStatus` (iota+1, zero reservado a `Unknown`) também vive em `identity/domain/services`. O agregado concreto em billing implementa essa interface satisfazendo o duck typing do Go — sem import explícito do tipo, billing apenas precisa expor métodos com mesma assinatura. `IsEntitled` é método de struct `EntitlementChecker` (R1) que aceita `Subscription` por interface.

## Alternativas Consideradas

- **Struct concreta exportada com campos públicos** — Vantagens: mais simples para chamar. Desvantagens: billing tem que converter seu agregado para esse struct antes de cada `IsEntitled`, duplica forma. Rejeitada — gera código de adapter trivial repetido.
- **`IsEntitled(status, periodEnd, graceEnd, now time.Time) bool`** com 4 parâmetros primitivos — Vantagens: nenhum tipo `Subscription` no domain de identity. Desvantagens: assinatura cresce a cada novo campo necessário (cancel reason, trial extension), chamadas perdem contexto semântico, viola OC #3 (primitivos sem encapsulamento). Rejeitada.
- **Mover `IsEntitled` para `billing/domain`** — Vantagens: ownership do conceito fica no módulo dono de subscription. Desvantagens: identity perde o checker; consumidores cross-module passam a importar billing/domain, criando dependência inversa do que está descrito no PRD. Rejeitada.

## Consequências

### Benefícios Esperados

- Sem import cíclico entre identity e billing.
- Testes unitários de `IsEntitled` usam fake `Subscription` simples sem mock externo.
- Contrato mínimo evolui via versionamento explícito (`SubscriptionStatus` enum), nunca por mudança silenciosa de struct.

### Trade-offs e Custos

- Interface em domínio (apesar de pequena) é exceção ao padrão "tipos concretos por padrão" (`architecture.md`). Justificada por cross-module boundary.
- Chamadas pagam custo mínimo de dispatch indireto (irrelevante na prática).

### Riscos e Mitigações

- **Risco:** Billing implementa a interface com semântica divergente (ex.: `GracePeriodEnd` retorna `time.Time{}` em vez de `CurrentPeriodEnd + grace`).
- **Mitigação:** Contract test em `internal/billing/domain` que valida implementação contra cenários canônicos de `IsEntitled` (responsabilidade do PRD de E2).
- **Risco:** Adição de campo ao contrato quebra todas as implementações.
- **Mitigação:** Versionamento via nova interface (`SubscriptionV2`) com adapter em billing — postergado até demanda real.

## Plano de Implementação

1. Criar `internal/identity/domain/services/subscription.go` com interface + enum.
2. Criar `internal/identity/domain/services/entitlement.go` com `EntitlementChecker.IsEntitled`.
3. Testes em `entitlement_test.go` usam fake local `type fakeSubscription struct{...}` implementando a interface.
4. Em E2, billing fará `var _ services.Subscription = (*billing.Subscription)(nil)` para verificação de compatibilidade em compile-time.

## Monitoramento e Validação

- Cobertura 100% de `IsEntitled` nas 6 transições + `nil` + boundary.
- `golangci-lint` valida que `identity/domain/services` não importa nada além de stdlib.

## Impacto em Documentação e Operação

- `internal/identity/README.md` documenta o contrato esperado de quem implementa `Subscription`.
- PRD de E2 cita este ADR para o leitor entender por que `billing.Subscription` precisa expor 3 getters específicos.

## Revisão Futura

Revisitar se um terceiro módulo precisar implementar `Subscription` (ex.: free-tier sintético) — pode justificar quebra para `SubscriptionV2` com método adicional.
