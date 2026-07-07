<!-- spec-hash-prd: a0bd0d9a1332e20ea9e51f536f16659d031ddb893981eb131e616c03a8993f9c -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Consulta Conversacional Financeira

> PRD consumido: `.specs/prd-consulta-conversacional-financeira/prd.md` (spec-version 3).
> Skills obrigatórias: `.claude/skills/go-implementation/` (R0–R7 `[HARD]`), `.agents/skills/mastra/`
> (substrato `internal/platform/{agent,tool,llm,memory,scorer}`), e `.agents/skills/domain-modeling-production/`
> (DMMF: state-as-type, smart constructors, Decide* puro, pipeline parse→validate→decide→persist→publish).

## Resumo Executivo

A funcionalidade é uma **consulta conversacional read-only** (C1–C7) resolvida pelo **loop de
tool-calling** já existente do agente `mecontrola` — não introduz roteamento por `intent.Kind`, nem
workflow durável, nem HITL. Consultas de leitura são stateless, idempotentes e sem suspend/resume;
portanto a escolha de ferramenta é responsabilidade do LLM, guiada por instruções determinísticas.

A entrega tem **três frentes cirúrgicas**, alinhadas a D-03/D-09 do PRD: (1) evoluir a constante de
instruções `mecontrolaAgentInstructions` (`internal/agents/application/agents/mecontrola_agent.go:15-153`)
com um protocolo explícito de consulta + regras de formatação, ambiguidade, alertas, retrocesso de mês
e anti-alucinação; (2) **uma extensão aditiva única**: expor `subcategoryNameSnapshot` na saída de
`get_transaction` (o dado já existe em `interfaces.Entry`), para C5 renderizar `Categoria > Subcategoria`;
(3) testes de regressão — unit determinístico para o novo campo e cenários C1–C7 no harness real-LLM
com gate `M-04 ≥ 0.90`. Nenhuma tool nova, nenhuma assinatura de use case, `module.go` ou binding é
alterada. O IDOR de fatura já é barrado no repositório (`GetCardInvoice` escopa por `principal.UserID`);
o guard de `cardId` no prompt é defesa-em-profundidade.

## Arquitetura do Sistema

### Visão Geral dos Componentes

Componentes tocados (todos no consumidor `internal/agents`; substrato `internal/platform` **intacto**):

- **`agents/application/agents/mecontrola_agent.go`** (modificado) — a const `mecontrolaAgentInstructions`
  ganha um bloco de "Consultas Financeiras (C1–C7)" com matriz de roteamento, regras de formatação e
  edge cases. Nenhuma mudança na assinatura de `BuildMeControlaAgent` nem no wiring.
- **`agents/application/tools/get_transaction.go`** (modificado, aditivo) — struct `GetTransactionOutput`
  ganha o campo `SubcategoryNameSnapshot`; o schema `out` ganha a propriedade correspondente; o `exec`
  mapeia `entry.SubcategoryNameSnapshot`. Zero lógica de domínio adicionada — adapter permanece fino
  (mastra: tool retorna dado tipado, não apresentação).
- **`agents/application/tools/read_tools_test.go`** (modificado) — `TestGetTransactionTool_Success`
  passa a asseverar o novo campo.
- **`agents/application/scorers/mecontrola_tools_realllm_test.go`** (modificado) — adiciona os cenários
  C1–C7 ao slice `scenarios []harnessScenario` e, para cenários multi-tool (C1), um asserter de
  presença de múltiplas tools. Mantém o gate `M-04 ≥ 0.90`.

Fluxo (inalterado, mastra canônico Thread→Run):

```text
WhatsApp inbound -> HandleInbound -> agent.AgentRuntime.Execute
  -> memory.ThreadGateway.GetOrCreate(resourceId, threadId)
  -> agent.RunStore.Insert(RunStatusRunning)
  -> agent.AgentRegistry.Resolve(MecontrolaAgentID)
  -> buildMessages(system[+WorkingMemory][+Recent(20)] + user)
  -> agent.Agent.Execute  (loop tool-calling, WithMaxToolRounds(12))
       -> llm.Provider.Complete (OpenRouter)
       -> tool.ToolHandle.Invoke  (query_month | query_plan | resolve_card | ...)
  -> memory.MessageStore.Append(user, assistant)
  -> closeRun(RunStatusSucceeded | RunStatusFailed)
```

