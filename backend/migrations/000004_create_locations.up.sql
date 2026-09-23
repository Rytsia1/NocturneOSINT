CREATE TABLE locations (
    id         uuid                  PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text                  NOT NULL CHECK (char_length(name) BETWEEN 1 AND 200),
    -- The authoritative coordinate: POINT(longitude latitude), WGS84.
    -- The typmod rejects any other geometry type or SRID.
    point      geometry(Point, 4326) NOT NULL,
    -- How precise the coordinate is; never imply more than the source supports.
    precision  text                  NOT NULL
               CHECK (precision IN ('country', 'region', 'city', 'specific', 'approximate')),
    created_at timestamptz           NOT NULL DEFAULT now(),
    updated_at timestamptz           NOT NULL DEFAULT now()
);

-- Bounding-box (map viewport) queries: point && envelope.
CREATE INDEX locations_point_idx ON locations USING GIST (point);
-- Nearby queries measure metres on the spheroid via point::geography.
CREATE INDEX locations_point_geog_idx ON locations USING GIST ((point::geography));
