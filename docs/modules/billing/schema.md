# Schema — internal/billing

> Schema PostgreSQL no search_path `mecontrola`. Todas as tabelas prefixadas com `billing_`.
> Migration de referência: `migrations/000001_initial_schema.up.sql` (seção Billing).

## Visão Geral

| Tabela | Propósito | Característica de Carga |
|--------|-----------|------------------------|
| `billing_plans` | Catálogo estático de planos (MONTHLY/QUARTERLY/ANNUAL) com mapeamento para produto Kiwify | Baixo volume; 3 linhas em produção |
| `billing_subscriptions` | Estado corrente de cada assinatura; sofre UPDATE frequente a cada evento Kiwify | Alto volume; update-heavy; `fillfactor=80` |
| `billing_processed_events` | Log de idempotência por `event_key`; evita duplo-processamento de webhooks | Alto volume; insert-heavy |
| `billing_kiwify_events` | Arquivo raw de todos os webhooks recebidos (JSONB) para auditoria e reprocessamento | Alto volume; purga periódica via TTL; `fillfactor=85` |
| `billing_reconciliation_checkpoints` | Watermark de progresso do job de reconciliação; uma linha por tarefa nomeada | Mínimo; poucas linhas |

---

## billing_subscriptions

### DDL

```sql
CREATE TABLE IF NOT EXISTS mecontrola.billing_subscriptions (
    id                     UUID        NOT NULL,
    funnel_token           TEXT        NOT NULL,
    user_id                UUID        NULL,
    kiwify_order_id        TEXT        NOT NULL,
    kiwify_subscription_id TEXT        NULL,
    plan_code              TEXT        NOT NULL,
    status                 TEXT        NOT NULL,
    period_start           TIMESTAMPTZ NOT NULL,
    period_end             TIMESTAMPTZ NOT NULL,
    grace_end              TIMESTAMPTZ NULL,
    last_event_at          TIMESTAMPTZ NOT NULL,
    customer_mobile_e164   TEXT        NULL,
    customer_email         TEXT        NULL,
    external_sale_id       TEXT        NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT billing_subscriptions_pkey        PRIMARY KEY (id),
    CONSTRAINT billing_subscriptions_status_check
        CHECK (status IN ('TRIALING', 'ACTIVE', 'PAST_DUE', 'CANCELED_PENDING', 'EXPIRED', 'REFUNDED')),
    CONSTRAINT billing_subscriptions_plan_code_fkey
        FOREIGN KEY (plan_code) REFERENCES mecontrola.billing_plans (code),
    CONSTRAINT billing_subscriptions_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES mecontrola.users(id) ON DELETE RESTRICT
) WITH (fillfactor = 80);
```

### Colunas

| Coluna | Tipo | Nullable | Default | Propósito | Invariante |
|---|---|---|---|---|---|
| `id` | UUID | NOT NULL | gerado na inserção | Chave primária da assinatura | Único; imutável |
| `funnel_token` | TEXT | NOT NULL | — | Token do funil de vendas da compra | Obrigatório; capturado no `sale_approved` |
| `user_id` | UUID | NULL | — | FK para `users.id`; nulo até o onboarding ser completado | Vinculado via `BindUser` |
| `kiwify_order_id` | TEXT | NOT NULL | — | ID do pedido na Kiwify; chave de idempotência para upsert | Único |
| `kiwify_subscription_id` | TEXT | NULL | — | ID da assinatura recorrente na Kiwify | Preenchido na primeira vez; preservado via `COALESCE` no upsert |
| `plan_code` | TEXT | NOT NULL | — | FK para `billing_plans.code` | Imutável após criação |
| `status` | TEXT | NOT NULL | — | Estado corrente do ciclo de vida | Restrito ao enum: `TRIALING`, `ACTIVE`, `PAST_DUE`, `CANCELED_PENDING`, `EXPIRED`, `REFUNDED` |
| `period_start` | TIMESTAMPTZ | NOT NULL | — | Início do período de acesso vigente | Obrigatório quando `status = 'ACTIVE'` |
| `period_end` | TIMESTAMPTZ | NOT NULL | — | Fim do período de acesso vigente | Atualizado em renovações via `ExtendPeriod` |
| `grace_end` | TIMESTAMPTZ | NULL | — | Prazo de graça para pagamento em atraso; nulo quando não em `PAST_DUE` | Zerado (`NULL`) ao renovar com sucesso |
| `last_event_at` | TIMESTAMPTZ | NOT NULL | — | Timestamp do último evento Kiwify processado | Monotonicamente crescente para o mesmo pedido/assinatura |
| `customer_mobile_e164` | TEXT | NULL | — | Telefone do comprador em formato E.164 | Preservado via `COALESCE` no upsert |
| `customer_email` | TEXT | NULL | — | E-mail do comprador | Preservado via `COALESCE` no upsert |
| `external_sale_id` | TEXT | NULL | — | ID de venda externo | Preservado via `COALESCE` no upsert |
| `created_at` | TIMESTAMPTZ | NOT NULL | `now()` | Data de criação do registro | Imutável |
| `updated_at` | TIMESTAMPTZ | NOT NULL | `now()` | Data da última modificação | Atualizado em todo `UPDATE` |

