import { defineConfig } from "vitest/config";
import { fileURLToPath, URL } from "node:url";

export default defineConfig({
  resolve: {
    alias: {
      "~": fileURLToPath(new URL("./src", import.meta.url)),
      // Mirror the vite.config alias: the generated client package lives at
      // repo-root clients/typescript.
      "@longhouse/client": fileURLToPath(
        new URL("../clients/typescript/index.ts", import.meta.url),
      ),
    },
  },
  test: {
    environment: "happy-dom",
    globals: true,
    setupFiles: ["./vitest.setup.ts"],
    // exclude the e2e-ish things we don't do — unit/component only.
    include: ["src/**/*.{test,spec}.{ts,tsx}"],
  },
});
