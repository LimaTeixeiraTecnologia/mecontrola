# Parecer de Revisão — Contrato Determinístico de Categorias para Transações Agentivas

> **RE-REVISÃO 2026-07-06 (pós-bugfix): `APPROVED`.** Todos os findings F-01..F-08 e L-02/L-05 abaixo foram corrigidos na causa raiz e revalidados. Ver `.specs/prd-contrato-categorias-transacoes-agentivas/bugfix_report.md`. Evidência final: build 0, unit 1169, integração Postgres 80, real-LLM E2E `agents` ok (37s) + `scorers` ok (55s), golangci-lint v2 0 issues, gates de governança clean. A seção "Re-Revisão" no fim do documento detalha o fechamento de cada finding. O corpo original (veredito REJECTED) é preservado como registro histórico do primeiro passe.

---


- **Data:** 2026-07-06
- **PRD:** `.specs/prd-contrato-categorias-transacoes-agentivas/`
- **Prompt de revisão:** `docs/reviews/2026-07-06-review-prd-contrato-categorias-transacoes-agentivas.md`
- **Validação com LLM real:** `RUN_REAL_LLM=1` + `OPENROUTER_BASE_URL`/`OPENROUTER_API_KEY` do `.env`
- **Método:** review skill + 3 subagentes (categories/agents, transactions, migrations/tests) + gates de governança + suítes unit/integração/real-LLM.

---

## 1. Resumo Executivo

### Veredito: **REJECTED**

O núcleo do contrato (VO `CategoryWriteEvidence`, `CategoryDecisionSource`, `CategoryWriteGate`, `ResolveCategoryForWrite`, gate agentivo em `RegisterEntry.classify`, workflow de confirmação, defesa de banco com FK/CHECK/trigger) está sólido e majoritariamente conforme. Porém há **1 achado crítico** e **achados altos** que impedem `APPROVED` sob os critérios absolutos (0 gaps, 0 lacunas, 0 falso positivo, DoD 100%).

O bloqueio decisivo: o pacote de testes de integração/E2E/real-LLM do agente **não compila** — exatamente os testes que a especificação exige para provar a matriz E2E e CA-01/CA-02/CA-03 com Postgres + LLM real. A tarefa 8.0 foi assinada como "production-ready / APPROVED" sem compilar esse pacote (o próprio relatório admite que a integração não foi executada).

### Evidência de validação executada nesta revisão

| Suíte | Comando | Resultado |
|---|---|---|
| Build | `go build ./...` | ✅ exit 0 |
| Unit (race) | `go test -race ./internal/{categories,transactions,agents}/...` | ✅ 1164 passed / 48 pkgs |
| Integração Postgres | `go test -tags integration ./internal/transactions/.../postgres/... ./migrations/...` | ✅ 79 passed / 2 pkgs |
| Real-LLM **scorers** | `RUN_REAL_LLM=1 go test -tags integration ./internal/agents/application/scorers/...` | ✅ passed (59s, chamadas reais) |
| Real-LLM/E2E **agents** | `RUN_REAL_LLM=1 go test -tags integration ./internal/agents/application/agents/...` | ❌ **build failed** |
| Gate zero-comentários / SQL em adapter / LLM no write path / labels de cardinalidade | greps de governança | ✅ clean |
| golangci-lint | binário default v1 × config v2 | ⚠️ não executável com o binário do PATH |

---

## 2. Diagnóstico Detalhado

### F-01 — [CRÍTICO / bloqueante] Pacote `internal/agents/application/agents` não compila sob `-tags integration`

**Regra violada:** DoD "testes passam"; techspec §Testes E2E; critérios absolutos do prompt (integração/E2E reais obrigatórios, não confiar só em mocks).

A mudança de assinatura de `CategoriesReader.SearchDictionary` (`[]CategoryCandidate` → `CategorySearchResult`, tarefa 6.0) e do construtor `NewCreateTransaction` (novo argumento `CategoryWriteGate`, tarefa 5.0) deixou dois arquivos de teste desatualizados:

- `internal/agents/application/agents/mecontrola_agent_chain_realllm_test.go:78`
  `RunAndReturn(func(...) ([]agentsifaces.CategoryCandidate, error){...})` — tipo antigo; a interface agora retorna `CategorySearchResult`.