### Índices

| Nome | Colunas | Condição WHERE | Query Servida |
|---|---|---|---|
| `billing_subscriptions_pkey` (PK) | `id` | — | Lookup por ID interno |
| `billing_subscriptions_user_active_uniq_idx` (UNIQUE parcial) | `user_id` | `user_id IS NOT NULL AND status IN ('ACTIVE', 'PAST_DUE', 'CANCELED_PENDING')` | Garante no máximo uma assinatura ativa por usuário |
| `billing_subscriptions_kiwify_order_uniq_idx` (UNIQUE) | `kiwify_order_id` | — | `FindByOrderID`; chave de idempotência do upsert |
| `billing_subscriptions_funnel_token_idx` | `funnel_token` | — | Consultas analíticas por funil de vendas |
| `billing_subscriptions_external_sale_id_idx` | `external_sale_id` | `external_sale_id IS NOT NULL` | Rastreamento por ID externo; filtra nulos |

### Constraints Notáveis

**`billing_subscriptions_user_active_uniq_idx` — Invariante de unicidade de assinatura ativa**

Este índice único parcial é a principal salvaguarda contra race conditions em eventos concorrentes de ativação. Garante que, em qualquer momento, um `user_id` não nulo possua no máximo **uma** assinatura em estado operacional (`ACTIVE`, `PAST_DUE` ou `CANCELED_PENDING`). A condição `WHERE user_id IS NOT NULL` exclui deliberadamente assinaturas não vinculadas a usuário (estado pré-onboarding).

Quando a violação ocorre (Postgres `23505` / `UniqueViolation` na constraint), o repositório traduz para o sentinel tipado `ErrConcurrentActiveSub`.

**`billing_subscriptions_user_id_fkey ON DELETE RESTRICT`**

Impede a remoção de um usuário que possua assinatura vinculada. Garante integridade referencial.

### Operações do Repositório

| Método | SQL (resumido) | Observações |
|---|---|---|
| `FindByOrderID` | `SELECT ... WHERE kiwify_order_id = $1` | Retorna `ErrSubscriptionNotFound` via `sql.ErrNoRows` |
| `FindByKiwifySubID` | `SELECT ... WHERE kiwify_subscription_id = $1 ORDER BY last_event_at DESC LIMIT 1` | Retorna o registro mais recente |
| `FindByUserID` | `SELECT ... WHERE user_id = $1 AND status IN ('ACTIVE', 'PAST_DUE', 'CANCELED_PENDING') LIMIT 1` | Retorna a assinatura operacional do usuário |
| `UpsertByOrder` | `INSERT ... ON CONFLICT (kiwify_order_id) DO UPDATE SET status, period_end, grace_end, last_event_at, kiwify_subscription_id = COALESCE(...)` | Campos de cadastro preservados via `COALESCE`; valida `PeriodStart` obrigatório para `ACTIVE` |
| `ExtendPeriod` | `UPDATE ... SET period_end=$1, last_event_at=$2, grace_end=NULL, status='ACTIVE' WHERE id=$3` | Zera `grace_end` e força `status=ACTIVE` |
| `ApplyTransition` | `UPDATE ... SET status=$1, grace_end=$2, last_event_at=$3 WHERE id=$4` | Detecta conflito de `user_active_uniq_idx` → `ErrConcurrentActiveSub` |
| `BindUser` | `UPDATE ... SET user_id=$1 WHERE id=$2` | Vincula assinatura pré-existente ao usuário recém-criado |
| `ListPastDueGraceExpired` | `SELECT id, user_id, grace_end, last_event_at WHERE status='PAST_DUE' AND grace_end < $1 ORDER BY grace_end ASC LIMIT $2` | Alimenta o job de expiração de graça; limite padrão = 100 |

