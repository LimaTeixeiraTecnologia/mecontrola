# Card Module — Rollback Runbook

## Contexto

Este runbook cobre o rollback completo do modulo `card`, incluindo desfazer o wiring em
`cmd/server/server.go`, reverter as migrations do schema e troubleshooting de situacoes
criticas em producao.

## Pre-condicoes Operacionais

- Header `X-User-ID` DEVE ser injetado pelo gateway antes de qualquer requisicao para `/api/v1/cards*`.
  Sem o gateway garantindo a injecao confiavel do `X-User-ID`, o middleware `InjectPrincipalFromHeader`
  nao consegue estabelecer o principal e todas as requisicoes serao rejeitadas com 401 (ADR-003, PRD S-07).
- O middleware `InjectPrincipalFromHeader` e dependencia transitoria do `CardRouter`. Se o modulo
  identity sofrer rollback simultaneo, o modulo card nao pode ser reiniciado antes que o middleware
  seja restaurado.

## Tabelas Arquivadas apos Rollback

Apos executar `migrate down` para as migrations 000007 e 000008, as seguintes tabelas serao
renomeadas (nao removidas) para preservar dados:

- `mecontrola.idempotency_keys_archived_20260609120000`
- `mecontrola.cards_archived_20260609120000`

## Passo a Passo de Rollback

### 1. Reverter o wiring em `cmd/server/server.go`

Remover o bloco abaixo do arquivo e o import `"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"`:

```go
cardModule := card.NewCardModule(cfg, o11y, dbManager)
if cardModule.CardRouter != nil {
    srv.RegisterRouters(cardModule.CardRouter)
}
o11y.Logger().Info(ctx, "card module wired", observability.Bool("router_registered", cardModule.CardRouter != nil))
```

Recompilar e reimplantar o binario:

```bash
task build
```

### 2. Reverter as migrations (2 passos)

```bash
migrate -database "$DATABASE_URL" -path migrations down 2
```

As migrations revertidas sao:
- `000008_create_cards.down.sql` — renomeia `mecontrola.cards` para `mecontrola.cards_archived_<timestamp>`
- `000007_create_idempotency_keys.down.sql` — renomeia `mecontrola.idempotency_keys` para `mecontrola.idempotency_keys_archived_<timestamp>`

### 3. Verificar arquivamento das tabelas

```sql
SELECT to_regclass('mecontrola.idempotency_keys_archived_20260609120000');
SELECT to_regclass('mecontrola.cards_archived_20260609120000');
```

Resultado esperado: ambas devem retornar o nome da tabela (nao NULL).

### 4. Re-aplicar migrations apos estabilizacao

```bash
migrate -database "$DATABASE_URL" -path migrations up
```

## Troubleshooting

### `mecontrola.idempotency_keys`: chave duplicada no `ON CONFLICT`

Sintoma: inserts de idempotency retornam `23505` (unique violation) mesmo com `ON CONFLICT DO NOTHING`.

Causa provavel: race condition — dois requests com o mesmo `(scope, key, user_id)` chegaram simultaneamente
antes de qualquer um registrar. O design com `ON CONFLICT DO NOTHING RETURNING` e idempotente; o segundo
insert nao gera erro — retorna zero rows. Verificar se o use case esta tratando esse caso corretamente
em `idempotency.Storage.Put`.

Verificar no banco:

```sql
SELECT scope, key, user_id, created_at
  FROM mecontrola.idempotency_keys
 WHERE key = '<valor-da-chave>'
 ORDER BY created_at DESC
 LIMIT 5;
```

### Rollback parcial: tabela `cards` existe mas `idempotency_keys` foi removida

Se o down de `000008` foi executado mas `000007` ainda nao:

```bash
migrate -database "$DATABASE_URL" -path migrations down 1
```

Verificar estado atual:

```bash
migrate -database "$DATABASE_URL" -path migrations version
```

### `America/Sao_Paulo` timezone nao carrega

Sintoma: `services.SaoPauloLocation()` chama `os.Exit(1)` no startup.

Causa: `tzdata` nao disponivel na imagem de container.

Solucao (Alpine Linux):

```dockerfile
RUN apk add --no-cache tzdata ca-certificates
```

Verificar no container:

```bash
ls /usr/share/zoneinfo/America/Sao_Paulo
```

### Cards nao aparecem apos rollback e re-apply

Apos `migrate down 2` + `migrate up`, as tabelas sao recriadas vazias. Dados das tabelas arquivadas
podem ser restaurados manualmente:

```sql
INSERT INTO mecontrola.cards
SELECT * FROM mecontrola.cards_archived_20260609120000;
```

Validar contagem antes de prosseguir.

### `X-User-ID` ausente: todos os cards retornam 401

O middleware `InjectPrincipalFromHeader` le o header `X-User-ID` injetado pelo gateway. Em ambiente
de desenvolvimento sem gateway, injetar o header manualmente:

```bash
curl -H "X-User-ID: <uuid-do-usuario>" /api/v1/cards
```

Em producao, garantir que o gateway sempre injete o header antes de rotear para `/api/v1/cards*`.
Registrar alerta no runbook de incidentes se o gateway nao estiver injetando corretamente.

## Dependencias do Rollback

| Componente | Dependencia | Acao necessaria |
|------------|-------------|-----------------|
| `CardRouter` | `InjectPrincipalFromHeader` | Modulo identity deve estar operacional |
| `idempotency.Storage` | `mecontrola.idempotency_keys` | Migration 000007 deve estar aplicada |
| `card.Repository` | `mecontrola.cards` | Migration 000008 deve estar aplicada |
| `services.SaoPauloLocation()` | `tzdata` no container | `apk add tzdata` na imagem |

## Notas

- O down migration renomeia a tabela (e seus constraints) para preservar dados.
- Um segundo `down` em estado ja arquivado e no-op (idempotente).
- Para reverter completamente o arquivamento, usar `ALTER TABLE ... RENAME TO ...` manualmente.
- Exclusao de usuario bloqueada por `ON DELETE RESTRICT` FK exige anonimizacao dos dados antes
  da exclusao (conformidade LGPD).

## Contato

Em caso de incidente critico, escalar para o runbook de resposta de incidente auth em
`docs/runbooks/auth-incident-response.md` para procedimentos de contato e escalacao.
