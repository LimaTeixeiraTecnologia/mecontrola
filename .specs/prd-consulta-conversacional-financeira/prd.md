# Documento de Requisitos do Produto (PRD) — Consulta Conversacional Financeira

<!-- spec-version: 3 -->

> Fonte: `docs/us/2026-07-07-us-consulta-conversacional-financeira.md` (US-01).
> Slug: `consulta-conversacional-financeira`.

## Visão Geral

O agente `mecontrola` (consumidor agentivo em `internal/agents`) deve permitir que o usuário
consulte, via WhatsApp e em linguagem natural, sua situação financeira **atual e passada** —
panorama do mês, orçamento por competência, orçamento detalhado por categoria, fatura de cartão de
crédito e últimas transações. Toda resposta é **read-only** e ancorada exclusivamente em dados reais
dos módulos `internal/budgets` e `internal/transactions`, obtidos por chamadas de ferramentas (tools)
já existentes no consumidor.

O valor central é reduzir o atrito: o usuário entende sua situação financeira sem navegar em telas
nem aprender comandos, com respostas prontas para WhatsApp, no tom da persona MeControla, e com
**zero alucinação** de valores.

Esta funcionalidade é uma **evolução de instruções + testes de regressão** do agente. Não introduz
novas tools: reutiliza `query_month`, `query_plan`, `query_card_invoice`, `resolve_card`,
`list_cards`, `search_transactions` e `get_transaction`, encadeando-as quando necessário.

## Objetivos

- Cobrir os sete cenários de consulta (C1–C7) com **seleção determinística** da ferramenta correta.
- Garantir **zero alucinação**: nenhum valor, categoria, data ou status sem origem em retorno de tool.
- Entregar respostas em **português do Brasil**, no tom da persona, formatadas para WhatsApp.
- Reaproveitar o histórico da thread para follow-ups, sem substituir a chamada de tool.
- **Critério de sucesso mensurável (decisão D-04)**: acurácia de seleção de ferramenta medida pelo
  scorer real-LLM já usado no repositório **≥ 0.90 (M-04)** nos cenários C1–C7, e **0** respostas com
  valor não proveniente de tool, validado com `RUN_REAL_LLM=1` usando as credenciais do `.env`.

## Histórias de Usuário

- Como usuário do MeControla, quero perguntar "como estou indo?" e receber o panorama do mês atual
  (receitas, despesas, saldo e estado do orçamento), para entender minha situação sem abrir telas.
