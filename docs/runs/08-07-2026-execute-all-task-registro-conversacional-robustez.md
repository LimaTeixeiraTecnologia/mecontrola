# Relatório de Orquestração de PRD

## PRD
- Slug: `registro-conversacional-robustez`
- Diretório: `.specs/prd-registro-conversacional-robustez/`
- PRD: `.specs/prd-registro-conversacional-robustez/prd.md`
- TechSpec: `.specs/prd-registro-conversacional-robustez/techspec.md`
- Tasks: `.specs/prd-registro-conversacional-robustez/tasks.md`
- ADRs: adr-001 (seed folha salário), adr-002 (propagação de erro), adr-003 (retry transitório
  limitado), adr-004 (confirmação única), adr-005 (guarda de kind)

## Resultado Final
- Status do orquestrador: **done**
- Total de tarefas no PRD: 8
- Tarefas done: 8
- Tarefas pending: 0
- Tarefas blocked: 0
- Tarefas failed: 0
- Tarefas needs_input: 0
- Requisitos funcionais cobertos: RF-01 a RF-32 (32/32), 0 desvios, 0 lacunas, 0 falso positivo,
  0 pendências, 0 ressalvas não resolvidas, 0 flexibilizações.

## Pré-voo

- `unset AI_PREFLIGHT_DONE` executado antes de qualquer comando.
- Hook `pre-execute-all-tasks.sh registro-conversacional-robustez` → OK (8 tarefas validadas).
- `check-invocation-depth.sh` (`.agents/lib/`) — sourced, `AI_INVOCATION_DEPTH=0`.
- Binário `ai-spec` presente (`/opt/homebrew/bin/ai-spec`).
- `ai-spec skills check` — 7 skills verificadas, sem drift bloqueante.
- `ai-spec check-spec-drift .specs/prd-registro-conversacional-robustez/tasks.md` → "OK: sem drift
  detectado."
- `prd.md` lido integralmente (324 linhas) antes de iniciar qualquer execução.

## Snapshot Inicial vs Final
| # | Título | Status inicial | Status final |
|---|--------|----------------|--------------|
| 1.0 | Seed folha income `Salário > Salário` + dicionário | pending | done |
| 2.0 | Consolidar formatação BRL canônica em `money.BRL()` | pending | done |
| 3.0 | Propagação de erro: fim do swallow + Run auditável no resume | pending | done |
| 4.0 | Retry transitório limitado + `IsTransient` + idempotência | pending | done |
| 5.0 | Guarda de kind + reclassificação + clarify único | pending | done |
| 6.0 | Confirmação única: reescrita de instruções do agente | pending | done |
| 7.0 | Heurística múltiplos lançamentos + slot de forma de pagamento | pending | done |
| 8.0 | Suíte E2E real-LLM + integração dos cenários Gherkin | pending | done |

