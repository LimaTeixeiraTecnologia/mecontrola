---
name: golden-signals-otel-standards
description: 'Define o padrão de telemetria de um serviço combinando os Four Golden Signals do Google SRE (Latência, Tráfego, Erros, Saturação) com os padrões oficiais do OpenTelemetry (Specification, OTLP e Semantic Conventions): mapeia cada sinal para instrumentos e métricas canônicas, instrumentação de SDK (Go como referência, com trechos Node.js, Python e Java), topologia de coleta via OpenTelemetry Collector, sampling, cardinalidade, custo e alertas/SLO por sintoma. Produz um documento STANDARD.md e assets prontos: config do Collector por porte, bootstrap de SDK, regras de alerta e queries PromQL. Usar ao definir estratégia de observabilidade, padronizar instrumentação OTel, mapear Golden Signals, planejar coleta OTLP ou dimensionar telemetria para aplicações de pequeno, médio e grande porte. Não usar para gerar JSON de dashboard (delegar a otel-grafana-dashboards ou otel-hybrid-dashboard-blueprint), instrumentação não-OTel, monitoramento só de infraestrutura ou setup de ingestão de vendor específico.'
---

# Padrão de Telemetria: Four Golden Signals + OpenTelemetry

Definir o padrão oficial de telemetria de um serviço, ancorado nas fontes primárias — Google SRE Book (Monitoring Distributed Systems) e OpenTelemetry Specification, OTLP e Semantic Conventions —, mapeando os Four Golden Signals para sinais e instrumentos OTel, dimensionando coleta, sampling e custo por porte, e materializando um documento de padrão auditável com assets prontos para produção.

<critical>Todos os artefatos entregues DEVEM ser escritos em PT-BR. Nomes de métricas, atributos, chaves de configuração e identificadores de código mantêm a grafia técnica original.</critical>
<critical>NÃO inventar nomes de métricas ou atributos. Usar exclusivamente OpenTelemetry Semantic Conventions. Quando o usuário pedir uma métrica fora do semconv, recusar e sugerir o nome canônico equivalente.</critical>
<critical>Latência SEMPRE via histograma e percentil (`histogram_quantile`). NUNCA usar média simples para latência — uma métrica de média mascara a cauda (tail latency).</critical>
<critical>Esta skill NÃO gera JSON de dashboard. Ao detectar pedido de dashboard, delegar para `otel-grafana-dashboards` ou `otel-hybrid-dashboard-blueprint`.</critical>
<critical>Toda recomendação factual (definições dos sinais, portas OTLP, buckets, nomes de métricas) deve corresponder às fontes oficiais citadas nos arquivos de `references/`. Não afirmar números ou nomes sem lastro.</critical>

## Passo 1: Coletar Contexto do Serviço (OBRIGATÓRIO)

Antes de qualquer geração, apresentar o formulário abaixo e aguardar resposta. Não inferir nem usar defaults silenciosamente. Apresentar cada item como múltipla escolha.

---

**Para definir o padrão de telemetria, responda:**

**1. Porte da aplicação**
- [ ] A) Pequeno (1 serviço, < 100 RPS, time enxuto)
- [ ] B) Médio (poucos serviços, 100–1.000 RPS)
- [ ] C) Grande (muitos serviços/microsserviços, > 1.000 RPS, multi-time)

**2. Tipo do serviço**
- [ ] A) API REST
- [ ] B) API gRPC
- [ ] C) Worker / Consumer (Kafka, SQS, RabbitMQ)
- [ ] D) Híbrido (REST + Worker)
- [ ] E) Outro: ___

**3. Linguagem principal**
- [ ] A) Go
- [ ] B) Node.js / TypeScript
- [ ] C) Python
- [ ] D) Java / JVM
- [ ] E) Outra: ___

**4. Plataforma de execução**
- [ ] A) Kubernetes
- [ ] B) AWS ECS
- [ ] C) VM / Bare Metal
- [ ] D) Serverless (Lambda, Cloud Run)
- [ ] E) Outra: ___

**5. Volume aproximado de requisições**
- [ ] A) Baixo (< 100 RPS)
- [ ] B) Médio (100–1.000 RPS)
- [ ] C) Alto (1.000–10.000 RPS)
- [ ] D) Muito alto (> 10.000 RPS)

**6. SLO alvo de disponibilidade**
- [ ] A) 99,0% | B) 99,5% | C) 99,9% | D) 99,95% | E) 99,99%

**7. SLO alvo de latência (P95)**
- [ ] A) < 100ms | B) < 300ms | C) < 500ms | D) < 1s | E) Outro: ___

**8. Dependências externas** _(marque todas)_
- [ ] A) Banco relacional | B) NoSQL | C) Cache | D) Fila/Stream | E) API upstream | F) Nenhuma

**9. Backend de telemetria (destino OTLP)**
- [ ] A) Grafana Stack (Prometheus + Tempo + Loki)
- [ ] B) Coralogix
- [ ] C) Outro backend compatível com OTLP
- [ ] D) Ainda indefinido

**Campo aberto:** Nome do serviço (`service.name`): ___

---

Campos obrigatórios: porte (1), tipo (2), linguagem (3), SLO (6 e 7) e nome do serviço. Se algum faltar, reapresentar apenas os itens faltantes no mesmo formato.

## Passo 2: Mapear os Four Golden Signals para OpenTelemetry

Ler `references/golden-signals-otel-mapping.md` para a tabela canônica que liga cada sinal (Latência, Tráfego, Erros, Saturação) ao sinal OTel, ao instrumento, à métrica de Semantic Convention e à fórmula PromQL, com as definições do SRE Book.

