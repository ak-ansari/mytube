import { expect, type Page } from "@playwright/test";
import { DEMO, KEYCLOAK_LOGIN_URL } from "../helpers/env";

/** Keycloak's login page (realm theme `mytube`, based on keycloak.v2 / PatternFly 5). */
export class KeycloakLoginPage {
  constructor(private readonly page: Page) {}

  readonly username = () => this.page.getByLabel("Username or email");
  readonly password = () => this.page.getByLabel("Password", { exact: true });
  readonly submit = () => this.page.getByRole("button", { name: "Sign in" });

  async expectVisible() {
    await this.page.waitForURL(KEYCLOAK_LOGIN_URL);
    await expect(this.username()).toBeVisible();
  }

  /** Fills the form and waits until Keycloak sends us back to the app. */
  async login(user = DEMO.username, pass = DEMO.password) {
    await this.expectVisible();
    await this.username().fill(user);
    await this.password().fill(pass);
    await this.submit().click();
    await this.page.waitForURL((url) => !KEYCLOAK_LOGIN_URL.test(url.href) && url.port !== "8180");
  }
}
