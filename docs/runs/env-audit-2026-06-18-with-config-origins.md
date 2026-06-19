# Relatório de Auditoria de Variáveis de Ambiente (com origem em configs/config.go) — 2026-06-18

**Servidor:** `187.77.45.48`
**Arquivo analisado:** `/opt/mecontrola/.env`
**Referência canônica:** `configs/config.go` (tags `mapstructure`) + `.env.example` + arquivos de infraestrutura (Compose, Caddy, scripts).

## Resumo executivo

| Métrica | Valor |
|---|---|
| Variáveis canônicas esperadas | 207 |
| Variáveis presentes no `.env` remoto (úteis) | 196 |
| Variáveis **faltantes** no servidor | 21 |
| Variáveis **em excesso** no servidor | 10 |
| Variáveis preenchidas no servidor | 168 |
| Variáveis vazias ou com placeholder no servidor | 18 |

## 1. Variáveis faltantes no servidor

> Variáveis que o código/infra espera e que **não estão definidas** em `/opt/mecontrola/.env`. A coluna **Origem em configs/config.go** mostra a struct, campo e linha onde a variável é declarada.

| Variável | Categoria | Origem em configs/config.go |
|---|---|---|
| `AGENT_LLM_PROSE_MAX_TOKENS` | Agent/LLM | configs/config.go:173 — AgentConfig.ProseMaxTokens |
| `BUDGETS_THRESHOLD_ALERTS_CRON` | Budgets | configs/config.go:109 — BudgetsConfig.ThresholdAlertsCron |
| `BUDGETS_THRESHOLD_ALERTS_MODE` | Budgets | configs/config.go:111 — BudgetsConfig.ThresholdAlertsMode |
| `BUDGETS_THRESHOLD_ALERTS_SCAN_LIMIT` | Budgets | configs/config.go:110 — BudgetsConfig.ThresholdAlertsScanLimit |
| `BUDGETS_THRESHOLD_CARD_RATIO` | Budgets | configs/config.go:114 — BudgetsConfig.ThresholdCardRatio |
| `BUDGETS_THRESHOLD_CATEGORY_RATIO` | Budgets | configs/config.go:112 — BudgetsConfig.ThresholdCategoryRatio |
| `BUDGETS_THRESHOLD_GOAL_RATIO` | Budgets | configs/config.go:113 — BudgetsConfig.ThresholdGoalRatio |
| `CARD_INVOICE_DUE_ALERTS_CRON` | Card | configs/config.go:88 — CardConfig.InvoiceDueAlertsCron |
| `CARD_INVOICE_DUE_ALERTS_ENABLED` | Card | configs/config.go:87 — CardConfig.InvoiceDueAlertsEnabled |
| `CARD_INVOICE_DUE_SCAN_LIMIT` | Card | configs/config.go:90 — CardConfig.InvoiceDueScanLimit |
| `CARD_INVOICE_DUE_WINDOW_DAYS` | Card | configs/config.go:89 — CardConfig.InvoiceDueWindowDays |
| `EMAIL_HTTP_TIMEOUT` | E-mail | configs/config.go:49 — EmailConfig.HTTPTimeout |
| `EMAIL_PROVIDER` | E-mail | configs/config.go:36 — EmailConfig.Provider |
| `EMAIL_REPLY_TO` | E-mail | configs/config.go:39 — EmailConfig.ReplyTo |
| `KIWIFY_WEBHOOK_RATE_LIMIT_BURST` | Kiwify | configs/config.go:221 — KiwifyConfig.WebhookRateLimitBurst |
| `KIWIFY_WEBHOOK_RATE_LIMIT_PER_MIN` | Kiwify | configs/config.go:220 — KiwifyConfig.WebhookRateLimitPerMin |
| `KIWIFY_WEBHOOK_TRUSTED_PROXIES` | Kiwify | configs/config.go:222 — KiwifyConfig.WebhookTrustedProxies |
| `ONBOARDING_TELEGRAM_DIRECT_ENABLED` | Onboarding | configs/config.go:140 — OnboardingConfig.TelegramDirectEnabled |
| `RESEND_API_KEY` | E-mail | configs/config.go:47 — EmailConfig.ResendAPIKey |
| `RESEND_BASE_URL` | E-mail | configs/config.go:48 — EmailConfig.ResendBaseURL |
| `SMTP_TIMEOUT` | E-mail | configs/config.go:46 — EmailConfig.SMTPTimeout |

