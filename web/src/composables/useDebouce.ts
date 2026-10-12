import { isRef, onScopeDispose, ref, watch, type Ref } from "vue";

export function useDebouncedRef<T>(initialValue: T | Ref<T>, delay = 300) {
  const state = (isRef(initialValue) ? initialValue : ref(initialValue)) as Ref<T>;
  const debounced = ref(state.value) as Ref<T>;

  let timeout: ReturnType<typeof setTimeout> | null = null;

  watch(state, (val) => {
    if (timeout) clearTimeout(timeout);
    timeout = setTimeout(() => {
      debounced.value = val;
    }, delay);
  });

  onScopeDispose(() => {
    if (timeout) clearTimeout(timeout);
  });

  return { state, debounced };
}
