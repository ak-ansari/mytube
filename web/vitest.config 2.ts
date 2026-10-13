import { defineConfig, mergeConfig } from "vitest/config";
import viteConfig from "./vite.config";

export default mergeConfig(
  viteConfig,
  defineConfig({
    test: {
      environment: "jsdom",
      include: ["tests/unit/**/*.spec.ts", "tests/component/**/*.spec.ts"],
      exclude: ["tests/e2e/**", "node_modules/**", "dist/**"],
      setupFiles: ["tests/setup.ts"],
      // Vuetify ships ESM that imports its own CSS; inline it so Vite's (stubbed) CSS handling applies
      server: { deps: { inline: ["vuetify"] } },
      css: { include: [] },
      env: {
        VITE_API_BASE_URL: "http://api.test",
        VITE_KEYCLOAK_URL: "http://keycloak.test",
        VITE_KEYCLOAK_REALM: "mytube",
        VITE_KEYCLOAK_CLIENT_ID: "mytube-frontend",
      },
      restoreMocks: false,
      coverage: {
        provider: "v8",
        include: ["src/**/*.{ts,vue}"],
        exclude: ["src/main.ts", "src/vite-env.d.ts", "src/types/**"],
        reporter: ["text-summary", "text", "html"],
        reportsDirectory: "coverage",
      },
    },
  })
);
