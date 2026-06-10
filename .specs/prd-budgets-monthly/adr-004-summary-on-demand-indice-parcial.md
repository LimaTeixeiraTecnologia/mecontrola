# ADR-004 — Resumo on-demand com índice composto parcial e soft-delete físico

## Metadados

- **Título:** Resumo mensal por agregação SQL sob demanda; despesas com `deleted_at` e índice parcial
- **Data:** 2026-06-09
- **Status:** Aceita
- **Decisores:** Time MeControla / AI Agent
- **Relacionados:** [PRD v24](./prd.md) (RF-48–RF-54, RT-29, RT-07, M-05, RF-27/RF-44–RF-47), [techspec.md](./techspec.md)

## Contexto

- RF-54 proíbe acumulado financeiro persistido como fonte de verdade.
- M-05 exige p95 ≤ 300 ms para consulta do resumo no perfil de até 100 despesas/usuário/mês.
- RF-27 exige exclusão física dos valores financeiros (despesa não pode aparecer em consultas nem contribuir para totais), porém a identidade canônica deve continuar bloqueando recriação por retry durante 24 meses (RF-45, tombstone).
- RF-30 exige que alterações de competência ou subcategoria reflitam no resumo no mesmo commit transacional.

## Decisão

Tabela única `budgets_expenses` com soft-delete físico (`deleted_at TIMESTAMPTZ NULL`) e `tombstone_version BIGINT NULL`. Linha com `deleted_at IS NOT NULL` é tombstone técnico (RF-47a): consultas do usuário filtram por `WHERE deleted_at IS NULL`; a unicidade `(user_id, source, external_transaction_id)` continua valendo, bloqueando reuso (RF-45). Após 24 meses, `retention_purge` apaga fisicamente a linha — só então o trio fica reusável (RF-47b).

Resumo mensal calculado on-demand:

```sql
SELECT root_slug, SUM(amount_cents) AS spent_cents
FROM budgets_expenses
WHERE user_id = $1 AND competence = $2 AND deleted_at IS NULL
GROUP BY root_slug;
```

Suportada por índice composto parcial:

```sql
CREATE INDEX budgets_expenses_summary_root_idx
    ON budgets_expenses (user_id, competence, root_slug)
    WHERE deleted_at IS NULL;
```

A coluna `root_slug` é desnormalizada na linha de despesa (preenchida no commit a partir de `CategoriesReader.ValidateExpenseSubcategory`). Evita JOIN com `categories` no caminho quente.

## Alternativas Consideradas

1. **Tabela separada `budgets_expense_tombstones`**.
   - Vantagens: isolamento físico de tombstones.
   - Desvantagens: exige UNIQUE em duas tabelas para identidade canônica; mover linha (DELETE + INSERT) em cada exclusão; mais tx, mais complexidade. Não trouxe benefício mensurável.
   - Rejeitada.

2. **View materializada `budgets_monthly_summary`**.
   - Vantagens: leitura ainda mais rápida.
   - Desvantagens: refresh assíncrono fere RF-30 (mesmo commit); refresh síncrono onera o caminho de escrita. Conflita com RF-54 (acumulado persistido).
   - Rejeitada.

3. **Coluna `state` enum (`active|deleted`)**.
   - Vantagens: explicita estado.
   - Desvantagens: complica o cálculo "24 meses contados da exclusão" para expurgo (precisa coluna adicional `deleted_at`). Soft-delete por `deleted_at` é equivalente e mais idiomático.
   - Rejeitada.

## Consequências

### Benefícios Esperados

- Zero divergência possível entre acumulado e despesas — o resumo é sempre soma direta.
- p95 esperado < 50 ms para o filtro `(user_id, competence)` no perfil RT-08.
- Idempotência por identidade canônica continua simples e cobre tombstone naturalmente.

### Trade-offs e Custos

- Cada consulta paga o custo do `SUM`. Aceitável até a ordem de grandeza de RT-08.
- Desnormalização de `root_slug` exige atenção em correções editoriais — ver risco.

### Riscos e Mitigações

- **Risco:** mudança de `subcategory_id` por edição exige recalcular `root_slug` na linha (RF-30).
  - **Mitigação:** use case `UpsertExpense` chama `ValidateExpenseSubcategory` e atualiza `root_slug` no UPDATE.
- **Risco:** subcategoria descontinuada e migrada a outra raiz (cenário editorial futuro).
  - **Mitigação:** categories proíbe mudar `parent_id` de uma subcategoria existente; ADR de categories cobre isso.
- **Risco:** índice parcial não usado por planner se o filtro de `deleted_at` for omitido em alguma query.
  - **Mitigação:** todos os repositórios incluem `WHERE deleted_at IS NULL` no método de leitura; integration test com `EXPLAIN` para confirmar uso do índice parcial.

## Plano de Implementação

1. Migration cria tabela + índices conforme techspec.
2. `expense_repository.go` aplica filtro `deleted_at IS NULL` em todos os reads.
3. `SoftDelete` atualiza `deleted_at = now()`, incrementa `version`, escreve `tombstone_version`.
4. `retention_purge` (job mensal) executa `DELETE FROM budgets_expenses WHERE deleted_at < now() - interval '24 months'` em lotes.
5. Integration test com `EXPLAIN ANALYZE` no `SumByRoot` confirma `Index Only Scan` no índice parcial.

## Monitoramento e Validação

- `budgets_summary_query_seconds` histograma.
- Alerta se p95 > 200 ms (headroom para M-05 = 300 ms).
- `budgets_retention_purged_total{table}` por execução do job.

## Impacto em Documentação e Operação

- Esquema documentado.
- Runbook: como inspecionar tombstones para suporte (`WHERE deleted_at IS NOT NULL`).

## Revisão Futura

- Reavaliar quando o p99 do `SumByRoot` aproximar 200 ms ou volume real exceder 1k despesas/usuário/mês.
- Reavaliar se RF-54a (resumo só por raiz) for relaxado para expor subcategoria.
