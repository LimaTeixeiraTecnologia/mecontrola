# Plano de execução — robustez, eficiência, evolutivo (production-proof)

Data: 2026-06-25
Escopo: diagnóstico do codebase + plano **pronto para execução**, ancorado em evidência de código (file:line) já verificada. **0 falso positivo**: cada item foi confirmado no código; o que já existe está marcado como FEITO e fora do plano.

---

## 1. O que é o mecontrola (resumo)

Produto de **finanças pessoais conversacional via WhatsApp**. Monólito modular Go 1.26, 3 entrypoints Cobra (`server`/`worker`/`migrate`), 9 bounded contexts DDD compostos por factories (`internal/<mod>/module.go`).

Espinha dorsal madura: **outbox transacional** (idempotência por `event_id`, `FOR UPDATE SKIP LOCKED`), **kernel de workflow durável** (`Engine[S]`, suspend/resume, concorrência otimista), **agente LLM** (OpenRouter, registry de tools, HITL, Thread→Run auditável), **observabilidade OTel** e **governança executável forte** (forbidigo, ai-spec, PCI R0–R7, distroless assinado).

---

## 2. Achados verificados (estado real — corrige falsos positivos)

| Item | Estado real (evidência) | Veredito |
|---|---|---|
| **Alerting-as-code** | `deployment/monitoring/prometheus-rules.yaml` com grupos infra/postgres/api/ssl/backup + **agent** (parse-unknown, fallback exhausted, provider error, policy block, idempotency replay); `alertmanager.yml`; grafana provisioning | ✅ **FEITO** (maduro). Gap estreito: §3 R2 |
| **Métricas LLM** | `agent_llm_tokens_total` (prompt/completion) em `openrouter/client.go:69,210`; provider call/error, confidence histogram | ✅ **FEITO**. Falta só custo USD: §3 E1 |
| **Housekeeping workflow_runs** | `HousekeepingJob` + `DeleteCompleted(retention,limit)` (`workflow/housekeeping.go`, `store.go:239`), wired em `agent/module.go:523` | ✅ **FEITO** |
| **TTL agent_observations / agent_sessions** | `expires_at` + `DELETE ... WHERE expires_at<=$1` (`observation_repository.go:87`); `DELETE FROM agent_sessions` (`agent_session_repository.go:174`) | ✅ **FEITO**. Falta confirmar runs/decisions/threads: §3 E2 |
| **pgx + PgBouncer transaction mode** | `instrumentDriver("pgx", uri)` (`postgres.go:24`, database/sql, extended protocol default); DSN `formatDSN` **sem** `default_query_exec_mode` (`config.go:291`); `pool_mode=transaction` **sem `max_prepared_statements`** (`pgbouncer.ini:15`) | 🔴 **RISCO REAL** (latente): §3 R1 |
| **Drift de classificação no agent** | `workflowFor`/`toolFor` × `IntentRegistry` divergem em 4 kinds (métrica mente) | 🔴 Coberto pelo SDD `prd-agent-capability-catalog` |

---

## 3. Itens de trabalho (escopo cirúrgico, sem inchaço)

### R1 — Corrigir pgx + PgBouncer (transaction mode) [P0, bloqueia produção] — ✅ CÓDIGO FEITO (2026-06-25) · ⏳ falta só load test real
**STATUS:** corrigido em `internal/platform/database/postgres/postgres.go` — `New()` registra `pgx.ConnConfig` com `DefaultQueryExecMode = QueryExecModeSimpleProtocol` via `stdlib.RegisterConnConfig` (zero prepared statements server-side → compatível com transaction pooling por construção). Verificado independentemente: `go build ./...` exit 0; `go test ./internal/platform/database/...` ok (+ 3 testes unitários novos em `postgres_test.go`); gate zero-comentários OK; **revisão adversarial (subagente reviewer) = "fix correto"**, confirmou que server/worker/migrate (`cmd/*`) passam por `New()` e que nenhum `sql.Open("pgx")` extended escapa em produção (os diretos só em `test_helper.go`, build-tagged, contra Postgres cru). **Pendência única:** validar sob carga contra PgBouncer real (Docker/PgBouncer indisponíveis neste ambiente) — passos abaixo. Risco residual baixo: simple protocol manda parâmetros como texto (sem binário); validar suíte de integração completa com Docker antes do deploy. Hardening opcional (defesa em profundidade): `PGBOUNCER_MAX_PREPARED_STATEMENTS` no PgBouncer ≥1.21.

