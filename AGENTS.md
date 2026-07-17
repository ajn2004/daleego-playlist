# AGENTS.md

## Project overview

This repository contains a local-first television rotation service for Plex.

The application maintains a set of active television series, tracks the next episode of each series, scores those episodes according to configurable policies, and publishes a generated rotation as a Plex playlist.

The intended user experience is:

1. Configure a Plex server.
2. Select active television series.
3. Define a rotation policy.
4. Generate a playlist of next episodes.
5. Watch the playlist using a normal Plex client.
6. Synchronize watched progress back into the application.
7. Advance the affected series and generate future rotations.

The service must remain usable without modifying Plex or installing a Plex plugin.

## Primary goals

The MVP must support:

* Connecting to one local Plex server.
* Importing television series and episodes.
* Activating and pausing series.
* Tracking a persistent next-episode cursor for each active series.
* Scoring candidates using:

  * individual episode rating;
  * forward-window weighted rating;
  * episode duration;
  * available viewing time;
  * recent-series repetition constraints.
* Generating a configurable rotation.
* Publishing the rotation to a Plex playlist.
* Synchronizing watched state from Plex.
* Monitoring and editing state through a web interface.
* Interacting with the backend through Bash and Lua clients.

## Non-goals for the MVP

Do not implement the following unless explicitly requested:

* Video streaming or transcoding.
* A custom Plex client.
* A Plex plugin.
* A Jellyfin adapter.
* Multi-user authentication.
* Cloud hosting.
* Remote Plex account discovery.
* Arbitrary user-supplied executable scoring code.
* Machine-learning recommendations.
* Automatic external metadata matching.
* A complex terminal UI.
* Mobile-native applications.

Design boundaries should allow some of these later, but do not build them prematurely.

## Technology stack

### Backend

* Go
* `chi` for HTTP routing
* `pgx` for PostgreSQL
* `sqlc` for generated database access
* `goose` for database migrations
* `slog` for structured logging

### Database

* PostgreSQL

### Frontend

* React
* TypeScript
* Vite

### Additional clients

* Bash using `curl` and `jq`
* Lua using a lightweight HTTP and JSON library

### Deployment

* Docker Compose
* Local network only for the MVP

## Repository layout

Use the following organization unless the existing repository establishes a better equivalent:

```text
.
├── cmd/
│   ├── server/
│   ├── plex-spike/
│   └── rotatorctl/
├── internal/
│   ├── api/
│   ├── config/
│   ├── media/
│   │   └── plex/
│   ├── progression/
│   ├── repository/
│   ├── rotation/
│   └── service/
├── db/
│   ├── migrations/
│   ├── queries/
│   └── sqlc.yaml
├── web/
├── clients/
│   ├── bash/
│   └── lua/
├── tests/
│   ├── fixtures/
│   └── integration/
├── compose.yaml
├── Makefile
├── README.md
└── AGENTS.md
```

Keep executable entry points small. Business logic belongs under `internal`.

## Core architecture

The application has four logical layers:

```text
HTTP clients
    ↓
Application services
    ↓
Domain logic
    ↓
Repositories and Plex adapter
```

The main components are:

### Plex adapter

Responsible for communicating with Plex.

It may:

* test connectivity;
* list libraries;
* list series;
* list episodes;
* inspect episode progress;
* create or update playlists.

It must not:

* contain rotation policy logic;
* directly update application database records;
* decide which episodes should be selected;
* expose raw Plex response types outside the adapter package.

### Progression service

Responsible for:

* determining the current next episode;
* advancing a series cursor;
* reconciling local progress with Plex;
* detecting completed series;
* preventing accidental cursor rewinds.

### Rotation engine

Responsible for:

* building candidate records;
* calculating ratings and arc scores;
* applying filters and constraints;
* selecting candidates for rotation slots;
* respecting duration budgets;
* producing deterministic results when given a random seed.

The rotation engine must not call Plex or PostgreSQL directly.

### Playlist publisher

Responsible for projecting a stored rotation into Plex.

PostgreSQL is authoritative for the rotation. The Plex playlist is a derived projection.

## Sources of truth

### Plex is authoritative for

* Plex server media identifiers.
* Library contents.
* Episode availability.
* Episode duration.
* Plex watch state.
* Plex playback progress.
* Actual Plex playlist contents.

### PostgreSQL is authoritative for

* Active-series membership.
* Series cursors.
* Rotation policies.
* Current and historical rotations.
* Candidate score details.
* Skips and rerolls.
* Manual corrections.
* Playlist bindings.
* Synchronization history.

