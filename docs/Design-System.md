**Version:** 1.0
**Status:** Foundation
**Platform:** Android
**UI Framework:** Jetpack Compose
**Product:** Nocturne — Mobile OSINT Research Tool

---

# 1. Design Philosophy

Nocturne is a **mobile-first OSINT research application**.

The interface must communicate:

- clarity
- credibility
- calmness
- information density
- traceability
- geographic awareness
- research utility

The UI should feel like a **professional research instrument**, not a fictional military command center.

## Core Principle

> **Information first. Interface second.**

Visual decoration must never compete with research data.

---

# 2. Design Personality

Nocturne should feel:

- analytical
- restrained
- precise
- modern
- slightly atmospheric
- trustworthy
- quiet
- technical

It should NOT feel:

- cyberpunk
- militaristic
- dystopian
- overly futuristic
- hacker-themed
- arcade-like
- excessively corporate
- like a Bloomberg terminal clone

---

# 3. Visual Direction

The visual language should combine:

**Dark editorial interface + geospatial research tool + modern Android application.**

The interface should have enough contrast and hierarchy to support long research sessions.

Avoid excessive:

- gradients
- glow effects
- neon colors
- glassmorphism
- decorative borders
- animated backgrounds
- excessive shadows

---

# 4. Color System

Use a dark-first visual system.

## 4.1 Base Colors

```text
Background / Deep
#0B0D10

Background / Surface
#11151A

Surface Elevated
#171C22

Surface Interactive
#1D232B

Border
#29313A

Border Strong
#37414C
```

These colors should establish depth without relying on strong shadows.

---

# 5. Text Colors

```text
Text Primary
#F1F3F5

Text Secondary
#B3BAC3

Text Tertiary
#7D8792

Text Disabled
#555E68
```

Primary text should be reserved for important information.

Do not use pure white everywhere.

---

# 6. Accent Color

Primary accent:

```text
Nocturne Accent
#7C9CFF
```

Use the accent for:

- primary actions
- selected navigation
- interactive links
- map selection
- active filters
- focused controls
- important highlights

Do NOT use the accent as a background for large areas.

Accent should remain relatively sparse.

---

# 7. Semantic Colors

Semantic colors must communicate state rather than decoration.

```text
Success
#6FCF97

Warning
#E8B86D

Error
#E07878

Information
#7C9CFF
```

Use semantic colors sparingly.

Never communicate meaning through color alone.

---

# 8. Source / Provenance Colors

Source classification may use subtle semantic treatments.

```text
Primary / Official
Information blue

Secondary
Neutral gray

Reported
Muted amber

Unverified
Muted red
```

The exact visual implementation should prioritize text labels and icons.

Example:

```text
SOURCE
Reuters

STATUS
Reported
```

Do not rely only on a colored dot.

---

# 9. Typography

Use a highly readable sans-serif font.

Preferred Android font:

**Roboto**

Optional display treatment:

**Roboto / system sans-serif**

No decorative fonts.

---

# 10. Typography Scale

## Display

Used sparingly.

```text
Display Large
32sp / 38sp

Display Medium
28sp / 34sp
```

## Headlines

```text
Headline Large
24sp / 30sp

Headline Medium
20sp / 26sp

Headline Small
18sp / 24sp
```

## Body

```text
Body Large
16sp / 24sp

Body Medium
14sp / 20sp

Body Small
13sp / 18sp
```

## Labels

```text
Label Large
14sp / 18sp

Label Medium
12sp / 16sp

Label Small
11sp / 14sp
```

Avoid text smaller than 11sp for essential information.

---

# 11. Typography Hierarchy

A typical article card:

```text
SOURCE
Reuters · 2h ago

Taiwan expands semiconductor
manufacturing capacity

Hsinchu, Taiwan
3 related entities
```

Hierarchy:

1. Article title
2. Source / publication time
3. Location
4. Supporting metadata

Do not make metadata visually louder than the primary content.

---

# 12. Spacing System

Use a 4dp base unit.

```text
4dp
8dp
12dp
16dp
20dp
24dp
32dp
40dp
48dp
64dp
```

Default content padding:

**16dp**

Large section spacing:

**24–32dp**

Avoid arbitrary spacing values.

---

# 13. Corner Radius

Use restrained rounding.

```text
Small
6dp

Medium
10dp

Large
14dp

Extra Large
20dp
```

Default cards:

**10–14dp**

Buttons:

**10–14dp**

Avoid excessive pill-shaped UI.

