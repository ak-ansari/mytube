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
make obs-up        # observability: Grafana, Prometheus, Loki, Alloy, postgres/redis exporters (see "Observability")
make keycloak-dev  # once per Keycloak volume: admin console over http + realm loginTheme=mytube
make run-api       # API on :8080
make run-worker    # pipeline worker (needs ffmpeg/ffprobe on PATH)
make down
go build ./... && go vet ./...   # compile check
go test ./...                                         # unit tests (fakes in internal/testutil, no Docker)
go test -tags integration -count=1 -p 1 ./...         # real Postgres/Redis/MinIO/ES; isolated mytube_test DB, mytube-test bucket, Redis DB 15
go test -tags e2e -count=1 -v -timeout 10m ./test/e2e/  # black-box against a running API + worker (E2E_API_URL, default :8080)
```

Ports: **HLS gateway 80** (nginx), Postgres 5432, Redis 6379, MinIO S3 API via nginx **9002** (presigned URLs point here), MinIO console 9001, ES 9200, Kibana 5601, **Keycloak 8180** (admin/admin). Observability: **Grafana 3000** (admin/admin, anonymous view), Prometheus 9090, Loki 3100, Alloy UI 12345, worker metrics 9091.

Gotchas:
- A Homebrew `redis-server` on `127.0.0.1:6379` shadows the Docker Redis. MinIO publishes upload events into the Docker Redis, so the worker never sees uploads (videos stay `upload_pending`). Stop the local Redis (`brew services stop redis`) or point `REDIS.REDIS_HOST` at the Docker one.
- The MinIO bucket needs an event rule sending `originals/` puts to Redis (`mc event add myminio/mytube arn:minio:sqs::primary:redis --event put,delete --prefix originals/`); it persists in the MinIO volume.

## Observability (logs + metrics)

`make obs-up` layers `deployments/docker-compose.observability.yml` on the app stack (same project, `make down` stops both). Grafana dashboard **MyTube → MyTube Overview** is provisioned from `deployments/observability/grafana/dashboards/` (UI edits are lost on restart; export the JSON there to keep them).

- **Logs:** with `OBSERVABILITY.LOG_DIR` set, `logger.NewZapLoggerWithFile` tees JSON lines (ANSI colours stripped, `service=api|worker`, rotated at 50 MB) to `logs/{api,worker}.log`; console output is unchanged. Alloy (`observability/alloy/config.alloy`) tails them into Loki with labels `job="mytube"`, `service`, `level`, and also ships every `mytube` container's stdout (`job="docker"`, `container`). `observability.HTTPMiddleware` writes one Info access-log line per request (`route`, `status`, `duration_ms`). Query e.g. `{service="api"} | json | status >= 500` or `{service="worker"} | json | videoId="…"`.
- **Metrics:** API serves `:8080/metrics`, worker `OBSERVABILITY.METRICS_ADDR` (`:9091`); Prometheus reaches both via `host.docker.internal`. App metrics (`internal/observability`): `mytube_http_requests_total{method,route,status}`, `mytube_http_request_duration_seconds`, `mytube_pipeline_stage_runs_total{stage,result}` + `_duration_seconds` (handlers wrapped by `HandlerRegistry.Instrument()`), pgx pool gauges, and worker-only gauges read from Postgres per scrape: `mytube_videos{stage,status}`, `mytube_dead_letter_jobs{source,stage}`. Infra: postgres-exporter, redis-exporter (queue depth via `redis_key_size{key=…}` for `video_queue`, `video_queue:delayed`, `minio-events-queue`), MinIO's own endpoint (`MINIO_PROMETHEUS_AUTH_TYPE=public`).
- Route labels use gin's template (`/api/videos/:id`), so ids never become labels. Keep it that way (no ids/user ids as labels).
- Gotcha: recreating `minio` gives it a new IP and nginx keeps the old one (502 on :9002). `docker restart nginx`.

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
logs/                    gitignored app JSON logs (OBSERVABILITY.LOG_DIR) tailed by Alloy
migrations/              goose SQL (applied on startup from ./migrations)
configs/                 config.example.yaml (+ your gitignored config.yaml)
deployments/             docker-compose.yml (project name pinned to "mytube"), docker-compose.observability.yml + observability/ (prometheus, loki, alloy, grafana provisioning), nginx/templates/ (envsubst-rendered), keycloak/ (import/ realm JSON, themes/mytube login theme)
docs/                    todo.txt
```

Run every command from the repo root. `make` passes `-f deployments/docker-compose.yml`.

