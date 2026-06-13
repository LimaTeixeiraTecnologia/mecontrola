# Tarefa 5.0: Use case RecordGatewayAuthFailure (outbox auth.failed)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Cria use case `RecordGatewayAuthFailure` que publica evento `auth.failed` no outbox com `reason="gateway_*"`, `request_id` e `client_ip` populados. Reutiliza o contrato e o consumer `ProjectAuthEvent` do `prd-auth-foundation`. Sem regra de negócio nova — apenas serialização + outbox publish + métrica.

<requirements>
- RF-08: evento `auth.failed` publicado para cada falha de gateway com um dos 4 `reason` novos
- Reuso do contrato outbox existente (`outbox.Event`) sem alteração do envelope
- `aggregate_user_id` (quando padrão futuro for adotado) fica na payload JSON; este PRD não introduz campo no envelope (escopo de tarefa #9 separada)
- Idempotência: `event_id` único por chamada
- Zero comentário em `.go`
- Sem nova dep em `go.mod`
</requirements>

## Subtarefas

- [ ] 5.1 Criar DTO `internal/identity/application/dtos/input/record_gateway_auth_failure.go` com `UserIDRaw string`, `Reason string`, `RequestID string`, `ClientIPRaw string`.
- [ ] 5.2 Criar use case `internal/identity/application/usecases/record_gateway_auth_failure.go` que recebe DTO, valida `Reason` ∈ {`gateway_missing_header`, `gateway_invalid_timestamp`, `gateway_stale_timestamp`, `gateway_invalid_signature`}, monta `entities.AuthEvent` com `Kind=auth.failed` e os 4 campos forensics, publica via outbox.
- [ ] 5.3 Reutilizar o `outbox.Publisher` injetado (mesmo padrão dos use cases existentes do `prd-auth-foundation`).
- [ ] 5.4 `event_id` gerado por `uuid.NewV7()` (ou padrão do repo) **no use case** — não em `Decide*` (esses são puros).
- [ ] 5.5 `time.Now().UTC()` inline no use case (regra de memória — sem `Clock`).
- [ ] 5.6 Teste unitário com mock de outbox publisher: assert que evento foi publicado com `reason`, `request_id`, `client_ip` corretos; assert que `event_id` é único entre chamadas.

## Detalhes de Implementação

Ver techspec seção "Pontos de Integração > Outbox + Auth Events". O consumer `ProjectAuthEvent` do `prd-auth-foundation` já mapeia o evento para a tabela; **valide** durante execução que ele lê os novos campos do payload (se não, ampliar nesta tarefa).

## Critérios de Sucesso

- `go test ./internal/identity/application/usecases/... -run "RecordGatewayAuthFailure" -v` PASS.
- Mock do outbox recebe payload com os 4 campos esperados.
- 4 reasons aceitos; qualquer outra string retorna erro.
- `grep` R-ADAPTER-001.1 sobre os novos arquivos retorna vazio.
- `task mocks` regenera sem erro.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Teste unitário do use case com mock outbox
- [ ] Teste de borda: reason inválida retorna erro tipado
- [ ] Teste de idempotência: duas chamadas geram event_ids diferentes

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/identity/application/dtos/input/record_gateway_auth_failure.go` (novo)
- `internal/identity/application/usecases/record_gateway_auth_failure.go` (novo)
- `internal/identity/application/usecases/record_gateway_auth_failure_test.go` (novo)
- `internal/identity/application/interfaces/mocks/` (regenerado por `task mocks`)