Pills should primarily represent:

- tags
- statuses
- filters
- categories

---

# 14. Elevation

Nocturne should rely primarily on:

- color contrast
- borders
- spacing

rather than large shadows.

Suggested elevation:

```text
Default surface
0dp

Elevated card
2dp

Modal / Bottom sheet
6dp
```

Avoid floating everything.

---

# 15. Iconography

Use Material Symbols / Material Icons.

Icons should be:

- simple
- recognizable
- consistent
- functional

Preferred style:

**outlined icons**

Avoid mixing unrelated icon sets.

---

# 16. Navigation

Primary navigation should be optimized for mobile.

Suggested navigation:

```text
Discover
Map
Investigate
Saved
```

Use bottom navigation for primary destinations.

Each destination must have:

- icon
- text label
- selected state

Do not rely on icons alone.

---

# 17. Discover Screen

Purpose:

> Find information worth investigating.

Content hierarchy:

```text
Header
Search
Filters
Featured / Relevant Sources
Article Feed
```

Article feed should support:

- compact cards
- thumbnails
- source
- publication time
- location
- entity hints

The screen should prioritize content over decoration.

---

# 18. Search

Search should be accessible from Discover.

Search interaction:

```text
Search
   ↓
Query
   ↓
Results
   ├── Articles
   ├── Entities
   ├── Sources
   ├── Locations
   └── Investigations
```

Search results should clearly communicate result type.

Example:

```text
ARTICLE
TSMC expands production capacity

ENTITY
TSMC

LOCATION
Hsinchu, Taiwan
```

---

# 19. Article Card

Article cards should contain:

```text
┌──────────────────────────┐
│ SOURCE · TIME             │
│                           │
│ Article title             │
│ Article title             │
│                           │
│ [thumbnail]               │
│                           │
│ LOCATION · ENTITIES       │
└──────────────────────────┘
```

Requirements:

- clear title
- source attribution
- timestamp
- optional image
- optional location
- optional entities

Do not force images when unavailable.

---

# 20. Article Detail

Hierarchy:

```text
Source
Title
Publication Date
Hero Image
Summary
Location
Entities
Events
Related Sources
Original Source
```

The original source link must be clearly identifiable.

Nocturne should never imply that it hosts or owns third-party article content.

---

# 21. Source Card

Source cards should emphasize provenance.

Example:

```text
REUTERS

Reuters is a global news organization.

127 articles
23 entities
14 locations
```

Source information should remain factual.

Avoid visual "trust scores" unless there is a documented methodology.

---

# 22. Entity Card

Entity cards should communicate type clearly.

Example:

```text
TSMC
Company

Taiwan Semiconductor Manufacturing Company

12 articles
4 events
3 locations
```

Entity type must be textual, not only represented by color.

---

# 23. Entity Detail

Suggested structure:

```text
Entity Header
Type
Description

Related Articles

Related Events

Relationships

Locations

Investigations
```

The page should support exploration rather than present a single conclusion.

---

# 24. Event Card

Event cards:

```text
EVENT
18 Sep 2026

Semiconductor investment announced

Hsinchu, Taiwan

3 entities
2 sources
```

Events must distinguish:

- known facts
- source-derived descriptions
- uncertain dates

---

# 25. Timeline

Timeline should be vertically oriented.

Example:

```text
18 SEP 2026
│
● Semiconductor investment announced
│  Hsinchu, Taiwan
│
│
16 SEP 2026
│
● Equipment shipment reported
│  Veldhoven, Netherlands
│
│
12 SEP 2026
│
● Company statement published
```

Use a simple vertical line.

Do not over-design the timeline.

---

# 26. Global Map

The Global Map is one of Nocturne's primary interfaces.

Purpose:

> Understand where information is geographically connected.

The map should display:

- events
- locations
- investigation locations
- optionally entity locations

---

# 27. Map Visual Language

The map should use a subdued base map.

Information overlays should be more visually prominent than the basemap.

Avoid:

- tactical grid overlays
- radar effects
- animated targeting circles
- military symbology
- fake satellite intelligence styling

Markers should communicate actual data.

---

# 28. Map Markers

Marker hierarchy:

```text
Default location
●

Selected location
◉

Cluster
● 12

Event
●

Investigation location
◆
```

Do not use dozens of colors.

Use shape + label + context where possible.

---

# 29. Map Clustering

When many points are close together:

```text
● 27
```

Selecting the cluster should zoom into the region.

This is important for mobile usability.

Do not render hundreds of independent markers simultaneously.

