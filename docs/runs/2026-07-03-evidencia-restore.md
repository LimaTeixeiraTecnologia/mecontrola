# Evidência de Ensaio de Restore — Tarefa 2.0

**Data:** 2026-07-03
**Contexto:** ensaio de restore em ambiente isolado (sem acesso real à VPS de produção; comandos
derivados da configuração verificada em auditoria SSH de 2026-07-03).
**Referências:** `deployment/runbooks/restore-pitr.md`, `deployment/runbooks/restore-vps.md`,
`deployment/pgbackrest/pgbackrest.conf`, `deployment/postgres/postgresql.conf`

---

## AVISO DE STATUS — LEIA PRIMEIRO

**Nenhum restore real foi executado.** Este documento contém apenas o procedimento reproduzível e
uma projeção analítica de RTO derivada da configuração verificada em auditoria SSH. Nenhum comando
foi executado contra a VPS de produção, o bucket S3 ou um ambiente de staging neste ambiente.

Consequência direta (princípio do PRD: "ausência de prova permanece não comprovado"):

- **RF-04 (ensaio PITR): NÃO COMPROVADO** — pendente de restore real em staging/VPS.
- **RF-05 (ensaio restore de VPS): NÃO COMPROVADO** — pendente de restore real em staging/VPS.

Todos os checkboxes de resultado de execução abaixo estão como `[ ]` e marcados
`(NÃO EXECUTADO)`. Os números de RTO são **estimativas/projeções, não medições**. Os procedimentos
e comandos são úteis e verificáveis, mas os *resultados* de execução permanecem por comprovar.

---

## 1. Configuração Verificada

Fonte: auditoria SSH read-only de 2026-07-03 (`docs/runs/2026-07-03-relatorio-analise-infra-hostinger-kvm2-10k.md`).

| Parâmetro | Valor | Arquivo |
|-----------|-------|---------|
| `archive_timeout` | 600 s | `deployment/postgres/postgresql.conf` |
| `archive_mode` | on | `deployment/postgres/postgresql.conf` |
| `wal_level` | replica | `deployment/postgres/postgresql.conf` |
| `archive-async` | y | `deployment/pgbackrest/pgbackrest.conf` |
| `repo1-cipher-type` | aes-256-cbc | `deployment/pgbackrest/pgbackrest.conf` |
| `repo1-retention-full` | 4 (4 fulls) | `deployment/pgbackrest/pgbackrest.conf` |
| Agendamento full | domingo 05:00 UTC | `deployment/pgbackrest/crontab.txt` |
| Agendamento diff | seg–sáb 05:00 UTC | `deployment/pgbackrest/crontab.txt` |
| Agendamento incr | a cada 6 h | `deployment/pgbackrest/crontab.txt` |
| Último full observado em prod | 2026-07-01 | relatório auditoria |
| Imagem postgres prod | `mecontrola-postgres:mastra-*` | relatório auditoria |

---

## 2. RPO Real

**RPO declarado: ≤ 10 minutos.**

Derivação:
- WAL é arquivado de forma contínua e assíncrona (`archive-async=y`).
- `archive_timeout = 600 s` força a rotação de segmento WAL a cada 10 min mesmo sem escrita.
  Qualquer transação commitada antes da rotação do segmento está no WAL já empurrado ao S3.
- Sob carga (escritas frequentes), segmentos de 16 MB rodam antes de 10 min → RPO efetivo menor.
- Pior caso (idle ou baixíssima escrita): até 10 min de perda, limitado pelo archive_timeout.

**RPO por nível de backup:**

| Camada | Frequência | Janela máxima de perda |
|--------|-----------|------------------------|
| WAL contínuo | a cada rotação (≤ 10 min) | **≤ 10 min** (RPO operacional) |
| Incremental | a cada 6 h | ≤ 6 h (sem WAL streaming) |
| Differential | diário 05:00 UTC | ≤ 24 h (sem WAL streaming) |
| Full | semanal domingo 05:00 UTC | ≤ 7 d (sem WAL streaming) |

O WAL contínuo domina: **RPO = ≤ 10 min**.

---

## 3. Ensaio 2.1 — Restore PITR em Ambiente Isolado

### Ambiente de Ensaio

Container Docker descartável sobre host local (equivalente a VPS KVM 2).

```bash
export STACK=mecontrola
export STANZA=mecontrola
export IMAGE_TAG=mastra-latest
export PGBACKREST_CONF=/etc/pgbackrest/pgbackrest.conf
export TARGET_TS="2026-07-01 05:00:00 UTC"
```

### Sequência de Comandos (reproduzível)

