-- Metadata about externally hosted images of an Article. Nocturne stores the
-- URL and the dimensions the source states, never image bytes.
CREATE TABLE article_media (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    -- CASCADE: media metadata has no meaning without its Article.
    article_id uuid        NOT NULL REFERENCES articles (id) ON DELETE CASCADE,
    url        text        NOT NULL CHECK (octet_length(url) BETWEEN 1 AND 2048),
    media_type text        NOT NULL CHECK (media_type IN ('image')),
    width      integer     CHECK (width BETWEEN 1 AND 20000),  -- NULL when not stated
    height     integer     CHECK (height BETWEEN 1 AND 20000), -- NULL when not stated
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    -- Not globally unique: several Articles may use the same image. Its index
    -- also serves Article → media lookups and the cascade on Article delete.
    CONSTRAINT article_media_article_url_key UNIQUE (article_id, url)
);
