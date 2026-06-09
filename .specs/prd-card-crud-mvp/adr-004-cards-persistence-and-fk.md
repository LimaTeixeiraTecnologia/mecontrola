# Registro de Decisão Arquitetural (ADR-004)

## Metadados

- **Título:** Persistência `mecontrola.cards` com FK física para `users(id)` e soft-delete
- **Data:** 2026-06-09
- **Status:** Aceita
- **Decisores:** Jailton (tech lead)
- **Relacionados:** `.specs/prd-card-crud-mvp/prd.md` (F-05, RF-09–RF-15, S-01), `.specs/prd-card-crud-mvp/techspec.md`

## Contexto

O PRD pede:

- Persistir `Card` com `id, user_id, name, nickname, closing_day, due_day, created_at, updated_at, deleted_at`.
- Soft-delete por `deleted_at`.
- Unicidade parcial de `nickname` entre cartões ativos do mesmo usuário.
- Índice composto para paginação por cursor.
- Não usar `init()`, não usar `clock.Clock`, manter `domain` puro.

Aberto pelo PRD (S-01): FK `cards.user_id → users.id` lógica vs. física. Inspecionando working tree, `mecontrola.users` existe no mesmo schema/banco (`000001_initial_baseline`), portanto FK física é viável.

Também há drift D-01 (schema `mecontrola.`) e D-02 (numeração de migrations) que precisam ser resolvidos coerentemente.

## Decisão

Criar `mecontrola.cards` em `000005_create_card_cards.{up,down}.sql` com:

- Colunas conforme RF-09.
- `PRIMARY KEY (id)`.
- `FOREIGN KEY (user_id) REFERENCES mecontrola.users(id) ON DELETE RESTRICT`.
- `CHECK closing_day BETWEEN 1 AND 31`, `CHECK due_day BETWEEN 1 AND 31`.
- `CHECK char_length(name) BETWEEN 1 AND 64`, `CHECK char_length(nickname) BETWEEN 1 AND 32`.
- Índice parcial `UNIQUE (user_id, nickname) WHERE deleted_at IS NULL`.
- Índice composto `(user_id, created_at DESC, id DESC) WHERE deleted_at IS NULL` para paginação.

Soft-delete por `deleted_at`. Inserção concorrente que viole `cards_user_nickname_active_uniq_idx` é mapeada para `ErrNicknameConflict` no repositório (`pgerrcode.UniqueViolation`).

`down`: `ALTER TABLE … RENAME TO cards_archived_<timestamp>` + `DROP INDEX IF EXISTS`. Nunca `DROP TABLE` direto.

Persistência em UTC; cálculo de fatura em SP (no domínio). Repositório nunca aplica regra de negócio (`R-ADAPTER-001.2`), apenas mapeia erros.

## Alternativas Consideradas

1. **FK lógica** (sem `FOREIGN KEY`) — Vantagens: nenhuma; Desvantagens: perde integridade, permite cartões órfãos por bug em outro módulo, sem benefício operacional dado que `users` está no mesmo schema. Rejeitada.
2. **FK com `ON DELETE CASCADE`** — Vantagens: limpeza automática; Desvantagens: contradiz a regra de imutabilidade histórica (PRD OBJ-06); usuário deletado pode aparecer em relatório retroativo. Rejeitada — RESTRICT é mais seguro; exclusão de usuário deve passar por anonimização explícita.
3. **Hard-delete** — Vantagens: schema simples; Desvantagens: contradiz PRD RF-13; perde rastreabilidade e impede recuperação. Rejeitada.
4. **Tabela separada `cards_history` para auditar mudanças de ciclo** — Vantagens: rastro completo; Desvantagens: fora do escopo MVP (PRD "Fora de Escopo"); volume baixo no MVP não justifica complexidade. Rejeitada.

## Consequências

### Benefícios Esperados

- Integridade referencial garantida; impossibilita cartão órfão.
- Soft-delete + rename na `down` preservam dados em rollback.
- Índices parciais reduzem custo de manutenção e mantêm queries < 50ms p99 (M-03).

### Trade-offs e Custos

- `ON DELETE RESTRICT` obriga módulo de identidade a anonimizar (ou cascatear via aplicação) antes de remover usuário. Tratável; alinhado com LGPD.
- Índice parcial requer Postgres ≥ 11 (já em uso).

### Riscos e Mitigações

- **Bloqueio em `users` em migrations futuras** → `lock_timeout` curto no `0010_initial_baseline` já é padrão; manter o mesmo pattern.
- **Crescimento da tabela com soft-deletes** → política de retenção em fase 2 (PRD S-02).
- **Inserção concorrente com mesmo `nickname`** → `INSERT` cai no índice único; repositório mapeia para `ErrNicknameConflict` → handler responde 409.

## Plano de Implementação

1. Migration `000005_create_card_cards.up.sql` + `.down.sql`.
2. Estender `migrations_integration_test.go` para validar `up`/`down`/`up`.
3. Repositório pgx (`card_repository.go`) com mapeamento de erros.
4. Testes de integração com testcontainers cobrindo unicidade parcial concorrente e paginação ≥ 250 itens.

Adoção concluída quando: migrations rodam idempotentes + integração verde + `EXPLAIN` confirma uso dos índices.

## Monitoramento e Validação

- Span `card.repository.pg.*` com atributo `outcome=success|conflict|not_found|error`.
- Métrica `http_server_request_duration_seconds{route=~"/api/v1/cards.*"}` para validar M-02/M-03.
- Critério de revisão: bloqueios > 0 em `pg_locks` sobre `mecontrola.cards` por > 30s.

## Impacto em Documentação e Operação

- `docs/runbooks/card-rollback.md` — descreve rename + revert.
- Onboarding de DB: notar que exclusão de usuário precisa passar por handler explícito.

## Revisão Futura

Revisitar se:

- Volume passar de 300k cartões (PRD volumetria) — avaliar particionamento por `user_id`.
- Política de retenção (PRD S-02) for definida — adicionar job de expurgo.
