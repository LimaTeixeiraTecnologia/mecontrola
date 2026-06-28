# Tarefa 4.0: Implementar Advisory Lock em cmd/migrate

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar um advisory lock no PostgreSQL no comando `migrate`, garantindo que apenas uma instância do serviço `migrate` execute migrations simultaneamente.

<requirements>
- Cobrir RF-25: advisory lock para execução única de migrations.
</requirements>

## Subtarefas

- [ ] 4.1 Adicionar função `acquireMigrationLock` em `cmd/migrate/migrate.go`.
- [ ] 4.2 Usar `pg_try_advisory_lock($1)` com ID fixo (ex.: 424242).
- [ ] 4.3 Garantir release do lock via `pg_advisory_unlock` no defer.
- [ ] 4.4 Retornar erro claro se lock não puder ser adquirido.
- [ ] 4.5 Adicionar testes unitários/integração para o advisory lock.
- [ ] 4.6 Validar que dois `migrate` simultâneos resultam em erro na segunda instância.

## Detalhes de Implementação

Ver seção "6. Migrations — Advisory Lock" de `techspec.md`. O lock deve ser adquirido antes de `migrator.Up()` e liberado ao final.

```go
const migrationAdvisoryLockID int64 = 424242

func acquireMigrationLock(ctx context.Context, db *sql.DB) (func(), error) {
    var acquired bool
    err := db.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", migrationAdvisoryLockID).Scan(&acquired)
    if err != nil {
        return nil, fmt.Errorf("advisory lock query: %w", err)
    }
    if !acquired {
        return nil, fmt.Errorf("outro processo de migrate esta em execucao")
    }
    return func() {
        _, _ = db.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", migrationAdvisoryLockID)
    }, nil
}
```

## Critérios de Sucesso

- Dois `docker run` do migrate simultâneos: um sucede, o outro retorna erro de lock.
- Lock é liberado ao final da execução (validar via `pg_locks`).
- `go test ./cmd/migrate/...` passa.
- Migrations são aplicadas corretamente quando não há concorrência.

## Skills Necessárias

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Teste unitário para `acquireMigrationLock` com mock de banco.
- [ ] Teste de integração com PostgreSQL real validando concorrência.
- [ ] Teste de regressão: migrations ainda funcionam normalmente.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `cmd/migrate/migrate.go`
- `cmd/migrate/migrate_test.go`
