# Registro Conversacional de Transações do Dia a Dia — User Story

> Fonte: solicitação de evolução do agente `mecontrola` para registrar receitas e despesas cotidianas via linguagem natural no WhatsApp, com zero alucinação, zero duplicidade e confirmação humana obrigatória antes de toda escrita.
> Objetivo da funcionalidade: permitir que o usuário registre transações financeiras do dia a dia conversando com o agente, garantindo que cada lançamento capture obrigatoriamente **dia**, **categoria**, **subcategoria**, **descrição** e **data que ocorreu a transação**, orquestrado pelo consumidor agentivo em `internal/agents` e persistido nos módulos `internal/transactions`, `internal/categories`, `internal/card` e `internal/budgets`.
> Data de geração: 2026-07-07
> Nome do arquivo: `2026-07-07-us-registro-conversacional-transacoes-dia-a-dia.md`

---

## Confronto com o Codebase

Esta seção mapeia a user story contra o código existente, identificando o que já está implementado, o que precisa ser reforçado e os gaps a eliminar para atingir **0 lacunas, 0 ressalvas, 0 falsos positivos**.

### Princípio orientador

> **Registrar transações é uma operação de escrita financeira. Toda escrita via agente deve passar por confirmação humana, idempotência por `wamid` e validação de categoria antes da persistência.**
>
> O codebase já possui a base do fluxo (`register_expense`, `register_income`, `pending-entry workflow`, `RegisterAttempt`, `IdempotentWrite`). Esta documentação orienta o refinamento para cobrir **datas relativas/implícitas**, **extrair categoria/subcategoria com precisão**, **evitar falsos positivos de valor** e **garantir que nenhum campo obrigatório seja inferido sem evidência**.

### Arquitetura relevante identificada

- **Módulo `internal/agents`**:
  - Agente LLM: `BuildMeControlaAgent` em `application/agents/mecontrola_agent.go` — runtime genérico de tool-calling com instruções específicas de formatação para WhatsApp.
  - Inbound: `HandleInbound` em `application/usecases/handle_inbound.go`; prioridade `PendingEntryContinuer` → `DestructiveConfirmContinuer` → `ResolveOnboardingOrAgent`.
  - Tools de escrita: `register_expense` (`application/tools/register_expense.go`), `register_income` (`application/tools/register_income.go`), `create_recurrence` (`application/tools/create_recurrence.go`), `edit_entry`/`delete_entry`.
  - Tools de consulta: `classify_category` (`application/tools/classify_category.go`), `list_categories` (`application/tools/list_categories.go`), `resolve_card` (`application/tools/resolve_card.go`), `list_cards` (`application/tools/list_cards.go`).
  - Use cases: `RegisterAttempt` (`application/usecases/register_attempt.go`), `IdempotentWrite` (`application/usecases/idempotent_write.go`), `PendingEntryContinuer` (`application/usecases/pending_entry_continuer.go`).
  - Workflows: `pending-entry` (`application/workflows/pending_entry_workflow.go`) para suspender quando faltam dados; `destructive-confirm` (`application/workflows/destructive_confirm_workflow.go`) para confirmações destrutivas.
  - Idempotência: `WriteLedger` (`infrastructure/persistence/write_ledger_repository.go`) com chave `(wamid, itemSeq, operation)`.
  - Binding/adapters: `internal/agents/infrastructure/binding/*` — `transactions_ledger_adapter.go`, `card_manager_adapter.go`, `categories_reader_adapter.go`, `budget_planner_adapter.go`.

- **Módulo `internal/transactions`**:
  - Domínio: `entities.Transaction` (direção, forma de pagamento, valor, descrição, categoria/subcategoria, mês de referência, cartão, parcelas, origem WhatsApp); `commands.CreateTransaction` / `UpdateTransaction`; domain services `TransactionWorkflow`, `InstallmentSplitter`, `BillingCycleResolver`, `RefMonthResolver`.
  - Value objects: `valueobjects.Direction`, `valueobjects.PaymentMethod`, `valueobjects.Money`, `valueobjects.Description`, `valueobjects.RefMonth`, `valueobjects.InstallmentCount`, `valueobjects.UserID/CardID/CategoryID/SubcategoryID`, `valueobjects.CategoryDecisionSource`, `valueobjects.CategoryWriteEvidence`.
  - Use cases: `CreateTransaction`, `UpdateTransaction`, `DeleteTransaction`, `GetTransaction`, `ListTransactions`, `SearchTransactions`, `GetCardInvoice`, `ListMonthlyEntries`, `GetMonthlySummary`, `CreateRecurringTemplate`.
  - Eventos: `TransactionCreated`, `TransactionUpdated`, `TransactionDeleted` publicados via outbox.

