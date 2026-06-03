# Outbox Benchmark Baseline

## Como reproduzir

```bash
task bench:outbox
```

O comando acima executa:
```
go test -bench=BenchmarkPublisher -benchmem -count=5 -run=^$ ./internal/infrastructure/outbox/...
```

e atualiza este arquivo com os resultados.

## Resultados de referência

**Data de execução:** 2026-06-02
**SHA do binário:** 09aa20f
**Máquina de referência:** Apple M1 Pro, darwin/arm64, Go 1.24
**Variância observada:** < 5% entre runs (critério: < 20%)

### Publisher.Publish (mock-fast Storage, sem I/O real)

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| BenchmarkPublisher_Publish_1Handler | ~24 500 | ~13 100 | 119 |
| BenchmarkPublisher_Publish_3Handlers | ~24 800 | ~13 300 | 121 |
| BenchmarkPublisher_Publish_5Handlers | ~25 000 | ~13 400 | 123 |

_Custo adicional por handler extra ≈ 200–500 ns/op (INSERT de delivery)._

### Dispatcher.DrainBacklog (mock-fast, 1 000 deliveries por iteração)

| Benchmark | ns/op | deliveries/s | B/op | allocs/op |
|---|---|---|---|---|
| BenchmarkDispatcher_DrainBacklog | ~16 600 000 | ~60 000 | ~5 700 000 | ~65 000 |

_Throughput sustentado: ≥ 60 000 deliveries/s (muito acima do mínimo de 100 deliveries/s — RF-36)._

## Interpretação

- O custo dominante do Publisher é a criação do span OTel + injeção de traceparent no carrier.
- O custo por handler adicional é marginal (< 500 ns) confirmando que o modelo +1 INSERT/handler é viável para até dezenas de subscriptions.
- O Dispatcher mock-fast mede overhead puro do loop de claim/dispatch sem latência de I/O; em produção, espera-se throughput limitado pelo Postgres (FOR UPDATE SKIP LOCKED).

## Próxima execução

Executar `task bench:outbox` após mudanças em `publisher.go`, `dispatcher.go` ou na política de backoff e comparar contra esta baseline.
