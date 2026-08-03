# Documento de Requisitos do Produto (PRD): Observabilidade Production-Ready — Fechamento de Gaps e Reconciliação (Four Golden Signals + OpenTelemetry)

<!-- spec-version: 2 -->

> Revisão spec-version 2 (2026-08-03): a v1 assumiu uma observabilidade imatura (sem Collector, sem SLO, "3 dashboards e 3 alertas") e propôs construir a stack. A investigação profunda de `deployment/telemetry/` revelou uma stack production-grade já versionada (OTel Collector com tail sampling; Grafana provisionado com alertas RED/saturação/host, 4+ dashboards, contact-point Telegram; LGTM + node/postgres/blackbox exporters). Este PRD foi re-escopado para **fechar os gaps residuais e reconciliar inconsistências**, não para construir do zero. Todos os requisitos abaixo refletem o estado real verificado. Consumidores downstream (techspec/tasks) devem usar esta versão.

## Visão Geral

O MeControla possui uma stack de observabilidade madura e versionada em `deployment/telemetry/`: um OpenTelemetry Collector (`otelcol-config.yaml`) com receivers OTLP 4317/4318, `memory_limiter`, `batch` e `tail_sampling` (10% probabilístico + retenção de erros e spans de fluxo de agente), exportando métricas/traces/logs para Prometheus/Tempo/Loki; Grafana provisionado com datasources, 4+ dashboards (`agent-runtime-overview`, `mecontrola-api`, `mecontrola-infra`, `mecontrola-ops`, `mecontrola-platform`) e um conjunto extenso de alertas por sintoma (RED da API, saturação de host via node-exporter, DB pool, pgbouncer, pgBackRest, outbox, agente e saúde do próprio Collector); contact-point Telegram; e exporters node/postgres/blackbox. A aplicação emite ~74 métricas de domínio e 412 spans nos 10 módulos, além do histograma HTTP canônico `http.server.request.duration` via o provider devkit.

O que impede operação totalmente confiável são gaps residuais e inconsistências, não ausência de fundação:

1. Não há métricas de saturação do processo Go (goroutines/heap/GC) — apenas saturação de host via node-exporter.
2. Os alertas de latência/erro são thresholds estáticos (p99 > 1s, 5xx > 5%), sem SLO formal nem burn-rate ligado a error budget.
3. Vários alertas provisionados referenciam métricas que não existem mais no código (herdadas do `internal/agent` descontinuado), disparando sobre dados ausentes.
4. O roteamento de alertas tem apenas Telegram, sem tier de e-mail por severidade.
5. Não há `observability/STANDARD.md` nem política documentada de cardinalidade/sampling.
6. O alerta de taxa de erro usa o counter auxiliar `http_server_request_count_total` em vez da série canônica do histograma.

Este PRD entrega o fechamento desses gaps e a reconciliação, preservando integralmente a fundação existente. Fonte canônica: US-OBS-001 (`docs/us/2026-08-03-observabilidade-golden-signals-otel-codebase.md`) e o padrão da skill `golden-signals-otel-standards`.

## Objetivos

- Adicionar saturação do processo Go (goroutines/heap/GC) ao pipeline OTLP existente, para API e worker.
- Formalizar SLO de disponibilidade 99,9% (error budget 0,1%) e SLO de latência P95 < 500ms em 30 dias, com alertas de burn-rate multi-janela ao lado dos thresholds existentes.
- Auditar e reconciliar os alertas/dashboards provisionados: nenhum alerta pode referenciar métrica ausente do código atual.
- Adicionar tier de e-mail para tickets, mantendo o Telegram para páginas (split por severidade).
- Reconciliar a fonte de RED para a série canônica do histograma `http.server.request.duration`, mantendo os counters devkit como auxiliares documentados.
- Materializar `observability/STANDARD.md` + assets, documentando a topologia real (tail sampling), a cardinalidade e as decisões, validado por `scripts/validate-standard.py`.
- Não regredir nenhuma métrica, span, dashboard, alerta válido ou configuração de Collector já existente.

Robustez e economia são inegociáveis: reusar a fundação existente em vez de recriar; métricas agregadas > traces amostráveis; cardinalidade estrita a rótulos enumerados; nenhum alerta que pague humano sem mapear a sintoma e sem métrica viva; zero regressão.

Métricas-chave de sucesso:

- Saturação do processo Go consultável para API e worker.
- SLO/error budget refletidos em alertas de burn-rate multi-janela ativos ao lado dos thresholds.
- Zero alertas provisionados referenciando métricas inexistentes (auditoria com resultado limpo).
- `scripts/validate-standard.py` retorna `SUCCESS` para o STANDARD.md.
- Nenhuma métrica/span/dashboard/alerta válido existente removido ou renomeado indevidamente (não-regressão).

## Histórias de Usuário

- Como engenheiro de plataforma responsável pelo on-call (persona primária), quero saturação do processo Go, alertas ligados ao error budget e alertas que refletem métricas vivas, para diagnosticar incidentes com confiança e sem falsos alarmes por métrica morta.
- Como desenvolvedor do produto (persona secundária), quero um `STANDARD.md` que documente a topologia real e o inventário de métricas, para instrumentar novas features sem divergir do que já existe.
- Como dono de produto/negócio (persona secundária), quero SLO formal e visão de confiabilidade, para acompanhar a saúde do serviço.

Fluxos e casos de borda estão detalhados na US-OBS-001 (cenários Gherkin, incluindo o de auditoria de alertas obsoletos).

## Funcionalidades Core

- **Saturação de runtime do processo Go**: instrumentar API e worker com `go.opentelemetry.io/contrib/instrumentation/runtime`, emitindo `go.goroutine.count`, `go.memory.*`, `go.memory.gc.goal`, `go.processor.limit`, `go.config.gogc` pelo pipeline OTLP. Importa porque hoje só há saturação de host, não do processo (goroutine leak/heap/GC invisíveis).
- **SLO formal + burn-rate**: definir SLO de disponibilidade e latência e adicionar alertas de burn-rate multi-janela derivados do error budget, complementando os thresholds estáticos existentes. Importa porque liga o alarme ao orçamento de erro.
- **Reconciliação alerta↔métrica**: auditar `provisioning/alerting/rules.yaml` (e dashboards) contra o inventário de métricas realmente emitidas; corrigir ou remover os que referenciam séries mortas. Importa porque alertas sobre métrica ausente dão falsa sensação de cobertura.
- **Tier de e-mail para tickets**: adicionar contact-point de e-mail (reusando SMTP/Resend) e rotear páginas (severidade alta) para Telegram e tickets (severidade baixa) para e-mail. Importa porque hoje tudo vai só para Telegram.
- **Reconciliação da fonte de RED**: padronizar tráfego/erros sobre `http_server_request_duration_seconds_count` filtrado por `http.response.status_code`, mantendo `http_server_request_count_total` como auxiliar documentado. Importa porque o alerta de erro atual usa o counter auxiliar.
- **Padrão auditável (`STANDARD.md`)**: documentar a topologia real (Collector, tail sampling, exporters LGTM), o inventário de métricas, a cardinalidade e as decisões, validado por script. Importa porque o padrão hoje é tribal.
- **Preservação da fundação**: manter Collector, tail sampling, dashboards e alertas válidos intactos. Importa porque a economia exige reuso, não reconstrução.

## Requisitos Funcionais