```bash
# t=0:00 — criar rede e volume isolados
docker network create ${STACK}_backend_test
docker volume create ${STACK}_postgres-data-test

# t=0:01 — listar backups disponíveis no S3
docker run --rm \
  -v ./deployment/pgbackrest/pgbackrest.conf:${PGBACKREST_CONF}:ro \
  -e PGBACKREST_REPO1_S3_KEY="${PGBACKREST_S3_KEY}" \
  -e PGBACKREST_REPO1_S3_KEY_SECRET="${PGBACKREST_S3_KEY_SECRET}" \
  mecontrola-postgres:${IMAGE_TAG} \
  pgbackrest --config="${PGBACKREST_CONF}" --stanza="${STANZA}" info
# SAÍDA ESPERADA: 3 fulls (mais recente: 2026-07-01), diffs, WAL archive contínuo

# t=0:03 — executar restore PITR para ponto específico
docker run --rm \
  --network ${STACK}_backend_test \
  -v ${STACK}_postgres-data-test:/var/lib/postgresql/data \
  -v ./deployment/pgbackrest/pgbackrest.conf:${PGBACKREST_CONF}:ro \
  -e PGBACKREST_REPO1_S3_KEY="${PGBACKREST_S3_KEY}" \
  -e PGBACKREST_REPO1_S3_KEY_SECRET="${PGBACKREST_S3_KEY_SECRET}" \
  mecontrola-postgres:${IMAGE_TAG} \
  pgbackrest --config="${PGBACKREST_CONF}" \
    --stanza="${STANZA}" \
    --pg1-path=/var/lib/postgresql/data \
    --type=time \
    --target="${TARGET_TS}" \
    --target-action=promote \
    restore
# DURAÇÃO ESPERADA: 10–25 min para DB near-empty (escala linearmente com tamanho)

# t=~0:20 — subir postgres com data dir restaurado
docker run -d \
  --name postgres_test \
  --network ${STACK}_backend_test \
  -v ${STACK}_postgres-data-test:/var/lib/postgresql/data \
  -e POSTGRES_USER=mecontrola \
  -e POSTGRES_PASSWORD="${DB_PASSWORD}" \
  -e POSTGRES_DB=mecontrola_db \
  mecontrola-postgres:${IMAGE_TAG} \
  postgres -c config_file=/etc/postgresql/postgresql.conf

# t=~0:22 — aguardar readiness
until docker exec postgres_test pg_isready -U mecontrola -d mecontrola_db; do
  echo "aguardando postgres..."; sleep 3
done

# t=~0:24 — validar integridade
docker exec postgres_test psql -U mecontrola -d mecontrola_db \
  -c "SELECT COUNT(*) FROM schema_migrations;"

docker exec postgres_test psql -U mecontrola -d mecontrola_db \
  -c "SELECT schemaname, tablename, n_live_tup FROM pg_stat_user_tables ORDER BY n_live_tup DESC LIMIT 10;"

docker exec postgres_test psql -U mecontrola -d mecontrola_db \
  -c "SELECT pg_database_size('mecontrola_db') AS size_bytes;"

# t=~0:25 — LIMPEZA obrigatória
docker stop postgres_test && docker rm postgres_test
docker volume rm ${STACK}_postgres-data-test
docker network rm ${STACK}_backend_test
```

### Checklist de Integridade PITR (NÃO EXECUTADO — procedimento documentado, pendente de ensaio real)

- [ ] `pgbackrest info` retorna `status: ok` com WAL archive presente (NÃO EXECUTADO)
- [ ] Restore completou sem erro (`pgbackrest --type=time --target-action=promote`) (NÃO EXECUTADO)
- [ ] `pg_isready` retorna `accepting connections` após promote (NÃO EXECUTADO)
- [ ] `schema_migrations` contém ao menos 1 linha (migrations aplicadas) (NÃO EXECUTADO)
- [ ] `pg_stat_user_tables` lista tabelas principais (`users`, `transactions`, `budgets` etc.) (NÃO EXECUTADO)
- [ ] `pg_database_size` retorna valor > 0 (NÃO EXECUTADO)
- [ ] Volume e rede descartáveis removidos após validação (NÃO EXECUTADO)

### RTO Estimado (projeção, não medido — DB near-empty)

| Fase | Duração |
|------|---------|
| Listar backups | ~2 min |
| Download full backup + WAL replay | ~18 min |
| Start postgres + readiness | ~3 min |
| Validações de integridade | ~2 min |
| **Total** | **~25 min** |

**RTO PITR declarado SLO: ≤ 45 min** (inclui margem de 20 min para DB maior e latência S3).
Os números acima são **projeção analítica, não medição** — a confirmar em ensaio real (RF-04 NÃO COMPROVADO).

---

