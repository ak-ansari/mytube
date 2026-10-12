import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { effectScope, nextTick, ref } from "vue";
import { useDebouncedRef } from "@/composables/useDebouce";

const scoped = <T>(fn: () => T) => effectScope().run(fn)!;

describe("useDebouncedRef", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("starts with the initial value in both refs", () => {
    const { state, debounced } = scoped(() => useDebouncedRef("x"));
    expect(state.value).toBe("x");
    expect(debounced.value).toBe("x");
  });

  it("updates the debounced value only after the delay", async () => {
    const { state, debounced } = scoped(() => useDebouncedRef("", 300));
    state.value = "cat";
    await nextTick();
    vi.advanceTimersByTime(299);
    expect(debounced.value).toBe("");
    vi.advanceTimersByTime(1);
    expect(debounced.value).toBe("cat");
  });

  it("only applies the last value of a burst", async () => {
    const { state, debounced } = scoped(() => useDebouncedRef("", 200));
    for (const v of ["c", "ca", "cat"]) {
      state.value = v;
      await nextTick();
      vi.advanceTimersByTime(100);
    }
    expect(debounced.value).toBe("");
    vi.advanceTimersByTime(200);
    expect(debounced.value).toBe("cat");
  });

  it("defaults to 300ms and reuses a passed-in ref as state", async () => {
    const source = ref(1);
    const { state, debounced } = scoped(() => useDebouncedRef(source));
    expect(state).toBe(source);
    source.value = 2;
    await nextTick();
    vi.advanceTimersByTime(299);
    expect(debounced.value).toBe(1);
    vi.advanceTimersByTime(1);
    expect(debounced.value).toBe(2);
  });

  it("cancels the pending update when the scope is disposed", async () => {
    const scope = effectScope();
    const r = scope.run(() => useDebouncedRef("a", 100))!;
    r.state.value = "b";
    await nextTick();
    scope.stop();
    vi.advanceTimersByTime(500);
    expect(r.debounced.value).toBe("a");
  });
});
