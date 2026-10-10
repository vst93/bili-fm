import { createFileRoute } from "@tanstack/react-router"

import { NotFound } from "@/components/not-found"
import { notFoundScript } from "@/lib/not-found"

// Prerendered as 404.html, which Cloudflare serves for paths without a page.
export const Route = createFileRoute("/404")({
  head: () => ({
    meta: [{ title: "Page not found · MyGo" }],
    scripts: [{ children: notFoundScript }],
  }),
  component: NotFound,
})