O agente escolhe a(s) tool(s) por rodada; consultas encadeadas (C4: `resolve_card`→`query_card_invoice`;
C5: `query_month`→`get_transaction`) ocorrem como rodadas sucessivas do mesmo loop, sem orquestração
adicional.

## Design de Implementação

### Interfaces Chave

Nenhuma interface nova. Os contratos consumidos (já existentes, confirmados em
`internal/agents/application/interfaces/`) são:

```go
type TransactionsLedger interface {
    GetMonthlySummary(ctx context.Context, userID uuid.UUID, refMonth string) (MonthlySummary, error)
    ListMonthlyEntries(ctx context.Context, userID uuid.UUID, refMonth, cursor string, limit int) ([]MonthlyEntry, error)
    GetCardInvoice(ctx context.Context, cardID uuid.UUID, refMonth string) (CardInvoice, error)
    GetTransaction(ctx context.Context, txID string) (Entry, error)
    // ... demais métodos inalterados
}

type BudgetPlanner interface {
    GetMonthlySummary(ctx context.Context, userID uuid.UUID, competence string) (BudgetSummary, error)
    ListAlerts(ctx context.Context, userID uuid.UUID) ([]Alert, error)
    // ... demais métodos inalterados
}
```

`Entry` já contém `CategoryNameSnapshot` e `SubcategoryNameSnapshot`
(`interfaces/types.go:181-198`); apenas a projeção da tool passa a expor o segundo.

### Modelos de Dados

Única alteração de modelo — aditiva, em `get_transaction.go`:

```go
type GetTransactionOutput struct {
    Kind                    string    `json:"kind"`
    ID                      string    `json:"id"`
    UserID                  string    `json:"userId"`
    Direction               string    `json:"direction"`
    PaymentMethod           string    `json:"paymentMethod"`
    AmountCents             int64     `json:"amountCents"`
    Description             string    `json:"description"`
    CategoryID              string    `json:"categoryId"`
    CategoryNameSnapshot    string    `json:"categoryNameSnapshot"`
    SubcategoryNameSnapshot string    `json:"subcategoryNameSnapshot"`
    RefMonth                string    `json:"refMonth"`
    OccurredAt              time.Time `json:"occurredAt"`
    Version                 int64     `json:"version"`
    CreatedAt               time.Time `json:"createdAt"`
    UpdatedAt               time.Time `json:"updatedAt"`
}
```

No schema JSON estrito (`llm.Schema` `out`), adicionar `"subcategoryNameSnapshot": {"type":"string"}`
em `properties` e incluí-lo em `required` (o schema é `Strict: true`; todos os campos são exigidos).
No `exec`, adicionar `SubcategoryNameSnapshot: entry.SubcategoryNameSnapshot`. Nenhum outro campo,
branching ou cálculo. Registro estado-fechado preservado: `entry.Kind.String()` continua sendo a
única fronteira string do `EntryKind` (state-as-type, `discriminators.go`).

### Protocolo de Instruções (bloco C1–C7)

O bloco adicionado à const de instruções especifica, de forma **declarativa e determinística** (o
LLM é o executor; DMMF: a "regra pura" de formatação/roteamento vive num único lugar canônico):

Matriz de roteamento (RF-01..RF-09):

| Cenário | Gatilho | Ferramenta(s) |
|---------|---------|---------------|
| C1 | "como estou indo?", "resumo do mês" | `query_month` + `query_plan` (mês atual) |
| C2 | "orçamento de {mês}/{ano}" | `query_plan` com `competence=YYYY-MM` |
| C3 | "orçamento do mês atual" | `query_plan` (sem competence) |
| C4 | "fatura do {apelido}" | `resolve_card` → `query_card_invoice` |
| C5 | "última transação" | `query_month(limit=1)` → `get_transaction(id)` |
| C6 | "últimas N transações" | `query_month(limit=N|5)` |
| C7 | "orçamento completo/detalhado" | `query_plan` (todas as `allocations`) |