- RF-01: API e worker DEVEM emitir métricas de saturação do processo Go via `go.opentelemetry.io/contrib/instrumentation/runtime` na versão compatível com `go.opentelemetry.io/otel v1.44.0` (v0.69.0), pelo pipeline OTLP existente. As séries são exatamente as emitidas pelo caminho default (`go.*`): `go.memory.used`, `go.memory.limit`, `go.memory.allocated`, `go.memory.allocations`, `go.memory.gc.goal`, `go.goroutine.count`, `go.processor.limit`, `go.config.gogc`. Nenhum nome pode ser inventado.
- RF-02: O wiring da instrumentação de runtime DEVE habilitar o registro do MeterProvider global (`RegisterGlobal: true` no `otel.Config` de server e worker), já que o provider devkit não expõe um `MeterProvider` público e a lib usa o provider global por default. A ativação NÃO pode quebrar as métricas de instância já emitidas via `o11y.Metrics()`.
- RF-03: A saturação de runtime DEVE ser exposta em dashboard e ter ao menos um alerta de causa (ex.: crescimento sustentado de goroutines ou de heap acima do alvo) que NÃO pagina humano isoladamente, servindo para diagnóstico.
- RF-04: O serviço DEVE ter SLO de disponibilidade de 99,9% (error budget 0,1%) e SLO de latência P95 < 500ms, ambos em janela de 30 dias, documentados no `STANDARD.md`.
- RF-05: DEVEM ser adicionados alertas de burn-rate multi-janela para a disponibilidade, seguindo o esquema do Google SRE Workbook derivado do error budget de 0,1%: página em 14,4× (2% do orçamento, janelas 1h e 5m); página em 6× (5%, 6h e 30m); ticket em 3× (10%, 1d e 2h); ticket em 1× (10%, 3d e 6h). Estes complementam, não removem, os thresholds estáticos existentes.
- RF-06: DEVE existir um alerta de latência ligado ao SLO P95 < 500ms (burn-rate ou threshold sobre P95 de sucesso), coexistindo com o alerta atual de p99 > 1s.
- RF-07: Uma auditoria de reconciliação DEVE comparar todo alerta/painel provisionado contra o inventário de métricas realmente emitidas; alertas que referenciam métricas ausentes do código atual (confirmados: `agent_intent_parsed_total`, `agent_llm_fallback_exhausted_total`, `agent_policy_blocks_total`, `agent_intent_routed_total`, `agent_idempotency_replay_total`, `outbox_pending_jobs`) DEVEM ser corrigidos para a métrica viva equivalente ou removidos com justificativa registrada. Ao final, nenhum alerta pode referenciar série morta.
- RF-08: Tráfego e taxa de erro em dashboards/alertas DEVEM usar como fonte primária a série canônica `http_server_request_duration_seconds_count` (erro filtrado por `http.response.status_code=~"5.."`); `http_server_request_count_total`/`_active` permanecem como auxiliares documentados. O alerta `mc-api-5xx` DEVE ser migrado para a série canônica ou ter o uso do auxiliar justificado no `STANDARD.md`.
- RF-09: DEVE ser adicionado um contact-point de e-mail (reusando a infraestrutura de envio existente, SMTP/Resend) e uma política de roteamento que envie alertas de severidade alta (página) ao Telegram e de severidade baixa (ticket) ao e-mail. O Telegram existente NÃO pode ser removido.
- RF-10: Cada alerta que pagina humano DEVE referenciar um runbook (em `docs/runbooks/` ou `deployment/runbooks/`, seguindo o padrão já usado nas anotações `description`); nenhum alerta de página pode existir sem runbook.
- RF-11: O worker DEVE ter um sinal de liveness observável (heartbeat/atividade por OTLP) e um alerta de ausência (staleness); se a cobertura atual (ex.: outbox queue/dead-letter) já suprir isso, a auditoria DEVE registrar a evidência e a decisão de não duplicar.
- RF-12: Logs de produção DEVEM carregar trace context (`trace_id`, `span_id`) para pivot log↔trace; se o provider devkit já o faz via `otelslog`, a auditoria DEVE registrar a evidência; caso contrário, DEVE ser habilitado.
- RF-13: Nenhuma métrica pode carregar rótulo de alta cardinalidade (`user_id`, `request_id`, `correlation_id`/`correlation_key`, `category_id`, IDs de sessão); rota HTTP DEVE usar `http.route`. Reforça R-TXN-004, R-WF-KERNEL-001.4 e R-AGENT-WF-001.5.
- RF-14: Nenhum nome de métrica/atributo pode ser inventado fora do OpenTelemetry Semantic Conventions e das métricas já existentes; a instrumentação de runtime usa os nomes `go.*` da lib oficial (RF-01).
- RF-15: A topologia de coleta existente (Collector `otelcol-config.yaml` com OTLP 4317/4318, `memory_limiter`, `tail_sampling`, `batch`, exporters Prometheus/Tempo/Loki) DEVE ser preservada; nenhuma mudança pode remover o tail sampling nem os três pipelines. Métricas permanecem não amostradas.
- RF-16: A estratégia de sampling documentada no `STANDARD.md` DEVE ser o `tail_sampling` já vigente (10% + retenção de erros e spans de agente), não head sampling. A app continua enviando 100% (head) ao Collector.
- RF-17: Todo sinal DEVE portar `service.name`, `service.version` e `deployment.environment`, preservando `service.instance.id` já injetado.
- RF-18: DEVE ser materializado `observability/STANDARD.md` + assets (queries PromQL dos golden signals, regras de alerta novas, e a referência ao Collector existente), passando em `scripts/validate-standard.py` com `SUCCESS`.
- RF-19: Os dashboards existentes DEVEM ser estendidos (ou um novo painel adicionado) para cobrir a saturação de runtime do processo e a visão de SLO/burn-rate; a geração/edição do JSON de dashboard é delegada à skill `otel-grafana-dashboards`.
- RF-20: A evolução NÃO pode remover nem renomear métricas, spans, dashboards ou alertas VÁLIDOS já existentes (agent-gate, whatsapp, transactions, e os provisionados que referenciam métricas vivas).

