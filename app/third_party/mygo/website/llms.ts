// Publish the same docs as plain Markdown, without a server in production.
import fs from "node:fs/promises"
import path from "node:path"
import type { Root } from "mdast"
import { toString } from "mdast-util-to-string"
import remarkGfm from "remark-gfm"
import remarkParse from "remark-parse"
import remarkStringify from "remark-stringify"
import { unified } from "unified"
import { visit } from "unist-util-visit"
import type { Plugin } from "vite"

import { omitRepositoryOnly, rewriteHref, slugOf, type LinkOptions } from "./src/lib/markdown-source.ts"
import { site } from "./src/lib/site.ts"

/** All downloads, keyed by their site path, for the build and dev server. */
export async function docsAssets(repoDir: string): Promise<Map<string, string>> {
  const docsDir = path.join(repoDir, "docs")
  const links: LinkOptions = { docsDir, repoDir, repoUrl: site.repo }
  const files = (await fs.readdir(docsDir, { recursive: true }))
    .filter((file) => file.endsWith(".md"))
    .sort((a, b) => {
      // Each book's introduction comes before its pages.
      const key = (file: string) => file.replace(/(^|[\\/])README\.md$/, "$1")
      return key(a).localeCompare(key(b), "en")
    })
  const assets = new Map<string, string>()
  const books = { docs: [] as string[], ui: [] as string[] }
  const full: string[] = [`# ${site.name} documentation`, site.description]
  const processor = unified().use(remarkParse).use(remarkGfm).use(remarkStringify, { fences: true })

  for (const file of files) {
    const abs = path.join(docsDir, file)
    const tree = processor.parse(await fs.readFile(abs, "utf8")) as Root
    omitRepositoryOnly(tree)
    visit(tree, (node) => {
      if (node.type !== "link" && node.type !== "definition" && node.type !== "image") return
      const href = rewriteHref(node.url, abs, links, true)
      // Absolute URLs also work when the page is in llms-full.txt.
      node.url = href.startsWith("/") && !href.startsWith("//") ? new URL(href, site.url).href : href
    })
    const slug = slugOf(file)
    const pathname = (slug ? `/docs/${slug}` : "/docs") + ".md"
    if (assets.has(pathname)) throw new Error(`Duplicate Markdown page: ${pathname}`)
    const url = site.url + pathname
    const title = tree.children.find((node) => node.type === "heading" && node.depth === 1)
    const lead = tree.children.find((node) => node.type === "paragraph")
    const name = title ? toString(title) : slug
    const description = lead ? toString(lead).replace(/\s+/g, " ") : ""
    const source = processor.stringify(tree)
    assets.set(pathname, source)
    const book = slug === "ui" || slug.startsWith("ui/") ? "ui" : "docs"
    const label = name.replace(/[\[\]\\]/g, "\\$&")
    books[book].push(`- [${label}](${url})${description ? `: ${description}` : ""}`)
    full.push(`---\n\nSource: ${url}\n\n${source.trimEnd()}`)
  }

  assets.set("/llms.txt", [
    `# ${site.name}`,
    `> ${site.description}`,
    `Individual Markdown pages are listed below. All documentation is also available at ${site.url}/llms-full.txt.`,
    "## Documentation\n\n" + books.docs.join("\n"),
    "## Native UI\n\n" + books.ui.join("\n"),
  ].join("\n\n") + "\n")
  assets.set("/llms-full.txt", full.join("\n\n") + "\n")
  return assets
}

export function llmsDocs(repoDir: string): Plugin {
  return {
    name: "mygo-markdown-docs",
    applyToEnvironment: (environment) => environment.name === "client",
    async generateBundle() {
      for (const [pathname, source] of await docsAssets(repoDir)) {
        this.emitFile({ type: "asset", fileName: pathname.slice(1), source })
      }
    },
    configureServer(server) {
      server.middlewares.use(async (req, res, next) => {
        const pathname = req.url?.split("?", 1)[0] ?? ""
        if (pathname !== "/llms.txt" && pathname !== "/llms-full.txt" && !/^\/docs(?:\/.*)?\.md$/.test(pathname)) return next()
        try {
          const source = (await docsAssets(repoDir)).get(pathname)
          if (source === undefined) return next()
          res.setHeader("Content-Type", `${pathname.endsWith(".md") ? "text/markdown" : "text/plain"}; charset=utf-8`)
          res.end(source)
        } catch (error) {
          next(error)
        }
      })
    },
  }
}
