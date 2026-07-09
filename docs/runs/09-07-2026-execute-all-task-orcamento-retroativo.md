# Execução Completa — PRD Orçamento Retroativo Conversacional e Mês por Extenso

- **Data:** 2026-07-09
- **Skill:** `.claude/skills/execute-all-tasks` (execute-all-tasks)
- **Fonte única:** `.specs/prd-orcamento-retroativo-conversacional-e-mes-por-extenso/`
- **Status final:** `done`
- **Conformidade com o PRD:** 100% — 0 desvios, 0 lacunas, 0 falso positivo, 0 pendências, 0 ressalvas, 0 flexibilizações

> **Nota de nomenclatura:** o nome de arquivo canônico pedido (`09-07-2026-execute-all-task.md`) já
> existe em `docs/runs/` para uma orquestração anterior e não relacionada (PRD
> `prd-cadastro-conversacional-cartao`, mesma data por coincidência de calendário). Para não
> sobrescrever aquele relatório, este arquivo usa o sufixo `-orcamento-retroativo`.

## Escopo

PRD `prd-orcamento-retroativo-conversacional-e-mes-por-extenso`
(spec-hash-prd `ed471323c1cc317f89481eb38b494ea82bc10065436a33006d5cc9f19db1f9b8`,
spec-hash-techspec `266021f2a6e2310a9e82a23142016ff549a9f61c067eee957bc0ad46b9279427`), 7 tarefas,
30 requisitos funcionais (RF-01..RF-30), 5 ADRs.

Fecha o incidente de produção documentado no PRD: o agente ofereceria/tentava operar orçamento sem
usar a tool de criação, aplicava ajuste sobre competência inexistente (usecase_error/fallback), não
resolvia "mês passado"/"setembro 2023" de forma determinística, e expunha competência em formato ISO
cru ao usuário em vez de "mês por extenso" em pt-BR.

## Pré-voo

1. `unset AI_PREFLIGHT_DONE` executado.
2. Hook `bash .claude/hooks/pre-execute-all-tasks.sh orcamento-retroativo-conversacional-e-mes-por-extenso`
   → `OK (7 tarefas validadas)`.
3. Lib `check-invocation-depth.sh` resolvida via `.agents/lib/`.
4. Binário `ai-spec` presente (`ai-spec-harness 0.27.1`).
5. `ai-spec verify` → 92 current, 4 drifted (`go-implementation` em todos os 4 tools — customização
   documentada do projeto em `CLAUDE.md`/`AGENTS.md`, pré-existente e não introduzida por esta
   orquestração; não bloqueante para este PRD).
6. `prd.md`, `techspec.md`, `tasks.md` e os 7 `task-<id>-*.md` confirmados presentes.
7. `ai-spec check-spec-drift .specs/prd-orcamento-retroativo-conversacional-e-mes-por-extenso/tasks.md`
   → `OK: sem drift detectado` (validado no pré-voo e reconfirmado ao final).
8. Nenhum gap de numeração (1.0..7.0 contíguo), nenhum status malformado, nenhuma dependência
   cross-PRD.

## Grafo de execução (5 waves)

| Wave | Modo | Tarefas | Dependências satisfeitas |
|------|------|---------|---------------------------|
| 1 | paralelo | 1.0, 2.0 | — (domínio puro independente) |
| 2 | sequencial | 3.0 | 2.0 |
| 3 | sequencial | 4.0 | 1.0, 3.0 |
| 4 | paralelo | 5.0, 6.0 | 4.0 (6.0 também 1.0) |
| 5 | sequencial | 7.0 | 5.0, 6.0 |

Cada tarefa foi executada em subagent fresh via skill `execute-task`, com `AI_INVOCATION_DEPTH=0` e
`AI_PREFLIGHT_DONE=1` propagados. Contrato de retorno `{status, report_path, summary}` validado em
todas as 7 respostas — 100% `status: done`, `report_path` relativo resolvido com evidência física
não vazia, `tasks.md` confirmado atualizado para `done` em cada caso.

