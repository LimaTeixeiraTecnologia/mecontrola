# Registro de Decisão Arquitetural (ADR-004)

## Metadados

- **Título:** Idempotência exatamente-uma-vez por intenção via ledger agent-owned
- **Data:** 2026-06-30
- **Status:** Aceita
- **Decisores:** Time de plataforma
- **Relacionados:** PRD (RF-38, D-19, D-22), techspec.md; `internal/platform/idempotency/middleware.go`; memória `feedback_agent_calls_modules_own_persistence`

## Contexto

Lançamentos exigem correção financeira: retries do agente, loops de tool-calling dentro de um mesmo `Run`, e reentregas do canal não podem duplicar. O middleware de idempotência atual **só persiste respostas 4xx** (`internal/platform/idempotency/middleware.go:136-154`): em sucesso (2xx) não grava, então uma segunda chamada cria novo registro. Os use cases de `transactions` geram `eventID := uuid.New()` por execução, sem aceitar chave de idempotência do cliente. O WhatsApp deduplica por `wamid` no dispatcher, mas isso não cobre múltiplas escritas dentro de um único inbound (D-22) nem re-execução do Run.

## Decisão

Introduzir um **ledger de idempotência agent-owned**: tabela `agents_write_ledger(user_id, wamid, item_seq, operation, resource_id, resource_kind, created_at)` com **unique `(wamid, item_seq, operation)`**. Um helper `IdempotentWrite` envolve toda tool de escrita: antes de chamar o use case, consulta o ledger por `(wamid, item_seq, operation)`; se existir, retorna o `resource_id` registrado como **replay** (`ToolOutcomeReplay`), sem segunda mutação; se não, executa o use case, grava o `resource_id` no ledger (mesma transação lógica quando possível) e retorna. `wamid` vem do inbound (`MessageID`); `item_seq` distingue múltiplos lançamentos de uma mesma mensagem (D-22); `operation` distingue create_transaction/create_card_purchase/edit/delete. O ledger é propriedade do módulo `agents` (não compartilha transação com outros módulos — apenas chama seus use cases via binding).

## Alternativas Consideradas

- **Corrigir o middleware para persistir 2xx** — Vantagem: resolve para todos os chamadores REST. Desvantagem: raio de impacto amplo, risco de regressão em billing/identity; fora do escopo do PRD. Rejeitada (registrada como risco residual).
- **Adicionar idempotency key aos use cases de transactions** — Vantagem: idempotência no produtor. Desvantagem: muda contrato de domínio de outro módulo; viola fronteira/escopo. Rejeitada.
- **Confiar só no dedup de `wamid` do WhatsApp** — não cobre loop de tool nem múltiplos itens por mensagem. Rejeitada (D-19 escolheu garantia explícita).

## Consequências

### Benefícios Esperados

- Exatamente-uma-vez por intenção sem tocar contratos de domínio; replay observável; suporta múltiplos lançamentos por mensagem.

### Trade-offs e Custos

- Tabela e consulta extra por escrita; o agente mantém persistência própria (alinhado à memória de design).

### Riscos e Mitigações

- **Janela entre use case e gravação do ledger** (crash no meio) → gravar o ledger na mesma transação do agent-owned quando o use case expõe o id de forma síncrona; em falha, o replay subsequente reconcilia por `wamid+item_seq`. Documentar semântica at-least-once→exactly-once via unique constraint.
- **Crescimento da tabela** → job de retenção análogo ao dedup do WhatsApp.

## Plano de Implementação

1. Migration `agents_write_ledger` + unique.
2. Repositório + `IdempotentWrite` helper.
3. Integrar nas tools de escrita; mapear replay para `ToolOutcomeReplay`.
4. Teste de concorrência (unique sob corrida) e replay.

## Monitoramento e Validação

- `agents_write_total{operation,outcome=created|replay}`; alerta se taxa de replay anômala.
- Teste de integração: dupla execução do mesmo inbound cria um único recurso.

## Impacto em Documentação e Operação

- Runbook: semântica de idempotência e job de retenção do ledger.

## Revisão Futura

- Reavaliar se o middleware global for corrigido para 2xx (poderia simplificar o ledger).

---

## Emenda 2026-07-01 — Idempotência alcança o domínio via `origin_ref` (PARA REVISÃO)

- **Status da emenda:** proposta / em revisão
- **Origem:** `docs/plans/2026-07-01-eliminar-janela-idempotencia-write-ledger.md` (Fase 2, opção D)
- **Reabre:** a alternativa "Adicionar idempotency key aos use cases de transactions", antes **rejeitada** na seção "Alternativas Consideradas", agora com contrato restrito que preserva a fronteira.

### Contexto da emenda

