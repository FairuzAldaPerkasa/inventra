BEGIN;

CREATE TABLE public.products (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    sku VARCHAR(50) NOT NULL,
    name VARCHAR(150) NOT NULL,
    category VARCHAR(100) NOT NULL,
    brand VARCHAR(100) NOT NULL,
    price NUMERIC(15, 2) NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,

    CONSTRAINT products_sku_unique UNIQUE (sku),
    CONSTRAINT products_sku_format
        CHECK (sku ~ '^[A-Z0-9][A-Z0-9_-]*$'),
    CONSTRAINT products_name_not_blank
        CHECK (LENGTH(TRIM(name)) > 0),
    CONSTRAINT products_category_not_blank
        CHECK (LENGTH(TRIM(category)) > 0),
    CONSTRAINT products_brand_not_blank
        CHECK (LENGTH(TRIM(brand)) > 0),
    CONSTRAINT products_price_valid
        CHECK (price >= 0 AND price <> 'NaN'::numeric),
    CONSTRAINT products_version_positive
        CHECK (version > 0)
);

COMMIT;