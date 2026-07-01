# Prompt Mandatório — Simplificação do CRUD de `internal/card` e Novo Caso de Uso "Melhor Dia de Compra"

> Gerado via skill `prompt-enricher` em 2026-07-01. Este arquivo é o prompt enriquecido, **pronto para uso**, para ser consumido como input da skill `create-prd` (etapa seguinte do fluxo, fora deste arquivo). **Nenhuma implementação foi realizada na criação deste arquivo. Nenhum PRD foi criado nesta etapa.**

---

## 1. Contexto Obrigatório (ler antes de qualquer ação futura)

- `AGENTS.md` e `.agents/skills/agent-governance/SKILL.md` são a fonte canônica de governança deste repositório e DEVEM ser lidos antes de qualquer análise ou edição.
- Este prompt é o **input** da skill `create-prd`. A skill `create-prd` DEVE ser usada para transformar este documento em um PRD formal, com escopo incluído/excluído, restrições e requisitos funcionais numerados — **não implementar código a partir deste arquivo diretamente**.
- Após o PRD, o fluxo natural passa por `.agents/skills/create-technical-specification/SKILL.md` e `.agents/skills/create-tasks/SKILL.md` antes de qualquer `execute-task`. Nenhuma dessas etapas foi executada aqui.
- Módulo alvo: `internal/card` (bounded context em `internal/`, arquitetura `domain -> application -> infrastructure` conforme `AGENTS.md`).
- Toda implementação futura em Go sobre este módulo é regida por `.agents/skills/go-implementation/SKILL.md` (Etapas 1-5, regras R0-R7) e pelo padrão DMMF (`Decide*` puro, state-as-type, smart constructors) já em uso no módulo (`services.CreateCardDecider`, `services.UpdateCardDecider`, `valueobjects.NewBillingCycle`, `valueobjects.NewCardLimit`).
- Zero comentários em código Go de produção é regra `[HARD]` inegociável neste repositório.
- Proibido `panic` em qualquer alteração futura.
- Proibido `_ = variavel` para silenciar parâmetro/import não utilizado.

## 2. Estado Real do Código (levantado por leitura direta, não presumido)

### 2.1 Estrutura de dados atual (`internal/card`)

- Entidade `entities.Card` (`internal/card/domain/entities/card.go:13-24`): `ID, UserID, Name, Nickname, Cycle (valueobjects.BillingCycle), LimitCents int64, Version, CreatedAt, UpdatedAt, DeletedAt`.
- `valueobjects.BillingCycle` (`internal/card/domain/valueobjects/billing_cycle.go:7-20`): `ClosingDay int`, `DueDay int`, ambos validados entre 1 e 31 via `NewBillingCycle`.
- `valueobjects.CardLimit` (`internal/card/domain/valueobjects/card_limit.go:9-29`): limite do cartão em centavos, validado entre 0 e 100.000.000 centavos.
- `valueobjects.CardName` (64 runas máx.) e `valueobjects.Nickname` (32 runas máx.) já existem e devem ser reaproveitados/adaptados, não recriados do zero.
- Tabela `mecontrola.cards` (`migrations/000001_initial_schema.up.sql:536-556`): colunas `id, user_id, name, nickname, closing_day, due_day, limit_cents, version, created_at, updated_at, deleted_at`, com `CHECK` constraints para `closing_day`, `due_day`, `name`, `nickname` e `limit_cents`.

### 2.2 Casos de uso e fluxos atuais que DEVEM ser reavaliados

