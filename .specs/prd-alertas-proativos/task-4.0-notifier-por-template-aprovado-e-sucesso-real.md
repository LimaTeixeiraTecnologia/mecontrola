# Tarefa 4.0: Notifier por template aprovado e sucesso real

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Alterar o notifier de thresholds para usar templates aprovados por kind e marcar sucesso apenas depois do envio aceito pelo gateway.

<requirements>
- Cobrir RF-05, RF-08, RF-09, RF-11, RF-12, RF-18, REQ-13, REQ-14 e REQ-21.
- Template não aprovado, ausente ou sem opt-in deve suprimir envio com motivo auditável.
- Falha Meta não pode marcar `notified_at`.
- Texto livre atual não pode virar fallback de proativo fora da janela.
</requirements>

## Subtarefas

- [ ] 4.1 Mapear `NotifyThresholdAlert` e repositórios de estado atuais.
- [ ] 4.2 Resolver template por kind e status aprovado.
- [ ] 4.3 Enviar via `SendTemplate` para proativos fora da janela.
- [ ] 4.4 Mover marcação de notificado para depois do sucesso.
- [ ] 4.5 Cobrir falhas Meta, template pendente e canal ausente por testes.

## Detalhes de Implementação

Referenciar `techspec.md` seções `Pontos de Integração`, `Tratamento de erro` e `Configuração`.

## Critérios de Sucesso

- Nenhum envio falho é persistido como notificado.
- Template `PENDING` bloqueia envio real.
- Opt-in ausente bloqueia template `MARKETING`.
- O fluxo antigo de texto livre preserva comportamento onde ainda for permitido.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `domain-modeling-production` — valida estados de notificação, supressão e erros de negócio.
- `design-patterns-mandatory` — confirma adapter fino sem pattern formal indevido.

## Testes da Tarefa

- [ ] `go test -race -count=1 ./internal/budgets/application/usecases/...`
- [ ] `go test -race -count=1 ./internal/budgets/infrastructure/messaging/database/consumers/...`
- [ ] `go vet ./internal/budgets/application/usecases/... ./internal/budgets/infrastructure/messaging/database/consumers/...`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/budgets/application/usecases/notify_threshold_alert.go`
- `internal/budgets/application/usecases/notify_threshold_alert_test.go`
- `internal/budgets/infrastructure/messaging/database/consumers/threshold_alert_notifier_integration_test.go`
- `internal/budgets/infrastructure/messaging/database/producers/threshold_alert_publisher.go`
