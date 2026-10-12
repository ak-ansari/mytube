import { defineComponent, h, type Component } from "vue";
import { mount, type ComponentMountingOptions } from "@vue/test-utils";
import { createVuetify, type ThemeDefinition } from "vuetify";
import * as components from "vuetify/components";
import * as directives from "vuetify/directives";
import { QueryClient, VueQueryPlugin } from "@tanstack/vue-query";
import { createRouter, createMemoryHistory, type RouteRecordRaw, type Router } from "vue-router";
import { createNotivue } from "notivue";

const Stub = defineComponent({ name: "RouteStub", render: () => h("div", { class: "route-stub" }) });

export const testRoutes: RouteRecordRaw[] = [
  { path: "/", name: "Home", component: Stub },
  { path: "/results", name: "Results", component: Stub },
  { path: "/watch/:videoId", name: "Watch", component: Stub, meta: { immersive: true } },
  { path: "/channel/:channelId", name: "Channel", component: Stub },
  { path: "/subscriptions", name: "Subscriptions", component: Stub },
  { path: "/liked", name: "Liked", component: Stub },
  { path: "/studio", name: "Studio", component: Stub },
  { path: "/upload", name: "Upload", component: Stub },
  { path: "/:pathMatch(.*)*", name: "NotFound", component: Stub },
];

export const createTestRouter = (routes: RouteRecordRaw[] = testRoutes) =>
  createRouter({ history: createMemoryHistory(), routes });

export const createTestQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: { retry: false, refetchOnWindowFocus: false, gcTime: Infinity },
      mutations: { retry: false },
    },
  });

const dark: ThemeDefinition = { dark: true, colors: { background: "#0f0f0f", accent: "#3ea6ff" } };
const light: ThemeDefinition = { dark: false, colors: { background: "#ffffff", accent: "#065fd4" } };

export const createTestVuetify = (defaultTheme = "myTubeDark") =>
  createVuetify({
    components,
    directives,
    theme: { defaultTheme, themes: { myTubeDark: dark, myTubeLight: light } },
    // render overlays in place (no teleport) so wrapper.find() sees dialog/menu content
    defaults: {
      VDialog: { attach: true },
      VMenu: { attach: true },
      VTooltip: { attach: true },
      VOverlay: { attach: true },
    },
  });

export interface MountExtras {
  /** initial route, default "/" */
  route?: string;
  router?: Router;
  queryClient?: QueryClient;
  vuetify?: ReturnType<typeof createVuetify>;
  /** wrap in <v-app> (needed by v-app-bar / v-navigation-drawer / v-main) */
  withApp?: boolean;
}

/** Mounts with Vuetify, a fresh QueryClient (retry off), a memory-history router and notivue. */
export async function mountWithPlugins<C extends Component>(
  component: C,
  options: ComponentMountingOptions<C> & MountExtras = {} as ComponentMountingOptions<C> & MountExtras
) {
  const { route = "/", router = createTestRouter(), queryClient = createTestQueryClient(), vuetify = createTestVuetify(), withApp, ...rest } =
    options;
  await router.push(route);
  await router.isReady();

  const mountOpts = rest as ComponentMountingOptions<C>;
  const global = {
    ...(mountOpts.global ?? {}),
    plugins: [...(mountOpts.global?.plugins ?? []), vuetify, [VueQueryPlugin, { queryClient }], router, createNotivue({})] as never[],
  };

  let wrapper;
  if (withApp) {
    const Wrapper = defineComponent({
      setup:
        (_p, { attrs }) =>
        () =>
          h(components.VApp, null, { default: () => h(component as never, attrs) }),
    });
    wrapper = mount(Wrapper, { ...(mountOpts as object), props: (mountOpts as { props?: object }).props, global } as never);
  } else {
    wrapper = mount(component, { ...mountOpts, global } as never);
  }
  return { wrapper: wrapper as ReturnType<typeof mount>, router, queryClient, vuetify };
}

/** Lets pending promises, query resolutions and Vue re-renders settle. */
export const settle = async (rounds = 5) => {
  const { flushPromises } = await import("@vue/test-utils");
  for (let i = 0; i < rounds; i++) {
    await flushPromises();
    await new Promise((r) => setTimeout(r, 0));
  }
};