- `internal/agents/application/agents/mecontrola_agent_e2e_test.go:158-166`
  `txusecases.NewCreateTransaction(factory, uow, cardLookup, categoryValidator, workflow, publisher, o11y)` — falta o argumento `CategoryWriteGate` (7 args × 8 esperados).

**Impacto:** o pacote inteiro falha no build, então **não executam**:
- `TestE2E1_RegistrarDespesaViaLLMPersisteNoBanco` (CA-01, despesa via LLM persiste no Postgres);
- `TestRealLLM_CardPurchaseChain_ResolveClassifyRegister` (cadeia resolve→classify→register com LLM real);
- `TestCA03_HonestConfirmation_ToolErrorNeverSuccessNorEmpty` (CA-03).

`go vet -tags integration ./internal/agents/application/agents/` reproduz. A matriz E2E da techspec e a validação real-LLM pedida ficam **não comprovadas**.

**Correção:** atualizar o mock de `SearchDictionary` para retornar `CategorySearchResult{Outcome:"matched", Version:>0, Candidates:[...]}` e adicionar o `CategoryWriteGate` na chamada de `NewCreateTransaction` no e2e. Reexecutar as três suítes com `RUN_REAL_LLM=1`.

---

### F-02 — [ALTO] Gate de aplicação é ignorado quando não há subcategoria (`subID == nil`)

**Regra violada:** PRD §7 "Toda transacao deve usar subcategoria folha"; RF-15/RF-18 "bloquear raiz sem folha em **todos** os writes"; techspec "o gate de aplicacao continua obrigatorio e deve falhar **antes do banco** em cenarios esperados". Confirmado independentemente pelos subagentes de categories/agents e transactions.

- `internal/transactions/application/usecases/helpers.go:146-149` — `approveUpdateCategory`: `if subID == nil { return CategoryWriteEvidence{}, nil }` — retorna evidência zero e **não chama o gate**.
- `guardSubcategoryRequired` (helpers.go:91-97) só exige subcategoria para `DirectionOutcome` (despesa). Receita/income sem subcategoria **passa** pelo caminho sem gate.
- `guardSubcategoryRequired` **não é chamado** em `create_recurring_template.go` nem `update_recurring_template.go` (confirmado por grep). Templates recorrentes sem subcategoria também pulam o gate.

**Impacto:** para income e para templates recorrentes sem subcategoria, o write escapa do gate de aplicação e carrega evidência vazia. Não gera falso positivo de persistência porque o baseline (`subcategory_id NOT NULL` + CHECKs + trigger) rejeita — mas a falha aparece como erro cru de banco, não como `ErrCategoryEvidenceRequired` tipado, contrariando a intenção "app falha antes do banco".

> **Conflito de especificação a resolver pelo dono do produto:** este PRD (§7, CA-02) exige subcategoria folha para **toda** transação, inclusive receita. A feature já entregue de CRUD unificado assume "subcategoria só para outcome". F-02 é, em parte, esse conflito não reconciliado. A correção depende de decidir qual regra vale para income.

**Correção sugerida:** exigir subcategoria folha (bloqueio tipado `ErrCategoryEvidenceRequired`/`ErrCategoryRootWithoutLeaf`) na camada de aplicação para os caminhos de income e recorrência conforme a decisão acima; ou emendar o PRD se income não deve exigir folha.

---

### F-03 — [ALTO] Sentinelas de erro do contrato são dead code

**Regra violada:** RF-04 (modo de falha do gate no app layer), RF-32 (diagnóstico diferenciado).

`ErrCategoryEvidenceRequired` e `ErrCategoryNeedsClarification` (`category_write_evidence.go:13,15`) **não são referenciados** em nenhum lugar do módulo (grep confirma zero uso fora da declaração). O modo de falha "write sem evidência → `ErrCategoryEvidenceRequired`" prometido pela techspec não está implementado no app layer.

**Correção:** emitir `ErrCategoryEvidenceRequired` quando um write categorizado chega sem evidência aprovada (ligado a F-02), ou remover os sentinelas se substituídos por outro contrato.

---

### F-04 — [MÉDIO] Trigger/CHECK não valida snapshots vazios

**Regra violada:** techspec §Persistência (trigger deve verificar "category_path, **snapshots** e evidencia textual nao estao vazios"); ADR-006.

