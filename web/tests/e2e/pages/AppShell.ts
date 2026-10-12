import { expect, type Page } from "@playwright/test";

/** App bar + navigation drawer (layouts/Mainlayout.vue, components/Appbar.vue, components/Sidemenu.vue). */
export class AppShell {
  constructor(readonly page: Page) {}

  readonly banner = () => this.page.getByRole("banner");
  readonly menuButton = () => this.page.getByRole("button", { name: "Menu", exact: true });
  readonly signInButton = () => this.banner().getByRole("button", { name: "Sign in" });
  readonly accountMenuButton = () => this.page.getByRole("button", { name: "Account menu" });
  readonly themeToggle = () => this.page.getByRole("button", { name: /^(Light|Dark) theme$/ });
  readonly drawer = () => this.page.locator("nav.v-navigation-drawer");
  readonly searchInput = () => this.page.getByRole("search").getByPlaceholder("Search");
  readonly searchSubmit = () => this.page.getByRole("search").getByRole("button", { name: "Search" });
  readonly suggestions = () => this.page.locator(".suggestions .suggestion");

  /** Waits until the app (not the index.html splash) is mounted and auth is resolved. */
  async ready() {
    await expect(this.menuButton()).toBeVisible({ timeout: 20_000 });
  }

  async expectSignedIn() {
    await expect(this.accountMenuButton()).toBeVisible({ timeout: 20_000 });
    await expect(this.signInButton()).toHaveCount(0);
  }

  async expectSignedOut() {
    await expect(this.signInButton()).toBeVisible({ timeout: 20_000 });
    await expect(this.accountMenuButton()).toHaveCount(0);
  }

  async openAccountMenu() {
    await this.accountMenuButton().click();
    const menu = this.page.locator(".v-overlay--active .v-card").filter({ hasText: "Sign out" });
    await expect(menu).toBeVisible();
    return menu;
  }

  async signOut() {
    const menu = await this.openAccountMenu();
    await menu.getByText("Sign out").click();
  }

  /** "dark" | "light", read from the Vuetify theme class on the app root. */
  async currentTheme(): Promise<"dark" | "light"> {
    const cls = (await this.page.locator(".v-application").first().getAttribute("class")) ?? "";
    return cls.includes("v-theme--myTubeLight") ? "light" : "dark";
  }

  async search(term: string) {
    await this.searchInput().fill(term);
    await this.searchInput().press("Enter");
  }
}
