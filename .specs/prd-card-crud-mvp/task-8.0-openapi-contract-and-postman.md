# Tarefa 8.0: Contrato OpenAPI 3.1 + contract tests (kin-openapi) + Postman collection

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Publicar o contrato HTTP completo do `card` como OpenAPI 3.1 (`internal/card/infrastructure/http/server/openapi.yaml`), validar via `github.com/getkin/kin-openapi` em contract tests com golden files (`testdata/`), e gerar coleção Postman a partir do YAML. Garante M-09 (100% dos endpoints documentados e validados) e RF-47 (golden files).

<requirements>
- `openapi.yaml` (OpenAPI 3.1) cobre os 6 endpoints com:
  - schemas request/response completos;
  - status codes 200/201/204/400/401/404/409/500;
  - headers obrigatórios (`X-User-ID`, `Idempotency-Key`);
  - mensagens de erro padrão em pt-BR.
- Dep nova `github.com/getkin/kin-openapi` adicionada ao `go.mod`.
- Contract tests carregam o YAML, sobem `chi` com `CardRouter`, executam requests canônicos e validam: (a) response status; (b) response body contra schema; (c) golden file diff byte-a-byte.
- Goldens em `internal/card/infrastructure/http/server/testdata/golden/<endpoint>_<status>.json`.
- Cenário replay end-to-end: POST com mesma `Idempotency-Key` duas vezes → resposta byte-idêntica (validar via golden).
- Postman collection em `docs/postman/card.postman_collection.json` gerada a partir do YAML (script ou subskill).
- Atualizar `mockery.yml` se novos mocks forem necessários para contract tests.
- Zero comentários em `.go` de produção.
</requirements>

## Subtarefas

- [ ] 8.1 `internal/card/infrastructure/http/server/openapi.yaml` — contrato completo.
- [ ] 8.2 Adicionar `github.com/getkin/kin-openapi` ao `go.mod` via `go get` + commit `go.sum`.
- [ ] 8.3 `internal/card/infrastructure/http/server/openapi_test.go` — carrega YAML, valida sintaxe via `openapi3.Loader`.
- [ ] 8.4 `internal/card/infrastructure/http/server/contract_test.go` — para cada endpoint canônico: monta request, executa via `httptest`, valida response contra schema + golden diff.
- [ ] 8.5 `testdata/golden/*.json` — fixtures canônicas (POST 201, GET 200 lista vazia/cheia, GET 200 individual, GET 404, PUT 200, DELETE 204, GET invoices 200).
- [ ] 8.6 `testdata/golden/replay_*.json` — golden para validar replay byte-idêntico.
- [ ] 8.7 Gerar `docs/postman/card.postman_collection.json` via skill `postman-collection-generator` aplicada ao `openapi.yaml`.
- [ ] 8.8 Test E2E contract de replay: POST com `Idempotency-Key=test-001` duas vezes → goldens iguais byte-a-byte.

## Detalhes de Implementação

Ver `.specs/prd-card-crud-mvp/techspec.md` §"Testes E2E" + §"Endpoints de API". Schema do YAML deve refletir DTOs `output/` da camada `application/`.

## Critérios de Sucesso

- `go test -race -count=1 ./internal/card/infrastructure/http/server/...` verde, contract tests inclusos.
- `openapi.yaml` valida sintaticamente via `openapi3.Loader.LoadFromFile` (test 8.3).
- Cada response real bate com schema do YAML; falha de bate → test reporta path do schema esperado.
- Golden replay (8.8) verde.
- Postman collection válida (importa em Postman sem erro).
- Artifact `openapi.yaml` exportado por CI (validar entrada no Taskfile ou workflow).

### Definition of Done

- [ ] `openapi.yaml` cobre 6 endpoints + erros canônicos.
- [ ] Dep `kin-openapi` adicionada (`go.mod` + `go.sum`).
- [ ] ≥ 10 goldens em `testdata/golden/`.
- [ ] Contract tests verdes.
- [ ] Postman collection gerada e commitada.
- [ ] CI publica `openapi.yaml` como artifact (entrada documentada no PR).
- [ ] RF-29, RF-47 explicitamente apontados no PR.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `postman-collection-generator` — gerar `card.postman_collection.json` a partir do `openapi.yaml` para distribuição testável aos consumidores (front, agentes IA, WhatsApp bot).

## Testes da Tarefa

- [ ] Testes unitários: validação sintática do YAML.
- [ ] Testes de integração: contract tests com golden diff + cenário replay E2E.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/card/infrastructure/http/server/openapi.yaml` (novo)
- `internal/card/infrastructure/http/server/openapi_test.go` (novo)
- `internal/card/infrastructure/http/server/contract_test.go` (novo)
- `internal/card/infrastructure/http/server/testdata/golden/*.json` (novo)
- `docs/postman/card.postman_collection.json` (novo)
- `go.mod`/`go.sum` (modificar — adicionar `kin-openapi`)
