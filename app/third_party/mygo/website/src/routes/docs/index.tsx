import { createFileRoute } from "@tanstack/react-router"

import { DocPage, docHead, loadDocPage } from "@/components/docs/doc-page"
import { NotFound } from "@/components/not-found"

export const Route = createFileRoute("/docs/")({
  loader: ({ parentMatchPromise }) => loadDocPage("", parentMatchPromise),
  head: ({ loaderData }) => docHead(loaderData),
  component: DocRoute,
})

function DocRoute() {
  const doc = Route.useLoaderData()
  return doc ? <DocPage doc={doc} /> : <NotFound />
}
