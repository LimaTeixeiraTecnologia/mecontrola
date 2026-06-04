# Plano de Infraestrutura VPS Hostinger para o `mecontrola`

> **Tipo de entrega**: este "plano" é o próprio artefato pedido pelo prompt
> `docs/prompts/infra-vps-hostinger-production-ready.md`. Não há código a ser
> escrito; o escopo do prompt é planejamento. Replicado de
> `/Users/jailtonjunior/.claude/plans/execute-users-jailtonjunior-git-mecontro-mossy-reef.md`
> conforme regra de memória `feedback_save_plans_to_docs_runs`.

## Context

- O repositório `mecontrola` é um monolito modular em Go 1.26 (`internal/identity`,
  `internal/billing`, `internal/platform`) com 3 entrypoints (`cmd/server`,
  `cmd/worker`, `cmd/migrate`) e PostgreSQL como banco principal
  (`go-migrate/v4`, `pgx/v5` em `go.sum`).
- Já existe `deployment/docker/Dockerfile` multi-stage → distroless nonroot
  (≤30 MB), `deployment/fly/fly.toml` (tentativa anterior em Fly.io) e
  runbooks operacionais em `deployment/runbooks/` (deploy, rollback,
  restore-pitr, rotate-secret).
- Restrições do usuário (esta sessão):
  - Foco em **MVP**, **menor custo** e **eficiência possível**.
  - Volume alvo: **até 3 mil usuários em 6 meses** (carga modesta).
  - Hospedagem em **VPS Hostinger via hPanel**.
  - Validação **local-first** antes de publicar.
- Restrições do prompt: não implementar nada, não criar arquivos no repo, não
  expor banco/Docker daemon, sem K8s, segredos fora do Git, SSH por chave.
- Saída obrigatória: Markdown em pt-BR com decisões justificadas, fontes
  oficiais 2026, checklist production-ready e plano futuro de implementação.

---

# Plano de Infraestrutura VPS Hostinger para o mecontrola

## Sumário Executivo

- **Stack recomendada**: Hostinger **KVM 2** (Ubuntu 24.04 LTS) → Docker
  Engine + Docker Compose v2 → **Caddy 2** (reverse proxy, HTTPS automático)
  → 3 containers Go (server, worker, migrate-one-shot) → **PostgreSQL 16
  containerizado** no mesmo host com volume nomeado → backups semanais via
  hPanel + `pg_dump` diário em offsite gratuito (Backblaze B2 free tier ou
  Cloudflare R2) → observabilidade mínima com `slog`/JSON em stdout coletado
  por Promtail → Grafana Cloud free tier.
- **Por que equilibra economia, segurança e robustez**:
  - KVM 2 (≈ US$ 7–9/mês, ~ R$ 35–45) entrega 2 vCPU / 8 GB RAM / 100 GB NVMe,
    folga suficiente para 3 k usuários, app Go + Postgres + Caddy no mesmo
    host com headroom para picos e migrations.
  - Docker Compose dá imutabilidade, rollback por tag, paridade local/prod e
    isolamento sem operação de cluster.
  - Caddy fornece TLS Let's Encrypt automático, redirect 80→443 e HSTS sem
    arquivo extra de certificado — elimina classe inteira de erro
    operacional.
  - Postgres no mesmo host evita custo de DB gerenciado no MVP; backup
    lógico diário + snapshot semanal mitigam o trade-off de durabilidade.
  - hPanel já entrega firewall gerenciado, malware scanner, snapshots e
    métricas básicas; soma-se UFW + fail2ban + `unattended-upgrades` no SO
    para hardening em profundidade.

## Fontes Consultadas

> Datas de acesso: 2026-06-04. Fontes do prompt enriquecido validadas + 1
> adicional para `unattended-upgrades`.

