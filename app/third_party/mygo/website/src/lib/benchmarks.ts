import { createServerFn } from "@tanstack/react-start"
import { staticFunctionMiddleware } from "@tanstack/start-static-server-functions"

import { loadBenchmarkDocs } from "@/lib/benchmarks.server"
import { site } from "@/lib/site"

/**
 * latest.json of the benchmarks branch, which scripts/bench.ts writes (its
 * Latest): the last commits measured, oldest first, and their values by
 * series.
 */
export interface BenchmarkData {
  version: 1
  commits: Commit[]
  /** By OS: the runner of each commit, null where the OS has no run. */
  runners: Record<string, (Runner | null)[]>
  /** By OS, package, benchmark and unit: the value at each commit, or null. */
  series: Record<string, Record<string, Record<string, Record<string, (number | null)[]>>>>
}

export interface Commit {
  sha: string
  date: string
  message: string
}

export interface Runner {
  arch: string
  cpu: string
  go: string
}

/** Where the page reads the results: the browser fetches them, as the workflow pushes them after the site is built. */
export const dataUrl: string = import.meta.env.VITE_BENCHMARKS_URL || "https://raw.githubusercontent.com/egoist/mygo/benchmarks/latest.json"

export const workflowUrl = `${site.repo}/actions/workflows/bench.yml`

export const getBenchmarkDocs = createServerFn({ method: "GET" })
  .middleware([staticFunctionMiddleware])
  .handler(() => loadBenchmarkDocs())

export const platforms = [
  { id: "darwin", label: "macOS" },
  { id: "linux", label: "Linux" },
  { id: "windows", label: "Windows" },
] as const

export type Platform = (typeof platforms)[number]["id"]

export const metrics = [
  { id: "time", unit: "ns/op", label: "Time", text: "time per operation" },
  { id: "memory", unit: "B/op", label: "Memory", text: "memory allocated per operation" },
  { id: "allocs", unit: "allocs/op", label: "Allocations", text: "allocations per operation" },
] as const

export type Metric = (typeof metrics)[number]["id"]

export const ranges = [50, 100, 300] as const

export interface Group {
  /** The package's directory, as scripts/bench.ts names it. */
  pkg: string
  title: string
  text: string
  /** The unit its charts show whatever the metric chosen, for results that are not Go benchmarks', and what it is. */
  unit?: string
  unitText?: string
  /** What its info button tells: terms and what they are, then a note. */
  info?: { terms: [string, string][]; note?: string }
}

export const groups: Group[] = [
  {
    pkg: "size",
    title: "App size",
    text: "Release builds of two examples, compiled as mygo build compiles them, without their icons.",
    unit: "bytes",
    unitText: "bytes",
  },
  {
    pkg: "memory",
    title: "Idle memory",
    text: "The same builds, six seconds after they start.",
    unit: "B",
    unitText: "bytes",
    info: {
      terms: [
        ["macOS", "Physical footprint, as Activity Monitor shows it."],
        ["Linux", "Proportional set size: shared libraries count in part."],
        ["Windows", "Private working set, as Task Manager shows it: shared pages left out."],
        ["hello", "With its webview's processes; hello/app without."],
      ],
      note: "GitHub's runners show 1024×768 at scale 1, so apps take less than on a Retina Mac: counter-native 22 MB there, 37 MB on an M5 MacBook.",
    },
  },
  {
    pkg: "internal/e2e",
    title: "Webview and windows",
    text: "The real backend: pages calling Go through the system webview, streaming from it, receiving its events and fetching from the app's scheme, and windows opening.",
  },
  {
    pkg: ".",
    title: "IPC, Go side",
    text: "What MyGo itself spends on calls, channels, events and custom schemes: decoding, calling, encoding and batching, on the fake backend, without a webview.",
  },
  { pkg: "ui", title: "Native UI", text: "Frames of native UI built, laid out, painted and rendered on the CPU; text areas; lists." },
  { pkg: "internal/text", title: "Text layout", text: "Paragraphs and labels shaped and broken into lines." },
  { pkg: "internal/raster", title: "CPU renderer", text: "Scenes drawn into memory, as windows without a GPU draw them." },
  { pkg: "internal/svg", title: "SVG", text: "Icons parsed and drawn." },
  { pkg: "plugins/terminal", title: "Terminal", text: "The terminal plugin taking a program's output, and drawing its view." },
]

/** What the sizes and memory measure, which are not Go benchmarks with comments. */
export const appDocs: Record<string, string> = {
  "size/hello": "examples/hello: a window with a web page calling a Go method.",
  "size/counter-native": "examples/counter-native: a window of native UI.",
  "memory/hello": "examples/hello, with its webview's processes.",
  "memory/hello/app": "examples/hello's own process.",
  "memory/counter-native": "examples/counter-native: native UI, no other process.",
}

