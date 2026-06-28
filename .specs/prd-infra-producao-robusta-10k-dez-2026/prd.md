# Infraestrutura de Produção Robusta para Escala de 10 Mil Usuários Ativos até Dezembro de 2026

<!-- spec-version: 4 -->

## Resumo Executivo

Esta iniciativa define a evolução da infraestrutura de produção do `mecontrola` para suportar o crescimento de **2** para **10.000 usuários ativos** até dezembro de 2026, com foco inegociável em **robustez, confiabilidade, segurança, performance e operabilidade**.

A arquitetura alvo prevê **2 réplicas de `cmd/server` e 2 réplicas de `cmd/worker` no início**, evoluindo para **3 réplicas de cada** quando a VPS tiver recursos computacionais adequados. A análise documental oficial e o confronto com o estado atual do repositório revelam que mesmo a configuração **2+2 é desafiadora na Hostinger KVM2 atual (2 vCPU / 8 GB RAM / 100 GB NVMe)**, embora menos inviável que 3+3. Este PRD estabelece os requisitos mínimos de produção, registra o conflito de recursos como **restrição explícita** e propõe um roadmap que prioriza a remoção de fragilidades lógicas agora, deixando a infraestrutura pronta para escalar horizontalmente assim que o hardware e o orçamento permitirem.

> **Conclusão central:** a configuração inicial será **2+2 réplicas**. A meta de **3+3 réplicas para 10.000 usuários ativos não cabe de forma robusta na Hostinger KVM2** e só será viável após upgrade de VPS (mínimo indicado: 8 vCPU / 32 GB RAM) ou arquitetura multi-node. O documento trata essa incompatibilidade de frente e define o que pode ser considerado "production-ready" dentro das restrições declaradas.

---

## Contexto Atual Validado

- **Projeto:** monolito modular em Go (`cmd/server`, `cmd/worker`, `cmd/migrate`).
- **Stack de deployment:** Docker Compose, PostgreSQL 16, pgBouncer 1.25.2, pgBackRest, Caddy 2, observabilidade LGTM (Grafana, Loki, Tempo, Prometheus, OpenTelemetry).
- **Ambiente alvo:** VPS Hostinger KVM2, Ubuntu 24.04, produção real, hoje com 2 usuários ativos.
- **Meta:** 10.000 usuários ativos até dezembro/2026, média de 15 interações/usuário/dia.
- **Capacidade desejada obrigatória:** iniciar com **2 réplicas de `server` e 2 réplicas de `worker`**, com caminho para escalar para **3 réplicas de cada**.

### Artefatos do repositório inspecionados

- `deployment/compose/compose.yml`
- `deployment/compose/compose.prod.yml`
- `deployment/caddy/Caddyfile`
- `deployment/postgres/postgresql.conf`
- `deployment/pgbouncer/pgbouncer.ini`
- `deployment/pgbackrest/pgbackrest.conf`
- `deployment/docker/Dockerfile`
- `deployment/docker/Dockerfile.postgres`
- `deployment/docker/Dockerfile.caddy`
- `deployment/telemetry/grafana/`
- `deployment/runbooks/deploy.md`, `restore-pitr.md`, `restore-vps.md`, `rollback.md`
- `README.md`, `AGENTS.md`

### Condições relevantes identificadas

- Compose prod executa **uma única réplica** de `server` e `worker`.
- `Caddyfile` aponta para `server:8080`, sem múltiplos upstreams e sem health checks ativos de backend.
- PostgreSQL 16 roda com resource limit de **1 CPU / 2 GB RAM**, `max_connections=100`, `statement_timeout=0`.
- pgBouncer opera em `pool_mode=transaction`, com `max_client_conn=200`, `default_pool_size=20`, `max_db_connections=50`.
- pgBackRest está configurado para S3, mas `archive_mode=off` no `postgresql.conf` base (ativação via setup é necessária para PITR).
- Dockerfile da aplicação usa Go 1.26.4 e imagem distroless `nonroot` (UID 65532).
- A extrapolação dos resource limits atuais para 2+2 réplicas excede a capacidade de CPU da KVM2, exigindo oversubscription significativo.

---

## Decisões Consolidadas pelas Rodadas de Esclarecimento

As seguintes decisões foram tomadas com o time fundador/engenharia e eliminam ambiguidades materiais do escopo:

| Tema | Decisão | Implicação no PRD |
|---|---|---|
| Réplicas iniciais | **2 `server` + 2 `worker`** no início | Arquitetura e Caddyfile devem suportar 2 upstreams desde o primeiro dia |
| Réplicas-alvo | **3 `server` + 3 `worker`** quando houver recursos | Upgrade de VPS ou multi-node e fase posterior |
| Orçamento | Manter KVM2 única, sem custo adicional | 2+2 e 3+3 ficam condicionados a upgrade futuro; PRD ajusta RPO/RTO e capacidade realista |
| Tolerância a indisponibilidade | Até 5 minutos/mês | Inatingível em VPS única; PRD propõe RPO/RTO realistas |
| Multi-node | Não no horizonte desta fase | HA real não existe; backups off-VM são a única proteção contra falha do host |
| RPO/RTO desejados | 5 min / 15 min | Inatingível em VPS única; PRD ajusta para RPO ≤ 1h, RTO ≤ 4h |
| Bucket de backups | Criar AWS S3, `us-east-1`, Standard, lifecycle para Glacier após 90 dias | pgBackRest terá destino off-VM válido |
| Retenção de observabilidade | 7 dias logs, 15 dias métricas, 7 dias traces | Reduz pressão sobre disco de 100 GB |
| Upgrade futuro de VPS | Depende do crescimento de receita | Fase 2 (3+3) fica como contingência com gatilhos objetivos |
| LLM | Apenas API externa (OpenRouter) | Carga de CPU/RAM na VPS é menor; latência depende do provedor |
| Backup PostgreSQL | Full semanal + diff diário + incremental a cada 6h, retenção 30 dias | RPO baixo, storage controlado |
| Estado da aplicação | `server` stateless, `worker` com fila no banco | 2+2 réplicas de server são viáveis; workers escalam via banco |
| Deploy | CI/CD via GitHub Actions como padrão; `deploy-local.sh` como contingência | Pipeline publica imagem e faz SSH para `docker stack deploy` |
| Crescimento do banco | Não estimado; usar premissa conservadora de até 30 GB | Alerta em 80 GB; volume extra se necessário |
| Idempotência de jobs | Auditar e garantir antes de escalar para 2+ workers | RF de pré-condição para scaling de workers |
| Segredos | `.env` no host para bootstrap; Docker secrets no Swarm | Segredos sensíveis não trafegam em variáveis de ambiente plain |
| Proteção de borda | Caddy 2 + plugin `caddy-ratelimit` | Rate limit global de 100 req/s por IP, burst 200 |
| Testes de restore | Mensais | Prova real da recuperação |
| Domínio | Configurado; Caddy obtém TLS automaticamente | Requisito de borda atendido |
| PostgreSQL | Permanecer na versão 16 até próximo ao EOL (2028), aplicando minor updates | Técnica de dívida documentada |
| Janela de manutenção | 22h–06h BRT | Deploys e manutenção fora do horário comercial |
| Alertas | Telegram (canal já configurado) | Mínimo aceitável para time pequeno |
| Acesso SSH | Chave SSH + usuário não-root + fail2ban + MFA | Hardening do host |
| Firewall | Apenas 22/80/443 abertos | Superfície de ataque reduzida |
| Rollback de deploy | Manual via `deploy-local.sh` ou CI/CD dispatch | Requer alguém acordado na janela de manutenção |
| Staging | Ambiente separado ou local com dados anonimizados | Validação antes de produção |
| LGPD | Dados anonimizados após 365 dias; exclusão sob demanda | Converge com configuração existente de billing |
| Observabilidade | LGTM permanece na VPS com retenção curta e limits rígidos | Risco de contenção conhecido e aceito |
| Comunicação de incidentes | Canal oficial do app (WhatsApp/Telegram) | Usa a própria plataforma; risco se ela estiver indisponível |
| On-call | Time fundador/engenharia | Runbooks devem ser claros e testados |
| Orquestração de réplicas | Docker Swarm single-node | Permite `deploy.replicas`, rolling updates e service discovery básica |
| Upstreams do Caddy | 2 services separados (`server-1`, `server-2`) no início, expandindo para 3 com health checks ativos | Health check por réplica real, sem depender de DNS round-robin |
| Backup de volumes | Apenas banco (pgBackRest) + configs versionadas no Git | Caddy data e Grafana dashboards são recriáveis/versionados |
| Rotação de segredos | Anual | Processo documentado; `runbook/rotate-secret.md` já existe |
| Gatilhos de upgrade de VPS | CPU > 70% ou RAM > 80% por mais de 15 min em pico | Decisão de upgrade baseada em dados |
| Patches de segurança | Mensais | Janela de manutenção + restart se necessário |
| Disaster recovery total | Recriar VPS do zero a partir de backups S3 + configs no Git | RTO alto (horas), mas viável |
| Multi-node futuro | Talvez, se custo justificar | Não é compromisso; fase futura |
| Backup do `.env` | S3 com SSE-S3 + IAM restricto, sincronizado a cada alteração | Protege contra perda do host |
| Migração para Swarm | Janela de manutenção única; parar Compose, subir stack Swarm, validar | Sem snapshot/rollback formal; risco aceito e registrado |
| CI/CD para Swarm | GitHub Actions faz SSH para VPS e executa `docker stack deploy` | Requer credenciais SSH seguras no CI |
| Locking da fila de jobs | `SELECT FOR UPDATE SKIP LOCKED` no PostgreSQL | Garante que múltiplos workers não processam o mesmo job |
| Retry de jobs | Backoff exponencial + DLQ após 3 tentativas | Converge com configuração atual de outbox |
| Thresholds de alerta | Disco > 80%, CPU > 70% por 5min, RAM > 80%, WAL lag > 15min, fila de jobs > 1000 | Alertas acionáveis de saturação |
| `statement_timeout` | 30 segundos | Proteção contra queries runaway |
| Concorrência de migrations | Advisory lock no PostgreSQL ou orquestração que permita apenas uma instância | Evita corrupção de schema migrations |
| Testes de carga | Benchmark mensal com k6/locust, executado localmente | Limitação: não reflete infra real; recomenda-se runner externo quando possível |
| Volumes no Swarm | Named volumes locais do Docker no mesmo host | Funciona em single-node; não é HA |

