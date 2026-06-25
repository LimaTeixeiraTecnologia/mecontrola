# Prompt Mandatório — Executar TODAS as tasks da Consolidação Mastra/Workflows (MVP robusto, production-ready)

- **Data:** 2026-06-24
- **Skill obrigatória:** `execute-all-tasks` (`.github/skills/execute-all-tasks/` / `.agents/skills/execute-all-tasks/`)
- **Bundle de tasks:** `.specs/prd-arquitetura-agente-consolidacao/` (tasks.md + task-1.0…task-8.0)
- **Plano-fonte:** `docs/plans/2026_06_24_arquitetura_agente_mastra_workflows_bounded_contexts.md`
- **Skills Go obrigatórias por task de código:** `go-implementation` (sempre) + `mastra` (tasks que tocam `internal/agent`)

> Uso: cole o bloco **PROMPT PRONTO PARA USO** abaixo como mensagem inicial de uma sessão dedicada. Ele é inegociável: a sessão só pode reportar `done` com evidências que satisfaçam DoD + Critérios de Aceite de cada task.

---

## PROMPT PRONTO PARA USO

```text
Use a skill execute-all-tasks para executar TODAS as tarefas do bundle
.specs/prd-arquitetura-agente-consolidacao/ (tasks.md + task-1.0…task-8.0),
derivado do plano docs/plans/2026_06_24_arquitetura_agente_mastra_workflows_bounded_contexts.md.

OBJETIVO INEGOCIÁVEL
Entregar um MVP robusto e production-ready/production-proof da consolidação da
arquitetura do agente (kernel como caminho único, remoção de legacy, HITL sempre-on,
registry canônica, testes de regressão e a nova tool de leitura). Não há entrega
parcial aceitável: cada task só é `done` quando seu DoD e seus Critérios de Aceite
forem atendidos FIELMENTE e com EVIDÊNCIA executada (não apenas descrita).

REGRAS DE GOVERNANÇA (HARD — não flexibilizar por ferramenta, pressa ou conveniência)
- Ler AGENTS.md no início e respeitar TODAS as regras .claude/rules/ aplicáveis:
  R-AGENT-WF-001 (.1/.2/.3/.4/.5/.6/.7 + addenda .6-A/.7-A/.8-A), R-WF-KERNEL-001
  (.1..7), R-ADAPTER-001 (zero comentários, adapters finos), R-TESTING-001
  (testify/suite whitebox), R-DTO-VALIDATE-001 quando aplicável.
- Toda alteração Go: carregar a skill go-implementation e executar Etapas 1–5 +
  checklist R0–R7 ANTES de editar. Tasks que tocam internal/agent: carregar também
  a skill mastra. Verificar a versão em go.mod antes de usar APIs novas.
- Zero comentários em .go de produção (exceto //go:, //nolint: justificado, // Code generated).
- DMMF state-as-type: AwaitingKind/TransactionKind/RunStatus/ToolOutcome/AwaitingApproval/
  OperationKind permanecem tipos fechados; nunca string livre.
- Não adicionar case intent.Kind novo em daily_ledger_agent.go — roteamento via registry.
- Kernel (internal/platform/workflow) sem import de domínio, sem regra/SQL/branching de
  domínio, sem LLM. Snapshot é fonte única no resume (merge-patch RFC 7386).

ORDEM E DEPENDÊNCIAS (respeitar o DAG de tasks.md — halt-first)
- 1.0 é pré-requisito de 2.0/3.0/4.0/5.0 e de 6.0. Sem 1.0 (kernel sempre-on,
  confirmEngine obrigatório), remover legacy REGRIDE — não pular.
- Cadeia serial: 1.0 → 2.0 → 3.0 → 4.0 → 5.0 (mesmos arquivos de serviço; não paralelizar).
- 6.0 (module.go + entrypoints) e 7.0 (domain/intent) podem rodar em paralelo entre si,
  após suas dependências (6.0 depende de 1.0; 7.0 não tem dependência).
- 8.0 só depois de 5.0 (registry canônica) e 7.0 (teste exaustivo a estender).
- ATENÇÃO ESCOPO Task 4.0: NÃO remover dispatchWriteDestructive/wireBudgetCommitGate
  (são o caminho canônico). Remover apenas o ramo de bypass (confirmEngine == nil) e o
  duplo caminho de execução dos tools de delete/edit/card-delete/budget-commit.

PROTOCOLO POR TASK (obrigatório, sem atalho)
1. Ler o task-X.0-*.md correspondente + o plano-fonte na seção citada + ADRs referenciados.
2. Implementar a responsabilidade única da task (não misturar preocupações de outra task).
3. Rodar TODOS os gates do bloco "Critérios de Aceite (gates executáveis)" do task file e
   colar a SAÍDA REAL de cada comando. Todo gate deve retornar OK/vazio/verde.
4. Criar e EXECUTAR os testes da task (unitários + integração quando aplicável) no padrão
   R-TESTING-001; colar a saída de `go test`. Para 7.0, provar localmente que o teste de
   paridade FALHA ao injetar divergência schema↔kind e volta a passar após reverter.
5. Validação proporcional ao risco no fim de cada task:
   go build ./... && go vet ./internal/agent/... && go test ./internal/agent/... ./internal/platform/workflow/...
   e os gates de regra: kernel sem import de domínio, zero comentários, switch não cresceu.
6. Marcar a task `done` em tasks.md SOMENTE com DoD 100% atendido e evidências coladas.
   Se um gate falhar, a task fica `failed`/`blocked` com o motivo — NÃO marcar done.

EVIDÊNCIA E RELATÓRIO (política de evidência — governance.md)
- Para cada task, registrar: arquivos alterados, gates executados (com saída real),
  testes executados (com saída real), riscos residuais e suposições.
- Não aprovar solução com lacuna crítica conhecida. Sem falso positivo: "production-ready"
  significa não-falso-positivo verificável, não escopo expandido.
- Ao final, relatório consolidado: 8/8 tasks done, suíte completa verde
  (go build ./... && go test ./...), e a lista de gates de regra R-* todos OK.

NÃO-OBJETIVOS (não fazer)
- Não introduzir RAG/vetor/fila nova, multi-agente, nem campo `plan` multi-tool (fora do MVP).
- Não refatorar além do escopo das 8 tasks. Não relaxar nenhuma regra.
- Não commitar nem abrir PR sem pedido explícito do usuário.

Comece resolvendo a ordem de execução a partir de tasks.md, confirme o DAG e execute
1.0 primeiro. Halt-first: ao primeiro gate vermelho irreparável, pare e reporte.
```

