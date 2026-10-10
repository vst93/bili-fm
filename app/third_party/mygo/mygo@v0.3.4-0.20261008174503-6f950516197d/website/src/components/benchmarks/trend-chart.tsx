import * as React from "react"

import { commitUrl, formatChange, formatDate, formatValue, median, shortCPU, type Commit, type Runner, type Step } from "@/lib/benchmarks"
import { cn } from "@/lib/utils"

const height = 112
const top = 16
const bottom = 6
// Room for the end dot at both ends.
const pad = 6

/**
 * A line of a benchmark's values over commits, with a triangle at each
 * step (findSteps): ▲ worse, ▼ better. Its axis fits the values but spans
 * at least a fifth of their median, so that noise stays small and a step
 * of a few percent shows. Timings move with the CPU of the runner, which
 * changes from run to run: their line joins the commits measured on the
 * last one's CPU, and gray lines those of other CPUs. The crosshair
 * follows the pointer or the arrow keys; a click or Enter opens the
 * commit.
 */
export function TrendChart({
  values,
  commits,
  runners,
  steps,
  unit,
  label,
}: {
  values: (number | null)[]
  commits: Commit[]
  runners: (Runner | null)[]
  /** The steps, by the index of their commit in values. */
  steps: Step[]
  unit: string
  label: string
}) {
  const ref = React.useRef<HTMLDivElement>(null)
  const width = useWidth(ref)
  const [hover, setHover] = React.useState<number | null>(null)
  // A tap shows a commit, and a second tap on it opens it.
  const touched = React.useRef(false)

  const n = values.length
  const present = values.flatMap((v, i) => (v == null ? [] : [i]))
  const [lo, hi] = domain(present.map((i) => values[i]!))
  const step = n > 1 ? (width - 2 * pad) / (n - 1) : 0
  const x = (i: number) => (n > 1 ? pad + i * step : width / 2)
  const y = (v: number) => top + (1 - (v - lo) / (hi - lo)) * (height - top - bottom)
  const base = height - bottom

  // A line a CPU for timings, the last commit's drawn over the others'.
  const byCPU = unit === "ns/op"
  const cpuOf = (i: number) => (byCPU ? (runners[i]?.cpu ?? "") : "")
  const cpu = present.length ? cpuOf(present.at(-1)!) : ""
  const lines = new Map<string, number[]>()
  for (const i of present) lines.set(cpuOf(i), [...(lines.get(cpuOf(i)) ?? []), i])
  const others = [...lines].filter(([c]) => c !== cpu).map(([, points]) => points)
  const path = (points: number[]) => points.map((i, k) => `${k ? "L" : "M"}${x(i).toFixed(1)} ${y(values[i]!).toFixed(1)}`).join("")

  /** The commit with a value nearest to index i. */
  const nearest = (i: number) => {
    let best = present[0]
    for (const p of present) if (Math.abs(p - i) < Math.abs(best! - i)) best = p
    return best ?? null
  }
  const pointAt = (clientX: number) => {
    const rect = ref.current!.getBoundingClientRect()
    return nearest(step ? Math.round((clientX - rect.left - pad) / step) : 0)
  }
  const open = (i: number | null) => {
    if (i != null && commits[i]) window.open(commitUrl(commits[i].sha), "_blank", "noopener,noreferrer")
  }
  const onKeyDown = (e: React.KeyboardEvent) => {
    const at = hover ?? present.at(-1) ?? null
    if (at == null) return
    const k = present.indexOf(at)
    const to = { ArrowLeft: present[k - 1], ArrowRight: present[k + 1], Home: present[0], End: present.at(-1) }[e.key]
    if (to !== undefined) {
      e.preventDefault()
      setHover(to)
    } else if (e.key === "Enter") open(at)
  }

  const h = hover != null && values[hover] != null ? hover : null
  const runner = h != null ? runners[h] : null
  const moved = h != null ? steps.find((st) => st.index === h) : undefined

  return (
    <div ref={ref} className="relative">
      <svg
        width={width}
        height={height}
        role="img"
        aria-label={label}
        tabIndex={0}
        className="block cursor-pointer touch-pan-y rounded-sm outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
        onPointerDown={(e) => (touched.current = e.pointerType !== "mouse")}
        onPointerMove={(e) => setHover(pointAt(e.clientX))}
        onPointerLeave={(e) => e.pointerType === "mouse" && setHover(null)}
        onClick={(e) => {
          const i = pointAt(e.clientX)
          if (touched.current && i !== hover) setHover(i)
          else open(i)
        }}
        onFocus={() => setHover(present.at(-1) ?? null)}
        onBlur={() => setHover(null)}
        onKeyDown={onKeyDown}
      >
        {[lo, (lo + hi) / 2, hi].map((v, k) => (
          <g key={k}>
            <line x1={0} x2={width} y1={y(v)} y2={y(v)} className="stroke-border" strokeWidth={1} />
            {k !== 1 && (
              <text x={0} y={y(v) - 4} className="fill-muted-foreground text-[10px] tabular-nums">
                {formatValue(v, unit)}
              </text>
            )}
          </g>
        ))}
        {others.map((points) => (
          <Line key={points[0]} points={points} d={path(points)} x={x} y={(i) => y(values[i]!)} className="stroke-muted-foreground/45" dot="fill-muted-foreground/45" />
        ))}
        {lines.has(cpu) && (
          <Line points={lines.get(cpu)!} d={path(lines.get(cpu)!)} x={x} y={(i) => y(values[i]!)} className="stroke-gopher-ink" dot="fill-gopher-ink" width={2} />
        )}
        {steps.map((st) =>
          values[st.index] == null ? null : <Mark key={st.index} cx={x(st.index)} cy={y(values[st.index]!)} worse={st.change > 0} />
        )}
        {present.length > 0 && h == null && <Dot cx={x(present.at(-1)!)} cy={y(values[present.at(-1)!]!)} />}
        {h != null && (
          <>
            <line x1={x(h)} x2={x(h)} y1={top} y2={base} className="stroke-foreground/40" strokeWidth={1} />
            <Dot cx={x(h)} cy={y(values[h]!)} />
          </>
        )}
      </svg>
      <div className="mt-1.5 flex justify-between gap-3 text-[10px] text-muted-foreground tabular-nums">
        <span>{commits[0] && formatDate(commits[0].date)}</span>
        {others.length > 0 && (
          <span className="flex min-w-0 items-center gap-3">
            <span className="flex min-w-0 items-center gap-1">
              <span className="h-0.5 w-3 shrink-0 rounded-full bg-gopher-ink" />
              <span className="truncate">{shortCPU(cpu)}</span>
            </span>
            <span className="flex shrink-0 items-center gap-1">
              <span className="h-0.5 w-3 rounded-full bg-muted-foreground/45" />
              other CPUs
            </span>
          </span>
        )}
        <span>{commits.at(-1) && formatDate(commits.at(-1)!.date)}</span>
      </div>
      {h != null && commits[h] && (
        <div
          className="pointer-events-none absolute top-0 z-10 w-max max-w-[min(17rem,80%)] rounded-md border bg-popover px-2.5 py-2 text-xs leading-5 text-popover-foreground shadow-md"
          style={x(h) < width / 2 ? { left: x(h) + 10 } : { right: width - x(h) + 10 }}
        >
          <div className="text-sm font-semibold tabular-nums">{formatValue(values[h]!, unit)}</div>
          <div className="truncate text-muted-foreground">
            <span className="font-mono text-foreground">{commits[h].sha.slice(0, 7)}</span> {commits[h].message}
          </div>
          <div className="text-muted-foreground">
            {formatDate(commits[h].date, true)}
            {runner && ` · ${runner.cpu}`}
          </div>
          {moved && (
            <div className={cn("font-medium", moved.change > 0 ? "text-worse" : "text-better")}>
              {moved.change > 0 ? "▲" : "▼"} {formatChange(moved.change)}: {formatValue(moved.before, unit)} before, {formatValue(moved.after, unit)} after
            </div>
          )}
        </div>
      )}
    </div>
  )
}