- `usecases.CreateCard` (`internal/card/application/usecases/create_card.go`): recebe `Name, Nickname, ClosingDay, DueDay *int (opcional), LimitCents`; se `DueDay` não informado, calcula `dueDay := closingDay + 7` (regra de negócio fixa e diferente da nova regra solicitada).
- `usecases.UpdateCard` (`internal/card/application/usecases/update_card.go`): atualiza `Name, Nickname, ClosingDay, DueDay` via `services.UpdateCardDecider`.
- `usecases.UpdateCardLimit` (`internal/card/application/usecases/update_card_limit.go`) + handler `infrastructure/http/server/handlers/update_card_limit.go`: caso de uso dedicado e rota HTTP exclusivos para atualizar `LimitCents` com controle de versão otimista (`expectedVersion`).
- `usecases.InvoiceFor` (`internal/card/application/usecases/invoice_for.go`) + `services.BillingCycleService.InvoiceFor` (`internal/card/domain/services/billing_cycle.go`): calcula `ClosingDate`/`DueDate` de uma fatura a partir de `cycle.ClosingDay`/`cycle.DueDay` e da data de compra — **este é o caso de uso mais próximo do "melhor dia de compra" pedido, mas hoje depende de `ClosingDay` já cadastrado, não de uma tabela de bancos**.
- `usecases.EvaluateInvoiceDueAlerts` / `NotifyInvoiceDue` / consumers/producers de `invoice_due` (`internal/card/application/usecases/evaluate_invoice_due_alerts.go`, `notify_invoice_due.go`, `internal/card/infrastructure/messaging/database/{consumers,producers}/invoice_due_*`): dependem de `DueDay`/`ClosingDay` para alertas de vencimento de fatura.
- Rotas HTTP e contrato (`internal/card/openapi.yaml:290-338`, `internal/card/infrastructure/http/server/router.go`, `handlers/create.go`, `handlers/update.go`, `handlers/update_card_limit.go`): expõem `closing_day`, `due_day`, `limit_cents` como campos do contrato público da API.

### 2.3 Acoplamento cross-module identificado (IMPACTO CRÍTICO — não pode ser ignorado no PRD)

Uma varredura no repositório confirma que **outros módulos dependem diretamente dos campos que esta simplificação pretende remover ou alterar**:

1. **`internal/transactions`** usa `ClosingDay`/`DueDay` do cartão para resolver o ciclo de fatura de uma compra:
   - `internal/transactions/domain/valueobjects/card_billing_snapshot.go:36-37` — `CardBillingSnapshot.ClosingDay()` e `.DueDay()`.
   - `internal/transactions/domain/services/billing_cycle_resolver.go:17-18` — usa ambos os valores para decidir a fatura de uma transação.
   - `internal/transactions/infrastructure/repositories/postgres/card_purchase_repository.go` e `internal/transactions/infrastructure/http/client/card_lookup_adapter.go` — leem esses campos do cartão via integração cross-module.
2. **`internal/budgets`** usa `LimitCents` do cartão para alertas de limite/threshold, **sem relação alguma com ciclo de fatura**:
   - `internal/budgets/application/usecases/evaluate_threshold_alerts.go:135,144` e `internal/budgets/infrastructure/repositories/postgres/card_threshold_reader.go:70`.
3. **`internal/agents`** (onboarding via WhatsApp) já coleta apenas `Nickname` + `DueDay` do usuário (sem perguntar `ClosingDay`):
   - `internal/agents/application/workflows/onboarding_workflow.go:115,156,178,431,445,451` — `DecideCardEntry(nickname, dueDay)`.
   - `internal/agents/infrastructure/binding/card_manager_adapter.go:50,91` — **hoje envia `ClosingDay: in.DueDay` (usa o dia de vencimento como se fosse o dia de fechamento)**, um comportamento pré-existente que resulta em `CreateCard` recalculando um `DueDay` diferente do informado pelo usuário (`closingDay + 7`). Este é um **drift de comportamento já existente**, não introduzido por este prompt, e deve ser registrado como tal no PRD (a nova regra de cálculo por banco pode ou não corrigir este drift — decisão de escopo do PRD).

**Implicação obrigatória para o PRD**: a simplificação do cadastro de cartão (nome/apelido + dia de vencimento) e a introdução do cálculo por tabela de bancos têm efeito cascata sobre `internal/transactions` (cálculo de ciclo de fatura) e potencialmente sobre `internal/budgets` (se `LimitCents` for removido do cartão) e `internal/agents` (se o adapter de onboarding precisar ser ajustado). O PRD DEVE tratar isso explicitamente como escopo incluído, escopo excluído, ou dependência declarada — não pode ser um gap silencioso.

## 3. Objetivo da Tarefa (o que o PRD futuro DEVE cobrir)

### 3.1 Simplificação do CRUD de cartão

Todo o CRUD de `internal/card` deve passar a expor e persistir **apenas**:
1. Nome/apelido do cartão (manter os value objects `CardName`/`Nickname` existentes, ou consolidar em um único campo — decisão a explicitar no PRD).
2. Banco emissor do cartão (campo novo, hoje inexistente na entidade/tabela `cards`).
3. Dia de vencimento da fatura (`DueDay`, já existente).

