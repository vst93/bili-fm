# GitHub Actions

A GitHub Actions workflow can build the apps of every platform when you
push a tag, sign them, and upload them with their [updates](updates.md) to
a draft release, which you publish. Each platform builds on a runner of
its own, at the same time. MyGo needs no cgo, so Ubuntu runners make the
Windows apps as well as the Linux ones; the macOS app needs a macOS runner,
because signing, notarization and disk images need macOS.

## The workflow

With `updates.github` in the configuration, save this as
`.github/workflows/release.yml`:

```yaml
name: Release

on:
  push:
    tags: ["v*"]

permissions:
  contents: write # the draft release, and the uploads to it

jobs:
  draft-release:
    runs-on: ubuntu-latest
    steps:
      - name: Create the draft release
        env:
          GH_TOKEN: ${{ github.token }}
        run: |
          if ! gh release view "$GITHUB_REF_NAME" --repo "$GITHUB_REPOSITORY" > /dev/null 2>&1; then
            gh release create "$GITHUB_REF_NAME" --repo "$GITHUB_REPOSITORY" \
              --draft --verify-tag --title "$GITHUB_REF_NAME" --generate-notes
          fi

  build:
    needs: draft-release
    name: ${{ matrix.name }}
    runs-on: ${{ matrix.os }}
    strategy:
      fail-fast: false
      matrix:
        include:
          - { name: macOS, os: macos-latest, platform: "darwin/arm64,darwin/amd64" }
          - { name: Windows, os: ubuntu-latest, platform: "windows/amd64,windows/arm64" }
          - { name: Linux, os: ubuntu-latest, platform: "linux/amd64,linux/arm64" }
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
      - uses: oven-sh/setup-bun@v2
      - run: bun install --frozen-lockfile
      - name: Install NSIS, for the Windows installers
        if: matrix.name == 'Windows' && runner.os == 'Linux'
        run: sudo apt-get update && sudo apt-get install -y nsis
      - name: Build and upload
        env:
          GH_TOKEN: ${{ github.token }}
          MYGO_UPDATER_PRIVATE_KEY: ${{ secrets.MYGO_UPDATER_PRIVATE_KEY }}
        run: bun run build -- -platform "${{ matrix.platform }}" -upload
```

A project of [native UI](ui/README.md) has no frontend, so no Bun: leave out the
`setup-bun` and `bun install` steps, and build with the CLI that `go.mod`
pins as a tool:

```yaml
        run: go tool mygo build -platform "${{ matrix.platform }}" -upload
```

