# Documento de Requisitos do Produto (PRD): Observabilidade Production-Ready (Four Golden Signals + OpenTelemetry)

<!-- spec-version: 1 -->

## Visão Geral

O MeControla já emite telemetria OpenTelemetry, mas de forma desigual e não governada: a latência/tráfego/erros HTTP e o pool de banco já são instrumentados, há ~74 métricas de domínio e 412 spans nos 10 módulos, porém não há métricas de saturação de runtime, não há SLO nem alertas de burn-rate, existem apenas 3 dashboards e 3 arquivos de alerta pontuais, não há um documento de padrão, não há topologia de Collector versionada e o sampling está fixo em 100%. Em um incidente de produção, o on-call não dispõe de uma visão RED por rota, de sinal de saturação, nem de alerta ligado ao orçamento de erro.

Este PRD define a evolução da observabilidade do codebase inteiro (API, worker e módulos de domínio) para um padrão único, auditável e production-ready, ancorado nos Four Golden Signals do Google SRE (Latência, Tráfego, Erros, Saturação) e nos padrões oficiais do OpenTelemetry (Specification, OTLP e Semantic Conventions). O valor é operacional: reduzir o tempo de detecção e diagnóstico de incidentes, ligar alertas ao error budget explícito e eliminar dependência de conhecimento tribal.

A fonte canônica desta iniciativa é a história de usuário `docs/us/2026-08-03-observabilidade-golden-signals-otel-codebase.md` (US-OBS-001) e o padrão da skill `golden-signals-otel-standards`.

## Objetivos

- Cobrir os quatro Golden Signals para API e worker, com latência sempre por histograma/percentil (nunca por média).
- Fechar a lacuna de saturação de runtime (goroutines, heap, GC) dentro do pipeline OTLP existente, para server e worker.
- Estabelecer SLO de disponibilidade de 99,9% (error budget 0,1%) e SLO de latência P95 < 500ms em janela de 30 dias, com alertas por sintoma derivados desse orçamento.
- Introduzir um OpenTelemetry Collector agent único como ponto de coleta padronizado, com sampling e cardinalidade governados.
- Materializar um documento de padrão auditável (`observability/STANDARD.md`) mais assets de coleta, alertas e queries PromQL, validado automaticamente.
- Especificar os sinais e queries dos novos dashboards (RED por rota, banco, saturação, visão de serviço), delegando a geração do JSON à skill de dashboards.
- Não regredir nenhuma métrica ou span de domínio já consumido pelos dashboards e alertas existentes.

Robustez e economia são requisitos não negociáveis desta iniciativa: robustez no sentido de sinais confiáveis, alertas ligados ao error budget (sem falso alarme nem detecção tardia) e ausência de regressão da instrumentação existente; economia no sentido de cardinalidade estritamente controlada, métricas agregadas (não amostradas) preferidas a traces caros, sampling configurável para conter custo sob volume e reuso máximo das métricas já emitidas em vez de novas séries.

Métricas-chave de sucesso:

- Os quatro sinais consultáveis para API e worker no backend.
- SLO e error budget definidos e refletidos em alertas de burn-rate multi-janela.
- `scripts/validate-standard.py` retorna `SUCCESS` para o STANDARD.md e o Collector gerados.
- Zero rótulos de alta cardinalidade nas métricas; zero nomes de métrica inventados fora do semconv ou do domínio já existente.
- Nenhuma métrica/span de domínio existente removido ou renomeado (não-regressão).

## Histórias de Usuário

- Como engenheiro de plataforma responsável pelo on-call (persona primária), quero uma visão RED por rota, sinal de saturação e alertas ligados ao error budget, para diagnosticar e responder a incidentes em minutos.
- Como desenvolvedor do produto (persona secundária), quero um padrão de instrumentação documentado e traces/métricas por módulo, para instrumentar novas features de forma consistente e diagnosticar regressões.
- Como dono de produto/negócio (persona secundária), quero uma visão de confiabilidade e SLO, para acompanhar a saúde percebida do serviço sem depender de detalhes técnicos.

Fluxos e casos de borda estão detalhados na US-OBS-001 (cenários Gherkin: RED por rota, saturação de runtime, burn-rate de disponibilidade, degradação lenta de latência, worker por liveness, coleta via Collector, bloqueio de alta cardinalidade, recusa de nome fora do semconv, falha de validação, não-regressão).

## Funcionalidades Core

