import { getRouteApi } from "@tanstack/react-router"
import { ArrowLeftIcon, ArrowRightIcon, ChevronRightIcon, MenuIcon } from "lucide-react"
import * as React from "react"

import { DocContent } from "@/components/docs/doc-content"
import { DocLink } from "@/components/docs/doc-link"
import { DocsNav } from "@/components/docs/docs-nav"
import { Button } from "@/components/ui/button"
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from "@/components/ui/sheet"
import { bookOf, getDoc, type Doc, type Navs } from "@/lib/docs"

/**
 * The loader of a page: null, before anything is fetched, for a slug without
 * a page, which the docs show as not found. (Rather than throwing notFound():
 * the router renders routes as the prerendered 404.html does, not errors.)
 */
export async function loadDocPage(slug: string, parent: Promise<{ loaderData?: Navs }>) {
  const nav = (await parent).loaderData?.[bookOf(slug)]
  if (!nav?.some((s) => s.items.some((i) => i.slug === slug))) return null
  return getDoc({ data: slug })
}

export function docHead(doc: Doc | null | undefined) {
  if (!doc) return {}
  const title = doc.slug ? `${doc.title} · MyGo` : doc.title
  return {
    meta: [
      { title },
      { name: "description", content: doc.description },
      { property: "og:title", content: title },
      { property: "og:description", content: doc.description },
    ],
    links: [
      { rel: "alternate", type: "text/markdown", href: (doc.slug ? `/docs/${doc.slug}` : "/docs") + ".md" },
      { rel: "describedby", href: "/llms.txt" },
    ],
  }
}

const docsRoute = getRouteApi("/docs")

export function DocPage({ doc }: { doc: Doc }) {
  const book = bookOf(doc.slug)
  const nav = docsRoute.useLoaderData()[book]
  const [menuOpen, setMenuOpen] = React.useState(false)

  return (
    <div className="min-w-0">
      <div className="sticky top-14 z-30 flex h-11 items-center gap-1 border-b bg-background/85 px-2 backdrop-blur-xl lg:hidden">
        <Sheet open={menuOpen} onOpenChange={setMenuOpen}>
          <SheetTrigger render={<Button variant="ghost" size="sm" />}>
            <MenuIcon /> Menu
          </SheetTrigger>
          <SheetContent side="left" className="w-72 overflow-y-auto">
            <SheetHeader>
              <SheetTitle>{book === "ui" ? "Native UI" : "Documentation"}</SheetTitle>
            </SheetHeader>
            <div className="px-2 pb-8">
              <DocsNav sections={nav} page={doc} onNavigate={() => setMenuOpen(false)} />
            </div>
          </SheetContent>
        </Sheet>
        <ChevronRightIcon className="size-3.5 shrink-0 text-muted-foreground" />
        <span className="truncate text-sm">{doc.title}</span>
      </div>

      <article className="max-w-[52rem] px-4 pt-10 pb-20 sm:px-10 lg:pt-14">
        <header>
          {doc.section && <p className="label">{doc.section}</p>}
          <h1 className="mt-3 text-3xl font-semibold tracking-[-0.03em] sm:text-4xl">{doc.title}</h1>
          {doc.summary && <p className="mt-3 text-lg leading-8 text-muted-foreground">{doc.summary}</p>}
        </header>

        <div className="mt-10">
          <DocContent html={doc.html} />
        </div>

        <footer className="mt-16">
          <nav className="grid gap-px overflow-hidden rounded-lg border bg-border sm:grid-cols-2">
            {doc.prev ? (
              <DocLink slug={doc.prev.slug} className="flex flex-col gap-1 bg-background p-4 transition-colors hover:bg-code">
                <span className="label flex items-center gap-1.5">
                  <ArrowLeftIcon className="size-3" /> Previous
                </span>
                <span className="font-medium">{doc.prev.title}</span>
              </DocLink>
            ) : (
              <span className="hidden bg-background sm:block" />
            )}
            {doc.next && (
              <DocLink slug={doc.next.slug} className="flex flex-col items-end gap-1 bg-background p-4 text-right transition-colors hover:bg-code">
                <span className="label flex items-center gap-1.5">
                  Next <ArrowRightIcon className="size-3" />
                </span>
                <span className="font-medium">{doc.next.title}</span>
              </DocLink>
            )}
          </nav>
          <a href={doc.editUrl} className="mt-6 inline-block text-sm text-muted-foreground hover:text-foreground">
            Edit this page on GitHub
          </a>
        </footer>
      </article>
    </div>
  )
}
