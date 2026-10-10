import { Link, createFileRoute } from "@tanstack/react-router"
import { ArrowDownIcon, ArrowUpIcon, ChevronDownIcon, InfoIcon, TableOfContentsIcon } from "lucide-react"
import * as React from "react"

import { TrendChart } from "@/components/benchmarks/trend-chart"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import {
  anchorOf,
  appDocs,
  commitUrl,
  dataUrl,
  formatChange,
  formatDate,
  formatValue,
  getBenchmarkDocs,
  groups,
  metrics,
  platforms,
  ranges,
  findSteps,
  lastValue,
  reading,
  trendOf,
  unitLabel,
  workflowUrl,
  type BenchmarkData,
  type Group,
  type Metric,
  type Platform,
  type Commit,
  type Runner,
  type Step,
  type Trend,
} from "@/lib/benchmarks"
import { site } from "@/lib/site"
import { cn } from "@/lib/utils"

const title = "Benchmarks · MyGo"
const description = "MyGo's benchmarks on macOS, Linux and Windows, measured on every push to main: IPC through the webview, windows opening, native UI frames, text layout, app size and idle memory."

export const Route = createFileRoute("/benchmarks")({
  loader: () => getBenchmarkDocs(),
  staleTime: Infinity,
  head: () => ({
    meta: [
      { title },
      { name: "description", content: description },
      { property: "og:title", content: title },
      { property: "og:description", content: description },
    ],
  }),
  component: Benchmarks,
})

type Load = { state: "loading" } | { state: "missing" } | { state: "error"; error: string } | { state: "ready"; data: BenchmarkData }

interface View {
  os: Platform
  metric: Metric
  range: number
}

const defaultView: View = { os: "darwin", metric: "time", range: 100 }

/** The view the page's URL asks for, such as ?os=linux&metric=allocs&commits=300. */
function viewOf(search: string): View {
  const q = new URLSearchParams(search)
  const os = platforms.find((p) => p.id === q.get("os"))?.id ?? defaultView.os
  const metric = metrics.find((m) => m.id === q.get("metric"))?.id ?? defaultView.metric
  const range = ranges.find((r) => String(r) === q.get("commits")) ?? defaultView.range
  return { os, metric, range }
}

