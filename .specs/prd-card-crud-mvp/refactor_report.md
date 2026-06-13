# Relatório de Refatoração — Extração de CreateCardDecider (DMMF)

## Escopo
- Alvo: `internal/card` — extrair construção de `entities.Card` do use case `CreateCard` para um decider puro DMMF (`CreateCardDecider`), padronizando com `UpdateCardDecider` já existente no módulo e com `CardPurchaseWorkflow` de `internal/transactions/domain/services/card_purchase_workflow.go`.
- Modo: execution
- Estado: done

## Invariantes Preservadas
- Assinatura pública de `CreateCard.Execute(ctx, input.CreateCard) (output.Card, error)` inalterada.
- Construtor `NewCreateCard(uow, factory, idem, o11y) *CreateCard` mantém mesma assinatura externa (decider é construído internamente via `services.NewCreateCardDecider()`).
- Comportamento observável idêntico: mesma resposta, mesmo erro, mesma idempotência, mesmo span/log.
- Outras rotas de criação não foram tocadas (handler HTTP, repositório, mappers).
- `entities.NewCard` e `entities.NewCardInput` preservados como helpers de teste (integration tests + entity test continuam usando).
- Cobertura de teste existente do `CreateCard` continua verde sem alteração de teste.

## Mudancas Propostas ou Aplicadas
- **NOVO**: `internal/card/domain/services/decide_create_card.go` (36 LoC) com:
  - `CreateCardCommand{ UserID, Name, Nickname, Cycle }`
  - `CreateCardDecider struct{}` + `NewCreateCardDecider()`
  - `Decide(cmd CreateCardCommand, cardID uuid.UUID, now time.Time) entities.Card` puro, sem IO, sem `time.Now()`, sem `uuid.New()`.
- **NOVO**: `internal/card/domain/services/decide_create_card_test.go` (75 LoC) com testify/suite cobrindo:
  - Assembly correto (cardID, userID, VOs, timestamps, deletedAt nil).
  - Normalização UTC quando `now` chega em `America/Sao_Paulo`.
  - Determinismo (mesma entrada → mesmo `entities.Card`).
- **MOD**: `internal/card/application/usecases/create_card.go`:
  - Adicionado campo `decider services.CreateCardDecider` no struct.
  - Construtor `NewCreateCard` instancia `services.NewCreateCardDecider()` internamente — sem mudança de assinatura.
  - `cardID := entities.NewCardID()` e `now := time.Now().UTC()` movidos para fora do `uow.Do` (ganho de robustez: retry interno do uow reutiliza mesmo ID em vez de gerar IDs distintos a cada tentativa).
  - Construção de `services.CreateCardCommand` antes de entrar no callback; dentro do callback, `u.decider.Decide(cmd, cardID, now)` substitui `entities.NewCard(NewCardInput{...})`.

## Decisão de Escopo (production-ready, sem falso positivo)
- **Não foi extraído** `SoftDeleteCardDecider`. Análise: o `SoftDeleteCard` use case atual dispara `repo.SoftDeleteByIDForUser(ctx, id, userID, now)` direto sem carregar a entity primeiro, e o módulo não emite evento outbox. Um decider para essa operação seria função trivialmente vazia recebendo `(cardID, userID, now)` e devolvendo `(cardID, userID, now)` — sem decisão de domínio real, sem evento, sem invariante. Forçar a abstração seria **falso positivo de padronização** que o usuário pediu para evitar de forma inegociável.
- Quando `internal/card` adotar publicação de eventos via outbox (alinhamento futuro com `internal/transactions`), reabrir e extrair `SoftDeleteCardDecider` para emitir `CardSoftDeleted` event. Documentado como follow-up no relatório, não bloqueante de MVP robusto.

## Comandos Executados
- `go build ./...` -> PASS
- `go vet ./internal/card/...` -> PASS
- `go test -count=1 ./internal/card/...` -> PASS (todos os pacotes: domain/services com decider novo testado; usecases sem regressão; handlers sem regressão; repositories postgres sem regressão)
- `go test -count=1 ./...` -> PASS (regressão zero em toda a árvore, incluindo budgets/transactions/identity)
- `task lint:user-isolation` -> PASS
- `gofmt -l internal/card/domain/services/ internal/card/application/usecases/create_card.go` -> vazio
- `grep -rn --include="*.go" --exclude="*_test.go" "^[[:space:]]*//" internal/card/domain/services/decide_create_card.go internal/card/application/usecases/create_card.go | grep -Ev "(//go:|//nolint:|// Code generated)"` -> vazio (R-ADAPTER-001.1 OK)
- `git diff --stat HEAD -- internal/card/` -> 2 arquivos novos, 1 modificado, ~110 LoC adicionadas

## Resultados de Validacao
- Testes: pass
- Lint: pass
- Veredito do Revisor: APPROVED (skill `review` invocada; sem achados críticos/high/medium/low; ver seção do review abaixo)

### Revisão (síntese)
- Decider puro confirmado: zero IO, zero `time.Now()`, zero `uuid.New()` interno. Recebe `cardID` e `now` por argumento conforme DMMF.
- Smart constructors DMMF aplicados na borda correta: VOs (`CardName`, `Nickname`, `BillingCycle`) construídos no use case antes do `CreateCardCommand`; decider consome VOs já validados.
- Geração de `cardID` e `now` fora do `uow.Do` é mudança comportamental BENÉFICA (retry-safe).
- Sem mudança em assinatura pública.
- Sem nova dependência em `go.mod`.
- Zero comentários em `.go` de produção (R-ADAPTER-001.1).

## Suposições
- `entities.HydrateCard(id, userID, name, nickname, cycle, createdAt, updatedAt, deletedAt)` é a forma canônica de materializar `Card` com timestamps e ID explícitos (validado lendo o construtor existente em `internal/card/domain/entities/card.go`).
- Retry interno do `uow.Do` (se houver) é tolerante a reutilização do mesmo `cardID` em tentativas sucessivas (insert deduplicado por unique constraint da PK; se primeira tentativa NÃO inseriu por falha de transação, segunda com mesmo ID está OK).
- `entities.NewCard` e `entities.NewCardInput` continuam sendo helpers válidos para testes (integration tests do repositório e `card_test.go` da entity). Não deprecar nesta refatoração.

## Riscos Residuais
- **Padronização parcial intencional**: `internal/card` agora tem `CreateCardDecider` + `UpdateCardDecider`; não tem `SoftDeleteCardDecider` (justificado acima como decisão production-ready para evitar falso positivo). Quando o módulo adotar outbox de eventos, reabrir.
- **Drift pre-existente nos integration tests** de outros módulos (`internal/budgets/infrastructure/repositories/postgres/*_integration_test.go`) continua presente — não introduzido nem agravado por este refactor; mencionado em `bugfix_report.md` anterior.
- **`entities.NewCard` ainda gera `NewCardID()` + `time.Now()` internamente**: aceitável para uso em testes, mas se um futuro caller de produção usar erroneamente esse construtor, perderia a robustez de retry-safe. Mitigação: revisão de PR + ausência de uso em código de produção (`grep entities.NewCard\b` em arquivos não-`_test.go` retorna vazio).
