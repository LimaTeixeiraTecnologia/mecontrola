# Prompt Mandatório — Executar TODAS as tasks do `prd-workflow-kernel` (MVP robusto, production-proof)

- **Data:** 2026-06-24
- **Alvo:** `.specs/prd-workflow-kernel/` (PRD + techspec + 5 ADRs + tasks.md + task-1.0…9.0)
- **Skill orquestradora:** `execute-all-tasks` (governance) — espelhada em `.github/skills/execute-all-tasks/`, `.agents/skills/execute-all-tasks/`, `.claude/skills/execute-all-tasks/`
- **Skills obrigatórias por tarefa:** `go-implementation` (toda edição `.go`, inegociável), `mastra` (tasks 6.0–9.0 em `internal/agent`), `agent-governance` (auto)
- **Pré-condição de governança (gate, RF-29):** task 1.0 redige `R-WF-KERNEL-001` + addendum `R-AGENT-WF-001.6/.8` ANTES de qualquer `.go` do kernel
- **Cobertura:** RF-01…RF-32 (validada por `ai-spec check-spec-drift` — sem drift)
- **Caminho crítico:** 1.0 → 2.0 → 3.0 → 4.0 → 5.0 → 8.0 → 9.0 · **Paralelo seguro:** 2.0 ↔ 6.0

---

## ⬇️ PROMPT PRONTO PARA USO (copie e cole na sessão de execução)

```text
MISSÃO (inegociável): executar TODAS as tarefas do PRD `prd-workflow-kernel` até o fim, entregando
um MVP ROBUSTO e PRODUCTION-READY/PROOF, com 0 gaps, 0 lacunas e 0 falso positivo. Atender FIELMENTE
o Definition of Done (DoD) e os Critérios de Aceite de CADA tarefa, com EVIDÊNCIA verificável.

INVOCAÇÃO CANÔNICA
- Invoque a skill `execute-all-tasks` com o slug: workflow-kernel
- O orquestrador roda o pré-voo (hook `pre-execute-all-tasks.sh`, `ai-spec skills --verify`,
  `ai-spec check-spec-drift .specs/prd-workflow-kernel/tasks.md`), constrói o DAG a partir de
  `.specs/prd-workflow-kernel/tasks.md` e spawna um subagent FRESH por tarefa via `execute-task`.
- NUNCA executar `execute-task` inline no orquestrador. Respeitar o contrato YAML estrito
  (`status`, `report_path`, `summary`) e a cadeia de validação (evidência física + consistência
  de tasks.md). Halt-first: qualquer tarefa ≠ done para a wave, gera relatório e encerra.

ORDEM E PARALELISMO (respeitar tasks.md, sem exceção)
- 1.0 é GATE bloqueante: governança (`R-WF-KERNEL-001` + addendum `R-AGENT-WF-001.6/.8`) ANTES de
  qualquer código do kernel. Não escrever `.go` do kernel enquanto 1.0 não estiver `done`.
- Caminho crítico: 1.0 → 2.0 → 3.0 → 4.0 → 5.0 → 8.0 → 9.0.
- Paralelizar SOMENTE 2.0 ↔ 6.0 (kernel de plataforma vs rename no agent — áreas disjuntas) e apenas
  se o tool suportar spawn nativo; senão, degradar para sequencial e registrar no relatório.

REGRAS DE IMPLEMENTAÇÃO (HARD — não flexibilizar por ferramenta, hook, conveniência ou deadline)
- Para TODA edição `.go`: carregar `go-implementation` e cumprir Etapas 1–5 + checklist R0–R7.
- Tasks 6.0–9.0 (em `internal/agent`): carregar TAMBÉM `mastra` (mapa Mastra→Go, R-AGENT-WF-001).
- Zero comentários em `.go` de produção (R-ADAPTER-001.1); exceções só `//go:`, `//nolint:`,
  `// Code generated`.
- Sem `init()` (R0), sem `panic` em produção (R5.12), `context.Context` em toda fronteira de IO (R6),
  interface no consumidor (a porta `Store` mora no kernel/consumidor), `errors.Join` +
  `fmt.Errorf("ctx: %w", err)` (R7). Sem abstração de tempo: `time.Now().UTC()` inline.
  `defer func(){ _ = rows.Close() }()` para rows.
- Kernel (`internal/platform/workflow`): SEM import de domínio (`intent`/`agent`/`transactions`),
  SEM regra/SQL/branching de domínio, SEM LLM; estados como TIPOS FECHADOS
  (`RunStatus`/`StepStatus`/`SuspendReason`); `StepOutput[S]` NÃO é mônada de erro (proibido
  `Result[T,E]`, currying, DSL de pipeline, monads).
- Agent: NENHUM novo `case intent.Kind` no switch de `daily_ledger_agent.go`; comportamento novo só
  via Workflow/Tool/steps no seam; LLM apenas no `ParseInbound`; `pendingexpense.Draft` como estado do
  run; WriteGuard preservado 1:1 (Authorize→Replay→Policy→Audit, ordem e short-circuit idênticos).
- Persistência: tabelas genéricas `workflow_runs`/`workflow_steps` (migração 000019) via uow; lock
  otimista por `version` (CAS) + índice parcial único de run ativo; snapshot SÓ para escrita/
  suspensível (leitura pura in-process); falha terminal determinística (run `failed`, sem retry
  infinito); housekeeping com retenção configurável. Sem tx cross-módulo.
- Cutover por feature flag `WORKFLOW_KERNEL_TRANSACTIONS_WRITE_ENABLED` (default OFF), com fallback ao
  caminho atual e drenagem do draft legado em `agent_sessions`.

