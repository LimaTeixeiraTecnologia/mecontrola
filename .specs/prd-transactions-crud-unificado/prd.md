# Documento de Requisitos do Produto (PRD)

<!-- spec-version: 2 -->

# CRUD Unificado de Transações (`internal/transactions`)

## Visão Geral

O módulo `internal/transactions` já entrega um CRUD de transações avulsas e, em
paralelo, mantém uma superfície separada de `card-purchase` para compras no cartão
de crédito (com parcelamento e vínculo de fatura). Essa dualidade cria ambiguidade:
uma compra no crédito pode ser registrada por dois caminhos diferentes — como
transação avulsa com `payment_method=credit_card` (sem parcela, sem fatura) ou como
`card-purchase` (com parcela e fatura). O resultado é atrito de produto,
inconsistência de dados e duplicação de regra.

Esta funcionalidade **unifica tudo em um único CRUD de transações**. Passa a existir
uma só porta de entrada para criar, editar, consultar, listar e remover transações,
para receita e despesa, com meios de pagamento aderentes ao Brasil. Quando o meio de
pagamento for `credit_card`, o próprio fluxo de transação dispara internamente a
lógica exclusiva de cartão: resolução da fatura de competência e parcelamento. Para
todos os demais meios, a transação é um lançamento simples e imediato.

O público é o usuário final de finanças pessoais do `mecontrola`, que registra seus
gastos e recebimentos categorizados por uma hierarquia raiz → subcategoria já
existente no catálogo do produto.

> **Nota de fidelidade ao codebase.** Duas premissas do pedido original não
> sobreviveram ao confronto com o código e ficam registradas aqui para evitar
> retrabalho:
> 1. A referência "uma tabela igual `bank_code.go`" para meios de pagamento é
>    inválida: `bank_code.go` **não** é tabela nem enum — é um Value Object de texto
>    livre normalizado. Meios de pagamento permanecem um **enum fechado**
>    (state-as-type), sem regressão para texto livre.
> 2. `credit_card` hoje é ambíguo (existe como método de transação avulsa **e** como
>    superfície `card-purchase` separada). Essa ambiguidade é exatamente o que a
>    unificação elimina.

## Objetivos

- **Porta única de escrita.** Toda transação (receita ou despesa, qualquer meio de
  pagamento) é criada, editada, consultada, listada e removida por um único CRUD de
  transações. Sucesso = zero caminhos concorrentes para registrar uma compra no
  crédito.
- **Regra de cartão embutida e transparente.** Lançamentos `credit_card` resolvem a
  fatura de competência e o parcelamento sem exigir do usuário um fluxo separado.
- **Catálogo de meios de pagamento fechado e aderente ao Brasil**, sem catálogo
  exaustivo nem texto livre.
- **Integridade de categorização.** Toda despesa carrega raiz + subcategoria válida
  da hierarquia real; receita carrega ao menos a raiz.
- **Métricas de sucesso mensuráveis:**
  - 100% das compras no crédito passam a ser registradas pelo CRUD unificado (0%
    pela superfície `card-purchase`, que é removida — ver RF-24).
  - 0 lançamentos de despesa persistidos sem subcategoria válida.
  - 0 lançamentos com meio de pagamento fora do catálogo fechado.
  - Distribuição de parcelas com soma exatamente igual ao total (0 divergência de
    centavos), auditável por transação.

## Histórias de Usuário

- Como usuário, quero registrar uma despesa à vista (pix, débito, dinheiro, boleto,
  vale) informando raiz + subcategoria, para acompanhar meu gasto categorizado.
- Como usuário, quero registrar uma compra no cartão de crédito parcelada em N vezes,
  para que o sistema distribua as parcelas pelas faturas corretas automaticamente.
- Como usuário, quero registrar uma compra no crédito à vista (1 parcela), para
  vinculá-la à fatura sem precisar declarar parcelamento.
- Como usuário, quero registrar uma receita (salário, freelance, cashback) na sua
  categoria, para acompanhar minha entrada de recursos.
- Como usuário, quero editar uma transação existente (inclusive uma compra parcelada),
  para corrigir valor, categoria ou parcelas.
- Como usuário, quero remover uma transação, para desfazer um lançamento equivocado.
- Como usuário, quero listar e buscar minhas transações, para revisar meu histórico.
- Caso de borda: registro minha primeira compra no crédito do mês, quando ainda não
  existe fatura aberta — o sistema deve **derivar e abrir a fatura de competência
  automaticamente**, sem me bloquear.

## Funcionalidades Core

