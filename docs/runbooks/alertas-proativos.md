# Runbook — Alertas Proativos (Release 1)

Spec: `.specs/prd-alertas-proativos/`. Status de templates Meta: `.specs/prd-alertas-proativos/meta-templates-status.md`.

## Escopo do Release 1

Somente quatro alertas são emissíveis:

| Kind | Prioridade | Categoria Meta | Template default |
|---|---|---|---|
| `category_threshold_100` | 1 | UTILITY | `mecontrola_category_threshold_100` |
| `budget_not_reviewed_day_3` | 2 | MARKETING | `mecontrola_budget_not_reviewed_day_3` |
| `budget_missing_month_start` | 3 | UTILITY | `mecontrola_budget_missing_month_start` |
| `category_threshold_80` | 4 | UTILITY | `mecontrola_category_threshold_80` |

`category_threshold_90`, `goal_achieved` e o kind legado `category_threshold` **não são emissíveis**.
O bloqueio de 90 é duplo: `services.IsReleaseOneEmittable` rejeita o kind e
`budgets_threshold_states_threshold_chk` só aceita `(80, 100)`. Liberar 90 exige migration própria
(ADR-003), não apenas mudança de configuração.

No máximo **um alerta por usuário por rodada diária**; os demais são suprimidos com motivo `priority`.

## Estado atual do rollout

**Dry-run é o default de produção.** `BUDGETS_THRESHOLD_ALERTS_DRY_RUN=true` avalia, prioriza,
metrifica e loga, mas não publica outbox, não grava dedup e não chama a Meta.

**Nenhum kind está liberado para envio real por default.**
`BUDGETS_THRESHOLD_TEMPLATES_APPROVED_KINDS` nasce vazio: mesmo com dry-run desligado, todo envio é
suprimido com motivo `template_unapproved` até o operador listar o kind explicitamente.

**`budget_not_reviewed_day_3` está permanentemente suprimido no Release 1.** A Meta classifica o
template como `MARKETING`, o que exige opt-in explícito (RF-18), e o repositório ainda não possui
fonte de consentimento — `alerts.DenyAllMarketingConsent` sempre nega. O alerta é avaliado e
contabilizado, mas nunca entregue. Para liberá-lo é preciso uma das duas ações:
reclassificar o template para `UTILITY` na Meta, ou implementar armazenamento de consentimento e
trocar o adapter de `MarketingConsentReader`.

**Quiet hours não usa timezone por usuário.** `mecontrola.users` não tem coluna de timezone;
`alerts.FallbackTimezoneResolver` devolve `BUDGETS_THRESHOLD_ALERTS_TIMEZONE_FALLBACK`
(`America/Sao_Paulo`) para todos. Quando existir timezone por usuário, basta trocar o adapter — o
use case já consome a porta `UserTimezoneResolver`.

## Procedimento para habilitar envio real por kind

1. Consultar o status do template na Meta Graph API. **Nunca** colar `META_ACCESS_TOKEN` em ticket,
   log, PR ou documento; use a variável de ambiente já provisionada no host.

   ```bash
   curl -s -G "https://graph.facebook.com/v20.0/${META_WABA_ID}/message_templates" \
     --data-urlencode "name=mecontrola_category_threshold_80" \
     -H "Authorization: Bearer ${META_ACCESS_TOKEN}" \
     | jq -r '.data[] | "\(.name)\t\(.status)\t\(.category)\t\(.language)"'
   ```

2. Confirmar `status == APPROVED` e anotar a `category` retornada. Se a Meta devolver `MARKETING`,
   o kind exige opt-in e **não** deve ser adicionado à lista de aprovados enquanto não houver fonte
   de consentimento.

3. Atualizar `.specs/prd-alertas-proativos/meta-templates-status.md` com a data da consulta.

4. Adicionar o kind à allowlist e reiniciar o worker:

   ```
   BUDGETS_THRESHOLD_TEMPLATES_APPROVED_KINDS=category_threshold_80,category_threshold_100
   ```

   A configuração rejeita kinds fora do Release 1 na subida (`validateBudgets`), então um typo
   derruba o processo em vez de silenciosamente não enviar.

5. Rodar o check de readiness antes de liberar. Ele cruza o quadro de status Meta com a allowlist e
   **falha com exit 1** em qualquer inconsistência:

   ```bash
   go run ./cmd/tools/audit-alert-readiness \
     --approved-kinds="category_threshold_80,category_threshold_100"
   ```

   Use `--json` para consumo em pipeline e `--consent-source=true` apenas quando existir, de fato,
   armazenamento de consentimento implantado. O check bloqueia: kind fora do Release 1 na allowlist,
   template não-`APPROVED` na allowlist, e kind `MARKETING` liberado sem fonte de consentimento.