A decisão original entregou **exactly-once do registro do ledger** e aceitou como risco residual a
**janela entre o commit do domínio e a gravação do ledger** (seção "Riscos e Mitigações"): sob crash
entre os dois commits (M1) ou concorrência de mesma chave (M2), a escrita de domínio pode duplicar,
porque o `uuid.New()` de `CreateTransaction`/`CreateCardPurchase` gera um agregado novo a cada execução
e o ledger vive em transação separada (o agente **não** compartilha transação com `transactions` —
`feedback_agent_calls_modules_own_persistence`).

A Fase 1 (advisory lock por chave, `internal/agents`) já serializou M2. Esta emenda fecha **M1**, que só
é eliminável se a idempotência **alcançar a linha de domínio**.

### Motivo para reabrir a alternativa rejeitada

A rejeição original supunha "idempotency key" como **regra de domínio** injetada no use case
(violaria R-TXN-001/002 e a fronteira). Esta emenda evita isso: `origin_ref` é uma **afordância de
infraestrutura de persistência**, não regra de negócio. Distinções que preservam a governança:

- **`Decide*` permanece puro e intocado** — `origin_ref` NÃO entra em nenhum `Decide*`, comando ou
  cálculo de negócio. É provenance carregada como metadado, definida **após** o `Decide*`, no mesmo
  ponto onde hoje já se chama `SetCategorySnapshots` (padrão de mutator pós-decisão já existente).
- **A unicidade vive no repositório** (`ON CONFLICT` sobre índice único parcial), não em branching de
  domínio no use case. Não há comparação de campo semântico (`amount`, `direction`) para decidir
  comportamento — preserva R-TXN-002/004 e o gate ADR-006.
- **Producers continuam apenas mapeando evento** (R-TXN-003): o evento de criação só é publicado
  quando a linha é **de fato criada** (`created=true`); no replay não há segundo evento (as projeções
  downstream já são idempotentes por `event_id`).

### Decisão da emenda

1. **Schema (aditivo):** `mecontrola.transactions` e `mecontrola.transactions_card_purchases` ganham
   colunas *nullable* `origin_wamid TEXT`, `origin_item_seq INT`, `origin_operation TEXT` e um
   **índice único parcial** `... (origin_wamid, origin_item_seq, origin_operation) WHERE origin_wamid
   IS NOT NULL`. Linhas legadas ficam `NULL` (fora do índice) — **sem backfill**. Índice não-`CONCURRENTLY`
   (migrations rodam em transação com o runner golang-migrate/pgx; partial index sobre coluna nova é de
   construção instantânea). Para tabela grande em produção, a construção `CONCURRENTLY` é passo de runbook
   fora da migration.
2. **Contrato de escrita:** `RawCreateTransaction`/`RawCreateCardPurchase` aceitam `OriginRef` **opcional**
   (`{Wamid, ItemSeq, Operation}`). Quando presente, o repositório insere com
   `INSERT ... ON CONFLICT (origin_*) WHERE origin_wamid IS NOT NULL DO NOTHING`; em conflito, retorna a
   linha **existente** e `created=false`. Quando ausente (caminho HTTP), o índice parcial não se aplica —
   comportamento inalterado.
3. **Efeito no agente:** com a escrita de domínio idempotente, o `IdempotentWrite` do agente reexecuta a
   `WriteFn` com segurança após M1 — a `ON CONFLICT` devolve o mesmo `resource_id` (sem 2ª linha) e o
   ledger é reconciliado. O ledger passa a ser **cache de replay rápido + auditoria**, não a única
   garantia. `agents_write_total{outcome}` ganha `reconciled` (linha já existia no domínio).

### Consequências da emenda

- **Benefício:** exactly-once **por intenção** de fato (M1 e M2 eliminados), sem compartilhar transação
  entre módulos e sem regra de negócio nova em `transactions`.
- **Custo:** 2 migrations aditivas + índices parciais; um `SELECT` extra apenas no caminho de replay
  (raro); leve aumento do contrato de input (`OriginRef` opcional).
- **Invariante de governança preservada:** `Decide*` puro; sem SQL/branching de domínio fora do repo;
  cardinalidade de métrica controlada; zero comentários.

### Gate de verificação da emenda (deve permanecer verde)

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "origin_wamid\|OriginRef\|origin_operation" \
  internal/transactions/domain/services/ \
  && echo "FAIL: origin_ref vazou para Decide*/domain services" && exit 1 || true
```

### Prova exigida antes de aprovar (production-ready)

- Teste de integração (testcontainers): duas criações com o mesmo `OriginRef` → **1** linha, **1**
  evento, 2ª retorna mesmo id com `created=false`.
- **Teste de crash-injection (M1):** commit de domínio seguido de reexecução com o mesmo `OriginRef`
  (simulando reentrega após crash antes do ledger) → **0** linhas adicionais.
- Concorrência (M2, defesa em profundidade com Fase 1): N goroutines mesma chave → 1 linha.
- Gate R-TXN e gate acima verdes; `go test ./...` sem falha.
