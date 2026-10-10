import { Outlet, createFileRoute, useLocation, useMatches } from "@tanstack/react-router"

import { DocsNav } from "@/components/docs/docs-nav"
import { bookOf, getDocsNav, slugOfPath, type Doc } from "@/lib/docs"

export const Route = createFileRoute("/docs")({
  loader: () => getDocsNav(),
  staleTime: Infinity,
  component: DocsLayout,
})

function DocsLayout() {
  const nav = Route.useLoaderData()
  const page = useMatches({ select: (matches) => matches.at(-1)?.loaderData as Doc | undefined })
  // Native UI's pages have a sidebar of their own.
  const book = bookOf(useLocation({ select: (l) => slugOfPath(l.pathname) }))
  return (
    <div className="flex-1 lg:grid lg:grid-cols-[15rem_minmax(0,1fr)]">
      <aside className="sticky top-14 hidden h-[calc(100svh-3.5rem)] overflow-y-auto border-r px-3 py-8 lg:block">
        <DocsNav sections={nav[book]} page={page?.toc ? page : undefined} />
      </aside>
      <Outlet />
    </div>
  )
}
