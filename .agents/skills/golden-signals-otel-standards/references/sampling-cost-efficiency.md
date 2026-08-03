# Sampling, Cardinalidade e Custo

Como controlar volume, custo e robustez da telemetria sem perder sinal. Foco em eficiência e economia por porte.

**Fontes:** OTel Specification (sampling), OTLP (backpressure/retry), Semantic Conventions (cardinalidade).

## Princípio: onde amostrar

- **Métricas**: são **agregadas** e baratas por natureza. **Não** aplicar sampling a métricas — perder amostras distorce taxas e percentis. O custo de métricas cresce com **cardinalidade**, não com volume de requisições.
- **Traces**: são o item de maior volume e custo. É onde o sampling se aplica.
- **Logs**: controlar por nível e por correlação (manter logs de erros e de traces amostrados).

## Head sampling vs. tail sampling

| Estratégia | Onde | Quando | Trade-off |
| --- | --- | --- | --- |
| **Head sampling** | Na aplicação (SDK) ou agent | Decide no início do trace, por probabilidade | Barato e simples; pode descartar traces de erro raros |
| **Tail sampling** | Gateway do Collector | Decide com o trace completo (ex.: manter todo trace com erro ou lento) | Preserva traces relevantes; exige buffer e CPU no gateway |

Recomendação por porte:
- **Pequeno**: head sampling simples (ex.: 100% em baixo RPS, ou taxa fixa) ou sem sampling.
- **Médio**: head sampling parametrizável (ex.: 10–25%).
- **Grande**: **tail sampling** no gateway — reter 100% de traces com erro/lentos e amostrar o restante.

## Controle de cardinalidade (o principal vetor de custo de métricas)

Cada combinação única de valores de atributos gera uma nova série temporal. Regras:
- **Proibido em métricas**: `user_id`, `request_id`, `session_id`, `trace_id`, e-mail, URL crua com query string.
- Usar `http.route` (template) em vez do path concreto.
- Limitar valores possíveis de `http.response.status_code` ao necessário; considerar classes (`2xx`, `4xx`, `5xx`) quando o backend permitir.
- Auditar dimensões antes de adicionar: "quantas séries isto multiplica?".

## Robustez (não perder dados sob carga)

- `memory_limiter` no Collector evita OOM em picos.
- `sending_queue` + `retry_on_failure` no exporter absorvem indisponibilidade temporária do backend.
- Respeitar backpressure do OTLP (`Unavailable` + `RetryInfo`) com backoff exponencial + jitter, em vez de retry agressivo que amplifica a sobrecarga.
- Em grande porte, filas persistentes (disco) evitam perda em reinício do Collector.

## Economia — checklist por porte

| Alavanca | Pequeno | Médio | Grande |
| --- | --- | --- | --- |
| Sampling de traces | opcional / 100% | head 10–25% | tail (erros/lentos 100%) |
| `batch` | sim | sim | sim |
| Cardinalidade controlada | sim | sim | sim (auditoria periódica) |
| Filas/retry | básico | queue + retry | queue persistente + retry |
| Métricas de negócio | mínimas | seletivas | seletivas + revisão de custo |

Documentar no STANDARD.md as decisões (taxa de sampling, dimensões permitidas, filas) e o racional de custo.
