
**Version:** 1.0
**Status:** Architecture Foundation
**Platform:** Android
**Backend:** Go
**Database:** PostgreSQL + PostGIS

---

# 1. Architecture Overview

Nocturne is a mobile-first OSINT research application.

The architecture is divided into two primary systems:

1. **Android Client**
2. **Backend Platform**

The Android application is responsible for:

- presentation
- user interaction
- local persistence
- offline-aware behavior
- map rendering
- API communication

The backend is responsible for:

- business logic
- data persistence
- source provenance
- search
- geospatial queries
- data ingestion
- synchronization
- API delivery

High-level architecture:

```text
┌──────────────────────────────────────────────┐
│                ANDROID CLIENT                │
│                                              │
│  Jetpack Compose                            │
│        │                                     │
│  Presentation / ViewModels                   │
│        │                                     │
│  Repository Layer                            │
│       ┌┴───────────────┐                     │
│       │                │                     │
│   Room / SQLite     Ktor Client              │
│       │                │                     │
│       └────────┬───────┘                     │
│                │                             │
└────────────────┼─────────────────────────────┘
                 │ HTTPS
                 ▼
┌──────────────────────────────────────────────┐
│                 GO BACKEND                   │
│                                              │
│  HTTP / REST API                             │
│        │                                     │
│  Application / Domain Logic                  │
│        │                                     │
│  Repository / Data Access                    │
│        │                                     │
│  PostgreSQL + PostGIS                        │
│                                              │
└──────────────────────────────────────────────┘
```

Future infrastructure:

```text
                Go Backend
                    │
          ┌─────────┴─────────┐
          │                   │
     PostgreSQL            Redis
      + PostGIS             │
          │              Cache / Jobs
          │
    Go Worker Processes
          │
          ▼
    Public Data Sources
```

Redis and worker processes are **future components** and are not required for the initial architecture.

---

# 2. Architectural Principles

## 2.1 Mobile First

The backend must be designed around mobile constraints.

The API should avoid unnecessarily large responses.

Prefer:

- pagination
- compact DTOs
- lazy loading
- selective fields
- cached data
- incremental synchronization

---

## 2.2 Offline-Aware

The Android client should not assume continuous connectivity.

Previously retrieved data should be stored locally.

The local database acts as a cache and local source of truth for the UI.

The application should remain useful when:

```text
Internet unavailable
        ↓
Show cached data
        ↓
User continues reading/researching
        ↓
Connection restored
        ↓
Synchronize
```

Nocturne is **offline-aware**, not completely offline.

---

## 2.3 Server as Source of Record

The backend remains authoritative for server-managed data.

The Android local database is responsible for:

- caching
- offline access
- local UI state
- pending synchronization state

It must not silently overwrite authoritative server data.

---

## 2.4 Modular Monolith First

The backend starts as a modular monolith.

```text
Go Application
├── HTTP
├── Application
├── Domain
├── Repository
└── Database
```

Do not split the backend into microservices unless a concrete scaling or ownership problem justifies it.

---

## 2.5 Database-Centric Domain Model

PostgreSQL is the primary relational data store.

The initial data model should remain relational.

Do not introduce:

- graph databases
- document databases
- vector databases
- search clusters

until there is a demonstrated requirement.

Relationships between OSINT objects can be represented effectively using PostgreSQL relations.

---

## 2.6 Provenance First

Every source-derived object should retain its provenance.

The architecture should make it possible to answer:

> Where did this information come from?

and:

> When was this information retrieved?

Source information must not be discarded during normalization.

---

# 3. Repository Structure

The repository should use a monorepo structure.

```text
nocturne/
│
├── android/
│   ├── app/
│   ├── core/
│   ├── data/
│   ├── domain/
│   └── feature/
│
├── backend/
│   ├── cmd/
│   │   └── server/
│   │
│   ├── internal/
│   │   ├── config/
│   │   ├── domain/
│   │   ├── application/
│   │   ├── repository/
│   │   ├── http/
│   │   └── database/
│   │
│   ├── migrations/
│   ├── Dockerfile
│   ├── go.mod
│   └── go.sum
│
├── docs/
│   ├── PRD.md
│   ├── Design-System.md
│   └── architecture.md
│
├── docker-compose.yml
├── README.md
└── .gitignore
```