## 2. Variáveis em excesso no servidor

> Variáveis presentes no `.env` remoto, mas **não reconhecidas** pela versão atual do código/infra.

| Variável | Valor mascarado | Sugestão |
|---|---|---|
| `AGENT_MODE` | ope***ter | Verificar se é usada por outro processo; se não for, remover. |
| `AGE_KEY_FILE` | CHANGE_ME_/etc/age/key.txt | Verificar se é usada por outro processo; se não for, remover. |
| `ALERT_CMD` | (vazio) | Verificar se é usada por outro processo; se não for, remover. |
| `LOKI_API_KEY` | CHANGE_ME_loki_api_key | Verificar se é usada por outro processo; se não for, remover. |
| `LOKI_URL` | CHANGE_ME_https://logs-prod-xxx.grafana.net/loki/api/v1/push | Verificar se é usada por outro processo; se não for, remover. |
| `LOKI_USER_ID` | CHANGE_ME_loki_user_id | Verificar se é usada por outro processo; se não for, remover. |
| `RESTORE_PORT` | 15432 | Verificar se é usada por outro processo; se não for, remover. |
| `SMOKE_SCHEMA` | mec***ola | Verificar se é usada por outro processo; se não for, remover. |
| `TELEGRAM_MSG_AGENT_STUB_RECEIVED` | MeC***ia. | Verificar se é usada por outro processo; se não for, remover. |
| `WA_MSG_AGENT_STUB_RECEIVED` | MeC***ia. | Verificar se é usada por outro processo; se não for, remover. |

## 3. Variáveis presentes em ambos

> Variáveis que estão no `.env` remoto e são reconhecidas pelo código/infra.

