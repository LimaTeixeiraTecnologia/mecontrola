# Prompt PRONTO PARA USO — `execute-all-tasks` · MeControlaAgent

> Data: 30/06/2026 (pt-br). Skill alvo: `execute-all-tasks` (`.github/skills/execute-all-tasks/`, `category: governance`, depends_on: `execute-task`, `agent-governance`).
> Cole o bloco abaixo como instrução. Execução **sem desvios, 0 gaps, 0 lacunas, 0 falso positivo, production-ready/proof, sem flexibilidade**.

---

## PROMPT (copie a partir daqui)

Use a skill **`execute-all-tasks`** para executar **TODAS** as tarefas do PRD **`prd-mecontrola-agent`** (slug: `mecontrola-agent`; pasta: `.specs/prd-mecontrola-agent/`), do início ao fim, respeitando o DAG, sem pular nem flexibilizar nenhuma regra.

### Fontes canônicas (ler na íntegra; não inferir além delas)
- PRD: `.specs/prd-mecontrola-agent/prd.md` (spec-version 5; RF-01..RF-39 + sub-RFs; decisões D-01..D-28).
- Techspec: `.specs/prd-mecontrola-agent/techspec.md` (8 ADRs `adr-001`..`adr-008`; seção "Contratos de Comunicação entre Módulos"; seção "Skills e Referências Obrigatórias").
- Tarefas: `.specs/prd-mecontrola-agent/tasks.md` (9 tarefas) + `task-1.0..task-9.0`.
- Spec-hash sincronizado e sem drift (validado): `ai-spec check-spec-drift .specs/prd-mecontrola-agent` DEVE retornar "sem drift" antes de iniciar e permanecer assim.

### Pré-voo obrigatório (halt-first; falhar em vez de degradar)
1. Rodar o hook `pre-execute-all-tasks.sh mecontrola-agent` (cascata `.agents/hooks/`→`.claude/hooks/`→…). Exit ≠ 0 → parar e repassar stderr.
2. `ai-spec skills --verify` → `blocked` se houver drift de skills.
3. `ai-spec check-spec-drift .specs/prd-mecontrola-agent/tasks.md` → `blocked` se algum RF não estiver coberto.
4. Confirmar presença de `prd.md`, `techspec.md`, `tasks.md` → `needs_input` se faltar.
5. **Não** prosseguir em "modo legado": ausência de `ai-spec`/hook/lib é `failed`/`needs_input`, nunca bypass.

### Ordem de execução (DAG — respeitar exatamente)
1.0 → (2.0 ∥ 3.0) → 4.0 → (5.0 ∥ 6.0) → 7.0 → 8.0 → 9.0
- Paralelizar **somente** os pares marcados em `tasks.md` (`2.0 ↔ 3.0`, `5.0 ↔ 6.0`) e apenas se o tool suportar spawn nativo; caso contrário, sequencial.
- **9.0 (cutover) é a última** e depende de todas; só executar com 1.0–8.0 `done`.

