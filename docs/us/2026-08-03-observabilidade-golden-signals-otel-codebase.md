# US-OBS-001: Padrão de Observabilidade Production-Ready (Four Golden Signals + OpenTelemetry) para o codebase inteiro

## Declaração
Como engenheiro de plataforma responsável pelo on-call do MeControla, quero que o codebase inteiro (API, worker e módulos de domínio) exponha os Four Golden Signals ancorados nas Semantic Conventions do OpenTelemetry, com coleta, dashboards e alertas por sintoma governados por um padrão único e verificável, para diagnosticar e responder a incidentes de produção em minutos, com error budget explícito e sem depender de conhecimento tribal.

## Contexto
- Problema: a instrumentação existe, mas é desigual e não governada. A latência/tráfego/erros HTTP já são emitidos pelo provider (`http.server.request.duration` e counters auxiliares), o pool de banco já é instrumentado por `otelsql`, e há ~74 métricas de domínio e 412 spans nos 10 módulos — porém não há métricas de saturação de runtime (goroutines/heap/GC/CPU/RSS), só existem 3 dashboards e 3 arquivos de alerta pontuais, não há SLO/error budget nem alertas de burn-rate, não existe um documento de padrão (`observability/STANDARD.md`), não há topologia de Collector no repositório e o sampling está fixo em 100%. Em um incidente, o on-call não tem uma visão RED por rota, nem sinal de saturação, nem alerta ligado ao orçamento de erro.
- Resultado esperado: um padrão de telemetria auditável e implementado que (1) cobre os quatro sinais para API e worker, (2) fecha a lacuna de saturação de runtime dentro do pipeline OTLP, (3) introduz um Collector agent único como ponto de coleta, (4) padroniza sampling e cardinalidade, (5) entrega dashboards e alertas por sintoma derivados de SLO de 99,9% de disponibilidade e P95 < 500ms, e (6) documenta tudo em `observability/STANDARD.md` validado pelo script da skill.
- Fonte: pedido do usuário (evolução da observabilidade/dashboards do codebase inteiro segundo `.claude/skills/golden-signals-otel-standards`); decisões confirmadas por múltipla escolha (formato de história única de habilitação; persona primária on-call, secundárias dev e PO; SLO disponibilidade 99,9%; SLO latência P95 < 500ms; RED com histograma como fonte canônica e counters devkit como auxiliares documentados; saturação via OTel contrib runtime; Collector agent único; head sampling parent-based por ambiente); confronto com a base de código.

