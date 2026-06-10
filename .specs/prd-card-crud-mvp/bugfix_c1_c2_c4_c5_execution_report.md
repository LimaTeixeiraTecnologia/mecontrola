# Generated: 2026-06-09T23:30:00Z

# Execution Report — Bugfix C-1, C-2, C-4, C-5 (MVP internal/card)

## Status

`done`

## Escopo

Correções da revisão do MVP `internal/card`:
- **C-1**: Tags JSON snake_case nos DTOs de output (`Card`, `CardList`, `Invoice`).
- **C-2**: OpenAPI 3.1 com schemas em snake_case (Card/CardList/Invoice/Error) e `UpdateCardRequest` com `minProperties: 1`.
- **C-4**: PUT `/cards/{id}` agora é sparse — request com ponteiros, validação de payload vazio (`empty_payload`/400), use case aplica VO somente para campos não-nulos.
- **C-5**: Datas de fatura formatadas em `YYYY-MM-DD` no fuso `America/Sao_Paulo` via `services.SaoPauloLocation()`.

## Arquivos Alterados

Produção:
- `internal/card/application/dtos/output/card.go` — tags JSON snake_case + `omitempty` em `deleted_at`.
- `internal/card/application/dtos/output/card_list.go` — `Items []Card` + `NextCursor *string`.
- `internal/card/application/dtos/output/invoice.go` — campos `ClosingDate`/`DueDate` agora `string`.
- `internal/card/application/dtos/input/update_card.go` — campos opcionais via ponteiro.
- `internal/card/application/usecases/update_card.go` — leitura do card antes da mutação; VO aplicado por campo não-nulo; mantém valores existentes; preserva idempotência.
- `internal/card/application/usecases/invoice_for.go` — formatação `YYYY-MM-DD` em SP.
- `internal/card/application/usecases/list_cards.go` — converte cursor vazio → `nil`, popula `Items`.
- `internal/card/infrastructure/http/server/handlers/update.go` — DTO de request com ponteiros + validação `empty_payload`.
- `internal/card/infrastructure/http/server/handlers/list.go` — `len(out.Items)` no log.
- `internal/card/infrastructure/http/server/openapi.yaml` — reescrito em snake_case; schemas `Card`/`CardList`/`Invoice`/`Error`/`ProblemDetail`; `Invoice` com `format: date`; `UpdateCardRequest` com `minProperties: 1`; limites 80/30 alinhados com VO.

Golden files (snake_case + nova representação):
- `internal/card/infrastructure/http/server/testdata/golden/post_cards_201.json`
- `internal/card/infrastructure/http/server/testdata/golden/get_card_200.json`
- `internal/card/infrastructure/http/server/testdata/golden/put_card_200.json`
- `internal/card/infrastructure/http/server/testdata/golden/replay_post_201.json`
- `internal/card/infrastructure/http/server/testdata/golden/get_cards_200.json`
- `internal/card/infrastructure/http/server/testdata/golden/get_invoices_200.json`

Testes ajustados:
- `internal/card/application/usecases/update_card_test.go` — input com ponteiros; teste invalid agora cobre cenário onde card existe e Name vazio.
- `internal/card/application/usecases/list_cards_test.go` — `Cards` → `Items`; comparação `*NextCursor`.
- `internal/card/application/usecases/invoice_for_test.go` — `IsZero()` → `NotEmpty` (string).
- `internal/card/infrastructure/http/server/contract_test.go` — `Items`/`NextCursor: nil`; `Invoice` com strings.
- `internal/card/infrastructure/http/server/handlers/invoice_for_test.go` — Invoice como string; remove import `time`.
- `internal/card/infrastructure/http/server/handlers/list_test.go` — `Cards` → `Items`.
- `internal/card/infrastructure/http/server/handlers/pii_regression_test.go` — `Cards` → `Items`; Invoice como string.

## Critérios de Aceite

- C-1: DTOs de output em snake_case `-> comprovado: cat dos três arquivos + golden files exibem tags snake_case`.
- C-2: OpenAPI 3.1 snake_case (schemas Card/CardList/Invoice/Error) `-> comprovado: openapi.yaml reescrito; openapi_test.go passou (suíte `OpenAPIValidation`)`.
- C-4: PUT sparse com validação de payload vazio (`empty_payload`/400) `-> comprovado: handler update.go retorna 400 + code "empty_payload" quando todos ponteiros nil; usecase preserva campos não enviados; UpdateCardSuite verde`.
- C-5: Datas SP YYYY-MM-DD `-> comprovado: invoice_for.go formata com loc SP; golden get_invoices_200.json contém "2026-01-15"/"2026-01-22"; contract test verde`.
- Zero comentários Go `-> comprovado: gate R-ADAPTER-001.1 retorna vazio`.
- Sem SQL em adapter `-> comprovado: gate R-ADAPTER-001.2 retorna vazio`.

## Comandos Executados

```bash
go build ./...                                           # exit 0
go test ./internal/card/... -race                        # exit 0 (todos pacotes ok)
go test ./...                                            # exit 0 (sem regressão fora de internal/card/)
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" --exclude="*_test.go" \
  "^[[:space:]]*//" internal/card/ \
  | grep -Ev "(//go:|//nolint:|// Code generated)"       # vazio (R-ADAPTER-001.1)
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
  internal/card/infrastructure/http/server/handlers/ \
  internal/card/infrastructure/messaging/database/consumers/ \
  internal/card/infrastructure/messaging/database/producers/ \
  internal/card/infrastructure/jobs/handlers/             # vazio (R-ADAPTER-001.2)
```

## Riscos Residuais

- Clientes externos consumindo a API anterior (PascalCase) precisarão atualizar. Como o MVP ainda não está em produção e a revisão indicou drift vs PRD, o break é a correção esperada.
- `Invoice.ClosingDate`/`DueDate` agora string — qualquer consumidor interno que dependia de `time.Time` precisaria de ajuste. Verificado: apenas testes e o handler de logging foram afetados, todos atualizados.

## Suposições

- O envelope de erro mantido é `ProblemDetail` (RFC 7807) já adotado pelo handler atual; a discovery de C-2 mencionava `Error{code,message}` mas o handler real usa `ErrorWithDetails` do devkit-go que produz ProblemDetail com `errors.code`. Adicionei o schema `Error` no OpenAPI para satisfazer a intenção da discovery, mantendo `ProblemDetail` como envelope real dos endpoints (sem regressão de contrato).
