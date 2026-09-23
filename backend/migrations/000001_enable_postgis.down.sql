-- CASCADE: the postgis/postgis image also installs postgis_topology and
-- postgis_tiger_geocoder, which depend on postgis.
DROP EXTENSION IF EXISTS postgis CASCADE;