## Regras de Negócio
- RN-01 (Cobertura obrigatória dos quatro sinais): a API HTTP e o worker DEVEM expor Latência, Tráfego, Erros e Saturação. Latência SEMPRE por histograma/percentil (`histogram_quantile`), NUNCA por média simples.
- RN-02 (Latência canônica): a fonte de verdade de latência é o histograma `http.server.request.duration` (unidade `s`), já emitido com buckets `[0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1.0, 2.5, 5.0, 7.5, 10.0]` — idênticos aos buckets recomendados pelo padrão; esses buckets NÃO devem ser reduzidos nem substituídos.
- RN-03 (Tráfego e Erros derivados do histograma): tráfego e taxa de erro DEVEM ser calculados a partir da série `http_server_request_duration_seconds_count`, filtrando erros por `http.response.status_code=~"5.."`. Os counters não-canônicos emitidos pelo provider (`http.server.request.count`, `http.server.request.active`, `http.server.request.error.count`) permanecem como sinais auxiliares e DEVEM ser documentados como tais no STANDARD.md — não são a fonte primária de RED em dashboards/alertas.
- RN-04 (Latência separada por classe): dashboards e alertas DEVEM permitir separar latência de sucesso e de erro; um erro lento não pode ser mascarado pela média.
- RN-05 (Saturação de runtime): server e worker DEVEM emitir métricas de saturação do processo via instrumentação OTel de runtime (`go.opentelemetry.io/contrib/instrumentation/runtime`), cobrindo goroutines, heap e GC, transportadas pelo mesmo pipeline OTLP. O nome real das séries emitido pela stack DEVE ser confirmado e registrado no STANDARD.md antes de ser usado em alerta.
- RN-06 (Saturação de recursos limitantes): a saturação de banco continua derivada das métricas `db.client.connections.*` já emitidas por `otelsql`, e a saturação do outbox continua derivada de `outbox_lag_seconds` e `outbox_dead_letter_total` já existentes.
- RN-07 (Liveness do worker): o worker, que não possui superfície HTTP, DEVE emitir um sinal de heartbeat/atividade por OTLP que permita alerta de ausência (staleness) quando o processamento em background parar.
- RN-08 (SLO e error budget): o SLO de disponibilidade é 99,9% (error budget 0,1%) e o SLO de latência é P95 < 500ms, ambos em janela de 30 dias. Alertas que paginam humano DEVEM mapear a um sintoma percebido pelo usuário e, quando houver SLO, DEVEM usar burn-rate multi-janela em vez de threshold fixo arbitrário.
- RN-09 (Burn-rate multi-janela): o alerta de disponibilidade DEVE combinar janela curta e longa — página rápida com burn-rate alto (ex.: 14,4×) confirmado em 1h e 5m, e ticket lento com burn-rate menor (ex.: ~3–6×) em 6h e 30m — derivados do error budget de 0,1%.
- RN-10 (Alertas de causa não paginam): alertas de causa (pool de conexões cheio, saturação de runtime acima de 80%, lag de outbox) servem para diagnóstico/ticket e NÃO paginam humano sozinhos, salvo quando já representam sintoma iminente ao usuário.
- RN-11 (Cardinalidade controlada): nenhuma métrica pode carregar rótulo de alta cardinalidade (`user_id`, `request_id`, `correlation_id`/`correlation_key`, `category_id`, IDs de sessão). Rota HTTP DEVE usar `http.route` (padrão declarado), nunca a URL crua. Essa regra reforça e não flexibiliza R-TXN-004, R-WF-KERNEL-001.4 e R-AGENT-WF-001.5 já vigentes.
- RN-12 (Nomes só do semconv): nenhum nome de métrica ou atributo pode ser inventado fora do OpenTelemetry Semantic Conventions e das métricas de domínio já existentes; quando um sinal não tiver nome semconv (ex.: saturação de runtime), usar o nome real emitido pela instrumentação oficial e registrá-lo no padrão.
- RN-13 (Topologia de coleta): a coleta DEVE passar por um OpenTelemetry Collector agent único, com receivers OTLP em 4317 (gRPC) e 4318 (HTTP), processadores `batch` e `memory_limiter`, dimensionado para porte pequeno/médio (1 API + 1 worker). Métricas NÃO são amostradas.
- RN-14 (Sampling): traces usam head sampling `ParentBased(TraceIDRatio)` com ratio configurável por ambiente (mantém 100% fora de produção; ratio reduzido em produção), sem quebrar as métricas, que permanecem 100%. A configuração DEVE reutilizar `OTEL_TRACE_SAMPLE_RATE` já existente.
- RN-15 (Atributos de recurso obrigatórios): cada sinal DEVE portar `service.name`, `service.version` e `deployment.environment`, preservando o `service.instance.id` já injetado no bootstrap.
- RN-16 (Padrão auditável): o resultado DEVE ser materializado em `observability/STANDARD.md` mais os assets de coleta, alertas e queries PromQL, e DEVE passar em `scripts/validate-standard.py` com `SUCCESS`.
- RN-17 (Sem regressão de instrumentação): a evolução NÃO pode remover nem renomear métricas/spans de domínio já emitidos e consumidos pelos dashboards/alertas atuais (agent-gate, whatsapp, transactions).

