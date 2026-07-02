# Documento de Requisitos do Produto (PRD) — Superfície de Tools do MeControla Agent

<!-- spec-version: 2 -->

> Entrada canônica: `docs/prompts/2026-07-02-create-prd-prompt-mecontrola-agent-tools.md`.
> Este PRD reflete estritamente o código real do workspace, verificado por inventário de código
> (arquivo:linha). Nenhuma tool, API, handler, workflow, use case, contrato ou comportamento foi
> inventado. Lacunas estão registradas em `Suposições e Questões em Aberto`.
> Skills obrigatórias para a futura implementação: `go-implementation` e `mastra`.

## Visão Geral

O MeControla é um monolito modular em Go. Sua capacidade agentiva usa o substrato
`internal/platform/{agent,llm,memory,workflow,tool,scorer}` e o consumidor `internal/agents`, cujo
agente de referência é `mecontrola-agent` (`internal/agents/application/agents/mecontrola_agent.go`).
O agente conversa com o usuário via WhatsApp e opera sobre os módulos de negócio `internal/budgets`,
`internal/card`, `internal/categories` e `internal/transactions`.

Hoje o agente expõe **9 tools** (registradas em `internal/agents/module.go:254-262`). Os quatro
módulos de negócio, porém, possuem capacidades de aplicação (use cases) relevantes ao usuário final
que **não estão disponíveis ao agente** — algumas nem sequer possuem binding em
`internal/agents/infrastructure/binding/`. O resultado é uma superfície de tools **incompleta**: o
usuário pode pedir ações legítimas (ex.: "qual a fatura do meu cartão este mês?", "busque meus
gastos com mercado", "crie um lançamento recorrente", "qual o melhor dia pra comprar no cartão?") e
o agente não tem instrumento para executá-las — cenário que induz respostas evasivas ou, pior,
simulação de sucesso sem execução real.

O problema de produto é: **a superfície de tools do `mecontrola-agent` não cobre, de forma completa,
precisa e efetivamente usada, as capacidades relevantes dos módulos de negócio, e não há critério
objetivo que impeça declarar "pronto" enquanto existir capacidade relevante não refletida no agente
ou tool registrada mas não usada de fato.**

Este PRD define a necessidade de produto para: (1) fechar os gaps de cobertura com um **conjunto-alvo
concreto de tools** ancorado em use cases reais; (2) tornar o mapeamento `capacidade do módulo → tool
do agente` formal, auditável e sem gaps; (3) garantir que a disponibilidade das tools seja
**efetiva** (usada em execução, determinística e auditável), não apenas nominal; e (4) estabelecer
gates objetivos que impeçam falso positivo de cobertura, desvio de domínio e simulação de sucesso.

Sucesso **não** significa "ter mais tools". Significa **cobertura correta, uso correto, aderência ao
código real e comportamento confiável em produção**.

## Objetivos

- **O-01 — Cobertura completa e verificada.** Toda capacidade de aplicação relevante ao usuário final
  em `budgets`, `card`, `categories` e `transactions` está classificada em exatamente um dos três
  buckets (exposta hoje / a expor / não expor) e, quando "a expor", tem uma tool-alvo nomeada e
  mapeada a um use case real.
- **O-02 — Zero gap silencioso.** Não existe capacidade relevante do módulo que fique fora dos três
  buckets sem justificativa registrada; a métrica de gaps abertos é 0 no critério de aceite.
- **O-03 — Uso efetivo, não nominal.** Cada tool registrada é comprovadamente exercida em execução
  real (evidência via Run auditável e/ou scorer de acurácia de tool-call), não apenas presente no
  runtime.
- **O-04 — Zero simulação de sucesso.** O agente nunca declara que uma ação foi executada sem que a
  tool correspondente tenha retornado sucesso real do use case.
- **O-05 — Segurança de operações destrutivas.** Toda operação destrutiva ou sensível passa por
  confirmação humana explícita antes da efetivação.
- **O-06 — Auditabilidade e observabilidade.** Todo uso de tool é observável como Run auditável com
  cardinalidade de métricas controlada.
- **O-07 — Aderência estrita ao domínio.** O agente permanece dentro do domínio financeiro pessoal do
  MeControla; nenhuma tool habilita ação fora desse domínio.

### Métricas-chave a acompanhar

- **M-01 Cobertura de capacidade** = (capacidades relevantes com bucket atribuído) / (capacidades
  relevantes totais). Meta: 100%.
- **M-02 Gaps abertos** = capacidades relevantes classificadas como "a expor" ainda sem tool-alvo
  mapeada a use case real. Meta: 0.
- **M-03 Tools registradas exercidas** = (tools com ao menos uma execução real observada em
  ambiente de validação) / (tools registradas). Meta: 100%.
- **M-04 Taxa de acerto de seleção de tool** (via scorer de tool-call accuracy sobre um conjunto de
  cenários canônicos determinísticos, com uma tool esperada por cenário). Meta: **≥ 0.90**.
- **M-05 Incidentes de sucesso simulado** (resposta afirmando execução sem retorno de sucesso real
  do use case), medidos em suíte de validação. Meta: 0.
- **M-06 Operações destrutivas sem confirmação** observadas em validação. Meta: 0.

## Histórias de Usuário

Ator principal: **usuário final do MeControla** (pessoa física gerindo finanças pessoais via
WhatsApp, canal texto). Ator secundário: **operador/mantenedor** que audita comportamento e cobertura.

- **US-01** — Como usuário, quero consultar a fatura do meu cartão em um mês, para saber quanto vou
  pagar, sem sair da conversa.
- **US-02** — Como usuário, quero listar e ver detalhes dos meus cartões, para conferir apelido,
  banco e dia de vencimento.
- **US-03** — Como usuário, quero saber o melhor dia para comprar no cartão, para maximizar o prazo
  até o vencimento.
- **US-04** — Como usuário, quero buscar meus lançamentos por descrição (ex.: "mercado"), para
  encontrar um gasto específico rapidamente.
- **US-05** — Como usuário, quero criar, listar, editar e excluir lançamentos recorrentes, para não
  registrar manualmente contas fixas todo mês.
- **US-06** — Como usuário, quero editar os dados de um cartão (apelido/banco/dia de vencimento),
  para corrigir um cadastro, com confirmação antes de aplicar.
- **US-07** — Como usuário, quando peço algo que o agente não pode fazer, quero uma resposta honesta
  ("ainda não consigo fazer isso"), nunca uma confirmação falsa de sucesso.
- **US-08** — Como operador, quero auditar cada uso de tool (thread, run, tool, status, duração,
  erro), para investigar comportamento e comprovar execução real.
- **US-09** — Como operador, quero um mapa formal `capacidade → tool` e um relatório de gaps, para
  saber, a qualquer momento, o que está coberto e o que não está.

## Funcionalidades Core

1. **Conjunto-alvo concreto de tools.** Especificação, capacidade por capacidade, de quais use cases
   reais viram tool nova, com nome de tool proposto e binding/use case de destino.
2. **Mapa formal `capacidade do módulo → tool do agente`.** Artefato mantido e verificável que lista
   toda capacidade relevante e sua decisão de bucket.
3. **Relatório de gaps.** Diferença formal entre as capacidades dos módulos e a tool surface atual.
4. **Uso determinístico das tools.** Regras de seleção de tool, coleta apenas do dado faltante e
   execução da ação correta, de forma reprodutível.
5. **Gate de operações destrutivas.** Reuso do fluxo de confirmação existente
   (`destructive-confirm`, `internal/agents/application/workflows/destructive_confirm_workflow.go`)
   para toda operação destrutiva/sensível nova.
6. **Auditabilidade e anti-simulação.** Run auditável por execução e proibição de declarar sucesso
   sem retorno real do use case.
7. **Gates anti-falso-positivo.** Critérios objetivos que bloqueiam "sucesso" enquanto houver gap
   aberto ou tool registrada não exercida.

## Requisitos Funcionais

> Convenção: capacidades e tools citadas abaixo são **verificadas no código atual**. Nomes de tool
> "a expor" são propostas de produto; o nome final e o desenho são responsabilidade da Especificação
> Técnica. IDs `RF-nn` são rastreáveis por `check-spec-drift`.

### Cobertura funcional exaustiva

- **RF-01** — O produto DEVE classificar cada capacidade de aplicação relevante ao usuário final dos
  módulos `internal/budgets`, `internal/card`, `internal/categories` e `internal/transactions` em
  exatamente um dos três buckets definidos na seção "Mapa Capacidade → Tool": (1) já exposta hoje;
  (2) existente e a expor; (3) existente e a NÃO expor como tool conversacional.
- **RF-02** — A classificação DEVE referenciar o use case real (arquivo/construtor) que sustenta cada
  capacidade; capacidade sem use case real correspondente é proibida de constar como coberta.
- **RF-03** — Nenhuma capacidade relevante pode ficar sem bucket. Capacidade sem decisão registrada
  é tratada como gap aberto e bloqueia o critério de sucesso (ver RF-30).

### Mapeamento formal capacidade → tool

- **RF-04** — O produto DEVE manter um mapa formal `capacidade do módulo → tool do agente`, com uma
  linha por capacidade relevante, contendo: módulo, use case real, bucket, tool atual ou tool-alvo,
  e (quando aplicável) se a capacidade já possui binding em
  `internal/agents/infrastructure/binding/` ou requer novo binding.
- **RF-05** — O mapa DEVE distinguir explicitamente capacidades que já possuem binding mas **não**
  possuem tool (ex.: `CardManager.ListCards`, hoje usada apenas internamente por
  `register_card_purchase`) de capacidades que **não possuem binding nem tool** (ex.:
  `SearchTransactions`, `GetCardInvoice`, `BestPurchaseDay`, `UpdateCard`, recurring templates).
- **RF-06** — O mapa DEVE ser a fonte única de verdade de cobertura, versionada junto ao PRD, de modo
  que a verificação de gaps (RF-07) opere sobre ele.

### Identificação formal de gaps

- **RF-07** — O produto DEVE produzir um relatório de gaps que enumere toda capacidade do bucket "a
  expor" ainda sem tool-alvo mapeada, e toda tool registrada sem capacidade/use case real
  correspondente. A meta de aceite é 0 em ambas as direções.
- **RF-08** — O relatório de gaps DEVE ser reprodutível a partir do código real (comparação entre os
  use cases dos módulos e as tools registradas em `internal/agents/module.go`), não de memória.

### Conjunto-alvo concreto de tools (bucket 2 → tools novas)

> Escopo confirmado com o solicitante (decisões D-01/D-02): os grupos abaixo entram como tools
> conversacionais — cartão (listar/detalhar/contar), fatura, melhor dia de compra, editar cartão,
> busca de lançamentos, recorrências (CRUD), detalhes de lançamento, listar compras do cartão e
> sugerir distribuição. Cada RF nomeia a capacidade real de destino.

- **RF-09** — O agente DEVE poder **listar cartões** do usuário (capacidade `CardManager.ListCards`;
  use case `internal/card/application/usecases/list_cards.go`), hoje sem tool própria de consulta.
- **RF-10** — O agente DEVE poder **consultar o detalhe de um cartão** (use case
  `internal/card/application/usecases/get_card.go` — `GetCard`), hoje sem binding e sem tool.
- **RF-11** — O agente DEVE poder **consultar a fatura de um cartão por mês** (use case
  `internal/transactions/application/usecases/get_card_invoice.go` — `GetCardInvoice`), hoje sem
  binding e sem tool.
- **RF-12** — O agente DEVE poder **informar o melhor dia de compra** no cartão (use case
  `internal/card/application/usecases/best_purchase_day.go` — `BestPurchaseDay`), hoje sem binding e
  sem tool.
- **RF-13** — O agente DEVE poder **buscar lançamentos por descrição** (use case
  `internal/transactions/application/usecases/search_transactions.go` — `SearchTransactions`), hoje
  sem binding e sem tool.
- **RF-14** — O agente DEVE poder **listar lançamentos recorrentes** (use case
  `internal/transactions/application/usecases/list_recurring_templates.go`), hoje ausente da
  superfície.
- **RF-15** — O agente DEVE poder **criar um lançamento recorrente** (use case
  `internal/transactions/application/usecases/create_recurring_template.go`), hoje ausente da
  superfície.
- **RF-16** — O agente DEVE poder **editar um lançamento recorrente** (use case
  `internal/transactions/application/usecases/update_recurring_template.go`), sob gate de confirmação
  (RF-22), hoje ausente da superfície.
- **RF-17** — O agente DEVE poder **excluir um lançamento recorrente** (use case
  `internal/transactions/application/usecases/delete_recurring_template.go`), sob gate de confirmação
  (RF-22), hoje ausente da superfície.
- **RF-18** — O agente DEVE poder **editar os dados de um cartão** — apelido, banco ou dia de
  vencimento (use case `internal/card/application/usecases/update_card.go` — `UpdateCard`), hoje sem
  binding e sem tool. A confirmação (gate destrutivo, RF-22) é EXIGIDA **apenas quando a edição
  altera o dia de vencimento** (impacta cálculo de faturas); edição isolada de apelido ou banco é
  aplicada diretamente, sem gate.
- **RF-18a** — O agente DEVE poder **consultar o detalhe de um lançamento** (use cases
  `internal/transactions/application/usecases/get_transaction.go` — `GetTransaction` — e
  `.../get_card_purchase.go` — `GetCardPurchase`), hoje sem tool.
- **RF-18b** — O agente DEVE poder **listar compras de um cartão** (use case
  `internal/transactions/application/usecases/list_card_purchases.go` — `ListCardPurchases`), hoje
  sem tool.
- **RF-18c** — O agente DEVE poder **contar os cartões ativos** do usuário (use case
  `internal/card/application/usecases/count_cards.go` — `CountCards`), hoje sem binding e sem tool.
- **RF-18d** — O agente DEVE poder **sugerir a distribuição de alocação** do orçamento (use case
  `internal/budgets/application/usecases/suggest_allocation.go` — `SuggestAllocation`), como leitura/
  cálculo puro (sem escrita), hoje sem binding e sem tool.
- **RF-19** — Cada tool nova DEVE ser um adapter fino que delega a um único use case/binding real
  (R-ADAPTER-001 / R-AGENT-WF-001.2): sem regra de negócio, SQL direto ou branching de domínio na
  tool. (Restrição de produto derivada das regras hard do repositório; o desenho fica na techspec.)
- **RF-20** — Cada tool nova DEVE ser efetivamente **registrada no agente** (via
  `buildFinancialTools`/`BuildMeControlaAgent` em `internal/agents/module.go`) e **declarada nas
  instruções** de `mecontrola_agent.go`, de modo que o modelo saiba quando e como usá-la.

### Uso determinístico das tools

- **RF-21** — O agente DEVE escolher a tool correta para a intenção do usuário de forma
  determinística e reprodutível para os cenários canônicos definidos (mesma entrada → mesma tool),
  pedir **apenas** o dado faltante para completar a chamada, e não solicitar dados que já possui no
  contexto/thread. Uma pergunta por mensagem (regra já presente nas instruções do agente).
- **RF-22** — Toda operação destrutiva ou sensível em escopo (excluir/editar lançamento,
  excluir/editar recorrência, excluir cartão via `delete_entry`, e editar cartão quando altera o dia
  de vencimento — RF-18) DEVE passar por confirmação humana explícita antes da efetivação,
  reutilizando o fluxo `destructive-confirm`
  (`internal/agents/application/workflows/destructive_confirm_workflow.go`) e seu estado fechado
  `ConfirmState`/`OperationKind` (`confirm_state.go`). Nenhuma operação destrutiva pode ser efetivada
  sem confirmação.
- **RF-23** — O gate de confirmação DEVE respeitar o contrato existente: persistir o estado de espera
  antes de perguntar, retomar por merge-patch antes de qualquer parse, re-perguntar uma vez em
  resposta ambígua, cancelar sem efeito em negativa/expiração (TTL de 5 minutos já implementado), e
  concluir o Run deterministicamente (sem draft órfão).

### Proibição de invenção e de simulação de sucesso

- **RF-24** — O agente é PROIBIDO de inventar dados, cartões, lançamentos, categorias, valores,
  identificadores, resultados ou capacidades que não venham do retorno real de um use case.
- **RF-25** — O agente é PROIBIDO de declarar que uma operação foi executada sem que a tool
  correspondente tenha retornado sucesso real do use case. Quando a capacidade não existir na
  superfície, o agente DEVE responder de forma honesta que ainda não consegue executar aquilo, em vez
  de simular sucesso ou improvisar uma resposta.
- **RF-26** — O agente é PROIBIDO de contornar o gate de destrutivas simulando uma confirmação em
  nome do usuário.

### Observabilidade e auditabilidade

- **RF-27** — Todo uso de tool DEVE ser observável como Run auditável contendo, no mínimo,
  `thread_id`, `run_id`, `agent_id`, tool, `status` (tipo fechado), duração e erro quando houver
  (R-AGENT-WF-001.5). Escritas referenciam o identificador de decisão do audit trail.
- **RF-28** — As métricas de uso de tool DEVEM ter cardinalidade controlada: labels restritos a
  enums fechados (ex.: `agent_id`, `channel`, `tool`, `status`, `outcome`); PROIBIDO usar `user_id`,
  `correlation_key` ou `category_id` como label.

### Validação de uso efetivo (não apenas registro no runtime)

- **RF-29** — A solução DEVE comprovar que cada tool registrada é **exercida em execução real** — não
  basta estar presente no runtime. A evidência é o conjunto de Runs auditáveis e/ou o resultado de um
  scorer de acurácia de tool-call (`internal/platform/scorer`, ex.: tool-call accuracy) sobre um
  conjunto de cenários canônicos que cubra toda tool registrada.
- **RF-30** — O critério de "sucesso" DEVE ser bloqueado enquanto: (a) existir capacidade relevante
  do módulo não refletida em nenhum bucket (RF-03); (b) existir tool do bucket 2 sem tool-alvo
  mapeada (RF-07); ou (c) existir tool registrada sem nenhuma execução real observada (RF-29). Estes
  são os gates anti-falso-positivo de cobertura.

### Anti-desvio de domínio

- **RF-31** — Nenhuma tool pode habilitar ação fora do domínio financeiro pessoal do MeControla. O
  agente DEVE recusar, de forma breve e redirecionando ao domínio, pedidos fora de escopo, sem
  inventar capacidade.
- **RF-32** — Capacidades de infraestrutura interna, jobs e consumers (bucket 3) são PROIBIDAS de
  virar tool conversacional (ver lista na seção "Mapa Capacidade → Tool").

### Critérios objetivos de production-ready (do ponto de vista de produto)

- **RF-33** — A solução só é classificável como production-ready quando, cumulativamente:
  M-01 = 100%, M-02 = 0, M-03 = 100%, M-05 = 0, M-06 = 0, e M-04 atinge o alvo declarado no critério
  de aceite; toda operação destrutiva tem gate de confirmação (RF-22); todo uso de tool é auditável
  (RF-27); e o mapa capacidade→tool e o relatório de gaps estão atualizados contra o código real.
- **RF-34** — "Ter mais tools" é explicitamente insuficiente: registrar uma tool sem uso efetivo
  comprovado, sem mapeamento a use case real, ou sem gate de destrutiva quando aplicável, NÃO conta
  como cobertura e mantém o critério de sucesso bloqueado.

### Consistência de escrita e validação

- **RF-35** — As novas tools de escrita DEVEM espelhar o padrão de idempotência/concorrência já
  usado no agente: **criações** (ex.: `create_recurrence`) reusam `IdempotentWrite` com a chave
  `(userID, wamid, itemSeq, operation)` — como `register_expense`/`register_income`/
  `register_card_purchase`; **edições e exclusões** (ex.: `update_recurrence`, `delete_recurrence`,
  `update_card`) usam concorrência otimista por `version` combinada ao gate destrutivo, como
  `edit_entry`/`delete_entry`. Nenhuma escrita nova pode ficar sem uma dessas garantias.
- **RF-36** — A validação de `go.mod` na implementação futura DEVE usar `go mod verify` +
  `go build ./...` + `go vet ./...` como gate (o script `scripts/verify-go-mod.sh` citado por
  `go-implementation` não existe no workspace e é substituído por esses comandos do toolchain). Esta
  é uma restrição de processo de implementação, não parte da superfície de tools.

## Mapa Capacidade → Tool (verificado no código)

> Fonte: inventário de código por arquivo:linha. Bindings em
> `internal/agents/infrastructure/binding/`. Tools em `internal/agents/application/tools/`.

### Bucket 1 — Já expostas hoje (9 tools, `internal/agents/module.go:254-262`)

| Tool (nome LLM) | Capacidade / use case real | Módulo |
|---|---|---|
| `register_expense` | `TransactionsLedger.CreateTransaction` (via `IdempotentWrite`) | transactions |
| `register_income` | `TransactionsLedger.CreateTransaction` (income) | transactions |
| `register_card_purchase` | `CardManager.ListCards` + `TransactionsLedger.CreateCardPurchase` | card + transactions |
| `query_month` | `TransactionsLedger.GetMonthlySummary` + `ListMonthlyEntries` | transactions |
| `query_plan` | `BudgetPlanner.GetMonthlySummary` + `ListAlerts` | budgets |
| `edit_entry` | gate `destructive-confirm` → `UpdateTransaction`/`UpdateCardPurchase` | transactions |
| `delete_entry` | gate `destructive-confirm` → `DeleteTransaction`/`DeleteCardPurchase`/`SoftDeleteCard` | transactions + card |
| `adjust_allocation` | `BudgetPlanner.EditCategoryPercentage` | budgets |
| `classify_category` | `CategoriesReader.SearchDictionary` | categories |

### Bucket 2 — Existentes e a expor (conjunto-alvo deste PRD)

| Tool-alvo (proposta) | Capacidade / use case real | Binding hoje? | Destrutiva? | RF |
|---|---|---|---|---|
| `list_cards` | `card/list_cards.go` (`ListCards`) | Sim (uso interno) | Não | RF-09 |
| `get_card` | `card/get_card.go` (`GetCard`) | Não | Não | RF-10 |
| `query_card_invoice` | `transactions/get_card_invoice.go` (`GetCardInvoice`) | Não | Não | RF-11 |
| `best_purchase_day` | `card/best_purchase_day.go` (`BestPurchaseDay`) | Não | Não | RF-12 |
| `search_transactions` | `transactions/search_transactions.go` (`SearchTransactions`) | Não | Não | RF-13 |
| `list_recurrences` | `transactions/list_recurring_templates.go` | Não | Não | RF-14 |
| `create_recurrence` | `transactions/create_recurring_template.go` | Não | Não | RF-15 |
| `update_recurrence` | `transactions/update_recurring_template.go` | Não | **Sim** | RF-16 |
| `delete_recurrence` | `transactions/delete_recurring_template.go` | Não | **Sim** | RF-17 |
| `update_card` | `card/update_card.go` (`UpdateCard`) | Não | **Cond.** (só dia venc.) | RF-18 |
| `get_transaction` | `transactions/get_transaction.go` (`GetTransaction`) | Não | Não | RF-18a |
| `get_card_purchase` | `transactions/get_card_purchase.go` (`GetCardPurchase`) | Não | Não | RF-18a |
| `list_card_purchases` | `transactions/list_card_purchases.go` (`ListCardPurchases`) | Não | Não | RF-18b |
| `count_cards` | `card/count_cards.go` (`CountCards`) | Não | Não | RF-18c |
| `suggest_allocation` | `budgets/suggest_allocation.go` (`SuggestAllocation`) | Não | Não | RF-18d |

### Bucket 3 — Existentes e a NÃO expor como tool conversacional (jobs/consumers/infra)

| Capacidade / use case | Módulo | Motivo |
|---|---|---|
| `EvaluateThresholdAlerts`, `NotifyThresholdAlert`, `EvaluateAlert` | budgets | job/consumer proativo, não conversacional |
| `ApplyPendingEvent`, `RunPendingEventsReaper`, `SignalAbandonedDrafts`, `PurgeRetention` | budgets | jobs periódicos/infra |
| `CreateOrAutoDraftForExpense`, `UpsertExpense`, `DeleteExpense` | budgets | event-driven a partir de eventos de transactions |
| `EvaluateInvoiceDueAlerts`, `NotifyInvoiceDue` | card | job/consumer proativo |
| `RecomputeMonthlySummary`, `ReconcileMonthlySummary`, `MaterializeRecurringForDay` | transactions | jobs/consumers |
| `ResolveBySlug`, `ValidateSubcategory` | categories | infra cross-module, não conversacional |

## Experiência do Usuário

- Canal: WhatsApp, **texto apenas**, respostas em PT-BR, formatação markdown do WhatsApp (regras já
  presentes em `mecontrola_agent.go`).
- Uma pergunta por mensagem; o agente pede apenas o dado faltante para completar a ação.
- Operações destrutivas produzem uma mensagem de confirmação com nota de impacto e aguardam
  `sim/não` (fluxo `destructive-confirm`, TTL 5 min, re-pergunta única).
- Quando a capacidade não existe, resposta honesta de indisponibilidade — nunca sucesso simulado.

## Restrições Técnicas de Alto Nível

- **RTA-01** — A capacidade agentiva consome o substrato `internal/platform/{agent,llm,memory,
  workflow,tool,scorer}`; comportamento novo entra como tool/workflow/scorer no consumidor
  `internal/agents`, montando primitivos — nunca reimplementando o substrato nem roteando por
  `switch case intent.Kind` (R-AGENT-WF-001.1).
- **RTA-02** — Tools são adapters finos (R-ADAPTER-001.2 / R-AGENT-WF-001.2): zero regra de negócio,
  SQL direto ou branching de domínio; delegam a um único use case/binding.
- **RTA-03** — Estados de fronteira (`ToolOutcome`/`RunStatus`/`ConfirmState`/`OperationKind`) são
  tipos fechados (DMMF state-as-type); nunca string livre.
- **RTA-04** — LLM apenas nas call-sites sancionadas (loop do agent, step que chama `Stream`, scorer
  LLM-judged); OpenRouter é o único provider. Kernel `internal/platform/workflow` permanece sem
  domínio/LLM.
- **RTA-05** — Zero comentários em código Go de produção (R-ADAPTER-001.1).
- **RTA-06** — Métricas com cardinalidade controlada (RF-28).
- **RTA-07** — Implementação futura obrigatoriamente conduzida sob as skills `go-implementation`
  (Etapas 1–5 + checklist R0–R7) e `mastra` (molde `internal/agents`).

## Fora de Escopo

- **FE-01** — Desenho técnico, assinaturas, esquemas de I/O das tools, novos bindings, pseudocódigo,
  diffs ou refactor — pertencem à Especificação Técnica.
- **FE-02** — Capacidades do bucket 3 (jobs/consumers/infra) como tool conversacional (RF-32).
- **FE-03** — Novos use cases de negócio nos módulos: este PRD só expõe capacidades **já existentes**;
  não cria capacidade nova de domínio.
- **FE-04** — Multi-canal/voz/imagem: mantém-se texto-WhatsApp.
- **FE-05** — Reforma do fluxo de onboarding (`onboarding-workflow`) além do necessário para não
  regredir a superfície existente.
- **FE-06** — Roteamento por intent/kind ou motor de decisão paralelo ao registry.
- **FE-07** — Excluir orçamento draft via conversa (`DeleteDraftBudget`): capacidade real existe, mas
  foi deliberadamente deixada fora desta iteração (operação destrutiva de baixa demanda
  conversacional; exclusão de draft continua disponível fora do agente). Decisão QA-01.
- **FE-08** — Navegar catálogo/dicionário de categorias via tools próprias
  (`GetCategory`/`ListCategories`/`ListDictionary`): deixado fora por já ser parcialmente atendido
  por `classify_category` e por risco de redundância conversacional. Decisão QA-01.
- **FE-09** — Expor o ciclo de vida de orçamento (`CreateBudget`/`ActivateBudget`/`CreateRecurrence`
  de budgets) como tools avulsas: permanece exclusivamente no `onboarding-workflow`; não vira tool
  conversacional avulsa nesta iteração.

## Decisões Resolvidas

> Todas as questões que estavam em aberto na spec-version 1 foram decididas com o solicitante.
> Nenhuma ressalva permanece. Registradas aqui para rastreabilidade.

- **D-01 (QA-01 — fronteira de escopo).** Entram no conjunto-alvo, além dos 4 grupos já confirmados:
  **detalhes/consultas de leitura** (`GetTransaction`, `GetCardPurchase`, `ListCardPurchases`,
  `CountCards` — RF-18a/18b/18c) e **sugestão de distribuição** (`SuggestAllocation` — RF-18d).
  Ficam **fora** (FE-07/FE-08/FE-09): `DeleteDraftBudget`, navegação de catálogo de categorias
  (`GetCategory`/`ListCategories`/`ListDictionary`) e ciclo de vida de orçamento avulso.
- **D-02 (QA-02 — `update_card`).** Confirmação exigida **apenas quando altera o dia de vencimento**;
  edição isolada de apelido/banco é direta (RF-18).
- **D-03 (QA-03 — meta M-04).** Taxa de acerto de seleção de tool **≥ 0.90** sobre conjunto canônico
  determinístico (uma tool esperada por cenário), no critério production-ready (M-04, RF-33).
- **D-04 (QA-04 — validação go.mod).** Gate = `go mod verify` + `go build ./...` + `go vet ./...`,
  substituindo o inexistente `scripts/verify-go-mod.sh` (RF-36).
- **D-05 (QA-05 — idempotência de escrita).** Criações reusam `IdempotentWrite`; edições/exclusões
  usam `version` + gate destrutivo (RF-35).
- **D-06 (QA-06 — relação com PRD amplo).** Este PRD é **complementar e independente** de
  `.specs/prd-mecontrola-agent/`; evolui com spec-version própria e não incrementa/regenera o PRD
  amplo nem seus artefatos downstream.
