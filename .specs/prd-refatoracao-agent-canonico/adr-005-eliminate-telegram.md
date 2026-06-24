# ADR-005 — Eliminação Total do Canal Telegram

## Metadados

- **Título:** WhatsApp Oficial da Meta como canal único; remoção completa de Telegram (código + schema)
- **Data:** 2026-06-24
- **Status:** Aceita
- **Decisores:** Solicitante + plataforma
- **Relacionados:** PRD (RF-01..05), techspec §"Eliminação Telegram", migration `000020`

## Contexto

O produto-alvo é **WhatsApp Oficial da Meta como canal único**. O Telegram tem pegada ampla
(plataforma, consumer, gateways, onboarding-Telegram, config/env, colunas/constraints de schema) e
aumenta superfície de manutenção/ataque sem servir ao alvo. O solicitante autorizou a remoção total
documentando os motivadores.

## Decisão

Remover integralmente o canal Telegram em três frentes:

1. **Deletar arquivos Telegram-only**: árvore `internal/platform/telegram/**`; consumer
   `telegram_inbound_consumer.go` + testes; e2e `f04_telegram_agent_flow.feature` + steps;
   onboarding-Telegram (`activate_telegram_by_token.go`, `telegram_message_processor.go`,
   `direct_telegram_activation_workflow.go`, `infrastructure/telegram/route.go`);
   `internal/platform/notification/adapters/telegram.go`;
   `internal/identity/infrastructure/http/server/telegram_router.go`;
   `cmd/server/telegram_wiring.go`; `configs/validate_production_telegram_agent_test.go`;
   scripts/diagramas/runbooks Telegram-only.
2. **Editar arquivos compartilhados** (remover ramo Telegram, preservar WhatsApp): `intent_router.go`
   (`RouteTelegram`/`TelegramOutbound`/`TelegramTo`/`ChannelTelegram`); Channel VO
   (`channel.go`/`external_id.go`); `resolve_preferred_channel.go`; `principal.go` (`SourceTelegram`);
   `notification/channel.go`; `magic_token.go` (+cascata repo/DTO/handler do `telegram_external_id`);
   `send_outreach.go`; `onboarding_session.go`/`start_budget_configuration.go`
   (`OnboardingChannelTelegram`); `agent/infrastructure/onboarding/budget_configurator.go`
   (`mapAgentChannelToOnboarding`); `inbound_event_publisher.go`
   (`EventTypeTelegramInbound`/`PublishTelegram`); `agent/module.go` e `onboarding/module.go`;
   `internal/bootstrap/channel.go`; `cmd/server/server.go`; `cmd/worker/worker.go`; `configs/config.go`
   (`TelegramConfig`/`validateProductionTelegram`/`setTelegramDefaults`); `.env.example`;
   `platform/channels/activation_command.go` (regex de deep link `/start ATIVAR_` é Telegram-only);
   OpenAPIs.
3. **Migration `000020_drop_telegram_channel`** (não editar baseline): drop coluna
   `onboarding_tokens.telegram_external_id` + índice parcial; recriar 3 CHECK constraints
   (`channel_processed_messages`, `user_identities`, `onboarding_sessions`) de
   `IN ('whatsapp','telegram')` → `IN ('whatsapp')`. `down` restaura o estado do baseline.

**Premissa aceita (decisão):** **zero usuários Telegram reais em produção** (canal era piloto). A
migration limpa apenas dedup residual (`DELETE FROM mecontrola.channel_processed_messages WHERE
channel='telegram'`) antes do `ADD CONSTRAINT`. **Verificação pré-deploy obrigatória (fail-fast):**
`SELECT count(*) FROM mecontrola.user_identities WHERE channel='telegram'` e idem
`onboarding_sessions`; se **> 0**, abortar o deploy e escalar (a premissa foi violada) — não aplicar a
migration às cegas. Esse check é um passo do runbook de release, não da migration em si.

**Fora desta ADR (decisão tomada — manter):** `ALERT_TELEGRAM_*` /
`deployment/telemetry/grafana/setup-alerting-telegram.sh` é notificação de **alerta do Grafana**
(observabilidade), **não** o canal de produto. Decisão do solicitante: **manter** — este PRD/ADR
**não** toca o alerting Telegram do Grafana. Eventual descontinuação será item separado.

### Migration up (resumo — versão completa no arquivo SQL)

```sql
DROP INDEX IF EXISTS mecontrola.onboarding_tokens_telegram_external_id_idx;
ALTER TABLE mecontrola.onboarding_tokens DROP COLUMN IF EXISTS telegram_external_id;
-- para cada uma das 3 tabelas: DROP CONSTRAINT ... ; ADD CONSTRAINT ... CHECK (channel IN ('whatsapp'));
```

## Alternativas Consideradas

- **Manter Telegram em paralelo**: contraria o alvo de canal único; mantém superfície. Rejeitada.
- **Desativar via flag sem remover**: mantém código morto e dívida; o solicitante pediu remoção total
  com motivadores. Rejeitada.

## Consequências

### Benefícios Esperados

- Menor superfície de manutenção/ataque; remoção de ramos condicionais por canal; base focada.

### Trade-offs e Custos

- Migração de schema com data-cleanup; ajuste/remoção de muitos testes; perda do canal Telegram
  (intencional).

### Riscos e Mitigações

- **Risco:** premissa "zero usuários Telegram" estar errada → `ADD CONSTRAINT` falha no deploy.
  **Mitigação:** verificação pré-deploy fail-fast (contagem > 0 aborta) + limpeza de dedup residual;
  `down` reverte schema. **Rollback:** migration `down` + restaurar arquivos via VCS.
- **Risco:** quebra de build por referência esquecida. **Mitigação:** ordem de execução
  (editar shared → deletar → migration → testes → `go build/test`) + grep amplo case-insensitive por
  `telegram`.

## Plano de Implementação

1. Editar compartilhados. 2. Deletar Telegram-only. 3. Migration 000020 + data-cleanup. 4. Ajustar/
   remover testes (lista no blueprint). 5. `go build ./...` + `go test ./...` + integração de migrations.

## Monitoramento e Validação

- Sucesso: grep `telegram` (case-insensitive) em `internal/`/`cmd/`/`configs/` retorna só refs de
  observabilidade (categoria B) ou nada; build/test verdes; migração up/down idempotente.

## Impacto em Documentação e Operação

- Remover/atualizar diagramas, runbooks Telegram, OpenAPIs, `.env.example`; comunicar fim do canal.

## Revisão Futura

- Reabrir só se houver decisão de produto de suportar novo canal (novo PRD).
