# The mygo CLI

`mygo` creates, develops, packages and publishes MyGo apps.

## Install

Projects with a web frontend depend on the CLI as the `mygo-cli` npm
package, which holds a prebuilt binary for macOS, Linux and Windows on
arm64 and x64, and run it from their scripts:

```sh
bun add -d mygo-cli    # or: npm install -D mygo-cli
bun run dev            # "dev": "mygo dev" in package.json
bunx mygo doctor       # any command, in the project
```

Outside a project, run `bunx mygo-cli` or `npx mygo-cli`, e.g.
`bunx mygo-cli init my-app`: npm has an unrelated package named `mygo`, so
`bunx mygo` would run it where mygo-cli is not installed. `npm install -g
mygo-cli` puts `mygo` on your `PATH`. `MYGO_CLI_BINARY` makes the package
run another build of the CLI.

Projects of [native UI](ui/README.md) have no npm packages: their `go.mod` has
the CLI as a [tool](https://go.dev/doc/modules/managing-dependencies#tools),
which `go tool mygo` runs at the version it pins.

The CLI is a Go program, which Go runs too, without installing it:

```sh
go run github.com/egoist/mygo/cmd/mygo@latest init my-app
```

`go install` puts `mygo` on your `PATH`:

```sh
go install github.com/egoist/mygo/cmd/mygo@latest
```

Either way, building apps needs Go, which compiles them.

The commands that take a project directory, `[dir]`, default to the
current directory.

## mygo init

```
mygo init [flags] <dir>
```

Creates a project in a new or empty directory: a Go module and a
TypeScript frontend built with Vite, side by side, a default icon in
`resources/icon.png`, and a package.json with the scripts `dev`, `build`
and `generate`. It installs the dependencies with Bun and generates the
TypeScript client. See [a web frontend](getting-started.md#a-web-frontend).

With `-template native` the project is a Go module alone, whose window
shows [native UI](ui/README.md): `main.go`, a test of its view, `mygo.json` and the
icon. The module has the CLI as a [tool](https://go.dev/doc/modules/managing-dependencies#tools),
so `go tool mygo dev` and `go tool mygo build` run the version it pins,
without Bun. See [native UI](getting-started.md#native-ui).

Both templates include the `mygo-maintenance` agent skill in
`.agents/skills/mygo-maintenance/`, with guidance on element lifetimes,
stable identity, threading, and testing when maintaining the app.

| Flag | |
|---|---|
| `-template` | `web`, a TypeScript frontend (the default), or `native`, native UI in Go |
| `-name` | the app's name (default: the directory's name) |
| `-module` | the Go module path (default: the directory's name) |
| `-mygo` | a checkout of MyGo to use, through a `replace` directive, instead of the released module; the scripts then run the checkout's CLI with `go run`; run `bun install && bun run build` in the checkout first, for the web template |

## mygo install-skills

```sh
mygo install-skills [dir]
```

Installs or updates the agent skills bundled with the CLI version you run
in an existing project's `.agents/skills/`. The directory defaults to the
current directory; no project configuration, Go build, or frontend tools
are needed. New projects receive these skills through `mygo init` too.

The command overwrites bundled files such as
`mygo-maintenance/SKILL.md` and `mygo-maintenance/agents/openai.yaml`,
including local edits to those files. Other skills and extra custom files
are preserved. Upgrade the CLI, then run the command again to refresh its
guidance.

## mygo dev

```
mygo dev [flags] [dir]
```

Develops the app with live reload. It writes the TypeScript client, runs
`devCommand` from the configuration, such as a Vite dev server, waits for
`devUrl` to answer, then builds a development app, which loads `devUrl` in
place of its built frontend, and starts it. Without `devUrl` the app serves
`frontendDist` from disk.

Changes to the Go code, the configuration, the icon or the resources rebuild
the app, regenerate the TypeScript client and restart the app: the running
build quits, then the new one starts, so the two never hold the same files,
locks or web view profile at once. A build that fails to compile keeps the
running one. Frontend changes are the dev server's to handle. Quitting the app, or
Ctrl+C, ends mygo dev, and `App.Relaunch` restarts the app.

The development app is named `<name> Dev`, with the identifier
`<identifier>.dev`, so that its data, preferences and single instance lock
stay apart from the installed app's. On macOS it is a real app bundle, in
`.mygo/dev`; on Windows its executable carries the icon, manifest and
version information that `mygo build` embeds.

| Flag | |
|---|---|
| `-skip-dev-command` | does not run `devCommand`, e.g. when the dev server already runs |
| `-sign` | the identity that signs the development app on macOS (default: `-`, ad hoc) |

## mygo build

```
mygo build [flags] [dir]
```

Builds production apps: runs `buildCommand`, compiles the app with
`frontendDist` embedded, and packages it for each platform in
`<out>/<os>-<arch>`. See [Building and distributing](distribution.md).

| Flag | |
|---|---|
| `-platform` | comma separated `GOOS/GOARCH` targets, e.g. `darwin/universal,windows/amd64,linux/amd64` (default: this machine's) |
| `-debug` | keeps development features, such as the web inspector and the [inspector of native UI](ui/inspector.md) |
| `-o` | the output directory (default: `out` of the configuration) |
| `-sign` | the macOS signing identity (default: `macos.signingIdentity`) |
| `-skip-build-command` | does not run `buildCommand` |
| `-skip-dmg` | does not make macOS disk images |
| `-skip-notarize` | does not notarize, even with `macos.notarize` set |
| `-upload` | uploads the installers and updates to a draft GitHub release of the version (`updates.github`), with `gh`, or to the bucket of `updates.s3` |

## mygo generate

```
mygo generate [-o file] [dir]
```

Writes the typed TypeScript client for the services bound with `mygo.Bind`
and the events declared with `mygo.NewEvent`, to `bindings` of the
configuration unless `-o` names another file. `mygo dev` and `mygo build` run
it for you; apps without a frontend get no client. See
[the generated client](bindings.md#the-generated-client).

## mygo keygen

```
mygo keygen [flags]
```

Creates the key pair that signs [updates](updates.md): `mygo-update.key`,
the secret, and `mygo-update.pub`, and prints the `updates` section to add
to the configuration.

| Flag | |
|---|---|
| `-o` | the directory to write the keys to (default: `mygo/update-keys` in the user's configuration directory) |
| `-force` | replaces existing keys |

## mygo doctor

Checks the development machine: Go, Bun and the webview of the platform,
which projects with a web frontend need, and the tools of optional
features: NSIS for Windows installers (which `mygo build` downloads on
Windows), `signtool` or `osslsigncode` for signing Windows apps, `gh` for
`-upload`, and Developer ID identities for signing macOS apps.

## mygo version

Prints the version of the CLI.

## Environment variables

| Variable | |
|---|---|
| `CGO_ENABLED` | `0` by default when unset or empty; set to `1` for app dependencies that need cgo, including in `mygo dev`, `mygo build` and `mygo generate` |
| `MYGO_INSPECTOR` | `1` keeps the [inspector of native UI](ui/inspector.md) in production builds of `mygo build`, which leave it out |
| `MYGO_UPDATER_PRIVATE_KEY` | the secret key that signs updates, for `mygo build` |
| `MYGO_WINDOWS_CERTIFICATE_PASSWORD` | the password of `windows.certificate`, for `mygo build` |
| `MYGO_CLI_BINARY` | a build of the CLI for the `mygo-cli` package to run |
| `MYGO_ENV` | `production` makes an app behave like a production build, e.g. without the web inspector |

For an app with dependencies that need cgo:

```sh
CGO_ENABLED=1 go tool mygo dev
CGO_ENABLED=1 go tool mygo build
```