| Variável | Categoria | Status | Valor mascarado |
|---|---|---|---|
| `AGENT_LLM_CIRCUIT_COOLDOWN` | Agent/LLM | ✅ Preenchida | 60s |
| `AGENT_LLM_CIRCUIT_FAILURES` | Agent/LLM | ✅ Preenchida | 5 |
| `AGENT_LLM_CIRCUIT_WINDOW` | Agent/LLM | ✅ Preenchida | 30s |
| `AGENT_LLM_FALLBACK_MODELS` | Agent/LLM | ✅ Preenchida | ope***4.5 |
| `AGENT_LLM_HTTP_REFERER` | Agent/LLM | ✅ Preenchida | htt***app |
| `AGENT_LLM_MAX_TOKENS` | Agent/LLM | ✅ Preenchida | 256 |
| `AGENT_LLM_PRIMARY_MODEL` | Agent/LLM | ✅ Preenchida | goo***ite |
| `AGENT_LLM_PROMPT_PAD_TOKENS` | Agent/LLM | ✅ Preenchida | 1100 |
| `AGENT_LLM_REQUEST_TIMEOUT` | Agent/LLM | ✅ Preenchida | 8s |
| `AGENT_LLM_TEMPERATURE` | Agent/LLM | ✅ Preenchida | 0 |
| `AGENT_LLM_X_TITLE` | Agent/LLM | ✅ Preenchida | MeC***ola |
| `AGE_RECIPIENT` | Backup | ⚠️ Placeholder | CHANGE_ME_age1... |
| `ALERTMANAGER_FROM_EMAIL` | Observabilidade | ⚠️ Placeholder | CHANGE_ME_alerts@yourdomain.com |
| `ALERTMANAGER_SMTP_HOST` | Observabilidade | ⚠️ Placeholder | CHANGE_ME_smtp.yourdomain.com |
| `ALERTMANAGER_SMTP_PASSWORD` | Observabilidade | ⚠️ Placeholder | CHANGE_ME_smtp_password |
| `ALERTMANAGER_SMTP_USER` | Observabilidade | ⚠️ Placeholder | CHANGE_ME_smtp_user |
| `ALERTMANAGER_TO_EMAIL` | Observabilidade | ⚠️ Placeholder | CHANGE_ME_oncall@yourdomain.com |
| `ALERT_TELEGRAM_BOT_TOKEN` | Observabilidade | ✅ Preenchida | 885***-HY |
| `ALERT_TELEGRAM_CHAT_ID` | Observabilidade | ✅ Preenchida | 118***338 |
| `APP_DOMAIN` | Infraestrutura | ✅ Preenchida | api***.br |
| `APP_MODE` | Aplicação | ✅ Preenchida | server |
| `AUTH_RATE_LIMIT_PER_USER_BURST` | HTTP | ✅ Preenchida | 60 |
| `AUTH_RATE_LIMIT_PER_USER_PER_MIN` | HTTP | ✅ Preenchida | 120 |
| `BACKUP_REMOTE` | Backup | ⚠️ Placeholder | CHANGE_ME_backup:mecontrola-backups |
| `BILLING_ANONYMIZATION_BATCH_SIZE` | Billing | ✅ Preenchida | 500 |
| `BILLING_ANONYMIZATION_RETENTION_DAYS` | Billing | ✅ Preenchida | 365 |
| `BILLING_ANONYMIZATION_SCHEDULE` | Billing | ✅ Preenchida | @daily |
| `BILLING_ENTITLEMENT_CACHE_CAPACITY` | Billing | ✅ Preenchida | 50000 |
| `BILLING_ENTITLEMENT_CACHE_TTL` | Billing | ✅ Preenchida | 5m |
| `BILLING_GRACE_EXPIRATION_SCHEDULE` | Billing | ✅ Preenchida | @daily |
| `BILLING_KIWIFY_EVENTS_HOUSEKEEPING_BATCH` | Billing | ✅ Preenchida | 500 |
| `BILLING_KIWIFY_EVENTS_HOUSEKEEPING_SCHEDULE` | Billing | ✅ Preenchida | @daily |
| `BILLING_KIWIFY_EVENTS_RETENTION_DAYS` | Billing | ✅ Preenchida | 90 |
| `BUDGETS_ABANDONED_DRAFT_CRON` | Budgets | ✅ Preenchida | 0 3**** * |
| `BUDGETS_PENDING_REAPER_INTERVAL` | Budgets | ✅ Preenchida | @ev***30s |
| `BUDGETS_PENDING_TTL_HOURS` | Budgets | ✅ Preenchida | 24 |
| `BUDGETS_RETENTION_PURGE_BATCH_SIZE` | Budgets | ✅ Preenchida | 500 |
| `BUDGETS_RETENTION_PURGE_CRON` | Budgets | ✅ Preenchida | 0 4**** * |
| `CADDY_EMAIL` | Infraestrutura | ✅ Preenchida | jai***com |
| `CADDY_IMAGE` | Infraestrutura | ✅ Preenchida | cad***ine |
| `CADDY_RATE_LIMIT_AUTH` | Infraestrutura | ✅ Preenchida | 20 |
| `CADDY_RATE_LIMIT_REQUESTS` | Infraestrutura | ✅ Preenchida | 100 |
| `CORS_ALLOWED_ORIGINS` | HTTP | ✅ Preenchida | htt***.br |
| `DB_CONN_MAX_IDLE_TIME` | Banco de dados | ✅ Preenchida | 5m |
| `DB_CONN_MAX_LIFETIME` | Banco de dados | ✅ Preenchida | 30m |
| `DB_HOST` | Banco de dados | ✅ Preenchida | loc***ost |
| `DB_MAX_CONNS` | Banco de dados | ✅ Preenchida | 10 |
| `DB_MAX_IDLE_CONNS` | Banco de dados | ✅ Preenchida | 5 |
| `DB_MIN_CONNS` | Banco de dados | ✅ Preenchida | 2 |
| `DB_NAME` | Banco de dados | ✅ Preenchida | mec***_db |
| `DB_PASSWORD` | Banco de dados | ✅ Preenchida | pKH***Yow |
| `DB_PORT` | Banco de dados | ✅ Preenchida | 5432 |
| `DB_SSL_MODE` | Banco de dados | ✅ Preenchida | dis***ble |
| `DB_USER` | Banco de dados | ✅ Preenchida | mec***ola |
| `EMAIL_ACTIVATE_URL` | E-mail | ✅ Preenchida | htt***var |
| `EMAIL_FROM_ADDRESS` | E-mail | ✅ Preenchida | nor***.br |
| `EMAIL_FROM_NAME` | E-mail | ✅ Preenchida | MeC***ola |
| `ENVIRONMENT` | Aplicação | ✅ Preenchida | local |
| `GRAFANA_ADMIN_PASSWORD` | Observabilidade | ✅ Preenchida | 85p***QDA |
| `GRAFANA_ADMIN_USER` | Observabilidade | ✅ Preenchida | admin |
| `IDENTITY_AUTH_EVENTS_HOUSEKEEPING_BATCH` | Identity | ✅ Preenchida | 500 |
| `IDENTITY_AUTH_EVENTS_HOUSEKEEPING_SCHEDULE` | Identity | ✅ Preenchida | @daily |
| `IDENTITY_AUTH_EVENTS_RETENTION_DAYS` | Identity | ✅ Preenchida | 90 |
| `IDENTITY_GATEWAY_AUTH_WINDOW` | Identity | ✅ Preenchida | 60s |
| `IDENTITY_GATEWAY_SHARED_SECRET_CURRENT` | Identity | ✅ Preenchida | 17a***c04 |
| `IDENTITY_GATEWAY_SHARED_SECRET_NEXT` | Identity | ✅ Preenchida | 17a***c04 |
| `IMAGE_NAME` | Deploy | ✅ Preenchida | ghc***ola |
| `IMAGE_TAG` | Deploy | ✅ Preenchida | latest |
| `KIWIFY_ACCOUNT_ID` | Kiwify | ✅ Preenchida | Jho***gTV |
| `KIWIFY_API_BASE_URL` | Kiwify | ✅ Preenchida | htt***com |
| `KIWIFY_CLIENT_ID` | Kiwify | ✅ Preenchida | ee7***292 |
| `KIWIFY_CLIENT_SECRET` | Kiwify | ⚠️ Placeholder | CHANGE_ME_generate_secure_secret_key_min_64_chars |
| `KIWIFY_HTTP_RETRY_BACKOFF` | Kiwify | ✅ Preenchida | 1s |
| `KIWIFY_HTTP_RETRY_MAX_ATTEMPTS` | Kiwify | ✅ Preenchida | 3 |
| `KIWIFY_HTTP_TIMEOUT` | Kiwify | ✅ Preenchida | 10s |
| `KIWIFY_OAUTH_TOKEN_SAFETY_MARGIN` | Kiwify | ✅ Preenchida | 5m |
| `KIWIFY_PRODUCT_ID_ANNUAL` | Kiwify | ✅ Preenchida | aba***c4a |
| `KIWIFY_PRODUCT_ID_MONTHLY` | Kiwify | ✅ Preenchida | 2d7***959 |
| `KIWIFY_PRODUCT_ID_QUARTERLY` | Kiwify | ✅ Preenchida | c2c***eb5 |
| `KIWIFY_RATE_LIMIT_BURST` | Kiwify | ✅ Preenchida | 10 |
| `KIWIFY_RATE_LIMIT_MAX_REQUESTS_PER_MIN` | Kiwify | ✅ Preenchida | 100 |
| `KIWIFY_RECONCILIATION_BATCH_SIZE` | Kiwify | ✅ Preenchida | 200 |
| `KIWIFY_RECONCILIATION_INTERVAL` | Kiwify | ✅ Preenchida | @ho***rly |
| `KIWIFY_WEBHOOK_SECRET` | Kiwify | ✅ Preenchida | 47c***gag |
| `KIWIFY_WEBHOOK_SECRET_NEXT` | Kiwify | ⚠️ Vazia | (vazio) |
| `KIWIFY_WEBHOOK_TOKEN_HEADER` | Kiwify | ✅ Preenchida | X-K***ken |
| `LOG_FORMAT` | Observabilidade | ✅ Preenchida | json |
| `LOG_LEVEL` | Observabilidade | ✅ Preenchida | debug |
| `META_ACCESS_TOKEN` | WhatsApp/Meta | ✅ Preenchida | EAA***DZD |
| `META_APP_SECRET` | WhatsApp/Meta | ✅ Preenchida | fd8***a13 |
| `META_APP_SECRET_NEXT` | WhatsApp/Meta | ⚠️ Vazia | (vazio) |
| `META_BOT_NUMBER_DISPLAY` | WhatsApp/Meta | ✅ Preenchida | +55***870 |
| `META_BOT_NUMBER_E164` | WhatsApp/Meta | ✅ Preenchida | +55***870 |
| `META_OUTREACH_TEMPLATE_NAME` | WhatsApp/Meta | ✅ Preenchida | act***der |
| `META_PHONE_NUMBER_ID` | WhatsApp/Meta | ✅ Preenchida | 122***702 |
| `META_VERIFY_TOKEN` | WhatsApp/Meta | ✅ Preenchida | 17e***d06 |
| `ONBOARDING_CHECKOUT_CORS_ORIGINS` | Onboarding | ✅ Preenchida | htt***.br |
| `ONBOARDING_CHECKOUT_RATE_LIMIT_BURST` | Onboarding | ✅ Preenchida | 5 |
| `ONBOARDING_CHECKOUT_RATE_LIMIT_PER_MIN` | Onboarding | ✅ Preenchida | 10 |
| `ONBOARDING_KIWIFY_ALLOWED_HOSTS` | Onboarding | ✅ Preenchida | pay***.br |
| `ONBOARDING_KIWIFY_CHECKOUT_URLS` | Onboarding | ✅ Preenchida | MON***eKA |
| `ONBOARDING_MAX_TOKEN_LOOKUP_ATTEMPTS` | Onboarding | ✅ Preenchida | 5 |
| `ONBOARDING_META_CLEANUP_SCHEDULE` | Onboarding | ✅ Preenchida | 30 **** * |
| `ONBOARDING_META_RETENTION_DAYS` | Onboarding | ✅ Preenchida | 30 |
| `ONBOARDING_OUTREACH_ENABLED` | Onboarding | ✅ Preenchida | false |
| `ONBOARDING_OUTREACH_GAP_HOURS` | Onboarding | ✅ Preenchida | 2 |
| `ONBOARDING_STATE_RATE_LIMIT_BURST` | Onboarding | ✅ Preenchida | 10 |
| `ONBOARDING_STATE_RATE_LIMIT_PER_MIN` | Onboarding | ✅ Preenchida | 30 |
| `ONBOARDING_TOKEN_ENCRYPTION_KEY` | Onboarding | ✅ Preenchida | LBJ***NI= |
| `ONBOARDING_TOKEN_EXPIRATION_SCHEDULE` | Onboarding | ✅ Preenchida | 0 3**** * |
| `ONBOARDING_TOKEN_TTL_DAYS` | Onboarding | ✅ Preenchida | 7 |
| `ONBOARDING_TRUSTED_PROXIES` | Onboarding | ✅ Preenchida | 127***128 |
| `OPENROUTER_API_KEY` | Agent/LLM | ✅ Preenchida | sk-***b6c |
| `OPENROUTER_BASE_URL` | Agent/LLM | ✅ Preenchida | htt***.ai |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Observabilidade | ✅ Preenchida | loc***317 |
| `OTEL_EXPORTER_OTLP_INSECURE` | Observabilidade | ✅ Preenchida | true |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | Observabilidade | ✅ Preenchida | grpc |
| `OTEL_LGTM_ADMIN_PASSWORD` | Observabilidade | ✅ Preenchida | f41***41a |
| `OTEL_LGTM_ADMIN_USER` | Observabilidade | ✅ Preenchida | admin |
| `OTEL_SERVICE_VERSION` | Observabilidade | ✅ Preenchida | dev |
| `OTEL_TRACE_SAMPLE_RATE` | Observabilidade | ✅ Preenchida | 1.0 |
| `OUTBOX_DISPATCHER_BATCH_SIZE` | Outbox | ✅ Preenchida | 50 |
| `OUTBOX_DISPATCHER_ENABLED` | Outbox | ✅ Preenchida | true |
| `OUTBOX_DISPATCHER_HANDLER_TIMEOUT` | Outbox | ✅ Preenchida | 10s |
| `OUTBOX_DISPATCHER_TICK_INTERVAL` | Outbox | ✅ Preenchida | 500ms |
| `OUTBOX_HOUSEKEEPING_RETENTION_DAYS` | Outbox | ✅ Preenchida | 90 |
| `OUTBOX_HOUSEKEEPING_SCHEDULE` | Outbox | ✅ Preenchida | @daily |
| `OUTBOX_REAPER_INTERVAL` | Outbox | ✅ Preenchida | @ev*** 1m |
| `OUTBOX_REAPER_STUCK_AFTER` | Outbox | ✅ Preenchida | 5m |
| `OUTBOX_RETRY_BASE_BACKOFF` | Outbox | ✅ Preenchida | 2s |
| `OUTBOX_RETRY_MAX_ATTEMPTS` | Outbox | ✅ Preenchida | 3 |
| `OUTBOX_RETRY_MAX_BACKOFF` | Outbox | ✅ Preenchida | 5m |
| `PGBACKREST_S3_BUCKET` | Backup | ⚠️ Placeholder | CHANGE_ME_meu-bucket-backups |
| `PGBACKREST_S3_ENDPOINT` | Backup | ⚠️ Placeholder | CHANGE_ME_s3.us-east-1.amazonaws.com |
| `PGBACKREST_S3_KEY` | Backup | ⚠️ Placeholder | CHANGE_ME_s3_access_key |
| `PGBACKREST_S3_KEY_SECRET` | Backup | ⚠️ Placeholder | CHANGE_ME_s3_secret_key |
| `PGBACKREST_S3_REGION` | Backup | ✅ Preenchida | us-***t-1 |
| `PORT` | HTTP | ✅ Preenchida | 8080 |
| `POSTGRES_IMAGE` | Infraestrutura | ✅ Preenchida | pos***ine |
| `RETENTION_DAYS` | Backup | ✅ Preenchida | 30 |
| `SERVICE_NAME_API` | HTTP | ✅ Preenchida | mec***api |
| `SERVICE_NAME_WORKER` | HTTP | ✅ Preenchida | mec***ker |
| `SMTP_HOST` | E-mail | ✅ Preenchida | smt***com |
| `SMTP_PASSWORD` | E-mail | ✅ Preenchida | re_***rwP |
| `SMTP_PORT` | E-mail | ✅ Preenchida | 587 |
| `SMTP_STARTTLS` | E-mail | ✅ Preenchida | true |
| `SMTP_USERNAME` | E-mail | ✅ Preenchida | resend |
| `TELEGRAM_API_BASE_URL` | Telegram | ✅ Preenchida | htt***org |
| `TELEGRAM_BOT_ID` | Telegram | ✅ Preenchida | 0 |
| `TELEGRAM_BOT_TOKEN` | Telegram | ⚠️ Placeholder | CHANGE_ME_telegram_bot_token |
| `TELEGRAM_BOT_USERNAME` | Telegram | ⚠️ Placeholder | CHANGE_ME_bot_username |
| `TELEGRAM_ENABLED` | Telegram | ✅ Preenchida | false |
| `TELEGRAM_MSG_ALREADY_ACTIVE` | Telegram | ✅ Preenchida | Seu***la. |
| `TELEGRAM_MSG_CODE_ALREADY_USED_OTHER_ACCOUNT` | Telegram | ✅ Preenchida | Est***te. |
| `TELEGRAM_MSG_CODE_EXPIRED_CONTACT_SUPPORT` | Telegram | ✅ Preenchida | Est***te. |
| `TELEGRAM_MSG_CODE_INVALID_CHECK_AGAIN` | Telegram | ✅ Preenchida | Cod***to. |
| `TELEGRAM_MSG_ONBOARDING_FALLBACK` | Telegram | ✅ Preenchida | Par***es. |
| `TELEGRAM_MSG_PAYMENT_STILL_PROCESSING_RETRY` | Telegram | ✅ Preenchida | Seu***os. |
| `TELEGRAM_MSG_PLEASE_USE_ATIVAR_COMMAND` | Telegram | ✅ Preenchida | Par***il. |
| `TELEGRAM_MSG_REQUIRES_WHATSAPP_ACTIVATION` | Telegram | ✅ Preenchida | Ant***do. |
| `TELEGRAM_MSG_SYSTEM_UNAVAILABLE_RETRY` | Telegram | ✅ Preenchida | Sis***os. |
| `TELEGRAM_MSG_WELCOME_ACTIVATED` | Telegram | ✅ Preenchida | Sua***ui. |
| `TELEGRAM_OUTBOUND_TIMEOUT` | Telegram | ✅ Preenchida | 10s |
| `TELEGRAM_SECRET_TOKEN` | Telegram | ⚠️ Placeholder | CHANGE_ME_telegram_secret_token |
| `TELEGRAM_SECRET_TOKEN_NEXT` | Telegram | ⚠️ Vazia | (vazio) |
| `TELEGRAM_WEBHOOK_PATH` | Telegram | ✅ Preenchida | /ap***ook |
| `TELEGRAM_WEBHOOK_RATE_LIMIT_BURST` | Telegram | ✅ Preenchida | 100 |
| `TELEGRAM_WEBHOOK_RATE_LIMIT_PER_MIN` | Telegram | ✅ Preenchida | 600 |
| `TRANSACTIONS_BRAZIL_TIMEZONE` | Transactions | ✅ Preenchida | Ame***ulo |
| `TRANSACTIONS_ENABLED` | Transactions | ✅ Preenchida | false |
| `TRANSACTIONS_IDEMPOTENCY_TTL` | Transactions | ✅ Preenchida | 24h |
| `TRANSACTIONS_MONTHLY_SUMMARY_DEBOUNCE_WINDOW` | Transactions | ✅ Preenchida | 1500ms |
| `TRANSACTIONS_MONTHLY_SUMMARY_RECONCILER_CRON` | Transactions | ✅ Preenchida | @daily |
| `TRANSACTIONS_MONTHLY_SUMMARY_RECONCILER_LOOKBACK_HOURS` | Transactions | ✅ Preenchida | 48 |
| `TRANSACTIONS_RECURRING_MATERIALIZER_CRON` | Transactions | ✅ Preenchida | @daily |
| `WA_MSG_ALREADY_ACTIVE` | WhatsApp/Meta | ✅ Preenchida | Sua***va. |
| `WA_MSG_CODE_ALREADY_USED_OTHER_ACCOUNT` | WhatsApp/Meta | ✅ Preenchida | Est***ta. |
| `WA_MSG_CODE_EXPIRED_CONTACT_SUPPORT` | WhatsApp/Meta | ✅ Preenchida | Est***te. |
| `WA_MSG_CODE_INVALID_CHECK_AGAIN` | WhatsApp/Meta | ✅ Preenchida | Cod***te. |
| `WA_MSG_INVALID_COUNTRY` | WhatsApp/Meta | ✅ Preenchida | Num***os. |
| `WA_MSG_PAYMENT_STILL_PROCESSING_RETRY` | WhatsApp/Meta | ✅ Preenchida | Seu***os. |
| `WA_MSG_PLEASE_USE_ATIVAR_COMMAND` | WhatsApp/Meta | ✅ Preenchida | Par***ao. |
| `WA_MSG_SYSTEM_UNAVAILABLE_RETRY` | WhatsApp/Meta | ✅ Preenchida | Sis***os. |
| `WA_MSG_WELCOME_ACTIVATED` | WhatsApp/Meta | ✅ Preenchida | Sua***la. |
| `WHATSAPP_WEBHOOK_RATE_LIMIT_BURST` | WhatsApp/Meta | ✅ Preenchida | 100 |
| `WHATSAPP_WEBHOOK_RATE_LIMIT_PER_MIN` | WhatsApp/Meta | ✅ Preenchida | 600 |

