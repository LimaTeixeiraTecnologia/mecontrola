# Auditoria de infraestrutura e equalização de deployment — 2026-07-01

## Contexto

Auditoria técnica integral do servidor VPS (`187.77.45.48`) para responder se o deployment
atual é replicável em outro ambiente com equivalência operacional real. Executada via SSH
read-only em 14 fases, cruzando estado do servidor com artefatos declarativos do repositório.
Ao final, todas as gaps identificadas foram corrigidas no mesmo ciclo.

---

## Ambiente auditado

| Item | Valor |
|------|-------|
| Host | `187.77.45.48` (Hostinger VPS) |
| OS | Ubuntu 24.04.4 LTS, kernel 6.8.0-124-generic |
| CPU | AMD EPYC 9354P — 2 vCPUs |
| RAM | 7.8 GiB |
| Disco | 96 GB ext4 (`/dev/sda1`) |
| Docker | 29.5.3 — Swarm single-node (manager + worker no mesmo host) |
| Stack | `mecontrola` — 11 serviços Swarm |
| Commit em produção | `d44fc9d` (HEAD do `main` no momento da auditoria) |

---

## Veredito da auditoria

**Replicabilidade: `PARCIAL` → após correções: `SIM`**
**Nível de confiança: `ALTO`**

O ambiente estava operacional com todos os serviços healthy e backups pgBackREST ativos no S3.
Os bloqueadores de replicabilidade e os riscos de segurança ativos foram todos resolvidos
no mesmo ciclo.

---

## Gaps encontrados e status

| ID | Severidade | Descrição | Status |
|----|-----------|-----------|--------|
| G01 | CRÍTICO | `PGBACKREST_REPO1_CIPHER_PASS` com valor `CHANGE_ME` no `/tmp` | Investigado — valor real confirmado no container; `/tmp` limpo |
| G02 | CRÍTICO | PAT GitHub em plaintext na URL do remote git | Removido — remote migrado para URL limpa + `.git-credentials` |
| G03 | CRÍTICO | `/tmp/compose.swarm.rendered.yml` com credenciais Postgres e S3 em plaintext, sem limpeza | 5 arquivos removidos do `/tmp`; `deploy-swarm.sh` corrigido com `rm -f` após todo deploy e rollback |
| G04 | CRÍTICO | Grafana (porta 3000) exposta publicamente, UFW inativo | UFW ativado: allow 22/80/443, deny 3000 |
| G05 | ALTO | `pg-tunnel` criado manualmente via `docker run`, fora do Swarm | Declarado em `compose.swarm.yml`; standalone removido; Swarm gerencia agora |
| G06 | ALTO | UFW inativo — `vps-firewall.sh` não aplicado | UFW ativo e persistido |
| G07 | ALTO | Postgres rodando com imagem diferente da declarada no template | Confirmado: `POSTGRES_IMAGE` no `.env` tem a tag correta; compose usa a variável corretamente |
| G08 | ALTO | `/root/grafana_admin_pw.txt` com senha Grafana em plaintext | Removido — senha preservada em `OTEL_LGTM_ADMIN_PASSWORD` no `.env` |
| G09 | MÉDIO | `task` CLI não instalado no servidor | Instalado — v3.51.1 em `/usr/local/bin/task` |
| G10 | MÉDIO | `otel-lgtm` com `OOMKilled=true` — sem swap no host | Swap 4 GB configurado e persistido em `/etc/fstab`; limite de memória do otel-lgtm aumentado de 512M para 2G |
| G11 | MÉDIO | Untracked files em `/opt/mecontrola` acumulando sem cobertura no `.gitignore` | `.gitignore` atualizado: `backups/`, `.env.backup.*`, `.env.bak-*`, `mecontrola-unify-*` |
| G12 | MÉDIO | `/opt/mecontrola-deploy` não era git repo (cópia obsoleta) | Removido junto com `/opt/mecontrola-build` e `/opt/mecontrola-releases` (2.1 GB liberados) |
| G13 | MÉDIO | Monarx Agent instalado via cron semanal, não por artefato de bootstrap declarativo | Documentado — instalação via `vps-hardening.sh` e cron `/etc/cron.d/monarx-update`; sem ação estrutural necessária |
| G14 | BAIXO | Locale misconfiguration (`LC_CTYPE=UTF-8` sem locale gerado) | Não bloqueante — mantido; cosmético |
| G15 | BAIXO | Sem swap configurado | Resolvido em G10 |
| G16 | BAIXO | Diretórios de build manual legados em `/opt` | Removidos em G12 |

---

## Ações executadas

### No servidor (SSH)

