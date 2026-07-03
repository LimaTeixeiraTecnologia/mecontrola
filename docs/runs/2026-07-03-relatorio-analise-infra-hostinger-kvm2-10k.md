# Relatório de Análise de Infraestrutura

- Data: 2026-07-03
- Prompt de origem: `docs/runs/2026-07-03-prompt-analise-infra-hostinger-kvm2-10k.md`
- Método: read-only, orientado a evidência. Nenhuma alteração de código/infra foi feita.
- Evidência de campo: acesso SSH read-only autorizado à VPS de produção `root@187.77.45.48` (somente comandos de leitura: `docker service ls`, `docker stats`, `free`, `df`, `pgbackrest info`, `ss`, `ufw status`, etc.).

> **Aviso de honestidade (mandato do prompt):** onde a prova não fecha, o veredito é `não comprovado` — não convertido em positivo. Preços Hostinger são snapshot promocional e devem ser reconferidos na data de contratação.

---

## 1. Escopo e método

**Objetivo.** Determinar se a infraestrutura atual (foco em `deployment/`) atende, com eficiência, confiabilidade e menor custo, a evolução de 0 até 10 mil usuários ativos — avaliando três envelopes separados (A: 10k/mês, B: 10k/dia, C: 10k simultâneos). Não implementar nada.

**Fontes do repositório consultadas (evidência primária).**
- `deployment/compose/compose.swarm.yml` (topologia, limits/reservations, secrets, redes, healthchecks, update policy)
- `deployment/postgres/postgresql.conf`, `deployment/pgbouncer/pgbouncer.ini`
- `deployment/config/prod.env`, `deployment/caddy/Caddyfile`
- `deployment/pgbackrest/pgbackrest.conf` + `crontab.txt`, `deployment/docker/Dockerfile.postgres`
- `deployment/terraform/main.tf` (+ variables/outputs), `deployment/scripts/backup-env-s3.sh`, `deployment/scripts/pgbackrest-setup.sh`
- `.github/workflows/ci-cd.yml`, `deployment/scripts/deploy-swarm.sh`, `deployment/scripts/render-stack.py`
- `deployment/runbooks/{deploy,rollback,restore-pitr,restore-vps}.md`
- `cmd/server/server.go`, `cmd/worker/worker.go`, `configs/config.go`, `internal/platform/outbox/storage_postgres.go`
- `scripts/loadtest/README.md`, `.specs/prd-whatsapp-ordenacao-idempotencia/prd.md`

**Fontes oficiais consultadas.** Hostinger VPS (hostinger.com/br/servidor-vps); Docker Swarm/Compose Deploy Spec (docs.docker.com); PostgreSQL 16 (postgresql.org/docs/16); PgBouncer (pgbouncer.org/config.html); Caddy 2 (caddyserver.com/docs); Grafana otel-lgtm (github.com/grafana/docker-otel-lgtm). URLs citadas nas seções 3 e 6.

**Regras de decisão.** (1) afirmação técnica → evidência concreta (arquivo:linha ou observação SSH); (2) capacidade/plataforma → também documentação oficial; (3) ausência de prova → `não comprovado`; (4) `production-ready`/`0 gaps` só como conclusão provada, nunca premissa.

**Estado observado do ambiente (SSH, 2026-07-03 10:44 UTC).**
- Host: Ubuntu 24.04.4 LTS, **2 vCPU**, **7,75 GiB RAM** (`MemTotal 8131472 kB`), **4 GiB swap**, disco **96 GiB (35 GiB usados / 62 GiB livres, 36%)**, uptime 16 dias, `load average 0.07`.
- Swarm: **1 nó** (`srv1761537`, Leader), Docker Engine 29.5.3.
- App **viva e servindo**: `curl http://localhost/healthz` → **HTTP 200 em 4 ms**.
- Uso real em ~0 usuários: **~871 MB somados em todos os containers**; `free`: 1,9 GiB usados / 6,35 GiB disponíveis; swap praticamente zero.

---