| Fonte | Sustenta |
|---|---|
| Hostinger – VPS Dashboard ([link](https://www.hostinger.com/support/5726606-how-to-use-the-vps-dashboard-in-hostinger/)) | Capacidades do hPanel: SSH keys, firewall, malware scanner, métricas, backup/monitor. |
| Hostinger – Docker VPS Template ([link](https://www.hostinger.com/support/8306612-how-to-use-the-docker-vps-template-at-hostinger/)) | Template Ubuntu 24.04 com Docker Engine + Compose pré-instalados. |
| Hostinger – Backups/Snapshots ([link](https://www.hostinger.com/support/1583232-how-to-back-up-or-restore-a-vps-at-hostinger/)) | Backups semanais por padrão; opção diária paga; snapshot manual único. |
| Docker – Compose em produção ([link](https://docs.docker.com/compose/how-tos/production/)) | Padrão de override `docker-compose.prod.yml`, sem bind mounts de código, restart policy. |
| Docker – Build best practices ([link](https://docs.docker.com/build/building/best-practices/)) | Multi-stage, usuário não-root, imagens slim. |
| Docker – Rootless mode ([link](https://docs.docker.com/engine/security/rootless/)) | Trade-offs do daemon rootless; recomendado quando viável. |
| Ubuntu Server – UFW ([link](https://documentation.ubuntu.com/server/how-to/security/firewalls/)) | UFW como frontend padrão. |
| Ubuntu Server – Automatic Updates ([link](https://documentation.ubuntu.com/server/how-to/software/automatic-updates/)) | `unattended-upgrades` para patches de segurança. |
| Ubuntu Server – OpenSSH ([link](https://documentation.ubuntu.com/server/how-to/security/openssh-server/)) | Hardening SSH (PermitRootLogin no, PasswordAuthentication no). |
| Caddy – Automatic HTTPS ([link](https://caddyserver.com/docs/automatic-https)) | TLS Let's Encrypt automático + renovação + redirect 80→443. |
| OWASP – Docker Security Cheat Sheet ([link](https://cheatsheetseries.owasp.org/cheatsheets/Docker_Security_Cheat_Sheet.html)) | `--cap-drop`, `no-new-privileges`, FS read-only, limites de recurso. |
| CIS Ubuntu 24.04 Benchmark v1.0.0 (referência geral, paywall) | Baseline conservador para SSH/auditd; usado como direção, não cópia literal. |

> Suposição: dimensionamento KVM 2 baseia-se em 3k usuários ativos com carga
> majoritariamente síncrona REST + worker leve. Para >10k usuários ou worker
> pesado, ver seção **Quando escalar**.

## Decisões Arquiteturais

### SO

- **Ubuntu Server 24.04 LTS** (Noble Numbat). Suporte padrão até 2029, ESM
  até 2034. Template oficial Hostinger com Docker pré-instalado existe.
- Evitar 26.04 no MVP: muito recente em 2026-06; aguardar 2 ciclos de
  point-releases antes de adotar em produção.

### Containerização

- **Sim, Docker Engine + Docker Compose v2** (não Swarm, não K8s).
- Rationale: paridade local/prod com o mesmo `Dockerfile` já existente,
  imagens imutáveis, rollback por retag, isolamento de processos sem custo
  operacional de orquestrador.
- **Rootless mode**: recomendado apenas se Caddy não precisar de portas <1024
  expostas via socket privilegiado. Como Caddy precisa bind em 80/443, o
  caminho mais simples é manter daemon rootful + containers com
  `--user 65532:65532`, `--read-only`, `--cap-drop=ALL`, `no-new-privileges`,
  conforme OWASP. Rootless fica como **evolução futura**.
- Imagens: continuar com `distroless/static-debian12:nonroot` (já no
  Dockerfile); pinning por **digest SHA256** em prod (não apenas tag).

### Reverse Proxy e TLS

- **Caddy 2**. Justificativa:
  - HTTPS automático via Let's Encrypt sem cron de renovação.
  - Caddyfile mínimo (5–10 linhas) cobre vhost, redirect, HSTS, compressão.
  - Imagem oficial `caddy:2-alpine` pequena.
  - Nginx exige `certbot`/`acme.sh` adicional; Traefik é mais poderoso, mas
    seu modelo de descoberta dinâmica é overhead para 1 serviço.
- Headers obrigatórios no MVP: `Strict-Transport-Security`,
  `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`,
  `Permissions-Policy` restritivo, `Content-Security-Policy` adequado à API
  (mais permissivo se houver dashboard estático).

### Banco de Dados

- **PostgreSQL 16 containerizado no mesmo host** para MVP. Volume nomeado
  Docker (`postgres-data`) em `/var/lib/docker/volumes/...` no NVMe.
- Não expor porta 5432 fora da rede Docker interna; conectar via DNS interno
  do Compose (`postgres:5432`).
- `shared_buffers`, `work_mem`, `effective_cache_size` ajustados para 8 GB
  (perfil `mixed` do pgtune; suposição: validar com carga real).
- Limite claro para migrar para gerenciado: **qualquer um** dos gatilhos:
  - DB > 20 GB ou crescimento > 2 GB/mês sustentado.
  - RPO exigido < 24h (`pg_dump` diário deixa de servir).
  - Picos de IO saturando o NVMe compartilhado com o app.
  - Necessidade de réplica de leitura ou PITR fino.
- Alvo de migração quando os gatilhos dispararem: **Neon free/launch** ou
  **Supabase free** (Postgres gerenciado com PITR nativo), antes de RDS.

### Ambientes Local/Staging/Produção

- **Local**: `docker compose -f compose.yml -f compose.local.yml up` com
  bind mount de migrations e seeds, Postgres em porta exposta apenas em
  `127.0.0.1`, `GOFLAGS=-race` em runs de teste, fixtures via task `make
  seed`.
- **Staging**: **não criar no MVP**. Justificativa econômica: 3k usuários
  não justifica 2× VPS. Mitigação: feature flags + canary manual no único
  host (deploy em horário de baixa, monitorar 30 min antes de declarar OK).
- **Produção**: VPS Hostinger única, deploy disparado manualmente via SSH
  (ou GitHub Actions com OIDC + chave restrita) executando `docker compose
  pull && up -d` com tag versionada.
- Reabrir decisão de staging quando: >10k usuários, time >2 devs, ou
  primeiro incidente de regressão silenciosa.

## Arquitetura Recomendada

### Diagrama textual

```text
                  ┌─────────────────────────────────────────────────────┐
                  │        VPS Hostinger KVM 2 (Ubuntu 24.04 LTS)       │
                  │                                                     │
  Internet ──TLS──▶│  Caddy 2 :80/:443  ──http──▶ mecontrola-server:8080│
                  │                                ▲                   │
                  │                                │ pgx/sql            │
                  │                       ┌────────┴────────┐           │
                  │                       │ postgres:16     │ vol:      │
                  │                       │ (rede interna)  │ postgres- │
                  │                       └────────┬────────┘ data      │
                  │                                ▲                    │
                  │                       mecontrola-worker             │
                  │                                                     │
                  │  Promtail ─▶ Grafana Cloud Loki (free)              │
                  │  node_exporter ─▶ Grafana Cloud Prom (free)         │
                  └─────────────────────────────────────────────────────┘
                              │                       │
                              ▼                       ▼
                         hPanel snapshots     pg_dump diário → B2/R2
                         (semanal/manual)     (offsite, 30 dias)
```

### Fluxo de request

1. Cliente → DNS A record → IP da VPS.
2. Caddy termina TLS, aplica HSTS, encaminha por rede Docker interna para
   `mecontrola-server:8080`.
3. Server fala com Postgres via DNS interno do Compose. Outbox/worker
   processa side-effects assíncronos.
4. Logs estruturados (slog JSON) em stdout → Docker logging driver
   `json-file` (rotação 10 MB × 5) → Promtail → Loki.

### Fluxo de deploy

1. CI builda imagem e publica em `ghcr.io/<org>/mecontrola:<git-sha>` (CI
   gratuito).
2. Em produção, atualizar tag no `compose.prod.yml` (ou `.env`).
3. SSH na VPS, executar `docker compose pull && docker compose run --rm
   migrate up && docker compose up -d --no-deps server worker`.
4. Health check `/healthz` poll por 60s; em falha, retag para versão
   anterior + `up -d`.

### Fluxo de backup/restore

- **Snapshots hPanel**: semanais automáticos + 1 manual antes de cada deploy
  com migração schema-breaking.
- **`pg_dump` diário**: cron 03:00 UTC dentro de container utilitário,
  comprime, envia a B2/R2 com retenção 30 dias, criptografado com `age` e
  chave armazenada fora do servidor.
- **Restore drill mensal**: baixar dump mais recente em VM descartável,
  restaurar, rodar `migrate up`, smoke test, registrar tempo em
  `deployment/runbooks/restore-pitr.md` (já existe).

## Baseline de Segurança

| Item | Regra inegociável MVP |
|---|---|
| SSH | `PermitRootLogin no`, `PasswordAuthentication no`, `KbdInteractiveAuthentication no`, chave ed25519, porta padrão (mover só se monitorar logs). |
| Firewall | UFW: default `deny incoming`, permitir 22/tcp, 80/tcp, 443/tcp; ICMP echo opcional. Firewall do hPanel duplica camada. |
| Containers | `--cap-drop=ALL`, capabilities adicionadas só sob necessidade; `no-new-privileges`; `--read-only` + tmpfs em `/tmp`; `user: "65532:65532"`. |
| Docker daemon | Socket NUNCA exposto via TCP; sem mounts de `/var/run/docker.sock` em containers de app. |
| Secrets | `.env` no host com `chmod 600`, dono `root`, lido apenas pelo Compose; nunca commitar; rotação trimestral mínima documentada em `rotate-secret.md` (já existe). Evoluir para `sops` + age quando >5 segredos. |
| TLS | Caddy gerencia LE; HSTS `max-age=31536000; includeSubDomains; preload`. Apenas TLS 1.2+. |
| Banco | Acessível só na rede interna Docker; senha do `postgres` superuser distinta do role da app; role da app só com `CONNECT, USAGE, SELECT, INSERT, UPDATE, DELETE` no schema da aplicação. |
| Atualizações | `unattended-upgrades` ativo apenas para `${distro_id}:${distro_codename}-security`; reboot agendado domingo 04:00 UTC se necessário. Imagens base do app recompiladas semanalmente. |
| Permissões | Usuário operacional `deploy` em grupo `docker`; root SSH desabilitado; `sudo` com auditoria via `auditd`. |
| Intrusão | `fail2ban` com jail `sshd` (5 falhas em 10 min → ban 1 h); logs de Caddy alimentam jail HTTP opcional. |
| Scan de imagem | `trivy fs` no Dockerfile no CI (gratuito); falhar build em CVE HIGH/CRITICAL conhecidos. |

## Estratégia Local-First

### Validação de cada feature localmente

1. `task lint:run` → `task test:unit` → `task security:vulncheck` (já no
   `Taskfile.yml`).
2. `docker compose -f compose.yml -f compose.local.yml up -d` levanta
   Postgres + server + worker idênticos à produção.
3. `task migrate:up` aplica migrations no DB local.
4. `task test:integration` roda testes que exigem Postgres real (regra de
   memória do projeto: não mockar DB).
5. Smoke manual via Postman/Bruno (coleção em `docs/postman/`, gerar com
   skill `postman-collection-generator` quando faltar).
6. Para features com side-effect/outbox: verificar idempotência por
   `event_id` (contrato obrigatório em `AGENTS.md`).

### Paridade local/produção

- Mesmo `Dockerfile`, mesma versão do Go (1.26), mesma major do Postgres,
  mesmo Caddy.
- Diferenças isoladas em `compose.local.yml` (override apenas para porta
  exposta de DB, bind mounts de migrations, `GOFLAGS=-race`, logs em texto).
- Variáveis de ambiente em `.env.example` versionado + `.env.local` privado.

### Checks antes de merge/deploy

- CI obrigatório verde: lint, test (race), `govulncheck`, build de imagem,
  `trivy` scan.
- Pre-commit: gofmt, gitsign (commits assinados — já configurado no Taskfile
  `setup`).
- PR review com a skill `review` quando diff afetar segurança ou DB.

## Operação em Produção

### Deploy

- Disparo manual ou via GitHub Actions com `workflow_dispatch`.
- Imagem por digest, não por tag mutável; tag semântica `vMAJOR.MINOR.PATCH`
  apontando para o digest.
- Migrações executadas como step antes de `up -d`; em caso de falha de
  migração, deploy aborta e reaplica imagem anterior.

### Rollback

- Tag anterior sempre presente no registry por ≥30 dias.
- Procedimento documentado em `deployment/runbooks/rollback.md` (já existe).
- Para migrações schema-breaking: usar padrão **expand/contract** em 2
  deploys (R7 de Go SKILL + práticas comuns) para tornar o rollback de
  código independente do schema.

### Logs

- App escreve JSON via `log/slog` em stdout (R7.2 das regras Go).
- Docker `json-file` com `max-size=10m`, `max-file=5`.
- Promtail (container leve) envia para **Grafana Cloud Loki free** (50 GB
  retenção 14 dias gratuito em 2026 — confirmar limite atual no onboarding).

### Métricas

- `node_exporter` no host → Grafana Cloud Prometheus.
- Métricas da aplicação via OTel (devkit-go já provê) exportando para
  Grafana Cloud OTLP.
- Dashboard base reaproveitando `deployment/grafana/mecontrola-platform.json`.

### Alertas

- Alertas mínimos no Grafana Cloud (free):
  - Disco > 80% (warning) / > 90% (page).
  - RAM > 85% por 10 min.
  - CPU > 80% por 15 min.
  - HTTP 5xx rate > 2% janela 5 min.
  - Healthcheck `/healthz` falho 3× consecutivos.
  - Backup `pg_dump` ausente nas últimas 26 h.
- Notificação: Telegram bot ou email (zero custo).
- Uptime externo: UptimeRobot free (50 monitores, 5 min de intervalo).

### Backup

- Hostinger: snapshot manual antes de cada deploy não-trivial;
  backup semanal automático mantido.
- `pg_dump` diário criptografado + offsite B2/R2.
- Backup do volume Caddy (certificados) incluído nos snapshots — perda
  significa apenas re-emissão LE.

### Restore drill

- Mensal, em VM descartável. Tempo alvo: RTO < 2 h, RPO ≤ 24 h no MVP.
- Registrar resultado em `deployment/runbooks/restore-pitr.md`.

### Rotina semanal/mensal

- **Semanal**: revisar alertas disparados, rotação de logs OK, `apt list
  --upgradable`, `docker system df`, `trivy image` no `latest` em produção.
- **Mensal**: restore drill, revisão de segredos próximos da rotação,
  revisão de capacidade (CPU/RAM/IO p95).
- **Trimestral**: revisão de retenção de backups, revisão de regras de
  firewall, atualização de imagem base do Dockerfile (Go LTS, distroless).

## Custos e Trade-offs

### Escolha mínima recomendada

| Item | Provedor | Custo aprox. mensal (USD, 2026-06) | Notas |
|---|---|---|---|
| VPS KVM 2 | Hostinger | ~7.99 (anual) a ~11.99 (mensal) | 2 vCPU/8 GB/100 GB NVMe — caber app+DB+Caddy. |
| Domínio | Registrar à escolha | ~1–1.5 | .com.br ~ R$ 40/ano. |
| Registry imagens | GHCR | 0 | Repo público ou private com plano free do GitHub. |
| CI | GitHub Actions | 0 | Free tier de minutos suficiente p/ 3k usuários. |
| Observabilidade | Grafana Cloud free | 0 | Logs 50GB/14d, Métricas 10k series, Traces 50GB. |
| Uptime externo | UptimeRobot free | 0 | 50 monitores, 5 min. |
| Backup offsite | Backblaze B2 free | 0 | 10 GB grátis; suficiente p/ dumps comprimidos curtos. |
| **Total estimado** | | **≈ US$ 9–13/mês** | |

> Suposição de preço: valores Hostinger variam por promoção/contrato anual;
> validar no checkout em 2026-06-04.

### Quando escalar

- **Vertical primeiro** (KVM 4 → KVM 8): mais barato e operacionalmente
  trivial até ~10–20k usuários ativos.
- **Separar Postgres** quando algum dos gatilhos da seção "Banco" disparar.
- **Adicionar staging** quando o time crescer ou após primeiro incidente
  de regressão silenciosa em prod.
- **Trocar VPS por PaaS (Fly.io/Render)** apenas se exigência de
  multi-região aparecer (já há `deployment/fly/fly.toml` como possível
  caminho).

### O que evitar no MVP

- Kubernetes (k3s/k0s incluído) — overhead operacional não amortizável.
- Service mesh, API gateway dedicado.
- Múltiplos nós + load balancer.
- DB gerenciado caro (RDS/Cloud SQL) antes dos gatilhos.
- Auto-scaling — não há padrão de carga que justifique.

## Checklist Production-Ready

### Bloqueantes (não publicar sem)

- [ ] VPS provisionada Ubuntu 24.04, SSH só com chave, root login desabilitado.
- [ ] UFW ativo: `default deny in`, permitir 22/80/443 apenas.
- [ ] `unattended-upgrades` ativo só para security pocket.
- [ ] `fail2ban` com jail `sshd`.
- [ ] Docker Engine + Compose v2 em versão suportada.
- [ ] Containers com `--cap-drop=ALL`, `no-new-privileges`, `read-only`, user
      `65532`.
- [ ] Caddy com HTTPS automático funcionando e HSTS.
- [ ] Postgres não exposto fora da rede Docker interna; role da app sem
      privilégios de super.
- [ ] Migrações automáticas no deploy com abort-on-failure.
- [ ] Healthcheck `/healthz` configurado no Compose e monitorado externo.
- [ ] Logs JSON estruturados saindo via stdout, rotação configurada.
- [ ] `pg_dump` diário criptografado em offsite, retenção 30 dias.
- [ ] Snapshot hPanel semanal ativo.
- [ ] Restore drill executado pelo menos 1 vez antes de publicar.
- [ ] Segredos fora do Git, `.env` com `chmod 600`.
- [ ] Alertas mínimos (5xx, disco, RAM, healthcheck, backup ausente) ativos.

### Recomendados

- [ ] `trivy` scan no CI bloqueando HIGH/CRITICAL.
- [ ] `govulncheck` no CI (já há `task security:vulncheck`).
- [ ] Imagens pinadas por digest no Compose.
- [ ] Auditd ativo no host.
- [ ] Dashboards Grafana baseados em `mecontrola-platform.json` publicados.
- [ ] Status page público simples (Uptime Kuma ou similar) — opcional.
- [ ] Página `/security.txt` e referência a `SECURITY.md`.

### Futuros (pós-MVP)

- [ ] Docker rootless mode.
- [ ] `sops`/age para gestão de segredos.
- [ ] Postgres gerenciado (Neon/Supabase) quando gatilhos dispararem.
- [ ] Staging dedicado.
- [ ] WAF (Cloudflare free na frente) se aparecerem padrões de abuso.
- [ ] CDN para assets estáticos.
- [ ] PITR contínuo (logical replication ou DB gerenciado).

## Plano de Implementação Futuro

> Sequência ordenada do que precisa ser criado quando for autorizado a
> implementar. **Nada disso deve ser feito agora.**

### Etapa 1 — Provisionamento da VPS (1 sessão)
- Comprar plano KVM 2 com OS template **Ubuntu 24.04** (não o template
  "Docker" para manter controle do hardening).
- Subir chave SSH no hPanel; conectar; criar usuário `deploy`; desabilitar
  root login; instalar atualizações.
- **Validação**: `ssh deploy@vps` funciona com chave; `ssh root@vps` falha.

### Etapa 2 — Hardening base do SO
- Instalar `ufw`, `fail2ban`, `unattended-upgrades`, `auditd`,
  `docker.io` (ou Docker oficial), `docker-compose-plugin`.
- Configurar UFW, fail2ban, unattended-upgrades conforme baseline.
- **Validação**: `ufw status verbose` mostra regras; `fail2ban-client status
  sshd`; logs `auth.log` registram tentativas bloqueadas; `apt list
  --upgradable` vazio.

### Etapa 3 — Estrutura de deploy (arquivos a criar fora do repo OU em
  `deployment/`)
- Arquivos prováveis a serem **propostos** (não criar agora):
  - `deployment/compose/compose.yml` — serviços `server`, `worker`,
    `postgres`, `caddy`, `promtail`.
  - `deployment/compose/compose.local.yml` — overrides locais.
  - `deployment/compose/compose.prod.yml` — overrides de produção
    (restart policy, limites de recurso, sem bind mount).
  - `deployment/caddy/Caddyfile` — vhost + automatic HTTPS.
  - `deployment/scripts/pg-dump.sh` — dump + upload B2/R2.
  - `deployment/scripts/deploy.sh` — wrapper SSH idempotente.
  - `.env.example` na raiz.
- **Validação**: `docker compose -f deployment/compose/compose.yml -f
  deployment/compose/compose.local.yml config` valida sem erro.

### Etapa 4 — Local-first
- Documentar `task` targets: `task local:up`, `task local:down`,
  `task local:seed`.
- Adicionar `deployment/postman/` com coleção via skill
  `postman-collection-generator`.
- **Validação**: nova pessoa clona o repo, roda `task local:up` e tem app
  respondendo em <10 min.

### Etapa 5 — CI/CD
- GitHub Actions workflow: lint → test → vulncheck → trivy → build
  multi-arch → push GHCR → tag por SHA.
- Workflow manual de deploy (`workflow_dispatch`) com input `image_tag`,
  conectando via OIDC + chave dedicada.
- **Validação**: PR azul executa pipeline completa < 8 min; deploy manual
  promove imagem em <2 min.

### Etapa 6 — Observabilidade
- Conta Grafana Cloud free.
- Promtail container + scrape config para `docker_sd`.
- node_exporter como systemd unit.
- Dashboards a partir de `deployment/grafana/mecontrola-platform.json`.
- Alertas configurados conforme seção **Alertas**.
- **Validação**: provocar 5xx em ambiente local apontando pro Grafana
  Cloud; alerta dispara em <5 min.

### Etapa 7 — Backup e Restore drill
- Configurar `pg-dump.sh` em cron host (não no container de app).
- Configurar bucket B2/R2 + chave dedicada com permissão apenas-write.
- Executar primeiro restore drill em VM descartável; cronometrar.
- **Validação**: dump existe, restaura, app sobe, smoke OK; tempo total
  < 2 h.

### Etapa 8 — Go-live
- Apontar DNS para a VPS.
- Esperar emissão LE; verificar HSTS e cadeia.
- Smoke test externo (Postman + UptimeRobot).
- Anúncio interno + janela de monitoramento 24 h.

### Riscos e critérios de parada

| Risco | Mitigação | Critério de parada |
|---|---|---|
| Único host = SPOF | hPanel snapshot + dump offsite + runbook claro | Indisponibilidade > 4 h sem causa raiz identificada → abrir frente de HA. |
| Postgres no mesmo host satura IO | Métricas IO, alerta CPU iowait | iowait > 20% sustentado → mover DB. |
| LE rate limit | Caddy reusa cert; staging endpoint para testes | Falha de emissão repetida → fallback manual com cert via DNS-01. |
| Dump diário falha silenciosamente | Alerta "backup ausente >26h" | Falha 2 dias seguidos → parar deploys até resolver. |
| Vazamento de segredo | Rotação trimestral, gitsign no histórico | Detecção via `gitleaks` no CI → rotacionar tudo em 24 h. |

## Perguntas em Aberto

> Apenas o que realmente bloqueia decisão segura. Demais escolhas estão
> tomadas acima.

1. **Domínio registrado?** Já existe domínio do `mecontrola` para apontar
   o DNS (necessário para emissão Let's Encrypt)? Se não, escolher
   registrar antes da Etapa 1.
2. **Conta GHCR/registry**: usar GHCR no namespace pessoal do dono do repo
   ou criar org? Afeta visibilidade da imagem e quem pode fazer pull no
   deploy.
3. **Idioma de notificação**: Telegram, email ou Discord para alertas?
   Apenas para fechar onde criar o webhook do Grafana Cloud.
4. **Plano Hostinger anual ou mensal?** Anual desbloqueia preço promocional
   listado no site (~US$ 4–7/mês com 24-36 meses pagos antecipadamente);
   mensal sobe para ~US$ 11–13. Decisão de compra do usuário.
5. **Volume previsto de dados em 6 meses**: 3k usuários é claro, mas qual o
   volume médio por usuário (linhas/mês)? Afeta validação dos gatilhos de
   migração de DB.

---

## Verificação do Plano

- **Conformidade com o prompt**: cobre todas as 12 perguntas obrigatórias, o
  formato Markdown exigido, restrições (não implementar, sem K8s, sem
  exposição de DB/Docker), cita fontes 2026 e separa local/prod.
- **Conformidade com AGENTS.md/CLAUDE.md**: respeita arquitetura existente,
  reusa `Dockerfile`, runbooks e dashboards já presentes; não inventa
  módulos nem wiring.
- **Aceite do usuário**: para considerar este plano concluído, basta
  aprovação do conteúdo via ExitPlanMode.