## Experiência do Usuário

Persona primária: engenheiro de plataforma/on-call — consome os dashboards existentes (estendidos com runtime e SLO), recebe páginas no Telegram e tickets por e-mail, e confia que os alertas refletem métricas vivas. Personas secundárias: desenvolvedor (segue o `STANDARD.md` e o inventário de métricas) e dono de produto (visão de SLO). A UI dos dashboards é entregue pela skill de dashboards delegada.

## Restrições Técnicas de Alto Nível

- Provider de observabilidade OTel do devkit-go v0.5.5: não expõe `MeterProvider` público e roda com `RegisterGlobal` desligado hoje; a instrumentação de runtime exige ativar `RegisterGlobal: true`.
- Backend de produção: Grafana Stack (Prometheus/Tempo/Loki) via OTLP, confirmado pelos exporters do Collector existente.
- Nova dependência: `go.opentelemetry.io/contrib/instrumentation/runtime` v0.69.0 (compatível com `otel v1.44.0`).
- Infra Docker em host único (não-Kubernetes); Collector, LGTM e exporters já rodam como containers no compose `deployment/compose/`.
- Conformidade obrigatória com o OpenTelemetry Semantic Conventions e com as regras de cardinalidade vigentes (R-TXN-004, R-WF-KERNEL-001.4, R-AGENT-WF-001.5).
- Política de zero comentários em Go de produção e adaptadores finos (R-ADAPTER-001) para qualquer código de instrumentação adicionado.
- Robustez e economia inegociáveis: reusar a fundação existente; preferir métricas agregadas a traces; cardinalidade limitada; nenhum alerta que pague sem sintoma e sem métrica viva; zero regressão.

## Fora de Escopo

- Reconstrução do Collector, do stack LGTM ou dos dashboards existentes (já existem e são preservados).
- Substituição do tail sampling por head sampling na aplicação.
- `go.schedule.duration` da lib de runtime (exige registrar um `metric.Producer` no Reader, que o provider devkit constrói internamente e não expõe; documentar como limitação, não implementar sem mudança no devkit).
- Geração do JSON dos dashboards Grafana (delegada à skill `otel-grafana-dashboards`).
- Monitoramento de host adicional além do node-exporter já presente.
- SLOs por endpoint individual ou de negócio por funcionalidade; este PRD fixa SLO de serviço.
- Alteração do código do provider devkit para renomear os counters HTTP auxiliares ao semconv.

## Decisões Fechadas

Todas as questões de produto foram resolvidas; não há suposição em aberto nem ressalva pendente.

- Escopo: fechamento de gaps + reconciliação sobre a stack existente (não construção do zero). Confirmado por investigação de `deployment/telemetry/`.
- Saturação de runtime: `contrib/instrumentation/runtime` v0.69.0, caminho default `go.*`, via `RegisterGlobal: true` (não há getter público de MeterProvider no devkit).
- SLO: disponibilidade 99,9% (budget 0,1%) e latência P95 < 500ms em 30 dias; alertas de burn-rate no esquema SRE de 4 janelas (RF-05), complementando os thresholds existentes.
- Backend: Grafana Stack (Prometheus/Tempo/Loki), já em uso via exporters do Collector.
- Coleta/sampling: tail sampling do Collector existente preservado; app envia 100% head; métricas 100%. Head sampling na app foi rejeitado por remover a retenção de erros do gateway.
- Roteamento: página→Telegram (existente), ticket→e-mail (novo, reusa SMTP/Resend); separação por severidade.
- Reconciliação: alertas que referenciam `agent_intent_parsed_total`, `agent_llm_fallback_exhausted_total`, `agent_policy_blocks_total`, `agent_intent_routed_total`, `agent_idempotency_replay_total` e `outbox_pending_jobs` são corrigidos ou removidos (métricas do `internal/agent` descontinuado / não emitidas).
- RED: fonte canônica é `http_server_request_duration_seconds_count`; counters devkit permanecem auxiliares documentados.
