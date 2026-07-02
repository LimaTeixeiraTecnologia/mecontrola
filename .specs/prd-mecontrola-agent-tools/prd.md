# Documento de Requisitos do Produto (PRD) — Superfície de Tools do MeControla Agent

<!-- spec-version: 3 -->

> Entrada canônica: `docs/prompts/2026-07-02-create-prd-prompt-mecontrola-agent-tools.md`.
> Este PRD reflete estritamente o código real do workspace, verificado por inventário de código
> (arquivo:linha). Nenhuma tool, API, handler, workflow, use case, contrato ou comportamento foi
> inventado. Lacunas estão registradas em `Suposições e Questões em Aberto`.
> Skills obrigatórias para a futura implementação: `go-implementation` e `mastra`.
>
> **spec-version 3 (2026-07-02) — correção de premissa falsa comprovada por evidência de produção.**
> A confrontação do PRD com a conversa real do usuário `06edc407-4f63-42e8-b07c-946b9ef0a19c`
> (WhatsApp +5511986896322) no ambiente remoto revelou que o substrato de escrita/leitura assumido
> como funcional (bucket 1) está **quebrado**: o agente afirmou "Despesa registrada com sucesso ✅"
> com **0 linhas** em `transactions`/`agents_write_ledger`, e disse não encontrar orçamento existindo
> budget `2026-07` ativo. Ver seção `Evidência de Produção`. A spec-version 3 absorve a correção do
> substrato como pré-requisito **P0 bloqueante** (RF-37..RF-40), reintroduz o fluxo de clarificação de
> registro (RF-41..RF-43), adiciona a tool `list_categories` (RF-18e) e endurece o critério de aceite
> de escrita para exigir linhas reais verificadas no banco (RF-29/RF-33/M-05). Decisões D-07..D-10.

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
- **O-08 — Substrato de escrita/leitura confiável (P0 bloqueante).** A identidade e a idempotência das
  operações (`userId`, `wamid`, `itemSeq`) são injetadas **server-side** a partir do `InboundRequest`/
  contexto do Run, nunca fornecidas pelo LLM; nenhum "sucesso de escrita" é reportado ao usuário sem
  retorno real de sucesso do use case; e o Run auditável evidencia a escrita real (mensagens de tool e
  `resource_id` persistidos). Este objetivo corrige o defeito comprovado em produção e é pré-requisito
  para a exposição de qualquer tool nova.

### Métricas-chave a acompanhar

- **M-01 Cobertura de capacidade** = (capacidades relevantes com bucket atribuído) / (capacidades
  relevantes totais). Meta: 100%.
- **M-02 Gaps abertos** = capacidades relevantes classificadas como "a expor" ainda sem tool-alvo
  mapeada a use case real. Meta: 0.
- **M-03 Tools registradas exercidas** = (tools com ao menos uma execução real observada em
  ambiente de validação) / (tools registradas). Meta: 100%.
- **M-04 Taxa de acerto de seleção de tool** (via scorer de tool-call accuracy sobre um conjunto de
  cenários canônicos determinísticos, com uma tool esperada por cenário). Meta: **≥ 0.90**.
- **M-05 Incidentes de sucesso simulado** (resposta afirmando execução sem escrita real). Medição
  **determinística por assert de linhas no banco**: cada cenário de escrita do harness real-LLM DEVE
  verificar a existência das linhas correspondentes em `transactions`/`transactions_card_purchases`/
  `agents_write_ledger`/`transactions_recurring_templates`; texto de sucesso do agente NÃO conta como
  evidência. Meta: 0.
- **M-06 Operações destrutivas sem confirmação** observadas em validação. Meta: 0.
- **M-07 Escritas com identidade injetada server-side** = (escritas cujo `userId`/`wamid`/`itemSeq`
  vêm do `InboundRequest`/contexto) / (total de escritas). Nenhuma escrita pode depender de valores de
  identidade/idempotência fornecidos pelo LLM. Meta: 100%.

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
- **US-10** — Como usuário, quero listar as categorias disponíveis ("quais categorias existem?"), para
  saber como classificar meus gastos sem adivinhar.
