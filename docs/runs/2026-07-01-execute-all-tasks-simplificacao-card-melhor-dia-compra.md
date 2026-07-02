# Prompt PRONTO PARA USO — `execute-all-tasks` do PRD `simplificacao-card-melhor-dia-compra`

> Data: 01/07/2026 (pt-br). Skill alvo: `.github/skills/execute-all-tasks/` (governance, v1.8.0).
> Este documento é o **prompt de invocação + contrato vinculante**, sem desvios, sem flexibilização.
> Pré-condições verificadas empiricamente em 2026-07-01 (ver §5). Nenhuma etapa aqui é opcional.

---

## 1. Invocação (copiar e usar)

```
/execute-all-tasks simplificacao-card-melhor-dia-compra
```

Equivalentes aceitos como input pela skill (slug, `prd-<slug>` ou path):
- `simplificacao-card-melhor-dia-compra`
- `.specs/prd-simplificacao-card-melhor-dia-compra/`

**Prompt textual completo para o orquestrador** (quando invocado por texto, use verbatim):

> Execute TODAS as tarefas do PRD `simplificacao-card-melhor-dia-compra` via a skill `execute-all-tasks`,
> sem desvios e sem flexibilizar nenhuma regra. Respeite integralmente o DAG e o `Paralelizável` de
> `.specs/prd-simplificacao-card-melhor-dia-compra/tasks.md`, o contrato YAML estrito por tarefa, a
> política halt-first (parar na primeira tarefa não-`done` após validar a wave) e a proibição de
> re-execução automática. Cada tarefa roda em subagent fresh via `execute-task`, carregando apenas
> governance + skill de linguagem detectada no diff + skills declaradas no task file. `go-implementation`
> é OBRIGATÓRIA em todas as 9 tarefas (Go/SQL). Não mutar `tasks.md` no orquestrador — só os subagents,
> via `execute-task`, com `flock`/rename atômico. Ao final, gerar
> `.specs/prd-simplificacao-card-melhor-dia-compra/_orchestration_report.md`.

---

## 2. Escopo imutável (artefatos de origem — NÃO editar durante a run)

| Artefato | Caminho |
|---|---|
| PRD (v2) | `.specs/prd-simplificacao-card-melhor-dia-compra/prd.md` |
| Techspec | `.specs/prd-simplificacao-card-melhor-dia-compra/techspec.md` |
| Tasks | `.specs/prd-simplificacao-card-melhor-dia-compra/tasks.md` |
| ADRs | `adr-001-bank-free-text-no-fk.md` … `adr-005-consolidate-nickname.md` |

`spec-hash-prd` = `5384373f31eb476dea92e45e83a8872200c7a32d552818507909436676973663`
`spec-hash-techspec` = `2185b3ec9320db97cce02658cb252934bb166450b7a3de303c9dc6419ad96efc`
Ambos gravados em `tasks.md` e conferidos por `ai-spec check-spec-drift` (sem drift). Se qualquer
artefato de origem for editado antes/durante a run, **abortar** e re-sincronizar via `create-tasks`.

---

## 3. Ordem de execução esperada (DAG → waves)

Derivada de `tasks.md` (deps + `Paralelizável`). O orquestrador deve reproduzir exatamente esta ordem
topológica; qualquer divergência é violação de contrato.

| Wave | Tarefas | Justificativa |
|---|---|---|
| W1 | **1.0**, **2.0**, **8.0** | sem dependências; módulos/arquivos disjuntos (migration ⟂ domínio card ⟂ budgets) — paralelizáveis (`Com`) |
| W2 | **3.0** | depende de 1.0 (tabela `banks`) + 2.0 (VO `BankCode`) |
| W3 | **4.0** | depende de 2.0 + 3.0 |
| W4 | **5.0**, **6.0** | 5.0 dep 4.0; 6.0 dep 2.0; HTTP ⟂ messaging — paralelizáveis entre si |
| W5 | **9.0** | depende de 4.0 (nova assinatura `cardinput.CreateCard`); pode correr junto de 6.0 se 6.0 ainda estiver na W4 |
| W6 | **7.0** | depende de 5.0 + 6.0 (contrato/e2e + não-regressão de transactions) |

> Observação: o loop topológico da skill (Etapa 3) recompõe as waves relendo `tasks.md` a cada rodada.
> A tabela acima é a expectativa verificável; a autoridade é o grafo em `tasks.md`.

---

## 4. Guardrails invioláveis (0 flexibilidade)

