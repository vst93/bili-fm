# AGENTS.md - Bili FM Codebase Guide

This file gives coding agents the repository-specific context needed to work on Bili FM without rediscovering the basics.

## Project Overview

Bili FM is a cross-platform desktop application for listening to Bilibili videos as audio. It is built with Wails v2: a Go backend packaged with a React/TypeScript frontend.

Primary stack:
- Backend: Go module `bilifm`, Wails v2.12.0
- Frontend: React 19, TypeScript, Vite 6, Tailwind CSS, HeroUI
- Package manager: `pnpm` for frontend commands
- Generated bindings: `frontend/wailsjs/`

## Build, Lint, and Development Commands

### Frontend Development (`frontend/`)

```bash
# Install dependencies using the repo's Wails-compatible install script
pnpm run install:ci

# Start Vite development server
pnpm run dev

# Type-check and build production assets
pnpm run build

# Lint and auto-fix frontend source files
pnpm run lint

# Preview production frontend build
pnpm run preview
```

The frontend build script is `tsc && vite build`, so TypeScript errors fail the build before Vite bundles.

### Full Application (`/`)

```bash
# Start Wails development mode with frontend watcher
wails dev

# Build the desktop application
wails build

# Generate Wails bindings after changing exported Go methods/types
wails generate module

# Build release archives for configured platforms
./build.sh

# Go checks
go build ./...
go mod tidy
```

`wails.json` drives frontend integration:
- `frontend:install`: `pnpm run install:ci`
- `frontend:build`: `pnpm run build`
- `frontend:dev:watcher`: `pnpm run dev`

## Repository Layout

- `main.go`, `app.go`, `menu.go`, `tray_*.go`, `dpi_*.go`: Wails app setup, platform behavior, menus/tray, embedded assets, image proxying, and startup wiring.
- `service/`: Bilibili API, app config, persistence, and backend utility code.
- `service/dkv/`: small disk-backed key-value storage helper.
- `frontend/src/`: React app source.
- `frontend/src/components/`: reusable UI and player/list components.
- `frontend/src/pages/`: route-level pages.
- `frontend/src/hooks/`, `frontend/src/utils/`, `frontend/src/types/`, `frontend/src/config/`: shared frontend support code.
- `frontend/wailsjs/`: generated Wails runtime and Go bindings. Do not hand-edit generated files.
- `build/`, `build/bin/`: Wails build output.

## Frontend Guidelines

### TypeScript and React

- Strict TypeScript is enabled.
- Target is `ES2020`; module format is `ESNext`; module resolution is `bundler`.
- Path alias `@/*` maps to `frontend/src/*` via `tsconfig.json` and `vite-tsconfig-paths`.
- `noUnusedLocals`, `noUnusedParameters`, and `noFallthroughCasesInSwitch` are enabled.
- Use function components and typed props.
- Destructure props in the function signature when it stays readable.
- Use optional chaining and nullish coalescing for nullable data.
- Clean up subscriptions, timers, event listeners, and Wails runtime callbacks in `useEffect` cleanup functions.
- Memoize only when it protects expensive work, stable callback identity, or noisy child renders.

### Imports and Formatting

ESLint enforces import ordering and Prettier warnings. Keep groups separated by blank lines:

1. `type`
2. builtin
3. object
4. external
5. internal
6. parent
7. sibling
8. index

The current ESLint config has a `~/**` path group but the active project alias is `@/*`; prefer `@/` for internal frontend imports.

Example:

```typescript
import type { FC } from "react";
import { useMemo } from "react";

import { Button } from "@heroui/button";

import { formatTitle } from "@/utils/string";

import { VideoCover } from "./videoCover";
```

### Styling and UI

- Use Tailwind CSS utilities and HeroUI components unless the local component already establishes a different pattern.
- Tailwind dark mode is class-based (`darkMode: "class"`).
- HeroUI is registered through `@heroui/theme` in `frontend/tailwind.config.js`.
- IconPark is available through `@icon-park/react`; prefer the project's existing icon approach for consistency.
- Keep visual changes consistent with the existing liquid-glass app direction from the README and current components.
- For substantial UI redesign work, use the `frontend-design` skill before implementation.

### Frontend Error Handling

- Wrap async calls in `try/catch` at the interaction boundary.
- Surface user-facing failures through existing toast utilities/components.
- Log errors with enough context to diagnose the failing action.
- Avoid swallowing rejected Wails calls; return or display meaningful failure state.

## Go Backend Guidelines

### Version and Packages

- Go version in `go.mod`: `1.25.0`.
- Module path: `bilifm`.
- Root package: `main`.
- Backend service package: `bilifm/service`.
- Keep platform-specific code behind Go build tags or platform-suffixed files, following the existing `*_windows.go` and `*_other.go` pattern.

### Style

- Standard library imports first, then external/internal packages separated by a blank line.
- Return errors as values; do not suppress errors with `_` unless the failure is intentionally irrelevant and obvious.
- Use `fmt.Errorf` with context for wrapped or constructed errors.
- Keep exported Wails-bound types and methods stable because they affect generated frontend bindings.
- Use JSON tags on structs that cross API or frontend boundaries.

Example:

```go
type User struct {
    ID       int64  `json:"id"`
    Username string `json:"username"`
}

func (bl *BL) GetUser(id int64) (*User, error) {
    // implementation
}
```

### Wails Bindings

- Exported methods on bound Go structs are callable from the frontend.
- After changing exported methods, parameter types, return types, or JSON-facing structs, run:

```bash
wails generate module
```

- Frontend code calls generated bindings from `frontend/wailsjs/go/...`.
- Do not manually edit files under `frontend/wailsjs`; regenerate them.

## Cross-Boundary Changes

When a change touches both Go and TypeScript:

- Update Go structs/method signatures first.
- Regenerate Wails bindings.
- Update TypeScript callers to use generated types/functions.
- Run `pnpm run build` in `frontend/` and `go build ./...` from the root when feasible.

## Testing and Verification

There is no dedicated test framework configured in this repository. Use focused verification:

- Frontend-only logic or UI: `cd frontend && pnpm run build`; use `pnpm run dev` for manual checks.
- Backend-only logic: `go build ./...`.
- Wails integration: `wails dev` for manual desktop verification.
- Release packaging: `./build.sh` when validating multi-platform artifacts.

If a command cannot be run because of missing local tooling, dependency installation, network access, or platform limits, mention that in the final handoff.

## Naming Conventions

Frontend:
- Components: PascalCase, matching existing filenames where the repo already uses lowercase component filenames.
- Hooks: camelCase with `use` prefix.
- Utilities/constants: camelCase for functions, UPPER_SNAKE_CASE for true constants.
- Types/interfaces: PascalCase with descriptive names.

Go:
- Package names: short lowercase names.
- Exported identifiers: PascalCase.
- Unexported identifiers: camelCase.
- Struct fields crossing JSON or Wails boundaries: exported with JSON tags.

## Commit Guidance

- Commit messages may be Chinese or English.
- Prefer imperative mood.
- Suggested format: `type(scope): description`.
- Common types: `feat`, `fix`, `refactor`, `docs`, `chore`.

## Working Notes for Agents

- Prefer small, scoped edits that match surrounding code style.
- Do not overwrite generated files manually.
- Do not treat `frontend/README.md` as project documentation; it is still the upstream Vite/NextUI template text.
- Avoid broad refactors while fixing a narrow bug.
- Preserve user changes in a dirty worktree; do not reset or revert unrelated files.
