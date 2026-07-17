Yes. I would treat this as three services/components plus multiple clients:

Postgres owns application state.
Go server owns Plex integration, progression, scoring, rotation generation, and playlist synchronization.
React web client provides normal configuration and monitoring.

Bash and Lua are not additional backends. They are lightweight clients of the same HTTP API.

Plex’s role remains playback. The user opens a normal playlist such as TV Rotation in Plex and watches it like any other playlist. Plex now provides an official Media Server API specifically for custom integrations, so this is a reasonable integration boundary.

MVP product definition

The MVP succeeds when it can:

Connect to one local Plex server.
Import selected television series.
Establish the next episode for each active series.
Calculate programmable candidate scores using:
next-episode rating;
forward-window weighted rating;
episode runtime;
available session time;
recency or repetition constraints.
Generate a persistent rotation.
Create or replace a Plex playlist containing that rotation.
Detect watched episodes.
Advance only the corresponding series pointer.
Regenerate the playlist when the rotation is complete.
Let a user inspect and modify all of this through a web UI.

The important vertical slice is:

Select series
    ↓
Calculate next episodes
    ↓
Generate 2-high / 1-low rotation
    ↓
Publish "TV Rotation" playlist to Plex
    ↓
Watch normally in Plex
    ↓
Service detects completion
    ↓
Advance pointers and generate next rotation
Runtime architecture
                         ┌────────────────────┐
                         │ Plex clients       │
                         │ TV / Roku / Web    │
                         └─────────┬──────────┘
                                   │
                              plays playlist
                                   │
┌──────────────────┐      ┌────────▼──────────┐
│ React web client │─────▶│ Go API server     │
└──────────────────┘ HTTP │                    │
                           │ rotation engine    │
┌──────────────────┐─────▶│ Plex adapter       │
│ Bash scripts     │ HTTP │ progress sync      │
└──────────────────┘      │ playlist publisher │
                           └─────┬────────┬─────┘
┌──────────────────┐             │        │
│ Lua client/TUI   │─────────────┘        │
└──────────────────┘                      │
                                 ┌────────▼────────┐
                                 │ PostgreSQL      │
                                 └─────────────────┘

Run it locally through Docker Compose:

plex-rotator-api       :8090
plex-rotator-web       :8091
plex-rotator-postgres  :5432

The frontend could later be compiled and served by the Go server, reducing deployment to only the API container and Postgres.

Recommended repository

This closely matches the stack you already used for Bread Lab:

plex-rotator/
├── cmd/
│   ├── server/
│   │   └── main.go
│   └── rotatorctl/
│       └── main.go
├── internal/
│   ├── api/
│   │   ├── handlers/
│   │   ├── middleware/
│   │   └── router.go
│   ├── config/
│   ├── media/
│   │   ├── media.go
│   │   └── plex/
│   │       ├── client.go
│   │       ├── library.go
│   │       ├── playlists.go
│   │       └── progress.go
│   ├── rotation/
│   │   ├── candidate.go
│   │   ├── policy.go
│   │   ├── scoring.go
│   │   └── generator.go
│   ├── progression/
│   │   ├── cursor.go
│   │   └── reconcile.go
│   ├── repository/
│   └── service/
├── db/
│   ├── migrations/
│   ├── queries/
│   └── sqlc.yaml
├── web/
│   ├── src/
│   ├── package.json
│   └── vite.config.ts
├── clients/
│   ├── bash/
│   │   └── rotator
│   └── lua/
│       └── rotator.lua
├── tests/
│   ├── fixtures/
│   └── integration/
├── compose.yaml
├── Makefile
├── AGENTS.md
├── README.md
└── go.mod

Suggested Go dependencies:

chi             HTTP routing
pgx             PostgreSQL driver
sqlc            generated database access
goose           migrations
slog            structured logging

Go’s database/sql model already provides pooled, concurrency-safe database access, although using pgx through sqlc is a good fit here.

There is a community Go Plex client, including a Plex linking flow, but I would place it behind your own narrow PlexClient interface. That prevents the rest of the system from depending on incomplete or changing client-library abstractions.

