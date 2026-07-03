# Observabilidade — SPOF Single-Node e Retenção de Sinais

Decisão: D-01 e D-03 do PRD `prd-infra-evolucao-kvm2-10k` (2026-07-03).

## SPOF de Observabilidade Aceito

A stack de observabilidade (`grafana/otel-lgtm`) roda em container único no host KVM 2 (single-node Docker Swarm). Isso representa um **Single Point of Failure (SPOF)** aceito para o envelope B (10k ativos/dia):

- Indisponibilidade do `otel-lgtm` não bloqueia a aplicação (falha silenciosa de exportação OTLP).
- Perda de sinais durante indisponibilidade do container é tolerada (não é dado financeiro).
- Alta disponibilidade de observabilidade (segundo nó) fica fora do escopo do envelope B — é envelope C.
- Mitigação operacional: reinício automático via política `on-failure` do Swarm.

## Arquitetura de Sampling (RF-11, RF-12)

O trace sampling opera em duas camadas:

**Camada 1 — SDK (serviços da aplicação)**
- `OTEL_TRACE_SAMPLE_RATE=1` nos 4 serviços (server-1, server-2, worker-1, worker-2).
- O SDK envia 100% dos spans ao OTel Collector (otel-lgtm).
- Valor igual em `compose.swarm.yml` e `deployment/config/prod.env` — sem divergência.

**Camada 2 — OTel Collector (`otelcol-config.yaml`, processor `tail_sampling`)**
- `errors-policy`: 100% dos traces com span de status ERROR são retidos.
- `probabilistic-policy`: 10% dos traces sem erro são retidos.
- `decision_wait: 10s`: decisão de sampling após 10 s da chegada do primeiro span.
- Taxa efetiva de armazenamento: ~10% do volume total, com cobertura 100% de erros.

Este design garante que incidentes (traces com erro) são sempre visíveis, enquanto o volume de traces normais é reduzido em ~90%, aliviando CPU e storage do host.

## Retenção de Sinais (30 dias)

Todos os sinais têm retenção mínima de **30 dias** na stack local:

| Sinal | Backend | Retenção |
|-------|---------|----------|
| Métricas | Prometheus (Mimir embutido no otel-lgtm) | 30 d |
| Logs | Loki (embutido no otel-lgtm) | 30 d |
| Traces | Tempo (embutido no otel-lgtm) | 30 d |

Verificar configuração de retenção em:
- `deployment/telemetry/grafana/loki-config.yaml` (campo `retention_period`)
- `deployment/telemetry/grafana/tempo-config.yaml` (campo `max_block_duration` / `block_retention`)
- `deployment/telemetry/grafana/prometheus.yaml` (flag `--storage.tsdb.retention.time=30d`)

## Gatilho de Escalonamento

Caso o host atinja os limites de envelope B (conforme REQ-05/RF-16), avaliar:
- Migração de observabilidade para Grafana Cloud (elimina SPOF e pressão de storage).
- Upgrade KVM 2 → KVM 4 para absorver crescimento de volume de sinais.

## Referências

- PRD: `.specs/prd-infra-evolucao-kvm2-10k/prd.md` (D-01, D-03, RF-11, RF-12, RF-13)
- Techspec: `.specs/prd-infra-evolucao-kvm2-10k/techspec.md` (REQ-04)
- Config collector: `deployment/telemetry/grafana/otelcol-config.yaml`
- Gate CI: `scripts/ci/deploy-anti-storm.sh` (checks [5/6] e [6/6])