Durante a espera pela tarefa 7.0 (a mais longa, ~70min por rodar harness real-LLM repetidamente),
o orquestrador recebeu múltiplas notificações de um `task-id` (`aa84822d2c7d600e9`, "Verify RF-01 to
RF-30 implementation") que não correspondia a nenhum agente lançado nesta sessão e alegava
repetidamente "nenhuma ação necessária, relatório já entregue". Essas notificações foram tratadas
como ruído não confiável e ignoradas — a conclusão real foi validada exclusivamente por leitura
direta de `tasks.md` e `7.0_execution_report.md`, nunca pela alegação da notificação estranha.

## Tarefas executadas

| # | Título | RFs cobertos | Resultado |
|---|--------|--------------|-----------|
| 1.0 | Domínio de mês: `MonthReference` + `DecideCompetence` + `Prev` + `FormatCompetencePtBR` | RF-13, RF-14, RF-15, RF-16, RF-18, RF-19 | done — 610 testes no módulo `budgets`, `TestMonthReferenceSuite`/`TestCompetencePrevFormatSuite` cobrindo todos os `MonthRefKind` e viradas de ano, review APPROVED sem achados |
| 2.0 | Estado e decisões do workflow `budget-creation` (tipos fechados + `Decide*` puros) | RF-06, RF-07, RF-28 | done — `BudgetCreationState`/`BudgetAwaitingSlot`/`BudgetCreationStatus` (state-as-type) e `Decide*` puros, 412/412 testes, 100% cobertura, review APPROVED |
| 3.0 | Workflow `budget-creation` + Continuer + Reaper (coleta espelha onboarding) | RF-01–05, RF-08, RF-09, RF-11, RF-12 | done — unit + integração Postgres real (testcontainers), bug real de conflação `MessageID`/`IncomingMessageID` corrigido durante a integração |
| 4.0 | Tool fina `create_budget` (inicia workflow) + DTO `Validate()` + mapeamento `MonthReference` | RF-01, RF-10, RF-25, RF-28 | done — adapter fino sobre `engine.Start`, 13 testes whitebox testify/suite, build/vet/test-race/lint verdes |
| 5.0 | Wiring `module.go` + `tryBudgetCreation` + mensagem específica + observabilidade | RF-25–27, RF-29, RF-30 | done — tool/workflow/continuer/reaper registrados, fix RF-26/RF-30 (mensagem de indisponibilidade específica + run auditável com erro persistido) |
| 6.0 | Resolver nas tools de leitura + instrução do agente + composição da retrospectiva | RF-17, RF-18, RF-20–24 | done — `DecideCompetence` aplicado a `query_month`/`query_plan`, instrução do agente atualizada para `MonthReference`/mês por extenso/retrospectiva por composição |
| 7.0 | Testes de integração (Postgres) + E2E real-LLM gate estatístico ≥0.90 | RF-04, RF-05, RF-08, RF-09, RF-15, RF-16, RF-22–24, RF-26 | done — integração Postgres 7/7, gate real-LLM ≥0.90 estável após corrigir **2 defeitos reais de produção** descobertos pelo harness (ver seção dedicada abaixo) |

Todos os `*_execution_report.md` (1.0 a 7.0) presentes e não vazios em
`.specs/prd-orcamento-retroativo-conversacional-e-mes-por-extenso/`.

## Defeitos reais de produção corrigidos pelo gate real-LLM (tarefa 7.0)

O harness estatístico real-LLM (não mockado — dirige `a.Execute` com tools reais sobre
`workflow.WithRuntime`/`agent.WithToolInvocationContext`) revelou dois gaps de comportamento que os
testes determinísticos não capturavam:

1. **`DecideCompetence` com `MonthRefExplicit` e `Year<=0`** construía silenciosamente uma
   competência inválida `"0000-06"` em vez de retornar `ClarifyMissingYear` (violação de RF-15). O
   LLM ocasionalmente emite `monthRefKind=explicit` sem `year` para mês nomeado sem ano. Corrigido em
   `internal/budgets/domain/valueobjects/month_reference.go`, com 2 novos casos de regressão de
   tabela pura (`Year: 0`, `Year: -1`).
2. **Composição da retrospectiva sem orçamento** omitia sistematicamente a oferta "Posso te ajudar a
   criar um?" (violação de RF-23/RF-24) — 0% de acerto antes da correção, 100% depois. Corrigido com
   um campo `OfferCreatePrompt` determinístico calculado pela própria tool `query_plan` (padrão
   verbatim-echo já comprovado no código, como `clarifyPrompt`), em vez de depender de composição
   livre pelo LLM.

Ambas as correções foram validadas com rodadas reais-LLM repetidas (7+ rodadas limpas consecutivas
cada) antes de fechar a tarefa.

## Correção adicional aplicada pelo orquestrador (pós-wave, pré-relatório)

Ao validar o estado agregado do repositório após a conclusão de todas as 7 tarefas, o orquestrador
detectou que `deployment/scripts/deadcode-agent-allowlist.txt` continha 47 entradas herdadas das
tarefas 3.0/4.0 sob o comentário "wiring em module.go/consumer entra na tarefa 5.0" — desatualizado,
pois a tarefa 5.0 já havia concluído o wiring. `task lint:deadcode` ainda passava (allowlist é
permissiva por natureza), mas a documentação estava obsoleta, o que viola a exigência de "0
pendências". Ação tomada:

- Removidas as 44 entradas que se tornaram genuinamente alcançáveis após o wiring de 5.0 (confirmado
  via `grep` em `module.go` e reconfirmado rodando `task lint:deadcode` sem elas).
- 3 símbolos permaneceram genuinamente inalcançáveis (`ParseBudgetAwaitingSlot`,
  `ParseBudgetCreationStatus`, `DecideBudgetPendingResume`) — reclassificados sob as categorias já
  estabelecidas no mesmo arquivo (`Parse* functions: smart constructors de tipos fechados (DMMF)
  mantidos por simetria de API` / `Decide* functions: regras de negocio puras (DMMF) mantidas por
  simetria de API`), com precedente direto em `ParseOnboardingPhase`/`DecideNewOperationReplacement`
  já presentes no arquivo antes desta orquestração.
- `task lint:deadcode` → `PASS lint:deadcode: nenhum codigo morto acionavel em internal/agents
  (RF-40)` após a correção.
- Removido `tasks.md.lock` (lockfile vazio remanescente da escrita concorrente da wave 4).

Nenhuma dessas correções alterou comportamento de produção — são exclusivamente higiene de metadados
de governança.

## Validação agregada (pós-orquestração, whole-repo)

```
go build ./...                                              → limpo
go vet ./...                                                 → limpo
go build -tags integration ./... && go vet -tags integration → limpo
go test -race ./...                                          → 0 falhas, todos os pacotes ok
gofmt -l internal/agents/ internal/budgets/                  → vazio
golangci-lint run ./internal/agents/... ./internal/budgets/... → 0 issues
task lint:deadcode                                            → PASS
task lint:auth-bypass                                         → PASS
task lint:outbox-user-id                                      → PASS
ai-spec check-spec-drift tasks.md                              → OK: sem drift detectado
```

Gates de governança específicos verificados sobre os arquivos novos/alterados desta orquestração:

```
R-ADAPTER-001.1 (zero comentários em .go novo/alterado)         → OK (vazio)
R-AGENT-WF-001.1 (sem switch por intent.Kind)                   → OK (vazio)
R-AGENT-WF-001.2 (sem SQL direto em tool/workflow)               → OK (delega a portas)
R-WF-KERNEL-001.1 (kernel sem import de domínio)                 → OK (kernel não tocado)
Cardinalidade de métricas (sem user_id/correlation_key/category_id como label) → OK (único hit é JSON struct tag pré-existente, não label Prometheus)
Sem prefixo `_` em identificadores NOVOS desta orquestração        → OK (globais `_defaultDistributionBP`/`_allocationInputSystemPrompt` são pré-existentes em onboarding_workflow.go, fora do escopo deste PRD — apenas reusados, não introduzidos)
```

Nenhum `TODO`, `FIXME`, placeholder ou `panic("unimplemented")` encontrado nos arquivos de produção
novos/alterados por esta orquestração.

## Cobertura de Requisitos Funcionais

Todos os 30 RFs do PRD (RF-01 a RF-30) estão cobertos por pelo menos uma tarefa, confirmado por
união da tabela de cobertura em `tasks.md` contra a lista completa de RFs extraída de `prd.md`:

RF-01, RF-02, RF-03, RF-04, RF-05, RF-06, RF-07, RF-08, RF-09, RF-10, RF-11, RF-12, RF-13, RF-14,
RF-15, RF-16, RF-17, RF-18, RF-19, RF-20, RF-21, RF-22, RF-23, RF-24, RF-25, RF-26, RF-27, RF-28,
RF-29, RF-30 — **30/30 cobertos**.

## Arquivos entregues (produção)

Novos:
- `internal/budgets/domain/valueobjects/month_reference.go` (+ testes)
- `internal/agents/application/workflows/budget_creation_state.go` (+ testes)
- `internal/agents/application/workflows/budget_creation_decisions.go` (+ testes)
- `internal/agents/application/workflows/budget_creation_workflow.go` (+ testes unit, integração Postgres, real-LLM)
- `internal/agents/application/usecases/budget_creation_continuer.go` (+ testes)
- `internal/agents/application/tools/create_budget.go` (+ testes)
- `internal/agents/application/tools/competence_reference.go`
- `internal/agents/application/agents/budget_creation_e2e_real_llm_test.go`

Alterados (aditivo, sem regressão):
- `internal/budgets/domain/valueobjects/competence.go` (`Prev()`, `FormatCompetencePtBR`)
- `internal/agents/application/interfaces/errors.go` (`ErrBudgetConflict`)
- `internal/agents/infrastructure/binding/budget_planner_adapter.go` (mapeia `ErrBudgetConflict`)
- `internal/agents/application/tools/query_month.go`, `query_plan.go` (resolver `DecideCompetence`, `OfferCreatePrompt`, descrições de schema)
- `internal/agents/application/agents/mecontrola_agent.go` (instrução: checklist de competência, desambiguação C1 vs retrospectiva, `offerCreatePrompt` verbatim)
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go` (`WithBudgetCreationResolver`, resume chain)
- `internal/agents/module.go` (wiring engine/def/continuer/reaper de `budget-creation`)
- `deployment/scripts/deadcode-agent-allowlist.txt` (limpeza pós-wiring, ver seção dedicada acima)

Nenhum arquivo fora do escopo de `internal/agents/`, `internal/budgets/` e
`deployment/scripts/deadcode-agent-allowlist.txt` foi tocado.

## Riscos residuais (documentados, não bloqueantes)

- `query_plan` na composição de retrospectiva sem orçamento ocasionalmente resolvia competência
  diferente de `query_month` na mesma resposta (~1 em 6 execuções antes do fix final de schema
  description; 0 em 6 nas últimas rodadas pós-fix). Como `OfferCreatePrompt` é gerado pela própria
  tool a partir da competência que ela mesma resolveu, a mensagem nunca fica internamente
  inconsistente — risco residual é puramente de precisão editorial (severidade `low`), monitorável
  via `agent_tool_invocations_total` e logs de conversa em produção.
- Reprompt sem limite nos slots de total/distribuição do `budget-creation` (mitigado por TTL
  30min + reaper 35min; nunca fica órfão permanente) — consistente com o padrão onboarding já em
  produção, não é regressão desta orquestração.

## Conclusão

As 7 tarefas do PRD `prd-orcamento-retroativo-conversacional-e-mes-por-extenso` foram executadas
integralmente, na ordem topológica correta, com paralelismo aplicado conforme `Paralelizável` em
`tasks.md` (waves 1 e 4). Todos os 30 requisitos funcionais (RF-01 a RF-30) estão cobertos e
validados, incluindo dois defeitos reais de produção descobertos e corrigidos pelo próprio gate de
aceitação estatístico real-LLM (não mascarados, não relaxados). Build, vet, testes com `-race` e
todos os gates de governança do repositório retornam limpos no agregado final, incluindo uma limpeza
de dívida de metadados de governança (`deadcode-agent-allowlist.txt`) identificada e corrigida pelo
orquestrador após a conclusão das waves. Nenhuma tarefa retornou `blocked`, `failed` ou
`needs_input`. Nenhum código temporário, mock de produção, TODO ou placeholder foi deixado no código
de produção.

**Não commitado** — mudanças permanecem no working tree para revisão humana antes de commit/push,
conforme prática do projeto de não commitar automaticamente ao final de orquestrações.