Domain boundaries
Plex is authoritative for
Media identity and Plex ratingKey.
Library availability.
Episode runtime.
Plex metadata and ratings.
Watched and in-progress state.
Actual playlist contents.
Postgres is authoritative for
Which series are active.
Current episode pointer.
Rotation policies.
Current frozen rotation.
Selection history.
Skips and rerolls.
Manual rating overrides.
Playlist binding.
Reconciliation history.
The playlist is a projection

Do not use the playlist itself as your source of truth.

Postgres rotation
      ↓ publish
Plex playlist

If somebody deletes or edits the playlist in Plex, the application should be able to recreate it.

Core data model

I would use these tables for the MVP.

media_servers
CREATE TABLE media_servers (
    id UUID PRIMARY KEY,
    kind TEXT NOT NULL CHECK (kind IN ('plex', 'jellyfin')),
    name TEXT NOT NULL,
    base_url TEXT NOT NULL,
    access_token_encrypted TEXT NOT NULL,
    server_identifier TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

Even though Plex is first, retaining kind makes the media adapter boundary explicit.

series
CREATE TABLE series (
    id UUID PRIMARY KEY,
    media_server_id UUID NOT NULL REFERENCES media_servers(id),
    server_series_id TEXT NOT NULL,
    library_id TEXT NOT NULL,
    title TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT false,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (media_server_id, server_series_id)
);
episodes
CREATE TABLE episodes (
    id UUID PRIMARY KEY,
    series_id UUID NOT NULL REFERENCES series(id),
    server_episode_id TEXT NOT NULL,
    season_number INTEGER NOT NULL,
    episode_number INTEGER NOT NULL,
    absolute_position INTEGER NOT NULL,
    title TEXT NOT NULL,
    duration_seconds INTEGER,
    rating NUMERIC(4, 2),
    vote_count INTEGER,
    originally_available_at DATE,
    metadata JSONB NOT NULL DEFAULT '{}',

    UNIQUE (series_id, server_episode_id),
    UNIQUE (series_id, absolute_position)
);
series_progress
CREATE TABLE series_progress (
    series_id UUID PRIMARY KEY REFERENCES series(id),
    last_watched_episode_id UUID REFERENCES episodes(id),
    next_episode_id UUID REFERENCES episodes(id),
    last_watched_position INTEGER,
    next_position INTEGER,
    synchronized_at TIMESTAMPTZ,
    candidate_score NUMERIC,
    candidate_score_details JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

The cursor uses absolute_position. Season and episode numbers remain display metadata.

rotation_profiles
CREATE TABLE rotation_profiles (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    configuration JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
rotations
CREATE TABLE rotations (
    id UUID PRIMARY KEY,
    profile_id UUID NOT NULL REFERENCES rotation_profiles(id),
    status TEXT NOT NULL
        CHECK (status IN ('draft', 'published', 'completed', 'cancelled')),
    generated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    available_minutes INTEGER,
    random_seed BIGINT
);
rotation_items
CREATE TABLE rotation_items (
    id UUID PRIMARY KEY,
    rotation_id UUID NOT NULL REFERENCES rotations(id),
    position INTEGER NOT NULL,
    series_id UUID NOT NULL REFERENCES series(id),
    episode_id UUID NOT NULL REFERENCES episodes(id),
    slot_kind TEXT NOT NULL,
    candidate_score NUMERIC NOT NULL,
    score_details JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'in_progress', 'watched', 'skipped')),
    UNIQUE (rotation_id, position)
);
playlist_bindings
CREATE TABLE playlist_bindings (
    id UUID PRIMARY KEY,
    media_server_id UUID NOT NULL REFERENCES media_servers(id),
    rotation_profile_id UUID NOT NULL REFERENCES rotation_profiles(id),
    server_playlist_id TEXT,
    playlist_name TEXT NOT NULL DEFAULT 'TV Rotation',
    synchronized_at TIMESTAMPTZ,
    UNIQUE (media_server_id, rotation_profile_id)
);
Programmable policy design

Do not begin with arbitrary executable user code.

Start with a declarative policy document that the web UI, Bash scripts, and Lua client can all manipulate.

{
  "slots": [
    {
      "name": "high-arc-1",
      "pool": {
        "order_by": "arc_rating",
        "direction": "descending",
        "limit": 5
      },
      "selection": "random"
    },
    {
      "name": "high-arc-2",
      "pool": {
        "order_by": "arc_rating",
        "direction": "descending",
        "limit": 5
      },
      "selection": "random"
    },
    {
      "name": "low-episode",
      "pool": {
        "order_by": "episode_rating",
        "direction": "ascending",
        "limit": 5
      },
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
    "different_series_per_cycle": true,
    "avoid_previous_series": true,
    "include_specials": false,
    "maximum_episode_minutes": null,
    "session_budget_minutes": 150
  }
}

This is programmable enough for the MVP without introducing a scripting runtime or code-execution security boundary.

Later, the declarative document can become an AST that supports:

filter
rank
sample
exclude
penalize
boost
fit-to-time

And only after the behavior is stable should you consider custom Lua scoring scripts.

Candidate model

The rotation engine should never operate directly on Plex objects.

Normalize everything into:

type Candidate struct {
    SeriesID           uuid.UUID
    EpisodeID          uuid.UUID
    SeriesTitle        string
    EpisodeTitle       string
    SeasonNumber       int
    EpisodeNumber      int
    Duration           time.Duration

    EpisodeRating      float64
    ArcRating          float64
    CombinedRating     float64

    LastSelectedAt     *time.Time
    LastWatchedAt      *time.Time
    ConsecutivePenalty float64
}

The forward arc:

func WeightedArcRating(
    ratings []float64,
    decay float64,
) float64 {
    var sum float64
    var totalWeight float64

    for offset, rating := range ratings {
        weight := math.Pow(decay, float64(offset))
        sum += rating * weight
        totalWeight += weight
    }

    if totalWeight == 0 {
        return 0
    }

    return sum / totalWeight
}

Combined score:

combined :=
    policy.EpisodeWeight*candidate.EpisodeRating +
    policy.ArcWeight*candidate.ArcRating

Keep the complete calculation in score_details:

{
  "episode_rating": 7.2,
  "arc_rating": 8.4,
  "episode_weight": 0.4,
  "arc_weight": 0.6,
  "combined_rating": 7.92,
  "window": [7.2, 8.6, 8.9, 8.1, 8.7]
}

That makes the tool explainable in the UI.

Time-budget behavior

Duration should initially be a constraint rather than a vague score modifier.

For example:

Available time: 125 minutes

Candidate rotation:
    44 minutes
    51 minutes
    48 minutes
Total:
    143 minutes — rejected

The generator can select slots sequentially while tracking remaining time:

remaining := policy.SessionBudget

for _, slot := range policy.Slots {
    pool := eligibleCandidates(slot, remaining)
    selected := selectCandidate(pool, slot)

    rotation = append(rotation, selected)
    remaining -= selected.Duration
}

The UI can expose:

Session duration

○ No limit
○ 60 minutes
○ 90 minutes
● 120 minutes
○ Custom

Eventually, you can support “best rotation fitting within two hours,” but sequential constrained selection is sufficient for the MVP.

Progress synchronization

Normal rotation generation must not scan the entire library.

Use two sync modes.

Import sync

Run when:

the Plex server is first connected;
a series becomes active;
the user explicitly refreshes metadata;
new episodes are detected.

This imports the show’s episode sequence.

Progress sync

Run frequently, but only against:

current rotation items;
active-series pointers;
recently changed Plex items.
Current item watched?
    yes → mark rotation item watched
          advance that series pointer
          recalculate its candidate score

    no  → leave state unchanged

Plex webhooks can later reduce polling, but they require Plex Pass.

For the MVP, use:

sync when the web page opens;
sync before generating a rotation;
polling every 60–120 seconds;
a manual Sync Plex action.
Plex adapter interface

Keep Plex isolated:

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

Everything outside internal/media/plex should depend only on this interface.

That makes Jellyfin a later adapter rather than a migration:

internal/media/plex
internal/media/jellyfin
HTTP API

Use /api/v1 from the start.

Connections
POST   /api/v1/media-servers
POST   /api/v1/media-servers/{id}/test
GET    /api/v1/media-servers
Library and series
POST   /api/v1/media-servers/{id}/sync
GET    /api/v1/series
GET    /api/v1/series/{id}
PATCH  /api/v1/series/{id}
POST   /api/v1/series/{id}/sync
POST   /api/v1/series/{id}/reconcile

Example:

{
  "active": true,
  "next_episode_id": "optional-manual-correction"
}
Policies
GET    /api/v1/rotation-profiles
POST   /api/v1/rotation-profiles
GET    /api/v1/rotation-profiles/{id}
PUT    /api/v1/rotation-profiles/{id}
POST   /api/v1/rotation-profiles/{id}/preview
Rotations
GET    /api/v1/rotations/current
POST   /api/v1/rotations/generate
POST   /api/v1/rotations/{id}/publish
POST   /api/v1/rotations/{id}/reroll
POST   /api/v1/rotation-items/{id}/skip
POST   /api/v1/rotations/{id}/sync
Status
GET    /healthz
GET    /readyz
GET    /api/v1/status
Web interface

The first web client needs four screens.

Dashboard
TV Rotation

Current cycle: 1 of 3 watched
Available time: 120 minutes
Plex playlist: synchronized 2 minutes ago

1. The Expanse       S03E06   44m   High
2. Babylon 5         S02E08   43m   High
3. Doctor Who        S11E05   50m   Low

[Sync Plex] [Reroll] [Open in Plex]
Active series
Series             Next episode    Episode   Arc    Active
The Expanse        S03E06           9.1       9.3    Yes
Babylon 5          S02E08           8.2       8.8    Yes
Doctor Who         S11E05           5.8       6.3    Yes

The user can:

activate or pause a series;
correct its next episode;
inspect the upcoming rating window;
manually refresh it.
Policy editor

Use normal controls, not a JSON textarea as the primary interface:

Cycle
    High / High / Low

Top pool size
    5

Window size
    5 episodes

Window decay
    0.72

Episode rating weight
    40%

Arc rating weight
    60%

Available time
    120 minutes

An advanced tab can expose the underlying JSON.

History

Show:

generated rotations;
watched items;
skipped items;
rerolls;
cursor corrections;
why each item was selected.
Bash interface

Because the API uses stable JSON, Bash can remain extremely small.

#!/usr/bin/env bash
set -euo pipefail

base_url="${ROTATOR_URL:-http://localhost:8090/api/v1}"

case "${1:-}" in
  status)
    curl --fail --silent "${base_url}/status" | jq
    ;;

  current)
    curl --fail --silent "${base_url}/rotations/current" | jq
    ;;

  generate)
    curl \
      --fail \
      --silent \
      --request POST \
      --header "Content-Type: application/json" \
      --data '{}' \
      "${base_url}/rotations/generate" | jq
    ;;

  publish)
    rotation_id="${2:?rotation id required}"
    curl \
      --fail \
      --silent \
      --request POST \
      "${base_url}/rotations/${rotation_id}/publish" | jq
    ;;

  sync)
    curl \
      --fail \
      --silent \
      --request POST \
      "${base_url}/rotations/current/sync" | jq
    ;;

  *)
    echo "usage: rotator {status|current|generate|publish ID|sync}" >&2
    exit 2
    ;;