1. **CRUD unificado de transações** — Create, Update (com controle de `version` /
   soft-delete), Delete, Get, List e Search, para `direction ∈ {income, outcome}`.
   É a única porta de escrita.
2. **Meio de pagamento como enum fechado** — validação na fronteira; `doc` bloqueado
   para criação (legado somente-leitura); `vale_refeicao` e `vale_alimentacao` como
   novos meios.
3. **Regra exclusiva de `credit_card`** — ao criar/editar transação com
   `payment_method=credit_card`: consulta do cartão (dia de fechamento/vencimento),
   resolução/abertura da fatura de competência e parcelamento em 1..24 com
   distribuição temporal e arredondamento determinístico. Nenhum outro meio de
   pagamento aciona fatura ou parcela.
4. **Categorização hierárquica** — categoria raiz (linha sem `parent_id`) +
   subcategoria (filha direta validada); subcategoria obrigatória em despesa.
5. **Esclarecimento de integração** — como recorrência e visão mensal já existentes
   se relacionam com o CRUD (recorrência materializa transações; visão mensal
   recomputa a partir dos eventos de transação), sem reimplementá-las.

## Requisitos Funcionais

### CRUD e escopo geral

- RF-01: O sistema DEVE oferecer um único CRUD de transações para criar, editar,
  consultar, listar, buscar e remover transações, cobrindo `direction=income` e
  `direction=outcome`.
- RF-02: A remoção DEVE ser soft-delete com controle de concorrência otimista por
  `version`, preservando o comportamento atual do módulo.
- RF-03: Toda transação DEVE registrar, no mínimo: `direction`, `payment_method`,
  `amount_cents` (> 0), `description`, `category_id`, `occurred_at` e, quando
  aplicável, `subcategory_id`.
- RF-04: A criação, edição e remoção DEVEM ser idempotentes e auditáveis, preservando
  as garantias de idempotência e outbox já existentes no módulo.

### Meios de pagamento (catálogo fechado)

- RF-05: `payment_method` DEVE ser um enum fechado (state-as-type); valores fora do
  catálogo DEVEM ser rejeitados na fronteira de entrada. É proibido aceitar meio de
  pagamento como texto livre.
- RF-06: Os meios de pagamento SUPORTADOS para criação DEVEM ser exatamente:
  `pix`, `ted`, `debit_in_account`, `debit_card`, `cash`, `boleto`, `credit_card`,
  `vale_refeicao`, `vale_alimentacao`.
- RF-07: `doc` DEVE permanecer legado **somente-leitura**: legível em registros
  existentes, bloqueado na criação e na edição que o reintroduza.
- RF-08: `vale_refeicao` e `vale_alimentacao` DEVEM ser adicionados como novos meios
  de pagamento suportados, tratados como lançamento simples (sem fatura, sem parcela).
- RF-09: Os meios EXCLUÍDOS do escopo DEVEM ser explicitados com racional: `cheque`
  (uso residual no Brasil), TEF (não é meio de pagamento do usuário final),
  cartão pré-pago (baixa aderência) e instrumentos corporativos (fora de finanças
  pessoais). Reintrodução exige novo PRD.

### Regra genérica vs. regra exclusiva de cartão de crédito

- RF-10: Para todo `payment_method` diferente de `credit_card`, a transação DEVE ser
  um lançamento simples e imediato, **sem** fatura e **sem** parcelamento.
- RF-11: Somente `payment_method=credit_card` DEVE acionar a lógica de cartão:
  consulta do cartão, resolução da fatura de competência e parcelamento.
- RF-11a: `payment_method=credit_card` DEVE ser válido **apenas** para
  `direction=outcome` (despesa). Uma transação `income` com `credit_card` DEVE ser
  rejeitada na fronteira de entrada (fatura/parcela não têm semântica em receita).
- RF-11b: Toda transação `credit_card` DEVE referenciar um `card_id` do usuário; a
  ausência de `card_id` para `credit_card` DEVE ser rejeitada. Para os demais meios de
  pagamento, `card_id` não se aplica.
- RF-12: Ao criar/editar uma transação `credit_card`, o sistema DEVE **resolver a
  fatura de competência** a partir do dia de fechamento/vencimento do cartão e, se ela
  ainda não existir, **abri-la/derivá-la automaticamente**. O sistema NÃO DEVE
  bloquear um lançamento legítimo por inexistência prévia de fatura aberta. ("Consultar
  se tem fatura aberta" tem semântica de *resolver a fatura de competência*, não de
  recusar o lançamento.)
- RF-13: O parcelamento DEVE aceitar número de parcelas no intervalo fechado 1..24.
- RF-14: O parcelamento DEVE ser **opcional**: `installments=1` é válido e representa
  compra no crédito à vista, ainda vinculada à fatura. O default de parcelas é 1.
