# PROMPT MANDATÓRIO — Executar TODAS as tarefas do Onboarding Conversacional

> **Uso:** cole o bloco em "PROMPT PRONTO PARA USO" (abaixo) como mensagem inicial de uma sessão de execução. Este documento é o **contrato de execução**: inegociável, sem flexibilização, MVP robusto e production-ready/proof, com fidelidade ao documento oficial e DoD/critérios de aceite atendidos **com evidência**.
>
> **Data:** 2026-06-25 · **Feature:** `.specs/prd-onboarding-conversacional/` · **Go:** 1.26.4

---

## PROMPT PRONTO PARA USO

```text
TAREFA: Implementar e ENTREGAR, de ponta a ponta, TODAS as 9 tarefas de
.specs/prd-onboarding-conversacional/ (task-1.0 … task-9.0), respeitando o DAG de
dependências, com MVP robusto e production-ready/proof. Inegociável e sem flexibilização.

MODO DE EXECUÇÃO:
- Use a skill `execute-all-tasks` para orquestrar o PRD inteiro (spawn de subagente fresh por
  tarefa, isolando contexto), respeitando o grafo de dependências de tasks.md, halt-first e
  retomada idempotente. Em paralelizáveis declaradas (3.0∥4.0, 7.0∥8.0), paralelize só quando as
  dependências permitirem.
- Alternativa equivalente: `execute-task` tarefa a tarefa na ordem 1.0 → 2.0 → (3.0∥4.0) → 5.0 →
  6.0 → (7.0∥8.0) → 9.0. NÃO pular tarefa, NÃO marcar `done` sem evidência.

FONTE DA VERDADE (ler na íntegra antes de cada tarefa):
- docs/oficial/2026_06_24_mecontrola_oficial.md — Cap. 07/08/10/11 (o produto é fiel a isto).
- .specs/prd-onboarding-conversacional/prd.md (RF-01..RF-30).
- .specs/prd-onboarding-conversacional/techspec.md (interfaces, modelos, riscos R1..R5).
- .specs/prd-onboarding-conversacional/adr-001..004 (decisões inegociáveis).
- .specs/prd-onboarding-conversacional/mapeamento-verbatim-onboarding.md (1:1 oficial×código).
- .specs/prd-onboarding-conversacional/tasks.md + task-1.0..9.0 (DoD por tarefa).
- AGENTS.md e CLAUDE.md (governança canônica).

SKILLS OBRIGATÓRIAS:
- `go-implementation` é MANDATÓRIA para TODA edição de Go: carregar SKILL.md e executar as Etapas
  1–5 (Regras Estritas R0–R7 + Checklist de Validação). Verificar a versão em go.mod (1.26.4)
  antes de usar APIs/dependências.
- `mastra` é OBRIGATÓRIA nas tarefas que tocam internal/agent (4.0, 5.0, 6.0, 7.0, 9.0).
- `agent-governance` é auto-carregada.

REGRAS HARD — INEGOCIÁVEIS (qualquer violação BLOQUEIA o `done`):
- R-AGENT-WF-001: roteamento Workflow→Tool/Step→binding→usecase; PROIBIDO novo `case intent.Kind`
  de domínio no switch de daily_ledger_agent.go; Tool/Step finos (sem regra/SQL/branching);
  ToolOutcome/RunStatus/AwaitingKind/AwaitingApproval/OnboardingPhase como TIPOS FECHADOS; Run
  auditável; resume ANTES do parse; LLM só na cadeia de onboarding (exceção .4), nunca em domínio.
- R-WF-KERNEL-001: internal/platform/workflow permanece GENÉRICO — proibido import de domínio,
  regra/branching/LLM/SQL no kernel; estados fechados; resume por merge-patch. Steps/LLM/binding
  vivem em internal/agent.
- R-ADAPTER-001.1: ZERO comentários em .go de produção (exceções: //go:, //nolint:, // Code generated).
- R-ADAPTER-001.2: adapters/consumers/jobs/handlers finos `adapter→usecase`; sem SQL direto.
- R-DTO-VALIDATE-001: todo input DTO com Validate() (errors.Join, campo nomeado), chamado após o span.
- R-TESTING-001: testes de usecase em testify/suite, whitebox, fake.NewProvider(), mocks por IIFE.
- DMMF (Wlaschin): state-as-type, smart constructors, Decide* PURO (sem IO/context/time.Now),
  pipeline parse→validate→decide→persist→publish. PROIBIDO Result/Either custom, currying, DSL,
  mônada. PROIBIDO abstrair tempo (usar time.Now().UTC() inline) e `var _ Interface = (*T)(nil)`.
- Idempotência por event_id em todo consumer de outbox; inbound idempotente por messageID.

FIDELIDADE AO OFICIAL (Cap. 08, ETAPA 1→8) — sem flexibilizar:
- 8 etapas distintas e na ordem do Cap. 07; SEM "Etapa X/4".
- ETAPA 1 boas-vindas + handshake "Vamos começar?" (aguarda "Sim"); não pede objetivo nem
  apresenta categorias no welcome.
- ETAPA 4 cartão coleta SÓ apelido + dia de VENCIMENTO; fechamento DERIVADO por offset
  configurável (ADR-003); nunca limite/banco/bandeira/fechamento perguntado.
- ETAPA 5 apresenta as 5 categorias oficiais + "Faz sentido?".
- ETAPA 6 valor por categoria, UMA A UMA; usuário SEMPRE informa; SEM auto-sugestão.
- ETAPA 7 resumo com valor + percentual + "Está tudo certo?"; gate HITL durável + correção
  guiada por LLM (reusa primitivos do kernel).
- ETAPA 8 conclui SEM exigir primeira transação (remove FirstTxRecorded de IsReadyToComplete);
  mensagem com exemplos; emite onboarding.completed.
- Comando diário durante o onboarding → redireciona gentilmente, NÃO registra (OutcomeDeferred).
- Substituição COMPLETA do legado: remover run_onboarding_turn.go, OnbPhaseFirstTx, auto-sugestão,
  headers "Etapa X/4", schema onboarding_first_tx (ADR-001).

DEFINITION OF DONE (por tarefa — TODOS obrigatórios):
1. Todos os <requirements> e Subtarefas do task-X.Y atendidos; todos os RF cobertos pela tarefa
   implementados e fiéis ao oficial.
2. Critérios de Sucesso do task file atendidos, demonstrados com EVIDÊNCIA (saída de comando/teste).
3. Testes da tarefa criados E executados (unitários sempre; integração/e2e onde o task exigir),
   PASSANDO. Sem teste = sem done.
4. `task build` OK · `task lint` OK · `task test` OK (e `task <modulo>:integration` quando a
   tarefa tiver fronteira de IO). `task security` sem regressão.
5. Gates de regra (abaixo) retornam VAZIO/OK.
6. Relatório de execução salvo em .specs/prd-onboarding-conversacional/task-X.Y_execution_report.md
   com: arquivos alterados, comandos rodados + saída, RF cobertos, riscos residuais, suposições.
7. Status em tasks.md atualizado para `done` somente após 1–6.

CRITÉRIOS DE ACEITE GLOBAIS (com evidência ao final):
- Jornada completa das 8 etapas (e2e) passando, fiel ao Cap. 08.
- Bordas: correção no resumo, "não uso cartão", desvio de comando diário (sem registrar),
  retomada após interrupção (resume durável), migração-reset de sessão legada.
- Propagação idempotente: splits_calculated→orçamento ativo; card_registered→cartão com
  vencimento+fechamento derivado; completed→working memory.
- Zero referência a "Etapa X/4"/first_tx/auto-sugestão no código.
- Cobertura: `ai-spec check-spec-drift .specs/prd-onboarding-conversacional` sem drift.

GATES DE VERIFICAÇÃO OBRIGATÓRIOS (devem retornar vazio/OK antes de cada `done`):
# zero comentários em Go de produção
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" --exclude="*_test.go" \
  "^[[:space:]]*//" internal/ configs/ cmd/ | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL comentários" || echo OK
# kernel genérico (sem domínio)
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "internal/agent\|internal/transactions\|internal/billing\|internal/identity" \
  internal/platform/workflow/ && echo "FAIL import domínio no kernel" || echo OK
# sem SQL direto nem LLM no kernel
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|openai\|anthropic\|ParseInbound" \
  internal/platform/workflow/ | grep -v "infrastructure/postgres" && echo "FAIL kernel" || echo OK
# switch de domínio não cresce em daily_ledger_agent.go
f=$(find internal/agent -name daily_ledger_agent.go ! -name "*_test.go"); \
  [ -n "$f" ] && c=$(grep -cE "^[[:space:]]*case intent\.Kind" "$f"); \
  [ "${c:-0}" -gt 1 ] && echo "FAIL switch cresceu" || echo OK
# sem SQL em tools/workflow do agent
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
  internal/agent/application/tools/ internal/agent/application/workflow/ 2>/dev/null \
  && echo "FAIL SQL em tool/workflow" || echo OK

PROIBIÇÕES (qualquer ocorrência invalida a entrega):
- Flexibilizar qualquer regra/etapa/mensagem por conveniência, ferramenta ou prazo.
- Marcar `done` sem testes executados e evidência anexada (falso positivo é inaceitável).
- Introduzir comentários em .go, SQL em adapter, regra de domínio em Tool/Step ou no kernel.
- Inventar package/handler/evento/tool/rota/consumer/memória inexistente sem verificar o código.
- Git destrutivo ou push/PR sem pedido explícito do usuário.
- Remover o legado (6.0) ANTES de 4.0 e 5.0 prontas e verdes.

PROTOCOLO POR TAREFA:
1. Ler prd.md + techspec.md + ADRs pertinentes + o task-X.Y.md.
2. Carregar go-implementation (Etapas 1–5) e, se internal/agent, mastra.
3. Modelar respeitando fronteiras; implementar adaptando exemplos ao código real (nunca copiar).
4. Criar e rodar testes; rodar build/lint/test (+integração quando aplicável) e os gates de regra.
5. Escrever o task-X.Y_execution_report.md com evidências; atualizar tasks.md para `done`.
6. Só então avançar para a próxima tarefa elegível no DAG.

ENTREGA FINAL:
- Relatório consolidado .specs/prd-onboarding-conversacional/_orchestration_report.md com o estado
  de cada tarefa, evidências dos critérios de aceite globais e riscos residuais.
- check-spec-drift sem drift; build/lint/test/integração/e2e verdes; jornada das 8 etapas provada.
- NÃO commitar/push/PR a menos que o usuário peça explicitamente.

Em caso de ambiguidade material não coberta por prd/techspec/ADR/oficial: PARAR e perguntar em
múltipla escolha (recomendação na 1ª opção). Não inventar comportamento.
```