esac

Example use:

rotator generate
rotator publish 152d4f5e-...
rotator current
Lua interface

The Lua client should initially be another HTTP client, not a plugin runtime.

A minimal terminal display can use LuaSocket and a JSON library:

local http = require("socket.http")
local json = require("cjson")

local base_url =
    os.getenv("ROTATOR_URL")
    or "http://localhost:8090/api/v1"

local body, status = http.request(
    base_url .. "/rotations/current"
)

if tonumber(status) ~= 200 then
    io.stderr:write(
        "request failed: " .. tostring(status) .. "\n"
    )
    os.exit(1)
end

local rotation = json.decode(body)

for _, item in ipairs(rotation.items) do
    print(string.format(
        "%d. %s — S%02dE%02d (%dm) [%s]",
        item.position,
        item.series_title,
        item.season_number,
        item.episode_number,
        math.floor(item.duration_seconds / 60),
        item.slot_kind
    ))
end

This can evolve into:

a terminal UI;
a Hyprland launcher;
a Roku-remote companion;
a desktop pop-up;
a Lua integration in your home automation system.

The same API remains underneath all of them.

Build order
Milestone 1: Plex integration spike

Do this before building the full schema or frontend.

Create one Go command that:

Reads PLEX_URL and PLEX_TOKEN.
Connects to Plex.
Lists television libraries.
Lists episodes for one configured show.
Prints:
rating key;
season and episode;
rating;
duration;
watched state.
Creates a three-item test playlist.

