# ADR-006 — Adoção seletiva de Domain Modeling Made Functional (DMMF)

## Metadados

- **Título:** Adotar smart constructors, passo `Decide*` puro e domain events como tipos do `domain/` — sem `Result` monad nem function-as-DI
- **Data:** 2026-06-12
- **Status:** Aceita
- **Decisores:** Engenharia (autor: agente IA a partir de refinement solicitado pelo usuário)
- **Relacionados:** PRD `.specs/prd-transactions-monthly/prd.md`; techspec `.specs/prd-transactions-monthly/techspec.md`; ADR-003 (single card purchase event); skill `go-implementation` (R0–R7, R-ADAPTER-001.2).

## Contexto

Durante o refinement da techspec, surgiu a pergunta se faz sentido aplicar conceitos de _Domain Modeling Made Functional_ (Scott Wlaschin, 2018) à implementação de `internal/transactions`. Go cobre boa parte do DMMF nativamente (erros como valores, sem exceções, imutabilidade por convenção) e o codebase tem governance que torna alguns pilares do livro contraproducentes neste contexto:

- **R1** (skill `go-implementation`): toda função deve ser método de struct, exceto `main`/factories/helpers. Inviabiliza function-as-DI puro.
- **R6.3**: interfaces no consumidor + `mockery` para mocks. Inviabiliza substituir interfaces por function types.
- **R-ADAPTER-001.2**: handler/job/consumer/producer são portas finas com fluxo `adapter → usecase → service/repo/client`. Já está alinhado com "workflows orquestrados no use case", mas o uso de monads como `Result[T, E]` adicionaria fricção contra `golangci-lint`, `mockery` e os padrões do `internal/budgets`/`internal/identity`.

Por outro lado, três pilares do DMMF resolvem dores reais já visíveis no codebase:

1. **Validação espalhada** — hoje `int <= 0`, range de parcelas e range de `RefMonth` aparecem em múltiplos pontos (handler, usecase, schema). Smart constructors centralizam num único lugar.
2. **Use cases mistos** (IO + decisão) — dificultam teste sem mocks. Separar a decisão pura ajuda cobertura do core.
3. **Eventos de domínio nascendo como infrastructure** — hoje o `outbox.Event` é construído direto na camada `messaging/database/producers`, ignorando que "decidir que um evento ocorreu" é responsabilidade do domínio.

## Decisão

Adotar DMMF **seletivamente** em `internal/transactions`, codificando 5 práticas obrigatórias e 3 práticas proibidas.

### Práticas obrigatórias

1. **Smart constructors em value objects.** Toda primitiva de domínio com invariante (`Money`, `RefMonth`, `InstallmentCount`, `Description`, `DayOfMonth`) é tipo opaco com `New*(...) (T, error)`; o construtor é o único caminho para criar uma instância válida. Validação **fora** do construtor é proibida.
2. **Passo `Decide*` puro em `domain/services/<aggregate>_workflow.go`.** Workflows com regra de negócio (`CreateTransaction`, `UpdateTransaction`, `CreateCardPurchase`, `UpdateCardPurchase`, `MaterializeRecurringForDay`) ganham método `Decide<Operation>(...) Decision` que recebe apenas dados (commands, snapshots, IDs, `now`) e retorna `Decision` (agregado + entidades dependentes + domain event). Sem `ctx`, sem repo, sem SQL, sem `time.Now()` interno.
3. **Domain events como tipos do `domain/entities/events.go`.** `CardPurchaseCreated`, `CardPurchaseUpdated`, `TransactionCreated`, etc., vivem em `domain/`. Producers fazem **apenas** o mapeamento `domain event → outbox.Envelope`. O cálculo de `ref_months_affected`, `event_id` (recebido como parâmetro), `aggregate_*` e `occurred_at` vive no `Decide*`.
4. **Optionals explícitos.** Campos genuinamente opcionais (`SubcategoryID`, `EndedAt`) usam `internal/transactions/domain/option/Option[T]` (tipo genérico curto: `Some(v)` / `None()` / `Get() (T, bool)`). Ponteiro continua permitido em DTO de output JSON, mas o domínio fala em `Option[T]`.
5. **Acumulação de validação com `errors.Join`.** A função `validate*` de cada use case acumula erros de smart-constructors e retorna `errors.Join(...)` em vez de fail-fast. Cliente recebe a lista completa em um único 400.

### Práticas proibidas

1. **`Result[T, E]` monad / railway-oriented operators.** Go tem `(T, error)` + early-return idiomático; `Result.Bind/Map` quebra ferramental e estilo do repo.
2. **Function-as-DI** (`type CreateCardPurchase func(ctx, cmd) (out, error)`) — viola R1 e inviabiliza `mockery`.
3. **`Decide*` para CRUD trivial** (`Get*`, `List*`, `Delete*` sem regra além de soft-delete + publish). Indireção sem ganho.

