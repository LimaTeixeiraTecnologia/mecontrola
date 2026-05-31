# Tarefa 8.0: Esqueletos dos 6 módulos de domínio + depguard fronteiras hexagonais

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Materializar os **6 módulos de domínio** previstos no discovery — `identity`, `conversation`, `agent`, `finance`, `notifications`, `telemetry` — com layout hexagonal `internal/<modulo>/{domain,application,adapters}` (vazios; somente `doc.go` declarando intenção + README com **scaffold pattern** que os PRDs subsequentes seguirão). Configurar **`depguard` em `.golangci.yml`** enforçando as 5 regras de fronteira hexagonal listadas no techspec. Cobre **RF-09** parcial (lado dos módulos).

<requirements>
- Para cada um dos 6 módulos, criar `internal/<modulo>/{domain,application,adapters}/doc.go` com declaração `// Package <subpacote> ...` explicando responsabilidade.
- README em `internal/<modulo>/README.md` (cada um) com o **scaffold pattern**: como declarar aggregate, entity, VO, port `Repository`, port de domain event, use case em application.
- Atualizar `.golangci.yml` com regra `depguard` enforçando:
  1. `internal/<modulo>/domain` NÃO importa `internal/<modulo>/application` nem `internal/<modulo>/adapters`.
  2. `internal/<modulo>/application` NÃO importa `internal/<modulo>/adapters`.
  3. `internal/<modulo>/domain` NÃO importa `internal/infrastructure/*` (exceto `internal/infrastructure/errors` se necessário, caso a caso).
  4. `internal/<modulo>/domain` NÃO importa `configs/*` nem `github.com/spf13/viper`.
  5. Cross-module: `internal/identity/*` NÃO importa `internal/finance/*` (e demais combinações cross — só via port `application`).
- Teste de import inválido: criar arquivo `*_test.go` propositalmente violador em uma feature branch local e validar que `golangci-lint run` bloqueia.
</requirements>

## Subtarefas

- [ ] 8.1 Criar estrutura `internal/identity/{domain,application,adapters}/doc.go` + `README.md`.
- [ ] 8.2 Replicar estrutura para `conversation`, `agent`, `finance`, `notifications`, `telemetry`.
- [ ] 8.3 Cada `doc.go` declara responsabilidade do módulo conforme discovery (e.g. `identity` = "usuário, sessão, JWT/refresh, RBAC, audit de acesso").
- [ ] 8.4 Cada `README.md` documenta o **scaffold pattern** canônico com snippet pseudo-Go: interface `AggregateRoot`, exemplo de VO, port `Repository`, port `EventPublisher`, use case com `UnitOfWork[T]`.
- [ ] 8.5 Atualizar `.golangci.yml` com regra `depguard` cobrindo as 5 regras listadas.
- [ ] 8.6 Criar `internal/identity/domain/_depguard_check_test.go` (ou similar; pode ficar dentro do test de outro pacote) com test estilo `//go:build depguard_test` que tenta importar `internal/identity/adapters` e valida que `golangci-lint` bloqueia (test de meta, executado em CI).
- [ ] 8.7 Documentar em `AGENTS.md` (no folder de cada módulo) os comandos `ai-spec` recomendados quando criar agregados nos PRDs subsequentes.

## Detalhes de Implementação

Ver techspec §"Fronteiras entre Application, Domain e Infrastructure" + §"Modelagem de Domínio" + ADR-001.

## Critérios de Sucesso

- `find internal -type d` mostra `internal/<modulo>/{domain,application,adapters}` para os 6 módulos.
- `cat internal/identity/domain/doc.go` (e demais) mostra `// Package domain` + responsabilidade.
- `golangci-lint run ./...` passa (sem violações iniciais).
- Tentativa proposital de violação (PR de teste): `internal/identity/application/foo.go` importando `internal/finance/adapters` é bloqueada por `depguard`.
- `cat internal/identity/README.md` (e demais) cobre scaffold pattern com 4 seções: Aggregate, VO, Repository, Use Case.
- Cobre RF-09 (lado dos módulos).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: n/a (pacotes vazios; testes virão nos PRDs subsequentes).
- [ ] Testes de integração: lint test — execução de `golangci-lint run ./...` com regras `depguard` ativas; teste manual de import inválido confirmado bloqueado em CI da task 10.0.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/identity/{domain,application,adapters}/doc.go`
- `internal/identity/README.md`
- `internal/conversation/{domain,application,adapters}/doc.go`
- `internal/conversation/README.md`
- `internal/agent/{domain,application,adapters}/doc.go`
- `internal/agent/README.md`
- `internal/finance/{domain,application,adapters}/doc.go`
- `internal/finance/README.md`
- `internal/notifications/{domain,application,adapters}/doc.go`
- `internal/notifications/README.md`
- `internal/telemetry/{domain,application,adapters}/doc.go`
- `internal/telemetry/README.md`
- `.golangci.yml` (regra `depguard` expandida)