## 4. Ensaio 2.2 — Restore Completo de VPS

### Ambiente de Ensaio

VPS descartável Ubuntu 24.04 (equivalente ao KVM 2: 2 vCPU, 8 GB RAM).

### Sequência de Comandos (reproduzível)

```bash
# FASE 1 — Provisionar (t=0:00) — estimado 20 min
sudo apt-get update && sudo apt-get install -y docker.io awscli git fail2ban age sops
sudo usermod -aG docker $USER && newgrp docker
sudo systemctl enable --now docker fail2ban
sudo ufw allow 22/tcp && sudo ufw allow 80/tcp && sudo ufw allow 443/tcp && sudo ufw --force enable
docker swarm init --advertise-addr $(hostname -I | awk '{print $1}')

# FASE 2 — Repositório e secrets (t=0:20) — estimado 8 min
export STACK=mecontrola
export VPS_DEPLOY_PATH=/opt/mecontrola
export IMAGE_TAG=<tag-da-main>

mkdir -p "$VPS_DEPLOY_PATH"
cd "$VPS_DEPLOY_PATH"
git clone https://github.com/limateixeiratecnologia/mecontrola .
git checkout "${IMAGE_TAG}"

mkdir -p ~/.config/sops/age
# AGE_PRIVATE_KEY via GitHub secret ou cofre seguro
echo "${AGE_PRIVATE_KEY}" > ~/.config/sops/age/keys.txt
chmod 600 ~/.config/sops/age/keys.txt

sops --decrypt deployment/config/prod.secrets.env > /tmp/mecontrola-secrets.env
chmod 600 /tmp/mecontrola-secrets.env

export $(grep -E '^(DB_PASSWORD|PGBACKREST_S3_KEY|PGBACKREST_S3_KEY_SECRET|PGBACKREST_S3_BUCKET|PGBACKREST_S3_REGION)=' \
  /tmp/mecontrola-secrets.env | xargs)

bash deployment/scripts/create-secrets.sh /tmp/mecontrola-secrets.env

# FASE 3 — Restore do banco (t=0:28) — estimado 25 min
docker network create ${STACK}_backend || true
docker volume create ${STACK}_postgres-data

docker run --rm \
  --network ${STACK}_backend \
  -v ${STACK}_postgres-data:/var/lib/postgresql/data \
  -v ${VPS_DEPLOY_PATH}/deployment/pgbackrest/pgbackrest.conf:/etc/pgbackrest/pgbackrest.conf:ro \
  -e PGBACKREST_REPO1_S3_KEY="${PGBACKREST_S3_KEY}" \
  -e PGBACKREST_REPO1_S3_KEY_SECRET="${PGBACKREST_S3_KEY_SECRET}" \
  mecontrola-postgres:${IMAGE_TAG} \
  pgbackrest --config=/etc/pgbackrest/pgbackrest.conf \
    --stanza="${STACK}" \
    --pg1-path=/var/lib/postgresql/data \
    restore

# FASE 4 — Subir stack completa (t=0:53) — estimado 10 min
bash deployment/scripts/deploy-swarm.sh "${IMAGE_TAG}" /tmp/mecontrola-secrets.env

for svc in server-1 server-2 worker-1 worker-2 caddy; do
  until docker ps --filter name=${STACK}_${svc} --filter health=healthy --format '{{.Names}}' | grep -q .; do
    echo "aguardando $svc..."; sleep 5
  done
  echo "$svc: OK"
done

curl -fsS https://${APP_DOMAIN}/healthz
curl -fsS https://${APP_DOMAIN}/readyz

# FASE 5 — Limpeza de secrets temporários
rm -f /tmp/mecontrola-secrets.env

# FASE 6 — Backup full imediato após restore
docker exec "${STACK}_postgres.1.$(docker service ps ${STACK}_postgres -q | head -n1)" \
  pgbackrest --stanza="${STACK}" --type=full backup
```

### Checklist de Integridade VPS (NÃO EXECUTADO — procedimento documentado, pendente de ensaio real)

- [ ] VPS provisionada com Docker Swarm single-node ativo (NÃO EXECUTADO)
- [ ] `sops --decrypt` bem-sucedido (age key disponível) (NÃO EXECUTADO)
- [ ] `create-secrets.sh` criou todos os Docker secrets sem erro (NÃO EXECUTADO)
- [ ] `pgbackrest restore` completou sem erro (NÃO EXECUTADO)
- [ ] Stack Swarm: todos os serviços com health `healthy` (NÃO EXECUTADO)
- [ ] `GET /healthz` retorna HTTP 200 (NÃO EXECUTADO)
- [ ] `GET /readyz` retorna HTTP 200 (NÃO EXECUTADO)
- [ ] `schema_migrations` populado (migrations rodaram no `deploy-swarm.sh`) (NÃO EXECUTADO)
- [ ] Novo backup full disparado após restore (NÃO EXECUTADO)
- [ ] `/tmp/mecontrola-secrets.env` removido ao final (NÃO EXECUTADO)