## 2. Inventário atual comprovado

| componente | função | onde está definido | depende de | persistência | exposição | observabilidade | criticidade |
|---|---|---|---|---|---|---|---|
| postgres | banco primário | `compose.swarm.yml:106` (prod: imagem custom `mecontrola-postgres:mastra-*` c/ pgBackRest) | volume `postgres-data`; conf montada | `postgres-data` (144 MB obs.) + `pgbackrest-repo` | interna (rede `backend`) | `postgres-exporter` + logs | **crítica (SPOF de dados)** |
| pgbouncer | pool de conexões (transaction) | `compose.swarm.yml:138` (`edoburu/pgbouncer:v1.25.2-p0`) | postgres | stateless | interna | healthcheck `pgrep` | alta |
| postgres-exporter | métricas do banco | `compose.swarm.yml:179` | postgres | stateless | interna | é a própria fonte | média |
| node-exporter | métricas do host | `compose.swarm.yml:205` | `/proc`,`/sys`,`/` (ro) | stateless | interna | fonte | média |
| migrate | migrações (run-once) | `compose.swarm.yml:235` (`restart: none`) | postgres, pgbouncer | efêmero | nenhuma | logs | alta (gate de deploy) |
| server-1 / server-2 | API HTTP (2 serviços × 1 réplica) | `compose.swarm.yml:273/320` | pgbouncer | stateless (`read_only`, `tmpfs /tmp`) | via caddy | `/healthz`, OTEL | **crítica** |
| worker-1 / worker-2 | jobs/outbox/crons | `compose.swarm.yml:367/413` | pgbouncer | stateless | `/readyz` interno | `/readyz`, OTEL | **crítica** |
| caddy | reverse proxy + TLS ACME | `compose.swarm.yml:459` (prod: imagem custom `mecontrola-caddy:*`) | server-1/2 | `caddy-data`,`caddy-config` | **80/443 host** | logs json | **crítica (ingress SPOF)** |
| otel-lgtm | observabilidade all-in-one (OTel Collector+Prometheus+Loki+Tempo+Grafana) | `compose.swarm.yml:499` (`grafana/otel-lgtm:0.7.5`) | — | 4 volumes (prom 89 MB, tempo 28 MB obs.) | **3000 host** (bloqueada por ufw) | é o backend | média-alta |
| pg-tunnel | túnel socat p/ acesso DBA | `compose.swarm.yml:546` (`alpine/socat`) | postgres | stateless | **15432 host** (bloqueada por ufw) | healthcheck `nc` | baixa (superfície) |
| secrets (17) | credenciais via `/run/secrets` (SOPS+age) | `compose.swarm.yml:52-103,586` | docker secret externo | — | — | — | crítica |
| redes | `backend` (overlay encrypted) / `frontend` (overlay) | `compose.swarm.yml:640-650` | swarm | — | — | — | alta |
| **CI runner** (fora do compose) | GitHub Actions self-hosted **no próprio host** | SSH: `/home/github-runner/actions-runner`, `runs-on: [self-hosted, staging]` (`ci-cd.yml:250`) | host | 2,4 GiB workspace + 22 GiB build cache | — | — | **alta (contenção)** |

Componentes externos obrigatórios: **OpenRouter** (LLM, caminho de inbound), **AWS S3** (repositório pgBackRest + backup de env), **Meta/WhatsApp**, **Kiwify**, **Resend/SMTP**.

---

## 3. Prova oficial do ambiente Hostinger / KVM 2