- **Cobertura RED da API**: latência (P95/P99 por rota), tráfego e taxa de erro derivados do histograma canônico `http.server.request.duration`, com separação de latência de sucesso e de erro. Importa porque é o sinal primário do que o usuário sente.
- **Saturação de runtime**: métricas de goroutines/heap/GC para server e worker via instrumentação OTel de runtime, transportadas pelo mesmo pipeline OTLP. Importa porque saturação é indicador antecipado de degradação, hoje ausente.
- **Saturação de recursos limitantes**: reaproveita `db.client.connections.*` (pool de banco) e `outbox_lag_seconds`/`outbox_dead_letter_total` (fila) já emitidos. Importa porque são os gargalos reais conhecidos do serviço.
- **Liveness do worker**: heartbeat/atividade por OTLP com alerta de ausência (staleness), já que o worker não tem superfície HTTP. Importa porque o processamento em background pode parar silenciosamente.
- **SLO e alertas por sintoma**: SLO de disponibilidade e latência com alertas de burn-rate multi-janela e alertas de causa que não paginam, roteados por severidade (chat para página, e-mail para ticket) e cada página com runbook dedicado. Importa porque liga o alarme ao orçamento de erro, reduz ruído e fecha o loop de resposta do on-call.
- **Correlação log↔trace**: logs carregando trace context (`trace_id`/`span_id`) via `otelslog`, com destino Loki, para pivot direto entre log e trace no incidente. Importa porque completa os três sinais OTel e acelera o diagnóstico da causa.
- **Coleta padronizada**: Collector agent único (OTLP 4317/4318, `batch` + `memory_limiter`) e head sampling parent-based por ambiente, métricas sem sampling. Importa porque desacopla o exporter e controla custo.
- **Padrão auditável**: `observability/STANDARD.md` + assets (Collector, alertas, queries PromQL) validados por script. Importa porque torna o padrão verificável e não tribal.
- **Especificação de dashboards**: sinais e queries dos novos dashboards (RED por rota, banco, saturação, visão de serviço), com a geração do JSON delegada. Importa porque cobre as lacunas de visualização sem violar a fronteira da skill de padrão.

## Requisitos Funcionais