---

## Base Documental Oficial Consultada

| Fonte | URL | Conclusões aplicadas |
|---|---|---|
| PostgreSQL 16 / 18 Docs | `https://www.postgresql.org/docs/` | PostgreSQL 16 permanece suportado até novembro/2028; série current é 18. Recomendação geral: aplicar minor updates e planejar upgrade major antes do EOL. `wal_level=replica` habilita archiving e replicação física. PITR exige `archive_mode=on` e `archive_command` funcional. |
| Docker / Docker Compose / Swarm | `https://docs.docker.com/compose/`, `https://docs.docker.com/engine/swarm/` | Compose single-host é viável com práticas operacionais. Para múltiplas réplicas com `deploy.replicas`, Docker Swarm é a opção nativa. Swarm single-node permite rolling updates e service discovery, mas não elimina o ponto único de falha físico. |
| Caddy Reverse Proxy | `https://caddyserver.com/docs/caddyfile/directives/reverse_proxy` | Load balancing entre upstreams com `lb_policy`, health checks ativos (`health_uri`, `health_interval`, `health_timeout`, `health_status`) e passivos (`fail_duration`, `max_fails`). Health checks ativos por upstream requerem endereços explícitos. |
| pgBouncer | `https://www.pgbouncer.org/config.html` | `pool_mode=transaction` é o padrão para aplicações web transacionais. `default_pool_size` deve ser dimensionado a partir da capacidade do Postgres (CPU/cores), não do número de clientes. `max_db_connections` atua como válvula de segurança. |
| pgBackRest | `https://pgbackrest.org/user-guide.html` | Suporta full/differential/incremental, retenção configurável, criptografia client-side, S3, PITR. Testes regulares de restore são mandatórios. Retention deve ser alinhada ao RPO/RTO. |
| Grafana / OpenTelemetry | `https://grafana.com/docs/`, `https://opentelemetry.io/docs/` | Stack de observabilidade monolítica no mesmo host da aplicação cria contenção de recursos; recomenda-se instância dedicada para produção séria. |
| Hostinger KVM2 | `https://www.vpsbenchmarks.com/hosters/hostinger/plans/kvm-2` e especificações comerciais | 2 vCPU cores, 8 GB RAM, 100 GB NVMe SSD, 8 TB bandwidth. |