- **Módulo `internal/categories`**:
  - Domínio: `entities.Category`, `entities.DictionaryEntry`; VOs `Kind`, `AllocationType`, `SignalType`, `Confidence`, `MatchQuality`, `MatchScore`, `SearchOutcome`, `SearchQuery`, `Slug`; domain service `CandidateResolver`.
  - Use cases: `SearchDictionary`, `ResolveCategoryForWrite`, `ListCategories`, `ResolveBySlug`.
  - Saída: candidatos com score, path (`Raiz > Sub`), outcome (`no_match`/`matched`/`ambiguous`).

- **Módulo `internal/card`**:
  - Domínio: `entities.Card`; VOs `Nickname`, `BankCode`, `BillingCycle`; services `CreateCardDecider`, `UpdateCardDecider`, `BillingCycleService`, `PurchaseDayService`.
  - Use cases: `CreateCard`, `ListCards`, `GetCard`, `ResolveCardByNickname`, `CountCards`, `BestPurchaseDay`, `InvoiceFor`.
  - Adaptado ao agente via interface `CardManager`.

- **Módulo `internal/budgets`**:
  - Consome eventos `transactions.transaction.created.v1` e projeta despesas; não cria transações diretamente. Orçamento afeta a experiência de confirmação e alertas futuros, mas não o registro em si.

- **Plataforma `internal/platform/{agent,llm,memory,workflow,tool,scorer}`**:
  - Substrato reutilizável consumido por `internal/agents`; não pode ser reimplementado no domínio (regra de ouro da skill `mastra`).

### Status por requisito da US

| Requisito | Status no codebase | Onde vive hoje | O que falta / gap a eliminar |
|---|---|---|---|
| Registro de despesa simples (pix, dinheiro, débito) | **Parcialmente implementado** | `register_expense`, `RegisterAttempt`, `CreateTransaction` | Refinar extração de **data que ocorreu** quando implícita/relativa ("ontem", "hoje", "anteontem", "segunda"). Hoje o agente pode inferir errado ou omitir. |
| Registro de receita (salário, freelancer) | **Parcialmente implementado** | `register_income`, `RegisterAttempt`, `CreateTransaction` | Mesmo gap de data relativa; validar descrição mínima e categoria de renda (`Kind = income`). |
| Compra parcelada no cartão de crédito | **Parcialmente implementado** | `register_expense` com `paymentMethod=credit_card`, `CardLookup`, `InstallmentSplitter`, `BillingCycleResolver` | Resolver cartão por apelido com precisão; validar `installments` 1..24; calcular mês de referência da primeira parcela com base no ciclo do cartão. |
| Extração de dia/data da transação | **Gap** | — | Não existe parser de data relativa consolidado no agente. Deve ser adicionado como função pura (DMMF) antes de chamar `RegisterAttempt`. |
| Captura obrigatória de categoria/subcategoria | **Parcialmente implementado** | `classify_category`, `SearchDictionary`, `ResolveCategoryForWrite`, `PendingEntryState` | Garantir que nenhuma transação persista sem `CategoryID` + `SubcategoryID` validados. Suspender em `pending-entry` quando score < 0,80 ou ambíguo. |
| Captura obrigatória de descrição | **Parcialmente implementado** | `valueobjects.Description` | O agente deve extrair descrição da frase do usuário; se ausente, perguntar antes de persistir. |
| Confirmação humana antes da escrita | **Implementado** | `pending-entry workflow`, `RegisterAttempt` | Manter: nenhuma escrita financeira sem confirmação explícita do usuário. |
| Idempotência de escrita | **Parcialmente implementado / gap crítico** | `IdempotentWrite` (`application/usecases/idempotent_write.go`) e `WriteLedgerRepository` (`infrastructure/persistence/write_ledger_repository.go`) existem, mas **não estão integrados** ao `pending-entry workflow` nem ao `executeWrite`. O repositório só é usado pelo job de retenção `PurgeLedger`. | Integrar `IdempotentWrite.Execute` ao fluxo produtivo de registro (antes de `ledger.CreateTransaction`/`CreateRecurringTemplate` no `executeWrite`), garantindo chave `(wamid, itemSeq, operation)` e retorno de `ToolOutcomeReplay` sem persistir novamente. |
| Formatação WhatsApp e anti-simulação | **Implementado** | Instruções do `mecontrola_agent.go` | Reforçar que o agente nunca deve preencher campos obrigatórios com valores inventados. |
| Tratamento de ambiguidade de cartão | **Parcialmente implementado** | `resolve_card`, `list_cards` | Se `resolve_card` retornar `found=false`, apresentar lista e pedir escolha antes de prosseguir. |
| Tratamento de ambiguidade de categoria | **Parcialmente implementado** | `classify_category`, `pending-entry` | Se outcome for `ambiguous` ou `no_match`, suspender e perguntar; nunca chutar. |
| Validação de valor monetário | **Implementado** | `valueobjects.Money` (sempre > 0) | O agente deve rejeitar valores zero, negativos ou irreais (> R$ 99.999.999,99) e pedir correção. |

