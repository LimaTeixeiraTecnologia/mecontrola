# Refactor advisory — `internal/card`

- **task_type**: `refactor.advisory` (sem alteração de comportamento; sem execução)
- **Modo**: advisory — só passa para `execution` mediante pedido explícito do usuário
- **Skills obrigatórias na execução**: `.agents/skills/refactor/SKILL.md` + `.agents/skills/go-implementation/SKILL.md` (Etapas 1–5, R0–R7, R-ADAPTER-001)

## Contexto

`internal/card` é um módulo CRUD relativamente enxuto (7 use cases, 6 handlers HTTP, 1 repositório Postgres) já bem estruturado em camadas DDD com value objects validados (`CardName`, `Nickname`, `BillingCycle`) e um único caminho de cálculo de negócio relevante: `InvoiceFor` (closing/due dates a partir de uma data de compra + `BillingCycle` + timezone São Paulo).

A revisão exploratória encontrou pontos de duplicação e atrito que vale endereçar **sem mudar comportamento**, e identificou onde ideias de DMMF (Domain Modeling Made Functional) trazem ganho real versus onde seriam overengineering para este módulo. Não há eventos de domínio hoje, e a discovery confirma que **introduzir eventos no card não se justifica** no escopo atual (sem consumidor real).

## Dores atuais (priorizadas)

1. **Duplicação de utilitários por use case**
   - `toCardOutput(entities.Card) output.Card` repetido em `create_card.go`, `list_cards.go`, `update_card.go` e provavelmente em `get_card.go`/`get_card_for_user.go`.
   - `classifyCardOutcome(err)` / `isCardValidationError(err)` redefinidos em múltiplos use cases.
   - Encode/decode de cursor de paginação aparece tanto em `application/usecases/list_cards.go` quanto em `infrastructure/repositories/postgres/card_repository.go`.

2. **Cálculo de fatura espalhado entre `services/billing_cycle.go` e o use case**
   - `usecases/invoice_for.go:61–71` faz `services.InvoiceFor(...)` mas também aplica formatação de data em fuso São Paulo (mistura cálculo + apresentação dentro do use case).
   - O service é puro e determinístico — está no lugar certo — mas a formatação `Format("2006-01-02")` deveria ficar no mapper de output.

3. **Parsing de input no handler HTTP**
   - `handlers/invoice_for.go:59` faz `time.Parse("2006-01-02", forParam)` direto no adapter, atravessando a fronteira input → use case com `time.Time` já parseado. Hoje funciona, mas mistura responsabilidades; o ideal é o handler entregar a string ao input DTO e o use case (ou um construtor de input) validar.

4. **Idempotência colada nos use cases de mutação**
   - `CreateCard`, `UpdateCard`, `SoftDeleteCard` carregam a mesma estrutura de checagem (`scope="card"`, expiração 24h). Não é incorreto, mas é repetitivo.

5. **Mapping manual entre `entities.Card` e DTOs**
   - Sem mapper único: cada use case escreve o seu, com risco de drift quando um campo novo entrar (ex.: o recém-adicionado `GetCardForUser` mostra que a evolução já está acontecendo).

6. **Validações de domínio com mensagens disformes**
   - `domain/errors.go` tem 14 erros, alguns retornados como sentinel e outros embrulhados com `fmt.Errorf`. Tornar o wrapping consistente facilita a classificação no adapter.

## Onde DMMF ajuda (adoção seletiva)

| Padrão DMMF | Aplicação em `card` | Ganho objetivo |
|---|---|---|
| Smart constructors | Já existem (`NewCardName`, `NewNickname`, `NewBillingCycle`). Manter e estender mensagens com `errors.Join` para coletar múltiplos erros de validação em `CreateCard`/`UpdateCard`. | UX melhor de 400; invariantes garantidos por tipo. |
| Workflow explícito | Apenas para `InvoiceFor`: extrair função pura `services.DecideInvoice(purchase, cycle, tz) Invoice` (já é puro; só renomear/clarificar fronteira) e remover formatação do use case. | Cálculo testável em isolamento; use case vira orquestrador fino. |
| Domain types para output | Criar `application/mappers/card_mapper.go` único: `ToCardOutput(entities.Card) output.Card` e `ToInvoiceOutput(domain.Invoice, tz) output.Invoice`. | Elimina duplicação; ponto único para evolução de schema. |
| Erros semânticos | Padronizar wrapping: `fmt.Errorf("card: <contexto>: %w", domain.ErrXxx)` em todos os pontos de retorno; classificação no adapter via `errors.Is`. | Logs e respostas HTTP consistentes; testes de classificação mais simples. |

## Onde DMMF NÃO ajuda (rejeitar)

- **`Decide*` para `GetCard`, `ListCards`, `SoftDeleteCard`**: são IO puro / orquestração simples. Indireção sem ganho.
- **Domain events / outbox no `card`**: nenhum consumidor real hoje. Introduzir publisher seria infra sem cliente — violaria a restrição "não criar interfaces sem consumidor real".
- **`Option[T]` genérico**: `Nickname` já é VO obrigatório; não há nullability semântica no agregado. Não trocar `*string` por `Option[T]` só por estética.
- **Result/Either / pipeline DSL**: rejeitados pela governança (`domain-modeling.md` anti-padrões; `go-implementation` prevalece).
- **`CardBillingSnapshot` à la transactions**: card não tem reconciliação cross-aggregate. Snapshot é resposta a um problema que `card` não tem.

