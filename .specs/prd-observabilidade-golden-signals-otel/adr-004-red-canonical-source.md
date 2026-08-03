# Registro de Decisão Arquitetural (ADR-004)

## Metadados

- **Título:** Fonte canônica de RED = série do histograma `http.server.request.duration`; counters devkit como auxiliares
- **Data:** 2026-08-03
- **Status:** Aceita
- **Decisores:** Engenharia de plataforma (on-call)
- **Relacionados:** PRD (RF-08, RF-14), techspec.md, ADR-002, `golden-signals-otel-mapping.md`

## Contexto

O provider devkit emite, além do histograma canônico `http.server.request.duration` (buckets `[0.005…10.0]`, atributos `http.request.method`/`http.route`/`http.response.status_code` — verificado em `pkg/observability/otel/http.go:14-84`), três counters auxiliares não-canônicos: `http.server.request.count`, `http.server.request.active`, `http.server.request.error.count`. O semantic convention HTTP deriva tráfego e erros do `_count` do histograma, não de counters separados. Hoje o alerta `mc-api-5xx` usa `http_server_request_count_total` como fonte da taxa de erro, divergindo do padrão.

## Decisão

Padronizar tráfego e taxa de erro sobre a série canônica `http_server_request_duration_seconds_count`, filtrando erros por `http.response.status_code=~"5.."`. Os counters `http_server_request_count_total`/`_active` permanecem como sinais AUXILIARES documentados (úteis para liveness `absent(...)` e in-flight), mas não são a fonte primária de RED em novos dashboards/alertas. O alerta `mc-api-5xx` é migrado para a série canônica ou tem o uso do auxiliar justificado explicitamente no `STANDARD.md`. Não se altera o provider devkit (fora de escopo).

## Alternativas Consideradas

- **Manter os counters devkit como fonte primária**: simples, mas foge do semconv e cria dependência de nomes não-canônicos. Rejeitada como fonte primária; mantida como auxiliar.
- **Patch no devkit para emitir só o canônico**: fora do controle do repositório. Rejeitada.
- **Adicionar `error.type` ao histograma**: exigiria mudança no provider devkit. Rejeitada (fora de escopo).

## Consequências

### Benefícios Esperados

- RED alinhado ao semconv, uma única fonte de verdade para latência/tráfego/erros.
- Consistência entre dashboards, alertas de threshold e de burn-rate (ADR-002).

### Trade-offs e Custos

- Reescrever queries do `mc-api-5xx` e de dashboards que usem o counter auxiliar.

### Riscos e Mitigações

- **Risco:** diferença numérica entre o counter auxiliar e o `_count` do histograma durante a transição. **Mitigação:** validar as duas séries em paralelo antes de cortar; documentar no STANDARD.md.
- **Risco:** liveness que hoje usa `http_server_request_active` quebrar. **Mitigação:** manter o auxiliar disponível para o alerta `mc-api-down`.

## Plano de Implementação

1. Confirmar as séries Prometheus (`http_server_request_duration_seconds_count` e labels) no ambiente.
2. Migrar `mc-api-5xx` para a fórmula canônica; ajustar dashboards.
3. Documentar counters auxiliares e seu uso permitido (liveness/in-flight) no STANDARD.md.

## Monitoramento e Validação

- Sucesso: taxa de erro e tráfego dos dashboards/alertas derivam do histograma; valores coerentes com o counter auxiliar no período de validação.

## Impacto em Documentação e Operação

- `observability/STANDARD.md`: fonte canônica de RED e papel dos auxiliares.
- `promql-golden-signals.md`: queries canônicas.

## Revisão Futura

- Revisitar se o devkit passar a emitir `error.type` no histograma ou remover os counters auxiliares.
