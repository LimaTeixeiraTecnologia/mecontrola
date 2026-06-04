# ADR-001 — `golang-migrate` + `embed.FS` para schema do módulo identity

## Metadados

- **Título:** Adoção de `golang-migrate` com `embed.FS` para migrations de `users` e `user_whatsapp_history`
- **Data:** 2026-06-03
- **Status:** Aceita
- **Decisores:** Engenharia de plataforma + autor do PRD identity
- **Relacionados:** PRD `.specs/prd-identity-foundation/prd.md` (SQ-01, RT-01), techspec `./techspec.md`, código existente `internal/platform/database/migrations.go`

## Contexto

O PRD declara em SQ-01 que a ferramenta de migration Postgres seria decidida na techspec entre `golang-migrate`, `goose` e `atlas`. O codebase já adota `golang-migrate v4.19.1` (declarada como dependência direta em `go.mod`), usa `embed.FS` em `migrations/embed.go` para carregar arquivos `*.sql` no binário, e expõe a função `database.RunMigrations` que aplica versões pendentes via `iofs.New(migrations.FS, ".")`. As migrations `0001_init` e `0002_outbox` seguem a convenção `NNNN_descricao.{up,down}.sql`. Trocar de ferramenta agora exigiria reescrever toda a fundação e regenerar testes de integração.

## Decisão

Adotar `golang-migrate/v4` com `iofs.FS` embedded como ferramenta única e exclusiva de migrations no projeto. As migrations do identity entram como `0003_identity.{up,down}.sql` e `0004_identity_admin_seed.{up,down}.sql` no diretório `migrations/` raiz. A função `database.RunMigrations` já existente é o ponto único de aplicação — não criar caminho paralelo. Cada migration tem par `up`/`down` reversível.

## Alternativas Consideradas

- **`goose`** — suporta migrations em Go (não só SQL) e transação automática. Vantagens: mais opinativo. Desvantagens: troca de ferramenta no meio do projeto, regenerar `0001`/`0002`, perder o pattern `iofs.FS` consolidado. Rejeitada por custo de migração desproporcional ao ganho.
- **`atlas`** — schema-as-code declarativo com diff automático. Vantagens: melhor para schemas grandes e em evolução constante. Desvantagens: curva de aprendizado, dependência externa não-Go, conflito com `embed.FS` existente. Rejeitada — sobre-engenharia para o tamanho atual do schema (3 tabelas até o fim de E1).
- **SQL puro aplicado fora do binário (psql + Makefile)** — Vantagens: zero código. Desvantagens: sem versionamento estruturado, sem rollback testável, contra `persistence.md` (R-PERSIST). Rejeitada.

## Consequências

### Benefícios Esperados

- Consistência total com o restante do projeto; zero retrabalho na fundação.
- Embedded FS garante que o binário não dependa de filesystem no deploy.
- Idempotência via `migrate.ErrNoChange` já tratada no helper.
- Down migrations testáveis em CI via testcontainers.

### Trade-offs e Custos

- Schema declarativo (`atlas`) ficaria mais ergonômico para evolução em larga escala — perde-se essa otimização futura.
- Cada alteração de schema exige par manual `up`/`down`.

### Riscos e Mitigações

- **Risco:** Down migration mal escrita corrompe estado em rollback.
- **Mitigação:** Integration test obrigatório aplica `up` + `down` + `up` em cada PR que adicione migration.
- **Rollback:** `migrate -path migrations -database "$DSN" down 2` reverte as duas migrations do identity.

## Plano de Implementação

1. Criar `migrations/0003_identity.up.sql` com schema `users` + `user_whatsapp_history` + índices.
2. Criar `migrations/0003_identity.down.sql` espelho.
3. Criar `migrations/0004_identity_admin_seed.up.sql` (DO block, ver ADR-005) e `down.sql` (no-op com `RAISE NOTICE`).
4. Validar localmente: `migrate -path migrations -database "$DSN" up && migrate down 2 && migrate up`.
5. Integration test executa o mesmo ciclo via testcontainers.

## Monitoramento e Validação

- CI gate: `go test -tags=integration ./internal/identity/...` passa.
- Sinal de falha: `database.ErrMigration` aparece em log de bootstrap.

## Impacto em Documentação e Operação

- `internal/identity/README.md` referencia esta ADR na seção "Migrations".
- Runbook `docs/runbooks/migrations.md` (se existir) ganha entrada para identity.

## Revisão Futura

Revisitar se o schema do projeto crescer além de ~30 tabelas com mudanças frequentes (>1/semana), ponto em que `atlas` passa a render mais.