DoD E CRITÉRIOS DE ACEITE COM EVIDÊNCIA (obrigatório por tarefa — sem isso, NÃO marcar `done`)
Cada subagent, via `execute-task`, DEVE produzir `.specs/prd-workflow-kernel/<id>_execution_report.md`
contendo evidência concreta (não apenas afirmação):
  1. Subtarefas e Critérios de Sucesso do task file: cada item marcado com a prova correspondente.
  2. Testes da tarefa CRIADOS e EXECUTADOS: colar a saída real (unit; integration `//go:build
     integration` com testcontainers onde a techspec exige — tasks 4.0/5.0). Verde obrigatório.
  3. Gates executados com saída real: conforme o escopo da tarefa —
     - R-ADAPTER-001 (grep zero comentários / zero SQL em adapter),
     - R-AGENT-WF-001 (sem novo case / LLM só no parse / estados fechados / pending step salvo),
     - R-WF-KERNEL-001 (kernel sem import de domínio / sem SQL de domínio / labels sem
       user_id|correlation_key|category_id),
     - R-TESTING-001 (testify/suite whitebox, fake.NewProvider, IIFE) nos use cases,
     - checklist R0–R7 de go-implementation.
  4. Arquivos alterados (diff/SHA), validações executadas, riscos residuais e suposições.
NÃO marcar `done` com qualquer gate vermelho, teste faltante/ignorado, ou critério sem evidência.
"Production-ready/proof sem falso positivo" = sem afirmar conclusão sem prova reproduzível.

ACEITE FINAL DA ENTREGA (task 9.0 — não regressão production-proof)
- Suíte de PARIDADE dirigida por tabela: para os mesmos inputs, `Reply`/`Outcome`/`Kind` IDÊNTICOS
  entre flag OFF (caminho atual) e flag ON (kernel) em TODOS os cenários: auto-log,
  ambiguous→choice→resume, needs_confirm→confirm/cancel→resume, authz_denied, replay, policy_blocked,
  usecase_error, missing_resolver. Divergência > 0 ⇒ falha (não mascarar).
- E2E inbound→reply do record-expense (consumer fake, flag ON).
- Todos os gates R-* + R0–R7 verdes com evidência. Flag permanece default OFF no merge.

RELATÓRIO E ENCERRAMENTO
- Ao final, o orquestrador consolida `.specs/prd-workflow-kernel/_orchestration_report.md`
  (snapshot inicial vs final, tabela de executadas/puladas, waves, próximos passos).
- Status final: `done` só se as 9 tarefas estiverem `done` com evidência; senão `partial`/`failed`
  com a tarefa bloqueante e o motivo. Para `partial`/`failed`, NÃO inventar conclusão — reportar o
  gap exato e o próximo passo. Não relaxar nenhuma regra hard para "fechar" a execução.

GIT/PUBLICAÇÃO
- Não commitar nem fazer push sem pedido explícito. Se for commitar, branch a partir de `main`,
  Conventional Commits, e a trailer de co-autoria padrão do repositório.
```

---

## Apêndice A — Checklist de DoD por tarefa (referência rápida)

| Tarefa | Aceite-chave (resumo — fonte: `task-<id>-*.md`) | Evidência mínima |
|--------|--------------------------------------------------|------------------|
| 1.0 | `R-WF-KERNEL-001` + addendum `R-AGENT-WF-001.6/.8`; gates grep retornam vazio | saída dos gates grep; sem `.go` de kernel |
| 2.0 | Kernel puro compila sem import de domínio; combinadores testados; Parallel cancela sem leak | `go test` puro verde; grep de imports |
| 3.0 | Engine Start/Resume/suspend/retry/falha terminal; `Durable=false` não persiste; métricas/spans | `go test` com fake; cardinalidade ok |
| 4.0 | Migração 000019 up/down; CAS rejeita versão velha; resume após restart; 2 resumes → 1 vence | integration tests (testcontainers) verdes |
| 5.0 | Housekeeping purga concluídos e preserva ativos; config valida defaults/erros | `go test` + config_test |
| 6.0 | `IntentWorkflow`/`IntentRegistry` em uso; suíte do agent verde sem mudança semântica | `go test ./internal/agent/...` |
| 7.0 | record-expense multi-step (guard 1:1 + branch + suspend/resume sem LLM); passos isolados | `go test` testify/suite; gates R-AGENT-WF-001 |
| 8.0 | Flag OFF = comportamento atual; ON = kernel + fallback; housekeeping registrado; zero novo case | testes de integração do agent + grep de case |
| 9.0 | Paridade OFF vs ON com 0 divergência; E2E; gates R-* + R0–R7 verdes | tabela de paridade + saída dos gates |

## Apêndice B — Artefatos de evidência esperados ao final

- `.specs/prd-workflow-kernel/1.0_execution_report.md` … `9.0_execution_report.md` (um por tarefa)
- `.specs/prd-workflow-kernel/_orchestration_report.md` (rollup do orquestrador)
- Saídas reais de `go test` (unit + `-tags integration`) e dos gates `R-*`/`R0–R7` coladas nos reports
- `tasks.md` com status atualizado para `done` por tarefa concluída (escrito pelos subagents via
  `execute-task`, nunca pelo orquestrador)

## Apêndice C — Pré-condições operacionais (validar antes de disparar)

- `ai-spec` no PATH (`command -v ai-spec`); hooks de governança presentes na cascata
  (`.agents/hooks/` → `.claude/hooks/` → `.github/hooks/`), instalados via `ai-spec install`.
- `check-spec-drift .specs/prd-workflow-kernel` deve retornar "sem drift" (já validado em 2026-06-24).
- Postgres disponível para os integration tests (tasks 4.0/5.0) via testcontainers (Docker ativo).