---

## US-01 — Registro Conversacional de Transações do Dia a Dia

**Como** usuário do MeControla, **quero** conversar com o agente para registrar minhas receitas e despesas do dia a dia usando frases naturais, **para que** eu não precise abrir telas ou memorizar comandos e meu controle financeiro acompanhe minha vida real sem esforço.

### Escopo

Esta US cobre o **registro de transações de entrada e saída** iniciado pelo usuário via WhatsApp, incluindo:

- Despesas em dinheiro, Pix, débito, boleto, vale-refeição/alimentação.
- Receitas como salário, freelas, presentes, reembolsos.
- Compras no cartão de crédito, à vista ou parceladas.
- Datas explícitas, relativas ("ontem", "hoje", "anteontem", "terça passada") ou implícitas (assume "hoje" quando não informado).
- Categorização automática quando o score for ≥ 0,80 e não ambíguo; suspensão conversacional quando não.

Não inclui: edição/exclusão de lançamentos (coberto pelas tools `edit_entry`/`delete_entry`), criação de cartões, criação de orçamentos, alertas proativos nem consultas financeiras.

### Cenários de uso

| ID | Intenção do usuário | Exemplos de frases |
|---|---|---|
| R1 | Despesa simples via Pix | "Gastei R$ 150,00 no supermercado no pix" |
| R2 | Despesa em dinheiro com data relativa | "Ontem fui na feira e gastei cinquenta reais em pastel no dinheiro" |
| R3 | Compra parcelada no cartão de crédito | "Comprei uma geladeira na Casas Bahia de R$ 2.000,00 em 10x no cartão Nubank" |
| R4 | Compra à vista no cartão de crédito | "Paguei R$ 120,00 de gasolina no cartão Inter" |
| R5 | Receita fixa | "Recebi meu décimo terceiro de R$ 10.000,00" |
| R6 | Receita variável | "Recebi duzentos reais de um freelancer" |
| R7 | Despesa com categoria incerta | "Gastei 35 reais com algo pro trabalho" |
| R8 | Múltiplas transações em uma mensagem | "Hoje gastei 30 reais no ônibus e 15 no café" |

---

### Critérios de aceite

#### CA-01 — Captura obrigatória dos cinco campos

Toda transação registrada deve conter, sem exceção:

1. **Dia / data que ocorreu a transação** — a data real do movimento financeiro, não a data do envio da mensagem.
2. **Categoria** — categoria raiz válida do catálogo editorial (`internal/categories`).
3. **Subcategoria** — subcategoria diretamente ligada à categoria raiz escolhida.
4. **Descrição** — texto descritivo do que foi gasto/recebido.
5. **Valor** — valor monetário positivo, em centavos.

- Se qualquer um dos campos 1 a 4 não puder ser extraído ou validado, o agente deve **suspender o registro no workflow `pending-entry` e perguntar ao usuário**, nunca preencher com valor inventado.
- O valor deve ser validado por `valueobjects.Money` e rejeitado se zero, negativo ou acima do limite máximo permitido pelo sistema.

