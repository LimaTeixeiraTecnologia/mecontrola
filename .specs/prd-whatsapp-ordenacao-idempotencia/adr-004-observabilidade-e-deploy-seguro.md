# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Observabilidade do caminho crítico e deploy seguro (anti-storm)
- **Data:** 2026-07-01
- **Status:** Aceita
- **Decisores:** time de plataforma (autor), owner do produto (decisões D-04/D-05 do PRD)
- **Relacionados:** PRD (RF-13/14/15/16), techspec `techspec.md`, ADR-001..003,
  regras R-TXN-004, R-WF-KERNEL-001.4, `observability.md`, `graceful-lifecycle.md`

## Contexto

Reconciliação de dois falsos-positivos do diagnóstico inicial (confirmados por pesquisa de código):

- **Traces existem** no caminho crítico: `whatsapp.handler.inbound` (`inbound_handler.go:28`),
  `whatsapp.dispatcher.route`, `agent.runtime.execute` (`runtime.go:69`), `llm.complete`
  (`openrouter.go:91`), `workflow.engine.*`. Sumiram no Tempo porque produção usa
  `OTEL_TRACE_SAMPLE_RATE=0.1` (sampler probabilístico → 90% descartado).
- **Métrica de conflito existe:** `workflow_version_conflict_total` (label `workflow`,
  `engine.go:46/57/463`), mas só incrementa em conflito **CAS no Save**. O bug de onboarding falha
  antes, no **INSERT** do índice único parcial (caminho diferente) → nunca incrementa. Logo o conflito
  de Start é invisível hoje.
- `OTEL_SERVICE_VERSION` cai no default `"dev"` (não setado no compose) → telemetria não reflete o
  binário; produção observou label `d44fc9d` divergente do binário `c95cfdb`.
- Deploy Swarm: `update_config` `parallelism:1`, `delay:20s`, `order:stop-first`, `failure_action:pause`,
  **sem `stop_grace_period`** (default 10s < 15s de shutdown do app). O incidente teve **deploy storm**
  (4 tags em ~27 min) drenando runs em voo.

Restrição: cardinalidade controlada (R-TXN-004 / R-WF-KERNEL-001.4) — sem `user_id`/`correlation_key`/
`category_id` em label; retenção 30d na stack otel-lgtm (memória `project_observability_otel_lgtm`).

## Decisão

1. **Amostragem parent-based com raiz sempre-amostrada no caminho inbound** para o percurso
   `webhook → agente → LLM → envio` ser observável fim-a-fim sem elevar o custo global para 100%. Não
   criar spans novos (já existem). **Propagar `traceparent` (W3C) no `metadata` do `outbox_events`** na
   publicação e restaurá-lo no consumer, costurando o hop assíncrono server→worker num **único trace**
   (decisão travada) — sem isso o worker iniciaria um trace separado. Producer/consumer permanecem
   adapters finos (R-ADAPTER-001).
2. **Observar o conflito de Start** (ADR-003): expor outcome `resumed_on_conflict` (label de outcome
   em `workflow_runs_total`/`workflow_resume_total`, cardinalidade controlada) quando o Start
   idempotente-resume retomar um run por unique_violation. `workflow_version_conflict_total` permanece
   para o caminho CAS.
3. **Corrigir `OTEL_SERVICE_VERSION=${IMAGE_TAG}`** nos 4 serviços (server-1/2, worker-1/2) no
   `compose.swarm.yml`.
4. **Deploy seguro:** adicionar `stop_grace_period` ≥ shutdown do app (ex.: 30s) a server e worker,
   garantindo drain cooperativo (o scheduler já espera in-flight via `allowWg.Wait()`); e **evitar
   deploy storm** — política de release que serializa/consolida deploys (não publicar múltiplas tags
   em minutos). Reaper `STUCK_AFTER=5m` permanece como rede de segurança.