## Tarefas Executadas Nesta Sessão
| # | Título | Status | Report Path | Summary |
|---|--------|--------|-------------|---------|
| 1.0 | Seed folha salário + dicionário | done | `.specs/prd-registro-conversacional-robustez/1.0_execution_report.md` | Folha income `Salário > Salário` (slug `salario-base`) + 4 termos de dicionário via migração 000006 idempotente; RF-01..RF-05 cobertos; review APPROVED_WITH_REMARKS (achado low corrigido). |
| 2.0 | Consolidar BRL `money.BRL()` | done | `.specs/prd-registro-conversacional-robustez/2.0_execution_report.md` | `money.BRL()` consolidado como fonte única; `formatBRL`/`formatAmountBR` removidos do módulo agents; 6 call-sites migrados. |
| 3.0 | Propagação de erro (fim do swallow) | done | `.specs/prd-registro-conversacional-robustez/3.0_execution_report.md` | Fim do swallow (`StepStatusFailed` + erro real) em `executeWithIdempotency`/`executeDirectWrite`/`validateCategoryForWrite`; Run auditável em `platform_runs` no resume; span/log pesquisável por `thread_id`/`run_id`/`wamid`. |
| 4.0 | Retry transitório + idempotência + **correção RF-23** | done | `.specs/prd-registro-conversacional-robustez/4.0_execution_report.md` | Retry limitado (max 2, backoff+jitter <2s) via `IsTransient`; **gap arquitetural real encontrado e corrigido**: RF-23 (retomada pós-exaustão de retry) exigia reviver um run `RunStatusFailed`, impossível via `Engine.Resume` clássico (exige `RunStatusSuspended`) — corrigido com novos primitivos genéricos `Store.LoadLatest`/`Engine.LoadLatestState` no kernel, sem violar ADR-002/R-WF-KERNEL-001; 0 regressão, evidência de teste prova reexecução sem reclassificar categoria. |
| 5.0 | Guarda de kind + reclassificação | done | `.specs/prd-registro-conversacional-robustez/5.0_execution_report.md` | Guarda de kind income/expense com reclassificação automática e clarify único; `ErrKindMismatch` preservado como defesa final; review APPROVED sem achados. |
| 6.0 | Confirmação única (instruções do agente) | done | `.specs/prd-registro-conversacional-robustez/6.0_execution_report.md` | Instruções reescritas para que o gate HITL do workflow seja o único emissor de confirmação; LLM apenas repassa `message` literalmente. |
| 7.0 | Heurística múltiplos + slot de pagamento | done | `.specs/prd-registro-conversacional-robustez/7.0_execution_report.md` | Heurística de milhar BRL corrigida no prompt; slot de pagamento com exemplos + proibição de inferência; fix `knownPaymentMethods` (`"doc"` → `ted`); review APPROVED. |
| 8.0 | Suíte E2E real-LLM + **correção RF-21** | done | `.specs/prd-registro-conversacional-robustez/8.0_execution_report.md` | 9 cenários Gherkin validados real-LLM+Postgres; 4 bugs de produção genuínos descobertos e corrigidos (evidência propagada ao gate real, slot de pagamento inalcançável, colisão de dicionário salário/décimo-terceiro, erro genérico do kernel vazando). **Gap real encontrado e corrigido**: RF-21 (texto de orientação de múltiplos lançamentos "inalterado") media 0/10 de aderência do LLM via instrução de prompt — corrigido estruturalmente em dois níveis: (1) bypass de tool-calling em `internal/platform/agent` que retorna verbatim qualquer resultado de tool marcado como texto canônico, sem nova chamada ao LLM (também fortalece RF-14..RF-18/RF-30); (2) guard determinístico pré-LLM (`multi_item_guard.go`) que intercepta o caso dominante (LLM nunca chama tool) via regex de token monetário, sem custo de LLM. 2 regressões reais pegas por revisão adversarial e corrigidas antes de fechar (escopo do guard em onboarding; falsos positivos de regex em datas/IDs/UUIDs). |

## Tarefas Puladas (já estavam done)
- Nenhuma — todas as 8 tarefas estavam `pending` no início desta sessão.

## Waves Executadas
| # | Modo | Tarefas | Observação |
|---|------|---------|-----------|
| 1 | paralelo | 1.0, 6.0 | Declaradas mutuamente paralelizáveis em tasks.md (arquivos disjuntos: migração vs instruções do agente). |
| 2 | sequencial | 2.0 | `Paralelizável: Não`. |
| 3 | sequencial | 3.0 | `Paralelizável: Não`; fundação de observabilidade para 4.0/5.0. |
| 4 | sequencial | 4.0 | Depende de 3.0; **+ correção RF-23 pós-aprovação** no mesmo escopo da tarefa. |
| 5 | sequencial | 5.0 | Depende de 3.0; toca `pending_entry_workflow.go` já modificado por 3.0/4.0. |
| 6 | sequencial | 7.0 | `Paralelizável: Não`; toca `mecontrola_agent.go` já modificado por 6.0. |
| 7 | sequencial | 8.0 | Depende de 1.0–7.0 (todas); **+ correção RF-21 pós-aprovação** no mesmo escopo da tarefa. |

