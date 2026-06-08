# ADR-011 — Bind cross-module + cap de retry no consumer + secrets via systemd + log shipping Loki

## Metadados

- **Título:** Quatro decisões operacionais para production-readiness do MVP
- **Data:** 2026-06-06
- **Status:** Aceita
- **Decisores:** PO (jailton), arquitetura (AI)
- **Relacionados:** `.specs/prd-onboarding-magic-token/techspec.md` §6.7, §6.9, §6.10; [ADR-003](./adr-003-contract-e2-to-e3-via-outbox-enriched.md), [ADR-009](./adr-009-deployment-vps-hostinger-with-reverse-proxy.md)

## Contexto

Quatro decisões residuais materiais ao MVP production-ready, agrupadas porque compartilham a mesma camada (cross-module + operação):

### A) Como o entitlement de E1 reprojeta após `onboarding.subscription_bound`
O `EntitlementProjector` atual (`internal/identity/.../subscription_event_projector.go`) consome 5 types `billing.subscription.{activated,renewed,past_due,canceled,refunded}` com payload tipado `SubscriptionEventPayload` que carrega `subscription_id, user_id, status, period_end, etc.`. O evento `onboarding.subscription_bound` carrega apenas `user_id, subscription_id, funnel_token_hash_prefix, bound_at, activation_path` — sem status nem period_end.

### B) Política de retry para token não encontrado no consumer
Race teórica: webhook Kiwify chega antes do COMMIT do `POST /checkout` (ou cliente forjou parâmetro). Consumer faz lookup, não acha token. Sem cap, retry infinito; com cap zero, perda silenciosa.

### C) Secrets na VPS Hostinger
Sem cofre managed integrado hoje. PRD não fixou ferramenta. Mínimo aceitável production-ready precisa ser definido.

### D) Log shipping para investigação externa
`devkit-go/pkg/observability/otel` já instrumenta. Mas sem decisão sobre onde os logs saem (apenas journald local OU shipping centralizado).

## Decisão

### A) Novo handler dedicado em E1: `SubscriptionBoundProjector`
- Novo arquivo `internal/identity/infrastructure/messaging/database/consumers/subscription_bound_projector.go`.
- Registrado no `events.Dispatcher` em `internal/identity/module.go` para type `onboarding.subscription_bound`.
- Recebe payload `SubscriptionBoundPayload{user_id, subscription_id, activation_path, ...}`.
- Faz SELECT na sub via **interface declarada no consumidor (E1)**: `SubscriptionReader.GetByID(ctx, sub_id) (Subscription, error)`. Implementação concreta vive em E2 (`internal/billing/application/usecases/get_subscription_read_view.go`) e é injetada no wiring de `cmd/worker`.
- Chama a mesma rotina interna de projeção que o handler existente (`projectEntitlement`), garantindo `INSERT ... ON CONFLICT (user_id) DO UPDATE` em `identity.entitlements`.
- Idempotente por design.

Trade-off aceito: +1 query SELECT por bind (custo desprezível); +1 interface cross-module respeitando R6 (interface no consumidor).

### B) Cap de 5 tentativas → degradação para `paid_without_token`
- `OnboardingConfig.MaxTokenLookupAttempts` default `5`.
- Consumer detecta cap via campo `attempt_count` do envelope outbox **se** o platform expor (verificar na primeira tarefa de implementação). Caso negativo, criar tabela auxiliar `onboarding.consumer_lookup_attempts(event_id UUID PK, attempts INT NOT NULL DEFAULT 0, last_attempt_at TIMESTAMPTZ)` na migration 0010 e gerenciar contador localmente com `INSERT ... ON CONFLICT DO UPDATE SET attempts = attempts + 1 RETURNING attempts`.
- Ao atingir o cap, consumer:
  1. Insere `support_signals(kind='paid_without_token', payload={..., note:"token_lookup_exhausted"})`.
  2. Emite métrica `onboarding_token_not_found_after_retries_total{result="degraded"}`.
  3. Emite log `slog.Warn("onboarding.consumer.token_lookup_exhausted", ...)`.
  4. **Acknowledge o evento** (retorna nil) → outbox marca processado.

Trade-off aceito: race extremamente rara (webhook Kiwify chegar antes do COMMIT do checkout local exigiria latência DB > backoff do dispatcher); cap de 5 dá ~5 minutos de window em backoff exponencial; após isso, suporte resolve manualmente via `support_signals`.

### C) Secrets via systemd `EnvironmentFile` + `chmod 600`
- Cada serviço tem arquivo de env separado:
  - `/etc/mecontrola/server.env` (modo `0640`, owner `root:mecontrola`)
  - `/etc/mecontrola/worker.env` (modo `0640`, owner `root:mecontrola`)
- Systemd units carregam via diretiva `EnvironmentFile=/etc/mecontrola/<svc>.env`.
- Processo Go (`viper`) lê env já populada.
- Rotação manual: editar arquivo, `systemctl reload mecontrola-server` (ou restart se reload não suportado pelo binário).
- Auditoria de leitura: `auditd` opcional na VPS (fora desta ADR).
- Backup do arquivo: incluído no backup geral da VPS, criptografado em destino (S3 ou similar).

Trade-off aceito: rotação manual; sem audit trail de leitura; vazável se VPS comprometida. Mitigação: `META_APP_SECRET_NEXT` permite rotação sem downtime (ADR-005); `KIWIFY_WEBHOOK_SECRET_NEXT` idem (ADR-002 E2). Evolução para cofre (Vault/Doppler/Infisical) após MVP, em E4 ou tarefa de hardening operacional.

