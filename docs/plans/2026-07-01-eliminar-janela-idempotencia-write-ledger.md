# Plano — Eliminar a janela de idempotência write→ledger (exactly-once real)

- **Data:** 2026-07-01
- **Autor:** revisão de plataforma (ciclo review→hardening do `prd-mecontrola-agent`)
- **Escopo:** `internal/agents` (helper `IdempotentWrite`, ledger agent-owned) + fronteira de escrita de `internal/transactions`
- **Status:** proposta para decisão (requer emenda governada da ADR-004 na Fase 2)
- **Skill obrigatória para execução Go:** `go-implementation` (carregar `.agents/skills/go-implementation/SKILL.md` antes de qualquer edição; DMMF, zero comentários R-ADAPTER-001.1, adapters finos, testes testify/suite R-TESTING-001).
- **Regras aplicáveis:** ADR-004 (idempotência agent-owned), ADR-006/`R-TXN-WORKFLOWS-001` (fronteira e pureza de `transactions`), `feedback_agent_calls_modules_own_persistence` (agente nunca compartilha transação com outro módulo), R-AGENT-WF-001.2 (tool fina).

---

## 1. Problema

A escrita idempotente do agente hoje é um **dual-write em duas transações independentes**:

```
IdempotentWrite.Execute (internal/agents/application/usecases/idempotent_write.go)
  1. ledger.FindByKey(wamid,item_seq,operation)         -> tx_ledger (read)
  2. write(ctx)  == txLedger.CreateTransaction(...)      -> tx_transactions (COMMIT próprio, uuid.New())
  3. ledger.Insert(entry{resource_id})                  -> tx_ledger (COMMIT próprio)
```

- `CreateTransaction.Execute` (`internal/transactions/application/usecases/create_transaction.go:80-87`) gera `txID := uuid.New()` e `eventID := uuid.New()` **por chamada** e commita na **própria** `uow` — não aceita chave de idempotência e não tem unicidade por origem.
- O ledger agent-owned (`mecontrola.agents_write_ledger`, unique `(wamid,item_seq,operation)`) commita numa transação **separada** da de `transactions` (obrigatório: o agente não compartilha transação com outro módulo).

Como os passos 2 e 3 não são atômicos entre si, existem **dois** modos de falha que produzem escrita duplicada:

| Modo | Sequência | Resultado |
|---|---|---|
| **M1 — crash entre commit do domínio e insert do ledger** | passo 2 commita a transação em `transactions`; processo morre antes do passo 3; WhatsApp reentrega o inbound | passo 1 não encontra ledger → passo 2 executa de novo com **novo** `uuid.New()` → **2 linhas** em `transactions` para a mesma intenção |
| **M2 — concorrência pura (dois consumidores/threads, mesma chave)** | ambos passam o passo 1 (miss), ambos executam passo 2 (2 commits de domínio), um insere ledger e o outro recebe `ON CONFLICT DO NOTHING` | **2 linhas** em `transactions`; o segundo racer ainda retorna o próprio `resource_id` como `created` |

A unique constraint do ledger garante exactly-once **do registro do ledger**, mas **não** do efeito no domínio, porque o domínio não participa da chave de idempotência. É a limitação clássica de dual-write.

### Estado atual e ADR-004

A ADR-004 **aceitou conscientemente** essa semântica `at-least-once → exactly-once via unique constraint` (seção "Riscos e Mitigações", linha 37) e **rejeitou** duas alternativas: (a) corrigir o middleware global para 2xx e (b) adicionar idempotency key aos use cases de `transactions` (por cruzar contrato de domínio). A ADR-004 possui cláusula "Revisão Futura", então revisá-la é caminho previsto de governança — **não** é flexibilização.

O replay **sequencial** (reentrega comum do WhatsApp, loop de tool dentro do mesmo Run) já é exactly-once e está provado por testcontainers (`transactions_integration_test.go::TestCenario2`, `write_ledger_repository_integration_test.go::TestUniqueConstraintUnderConcurrency`, E2E `TestE2E2_ReprocessarMesmoWamidNaoDuplica`). O que este plano ataca é **M1 (crash)** e **M2 (concorrência)**.

