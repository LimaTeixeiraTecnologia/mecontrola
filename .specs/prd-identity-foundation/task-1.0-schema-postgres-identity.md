# Tarefa 1.0: Schema Postgres `0003_identity` + admin seed `0004_identity_admin_seed`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar o substrato persistente do módulo identity em duas migrations versionadas com `golang-migrate` + `embed.FS`: schema das tabelas `users` e `user_whatsapp_history` (`0003_identity`) e migration idempotente de promoção dos admins iniciais via `current_setting('app.admin_whatsapp_numbers')` (`0004_identity_admin_seed`). Aplicar a convenção oficial Postgres 2026 para nomes de constraints e índices: `pk_<tabela>`, `fk_<tabela>_<coluna>`, `uq_<tabela>_<coluna>`, `ck_<tabela>_<regra>`, `idx_<tabela>_<coluna(s)>`.

<requirements>
- RF-08: tabela `users` com colunas exatas exigidas pelo PRD.
- RF-09: índice único em `whatsapp_number` enforçado em nível de banco — implementado como UNIQUE parcial `WHERE deleted_at IS NULL` (ADR-006).
- RF-10: tabela `user_whatsapp_history` com FK CASCADE para `users(id)`.
- Migrations seguem padrão sequencial `NNNN_descricao.{up,down}.sql` em `migrations/` (consistente com `0001_init` e `0002_outbox`).
- Down migration reversível para ambas as versões.
- Helper `database.SetAdminWhatsAppNumbers(ctx, manager, csv)` em `internal/platform/database/admin_seed.go` valida formato E.164 BR e executa `ALTER DATABASE current_database() SET app.admin_whatsapp_numbers = $1`.
- Bootstrap do migrator chama `SetAdminWhatsAppNumbers` antes de `RunMigrations` quando `ADMIN_WHATSAPP_NUMBERS` está definido.
</requirements>

## Subtarefas

- [ ] 1.1 Criar `migrations/0003_identity.up.sql` com tabelas `users` e `user_whatsapp_history`, constraints `pk_users`, `pk_user_whatsapp_history`, `fk_user_whatsapp_history_user_id`, `ck_users_status`, e índices `uq_users_whatsapp_number`, `uq_users_email`, `idx_users_status`, `idx_user_whatsapp_history_user_id_active`, `idx_user_whatsapp_history_number`.
- [ ] 1.2 Criar `migrations/0003_identity.down.sql` revertendo na ordem inversa (drop indexes → drop user_whatsapp_history → drop indexes restantes → drop users).
- [ ] 1.3 Criar `migrations/0004_identity_admin_seed.up.sql` com `DO $$ ... $$` que lê `current_setting('app.admin_whatsapp_numbers', true)`, faz `string_to_array(',')` e executa `UPDATE users SET is_admin = true, updated_at = now() WHERE whatsapp_number = trim(nbr) AND deleted_at IS NULL`.
- [ ] 1.4 Criar `migrations/0004_identity_admin_seed.down.sql` (no-op com `RAISE NOTICE`).
- [ ] 1.5 Implementar `internal/platform/database/admin_seed.go` com `SetAdminWhatsAppNumbers(ctx context.Context, manager *Manager, csv string) error` validando regex `^\+55\d{11}$` e executando `ALTER DATABASE`.
- [ ] 1.6 Validar localmente via testcontainer simples: `migrate up && migrate down 2 && migrate up` sem erro.

## Detalhes de Implementação

Ver techspec §"Modelos de Dados" subseções `0003_identity.up.sql`/`down.sql` e `0004_identity_admin_seed.up.sql`; ADR-001 (`golang-migrate` + `embed.FS`), ADR-005 (admin seed) e ADR-006 (UNIQUE parcial). Tabela de convenções de nomenclatura Postgres está na techspec §"Schema Postgres".

## Critérios de Sucesso

- `0003_identity.up.sql` cria 2 tabelas com todas as colunas exigidas pelo RF-08 e RF-10.
- `pk_users`, `pk_user_whatsapp_history` aparecem em `pg_constraint` após `migrate up`.
- `fk_user_whatsapp_history_user_id` tem `ON DELETE CASCADE`.
- `ck_users_status` aceita apenas `'ACTIVE','BLOCKED','DELETED'`.
- `uq_users_whatsapp_number` é UNIQUE parcial `WHERE deleted_at IS NULL` (verificável em `pg_indexes.indexdef`).
- `down.sql` reverte em sequência sem violar FK.
- `database.SetAdminWhatsAppNumbers` rejeita CSV malformado (`+55119`, `5511988887777` sem `+`, vazio em token) com erro tipado.
- Migration `0004` é no-op silencioso (com `RAISE NOTICE`) quando `app.admin_whatsapp_numbers` não está setado.

## Definition of Done (DoD)

- [ ] `migrate -path migrations -database "$DSN" up` aplica 0003 e 0004 sem erro em Postgres limpo (testado em container 16-alpine).
- [ ] `migrate -path migrations -database "$DSN" down 2` reverte ambas sem erro.
- [ ] `migrate up` após `down 2` é idempotente (mesma estrutura final).
- [ ] `SELECT indexdef FROM pg_indexes WHERE indexname IN ('uq_users_whatsapp_number','uq_users_email')` retorna definições com `WHERE (deleted_at IS NULL)`.
- [ ] `SELECT conname FROM pg_constraint WHERE conrelid IN ('users'::regclass,'user_whatsapp_history'::regclass)` retorna exatamente `pk_users`, `pk_user_whatsapp_history`, `fk_user_whatsapp_history_user_id`, `ck_users_status`.
- [ ] Teste unitário de `SetAdminWhatsAppNumbers` rejeita 4 inputs malformados e aceita CSV válido (`+5511988887777,+5521977776666`).
- [ ] `go build ./...` passa.
- [ ] `golangci-lint run` passa.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unit: `SetAdminWhatsAppNumbers` table-driven (CSV vazio, válido, malformado, com espaços).
- [ ] Smoke local: `migrate up && migrate down 2 && migrate up` em container Postgres 16-alpine.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `migrations/0003_identity.up.sql` (novo)
- `migrations/0003_identity.down.sql` (novo)
- `migrations/0004_identity_admin_seed.up.sql` (novo)
- `migrations/0004_identity_admin_seed.down.sql` (novo)
- `internal/platform/database/admin_seed.go` (novo)
- `internal/platform/database/admin_seed_test.go` (novo)
- `migrations/embed.go` (sem alteração — `//go:embed *.sql` já cobre)