### Regras inegociáveis por tarefa (sem exceção)
- **Skills**: cada tarefa Go carrega `go-implementation` (auto, por diff — Etapas 1–5, R0–R7) e **`mastra`** (declarada em `## Skills Necessárias`, base canônica do agente sobre `internal/platform`). **DMMF** (Domain Modeling Made Functional) é regra de modelagem mandatória: state-as-type (`OnboardingPhase`, `AwaitingKind`, `OperationKind`, `ToolOutcome`, `RunStatus` fechados), smart constructors, `Decide*` puro; **proibido** `Result[T,E]`/`Either`/currying/DSL de pipeline/monades.
- **Gates de governança (todos verdes)**: R-AGENT-WF-001 (registry sem `switch intent.Kind`; tool fina sem regra/SQL/branching; estados fechados; LLM só nas call-sites sancionadas; Thread-first; pending step antes de clarify; HITL `.7-A` reemitido), R-WF-KERNEL-001 (kernel genérico, merge-patch no resume), R-ADAPTER-001 (zero comentários em `.go` de produção; adapters finos; sem SQL direto fora do postgres), R-DTO-VALIDATE-001 (`Validate()` após span), R-TESTING-001 (testify/suite whitebox, `fake.NewProvider`, IIFE por mock).
- **Correção financeira**: fronteira de tools (escrita **só** via `internal/transactions`; **nunca** `budgets.UpsertExpense` — anti-dupla-contagem D-13); idempotência exatamente-uma-vez (RF-38, ledger `(wamid,item_seq,operation)`); operações destrutivas **só** com confirmação humana + aviso de impacto (RF-27); data-default `America/Sao_Paulo`; parcelamento ≤24; distribuição fecha 100%.
- **Proibições específicas do repositório**: sem `init()`, sem `panic` em produção, `context.Context` nas fronteiras de IO, `errors.Join`/`fmt.Errorf("ctx: %w", err)`, goroutines canceláveis; **proibido abstrair tempo** (`time.Now().UTC()` inline); **proibido** `var _ Interface = (*T)(nil)`; `defer func(){ _ = rows.Close() }()`.

### Definição de "done" por tarefa (objetiva)
- Subtarefas concluídas; critérios de aceitação do `task-X.0` atendidos; RFs da tarefa cobertos.
- **Testes criados e executados** (testify/suite whitebox; integração com testcontainers Postgres onde a tarefa exige — ex.: 3.0/4.0/6.0 provando idempotência e propagação lançamento→`budgets_expenses`→`GetMonthlySummary` **sem dupla contagem**).
- `go build ./...`, `gofmt` e os gates de governança **verdes**; report de execução com `DiffSHA` válido.
- Variante real atrás de `RUN_REAL_LLM` para tool-calling + structured output `Strict:true` (modelo `openai/gpt-4o-mini`).

### Definição de pronto global (Definition of Done do PRD)
- 9.0 conclui o **cutover sem resíduo**: `grep -rn "weather\|WeatherClient" internal/agents cmd/` retorna vazio; `internal/onboarding` (ativação por magic token) **intacto**.
- Jornada validada ponta a ponta no WhatsApp: onboarding de 8 etapas (distribuição em mensagem única fechando 100%; cartão só apelido+vencimento) + operação diária (registrar receita/despesa, cartão parcelado, consultar resumo, editar/remover com confirmação).
- `ai-spec check-spec-drift .specs/prd-mecontrola-agent` sem drift ao final; suíte de testes determinística verde no CI.

### Comportamento de orquestração
- **Halt-first**: à primeira falha de gate/teste, parar a cadeia afetada e reportar `failed`/`blocked` com evidência (não mascarar, não "seguir mesmo assim").
- **Retomada idempotente**: tarefas `done` não re-executam; reentrância segura.
- **Sem flexibilização** por conveniência, ferramenta, deadline ou "modo legado". Evidência obrigatória (arquivos alterados, validações executadas, riscos residuais).

Comece pelo pré-voo, construa o grafo a partir de `tasks.md`, e execute 1.0→9.0 na ordem do DAG, spawnando um subagent fresh por tarefa.

## (fim do prompt)

---

### Notas de uso
- Invocação equivalente em comando: `execute-all-tasks mecontrola-agent` (ou pelo path `.specs/prd-mecontrola-agent/`).
- Pré-requisitos: binário `ai-spec` no PATH, hook `pre-execute-all-tasks.sh` instalado, Postgres para testes de integração (testcontainers), `RUN_REAL_LLM` + `OPENROUTER` configurados para a variante real.
- Rastreabilidade: `tasks.md` carrega `spec-hash-prd c8ae9e47…` e `spec-hash-techspec e303a4a2…`; qualquer edição posterior de PRD/techspec exige re-sincronizar antes de executar (evita drift silencioso).