**Problema confirmado:** sob `pool_mode=transaction`, prepared statements server-side criados pelo pgx (extended protocol default) não sobrevivem à troca de conexão do pool → erros intermitentes `prepared statement "stmtcacheXXX" does not exist`/`already exists` sob concorrência.
**Correção (escolher 1, validar sob carga):**
- (a) Registrar `pgx.ConnConfig` com `DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol` (ou `QueryExecModeExec`) via `stdlib.RegisterConnConfig`, usando o nome retornado como `driverName` em `instrumentDriver`; **ou**
- (b) Habilitar `max_prepared_statements` no PgBouncer (≥1.21) em `pgbouncer.ini` + `compose.yml` (env bitnami `PGBOUNCER_MAX_PREPARED_STATEMENTS`).
**DoD:** teste de carga concorrente contra PgBouncer transaction-mode sem erro de prepared statement; decisão documentada (a ou b) com justificativa; evidência anexada.
**Arquivos:** `internal/platform/database/postgres/{postgres.go,otelsql.go}`, `configs/config.go` (DSN) e/ou `deployment/pgbouncer/pgbouncer.ini` + `deployment/compose/compose.yml`.

### R2 — alertas faltantes [P1, quick win] — ✅ FEITO (2026-06-25)
**STATUS:** adicionadas 3 rules ao grupo novo `mecontrola.platform` em `deployment/monitoring/prometheus-rules.yaml`: `OutboxDeadLetter` (`increase(outbox_dead_letter_total[10m]) > 0`, critical), `WorkflowRunFailures` (`workflow_runs_total{status="failed"}` ratio > 0.10, warning), `WorkflowVersionConflictSpike` (`rate(workflow_version_conflict_total[5m]) > 1`, warning). Nomes/labels de métrica confirmados no código (`outbox/dispatcher.go:59`, `workflow/engine.go:51,57`). Verificado independentemente: `promtool check rules` = SUCCESS (16 rules).

### E2 — Confirmar retention de agent_runs/agent_decisions/agent_threads [P1]
Verificar se essas 3 tabelas JSONB têm poda. `observations`/`sessions` já têm; se runs/decisions/threads crescem sem TTL, adicionar job de retention (espelhar `HousekeepingJob`).
**DoD:** ou evidência de que já há poda, ou job de retention novo com teste de integração.

### E1 — Custo USD por modelo [P2, opcional]
Derivar custo a partir de `agent_llm_tokens_total` × tabela de preço por modelo (label `model`/`provider`). Não bloqueia nada; habilita FinOps do LLM.

---

## 4. Os dois SDDs prontos em `.specs/`

| SDD | Escopo | Estado | Prompt de execução |
|---|---|---|---|
| `prd-agent-capability-catalog` | Catálogo canônico + runtime deriva auditoria/métrica + skill mastra (Fase 1+2 do gap-map). Corrige o drift dos 4 kinds. | ✅ Completo: prd+techspec+3 ADRs+tasks (6) | `docs/runs/2026-06-25-execucao-agent-capability-catalog.md` |
| `prd-onboarding-conversacional` | Onboarding 8 etapas no kernel durável, fase como tipo fechado, conclusão sem 1ª transação. | ✅ Completo: prd+techspec+4 ADRs+tasks (9) | `docs/runs/2026-06-25-executar-onboarding-conversacional.md` |

**Interação crítica:** ambos tocam o wiring `internal/agent/module.go`. Executar **em sequência** (não em paralelo) para evitar conflito; o onboarding registra `CapabilitySpec` dos seus kinds/workflows **se** o catálogo já existir → catálogo antes do onboarding reduz retrabalho.

---

## 5. SEQUÊNCIA DE EXECUÇÃO — o que fazer, em ordem

> Regra de ouro: **corrigir fundação e visibilidade antes de expandir capabilities.** Cada passo só fecha com evidência (teste verde + gate). Não pular, não flexibilizar.