/** How to read the page, which its info button tells. */
export const reading: NonNullable<Group["info"]> = {
  terms: [
    ["▲ ▼", "A commit that moved a result beyond its noise: worse, better."],
    ["Card", "The last value, against the first ones shown."],
    ["Gray", "Timings on other CPUs: runners get one of several, so timings compare only on the same one."],
  ],
}

/** What a unit measures, as the summary names it. */
export function unitLabel(unit: string) {
  const metric = metrics.find((m) => m.unit === unit)
  return metric ? metric.label.toLowerCase() : unit === "bytes" ? "size" : unit === "B" ? "idle" : unit
}

/** The smallest change of a series that stands out, whatever its noise. */
function minChange(unit: string) {
  return unit === "ns/op" ? 0.05 : unit === "bytes" ? 0.002 : 0.01
}

/**
 * The smallest difference that stands out: a byte or an allocation now
 * and then is a rounding of Go's averages, and a process's memory moves by
 * pages.
 */
function minDelta(unit: string) {
  return unit === "B/op" ? 16 : unit === "allocs/op" ? 1 : unit === "B" ? 64 << 10 : 0
}

/**
 * The relative change a series makes on its own, given values of it: five
 * median absolute deviations, a robust spread (about one series in a
 * thousand moves that much by chance), or their whole range while there
 * are too few of them to tell, and at least minChange.
 */
function noiseOf(values: number[], unit: string) {
  let noise = minChange(unit)
  const m = median(values)
  if (values.length >= 5 && m > 0) {
    noise = Math.max(noise, (5 * median(values.map((v) => Math.abs(v - m)))) / m)
    if (values.length < 15) noise = Math.max(noise, (Math.max(...values) - Math.min(...values)) / m)
  }
  return noise
}

/** after / before - 1, Infinity from zero. */
function ratio(after: number, before: number) {
  return before > 0 ? after / before - 1 : after === 0 ? 0 : Infinity
}

/** A commit that moved a series for good. */
export interface Step {
  /** The commit's index. */
  index: number
  /** The median of the five values before it, and of the five from it. */
  before: number
  after: number
  change: number
}

/**
 * Finds the commits that moved a series: those whose value, and the
 * median of the five from them, differ from the median of the five before
 * by more than its noise (noiseOf the twenty before) and minDelta, in the
 * same direction. Commits in a row that do are one step, at the biggest
 * jump, and the values before a step are left behind: the five after it
 * tell nothing yet, as the first five do. A series that was not steady
 * needs ten values before a step, and two commits after it to confirm it.
 *
 * Timings are compared only among the commits measured on the same CPU:
 * GitHub's runners get one of several from run to run, which moves them
 * more than most changes.
 */
export function findSteps(values: (number | null)[], unit: string, runners: (Runner | null)[]): Step[] {
  if (unit !== "ns/op") return stepsOf(values, unit)
  const cpus = new Set(runners.flatMap((r) => (r ? [r.cpu] : [])))
  return [...cpus].flatMap((cpu) => stepsOf(onCPU(values, runners, cpu), unit)).sort((a, b) => a.index - b.index)
}

/** The values measured on cpu, null elsewhere. */
export function onCPU(values: (number | null)[], runners: (Runner | null)[], cpu: string | undefined) {
  return values.map((v, i) => (runners[i]?.cpu === cpu ? v : null))
}

/** The steps of findSteps, in values of one CPU. */
function stepsOf(values: (number | null)[], unit: string): Step[] {
  const at = values.flatMap((v, i) => (v == null ? [] : [i]))
  const v = (k: number) => values[at[k]!]!
  const steps: Step[] = []
  // level is where the values compared from began: the first, or the last step.
  let level = 0
  let run: { step: Step; jump: number; k: number } | undefined
  const end = () => {
    if (!run) return
    steps.push(run.step)
    level = run.k
    run = undefined
  }
  for (let k = 5; k < at.length; k++) {
    if (k - level < 5) continue
    const i = at[k]!
    const before = at.slice(Math.max(level, k - 20), k).map((j) => values[j]!)
    const noise = noiseOf(before, unit)
    const b = median(before.slice(-5))
    const a = median(at.slice(k, k + 5).map((j) => values[j]!))
    const change = ratio(a, b)
    const own = ratio(v(k), b)
    const steady = Math.max(...before) - Math.min(...before) <= minChange(unit) * b
    const small = Math.abs(a - b) < minDelta(unit) || Math.abs(v(k) - b) < minDelta(unit)
    // A series that moves needs ten values to tell its noise, and two
    // commits after a step to confirm it.
    const unconfirmed = !steady && (before.length < 10 || at.length - k < 3)
    if (small || unconfirmed || Math.abs(change) <= noise || Math.abs(own) <= noise || Math.sign(change) !== Math.sign(own)) {
      end()
      continue
    }
    const jump = Math.abs(v(k) - v(k - 1)) / Math.max(b, Number.MIN_VALUE)
    if (!run || jump > run.jump) run = { step: { index: i, before: b, after: a, change }, jump, k }
  }
  end()
  return steps
}