Do not infer application state solely from the Plex playlist.

## Domain invariants

Preserve these invariants unless a task explicitly changes them.

### Active-series cursor

Each active series has at most one next episode.

The cursor should use an application-defined ordered position rather than relying only on season and episode numbers.

```text
last watched episode
        ↓
next episode
```

When an episode is completed:

1. Mark the current rotation item watched.
2. Set it as the series' last watched episode.
3. Advance the series to the next eligible episode.
4. Recalculate only that series' candidate score.
5. Do not scan the entire media library.

### Cursor reconciliation

When Plex is ahead of the local cursor, local progress may advance.

When Plex appears behind the local cursor, do not automatically rewind. Record or report the discrepancy instead.

Synchronization must be idempotent.

Running the same synchronization twice must not advance a series twice.

### Rotations are frozen

Once a rotation is published, refreshing the page or requesting the current rotation must not silently regenerate it.

A rotation changes only through an explicit operation such as:

* complete;
* reroll;
* cancel;
* regenerate;
* manual edit.

### Rotation slots

A policy defines ordered slots.

Example:

```text
high arc
high arc
low next-episode rating
```

When enough eligible series exist, avoid selecting the same series more than once in one rotation.

### Playlist projection

Publishing the same rotation repeatedly must be safe.

The playlist publisher should create or update the configured Plex playlist to match the stored rotation exactly.

## Candidate representation

Rotation logic should operate on normalized application types, never raw Plex objects.

A candidate should contain at least:

```go
type Candidate struct {
    SeriesID      uuid.UUID
    EpisodeID     uuid.UUID
    SeriesTitle   string
    EpisodeTitle  string

    SeasonNumber  int
    EpisodeNumber int

    Duration      time.Duration

    EpisodeRating float64
    ArcRating     float64
    CombinedScore float64

    LastSelectedAt *time.Time
    LastWatchedAt  *time.Time
}
```

Add fields only when they are required by a concrete feature.

## Rating behavior

The initial scoring system should support:

* next-episode rating;
* forward-window weighted rating;
* a weighted combination of those values;
* duration filtering;
* repetition penalties or exclusions.

The forward-window rating should use an explicit decay:

```go
weight := math.Pow(decay, float64(offset))
```

Store score explanations with generated rotation items.

For example:

```json
{
  "episode_rating": 7.2,
  "arc_rating": 8.4,
  "episode_weight": 0.4,
  "arc_weight": 0.6,
  "combined_score": 7.92,
  "window": [7.2, 8.6, 8.9, 8.1, 8.7]
}
```

A user must be able to understand why an episode was selected.

## Policy representation

Use declarative configuration rather than arbitrary executable scripts.

Example:

```json
{
  "slots": [
    {
      "name": "high-arc-1",
      "rank_by": "arc_rating",
      "direction": "descending",
      "pool_size": 5,
      "selection": "random"
    },
    {
      "name": "high-arc-2",
      "rank_by": "arc_rating",
      "direction": "descending",
      "pool_size": 5,
      "selection": "random"
    },
    {
      "name": "low-episode",
      "rank_by": "episode_rating",
      "direction": "ascending",
      "pool_size": 5,
      "selection": "random"
    }
  ],
  "scoring": {
    "episode_weight": 0.4,
    "arc_weight": 0.6,
    "window_size": 5,
    "window_decay": 0.72
  },
  "constraints": {
    "different_series_per_rotation": true,
    "include_specials": false,
    "session_budget_minutes": 120
  }
}
```

Validate policy configuration at the API boundary.

Do not allow invalid policies to enter the database.

## API conventions

Use versioned routes:

```text
/api/v1
```

Use JSON request and response bodies.

Return structured errors:

```json
{
  "error": {
    "code": "rotation_not_found",
    "message": "No active rotation exists.",
    "details": {}
  }
}
```

Use appropriate HTTP status codes.

Suggested routes include:

```text
GET    /healthz
GET    /readyz

GET    /api/v1/status

GET    /api/v1/media-servers
POST   /api/v1/media-servers
POST   /api/v1/media-servers/{id}/test

GET    /api/v1/series
GET    /api/v1/series/{id}
PATCH  /api/v1/series/{id}
POST   /api/v1/series/{id}/sync
POST   /api/v1/series/{id}/reconcile

GET    /api/v1/rotation-profiles
POST   /api/v1/rotation-profiles
PUT    /api/v1/rotation-profiles/{id}
POST   /api/v1/rotation-profiles/{id}/preview

GET    /api/v1/rotations/current
POST   /api/v1/rotations/generate
POST   /api/v1/rotations/{id}/publish
POST   /api/v1/rotations/{id}/reroll
POST   /api/v1/rotations/{id}/sync
```