---

## Projeção Realista para 10.000 Usuários Ativos

### Volume e distribuição

- **Interações diárias:** 10.000 usuários ativos × 15 interações/dia = **150.000 interações/dia**.
- **Janela de pico estimada:** 12h/dia (07h–23h BRT), concentrando ~70% do volume.
- **Média em pico:** 150.000 × 0,70 / 12 ≈ **8.750 interações/hora** ≈ **146/min** ≈ **2,4 req/s**.
- **Burst esperado:** 3×–5× em função de comportamento humano e eventos (início/fim de hora, notificações). Pico sustentado: **~10–15 req/s** na API.

### Trabalho assíncrono

- Cada interação conversacional pode disparar jobs de outbox, processamento via LLM, materialização de transações, reconciliação de assinaturas etc.
- O `worker` deve absorver burst de jobs sem degradar a latência da API.

### Implicações para réplicas

- **2 réplicas de `server`** distribuem carga HTTP e eliminam o risco de restart/deploy como evento de indisponibilidade total.
- **2 réplicas de `worker`** garantem throughput de jobs e resiliência a falhas de processo.
- **3 réplicas de cada** são a meta para dezembro/2026, mas dependem de upgrade de hardware.

### Pressão sobre recursos — configuração inicial 2+2

| Serviço | Quantidade | CPU | RAM |
|---|---:|---:|---:|
| `server` | 2 | 2,0 | 2,0 GB |
| `worker` | 2 | 1,0 | 1,0 GB |
| `postgres` | 1 | 1,0 | 2,0 GB |
| `pgbouncer` | 1 | 0,25 | 128 MB |
| `caddy` | 1 | 0,5 | 256 MB |
| `otel-lgtm` | 1 | 1,0 | 1,0 GB |
| **Total aproximado** | | **4,75 vCPU** | **6,4 GB** |

> Esse total **não inclui overhead do host**, cache do PostgreSQL, buffers do sistema, WAL, logs e retenção de métricas/traces.

### Pressão sobre recursos — configuração-alvo 3+3

| Serviço | Quantidade | CPU | RAM |
|---|---:|---:|---:|
| `server` | 3 | 3,0 | 3,0 GB |
| `worker` | 3 | 1,5 | 1,5 GB |
| `postgres` | 1 | 1,0 | 2,0 GB |
| `pgbouncer` | 1 | 0,25 | 128 MB |
| `caddy` | 1 | 0,5 | 256 MB |
| `otel-lgtm` | 1 | 1,0 | 1,0 GB |
| **Total aproximado** | | **6,25 vCPU** | **7,9 GB** |

### Confronto com Hostinger KVM2

| Recurso | KVM2 | Demanda 2+2 | Demanda 3+3 | Situação |
|---|---|---|---|---|
| vCPU | 2 | 4,75 | 6,25 | **2,4×–3× acima da capacidade** |
| RAM | 8 GB | ~6,4 GB + overhead | ~7,9 GB + overhead | **apertado, especialmente para 3+3** |
| NVMe | 100 GB | dados + WAL + logs + métricas/traces | dados + WAL + logs + métricas/traces | **provavelmente insuficiente em ambos** |
| Bandwidth | 8 TB/mês | depende de payload | depende de payload | geralmente adequado |

### Conclusão de capacidade

A configuração **2+2 réplicas é desafiadora na Hostinger KVM2**, pois a demanda de CPU (~4,75 vCPU) ainda excede os 2 vCPU disponíveis, exigindo oversubscription significativo do scheduler. No entanto, é o **ponto de partida realista** dentro do orçamento zero e representa melhoria sobre a única réplica atual.

A configuração **3+3 réplicas não é viável na KVM2** sem degradação severa, risco de OOM kill e latência inaceitável. Só deve ser ativada após upgrade para no mínimo **8 vCPU / 32 GB RAM** (Hostinger KVM8) ou arquitetura multi-node.

### Margem de segurança recomendada

- Utilização máxima sustentável: **60–70%** de CPU/RAM em pico.
- Para operar **2+2 réplicas** com folga: recomenda-se **4 vCPU / 16 GB RAM** (Hostinger KVM4).
- Para operar **3+3 réplicas** com folga: mínimo de **8 vCPU / 32 GB RAM** (Hostinger KVM8) ou separação de banco/observabilidade em nodes dedicados.