### Escopo de aplicação

Aplicar `Decide*` puro **apenas** aos 5 workflows com regra de negócio:

| Workflow | Justificativa |
|---|---|
| `CreateTransaction` | Validação composta + cálculo de `ref_month` em fuso BR + decisão de evento |
| `UpdateTransaction` | Mudança de `ref_month` exige `ref_months_affected = old ∪ new` |
| `CreateCardPurchase` | Split de parcelas + `BillingCycleResolver` + `ref_months_affected` |
| `UpdateCardPurchase` | Cascata silenciosa em faturas fechadas + `ref_months_affected` (ADR-003 + ADR-005) |
| `MaterializeRecurringForDay` | Filtro por `day_of_month` + escolha credit_card vs default + idempotência |

Demais use cases (`Get*`, `List*`, `Delete*`, `RecomputeMonthlySummary`, `ReconcileMonthlySummary`) permanecem como orquestradores diretos sem `Decide*`.

## Alternativas Consideradas

### A. Adotar DMMF integralmente (incluindo `Result[T,E]`, function-as-DI, pipelines com `bind`)
- **Vantagens**: aderência total ao livro; pipeline mais legível em casos felizes.
- **Desvantagens**: quebra R1/R6.3, inviabiliza `mockery`, contradiz padrão dos módulos existentes (`internal/budgets`, `internal/identity`, `internal/billing`); revisão precisa aprender novo vocabulário; `golangci-lint` rejeita parte.
- **Motivo da rejeição**: custo ferramental e cognitivo alto; ganho marginal sobre `(T, error)` + early-return.

### B. Não adotar DMMF; manter padrão atual do repo (validação inline no usecase, eventos construídos no producer)
- **Vantagens**: aderência ao status quo; zero refactor preventivo.
- **Desvantagens**: validação repetida; use cases difíceis de testar sem mocks pesados; producer carregando regra de domínio (`ref_months_affected`) viola R-ADAPTER-001.2.
- **Motivo da rejeição**: os 3 pilares deste ADR atacam dores reais já visíveis na techspec.

### C. Adotar apenas smart constructors (pular `Decide*` puro e domain events tipados)
- **Vantagens**: menor refactor; ganho imediato em validação.
- **Desvantagens**: deixa producers carregando lógica de evento (`ref_months_affected`) e use cases ainda misturando IO + decisão; perde 60% do ganho.
- **Motivo da rejeição**: meio-termo sem coerência interna.

## Consequências

### Benefícios Esperados

- **Invariantes garantidos pelo tipo.** Impossível instanciar `InstallmentCount{v: 25}` ou `Money{cents: -100}` em qualquer ponto do sistema; checks em check constraints SQL viram defesa-em-profundidade, não primeira linha.
- **Testabilidade do core.** `Decide*` testável com `suite.Run` puro, sem mocks de repo, `CardLookup` ou `CategoryValidator`. Aumenta cobertura efetiva do core sem testcontainers.
- **R-ADAPTER-001.2 reforçada.** Producer fica trivialmente fino — mapeia struct A → struct B; impossível acidentalmente puxar regra de domínio.
- **Lista de erros de validação completa** no response 400, melhor UX.
- **ADR-003** (single event com `ref_months_affected`) fica naturalmente consistente porque o cálculo vive no `Decide*`, único lugar.

### Trade-offs e Custos

- **+1 arquivo por workflow não-trivial**: `domain/services/<aggregate>_workflow.go` (5 arquivos no MVP).
- **+1 tipo por VO**: `InstallmentCount`, `Money`, `Description`, `DayOfMonth`, `RefMonth` (já no PRD).
- **+1 pacote utilitário**: `internal/transactions/domain/option/` (~30 linhas).
- **+1 tipo por evento**: `domain/entities/events.go` com `CardPurchaseCreated`, `CardPurchaseUpdated`, `CardPurchaseDeleted`, `TransactionCreated`, `TransactionUpdated`, `TransactionDeleted`, `RecurringTemplateCreated`, `RecurringTemplateUpdated`, `RecurringTemplateDeleted` (≈ 9 structs simples).
- **Use case ~20 linhas maior**, mas cada bloco é menor e tem propósito único.
- **Curva de aprendizado** para colaboradores acostumados ao padrão `internal/budgets` atual — mitigada por README curto na pasta `domain/services/`.

### Riscos e Mitigações

- **Risco**: outros módulos não adotam o padrão e geram inconsistência percebida.
  - **Mitigação**: este ADR é deliberadamente escopado a `internal/transactions`. Se a prática provar valor, ADR transversal futuro pode propagar.
- **Risco**: `Option[T]` genérico vira tentação de reuso fora do módulo sem coordenação.
  - **Mitigação**: pacote vive em `internal/transactions/domain/option/` — uso local; promover a `internal/platform/option` exige novo ADR.
