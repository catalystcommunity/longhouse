import { createSignal, createEffect } from "solid-js";

/**
 * Theme + hero-scene preferences.
 *
 * Pre-paint init lives in index.html so there's no flash on load.
 * This module mirrors what that script wrote into <html data-theme> /
 * <html data-scene>, exposes signals, and persists user changes.
 */

const THEME_KEY = "longhouse.theme";
const SCENE_KEY = "longhouse.scene";

type Theme = "light" | "dark";

const readTheme = (): Theme => {
  const attr = document.documentElement.getAttribute("data-theme");
  return attr === "dark" ? "dark" : "light";
};
const readScene = (): boolean =>
  document.documentElement.getAttribute("data-scene") !== "off";

const [theme, setThemeSig] = createSignal<Theme>(readTheme());
const [sceneOn, setSceneSig] = createSignal<boolean>(readScene());

createEffect(() => {
  document.documentElement.setAttribute("data-theme", theme());
});
createEffect(() => {
  document.documentElement.setAttribute("data-scene", sceneOn() ? "on" : "off");
});

export const useTheme = () => theme;
export const useSceneOn = () => sceneOn;

export const toggleTheme = () => {
  const next: Theme = theme() === "dark" ? "light" : "dark";
  setThemeSig(next);
  localStorage.setItem(THEME_KEY, next);
};

export const toggleScene = () => {
  const next = !sceneOn();
  setSceneSig(next);
  localStorage.setItem(SCENE_KEY, next ? "on" : "off");
};

// follow system preference when the user has not explicitly chosen.
const mql = window.matchMedia("(prefers-color-scheme: dark)");
mql.addEventListener("change", (e) => {
  if (!localStorage.getItem(THEME_KEY)) {
    setThemeSig(e.matches ? "dark" : "light");
  }
});
