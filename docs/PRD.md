**Version:** 1.0
**Status:** Product Definition
**Platform:** Android
**Primary Audience:** OSINT researchers, analysts, journalists, students, researchers, technically curious users
**Product Type:** Mobile OSINT Research & Investigation Tool

---

# 1. Product Overview

## 1.1 Product Name

**Nocturne**

## 1.2 Product Description

Nocturne is a lightweight, mobile-first OSINT research application designed to help users discover, inspect, organize, and connect publicly available information.

The application combines:

- news and public-source discovery
- source provenance
- entity exploration
- event timelines
- geographic context
- relationship mapping
- investigation / dossier organization

The core principle is:

> **Search → Discover → Inspect → Connect → Preserve → Investigate**

Nocturne is not intended to replace professional intelligence platforms.

It is a research-oriented tool focused on making public-source investigation practical on a mobile device.

---

# 2. Problem Statement

Public information is abundant, but conducting structured research from a mobile device is difficult.

Users often need to:

- search across multiple sources
- determine where an event occurred
- identify people, organizations, companies, or places mentioned
- compare information from different sources
- remember where information came from
- organize findings into a coherent investigation
- understand relationships between entities
- revisit previous research

Typical mobile news applications optimize for consumption rather than investigation.

Nocturne addresses this gap by treating public information as interconnected research objects rather than isolated articles.

---

# 3. Product Vision

Create a lightweight mobile research environment where users can move naturally from:

> **Information → Source → Entity → Event → Location → Relationship → Investigation**

without requiring a desktop workstation.

The application should feel:

- professional
- analytical
- calm
- lightweight
- trustworthy
- technically capable

It should NOT feel like:

- a fictional military command center
- a tactical battlefield interface
- a cyberpunk HUD
- an intelligence-agency simulator

---

# 4. Target Users

## 4.1 Primary Users

### OSINT / Open-source researchers

Users who investigate publicly available information and need to organize sources, entities, events, and locations.

### Journalists / researchers

Users who need to cross-reference public information and preserve source context.

### Students

Students learning research, information verification, geopolitics, technology, business, or related fields.

### Technically curious users

Users interested in understanding complex events through public information.

---

# 5. Non-Goals

Nocturne will NOT initially attempt to provide:

- classified information
- private information acquisition
- credential harvesting
- unauthorized access
- surveillance
- automated targeting
- operational military intelligence
- real-time tactical intelligence
- weapons-related operational guidance
- unrestricted web scraping
- a complete replacement for professional intelligence platforms

The product focuses on **lawful, publicly available information**.

---

# 6. Core Product Concepts

Nocturne is built around several primary objects.

## 6.1 Source

A publisher or public information origin.

Examples:

- Reuters
- government websites
- public organizations
- public databases
- academic institutions

A Source answers:

> "Where did this information come from?"

---

## 6.2 Article

A specific piece of published information.

Attributes include:

- title
- URL
- summary
- publication date
- source
- associated entities
- associated location
- associated events

---

## 6.3 Entity

A person, organization, company, government body, place, or other identifiable object.

Examples:

- NVIDIA
- TSMC
- ASML
- a government agency
- a public official
- Hsinchu

---

## 6.4 Event

Something that happened at a particular point in time.

An event may have:

- title
- description
- time
- location
- related entities
- supporting sources

---

## 6.5 Location

A geographic context associated with an article, entity, or event.

Locations can have different precision levels:

- Country
- Region
- City
- Specific Location

Nocturne must never imply greater geographic precision than the available source supports.

---

## 6.6 Evidence

A piece of information collected from a public source.

Evidence maintains context about:

- source
- article
- timestamp
- associated entity
- associated event
- location

---

## 6.7 Investigation / Dossier

A structured research workspace containing selected:

- sources
- articles
- entities
- events
- locations
- notes
- evidence

An investigation represents the user's research around a particular subject.

---

# 7. Core User Journey

The primary workflow is:

```text
Discover
   ↓
Open Source
   ↓
Inspect Article
   ↓
Identify Entity / Event / Location
   ↓
Explore Related Information
   ↓
Save Relevant Evidence
   ↓
Add to Investigation
   ↓
Review Timeline / Map / Relationships
```

---

# 8. Functional Requirements

# 8.1 Discovery

Users must be able to browse public-source content.

Each article card should provide:

- source name
- title
- publication time
- thumbnail where available
- location where available
- relevant entities
- save action

The interface should remain compact and mobile-friendly.

---

# 8.2 Search

Users must be able to search across relevant Nocturne data.

Initial search targets:

- articles
- entities
- sources
- locations
- investigations

Search should return relevant results without requiring users to understand internal database terminology.

---

# 8.3 Article Detail

Users must be able to inspect an article.

The article page should display:

- article title
- source
- publication time
- URL
- summary
- article image where available
- location
- entities
- related events
- provenance information

Actions:

- Save
- Add to Investigation
- Open Source
- View Location

---

# 8.4 Source Provenance

Every article must retain information about its source.

Users should be able to determine:

> "Where did this information come from?"

Source status may include:

- Official Source
- Primary Source
- Secondary Source
- Reported
- Derived
- Unverified

Nocturne must not claim verification that it has not actually performed.

---

# 8.5 Entity Exploration

Users must be able to open an entity.

Entity pages should provide:

- name
- type
- description
- related articles
- related events
- relationships
- locations
- associated investigations

Example:

```text
TSMC
Company · Taiwan

Recent Sources
...

Relationships
NVIDIA
ASML

Locations
Hsinchu, Taiwan
```

---

# 8.6 Event Timeline

Users must be able to view events chronologically.

Each event may include:

- title
- date/time
- location
- entities
- sources

Example:

```text
18 Sep 2026
Semiconductor investment announced
Hsinchu, Taiwan
Reuters

16 Sep 2026
Equipment shipment reported
Veldhoven, Netherlands
Financial Times
```

---

# 8.7 Global Map

Nocturne must provide a geographic research view.

The Global Map should display relevant:

- events
- sources
- entities
- investigation locations

Users should be able to:

- pan
- zoom
- select locations
- inspect clusters
- open related events
- open related sources
- open related investigations

The map must be a research visualization rather than a tactical interface.

---

# 8.8 Location Detail

A location can be opened from:

- an article
- an event
- an entity
- the Global Map

Location details may include:

- name
- country
- coordinates when available
- geographic precision
- related sources
- related entities
- related events
- related investigations

Example:

```text
Hsinchu
Taiwan

Location precision: City

12 Sources
4 Entities
3 Events
```

---

# 8.9 Investigation / Dossier

Users must be able to create investigations.

An investigation contains:

- name
- description
- sources
- articles
- entities
- events
- locations
- notes

Primary investigation views:

- Overview
- Sources
- Entities
- Timeline
- Map
- Notes

---

# 8.10 Investigation Map

Every investigation may have a geographic view.

The map should display only locations relevant to the investigation.

Example:

```text
NVIDIA Semiconductor Supply Chain

4 Locations

Hsinchu
Veldhoven
Santa Clara
Taipei
```

Users can open the underlying entities/events/sources from map locations.

---

# 8.11 Saved Items

Users must be able to save:

- articles
- sources
- entities
- investigations

Saved items act as a lightweight research inbox.

Saved items are intentionally separate from Investigations.

---

# 8.12 Notes

Users should eventually be able to attach notes to investigations.

Notes may reference:

- articles
- entities
- events
- locations

Notes are user-generated research context and must remain distinguishable from source-derived information.

---

# 9. Offline-Aware Requirements

Nocturne should be **offline-aware**, not completely offline.

The application should cache relevant previously accessed information locally.

Potential cached data:

- recently viewed articles
- saved items
- investigations
- entities
- locations
- timelines
- notes

