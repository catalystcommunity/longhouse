import { defineConfig } from "vite";
import solid from "vite-plugin-solid";
import { fileURLToPath, URL } from "node:url";

export default defineConfig({
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