## Critérios de Aceite
```gherkin
Cenário: On-call abre a visão RED por rota da API em produção
  Dado que a API roda com o provider de observabilidade habilitado por WithMetrics, WithTracing e WithOTelMetrics
  E que o histograma http.server.request.duration é exportado via OTLP para o Collector e daí ao backend Grafana
  Quando o engenheiro de plataforma abre o dashboard de visão de serviço da API
  Então ele vê latência P95 e P99 por http.route calculadas com histogram_quantile sobre http_server_request_duration_seconds_bucket
  E vê o tráfego como rate(http_server_request_duration_seconds_count) e a taxa de erro como a fração com http.response.status_code=~"5.."
  E consegue separar a latência de sucesso da latência de erro no mesmo painel

Cenário: On-call detecta saturação de runtime antes da degradação (fluxo alternativo)
  Dado que server e worker foram instrumentados com go.opentelemetry.io/contrib/instrumentation/runtime
  E que as séries de goroutines, heap e GC estão sendo exportadas por OTLP e seus nomes reais estão registrados no STANDARD.md
  Quando o número de goroutines ou o uso de heap cresce de forma sustentada acima do alvo definido
  Então o painel de saturação exibe a tendência por serviço e instância
  E o alerta de saturação abre um ticket de diagnóstico sem paginar humano, conforme a política de alerta por causa

Cenário: Burn-rate de disponibilidade dispara página em incidente agudo
  Dado o SLO de disponibilidade de 99,9% em 30 dias com error budget de 0,1%
  E alertas de burn-rate multi-janela derivados desse orçamento
  Quando a taxa de erro consome o orçamento a um burn-rate de 14,4x sustentado por 1h e confirmado em 5m
  Então um alerta de página é disparado mapeado ao sintoma "usuários recebendo erros"
  E o runbook correspondente é referenciado no alerta

Cenário: Degradação lenta de latência gera ticket sem falso alarme (fluxo alternativo)
  Dado o SLO de latência P95 < 500ms
  Quando o P95 de latência de sucesso permanece acima de 500ms de forma sustentada em janelas de 6h e 30m com burn-rate menor
  Então um alerta de ticket é aberto para investigação
  E picos isolados de menos de 5 minutos não disparam alerta, por exigência da janela curta combinada

Cenário: Worker sem superfície HTTP é coberto por liveness (fluxo alternativo)
  Dado que o worker não expõe servidor HTTP e processa o outbox em background
  E que o worker emite um sinal de heartbeat/atividade por OTLP
  Quando o processamento em background para e o heartbeat deixa de ser emitido pela janela configurada
  Então um alerta de ausência (staleness) é disparado indicando worker inativo
  E o lag do outbox (outbox_lag_seconds) e as dead-letters (outbox_dead_letter_total) confirmam o sintoma no dashboard

Cenário: Coleta passa pelo Collector agent único
  Dado um OpenTelemetry Collector agent configurado com receivers OTLP em 4317 e 4318, processadores batch e memory_limiter
  Quando API e worker exportam métricas, traces e logs
  Então os sinais transitam pelo Collector antes do backend
  E métricas não sofrem sampling enquanto traces seguem head sampling ParentBased por ambiente

Cenário: Tentativa de introduzir métrica com rótulo de alta cardinalidade é bloqueada (erro/bloqueio)
  Dado a regra de cardinalidade controlada do padrão e as regras vigentes R-TXN-004, R-WF-KERNEL-001.4 e R-AGENT-WF-001.5
  Quando uma mudança adiciona user_id, request_id, correlation_id ou category_id como rótulo de métrica, ou usa URL crua em vez de http.route
  Então a mudança é rejeitada na revisão por violar a política de cardinalidade
  E o autor é direcionado a usar atributos de baixa cardinalidade ou spans/exemplars para o dado de alta cardinalidade

Cenário: Nome de métrica inventado fora do semconv é recusado (erro/bloqueio)
  Dado a regra de que apenas nomes do OpenTelemetry Semantic Conventions ou métricas de domínio já existentes são permitidos
  Quando alguém propõe uma métrica com nome fora do semconv para um sinal que possui nome canônico
  Então a proposta é recusada e o nome canônico equivalente é exigido
  E a exceção só é aceita para sinais sem nome semconv (ex.: runtime), com o nome real registrado no STANDARD.md

Cenário: Padrão é validado automaticamente antes de ser considerado pronto (erro/bloqueio)
  Dado observability/STANDARD.md e observability/otel-collector.yaml gerados
  Quando o validador python3 scripts/validate-standard.py --standard observability/STANDARD.md --collector observability/otel-collector.yaml é executado
  E algum sinal está ausente, um nome está fora do semconv ou falta uma chave obrigatória do Collector
  Então o script falha e a lacuna apontada precisa ser corrigida
  E o padrão só é aceito quando o script retorna SUCCESS

Cenário: Dashboards e métricas de domínio existentes continuam funcionando (fluxo feliz de não-regressão)
  Dado os dashboards atuais mecontrola-agent-gate-posdeploy, mecontrola-observabilidade-whatsapp e transactions-overview
  E os alertas atuais de transactions, agent-gate e whatsapp-dead-letter
  Quando a evolução de observabilidade é aplicada
  Então nenhuma métrica ou span de domínio consumido por esses artefatos é removido ou renomeado
  E os dashboards e alertas existentes continuam resolvendo suas séries sem quebra
```

