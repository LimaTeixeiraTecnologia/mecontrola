# ADR-010 — Transacionalidade encapsulada no repositório: `LinkNewNumber` e `SoftDelete` abrem UoW interna

## Metadados

- **Título:** `PgxUserRepository` recebe `*database.Manager` e abre `UnitOfWork[T]` interna em operações multi-statement
- **Data:** 2026-06-03
- **Status:** Aceita
- **Decisores:** Engenharia + autor do PRD
- **Relacionados:** PRD (RF-11, RF-18), techspec §Implementação Postgres, ADR-009 (cascata SoftDelete), código existente `internal/platform/database/uow.go`, `internal/platform/outbox/publisher.go`

## Contexto

Duas operações do `UserRepository` exigem múltiplas SQLs atômicas:

- `LinkNewNumber`: UPDATE history → INSERT history → UPDATE users (3 statements).
- `SoftDelete`: UPDATE users → UPDATE history (2 statements, conforme ADR-009).

Há três padrões possíveis:

1. **UoW interna no repositório** — `PgxUserRepository` recebe `*database.Manager` e abre `database.NewUnitOfWork[T](r.manager).Do(...)` internamente. Use case fica trivial (1 chamada).
2. **UoW no use case** — repositório expõe métodos finos (`DeactivateActiveHistory`, `InsertHistory`, `UpdateUserNumber`) e o `LinkNewNumberUseCase` orquestra com UoW externa.
3. **`tx` como parâmetro do port** — `LinkNewNumber(ctx, tx, ...)` (padrão do `outbox.Publisher.Publish(ctx, tx, evt)`). Use case abre UoW e passa tx.

O codebase mostra dois patterns coexistindo:
- `outbox.Publisher` usa (3): aceita `tx database.DBTX` porque é projetado para ser invocado por um use case maior que já tem UoW aberta envolvendo outras escritas (agregado + outbox event no mesmo commit).
- `database.UnitOfWork[T]` é genérico, projetado para ser composto por qualquer camada.

Para identity neste PRD, **não há outra escrita transacional fora do repositório** — `UpsertByWhatsAppNumber`, `SoftDelete`, `LinkNewNumber` são operações atômicas auto-contidas. Forçar (3) abriria espaço para erro (use case esquece de abrir UoW). Forçar (2) explode o port em 5+ métodos finos que só fazem sentido juntos. Decisão pelo padrão (1): encapsular UoW no repositório.

## Decisão

`PgxUserRepository` recebe `*database.Manager` (não `devkitmanager.Manager`) no construtor. Métodos multi-statement (`SoftDelete`, `LinkNewNumber`) abrem `database.NewUnitOfWork[struct{}](r.manager).Do(...)` internamente; o callback recebe `tx devkitdb.DBTX` e executa as SQLs. Métodos single-statement (`UpsertByWhatsAppNumber`, `FindByID`, `FindByWhatsAppNumber`) usam `r.manager.Inner().DBTX(ctx)` direto, sem UoW (leitura/INSERT único não justifica overhead transacional).

O port `interfaces.UserRepository` permanece estável (assinaturas em `application/interfaces/user_repository.go`); apenas o adapter conhece o detalhe transacional.

Quando, no futuro, um caso de uso precisar coordenar identity + outbox + outra escrita no mesmo commit (ex.: emitir `identity.user.created` via outbox no UPSERT), o port ganhará um método adicional `WithTx(ctx, tx, ...)` específico, ou um decorator transacional. **Fora de escopo deste PRD** (RT-07).

## Alternativas Consideradas

- **UoW no use case + repository com métodos finos** — Vantagens: explicita transacionalidade na camada application. Desvantagens: explode port em ~7 métodos onde 5 só fazem sentido em sequência; vaza detalhe SQL para application; testes do use case ficam complexos (mock de 5 métodos por cenário). Rejeitada — viola "repository expõe operações do domínio, não queries genéricas" (`persistence.md`).
- **`tx` como parâmetro do port (padrão outbox)** — Vantagens: composabilidade futura (identity + outbox no mesmo commit). Desvantagens: forçar uso de UoW pelo caller mesmo quando não há composição cross-module; sem caso real no MVP; mock fica complexo. Rejeitada para este PRD; reabrir quando outbox for adotado em identity (PRD futuro).
- **`sync.Mutex` em vez de tx** — Rejeitada de imediato: não cobre o problema (DB concorrente, múltiplas instâncias).

## Consequências

### Benefícios Esperados

- Use case fica trivial: `r.userRepository.LinkNewNumber(ctx, id, number, reason)`.
- Atomicidade impossível de esquecer (não há caminho que pule UoW).
- Port permanece simples (5 métodos, todos com semântica de operação de domínio).
- Mock de `UserRepository` continua ergonômico para testes do use case.

### Trade-offs e Custos

- Composição futura com outbox/eventos exigirá variant `WithTx` quando demanda real aparecer (não é problema agora — RT-07).
- Acoplamento entre `PgxUserRepository` e `*database.Manager` em vez do tipo mais abstrato `devkitmanager.Manager` — irrelevante já que o adapter é Postgres-específico.

### Riscos e Mitigações

- **Risco:** PR futuro precisar emitir evento outbox em `UpsertByWhatsAppNumber` e descobrir que o port não suporta `tx` externa.
- **Mitigação:** PRD futuro adiciona método `WithTx` ou decorator; mudança incremental sem quebrar contrato atual.
- **Risco:** UoW interna mascara timeout — caller passa ctx sem deadline e UoW aplica default 5s (já documentado em `database.UnitOfWork`).
- **Mitigação:** `database.UnitOfWork.Do` traduz `ctx.Err() == DeadlineExceeded` para `ErrDeadlineExceeded` — caller recebe sinal claro.

## Plano de Implementação

1. `NewPgxUserRepository(manager *database.Manager, idGenerator interfaces.IDGenerator, clock clock.Clock) *PgxUserRepository`.
2. `LinkNewNumber` e `SoftDelete` abrem `database.NewUnitOfWork[struct{}](r.manager).Do(ctx, func(ctx, tx) (struct{}, error) {...})`.
3. `UpsertByWhatsAppNumber`, `FindByID`, `FindByWhatsAppNumber` usam `r.manager.Inner().DBTX(ctx)` direto.
4. Integration test `TestLinkNewNumberRollbackEmFalha` injeta erro no segundo statement e valida rollback (sem rows novas em histórico, sem UPDATE em users).

## Monitoramento e Validação

- Métrica futura: histograma de duração de `SoftDelete` e `LinkNewNumber` (p99 esperado < 50ms).
- Log com `slog.Info("identity: link new number", slog.String("user_id", id.String()), slog.Duration("elapsed", ...))`.

## Impacto em Documentação e Operação

- `internal/identity/README.md` documenta que `UserRepository` é transacional sem que o caller precise abrir UoW.
- Quando outbox/eventos forem adotados em identity, esta ADR vira referência para a substituição.

## Revisão Futura

Reavaliar quando emergir caso de composição cross-module na mesma transação (ex.: PRD de outbox identity, ou billing precisando coordenar criação de user + subscription no mesmo commit).
