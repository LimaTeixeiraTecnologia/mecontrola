# Tarefa 5.0: Infra HTTP card — handlers, router, wiring

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adaptar a camada HTTP de `internal/card` ao novo contrato: `createCardRequest`/`updateCardRequest` sem
`closing_day`/`limit_cents`, com `bank`; novo handler `best_purchase_day` (`GET /cards/best-purchase-day`)
sob o grupo autenticado; remover o handler e a rota `PATCH /cards/{id}/limit`; ajustar o wiring em
`module.go`. Adapters permanecem finos (R-ADAPTER-001): só mapeiam request → use case.

<requirements>
- RF-01: `POST /cards` aceita `{nickname, bank, due_day}`; resposta com campos derivados.
- RF-05: remover `handlers/update_card_limit.go` e a rota `PATCH /cards/{id}/limit`; remover wiring correspondente.
- RF-13: novo handler `GET /cards/best-purchase-day?bank=&due_day=`, **registrado antes** de `Route("/{id}")`, sob `gatewayAuth`+`RequireUser`.
- RF-18: contrato HTTP reflete o novo formato (validação em `openapi.yaml` fica na tarefa 7.0).
- R-ADAPTER-001: zero comentários; sem SQL/regra de negócio no handler; fluxo handler → usecase.
</requirements>

## Subtarefas

- [ ] 5.1 Editar `handlers/create.go` (`createCardRequest{Nickname, Bank, DueDay}`) e `handlers/update.go` (`{Nickname?, Bank?, DueDay?}`).
- [ ] 5.2 Criar `handlers/best_purchase_day.go`: lê query `bank`/`due_day`, monta `input.BestPurchaseDay`, chama use case, responde `{closing_day, best_purchase_day}`.
- [ ] 5.3 Editar `router.go`: registrar `GET /best-purchase-day` **antes** de `Route("/{id}")`; remover `Patch("/limit", ...)`.
- [ ] 5.4 Remover `handlers/update_card_limit.go` (+test).
- [ ] 5.5 Editar `module.go`: instanciar `BestPurchaseDay` UC + handler; remover `UpdateCardLimitUC`/handler do struct e do wiring; injetar `BankDaysReader` nos use cases de create/update.

## Detalhes de Implementação

Ver `techspec.md` §"Endpoints de API" (auth + ordering chi) e §"Fluxo de Dados". Handler novo segue o
padrão dos existentes (`invoice_for.go`): parse de query, `Validate()` após `defer span.End()`,
mapeamento de erro de domínio para status HTTP. Sem principal necessário no cálculo, mas rota herda auth.

## Critérios de Sucesso

- `POST /cards` e `PUT /cards/{id}` operam no novo contrato; `PATCH /cards/{id}/limit` retorna 404 (rota removida).
- `GET /cards/best-purchase-day?bank=nubank&due_day=20` → `{closing_day:13, best_purchase_day:14}`.
- `router_test.go` prova que `best-purchase-day` não é capturado como `{id}`.
- Zero comentários nos handlers; build do módulo verde.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: `handlers/create_test.go`, `update_test.go`, `best_purchase_day_test.go`, `router_test.go`.
- [ ] Testes de integração: n/a nesta tarefa (coberto em 7.0 via e2e).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/card/infrastructure/http/server/handlers/{create,update,best_purchase_day}.go`, `router.go`, `internal/card/module.go`
- Remover: `internal/card/infrastructure/http/server/handlers/update_card_limit.go` (+test)