When offline:

Users should be able to:

- read cached articles
- inspect cached entities
- inspect saved items
- open cached investigations
- review timelines
- review previously accessed locations
- edit local notes where supported

Users should NOT expect:

- new web searches
- new article retrieval
- live source updates
- new geocoding
- real-time ingestion

When connectivity returns, the application should synchronize appropriate data.

---

# 10. Mobile Requirements

Nocturne is Android-first.

The application must prioritize:

- one-handed interaction
- responsive layouts
- touch-friendly controls
- low memory usage
- efficient image loading
- lazy loading
- cached content
- clear loading states
- clear offline states
- graceful network failure

Avoid excessively dense desktop-style interfaces.

---

# 11. Global Map Requirements

The map must:

- support world-level exploration
- support regional zoom
- display relevant locations
- cluster nearby points
- open location details
- connect locations to investigations
- display geographic precision
- avoid fabricated coordinates

Map states:

- Loading
- Loaded
- Empty
- No location available
- Offline
- Error

---

# 12. Data Integrity Requirements

Nocturne must distinguish between:

### Source-derived information

Information explicitly provided by a source.

### Derived information

Information calculated or inferred from existing structured data.

### User-created information

Notes, investigation organization, tags, and other user input.

These categories should not be visually conflated.

---

# 13. Backend Architecture

The initial architecture is a modular monolith.

```text
Android App
    │
    │ HTTPS
    ▼
Go REST API
    │
    ├── PostgreSQL
    │      └── PostGIS
    │
    └── Redis (future)
```

Future ingestion workers may be added without immediately converting the application into microservices.

---

# 14. Technology Stack

## Android

- Kotlin
- Jetpack Compose
- Room / SQLite
- Ktor Client
- MapLibre

## Backend

- Go
- REST API

## Database

- PostgreSQL
- PostGIS

## Cache / Jobs

Future:

- Redis
- Go workers

## Infrastructure

- Docker for backend development/deployment
- GitHub Actions for CI/CD

---

# 15. Initial Backend Domain Model

Core tables:

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

Additional tables should be introduced only when required by a concrete feature.

---

# 16. API Principles

The API should:

- use JSON
- use HTTPS in production
- return consistent errors
- validate inputs
- use pagination
- avoid returning unnecessary data
- support mobile-friendly payload sizes

Potential future endpoints:

```text
GET    /api/articles
GET    /api/articles/{id}

GET    /api/sources
GET    /api/sources/{id}

GET    /api/entities
GET    /api/entities/{id}

GET    /api/events
GET    /api/events/{id}

GET    /api/locations
GET    /api/locations/{id}

GET    /api/investigations
POST   /api/investigations
GET    /api/investigations/{id}
```

Exact API design should be established during implementation rather than prematurely implementing every endpoint.

---

# 17. Data Ingestion

Data ingestion is a future subsystem.

Potential sources:

- RSS feeds
- public APIs
- public government sources
- public databases

The ingestion pipeline should eventually perform:

```text
Source
 ↓
Fetch
 ↓
Normalize
 ↓
Deduplicate
 ↓
Extract metadata
 ↓
Extract entities
 ↓
Resolve locations
 ↓
Store
```

Nocturne should respect:

- API terms
- robots.txt where applicable
- source licensing
- rate limits
- attribution requirements

---

# 18. Geospatial Data

Locations should use:

**PostGIS**

with WGS84 / SRID 4326.

The system should support:

- point locations
- geographic bounding queries
- distance queries
- investigation-specific geographic queries

Future examples:

> Events within 50 km of a location.

> All investigation locations in Taiwan.

> Sources associated with a geographic region.

---

# 19. Performance Requirements

Nocturne should be designed for mobile constraints.

Target principles:

- minimize payload size
- paginate lists
- lazy-load images
- cache frequently accessed data
- avoid unnecessary API requests
- avoid downloading full datasets
- use database indexes
- use spatial indexes for geospatial queries