**Config lives in two places that must agree:** `deployments/docker-compose.yml` holds the infra credentials as literal dev values (Postgres user/pass/db, MinIO root user/pass, MinIO event key, bucket name, nginx `VIDEO_TOKEN_SECRET`). `configs/config.yaml` holds the same values for the Go apps. The makefile repeats the MinIO credentials for `setup-bucket`. Change all three together.

## Architecture

- `cmd/api/main.go` — config, logger, pg pool (runs migrations), redis queue/cache, ES index, MinIO store, `VideoService` (upload pipeline), `UserService`, `VideoCatalogService`, Keycloak verifier → `api.SetupRouter`.
- `cmd/worker/main.go` — same deps + `workers.Runner` (2 goroutines polling bucket events and the job queue; shutdown waits for them via `Runner.Wait`).
  - **Retries + dead letters** (`Runner.WithRetry`, `workers/retry.go`). `IsRetryable` classifies a failure: `jobs.Permanent(err)`, a missing video (`pgx.ErrNoRows`/`ErrNotFound`), ffmpeg/ffprobe exit errors (unless the output mentions a network failure) and JSON errors are permanent; context, network, Postgres connection/serialization, MinIO 5xx/429 and unknown errors are retryable. A retryable stage failure is re-enqueued with `Attempt+1` after exponential backoff with jitter (Redis ZSET `<queue>:delayed`, moved back onto the list by `PromoteDueRetries` each iteration) until `WORKER.MAX_ATTEMPTS` (default 4) runs. A permanent failure, or one out of attempts, is written to `dead_letter_jobs` and the video is marked failed (`PipelineCoordinator.Fail` → `phase_1_failed` / `phase_2_failed`). An `Advance` error after a successful handler is retried too (handlers are idempotent). A stage interrupted by shutdown is re-queued unchanged. Bucket events retry in place: the event stays in the hash with backoff state in cache key `bucket_event_retry:<key>`, then is dead-lettered (`source = bucket_event`) and dropped. Panics are recovered per iteration; infrastructure errors (e.g. Redis down) pause the loop ~1s.
  - A job is lost if the worker process dies mid-stage (BLPOP removed it); the video then sits at its stage. Not handled yet.
  - `PipelineCoordinator.Advance` moves stages with `VideoRepository.AdvanceState` (`UPDATE … WHERE stage = <from>`), so concurrent callers advance a stage once and only the winner enqueues the next job.
- `internal/services/video_service.go` (+ `pipeline_coordinator.go`, `video_state_machine_service.go`) — original upload/processing logic.
- `internal/services/video_catalog_service.go` — feed/search/watch, studio CRUD, views, reactions, comments. `user_service.go` — profile, channels, subscriptions. `errors.go` — `ErrNotFound/ErrForbidden/ErrBadRequest` that handlers map to 404/403/400.
- `internal/repository/*.go` — interfaces; `internal/repository/postgres/*` — pgx implementations (raw SQL). `video_catalog_pg_repo.go` builds the joined "video card" query (videos + users + video_metadata) and owns `publishedVideoCond`.
- `internal/storage` — `ObjectStore` + MinIO impl. Keys: `originals/{videoId}/original.{ext}`, `thumbnails/{videoId}/…` (auto `{videoId}.jpeg`, custom uploads `{uuid}.{ext}`), `transcoded/{videoId}/{q}.mp4`, `segments/{videoId}/master.m3u8`, `avatars/{userId}/{uuid}.{ext}`. Bucket is private (see above).
- `internal/index/elasticsearch` — `video_index`; search returns IDs, rows then come from Postgres.
- `internal/models/catalog.go` — API shapes: `VideoCard`, `VideoDetails`, `Channel`, `Profile`, `Comment`, `Page[T]`. Services fill presigned `*_url` fields (`thumbnail_url`, `avatar_url`, `manifest_url`) so the client needs no extra calls.
- `migrations/` — goose SQL, auto-applied on startup. 004 adds `users.email/description`; 005 adds `videos.views`, `video_reactions`, `comments`, `subscriptions`; 006 drops the btree indexes on `videos.title/description` (they broke writes over ~2.7KB) and adds a unique index on `video_metadata(video_id)`; 007 adds `dead_letter_jobs` (no FK, so entries outlive deleted videos); 008 adds a trigger bumping `videos.updated_at` when `stage`/`status` change (not on views/edits). Next: `009_…sql`.

### Upload + processing flow

