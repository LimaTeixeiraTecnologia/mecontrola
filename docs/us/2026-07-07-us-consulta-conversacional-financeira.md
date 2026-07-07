# Consulta Conversacional Financeira — User Story

> Fonte: solicitação de evolução do agente `mecontrola` para atender consultas de fatura, orçamento e histórico de transações via WhatsApp de forma fluida, segura e sem alucinação.
> Objetivo da funcionalidade: permitir que o usuário consulte sua situação financeira atual e passada usando linguagem natural, com respostas sempre baseadas em dados reais dos módulos `internal/budgets` e `internal/transactions`, orquestradas pelo consumidor agentivo em `internal/agents`.

---

## US-01 — Consulta Conversacional Financeira

**Como** usuário do MeControla, **quero** conversar com o agente para consultar minha fatura, meu orçamento e minhas transações usando frases do dia a dia, **para que** eu entenda minha situação financeira sem precisar navegar em telas ou usar comandos técnicos.

### Escopo

Esta US cobre apenas **consultas de leitura** (read-only) iniciadas pelo usuário via WhatsApp. Não inclui registro, edição, exclusão, alertas proativos nem onboarding.

### Cenários de uso

| ID | Intenção do usuário | Exemplos de frases |
|----|---------------------|--------------------|
| C1 | Panorama geral do mês atual | "Como estou indo?", "Como está meu mês?", "Resumo do mês" |
| C2 | Orçamento de um mês específico | "Como foi meu orçamento de janeiro/2026?", "Orçamento de março/2025" |
| C3 | Orçamento do mês atual | "Como está meu orçamento do mês atual?", "Meu orçamento" |
| C4 | Fatura de cartão específico | "Quanto está minha fatura do cartão Nubank?", "Fatura do Inter" |
| C5 | Última transação | "Qual foi a minha última transação?", "O que eu gastei por último?" |
| C6 | Últimas N transações | "Quais foram as minhas últimas 5 transações?", "Últimos lançamentos" |
| C7 | Orçamento completo por categoria | "Me mostra o orçamento completo", "Quanto posso gastar em cada categoria?", "Orçamento detalhado" |

---

### Critérios de aceite

#### CA-01 — Seleção determinística de ferramenta

- Para cada mensagem do usuário, o agente deve escolher **exatamente** a ferramenta correspondente ao cenário:
  - C1, C3: `query_month` + `query_plan` quando o usuário pedir panorama ou orçamento.
  - C2, C7: `query_plan` com `competence` explícito (C2) ou mês atual (C7).
  - C4: `resolve_card` para obter o `cardId`, depois `query_card_invoice`.
  - C5, C6: `query_month` com `limit` adequado para recuperar as transações mais recentes.
- É proibido usar uma ferramenta como substituta de outra (ex.: responder de memória, inventar valores ou usar `search_transactions` para listar "últimas transações" sem termo de busca).

#### CA-02 — Anti-simulação e zero alucinação

- O agente **nunca** deve inventar, estimar ou simular valores financeiros.
- Toda resposta com valor, categoria, data ou status deve vir do retorno de uma ferramenta.
- Se nenhuma ferramenta puder responder (dados ausentes, erro técnico ou fora de domínio), o agente deve informar claramente a impossibilidade, sem fabricar uma resposta.

#### CA-03 — Idioma e tom

- Todas as respostas devem ser em **português do Brasil**.
- Tom amigável, simples e sem julgamento, conforme persona do MeControla.
- Uso obrigatório de emojis contextuais:
  - 📊 para resumos/orçamento.
  - 💰 para valores/fatura.
  - ✅ para confirmação de ações (quando aplicável neste fluxo de leitura).

#### CA-04 — Formatação WhatsApp

- Negrito apenas com `*asterisco simples*`.
- **Proibido** usar `**duplo asterisco**`.
- Respostas prontas para WhatsApp, sem markdown incompatível.

#### CA-05 — Resolução de ambiguidade