#### CA-02 — Extração precisa de data

- O agente deve interpretar datas relativas com base no **fuso `America/Sao_Paulo`** e na data atual real:
  - "hoje" → data atual.
  - "ontem" → data atual − 1 dia.
  - "anteontem" → data atual − 2 dias.
  - "segunda", "terça" etc. → ocorrência mais recente daquele dia da semana (se hoje for segunda, "segunda passada" = −7 dias; "segunda" sozinho = hoje).
  - "semana passada", "mês passado" → rejeitar e pedir data específica (muita ambiguidade).
- Quando o usuário não informar data, o agente deve assumir **hoje** e confirmar com o usuário antes de persistir.
- A data extraída deve ser convertida para o formato `YYYY-MM-DD` e usada para calcular o `RefMonth` da transação.
- Para compras no cartão de crédito, a **data da compra** é o campo obrigatório; o mês de referência da primeira parcela é calculado pelo ciclo do cartão (`BillingCycleResolver`).

#### CA-03 — Categorização determinística e segura

- O agente deve usar a tool `classify_category` para resolver categoria/subcategoria a partir da descrição do usuário.
- Regras de decisão:
  - **Score ≥ 0,80 e outcome = matched e não ambíguo** → prosseguir com o par `(RootCategoryID, SubcategoryID)` validado por `ResolveCategoryForWrite`.
  - **Score entre 0,55 e 0,79 ou outcome = ambiguous** → suspender no `pending-entry` e apresentar até 3 candidatos para o usuário escolher.
  - **Score < 0,55 ou outcome = no_match** → suspender e perguntar "Em qual categoria isso se encaixa?".
- É proibido ao agente escolher uma categoria "por eliminação" ou inventar uma categoria que não exista no catálogo.
- O `Kind` da categoria deve coincidir com a direção da transação (`income` para receitas, `expense` para despesas).

#### CA-04 — Resolução de cartão sem falsos positivos

- Quando a forma de pagamento for cartão de crédito, o agente deve:
  1. Extrair o apelido/banco mencionado pelo usuário.
  2. Chamar `resolve_card` para obter o `cardId`.
  3. Se `found=false`, chamar `list_cards` e apresentar os cartões cadastrados para o usuário escolher.
  4. Se `found=true` mas o usuário não tiver cartão cadastrado, oferecer criar o cartão (fora do escopo desta US; redirecionar).
- O número de parcelas deve ser extraído explicitamente; se não informado, assumir **1 (à vista)** e confirmar.
- Se o usuário disser "no cartão" sem especificar qual, o agente deve perguntar qual cartão antes de prosseguir.

#### CA-05 — Confirmação humana obrigatória

- Antes de toda escrita no banco, o agente deve apresentar um resumo da transação e pedir confirmação explícita.
- Para **despesas**, o resumo deve conter: descrição, valor, categoria > subcategoria, data e forma de pagamento.
  - Exemplo: "Confirma? *Supermercado* — *R$ 150,00* — *Custo Fixo > Supermercado* — *hoje* — *Pix*?"
- Para **compras no cartão de crédito**, incluir também cartão e número de parcelas.
  - Exemplo: "Confirma? *Geladeira — Casas Bahia* — *R$ 2.000,00* — *Metas > Compra Planejada* — *hoje* — *10x no Nubank*?"
- Para **receitas**, o resumo não deve perguntar forma de pagamento (o sistema fixa `paymentMethod = pix`); deve conter descrição, valor, categoria > subcategoria e data.
  - Exemplo: "Confirma? *Freelancer* — *R$ 200,00* — *Renda Variável > Freelance* — *hoje*?"
- A confirmação pode ser:
  - "sim", "ok", "pode registrar", "confirmo".
  - "não", "cancela", "errado" → cancelar e pedir correção.
- Enquanto o usuário não confirmar, a transação permanece no estado `pending` do workflow `pending-entry`.

#### CA-06 — Idioma e tom

- Todas as respostas em **português do Brasil**.
- Tom amigável, simples e sem julgamento.
- Emojis contextuais:
  - 💰 para receitas.
  - 💸 para despesas.
  - 💳 para cartões.
  - ✅ para confirmação.
  - ❓ para perguntas de clarificação.