---

## billing_plans

### DDL

```sql
CREATE TABLE IF NOT EXISTS mecontrola.billing_plans (
    kiwify_product_id TEXT    NOT NULL,
    code              TEXT    NOT NULL,
    duration_days     INTEGER NOT NULL,
    currency          TEXT    NOT NULL DEFAULT 'BRL',

    CONSTRAINT billing_plans_pkey                PRIMARY KEY (kiwify_product_id),
    CONSTRAINT billing_plans_code_uniq           UNIQUE (code),
    CONSTRAINT billing_plans_code_check          CHECK (code IN ('MONTHLY', 'QUARTERLY', 'ANNUAL')),
    CONSTRAINT billing_plans_duration_days_check CHECK (duration_days > 0),
    CONSTRAINT billing_plans_currency_check      CHECK (currency <> '')
);
```

### Colunas

| Coluna | Tipo | Nullable | Default | Propósito | Invariante |
|---|---|---|---|---|---|
| `kiwify_product_id` | TEXT | NOT NULL | — | ID do produto na Kiwify; PK para lookup por produto | Único; imutável após configuração |
| `code` | TEXT | NOT NULL | — | Código canônico do plano no domínio | Único; restrito ao enum `MONTHLY`, `QUARTERLY`, `ANNUAL` |
| `duration_days` | INTEGER | NOT NULL | — | Duração do período de acesso em dias | Estritamente positivo (`> 0`) |
| `currency` | TEXT | NOT NULL | `'BRL'` | Moeda do plano | Não-vazia |

### Operações do Repositório

| Método | SQL (resumido) | Observações |
|---|---|---|
| `FindByKiwifyProductID` | `SELECT code, duration_days WHERE kiwify_product_id = $1` | Retorna `ErrPlanNotFound` quando não localizado |
| `FindByCode` | `SELECT code, duration_days WHERE code = $1` | Aceita `valueobjects.PlanCode` |
| `ConfigureProductIDs` | `UPDATE billing_plans SET kiwify_product_id = CASE code WHEN 'MONTHLY' THEN $1 ... END WHERE code IN (...)` | Exige exatamente 3 linhas afetadas |

---

## billing_processed_events

### DDL

```sql
CREATE TABLE IF NOT EXISTS mecontrola.billing_processed_events (
    event_key   TEXT        NOT NULL,
    trigger     TEXT        NOT NULL,
    recurso_id  TEXT        NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    status      TEXT        NOT NULL,

    CONSTRAINT billing_processed_events_pkey         PRIMARY KEY (event_key),
    CONSTRAINT billing_processed_events_status_check CHECK (status IN ('applied', 'superseded'))
);

CREATE INDEX IF NOT EXISTS billing_processed_events_recurso_idx
    ON mecontrola.billing_processed_events (recurso_id);
```

### Colunas