Campos a serem removidos ou reavaliados no PRD, com decisão explícita para cada um:
- `ClosingDay` (`closing_day`): passa a ser **calculado automaticamente** (não mais informado pelo cliente) via regra de banco, não mais um campo de entrada de cadastro. Decidir se continua sendo persistido (cache do cálculo) ou se é sempre derivado em tempo de leitura.
- `LimitCents` (`limit_cents`): o pedido do usuário diz "TODO CRUD tenha apenas: nome do cartão/apelido e o dia de pagamento da fatura" — isso sugere remoção do limite do cartão do cadastro. Isto tem impacto direto em `internal/budgets` (seção 2.3, item 2). O PRD DEVE decidir explicitamente: (a) remover `LimitCents` de `internal/card` e criar/realocar essa responsabilidade em outro lugar (ex.: `internal/budgets`), ou (b) manter `LimitCents` como campo opcional fora do escopo desta simplificação. **Não presumir a resposta — esta é uma decisão de produto que deve ser levantada como pergunta obrigatória durante o `create-prd`.**
- `UpdateCardLimit` (caso de uso + handler + rota HTTP dedicados): decisão de manter, remover ou substituir depende diretamente da decisão anterior sobre `LimitCents`.

### 3.2 Novo caso de uso: "Melhor Dia de Compra" (nome de caso de uso a definir no PRD, ex.: `BestPurchaseDay` ou `RecommendedPurchaseDay`)

Regra de negócio completa, fornecida pelo usuário e validada como requisito funcional obrigatório:

**Entrada do usuário (apenas):**
1. Banco emissor do cartão.
2. Dia de vencimento da fatura.

**Cálculo determinístico (função pura, sem IO — aderente ao padrão `Decide*`/domain service já usado no módulo, ex.: `services.BillingCycleService`):**
```
dias_antes_do_vencimento = tabela_bancos[banco] OU 7 (regra padrão se o banco não estiver na tabela)
fechamento = vencimento - dias_antes_do_vencimento
melhor_dia_de_compra = fechamento + 1
```

**Tabela de referência inicial (dias antes do vencimento em que a fatura fecha), fornecida pelo usuário — DEVE ser validada/ajustada durante o `create-prd` quanto à fonte oficial e à necessidade de atualização futura sem deploy de código:**

| Banco            | Dias antes do vencimento |
|------------------|--------------------------:|
| Nubank           | 7 |
| Itaú             | 8 |
| Santander        | 8 |
| Bradesco         | 7 |
| Banco do Brasil  | 7 |
| Caixa            | 7 |
| Inter            | 7 |
| C6 Bank          | 7 |

**Regra padrão obrigatória**: se o banco informado não constar na tabela, usar `dias_antes_do_vencimento = 7`.

**Exemplo de validação funcional (caso de teste mínimo obrigatório):**
- Banco: Nubank, Vencimento: dia 20 → dias antes: 7 → Fechamento: dia 13 → Melhor dia de compra: dia 14.

**Observação de borda a resolver no PRD**: a fórmula `fechamento = vencimento - dias_antes` e `melhor_dia = fechamento + 1` pode gerar dias fora do intervalo `1-31` ou cair em meses com menos dias (ex.: vencimento dia 3, 7 dias antes = dia -4, ou vencimento dia 1 com bancos de 8 dias). O PRD DEVE definir a regra de "virada de mês" (ex.: reaproveitar a lógica de `clamp`/`advanceMonth` já existente em `internal/card/domain/services/billing_cycle.go:36-80`, adaptada ao novo cálculo) — não deixar como lacuna implícita.

### 3.3 Relação com o caso de uso `InvoiceFor` existente

O caso de uso `usecases.InvoiceFor` (`internal/card/application/usecases/invoice_for.go`) e o `services.BillingCycleService` já calculam `ClosingDate`/`DueDate` de uma fatura a partir de uma compra, mas usam `ClosingDay` como entrada direta (hoje cadastrado pelo cliente). O PRD DEVE decidir explicitamente a relação entre:
(a) o novo caso de uso "melhor dia de compra" (baseado em banco + vencimento, sem depender de uma compra específica), e
(b) o caso de uso `InvoiceFor` existente (que calcula a fatura de uma compra específica, e hoje é consumido por `internal/transactions` conforme seção 2.3).
Uma hipótese razoável — a confirmar no PRD, não a assumir como decisão fechada — é que `ClosingDay` deixa de ser um dado de entrada do cliente e passa a ser **derivado** da tabela de bancos + `DueDay`, e `InvoiceFor`/`BillingCycleService` passam a consumir esse `ClosingDay` derivado internamente, preservando o contrato de `internal/transactions` sempre que possível.

