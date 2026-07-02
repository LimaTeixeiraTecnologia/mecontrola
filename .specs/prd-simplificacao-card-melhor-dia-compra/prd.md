# Documento de Requisitos do Produto (PRD)

<!-- spec-version: 2 -->

> Origem: `docs/prompts/2026-07-01-simplificacao-card-crud-melhor-dia-compra.md` (prompt enriquecido).
> Prompt original do usuário e levantamento de código preservados nesse arquivo de origem.
> Decisões de produto abaixo foram resolvidas por clarificação direta com o usuário em 2026-07-01
> (não são suposições). Fatos de código verificados por leitura direta do repositório (file:line).

## Visão Geral

O módulo `internal/card` hoje exige do usuário quatro dados de ciclo/limite ao cadastrar um cartão:
nome, apelido, dia de fechamento (`closing_day`), dia de vencimento (`due_day`) e limite
(`limit_cents`). Na prática, o dia de fechamento e o "melhor dia de compra" são deriváveis do
**banco emissor** + **dia de vencimento**, e o limite não pertence conceitualmente à identificação
do cartão (é consumido apenas por alertas de orçamento em `internal/budgets`).

Esta iniciativa **simplifica o CRUD de cartão** para exigir somente **nome/apelido do cartão**,
**banco emissor** e **dia de vencimento da fatura**, e introduz um **cálculo determinístico do
melhor dia de compra** a partir de uma **tabela de bancos administrável** (dias-antes-do-vencimento,
com regra padrão de 7 dias para banco não catalogado). O dia de fechamento passa a ser **derivado e
persistido como cache**, preservando o contrato consumido por `internal/transactions` sem alterá-lo.

Valor: menos fricção no cadastro (WhatsApp e API), remoção de campos que não pertencem ao domínio do
cartão, e um cálculo de fatura correto por padrão para os principais bancos brasileiros.

## Objetivos

- Reduzir a entrada de cadastro de cartão de 5 dados para 3 (nome/apelido, banco, dia de vencimento).
- Tornar `closing_day` e o melhor dia de compra 100% derivados de `banco + due_day`, sem entrada manual.
- Entregar o exemplo funcional do usuário como critério testável: **Nubank, vencimento 20 → fechamento 13 → melhor dia 14**.
- Garantir regra padrão de **7 dias** para qualquer banco fora da tabela.
- Permitir atualização da tabela de bancos **sem deploy de código** (tabela persistida administrável).
- Preservar o contrato de ciclo de fatura consumido por `internal/transactions` (zero regressão em `InvoiceFor` e no cálculo de parcelas).
- Corrigir o drift pré-existente de onboarding (`ClosingDay = DueDay`) em `internal/agents`.
- Não deixar nenhum campo/rota/DTO morto: tudo fora do novo escopo é **removido por completo** (regra HARD do prompt de origem).

### Métricas de sucesso

- Cadastro de cartão via API e via onboarding WhatsApp exige exatamente 3 dados de negócio (nome/apelido, banco, dia de vencimento). Verificável no OpenAPI e no schema de extração do onboarding.
- 100% dos 8 bancos da tabela inicial produzem fechamento/melhor-dia corretos; banco desconhecido cai em 7 dias. Verificável por testes unitários do domain service puro.
- Zero referência a `limit_cents`/`closing_day` como **entrada** no contrato público do cartão (OpenAPI) após a entrega.
- `internal/transactions` continua compilando e passando nos testes sem alteração de contrato de leitura do cartão.

## Histórias de Usuário

- Como usuário que cadastra um cartão pela API, quero informar apenas nome/apelido, banco e dia de
  vencimento, para que o sistema calcule automaticamente o fechamento e o melhor dia de compra.
- Como usuário no onboarding via WhatsApp, quero informar o banco do cartão além do apelido e dia de
  vencimento, para que o cartão seja cadastrado com o ciclo correto (sem o bug atual de fechamento = vencimento).