The exact package structure may evolve as implementation progresses.

Avoid creating directories without a concrete purpose.

---

# 4. Android Architecture

The Android client follows a layered architecture inspired by modern Android application architecture.

```text
┌─────────────────────────────┐
│        Presentation         │
│                             │
│ Compose UI                  │
│ ViewModels                  │
└──────────────┬──────────────┘
               │
┌──────────────▼──────────────┐
│           Domain            │
│                             │
│ Use Cases                   │
│ Domain Models               │
└──────────────┬──────────────┘
               │
┌──────────────▼──────────────┐
│            Data             │
│                             │
│ Repository                 │
│ Room                        │
│ Ktor Client                 │
└─────────────────────────────┘
```

Not every feature requires a dedicated use-case class.

Avoid unnecessary abstraction.

---

# 5. Android Presentation Layer

Technology:

- Kotlin
- Jetpack Compose
- ViewModel
- Kotlin Coroutines
- StateFlow where appropriate

Responsibilities:

- render UI
- handle user interaction
- observe state
- expose loading/error/empty/offline states
- trigger domain operations

The UI should not directly:

- call HTTP APIs
- access Room
- execute SQL
- contain business logic

---

# 6. Android Domain Layer

The domain layer contains application concepts independent of implementation details.

Examples:

```text
Article
Source
Entity
Event
Location
Investigation
Evidence
```

Potential operations:

```text
GetArticles
GetArticle
Search
GetEntity
GetLocation
CreateInvestigation
AddToInvestigation
SaveItem
```

Do not create a use case for every trivial getter unless it improves clarity.

---

# 7. Android Data Layer

The data layer coordinates local and remote data.

```text
Repository
    │
    ├── Local Data Source
    │      └── Room
    │
    └── Remote Data Source
           └── Ktor
```

Example:

```text
ArticleRepository
       │
       ├── ArticleDao
       │
       └── ArticleApi
```

The UI should depend on the repository abstraction rather than directly depending on API clients or DAOs.

---

# 8. Local Database

Use:

**Room + SQLite**

for Android local persistence.

Local storage should contain data required for:

- recently viewed content
- saved items
- investigations
- entities
- events
- locations
- offline reading
- synchronization state

---

# 9. Local Database Strategy

The Android database should be treated as a **local read model/cache**, not an independent backend.

Example:

```text
Backend
   │
   │ API
   ▼
Repository
   │
   ▼
Room
   │
   ▼
Compose UI
```

For normal reads:

```text
UI
 ↓
Repository
 ↓
Room
 ↓
UI
```

Remote synchronization:

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

This avoids making every screen dependent on network availability.

---

# 10. Synchronization Model

Synchronization should be incremental.

Do not download the entire dataset.

Conceptually:

```text
Last Sync
    │
    ▼
Request changes since timestamp/version
    │
    ▼
Receive updated records
    │
    ▼
Update Room
    │
    ▼
UI reflects new state
```

A concrete sync protocol will be designed when synchronization is implemented.

Do not prematurely implement complex conflict resolution.

---

# 11. Backend Architecture

The backend is a modular monolith.

```text
HTTP Layer
     │
     ▼
Application Layer
     │
     ▼
Domain Layer
     │
     ▼
Repository Layer
     │
     ▼
PostgreSQL
```

Responsibilities are separated by concern, not by microservice.

---

# 12. Backend Layers

## HTTP Layer

Responsible for:

- routing
- request parsing
- authentication middleware later
- validation
- response serialization
- HTTP status codes

Handlers should remain thin.

Bad:

```text
HTTP Handler
 ├── SQL query
 ├── business logic
 ├── data transformation
 └── response
```

Preferred:

```text
HTTP Handler
      ↓
Application Service
      ↓
Repository
      ↓
Database
```

