CREATE TABLE sources (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 200),
    -- Stored exactly as provided (provenance); byte limit keeps the unique
    -- index under PostgreSQL's btree row size limit.
    url         text        NOT NULL CHECK (octet_length(url) BETWEEN 1 AND 2048),
    description text        NOT NULL DEFAULT '' CHECK (char_length(description) <= 2000),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT sources_url_key UNIQUE (url)
);

-- No index on (created_at, id) for List ordering yet: the table is small.
-- Add one if listing sources shows up as slow.
