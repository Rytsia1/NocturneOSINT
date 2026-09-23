# Nocturne

Mobile-first OSINT research application. See `docs/` for the PRD, design system,
and architecture, and `AGENTS.md` for contributor/agent guidelines.

## Backend development

Requirements: Docker, Go 1.25+.

### Start everything (PostgreSQL + PostGIS, migrations, backend)

```bash
docker compose up -d --build
```

This starts `postgres` (PostGIS image), runs the `migrate` service once
(`migrate up`), then starts `backend` on http://localhost:8080.

```bash
curl localhost:8080/health   # 200 {"status":"ok"}    — process is up
curl localhost:8080/ready    # 200 {"status":"ready"} — database reachable; 503 if not
```

### API

| Endpoint | Success | Errors |
|---|---|---|
| `GET /api/sources?limit=20&cursor=…` | 200 `{"items":[…],"next_cursor":…}` newest first; `limit` 1–100 | 400 |
| `GET /api/sources/{id}` | 200 source | 400 bad id, 404 |
| `POST /api/sources` `{"name","url","description"}` | 201 source + `Location` | 400 invalid, 409 duplicate URL |
| `DELETE /api/sources/{id}` | 204 | 400 bad id, 404, 409 source still has articles |
| `POST /api/sources/{id}/ingest` (no body) | 200 `{"source_id","fetched","inserted","duplicates","invalid","truncated","errors":[{"item","reason"}]}` | 400 bad id, 404, 422 `source_not_ingestible` / `invalid_feed` / `feed_too_large`, 502 `upstream_error`, 504 `upstream_timeout` |
| `GET /api/articles?source_id=…&limit=20&cursor=…` | 200 `{"items":[…],"next_cursor":…}` by `published_at` (else `retrieved_at`), newest first | 400 |
| `GET /api/articles/{id}` | 200 article | 400 bad id, 404 |
| `POST /api/articles` `{"source_id","title","url","summary","published_at"}` | 201 article + `Location` | 400 invalid, 409 duplicate URL, 422 unknown source |
| `DELETE /api/articles/{id}` | 204 (also removes its media metadata) | 400 bad id, 404 |
| `POST /api/articles/{id}/media` `{"url","media_type","width","height"}` | 201 media + `Location`; `media_type` = `image`; `width`/`height` optional, 1–20 000 | 400 invalid, 404 unknown article, 409 URL already attached |
| `GET /api/articles/{id}/media` | 200 `{"items":[{"id","article_id","url","media_type","width","height",…}],"truncated":…}` oldest first (the first is the thumbnail candidate), max 100 | 400, 404 |
| `GET /api/articles/{id}/media/{media_id}` | 200 media | 400 bad id, 404 (also for another Article's media) |
| `DELETE /api/articles/{id}/media/{media_id}` | 204 (the Article stays) | 400 bad id, 404 |
| `GET /api/search/articles?q=…&source_id=…&published_from=…&published_to=…&limit=20&cursor=…` | 200 `{"items":[{…article…,"score"}],"next_cursor":…}` most relevant first | 400 `invalid_query`, `invalid_source_id`, `invalid_date`, `invalid_date_range`, `invalid_limit`, `invalid_cursor` |
| `GET /api/locations?limit=20&cursor=…` | 200 `{"items":[…],"next_cursor":…}` newest first | 400 |
| `GET /api/locations?min_lat=&min_lon=&max_lat=&max_lon=&limit=100` | 200 `{"items":[…],"truncated":…}` inside the box (edges inclusive) | 400 (incl. boxes crossing the antimeridian) |
| `GET /api/locations/nearby?lat=&lon=&radius_m=&limit=100` | 200 `{"items":[…],"truncated":…}` nearest first, each with `distance_m`; radius ≤ 50 000 m | 400 |
| `GET /api/locations/{id}` | 200 location | 400 bad id, 404 |
| `POST /api/locations` `{"name","latitude","longitude","precision"}` | 201 location + `Location`; `precision` ∈ country, region, city, specific, approximate | 400 invalid |
| `DELETE /api/locations/{id}` | 204 | 400 bad id, 404, 409 still referenced by an event |
| `GET /api/events?limit=20&cursor=…` | 200 `{"items":[…],"next_cursor":…}` by `occurred_at` (else `created_at`), newest first | 400 |
| `GET /api/events/{id}` | 200 event + `locations: [{location_id, role}]` | 400 bad id, 404 |
| `POST /api/events` `{"title","description","occurred_at","occurred_at_precision"}` | 201 event + `Location`; precision ∈ exact, day, month, year | 400 invalid |
| `DELETE /api/events/{id}` | 204 (removes its location links, not the locations) | 400 bad id, 404 |
| `GET /api/events/{id}/locations` | 200 `{"items":[{"role","location":{…}}],"truncated":…}`, sites first | 400 bad id, 404 |
| `POST /api/events/{id}/locations` `{"location_id","role"}` | 201 `{"role","location":{…}}`; role ∈ site, related | 400 invalid, 404 unknown event, 409 already attached, 422 unknown location |
| `DELETE /api/events/{id}/locations/{location_id}` | 204 | 400 bad id, 404 not attached |
| `POST /api/evidence` `{"article_id","event_id"}` | 201 evidence + `Location` | 400 invalid, 409 already linked, 422 unknown article/event |
| `GET /api/evidence/{id}` | 200 evidence | 400 bad id, 404 |
| `DELETE /api/evidence/{id}` | 204 (removes the link, not the Article or Event) | 400 bad id, 404 |
| `GET /api/events/{id}/articles?limit=20&cursor=…` | 200 `{"items":[{"evidence_id","article_id","source_id","title","url","published_at","retrieved_at"}],"next_cursor":…}` | 400, 404 |
| `GET /api/articles/{id}/events?limit=20&cursor=…` | 200 `{"items":[{"evidence_id","event_id","title","occurred_at","occurred_at_precision"}],"next_cursor":…}` | 400, 404 |

Errors use `{"error":{"code":"…","message":"…"}}`. URLs must be absolute
http(s) and are stored exactly as sent. `published_at` is optional (RFC 3339);
`retrieved_at` is set by the server when the article is recorded.

**Feed ingestion.** `POST /api/sources/{id}/ingest` fetches the Source's own
`url` as an RSS 2.0 or Atom feed and stores its new items as Articles of that
Source. It is manual (no scheduler), idempotent, and never modifies existing
Articles. Counters: `fetched` items considered (first 200), `inserted` new
Articles, `duplicates` URLs already stored or repeated in the feed, `invalid`
items skipped (reasons for up to 10 in `errors`); `truncated` means the feed
had more than 200 items; `media` image metadata rows stored and
`invalid_media` malformed image entries skipped. Only public http(s) addresses are fetched (15 s
timeout, 2 MiB, 5 redirects), so a feed on `localhost` or a private network is
refused with 422.

**Search.** `GET /api/search/articles` does PostgreSQL full-text search over
Article titles and summaries. `q` (required, 1–200 characters) accepts plain
words, `"quoted phrases"`, `OR` and `-excluded` words; words match whole and
case-insensitively, without stemming (`earthquakes` does not match
`earthquake`). Optional filters: `source_id`, and `published_from` /
`published_to` (RFC 3339, inclusive; Articles without `published_at` never
match a date filter). Results are paged with `limit` (1–100, default 20) and
the returned `next_cursor`. `score` is textual relevance only (title words
count more than summary words); it says nothing about credibility or accuracy.

**Article media.** Media rows are metadata about externally hosted images: the
URL (kept exactly as given), `media_type` (`image`), and `width`/`height` only
when the feed or client states them. Nocturne never downloads, stores, resizes
or proxies images, and never requests a media URL; clients load it directly.
Ingestion records, for each newly created Article, up to 10 images from
`<media:thumbnail>`, `<media:content>` (image type), RSS `<enclosure>` and Atom
`<link rel="enclosure">` with an `image/*` type.

### Run the backend locally instead of in Docker

```bash
docker compose up -d postgres migrate
cd backend
DATABASE_URL="postgres://nocturne:nocturne@localhost:5432/nocturne?sslmode=disable" go run ./cmd/server
```

### Migrations

Migrations live in `backend/migrations/` as `NNNNNN_name.up.sql` / `NNNNNN_name.down.sql`
and are applied with [golang-migrate](https://github.com/golang-migrate/migrate) via Docker:

```bash
docker compose run --rm migrate up        # apply all pending
docker compose run --rm migrate down 1    # roll back the latest
docker compose run --rm migrate version   # show current version
```

Never change the schema outside a migration.

### Tests

```bash
cd backend
go test ./...                                   # unit tests; DB tests skip
DATABASE_URL="postgres://nocturne:nocturne@localhost:5432/nocturne?sslmode=disable" go test ./...   # + integration tests
```

### Stop

```bash
docker compose down      # stops containers; database data is kept
docker compose down -v   # also DELETES the database volume
```

### Environment variables

See `.env.example`. The backend reads `APP_ENV`, `PORT`, `DATABASE_URL`, `LOG_LEVEL`;
`docker compose` reads `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` from `.env`
if present (defaults are development-only). Never commit `.env`.
