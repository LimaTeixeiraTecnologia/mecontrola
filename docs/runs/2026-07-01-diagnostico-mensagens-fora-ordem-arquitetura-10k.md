# Diagnóstico — Mensagens fora de ordem no fluxo WhatsApp e arquitetura para 10k usuários

- Data do incidente: 2026-07-01 (janela ~19:59–22:58 UTC)
- Recriado/consolidado: 2026-07-02 (verificação contra código no commit `571425f` e contra o banco de
  produção `srv1761537` / imagem `mastra-20260629` via SSH read-only)
- Usuário do incidente: `06edc407-…` (único usuário com tráfego real)
- Consome: origem do PRD `.specs/prd-whatsapp-ordenacao-idempotencia/prd.md` (v3) e ADRs 001–005

## Sintomas observados

- Mensagens do WhatsApp respondidas **fora de ordem**; respostas incoerentes com a última mensagem.
- **Reinício repetido do onboarding** para usuário que já havia respondido (~68% `onboarding_error` na
  janela).
- **Confirmações de lançamento sem efeito real** ("registrado com sucesso" sem persistir) e, por vezes,
  **mensagens vazias** enviadas ao usuário.
- Ausência de traces do caminho inbound no Tempo; label de versão de telemetria divergente do binário.

Um **deploy storm** (múltiplas tags em ~27 min) foi o gatilho que expôs fraquezas permanentes — não a
causa raiz.

## Causas-raiz (com evidência de código)

1. **Ausência de serialização por usuário.** `internal/platform/outbox/storage_postgres.go` `ClaimBatch`
   (linhas 51–122) reivindica com `ORDER BY next_attempt_at ... FOR UPDATE SKIP LOCKED`, **sem partição
   por `aggregate_user_id`**. Com 2 workers a 500ms, eventos do mesmo usuário rodam concorrentes e fora
   de ordem. → ADR-001 (claim particionado).
2. **Advisory lock de sessão, desligado e incompatível com pgbouncer.**
   `internal/agents/infrastructure/persistence/advisory_key_locker.go:32` usa `pg_advisory_lock`
   (sessão), inseguro sob `pool_mode=transaction`; gated por `AGENT_WRITE_ADVISORY_LOCK` (default off).
   → ADR-002 (remoção + idempotência natural).
3. **Idempotência não à prova de corrida.** `internal/agents/application/usecases/idempotent_write.go` é
   **check-then-insert** (`FindByKey` → `write()` de domínio → `Insert`); o repositório
   `internal/agents/infrastructure/persistence/write_ledger_repository.go:64-67` faz
   `INSERT ... ON CONFLICT (wamid,item_seq,operation) DO NOTHING` — protege **só o ledger**, não o
   `write()` de domínio (que vive em outro módulo, sem tx compartilhada). Sob corrida, dupla mutação de
   domínio. → ADR-002 emenda v3 (chave natural + ledger-first + timeout).
4. **TOCTOU no início do onboarding.**
   `internal/agents/application/usecases/resolve_onboarding_or_agent.go` faz `Load`→checa marcador→
   `engine.Start` sem atomicidade; a 2ª `Start` concorrente viola o índice único parcial
   `workflow_runs_active_key_uidx (workflow, correlation_key) WHERE status IN ('running','suspended')` e
   retorna erro genérico → `onboarding_error`. → ADR-003 (Start idempotente-resume).
5. **Sucesso alucinado + resposta vazia.** `internal/platform/agent/agent.go` `invokeToolCall` engole
   erro de tool em `content=""`; `internal/platform/agent/runtime.go` hardcoda `RunStatusSucceeded` +
   `ToolOutcomeRouted`; `whatsapp_inbound_consumer.go` `sendReply` com `content==""` faz `return nil`
   (não envia, não erra). → ADR-002 (confirmação honesta via `ToolOutcome` + guarda de envio vazio).
