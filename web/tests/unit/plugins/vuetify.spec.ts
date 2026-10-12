import { beforeEach, describe, expect, it, vi } from "vitest";

describe("plugins/vuetify", { timeout: 30_000 }, () => {
  beforeEach(() => vi.resetModules());

  it("defaults to the dark theme", async () => {
    const { vuetify } = await import("@/plugins/vuetify");
    expect(vuetify.theme.global.name.value).toBe("myTubeDark");
    expect(Object.keys(vuetify.theme.themes.value)).toEqual(expect.arrayContaining(["myTubeDark", "myTubeLight"]));
  });

  it("restores a persisted light theme", async () => {
    localStorage.setItem("mytube-theme", "myTubeLight");
    const { vuetify } = await import("@/plugins/vuetify");
    expect(vuetify.theme.global.name.value).toBe("myTubeLight");
  });

  it("ignores unknown persisted values", async () => {
    localStorage.setItem("mytube-theme", "neon");
    const { vuetify } = await import("@/plugins/vuetify");
    expect(vuetify.theme.global.name.value).toBe("myTubeDark");
  });
});
