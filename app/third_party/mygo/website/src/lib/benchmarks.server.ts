import fs from "node:fs/promises"
import path from "node:path"

const root = path.resolve(process.cwd(), "..")

// Directories without Go packages, or with their test data.
const skip = new Set(["node_modules", "website", "testdata", "dist", "build"])

/**
 * The doc comments of the repository's Go benchmarks, by "<package>/<name>":
 * the package's directory ("." for the root) and the name without
 * "Benchmark", as scripts/bench.ts names them. A benchmark without a comment
 * of its own has the one that names it, as a comment of several one-line
 * benchmarks does.
 */
export async function loadBenchmarkDocs(): Promise<Record<string, string>> {
  const docs: Record<string, string> = {}
  for (const file of await testFiles(root)) {
    const src = await fs.readFile(file, "utf8")
    if (!src.includes("func Benchmark")) continue
    const pkg = path.relative(root, path.dirname(file)).split(path.sep).join("/") || "."
    const lines = src.split("\n")
    const blocks: { end: number; text: string }[] = []
    for (let i = 0; i < lines.length; i++) {
      if (!lines[i]!.startsWith("//")) continue
      const start = i
      while (i + 1 < lines.length && lines[i + 1]!.startsWith("//")) i++
      const text = lines
        .slice(start, i + 1)
        .map((l) => l.replace(/^\/\/ ?/, ""))
        .join(" ")
      blocks.push({ end: i, text: text.replace(/\s+/g, " ").trim() })
    }
    lines.forEach((line, i) => {
      const name = /^func Benchmark(\w+)\(/.exec(line)?.[1]
      if (!name) return
      const block = blocks.find((b) => b.end === i - 1) ?? blocks.find((b) => new RegExp(`\\bBenchmark${name}\\b`).test(b.text))
      if (block) docs[`${pkg}/${name}`] = describe(name, block.text)
    })
  }
  return docs
}

/** "BenchmarkFrame builds a frame" → "Builds a frame", with the names of benchmarks the comment mentions shortened as the page shows them. */
function describe(name: string, text: string) {
  const s = text.replace(/\bBenchmark(\w)/g, "$1")
  if (!s.startsWith(name + " ")) return s
  const rest = s.slice(name.length + 1)
  return rest.charAt(0).toUpperCase() + rest.slice(1)
}

async function testFiles(dir: string): Promise<string[]> {
  const files: string[] = []
  for (const entry of await fs.readdir(dir, { withFileTypes: true })) {
    if (entry.name.startsWith(".") || skip.has(entry.name)) continue
    const p = path.join(dir, entry.name)
    if (entry.isDirectory()) files.push(...(await testFiles(p)))
    else if (entry.name.endsWith("_test.go")) files.push(p)
  }
  return files
}