#### CA-07 — Formatação WhatsApp

- Negrito apenas com `*asterisco simples*`.
- Proibido `**duplo asterisco**`.
- Valores em reais com duas casas decimais (ex.: `R$ 1.234,50`).
- Datas no formato `DD/MM/YYYY` na resposta ao usuário.

#### CA-08 — Idempotência e zero duplicidade

- Toda escrita via agente deve passar por `IdempotentWrite` com chave `(wamid, itemSeq, operation)`.
- Se o mesmo `wamid` + `itemSeq` + `operation` já tiver sido processado, o agente deve retornar `ToolOutcomeReplay` e a resposta já computada, sem persistir novamente.
- A confirmação de uma transação pendente pode reutilizar a mesma chave da mensagem original (pois ainda é o mesmo item) ou uma chave derivada do `incomingMessageId` da resposta; o importante é que uma mesma mensagem do WhatsApp não gere duas escritas.
- **Gap atual**: o use case `IdempotentWrite` e o repositório `WriteLedger` existem, mas ainda não estão acionados no fluxo produtivo de registro (`executeWrite` do `pending-entry workflow`). A implementação desta US deve eliminar essa lacuna.

#### CA-09 — Tratamento de múltiplas transações em uma mensagem

- Se o usuário mencionar mais de uma transação na mesma mensagem (R8), o agente deve:
  1. Confirmar que entendeu múltiplos lançamentos.
  2. Processar cada transação como item separado, com seu próprio `itemSeq`.
  3. Suspender o workflow se algum item estiver incompleto.
  4. Pedir confirmação agrupada ou individual, conforme implementação escolhida, mas **sempre** validar cada um dos cinco campos por item.

#### CA-10 — Anti-simulação e zero alucinação

- O agente **nunca** deve inventar valor, data, categoria, subcategoria, cartão ou forma de pagamento.
- Se o usuário fornecer informação incompleta ou ambígua, o agente deve pedir esclarecimento.
- O agente não deve responder com base em memória de transações passadas para inferir uma nova transação.

---

### Regras de negócio

1. **Direção determinada pelo contexto**:
   - Verbos como "gastei", "paguei", "comprei", "gasto" → despesa (`outcome`).
   - Verbos como "recebi", "ganhei", "caiu", "entrada" → receita (`income`).
2. **Formas de pagamento mapeadas**:
   - Despesas:
     - "pix" → `pix`.
     - "dinheiro" → `cash`.
     - "débito", "no débito" → `debit_card`.
     - "cartão", "no cartão", "crédito" → `credit_card` (exige cartão + parcelas).
     - "boleto" → `boleto`.
     - "vale refeição", "VR" → `vale_refeicao`.
     - "vale alimentação", "VA" → `vale_alimentacao`.
   - Receitas:
     - O sistema armazena `paymentMethod = pix` para toda receita (`registerIncomePaymentMethod = "pix"` em `internal/agents/application/usecases/register_entry.go`). Não perguntar forma de pagamento para receitas.
3. **Parcelas**:
   - Aplica-se apenas a despesas no cartão de crédito.
   - Mínimo 1, máximo 24 (`valueobjects.InstallmentCount`).
   - Valor total dividido pelas parcelas com resto distribuído nas primeiras (`InstallmentSplitter`).
4. **Data e mês de referência**:
   - A data da transação define o `RefMonth` para pagamentos não cartão.
   - Para cartão de crédito, o `RefMonth` da primeira parcela é calculado pelo ciclo de fechamento/vencimento do cartão.
5. **Categorias de receita**:
   - Devem pertencer ao `Kind = income` do catálogo editorial (`internal/categories`).
   - Raízes de receita reais: `Salário`, `Renda Variável`, `Investimentos`, `Aluguel Recebido`, `Restituições e Cashback`, `Presentes Recebidos`, `Vendas`, `Outras Receitas`.
   - Exemplos de paths: `Salário > Décimo Terceiro`, `Renda Variável > Freelance`, `Restituições e Cashback > Cashback`, `Presentes Recebidos > Presentes em Dinheiro`.
6. **Categorias de despesa**:
   - Devem pertencer ao `Kind = expense` do catálogo editorial.
