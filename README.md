# WinnerProxy

A lightweight, high-performance in-memory cache HTTP service written in Go. It exposes a small REST API for storing, retrieving, deleting, and inspecting cache entries, backed by [FreeCache](https://github.com/coocood/freecache) and served via [Gin](https://github.com/gin-gonic/gin).

## Features

- In-memory key/value cache powered by FreeCache (no external dependencies)
- RESTful HTTP API built on Gin
- Per-entry TTL (or "never expire" when TTL is `0`)
- Hit / miss / eviction / expiration statistics
- Auto-generates a default `config.yml` on first run
- Single static binary, no DB required

## Project Layout

```
WinnerProxy/
├── main.go                    # entrypoint
├── config/config.go          # YAML config loader
├── internal/
│   ├── cache/freecache.go    # FreeCache wrapper
│   ├── handler/handler.go    # HTTP handlers
│   └── router/router.go      # route registration
├── config.yml                # generated on first run
├── go.mod
└── go.sum
```

## Requirements

- Go 1.26.4 or later

## Build & Run

```bash
# Build a static binary into ./build
go build -o build/winnerproxy .

# Run from the directory where config.yml lives (the binary
# loads config.yml from the same folder as the executable)
./build/winnerproxy 
```

On first launch, WinnerProxy writes a default `config.yml` next to the executable if one is not present, then listens on the configured address.

## Configuration

`config.yml` is read from the directory containing the executable. Defaults:

```yaml
addr: ":2779"                    # HTTP listen address
cache_size: 104857600            # FreeCache size in bytes (100 MB)
```

Notes:
- `cache_size` has a minimum of 512 KB; smaller values are rounded up by FreeCache.
- Missing or unreadable config files fall back to defaults.

## API

Base URL: `http://<addr>`

### Health check

```
GET /health
```

Response `200 OK`:
```json
{ "status": "ok" }
```

### Get a cached value

```
GET /cache/:key
```

- `200 OK` — returns the raw value as `application/octet-stream`
- `404 Not Found` — key missing

### Set a cached value

```
POST /cache
Content-Type: application/json
```

Request body:
```json
{
  "key": "user:42",
  "value": "alice",
  "ttl_seconds": 60
}
```

- `ttl_seconds: 0` means the entry never expires.
- Response `200 OK`:
  ```json
  {
    "key": "user:42",
    "ttl_seconds": 60,
    "expires_at": "2026-07-03T12:34:56Z"
  }
  ```

### Delete a cached value

```
DELETE /cache/:key
```

Response `200 OK`:
```json
{ "key": "user:42", "deleted": true }
```

### Cache statistics

```
GET /cache/stats
```

Response `200 OK`:
```json
{
  "hit_count": 12,
  "miss_count": 3,
  "lookup_count": 15,
  "hit_rate": 0.8,
  "entry_count": 7,
  "overwrite_count": 1,
  "evacuate_count": 0,
  "expired_count": 0
}
```

## Quick Example

```bash
# Set a value (60s TTL)
curl -X POST http://localhost:2779/cache \
  -H 'Content-Type: application/json' \
  -d '{"key":"hello","value":"world","ttl_seconds":60}'

# Get it back
curl http://localhost:2779/cache/hello

# Inspect stats
curl http://localhost:2779/cache/stats

# Delete it
curl -X DELETE http://localhost:2779/cache/hello
```

## License

See [LICENSE](LICENSE).
