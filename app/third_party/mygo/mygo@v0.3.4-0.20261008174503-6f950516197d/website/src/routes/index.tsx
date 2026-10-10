import { ShikiMagicMovePrecompiled } from "@shikijs/magic-move/react"
import "@shikijs/magic-move/style.css"
import { Link, createFileRoute } from "@tanstack/react-router"
import { ArrowRightIcon } from "lucide-react"
import * as React from "react"

import { InstallCommand } from "@/components/install-command"
import { buttonVariants } from "@/components/ui/button"
import { getHome } from "@/lib/home"
import { site } from "@/lib/site"
import { cn } from "@/lib/utils"

export const Route = createFileRoute("/")({
  loader: () => getHome(),
  component: Home,
})

// From the architecture guide, on macOS: a hello-world app, its memory with
// the processes WKWebView runs for it, and a small window of native UI.
const stats = [
  { value: "~7", unit: "MB", label: "Binary" },
  { value: "~44", unit: "MB", label: "Memory, native UI" },
  { value: "~62", unit: "MB", label: "Memory, webview included" },
  { value: "0", unit: "%", label: "CPU at idle" },
]

// README.md's lists, in short.
const features = [
  {
    title: "Web frontends",
    text: "Any web stack, in the webview the OS already has: WKWebView, WebKitGTK, WebView2. No browser bundled.",
    slug: "frontend",
  },
  {
    title: "Native UI",
    text: "Interfaces written in Go alone and drawn on the GPU: flexbox and grid layout, widgets, text editing, screen reader support. No webview at all.",
    slug: "ui",
  },
  {
    title: "Typed IPC",
    text: "Pages call bound Go services, stream values through channels and get typed events. The TypeScript client is generated from your Go code.",
    slug: "bindings",
  },
  {
    title: "Desktop APIs",
    text: "Windows, menus, tray, dialogs, notifications, global shortcuts, deep links, file associations and more, for windows of both kinds.",
    slug: "native",
  },
  { title: "Pure Go, no cgo", text: "Build for every platform from any machine.", slug: "distribution" },
  {
    title: "Ready to ship",
    text: "App bundles and disk images, Windows installers, Debian packages, code signing, notarization and signed auto-updates.",
    slug: "distribution",
  },
]

const kinds = [
  { id: "web", label: "Web frontend" },
  { id: "native", label: "Native UI" },
] as const

type Kind = (typeof kinds)[number]["id"]

// main.go morphs between the two kinds: what both share moves, the rest
// fades. The colors come from .shiki's variables, not the container.
const magicMove = { duration: 600, stagger: 0, containerStyle: false, animateContainer: true }

