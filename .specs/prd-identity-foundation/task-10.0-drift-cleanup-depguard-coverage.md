# Tarefa 10.0: Drift cleanup + depguard confirmação + mockery.yml + cobertura final

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Tarefa de gate final. Três entregáveis distintos mas coesos por validação cruzada de requisitos do PRD:

1. **Drift cleanup textual (RF-15):** reescrever `internal/identity/AGENTS.md`, `README.md`, `domain/doc.go`, `application/doc.go`, `infrastructure/doc.go` removendo toda menção a JWT/RBAC/audit de acesso e refletindo o escopo real (agregado `User`, VOs, port `UserRepository`, regra pura `IsEntitled`, soft delete, mascaramento PII).
2. **`depguard` confirmação (RF-16):** validar que `.golangci.yml` cobre as 3 regras canônicas para `internal/identity/*` (já existentes em grande parte) e adicionar o que faltar com escopo mínimo do PRD.
3. **Cobertura final (RF-17 consolidação + MS-01, MS-02, MS-03):** rodar `go test -cover` agregado, validar grep contra `JWT|RBAC|role` em `internal/identity/**/*.go`, validar `mockery --dry-run` verde.

<requirements>
- RF-15: zero menção a `JWT`, `RBAC` ou `role` em `internal/identity/**/*.go` exceto em comentário histórico explícito (`// removed: RBAC out of scope`).
- RF-16: `.golangci.yml` proíbe (a) `domain` importar `application`/`infrastructure`, (b) `application` importar `infrastructure`, (c) `infrastructure` importar diretamente `internal/billing/*`, `internal/onboarding/*`, `internal/finance/*`.
- RF-17 (consolidação): cobertura agregada em VOs (`NewWhatsAppNumber`, `NewEmail`) e `IsEntitled` ≥ 100%.
- MS-01: `go test -cover` reporta 100% nos pontos críticos.
- MS-02: `golangci-lint run` verde, sem violação `depguard` em `internal/identity/`.
- MS-03: `grep -rn 'JWT\|RBAC\|role' internal/identity/**/*.go` retorna apenas comentários históricos ou ausência total.
- `mockery.yml` declara `UserRepository` e `IDGenerator` (sincronizado com task 6.0).
- `internal/identity/AGENTS.md` documenta a convenção `_masked` para logging de PII (ADR-004) e a regra "`RehydrateUser` é restrito ao mapper" (ADR-008).
</requirements>

## Subtarefas

- [ ] 10.1 Reescrever `internal/identity/domain/doc.go` removendo "JWT/refresh, RBAC e audit de acesso" e declarando: "agregado `User`, Value Objects (`WhatsAppNumber`, `Email`, `UserStatus`), domain services (`EntitlementChecker`, contrato `Subscription`)".
- [ ] 10.2 Reescrever `internal/identity/application/doc.go` declarando "use cases finos (`UpsertUserByWhatsAppNumberUseCase` etc.) e ports (`UserRepository`, `IDGenerator`)".
- [ ] 10.3 Reescrever `internal/identity/infrastructure/doc.go` declarando "adapter Postgres (`PgxUserRepository`), gerador UUID v4 (`UUIDGenerator`), mapper de row Postgres".
- [ ] 10.4 Reescrever `internal/identity/README.md` com: visão de responsabilidade (sem RBAC), tabela de Value Objects, seção sobre `IsEntitled`, convenção `_masked` para logging, regra de `RehydrateUser` exclusivo do mapper, fronteiras `depguard` em tabela, referência ao bundle `consolidacao-core` e à techspec.
- [ ] 10.5 Reescrever `internal/identity/AGENTS.md` com: papel do módulo, contratos (`UserRepository`, `IDGenerator`), regra de logging PII (`mask.WhatsApp`/`mask.Email` + chave `_masked`), ADR-008 RehydrateUser, referência ao PRD e techspec.
- [ ] 10.6 Auditar `.golangci.yml`: confirmar `domain-no-infrastructure`, `application-no-infrastructure`, regras cross-module `identity-no-finance`, `finance-no-identity`. Caso o PRD exija explicitamente `identity-no-billing` ou `identity-no-onboarding` (RF-16), adicionar as deny rules apropriadas com escopo mínimo.
- [ ] 10.7 Validar `mockery.yml` lista `UserRepository` e `IDGenerator`; rodar `mockery --config mockery.yml --dry-run` esperando zero diff.
- [ ] 10.8 Validar com greps:
  - `grep -rnE '\b(JWT|RBAC|role)\b' internal/identity/**/*.go | grep -v '// removed'` retorna vazio.
  - `golangci-lint run` passa.
- [ ] 10.9 Rodar cobertura agregada: `go test -coverprofile=cover.out ./internal/identity/...` e validar via `go tool cover -func=cover.out | grep -E 'NewWhatsAppNumber|NewEmail|IsEntitled'` reporta 100%.
- [ ] 10.10 (Opcional) Adicionar receita `test:coverage:identity` no `Taskfile.yml` para reproducibilidade.

## Detalhes de Implementação

Ver PRD F-07 (drift removal), techspec §"Conformidade com Padrões". ADR-004 documenta a convenção `_masked`. ADR-008 documenta `RehydrateUser`. AGENTS.md raiz é referência canônica para o tom dos novos textos.

## Critérios de Sucesso

- 5 arquivos textuais reescritos sem menção a JWT/RBAC.
- `golangci-lint run` verde end-to-end (todo o repo).
- `mockery --dry-run` zero diff.
- Cobertura agregada em pontos críticos atinge 100%.
- README do módulo é navegável por novo dev sem cruzar com PRD.

## Definition of Done (DoD)

- [ ] `grep -rnE '\bJWT\b|\bRBAC\b|\brole\b' internal/identity/ --include='*.go'` (excluindo `// removed`) retorna vazio.
- [ ] `grep -rnE '\bJWT\b|\bRBAC\b' internal/identity/AGENTS.md internal/identity/README.md` retorna vazio.
- [ ] `golangci-lint run` passa em todo o repo.
- [ ] `mockery --config mockery.yml --dry-run` reporta zero diff.
- [ ] `go test -coverprofile=cover.out ./internal/identity/...` + `go tool cover -func=cover.out` mostra 100% em `NewWhatsAppNumber`, `NewEmail`, `IsEntitled`.
- [ ] `go test -tags=integration -race ./internal/identity/...` passa (regressão consolidada).
- [ ] `ai-spec check-spec-drift .specs/prd-identity-foundation/tasks.md` retorna verde (spec-hashes sincronizados após cada edit anterior).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Cobertura agregada (`go test -cover`).
- [ ] Greps de drift e depguard.
- [ ] `mockery --dry-run`.
- [ ] Regressão da suite completa (`go test ./... && go test -tags=integration ./...`).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/identity/domain/doc.go` (alterado)
- `internal/identity/application/doc.go` (alterado)
- `internal/identity/infrastructure/doc.go` (alterado)
- `internal/identity/README.md` (alterado)
- `internal/identity/AGENTS.md` (alterado)
- `.golangci.yml` (alterado se necessário)
- `mockery.yml` (validado, sem alteração se task 6.0 deixou correto)
- `Taskfile.yml` (alterado opcional)
