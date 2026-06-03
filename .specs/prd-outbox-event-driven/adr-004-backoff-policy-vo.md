# ADR-004 — `BackoffPolicy` como Value Object com `rand.Rand` injetável

## Metadados

- **Título:** Política de backoff exponencial com jitter encapsulada em VO testável
- **Data:** 2026-06-02
- **Status:** Aceita
- **Decisores:** Tech lead backend
- **Relacionados:** PRD `prd-outbox-event-driven` v4 (RF-12, D-03); techspec; ADR-003 (modelagem VO)

## Contexto

A política de retry do Outbox (RF-12) usa backoff exponencial com jitter:

```
delay = min(base * 2^attempt * (0.5 + rand[0,1)), cap)
nextRetryAt = now + delay
```

Defaults: `base=2s`, `cap=5min`, `attempts=15` (janela total ~46min, alinhada com US-11).

Há duas formas idiomáticas de expressar essa lógica em Go:

1. Função pura standalone `nextRetry(base, cap time.Duration, attempt int, now time.Time, rng *rand.Rand) time.Time`.
2. VO `BackoffPolicy` com método `NextRetryAt(attempt Attempt, now time.Time) time.Time`.

Regra R1 da `go-implementation` (`[HARD]`): "Toda função deve ser método de struct" exceto `main`, construtores `New*`, `TestXxx` e utilitários sem estado em `pkg/`. `nextRetry` não se encaixa nas exceções.

`math/rand.Rand` não é thread-safe; cada Dispatcher precisa ter sua própria instância ou usar `rand.Lock` (custo). Testes precisam de seed fixa para serem determinísticos.

## Decisão

Modelar `BackoffPolicy` como VO no pacote `outbox`:

```go
type BackoffPolicy struct {
    base time.Duration
    cap  time.Duration
    rng  *rand.Rand
}

func NewBackoffPolicy(base, cap time.Duration, rng *rand.Rand) (BackoffPolicy, error) {
    if base <= 0 || cap <= 0 || base > cap {
        return BackoffPolicy{}, fmt.Errorf("outbox: backoff base=%s e cap=%s invalidos", base, cap)
    }
    if rng == nil {
        rng = rand.New(rand.NewSource(time.Now().UnixNano()))
    }
    return BackoffPolicy{base: base, cap: cap, rng: rng}, nil
}

func (p BackoffPolicy) NextRetryAt(attempt Attempt, now time.Time) time.Time {
    factor := math.Pow(2, float64(attempt))
    jitter := 0.5 + p.rng.Float64()
    delay := time.Duration(float64(p.base) * factor * jitter)
    if delay > p.cap { delay = p.cap }
    return now.Add(delay)
}
```

Construído uma vez no `Subsystem.Start`, injetado no `Dispatcher`. **Cada Dispatcher tem sua própria `BackoffPolicy`** — não compartilhar entre goroutines.

## Alternativas Consideradas

- **Função pura standalone**: viola R1 da `go-implementation`. Recusada.
- **Variável global `var DefaultBackoff = BackoffPolicy{...}`**: viola R6.6 (zero estado global). Recusada.
- **Construtor `BackoffPolicy` sem `rng` injetável (usa `rand.Float64` global)**: `math/rand` global em Go 1.20+ é thread-safe mas determinístico-em-teste fica difícil; pior, em testes paralelos `t.Parallel()` o seed global vaza. Recusada.
- **`BackoffPolicy` com `rng` injetado via `crypto/rand`**: overkill — jitter de backoff não é segurança; performance pior.

## Consequências

**Benefícios**:
- Testes determinísticos via `rand.New(rand.NewSource(seed))` injetado.
- Conformidade com R1 (`go-implementation`) e R-DDD-001.
- Política trocável no futuro sem mudar `Dispatcher` (ex.: `LinearBackoffPolicy` para evolução).
- Construção valida `base > 0`, `cap > 0`, `base ≤ cap` fail-fast no boot.

**Custos**:
- 1 arquivo `backoff_policy.go` + `backoff_policy_test.go` adicional.
- `rand.Rand` não-thread-safe exige instância por Dispatcher — documentado em godoc.

**Riscos / Mitigações**:
- **Reutilização inadvertida entre goroutines**: documentado no godoc; revisão de PR detecta.
- **Mudança futura para `rand/v2` (Go 1.22+)**: `BackoffPolicy` aceita interface `Float64()` no V2 — refactor local.
- **`math.Pow` em float gera imprecisão para attempt grande**: `attempt=15` → `2^15=32768`, dentro do range de `float64` sem perda. Cap protege antes de explodir.

## Plano de Implementação

`backoff_policy.go` no pacote `outbox`. Construção em `subsystem.go`. Teste com seed fixa cobre: attempt=0 (~base), attempt=15 (cap), erro de construção com bases inválidas.

## Monitoramento e Validação

- `outbox.deliveries.failed.total` por subscription mostra distribuição de retries.
- Teste verifica `NextRetryAt(15, t0)` retorna `t0 + cap` ± 50% jitter dentro do range esperado.
- Race detector (`go test -race`) cobre uso single-threaded por Dispatcher.

## Revisão Futura

Migrar para `math/rand/v2` quando Go 1.22+ for o mínimo do projeto (já é Go 1.26.3). Avaliar se faz sentido permitir política configurável por subscription (FE-08 deixa fora-de-escopo no MVP).
