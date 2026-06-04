# ADR-009 — `SoftDelete` propaga cascata para `user_whatsapp_history.active = false`

## Metadados

- **Título:** Soft delete do `User` desativa todas as rows ativas em `user_whatsapp_history`
- **Data:** 2026-06-03
- **Status:** Aceita
- **Decisores:** Engenharia + Produto/Suporte (autor do PRD, perspectiva LGPD)
- **Relacionados:** PRD (HU-04, RT-05, FE-04), techspec §Implementação Postgres §SoftDelete

## Contexto

O PRD declara em RT-05 que soft delete é obrigatório e anonimização efetiva entra em PRD futuro (FE-04). A tabela `user_whatsapp_history` mantém histórico de números vinculados ao usuário com flag `active boolean` e `unlinked_at timestamptz`.

A pergunta: quando `SoftDelete` for chamado no `User`, o que fazer com as rows de histórico que ainda têm `active = true`?

Três caminhos:

1. **Cascata: marcar `active = false, unlinked_at = now, reason = 'user_soft_deleted'`** — histórico fica consistente; nenhum número permanece ativo apontando para user deletado.
2. **Deixar intacto** — `SoftDelete` toca apenas `users`; histórico permanece com `active = true` em rows órfãs.
3. **Hard delete (CASCADE)** — FK já tem `ON DELETE CASCADE`; basta `DELETE FROM user_whatsapp_history WHERE user_id = $1`.

(3) viola LGPD spirit (perde-se audit trail antes da anonimização efetiva).
(2) cria estado onde queries precisam JOIN com `users.deleted_at IS NULL` em todo lugar.
(1) preserva auditoria, mantém invariante "history.active=true ⇔ user ativo" e simplifica futura anonimização (FE-04 anonimiza tombstones identificáveis por `reason = 'user_soft_deleted'`).

## Decisão

`PgxUserRepository.SoftDelete` abre `database.UnitOfWork[struct{}]` e executa atomicamente:

```sql
-- (a) marca o user como deletado
UPDATE users
   SET deleted_at = $1, status = 'DELETED', updated_at = $1
 WHERE id = $2 AND deleted_at IS NULL;

-- (b) desativa todas as rows ativas no histórico
UPDATE user_whatsapp_history
   SET active = false, unlinked_at = $1, reason = 'user_soft_deleted'
 WHERE user_id = $2 AND active = true;
```

Se (a) afetar 0 rows → `ErrUserNotFound`. (b) pode afetar 0 ou mais rows — não é erro (user pode nunca ter trocado de número, então só há a row do upsert inicial, que continua `active = true` … e portanto será desativada também por (b)).

`reason = 'user_soft_deleted'` é um marcador estável que servirá como filtro para o job de anonimização do PRD FE-04.

## Alternativas Consideradas

- **Deixar histórico intacto** — Vantagens: SQL mais simples; soft delete é uma única UPDATE. Desvantagens: histórico mantém `active = true` para user já deletado; consultas precisam JOIN sempre; LGPD complexa porque tombstone não é identificável por flag local. Rejeitada.
- **Hard delete de histórico via CASCADE** — Vantagens: schema autocoerente. Desvantagens: perde audit trail antes da janela LGPD de 30 dias; sem como reconstruir "qual número este user usou e quando" para resposta a pedido judicial ou de auditoria. Rejeitada.
- **Cascata + trigger Postgres** — Vantagens: lógica no banco, impossível esquecer. Desvantagens: triggers escondem comportamento de quem lê apenas o código Go; mais difícil testar; conflita com `persistence.md` (lógica no repository, não no DB). Rejeitada.

## Consequências

### Benefícios Esperados

- Histórico consistente: para qualquer query `WHERE active = true`, joining com `users.deleted_at IS NULL` é redundante (a invariante já foi propagada pelo `SoftDelete`).
- LGPD facilitada: job de anonimização (FE-04) filtra por `reason = 'user_soft_deleted' AND unlinked_at < now() - interval '30 days'`.
- Operação atômica: usuário soft-deletado nunca aparece "ativo" em hist por janela transiente.

### Trade-offs e Custos

- `SoftDelete` deixa de ser UPDATE simples e vira transação multi-statement — pequena complexidade adicional no repositório.
- Job de anonimização do PRD FE-04 precisará considerar o filtro `reason`.

### Riscos e Mitigações

- **Risco:** Outro fluxo registra reason diferente para `unlinked_at` (ex.: troca de número via `LinkNewNumber` com reason custom), e o job de anonimização precisa distinguir.
- **Mitigação:** `reason = 'user_soft_deleted'` é constante reservada documentada no `domain/errors.go` ou `domain/services/history_reasons.go`; reasons custom são livres mas não podem casar exatamente a string reservada (validação no construtor de `LinkNewNumberUseCase`).
- **Risco:** Reativação futura de user (FE-04+) precisa decidir se restaura rows do histórico — fora de escopo.
- **Mitigação:** Decisão fica para PRD de reativação.

## Plano de Implementação

1. `PgxUserRepository.SoftDelete` abre `database.NewUnitOfWork[struct{}](r.manager)` e executa (a) + (b) na mesma tx.
2. Declarar constante `domain.ReasonUserSoftDeleted = "user_soft_deleted"` (ou em pacote `services`).
3. `LinkNewNumberUseCase.Execute` rejeita `reason == "user_soft_deleted"` (`errors.New("identity: reason reservado para soft delete do usuário")`).
4. Integration test `TestSoftDeleteCascataDesativaHistorico` valida que após soft delete, `COUNT(*) FROM user_whatsapp_history WHERE user_id = $id AND active = true` é 0.

## Monitoramento e Validação

- Métrica futura: counter `identity_soft_delete_total` + histograma de rows afetadas no histórico (mediana esperada: 1).
- Log com chave `identity.soft_delete.history_archived` e contagem de rows.

## Impacto em Documentação e Operação

- `internal/identity/README.md` documenta a cascata.
- Runbook futuro de LGPD/FE-04 referencia esta ADR como contrato em vigor.

## Revisão Futura

Revisitar quando FE-04 (anonimização efetiva pós-30-dias) for implementado — pode demandar `reason` enum em vez de string livre.
