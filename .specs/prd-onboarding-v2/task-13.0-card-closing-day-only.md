# Tarefa 13.0: [card] Contrato closing_day-only (due_day opcional + derivado)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Ajustar `internal/card` (módulo dono) para exigir apenas `closing_day` na criação: tornar `DueDay`
**opcional** em `input.CreateCard`/`NewBillingCycle` e **derivar o vencimento internamente** quando
ausente (o `billing_cycle.go` usa `due_day` para calcular datas). Retrocompatível com callers que
ainda enviam `due_day` (HTTP handler, daily). Habilita o onboarding a enviar só o dia de fechamento.

<requirements>
- GAP-V1: `card` exigia closing_day E due_day (1..31); passa a aceitar due_day opcional + derivado.
- DR-11: onboarding/agent enviam só `closing_day`; `card` deriva o resto.
- ADR-005, ADR-006 (regra permanece no dono `internal/card`).
- Retrocompatibilidade: callers existentes que informam `due_day` continuam válidos.
</requirements>

## Subtarefas

- [ ] 13.1 `input.CreateCard`: `DueDay` opcional (`*int` ou regra "0/ausente = derivar"); `Validate()` aceita ausência; quando presente, valida 1..31.
- [ ] 13.2 `NewBillingCycle`/serviço: quando `due_day` ausente, derivar de `closing_day` (regra determinística no card; documentar a regra escolhida).
- [ ] 13.3 `CreateCard.Execute`: usar a derivação; manter comportamento para callers que enviam `due_day`.
- [ ] 13.4 Testes unitários: só closing_day → due derivado; closing+due explícito → preservado; ranges 1..31; cálculo de datas do `billing_cycle` correto em ambos.

## Detalhes de Implementação

Ver techspec.md → "Princípio de Fronteira" (`internal/card`), "Contratos de Validação por Módulo"
(GAP-V1) e ADR-005. Não quebrar `card_repository` (que reconstrói `BillingCycle` de closing/due
persistidos). Mudança coesa no dono; não vaza regra para onboarding/agent.

## Critérios de Sucesso

- Criar cartão com apenas `closing_day` funciona (due derivado, datas de fatura corretas).
- Callers existentes que enviam `due_day` permanecem inalterados (sem regressão).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem). go-implementation (linguagem, auto) aplica-se.

## Testes da Tarefa

- [ ] Testes unitários (due opcional derivado; due explícito preservado; billing_cycle datas; ranges)
- [ ] Testes de integração (T12 — criação de cartão no onboarding com só closing_day)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Definition of Done (DoD)

- [ ] `DueDay` opcional + derivação interna; retrocompatível.
- [ ] `billing_cycle` calcula datas corretamente com e sem `due_day` explícito.
- [ ] Zero comentários no `.go` de produção; validação na fronteira (R-DTO-VALIDATE-001).
- [ ] `go build ./internal/card/...` e `go test ./internal/card/... -count=1` passam (incl. callers existentes).

## Critérios de Aceite (validações executáveis)

```bash
go build ./internal/card/... && go test ./internal/card/... -count=1
# due_day continua aceito quando informado (sem quebrar HTTP handler/daily)
go test ./internal/card/application/usecases/... -run CreateCard -count=1
```

## Arquivos Relevantes
- `internal/card/application/dtos/input/create_card.go` (DueDay opcional + Validate)
- `internal/card/domain/valueobjects/billing_cycle.go` (derivação quando ausente)
- `internal/card/application/usecases/create_card.go`
- `internal/card/infrastructure/repositories/postgres/card_repository.go` (verificar reconstrução do cycle)