| Coluna | Tipo | Nullable | Default | Propósito | Invariante |
|---|---|---|---|---|---|
| `event_key` | TEXT | NOT NULL | — | Chave composta de idempotência derivada pelo use case | PK; inserção falha com `UniqueViolation` se o evento já foi processado |
| `trigger` | TEXT | NOT NULL | — | Nome do tipo de evento Kiwify que originou o processamento | Auditoria e filtragem |
| `recurso_id` | TEXT | NOT NULL | — | Identificador do recurso de negócio afetado (ex.: `order_id`) | Indexado para consultas por recurso |
| `occurred_at` | TIMESTAMPTZ | NOT NULL | — | Timestamp original do evento na Kiwify | Preserva a linha do tempo de domínio |
| `applied_at` | TIMESTAMPTZ | NOT NULL | `now()` | Momento em que o evento foi processado | Auditoria de latência |
| `status` | TEXT | NOT NULL | — | Estado atual do registro de idempotência | Restrito ao enum `applied` ou `superseded` |

### Semântica de status: applied vs superseded

- **`applied`**: o evento foi processado com sucesso. Estado inicial de todo registro inserido via `MarkApplied`.
- **`superseded`**: o efeito deste evento foi sobrescrito por um evento posterior de maior precedência. A transição é feita via `MarkSuperseded` e nunca é revertida. O registro permanece para auditoria completa.

### Operações do Repositório

| Método | SQL (resumido) | Observações |
|---|---|---|
| `MarkApplied` | `INSERT INTO ... (event_key, trigger, recurso_id, occurred_at, status) VALUES (...,'applied')` | Falha com `ErrEventAlreadyProcessed` em `UniqueViolation` na PK |
| `MarkSuperseded` | `UPDATE ... SET status='superseded' WHERE event_key=$1` | Operação silenciosa se a chave não existir |

---

## billing_kiwify_events

### DDL

```sql
CREATE TABLE IF NOT EXISTS mecontrola.billing_kiwify_events (
    envelope_id      TEXT        NOT NULL,
    trigger          TEXT        NOT NULL,
    raw_body         JSONB       NOT NULL,
    received_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at     TIMESTAMPTZ NULL,
    signature_status TEXT        NOT NULL,

    CONSTRAINT billing_kiwify_events_pkey PRIMARY KEY (envelope_id),
    CONSTRAINT billing_kiwify_events_signature_status_check
        CHECK (signature_status IN ('valid', 'invalid', 'rotated'))
) WITH (fillfactor = 85);
```

### Colunas

| Coluna | Tipo | Nullable | Default | Propósito | Invariante |
|---|---|---|---|---|---|
| `envelope_id` | TEXT | NOT NULL | — | Identificador único do envelope do webhook Kiwify | PK; duplicatas descartadas via `ON CONFLICT DO NOTHING` |
| `trigger` | TEXT | NOT NULL | — | Tipo de evento declarado pela Kiwify | Indexado para filtragem analítica |
| `raw_body` | JSONB | NOT NULL | — | Corpo integral do webhook como JSONB para auditoria | Imutável após inserção |
| `received_at` | TIMESTAMPTZ | NOT NULL | `now()` | Momento de chegada ao handler HTTP | Base para TTL de purga via `DeleteOlderThan` |
| `processed_at` | TIMESTAMPTZ | NULL | — | Momento em que o processamento de domínio foi concluído | Nulo indica evento recebido mas ainda não processado |
| `signature_status` | TEXT | NOT NULL | — | Resultado da verificação HMAC | Restrito ao enum `valid`, `invalid`, `rotated` |

### Semântica de signature_status: valid | invalid | rotated

- **`valid`**: assinatura HMAC-SHA1 corresponde ao segredo ativo. Eventos com este status são elegíveis para processamento.
- **`invalid`**: assinatura não corresponde a nenhum segredo. Evento é gravado para auditoria mas **não** gera efeito de domínio.
- **`rotated`**: assinatura corresponde a um segredo anterior (rotacionado). Permite auditoria durante janelas de rotação de chave.

O `fillfactor = 85` foi escolhido porque esta tabela recebe `UPDATE` de `processed_at` após cada processamento, mas a frequência é menor do que em `billing_subscriptions`.

### Índices

| Nome | Colunas | Condição WHERE | Query Servida |
|---|---|---|---|
| `billing_kiwify_events_pkey` (PK) | `envelope_id` | — | Deduplicação via `ON CONFLICT DO NOTHING` no `Persist` |
| `billing_kiwify_events_received_at_idx` | `received_at` | — | `DeleteOlderThan`: filtra por janela de tempo para purga |
| `billing_kiwify_events_trigger_idx` | `trigger` | — | Reprocessamento seletivo por tipo de evento |

