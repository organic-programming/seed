import { defineConfig } from "vite";
import { fileURLToPath, URL } from "node:url";

export default defineConfig({
  resolve: {
    alias: {
      protobufjs: fileURLToPath(new URL("./node_modules/protobufjs", import.meta.url)),
    },
  },
  server: {
    port: 5173,
  },
});