---

## 2. Objetivo e critério de sucesso

**Invariante alvo (exactly-once por intenção):** para qualquer `(wamid, item_seq, operation)`, o número de agregados de domínio efetivamente criados é **exatamente 1**, mesmo sob:
- reentrega do canal,
- reinício/crash do processo entre commit de domínio e registro do ledger,
- execução concorrente de N consumidores/threads com a mesma chave.

**Critérios de aceite mensuráveis:**
1. Teste de concorrência: 50 goroutines com a mesma chave → **1** linha em `transactions`/`transactions_card_purchases`, N−1 replays.
2. Teste de crash-injection (testcontainers): falha injetada **após** commit do domínio e **antes** do registro do ledger; reexecução → **0** linhas adicionais.
3. Métrica `agents_write_total{outcome}` distingue `created|replay|reconciled` com replay ≥ 1 nos testes acima.
4. Nenhuma regressão de contrato: `transactions` continua com `Decide*` puro e sem regra de negócio nova (ADR-006/R-TXN mantidos).

---

## 3. Restrições de projeto (não negociáveis)

- **RC-1** O agente **não** compartilha transação com `transactions` (`feedback_agent_calls_modules_own_persistence`). Qualquer atomicidade cross-módulo está proibida.
- **RC-2** Tool é adapter fino; sem SQL/regra/branching de domínio (R-AGENT-WF-001.2). A lógica de idempotência vive em use case/helper e no adapter de persistência.
- **RC-3** `transactions` mantém `Decide*` puro; idempotência é afor­dância **de infraestrutura** (chave + unicidade no repositório), não regra de negócio no use case (R-TXN-001/002).
- **RC-4** Zero comentários em produção (R-ADAPTER-001.1); estados fechados (DMMF state-as-type) para novos outcomes.

---

## 4. Análise de opções

| # | Abordagem | Elimina M1 (crash)? | Elimina M2 (concorrência)? | Toca `transactions`? | Esforço | Risco |
|---|---|:--:|:--:|:--:|:--:|:--:|
| A | **Claim-first no ledger** (reserva `pending` antes do write, confirma depois) | Parcial¹ | **Sim** | Não | M | M |
| B | **Advisory lock** `pg_advisory_xact_lock(hash(chave))` no helper | Não | **Sim** | Não | P | B |
| C | **ID determinístico do agregado** = UUIDv5(chave) + `INSERT ON CONFLICT (id) DO NOTHING` no repo de `transactions` | **Sim** | **Sim** | Sim (aceita id/chave) | M | M |
| D | **Coluna de origem + índice único parcial** em `transactions`/`transactions_card_purchases` (`origin_wamid, origin_item_seq, origin_operation`), repo faz `ON CONFLICT (origin_*) DO NOTHING` e retorna a linha existente | **Sim** | **Sim** | Sim (aceita `OriginRef` opcional) | M | B |
| E | **Inbox/outbox no agente** (grava intenção serializada, relay executa) | Parcial¹ | Sim | Não | G | A |

¹ Claim-first e inbox só eliminam M1 se combinados com idempotência que alcance a escrita de domínio (C ou D); sozinhos, um crash após o commit do domínio e antes de confirmar deixa o claim `pending` sem saber se o domínio persistiu — não é possível decidir com segurança entre replay e reexecução.

**Conclusão de engenharia:** a eliminação **total e comprovável** de M1 exige que a **idempotência alcance a linha de domínio** — ou seja, `transactions` precisa participar (opção **C** ou **D**). Todas as demais apenas encolhem a janela. Entre C e D, **D é preferível**: colunas de origem *nullable* + índice único parcial são minimamente invasivas, preservam o PK atual, permitem backfill incremental, e mantêm o id gerado pelo domínio (sem acoplar o PK a uma chave externa).

