# ADR-007 — Índices parciais únicos em `users` (Postgres)

## Metadados

- **Título:** Estratégia de unicidade para `whatsapp_number` e `email` com soft delete
- **Data:** 2026-06-05
- **Status:** Aceita
- **Decisores:** Time MeControla (owner: Jailton Junior)
- **Relacionados:**
  - PRD: [`prd.md`](./prd.md) — RF-06, RF-07, RF-08, RF-08-ter, RF-16, Q-03
  - Tech Spec: [`techspec.md`](./techspec.md)
  - PRD Q em aberto fechada: **Q-03**
  - Decisões correlatas: [ADR-006](./adr-006-reanimation-window-constant.md), [ADR-004](./adr-004-typed-errors-application-package.md)

## Contexto

A tabela `users` precisa satisfazer simultaneamente:

1. **RF-06:** `status TEXT NOT NULL` em `{ACTIVE, DELETED}`, `deleted_at TIMESTAMPTZ NULL`, com invariante `status = 'DELETED' ⇔ deleted_at IS NOT NULL`.
2. **RF-07:** toda leitura filtra `WHERE deleted_at IS NULL`. Hard delete proibido.
3. **RF-08:** `UNIQUE` em `whatsapp_number` **apenas para linhas não deletadas**, e `UNIQUE` parcial em `email` quando `email IS NOT NULL`.
4. **RF-08-ter:** dentro da janela de 30 dias, reanimação é possível — então linhas com `deleted_at IS NOT NULL` **podem ser reaproveitadas**; `UpsertByWhatsAppNumber` precisa achar a linha soft-deletada para reanimar.

Conflito potencial:

- Se o índice único for `UNIQUE (whatsapp_number)` cobrindo tudo, reanimação dentro da janela falha em `INSERT ON CONFLICT` porque ainda existe a linha antiga.
- Se o índice ignorar linhas deletadas, um número pode aparecer duas vezes (uma ACTIVE, outra DELETED). Mas como **só pode haver uma linha por número em qualquer estado vivo**, e o lookup de reanimação faz `SELECT` antes do upsert, não há conflito.

Restrições adicionais:

- `golang-migrate` (RF-16) é a tool ativa; próximo número é `000002_`.
- Postgres suporta índices parciais nativamente.
- A consulta canônica para reanimação precisa achar **a linha soft-deletada com aquele número**, ainda que filtros padrão sejam `deleted_at IS NULL`.

## Decisão

A migration `000002_identity_users` cria a tabela `users` e os seguintes índices:

```sql
CREATE TABLE users (
    id              UUID         NOT NULL,
    whatsapp_number TEXT         NOT NULL,
    email           TEXT         NULL,
    display_name    TEXT         NULL,
    status          TEXT         NOT NULL DEFAULT 'ACTIVE',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ  NULL,

    CONSTRAINT users_pkey PRIMARY KEY (id),
    CONSTRAINT users_status_check CHECK (status IN ('ACTIVE','DELETED')),
    CONSTRAINT users_status_deleted_at_check
        CHECK ((status = 'DELETED') = (deleted_at IS NOT NULL))
);

-- Unicidade de whatsapp_number entre linhas vivas (não deletadas).
-- Permite reanimação: linha soft-deletada não bloqueia upsert; o use case
-- detecta a linha soft-deletada via SELECT separado antes do upsert e
-- decide entre reanimar (mesmo UUID) ou criar nova (novo UUID).
CREATE UNIQUE INDEX users_whatsapp_number_active_uniq_idx
    ON users (whatsapp_number)
    WHERE deleted_at IS NULL;

-- Lookup auxiliar para reanimação (apenas linhas deletadas).
-- Mantém p99 da consulta de reanimação previsível mesmo com tabela grande.
CREATE INDEX users_whatsapp_number_deleted_idx
    ON users (whatsapp_number)
    WHERE deleted_at IS NOT NULL;

-- Unicidade de email entre linhas vivas com email presente.
CREATE UNIQUE INDEX users_email_active_uniq_idx
    ON users (email)
    WHERE email IS NOT NULL AND deleted_at IS NULL;

-- Índice de leitura por id já é coberto pela PK.
```

