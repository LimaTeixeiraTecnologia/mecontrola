# Alertas e SLO Orientados a Sintomas

Como derivar alertas e SLOs dos Four Golden Signals, priorizando sintomas e reduzindo ruído. Base de regras em `assets/alert-rules.yaml`.

**Fontes:** Google SRE Book — *Monitoring Distributed Systems* e *Service Level Objectives*.

## Princípio: alertar por sintoma

> Your monitoring system should address two questions: "what's broken, and why?"

- **Sintoma** = o que o usuário sente (erros, lentidão). **Causa** = o porquê interno.
- "It's better to spend much more effort on catching symptoms than causes."
- "In a multilayered system, one person's symptom is another person's cause."
- Paginar humano apenas quando um sinal está problemático (ou, no caso de saturação, quase problemático).

## SLI, SLO e error budget

- **SLI** (indicador): a medida real do sinal (ex.: proporção de requisições com sucesso, proporção sob o limite de latência).
- **SLO** (objetivo): a meta sobre o SLI (ex.: 99,9% de disponibilidade em 30 dias).
- **Error budget** = `1 − SLO`. Para 99,9%, o orçamento é 0,1% das requisições (ou do tempo) na janela.

Tabela de referência (downtime anual aproximado):

| SLO | Error budget | Downtime/ano aprox. |
| --- | --- | --- |
| 99,0% | 1,0% | ~3,65 dias |
| 99,5% | 0,5% | ~1,83 dia |
| 99,9% | 0,1% | ~8,77 horas |
| 99,95% | 0,05% | ~4,38 horas |
| 99,99% | 0,01% | ~52,6 minutos |

## Burn rate multi-janela (reduzir ruído e detecção tardia)

**Burn rate** = velocidade de consumo do error budget. Burn rate 1 = consome todo o orçamento exatamente ao fim da janela do SLO; burn rate 14,4 = consumiria em ~1/14,4 do período.

Alertar com **duas janelas** (curta + longa) para equilibrar rapidez e estabilidade:
- **Alerta rápido (página)**: burn rate alto (ex.: 14,4) sustentado em janela longa (1h) **e** confirmado em janela curta (5m) — detecta incidentes agudos rapidamente.
- **Alerta lento (ticket)**: burn rate menor (ex.: ~3) em janelas de 6h/30m — pega degradação lenta que ainda estoura o orçamento.

Combinar as janelas evita: (a) alarme falso por picos curtos (janela curta sozinha) e (b) detecção tardia (janela longa sozinha).

## Alertas por Golden Signal

| Sinal | Sintoma alvo | Base do alerta |
| --- | --- | --- |
| Latência | P95/P99 acima do SLO de latência | `histogram_quantile` sobre `http_server_request_duration_seconds_bucket` |
| Tráfego | queda anômala ou pico (contexto) | `rate()` do `_count` |
| Erros | taxa de erro acima do budget / burn rate | ratio de `5xx` sobre total |
| Saturação | recurso limitante > alvo (ex.: 80%) | métrica de runtime/host do recurso constrangido |

## Thresholds default (quando o usuário não especificar)

- Error rate > 5% por 5 min (página).
- Latência P95 > SLO de latência do Passo 1 por 10 min.
- Saturação (CPU/memória do runtime) > 80% por 10 min.

Preferir, sempre que houver SLO definido, os alertas de **burn rate** aos thresholds fixos, pois eles ligam o alarme diretamente ao orçamento de erro.

## Regras
- Todo alerta que paga humano deve mapear a um sintoma percebido pelo usuário.
- Alertas de causa (ex.: pool de conexões cheio) servem para diagnóstico/tickets, não necessariamente para página.
- Derivar thresholds do SLO informado no Passo 1; não usar números arbitrários sem justificativa.