---

## Problema

A infraestrutura atual foi construída para poucos usuários e precisa ser fortalecida agora para suportar **10.000 usuários ativos em dezembro/2026** sem grandes refações estruturais posteriores.

O problema central é a **incompatibilidade entre a meta de capacidade (3 réplicas de `server` e `worker`, alta confiabilidade) e as restrições reais de uma VPS única de entrada**. A decisão de iniciar com **2+2 réplicas** reduz o gap, mas não o elimina. Sem revisão, os riscos operacionais são:

- indisponibilidade total em deploys por ter apenas uma réplica;
- contenção de recursos, OOM kills e latência degradada;
- falta de isolamento entre falhas de componentes;
- observabilidade e banco competindo pela mesma máquina;
- falsa sensação de robustez advinda apenas do uso de Docker e pgBackRest.

---

## Objetivos

1. Definir arquitetura alvo de execução que suporte **2 réplicas de `server` e 2 de `worker` no início**, evoluindo para **3 réplicas de cada** quando provisionada com recursos adequados.
2. Remover fragilidades lógicas de single-node: health checks, startup ordenado, shutdown gracioso, rollback e observabilidade mínima.
3. Garantir que banco, backup, proxy e aplicação sigam práticas recomendadas oficiais.
4. Deixar explícitas as limitações da VPS única e os gatilhos de upgrade/multi-node.
5. Estabelecer critérios objetivos de **"production-ready"** para esta fase.

---

## Escopo Incluído

- Arquitetura alvo de execução para `server`, `worker`, `postgres`, `pgbouncer`, `pgbackrest`, `caddy`, observabilidade e rotinas de `migrate`.
- Requisitos de disponibilidade e comportamento esperado em reinício, crash, deploy, rollback e degradação parcial.
- Requisitos para iniciar com **2 réplicas de `server` e 2 de `worker`**, com caminho para escalar para **3 réplicas de cada**.
- Requisitos de balanceamento, health checks, startup/shutdown ordenado, readiness e liveness.
- Requisitos de isolamento de rede, exposição de portas, TLS, headers de segurança, segredos e hardening.
- Requisitos de persistência, pooling, tuning, backups, restore, PITR e recuperação de desastre.
- Requisitos de observabilidade operacional mínima: logs, métricas, traces, alertas e sinais de saturação.
- Projeção de capacidade e performance compatíveis com 10.000 usuários ativos e 15 interações diárias por usuário.
- Requisitos derivados de projeção de carga realista, cobrindo pico, concorrência, burst, reprocessamento e folga operacional.
- Requisitos de operação segura em VPS única, deixando explícito o que é robusto e o que continua sendo risco por ser single-node.
- Requisitos para deploy repetível, previsível, auditável e reversível.
- Requisitos para evitar pontos únicos de falha lógicos dentro do possível no contexto informado.

---

## Escopo Excluído

- Reescrita do sistema em microserviços.
- Mudança de linguagem.
- Troca de provedor cloud sem evidência objetiva de necessidade.
- Implementação detalhada de pipelines CI/CD (o repositório já possui CI/CD; apenas requisitos de deploy serão tratados).
- Passo a passo de comandos de implantação.
- Mudanças de produto não relacionadas à robustez da infraestrutura.

---

## Requisitos Funcionais