- Como usuário, quero perguntar "qual o melhor dia para comprar no cartão do Nubank que vence dia 20?"
  e receber "dia 14", para planejar minhas compras antes do fechamento da fatura.
- Como operador do produto, quero atualizar a tabela banco→dias-antes-do-vencimento sem novo deploy,
  para reagir quando um banco mudar sua regra de fechamento.
- Como mantenedor do módulo `internal/budgets`, preciso que a responsabilidade de "limite do cartão"
  seja tratada explicitamente, já que ela deixa de existir em `internal/card`.

## Funcionalidades Core

1. **CRUD de cartão simplificado** — cadastro/edição/leitura expõem somente identificação
   (nome/apelido), banco emissor e dia de vencimento. `closing_day` é derivado e persistido; o
   melhor dia de compra é derivado e retornado (não é entrada e não precisa ser coluna de entrada).

2. **Tabela de bancos administrável** — tabela persistida `banco → dias_antes_do_vencimento`,
   consultável em runtime, atualizável sem deploy, com seed inicial dos 8 bancos e fallback de 7 dias.

3. **Cálculo "Melhor Dia de Compra"** — função de domínio pura (padrão `Decide*`/domain service,
   sem IO, sem `context.Context`, determinística) que recebe `dias_antes` + `due_day` e retorna
   `closing_day` e `best_purchase_day`, com tratamento de virada de mês/dias inválidos reutilizando a
   lógica de `clamp`/`advanceMonth` já existente em `internal/card/domain/services/billing_cycle.go`.

4. **Exposição do melhor dia** — consulta pura (não exige cartão persistido), exposta como endpoint
   HTTP próprio **e** retornada automaticamente na resposta de criação/edição do cartão.

5. **Correção do onboarding de cartão** — `internal/agents` passa a perguntar o banco emissor e a
   enviar `DueDay` corretamente (fim do drift `ClosingDay = DueDay`).

## Requisitos Funcionais

### Simplificação do cadastro de cartão

- RF-01: O cadastro de cartão (`CreateCard`) DEVE exigir somente três dados de negócio: identificação
  do cartão (nome/apelido), banco emissor e dia de vencimento da fatura (`due_day`, 1..31).
- RF-02: A identificação do cartão DEVE ser consolidada em **um único campo** (apelido/nome do cartão).
  Os value objects/colunas hoje separados (`Name` e `Nickname`) DEVEM ser reduzidos a um só; o campo
  remanescente preserva a unicidade por usuário hoje aplicada ao apelido.
- RF-03: O campo **banco emissor** DEVE ser aceito como **texto livre** normalizado para lookup na
  tabela de bancos; banco não catalogado NÃO é erro de validação — recai no fallback de 7 dias (RF-09).
- RF-04: `closing_day` DEIXA de ser entrada do cliente em qualquer operação de cadastro/edição. Ele
  DEVE ser **derivado** de `banco + due_day` (RF-08) e **persistido** na coluna `closing_day` como
  cache, para preservar o contrato de leitura consumido por `internal/transactions` (RF-14).
- RF-05: `limit_cents` e todo o caso de uso/rota `UpdateCardLimit` DEVEM ser **removidos por completo**
  de `internal/card`: coluna `limit_cents`, campo na entidade, VO `CardLimit`, DTOs, usecase
  `UpdateCardLimit`, handler e rota `PATCH /cards/{id}/limit`. (Ver RF-15 para a remoção da feature
  dependente em `internal/budgets` e RF-19 para a notificação de fatura.)
- RF-06: A regra atual `dueDay = closingDay + 7` do `CreateCard` DEVE ser removida (o vínculo passa a
  ser o inverso: `closing_day` deriva de `due_day` + dias do banco).
- RF-07: A edição de cartão (`UpdateCard`) DEVE permitir alterar identificação, banco e/ou dia de
  vencimento; ao alterar banco ou `due_day`, o `closing_day` derivado DEVE ser recalculado e
  re-persistido, mantendo o controle de versão otimista já existente.

