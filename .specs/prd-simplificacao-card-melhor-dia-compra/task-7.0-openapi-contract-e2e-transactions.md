# Tarefa 7.0: OpenAPI + testes de contrato + e2e; não-regressão de `internal/transactions`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechar o contrato público e a validação end-to-end: atualizar `openapi.yaml` ao novo formato, revalidar
os testes de contrato, atualizar/adicionar os cenários e2e (godog) e **provar por suíte verde e zero diff**
que `internal/transactions` não sofreu alteração de contrato (RF-14).

<requirements>
- RF-18: `openapi.yaml` — `CreateCardRequest`/`UpdateCardRequest` sem `closing_day`/`limit_cents`, com `bank`; remover `UpdateCardLimitRequest` e a rota `/limit`; `Card` sem `limit_cents`, com `best_purchase_day`, mantendo `closing_day`; novo path `GET /cards/best-purchase-day`.
- RF-14: `internal/transactions` inalterado — suíte de transactions verde **sem diff** de código no módulo.
- e2e cobre: criar cartão (bank), consulta `best-purchase-day` (Nubank/20→14), remoção da rota de limite.
</requirements>

## Subtarefas

- [ ] 7.1 Atualizar `internal/card/openapi.yaml`: schemas + paths conforme techspec §"Endpoints de API".
- [ ] 7.2 Atualizar `infrastructure/http/server/{contract_openapi_test.go,contract_test.go,openapi_test.go}` para o novo contrato.
- [ ] 7.3 Atualizar e2e `internal/card/e2e/`: `steps_create_test.go`/`steps_update_test.go`/`steps_read_list_test.go` (bank, sem limit/closing de entrada, resposta com `best_purchase_day`); remover passos de `update_card_limit`; novo cenário `best-purchase-day`.
- [ ] 7.4 Rodar a suíte de `internal/transactions` e confirmar verde **sem** qualquer alteração de arquivo em `internal/transactions/` (evidência de RF-14).

## Detalhes de Implementação

Ver `techspec.md` §"Endpoints de API", §"Pontos de Integração" (RF-14) e §"Abordagem de Testes".
`internal/transactions` consome `cycle.ClosingDay`/`DueDay` via card lookup; como ambos permanecem
persistidos, o contrato é preservado por construção — a tarefa apenas **verifica**.

## Critérios de Sucesso

- `openapi.yaml` válido; testes de contrato verdes no novo formato.
- e2e verde: criação com `bank`, `GET best-purchase-day` retorna `{13,14}` para Nubank/20; rota `/limit` ausente.
- `go test ./internal/transactions/...` verde; `git diff --stat internal/transactions/` vazio.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: contrato (`contract_test.go`, `openapi_test.go`).
- [ ] Testes de integração/e2e: `internal/card/e2e/` (godog); suíte de `internal/transactions` como gate de não-regressão.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/card/openapi.yaml`, `internal/card/infrastructure/http/server/{contract_openapi_test,contract_test,openapi_test}.go`
- `internal/card/e2e/*_test.go`
- `internal/transactions/**` (somente verificação — não alterar)