---

# 30. Location Precision

Every location should communicate precision.

Example:

```text
Hsinchu, Taiwan

Location precision
City
```

Possible values:

```text
Country
Region
City
Specific Location
Approximate
Unknown
```

The UI must never visually imply exact coordinates when the source only identifies a city or region.

---

# 31. Location Detail

Suggested hierarchy:

```text
Location Name
Country

Map

Precision

Related Events
Related Sources
Related Entities
Investigations
```

---

# 32. Investigation / Dossier

Investigation should be the main workspace for research.

Suggested header:

```text
INVESTIGATION

Taiwan Semiconductor Supply Chain

Updated 2h ago

12 Sources
8 Entities
5 Events
4 Locations
```

Tabs:

```text
Overview
Sources
Entities
Timeline
Map
Notes
```

---

# 33. Investigation Overview

Overview should summarize the research without pretending to provide intelligence conclusions.

Display:

- source count
- entity count
- event count
- location count
- recent activity
- key sources
- timeline preview
- geographic preview

Avoid automated "threat level" or "risk score" unless a transparent methodology is implemented.

---

# 34. Evidence

Evidence must visibly retain provenance.

Example:

```text
EVIDENCE

TSMC announced additional capacity...

SOURCE
Reuters

PUBLISHED
18 Sep 2026

ORIGINAL ARTICLE
Open Source
```

User-generated notes must look different from source-derived evidence.

---

# 35. Notes

Notes should use a visually distinct but restrained surface.

Example:

```text
MY NOTE

Check whether this announcement
connects with the previous shipment event.
```

Use a label such as:

**MY NOTE**

Never visually merge user interpretation with source content.

---

# 36. Buttons

Primary button:

```text
Filled
```

Secondary button:

```text
Outlined / Tonal
```

Tertiary:

```text
Text button
```

Examples:

```text
[ Add to Investigation ]

[ View Source ]

[ Open Map ]
```

Buttons should use clear verbs.

Avoid vague labels such as:

- Continue
- Proceed
- Explore

when a more specific action is possible.

---

# 37. Chips

Use chips for:

- entity types
- source types
- filters
- geographic precision
- statuses

Examples:

```text
Company
Taiwan
City
Reported
Saved
```

Chips should remain compact.

---

# 38. Bottom Sheets

Bottom sheets are appropriate for:

- map location previews
- filter controls
- source previews
- article actions
- entity previews

A map interaction should not always navigate away from the map.

Example:

```text
Map
 ↓
Tap Location
 ↓
Bottom Sheet
 ↓
Location Preview
 ↓
Open Details
```

This preserves geographic context.

---

# 39. Loading States

Use skeleton loading for content-heavy screens.

Example:

```text
████████████████
██████████

████████████████████
████████████
```

Avoid indefinite spinners.

For short operations:

```text
CircularProgressIndicator
```

is acceptable.

---

# 40. Empty States

Empty states must explain what happened and what the user can do.

Example:

```text
No investigations yet.

Create an investigation to organize
sources, entities, events, and locations.

[ Create Investigation ]
```

Avoid decorative illustrations that consume excessive screen space.

---

# 41. Error States

Errors must be human-readable.

Example:

```text
Unable to load sources.

Check your connection and try again.

[ Retry ]
```

Do not expose:

```text
HTTP 500
ECONNRESET
SQLSTATE 08006
```

to users.

Technical details may be logged internally.

---

# 42. Offline State

When offline:

```text
OFFLINE

Showing cached information.
Some data may be outdated.
```

The user should still be able to access available cached content.

Do not present offline mode as an error when the application is functioning normally with cached data.

---

# 43. Images

Images should be treated as supporting evidence/context rather than decoration.

Preferred behavior:

- lazy loading
- caching
- compression
- fixed aspect ratios
- placeholders
- graceful fallback

If an article has no image:

Do not create a fake image.

---

# 44. Image Attribution

When required by the source/provider:

display appropriate attribution.

Nocturne must respect image licensing and API terms.

---

# 45. Motion

Motion should be subtle.

Use animation for:

- navigation transitions
- bottom sheet presentation
- map selection
- loading state transitions
- list insertion/removal

Avoid:

- constant pulsing
- glowing markers
- animated scanlines
- decorative particle effects
- excessive parallax

The interface should feel calm.

---

# 46. Accessibility

Minimum requirements:

- WCAG-aware contrast
- minimum touch target around 48dp
- screen-reader labels
- semantic Compose components
- scalable typography
- no color-only meaning
- accessible map alternatives