## 4. Restrições Mandatórias (zero desvio, zero flexibilização)

- **[MANDATÓRIO E INEGOCIÁVEL] Exclusão de campos fora de escopo**: todo campo, coluna, DTO, caso de uso, rota HTTP, mock, teste ou trecho de código que não fizer mais sentido dentro do novo escopo simplificado (nome/apelido do cartão + banco emissor + dia de vencimento + cálculo automático) DEVE ser **excluído/removido por completo** — não pode ser mantido como campo morto, opcional "por segurança", depreciado sem remoção, comentado, ou preservado apenas por retrocompatibilidade não solicitada. Isso vale explicitamente para candidatos já identificados na seção 2 deste documento (ex.: `LimitCents`/`UpdateCardLimit` caso a decisão do PRD confirme que saem do domínio de `internal/card`; `ClosingDay` como campo de entrada do cliente, caso a decisão do PRD confirme que passa a ser somente calculado). Esta regra é **inegociável**: o PRD e as etapas futuras (`create-technical-specification`, `create-tasks`, `execute-task`) não podem justificar a permanência de campos fora de escopo por conveniência, medo de quebrar contrato, ou ausência de decisão explícita — a ausência de decisão explícita é, por si só, uma pergunta obrigatória a ser resolvida (seção 4, perguntas 1-7), nunca um motivo para manter o campo.
- **NÃO implementar nada nesta etapa.** Este arquivo é exclusivamente o input enriquecido para a skill `create-prd`.
- **NÃO criar o PRD neste arquivo** — o PRD deve ser gerado pela skill `create-prd` a partir deste prompt, seguindo seu próprio processo de perguntas/clarificação.
- O `create-prd` DEVE levantar como perguntas obrigatórias (não como suposições):
  1. O que acontece com `LimitCents`/`UpdateCardLimit` e com a dependência de `internal/budgets` sobre esse campo.
  2. Se `ClosingDay` deixa de existir como coluna persistida ou passa a ser derivado/cacheado.
  3. Se o novo caso de uso "melhor dia de compra" é uma consulta pura (sem persistência, sem exigir cartão já cadastrado) ou se deve ser sempre associado a um cartão existente.
  4. Onde a tabela de bancos deve residir (constante em código versionado vs. tabela de banco de dados administrável) — o próprio usuário sugeriu "permite atualizar a tabela caso algum banco altere sua regra", o que aponta para uma tabela persistida e não uma constante fixa em código; isso deve virar requisito funcional explícito, não ficar implícito.
  5. Impacto em `internal/transactions` (cálculo de ciclo de fatura de uma compra) e se essa simplificação exige coordenação/migração de dados também nesse módulo.
  6. Impacto no drift já existente em `internal/agents/infrastructure/binding/card_manager_adapter.go:50` (onboarding envia `DueDay` como `ClosingDay`) — decidir se este prompt/PRD corrige esse comportamento ou se é explicitamente fora de escopo.
  7. Estratégia de migração de dados para cartões já cadastrados (a coluna `closing_day` já existe com valores reais; a tabela `cards` precisa de migration para adicionar `banco emissor` e para remover/alterar `limit_cents`/`closing_day` como entrada).
- A implementação futura (fora deste prompt) DEVE seguir DMMF: cálculo de "melhor dia de compra" como função `Decide*`/domain service pura, sem IO, sem `context.Context`, determinística, recebendo `now`/parâmetros explícitos quando necessário — nunca lógica de negócio em handler, consumer ou repository.
- Toda mudança de contrato público (`internal/card/openapi.yaml`) deve ser tratada como mudança de API e versionada/documentada no PRD e na técnica specification subsequente.
- Zero comentários em código Go de produção continua válido para qualquer implementação futura.

## 5. Critérios de Aceitação do PRD Gerado (mensuráveis)

