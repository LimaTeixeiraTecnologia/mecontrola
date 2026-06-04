package postgres

const (
	findActiveByUserIDForUpdate = `
		SELECT
			id, user_id, provider, external_subscription_id, plan_code, status,
			period_start, period_end, grace_period_end, refund_amount_cents,
			last_event_at, last_webhook_event_id, created_at, updated_at, deleted_at
		FROM subscriptions
		WHERE user_id = $1
		  AND status IN ('TRIALING','ACTIVE','PAST_DUE','CANCELED_PENDING')
		  AND deleted_at IS NULL
		FOR UPDATE
	`

	findActiveByUserID = `
		SELECT
			id, user_id, provider, external_subscription_id, plan_code, status,
			period_start, period_end, grace_period_end, refund_amount_cents,
			last_event_at, last_webhook_event_id, created_at, updated_at, deleted_at
		FROM subscriptions
		WHERE user_id = $1
		  AND status IN ('TRIALING','ACTIVE','PAST_DUE','CANCELED_PENDING')
		  AND deleted_at IS NULL
	`

	findByExternalID = `
		SELECT
			id, user_id, provider, external_subscription_id, plan_code, status,
			period_start, period_end, grace_period_end, refund_amount_cents,
			last_event_at, last_webhook_event_id, created_at, updated_at, deleted_at
		FROM subscriptions
		WHERE provider = $1
		  AND external_subscription_id = $2
		  AND deleted_at IS NULL
	`

	upsertSubscription = `
		INSERT INTO subscriptions (
			id, user_id, provider, external_subscription_id, plan_code, status,
			period_start, period_end, grace_period_end, refund_amount_cents,
			last_event_at, last_webhook_event_id, created_at, updated_at, deleted_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13, $14, $15
		)
		ON CONFLICT (id) DO UPDATE SET
			status               = EXCLUDED.status,
			period_start         = EXCLUDED.period_start,
			period_end           = EXCLUDED.period_end,
			grace_period_end     = EXCLUDED.grace_period_end,
			refund_amount_cents  = EXCLUDED.refund_amount_cents,
			last_event_at        = EXCLUDED.last_event_at,
			last_webhook_event_id = EXCLUDED.last_webhook_event_id,
			updated_at           = EXCLUDED.updated_at,
			deleted_at           = EXCLUDED.deleted_at
	`

	insertIfNewWebhookEvent = `
		INSERT INTO webhook_events (
			id, provider, external_event_id, event_type, signature,
			headers, payload, received_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8
		)
		ON CONFLICT (provider, external_event_id) DO NOTHING
	`

	findRawPayload = `
		SELECT payload
		FROM webhook_events
		WHERE id = $1
	`

	markProcessed = `
		UPDATE webhook_events
		SET processed_at = $2
		WHERE id = $1
	`

	recordApplication = `
		INSERT INTO billing_event_applications (event_id, subscription_id, applied_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (event_id) DO NOTHING
	`

	listPendingAnonymization = `
		SELECT
			id, provider, external_event_id, event_type, signature,
			headers, payload, received_at
		FROM webhook_events
		WHERE received_at < $1
		  AND anonymized_at IS NULL
		LIMIT $2
	`

	anonymize = `
		UPDATE webhook_events
		SET payload = $2, anonymized_at = $3
		WHERE id = $1
	`

	loadKiwifyProductPlans = `
		SELECT kiwify_product_id, plan_code
		FROM billing_plans
		WHERE kiwify_product_id IS NOT NULL
	`
)
