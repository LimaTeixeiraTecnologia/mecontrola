# Documento de Requisitos do Produto (PRD)

<!-- spec-version: 1 -->

> Slug: `registro-conversacional-transacoes-dia-a-dia`
> Fonte: `docs/us/2026-07-07-us-registro-conversacional-transacoes-dia-a-dia.md`
> Data: 2026-07-07
> Módulos: `internal/agents` (consumidor agentivo), `internal/transactions`, `internal/categories`, `internal/card`, `internal/budgets`
> Skills obrigatórias na implementação: `@.agents/skills/go-implementation/`, `@.agents/skills/mastra/`, e `@.agents/skills/domain-modeling-production/` (DMMF: state-as-type, smart constructors, Decide* puro, pipeline parse→validate→decide→persist→publish).

## Visão Geral

Permitir que o usuário do MeControla registre receitas e despesas do dia a dia conversando em linguagem natural pelo WhatsApp, sem abrir telas nem memorizar comandos. Cada lançamento captura obrigatoriamente **data que a transação ocorreu**, **categoria raiz**, **subcategoria folha**, **descrição** e **valor**, orquestrado pelo agente `mecontrola` em `internal/agents` e persistido nos módulos financeiros.

Registrar uma transação é uma **operação de escrita financeira**. Por isso, o produto é regido por três invariantes inegociáveis:

1. **Zero alucinação** — nenhum campo obrigatório é inventado ou inferido sem evidência na mensagem do usuário.
2. **Zero duplicidade** — toda escrita passa por idempotência determinística por `wamid`.
3. **Confirmação humana universal** — nenhuma transação é persistida sem confirmação explícita do usuário, reafirmando o gate universal já travado na PRD `conversa-agentiva-fluida`.

Esta PRD **evolui um fluxo já parcialmente existente**. A base (`register_expense`, `register_income`, `pending-entry workflow`, `RegisterAttempt`, campo `occurredAt`, parser relativo `hoje/ontem/anteontem`, thresholds `0.80`/`0.55`, `IdempotentWrite`, `WriteLedgerRepository` e a tabela `agents_write_ledger`) **já está no código**. O foco é fechar lacunas reais e endurecer o comportamento contra falsos positivos.

### Correções de premissa (confronto com o código real)

Durante o discovery, três afirmações da User Story foram **refutadas contra o código** e ficam corrigidas nesta PRD para evitar retrabalho e falso positivo:

- **`occurredAt` NÃO é um gap total.** O campo já existe nos schemas de `register_expense`/`register_income`, na entidade `Transaction` (`occurredAt time.Time`), no comando `CreateTransaction` e em `resolveEntryDate` (que já assume "hoje" em `America/Sao_Paulo`). O gap real de data é **apenas** o parsing de **dias da semana**.
- **O parser de data relativa já existe** para `hoje`/`ontem`/`anteontem` + `DD/MM` em `internal/agents/application/workflows/pending_entry_decisions.go`. Não recriar; **estender**.
- **O limite de valor "R$ 99.999.999,99" é fictício.** O VO `valueobjects.Money` valida apenas `cents > 0`, sem teto. O teto será um sanity-check na camada do agente (ver RF-05), sem alterar o invariante de domínio compartilhado.

## Objetivos

- Registrar receitas e despesas do dia a dia via WhatsApp com os cinco campos obrigatórios sempre presentes e validados.
- **Eliminar o gap crítico de idempotência**: integrar `IdempotentWrite` ao caminho produtivo de escrita (`executeWrite` do `pending-entry workflow`), hoje ausente.
- Interpretar datas explícitas, relativas (incluindo **dias da semana**) e implícitas ("hoje") com precisão e sem ambiguidade silenciosa.
- Garantir categorização determinística e segura: auto-selecionar só quando `score ≥ 0,80` e não ambíguo; suspender e perguntar caso contrário.
- Resolver cartão de crédito por apelido sem falso positivo; nunca chutar cartão nem número de parcelas.

### Métricas de sucesso

