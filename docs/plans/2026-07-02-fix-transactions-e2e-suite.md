# Plano — Corrigir a suíte `internal/transactions/e2e` (falhas pré-existentes)

- **Data:** 2026-07-02
- **Autor:** revisão automatizada (Claude) + evidência de execução Docker
- **Escopo:** `internal/transactions/e2e/`, `internal/transactions/infrastructure/http/server/`, wiring do módulo transactions
- **Fora de escopo:** `internal/card` (feature `simplificacao-card-melhor-dia-compra` já mergeada e verde), regras de domínio de outros módulos
- **Skill obrigatória:** `.agents/skills/go-implementation/SKILL.md` (Etapas 1–5, R0–R7) — toda edição Go/SQL passa por ela. Testes seguem `.claude/rules/go-testing.md` quando aplicável.

> **Premissa de rastreabilidade:** estas falhas são **pré-existentes** e independentes da entrega de card
> (transactions produção é zero-diff; a migration 000002 não toca tabelas de transactions/categories).
> Confirmado ao executar `go test -tags=e2e ./internal/transactions/e2e/...` com Docker/testcontainers.

---

## 1. Estado atual (evidência de execução)

`go test -count=1 -tags=e2e ./internal/transactions/e2e/...` → **FAIL** (`TestE2ETransactions`), com 3 assinaturas distintas de falha:

| # | Assinatura | Ocorrências |
|---|-----------|-------------|
| RC-1 | `transactions: outcome exige subcategory_id` (setup de transação e materialização de recorrência retornam 400) | 7 (5 setup + 1 setup + 1 lista) + 2 materialize |
| RC-2 | `atualizar transação inexistente`: esperado **404**, recebido **500** (`erro interno`) | 1 |
| RC-3 | `fatura do cartão após card-purchase`: esperado **200**, recebido **404** (`404 page not found`) | 1 |

Total: **10 cenários** falhando, agrupados em 3 causas-raiz.

---

## 2. Causa-raiz por categoria (com `file:line`)

### RC-1 — Outcome exige `subcategory_id`, mas os payloads do e2e nunca o enviam

- **Regra de domínio:** `internal/transactions/application/usecases/create_transaction.go:69`
  ```go
  if cmd.Direction == valueobjects.DirectionOutcome && !cmd.SubcategoryID.IsPresent() {
      return output.Transaction{}, ErrOutcomeTransactionRequiresSubcategory
  }
  ```
  Mapeada para **400** em `error_mapper.go:33`.
- **Payloads do e2e enviam só `category_id`**, sem `subcategory_id`:
  - `internal/transactions/e2e/steps_transactions_test.go` — `oUsuarioCriaUmaTransacao` (~L38-47), `queExisteUmaTransacaoCriada` (~L73-82), `existemNTransacoesCriadasParaOUsuario` (~L106-118), `oUsuarioAtualizaATransacao` (~L136-158), `oUsuarioTentaAtualizarTransacaoComIDInexistente` (~L181-190).
  - `internal/transactions/e2e/steps_jobs_test.go:25-36` e `steps_recurring_templates_test.go` (L37, L80, L126, L159, L186) — templates de recorrência **outcome** sem subcategoria → falham na **materialização** (`materialize_recurring_for_day.go`), pois a transação materializada também é outcome.
- **Taxonomia já semeada por migration** (`migrations/000001_initial_schema.up.sql`):
  - Categoria raiz `prazeres` = `ac535261-4060-56ef-b2e8-57c8cc7032d1` (L1110) — já usada como `txE2EPrazerosRootCategoryUUID` (`suite_test.go:33`).
  - Subcategoria `outros-prazeres` = `0016763e-655c-571a-90cb-bec5a18d4969` (L1198), filha de `prazeres`. **Disponível, basta referenciar.**
- **Conclusão:** os cenários outcome precisam enviar `subcategory_id = 0016763e-...`. Nenhuma mudança de produção necessária para RC-1 — é ajuste de dados de teste.

### RC-2 — `PATCH` de transação inexistente retorna 500 em vez de 404

- **Fluxo:** `internal/transactions/application/usecases/update_transaction.go`
  - L69–74: `categoryValidator.Validate(cmd.CategoryID, catSubID)` roda **antes** da verificação de existência.
  - L82–85: `repo.GetByID(...)` → não encontrado → `interfaces.ErrTransactionNotFound` (`transaction_repository.go` GetByID, ramo `len==0`), **mapeado para 404** em `error_mapper.go:21-29`.
