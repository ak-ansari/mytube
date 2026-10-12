import { computed } from "vue";
import { useTheme } from "vuetify";

export const useThemeToggle = () => {
  const theme = useTheme();
  const isDark = computed(() => theme.global.current.value.dark);
  const toggle = () => {
    const next = isDark.value ? "myTubeLight" : "myTubeDark";
    theme.change(next);
    try {
      localStorage.setItem("mytube-theme", next);
    } catch {
      // storage unavailable (private mode); theme just won't persist
    }
  };
  return { isDark, toggle };
};
