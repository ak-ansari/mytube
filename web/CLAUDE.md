# MyTube — Frontend (Vue 3)

Learning project: a bare-minimum YouTube clone UI. Backend is the sibling Go repo `../mytube` (see its `CLAUDE.md` for the API, auth, upload pipeline and storage layout).

## Commands

```bash
npm install
npm run dev      # Vite on http://localhost:5173 — the only origin the backend CORS and the Keycloak client allow
npm run build    # vue-tsc -b && vite build — this is the typecheck; must pass
npm run preview
npm test               # Vitest unit (tests/unit) + component (tests/component), jsdom; npm run test:coverage
npm run test:e2e       # Playwright (tests/e2e) against the running dev stack; logs in as demo via the real Keycloak page
```

No lint setup. `tsconfig.app.json` is strict with `noUnusedLocals`/`noUnusedParameters`, so unused imports/vars fail the build. Vite warns about Node 20.16 (wants 20.19+), but it still works.

Login and registration pages are Keycloak's, restyled by the `mytube` theme in `../mytube/deployments/keycloak/themes/mytube`. Change their look there, not here.

`.env`: `VITE_API_BASE_URL` (http://localhost:8080), `VITE_KEYCLOAK_URL` (http://localhost:8180), `VITE_KEYCLOAK_REALM` (mytube), `VITE_KEYCLOAK_CLIENT_ID` (mytube-frontend). Demo login: **demo / demo123**, or register on the Keycloak page.

## Stack

Vue 3 `<script setup lang="ts">`, Vite, Vuetify 3 (auto-import, `@mdi/font`), TanStack Vue Query, axios, keycloak-js, notivue (toasts), video.js (HLS), moment. Pinia is installed but there are no stores; server state lives in Vue Query, auth state in `src/auth/keycloak.ts`. Path alias `@/` → `src/`.

## Structure & data flow

```
auth/keycloak.ts               keycloak-js instance, initAuth (check-sso, PKCE), getToken, login/register/logout, reactive `isAuthenticated`
api/http.ts                    axios: adds Bearer token (refreshing it), unwraps res.data.data, rejects with backend body; `errorMessage(err)`
api/<domain>.service.ts        catalog (feed/watch/studio/engagement), user (profile/channels/subscriptions), video (upload presign/confirm), file (presign/PUT)
composables/<domain>/          *.keys.ts (query keys) · *.query.ts (useQuery / usePagedQuery) · *.mutation.ts (invalidate on success)
composables/usePagedQuery.ts   useInfiniteQuery over `{items, page, size, has_more}` → flattened `items`
composables/file/uploadObject  presign + PUT to MinIO under a prefix, returns the object key
components/video|channel|studio|common   VideoCard (grid/list/compact), VideoFeed, VideoThumbnail, CommentsSection, ChannelAvatar, SubscribeButton, dialogs, LoadMore (IntersectionObserver), EmptyState
pages/                         Home, Results (?q=), Watch, Channel (?edit=1 opens customize), Subscriptions, Liked, Studio, UploadVideo, 404
layouts/Mainlayout.vue         app bar + nav drawer (permanent with rail toggle on lg+, overlay on smaller screens and on the watch page)
plugins/vuetify.ts             myTubeDark (default) / myTubeLight themes + component defaults; theme persisted via useThemeToggle
utils/format.ts                formatCount/Views/Subscribers/Duration, timeAgo, channelName, videoStatusInfo, visibilityIcon
```

- `main.ts` awaits `initAuth()` before mounting, so the router guard knows the session. `index.html` shows a splash meanwhile. Routes with `meta.requiresAuth` redirect to the Keycloak login. `/auth/login` and `/auth/signup` jump straight to Keycloak.
- Reads are public. For actions that need login (like, comment, subscribe), call `login()` when `!isAuthenticated.value` instead of failing.
- A global `MutationCache.onError` toasts the backend `message` for every failed mutation, so don't add per-mutation error toasts.
- The API returns presigned `thumbnail_url` / `avatar_url` / `manifest_url`; render those directly. Uploads (video, thumbnail `thumbnails/{videoId}/…`, avatar `avatars/{userId}/…`) go straight to MinIO through presigned PUTs, and only the key goes to the API. The API only presigns/accepts JPEG/PNG/WebP/GIF images in those folders (yours only), so image pickers use `IMAGE_ACCEPT`/`isAcceptedImage` from `uploadObject.ts`.
- Upload flow (`pages/UploadVideo.vue`): presign → PUT with progress → poll `/videos/:id/details` until `status === valid` and the thumbnail exists → enable Publish → `confirm`. Never confirm earlier (backend pipeline limitation). Studio's "Finish" on a draft uses the same confirm. The video PUT is abortable: Discard, leaving the route (after the confirm prompt) or unmounting mid-upload aborts it and best-effort deletes the draft; a failed upload deletes it too. A `beforeunload` prompt guards closing the tab while uploading.
- `videoStatusInfo` treats `upload_pending`/`uploaded` drafts whose `updated_at` is over 15 minutes old as "Upload incomplete" (not processing), so Studio stops its 4s polling for them; they can still be deleted.
- Playback: `VideoPlayer` (video.js) with `application/x-mpegURL` for the HLS master playlist. The view is recorded on first `play`.

## Conventions

- New backend domain → `api/<domain>.service.ts` + `composables/<domain>/{keys,query,mutation}.ts` + types in `types/` (snake_case, mirror the Go JSON).
- Use Vuetify components and theme tokens (`bg-background`, `surface-bright`, `color="accent"`, `.text-muted`). Avoid hardcoded hex values; both themes must look right. Buttons are pill-shaped by default.
- Shared layout CSS lives in `style.css` (`.page`, `.video-grid`, `.page-title`, `.line-clamp-*`).
- `vite.config.ts` pre-scans all `.vue` files (`optimizeDeps.entries`). Without that, the first visit to a lazy route triggers a dev-server full reload.

## Notes

`components/VideoPlayer.vue` is the only player. `composables/video/` only holds `useSuggestSearch` (search-bar suggestions); `api/video.service.ts` is just the upload presign, suggestions and confirm. Everything else goes through `catalog.service`.