The acceptance condition is:

A test playlist created by the Go program appears and plays in a normal Plex client.

Until this works, do not build the UI.

Milestone 2: Persistent active-series model

Implement:

migrations;
series import;
episode import;
active-series toggle;
progress cursor;
manual cursor correction;
progress reconciliation.

Acceptance condition:

Restarting the service preserves active series and their next episodes.

Milestone 3: Deterministic rotation engine

Implement the rotation engine without Plex calls.

Feed it fixtures and test:

high/high/low selection;
five-item pools;
no duplicate series;
rating windows;
duration budget;
missing ratings;
short seasons;
end-of-series;
seeded random selection.

Acceptance condition:

Given a fixture and random seed, it always produces the expected rotation.

Milestone 4: Playlist projection

Connect the engine to Plex:

database rotation
    → Plex playlist
    → verify playlist contents

Acceptance condition:

Generating and publishing a rotation produces a playable Plex playlist with exactly the selected episodes in order.

Milestone 5: Progress loop

Implement:

poll current items
    → mark watched
    → advance pointer
    → complete cycle
    → generate next cycle
    → update playlist

Acceptance condition:

Watching one episode in Plex advances only that series and updates application state without a full library scan.

Milestone 6: Web client

Build the four screens after the complete backend flow works.

Milestone 7: Bash and Lua clients

