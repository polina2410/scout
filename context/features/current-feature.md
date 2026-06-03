# Current Feature: Data Layer (SQLite)

## Status
In Progress

## Goals
- Add `modernc.org/sqlite` pure-Go driver (no CGo)
- Define `Photo`, `Prediction`, `ClassID` types and `ListParams` in `internal/db/types.go`
- Implement `Open`/`Close` in `internal/db/db.go` opening `predictions.db` read-only
- Implement `GetPhoto(ctx, id)` returning typed `ErrNotFound` sentinel on miss
- Implement `ListPhotos(ctx, ListParams)` with keyset cursor pagination and single-prediction filter (one prediction must satisfy both `classId` AND `minConfidence`)
- Wire DB open/close into `main.go` startup
- Tests: `GetPhoto` happy + not-found, `ListPhotos` no filter / class / confidence / combined — all green

## Notes
- `predictions.db` is read-only — open with `mode=ro` URI param, never write
- Filter invariant: both filters must be satisfied by the **same** prediction row (subquery approach)
- `OriginalURL` field stays empty from the DB layer; handler fills it from MinIO presigner
- Cursor encodes `captured_at|id` (base64) for stable pagination
- `modernc.org/sqlite` published 2021-02-01, well past the 1-week dependency rule

## History

### project-structure-and-tooling
Scaffolded Go module, Makefile, Docker Compose for MinIO, `.env.example`, `go.sum`, and CI-ready project skeleton.

### config-logging-errors
Wired typed env-var config (`internal/config`), structured JSON logger (`internal/logger`), correlation-ID middleware (`internal/middleware`), and centralised error/JSON response helpers (`internal/handler`). Updated `main.go` to wire all components; `/health` returns `{"status":"ok","version":"dev"}` with `X-Request-ID` header.
