import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { readdirSync } from "node:fs";
import { resolve } from "node:path";

/** Multi-entry: islands/Counter.tsx → rollup input "Counter" (+ manifest key islands/Counter.tsx). */
function islandEntries(): Record<string, string> {
  const dir = resolve("islands");
  const entries: Record<string, string> = {};
  for (const file of readdirSync(dir)) {
    if (file.startsWith("_")) continue;
    if (!/\.(tsx|jsx)$/.test(file)) continue;
    const name = file.replace(/\.(tsx|jsx)$/, "");
    entries[name] = resolve(dir, file);
  }
  return entries;
}

export default defineConfig({
  plugins: [react()],
  build: {
    manifest: true,
    outDir: "public/islands",
    emptyOutDir: true,
    rollupOptions: {
      input: islandEntries(),
    },
  },
  server: {
    origin: "http://localhost:5173",
    cors: true,
  },
});
