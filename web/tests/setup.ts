import { afterEach, beforeEach, vi } from "vitest";
import ResizeObserverPolyfill from "resize-observer-polyfill";
import { MockIntersectionObserver } from "./helpers/intersection";

// ---- env: make sure VITE_* values exist even if .env is not loaded
const env = import.meta.env as unknown as Record<string, string>;
env.VITE_API_BASE_URL ||= "http://api.test";
env.VITE_KEYCLOAK_URL ||= "http://keycloak.test";
env.VITE_KEYCLOAK_REALM ||= "mytube";
env.VITE_KEYCLOAK_CLIENT_ID ||= "mytube-frontend";

// ---- browser APIs jsdom lacks
globalThis.ResizeObserver ||= ResizeObserverPolyfill as unknown as typeof ResizeObserver;
globalThis.IntersectionObserver = MockIntersectionObserver as unknown as typeof IntersectionObserver;
if (!window.matchMedia) {
  window.matchMedia = (query: string) =>
    ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }) as unknown as MediaQueryList;
}
if (!window.visualViewport) {
  const vv = Object.assign(new EventTarget(), {
    width: window.innerWidth,
    height: window.innerHeight,
    offsetLeft: 0,
    offsetTop: 0,
    pageLeft: 0,
    pageTop: 0,
    scale: 1,
    onresize: null,
    onscroll: null,
  });
  Object.defineProperty(window, "visualViewport", { value: vv, configurable: true });
}
window.scrollTo = (() => {}) as typeof window.scrollTo;
Element.prototype.scrollTo ||= function () {} as typeof Element.prototype.scrollTo;
Element.prototype.scrollIntoView ||= function () {};
if (!URL.createObjectURL) URL.createObjectURL = () => "blob:mock";
if (!URL.revokeObjectURL) URL.revokeObjectURL = () => {};

// ---- keycloak: never touch the real adapter
vi.mock("keycloak-js", () => ({
  default: vi.fn().mockImplementation(() => ({
    init: vi.fn().mockResolvedValue(false),
    login: vi.fn().mockResolvedValue(undefined),
    register: vi.fn().mockResolvedValue(undefined),
    logout: vi.fn().mockResolvedValue(undefined),
    updateToken: vi.fn().mockResolvedValue(true),
    clearToken: vi.fn(),
    createAccountUrl: vi.fn(() => "http://keycloak.test/account"),
    authenticated: false,
    token: undefined,
  })),
}));

vi.mock("@/auth/keycloak", async () => {
  const { ref } = await import("vue");
  return {
    keycloak: {},
    isAuthenticated: ref(false),
    initAuth: vi.fn().mockResolvedValue(undefined),
    getToken: vi.fn().mockResolvedValue(undefined),
    login: vi.fn().mockResolvedValue(undefined),
    register: vi.fn().mockResolvedValue(undefined),
    logout: vi.fn().mockResolvedValue(undefined),
    accountUrl: vi.fn(() => "http://keycloak.test/account"),
  };
});

// ---- toasts: keep the real notivue exports but spy on push
vi.mock("notivue", async (importOriginal) => {
  const actual = await importOriginal<typeof import("notivue")>();
  const fn = () => vi.fn();
  return {
    ...actual,
    push: Object.assign(fn(), { success: fn(), error: fn(), info: fn(), warning: fn(), promise: fn(), load: fn(), clearAll: fn(), destroyAll: fn() }),
  };
});


beforeEach(async () => {
  const auth = await import("@/auth/keycloak");
  auth.isAuthenticated.value = false;
  MockIntersectionObserver.instances.length = 0;
  try {
    localStorage.clear();
  } catch {
    /* ignore */
  }
});

afterEach(() => {
  vi.clearAllMocks();
  vi.useRealTimers();
  document.body.innerHTML = "";
});