Do not expose database models directly as API response types.

Define explicit request and response DTOs.

## Plex integration rules

Treat the Plex API as an unreliable external dependency even when it is local.

Every Plex request must:

* accept a `context.Context`;
* use a bounded timeout;
* return descriptive errors;
* avoid logging access tokens;
* validate required response fields;
* tolerate missing optional metadata;
* use fixtures in parser tests.

Do not spread Plex endpoint construction throughout the repository.

Centralize it under:

```text
internal/media/plex
```

Prefer a narrow internal interface:

```go
type MediaServer interface {
    TestConnection(ctx context.Context) error

    ListLibraries(ctx context.Context) ([]Library, error)

    ListSeries(
        ctx context.Context,
        libraryID string,
    ) ([]SeriesMetadata, error)

    ListEpisodes(
        ctx context.Context,
        seriesID string,
    ) ([]EpisodeMetadata, error)

    GetEpisodeProgress(
        ctx context.Context,
        episodeIDs []string,
    ) ([]EpisodeProgress, error)

    UpsertPlaylist(
        ctx context.Context,
        playlistID *string,
        name string,
        episodeIDs []string,
    ) (Playlist, error)
}
```

Do not add Jellyfin-specific behavior to this interface until a Jellyfin adapter is actually being implemented.

## Database rules

All schema changes require a migration.

Never edit an existing migration that may already have been applied. Add a new migration instead.

Use transactions for operations that change related records, especially:

* advancing a cursor;
* generating a rotation;
* publishing rotation state;
* reconciling watched progress.

Keep SQL in `db/queries` when using `sqlc`.

Do not write handwritten repository SQL when an equivalent `sqlc` query can be added cleanly.

Avoid putting complex policy calculations into SQL. Keep rotation logic in Go.

## Frontend rules

The frontend is a client of the HTTP API.

It must not:

* connect directly to PostgreSQL;
* call Plex directly;
* duplicate rotation scoring logic;
* infer cursor advancement locally.

The initial interface should prioritize:

* current rotation;
* playlist synchronization state;
* active-series management;
* progress correction;
* policy editing;
* selection explanations;
* history.

Use accessible HTML and keyboard-friendly controls.

The policy editor should use normal form controls first. Raw JSON may exist as an advanced view, but should not be the primary interface.

## Bash and Lua clients

Bash and Lua clients must use the same public HTTP API as the React frontend.

Do not create special backend behavior only for one client.

Bash scripts should:

* use `set -euo pipefail`;
* respect `ROTATOR_URL`;
* use `curl --fail`;
* produce useful exit codes;
* use `jq` for JSON presentation.

Lua clients should remain lightweight HTTP clients during the MVP.

Do not embed the rotation engine in Lua.

## Configuration and secrets

Configuration should be loaded from environment variables.

Examples:

```text
DATABASE_URL
HTTP_ADDR
PLEX_URL
PLEX_TOKEN
PLEX_PLAYLIST_NAME
LOG_LEVEL
```

Never commit:

* Plex tokens;
* database passwords;
* `.env` files containing secrets;
* session data;
* generated credentials.

Provide `.env.example` with placeholder values.

Never log a full Plex token or database connection string containing a password.

## Logging

Use `log/slog`.

Logs should include useful identifiers such as:

* series ID;
* episode ID;
* rotation ID;
* Plex rating key;
* synchronization operation.

Do not log large Plex response bodies during normal operation.

Errors should provide enough context to diagnose the failed operation without exposing secrets.

## Testing expectations

Every behavioral change should include tests at the narrowest useful layer.

### Unit tests

Required for:

* Plex response parsing;
* episode ordering;
* cursor advancement;
* weighted arc scoring;
* policy validation;
* candidate ranking;
* duration constraints;
* seeded random selection;
* duplicate-series prevention.

### Repository tests

Required for:

* cursor persistence;
* rotation creation;
* idempotent synchronization;
* transaction behavior;
* playlist binding state.

### Integration tests

Important integration scenarios include:

```text
Given several active series
When a rotation is generated
Then it selects the expected number of distinct next episodes.

Given a fixed random seed
When the same policy and candidate set are evaluated
Then the same rotation is produced.

Given a watched Plex episode
When reconciliation runs twice
Then the cursor advances exactly once.

Given Plex reports progress behind the local cursor
When reconciliation runs
Then the cursor is not automatically rewound.

Given a published rotation
When publish is called again
Then the Plex playlist still exactly matches the rotation.

Given an episode-duration budget
When a candidate would exceed the remaining time
Then another eligible candidate is considered.

Given a series at its final episode
When that episode is completed
Then the series has no next episode and is excluded from future rotations.
```

Use HTTP and Plex fixtures where possible.

Tests must not require a live Plex server unless explicitly marked as manual integration tests.

## Commands

Prefer stable commands exposed through a `Makefile`.

Expected commands should include equivalents of:

```bash
make dev
make test
make test-integration
make lint
make fmt
make generate
make migrate-up
make migrate-down
make web-dev
make compose-up
make compose-down
```

Before adding a new command, inspect the existing `Makefile`, `README.md`, and package scripts.

Do not invent duplicate command paths when an established one exists.

## Code quality

Go code must:

* pass `gofmt`;
* pass `go test ./...`;
* propagate contexts through I/O boundaries;
* wrap errors with operation context;
* avoid package-level mutable state;
* keep interfaces narrow;
* prefer explicit types over unstructured maps;
* avoid unnecessary abstraction.

TypeScript code must:

* pass the configured formatter and linter;
* avoid `any` unless justified;
* use typed API response models;
* keep server state separate from component-local UI state.

Do not introduce new major dependencies without a clear need.

## Working style for coding agents

Before editing:

1. Read this file.
2. Read `README.md`.
3. Inspect the relevant packages and tests.
4. Identify existing conventions.
5. Check the current migration state.
6. Run the smallest relevant test command.

When implementing:

* Make the smallest coherent change that completes the requested behavior.
* Prefer extending existing abstractions over creating parallel ones.
* Do not refactor unrelated code.
* Do not rename public API fields without explicit instruction.
* Preserve backward compatibility unless the task requires a breaking change.
* Add tests before or alongside behavioral code.
* Keep generated files generated; do not hand-edit them.
* Update documentation when commands, configuration, or API behavior change.

After implementing:

1. Run formatting.
2. Run relevant unit tests.
3. Run the full Go test suite when feasible.
4. Run frontend checks when frontend code changed.
5. Verify migrations when database code changed.
6. Summarize changed files and behavior.
7. Report any tests that could not be run.

## Scope control

Do not expand a task merely because a broader design appears attractive.

For example:

* A request to add arc scoring does not require redesigning the frontend.
* A request to add a Plex endpoint does not require adding Jellyfin.
* A request to add a CLI command does not require a terminal UI.
* A request to persist a field does not require replacing the repository layer.
* A request to create a playlist does not require remote playback control.

When encountering an architectural weakness outside the requested scope, document it briefly rather than fixing it opportunistically.

## Error handling philosophy

Failures should be visible and recoverable.

Examples:

* If Plex is unavailable, preserve the current rotation and report the synchronization failure.
* If playlist publishing fails, do not mark the rotation published.
* If one series cannot synchronize, continue processing other series where safe and report the partial failure.
* If an episode is missing from Plex, flag the series for reconciliation.
* If a policy cannot fill all slots, return a partial result only when the API contract explicitly permits it.

Do not silently discard inconsistent state.

## Definition of done

A task is complete when:

* the requested behavior is implemented;
* relevant tests pass;
* formatting and lint checks pass;
* API and database changes are documented;
* no secrets are exposed;
* failures return useful errors;
* the implementation respects the architecture and invariants in this file.

For Plex-facing work, completion normally also requires either:

* fixture-backed tests proving request and response handling; or
* a documented manual verification procedure against a local Plex server.

## Current implementation priority

Unless the repository has progressed beyond this point, prioritize work in this order:

1. Prove Plex connectivity and playlist creation.
2. Persist imported series and episodes.
3. Implement active-series cursors.
4. Implement deterministic rotation generation.
5. Publish stored rotations to Plex.
6. Reconcile watched state.
7. Build the web interface.
8. Add Bash and Lua clients.
9. Add advanced policy features.

The first major acceptance milestone is:

> A Go command connects to a local Plex server, selects three known episodes by Plex identifier, and creates a playable playlist named `TV Rotation Test`.

The second major acceptance milestone is:

> The application generates a rotation from persisted active-series cursors, publishes it to Plex, detects a watched episode, and advances exactly one series without scanning the entire library.
