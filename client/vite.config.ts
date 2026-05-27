import { defineConfig } from "vite";
import solid from "vite-plugin-solid";
import { fileURLToPath, URL } from "node:url";
import { readFileSync } from "node:fs";

// The release version is owned by version/VERSION.txt at the repo root
// (CI bumps it on push-to-main). Inject it as a compile-time constant so
// the SPA footer doesn't have to be hand-edited and drift out of sync.
const appVersion = readFileSync(
  fileURLToPath(new URL("../version/VERSION.txt", import.meta.url)),
  "utf8",
).trim();

export default defineConfig({
  define: {
    __APP_VERSION__: JSON.stringify(appVersion),
  },
  plugins: [solid()],
  resolve: {
    alias: {
      "~": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    port: 5173,
    host: "localhost",
    proxy: {
      // dev-only: forward all /api/* requests to the local api server.
      // Match LONGHOUSE_API_PORT in api/internal/config/config.go (6080).
      "/api": {
        target: "http://localhost:6080",
        changeOrigin: false,
      },
    },
  },
  build: {
    target: "es2022",
    sourcemap: true,
  },
});
