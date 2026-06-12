# Incorporar DMMF (Domain Modeling Made Functional) ao go-implementation

## Contexto

O usuário quer trazer os conceitos de *Domain Modeling Made Functional* (Scott Wlaschin) para a skill `go-implementation`, mantendo a robustez production-ready e a economia de contexto já estabelecidas. Após survey do codebase (`internal/billing`, `internal/budgets`, `internal/identity`, `internal/onboarding`, `internal/card`):

- **Já existe e é forte**: value objects com smart constructors (`Email`, `Cents`, `CardName`, `Status`), Commands tipados com construtor validador (`internal/budgets/domain/commands/`), eventos tipados (`onboarding/application/events`), invariantes via campos não exportados + getters.
- **Lacuna real (alvo do DMMF)**: estados complexos modelados como *enum + campos nulláveis* (`Subscription`, `MagicToken`, `Budget`) com validação de transição em runtime via `TransitionService`; use cases monolíticos (`ConsumeMagicToken.Execute` ~117 linhas); IDs como `string` primitivo; ausência de discriminated unions.

A skill deve absorver o que **realmente paga em Go idiomático** — *fazer estados ilegais irrepresentáveis*, *discriminated unions via sealed interface*, *state-as-type para máquinas críticas*, *workflows como pipeline composável*, *pure core / IO shell*. Deve **rejeitar explicitamente** importações que conflitam com Go idiomático (Result/Either monádico, partial application chaining, currying).

Tudo respeitando: zero comentários em Go (R-ADAPTER-001.1), sem `var _ Interface = (*Type)(nil)` (R6.4), sem abstração de tempo, DI por construtor, economia de contexto (máx 4 refs simultâneas).

## Alvos de edição

Fonte de verdade: `.agents/skills/go-implementation/` — `.claude/skills/` é symlink e atualiza automaticamente.

### 1. Nova referência — `references/domain-modeling.md` (~700-900 tokens)

Conteúdo enxuto, orientado a regras + exemplos curtos:

- **TL;DR** com keywords e gatilhos de carregamento (padrão do repo).
- **Princípio 1 — Make illegal states unrepresentable**: generalizar o padrão já presente em `Email`/`Cents`. Regra: VO com campo não exportado + `New*` que retorna `(T, error)`. Construtor é a *única* porta de entrada; getters não dão setter equivalente.
- **Princípio 2 — Distinct types para IDs e primitivos com significado de domínio**: `type UserID struct { v string }` quando o ID cruzar fronteira de agregado/módulo e a troca acidental for um risco real. Custo vs. benefício explícito — *não migrar IDs existentes em massa*, adotar em superfícies novas e em pontos onde houve ou pode haver troca acidental.
- **Princípio 3 — Discriminated union via sealed interface** (substitui enum + nullable quando a invariante "campo X só existe no estado Y" precisa ser garantida em tempo de compilação):

  ```go
  type SubscriptionState interface{ isSubscriptionState() }

  type Active struct{ since time.Time }
  type PastDue struct{ since, graceEnd time.Time }
  type Canceled struct{ at time.Time; reason CancelReason }

  func (Active) isSubscriptionState()   {}
  func (PastDue) isSubscriptionState()  {}
  func (Canceled) isSubscriptionState() {}
  ```

  Quando usar: estado tem campos exclusivos por variante (graça só faz sentido em `PastDue`). Quando **não** usar: enum simples sem dados associados (status binário ativo/inativo).
