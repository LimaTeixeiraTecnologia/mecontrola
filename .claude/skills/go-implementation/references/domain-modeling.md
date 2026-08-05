# Domain Modeling em Go (DMMF adaptado)

<!-- TL;DR
Princípios de Domain Modeling Made Functional (Scott Wlaschin) adaptados a Go idiomático: estados ilegais irrepresentáveis, smart constructors, discriminated unions via sealed interface, state-as-type, workflow como pipeline de funções pequenas, pure core / IO shell. Rejeita explicitamente importações monádicas (Result/Either, currying, DSL de pipeline).
Keywords: ddd, dmmf, value-object, smart-constructor, sum-type, discriminated-union, state-machine, state-as-type, workflow, pipeline, pure-core
Load complete when: a tarefa modelar VO, agregado, máquina de estado, comando, evento ou refatorar use case monolítico para passos composáveis.
-->

Esta referência traduz os conceitos de DMMF para Go production-ready respeitando R0–R7 e R-ADAPTER-001. **Não é um guia de F# em Go**: o que conflita com idiomas Go está na seção "Anti-padrões rejeitados" no fim do arquivo.

Precedência em conflito com estilo idiomático genérico: ver `.claude/rules/governance.md` — esta referência prevalece para regras de tipo e estado; estilo genérico continua autoritativo para layout, naming e wrapping de erro.

## Princípio 1 — Make illegal states unrepresentable

Tipo de domínio com invariante ou normalização **deve** ter campo não exportado e construtor `New*(...) (T, error)` como única porta de entrada. Sem setter público equivalente.

```go
package valueobjects

import (
	"errors"
	"strings"
)

type Email struct{ addr string }

func NewEmail(raw string) (Email, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	if !strings.Contains(v, "@") {
		return Email{}, errors.New("email: formato invalido")
	}
	return Email{addr: v}, nil
}

func (e Email) String() string { return e.addr }
```

Critérios:
- Campo privado obrigatório quando há regra (validação, normalização, range).
- Construtor retorna `(T, error)` — nunca `*T` só para sinalizar erro por nil.
- Sem `New*` quando o zero value é seguro e não há invariante (R6.4).

## Princípio 2 — Distinct types para IDs e primitivos com significado de domínio

Em vez de propagar `string` ou `uuid.UUID`, declarar um tipo distinto quando:
- O ID cruza fronteira entre agregados ou módulos.
- Houve ou pode haver troca acidental (`userID` passado onde se esperava `subscriptionID`).
- O construtor precisa rejeitar valores fora do formato.

```go
type UserID struct{ v string }

func NewUserID(raw string) (UserID, error) {
	if raw == "" {
		return UserID{}, errors.New("user id: vazio")
	}
	return UserID{v: raw}, nil
}

func (u UserID) String() string { return u.v }
```

**Não migrar IDs primitivos existentes em massa.** Adotar em superfícies novas e em pontos de risco real. Conversão acontece no adapter (handler/repository), nunca no use case.

## Princípio 3 — Discriminated union via sealed interface

Substitui o par `enum status + campos nulláveis` quando há campo exclusivo por variante. A invariante "`graceEnd` só existe em `PastDue`" passa a ser garantida em tempo de compilação.

```go
type SubscriptionState interface{ isSubscriptionState() }

type Active struct {
	since time.Time
}

type PastDue struct {
	since    time.Time
	graceEnd time.Time
}

type Canceled struct {
	at     time.Time
	reason CancelReason
}

func (Active) isSubscriptionState()   {}
func (PastDue) isSubscriptionState()  {}
func (Canceled) isSubscriptionState() {}
```

Consumo com type switch. **Go não tem exaustividade de switch nativa** — o compilador não avisa quando uma variante nova fica sem `case`. Mitigar com uma destas duas táticas (escolher uma, documentar na primeira ocorrência do tipo no pacote):

1. **Default explícito que falha alto** quando o estado é classificado como "impossível":

   ```go
   func describe(s SubscriptionState) string {
   	switch v := s.(type) {
   	case Active:
   		return "ativa desde " + v.since.Format(time.RFC3339)
   	case PastDue:
   		return "vencida; graca ate " + v.graceEnd.Format(time.RFC3339)
   	case Canceled:
   		return "cancelada"
   	default:
   		panic(fmt.Sprintf("subscription state nao tratado: %T", s))
   	}
   }
   ```

   Use apenas quando o tipo é fechado dentro do pacote e *qualquer* variante nova obrigatoriamente passa por revisão deste switch. `panic` aqui é exceção a R5.12 (linha de defesa contra programador, não contra input).