Because they share the HTTP API, these should be small once the API is stable.

Initial integration tests

The most important tests are not UI tests.

Given three active series
When a rotation is generated
Then three distinct next episodes are selected.

Given a five-episode scoring window
When the cursor advances one episode
Then only that series score is recalculated.

Given a 120-minute budget
When three selected episodes total 145 minutes
Then the generator chooses another eligible candidate.

Given an existing published rotation
When the endpoint is called again
Then it returns the same frozen rotation.

Given a watched Plex episode
When reconciliation runs
Then the corresponding pointer advances exactly once.

Given Plex temporarily reports stale state
When the local cursor is ahead
Then the service does not automatically rewind it.

Given a deleted Plex playlist
When publish runs
Then the playlist is recreated.
The first Codex work order

Start Codex with one narrow objective:

Build the Plex integration spike for a local Go project.

Requirements:

- Initialize a Go module.
- Use environment variables PLEX_URL and PLEX_TOKEN.
- Define a narrow PlexClient interface.
- Implement:
  - test connection;
  - list libraries;
  - list television series;
  - list ordered episodes for a series;
  - create or replace a playlist from episode rating keys.
- Add a CLI under cmd/plex-spike.
- Do not add Postgres or a frontend yet.
- Use context-aware HTTP requests, structured errors, and slog.
- Store Plex XML/JSON response fixtures under tests/fixtures.
- Add unit tests for response parsing.
- Never log the Plex token.
- Document exact commands in README.md.

Completion criterion:

Running the CLI against a local Plex server creates a playable
three-episode playlist called "TV Rotation Test".

That is where I would begin: prove library reading and playlist writing first, then place the durable cursor and policy engine behind it. The entire product depends on that vertical Plex interaction, while Postgres, React, Bash, and Lua are all straightforward once that contract is stable.
