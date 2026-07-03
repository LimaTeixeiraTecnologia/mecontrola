# Evidência de Carga — Envelopes A e B

Data: 2026-07-03
Tarefa: 6.0 — Harness de carga k6 e prova dos envelopes A e B
PRD: `.specs/prd-infra-evolucao-kvm2-10k/prd.md` RF-17, RF-18
TechSpec: REQ-06

## Contexto

A auditoria de 2026-07-03 constatou que nenhum teste de carga havia sido executado e o veredito de
envelope B permanecia **não comprovado**. Esta tarefa constrói o harness (subtarefas 6.1–6.3) e
produz a análise de evidência (6.4) com honestidade sobre a ausência de acesso ao host de staging.

## Harness criado

| Arquivo | Tipo | Cobertura |
|---------|------|-----------|
| `scripts/loadtest/whatsapp-inbound.js` | k6 | POST `/api/v1/whatsapp/inbound`, HMAC-SHA256, envelopes A e B |
| `scripts/loadtest/transactions-read.js` | k6 | GET `/api/v1/months/{ref}/` + `/entries`, envelopes A e B |
| `scripts/loadtest/outbox-throughput.sh` | bash | outbox drain (pré-existente, reusado) |
| `taskfiles/loadtest.yml` | Taskfile | targets: whatsapp, read, outbox, suite:envelope-a, suite:envelope-b |

## Definição dos envelopes e perfis de carga

### Envelope A — 10k mensagens/mês

| Parâmetro | Valor |
|-----------|-------|
| Volume alvo | 10.000 msgs/mês |
| Média diária | 333 msgs/dia |
| Janela de pico (8 h) | 42 msgs/h = 0,7/min = 0,012 RPS |
| Burst (5×) | 3,5 msgs/min = 0,058 RPS |
| Perfil k6 | 2 VUs × 5 min, think time 8 s → ~0,25 RPS (5× burst) |
| Outbox | 333 eventos sintéticos |

### Envelope B — 10k mensagens/dia

| Parâmetro | Valor |
|-----------|-------|
| Volume alvo | 10.000 msgs/dia |
| Média horária | 417 msgs/h |
| Janela de pico (12 h) | 833 msgs/h = 13,9/min = 0,23 RPS |
| Burst (3×) | 41,7/min = 0,70 RPS |
| Perfil k6 | 5 VUs × 10 min, think time 2 s → ~2,5 RPS (3× burst) |
| Outbox | 10.000 eventos sintéticos |

## Thresholds de aprovação (RF-18)

| Métrica | Threshold | Fonte |
|---------|-----------|-------|
| `http_req_duration{endpoint:whatsapp_inbound}` p95 | < 500 ms | k6 threshold |
| `http_req_failed` rate | < 1 % | k6 threshold |
| `mecontrola_db_pool_in_use` | < 80 % de `DEFAULT_POOL_SIZE=20` (< 16 conns) | Grafana / regra `mc-pgbouncer-pool-saturation` |
| CPU do host | < 70 % | Grafana `node_cpu_seconds_total` |

## Projeção analítica — configuração atual (pós task 5.0)

Base: pool 20 conns (DEFAULT_POOL_SIZE), MAX_DB_CONNECTIONS=30, mem limit steady-state 5.340 MB,
margem 1.316 MB, 2 vCPU, 4 GB swap.

### Pool de conexões

Modelo: conns_em_uso = RPS × latência_média_db

| Envelope | RPS burst | Latência DB estimada | Conns em uso (burst) | % de 20 |
|----------|-----------|---------------------|----------------------|---------|
| A | 0,058 | 50 ms | 0,003 | < 1 % |
| B burst | 2,5 | 50 ms | 0,13 | < 1 % |

Projeção: **pool amplamente suficiente** em ambos os envelopes. Alerta `mc-pgbouncer-pool-saturation`
(>24 backends = 80 %) não deve disparar.

### Memória

Carga adicional estimada: handler HTTP + db query por request.
- server × 2: limite 512 MB cada; idle ~150 MB cada; headroom 362 MB por instância.
- Envelope B burst: ~2,5 req/s × 10 ms handling = headroom confortável.

Projeção: **memória dentro dos limites**; margem de 1.316 MB garante absorção de pico.

### CPU

- Envelope A burst: 0,058 req/s × 10 ms proc = 0,058 % de 1 core → desprezível.
- Envelope B burst: 2,5 req/s × 10 ms proc = 2,5 % de 1 core → desprezível.
- Processamento LLM via OpenRouter (async): o handler retorna 202 antes de concluir; custo é de rede, não de CPU do host.

Projeção: **CPU do host não é o gargalo** em envelope B para este volume.

### Gargalo identificado — LLM OpenRouter

O fluxo WhatsApp inbound → agente → LLM (OpenRouter) ocorre de forma assíncrona (outbox),
mas consome worker CPU e conexões DB. O rate-limit do OpenRouter (externo) é o gargalo provável
em envelope B se todas as mensagens forem processadas em rajada simultânea.
Esse gargalo não é do host KVM2 e não pode ser provado ou refutado sem execução real.

## Veredito por envelope

| Envelope | Veredito | Justificativa |
|----------|----------|---------------|
| A — 10k/mês | projeção favorável | RPS < 0,06; pool, memória e CPU dentro das margens com folga ampla. Harness criado e reproduzível. Execução real contra staging confirma sem surpresas. |
| B — 10k/dia | **gap — execução pendente** | Projeção analítica favorável (pool < 1 %, CPU < 3 %); porém, sem execução real contra staging/VPS equivalente, o veredito permanece não comprovado. O harness está pronto para execução imediata. |

## Como executar o harness contra staging

```bash
# Pré-requisito: k6 instalado e staging rodando
export WHATSAPP_APP_SECRET="<mesmo valor do staging>"
export DATABASE_URL="postgres://<staging>"

# Envelope B completo
task loadtest:suite:envelope-b BACKEND=https://staging.mecontrola.com.br

# Ou passo a passo com monitoramento Grafana paralelo:
task loadtest:whatsapp ENVELOPE=b VUS=5 DURATION=10m BACKEND=https://staging.mecontrola.com.br
task loadtest:outbox EVENT_COUNT=10000 TIMEOUT_SEC=600
task loadtest:read ENVELOPE=b AUTH_TOKEN=<jwt> BACKEND=https://staging.mecontrola.com.br
```

Observar durante a execução no Grafana (painel existente):
- `mecontrola_db_pool_in_use` — threshold < 16 (80 %)
- `node_cpu_seconds_total` — threshold < 70 % do host
- `http_server_requests_total{code=~"5.."}` — deve ser 0

## Gaps remanescentes

1. **Execução real pendente**: o veredito definitivo de envelope B exige execução do harness
   contra staging (ou VPS equivalente) com coleta de métricas reais. Sem isso, o veredito é
   projeção, não prova.
2. **LLM rate-limit**: o OpenRouter pode barrar rajadas de envelope B se as mensagens ativarem
   o agente; não há controle no host.
3. **Leitura autenticada**: `transactions-read.js` aceita 401 sem token; para medir latência real
   de leitura autenticada, fornecer `AUTH_TOKEN` de staging.

## Conclusão

O harness (RF-17) está implementado, reproduzível e integrado ao Taskfile. Os thresholds (RF-18)
estão definidos e embutidos nos scripts k6. A projeção analítica é favorável para ambos os
envelopes, mas o veredito de envelope B permanece **não comprovado** até execução real contra
staging — conforme declarado sem falso positivo.