## Dados e Permissões
- Dados obrigatórios: SLO de disponibilidade 99,9% e SLO de latência P95 < 500ms (janela de 30 dias); atributos de recurso `service.name`, `service.version`, `deployment.environment`, `service.instance.id`; endpoint e protocolo OTLP (`OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_EXPORTER_OTLP_INSECURE`) e taxa de sampling (`OTEL_TRACE_SAMPLE_RATE`) já existentes na configuração.
- Perfis/permissões: persona primária — engenheiro de plataforma/on-call (consome dashboards, recebe páginas/tickets, edita alertas e Collector). Personas secundárias — desenvolvedor do produto (instrumenta features novas seguindo o STANDARD.md e consulta traces/métricas por módulo) e dono de produto/negócio (consome a visão de confiabilidade e SLO). Acesso de escrita ao repositório para os assets em `observability/`, `docs/dashboards/`, `docs/alerts/` e `docs/runbooks/`; acesso ao backend Grafana/OTLP para importar dashboards e configurar contatos de alerta.

## Dependências
- Provider de observabilidade OTel do devkit-go v0.5.5 (`github.com/JailtonJunior94/devkit-go`), que já emite o histograma HTTP e expõe o hook `HTTP()` (`otel.NewProvider`), verificado no module cache.
- Backend de telemetria compatível com OTLP (Grafana/Prometheus/Tempo/Loki), evidenciado pela configuração de exportador OTLP; o exporter final de produção deve ser confirmado antes do deploy do Collector.
- Biblioteca `go.opentelemetry.io/contrib/instrumentation/runtime` a ser adicionada ao `go.mod` para a saturação de runtime (nova dependência).
- Skill `golden-signals-otel-standards` (padrão e `scripts/validate-standard.py`) para materializar e validar o STANDARD.md e os assets.
- Skill de dashboards (`otel-grafana-dashboards`) para gerar o JSON dos novos dashboards — esta história especifica os sinais e queries, mas a geração do JSON é delegada, pois o padrão golden-signals não gera dashboard.
- Regras de governança de cardinalidade já vigentes: R-TXN-004 (`.claude/rules/transactions-workflows.md`), R-WF-KERNEL-001.4 (`.claude/rules/workflow-kernel.md`) e R-AGENT-WF-001.5 (`.claude/rules/agent-workflows-tools.md`).

## Fora de Escopo
- Geração do JSON dos dashboards Grafana neste artefato (delegada à skill `otel-grafana-dashboards`/`otel-hybrid-dashboard-blueprint`; o padrão golden-signals explicitamente não gera JSON de dashboard).
- Instrumentação não-OTel e monitoramento exclusivo de infraestrutura de host/nó (CPU/RSS do host via node_exporter foi descartado na decisão; a saturação coberta é a de runtime do processo via OTel).
- Tail sampling e topologia de Collector gateway/balanceado de grande porte (acima da necessidade de 1 API + 1 worker).
- Mudança de SLO por endpoint individual ou SLOs de negócio por funcionalidade (esta história fixa SLO de serviço; refinamentos por rota ficam para iteração futura).
- Alteração do código do provider devkit-go para renomear os counters HTTP auxiliares ao semconv (fora do controle do repositório; tratados como auxiliares documentados).
- Setup de ingestão específico de vendor além do destino OTLP genérico.

