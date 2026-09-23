
## Nocturne — AI Coding Agent Guidelines

This document defines how AI coding agents should work inside the Nocturne repository.

It is the operational counterpart to:

- `docs/PRD.md`
- `docs/Design-System.md`
- `docs/architecture.md`

When implementing changes, treat these documents as the product and architecture source of truth.

---

# 1. Project Overview

Nocturne is a **mobile-first OSINT research application**.

The primary platform is:

- Android
- Kotlin
- Jetpack Compose

The backend is:

- Go
- REST API
- PostgreSQL
- PostGIS

The Android application is designed to be:

- lightweight
- offline-aware
- mobile-first
- performance-conscious
- research-oriented

The backend is designed to be:

- simple
- modular
- maintainable
- geospatially capable
- provenance-aware

## Current Status

The backend is currently ahead of the client:

```text
Backend:
Implemented through Step 7 (controlled RSS / Atom feed ingestion).

Android:
Not yet implemented. The repository contains no android/ code yet;
the Android stack above is the planned client architecture.
```

Implemented backend domains:

```text
Source
Article
Location
Event
EventLocation
Evidence
```

Current data model:

```text
Source
   │
   ▼
Article
   │
   │ Evidence
   ▼
Event
   │
   ▼
EventLocation
   │
   ▼
Location
   │
   ▼
PostGIS
```

Core rule:

```text
Article does NOT directly reference Event.
Event does NOT directly reference Article.

Evidence is the explicit provenance relationship.
```

Do not add `articles.event_id` or `events.article_id`. Evidence records that an Article provides information about an Event; it does not establish that the Event is true or that the Article is reliable.

---

# 2. Core Principle

> **Build the smallest correct thing that moves the project forward.**

Do not implement future architecture merely because it is mentioned in the documentation.

Do not turn a small feature into a platform-wide refactor.

Do not add infrastructure without a concrete requirement.

---

# 3. Source of Truth

When making decisions, use this priority:

```text
1. Current user request
2. PRD.md
3. architecture.md
4. Design-System.md
5. Existing implementation
6. Agent assumptions
```

If the current request conflicts with the existing implementation, follow the current request unless it violates the architecture or product requirements.

If requirements are genuinely ambiguous, inspect the repository first.

Do not invent requirements unnecessarily.

---

# 4. Before Making Changes

Before modifying code:

1. Inspect the repository structure.
2. Identify the relevant application/module.
3. Read the relevant existing implementation.
4. Check current dependencies.
5. Check existing tests.
6. Check related documentation.
7. Determine the smallest set of files required.

Do not immediately rewrite existing code.

Prefer incremental changes.

---

# 5. Scope Discipline

Every task must have an explicit scope.

For example:

```text
Task:
Implement Article API endpoint.
```

The agent should NOT automatically implement:

- authentication
- Redis
- search engine
- article ingestion
- AI extraction
- synchronization
- unrelated UI changes

unless explicitly required.

---

# 6. Step-by-Step Development

Nocturne is intentionally developed incrementally.

This is the single authoritative development roadmap. These are development milestones, not necessarily the final product feature hierarchy. Each milestone should remain independently reviewable.

```text
Step 1   Backend Foundation                     ✓ implemented
        ↓
Step 2   PostgreSQL + PostGIS                   ✓ implemented
        ↓
Step 3   Source                                 ✓ implemented
        ↓
Step 4   Article + Source Provenance            ✓ implemented
        ↓
Step 5   Location + Event                       ✓ implemented
        ↓
Step 6   Evidence / Provenance                  ✓ implemented
        ↓
Step 7   Controlled RSS / Atom Ingestion        ✓ implemented
        ↓
Step 8   Article Media / Feed Thumbnails        ← next
        ↓
Step 9   Search
        ↓
Step 10  Android Foundation
        ↓
Step 11  Room / Offline-Aware Client
        ↓
Step 12  Global Map Android Integration
        ↓
Step 13  Investigation / Dossier Client
        ↓
Step 14  Advanced Enrichment
```

Step 5 covers the Location domain with PostGIS spatial queries (bounding-box and nearby) plus Events and their EventLocation associations.

Step 8 direction (not yet implemented):

```text
RSS / Atom feed
      ↓
Article
      +
optional external image metadata (URL, type, stated dimensions)
```

not:

```text
Backend
 ↓
download image
 ↓
store image
```

Media hosting, image proxying, resizing and CDN infrastructure remain deferred.

Do not skip multiple stages unless explicitly instructed.

When a stage is complete, stop.

Do not automatically continue to the next stage.

---