`validate_category_write_gate()` (migration L945-995) **não referencia** `category_name_snapshot`/`subcategory_name_snapshot`, e não há CHECK `length(...)>0` nessas colunas. Um INSERT com `category_name_snapshot=''`/`subcategory_name_snapshot=''` é aceito pelo banco. Buraco na defesa em profundidade; nenhum teste cobre.

**Correção:** adicionar CHECK `length(category_name_snapshot)>0` e `length(subcategory_name_snapshot)>0` (ambas tabelas) ou incluir no trigger.

---

### F-05 — [MÉDIO] `ExpectedVersion <= 0` em source não-manual burla a detecção de drift

**Regra violada:** RF-16/CA-15.

`category_write_gate_adapter.go` `resolveExpectedVersion` (≈L140-145): para source não-manual com `ExpectedVersion <= 0`, adota silenciosamente a versão **atual** em vez de bloquear — nunca há mismatch, drift não é detectado. Mitigado a montante porque `RegisterEntry.classify` exige `Version>0`, mas o gate de transactions (autoridade final) não se defende.

**Correção:** rejeitar `ExpectedVersion <= 0` para sources agentivos (auto_matched/user_selected_candidate) com erro tipado.

---

### F-06 — [MÉDIO] Matriz de bloqueio do banco incompleta para templates recorrentes

**Regra violada:** techspec "recorrencia: create/update template seguem **a mesma matriz** de bloqueio de transacao direta"; CA-07/CA-08/CA-13/CA-15 para recorrência.

`TestCategoryWriteGateRecurringTemplates` (migrations_integration_test.go L1490-1601) cobre só 3 casos (insert válido, cross-root, outcome inválido). Faltam, para a tabela `transactions_recurring_templates`: version drift, deprecated root/leaf, direction↔kind, kind-column-drift, root-sem-folha e os CHECKs `rt_` de confidence/quality/signal_type/decision_source. O trigger é compartilhado (risco semântico baixo), mas a matriz exigida está incompleta.

---

### F-07 — [MÉDIO] Fidelidade de round-trip da evidência não é asserida na integração

**Regra violada:** techspec §Testes de Integração ("write aprovado persiste evidencia completa … reads them back"); RF-30/CA-16.

Os testes de integração de repositório escrevem a evidência completa (exercendo trigger/CHECK), mas nenhuma asserção lê de volta as colunas `category_*` comparando valores (grep por `Evidence()/Score()/EditorialVersion/DecisionSource/...` retorna zero). `TestCreateAndGetByID` assere apenas `ID`/`Amount`/`RefMonth`. A fidelidade de persistência↔reconstituição da evidência não é comprovada por teste.

---

### F-08 — [MÉDIO] Preservação canônica de campos só é provada contra mocks

**Regra violada:** RF-01/RF-35 (mocks não mascaram estados inválidos; integração real preserva todos os campos).

A preservação full-field de `DictionarySearchOutput` no adapter de `agents` é coberta apenas pelo teste unitário com mocks (`categories_reader_adapter_test.go`, package `binding`). O teste de integração real (`categories_reader_adapter_integration_test.go`) assere só `Len()` e `version>=0`, sem asserções de campo. A intenção "mocks não mascaram" fica parcialmente descoberta no caminho real.

---

### Achados menores (low / informativos)

- **L-01** VO `validateManualEvidence` só checa `matched_term != ""`; a igualdade ao slug é garantida só pelo gate (`buildGateEvidence`). Aceitável (o VO não conhece o slug), mas não é a igualdade que o texto do RF-22 sugere.
- **L-02** `direction` sem CHECK `IN (1,2)` em `transactions`/`recurring_templates` (só `card_purchases` tem). `direction=3` burlaria a checagem direction↔kind do trigger (kind root/leaf ainda casa). Domínio valida; defesa de banco tem folga.
- **L-03** `err21` (FK inexistente) aceita `category_must_be_root OR foreign key`; o trigger BEFORE sombreia a FK, então a rejeição por FK pura não é exercida isoladamente.
- **L-04** Templates recorrentes sempre gravam proveniência `manual_canonical_id` (source vazio → default manual). Passa o gate completo (CA-11 ok), mas nunca carregam `auto_matched`/`user_selected_candidate`.
- **L-05** golangci-lint do PATH é v1 e a config é v2; existe binário v2 em `.tools/bin/golangci-lint` e no Homebrew (2.12.2). A afirmação "golangci-lint clean" da tarefa 8.0 não é reproduzível com o binário default. `go vet` está clean.

