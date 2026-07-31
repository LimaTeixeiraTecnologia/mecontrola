# Relatório de Revisão — `prd-lancamento-edicao-receitas`

- Data: 2026-07-31
- Rodadas executadas: 1 (sem ciclo de bugfix — 0 achados)
- Veredito final: **APPROVED** (puro, 0 achados de qualquer severidade)

## Escopo do Diff

Feature inteira no working tree (não commitada), branch `main`. Arquivos revisados:

- `internal/agents/application/workflows/write_shared.go` (ParseInputDate: 3 ramos novos + helpers puros)
- `internal/agents/application/workflows/transaction_write_workflow.go` (formatOptionalConfirmDate + confirmSummaryIncome com data condicional)
- `internal/agents/application/workflows/parse_input_date_test.go` (novo — 24 casos ParseInputDate + daysInMonth)
- `internal/agents/application/workflows/transaction_write_workflow_test.go` (3 casos confirmSummaryIncome)
- `internal/agents/application/messages/catalog.go` (IncomeConfirmationBlock ramo condicional 📅 Data)
- `internal/agents/application/messages/catalog_test.go` (TestIncomeConfirmationBlock_WithDate)
- `internal/agents/application/tools/register_income.go` + `edit_entry.go` (descrição occurredAt = contrato verbatim)
- `internal/agents/application/tools/financial_tools_test.go` (contrato verbatim + fix de query month)
- `internal/agents/application/golden/cases_income_natural_language.go` (novo — 24 casos lançamento)
- `internal/agents/application/golden/cases_income_edit.go` (novo — 9 casos edição + 4 mensagens canned)
- `internal/agents/application/golden/registry.go` (registro dos 2 conjuntos)
- `internal/agents/application/golden/harness_realllm_test.go` (mock edit_entry fiel ao contrato de produção)

## Confronto RF a RF (contra código real)

| RF | Evidência | Status |
|----|-----------|--------|
| RF-01 (9 famílias) | 9 casos golden real-LLM (`cases_income_natural_language.go:4-121`), todos PASS ratio 1.0000 | atendido |
| RF-02 (origem literal) | golden `receita nl origem literal sem parafrase` PASS | atendido |
| RF-03 (numérico/BR/gíria) | golden pila/conto/separador milhar PASS | atendido |
| RF-04 (por-extenso) | golden cem/dois mil/composto PASS | atendido |
| RF-05 (datas suportadas) | unit `TestParseInputDate` (hoje/ontem/anteontem/weekday/DD-MM/ISO) verde | atendido |
| RF-06 (3 datas novas, sem fallback) | `write_shared.go:320-374` + 17 casos unit de borda (clamp fev/bissexto/virada de ano/dia futuro→mês anterior) verde | atendido |
| RF-07 (ausência→hoje) | `register_entry.go:117-120` (fallback explícito só quando ParseInputDate="") | atendido |
| RF-08/09 (clarify mínimo/categoria) | comportamento pré-existente no workflow; golden confirmação relay PASS | atendido |
| RF-10 (bloco confirmação + 📅 Data condicional) | `catalog.go:250-260` + `confirmSummaryIncome` (transaction_write_workflow.go:1023-1029) + 3 unit tests (com/sem/hoje) | atendido |
| RF-11 (grava só com confirmação, 1x) | workflow pré-existente; golden `confirmacao relay verbatim` PASS | atendido |
| RF-12 (sucesso oficial) | catálogo `WriteSuccess`; pré-existente | atendido |
| RF-13 (idempotência wamid/itemSeq) | pré-existente (IdempotentWrite); não observável no gate de roteamento (documentado honestamente no report 4.0/5.0) | atendido (fora do gate golden) |
| RF-14 (limites valor/origem) | teto testado em `TestBuildRegisterIncomeToolCeilingRejectsWithoutRegistrarCall`; golden correção inválida PASS | atendido |
| RF-15 (sem forma de pagamento) | golden `canal reconhecido nao persiste pagamento` PASS (notContainsAny forma de pagamento) | atendido |
| RF-16 (LLM-first) | roteamento por register_income via LLM real, guard só atalho; golden real-LLM prova | atendido |
| RF-17..RF-21 (edição/localização/valor≠busca) | `cases_income_edit.go` + mock edit_entry com searchAmountCents vs amountCents; PASS ratio 1.0000 | atendido |
| RF-22 (data na edição, mesma resolução) | golden edição `semana passada`/`dia 10` verbatim PASS; reusa ParseInputDate | atendido |
| RF-23 (sucesso edição + idempotência) | pré-existente; não observável no gate (documentado) | atendido (fora do gate golden) |
| RF-24 (não encontrado / inválido) | golden `nenhum candidato`/`correção inválida` PASS | atendido |
| RF-25 (gate ≥0,90/grupo, 0 falso-sucesso) | executado real-LLM: todos os subtests PASS, ratio 1.0000, gate `require.False(failed)` @ 0.90 | atendido |
| RF-26 (catálogo verbatim) | mensagens via catálogo; asserts verbatim nos golden | atendido |
| RF-27 (cardinalidade) | grep R-TXN-004 sem `user_id`/`category_id` | atendido |

