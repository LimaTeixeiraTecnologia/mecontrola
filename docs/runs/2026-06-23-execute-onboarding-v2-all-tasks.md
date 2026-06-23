# Prompt Mandatório — Executar TODAS as Tarefas do Onboarding V2 (`execute-all-tasks`)

- **Data**: 2026-06-23
- **Skill alvo**: `.github/skills/execute-all-tasks/` (orquestra `execute-task` por tarefa)
- **Bundle de origem**: `.specs/prd-onboarding-v2/` (prd.md, techspec.md, adr-001..006, tasks.md, task-1.0..13.0)
- **Escopo**: 13 tarefas; MVP robusto, production-ready/proof, inegociável
- **Skills obrigatórias na execução Go**: `go-implementation` (auto por linguagem) + `mastra` (declarada nas tasks 8–12) + `agent-governance` (auto)

---

## Como usar

Cole o bloco abaixo (entre as cercas) como prompt para o agente nesta mesma sessão/repo. Ele invoca a
skill `execute-all-tasks` com o slug `onboarding-v2` e impõe os critérios de aceite com evidência.

```text
Você é o orquestrador de execução. Invoque a skill `execute-all-tasks` para o PRD `onboarding-v2`
(`.specs/prd-onboarding-v2/`) e execute TODAS as 13 tarefas até `done`, de forma INEGOCIÁVEL e
production-ready/proof. Não faça nada fora do que as tarefas, o PRD, a techspec e os ADRs definem.

ENTRADA
- slug: onboarding-v2
- bundle: .specs/prd-onboarding-v2/ (prd.md, techspec.md, adr-001..006, tasks.md, task-1.0..13.0)

REGRAS INVIOLÁVEIS (além das da própria skill)
1. Cada tarefa roda em subagent fresh via `execute-task`; o orquestrador NUNCA executa inline.
   Respeite o DAG e o `Paralelizável` de tasks.md (waves: 1.0↔2.0↔4.0↔13.0; depois 5.0↔6.0↔7.0;
   depois 8.0; 9.0; 10.0↔11.0; por fim 12.0). Halt-first: pare na primeira tarefa ≠ done.
2. FRONTEIRA DE BOUNDED CONTEXTS (ADR-006) — inegociável: `internal/agent` é só ponte
   (WhatsApp→cmd/server→agent→OpenRouter→módulos). Proibido regra/persistência de outro módulo no
   agent. Distribuição/budget→`internal/budgets`; cartão→`internal/card`; lançamento→
   `internal/transactions`; regras de onboarding→`internal/onboarding`. Integração via binding→usecase
   ou eventos do outbox. NÃO recriar a integração já existente (consumers de `splits_calculated`/
   `card_registered`; `ExpenseRecorder`; `SynchronousCardCreator`).
3. CONTRATOS DE VALIDAÇÃO POR MÓDULO (techspec "Contratos de Validação por Módulo") são obrigatórios
   ao chamar: renda R$500..R$1B; objetivo ≤280; nickname 1..32; split = exatamente 5 categorias com
   soma == income; Σ basis points ≤10000; competence YYYY-MM; `doc` proibido; outcome exige
   subcategoria; cartão closing_day 1..31 (due_day opcional, derivado no `internal/card` — Tarefa 13.0).
4. DoD + CRITÉRIOS DE ACEITE COM EVIDÊNCIA: uma tarefa só é `done` quando TODOS os itens de
   "Definition of Done (DoD)" e "Critérios de Aceite (validações executáveis)" do seu task-*.md
   forem satisfeitos e EXECUTADOS, com a saída real (comandos `go build`/`go test`/greps de gate)
   registrada no `<id>_execution_report.md`. Sem evidência física e legível = NÃO é done
   (`failed: missing evidence`). Proibido falso positivo: relatar verde sem rodar é violação.
5. SKILLS: cada subagent carrega `go-implementation` (Etapas 1–5 + checklist R0–R7) por ser Go, e
   `mastra` quando a tarefa tocar `internal/agent` (tasks 8–12, conforme coluna Skills de tasks.md).
   Zero comentários em `.go` de produção (R-ADAPTER-001.1); testes no padrão testify/suite
   (R-TESTING-001); DTOs de input com `Validate()` (R-DTO-VALIDATE-001).
6. GATES GLOBAIS (Tarefa 12.0 + por tarefa): `go build ./...`; `go test` dos módulos tocados (incl.
   `-tags=integration` onde aplicável); e os gates de fronteira — devem retornar vazio:
   - `grep -rn "buildAutoSplits" internal/agent --include="*.go" | grep -v _test`
   - SQL direto em consumers/adapters do agente
   - comentários proibidos em tools/workflow/consumers do agente
   - `grep -rn "OnboardingLLMEnabled" internal/ configs/` (flag removida — Parte 1)
   Parte 1 (auto-start, LLM mandatório, allowlist OpenRouter) JÁ está implementada — validar
   integridade, NÃO recriar.
7. ROBUSTEZ: idempotência da saudação (welcome_sent_at + AgentDecision por event_id); conclusão
   determinística (state=active + completed_at + evento, no mesmo uow); WorkingMemory assíncrona via
   consumer de `onboarding.completed`; cap de retry + dead-letter (`max_attempts` + alerta
   `outbox_dead_letter_total`); falha de LLM = retry seguro sem corromper estado.

PROCEDIMENTO
- Rode o pré-voo da skill (hook `pre-execute-all-tasks.sh`, `ai-spec skills --verify`,
  `ai-spec check-spec-drift .specs/prd-onboarding-v2/tasks.md`). Se algum gate falhar, retorne
  `blocked`/`needs_input` com o stderr — não degrade silenciosamente.
- Execute as waves respeitando dependências e `Paralelizável`. Aguarde TODOS da wave antes de decidir
  halt. Cada subagent retorna o YAML `{status, report_path, summary}`; valide os 4 passos
  (formato, status canônico, evidência física em `<id>_execution_report.md`, consistência tasks.md).
- Ao final, gere `.specs/prd-onboarding-v2/_orchestration_report.md` com snapshot inicial vs final,
  tabela de tarefas executadas/puladas, waves, gates rodados e próximos passos.

CRITÉRIO DE ENCERRAMENTO
- `done` somente se as 13 tarefas estiverem `done` com evidência e os gates globais (Tarefa 12.0)
  retornarem OK. Caso contrário, `partial`/`blocked`/`failed` com o motivo exato e a primeira tarefa
  que travou. NÃO marque `done` com qualquer DoD/critério de aceite não comprovado.
```