**Governança por tarefa (todas as 9):**
- `go-implementation` OBRIGATÓRIA (Etapas 1-5, R0-R7) — auto-carregada pelo diff Go/SQL; mandatória por `CLAUDE.md`.
- DMMF: `PurchaseDayService.Decide` e `Decide*` puros (sem IO, sem `context`, `now`/`tz` explícitos); smart constructors; state-as-type (`ThresholdAlertKind`, `BankCode`).
- **Zero comentários** em `.go` de produção (R-ADAPTER-001.1). Proibido `panic` (R5.12). Proibido `_ = var` para silenciar não-uso. Proibido abstrair tempo (usar `time.Now().UTC()` inline).
- Adapters finos (R-ADAPTER-001.2): handler/consumer/producer só mapeiam → usecase; sem SQL/branching de domínio.
- Input DTOs: `Validate()` com `errors.Join`, campo nomeado, após `defer span.End()` (R-DTO-VALIDATE-001).
- Testes: testify/suite whitebox, `fake.NewProvider()`, IIFE por mock (R-TESTING-001).
- Skills declaradas extra: `otel-grafana-dashboards` (8.0), `mastra` (9.0).

**Riscos de integração que NÃO podem ser violados:**
1. **Co-entrega 1.0 + 8.0:** a migration (1.0) dropa `cards.limit_cents`; enquanto 8.0 não remover a leitura em `internal/budgets`, uma query a `c.limit_cents` quebra em runtime. As duas devem entrar no MESMO deploy. Nenhuma vai a produção sozinha.
2. **Ordering chi (5.0):** `GET /cards/best-purchase-day` registrado ANTES de `Route("/{id}")`; gate em `router_test.go`.
3. **Renomeação atômica (6.0):** `card_name` → `card_nickname` em producer + consumer + `NotifyInvoiceDueInput` no mesmo PR; sem `LimitCents` na cadeia.
4. **RF-14 (7.0):** `internal/transactions` NÃO pode ser alterado. Prova = suíte de transactions verde E `git diff --stat internal/transactions/` vazio. Qualquer diff = regressão de contrato → halt.
5. **Cache `closing_day` (ADR-002):** derivado e persistido no cadastro/edição; recomputado no `UpdateCard` ao mudar `bank`/`due_day`. Sem reconciliação em massa (fora de escopo).

**Protocolo do orquestrador (Regras invioláveis da skill):**
- Toda tarefa em subagent fresh — orquestrador NUNCA executa `execute-task` inline.
- Contrato YAML estrito (`status`, `report_path`, `summary`) — violação = `failed: contract violation`, halt.
- Paralelismo só com flag `Paralelizável` em `tasks.md` E suporte nativo do tool.
- Halt-first: aguardar TODOS da wave, validar cada retorno (4 passos), só então decidir halt.
- NÃO mutar `tasks.md` no orquestrador; NÃO re-executar tarefa automaticamente.
- Cada subagent: `export AI_INVOCATION_DEPTH=0`, `source` de `check-invocation-depth.sh`, `export AI_PREFLIGHT_DONE=1`.

---

## 5. Pré-condições — VERIFICADAS em 2026-07-01 (evidência, sem falso positivo)

| Gate (Etapa 1 da skill) | Comando | Resultado real |
|---|---|---|
| Binário `ai-spec` | `command -v ai-spec` | `/opt/homebrew/bin/ai-spec` ✓ |
| Hook de pré-voo (enforcement real) | `bash .agents/hooks/pre-execute-all-tasks.sh simplificacao-card-melhor-dia-compra` | `OK (... 9 tarefas validadas)`, exit 0 ✓ |
| Hooks de wave/tarefa | presença | `.agents/hooks/{post-execute-task,post-wave}.sh` ✓ |
| Lib de profundidade | cascata `.agents/lib` → `scripts/lib` | `.agents/lib/check-invocation-depth.sh` ✓ |
| Cobertura de RF / drift | `ai-spec check-spec-drift .specs/prd-.../tasks.md` | `OK: sem drift detectado` (RF-01..RF-20) ✓ |
| Lockfile de skills externas | `ai-spec skills check` | `6 skill(s) verificadas` (versões "desconhecidas" só em skills externas não usadas por esta run) ✓ |
| Subagent nativo (isolamento) | `ls .claude/agents/task-executor.md .github/agents/task-executor.agent.md` | ambos presentes ✓ (Claude + Copilot = **verificado**) |

