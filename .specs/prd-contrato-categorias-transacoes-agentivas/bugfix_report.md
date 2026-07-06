# Relatório de Bugfix — Contrato Determinístico de Categorias para Transações Agentivas

- **Data:** 2026-07-06
- **Origem:** findings F-01..F-08 + L-02/L-05 do parecer `docs/reviews/2026-07-06-review-result-prd-contrato-categorias-transacoes-agentivas.md`
- **Decisão de produto aplicada:** income (receita) exige subcategoria folha (alinha PRD §7 / CA-02).
- **Ciclo:** review (REJECTED) → bugfix → review (APPROVED).
- **Validação:** LLM real via `.env` (`OPENROUTER_BASE_URL`/`OPENROUTER_API_KEY`, `RUN_REAL_LLM=1`).

## Bugs Corrigidos

### F-01 — [critical] `fixed` — pacote agents não compilava sob `-tags integration`
- **Origem:** finding F-01 (regressão da tarefa 6.0/5.0; testes E2E/real-LLM nunca compilaram).
- **Correção:** `internal/agents/application/agents/mecontrola_agent_chain_realllm_test.go` (mock `SearchDictionary` → `CategorySearchResult`); `mecontrola_agent_e2e_test.go` (`e2eStubCategoryGate` + arg `CategoryWriteGate` em `NewCreateTransaction`; seed real de root+leaf; versão editorial lida do DB).
- **Regressão validada com LLM real + Postgres:** `TestE2E1_RegistrarDespesaViaLLMPersisteNoBanco` (CA-01) PASS; `TestRealLLM_CardPurchaseChain_ResolveClassifyRegister` PASS; `TestE2E2_ReprocessarMesmoWamidNaoDuplica` PASS.

### F-02 — [major] `fixed` — income/recorrência exigem subcategoria folha no app layer
- **Origem:** finding F-02 + decisão de produto (income exige folha).
- **Correção:** `helpers.go` `guardSubcategoryRequired` passa a exigir subcategoria para ambas as direções; `errors.go` renomeia `ErrOutcomeTransactionRequiresSubcategory` → `ErrTransactionRequiresSubcategory`; `create_recurring_template.go`/`update_recurring_template.go` passam a chamar o guard; `error_mapper.go` atualizado.
- **Regressão:** `TestExecute_IncomeWithoutSubcategory_ReturnsValidationError` em Create/Update Transaction e Create/Update RecurringTemplate.

### F-03 — [major] `fixed` — sentinelas mortos
- **Origem:** finding F-03.
- **Correção:** `approveUpdateCategory` (nil subID) retorna `ErrCategoryEvidenceRequired` tipado (defesa antes do banco); `ErrCategoryNeedsClarification` (zero refs) removido de `category_write_evidence.go`.

### F-04 — [minor] `fixed` — snapshots vazios aceitos pelo banco
- **Origem:** finding F-04 (techspec §Persistência: "snapshots não estão vazios").
- **Correção:** CHECK `length(...) > 0` em `category_name_snapshot`/`subcategory_name_snapshot` nas duas tabelas (`transactions` e `transactions_recurring_templates`) em `migrations/000001_initial_schema.up.sql`.
- **Regressão (integração):** casos de rejeição para snapshot vazio nas duas tabelas.

### F-05 — [minor] `fixed` — `ExpectedVersion<=0` burlava drift no gate
- **Origem:** finding F-05 (RF-16/CA-15).
- **Correção:** `category_write_gate_adapter.go` `resolveExpectedVersion`: só `manual_canonical_id` lê versão atual; sources agentivos com `ExpectedVersion<=0` retornam `ErrCategoryVersionChanged`.

### F-06 — [minor] `fixed` — matriz DB incompleta para templates recorrentes
- **Origem:** finding F-06 (techspec "mesma matriz").
- **Correção (integração):** `TestCategoryWriteGateRecurringTemplates` estendido com version drift, deprecated root/leaf, direction↔kind, kind-column-drift, category-is-leaf, snapshots vazios e `direction=3`.

### F-07 — [minor] `fixed` — fidelidade de round-trip da evidência não asserida
- **Origem:** finding F-07 (RF-30/CA-16).
- **Correção (integração):** `TestCreateAndGetByID` (transaction + recurring) assere readback de `Evidence()` (score, confidence, quality, signalType, matchedTerm, matchReason, editorialVersion, source, path, kind, decidedAt).

### F-08 — [minor] `fixed` — preservação canônica só provada contra mocks
- **Origem:** finding F-08 (RF-01/RF-35).
- **Correção (integração):** `TestResolveAndValidateFullFields` valida campos canônicos (kind, ids, parent, names, version) contra Postgres real.

### L-02 — [low] `fixed` — `direction` sem CHECK de domínio
- **Correção:** CHECK `direction IN (1,2)` em `transactions` e `transactions_recurring_templates`; regressão com `direction=3` rejeitado.

### L-05 — [low] `fixed` — tooling de lint
- **Correção:** validação executada com golangci-lint v2 (2.12.2 em `/opt/homebrew/...`), reproduzindo a config v2 do repo. Surgiram e foram corrigidos 4 issues no código agentivo do feature que a tarefa 8.0 (rodando o binário v1 quebrado) nunca detectou: goimports em `register_entry.go` e `categories_reader_adapter.go`; `unconvert` em `register_entry.go` (`string(result.Outcome)`); `unused` type em `classify_category_test.go`. Também `function-length` (revive) nos dois use cases de recorrência resolvido por extração de helper; goimports em `raw_create_transaction.go`.

## Validação Final

| Gate | Comando | Resultado |
|---|---|---|
| Build | `go build ./...` | ✅ 0 |
| Vet | `go vet ./internal/{categories,transactions,agents}/...` | ✅ clean |
| Unit (race) | `go test -race ./internal/{categories,transactions,agents}/...` | ✅ 1169 passed |
| Integração Postgres | `go test -tags integration ./internal/transactions/.../postgres/... ./migrations/...` | ✅ 80 passed |
| Real-LLM E2E (agents) | `RUN_REAL_LLM=1 go test -tags integration -run 'MeControlaAgentE2ESuite|RealLLM|CardPurchaseChain'` | ✅ ok (37s, Postgres+LLM reais) |
| Real-LLM scorers | idem `./scorers/...` | ✅ ok (55s) |
| golangci-lint v2 | `golangci-lint run ./internal/{transactions,categories,agents}/...` | ✅ 0 issues |
| Gates governança | zero-comentários / SQL-em-adapter / LLM-no-write-path / cardinalidade | ✅ clean |

## Estado Final
`done` — 8 findings + 2 low corrigidos na causa raiz, regressões adicionadas (unit + integração + real-LLM), validação completa verde.