### Cálculo "Melhor Dia de Compra" e tabela de bancos

- RF-08: DEVE existir um domain service **puro** (sem IO, sem `context.Context`, determinístico) que,
  dados `dias_antes_do_vencimento` e `due_day`, calcule:
  `fechamento = due_day - dias_antes` e `melhor_dia = fechamento + 1`.
- RF-09: A quantidade `dias_antes_do_vencimento` DEVE vir de uma **tabela persistida** banco→dias.
  Se o banco informado não constar na tabela, o cálculo DEVE usar **7 dias** (regra padrão obrigatória).
- RF-10: A tabela de bancos DEVE ser **administrável sem deploy** (persistida em banco de dados) e DEVE
  ser criada com o seed inicial: Nubank 7, Itaú 8, Santander 8, Bradesco 7, Banco do Brasil 7,
  Caixa 7, Inter 7, C6 Bank 7.
- RF-11: O cálculo DEVE tratar **virada de mês e dias inválidos** (resultado fora de 1..31 ou mês com
  menos dias), reutilizando/adaptando a lógica de `clamp`/`advanceMonth` de
  `internal/card/domain/services/billing_cycle.go` (linhas 62-80). O resultado sempre é um dia válido 1..31.
- RF-12: O caso de uso "melhor dia de compra" DEVE ser uma **consulta pura**: recebe banco + dia de
  vencimento e retorna `closing_day` e `best_purchase_day` **sem exigir cartão persistido**.
- RF-13: O melhor dia de compra DEVE ser exposto de duas formas: (a) endpoint HTTP dedicado
  **`GET /cards/best-purchase-day`**, consulta stateless com query params `bank` e `due_day`,
  retornando `closing_day` e `best_purchase_day` (não exige cartão persistido); e (b) campos derivados
  (`closing_day`, `best_purchase_day`) retornados automaticamente na resposta de
  `CreateCard`/`UpdateCard`/leitura de cartão. O exemplo **Nubank/venc.20 → fechamento 13 → melhor
  dia 14** DEVE ser um critério de aceitação testável.

### Dependências cross-module (declaradas)

- RF-14: `internal/transactions` NÃO DEVE ser alterado no seu contrato de leitura do cartão. Como
  `closing_day` e `due_day` permanecem persistidos e legíveis via o card lookup
  (`internal/transactions/infrastructure/http/client/card_lookup_adapter.go:56`,
  `billing_cycle_resolver.go:17-18`), o cálculo de ciclo de fatura e de parcelas permanece intacto.
- RF-15: A feature de **alerta de threshold baseada em limite de cartão** em `internal/budgets` DEVE
  ser **removida por completo** desta entrega (o produto deixa de alertar por limite de cartão). A
  remoção é **cirúrgica** e NÃO PODE amputar os demais alertas de threshold do módulo. O usecase
  `evaluate_threshold_alerts` (`internal/budgets/application/usecases/evaluate_threshold_alerts.go`)
  avalia três tipos: `ThresholdAlertCategory` (orçamento por categoria) e `ThresholdAlertGoal` (metas)
  — que **DEVEM permanecer intactos** — e `ThresholdAlertCardLimit` — que **DEVE ser removido**.
  Itens a remover: `CardThresholdReader` (interface + adapter Postgres
  `card_threshold_reader.go` + mock), o método `ListActiveCardsForThresholdScan` no factory,
  `interfaces.ActiveCardForScan`, o ramo `buildCardSnapshots`/`activeCards` do usecase
  (`evaluate_threshold_alerts.go:91-121,132-149`), a constante fechada `ThresholdAlertCardLimit` do
  tipo `services.ThresholdAlertKind`, e testes associados. Após a remoção, `cards.limit_cents` não é
  lido por nenhum módulo (satisfaz RF-05).