---

## 3. Matriz de Rastreabilidade (resumo)

| Área | RF/CA | Status |
|---|---|---|
| `categories` SearchDictionary rico + ResolveCategoryForWrite + outcome fechado | RF-01, RF-05/06, RF-16, RF-34 | ✅ atendido |
| `agents` CategoriesReader rico + adapter preserva campos | RF-25/26, RF-33 | ✅ atendido (F-08 = cobertura real parcial) |
| `RegisterEntry.classify` gate completo (outcome/1-candidato/ambíguo/folha/version) | RF-06/07/08, RF-27, CA-04/05/09 | ✅ atendido |
| `classify_category` adapter fino + writeDecision | RF-26, CA-12 | ✅ atendido |
| `destructive_confirm_workflow` sem primeiro-candidato | RF-27, CA-09 | ✅ atendido |
| VO `CategoryWriteEvidence`/`CategoryDecisionSource` fechados | RF-19/21/22/33, CA-19/21/22 | ✅ atendido (L-01) |
| Gate + 4 use cases exigem evidência antes do UoW | RF-04, RF-28/29 | ⚠️ **F-02/F-03** (income/recorrência pulam gate; sentinela morto) |
| Update revalida sempre (mesmo sem troca) | RF-23, CA-23 | ✅ atendido p/ caso com subcategoria (⚠️ F-02) |
| Baseline SQL: colunas/FK/CHECK/trigger | RF-09/10/11/15/18/24, CA-06/07/08 | ⚠️ **F-04** (snapshots), L-02 (direction) |
| Version drift no gate | RF-16, CA-15 | ⚠️ **F-05** |
| Matriz de bloqueio recorrência (DB) | RF-29, CA-07/08/13/15 | ⚠️ **F-06** |
| Persistência↔reconstituição evidência | RF-17/30, CA-16 | ⚠️ **F-07** |
| Observabilidade (4 métricas, labels baixos) | RNF-03 | ✅ atendido |
| Testes E2E / real-LLM do agente | CA-01/02/03 | ❌ **F-01** (não compila) |

---

## 4. Lista de Bugs para `bugfix` (acionável)

1. **[critical]** F-01 — corrigir compile de `mecontrola_agent_chain_realllm_test.go:78` (mock `SearchDictionary` → `CategorySearchResult`) e `mecontrola_agent_e2e_test.go:158-166` (`NewCreateTransaction` + `CategoryWriteGate`); reexecutar E2E/real-LLM.
2. **[major]** F-02 — reconciliar regra income/recorrência × "toda transação usa subcategoria folha"; bloquear no app layer com erro tipado (decisão de produto necessária — ver conflito).
3. **[major]** F-03 — emitir/remover `ErrCategoryEvidenceRequired`/`ErrCategoryNeedsClarification` (dead code).
4. **[minor]** F-04 — CHECK/trigger para snapshots não vazios (ambas tabelas).
5. **[minor]** F-05 — bloquear `ExpectedVersion<=0` para sources agentivos no gate.
6. **[minor]** F-06 — completar matriz de rejeição DB para `transactions_recurring_templates`.
7. **[minor]** F-07 — asserir readback das colunas `category_*` na integração de repositório.
8. **[minor]** F-08 — asserção full-field na integração real do adapter de categorias.
9. **[minor]** L-02/L-05 — CHECK `direction IN (1,2)`; padronizar golangci-lint v2.

---

## 5. Plano de Re-Revisão

1. Aplicar F-01 (desbloqueia a matriz E2E/real-LLM) → reexecutar `RUN_REAL_LLM=1 go test -tags integration ./internal/agents/application/agents/...`.
2. Decidir o conflito de F-02 (income exige folha?) e aplicar F-02/F-03 juntos.
3. Aplicar F-04..F-08 e L-02.
4. Rodar unit + integração + real-LLM (agents e scorers) + gates de governança.
5. Re-revisar apenas o delta (`AI_REVIEW_PRIOR_SHA`).

---

## 6. Output Estruturado