- RF-01: A API DEVE expor latência via histograma `http.server.request.duration` (unidade `s`), preservando os buckets já emitidos `[0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1.0, 2.5, 5.0, 7.5, 10.0]`; latência DEVE ser consultada por percentil (`histogram_quantile`), nunca por média.
- RF-02: Tráfego e taxa de erro DEVEM ser derivados da série `http_server_request_duration_seconds_count`, com erro filtrado por `http.response.status_code=~"5.."`.
- RF-03: Os counters não-canônicos do provider (`http.server.request.count`, `http.server.request.active`, `http.server.request.error.count`) DEVEM ser documentados como sinais auxiliares e NÃO DEVEM ser a fonte primária de RED em dashboards e alertas.
- RF-04: Dashboards e alertas DEVEM permitir separar latência de sucesso da latência de erro.
- RF-05: Server e worker DEVEM emitir métricas de saturação de runtime (goroutines, heap, GC) via instrumentação OTel de runtime, pelo pipeline OTLP existente.
- RF-06: As métricas de saturação de runtime DEVEM ser exatamente as séries emitidas por `go.opentelemetry.io/contrib/instrumentation/runtime` (goroutines, heap, GC), enumeradas no `observability/STANDARD.md`; nenhum nome pode ser inventado nem renomeado. A enumeração da lista completa de séries ocorre na especificação técnica, sempre a partir dessa fonte oficial.
- RF-07: A saturação de banco DEVE ser derivada das métricas `db.client.connections.*` já emitidas por `otelsql`; a saturação de fila DEVE ser derivada de `outbox_lag_seconds` e `outbox_dead_letter_total` já existentes.
- RF-08: O worker DEVE emitir um sinal de heartbeat/atividade por OTLP que permita alerta de ausência (staleness) quando o processamento em background parar.
- RF-09: O serviço DEVE ter SLO de disponibilidade de 99,9% (error budget 0,1%) e SLO de latência P95 < 500ms, ambos em janela de 30 dias.
- RF-10: Todo alerta que pagina humano DEVE mapear a um sintoma percebido pelo usuário e, havendo SLO, DEVE usar burn-rate multi-janela em vez de threshold fixo arbitrário.
- RF-11: O alerta de disponibilidade DEVE seguir o esquema multi-janela/multi-burn-rate do Google SRE Workbook para SLO de 30 dias, com quatro alertas derivados do error budget de 0,1%: página em burn-rate 14,4× (consome 2% do orçamento, janelas 1h e 5m); página em burn-rate 6× (5% do orçamento, janelas 6h e 30m); ticket em burn-rate 3× (10% do orçamento, janelas 1d e 2h); ticket em burn-rate 1× (10% do orçamento, janelas 3d e 6h).
- RF-12: Alertas de causa (pool cheio, saturação de runtime acima do alvo, lag de outbox) NÃO DEVEM paginar humano isoladamente, servindo para diagnóstico/ticket, salvo quando já representam sintoma iminente ao usuário.
- RF-13: Nenhuma métrica pode carregar rótulo de alta cardinalidade (`user_id`, `request_id`, `correlation_id`/`correlation_key`, `category_id`, IDs de sessão); a rota HTTP DEVE usar `http.route`, nunca a URL crua. Esta regra reforça e não flexibiliza R-TXN-004, R-WF-KERNEL-001.4 e R-AGENT-WF-001.5.
- RF-14: Nenhum nome de métrica ou atributo pode ser inventado fora do OpenTelemetry Semantic Conventions e das métricas de domínio já existentes; sinais sem nome semconv (ex.: runtime) DEVEM usar o nome real da instrumentação oficial, registrado no padrão.
- RF-15: A coleta DEVE passar por um OpenTelemetry Collector agent único, com receivers OTLP em 4317 (gRPC) e 4318 (HTTP) e processadores `batch` e `memory_limiter`, dimensionado para porte pequeno/médio (1 API + 1 worker).
- RF-16: Traces DEVEM usar head sampling `ParentBased(TraceIDRatio)` reutilizando `OTEL_TRACE_SAMPLE_RATE`; o ratio default em produção é 100% (justificado pelo baixo RPS atual: custo baixo e sinal máximo), permanecendo configurável para redução imediata sem deploy caso o tráfego cresça. Métricas NÃO DEVEM ser amostradas.
- RF-17: Todo sinal DEVE portar os atributos de recurso `service.name`, `service.version` e `deployment.environment`, preservando o `service.instance.id` já injetado no bootstrap.
- RF-18: O resultado DEVE ser materializado em `observability/STANDARD.md` mais os assets de coleta (`otel-collector.yaml`), alertas (`alert-rules.yaml`) e queries PromQL (`promql-golden-signals.md`), e DEVE passar em `scripts/validate-standard.py` com `SUCCESS`.
- RF-19: Os sinais e queries dos novos dashboards (RED por rota, banco, saturação, visão de serviço da API e do worker) DEVEM ser especificados; a geração do JSON dos dashboards é delegada à skill `otel-grafana-dashboards`.
- RF-20: A evolução NÃO pode remover nem renomear métricas ou spans de domínio já emitidos e consumidos pelos dashboards e alertas atuais (agent-gate, whatsapp, transactions).
- RF-21: Alertas de página (severidade alta) DEVEM notificar um canal de chat de operação (Telegram ou Slack, definido no contact point do Grafana Alerting); alertas de ticket (severidade baixa) DEVEM notificar por e-mail, reutilizando a infraestrutura de envio já existente (SMTP/Resend). A separação de canal por severidade é obrigatória.
- RF-22: Cada alerta que pagina humano DEVE referenciar um runbook dedicado em `docs/runbooks/` com passos de diagnóstico e mitigação; nenhum alerta de página pode existir sem runbook correspondente.
- RF-23: Logs de produção DEVEM carregar o trace context (`trace_id`, `span_id`) para permitir o pivot log↔trace no backend, usando o bridge `otelslog` já presente, com destino Loki via OTLP.
- RF-24: O OpenTelemetry Collector agent DEVE rodar como um container Docker único no host, recebendo OTLP de API e worker, coerente com a topologia de deploy Docker do projeto (infra host, não-Kubernetes).

## Experiência do Usuário

Persona primária: engenheiro de plataforma/on-call — consome dashboards, recebe páginas e tickets, edita alertas e a configuração do Collector. Fluxo principal em incidente: abre a visão de serviço (RED por rota), correlaciona com saturação de runtime e recursos limitantes, e age a partir de alertas de burn-rate que referenciam runbooks.

Personas secundárias: desenvolvedor do produto (instrumenta features novas seguindo o STANDARD.md; consulta traces/métricas por módulo) e dono de produto/negócio (consome a visão de confiabilidade e SLO).

Considerações: os dashboards e alertas vivem no backend Grafana/OTLP; a experiência de UI/UX dos dashboards em si é entregue pela skill de dashboards delegada. Não há requisito de acessibilidade específico além do padrão do backend de visualização.