- Se o usuário mencionar um cartão por apelido e o `resolve_card` retornar `found=false`, o agente deve chamar `list_cards`, apresentar os cartões cadastrados e pedir para o usuário escolher.
- Se o usuário perguntar "últimas transações" sem quantidade, o agente deve usar `limit=5` como padrão.
- Se o usuário pedir "como estou indo?" sem mencionar mês, o agente deve usar o **mês atual no fuso `America/Sao_Paulo`**.

#### CA-06 — Erros e ausência de dados

- Orçamento não encontrado para a competência: "Você ainda não tem um orçamento para *{competência}*. Posso te ajudar a criar um?"
- Fatura não encontrada: "Não encontrei fatura para o cartão *{apelido}* em *{mês}*."
- Sem transações no mês: "Não há lançamentos em *{mês}*."
- Erro técnico: "Não consegui consultar agora. Tente novamente em breve." — sem expor detalhes técnicos.

#### CA-07 — Contexto de mês

- Quando o usuário mencionar mês/ano (ex.: "janeiro/2026"), o agente deve converter para o formato `YYYY-MM` (`2026-01`) antes de chamar a ferramenta.
- Quando o usuário mencionar apenas "mês atual", o agente deve usar `time.Now()` no fuso `America/Sao_Paulo` e formatar como `YYYY-MM`.

#### CA-08 — Uso de memória e continuidade

- O agente deve aproveitar o histórico da thread (`internal/platform/memory`) para responder perguntas de follow-up como "E a fatura?" ou "E as últimas transações?" quando o contexto estiver claro.
- A resolução de contexto não pode substituir a chamada à ferramenta: o agente ainda deve invocar a tool correta para obter os dados atualizados.

#### CA-09 — Orçamento completo por categoria

- Quando o usuário solicitar o orçamento completo ou detalhado, o agente deve chamar `query_plan` e apresentar **todas as categorias** retornadas no campo `allocations`.
- Para cada categoria devem ser exibidos:
  - nome da categoria (ex.: *Custo Fixo*, *Conhecimento*, *Prazeres*, *Metas*, *Liberdade Financeira*);
  - **quanto pode gastar**: valor planejado em reais (`plannedCents` → R$);
  - **quanto gastou**: valor realizado em reais (`spentCents` → R$);
  - **percentual de execução**: `percentageSpent` arredondado em `%` (ex.: 73%).
- Categorias com `plannedCents` nulo (orçamento automático/rascunho sem valor definido) devem ser exibidas com "*Sem limite definido*" ou "*R$ 0,00*", conforme regra de negócio do módulo `budgets`, **nunca omitidas**.
- O total geral deve ser exibido no topo: total planejado, total gasto e percentual geral, quando disponíveis.
- Valores em centavos devem ser convertidos para reais com duas casas decimais (ex.: `123450` → `R$ 1.234,50`).

---

### Regras de negócio

1. **Domínio permitido**: apenas consultas sobre lançamentos, cartões de crédito, orçamento e fatura.
2. **Fora do domínio**: investimentos, empréstimos, seguros, impostos complexos ou temas não financeiros devem ser recusados educadamente.
3. **Privacidade**: o agente só responde sobre o `resourceID` da thread ativa; nunca cruza dados entre usuários.
4. **Idempotência**: consultas são read-only e podem ser repetidas sem efeitos colaterais.
5. **Ordenação das últimas transações**: por `createdAt` descendente, limitadas ao mês de referência quando não houver indicação contrária.

---

### Fundamentos técnicos e arquitetura

Esta US é construída sobre os primitivos reais do repositório e segue obrigatoriamente as skills e referências abaixo:

- `@.agents/skills/go-implementation/` — regras R0-R7, DI manual, contratos Go e validação proporcional.
- `@.agents/skills/mastra/` — arquitetura do consumidor agentivo `internal/agents`, uso de `Agent`, `Tool`, `Memory` e `Runtime`.
- **Domain Modeling Made Functional: Tackle Software Complexity with Domain-Driven Design and F#** — smart constructors, estados ilegais irrepresentáveis, state-as-type e workflow como pipeline.

