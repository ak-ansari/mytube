import { defineComponent, h } from "vue";
import { mount } from "@vue/test-utils";
import { VueQueryPlugin, type QueryClient } from "@tanstack/vue-query";
import { createTestQueryClient } from "./mount";

/** Runs a composable inside a mounted component (so inject()/lifecycle work) with a fresh QueryClient. */
export function withSetup<T>(composable: () => T, opts: { queryClient?: QueryClient; plugins?: unknown[] } = {}) {
  const queryClient = opts.queryClient ?? createTestQueryClient();
  let result!: T;
  const Comp = defineComponent({
    setup() {
      result = composable();
      return () => h("div");
    },
  });
  const wrapper = mount(Comp, {
    global: { plugins: [[VueQueryPlugin, { queryClient }], ...(opts.plugins ?? [])] as never[] },
  });
  return { result, wrapper, queryClient };
}