---

## Critérios de aceite do próprio run (como saber que o prompt foi cumprido)

1. `tasks.md` com as 8 tasks em `done` e evidências por task (gates + `go test` colados).
2. Suíte global verde: `go build ./...` e `go test ./...`.
3. Gates de regra todos OK, em especial:
   - Kernel sem import de domínio (R-WF-KERNEL-001.1) e merge-patch no resume preservado (.7).
   - Zero comentários em Go de produção (R-ADAPTER-001.1).
   - Switch de `daily_ledger_agent.go` não cresceu (R-AGENT-WF-001.1).
   - `AwaitingApproval`/`OperationKind`/`ToolOutcome`/`RunStatus` sem string solta (R-AGENT-WF-001.3/.7-A).
   - Nenhuma operação destrutiva executa sem `confirm_gate` (ADR-002/003).
4. `parity_test.go` adaptado e verde; novo teste exaustivo de `intent.Kind` verde (22 kinds após 8.0).
5. Relatório final com riscos residuais e suposições — sem lacuna crítica conhecida.

## Referências
- Bundle: `.specs/prd-arquitetura-agente-consolidacao/tasks.md`
- Plano-fonte: `docs/plans/2026_06_24_arquitetura_agente_mastra_workflows_bounded_contexts.md`
- Regras: `.claude/rules/{agent-workflows-tools,workflow-kernel,go-adapters,go-testing,governance}.md`
- ADRs: `.specs/prd-agent-platform-evolution/adr-00{1,2,3,4}-*.md`