- RF-15: Quando parcelado (`installments > 1`), o sistema DEVE distribuir as parcelas
  pelos meses de fatura corretos e tratar a sobra de centavos de forma
  **determinística**, de modo que a soma das parcelas seja exatamente igual ao valor
  total. O contrato de entrada é: valor total + número de parcelas (o usuário não
  informa valor por parcela).
- RF-16: A edição de uma transação `credit_card` parcelada DEVE recompor as parcelas e
  reaplicar os vínculos/deltas em **todas as faturas afetadas** de forma **atômica**
  (numa única transação de banco), consistente com RF-12 a RF-15, mantendo a soma de
  cada fatura correta.
- RF-16a: A remoção de uma transação `credit_card` parcelada DEVE **reverter os deltas
  de todas as parcelas em todas as faturas afetadas**, de forma atômica, sem deixar
  fatura com saldo residual.

### Categorização (hierarquia raiz → subcategoria)

- RF-17: A categoria raiz DEVE ser uma categoria sem `parent_id` do catálogo existente.
  Para despesa, as raízes são `custo-fixo`, `conhecimento`, `prazeres`, `metas`,
  `liberdade-financeira`; para receita, as raízes de `income` já semeadas
  (ex.: `salario`, `renda-variavel`, `investimentos`, `aluguel-recebido`, ...).
- RF-18: A subcategoria, quando informada, DEVE ser uma **filha direta** da categoria
  raiz escolhida e DEVE ser validada contra o catálogo (relação pai-filho e `kind`
  coerentes). Escolha de categoria em nível arbitrário sem relação hierárquica é
  proibida.
- RF-19: Para `direction=outcome` (despesa), a subcategoria DEVE ser **obrigatória**.
- RF-20: Para `direction=income` (receita), a subcategoria DEVE ser **opcional**: a
  receita pode ser registrada apenas na raiz.
- RF-21: O `kind` da categoria (income/expense) DEVE ser coerente com o `direction` da
  transação.

### Integração com recorrência e visão mensal

- RF-22: O PRD DEVE explicitar que a materialização de recorrências gera transações
  através do CRUD unificado (mesma regra de meio de pagamento, cartão e
  categorização), sem que esta feature reimplemente recorrência.
- RF-23: O PRD DEVE explicitar que a visão mensal (resumo/entradas) é recomputada a
  partir dos eventos de transação já emitidos pelo módulo, permanecendo consistente
  após a unificação, sem que esta feature reimplemente a visão mensal.

### Descontinuação da superfície `card-purchase`

- RF-24: A superfície `card-purchase` (endpoints, handlers e rotas dedicadas) DEVE ser
  **removida** como parte desta feature; compras no crédito passam a ser criadas
  exclusivamente pelo CRUD unificado com `payment_method=credit_card`. Trata-se de
  **breaking change** (ver Restrições e Riscos).
- RF-24a: Os dados existentes de `card_purchases` (tabela, parcelas e vínculos
  correlatos) DEVEM ser **descartados** junto com a remoção da superfície. Esta decisão
  é aceitável porque a produção atual não possui dados reais de compras no crédito
  (ambiente com 1 usuário / ledger vazio); é **irreversível** e DEVE ser confirmada
  contra o estado de produção imediatamente antes do release. O mecanismo de fatura
  (`card_invoice` por mês) permanece e passa a ser alimentado apenas pelo CRUD
  unificado.

## Experiência do Usuário

- Fluxo principal (despesa à vista): usuário escolhe direção `outcome`, meio de
  pagamento (ex.: `pix`), raiz + subcategoria obrigatória, valor, descrição e data →
  transação criada.
- Fluxo de crédito parcelado: usuário escolhe `credit_card`, informa o cartão, valor
  total e número de parcelas (1..24) → sistema resolve/abre a fatura de competência e
  distribui as parcelas automaticamente.
- Fluxo de crédito à vista: mesmo fluxo com `installments=1` (ou omitido) → uma parcela
  vinculada à fatura.
- Fluxo de receita: usuário escolhe `income`, raiz de receita e (opcionalmente)
  subcategoria, valor e data.
- Caso de borda (primeira compra do mês no cartão): não há fatura aberta → sistema
  abre a fatura de competência automaticamente e conclui o lançamento sem erro.
- Mensagens de validação DEVEM nomear o campo em falha (ex.: `subcategory_id:
  obrigatório para despesa`, `payment_method: valor não suportado`).

## Restrições Técnicas de Alto Nível

