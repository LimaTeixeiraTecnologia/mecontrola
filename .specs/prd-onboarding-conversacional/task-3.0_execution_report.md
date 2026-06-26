# Generated: 2026-06-26T13:44:02Z

# Relatório de Execução — Tarefa 3.0: Módulo card — vencimento + fechamento derivado

## Status

`done`

## Resumo

Ajustou o seam `onboarding.card_registered → internal/card` para criar cartões com `DueDay` obrigatório (coletado no onboarding) e `ClosingDay` derivado por offset configurável, preservando o contrato HTTP público e garantindo idempotência por `event_id` no consumer.

## Arquivos Alterados

- `internal/card/infrastructure/messaging/database/consumers/onboarding_card_consumer.go`
- `internal/card/infrastructure/messaging/database/consumers/onboarding_card_consumer_test.go`
- `internal/card/infrastructure/messaging/database/consumers/onboarding_card_consumer_integration_test.go`
- `internal/card/module.go`
- `.env.example`

## Requisitos Funcionais Cobertos

- **RF-08:** Cartão no onboarding coleta apenas apelido + dia de vencimento; `ClosingDay` é derivado, nunca perguntado.
- **RF-10:** Consumer cria cartão com `DueDay` + `ClosingDay` derivado; evento `onboarding.card_registered` idempotente.
- **RF-27/RF-28:** Propagação por domain event; idempotência por `event_id` no consumer.

## Mudanças Implementadas

### 3.1 — Contrato/validação do caminho de vencimento

- O consumer exige `DueDay` entre 1 e 31 (`p.DueDay < 1 || p.DueDay > 31`) antes de chamar `CreateCard`.
- A obrigatoriedade do vencimento fica no seam do onboarding (consumer), sem alterar o DTO público `input.CreateCard`, que mantém `DueDay *int` para callers HTTP.
- O `ClosingDay` continua sendo passado no evento (já derivado em `internal/onboarding` pela tarefa 2.0) e repassado ao use case.

### 3.2 — Offset configurável

- O offset de fechamento (`ONBOARDING_CARD_CLOSING_OFFSET_DAYS`) já estava mapeado em `configs.OnboardingConfig.CardClosingOffsetDays` com default `10` no carregador de config.
- Documentado em `.env.example` com comentário referenciando ADR-003.
- Também documentado `AGENT_ONBOARDING_CARD_CLOSING_OFFSET_DAYS=10` na seção do agent/LLM.

### 3.3 — Consumer idempotente

- `OnboardingCardConsumer` recebe `idempotency.Storage`.
- Antes de processar, consulta `idempotency_keys` pela chave composta:
  - `scope = "event:onboarding.card_registered"`
  - `key = envelope.ID` (`event_id`)
  - `userID = aggregate_user_id`
- Se registro existir, retorna `nil` sem executar o use case (replay sem efeito).
- Se não existir, injeta `IdempotencyContext` no contexto com `RequestHash = event_id`, de modo que `CreateCard` persista o registro de idempotência após inserção bem-sucedida (dentro da mesma transação UoW).
- TTL do registro: `30 * 24 * time.Hour`.

## Testes

### Comandos executados

```bash
go test -race -count=1 ./internal/card/infrastructure/messaging/database/consumers/... ./internal/card/application/usecases/... ./internal/card/...
```

Saída:

```text
ok  	github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/messaging/database/consumers
ok  	github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases
ok  	github.com/LimaTeixeiraTecnologia/mecontrola/internal/card
ok  	github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input
...
```

(todos os pacotes de `internal/card/...` passaram)

```bash
go test -race -count=1 ./internal/onboarding/application/usecases/... ./internal/onboarding/domain/...
```

Saída:

```text
ok  	github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases
ok  	github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities
ok  	github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/services
ok  	github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects
```

```bash
golangci-lint run ./internal/card/... ./configs/...
```

Saída:

```text
0 issues.
```

### Novos/alterados testes unitários

- `TestHappyPath_CallsExecuteWithCorrectFields`: verifica que `DueDay` e `ClosingDay` chegam corretamente ao use case e que o contexto de idempotência é propagado.
- `TestMissingDueDay_ReturnsError`: substitui o teste anterior que mapeava `DueDay=0` para `nil`; agora o consumer rejeita vencimento ausente.
- `TestReplay_SkipsCreate`: simula replay do mesmo `event_id`; o segundo handle não chama o use case.
- Testes de integração (`onboarding_card_consumer_integration_test.go`) atualizados para receber `idempotency.Storage`.

## Gates de Governança

Executados os gates obrigatórios do runbook:

```bash
# zero comentários em Go de produção
OK
# kernel genérico (sem domínio)
OK
# sem SQL direto nem LLM no kernel
OK
# switch de domínio não cresce em daily_ledger_agent.go
OK
# sem SQL em tools/workflow do agent
OK
```

## Build

- `go build ./internal/card/...` — OK (compilação e testes dos pacotes afetados).
- `go build ./...` — **falha pré-existente** em `internal/agent/application/workflow/onboarding_steps_*.go` (task 4.0 ainda pendente, arquivos novos com import faltante de `valueobjects`). A task 3.0 não alterou esses arquivos; a falha é externa ao escopo e será resolvida pela task 4.0.

## Riscos Residuais

1. **Build global temporariamente quebrado:** arquivos da task 4.0 (`internal/agent/application/workflow/*`) impedem `go build ./...`. O build do escopo da task 3.0 (`internal/card/...`) está verde.
2. **Idempotência do consumer depende da tabela `mecontrola.idempotency_keys`:** o storage é injetado pelo módulo `card` com `idempotency.NewPostgresStorage(db)`, consistente com o uso HTTP existente.
3. **Race mínima em replay concorrente:** se duas instâncias processarem o mesmo evento simultaneamente antes do primeiro `Put`, ambas podem tentar inserir; a constraint única de `idempotency_keys` e/ou de `cards(nickname, user_id)` garante não-duplicação, com retry do outbox.

## Suposições

- A derivação `DeriveClosingDay(dueDay, offset)` continua implementada em `internal/onboarding/domain/services/card_closing.go` (tarefa 1.0) e o payload do evento já entrega `ClosingDay` derivado.
- O use case `CreateCard` já persistia registro de idempotência quando `IdempotencyContext` estava presente; o consumer passou a fornecer esse contexto e a fazer o pre-check.
- O offset de fechamento já estava disponível via `configs.OnboardingConfig.CardClosingOffsetDays`; a tarefa apenas documentou a variável de ambiente.

## Critérios de Sucesso

- [x] Cartão criado via onboarding tem `DueDay` = informado e `ClosingDay` = derivado coerente.
- [x] API HTTP de `card` inalterada para callers externos.
- [x] Replay do evento não duplica cartão.
