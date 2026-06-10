# Generated: 2026-06-09T23:55:00Z

# Execution Report — Bugfix C-3 (MVP internal/card)

## Status

`done`

## Escopo

Correção da revisão do MVP `internal/card`:

- **C-3**: Substituir o "contract test" tautológico (que apenas carregava o YAML) por validação REAL do response contra `openapi.yaml`, usando `github.com/getkin/kin-openapi/openapi3filter` + `routers/gorillamux`. Cada operação do contrato passa por:
  1. Construção de `*http.Request` real.
  2. `gorillamux.NewRouter(doc).FindRoute(req)` para resolver `*routers.Route` + `pathParams`.
  3. `openapi3filter.ValidateRequest` para requisições com body (POST/PUT).
  4. Execução do handler real via `chi.Router` montado por `server.NewCardRouter` com use cases mockados.
  5. `openapi3filter.ValidateResponse` confrontando status, headers e body do `httptest.ResponseRecorder` contra o schema.
  6. Property tests por response: decodificação em `map[string]any` e asserts de presença das chaves canônicas snake_case (`id`, `user_id`, `closing_day`, `due_day`, `created_at`, `updated_at`, `items`, `next_cursor`, `closing_date`, `due_date`) + regex `^\d{4}-\d{2}-\d{2}$` para as datas de fatura.

## Arquivos Alterados

Novo:
- `internal/card/infrastructure/http/server/contract_openapi_test.go` — suite `ContractOpenAPISuite` com 6 testes (POST/GET list/GET by id/PUT/DELETE/GET invoices). Reutiliza fixtures `contractCard()`, `contractUpdatedCard()`, `contractInvoice()`, `contractCardID`, `contractUserID` já definidas em `contract_test.go` (mesmo pacote `server_test`). Reutiliza mocks `mockCreateCard`/`mockListCards`/etc. já definidos em `router_test.go`.

Dependências:
- `go.mod`/`go.sum` — adicionada dependência transitiva `github.com/gorilla/mux v1.8.0` exigida por `github.com/getkin/kin-openapi/routers/gorillamux@v0.140.0`. `go mod tidy` aplicado.

## Critérios de Aceite

- Validação real de response contra schema (não tautológica) `-> comprovado: contract_openapi_test.go usa openapi3filter.ValidateResponse com ResponseValidationInput.SetBodyBytes(respBody); experimento de divergência (adicionar required 'xpto' em Invoice) fez TestContract_GetInvoices_RealValidation falhar com "response body doesn't match schema #/components/schemas/Invoice: ... missing property 'xpto'"; reversão do YAML restabeleceu PASS`.
- 6 operações cobertas (POST /cards, GET /cards, GET /cards/{id}, PUT /cards/{id}, DELETE /cards/{id}, GET /cards/{id}/invoices) `-> comprovado: 6 subtestes em ContractOpenAPISuite executando handlers reais e validando contra schema`.
- Property test snake_case `-> comprovado: cada subteste faz s.Contains(m, "<chave>") para todas as chaves canônicas listadas; subteste de Invoice valida regex YYYY-MM-DD via s.Regexp + time.Parse`.
- ValidateRequest aplicado a POST/PUT `-> comprovado: trecho if body != "" { openapi3filter.ValidateRequest(...) } no helper execAndValidate, com erros required.NoError`.
- Zero comentários no arquivo `.go` `-> comprovado: grep -nE "^[[:space:]]*//[^g][^o]" contract_openapi_test.go retorna vazio`.
- Sem regressão em código de produção `-> comprovado: gate R-ADAPTER-001.1 vazio em internal/card/ (produção)`.
- Testes do pacote afetado verdes `-> comprovado: go test ./internal/card/infrastructure/http/server/... -race -> ok`.
- Suíte completa sem regressão `-> comprovado: go test ./... -> sem FAIL`.

## Comandos Executados

```bash
go get github.com/getkin/kin-openapi/routers/gorillamux@v0.140.0    # adicionou gorilla/mux ao go.sum
go build ./...                                                       # exit 0
go test ./internal/card/infrastructure/http/server/... -race -run ContractOpenAPI  # ok 1.912s
# experimento de detecção de divergência:
#   Edit openapi.yaml -> required: [closing_date, due_date, xpto]
#   go test ... -run ContractOpenAPI                                 # FAIL TestContract_GetInvoices_RealValidation
#     erro: "response body doesn't match schema #/components/schemas/Invoice: ... missing property 'xpto'"
#   cp /tmp/openapi.yaml.bak openapi.yaml                            # revertido
#   go test ... -run ContractOpenAPI                                 # ok (novamente verde)
go test ./internal/card/infrastructure/http/server/... -race        # ok (server: 1.787s, handlers: cached)
go test ./...                                                        # sem FAIL
go mod tidy                                                          # go.mod +3 linhas, go.sum +6 linhas (gorilla/mux)
grep -nE "^[[:space:]]*//[^g][^o]" .../contract_openapi_test.go     # vazio (zero comentários)
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" \
  --exclude="*_test.go" "^[[:space:]]*//" internal/card/ \
  | grep -Ev "(//go:|//nolint:|// Code generated)"                  # vazio (R-ADAPTER-001.1 produção)
```

## Riscos Residuais

- `kin-openapi v0.140.0` suporta OpenAPI 3.1 com algumas limitações; a sintaxe `type: ["string", "null"]` usada em `deleted_at` e `next_cursor` foi aceita pelo loader e pelo validator (caso contrário, os testes da operação GET /cards e GET /cards/{id} falhariam ao validar o response). Não foi necessário fallback para nullable 3.0.
- Helper `execAndValidate` reconstrói o request duas vezes (uma para validação OpenAPI, outra para o chi.Router) por causa do consumo do body pelo decoder. Não afeta cobertura nem corretude; mantido por simplicidade.
- Resposta `application/json` produzida por `responses.JSON` casa com o `content.application/json` declarado no spec. Respostas de erro usam `application/problem+json` (ProblemDetail), validadas em `contract_test.go` (golden); a nova suíte cobre apenas os caminhos felizes 2xx, alinhada com o escopo da correção C-3.

## Suposições

- Dependência transitiva `gorilla/mux v1.8.0` introduzida pelo subpacote `routers/gorillamux` é aceitável — alternativa seria implementar lookup manual em `doc.Paths`, mais frágil e duplicando lógica de matching de path templates. A escolha alinha com a sugestão da diretriz da tarefa (uso de `gorillamux.NewRouter`).
- Reutilização de fixtures (`contractCard`, etc.) e mocks (`mockCreateCard`, etc.) do mesmo pacote de teste é o caminho mais barato em token e mantém uma única fonte de verdade para a forma do payload — manter o `contract_test.go` original (com goldens) e o novo `contract_openapi_test.go` (com schema-based validation) como camadas complementares.