---

## Notas de uso (fora do prompt)

- **Ordem/DAG:** `1.0 → 2.0 → (3.0 ∥ 4.0) → 5.0 → 6.0 → (7.0 ∥ 8.0) → 9.0`. A `1.0` (domínio puro/tipos fechados) é fundação e desbloqueia tudo; a remoção do legado (`6.0`) só após `4.0`+`5.0`.
- **Comandos de validação reais:** `task build`, `task lint`, `task test`, `task security`, `task check`/`task ci`; integração por módulo (ex.: `task card:integration`). Go 1.26.4 (`go.mod`).
- **Evidência = contrato:** cada tarefa só fecha com `task-X.Y_execution_report.md` (arquivos, comandos+saída, RF cobertos, riscos, suposições) — política de evidência de `.claude/rules/governance.md`.
- **Rastreabilidade:** `tasks.md` carrega `spec-hash-prd`/`spec-hash-techspec`; rodar `ai-spec check-spec-drift .specs/prd-onboarding-conversacional` ao final.
- **Decisões inegociáveis já fechadas:** ADR-001 (workflow no kernel + substituição completa + conclusão sem 1ª tx), ADR-002 (`OnboardingPhase` fechado + migração-reset), ADR-003 (cartão só vencimento + fechamento derivado), ADR-004 (gate HITL + desvio diário reusando primitivos do kernel).
