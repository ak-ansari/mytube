import { beforeEach, describe, expect, it, vi } from "vitest";
import { AxiosError, AxiosHeaders, type AxiosAdapter, type InternalAxiosRequestConfig } from "axios";
import { http, errorMessage } from "@/api/http";
import { getToken, isAuthenticated, login } from "@/auth/keycloak";

const ok =
  (body: unknown, status = 200): AxiosAdapter =>
  async (config) => ({ data: body, status, statusText: "OK", headers: {}, config });

const fail =
  (body: unknown, status: number): AxiosAdapter =>
  async (config) => {
    const response = { data: body, status, statusText: "ERR", headers: {}, config };
    throw new AxiosError("Request failed", "ERR_BAD_RESPONSE", config, null, response);
  };

let adapter: ReturnType<typeof vi.fn>;
const useAdapter = (a: AxiosAdapter) => {
  adapter = vi.fn(a);
  http.defaults.adapter = adapter as unknown as AxiosAdapter;
};
const lastConfig = () => adapter.mock.calls.at(-1)![0] as InternalAxiosRequestConfig;

describe("http client", () => {
  beforeEach(() => {
    useAdapter(ok({ success: true, data: { hello: "world" } }));
  });

  it("uses the API base URL from env and a timeout", () => {
    expect(http.defaults.baseURL).toBe(import.meta.env.VITE_API_BASE_URL);
    expect(http.defaults.timeout).toBe(10000);
  });

  it("unwraps res.data.data", async () => {
    await expect(http.get("/api/x")).resolves.toEqual({ hello: "world" });
  });

  it("resolves undefined when the envelope has no data", async () => {
    useAdapter(ok({ success: true, message: "ok" }));
    await expect(http.post("/api/x")).resolves.toBeUndefined();
  });

  it("adds no Authorization header when signed out", async () => {
    vi.mocked(getToken).mockResolvedValueOnce(undefined);
    await http.get("/api/x");
    expect(getToken).toHaveBeenCalledTimes(1);
    expect(AxiosHeaders.from(lastConfig().headers).get("Authorization")).toBeUndefined();
  });

  it("adds a fresh Bearer token on every request (getToken refreshes it)", async () => {
    vi.mocked(getToken).mockResolvedValueOnce("token-1").mockResolvedValueOnce("token-2");
    await http.get("/api/a");
    expect(lastConfig().headers.Authorization).toBe("Bearer token-1");
    await http.get("/api/b");
    expect(lastConfig().headers.Authorization).toBe("Bearer token-2");
    expect(getToken).toHaveBeenCalledTimes(2);
  });

  it("rejects with the backend body on errors", async () => {
    const body = { success: false, message: "Title is required", status: 400 };
    useAdapter(fail(body, 400));
    await expect(http.post("/api/x", {})).rejects.toEqual(body);
  });

  it("rejects with the axios error when there is no response (network down)", async () => {
    useAdapter(async (config) => {
      throw new AxiosError("Network Error", "ERR_NETWORK", config);
    });
    const err = await http.get("/api/x").catch((e) => e);
    expect(err).toBeInstanceOf(AxiosError);
    expect(err.message).toBe("Network Error");
  });

  it("401 while signed out does not start a login", async () => {
    isAuthenticated.value = false;
    useAdapter(fail({ message: "unauthorized" }, 401));
    await http.get("/api/x").catch(() => undefined);
    expect(login).not.toHaveBeenCalled();
  });

  it("401 while signed in redirects to login only once for concurrent failures", async () => {
    isAuthenticated.value = true;
    useAdapter(fail({ message: "unauthorized" }, 401));
    await Promise.all([http.get("/api/a").catch(() => 0), http.get("/api/b").catch(() => 0), http.get("/api/c").catch(() => 0)]);
    expect(login).toHaveBeenCalledTimes(1);
  });

  it("non-401 errors never trigger login", async () => {
    isAuthenticated.value = true;
    useAdapter(fail({ message: "forbidden" }, 403));
    await http.get("/api/x").catch(() => undefined);
    expect(login).not.toHaveBeenCalled();
  });
});

describe("http 401 retry after a failed login redirect", () => {
  it("allows another redirect when login() itself fails", async () => {
    vi.resetModules();
    const { http: fresh } = await import("@/api/http");
    const auth = await import("@/auth/keycloak");
    auth.isAuthenticated.value = true;
    vi.mocked(auth.login).mockRejectedValueOnce(new Error("keycloak down"));
    fresh.defaults.adapter = fail({ message: "unauthorized" }, 401);
    await fresh.get("/a").catch(() => 0);
    await new Promise((r) => setTimeout(r, 0));
    await fresh.get("/b").catch(() => 0);
    expect(auth.login).toHaveBeenCalledTimes(2);
  });
});

describe("errorMessage", () => {
  it("returns the backend message", () => {
    expect(errorMessage({ message: "Video not found" })).toBe("Video not found");
    expect(errorMessage(new Error("boom"))).toBe("boom");
  });
  it("falls back for empty or missing messages", () => {
    expect(errorMessage(undefined)).toBe("Something went wrong");
    expect(errorMessage(null)).toBe("Something went wrong");
    expect(errorMessage("a string")).toBe("Something went wrong");
    expect(errorMessage({ message: "" })).toBe("Something went wrong");
    expect(errorMessage({ message: 42 })).toBe("Something went wrong");
    expect(errorMessage({}, "Upload failed")).toBe("Upload failed");
  });
});
