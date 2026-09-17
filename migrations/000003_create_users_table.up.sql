BEGIN;

CREATE TABLE public.users (
    id UUID PRIMARY KEY,

    name VARCHAR(150) NOT NULL,
    email VARCHAR(254) NOT NULL,
    password_hash TEXT NOT NULL,

    role VARCHAR(20) NOT NULL DEFAULT 'viewer',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT users_name_not_blank
        CHECK (length(btrim(name)) > 0),

    CONSTRAINT users_email_normalized
        CHECK (
            email = lower(btrim(email))
            AND length(email) > 0
        ),

    CONSTRAINT users_email_unique
        UNIQUE (email),

    CONSTRAINT users_password_hash_not_blank
        CHECK (length(btrim(password_hash)) > 0),

    CONSTRAINT users_role_valid
        CHECK (role IN ('admin', 'staff', 'viewer'))
);

COMMIT;