- RF-16: `internal/agents` (onboarding WhatsApp) DEVE passar a **perguntar o banco emissor** do cartão
  e enviar corretamente o dia de vencimento:
  (a) o schema de extração/prompt do onboarding
  (`internal/agents/application/workflows/onboarding_workflow.go`) coleta `banco` além de apelido e
  `due_day`;
  (b) o adapter `card_manager_adapter.go:46-52` DEVE **corrigir o drift** enviando `DueDay: in.DueDay`
  (em vez de `ClosingDay: in.DueDay`) e passar o `Bank`;
  (c) os tipos de agents que expõem `ClosingDay`/`LimitCents`
  (`internal/agents/application/interfaces/types.go:137-143`, `card_manager_adapter.go:88-94`) DEVEM
  ser ajustados: `LimitCents` é **removido** (não existe mais no cartão); `ClosingDay` passa a
  refletir o valor derivado retornado pelo cartão; `Bank` é adicionado;
  (d) quando o usuário informar banco fora da tabela, o onboarding DEVE **seguir o cadastro aplicando
  o fallback de 7 dias sem atrito** (banco desconhecido nunca interrompe o fluxo nem pede confirmação).

- RF-19: A notificação de fatura a vencer e o evento `card.invoice_due.v1` DEVEM **deixar de exibir/
  transportar `limit_cents`**. O payload do evento
  (`internal/card/infrastructure/messaging/database/producers/invoice_due_publisher.go:22-29`), o
  consumer (`consumers/invoice_due_notifier.go`), o input `NotifyInvoiceDueInput` e o texto renderizado
  por `NotifyInvoiceDue` (`internal/card/application/usecases/notify_invoice_due.go`) DEVEM remover o
  campo/menção de limite. O alerta continua com identificação do cartão + data de vencimento + dias
  restantes. Como não há cartões em produção, não há evento legado em trânsito a versionar.

- RF-20: O campo `bank` DEVE ser normalizado de forma determinística (ex.: trim + lowercase/sem
  acentos) para o lookup na tabela de bancos, preservando o texto original informado para exibição. A
  regra de normalização é única e compartilhada entre cadastro, endpoint de consulta e onboarding, de
  modo que "Nubank", "nubank" e "NuBank" resolvam para a mesma linha da tabela.

### Migração de esquema e contrato

- RF-17: A tabela `mecontrola.cards` DEVE ganhar a coluna `bank` (texto, NOT NULL) e **perder** a
  coluna `limit_cents`. A coluna `closing_day` permanece (agora populada por derivação, não por
  entrada). Como **não há cartões em produção** (confirmado pelo usuário), NÃO é necessário backfill
  de dados; a migration pode alterar o esquema livremente, incluindo constraints.
- RF-18: O contrato público OpenAPI (`internal/card/openapi.yaml`) DEVE ser atualizado como mudança de
  API: remover `limit_cents` e `closing_day` como campos de **entrada** de `CreateCardRequest`/
  `UpdateCardRequest`, remover `UpdateCardLimitRequest` e a rota de limite, adicionar `bank` como
  entrada, e adicionar `closing_day`/`best_purchase_day` como campos **derivados** de resposta.

## Restrições Técnicas de Alto Nível

- Toda implementação Go futura é regida por `.agents/skills/go-implementation/SKILL.md` (Etapas 1-5,
  R0-R7) e pelas regras HARD do repositório (`.claude/rules/*`, `AGENTS.md`).
- O cálculo do melhor dia de compra DEVE ser função pura (padrão `Decide*`/domain service): sem IO,
  sem `context.Context`, determinística, recebendo parâmetros explícitos — nunca lógica de negócio em
  handler, consumer ou repository.
- **Zero comentários** em código Go de produção (regra HARD inegociável). Proibido `panic`. Proibido
  `_ = variavel` para silenciar não-uso.
- Value objects existentes (`Nickname`, `BillingCycle` no que resta) devem ser reaproveitados/adaptados,
  não recriados do zero. Validação de invariante permanece em smart constructors (R-TXN-002 / R-DTO).