| atributo | valor oficial | fonte oficial | impacto na arquitetura |
|---|---|---|---|
| KVM 2 vCPU | 2 cores | hostinger.com/br/servidor-vps | **restrição vinculante** — limits do compose somam 5,55 vCPU (2,8× o host) |
| KVM 2 RAM | 8 GB | idem | limits do compose somam 6,77 GB → 87% do host antes de OS/Docker |
| KVM 2 NVMe | 100 GB | idem | confirmado no host (96 GiB usáveis); 35 GiB já ocupados |
| KVM 2 banda | 8 TB/mês | idem | não é gargalo (tráfego WhatsApp/LLM é leve) |
| KVM 2 preço | R$ 43,99/mês promo · **R$ 77,99/mês renovação** | idem | base de custo; reconferir na data |
| Backups Hostinger | semanais grátis | idem | complemento, **não substitui** PITR próprio |
| KVM 4 (upgrade) | 4 vCPU / 16 GB / 200 GB / 16 TB — R$ 59,99 promo · R$ 149,99 renov. | idem | rota de escala vertical para envelopes B/C |
| KVM 8 | 8 vCPU / 32 GB / 400 GB / 32 TB — R$ 119,99 promo · R$ 259,99 renov. | idem | teto de escala vertical single-node |

Confirmação por SSH: `MemTotal 8131472 kB`, `nproc=2`, `df / = 96G`, `PRETTY_NAME="Ubuntu 24.04.4 LTS"` — **o dado de entrada informado bate com a realidade do host**, e a RAM (omitida no input) é **8 GB comprovada**.