---

## 4-bis. Veredito de arquitetura — como o agente fala com transactions

**Como está hoje (verificado no código):**

```
tool register_expense (adapter fino, R-AGENT-WF-001.2)
 └─ IdempotentWrite.Execute            [ledger agent-owned, TX PRÓPRIA]
     └─ WriteFn → binding (transactions_ledger_adapter.CreateTransaction)
         └─ transactions.CreateTransaction.Execute   [SÍNCRONO, IN-PROCESS, VIA USE CASE]
             └─ uow.Do (UMA tx do transactions):
                 ├─ repo.Create(transaction)                  [escrita de domínio]
                 └─ publisher.PublishCreated(ctx, db, evt)    [OUTBOX na MESMA tx → atômico]
```

Fatos:
- **Pela use case? Sim** — `internal/agents/infrastructure/binding/transactions_ledger_adapter.go:CreateTransaction` chama `transactions.CreateTransaction.Execute`. Nunca SQL/repo direto (ADR-003).
- **Dispara evento? Sim** — `create_transaction.go:87-104` faz `repo.Create` + `publisher.PublishCreated(ctx, db, ...)` na **mesma** `uow.Do` (outbox transacional atômico).
- **O evento é transporte da chamada do agente? Não** — ele alimenta **projeções downstream**: `internal/budgets/.../transaction_created_consumer.go` (reflete gasto no orçamento) + recompute de resumo mensal, consumidos **exactly-once por `event_id`** (`outbox ... ON CONFLICT (id) DO NOTHING`).
- **`transactions` tem consumer inbound de comando? Não** — só produz eventos; a entrada é sempre síncrona (HTTP ou binding do agente).

**Padrão já correto:** comando **síncrono** pela use case (consistência imediata) + saída **event-driven idempotente** para quem projeta. O único gap é a idempotência do agente viver em transação separada (dual-write).

**Veredito — melhor arquitetura para eliminar TODOS os gaps:** manter a **chamada síncrona pela use case** e fazer a **idempotência alcançar a linha de domínio** (opção **D**: `origin_ref` + índice único parcial em `transactions`). Regra: **comando = síncrono com idempotência no domínio; evento = assíncrono pelo outbox para quem projeta.**

**Por que NÃO tornar a escrita event/command-driven (assíncrona):** lançamento é report-only (`project_agent_writes_report_only`), mas a resposta conversacional ("registrado ✅") e o `query_month` do **mesmo turno** exigem consistência imediata. Command-via-outbox tornaria a escrita eventual (agente responde antes de existir a transação) e exigiria consumer inbound novo + reconciliação de UX — **mais superfície, não menos gaps**. O event-driven já está no lugar certo (projeção downstream); trazê-lo para o transporte da requisição não fecha gap adicional. Fica registrado como alternativa **apenas** se surgir requisito de desacoplamento/escala (ver §4, opção E).

---

## 5. Recomendação — defense-in-depth em 2 fases

> **Refinamento (2026-07-01, pós-análise de código):** a `internal/agents.Deps.DB` é `database.DBTX`
> (sem `Conn()`/`Begin()`), e o **dispatcher do WhatsApp já deduplica por `wamid`** (contexto ADR-004),
> então **M2 (concorrência de mesma chave) já está majoritariamente mitigado** em produção. O risco
> material remanescente é **M1 (crash)**, que só a Fase 2 (`origin_ref` no domínio) fecha. Por isso a
> Fase 1 entrega **apenas o advisory lock (F1.1)** como defense-in-depth de M2, e o **estado de claim
> do ledger (`reserved/confirmed`) foi movido para a Fase 2** — implementá-lo isolado regrediria o M1
> (claim `reserved` órfão com domínio já escrito e `resource_id` desconhecido → escrita **bloqueada**
> na reentrega, pior que o duplicado atual). Só com `origin_ref` (Fase 2) a reconciliação do claim é
> segura.