# 7. Repository Structure

Expected high-level structure:

```text
nocturne/
│
├── android/          (planned — not yet present)
│
├── backend/
│
├── docs/
│
├── docker-compose.yml
│
├── README.md
└── AGENTS.md
```

Backend (current):

```text
backend/
├── cmd/
│   └── server/
├── internal/
│   ├── config/
│   ├── database/
│   ├── domain/
│   ├── repository/
│   ├── http/
│   └── ingest/       (feed ingestion service: fetch → parse → normalize → store)
├── migrations/
├── Dockerfile
├── go.mod
└── go.sum
```

Android structure may evolve according to feature requirements.

Do not force an architecture mechanically if the repository already has a clean equivalent.

---

# 8. Technology Rules

## Android

Planned client stack (no Android code exists yet; applies from Step 10). Use:

- Kotlin
- Jetpack Compose
- Android SDK
- Room
- Kotlin Coroutines
- Ktor Client
- MapLibre

Do not introduce React Native, Flutter, or another UI framework.

---

## Backend

Use:

- Go
- REST
- PostgreSQL
- PostGIS

Do not introduce Python into the backend unless a specific future ML/data-processing requirement justifies it.

---

## Infrastructure

Use:

- Docker
- Docker Compose
- GitHub Actions

Docker is for backend infrastructure/development.

The Android application must NOT be containerized.

---

# 9. Dependency Policy

Before adding a dependency, determine:

1. Is it actually necessary?
2. Can the standard library solve the problem?
3. Is there already an existing dependency that solves it?
4. Is the dependency maintained?
5. Does it significantly increase project complexity?

Prefer fewer dependencies.

Do not add a library simply because it is popular.

---

# 10. Go Guidelines

Write idiomatic Go.

Prefer:

- small functions
- explicit dependencies
- context propagation
- clear errors
- simple interfaces
- standard library solutions where practical

Avoid:

- unnecessary abstractions
- generic repository frameworks
- dependency injection frameworks
- service locator patterns
- global mutable state
- giant service structs
- giant handler functions

---

# 11. Go Package Boundaries

Use clear responsibility boundaries.

Preferred flow:

```text
HTTP Handler
     ↓
Application Service
     ↓
Repository
     ↓
Database
```

Handlers should not contain complex business logic.

Handlers should not directly execute SQL.

Repositories should not know about HTTP responses.

Domain models should not depend on HTTP implementation details.

---

# 12. Error Handling

Errors should be explicit.

Prefer:

```go
if err != nil {
    return err
}
```

over silently ignoring failures.

Do not expose internal errors directly to API consumers.

External response:

```json
{
  "error": {
    "code": "internal_error",
    "message": "internal server error"
  }
}
```

Internal logs may contain more diagnostic information.

Never expose:

- SQL statements containing secrets
- passwords
- API keys
- authentication tokens
- stack traces

to users.

---

# 13. Context Handling

Go functions that perform:

- database operations
- network requests
- long-running work

should accept `context.Context`.

Do not create unnecessary `context.Background()` deep inside application logic.

Propagate request context where appropriate.

---

# 14. Database Rules

Use PostgreSQL.

Use PostGIS for geographic data.

All schema changes must use migrations.

Never instruct users to manually modify production schema.

---

# 15. Database IDs

Use the repository's established ID strategy consistently.

If the project has not established one yet:

Prefer UUIDs.

Do not mix:

```text
integer
UUID
string
```

without a concrete reason.

---

# 16. Database Timestamps

Store timestamps consistently in UTC.

Distinguish:

```text
published_at
retrieved_at
created_at
updated_at
```

These represent different concepts.

Do not substitute one for another merely because they are all timestamps.

---

# 17. SQL Rules

Use parameterized queries.

Never construct SQL by concatenating user input.

Bad:

```text
"SELECT * FROM articles WHERE title = '" + query + "'"
```

Good:

```text
SELECT * FROM articles WHERE title = $1
```

Use appropriate indexes based on actual query patterns.

Do not blindly index every column.

---

# 18. PostGIS Rules

Geospatial data must use appropriate spatial types.

Preferred:

```text
geometry(Point, 4326)
```

for point locations.

Do not treat latitude/longitude as sufficient justification for claiming PostGIS support.

Use spatial indexes where appropriate.

---

# 19. Geographic Precision

This is an important Nocturne rule.

Never fabricate geographic precision.

If the source only identifies:

```text
Taiwan
```

do not automatically create:

```text
Taipei, Taiwan
```

as the event location.

The system must preserve precision such as:

```text
Country
Region
City
Specific Location
Approximate
Unknown
```

