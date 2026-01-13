# AGENTS.md - Bili FM Codebase Guide

This file provides guidelines for agentic coding agents operating in this repository.

## Project Overview

Bili FM is a cross-platform desktop application built with [Wails](https://wails.io/) (Go backend + React/TypeScript frontend). It allows users to listen to Bilibili videos as audio.

## Build/Lint/Test Commands

### Frontend Development (in `/frontend`)

```bash
# Install dependencies
npm install

# Start development server
npm run dev

# Type-check + build for production
npm run build

# Lint with auto-fix
npm run lint

# Preview production build
npm run preview
```

### Full Application Build (from root)

```bash
# Build desktop app (requires wails CLI)
wails build

# Build for all platforms (see build.sh)
./build.sh

# Go commands
go mod tidy
go build
```

### Wails-specific Commands

```bash
# Development with hot reload
wails dev

# Generate bindings
wails generate module
```

## Code Style Guidelines

### Frontend (TypeScript/React)

**TypeScript Configuration**
- Strict mode enabled (`"strict": true`)
- Path alias: `@/*` maps to `./src/*`
- Target: ES2020, Module: ESNext
- `noUnusedLocals: true`, `noUnusedParameters: true`

**Import Ordering** (enforced by ESLint)
Order groups (from top to bottom):
1. `type` imports
2. Built-in modules
3. Object/builtin types
4. External packages (npm)
5. Internal aliases (`~/**`)
6. Parent directories (`../`)
7. Sibling files (`./`)
8. Index imports (`./index`)

Example:
```typescript
import type { FC } from "react";
import { useState } from "react";
import { Button } from "@heroui/button";
import { useQuery } from "@tanstack/react-query";
import { formatDate } from "@/utils/date";
import "./styles.css";
import IndexPage from "@/pages/index";
```

**React Component Patterns**
- Use functional components with TypeScript interfaces
- Destructure props directly in function signature
- Use optional chaining and nullish coalescing: `onClick?.()`, `value ?? default`
- Use `useEffect` cleanup functions for subscriptions/timers
- Prefer `useCallback` and `useMemo` for expensive operations

**Naming Conventions**
- Components: PascalCase (`Player`, `VideoInfo`)
- Hooks: camelCase with `use` prefix (`useTheme`, `useAuth`)
- Utils/constants: camelCase (`formatDate`, `DEFAULT_PAGE_SIZE`)
- Types/interfaces: PascalCase with descriptive names (`VideoItem`, `SearchResult`)
- CSS classes: lowercase with dashes (Tailwind)

**Error Handling**
- Handle async operations with try/catch
- Show user feedback via Toast components
- Log errors with context for debugging

**Styling**
- Tailwind CSS for utility classes
- Tailwind Variants for component variants
- HeroUI components for consistent design
- Dark mode support via `dark` class on root

### Go Backend (in `/service` and root)

**Go Version**
- Go 1.24.0+

**Package Structure**
- Root package: `main`
- Service package: `bilifm/service`
- Import paths use module `bilifm`

**Naming Conventions**
- Package names: lowercase single word (`service`, `util`)
- Exported types/functions: PascalCase (`App`, `GetLoginStatus`)
- Unexported: lowercase (`startup`, `app`)
- Struct fields: PascalCase with JSON tags

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

**Error Handling**
- Return errors as values, don't suppress with `_`
- Use `errors.New()` or `fmt.Errorf()` for error creation
- Check errors immediately after calls
- Handle errors at appropriate level (return or log)

**Import Organization**
- Standard library first, then external packages
- Blank line between groups
- Alphabetical within groups

```go
import (
    "context"
    "fmt"
    "net/http"

    "github.com/wailsapp/wails/v2"
    "github.com/wailsapp/wails/v2/pkg/menu"
)
```

**JSON Handling**
- Use struct tags for JSON field mapping
- Decode with `json.NewDecoder(r.Body).Decode(&struct)`
- Encode with `json.MarshalIndent()` for debugging

**Wails Binding**
- Exported methods are automatically bound to frontend
- Bind struct methods in `wails.Run()` `Bind` option
- Frontend calls via `wailsjs/go/package/MethodName`

### General Guidelines

**File Organization**
- Frontend: `src/components/` for reusable, `src/pages/` for routes
- Go: `service/` for business logic, root for main/app
- Keep files focused (<300 lines preferred)

**Commit Messages**
- Chinese or English, imperative mood
- Format: `type(scope): description`
- Types: `feat`, `fix`, `refactor`, `docs`, `chore`

**Testing**
- No test framework currently configured
- Manual testing via `wails dev` or frontend dev server
- Test critical paths before committing

**Visual/UI Changes**
- Delegate to frontend-ui-ux-engineer for styling/layout changes
- Pure logic changes (API calls, state) can be handled directly