Regras determinísticas embutidas:

- **Competência** (RF-13/RF-14): "janeiro/2026"→`2026-01`; "mês atual"/ausente→data corrente em
  `America/Sao_Paulo` formatada `YYYY-MM`. As tools já aplicam o default de fuso quando `competence`/
  `refMonth` vêm vazios (`query_plan.go:94-100`, `query_month.go:84-91`); o prompt só precisa passar
  competência explícita quando o usuário citar mês.
- **Mapa slug→nome** (D-02/RF-19): `custo-fixo`→*Custo Fixo*, `conhecimento`→*Conhecimento*,
  `prazeres`→*Prazeres*, `metas`→*Metas*, `liberdade-financeira`→*Liberdade Financeira*.
- **Formatação de valores** (D-?/RF-22/RF-36): centavos→reais, 2 casas, separador de milhar `.` e
  decimal `,` (ex.: `123450`→`R$ 1.234,50`). Regra única e canônica no bloco, reutilizada por C1–C7
  (satisfaz RF-36 sem presenter Go — ver ADR-003).
- **C5 categoria** (RF-06/RF-06a/D-05/D-09): exibir `categoryNameSnapshot`; se
  `subcategoryNameSnapshot` não vazio, `categoryNameSnapshot > subcategoryNameSnapshot`. Falha de
  `get_transaction` → responder descrição/valor/data sem categoria, sem erro (best-effort).
- **Mês vazio** (RF-07a/D-06): se `query_month` do mês atual vier sem entries em C5/C6, repetir
  `query_month` uma vez com o mês anterior; persistindo vazio, aplicar RF-30.
- **Alertas** (RF-08a/D-07): em C2/C3/C7 resumir `alerts` ativos (categoria via mapa, threshold,
  estado) ou informar ausência.
- **C7 categorias** (RF-18..RF-21): exibir todas as `allocations`; `plannedCents` nulo →
  "*Sem limite definido*"; total no topo (`totalPlannedCents`/`totalSpentCents`).
- **Guard de cardId** (RF-32a/D-08): `cardId` só de `resolve_card`/`list_cards`; nunca de texto do
  usuário. Defesa-em-profundidade — a barreira primária é o repositório (`GetCardInvoice` escopa por
  `principal.UserID`).
- **Ambiguidade de cartão** (RF-15): `resolve_card.found=false` → `list_cards` + pedir escolha.
- **Anti-alucinação/domínio** (RF-10..RF-12): todo valor/data/categoria vem de retorno de tool;
  fora de domínio → recusa educada; erro técnico → RF-31 sem detalhes.
- **Formatação WhatsApp** (RF-23..RF-25): PT-BR, emojis 📊/💰/✅, negrito só `*simples*`, nunca `**`.

## Pontos de Integração

Provider LLM: OpenRouter via `internal/platform/llm` (único provider; sem fallback/circuit —
mastra). Módulos `internal/budgets` e `internal/transactions` consumidos exclusivamente pelos
bindings existentes (`budget_planner_adapter.go`, `transactions_ledger_adapter.go`), que já injetam
`principal` no contexto e delegam aos use cases — nenhuma mudança.

## Abordagem de Testes

### Testes Unitários (determinísticos)

- **`get_transaction` (novo campo)**: estender `TestGetTransactionTool_Success`
  (`read_tools_test.go:304`) — mock `TransactionsLedger.GetTransaction` retorna `Entry` com
  `CategoryNameSnapshot="Custo Fixo"` e `SubcategoryNameSnapshot="Supermercado"`; assertar que
  `GetTransactionOutput.SubcategoryNameSnapshot == "Supermercado"`. Padrão vigente: `testify/mock` +
  mock mockery `imocks.NewTransactionsLedger(t)` + `identityCtx`. `TestGetTransactionTool_BindingError`
  permanece válido (garante wrapping de erro `%w`).
