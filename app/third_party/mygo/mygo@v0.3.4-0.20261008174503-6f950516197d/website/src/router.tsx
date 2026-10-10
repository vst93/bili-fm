import { createRouter } from "@tanstack/react-router"

import { routeTree } from "./routeTree.gen"

export function getRouter() {
  // 404.html is the page of /404, served at any path without a page: the
  // router reads that path as /404, so that the page hydrates as rendered,
  // and still shows the path in the address bar.
  const notFound = typeof window === "undefined" ? undefined : window.__notFoundPath
  return createRouter({
    routeTree,
    scrollRestoration: true,
    defaultPreload: "intent",
    rewrite: notFound
      ? {
          input: ({ url }) => {
            if (url.pathname === notFound) url.pathname = "/404"
            return url
          },
          output: ({ url }) => {
            if (url.pathname === "/404") url.pathname = notFound
            return url
          },
        }
      : undefined,
  })
}

declare module "@tanstack/react-router" {
  interface Register {
    router: ReturnType<typeof getRouter>
  }
}