### Operações do Repositório

| Método | SQL (resumido) | Observações |
|---|---|---|
| `Persist` | `INSERT INTO ... ON CONFLICT (envelope_id) DO NOTHING` | Idempotente; duplicatas descartadas sem erro |
| `MarkProcessed` | `UPDATE ... SET processed_at=$1 WHERE envelope_id=$2` | Marca conclusão do processamento |
| `DeleteOlderThan` | `DELETE FROM ... WHERE envelope_id IN (SELECT envelope_id WHERE received_at < $1 LIMIT N)` | Purga em lotes; subquery evita lock de tabela inteira |

---

## billing_reconciliation_checkpoints

### DDL

```sql
CREATE TABLE IF NOT EXISTS mecontrola.billing_reconciliation_checkpoints (
    name       TEXT        NOT NULL,
    watermark  TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT billing_reconciliation_checkpoints_pkey PRIMARY KEY (name)
);
```

### Colunas

| Coluna | Tipo | Nullable | Default | Propósito | Invariante |
|---|---|---|---|---|---|
| `name` | TEXT | NOT NULL | — | Identificador lógico do checkpoint (ex.: `kiwify_sales`) | PK; nomes definidos pelos jobs |
| `watermark` | TIMESTAMPTZ | NOT NULL | — | Último `occurred_at` processado com sucesso | Avança monotonicamente |
| `updated_at` | TIMESTAMPTZ | NOT NULL | `now()` | Momento da última atualização | Atualizado a cada avanço do watermark |

### Semântica do watermark

O watermark representa o **high-water mark** do processamento batch. A cada execução do job de reconciliação:

1. `Get` recupera o watermark com `SELECT ... FOR UPDATE` — serializa execuções concorrentes.
2. Se `ErrCheckpointNotFound`, o job inicializa com valor de bootstrap (`now - 1h`).
3. Após processar o lote, `Set` avança o watermark via `INSERT ... ON CONFLICT DO UPDATE`.

O `SELECT ... FOR UPDATE` é a única operação de leitura que adquire lock explícito no módulo billing.

### Operações do Repositório

| Método | SQL (resumido) | Observações |
|---|---|---|
| `Get` | `SELECT watermark WHERE name=$1 FOR UPDATE` | Retorna `ErrCheckpointNotFound` via `sql.ErrNoRows` |
| `Set` | `INSERT INTO ... ON CONFLICT (name) DO UPDATE SET watermark=EXCLUDED.watermark, updated_at=now()` | Upsert idempotente; cria o checkpoint na primeira execução |

---

## Decisões de Design

### fillfactor 80/85

`billing_subscriptions` usa `fillfactor = 80` porque cada evento Kiwify gera pelo menos um `UPDATE` na linha da assinatura. Com `fillfactor = 80`, o Postgres reserva 20% de cada página heap para HOT updates, reduzindo page splits e a necessidade de vacuum agressivo.

`billing_kiwify_events` usa `fillfactor = 85` — a taxa de updates é menor (apenas `processed_at` é preenchido após o insert).

As demais tabelas usam o `fillfactor` padrão (100) porque são insert-heavy ou sofrem updates raros.

### Schema isolation (mecontrola.*)

Todas as tabelas residem no schema `mecontrola` (não em `public`). Garante separação de namespace, permissões gerenciáveis por schema e compatibilidade com múltiplos ambientes no mesmo cluster.

### Uso de TIMESTAMPTZ (não TIMESTAMP)

Todas as colunas temporais usam `TIMESTAMPTZ`. O Postgres armazena internamente em UTC e converte na leitura conforme o `TimeZone` da sessão. A aplicação Go usa `time.Time` com localização UTC explícita, garantindo comparações temporais inequívocas.

---

## Referências

- [domain.md](domain.md)
- [usecases.md](usecases.md)
- Migration: `migrations/000001_initial_schema.up.sql`
- Repositórios: `internal/billing/infrastructure/repositories/postgres/`
