# Configuration

`mygo.config.ts`, at the root of a project, describes the app and how the
CLI develops and builds it. Its default export is the configuration, typed
by `defineConfig` from the `mygo-cli` package that projects depend on, so
editors complete and document every field:

```ts
import { defineConfig } from "mygo-cli";

export default defineConfig({
  name: "My App",
  identifier: "com.example.myapp",
  version: "1.2.0",
  copyright: "© 2026 Example Inc.",

  devUrl: "http://localhost:5173",
  devCommand: "bun run dev:web",
  buildCommand: "bun run build:web",
  frontendDist: "dist",
  bindings: "src/mygo.ts",
  out: "build",

  resources: ["third_party/licenses"],
  urlSchemes: ["myapp"],
  fileAssociations: [{ ext: ["md"], name: "Markdown Document", mimeType: "text/markdown" }],

  updates: { publicKey: "…", github: "you/my-app" },
  macos: { signingIdentity: "Developer ID Application: Jane Doe (TEAMID)" },
  windows: { certificate: "certs/code-signing.pfx" },
  linux: { maintainer: "Jane Doe <jane@example.com>" },
});
```

Every field is optional. The configuration can also be JSON, in
[mygo.json](#mygojson), as projects of [native UI](ui/README.md) have it: they
have no JavaScript, and no frontend to develop or build, so they leave out
the fields of [the frontend](#development-and-the-frontend).

## The app

| Field | Default | |
|---|---|---|
| `name` | the directory's name | the name users see: the app, its executable, menus, installers |
| `identifier` | `com.mygo.` and the name's letters and digits, e.g. `com.mygo.myapp` | a reverse DNS name unique to the app, e.g. `com.example.myapp`; systems key preferences, permissions and registrations on it |
| `version` | `0.1.0` | the version of the app, which `App.Version()` returns and updates compare |
| `copyright` | | the copyright notice of the app, in its `Info.plist` (macOS) and file properties (Windows) |
| `icon` | `resources/icon.png` when it exists | a square PNG, ideally 1024×1024 |
| `main` | `.` | the Go package of the app, relative to the project |

## Development and the frontend

| Field | Default | |
|---|---|---|
| `devUrl` | | the dev server that `mygo dev` points the app at, an `http(s)` URL; windows load it for URLs without a scheme, such as `/` |
| `devCommand` | | runs in the project directory while `mygo dev` runs, e.g. a Vite dev server; the app starts once `devUrl` answers |
| `buildCommand` | | builds the frontend before `mygo build` compiles the app |
| `frontendDist` | | the directory of the built frontend, which `mygo build` embeds into the app; `mygo dev` serves it from disk when there is no `devUrl` |
| `bindings` | `frontend/src/mygo.ts` when there is a `frontend` directory, else `src/mygo.ts` when package.json is at the root, else `mygo.ts` when the configuration has a frontend (`devUrl`, `devCommand`, `buildCommand` or `frontendDist`); none for an app without one, such as one of [native UI](ui/README.md) | where `mygo generate` writes the TypeScript client |
| `out` | `dist` | where `mygo build` writes the builds, one directory per platform |

See [the frontend](frontend.md#how-pages-load) for how they fit together.
New projects set `out` to `build`, apart from Vite's `dist`.

## Integration

| Field | |
|---|---|
| `resources` | files and directories copied into the app of every platform, next to the contents of the `resources` directory, under their base names; see [resources](distribution.md#resources), and [platform resources](distribution.md#platform-resources) for the files of one platform |
| `urlSchemes` | the URL schemes the app opens, such as `myapp` for `myapp://…`, which reach `App.OnOpenURL`; see [deep links](app.md#deep-links) |
| `fileAssociations` | the file types the app opens, which reach `App.OnOpenFile`; see [file associations](app.md#file-associations) |

Each file association has:

| Field | |
|---|---|
| `ext` | the file name extensions, without dots: `["md", "markdown"]` |
| `name` | what the files are: `"Markdown Document"` |
| `mimeType` | their MIME type, by which Linux identifies files; without one the app defines `application/x-<app>-<ext>` |
| `role` | `Editor` (the default) or `Viewer`, on macOS |

## updates

Signed [auto-updates](updates.md). `publicKey` and one of `github` and
`url` are required.

| Field | Default | |
|---|---|---|
| `publicKey` | | the contents of `mygo-update.pub`, from `mygo keygen` |
| `github` | | a public repository, `owner/name`, whose releases hold the updates |
| `tagPrefix` | `v` | what precedes the version in release tags; another prefix, such as `desktop-v`, makes apps find the newest release tagged with it rather than the repository's latest |
| `url` | | instead of `github`, the HTTPS URL of a directory holding the updates |
| `s3` | | the bucket that serves `url`, which `mygo build -upload` uploads to: `bucket`, and optionally `prefix`, its directory, `region` (default: `AWS_REGION`, else `us-east-1`), `endpoint`, of a compatible service such as Cloudflare R2, and `pathStyle` (see [publishing to S3](updates.md#publishing-to-s3)) |
| `privateKey` | | the path of `mygo-update.key`, for `mygo build`; the `MYGO_UPDATER_PRIVATE_KEY` environment variable, holding the key, takes precedence |
| `changelog` | `CHANGELOG.md` when it exists | a Markdown file whose `## <version>` section becomes the release notes |
| `deltas` | `3` | how many earlier versions get a [delta update](updates.md#delta-updates); `0` for none |

## macos

| Field | Default | |
|---|---|---|
| `minimumSystemVersion` | `12.0` | the oldest macOS the app runs on |
| `signingIdentity` | `-`, ad hoc | the identity that signs the app and its disk image, e.g. `Developer ID Application: Jane Doe (TEAMID)` |
| `entitlements` | | a property list of entitlements to sign the app with |
| `helperEntitlements` | | property lists of entitlements for code among the resources, by its path there, e.g. `{ "bin/server": "server.entitlements.plist" }`; other code keeps the entitlements it is signed with. See [helper executables](distribution.md#helper-executables) |
| `infoPlist` | | keys added to the app's `Info.plist`, replacing MyGo's, e.g. `NSCameraUsageDescription` |
| `dmgTitle` | `name` | the volume name of the disk image |
| `notarize` | | notarizes production disk images: `keychainProfile`, the profile of `xcrun notarytool store-credentials`, and optionally `keychain`, the keychain holding it |

See [macOS](distribution.md#macos).

## windows

| Field | Default | |
|---|---|---|
| `certificate` | | a code signing certificate (`.pfx`) for the executable, the unsigned executables and libraries among the resources, and the installer; its password comes from `MYGO_WINDOWS_CERTIFICATE_PASSWORD` |
| `signCommand` | | instead of `certificate`, a command that signs a file, with `%1` for its path |
| `timestampUrl` | `http://timestamp.digicert.com` | the time stamping server of `certificate` |

See [Windows](distribution.md#windows).

## linux

| Field | Default | |
|---|---|---|
| `maintainer` | `author` in package.json, else `name` | `Name <email>`, the maintainer of the Debian package |
| `comment` | | a short description, for the desktop entry and the package |
| `categories` | `["Utility"]` | the categories of the desktop entry, which place it in application menus |
| `depends` | | Debian packages the app needs besides GTK and WebKitGTK |
| `command` | none | a command that runs the app: `/usr/bin/<command>` from the Debian package, `~/.local/bin/<command>` from install.sh |

See [Linux](distribution.md#linux).

## Computed configuration

The configuration is code, so it can compute fields: read the version from
package.json, or a signing identity from the environment of CI. The default
export may also be a function, which can be async, of the command running
(`"dev"`, `"build"`, `"generate"` or `"init"`):

```ts
import { defineConfig } from "mygo-cli";
import pkg from "./package.json" with { type: "json" };

export default defineConfig(({ command }) => ({
  name: command === "dev" ? "My App (dev)" : "My App",
  version: pkg.version,
  macos: {
    signingIdentity: process.env.MACOS_SIGNING_IDENTITY ?? "-",
  },
}));
```

## How it runs

The CLI evaluates `mygo.config.ts` with [Bun](https://bun.sh), or else
Node.js 22.6 or later, in the project directory, and checks the result.
Node.js runs TypeScript by removing the types, so the file may use type
annotations but not syntax that generates code, such as `enum`. What the
file prints goes to the terminal, apart from the CLI's output. `mygo dev`
reloads the app when the file changes, but not when files it imports do.

## mygo.json

The configuration can be JSON instead, in `mygo.json`, which needs no
JavaScript runtime to read; `mygo init -template native` makes one:

```json
{
  "name": "My App",
  "identifier": "com.example.myapp",
  "devUrl": "http://localhost:5173",
  "macos": { "signingIdentity": "Developer ID Application: Jane Doe (TEAMID)" }
}
```

It has the same fields. A project has either `mygo.config.ts` or
`mygo.json`, not both.