2. **Linter `exhaustive`** (`github.com/nishanths/exhaustive`) configurado no golangci-lint para falhar build quando faltar `case`. Preferível em código de fronteira onde panic não é aceitável.

Não combinar os dois — escolha um regime por pacote para manter o contrato claro.

Usar quando: campos exclusivos por variante, regras de transição complexas, risco de combinar dados inválidos em memória. Não usar quando: estado binário sem dados associados (use `bool` ou enum simples), variante adicionada raramente sem custo de inconsistência observado.

## Princípio 4 — State-as-type para máquinas críticas

Quando "operação X só é válida no estado Y" hoje vive em `if status != X { return err }`, promover cada etapa a um tipo distinto. Transições viram funções puras `(StateA, deps) → (StateB, error)`.

```go
type UnvalidatedOrder struct{ raw RawOrder }
type ValidatedOrder struct{ items []Item; customer CustomerID }
type PricedOrder struct{ items []Item; customer CustomerID; total Cents }

func Validate(o UnvalidatedOrder, repo CustomerRepo) (ValidatedOrder, error) {
	cust, err := repo.FindByEmail(o.raw.CustomerEmail)
	if err != nil {
		return ValidatedOrder{}, err
	}
	items, err := parseItems(o.raw.Items)
	if err != nil {
		return ValidatedOrder{}, err
	}
	return ValidatedOrder{items: items, customer: cust.ID()}, nil
}

func Price(o ValidatedOrder, pricer Pricer) (PricedOrder, error) {
	total, err := pricer.Total(o.items)
	if err != nil {
		return PricedOrder{}, err
	}
	return PricedOrder{items: o.items, customer: o.customer, total: total}, nil
}
```

Aplicar seletivamente. Custo: mais tipos, mais conversão. Benefício: o compilador impede pular etapa. Vale quando o invariante hoje é runtime e o custo de violar é alto.

## Princípio 5 — Workflow como pipeline composável

Use case monolítico (`Execute` com 100+ linhas) vira sequência de funções privadas pequenas, chamadas em ordem, com early return. **Sem framework, sem monad, sem DSL.**

```go
func (uc *ConsumeMagicToken) Execute(ctx context.Context, in dtos.ConsumeMagicTokenInput) (*dtos.ConsumeMagicTokenOutput, error) {
	token, err := uc.loadToken(ctx, in)
	if err != nil {
		return nil, err
	}
	if err := uc.ensureConsumable(token); err != nil {
		return nil, err
	}
	user, err := uc.bindSubscription(ctx, token)
	if err != nil {
		return nil, err
	}
	if err := uc.markConsumed(ctx, token, user); err != nil {
		return nil, err
	}
	return uc.buildOutput(token, user), nil
}
```

Cada passo é um método privado coeso. IO continua orquestrado no shell; passos puros recebem dados e retornam dados ou erro. Não tentar passar `Result` por canal nem encadear via `pipe`.

## Princípio 6 — Pure core / IO shell

- Regras de domínio (entidade, VO, service de domínio): funções puras. Sem `context.Context`, sem chamada de repositório, sem `time.Now()` injetado.
- Use case: shell. Recebe `ctx`, lê via repositório, chama o domínio puro, persiste, publica evento.
- Tempo entra **inline** no shell via `time.Now().UTC()` no momento exato do uso. Sem `Clock` interface, sem `now func() time.Time` (memória `feedback_no_time_abstraction`).

Reforça R6.1 (context só na fronteira) e R6.7 (sem clock no domínio).

## Princípio 7 — Commands e Events como linguagem ubíqua

Já normado por R6.6. Esta referência amplia com evento como struct imutável + factory:

```go
type SubscriptionBoundEvent struct {
	subscriptionID SubscriptionID
	userID         UserID
	boundAt        time.Time
}

func NewSubscriptionBoundEvent(sub SubscriptionID, user UserID) SubscriptionBoundEvent {
	return SubscriptionBoundEvent{
		subscriptionID: sub,
		userID:         user,
		boundAt:        time.Now().UTC(),
	}
}

func (e SubscriptionBoundEvent) SubscriptionID() SubscriptionID { return e.subscriptionID }
func (e SubscriptionBoundEvent) UserID() UserID                 { return e.userID }
func (e SubscriptionBoundEvent) BoundAt() time.Time             { return e.boundAt }
```