Cada wave foi validada de forma independente pelo orquestrador (build + vet + test race + lint +
gates de governança no projeto inteiro) antes de iniciar a próxima, além da validação interna de
cada subagent.

## Gaps Reais Encontrados e Corrigidos Durante a Execução

Duas tarefas concluídas pelos subagents ficaram, em uma primeira passada, com risco residual
documentado (não "ressalva silenciosa" — achado transparente com análise técnica completa). Por
exigência explícita do usuário (0 ressalvas, 0 lacunas), ambas foram investigadas a fundo e
corrigidas estruturalmente antes de considerar o PRD concluído:

1. **RF-23 (tarefa 4.0)** — conflito real entre o contrato testado de ADR-002 (`StepStatusFailed`
   sempre em erro real de escrita) e o texto de ADR-003 §4 ("pending entry permanece retomável").
   Investigação confirmou que `Engine.Resume` exige `RunStatusSuspended`, tornando um run `Failed`
   irrecuperável pelo caminho clássico. Corrigido com `Store.LoadLatest`/`Engine.LoadLatestState`
   — primitivos genéricos novos no kernel (sem semântica de domínio, preservando
   R-WF-KERNEL-001) que permitem ao consumidor reviver o snapshot mais recente e reexecutar a
   escrita sem reclassificar categoria, sem duplicar lançamento (idempotência preservada) e sem
   reabrir o gate HITL.
2. **RF-21 (tarefa 8.0)** — a suíte E2E real-LLM mediu 0/10 de aderência do modelo de produção
   (`gpt-4o-mini`) ao texto de orientação de múltiplos lançamentos "inalterado", mesmo após 3
   rodadas de reforço de prompt em tarefas anteriores. Investigação confirmou que o relay verbatim
   de TODAS as mensagens canônicas do sistema (confirmação, clarify, slot de pagamento, orientação
   de múltiplos lançamentos) dependia exclusivamente de instrução de prompt, sem nenhum enforcement
   estrutural. Corrigido em dois níveis complementares (bypass de tool-calling + guard
   determinístico pré-LLM), fortalecendo também RF-08/RF-15/RF-16/RF-30 que compartilhavam o mesmo
   padrão frágil.

Em ambos os casos, o fechamento foi validado com evidência física de teste (real-LLM + Postgres
real via testcontainer onde aplicável), 0 regressão comprovada nos RFs já cobertos, e gates de
governança completos re-executados no projeto inteiro após cada correção.

## Validação Final Independente (pelo orquestrador, após a última correção)

- `go build ./...` → pass (sem output/erros)
- `go vet ./...` → pass (sem output/erros)
- `go build -tags integration ./...` → pass
- `go vet -tags integration ./...` → pass
- `go test -race -count=1 ./...` → pass, **136/136 pacotes, exit 0**, 0 `FAIL`
- `task lint:run` → **0 issues**; gates `lint:auth-bypass`, `lint:outbox-user-id`, `lint:deadcode`
  todos PASS
- Gate R-WF-KERNEL-001.1 (import de domínio/camada superior no kernel) → vazio (OK)
- Gate R-AGENT-WF-001 (switch por `intent.Kind`) → vazio (OK)
- Gate R-ADAPTER-001.1 (zero comentários fora das exceções) → vazio (OK)
- Gate cardinalidade de métricas (`user_id`/`correlation_key`/`category_id` como label) → vazio (OK)
- `.env` com `OPENROUTER_*` presente (suíte real-LLM executável)
- Evidência adicional do subagent da correção RF-21 (independentemente reconferida quanto a
  build/vet/lint/gates pelo orquestrador): `go test -race -count=1 ./...` com `-tags integration` e
  `RUN_REAL_LLM=1` → 4292/4292 testes, incluindo suíte Gherkin completa G1-G9 (9/9, 2 execuções),
  `TestRealLLM_ToolCoverage_All22Tools` M-04=1.00 (29/29 tools).