6. **Webhook processa só a 1ª mensagem.** `internal/platform/whatsapp/payload/parser.go`
   `ExtractFirstMessage` retorna `Messages[0]` e descarta o resto. → ADR-005.
7. **Ordenação por `now()`, não pelo timestamp da Meta.** `internal/platform/whatsapp/dispatcher/`
   usa `time.Now().UTC()` no `OccurredAt`. → ADR-005 + RF-18.
8. **Traces derrubados e versão errada.** `OTEL_TRACE_SAMPLE_RATE=0.1` descarta 90%;
   `OTEL_SERVICE_VERSION` cai no default `"dev"` (não setado no `compose.swarm.yml`). → ADR-004.
9. **Deploy sem grace period + storm.** `compose.swarm.yml` sem `stop_grace_period` (default 10s <
   ~15s de shutdown do app); `order: stop-first`, `parallelism:1`. → ADR-004 (grace 30s + gate CI
   anti-storm; mantém stop-first).

## Evidência de produção (SSH read-only, 2026-07-02)

- Topologia confirmada: `server-1/2`, `worker-1/2`, `postgres`, `pgbouncer` (`pool_mode=transaction`,
  `DEFAULT_POOL_SIZE=15`, `MAX_DB_CONNECTIONS=60`), `otel-lgtm`.
- `outbox_events`: **118 linhas, todas `status=3`** (publicadas), janela única 19:59–22:58 de
  2026-07-01; **1 único `aggregate_user_id`**; tipos: 41 `agents.whatsapp.inbound.v1`,
  41 `auth.principal_established`, 32 `platform.memory.embedding.index.v1`, mais singletons de
  onboarding/billing. **Zero eventos presos** (`status=2`) e **zero duplicata em voo**.
- `agents_write_ledger`: **0 linhas** — apesar de 16 mensagens `user` + 16 `assistant` em
  `platform_messages`. Evidência direta de "sucesso alucinado"/escrita que não chega ao ledger.
- Schema confirmado (sem drift com a migration local `000001`): `outbox_events` com `aggregate_user_id`
  (uuid nullable), `occurred_at` (timestamptz), `metadata` (jsonb), `status` smallint (1..4);
  `workflow_runs` (status **texto**), `platform_runs`, `platform_threads`, `platform_messages`
  (`thread_pk`), `agents_write_ledger` (UNIQUE `(wamid,item_seq,operation)`). Os índices do claim
  particionado (migration `000002`) **ainda não existem** e nascem vazios (0 linhas `status=2`).

**Conclusão de contexto:** produção está em **~0 tráfego real (1 usuário)**. A meta de 10k é
forward-looking; SLOs, pool e fases **não têm baseline de produção** e só são validáveis por **carga
sintética** (gate por fase — RF-23/D-20). As causas-raiz, porém, são estruturais e reproduzíveis no
código independentemente do volume.

## Remediação (resumo — detalhe nos ADRs)

| Causa | ADR | Requisitos |
|-------|-----|-----------|
| 1 Serialização por usuário | ADR-001 | RF-01/02/03/18/19 |
| 2/3 Idempotência à prova de corrida | ADR-002 (v3) | RF-04/05/20/21 |
| 5 Confirmação honesta + envio vazio | ADR-002 | RF-06/07/08 |
| 4 TOCTOU onboarding | ADR-003 | RF-09/10/11/12 |
| 8/9 Observabilidade + deploy seguro | ADR-004 | RF-13/14/15/16 |
| 6/7 Ingestão em lote + timestamp Meta | ADR-005 | RF-17/18 |
| Poison head-of-line | ADR-001 | RF-22 |
| Validação de escala | techspec (gate de carga) | RF-23 |

## Restrições preservadas

Sem broker externo, sharding ou cache distribuído; solução usa o outbox Postgres + claim particionado.
Respeita R-WF-KERNEL-001, R-AGENT-WF-001, R-ADAPTER-001 e a skill `go-implementation`. Compatível com
pgbouncer `pool_mode=transaction` (nada segura conexão durante o LLM).