Nomes de Command e Event vêm da linguagem ubíqua do bounded context, não do schema técnico.

## Anti-padrões rejeitados (não importar de F#)

| Padrão | Por que rejeitar em Go |
|--------|-----------------------|
| `Result[T]` / `Either[L,R]` customizado | `(T, error)` idiomático cobre o caso; tipo genérico extra obscurece tooling, stack trace e exhaustividade. |
| Currying e partial application | Go não tem suporte de linguagem; emula via closure aumenta indireção sem ganho. |
| DSL de pipeline (`x \|> f \|> g`) | Composição sequencial em Go é sequência de statements com early return — já é a forma idiomática. |
| Functional Options para tudo | Limitar ao gatilho R5.50: ≥4 campos opcionais relevantes. Para Command com 2 campos use struct literal. |
| `Maybe[T]` / `Option[T]` | Usar `(T, bool)` na assinatura ou ponteiro com semântica documentada. Não introduzir tipo paralelo. |

## Quando NÃO aplicar

- VO trivial sem invariante real — não criar wrapper para `string` que só serve de marcador semântico fraco.
- Agregado com 2 estados sem campos exclusivos por variante — `bool` ou enum simples basta.
- CRUD raso sem regra de transição — pipeline em passos é overhead.
- Migração retroativa de IDs ou estados existentes sem demanda explícita — DMMF entra em superfície nova ou refactor escopado, não como sweep.

## Critérios de verificação

### R6.8 — Gate determinístico em VO/entity novos `[HARD]`

Escopo: **apenas arquivos adicionados** em `internal/*/domain/valueobjects/**` ou `internal/*/domain/entities/**`. Não aplica a arquivos modificados nem a legado, eliminando falso positivo de migração retroativa.

```bash
set -euo pipefail
added=$(git diff --name-only --diff-filter=A --cached -- \
  'internal/*/domain/valueobjects/*.go' \
  'internal/*/domain/entities/*.go' \
  ':!*_test.go' || true)
[ -z "$added" ] && exit 0
fail=0
while IFS= read -r f; do
  rg -qU '^type [A-Z]\w+ struct \{[^}]*\b\w+\s+\w' "$f" || continue
  if ! rg -q '^\s+[a-z]\w*\s+\w' "$f"; then
    echo "FAIL R6.8: $f — struct de dominio nova sem campo nao exportado"; fail=1
  fi
  if ! rg -qU 'func New[A-Z]\w+\([^)]*\)\s*\([A-Z]\w+,\s*error\)' "$f"; then
    echo "FAIL R6.8: $f — sem construtor New<Type>(...) (T, error)"; fail=1
  fi
done <<< "$added"
exit "$fail"
```

Características anti-falso-positivo:
- Filtra `--diff-filter=A`: legado intocado nunca dispara.
- Filtra `_test.go`: helpers de teste podem ter struct pública.
- Pula struct vazia (`struct{}`) — marcadores sem dado não exigem invariante.
- Exige tanto campo não exportado quanto construtor `(T, error)` — qualquer um falta, falha; nenhum dos dois sozinho dispara.

### R6.9 — Checklist de revisão em discriminated union novo

Não há grep confiável para "enum + campo nullable" sem alto ruído. Verificação é manual sobre o diff e o agente deve aplicar antes de aprovar `task_type: domain-model`:

1. A entity nova ou refatorada tem campo cujo significado depende de outro campo (`graceEnd` só vale se `status == PastDue`)?
2. Se sim, ela usa discriminated union (sealed interface + variantes) ou continua como `enum + nullable`?
3. Se permanece como enum + nullable: existe justificativa registrada no diff (refatoração escopada fora, variantes < 3, ausência de campos exclusivos)?

Falha em 1+2 sem 3 → revisar antes de merge. Não é gate automático por escolha — falso positivo aqui paga mais caro que falso negativo.

## Gatilhos de carregamento

Carregar esta referência quando o diff:
- introduzir VO ou entity em `**/domain/valueobjects/**` ou `**/domain/entities/**`;
- introduzir Command ou Event tipado em `**/domain/commands/**` ou `**/application/events/**`;
- modelar máquina de estado nova ou refatorar transição existente para tipos distintos;
- decompor `Execute` monolítico em passos nomeados.

Não carregar para: edição de adapter (handler/consumer/producer/job), mudança de wiring, ajuste de teste sem alteração de modelo.