- Input DTOs seguem `R-DTO-VALIDATE-001`: `Validate()` com `errors.Join`, campo nomeado no erro.
- Adapters permanecem finos (`R-ADAPTER-001`): a tool/binding de onboarding e os handlers HTTP apenas
  mapeiam para o usecase; o cálculo do fechamento/melhor-dia vive no domínio do card.
- Persistência da tabela de bancos e leitura em runtime dentro de `internal/card` (SQL apenas em
  repositório de infraestrutura). O domain service recebe o número de dias já resolvido — não faz IO.
- Mudança de contrato público (`openapi.yaml`) tratada e documentada como mudança de API na Especificação Técnica.

## Fora de Escopo

- Recalcular/backfill de dados de cartões existentes: **não há cartões em produção**, logo nenhuma
  migração de dados é necessária (apenas migração de esquema).
- Qualquer alteração em `internal/budgets` além da remoção cirúrgica da feature de alerta de threshold
  por limite de cartão (RF-15). Os alertas de categoria e de metas permanecem intactos; nenhuma feature
  nova de orçamento (inclusive uma eventual reintrodução de "limite" com origem própria) faz parte
  desta entrega.
- Alteração do contrato de leitura do cartão consumido por `internal/transactions` (RF-14 preserva-o).
- CRUD administrativo rico da tabela de bancos (UI/administração): esta entrega cobre a tabela
  persistida + seed + leitura; ferramentas de administração são consideração futura.
- Enum fechado de bancos suportados: rejeitado em favor de texto livre + fallback (RF-03/RF-09).
- Manutenção de qualquer campo/rota/DTO fora do novo escopo "por retrocompatibilidade" — proibido pela
  regra HARD do prompt de origem (remoção total, sem campo morto/depreciado/comentado).

## Suposições e Questões em Aberto

**Nenhuma questão em aberto.** Todas as sete perguntas obrigatórias e as cinco ambiguidades do prompt
de origem, mais os quatro pontos de borda cross-module, foram **resolvidos** por clarificação direta
com o usuário em 2026-07-01:

1. `LimitCents`/`UpdateCardLimit`: **removidos por completo** de `internal/card` (RF-05).
2. Feature de alerta por limite de cartão em `internal/budgets`: **removida cirurgicamente**,
   preservando alertas de categoria e metas (RF-15).
3. `ClosingDay`: **derivado e cacheado** na coluna `closing_day` (RF-04).
4. Melhor dia de compra: **consulta pura**, exposta como `GET /cards/best-purchase-day` **e** retornada
   no cadastro/edição/leitura (RF-12, RF-13).
5. Tabela de bancos: **tabela persistida administrável** com seed inicial dos 8 bancos (RF-10).
6. Impacto em `internal/transactions`: **preservado sem alteração de contrato** (RF-14).
7. Drift de onboarding em `internal/agents`: **corrigido**; onboarding passa a perguntar o banco;
   banco desconhecido segue com fallback de 7 dias sem atrito (RF-16).
8. Migração de dados: **desnecessária** — não há cartões em produção; apenas migração de esquema (RF-17).
9. Nome vs apelido: **consolidados em um único campo** (RF-02).
10. Banco emissor: **texto livre + fallback 7 dias**, com normalização determinística para lookup
    (RF-03, RF-09, RF-20).
11. Notificação de fatura / evento `card.invoice_due.v1`: **limite removido** do texto e do payload (RF-19).
12. Endpoint do melhor dia: **`GET`** com query params `bank`/`due_day` (RF-13).

Nenhuma suposição remanescente bloqueia o fluxo. A única decisão deixada para a Especificação Técnica é
**puramente de desenho** (não de produto): a estratégia de modelagem da tabela de bancos (colunas,
chave de normalização, mecanismo de seed via migration) e o formato exato dos campos derivados no
OpenAPI — ambos já delimitados pelos RFs acima.
