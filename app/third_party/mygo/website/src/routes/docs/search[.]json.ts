import { createFileRoute } from "@tanstack/react-router"

import { loadSearchIndex } from "@/lib/docs.server"

// Prerendered to /docs/search.json, which the search of the site fetches.
export const Route = createFileRoute("/docs/search.json")({
  server: {
    handlers: {
      GET: async () => Response.json(await loadSearchIndex()),
    },
  },
})
