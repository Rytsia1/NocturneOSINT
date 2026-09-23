-- Full-text search over Article title (weight A) and summary (weight B).
-- 'simple' is language-neutral (lower-cases words, no stemming or stop
-- words): sources are multilingual and no language is assumed. Generated from
-- the row itself, so it can never drift from the Article text.
ALTER TABLE articles ADD COLUMN search_vector tsvector GENERATED ALWAYS AS (
    setweight(to_tsvector('simple'::regconfig, title), 'A') ||
    setweight(to_tsvector('simple'::regconfig, summary), 'B')
) STORED;

CREATE INDEX articles_search_idx ON articles USING GIN (search_vector);