1. Para cada um dos quatro sinais, registrar: métrica OTel canônica, atributos obrigatórios do semconv e query PromQL.
2. Separar latência de sucesso e de erro (um erro lento é pior que um erro rápido).
3. Usar os buckets estáveis de `http.server.request.duration` documentados na referência; não inventar buckets.
4. Definir saturação a partir de métricas de runtime/host reais (CPU, memória, filas, pool de conexões), lembrando que latência crescente é indicador antecipado de saturação.

## Passo 3: Definir a Instrumentação por Linguagem

Ler `references/sdk-instrumentation.md`. Usar `assets/sdk-bootstrap-go.md` como referência principal (Go completo) e `assets/sdk-bootstrap-polyglot.md` para Node.js, Python e Java.

1. Garantir `service.name` (obrigatório) e os atributos de recurso `service.version` e `deployment.environment`.
2. Decidir entre auto-instrumentação (middleware/agentes) e instrumentação manual, priorizando bibliotecas oficiais que já emitem métricas de semconv.
3. Habilitar os três sinais quando aplicável: métricas (obrigatório para os Golden Signals), traces (correlação e causa) e logs (correlacionados por trace context).

## Passo 4: Projetar a Coleta e o Transporte (OTLP + Collector)

Ler `references/collector-otlp-topologies.md`. Escolher a topologia pelo porte definido no Passo 1 e selecionar o asset correspondente: `assets/otel-collector.small.yaml`, `.medium.yaml` ou `.large.yaml`.

1. Padronizar OTLP: gRPC na porta 4317 e HTTP na 4318.
2. Aplicar `batch` sempre; adicionar `memory_limiter`, retry com backoff e filas conforme o porte.
3. Pequeno porte: agent único. Médio: agent com memory_limiter e fila. Grande: gateway com tail sampling e balanceamento.

## Passo 5: Definir Sampling, Cardinalidade e Custo

Ler `references/sampling-cost-efficiency.md`.

1. Escolher a estratégia de sampling por porte: head sampling (pequeno/médio) ou tail sampling no gateway (grande).
2. Manter métricas sem sampling (são agregadas e baratas); aplicar sampling apenas a traces.
3. Controlar cardinalidade: proibir atributos de alta cardinalidade em métricas (`user_id`, `request_id`, IDs de sessão). Preferir `http.route` a URL crua.
4. Documentar o impacto de custo e as decisões de economia no STANDARD.md.

## Passo 6: Definir Alertas e SLO Orientados a Sintomas

Ler `references/alerting-slo.md` e usar `assets/alert-rules.yaml` como base.

1. Alertar por sintoma (o que está quebrado para o usuário), não por causa isolada.
2. Derivar o error budget do SLO informado no Passo 1.
3. Configurar alertas de burn rate multi-janela (janela curta + longa) para reduzir ruído e detecção tardia.
4. Preencher thresholds a partir dos SLOs de disponibilidade e latência do Passo 1.

## Passo 7: Gerar as Saídas

1. Ler `assets/STANDARD.template.md` e preencher todas as seções com as decisões dos Passos 1–6. Escrever em `observability/STANDARD.md`.
2. Instanciar os assets em `observability/`:
   - `otel-collector.yaml` (a partir do porte escolhido)
   - `alert-rules.yaml` (com thresholds preenchidos)
   - `promql-golden-signals.md` (queries dos quatro sinais)
   - bootstrap de SDK da linguagem principal
3. Produzir um resumo: sinais cobertos, porte, topologia de coleta, estratégia de sampling e decisões de custo.

## Passo 8: Validar

1. Executar `python3 scripts/validate-standard.py --standard observability/STANDARD.md --collector observability/otel-collector.yaml`.
2. Se falhar, ler o stderr, corrigir a lacuna apontada (sinal ausente, métrica fora do semconv, chave de collector faltante) e revalidar até `SUCCESS`.

## Checklist Final (Crítico)

- [ ] Os quatro Golden Signals estão mapeados para métricas canônicas de semconv
- [ ] Latência usa histograma/percentil, nunca média
- [ ] `service.name` e atributos de recurso definidos
- [ ] Topologia de Collector coerente com o porte (OTLP 4317/4318, batch, memory_limiter conforme porte)
- [ ] Estratégia de sampling e regras de cardinalidade documentadas
- [ ] Alertas por sintoma + burn rate multi-janela derivados do SLO
- [ ] Nenhum nome de métrica/atributo inventado fora do semconv
- [ ] Nenhum JSON de dashboard gerado (delegado quando solicitado)
- [ ] `validate-standard.py` retornou `SUCCESS`

## Tratamento de Erros

* **Contexto obrigatório ausente:** Não gerar o padrão. Reapresentar apenas os campos faltantes do Passo 1 no formato de múltipla escolha.
* **Métrica/atributo fora do semconv:** Recusar e sugerir o nome canônico de `references/golden-signals-otel-mapping.md`.
* **Pedido de dashboard:** Não gerar JSON. Explicar a fronteira e delegar para `otel-grafana-dashboards` ou `otel-hybrid-dashboard-blueprint`.
* **Backend de telemetria indefinido:** Assumir destino OTLP genérico (4317/4318) e avisar que o exporter final deve ser confirmado antes do deploy.
* **Falha na validação:** Ler o stderr de `scripts/validate-standard.py`, corrigir a lacuna específica e revalidar.
* **Linguagem fora do escopo poliglota:** Entregar o padrão e as configs de Collector (agnósticas) e sinalizar que o bootstrap de SDK deve seguir a documentação oficial da linguagem, mantendo os mesmos atributos de recurso e métricas de semconv.
