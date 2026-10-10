# AGENTS.md

MyGo is a desktop application framework in Go. Windows show web pages in the
system webview (WKWebView on macOS, WebKitGTK on Linux, WebView2 on Windows),
with typed Go ↔ TypeScript IPC, or native UI that MyGo draws itself (package
`ui`).

**Read [docs/architecture.md](docs/architecture.md) before changing code.** It
describes the layers, the threading model, native interop without cgo, the IPC
wire protocol, TypeScript generation, and the checklist for adding features.

## Hard rules

- **No cgo.** Everything must build with `CGO_ENABLED=0`. Call native code
  through purego (`internal/darwin`, `internal/linux`) or `syscall`
  (`internal/windows`), never `import "C"`.
- **Bun is dev tooling only.** The repository is a Bun workspace
  (`packages/bridge`, `packages/runtime`, `packages/cli` with its platform
  packages, the plugins' packages in `plugins/`, and examples with a frontend, which keep it at their root like
  the project template, and `website`); Bun also installs and runs the template's scripts
  (Vite, the mygo-cli package). Nothing in an app may need Bun at run time.
- **Scope:** system webview on macOS, Linux and Windows (WebView2), and
  native UI that MyGo draws itself (`WindowOptions.Content`, package `ui`;
  see "Native UI" in the architecture guide). The bundled CEF option is out
  of scope until asked. Other platforms must keep compiling through
  `internal/unsupported`.
- **Great DX over Electron parity.** Keep the familiar feel (app lifecycle,
  windows, menus) but design Go-first APIs: typed IPC via `mygo.Bind` and
  `mygo.NewEvent[T]` with the generated client, not string channels.
- **Threading.** Backend methods and handler callbacks run on the main thread.
  Public methods must be safe from any goroutine (`onMain`/`postMain`/`await`
  in `loop.go`). Never block the main thread waiting on work that needs it.
- **purego callbacks are a scarce, never-freed resource.** Create them once
  per signature at startup and route by user data, never per window or call.

## Commands

```sh
go build ./... && GOOS=linux go build ./... && GOOS=windows go build ./...  # all must pass
go vet ./... && GOOS=linux go vet ./... && GOOS=windows go vet ./...
go vet -tags mygo_noinspector ./...        # production builds, without the inspector of native UI
go test ./...                              # core (fake backend), tsgen, CLI, accelerator
MYGO_E2E=1 go test ./internal/e2e          # real GUI tests (macOS desktop session)
bun install && bun run test && bun run typecheck   # the workspace, from the root
bun run build                              # build internal/bridge/bridge.js and the npm packages' dist/
bun run --cwd packages/cli binaries        # cross-compile the CLI into the mygo-cli platform packages
go run ./cmd/mygo generate examples/todo   # regenerate an example's TypeScript client
go run ./cmd/mygo dev examples/todo        # live reload (dev bundle in examples/todo/.mygo)
go run ./cmd/mygo build examples/todo      # .app + .dmg in examples/todo/build
go run ./examples/gallery                  # the native UI toolkit's tour
go run ./examples/terminal                 # a terminal (downloads libghostty-vt once)
go generate ./plugins/terminal             # build libghostty-vt for every platform (with Zig), write Ghostty's themes
go generate ./internal/gpu/d3d11           # recompile the Direct3D shader (on Windows)
go generate ./internal/gpu/metal           # recompile the Metal shader (on macOS, with Xcode)
go generate ./plugins/glass                # recompile the glass plugin's shaders (on macOS, and on Windows)
bun run --cwd website dev                  # the website, with docs/ at /docs (see website/README.md)
bun scripts/bench.ts run [--e2e]           # benchmarks and app sizes, medians of 6 runs (CI: bench.yml, /benchmarks)
go run ./internal/idlemem app...           # apps' memory once idle, with their webviews' processes
```

Linux GUI tests cross-compile and run in a container with WebKitGTK and Xvfb;
Windows GUI tests need Windows with the WebView2 Runtime (a GitHub Actions
`windows-latest` runner has it). See "Testing" in the architecture guide.

## Conventions

- Never write `[skip ci]` (or `[ci skip]`, `[no ci]`) in a commit message,
  not even quoted in its body: GitHub skips the push's workflows, CI and
  benchmarks included, and Cloudflare the website's deploy. Only the
  benchmarks branch's commits say it.
- New behavior goes into `package mygo` first; backends only translate
  `internal/platform` calls to native ones. Implement every platform method in
  `darwin`, `linux`, `windows`, `fake` and `unsupported`.
- Objective-C: `alloc`/`init` objects are owned and must be released; wrap
  temporary objects in `withPool`; copy blocks you call later.
- Official plugins live in `plugins/<name>`: the Go package and its npm
  package (`@mygo-plugins/<name>`) side by side, released with
  mygo-runtime's version. The updater plugin is Go only; its `native`
  package draws the update window in native UI. The terminal plugin is Go
  only too, for native UI, with libghostty-vt loaded through purego: its
  `mygo-plugin.json` pins the builds the CLI puts into apps, published as
  release assets (`libghostty-vt-<commit>`); change it with `go generate`. The glass
  plugin is Go only too, an effect of the renderers (`scene.Effect`): its
  shader in Metal, HLSL and GLSL with a CPU twin that must draw the same
  pixels, compiled ahead of time by `go generate` on macOS and Windows.
- The `dist/` of npm packages is not committed: run `bun run build` after
  `bun install` (CI and releases do). Commit `internal/bridge/bridge.js`
  whenever its sources change, and keep the generated clients of examples
  (`examples/*/src/mygo.ts`) up to date. Built frontends are not
  committed: `mygo build` embeds them.
- `mygo-runtime` (`packages/runtime`) and `mygo-cli` (`packages/cli`, with
  its per-platform packages in `packages/cli/npm`) are released to npm with
  the same version as the Go module (`mygo.Version`, also the CLI's
  `version`), which `bun scripts/version.ts <version>` sets everywhere; the
  template depends on both. Pushing a `v<version>` tag releases everything
  (`.github/workflows/release.yml`, see "Releasing" in the architecture
  guide); never commit the CLI's binaries.
- The configuration types of `packages/cli/index.d.ts` (for `mygo.config.ts`)
  mirror `Config` in `cmd/mygo/config.go`; `TestConfigTypes` checks them.
- Tests: unit tests through `internal/fake`; GUI behavior in `internal/e2e`.