## Plano incremental (pequeno, sem mudança de comportamento)

Cada passo é um commit isolado, com testes existentes verdes em cada parada.

### Passo 1 — Mapper único `entities.Card → output.Card`
- Criar `internal/card/application/mappers/card_mapper.go` com `ToCardOutput(entities.Card) output.Card` e `ToCardListOutput([]entities.Card, nextCursor string) output.CardList`.
- Substituir as cópias inline em `create_card.go`, `update_card.go`, `list_cards.go`, `get_card.go`, `get_card_for_user.go`.
- Validação: testes de use case existentes devem passar sem alteração; rodar `go test ./internal/card/...`.

### Passo 2 — Helpers de erro/classificação compartilhados
- Mover `classifyCardOutcome` e `isCardValidationError` para `internal/card/application/usecases/errors.go` (ou um sub-pacote `internal/card/application/errorx`), removendo as cópias dos use cases.
- Sem mudança de mensagem; sem mudança de status code.
- Validação: testes de handler que verificam status de erro permanecem verdes.

### Passo 3 — Cursor de paginação em um lugar só
- Definir encode/decode em `internal/card/application/pagination/cursor.go` (ou similar).
- Use case e repositório consomem o mesmo helper.
- Validação: `list_cards_test.go` (use case) e `card_repository_integration_test.go` permanecem verdes; testar round-trip encode/decode com bytes determinísticos.

### Passo 4 — Mover formatação de data de `InvoiceFor` para o mapper de output
- `services.InvoiceFor` continua retornando `Invoice{ClosingDate time.Time, DueDate time.Time}` (já é puro).
- Use case devolve a struct de domínio; mapper `ToInvoiceOutput(invoice, tz)` formata em `2006-01-02`.
- Validação: testes de handler `invoice_for_test.go` permanecem verdes (mesmo JSON de saída).

### Passo 5 — Parsing de `for=YYYY-MM-DD` no construtor de input
- Mover `time.Parse` do handler para `input.NewInvoiceFor(cardID, userID, forRaw string)` que devolve `(input.InvoiceFor, error)`.
- Handler vira fino: extrai querystring, chama `NewInvoiceFor`, mapeia erro para 400.
- Validação: handler tests permanecem verdes; mensagem de erro 400 idêntica.

### Passo 6 — Wrapping consistente de erros de domínio
- Auditar `domain/errors.go` e pontos de retorno: garantir `fmt.Errorf("card: <ctx>: %w", ErrXxx)` em toda fronteira; adapter classifica via `errors.Is`.
- Sem mudança de status code nem de mensagem visível na resposta HTTP.
- Validação: `pii_regression_test.go`, contract tests e openapi tests permanecem verdes.

### Passo 7 (opcional, só se pedido) — `errors.Join` em `CreateCard`/`UpdateCard`
- Acumular erros de validação dos VOs (`CardName`, `Nickname`, `BillingCycle`) num único 400 com lista de campos.
- **Atenção**: isso muda o **shape** da resposta de erro — não é puro refactor. Marcar como **fora do escopo advisory** e exigir aprovação explícita antes de executar.

## Restrições reafirmadas

- Sem mudar contrato HTTP, payload de erro, formato de cursor, formato de data, ordem de listagem ou semântica de soft-delete.
- Sem introduzir `panic`, `init()`, `var _ Interface = (*Type)(nil)`, abstração de tempo ou comentários em código Go (R-ADAPTER-001.1).
- Adapters continuam finos (R-ADAPTER-001.2): handler/consumer/job/producer só orquestram.
- Não criar `RepositoryFactory` novo, `Publisher` ou interface sem consumidor real.

## Arquivos críticos (referência)

- Wiring: `internal/card/module.go`
- Cálculo: `internal/card/domain/services/billing_cycle.go`, `internal/card/domain/services/timezone.go`
- Use cases: `internal/card/application/usecases/{create,get,list,update,soft_delete,invoice_for,get_card_for_user}_card.go`
- DTOs: `internal/card/application/dtos/{input,output}/`
- VOs: `internal/card/domain/valueobjects/{card_name,nickname,billing_cycle}.go`
- Handlers: `internal/card/infrastructure/http/server/handlers/`
- Repositório: `internal/card/infrastructure/repositories/postgres/card_repository.go`
- Erros: `internal/card/domain/errors.go`

## Validações proporcionais (para a execução, quando solicitada)

- `go build ./...`
- `go test ./internal/card/...` (unit + integration; integration requer Postgres)
- `go vet ./internal/card/...`
- Gate R-ADAPTER-001.1 (zero comentários em `.go` de produção)
- Gate R-ADAPTER-001.2 (sem SQL direto em adapter)
- Diff de payloads HTTP antes/depois nos golden tests de handler (`pii_regression_test.go`, contract tests)

## Status final

`needs_input` — advisory entregue; aguardando decisão do usuário sobre executar (e, em caso afirmativo, se inclui Passo 7 opcional ou se fica restrito aos Passos 1–6 estritamente sem mudança de comportamento).