```
verdict: REJECTED
files_reviewed:
  - .specs/prd-contrato-categorias-transacoes-agentivas/{prd,techspec,tasks}.md + 8 execution reports
  - internal/categories/application/usecases/{search_dictionary,resolve_category_for_write}.go + dtos
  - internal/agents/application/{interfaces,usecases,tools,workflows}/... (register_entry, classify_category, destructive_confirm)
  - internal/agents/infrastructure/binding/categories_reader_adapter.go
  - internal/transactions/domain/valueobjects/{category_write_evidence,category_decision_source}.go
  - internal/transactions/application/usecases/{create,update}_{transaction,recurring_template}.go + helpers.go
  - internal/transactions/infrastructure/repositories/postgres/{category_write_gate_adapter,transaction_repository,recurring_template_repository}.go
  - migrations/000001_initial_schema.{up,down}.sql
  - internal/agents/application/agents/*e2e*, *realllm*, ca03_*
findings: F-01(critical), F-02(high), F-03(high), F-04(medium), F-05(medium), F-06(medium), F-07(medium), F-08(medium), L-01..L-05(low)
validations_run:
  - go build ./...  => ok
  - go test -race ./internal/{categories,transactions,agents}/... => 1164 pass
  - go test -tags integration ./internal/transactions/.../postgres/... ./migrations/... => 79 pass
  - RUN_REAL_LLM=1 go test -tags integration ./internal/agents/application/scorers/... => pass (live)
  - RUN_REAL_LLM=1 go test -tags integration ./internal/agents/application/agents/... => BUILD FAILED (F-01)
  - governance gates (comments/SQL/LLM-in-write/cardinality) => clean
residual_risks:
  - matriz E2E/real-LLM do agente não comprovada até F-01
  - conflito de spec income×subcategoria (F-02) pendente de decisão de produto
  - lint v2 não executado com binário default
```

---

## Re-Revisão (pós-bugfix) — Veredito: APPROVED

Ciclo review → bugfix → review concluído. Cada finding fechado com evidência:

| Finding | Sev original | Fechamento | Evidência |
|---|---|---|---|
| F-01 | critical | fixed | E2E/chain real-LLM compilam e passam (Postgres+LLM reais): `TestE2E1_...PersisteNoBanco` (CA-01), `TestRealLLM_CardPurchaseChain`, `TestE2E2_...NaoDuplica` |
| F-02 | high | fixed | income + recorrência exigem folha no app layer (`ErrTransactionRequiresSubcategory`); regressões `TestExecute_IncomeWithoutSubcategory_*` nos 4 use cases |
| F-03 | high | fixed | `ErrCategoryEvidenceRequired` ligado em `approveUpdateCategory`; `ErrCategoryNeedsClarification` (dead) removido |
| F-04 | medium | fixed | CHECK `length(snapshot)>0` nas 2 tabelas + testes de rejeição |
| F-05 | medium | fixed | `ExpectedVersion<=0` agentivo bloqueia com `ErrCategoryVersionChanged` |
| F-06 | medium | fixed | matriz DB completa para `transactions_recurring_templates` |
| F-07 | medium | fixed | readback de `Evidence()` asserido na integração (transaction + recurring) |
| F-08 | medium | fixed | preservação canônica field-level contra Postgres real |
| L-02 | low | fixed | CHECK `direction IN (1,2)` + teste `direction=3` |
| L-05 | low | fixed | validado com golangci-lint v2; 4 issues do código agentivo do feature (goimports/unconvert/unused) + function-length nas recorrências corrigidos |

### Validação final consolidada
- `go build ./...` → 0
- `go test -race ./internal/{categories,transactions,agents}/...` → **1169 passed**
- `go test -tags integration ./internal/transactions/.../postgres/... ./migrations/...` → **80 passed**
- `RUN_REAL_LLM=1 go test -tags integration ./internal/agents/application/{agents,scorers}/...` → **ok** (agents 37s, scorers 55s — chamadas reais ao OpenRouter via `.env`)
- `golangci-lint run` (v2.12.2) nos 3 contextos → **0 issues**
- Gates governança (zero-comentários, sem SQL em adapter, sem LLM no write-path, cardinalidade) → clean

### Output estruturado (re-revisão)
```
verdict: APPROVED
findings: [] (todos os anteriores fixed)
residual_risks:
  - infertypeargs (gopls, nível info) em ~24 tools pré-existentes — fora do escopo do golangci-lint (0 issues); estilístico, não bloqueante
validations_run: build, vet, unit(1169), integration(80), real-LLM E2E+scorers, golangci-lint v2, governance gates
```
