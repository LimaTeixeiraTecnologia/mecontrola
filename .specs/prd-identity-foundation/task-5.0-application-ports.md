# Tarefa 5.0: Ports `UserRepository` e `IDGenerator` em `application/interfaces`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Declarar as duas interfaces consumidas pelos use cases (e implementadas pela infrastructure): `UserRepository` (operações canônicas sobre o agregado `User`) e `IDGenerator` (port para gerar UUIDs v4 determinístico em testes). Pacote `internal/identity/application/interfaces/`. Sem implementação. Sem mock — mocks são gerados pela mockery na task 6.0.

<requirements>
- RF-11: `UserRepository` expõe exatamente as 5 operações canônicas: `UpsertByWhatsAppNumber`, `FindByID`, `FindByWhatsAppNumber`, `SoftDelete`, `LinkNewNumber`.
- `IDGenerator` expõe `NewUserID() string` (retorna UUID v4 cru — caller passa para `entities.NewUserID` ou usa em `user_whatsapp_history.id`).
- Pacote `interfaces` importa apenas `context`, `internal/identity/domain/entities` e `internal/identity/domain/valueobjects`.
- Nenhum tipo concreto vazado: `tx`, `pgx`, `sql.Conn` proibidos na assinatura (transacionalidade encapsulada no adapter — ADR-010).
</requirements>

## Subtarefas

- [ ] 5.1 Criar `internal/identity/application/interfaces/user_repository.go` com `type UserRepository interface { ... }` cobrindo as 5 operações com assinaturas exatas conforme techspec.
- [ ] 5.2 Criar `internal/identity/application/interfaces/id_generator.go` com `type IDGenerator interface { NewUserID() string }`.
- [ ] 5.3 Adicionar godoc curto em cada interface declarando: contrato, regra de soft delete (filtro em leituras), comportamento esperado de erros (`ErrUserNotFound` quando applicable).

## Detalhes de Implementação

Ver techspec §"Interfaces Chave" subseções `application/interfaces/user_repository.go` e `application/interfaces/id_generator.go`. ADR-010 justifica por que `LinkNewNumber` e `SoftDelete` não recebem `tx` como parâmetro.

## Critérios de Sucesso

- `UserRepository` tem exatamente 5 métodos. Nenhum método retorna tipo de infraestrutura (pgx, sql).
- `IDGenerator` tem 1 método sem parâmetros.
- `depguard` valida que `application/interfaces` não importa `infrastructure` nem `platform` nem `configs`.
- Pacote compila sem dependência circular.

## Definition of Done (DoD)

- [ ] `go build ./internal/identity/application/interfaces/...` passa.
- [ ] `go vet ./internal/identity/application/interfaces/...` passa.
- [ ] `golangci-lint run ./internal/identity/application/interfaces/...` passa.
- [ ] `grep -rn 'pgx\|sql\.\|database/sql' internal/identity/application/interfaces/` retorna vazio.
- [ ] Godoc legível: `go doc ./internal/identity/application/interfaces UserRepository` mostra as 5 assinaturas.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Nenhum teste direto (interface sem implementação). Validação por compile + lint + dep boundary.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/identity/application/interfaces/user_repository.go` (novo)
- `internal/identity/application/interfaces/id_generator.go` (novo)