---

# 13. Application Layer

The application layer coordinates operations.

Examples:

```text
CreateInvestigation
GetArticle
SearchArticles
AddEntityToInvestigation
GetInvestigationMap
```

Application services should coordinate domain behavior without becoming massive "god services".

---

# 14. Domain Layer

The domain layer defines core concepts.

Example:

```text
Article
Source
Entity
Event
Location
Investigation
```

Domain models should not depend directly on:

- HTTP
- SQL drivers
- JSON transport details

where practical.

---

# 15. Repository Layer

Repositories handle persistence.

Example:

```text
ArticleRepository
SourceRepository
EntityRepository
EventRepository
LocationRepository
InvestigationRepository
```

Repositories should provide operations meaningful to the domain.

Avoid generic repositories such as:

```text
Repository[T]
```

unless there is a compelling reason.

Explicit code is preferred over excessive generic abstraction.

---

# 16. Database Architecture

Primary database:

**PostgreSQL**

Spatial extension:

**PostGIS**

Core schema:

```text
sources
articles
entities
locations
events
investigations

article_entities
event_entities
investigation_sources
investigation_entities
investigation_events
```

Future tables may include:

```text
evidence
notes
saved_items
article_locations
entity_relationships
sync_records
```

These should be added only when their corresponding features are implemented.

---

# 17. Geospatial Architecture

PostGIS is responsible for geographic operations.

Locations should use:

```text
geometry(Point, 4326)
```

where exact point coordinates are available.

Example:

```text
Location
├── name
├── country
├── latitude
├── longitude
├── geometry
└── precision
```

The geometry and coordinates must remain consistent.

---

# 18. Geographic Precision

The architecture must represent uncertainty.

A location may be:

```text
Country
Region
City
Specific Location
Approximate
Unknown
```

The backend must not fabricate coordinates when only approximate geographic information exists.

If a source says:

> "Taiwan"

the system must not automatically place the event at:

> Taipei, Taiwan

unless the source supports that inference.

---

# 19. Spatial Queries

PostGIS should eventually support:

```text
Events near location
Locations within investigation bounds
Events inside geographic regions
Distance between locations
Map viewport queries
```

Map requests should be bounded by the current viewport when practical.

Do not send the entire world dataset to the mobile client.

---

# 20. Map Data Flow

The Global Map should request only relevant data.

Example:

```text
User moves map
      ↓
Viewport changes
      ↓
Android requests bounding box
      ↓
Go API
      ↓
PostGIS spatial query
      ↓
Return relevant locations/events
      ↓
Android renders markers
```

This is important for mobile performance.

---

# 21. API Architecture

Use REST over HTTPS.

Base path:

```text
/api
```

Potential resources:

```text
/api/articles
/api/sources
/api/entities
/api/events
/api/locations
/api/investigations
```

Example:

```text
GET /api/articles
GET /api/articles/{id}

GET /api/entities/{id}

GET /api/locations/{id}

GET /api/investigations
POST /api/investigations
GET /api/investigations/{id}
```

Endpoints should be introduced incrementally with each feature.

---

# 22. API Response Design

Responses should be optimized for mobile.

Avoid unnecessarily nested objects.

Example:

```json
{
  "id": "uuid",
  "title": "Example article",
  "source": {
    "id": "uuid",
    "name": "Example Source"
  },
  "published_at": "2026-09-20T10:00:00Z"
}
```

Large related datasets should use dedicated endpoints or pagination rather than being embedded indefinitely.

---

# 23. Pagination

Collection endpoints should support pagination.

Preferred approach:

```text
GET /api/articles?limit=20&cursor=...
```

Cursor-based pagination should be preferred for large or frequently changing feeds.

Do not return thousands of records to a mobile device.

---

# 24. Search Architecture

Initial search can use PostgreSQL capabilities.

Potentially:

```text
PostgreSQL
├── Full-text search
└── Indexed fields
```

Do not introduce Elasticsearch/OpenSearch at MVP stage.

