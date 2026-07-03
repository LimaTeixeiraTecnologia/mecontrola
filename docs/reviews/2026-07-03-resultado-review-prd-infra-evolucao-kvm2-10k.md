# Resultado da Review — PRD infra-evolucao-kvm2-10k

- Data: 2026-07-03
- Ciclo: `review → bugfix → review` (2 rodadas), 4 revisores + 4 bugfixers especializados
- Fonte da verdade: working tree atual
- Regra soberana aplicada: PRD linha 85 — *"Toda afirmação de 'comprovado' exige evidência anexada; ausência de prova permanece não comprovado."*

## Veredito final

**APPROVED para o escopo implementável (código/config/doc) — 19/22 RFs.**
**NÃO COMPROVADO (bloqueio operacional, impossível neste ambiente) — RF-04, RF-05, RF-19** e a fração "execução real" de RF-06/RF-07/RF-10.

Não é possível emitir um `APPROVED` global honesto: 3 RFs exigem execução no host real de produção / S3 / staging, que nenhum agente executa neste ambiente. Emitir `APPROVED` para eles seria o falso positivo que o próprio PRD proíbe. Todos os falsos positivos que existiam nos relatórios foram **removidos** (rebaixados para status honesto).

## Rodada 1 — findings (consolidados de 4 tracks)

| # | Sev | RF | Achado | Resolução |
|---|-----|----|--------|-----------|
| A-F1 | critical | RF-03 | Report 1.0 afirmava "14/14 PASS"; suite committed falhava **5/14** (guard de imagem ausente em `deploy-swarm.sh`) | **CORRIGIDO**: guard allowlist `mecontrola-postgres:` + suite re-executada **14/14 PASS** |
| A-F2 | high | RF-02 | Nenhuma regra de alerta de backup stale/archive-push; nome de métrica divergente | **CORRIGIDO**: `mc-backup-full-stale`/`diff-stale`/`archive-push-failed` em rules.yaml + prometheus-rules.yaml alinhado |
| A-F3 | critical | RF-04/05 | Evidência de restore marcava `[x]` "restore completou sem erro" sobre ensaio auto-declarado "ESTIMADO sem execução real" | **HONESTIDADE**: checkboxes → `[ ] (NÃO EXECUTADO)`; RF-04/05 = NÃO COMPROVADO |
| A-F4 | high | RF-06 | Runbooks rotulavam RTO estimado como "medido/confirmado em ensaio" | **HONESTIDADE**: rebaixado para "estimado (projeção, pendente de restore real)" |
| A-F5 | medium | RF-03 | Guard `render-stack.py` era denylist, não allowlist | **CORRIGIDO**: allowlist positiva `mecontrola-postgres:` |
| B-F1 | high | RF-20 | Alerta `mc-version-skew` morto (`changes()==0` + `gt 0` nunca dispara) | **CORRIGIDO**: expr de contagem de versões distintas em 25h |
| B-F2 | high | RF-19 | Report 7.0 marcava RF-19 "comprovado" sem evidência real de deploy | **HONESTIDADE**: RF-19 = NÃO COMPROVADO; pipeline reconciliado (GitHub-hosted SSH) |
| B-F5 | low | RF-10 | `docker-prune.sh` com flag buildx frágil/deprecada | **CORRIGIDO**: detecção `--reserved-space`/`--keep-storage` |
| C-F1 | medium | RF-11 | `tail_sampling num_traces=100` subdimensionado → evicção de traces de erro sob burst | **CORRIGIDO**: 10000 + memory_limiter 512 |
| C-F2 | low | RF-15 | Alerta `mc-pgbouncer-client-queue` media pool da app, não fila do pgBouncer | **CORRIGIDO**: título/descrição corrigidos |
| C-F3 | low | — | Rótulos RF trocados nos echos do gate anti-storm | **CORRIGIDO** |
| C-F4 | low | RF-16 | Orçamento usava 8000 MB; host real 7936 MB | **CORRIGIDO** |
| D-F1 | low | RF-21 | `NewRateLimiter` variádico de slices lia só `[0]` | **CORRIGIDO**: assinatura `[]string` + callers |
| D-F3 | low | RF-18 | `transactions-read.js` aceitava 401 mas threshold conta 4xx | **CORRIGIDO**: precondition AUTH_TOKEN |

