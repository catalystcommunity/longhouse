import { defineConfig } from "vitest/config";
import solid from "vite-plugin-solid";
import { fileURLToPath, URL } from "node:url";

export default defineConfig({
  plugins: [solid()],
  resolve: {
    alias: {
      "~": fileURLToPath(new URL("./src", import.meta.url)),
      // Mirror the vite.config alias: the generated client package lives at
      // repo-root clients/typescript.
      "@longhouse/client": fileURLToPath(
        new URL("../clients/typescript/index.ts", import.meta.url),
      ),
    },
    // Solid needs its dev/browser build for reactivity to behave in tests;
    // without these conditions vitest pulls the server build and effects
    // don't run the way they do in the browser.
    conditions: ["development", "browser"],
  },
  test: {
    environment: "happy-dom",
    globals: true,
    setupFiles: ["./vitest.setup.ts"],
    // exclude the e2e-ish things we don't do — unit/component only.
    include: ["src/**/*.{test,spec}.{ts,tsx}"],
  },
});
