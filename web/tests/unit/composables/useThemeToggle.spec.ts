import { afterEach, describe, expect, it, vi } from "vitest";
import { useThemeToggle } from "@/composables/useThemeToggle";
import { withSetup } from "../../helpers/withSetup";
import { createTestVuetify } from "../../helpers/mount";

describe("useThemeToggle", () => {
  afterEach(() => vi.restoreAllMocks());

  it("toggles dark <-> light and persists the choice", () => {
    const vuetify = createTestVuetify("myTubeDark");
    const { result } = withSetup(() => useThemeToggle(), { plugins: [vuetify] });
    expect(result.isDark.value).toBe(true);

    result.toggle();
    expect(vuetify.theme.global.name.value).toBe("myTubeLight");
    expect(result.isDark.value).toBe(false);
    expect(localStorage.getItem("mytube-theme")).toBe("myTubeLight");

    result.toggle();
    expect(vuetify.theme.global.name.value).toBe("myTubeDark");
    expect(result.isDark.value).toBe(true);
    expect(localStorage.getItem("mytube-theme")).toBe("myTubeDark");
  });

  it("still switches theme when storage is unavailable", () => {
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("private mode");
    });
    const vuetify = createTestVuetify("myTubeLight");
    const { result } = withSetup(() => useThemeToggle(), { plugins: [vuetify] });
    expect(result.isDark.value).toBe(false);
    expect(() => result.toggle()).not.toThrow();
    expect(result.isDark.value).toBe(true);
  });
});