## Evidências
- Entrada: pedido do usuário para evoluir observabilidade/dashboards do codebase inteiro segundo `.claude/skills/golden-signals-otel-standards`, production-ready, sem inventar resposta; respostas de múltipla escolha que fixaram história única de habilitação, personas, SLOs (99,9% / P95 < 500ms) e as decisões técnicas de RED, saturação, coleta e sampling.
- Base de código:
  - Provider e habilitação de telemetria: `cmd/server/server.go:75` (`otel.NewProvider`) e `cmd/server/server.go:106-117` (`httpserver.WithMetrics()`, `WithTracing()`, `WithOTelMetrics()`); worker em `cmd/worker/worker.go:80` e `migrate` em `cmd/migrate/migrate.go:232` usam o mesmo provider.
  - Métrica de latência HTTP e buckets idênticos ao padrão: devkit `pkg/observability/otel/http.go:15-18` (nomes `http.server.request.duration`, `http.server.request.count`, `http.server.request.active`, `http.server.request.error.count`) e `pkg/observability/otel/http.go:63-84` (buckets `[0.005…10.0]`, atributos `http.request.method`, `http.route`, `http.response.status_code`); middleware que aciona o hook em `pkg/http_server/chi_server/middleware.go:110-149` e `server.go:142-147`.
  - Saturação de banco já instrumentada: `internal/platform/database/postgres/otelsql.go:12-29` (`otelsql.Open` + `RegisterDBStatsMetrics` → `db.client.connections.*`).
  - Configuração OTLP e defaults: `configs/config.go:307-314` (`O11yConfig`) e `configs/config.go:709-710` (`OTEL_EXPORTER_OTLP_PROTOCOL=grpc`, `OTEL_EXPORTER_OTLP_INSECURE=true`); `configs/testdata/valid/.env:15` e `configs/testdata/insecure-prod/.env:15` evidenciam destino OTLP (`localhost:4317` e `otlp.grafana.net`).
  - Instrumentação de domínio existente (não regredir): ~74 emissões de métrica via `o11y.Metrics().Counter/Histogram/Gauge` distribuídas em agents (15), platform (16), onboarding (14), budgets (10), identity (6), billing (4), categories (4), card (3), transactions (2); 412 spans via `o11y.Tracer().Start` nos 10 módulos; outbox instrumentado em `internal/platform/outbox/dispatcher.go:59-64` (`outbox_dead_letter_total`, `outbox_lag_seconds`).
  - Dashboards atuais (3): `docs/dashboards/mecontrola-agent-gate-posdeploy.json`, `docs/dashboards/mecontrola-observabilidade-whatsapp.json`, `docs/dashboards/transactions-overview.json`.
  - Alertas atuais por sintoma (3): `docs/alerts/mecontrola-agent-gate-posdeploy.yaml`, `docs/alerts/transactions.yaml`, `docs/alerts/whatsapp-dead-letter.yaml` (ex.: `FinancialWriteFalseSuccess`, `OutboxDeadLetter`, `OutboxLagP95High`).
  - Regras de cardinalidade vigentes: `.claude/rules/transactions-workflows.md` (R-TXN-004), `.claude/rules/workflow-kernel.md` (R-WF-KERNEL-001.4), `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001.5).
- Inferências: porte pequeno/médio (uma API + um worker num único serviço) inferido da estrutura `cmd/server` + `cmd/worker` e do único módulo de aplicação — determina Collector agent único e head sampling; o backend Grafana/OTLP é a leitura mais provável a partir da config de exportador, a ser confirmado no deploy.
- Não evidenciado: métricas de saturação de runtime/processo (busca por `contrib/instrumentation/runtime`, `runtime.Start`, `go_goroutines`, `process_resident_memory`, `MemStats`, `NumGoroutine` no repo e no devkit retornou vazio — lacuna real); documento de padrão `observability/STANDARD.md` (busca por `STANDARD.md` fora de skills retornou vazio); configuração de OpenTelemetry Collector no repositório (busca por `otel-collector`/`4317`/`otel-lgtm` em compose/deployments retornou vazio); alertas de burn-rate de disponibilidade/latência e alerta de liveness do worker (ausentes nos 3 arquivos de alerta atuais); dashboards de RED-por-rota HTTP, de banco e de saturação (ausentes nos 3 dashboards atuais).

## Notas de Validação
- Os quatro Golden Signals estão endereçados: Latência (RN-02/RN-04), Tráfego e Erros (RN-03), Saturação (RN-05/RN-06/RN-07).
- Latência sempre por histograma/percentil, nunca por média (RN-01/RN-02), conforme exigência crítica da skill.
- Nenhum nome de métrica/atributo inventado: os nomes usados ou já existem no código (verificados por caminho/linha) ou pertencem ao semconv/instrumentação oficial; o único sinal sem nome semconv (runtime) exige registro do nome real antes do uso (RN-05/RN-12).
- Nenhum JSON de dashboard é gerado aqui; a geração é delegada e listada em Dependências/Fora de Escopo, respeitando a fronteira da skill.
- Cardinalidade e sampling documentados e alinhados às regras vigentes do repositório; a história reforça, não flexibiliza, R-TXN-004, R-WF-KERNEL-001.4 e R-AGENT-WF-001.5.
- Cenários cobrem fluxo feliz (RED por rota; não-regressão), fluxos alternativos (saturação de runtime; degradação lenta; worker por liveness; coleta via Collector) e erros/bloqueios (alta cardinalidade; nome fora do semconv; falha na validação do padrão).
- Decisões materiais foram resolvidas por múltipla escolha (formato, personas, SLOs, RED, saturação, coleta, sampling); não há marcadores pendentes nem ressalvas em aberto.
