# Tarefa 7.0: [onboarding] SaveOnboardingCard por closing_day

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Ajustar `SaveOnboardingCard` (e seu DTO de input) em `internal/onboarding` para coletar **dia de
fechamento** (`ClosingDay`) em vez de vencimento, propagando ao `SynchronousCardCreator`
(→ `internal/card`) e ao evento `onboarding.card_registered`. O cartão real é criado em
`internal/card` (não no onboarding) — apenas o draft de fluxo fica no payload.

<requirements>
- RF-12: coleta de cartão em uma mensagem, por dia de fechamento; aceita "Não uso".
- ADR-005 (closing_day), ADR-006 (cartão é domínio de `internal/card`; onboarding delega).
- R-DTO-VALIDATE-001: `SaveOnboardingCardInput.Validate()` com `errors.Join` e nome de campo.
</requirements>

## Subtarefas

- [ ] 7.1 `SaveOnboardingCardInput`: coletar `ClosingDay` (1..31) + `Nickname` (1..32) com `Validate()`.
- [ ] 7.2 Propagar `closing_day` + nickname (≤32) ao `SynchronousCardCreator` e ao evento `onboarding.card_registered`, **sem enviar `due_day`** (o `internal/card` deriva — Tarefa 13.0).
- [ ] 7.3 Testes unitários (suite testify: closing_day válido/ inválido; "Não uso"; delegação ao card creator mockado).

## Detalhes de Implementação

Ver techspec.md → "Cartão por dia de fechamento (DR-05)" e ADR-005. Não coletar limite/vencimento
(cartão skeleton). Não acessar persistência de `internal/card` diretamente — usar o seam
`SynchronousCardCreator` já injetado.

## Critérios de Sucesso

- `closing_day` fora de 1..31 é rejeitado na validação do DTO (re-pergunta no fluxo — EB-14).
- O cartão real é criado via `SynchronousCardCreator`/evento; onboarding não toca a tabela de cartões.
- **Contrato `internal/card` honrado (techspec "Contratos de Validação")**: `nickname` validado
  1..32 na fronteira (GAP-V2/EB-16). O onboarding envia **somente `closing_day`** (1..31); a
  derivação de `due_day` vive no `internal/card` (Tarefa 13.0 — GAP-V1/DR-11), não no onboarding.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem). go-implementation (linguagem, auto) aplica-se.

## Testes da Tarefa

- [ ] Testes unitários (DTO Validate; mapeamento ClosingDay; delegação mockada)
- [ ] Testes de integração (T12 — etapa de cartões e2e)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Definition of Done (DoD)

- [ ] `ClosingDay` coletado e propagado ao card creator + evento.
- [ ] DTO com `Validate()` (errors.Join, nome de campo); zero comentários no `.go`.
- [ ] `go build ./internal/onboarding/...` e `go test ./internal/onboarding/application/usecases/... -run SaveOnboardingCard` passam.

## Critérios de Aceite (validações executáveis)

```bash
go build ./internal/onboarding/... && \
go test ./internal/onboarding/application/usecases/... -run SaveOnboardingCard -count=1
# onboarding não importa persistência de internal/card diretamente
grep -rn "internal/card/infrastructure" internal/onboarding --include="*.go" | grep -v _test && echo FAIL || echo OK
```

## Arquivos Relevantes
- `internal/onboarding/application/usecases/save_onboarding_card.go` (modificado)
- `internal/onboarding/application/dtos/input/` (DTO `ClosingDay` + Validate)
- `internal/onboarding/domain/entities/onboarding_session.go` (`OnboardingCardDraft.ClosingDay`)