- **US-11** — Como usuário, quando registro uma despesa/receita, quero que o agente só me pergunte a
  categoria quando ela estiver ausente ou ambígua e assuma a data por padrão (hoje/"ontem"), para não
  ter atrito desnecessário — e quando confirmar sucesso, que a operação esteja de fato gravada.
- **US-12** — Como operador, quero que "sucesso de escrita" reportado ao usuário seja sempre lastreado
  por linha real no banco, e que a identidade/idempotência seja injetada pelo servidor, para eliminar
  sucesso simulado e escrita perdida.

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
8. **Substrato de escrita/leitura confiável (P0 bloqueante).** Injeção server-side de identidade/
   idempotência (`userId`/`wamid`/`itemSeq`), guard bloqueante que impede reportar sucesso de escrita
   sem retorno real do use case, e Run auditável que persiste mensagens de tool e evidencia a escrita.
9. **Clarificação de registro (categoria/data).** Fluxo que pergunta a categoria apenas quando ausente/
   ambígua e resolve a data por default determinístico, reutilizando o substrato `ConfirmState` com um
   `OperationKind` não-destrutivo, sem criar mecanismo HITL paralelo.
10. **Listar categorias.** Tool de listagem das categorias disponíveis do usuário, mapeada ao use case
    real `ListCategories`.

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
- **RF-18e** — O agente DEVE poder **listar as categorias disponíveis** do usuário (use case
  `internal/categories/application/usecases/list_categories.go` — `ListCategories`), atendendo pedidos
  como "quais categorias existem/estão disponíveis?". A interface `CategoriesReader`
  (`internal/agents/application/interfaces/categories_reader.go`, hoje apenas `SearchDictionary`/
  `ResolveRootsBySlug`) DEVE ser estendida com um método de listagem. Esta capacidade sai do Fora de
  Escopo FE-08 (que permanece válido apenas para navegação de dicionário `GetCategory`/`ListDictionary`).
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

### Substrato de escrita/leitura confiável (P0 — bloqueante)

> Corrige defeito comprovado em produção (ver seção `Evidência de Produção`). Estes requisitos são
> pré-requisito bloqueante: nenhuma tool nova (RF-09..RF-18e) é considerada coberta enquanto o
> substrato não estiver corrigido e verificado (RF-40).

- **RF-37** — Injeção server-side de identidade/idempotência. As tools de escrita e de leitura por
  usuário (`register_expense`, `register_income`, `register_card_purchase`, `query_month`,
  `query_plan`, `create_recurrence`, e toda tool nova que precise de `userId`/`wamid`/`itemSeq`) NÃO
  DEVEM receber `userId`, `wamid` ou `itemSeq` como argumentos fornecidos pelo LLM. Esses valores DEVEM
  ser injetados **server-side** a partir do `InboundRequest`/contexto do Run no ponto de invocação da
  tool (`internal/platform/agent`, `invokeToolCall`), e removidos do schema exposto ao modelo. (Defeito:
  `internal/agents/application/tools/register_expense.go:52` marca `wamid`/`itemSeq`/`userId` como
  `required` do LLM com `Strict:true`; `internal/platform/agent/runtime.go:173-193` — `buildMessages`
  — nunca injeta `in.ResourceID`/`in.MessageID`, então o modelo não pode fornecê-los corretamente.)
- **RF-38** — Guard bloqueante de anti-simulação. O runtime NÃO DEVE reportar sucesso de operação de
  escrita ao usuário sem que a tool de escrita correspondente tenha retornado um `ToolOutcome` real de
  sucesso (`routed`/`reconciled`/`replay`). É PROIBIDO marcar `RunStatusSucceeded`/`ToolOutcomeRouted`
  apenas por `result.Content` não-vazio quando a intenção do usuário era uma escrita e nenhuma tool de
  escrita retornou sucesso. (Defeito: `internal/platform/agent/runtime.go:155-162` marca sucesso por
  qualquer conteúdo não-vazio; o `anyFinancialToolScorer` roda assíncrono e não bloqueia a resposta em
  `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go:163`.)
- **RF-39** — Run auditável com evidência de escrita real. Cada execução DEVE persistir as mensagens de
  tool (`memory.RoleTool`) e o `resource_id` retornado pela escrita, de modo que o Run distinga escrita
  real de texto de sucesso. (Defeito: `internal/platform/agent/runtime.go:138-153` só persiste
  `RoleUser`/`RoleAssistant`; `RoleTool` é definido mas nunca gravado.)
