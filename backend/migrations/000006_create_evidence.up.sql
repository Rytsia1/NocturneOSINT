-- Evidence records that an Article provides information about an Event. It
-- does not say the Event is true or the Article reliable. Articles and Events
-- stay uncoupled: neither table references the other.
CREATE TABLE evidence (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    -- CASCADE both ways: the record is only meaningful while both ends exist,
    -- so deleting either one removes it rather than leaving an orphan.
    article_id uuid        NOT NULL REFERENCES articles (id) ON DELETE CASCADE,
    event_id   uuid        NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    -- One record per Article/Event pair; its index also serves Article → Events.
    CONSTRAINT evidence_article_event_key UNIQUE (article_id, event_id)
);

-- Event → Articles, and the FK check when an Event is deleted.
CREATE INDEX evidence_event_idx ON evidence (event_id);