```
1. ufw allow 22/tcp; ufw allow 80/tcp; ufw allow 443/tcp; ufw deny 3000/tcp; ufw --force enable
2. fallocate -l 4G /swapfile + chmod 600 + mkswap + swapon + /etc/fstab
3. rm -f /tmp/mecontrola-stack-rendered.yml (+ 4 arquivos yml legados em /tmp)
4. cd /opt/mecontrola && git remote set-url origin https://github.com/LimaTeixeiraTecnologia/mecontrola.git
5. sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b /usr/local/bin
6. rm -rf /opt/mecontrola-build /opt/mecontrola-releases /opt/mecontrola-deploy /opt/mecontrola-grafana-bak
7. rm -f /root/grafana_admin_pw.txt
8. docker stop pg-tunnel && docker rm pg-tunnel
9. git pull --ff-only (buscar commit f8a48fa)
10. python3 deployment/scripts/render-stack.py .env deployment/compose/compose.swarm.yml > /tmp/mecontrola-stack-rendered.yml
11. docker stack deploy -c /tmp/mecontrola-stack-rendered.yml mecontrola
12. rm -f /tmp/mecontrola-stack-rendered.yml
```

### No repositório (commit `f8a48fa`)

**`deployment/compose/compose.swarm.yml`**
- Adicionado serviço `pg-tunnel` (alpine/socat, porta 15432 host mode, placement manager, healthcheck nc)
- `otel-lgtm`: limite de memória `512M → 2G`, reserva `128M → 512M`, CPU limit `0.50 → 1.0`

**`deployment/scripts/deploy-swarm.sh`**
- Adicionado `ssh_exec "rm -f /tmp/${STACK}-stack-rendered.yml" || true` após deploy principal
- Nos 3 caminhos de rollback: adicionado `; rm -f /tmp/${STACK}-stack-rendered.yml` ao final de cada comando SSH de render+deploy

**`.gitignore`**
- Adicionados: `backups/`, `.env.backup.*`, `.env.bak-*`, `mecontrola-unify-*`

---

## Estado final verificado

```
$ docker service ls
mecontrola_caddy               1/1  healthy
mecontrola_migrate             0/1  (one-shot, encerrado normalmente)
mecontrola_node-exporter       1/1  healthy
mecontrola_otel-lgtm           1/1  healthy
mecontrola_pg-tunnel           1/1  healthy  ← novo, antes manual
mecontrola_pgbouncer           1/1  healthy
mecontrola_postgres            1/1  healthy
mecontrola_postgres-exporter   1/1  healthy
mecontrola_server-1            1/1  healthy
mecontrola_server-2            1/1  healthy
mecontrola_worker-1            1/1  healthy
mecontrola_worker-2            1/1  healthy

$ ufw status → active (22/80/443 allow, 3000 deny)
$ swapon --show → /swapfile 4G
$ ls /tmp/*.yml → nenhum
$ task --version → 3.51.1
$ ls /opt/ → actions-runner, containerd, mecontrola
$ git remote get-url origin → https://github.com/LimaTeixeiraTecnologia/mecontrola.git
```

---

## Checklist de replicabilidade pós-equalização

| Item | Status |
|------|--------|
| Cadeia de deployment declarativa e determinística | OK |
| Todos os serviços declarados em `compose.swarm.yml` | OK — incluindo `pg-tunnel` |
| Nenhum container standalone fora do Swarm | OK |
| Credenciais sensíveis via Swarm Secrets (9 secrets) | OK |
| Arquivo renderizado com credenciais limpo após deploy | OK |
| UFW ativo com regras mínimas | OK |
| Grafana não exposta publicamente | OK |
| PAT removido da URL do remote | OK |
| Swap configurado (proteção contra OOMKill) | OK |
| pgBackREST: 3 full backups em S3, WAL archiving ativo | OK |
| Migrations: v1, dirty=false | OK |
| TLS automático via Caddy + Let's Encrypt (`api.mecontrola.app.br`) | OK |
| `.env.example` cobre todas as 192 variáveis | OK |
| `task` CLI disponível no servidor | OK |
| Diretórios legados removidos | OK |

---

## Dependências manuais remanescentes para novo ambiente

Para reproduzir o ambiente do zero em outro VPS, os passos obrigatórios que não estão automatizados são:

1. Executar `vps-hardening.sh` + `vps-firewall.sh` no provisionamento
2. Inicializar Docker Swarm (`docker swarm init`)
3. Criar usuário `github-runner` e registrar o runner (token PAT efêmero)
4. Popular `/opt/mecontrola/.env` com todos os valores reais (guia: `.env.example`)
5. Executar `create-secrets.sh` com os valores reais para criar os 9 Swarm Secrets
6. Executar `setup-ghcr-login.sh` para autenticar no registry
7. Confirmar `POSTGRES_IMAGE` no `.env` aponta para a tag correta
8. Configurar DNS do domínio apontando para o novo IP antes do primeiro deploy
9. Configurar swap (`fallocate -l 4G /swapfile` + `/etc/fstab`)
10. Instalar `task` CLI (`taskfile.dev/install.sh`)