#### Bounded contexts envolvidos

- `internal/agents` — consumidor agentivo `mecontrola`; orquestra as tools e as instruções do agente.
- `internal/budgets` — fornece orçamento, alertas e resumo mensal via `BudgetPlanner`.
- `internal/transactions` — fornece lançamentos, fatura de cartão e resumo mensal via `TransactionsLedger`.

#### Tools existentes a serem utilizadas

- `query_month` (`internal/agents/application/tools/query_month.go`) — resumo financeiro e lançamentos do mês.
- `query_plan` (`internal/agents/application/tools/query_plan.go`) — plano orçamentário mensal com alertas.
- `query_card_invoice` (`internal/agents/application/tools/query_card_invoice.go`) — fatura de cartão de crédito.
- `resolve_card` (`internal/agents/application/tools/resolve_card.go`) — resolver cartão por apelido.
- `list_cards` (`internal/agents/application/tools/list_cards.go`) — listar cartões cadastrados.
- `search_transactions` (`internal/agents/application/tools/search_transactions.go`) — buscar lançamentos por termo.
- `get_transaction` (`internal/agents/application/tools/get_transaction.go`) — detalhar lançamento por ID.

#### Adaptadores (binding)

- `internal/agents/infrastructure/binding/budget_planner_adapter.go` — adapta os use cases de `internal/budgets` para a interface `agents.BudgetPlanner`.
- `internal/agents/infrastructure/binding/transactions_ledger_adapter.go` — adapta os use cases de `internal/transactions` para a interface `agents.TransactionsLedger`.

#### Instruções do agente

- As instruções em `internal/agents/application/agents/mecontrola_agent.go` devem ser atualizadas para incluir a intenção de consulta conversacional e reforçar:
  - seleção determinística das tools acima;
  - anti-simulação;
  - resolução de cartão por apelido;
  - uso do fuso `America/Sao_Paulo` para "mês atual".

---

### Aplicação de DMMF

1. **Smart constructors**: comandos como `GetMonthlySummaryCommand` em `internal/budgets/domain/commands` já validam `userID` e `competence`. Qualquer novo command introduzido nesta US deve seguir o mesmo padrão `(T, error)`.
2. **State-as-type**: status de orçamento (`active`/`draft`), status de alerta e tipos de lançamento devem permanecer tipos fechados; nunca propagar `string` livre em assinatura pública.
3. **Workflow pipeline**: se for necessário um fluxo de múltiplos passos (ex.: resolver cartão + consultar fatura), deve ser modelado como sequência de passos dentro do use case/tool, sem lógica de domínio no handler ou no prompt.
4. **Pure core / IO shell**: regras de formatação de resposta (ex.: conversão de centavos para reais, ordenação) podem viver em funções puras; IO fica nas tools/adapters.

---

### Exemplos de diálogo

#### C1 — Como estou indo?

**Usuário:** Como estou indo?

**Agente:** 📊 *Resumo de julho/2026*

- Receitas: *R$ 8.000,00*
- Despesas: *R$ 5.420,00*
- Saldo: *R$ 2.580,00*

Seu orçamento está *ativo*. Quer ver o detalhe por categoria?

#### C2 — Orçamento de janeiro/2026

**Usuário:** Como foi meu orçamento de janeiro/2026?

**Agente:** 📊 *Orçamento de janeiro/2026*

- Total planejado: *R$ 7.000,00*
- Total gasto: *R$ 6.850,00*
- Custo Fixo: *R$ 2.800,00* de *R$ 3.000,00* (93%)
- Prazeres: *R$ 1.200,00* de *R$ 1.400,00* (86%)

Nenhum alerta ativo para esse mês. ✅

#### C3 — Orçamento do mês atual

**Usuário:** Como está meu orçamento do mês atual?

**Agente:** 📊 *Orçamento de julho/2026*

- Total planejado: *R$ 8.000,00*
- Gasto até agora: *R$ 4.100,00* (51%)
- Metas: *R$ 800,00* de *R$ 1.600,00* (50%)

Você está dentro do planejamento. 🎯

