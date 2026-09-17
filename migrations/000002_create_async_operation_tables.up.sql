BEGIN;

CREATE TABLE public.operations (
    id UUID PRIMARY KEY,
    action VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    product_id BIGINT REFERENCES public.products(id),
    error_code VARCHAR(50),
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMPTZ,

    CONSTRAINT operations_action_valid CHECK (
        action IN ('product.create', 'product.update', 'product.delete')
    ),
    CONSTRAINT operations_status_valid CHECK (
        status IN ('pending', 'succeeded', 'failed')
    ),
    CONSTRAINT operations_completion_valid CHECK (
        (status = 'pending' AND completed_at IS NULL)
        OR
        (status IN ('succeeded', 'failed') AND completed_at IS NOT NULL)
    ),
    CONSTRAINT operations_result_valid CHECK (
        (
            status = 'pending'
            AND error_code IS NULL
            AND error_message IS NULL
        )
        OR
        (
            status = 'succeeded'
            AND product_id IS NOT NULL
            AND error_code IS NULL
            AND error_message IS NULL
        )
        OR
        (
            status = 'failed'
            AND error_code IS NOT NULL
            AND error_message IS NOT NULL
        )
    )
);

CREATE TABLE public.outbox_messages (
    operation_id UUID PRIMARY KEY REFERENCES public.operations(id),
    routing_key VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL,
    publish_attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT outbox_routing_key_valid CHECK (
        routing_key IN ('product.create', 'product.update', 'product.delete')
    ),
    CONSTRAINT outbox_payload_object CHECK (
        jsonb_typeof(payload) = 'object'
    ),
    CONSTRAINT outbox_attempts_nonnegative CHECK (
        publish_attempts >= 0
    )
);

CREATE INDEX outbox_pending_idx
    ON public.outbox_messages (next_attempt_at, created_at)
    WHERE published_at IS NULL;

CREATE TABLE public.audit_logs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    operation_id UUID NOT NULL UNIQUE REFERENCES public.operations(id),
    product_id BIGINT REFERENCES public.products(id),
    action VARCHAR(50) NOT NULL,
    outcome VARCHAR(20) NOT NULL,
    actor_id UUID,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT audit_action_valid CHECK (
        action IN ('product.create', 'product.update', 'product.delete')
    ),
    CONSTRAINT audit_outcome_valid CHECK (
        outcome IN ('succeeded', 'failed')
    ),
    CONSTRAINT audit_details_object CHECK (
        jsonb_typeof(details) = 'object'
    )
);

CREATE INDEX audit_product_history_idx
    ON public.audit_logs (product_id, created_at DESC);

COMMIT;