- **RF-40** — Premissa corrigida (bucket 1 não é assumido funcional). A correção do substrato
  (RF-37..RF-39) é pré-requisito **bloqueante** para a exposição das tools novas: nenhuma tool nova é
  considerada "coberta"/"exercida" enquanto o substrato não for corrigido e verificado por escrita real
  no banco (RF-29, RF-33). Isto substitui, para o eixo de plataforma, a premissa da spec-version 2 de
  que as tools de escrita/leitura do bucket 1 já funcionavam.

### Clarificação de registro (categoria/data)

- **RF-41** — O agente DEVE clarificar a **categoria** de um lançamento antes de gravar **apenas quando**
  ela estiver ausente ou ambígua (não resolvida com confiança por `classify_category`). Quando a
  categoria é resolvida com confiança, o agente grava sem perguntar (RF-21 — pede apenas o dado
  faltante).
- **RF-42** — O agente DEVE resolver a **data** do lançamento por default determinístico (data corrente
  em `America/Sao_Paulo`; inferindo "ontem"/data relativa/data explícita quando o usuário indicar) **sem
  perguntar**; confirmação de data só quando genuinamente ambígua.
- **RF-43** — O estado de espera da clarificação de registro DEVE reutilizar o substrato `ConfirmState`
  (`internal/agents/application/workflows/confirm_state.go`) com um `OperationKind` **não-destrutivo**
  dedicado (ex.: `OpConfirmRegister`), respeitando o contrato de pending step (persistir o estado antes
  de perguntar, retomar por merge-patch antes de qualquer parse, concluir o Run deterministicamente —
  R-AGENT-WF-001.7). PROIBIDO criar um mecanismo HITL paralelo ao existente.

### Observabilidade e auditabilidade

- **RF-27** — Todo uso de tool DEVE ser observável como Run auditável contendo, no mínimo,
  `thread_id`, `run_id`, `agent_id`, tool, `status` (tipo fechado), duração e erro quando houver
  (R-AGENT-WF-001.5). Escritas referenciam o identificador de decisão do audit trail.
- **RF-28** — As métricas de uso de tool DEVEM ter cardinalidade controlada: labels restritos a
  enums fechados (ex.: `agent_id`, `channel`, `tool`, `status`, `outcome`); PROIBIDO usar `user_id`,
  `correlation_key` ou `category_id` como label.

### Validação de uso efetivo (não apenas registro no runtime)

- **RF-29** — A solução DEVE comprovar que cada tool registrada é **exercida em execução real** — não
  basta estar presente no runtime. A evidência é o conjunto de Runs auditáveis e o resultado de um
  scorer de acurácia de tool-call (`internal/platform/scorer`, tool esperada por cenário) sobre um
  conjunto de cenários canônicos que cubra toda tool registrada. Para cenários de **escrita**, a
  evidência DEVE incluir **assert de linhas reais** nas tabelas de destino (`transactions`,
  `transactions_card_purchases`, `agents_write_ledger`, `transactions_recurring_templates`) executado
  no harness real-LLM (`RUN_REAL_LLM=1` + `OPENROUTER_*`); nem o Run marcar sucesso, nem o scorer
  indicar tool chamada, contam como prova de escrita.
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
  M-01 = 100%, M-02 = 0, M-03 = 100%, M-05 = 0 (verificado por assert de linhas no banco — RF-29),
  M-06 = 0, M-07 = 100%, e M-04 atinge o alvo declarado no critério de aceite; o substrato P0
  (RF-37..RF-40) está corrigido e verificado; toda operação destrutiva tem gate de confirmação (RF-22);
  todo uso de tool é auditável com evidência de escrita real (RF-27/RF-39); e o mapa capacidade→tool e o
  relatório de gaps estão atualizados contra o código real.
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
| `list_categories` | `categories/list_categories.go` (`ListCategories`) | Não (estende `CategoriesReader`) | Não | RF-18e |

### Bucket 3 — Existentes e a NÃO expor como tool conversacional (jobs/consumers/infra)

