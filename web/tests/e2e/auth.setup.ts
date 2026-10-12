import { test as setup } from "@playwright/test";
import { AUTH_FILE } from "./helpers/env";
import { AppShell } from "./pages/AppShell";
import { KeycloakLoginPage } from "./pages/KeycloakLoginPage";

/**
 * Logs in through the real (themed) Keycloak page and saves the Keycloak SSO cookies. keycloak-js
 * restores the session on every load via check-sso, so specs using this storageState start signed in.
 */
setup("authenticate as demo", async ({ page }) => {
  const shell = new AppShell(page);
  await page.goto("/");
  await shell.ready();
  await shell.signInButton().click();
  await new KeycloakLoginPage(page).login();
  await shell.expectSignedIn();
  await page.context().storageState({ path: AUTH_FILE });
});