#### C4 — Fatura do cartão Nubank

**Usuário:** Quanto está minha fatura do cartão Nubank?

**Agente:** 💰 *Fatura Nubank — julho/2026*

- Fechamento: *10/07/2026*
- Vencimento: *17/07/2026*
- Total: *R$ 1.450,00*

Quer ver os lançamentos dessa fatura?

#### C5 — Última transação

**Usuário:** Qual foi a minha última transação?

**Agente:** 💰 *Último lançamento*

- Descrição: *Supermercado Extra*
- Valor: *R$ 320,50*
- Data: *06/07/2026*
- Categoria: *Custo Fixo > Supermercado*

#### C6 — Últimas 5 transações

**Usuário:** Quais foram as minhas últimas 5 transações?

**Agente:** 💰 *Últimos lançamentos*

1. *Supermercado Extra* — R$ 320,50 (06/07)
2. *Uber* — R$ 24,90 (05/07)
3. *Spotify* — R$ 19,90 (05/07)
4. *Farmácia Pague Menos* — R$ 87,00 (04/07)
5. *Gasolina Shell* — R$ 210,00 (03/07)

#### C7 — Orçamento completo por categoria

**Usuário:** Me mostra o orçamento completo

**Agente:** 📊 *Orçamento completo de julho/2026*

- Total planejado: *R$ 8.000,00*
- Total gasto: *R$ 4.100,00* (51%)

*Custo Fixo* — gasto *R$ 2.100,00* de *R$ 3.000,00* (70%)
*Conhecimento* — gasto *R$ 400,00* de *R$ 800,00* (50%)
*Prazeres* — gasto *R$ 1.000,00* de *R$ 1.600,00* (63%)
*Metas* — gasto *R$ 400,00* de *R$ 1.600,00* (25%)
*Liberdade Financeira* — gasto *R$ 200,00* de *R$ 1.000,00* (20%)

Você ainda tem *R$ 3.900,00* disponíveis no mês. 🎯

---

### Rastreabilidade

- `internal/agents/application/agents/mecontrola_agent.go` — instruções do agente.
- `internal/agents/application/tools/query_month.go`
- `internal/agents/application/tools/query_plan.go`
- `internal/agents/application/tools/query_card_invoice.go`
- `internal/agents/application/tools/resolve_card.go`
- `internal/agents/application/tools/list_cards.go`
- `internal/agents/application/tools/search_transactions.go`
- `internal/agents/application/tools/get_transaction.go`
- `internal/agents/infrastructure/binding/budget_planner_adapter.go`
- `internal/agents/infrastructure/binding/transactions_ledger_adapter.go`
- `internal/budgets/application/usecases/get_monthly_summary.go`
- `internal/transactions/application/usecases/get_card_invoice.go`

### Critérios de validação

- `go build ./internal/agents/...`
- `go vet ./internal/agents/...`
- `go test -race -count=1 ./internal/agents/application/tools/...`
- `go test -race -count=1 ./internal/agents/application/agents/...`
- `golangci-lint run ./internal/agents/...`

---

## Notas de implementação

1. **Não criar novas tools** se as existentes já atenderem ao cenário. Reutilizar `query_month`, `query_plan`, `query_card_invoice`, `resolve_card`, `list_cards`.
2. **Se uma nova tool for necessária**, seguir o padrão `Build*Tool` com `tool.NewTool[I,O]`, schema JSON estrito e exec fino delegando para interface da aplicação.
3. Manter `internal/platform/workflow` como kernel genérico: não importar domínio financeiro no kernel.
4. Preservar a separação `infrastructure -> application -> domain` nos módulos `budgets` e `transactions`.
5. Testes de regressão devem cobrir cenários C1 a C7, incluindo casos de erro, ambiguidade de cartão, ausência de dados e orçamento completo com todas as categorias exibidas em R$ e %.
6. A formatação de centavos para reais deve ser feita por função pura reutilizável (ex.: helper no pacote de presenters ou mappers), garantindo consistência entre C2, C3 e C7.