7. **Confirmação destrutiva/impactante**:
   - Toda escrita financeira é considerada impactante e exige confirmação antes da persistência.
8. **Idempotência**:
   - A chave de idempotência deve ser `(wamid, itemSeq, operation)`.

---

### Fundamentos técnicos e arquitetura

Esta US é construída sobre os primitivos reais do repositório e segue obrigatoriamente as skills e referências abaixo:

- `@.agents/skills/go-implementation/` — regras R0-R7, DI manual, contratos Go, smart constructors, state-as-type e validação proporcional.
- `@.agents/skills/mastra/` — arquitetura do consumidor agentivo `internal/agents`, uso de `Agent`, `Tool`, `Memory`, `Runtime` e `Workflow`.
- **Domain Modeling Made Functional: Tackle Software Complexity with Domain-Driven Design and F#** — smart constructors, estados ilegais irrepresentáveis, state-as-type, discriminated union via errors, workflow pipeline e pure core / IO shell.

#### Bounded contexts envolvidos

- `internal/agents` — consumidor agentivo `mecontrola`; orquestra as tools, instruções do agente e workflows `pending-entry` / `destructive-confirm`.
- `internal/transactions` — registra, atualiza e consulta lançamentos; consome cartão para ciclos de fatura.
- `internal/categories` — classifica e valida categoria/subcategoria.
- `internal/card` — resolve cartões por apelido e fornece ciclo de fatura.
- `internal/budgets` — recebe eventos de transação para projeção de despesas e alertas futuros.

#### Tools existentes a serem utilizadas/reutilizadas

- `register_expense` (`internal/agents/application/tools/register_expense.go`) — inicia registro de despesa.
- `register_income` (`internal/agents/application/tools/register_income.go`) — inicia registro de receita.
- `classify_category` (`internal/agents/application/tools/classify_category.go`) — resolve categoria/subcategoria.
- `list_categories` (`internal/agents/application/tools/list_categories.go`) — lista catálogo quando necessário.
- `resolve_card` (`internal/agents/application/tools/resolve_card.go`) — resolve cartão por apelido.
- `list_cards` (`internal/agents/application/tools/list_cards.go`) — lista cartões cadastrados.

#### Use cases e workflows da aplicação agentiva

- `RegisterAttempt` (`internal/agents/application/usecases/register_attempt.go`) — inicia workflow de pendência, classifica categoria e solicita confirmação.
- `PendingEntryContinuer` (`internal/agents/application/usecases/pending_entry_continuer.go`) — retoma workflow com resposta do usuário.
- `IdempotentWrite` (`internal/agents/application/usecases/idempotent_write.go`) — garante idempotência de escritas. **Gap atual**: existem, mas ainda não são acionados no `executeWrite` do `pending-entry workflow`; devem ser integrados nesta US.
- `pending-entry workflow` (`internal/agents/application/workflows/pending_entry_workflow.go`) — gerencia estados de espera por dados.

#### Adaptadores (binding)

- `internal/agents/infrastructure/binding/transactions_ledger_adapter.go` — adapta `internal/transactions` para `agents.TransactionsLedger`.
- `internal/agents/infrastructure/binding/categories_reader_adapter.go` — adapta `internal/categories` para `agents.CategoriesReader`.
- `internal/agents/infrastructure/binding/card_manager_adapter.go` — adapta `internal/card` para `agents.CardManager`.

#### Instruções do agente

- As instruções em `internal/agents/application/agents/mecontrola_agent.go` devem ser atualizadas para:
  - Reforçar a extração obrigatória dos cinco campos.
  - Listar as formas de pagamento suportadas e o mapeamento para `PaymentMethod`.
  - Proibir inferência de data, categoria, subcategoria ou cartão sem evidência.
  - Determinar quando suspender para `pending-entry`.
  - Definir o parser de data relativa ("ontem", "hoje", "anteontem", dias da semana).

---

### Aplicação de DMMF