E para `user_whatsapp_history`:

```sql
CREATE TABLE user_whatsapp_history (
    id           UUID         NOT NULL,
    user_id      UUID         NOT NULL,
    number       TEXT         NOT NULL,
    active       BOOLEAN      NOT NULL,
    linked_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    unlinked_at  TIMESTAMPTZ  NULL,
    reason       TEXT         NULL,

    CONSTRAINT user_whatsapp_history_pkey PRIMARY KEY (id),
    CONSTRAINT user_whatsapp_history_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT user_whatsapp_history_active_unlinked_at_check
        CHECK ((active = TRUE) = (unlinked_at IS NULL))
);

CREATE INDEX user_whatsapp_history_user_active_idx
    ON user_whatsapp_history (user_id, active);

CREATE INDEX user_whatsapp_history_number_idx
    ON user_whatsapp_history (number);
```

A migration `down` deve fazer o `DROP` ordenado.

## Alternativas Consideradas

### A) `UNIQUE (whatsapp_number)` total (sem `WHERE deleted_at IS NULL`)

- **Vantagens:** índice mais simples.
- **Desvantagens:**
  - Bloqueia reanimação no caminho `INSERT ... ON CONFLICT`: a linha soft-deletada conflita com a nova tentativa.
  - Força exclusão hard-delete antes de inserir — viola RF-07.
- **Motivo de não escolher:** incompatível com soft delete.

### B) `UNIQUE` em coluna calculada (`COALESCE(deleted_at, '<sentinela>')`)

- **Vantagens:** unicidade composta sem `WHERE`.
- **Desvantagens:**
  - Truque pouco idiomático.
  - Tornaria o índice difícil de raciocinar.
  - Sentinela arbitrária (`'9999-12-31'`?) traz risco semântico.
- **Motivo de não escolher:** Postgres oferece `WHERE` em índices parciais — usar o recurso nativo.

### C) Tabela separada `user_active` para unicidade

- **Vantagens:** isolamento físico.
- **Desvantagens:**
  - Duplica fonte de verdade.
  - Sincronização exige trigger ou transação cuidadosa.
- **Motivo de não escolher:** complexidade desproporcional.

### D) Não criar índice auxiliar para `whatsapp_number` deletado

- **Vantagens:** menos índices, menos custo de manutenção.
- **Desvantagens:**
  - Consulta de reanimação faz scan na linha deletada — em volume alto, fica lenta.
- **Motivo de não escolher:** custo de manutenção do índice é baixo; ganho de previsibilidade vale o tradeoff.

## Consequências

### Benefícios Esperados

- **Reanimação atômica e rápida:**
  1. `SELECT ... FROM users WHERE whatsapp_number = $1 AND deleted_at IS NOT NULL FOR UPDATE` (usa `users_whatsapp_number_deleted_idx`).
  2. Se encontrou, decidir reanimar ou criar (lógica em `application` usando `CanReanimate(now)` — ADR-006).
  3. Se reanimar: `UPDATE` (mesma linha; reset `deleted_at`, `status`, `email`, `display_name`).
  4. Se criar novo: `INSERT` (novo UUID); o índice único vivo aceita porque a linha velha está deletada.
- **Conflitos detectáveis:** violação de `users_whatsapp_number_active_uniq_idx` retorna `pgerrcode.UniqueViolation` → repository devolve `application.ErrWhatsAppNumberInUse` (ADR-004).
- **Invariante de domínio defendida no schema** (CHECK constraint).
- **Performance previsível** em queries de leitura (índice parcial é menor).

### Trade-offs e Custos

