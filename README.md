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
| `DELETE /api/sources/{id}` | 204 | 400 bad id, 404 |

Errors use `{"error":{"code":"…","message":"…"}}`. Source URLs must be absolute
http(s) and are stored exactly as sent.

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
