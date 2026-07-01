Welcome to Ubuntu 24.04.4 LTS (GNU/Linux 6.8.0-124-generic x86_64)

 * Documentation:  https://help.ubuntu.com
 * Management:     https://landscape.canonical.com
 * Support:        https://ubuntu.com/pro

 System information as of Wed Jul  1 14:31:43 UTC 2026

  System load:  0.42               Processes:             173
  Usage of /:   28.7% of 95.82GB   Users logged in:       0
  Memory usage: 30%                IPv4 address for eth0: 187.77.45.48
  Swap usage:   0%                 IPv6 address for eth0: 2a02:4780:6e:79bc::1

 * Strictly confined Kubernetes makes edge and IoT secure. Learn how MicroK8s
   just raised the bar for easy, resilient and secure K8s cluster deployment.

   https://ubuntu.com/engage/secure-kubernetes-at-the-edge

Expanded Security Maintenance for Applications is not enabled.

31 updates can be applied immediately.
16 of these updates are standard security updates.
To see these additional updates run: apt list --upgradable

Enable ESM Apps to receive additional future security updates.
See https://ubuntu.com/esm or run: sudo pro status


1 updates could not be installed automatically. For more details,
see /var/log/unattended-upgrades/unattended-upgrades.log

--
-- PostgreSQL database dump
--

\restrict frdPr7GOvcfNcFYDgiNTsR2gQxRFsyKEd9k2n0RYxHBG9zet6WYyYcTDw2oagpw