1. **Smart constructors**: comandos como `commands.CreateTransaction` em `internal/transactions/domain/commands` e `CreateBudgetCommand` em `internal/budgets/domain/commands` validam invariantes e retornam `(T, error)`. Novos comandos introduzidos por esta US (ex.: parser de data relativa) devem seguir o mesmo padrão.
2. **State-as-type**: status de transação, forma de pagamento, direção, status do workflow (`PendingStatus`, `AwaitingSlot`, `AwaitingKind`) e outcome de categoria (`SearchOutcome`) devem permanecer tipos fechados; nunca propagar `string` livre em assinatura pública.
3. **Discriminated union via errors**: divergências de fluxo (ex.: categoria ambígua, cartão não encontrado, data inválida) devem ser modeladas como tipos de erro customizados, e o handler/tool deve usar `errors.As` para decidir o próximo passo.
4. **Workflow pipeline**: o registro de uma transação deve seguir o pipeline `parse → validate → decide → persist → publish`:
   - **parse**: extrair valor, data, descrição, forma de pagamento, cartão, parcelas da mensagem.
   - **validate**: validar VOs (`Money`, `Description`, `InstallmentCount`, data, categoria via `ResolveCategoryForWrite`).
   - **decide**: `TransactionWorkflow` decide parcelas, mês de referência e eventos.
   - **persist**: `CreateTransaction` persiste e publica evento no outbox.
   - **publish**: `TransactionCreated` é consumido por `internal/budgets`.
5. **Pure core / IO shell**: regras de parsing de data relativa, normalização de descrição e decisão de categoria são funções puras sem IO; IO fica nas tools, adapters e repositórios.

---

### Exemplos de diálogo

#### R1 — Despesa simples via Pix

**Usuário:** Gastei R$ 150,00 no supermercado no pix

**Agente:** 💸 Entendi: *Supermercado* — *R$ 150,00* — *Pix* — *hoje (07/07/2026)*.

Categoria sugerida: *Custo Fixo > Supermercado*. Confirma?

**Usuário:** sim

**Agente:** ✅ Registrado! *Supermercado* — *R$ 150,00* no Pix.

---

#### R2 — Despesa em dinheiro com data relativa

**Usuário:** Ontem fui na feira e gastei cinquenta reais em pastel no dinheiro

**Agente:** 💸 Entendi: *Pastel na feira* — *R$ 50,00* — *dinheiro* — *06/07/2026*.

Categoria sugerida: *Prazeres > Bares e Lanches*. Confirma?

**Usuário:** sim

**Agente:** ✅ Registrado! *Pastel na feira* — *R$ 50,00* em dinheiro no dia 06/07.

---

#### R3 — Compra parcelada no cartão de crédito

**Usuário:** Comprei uma geladeira na Casas Bahia de R$ 2.000,00 em 10x no cartão Nubank

**Agente:** 💳 Entendi: *Geladeira — Casas Bahia* — *R$ 2.000,00* — *10x no Nubank* — *hoje (07/07/2026)*.

Categoria sugerida: *Metas > Compra Planejada*. Confirma?

**Usuário:** sim

**Agente:** ✅ Registrado! *Geladeira — Casas Bahia* parcelado em 10x de *R$ 200,00* no Nubank. A primeira parcela entra na fatura de julho/2026.

---

#### R5 — Receita fixa

**Usuário:** Recebi meu décimo terceiro de R$ 10.000,00

**Agente:** 💰 Entendi: *Décimo terceiro* — *R$ 10.000,00* — *hoje (07/07/2026)*.

Categoria sugerida: *Salário > Décimo Terceiro*. Confirma?

**Usuário:** sim

**Agente:** ✅ Registrado! *Décimo terceiro* — *R$ 10.000,00*.

---

#### R6 — Receita variável

**Usuário:** Recebi duzentos reais de um freelancer

**Agente:** 💰 Entendi: *Freelancer* — *R$ 200,00* — *hoje (07/07/2026)*.

Categoria sugerida: *Renda Variável > Freelance*. Confirma?

**Usuário:** sim

**Agente:** ✅ Registrado! *Freelancer* — *R$ 200,00*.

---

#### R7 — Categoria incerta

**Usuário:** Gastei 35 reais com algo pro trabalho

**Agente:** ❓ Não consegui definir a categoria com segurança. Você quis dizer:

1. *Conhecimento > Cursos e Treinamentos*
2. *Custo Fixo > Assinaturas Essenciais*
3. *Prazeres > Compras Pessoais*

Qual se encaixa melhor?

---

#### R8 — Múltiplas transações