/** A line through points, or a dot for one. */
function Line({
  points,
  d,
  x,
  y,
  className,
  dot,
  width = 1.5,
}: {
  points: number[]
  d: string
  x: (i: number) => number
  y: (i: number) => number
  className: string
  dot: string
  width?: number
}) {
  if (points.length === 1) return <circle cx={x(points[0]!)} cy={y(points[0]!)} r={2} className={dot} />
  return <path d={d} fill="none" className={className} strokeWidth={width} strokeLinejoin="round" strokeLinecap="round" />
}

/** The end dot, ringed with the surface. */
function Dot({ cx, cy }: { cx: number; cy: number }) {
  return <circle cx={cx} cy={cy} r={4} className="fill-gopher-ink stroke-background" strokeWidth={2} />
}

/** A step's triangle, pointing up for worse, down for better, ringed with the surface. */
function Mark({ cx, cy, worse }: { cx: number; cy: number; worse: boolean }) {
  const d = worse ? `M${cx} ${cy - 6}l5.5 9.5h-11z` : `M${cx} ${cy + 6}l5.5 -9.5h-11z`
  return <path d={d} className={worse ? "fill-worse stroke-background" : "fill-better stroke-background"} strokeWidth={1.5} strokeLinejoin="round" />
}

/**
 * The axis of values: their range with some room, a fifth of their median
 * at least, from zero when it would start near it.
 */
function domain(values: number[]): [number, number] {
  if (!values.length) return [0, 1]
  const min = Math.min(...values)
  const max = Math.max(...values)
  const span = Math.max((max - min) * 1.2, median(values) * 0.2) || 1
  const lo = (min + max) / 2 - span / 2
  if (lo < span * 0.25) return [0, Math.max(span, max * 1.1)]
  return [lo, lo + span]
}

function useWidth(ref: React.RefObject<HTMLElement | null>) {
  const [width, setWidth] = React.useState(320)
  React.useLayoutEffect(() => {
    const el = ref.current
    if (!el) return
    setWidth(el.clientWidth)
    const observer = new ResizeObserver(([entry]) => entry && setWidth(entry.contentRect.width))
    observer.observe(el)
    return () => observer.disconnect()
  }, [ref])
  return width
}