- **M-01 — Completude de campos:** 100% das transações persistidas contêm os cinco campos obrigatórios válidos (data, categoria raiz, subcategoria folha, descrição, valor > 0). Meta: 100% (invariante bloqueante, não amostral).
- **M-02 — Zero duplicidade:** 0 escritas duplicadas para o mesmo `(wamid, itemSeq, operation)`, medido no `agents_write_ledger`. Meta: 0.
- **M-03 — Zero alucinação de campo:** em validação real-LLM sobre os cenários R1–R7, 0 lançamentos com valor, data, categoria, subcategoria ou cartão inventados sem evidência na mensagem.
- **M-04 — Acurácia conversacional:** harness real-LLM dos cenários de aceite com score ≥ 0,90 (padrão do projeto para features agentivas).
- **M-05 — Confirmação universal:** 0 escritas persistidas sem confirmação humana explícita (`M-07 = 0` na terminologia da PRD `conversa-agentiva-fluida`).

## Histórias de Usuário

- **US-01 (primária):** Como usuário do MeControla, quero conversar com o agente para registrar minhas receitas e despesas do dia a dia usando frases naturais, para que eu não precise abrir telas nem memorizar comandos e meu controle financeiro acompanhe minha vida real sem esforço.
- **US-02:** Como usuário, quero informar datas do jeito que falo ("ontem", "terça"), para que o lançamento reflita quando o dinheiro realmente se moveu, não quando mandei a mensagem.
- **US-03:** Como usuário, quero registrar compras parceladas no cartão, para que cada parcela caia na fatura correta segundo o ciclo do cartão.
- **US-04:** Como usuário, quero que o agente me pergunte quando estiver em dúvida (categoria, cartão, data), para que nada seja registrado errado ou inventado.
- **US-05:** Como usuário, quero confirmar cada lançamento antes de gravar, para que eu tenha controle total do que entra no meu histórico.

### Cenários de referência

| ID | Intenção | Exemplo de frase |
|---|---|---|
| R1 | Despesa simples via Pix | "Gastei R$ 150,00 no supermercado no pix" |
| R2 | Despesa em dinheiro, data relativa | "Ontem fui na feira e gastei cinquenta reais em pastel no dinheiro" |
| R3 | Compra parcelada no crédito | "Comprei uma geladeira na Casas Bahia de R$ 2.000,00 em 10x no cartão Nubank" |
| R4 | Compra à vista no crédito | "Paguei R$ 120,00 de gasolina no cartão Inter" |
| R5 | Receita fixa | "Recebi meu décimo terceiro de R$ 10.000,00" |
| R6 | Receita variável | "Recebi duzentos reais de um freelancer" |
| R7 | Categoria incerta | "Gastei 35 reais com algo pro trabalho" |
| R8 | Múltiplas transações (fora de escopo, ver RF-16) | "Hoje gastei 30 reais no ônibus e 15 no café" |

## Funcionalidades Core

1. **Registro conversacional de despesa** — extrai valor, descrição, forma de pagamento e data; classifica categoria; confirma; persiste com idempotência.
2. **Registro conversacional de receita** — mesma jornada, sem forma de pagamento (o sistema fixa `pix`), com categoria de `Kind = income`.
3. **Compra no cartão de crédito (à vista ou parcelada)** — resolve cartão por apelido, valida parcelas 1..24 e calcula o mês de referência da primeira parcela pelo ciclo do cartão.
4. **Interpretação de datas** — explícitas, relativas (`hoje`/`ontem`/`anteontem` + **dias da semana**) e implícitas ("hoje"), sempre em `America/Sao_Paulo`.
5. **Categorização determinística** — auto-seleção só acima do limiar; suspensão conversacional com candidatos caso contrário.
6. **Gate de confirmação universal + idempotência** — resumo humano antes de toda escrita e dedup por `(wamid, itemSeq, operation)`.

## Requisitos Funcionais

### Captura obrigatória e validação