### PASSO 0 — AGORA (bloqueia produção, independe dos SDDs) — ✅ CONCLUÍDO (2026-06-25)
1. **R1 — pgx/PgBouncer.** ✅ Código corrigido + revisado adversarialmente + build/test/gate verdes. ⏳ Resta apenas a validação de carga contra PgBouncer real (não reproduzível neste ambiente sem Docker) — executar os passos da seção R1 antes do deploy.
2. **R2 — alertas** (outbox dead-letter + workflow failures/conflict). ✅ 3 rules adicionadas, `promtool` verde.

> **Próximo:** PASSO 1 (executar o SDD `prd-agent-capability-catalog`). Nada mais do PASSO 0 está pendente além do load test manual do R1.

#### Validação de carga do R1 (executar antes do deploy)
1. Subir `deployment/compose/compose.yml` (PgBouncer `pool_mode=transaction`) + Postgres + `server`/`worker`.
2. Forçar troca de backend por transação: `DEFAULT_POOL_SIZE=2`, `MAX_DB_CONNECTIONS=2`, clientes >> servidores.
3. Gerar concorrência num endpoint de leitura: `hey -z 60s -c 50 https://<host>/<endpoint>` (ou k6/vegeta).
4. **Esperado pós-fix:** zero ocorrências de `prepared statement "stmtcacheXXX" does not exist`/`already exists` nos logs de `server`/`worker` e no PgBouncer (`log_pooler_errors=1`).
5. Sanidade: rodar serviço `migrate` (conecta direto ao Postgres) — `migrate up` sem erro.

### PASSO 1 — Fundação do agente (SDD #1)
3. **Executar `prd-agent-capability-catalog`** (6 tasks) com o prompt `docs/runs/2026-06-25-execucao-agent-capability-catalog.md`, via skill `execute-all-tasks`. Corrige o drift de métrica (mentira em produção) e cria a fonte única de verdade. Menor e foundational → primeiro.

### PASSO 2 — Feature de onboarding (SDD #2)
4. **Executar `prd-onboarding-conversacional`** (9 tasks) com o prompt `docs/runs/2026-06-25-executar-onboarding-conversacional.md`. Rebase sobre o catálogo: registrar `CapabilitySpec` dos kinds/workflows novos do onboarding. Sequencial ao Passo 1 (conflito de `module.go`).

### PASSO 3 — Hardening de operação (pós-fundação)
5. **E2 — retention** de agent_runs/agent_decisions/agent_threads (verificar; adicionar job se faltar).
6. **E1 — custo USD por modelo** (opcional, FinOps do LLM).

### PASSO 4 — Evolução (specs futuros, ordem do gap-map)
7. Gerar e executar, um por vez via pipeline `create-prd → create-technical-specification → create-tasks`:
   **Evals (Fase 4) → Console operacional (Fase 3) → Memory 2.0 (Fase 5) → Multi-agent/workspaces (Fase 6) → Capabilities externas (Fase 7).**
   Cada fase só cresce sobre a base estável e introspectável dos Passos 1–2.

---

## 6. Critérios inegociáveis (todo passo)
- DoD e critério de aceite atendidos **com evidência** (saída de teste + gate) antes de `done`.
- Gates de governança verdes: zero comentários Go (R-ADAPTER-001.1), cardinalidade de métrica (R-AGENT-WF-001.5), sem SQL em adapter, sem `case intent.Kind` novo, DMMF state-as-type.
- Sem falso positivo: teste que falha é reportado com a saída; nada de "verde" sem rodar.
- Não abrir PR/commit/push sem pedido explícito (segurança operacional, `governance.md`).

## 7. Referências
- Roadmap: `docs/plans/2026-06-25-mastra-gap-map-mecontrola.md`
- SDD #1: `.specs/prd-agent-capability-catalog/` · prompt `docs/runs/2026-06-25-execucao-agent-capability-catalog.md`
- SDD #2: `.specs/prd-onboarding-conversacional/` · prompt `docs/runs/2026-06-25-executar-onboarding-conversacional.md`
- Evidência de código citada inline (§2) — toda verificada nesta sessão.