**Usuário:** Hoje gastei 30 reais no ônibus e 15 no café

**Agente:** 💸 Encontrei 2 lançamentos:

1. *Ônibus* — *R$ 30,00* — Pix/dinheiro? (hoje)
2. *Café* — *R$ 15,00* — Pix/dinheiro? (hoje)

Qual a forma de pagamento de cada um?

---

### Rastreabilidade

- `internal/agents/application/agents/mecontrola_agent.go` — instruções do agente.
- `internal/agents/application/tools/register_expense.go`
- `internal/agents/application/tools/register_income.go`
- `internal/agents/application/tools/classify_category.go`
- `internal/agents/application/tools/list_categories.go`
- `internal/agents/application/tools/resolve_card.go`
- `internal/agents/application/tools/list_cards.go`
- `internal/agents/application/usecases/register_attempt.go`
- `internal/agents/application/usecases/pending_entry_continuer.go`
- `internal/agents/application/usecases/idempotent_write.go`
- `internal/agents/application/workflows/pending_entry_workflow.go`
- `internal/agents/infrastructure/binding/transactions_ledger_adapter.go`
- `internal/agents/infrastructure/binding/categories_reader_adapter.go`
- `internal/agents/infrastructure/binding/card_manager_adapter.go`
- `internal/transactions/application/usecases/create_transaction.go`
- `internal/transactions/domain/commands/create_transaction.go`
- `internal/transactions/domain/valueobjects/payment_method.go`
- `internal/transactions/domain/valueobjects/money.go`
- `internal/transactions/domain/services/transaction_workflow.go`
- `internal/transactions/domain/services/installment_splitter.go`
- `internal/transactions/domain/services/billing_cycle_resolver.go`
- `internal/categories/application/usecases/search_dictionary.go`
- `internal/categories/application/usecases/resolve_category_for_write.go`
- `internal/card/application/usecases/resolve_card_by_nickname.go`
- `internal/card/application/usecases/invoice_for.go`

### Critérios de validação

- `go build ./internal/agents/...`
- `go build ./internal/transactions/...`
- `go build ./internal/categories/...`
- `go build ./internal/card/...`
- `go vet ./internal/agents/... ./internal/transactions/... ./internal/categories/... ./internal/card/...`
- `go test -race -count=1 ./internal/agents/application/tools/...`
- `go test -race -count=1 ./internal/agents/application/usecases/...`
- `go test -race -count=1 ./internal/agents/application/workflows/...`
- `go test -race -count=1 ./internal/transactions/domain/services/...`
- `go test -race -count=1 ./internal/categories/domain/services/...`
- `golangci-lint run ./internal/agents/... ./internal/transactions/... ./internal/categories/... ./internal/card/...`

---

## Notas de implementação

1. **Não criar novos primitivos de plataforma**. Reutilizar `internal/platform/agent`, `internal/platform/tool`, `internal/platform/memory`, `internal/platform/workflow`.
2. **Não reimplementar lógica de domínio em tools/handlers**. As tools devem ser adapters finos; regras de negócio vivem em `domain/services` e use cases dos módulos financeiros.
3. **Parser de data relativa** deve ser função pura em `internal/agents/application/usecases` ou helper, sem IO, testável isoladamente.
4. **Integrar `IdempotentWrite` ao fluxo produtivo** em `internal/agents/application/workflows/pending_entry_workflow.go` (`executeWrite`), injetando o use case no `BuildPendingEntryWorkflow` e usando-o antes de chamar `ledger.CreateTransaction`/`CreateRecurringTemplate`. A chave deve ser `(wamid, itemSeq, operation)`.
5. **Se nova tool for necessária**, seguir o padrão `Build*Tool` com `tool.NewTool[I,O]`, schema JSON estrito e `exec` delegando para interface da aplicação.
6. **Manter separação `infrastructure -> application -> domain`** em todos os módulos envolvidos.
7. **Testes de regressão** devem cobrir: R1 a R8, datas relativas, ambiguidade de categoria, ambiguidade de cartão, idempotência por `wamid`, confirmação humana, cancelamento e valores inválidos.
8. **Formatação de centavos para reais** e **datas** devem usar funções puras reutilizáveis, garantindo consistência em todos os pontos de contato com o usuário.
