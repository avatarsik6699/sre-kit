import { tanstackStart } from "@tanstack/react-start/plugin/vite";
import viteReact from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  server: {
    port: 3000,
    // Dev-only: proxies API/WS calls to the Go backend (cmd/server) so `pnpm dev` can be used
    // against a locally running core without a production static-serving route existing yet.
    // Doesn't affect `vite build` output. See docs/changes/04-minimal-ui.md § Implementation
    // Notes for the follow-up this implies (no route in cmd/server/main.go serves the built
    // frontend yet).
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        ws: true,
      },
    },
  },
  resolve: {
    tsconfigPaths: true,
  },
  plugins: [
    tanstackStart(),
    // react's vite plugin must come after start's vite plugin
    viteReact(),
  ],
});
