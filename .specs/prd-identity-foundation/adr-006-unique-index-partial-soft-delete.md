# ADR-006 — Índice UNIQUE parcial `WHERE deleted_at IS NULL` em `users.whatsapp_number`

## Metadados

- **Título:** Coexistência de unicidade e soft delete via índice único parcial
- **Data:** 2026-06-03
- **Status:** Aceita
- **Decisores:** Engenharia + autor do PRD
- **Relacionados:** PRD (RF-08, RF-09, RT-05), techspec §Modelos de Dados

## Contexto

RF-09 obriga índice único em `users.whatsapp_number` enforçado em nível de banco. RT-05 obriga soft delete (linha preservada com `deleted_at` setado). Esses dois requisitos colidem: um UNIQUE total impede um novo usuário com mesmo número após soft delete do usuário anterior — cenário esperado quando uma pessoa pede exclusão (LGPD) e meses depois reabre conta com o mesmo número.

Postgres suporta nativamente UNIQUE INDEX parcial (`WHERE deleted_at IS NULL`), recurso estável desde 8.0 e compatível com a versão atual (16-alpine).

## Decisão

Criar índice único parcial:

```sql
CREATE UNIQUE INDEX uq_users_whatsapp_active
    ON users (whatsapp_number)
    WHERE deleted_at IS NULL;
```

Análogo para `email`:

```sql
CREATE UNIQUE INDEX uq_users_email_active
    ON users (lower(email))
    WHERE deleted_at IS NULL AND email IS NOT NULL;
```

Caller `PgxUserRepository.UpsertByWhatsAppNumber` confia na constraint: tenta INSERT, captura `pgerrcode.UniqueViolation` e traduz para `postgres.ErrDuplicateWhatsAppNumber`. Fluxo de upsert: SELECT por `whatsapp_number WHERE deleted_at IS NULL` → se hit, retorna; se miss, INSERT. Race condition entre SELECT e INSERT é coberta pela constraint (segundo INSERT falha e callback pode retry com SELECT vencedor — pattern padrão).

## Alternativas Consideradas

- **UNIQUE total + repositório trata reativação** (`UPDATE users SET deleted_at = NULL WHERE id = ...`) — Vantagens: schema mais simples. Desvantagens: viola LGPD spirit (usuário pediu exclusão; reativar preserva PII que deveria ser anonimizada); confunde identidade ("é o mesmo usuário ou novo?"); cria estado ambíguo. Rejeitada.
- **Sem UNIQUE no schema, unicidade só em código** — Rejeitada por violar RF-09 e expor a race condition acima.
- **UNIQUE parcial + chave composta `(whatsapp_number, deleted_at)`** — Vantagens: ambos os campos no índice. Desvantagens: `NULL` não participa de equality em UNIQUE multi-coluna (semântica NULL Postgres), então duas linhas com `deleted_at IS NULL` e mesmo número PASSAM. Rejeitada — não atende o objetivo.

## Consequências

### Benefícios Esperados

- Soft delete funciona sem conflito com unicidade.
- Postgres garante invariante atomicamente — repositório não precisa de lock externo.
- Índice menor (só linhas ativas) → leituras mais rápidas.

### Trade-offs e Custos

- Operação de `SELECT ... FOR UPDATE` em linhas soft-deletadas não é coberta pelo índice (precisa de scan adicional). Não relevante para os fluxos do PRD.
- Reativação de usuário (limpar `deleted_at`) exige confirmar que não há outra linha ativa com mesmo número — em código, a query de reativação SELECTaria o ativo primeiro. Fora de escopo deste PRD (FE-03).

### Riscos e Mitigações

- **Risco:** Operação manual (psql) restaura `deleted_at = NULL` em row com número que já tem outro ativo → constraint quebra na transação.
- **Mitigação:** Comportamento esperado e seguro; constraint impede corrupção. Runbook futuro de reativação documenta procedimento.
- **Risco:** Índice parcial não pega `whatsapp_number` reusado por mistake em row ativa duplicada se constraint for dropada por outra migration.
- **Mitigação:** Migration de drop só ocorre com PR explícito; CI integration test valida que a constraint existe pós-migrations.

## Plano de Implementação

1. Incluir o índice na `0003_identity.up.sql`.
2. Integration test `TestUniqueIndexParcialPermiteReuso` valida o cenário: cria user A com número X → soft delete → upsert número X → cria user B com novo ID, sem violação.
3. Integration test `TestDuplicateWhatsAppNumberConcorrente` valida que dois INSERT diretos simultâneos com mesmo número ativo falham (constraint trigger).
4. Mapper de pgerr code → `ErrDuplicateWhatsAppNumber` em `infrastructure/repositories/postgres/user_repository.go`.

## Monitoramento e Validação

- `SELECT indexdef FROM pg_indexes WHERE indexname = 'uq_users_whatsapp_active'` em smoke pós-deploy.
- Métrica futura: counter de `ErrDuplicateWhatsAppNumber` por minuto — pico indica regressão em upsert.

## Impacto em Documentação e Operação

- `internal/identity/README.md` documenta a constraint e a semântica de reativação (fora de escopo agora, mas explicada).

## Revisão Futura

Revisitar se a anonimização efetiva pós-30-dias (FE-04) transformar a linha soft-deletada em "tombstone" sem `whatsapp_number` — a parcial fica trivialmente válida sem revisão.