If search requirements later exceed PostgreSQL capabilities, reassess based on actual measurements.

---

# 25. Caching

Caching should be introduced only where measurement shows value.

Future architecture:

```text
Client
  ↓
Go API
  ↓
Redis
  ↓
PostgreSQL
```

Potential cache targets:

- popular articles
- source metadata
- frequently accessed entities
- map queries
- search results

Do not cache everything.

---

# 26. Data Ingestion Architecture

Data ingestion is a backend subsystem.

Future structure:

```text
Public Source
     │
     ▼
Fetcher
     │
     ▼
Normalizer
     │
     ▼
Deduplicator
     │
     ▼
Enrichment
     │
     ▼
PostgreSQL
```

Potential sources:

- RSS
- public APIs
- government sources
- public databases

Each source adapter should remain isolated.

---

# 27. Source Adapter Concept

A future source adapter may conceptually implement:

```text
Fetch
Parse
Normalize
Validate
```

The adapter should convert external source formats into Nocturne's internal domain representation.

Example:

```text
Reuters RSS
      ↓
ReutersAdapter
      ↓
Normalized Article
      ↓
ArticleRepository
```

External source-specific details should not leak throughout the application.

---

# 28. Deduplication

Articles may appear across multiple sources.

Deduplication should eventually consider:

- canonical URL
- source
- title similarity
- publication timestamp
- content fingerprints

Do not build sophisticated fuzzy matching until real duplicate data exists.

---

# 29. Provenance Model

Every imported object should preserve source information.

Example:

```text
Article
├── source_id
├── url
├── retrieved_at
├── published_at
└── content metadata
```

The system should distinguish:

```text
Published At
```

from:

```text
Retrieved At
```

These are not interchangeable.

---

# 30. Evidence Architecture

Evidence should connect:

```text
Evidence
├── Source
├── Article
├── Entity
├── Event
└── Investigation
```

Evidence is not the same thing as an AI-generated interpretation.

Source-derived information must remain traceable to its origin.

---

# 31. Investigation Architecture

An investigation is a user-managed collection.

```text
Investigation
    │
    ├── Sources
    ├── Articles
    ├── Entities
    ├── Events
    ├── Locations
    └── Notes
```

The backend should not automatically infer a complete investigation.

User decisions determine what belongs in the investigation.

---

# 32. Entity Relationships

Relationships may eventually be represented explicitly.

Example:

```text
Entity A
    │
    │ relationship
    ▼
Entity B
```

Possible relationship metadata:

```text
type
source
confidence
created_at
```

However, relationship inference should not be implemented until there is a clear requirement.

Do not create a graph database for this purpose initially.

---

# 33. Security Architecture

Production communication:

```text
Android
   │
 HTTPS
   ▼
Go API
```

Security responsibilities:

- TLS
- authentication
- authorization
- input validation
- rate limiting
- safe SQL queries
- secret management
- secure token storage

Authentication is intentionally deferred from the initial backend foundation.

---

# 34. Authentication

When authentication is introduced:

```text
Android
   │
 Login
   ▼
Go API
   │
   ▼
Authentication
   │
   ▼
Token
```

The Android client should store credentials/tokens using secure Android mechanisms.

Do not store sensitive authentication data in plain SharedPreferences.

Exact authentication mechanism should be selected when the feature is implemented.

---

# 35. Error Handling

Backend errors should use consistent JSON.

Example:

```json
{
  "error": {
    "code": "article_not_found",
    "message": "Article not found"
  }
}
```

Internal errors should not expose:

- SQL errors
- stack traces
- database credentials
- infrastructure details

Detailed information belongs in server logs.

---

# 36. Observability

Initial backend observability:

- structured logs
- request ID
- HTTP status
- request duration
- database errors

Future:

- metrics
- tracing
- error monitoring

Do not introduce a complete observability platform before it is needed.

---

# 37. Logging

Every request should ideally contain:

```text
request_id
method
path
status
duration
```

Example:

```text
INFO
request_id=abc123
method=GET
path=/api/articles
status=200
duration=42ms
```