1. `POST /api/videos/upload/presign {file_name, size}` → inserts `videos` (stage 1 `upload_pending`) + `video_metadata`, returns `{id, key, url}`; client PUTs to MinIO.
2. MinIO → Redis hash → worker `ProcessBucketEvents` (resolves the key to a video via `VideoMetadataRepo.GetByKey`, cached 24h at presign) → `PipelineCoordinator.Advance`. Validate runs, then the thumbnail job (frame at `min(3s, duration/2)`); at the phase boundary `Advance` **waits until the video has a title** (status `valid`, thumbnail set).
3. `POST /api/videos/:id/confirm` saves title/desc/visibility/thumbnail to Postgres, then indexes into ES, and advances to phase 2 (transcode → HLS segments → checksum (stage 7 `checksum`) → publish → stage 8 `published`).
   Confirm returns 400 once the video is past phase 1 (stage > 3) or failed. It only calls `Advance` itself when phase 1 is finished (stage 3 `valid` and `video_metadata.thumbnail` set); otherwise the worker's `Advance` at the phase boundary sees the title and continues. `thumbnail` is optional (`""` keeps the auto-generated one) and must be a jpg/jpeg/png/webp/gif directly under `thumbnails/{videoId}/`. Title (3-100) and description (≤5000, trimmed) lengths count characters (runes), not bytes.
   Transcoding always writes MP4 renditions and skips qualities taller than the source (the smallest is always produced).

A video is publicly visible when `status = 'published' AND visibility = 'public'`; unlisted is watchable by link; owners can see everything.

### Endpoints (all under `/api`; 🔒 = login required)

| Method | Path | Notes |
|---|---|---|
| GET | `/videos/browse?search=&page=&size=` | home feed (latest) or ES search, published+public only |
| GET | `/videos/:id/details` | watch page payload (counts, my_reaction, is_subscribed, manifest_url) |
| GET | `/videos/:id/related` | other published videos |
| POST | `/videos/:id/view` | +1 view if the viewer may watch it, deduped per user/IP for 30 min (Redis SETNX; no proxies trusted, so `X-Forwarded-For` is ignored) |
| PUT 🔒 | `/videos/:id/reaction` `{value: 1\|-1\|0}` | like / dislike / clear |
| GET / POST 🔒 | `/videos/:id/comments` | list (paged) / add `{content}` |
| DELETE 🔒 | `/videos/:id/comments/:commentId` | comment author or video owner |
| GET 🔒 | `/videos/mine`, `/videos/liked`, `/videos/subscriptions` | studio list (all statuses), liked, subscription feed |
| PUT 🔒 | `/videos/:id` `{title?, description?, visibility?, thumbnail?}` | owner; also re-indexes ES |
| DELETE 🔒 | `/videos/:id` | owner; removes rows (cascade), ES doc and all object prefixes |
| POST 🔒 | `/videos/upload/presign`, `/videos/:id/confirm` | upload flow (confirm: owner only) |
| GET | `/videos/search`, `/videos/suggest`, `/videos/url`, `/videos/:id`, `/video-meta/:id` | original endpoints (search/suggest only return published+public videos; prefer `/browse`; `/videos/url` only signs thumbnail/avatar images; unknown id → 404) |
| GET 🔒 / PUT 🔒 | `/users/me` | profile; update `{channel_name?, description?, avatar?}` |
| GET 🔒 | `/users/me/subscriptions` | subscribed channels |
| GET | `/channels/:id`, `/channels/:id/videos` | public channel + its published videos |
| POST 🔒 / DELETE 🔒 | `/channels/:id/subscribe` | subscribe / unsubscribe |
| GET 🔒 / GET | `/files/presign?key=&size=`, `/files/url?key=` | `services.FileService`. Presigned PUT only for `avatars/{me}/{img}` or `thumbnails/{videoId}/{img}` of a video you own, 1 B–10 MB / presigned GET only for thumbnail and avatar images |

Paged responses are `{items, page, size, has_more}` (`size` capped at 50).

## Conventions

- Responses: `util.NewResponse(status, message, data, err)` → `{message, success, data, status, error}`; the frontend unwraps `data` and shows `message` on errors. New handlers use `respondOK` / `respondError` / `badRequest` / `uuidParam` / `paging` from `internal/api/handlers/respond.go`.
- Request DTOs: existing ones in `internal/api/dto`; new input structs live next to their service (`UpdateVideoInput`, `UpdateProfileInput`).
- Nullable DB columns scan into pointer fields; list queries fetch `size+1` rows and `models.NewPage` trims and sets `has_more`.
- Logging via `pkg/logger` (zap) in services/workers; handlers log unexpected errors in `respondError`.
- Module path: `github.com/ak-ansari/mytube`.

## Known gaps / later

- Image keys (thumbnails, avatars) are checked by extension only; the uploaded bytes aren't sniffed. A custom thumbnail key isn't checked to exist before confirm.
- Browse search sets `has_more` from the ES hit count, before Postgres drops unpublished/private hits, so a last page can come back empty.
- `users.subscribers` column is unused (counts come from `subscriptions`).
