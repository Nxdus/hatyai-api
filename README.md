# Hatyai Flood SOS API

Reverse-proxy + cache for flood assistance data in Hatyai. Built with Go (Fiber), caches responses in Redis (TTL 60s), and fronted by an Nginx load balancer that spreads traffic across multiple app instances.

## Quick start with Docker
- Requirements: Docker + Docker Compose
- Command:
  ```bash
  docker compose up --build -d
  ```
- Services:
  - `nginx` listening at `http://localhost` (port 80)
  - `app1`, `app2` are Go API containers (internal port 3000)
  - `redis` used for cache (`REDIS_ADDR` = `redis:6379` by default)

Check status: `curl http://localhost/health`

## Endpoints
- `GET /v1` : return the raw feed from upstream (or cache) as-is
- `GET /v1/health` : Redis check (200 = ok, 503 = down)
- `GET /v1/province/:name` : filter items by province name
- `GET /v1/district/:name` : filter by district
- `GET /v1/subdistrict/:name` : filter by subdistrict
- `GET /v1/area_summary` : summary counts of provinces/districts/subdistricts
- `GET /v1/priority` : urgency ranking (southern region only). Query params: `priority_level` (`critical|high|medium|low|all`) and `limit` (number of items)
- `GET /v1/south` : items in the southern region only (by coordinates and province list)
- `GET /v1/area_summary/south` : area summary limited to the southern region

> You can use Thai or English names, e.g. `/province/Songkhla` or `/province/songkhla`. If the name has spaces, URL-encode with `%20` or use `+`.

## Sample requests/responses

### 1) Health check
```bash
curl http://localhost/v1/health
```
```json
{"status":"ok"}
```

### 2) Raw feed (truncated)
```bash
curl http://localhost/v1
```
```json
{
  "fetched_at": "2025-11-26T12:01:46.626112",
  "data": {
    "data": [
      {
        "_id": "692449f459f42522e305db79",
        "location": {
          "type": "Feature",
          "properties": {
            "other": "8 people trapped (2 kids, 1 elderly). Need urgent evacuation and help for a bedridden patient.",
            "province": "Songkhla",
            "district": "Hat Yai",
            "subdistrict": "Hat Yai",
            "sick_level_summary": 4,
            "running_number": "HDY68-1124-0180",
            "type_name": "Medical/volunteer doctor request",
            "ages": "60",
            "disease": "Has chronic condition",
            "updated_at": "2025-11-24T12:05:08.017000Z",
            "patient": 1
          },
          "geometry": {
            "type": "Point",
            "coordinates": [100.4630701089612, 7.000351366600654]
          }
        },
        "running_number": "HDY68-1124-0180",
        "updated_at": "2025-11-24T12:05:08.017000Z",
        "created_at": "2025-11-24T12:05:08.017000Z"
      }
      // ... other items ...
    ]
  }
}
```

### 3) Filter by province
```bash
curl http://localhost/v1/province/songkhla
```
```json
{
  "province": "Songkhla",
  "count": 1,
  "items": [
    {
      "_id": "692449f459f42522e305db79",
      "location": { "...": "..." },
      "running_number": "HDY68-1124-0180",
      "updated_at": "2025-11-24T12:05:08.017000Z",
      "created_at": "2025-11-24T12:05:08.017000Z"
    }
  ]
}
```

### 4) Priority list (south only)
```bash
curl "http://localhost/v1/priority?limit=2&priority_level=critical"
```
```json
{
  "count": 42,
  "items": [
    {
      "_id": "692449f459f42522e305db79",
      "location": { "...": "..." },
      "running_number": "HDY68-1124-0180",
      "priority": {
        "score": 83,
        "level": "critical",
        "reasons": [
          "<reason 1>",
          "<reason 2>",
          "<reason 3>"
        ]
      }
    }
    // ... sorted by score, desc ...
  ]
}
```

### 5) Area summary
```bash
curl http://localhost/v1/area_summary
```
```json
{
  "provinces": { "total": 14, "items": [ { "name": "Songkhla", "count": 10 }, { "name": "Phatthalung", "count": 4 } ] },
  "districts": { "total": 32, "items": [ { "name": "Hat Yai", "count": 6 }, { "name": "Khlong Hoi Khong", "count": 1 } ] },
  "subdistricts": { "total": 40, "items": [ { "name": "Hat Yai", "count": 3 } ] }
}
```

## How it works (summary)
- Fetches data from `https://storage.googleapis.com/pple-media/hdy-flood/sos.json`
- Caches in Redis for 60 seconds; background refresh via ETag
- Nginx load balancing (`least_conn`) to `app1` and `app2`
- Only port 80 is exposed (`http://localhost`)

## Dev / testing without Docker
- Requires Go >= 1.21 and a running Redis
- Set `REDIS_ADDR` (e.g. `localhost:6379`) then run `go run .`
- Use `http://localhost:3000` (bypassing Nginx)

## Notes
- Data is pass-through from upstream; fields inside `properties` may change
- `priority.reasons` are heuristic strings (original encoding may appear as-is)
- If the filter name contains spaces, URL-encode (`%20`) or use `+`