Never log:

- passwords
- authentication tokens
- API keys
- private user data

---

# 38. Configuration

Backend configuration should come from environment variables.

Examples:

```text
APP_ENV
PORT
DATABASE_URL
LOG_LEVEL
```

Secrets must never be hard-coded.

Provide:

```text
.env.example
```

Do not commit:

```text
.env
```

---

# 39. Docker Architecture

Docker is used for backend infrastructure and development consistency.

It is NOT part of the Android runtime.

Development:

```text
Docker Compose
│
├── PostgreSQL + PostGIS
└── Go Backend (optional)
```

Android runs normally through:

```text
Android Studio
Gradle
ADB
```

The Android application does not run inside Docker.

---

# 40. Deployment Architecture

Initial production deployment can remain simple:

```text
Internet
   │
 HTTPS
   ▼
Reverse Proxy / Load Balancer
   │
   ▼
Go Application
   │
   ▼
PostgreSQL + PostGIS
```

A managed PostgreSQL service is acceptable.

Do not require Kubernetes.

---

# 41. Background Workers

Background workers will eventually handle:

- RSS ingestion
- source synchronization
- normalization
- deduplication
- enrichment

Potential architecture:

```text
Go API
   │
   ▼
Job Queue
   │
   ▼
Go Worker
   │
   ▼
PostgreSQL
```

Redis may later provide queue infrastructure.

This is deferred until ingestion is implemented.

---

# 42. Mobile Network Strategy

The Android client should treat network calls as unreliable.

Every remote operation should account for:

```text
Loading
Success
Empty
Offline
Timeout
Server Error
Unauthorized
Rate Limited
```

The UI should map these states into human-readable feedback.

---

# 43. Image Delivery

Images should not be unnecessarily downloaded at full resolution.

Backend responses should provide appropriate image URLs/metadata where available.

Android should:

- lazy-load images
- cache images
- constrain dimensions
- display placeholders
- handle unavailable images

Image processing/CDN infrastructure can be added later.

---

# 44. Performance Strategy

Performance priorities:

## Android

- lazy lists
- stable Compose state
- Room caching
- efficient image loading
- limited recomposition
- bounded map markers

## Backend

- database indexes
- pagination
- bounded queries
- connection pooling
- compact responses

## Database

- indexes on common lookup fields
- spatial indexes
- query analysis when needed

Performance should be measured before introducing infrastructure complexity.

---

# 45. Database Indexing

Initial indexes should target real query patterns.

Likely candidates:

```text
articles.source_id
articles.published_at
entities.name
events.occurred_at
locations.geometry
```

Join-table foreign keys should also be indexed appropriately.

Do not blindly index every column.

---

# 46. API Versioning

The initial API may use:

```text
/api/...
```

If breaking changes become necessary, introduce:

```text
/api/v2/...
```

Do not version every endpoint prematurely.

---

# 47. Testing Architecture

## Android

Test:

- ViewModels
- repositories
- local persistence
- critical UI behavior

## Backend

Test:

- domain logic
- application services
- HTTP handlers
- repositories
- database integration

## Integration

At minimum:

```text
API
 ↓
Repository
 ↓
PostgreSQL/PostGIS
```

should be testable in an isolated environment.

---

# 48. CI/CD

GitHub Actions should eventually run:

```text
Android
├── lint
├── unit tests
└── build

Backend
├── gofmt check
├── go vet
├── tests
└── build
```

Integration tests may run against a temporary PostgreSQL/PostGIS environment.

---

# 49. Dependency Management

Dependencies should be kept minimal.

Before adding a dependency, ask:

1. Is the functionality actually needed?
2. Is the standard library sufficient?
3. Is the dependency maintained?
4. Does it introduce significant transitive dependencies?
5. Does it solve a real problem?

Avoid dependency accumulation.

---

# 50. Architecture Decision Records

Significant architectural decisions should be documented.

Examples:

```text
docs/adr/
├── 001-android-native.md
├── 002-go-backend.md
├── 003-postgresql-postgis.md
└── 004-modular-monolith.md
```