If coordinates are inferred or approximate, the data model and UI must communicate that appropriately.

---

# 20. API Design

Use REST.

Responses should be:

- JSON
- predictable
- compact
- mobile-friendly

Use pagination for collections.

Avoid returning large nested datasets.

Example:

```text
GET /api/articles
GET /api/articles/{id}

GET /api/entities
GET /api/entities/{id}

GET /api/events
GET /api/events/{id}

GET /api/locations
GET /api/locations/{id}

GET /api/investigations
GET /api/investigations/{id}
```

Only implement endpoints required by the current task.

---

# 21. API Pagination

Do not return thousands of records.

For large collections, prefer cursor-based pagination.

Example:

```text
GET /api/articles?limit=20&cursor=...
```

Respect mobile bandwidth and memory constraints.

---

# 22. Android Guidelines

Sections 22–26 describe the planned Android client (Steps 10–13). No Android code exists yet; do not describe these as implemented.

Use modern Android architecture.

Preferred:

```text
Compose UI
    ↓
ViewModel
    ↓
Repository
    ↓
Room / API
```

The UI must not directly access:

- Room DAOs
- SQL
- HTTP clients

---

# 23. Jetpack Compose Rules

Use reusable components for recurring patterns.

Prefer:

```text
ArticleCard
SourceCard
EntityCard
EventCard
LocationCard
InvestigationCard
```

over duplicated UI implementations.

Do not create a component for every tiny piece of UI.

Avoid unnecessary recomposition.

Use stable state patterns.

---

# 24. Android State Handling

Screens should explicitly handle:

```text
Loading
Success
Empty
Error
Offline
```

Do not assume every API request succeeds.

Do not display blank screens when data fails to load.

---

# 25. Offline-Aware Architecture

Nocturne is designed to be offline-aware. Offline support is a future Android capability (Step 11); Nocturne does not support offline mode yet.

It is NOT required to be completely offline.

```text
Backend currently supports:
- bounded API responses
- pagination
- compact resource representations

Future Android client:
- Room cache
- cached investigations
- cached articles
- offline-aware UI
- synchronization
```

The preferred flow is:

```text
UI
 ↓
Repository
 ↓
Room
 ↓
UI
```

with remote synchronization:

```text
Repository
 ↓
API
 ↓
Server
 ↓
Room
 ↓
UI
```

Previously retrieved information should remain accessible when possible.

---

# 26. Room Rules

Room is the Android local persistence layer.

Use it for:

- cached articles
- saved items
- investigations
- entities
- events
- locations
- local notes
- synchronization state

Do not use Room as a second independent backend.

The server remains authoritative for server-managed data.

---

# 27. Network Failure

Assume network requests can fail.

Handle:

- timeout
- no connection
- server error
- malformed response
- rate limit
- authentication failure

The application should provide meaningful user-facing states.

---

# 28. Image Handling

Images are potentially expensive on mobile.

Always consider:

- lazy loading
- caching
- image dimensions
- compression
- placeholders
- failed image loading

Do not download full-resolution images when a smaller version is sufficient.

Do not introduce fake images when source imagery is unavailable.

Backend direction (Step 8, not yet implemented): store only optional external image metadata for Articles (URL, media type, dimensions only when the source states them). The backend must not download, store, proxy, resize or cache remote media; hosting, proxying and CDN infrastructure remain deferred.

---

# 29. Map Development Rules

MapLibre is the intended map implementation.

The map should remain performant on mobile.

Avoid rendering huge numbers of individual markers.

Prefer:

- clustering
- viewport-based queries
- incremental loading

The backend already provides the spatial queries a map needs; the Android Global Map (Step 12) does not exist yet:

```text
CURRENT BACKEND

Location
↓
PostGIS
↓
Bounding-box / nearby queries


FUTURE CLIENT

Kotlin
↓
Ktor
↓
MapLibre
↓
Global Map
```

---

# 30. Map UX Rule

The Global Map exists to answer:

> "Where is this information connected?"

It is not intended to look like:

- a military targeting system
- radar
- tactical command software
- a battlefield interface

Do not introduce:

- targeting reticles
- fake radar sweeps
- tactical grids
- glowing military overlays
- decorative satellite effects

unless explicitly requested for a legitimate visual purpose.

---

# 31. OSINT Data Rules

Nocturne works with publicly available information.

When implementing ingestion or enrichment:

- preserve source URLs
- preserve publication timestamps
- preserve retrieval timestamps
- preserve attribution
- respect API terms
- respect rate limits
- respect licensing
- avoid bypassing access controls

