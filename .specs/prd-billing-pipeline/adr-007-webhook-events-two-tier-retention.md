# ADR-007 — Retenção two-tier de `webhook_events` com anonimização irreversível em-place

## Metadados

- **Título:** Política de retenção e anonimização de webhook_events
- **Data:** 2026-06-03
- **Status:** Aceita
- **Decisores:** Equipe de produto + jurídico (LGPD) + plataforma
- **Relacionados:** `prd-billing-pipeline/prd.md` (RF-48..52, CA-12, D-08), `techspec.md` §Schema Postgres

## Contexto

`webhook_events.payload` (JSONB) contém PII bruto: CPF, email, telefone, endereço, dados de cartão. Trade-offs:
- LGPD: minimização de retenção de dados pessoais.
- Obrigação fiscal BR: pagamentos devem ser retidos ~5 anos.
- Valor forense: payload é gold para disputas Kiwify (chargeback prazo ~120 dias).
- Storage: tabela cresce indefinidamente sem rotação.

PRD D-08 decidiu: **two-tier — 365d íntegro + anonimização irreversível em-place** preservando metadados indefinidamente. Sem hard delete no MVP.

## Decisão

1. **Janela de payload íntegro:** 365 dias a partir de `received_at`. Cobre chargeback Kiwify (~120d) + ciclo anual completo + buffer para disputas tardias.
2. **Anonimização irreversível em-place:** após 365 dias, job diário `billing.webhook_events.anonymize` substitui `payload` (JSONB) por versão com PII redactada. Campos canônicos redactados:
   - `customer.cpf`, `customer.cnpj`
   - `customer.email`
   - `customer.mobile`
   - `customer.address.*` (todos os subcampos)
   - `card.*` (todos os subcampos)
   - `payment.*.card.*`

   Substituídos pela string literal `"[REDACTED]"`. Demais campos (`product`, `tracking`, `subscription`, datas, valores) preservados.
3. **Metadados preservados indefinidamente:** `id`, `provider`, `external_event_id`, `event_type`, `signature`, `headers`, `received_at`, `processed_at`, `anonymized_at`. Sem janela de purga.
4. **Coluna `anonymized_at TIMESTAMPTZ NULL`** sinaliza estado (NULL = íntegro, NOT NULL = anonimizado). Índice parcial `WHERE anonymized_at IS NULL` acelera varredura.
5. **Idempotência:** segunda execução do job na mesma linha é no-op (filtro `anonymized_at IS NULL` exclui).
6. **Job:** schedule `@daily` configurável (`BILLING_ANONYMIZATION_SCHEDULE`), batch 500 linhas/execução (configurável `BILLING_ANONYMIZATION_BATCH_SIZE`), retention threshold configurável (`BILLING_ANONYMIZATION_RETENTION_DAYS=365`).

## Alternativas Consideradas

### Hard delete após 90 dias

- Vantagem: máxima minimização LGPD.
- Desvantagem: forense limitada para disputas tardias; perde alinhamento com retenção fiscal.
- Rejeitada por janela curta demais para disputas.

### Retenção 5 anos integral (sem anonimização)

- Vantagem: maximiza forense + cumpre fiscal.
- Desvantagem: PII retida muito além do necessário; risco LGPD alto.
- Rejeitada por exposição.

### Hard delete pós-anonimização (após 5 anos do anonimizado)

- Vantagem: storage não cresce indefinidamente.
- Desvantagem: metadados são úteis para análise histórica; tamanho de uma linha sem payload é pequeno (~200 bytes).
- Rejeitada para MVP; revisitar em E4.

### Dump anonimizado para cold storage (S3) + delete

- Vantagem: separa hot/cold; menor footprint no Postgres.
- Desvantagem: complexidade operacional + nova dependência externa.
- Rejeitada por overhead para MVP.

## Consequências

### Benefícios Esperados

- LGPD compliance: PII removida após período razoável.
- Forense + fiscal: 365d cobrem chargeback Kiwify + disputas.
- Metadados auditáveis indefinidamente.
- Implementação simples e idempotente.

### Trade-offs e Custos

- Tabela cresce em linhas-metadados indefinidamente — ~200 bytes/linha; em 5 anos com 10k webhooks/dia ≈ 3.6 GB.
- Anonimização irreversível: forense pós-365d não recupera payload original. Documentado.

### Riscos e Mitigações

- **Risco:** anonimização rodando concorrente com `ProcessBillingEventUseCase` lendo `payload` via `FindRawPayload`. **Mitigação:** processor lookup é no caminho de webhook recém-chegado (`received_at < 5min`), bem antes do threshold de 365d. Race window é zero na prática. Se concorrer, processor lê `[REDACTED]` e `ParseEvent` falha com `ErrPayloadDecode` → DLQ → operador inspeciona.
- **Risco:** schema do payload Kiwify evoluir, novos campos PII surgirem. **Mitigação:** lista de paths em código com testes de regressão; PR de schema evolution deve incluir review do redaction list.

## Plano de Implementação

1. Migration `0009_billing_schema.up.sql` cria `webhook_events` com coluna `anonymized_at` + índice parcial.
2. `WebhookEventRepository.ListPendingAnonymization(olderThan, limit)` retorna linhas elegíveis.
3. `WebhookEventRepository.Anonymize(id, newPayload, at)` executa `UPDATE webhook_events SET payload = $2, anonymized_at = $3 WHERE id = $1 AND anonymized_at IS NULL` (idempotente).
4. `AnonymizeWebhookEventsUseCase` percorre batch, para cada linha aplica `redactor.Strip(payload)` (função pura), chama `Anonymize`.
5. `redactor.Strip(json.RawMessage) (json.RawMessage, error)` itera lista de paths e aplica substituição.
6. Integration test (CA-12): popula linhas com `received_at = NOW() - INTERVAL '366 days'`, executa job, verifica PII redactada e timestamp.

## Monitoramento e Validação

- Métrica `billing_webhook_events_anonymized_total` (counter) por execução.
- Métrica `billing_webhook_events_pending_anonymization` (gauge) consulta `count(*) WHERE received_at < NOW() - INTERVAL '365 days' AND anonymized_at IS NULL`.
- Alerta em `pending_anonymization > 5000` sustentado 6h (sinal de job parado).

## Impacto em Documentação e Operação

- Runbook LGPD: documentar caminho de "Direito ao Esquecimento" — solicitação manual chama método admin que aplica `Anonymize` em todas as linhas com `customer.cpf = X` independente de `received_at`.
- AGENTS.md billing documenta a política e a lista de paths redactados.

## Revisão Futura

- Em E4, considerar hard delete pós-5y para reduzir storage de metadados.
- Reavaliar lista de paths a cada PR de schema evolution Kiwify.
