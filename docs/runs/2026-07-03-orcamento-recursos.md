# Orçamento de Recursos KVM 2 e Gatilho de Upgrade KVM 4

Data: 2026-07-03
Tarefa: 5.0 (PRD `.specs/prd-infra-evolucao-kvm2-10k/prd.md`, RF-14, RF-15, RF-16)
Host de referência: Hostinger KVM 2 — 2 vCPU / 8 GB RAM nominal (7,75 GiB = 7936 MB utilizáveis, medido em auditoria SSH) / 96 GB NVMe / swap 4 GB

## Orçamento de Memória Aprovado

### Modelo de overhead

| Componente | Estimativa |
|-----------|-----------|
| OS (Debian/Ubuntu kernel + userland) | 300 MB |
| Docker daemon + Swarm agent | 200 MB |
| buffers/cache do kernel | 500 MB |
| **Total de overhead** | **~1,0 GB (conservador 1,5 GB)** |

Memória disponível para containers: 7936 MB − 1500 MB = **6436 MB (~6,3 GB, piso conservador)**.

### Limites aprovados (steady-state, excluindo `migrate` que é transiente)

| Serviço | Limit anterior | Limit aprovado | Reservação |
|---------|---------------|----------------|-----------|
| postgres | 2048 MB | 2048 MB | 512 MB |
| pgbouncer | 128 MB | 128 MB | 32 MB |
| postgres-exporter | 128 MB | 128 MB | 32 MB |
| node-exporter | 128 MB | 128 MB | 32 MB |
| server-1 | 768 MB | 512 MB | 128 MB |
| server-2 | 768 MB | 512 MB | 128 MB |
| worker-1 | 384 MB | 256 MB | 64 MB |
| worker-2 | 384 MB | 256 MB | 64 MB |
| caddy | 128 MB | 128 MB | 32 MB |
| otel-lgtm | 2048 MB | 1228 MB | 512 MB |
| pg-tunnel | 16 MB | 16 MB | 8 MB |
| **Total steady-state** | **6 928 MB (6,77 GB)** | **5 340 MB (5,21 GB)** | |

Margem sobre a RAM real (7936 MB): **7 936 − 1 500 (overhead) − 5 340 = 1 096 MB (~1,07 GB)**.

Bases de corte:
- `server-1/2` (512 MB): processo Go HTTP com OTel; uso real medido em idle < 150 MB; 512 MB cobre pico de 3× o idle.
- `worker-1/2` (256 MB): worker de outbox; uso real < 100 MB idle; 256 MB cobre pico.
- `otel-lgtm` (1 228 MB): idle observado na VPS prod = 640 MB; 1 228 MB = ~92% headroom sobre idle.
- `postgres` (2 048 MB): mantido para shared_buffers + working memory; calibrar com REQ-06.

### Calibração pós-carga (REQ-06)

Os limites acima são conservadores e devem ser revisados após a tarefa 6.0 (harness k6) com os valores p95 observados sob carga envelope B. Se p95 de RSS de server ou worker ultrapassar 400 MB e 200 MB respectivamente, ajustar os limits upward antes de reduzir.

## Dimensionamento do Pool de Conexões

### pgBouncer (transaction mode)

| Parâmetro | Valor anterior | Valor aprovado | Justificativa |
|-----------|---------------|----------------|--------------|
| `DEFAULT_POOL_SIZE` | 15 | 20 | headroom para 4 processos × 10 client conns + outbox workers |
| `RESERVE_POOL_SIZE` | 5 | 5 | inalterado; cobre pico de burst |
| `MAX_CLIENT_CONN` | 300 | 300 | inalterado; teto de clientes simultâneos |
| `MAX_DB_CONNECTIONS` | 60 | 30 | cap realista: 20 pool + 5 reserve + 5 emergencial |

### Aplicação (por processo)

| Parâmetro | Valor | Observação |
|-----------|-------|-----------|
| `DB_MAX_CONNS` | 10 | 4 processos × 10 = 40 client conns ao pgBouncer |
| `DB_MIN_CONNS` | 2 | inalterado |
| `DB_MAX_IDLE_CONNS` | 5 | inalterado |

Em transaction mode, os 40 client conns são servidos pelos 20 backend conns do `DEFAULT_POOL_SIZE`. A relação típica de multiplexação em transações curtas é 3:1 a 5:1, logo 20 backend conns sustentam 60–100 transações simultâneas de baixa duração.

## Alertas de Saturação de Pool

Dois alertas provisionados em `deployment/telemetry/grafana/provisioning/alerting/rules.yaml` (grupo `pool`):

| UID | Condição | Severidade |
|-----|---------|-----------|
| `mc-pgbouncer-pool-saturation` | `pg_stat_database_numbackends{datname="mecontrola_db"} > 24` (80% de MAX_DB_CONNECTIONS=30) por 5 min | critical |
| `mc-pgbouncer-client-queue` | `sum(rate(database_pool_wait_count_total[5m])) > 2` por 3 min | warning |

O alerta `mc-db-pool-wait` pré-existente (grupo `tecnico`) complementa cobrindo o lado da aplicação com limiar 1/s.

## Gatilho Objetivo de Upgrade KVM 2 → KVM 4

Upgrade deve ser iniciado quando **qualquer** dos critérios abaixo for sustentado por 30 minutos em janela de carga normal (envelope B):

| Métrica | Limiar de upgrade | Alerta mapeado |
|---------|-----------------|---------------|
| CPU do host (`node_cpu_seconds_total`) | > 70% por 30 min | `mc-cpu-usage-high` (warning já existente em 70%) |
| RAM disponível do host | < 800 MB livres (~10% de 7936 MB) por 30 min | `mc-memory-usage-high` (warning em 80%) |
| Pool wait da aplicação | > 5 esperas/s por 10 min | ampliar `mc-pgbouncer-client-queue` (warning em 2/s) |
| `pg_stat_database_numbackends` | > 24 (80% de MAX=30) por 15 min | `mc-pgbouncer-pool-saturation` (critical em 24) |
| OOM kill detectado | qualquer evento | `mc-memory-usage-high` em critical + manual |

Preços de referência Hostinger (snapshot 2026-07-03):
- KVM 2: R$ 43,99/mês (promo) / R$ 77,99/mês (renovação)
- KVM 4: R$ 59,99/mês (promo) / R$ 149,99/mês (renovação)

Reconferir valores em https://hostinger.com.br antes de contratar.

## Próximos Passos

1. Executar tarefa 6.0 (harness k6 envelope A/B) e calibrar limites com p95 real.
2. Se carga envelope B atingir > 60% CPU sustentado, iniciar processo de upgrade para KVM 4.
3. Revisar `MAX_DB_CONNECTIONS` se tarefa 6.0 mostrar fila de pool > 1/s em regime normal.
