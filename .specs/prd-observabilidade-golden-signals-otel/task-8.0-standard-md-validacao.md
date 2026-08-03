# Tarefa 8.0: STANDARD.md + assets + validação + cardinalidade/sampling

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Materializar `observability/STANDARD.md` + `observability/promql-golden-signals.md` + `observability/alert-rules-slo.yaml`, consolidando a documentação auditável da observabilidade: a topologia REAL de coleta (Collector preservado), a estratégia de sampling vigente, a política de cardinalidade controlada, o inventário de métricas e a evidência de correlação log↔trace. Rodar `scripts/validate-standard.py` da skill `golden-signals-otel-standards` até `SUCCESS`. Cobre RF-12, RF-13, RF-15, RF-16, RF-18. Depende das Tarefas 2.0, 4.0, 5.0, 6.0 e 7.0 (consolida tudo).

<requirements>
- Documentar a topologia REAL de coleta: Collector `deployment/telemetry/grafana/otelcol-config.yaml` com OTLP 4317/4318, `memory_limiter`, `tail_sampling` (10% + retenção de erros e spans de agente), `batch`, exporters Prometheus/Tempo/Loki — PRESERVADA (RF-15); nenhuma mudança remove o tail sampling nem os três pipelines.
- Documentar a estratégia de sampling = `tail_sampling` vigente, NÃO head sampling; app envia 100% (head) ao Collector; métricas 100% não amostradas (RF-16).
- Documentar a política de cardinalidade controlada: sem `user_id`/`request_id`/`correlation_id`/`correlation_key`/`category_id`/IDs de sessão; rota HTTP via `http.route`, não URL crua (RF-13, reforça R-TXN-004, R-WF-KERNEL-001.4, R-AGENT-WF-001.5).
- Documentar o inventário de métricas, incluindo as `go.*` de runtime e os counters HTTP auxiliares.
- Confirmar e registrar a evidência de que a correlação log↔trace já é emitida pelo devkit (`pkg/observability/otel/logger.go:229-238` adiciona `trace_id`) — RF-12 já satisfeito; nenhum novo trabalho.
- Rodar `python3 .agents/skills/golden-signals-otel-standards/scripts/validate-standard.py --standard observability/STANDARD.md --collector <collector>` até retornar `SUCCESS` (RF-18).
</requirements>

## Subtarefas

- [ ] 8.1 Materializar `observability/STANDARD.md` documentando a topologia real (Collector, OTLP 4317/4318, `memory_limiter`, `tail_sampling`, `batch`, exporters LGTM — preservados), a estratégia de sampling (tail sampling, não head; métricas 100%) e a política de cardinalidade controlada.
- [ ] 8.2 Materializar `observability/promql-golden-signals.md` (queries PromQL dos golden signals) e `observability/alert-rules-slo.yaml` (regras de alerta SLO como asset).
- [ ] 8.3 Documentar o inventário de métricas, incluindo `go.*` de runtime e counters HTTP auxiliares.
- [ ] 8.4 Confirmar e registrar a evidência da correlação log↔trace emitida pelo devkit (`pkg/observability/otel/logger.go:229-238`), marcando RF-12 como já satisfeito.
- [ ] 8.5 Rodar `scripts/validate-standard.py` apontando para `observability/STANDARD.md` e o Collector, corrigir o que for apontado, até obter `SUCCESS`.

## Detalhes de Implementação

Ver techspec.md, seção "Sequenciamento de Desenvolvimento → Ordem de Build" (item 7, STANDARD.md consolida topologia real, inventário de métricas, cardinalidade e decisões; roda `scripts/validate-standard.py`) e "Monitoramento e Observabilidade" (correlação `trace_id`/`span_id` já emitida pelo devkit em `pkg/observability/otel/logger.go:229-238` — apenas confirmar, RF-12). A topologia de Collector preservada e a estratégia de sampling estão fixadas em RF-15/RF-16 (PRD); a política de cardinalidade em RF-13 reforça R-TXN-004, R-WF-KERNEL-001.4 e R-AGENT-WF-001.5. A fonte canônica de RED e o papel dos counters auxiliares seguem `adr-004-red-canonical-source.md`; os SLOs e a tabela de burn-rate vêm da Tarefa 4.0 (`adr-002-burn-rate-slo-alerts.md`).

## Critérios de Sucesso

- `observability/STANDARD.md`, `observability/promql-golden-signals.md` e `observability/alert-rules-slo.yaml` materializados e coerentes com a stack real.
- A topologia de Collector (tail sampling + três pipelines) documentada como preservada; sampling documentado como tail sampling (não head), métricas 100%.
- A política de cardinalidade controlada documentada; nenhum rótulo proibido (`user_id`/`request_id`/`correlation_id`/`correlation_key`/`category_id`/IDs de sessão) presente nas métricas/alertas.
- Inventário de métricas inclui `go.*` de runtime e counters HTTP auxiliares; correlação log↔trace registrada com evidência (`logger.go:229-238`).
- `scripts/validate-standard.py` retorna `SUCCESS` para o STANDARD.md.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `golden-signals-otel-standards` — template STANDARD.md, assets e script validate-standard.py.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

Validar que `scripts/validate-standard.py` retorna `SUCCESS` sobre o `observability/STANDARD.md` e que a checagem de cardinalidade não encontra nenhum rótulo proibido nas métricas/alertas.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `observability/STANDARD.md` — padrão auditável (topologia real, sampling, cardinalidade, inventário de métricas, correlação log↔trace).
- `observability/promql-golden-signals.md` — queries PromQL dos golden signals.
- `observability/alert-rules-slo.yaml` — regras de alerta SLO como asset.
