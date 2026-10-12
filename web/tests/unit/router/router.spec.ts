import { beforeEach, describe, expect, it, vi } from "vitest";

// keep page modules out of the router test: only routing behaviour matters here
vi.mock("@/layouts/Mainlayout.vue", () => ({ default: { name: "MainLayout", render: () => null } }));
vi.mock("@/pages/Home.vue", () => ({ default: { name: "Home", render: () => null } }));
vi.mock("@/pages/404.vue", () => ({ default: { name: "NotFound", render: () => null } }));
for (const p of ["Results", "Watch", "Channel", "Subscriptions", "Liked", "Studio", "UploadVideo"]) {
  vi.doMock(`@/pages/${p}.vue`, () => ({ default: { name: p, render: () => null } }));
}

const freshRouter = async () => {
  vi.resetModules();
  const auth = await import("@/auth/keycloak");
  const router = (await import("@/router")).default;
  await router.push("/");
  await router.isReady();
  return { router, auth };
};

describe("router", () => {
  beforeEach(() => {
    window.history.replaceState({}, "", "/");
  });

  it("public routes resolve without login", async () => {
    const { router, auth } = await freshRouter();
    await router.push("/results?q=cats");
    expect(router.currentRoute.value.name).toBe("Results");
    await router.push("/watch/abc");
    expect(router.currentRoute.value.name).toBe("Watch");
    expect(router.currentRoute.value.meta.immersive).toBe(true);
    await router.push("/channel/c1");
    expect(router.currentRoute.value.params.channelId).toBe("c1");
    expect(auth.login).not.toHaveBeenCalled();
  });

  it.each(["/studio", "/upload", "/liked", "/subscriptions"])("requiresAuth %s calls login(fullPath) and aborts when signed out", async (path) => {
    const { router, auth } = await freshRouter();
    auth.isAuthenticated.value = false;
    await router.push(`${path}?x=1`);
    expect(auth.login).toHaveBeenCalledWith(`${path}?x=1`);
    expect(router.currentRoute.value.fullPath).toBe("/");
  });

  it("requiresAuth routes open when signed in", async () => {
    const { router, auth } = await freshRouter();
    auth.isAuthenticated.value = true;
    await router.push("/studio");
    expect(router.currentRoute.value.name).toBe("Studio");
    expect(auth.login).not.toHaveBeenCalled();
  });

  it("/auth/login starts the Keycloak login and lands on /", async () => {
    const { router, auth } = await freshRouter();
    await router.push("/results");
    await router.push("/auth/login");
    expect(auth.login).toHaveBeenCalledWith("/");
    expect(router.currentRoute.value.fullPath).toBe("/");
  });

  it("/auth/signup starts Keycloak registration and lands on /", async () => {
    const { router, auth } = await freshRouter();
    await router.push("/results");
    await router.push("/auth/signup");
    expect(auth.register).toHaveBeenCalledWith("/");
    expect(router.currentRoute.value.fullPath).toBe("/");
  });

  it("unknown paths hit NotFound", async () => {
    const { router } = await freshRouter();
    await router.push("/nope/really");
    expect(router.currentRoute.value.name).toBe("NotFound");
  });

  it("scrollBehavior restores saved positions, otherwise scrolls to top", async () => {
    const { router } = await freshRouter();
    const sb = router.options.scrollBehavior!;
    const r = router.currentRoute.value;
    expect(sb(r, r, { left: 0, top: 120 })).toEqual({ left: 0, top: 120 });
    expect(sb(r, r, null)).toEqual({ top: 0 });
  });
});