- **Payload do cenário** (`steps_transactions_test.go:181-190`): `direction=outcome`, `category_id=prazeres (raiz)`, **sem `subcategory_id`**.
  - `update_transaction` **não** possui a guarda "outcome exige subcategory" do create; então chega em `categoryValidator.Validate(categoriaRaiz, nil)`. Essa validação (categoria raiz usada como folha, sem subcategoria) retorna um erro que **não está no `error_mapper`** → cai no `default` → **500** (`error_mapper.go:57`).
- **Conclusão:** dupla natureza —
  1. **Dado de teste:** o cenário deve enviar `subcategory_id` válido (mesma correção de RC-1); com subcategoria válida, `Validate` passa e o `GetByID` de UUID aleatório retorna 404.
  2. **Robustez (defense-in-depth, opcional mas recomendado):** garantir que erros de validação de categoria/subcategoria (categoria inexistente, subcategoria não pertencente à categoria, categoria raiz usada indevidamente) mapeiem para **400/404**, nunca 500. Requer identificar o erro concreto retornado por `categoryValidator.Validate` e adicioná-lo ao `error_mapper.go`.

### RC-3 — Rota de fatura do cartão não registrada (`404 page not found`)

- **Cenário** (`steps_card_invoice_test.go:75`): `GET /api/v1/cards/{cardID}/invoices/{refMonth}`.
- **Router transactions** (`transactions_router.go:88-133` `Register`) registra **apenas**: `/api/v1/transactions`, `/api/v1/card-purchases`, `/api/v1/recurring-templates`, `/api/v1/months`. **Não há** rota `/api/v1/cards/{id}/invoices/...`.
- Existe `handlers/get_card_invoice_handler.go` **órfão** — o handler não está no struct `TransactionsRouter`, não é recebido por `NewTransactionsRouter`, e não é registrado.
- A rota de fatura do **módulo card** tem forma diferente: `GET /cards/{id}/invoices?for=YYYY-MM-DD` (query param), não `/invoices/{refMonth}` (path param). O e2e transactions monta só o router de transactions.
- **Conclusão:** requer **decisão de design** (ver Seção 4, D-1): (a) wire do handler órfão `get_card_invoice_handler` no router transactions com o path esperado; ou (b) corrigir o e2e para consumir o endpoint canônico de fatura (do módulo card) e montar o card router no servidor de teste; ou (c) remover o cenário se a responsabilidade de fatura saiu do transactions.

---

## 3. Estratégia de correção

Ordem sugerida: **RC-1 → RC-2 → RC-3**. RC-1 e RC-2 compartilham a correção de dados de teste (subcategoria) e destravam 8/10 cenários. RC-3 é isolado e depende de decisão.

### Tarefas

| # | Tarefa | Arquivos | Tipo |
|---|--------|----------|------|
| 1.0 | Adicionar constante `txE2EOutrosPrazeresSubcategoryUUID = "0016763e-655c-571a-90cb-bec5a18d4969"` em `suite_test.go` | `internal/transactions/e2e/suite_test.go` | test |
| 2.0 | Incluir `"subcategory_id": txE2EOutrosPrazeresSubcategoryUUID` em **todos** os payloads outcome: transações (`steps_transactions_test.go`), templates de recorrência (`steps_recurring_templates_test.go`, `steps_jobs_test.go`) | 3 arquivos e2e | test |
| 3.0 | Rodar e2e; confirmar RC-1 (7+2) e RC-2 (1) verdes. Se RC-2 ainda 500, capturar o erro de `categoryValidator.Validate` e mapear em `error_mapper.go` (→ 400/404) | `error_mapper.go` (se necessário) | prod (condicional) |
| 4.0 | **Decisão D-1** sobre RC-3, então: (a) wire `get_card_invoice_handler` no `TransactionsRouter` (struct + `NewTransactionsRouter` + `Register` + `module.go`) no path `GET /api/v1/cards/{id}/invoices/{ref_month}`; **ou** (b) ajustar o cenário/servidor de teste ao endpoint canônico de fatura | `transactions_router.go`, `module.go`, handler; ou `steps_card_invoice_test.go` + `suite_test.go` | prod ou test |
| 5.0 | Rodar suíte e2e transactions completa + `go build`, `golangci-lint` (v2), unit de transactions; garantir 0 regressão e RF-14 (produção de transactions sem mudança de contrato de leitura) | — | validação |

### Detalhe RC-1/RC-2 (Tarefas 1.0–3.0)