### Fase 1 (mitigação imediata, sem tocar `transactions`) — serializa M2

**F1.1 — Advisory lock por chave no helper (opção B).** Envolver `IdempotentWrite.Execute` inteiro num
`KeyLocker.WithKeyLock(ctx, key, fn)`, onde `key = wamid|item_seq|operation`. O locker fixa uma conexão
dedicada (`*sqlx.DB.Conn`) e faz `SELECT pg_advisory_lock(hashtext($1))` na entrada e `pg_advisory_unlock`
na saída (session-scoped, atravessa a chamada a `transactions` que roda em conexão própria — o lock é
global por chave). Serializa racers da mesma chave: o segundo espera, revê o ledger e faz replay. Elimina
**M2** sem tocar em `transactions` e **sem regredir M1** (comportamento de crash idêntico ao atual).

- Injeção pela composition root (`cmd/server`, `cmd/worker` já têm o `*sqlx.DB`) via `Deps.KeyLocker`
  opcional; quando `nil`, `IdempotentWrite` roda direto (compat + testes sem DB).
- Gate por flag `AGENT_WRITE_ADVISORY_LOCK` (default `true`) para rollout/reversão.

**F1.2 — Estado de claim (`reserved/confirmed`) do ledger → MOVIDO PARA A FASE 2.** Ver refinamento em §5:
isolado, regride M1. Só é seguro junto com `origin_ref`.

Entregável da Fase 1: M2 serializado (defense-in-depth sobre o dedup de `wamid` do dispatcher), zero
mudança de contrato de `transactions`, zero regressão de M1. Deploy reversível por flag.

### Fase 2 (eliminação total de M1) — idempotência alcança o domínio (opção D)

**F2.1 — Emenda governada à ADR-004** reabrindo a alternativa "chave no produtor" com contrato restrito: `transactions` aceita um **`OriginRef` opcional** (infra, não domínio) e o **repositório** aplica `ON CONFLICT (origin_*) DO NOTHING RETURNING`. Isso não adiciona regra de negócio ao `Decide*` (mantém R-TXN-001), apenas unicidade de infraestrutura no adapter de persistência.

**F2.2 — Colunas + índice único parcial** em `mecontrola.transactions` e `mecontrola.transactions_card_purchases`:
```sql
ALTER TABLE mecontrola.transactions
  ADD COLUMN origin_wamid     TEXT NULL,
  ADD COLUMN origin_item_seq  INT  NULL,
  ADD COLUMN origin_operation TEXT NULL;

CREATE UNIQUE INDEX CONCURRENTLY transactions_origin_uk
  ON mecontrola.transactions (origin_wamid, origin_item_seq, origin_operation)
  WHERE origin_wamid IS NOT NULL;
```
(análogo para `transactions_card_purchases`). Linhas antigas ficam com `NULL` e saem do índice — backfill não é necessário.

**F2.3 — Fluxo final do `IdempotentWrite`:**
```
1. advisory-lock(chave)                       # F1.1 (serializa M2)
2. hit := ledger.FindByKey(chave)             # replay rápido, caso comum
   if hit -> return replay(hit.resource_id)
3. resourceID, created := write(ctx, originRef=chave)
      # repo de transactions: INSERT ... ON CONFLICT (origin_*) DO NOTHING RETURNING id
      # se conflito (linha já existe por crash anterior): retorna a linha existente (created=false)
4. ledger.Upsert(chave, resourceID)           # idempotente; reconcilia claim reserved->confirmed
5. return created ? routed : reconciled
```
Com F2.2, mesmo que M1 ocorra (crash entre 3 e 4), a reexecução do passo 3 **não cria** segunda linha — o índice único por origem barra o insert e devolve a linha existente. Ledger é reconciliado no passo 4. **M1 eliminado.**

**Escopo de operações:** aplicar `OriginRef` às três criações (`create_transaction` de despesa/receita e `create_card_purchase`). Edição/remoção (HITL) já são exactly-once via CAS do snapshot do kernel (workflow durável) e ficam fora deste plano.

