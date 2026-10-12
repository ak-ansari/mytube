import "@mdi/font/css/materialdesignicons.css";
import "./style.css";
import App from "./App.vue";
import { createApp } from "vue";
import { vuetify } from "./plugins/vuetify";
import router from "./router";
import { MutationCache, QueryClient, VueQueryPlugin } from "@tanstack/vue-query";
import { createPinia } from "pinia";
import { push } from "notivue";
import { useSetupNotification } from "./composables/useSetupNotification";
import { initAuth } from "./auth/keycloak";
import { errorMessage } from "./api/http";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { refetchOnWindowFocus: false, staleTime: 30_000 },
  },
  // every failed mutation surfaces the backend message as a toast
  mutationCache: new MutationCache({
    onError: (err) => push.error({ title: "Error", message: errorMessage(err) }),
  }),
});

// restore the Keycloak session before the router runs its auth guard
initAuth().then(() => {
  createApp(App)
    .use(vuetify)
    .use(VueQueryPlugin, { queryClient })
    .use(createPinia())
    .use(router)
    .use(useSetupNotification())
    .mount("#app");
});
