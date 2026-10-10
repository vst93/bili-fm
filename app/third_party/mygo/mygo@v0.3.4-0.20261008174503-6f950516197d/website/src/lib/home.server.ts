import fs from "node:fs/promises"
import path from "node:path"
import type { Code, Root } from "mdast"
import remarkParse from "remark-parse"
import { unified } from "unified"

import type { HomeData } from "@/lib/home"
import { highlightCode, highlightTokens } from "@/lib/markdown.server"

/**
 * The examples at the top of the docs: a Go service and the page calling it,
 * then a window of native UI, in that order.
 */
export async function loadHome(): Promise<HomeData> {
  const readme = await fs.readFile(path.resolve(process.cwd(), "../docs/README.md"), "utf8")
  const tree = unified().use(remarkParse).parse(readme) as Root
  const blocks = tree.children.filter((n): n is Code => n.type === "code")
  const [go, native] = blocks.filter((b) => b.lang === "go")
  const ts = blocks.find((b) => b.lang === "ts")
  if (!go || !ts || !native) throw new Error("docs/README.md: expected a go and a ts example, then a go example of native UI")
  return {
    main: [await highlightTokens(go.value, "go"), await highlightTokens(native.value, "go")],
    ts: await highlightCode(ts.value, "ts"),
  }
}