---

## 6. Design detalhado (Fase 2)

### 6.1 Contrato `transactions` (fronteira de infraestrutura)
- `RawCreateTransaction`/`RawCreateCardPurchase` ganham campo opcional `OriginRef *OriginRef` (`{Wamid string; ItemSeq int; Operation string}`), validado só quando presente (não-vazio) no smart constructor do input (R-DTO-VALIDATE-001) — **sem** validação de enum de domínio.
- `Decide*` permanece **inalterado e puro**: o `OriginRef` é carregado como metadado até o repositório; não participa de nenhum cálculo de negócio.
- Repositório (`internal/transactions/infrastructure/repositories/postgres/*`): `INSERT ... ON CONFLICT (origin_wamid, origin_item_seq, origin_operation) WHERE origin_wamid IS NOT NULL DO NOTHING RETURNING id`. Se `RETURNING` vazio, fazer `SELECT id` pela chave de origem e retornar `(id, created=false)`.
- Use case mapeia `created=false` para um resultado idempotente (sem novo domain event de criação — o producer só publica quando `created=true`, preservando R-TXN-003).

### 6.2 Contrato `agents`
- `interfaces.TransactionsLedger.CreateTransaction/CreateCardPurchase` ganham o `OriginRef` no DTO consumer-side; o adapter `transactions_ledger_adapter.go` repassa ao input de `transactions`.
- `IdempotentWrite`: acrescenta advisory-lock (F1.1), passa `OriginRef` para a `WriteFn`, e trata `Insert` do ledger como `Upsert` idempotente. Novo `ToolOutcome` fechado `reconciled` (DMMF state-as-type; `agent.ToolOutcome`).
- Ledger DDL ganha `status TEXT NOT NULL DEFAULT 'confirmed'` (com `reserved` para claim-first) — coluna aditiva, compatível.

### 6.3 Migrations
- `00000N_transactions_origin_ref.up.sql`: colunas + `CREATE UNIQUE INDEX CONCURRENTLY ... WHERE origin_wamid IS NOT NULL` (sem lock longo; rodar fora de transação — respeitar padrão de migrations do repo).
- `00000N_agents_write_ledger_status.up.sql`: coluna `status` aditiva.
- Downs correspondentes (drop index/coluna).

---

## 7. Estratégia de testes (testcontainers + real)

- **Concorrência (M2):** suite testcontainers com 50 goroutines chamando `IdempotentWrite.Execute` na mesma chave → asserta 1 linha em `transactions` e N−1 `replay/reconciled`. Estende `write_ledger_repository_integration_test.go`.
- **Crash-injection (M1):** `WriteFn` de teste que commita o domínio e então retorna erro simulando morte antes do ledger; reexecuta → asserta 0 linhas adicionais e `outcome=reconciled`. Requer o índice de origem (Fase 2) para passar — é o teste que **prova** a eliminação.
- **E2E real-LLM:** estender `mecontrola_agent_e2e_test.go` com reprocessamento do mesmo `wamid` forçando itemSeq idêntico → 1 transação.
- **Regressão de fronteira:** gate `R-TXN` (regra de domínio fora de `Decide*`) continua verde; `OriginRef` não aparece em `Decide*`.
- Rodar via novo target `task agents:integration` e `task transactions:integration`.

---

## 8. Observabilidade

- `agents_write_total{operation,outcome}` com `outcome ∈ {created, replay, reconciled, usecase_error}` (cardinalidade controlada — sem `user_id`, herda R-TXN-004/R-AGENT-WF-001.5).
- Novo contador `agents_write_reconciled_total` e gauge de `claims_orphan` (claims `reserved` sem confirmação além de TTL) → alerta de taxa anômala (sinal direto de M1 em produção).
- Runbook: seção "idempotência exactly-once" + procedimento de reconciliação manual (não deve ser necessário após Fase 2).