## Mapeamento Completo de Requisitos Funcionais

| Tarefa | RFs cobertos | Status |
|--------|-------------|--------|
| 1.0 | RF-01, RF-02, RF-03, RF-04, RF-05 | done |
| 2.0 | RF-26, RF-27, RF-28 | done |
| 3.0 | RF-10, RF-11, RF-12, RF-13 | done |
| 4.0 | RF-22, RF-23, RF-24, RF-25 | done (RF-23 fechado por correção pós-aprovação) |
| 5.0 | RF-06, RF-07, RF-08, RF-09 | done |
| 6.0 | RF-14, RF-15, RF-16, RF-17, RF-18 | done |
| 7.0 | RF-19, RF-20, RF-21, RF-29, RF-30, RF-31, RF-32 | done |
| 8.0 | RF-01..RF-32 (todos, validação E2E) | done (RF-21 fechado por correção pós-aprovação) |

Todos os 32 requisitos funcionais do PRD (RF-01 a RF-32) estão cobertos e comprovados com evidência
física de teste. Nenhum requisito ficou parcialmente implementado, adiado ou flexibilizado.

## Próximos Passos

- Nenhuma ação bloqueante pendente para conformidade com este PRD.
- Recomendação operacional (fora do escopo deste PRD, mencionada pelos subagents como risco de
  infraestrutura pré-existente, não uma lacuna de requisito): a exclusividade de `Engine.Start`
  concorrente para a mesma `correlation_key` hoje depende da topologia de produção
  (`replicas: 1` no dispatcher). Revisitar com lock distribuído explícito se o dispatcher migrar
  para múltiplas réplicas.
- Recomendação operacional (mencionada pelo subagent de 3.0, não uma lacuna de requisito): o
  consumidor WhatsApp não envia resposta ao usuário em falha real de
  `PendingEntryContinuer.Continue` fora do fluxo de escrita coberto por este PRD — candidato a
  hardening futuro, sem RF associado neste PRD.
- Deploy: a migração 000006 (seed da folha salário) e 000007 (correção da colisão de dicionário
  décimo-terceiro, descoberta pela suíte E2E da tarefa 8.0) precisam ser aplicadas em produção antes
  do comportamento ficar disponível para usuários reais.

## Suposições

- O arquivo de relatório final desta orquestração foi salvo com nome disambiguado
  (`08-07-2026-execute-all-task-registro-conversacional-robustez.md`) em vez do nome literal pedido
  (`08-07-2026-execute-all-task.md`), pois esse nome exato já existia em `docs/runs/` para outro PRD
  concluído na mesma data (`prd-onboarding-valor-opcional-meta`) — sobrescrevê-lo destruiria o
  relatório de uma execução anterior não relacionada.
- As correções pós-aprovação de RF-23 e RF-21 foram tratadas como parte do escopo já aprovado das
  tarefas 4.0 e 8.0 respectivamente (mesmo padrão usado nas duas correções: fechar gap descoberto
  durante a própria cadeia de implementação, sem abrir uma tarefa numerada nova em tasks.md), dado
  que ambos os RFs já estavam explicitamente atribuídos a essas tarefas na tabela de Cobertura de
  Requisitos original.

## Riscos Residuais

- Nenhum bloqueante para este PRD. Os únicos riscos residuais documentados nos relatórios de tarefa
  (topologia `replicas: 1` para exclusividade de `Start`; ausência de resposta ao usuário em falha
  de infraestrutura do `PendingEntryContinuer` fora do caminho de escrita) são riscos operacionais
  de infraestrutura pré-existente, não lacunas de requisito funcional deste PRD, e foram assim
  classificados explicitamente pelos subagents que os reportaram.
