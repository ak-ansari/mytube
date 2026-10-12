import { beforeEach, describe, expect, it, vi } from "vitest";

// exercise the real auth module (the global setup mocks it for everyone else); keycloak-js stays mocked
vi.unmock("@/auth/keycloak");

import * as auth from "@/auth/keycloak";

type KcMock = {
  init: ReturnType<typeof vi.fn>;
  login: ReturnType<typeof vi.fn>;
  register: ReturnType<typeof vi.fn>;
  logout: ReturnType<typeof vi.fn>;
  updateToken: ReturnType<typeof vi.fn>;
  clearToken: ReturnType<typeof vi.fn>;
  createAccountUrl: ReturnType<typeof vi.fn>;
  authenticated?: boolean;
  token?: string;
  onAuthSuccess?: () => void;
  onAuthLogout?: () => void;
  onTokenExpired?: () => void;
};
const kc = auth.keycloak as unknown as KcMock;

beforeEach(() => {
  kc.authenticated = false;
  kc.token = undefined;
  kc.updateToken.mockResolvedValue(true);
  kc.init.mockResolvedValue(false);
  auth.isAuthenticated.value = false;
});

describe("auth/keycloak", () => {
  it("initAuth runs check-sso with PKCE and mirrors the session", async () => {
    kc.init.mockImplementation(async () => {
      kc.authenticated = true;
      return true;
    });
    await auth.initAuth();
    expect(kc.init).toHaveBeenCalledWith(expect.objectContaining({ onLoad: "check-sso", pkceMethod: "S256", checkLoginIframe: false }));
    expect(auth.isAuthenticated.value).toBe(true);
  });

  it("initAuth never throws when Keycloak is down", async () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    kc.init.mockRejectedValue(new Error("unreachable"));
    await expect(auth.initAuth()).resolves.toBeUndefined();
    expect(auth.isAuthenticated.value).toBe(false);
    warn.mockRestore();
  });

  it("getToken returns undefined when signed out without refreshing", async () => {
    await expect(auth.getToken()).resolves.toBeUndefined();
    expect(kc.updateToken).not.toHaveBeenCalled();
  });

  it("getToken refreshes tokens close to expiry (30s) and returns the current token", async () => {
    kc.authenticated = true;
    kc.updateToken.mockImplementation(async () => {
      kc.token = "fresh-token";
      return true;
    });
    await expect(auth.getToken()).resolves.toBe("fresh-token");
    expect(kc.updateToken).toHaveBeenCalledWith(30);
  });

  it("getToken clears the session when the refresh fails", async () => {
    kc.authenticated = true;
    kc.token = "stale";
    kc.updateToken.mockRejectedValue(new Error("refresh failed"));
    await expect(auth.getToken()).resolves.toBeUndefined();
    expect(kc.clearToken).toHaveBeenCalled();
  });

  it("event hooks keep isAuthenticated in sync and refresh expired tokens", async () => {
    kc.onAuthSuccess!();
    expect(auth.isAuthenticated.value).toBe(true);
    kc.onAuthLogout!();
    expect(auth.isAuthenticated.value).toBe(false);
    kc.updateToken.mockRejectedValueOnce(new Error("x"));
    kc.onTokenExpired!();
    await new Promise((r) => setTimeout(r, 0));
    expect(kc.updateToken).toHaveBeenCalledWith(30);
    expect(kc.clearToken).toHaveBeenCalled();
  });

  it("login / register / logout redirect back to absolute app URLs", () => {
    const origin = window.location.origin;
    auth.login("/studio");
    expect(kc.login).toHaveBeenLastCalledWith({ redirectUri: `${origin}/studio` });
    auth.login();
    expect(kc.login).toHaveBeenLastCalledWith({ redirectUri: `${origin}${window.location.pathname}${window.location.search}` });
    auth.register();
    expect(kc.register).toHaveBeenCalledWith({ redirectUri: `${origin}/` });
    auth.logout();
    expect(kc.logout).toHaveBeenCalledWith({ redirectUri: `${origin}/` });
    expect(auth.accountUrl()).toBe("http://keycloak.test/account");
  });
});
