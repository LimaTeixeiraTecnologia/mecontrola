# ADR-003 — Estado de cruzamento de limiar em tabela dedicada com versão monotônica

## Metadados

- **Título:** Tabela `budgets_threshold_states` como fonte de verdade do cruzamento de limiar
- **Data:** 2026-06-09
- **Status:** Aceita
- **Decisores:** Time MeControla / AI Agent
- **Relacionados:** [PRD v24](./prd.md) (RF-59, RF-60, RF-60a/b, RF-60e/f, RF-61), [techspec.md](./techspec.md), [ADR-002](./adr-002-outbox-event-unico-expense-committed.md)

## Contexto

- O PRD impõe que um alerta seja gerado **apenas** quando o gasto da categoria cruza de abaixo para igual ou acima do limiar (RF-59), permanecendo em mute enquanto cruzado (RF-60), com **rearme imediato** se cair abaixo (RF-60b). Edições/exclusões que reduzem o gasto rearmam o limiar.
- O recálculo é assíncrono via outbox (ADR-002) e at-least-once: o avaliador pode receber replays e mensagens fora de ordem. O sistema precisa decidir a transição sem depender da ordem de chegada.
- Há 5 raízes × 2 limiares = 10 chaves de estado por usuário/competência. Volume: 10k usuários × 12 meses × 10 chaves ≈ 1,2M linhas — trivial para Postgres.

## Decisão

Persistir o estado em tabela dedicada:

```sql
CREATE TABLE budgets_threshold_states (
    user_id                       UUID NOT NULL,
    competence                    CHAR(7) NOT NULL,
    root_slug                     TEXT NOT NULL,
    threshold                     SMALLINT NOT NULL,
    currently_crossed             BOOLEAN NOT NULL DEFAULT FALSE,
    version                       BIGINT NOT NULL DEFAULT 0,
    last_crossed_at               TIMESTAMPTZ NULL,
    last_uncrossed_at             TIMESTAMPTZ NULL,
    last_evaluated_committed_at   TIMESTAMPTZ NULL,
    PRIMARY KEY (user_id, competence, root_slug, threshold)
);
```

Operação atômica `UpsertIfTransition` (em SQL `INSERT ... ON CONFLICT ... DO UPDATE`):

- Entrada: chave, `nowCrossed` (resultado do recálculo), `committedAt` (do evento).
- Se `committedAt < last_evaluated_committed_at`: ignora (out-of-order safe), retorna `transitioned=false`.
- Se `nowCrossed != currently_crossed`: atualiza `currently_crossed`, incrementa `version`, atualiza `last_crossed_at` ou `last_uncrossed_at`, retorna `transitioned=true`.
- Caso contrário: atualiza apenas `last_evaluated_committed_at`, retorna `transitioned=false`.

`AlertRepository.Insert` é chamado **apenas** quando `transitioned=true` E `nowCrossed=true` E demais regras (budget ACTIVE, dentro do cutoff, contador < 10).

## Alternativas Consideradas

1. **Derivar estado do histórico de `budgets_alerts`**.
   - Vantagens: economiza tabela.
   - Desvantagens: mistura responsabilidade de "evento entregue" com "estado da máquina"; rearme após exclusão (RF-60b) exige varrer alertas + recompor estado; replays ficam ambíguos.
   - Rejeitada por complicar o invariante de rearme e o tratamento at-least-once.

2. **Recalcular sob demanda sem estado persistido (consultar `budgets_alerts` apenas para dedup)**.
   - Vantagens: sem nova tabela.
   - Desvantagens: avaliação out-of-order pode emitir alerta duplicado quando uma mutação antiga é processada depois de uma nova; rearme após edição não é detectável sem tracking de "estava cruzado".
   - Rejeitada por não cumprir RF-59 sob at-least-once.

## Consequências

### Benefícios Esperados

- Determinismo total da transição independente de ordem de chegada.
- Rearme imediato (RF-60b) trivial: avaliador atualiza `currently_crossed=false` no próximo recálculo abaixo do limiar.
- Audit trail mínimo (`version` + timestamps) suficiente para investigar incidentes.

### Trade-offs e Custos

- Nova tabela + repositório + integration test.
- Atualização extra por commit financeiro (1 UPSERT por (raiz × limiar) potencialmente afetada — limitado a 2 por raiz).

### Riscos e Mitigações

- **Risco:** divergência entre `currently_crossed` e o gasto real após bugs/falhas.
  - **Mitigação:** o recálculo é a fonte de verdade do gasto; `currently_crossed` é derivado a cada evento. Job de reconciliação opcional (pós-MVP) pode varrer e corrigir.
- **Risco:** crescimento ilimitado.
  - **Mitigação:** `retention_purge` remove linhas com `competence` fora da retenção de 24 meses.

## Plano de Implementação

1. Migration `000009_create_budgets_baseline.up.sql` cria a tabela.
2. Repositório `threshold_state_repository.go` implementa `UpsertIfTransition` com SQL único `INSERT ... ON CONFLICT`.
3. `EvaluateAlert` chama o repositório dentro do mesmo tx do INSERT de alerta para garantir transição → alerta atomicamente.
4. Integration test cobre: transição true→false, replay out-of-order ignorado, rearme após queda.

## Monitoramento e Validação

- `budgets_threshold_transitions_total{root_slug,threshold,direction}`.
- Log `INFO` em transição com `version` resultante.

## Impacto em Documentação e Operação

- Esquema documentado na techspec.
- Runbook: reconciliação manual via SQL caso seja detectado drift.

## Revisão Futura

- Reavaliar se a tabela atingir > 50M linhas (sinal de mudança de volumetria que invalida RT-08).
- Reavaliar se decidirmos publicar transições como evento (extensão para LLM provider).
