# Tarefa 1.0: Migrations Postgres + invariantes (users + user_whatsapp_history)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar duas migrations SQL via `golang-migrate` (já presente em `cmd/migrate`) materializando as tabelas `users` e `user_whatsapp_history` com as invariantes e índices definidos em ADR-007. Schema é pré-requisito para a implementação Postgres (tarefa 6.0) e para os testes de integração.

<requirements>
- RF-06: coluna `deleted_at TIMESTAMPTZ NULL`, coluna `status TEXT NOT NULL DEFAULT 'ACTIVE'`, CHECK `status IN ('ACTIVE','DELETED')`, CHECK `(status = 'DELETED') = (deleted_at IS NOT NULL)`.
- RF-07: DDL deve preparar filtro `deleted_at IS NULL` (índices parciais).
- RF-08: UNIQUE parcial em `whatsapp_number` quando `deleted_at IS NULL`; UNIQUE parcial em `email` quando `email IS NOT NULL AND deleted_at IS NULL`.
- RF-09: tabela `user_whatsapp_history` com colunas mínimas (`id`, `user_id`, `number`, `active`, `linked_at`, `unlinked_at`, `reason`), FK `user_id → users(id) ON DELETE CASCADE`, índices `(user_id, active)` e `(number)`.
- RF-16: numeração contínua a partir do último arquivo em `migrations/`. Próximo número livre: `000002` (anterior é `000001_outbox_events`).
- Nomes exatos das constraints devem casar com os mapeamentos de erro em 6.0 (`users_whatsapp_number_active_uniq_idx`, `users_email_active_uniq_idx`).
</requirements>

## Subtarefas

- [ ] 1.1 Criar `migrations/000002_identity_users.up.sql` conforme ADR-007 (CREATE TABLE `users` + 2 CHECKs + 3 índices parciais).
- [ ] 1.2 Criar `migrations/000002_identity_users.down.sql` com `DROP INDEX` na ordem inversa + `DROP TABLE users`.
- [ ] 1.3 Criar `migrations/000003_identity_user_whatsapp_history.up.sql` (CREATE TABLE + FK CASCADE + 2 índices).
- [ ] 1.4 Criar `migrations/000003_identity_user_whatsapp_history.down.sql`.
- [ ] 1.5 Aplicar as migrations em Postgres local (ou testcontainer ad-hoc) e validar que `\d users` mostra os CHECKs e índices nomeados corretamente.
- [ ] 1.6 Garantir que `migrations/embed.go` continua expondo as novas migrations (deve funcionar automaticamente — apenas validar build).

## Detalhes de Implementação

Referenciar:
- [`techspec.md` §11](./techspec.md) — resumo das migrations e nomes exatos de constraints/índices.
- [ADR-007](./adr-007-postgres-partial-unique-indexes.md) — SQL completo + alternativas rejeitadas.

**Constraints obrigatórias (nomes exatos — working tree é fonte da verdade após bugfix C-2):**

- `users_pkey` (PRIMARY KEY)
- `users_status_check` (CHECK)
- `users_status_deleted_at_check` (CHECK)
- `users_whatsapp_number_active_uniq_idx` (UNIQUE INDEX parcial — consumido pelo mapping de erro em 6.0)
- `users_whatsapp_number_deleted_idx` (INDEX auxiliar para reanimação)
- `users_email_active_uniq_idx` (UNIQUE INDEX parcial — consumido pelo mapping de erro em 6.0)
- `user_whatsapp_history_pkey` (PRIMARY KEY)
- `user_whatsapp_history_user_id_fkey` (FOREIGN KEY ON DELETE CASCADE)
- `user_whatsapp_history_active_unlinked_at_check` (CHECK invariante `active = TRUE ⇔ unlinked_at IS NULL`)
- `user_whatsapp_history_user_active_idx`
- `user_whatsapp_history_number_idx`

## Critérios de Sucesso

- `migrations/000002_identity_users.{up,down}.sql` e `migrations/000003_identity_user_whatsapp_history.{up,down}.sql` presentes.
- Aplicar `up.sql` em Postgres limpo cria tabelas/índices com os nomes exatos acima.
- Tentativa de `INSERT INTO users (status, deleted_at, ...) VALUES ('DELETED', NULL, ...)` é rejeitada pelo CHECK `users_status_deleted_at_check` (cobre CA-04(h) do PRD em camada de DDL).
- Aplicar `down.sql` deixa o schema limpo (sem objetos residuais).
- `go build ./...` continua verde (embed.FS atualizado automaticamente).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff). -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Aplicação manual de up/down em Postgres local valida ausência de objetos residuais.
- [ ] Tentativa de violar invariante `status='DELETED' AND deleted_at IS NULL` via SQL direto retorna erro `users_status_deleted_at_check`.
- [ ] Tentativa de inserir dois usuários vivos com mesmo `whatsapp_number` viola `users_whatsapp_number_active_uniq_idx`.
- [ ] Inserção de usuário com `deleted_at IS NOT NULL` permite inserir outro vivo com o mesmo número (índice é parcial).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `migrations/000002_identity_users.up.sql` (criar)
- `migrations/000002_identity_users.down.sql` (criar)
- `migrations/000003_identity_user_whatsapp_history.up.sql` (criar)
- `migrations/000003_identity_user_whatsapp_history.down.sql` (criar)
- `migrations/embed.go` (validar, sem editar)
- Referência: `migrations/000001_outbox_events.up.sql` (padrão de estilo)
