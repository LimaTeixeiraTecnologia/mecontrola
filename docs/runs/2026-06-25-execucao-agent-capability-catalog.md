# Prompt Mandatório de Execução — PRD Agent Capability Catalog

Data: 2026-06-25 · Spec: `.specs/prd-agent-capability-catalog/` · Slug: `agent-capability-catalog`

> Este arquivo é um **prompt pronto para uso**. Copie o bloco "PROMPT" abaixo e envie-o como instrução de execução. Ele é **inegociável**: sem flexibilização, sem desvios, foco em MVP robusto e production-ready/proof, com evidência fiel ao DoD e aos critérios de aceite de cada tarefa.

---

## PROMPT

Você vai **executar TODAS as 6 tarefas** do PRD `.specs/prd-agent-capability-catalog/` até `done`, em MVP robusto e **production-ready/proof**. Regras inegociáveis abaixo — **zero flexibilidade, zero desvio, zero falso positivo, zero gap, zero lacuna**.

### 0. Contrato de carga base (obrigatório antes de tocar código)
1. Ler `AGENTS.md` por completo.
2. Ler `.specs/prd-agent-capability-catalog/prd.md` (spec-version 2), `techspec.md` e os 4 ADRs (`adr-001..003` + nota de drift). **Não** reinterpretar escopo: Fase 1+2 apenas; Fases 3–7 são fora de escopo.
3. Carregar as regras hard aplicáveis e respeitá-las como bloqueantes: `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001), `.claude/rules/workflow-kernel.md` (R-WF-KERNEL-001), `.claude/rules/go-adapters.md` (R-ADAPTER-001), `.claude/rules/go-testing.md` (R-TESTING-001), `.claude/rules/governance.md`.
4. **Go é mandatório:** carregar `.agents/skills/go-implementation/SKILL.md` e executar suas Etapas 1–5 na íntegra antes de qualquer edição `.go`. Verificar a versão em `go.mod` antes de usar APIs novas.
5. **Skill `mastra` obrigatória** nas tarefas 1.0–5.0 (declaradas em cada `task-*.md`): carregar `.agents/skills/mastra/SKILL.md` antes de alterar capability/kind/workflow/tool/outcome de `internal/agent`.

### 1. Orquestração
- Usar a skill **`execute-all-tasks`** sobre `.specs/prd-agent-capability-catalog/`, respeitando o DAG declarado em `tasks.md`:
  - Ordem: `1.0 → 2.0 → 3.0 → {4.0, 6.0-gate}` e `2.0 → 5.0`; `5.0` pode rodar em paralelo a `3.0` (arquivos disjuntos: `.agents/skills/` vs `internal/agent`); `4.0` depende de `3.0` (wiring compartilhado em `module.go`); `6.0` depende de `3.0, 4.0, 5.0`.
  - **Halt-first:** ao primeiro erro irrecuperável de uma tarefa, **parar** e reportar — não mascarar, não pular, não marcar `done` parcial.
  - Spawnar subagent fresh por tarefa (isolamento de contexto) conforme a skill.
- Cada tarefa só vira `done` quando **todos** os seus `## Critérios de Sucesso`, `<requirements>` e `## Testes da Tarefa` estiverem satisfeitos **com evidência capturada** (ver §3).

### 2. Definition of Done (DoD) — por tarefa, sem exceção
Uma tarefa é `done` **se e somente se**:
1. Todos os `RF-nn` da linha de cobertura da tarefa em `tasks.md` estão implementados e verificáveis.
2. Código Go sem comentários (R-ADAPTER-001.1), sem SQL em adapter, sem `panic` em produção (R5.12), `context.Context` nas fronteiras de IO (R6), erros via `fmt.Errorf("ctx: %w", err)`/`errors.Join` (R7.6).
3. Tipos de estado fechados (DMMF state-as-type): `CapabilityMode` nunca string livre; `OperationKind`/`RunStatus`/`ToolOutcome` preservados fechados.
4. Testes da tarefa **escritos e executados verde** — unit no padrão R-TESTING-001 (testify/suite whitebox, `fake.NewProvider()`, mocks por IIFE) onde aplicável.
5. Sem regressão: nenhuma suíte existente quebrada.
6. Evidência anexada ao relatório de execução da tarefa (§3).

### 3. Evidência obrigatória (production-proof)
Para cada tarefa, gravar um `*_execution_report.md` na pasta do spec contendo:
- Arquivos criados/modificados (lista exata).
- Saída literal dos testes da tarefa (`go test ...`) — verde, com contagem.
- Saída literal dos **gates de verificação** rodados (§4) — devem retornar vazio/OK.
- Mapeamento `RF-nn → evidência` (qual teste/checagem prova cada requisito).
- Suposições residuais (idealmente nenhuma) e riscos remanescentes.
**Proibido** reportar `done` sem essa evidência. **Proibido** falso positivo: se um teste falha, reportar a falha com a saída — nunca afirmar verde sem rodar.

