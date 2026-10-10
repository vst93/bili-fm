import fs from "node:fs"
import path from "node:path"
import tailwindcss from "@tailwindcss/vite"
import { tanstackStart } from "@tanstack/react-start/plugin/vite"
import viteReact from "@vitejs/plugin-react"
import { defineConfig } from "vite"
import { llmsDocs } from "./llms.ts"

const repo = (file: string) => new URL(`../${file}`, import.meta.url)

// The pages of the repository's docs, plugins/fetch.md at /docs/plugins/fetch,
// and a directory's README at the directory, ui/README.md at /docs/ui. /docs
// itself, like every route without parameters, is found without being
// listed.
const docs = fs
  .readdirSync(repo("docs"), { recursive: true, encoding: "utf8" })
  .filter((file) => file.endsWith(".md") && file !== "README.md")
  .map((file) => `/docs/${file.slice(0, -3).split(path.sep).join("/").replace(/\/README$/, "")}`)

const version = /const Version = "([^"]+)"/.exec(fs.readFileSync(repo("mygo.go"), "utf8"))?.[1]
const goVersion = /^go (\S+)/m.exec(fs.readFileSync(repo("go.mod"), "utf8"))?.[1]

export default defineConfig({
  define: {
    __MYGO_VERSION__: JSON.stringify(version),
    __GO_VERSION__: JSON.stringify(goVersion),
  },
  resolve: { tsconfigPaths: true },
  plugins: [
    llmsDocs(path.resolve(import.meta.dirname, "..")),
    tailwindcss(),
    tanstackStart({
      // A static site: every page is rendered to HTML at build time.
      prerender: { enabled: true, crawlLinks: true, filter: ({ path }) => !path.includes("#") },
      pages: [
        ...docs.map((path) => ({ path })),
        { path: "/docs/search.json" },
        // Served by Cloudflare for any path that has no page.
        { path: "/404", prerender: { enabled: true, outputPath: "/404.html" } },
      ],
    }),
    viteReact(),
  ],
})