---

## Ordem de execução esperada (DAG de tasks.md)

| Wave | Tarefas (paralelas) | Módulo / foco |
|------|---------------------|----------------|
| 1 | 1.0, 2.0, 4.0, 13.0 | onboarding domain (perfil, payload), budgets SuggestAllocation, card closing_day-only |
| 2 | 3.0 | onboarding repository (JSON + drift) — dep 2.0 |
| 3 | 5.0, 6.0, 7.0 | onboarding usecases (split, lifecycle, card) |
| 4 | 8.0 | agent RunOnboardingTurn + adapters |
| 5 | 9.0 | agent tools + scripts |
| 6 | 10.0, 11.0 | agent WM consumer + greeting hardening |
| 7 | 12.0 | validação integração + E2E + gates |

> Observação: a numeração não reflete ordem topológica pura — 13.0 (card) roda na wave 1 por ser
> pré-requisito de 7.0. As dependências em tasks.md são a fonte da verdade.

## Pré-requisitos operacionais
- Binário `ai-spec` no PATH; hooks de governança instalados (`.agents/hooks/` via `ai-spec install`).
- `check-spec-drift .specs/prd-onboarding-v2` deve retornar "sem drift" antes de iniciar (já validado
  em 2026-06-23).
- Banco Postgres + testcontainers disponíveis para os testes de integração da Tarefa 12.0.

## Evidência mínima por tarefa (no `<id>_execution_report.md`)
- Saída de `go build` e `go test` dos pacotes tocados (verde).
- Saída dos greps de gate aplicáveis (vazio onde exigido).
- Checklist de DoD marcado com referência à evidência.
- Confirmação dos contratos de validação relevantes à tarefa.