An ADR should explain:

- context
- decision
- alternatives
- consequences

---

# 51. Architecture Evolution

Nocturne should evolve in stages.

## Stage 1 — Foundation

```text
Kotlin
Go
PostgreSQL
PostGIS
Room
```

## Stage 2 — Product

```text
REST API
CRUD
Search
Map
Investigations
```

## Stage 3 — Offline

```text
Room
Sync
Caching
Offline-aware UX
```

## Stage 4 — Ingestion

```text
RSS
Public APIs
Go Workers
Deduplication
```

## Stage 5 — Scale

Only if required:

```text
Redis
Job Queue
Search Engine
Object Storage
Observability
```

---

# 52. Explicit Non-Goals

Do not introduce these into the initial architecture:

- microservices
- Kubernetes
- Kafka
- Elasticsearch
- graph database
- vector database
- LLM infrastructure
- event sourcing
- CQRS
- service mesh
- multi-region deployment

These technologies may be useful in other systems but are not justified by the initial Nocturne requirements.

---

# 53. Architectural Trade-Offs

## Kotlin vs React Native

Kotlin was selected because Nocturne is Android-first and requires:

- strong offline support
- local persistence
- Android lifecycle integration
- battery-conscious behavior
- native map integration
- mobile performance

Cross-platform support can be reconsidered later.

---

## Go vs Python

Go was selected because the initial backend emphasizes:

- HTTP APIs
- concurrent ingestion
- networking
- efficient resource usage
- background processing

Python may still be introduced later as an isolated enrichment/ML service if the product develops genuine ML requirements.

Python should not be added simply because "OSINT uses AI."

---

## PostgreSQL vs Dedicated Search Engine

PostgreSQL is sufficient for the initial dataset.

A dedicated search engine should only be introduced after PostgreSQL search capabilities become an actual bottleneck.

---

## PostgreSQL/PostGIS vs Graph Database

Nocturne's initial relationships are relational.

PostgreSQL can represent:

```text
Article
Entity
Event
Location
Investigation
```

through standard relationships.

A graph database becomes justified only if graph traversal becomes a demonstrated core workload.

---

# 54. Key Architectural Rule

The architecture should optimize for:

> **Simplicity first, capability second, scale third.**

Do not build infrastructure for hypothetical problems.

Every architectural addition should have a concrete product or engineering reason.

---

# 55. Final Architecture

The intended initial architecture is:

```text
                       NOCTURNE
                           │
              ┌────────────┴────────────┐
              │                         │
              ▼                         ▼
      Android Application          Public Sources
              │                         │
      Kotlin + Compose                 │
              │                         │
        ┌─────┴─────┐                   │
        │           │                   │
      Room       Ktor Client             │
        │           │                   │
        │           │ HTTPS              │
        │           ▼                   │
        │      ┌─────────────┐          │
        │      │   Go API    │◄─────────┘
        │      └──────┬──────┘
        │             │
        │       Application
        │          Logic
        │             │
        │        Repository
        │             │
        │             ▼
        │   ┌──────────────────┐
        │   │ PostgreSQL       │
        │   │ + PostGIS        │
        │   └──────────────────┘
        │
        └──── Local-first UI
```

Future:

```text
                         Go Backend
                              │
                ┌─────────────┼─────────────┐
                │             │             │
                ▼             ▼             ▼
           PostgreSQL       Redis       Go Workers
           + PostGIS                     │
                                         ▼
                                  Public Sources
```

This architecture should remain the baseline until real requirements justify changing it.

---

# 56. Final Principle

Nocturne is not designed to demonstrate how many technologies can be placed into one repository.

It is designed to demonstrate that a developer can build a **real, constrained, mobile-first research system** with:

- thoughtful data modeling
- reliable backend engineering
- geospatial capability
- offline-aware mobile architecture
- source provenance
- incremental synchronization
- performance-conscious design
- maintainable code

The architecture should remain boring where boring is good.

The interesting part should be what the system enables.
