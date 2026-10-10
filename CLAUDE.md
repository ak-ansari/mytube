# MyTube — Backend (Go)

Learning project: a bare-minimum YouTube clone. This repo is the API + media-processing worker.
Frontend lives in the sibling repo `../mytube-frontend` (Vue 3) and talks to this API at `http://localhost:8080`.

## Ground rules for changes

- **Do not refactor or optimise existing (pre-catalog) backend code right now.** Add new APIs/files alongside it. Edits to existing files should be limited to wiring (registering routes, constructing services in `cmd/*/main.go`, extending an interface) or a fix the new feature cannot work without — call those out explicitly.
- **Postgres is the primary datastore.** New features go in Postgres. Elasticsearch is only a search index; Redis is only queue/cache/locks. Keycloak owns identities (passwords, sessions); Postgres `users` holds the profile.
- Follow the layering for new features: `routes` → `handlers` → `services` → `repository` interface → `repository/postgres` impl. New feature = new files.
- Schema changes are new goose migrations only; never edit an applied migration.

## Commands

All commands run from the repo root (migrations are loaded from `./migrations` relative to CWD).

```bash
cp configs/config.example.yaml configs/config.yaml   # first time; config.yaml is gitignored
make               # docker-compose up (postgres, redis, minio, nginx, elasticsearch, kibana, keycloak) + MinIO bucket (needs `mc`)
make keycloak-dev  # once per Keycloak volume: admin console over http + realm loginTheme=mytube
make run-api       # API on :8080
make run-worker    # pipeline worker (needs ffmpeg/ffprobe on PATH)
make down
go build ./... && go vet ./...   # compile check; there are no tests yet
```

Ports: **HLS gateway 80** (nginx), Postgres 5432, Redis 6379, MinIO S3 API via nginx **9002** (presigned URLs point here), MinIO console 9001, ES 9200, Kibana 5601, **Keycloak 8180** (admin/admin).

Gotchas:
- A Homebrew `redis-server` on `127.0.0.1:6379` shadows the Docker Redis. MinIO publishes upload events into the Docker Redis, so the worker never sees uploads (videos stay `upload_pending`). Stop the local Redis (`brew services stop redis`) or point `REDIS.REDIS_HOST` at the Docker one.
- The MinIO bucket needs an event rule sending `originals/` puts to Redis (`mc event add myminio/mytube arn:minio:sqs::primary:redis --event put,delete --prefix originals/`); it persists in the MinIO volume.

## Private bucket + HLS playback gateway

The MinIO bucket is **private**. There are three ways in:

1. **S3 API on :9002 (nginx → MinIO).** Only SigV4-signed requests pass: an `Authorization` header (SDK, `mc`) or a presigned `X-Amz-Signature` query. Everything else gets 403. Thumbnails and avatars reach clients as presigned GETs (`thumbnail_url`, `avatar_url`), and uploads as presigned PUTs.
2. **HLS gateway on :80 (nginx `secure_link`).** URL: `/videos/<token>/<expires>/<videoId>/<file.m3u8|.ts>`, where `token = base64url(md5("<expires>:<videoId>:<STREAM.TOKEN_SECRET>"))`. The token sits in the *path*, so the relative URIs inside `master.m3u8` and the variant playlists inherit it and the player needs no hooks. Bad or foreign token gives 403, expired gives 410. nginx rewrites the request to MinIO's internal `:9000/<bucket>/segments/<id>/<file>`.
3. **Anonymous `GetObject` on `segments/*`** (set by `make setup-bucket`). It's only reachable on the docker network, because MinIO's :9000 isn't published to the host. This is the hop nginx uses; in production the equivalent is CloudFront Origin Access Control plus signed URLs or cookies.

- `internal/stream/signer.go` mints the URLs. `VideoCatalogService.GetDetails` returns `manifest_url` from it, so visibility and owner checks happen before a token is issued. TTL comes from `STREAM.TOKEN_TTL` and is rounded up to the hour so URLs stay cacheable.
- nginx config: `deployments/nginx/templates/default.conf.template`. The official image renders it with envsubst, taking `VIDEO_TOKEN_SECRET` and `MINIO_BUCKET` from the compose file (`VIDEO_TOKEN_SECRET` must equal `STREAM.TOKEN_SECRET`). After changing the secret, recreate nginx (`make docker-up`).
- `configs/config.yaml` needs a `STREAM` section (`BASE_URL`, `TOKEN_SECRET`, `TOKEN_TTL`). The API refuses to start without it.

## Auth (Keycloak)