1. Constante da subcategoria no bloco `const (...)` de `suite_test.go` (junto de `txE2EPrazerosRootCategoryUUID`).
2. Em cada `payload := map[string]any{ ... "category_id": ..., }` de **outcome**, acrescentar `"subcategory_id": txE2EOutrosPrazeresSubcategoryUUID`. Não alterar cenários `income` (não exigem subcategoria).
3. Reexecutar. Esperado: os 7 setups e os 2 materialize passam; o update-inexistente passa a 404. Caso o 500 persista, é sinal de erro não mapeado no `error_mapper` — adicionar o caso, preservando o gate de cardinalidade e sem regra de negócio no adapter (R-ADAPTER-001).

### Detalhe RC-3 (Tarefa 4.0)

- Investigar a intenção de `get_card_invoice_handler.go` (existe → provável wiring esquecido) e o contrato esperado (path `/{ref_month}`).
- **Se transactions deve expor a fatura por ref_month:** adicionar campo `getCardInvoice *handlers.GetCardInvoiceHandler` ao struct, parâmetro em `NewTransactionsRouter`, wiring em `internal/transactions/module.go`, e no `Register`:
  ```go
  g.Route("/api/v1/cards/{id}/invoices", func(sub chi.Router) {
      sub.Get("/{ref_month}", rt.getCardInvoice.Handle)
  })
  ```
  (validar contra `router_test.go:TestRouterRegistersAllTransactionRoutes` e atualizar o teste de rotas).
- **Se a fatura pertence ao módulo card:** ajustar o cenário para o endpoint canônico e montar o card router no servidor de teste (o `cardModule` já é construído em `suite_test.go`).

---

## 4. Decisões pendentes

- **D-1 (RC-3):** a fatura de cartão por `ref_month` é responsabilidade do **módulo transactions** (wire do handler órfão) ou do **módulo card** (`/invoices?for=`)? Esta decisão define se a Tarefa 4.0 é fix de produção (wiring) ou fix de teste (endpoint/montagem). **Recomendação:** inspecionar `get_card_invoice_handler.go` + techspec de transactions; se o handler foi escrito deliberadamente, opção (a) (wire) é o caminho de menor surpresa.

---

## 5. Validação (gates obrigatórios)

Executar no escopo alterado, proporcional ao risco:

```bash
go build ./...
go vet -tags "integration e2e" ./internal/transactions/...
golangci-lint run ./internal/transactions/...            # usar binário v2 (config .golangci.yml é v2)
go test -race -count=1 ./internal/transactions/...        # unit
go test -count=1 -tags=e2e -timeout=15m ./internal/transactions/e2e/...   # Docker/testcontainers
```

Critério de pronto: **suíte e2e de transactions verde (0 falhas)**, build/lint/unit verdes, e **produção de transactions sem mudança de contrato de leitura de cartão** (RF-14 da entrega de card permanece intacta).

> Nota de ambiente: o `golangci-lint` do PATH pode ser v1 (incompatível com o `.golangci.yml` v2). Usar o
> binário v2 (`golangci-lint 2.x`). O pre-commit hook precisa do v2 no PATH para não falsear falha.

---

## 6. Riscos e mitigação

- **Cardinalidade / R-TXN-004:** ao mexer no `error_mapper`, não introduzir labels de alta cardinalidade nem regra de negócio no adapter.
- **RF-14:** RC-2 (opcional) e RC-3 (opção a) tocam **produção** de transactions. Manter o **contrato de leitura de cartão** inalterado; qualquer novo endpoint é adição, não mudança de contrato existente.
- **Materialização de recorrência:** validar que templates outcome com subcategoria materializam corretamente (o job `materialize_recurring_for_day` propaga `SubcategoryID` do template — `L184-186, L214-216`).
- **Teste de rotas:** se wire de RC-3, atualizar `router_test.go` para incluir a nova rota, senão o teste de "todas as rotas registradas" quebra.
- **Ordem de execução do e2e:** cenários compartilham seed de usuário/categoria; garantir que a subcategoria referenciada existe no seed da migration antes de qualquer POST.

---

## 7. Estimativa

- RC-1 + RC-2 (dados de teste + eventual mapeamento de erro): **pequeno** (1 constante + edições de payload + 1 reexecução; +mapeamento condicional).
- RC-3 (wiring ou fix de teste): **pequeno-médio**, dependente de D-1.
- Total: uma sessão focada, sem mudança de esquema nem de contrato de leitura.
