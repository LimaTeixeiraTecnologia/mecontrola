# ADR-001 — `Reason` como `type Reason string` com constantes nomeadas

## Metadados

- **Título:** Representação do motivo retornado por `IsEntitled`
- **Data:** 2026-06-05
- **Status:** Aceita
- **Decisores:** Time MeControla (owner: Jailton Junior)
- **Relacionados:**
  - PRD: [`prd.md`](./prd.md) — RF-12, F-08
  - Tech Spec: [`techspec.md`](./techspec.md)
  - PRD Q em aberto fechada: **Q-06**

## Contexto

`IsEntitled(sub Subscription, now time.Time) (bool, Reason)` é a única função de decisão de entitlement do produto (RF-12). O segundo retorno (`Reason`) será consumido em três caminhos distintos:

1. **`EntitlementService` em E2** — populará `Decision.Reason` no cache de entitlement.
2. **Handlers de WhatsApp em E3** — gerarão copy de bloqueio sem replicar a regra de decisão (espelha `handler.copyForBlocked` na discovery).
3. **Telemetria/logs** — toda decisão negativa será logada com motivo estruturado, sem PII.

Restrições relevantes:

- O cache de entitlement em E2 é serializado em JSON; o valor de `Reason` precisa ser legível e estável entre versões.
- `AGENTS.md` proíbe `init()` (R0), exige métodos de struct em vez de funções soltas (R1) e impõe `errors.Join`/`fmt.Errorf` para erros (R5.10/R7.6), mas não restringe a forma do `Reason`.
- O PRD enumera 8 valores fixos para `Reason`: `no_subscription | active | trialing | canceled_pending | past_due_grace | expired | refunded | past_due_no_grace`.
- O working tree não tem precedente de enum em `internal/identity` — a única referência em `internal/platform/outbox` usa `SMALLINT` para `status` na tabela, mas com tipo Go interno (`type Status int`).

## Decisão

`Reason` será declarado em `internal/identity/domain/entitlement.go` como:

```go
type Reason string

const (
    ReasonNoSubscription   Reason = "no_subscription"
    ReasonActive           Reason = "active"
    ReasonTrialing         Reason = "trialing"
    ReasonCanceledPending  Reason = "canceled_pending"
    ReasonPastDueGrace     Reason = "past_due_grace"
    ReasonExpired          Reason = "expired"
    ReasonRefunded         Reason = "refunded"
    ReasonPastDueNoGrace   Reason = "past_due_no_grace"
)
```

Cada constante é o literal estável usado em JSON, logs estruturados e copy via lookup. `IsEntitled` retorna sempre uma constante nomeada — nunca string crua.

## Alternativas Consideradas

### A) `type Reason int` com `String()` e `iota + 1`

- **Vantagens:** zero-value `0` permanece reservado (alinhado com R5.8), payload menor em wire.
- **Desvantagens:**
  - Cache JSON em E2 fica ilegível (`{"reason": 3}`) — exige mapeamento extra no consumidor.
  - Logs estruturados ficam dependentes do `String()` em todo sink.
  - Mudança de ordem das constantes quebra dados persistidos silenciosamente.
- **Motivo de não escolher:** custo de interop alto para benefício marginal de payload.

### B) Sealed iota com tipo opaco e funções de teste

- **Vantagens:** segurança de tipo máxima; impossível criar `Reason` inválida.
- **Desvantagens:** mecanismo idiomático em Go (`type Reason struct{ v int }`) é mais verboso, exige expor `String()` e factories para cada motivo, e não traz ganho real sobre `type Reason string` quando os valores são estáveis e fechados.
- **Motivo de não escolher:** complexidade desproporcional ao problema.

### C) `string` cru sem tipo nomeado

- **Vantagens:** menor cerimônia.
- **Desvantagens:** perde a documentação implícita do tipo e abre porta para typos sem o linter pegar.
- **Motivo de não escolher:** viola economia de tipos do Go e dificulta refator futuro.

## Consequências

### Benefícios Esperados

- **Interop direto com JSON** em E2 (`Decision.Reason` é `string` no cache).
- **Logs auto-documentados:** `observability.String("reason", string(reason))` produz `"reason":"past_due_grace"`.
- **Lookup de copy em E3** vira `map[Reason]string` direto.
- **Diff-friendly:** novos motivos são adições puras, não exigem renumeração.

### Trade-offs e Custos

- Payload ligeiramente maior em wire (irrelevante em volume de entitlement).
- Possibilidade teórica de criar `Reason("foo")` fora das constantes — mitigada por `golangci-lint` (regra `exhaustive` cobre `switch` sobre tipos string com constantes nomeadas via configuração específica) e por testes parametrizados.

### Riscos e Mitigações

- **Risco:** divergência entre constantes de domínio e copy/cache de E2.
  - **Mitigação:** E2 importa as constantes diretamente do pacote `domain` de identity; sem replicação de literais.
- **Risco:** novo motivo adicionado sem atualizar testes de `IsEntitled`.
  - **Mitigação:** CA-01 exige cobertura 100% e o `switch` no corpo de `IsEntitled` é exhaustive testável.

## Plano de Implementação

1. Declarar `type Reason string` e as 8 constantes em `internal/identity/domain/entitlement.go`.
2. `IsEntitled` retorna `(bool, Reason)` usando apenas as constantes.
3. Testes parametrizados em `internal/identity/domain/entitlement_test.go` cobrindo as 11 transições da RF-12 + caso `sub == nil`.
4. E2 importa o pacote `domain` e usa as constantes diretamente no cache.

## Monitoramento e Validação

- **Validação imediata:** `go test -cover ./internal/identity/domain/...` deve mostrar 100%.
- **Validação em E2:** `EntitlementService.Decision.Reason` armazena exatamente o literal da constante.
- **Sinal de drift:** qualquer log com `"reason":"<valor não previsto>"` indica bypass do tipo — investigar.

## Impacto em Documentação e Operação

- `internal/identity/doc.go` documenta a lista de `Reason` válida.
- Tabela de copy em E3 (futura) cita ADR-001 como fonte.
- Runbook LGPD não é afetado: `Reason` não é PII.

## Revisão Futura

- Revisitar quando o E2 implementar `EntitlementService` real (validar interop).
- Revisitar se surgir necessidade de localizar mensagens (`Reason` continua estável; localização vira camada de apresentação separada).
- Revisitar se o número de motivos passar de ~15 (avaliar enum tipado).