| Capacidade / use case | Módulo | Motivo |
|---|---|---|
| `EvaluateThresholdAlerts`, `NotifyThresholdAlert`, `EvaluateAlert` | budgets | job/consumer proativo, não conversacional |
| `ApplyPendingEvent`, `RunPendingEventsReaper`, `SignalAbandonedDrafts`, `PurgeRetention` | budgets | jobs periódicos/infra |
| `CreateOrAutoDraftForExpense`, `UpsertExpense`, `DeleteExpense` | budgets | event-driven a partir de eventos de transactions |
| `EvaluateInvoiceDueAlerts`, `NotifyInvoiceDue` | card | job/consumer proativo |
| `RecomputeMonthlySummary`, `ReconcileMonthlySummary`, `MaterializeRecurringForDay` | transactions | jobs/consumers |
| `ResolveBySlug`, `ValidateSubcategory` | categories | infra cross-module, não conversacional |

## Evidência de Produção (spec-version 3)

> Fonte: conversa real do usuário `06edc407-4f63-42e8-b07c-946b9ef0a19c` (WhatsApp +5511986896322),
> banco `mecontrola_db` no host remoto, thread `platform_threads`
> `bbf7c466-83f4-45ef-b4a5-edc98358cf1c`, 2026-07-01. Fatos verificados por consulta ao banco; nenhum
> valor inventado. Esta seção justifica os RFs P0 (RF-37..RF-40) e o endurecimento do aceite (RF-29/33).

- **EP-01 — Sucesso alucinado (escrita perdida).** O agente respondeu "Despesa registrada com sucesso
  ✅" para "compra no mercado R$150" e para "compra no mercado de R$1.500", mas
  `select count(*) from transactions where user_id = '06edc407-…'` retornou **0** e
  `agents_write_ledger` retornou **0**. Nenhuma escrita ocorreu. → RF-37, RF-38, RF-39, M-05, M-07.
- **EP-02 — Leitura de orçamento inoperante.** O agente respondeu "não encontrei seu plano orçamentário
  para julho de 2026" repetidamente, embora exista `budgets` `competence='2026-07'`,
  `total_cents=800000`, `state=2` (ativo) para o usuário. A tool `query_plan` não foi efetivamente
  exercida (mesma causa raiz de identidade não injetada). → RF-37, RF-40.
- **EP-03 — Listar categorias sem instrumento.** O usuário pediu "Quais são as categorias disponíveis?"
  e o agente respondeu com pergunta de mês/ano de orçamento (tool errada). Não havia tool de listagem. →
  RF-18e (tool `list_categories`).
- **EP-04 — Atrito de confirmação inconsistente.** No primeiro registro o agente pediu categoria e
  confirmação de data; em outro registrou "instantâneo" sem categoria — comportamento inconsistente. →
  RF-41 (categoria só quando ausente/ambígua), RF-42 (data por default), RF-43 (estado de espera único).
- **EP-05 — Run auditável não discrimina.** 16 linhas em `platform_runs`, todas `status=succeeded`,
  `outcome=routed`, sem erro, apesar de 0 escritas; `platform_messages` contém apenas `role=user`/
  `assistant`, nenhuma `role=tool`. O Run marca sucesso por conteúdo não-vazio. → RF-38, RF-39.

## Experiência do Usuário

- Canal: WhatsApp, **texto apenas**, respostas em PT-BR, formatação markdown do WhatsApp (regras já
  presentes em `mecontrola_agent.go`).
- Uma pergunta por mensagem; o agente pede apenas o dado faltante para completar a ação.
- Operações destrutivas produzem uma mensagem de confirmação com nota de impacto e aguardam
  `sim/não` (fluxo `destructive-confirm`, TTL 5 min, re-pergunta única).
- No registro de lançamentos, o agente pergunta a categoria **apenas quando ausente/ambígua** e assume
  a data por default (hoje/"ontem"/data explícita) **sem perguntar** (RF-41/RF-42).
- Sucesso de escrita só é confirmado ao usuário quando o use case retornou sucesso real e a linha
  existe no banco (RF-38); nunca há confirmação de gravação sem gravação.
- Quando a capacidade não existe, resposta honesta de indisponibilidade — nunca sucesso simulado.

## Restrições Técnicas de Alto Nível

