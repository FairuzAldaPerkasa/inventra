BEGIN;

CREATE TABLE public.user_sessions (
    token_hash BYTEA PRIMARY KEY,

    user_id UUID NOT NULL
        REFERENCES public.users(id)
        ON DELETE CASCADE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,

    CONSTRAINT user_sessions_token_hash_length
        CHECK (octet_length(token_hash) = 32),

    CONSTRAINT user_sessions_expiry_valid
        CHECK (expires_at > created_at)
);

CREATE INDEX user_sessions_user_id_idx
    ON public.user_sessions (user_id);

CREATE INDEX user_sessions_expires_at_idx
    ON public.user_sessions (expires_at);

COMMIT;