Add the secret key that signs updates, the contents of `mygo-update.key`
(see [auto-updates](updates.md#set-up)), to the repository's secrets:

```sh
gh secret set MYGO_UPDATER_PRIVATE_KEY < path/to/mygo-update.key
```

Then bump `version` in the configuration, commit, and push its tag:

```sh
git tag v1.2.0
git push origin v1.2.0
```

The draft release job creates the draft release `v1.2.0`. Then the build
job runs once for each row of its matrix, at the same time, and each
uploads to the draft: the macOS disk images, the Windows installers, and
the Linux archives, Debian packages and install script, each with their
updates.
Review the draft and publish it: apps then update to it.

- `mygo build` uploads to the release of the configuration's version, with
  the `tagPrefix` of `updates` (`v` by default), whichever tag started the
  workflow. A step can check that they agree, as [below](#checking-the-version).
- The draft exists before the builds start, so they upload to it: without
  it, each `mygo build -upload` would create a draft of its own. Running
  the workflow again keeps the draft, and replaces its files.
- With `fail-fast: false`, the other platforms finish when one fails. "Re-run
  failed jobs" builds that one again, and it uploads to the same draft.
- A row's `platform` may list several targets, which `mygo build` builds
  one after the other. A row for each target, such as `windows/amd64` and
  `windows/arm64`, builds them at the same time, on more runners.
- The release page gets the notes that GitHub generates from the pull
  requests since the last release, which you can edit in the draft. The
  update window shows the version's section of `CHANGELOG.md`, which the
  update manifests carry.
- The `GITHUB_TOKEN` of the workflow lets `gh`, which the runners have,
  create the release and upload to it. With a `tagPrefix`, it also lets
  `mygo build` find the earlier releases that
  [delta updates](updates.md#delta-updates) start from.

As it is, the workflow makes apps that work but are not signed for other
machines: macOS apps signed ad hoc, which Gatekeeper blocks on other Macs,
and Windows apps without a signature, for which SmartScreen warns users.
The next sections sign them.

## Signing and notarizing macOS apps

The macOS row needs your Developer ID certificate and the credentials of
Apple's notary service (see [signing and notarization](distribution.md#signing-and-notarization)).
Export the "Developer ID Application" certificate, with its private key,
from Keychain Access as a `.p12` file with a password, and add the
secrets:

```sh
base64 -i certificate.p12 | gh secret set MACOS_CERTIFICATE
gh secret set MACOS_CERTIFICATE_PASSWORD   # the password of the .p12
gh secret set APPLE_ID                     # the Apple ID of your developer account
gh secret set APPLE_TEAM_ID                # your team ID, as in the identity's (TEAMID)
gh secret set APPLE_APP_PASSWORD           # an app-specific password of the Apple ID
```

Create the app-specific password at
[account.apple.com](https://account.apple.com), under Sign-In and Security.
Then, before "Build and upload", put them in a keychain of the macOS
runner:

```yaml
      - name: Import the certificate and the notary credentials
        if: runner.os == 'macOS'
        env:
          MACOS_CERTIFICATE: ${{ secrets.MACOS_CERTIFICATE }}
          MACOS_CERTIFICATE_PASSWORD: ${{ secrets.MACOS_CERTIFICATE_PASSWORD }}
          APPLE_ID: ${{ secrets.APPLE_ID }}
          APPLE_TEAM_ID: ${{ secrets.APPLE_TEAM_ID }}
          APPLE_APP_PASSWORD: ${{ secrets.APPLE_APP_PASSWORD }}
        run: |
          keychain="$RUNNER_TEMP/signing.keychain-db"
          password="$(openssl rand -base64 24)"
          security create-keychain -p "$password" "$keychain"
          security set-keychain-settings -lut 21600 "$keychain"
          security unlock-keychain -p "$password" "$keychain"
          echo "$MACOS_CERTIFICATE" | base64 --decode > "$RUNNER_TEMP/certificate.p12"
          security import "$RUNNER_TEMP/certificate.p12" -k "$keychain" \
            -P "$MACOS_CERTIFICATE_PASSWORD" -T /usr/bin/codesign
          rm "$RUNNER_TEMP/certificate.p12"
          security set-key-partition-list -S apple-tool:,apple: -s -k "$password" "$keychain"
          security list-keychains -d user -s "$keychain" login.keychain
          xcrun notarytool store-credentials notary --keychain "$keychain" \
            --apple-id "$APPLE_ID" --team-id "$APPLE_TEAM_ID" --password "$APPLE_APP_PASSWORD"
          echo "NOTARY_KEYCHAIN=$keychain" >> "$GITHUB_ENV"
```

The keychain lives as long as the job, and codesign finds the certificate
in it. The configuration signs with the certificate and notarizes with
the profile, from that keychain in the workflow and from the login
keychain on your Mac, where `NOTARY_KEYCHAIN` is not set:

```ts
export default defineConfig({
  macos: {
    signingIdentity: "Developer ID Application: Jane Doe (TEAMID)",
    notarize: { keychainProfile: "notary", keychain: process.env.NOTARY_KEYCHAIN },
  },
});
```

`mygo build` then signs the app, submits the disk image to the notary
service and waits for it, usually a few minutes, and staples the ticket.
An App Store Connect API key works instead of the Apple ID: give
`notarytool store-credentials` the key's file, ID and issuer with `--key`,
`--key-id` and `--issuer`.

## Signing Windows apps

On the Ubuntu runner, `osslsigncode` signs the Windows apps and their
installers with a `.pfx` certificate. Add it and its password to the
secrets:

```sh
base64 -i code-signing.pfx | gh secret set WINDOWS_CERTIFICATE
gh secret set WINDOWS_CERTIFICATE_PASSWORD
```

For the Windows row, install `osslsigncode` with NSIS, write the
certificate to a file, and give its path and password to the build:

```yaml
      - name: Install NSIS and osslsigncode
        if: matrix.name == 'Windows' && runner.os == 'Linux'
        run: sudo apt-get update && sudo apt-get install -y nsis osslsigncode
      - name: Write the certificate
        if: matrix.name == 'Windows'
        env:
          WINDOWS_CERTIFICATE: ${{ secrets.WINDOWS_CERTIFICATE }}
        run: |
          echo "$WINDOWS_CERTIFICATE" | base64 --decode > "$RUNNER_TEMP/code-signing.pfx"
          echo "WINDOWS_CERTIFICATE_FILE=$RUNNER_TEMP/code-signing.pfx" >> "$GITHUB_ENV"
      - name: Build and upload
        env:
          GH_TOKEN: ${{ github.token }}
          MYGO_UPDATER_PRIVATE_KEY: ${{ secrets.MYGO_UPDATER_PRIVATE_KEY }}
          MYGO_WINDOWS_CERTIFICATE_PASSWORD: ${{ secrets.WINDOWS_CERTIFICATE_PASSWORD }}
        run: bun run build -- -platform "${{ matrix.platform }}" -upload
```

```ts
export default defineConfig({
  windows: { certificate: process.env.WINDOWS_CERTIFICATE_FILE },
});
```

The first step replaces the one that installs NSIS. Builds on your
machine, without the variable, are not signed.

### Signing on Windows

Certificates whose keys stay in a hardware token or a cloud service, such
as Azure Trusted Signing, sign with `signCommand` (see
[code signing](distribution.md#code-signing)), which often needs
`signtool`. Build the Windows apps on a Windows runner then, which has
`signtool`, and where `mygo build` downloads NSIS itself; the step that
installs NSIS is for Linux runners only:

```yaml
          - { name: Windows, os: windows-latest, platform: "windows/amd64,windows/arm64" }
```

Sign in to the signing service in a step for the Windows row, before
"Build and upload", as its documentation says. Steps run in PowerShell on
Windows runners unless they set `shell: bash`.

`mygo build` downloads NSIS on every run of a fresh runner. A step before
"Build and upload" keeps it in the Actions cache instead, so builds need
neither the download nor the hosts it comes from. The lockfile pins
mygo-cli, which pins NSIS:

```yaml
      - name: Cache NSIS
        if: runner.os == 'Windows'
        uses: actions/cache@v6
        with:
          path: ~\AppData\Local\mygo\nsis-*
          key: nsis-${{ hashFiles('bun.lock') }}
```

In a project of native UI, `go.mod` pins the CLI: key the cache on
`hashFiles('go.sum')`.

## Checking the version

A tag that does not match the configuration's version would publish the
build under another version. Keep the version in package.json, which the
configuration reads (see [computed configuration](configuration.md#computed-configuration)):

```ts
import pkg from "./package.json" with { type: "json" };

export default defineConfig({
  version: pkg.version,
});
```

and check it in the draft release job, before it creates the draft, so
that a wrong tag builds nothing:

```yaml
      - uses: actions/checkout@v7
      - name: Check the version
        run: test "$GITHUB_REF_NAME" = "v$(jq -r .version package.json)"
```

A project of native UI has the version in `mygo.json`: check
`jq -r .version mygo.json` instead.

## Publishing the release

Publish the draft yourself after looking at it, or let the workflow
publish it once every job uploaded:

```yaml
  publish:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - env:
          GH_TOKEN: ${{ github.token }}
        run: gh release edit "$GITHUB_REF_NAME" --repo "$GITHUB_REPOSITORY" --draft=false
```

With a `tagPrefix` such as `desktop-v`, trigger the workflow on its tags
(`tags: ["desktop-v*"]`), check them against `desktop-v` and the version,
and create and publish the release with `--latest=false`, so the
repository's latest release stays the other releases' (see
[build and publish](updates.md#build-and-publish)).

## Publishing to S3

With `updates.s3`, `mygo build -upload` uploads to the bucket instead (see
[publishing to S3](updates.md#publishing-to-s3)). Give the jobs the
bucket's credentials in place of `GH_TOKEN`:

```yaml
        env:
          AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
          AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          MYGO_UPDATER_PRIVATE_KEY: ${{ secrets.MYGO_UPDATER_PRIVATE_KEY }}
```

There is no draft release: leave out the draft release job, and the
`needs` of the build job. The update of each platform is out as soon as
its job uploaded its manifest, and the workflow needs no `contents: write`
permission.

## Without a release

Without `updates`, `-upload` has nowhere to upload to. Build without it and
keep the installers as artifacts of the run, which works on every push too,
to check that the apps build:

```yaml
      - run: bun run build -- -platform "${{ matrix.platform }}"
      - uses: actions/upload-artifact@v7
        with:
          name: ${{ matrix.name }}
          path: |
            build/*/*.dmg
            build/*/*Setup*.exe
            build/*/*.deb
            build/*/*.tar.gz
```

`build` is the `out` of new projects' configuration.