- Realm `mytube` is imported from `deployments/keycloak/import/mytube-realm.json` on first start: public client `mytube-frontend` (PKCE, redirect `http://localhost:5173/*`, direct-access grants on for curl testing), self-registration enabled, demo user **demo / demo123**.
- `internal/auth/keycloak.go` verifies RS256 access tokens against the realm JWKS (`KEYCLOAK.URL/realms/REALM`), checks `iss` and `azp == CLIENT_ID`. Token `iss` must match `KEYCLOAK.URL` exactly, so frontend and API must both use `http://localhost:8180`.
- `middleware.Authenticate` (on all of `/api`): no header → anonymous; invalid token → 401; valid → `UserService.SyncFromToken` upserts `users` (id = Keycloak `sub`) and sets `c.Get("user")` (`*models.User`).
- `middleware.RequireUser()` guards write routes; `middleware.RequireVideoOwner(catalog)` guards `/:id` routes by uploader. Helpers: `middleware.CurrentUser(c)`, `CurrentUserID(c)` (nil when anonymous).
- The old hardcoded `middleware.AuthMiddleware()` stub is no longer wired.
- **Login theme `mytube`** (`deployments/keycloak/themes/mytube/login`, mounted into the container). It extends `keycloak.v2` (PatternFly 5) and is always dark, matching the app palette. `template.ftl` is a copy of keycloak.v2's with myTube header, subtitle, Roboto, favicon and footer added. `resources/css/mytube.css` restyles via PatternFly CSS variables; `messages/messages_en.properties` overrides the copy text. `start-dev` disables theme caching, so edits show on refresh. The realm JSON sets `loginTheme: mytube` for fresh volumes; existing volumes need `make keycloak-dev`. When upgrading Keycloak, re-diff `template.ftl` against the new keycloak.v2 one.
- Token for curl: `curl -s -X POST localhost:8180/realms/mytube/protocol/openid-connect/token -d grant_type=password -d client_id=mytube-frontend -d username=demo -d password=demo123`

## Layout (golang-standards/project-layout)

```
cmd/api, cmd/worker      entrypoints (main.go only wires dependencies)
internal/                all application code (not importable from outside the module)
  api/                   gin router, routes/, handlers/, middleware/, dto/
  auth/                  Keycloak token verification
  services/              business logic          repository/ (+ postgres/)  data access
  models/                domain + API structs    storage/, index/, queue/, cache/, media/, workers/, jobs/, config/, db/, util/
  workers/testdata/      sample MinIO bucket event
pkg/                     reusable, app-agnostic clients (logger, elastic, redis)
migrations/              goose SQL (applied on startup from ./migrations)
configs/                 config.example.yaml (+ your gitignored config.yaml)
deployments/             docker-compose.yml (project name pinned to "mytube"), nginx/templates/ (envsubst-rendered), keycloak/ (import/ realm JSON, themes/mytube login theme)
docs/                    todo.txt
```

Run every command from the repo root. `make` passes `-f deployments/docker-compose.yml`.

**Config lives in two places that must agree:** `deployments/docker-compose.yml` holds the infra credentials as literal dev values (Postgres user/pass/db, MinIO root user/pass, MinIO event key, bucket name, nginx `VIDEO_TOKEN_SECRET`). `configs/config.yaml` holds the same values for the Go apps. The makefile repeats the MinIO credentials for `setup-bucket`. Change all three together.

## Architecture

- `cmd/api/main.go` — config, logger, pg pool (runs migrations), redis queue/cache, ES index, MinIO store, `VideoService` (upload pipeline), `UserService`, `VideoCatalogService`, Keycloak verifier → `api.SetupRouter`.
- `cmd/worker/main.go` — same deps + `workers.Runner` (2 goroutines polling bucket events and the job queue).
- `internal/services/video_service.go` (+ `pipeline_coordinator.go`, `video_state_machine_service.go`) — original upload/processing logic.
- `internal/services/video_catalog_service.go` — feed/search/watch, studio CRUD, views, reactions, comments. `user_service.go` — profile, channels, subscriptions. `errors.go` — `ErrNotFound/ErrForbidden/ErrBadRequest` that handlers map to 404/403/400.
- `internal/repository/*.go` — interfaces; `internal/repository/postgres/*` — pgx implementations (raw SQL). `video_catalog_pg_repo.go` builds the joined "video card" query (videos + users + video_metadata) and owns `publishedVideoCond`.
- `internal/storage` — `ObjectStore` + MinIO impl. Keys: `originals/{videoId}/original.{ext}`, `thumbnails/{videoId}/…` (auto `{videoId}.jpeg`, custom uploads `{uuid}.{ext}`), `transcoded/{videoId}/{q}.mp4`, `segments/{videoId}/master.m3u8`, `avatars/{userId}/{uuid}.{ext}`. Bucket is private (see above).
- `internal/index/elasticsearch` — `video_index`; search returns IDs, rows then come from Postgres.
- `internal/models/catalog.go` — API shapes: `VideoCard`, `VideoDetails`, `Channel`, `Profile`, `Comment`, `Page[T]`. Services fill presigned `*_url` fields (`thumbnail_url`, `avatar_url`, `manifest_url`) so the client needs no extra calls.
- `migrations/` — goose SQL, auto-applied on startup. 004 adds `users.email/description`; 005 adds `videos.views`, `video_reactions`, `comments`, `subscriptions`. Next: `006_…sql`.