- **3 índices** sobre `users` (PK + 2 parciais) + 1 índice `users_whatsapp_number_deleted_idx`. Custo de escrita aumenta marginalmente.
- Migrations futuras que adicionem estado ao enum (`BLOCKED`, etc.) precisam revisar o CHECK constraint na mesma migration (S-08 do PRD reafirma).
- Reanimação não é `INSERT ON CONFLICT DO UPDATE` puro — requer `SELECT` prévio (ou `INSERT ... ON CONFLICT (whatsapp_number) WHERE deleted_at IS NULL DO UPDATE SET deleted_at = NULL, status = 'ACTIVE', email = ..., display_name = ...`, mas a forma com `WHERE` em `ON CONFLICT` constraint name requer índice parcial nomeado e tem semântica delicada). A techspec recomenda a forma transacional `SELECT FOR UPDATE` → branch → `INSERT` ou `UPDATE`.

### Riscos e Mitigações

- **Risco:** alteração futura do enum `status` quebrar CHECK silenciosamente.
  - **Mitigação:** PR que adicione novo estado deve atualizar o CHECK na mesma migration; revisão é responsável.
- **Risco:** consulta de reanimação esquecer o filtro `deleted_at IS NOT NULL` e usar índice errado.
  - **Mitigação:** repository centraliza a consulta; teste E2E (CA-04(d)/(e)) cobre.
- **Risco:** índice parcial não ser escolhido pelo planner.
  - **Mitigação:** consulta canônica casa exatamente com o `WHERE` do índice; `EXPLAIN` no smoke E2E valida.

## Plano de Implementação

1. Criar `migrations/000002_identity_users.up.sql` com `users`, índices parciais, CHECK constraint.
2. Criar `migrations/000002_identity_users.down.sql` com `DROP` ordenado.
3. Criar `migrations/000003_identity_user_whatsapp_history.up.sql` (separar para reduzir blast radius da migration).
4. Repository postgres usa transação:
   - `BeginTx`.
   - `SELECT ... FOR UPDATE` em ambos índices (vivo e deletado) — duas queries ou query composta.
   - Branch reanimar vs criar.
   - `Commit` ou `Rollback` com `errors.Join`.
5. Smoke E2E (CA-04 a–h) valida unicidade, reanimação dentro/fora da janela, CHECK constraint.

## Monitoramento e Validação

- **Validação imediata:** `make migrate-up && make migrate-down` (ou equivalente) executa sem erro.
- **EXPLAIN check** no smoke: `EXPLAIN SELECT ... FROM users WHERE whatsapp_number = '+5511...' AND deleted_at IS NULL` mostra `Index Scan using users_whatsapp_number_active_uniq_idx`.
- **Pg stats:** `pg_stat_user_indexes` em staging mostra `idx_scan > 0` para ambos os índices parciais após smoke.
- **Sinal de drift:** taxa de `pgerrcode.UniqueViolation` em `UpsertByWhatsAppNumber` acima do esperado indica race condition no `SELECT FOR UPDATE` (alerta operacional futuro).

## Impacto em Documentação e Operação

- `internal/identity/infrastructure/repositories/postgres/user_repository.go` documenta a sequência transacional no doc comment.
- Runbook LGPD (E4) menciona o índice deletado como ponto de busca para anonimização.
- README do módulo cita ADR-007 ao explicar soft delete.

## Revisão Futura

- Revisitar se enum `status` crescer (atualizar CHECK).
- Revisitar se o número de linhas `deleted_at IS NOT NULL` crescer demais antes de E4 entrar em produção (avaliar TTL operacional).
- Revisitar se Postgres planner deixar de escolher o índice parcial em queries específicas (`pg_hint_plan` ou refator de query).

## Histórico

- 2026-06-06: nomes de constraints/índices alinhados ao working tree real após bugfix C-2 (sufixo `_idx` nos UNIQUE INDEX, CHECK `users_status_deleted_at_check`, FK `user_whatsapp_history_user_id_fkey`). A versão anterior deste ADR usava nomenclatura sem `_idx`/`_pkey`/`_fkey` que nunca chegou a ser materializada na migration `000002`. Mapping de erro em `internal/identity/infrastructure/repositories/postgres/user_repository.go` casa com os nomes reais. Constraint adicional `user_whatsapp_history_active_unlinked_at_check` documentada explicitamente.