## Restrições Técnicas de Alto Nível

- Provider de observabilidade OTel do devkit-go v0.5.5, que já emite o histograma HTTP e expõe o hook de instrumentação HTTP; a evolução opera sobre esse provider, sem reescrevê-lo.
- Backend de telemetria de produção: Grafana Stack via OTLP — métricas em Prometheus, traces em Tempo, logs em Loki; os exporters do Collector são configurados para esse destino.
- Nova dependência `go.opentelemetry.io/contrib/instrumentation/runtime` para saturação de runtime.
- Conformidade obrigatória com o OpenTelemetry Semantic Conventions (proibido inventar nomes) e com as regras de cardinalidade já vigentes no repositório (R-TXN-004, R-WF-KERNEL-001.4, R-AGENT-WF-001.5).
- Meta de disponibilidade 99,9% e de latência P95 < 500ms em 30 dias; porte pequeno/médio (1 API + 1 worker) define topologia de Collector agent único e head sampling.
- Infraestrutura de deploy baseada em Docker em host único (não-Kubernetes); o Collector é um container Docker no host. Roteamento de alertas reutiliza o envio de e-mail existente (SMTP/Resend) para tickets e um contact point de chat (Telegram/Slack) para páginas.
- O padrão e os assets devem passar em `scripts/validate-standard.py` (gate de validação automática).
- Política de zero comentários em Go de produção e adaptadores finos (R-ADAPTER-001) aplica-se a qualquer instrumentação adicionada em código.
- Robustez e economia inegociáveis: preferir séries agregadas (métricas) a traces amostráveis; reusar métricas já emitidas antes de criar novas; manter cardinalidade limitada a rótulos enumerados; nenhum alerta que pague humano sem mapear a sintoma; nenhuma regressão de instrumentação existente.

## Fora de Escopo

- Geração do JSON dos dashboards Grafana (delegada à skill `otel-grafana-dashboards`/`otel-hybrid-dashboard-blueprint`; o padrão golden-signals não gera JSON de dashboard).
- Instrumentação não-OTel e monitoramento exclusivo de infraestrutura de host/nó (CPU/RSS do host via node_exporter); a saturação coberta é a de runtime do processo via OTel.
- Tail sampling e topologia de Collector gateway/balanceado de grande porte.
- SLOs por endpoint individual ou SLOs de negócio por funcionalidade; este PRD fixa SLO de serviço.
- Alteração do código do provider devkit-go para renomear os counters HTTP auxiliares ao semconv (fora do controle do repositório).
- Setup de ingestão específico de vendor além do destino OTLP genérico.

## Decisões Fechadas

Todas as questões de produto foram resolvidas; não há suposição em aberto nem ressalva pendente.

- Backend de produção: Grafana Stack via OTLP (Prometheus para métricas, Tempo para traces, Loki para logs). Reflete a evidência de `configs/testdata/insecure-prod/.env` e a stack já adotada no projeto.
- Burn-rate: esquema padrão do Google SRE Workbook com quatro alertas (14,4× / 6× / 3× / 1×) e janelas fixadas em RF-11, derivados do error budget de 0,1%.
- Sampling em produção: default 100% (baixo RPS torna o custo baixo e o sinal máximo), configurável para redução imediata via `OTEL_TRACE_SAMPLE_RATE` sob crescimento de volume; métricas sempre 100%.
- Saturação de runtime: nomes exatos são os emitidos por `go.opentelemetry.io/contrib/instrumentation/runtime`, enumerados no `observability/STANDARD.md` a partir dessa fonte oficial (RF-06); a especificação técnica apenas lista as séries, sem inventar nomes.
- Porte: pequeno/médio (1 API + 1 worker) é a fronteira intencional desta iniciativa; crescimento para múltiplos serviços/alto RPS (tail sampling/gateway) está explicitamente em Fora de Escopo, como reavaliação futura — não como lacuna.
- Roteamento de alertas: página (severidade alta) para canal de chat (Telegram/Slack); ticket (severidade baixa) para e-mail via SMTP/Resend já existente. Separação por severidade obrigatória (RF-21).
- Runbooks: um runbook dedicado por alerta que pagina, em `docs/runbooks/`; nenhuma página sem runbook (RF-22).
- Logs: correlacionados por trace context (`trace_id`/`span_id`) via `otelslog`, destino Loki (RF-23), completando os três sinais OTel.
- Collector: container Docker único no host (infra não-Kubernetes), recebendo OTLP de API e worker (RF-24).