- **RTA-01** — A capacidade agentiva consome o substrato `internal/platform/{agent,llm,memory,
  workflow,tool,scorer}`; comportamento novo entra como tool/workflow/scorer no consumidor
  `internal/agents`, montando primitivos — nunca reimplementando o substrato nem roteando por
  `switch case intent.Kind` (R-AGENT-WF-001.1).
- **RTA-02** — Tools são adapters finos (R-ADAPTER-001.2 / R-AGENT-WF-001.2): zero regra de negócio,
  SQL direto ou branching de domínio; delegam a um único use case/binding.
- **RTA-03** — Estados de fronteira (`ToolOutcome`/`RunStatus`/`ConfirmState`/`OperationKind`) são
  tipos fechados (DMMF state-as-type); nunca string livre. O `OperationKind` não-destrutivo
  `OpConfirmRegister` (RF-43) é enumerado no mesmo tipo fechado, sem string solta.
- **RTA-08** — Identidade e idempotência (`userId`/`wamid`/`itemSeq`) são injetadas server-side no ponto
  de invocação de tool (`internal/platform/agent`, `invokeToolCall`) a partir do `InboundRequest`/
  contexto do Run; PROIBIDO expô-las no schema de tool ou confiar em valor fornecido pelo LLM (RF-37).
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
- **FE-03** — Novos use cases de negócio nos módulos de domínio: este PRD só expõe capacidades **já
  existentes**; não cria capacidade nova de domínio. **Exceção de plataforma (spec-version 3):** a
  correção do substrato de agent/runtime (injeção server-side de identidade/idempotência, guard
  bloqueante de anti-simulação e persistência de mensagens de tool — RF-37..RF-40) está **dentro** do
  escopo, pois é infraestrutura de plataforma de agent (`internal/platform/agent`), não capacidade de
  domínio.
- **FE-04** — Multi-canal/voz/imagem: mantém-se texto-WhatsApp.
- **FE-05** — Reforma do fluxo de onboarding (`onboarding-workflow`) além do necessário para não
  regredir a superfície existente.
- **FE-06** — Roteamento por intent/kind ou motor de decisão paralelo ao registry.
- **FE-07** — Excluir orçamento draft via conversa (`DeleteDraftBudget`): capacidade real existe, mas
  foi deliberadamente deixada fora desta iteração (operação destrutiva de baixa demanda
  conversacional; exclusão de draft continua disponível fora do agente). Decisão QA-01.
- **FE-08** — Navegar catálogo/dicionário de categorias via tools próprias de **dicionário**
  (`GetCategory`/`ListDictionary`): permanece fora por já ser parcialmente atendido por
  `classify_category` e por risco de redundância conversacional. **Alteração spec-version 3:**
  **`ListCategories`** (listar as categorias disponíveis do usuário) sai deste Fora de Escopo e entra
  como tool `list_categories` (RF-18e), pois o usuário real pediu explicitamente a listagem e o agente
  não tinha instrumento. Decisão D-08.
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
- **D-07 (spec-version 3 — substrato quebrado).** A correção do substrato de escrita/leitura (injeção
  server-side de `userId`/`wamid`/`itemSeq`, guard bloqueante de anti-simulação, Run auditável com
  mensagens de tool) é **absorvida neste PRD como P0 bloqueante** (RF-37..RF-40), não separada em outro
  documento. Motivo: sem ela, cada tool nova nasce com o mesmo defeito de sucesso alucinado comprovado
  em produção.
- **D-08 (spec-version 3 — listar categorias).** Adicionada a tool `list_categories` (RF-18e) mapeada
  ao use case real `ListCategories`; FE-08 é estreitado para cobrir apenas navegação de dicionário.
  Motivo: pedido explícito do usuário real ("quais categorias disponíveis?") ficou descoberto.
- **D-09 (spec-version 3 — clarificação de registro).** Reintroduzido o fluxo de clarificação de
  registro (RF-41..RF-43): categoria perguntada **apenas quando ausente/ambígua**, data por **default
  determinístico sem perguntar**, estado de espera reutilizando `ConfirmState` com `OperationKind`
  não-destrutivo (`OpConfirmRegister`), sem mecanismo HITL paralelo.
- **D-10 (spec-version 3 — aceite de escrita).** O critério de aceite de escrita exige **assert de
  linhas reais no banco** no harness real-LLM (RF-29/RF-33/M-05); texto de sucesso do agente não conta
  como evidência.