**Ressalva honesta e obrigatória (não é bloqueio, mas NÃO pode ser "corrigida" cegamente):**
`ai-spec verify` reporta `go-implementation` como **DRIFTED**. Isto é **intencional**: a skill
`go-implementation` está customizada com as regras mandatórias do projeto (ver `CLAUDE.md` e
`.claude/rules/*`). O hook autoritativo de pré-voo passou (exit 0) e o `pre-execute-all-tasks.sh` NÃO
valida hash de conteúdo desta skill. **NÃO executar `ai-spec install`/`ai-spec upgrade`** para "resolver"
o drift — isso sobrescreveria as regras locais do projeto (regressão de governança). O gate de drift
relevante para esta run é `ai-spec check-spec-drift` (spec-hash PRD/techspec ↔ tasks), que passou.

**Nota de tool (Claude Code):** a primitiva `Agent` roda in-process — sem kill nativo no timeout
(`AI_TASK_TIMEOUT_SECONDS`, default 1800s): soft-discard do YAML tardio. Registrar `subagente: Agent
(in-process)` no `_orchestration_report.md`. Em Codex/Gemini/Copilot o subprocesso é morto no timeout.

---

## 6. Critérios de aceitação da run (production-ready / proof)

A run só é `done` quando TODOS forem verdade:
- [ ] `tasks.md`: as 9 tarefas com `status: done` (1.0–9.0).
- [ ] `.specs/prd-simplificacao-card-melhor-dia-compra/_orchestration_report.md` gerado (snapshot inicial vs final, waves, subagente usado).
- [ ] Cada tarefa com `<id>_execution_report.md` não-vazio (evidência física validada, path relativo à raiz).
- [ ] `ai-spec check-spec-drift .specs/prd-simplificacao-card-melhor-dia-compra` → sem drift ao final.
- [ ] `go build ./...` e `go test ./internal/card/... ./internal/budgets/... ./internal/agents/... ./migrations/...` verdes.
- [ ] **RF-14:** `go test ./internal/transactions/...` verde E `git diff --stat internal/transactions/` VAZIO.
- [ ] Gates de regra sem violação: zero comentários em `.go` novos/alterados; sem `panic`; adapters finos; DTOs com `Validate()`.
- [ ] Grep de resíduo vazio: `LimitCents`/`CardLimit`/`CardName`/`card_limit_near`/`CardThresholdReader` fora de contexto legado; `card_name` só onde legítimo.
- [ ] Exemplo funcional (RF-13): `GET /cards/best-purchase-day?bank=nubank&due_day=20` → `{closing_day:13, best_purchase_day:14}` coberto por teste.

Se qualquer subagent retornar `≠ done`: **halt** (não re-executar), consolidar `_orchestration_report.md`,
retornar status agregado `partial` e reportar a tarefa bloqueante + `report_path`.

---

## 7. Comandos de verificação pós-run (rodar após status agregado)

```bash
# 1) Cobertura e hashes
ai-spec check-spec-drift .specs/prd-simplificacao-card-melhor-dia-compra

# 2) Build + testes dos módulos afetados
go build ./...
go test ./internal/card/... ./internal/budgets/... ./internal/agents/... ./migrations/...

# 3) RF-14 — transactions inalterado (deve ser VAZIO)
git diff --stat internal/transactions/
go test ./internal/transactions/...

# 4) Zero comentários em Go de produção (deve retornar vazio)
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" "^[[:space:]]*//" \
  internal/card internal/budgets internal/agents | grep -Ev "(//go:|//nolint:|// Code generated)" || true

# 5) Resíduo de campos removidos (deve estar limpo / só legado)
grep -rn "LimitCents\|CardLimit\|card_limit_near\|CardThresholdReader\|ActiveCardForScan" \
  internal/card internal/budgets internal/agents || true
```

---

## 8. O que este prompt NÃO autoriza (limites explícitos)

- NÃO editar `prd.md`/`techspec.md`/`tasks.md`/ADRs durante a run (drift → abortar).
- NÃO rodar `ai-spec install`/`upgrade` para "resolver" o drift de conteúdo de `go-implementation`.
- NÃO alterar `internal/transactions` (RF-14).
- NÃO reintroduzir `limit_cents`/`CardName`/`closing_day` como entrada, nem manter campo morto/enganoso.
- NÃO paralelizar tarefas fora do que `tasks.md` marca como `Paralelizável`.
- NÃO re-executar tarefa que retornou `blocked`/`failed`/`needs_input` sem decisão humana explícita.