- **RF-01:** Toda transação persistida DEVE conter os cinco campos obrigatórios: (1) data que a transação ocorreu, (2) categoria raiz válida, (3) subcategoria folha ligada à raiz, (4) descrição, (5) valor positivo em centavos. Se qualquer um dos campos 1–4 não puder ser extraído ou validado, o agente DEVE suspender no workflow `pending-entry` e perguntar ao usuário — nunca preencher com valor inventado.
- **RF-02:** A direção da transação DEVE ser inferida do contexto verbal: "gastei/paguei/comprei/gasto" → despesa (`outcome`); "recebi/ganhei/caiu/entrada" → receita (`income`). A direção determina o `Kind` de categoria exigido.
- **RF-03:** A subcategoria DEVE ser uma folha diretamente ligada à categoria raiz escolhida, válida e não descontinuada, resolvida por `ResolveCategoryForWrite`. Vale tanto para despesa quanto para receita (reafirma a decisão "income exige subcategoria folha").
- **RF-04:** O valor DEVE ser validado por `valueobjects.Money` (estritamente positivo). Valores zero ou negativos são rejeitados com pedido de correção.
- **RF-05:** O agente DEVE aplicar um **sanity-check de teto de valor na camada da aplicação agentiva** (limite máximo configurável, default sugerido R$ 10.000.000,00), rejeitando valores acima do teto com pedido de correção. Este teto NÃO altera o invariante do VO `Money` — permanece exclusivo da fronteira do agente para não impactar `transactions`, `budgets` e `card`. _(Decisão D-01)_

### Datas

- **RF-06:** O agente DEVE interpretar datas em `America/Sao_Paulo` com base na data real atual:
  - "hoje" → data atual; "ontem" → −1 dia; "anteontem" → −2 dias (já existentes, manter).
  - `DD/MM` e `DD/MM/AAAA` explícitos (já existentes, manter).
- **RF-07:** O agente DEVE **estender** o parser com **dias da semana** ("segunda".."domingo"), resolvendo para a ocorrência mais recente daquele dia: se hoje for o próprio dia, "<dia>" isolado = hoje; "<dia> passada/passado" = −7 dias em relação à ocorrência corrente. Esta lógica DEVE ser uma **função pura determinística**, sem IO, testável isoladamente (Pure core / IO shell). _(Decisão D-02)_
- **RF-08:** Expressões de baixa precisão ("semana passada", "mês passado") DEVEM ser rejeitadas, com o agente pedindo uma data específica. _(Decisão D-02)_
- **RF-09:** Quando o usuário não informar data, o agente DEVE assumir **hoje**. A data assumida DEVE aparecer **explicitamente no resumo único de confirmação** (ex.: "hoje (07/07/2026)"). Não há passo extra de confirmação de data — o "sim" do gate universal confirma tudo, inclusive a data assumida. _(Decisão D-06)_
- **RF-10:** A data extraída DEVE ser convertida para `YYYY-MM-DD` e usada como `OccurredAt`. Para pagamentos não-cartão, `OccurredAt` define o `RefMonth` (`RefMonthFromTime`). Para cartão de crédito, `OccurredAt` é a data da compra e o `RefMonth` da primeira parcela é calculado pelo `BillingCycleResolver` a partir do ciclo do cartão.

### Categorização

- **RF-11:** O agente DEVE usar a tool `classify_category` para resolver categoria/subcategoria a partir da descrição, restringindo a busca ao `Kind` correspondente à direção da transação (income vs expense), de modo que o `Kind` da categoria sempre coincida com a direção.
- **RF-12:** Regras de decisão de categoria (thresholds já existentes em `match_score.go`: `ScoreAutoThreshold = 0.80`, `ScoreConfirmThreshold = 0.55`):
  - `score ≥ 0,80` e `outcome = matched` e não ambíguo → auto-selecionar o par `(RootCategoryID, SubcategoryID)` validado por `ResolveCategoryForWrite` e prosseguir para o resumo de confirmação.
  - `0,55 ≤ score < 0,80` ou `outcome = ambiguous` → suspender no `pending-entry` e apresentar até 3 candidatos para escolha.
  - `score < 0,55` ou `outcome = no_match` → suspender e perguntar "Em qual categoria isso se encaixa?".