- **RF-01:** A arquitetura de execução deve permitir operar **2 réplicas de `cmd/server` e 2 réplicas de `cmd/worker` no início**, evoluindo para **3 réplicas de cada** quando provisionada com recursos adequados (mínimo indicado: 8 vCPU / 32 GB RAM ou arquitetura multi-node).
- **RF-02:** A orquestração deve usar **Docker Swarm single-node**, permitindo `deploy.replicas`, rolling updates e service discovery básica.
- **RF-03:** O proxy de borda (Caddy) deve listar **2 upstreams explícitos** (`server-1`, `server-2`) no início, expandindo para 3 quando a terceira réplica for ativada, todos com **health checks ativos** e política de load balancing (`lb_policy`) explicitamente definida.
- **RF-04:** O `server` deve expor endpoints distintos de **readiness** (pronto para receber tráfego) e **liveness** (processo saudável), consumidos pelo proxy e pelo orquestrador.
- **RF-05:** O `worker` deve expor endpoint de readiness/liveness que valide a capacidade real de processar jobs, não apenas a existência do processo.
- **RF-06:** A ordem de startup deve ser determinística: `postgres` saudável → `pgbouncer` saudável → `migrate` executado com sucesso → `server`/`worker` saudáveis → `caddy` saudável.
- **RF-07:** O shutdown deve ser gracioso: `server` deve drenar conexões ativas antes de sair; `worker` deve finalizar job em execução ou devolver para a fila dentro de timeout seguro.
- **RF-08:** A rede `backend` deve ser isolada (overlay encrypted no Swarm ou `internal` no Compose) e a `frontend` deve expor apenas o Caddy nas portas 80/443.
- **RF-09:** O PostgreSQL não deve ser exposto publicamente; a porta 5432 deve permanecer vinculada a `127.0.0.1` ou a rede interna apenas.
- **RF-10:** Todos os segredos sensíveis devem ser injetados via **Docker secrets** no Swarm; o `.env` no host serve apenas para bootstrap/administração e nunca entra em imagem ou config versionada.
- **RF-11:** O arquivo `.env` de produção deve ser sincronizado para o **S3 criptografado (SSE-S3 + IAM restricto) a cada alteração**, permitindo recuperação em caso de perda total da VPS.
- **RF-12:** A imagem da aplicação deve continuar sendo distroless, nonroot, com tag imutável (digest ou SHA), e **nunca `latest` em produção**.
- **RF-13:** O pool de conexões PostgreSQL (pgBouncer + app-side pool) deve ser dimensionado para que o número real de backends ao Postgres não exceda `max_connections` menos `superuser_reserved_connections`, considerando 2 servers + 2 workers + jobs + admin (expandindo para 3+3 no futuro).
- **RF-14:** O PostgreSQL deve operar com `wal_level=replica`, `archive_mode=on`, `archive_command` funcional e `statement_timeout=30s` quando pgBackRest estiver habilitado, permitindo PITR e proteção contra queries runaway.
- **RF-15:** Backups full devem ser executados **semanalmente**, diferenciais **diariamente** e incrementais **a cada 6 horas**, com retenção de **30 dias**.
- **RF-16:** Backups e arquivos WAL devem ser armazenados em repositório **off-VM** (AWS S3 `us-east-1`, Standard, lifecycle para Glacier após 90 dias) com criptografia.
- **RF-17:** Deve existir runbook testado de restore PITR e runbook de restore completo da VPS.
- **RF-18:** A stack de observabilidade deve coletar logs estruturados (JSON), métricas de infraestrutura e aplicação, traces amostrados, e expor alertas mínimos de saturação (disco > 80%, CPU > 70% por 5min, RAM > 80%, WAL lag > 15min, fila de jobs > 1000).
- **RF-19:** O deploy deve ser repetível: mesma imagem, mesma configuração, mesma ordem de startup; deve ser reversível para a imagem anterior em caso de falha de healthcheck.
- **RF-20:** Todos os serviços devem ter `restart` policy, resource limits/reservations e logging rotativo configurados.
- **RF-21:** Deve existir proteção de **rate limiting** na borda com Caddy: 100 req/s por IP, burst 200.
- **RF-22:** Deve existir estratégia de housekeeping de logs, métricas, traces e eventos de auditoria para evitar consumo ilimitado de disco.
- **RF-23:** A fila de jobs do `worker` deve usar **locking no banco** (`SELECT FOR UPDATE SKIP LOCKED`) para garantir que múltiplos workers não processem o mesmo job.
- **RF-24:** Jobs devem ser **idempotentes** antes de escalar para 2+ workers; deve haver auditoria que comprove essa condição.
- **RF-25:** O serviço `migrate` deve usar **advisory lock** no PostgreSQL (ou orquestração equivalente) para garantir execução única em deploys concorrentes.
- **RF-26:** Jobs que falham devem ter **retry com backoff exponencial** e mover para **DLQ após 3 tentativas**.

---

## Requisitos Não-Funcionais

