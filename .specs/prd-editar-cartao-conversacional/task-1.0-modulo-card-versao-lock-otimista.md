# Tarefa 1.0: Módulo card — expor versão e lock otimista atômico

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechar o gap de versão e introduzir lock otimista real e atômico no módulo `internal/card`, de forma aditiva e compatível com o endpoint REST existente. Base de todo o caminho de edição conversacional (ADR-002).

<requirements>
- Expor `Version` na saída do cartão sem quebrar contrato existente.
- Adicionar `ExpectedVersion` opcional ao update; quando presente, aplicar lock otimista atômico.
- Adicionar `ClosingDay` opcional ao update e honrá-lo em `resolveUpdate` (RF-17: banco não reconhecido usa o fechamento informado, não o fallback).
- Introduzir erro de domínio dedicado para conflito de versão.
- Manter o comportamento do endpoint REST idêntico quando `ExpectedVersion`/`ClosingDay` não são informados.
- Cobre RF-06, RF-16, RF-17, RF-18, RF-20, RF-23, RF-27, RF-28.
</requirements>

## Subtarefas

- [ ] 1.1 Adicionar sentinela `ErrCardVersionConflict` em `internal/card/domain/errors.go`.
- [ ] 1.2 Adicionar `Version int64` em `internal/card/application/dtos/output/card.go` e mapear em `internal/card/application/mappers/card_mapper.go` (`ToCardOutput`).
- [ ] 1.3 Adicionar `ExpectedVersion *int64` e `ClosingDay *int` em `internal/card/application/dtos/input/update_card.go` (ambos opcionais; `ClosingDay` habilita a persistência do fechamento informado para banco não reconhecido — RF-17; `Validate()` valida `ClosingDay` em 1..31 quando presente).
- [ ] 1.4 Alterar `internal/card/application/interfaces/repository.go` e o adapter Postgres `card_repository.go`: `UpdateByIDForUser(ctx, c, expectedVersion *int64)`; quando `expectedVersion != nil`, incluir `AND version = $x`; em 0 linhas afetadas, desambiguar `ErrCardVersionConflict` vs `ErrCardNotFound` por verificação de existência.
- [ ] 1.5 Alterar `internal/card/application/usecases/update_card.go`: após carregar o cartão corrente, se `ExpectedVersion != nil && corrente.Version != *ExpectedVersion` retornar `ErrCardVersionConflict`; propagar `expectedVersion` ao repositório. Preservar recálculo de fechamento ao mudar banco/vencimento e persistência do novo `due_day`.
- [ ] 1.6 Em `resolveUpdate` (mesmo arquivo): quando `in.ClosingDay != nil`, construir o ciclo diretamente via `NewBillingCycle(*in.ClosingDay, dueDay)` **sem** chamar `BankDaysReader.DaysBeforeDue` (espelha `create_card.go:91-106`); esse branch tem precedência sobre a derivação. Sem ele, o fechamento informado para banco não reconhecido é silenciosamente substituído pelo `fallbackDaysBeforeDue` (RF-17 não atendido).
- [ ] 1.7 Regenerar mocks (`.mockery.yml` + `task mocks`) para os contratos alterados.

## Detalhes de Implementação

Ver `techspec.md` seções "Interfaces Chave", "Modelos de Dados" e ADR-002. Sem migration nova (coluna `version` já existe na migration 000001).

## Critérios de Sucesso

- `ExpectedVersion` divergente resulta em `ErrCardVersionConflict`; igual grava e incrementa versão; nil mantém o comportamento atual do REST.
- Lock aplicado atomicamente no `UPDATE` (sem janela TOCTOU).
- `gofmt`/`go vet`/`golangci-lint`/`go test -race` verdes no módulo `internal/card`.
- Zero comentários em `.go` de produção (R-ADAPTER-001.1); SQL apenas no adapter Postgres.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `domain-modeling-production` — introduz erro de domínio `ErrCardVersionConflict` e invariante de concorrência (state/version); modelagem de erro de negócio explícita.
- `postgresql-production-standards` — lock otimista atômico via `AND version = $x`, desambiguação de 0 linhas e robustez de transação no repositório Postgres.

## Testes da Tarefa

- [ ] Testes unitários: use case `update_card` (ExpectedVersion nil/igual/divergente); `resolveUpdate` com `ClosingDay` informado usa o valor (não o fallback) para banco não reconhecido; mapper propaga `Version`.
- [ ] Testes de integração: repositório `UpdateByIDForUser` com testcontainers Postgres — conflito de versão (0 linhas → `ErrCardVersionConflict`), not-found, cartão soft-deleted → `ErrCardNotFound` (RF-28), caminho nil inalterado.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/card/domain/errors.go`
- `internal/card/application/dtos/output/card.go`
- `internal/card/application/mappers/card_mapper.go`
- `internal/card/application/dtos/input/update_card.go`
- `internal/card/application/interfaces/repository.go`
- `internal/card/application/usecases/update_card.go`
- `internal/card/infrastructure/repositories/postgres/card_repository.go`
- `.mockery.yml` e mocks gerados