5. **Métricas de ordenação/idempotência** (novas, cardinalidade controlada): lag
   `occurred_at → published_at` (p95), duplicidade de escrita (=0), outbound vazio (=0), reivindicações
   adiadas por "usuário em voo".

## Alternativas Consideradas

1. **Elevar `OTEL_TRACE_SAMPLE_RATE` global para 1.0:** simples, mas custo de storage no otel-lgtm
   cresce ~10x; rejeitada — parent-based na raiz inbound dá a visibilidade necessária sem 100% global.
2. **Tail-based sampling (coletor):** melhor seletividade (amostrar só traces com erro/lentidão), mas
   exige coletor com processador tail e mais operação; reservada como evolução se o parent-based não
   bastar.
3. **Contador dedicado para conflito de Start** (`workflow_start_conflict_total`): possível, mas um
   outcome em métrica existente evita proliferação; escolhido o outcome label.
4. **`order: start-first` no deploy:** reduz janela de indisponibilidade por serviço, mas com 1
   réplica por serviço exige rodar 2 tasks transitórias (recursos no nó único). **Rejeitada** (decisão
   travada): mantém-se `order: stop-first` (Caddy roteia para o outro server) + `stop_grace_period 30s`
   + gate de CI anti-storm — ataca a causa (storm), sem custo extra de recursos.
5. **Contador dedicado para lag/idempotência vs. reuso de séries existentes:** optou-se por novas
   métricas dedicadas de lag `occurred_at→published_at`, duplicidade e outbound-vazio, todas com
   cardinalidade controlada.

## Consequências

### Benefícios Esperados

- Caminho inbound rastreável fim-a-fim → MTTR de incidentes cai de horas para minutos.
- Conflito de Start e lag/idempotência tornam-se mensuráveis e alertáveis.
- Telemetria com versão correta; deploy sem drenar runs em massa → sem storm.

### Trade-offs e Custos

- Mais volume de traces do inbound (custo de storage 30d) — dimensionar a taxa da raiz.
- Ajuste de deploy exige mudança no `compose.swarm.yml` e disciplina de release.

### Riscos e Mitigações

- **Risco:** parent-based ainda perder traces se a raiz não for corretamente marcada. **Mitigação:**
  validar propagação de contexto do handler até o worker (o hop de outbox quebra o trace — o worker
  inicia novo trace; considerar propagar `traceparent` no `metadata` do evento para linkar).
  **Rollback:** reverter para taxa fixa.
- **Risco:** `stop_grace_period` alto atrasa deploy. **Mitigação:** 30s é suficiente e proporcional.

## Plano de Implementação

1. Configurar sampler parent-based (provider OTel em `cmd/server`/`cmd/worker`); opcional: propagar
   `traceparent` no `metadata` do evento outbox para costurar o trace através do hop assíncrono.
2. Adicionar outcome `resumed_on_conflict` (cardinalidade controlada).
3. `compose.swarm.yml`: `OTEL_SERVICE_VERSION=${IMAGE_TAG}` + `stop_grace_period: 30s` nos 4 serviços.
4. Novas métricas de lag/idempotência/outbound-vazio.
5. Política de release anti-storm (CI/CD) documentada em runbook.

Concluído quando: CA-06 verde; traces do inbound visíveis no Tempo; `OTEL_SERVICE_VERSION` == binário.

## Monitoramento e Validação

- Dashboards Grafana (otel-lgtm): trace do inbound fim-a-fim, lag p95, `onboarding_error`,
  `resumed_on_conflict`, outbound vazio. Alertas: lag p95 > 30s, duplicidade > 0, outbound vazio > 0.

## Impacto em Documentação e Operação

- Runbooks de deploy (anti-storm, grace period) e de observabilidade; dashboards; `.env.example`
  (documentar `OTEL_SERVICE_VERSION`); config do sampler.

## Revisão Futura

- Revisar para tail-based sampling se o custo do parent-based inbound crescer; revisar a política de
  deploy ao adicionar réplicas (fase de escala).
