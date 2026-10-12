import { API_URL, CLIENT_ID, DEMO, KEYCLOAK_URL, REALM } from "./env";

/**
 * Thin API client for test setup / verification / cleanup. Uses a Keycloak direct-grant token
 * (the realm's public client has direct access grants enabled for exactly this kind of use).
 */
type Envelope<T> = { data: T; message: string; success: boolean };
export type Page<T> = { items: T[]; page: number; size: number; has_more: boolean };
export type Channel = { id: string; user_name: string; channel_name?: string | null; is_subscribed?: boolean; subscribers?: number };
export type VideoCard = { id: string; title: string; status: string; visibility?: string; channel: Channel };
export type VideoDetails = VideoCard & {
  likes: number;
  dislikes: number;
  my_reaction: number;
  comment_count: number;
  is_subscribed: boolean;
  manifest_url?: string;
};
export type Comment = { id: string; content: string; author: Channel };

let cached: { token: string; exp: number } | null = null;

export const getToken = async (): Promise<string> => {
  if (cached && cached.exp > Date.now() + 15_000) return cached.token;
  const res = await fetch(`${KEYCLOAK_URL}/realms/${REALM}/protocol/openid-connect/token`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({ grant_type: "password", client_id: CLIENT_ID, username: DEMO.username, password: DEMO.password }),
  });
  if (!res.ok) throw new Error(`Keycloak direct grant failed: ${res.status} ${await res.text()}`);
  const body = (await res.json()) as { access_token: string; expires_in: number };
  cached = { token: body.access_token, exp: Date.now() + body.expires_in * 1000 };
  return cached.token;
};

const call = async <T>(method: string, path: string, opts: { auth?: boolean; body?: unknown } = {}): Promise<T> => {
  const headers: Record<string, string> = {};
  if (opts.auth !== false) headers.Authorization = `Bearer ${await getToken()}`;
  if (opts.body !== undefined) headers["Content-Type"] = "application/json";
  const res = await fetch(`${API_URL}/api${path}`, {
    method,
    headers,
    body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
  });
  const text = await res.text();
  if (!res.ok) throw Object.assign(new Error(`${method} ${path} -> ${res.status}: ${text}`), { status: res.status });
  return text ? (JSON.parse(text) as Envelope<T>).data : (undefined as T);
};

export const api = {
  me: () => call<Channel>("GET", "/users/me"),
  browse: (search = "", size = 50) =>
    call<Page<VideoCard>>("GET", `/videos/browse?size=${size}${search ? `&search=${encodeURIComponent(search)}` : ""}`, { auth: false }),
  suggest: (search: string) => call<string[]>("GET", `/videos/suggest?search=${encodeURIComponent(search)}`, { auth: false }),
  details: (id: string) => call<VideoDetails>("GET", `/videos/${id}/details`),
  mine: () => call<Page<VideoCard>>("GET", "/videos/mine?size=50"),
  comments: (id: string) => call<Page<Comment>>("GET", `/videos/${id}/comments?size=50`, { auth: false }),
  deleteComment: (videoId: string, commentId: string) => call<unknown>("DELETE", `/videos/${videoId}/comments/${commentId}`),
  react: (id: string, value: -1 | 0 | 1) => call<unknown>("PUT", `/videos/${id}/reaction`, { body: { value } }),
  channel: (id: string) => call<Channel>("GET", `/channels/${id}`),
  subscribe: (id: string) => call<Channel>("POST", `/channels/${id}/subscribe`),
  unsubscribe: (id: string) => call<Channel>("DELETE", `/channels/${id}/subscribe`),
  deleteVideo: (id: string) => call<unknown>("DELETE", `/videos/${id}`),
};

/** Deletes a video if it still exists; ignores 404s. */
export const deleteVideoQuietly = async (id: string) => {
  try {
    await api.deleteVideo(id);
  } catch (err) {
    if ((err as { status?: number }).status !== 404) console.warn(`cleanup: could not delete video ${id}:`, err);
  }
};

/** A published public video that is NOT owned by the demo user (for like / subscribe tests). */
export const findForeignVideo = async (): Promise<VideoCard | undefined> => {
  const me = await api.me();
  const feed = await api.browse();
  return feed.items.find((v) => v.channel.id !== me.id);
};
