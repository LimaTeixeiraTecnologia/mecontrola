# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Serialização por usuário via claim particionado do outbox
- **Data:** 2026-07-01
- **Status:** Aceita
- **Decisores:** time de plataforma (autor), owner do produto (decisões D-01/D-06/D-08 do PRD)
- **Relacionados:** PRD `.specs/prd-whatsapp-ordenacao-idempotencia/prd.md` (RF-01/02/03/18/19),
  techspec `techspec.md`, diagnóstico `docs/runs/2026-07-01-diagnostico-mensagens-fora-ordem-arquitetura-10k.md`,
  regras R-WF-KERNEL-001, R-ADAPTER-001, R-TXN-004

## Contexto

O `outbox` reivindica eventos com `ORDER BY next_attempt_at ... FOR UPDATE SKIP LOCKED`
(`internal/platform/outbox/storage_postgres.go` `ClaimBatch`), sem partição por usuário. Com 2
workers (`mecontrola_worker-1/2`) rodando o dispatcher a cada 500ms, eventos do mesmo usuário são
processados **concorrentemente e fora de ordem** — raiz comprovada do incidente de 2026-07-01.

Restrições duras: pgbouncer em `pool_mode=transaction` (confirmado) torna **inseguro**
`pg_advisory_lock` de sessão (usado em `advisory_key_locker.go:32`, hoje desligado). O LLM leva ~3s
por turno; **segurar uma transação/conexão durante o LLM esgotaria o pool** (`default_pool_size=15`).
O kernel/outbox deve permanecer genérico (R-WF-KERNEL-001) e sem novo componente de infra.

## Decisão

Transformar o claim do outbox em **claim particionado por `aggregate_user_id`**: no máximo **1 evento
em voo (status=2) por usuário**; o próximo evento de um usuário só é reivindicado após o anterior
concluir. Ordenação FIFO por `occurred_at` (que passará a carregar o **timestamp da Meta**, RF-18).

Mecanismo (adapter Postgres, sem mudar a assinatura pública do repositório):
- Migration `000003`: índice parcial `(aggregate_user_id, occurred_at) WHERE status=1` e índice
  **único** parcial `(aggregate_user_id) WHERE status=2` (backstop "1 em voo").
- `ClaimBatch` reivindica apenas eventos de usuários sem evento em voo e sem pendente anterior
  (`NOT EXISTS`), `ORDER BY occurred_at`, `FOR UPDATE SKIP LOCKED`, marcando status=2 na mesma
  transação curta. Colisão entre worker-1/2 esbarra no índice único → tratada como "adiar", não fatal.
- Eventos sistêmicos (`aggregate_user_id IS NULL`) não são serializados.

A serialização vive no **mecanismo do outbox** (genérico, chave opaca `aggregate_user_id`), não no
domínio — preserva R-WF-KERNEL-001. Nenhuma conexão é segurada durante o LLM.

## Alternativas Consideradas

1. **`pg_advisory_lock` de sessão por usuário** (reabilitar `advisory_key_locker`): rejeitada — quebra
   sob pgbouncer transaction pooling (lock vaza/solta entre clientes); já é o mecanismo desligado.
2. **`pg_advisory_xact_lock` abrangendo todo o handler:** correto sob transaction pooling, mas mantém
   transação aberta durante o LLM (~3s) → esgota `default_pool_size=15` com ~15 usuários simultâneos.
   Rejeitada por não escalar; aceitável só como lock auxiliar curtíssimo.
3. **Broker externo com partição por usuário (Kafka/NATS):** resolve ordenação nativamente, mas
   introduz componente de infra, custo e operação — fora de escopo (PRD) e desnecessário até >10k.
4. **Partição física por hash de usuário (N filas):** mais throughput, mais complexidade; reservada
   como evolução da fase 2.000–10.000 se o `NOT EXISTS` mostrar contenção.

## Consequências

### Benefícios Esperados

- Ordem FIFO por usuário garantida mesmo com N workers → elimina fora-de-ordem e respostas incoerentes.
- Sem conexão segura durante o LLM → escala no pool; suporta escalonamento horizontal (D-06).
- Sem novo componente de infra; muda apenas SQL do adapter e uma migration.

### Trade-offs e Custos

- `ClaimBatch` fica mais caro (subconsultas `NOT EXISTS`); mitigado pelos índices parciais.
- Complexidade de SQL maior no adapter (aceitável; isolado no Postgres, R-ADAPTER-001).
- Throughput por usuário limitado a 1 evento em voo (desejado: é a própria garantia de ordem).

### Riscos e Mitigações

- **Risco:** contenção/latência sob alto fan-out. **Mitigação:** índices parciais; métrica de lag
  p95; evolução para partição por hash se necessário. **Rollback:** reverter `ClaimBatch` ao
  `ORDER BY next_attempt_at` e dropar índices (migration down) — comportamento antigo restaurado.
- **Risco:** colisão no índice único em voo vazar como erro. **Mitigação:** o `UPDATE ... FROM
  claimable` é atômico por statement — uma violação `SQLSTATE 23505` aborta o lote inteiro, não só a
  linha. O `ClaimBatch` DEVE capturar o 23505 e **descartar o lote**, reivindicando no próximo tick
  (evento segue pendente). Colisão é rara (`FOR UPDATE SKIP LOCKED` + `NOT EXISTS`); perder 1 tick é
  aceitável (D-14).
- **Risco:** poison head-of-line — evento inbound com falha permanente e `occurred_at` anterior
  (status=1) bloqueia os seguintes do usuário via `NOT EXISTS`. **Mitigação:** `max_attempts`/backoff
  dos eventos inbound dimensionados para dead-letter (`status=4`, excluído do bloqueio) em ~1 turno;
  alerta em `status=4 > 0` (RF-22/D-19). FIFO estrito preservado.

## Plano de Implementação

1. Migration `000003` com os dois índices parciais (`IF NOT EXISTS`, sem downtime).
2. Reescrever `ClaimBatch` (CTE `claimable` + UPDATE ... RETURNING) no adapter Postgres.
3. Propagar `msg.Timestamp` (Meta) ao `OccurredAt` do evento (RF-18) para o FIFO refletir o usuário.
4. Testes de integração de concorrência (testcontainers): 1 em voo por usuário; paralelismo entre
   usuários; ordenação por `occurred_at`; evento sistêmico não bloqueia.
5. Ensaio sob carga e observação do lag p95.

Concluído quando: CA-01 verde e lag p95 < 5s sob carga sintética.

## Monitoramento e Validação

- Métricas: lag `occurred_at → published_at` (p95 < 5s; alerta > 30s), reivindicações adiadas por
  "usuário em voo", 0 duplicidade.
- Dashboards Grafana (otel-lgtm). Reverter se lag p95 degradar sem ganho de ordem.

## Impacto em Documentação e Operação

- Runbook do outbox (novo comportamento de claim), dashboards de lag, `docs/runs` do diagnóstico.

## Revisão Futura

- Revisar na fase 2.000–10.000 (partição física por hash) ou se o lag p95 exceder o SLO de forma
  sustentada. Substituível por ADR de particionamento físico sem reescrever o restante.