Do not implement credential theft, unauthorized access, or private-data acquisition.

## Current Ingestion (Step 7)

Feed ingestion is implemented in `backend/internal/ingest` and is intentionally conservative:

```text
Source.url
   ↓
Fetcher
   ↓
RSS / Atom Parser
   ↓
Normalizer
   ↓
Deduplication (by Article URL)
   ↓
Article
```

Characteristics:

- manual trigger: `POST /api/sources/{id}/ingest`
- the stored Source's URL is the feed URL
- RSS 2.0 and Atom supported
- URL-based Article deduplication; repeated ingestion is idempotent
- bounded feed size, bounded item count, request timeout, redirect limit
- SSRF / private-address protection
- no scheduler
- no AI extraction
- no automatic Event, Location or Evidence creation

Current versus future execution:

```text
CURRENT

POST /api/sources/{id}/ingest
        ↓
synchronous ingestion
        ↓
Article


FUTURE

Scheduler / Job Queue
        ↓
Worker
        ↓
same ingestion pipeline
```

No scheduler, queue or worker exists yet; background workers remain future infrastructure.

Agents must NOT:

- add arbitrary URL ingestion
- bypass stored Source objects
- scrape HTML
- add browser automation
- add JavaScript rendering
- invent Article metadata
- overwrite existing Articles during repeated ingestion
- automatically create Events
- automatically create Locations
- automatically create Evidence

---

# 32. Source Provenance

Source provenance is a core architectural requirement.

Whenever possible, preserve:

```text
source
url
published_at
retrieved_at
```

Do not remove provenance merely to simplify a response object.

---

# 33. Source vs Interpretation

Keep these concepts separate:

```text
Source-derived information
        ≠
System-derived information
        ≠
User notes
        ≠
AI-generated information
```

Do not silently transform interpretation into fact.

---

# 34. AI Features

AI is not a core dependency of the initial architecture.

```text
Current:
No AI/LLM dependency.

Future:
AI may be introduced as an enrichment layer (Step 14) if a concrete
product requirement justifies it.
```

Potential future enrichment (none of this is implemented; Nocturne does not currently perform automatic intelligence analysis):

- entity extraction
- location extraction
- event extraction
- semantic search

Do not introduce:

- LLM APIs
- vector databases
- embeddings
- RAG
- AI agents

unless explicitly requested.

If AI is introduced later:

- clearly label generated output
- preserve source provenance
- avoid presenting generated claims as source facts
- provide appropriate uncertainty/context

---

# 35. Security Rules

Never commit:

- API keys
- passwords
- tokens
- private credentials
- production secrets

Use environment variables or secure secret management.

Never disable TLS verification merely to make development work.

Do not bypass authentication or authorization checks for convenience.

---

# 36. Logging Rules

Use structured logging.

Include useful diagnostic information:

```text
request_id
method
path
status
duration
```

Never log:

- passwords
- tokens
- API keys
- private user information

---

# 37. Testing

Every meaningful feature should include appropriate tests.

Backend:

- unit tests
- repository tests
- handler tests
- integration tests when appropriate

Android:

- ViewModel tests
- repository tests
- database tests
- critical UI tests

Do not write tests solely to increase coverage numbers.

Tests should verify behavior.

---

# 38. Test Before Completion

Before declaring a task complete:

### Backend

Run:

```text
gofmt -l .
go vet ./...
go test ./...
go build ./...
```

where applicable.

When database integration is required:

- use real PostgreSQL/PostGIS (`docker compose up -d`, then run tests with `DATABASE_URL` set)
- do not replace integration tests with mocks merely to avoid infrastructure
- verify migrations (up and down) when schema changes are involved

Docker remains development infrastructure for the backend, not the Android runtime.

### Android

(Applies once the Android client exists.)

Run the appropriate:

```text
./gradlew test
./gradlew lint
./gradlew assembleDebug
```

commands according to the repository configuration.

Fix failures before completion.

---

# 39. Migration Safety

When modifying database schema:

1. Create a migration.
2. Run the migration locally.
3. Verify the resulting schema.
4. Test affected queries.
5. Verify existing data remains compatible where relevant.

Do not modify migration history that has already been applied unless explicitly performing a controlled migration repair.

---

# 40. Documentation Rules

When architecture changes materially, update:

```text
docs/architecture.md
```

When product behavior changes materially, update:

```text
docs/PRD.md
```

When reusable UI conventions change, update:

```text
docs/Design-System.md
```

Do not let documentation become obviously inconsistent with the implementation.

---

# 41. Git Discipline

Keep changes focused.