- **RF-13:** É proibido ao agente escolher categoria "por eliminação" ou inventar categoria/subcategoria inexistente no catálogo editorial.

### Cartão

- **RF-14:** Quando a forma de pagamento for cartão de crédito, o agente DEVE: extrair o apelido/banco; chamar `resolve_card`; se não resolver, chamar `list_cards` e apresentar os cartões para escolha. Se o usuário disser "no cartão" sem especificar qual, o agente DEVE perguntar antes de prosseguir. Oferecer criar cartão inexistente está fora de escopo (redirecionar).
- **RF-15:** O número de parcelas DEVE ser extraído explicitamente e validado em 1..24 (`valueobjects.InstallmentCount`). Se não informado, assumir **1 (à vista)** e refletir no resumo de confirmação. O valor total é dividido pelo `InstallmentSplitter` com resto nas primeiras parcelas.

### Múltiplas transações (fronteira de escopo)

- **RF-16:** O MVP registra **uma transação por mensagem**. Quando a mensagem contiver múltiplos lançamentos (ex.: R8), o agente DEVE reconhecer a situação e **pedir que o usuário envie um lançamento por mensagem, sem persistir nenhum**. Suporte a multi-item (`itemSeq > 0` no mesmo `wamid`) é **non-goal** desta PRD e endereçado em PRD futura. _(Decisões D-03 e D-05)_

### Confirmação humana

- **RF-17:** Antes de toda escrita, o agente DEVE apresentar um resumo e pedir confirmação explícita. Conteúdo do resumo:
  - **Despesa:** descrição, valor, `categoria > subcategoria`, data, forma de pagamento.
  - **Cartão de crédito:** o acima + cartão e número de parcelas.
  - **Receita:** descrição, valor, `categoria > subcategoria`, data (sem forma de pagamento).
- **RF-18:** A confirmação positiva ("sim", "ok", "pode registrar", "confirmo") efetiva a escrita; a negativa ("não", "cancela", "errado") cancela e pede correção. Enquanto não confirmado, a transação permanece no estado `pending` do workflow `pending-entry`.

### Idempotência (gap crítico a fechar)

- **RF-19:** Toda escrita via agente DEVE passar por `IdempotentWrite` com chave `(wamid, itemSeq, operation)`, integrando o use case ao `executeWrite` do `pending-entry workflow` — hoje `executeWrite` chama `callLedger` diretamente, sem idempotência (gap confirmado no código: `IdempotentWrite` só é instanciado em testes). `IdempotentWrite` DEVE ser injetado em `BuildPendingEntryWorkflow` e acionado antes de `ledger.CreateTransaction`/`CreateRecurringTemplate`.
- **RF-20:** A chave de idempotência é derivada da **mensagem original do lançamento** (`wamid` original + `itemSeq` + `operation`), não da mensagem de confirmação. Reenvio da mensagem original OU replay da confirmação DEVEM deduplicar para a mesma escrita. Se a chave já foi processada, o agente retorna `ToolOutcomeReplay` e a resposta já computada, sem persistir novamente. _(Decisão D-04)_

### Anti-simulação e idioma

- **RF-21:** O agente NUNCA deve inventar valor, data, categoria, subcategoria, cartão ou forma de pagamento, nem inferir uma nova transação a partir de memória de transações passadas. Informação incompleta/ambígua → pedir esclarecimento.
- **RF-22:** Todas as respostas em português do Brasil, tom amigável e sem julgamento, com emojis contextuais (💰 receita, 💸 despesa, 💳 cartão, ✅ confirmação, ❓ clarificação).
- **RF-23:** Formatação WhatsApp: negrito só com `*asterisco simples*` (proibido `**duplo**`); valores em reais com duas casas (`R$ 1.234,50`); datas ao usuário em `DD/MM/YYYY`. Formatação de centavos→reais e de datas DEVE usar funções puras reutilizáveis para consistência.