function Benchmarks() {
  const docs = Route.useLoaderData()
  const [load, setLoad] = React.useState<Load>({ state: "loading" })
  const [view, setView] = React.useState(defaultView)

  // The results come from the benchmarks branch, newer than the page.
  React.useEffect(() => {
    setView(viewOf(window.location.search))
    const controller = new AbortController()
    fetch(dataUrl, { signal: controller.signal })
      .then(async (res) => {
        // Until the workflow's first run, the branch has no results.
        if (res.status === 404) return setLoad({ state: "missing" })
        if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
        setLoad({ state: "ready", data: (await res.json()) as BenchmarkData })
      })
      .catch((err: Error) => !controller.signal.aborted && setLoad({ state: "error", error: err.message }))
    return () => controller.abort()
  }, [])

  const update = (change: Partial<View>) => {
    const next = { ...view, ...change }
    setView(next)
    const q = new URLSearchParams()
    if (next.os !== defaultView.os) q.set("os", next.os)
    if (next.metric !== defaultView.metric) q.set("metric", next.metric)
    if (next.range !== defaultView.range) q.set("commits", String(next.range))
    const search = q.size ? `?${q}` : ""
    window.history.replaceState(window.history.state, "", `${window.location.pathname}${search}${window.location.hash}`)
  }

  const sections = load.state === "ready" ? sectionsOf(load.data, view) : []

  return (
    <main className="flex-1">
      <section className="border-b px-4 pt-16 pb-12 sm:px-10 md:pt-20">
        <p className="label">Benchmarks</p>
        <h1 className="mt-5 text-3xl leading-[1.1] font-semibold tracking-[-0.035em] sm:text-4xl">Every push, measured.</h1>
        <p className="mt-5 max-w-2xl text-base leading-7 text-muted-foreground">
          Every push to main runs on GitHub&apos;s macOS, Linux and Windows runners. Each point is a commit, the median of six runs: lower is better.
        </p>
        <p className="mt-6 flex flex-wrap items-center gap-x-6 gap-y-2 text-sm">
          <Info
            label="How to read the charts"
            info={reading}
            trigger={
              <button
                type="button"
                className="inline-flex items-center gap-1.5 font-medium underline decoration-gopher/45 underline-offset-[5px] hover:decoration-gopher"
              />
            }
          >
            <InfoIcon className="size-3.5 text-muted-foreground" />
            Reading the charts
          </Info>
          <a href={workflowUrl} className="font-medium underline decoration-gopher/45 underline-offset-[5px] hover:decoration-gopher">
            The workflow
          </a>
          <a href={`${site.repo}/tree/benchmarks`} className="font-medium underline decoration-gopher/45 underline-offset-[5px] hover:decoration-gopher">
            The data
          </a>
          <Link
            to="/docs/$"
            params={{ _splat: "architecture" }}
            hash="benchmarks"
            className="font-medium underline decoration-gopher/45 underline-offset-[5px] hover:decoration-gopher"
          >
            Running them yourself
          </Link>
        </p>
      </section>

      {/* Sticky where the controls fit on a line, under the header's border:
          the line above them is the section's. */}
      <div className="z-30 flex flex-wrap items-center gap-x-7 gap-y-3 border-b bg-background/85 px-4 py-3 backdrop-blur-xl sm:px-10 lg:sticky lg:top-14">
        <Segmented label="Platform" options={platforms} value={view.os} onChange={(os) => update({ os })} />
        <Segmented label="Metric" options={metrics} value={view.metric} onChange={(metric) => update({ metric })} />
        <Segmented
          label="Commits"
          options={ranges.map((r) => ({ id: r, label: String(r) }))}
          value={view.range}
          onChange={(range) => update({ range })}
        />
        <JumpMenu sections={sections} />
      </div>

      {load.state === "loading" && <Message>Loading the results…</Message>}
      {load.state === "missing" && (
        <Message>
          No results yet: they show once{" "}
          <a href={workflowUrl} className="text-foreground underline underline-offset-4">
            the workflow
          </a>{" "}
          has measured a commit.
        </Message>
      )}
      {load.state === "error" && (
        <Message>
          The results could not be loaded ({load.error}). They are at{" "}
          <a href={dataUrl} className="text-foreground underline underline-offset-4">
            latest.json
          </a>
          .
        </Message>
      )}
      {load.state === "ready" && (
        <Results data={load.data} sections={sections} docs={docs} view={view} onMetric={(metric) => update({ metric })} />
      )}
    </main>
  )
}

/** A group of benchmarks the page shows, with each one's values in the unit it shows. */
interface Section {
  group: Group
  unit: string
  rows: { name: string; values: (number | null)[] }[]
}

/** The groups with results on the platform in the metric of the view, and, after the groups the page knows, any other package's. */
function sectionsOf(data: BenchmarkData, view: View): Section[] {
  const series = data.series[view.os]
  if (!series) return []
  const unit = metrics.find((m) => m.id === view.metric)!.unit
  const known = new Set(groups.map((g) => g.pkg))
  const shown: Group[] = [...groups, ...Object.keys(series).filter((pkg) => !known.has(pkg)).map((pkg) => ({ pkg, title: pkg, text: "" }))]
  return shown.flatMap((group) => {
    const u = group.unit ?? unit
    const rows = Object.entries(series[group.pkg] ?? {})
      .filter(([, units]) => units[u]?.some((v) => v != null))
      .map(([name, units]) => ({ name, values: units[u]! }))
    return rows.length ? [{ group, unit: u, rows }] : []
  })
}

/**
 * Scrolls to a benchmark's chart, or its row where its group shows a table,
 * focuses it, and flashes it. At once: the flash would be over before a
 * smooth scroll across the page got there.
 */
