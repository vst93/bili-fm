import { defineConfig } from "mygo-cli";

export default defineConfig({
  name: "Todo",
  identifier: "dev.mygo.todo",
  version: "1.0.0",
  devUrl: "http://localhost:5173",
  devCommand: "bun run dev:web",
  buildCommand: "bun run build:web",
  frontendDist: "dist",
  bindings: "src/mygo.ts",
  out: "build",
});
