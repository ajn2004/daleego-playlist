# Rotator - Local-First Plex TV Rotation Service

A service that manages a rotating playlist of TV episodes from Plex.

## Quick Start

1. Copy `.env.example` to `.env` and configure:

```bash
cp .env.example .env
# Edit .env with your Plex URL and token
```

2. Start the database:

```bash
docker compose up -d postgres
```

3. Run migrations:

```bash
make migrate-up
```

Run this command after updating the application to apply pending database
migrations before using new features.

4. Start the server:

```bash
make dev
```

## Plex Spike

Test basic Plex connectivity:

```bash
export PLEX_URL=http://your-plex-server:32400
export PLEX_TOKEN=your-token
make spike
```

## API

- `GET /healthz` - Health check
- `GET /api/v1/status` - Service status
- `GET /api/v1/series` - List series
- `PATCH /api/v1/series/{id}` - Update series (set active)
- `POST /api/v1/rotations/generate` - Generate a new rotation
- `POST /api/v1/rotations/{id}/publish` - Publish rotation to Plex
- `POST /api/v1/rotations/current/sync` - Sync watched state
- `POST /api/v1/playlists/{id}/clear` - Clear the stored queue and its Plex playlist
- `POST /api/v1/playlists/{id}/refill` - Rebuild and publish a fresh queue
- `POST /api/v1/playlists/{id}/sync` - Detect playback progress, advance completed serial cursors, and refill the queue
- `GET /api/v1/playlists/{id}/plex-items` - Read the current Plex playlist order
- `PUT /api/v1/playlists/{id}/plex-items` - Replace the Plex playlist with ordered episode IDs

## Clients

### Bash

```bash
export ROTATOR_URL=http://localhost:8090/api/v1
./clients/bash/rotator status
./clients/bash/rotator generate
```

### Lua

```bash
lua clients/lua/rotator.lua
```

The Lua client requires LuaSocket and Lua CJSON. Install them with your package manager or LuaRocks:

```bash
# LuaRocks
luarocks install luasocket
luarocks install lua-cjson

# Ubuntu/Debian
apt install lua-socket lua-cjson

# macOS (Homebrew)
brew install lua-socket lua-cjson
```