### Upload + processing flow

1. `POST /api/videos/upload/presign {file_name, size}` → inserts `videos` (stage 1 `upload_pending`) + `video_metadata`, returns `{id, key, url}`; client PUTs to MinIO.
2. MinIO → Redis hash → worker `ProcessBucketEvents` → `PipelineCoordinator.Advance`. Validate runs, then the thumbnail job; at the phase boundary `Advance` **waits until the video has a title** (status `valid`, thumbnail set).
3. `POST /api/videos/:id/confirm` saves title/desc/visibility/thumbnail, indexes into ES, advances to phase 2 (transcode → HLS segments → checksum → publish → `published`).
   Confirm must only be sent once phase 1 is finished (`status == valid` and `thumbnail` set). Earlier, `Advance` double-steps the pipeline. The frontend enforces this; the backend doesn't yet.

A video is publicly visible when `status = 'published' AND visibility = 'public'`; unlisted is watchable by link; owners can see everything.

### Endpoints (all under `/api`; 🔒 = login required)

| Method | Path | Notes |
|---|---|---|
| GET | `/videos/browse?search=&page=&size=` | home feed (latest) or ES search, published+public only |
| GET | `/videos/:id/details` | watch page payload (counts, my_reaction, is_subscribed, manifest_url) |
| GET | `/videos/:id/related` | other published videos |
| POST | `/videos/:id/view` | +1 view, deduped per user/IP for 30 min (Redis SETNX) |
| PUT 🔒 | `/videos/:id/reaction` `{value: 1\|-1\|0}` | like / dislike / clear |
| GET / POST 🔒 | `/videos/:id/comments` | list (paged) / add `{content}` |
| DELETE 🔒 | `/videos/:id/comments/:commentId` | comment author or video owner |
| GET 🔒 | `/videos/mine`, `/videos/liked`, `/videos/subscriptions` | studio list (all statuses), liked, subscription feed |
| PUT 🔒 | `/videos/:id` `{title?, description?, visibility?, thumbnail?}` | owner; also re-indexes ES |
| DELETE 🔒 | `/videos/:id` | owner; removes rows (cascade), ES doc and all object prefixes |
| POST 🔒 | `/videos/upload/presign`, `/videos/:id/confirm` | upload flow (confirm: owner only) |
| GET | `/videos/search`, `/videos/suggest`, `/videos/url`, `/videos/:id`, `/video-meta/:id` | original endpoints (search here is unfiltered; prefer `/browse`) |
| GET 🔒 / PUT 🔒 | `/users/me` | profile; update `{channel_name?, description?, avatar?}` |
| GET 🔒 | `/users/me/subscriptions` | subscribed channels |
| GET | `/channels/:id`, `/channels/:id/videos` | public channel + its published videos |
| POST 🔒 / DELETE 🔒 | `/channels/:id/subscribe` | subscribe / unsubscribe |
| GET 🔒 / GET | `/files/presign?key=&size=`, `/files/url?key=` | presigned PUT (thumbnails, avatars) / GET — both still accept any key (see known gaps) |

Paged responses are `{items, page, size, has_more}` (`size` capped at 50).

## Conventions

- Responses: `util.NewResponse(status, message, data, err)` → `{message, success, data, status, error}`; the frontend unwraps `data` and shows `message` on errors. New handlers use `respondOK` / `respondError` / `badRequest` / `uuidParam` / `paging` from `internal/api/handlers/respond.go`.
- Request DTOs: existing ones in `internal/api/dto`; new input structs live next to their service (`UpdateVideoInput`, `UpdateProfileInput`).
- Nullable DB columns scan into pointer fields; list queries fetch `size+1` rows and `models.NewPage` trims and sets `has_more`.
- Logging via `pkg/logger` (zap) in services/workers; handlers log unexpected errors in `respondError`.
- Module path: `github.com/ak-ansari/mytube`.

## Known gaps / later

- **Security:** `/files/presign` signs a PUT for *any* key (overwrite anyone's objects). `/files/url`, `/videos/url` and `GET /videos/:id` hand out presigned GETs for any key, including originals of private videos. Client-supplied `thumbnail` and `avatar` keys aren't checked against the caller's prefix.

- `VideoMetadataRepo.GetByKey` returns the metadata id, not the video id. The worker only works because the presign step caches key→video id for 24h.
- `confirm` has no "phase 1 finished" guard (see above). The original `/videos/search` doesn't filter by visibility.
- No tests. `users.subscribers` column is unused (counts come from `subscriptions`).