-- Dumped from database version 16.14 (Debian 16.14-1.pgdg13+1)
-- Dumped by pg_dump version 16.14 (Debian 16.14-1.pgdg13+1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: mecontrola; Type: SCHEMA; Schema: -; Owner: mecontrola
--

CREATE SCHEMA mecontrola;


ALTER SCHEMA mecontrola OWNER TO mecontrola;

--
-- Name: categories_parent_kind_change_blocks_children(); Type: FUNCTION; Schema: mecontrola; Owner: mecontrola
--

CREATE FUNCTION mecontrola.categories_parent_kind_change_blocks_children() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    child_count INT;
BEGIN
    IF NEW.kind = OLD.kind THEN
        RETURN NEW;
    END IF;
    SELECT count(*) INTO child_count FROM mecontrola.categories WHERE parent_id = NEW.id;
    IF child_count > 0 THEN
        RAISE EXCEPTION 'categories_parent_kind_change_blocks_children: cannot change kind of parent % with % active children', NEW.id, child_count;
    END IF;
    RETURN NEW;
END;
$$;


ALTER FUNCTION mecontrola.categories_parent_kind_change_blocks_children() OWNER TO mecontrola;

--
-- Name: categories_parent_same_kind(); Type: FUNCTION; Schema: mecontrola; Owner: mecontrola
--

CREATE FUNCTION mecontrola.categories_parent_same_kind() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    parent_kind TEXT;
BEGIN
    IF NEW.parent_id IS NULL THEN
        RETURN NEW;
    END IF;
    SELECT kind INTO parent_kind FROM mecontrola.categories WHERE id = NEW.parent_id;
    IF parent_kind IS NULL THEN
        RAISE EXCEPTION 'categories_parent_same_kind: parent_id % not found', NEW.parent_id;
    END IF;
    IF parent_kind <> NEW.kind THEN
        RAISE EXCEPTION 'categories_parent_same_kind: child kind % does not match parent kind %', NEW.kind, parent_kind;
    END IF;
    RETURN NEW;
END;
$$;


ALTER FUNCTION mecontrola.categories_parent_same_kind() OWNER TO mecontrola;

--
-- Name: immutable_unaccent(text); Type: FUNCTION; Schema: mecontrola; Owner: mecontrola
--

CREATE FUNCTION mecontrola.immutable_unaccent(text) RETURNS text
    LANGUAGE sql IMMUTABLE PARALLEL SAFE
    AS $_$
    SELECT mecontrola.unaccent($1);
$_$;


ALTER FUNCTION mecontrola.immutable_unaccent(text) OWNER TO mecontrola;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: agents_write_ledger; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.agents_write_ledger (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    wamid text NOT NULL,
    item_seq integer NOT NULL,
    operation text NOT NULL,
    resource_id uuid NOT NULL,
    resource_kind text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT agents_write_ledger_item_seq_positive_check CHECK ((item_seq >= 0)),
    CONSTRAINT agents_write_ledger_operation_nonempty_check CHECK ((length(operation) > 0)),
    CONSTRAINT agents_write_ledger_wamid_nonempty_check CHECK ((length(wamid) > 0))
);


ALTER TABLE mecontrola.agents_write_ledger OWNER TO mecontrola;

--
-- Name: auth_events; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.auth_events (
    id uuid NOT NULL,
    occurred_at timestamp with time zone DEFAULT now() NOT NULL,
    user_id uuid,
    kind text NOT NULL,
    source text NOT NULL,
    reason text,
    request_id text,
    client_ip inet,
    CONSTRAINT auth_events_kind_check CHECK ((kind = ANY (ARRAY['principal_established'::text, 'failed'::text, 'unknown_user'::text]))),
    CONSTRAINT auth_events_reason_check CHECK ((((kind = 'failed'::text) AND (reason = ANY (ARRAY['invalid_signature'::text, 'unknown_wa_id'::text, 'invalid_country'::text, 'invalid_payload'::text, 'rate_limited'::text, 'db_unavailable'::text, 'gateway_missing_header'::text, 'gateway_invalid_timestamp'::text, 'gateway_stale_timestamp'::text, 'gateway_invalid_signature'::text, 'stale_webhook'::text, 'invalid_webhook_timestamp'::text]))) OR ((kind <> 'failed'::text) AND (reason IS NULL)))),
    CONSTRAINT auth_events_source_check CHECK ((source = ANY (ARRAY['whatsapp'::text, 'gateway'::text])))
);


ALTER TABLE mecontrola.auth_events OWNER TO mecontrola;

--
-- Name: billing_kiwify_events; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.billing_kiwify_events (
    envelope_id text NOT NULL,
    trigger text NOT NULL,
    raw_body jsonb NOT NULL,
    received_at timestamp with time zone DEFAULT now() NOT NULL,
    processed_at timestamp with time zone,
    signature_status text NOT NULL,
    CONSTRAINT billing_kiwify_events_signature_status_check CHECK ((signature_status = ANY (ARRAY['valid'::text, 'invalid'::text, 'rotated'::text])))
)
WITH (fillfactor='85');


ALTER TABLE mecontrola.billing_kiwify_events OWNER TO mecontrola;

--
-- Name: billing_plans; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.billing_plans (
    kiwify_product_id text NOT NULL,
    code text NOT NULL,
    duration_days integer NOT NULL,
    currency text DEFAULT 'BRL'::text NOT NULL,
    CONSTRAINT billing_plans_code_check CHECK ((code = ANY (ARRAY['MONTHLY'::text, 'QUARTERLY'::text, 'ANNUAL'::text]))),
    CONSTRAINT billing_plans_currency_check CHECK ((currency <> ''::text)),
    CONSTRAINT billing_plans_duration_days_check CHECK ((duration_days > 0))
);


ALTER TABLE mecontrola.billing_plans OWNER TO mecontrola;

--
-- Name: billing_processed_events; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.billing_processed_events (
    event_key text NOT NULL,
    trigger text NOT NULL,
    recurso_id text NOT NULL,
    occurred_at timestamp with time zone NOT NULL,
    applied_at timestamp with time zone DEFAULT now() NOT NULL,
    status text NOT NULL,
    CONSTRAINT billing_processed_events_status_check CHECK ((status = ANY (ARRAY['applied'::text, 'superseded'::text])))
);


ALTER TABLE mecontrola.billing_processed_events OWNER TO mecontrola;

--
-- Name: billing_reconciliation_checkpoints; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.billing_reconciliation_checkpoints (
    name text NOT NULL,
    watermark timestamp with time zone NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE mecontrola.billing_reconciliation_checkpoints OWNER TO mecontrola;

--
-- Name: billing_subscriptions; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.billing_subscriptions (
    id uuid NOT NULL,
    funnel_token text NOT NULL,
    user_id uuid,
    kiwify_order_id text NOT NULL,
    kiwify_subscription_id text,
    plan_code text NOT NULL,
    status text NOT NULL,
    period_start timestamp with time zone NOT NULL,
    period_end timestamp with time zone NOT NULL,
    grace_end timestamp with time zone,
    last_event_at timestamp with time zone NOT NULL,
    customer_mobile_e164 text,
    customer_email text,
    external_sale_id text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT billing_subscriptions_status_check CHECK ((status = ANY (ARRAY['TRIALING'::text, 'ACTIVE'::text, 'PAST_DUE'::text, 'CANCELED_PENDING'::text, 'EXPIRED'::text, 'REFUNDED'::text])))
)
WITH (fillfactor='80');


ALTER TABLE mecontrola.billing_subscriptions OWNER TO mecontrola;

--
-- Name: budget_alerts_sent; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.budget_alerts_sent (
    user_id uuid NOT NULL,
    budget_id uuid NOT NULL,
    kind text NOT NULL,
    ref_day date NOT NULL,
    sent_at timestamp with time zone DEFAULT now() NOT NULL,
    notified_at timestamp with time zone,
    notify_channel text,
    CONSTRAINT budget_alerts_sent_kind_chk CHECK ((kind = ANY (ARRAY['category_threshold'::text, 'goal_achieved'::text, 'card_limit_near'::text])))
);


ALTER TABLE mecontrola.budget_alerts_sent OWNER TO mecontrola;

--
-- Name: budgets; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.budgets (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    competence text NOT NULL,
    total_cents bigint DEFAULT 0 NOT NULL,
    state smallint NOT NULL,
    activated_at timestamp with time zone,
    auto_draft boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    CONSTRAINT budgets_competence_chk CHECK ((competence ~ '^\d{4}-(0[1-9]|1[0-2])$'::text)),
    CONSTRAINT budgets_state_chk CHECK ((state = ANY (ARRAY[1, 2])))
);


ALTER TABLE mecontrola.budgets OWNER TO mecontrola;

--
-- Name: budgets_abandoned_draft_signals; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.budgets_abandoned_draft_signals (
    budget_id uuid NOT NULL,
    signaled_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE mecontrola.budgets_abandoned_draft_signals OWNER TO mecontrola;

--
-- Name: budgets_alerts; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.budgets_alerts (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    competence text NOT NULL,
    root_slug text NOT NULL,
    threshold smallint NOT NULL,
    state smallint NOT NULL,
    triggered_by_committed_at timestamp with time zone NOT NULL,
    spent_cents bigint NOT NULL,
    planned_cents bigint NOT NULL,
    created_at timestamp with time zone NOT NULL,
    CONSTRAINT budgets_alerts_competence_chk CHECK ((competence ~ '^\d{4}-(0[1-9]|1[0-2])$'::text)),
    CONSTRAINT budgets_alerts_state_chk CHECK (((state >= 1) AND (state <= 5))),
    CONSTRAINT budgets_alerts_threshold_chk CHECK ((threshold = ANY (ARRAY[80, 100])))
);


ALTER TABLE mecontrola.budgets_alerts OWNER TO mecontrola;

--
-- Name: budgets_allocations; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.budgets_allocations (
    budget_id uuid NOT NULL,
    root_slug text NOT NULL,
    basis_points integer NOT NULL,
    planned_cents bigint NOT NULL,
    CONSTRAINT budgets_allocations_basis_points_chk CHECK (((basis_points >= 0) AND (basis_points <= 10000))),
    CONSTRAINT budgets_allocations_root_chk CHECK ((root_slug = ANY (ARRAY['expense.custo_fixo'::text, 'expense.conhecimento'::text, 'expense.prazeres'::text, 'expense.metas'::text, 'expense.liberdade_financeira'::text])))
);


ALTER TABLE mecontrola.budgets_allocations OWNER TO mecontrola;

--
-- Name: budgets_expense_events_pending; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.budgets_expense_events_pending (
    id uuid NOT NULL,
    event_id uuid NOT NULL,
    source text NOT NULL,
    user_id uuid NOT NULL,
    external_transaction_id text NOT NULL,
    expected_version bigint NOT NULL,
    mutation_kind smallint NOT NULL,
    payload jsonb NOT NULL,
    state smallint NOT NULL,
    received_at timestamp with time zone NOT NULL,
    transitioned_at timestamp with time zone,
    reason text,
    CONSTRAINT budgets_expense_events_pending_mutation_chk CHECK (((mutation_kind >= 1) AND (mutation_kind <= 3))),
    CONSTRAINT budgets_expense_events_pending_state_chk CHECK (((state >= 1) AND (state <= 4)))
);


ALTER TABLE mecontrola.budgets_expense_events_pending OWNER TO mecontrola;

--
-- Name: budgets_expenses; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.budgets_expenses (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    source text NOT NULL,
    external_transaction_id text NOT NULL,
    subcategory_id uuid NOT NULL,
    root_slug text NOT NULL,
    competence text NOT NULL,
    amount_cents bigint NOT NULL,
    occurred_at timestamp with time zone NOT NULL,
    version bigint NOT NULL,
    tombstone_version bigint,
    deleted_at timestamp with time zone,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    CONSTRAINT budgets_expenses_amount_chk CHECK ((amount_cents > 0)),
    CONSTRAINT budgets_expenses_competence_chk CHECK ((competence ~ '^\d{4}-(0[1-9]|1[0-2])$'::text)),
    CONSTRAINT budgets_expenses_root_chk CHECK ((root_slug = ANY (ARRAY['expense.custo_fixo'::text, 'expense.conhecimento'::text, 'expense.prazeres'::text, 'expense.metas'::text, 'expense.liberdade_financeira'::text])))
);


ALTER TABLE mecontrola.budgets_expenses OWNER TO mecontrola;

--
-- Name: budgets_threshold_states; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.budgets_threshold_states (
    user_id uuid NOT NULL,
    competence text NOT NULL,
    root_slug text NOT NULL,
    threshold smallint NOT NULL,
    currently_crossed boolean DEFAULT false NOT NULL,
    version bigint DEFAULT 0 NOT NULL,
    last_crossed_at timestamp with time zone,
    last_uncrossed_at timestamp with time zone,
    last_evaluated_committed_at timestamp with time zone,
    CONSTRAINT budgets_threshold_competence_chk CHECK ((competence ~ '^\d{4}-(0[1-9]|1[0-2])$'::text)),
    CONSTRAINT budgets_threshold_states_threshold_chk CHECK ((threshold = ANY (ARRAY[80, 100])))
);


ALTER TABLE mecontrola.budgets_threshold_states OWNER TO mecontrola;

--
-- Name: card_invoice_alerts_sent; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.card_invoice_alerts_sent (
    user_id uuid NOT NULL,
    card_id uuid NOT NULL,
    ref_due_date date NOT NULL,
    sent_at timestamp with time zone DEFAULT now() NOT NULL,
    notified_at timestamp with time zone,
    notify_channel text
);


ALTER TABLE mecontrola.card_invoice_alerts_sent OWNER TO mecontrola;

--
-- Name: cards; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.cards (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    name text NOT NULL,
    nickname text NOT NULL,
    closing_day smallint NOT NULL,
    due_day smallint NOT NULL,
    limit_cents bigint DEFAULT 0 NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT cards_closing_day_chk CHECK (((closing_day >= 1) AND (closing_day <= 31))),
    CONSTRAINT cards_due_day_chk CHECK (((due_day >= 1) AND (due_day <= 31))),
    CONSTRAINT cards_limit_cents_chk CHECK (((limit_cents >= 0) AND (limit_cents <= 100000000))),
    CONSTRAINT cards_name_len_chk CHECK (((char_length(name) >= 1) AND (char_length(name) <= 64))),
    CONSTRAINT cards_nickname_len_chk CHECK (((char_length(nickname) >= 1) AND (char_length(nickname) <= 32)))
);


ALTER TABLE mecontrola.cards OWNER TO mecontrola;

--
-- Name: categories; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.categories (
    id uuid NOT NULL,
    slug text NOT NULL,
    name text NOT NULL,
    kind text NOT NULL,
    parent_id uuid,
    allocation_type text DEFAULT 'consumption'::text NOT NULL,
    deprecated_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT categories_allocation_type_check CHECK ((allocation_type = ANY (ARRAY['consumption'::text, 'asset_allocation'::text]))),
    CONSTRAINT categories_kind_check CHECK ((kind = ANY (ARRAY['income'::text, 'expense'::text]))),
    CONSTRAINT categories_no_cycles CHECK (((parent_id IS NULL) OR (parent_id <> id)))
);


ALTER TABLE mecontrola.categories OWNER TO mecontrola;

--
-- Name: category_dictionary; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.category_dictionary (
    id uuid NOT NULL,
    category_id uuid NOT NULL,
    kind text NOT NULL,
    term text NOT NULL,
    term_normalized text GENERATED ALWAYS AS (lower(mecontrola.immutable_unaccent(term))) STORED,
    signal_type text NOT NULL,
    confidence text NOT NULL,
    is_ambiguous boolean DEFAULT false NOT NULL,
    deprecated_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT dictionary_confidence_check CHECK ((confidence = ANY (ARRAY['high'::text, 'medium'::text, 'low'::text]))),
    CONSTRAINT dictionary_kind_check CHECK ((kind = ANY (ARRAY['income'::text, 'expense'::text]))),
    CONSTRAINT dictionary_signal_type_check CHECK ((signal_type = ANY (ARRAY['canonical_name'::text, 'alias'::text, 'phrase'::text, 'merchant'::text, 'segment'::text])))
);


ALTER TABLE mecontrola.category_dictionary OWNER TO mecontrola;

--
-- Name: category_editorial_version; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.category_editorial_version (
    version bigint DEFAULT 1 NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE mecontrola.category_editorial_version OWNER TO mecontrola;

--
-- Name: channel_processed_messages; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.channel_processed_messages (
    channel text NOT NULL,
    message_id text NOT NULL,
    processed_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT channel_processed_messages_channel_check CHECK ((channel = 'whatsapp'::text)),
    CONSTRAINT channel_processed_messages_message_id_nonempty_check CHECK ((length(message_id) > 0))
);


ALTER TABLE mecontrola.channel_processed_messages OWNER TO mecontrola;

--
-- Name: consumer_lookup_attempts; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.consumer_lookup_attempts (
    event_id text NOT NULL,
    attempts integer DEFAULT 1 NOT NULL,
    last_attempt_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT consumer_lookup_attempts_attempts_check CHECK ((attempts > 0))
);


ALTER TABLE mecontrola.consumer_lookup_attempts OWNER TO mecontrola;

--
-- Name: idempotency_keys; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.idempotency_keys (
    scope text NOT NULL,
    key text NOT NULL,
    user_id uuid NOT NULL,
    request_hash text NOT NULL,
    response_status integer NOT NULL,
    response_body bytea NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT idempotency_keys_body_size_chk CHECK ((octet_length(response_body) <= 65536)),
    CONSTRAINT idempotency_keys_key_len_chk CHECK (((char_length(key) >= 1) AND (char_length(key) <= 128))),
    CONSTRAINT idempotency_keys_request_hash_len_chk CHECK ((char_length(request_hash) = 64)),
    CONSTRAINT idempotency_keys_status_chk CHECK (((response_status >= 200) AND (response_status <= 599)))
);


ALTER TABLE mecontrola.idempotency_keys OWNER TO mecontrola;

--
-- Name: identity_entitlements; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.identity_entitlements (
    user_id uuid NOT NULL,
    subscription_id uuid NOT NULL,
    status text NOT NULL,
    period_end timestamp with time zone NOT NULL,
    grace_end timestamp with time zone,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT identity_entitlements_status_check CHECK ((status = ANY (ARRAY['TRIALING'::text, 'ACTIVE'::text, 'PAST_DUE'::text, 'CANCELED_PENDING'::text, 'EXPIRED'::text, 'REFUNDED'::text])))
)
WITH (fillfactor='80');


ALTER TABLE mecontrola.identity_entitlements OWNER TO mecontrola;

--
-- Name: identity_entitlements_pending; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.identity_entitlements_pending (
    subscription_id uuid NOT NULL,
    funnel_token text NOT NULL,
    payload jsonb NOT NULL,
    received_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE mecontrola.identity_entitlements_pending OWNER TO mecontrola;

--
-- Name: onboarding_activation_nomatch_throttle; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.onboarding_activation_nomatch_throttle (
    mobile_e164 text NOT NULL,
    window_start timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE mecontrola.onboarding_activation_nomatch_throttle OWNER TO mecontrola;

--
-- Name: onboarding_tokens; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.onboarding_tokens (
    id uuid NOT NULL,
    token_hash bytea NOT NULL,
    status text NOT NULL,
    plan_id text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    paid_at timestamp with time zone,
    consumed_at timestamp with time zone,
    outreach_sent_at timestamp with time zone,
    activation_token_ciphertext text NOT NULL,
    subscription_id uuid,
    customer_mobile_e164 text,
    customer_email text,
    external_sale_id text,
    consumed_by_user_id uuid,
    consumed_by_mobile_e164 text,
    activation_path text,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    email_sent_at timestamp with time zone,
    page_opened_at timestamp with time zone,
    activation_started_at timestamp with time zone,
    whatsapp_opened_at timestamp with time zone,
    CONSTRAINT onboarding_tokens_activation_path_check CHECK ((activation_path = ANY (ARRAY['direct'::text, 'fallback_e164'::text, 'outreach'::text, 'admin'::text]))),
    CONSTRAINT onboarding_tokens_status_check CHECK ((status = ANY (ARRAY['PENDING'::text, 'PAID'::text, 'CONSUMED'::text, 'EXPIRED'::text])))
);


ALTER TABLE mecontrola.onboarding_tokens OWNER TO mecontrola;

--
-- Name: onboarding_welcome_processed; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.onboarding_welcome_processed (
    event_id text NOT NULL,
    processed_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT onboarding_welcome_processed_event_id_nonempty_check CHECK ((length(event_id) > 0))
);


ALTER TABLE mecontrola.onboarding_welcome_processed OWNER TO mecontrola;

--
-- Name: outbox_events; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.outbox_events (
    id uuid NOT NULL,
    event_type text NOT NULL,
    aggregate_type text NOT NULL,
    aggregate_id text NOT NULL,
    aggregate_user_id uuid,
    payload jsonb NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    status smallint NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    max_attempts integer NOT NULL,
    next_attempt_at timestamp with time zone NOT NULL,
    last_error text,
    locked_at timestamp with time zone,
    locked_by text,
    published_at timestamp with time zone,
    occurred_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT outbox_events_attempts_check CHECK ((attempts >= 0)),
    CONSTRAINT outbox_events_attempts_max_check CHECK ((attempts <= max_attempts)),
    CONSTRAINT outbox_events_max_attempts_check CHECK ((max_attempts > 0)),
    CONSTRAINT outbox_events_published_status_check CHECK (((status = 3) = (published_at IS NOT NULL))),
    CONSTRAINT outbox_events_status_check CHECK ((status = ANY (ARRAY[1, 2, 3, 4])))
)
WITH (fillfactor='70', autovacuum_vacuum_scale_factor='0.05', autovacuum_analyze_scale_factor='0.02', autovacuum_vacuum_cost_delay='2');


ALTER TABLE mecontrola.outbox_events OWNER TO mecontrola;

--
-- Name: COLUMN outbox_events.status; Type: COMMENT; Schema: mecontrola; Owner: mecontrola
--

COMMENT ON COLUMN mecontrola.outbox_events.status IS '1=Pending, 2=Processing, 3=Published, 4=Failed';


--
-- Name: platform_embeddings; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.platform_embeddings (
    id uuid NOT NULL,
    resource_id text NOT NULL,
    thread_id text NOT NULL,
    source_message_pk uuid,
    content text NOT NULL,
    embedding mecontrola.vector(1536) NOT NULL,
    model text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT platform_embeddings_resource_len_chk CHECK (((char_length(resource_id) >= 1) AND (char_length(resource_id) <= 256))),
    CONSTRAINT platform_embeddings_thread_len_chk CHECK (((char_length(thread_id) >= 1) AND (char_length(thread_id) <= 256)))
);


ALTER TABLE mecontrola.platform_embeddings OWNER TO mecontrola;

--
-- Name: platform_messages; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.platform_messages (
    id uuid NOT NULL,
    thread_pk uuid NOT NULL,
    resource_id text NOT NULL,
    role text NOT NULL,
    content text DEFAULT ''::text NOT NULL,
    parts jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT platform_messages_resource_len_chk CHECK (((char_length(resource_id) >= 1) AND (char_length(resource_id) <= 256))),
    CONSTRAINT platform_messages_role_chk CHECK ((role = ANY (ARRAY['user'::text, 'assistant'::text, 'tool'::text, 'system'::text])))
);


ALTER TABLE mecontrola.platform_messages OWNER TO mecontrola;

--
-- Name: platform_resources; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.platform_resources (
    resource_id text NOT NULL,
    working_memory text DEFAULT ''::text NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT platform_resources_resource_len_chk CHECK (((char_length(resource_id) >= 1) AND (char_length(resource_id) <= 256)))
);


ALTER TABLE mecontrola.platform_resources OWNER TO mecontrola;

--
-- Name: platform_runs; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.platform_runs (
    id uuid NOT NULL,
    thread_pk uuid NOT NULL,
    resource_id text NOT NULL,
    thread_id text NOT NULL,
    agent_id text DEFAULT ''::text NOT NULL,
    workflow text DEFAULT ''::text NOT NULL,
    correlation_key text DEFAULT ''::text NOT NULL,
    status text NOT NULL,
    outcome text DEFAULT ''::text NOT NULL,
    error text DEFAULT ''::text NOT NULL,
    started_at timestamp with time zone DEFAULT now() NOT NULL,
    ended_at timestamp with time zone,
    duration_ms bigint DEFAULT 0 NOT NULL,
    CONSTRAINT platform_runs_duration_chk CHECK ((duration_ms >= 0)),
    CONSTRAINT platform_runs_resource_len_chk CHECK (((char_length(resource_id) >= 1) AND (char_length(resource_id) <= 256))),
    CONSTRAINT platform_runs_status_chk CHECK ((status = ANY (ARRAY['running'::text, 'succeeded'::text, 'failed'::text]))),
    CONSTRAINT platform_runs_thread_len_chk CHECK (((char_length(thread_id) >= 1) AND (char_length(thread_id) <= 256)))
);


ALTER TABLE mecontrola.platform_runs OWNER TO mecontrola;

--
-- Name: platform_scorer_results; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.platform_scorer_results (
    id uuid NOT NULL,
    run_id uuid NOT NULL,
    scorer_id text NOT NULL,
    kind text NOT NULL,
    score double precision DEFAULT 0 NOT NULL,
    reason text DEFAULT ''::text NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    sampled boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT platform_scorer_results_kind_chk CHECK ((kind = ANY (ARRAY['code_based'::text, 'llm_judged'::text]))),
    CONSTRAINT platform_scorer_results_score_chk CHECK (((score >= (0)::double precision) AND (score <= (1)::double precision)))
);


ALTER TABLE mecontrola.platform_scorer_results OWNER TO mecontrola;

--
-- Name: platform_threads; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.platform_threads (
    id uuid NOT NULL,
    resource_id text NOT NULL,
    thread_id text NOT NULL,
    title text DEFAULT ''::text NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT platform_threads_resource_len_chk CHECK (((char_length(resource_id) >= 1) AND (char_length(resource_id) <= 256))),
    CONSTRAINT platform_threads_thread_len_chk CHECK (((char_length(thread_id) >= 1) AND (char_length(thread_id) <= 256)))
);


ALTER TABLE mecontrola.platform_threads OWNER TO mecontrola;

--
-- Name: schema_migrations; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.schema_migrations (
    version bigint NOT NULL,
    dirty boolean NOT NULL
);


ALTER TABLE mecontrola.schema_migrations OWNER TO mecontrola;

--
-- Name: support_signals; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.support_signals (
    id uuid NOT NULL,
    kind text NOT NULL,
    payload jsonb NOT NULL,
    occurred_at timestamp with time zone DEFAULT now() NOT NULL,
    resolved_at timestamp with time zone,
    resolved_by text,
    notes text,
    CONSTRAINT support_signals_kind_check CHECK ((kind = ANY (ARRAY['orphan_expired_subscription'::text, 'paid_without_token'::text, 'token_reuse_attempt'::text])))
);


ALTER TABLE mecontrola.support_signals OWNER TO mecontrola;

--
-- Name: transactions; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.transactions (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    direction smallint NOT NULL,
    payment_method smallint NOT NULL,
    amount_cents bigint NOT NULL,
    description text NOT NULL,
    category_id uuid NOT NULL,
    subcategory_id uuid,
    category_name_snapshot text NOT NULL,
    subcategory_name_snapshot text,
    ref_month text NOT NULL,
    occurred_at timestamp with time zone NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    deleted_at timestamp with time zone,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    origin_wamid text,
    origin_item_seq integer,
    origin_operation text,
    CONSTRAINT transactions_amount_cents_chk CHECK ((amount_cents > 0)),
    CONSTRAINT transactions_ref_month_chk CHECK ((ref_month ~ '^\d{4}-(0[1-9]|1[0-2])$'::text))
);


ALTER TABLE mecontrola.transactions OWNER TO mecontrola;

--
-- Name: transactions_card_invoice_items; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.transactions_card_invoice_items (
    id uuid NOT NULL,
    invoice_id uuid NOT NULL,
    purchase_id uuid NOT NULL,
    user_id uuid NOT NULL,
    ref_month text NOT NULL,
    installment_index smallint NOT NULL,
    amount_cents bigint NOT NULL,
    deleted_at timestamp with time zone,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    CONSTRAINT transactions_cii_amount_cents_chk CHECK ((amount_cents > 0)),
    CONSTRAINT transactions_cii_ref_month_chk CHECK ((ref_month ~ '^\d{4}-(0[1-9]|1[0-2])$'::text))
);


ALTER TABLE mecontrola.transactions_card_invoice_items OWNER TO mecontrola;

--
-- Name: transactions_card_invoices; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.transactions_card_invoices (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    card_id uuid NOT NULL,
    ref_month text NOT NULL,
    closing_at timestamp with time zone NOT NULL,
    due_at timestamp with time zone NOT NULL,
    items_total_cents bigint DEFAULT 0 NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    CONSTRAINT transactions_ci_ref_month_chk CHECK ((ref_month ~ '^\d{4}-(0[1-9]|1[0-2])$'::text))
);


ALTER TABLE mecontrola.transactions_card_invoices OWNER TO mecontrola;

--
-- Name: transactions_card_purchases; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.transactions_card_purchases (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    card_id uuid NOT NULL,
    direction smallint NOT NULL,
    total_amount_cents bigint NOT NULL,
    installments_total smallint NOT NULL,
    description text NOT NULL,
    category_id uuid NOT NULL,
    subcategory_id uuid,
    category_name_snapshot text NOT NULL,
    subcategory_name_snapshot text,
    purchased_at timestamp with time zone NOT NULL,
    card_closing_day smallint NOT NULL,
    card_due_day smallint NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    deleted_at timestamp with time zone,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    origin_wamid text,
    origin_item_seq integer,
    origin_operation text,
    CONSTRAINT transactions_cp_amount_cents_chk CHECK ((total_amount_cents > 0)),
    CONSTRAINT transactions_cp_closing_day_chk CHECK (((card_closing_day >= 1) AND (card_closing_day <= 31))),
    CONSTRAINT transactions_cp_direction_chk CHECK ((direction = 2)),
    CONSTRAINT transactions_cp_due_day_chk CHECK (((card_due_day >= 1) AND (card_due_day <= 31))),
    CONSTRAINT transactions_cp_installments_chk CHECK (((installments_total >= 1) AND (installments_total <= 24)))
);


ALTER TABLE mecontrola.transactions_card_purchases OWNER TO mecontrola;

--
-- Name: transactions_monthly_summary; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.transactions_monthly_summary (
    user_id uuid NOT NULL,
    ref_month text NOT NULL,
    income_cents bigint DEFAULT 0 NOT NULL,
    outcome_cents bigint DEFAULT 0 NOT NULL,
    total_cents bigint DEFAULT 0 NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    CONSTRAINT transactions_ms_ref_month_chk CHECK ((ref_month ~ '^\d{4}-(0[1-9]|1[0-2])$'::text))
);


ALTER TABLE mecontrola.transactions_monthly_summary OWNER TO mecontrola;

--
-- Name: transactions_recurring_materializations; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.transactions_recurring_materializations (
    template_id uuid NOT NULL,
    ref_month text NOT NULL,
    materialized_transaction_id uuid,
    materialized_purchase_id uuid,
    materialized_at timestamp with time zone NOT NULL,
    CONSTRAINT transactions_rm_ref_month_chk CHECK ((ref_month ~ '^\d{4}-(0[1-9]|1[0-2])$'::text))
);


ALTER TABLE mecontrola.transactions_recurring_materializations OWNER TO mecontrola;

--
-- Name: transactions_recurring_templates; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.transactions_recurring_templates (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    direction smallint NOT NULL,
    payment_method smallint NOT NULL,
    card_id uuid,
    amount_cents bigint NOT NULL,
    description text NOT NULL,
    category_id uuid NOT NULL,
    subcategory_id uuid,
    category_name_snapshot text NOT NULL,
    subcategory_name_snapshot text,
    frequency smallint NOT NULL,
    day_of_month smallint NOT NULL,
    installments_total smallint DEFAULT 1 NOT NULL,
    started_at timestamp with time zone NOT NULL,
    ended_at timestamp with time zone,
    version bigint DEFAULT 1 NOT NULL,
    deleted_at timestamp with time zone,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    CONSTRAINT transactions_recurring_templates_credit_chk CHECK (((payment_method <> 7) OR (card_id IS NOT NULL))),
    CONSTRAINT transactions_rt_amount_cents_chk CHECK ((amount_cents > 0)),
    CONSTRAINT transactions_rt_day_of_month_chk CHECK (((day_of_month >= 1) AND (day_of_month <= 28))),
    CONSTRAINT transactions_rt_installments_chk CHECK (((installments_total >= 1) AND (installments_total <= 24)))
);


ALTER TABLE mecontrola.transactions_recurring_templates OWNER TO mecontrola;

--
-- Name: user_identities; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.user_identities (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    channel text NOT NULL,
    external_id text NOT NULL,
    verified_at timestamp with time zone DEFAULT now() NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    unlinked_at timestamp with time zone,
    CONSTRAINT user_identities_channel_check CHECK ((channel = 'whatsapp'::text)),
    CONSTRAINT user_identities_external_id_nonempty_check CHECK ((length(external_id) > 0)),
    CONSTRAINT user_identities_status_unlinked_at_check CHECK (((unlinked_at IS NULL) OR (unlinked_at >= created_at)))
);


ALTER TABLE mecontrola.user_identities OWNER TO mecontrola;

--
-- Name: user_whatsapp_history; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.user_whatsapp_history (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    number text NOT NULL,
    active boolean NOT NULL,
    linked_at timestamp with time zone DEFAULT now() NOT NULL,
    unlinked_at timestamp with time zone,
    reason text,
    CONSTRAINT user_whatsapp_history_active_unlinked_at_check CHECK (((active = true) = (unlinked_at IS NULL)))
);


ALTER TABLE mecontrola.user_whatsapp_history OWNER TO mecontrola;

--
-- Name: users; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.users (
    id uuid NOT NULL,
    whatsapp_number text NOT NULL,
    email text,
    display_name text,
    status text DEFAULT 'ACTIVE'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT users_status_check CHECK ((status = ANY (ARRAY['ACTIVE'::text, 'DELETED'::text]))),
    CONSTRAINT users_status_deleted_at_check CHECK (((status = 'DELETED'::text) = (deleted_at IS NOT NULL)))
);


ALTER TABLE mecontrola.users OWNER TO mecontrola;

--
-- Name: whatsapp_message_status; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.whatsapp_message_status (
    id uuid NOT NULL,
    message_id text NOT NULL,
    status text NOT NULL,
    recipient_id text DEFAULT ''::text NOT NULL,
    error_code text,
    error_title text,
    status_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT whatsapp_message_status_message_id_len_chk CHECK (((char_length(message_id) >= 1) AND (char_length(message_id) <= 256))),
    CONSTRAINT whatsapp_message_status_status_chk CHECK ((status = ANY (ARRAY['sent'::text, 'delivered'::text, 'read'::text, 'failed'::text])))
);


ALTER TABLE mecontrola.whatsapp_message_status OWNER TO mecontrola;

--
-- Name: workflow_runs; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.workflow_runs (
    id uuid NOT NULL,
    workflow text NOT NULL,
    correlation_key text NOT NULL,
    status text NOT NULL,
    suspend_reason text DEFAULT ''::text NOT NULL,
    cursor integer DEFAULT 0 NOT NULL,
    state jsonb DEFAULT '{}'::jsonb NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    max_attempts integer NOT NULL,
    version bigint DEFAULT 1 NOT NULL,
    last_error text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    ended_at timestamp with time zone,
    CONSTRAINT workflow_runs_attempts_check CHECK ((attempts >= 0)),
    CONSTRAINT workflow_runs_max_attempts_check CHECK ((max_attempts > 0)),
    CONSTRAINT workflow_runs_status_check CHECK ((status = ANY (ARRAY['running'::text, 'suspended'::text, 'succeeded'::text, 'failed'::text])))
)
WITH (fillfactor='70');


ALTER TABLE mecontrola.workflow_runs OWNER TO mecontrola;

--
-- Name: workflow_steps; Type: TABLE; Schema: mecontrola; Owner: mecontrola
--

CREATE TABLE mecontrola.workflow_steps (
    id uuid NOT NULL,
    run_id uuid NOT NULL,
    step_id text NOT NULL,
    seq integer NOT NULL,
    status text NOT NULL,
    attempt integer DEFAULT 1 NOT NULL,
    duration_ms bigint DEFAULT 0 NOT NULL,
    error text DEFAULT ''::text NOT NULL,
    started_at timestamp with time zone DEFAULT now() NOT NULL,
    ended_at timestamp with time zone,
    CONSTRAINT workflow_steps_status_check CHECK ((status = ANY (ARRAY['completed'::text, 'suspended'::text, 'failed'::text, 'skipped'::text])))
);


ALTER TABLE mecontrola.workflow_steps OWNER TO mecontrola;

--
-- Name: agents_write_ledger agents_write_ledger_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.agents_write_ledger
    ADD CONSTRAINT agents_write_ledger_pkey PRIMARY KEY (id);


--
-- Name: agents_write_ledger agents_write_ledger_uniq; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.agents_write_ledger
    ADD CONSTRAINT agents_write_ledger_uniq UNIQUE (wamid, item_seq, operation);


--
-- Name: auth_events auth_events_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.auth_events
    ADD CONSTRAINT auth_events_pkey PRIMARY KEY (id);


--
-- Name: billing_kiwify_events billing_kiwify_events_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.billing_kiwify_events
    ADD CONSTRAINT billing_kiwify_events_pkey PRIMARY KEY (envelope_id);


--
-- Name: billing_plans billing_plans_code_uniq; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.billing_plans
    ADD CONSTRAINT billing_plans_code_uniq UNIQUE (code);


--
-- Name: billing_plans billing_plans_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.billing_plans
    ADD CONSTRAINT billing_plans_pkey PRIMARY KEY (kiwify_product_id);


--
-- Name: billing_processed_events billing_processed_events_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.billing_processed_events
    ADD CONSTRAINT billing_processed_events_pkey PRIMARY KEY (event_key);


--
-- Name: billing_reconciliation_checkpoints billing_reconciliation_checkpoints_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.billing_reconciliation_checkpoints
    ADD CONSTRAINT billing_reconciliation_checkpoints_pkey PRIMARY KEY (name);


--
-- Name: billing_subscriptions billing_subscriptions_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.billing_subscriptions
    ADD CONSTRAINT billing_subscriptions_pkey PRIMARY KEY (id);


--
-- Name: budget_alerts_sent budget_alerts_sent_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.budget_alerts_sent
    ADD CONSTRAINT budget_alerts_sent_pkey PRIMARY KEY (user_id, budget_id, kind, ref_day);


--
-- Name: budgets_abandoned_draft_signals budgets_abandoned_draft_signals_pk; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.budgets_abandoned_draft_signals
    ADD CONSTRAINT budgets_abandoned_draft_signals_pk PRIMARY KEY (budget_id);


--
-- Name: budgets_alerts budgets_alerts_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.budgets_alerts
    ADD CONSTRAINT budgets_alerts_pkey PRIMARY KEY (id);


--
-- Name: budgets_allocations budgets_allocations_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.budgets_allocations
    ADD CONSTRAINT budgets_allocations_pkey PRIMARY KEY (budget_id, root_slug);


--
-- Name: budgets_expense_events_pending budgets_expense_events_pending_event_uk; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.budgets_expense_events_pending
    ADD CONSTRAINT budgets_expense_events_pending_event_uk UNIQUE (event_id);


--
-- Name: budgets_expense_events_pending budgets_expense_events_pending_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.budgets_expense_events_pending
    ADD CONSTRAINT budgets_expense_events_pending_pkey PRIMARY KEY (id);


--
-- Name: budgets_expenses budgets_expenses_identity_uk; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.budgets_expenses
    ADD CONSTRAINT budgets_expenses_identity_uk UNIQUE (user_id, source, external_transaction_id);


--
-- Name: budgets_expenses budgets_expenses_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.budgets_expenses
    ADD CONSTRAINT budgets_expenses_pkey PRIMARY KEY (id);


--
-- Name: budgets budgets_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.budgets
    ADD CONSTRAINT budgets_pkey PRIMARY KEY (id);


--
-- Name: budgets_threshold_states budgets_threshold_states_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.budgets_threshold_states
    ADD CONSTRAINT budgets_threshold_states_pkey PRIMARY KEY (user_id, competence, root_slug, threshold);


--
-- Name: budgets budgets_user_comp_uk; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.budgets
    ADD CONSTRAINT budgets_user_comp_uk UNIQUE (user_id, competence);


--
-- Name: card_invoice_alerts_sent card_invoice_alerts_sent_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.card_invoice_alerts_sent
    ADD CONSTRAINT card_invoice_alerts_sent_pkey PRIMARY KEY (user_id, card_id, ref_due_date);


--
-- Name: cards cards_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.cards
    ADD CONSTRAINT cards_pkey PRIMARY KEY (id);


--
-- Name: categories categories_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.categories
    ADD CONSTRAINT categories_pkey PRIMARY KEY (id);


--
-- Name: category_dictionary category_dictionary_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.category_dictionary
    ADD CONSTRAINT category_dictionary_pkey PRIMARY KEY (id);


--
-- Name: category_editorial_version category_editorial_version_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.category_editorial_version
    ADD CONSTRAINT category_editorial_version_pkey PRIMARY KEY (version);


--
-- Name: channel_processed_messages channel_processed_messages_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.channel_processed_messages
    ADD CONSTRAINT channel_processed_messages_pkey PRIMARY KEY (channel, message_id);


--
-- Name: consumer_lookup_attempts consumer_lookup_attempts_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.consumer_lookup_attempts
    ADD CONSTRAINT consumer_lookup_attempts_pkey PRIMARY KEY (event_id);


--
-- Name: idempotency_keys idempotency_keys_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.idempotency_keys
    ADD CONSTRAINT idempotency_keys_pkey PRIMARY KEY (scope, key, user_id);


--
-- Name: identity_entitlements_pending identity_entitlements_pending_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.identity_entitlements_pending
    ADD CONSTRAINT identity_entitlements_pending_pkey PRIMARY KEY (subscription_id);


--
-- Name: identity_entitlements identity_entitlements_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.identity_entitlements
    ADD CONSTRAINT identity_entitlements_pkey PRIMARY KEY (user_id);


--
-- Name: onboarding_activation_nomatch_throttle onboarding_activation_nomatch_throttle_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.onboarding_activation_nomatch_throttle
    ADD CONSTRAINT onboarding_activation_nomatch_throttle_pkey PRIMARY KEY (mobile_e164, window_start);


--
-- Name: onboarding_tokens onboarding_tokens_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.onboarding_tokens
    ADD CONSTRAINT onboarding_tokens_pkey PRIMARY KEY (id);


--
-- Name: onboarding_tokens onboarding_tokens_token_hash_uniq; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.onboarding_tokens
    ADD CONSTRAINT onboarding_tokens_token_hash_uniq UNIQUE (token_hash);


--
-- Name: onboarding_welcome_processed onboarding_welcome_processed_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.onboarding_welcome_processed
    ADD CONSTRAINT onboarding_welcome_processed_pkey PRIMARY KEY (event_id);


--
-- Name: outbox_events outbox_events_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.outbox_events
    ADD CONSTRAINT outbox_events_pkey PRIMARY KEY (id);


--
-- Name: platform_embeddings platform_embeddings_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.platform_embeddings
    ADD CONSTRAINT platform_embeddings_pkey PRIMARY KEY (id);


--
-- Name: platform_messages platform_messages_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.platform_messages
    ADD CONSTRAINT platform_messages_pkey PRIMARY KEY (id);


--
-- Name: platform_resources platform_resources_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.platform_resources
    ADD CONSTRAINT platform_resources_pkey PRIMARY KEY (resource_id);


--
-- Name: platform_runs platform_runs_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.platform_runs
    ADD CONSTRAINT platform_runs_pkey PRIMARY KEY (id);


--
-- Name: platform_scorer_results platform_scorer_results_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.platform_scorer_results
    ADD CONSTRAINT platform_scorer_results_pkey PRIMARY KEY (id);


--
-- Name: platform_threads platform_threads_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.platform_threads
    ADD CONSTRAINT platform_threads_pkey PRIMARY KEY (id);


--
-- Name: platform_threads platform_threads_resource_thread_uniq; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.platform_threads
    ADD CONSTRAINT platform_threads_resource_thread_uniq UNIQUE (resource_id, thread_id);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


--
-- Name: support_signals support_signals_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.support_signals
    ADD CONSTRAINT support_signals_pkey PRIMARY KEY (id);


--
-- Name: transactions_card_invoice_items transactions_card_invoice_items_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.transactions_card_invoice_items
    ADD CONSTRAINT transactions_card_invoice_items_pkey PRIMARY KEY (id);


--
-- Name: transactions_card_invoice_items transactions_card_invoice_items_purchase_uk; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.transactions_card_invoice_items
    ADD CONSTRAINT transactions_card_invoice_items_purchase_uk UNIQUE (purchase_id, installment_index);


--
-- Name: transactions_card_invoices transactions_card_invoices_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.transactions_card_invoices
    ADD CONSTRAINT transactions_card_invoices_pkey PRIMARY KEY (id);


--
-- Name: transactions_card_invoices transactions_card_invoices_uk; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.transactions_card_invoices
    ADD CONSTRAINT transactions_card_invoices_uk UNIQUE (user_id, card_id, ref_month);


--
-- Name: transactions_card_purchases transactions_card_purchases_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.transactions_card_purchases
    ADD CONSTRAINT transactions_card_purchases_pkey PRIMARY KEY (id);


--
-- Name: transactions_monthly_summary transactions_monthly_summary_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.transactions_monthly_summary
    ADD CONSTRAINT transactions_monthly_summary_pkey PRIMARY KEY (user_id, ref_month);


--
-- Name: transactions transactions_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.transactions
    ADD CONSTRAINT transactions_pkey PRIMARY KEY (id);


--
-- Name: transactions_recurring_materializations transactions_recurring_materializations_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.transactions_recurring_materializations
    ADD CONSTRAINT transactions_recurring_materializations_pkey PRIMARY KEY (template_id, ref_month);


--
-- Name: transactions_recurring_templates transactions_recurring_templates_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.transactions_recurring_templates
    ADD CONSTRAINT transactions_recurring_templates_pkey PRIMARY KEY (id);


--
-- Name: user_identities user_identities_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.user_identities
    ADD CONSTRAINT user_identities_pkey PRIMARY KEY (id);


--
-- Name: user_whatsapp_history user_whatsapp_history_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.user_whatsapp_history
    ADD CONSTRAINT user_whatsapp_history_pkey PRIMARY KEY (id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: whatsapp_message_status whatsapp_message_status_message_status_uniq; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.whatsapp_message_status
    ADD CONSTRAINT whatsapp_message_status_message_status_uniq UNIQUE (message_id, status);


--
-- Name: whatsapp_message_status whatsapp_message_status_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.whatsapp_message_status
    ADD CONSTRAINT whatsapp_message_status_pkey PRIMARY KEY (id);


--
-- Name: workflow_runs workflow_runs_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.workflow_runs
    ADD CONSTRAINT workflow_runs_pkey PRIMARY KEY (id);


--
-- Name: workflow_steps workflow_steps_pkey; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.workflow_steps
    ADD CONSTRAINT workflow_steps_pkey PRIMARY KEY (id);


--
-- Name: workflow_steps workflow_steps_run_seq_attempt_uidx; Type: CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.workflow_steps
    ADD CONSTRAINT workflow_steps_run_seq_attempt_uidx UNIQUE (run_id, seq, attempt);


--
-- Name: agents_write_ledger_user_created_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX agents_write_ledger_user_created_idx ON mecontrola.agents_write_ledger USING btree (user_id, created_at);


--
-- Name: auth_events_failed_occurred_at_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX auth_events_failed_occurred_at_idx ON mecontrola.auth_events USING btree (occurred_at DESC, reason) WHERE (kind = 'failed'::text);


--
-- Name: auth_events_request_id_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX auth_events_request_id_idx ON mecontrola.auth_events USING btree (request_id) WHERE (request_id IS NOT NULL);


--
-- Name: auth_events_user_id_occurred_at_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX auth_events_user_id_occurred_at_idx ON mecontrola.auth_events USING btree (user_id, occurred_at DESC) WHERE (user_id IS NOT NULL);


--
-- Name: billing_kiwify_events_received_at_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX billing_kiwify_events_received_at_idx ON mecontrola.billing_kiwify_events USING btree (received_at);


--
-- Name: billing_kiwify_events_trigger_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX billing_kiwify_events_trigger_idx ON mecontrola.billing_kiwify_events USING btree (trigger);


--
-- Name: billing_processed_events_recurso_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX billing_processed_events_recurso_idx ON mecontrola.billing_processed_events USING btree (recurso_id);


--
-- Name: billing_subscriptions_external_sale_id_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX billing_subscriptions_external_sale_id_idx ON mecontrola.billing_subscriptions USING btree (external_sale_id) WHERE (external_sale_id IS NOT NULL);


--
-- Name: billing_subscriptions_funnel_token_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX billing_subscriptions_funnel_token_idx ON mecontrola.billing_subscriptions USING btree (funnel_token);


--
-- Name: billing_subscriptions_kiwify_order_uniq_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE UNIQUE INDEX billing_subscriptions_kiwify_order_uniq_idx ON mecontrola.billing_subscriptions USING btree (kiwify_order_id);


--
-- Name: billing_subscriptions_user_active_uniq_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE UNIQUE INDEX billing_subscriptions_user_active_uniq_idx ON mecontrola.billing_subscriptions USING btree (user_id) WHERE ((user_id IS NOT NULL) AND (status = ANY (ARRAY['ACTIVE'::text, 'PAST_DUE'::text, 'CANCELED_PENDING'::text])));


--
-- Name: budget_alerts_sent_pending_notify_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX budget_alerts_sent_pending_notify_idx ON mecontrola.budget_alerts_sent USING btree (sent_at) WHERE (notified_at IS NULL);


--
-- Name: budget_alerts_sent_user_ref_day_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX budget_alerts_sent_user_ref_day_idx ON mecontrola.budget_alerts_sent USING btree (user_id, ref_day DESC);


--
-- Name: budgets_alerts_listing_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX budgets_alerts_listing_idx ON mecontrola.budgets_alerts USING btree (user_id, created_at DESC) WHERE (state = ANY (ARRAY[1, 2]));


--
-- Name: budgets_alerts_user_comp_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX budgets_alerts_user_comp_idx ON mecontrola.budgets_alerts USING btree (user_id, competence, root_slug, threshold);


--
-- Name: budgets_competence_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX budgets_competence_idx ON mecontrola.budgets USING btree (competence);


--
-- Name: budgets_expenses_deleted_at_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX budgets_expenses_deleted_at_idx ON mecontrola.budgets_expenses USING btree (deleted_at) WHERE (deleted_at IS NOT NULL);


--
-- Name: budgets_expenses_summary_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX budgets_expenses_summary_idx ON mecontrola.budgets_expenses USING btree (user_id, competence, subcategory_id) WHERE (deleted_at IS NULL);


--
-- Name: budgets_expenses_summary_root_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX budgets_expenses_summary_root_idx ON mecontrola.budgets_expenses USING btree (user_id, competence, root_slug) WHERE (deleted_at IS NULL);


--
-- Name: budgets_pending_identity_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX budgets_pending_identity_idx ON mecontrola.budgets_expense_events_pending USING btree (user_id, source, external_transaction_id) WHERE (state = 1);


--
-- Name: budgets_pending_state_received_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX budgets_pending_state_received_idx ON mecontrola.budgets_expense_events_pending USING btree (state, received_at) WHERE (state = 1);


--
-- Name: card_invoice_alerts_sent_pending_notify_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX card_invoice_alerts_sent_pending_notify_idx ON mecontrola.card_invoice_alerts_sent USING btree (sent_at) WHERE (notified_at IS NULL);


--
-- Name: card_invoice_alerts_sent_user_due_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX card_invoice_alerts_sent_user_due_idx ON mecontrola.card_invoice_alerts_sent USING btree (user_id, ref_due_date DESC);


--
-- Name: cards_due_day_scan_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX cards_due_day_scan_idx ON mecontrola.cards USING btree (due_day) WHERE (deleted_at IS NULL);


--
-- Name: cards_user_limit_positive_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX cards_user_limit_positive_idx ON mecontrola.cards USING btree (user_id) WHERE ((limit_cents > 0) AND (deleted_at IS NULL));


--
-- Name: cards_user_nickname_active_uniq_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE UNIQUE INDEX cards_user_nickname_active_uniq_idx ON mecontrola.cards USING btree (user_id, nickname) WHERE (deleted_at IS NULL);


--
-- Name: cards_user_pagination_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX cards_user_pagination_idx ON mecontrola.cards USING btree (user_id, created_at DESC, id DESC) WHERE (deleted_at IS NULL);


--
-- Name: categories_kind_parent_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX categories_kind_parent_idx ON mecontrola.categories USING btree (kind, parent_id) WHERE (deprecated_at IS NULL);


--
-- Name: categories_kind_slug_uniq_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE UNIQUE INDEX categories_kind_slug_uniq_idx ON mecontrola.categories USING btree (kind, slug);


--
-- Name: categories_parent_sort_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX categories_parent_sort_idx ON mecontrola.categories USING btree (parent_id, name COLLATE "pt-BR-x-icu") WHERE (deprecated_at IS NULL);


--
-- Name: channel_processed_messages_processed_at_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX channel_processed_messages_processed_at_idx ON mecontrola.channel_processed_messages USING btree (processed_at);


--
-- Name: consumer_lookup_attempts_last_attempt_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX consumer_lookup_attempts_last_attempt_idx ON mecontrola.consumer_lookup_attempts USING btree (last_attempt_at);


--
-- Name: dictionary_active_term_uniq_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE UNIQUE INDEX dictionary_active_term_uniq_idx ON mecontrola.category_dictionary USING btree (kind, category_id, term_normalized) WHERE (deprecated_at IS NULL);


--
-- Name: dictionary_kind_term_normalized_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX dictionary_kind_term_normalized_idx ON mecontrola.category_dictionary USING btree (kind, term_normalized COLLATE "pt-BR-x-icu") WHERE (deprecated_at IS NULL);


--
-- Name: dictionary_term_normalized_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX dictionary_term_normalized_idx ON mecontrola.category_dictionary USING btree (term_normalized COLLATE "pt-BR-x-icu") WHERE (deprecated_at IS NULL);


--
-- Name: dictionary_term_trgm_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX dictionary_term_trgm_idx ON mecontrola.category_dictionary USING gin (term_normalized mecontrola.gin_trgm_ops) WHERE (deprecated_at IS NULL);


--
-- Name: idempotency_keys_expires_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX idempotency_keys_expires_idx ON mecontrola.idempotency_keys USING btree (expires_at);


--
-- Name: identity_entitlements_pending_funnel_token_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX identity_entitlements_pending_funnel_token_idx ON mecontrola.identity_entitlements_pending USING btree (funnel_token);


--
-- Name: identity_entitlements_subscription_id_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX identity_entitlements_subscription_id_idx ON mecontrola.identity_entitlements USING btree (subscription_id);


--
-- Name: onboarding_tokens_by_mobile_paid_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX onboarding_tokens_by_mobile_paid_idx ON mecontrola.onboarding_tokens USING btree (customer_mobile_e164) WHERE ((status = 'PAID'::text) AND (outreach_sent_at IS NOT NULL));


--
-- Name: onboarding_tokens_mobile_activable_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX onboarding_tokens_mobile_activable_idx ON mecontrola.onboarding_tokens USING btree (customer_mobile_e164, paid_at) WHERE (status = 'PAID'::text);


--
-- Name: onboarding_tokens_outreach_pick_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX onboarding_tokens_outreach_pick_idx ON mecontrola.onboarding_tokens USING btree (status, paid_at) WHERE ((status = 'PAID'::text) AND (outreach_sent_at IS NULL));


--
-- Name: onboarding_tokens_status_expires_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX onboarding_tokens_status_expires_idx ON mecontrola.onboarding_tokens USING btree (status, expires_at) WHERE (status = ANY (ARRAY['PENDING'::text, 'PAID'::text]));


--
-- Name: outbox_events_aggregate_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX outbox_events_aggregate_idx ON mecontrola.outbox_events USING btree (aggregate_type, aggregate_id);


--
-- Name: outbox_events_aggregate_user_id_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX outbox_events_aggregate_user_id_idx ON mecontrola.outbox_events USING btree (aggregate_user_id) WHERE (aggregate_user_id IS NOT NULL);


--
-- Name: outbox_events_dispatcher_pending_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX outbox_events_dispatcher_pending_idx ON mecontrola.outbox_events USING btree (next_attempt_at) WHERE (status = 1);


--
-- Name: outbox_events_housekeeping_published_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX outbox_events_housekeeping_published_idx ON mecontrola.outbox_events USING btree (published_at) WHERE (status = 3);


--
-- Name: outbox_events_reaper_processing_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX outbox_events_reaper_processing_idx ON mecontrola.outbox_events USING btree (locked_at) WHERE (status = 2);


--
-- Name: platform_embeddings_hnsw_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX platform_embeddings_hnsw_idx ON mecontrola.platform_embeddings USING hnsw (embedding mecontrola.vector_cosine_ops);


--
-- Name: platform_embeddings_resource_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX platform_embeddings_resource_idx ON mecontrola.platform_embeddings USING btree (resource_id);


--
-- Name: platform_embeddings_source_model_uniq; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE UNIQUE INDEX platform_embeddings_source_model_uniq ON mecontrola.platform_embeddings USING btree (source_message_pk, model) WHERE (source_message_pk IS NOT NULL);


--
-- Name: platform_messages_thread_created_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX platform_messages_thread_created_idx ON mecontrola.platform_messages USING btree (thread_pk, created_at);


--
-- Name: platform_runs_resource_started_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX platform_runs_resource_started_idx ON mecontrola.platform_runs USING btree (resource_id, started_at DESC);


--
-- Name: platform_runs_thread_started_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX platform_runs_thread_started_idx ON mecontrola.platform_runs USING btree (thread_pk, started_at DESC);


--
-- Name: platform_scorer_results_run_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX platform_scorer_results_run_idx ON mecontrola.platform_scorer_results USING btree (run_id);


--
-- Name: platform_threads_resource_id_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX platform_threads_resource_id_idx ON mecontrola.platform_threads USING btree (resource_id);


--
-- Name: support_signals_kind_open_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX support_signals_kind_open_idx ON mecontrola.support_signals USING btree (kind, occurred_at) WHERE (resolved_at IS NULL);


--
-- Name: transactions_card_invoice_items_user_month_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX transactions_card_invoice_items_user_month_idx ON mecontrola.transactions_card_invoice_items USING btree (user_id, ref_month) WHERE (deleted_at IS NULL);


--
-- Name: transactions_card_purchases_origin_uk; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE UNIQUE INDEX transactions_card_purchases_origin_uk ON mecontrola.transactions_card_purchases USING btree (origin_wamid, origin_item_seq, origin_operation) WHERE (origin_wamid IS NOT NULL);


--
-- Name: transactions_card_purchases_user_card_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX transactions_card_purchases_user_card_idx ON mecontrola.transactions_card_purchases USING btree (user_id, card_id, created_at DESC, id DESC) WHERE (deleted_at IS NULL);


--
-- Name: transactions_origin_uk; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE UNIQUE INDEX transactions_origin_uk ON mecontrola.transactions USING btree (origin_wamid, origin_item_seq, origin_operation) WHERE (origin_wamid IS NOT NULL);


--
-- Name: transactions_recurring_templates_user_day_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX transactions_recurring_templates_user_day_idx ON mecontrola.transactions_recurring_templates USING btree (user_id, day_of_month) WHERE (deleted_at IS NULL);


--
-- Name: transactions_user_created_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX transactions_user_created_idx ON mecontrola.transactions USING btree (user_id, created_at DESC, id DESC) WHERE (deleted_at IS NULL);


--
-- Name: transactions_user_month_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX transactions_user_month_idx ON mecontrola.transactions USING btree (user_id, ref_month) WHERE (deleted_at IS NULL);


--
-- Name: user_identities_channel_external_active_uniq_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE UNIQUE INDEX user_identities_channel_external_active_uniq_idx ON mecontrola.user_identities USING btree (channel, external_id) WHERE (unlinked_at IS NULL);


--
-- Name: user_identities_channel_external_unlinked_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX user_identities_channel_external_unlinked_idx ON mecontrola.user_identities USING btree (channel, external_id) WHERE (unlinked_at IS NOT NULL);


--
-- Name: user_identities_user_channel_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX user_identities_user_channel_idx ON mecontrola.user_identities USING btree (user_id, channel) WHERE (unlinked_at IS NULL);


--
-- Name: user_whatsapp_history_number_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX user_whatsapp_history_number_idx ON mecontrola.user_whatsapp_history USING btree (number);


--
-- Name: user_whatsapp_history_user_active_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX user_whatsapp_history_user_active_idx ON mecontrola.user_whatsapp_history USING btree (user_id, active);


--
-- Name: users_email_active_uniq_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE UNIQUE INDEX users_email_active_uniq_idx ON mecontrola.users USING btree (email) WHERE ((email IS NOT NULL) AND (deleted_at IS NULL));


--
-- Name: users_whatsapp_number_active_uniq_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE UNIQUE INDEX users_whatsapp_number_active_uniq_idx ON mecontrola.users USING btree (whatsapp_number) WHERE (deleted_at IS NULL);


--
-- Name: users_whatsapp_number_deleted_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX users_whatsapp_number_deleted_idx ON mecontrola.users USING btree (whatsapp_number) WHERE (deleted_at IS NOT NULL);


--
-- Name: whatsapp_message_status_message_id_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX whatsapp_message_status_message_id_idx ON mecontrola.whatsapp_message_status USING btree (message_id);


--
-- Name: whatsapp_message_status_status_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX whatsapp_message_status_status_idx ON mecontrola.whatsapp_message_status USING btree (status);


--
-- Name: workflow_runs_active_key_uidx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE UNIQUE INDEX workflow_runs_active_key_uidx ON mecontrola.workflow_runs USING btree (workflow, correlation_key) WHERE (status = ANY (ARRAY['running'::text, 'suspended'::text]));


--
-- Name: workflow_runs_status_updated_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX workflow_runs_status_updated_idx ON mecontrola.workflow_runs USING btree (status, updated_at);


--
-- Name: workflow_steps_run_seq_idx; Type: INDEX; Schema: mecontrola; Owner: mecontrola
--

CREATE INDEX workflow_steps_run_seq_idx ON mecontrola.workflow_steps USING btree (run_id, seq);


--
-- Name: categories categories_parent_kind_change_blocks_children_trg; Type: TRIGGER; Schema: mecontrola; Owner: mecontrola
--

CREATE TRIGGER categories_parent_kind_change_blocks_children_trg BEFORE UPDATE OF kind ON mecontrola.categories FOR EACH ROW EXECUTE FUNCTION mecontrola.categories_parent_kind_change_blocks_children();


--
-- Name: categories categories_parent_same_kind_trg; Type: TRIGGER; Schema: mecontrola; Owner: mecontrola
--

CREATE TRIGGER categories_parent_same_kind_trg BEFORE INSERT OR UPDATE OF parent_id, kind ON mecontrola.categories FOR EACH ROW EXECUTE FUNCTION mecontrola.categories_parent_same_kind();


--
-- Name: billing_subscriptions billing_subscriptions_plan_code_fkey; Type: FK CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.billing_subscriptions
    ADD CONSTRAINT billing_subscriptions_plan_code_fkey FOREIGN KEY (plan_code) REFERENCES mecontrola.billing_plans(code);


--
-- Name: billing_subscriptions billing_subscriptions_user_id_fkey; Type: FK CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.billing_subscriptions
    ADD CONSTRAINT billing_subscriptions_user_id_fkey FOREIGN KEY (user_id) REFERENCES mecontrola.users(id) ON DELETE RESTRICT;


--
-- Name: budgets_allocations budgets_allocations_budget_fk; Type: FK CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.budgets_allocations
    ADD CONSTRAINT budgets_allocations_budget_fk FOREIGN KEY (budget_id) REFERENCES mecontrola.budgets(id) ON DELETE CASCADE;


--
-- Name: card_invoice_alerts_sent card_invoice_alerts_sent_card_fk; Type: FK CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.card_invoice_alerts_sent
    ADD CONSTRAINT card_invoice_alerts_sent_card_fk FOREIGN KEY (card_id) REFERENCES mecontrola.cards(id) ON DELETE CASCADE;


--
-- Name: cards cards_user_fk; Type: FK CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.cards
    ADD CONSTRAINT cards_user_fk FOREIGN KEY (user_id) REFERENCES mecontrola.users(id) ON DELETE RESTRICT;


--
-- Name: categories categories_parent_id_fkey; Type: FK CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.categories
    ADD CONSTRAINT categories_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES mecontrola.categories(id);


--
-- Name: category_dictionary category_dictionary_category_id_fkey; Type: FK CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.category_dictionary
    ADD CONSTRAINT category_dictionary_category_id_fkey FOREIGN KEY (category_id) REFERENCES mecontrola.categories(id);


--
-- Name: identity_entitlements identity_entitlements_user_id_fkey; Type: FK CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.identity_entitlements
    ADD CONSTRAINT identity_entitlements_user_id_fkey FOREIGN KEY (user_id) REFERENCES mecontrola.users(id);


--
-- Name: platform_messages platform_messages_thread_fkey; Type: FK CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.platform_messages
    ADD CONSTRAINT platform_messages_thread_fkey FOREIGN KEY (thread_pk) REFERENCES mecontrola.platform_threads(id) ON DELETE CASCADE;


--
-- Name: platform_runs platform_runs_thread_fkey; Type: FK CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.platform_runs
    ADD CONSTRAINT platform_runs_thread_fkey FOREIGN KEY (thread_pk) REFERENCES mecontrola.platform_threads(id) ON DELETE CASCADE;


--
-- Name: platform_scorer_results platform_scorer_results_run_fkey; Type: FK CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.platform_scorer_results
    ADD CONSTRAINT platform_scorer_results_run_fkey FOREIGN KEY (run_id) REFERENCES mecontrola.platform_runs(id) ON DELETE CASCADE;


--
-- Name: transactions_card_invoice_items transactions_card_invoice_items_invoice_id_fkey; Type: FK CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.transactions_card_invoice_items
    ADD CONSTRAINT transactions_card_invoice_items_invoice_id_fkey FOREIGN KEY (invoice_id) REFERENCES mecontrola.transactions_card_invoices(id);


--
-- Name: transactions_card_invoice_items transactions_card_invoice_items_purchase_id_fkey; Type: FK CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.transactions_card_invoice_items
    ADD CONSTRAINT transactions_card_invoice_items_purchase_id_fkey FOREIGN KEY (purchase_id) REFERENCES mecontrola.transactions_card_purchases(id);


--
-- Name: transactions_card_purchases transactions_card_purchases_card_fk; Type: FK CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.transactions_card_purchases
    ADD CONSTRAINT transactions_card_purchases_card_fk FOREIGN KEY (card_id) REFERENCES mecontrola.cards(id) ON DELETE RESTRICT;


--
-- Name: transactions_recurring_materializations transactions_recurring_materializations_template_id_fkey; Type: FK CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.transactions_recurring_materializations
    ADD CONSTRAINT transactions_recurring_materializations_template_id_fkey FOREIGN KEY (template_id) REFERENCES mecontrola.transactions_recurring_templates(id);


--
-- Name: transactions_recurring_templates transactions_recurring_templates_card_fk; Type: FK CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.transactions_recurring_templates
    ADD CONSTRAINT transactions_recurring_templates_card_fk FOREIGN KEY (card_id) REFERENCES mecontrola.cards(id) ON DELETE RESTRICT;


--
-- Name: user_identities user_identities_user_id_fkey; Type: FK CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.user_identities
    ADD CONSTRAINT user_identities_user_id_fkey FOREIGN KEY (user_id) REFERENCES mecontrola.users(id) ON DELETE CASCADE;


--
-- Name: user_whatsapp_history user_whatsapp_history_user_id_fkey; Type: FK CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.user_whatsapp_history
    ADD CONSTRAINT user_whatsapp_history_user_id_fkey FOREIGN KEY (user_id) REFERENCES mecontrola.users(id) ON DELETE CASCADE;


--
-- Name: workflow_steps workflow_steps_run_fkey; Type: FK CONSTRAINT; Schema: mecontrola; Owner: mecontrola
--

ALTER TABLE ONLY mecontrola.workflow_steps
    ADD CONSTRAINT workflow_steps_run_fkey FOREIGN KEY (run_id) REFERENCES mecontrola.workflow_runs(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

\unrestrict frdPr7GOvcfNcFYDgiNTsR2gQxRFsyKEd9k2n0RYxHBG9zet6WYyYcTDw2oagpw