---

## 9. Rollout

1. **Fase 1** atrás de flag `AGENT_WRITE_ADVISORY_LOCK` (default on em prod após canário). Reversível.
2. **Fase 2**: deploy da migration de colunas/índice (aditiva, `CONCURRENTLY`) **antes** do código que popula `OriginRef`; código novo passa a enviar `OriginRef`; índice único entra em vigor para linhas novas. Sem backfill (linhas antigas `NULL`).
3. Habilitar crash-injection test no CI como gate bloqueante da invariante exactly-once.
4. Atualizar ADR-004 (emenda) + revisar nota de risco residual (passa de "aceito" para "eliminado na Fase 2").

---

## 10. Riscos e mitigações

| Risco | Mitigação |
|---|---|
| Emenda à ADR-004 reabre alternativa antes rejeitada | Restringir o contrato a `OriginRef` **de infraestrutura** no repositório; `Decide*` intocado; documentar que difere da "idempotency key no domínio" rejeitada por não injetar regra de negócio |
| `CREATE INDEX CONCURRENTLY` em tabela grande | Rodar fora de transação, em janela de baixo tráfego; índice parcial reduz custo |
| Advisory lock hot-key serializa demais | Chave é `(wamid,item_seq,operation)` — altíssima cardinalidade, contenção só entre duplicatas reais (que devem serializar mesmo) |
| Divergência ledger×domínio durante Fase 1 (antes de F2.2) | M1 permanece observável (claim órfão) e raro; F2 fecha; nunca pior que o estado atual |

---

## 11. Alternativas descartadas

- **Corrigir middleware global de idempotência para 2xx** — raio de impacto amplo (billing/identity), fora de escopo; mantido como descartado da ADR-004.
- **Compartilhar transação agente↔transactions** — viola RC-1 (`feedback_agent_calls_modules_own_persistence`).
- **ID determinístico como PK (opção C)** — funciona, mas acopla o PK a uma chave externa e complica migração/legado; D entrega o mesmo exactly-once com menor invasão.

---

## 12. DoD do plano de execução

- [ ] Fase 1 implementada, flag, testes de concorrência (M2) verdes em testcontainers.
- [ ] Emenda ADR-004 redigida e aprovada antes de qualquer código da Fase 2 (gate de governança).
- [ ] Migrations aditivas (colunas origem + índice parcial + status do ledger) + downs.
- [ ] `transactions` aceita `OriginRef` opcional com `ON CONFLICT` no repo; `Decide*` inalterado; gate R-TXN verde.
- [ ] `IdempotentWrite` reescrito (advisory-lock + OriginRef + upsert ledger + outcome `reconciled` fechado).
- [ ] Teste de crash-injection (M1) verde — prova a eliminação.
- [ ] E2E real-LLM de reprocessamento verde; métricas/runbook atualizados.
- [ ] Nota de risco residual da entrega mecontrola-agent atualizada: janela **eliminada**.

---

## Referências

- ADR-004: `.specs/prd-mecontrola-agent/adr-004-idempotencia-escrita-agent-owned.md` (seção Riscos/Mitigações + Revisão Futura)
- Código: `internal/agents/application/usecases/idempotent_write.go`; `internal/agents/infrastructure/persistence/write_ledger_repository.go`; `internal/transactions/application/usecases/create_transaction.go:80-87`, `create_card_purchase.go`
- Schema: `migrations/000001_initial_schema.up.sql:764-850` (transactions, transactions_card_purchases); `migrations/000006_agents_write_ledger.up.sql`
- Regras: `.claude/rules/transactions-workflows.md` (R-TXN), `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001.2/.5), memória `feedback_agent_calls_modules_own_persistence`
- Provas atuais de exactly-once sequencial: `internal/agents/infrastructure/binding/transactions_integration_test.go::TestCenario2`; `internal/agents/application/agents/mecontrola_agent_e2e_test.go::TestE2E2`