- **RNF-01 (Disponibilidade):** Não há alta disponibilidade real em VPS única. O objetivo é minimizar o tempo de indisponibilidade em deploys e crashes de processo, com failover de réplica quando houver múltiplas réplicas. RPO/RTO realista para KVM2 única: **RPO ≤ 1 hora**, **RTO ≤ 4 horas** (restore manual + validação).
- **RNF-02 (Latência):** p95 das requisições HTTP não críticas < 500 ms; p99 < 1.000 ms em condições normais. Latência de jobs assíncronos < 30s em percentil 95.
- **RNF-03 (Throughput):** suportar burst de até **15 req/s** na API e processamento correspondente de jobs sem erros 5xx ou perda de mensagens.
- **RNF-04 (Segurança):** containers rodando como nonroot, read-only rootfs, `no-new-privileges`, `cap_drop ALL`; TLS 1.2+ automático; headers de segurança; admin/debug endpoints bloqueados na borda; SSH com chave + não-root + fail2ban + MFA; firewall apenas 22/80/443.
- **RNF-05 (Operabilidade):** deploy com tag imutável via CI/CD (SSH + `docker stack deploy`); rollback manual ≤ 10 min; logs centralizados; alertas acionáveis via Telegram.
- **RNF-06 (Escalabilidade):** a arquitetura deve permitir escalar `server` e `worker` horizontalmente adicionando réplicas no Swarm, sem alteração de código, quando houver recursos.
- **RNF-07 (Manutenção):** patches de segurança mensais no host e containers; janela de manutenção 22h–06h BRT.
- **RNF-08 (Governança de dados):** dados pessoais anonimizados após 365 dias; exclusão sob demanda conforme LGPD.

---

## Riscos, Restrições e Dependências

- **Risco 1 — Recursos insuficientes na KVM2:** mesmo a configuração 2+2 excede a CPU da VPS (~4,75 vCPU vs 2 disponíveis). **Mitigação:** iniciar com 2+2, monitorar thresholds (CPU > 70% ou RAM > 80% por 15 min), e definir upgrade para KVM4/KVM8 como próximo passo.
- **Risco 2 — Ponto único de falha físico:** falha do host da Hostinger ou corrupção do volume PostgreSQL só é recuperável via backup off-VM. **Mitigação:** pgBackRest para S3, backup do `.env`, testes regulares de restore.
- **Risco 3 — Contenção de recursos:** observabilidade LGTM na mesma máquina consome CPU/RAM e disco. **Mitigação:** retenção curta (7 logs / 15 métricas / 7 traces) e limits rígidos.
- **Risco 4 — Connection pool mal dimensionado:** 2+2 servers/workers + jobs podem esgotar as conexões do Postgres. **Mitigação:** pgBouncer transaction mode + app-side pool conservador + monitoramento.
- **Risco 5 — Versão defasada do PostgreSQL:** PostgreSQL 16 está suportado até 11/2028, mas a série current é 18. **Dependência:** manter minor updates e planejar upgrade major antes do EOL.
- **Risco 6 — Migração para Swarm sem snapshot/rollback:** a janela de manutenção única não prevê rollback rápido. **Mitigação:** testar exaustivamente em staging antes; manter backups S3 e configs no Git.
- **Risco 7 — Testes de carga locais:** não refletem a infraestrutura real de produção. **Mitigação:** usar como sanity check; priorizar runner externo assim que possível.
- **Risco 8 — Idempotência de jobs não confirmada:** scaling de workers depende de auditoria. **Mitigação:** RF-24 bloqueia scaling até confirmação.
- **Restrição 1 — Orçamento zero para infra adicional:** sem segunda VPS ou storage gerenciado. O PRD aceita isso e ajusta RPO/RTO e capacidade realista.
- **Dependência 1 — Bucket AWS S3 configurado para pgBackRest e backup do `.env`.** Sem ele, não há recuperação de desastre real.
- **Dependência 2 — Domínio e DNS configurados** para Caddy obter TLS via Let's Encrypt.
- **Dependência 3 — Credenciais SSH seguras no GitHub Actions** para deploy no Swarm.

---

## Drift entre Estado Atual e Prática Recomendada

