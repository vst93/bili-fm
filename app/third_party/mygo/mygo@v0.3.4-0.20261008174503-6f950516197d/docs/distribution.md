# Building and distributing

`mygo build` turns a project into apps people install: an app bundle and a
disk image on macOS, an executable and an installer on Windows, an
executable, a desktop entry, an archive with its install script and a Debian
package on Linux.

```sh
bun run build        # mygo build, in a project with a web frontend
go tool mygo build   # in a project of native UI
```

It:

1. with a web frontend, writes the TypeScript client, so the frontend
   builds against the Go code, and runs `buildCommand` from the
   configuration, which builds the frontend into `frontendDist`;
2. compiles the app, with the frontend embedded when there is one, for
   production: without the web inspector, and with the name, identifier
   and version of the configuration linked in;
3. packages it for each platform in `build/<os>-<arch>/` (`out` in
   the configuration);
4. signs what it can, and with [updates](updates.md) configured, writes the
   signed update archives, and delta updates from the published versions.

## Platforms

`-platform` lists `GOOS/GOARCH` targets, by default the machine's own:

```sh
mygo build -platform darwin/universal,windows/amd64,windows/arm64,linux/amd64,linux/arm64
```

`darwin/universal` combines arm64 and amd64 in one app. MyGo needs no cgo,
so any machine compiles for every platform. Some steps need the tools of a
platform, and are skipped with a note elsewhere: macOS apps are signed and
put in disk images on macOS only, and Windows installers are made on other
systems only where [NSIS](#the-installer) is installed.

The CLI defaults to `CGO_ENABLED=0`. If an app's dependencies need cgo,
set `CGO_ENABLED=1` in the environment for `mygo dev`, `mygo build` and
`mygo generate`. Those builds need a C compiler for the target platform;
when cross-compiling, set `CC` to a suitable cross compiler.

| Platform | In `build/<os>-<arch>/` |
|---|---|
| macOS | `My App.app`, signed, and `My App 0.1.0.dmg` |
| Windows | `My App.exe`, the files of the app, and the installer `My App Setup 0.1.0.exe` |
| Linux | `my-app`, `my-app.desktop`, `my-app.png`, `my-app.xml` for the file types the app defines, the files of the app, their archive `my-app-0.1.0-linux-amd64.tar.gz` with [`install.sh`](#the-install-script), and the Debian package `my-app_0.1.0_amd64.deb` |

Other flags: `-debug` keeps development features such as the inspector,
`-skip-dmg` and `-skip-notarize` skip those steps, `-sign` overrides the
signing identity of macOS, and `-o` the output directory. See
[the CLI](cli.md#mygo-build).

## Name, icon and version

The configuration describes the app:

```ts
export default defineConfig({
  name: "My App",
  identifier: "com.example.myapp",
  version: "1.2.0",
  copyright: "© 2026 Example Inc.",
});
```

- `name` is what users see: the app bundle, the executable, menus, the
  installer.
- `identifier` is a reverse DNS name unique to the app. Systems key
  preferences, permissions and registrations on it: keep it once released.
- `version` is the app's version, which `App.Version()` returns and
  [updates](updates.md) compare.

The icon is `resources/icon.png`, or the `icon` of the configuration: a square PNG,
ideally 1024×1024. `mygo build` makes it the `.icns` of macOS, the icon
resource of the Windows executable and the icon of the Linux desktop entry.

## Resources

Files the app reads at run time, such as a database seed, a helper binary
or the images of a tray icon, ship with it. Everything in the `resources`
directory of the project is copied into the app, as are the files and
directories listed in `resources` in the configuration, under their base names:

```ts
export default defineConfig({
  resources: ["third_party/licenses", "bin/helper"],
});
```

At run time `App.Path(mygo.PathResources)` is where they are:
`Contents/Resources` in a macOS app, the executable's directory elsewhere,
and the project's `resources` directory under `go run` and `go test`:

```go
dir, _ := mygo.App.Path(mygo.PathResources)
seed := filepath.Join(dir, "seed.db")
```

`mygo dev` copies them into the development app too, and rebuilds it when
they change. Hidden files are left out.

### Platform resources

Some resources belong to one platform, such as a program built for one
system and processor. Directories of `resources` named after a platform
hold them, laid out like `resources` itself:

```
resources/
├── data/seed.db                    every app
├── darwin/                         macOS apps
├── darwin-arm64/bin/server         macOS on Apple silicon
├── darwin-amd64/bin/server         macOS on Intel
├── linux-amd64/bin/server          Linux on x86-64
├── linux-arm64/bin/server          Linux on ARM64
└── windows-amd64/bin/server.exe    Windows on x86-64
```

They are named like the targets of `-platform`: a system, `darwin`,
`linux` or `windows`, for all of its apps, or a system and an
architecture, as in `darwin-arm64` or `windows-amd64`, for one. An app
ships the platform directories of its target merged with the other
resources, so the Linux x86-64 app above has `data/seed.db` and its own
`bin/server`, and nothing of the other platforms. A path that two
directories would both install fails the build, and so does a directory
named the way other tools name platforms, such as `darwin-x64` or `macos`,
which would ship to every platform. The build also notes a target without
a directory of its own when another architecture of its system has one,
such as `linux/arm64` next to `linux-amd64`.

A universal macOS app (`darwin/universal`) ships `darwin` and
`darwin-universal`, and combines `darwin-arm64` with `darwin-amd64`:
every file of one needs its counterpart in the other, programs and
libraries become universal binaries, as `lipo` makes them, and other files
must be identical. Put what serves both architectures, such as programs
that already are universal binaries, in `darwin`.

`mygo dev` ships the platform directories of the machine it runs on.
Under `go run` and `go test`, `PathResources` is the `resources`
directory as it is, platform directories included.

### Helper executables

Programs the app runs, such as a server written in another language or a
tool like `ffmpeg`, are resources too: put the build for each platform in
its [platform directory](#platform-resources) and run it from there.

```go
dir, _ := mygo.App.Path(mygo.PathResources)
cmd := exec.Command(filepath.Join(dir, "bin", "server"))
```

The same code runs `bin\server.exe` on Windows, where `exec.Command`
finds the program of an absolute path without its extension.

`mygo build` signs them with the app, so that Gatekeeper and SmartScreen
accept them and notarization passes:

- On macOS, every Mach-O executable and library among the resources, and
  every app, framework or plug-in bundle holding some, is signed from the
  inside out with `signingIdentity`, with the hardened runtime and a
  timestamp for a Developer ID. Signed ad hoc, the default, only code
  without a signature is signed: Apple silicon runs no unsigned code.
- On Windows, with `certificate` or `signCommand`, every executable and
  DLL among the resources that has no signature is signed. Those signed by
  their publishers keep their signatures.

Under the hardened runtime, some programs need entitlements, such as
`com.apple.security.cs.allow-jit` for a JavaScript runtime that compiles
code as it runs. Code keeps the entitlements it was signed with, so the
official builds of such runtimes work as they are, and
`macos.helperEntitlements` gives code entitlements of its own, by its path
in the app's resources, such as `bin/server` for
`resources/darwin-arm64/bin/server`:

```ts
export default defineConfig({
  macos: {
    signingIdentity: "Developer ID Application: Jane Doe (TEAMID)",
    helperEntitlements: { "bin/server": "server.entitlements.plist" },
  },
});
```

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>com.apple.security.cs.allow-jit</key>
  <true/>
</dict>
</plist>
```

### Native libraries of packages

A Go package that loads a native library with purego, as the
[terminal plugin](plugins/terminal.md) loads libghostty-vt, names its
builds in a `mygo-plugin.json` next to its sources, with a file per
platform, where to download it and its SHA-256:

```json
{
  "libraries": [
    {
      "name": "libexample",
      "files": {
        "darwin-arm64": { "name": "libexample.dylib", "url": "https://…/libexample-darwin-arm64.dylib", "sha256": "…" },
        "linux-amd64": { "name": "libexample.so", "url": "https://…/libexample-linux-amd64.so", "sha256": "…" },
        "windows-amd64": { "name": "example.dll", "url": "https://…/example-windows-amd64.dll", "sha256": "…" }
      }
    }
  ]
}
```

`mygo build` and `mygo dev` find these files in the packages the app is
built from, download each library once into the user's cache
(`<cache>/mygo/natives/<sha256>/`), check its SHA-256, and install it at
the top of the app's resources under its name, signed like the other
code there; a universal macOS app gets both architectures in one file.
The package loads it from `PathResources`. Building for a platform the
package has no file for fails.

## macOS

`mygo build` makes an app bundle, signs it, and puts it in a disk image
whose window invites users to drag the app to Applications.

### Signing and notarization

Apps signed ad hoc, the default, run on the Mac that built them, but
Gatekeeper blocks them on others. To ship an app, sign it with a Developer
ID of the [Apple Developer Program](https://developer.apple.com/programs/)
and have Apple notarize it:

```ts
export default defineConfig({
  macos: {
    signingIdentity: "Developer ID Application: Jane Doe (TEAMID)",
    notarize: { keychainProfile: "notary" },
  },
});
```

Store the credentials of the notary service in the keychain once:

```sh
xcrun notarytool store-credentials notary
```

`mygo build` then signs the app with the hardened runtime, submits the disk
image to the notary service, and staples the ticket to the disk image and
to the app, which updates ship. `security
find-identity -v -p codesigning` lists your identities, and `mygo doctor`
counts the Developer IDs.

### Info.plist and entitlements

`macos.infoPlist` adds keys to the app's `Info.plist`, or replaces MyGo's,
for example the usage descriptions that pages using the camera or the
microphone need:

```ts
export default defineConfig({
  macos: {
    minimumSystemVersion: "13.0",
    infoPlist: {
      NSCameraUsageDescription: "Scan documents with the camera.",
      LSApplicationCategoryType: "public.app-category.productivity",
    },
    entitlements: "entitlements.plist",
    dmgTitle: "My App Installer",
  },
});
```

`entitlements` signs the app with a property list of entitlements, and
`helperEntitlements` [helper executables](#helper-executables).
`minimumSystemVersion` is the oldest macOS the app runs on, 12.0 by
default.

## Windows

The executable carries the icon, the version information and a manifest
(per-monitor DPI awareness, modern controls), and runs without a console
window. A `.syso` file of your own in the main package replaces them.

### The installer

`mygo build` also makes `My App Setup 1.2.0.exe`. It installs the app for
the current user in `%LOCALAPPDATA%\Programs\My App`, which needs no
administrator rights and lets the app [update itself](updates.md), adds a
Start menu shortcut and an uninstaller listed in Settings, and registers
the app's URL schemes and file associations. Its last page offers to run
the app and to create a desktop shortcut, which is checked; silent
installs (`/S`) create none.

The installer is made with [NSIS](https://nsis.sourceforge.io). On
Windows, `mygo build` uses NSIS when it is installed, and otherwise
downloads it the first time it needs it: the official NSIS 3.13 zip, from
a copy on MyGo's GitHub releases or else from SourceForge, checked against
its SHA-256 and kept in `%LOCALAPPDATA%\mygo`. On macOS and
Linux, install NSIS to make Windows installers (`brew install makensis`,
`apt install nsis`); without it, `mygo build` skips the installer.

### Code signing

Unsigned apps make SmartScreen warn users. Sign the executable, the
[helper executables](#helper-executables), the installer and its
uninstaller with a certificate:

```ts
export default defineConfig({
  windows: { certificate: "certs/code-signing.pfx" },
});
```

The password comes from the `MYGO_WINDOWS_CERTIFICATE_PASSWORD`
environment variable. On Windows `signtool` signs, from the Windows SDK;
elsewhere `osslsigncode`. `timestampUrl` changes the time stamping server,
by default `http://timestamp.digicert.com`.

Certificates on hardware tokens or in cloud services, such as Azure Trusted
Signing, sign with a command of their own, in which `%1` is the file:

```ts
export default defineConfig({
  windows: {
    signCommand: "signtool sign /fd sha256 /tr http://timestamp.digicert.com /td sha256 /a %1",
  },
});
```

## Linux

`mygo build` writes the executable, named after the app in lower case, a
desktop entry and the icon, and a Debian package, which installs the app
in `/opt/my-app` with its desktop entry, icons, URL schemes and file
types:

```ts
export default defineConfig({
  linux: {
    maintainer: "Jane Doe <jane@example.com>",
    comment: "Take notes",
    categories: ["Office"],
    depends: ["libayatana-appindicator3-1"],
    command: "my-app",
  },
});
```

The package depends on GTK 3 and WebKitGTK; `depends` adds more packages.
`maintainer` defaults to the `author` of package.json, else the name of
the app.
`comment` describes the app in its desktop entry and package, and
`categories` places it in application menus (`Utility` by default).
The app opens from the applications menu. `command` also gives it a
command, `/usr/bin/my-app` from the package and `~/.local/bin/my-app` from
[`install.sh`](#the-install-script); without it, neither adds a command,
which leaves the name to a command-line tool of the same name, such as
the app's own.

### The install script

Every Linux build also gets the app as an archive,
`my-app-1.2.0-linux-amd64.tar.gz`, and `install.sh`, which installs it for
the user without root: the app in `~/.local/my-app.app`, where it can
[update itself](updates.md), its desktop entry, icon, URL schemes and file
types, and with `linux.command` that command in `~/.local/bin`, unless
another program is there. With
[updates](updates.md) configured, it downloads the latest version, which
the update manifest of the machine names, so one line installs the app:

```sh
curl -fsSL https://github.com/me/my-app/releases/latest/download/install.sh | sh
```

With a [`tagPrefix`](updates.md#set-up) other than `v`, the script of any
release, such as `…/releases/download/desktop-v1.2.0/install.sh`, installs
the newest release tagged with the prefix.

`-upload` publishes it with the archives; with `updates.url`, publish it
next to the manifests. Next to the archive of its version, as in
`build/linux-amd64/`, it installs that archive instead, and it installs
the one it is given:

```sh
sh install.sh my-app-1.2.0-linux-amd64.tar.gz
sh install.sh --uninstall   # keeps the settings and data of the app
```

Running it again installs the version it finds over the installed one.
The app needs WebKitGTK, which `install.sh` does not install: when the
system's library cache has none, it says so after installing, with the
command that installs it on Debian, Ubuntu, Fedora, Arch and openSUSE.
When the app updates itself, it registers the desktop entry and file types
of the new version, so a version that adds URL schemes or file types gets
them. The Debian package suits people who want the system's package manager to
update the app; `install.sh` suits the others, and is the Linux install
that updates itself.

## URL schemes and file types

`urlSchemes` and `fileAssociations` in the configuration (see
[deep links](app.md#deep-links) and [file associations](app.md#file-associations))
are registered by each package: in the macOS app's `Info.plist`, by the
Windows installer, and by the Linux desktop entry, install script and
Debian package.

## Publishing

With `updates.github` set in the configuration, `mygo build -upload` uploads the
disk images, installers, packages, Linux archives and install script, and
update files to the GitHub release of the version, tagged `v1.2.0`,
creating it as a draft. Review the draft and publish it. It needs the
[GitHub CLI](https://cli.github.com) (`gh`), signed in. With `updates.s3`,
it uploads them to a bucket of Amazon S3 or of a compatible service, such
as Cloudflare R2, with the credentials in `AWS_ACCESS_KEY_ID` and
`AWS_SECRET_ACCESS_KEY` (see [publishing to S3](updates.md#publishing-to-s3)).

Build on each platform in CI and upload to the same release: macOS runners
sign, notarize and make the disk images; any runner compiles and packages
Windows and Linux apps. Keep the signing secrets in CI secrets:
`MYGO_WINDOWS_CERTIFICATE_PASSWORD`, `MYGO_UPDATER_PRIVATE_KEY`, and the
notary credentials in a keychain of the macOS runner.
[GitHub Actions](github-actions.md) has a workflow that does it all.