Prefer commits such as:

```text
feat: add article repository
feat: add article API
fix: handle article fetch timeout
test: add article repository tests
refactor: simplify location query
```

Avoid giant commits containing unrelated features.

Do not rewrite unrelated files.

Also:

- one logical milestone per commit where practical
- no unnecessary history rewrites
- no force push merely to make history aesthetically cleaner
- do not amend or rewrite already-pushed history unless explicitly requested
- commit only when explicitly asked

---

# 42. Refactoring Rules

Refactor when:

- duplication is becoming meaningful
- code is difficult to test
- responsibilities are clearly mixed
- architecture is being violated

Do not refactor simply because a different style looks prettier.

Avoid large refactors during feature implementation unless required.

---

# 43. Dependency Changes

When adding a dependency:

Document:

- why it is needed
- what problem it solves
- why existing dependencies are insufficient

Avoid introducing multiple libraries for the same purpose.

---

# 44. Performance Rules

Do not optimize based on assumptions alone.

First identify:

```text
What is slow?
Why is it slow?
What is the measured bottleneck?
```

Then optimize.

Potential mobile bottlenecks:

- excessive recomposition
- large images
- large API payloads
- excessive database queries
- too many map markers
- unnecessary synchronization

Potential backend bottlenecks:

- unindexed queries
- N+1 queries
- excessive payloads
- inefficient spatial queries
- unnecessary external requests

---

# 45. Avoid Premature Infrastructure

Do NOT introduce:

```text
Kubernetes
Kafka
Elasticsearch
Redis
Graph database
Vector database
Microservices
Service mesh
```

unless the current implementation has a demonstrated need.

A simple system that works is preferable to a sophisticated system that is difficult to maintain.

Search (Step 9) follows the same rule:

```text
Current:
Resource listing and filtering exist where already implemented.

Future:
Cross-resource search.

Initial approach:
PostgreSQL search capabilities.

Dedicated search infrastructure only if actual
performance requirements justify it.
```

Nocturne has no Elasticsearch/OpenSearch support.

---

# 46. Definition of Done

A task is complete when:

1. Requested functionality works.
2. Existing functionality is not unnecessarily broken.
3. Tests pass.
4. Formatting/linting passes where applicable.
5. Error states are handled.
6. Documentation is updated if required.
7. No secrets were introduced.
8. No unrelated architecture was added.
9. The implementation follows the project's existing conventions.

---

# 47. When Something Goes Wrong

If implementation fails:

1. Read the actual error.
2. Identify the failing layer.
3. Reproduce the issue.
4. Fix the smallest underlying cause.
5. Run the relevant tests again.
6. Check for regressions.

Do not randomly change multiple parts of the architecture.

Do not hide errors.

Do not disable tests to make a build pass.

---

# 48. When Requirements Are Ambiguous

Use this process:

```text
Inspect existing code
        ↓
Inspect PRD
        ↓
Inspect architecture
        ↓
Determine smallest reasonable interpretation
        ↓
Implement
```

If ambiguity materially affects architecture or product behavior, stop and ask for clarification.

Do not invent major product decisions.

---

# 49. Agent Output Format

When completing a task, report:

## Changes

Briefly list what changed.

## Files

List important files modified.

## Tests

List tests/build commands executed and their results.

## Architecture Impact

State whether architecture changed.

## Remaining Work

Mention only work that is directly relevant to the current task.

Example:

```text
## Changes
- Added ArticleRepository
- Added GET /api/articles
- Added pagination

## Files
- backend/internal/repository/article.go
- backend/internal/http/handlers/article.go

## Tests
- go test ./... ✓
- go vet ./... ✓
- go build ./... ✓

## Architecture Impact
None.

## Remaining Work
Article detail endpoint will be implemented in the next step.
```

---

# 50. Stop Conditions

The agent MUST stop when:

- the requested task is complete
- tests pass
- the defined scope has been implemented

Do not continue implementing "obvious next features."

Do not automatically:

```text
implement → refactor everything → add auth → add Redis → add AI
```

One task at a time.

---

# 51. Nocturne Engineering Philosophy

The project follows:

> **Simple before clever.**

> **Measured before optimized.**

> **Provenance before inference.**

> **Mobile constraints before desktop assumptions.**

> **Working architecture before theoretical scalability.**

> **Incremental delivery before massive implementation.**

---

# 52. Final Rule

When uncertain, ask:

> **"Is this necessary for the current task?"**

If the answer is no, do not implement it yet.

Nocturne should grow from a small working system into a capable research platform—not from a giant architecture diagram into an unfinished application.
