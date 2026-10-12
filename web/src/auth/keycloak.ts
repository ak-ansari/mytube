import Keycloak from "keycloak-js";
import { ref } from "vue";

export const keycloak = new Keycloak({
  url: import.meta.env.VITE_KEYCLOAK_URL,
  realm: import.meta.env.VITE_KEYCLOAK_REALM,
  clientId: import.meta.env.VITE_KEYCLOAK_CLIENT_ID,
});

/** Reactive mirror of keycloak.authenticated for templates and computed()s. */
export const isAuthenticated = ref(false);

keycloak.onAuthSuccess = () => (isAuthenticated.value = true);
keycloak.onAuthLogout = () => (isAuthenticated.value = false);
keycloak.onTokenExpired = () => {
  keycloak.updateToken(30).catch(() => keycloak.clearToken());
};

/** Restores an existing Keycloak session without forcing a login. Never throws so the app still loads if Keycloak is down. */
export const initAuth = async () => {
  try {
    await keycloak.init({
      onLoad: "check-sso",
      pkceMethod: "S256",
      silentCheckSsoRedirectUri: `${window.location.origin}/silent-check-sso.html`,
      checkLoginIframe: false,
      messageReceiveTimeout: 5000,
    });
  } catch (err) {
    console.warn("Keycloak is not reachable, continuing anonymously", err);
  }
  isAuthenticated.value = !!keycloak.authenticated;
};

/** A fresh access token, or undefined when signed out. */
export const getToken = async (): Promise<string | undefined> => {
  if (!keycloak.authenticated) return undefined;
  try {
    await keycloak.updateToken(30);
  } catch {
    keycloak.clearToken();
    return undefined;
  }
  return keycloak.token;
};

const absolute = (path = "/") => `${window.location.origin}${path}`;

export const login = (redirectPath?: string) =>
  keycloak.login({ redirectUri: absolute(redirectPath ?? window.location.pathname + window.location.search) });

export const register = (redirectPath = "/") => keycloak.register({ redirectUri: absolute(redirectPath) });

export const logout = () => keycloak.logout({ redirectUri: absolute("/") });

export const accountUrl = () => keycloak.createAccountUrl();
