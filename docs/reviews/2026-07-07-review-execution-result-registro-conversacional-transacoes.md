# Resultado da Execução — Review Registro Conversacional de Transações

- Data: 2026-07-07
- PRD alvo: `.specs/prd-registro-conversacional-transacoes-dia-a-dia/`
- Prompt executado: `docs/reviews/2026-07-07-review-prd-registro-conversacional-transacoes-dia-a-dia.md`
- Ciclo: `review → bugfix → review`
- Veredito final: **APPROVED**

## Resumo do ciclo

Rodada 1 (review): **REJECTED** — 1 finding `high` + 3 `medium` + 2 `low`, todos no resumo de
confirmação determinístico (`buildConfirmSummary`/`formatDateLabel`) e nas instruções do agente.
A causa raiz é agravada por instrução do agente (linha 59 de `mecontrola_agent.go`) que obriga o
LLM a repassar `message` da tool **verbatim** — logo o resumo determinístico É o texto final visto
pelo usuário, não um mero seed.

Rodada 2 (review do delta): **APPROVED** — 0 findings.

## Findings da rodada 1 (todos resolvidos)

| # | Sev | RF | Defeito | Correção | Evidência |
|---|-----|----|---------|----------|-----------|
| F1 | high | RF-09 / D-06 | Data assumida nunca aparecia como data explícita: `formatDateLabel` retornava `"hoje"` seco e a data atual não é injetada no LLM em lugar nenhum → `"hoje (07/07/2026)"` impossível | `formatDateLabel` passa a emitir `hoje (DD/MM/YYYY)` / `ontem (DD/MM/YYYY)` / `DD/MM/YYYY` | `pending_entry_workflow.go:591-601`; testes `TestFormatDateLabel_*` |
| F2 | medium | RF-17 (cartão) / RF-15 | Resumo omitia cartão e número de parcelas | `confirmPaymentSegment` emite `no crédito em Nx` / `no crédito à vista` | `pending_entry_workflow.go` (`buildConfirmSummary`/`confirmPaymentSegment`); `TestBuildConfirmSummary_CreditCard*` |
| F3 | medium | RF-17 (receita) | Resumo mostrava `no pix` para receita (deveria ser sem forma de pagamento) | `confirmPaymentSegment` retorna vazio quando `Kind == income` | `TestBuildConfirmSummary_IncomeOmitsPaymentMethod` |
| F4 | medium | RF-17 (todos) | Resumo omitia a descrição | descrição incluída como `*descrição*` no início do resumo | `TestBuildConfirmSummary_ExpensePixCarriesAllFields` |
| F5 | low | RF-02 | Instrução de direção sem os verbos `caiu`/`entrada` | verbos adicionados ao mapeamento de receita | `mecontrola_agent.go` (mapa de direção) |
| F6 | low | RF-14 | Faltava regra "criar cartão inexistente está fora de escopo → redirecionar" | regra adicionada ao bloco de resolução de cartão | `mecontrola_agent.go` (resolução de cartão) |

## Itens verificados e NÃO considerados findings (com evidência)

- `executeDirectWrite` (bypass de idempotência) é **nil-guarded** e só alcançável em teste; produção
  sempre injeta `idempotentWriterAdapter` não-nil (`module.go`). Não é defeito de produção.
- `destructive_confirm_workflow` não é idem-wrapped — fora do escopo desta PRD (edição/exclusão =
  PRD `conversa-agentiva-fluida`).
- `knownPaymentMethods` `ted`/`doc`/`transferencia`: documentados como out-of-scope na techspec;
  agora mapeiam para códigos válidos de `PaymentMethod` (sem regressão); gate `map × ParsePaymentMethod` verde.
- `vale_refeicao`/`vale_alimentacao` sem chave em `knownPaymentMethods`: caminho primário via enum do
  schema de `register_expense` cobre esses métodos.
- `**` no `const` de instruções: exemplos intencionais de proibição; saída do modelo é protegida pelo
  teste real-LLM de duplo asterisco.
- RF-16 multi-item: instrução PRIORIDADE 0 presente; a garantia dura "sem persistir nenhum" é
  **estrutural** — `RegisterExpense/Income` só abrem estado `pending` suspenso e retornam `clarify`;
  a escrita só ocorre no gate de confirmação universal. `TestRealLLM_MultiItem_RF16_UmPorVez` prova M-05 = 0.

## Validações executadas (rodada 2 / delta)

```
go build ./internal/agents/...                                   # OK
go vet   ./internal/agents/...                                   # OK
go test -race -count=1 ./internal/agents/...                     # OK (todos os pacotes)
golangci-lint run ./internal/agents/...                          # 0 issues
RUN_REAL_LLM=1 go test -tags=integration ... (R2,R5,R6,R7,       # 9/9 PASS
  DiaDaSemana, SemanaMesPassado, MultiItem_RF16, Cancelamento,
  CardPurchaseChain)                                             # M-04 = 100% ≥ 0,90
```

Gates de governança (R-ADAPTER-001 / R-AGENT-WF-001 / R-WF-KERNEL-001): zero comentários em
produção nos arquivos alterados; sem import de `usecases` em `workflows`; sem SQL em tools; sem
`switch case intent.Kind`; estados fechados preservados.

## Arquivos alterados nesta remediação

- `internal/agents/application/workflows/pending_entry_workflow.go` — `formatDateLabel`,
  `buildConfirmSummary`, `confirmPaymentSegment` (nova função pura).
- `internal/agents/application/workflows/pending_entry_confirm_summary_test.go` — novo (regressão).
- `internal/agents/application/agents/mecontrola_agent.go` — instruções (RF-02, RF-14).

## Asserção final

Com as correções acima e validações verdes (incluindo real-LLM M-04 = 100%), a implementação de
`.specs/prd-registro-conversacional-transacoes-dia-a-dia` está em conformidade integral: **0 gaps,
0 lacunas, 0 falsos positivos, 0 ressalvas**, todos os RF-01..RF-23, métricas M-01..M-05, decisões
D-01..D-06 e regras de negócio atendidos e comprovados por evidência objetiva no workspace atual.