No hard performance number should be claimed until measured.

---

# 20. Security Requirements

Initial requirements:

- HTTPS in production
- secure authentication when authentication is introduced
- hashed passwords if passwords are supported
- no secrets in source control
- environment-based configuration
- input validation
- SQL parameterization
- rate limiting
- safe error responses
- secure mobile token storage

Security should be implemented incrementally.

---

# 21. Accessibility

The Android application should support:

- readable typography
- sufficient contrast
- accessible touch targets
- semantic labels
- screen-reader-friendly controls
- reduced reliance on color alone

Map markers should have accessible alternatives through lists or location cards.

---

# 22. MVP Scope

The first functional MVP should focus on:

### Core

- Android application
- Go backend
- PostgreSQL/PostGIS
- Source model
- Article model
- Entity model
- Location model
- Event model
- Investigation model

### User Features

- Browse articles
- Search
- Article detail
- Entity detail
- Location detail
- Global Map
- Timeline
- Create Investigation
- Add sources/entities/events to Investigation
- Saved items

### Infrastructure

- REST API
- database migrations
- local caching
- Docker development environment
- automated tests
- CI

---

# 23. Post-MVP

After the MVP is stable:

### Phase 2

- RSS ingestion
- source synchronization
- deduplication
- background workers
- Redis caching
- better search

### Phase 3

- entity extraction
- location extraction
- automated event extraction
- relationship suggestions
- semantic search

### Phase 4

- richer offline synchronization
- investigation export
- PDF / Markdown reports
- advanced geospatial queries
- collaboration

AI should remain an enrichment layer rather than the central product.

---

# 24. Explicitly Deferred Features

Do not implement during the initial MVP:

- LLM assistant
- automated intelligence conclusions
- predictive analysis
- threat scoring
- sentiment scoring
- facial recognition
- surveillance
- social-media account tracking
- dark-web monitoring
- credential discovery
- automated target profiling
- tactical mapping
- real-time military tracking

These are outside the initial product scope.

---

# 25. Success Criteria

The MVP is successful when a user can:

1. Open Nocturne.
2. Discover an article.
3. Inspect its source.
4. See associated entities.
5. See where the event occurred.
6. Open that location on the map.
7. Explore related events.
8. Save useful information.
9. Create an investigation.
10. Add sources/entities/events to the investigation.
11. View the investigation timeline.
12. View the investigation geographically.
13. Reopen the investigation later, including previously cached information when offline.

---

# 26. Engineering Success Criteria

The project should demonstrate:

- idiomatic Kotlin
- modern Android architecture
- clean Compose UI architecture
- local persistence
- REST API design
- idiomatic Go
- relational database modeling
- PostGIS usage
- geospatial queries
- caching
- asynchronous processing
- testing
- CI/CD
- Dockerized backend environment
- mobile performance awareness

The architecture should remain understandable to another developer.

---

# 27. Development Philosophy

Nocturne should be built incrementally.

Each development stage must produce a working system.

Recommended order:

```text
STEP 1
Backend Foundation
        ↓
STEP 2
Database + CRUD API
        ↓
STEP 3
Android Foundation
        ↓
STEP 4
Article / Source Flow
        ↓
STEP 5
Entity + Event Model
        ↓
STEP 6
Global Map
        ↓
STEP 7
Investigation / Dossier
        ↓
STEP 8
Local Cache / Offline-Aware
        ↓
STEP 9
Source Ingestion
        ↓
STEP 10
Search + Optimization
        ↓
STEP 11
Advanced Enrichment
```

Do not skip directly to the final architecture.

---

# 28. Product Principle

The most important principle of Nocturne is:

> **Less intelligence theater. More actual research utility.**

Every feature should answer a real research question.

If a feature only exists to make the interface look sophisticated but does not improve research, it should not be prioritized.

Nocturne should ultimately feel like:

> **A serious research tool that happens to fit in your pocket.**
