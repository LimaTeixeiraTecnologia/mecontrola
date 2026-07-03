<!-- spec-hash-prd: d7a97a8fa98a63232897442fa44617bc6e7ef670cf9fbd519f40989469e35b47 -->
<!-- spec-hash-techspec: 6c8376de1dd038dd69d33c0bb8c8dd330cd23cf3d03a3b1da6c2c851242be5bd -->
<!-- MANDATÓRIO: preenchido por `ai-spec sync-spec-hash` com sha256 do PRD consumido.
     NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Evolução de Infraestrutura KVM 2 (envelope B)

PRD: `.specs/prd-infra-evolucao-kvm2-10k/prd.md` (v1, RF-01..22)
Relatório-base: `docs/runs/2026-07-03-relatorio-analise-infra-hostinger-kvm2-10k.md`

## Resumo Executivo

Todas as mudanças são de **operação, pipeline e configuração** sobre a stack existente (Docker Swarm single-node na VPS Hostinger KVM 2). Não há nova arquitetura de aplicação, novo broker, sharding nem HA. O trabalho fecha os gaps A–M do relatório dentro do alvo de **envelope B**, otimizando eficiência, economia e robustez. Onde houver código Go (config de pool, gate de drift, rate-limit), aplica-se `go-implementation` e as regras `.claude/rules/*` (zero comentários, DTO validate, adapters finos).

Estado de referência verificado (SSH, 2026-07-03): host 2 vCPU / 7,75 GiB / 96 GiB / swap 4 GiB; Swarm 1 nó (Leader); prod em imagem app `571425f` e postgres custom `mecontrola-postgres:mastra-*`; pgBackRest com 3 fulls (último 2026-07-01) e sem cron; runner GitHub em `/home/github-runner`; build cache 22 GiB; `OTEL_TRACE_SAMPLE_RATE` efetivo "1" no compose.

## Requisitos Técnicos

### REQ-01 — Automação e alerta de backup (cobre RF-01, RF-02, RF-03)
- Provisionar o agendamento de `deployment/pgbackrest/crontab.txt` de forma idempotente e versionada: script `pgbackrest-setup.sh` estende-se (ou novo `deployment/scripts/pgbackrest-schedule.sh`) para instalar um **systemd-timer** (ou cron do host) que dispara `pgbackrest ... backup` dentro do container postgres; o agendamento é aplicável a partir do repositório, sem edição manual não rastreada.
- Alerta de staleness: regra Prometheus/Grafana (provisionada em `deployment/telemetry/grafana/provisioning/alerting/rules.yaml` ou `deployment/monitoring/prometheus-rules.yaml`) sobre a idade do último backup e sobre falha de `archive-push`, exportada via um coletor simples (script que publica métrica de "idade do último backup" — ex.: textfile do node-exporter ou endpoint scrapeável).
- Guard de imagem: `deployment/scripts/deploy-swarm.sh` e/ou `render-stack.py` **falham** o deploy de produção se `POSTGRES_IMAGE` resolver para `postgres:*-alpine` (sem pgBackRest); exige tag `mecontrola-postgres:*`.
- Evidência de aceite: `pgbackrest --stanza=mecontrola info` mostra full ≤ 7 d e diff ≤ 24 h; alerta dispara em teste de staleness forçado.

### REQ-02 — Ensaio de restore PITR e restore de VPS (cobre RF-04, RF-05, RF-06)
- Restore PITR: em ambiente isolado (container/VPS descartável), `pgbackrest --stanza=mecontrola --type=time "--target=<ts>" restore`, subir Postgres, validar integridade (contagem/консистência de tabelas-chave), medir RTO.
- Restore de VPS: seguir `restore-vps.md` do zero (provisionar host, clonar repo, restaurar secrets via `backup-env-s3.sh`, restaurar banco, subir stack), medir RTO.
- Atualizar `deployment/runbooks/restore-pitr.md` e `restore-vps.md` com RPO/RTO reais e SLO; anexar evidência (log/relatório) via `ai-spec validate-evidence` quando aplicável.
- Sem mudança em produção; apenas ambiente de ensaio.

### REQ-03 — Remoção do runner do host + rewire do deploy (cobre RF-07, RF-08, RF-09, RF-10)
- Remoção: desregistrar o runner no GitHub, `systemctl disable/stop` do serviço, remover `/home/github-runner/actions-runner` e o usuário; `docker builder prune -af` e `docker image prune -af` para recuperar ~50 GiB.
- Rewire de `.github/workflows/ci-cd.yml`: jobs `build`, `build-image`, `scan-image`, `sign-image` em `ubuntu-24.04` (GitHub-hosted, já é o caso da maioria); job `deploy` passa de `runs-on: [self-hosted, staging]` para `ubuntu-24.04` executando `deployment/scripts/deploy-swarm.sh` **via SSH** (docker context remoto ou `ssh` explícito), com a chave em secret e host key fixada; preserva descriptografia SOPS, `create-secrets.sh`, migrate com advisory lock, `docker stack deploy`, waiters de health e rollback.
- Higiene: cron/systemd-timer de `docker system prune` controlado + alerta de disco (regra sobre `node_filesystem_avail_bytes`).
- Evidência: ausência de processo runner (`ps`), `df /` com disco majoritariamente livre, deploy verde pelo novo caminho.