Documentação oficial sobre plataforma (evidência de comportamento):
- **Swarm single-node = sem HA/failover.** Raft exige quórum de managers; reduzir a 1 manager é "unsafe... not recommended"; se o nó cai, "the swarm becomes unavailable" (docs.docker.com/engine/swarm/admin_guide/).
- **`order: stop-first` (default)** para a tarefa antiga antes de subir a nova (janela de indisponibilidade da tarefa); `start-first` sobrepõe as duas (docs.docker.com/reference/compose-file/deploy/).
- **`limits` = teto rígido; `reservations` = garantia de scheduling** (idem).
- **`depends_on` não é honrado em `docker stack deploy`**: comportamento histórico e amplamente confirmado, porém a citação literal na doc **viva atual** é `não comprovado` (estava na ref. legada v3). A doc atual confirma apenas que o Swarm ignora opções não suportadas.
- **otel-lgtm é "intended for development, demo, and testing"**, com recomendação oficial de usar Grafana Cloud para produção (github.com/grafana/docker-otel-lgtm) → **comprovado que a stack de observabilidade não é imagem production-grade**.
- PgBouncer `transaction`: server devolvido ao pool ao fim de cada transação; `default_pool_size` limita conexões server por par user/db; `max_client_conn` limita clientes (pgbouncer.org/config.html).
- Caddy: HTTPS automático via ACME (Let's Encrypt/ZeroSSL) e health checks ativos com `health_uri`/`health_interval` (caddyserver.com/docs).

---

## 4. Orçamento real da stack atual

Valores declarados em `compose.swarm.yml`. `migrate` é run-once (`restart_policy: none`, observado `0/1`), listado à parte.

| serviço | réplicas | cpu_reservada | cpu_limite | mem_reservada | mem_limite | storage | uso real observado (SSH, idle) |
|---|---|---|---|---|---|---|---|
| postgres | 1 | 0.25 | 1.00 | 512M | 2G | postgres-data 144M + repo | 131,9 MB / 0,07% CPU |
| pgbouncer | 1 | 0.05 | 0.25 | 32M | 128M | — | 1,8 MB |
| postgres-exporter | 1 | 0.05 | 0.25 | 32M | 128M | — | 14,9 MB |
| node-exporter | 1 | 0.05 | 0.25 | 32M | 128M | — | 9,7 MB |
| server-1 | 1 | 0.10 | 0.75 | 128M | 768M | — | 13,9 MB / 0,02% |
| server-2 | 1 | 0.10 | 0.75 | 128M | 768M | — | 14,1 MB / 0,01% |
| worker-1 | 1 | 0.05 | 0.50 | 64M | 384M | — | 15,0 MB / 0,13% |
| worker-2 | 1 | 0.05 | 0.50 | 64M | 384M | — | 15,1 MB / 0,10% |
| caddy | 1 | 0.05 | 0.25 | 32M | 128M | caddy-data/config | 12,6 MB |
| otel-lgtm | 1 | 0.10 | 1.00 | 512M | 2G | 4 vols (~120 MB) | **639,9 MB / 3,03%** |
| pg-tunnel | 1 | 0.01 | 0.05 | 8M | 16M | — | 2,6 MB |
| **Subtotal (persistente)** | 11 | **0.86** | **5.55** | **1.51 GB** | **6.77 GB** | — | **~871 MB** |
| migrate (transiente no deploy) | 1 | 0.05 | 0.25 | 64M | 256M | — | 0/1 (não roda em regime) |

- **Total indispensável para operação mínima** ≈ soma das reservations: **0,86 vCPU / 1,51 GB** — cabe folgado.
- **Total worst-case (limits)**: **5,55 vCPU / 6,77 GB** (+256 MB de migrate durante deploy) — **oversubscription de CPU 2,8× e memória ~87–91% do host** antes do overhead de OS/Docker/Swarm.
- **Margem restante no host** (worst-case memória): 7,75 GB − 6,77 GB ≈ **0,98 GB** para OS+Docker+Swarm → **sem margem segura**; o colchão real é o **swap de 4 GB** + o fato de os limits raramente serem atingidos simultaneamente.

**Oversubscription declarada explicitamente:** sim. A stack **depende** de que os picos não coincidam. Em ~0 usuários isso é invisível (871 MB, CPU ~0); sob carga real de 10k é onde o risco se materializa (ver seção 6).

---

## 5. Comparativo stack × host

| recurso | capacidade oficial do host | demanda declarada da stack | margem | status |
|---|---|---|---|---|
| CPU (reservations) | 2,0 vCPU | 0,86 vCPU | +1,14 vCPU | ✅ cabe |
| CPU (limits, worst-case) | 2,0 vCPU | 5,55 vCPU | −3,55 vCPU | ⚠️ oversubscrito 2,8× (aceitável só em baixa concorrência) |
| RAM (reservations) | 7,75 GiB | 1,51 GB | +6,24 GiB | ✅ cabe |
| RAM (limits, worst-case) | 7,75 GiB | 6,77 GB (+0,26 migrate) | ~0,7–0,98 GiB p/ OS | ⚠️ sem margem segura (depende de swap) |
| RAM (uso real, idle) | 7,75 GiB | ~1,9 GiB usados | +5,9 GiB | ✅ folga real hoje |
| Disco | 96 GiB (62 livres) | app+dados <0,3 GiB; **CI runner ~50 GiB** (25 GiB imagens + 22 GiB build cache + 2,4 GiB workspace) | 62 GiB | ⚠️ 36% já usado por **CI no host de produção**, não pela app |
| Banda | 8 TB/mês | tráfego leve (texto+LLM API) | alta | ✅ cabe |

**Leitura:** a aplicação em si é enxuta e cabe com sobra. Os dois pontos de pressão do host **não são a app**: (1) os **limits worst-case** deixam a memória sem margem em cenário de pico simultâneo; (2) o **runner de CI reside no host de produção** e consome ~50 GiB de disco + rouba os 2 vCPU durante builds.

---

## 6. Análise por envelope de capacidade

Fatores estruturais que valem para todos os envelopes:
- **Ingress**: Caddy `100 req/s por IP, burst 200` (`Caddyfile:17-25`); webhook WhatsApp `600/min = 10 req/s` (`prod.env:191`). Como os webhooks da Meta chegam de **poucos IPs de origem**, o rate-limit por IP pode **estrangular rajadas legítimas** — impacto `não comprovado` (sem teste).
- **Pool de conexões**: 4 processos × `DB_MAX_CONNS=10` = **40 conexões cliente** contra `MAX_DB_CONNECTIONS=60` no pgbouncer e `max_connections=100` no Postgres. Folga de ~20; jobs/migrate podem estreitar (`config.go`, `pgbouncer` env). Observado: 20 conexões abertas em idle.
- **Serialização por usuário** (`internal/platform/outbox/storage_postgres.go:63-93`): no máximo **1 evento em voo por `aggregate_user_id`**, ordem `(occurred_at, created_at, id)`. Correto para ordenação, mas **limita paralelismo**.
- **LLM externo no caminho de inbound** (OpenRouter, `AGENT_INBOUND_TIMEOUT=90s`, sem fallback chain): latência p95 ~3 s e teto de 90 s por evento — **não controlável**.
- **Throughput outbox**: teórico `50 × 2/s = 100 ev/s por worker`; real documentado `~150–250 ev/s por worker` **sem** serialização; **com** serialização + LLM, o efetivo cai para ~dezenas de ev/s (`loadtest/README.md:100`).
- **Nenhum teste de carga foi executado em qualquer envelope de 10k** (só baseline planejada) → qualquer afirmação de "atende 10k" é, na melhor hipótese, `atende com ajustes` por cálculo, nunca `comprovado por ensaio`.

### 6.1 Envelope A — 10k ativos/mês
- **Gargalos**: nenhum atingido; ~333 usuários/dia, concorrência de pico na casa de poucos req/s.
- **Fatores limitantes**: latência do LLM (UX), não recursos do host.
- **Satura primeiro**: nada, em recursos; a UX depende do OpenRouter.
- **Comprovado**: recursos folgam (reservations 0,86 vCPU/1,51 GB; idle real 1,9 GiB); app viva (HTTP 200/4 ms).
- **Não comprovado**: p95 sob a distribuição real de picos (sem load test).
- **Veredito**: **atende com ajustes obrigatórios** (ajustes = seção 7, não capacidade).

### 6.2 Envelope B — 10k ativos/dia
- **Gargalos**: 2 vCPU compartilhados entre 2 servers + 2 workers + postgres + otel-lgtm **+ CI runner**; contenção nos picos diários; pool de 40 conexões sob rajada de retry.
- **Fatores limitantes**: concorrência de handlers LLM-bound; 2 workers com 1 slot/usuário; **100% de trace sampling** (ver seção 7, item G) inflando otel-lgtm.
- **Satura primeiro**: CPU do host em janelas de pico coincidindo com jobs `@every 30s`/`@hourly`; secundariamente o pool de conexões.
- **Comprovado**: a stack roda estável; o design serializa corretamente; recursos médios cabem.
- **Não comprovado**: p95 em pico diário — **nenhum ensaio proporcional executado**; a nota de design do PRD sugere degradação bem antes de 10k concorrentes.
- **Veredito**: **não comprovado** (indício de que atende com ajustes de compute, mas sem prova de carga — o prompt proíbe converter ausência de prova em positivo).

### 6.3 Envelope C — 10k simultâneos em pico
- **Gargalos**: 10k eventos exigindo processamento quase concomitante; 2 workers, 1 evento/usuário em voo, handler LLM 3–10 s; rate-limit por IP dos webhooks Meta.
- **Fatores limitantes**: drenagem = nº de eventos ÷ throughput efetivo (dezenas/s) → **minutos** de atraso para a cauda; single-node sem escala horizontal.
- **Satura primeiro**: fila de outbox (`status=1`) e latência ponta-a-ponta; depois CPU.
- **Comprovado (evidência de projeto)**: o próprio PRD `prd-whatsapp-ordenacao-idempotencia` registra que **partição única sustenta ~2000 usuários e ~7 s em 10k**, indicando necessidade de sharding por hash (ADR-001) — ainda **não implementado**.
- **Não comprovado numericamente em código**, mas o conjunto (2 vCPU + 2 workers + LLM + serialização single-partition + rate-limit por IP) torna a simultaneidade real de 10k inviável no desenho atual.
- **Veredito**: **não atende** para 10k estritamente simultâneos no single-node atual.

---

## 7. Gaps, lacunas e riscos residuais

| item | categoria | evidência | impacto | severidade | bloqueia 10k? | ação obrigatória |
|---|---|---|---|---|---|---|
| A. Swarm **single-node = SPOF** total | risco/arquitetura | SSH `docker node ls` (1 nó Leader); docs.docker.com admin_guide | qualquer falha do nó = downtime completo (app, banco, obs) | **alta** | B parcialmente, **C sim** | multi-node OU aceitar e documentar RTO; healthchecks já mitigam parcialmente |
| B. **Backups não agendados** | gap operacional | SSH: sem crontab (root/github-runner/postgres), `/etc/cron.d` sem backup; `pgbackrest info` último full **2026-07-01** (hoje 07-03) | base backup envelhece; janela de PITR e tempo de replay crescem | **alta** | não (risco de dados) | instalar `pgbackrest/crontab.txt` como cron/systemd-timer versionado; alertar falha de backup |
| C. **Restore/PITR nunca testado** | gap/DR | runbooks `restore-pitr.md`/`restore-vps.md` dizem "atualizar após primeiro restore real"; `deploy.md:195` condiciona a tarefa 6.0 (sem `done`) | DR não validado = DR inexistente na prática | **alta** | não (risco de recuperação) | ensaio de restore PITR + restore-VPS em staging, com evidência anexada |
| D. **CI runner no host de produção** | risco/custo | SSH `/home/github-runner/actions-runner`, build cache 22 GiB, imagens 25 GiB, workspace 2,4 GiB; `ci-cd.yml:250` self-hosted | builds roubam os 2 vCPU e ~50 GiB de disco da produção | **alta** | B/C (contenção em pico) | mover CI p/ runner GitHub-hosted ou host separado; `docker builder prune` agendado |
| E. **Trace sampling 100% em prod** | risco/custo | `compose.swarm.yml:287` `OTEL_TRACE_SAMPLE_RATE:"1"` sobrepõe `prod.env:44 =0.1`; gate `deploy-anti-storm` **exige** "1" | custo de CPU/storage de observabilidade explode com carga | **média-alta** | B/C | reduzir sampling em prod (ex. 0,1 c/ tail-sampling) e revisar o gate |
| F. **otel-lgtm não é production-grade** + single replica | risco | github.com/grafana/docker-otel-lgtm ("development, demo, testing"); `compose.swarm.yml:499` 1 réplica | perda de observabilidade num incidente; concorre por 2 GB RAM | **média** | não | externalizar (Grafana Cloud free) ou aceitar/documentar limite |
| G. **Pool de conexões estreito** | risco/capacidade | 40 clientes vs `MAX_DB_CONNECTIONS=60` (`compose.swarm.yml:151`); jobs adicionais | esgotamento em rajada de retry do outbox | **média** | B | monitorar `pgbouncer` pool; dimensionar antes de subir réplicas |
| H. **Ordenação single-partition** (sem sharding) | lacuna/capacidade | `storage_postgres.go:63-93`; PRD: "~2000 users, 10k ~7s"; ADR-001 sharding não feito | p95 degrada muito antes de 10k simultâneos | **alta** | **C** | implementar sharding por hash (ADR-001) antes de mirar C |
| I. **LLM externo sem fallback no caminho crítico** | risco | `config.go` `AGENT_INBOUND_TIMEOUT=90s`; OpenRouter único provider | indisponibilidade/lentidão do provedor trava inbound | **média** | B/C (UX) | timeout curto + circuit breaker + fila de retry (parte já existe) |
| J. **Deploy lag** | higiene | SSH: prod roda `571425f`; `main` = `8bd5ad6` (5 commits à frente) | prod não reflete correções mergeadas (ex. confirmação honesta) | **média** | não | redeploy da `main`; alerta de drift de versão |
| K. **Rate-limit por IP vs webhooks Meta** | lacuna | `Caddyfile` 100/s por IP + webhook 10/s; Meta usa poucos IPs | rajadas legítimas podem tomar 429 | **média** | B/C | `não comprovado` — validar allowlist/limite específico p/ IPs da Meta |
| L. pg-tunnel exposto em `0.0.0.0:15432` | superfície | SSH `ss`: `0.0.0.0:15432`; ufw **bloqueia** (default deny, só 22/80/443) | superfície de banco depende só do ufw | **baixa** | não | bind em loopback ou remover quando não usado |
| M. **Nenhum load test em envelope 10k** | lacuna de prova | `loadtest/README.md` só baseline; nenhum run 10k | impossível declarar B/C `comprovado` | **alta** (p/ veredito) | B/C | ensaio de carga proporcional por envelope com k6 |

Positivos comprovados (não são gaps, registrados para equilíbrio): pipeline CI/CD com lint+test+vulncheck+**Trivy bloqueante**+**cosign keyless**+tags imutáveis por SHA (`ci-cd.yml`); deploy `stop-first`/`parallelism:1`/`stop_grace_period:30s` com **rollback automático** por health (`deploy-swarm.sh:187-226`); migrações com advisory lock; **ufw ativo** (só 22/80/443; 3000 negada); containers `read_only`, `cap_drop: ALL`, user não-root `65532`; rede `backend` overlay **encrypted**; secrets via SOPS+age em `/run/secrets`; **pgBackRest operacional com cifra AES-256 e WAL archiving contínuo para S3** (base backups existem, faltam agendamento e teste de restore).

---

## 8. Eficiência de custo

| opção | custo oficial comprovado | atende quais envelopes | trade-offs | decisão |
|---|---|---|---|---|
| **Manter KVM 2 + ajustes** (seção 9 F1–F2) | R$ 43,99 promo / **R$ 77,99 renov.** | A (sim); B (provável, sob prova) | teto de 2 vCPU; oversubscription persiste | **recomendado até fechar A/B com prova** |
| KVM 1 (downgrade) | R$ 29,99 / R$ 59,99 | nenhum desta stack | 4 GB RAM não comporta limits (otel-lgtm 2 GB) e 1 vCPU asfixia | **rejeitado** (viola gates) |
| KVM 4 (upgrade vertical) | R$ 59,99 / R$ 149,99 | A, B; C ainda limitado por design | +CPU/RAM resolve contenção; **continua single-node SPOF** | **rota para B/C** após sharding |
| KVM 8 | R$ 119,99 / R$ 259,99 | A, B; C só com sharding | teto vertical; custo alto | reserva p/ crescimento além de B |
| Externalizar observabilidade (Grafana Cloud free) + tirar CI do host | economia de ~2 GB RAM e ~50 GiB disco no host | melhora todos | depende de tier gratuito e rede | **recomendado** (libera KVM 2 por mais tempo) |

**Menor custo com confiabilidade suficiente:** para **A** e provavelmente **B**, é **manter o KVM 2** e aplicar os ajustes de baixo custo (backup agendado, tirar CI do host, reduzir sampling, testar restore). Para **C**, nenhum plano single-node basta sem o sharding de ordenação (ADR-001); só então KVM 4/8 faz sentido. **Não** se justifica upgrade de plano antes de esgotar os ajustes gratuitos. Preços são snapshot promocional → `reconferir na data`.

---

## 9. Plano de evolução completo

| fase | gatilho | mudança | custo incremental | risco mitigado | prova exigida para concluir |
|---|---|---|---|---|---|
| **F0 — Correções imediatas (estado atual)** | agora | (a) agendar pgBackRest via cron/systemd-timer versionado + alerta de falha; (b) **ensaio de restore PITR + restore-VPS** em staging; (c) redeploy da `main` (sair de `571425f`) | R$ 0 | perda de dados sem backup fresco; DR não validado; drift de versão | `pgbackrest info` com full ≤24 h; log de restore bem-sucedido; `docker service ls` na `main` |
| **F1 — Tirar CI do host + higiene de disco** | F0 done | mover runner p/ GitHub-hosted ou VM separada; `docker builder prune`/`image prune` agendado | R$ 0 (hosted) ou custo de 1 VM pequena | contenção de 2 vCPU e ~50 GiB de disco em produção | `df /` com CI fora; ausência de processo runner no host |
| **F2 — Enxugar footprint de observabilidade** | F1 done | reduzir `OTEL_TRACE_SAMPLE_RATE` em prod (rever gate anti-storm); avaliar Grafana Cloud free | R$ 0 | explosão de CPU/storage de traces sob carga | painel de custo de obs estável sob carga sintética |
| **F3 — Provar envelope A/B** | F0–F2 done | **load test proporcional** (k6) p/ A e B com métricas de pool, CPU, p95, drenagem de outbox | R$ 0 | veredito "não comprovado" de A/B | relatório k6 com p95<meta e `http_req_failed`<1% no envelope |
| **F4 — Habilitar caminho para C: sharding de ordenação** | mirar >~2000 simultâneos | implementar sharding por hash do `aggregate_user_id` (ADR-001) + aumentar réplicas de worker | esforço de dev; sem custo de infra imediato | p95 degradando antes de 10k simultâneos | load test 10k simultâneos com p95 dentro da meta |
| **F5 — Escala vertical quando F3/F4 provarem necessidade** | F3/F4 com gargalo de CPU/RAM comprovado | upgrade KVM 2 → **KVM 4** | +R$ ~16 promo / +R$ ~72 renov./mês | oversubscription de CPU/RAM sob carga real | métricas pós-upgrade com margem >30% em pico |
| **F6 — HA (opcional, se SLA exigir)** | requisito de disponibilidade formal | multi-node Swarm (3 managers) OU banco gerenciado externo | +≥2 VPS ou custo de DBaaS | SPOF do nó único | failover ensaiado com evidência |

Nenhuma fase usa termo vago: cada uma tem gatilho, mudança concreta, custo e prova de saída objetiva.

---

## 10. Veredito final

- **A infra atual no KVM 2 atende hoje o envelope A (10k/mês)?** — **Sim, com ajustes obrigatórios (F0)**. Capacidade folga (reservations 0,86 vCPU/1,51 GB; idle real 1,9 GiB; app HTTP 200/4 ms). A ressalva é operacional (backup/DR/CI), não de capacidade.
- **A infra atual no KVM 2 atende hoje o envelope B (10k/dia)?** — **Não comprovado.** O cálculo indica viabilidade com ajustes de compute, mas **nenhum ensaio de carga proporcional foi executado** e há contenção real de 2 vCPU agravada pelo CI no host. Não convertível em positivo sem F3.
- **A infra atual no KVM 2 atende hoje o envelope C (10k simultâneos)?** — **Não atende.** Single-node, 2 workers, serialização single-partition (PRD: ~2000 usuários / 10k ~7 s), LLM externo e rate-limit por IP inviabilizam simultaneidade real de 10k sem o sharding do ADR-001 (F4).
- **A infra atual é a opção de menor custo possível com confiabilidade suficiente?** — **Para A/B, sim** (KVM 2 é o piso viável desta stack; KVM 1 não comporta). Mas há **desperdício removível sem custo**: CI e ~50 GiB de disco no host de produção, e 100% de trace sampling. Confiabilidade "suficiente" ainda depende de **backup agendado + restore testado** (F0).
- **O relatório fechou 0 gaps e 0 lacunas com prova?** — **Não.** Restam gaps comprovados (backup não agendado, restore não testado, CI no host, SPOF single-node, sharding ausente) e lacunas de prova (load test 10k, impacto do rate-limit por IP, p95 de B/C).
- **O que impede declarar production-ready/proof?** — Três bloqueios objetivos: **(1)** DR não validado (backups sem agendamento + restore/PITR nunca ensaiado); **(2)** ausência de qualquer teste de carga nos envelopes de 10k; **(3)** SPOF de nó único sem HA nem sharding para o envelope C. Enquanto (1)–(3) não forem fechados com evidência, o veredito global é **NÃO production-ready/proof** — a infraestrutura está **funcional e sólida para 0→A**, mas **não comprovada** para B e **insuficiente por desenho** para C.

**Classificação por envelope:** A = `atende com ajustes obrigatórios` · B = `não comprovado` · C = `não atende`.