function jumpTo(pkg: string, name: string) {
  const id = anchorOf(pkg, name)
  const el = document.getElementById(id) ?? document.getElementById(`${id}-row`)
  if (!el) return
  el.scrollIntoView({ block: "start" })
  el.focus({ preventScroll: true })
  const gopher = getComputedStyle(el).getPropertyValue("--gopher").trim()
  el.animate([{ backgroundColor: `color-mix(in oklch, ${gopher} 14%, transparent)` }, { backgroundColor: "transparent" }], {
    duration: 1600,
    easing: "ease-out",
  })
}

/** A menu of every benchmark shown, by group, with its last value, which scrolls to the one chosen. */
function JumpMenu({ sections }: { sections: Section[] }) {
  // Chosen in the menu, scrolled to once it has closed and unlocked the
  // page's scrolling, and focused in place of the button.
  const chosen = React.useRef<[string, string] | null>(null)
  return (
    <DropdownMenu
      onOpenChangeComplete={(open) => {
        if (open || !chosen.current) return
        jumpTo(...chosen.current)
        chosen.current = null
      }}
    >
      <DropdownMenuTrigger disabled={!sections.length} render={<Button variant="outline" className="ml-auto" />}>
        <TableOfContentsIcon />
        Jump to
        <ChevronDownIcon className="text-muted-foreground" />
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        className="max-h-[min(var(--available-height),32rem)] w-80"
        finalFocus={() => !chosen.current}
      >
        {sections.map((section, i) => (
          <React.Fragment key={section.group.pkg}>
            {i > 0 && <DropdownMenuSeparator />}
            <DropdownMenuGroup>
              <DropdownMenuLabel>{section.group.title}</DropdownMenuLabel>
              {section.rows.map(({ name, values }) => {
                const latest = lastValue(values)
                return (
                  <DropdownMenuItem key={name} onClick={() => (chosen.current = [section.group.pkg, name])}>
                    <span className="truncate font-mono text-[13px]">{name}</span>
                    {latest !== undefined && (
                      <span className="ml-auto pl-4 text-xs text-muted-foreground tabular-nums">{formatValue(latest, section.unit)}</span>
                    )}
                  </DropdownMenuItem>
                )
              })}
            </DropdownMenuGroup>
          </React.Fragment>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

/** A step of a benchmark's series in a unit, as the summary lists them. */
interface Moved {
  pkg: string
  name: string
  unit: string
  metric?: (typeof metrics)[number]
  step: Step
}

function Results({
  data,
  sections,
  docs,
  view,
  onMetric,
}: {
  data: BenchmarkData
  sections: Section[]
  docs: Record<string, string>
  view: View
  onMetric: (metric: Metric) => void
}) {
  const platform = platforms.find((p) => p.id === view.os)!
  const series = data.series[view.os]
  const runners = data.runners[view.os] ?? []
  if (!series) return <Message>No results on {platform.label} yet.</Message>

  const from = Math.max(0, data.commits.length - view.range)
  const commits = data.commits.slice(from)
  const windowRunners = runners.slice(from)

  let last = runners.length - 1
  while (last >= 0 && !runners[last]) last--
  const latest = data.commits[last]
  const runner = runners[last]

  // The commits shown that moved results, newest first, with their steps,
  // the worse first, in every unit: a step stays listed however many
  // commits came after it.
  const byCommit = new Map<number, Moved[]>()
  for (const [pkg, benchmarks] of Object.entries(series)) {
    for (const [name, units] of Object.entries(benchmarks)) {
      for (const [unit, values] of Object.entries(units)) {
        for (const step of findSteps(values, unit, runners)) {
          if (step.index < from) continue
          const moved = byCommit.get(step.index) ?? []
          moved.push({ pkg, name, unit, metric: metrics.find((m) => m.unit === unit), step })
          byCommit.set(step.index, moved)
        }
      }
    }
  }
  const moving = [...byCommit].sort(([a], [b]) => b - a)
  for (const [, moved] of moving) {
    moved.sort((a, b) => Number(b.step.change > 0) - Number(a.step.change > 0) || Math.abs(b.step.change) - Math.abs(a.step.change))
  }

  const jump = (pkg: string, name: string, metric?: Metric) => {
    if (metric) onMetric(metric)
    // Once the page shows the metric.
    requestAnimationFrame(() => jumpTo(pkg, name))
  }

  return (
    <>
      {latest && runner && (
        <section className="grid border-b md:grid-cols-[minmax(0,1fr)_minmax(0,1.6fr)] md:divide-x">
          <div className="px-4 py-8 sm:px-10">
            <p className="label">Last measured on {platform.label}</p>
            <a href={commitUrl(latest.sha)} className="group mt-3 block">
              <span className="font-mono text-sm text-gopher-ink">{latest.sha.slice(0, 7)}</span>{" "}
              <span className="font-medium group-hover:underline group-hover:underline-offset-4">{latest.message}</span>
            </a>
            <p className="mt-2 text-sm text-muted-foreground">
              {formatDate(latest.date, true)} · {runner.cpu} · {runner.go}
            </p>
          </div>
          <div className="min-w-0 border-t px-4 py-8 sm:px-10 md:border-t-0">
            <p className="label">Moved in the last {commits.length} commits</p>
            {moving.length ? (
              <ol className="mt-3 space-y-5">
                {moving.slice(0, 5).map(([index, moved]) => {
                  const commit = data.commits[index]!
                  return (
                    <li key={index}>
                      <a href={commitUrl(commit.sha)} className="group flex min-w-0 items-baseline gap-2 text-sm">
                        <span className="font-mono text-gopher-ink">{commit.sha.slice(0, 7)}</span>
                        <span className="truncate font-medium group-hover:underline group-hover:underline-offset-4">{commit.message}</span>
                        <span className="ml-auto shrink-0 pl-2 text-xs text-muted-foreground">{formatDate(commit.date)}</span>
                      </a>
                      <ul className="mt-2 flex flex-wrap gap-2">
                        {moved.slice(0, 6).map((m) => (
                          <li key={`${m.pkg}/${m.name}/${m.unit}`}>
                            <button
                              type="button"
                              onClick={() => jump(m.pkg, m.name, m.metric?.id)}
                              title={`${formatValue(m.step.before, m.unit)} → ${formatValue(m.step.after, m.unit)}`}
                              className="inline-flex items-center gap-2 rounded-md border px-2.5 py-1 text-sm transition-colors hover:bg-muted"
                            >
                              <Change change={m.step.change} significant />
                              <span className="font-mono text-[13px]">{displayName(m.pkg, m.name)}</span>
                              <span className="text-muted-foreground">{unitLabel(m.unit)}</span>
                            </button>
                          </li>
                        ))}
                        {moved.length > 6 && <li className="self-center text-sm text-muted-foreground">and {moved.length - 6} more</li>}
                      </ul>
                    </li>
                  )
                })}
              </ol>
            ) : (
              <p className="mt-3 text-sm text-muted-foreground">Nothing moved beyond its noise.</p>
            )}
            {moving.length > 5 && (
              <p className="mt-4 text-sm text-muted-foreground">
                And {moving.length - 5} commits before, which the charts mark.
              </p>
            )}
          </div>
        </section>
      )}

      {sections.map(({ group, unit: u, rows }) => (
        <GroupSection key={group.pkg} group={group} unit={u}>
          {(table) =>
            table ? (
              <ResultsTable pkg={group.pkg} rows={rows} unit={u} commits={data.commits} runners={runners} from={from} />
            ) : (
              <div className="-mr-px -mb-px grid border-t sm:grid-cols-2 lg:grid-cols-3">
                {rows.map(({ name, values }) => {
                  const trend = trendOf(values, u, runners, from)
                  const doc = docs[`${group.pkg}/${name.split("/")[0]}`] ?? appDocs[`${group.pkg}/${name}`]
                  const label = `${displayName(group.pkg, name)}: ${trend ? formatValue(trend.latest, u) : "no results"}`
                  return (
                    <article
                      key={name}
                      id={anchorOf(group.pkg, name)}
                      tabIndex={-1}
                      className="flex min-w-0 flex-col gap-4 border-r border-b px-4 py-5 outline-none sm:px-6 lg:scroll-mt-4"
                    >
                      <header className="flex items-start justify-between gap-4">
                        <div className="min-w-0">
                          <h3 className="truncate font-mono text-[13px] font-medium" title={name}>
                            {name}
                          </h3>
                          {doc && (
                            <p className="mt-1 line-clamp-2 text-xs leading-5 text-muted-foreground" title={doc}>
                              {doc}
                            </p>
                          )}
                        </div>
                        {trend && (
                          <div className="shrink-0 text-right">
                            <div className="text-lg leading-6 font-semibold tracking-tight">{formatValue(trend.latest, u)}</div>
                            <Change
                              change={trend.change}
                              significant={trend.significant}
                              title={rangeHint(trend, u, commits[0]?.date)}
                              className="text-xs"
                            />
                          </div>
                        )}
                      </header>
                      <TrendChart
                        values={values.slice(from)}
                        commits={commits}
                        runners={windowRunners}
                        steps={trend?.steps.map((s) => ({ ...s, index: s.index - from })) ?? []}
                        unit={u}
                        label={label}
                      />
                    </article>
                  )
                })}
              </div>
            )
          }
        </GroupSection>
      ))}
    </>
  )
}

/** What a card's change compares. */
function rangeHint(trend: Trend, unit: string, since?: string) {
  const start = trend.start !== undefined ? ` (${formatValue(trend.start, unit)})` : ""
  return `The last value against the first ones shown${start}${since ? `, since ${formatDate(since)}` : ""}; this series moves ±${(trend.noise * 100).toFixed(1)}% on its own`
}

function GroupSection({ group, unit, children }: { group: Group; unit: string; children: (table: boolean) => React.ReactNode }) {
  const [table, setTable] = React.useState(false)
  const id = `group-${anchorOf(group.pkg, "")}`
  return (
    <section aria-labelledby={id} className="border-b">
      <div className="flex flex-wrap items-end justify-between gap-4 px-4 pt-12 pb-6 sm:px-10">
        <div className="max-w-2xl">
          <h2 id={id} className="flex items-center gap-1.5 text-xl font-semibold tracking-tight">
            {group.title}
            {group.info && (
              <Info
                label={`About ${group.title.toLowerCase()}`}
                info={group.info}
                trigger={<Button variant="ghost" size="icon-sm" aria-label={`About ${group.title.toLowerCase()}`} className="text-muted-foreground" />}
              >
                <InfoIcon />
              </Info>
            )}
          </h2>
          <p className="mt-2 text-sm leading-6 text-muted-foreground">
            {group.text}
            {group.text && " "}
            <span className="whitespace-nowrap">In {group.unitText ?? metrics.find((m) => m.unit === unit)?.text ?? unit}.</span>
          </p>
        </div>
        <Segmented
          label="View"
          hideLabel
          options={[
            { id: false, label: "Charts" },
            { id: true, label: "Table" },
          ]}
          value={table}
          onChange={setTable}
        />
      </div>
      {children(table)}
    </section>
  )
}

/** An info button, the trigger with children, and what it tells in a popover. */
function Info({
  label,
  info,
  trigger,
  children,
}: {
  label: string
  info: NonNullable<Group["info"]>
  trigger: React.ReactElement
  children: React.ReactNode
}) {
  return (
    <Popover>
      <PopoverTrigger render={trigger}>{children}</PopoverTrigger>
      <PopoverContent align="start" aria-label={label} className="w-80 text-[13px] leading-5 font-normal tracking-normal">
        <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1.5">
          {info.terms.map(([term, text]) => (
            <React.Fragment key={term}>
              <dt className="font-medium">{term}</dt>
              <dd className="text-muted-foreground">{text}</dd>
            </React.Fragment>
          ))}
        </dl>
        {info.note && <p className="mt-3 border-t pt-3 text-muted-foreground">{info.note}</p>}
      </PopoverContent>
    </Popover>
  )
}

/** The table view of a group: each benchmark's last value against the first shown, and the commits that moved it. */
function ResultsTable({
  pkg,
  rows,
  unit,
  commits,
  runners,
  from,
}: {
  pkg: string
  rows: Section["rows"]
  unit: string
  commits: Commit[]
  runners: (Runner | null)[]
  from: number
}) {
  const th = "border-b px-4 py-2.5 font-mono text-[11px] font-medium tracking-[0.08em] whitespace-nowrap text-muted-foreground uppercase sm:px-6"
  const td = "border-b px-4 py-2.5 whitespace-nowrap tabular-nums sm:px-6"
  return (
    <div className="overflow-x-auto border-t">
      <table className="w-full border-collapse text-left text-sm">
        <thead>
          <tr>
            <th className={th}>Benchmark</th>
            <th className={cn(th, "text-right")}>Last</th>
            <th className={cn(th, "text-right")}>First shown</th>
            <th className={cn(th, "text-right")}>Change</th>
            <th className={th}>Moved by</th>
          </tr>
        </thead>
        <tbody>
          {rows.map(({ name, values }) => {
            const trend = trendOf(values, unit, runners, from)
            return (
              <tr key={name} id={`${anchorOf(pkg, name)}-row`} tabIndex={-1} className="outline-none lg:scroll-mt-4">
                <td className={cn(td, "font-mono text-[13px]")}>{name}</td>
                <td className={cn(td, "text-right font-medium")}>{trend ? formatValue(trend.latest, unit) : "—"}</td>
                <td className={cn(td, "text-right text-muted-foreground")}>{trend?.start !== undefined ? formatValue(trend.start, unit) : "—"}</td>
                <td className={cn(td, "text-right")}>
                  {trend ? <Change change={trend.change} significant={trend.significant} title={rangeHint(trend, unit, commits[from]?.date)} /> : "—"}
                </td>
                <td className={td}>
                  <span className="flex gap-4">
                    {trend?.steps.map((s) => (
                      <a key={s.index} href={commitUrl(commits[s.index]!.sha)} className="inline-flex items-center gap-1.5 hover:underline">
                        <Change change={s.change} significant />
                        <span className="font-mono text-[13px] text-muted-foreground">{commits[s.index]!.sha.slice(0, 7)}</span>
                      </a>
                    ))}
                  </span>
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}

/** A relative change: worse (up) or better (down) beyond the noise of its series, with an arrow, or muted within it. */
function Change({ change, significant, title, className }: { change?: number; significant: boolean; title?: string; className?: string }) {
  if (change === undefined) return <span className={cn("text-muted-foreground", className)}>new</span>
  const text = formatChange(change)
  if (!significant) {
    return (
      <span className={cn("text-muted-foreground tabular-nums", className)} title={title}>
        {text}
      </span>
    )
  }
  const worse = change > 0
  const Icon = worse ? ArrowUpIcon : ArrowDownIcon
  return (
    <span className={cn("inline-flex items-center gap-0.5 font-medium tabular-nums", worse ? "text-worse" : "text-better", className)} title={title}>
      <Icon className="size-3.5" aria-hidden />
      {text}
      <span className="sr-only">{worse ? " worse" : " better"}</span>
    </span>
  )
}

function Segmented<T extends string | number | boolean>({
  label,
  hideLabel,
  options,
  value,
  onChange,
}: {
  label: string
  hideLabel?: boolean
  options: readonly { id: T; label: string }[]
  value: T
  onChange: (value: T) => void
}) {
  return (
    <div className="flex items-center gap-3">
      {!hideLabel && <span className="label">{label}</span>}
      <div role="radiogroup" aria-label={label} className="inline-flex rounded-lg border p-0.5">
        {options.map((o) => (
          <button
            key={String(o.id)}
            type="button"
            role="radio"
            aria-checked={o.id === value}
            onClick={() => onChange(o.id)}
            className={cn(
              "h-7 rounded-md px-2.5 text-sm transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring/60",
              o.id === value ? "bg-muted font-medium text-foreground" : "text-muted-foreground hover:text-foreground"
            )}
          >
            {o.label}
          </button>
        ))}
      </div>
    </div>
  )
}

function Message({ children }: { children: React.ReactNode }) {
  return <p className="px-4 py-24 text-center text-sm text-muted-foreground sm:px-10">{children}</p>
}

/** A benchmark as the summary names it: its package, then its name. */
function displayName(pkg: string, name: string) {
  return pkg === "." ? name : `${pkg.split("/").at(-1)}/${name}`
}
