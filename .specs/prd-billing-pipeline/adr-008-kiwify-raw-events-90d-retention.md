# ADR-008 — Persistir Kiwify raw events em billing_kiwify_events com retention 90 dias

## Metadados

- **Título:** Kiwify raw events persistidos por 90 dias
- **Data:** 2026-06-05
- **Status:** Aceita
- **Decisores:** PO (jailton), arquitetura (AI)
- **Relacionados:** `.specs/prd-billing-pipeline/techspec.md` §5.1, §6.7, §8.4, RF-11 (auditoria), Q-03 (refund parcial)

## Contexto

PRD trata idempotência como "garantia de auditoria" (Restrições Técnicas). Q-03 (refund parcial) está travada como REFUNDED total no MVP, mas pode mudar — forensics de payload bruto seria a única maneira de re-decidir retroativamente. Logs efêmeros (slog rotacionado) não garantem o passado.

Trade-off: persistir tudo (storage + PII) vs depender de logs (forensics frágil).

## Decisão

Tabela `billing_kiwify_events(envelope_id PK TEXT, trigger TEXT, raw_body JSONB, received_at TIMESTAMPTZ, processed_at TIMESTAMPTZ NULL, signature_status TEXT CHECK IN ('valid','invalid','rotated'))` persistida **fora da transação** do use case (auditoria proativa antes mesmo da validação fim-a-fim).

Insert em **toda** request com assinatura válida e com inválida (campo `signature_status` discrimina). Não persistir requests rejeitadas por content-type/body-limit (filtradas no servidor antes de atingir o handler).

Housekeeping: `KiwifyEventsHousekeepingJob` registrado no `WorkerManager` (estilo `outbox.HousekeepingJob`), com `Schedule() = "@daily"`. Apaga rows com `received_at < now() - 90 days`. Configurável via `BILLING_KIWIFY_EVENTS_RETENTION_DAYS` (default 90).

## Alternativas Consideradas

1. **Só logar via slog.** Recusada — slog rotaciona/expira sem garantia; impróprio para auditoria contábil.
2. **Persistir apenas em caso de erro de processamento (dead-letter only).** Recusada — perde auditoria proativa; não cobre Q-03 retroativo nem investigação de comportamento dúbio.
3. **Persistir indefinidamente (sem housekeeping).** Recusada — crescimento sem limite, PII em DB indefinida; viola princípio de minimização (LGPD).
4. **Tabela em schema separado para isolamento de retenção.** Adiada — não traz ganho prático no MVP; pode entrar em E4 se a equipe legal exigir.

## Consequências

### Benefícios Esperados

- Auditoria proativa: qualquer evento dos últimos 90d é re-inspecionável.
- Forensics em incidente (cobranças duplicadas, refund disputado, payload inesperado).
- Suporte a Q-03 retroativo (se decidir reabrir, dá pra recalcular sobre os últimos 90d).
- Diagnóstico de regressão na Kiwify (mudanças não anunciadas no schema).

### Trade-offs e Custos

- +1 tabela com crescimento proporcional ao volume de webhooks. Para MVP (poucos milhares de eventos/dia), JSONB cabe em GB-orders, manageável.
- PII no banco por 90d (telefone, e-mail do customer no payload Kiwify). Mitigação: housekeeping diário + plano de anonimização programada em E4.
- Insert fora da transação adiciona uma escrita por webhook (2 inserts no caminho feliz: kiwify_events + processed_events).

### Riscos e Mitigações

- **R:** Crescimento explosivo (Kiwify retry storm). **M:** Housekeeping diário + alerta de tabela > N GB.
- **R:** Insert fora da transação falha (DB down) — afeta auditoria mas não bloqueia o processamento. **M:** Aceito; falha de insert vira erro 5xx do handler, Kiwify retry; pré-condição: DB up.
- **R:** PII em dump de DB exposto. **M:** RBAC restrito; backup criptografado (responsabilidade ops, fora do escopo do código); anonimização programada em E4.

## Plano de Implementação

1. Migration `0007_create_billing_kiwify_events.up.sql`.
2. `internal/billing/application/interfaces/kiwify_event_repository.go`.
3. `internal/billing/infrastructure/repositories/postgres/kiwify_event_repository.go`.
4. Handler `KiwifyWebhookHandler.Handle` insere antes de dispatch para use case; atualiza `processed_at` no fim (mesmo se idempotente no-op).
5. `internal/billing/infrastructure/jobs/handlers/kiwify_events_housekeeping_job.go` — `worker.Job` `@daily`.
6. Adicionar `BILLING_KIWIFY_EVENTS_RETENTION_DAYS` em `configs.BillingConfig` (default 90).

## Monitoramento e Validação

- Métrica `billing_kiwify_events_inserted_total{signature_status,trigger}`.
- Métrica `billing_kiwify_events_housekeeping_deleted_total`.
- Métrica `billing_kiwify_events_table_rows` (gauge, expor via probe).
- Alerta: tabela > 10M rows ou >10 GB → revisar retention/volume.

## Impacto em Documentação e Operação

- README operacional documenta retention e como inspecionar evento histórico via SQL.
- Política LGPD (em E4) precisará atualizar a anonimização para incluir esta tabela.

## Revisão Futura

- Reabrir em E4 para integrar com anonimização programada.
- Reabrir se o volume crescer além do esperado (avaliar particionamento por mês ou movimento para object storage).
- Reabrir se a equipe legal exigir retention menor (ex.: 30d).