| Área | Estado Atual | Prática Recomendada (fonte) | Drift |
|---|---|---|---|
| PostgreSQL | Versão 16 | Current = 18; 16 suportado até 11/2028 | Moderado: requer plano de upgrade |
| Orquestração | Docker Compose single-host | Docker Swarm para réplicas múltiplas | Alto: impede 2+2/3+3 réplicas |
| Réplicas `server`/`worker` | 1 de cada | 2 de cada no início; 3 para meta final | Alto: arquitetura não atende meta na VPS atual |
| Caddy load balancing | 1 upstream (`server:8080`) | 2 upstreams explícitos + health checks ativos (Caddy docs) | Alto: sem failover de réplica |
| Health checks | Básicos (`wget`/`pgrep`) | Readiness/liveness distintos e significativos | Moderado |
| PostgreSQL `archive_mode` | `off` no conf base | `on` para PITR (PostgreSQL docs) | Alto se não ativado via setup |
| `statement_timeout` | 0 | 30s para proteção contra runaway | Alto |
| Secrets | `.env` montado nos containers | Docker secrets no Swarm | Moderado/Alto |
| pgBouncer pool | `default_pool_size=20`, `max_db_connections=50` | Dimensionar a partir de CPU/cores do Postgres | Moderado: requer validação sob carga |
| Observabilidade | LGTM no mesmo host | Instância dedicada em produção séria | Moderado: risco de contenção |
| Backup do `.env` | Não existe | S3 criptografado a cada alteração | Alto |
| Resource limits | Definidos | Devem ser revalidados para carga projetada | Moderado/Alto |
| Deploy | CI/CD + `deploy-local.sh` | Repetível, reversível, auditável; adaptar para Swarm | Moderado |

---

## Critérios de Sucesso Mensuráveis

- **CS-01:** Infraestrutura documentada e revisada para suportar 10.000 usuários ativos, com projeção de capacidade validada para 2+2 e 3+3 réplicas.
- **CS-02:** Arquitetura capaz de operar **2 réplicas de `server` e 2 de `worker` no início**, evoluindo para 3 de cada quando provisionada com recursos adequados (mínimo 8 vCPU / 32 GB RAM).
- **CS-03:** SLOs definidos: disponibilidade ≥ 99,5% (limitado por VPS única), p95 latência API < 500 ms, taxa de erro 5xx < 0,5%.
- **CS-04:** Estratégia de backup, restore e PITR documentada e testada pelo menos uma vez antes de dezembro/2026.
- **CS-05:** Critérios de rollback definidos: healthcheck pós-deploy falha → reversão para imagem anterior em ≤ 10 min.
- **CS-06:** **Production-ready** definido como: (a) stack Swarm funcional; (b) 2 upstreams no Caddy com health checks; (c) readiness/liveness nos serviços; (d) startup ordenado; (e) shutdown gracioso; (f) backups off-VM testados; (g) `.env` backup no S3; (h) observabilidade mínima operacional; (i) runbooks de deploy/restore/rollback atualizados; (j) auditoria de idempotência dos jobs concluída.

---

## Premissas Fixas

As seguintes premissas foram validadas e fixadas para a projeção e requisitos deste PRD:

- **Premissa 1 — Tamanho do banco:** crescimento estimado de até **30 GB** até dezembro/2026. Se o volume real se aproximar de **80 GB**, o disco de 100 GB da KVM2 entrará em risco e será necessário volume extra ou housekeeping agressiva.
- **Premissa 2 — Tráfego:** média de **150.000 interações/dia** (10.000 usuários ativos × 15 interações/dia), distribuídas em janela de pico de 12h, com **burst de 3×–5×** sobre a média, resultando em pico sustentado de **~10–15 req/s** na API.
- **Premissa 3 — Jobs assíncronos:** média de **3 jobs por interação** do usuário (ex.: outbox, processamento LLM, materialização de transações), gerando carga correspondente na fila do `worker`.

## Dependências Externas

As seguintes dependências não estão sob controle direto do escopo técnico e são tratadas como restrições de negócio:

- **Dependência externa 1 — Orçamento futuro:** upgrade para KVM4/KVM8 ou segunda VPS depende do crescimento de receita. Enquanto isso não ocorrer, a capacidade real permanece limitada a 2+2 na KVM2.
- **Dependência externa 2 — Multi-node futuro:** arquitetura multi-node é **fase futura opcional**, sem compromisso de cronograma. Só será considerada se o custo/benefício justificar.

> **Nota:** todas as questões materiais levantadas durante as rodadas de esclarecimento foram convertidas em requisitos, decisões, riscos, premissas fixas ou dependências externas registradas neste documento.