### 4. Gates de verificação bloqueantes (rodar e exigir vazio/OK)
Rodar ao final de cada tarefa que toca `internal/agent` e na tarefa 6.0:
```bash
# zero comentários em Go de produção (R-ADAPTER-001.1)
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" "^[[:space:]]*//" \
  internal/agent/ | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL: comentários" || echo "OK: sem comentários"

# switch de domínio NÃO cresce em daily_ledger_agent.go (R-AGENT-WF-001.1)
f=internal/agent/application/services/daily_ledger_agent.go
cases=$(grep -cE "^[[:space:]]*case intent\.Kind" "$f" || true)
[ "${cases:-0}" -gt 1 ] && echo "FAIL: switch domínio cresceu ($cases)" || echo "OK: switch contido"

# CapabilityMode não é string solta (DMMF state-as-type)
grep -rn --include="*.go" --exclude="*_test.go" 'CapabilityMode\s*=\s*"' internal/agent/ \
  && echo "FAIL: CapabilityMode string solta" || echo "OK: tipo fechado"

# workflowFor/toolFor removidos após a tarefa 3.0 (D-05/RF-08)
grep -rn --include="*.go" --exclude="*_test.go" "func workflowFor\|func toolFor" internal/agent/ \
  && echo "FAIL: workflowFor/toolFor ainda existem" || echo "OK: legado removido"

# cardinalidade de métricas (R-AGENT-WF-001.5): sem labels proibidos
grep -rn --include="*.go" --exclude="*_test.go" '"user_id"\|"correlation_key"\|"category_id"' \
  internal/agent/application/services/agent_runtime.go \
  && echo "FAIL: label de alta cardinalidade" || echo "OK: cardinalidade controlada"

# suíte completa (RF-16)
go test ./internal/agent/... ./internal/platform/workflow/...
```
> O gate "workflowFor/toolFor removidos" só se aplica a partir da conclusão de 3.0. Antes disso, é esperado existirem.

### 5. Critérios de aceite específicos (fidelidade às decisões travadas)
Validar **literalmente**, com teste:
- **D-01 / R2:** teste de consistência registry↔catálogo verde — para todo kind de `routableKinds()`, `catalog.Classify(kind).workflow == owner real de IntentRegistry.Resolve(kind)`.
- **RF-10:** teste-guard falha o build se um kind roteável não tiver `CapabilitySpec`.
- **D-02 / RF-09 / RF-17:** teste de equivalência por kind verde com **exatamente 4 exceções** declaradas (`KindQueryIncomeSummary`→`transactions`, `KindBudgetRecurrence`→`budget`, `KindDeleteTransactionByRef` e `KindEditTransactionByRef`→destrutivo). Todos os demais kinds: label **idêntico** ao legado. Comunicar o impacto de métricas no corpo do PR.
- **D-04:** `MetricsKey` espelha `ToolName`; label `tool` das métricas inalterado.
- **D-05:** `workflowFor`/`toolFor` inexistentes no código pós-3.0.
- **D-06 / ADR-003:** teste de consistência catálogo↔`intentToOperationKind` (todo destrutivo com `RequiresConfirmation==true`); suíte HITL verde; nenhuma operação destrutiva executa sem confirmação humana explícita.
- **RF-14/15:** skill `mastra` sem a afirmação "buildRegistry é o único seam"; 5 seams documentados; checklist de extensão com os 6 pontos incluindo "registrar `CapabilitySpec`".

### 6. Proibições explícitas (qualquer uma reprova a entrega)
- Adicionar `case intent.Kind` de domínio ao switch de `daily_ledger_agent.go`.
- Vazar semântica de catálogo/capability para `internal/platform/workflow` (kernel genérico).
- Preservar o label errado dos 4 kinds de drift (contraria D-02).
- Introduzir LLM fora do step de parse; tocar o ponto de parse nesta iniciativa.
- Anti-padrões DMMF: `Result/Either` custom, currying, DSL de pipeline, mônada.
- Comentário em `.go` de produção fora das exceções sancionadas.
- Marcar `done` sem evidência ou com teste não executado (falso positivo).

### 7. Fechamento
Ao concluir as 6 tarefas:
1. Rodar `ai-spec check-spec-drift .specs/prd-agent-capability-catalog` — deve retornar sem drift.
2. Atualizar `Status` de cada linha em `tasks.md` para `done`.
3. Produzir `_orchestration_report.md` consolidando: tarefas `done`, evidências por tarefa, resultado dos gates, impacto de métricas a comunicar no PR, e confirmação de cobertura RF-01..RF-17.
4. Não abrir PR nem fazer commit/push sem pedido explícito do usuário (segurança operacional, `governance.md`).

**Fim do prompt. Execução inegociável.**

---

## Referências
- PRD: `.specs/prd-agent-capability-catalog/prd.md` (Decisões Travadas D-01..D-06)
- Techspec: `.specs/prd-agent-capability-catalog/techspec.md` (mapa RF→decisão→teste)
- ADRs: `adr-001-catalogo-canonico-fonte-unica.md`, `adr-002-runtime-deriva-do-catalogo.md`, `adr-003-migracao-destrutivo-para-capabilityspec.md`
- Roadmap origem: `docs/plans/2026-06-25-mastra-gap-map-mecontrola.md` (Fase 1+2)
- Skills: `execute-all-tasks`, `go-implementation`, `mastra`