function Home() {
  const { main, ts } = Route.useLoaderData()
  const [kind, setKind] = React.useState<Kind>("web")
  // The code appears as it is, and only moves once the reader switches,
  // unless they asked for less motion.
  const [animate, setAnimate] = React.useState(false)
  return (
    <main>
      <section className="px-4 py-20 sm:px-10 md:py-28">
        <a href={`${site.repo}/releases/tag/v${__MYGO_VERSION__}`} className="label inline-flex items-center gap-2 hover:text-foreground">
          <span className="size-1.5 rounded-full bg-gopher" />v{__MYGO_VERSION__} · macOS, Linux, Windows
        </a>
        <h1 className="mt-6 text-4xl leading-[1.05] font-semibold tracking-[-0.04em] sm:text-5xl md:text-6xl">
          Desktop apps in Go.
          <span className="block text-muted-foreground">Tiny, fast, fully typed.</span>
        </h1>
        <p className="mt-6 max-w-xl text-lg leading-8 text-muted-foreground">
          One small binary for macOS, Windows and Linux. A web frontend or native UI in Go, with installers and auto-updates included.
        </p>
        <div className="mt-10 flex flex-wrap items-center gap-3">
          <InstallCommand />
          <Link to="/docs/$" params={{ _splat: "getting-started" }} className={buttonVariants({ size: "lg", className: "h-11 px-5" })}>
            Get started <ArrowRightIcon />
          </Link>
        </div>
      </section>

      <dl className="grid grid-cols-2 gap-px border-t bg-border md:grid-cols-4">
        {stats.map((stat) => (
          <div key={stat.label} className="flex flex-col-reverse justify-end gap-2 bg-background px-4 py-6 sm:px-10 sm:py-8">
            <dt className="label">{stat.label}</dt>
            <dd className="text-3xl font-semibold tracking-tight tabular-nums sm:text-4xl">
              {stat.value}
              <span className="ml-1 text-base font-normal text-muted-foreground">{stat.unit}</span>
            </dd>
          </div>
        ))}
      </dl>

      <div className="border-t bg-code">
        <div role="tablist" aria-label="Kind of window" className="flex border-b">
          {kinds.map((k) => (
            <button
              key={k.id}
              type="button"
              role="tab"
              id={`tab-${k.id}`}
              aria-selected={kind === k.id}
              aria-controls="code-panel"
              onClick={() => {
                setKind(k.id)
                setAnimate(!window.matchMedia("(prefers-reduced-motion: reduce)").matches)
              }}
              className={cn(
                "label -mb-px border-r border-b border-b-transparent px-4 py-3 transition-colors hover:text-foreground sm:px-6",
                kind === k.id && "border-b-gopher bg-background text-foreground"
              )}
            >
              {k.label}
            </button>
          ))}
        </div>
        <div role="tabpanel" id="code-panel" aria-labelledby={`tab-${kind}`} className="grid lg:grid-cols-[1.3fr_1fr] lg:divide-x">
          <figure className="min-w-0">
            <figcaption className="label border-b px-4 py-3 sm:px-6">main.go</figcaption>
            <div className="shiki overflow-x-auto px-4 py-5 sm:px-6">
              <ShikiMagicMovePrecompiled
                steps={main}
                step={kind === "web" ? 0 : 1}
                animate={animate}
                options={magicMove}
              />
            </div>
          </figure>
          {kind === "web" ? (
            <figure className="flex min-w-0 flex-col border-t lg:border-t-0">
              <figcaption className="label border-b px-4 py-3 sm:px-6">src/main.ts</figcaption>
              <div className="overflow-x-auto px-4 py-5 sm:px-6" dangerouslySetInnerHTML={{ __html: ts }} />
              <p className="mt-auto border-t px-4 py-4 text-sm leading-6 text-muted-foreground sm:px-6">
                <code className="font-mono text-foreground">./mygo</code> is generated from main.go, with the types of your structs and the docs of your methods.
              </p>
            </figure>
          ) : (
            <div className="flex min-w-0 flex-col border-t lg:border-t-0">
              <p className="label border-b px-4 py-3 sm:px-6">No webview</p>
              <div className="space-y-4 px-4 py-5 text-sm leading-6 text-muted-foreground sm:px-6">
                <p>
                  The view is a Go function of your app&apos;s state. MyGo calls it to build each frame and draws it on the GPU, with Metal, Direct3D or
                  OpenGL: no HTML, no JavaScript, no bindings.
                </p>
                <p>Its windows open at once, and an app of native UI alone needs only Go to build and no webview to run.</p>
              </div>
              <figure className="mt-auto border-t">
                <figcaption className="label border-b px-4 py-3 sm:px-6">Start a project</figcaption>
                <pre className="overflow-x-auto px-4 py-4 font-mono text-[13px] leading-[1.7] sm:px-6">
                  <span className="text-gopher-ink select-none">$ </span>
                  {"go run github.com/egoist/mygo/cmd/mygo@latest \\\n    init -template native my-app\n"}
                  <span className="text-gopher-ink select-none">$ </span>
                  {"cd my-app && go tool mygo dev"}
                </pre>
              </figure>
            </div>
          )}
        </div>
      </div>

      <ul className="grid gap-px border-t bg-border sm:grid-cols-2">
        {features.map((feature) => (
          <li key={feature.title} className="bg-background">
            <Link to="/docs/$" params={{ _splat: feature.slug }} className="group flex h-full flex-col px-4 py-8 transition-colors hover:bg-code sm:px-10">
              <h2 className="flex items-center justify-between font-semibold tracking-tight">
                {feature.title}
                <ArrowRightIcon className="size-4 text-muted-foreground transition-transform group-hover:translate-x-0.5 group-hover:text-foreground" />
              </h2>
              <p className="mt-2 max-w-md text-sm leading-6 text-muted-foreground">{feature.text}</p>
            </Link>
          </li>
        ))}
      </ul>
    </main>
  )
}