- **Domain Modeling Made Functional (Scott Wlaschin) + `.claude/skills/go-implementation/`
  são obrigatórios.** Especificamente: `PaymentMethod` e `Direction` como tipos
  fechados (state-as-type); Value Objects com smart constructors; workflows de domínio
  (`Decide*`) puros e sem IO (R-TXN-001/002); validação de input DTO agregada por
  `errors.Join` na fronteira da aplicação (R-DTO-VALIDATE-001); zero comentários em Go
  de produção (R-ADAPTER-001.1); adaptadores finos `handler → usecase`.
- **Padrão de projeto para meios de pagamento (candidato a ADR, não desenho final).**
  Strategy clássico (uma classe-estratégia por meio de pagamento) **não se justifica**:
  dos meios suportados, apenas `credit_card` diverge de comportamento (fatura +
  parcela); os demais são lançamento simples idêntico. O caminho viável e idiomático é
  um despacho por tipo fechado (ex.: registry `map[PaymentMethod]→decisão` para funções
  `Decide*` puras) — o "espírito" do Strategy sem hierarquia de classes, coerente com
  DMMF/go-implementation. Referência conceitual apenas: refactoring.guru/design-patterns.
  A escolha final de desenho pertence à Especificação Técnica.
- **Reaproveitamento obrigatório.** A lógica de cartão (consulta via `CardLookup` →
  `CardBillingSnapshot`, `InstallmentCount` 1..24, `InstallmentSplitter`, upsert de
  fatura por mês) já existe e DEVE ser reaproveitada, não reescrita, ao ser embutida no
  fluxo de transação.
- **Idempotência e outbox** existentes no módulo DEVEM ser preservados para os eventos
  de transação.
- **Compliance/dados.** DOC descontinuado no SPB brasileiro justifica RF-07 (legado
  read-only). Nenhum dado sensível novo é introduzido.

## Fora de Escopo

- Reimplementar recorrência (recurring templates) ou visão mensal (monthly views /
  reconciler) — apenas esclarecer a integração (RF-22, RF-23).
- Catálogo exaustivo de instrumentos de pagamento do Brasil; meios corporativos,
  legados ou de baixa aderência (RF-09).
- Reabertura de `doc` para criação (RF-07).
- Bloqueio de lançamento por inexistência de fatura aberta (RF-12 estabelece o oposto).
- Alterações no catálogo de categorias (a hierarquia raiz/subcategoria é consumida como
  está; esta feature não semeia nem edita categorias).
- Desenho técnico detalhado (schema, endpoints finais, assinaturas) — pertence à
  Especificação Técnica.

## Decisões Fechadas

Todas as decisões materiais foram confrontadas com o codebase e resolvidas com
recomendação explícita. Não há questão de produto em aberto.

- **D-01 — Unificação:** um CRUD único de transações; `credit_card` aciona
  fatura+parcela internamente (RF-01, RF-11). Risco assumido: reprojeto de
  handlers/workflows/agregado — de responsabilidade da Especificação Técnica, não é
  ambiguidade de produto.
- **D-08/RF-24/RF-24a — `card-purchase` removida e dados descartados:** breaking change
  deliberado, viabilizado por produção sem dados reais (ledger vazio). Confirmação
  obrigatória contra produção antes do release (operacional, não decisão de produto).
- **D-13 — VR/VA:** dois meios distintos (`vale_refeicao`, `vale_alimentacao`) — RF-06,
  RF-08. **Fechado**, não é mais pendência.
- **Crédito x direção:** `credit_card` restrito a despesa (RF-11a); `card_id`
  obrigatório para crédito (RF-11b).
- **Editar/remover parcelado:** recomposição/reversão atômica em todas as faturas
  afetadas (RF-16, RF-16a).

## Notas para a Especificação Técnica (não são lacunas de produto)

Estes itens são **decisões de desenho/implementação** que pertencem à Especificação
Técnica e não abrem ambiguidade neste PRD:

- Estratégia de despacho por `PaymentMethod` (registry de funções `Decide*` puras vs.
  alternativa) — vira ADR (ver Restrições Técnicas).
- Script/rotina de remoção da tabela e rotas de `card_purchases` (RF-24a) e verificação
  do estado de produção imediatamente antes do corte.
- Pontos exatos de medição das métricas de sucesso, reusando idempotência, eventos de
  transação e reconciliação mensal já existentes no módulo.
- Derivação precisa do mês de competência da fatura a partir de `occurred_at` e do
  dia de fechamento/vencimento do cartão (reusa `CardBillingSnapshot`/`InstallmentSplitter`).
