import "vuetify/styles/main.css";
import { createVuetify, type ThemeDefinition } from "vuetify";
import * as components from "vuetify/components";
import * as directives from "vuetify/directives";

const myTubeDark: ThemeDefinition = {
  dark: true,
  colors: {
    background: "#0f0f0f",
    surface: "#0f0f0f",
    "surface-bright": "#272727",
    "surface-light": "#212121",
    "surface-variant": "#3f3f3f",
    "on-surface-variant": "#f1f1f1",
    primary: "#ff0033",
    secondary: "#f1f1f1",
    accent: "#3ea6ff",
    info: "#3ea6ff",
    success: "#2ba640",
    warning: "#f9a825",
    error: "#ff4e45",
  },
  variables: {
    "muted-text": "#aaaaaa",
  },
};

const myTubeLight: ThemeDefinition = {
  dark: false,
  colors: {
    background: "#ffffff",
    surface: "#ffffff",
    "surface-bright": "#f2f2f2",
    "surface-light": "#f8f8f8",
    "surface-variant": "#e5e5e5",
    "on-surface-variant": "#0f0f0f",
    primary: "#ff0033",
    secondary: "#0f0f0f",
    accent: "#065fd4",
    info: "#065fd4",
    success: "#2ba640",
    warning: "#f57f17",
    error: "#cc0000",
  },
  variables: {
    "muted-text": "#606060",
  },
};

const savedTheme = (() => {
  try {
    return localStorage.getItem("mytube-theme");
  } catch {
    return null;
  }
})();

export const vuetify = createVuetify({
  components,
  directives,
  theme: {
    defaultTheme: savedTheme === "myTubeLight" ? "myTubeLight" : "myTubeDark",
    themes: { myTubeLight, myTubeDark },
  },
  defaults: {
    VBtn: { rounded: "pill", class: "text-none" },
    VTextField: { variant: "outlined", density: "comfortable", color: "accent" },
    VTextarea: { variant: "outlined", density: "comfortable", color: "accent" },
    VSelect: { variant: "outlined", density: "comfortable", color: "accent" },
    VCard: { rounded: "xl" },
    VDialog: { maxWidth: 640 },
  },
});
