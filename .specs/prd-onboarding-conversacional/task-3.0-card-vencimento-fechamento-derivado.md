# Tarefa 3.0: Módulo card — vencimento + fechamento derivado

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Permitir a criação de cartão a partir do onboarding coletando apenas o vencimento, com o fechamento derivado, preservando a competência e o contrato HTTP público.

<requirements>
- `card.CreateCard` aceita `DueDay` obrigatório no caminho do onboarding e `ClosingDay` derivado; manter `DueDay *int` no DTO público sem quebrar a API HTTP existente (ADR-003, RF-08).
- Consumer `onboarding.card_registered` cria cartão com `DueDay` (coletado) + `ClosingDay` (derivado por `DeriveClosingDay`, offset configurável `AGENT_ONBOARDING_CARD_CLOSING_OFFSET_DAYS`, default 10) (RF-10).
- Idempotência por `event_id` no consumer; sem segunda criação em replay (RF-27, RF-28).
- Consumer fino `adapter → usecase` (R-ADAPTER-001), sem SQL/branching de domínio.
</requirements>

## Subtarefas

- [ ] 3.1 Ajustar validação/contrato de `card.CreateCard` para o caminho de vencimento (sem quebrar HTTP).
- [ ] 3.2 Offset configurável (config + default documentado).
- [ ] 3.3 Consumer `onboarding.card_registered` → `CreateCard` com DueDay + ClosingDay derivado; idempotência por `event_id`.

## Detalhes de Implementação

Ver `techspec.md` → "Pontos de Integração", "Riscos R2" e ADR-003. A derivação (`DeriveClosingDay`) vem da 1.0; aqui é o seam onboarding→card.

## Critérios de Sucesso

- Cartão criado via onboarding tem `DueDay` = informado e `ClosingDay` = derivado coerente.
- API HTTP de `card` inalterada para callers externos.
- Replay do evento não duplica cartão.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: `CreateCard` (DueDay obrigatório no caminho onboarding, ClosingDay derivado); consumer idempotente (testify/suite, mocks por IIFE).
- [ ] Testes de integração — cobertos na 9.0 (propagação do evento com Postgres).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/card/application/dtos/input/create_card.go`
- `internal/card/application/usecases/create_card.go`
- `internal/card/infrastructure/messaging/database/consumers/onboarding_card_consumer.go`
- `configs/` (offset de fechamento)
