import { createServerFn } from "@tanstack/react-start"
import { staticFunctionMiddleware } from "@tanstack/start-static-server-functions"

import { loadDoc, loadNav } from "@/lib/docs.server"

export interface TocItem {
  id: string
  text: string
  depth: number
}

/** A page of the docs (`slug`, "" for the introduction) or an external link (`href`). */
export interface NavItem {
  slug?: string
  href?: string
  title: string
  description?: string
}

export interface NavSection {
  title: string
  items: NavItem[]
}

/** The docs, and native UI's, which have a sidebar of their own at /docs/ui. */
export type Book = "docs" | "ui"

export type Navs = Record<Book, NavSection[]>

/** The book of a page: native UI's for ui and the pages in ui/. */
export function bookOf(slug: string): Book {
  return slug === "ui" || slug.startsWith("ui/") ? "ui" : "docs"
}

/** The slug of the page at a path: "/docs/ui/button" → "ui/button". */
export function slugOfPath(pathname: string) {
  return pathname.replace(/^\/docs\/?/, "").replace(/\/$/, "")
}

export interface Doc {
  slug: string
  title: string
  /** For search engines: the summary, else the first paragraph. */
  description: string
  /** What the page covers, from README.md's list of pages. */
  summary?: string
  /** The sidebar section of the page. */
  section?: string
  html: string
  toc: TocItem[]
  prev?: { slug: string; title: string }
  next?: { slug: string; title: string }
  editUrl: string
}

export interface SearchEntry {
  slug: string
  page: string
  /** The anchor of the heading, "" for the top of the page. */
  id: string
  heading: string
  text: string
}

export function docHref(slug: string, id = "") {
  return (slug ? `/docs/${slug}` : "/docs") + (id ? `#${id}` : "")
}

// Server functions run while the site is prerendered; the static middleware
// saves their results as JSON files, which client-side navigation fetches.

export const getDocsNav = createServerFn({ method: "GET" })
  .middleware([staticFunctionMiddleware])
  .handler(() => loadNav())

export const getDoc = createServerFn({ method: "GET" })
  .middleware([staticFunctionMiddleware])
  .validator((slug: string) => slug)
  .handler(async ({ data }) => (await loadDoc(data)) ?? null)