## Experiência do Usuário

Jornada canônica (despesa via Pix):

> **Usuário:** Gastei R$ 150,00 no supermercado no pix
> **Agente:** 💸 Entendi: *Supermercado* — *R$ 150,00* — *Pix* — *hoje (07/07/2026)*. Categoria sugerida: *Custo Fixo > Supermercado*. Confirma?
> **Usuário:** sim
> **Agente:** ✅ Registrado! *Supermercado* — *R$ 150,00* no Pix.

Clarificação de categoria (R7):

> **Usuário:** Gastei 35 reais com algo pro trabalho
> **Agente:** ❓ Não consegui definir a categoria com segurança. Você quis dizer: 1. *Conhecimento > Cursos e Treinamentos*; 2. *Custo Fixo > Assinaturas Essenciais*; 3. *Prazeres > Compras Pessoais*. Qual se encaixa melhor?

Fronteira de múltiplas transações (R8, RF-16):

> **Usuário:** Hoje gastei 30 reais no ônibus e 15 no café
> **Agente:** Percebi mais de um lançamento na mesma mensagem. Por segurança, registro um de cada vez — me manda o primeiro (ex.: "gastei 30 no ônibus") que eu já cuido dele. 🙂

Os diálogos R1–R7 do documento de User Story permanecem como referência de tom e formato para o harness de aceite.

## Restrições Técnicas de Alto Nível

- **Substrato de plataforma:** reutilizar `internal/platform/{agent,tool,memory,workflow,scorer}`; proibido reimplementar primitivos de plataforma no domínio (regra de ouro da skill `mastra` e R-WF-KERNEL-001 / R-AGENT-WF-001).
- **Adaptadores finos:** tools são adapters finos (`tool.NewTool[I,O]`), sem regra de negócio, SQL direto ou branching de domínio (R-ADAPTER-001). Regra de negócio vive em `domain/services` e use cases dos módulos financeiros.
- **DMMF obrigatório:** smart constructors com `(T, error)`; state-as-type para estados fechados (`Direction`, `PaymentMethod`, `SearchOutcome`, `PendingStatus`, `AwaitingSlot`, `AwaitingKind`) — nunca `string` livre em assinatura pública; funções de decisão puras (`Decide*`) sem IO; erros de fluxo como tipos customizados resolvidos por `errors.As`; pipeline `parse → validate → decide → persist → publish`.
- **Idempotência de domínio:** chave `(wamid, itemSeq, operation)` sobre a tabela `mecontrola.agents_write_ledger` (unique constraint já existente); nenhuma nova migração de schema é necessária para a idempotência.
- **Fuso horário:** toda resolução de data em `America/Sao_Paulo`.
- **Zero comentários** em `.go` de produção (R-ADAPTER-001.1).
- **Fronteira entre camadas:** manter `infrastructure → application → domain` em todos os módulos.
- **Validação real-LLM obrigatória:** fixes/feature do agente exigem harness com `RUN_REAL_LLM=1` e credenciais do `.env` (OpenRouter); mocks não bastam como evidência.
- **Eventos:** `TransactionCreated` continua sendo publicado via outbox e consumido por `internal/budgets`; esta PRD não altera esse contrato.

## Fora de Escopo

- **Múltiplas transações numa única mensagem** (multi-item real com `itemSeq > 0`) — non-goal; PRD futura (RF-16).
- **Edição e exclusão de lançamentos** — cobertas por `edit_entry`/`delete_entry` e pela PRD `conversa-agentiva-fluida`.
- **Criação de cartões, orçamentos e categorias** — fora do fluxo de registro (redirecionar).
- **Alertas proativos e consultas financeiras** — cobertos pela PRD `consulta-conversacional-financeira`.
- **Alterar o invariante do VO `Money`** (adicionar teto no domínio) — explicitamente descartado; teto fica na camada do agente (RF-05).
- **Recriar o campo `occurredAt` ou o parser `hoje/ontem/anteontem`** — já existem; a entrega estende, não recria.

