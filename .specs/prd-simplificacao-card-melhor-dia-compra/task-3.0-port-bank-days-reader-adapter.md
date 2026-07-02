# Tarefa 3.0: Porta `BankDaysReader` + adapter `bank_repository` (fallback 7) + wiring

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Introduzir a porta consumidora `BankDaysReader` (resolve `days_before_due` de um `BankCode`, aplicando o
fallback de 7 dias quando o banco não está em `mecontrola.banks`) e seu adapter Postgres `bank_repository`,
plugado na `RepositoryFactory` do módulo. O fallback vive no adapter — o consumidor recebe sempre um `int`.

<requirements>
- RF-09: fallback de 7 dias no adapter quando o banco (normalizado) não existe em `banks`; nunca erro por "não encontrado".
- RF-10: leitura da tabela `banks` em runtime (lookup por `bank.LookupKey()`).
- Interface no consumidor (DMMF/R6.3): `BankDaysReader` em `application/interfaces/`; adapter em `infrastructure/repositories/postgres/`.
- Padrão de repositório: `database.DBTX`, `QueryRowContext`, `defer func(){ _ = rows.Close() }()` quando aplicável, span+logger.
</requirements>

## Subtarefas

- [ ] 3.1 Criar `application/interfaces/bank_days_reader.go`: `BankDaysReader.DaysBeforeDue(ctx, bank valueobjects.BankCode) (int, error)`.
- [ ] 3.2 Criar `infrastructure/repositories/postgres/bank_repository.go`: `SELECT days_before_due FROM mecontrola.banks WHERE code = $1` com `bank.LookupKey()`; `sql.ErrNoRows` → retorna 7 (fallback), nunca erro.
- [ ] 3.3 Registrar o método `BankDaysReader(db)` na `RepositoryFactory` (`application/interfaces/repository.go` + `infrastructure/repositories/factory.go`).
- [ ] 3.4 Gerar mock (`mockery`) da nova interface para uso nos use cases (tarefa 4.0).

## Detalhes de Implementação

Ver `techspec.md` §"Interfaces Chave" e §"Modelos de Dados"; ADR-001. Fallback = constante 7 no adapter.
Sem branching de domínio no adapter — só query + fallback + wrapping de erro. Query pode omitir prefixo de
schema (`banks`) conforme precedente `plan_repository.go`, ou usar `mecontrola.banks` (consistente com `cards`).

## Critérios de Sucesso

- Banco do seed → retorna `days_before_due` correto (ex.: `nubank` → 7, `itau` → 8).
- Banco fora do seed → retorna 7 (fallback), sem erro.
- Erro real de IO (não `ErrNoRows`) é propagado com `fmt.Errorf("...: %w", err)`.
- `RepositoryFactory` expõe `BankDaysReader(db)`; mock gerado.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: n/a (adapter fino; validado por integração).
- [ ] Testes de integração: `bank_repository_integration_test.go` — seed lido; código inexistente → 7.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/card/application/interfaces/bank_days_reader.go` (novo)
- `internal/card/infrastructure/repositories/postgres/bank_repository.go` (novo)
- `internal/card/application/interfaces/repository.go`, `infrastructure/repositories/factory.go` (editar)
- `internal/card/application/interfaces/mocks/` (mock novo)