6. Só então desligar o dry-run: `BUDGETS_THRESHOLD_ALERTS_DRY_RUN=false`. Com dry-run desligado, a
   config passa a exigir que os quatro nomes de template estejam preenchidos.

## Readiness por kind (auditoria)

`cmd/tools/audit-alert-readiness` é a fonte auditável de "o que está liberado e por quê". Com os
defaults de produção (allowlist vazia, sem fonte de consentimento) **nenhum kind é entregável** —
inclusive `weekly_motivation` e `usage_reactivation_3d`, que aparecem explicitamente bloqueados por
estarem fora do Release 1 e por exigirem opt-in `MARKETING`. É esse relatório que torna o gate do
RF-05 verificável em vez de implícito.

## Métricas

Todas com cardinalidade controlada. Labels permitidos: `kind`, `reason`, `channel`, `outcome`,
`status`. **Nunca** adicionar `user_id`, `budget_id`, telefone ou `message_id` como label.

| Métrica | Labels | Onde |
|---|---|---|
| `proactive_alerts_evaluated_total` | `kind`, `outcome` | `EvaluateThresholdAlerts` |
| `proactive_alerts_suppressed_total` | `kind`, `reason` | `EvaluateThresholdAlerts`, `NotifyThresholdAlert` |
| `proactive_alerts_queued_total` | `kind` | `EvaluateThresholdAlerts` |
| `proactive_alerts_dry_run_total` | `kind`, `outcome` | `EvaluateThresholdAlerts` |
| `proactive_alerts_notified_total` | `kind`, `channel`, `outcome` | `NotifyThresholdAlert` |
| `proactive_alerts_template_status` | `kind`, `status` | `NotifyThresholdAlert` |

Motivos de supressão (`reason`): `priority`, `frequency`, `no_channel`, `opt_in_missing`,
`quiet_hours`, `template_unapproved`, `kind_blocked`.

## Diagnóstico

**Dry-run ligado e nada acontece** — esperado. Confira `proactive_alerts_dry_run_total{outcome="skipped"}`;
se estiver zerado, o problema é elegibilidade, não envio.

**Dry-run desligado e nada é entregue** — verifique nesta ordem:
`proactive_alerts_template_status{status!="APPROVED"}` (kind fora da allowlist),
`proactive_alerts_suppressed_total{reason="opt_in_missing"}` (template MARKETING),
`{reason="quiet_hours"}` (janela 20:00–08:00), `{reason="no_channel"}` (usuário sem WhatsApp).

**Alerta parece perdido durante a noite** — quiet hours suprime **sem** marcar `notified_at`, então
o alerta volta a ser elegível na próxima rodada fora da janela. Não é perda.

**Falha da Meta** — `proactive_alerts_notified_total{outcome="channel_failed"}`. `notified_at` **não**
é gravado nesse caminho; o outbox reprocessa. Nunca há sucesso falso.

## Invariantes que não podem regredir

- Envio proativo fora da janela usa **exclusivamente** `SendTemplate`. Texto livre (`SendText`) não é
  fallback e é assertado em teste (`notify_threshold_alert_test.go`).
- `MarkNotified` acontece **depois** do aceite do gateway, nunca antes.
- Dry-run não publica outbox, não grava dedup e não chama a Meta.
- Dedup por `(user_id, budget_id, kind, ref_day)`: 80 e 100 são kinds distintos e não colapsam.
- `SendText` e `SendActivationTemplate` seguem funcionando para onboarding e outreach.

## Follow-up agentivo

Após envio real confirmado, `NotifyThresholdAlert` chama `AlertContextRecorder.Record`, que:

1. resolve a thread do usuário via `ThreadGateway.GetOrCreate(userID, telefone)`;
2. anexa uma mensagem `assistant` no `MessageStore` descrevendo o alerta e o próximo passo esperado;
3. grava `last_proactive_alert` (`kind`, `sent_at`, `follow_up_topic`) na working memory.

O runtime já injeta working memory e as últimas mensagens no system prompt, então um "sim" do usuário
é resolvido contra esse contexto pelas ferramentas existentes — sem workflow novo e sem switch por kind.

**Expiração:** `HandleInbound` chama `PurgeExpired` **antes** de `AgentRuntime.Execute`. Se
`now - sent_at > BUDGETS_THRESHOLD_ALERT_CONTEXT_TTL` (default 24h), a chave é zerada e o contexto não
chega ao prompt — o agente não tem do que inferir intenção e pede esclarecimento. A decisão é
determinística, sem LLM.

Falha ao gravar ou expirar contexto **nunca** invalida o envio nem bloqueia o inbound: é logada e
seguida adiante, porque a mensagem já foi entregue ao usuário.
