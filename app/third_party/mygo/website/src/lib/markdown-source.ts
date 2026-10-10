// Rules shared by the HTML pages and the Markdown downloads.
import path from "node:path"
import type { Root } from "mdast"

export interface LinkOptions {
  docsDir: string
  repoDir: string
  repoUrl: string
}

/** README.md is the directory's page; other files keep their path. */
export function slugOf(file: string) {
  const name = file.replace(/\.md$/, "").split(path.sep).join("/")
  return name === "README" ? "" : name.replace(/\/README$/, "")
}

/** Repository-only blocks stay on GitHub, outside all published formats. */
export function omitRepositoryOnly(tree: Root) {
  let hidden = false
  tree.children = tree.children.filter((node) => {
    if (node.type === "html") {
      const marker = node.value.trim()
      if (marker === "<!-- repository-only:start -->" || marker === "<!-- repository-only:end -->") {
        hidden = marker === "<!-- repository-only:start -->"
        return false
      }
    }
    return !hidden
  })
}

/** Links between docs stay on the site; links to repository files go to GitHub. */
export function rewriteHref(href: string, file: string, links: LinkOptions, markdown = false) {
  if (/^[a-z][a-z0-9+.-]*:|^\/\/|^#|^\//i.test(href)) return href
  const end = href.search(/[?#]/)
  const target = end < 0 ? href : href.slice(0, end)
  const suffix = end < 0 ? "" : href.slice(end)
  const abs = path.resolve(path.dirname(file), decodeURIComponent(target))
  const rel = path.relative(links.docsDir, abs)
  if (abs.endsWith(".md") && !rel.startsWith("..") && !path.isAbsolute(rel)) {
    const slug = slugOf(rel)
    return (slug ? `/docs/${slug}` : "/docs") + (markdown ? ".md" : "") + suffix
  }
  const repoPath = path.relative(links.repoDir, abs).split(path.sep).join("/")
  return `${links.repoUrl}/${path.extname(abs) ? "blob" : "tree"}/main/${repoPath}${suffix}`
}
