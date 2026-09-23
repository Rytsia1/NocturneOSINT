CREATE TABLE articles (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    -- RESTRICT: deleting a Source must never silently delete the Articles it
    -- published; their Articles have to be removed first.
    source_id    uuid        NOT NULL REFERENCES sources (id) ON DELETE RESTRICT,
    title        text        NOT NULL CHECK (char_length(title) BETWEEN 1 AND 500),
    -- Stored exactly as provided (provenance). Unique by exact string only:
    -- no canonicalization. Drop the constraint if that proves too strict.
    url          text        NOT NULL CHECK (octet_length(url) BETWEEN 1 AND 2048),
    summary      text        NOT NULL DEFAULT '' CHECK (char_length(summary) <= 2000),
    published_at timestamptz,                        -- as stated by the Source; NULL when unknown
    retrieved_at timestamptz NOT NULL DEFAULT now(), -- when Nocturne recorded the Article
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    -- Feed sort key: publication time when known, otherwise when recorded.
    feed_at      timestamptz GENERATED ALWAYS AS (COALESCE(published_at, retrieved_at)) STORED,
    CONSTRAINT articles_url_key UNIQUE (url)
);

-- Recent-articles feed (keyset pagination on feed_at, id).
CREATE INDEX articles_feed_idx ON articles (feed_at, id);
-- Per-Source feed; also serves the FK check when a Source is deleted.
CREATE INDEX articles_source_feed_idx ON articles (source_id, feed_at, id);