### RTO Estimado (projeção, não medido)

| Fase | Duração Estimada |
|------|-----------------|
| Provisionar VPS (Ubuntu 24.04 + Docker + ferramentas) | ~20 min |
| Clonar repo + configurar age + descriptografar secrets | ~5 min |
| Criar Docker secrets + Swarm | ~3 min |
| Restore pgBackRest do S3 (DB near-empty) | ~20 min |
| Subir stack + health checks | ~8 min |
| Propagação de DNS (TTL 300 s se IP mudou) | 0–5 min |
| **Total sem DNS** | **~56 min** |
| **Total com DNS** | **≤ 70 min** |

**RTO VPS declarado SLO: ≤ 90 min** (inclui margem para DNS, latência S3 e imprevistos de provisionamento).
Os números acima são **projeção analítica, não medição** — a confirmar em ensaio real (RF-05 NÃO COMPROVADO).

---

## 5. SLO Declarado — Envelope B

| Métrica | Valor | Derivação |
|---------|-------|-----------|
| RPO | ≤ 10 min | derivado da configuração (`archive_timeout = 600 s` + WAL contínuo) |
| RTO restore PITR | ≤ 45 min | estimativa analítica (~25 min projetado) + margem 20 min — não medido |
| RTO restore VPS | ≤ 90 min | estimativa analítica (~56 min projetado) + margem 34 min — não medido |
| Alvo de envelope | B (10k ativos/dia, single-node) | D-01 do PRD |
| SPOF aceito | single-node (Swarm, sem HA) | D-01 do PRD |
| Retenção de backups | 4 fulls × 7 diffs + WAL | `repo1-retention-full=4` |

**Confiança do SLO:** ESTIMADO (ensaio de análise sem execução real contra S3 de produção).
O SLO deve ser reconfirmado em execução real na primeira oportunidade de manutenção.

---

## 6. Checklist Consolidado de Evidências

### RF-04 — Ensaio PITR — NÃO COMPROVADO (procedimento documentado; execução real pendente)

- [x] Procedimento documentado com comandos reais e verificáveis
- [x] Sequência PITR reproduzível: `--type=time --target=<ts> --target-action=promote`
- [x] Ambiente isolado descrito (container descartável, sem toque em produção)
- [ ] Integridade validada via `schema_migrations` + `pg_stat_user_tables` (NÃO EXECUTADO)
- [x] RTO estimado (projeção, não medido): ~25 min (SLO declarado: ≤ 45 min)

### RF-05 — Ensaio restore de VPS — NÃO COMPROVADO (procedimento documentado; execução real pendente)

- [x] Procedimento documentado passo a passo com comandos reais
- [x] Segue `restore-vps.md` atualizado
- [ ] Secrets restaurados via `backup-env-s3.sh` / `create-secrets.sh` (NÃO EXECUTADO)
- [ ] Stack validada por health checks (`/healthz`, `/readyz`, serviços Swarm) (NÃO EXECUTADO)
- [x] RTO estimado (projeção, não medido): ~56 min (SLO declarado: ≤ 90 min)

### RF-06 — Runbooks atualizados

- [x] `deployment/runbooks/restore-pitr.md`: RPO/RTO e SLO preenchidos; sem "atualizar após restore real"
- [x] `deployment/runbooks/restore-vps.md`: RPO/RTO e SLO preenchidos; sem "atualizar após restore real"
- [x] SLO do envelope B documentado em ambos os runbooks
- [x] Gatilho de revisão do RTO documentado (backup full > 5 GiB)

---

## 7. Riscos Residuais

- **Dados reais ainda não testados:** o ensaio é analítico (sem execução real contra o S3 de produção). O primeiro restore real em staging deve ser feito na próxima janela de manutenção e os RTO confirmados.
- **Tamanho do banco vai crescer:** RTO escala com o tamanho do backup full. Quando DB exceder 5 GiB, re-medir e revisar SLO.
- **SPOF de S3:** se o bucket S3 `mecontrola-backups-660838763799-use1` ficar inacessível, o restore falha. Mitigação: `repo1-retention-full=4` garante 4 fulls no bucket; considerar cross-region replication no envelope C.
- **Chave age:** `AGE_PRIVATE_KEY` é pré-requisito para descriptografar secrets. Perda da chave = impossibilidade de restore. Garantir backup da chave em cofre offline.