- **Risco**: developer pula o `Decide*` e coloca regra direto no use case "por agilidade".
  - **Mitigação**: code review usa gate textual: "se há regra de domínio fora de `Decide*` num dos 5 workflows da tabela, PR é bloqueado".

## Plano de Implementação

1. **Pacote `option`**: criar `internal/transactions/domain/option/option.go` com `Option[T any]` (`Some`, `None`, `Get`, `IsPresent`). ≤ 30 linhas.
1.1. **Pacote `commands`** _(audit fix #1)_: criar `internal/transactions/domain/commands/` com tipos exportados (`CreateTransaction`, `UpdateTransaction`, `CreateCardPurchase`, `UpdateCardPurchase`, `CreateRecurringTemplate`, `UpdateRecurringTemplate`, `MaterializeRecurring`) e smart constructors `NewXxx(raw RawXxx, principal auth.Principal) (Xxx, error)` que rodam fora da camada `application` para evitar import cycle `domain/services → application`. Espelha `internal/budgets/domain/commands/`. Workflows consomem `commands.Xxx` diretamente.
2. **Value Objects**: `Money`, `InstallmentCount`, `Description`, `DayOfMonth`, `RefMonth`, **`CardBillingSnapshot`** (audit fix #2 — nome único em todo o módulo) em `internal/transactions/domain/valueobjects/`. Smart constructors + sentinel errors (`ErrXxx`) + `Equals` quando útil.
3. **Domain events**: `internal/transactions/domain/entities/events.go` com 9 structs simples (`event_id`, `aggregate_id`, `user_id`, `occurred_at`, campos específicos do evento).
4. **Workflows puros**: 5 arquivos em `internal/transactions/domain/services/`:
   - `transaction_workflow.go` (`DecideCreate`, `DecideUpdate`)
   - `card_purchase_workflow.go` (`DecideCreate`, `DecideUpdate`)
   - `recurring_workflow.go` (`DecideMaterializeForDay`)
5. **Use cases**: adicionar `validate*` privada com `errors.Join`; chamar `Decide*` antes do `uow.Execute`; passar `decision.Event` ao publisher via `mapDomainEventToEnvelope`.
6. **Producers**: refatorar para receber `domain event` em vez de campos soltos; payload JSON serializa direto do tipo de domínio (campos `json:` ficam no struct de evento, sem ressuscitar payload no producer).
7. **Testes**: cada `Decide*` ganha suite própria sem mocks; use case mantém suite com mocks só para IO; producer mantém suite verificando que mapeamento preserva campos.

Critério de "decisão concluída": os 5 workflows da tabela estão refatorados; gate de revisão "regra fora de `Decide*` = bloqueia PR" documentado em `.claude/rules/transactions-workflows.md` (criar como parte da implementação).

## Monitoramento e Validação

- **Cobertura de teste do `domain/services/`**: meta ≥ 90% line coverage nos 5 workflows; `Decide*` sem mocks deve ser barato de testar.
- **Cobertura de teste do `application/usecases/` (5 workflows não-triviais)**: meta ≥ 80% com mocks só para IO (`RepositoryFactory`, `CardLookup`, `CategoryValidator`, `Publisher`, `Idempotency`).
- **`golangci-lint`**: nenhuma regra extra; aderência ao padrão Go idiomático garante revisão tradicional.
- **Métrica de qualidade**: medir incidência de "validação repetida" em PRs (revisores marcam quando veem `if x <= 0` em arquivo que não é `valueobjects/`) — meta = 0.
- **Sinal de problema**: developer comentando "tive que importar X só para chamar `Decide*`" → revisitar ergonomia.

## Impacto em Documentação e Operação

- **`.claude/rules/transactions-workflows.md`**: novo arquivo de regra hard para o módulo, codificando: (a) lista dos 5 workflows com `Decide*` obrigatório, (b) proibição de validação fora de smart constructors, (c) producers só mapeiam domain event → envelope.
- **README curto** em `internal/transactions/domain/services/README.md`: 1 parágrafo explicando "por que `Decide*` é puro e onde efeitos vivem".
- **Runbook `docs/runbooks/transactions.md`**: nenhuma seção operacional nova; ADR é interna ao desenho.
- **Techspec**: vincula esta ADR na seção "Decisões Chave" (próximo edit).

## Revisão Futura

Revisitar em 6 meses (≈ 2026-12-12) ou antes se:

- Cobertura efetiva do `domain/services/` ficar < 80% (sinal de que `Decide*` está difícil de testar — provavelmente sujo de IO acidental).
- Aparecer demanda forte para reusar `Option[T]` em outros módulos (promover a `internal/platform/option` via ADR transversal).
- Surgir necessidade de propagar o padrão para `internal/budgets` ou `internal/billing` (ADR transversal de governance).
- Houver atrito sustentado de novos colaboradores com o padrão `Decide*` (revisitar README, exemplos, ou reverter para casos específicos).
