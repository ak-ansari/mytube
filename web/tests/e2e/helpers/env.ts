import { execSync } from "node:child_process";

export const AUTH_FILE = "tests/e2e/.auth/demo.json";
export const API_URL = process.env.E2E_API_URL ?? "http://localhost:8080";
export const KEYCLOAK_URL = process.env.E2E_KEYCLOAK_URL ?? "http://localhost:8180";
export const REALM = "mytube";
export const CLIENT_ID = "mytube-frontend";
export const DEMO = { username: process.env.E2E_USER ?? "demo", password: process.env.E2E_PASSWORD ?? "demo123" };

/** Matches any Keycloak login page URL for the realm. */
export const KEYCLOAK_LOGIN_URL = new RegExp(`${KEYCLOAK_URL.replace(/[.:/]/g, "\\$&")}/realms/${REALM}/protocol/openid-connect/auth`);

/** True when a `go run ./cmd/worker` (or built worker binary) process is alive. */
export const isWorkerRunning = (): boolean => {
  if (process.env.E2E_WORKER === "1") return true;
  if (process.env.E2E_WORKER === "0") return false;
  try {
    const out = execSync("ps ax -o command", { encoding: "utf8" });
    return out.split("\n").some((l) => /cmd\/worker|\/worker(\s|$)/.test(l) && !/grep/.test(l));
  } catch {
    return false;
  }
};

/** Unique, recognisable marker for data created by the e2e suite. */
export const e2eTag = (what: string) => `e2e ${what} ${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