## Rodada 2 — validações reais (todas verdes)

- `bash deployment/scripts/tests/pgbackrest-schedule.test.sh` → **14/14 PASS, EXIT=0**
- `bash scripts/ci/deploy-anti-storm.sh` → **6/6 OK**; `deploy-anti-storm_test.sh` → **4 PASS / 0 FAIL**
- `go build ./...` → OK; `go test ./internal/onboarding/.../middleware/...` → **11 passed**
- `yaml.safe_load` de rules.yaml / otelcol / prometheus-rules / ambos workflows → OK
- Zero-comentários Go de produção (middleware, config.go, whatsapp_wiring.go) → limpo
- Varredura de falso positivo ("medido/confirmado em ensaio", `[x]` de execução) → **zero remanescente**

## Matriz de rastreabilidade (RF-01..RF-22)

| RF | Status | Base |
|----|--------|------|
| RF-01 | atendido (código) / execução VPS não verificável | systemd-timer idempotente + cron fallback |
| RF-02 | **atendido** | 3 alertas de backup + métricas confirmadas no exporter |
| RF-03 | **atendido** (comprovado por teste) | guard allowlist + suite 14/14 |
| RF-04 | **NÃO COMPROVADO** | restore PITR real não executável aqui (honestamente marcado) |
| RF-05 | **NÃO COMPROVADO** | restore VPS real não executável aqui |
| RF-06 | atendido parcial | runbooks + SLO documentados; "RTO real medido" pendente de ensaio real |
| RF-07 | atendido (artefato) / execução no host NÃO COMPROVADA | `remove-runner.sh` idempotente |
| RF-08 | **atendido** | build/scan/sign GitHub-hosted, gates bloqueantes |
| RF-09 | **atendido** | deploy SSH GitHub-hosted preserva SOPS/secrets/migrate/rollback |
| RF-10 | atendido (artefato) / instalação no host não comprovada | prune timer + alerta disco |
| RF-11 | **atendido** | sampling alinhado (collector) + tail_sampling dimensionado |
| RF-12 | **atendido** | gate anti-storm 6/6 |
| RF-13 | **atendido** | runbook SPOF/retenção |
| RF-14 | **atendido** | worst-case ~5340 MB, margem ~1,1 GB em 7936 MB |
| RF-15 | **atendido** | pool 20/30 + alertas de saturação |
| RF-16 | **atendido** | orçamento + gatilho KVM2→KVM4 |
| RF-17 | **atendido** | harness k6 A/B reproduzível |
| RF-18 | **atendido por registro honesto de gap** | envelope B declarado não comprovado (permitido pelo PRD) |
| RF-19 | **NÃO COMPROVADO** | deploy real da main não executável aqui |
| RF-20 | **atendido** | `version-drift-check.yml` funcional (primário) + `mc-version-skew` corrigido |
| RF-21 | **atendido** | allowlist 14 CIDRs Meta (Caddy + Go) |
| RF-22 | **atendido** | pg-tunnel `127.0.0.1:15432` |

## Riscos residuais (não bloqueantes, honestos)

1. **RF-04/05/19 e execução de RF-06/07/10**: comprovação real depende de restore em staging, deploy da main e ação no host — a executar na próxima janela de manutenção, anexando evidência física (RTO real, `docker service ls`, `ps`/`df`).
2. `mc-version-skew` (Grafana) é proxy metric-only; a cobertura primária de RF-20 é o workflow CI que compara SHA vs `main` HEAD.
3. CIDRs Meta hardcoded sujeitos a drift — revisão anual documentada no runbook.
4. Calibração de `otel-lgtm` (1228M) e pool sob carga real pendente da execução do harness (RF-18).