- [ ] PRD numera requisitos funcionais separados para: (a) simplificação do cadastro de cartão, (b) novo caso de uso de melhor dia de compra, (c) decisão explícita sobre `LimitCents`, (d) decisão explícita sobre persistência/administração da tabela de bancos.
- [ ] PRD contém escopo explicitamente excluído (ex.: se `internal/transactions`/`internal/budgets` serão ou não alterados nesta entrega).
- [ ] PRD reproduz o exemplo funcional do usuário (Nubank, vencimento 20 → fechamento 13 → melhor dia 14) como critério de aceitação testável.
- [ ] PRD trata explicitamente a regra padrão de 7 dias para bancos fora da tabela.
- [ ] PRD trata explicitamente o comportamento de virada de mês/borda de dias inválidos (1-31) no cálculo de fechamento/melhor dia.
- [ ] PRD identifica e declara todas as dependências cross-module listadas na seção 2.3 deste documento (não pode omitir nenhuma).
- [ ] PRD não presume respostas para as 7 perguntas obrigatórias da seção 4 — cada uma deve ser resolvida via clarificação com o usuário, conforme o processo padrão da skill `create-prd`.
- [ ] PRD declara explicitamente, para cada campo/caso de uso/rota candidato à remoção (`LimitCents`, `UpdateCardLimit`, `ClosingDay` como entrada), a decisão de exclusão total ou a justificativa concreta de permanência — nunca omissão ou "manter por enquanto".
- [ ] Nenhuma linha de código Go, SQL ou OpenAPI foi alterada nesta etapa (somente este documento markdown foi criado).

## 6. Formato de Saída Esperado do Próximo Passo (`create-prd`)

- Documento de PRD estruturado conforme o padrão já usado pela skill `create-prd` neste repositório: objetivo, escopo incluído, escopo excluído, restrições, requisitos funcionais numerados.
- Rastreabilidade explícita a este arquivo de prompt como origem (ex.: seção de contexto/histórico do PRD referenciando `docs/prompts/2026-07-01-simplificacao-card-crud-melhor-dia-compra.md`).
- Nenhuma implementação de código nesta etapa — o PRD é insumo para `create-technical-specification` e `create-tasks` em etapas futuras e distintas.

## 7. Ambiguidades Identificadas (a resolver obrigatoriamente durante o `create-prd`, não durante esta enriquecimento)

1. Se "nome do cartão/apelido" deve continuar sendo dois campos (`Name` + `Nickname`, como hoje) ou consolidar em um único campo, já que o pedido do usuário menciona "nome do cartão/apelido" como um único conceito.
2. Se o "banco emissor" deve ser um enum fechado (lista fixa de bancos suportados) ou texto livre com fallback para a regra padrão de 7 dias — o texto do usuário sugere texto livre com fallback, mas isso deve ser confirmado.
3. Se a tabela de bancos x dias deve ser uma tabela de banco de dados nova (com CRUD administrativo próprio) ou uma configuração versionada em código — impacta diretamente a arquitetura da solução técnica futura.
4. Se cartões já cadastrados (com `closing_day` e `limit_cents` preenchidos) precisam de uma migração de dados explícita (ex.: recalcular `closing_day` a partir do banco informado, ou manter o valor legado até nova atualização pelo usuário).
5. Se o novo caso de uso de "melhor dia de compra" deve ser exposto como endpoint HTTP novo, como parte do `CreateCard`/`UpdateCard` (retornado automaticamente no cadastro), ou ambos.

---

**Prompt original do usuário (preservado para rastreabilidade):**

> Eu quero que o módulo internal/card seja simplicidado, e TODO CRUD tenha apenas: nome do cartão/apelido e o dia de pagamento da fatura.
>
> Eu quero um caso de uso apenas para retornar em qual fatura deve ser preenchida a compra, por exemplo: cliente cadastrou cartão Nubank com vencimento dia 1, então o sistema deve consultar uma tabela no banco e fazer:
>
> 1. Cliente informa o banco emissor.
> 2. Cliente informa o dia de vencimento.
> 3. O sistema consulta uma tabela do banco.
> 4. Fechamento = vencimento - dias_do_banco.
> 5. Melhor dia = fechamento + 1.
>
> | Banco           | Dias antes do vencimento |
> | --------------- | -----------------------: |
> | Nubank          |                        7 |
> | Itaú            |                        8 |
> | Santander       |                        8 |
> | Bradesco        |                        7 |
> | Banco do Brasil |                        7 |
> | Caixa           |                        7 |
> | Inter           |                        7 |
> | C6 Bank         |                        7 |
>
> fechamento = vencimento - dias_antes
> melhor_dia = fechamento + 1
>
> Banco: Nubank, Vencimento: 20, Dias antes: 7, Fechamento: 13, Melhor dia: 14.
>
> Se o banco não estiver na tabela, use uma regra padrão: dias_antes = 7.
>
> Assim, o cadastro precisa apenas de: Banco emissor, Dia do vencimento. Todo o restante é calculado automaticamente. Essa abordagem é simples de implementar e permite atualizar a tabela caso algum banco altere sua regra.