## 4. Observações e recomendações

- O arquivo `.env.example` local possui `IDENTITY_AUTH_EVENTS_RETENTION_DAYS` declarado duas vezes (linhas 278 e 326). No `.env` remoto ele aparece uma vez.
- `DATABASE_URL` está presente no `.env` remoto, mas é usada apenas para testes de integração locais.
- Existem 15 variáveis reconhecidas preenchidas com placeholders do tipo `CHANGE_ME_*`.
- Existem 3 variáveis reconhecidas que estão vazias no `.env` remoto.
- `ENVIRONMENT` está definido como `local` no `.env` remoto, embora o `compose.prod.yml` sobrescreva para `production`.
- `IMAGE_TAG` está como `latest`. Recomenda-se fixar uma tag imutável.
- `LOG_LEVEL` está como `debug` em produção; o recomendado é `info`.
- `KIWIFY_CLIENT_SECRET` ainda está com placeholder, apesar das outras variáveis Kiwify estarem preenchidas.
- **Ação prioritária:** revisar as 21 variáveis faltantes e corrigir placeholders críticos antes do próximo deploy.
- **Limpeza:** avaliar se as 10 variáveis em excesso podem ser removidas.

---
*Relatório gerado automaticamente. Nenhuma alteração foi feita no servidor.*