- **Princípio 4 — State-as-type para máquinas críticas**: tipos distintos por etapa quando a transição errada é cara (`UnvalidatedOrder` → `ValidatedOrder` → `PricedOrder`). Cada transição é função pura `(StateA, deps) → (StateB, error)`. Aplicar seletivamente em aggregates onde o invariante "essa operação só é válida nessa etapa" hoje é checado por `if status != X { return err }`.
- **Princípio 5 — Workflow como pipeline composável**: refatorar `Execute` monolítico em passos nomeados privados (`validate`, `enrich`, `persist`, `publish`) chamados em sequência. Sem framework de pipeline, sem monad — apenas funções pequenas com responsabilidade única e early return. Side effects continuam orquestrados na borda do use case.
- **Princípio 6 — Pure core / IO shell**: regras de domínio são funções puras (sem `ctx`, sem IO). Use case é o shell: recebe `ctx`, chama o domínio puro, dispara IO. Reforça R6.1 (context só na fronteira) e R6.7 (sem clock no domínio).
- **Princípio 7 — Commands e Events como linguagem ubíqua**: já coberto por R6.6; este arquivo apenas referencia e amplia com exemplo de evento como struct imutável com factory.
- **Anti-padrões rejeitados (não importar de F#)**: `Result[T]`/`Either[L,R]` customizado (use `(T, error)` idiomático); currying e partial application; pipelines via DSL/operador; functional options para tudo (já delimitado por R5.50). Cada um com justificativa de uma linha.
- **Quando NÃO aplicar**: VO trivial sem invariante; aggregate com 2 estados sem campos exclusivos; CRUD raso onde pipeline = overhead.

### 2. Atualizar `SKILL.md`

- Adicionar **R6.8 — Smart constructor para tipos de domínio com invariante** `[HARD contextual]`: VO ou ID com regra de validação ou normalização exige campo não exportado + `New*(...) (T, error)`. Ativar quando o diff introduzir VO/ID em `**/domain/valueobjects/**` ou `**/domain/entities/**`. Linka `references/domain-modeling.md`.
- Adicionar **R6.9 — Discriminated union para estados com campos exclusivos por variante** `[HARD contextual]`: quando um agregado tem campo que só é válido em um subconjunto de estados (ex.: `graceEnd` apenas em `PastDue`), preferir sealed interface a `enum + nullable`. Ativar somente em aggregate novo ou refatoração explicitamente escopada. Não exige migração retroativa.
- Adicionar item na seção **Patterns frequentes**: *Workflow pipeline* (decomposição de `Execute` em passos nomeados) e *State-as-type* (estados como tipos distintos).
- Atualizar a tabela de Índice de regras com R6.8 e R6.9 → `references/domain-modeling.md`.

### 3. Atualizar `references/INDEX.yaml`

- Adicionar entrada `domain-modeling` (file `domain-modeling.md`, topics `[domain, ddd, dmmf, value-object, sum-type, state-machine, workflow]`, est_tokens ~800).
- Adicionar `domain-modeling` em `optional_refs` de `usecase-write` e `repository`.
- Adicionar novo `task_type: domain-model` com `priority: 70`, `file_patterns: ["**/domain/valueobjects/**", "**/domain/entities/**", "**/domain/services/**", "**/domain/commands/**"]`, `diff_signals: ["type [A-Z][a-zA-Z]+ struct", "func New", "isState()", "func \\(.*\\) Validate"]`, `required_refs: [architecture, domain-modeling]`, `optional_refs: [interfaces, examples-domain-flow, testing-unit]`, `validation_profile: boundary`.

### 4. Atualizar `references/architecture.md`

- Acréscimo curto na seção de princípios: linkar `domain-modeling.md` como leitura complementar quando a tarefa modelar VO, agregado ou state machine. **Não duplicar** conteúdo — apenas ponteiro.

### 5. Atualizar `.claude/rules/governance.md` (precedência)

- Adicionar nota: em conflito entre `domain-modeling.md` e estilo idiomático genérico, prevalece `domain-modeling.md` para regras de tipo/estado; estilo genérico (Uber) continua autoritativo para layout, naming, error wrapping.

## Arquivos não alterados (por escolha consciente)

- `examples-domain-flow.md` — não fundir DMMF aqui; o exemplo novo merece arquivo próprio se for criado, mas ficará **fora do escopo desta entrega** para não inflar tokens. A referência `domain-modeling.md` traz snippets suficientes (≤ 20 linhas cada).
- Código de produção em `internal/` — esta entrega altera **apenas a skill**, sem refatorar agregados existentes. Migrações reais (ex.: `Subscription` para sealed interface) entram como tarefas separadas guiadas pela nova regra, sob revisão explícita.
- IDs primitivos atuais — não há migração retroativa. Regra aplica-se a IDs novos.

## Verificação

1. `cat .agents/skills/go-implementation/references/domain-modeling.md` — arquivo existe, TL;DR presente, anti-padrões listados, ≤ 900 tokens.
2. `grep -E "R6\.(8|9)" .agents/skills/go-implementation/SKILL.md` — regras novas presentes com severidade `[HARD contextual]`.
3. `yq '.task_types[] | select(.id == "domain-model")' .agents/skills/go-implementation/references/INDEX.yaml` — task_type novo aparece e referencia `domain-modeling`.
4. `yq '.references[] | select(.id == "domain-modeling")' .agents/skills/go-implementation/references/INDEX.yaml` — reference registrada.
5. `ls -la .claude/skills/go-implementation/references/domain-modeling.md` — symlink resolve para `.agents/...`.
6. Smoke conceitual: simular Etapa 2 da skill para um diff em `internal/billing/domain/entities/subscription.go` — classificação cai em `domain-model`, carrega `architecture + domain-modeling` (2 refs), respeita `max_additional_references: 4`.
7. Sanidade dos exemplos Go embutidos em `domain-modeling.md`: zero comentários, sem `var _ X = (*Y)(nil)`, sem `now func() time.Time`, sem `Clock` interface — coerência com R-ADAPTER-001.1, R6.4 e feedback `no_time_abstraction`.

## Fora de escopo

- Reescrita de `Subscription`, `MagicToken`, `Budget` para discriminated union (tarefa separada, sob revisão explícita).
- Introdução de tipos distintos para IDs existentes (`UserID`, `SubscriptionID` etc.) — opt-in em superfícies novas.
- Qualquer DSL de pipeline ou tipo `Result[T]` — explicitamente rejeitados na seção de anti-padrões.
- Replicar plano em `docs/planos/2026-06-12-incorporar-dmmf-go-implementation.md` — será feito imediatamente após aprovação do plano (plan mode restringe edições agora).