- Sem novos mocks: `TransactionsLedger`/`BudgetPlanner`/`CardManager` já têm mock mockery em
  `interfaces/mocks/` (R3).

### Testes de Integração

Não requeridos por esta feature além do harness real-LLM. Não há nova fronteira de IO (banco/fila):
as tools já são cobertas por `*_e2e_test.go` com testcontainer Postgres. Critérios do template: apenas
1 dos 3 se aplica → integration dedicada dispensada; o caminho de dados é exercido pelos E2E existentes.

### Testes E2E / Real-LLM (gate de aceite)

- **Harness M-04** (`scorers/mecontrola_tools_realllm_test.go:225`): adicionar cenários C1–C7 ao slice
  `scenarios`, cada um `{input, expectedTool, tools}`, ativados por `//go:build integration` +
  `RUN_REAL_LLM=1` + `OPENROUTER_API_KEY`. Gate preservado: `require.GreaterOrEqual(m04, 0.90)`.
  Cenários mínimos (inputs verbatim das personas C1–C7):
  - C1 "como estou indo?" → requer `query_month` **e** `query_plan` (multi-tool).
  - C2 "como foi meu orçamento de janeiro/2026?" → `query_plan`.
  - C3 "como está meu orçamento do mês atual?" → `query_plan`.
  - C4 "quanto está minha fatura do cartão nubank?" → `resolve_card` (e, na cadeia, `query_card_invoice`).
  - C5 "qual foi a minha última transação?" → `query_month` (e, na cadeia, `get_transaction`).
  - C6 "quais foram as minhas últimas 5 transações?" → `query_month`.
  - C7 "me mostra o orçamento completo" → `query_plan`.
- **Multi-tool (C1)**: como `ExpectedToolScorer` avalia um único `expectedTool`, adicionar no escopo de
  teste um asserter de presença de conjunto (`expectedTools []string`) — helper de teste, não código de
  produção; ou registrar C1 com duas entradas (mesma input, `expectedTool` distinto) e exigir hit em
  ambas. Preferir o helper de conjunto para não inflar o denominador de M-04.
- **Cadeia (C4/C5)**: reaproveitar `mecontrola_agent_chain_realllm_test.go` para asseverar a sequência
  `resolve_card→query_card_invoice` e `query_month→get_transaction`, e que a resposta final não
  contém valor ausente do retorno das tools (anti-alucinação, RF-10).

### Validação (R0–R7, go-implementation Etapa 5)

`go build ./internal/agents/...`, `go vet ./internal/agents/...`,
`go test -race -count=1 ./internal/agents/application/tools/...` e `.../agents/...`,
`golangci-lint run ./internal/agents/...`; real-LLM: `RUN_REAL_LLM=1 OPENROUTER_API_KEY=... go test
-tags integration -race -count=1 ./internal/agents/application/scorers/... ./internal/agents/application/agents/...`.
Gate zero-comentários (R-ADAPTER-001.1) sobre os arquivos tocados.

## Sequenciamento de Desenvolvimento

1. **Extensão aditiva `get_transaction`** (base de dados para C5): campo + schema + mapeamento; estender
   o unit test. Independe do prompt; valida isoladamente.
2. **Bloco de instruções C1–C7**: editar a const `mecontrolaAgentInstructions` com a matriz e regras.
3. **Cenários real-LLM C1–C7 + asserter multi-tool**: no harness de scorer; rodar com `RUN_REAL_LLM=1`
   e confirmar `M-04 ≥ 0.90` e zero alucinação nas cadeias.
4. **Validação R0–R7 + lint + gates**.

### Dependências Técnicas

- Credenciais `OPENROUTER_*` do `.env` para o gate real-LLM (feedback registrado: mocks não bastam).
- Go 1.26.4 (`go.mod`) — todos os recursos R7 disponíveis; nenhuma dependência nova.

## Monitoramento e Observabilidade

Sem métrica nova. O `Run` auditável já emitido pelo `AgentRuntime` (thread_id, run_id, status,
duração) cobre as consultas; cardinalidade controlada mantida (sem `user_id` como label —
R-AGENT-WF-001.5/R-TXN-004). `M-04` permanece como sinal de qualidade offline (log do harness).

