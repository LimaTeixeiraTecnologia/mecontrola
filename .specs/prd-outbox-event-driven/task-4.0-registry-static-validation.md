# Tarefa 4.0: Registry estático com validação de duplicidade

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar o `outbox.Registry` como contrato imutável após bootstrap: `map[events.EventName][]Subscription` populado uma vez no `cmd/worker`, validado no startup quanto a duplicidade `(SubscriptionName, EventType)` (D-09) e consumido por Publisher (para descobrir subscriptions) e Dispatcher (para hidratar `Handler`). Sem cabos soltos: qualquer inconsistência precisa derrubar o startup explicitamente (`ErrDuplicateSubscription`).

<requirements>
- RF-06: Registry estático com `Register(Subscription)` no bootstrap declarando `Name`, `EventType`, `Handler`.
- RF-07: validação no startup que duas subscriptions com o mesmo `(SubscriptionName, EventType)` falham com erro explícito.
- RF-08: cardinalidade 1×N — mesmo `EventType` pode ter múltiplos handlers; cada um com ciclo de vida independente.
- D-09: chave de unicidade é o par `(Name, EventType)`, permitindo reuso do mesmo `Name` em event types distintos.
</requirements>

## Subtarefas

- [ ] 4.1 Criar `registry.go` definindo `Registry` (interface) e `staticRegistry` (struct concreto) com método `NewRegistry() Registry`.
- [ ] 4.2 Implementar `Register(s Subscription) error` que valida `(Name, EventType)` único e empurra a `Subscription` em `map[events.EventName][]Subscription` interno.
- [ ] 4.3 Implementar `SubscriptionsFor(t events.EventName) []Subscription` retornando cópia defensiva (slice nova) para impedir mutação externa.
- [ ] 4.4 Implementar `Validate() error` chamado no `Subsystem.Start` que percorre todas as subscriptions garantindo: (a) `Handler` não nil; (b) `Name` válida (delega `SubscriptionName.Validate` se aplicável); (c) `EventType` aceito por `events.EventName`.
- [ ] 4.5 Gerar mock via `task mocks` em `internal/infrastructure/outbox/mocks/registry.go`.
- [ ] 4.6 Criar `registry_test.go` com `testify/suite` + table-driven.

## Detalhes de Implementação

Ver techspec.md seções **Arquitetura do Sistema → Componentes**, **Design de Implementação → Interfaces Chave** (assinatura da interface `Registry`) e **Estratégia de Erros** (`ErrDuplicateSubscription`).

## Critérios de Sucesso

- `go test ./internal/infrastructure/outbox/...` verde para o registry.
- Cenário 1: `Register` da mesma `Subscription` 2× retorna `ErrDuplicateSubscription`.
- Cenário 2: `Register` com mesmo `Name` mas `EventType` diferente é permitido (D-09).
- Cenário 3: `SubscriptionsFor(unknown)` retorna `nil` ou slice vazia (consistente; documentar a escolha em godoc).
- Cenário 4: `SubscriptionsFor(t)` retornado não compartilha backing array com a interna (mutação externa não corrompe estado).
- Cenário 5: `Validate()` falha se algum `Handler` for `nil`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: `registry_test.go` cobrindo os 5 cenários acima + duplicidade detectada em diferentes ordens de registro.
- [ ] Testes de integração: não aplicável (estrutura pura em memória).

**Definition of Done**:
- [ ] `Registry.Register` é a única forma de mutação; ausência de `Set`/`Delete`/`Unregister` exportados.
- [ ] `ErrDuplicateSubscription` exportado e cobertos por `errors.Is`.
- [ ] Mock gerado e callable em testes do Publisher (task 5.0).
- [ ] Cobertura ≥ 95% no `registry.go`.
- [ ] `gofmt -w .` aplicado; `golangci-lint run` verde para o arquivo.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/infrastructure/outbox/registry.go` (novo)
- `internal/infrastructure/outbox/registry_test.go` (novo)
- `internal/infrastructure/outbox/mocks/registry.go` (gerado por mockery)
- `internal/infrastructure/outbox/errors.go` (consumido — criado em 2.0)
- `internal/infrastructure/outbox/subscription.go` (consumido — criado em 2.0)