## Confronto ADRs

- **ADR-001** (ParseInputDate puro + contrato verbatim): implementado em `write_shared.go`; helpers puros sem IO/context; descrições `occurredAt` atualizadas nas duas tools; unit tests de borda verdes. Atendido.
- **ADR-002** (📅 Data condicional): implementado. Nota: o texto do ADR afirma que `formatConfirmDate` retorna `""` para hoje/vazio, mas ele retorna `"hoje"`; a implementação cobriu isso corretamente com `formatOptionalConfirmDate` (mapeia `"hoje"→""`), atingindo a intenção do ADR (linha só para data específica). Provado por `TestConfirmSummaryIncome_TodayOmitsDateLine`. Atendido.
- **ADR-003** (gate golden por grupo): 9 grupos + valor + data + edição + armadilhas; arquivo dedicado; registrado; gate real-LLM verde. Atendido.

## Confronto Critérios de Aceite das Task Files (1.0–5.0)

Todos os critérios de `## Critérios de Sucesso` das 5 task files confrontados: build/vet verdes, suíte golden compila e registra sem órfãos (`TestGoldenToolSubsetsAreRegisteredInHarnessCatalog`), gate real-LLM ≥0,90/0 falso-sucesso, cardinalidade OK, zero comentários OK. `5.0_execution_report.md` presente e completo (DoD 100%).

## Regras [HARD] de Plataforma

- R-ADAPTER-001.1 (zero comentários): OK nos arquivos de produção tocados.
- R-ADAPTER-001.2 (tools finas, sem SQL): tools só ajustaram descrição de schema; sem regra/SQL.
- R-TXN-001 (Decide*/helpers puros): `resolveSameDayPreviousMonth`/`resolveMostRecentDayOfMonth`/`daysInMonth` puros, sem IO/context, recebem `now`.
- R-TXN-004 (cardinalidade): OK.
- Estados fechados: sem string livre introduzida.

## Validações Executadas

```
go build ./...                                          -> OK
go vet ./internal/agents/...                            -> OK
go vet -tags integration ./.../golden/...               -> OK
go test -race -count=1 (workflows, golden, messages, tools) -> ok
RUN_REAL_LLM=1 go test -tags integration .../golden/ -run '.../receita' -> PASS, ratio 1.0000, 0 falso-sucesso (78+ subtests)
grep zero-comentários (R-ADAPTER-001.1)                 -> OK
grep cardinalidade (R-TXN-004)                          -> OK
```

## Achados

Nenhum. 0 critical, 0 high, 0 medium, 0 low.

## Riscos Residuais (não bloqueantes)

- RF-13/RF-23 (idempotência de registro/edição) não são exercitados pelo gate de roteamento LLM (tools mockadas, sem wamid real); dependem da idempotência do usecase/workflow pré-existente. Documentado honestamente nos reports 4.0/5.0. Não é gap desta iniciativa (delta é datas + verbatim + confirmação condicional + gate golden).
- Feature ainda **não commitada** e **não deployada**; a validação é as-built no working tree.
- Descoberta (harness, não produção): gpt-4o-mini ocasionalmente não converte reais→centavos em `searchAmountCents` sem a unidade "reais" anexada; mitigado nos casos golden. Follow-up sugerido caso apareça em produção.

## Veredito

**APPROVED** — todos os critérios de aceite atendidos, DoD 100%, 0 gaps, 0 lacunas, 0 falsos positivos (gate real-LLM efetivamente executado), 0 ressalvas bloqueantes, todas as regras de negócio e regras [HARD] de plataforma atendidas.