Every map-based feature must have a non-map alternative.

For example:

```text
Map
List of Locations
```

---

# 47. Mobile Performance

The UI must prioritize low resource consumption.

Requirements:

- lazy lists
- image caching
- image size constraints
- avoid unnecessary recomposition
- stable Compose state
- pagination
- incremental loading
- limited simultaneous map markers
- local caching

Do not render the entire dataset at once.

---

# 48. Data Density

Nocturne is information-dense, but density must remain readable.

Prefer:

```text
High information density
+
Strong hierarchy
+
Generous grouping
```

rather than:

```text
Everything everywhere
```

---

# 49. Responsive Behavior

Although Android is the primary platform, layouts should support:

- small phones
- standard phones
- large phones
- tablets where practical

Do not hard-code a single screen width.

---

# 50. Dark Mode

Dark mode is the primary theme.

If light mode is eventually introduced, it should be treated as a separate tested theme rather than simply inverting colors.

The current product priority is:

**Dark theme first.**

---

# 51. Design Tokens

Implementation should centralize design tokens.

Example:

```kotlin
object NocturneSpacing {
    val xs = 4.dp
    val sm = 8.dp
    val md = 12.dp
    val lg = 16.dp
    val xl = 24.dp
    val xxl = 32.dp
}
```

Colors, typography, shapes, and spacing should be centralized through the Compose theme.

Avoid hard-coded colors throughout individual components.

---

# 52. Component Architecture

Reusable components should be created for recurring patterns.

Examples:

```text
NocturneTopBar
NocturneBottomNavigation
ArticleCard
SourceCard
EntityCard
EventCard
LocationCard
EvidenceCard
InvestigationCard
StatusChip
SourceBadge
OfflineBanner
EmptyState
ErrorState
LoadingSkeleton
MapLocationSheet
```

Do not create components for one-off layouts unless reuse is likely.

---

# 53. Component States

Interactive components should define:

- default
- pressed
- focused
- selected
- disabled
- loading
- error

Map components should additionally define:

- offline
- empty
- location unavailable

---

# 54. Information Hierarchy Rules

When deciding what should visually dominate:

1. Current task
2. Primary information
3. Source/provenance
4. Context
5. Metadata
6. Secondary actions
7. Decorative elements

Decorative elements should always have the lowest priority.

---

# 55. Trust & Transparency

Nocturne must not visually imply certainty that does not exist.

Avoid:

- fake confidence percentages
- arbitrary threat levels
- unexplained credibility scores
- AI-generated conclusions presented as facts
- exact locations derived from vague sources

Whenever uncertainty exists, communicate it.

---

# 56. Design Anti-Patterns

Do NOT introduce:

### Military HUD styling

No:

- targeting reticles
- radar screens
- tactical grids
- fake classified labels

### Cyberpunk styling

No:

- excessive neon
- glitch effects
- scanlines
- terminal decoration

### Dashboard overload

No:

- 20 metrics on one screen
- unnecessary charts
- tiny text
- excessive cards

### Desktop UI on mobile

No:

- dense tables
- tiny buttons
- multi-column layouts that compromise readability

---

# 57. Design Principle for Global Map

The map exists to answer:

> **"Where is this information connected?"**

Not:

> **"How cool can we make a military-looking globe?"**

The map should therefore prioritize:

- geographic context
- relationships
- event distribution
- source coverage
- investigation scope

---

# 58. Design Principle for Investigations

An investigation should answer:

> **"What have I collected and how is it connected?"**

It should NOT automatically answer:

> **"What is really happening?"**

The latter requires human interpretation and source evaluation.

---

# 59. Design Principle for AI

If AI features are introduced later:

AI-generated information must be visually distinguishable from:

- source content
- structured database information
- user notes

AI output should include:

- provenance where possible
- uncertainty where relevant
- clear indication that it is generated

AI must not silently alter source-derived facts.

---

# 60. Definition of a Good Nocturne Screen

A screen is successful when a user can answer:

1. Where am I?
2. What am I looking at?
3. Where did this information come from?
4. What can I do next?
5. What is known versus uncertain?

If those questions cannot be answered quickly, simplify the interface.

---

# 61. Final Design Principle

Nocturne should feel like:

> **A serious research tool that happens to fit in your pocket.**

Not a military simulator.

Not a cyberpunk dashboard.

Not an AI gimmick.

Not a news reader.

It is a **mobile environment for understanding public information and connecting evidence.**