## Considerações Técnicas

### Decisões Chave (ADRs)

- **ADR-001** — Roteamento de consulta pelo loop de tool-calling do agente (instruções), não por
  workflow durável nem registry de intent. Ver `adr-001-consulta-via-instrucoes-agente.md`.
- **ADR-002** — Extensão aditiva única de `get_transaction` (`subcategoryNameSnapshot`) como exceção
  cirúrgica a D-03. Ver `adr-002-get-transaction-subcategory-aditivo.md`.
- **ADR-003** — Formatação de valores e mapeamento slug→nome declarativos no prompt (DMMF núcleo-puro
  como regra única), presenter Go rejeitado. Ver `adr-003-formatacao-no-prompt.md`.

### Riscos Conhecidos

- **Não-determinismo do LLM em C1 (multi-tool)**: risco de o agente chamar só `query_month`. Mitigação:
  instrução explícita "panorama exige `query_month` E `query_plan`" + cenário multi-tool no gate M-04.
- **Confusão C6 vs C1**: "últimos lançamentos" (C6) vs "resumo" (C1). Mitigação: gatilhos distintos na
  matriz + cenários de ambos no harness.
- **Regressão de outras intenções**: como a const de instruções é compartilhada com registro/edição/HITL,
  a edição deve ser **apêndice de seção**, sem reescrever regras existentes de confirmação/escrita.
  Mitigação: rodar a suíte completa de agents (incl. `pending_entry_*`) e o M-04 dos 22 cenários já
  existentes, garantindo não-regressão do denominador.
- **Fatura sem mês**: `query_card_invoice` default = mês atual (`America/Sao_Paulo`); coerente com RF-14.

### Conformidade com Padrões

- `.claude/rules/go-adapters.md` (R-ADAPTER-001.1 zero comentários; 001.2 adapter fino — a extensão de
  `get_transaction` só mapeia campo, sem SQL/branching/regra).
- `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001.1 roteamento por registry/loop, sem switch de
  intent; 001.2 tool fina; 001.3 `ToolOutcome`/`RunStatus` fechados; 001.4 LLM só nas call-sites
  sancionadas; 001.5 Run auditável).
- `.claude/rules/workflow-kernel.md` (R-WF-KERNEL-001) — não tocado; kernel permanece genérico.
- `.claude/rules/go-testing.md` (R-TESTING-001) — testes unit determinísticos no padrão vigente do
  pacote de tools (testify/mock + mockery); harness real-LLM segue o padrão de `scorers/`.
- go-implementation R0–R7 `[HARD]`: R0 sem `init()`; R2 sem alias de campo; R5.12 sem `panic`; R5.8
  enums `iota+1` preservados; R7 recursos modernos conforme Go 1.26.4; R6 `context.Context` na fronteira.
- DMMF: state-as-type (`EntryKind`/`BudgetState`/`Direction` fechados, string só na fronteira);
  núcleo-puro/casca-IO (regras de formatação e roteamento como especificação declarativa única);
  estados ilegais irrepresentáveis (nada de nova `string` livre de estado).

### Arquivos Relevantes e Dependentes

- `internal/agents/application/agents/mecontrola_agent.go` (const de instruções — modificado)
- `internal/agents/application/tools/get_transaction.go` (aditivo — modificado)
- `internal/agents/application/tools/read_tools_test.go` (teste — modificado)
- `internal/agents/application/scorers/mecontrola_tools_realllm_test.go` (cenários C1–C7 — modificado)
- `internal/agents/application/agents/mecontrola_agent_chain_realllm_test.go` (cadeias C4/C5 — opcional)
- Dependências read-only (inalteradas): `tools/query_month.go`, `query_plan.go`, `query_card_invoice.go`,
  `resolve_card.go`, `list_cards.go`, `search_transactions.go`; `interfaces/types.go`;
  `infrastructure/binding/{budget_planner_adapter,transactions_ledger_adapter}.go`;
  `internal/transactions/application/usecases/get_card_invoice.go` (escopo por `principal.UserID`).