### D) Log shipping: stdout → journald → Grafana Alloy → Loki
- Binários (server e worker) logam **JSON em stdout** via `slog` configurado com `LogFormat=json` (`devkit-go/observability` já suporta).
- Systemd captura stdout no `journald` automaticamente.
- **Grafana Alloy** (sucessor de Promtail) roda como systemd unit na mesma VPS, lê do `journald` (`loki.source.journal`) e envia para **Grafana Cloud Loki** (tier gratuito ~50GB/mês cobre MVP).
- Labels Alloy: `service=<server|worker>`, `env=<dev|staging|prod>`, `module=onboarding|billing|identity|platform`.
- Métricas Prometheus expostas via `/metrics` no server (já existente em `devkit-go/observability`); Alloy também faz scrape e remote-write para Grafana Cloud Mimir.
- Traces OTLP: exporter já configurado (`OTLPEndpoint`); apontar para endpoint OTLP do Grafana Cloud Tempo.

Configuração Alloy minimal (referência):
```hcl
loki.source.journal "system" {
  forward_to = [loki.write.grafana_cloud.receiver]
  labels     = { job = "mecontrola" }
}
loki.write "grafana_cloud" {
  endpoint { url = sys.env("LOKI_URL"); basic_auth { username = sys.env("LOKI_USER"); password = sys.env("LOKI_TOKEN") } }
}
```

Trade-off aceito: dependência externa (Grafana Cloud) — gratuita no tier MVP; +1 systemd unit (Alloy); cota Loki monitorar.

## Alternativas Consideradas

### Para (A)
1. **E3 republica `billing.subscription.activated`** — recusada: viola fronteira, confunde origem do evento, infla métricas billing.
2. **E2 escuta `subscription_bound` e republica billing event** — recusada: 3 saltos de mensageria por ativação, latência maior, complexidade.

### Para (B)
1. **Falhar na 1ª tentativa (sem retry)** — recusada: race perdida.
2. **Retry indefinido** — recusada: outbox cresce, métricas inflam, sem auto-resolução.

### Para (C)
1. **HashiCorp Vault / 1Password Connect / AWS Secrets Manager** — recusada para MVP: complexidade operacional desproporcional.
2. **Doppler / Infisical** — recusada para MVP: dependência externa nova; reavaliar após MVP.

### Para (D)
1. **Só journald local** — recusada: SSH para investigar não é production-ready.
2. **Arquivos + rsync diário** — recusada: latência de investigação alta; busca difícil.
3. **OTLP direto sem Alloy** — viável (devkit-go já exporta), mas Loki via Alloy + journald é mais resiliente: se backend Go cair, journald ainda captura crashes (`stderr`); OTLP direto perde logs do crash.

## Consequências

### Benefícios
- Bind cross-module respeitando fronteiras (R6).
- Cap de retry previne loops infinitos sem perder caminho de suporte.
- Secrets protegidos no mínimo aceitável production-ready com paths de evolução.
- Logs centralizados desde dia 1 facilitam investigação.

### Trade-offs
- +1 interface cross-module (`SubscriptionReader`) — wiring no `cmd/worker`.
- Possível tabela auxiliar `consumer_lookup_attempts` se platform não expõe `attempt_count`.
- Rotação manual de secrets até cofre futuro.
- Dependência externa Grafana Cloud (mitigável trocando backend OTLP-compatible).

### Riscos e Mitigações
- **R:** SubscriptionReader de E2 retorna sub sem `user_id` (porque bind ainda não foi aplicado em billing). **M:** Onboarding atualiza `billing_subscriptions.user_id` na mesma UoW do consume (já descrito em §5.1 do techspec); SubscriptionReader devolve o estado pós-COMMIT.
- **R:** Tabela `consumer_lookup_attempts` cresce. **M:** Housekeeping mensal (`DELETE WHERE last_attempt_at < now() - 30d`) no mesmo job `MetaProcessedMessagesCleanup`.
- **R:** Arquivo `.env` versionado por engano. **M:** `.gitignore` em `/etc/mecontrola/` é file system não-git; runbook explicita "nunca commitar". CI lint para repos de config (se houver).
- **R:** Grafana Cloud Loki quota estourada. **M:** Alerta de uso em 80%; downgrade de log level via env (`O11Y_LOG_LEVEL=warn`) absorve picos sem deploy.

## Plano de Implementação
1. Validar se `internal/platform/outbox` expõe `attempt_count`; se não, migration 0010 inclui `consumer_lookup_attempts`.
2. Criar `SubscriptionBoundProjector` em E1 + interface `SubscriptionReader` em E1, implementação concreta em E2 (`get_subscription_read_view.go`).
3. Registrar handler `onboarding.subscription_bound` em `internal/identity/module.go`.
4. Implementar lógica de cap em `SubscriptionPaidConsumer` (E3).
5. Criar runbook `docs/runbooks/deployment-vps-hostinger.md` cobrindo systemd units + EnvironmentFile + Alloy config (fora desta techspec; tarefa operacional separada).

## Monitoramento
- `onboarding_token_not_found_after_retries_total{result}` (rate baixo esperado).
- `outbox_consumer_failures_total{type, consumer}` (já existente).
- Quota Loki/Mimir via dashboard Grafana Cloud.
- `meta_signature_invalid_total`, `onboarding_confirmation_failed_total`, demais métricas já listadas em techspec §9.2.