### REQ-04 — Sampling de traces proporcional (cobre RF-11, RF-12, RF-13)
- Alinhar o sampling: definir `OTEL_TRACE_SAMPLE_RATE` efetivo único (ex.: `0.1`) em produção, com **error-biased sampling** (100% de traces com erro) via configuração do OTel Collector em `deployment/telemetry/grafana/otelcol-config.yaml` (tail-sampling policy) ou parent-based no SDK; remover a divergência entre `compose.swarm.yml` (hoje `"1"`) e `prod.env` (`0.1`).
- Ajustar `scripts/ci/deploy-anti-storm.sh` para validar o novo valor acordado em vez de exigir `"1"`, mantendo as demais invariantes do gate.
- Documentar em `deployment/runbooks/` o SPOF de observabilidade aceito e a retenção (30 d) dos sinais.

### REQ-05 — Orçamento de recursos e pool (cobre RF-14, RF-15, RF-16)
- Rever `limits`/`reservations` em `compose.swarm.yml` para que a soma de `limits` de memória caiba com margem em 8 GB menos overhead (reduzir tetos superdimensionados de `server`/`worker`/`otel-lgtm` sem estrangular o p95 medido no REQ-06); manter `reservations` como garantia de scheduling.
- Pool: revisar `DB_MAX_CONNS`/pgBouncer (`DEFAULT_POOL_SIZE`, `MAX_DB_CONNECTIONS`) para folga sobre 4 processos × conexões + jobs; adicionar alerta de saturação (`postgres-exporter`/pgBouncer stats) em Grafana.
- Documentar o orçamento aprovado e o gatilho de upgrade KVM 2 → KVM 4 (ex.: p95 de CPU do host > X% sustentado, ou memória disponível < Y por Z min).
- Código Go (se `configs/config.go` mudar defaults) segue `go-implementation` + `R-DTO-VALIDATE-001` quando aplicável.

### REQ-06 — Harness de carga e prova de A/B (cobre RF-17, RF-18)
- Estender `scripts/loadtest/` com perfis proporcionais aos envelopes A e B (webhook WhatsApp, outbox drain, e leitura quando disponível), parametrizados por VUs/duração; integrar como target de Taskfile.
- Definir thresholds (p95, `http_req_failed`, `mecontrola_db_pool_in_use`, CPU do host) e produzir relatório de evidência em `docs/runs/`; veredito honesto (`comprovado`/gap) por envelope.
- Executar contra staging/ambiente equivalente ao host (não sobre produção com usuários reais).

### REQ-07 — Deploy da main + alerta de drift (cobre RF-19, RF-20)
- Deployar a `main` corrente via o novo pipeline (REQ-03) e confirmar saúde; produção sai de `571425f`.
- Alerta de drift: métrica/gate que compara `OTEL_SERVICE_VERSION` em execução com o `HEAD` de `main` (job agendado no CI ou regra sobre a métrica de versão exposta), disparando quando divergir por mais de um limiar de tempo.

### REQ-08 — Endurecimento de superfície (cobre RF-21, RF-22)
- Rate-limit × Meta: em `deployment/caddy/Caddyfile` (e/ou `WHATSAPP_WEBHOOK_RATE_LIMIT_*` no app), aplicar allowlist/limite específico para os blocos de IP de origem da Meta, preservando a proteção contra abuso para o restante; validar contra a doc oficial da Meta sobre IPs de webhook.
- `pg-tunnel`: alterar o bind de `0.0.0.0:15432` para loopback (ou tornar o serviço opt-in/removível quando ocioso) em `compose.swarm.yml`, mantendo o ufw como segunda camada.

## Estratégia de Testes

- Unit: mudanças Go (config de pool, gate de drift, rate-limit) com testify/suite (`R-TESTING-001`).
- Integração/operacional: ensaios de restore (REQ-02) e carga (REQ-06) com evidência anexada.
- Validação de pipeline: dry-run do deploy rewired (REQ-03) contra staging antes de produção.
- Gates de CI existentes (lint, vulncheck, Trivy, cosign, anti-storm ajustado) permanecem verdes.

## Riscos e Mitigações

- **Rewire de deploy quebrar a entrega** → validar via SSH em staging antes; manter rollback automático; janela de baixa demanda.
- **Sampling reduzido esconder incidentes** → error-biased 100% em traces com erro + métricas/logs preservados.
- **Redução de `limits` estrangular p95** → calibrar com base no REQ-06 (medir antes de cortar).
- **SPOF single-node permanece** → aceito por D-01, documentado; DR comprovado (REQ-02) reduz o impacto.