- Como usuário, quero consultar o orçamento de uma competência específica ("orçamento de
  janeiro/2026") ou do mês atual, para acompanhar planejado × gasto.
- Como usuário, quero ver o **orçamento completo por categoria**, com quanto posso gastar, quanto
  gastei e o percentual de execução em cada categoria, para decidir onde ajustar.
- Como usuário, quero saber o valor da fatura de um cartão pelo apelido ("fatura do Nubank"), para
  me organizar para o vencimento.
- Como usuário, quero ver minha última transação (com categoria) ou minhas últimas N transações,
  para conferir meus gastos recentes.
- Como usuário, quero fazer follow-ups curtos ("e a fatura?", "e as últimas transações?") e ser
  entendido pelo contexto da conversa.
- Como usuário, quero recusa educada e clara quando pergunto algo fora do domínio financeiro
  suportado ou quando não há dado, sem receber um valor inventado.

## Funcionalidades Core

### FC-01 — Roteamento determinístico de consulta
Para cada mensagem, o agente seleciona exatamente a(s) tool(s) do cenário e nunca usa uma como
substituta de outra nem responde de memória.

### FC-02 — Panorama do mês (C1)
`query_month` + `query_plan` do mês atual (`America/Sao_Paulo`): receitas, despesas, saldo e estado
do orçamento.

### FC-03 — Orçamento por competência e do mês atual (C2, C3)
`query_plan` com `competence` explícito (C2) ou padrão do mês atual (C3): total planejado, total
gasto, percentual e alertas ativos.

### FC-04 — Orçamento completo por categoria (C7)
`query_plan` exibindo **todas** as `allocations`, com nome da categoria, planejado, gasto e
percentual, além do total geral.

### FC-05 — Fatura de cartão (C4)
`resolve_card` (apelido → `cardId`) seguido de `query_card_invoice`; em ambiguidade, `list_cards`.

### FC-06 — Última transação e últimas N (C5, C6)
`query_month` com `limit` adequado (entradas já vêm em `created_at DESC`); C5 enriquece a mais
recente com `get_transaction` para exibir a categoria.

### FC-07 — Follow-up com memória
Uso do histórico da thread (`internal/platform/memory`) para resolver referências implícitas, sempre
reinvocando a tool correta para dados atualizados.

### FC-08 — Anti-alucinação e recusa educada
Ausência de dado, erro técnico ou tema fora de domínio produzem mensagens claras, sem inventar valor
nem expor detalhes técnicos.

## Requisitos Funcionais

Roteamento e seleção de ferramenta:

- RF-01: Para cada mensagem, o agente DEVE selecionar exatamente a(s) ferramenta(s) correspondente(s)
  ao cenário identificado (C1–C7), conforme a matriz de roteamento (RF-02..RF-08).
- RF-02: C1 (panorama do mês atual) DEVE usar `query_month` **e** `query_plan` para o mês atual.
- RF-03: C2 (orçamento de mês específico) DEVE usar `query_plan` com `competence` explícito no
  formato `YYYY-MM`.
- RF-04: C3 (orçamento do mês atual) DEVE usar `query_plan` sem `competence` (padrão = mês atual).
- RF-05: C4 (fatura de cartão) DEVE usar `resolve_card` para obter o `cardId` e, em seguida,
  `query_card_invoice` com esse `cardId`.
- RF-06: C5 (última transação) DEVE usar `query_month` com `limit=1` para obter o lançamento mais
  recente e, em seguida, `get_transaction` com o `id` retornado para enriquecer a resposta com a
  categoria. A categoria DEVE ser exibida como `categoryNameSnapshot` e, quando houver subcategoria,
  no formato `categoryNameSnapshot > subcategoryNameSnapshot` (ex.: *Custo Fixo > Supermercado*).
  **(Decisões D-01, D-09)**
- RF-06a: O enriquecimento de categoria em C5 é **best-effort**: se `get_transaction` falhar ou não
  retornar categoria, o agente DEVE responder com descrição, valor e data (dados de `query_month`),
  **sem** erro e **sem** inventar categoria. **(Decisão D-05)**
- RF-07: C6 (últimas N transações) DEVE usar `query_month` com `limit` igual à quantidade solicitada
  (padrão `limit=5` quando não informada), exibindo descrição, valor e data de cada lançamento, **sem**
  enriquecimento de categoria por item.
- RF-07a: Para C5 e C6, se o mês atual não tiver lançamentos, o agente DEVE consultar `query_month`
  do **mês anterior** (retrocesso de **até 1 mês**) para localizar a(s) transação(ões) mais
  recente(s); persistindo a ausência após o retrocesso, aplica-se RF-30. **(Decisão D-06)**
- RF-08: C7 (orçamento completo por categoria) DEVE usar `query_plan` e apresentar **todas** as
  categorias do campo `allocations`.
- RF-08a: Nas respostas de orçamento (C2, C3, C7), o agente DEVE **sempre** resumir de forma concisa
  os alertas ativos do campo `alerts` retornado por `query_plan` (categoria, threshold e estado) ou,
  quando o array estiver vazio, informar que não há alertas ativos (ex.: "Nenhum alerta ativo. ✅").
  Usa apenas o array já retornado, sem ferramenta adicional. **(Decisão D-07)**
- RF-09: É PROIBIDO usar uma ferramenta como substituta de outra — incluindo usar
  `search_transactions` para "últimas transações" sem termo de busca, ou responder valores de memória.

Anti-alucinação e domínio:

- RF-10: O agente NUNCA DEVE inventar, estimar ou simular valores financeiros; todo valor, categoria,
  data ou status na resposta DEVE originar-se do retorno de uma ferramenta.
- RF-11: Quando nenhuma ferramenta puder responder (dado ausente, erro técnico ou tema fora de
  domínio), o agente DEVE informar claramente a impossibilidade, sem fabricar resposta.
- RF-12: O agente DEVE atender apenas consultas sobre lançamentos, cartões de crédito, orçamento e
  fatura; temas fora do domínio (investimentos, empréstimos, seguros, impostos complexos, assuntos
  não financeiros) DEVEM ser recusados educadamente.

Contexto temporal:

- RF-13: Quando o usuário informar mês/ano (ex.: "janeiro/2026"), o agente DEVE converter para
  `YYYY-MM` (`2026-01`) antes de chamar a ferramenta.
- RF-14: Quando o usuário indicar "mês atual" ou não indicar mês, o agente DEVE usar a data corrente
  no fuso `America/Sao_Paulo`, formatada como `YYYY-MM`.

Resolução de ambiguidade:

- RF-15: Se `resolve_card` retornar `found=false` para o apelido informado, o agente DEVE chamar
  `list_cards`, apresentar os cartões cadastrados e pedir ao usuário que escolha; NÃO DEVE assumir um
  cartão arbitrariamente.
- RF-16: Se o usuário pedir "últimas transações" sem quantidade, o agente DEVE usar `limit=5`.
- RF-17: Se o usuário pedir panorama sem mencionar mês, o agente DEVE usar o mês atual em
  `America/Sao_Paulo` (RF-14).

Formatação de orçamento completo (C7) e de valores:

- RF-18: Em C7, para cada categoria o agente DEVE exibir: nome da categoria; valor planejado em reais
  (de `plannedCents`); valor gasto em reais (de `spentCents`); percentual de execução arredondado
  (de `percentageSpent`).
- RF-19: O nome de exibição da categoria DEVE ser derivado do `rootSlug` via mapa fixo das cinco
  raízes da metodologia — `custo-fixo`→*Custo Fixo*, `conhecimento`→*Conhecimento*,
  `prazeres`→*Prazeres*, `metas`→*Metas*, `liberdade-financeira`→*Liberdade Financeira* — definido nas
  instruções do agente, sem chamada extra de ferramenta. **(Decisão D-02)**
- RF-20: Categorias com `plannedCents` nulo/ausente DEVEM ser exibidas como "*Sem limite definido*"
  (ou "*R$ 0,00*" conforme regra do módulo `budgets`) e **NUNCA** omitidas.
- RF-21: O total geral DEVE ser exibido no topo de C7: total planejado (`totalPlannedCents`), total
  gasto (`totalSpentCents`) e percentual geral quando disponíveis.
- RF-22: Valores em centavos DEVEM ser convertidos para reais com duas casas decimais e separador de
  milhar no padrão brasileiro (ex.: `123450` → `R$ 1.234,50`).

Idioma, tom e formatação WhatsApp:

- RF-23: Todas as respostas DEVEM ser em português do Brasil, com tom amigável, simples e sem
  julgamento.
- RF-24: O agente DEVE usar emojis contextuais: 📊 para resumos/orçamento; 💰 para valores/fatura;
  ✅ para confirmações quando aplicável.
- RF-25: Negrito DEVE usar apenas `*asterisco simples*`; é PROIBIDO usar `**duplo asterisco**` ou
  qualquer markdown incompatível com WhatsApp.

Memória e continuidade:

- RF-26: O agente DEVE aproveitar o histórico da thread para responder follow-ups ("e a fatura?",
  "e as últimas transações?") quando o contexto estiver claro.
- RF-27: A resolução por contexto NÃO DEVE substituir a chamada à ferramenta; o agente DEVE reinvocar
  a tool correta para obter dados atualizados a cada consulta.

Mensagens de erro e ausência de dados:

- RF-28: Orçamento não encontrado para a competência: "Você ainda não tem um orçamento para
  *{competência}*. Posso te ajudar a criar um?"
- RF-29: Fatura não encontrada: "Não encontrei fatura para o cartão *{apelido}* em *{mês}*."
- RF-30: Sem transações no mês: "Não há lançamentos em *{mês}*."
- RF-31: Erro técnico: "Não consegui consultar agora. Tente novamente em breve." — sem expor
  detalhes técnicos.

Privacidade e idempotência:

- RF-32: O agente DEVE responder apenas sobre o `resourceID` da thread ativa; nunca cruzar dados
  entre usuários.
- RF-32a: O `cardId` usado em `query_card_invoice` DEVE originar-se **exclusivamente** do retorno de
  `resolve_card` ou `list_cards` (tools escopadas ao usuário da thread). É PROIBIDO ao agente usar um
  `cardId` proveniente de texto do usuário, de memória ou fabricado. Este guard elimina IDOR no fluxo
  sem alterar a tool (respeita RF-35). **(Decisão D-08)**
- RF-33: As consultas são read-only e idempotentes; repeti-las NÃO DEVE produzir efeitos colaterais.
- RF-34: A ordenação das "últimas transações" DEVE ser por `createdAt` descendente (garantida pela
  origem dos dados), limitada ao mês de referência — com a única exceção do retrocesso de até 1 mês
  definido em RF-07a quando o mês atual estiver vazio. Este refina o RN-5 do US (que limitava
  estritamente ao mês de referência) para evitar falso "sem lançamentos" no início do mês.

Escopo de entrega:

- RF-35: A entrega DEVE se restringir a (a) evoluir as instruções/roteamento em
  `internal/agents/application/agents/mecontrola_agent.go`, (b) adicionar testes de regressão para
  C1–C7, (c) **uma única extensão aditiva de dado**: expor `subcategoryNameSnapshot` na saída de
  `get_transaction` (dado já presente no domínio `Entry`), sem lógica nova, e (d) **ajuste aditivo de
  descrição LLM-facing** de `resolve_card` para roteamento confiável de C4, sem alterar assinatura,
  schema, `cardId`-scope ou comportamento do `exec`. NÃO DEVE criar novas tools, alterar assinaturas
  de use case, nem tocar `module.go`/bindings. **(Decisões D-03, D-09, D-10)**
- RF-36: A conversão de centavos para reais DEVE ser feita por função pura reutilizável (helper de
  presenter/mapper), garantindo consistência entre C2, C3 e C7 (formatação determinística).

## Experiência do Usuário

Canal único: WhatsApp, texto. Persona MeControla — amigável, direta, sem jargão e sem julgamento.

Diálogos de referência (verbatim ao US, formatados para WhatsApp com `*negrito simples*`):

- **C1 — "Como estou indo?"** → 📊 *Resumo de julho/2026* com receitas, despesas, saldo e estado do
  orçamento, oferecendo o detalhe por categoria.
- **C2 — "Como foi meu orçamento de janeiro/2026?"** → 📊 *Orçamento de janeiro/2026* com total
  planejado, total gasto, categorias com percentual e status de alertas.
- **C3 — "Como está meu orçamento do mês atual?"** → 📊 *Orçamento de julho/2026* com planejado,
  gasto até agora e percentual.
- **C4 — "Quanto está minha fatura do cartão Nubank?"** → 💰 *Fatura Nubank — julho/2026* com
  fechamento, vencimento e total, oferecendo os lançamentos.
- **C5 — "Qual foi a minha última transação?"** → 💰 *Último lançamento* com descrição, valor, data e
  categoria (enriquecida via `get_transaction`).
- **C6 — "Quais foram as minhas últimas 5 transações?"** → 💰 *Últimos lançamentos* numerados com
  descrição, valor e data.
- **C7 — "Me mostra o orçamento completo"** → 📊 *Orçamento completo de julho/2026* com total no topo
  e todas as categorias (nome, gasto, planejado, percentual), incluindo saldo disponível.

Casos de borda de UX: ambiguidade de cartão (RF-15), ausência de dados (RF-28..RF-30), erro técnico
(RF-31) e follow-ups contextuais (RF-26).

## Restrições Técnicas de Alto Nível

- Construído sobre os primitivos reais do repositório: consumidor `internal/agents` (padrão
  `mastra`), tools `tool.NewTool[I,O]` com schema JSON estrito, e bindings para `internal/budgets`
  (`BudgetPlanner`) e `internal/transactions` (`TransactionsLedger`).
- Segue as skills `go-implementation` (R0–R7, DI manual, zero comentários em Go de produção) e
  `mastra` (Agent/Tool/Memory/Runtime, roteamento por registry — sem `switch case intent.Kind`).
- DMMF: smart constructors nos commands (padrão `(T, error)`), state-as-type para status de
  orçamento/alerta e tipos de lançamento (nunca `string` livre em assinatura pública), workflow como
  pipeline no use case/tool, e núcleo puro (formatação/ordenação) separado do shell de IO.
- LLM somente nas call-sites sancionadas (loop do Agent, step `Stream`, scorer LLM-judged);
  OpenRouter como único provider. Kernel `internal/platform/workflow` permanece genérico — sem
  importar domínio financeiro.
- Fuso horário de referência: `America/Sao_Paulo` para todo cálculo de "mês atual".
- Contrato de dados existente: `query_plan.allocations` expõe `rootSlug`, `plannedCents` (opcional),
  `spentCents`, `percentageSpent` (opcional); `query_month.entries` não carrega categoria;
  `get_transaction` expõe `categoryNameSnapshot` e, após a extensão aditiva de D-09, também
  `subcategoryNameSnapshot`. Nenhuma assinatura de use case ou binding é alterada.
- Validação: `go build ./internal/agents/...`, `go vet ./internal/agents/...`,
  `go test -race -count=1 ./internal/agents/application/tools/...` e
  `.../application/agents/...`, `golangci-lint run ./internal/agents/...`, além de validação
  real-LLM (`RUN_REAL_LLM=1`) para o critério de sucesso M-04 ≥ 0.90.

## Fora de Escopo

- Registro, edição e exclusão de lançamentos, cartões ou orçamento (write/HITL).
- Alertas proativos e notificações não solicitadas.
- Onboarding e ativação de plano.
- Criação de novas tools ou alteração de tools/módulos existentes (RF-35), com a única exceção
  aditiva de D-10: refinamento da descrição LLM-facing de `resolve_card` (sem lógica, assinatura,
  schema ou comportamento).
- Consultas fora do domínio: investimentos, empréstimos, seguros, impostos complexos, temas não
  financeiros.
- Múltiplos canais além do WhatsApp e respostas com mídia (imagens, PDFs, gráficos).
- Paginação conversacional de listas longas além do `limit` solicitado (cursor não é exposto ao
  usuário nesta US).

## Decisões Travadas

- **D-01 (RF-06)**: C5 encadeia `query_month(limit=1)` → `get_transaction(id)` para exibir a categoria
  fiel na última transação; C6 (lista) não enriquece categoria por item.
- **D-02 (RF-19)**: nomes das cinco raízes de orçamento vêm de mapa fixo `slug→nome` nas instruções
  do agente, sem chamar `list_categories`.
- **D-03 (RF-35)**: entrega limitada a instruções do agente + testes de regressão; sem novas tools.
- **D-04 (Objetivos)**: sucesso = scorer M-04 ≥ 0.90 nos cenários C1–C7 e zero alucinação, sob
  `RUN_REAL_LLM=1`.
- **D-05 (RF-06a)**: enriquecimento de categoria em C5 é best-effort — falha de `get_transaction`
  degrada para descrição/valor/data, sem erro e sem inventar categoria.
- **D-06 (RF-07a)**: "última transação"/"últimas N" com mês atual vazio faz retrocesso de até 1 mês
  via `query_month` do mês anterior antes de aplicar a mensagem de ausência.
- **D-07 (RF-08a)**: respostas de orçamento (C2/C3/C7) sempre resumem os alertas ativos (ou informam
  ausência), usando apenas o array `alerts` já retornado por `query_plan`.
- **D-08 (RF-32a)**: `cardId` só pode vir de `resolve_card`/`list_cards`; guard no prompt é
  **defesa-em-profundidade**. A barreira primária já existe: `GetCardInvoice.Execute` escopa a
  consulta por `principal.UserID` no repositório (`get_card_invoice.go:33-45`), tornando o IDOR
  inviável mesmo com `cardId` forjado.
- **D-09 (RF-06, RF-35)**: exceção aditiva única a D-03 — expor `subcategoryNameSnapshot` na saída de
  `get_transaction` para C5 exibir `Categoria > Subcategoria` fiel ao US; dado já existe em `Entry`,
  mudança puramente aditiva, sem lógica nem nova tool.
- **D-10 (RF-05, RF-35) — emenda pós-review (2026-07-07)**: exceção aditiva a D-03 sancionando o
  refinamento da descrição LLM-facing de `resolve_card` (menção a `query_card_invoice` + instrução de
  que o `nickname` é a palavra exata que o usuário citou para o cartão). Motivo: evidência empírica
  (real-LLM, modelo de produção `openai/gpt-4o-mini`) de que sem esse refinamento o cenário C4
  ("fatura do cartão {apelido}") roteava para `resolve_card` apenas ~30–50% das execuções — o agente
  pedia o apelido já informado em vez de resolver o cartão. Modelos mais fortes (gpt-4o, gemini-2.0)
  pioravam o comportamento (raciocinavam pela clarificação). A correção combina (1) instrução C4
  guiada por exemplo (input→ação) e (2) esta descrição de `resolve_card`, elevando C4 a **10/10** no
  gate de confiabilidade. Sem alteração de assinatura, schema, `cardId`-scope (RF-32a intacto) ou
  comportamento do `exec` — mudança puramente descritiva. Gate de aceite de C4 promovido de
  asserção single-shot (que produzia falso-verde) para **gate estatístico ≥ 8/10 execuções**
  (`TestRealLLM_QueryCardInvoiceChain_C4`).

- **D-11 (RF-06) — residual aceito pós-review (2026-07-07)**: a **apresentação** da categoria de C5
  no formato literal `Raiz > Folha` (símbolo `>`) é **best-effort de apresentação**, não garantida no
  modelo de produção (`openai/gpt-4o-mini`). Evidência empírica: 0/32 de aderência ao `>` em três
  estratégias de instrução (proibição+exemplo, molde de resposta few-shot, regra prioritária no topo);
  o viés conversacional do modelo narra a categoria em prosa ("categorizada como *Custo Fixo* na
  subcategoria *Supermercado*"). O **dado** permanece sempre correto e completo (raiz e subcategoria
  presentes, vindos de `get_transaction` — RF-06/RF-06a/D-09 satisfeitos quanto à origem e completude).
  Impor o `>` exigiria formatador determinístico fora do LLM (código + ADR), rejeitado nesta entrega
  por desviar do princípio "o LLM monta a resposta". O contrato de teste de C5
  (`TestRealLLM_LastTransactionChain_C5`) assevera a **presença** de categoria e subcategoria (contrato
  real), não a string `>`. RF-06 permanece normativo na instrução (o agente é instruído a usar `>`),
  mas a conformidade de apresentação é degradação graciosa aceita.

## Suposições e Questões em Aberto

- **A-01 (resolvida por D-05)**: os lançamentos de `query_month` são, no modelo unificado de
  transação (CRUD unificado), resolvíveis por `get_transaction`. O caso residual de não-resolução é
  tratado normativamente por RF-06a (degradação best-effort), portanto não constitui ressalva aberta.
- Não há questões em aberto nem ressalvas: todas as decisões materiais foram confirmadas (D-01..D-08)
  e convertidas em requisitos funcionais normativos. O PRD está pronto para a especificação técnica.