## Rastreabilidade (código real)

- `internal/agents/application/agents/mecontrola_agent.go` — instruções do agente (estender: cinco campos, mapeamento de pagamento, proibições de inferência, gatilhos de suspensão, parser de dias da semana).
- `internal/agents/application/tools/{register_expense,register_income,classify_category,list_categories,resolve_card,list_cards}.go` — schemas já contêm `occurredAt`.
- `internal/agents/application/usecases/{register_attempt,register_entry,pending_entry_continuer,idempotent_write}.go` — `resolveEntryDate` já assume "hoje"; `registerIncomePaymentMethod = "pix"` confirmado; `IdempotentWrite` a integrar.
- `internal/agents/application/workflows/pending_entry_workflow.go` (`executeWrite`, `callLedger`, `BuildPendingEntryWorkflow`) e `pending_entry_decisions.go` (parser relativo a estender).
- `internal/agents/infrastructure/persistence/write_ledger_repository.go` + tabela `mecontrola.agents_write_ledger` (unique `(wamid, item_seq, operation)`).
- `internal/agents/infrastructure/binding/{transactions_ledger_adapter,categories_reader_adapter,card_manager_adapter}.go`.
- `internal/transactions/domain/{entities/transaction.go (occurredAt), commands/create_transaction.go, valueobjects/{payment_method,money,direction}.go, services/{transaction_workflow,installment_splitter,billing_cycle_resolver,ref_month_resolver}.go}`.
- `internal/categories/{application/usecases/{search_dictionary,resolve_category_for_write}.go, domain/valueobjects/{search_outcome,kind,match_score}.go}`.
- `internal/card/application/usecases/{resolve_card_by_nickname,invoice_for}.go`.

## Critérios de Validação

- `go build ./internal/agents/... ./internal/transactions/... ./internal/categories/... ./internal/card/...`
- `go vet ./internal/agents/... ./internal/transactions/... ./internal/categories/... ./internal/card/...`
- `go test -race -count=1 ./internal/agents/application/{tools,usecases,workflows}/...`
- `go test -race -count=1 ./internal/transactions/domain/services/... ./internal/categories/domain/...`
- `golangci-lint run ./internal/agents/... ./internal/transactions/... ./internal/categories/... ./internal/card/...`
- Harness real-LLM (`RUN_REAL_LLM=1`) cobrindo R1–R7, datas relativas incl. dias da semana, rejeição de "semana/mês passado", ambiguidade de categoria e de cartão, idempotência por `wamid` (replay), confirmação e cancelamento, valores inválidos (≤ 0 e acima do teto), e fronteira multi-item (RF-16). Meta M-04 ≥ 0,90; M-03/M-05 = 0 violações.

## Decisões Travadas

- **D-01 (RF-05):** Teto de valor via sanity-check na camada do agente (default R$ 10.000.000,00), sem alterar o VO `Money`.
- **D-02 (RF-07, RF-08):** Dias da semana como função pura; rejeitar "semana passada"/"mês passado".
- **D-03 / D-05 (RF-16):** MVP = 1 transação por mensagem; múltiplos numa mensagem → pedir um por vez sem persistir; multi-item é non-goal.
- **D-04 (RF-20):** Idempotência ancorada no `wamid` da mensagem original do lançamento.
- **D-06 (RF-09):** Data sempre explícita no resumo único de confirmação; sem passo extra de confirmação de data.

## Suposições e Questões em Aberto

- **Suposição:** o teto de valor default (R$ 10.000.000,00) é um parâmetro configurável; o valor exato pode ser ajustado na techspec sem alterar RFs.
- **Suposição:** a semântica de "sim/não" e o TTL do estado `pending-entry` seguem o contrato já travado na PRD `conversa-agentiva-fluida` (gate universal, TTL 30min); esta PRD não os redefine.
- **Sem questões em aberto bloqueantes.** Todas as ambiguidades materiais de escopo, data, idempotência e valor foram resolvidas nas decisões D-01..D-06.
