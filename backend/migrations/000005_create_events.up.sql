CREATE TABLE events (
    id                    uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    title                 text        NOT NULL CHECK (char_length(title) BETWEEN 1 AND 500),
    description           text        NOT NULL DEFAULT '' CHECK (char_length(description) <= 2000),
    -- When the event happened, if known; NULL is never replaced by a guess.
    occurred_at           timestamptz,
    -- How much of occurred_at is known: 'day' means only the (UTC) date, etc.
    occurred_at_precision text        CHECK (occurred_at_precision IN ('exact', 'day', 'month', 'year')),
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    -- Feed sort key: occurrence time when known, otherwise when recorded.
    feed_at               timestamptz GENERATED ALWAYS AS (COALESCE(occurred_at, created_at)) STORED,
    CONSTRAINT events_occurred_at_precision_pair
        CHECK ((occurred_at IS NULL) = (occurred_at_precision IS NULL))
);

CREATE INDEX events_feed_idx ON events (feed_at, id);

-- An Event's geographic references. Coordinates live only in locations.
CREATE TABLE event_locations (
    -- CASCADE: the associations belong to the Event.
    event_id    uuid NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    -- RESTRICT: a Location still referenced by an Event cannot be deleted.
    location_id uuid NOT NULL REFERENCES locations (id) ON DELETE RESTRICT,
    -- 'site': where the event happened; 'related': another connected place.
    role        text NOT NULL CHECK (role IN ('site', 'related')),
    PRIMARY KEY (event_id, location_id)
);

-- Location → Events lookups (future "events near here") and the RESTRICT check.
CREATE INDEX event_locations_location_idx ON event_locations (location_id);