/** How a series went over the commits shown, from index from on. */
export interface Trend {
  /** The last value, and its index. */
  latest: number
  index: number
  /** The median of the first five values shown, before the last. */
  start?: number
  /** latest against start. */
  change?: number
  /** How much the series moves on its own, relatively: the change that stands out. */
  noise: number
  significant: boolean
  /** The steps among the commits shown. */
  steps: Step[]
}

/**
 * Compares a series' last value with the first ones shown, from index
 * from, so that a step stays in sight however many commits came after it,
 * and finds its steps. A timing is compared with those measured on the
 * same CPU.
 */
export function trendOf(values: (number | null)[], unit: string, runners: (Runner | null)[], from: number): Trend | undefined {
  let index = values.length - 1
  while (index >= 0 && values[index] == null) index--
  if (index < 0) return undefined
  const latest = values[index]!
  const nonNull = (v: number | null): v is number => v != null
  const like = unit === "ns/op" ? onCPU(values, runners, runners[index]?.cpu) : values
  const noise = noiseOf(like.slice(0, index).filter(nonNull).slice(-20), unit)
  const shown = like.slice(from, index).filter(nonNull)
  const steps = findSteps(values, unit, runners).filter((s) => s.index >= from)
  if (!shown.length) return { latest, index, noise, significant: false, steps }
  const start = median(shown.slice(0, 5))
  const change = ratio(latest, start)
  // The noise of a series that moves needs ten values to tell, one that
  // does not five.
  const history = like.slice(0, index).filter(nonNull).slice(-20)
  const steady = history.length > 0 && Math.max(...history) - Math.min(...history) <= minChange(unit) * median(history)
  const known = shown.length >= 5 && history.length >= (steady ? 5 : 10)
  const significant = known && Math.abs(change) > noise && Math.abs(latest - start) >= minDelta(unit)
  return { latest, index, start, change, noise, significant, steps }
}

/** A CPU's name without what all of GitHub's runners' share: "AMD EPYC 7763 64-Core Processor" is "EPYC 7763". */
export function shortCPU(cpu: string) {
  return cpu
    .replace(/\((R|TM)\)/gi, "")
    .replace(/\b(AMD|Intel|CPU|Processor|\d+-Core)\b/gi, "")
    .replace(/\s+/g, " ")
    .trim()
}

/** The last value of a series. */
export function lastValue(values: (number | null)[]) {
  for (let i = values.length - 1; i >= 0; i--) if (values[i] != null) return values[i]!
  return undefined
}

export function median(values: number[]) {
  const s = [...values].sort((a, b) => a - b)
  const mid = s.length >> 1
  return s.length % 2 ? s[mid]! : (s[mid - 1]! + s[mid]!) / 2
}

function sig(v: number) {
  return v >= 100 ? Math.round(v).toLocaleString("en-US") : String(Number(v.toPrecision(3)))
}

/** Formats a value of a unit of scripts/bench.ts: "27.4 µs", "4.09 kB", "14 allocs". */
export function formatValue(v: number, unit: string) {
  switch (unit) {
    case "ns/op":
      if (v >= 1e9) return `${sig(v / 1e9)} s`
      if (v >= 1e6) return `${sig(v / 1e6)} ms`
      if (v >= 1e3) return `${sig(v / 1e3)} µs`
      return `${sig(v)} ns`
    case "B/op":
    case "bytes":
    case "B":
      if (v >= 1e9) return `${sig(v / 1e9)} GB`
      if (v >= 1e6) return `${sig(v / 1e6)} MB`
      if (v >= 1e3) return `${sig(v / 1e3)} kB`
      return `${sig(v)} B`
    case "allocs/op":
      return `${Number.isInteger(v) ? v.toLocaleString("en-US") : sig(v)} ${v === 1 ? "alloc" : "allocs"}`
    default:
      return `${sig(v)} ${unit}`
  }
}

export function formatChange(change: number) {
  if (change === Infinity) return "from 0"
  if (change >= 9) return `${(change + 1).toFixed(change >= 99 ? 0 : 1)}×`
  const pct = Math.abs(change * 100)
  const digits = pct < 10 ? 1 : 0
  return `${change < 0 ? "−" : "+"}${pct.toFixed(digits)}%`
}

export function formatDate(iso: string, withYear = false) {
  return new Date(iso).toLocaleDateString("en-US", { month: "short", day: "numeric", ...(withYear ? { year: "numeric" } : {}) })
}

export function commitUrl(sha: string) {
  return `${site.repo}/commit/${sha}`
}

/** The id of a benchmark's chart, as a fragment of the page's URL. */
export function anchorOf(pkg: string, name: string) {
  return `${pkg === "." ? "mygo" : pkg}/${name}`.replace(/[^\w-]+/g, "-")
}
