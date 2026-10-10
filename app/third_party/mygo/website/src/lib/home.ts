import type { KeyedTokensInfo } from "@shikijs/magic-move/types"
import { createServerFn } from "@tanstack/react-start"
import { staticFunctionMiddleware } from "@tanstack/start-static-server-functions"

import { loadHome } from "@/lib/home.server"

export interface HomeData {
  /** main.go of a window with a web page, then of one with native UI, which morph into each other. */
  main: KeyedTokensInfo[]
  /** The page's code, highlighted. */
  ts: string
}

export const getHome = createServerFn({ method: "GET" })
  .middleware([staticFunctionMiddleware])
  .handler(() => loadHome())
